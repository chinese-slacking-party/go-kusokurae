package gameserver

import (
	"testing"
	"time"

	"github.com/bs-iron-trio/go-kusokurae/sm"
	"github.com/stretchr/testify/assert"
)

func TestRun_RoutesPlayerMessage(t *testing.T) {
	InitRoomRepository()
	host, _ := NewPlayer("host")
	host.Session = &Session{} // needed so sendToPlayer doesn't skip
	config := &sm.GameConfig{NumPlayers: 3}
	_ = NewRoom("test-room", host, config)

	// Send START_GAME — should fail (not enough players) and send error to NoticeCh
	host.OperatorCh <- Message{MsgType: MSG_TYPE_START_GAME}

	select {
	case msg := <-host.NoticeCh:
		assert.Equal(t, MSG_TYPE_ERROR, msg.MsgType)
		errBody, ok := msg.MsgBody.(*ErrorBody)
		assert.True(t, ok)
		assert.Contains(t, errBody.Message, "not enough players")
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for error response")
	}
}

func TestRun_DisconnectHandlesAutoPlay(t *testing.T) {
	InitRoomRepository()
	host, _ := NewPlayer("host")
	config := &sm.GameConfig{NumPlayers: 2}
	room := NewRoom("test-room-dc", host, config)

	// Add second player
	p2, _ := NewPlayer("p2")
	err := room.AddPlayer(p2)
	assert.NoError(t, err)

	// Set up state on run() goroutine to avoid data races
	done := make(chan struct{})
	room.internalCh <- func() {
		close(p2.Disconnected)
		p2.Session = nil
		close(done)
	}
	<-done

	// run() should pick up the closed discChs[1] and call handleDisconnect
	// Since there's no active game, it should just nil out discChs[1]
	time.Sleep(50 * time.Millisecond)

	// Read discChs[1] safely on the run() goroutine
	var discChs1 chan struct{}
	readDone := make(chan struct{})
	room.internalCh <- func() {
		discChs1 = room.discChs[1]
		close(readDone)
	}
	<-readDone

	assert.Nil(t, discChs1)
}

func TestRun_GameEventRoutesToPlayers(t *testing.T) {
	InitRoomRepository()
	host, _ := NewPlayer("host")
	config := &sm.GameConfig{NumPlayers: 2}
	room := NewRoom("test-room-ev", host, config)

	// Manually wire game channels via internalCh to avoid data races
	eventCh := make(chan GameEvent, 1)
	gameEndCh := make(chan struct{})
	done := make(chan struct{})
	room.internalCh <- func() {
		room.eventCh = eventCh
		room.gameEndCh = gameEndCh
		close(done)
	}
	<-done

	// Send a broadcast event
	eventCh <- GameEvent{
		Target: -1,
		Msg:    Message{MsgType: MSG_TYPE_MOVE_MADE, MsgBody: &MoveMadeBody{PlayerIdx: 0}},
	}

	// Kick the select loop so it re-evaluates with the newly set channels
	host.OperatorCh <- Message{MsgType: MSG_TYPE_PLAY_CARD}

	time.Sleep(50 * time.Millisecond)

	// EventCh should be drained
	select {
	case <-eventCh:
		t.Fatal("event should have been consumed")
	default:
		// Expected: channel is empty
	}
}

func TestRun_GameOverClearsChannels(t *testing.T) {
	InitRoomRepository()
	host, _ := NewPlayer("host")
	config := &sm.GameConfig{NumPlayers: 2}
	room := NewRoom("test-room-go", host, config)

	// Set up game channels via internalCh
	done := make(chan struct{})
	room.internalCh <- func() {
		room.eventCh = make(chan GameEvent)
		room.gameEndCh = make(chan struct{})
		close(done)
	}
	<-done

	// Signal game end
	close(room.gameEndCh)

	// Kick run() to re-evaluate select
	host.OperatorCh <- Message{MsgType: MSG_TYPE_PLAY_CARD}

	time.Sleep(50 * time.Millisecond)

	// Read field values safely on the run() goroutine
	var (
		gotEventCh chan GameEvent
		gotEndCh   chan struct{}
	)
	syncDone := make(chan struct{})
	room.internalCh <- func() {
		gotEventCh = room.eventCh
		gotEndCh = room.gameEndCh
		close(syncDone)
	}
	<-syncDone

	assert.Nil(t, gotEventCh)
	assert.Nil(t, gotEndCh)
}

func TestRun_GameEndAllowsRestart(t *testing.T) {
	InitRoomRepository()
	host, _ := NewPlayer("host")
	config := &sm.GameConfig{NumPlayers: 2}
	room := NewRoom("test-room-restart", host, config)

	// Simulate a game that ended: wire channels then close the end channel
	done := make(chan struct{})
	room.internalCh <- func() {
		room.eventCh = make(chan GameEvent)
		room.gameEndCh = make(chan struct{})
		room.game.Store(NewGame(config, 2))
		close(done)
	}
	<-done

	close(room.gameEndCh)
	host.OperatorCh <- Message{MsgType: MSG_TYPE_PLAY_CARD}
	time.Sleep(50 * time.Millisecond)

	// After handleGameEnd, r.game must be released so a new game can start
	var gotGame *Game
	readDone := make(chan struct{})
	room.internalCh <- func() {
		gotGame = room.game.Load()
		close(readDone)
	}
	<-readDone

	assert.Nil(t, gotGame)
}

func TestRun_DisconnectStaleEventIgnored(t *testing.T) {
	InitRoomRepository()
	host, _ := NewPlayer("host")
	config := &sm.GameConfig{NumPlayers: 2}
	room := NewRoom("test-room-stale", host, config)

	// Simulate reconnect: new session exists, but old Disconnected fires
	oldCh := make(chan struct{})

	// Set discChs[0] and Session on run() goroutine to avoid data races
	done := make(chan struct{})
	room.internalCh <- func() {
		room.discChs[0] = oldCh
		host.Session = &Session{}
		close(done)
	}
	<-done

	close(oldCh) // old channel fires

	// Kick run() to re-evaluate select
	host.OperatorCh <- Message{MsgType: MSG_TYPE_PLAY_CARD}

	time.Sleep(50 * time.Millisecond)

	// discChs[0] should be nil'd (to prevent tight loop on closed channel)
	var discChs0 chan struct{}
	readDone := make(chan struct{})
	room.internalCh <- func() {
		discChs0 = room.discChs[0]
		close(readDone)
	}
	<-readDone

	assert.Nil(t, discChs0)
}
