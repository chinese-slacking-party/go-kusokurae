package gameserver

import (
	"context"
	"errors"
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
	cmdCh      chan GameCommand

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
	}
	r.Players[0] = host
	host.Sit(id, 0)
	roomRepository[id] = r

	// Start consumer goroutine for host's OperatorCh
	go func(player *Player) {
		for {
			select {
			case msg := <-player.OperatorCh:
				r.handleRoomMessage(player, msg)
			case <-r.Ctx.Done():
				return
			}
		}
	}(host)

	return r
}

func (r *Room) Game() *Game { return r.game.Load() }

func (r *Room) AddPlayer(player *Player) error {
	r.Mutex.Lock()
	defer r.Mutex.Unlock()
	if r.CurrentPlayers >= r.GameConfig.NumPlayers {
		return ErrRoomFull
	}
	position := r.CurrentPlayers
	r.Players[position] = player
	r.CurrentPlayers++
	player.Sit(r.ID, position)

	// Start per-player consumer for OperatorCh messages (e.g., START_GAME)
	go func(player *Player) {
		for {
			select {
			case msg := <-player.OperatorCh:
				r.handleRoomMessage(player, msg)
			case <-r.Ctx.Done():
				return
			}
		}
	}(player)

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
	for _, p := range r.Players {
		if p != nil && p.Session != nil {
			select {
			case p.NoticeCh <- msg:
			default:
			}
		}
	}
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

	go g.GameFn(context.Background())
	go r.gameEventLoop(g, players)

	return nil
}

func (r *Room) RoomFn(ctx context.Context) {
	// Deprecated: per-player consumer goroutines are now started in AddPlayer
	<-ctx.Done()
}

func (r *Room) handleRoomMessage(player *Player, msg Message) {
	switch msg.MsgType {
	case MSG_TYPE_START_GAME:
		if err := r.StartGame(player.ID); err != nil {
			select {
			case player.NoticeCh <- Message{
				MsgType: MSG_TYPE_ERROR,
				MsgBody: &ErrorBody{Message: err.Error()},
			}:
			default:
			}
		}

	case MSG_TYPE_PLAY_CARD:
		if g := r.game.Load(); g != nil {
			select {
			case g.CmdCh <- GameCommand{
				PlayerIdx: int(player.RoomPosition),
				Msg:       msg,
			}:
			case <-g.GameOver:
			case <-r.Ctx.Done():
			}
		}
	}
}

func (r *Room) gameEventLoop(g *Game, players []*Player) {
	var currentTurnIdx int = -1
	var playableIndices []int

	for {
		select {
		case event := <-g.EventCh:
			switch event.Msg.MsgType {
			case MSG_TYPE_YOUR_TURN:
				idx := event.Target
				body := event.Msg.MsgBody.(*YourTurnBody)
				if !r.isPlayerConnected(players[idx]) {
					r.autoPlayCard(g, idx, body.PlayableIndices)
				} else {
					r.sendToPlayer(players[idx], event.Msg)
					currentTurnIdx = idx
					playableIndices = body.PlayableIndices
				}

			default:
				if event.Target == -1 {
					r.broadcastToPlayers(players, event.Msg)
				} else {
					r.sendToPlayer(players[event.Target], event.Msg)
				}
			}

		case <-r.gameDisconnected(players, currentTurnIdx):
			if currentTurnIdx >= 0 && len(playableIndices) > 0 {
				r.autoPlayCard(g, currentTurnIdx, playableIndices)
				currentTurnIdx = -1
				playableIndices = nil
			}

		case <-g.GameOver:
			return
		case <-r.Ctx.Done():
			return
		}
	}
}

func (r *Room) sendToPlayer(p *Player, msg Message) {
	if p == nil || p.Session == nil {
		return
	}
	select {
	case p.NoticeCh <- msg:
	default:
	}
}

func (r *Room) broadcastToPlayers(players []*Player, msg Message) {
	for _, p := range players {
		r.sendToPlayer(p, msg)
	}
}

func (r *Room) isPlayerConnected(p *Player) bool {
	return p != nil && p.Session != nil
}

func (r *Room) gameDisconnected(players []*Player, idx int) chan struct{} {
	if idx < 0 || idx >= len(players) || players[idx] == nil {
		return nil
	}
	return players[idx].Disconnected
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

	switch msg.MsgType {
	case MSG_TYPE_START_GAME:
		if err := r.StartGame(p.ID); err != nil {
			select {
			case p.NoticeCh <- Message{
				MsgType: MSG_TYPE_ERROR,
				MsgBody: &ErrorBody{Message: err.Error()},
			}:
			default:
			}
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
		body := event.Msg.MsgBody.(*YourTurnBody)
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
	r.cmdCh = nil
	r.currentTurnIdx = -1
	r.playableIndices = nil
}

// UpdateDiscChannel updates the disconnect channel for a player seat.
// Called from the HTTP handler on reconnect after replacing player.Disconnected.
func (r *Room) UpdateDiscChannel(idx int, ch chan struct{}) {
	r.discChs[idx] = ch
}
