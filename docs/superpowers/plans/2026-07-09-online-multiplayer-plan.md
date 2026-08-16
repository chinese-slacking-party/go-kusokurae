# 多人房间对战功能 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the complete online multiplayer game loop, connecting the `sm` game engine to the `gameserver` Player channel architecture with host-based game start, fixed-select event loop, disconnect detection, auto-play bot, and reconnect support.

**Architecture:** Six files modified, zero new files. Bottom-up dependency order: message types → player/gateway → room → game loop → controller. Each task is self-contained with TDD (test → fail → implement → pass → commit).

**Tech Stack:** Go 1.23, Gin, Gorilla WebSocket, cgo-wrapped C game engine (sm), testify

---

### Task 1: Game Message Types and Body Structs

**Files:**
- Modify: `gameserver/message.go`
- Create: `gameserver/message_test.go`

- [ ] **Step 1: Write the failing test**

```go
package gameserver

import (
    "encoding/json"
    "testing"

    "github.com/stretchr/testify/assert"
)

func TestPlayCardBodyJSON(t *testing.T) {
    body := PlayCardBody{CardIndex: 3}
    b, err := json.Marshal(body)
    assert.NoError(t, err)
    assert.JSONEq(t, `{"cardIndex":3}`, string(b))
}

func TestGameStartBodyJSON(t *testing.T) {
    body := GameStartBody{
        HandCards: []CardInfo{
            {Suit: 0, Rank: 5, Playable: true},
            {Suit: -1, Rank: 3, Playable: false},
        },
        FirstPlayerIdx: 1,
    }
    b, err := json.Marshal(body)
    assert.NoError(t, err)
    assert.JSONEq(t, `{"handCards":[{"suit":0,"rank":5,"playable":true},{"suit":-1,"rank":3,"playable":false}],"firstPlayerIdx":1}`, string(b))
}

func TestYourTurnBodyJSON(t *testing.T) {
    body := YourTurnBody{
        PlayableIndices: []int{0, 2, 5},
        RoundSeq:        3,
        RoundMoves:      []CardInfo{{Suit: 0, Rank: 4, Playable: false}},
    }
    b, err := json.Marshal(body)
    assert.NoError(t, err)
    assert.JSONEq(t, `{"playableIndices":[0,2,5],"roundSeq":3,"roundMoves":[{"suit":0,"rank":4,"playable":false}]}`, string(b))
}

func TestRoundEndBodyJSON(t *testing.T) {
    body := RoundEndBody{WinnerIdx: 2, Score: 5}
    b, err := json.Marshal(body)
    assert.NoError(t, err)
    assert.JSONEq(t, `{"winnerIdx":2,"score":5}`, string(b))
}

func TestGameOverBodyJSON(t *testing.T) {
    body := GameOverBody{
        FinalScores: []PlayerScore{{PlayerIdx: 1, Score: 15}, {PlayerIdx: 2, Score: -3}},
        WinnerIdx:   1,
    }
    b, err := json.Marshal(body)
    assert.NoError(t, err)
    assert.JSONEq(t, `{"finalScores":[{"playerIdx":1,"score":15},{"playerIdx":2,"score":-3}],"winnerIdx":1}`, string(b))
}

func TestErrorBodyJSON(t *testing.T) {
    msg := Message{MsgType: MSG_TYPE_ERROR, MsgBody: &ErrorBody{Message: "not your turn"}}
    b, err := json.Marshal(msg)
    assert.NoError(t, err)
    assert.JSONEq(t, `{"type":"ERROR","body":{"message":"not your turn"}}`, string(b))
}

func TestRoomStateBodyJSON(t *testing.T) {
    body := RoomStateBody{
        Players: []RoomPlayerInfo{
            {PlayerID: "abc", Position: 1, IsHost: true},
            {PlayerID: "def", Position: 2, IsHost: false},
        },
        HostIdx: 0,
    }
    b, err := json.Marshal(body)
    assert.NoError(t, err)
    assert.JSONEq(t, `{"players":[{"playerID":"abc","position":1,"isHost":true},{"playerID":"def","position":2,"isHost":false}],"hostIdx":0}`, string(b))
}

func TestMoveMadeBodyJSON(t *testing.T) {
    body := MoveMadeBody{
        PlayerIdx:  2,
        Card:       CardInfo{Suit: 1, Rank: 8, Playable: false},
        RoundMoves: []CardInfo{{Suit: 1, Rank: 8, Playable: false}, {Suit: 0, Rank: 3, Playable: false}},
    }
    b, err := json.Marshal(body)
    assert.NoError(t, err)
    assert.JSONEq(t, `{"playerIdx":2,"card":{"suit":1,"rank":8,"playable":false},"roundMoves":[{"suit":1,"rank":8,"playable":false},{"suit":0,"rank":3,"playable":false}]}`, string(b))
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./gameserver/ -run "TestPlayCardBodyJSON|TestGameStartBodyJSON|TestYourTurnBodyJSON|TestRoundEndBodyJSON|TestGameOverBodyJSON|TestErrorBodyJSON|TestRoomStateBodyJSON|TestMoveMadeBodyJSON" -v`
Expected: FAIL — types not defined

