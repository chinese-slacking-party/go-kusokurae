#ifndef BS_KUSOKURAE_SM_INTERNAL_H
#define BS_KUSOKURAE_SM_INTERNAL_H

#ifdef __cplusplus
extern "C" {
#endif

#include "sm.h"

#define MS_RAND_MAX 32767

#define MASK_PLAYED_IN_ROUND    0x7F
#define MASK_PLAYABLE           0x80

int16_t urand(void *state);

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
