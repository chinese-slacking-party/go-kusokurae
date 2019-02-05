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
