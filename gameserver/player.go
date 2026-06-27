package gameserver

import (
	"context"
	"log"
	"sync"

	"github.com/google/uuid"
)

type Player struct {
	ID            string
	RoomID        string
	RoomPosistion int32
	NoticeCh      chan Message
	OperatorCh    chan Message
	sync.Mutex
	joystick Joystick
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
		NoticeCh:      make(chan Message),
		OperatorCh:    make(chan Message),
		joystick:      nil,
	}
}

func (p *Player) Sit(roomID string, roomPosition int32) {
	p.RoomID = roomID
	p.RoomPosistion = roomPosition
}

func (p *Player) getJoystick() Joystick {
	p.Mutex.Lock()
	defer p.Mutex.Unlock()
	return p.joystick
}

func (p *Player) setJoystick(joystick Joystick) {
	p.Mutex.Lock()
	defer p.Mutex.Unlock()
	p.joystick = joystick
}

func (p *Player) WriteControlSignalFn(ctx context.Context) {
	for {
		msg := <-p.NoticeCh
		joystick := p.getJoystick()
		if joystick == nil {
			log.Println("discards msg: ", msg)
		} else {
			if err := p.joystick.WriteMessage(msg); err != nil {
				log.Println("write msg into joystick error: ", err.Error())
				p.setJoystick(nil)
			}
		}
	}
}

func (p *Player) ReadControlSignalFn(ctx context.Context) {
	for {
		joystick := p.getJoystick()
		msg, err := joystick.ReadMessage()
		if err != nil {
			log.Println("read msg from joystick error: ", err.Error())
			p.setJoystick(nil)
			break
		}
		p.OperatorCh <- msg
	}
}
