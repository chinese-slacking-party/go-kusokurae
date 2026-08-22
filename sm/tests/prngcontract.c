// Checks that dealing survives a generator that breaks the contract of
// kusokurae_set_prng(). A replacement generator is public API, so sample() must
// not depend on it behaving: whatever comes back, every player has to end up
// with the right number of real cards and the deck has to stay partitioned.
//
// The interesting case is a generator that means to return [0, 65535]. It
// cannot say so through an int16_t, so half of its values arrive negative, and
// a negative draw compares below every threshold -- including the zero
// threshold that is supposed to stop selection once a hand is full.
//
//	gcc -std=gnu11 -fsanitize=address -I .. -o /tmp/prngcontract prngcontract.c ../sm.c
//	/tmp/prngcontract

#include <stdio.h>
#include <string.h>
#include <stdint.h>
#include "sm.h"
#include "sm_internal.h"

static uint32_t seed = 42;

// Means to cover [0, 65535]; half of it arrives negative.
static int16_t wide_rand(void *state) {
    (void)state;
    seed = 1664525u * seed + 1013904223u;
    return (int16_t)(uint16_t)(seed >> 16);
}

static int16_t always_negative(void *state) { (void)state; return -1; }
static int16_t always_zero(void *state)     { (void)state; return 0; }
static int16_t always_max(void *state)      { (void)state; return KUSOKURAE_RAND_MAX; }
static int16_t always_min(void *state)      { (void)state; return INT16_MIN; }

static int failures;

static void check_deal(const char *name, int16_t (*prng)(void *), int np) {
    int cards_each = (np == 4 ? KUSOKURAE_DECK_SIZE - 1 : KUSOKURAE_DECK_SIZE) / np;
    // A four-player deck drops DECK[0], the Angel with the highest display
    // order, so the dealt cards run 1..32 rather than 1..33.
    int top = KUSOKURAE_DECK_SIZE - (np == 4 ? 1 : 0);
    kusokurae_game_state_t g;
    kusokurae_game_config_t cfg = {np, 0};

    kusokurae_set_prng(prng);
    memset(&g, 0, sizeof(g));
    kusokurae_game_init(&g, &cfg, NULL);
    kusokurae_game_start(&g);

    int seen[KUSOKURAE_DECK_SIZE + 1] = {0};
    int invalid = 0, duplicated = 0, missing = 0;
    for (int p = 0; p < np; p++) {
        if (g.players[p].ncards != cards_each) {
            printf("FAIL  %-16s %dP ncards=%d，应为 %d\n", name, p + 1, g.players[p].ncards, cards_each);
            failures++;
        }
        for (int j = 0; j < cards_each; j++) {
            uint32_t o = g.players[p].cards[j].display_order;
            if (o == 0 || o > KUSOKURAE_DECK_SIZE) {
                invalid++;
            } else if (seen[o]++) {
                duplicated++;
            }
        }
    }
    int extra = 0;
    for (int o = 1; o <= KUSOKURAE_DECK_SIZE; o++) {
        if (o <= top && !seen[o]) missing++;
        if (o > top && seen[o]) extra++;
    }
    duplicated += extra;
    if (invalid || duplicated || missing) {
        printf("FAIL  %-16s 无效 %d 张，重复 %d 张，漏发 %d 张\n",
               name, invalid, duplicated, missing);
        failures++;
    } else {
        printf("ok    %-16s %d 人局，牌堆完整分割\n", name, np);
    }
}

int main(void) {
    kusokurae_global_init();

    struct { const char *name; int16_t (*fn)(void *); } bad[] = {
        {"wide_rand",       wide_rand},
        {"always_negative", always_negative},
        {"always_zero",     always_zero},
        {"always_max",      always_max},
        {"always_min",      always_min},
    };
    for (size_t i = 0; i < sizeof(bad) / sizeof(bad[0]); i++) {
        check_deal(bad[i].name, bad[i].fn, 3);
        check_deal(bad[i].name, bad[i].fn, 4);
    }

    kusokurae_set_prng(&ms_rand);
    check_deal("ms_rand", &ms_rand, 3);
    check_deal("ms_rand", &ms_rand, 4);

    printf("\n%d 项失败\n", failures);
    return failures ? 1 : 0;
}
