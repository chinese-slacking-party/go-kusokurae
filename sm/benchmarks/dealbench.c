// Compares ways of dealing the deck. The engine uses selection sampling
// (Knuth Algorithm S), which draws one random number per card scanned and
// therefore 33 + 22 = 55 draws for a three-player deal, but leaves each hand in
// deck order — already sorted, no sort needed. The alternatives shuffle first
// and pay for a sort, except the last one, which shuffles ownership labels
// instead of cards and so keeps hands in deck order for free.
//
// Which one wins depends on what a random number costs. With the engine's own
// PRNG a draw is ~2ns and the sort dominates; with the Go PRNG reached through
// a cgo callback a draw costs ~91ns and the draw count dominates. See the
// PRNG benchmarks in sm/rand_test.go for that side of the comparison.
//
//	gcc -O2 -std=gnu11 -I .. -o /tmp/dealbench dealbench.c ../sm.c && /tmp/dealbench

#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

#include "sm_internal.h"

#define REPS 2000000
#define PLAYERS 3
#define HAND (KUSOKURAE_DECK_SIZE / PLAYERS)

static kusokurae_card_t DECK2[KUSOKURAE_DECK_SIZE];

static int compcard(const void *l, const void *r) {
    uint32_t a = ((const kusokurae_card_t *)l)->display_order;
    uint32_t b = ((const kusokurae_card_t *)r)->display_order;
    return a > b ? -1 : (a < b ? 1 : 0);
}

static double now(void) {
    struct timespec t;
    timespec_get(&t, TIME_UTC);
    return t.tv_sec + t.tv_nsec / 1e9;
}

// Draws a uniform value in [0, bound) without modulo bias.
static int roll(void *state, int bound, int *draws) {
    int limit = (KUSOKURAE_RAND_MAX + 1) / bound * bound, r;
    do {
        r = ms_rand(state);
        (*draws)++;
    } while (r >= limit);
    return r % bound;
}

// A: what the engine does today. One draw per card scanned, hands stay sorted.
static int deal_selection(void *state) {
    kusokurae_card_t hands[PLAYERS][HAND], rem[KUSOKURAE_DECK_SIZE], rem2[KUSOKURAE_DECK_SIZE];
    struct { kusokurae_card_t *src, *dst, *rej; size_t count, wanted; } pass[2] = {
        {DECK2, hands[0], rem,  KUSOKURAE_DECK_SIZE,      HAND},
        {rem,   hands[1], rem2, KUSOKURAE_DECK_SIZE - HAND, HAND},
    };
    int draws = 0;
    for (int k = 0; k < 2; k++) {
        kusokurae_card_t *psrc = pass[k].src, *pdst = pass[k].dst, *prej = pass[k].rej;
        size_t rc = pass[k].count, rw = pass[k].wanted;
        while (rc > 0) {
            int dice = ms_rand(state);
            draws++;
            if (dice < (int64_t)(KUSOKURAE_RAND_MAX + 1ULL) * rw / rc) {
                *pdst++ = *psrc;
                rw--;
            } else {
                *prej++ = *psrc;
            }
            psrc++;
            rc--;
        }
    }
    memmove(hands[2], rem2, HAND * sizeof(kusokurae_card_t));
    return draws;
}

// B: shuffle the deck, slice it, sort each hand with qsort.
static int deal_shuffle_qsort(void *state) {
    kusokurae_card_t d[KUSOKURAE_DECK_SIZE], hands[PLAYERS][HAND];
    int draws = 0;
    memmove(d, DECK2, sizeof(d));
    for (int i = KUSOKURAE_DECK_SIZE - 1; i > 0; i--) {
        int j = roll(state, i + 1, &draws);
        kusokurae_card_t t = d[i]; d[i] = d[j]; d[j] = t;
    }
    for (int p = 0; p < PLAYERS; p++) {
        memmove(hands[p], d + p * HAND, HAND * sizeof(kusokurae_card_t));
        qsort(hands[p], HAND, sizeof(kusokurae_card_t), compcard);
    }
    return draws;
}

// B': same, with an inline insertion sort instead of qsort's indirect calls.
static int deal_shuffle_isort(void *state) {
    kusokurae_card_t d[KUSOKURAE_DECK_SIZE], hands[PLAYERS][HAND];
    int draws = 0;
    memmove(d, DECK2, sizeof(d));
    for (int i = KUSOKURAE_DECK_SIZE - 1; i > 0; i--) {
        int j = roll(state, i + 1, &draws);
        kusokurae_card_t t = d[i]; d[i] = d[j]; d[j] = t;
    }
    for (int p = 0; p < PLAYERS; p++) {
        memmove(hands[p], d + p * HAND, HAND * sizeof(kusokurae_card_t));
        for (int i = 1; i < HAND; i++) {
            kusokurae_card_t key = hands[p][i];
            int k = i - 1;
            while (k >= 0 && hands[p][k].display_order < key.display_order) {
                hands[p][k + 1] = hands[p][k];
                k--;
            }
            hands[p][k + 1] = key;
        }
    }
    return draws;
}

// C: shuffle ownership labels, then walk the deck in order. Hands come out
// sorted because the deck is, and the draw count no longer grows with the
// number of players.
static int deal_labels(void *state) {
    unsigned char owner[KUSOKURAE_DECK_SIZE];
    kusokurae_card_t hands[PLAYERS][HAND];
    int n[PLAYERS] = {0}, draws = 0;
    for (int i = 0; i < KUSOKURAE_DECK_SIZE; i++) {
        owner[i] = i / HAND;
    }
    for (int i = KUSOKURAE_DECK_SIZE - 1; i > 0; i--) {
        int j = roll(state, i + 1, &draws);
        unsigned char t = owner[i]; owner[i] = owner[j]; owner[j] = t;
    }
    for (int i = 0; i < KUSOKURAE_DECK_SIZE; i++) {
        hands[owner[i]][n[owner[i]]++] = DECK2[i];
    }
    return draws;
}

int main(void) {
    kusokurae_global_init();
    for (int i = 0; i < KUSOKURAE_DECK_SIZE; i++) {
        DECK2[i].display_order = KUSOKURAE_DECK_SIZE - i;
    }

    struct { const char *name; int (*fn)(void *); } cases[] = {
        {"A  选择抽样（现行，手牌天然有序）", deal_selection},
        {"B  Fisher-Yates + 每份 qsort    ", deal_shuffle_qsort},
        {"B' Fisher-Yates + 手写插入排序  ", deal_shuffle_isort},
        {"C  洗归属标签 + 按牌序回填      ", deal_labels},
    };

    int32_t state = 12345;
    for (size_t c = 0; c < sizeof(cases) / sizeof(cases[0]); c++) {
        cases[c].fn(&state);
        long draws = 0;
        double t0 = now();
        for (int i = 0; i < REPS; i++) {
            draws += cases[c].fn(&state);
        }
        double dt = now() - t0;
        printf("%s  %6.0f ns/局   平均 %.1f 次抽签\n",
               cases[c].name, dt / REPS * 1e9, (double)draws / REPS);
    }
    return 0;
}
