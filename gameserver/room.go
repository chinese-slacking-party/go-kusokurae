package gameserver

import (
	"errors"
	"sync"

	"github.com/bs-iron-trio/go-kusokurae/sm"
)

var ErrRoomFull = errors.New("room is full!!!")
var ErrRoomNotFound = errors.New("room not found!!!")
var ErrRoomPlayerNotFound = errors.New("this player not contained by room")

type Room struct {
	ID                string
	Mutex             sync.Mutex // protects the room state
	RoomGameConfig    *sm.GameConfig
	CurrentGame       *Game
	CurrentPlayers    int32
	Players           []*Player
	PlayerReadyStatus []bool
}

var roomRepositoryMu sync.Mutex
var roomRepository map[string]*Room

func InitRoomRepository() {
	roomRepository = make(map[string]*Room)
}

func GetRoomByID(roomID string) (*Room, error) {
	room, exists := roomRepository[roomID]
	if !exists {
		return nil, ErrRoomFull
	} else {
		return room, nil
	}
}

func NewRoom(id string, config *sm.GameConfig) *Room {
	roomRepositoryMu.Lock()
	defer roomRepositoryMu.Unlock()
	r := &Room{
		ID:                id,
		Mutex:             sync.Mutex{},
		RoomGameConfig:    config,
		CurrentPlayers:    0,
		Players:           make([]*Player, config.NumPlayers),
		PlayerReadyStatus: make([]bool, config.NumPlayers),
	}
	roomRepository[id] = r
	return r
}

func (r *Room) AddPlayer(player *Player) error {
	r.Mutex.Lock()
	defer r.Mutex.Unlock()
	if r.CurrentPlayers >= r.RoomGameConfig.NumPlayers {
		return ErrRoomFull
	}
	r.Players[r.CurrentPlayers] = player
	r.CurrentPlayers++
	player.Sit(r.ID, r.CurrentPlayers-1)
	return nil
}

func (r *Room) FindPlayerByID(playerID string) (*Player, error) {
	for _, p := range r.Players {
		if p.ID == playerID {
			return p, nil
		}
	}
	return nil, ErrRoomPlayerNotFound
}

func (r *Room) Ready(playerID string, ReadyStatus bool) error {
	r.Mutex.Lock()
	defer r.Mutex.Unlock()
	for i, p := range r.Players {
		if p.ID == playerID {
			r.PlayerReadyStatus[i] = ReadyStatus
			return nil
		}
	}
	return ErrRoomPlayerNotFound

}
