package gameserver

import (
	"context"
	"log"
	"math/rand"

	"github.com/bs-iron-trio/go-kusokurae/sm"
	"github.com/google/uuid"
)

const MaxPlayers = 4

type Game struct {
	ID              string
	Config          *sm.GameConfig
	State           *sm.GameState
	Players         []*Player
	PlayerReaderChs []chan Message
}

func NewGame(config *sm.GameConfig, players []*Player) *Game {
	u, err := uuid.NewRandom()
	if err != nil {
		panic("failed to generate game ID")
	}
	n := int(config.NumPlayers)
	g := &Game{
		ID:              u.String(),
		Config:          config,
		State:           nil,
		Players:         players[:n],
		PlayerReaderChs: make([]chan Message, MaxPlayers),
	}
	for i := 0; i < n; i++ {
		g.PlayerReaderChs[i] = make(chan Message, 1)
	}
	// Slots n..3 remain nil → dead cases in select
	return g
}

func (g *Game) broadcast(msg Message) {
	for _, p := range g.Players {
		if p != nil && p.Session != nil {
			select {
			case p.NoticeCh <- msg:
			default:
			}
		}
	}
}

func (g *Game) sendTo(idx int, msg Message) {
	if idx < 0 || idx >= len(g.Players) {
		return
	}
	p := g.Players[idx]
	if p != nil && p.Session != nil {
		select {
		case p.NoticeCh <- msg:
		default:
		}
	}
}

func (g *Game) GameFn(ctx context.Context) {
	var err error
	g.State, err = sm.NewGame(*g.Config, nil)
	if err != nil {
		log.Printf("GameFn: failed to create game state: %v", err)
		return
	}

	if err = g.State.Start(); err != nil {
		log.Printf("GameFn: failed to start game: %v", err)
		return
	}

	// Start relay goroutines for each player
	for i, p := range g.Players {
		go func(idx int, player *Player) {
			for {
				select {
				case msg := <-player.OperatorCh:
					g.PlayerReaderChs[idx] <- msg
				case <-ctx.Done():
					return
				}
			}
		}(i, p)
	}

	// Broadcast GAME_START
	firstPlayer := g.State.GetActivePlayer()
	firstIdx := int32(firstPlayer.GetIndex() - 1)
	for i := 0; i < int(g.Config.NumPlayers); i++ {
		handCards := g.buildCardInfos(g.State.GetPlayer(int32(i)).GetHandCards())
		g.sendTo(i, Message{
			MsgType: MSG_TYPE_GAME_START,
			MsgBody: &GameStartBody{HandCards: handCards, FirstPlayerIdx: firstIdx},
		})
	}

	// Main game loop
	for g.State.GetStatus() == sm.StatusPlay {
		activePlayer := g.State.GetActivePlayer()
		activeIdx := activePlayer.GetIndex() - 1

		g.sendYOUR_TURN(activeIdx)

		g.waitForMove(ctx, activeIdx)
	}

	// Game over — broadcast final scores
	g.broadcastGameOver()
}

func (g *Game) waitForMove(ctx context.Context, activeIdx int) {
	select {
	case msg := <-g.PlayerReaderChs[0]:
		g.handleMove(0, msg)
	case msg := <-g.PlayerReaderChs[1]:
		g.handleMove(1, msg)
	case msg := <-g.PlayerReaderChs[2]:
		g.handleMove(2, msg)
	case msg := <-g.PlayerReaderChs[3]:
		g.handleMove(3, msg)

	case <-g.disconnectedCh(0):
		if g.isActivePlayer(0) {
			g.autoPlay(0)
		}
	case <-g.disconnectedCh(1):
		if g.isActivePlayer(1) {
			g.autoPlay(1)
		}
	case <-g.disconnectedCh(2):
		if g.isActivePlayer(2) {
			g.autoPlay(2)
		}
	case <-g.disconnectedCh(3):
		if g.isActivePlayer(3) {
			g.autoPlay(3)
		}

	case <-ctx.Done():
		return
	}
}

