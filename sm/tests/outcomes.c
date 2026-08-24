// Statistical survey of finished games: how often the table ties for first, how
// far the total score can swing, and how often a naive "highest score wins"
// implementation would name the wrong seat. Plays random legal moves, which
// samples the outcome space far better than always picking the first playable
// card (hands are kept in deck order, so that degenerates into everyone leading
// with their highest card).
//
// Structural invariants are checked on every game; a violation exits non-zero.
// The statistics themselves are reported, not asserted.
//
//	gcc -O2 -std=gnu11 -I .. -o /tmp/outcomes outcomes.c ../sm.c && /tmp/outcomes

#include <stdint.h>
#include <stdio.h>
#include <string.h>

#include "sm_internal.h"

// xorshift64, independent of the engine's own PRNG so that the choice of move
// does not perturb the deal.
static uint64_t rs = 0x243F6A8885A308D3ull;
static uint32_t pick(uint32_t n) {
    rs ^= rs << 13; rs ^= rs >> 7; rs ^= rs << 17;
    return (uint32_t)(rs % n);
}

static int broken;

static void survey(int np, long games) {
    kusokurae_game_state_t g;
    kusokurae_game_config_t cfg = {.np = np, .first_player_idx = 0};
    long ties = 0, max_le_zero = 0, negative_total = 0, naive_wrong = 0, widest_tie = 0;
    int lowest_total = 9999, highest_total = -9999, lowest_seat_score = 9999;
    int cards_each = (np == 4 ? KUSOKURAE_DECK_SIZE - 1 : KUSOKURAE_DECK_SIZE) / np;

    for (long t = 0; t < games; t++) {
        memset(&g, 0, sizeof(g));
        kusokurae_game_init(&g, &cfg, NULL);
        kusokurae_game_start(&g);

        while (g.status == KUSOKURAE_STATUS_PLAY) {
            kusokurae_player_t *p = kusokurae_get_active_player(&g);
            if (p == NULL) {
                break;
            }
            kusokurae_card_t options[KUSOKURAE_MAX_HAND_CARDS];
            int n = 0;
            for (int i = 0; i < p->ncards; i++) {
                if (!kusokurae_card_round_played(p->cards[i]) &&
                    kusokurae_card_is_playable(p->cards[i])) {
                    options[n++] = p->cards[i];
                }
            }
            if (n == 0) {
                printf("BROKEN: %dP 无牌可出，nround=%d\n", p->index, g.nround);
                broken++;
                break;
            }
            kusokurae_game_play(&g, options[pick(n)]);
        }

        // Structural invariants: the game ends, every trick is accounted for.
        if (g.status != KUSOKURAE_STATUS_FINISH || g.nround != cards_each) {
            printf("BROKEN: 对局未正常结束 status=%d nround=%d\n", g.status, g.nround);
            broken++;
        }
        int taken = 0;
        for (int i = 0; i < np; i++) {
            taken += g.players[i].cards_taken;
        }
        if (taken != cards_each * np) {
            printf("BROKEN: 吃牌总数 %d，应为 %d\n", taken, cards_each * np);
            broken++;
        }

        int high = -9999, tied = 0, total = 0;
        for (int i = 0; i < np; i++) {
            int s = g.players[i].score;
            total += s;
            if (s > high) high = s;
            if (s < lowest_seat_score) lowest_seat_score = s;
        }
        for (int i = 0; i < np; i++) {
            if (g.players[i].score == high) tied++;
        }
        if (tied > 1) {
            ties++;
            if (tied > widest_tie) widest_tie = tied;
        }
        if (high <= 0) max_le_zero++;
        if (total < 0) negative_total++;
        if (total < lowest_total) lowest_total = total;
        if (total > highest_total) highest_total = total;

        // What a running maximum seeded with 0 would have reported.
        int naive_seat = 0, naive_high = 0;
        for (int i = 0; i < np; i++) {
            if (g.players[i].score > naive_high) {
                naive_high = g.players[i].score;
                naive_seat = i;
            }
        }
        if (g.players[naive_seat].score != high) naive_wrong++;
    }

    printf("%d 人局 / %ld 局\n", np, games);
    printf("  并列第一        %6ld 局 (%5.2f%%)，最多 %ld 人并列\n",
           ties, 100.0 * ties / games, widest_tie);
    printf("  最高分 <= 0     %6ld 局 (%5.2f%%)\n", max_le_zero, 100.0 * max_le_zero / games);
    printf("  总分为负        %6ld 局 (%5.2f%%)\n", negative_total, 100.0 * negative_total / games);
    printf("  总分范围        [%d, %d]，单人最低分 %d\n",
           lowest_total, highest_total, lowest_seat_score);
    printf("  高分初值取 0 的实现报出非最高分者：%ld 局 (%5.2f%%)\n\n",
           naive_wrong, 100.0 * naive_wrong / games);
}

int main(void) {
    kusokurae_global_init();
    survey(3, 300000);
    survey(4, 300000);
    if (broken) {
        printf("%d 项结构性不变量被破坏\n", broken);
        return 1;
    }
    return 0;
}
