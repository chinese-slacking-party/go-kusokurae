package gameserver

import (
	"context"

	"github.com/gorilla/websocket"
)

type Session struct {
	Conn               *websocket.Conn
	Player             *Player
	ClosedCh           chan struct{}
	inputStreamClosed  chan struct{}
	outputStreamClosed chan struct{}
}

func NewSession(conn *websocket.Conn, player *Player) *Session {
	s := &Session{
		Conn:               conn,
		Player:             player,
		ClosedCh:           make(chan struct{}),
		inputStreamClosed:  make(chan struct{}),
		outputStreamClosed: make(chan struct{}),
	}
	player.Session = s
	return s
}

func (s *Session) Input(ctx context.Context) {
	defer func() {
		s.inputStreamClosed <- struct{}{}
		close(s.inputStreamClosed)
	}()

	var msg Message
	for {
		if err := s.Conn.ReadJSON(&msg); err != nil {
			close(s.Player.Disconnected)
			s.Player.Session = nil
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

	for {
		select {
		case msg := <-s.Player.NoticeCh:
			if err := s.Conn.WriteJSON(&msg); err != nil {
				close(s.Player.Disconnected)
				s.Player.Session = nil
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (s *Session) SessionControl(ctx context.Context) {
	<-s.inputStreamClosed
	<-s.outputStreamClosed
	s.ClosedCh <- struct{}{}
	close(s.ClosedCh)
}
