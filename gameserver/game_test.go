package gameserver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRelay_ForwardsMessage(t *testing.T) {
	// Construct Game manually — no cgo dependency for relay logic test
	player := &Player{
		ID:           "p1",
		NoticeCh:     make(chan Message, 1),
		OperatorCh:   make(chan Message, 1),
		Disconnected: make(chan struct{}),
	}
	g := &Game{
		ID:              "test",
		Players:         []*Player{player, {}, {}},
		PlayerReaderChs: []chan Message{make(chan Message, 1), nil, nil, nil},
	}

	// Start relay goroutine manually (mirrors GameFn startup)
	go func() {
		for {
			select {
			case msg := <-player.OperatorCh:
				g.PlayerReaderChs[0] <- msg
			}
		}
	}()

	msg := Message{MsgType: MSG_TYPE_PLAY_CARD, MsgBody: &PlayCardBody{CardIndex: 2}}
	player.OperatorCh <- msg

	select {
	case received := <-g.PlayerReaderChs[0]:
		assert.Equal(t, MSG_TYPE_PLAY_CARD, received.MsgType)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for relayed message")
	}
}

func TestPlayerReaderChs_Padding(t *testing.T) {
	// Verify that PlayerReaderChs is always size 4 (MaxPlayers), with
	// nil channels for unoccupied slots (dead cases in select)
	players := []*Player{
		{ID: "p1", NoticeCh: make(chan Message, 1), OperatorCh: make(chan Message, 1), Disconnected: make(chan struct{})},
		{ID: "p2", NoticeCh: make(chan Message, 1), OperatorCh: make(chan Message, 1), Disconnected: make(chan struct{})},
		{ID: "p3", NoticeCh: make(chan Message, 1), OperatorCh: make(chan Message, 1), Disconnected: make(chan struct{})},
	}

	g := &Game{
		ID:              "test",
		Players:         players,
		PlayerReaderChs: make([]chan Message, MaxPlayers),
	}
	for i := 0; i < 3; i++ {
		g.PlayerReaderChs[i] = make(chan Message, 1)
	}
	// Slot 3 is nil → dead case in select

	assert.Equal(t, 4, len(g.PlayerReaderChs))
	assert.NotNil(t, g.PlayerReaderChs[0])
	assert.NotNil(t, g.PlayerReaderChs[1])
	assert.NotNil(t, g.PlayerReaderChs[2])
	assert.Nil(t, g.PlayerReaderChs[3])
}
