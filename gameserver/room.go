package gameserver

import (
	"context"
	"errors"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bs-iron-trio/go-kusokurae/sm"
)

var ErrRoomFull = errors.New("room is full!!!")
var ErrRoomNotFound = errors.New("room not found!!!")
var ErrRoomPlayerNotFound = errors.New("this player not contained by room")
var ErrGameAlreadyStarted = errors.New("game already started")
var ErrNotEnoughPlayers = errors.New("not enough players")
var ErrNotHost = errors.New("only host can start game")

type Room struct {
	ID                  string
	Ctx                 context.Context
	cancel              context.CancelFunc
	Mutex               sync.Mutex
	GameConfig          *sm.GameConfig
	game                atomic.Pointer[Game]
	HostPlayerIdx       int32
	CurrentPlayers      int32
	Players             []*Player
	TurnTimeoutSec      int32
	TurnSyncIntervalSec int32
	nextStartPlayer     int32

	// Slot arrays for run() select — nil means empty/inactive
	opChs     [4]chan Message
	discChs   [4]<-chan struct{}
	eventCh   chan GameEvent
	gameEndCh chan struct{}

	// Current session per seat; owned exclusively by run() goroutine.
	sessions      [4]*Session
	resyncPending [4]bool

	// Internal command channel for thread-safe slot array writes
	internalCh chan func()
}

var roomRepositoryMu sync.Mutex
var roomRepository map[string]*Room

func InitRoomRepository() {
	roomRepository = make(map[string]*Room)
}

func GetRoomByID(roomID string) (*Room, error) {
	roomRepositoryMu.Lock()
	defer roomRepositoryMu.Unlock()
	room, exists := roomRepository[roomID]
	if !exists {
		return nil, ErrRoomNotFound
	}
	return room, nil
}

func NewRoom(id string, host *Player, config *sm.GameConfig) *Room {
	roomRepositoryMu.Lock()
	defer roomRepositoryMu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	r := &Room{
		ID:             id,
		Ctx:            ctx,
		cancel:         cancel,
		GameConfig:     config,
		HostPlayerIdx:  0,
		CurrentPlayers: 1,
		Players:        make([]*Player, config.NumPlayers),
		internalCh:     make(chan func()),
	}
	r.Players[0] = host
	host.Sit(id, 0)
	roomRepository[id] = r

	// Wire host's channels into slot 0
	r.opChs[0] = host.OperatorCh

	// Single goroutine for all message routing
	go r.run()

	return r
}

func (r *Room) Game() *Game { return r.game.Load() }

func (r *Room) AddPlayer(player *Player) error {
	errCh := make(chan error, 1)
	r.internalCh <- func() {
		errCh <- r.addPlayerInternal(player)
	}
	return <-errCh
}

func (r *Room) addPlayerInternal(player *Player) error {
	r.Mutex.Lock()
	defer r.Mutex.Unlock()
	if r.CurrentPlayers >= r.GameConfig.NumPlayers {
		return ErrRoomFull
	}
	position := r.CurrentPlayers
	r.Players[position] = player
	r.CurrentPlayers++
	player.Sit(r.ID, position)

	// Wire player's channels into the slot for run() select
	r.opChs[position] = player.OperatorCh

	r.Broadcast(Message{
		MsgType: MSG_TYPE_PLAYER_JOINED,
		MsgBody: &PlayerJoinedBody{PlayerID: player.ID, Position: position},
	})
	r.broadcastRoomState()
	return nil
}

func (r *Room) FindPlayerByID(playerID string) (*Player, error) {
	for _, p := range r.Players {
		if p != nil && p.ID == playerID {
			return p, nil
		}
	}
	return nil, ErrRoomPlayerNotFound
}

func (r *Room) Broadcast(msg Message) {
	// for _, p := range r.Players {
	// 	if p != nil && p.Session != nil {
	// 		p.NoticeCh <- msg
	// 	}
	// }
	r.broadcastToPlayers(r.Players, msg)
}

func (r *Room) buildRoomStateMessage() Message {
	players := make([]RoomPlayerInfo, r.CurrentPlayers)
	for i := int32(0); i < r.CurrentPlayers; i++ {
		players[i] = RoomPlayerInfo{
			PlayerID: r.Players[i].ID,
			Nickname: r.Players[i].Nickname,
			Position: i,
			IsHost:   i == r.HostPlayerIdx,
		}
	}
	return Message{
		MsgType: MSG_TYPE_ROOM_STATE,
		MsgBody: &RoomStateBody{Players: players, HostIdx: r.HostPlayerIdx},
	}
}

func (r *Room) broadcastRoomState() {
	r.Broadcast(r.buildRoomStateMessage())
}

