package sm

// Exposes the C compiler's view of the shared struct layout so layout_test.go
// can compare it against Go's. Nothing here is used by the library itself; the
// test_ prefix says so, since cgo is not allowed in a _test.go file.

/*
#include <stddef.h>
#include "sm.h"

static size_t sz_card(void)   { return sizeof(kusokurae_card_t); }
static size_t sz_player(void) { return sizeof(kusokurae_player_t); }
static size_t sz_config(void) { return sizeof(kusokurae_game_config_t); }
static size_t sz_cbs(void)    { return sizeof(kusokurae_game_callbacks_t); }
static size_t sz_state(void)  { return sizeof(kusokurae_game_state_t); }

static size_t off_p_index(void)      { return offsetof(kusokurae_player_t, index); }
static size_t off_p_active(void)     { return offsetof(kusokurae_player_t, active); }
static size_t off_p_cards(void)      { return offsetof(kusokurae_player_t, cards); }
static size_t off_p_ncards(void)     { return offsetof(kusokurae_player_t, ncards); }
static size_t off_p_taken(void)      { return offsetof(kusokurae_player_t, cards_taken); }
static size_t off_p_score(void)      { return offsetof(kusokurae_player_t, score); }
static size_t off_p_busted(void)     { return offsetof(kusokurae_player_t, busted); }

static size_t off_g_cfg(void)        { return offsetof(kusokurae_game_state_t, cfg); }
static size_t off_g_status(void)     { return offsetof(kusokurae_game_state_t, status); }
static size_t off_g_players(void)    { return offsetof(kusokurae_game_state_t, players); }
static size_t off_g_nround(void)     { return offsetof(kusokurae_game_state_t, nround); }
static size_t off_g_ghost(void)      { return offsetof(kusokurae_game_state_t, ghost_holder_index); }
static size_t off_g_highranker(void) { return offsetof(kusokurae_game_state_t, high_ranker_index); }
static size_t off_g_curround(void)   { return offsetof(kusokurae_game_state_t, current_round); }
static size_t off_g_rngstate(void)   { return offsetof(kusokurae_game_state_t, rng_state); }
static size_t off_g_cbs(void)        { return offsetof(kusokurae_game_state_t, cbs); }

static size_t save_bytes(void)       { return KUSOKURAE_SAVE_BYTES; }
*/
import "C"

// cLayout is what the C compiler decided, read back through cgo.
var cLayout = struct {
	SizeofCard, SizeofPlayer, SizeofConfig, SizeofCallbacks, SizeofState uintptr

	PlayerIndex, PlayerActive, PlayerCards, PlayerNCards,
	PlayerCardsTaken, PlayerScore, PlayerBusted uintptr

	StateCfg, StateStatus, StatePlayers, StateNRound, StateGhost,
	StateHighRanker, StateCurRound, StateRNGState, StateCbs uintptr

	SaveBytes uintptr
}{
	SizeofCard:      uintptr(C.sz_card()),
	SizeofPlayer:    uintptr(C.sz_player()),
	SizeofConfig:    uintptr(C.sz_config()),
	SizeofCallbacks: uintptr(C.sz_cbs()),
	SizeofState:     uintptr(C.sz_state()),

	PlayerIndex:      uintptr(C.off_p_index()),
	PlayerActive:     uintptr(C.off_p_active()),
	PlayerCards:      uintptr(C.off_p_cards()),
	PlayerNCards:     uintptr(C.off_p_ncards()),
	PlayerCardsTaken: uintptr(C.off_p_taken()),
	PlayerScore:      uintptr(C.off_p_score()),
	PlayerBusted:     uintptr(C.off_p_busted()),

	StateCfg:        uintptr(C.off_g_cfg()),
	StateStatus:     uintptr(C.off_g_status()),
	StatePlayers:    uintptr(C.off_g_players()),
	StateNRound:     uintptr(C.off_g_nround()),
	StateGhost:      uintptr(C.off_g_ghost()),
	StateHighRanker: uintptr(C.off_g_highranker()),
	StateCurRound:   uintptr(C.off_g_curround()),
	StateRNGState:   uintptr(C.off_g_rngstate()),
	StateCbs:        uintptr(C.off_g_cbs()),

	SaveBytes: uintptr(C.save_bytes()),
}
