#ifndef BS_KUSOKURAE_SM_H
#define BS_KUSOKURAE_SM_H

#include <assert.h>
#include <limits.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

#define KUSOKURAE_DECK_SIZE         33
#define KUSOKURAE_MAX_HAND_CARDS    22
#define KUSOKURAE_MAX_PLAYERS       4

// Inclusive upper bound of the values a random number generator must
// produce. See kusokurae_set_prng().
#define KUSOKURAE_RAND_MAX          32767

// The generator returns int, so the range asked of it has to fit there. C
// only guarantees INT_MAX >= 32767, which is exactly why the standard library
// sets RAND_MAX >= 32767 and no higher: the bound above sits on that floor
// and is portable everywhere, including an implementation with a 16-bit int.
// Raising it would quietly depend on a wider one.
//
// static_assert rather than _Static_assert: <assert.h> defines it as a macro
// in C11 and later, and it is a keyword in C++11 and later, so this one
// spelling works for every consumer of this header.
static_assert(KUSOKURAE_RAND_MAX <= INT_MAX,
              "KUSOKURAE_RAND_MAX does not fit the int the PRNG returns");

struct kusokurae_game_state_t; // Forward declaration

typedef void (*state_transition_cb)(struct kusokurae_game_state_t *self, int32_t newstate, void *userdata);

typedef struct {
    int32_t np;               // Number of players (3 or 4)
    int32_t first_player_idx; // 0-based first-round leader; mod np at start
} kusokurae_game_config_t;

typedef struct {
    // State transition callback - to be called BEFORE each state change and end
    // of each round.
    void *userdata_of_state_transition;
    state_transition_cb state_transition;
} kusokurae_game_callbacks_t;

typedef enum {
    // 0 - Zero value
    KUSOKURAE_STATUS_NULL,

    // 1 - Struct initialized
    KUSOKURAE_STATUS_INIT,

    // 2 - Game in progress
    KUSOKURAE_STATUS_PLAY,

    // 3 - Game finished (you can retrieve results and/or start a new game)
    KUSOKURAE_STATUS_FINISH,

    // Keep this line at the bottom
    KUSOKURAE_STATUS_MAX,
} kusokurae_game_status_t;

typedef enum {
    // lit. "Shit"
    KUSOKURAE_SUIT_XIANG = -1,

    // lit. "Fried bread stick"
    KUSOKURAE_SUIT_YOUTIAO = 0,

    // lit. "Stuffed bun"
    KUSOKURAE_SUIT_BAOZI = 1,

    // In all the suits above, the number -1, 0 & 1 equal their values in a game,
    // but the following OTHER card type should be treated specially.
    KUSOKURAE_SUIT_OTHER = 2,
} kusokurae_card_suit_t;

typedef struct {
    // Position in the new, unshuffled deck, and the order a hand is laid out
    // in. Higher value precedes lower. This is presentation only -- a trick
    // is decided by rank alone, so the Ghost outranks the Angels while
    // sitting behind them here, at 31 against their 33 and 32.
    // 0 indicates invalid data (unfilled slot).
    // Should be filled during global initialization and copied afterwards.
    uint32_t display_order;

    // Declared above (kusokurae_card_suit_t)
    int32_t suit;

    // 0~9 for the numbered cards of every suit, 10 for the two Angels (which
    // are BAOZI, so they also score +1), and 11 for the Ghost, which beats
    // everything.
    int32_t rank;

    // Bits 0~6: round index (counting from 1) in which the card is played
    // Bit 7: whether the card could be played in the current round
    // Bits 8~31: reserved
    uint32_t flags;
} kusokurae_card_t;

typedef enum {
    KUSOKURAE_ROUND_WAITING,
    KUSOKURAE_ROUND_ACTIVE,
    KUSOKURAE_ROUND_DONE,
} kusokurae_round_status_t;

typedef struct {
    // 1~4 (0 for invalid)
    int32_t index;

    // 1 - active (playing), 2 - already played
    int32_t active;

    // Everything dealt to this player, played and unplayed alike, in deck
    // order. 22 slots (reserved for playing with 2 decks).
    //
    // A played card is not removed: it stays in place and its flags record
    // the round it went out in, which is what makes the whole game
    // reconstructible from the final state. Use
    // kusokurae_card_round_played() to tell the two apart.
    kusokurae_card_t cards[KUSOKURAE_MAX_HAND_CARDS];

    // How many of those slots were dealt. Fixed for the whole game, since
    // nothing is ever removed -- the engine relies on that and reads it as
    // "cards dealt per player" when deciding the game is over.
    int32_t ncards;

    // If the player wins a round, he/she takes all cards played in that round.
    // cards_taken will always be multiples of player count.
    int32_t cards_taken;

    // The score accumulated from cards_taken.
    int32_t score;

    // When you say a player is busted, it means he/she is forced to play
    // forbidden moves because no other card's available.
    int32_t busted;
} kusokurae_player_t;

