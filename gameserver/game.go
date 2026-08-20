package gameserver

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/bs-iron-trio/go-kusokurae/sm"
	"github.com/google/uuid"
)

const MaxPlayers = 4

const (
	DefaultTurnTimeout      = 30 * time.Second
	MinTurnTimeoutSec       = 5
	MaxTurnTimeoutSec       = 120
	DefaultTurnSyncInterval = 5 * time.Second
	MinTurnSyncIntervalSec  = 1
	MaxTurnSyncIntervalSec  = 60
)

// ValidateTurnTimeoutSec checks a turn timeout value in seconds.
// 0 means "use the default" and is always valid.
func ValidateTurnTimeoutSec(secs int32) error {
	if secs == 0 {
		return nil
	}
	if secs < MinTurnTimeoutSec || secs > MaxTurnTimeoutSec {
		return fmt.Errorf("turn_timeout_seconds must be between %d and %d", MinTurnTimeoutSec, MaxTurnTimeoutSec)
	}
	return nil
}

// ValidateTurnSyncIntervalSec checks a sync interval value in seconds against
// the turn timeout. 0 means "use the default". A valid interval is within
// 1..MaxTurnSyncIntervalSec and strictly less than the effective turn timeout
// (turnTimeoutSec 0 means the default 30s).
func ValidateTurnSyncIntervalSec(secs, turnTimeoutSec int32) error {
	if secs == 0 {
		return nil
	}
	if secs < MinTurnSyncIntervalSec || secs > MaxTurnSyncIntervalSec {
		return fmt.Errorf("turn_sync_interval_seconds must be between %d and %d", MinTurnSyncIntervalSec, MaxTurnSyncIntervalSec)
	}
	effectiveTimeout := turnTimeoutSec
	if effectiveTimeout == 0 {
		effectiveTimeout = int32(DefaultTurnTimeout / time.Second)
	}
	if secs >= effectiveTimeout {
		return fmt.Errorf("turn_sync_interval_seconds must be less than turn_timeout_seconds (%d)", effectiveTimeout)
	}
	return nil
}

type GameCommand struct {
	PlayerIdx int
	Msg       Message
}

type GameEvent struct {
	Target int // player index, -1 = broadcast
	Msg    Message
}

type Game struct {
	ID               string
	StateMutex       sync.Mutex
	Config           *sm.GameConfig
	State            *sm.GameState
	NumPlayers       int32
	TurnTimeout      time.Duration
	TurnSyncInterval time.Duration
	CmdCh            chan GameCommand
	EventCh          chan GameEvent
	GameEnd          chan struct{}
	abortCh          chan struct{}
	abortOnce        sync.Once
	pendingRoundEnd  *RoundEndBody
	turnDeadline     time.Time
	panicHook        func()
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
		GameEnd:    make(chan struct{}),
		abortCh:    make(chan struct{}),
	}
	return g
}