// BuildRoomStateMessage returns the current room roster as a ROOM_STATE message.
// Thread-safe: takes the room mutex, safe to call from the HTTP handler goroutine.
func (r *Room) BuildRoomStateMessage() Message {
	r.Mutex.Lock()
	defer r.Mutex.Unlock()
	return r.buildRoomStateMessage()
}

// startPlayerForNextGame returns the first-round leader seat for the next game
// and advances the rotation counter. Rotates 0,1,2,... mod NumPlayers for as
// long as the room exists; never reset (see ADR-0008).
func (r *Room) startPlayerForNextGame() int32 {
	p := r.nextStartPlayer
	r.nextStartPlayer = (r.nextStartPlayer + 1) % r.GameConfig.NumPlayers
	return p
}

func (r *Room) StartGame(requesterID string) error {
	r.Mutex.Lock()
	defer r.Mutex.Unlock()

	if r.game.Load() != nil {
		return ErrGameAlreadyStarted
	}
	if r.CurrentPlayers < r.GameConfig.NumPlayers {
		return ErrNotEnoughPlayers
	}
	if r.Players[r.HostPlayerIdx].ID != requesterID {
		return ErrNotHost
	}

	players := make([]*Player, r.GameConfig.NumPlayers)
	copy(players, r.Players[:r.GameConfig.NumPlayers])
	cfg := *r.GameConfig
	cfg.FirstPlayerIdx = r.startPlayerForNextGame()
	g := NewGame(&cfg, int32(len(players)))
	g.TurnTimeout = time.Duration(r.TurnTimeoutSec) * time.Second
	if g.TurnTimeout <= 0 {
		g.TurnTimeout = DefaultTurnTimeout
	}
	g.TurnSyncInterval = time.Duration(r.TurnSyncIntervalSec) * time.Second
	if g.TurnSyncInterval <= 0 {
		g.TurnSyncInterval = DefaultTurnSyncInterval
	}
	r.game.Store(g)

	// Wire game channels into run() select
	r.eventCh = g.EventCh
	r.gameEndCh = g.GameEnd

	go g.GameFn(context.Background())

	return nil
}

func (r *Room) sendToPlayer(p *Player, msg Message) {
	if p == nil || r.sessions[p.RoomPosition] == nil {
		return
	}
	p.NoticeCh <- msg
	log.Printf("Room %s notice %s to %s\n", trunc8(r.ID), msg.MsgType, trunc8(p.ID))
}

func (r *Room) broadcastToPlayers(players []*Player, msg Message) {
	for _, p := range players {
		r.sendToPlayer(p, msg)
	}
}

func (r *Room) isPlayerConnected(p *Player) bool {
	return p != nil && r.sessions[p.RoomPosition] != nil
}

func (r *Room) autoPlayCard(g *Game, idx int) {
	// Pick the largest playable card from the current engine state.
	g.StateMutex.Lock()
	chosen := -1
	if g.State != nil {
		if p := g.State.GetPlayer(int32(idx)); p != nil {
			chosen = sm.PickLargestPlayable(p.GetHandCards())
		}
	}
	g.StateMutex.Unlock()
	if chosen < 0 {
		return
	}
	select {
	case g.CmdCh <- GameCommand{
		PlayerIdx: idx,
		Msg: Message{
			MsgType: MSG_TYPE_PLAY_CARD,
			MsgBody: &PlayCardBody{CardIndex: chosen},
		},
	}:
	case <-g.GameEnd:
	}
}

func (r *Room) handlePlayerMessage(idx int, msg Message) {
	p := r.Players[idx]
	if p == nil {
		return
	}

	log.Printf("Room %s Player %s recv msg %s\n", trunc8(r.ID), trunc8(p.ID), msg.MsgType)

	switch msg.MsgType {
	case MSG_TYPE_START_GAME:
		if err := r.StartGame(p.ID); err != nil {
			r.sendToPlayer(p, Message{
				MsgType: MSG_TYPE_ERROR,
				MsgBody: &ErrorBody{Message: err.Error()},
			})
		}

	case MSG_TYPE_PLAY_CARD:
		if g := r.game.Load(); g != nil {
			select {
			case g.CmdCh <- GameCommand{
				PlayerIdx: int(p.RoomPosition),
				Msg:       msg,
			}:
			case <-g.GameEnd:
			case <-r.Ctx.Done():
			}
		}

	case MSG_TYPE_RESYNC_STATE:
		r.handleResyncState(int(p.RoomPosition))
	}
}

