#ifndef BS_KUSOKURAE_SM_H
#define BS_KUSOKURAE_SM_H

#ifdef __cplusplus
extern "C" {
#endif

#define KUSOKURAE_DECK_SIZE         33
#define KUSOKURAE_MAX_HAND_CARDS    22
#define KUSOKURAE_MAX_PLAYERS       4

typedef struct {
    int np; // Number of players (3 or 4)
} kusokurae_game_config_t;

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
    // Sequence in the new, unshuffled deck. Higher value precedes lower
    // e.g. The most newby card, Ghost, has a display_order of 33.
    // 0 indicates invalid data (unfilled slot).
    // Should be filled during global initialization and copied afterwards.
    unsigned display_order;

    // Declared above
    kusokurae_card_suit_t suit;

    // 0~10 for BAOZI
    // 0~9 for YOUTIAO and XIANG
    // 10 for OTHER
    int rank;
} kusokurae_card_t;

typedef enum {
    KUSOKURAE_ROUND_WAITING,
    KUSOKURAE_ROUND_ACTIVE,
    KUSOKURAE_ROUND_DONE,
} kusokurae_round_status_t;

typedef struct {
    // 1~4 (0 for invalid)
    int index;

    // 1 - active (playing), 2 - already played
    kusokurae_round_status_t active;

    // 22 card slots (reserved for playing with 2 decks)
    kusokurae_card_t hand[KUSOKURAE_MAX_HAND_CARDS];

    // The number of valid cards in hand.
    // When a card is played, it is removed from hand and all following cards
    //   should be moved ahead to keep the array consecutive.
    int ncards;

    // If the player wins a round, he/she takes all cards played in that round.
    // cards_taken will always be multiples of player count.
    int cards_taken;

    // The score accumulated from cards_taken.
    int score;
} kusokurae_player_t;

typedef enum {
    KUSOKURAE_SUCCESS,
    KUSOKURAE_ERROR_NULLPTR,
    KUSOKURAE_ERROR_BAD_NUMBER_OF_PLAYERS,
    KUSOKURAE_ERROR_UNINITIALIZED,
} kusokurae_error_t;

typedef struct {
    kusokurae_game_config_t cfg;
    kusokurae_game_status_t status;

    // Max 4 players
    kusokurae_player_t players[KUSOKURAE_MAX_PLAYERS];

    // Active player (0~3) - only valid if status is PLAY
    int active_player_index;

    // Finished round count
    int nround;

    // Who has the ghost in hand?
    int ghost_holder_index;

    // Cards played in the current round.
    // players[n]'s move is placed in current_round[n].
    kusokurae_card_t current_round[KUSOKURAE_MAX_PLAYERS];
} kusokurae_game_state_t;

typedef struct {
    // On screen: "Round <seq>"
    int seq;

    // Whether there is a ghost
    int is_doubled;

    // Total score in cards played
    int score_on_board;

    // The current winning player
    int leader;
} kusokurae_round_state_t;

void kusokurae_global_init();

kusokurae_error_t kusokurae_game_init(kusokurae_game_state_t *self,
                                      kusokurae_game_config_t *cfg);

kusokurae_error_t kusokurae_game_start(kusokurae_game_state_t *self);

kusokurae_error_t kusokurae_game_play(kusokurae_game_state_t *self,
                                      kusokurae_card_t card);

kusokurae_error_t kusokurae_game_autoplay(kusokurae_game_state_t *self);

int kusokurae_get_active_player(kusokurae_game_state_t *self);

void kusokurae_get_round_state(kusokurae_game_state_t *self,
                               kusokurae_round_state_t *out);

#ifdef __cplusplus
}
#endif

#endif // BS_KUSOKURAE_SM_H
