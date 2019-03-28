#include <string.h>
#include <stdio.h>
#include <stdlib.h>
#include <time.h>
#include "sm.h"
#include "sm_internal.h"

static kusokurae_card_t DECK[KUSOKURAE_DECK_SIZE];
static int16_t (*rng)(void *);

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
            memmove(pdst, psrc, size);
            pdst += size;
            rwanted--;
        } else {
            memmove(prej, psrc, size);
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

static int compcard2(const void *lhs, const void *rhs) {
    return ((const kusokurae_card_t *)lhs)->rank - ((const kusokurae_card_t *)rhs)->rank;
}

static int is_zero_card(kusokurae_card_t *p) {
    // Card assigned in this lib must have display_order set.
    return p->display_order == 0;
}

static int round_score(kusokurae_game_state_t *g, int *p_bonus_flag) {
    int ret = 0;
    int bonus_flag;
    if (p_bonus_flag == NULL) {
        // Optionally export Ghostbonus status
        p_bonus_flag = &bonus_flag;
    }
    *p_bonus_flag = 0;
    for (int i = 0; i < KUSOKURAE_MAX_PLAYERS; i++) {
        if (!g->current_round[i].display_order) {
            continue;
        }
        switch (g->current_round[i].suit) {
        case KUSOKURAE_SUIT_OTHER:
            (*p_bonus_flag)++;
        default:
            if (g->current_round[i].suit != KUSOKURAE_SUIT_OTHER) {
                ret += g->current_round[i].suit;
            }
        }
    }
    ret <<= *p_bonus_flag;
    return ret;
}

int16_t urand(void *state) {
    // Ref https://bitbucket.org/shlomif/fc-solve/src/dd80a812e8b3aba98a014d939ed77eb1ce764e04/fc-solve/source/board_gen/pi_make_microsoft_freecell_board.c
    int32_t *istate = (int32_t *)state;
    *istate = 214013 * (*istate) + 2531011;
    *istate &= 0x7FFFFFFF;
    return *istate >> 16;
}

void game_state_change(kusokurae_game_state_t *g, int32_t newstate) {
    if (g->cbs.state_transition != NULL) {
        g->cbs.state_transition(g, newstate, g->cbs.userdata_of_state_transition);
    }
    g->status = newstate;
}

int player_has_card(kusokurae_player_t *player, kusokurae_card_t *card) {
    for (int i = 0; i < player->ncards; i++) {
        if (player->cards[i].rank == card->rank &&
            player->cards[i].suit == card->suit &&
            !kusokurae_card_round_played(player->cards[i])) {
            *card = player->cards[i]; // Copy metadata of the hand card out
            return i;
        }
    }
    return -1;
}

void player_drop_card(kusokurae_player_t *player, int index) {
    if (index < 0 || index >= player->ncards) {
        return;
    }
    memmove(&player->cards[index], &player->cards[index + 1], (--player->ncards - index) * sizeof(kusokurae_card_t));
}

void player_set_card_played(kusokurae_player_t *player, int index, int nround) {
    if (index < 0 || index >= player->ncards) {
        return;
    }
    uint32_t n = nround & 0x7F; // 1~127 - play, 0 - unplay
    player->cards[index].flags &= (~MASK_PLAYED_IN_ROUND);
    player->cards[index].flags |= n;
}

void player_set_card_playable(kusokurae_player_t *player, int index, int status) {
    if (index < 0 || index >= player->ncards) {
        return;
    }
    if (status) {
        player->cards[index].flags |= MASK_PLAYABLE;
    } else {
        player->cards[index].flags &= (~MASK_PLAYABLE);
    }
}

void player_set_playable_flags(kusokurae_player_t *player, int is_leader) {
    int i, status, goodcnt = 0, badcnt = 0, lastrank = 0;
    for (i = 0; i < player->ncards; i++) {
        if (kusokurae_card_round_played(player->cards[i])) {
            continue;
        }
        if (!is_leader || player->cards[i].rank > 0) {
            status = 1;
            goodcnt++;
            lastrank = player->cards[i].rank;
        } else {
            // Leader can't play rank 0 unless he/she has NO CHOICE
            status = 0;
            badcnt++;
        }
        player_set_card_playable(player, i, status);
    }
    if (!goodcnt && badcnt) {
        // NO CHOICE
        for (i = 0; i < player->ncards; i++) {
            player_set_card_playable(player, i, 1);
        }
        player->busted = 2;
    }
    if (goodcnt == 1 && lastrank == 0) {
        // A Zero held back
        player->busted = 1;
    }
}

kusokurae_player_t *player_find_next(kusokurae_game_state_t *game, kusokurae_player_t *player) {
    int index = player->index;
    if (index >= game->cfg.np) {
        index = 0;
    }
    return &game->players[index];
}

void kusokurae_global_init() {
    int i;
    // Special treatment for jokers
    DECK[0].suit = KUSOKURAE_SUIT_BAOZI;
    DECK[0].rank = 10;
    DECK[0].display_order = KUSOKURAE_DECK_SIZE;
    for (i = 1; i < 3; i++) {
        DECK[i] = DECK[0];
        DECK[i].display_order -= i;
    }
    // Place the Ghost on 3rd place, so as to be able to simply skip the first
    // card (one of the 2 Angels) when dealing a 4-player game.
    DECK[2].suit = KUSOKURAE_SUIT_OTHER;

    kusokurae_card_suit_t cursuit = KUSOKURAE_SUIT_BAOZI;
    int currank = 9;
    for (; i < KUSOKURAE_DECK_SIZE; i++) {
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

    for (i = 0; i < KUSOKURAE_DECK_SIZE; i++) {
        DECK[i].flags = 0;
    }

    // Use the default PRNG
    rng = &urand;
}

void kusokurae_set_prng(int16_t (*fn)(void *)) {
    if (fn != NULL) {
        rng = fn;
    }
}

kusokurae_error_t kusokurae_game_init(kusokurae_game_state_t *self,
                                      kusokurae_game_config_t *cfg,
                                      kusokurae_game_callbacks_t *cbs) {
    if (self == NULL || cfg == NULL) {
        return KUSOKURAE_ERROR_NULLPTR;
    }
    if (cfg->np < 3 || cfg->np > KUSOKURAE_MAX_PLAYERS) {
        // 3 is the absolute minimal for typical (i.e. in which cards are dealed
        // all at once) card games.
        return KUSOKURAE_ERROR_BAD_NUMBER_OF_PLAYERS;
    }
    memmove(&self->cfg, cfg, sizeof(kusokurae_game_config_t));
    if (cbs != NULL) {
        memmove(&self->cbs, cbs, sizeof(kusokurae_game_callbacks_t));
    } else {
        memset(&self->cbs, 0, sizeof(kusokurae_game_callbacks_t));
    }

    // Seed the PRNG. You can assign to state later if different seeding is needed
    time_t *state = (time_t *)&self->rng_state;
    *state = time(0);
    // Discard the first number that is not quite random.
    // It will be better if nanosecond clock is used as seed.
    urand(state);

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
    sample(deck_base, count, sizeof(kusokurae_card_t), counteach, self->players[0].cards, remaining, &self->rng_state);
    if (self->cfg.np == 4) {
        sample(remaining, count - counteach, sizeof(kusokurae_card_t), counteach, self->players[1].cards, remaining2, &self->rng_state);
        sample(remaining2, count - counteach * 2, sizeof(kusokurae_card_t), counteach, self->players[2].cards, self->players[3].cards, &self->rng_state);
    } else {
        sample(remaining, count - counteach, sizeof(kusokurae_card_t), counteach, self->players[1].cards, self->players[2].cards, &self->rng_state);
    }

    int i;
    // Set up player data and find the ghost holder.
    for (i = 0; i < self->cfg.np; i++) {
        self->players[i].index = i + 1;
        self->players[i].active = KUSOKURAE_ROUND_WAITING;
        self->players[i].ncards = counteach;
        self->players[i].busted = 0;
        if (self->players[i].cards[0].suit == KUSOKURAE_SUIT_OTHER ||
            self->players[i].cards[1].suit == KUSOKURAE_SUIT_OTHER ||
            self->players[i].cards[2].suit == KUSOKURAE_SUIT_OTHER) {
            self->ghost_holder_index = i;
        }
    }
    for (; i < KUSOKURAE_MAX_PLAYERS; i++) {
        memset(&self->players[i], 0, sizeof(kusokurae_player_t));
    }

    memset(&self->current_round, 0, sizeof(self->current_round));
    // It's 1P (players[0])'s turn now
    self->players[0].active = KUSOKURAE_ROUND_ACTIVE;
    game_state_change(self, KUSOKURAE_STATUS_PLAY);
    player_set_playable_flags(&self->players[0], 1);
    self->nround = 0;
    self->high_ranker_index = -1;
    return KUSOKURAE_SUCCESS;
}

kusokurae_error_t kusokurae_game_play(kusokurae_game_state_t *self,
                                      kusokurae_card_t card) {
    if (self == NULL) {
        return KUSOKURAE_ERROR_NULLPTR;
    }
    if (self->status != KUSOKURAE_STATUS_PLAY) {
        return KUSOKURAE_ERROR_NOT_IN_GAME;
    }
    kusokurae_player_t *p = kusokurae_get_active_player(self);
    if (p == NULL) {
        return KUSOKURAE_ERROR_BUG_NOBODY_ACTIVE;
    }

    int pos = player_has_card(p, &card);
    if (pos < 0) {
        return KUSOKURAE_ERROR_CARD_NOT_FOUND;
    }
    if (!kusokurae_card_is_playable(p->cards[pos])) {
        return KUSOKURAE_ERROR_FORBIDDEN_MOVE;
    }

    player_set_card_played(p, pos, self->nround + 1);
    // precord 'pointer to record', not 'pre-cord'
    kusokurae_card_t *precord = &self->current_round[p->index - 1];
    if (!is_zero_card(precord)) {
        // This is the first move in a round (current_round is holding the last
        // trick). Clear it.
        memset(&self->current_round, 0, sizeof(self->current_round));
    }
    *precord = card;

    // Update current round winner
    if (self->high_ranker_index < 0) {
        self->high_ranker_index = p->index - 1;
    } else {
        if (compcard2(&self->current_round[self->high_ranker_index],
                      &self->current_round[p->index - 1]) < 0) {
            self->high_ranker_index = p->index - 1;
        }
    }

    kusokurae_player_t *nextp = player_find_next(self, p);
    if (nextp->active != KUSOKURAE_ROUND_WAITING) {
        // The next player has already played his/her move:
        // the current round (trick) should conclude.
        kusokurae_player_t *winner = &self->players[self->high_ranker_index];
        winner->cards_taken += self->cfg.np;
        winner->score += round_score(self, NULL);

        // Before getting into the next round, call the state change callback to
        // notify library user.
        // Here the state does not really 'change'.
        game_state_change(self, KUSOKURAE_STATUS_PLAY);

        // Next round
        for (int i = 0; i < self->cfg.np; i++) {
            self->players[i].active = KUSOKURAE_ROUND_WAITING;
        }
        player_set_playable_flags(winner, 1);
        self->high_ranker_index = -1;
        winner->active = KUSOKURAE_ROUND_ACTIVE;

        // Game finish
        self->nround++;
        if (self->nround >= self->players[0].ncards) {
            game_state_change(self, KUSOKURAE_STATUS_FINISH);
        }
    } else {
        player_set_playable_flags(nextp, 0);
        p->active = KUSOKURAE_ROUND_DONE;
        nextp->active = KUSOKURAE_ROUND_ACTIVE;
    }

    return KUSOKURAE_SUCCESS;
}

int kusokurae_game_is_final_round(kusokurae_game_state_t *self) {
    if (self == NULL) {
        return 1; // End the caller as soon as possible
    }
    if (self->nround + 1 >= self->players[0].ncards) {
        return 1;
    }
    return 0;
}

kusokurae_player_t *kusokurae_get_active_player(kusokurae_game_state_t *self) {
    if (self == NULL) {
        return NULL;
    }
    for (int i = 0; i < KUSOKURAE_MAX_PLAYERS; i++) {
        if (self->players[i].active == KUSOKURAE_ROUND_ACTIVE) {
            return &self->players[i];
        }
    }
    return NULL;
}

void kusokurae_get_round_state(kusokurae_game_state_t *self,
                               kusokurae_round_state_t *out) {
    if (self == NULL || out == NULL) {
        return;
    }

    // The following two values are already available in *self and put here for
    // convenience.
    out->seq = self->nround + 1;
    out->round_winner = self->high_ranker_index;

    int bonus_flag;
    out->score_on_board = round_score(self, &bonus_flag);
    if (bonus_flag) {
        out->is_doubled = 1;
    } else {
        out->is_doubled = 0;
    }

    memset(&out->moves, 0, KUSOKURAE_MAX_PLAYERS * sizeof(kusokurae_card_t));
    // Get current round's moves.
    kusokurae_player_t *tail = kusokurae_get_active_player(self), *head = tail;
    if (head == NULL) {
        return;
    }
    int i = 0;
    kusokurae_card_t *move;
    do {
        head = player_find_next(self, head);
        move = &self->current_round[head->index - 1];
        if (is_zero_card(move)) {
            if (i > 0) {
                // List end
                break;
            }
        } else {
            if (head->active == KUSOKURAE_ROUND_WAITING) {
                // Round beginning
                break;
            } else {
                out->moves[i++] = *move;
            }
        }
    } while (head != tail);
}

inline int kusokurae_card_is_playable(kusokurae_card_t card) {
    return(card.flags & MASK_PLAYABLE);
}

inline int kusokurae_card_round_played(kusokurae_card_t card) {
    return(card.flags & MASK_PLAYED_IN_ROUND);
}
