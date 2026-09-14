package gameserver

import (
	"testing"
	"time"

	"github.com/bs-iron-trio/go-kusokurae/sm"
	"github.com/stretchr/testify/require"
)

// destroyedRoom builds a room, has the host leave to trigger teardown, and
// returns once run()'s context is actually cancelled.
func destroyedRoom(t *testing.T, id string) *Room {
	t.Helper()
	InitRoomRepository()

	host, err := NewPlayer("host")
	require.NoError(t, err)
	r := NewRoom(id, host, &sm.GameConfig{NumPlayers: 3})
	r.AttachSession(0, NewSession(nil, host))

	// The host leaving takes run() into destroyInternal, whose cancel() stops
	// it from ever returning to the select.
	host.OperatorCh <- Message{MsgType: MSG_TYPE_LEAVE_ROOM}
	select {
	case <-r.Ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("room was not destroyed in time")
	}
	return r
}

// TestPostDestroyCallsDoNotHang covers calling into a room that is already
// gone.
//
// Destroy, AttachSession and AddPlayer share one shape: post a closure to the
// unbuffered internalCh, then block for the result. After teardown run() has
// exited, so nothing receives from internalCh, the send blocks forever and the
// calling goroutine leaks. destroyOnce does not help -- that sync.Once sits
// inside the closure that never runs.
//
// What reaches this is a TOCTOU window: a request goroutine looks the room up
// in the repository, run() destroys it because the host left, and only then
// does the request goroutine call AttachSession. A reconnect hits that window
// easily, and both AttachSession and AddPlayer are called from HTTP/WebSocket
// request goroutines (controller.go:58 and :147).
//
// Destroy has no caller today, but its comment says it may be called from any
// goroutine, and calling it from run()'s own goroutine deadlocks too: the
// unbuffered send blocks while the only possible receiver is stuck inside a
// handler.
//
// Expected to fail until the fix lands; each subtest sits on its timeout for
// two seconds.
func TestPostDestroyCallsDoNotHang(t *testing.T) {
	cases := []struct {
		name string
		call func(*Room)
	}{
		{"Destroy", func(r *Room) {
			r.Destroy("destroyed again")
		}},
		{"AttachSession", func(r *Room) {
			p, _ := NewPlayer("late")
			r.AttachSession(1, NewSession(nil, p))
		}},
		{"AddPlayer", func(r *Room) {
			p, _ := NewPlayer("late")
			_ = r.AddPlayer(p)
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := destroyedRoom(t, "post-destroy-"+tc.name)

			done := make(chan struct{})
			go func() {
				defer close(done)
				tc.call(r)
			}()

			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatalf("%s did not return after the room was destroyed: "+
					"run() is gone and nothing receives from the unbuffered "+
					"internalCh, so this goroutine is leaked", tc.name)
			}
		})
	}
}
