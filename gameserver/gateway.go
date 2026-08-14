package gameserver

import (
	"context"
	"sync"

	"github.com/gorilla/websocket"
)

// Session owns a single WebSocket connection and its player's I/O goroutines.
// The room tracks connection state via Session.discCh (closed once on detach),
// so Player no longer carries a mutable Session pointer.
type Session struct {
	Conn   *websocket.Conn
	Player *Player

	ctx    context.Context
	cancel context.CancelFunc

	discCh     chan struct{}
	detachOnce sync.Once
	ClosedCh   chan struct{}
	inputDone  chan struct{}
	outputDone chan struct{}
}

func NewSession(conn *websocket.Conn, player *Player) *Session {
	ctx, cancel := context.WithCancel(context.Background())
	return &Session{
		Conn:       conn,
		Player:     player,
		ctx:        ctx,
		cancel:     cancel,
		discCh:     make(chan struct{}),
		ClosedCh:   make(chan struct{}),
		inputDone:  make(chan struct{}),
		outputDone: make(chan struct{}),
	}
}

// DiscCh returns the channel closed exactly once when this session detaches
// (read error, write error, or Close). The room selects on the CURRENT
// session's channel only, so a stale session's close is naturally ignored.
func (s *Session) DiscCh() <-chan struct{} {
	return s.discCh
}

// detach marks the session as gone, exactly once.
func (s *Session) detach() {
	s.detachOnce.Do(func() {
		close(s.discCh)
	})
}

// Detach marks the session as gone (public for the room tests / lifecycle).
func (s *Session) Detach() {
	s.detach()
}

// Close shuts the connection down and cancels the session context so the
// I/O goroutines (e.g. Output blocked on NoticeCh) stop promptly.
func (s *Session) Close() {
	if s.Conn != nil {
		s.Conn.Close()
	}
	s.cancel()
}

func (s *Session) Input() {
	defer func() {
		close(s.inputDone)
	}()

	var msg Message
	for {
		if err := s.Conn.ReadJSON(&msg); err != nil {
			s.detach()
			return
		}
		select {
		case s.Player.OperatorCh <- msg:
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *Session) Output() {
	defer func() {
		close(s.outputDone)
	}()

	for {
		select {
		case msg := <-s.Player.NoticeCh:
			if err := s.Conn.WriteJSON(&msg); err != nil {
				s.detach()
				return
			}
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *Session) SessionControl() {
	<-s.inputDone
	<-s.outputDone
	close(s.ClosedCh)
}