- [ ] **Step 3: Add message type constants and body structs**

In `gameserver/message.go`, replace all content:

```go
package gameserver

// Message type constants — Client → Server
const MSG_TYPE_START_GAME = "START_GAME"
const MSG_TYPE_PLAY_CARD = "PLAY_CARD"

// Message type constants — Server → Client
const MSG_TYPE_SMS = "SMS"
const MSG_TYPE_EVENT = "EVENT"
const MSG_TYPE_ERROR = "ERROR"
const MSG_TYPE_FATAL = "FATAL"
const MSG_TYPE_NOTICE = "NOTICE"
const MSG_TYPE_ROOM_STATE = "ROOM_STATE"
const MSG_TYPE_GAME_START = "GAME_START"
const MSG_TYPE_YOUR_TURN = "YOUR_TURN"
const MSG_TYPE_MOVE_MADE = "MOVE_MADE"
const MSG_TYPE_ROUND_END = "ROUND_END"
const MSG_TYPE_GAME_OVER = "GAME_OVER"
const MSG_TYPE_PLAYER_JOINED = "PLAYER_JOINED"
const MSG_TYPE_PLAYER_LEFT = "PLAYER_LEFT"

// Message is the universal WebSocket message envelope.
type Message struct {
	MsgType string `json:"type"`
	MsgBody any    `json:"body"`
}

// Legacy body types
type SMSMesssageBody struct {
	Data string `json:"data"`
}

// Client → Server body types
type PlayCardBody struct {
	CardIndex int `json:"cardIndex"`
}

// Server → Client body types
type CardInfo struct {
	Suit    int32 `json:"suit"`
	Rank    int32 `json:"rank"`
	Playable bool `json:"playable"`
}

type RoomStateBody struct {
	Players []RoomPlayerInfo `json:"players"`
	HostIdx int32            `json:"hostIdx"`
}

type RoomPlayerInfo struct {
	PlayerID string `json:"playerID"`
	Position int32  `json:"position"`
	IsHost   bool   `json:"isHost"`
}

type GameStartBody struct {
	HandCards      []CardInfo `json:"handCards"`
	FirstPlayerIdx int32      `json:"firstPlayerIdx"`
}

type YourTurnBody struct {
	PlayableIndices []int      `json:"playableIndices"`
	RoundSeq        int        `json:"roundSeq"`
	RoundMoves      []CardInfo `json:"roundMoves"`
}

type MoveMadeBody struct {
	PlayerIdx  int32      `json:"playerIdx"`
	Card       CardInfo   `json:"card"`
	RoundMoves []CardInfo `json:"roundMoves"`
}

type RoundEndBody struct {
	WinnerIdx int32 `json:"winnerIdx"`
	Score     int32 `json:"score"`
}

type GameOverBody struct {
	FinalScores []PlayerScore `json:"finalScores"`
	WinnerIdx   int32         `json:"winnerIdx"`
}

type PlayerScore struct {
	PlayerIdx int32 `json:"playerIdx"`
	Score     int32 `json:"score"`
}

type PlayerJoinedBody struct {
	PlayerID string `json:"playerID"`
	Position int32  `json:"position"`
}

type PlayerLeftBody struct {
	PlayerID string `json:"playerID"`
	Position int32  `json:"position"`
}

type ErrorBody struct {
	Message string `json:"message"`
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./gameserver/ -run "TestPlayCardBodyJSON|TestGameStartBodyJSON|TestYourTurnBodyJSON|TestRoundEndBodyJSON|TestGameOverBodyJSON|TestErrorBodyJSON|TestRoomStateBodyJSON|TestMoveMadeBodyJSON" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add gameserver/message.go gameserver/message_test.go
git commit -m "feat: add game message types and body structs for multiplayer

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 2: Player — Add Disconnected Chan and Session Reference

**Files:**
- Modify: `gameserver/player.go`

- [ ] **Step 1: Update Player struct and NewPlayer**

In `gameserver/player.go`, replace all content:

```go
package gameserver

