package gameserver

import (
	"testing"
	"time"

	"github.com/bs-iron-trio/go-kusokurae/sm"
	"github.com/stretchr/testify/assert"
)

func TestRun_RoutesPlayerMessage(t *testing.T) {
	InitRoomRepository()
	host := NewPlayer()
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
	host := NewPlayer()
	config := &sm.GameConfig{NumPlayers: 2}
	room := NewRoom("test-room-dc", host, config)

	// Add second player
	p2 := NewPlayer()
	err := room.AddPlayer(p2)
	assert.NoError(t, err)

	// Set up a fake disconnect: close player's Disconnected channel
	// First, simulate what Session does on error
	close(p2.Disconnected)
	p2.Session = nil

	// run() should pick up the closed discChs[1] and call handleDisconnect
	// Since there's no active game, it should just nil out discChs[1]
	time.Sleep(50 * time.Millisecond)

	// discChs[1] should be nil'd to prevent repeated firing
	assert.Nil(t, room.discChs[1])
}

func TestRun_GameEventRoutesToPlayers(t *testing.T) {
	InitRoomRepository()
	host := NewPlayer()
	config := &sm.GameConfig{NumPlayers: 2}
	room := NewRoom("test-room-ev", host, config)

	// Manually wire game channels to simulate active game
	eventCh := make(chan GameEvent, 1)
	gameOverCh := make(chan struct{})
	room.eventCh = eventCh
	room.gameOverCh = gameOverCh

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
	host := NewPlayer()
	config := &sm.GameConfig{NumPlayers: 2}
	room := NewRoom("test-room-go", host, config)

	room.eventCh = make(chan GameEvent)
	room.gameOverCh = make(chan struct{})
	room.cmdCh = make(chan GameCommand, 1)

	// Signal game over
	close(room.gameOverCh)

	// Kick run() to re-evaluate select (race prevention)
	host.OperatorCh <- Message{MsgType: MSG_TYPE_PLAY_CARD}

	time.Sleep(50 * time.Millisecond)

	assert.Nil(t, room.eventCh)
	assert.Nil(t, room.gameOverCh)
	assert.Nil(t, room.cmdCh)
	assert.Equal(t, -1, room.currentTurnIdx)
	assert.Nil(t, room.playableIndices)
}

func TestRun_DisconnectStaleEventIgnored(t *testing.T) {
	InitRoomRepository()
	host := NewPlayer()
	config := &sm.GameConfig{NumPlayers: 2}
	room := NewRoom("test-room-stale", host, config)

	// Simulate reconnect: new session exists, but old Disconnected fires
	oldCh := make(chan struct{})
	room.discChs[0] = oldCh
	host.Session = &Session{} // non-nil session = connected

	close(oldCh) // old channel fires

	// Kick run() to re-evaluate select (race prevention)
	host.OperatorCh <- Message{MsgType: MSG_TYPE_PLAY_CARD}

	time.Sleep(50 * time.Millisecond)

	// discChs[0] should be nil'd (to prevent tight loop on closed channel)
	// Stale guard returns early, but nil-out happens first
	assert.Nil(t, room.discChs[0])
}
