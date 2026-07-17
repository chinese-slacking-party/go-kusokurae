package gameserver

import (
	"context"
	"errors"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"

	"github.com/bs-iron-trio/go-kusokurae/sm"
)

var ErrRoomFull = errors.New("room is full!!!")
var ErrRoomNotFound = errors.New("room not found!!!")
var ErrRoomPlayerNotFound = errors.New("this player not contained by room")
var ErrGameAlreadyStarted = errors.New("game already started")
var ErrNotEnoughPlayers = errors.New("not enough players")
var ErrNotHost = errors.New("only host can start game")

type Room struct {
	ID             string
	Ctx            context.Context
	cancel         context.CancelFunc
	Mutex          sync.Mutex
	GameConfig     *sm.GameConfig
	game           atomic.Pointer[Game]
	HostPlayerIdx  int32
	CurrentPlayers int32
	Players        []*Player

	// Slot arrays for run() select — nil means empty/inactive
	opChs      [4]chan Message
	discChs    [4]chan struct{}
	eventCh    chan GameEvent
	gameOverCh chan struct{}

	// Internal command channel for thread-safe slot array writes
	internalCh chan func()

	// Per-game state for disconnect auto-play
	currentTurnIdx  int
	playableIndices []int
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
	r.discChs[0] = host.Disconnected

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
	r.discChs[position] = player.Disconnected

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

func (r *Room) broadcastRoomState() {
	players := make([]RoomPlayerInfo, r.CurrentPlayers)
	for i := int32(0); i < r.CurrentPlayers; i++ {
		players[i] = RoomPlayerInfo{
			PlayerID: r.Players[i].ID,
			Position: i,
			IsHost:   i == r.HostPlayerIdx,
		}
	}
	r.Broadcast(Message{
		MsgType: MSG_TYPE_ROOM_STATE,
		MsgBody: &RoomStateBody{Players: players, HostIdx: r.HostPlayerIdx},
	})
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
	g := NewGame(r.GameConfig, int32(len(players)))
	r.game.Store(g)

	// Wire game channels into run() select
	r.eventCh = g.EventCh
	r.gameOverCh = g.GameOver

	go g.GameFn(context.Background())

	return nil
}

func (r *Room) sendToPlayer(p *Player, msg Message) {
	if p == nil || p.Session == nil {
		return
	}
	p.NoticeCh <- msg
	log.Printf("Room %s notice %s to %s\n", r.ID[:8], msg.MsgType, p.ID[:8])
}

func (r *Room) broadcastToPlayers(players []*Player, msg Message) {
	for _, p := range players {
		r.sendToPlayer(p, msg)
	}
}

func (r *Room) isPlayerConnected(p *Player) bool {
	return p != nil && p.Session != nil
}

func (r *Room) autoPlayCard(g *Game, idx int, playableIndices []int) {
	if len(playableIndices) == 0 {
		return
	}
	chosen := playableIndices[rand.Intn(len(playableIndices))]
	select {
	case g.CmdCh <- GameCommand{
		PlayerIdx: idx,
		Msg: Message{
			MsgType: MSG_TYPE_PLAY_CARD,
			MsgBody: &PlayCardBody{CardIndex: chosen},
		},
	}:
	case <-g.GameOver:
	}
}

func (r *Room) handlePlayerMessage(idx int, msg Message) {
	p := r.Players[idx]
	if p == nil {
		return
	}

	log.Printf("Room %s Player %s recv msg %s\n", r.ID[:8], p.ID[:8], msg.MsgType)

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
			case <-g.GameOver:
			case <-r.Ctx.Done():
			}
		}
	}
}

func (r *Room) handleGameEvent(event GameEvent) {
	switch event.Msg.MsgType {
	case MSG_TYPE_YOUR_TURN:
		idx := event.Target
		body, ok := event.Msg.MsgBody.(*YourTurnBody)
		if !ok {
			return
		}
		if !r.isPlayerConnected(r.Players[idx]) {
			if g := r.game.Load(); g != nil {
				r.autoPlayCard(g, idx, body.PlayableIndices)
			}
		} else {
			r.sendToPlayer(r.Players[idx], event.Msg)
			r.currentTurnIdx = idx
			r.playableIndices = body.PlayableIndices
		}

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

	log.Printf("Player %s disconnect\n", p.ID[:8])

	// Nil out the fired channel immediately to prevent tight loop.
	// A closed channel fires on every select iteration, so we must
	// remove it before any early-return path.
	r.discChs[idx] = nil

	// Stale close event from a replaced Disconnected channel (player reconnected).
	// UpdateDiscChannel will set the new channel after this returns.
	if p.Session != nil {
		return
	}

	// Auto-play if it's this disconnected player's turn
	if r.currentTurnIdx == idx && len(r.playableIndices) > 0 {
		if g := r.game.Load(); g != nil {
			r.autoPlayCard(g, idx, r.playableIndices)
		}
		r.currentTurnIdx = -1
		r.playableIndices = nil
	}
}

func (r *Room) handleGameOver() {
	r.eventCh = nil
	r.gameOverCh = nil
	r.currentTurnIdx = -1
	r.playableIndices = nil
}

// UpdateDiscChannel updates the disconnect channel for a player seat.
// Called from the HTTP handler on reconnect after replacing player.Disconnected.
func (r *Room) UpdateDiscChannel(idx int, ch chan struct{}) {
	r.internalCh <- func() {
		r.discChs[idx] = ch
	}
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
		case <-r.gameOverCh:
			r.handleGameOver()
		case fn := <-r.internalCh:
			fn()
		case <-r.Ctx.Done():
			return
		}
	}
}
