package gameserver

import (
	"testing"
	"time"

	"github.com/bs-iron-trio/go-kusokurae/sm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun_RoutesPlayerMessage(t *testing.T) {
	InitRoomRepository()
	host, _ := NewPlayer("host")
	config := &sm.GameConfig{NumPlayers: 3}
	room := NewRoom("test-room", host, config)
	// Attach a session so sendToPlayer delivers (conn is nil; not used here)
	room.AttachSession(0, NewSession(nil, host))

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

func TestRun_DisconnectClearsChannel(t *testing.T) {
	InitRoomRepository()
	host, _ := NewPlayer("host")
	config := &sm.GameConfig{NumPlayers: 2}
	room := NewRoom("test-room-dc", host, config)

	// Add second player
	p2, _ := NewPlayer("p2")
	err := room.AddPlayer(p2)
	assert.NoError(t, err)

	// Set up state on run() goroutine to avoid data races
	room.AttachSession(1, NewSession(nil, p2))
	done := make(chan struct{})
	room.internalCh <- func() {
		close(done)
	}
	<-done

	// Detach p2's session: run() picks up the closed discChs[1]
	room.sessions[1].Detach()
	time.Sleep(50 * time.Millisecond)

	// Read discChs[1] safely on the run() goroutine
	var discChs1 <-chan struct{}
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

	// Simulate reconnect: session s1 is replaced by s2, then s1 detaches late
	s1 := NewSession(nil, host)
	room.AttachSession(0, s1)
	s2 := NewSession(nil, host)
	old := room.AttachSession(0, s2)
	assert.Equal(t, s1, old, "AttachSession returns the replaced session")

	// Stale session detaches: its channel is no longer in the select, so the
	// room must keep the current session intact.
	s1.Detach()
	host.OperatorCh <- Message{MsgType: MSG_TYPE_PLAY_CARD} // kick the select
	time.Sleep(50 * time.Millisecond)

	var gotSession *Session
	var gotDiscCh <-chan struct{}
	readDone := make(chan struct{})
	room.internalCh <- func() {
		gotSession = room.sessions[0]
		gotDiscCh = room.discChs[0]
		close(readDone)
	}
	<-readDone

	assert.Equal(t, s2, gotSession, "stale detach must not clear the current session")
	assert.NotNil(t, gotDiscCh, "current session's disc channel must remain selected")
}

func TestRun_ResyncStateNoGame(t *testing.T) {
	InitRoomRepository()
	host, _ := NewPlayer("host")
	config := &sm.GameConfig{NumPlayers: 3}
	room := NewRoom("test-room-resync", host, config)
	room.AttachSession(0, NewSession(nil, host))

	// Player requests state while no game is running: reply with game:null
	host.OperatorCh <- Message{MsgType: MSG_TYPE_RESYNC_STATE}

	select {
	case msg := <-host.NoticeCh:
		assert.Equal(t, MSG_TYPE_RESYNC_STATE, msg.MsgType)
		body, ok := msg.MsgBody.(*ResyncStateBody)
		require.True(t, ok)
		assert.Nil(t, body.Room.Game, "game must be null when no game is in progress")
		assert.Equal(t, int32(0), body.Room.HostIdx)
		assert.Len(t, body.Room.Players, 1)
		assert.Equal(t, host.ID, body.Room.Players[0].PlayerID)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for RESYNC_STATE reply")
	}
}

func TestRoom_StartPlayerRotation(t *testing.T) {
	room := &Room{nextStartPlayer: 0, GameConfig: &sm.GameConfig{NumPlayers: 3}}
	assert.Equal(t, int32(0), room.startPlayerForNextGame())
	assert.Equal(t, int32(1), room.startPlayerForNextGame())
	assert.Equal(t, int32(2), room.startPlayerForNextGame())
	assert.Equal(t, int32(0), room.startPlayerForNextGame(), "rotation wraps after 3 games")
}