// handleResyncState serves a RESYNC_STATE request: marks the seat as awaiting
// its game snapshot and either queues the request to the game goroutine or
// replies immediately when no game is in progress.
func (r *Room) handleResyncState(idx int) {
	p := r.Players[idx]
	if p == nil {
		return
	}
	r.resyncPending[idx] = true
	if g := r.game.Load(); g != nil {
		select {
		case g.ResyncCh <- int32(idx):
		default:
			// Game cannot accept the request right now: reply without game part.
			r.sendResyncState(p, nil)
		}
		return
	}
	r.sendResyncState(p, nil)
}

// sendResyncState sends the composite RESYNC_STATE reply (room part always,
// game part when provided) to the player and clears the pending flag.
func (r *Room) sendResyncState(p *Player, game *GameResyncBody) {
	r.resyncPending[p.RoomPosition] = false
	roomMsg := r.buildRoomStateMessage()
	roomBody := roomMsg.MsgBody.(*RoomStateBody)
	r.sendToPlayer(p, Message{
		MsgType: MSG_TYPE_RESYNC_STATE,
		MsgBody: &ResyncStateBody{
			Room: RoomResyncBody{
				Players: roomBody.Players,
				HostIdx: roomBody.HostIdx,
				Game:    game,
			},
		},
	})
}

func (r *Room) handleGameEvent(event GameEvent) {
	switch event.Msg.MsgType {
	case MSG_TYPE_YOUR_TURN:
		idx := event.Target
		if !r.isPlayerConnected(r.Players[idx]) {
			if g := r.game.Load(); g != nil {
				r.autoPlayCard(g, idx)
			}
		} else {
			r.sendToPlayer(r.Players[idx], event.Msg)
		}

	case MSG_TYPE_GAME_RESYNC:
		idx := event.Target
		if idx < 0 || idx >= len(r.Players) || !r.resyncPending[idx] {
			return
		}
		p := r.Players[idx]
		if p == nil {
			return
		}
		body, ok := event.Msg.MsgBody.(*GameResyncBody)
		if !ok {
			return
		}
		r.sendResyncState(p, body)

	default:
		if event.Target == -1 {
			r.broadcastToPlayers(r.Players, event.Msg)
		} else {
			r.sendToPlayer(r.Players[event.Target], event.Msg)
		}
	}
}

func (r *Room) handleDisconnect(idx int) {
	p := r.Players[idx]
	if p == nil {
		return
	}

	log.Printf("Player %s disconnect\n", trunc8(p.ID))

	// Nil out the fired channel immediately to prevent tight loop.
	// A closed channel fires on every select iteration, so we must
	// remove it before any early-return path.
	r.discChs[idx] = nil
	r.sessions[idx] = nil
	r.resyncPending[idx] = false

	// Auto-play immediately if it's this disconnected player's turn
	if g := r.game.Load(); g != nil {
		g.StateMutex.Lock()
		isTurn := false
		if g.State != nil {
			if ap := g.State.GetActivePlayer(); ap != nil {
				isTurn = ap.GetIndex()-1 == idx
			}
		}
		g.StateMutex.Unlock()
		if isTurn {
			r.autoPlayCard(g, idx)
		}
	}
}

func (r *Room) handleGameEnd() {
	r.eventCh = nil
	r.gameEndCh = nil
	// Release the finished game so the room can start a new one.
	r.game.Store(nil)
}

// AttachSession registers the current session for a seat and returns the
// previous session (if any). Serialized through the room goroutine so the
// connection state is owned solely by run(). The room selects on the current
// session's DiscCh only, so a stale session's detach is naturally ignored.
func (r *Room) AttachSession(idx int32, s *Session) *Session {
	result := make(chan *Session, 1)
	r.internalCh <- func() {
		old := r.sessions[idx]
		r.sessions[idx] = s
		r.discChs[idx] = s.DiscCh()
		result <- old
	}
	return <-result
}

func (r *Room) run() {
	for {
		select {
		case msg := <-r.opChs[0]:
			r.handlePlayerMessage(0, msg)
		case <-r.discChs[0]:
			r.handleDisconnect(0)
		case msg := <-r.opChs[1]:
			r.handlePlayerMessage(1, msg)
		case <-r.discChs[1]:
			r.handleDisconnect(1)
		case msg := <-r.opChs[2]:
			r.handlePlayerMessage(2, msg)
		case <-r.discChs[2]:
			r.handleDisconnect(2)
		case msg := <-r.opChs[3]:
			r.handlePlayerMessage(3, msg)
		case <-r.discChs[3]:
			r.handleDisconnect(3)
		case event := <-r.eventCh:
			r.handleGameEvent(event)
		case <-r.gameEndCh:
			r.handleGameEnd()
		case fn := <-r.internalCh:
			fn()
		case <-r.Ctx.Done():
			return
		}
	}
}
