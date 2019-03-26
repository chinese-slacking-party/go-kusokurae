package sm

/*
#cgo CFLAGS: -DWHATEVER_YOU_WANT_TO_INDICATE_CGO=1
#cgo CXXFLAGS: -DWHATEVER_YOU_WANT_TO_INDICATE_CGO=1
#include "sm.h"

extern void goRandom(int16_t *);
static inline int16_t cgo_random(void *state) {
	int16_t ret;
	goRandom(&ret);
	return ret;
}
static inline void set_prng() {
	kusokurae_set_prng(&cgo_random);
}

extern void goGameStateCB(kusokurae_game_state_t *, int32_t, void *);
static inline void cgo_game_state_cb(kusokurae_game_state_t *self, int32_t newstate, void *userdata) {
	goGameStateCB(self, newstate, userdata);
}
static inline void *get_cgo_cb_bridge_ptr() {
	return &cgo_game_state_cb;
}
*/
import "C"

import (
	"errors"
	"math/rand"
	"runtime"
	"sync/atomic"
	"time"
	"unsafe"
)

//export goRandom
func goRandom(out *C.int16_t) {
	rnd := int16(rand.Intn(0x7FFF))
	*out = C.int16_t(rnd)
}

//export goGameStateCB
func goGameStateCB(self *C.kusokurae_game_state_t, newstate C.int32_t, userdata unsafe.Pointer) {
	if self == nil {
		// TODO: bugcheck?
		return
	}
	// userdata is not used
	obj := (*GameState)(unsafe.Pointer(self))
	if obj.goStateCallbackNo > 0 {
		callbackMap[obj.goStateCallbackNo](GameStatus(newstate))
	}
}

func init() {
	C.kusokurae_global_init()
	rand.Seed(time.Now().UnixNano())
	C.set_prng()
	callbackMap = make(map[int32]func(GameStatus))
}

var cbs = GameCallbacks{
	StateTransition: uintptr(C.get_cgo_cb_bridge_ptr()),
}

// GameStatus is equivalent to kusokurae_game_status_t.
type GameStatus int32

// GameStatus values.
const (
	StatusNull GameStatus = iota
	StatusInit
	StatusPlay
	StatusFinish

	StatusMax
)

// Suit is equivalent to C.kusokurae_card_suit_t.
type Suit int32

// Suit values.
const (
	SuitUnknown Suit = -2 // Not in C enum definition
	SuitXiang        = -1
	SuitYoutiao      = 0
	SuitBaozi        = 1
	SuitOther        = 2
)

// RoundStatus is equivalent to C.kusokurae_round_status_t.
type RoundStatus int32

const (
	RoundWaiting RoundStatus = iota
	RoundActive
	RoundDone
)

// Errors from underlying library.
// Don't forget also to change here after adding new error codes in C interface.
var (
	ErrNullPtr         = errors.New("KUSOKURAE_ERROR_NULLPTR")
	ErrBadNPlayers     = errors.New("KUSOKURAE_ERROR_BAD_NUMBER_OF_PLAYERS")
	ErrUninitialized   = errors.New("KUSOKURAE_ERROR_UNINITIALIZED")
	ErrNotInGame       = errors.New("KUSOKURAE_ERROR_NOT_IN_GAME")
	ErrBugNobodyActive = errors.New("KUSOKURAE_ERROR_BUG_NOBODY_ACTIVE")
	ErrCardNotFound    = errors.New("KUSOKURAE_ERROR_CARD_NOT_FOUND")
	ErrForbiddenMove   = errors.New("KUSOKURAE_ERROR_FORBIDDEN_MOVE")

	ErrUnknown = errors.New("Unknown")
)

var errMap = map[C.kusokurae_error_t]error{
	C.KUSOKURAE_SUCCESS:                     nil,
	C.KUSOKURAE_ERROR_NULLPTR:               ErrNullPtr,
	C.KUSOKURAE_ERROR_BAD_NUMBER_OF_PLAYERS: ErrBadNPlayers,
	C.KUSOKURAE_ERROR_UNINITIALIZED:         ErrUninitialized,
	C.KUSOKURAE_ERROR_NOT_IN_GAME:           ErrNotInGame,
	C.KUSOKURAE_ERROR_BUG_NOBODY_ACTIVE:     ErrBugNobodyActive,
	C.KUSOKURAE_ERROR_CARD_NOT_FOUND:        ErrCardNotFound,
	C.KUSOKURAE_ERROR_FORBIDDEN_MOVE:        ErrForbiddenMove,
}

// GameConfig has the same memory layout with C.kusokurae_game_config_t.
type GameConfig struct {
	NumPlayers int32
}

// GameCallbacks has the same memory layout with C.kusokurae_game_callbacks_t.
type GameCallbacks struct {
	UserDataOfStateTransition int // void * in C made int for convenience
	StateTransition           uintptr
}

// Card has the same memory layout with C.kusokurae_card_t.
type Card struct {
	displayOrder uint32
	suit         Suit
	rank         int32
	flags        uint32
}

// Player has the same memory layout with C.kusokurae_player_t.
type Player struct {
	index      int32
	active     RoundStatus
	allCards   [C.KUSOKURAE_MAX_HAND_CARDS]Card
	numCards   int32
	cardsTaken int32
	score      int32
}

