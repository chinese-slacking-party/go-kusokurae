package gameserver

import (
	"github.com/google/uuid"
)

type Player struct {
	ID           string
	RoomID       string
	RoomPosition int32
	NoticeCh     chan Message
	OperatorCh   chan Message
	Disconnected chan struct{}
	Session      *Session
}

func NewPlayer() *Player {
	u, err := uuid.NewRandom()
	if err != nil {
		panic("failed to generate player ID")
	}
	return &Player{
		ID:           u.String(),
		RoomID:       "",
		RoomPosition: -1,
		NoticeCh:     make(chan Message),
		OperatorCh:   make(chan Message),
		Disconnected: make(chan struct{}),
	}
}

func (p *Player) Sit(roomID string, roomPosition int32) {
	p.RoomID = roomID
	p.RoomPosition = roomPosition
}
