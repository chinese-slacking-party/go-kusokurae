package gameserver

import (
	"context"
	"testing"
	"time"

	"github.com/bs-iron-trio/go-kusokurae/sm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestGame creates a started game with a custom turn timeout and sync
// interval, with GameFn running in the background. Events must be drained from
// g.EventCh by the test.
func newTestGame(t *testing.T, timeout, syncInterval time.Duration) (*Game, context.CancelFunc) {
	cfg := &sm.GameConfig{NumPlayers: 3}
	g := NewGame(cfg, 3)
	g.TurnTimeout = timeout
	g.TurnSyncInterval = syncInterval
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
	g, cancel := newTestGame(t, 100*time.Millisecond, 0)
	defer cancel()

	ev := waitEvent(t, g, MSG_TYPE_MOVE_MADE)
	body, ok := ev.Msg.MsgBody.(*MoveMadeBody)
	require.True(t, ok)
	assert.True(t, body.AutoPlay, "timeout move should be flagged as auto-play")
	// Auto-play must carry the hand index of the played card
	assert.GreaterOrEqual(t, int(body.CardIdx), 0)
}

func TestGameFn_InvalidMoveResendsYourTurn(t *testing.T) {
	g, cancel := newTestGame(t, 5*time.Second, 0)
	defer cancel()

	// Wait for the first YOUR_TURN of the game
	waitEvent(t, g, MSG_TYPE_YOUR_TURN)

	// Send an out-of-range play from the active player
	g.CmdCh <- GameCommand{
		PlayerIdx: 0,
		Msg:       Message{MsgType: MSG_TYPE_PLAY_CARD, MsgBody: map[string]interface{}{"card_index": float64(999)}},
	}

	// Expect ERROR, then a fresh YOUR_TURN carrying the true remaining time
	// (not a full restart of the countdown).
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
					body, ok := ev.Msg.MsgBody.(*YourTurnBody)
					require.True(t, ok)
					assert.Greater(t, body.RemainingSeconds, 0)
					assert.LessOrEqual(t, body.RemainingSeconds, body.TimeoutSeconds)
					return
				}
			}
		case <-deadline:
			t.Fatal("timeout: expected ERROR then re-sent YOUR_TURN")
		}
	}
}

func TestGameFn_InvalidMovesDoNotExtendDeadline(t *testing.T) {
	g, cancel := newTestGame(t, 200*time.Millisecond, 0)
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

func TestGameFn_TurnTimeSyncTicks(t *testing.T) {
	g, cancel := newTestGame(t, 300*time.Millisecond, 50*time.Millisecond)
	defer cancel()

	// YOUR_TURN must carry remaining_seconds
	ev := waitEvent(t, g, MSG_TYPE_YOUR_TURN)
	yt, ok := ev.Msg.MsgBody.(*YourTurnBody)
	require.True(t, ok)
	assert.Greater(t, yt.RemainingSeconds, 0)

	// TURN_TIME_SYNC ticks must arrive at the active player before auto-play
	deadline := time.After(2 * time.Second)
	for {
		select {
		case ev := <-g.EventCh:
			switch ev.Msg.MsgType {
			case MSG_TYPE_TURN_TIME_SYNC:
				body, ok := ev.Msg.MsgBody.(*TurnTimeSyncBody)
				require.True(t, ok)
				assert.Greater(t, body.RemainingSeconds, 0)
				assert.GreaterOrEqual(t, ev.Target, 0)
				assert.Equal(t, yt.RoundSeq, body.RoundSeq)
				return
			case MSG_TYPE_MOVE_MADE:
				t.Fatal("auto-play fired before any TURN_TIME_SYNC tick")
			}
		case <-deadline:
			t.Fatal("timeout: no TURN_TIME_SYNC received")
		}
	}
}

func TestGameFn_MoveMadeCarriesCardIdx(t *testing.T) {
	g, cancel := newTestGame(t, 5*time.Second, 0)
	defer cancel()

	// Wait for YOUR_TURN, then play the highest-indexed playable card
	ev := waitEvent(t, g, MSG_TYPE_YOUR_TURN)
	yt, ok := ev.Msg.MsgBody.(*YourTurnBody)
	require.True(t, ok)
	require.NotEmpty(t, yt.PlayableIndices)
	wantIdx := yt.PlayableIndices[0]
	for _, i := range yt.PlayableIndices {
		if i > wantIdx {
			wantIdx = i
		}
	}

	g.CmdCh <- GameCommand{
		PlayerIdx: 0,
		Msg:       Message{MsgType: MSG_TYPE_PLAY_CARD, MsgBody: map[string]interface{}{"card_index": float64(wantIdx)}},
	}

	ev = waitEvent(t, g, MSG_TYPE_MOVE_MADE)
	mm, ok := ev.Msg.MsgBody.(*MoveMadeBody)
	require.True(t, ok)
	assert.Equal(t, int32(wantIdx), mm.CardIdx)
}

func TestGameFn_PanicEmitsFatal(t *testing.T) {
	cfg := &sm.GameConfig{NumPlayers: 3}
	g := NewGame(cfg, 3)
	g.panicHook = func() { panic("boom") }
	var err error
	g.State, err = sm.NewGame(*cfg, nil)
	require.NoError(t, err)
	require.NoError(t, g.State.Start())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go g.GameFn(ctx)

	ev := waitEvent(t, g, MSG_TYPE_GAME_FATAL)
	fb, ok := ev.Msg.MsgBody.(*GameFatalBody)
	require.True(t, ok)
	assert.Contains(t, fb.Message, "boom")
	assert.Equal(t, -1, ev.Target, "GAME_FATAL must be broadcast")

	select {
	case <-g.GameEnd:
	case <-time.After(2 * time.Second):
		t.Fatal("GameEnd not closed after fatal")
	}
}

func TestGameFn_ResyncEmitsGameState(t *testing.T) {
	g, cancel := newTestGame(t, 5*time.Second, 0)
	defer cancel()

	// Ask the game goroutine for a resync snapshot for player 0
	g.ResyncCh <- 0

	ev := waitEvent(t, g, MSG_TYPE_GAME_RESYNC)
	body, ok := ev.Msg.MsgBody.(*GameResyncBody)
	require.True(t, ok)
	assert.Equal(t, int32(sm.StatusPlay), body.Status)
	assert.NotEmpty(t, body.HandCards, "resyncing player must receive hand cards")
	assert.GreaterOrEqual(t, body.RoundSeq, 1)
	assert.Len(t, body.Scores, 3)
	assert.Equal(t, int32(0), body.ActivePlayerIdx, "first leader is player 0")
	assert.NotEmpty(t, body.PlayableIndices, "leader has playable cards")
	assert.Greater(t, body.RemainingSeconds, 0)
	assert.Equal(t, 0, ev.Target, "game resync event targets the requesting player")
}
