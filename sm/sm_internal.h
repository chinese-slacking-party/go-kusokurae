#ifndef BS_KUSOKURAE_SM_INTERNAL_H
#define BS_KUSOKURAE_SM_INTERNAL_H

#ifdef __cplusplus
extern "C" {
#endif

#include "sm.h"

int player_has_card(kusokurae_player_t *player, kusokurae_card_t *card);
void player_drop_card(kusokurae_player_t *player, int index);

#ifdef __cplusplus
}
#endif

#endif // BS_KUSOKURAE_SM_INTERNAL_H
