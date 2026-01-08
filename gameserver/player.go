package main

import "github.com/google/uuid"

type Player struct {
	ID            string
	RoomID        string
	RoomPosistion int32
}

func NewPlayer() *Player {
	u, err := uuid.NewRandom()
	if err != nil {
		panic("failed to generate player ID")
	}
	return &Player{
		ID:            u.String(),
		RoomID:        "",
		RoomPosistion: -1,
	}
}

func (p *Player) Sit(roomID string, roomPosition int32) {
	p.RoomID = roomID
	p.RoomPosistion = roomPosition
}
