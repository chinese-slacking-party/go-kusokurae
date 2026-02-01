package gameserver

import (
	"context"
	"log"

	"github.com/gorilla/websocket"
)

type Session struct {
	Conn               *websocket.Conn
	Player             *Player
	ColsedCh           chan struct{}
	inputStreamClosed  chan struct{}
	outputStreamClosed chan struct{}
}

func NewSession(conn *websocket.Conn, player *Player) (s *Session) {
	return &Session{
		Conn:               conn,
		Player:             player,
		ColsedCh:           make(chan struct{}),
		inputStreamClosed:  make(chan struct{}),
		outputStreamClosed: make(chan struct{}),
	}
}

func (s *Session) Input(ctx context.Context) {

	defer func() {
		s.inputStreamClosed <- struct{}{}
		close(s.inputStreamClosed)
	}()

	defer func() {
		if p := recover(); p != nil {
			log.Printf("Session input error: %s", p)
		}
	}()

	var msg Message
	for {
		if err := s.Conn.ReadJSON(&msg); err != nil {
			log.Panicf("读取消息失败: %s", err)
			return
		}
		s.Player.OperatorCh <- msg
	}

}

func (s *Session) Output(ctx context.Context) {

	defer func() {
		s.outputStreamClosed <- struct{}{}
		close(s.outputStreamClosed)
	}()

	defer func() {
		if p := recover(); p != nil {
			log.Printf("Session input error: %s", p)
		}
	}()

	for {
		msg := <-s.Player.NoticeCh
		if err := s.Conn.WriteJSON(&msg); err != nil {
			log.Panicf("写入消息失败: %s", err)
			return
		}
	}

}

func (s *Session) SessionControl(ctx context.Context) {
	<-s.inputStreamClosed
	<-s.outputStreamClosed
	s.ColsedCh <- struct{}{}
	close(s.ColsedCh)
}