func trunc8(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

func (g *Game) emit(target int, msg Message) bool {
	select {
	case g.EventCh <- GameEvent{Target: target, Msg: msg}:
		log.Printf("Game %s emit %s msg to %v\n", trunc8(g.ID), msg.MsgType, target)
		return true
	case <-g.abortCh:
		// Game is being destroyed; drop the event.
		return false
	}
}

// Abort signals the game goroutine to stop promptly, even if it is blocked on
// an EventCh emit. Safe to call multiple times.
func (g *Game) Abort() {
	g.abortOnce.Do(func() {
		close(g.abortCh)
	})
}

func (g *Game) GameFn(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer close(g.GameEnd)

	// Recover from any panic during the game: notify all players with
	// GAME_FATAL, then let the defers close GameEnd. Runs before close(GameEnd)
	// so the room broadcasts the fatal before cleaning up.
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Game %s panicked: %v", trunc8(g.ID), r)
			g.emit(-1, Message{
				MsgType: MSG_TYPE_GAME_FATAL,
				MsgBody: &GameFatalBody{Message: fmt.Sprint(r)},
			})
		}
	}()

	var err error
	g.StateMutex.Lock()
	g.State, err = sm.NewGame(*g.Config, func(state sm.GameStatus) {
		var status = g.State.GetStatus()
		if status != state {
			log.Printf("Game %s GameStatus %v -> %v", g.ID, status, state)
			return
		}

		rs := g.State.GetRoundState()
		g.pendingRoundEnd = &RoundEndBody{
			WinnerIdx: int32(rs.RoundWinner.GetIndex() - 1),
			Score:     int32(rs.ScoreOnBoard),
			IsDoubled: rs.IsDoubled,
		}
	})
	g.StateMutex.Unlock()
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

	// Main game loop
	if g.TurnTimeout <= 0 {
		g.TurnTimeout = DefaultTurnTimeout
	}
	if g.TurnSyncInterval <= 0 {
		g.TurnSyncInterval = DefaultTurnSyncInterval
	}
	for g.State.GetStatus() == sm.StatusPlay {
		if g.panicHook != nil {
			g.panicHook()
		}
		activeIdx := int(g.State.GetActivePlayer().GetIndex() - 1)
		g.turnDeadline = time.Now().Add(g.TurnTimeout)
		g.emitYOUR_TURN(activeIdx)
		if !g.waitForMove(ctx, activeIdx, g.turnDeadline) {
			return
		}
	}

	g.broadcastGameOver()
}

// waitForMove waits for the active player to play, returning false only when
// the context is canceled. The countdown is anchored to the start of the turn:
// failed commands (invalid body, forbidden move, ...) only get an ERROR and a
// re-sent YOUR_TURN — they do NOT reset the deadline. Returns true once a card
// has been played (by the player or by timeout auto-play).
func (g *Game) waitForMove(ctx context.Context, activeIdx int, deadline time.Time) bool {
	ticker := time.NewTicker(g.TurnSyncInterval)
	defer ticker.Stop()
	for {
		select {
		case cmd := <-g.CmdCh:
			if g.handleCommand(cmd) {
				return true
			}
			// Failed move or non-turn-ending command: error (+ re-sent YOUR_TURN)
			// may already have been emitted; keep waiting on the same deadline.
		case <-time.After(time.Until(deadline)):
			g.handleTurnTimeout(activeIdx)
			return true
		case <-ticker.C:
			g.emitTurnTimeSync(activeIdx, time.Until(deadline))
		case <-g.abortCh:
			return false
		case <-ctx.Done():
			return false
		}
	}
}

// handleCommand dispatches a game command by message type. Returns true if a
// card was played (the turn has ended); false otherwise.
func (g *Game) handleCommand(cmd GameCommand) bool {
	switch cmd.Msg.MsgType {
	case MSG_TYPE_PLAY_CARD:
		return g.handleMove(cmd.PlayerIdx, cmd.Msg)
	case MSG_TYPE_RESYNC_STATE:
		g.emitGameResync(cmd.PlayerIdx)
		return false
	default:
		log.Printf("Game %s dropped unknown command %s from %d\n", trunc8(g.ID), cmd.Msg.MsgType, cmd.PlayerIdx)
		return false
	}
}

// emitTurnTimeSync sends a periodic time-sync tick to the active player.
// Ticks with no time left are dropped: the timeout case in waitForMove takes
// over at the deadline.
func (g *Game) emitTurnTimeSync(activeIdx int, remaining time.Duration) {
	secs := int(math.Ceil(remaining.Seconds()))
	if secs <= 0 {
		return
	}
	rs := g.State.GetRoundState()
	g.emit(activeIdx, Message{
		MsgType: MSG_TYPE_TURN_TIME_SYNC,
		MsgBody: &TurnTimeSyncBody{RemainingSeconds: secs, RoundSeq: rs.Seq},
	})
}

