package gameserver

import (
	"context"
	"fmt"

	"github.com/bs-iron-trio/go-kusokurae/sm"
	"github.com/google/uuid"
)

type Game struct {
	ID              string
	Config          *sm.GameConfig
	State           *sm.GameState
	PlayerWriterChs []chan Message
	PlayerReaderChs []chan Message
}

func NewGame(config *sm.GameConfig) *Game {
	u, err := uuid.NewRandom()
	if err != nil {
		panic("failed to generate player ID")
	}
	var g = &Game{
		ID:              u.String(),
		Config:          config,
		State:           nil,
		PlayerWriterChs: make([]chan Message, config.NumPlayers),
		PlayerReaderChs: make([]chan Message, config.NumPlayers),
	}
	for i := 0; i < int(config.NumPlayers); i++ {
		g.PlayerWriterChs[i] = make(chan Message)
		g.PlayerReaderChs[i] = make(chan Message)

	}
	return g
}

func (g *Game) GameFn(ctx context.Context) {
	if p := recover(); p != nil {
		var errMsg = Message{
			MsgType: MSG_TYPE_FATAL,
			MsgBody: &SMSMesssageBody{
				Data: fmt.Sprint("error %w", p),
			},
		}
		for i := 0; i < int(g.Config.NumPlayers); i++ {
			g.PlayerWriterChs[i] <- errMsg
		}
	}

}