typedef enum {
    KUSOKURAE_SUCCESS,
    KUSOKURAE_ERROR_NULLPTR,
    KUSOKURAE_ERROR_BAD_NUMBER_OF_PLAYERS,
    KUSOKURAE_ERROR_UNINITIALIZED,
    KUSOKURAE_ERROR_NOT_IN_GAME,
    KUSOKURAE_ERROR_BUG_NOBODY_ACTIVE,
    KUSOKURAE_ERROR_CARD_NOT_FOUND,
    KUSOKURAE_ERROR_FORBIDDEN_MOVE,

    KUSOKURAE_ERROR_UNIMPLEMENTED,
    KUSOKURAE_ERROR_UNSPECIFIED,
} kusokurae_error_t;

typedef struct kusokurae_game_state_t {
    kusokurae_game_config_t cfg;
    int32_t status;

    // Max 4 players
    kusokurae_player_t players[KUSOKURAE_MAX_PLAYERS];

    // Finished round count
    int32_t nround;

    // Who has the ghost in hand?
    int32_t ghost_holder_index;

    // Rank leader in the current round.
    // Set to -1 before anyone plays and updated on each play.
    int32_t high_ranker_index;

    // Cards played in the current round.
    // players[n]'s move is placed in current_round[n].
    kusokurae_card_t current_round[KUSOKURAE_MAX_PLAYERS];

    // 8 bytes of state for the random number generator, private to this
    // game. Keeping the state here rather than in a global is what lets one
    // game state per room, each driven by a single thread, run without any
    // locking. kusokurae_game_init() seeds the low 32 bits of it; a
    // replacement generator may use all 8 bytes however it likes.
    uint64_t rng_state;

    // Game-specific callbacks should be put at the bottom, because their sizes
    // are machine-dependent.
    kusokurae_game_callbacks_t cbs;
} kusokurae_game_state_t;

typedef struct {
    // On screen: "Round <seq>"
    int32_t seq;

    // Whether there is a ghost
    int32_t is_doubled;

    // Total score in cards played
    int32_t score_on_board;

    // The current winning player
    int32_t round_winner;

    // Moves made in this round, ordered chronologically (e.g. if there're 3
    // players and the trick leader is 2P, then moves[0] is 2P's move, moves[1]
    // is 3P's move, moves[2] is 1P's move, and moves[3] is unused)
    kusokurae_card_t moves[KUSOKURAE_MAX_PLAYERS];
} kusokurae_round_state_t;

void kusokurae_global_init();

// kusokurae_set_prng installs the generator used to deal cards. fn receives
// a pointer to the rng_state field of the game being dealt, and must return
// a uniformly distributed value in the closed range [0, KUSOKURAE_RAND_MAX].
// Passing NULL leaves the current generator in place.
//
// The return type is int rather than a type exactly as wide as the range,
// mirroring the C library, where rand() returns int and RAND_MAX carries the
// range on its own. That states the range in one place instead of two that
// can disagree, leaves room to raise KUSOKURAE_RAND_MAX without touching
// this signature, and lets a generator covering [0, 65535] say so instead of
// silently handing back negative numbers.
//
// Return values outside the range are not rejected. Dealing stays memory safe
// and every player still receives the right number of cards -- sample() in
// sm.c bounds the selection so that holds for any generator -- but the deal
// is no longer uniform: values below the range are always taken and values
// above it are always passed over.
//
// The C library's rand() is deliberately not used: it keeps one global state
// and is not thread safe. The convention here is one game state per room,
// each driven by a single thread, so the generator is handed per-game state
// instead and games never contend. A replacement generator is expected to
// honour that and keep whatever it needs in the pointer it is given rather
// than in globals -- or, like the Go binding does, to be safe to call from
// several games at once.
void kusokurae_set_prng(int (*fn)(void *));

kusokurae_error_t kusokurae_game_init(kusokurae_game_state_t *self,
                                      kusokurae_game_config_t *cfg,
                                      kusokurae_game_callbacks_t *cbs);

kusokurae_error_t kusokurae_game_start(kusokurae_game_state_t *self);

kusokurae_error_t kusokurae_game_play(kusokurae_game_state_t *self,
                                      kusokurae_card_t card);

int kusokurae_game_is_final_round(kusokurae_game_state_t *self);

kusokurae_player_t *kusokurae_get_active_player(kusokurae_game_state_t *self);

void kusokurae_get_round_state(kusokurae_game_state_t *self,
                               kusokurae_round_state_t *out);

int kusokurae_card_is_playable(kusokurae_card_t card);

int kusokurae_card_round_played(kusokurae_card_t card);

#ifdef __cplusplus
}
#endif

#endif // BS_KUSOKURAE_SM_H
