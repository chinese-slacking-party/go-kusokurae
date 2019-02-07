#include <string.h>
#include <stdio.h>
#include <stdlib.h>
#include <time.h>
#include "sm.h"
#include "sm_internal.h"

static kusokurae_card_t DECK[KUSOKURAE_DECK_SIZE];
static int16_t (*rng)(void *);

static void print_cards(kusokurae_card_t *base, size_t count) {
    for (int i = 0; i < count; i++) {
        printf("%p %d %d %d\n", &base[i], base[i].suit, base[i].rank, base[i].display_order);
    }
}

static void sample(void *ptr, size_t count, size_t size,
                   size_t wanted, void *pchosen, void *pdiscarded,
                   void *rng_state) {
    char *psrc = (char *)ptr, *pdst = (char *)pchosen, *prej = (char *)pdiscarded;
    size_t rcount = count, rwanted = wanted; // r for remaining
    int64_t threshold;
    int16_t dice;
    while (rcount > 0) {
        dice = rng(rng_state);
        threshold = (MS_RAND_MAX + 1ULL) * rwanted / rcount;
        //printf("%ld wanted, %ld remaining, %lld/%lld\n", rwanted, rcount, dice, threshold);
        if (dice < threshold) {
            memcpy(pdst, psrc, size);
            pdst += size;
            rwanted--;
        } else {
            memcpy(prej, psrc, size);
            prej += size;
        }
        psrc += size;
        rcount--;
    }
}

static int compcard(const void *lhs, const void *rhs) {
    if (((const kusokurae_card_t *)lhs)->display_order > ((const kusokurae_card_t *)rhs)->display_order) {
        return -1;
    } else if (((const kusokurae_card_t *)lhs)->display_order < ((const kusokurae_card_t *)rhs)->display_order) {
        return 1;
    }
    return 0;
}

int16_t urand(void *state) {
    // Ref https://bitbucket.org/shlomif/fc-solve/src/dd80a812e8b3aba98a014d939ed77eb1ce764e04/fc-solve/source/board_gen/pi_make_microsoft_freecell_board.c
    int32_t *istate = (int32_t *)state;
    *istate = 214013 * (*istate) + 2531011;
    *istate &= 0x7FFFFFFF;
    return *istate >> 16;
}

int player_has_card(kusokurae_player_t *player, kusokurae_card_t *card) {
    for (int i = 0; i < player->ncards; i++) {
        if (player->hand[i].rank == card->rank &&
            player->hand[i].suit == card->suit) {
            return i;
        }
    }
    return -1;
}

void player_drop_card(kusokurae_player_t *player, int index) {
    if (index < 0 || index >= player->ncards) {
        return;
    }
    memmove(&player->hand[index], &player->hand[index + 1], (--player->ncards - index) * sizeof(kusokurae_card_t));
}

void kusokurae_global_init() {
    // Special treatment for jokers
    DECK[0].suit = KUSOKURAE_SUIT_BAOZI;
    DECK[0].rank = 10;
    DECK[0].display_order = KUSOKURAE_DECK_SIZE;
    for (int i = 1; i < 3; i++) {
        DECK[i] = DECK[0];
        DECK[i].display_order -= i;
    }
    // Place the Ghost on 3rd place, so as to be able to simply skip the first
    // card (one of the 2 Angels) when dealing a 4-player game.
    DECK[2].suit = KUSOKURAE_SUIT_OTHER;

    kusokurae_card_suit_t cursuit = KUSOKURAE_SUIT_BAOZI;
    int currank = 9;
    for (int i = 3; i < KUSOKURAE_DECK_SIZE; i++) {
        DECK[i].suit = cursuit;
        DECK[i].rank = currank;
        DECK[i].display_order = KUSOKURAE_DECK_SIZE - i;
        if (currank == 0) {
            cursuit = (kusokurae_card_suit_t)((int)cursuit - 1);
            currank = 9;
        } else {
            currank--;
        }
    }
    //print_cards(DECK, KUSOKURAE_DECK_SIZE);
    // Use the default PRNG
    rng = &urand;
}