// handleMove processes a player command. Returns true if a card was played
// (the turn has ended); false otherwise. Failed moves from the active player
// re-send YOUR_TURN so the client can retry on the same countdown.
func (g *Game) handleMove(idx int, msg Message) bool {
	g.StateMutex.Lock()
	defer g.StateMutex.Unlock()
	log.Printf("Game %s recv %d, %s\n", trunc8(g.ID), idx, msg.MsgType)
	if !g.isActivePlayer(int32(idx)) {
		g.emit(idx, Message{
			MsgType: MSG_TYPE_ERROR,
			MsgBody: &ErrorBody{Message: "not your turn"},
		})
		return false
	}

	if msg.MsgType != MSG_TYPE_PLAY_CARD {
		g.emit(idx, Message{
			MsgType: MSG_TYPE_ERROR,
			MsgBody: &ErrorBody{Message: "unexpected message type: " + msg.MsgType},
		})
		g.emitYOUR_TURN(idx)
		return false
	}

	cardIdx, ok := playCardIndex(msg.MsgBody)
	if !ok {
		g.emit(idx, Message{
			MsgType: MSG_TYPE_ERROR,
			MsgBody: &ErrorBody{Message: "invalid play card body"},
		})
		g.emitYOUR_TURN(idx)
		return false
	}

	handCards := g.State.GetActivePlayer().GetHandCards()
	if cardIdx < 0 || cardIdx >= len(handCards) {
		g.emit(idx, Message{
			MsgType: MSG_TYPE_ERROR,
			MsgBody: &ErrorBody{Message: "card index out of range"},
		})
		g.emitYOUR_TURN(idx)
		return false
	}

	if !g.playCard(idx, cardIdx, false) {
		// Engine rejected the move (e.g. forbidden): re-prompt the player.
		g.emitYOUR_TURN(idx)
		return false
	}
	return true
}

// playCardIndex extracts the card index from a PLAY_CARD body, accepting both
// the JSON-decoded map form (from WebSocket input) and the typed form (from
// server-side auto-play).
func playCardIndex(body any) (int, bool) {
	switch b := body.(type) {
	case map[string]interface{}:
		f, ok := b["card_index"].(float64)
		if !ok {
			return 0, false
		}
		return int(f), true
	case *PlayCardBody:
		return b.CardIndex, true
	}
	return 0, false
}

// playCard plays hand[cardIdx] for playerIdx and broadcasts the result.
// The caller must hold StateMutex. Returns true if the card was played;
// on engine rejection an ERROR is emitted to the player and false is returned.
func (g *Game) playCard(playerIdx, cardIdx int, autoPlay bool) bool {
	handCards := g.State.GetActivePlayer().GetHandCards()
	if cardIdx < 0 || cardIdx >= len(handCards) {
		return false
	}
	card := handCards[cardIdx]
	if err := g.State.Play(card); err != nil {
		g.emit(playerIdx, Message{
			MsgType: MSG_TYPE_ERROR,
			MsgBody: &ErrorBody{Message: err.Error()},
		})
		return false
	}

	roundState := g.State.GetRoundState()
	moveInfos := g.buildCardInfos(roundState.Moves)
	playedInfo := CardInfo{Suit: int32(card.GetSuit()), Rank: int32(card.GetRank()), Playable: false}
	g.emit(-1, Message{
		MsgType: MSG_TYPE_MOVE_MADE,
		MsgBody: &MoveMadeBody{PlayerIdx: int32(playerIdx), CardIdx: int32(cardIdx), Card: playedInfo, RoundMoves: moveInfos, AutoPlay: autoPlay},
	})

	if g.pendingRoundEnd != nil {
		<-time.After(500 * time.Millisecond)
		g.emit(-1, Message{
			MsgType: MSG_TYPE_ROUND_END,
			MsgBody: g.pendingRoundEnd,
		})
		g.pendingRoundEnd = nil
	}
	return true
}

