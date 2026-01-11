package gameserver

import (
	"errors"
	"sync"

	"github.com/bs-iron-trio/go-kusokurae/sm"
)

var ErrRoomFull = errors.New("room is full!!!")
var ErrRoomNotFound = errors.New("room not found!!!")

type Room struct {
	ID                string
	Mutex             sync.Mutex // protects the room state
	RoomGameConfig    *sm.GameConfig
	CurrentGame       *Game
	CurrentPlayers    int32
	Players           []*Player
	PlayerReadyStatus []*int32
}

func NewRoom(id string, config *sm.GameConfig) *Room {
	return &Room{
		ID:                id,
		Mutex:             sync.Mutex{},
		RoomGameConfig:    config,
		CurrentPlayers:    0,
		Players:           make([]*Player, config.NumPlayers),
		PlayerReadyStatus: make([]*int32, config.NumPlayers),
	}
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
