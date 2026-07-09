package gameserver

import (
	"context"
	"errors"
	"sync"

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
	Mutex          sync.Mutex
	GameConfig     *sm.GameConfig
	Game           *Game
	HostPlayerIdx  int32
	CurrentPlayers int32
	Players        []*Player
}

var roomRepositoryMu sync.Mutex
var roomRepository map[string]*Room

func InitRoomRepository() {
	roomRepository = make(map[string]*Room)
}

func GetRoomByID(roomID string) (*Room, error) {
	room, exists := roomRepository[roomID]
	if !exists {
		return nil, ErrRoomNotFound
	}
	return room, nil
}

func NewRoom(id string, host *Player, config *sm.GameConfig) *Room {
	roomRepositoryMu.Lock()
	defer roomRepositoryMu.Unlock()
	r := &Room{
		ID:             id,
		GameConfig:     config,
		HostPlayerIdx:  0,
		CurrentPlayers: 1,
		Players:        make([]*Player, config.NumPlayers),
	}
	r.Players[0] = host
	host.Sit(id, 0)
	roomRepository[id] = r
	return r
}

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

	if r.Game != nil {
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
	r.Game = NewGame(r.GameConfig, players)
	go r.Game.GameFn(context.Background())
	return nil
}

func (r *Room) RoomFn(ctx context.Context) {
	for _, p := range r.Players {
		if p == nil {
			continue
		}
		go func(player *Player) {
			for {
				select {
				case msg := <-player.OperatorCh:
					r.handleRoomMessage(player, msg)
				case <-ctx.Done():
					return
				}
			}
		}(p)
	}
	<-ctx.Done()
}

func (r *Room) handleRoomMessage(player *Player, msg Message) {
	switch msg.MsgType {
	case MSG_TYPE_START_GAME:
		if err := r.StartGame(player.ID); err != nil {
			player.NoticeCh <- Message{
				MsgType: MSG_TYPE_ERROR,
				MsgBody: &ErrorBody{Message: err.Error()},
			}
		}
	}
}
