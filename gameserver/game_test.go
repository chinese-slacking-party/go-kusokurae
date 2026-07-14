package gameserver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCmdCh_RoutesMessage(t *testing.T) {
	g := &Game{
		ID:      "test",
		Players: []*Player{{ID: "p1"}, {ID: "p2"}, {ID: "p3"}},
		CmdCh:   make(chan GameCommand, 1),
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