func (g *Game) handleMove(idx int, msg Message) {
	if !g.isActivePlayer(int32(idx)) {
		g.sendTo(idx, Message{
			MsgType: MSG_TYPE_ERROR,
			MsgBody: &ErrorBody{Message: "not your turn"},
		})
		return
	}

	if msg.MsgType != MSG_TYPE_PLAY_CARD {
		g.sendTo(idx, Message{
			MsgType: MSG_TYPE_ERROR,
			MsgBody: &ErrorBody{Message: "unexpected message type: " + msg.MsgType},
		})
		return
	}

	body, ok := msg.MsgBody.(map[string]interface{})
	if !ok {
		g.sendTo(idx, Message{
			MsgType: MSG_TYPE_ERROR,
			MsgBody: &ErrorBody{Message: "invalid play card body"},
		})
		return
	}
	cardIdxFloat, ok := body["cardIndex"].(float64)
	if !ok {
		g.sendTo(idx, Message{
			MsgType: MSG_TYPE_ERROR,
			MsgBody: &ErrorBody{Message: "cardIndex must be a number"},
		})
		return
	}
	cardIdx := int(cardIdxFloat)

	handCards := g.State.GetActivePlayer().GetHandCards()
	if cardIdx < 0 || cardIdx >= len(handCards) {
		g.sendTo(idx, Message{
			MsgType: MSG_TYPE_ERROR,
			MsgBody: &ErrorBody{Message: "card index out of range"},
		})
		return
	}

	card := handCards[cardIdx]
	if err := g.State.Play(card); err != nil {
		g.sendTo(idx, Message{
			MsgType: MSG_TYPE_ERROR,
			MsgBody: &ErrorBody{Message: err.Error()},
		})
		return
	}

	// Broadcast move
	roundState := g.State.GetRoundState()
	moveInfos := g.buildCardInfos(roundState.Moves)
	playedInfo := CardInfo{Suit: int32(card.GetSuit()), Rank: int32(card.GetRank()), Playable: false}
	g.broadcast(Message{
		MsgType: MSG_TYPE_MOVE_MADE,
		MsgBody: &MoveMadeBody{PlayerIdx: int32(idx), Card: playedInfo, RoundMoves: moveInfos},
	})

	// Check round end
	if g.State.GetActivePlayer().GetRoundStatus() == sm.RoundActive {
		rs := g.State.GetRoundState()
		if rs.RoundWinner != nil {
			g.broadcast(Message{
				MsgType: MSG_TYPE_ROUND_END,
				MsgBody: &RoundEndBody{
					WinnerIdx: int32(rs.RoundWinner.GetIndex() - 1),
					Score:     int32(rs.ScoreOnBoard),
				},
			})
		}
	}
}

func (g *Game) autoPlay(idx int) {
	p := g.State.GetPlayer(int32(idx))
	if p == nil {
		return
	}
	handCards := p.GetHandCards()

	// Find playable cards
	var playable []int
	for i, c := range handCards {
		if c.Playable() {
			playable = append(playable, i)
		}
	}
	if len(playable) == 0 {
		playable = append(playable, 0) // fallback: play first card (busted)
	}

	chosen := playable[rand.Intn(len(playable))]
	card := handCards[chosen]
	if err := g.State.Play(card); err != nil {
		log.Printf("autoPlay: play error for player %d: %v", idx, err)
		return
	}

	roundState := g.State.GetRoundState()
	moveInfos := g.buildCardInfos(roundState.Moves)
	playedInfo := CardInfo{Suit: int32(card.GetSuit()), Rank: int32(card.GetRank()), Playable: false}
	g.broadcast(Message{
		MsgType: MSG_TYPE_MOVE_MADE,
		MsgBody: &MoveMadeBody{PlayerIdx: int32(idx), Card: playedInfo, RoundMoves: moveInfos},
	})
}

func (g *Game) broadcastGameOver() {
	scores := make([]PlayerScore, g.Config.NumPlayers)
	var winnerIdx int32
	var highScore int32
	for i := int32(0); i < g.Config.NumPlayers; i++ {
		s := g.State.GetPlayer(i).GetScore()
		scores[i] = PlayerScore{PlayerIdx: i, Score: int32(s)}
		if int32(s) > highScore {
			highScore = int32(s)
			winnerIdx = i
		}
	}
	g.broadcast(Message{
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

func (g *Game) disconnectedCh(idx int) chan struct{} {
	if idx >= len(g.Players) || g.Players[idx] == nil {
		return nil
	}
	return g.Players[idx].Disconnected
}

func (g *Game) sendYOUR_TURN(idx int) {
	p := g.State.GetPlayer(int32(idx))
	handCards := p.GetHandCards()
	playableIndices := make([]int, 0)
	for i, c := range handCards {
		if c.Playable() {
			playableIndices = append(playableIndices, i)
		}
	}
	rs := g.State.GetRoundState()
	g.sendTo(idx, Message{
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
