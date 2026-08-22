#ifndef BS_KUSOKURAE_SM_INTERNAL_H
#define BS_KUSOKURAE_SM_INTERNAL_H

#ifdef __cplusplus
extern "C" {
#endif

#include "sm.h"

#define MASK_PLAYED_IN_ROUND    0x7F
#define MASK_PLAYABLE           0x80

// ms_rand is the default generator: a replica of the LCG behind the
// Microsoft implementation of C rand(), whose RAND_MAX of 32767 is where
// KUSOKURAE_RAND_MAX comes from. Returns [0, KUSOKURAE_RAND_MAX] as an int,
// signed and machine-word wide like the rand() it replicates.
int ms_rand(void *state);

void game_state_change(kusokurae_game_state_t *g, int32_t newstate);

int player_has_card(kusokurae_player_t *player, kusokurae_card_t *card);
void player_drop_card(kusokurae_player_t *player, int index);

void player_set_card_played(kusokurae_player_t *player, int index, int nround);
void player_set_card_playable(kusokurae_player_t *player, int index, int status);
void player_set_playable_flags(kusokurae_player_t *player, int is_leader);

kusokurae_player_t *player_find_next(kusokurae_game_state_t *game, kusokurae_player_t *player);

#ifdef __cplusplus
}
#endif

#endif // BS_KUSOKURAE_SM_INTERNAL_H