// GameState has the same memory layout with C.kusokurae_game_state_t.
type GameState struct {
	cfg         GameConfig
	status      GameStatus
	players     [C.KUSOKURAE_MAX_PLAYERS]Player
	numRound    int32
	ghostHolder int32
	highRanker  int32
	curRound    [C.KUSOKURAE_MAX_PLAYERS]Card
	rngState    int64
	cbs         GameCallbacks

	// Extra fields for Go library users go here
	// ------- -------
	// We can't put actual function variable here, because runtime will complain
	// about cgo argument containing Go pointer.
	goStateCallbackNo int32
}

var (
	nextCBNo    int32
	callbackMap map[int32]func(GameStatus)
)

// RoundState corresponds to C.kusokurae_round_state_t, but does not preserve
// its memory layout - a bit of conversion needs to be done when providing this
// to user Go code.
type RoundState struct {
	Seq          int
	IsDoubled    bool
	ScoreOnBoard int
	RoundWinner  *Player
}

func errcode2Go(code C.kusokurae_error_t) (err error) {
	err, ok := errMap[code]
	if !ok {
		err = ErrUnknown
	}
	return
}

// GetSuit returns the Card's intrinsic value.
func (p *Card) GetSuit() Suit {
	return p.suit
}

// GetRank returns the Card's relative power.
func (p *Card) GetRank() int {
	return int(p.rank)
}

// Playable checks whether the card is legal move now.
func (p *Card) Playable() bool {
	return (C.kusokurae_card_is_playable(*(*C.kusokurae_card_t)(unsafe.Pointer(p))) != 0)
}

// RoundPlayed returns the round number in which the card is played, or 0 if it
// is still in hand.
func (p *Card) RoundPlayed() int {
	return int(C.kusokurae_card_round_played(*(*C.kusokurae_card_t)(unsafe.Pointer(p))))
}

// GetCards returns a slice holding the player's cards. It operates in constant
// time.
func (p *Player) GetCards() []Card {
	return p.allCards[0:p.numCards]
}

// GetHandCards return the player's cards IN HAND (not played yet).
func (p *Player) GetHandCards() (ret []Card) {
	for i := 0; i < int(p.numCards); i++ {
		if p.allCards[i].RoundPlayed() == 0 {
			ret = append(ret, p.allCards[i])
		}
	}
	return
}

// NewGame creates a new game state with specified number of players.
func NewGame(cfg GameConfig, stateFn func(GameStatus)) (ret *GameState, err error) {
	ret = &GameState{}
	err = ret.init(cfg, stateFn)
	return
}

func (g *GameState) init(cfg GameConfig, stateFn func(GameStatus)) error {
	var cbNo int32
	if stateFn != nil {
		cbNo = atomic.AddInt32(&nextCBNo, 1)
		callbackMap[cbNo] = stateFn
	}
	g.goStateCallbackNo = cbNo
	runtime.SetFinalizer(g, func(g *GameState) {
		delete(callbackMap, g.goStateCallbackNo)
	})
	pret := unsafe.Pointer(g)
	pcfg := unsafe.Pointer(&cfg)
	pcbs := unsafe.Pointer(&cbs)
	return errcode2Go(C.kusokurae_game_init(
		(*C.kusokurae_game_state_t)(pret),
		(*C.kusokurae_game_config_t)(pcfg),
		(*C.kusokurae_game_callbacks_t)(pcbs),
	))
}

func (g *GameState) cPtr() *C.kusokurae_game_state_t {
	return (*C.kusokurae_game_state_t)(unsafe.Pointer(g))
}

// Start deals cards to each player and begins waiting for play from the first
// player.
func (g *GameState) Start() (err error) {
	err = errcode2Go(C.kusokurae_game_start(g.cPtr()))
	return
}

// IsFinalRound checks if the game is in (or after) its last round.
func (g *GameState) IsFinalRound() bool {
	if C.kusokurae_game_is_final_round(g.cPtr()) != 0 {
		return true
	}
	return false
}

// GetActivePlayer returns the player whose turn it is now, or nil if the game
// is not in progress.
func (g *GameState) GetActivePlayer() *Player {
	cActivePlayer := C.kusokurae_get_active_player(g.cPtr())
	return (*Player)(unsafe.Pointer(cActivePlayer))
}

// GetPlayer returns the player with specified index, or nil if index is out of
// range.
func (g *GameState) GetPlayer(index int32) *Player {
	if index < 0 || index >= g.cfg.NumPlayers {
		return nil
	}
	return &g.players[index]
}

// GetRoundState returns some useful info about the current round.
func (g *GameState) GetRoundState() (ret RoundState) {
	var cRoundState C.kusokurae_round_state_t
	C.kusokurae_get_round_state(g.cPtr(), (*C.kusokurae_round_state_t)(unsafe.Pointer(&cRoundState)))
	ret.IsDoubled = (cRoundState.is_doubled != 0)
	if cRoundState.round_winner < 0 || int32(cRoundState.round_winner) >= g.cfg.NumPlayers {
		// Array bound check
	} else {
		ret.RoundWinner = &g.players[cRoundState.round_winner]
	}
	ret.ScoreOnBoard = int(cRoundState.score_on_board)
	ret.Seq = int(cRoundState.seq)
	return
}

// Play plays a card for the active player and return the operation result.
func (g *GameState) Play(move Card) error {
	return errcode2Go(C.kusokurae_game_play(g.cPtr(), *(*C.kusokurae_card_t)(unsafe.Pointer(&move))))
}