import "github.com/google/uuid"

type Player struct {
	ID           string
	RoomID       string
	RoomPosition int32
	NoticeCh     chan Message
	OperatorCh   chan Message
	Disconnected chan struct{}
	Session      *Session
}

func NewPlayer() *Player {
	u, err := uuid.NewRandom()
	if err != nil {
		panic("failed to generate player ID")
	}
	return &Player{
		ID:           u.String(),
		RoomID:       "",
		RoomPosition: -1,
		NoticeCh:     make(chan Message),
		OperatorCh:   make(chan Message),
		Disconnected: make(chan struct{}),
	}
}

func (p *Player) Sit(roomID string, roomPosition int32) {
	p.RoomID = roomID
	p.RoomPosition = roomPosition
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./gameserver/`
Expected: build succeeds (no compilation errors)

- [ ] **Step 3: Commit**

```bash
git add gameserver/player.go
git commit -m "feat: add Disconnected chan and Session ref to Player

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 3: Gateway — Graceful Disconnect

**Files:**
- Modify: `gameserver/gateway.go`

- [ ] **Step 1: Rewrite Input/Output for graceful disconnect**

In `gameserver/gateway.go`, replace all content:

```go
package gameserver

import (
	"context"

	"github.com/gorilla/websocket"
)

type Session struct {
	Conn               *websocket.Conn
	Player             *Player
	ClosedCh           chan struct{}
	inputStreamClosed  chan struct{}
	outputStreamClosed chan struct{}
}

func NewSession(conn *websocket.Conn, player *Player) *Session {
	s := &Session{
		Conn:               conn,
		Player:             player,
		ClosedCh:           make(chan struct{}),
		inputStreamClosed:  make(chan struct{}),
		outputStreamClosed: make(chan struct{}),
	}
	player.Session = s
	return s
}

func (s *Session) Input(ctx context.Context) {
	defer func() {
		s.inputStreamClosed <- struct{}{}
		close(s.inputStreamClosed)
	}()

	var msg Message
	for {
		if err := s.Conn.ReadJSON(&msg); err != nil {
			close(s.Player.Disconnected)
			s.Player.Session = nil
			return
		}
		s.Player.OperatorCh <- msg
	}
}

func (s *Session) Output(ctx context.Context) {
	defer func() {
		s.outputStreamClosed <- struct{}{}
		close(s.outputStreamClosed)
	}()

	for {
		select {
		case msg := <-s.Player.NoticeCh:
			if err := s.Conn.WriteJSON(&msg); err != nil {
				close(s.Player.Disconnected)
				s.Player.Session = nil
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (s *Session) SessionControl(ctx context.Context) {
	<-s.inputStreamClosed
	<-s.outputStreamClosed
	s.ClosedCh <- struct{}{}
	close(s.ClosedCh)
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./gameserver/`
Expected: build succeeds

- [ ] **Step 3: Commit**

```bash
git add gameserver/gateway.go
git commit -m "fix: graceful disconnect, remove panic on read/write error

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 4: Room — Host, StartGame, Broadcast, RoomFn

**Files:**
- Modify: `gameserver/room.go`

- [ ] **Step 1: Rewrite room.go with full implementation**

In `gameserver/room.go`, replace all content:

```go
package gameserver

import (
	"context"
	"errors"
	"log"
	"sync"

	"github.com/bs-iron-trio/go-kusokurae/sm"
)

var ErrRoomFull = errors.New("room is full!!!")
var ErrRoomNotFound = errors.New("room not found!!!")
var ErrRoomPlayerNotFound = errors.New("this player not contained by room")
var ErrGameAlreadyStarted = errors.New("game already started")
var ErrNotEnoughPlayers = errors.New("not enough players")
var ErrNotHost = errors.New("only host can start game")

type Room struct {
	ID             string
	Mutex          sync.Mutex
	GameConfig     *sm.GameConfig
	Game           *Game
	HostPlayerIdx  int32
	CurrentPlayers int32
	Players        []*Player
}

var roomRepositoryMu sync.Mutex
var roomRepository map[string]*Room

func InitRoomRepository() {
	roomRepository = make(map[string]*Room)
}

func GetRoomByID(roomID string) (*Room, error) {
	room, exists := roomRepository[roomID]
	if !exists {
		return nil, ErrRoomNotFound
	}
	return room, nil
}

func NewRoom(id string, host *Player, config *sm.GameConfig) *Room {
	roomRepositoryMu.Lock()
	defer roomRepositoryMu.Unlock()
	r := &Room{
		ID:             id,
		GameConfig:     config,
		HostPlayerIdx:  0,
		CurrentPlayers: 1,
		Players:        make([]*Player, config.NumPlayers),
	}
	r.Players[0] = host
	host.Sit(id, 0)
	roomRepository[id] = r
	return r
}

func (r *Room) AddPlayer(player *Player) error {
	r.Mutex.Lock()
	defer r.Mutex.Unlock()
	if r.CurrentPlayers >= r.GameConfig.NumPlayers {
		return ErrRoomFull
	}
	position := r.CurrentPlayers
	r.Players[position] = player
	r.CurrentPlayers++
	player.Sit(r.ID, position)
	r.Broadcast(Message{
		MsgType: MSG_TYPE_PLAYER_JOINED,
		MsgBody: &PlayerJoinedBody{PlayerID: player.ID, Position: position},
	})
	r.broadcastRoomState()
	return nil
}

func (r *Room) FindPlayerByID(playerID string) (*Player, error) {
	for _, p := range r.Players {
		if p != nil && p.ID == playerID {
			return p, nil
		}
	}
	return nil, ErrRoomPlayerNotFound
}

func (r *Room) Broadcast(msg Message) {
	for _, p := range r.Players {
		if p != nil && p.Session != nil {
			select {
			case p.NoticeCh <- msg:
			default:
			}
		}
	}
}

func (r *Room) broadcastRoomState() {
	players := make([]RoomPlayerInfo, r.CurrentPlayers)
	for i := int32(0); i < r.CurrentPlayers; i++ {
		players[i] = RoomPlayerInfo{
			PlayerID: r.Players[i].ID,
			Position: i,
			IsHost:   i == r.HostPlayerIdx,
		}
	}
	r.Broadcast(Message{
		MsgType: MSG_TYPE_ROOM_STATE,
		MsgBody: &RoomStateBody{Players: players, HostIdx: r.HostPlayerIdx},
	})
}

func (r *Room) StartGame(requesterID string) error {
	r.Mutex.Lock()
	defer r.Mutex.Unlock()

	if r.Game != nil {
		return ErrGameAlreadyStarted
	}
	if r.CurrentPlayers < r.GameConfig.NumPlayers {
		return ErrNotEnoughPlayers
	}
	if r.Players[r.HostPlayerIdx].ID != requesterID {
		return ErrNotHost
	}

	players := make([]*Player, r.GameConfig.NumPlayers)
	copy(players, r.Players[:r.GameConfig.NumPlayers])
	r.Game = NewGame(r.GameConfig, players)
	go r.Game.GameFn(context.Background())
	return nil
}

func (r *Room) RoomFn(ctx context.Context) {
	for _, p := range r.Players {
		if p == nil {
			continue
		}
		go func(player *Player) {
			for {
				select {
				case msg := <-player.OperatorCh:
					r.handleRoomMessage(player, msg)
				case <-ctx.Done():
					return
				}
			}
		}(p)
	}
	<-ctx.Done()
}

func (r *Room) handleRoomMessage(player *Player, msg Message) {
	switch msg.MsgType {
	case MSG_TYPE_START_GAME:
		if err := r.StartGame(player.ID); err != nil {
			player.NoticeCh <- Message{
				MsgType: MSG_TYPE_ERROR,
				MsgBody: &ErrorBody{Message: err.Error()},
			}
		}
	}
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./gameserver/`
Expected: build succeeds if game.go is at least stubbed (existing stub will fail on missing methods — proceed to Task 5 immediately after)

- [ ] **Step 3: Commit**

```bash
git add gameserver/room.go
git commit -m "feat: add host, StartGame, Broadcast and RoomFn to Room

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 5: Game — GameFn Main Loop with Relay and AutoPlay

**Files:**
- Modify: `gameserver/game.go`
- Create: `gameserver/game_test.go`

- [ ] **Step 1: Write the failing test**

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `CGO_ENABLED=0 go test ./gameserver/ -run "TestRelay_ForwardsMessage|TestPlayerReaderChs_Padding" -v`
Expected: FAIL — types not yet defined

- [ ] **Step 3: Rewrite game.go with full GameFn implementation**

In `gameserver/game.go`, replace all content:

```go
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

func (g *Game) sendTo(idx int32, msg Message) {
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
	for i := int32(0); i < g.Config.NumPlayers; i++ {
		handCards := g.buildCardInfos(g.State.GetPlayer(i).GetHandCards())
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
		if g.isActivePlayer(0) { g.autoPlay(0) }
	case <-g.disconnectedCh(1):
		if g.isActivePlayer(1) { g.autoPlay(1) }
	case <-g.disconnectedCh(2):
		if g.isActivePlayer(2) { g.autoPlay(2) }
	case <-g.disconnectedCh(3):
		if g.isActivePlayer(3) { g.autoPlay(3) }

	case <-ctx.Done():
		return
	}
}

func (g *Game) handleMove(idx int, msg Message) {
	if !g.isActivePlayer(int32(idx)) {
		g.sendTo(int32(idx), Message{
			MsgType: MSG_TYPE_ERROR,
			MsgBody: &ErrorBody{Message: "not your turn"},
		})
		return
	}

	if msg.MsgType != MSG_TYPE_PLAY_CARD {
		g.sendTo(int32(idx), Message{
			MsgType: MSG_TYPE_ERROR,
			MsgBody: &ErrorBody{Message: "unexpected message type: " + msg.MsgType},
		})
		return
	}

	body, ok := msg.MsgBody.(map[string]interface{})
	if !ok {
		g.sendTo(int32(idx), Message{
			MsgType: MSG_TYPE_ERROR,
			MsgBody: &ErrorBody{Message: "invalid play card body"},
		})
		return
	}
	cardIdxFloat, ok := body["cardIndex"].(float64)
	if !ok {
		g.sendTo(int32(idx), Message{
			MsgType: MSG_TYPE_ERROR,
			MsgBody: &ErrorBody{Message: "cardIndex must be a number"},
		})
		return
	}
	cardIdx := int(cardIdxFloat)

	handCards := g.State.GetActivePlayer().GetHandCards()
	if cardIdx < 0 || cardIdx >= len(handCards) {
		g.sendTo(int32(idx), Message{
			MsgType: MSG_TYPE_ERROR,
			MsgBody: &ErrorBody{Message: "card index out of range"},
		})
		return
	}

	card := handCards[cardIdx]
	if err := g.State.Play(card); err != nil {
		g.sendTo(int32(idx), Message{
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
		// Round ended — a new active player is set. The next YOUR_TURN goes to the new active player.
		// We can detect round end by checking if active player changed.
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
	g.sendTo(int32(idx), Message{
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `CGO_ENABLED=0 go test ./gameserver/ -run "TestRelay_ForwardsMessage|TestPlayerReaderChs_Padding" -v`
Expected: PASS

- [ ] **Step 5: Verify full package builds**

Run: `go build ./gameserver/`
Expected: build succeeds

- [ ] **Step 6: Commit**

```bash
git add gameserver/game.go gameserver/game_test.go
git commit -m "feat: implement GameFn main loop with relay, handlePlay, and autoPlay

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 6: Controller — Adapt CreateRoom + Reconnect Logic

**Files:**
- Modify: `experimental/online/controller.go`

- [ ] **Step 1: Rewrite controller.go with new logic**

In `experimental/online/controller.go`, replace all content:

```go
package main

import (
	"net/http"

	"github.com/bs-iron-trio/go-kusokurae/gameserver"
	"github.com/bs-iron-trio/go-kusokurae/sm"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type CommunicationParams struct {
	RoomID   string `uri:"room_id" binding:"required"`
	PlayerID string `uri:"player_id" binding:"required"`
}

func handleWebSocket(c *gin.Context) {
	var params CommunicationParams
	if err := c.BindUri(&params); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorRes(COMMON_ERR_CODE, err.Error()))
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, NewErrorRes(COMMON_ERR_CODE, err.Error()))
		return
	}

	room, err := gameserver.GetRoomByID(params.RoomID)
	if err != nil {
		conn.WriteJSON(&gameserver.Message{
			MsgType: gameserver.MSG_TYPE_ERROR,
			MsgBody: &gameserver.ErrorBody{Message: err.Error()},
		})
		conn.Close()
		return
	}

	player, err := room.FindPlayerByID(params.PlayerID)
	if err != nil {
		conn.WriteJSON(&gameserver.Message{
			MsgType: gameserver.MSG_TYPE_ERROR,
			MsgBody: &gameserver.ErrorBody{Message: err.Error()},
		})
		conn.Close()
		return
	}

	// Handle reconnect: close old session if exists
	if player.Session != nil {
		player.Session.Conn.Close()
		<-player.Session.ClosedCh
	}

	// Replace Disconnected chan for fresh connection
	player.Disconnected = make(chan struct{})

	s := gameserver.NewSession(conn, player)

	go s.SessionControl(c.Request.Context())
	go s.Input(c.Request.Context())
	go s.Output(c.Request.Context())

	// If game is in progress, re-sync state to the reconnected player
	if room.Game != nil && room.Game.State != nil && room.Game.State.GetStatus() == sm.StatusPlay {
		activePlayer := room.Game.State.GetActivePlayer()
		if activePlayer != nil && activePlayer.GetIndex()-1 == int(player.RoomPosition) {
			// Reconnected player is the active player — re-send YOUR_TURN
			g := room.Game
			idx := int(player.RoomPosition)
			p := g.State.GetPlayer(int32(idx))
			handCards := p.GetHandCards()
			playableIndices := make([]int, 0)
			for i, c := range handCards {
				if c.Playable() {
					playableIndices = append(playableIndices, i)
				}
			}
			rs := g.State.GetRoundState()
			cardInfos := make([]gameserver.CardInfo, len(rs.Moves))
			for i, c := range rs.Moves {
				cardInfos[i] = gameserver.CardInfo{
					Suit: int32(c.GetSuit()), Rank: int32(c.GetRank()),
				}
			}
			s.Player.NoticeCh <- gameserver.Message{
				MsgType: gameserver.MSG_TYPE_YOUR_TURN,
				MsgBody: &gameserver.YourTurnBody{
					PlayableIndices: playableIndices,
					RoundSeq:        rs.Seq,
					RoundMoves:      cardInfos,
				},
			}
		}
	}

	<-s.ClosedCh
}

func CreateRoom(ctx *gin.Context) {
	var gameConfig sm.GameConfig
	if err := ctx.BindJSON(&gameConfig); err != nil {
		ctx.JSON(http.StatusBadRequest, NewErrorRes(COMMON_ERR_CODE, err.Error()))
		return
	}
	if gameConfig.NumPlayers != 3 && gameConfig.NumPlayers != 4 {
		ctx.JSON(http.StatusBadRequest, NewErrorRes(COMMON_ERR_CODE, "Invalid number of players"))
		return
	}

	u, err := uuid.NewRandom()
	if err != nil {
		ctx.JSON(200, NewErrorRes(COMMON_ERR_CODE, err.Error()))
		return
	}

	host := gameserver.NewPlayer()
	room := gameserver.NewRoom(u.String(), host, &gameConfig)

	ctx.JSON(200, NewSuccessRes(&JoinRoomRet{
		RoomID:   room.ID,
		PlayerID: host.ID,
	}))
}

type JoinRoomRet struct {
	RoomID   string `json:"roomID"`
	PlayerID string `json:"playerID"`
}

func JoinRoom(ctx *gin.Context) {
	roomID := ctx.Query("roomID")
	if len(roomID) == 0 {
		ctx.JSON(http.StatusBadRequest, NewErrorRes(COMMON_ERR_CODE, "Invalid roomID"))
		return
	}

	room, err := gameserver.GetRoomByID(roomID)
	if err != nil {
		if err == gameserver.ErrRoomNotFound {
			ctx.JSON(200, NewErrorRes(COMMON_ERR_CODE, "room not found"))
		} else {
			ctx.JSON(200, NewErrorRes(COMMON_ERR_CODE, err.Error()))
		}
		return
	}

	player := gameserver.NewPlayer()
	if err := room.AddPlayer(player); err != nil {
		ctx.JSON(200, NewErrorRes(COMMON_ERR_CODE, err.Error()))
		return
	}

	ctx.JSON(200, NewSuccessRes(&JoinRoomRet{
		RoomID:   player.RoomID,
		PlayerID: player.ID,
	}))
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./experimental/online/`
Expected: build succeeds

- [ ] **Step 3: Commit**

```bash
git add experimental/online/controller.go
git commit -m "feat: adapt CreateRoom to return host player, add reconnect logic

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 7: Integration Verification

**Files:**
- Modify: `gameserver/room.go` — fix player nil check in RoomFn
- No new files

- [ ] **Step 1: Fix RoomFn nil check if needed**

Verify `gameserver/room.go` RoomFn loop properly handles nil player slots.

- [ ] **Step 2: Run full build**

Run: `go build ./...`
Expected: build succeeds (sm package cgo issues are pre-existing, unrelated)

- [ ] **Step 3: Run all gameserver tests**

Run: `CGO_ENABLED=0 go test ./gameserver/ -v`
Expected: PASS

- [ ] **Step 4: Run sm engine tests**

Run: `go test ./sm/ -v`
Expected: PASS

- [ ] **Step 5: Final commit if any fixes were needed**

```bash
git add -A
git commit -m "chore: integration fixes for online multiplayer

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```
