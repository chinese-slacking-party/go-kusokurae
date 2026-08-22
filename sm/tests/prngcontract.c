// Checks that dealing survives a generator that breaks the contract of
// kusokurae_set_prng(). A replacement generator is public API, so sample() must
// not depend on it behaving: whatever comes back, every player has to end up
// with the right number of real cards and the deck has to stay partitioned.
//
// The worst case is a negative draw: it compares below every threshold,
// including the zero threshold that is supposed to stop selection once a hand
// is full. wide_rand covers [0, 65535], which the int return type can express
// but the contract does not allow; negative_wide_rand is what the same
// generator used to look like back when the callback returned int16_t and half
// its values wrapped to negative.
//
//	gcc -std=gnu11 -fsanitize=address -I .. -o /tmp/prngcontract prngcontract.c ../sm.c
//	/tmp/prngcontract

#include <stdio.h>
#include <string.h>
#include <stdint.h>
#include <limits.h>
#include "sm.h"
#include "sm_internal.h"

static uint32_t seed = 42;

static uint16_t next16(void) {
    seed = 1664525u * seed + 1013904223u;
    return (uint16_t)(seed >> 16);
}

// Twice the permitted range, stated honestly. Every draw above
// KUSOKURAE_RAND_MAX simply fails its comparison, so the deal skews without
// ever going out of bounds.
static int wide_rand(void *state) { (void)state; return next16(); }

// The same generator as it behaved when the callback returned int16_t: half the
// values wrapped to negative and were therefore always selected.
static int negative_wide_rand(void *state) { (void)state; return (int16_t)next16(); }

static int always_negative(void *state) { (void)state; return -1; }
static int always_zero(void *state)     { (void)state; return 0; }
static int always_max(void *state)      { (void)state; return KUSOKURAE_RAND_MAX; }
static int over_max(void *state)        { (void)state; return KUSOKURAE_RAND_MAX + 1; }
static int always_min(void *state)      { (void)state; return INT_MIN; }

static int failures;

static void check_deal(const char *name, int (*prng)(void *), int np) {
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
            printf("FAIL  %-19s %dP ncards=%d，应为 %d\n", name, p + 1, g.players[p].ncards, cards_each);
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
        printf("FAIL  %-19s 无效 %d 张，重复 %d 张，漏发 %d 张\n",
               name, invalid, duplicated, missing);
        failures++;
    } else {
        printf("ok    %-19s %d 人局，牌堆完整分割\n", name, np);
    }
}

int main(void) {
    kusokurae_global_init();

    struct { const char *name; int (*fn)(void *); } bad[] = {
        {"wide_rand",          wide_rand},
        {"negative_wide_rand", negative_wide_rand},
        {"always_negative",    always_negative},
        {"always_zero",        always_zero},
        {"always_max",         always_max},
        {"over_max",           over_max},
        {"always_min",         always_min},
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
