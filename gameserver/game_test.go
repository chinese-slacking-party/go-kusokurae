package gameserver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCmdCh_RoutesMessage(t *testing.T) {
	g := &Game{
		ID:         "test-game-01",
		NumPlayers: 3,
		CmdCh:      make(chan GameCommand, 1),
	}

	cmd := GameCommand{
		PlayerIdx: 0,
		Msg:       Message{MsgType: MSG_TYPE_PLAY_CARD, MsgBody: &PlayCardBody{CardIndex: 2}},
	}
	g.CmdCh <- cmd

	select {
	case received := <-g.CmdCh:
		assert.Equal(t, 0, received.PlayerIdx)
		assert.Equal(t, MSG_TYPE_PLAY_CARD, received.Msg.MsgType)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for command on CmdCh")
	}
}

func TestEventCh_EmitsEvent(t *testing.T) {
	g := &Game{
		ID:         "test-game-02",
		NumPlayers: 2,
		EventCh:    make(chan GameEvent, 1),
	}

	g.emit(0, Message{MsgType: MSG_TYPE_YOUR_TURN, MsgBody: &YourTurnBody{PlayableIndices: []int{0, 1}}})

	select {
	case event := <-g.EventCh:
		assert.Equal(t, 0, event.Target)
		assert.Equal(t, MSG_TYPE_YOUR_TURN, event.Msg.MsgType)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event on EventCh")
	}
}

func TestEventCh_BroadcastTarget(t *testing.T) {
	g := &Game{
		ID:         "test-game-03",
		NumPlayers: 3,
		EventCh:    make(chan GameEvent, 1),
	}

	g.emit(-1, Message{MsgType: MSG_TYPE_MOVE_MADE})

	select {
	case event := <-g.EventCh:
		assert.Equal(t, -1, event.Target)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for broadcast event")
	}
}

func TestValidateTurnTimeoutSec(t *testing.T) {
	assert.NoError(t, ValidateTurnTimeoutSec(0))
	assert.NoError(t, ValidateTurnTimeoutSec(5))
	assert.NoError(t, ValidateTurnTimeoutSec(120))
	assert.Error(t, ValidateTurnTimeoutSec(4))
	assert.Error(t, ValidateTurnTimeoutSec(121))
}

func TestValidateTurnSyncIntervalSec(t *testing.T) {
	// 0 = default, always valid
	assert.NoError(t, ValidateTurnSyncIntervalSec(0, 0))
	assert.NoError(t, ValidateTurnSyncIntervalSec(0, 30))
	// valid: within 1..60 and strictly less than turn timeout
	assert.NoError(t, ValidateTurnSyncIntervalSec(5, 30))
	assert.NoError(t, ValidateTurnSyncIntervalSec(1, 5))
	assert.NoError(t, ValidateTurnSyncIntervalSec(60, 120))
	// turn timeout 0 means default 30
	assert.NoError(t, ValidateTurnSyncIntervalSec(29, 0))
	// must be strictly less than turn timeout
	assert.Error(t, ValidateTurnSyncIntervalSec(30, 30))
	assert.Error(t, ValidateTurnSyncIntervalSec(5, 5))
	assert.Error(t, ValidateTurnSyncIntervalSec(30, 0))
	// range violations
	assert.Error(t, ValidateTurnSyncIntervalSec(61, 120))
	assert.Error(t, ValidateTurnSyncIntervalSec(-1, 30))
}