void kusokurae_set_prng(int16_t (*fn)(void *)) {
    if (fn != NULL) {
        rng = fn;
    }
}

kusokurae_error_t kusokurae_game_init(kusokurae_game_state_t *self,
                                      kusokurae_game_config_t *cfg) {
    if (self == NULL || cfg == NULL) {
        return KUSOKURAE_ERROR_NULLPTR;
    }
    if (cfg->np < 3 || cfg->np > 4) {
        return KUSOKURAE_ERROR_BAD_NUMBER_OF_PLAYERS;
    }
    memcpy(&self->cfg, cfg, sizeof(kusokurae_game_config_t));

    // Seed the PRNG. You can assign to state later if different seeding is needed
    time_t *state = (time_t *)&self->rng_state;
    *state = time(0);
    // Discard the first number that is not quite random.
    // It will be better if nanosecond clock is used as seed.
    urand(state);

    self->status = KUSOKURAE_STATUS_INIT;

    for (int i = 0; i < self->cfg.np; i++) {
        self->players[i].index = i + 1;
    }
    return KUSOKURAE_SUCCESS;
}

kusokurae_error_t kusokurae_game_start(kusokurae_game_state_t *self) {
    if (self == NULL) {
        return KUSOKURAE_ERROR_NULLPTR;
    }
    if (self->cfg.np == 0) {
        return KUSOKURAE_ERROR_UNINITIALIZED;
    }

    // Deck should be already prepared in kusokurae_global_init().
    // Here we pick the useful part: the whole deck (if there're 3 players) or
    // the deck with one Angel removed (if there're 4 players).
    kusokurae_card_t *deck_base = DECK;
    size_t count = KUSOKURAE_DECK_SIZE;
    if (self->cfg.np == 4) {
        deck_base++;
        count--;
    }
    size_t counteach = count / self->cfg.np;
    // At most two remainder areas are used.
    // TODO: more flexible card assignment (e.g. 5~6 players, 2 decks)
    kusokurae_card_t remaining[KUSOKURAE_DECK_SIZE], remaining2[KUSOKURAE_DECK_SIZE];
    sample(deck_base, count, sizeof(kusokurae_card_t), counteach, self->players[0].hand, remaining, &self->rng_state);
    if (self->cfg.np == 4) {
        sample(remaining, count - counteach, sizeof(kusokurae_card_t), counteach, self->players[1].hand, remaining2, &self->rng_state);
        sample(remaining2, count - counteach * 2, sizeof(kusokurae_card_t), counteach, self->players[2].hand, self->players[3].hand, &self->rng_state);
    } else {
        sample(remaining, count - counteach, sizeof(kusokurae_card_t), counteach, self->players[1].hand, self->players[2].hand, &self->rng_state);
    }

    int i;
    // Set up player data and find the ghost holder.
    for (i = 0; i < self->cfg.np; i++) {
        //print_cards(self->players[i].hand, counteach);
        self->players[i].index = i + 1;
        self->players[i].active = KUSOKURAE_ROUND_WAITING;
        self->players[i].ncards = counteach;
        if (self->players[i].hand[0].suit == KUSOKURAE_SUIT_OTHER ||
            self->players[i].hand[1].suit == KUSOKURAE_SUIT_OTHER ||
            self->players[i].hand[2].suit == KUSOKURAE_SUIT_OTHER) {
            self->ghost_holder_index = i;
        }
    }
    for (; i < KUSOKURAE_MAX_PLAYERS; i++) {
        memset(&self->players[i], 0, sizeof(kusokurae_player_t));
    }

    memset(&self->current_round, 0, sizeof(self->current_round));
    // It's 1P (players[0])'s turn now
    self->players[0].active = KUSOKURAE_ROUND_ACTIVE;
    self->status = KUSOKURAE_STATUS_PLAY;
    self->nround = 0;
    self->last_player_index = -1;
    return KUSOKURAE_SUCCESS;
}

kusokurae_error_t kusokurae_game_play(kusokurae_game_state_t *self,
                                      kusokurae_card_t card) {
    // Stub
    return KUSOKURAE_ERROR_UNIMPLEMENTED;
}
