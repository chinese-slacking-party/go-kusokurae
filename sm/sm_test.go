package sm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewGame(t *testing.T) {
	_, err := NewGame(GameConfig{
		NumPlayers: 2,
	})
	assert.Equal(t, ErrBadNPlayers, err)

	correctCfg := GameConfig{
		NumPlayers: 3,
	}
	state, err := NewGame(correctCfg)
	assert.NoError(t, err)
	assert.Equal(t, correctCfg, state.cfg)
	assert.Equal(t, StatusInit, state.status)
	assert.Equal(t, 1, int(state.players[0].Index))
	assert.Equal(t, 2, int(state.players[1].Index))
	assert.Equal(t, 3, int(state.players[2].Index))
	assert.Equal(t, 0, int(state.players[3].Index))
}

func TestGameStart(t *testing.T) {
	state, err := NewGame(GameConfig{
		NumPlayers: 3,
	})
	assert.NoError(t, err)

	err = state.Start()
	assert.NoError(t, err)
	assert.Equal(t, RoundActive, state.players[0].Active)
	assert.Equal(t, RoundWaiting, state.players[1].Active)
	assert.Equal(t, RoundWaiting, state.players[2].Active)
	assert.Equal(t, StatusPlay, state.status)

	t.Log(state.players[0].Hand)
	t.Log(state.players[1].Hand)
	t.Log(state.players[2].Hand)
	var cards [33]bool
	var i, j int
	var order uint32
	for i = 0; i < 3; i++ {
		for j = 0; j < 11; j++ {
			order = state.players[i].Hand[j].DisplayOrder - 1
			if cards[order] {
				t.Errorf("Duplicate card %+v", state.players[i].Hand[j])
			}
		}
	}
}
