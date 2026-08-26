// Checks the engine's own generator and its seeding -- the parts the Go
// binding normally hides behind its own PRNG bridge.
//
//	gcc -std=c17 -fsanitize=address,undefined -I .. -o /tmp/nativeprng nativeprng.c ../sm.c
//	/tmp/nativeprng

#include <stdio.h>
#include <string.h>
#include <stdint.h>

#include "sm_internal.h"

static int failures;

static void report(const char *name, int ok) {
    printf("%-4s  %s\n", ok ? "ok" : "FAIL", name);
    if (!ok) {
        failures++;
    }
}

// ms_rand() replicates Microsoft's rand(). These are the values it yields from
// seed 1, and pinning them is what keeps the unsigned rewrite honest.
static void check_golden(void) {
    static const int want[] = {
        41, 18467, 6334, 26500, 19169, 15724,
        11478, 29358, 26962, 24464, 5705, 28145,
    };
    uint32_t state = 1;
    int ok = 1;
    for (size_t i = 0; i < sizeof(want) / sizeof(want[0]); i++) {
        int got = ms_rand(&state);
        if (got != want[i]) {
            printf("      draw %zu: got %d, want %d\n", i, got, want[i]);
            ok = 0;
        }
    }
    report("ms_rand 复刻 Microsoft rand() 的序列", ok);
}

// The contract is a closed range, so both endpoints have to be reachable and
// nothing may fall outside.
static void check_range(void) {
    uint32_t state = 12345;
    int lo = 0, hi = 0, out = 0;
    for (long i = 0; i < 4000000; i++) {
        int v = ms_rand(&state);
        if (v < 0 || v > KUSOKURAE_RAND_MAX) {
            out++;
        } else if (v == 0) {
            lo++;
        } else if (v == KUSOKURAE_RAND_MAX) {
            hi++;
        }
    }
    report("ms_rand 落在 [0, KUSOKURAE_RAND_MAX] 内", out == 0);
    report("ms_rand 能取到区间两端", lo > 0 && hi > 0);
}

// Games created back to back must not be dealt alike. Before the seed came
// from a nanosecond clock, two games started in the same second shared one;
// on a big-endian machine the seed was always zero.
static void check_seed_varies(void) {
    enum { GAMES = 8, NP = 3, HAND = KUSOKURAE_DECK_SIZE / NP };
    kusokurae_game_config_t cfg = {.np = NP, .first_player_idx = 0};
    uint32_t deal[GAMES][KUSOKURAE_DECK_SIZE];
    int ok = 1;

    for (int i = 0; i < GAMES; i++) {
        kusokurae_game_state_t g;
        memset(&g, 0, sizeof(g));
        // A fresh init per game: it is the seeding under test, not the stream.
        kusokurae_game_init(&g, &cfg, NULL);
        kusokurae_game_start(&g);
        for (int p = 0; p < NP; p++) {
            for (int j = 0; j < HAND; j++) {
                deal[i][p * HAND + j] = g.players[p].cards[j].display_order;
            }
        }
    }
    for (int i = 0; ok && i < GAMES; i++) {
        for (int j = i + 1; j < GAMES; j++) {
            if (memcmp(deal[i], deal[j], sizeof(deal[0])) == 0) {
                printf("      第 %d 局和第 %d 局发牌完全相同\n", i, j);
                ok = 0;
                break;
            }
        }
    }
    report("相继开局的 8 局发牌两两不同", ok);
}

// Many deals with the real generator: each has to be a partition of the deck,
// and the Ghost has to reach every seat. prngcontract.c covers the partition
// against generators that break the contract; this covers it against volume.
static void check_deals(void) {
    enum { NP = 3, HAND = KUSOKURAE_DECK_SIZE / NP, DEALS = 3000 };
    kusokurae_game_config_t cfg = {.np = NP, .first_player_idx = 0};
    kusokurae_game_state_t g;
    long seat[NP] = {0};
    int ok = 1;

    memset(&g, 0, sizeof(g));
    kusokurae_game_init(&g, &cfg, NULL);
    for (long d = 0; d < DEALS && ok; d++) {
        kusokurae_game_start(&g);

        int seen[KUSOKURAE_DECK_SIZE + 1] = {0};
        for (int p = 0; p < NP; p++) {
            for (int j = 0; j < HAND; j++) {
                uint32_t o = g.players[p].cards[j].display_order;
                if (o == 0 || o > KUSOKURAE_DECK_SIZE || seen[o]++) {
                    printf("      第 %ld 副：display_order %u 无效或重复\n", d, o);
                    ok = 0;
                }
            }
        }
        for (int o = 1; ok && o <= KUSOKURAE_DECK_SIZE; o++) {
            if (!seen[o]) {
                printf("      第 %ld 副：漏发 display_order %d\n", d, o);
                ok = 0;
            }
        }

        int who = g.ghost_holder_index[0];
        if (who < 0 || who >= NP) {
            printf("      第 %ld 副：鬼牌持有者 %d\n", d, who);
            ok = 0;
        } else {
            seat[who]++;
        }
    }
    report("每副牌都是牌堆的完整划分", ok);

    // Expected DEALS/NP per seat with a standard deviation of ~26, so the
    // +/-20% window is far outside the noise.
    for (int i = 0; ok && i < NP; i++) {
        if (seat[i] < DEALS / NP * 4 / 5 || seat[i] > DEALS / NP * 6 / 5) {
            printf("      座位 %d 拿到鬼牌 %ld 次，期望约 %d 次\n",
                   i, seat[i], DEALS / NP);
            ok = 0;
        }
    }
    report("鬼牌均匀落到每个座位", ok);
}

static void check_manual_seed(void) {
    enum { NP = 3, HAND = KUSOKURAE_DECK_SIZE / NP };
    kusokurae_game_config_t cfg = {.np = NP, .first_player_idx = 0};
    kusokurae_game_state_t g1, g2;
    int ok = 1;

    memset(&g1, 0, sizeof(g1));
    memset(&g2, 0, sizeof(g2));

    uint8_t seed[32] = {0};
    seed[0] = 0x12;
    seed[1] = 0x34;

    kusokurae_game_init(&g1, &cfg, NULL);
    kusokurae_game_seed(&g1, seed);
    kusokurae_game_start(&g1);

    kusokurae_game_init(&g2, &cfg, NULL);
    kusokurae_game_seed(&g2, seed);
    kusokurae_game_start(&g2);

    for (int p = 0; p < NP; p++) {
        for (int j = 0; j < HAND; j++) {
            if (g1.players[p].cards[j].display_order != g2.players[p].cards[j].display_order) {
                ok = 0;
                break;
            }
        }
    }
    report("相同的种子产生相同的发牌 (Option A API)", ok);
}

int main(void) {
    kusokurae_global_init();
    check_golden();
    check_range();
    check_seed_varies();
    check_deals();
    check_manual_seed();
    printf("\n%d 项失败\n", failures);
    return failures ? 1 : 0;
}