// emitGameResync builds the game-layer snapshot for a resyncing player and
// emits it as an internal GAME_RESYNC event (consumed by the room). Runs on
// the GameFn goroutine which owns the state, so no locking is needed.
func (g *Game) emitGameResync(playerIdx int) {
	if g.State.GetStatus() != sm.StatusPlay {
		return
	}
	p := g.State.GetPlayer(int32(playerIdx))
	if p == nil {
		return
	}
	rs := g.State.GetRoundState()

	body := &GameResyncBody{
		Status:     int32(sm.StatusPlay),
		HandCards:  g.buildCardInfos(p.GetHandCards()),
		RoundSeq:   rs.Seq,
		RoundMoves: g.buildCardInfos(rs.Moves),
	}
	for i := int32(0); i < g.NumPlayers; i++ {
		pl := g.State.GetPlayer(i)
		body.Scores = append(body.Scores, PlayerScore{PlayerIdx: i, Score: int32(pl.GetScore())})
	}
	ap := g.State.GetActivePlayer()
	if ap != nil {
		body.ActivePlayerIdx = int32(ap.GetIndex() - 1)
		if body.ActivePlayerIdx == int32(playerIdx) {
			hand := ap.GetHandCards()
			for i, c := range hand {
				if c.Playable() {
					body.PlayableIndices = append(body.PlayableIndices, i)
				}
			}
			body.RemainingSeconds = int(math.Ceil(time.Until(g.turnDeadline).Seconds()))
			if body.RemainingSeconds < 0 {
				body.RemainingSeconds = 0
			}
		}
	}
	g.emit(playerIdx, Message{
		MsgType: MSG_TYPE_GAME_RESYNC,
		MsgBody: body,
	})
}

// handleTurnTimeout auto-plays the largest playable card for the player whose
// turn this is. Re-checks status and active player defensively: the state can
// only change through this goroutine, so a stale activeIdx here would indicate
// a bug, not a race.
func (g *Game) handleTurnTimeout(activeIdx int) {
	g.StateMutex.Lock()
	defer g.StateMutex.Unlock()
	if g.State.GetStatus() != sm.StatusPlay {
		return
	}
	ap := g.State.GetActivePlayer()
	if ap == nil || ap.GetIndex()-1 != activeIdx {
		return
	}
	chosen := sm.PickLargestPlayable(ap.GetHandCards())
	if chosen < 0 {
		return
	}
	g.playCard(activeIdx, chosen, true)
}

func (g *Game) broadcastGameOver() {
	scores := make([]PlayerScore, g.NumPlayers)
	for i := int32(0); i < g.NumPlayers; i++ {
		scores[i] = PlayerScore{PlayerIdx: i, Score: int32(g.State.GetPlayer(i).GetScore())}
	}
	g.emit(-1, Message{
		MsgType: MSG_TYPE_GAME_OVER,
		MsgBody: &GameOverBody{FinalScores: scores, WinnerIdxs: determineWinners(scores)},
	})
}

// determineWinners returns every player tied for the highest score, in seat
// order. Scores are routinely negative (Shit cards are worth -1 each, and the
// Ghost doubles a losing trick), so the running maximum must start below any
// reachable score rather than at zero.
func determineWinners(scores []PlayerScore) []int32 {
	if len(scores) == 0 {
		return nil
	}
	highScore := int32(math.MinInt32)
	for _, ps := range scores {
		if ps.Score > highScore {
			highScore = ps.Score
		}
	}
	winners := make([]int32, 0, len(scores))
	for _, ps := range scores {
		if ps.Score == highScore {
			winners = append(winners, ps.PlayerIdx)
		}
	}
	return winners
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
			PlayableIndices:  playableIndices,
			RoundSeq:         rs.Seq,
			RoundMoves:       g.buildCardInfos(rs.Moves),
			TimeoutSeconds:   int(g.TurnTimeout / time.Second),
			RemainingSeconds: int(math.Ceil(time.Until(g.turnDeadline).Seconds())),
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
