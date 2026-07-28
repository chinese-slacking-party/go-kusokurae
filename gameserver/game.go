package gameserver

import (
	"context"
	"log"
	"sync"

	"github.com/bs-iron-trio/go-kusokurae/sm"
	"github.com/google/uuid"
)

const MaxPlayers = 4

type GameCommand struct {
	PlayerIdx int
	Msg       Message
}

type GameEvent struct {
	Target int // player index, -1 = broadcast
	Msg    Message
}

type Game struct {
	ID         string
	StateMutex sync.Mutex
	Config     *sm.GameConfig
	State      *sm.GameState
	NumPlayers int32
	CmdCh      chan GameCommand
	EventCh    chan GameEvent
	GameOver   chan struct{}
}

func NewGame(config *sm.GameConfig, numPlayers int32) *Game {
	u, err := uuid.NewRandom()
	if err != nil {
		panic("failed to generate game ID")
	}
	g := &Game{
		ID:         u.String(),
		Config:     config,
		NumPlayers: numPlayers,
		CmdCh:      make(chan GameCommand),
		EventCh:    make(chan GameEvent),
		GameOver:   make(chan struct{}),
	}
	return g
}

func trunc8(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

func (g *Game) emit(target int, msg Message) {
	g.EventCh <- GameEvent{Target: target, Msg: msg}
	log.Printf("Game %s emit %s msg to %v\n", trunc8(g.ID), msg.MsgType, target)
}

func (g *Game) GameFn(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer close(g.GameOver)

	var err error
	g.State, err = sm.NewGame(*g.Config, func(state sm.GameStatus) {
		// if g.State.GetStatus() != state {
		// 	g.emit(-1, Message{
		// 		MsgType: MSG_TYPE_ERROR,
		// 		MsgBody: &ErrorBody{Message: fmt.Sprintf("GameStatus %v -> %v", g.State.GetStatus(), state)},
		// 	})
		// 	return
		// }
		rs := g.State.GetRoundState()
		g.emit(-1, Message{
			MsgType: MSG_TYPE_ROUND_END,
			MsgBody: &RoundEndBody{
				WinnerIdx: int32(rs.RoundWinner.GetIndex() - 1),
				Score:     int32(rs.ScoreOnBoard),
				IsDoubled: rs.IsDoubled,
			},
		})
	})
	if err != nil {
		log.Printf("GameFn: failed to create game state: %v", err)
		return
	}

	if err = g.State.Start(); err != nil {
		log.Printf("GameFn: failed to start game: %v", err)
		return
	}

	// Broadcast GAME_START — per-player events
	firstPlayer := g.State.GetActivePlayer()
	firstIdx := int32(firstPlayer.GetIndex() - 1)
	for i := int32(0); i < g.NumPlayers; i++ {
		handCards := g.buildCardInfos(g.State.GetPlayer(i).GetHandCards())
		g.emit(int(i), Message{
			MsgType: MSG_TYPE_GAME_START,
			MsgBody: &GameStartBody{HandCards: handCards, FirstPlayerIdx: firstIdx, YourPlayerIdx: i},
		})
	}

	// Main game loop — no disconnect/autoPlay cases
	for g.State.GetStatus() == sm.StatusPlay {
		activeIdx := int(g.State.GetActivePlayer().GetIndex() - 1)
		g.emitYOUR_TURN(activeIdx)

		select {
		case cmd := <-g.CmdCh:
			g.handleMove(cmd.PlayerIdx, cmd.Msg)
		case <-ctx.Done():
			return
		}
	}

	g.broadcastGameOver()
}

func (g *Game) handleMove(idx int, msg Message) {
	g.StateMutex.Lock()
	defer g.StateMutex.Unlock()
	log.Printf("Game %s recv %d, %s\n", trunc8(g.ID), idx, msg.MsgType)
	if !g.isActivePlayer(int32(idx)) {
		g.emit(idx, Message{
			MsgType: MSG_TYPE_ERROR,
			MsgBody: &ErrorBody{Message: "not your turn"},
		})
		return
	}

	if msg.MsgType != MSG_TYPE_PLAY_CARD {
		g.emit(idx, Message{
			MsgType: MSG_TYPE_ERROR,
			MsgBody: &ErrorBody{Message: "unexpected message type: " + msg.MsgType},
		})
		return
	}

	body, ok := msg.MsgBody.(map[string]interface{})
	if !ok {
		g.emit(idx, Message{
			MsgType: MSG_TYPE_ERROR,
			MsgBody: &ErrorBody{Message: "invalid play card body"},
		})
		return
	}
	cardIdxFloat, ok := body["card_index"].(float64)
	if !ok {
		g.emit(idx, Message{
			MsgType: MSG_TYPE_ERROR,
			MsgBody: &ErrorBody{Message: "card_index must be a number"},
		})
		return
	}
	cardIdx := int(cardIdxFloat)

	handCards := g.State.GetActivePlayer().GetHandCards()
	if cardIdx < 0 || cardIdx >= len(handCards) {
		g.emit(idx, Message{
			MsgType: MSG_TYPE_ERROR,
			MsgBody: &ErrorBody{Message: "card index out of range"},
		})
		return
	}

	card := handCards[cardIdx]
	if err := g.State.Play(card); err != nil {
		g.emit(idx, Message{
			MsgType: MSG_TYPE_ERROR,
			MsgBody: &ErrorBody{Message: err.Error()},
		})
		return
	}

	roundState := g.State.GetRoundState()
	moveInfos := g.buildCardInfos(roundState.Moves)
	playedInfo := CardInfo{Suit: int32(card.GetSuit()), Rank: int32(card.GetRank()), Playable: false}
	g.emit(-1, Message{
		MsgType: MSG_TYPE_MOVE_MADE,
		MsgBody: &MoveMadeBody{PlayerIdx: int32(idx), Card: playedInfo, RoundMoves: moveInfos},
	})

}

func (g *Game) broadcastGameOver() {
	scores := make([]PlayerScore, g.NumPlayers)
	var winnerIdx int32
	var highScore int32
	for i := int32(0); i < g.NumPlayers; i++ {
		s := g.State.GetPlayer(i).GetScore()
		scores[i] = PlayerScore{PlayerIdx: i, Score: int32(s)}
		if int32(s) > highScore {
			highScore = int32(s)
			winnerIdx = i
		}
	}
	g.emit(-1, Message{
		MsgType: MSG_TYPE_GAME_OVER,
		MsgBody: &GameOverBody{FinalScores: scores, WinnerIdx: winnerIdx},
	})
}

func (g *Game) isActivePlayer(idx int32) bool {
	ap := g.State.GetActivePlayer()
	if ap == nil {
		return false
	}
	return ap.GetIndex()-1 == int(idx)
}

func (g *Game) emitYOUR_TURN(idx int) {
	p := g.State.GetPlayer(int32(idx))
	handCards := p.GetHandCards()
	playableIndices := make([]int, 0)
	for i, c := range handCards {
		if c.Playable() {
			playableIndices = append(playableIndices, i)
		}
	}
	rs := g.State.GetRoundState()
	g.emit(idx, Message{
		MsgType: MSG_TYPE_YOUR_TURN,
		MsgBody: &YourTurnBody{
			PlayableIndices: playableIndices,
			RoundSeq:        rs.Seq,
			RoundMoves:      g.buildCardInfos(rs.Moves),
		},
	})
}

func (g *Game) buildCardInfos(cards []sm.Card) []CardInfo {
	infos := make([]CardInfo, len(cards))
	for i, c := range cards {
		infos[i] = CardInfo{
			Suit:     int32(c.GetSuit()),
			Rank:     int32(c.GetRank()),
			Playable: c.Playable(),
		}
	}
	return infos
}
