package gameserver

import (
	"context"
	"testing"
	"time"

	"github.com/bs-iron-trio/go-kusokurae/sm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestGame creates a started game with a custom turn timeout, with GameFn
// running in the background. Events must be drained from g.EventCh by the test.
func newTestGame(t *testing.T, timeout time.Duration) (*Game, context.CancelFunc) {
	cfg := &sm.GameConfig{NumPlayers: 3}
	g := NewGame(cfg, 3)
	g.TurnTimeout = timeout
	var err error
	g.State, err = sm.NewGame(*cfg, nil)
	require.NoError(t, err)
	require.NoError(t, g.State.Start())
	ctx, cancel := context.WithCancel(context.Background())
	go g.GameFn(ctx)
	return g, cancel
}

// waitEvent reads events until one with the given type arrives.
func waitEvent(t *testing.T, g *Game, msgType string) GameEvent {
	deadline := time.After(3 * time.Second)
	for {
		select {
		case ev := <-g.EventCh:
			if ev.Msg.MsgType == msgType {
				return ev
			}
		case <-deadline:
			t.Fatalf("timeout waiting for %s event", msgType)
		}
	}
}

func TestGameFn_TurnTimeoutAutoPlay(t *testing.T) {
	g, cancel := newTestGame(t, 100*time.Millisecond)
	defer cancel()

	ev := waitEvent(t, g, MSG_TYPE_MOVE_MADE)
	body, ok := ev.Msg.MsgBody.(*MoveMadeBody)
	require.True(t, ok)
	assert.True(t, body.AutoPlay, "timeout move should be flagged as auto-play")
}

func TestGameFn_InvalidMoveResendsYourTurn(t *testing.T) {
	g, cancel := newTestGame(t, 5*time.Second)
	defer cancel()

	// Wait for the first YOUR_TURN of the game
	waitEvent(t, g, MSG_TYPE_YOUR_TURN)

	// Send an out-of-range play from the active player
	g.CmdCh <- GameCommand{
		PlayerIdx: 0,
		Msg:       Message{MsgType: MSG_TYPE_PLAY_CARD, MsgBody: map[string]interface{}{"card_index": float64(999)}},
	}

	// Expect ERROR followed by a re-sent YOUR_TURN (same turn, new prompt)
	deadline := time.After(3 * time.Second)
	gotErr := false
	for {
		select {
		case ev := <-g.EventCh:
			switch ev.Msg.MsgType {
			case MSG_TYPE_ERROR:
				gotErr = true
			case MSG_TYPE_YOUR_TURN:
				if gotErr {
					return
				}
			}
		case <-deadline:
			t.Fatal("timeout: expected ERROR then re-sent YOUR_TURN")
		}
	}
}

func TestGameFn_InvalidMovesDoNotExtendDeadline(t *testing.T) {
	g, cancel := newTestGame(t, 200*time.Millisecond)
	defer cancel()

	// Continuously send invalid moves for ~900ms. With the deadline anchored to
	// turn start, auto-play still fires at ~200ms. If invalid moves reset the
	// timer, the turn would be extended indefinitely and no auto-play arrives
	// within the window.
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		cmd := GameCommand{
			PlayerIdx: 0,
			Msg:       Message{MsgType: MSG_TYPE_PLAY_CARD, MsgBody: map[string]interface{}{"card_index": float64(999)}},
		}
		for i := 0; i < 18; i++ {
			select {
			case <-stop:
				return
			case <-ticker.C:
				select {
				case g.CmdCh <- cmd:
				case <-stop:
					return
				}
			}
		}
	}()

	ev := waitEvent(t, g, MSG_TYPE_MOVE_MADE)
	body, ok := ev.Msg.MsgBody.(*MoveMadeBody)
	require.True(t, ok)
	assert.True(t, body.AutoPlay)
}
