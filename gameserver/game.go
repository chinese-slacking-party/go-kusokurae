package gameserver

import (
	"github.com/bs-iron-trio/go-kusokurae/sm"
	"github.com/google/uuid"
)

type Game struct {
	ID     string
	Config *sm.GameConfig
	State  *sm.GameState
}

func NewGame(config *sm.GameConfig) *Game {
	u, err := uuid.NewRandom()
	if err != nil {
		panic("failed to generate player ID")
	}
	return &Game{
		ID:     u.String(),
		Config: config,
		State:  &sm.GameState{},
	}
}
