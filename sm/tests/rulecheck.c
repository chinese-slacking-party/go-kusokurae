// Rule conformance checks for the C engine, driven through the public API with
// hands written directly so that a specific trick can be set up (the engine
// deals at random). Each check states the rule it comes from.
//
// Checks known to fail against the current engine are marked XFAIL: they do not
// affect the exit code, but if one starts passing it is reported so the marker
// can be removed. Any other failure exits non-zero.
//
//	gcc -std=gnu11 -I .. -o /tmp/rulecheck rulecheck.c ../sm.c && /tmp/rulecheck

#include <stdio.h>
#include <string.h>
#include "sm.h"
#include "sm_internal.h"

static int failures, xfailures, xpasses;

#define ANGEL    (kusokurae_card_t){33, KUSOKURAE_SUIT_BAOZI, 10, MASK_PLAYABLE}
#define GHOST    (kusokurae_card_t){31, KUSOKURAE_SUIT_OTHER, 10, MASK_PLAYABLE}
#define BAOZI(r) (kusokurae_card_t){(unsigned)(21 + (r)), KUSOKURAE_SUIT_BAOZI,   (r), MASK_PLAYABLE}
#define XIANG(r) (kusokurae_card_t){(unsigned)( 1 + (r)), KUSOKURAE_SUIT_XIANG,   (r), MASK_PLAYABLE}

// Deals each player the single card they are about to play, then plays them in
// seat order and reports the resulting scores.
static void play_one_trick(kusokurae_game_state_t *g, int np, const kusokurae_card_t *hand) {
    kusokurae_game_config_t cfg = {np, 0};
    memset(g, 0, sizeof(*g));
    kusokurae_game_init(g, &cfg, NULL);
    kusokurae_game_start(g);
    for (int i = 0; i < np; i++) {
        memset(g->players[i].cards, 0, sizeof(g->players[i].cards));
        g->players[i].cards[0] = hand[i];
        g->players[i].ncards = 1;
        g->players[i].score = 0;
        g->players[i].cards_taken = 0;
    }
    memset(&g->current_round, 0, sizeof(g->current_round));
    g->high_ranker_index = -1;
    g->nround = 0;
    for (int i = 0; i < np; i++) {
        kusokurae_error_t e = kusokurae_game_play(g, hand[i]);
        if (e != KUSOKURAE_SUCCESS) {
            printf("  !! %dP 出牌返回错误 %d\n", i + 1, e);
        }
    }
}

// want_winner and want_score describe the seat that should take the trick and
// the score it should collect.
static void check_trick(const char *rule, const char *desc, int xfail,
                        int np, const kusokurae_card_t *hand,
                        int want_winner, int want_score) {
    kusokurae_game_state_t g;
    play_one_trick(&g, np, hand);

    int got_winner = -1, got_score = 0;
    for (int i = 0; i < np; i++) {
        if (g.players[i].score != 0 || g.players[i].cards_taken != 0) {
            got_winner = i;
            got_score = g.players[i].score;
        }
    }
    int ok = (got_winner == want_winner && got_score == want_score);
    if (ok && xfail) {
        printf("XPASS %s | %s —— 已经通过，请去掉 XFAIL 标记\n", rule, desc);
        xpasses++;
    } else if (ok) {
        printf("ok    %s | %s\n", rule, desc);
    } else if (xfail) {
        printf("XFAIL %s | %s\n        期望 %dP 得 %d 分，实际 %dP 得 %d 分（已知缺陷）\n",
               rule, desc, want_winner + 1, want_score, got_winner + 1, got_score);
        xfailures++;
    } else {
        printf("FAIL  %s | %s\n        期望 %dP 得 %d 分，实际 %dP 得 %d 分\n",
               rule, desc, want_winner + 1, want_score, got_winner + 1, got_score);
        failures++;
    }
}

int main(void) {
    kusokurae_global_init();

    check_trick("规则 3", "同点数先出者为大", 0, 3,
                (kusokurae_card_t[]){BAOZI(7), XIANG(7), BAOZI(3)}, 0, 1 + -1 + 1);

    check_trick("规则 3", "点数大者吃墩", 0, 3,
                (kusokurae_card_t[]){BAOZI(3), BAOZI(9), XIANG(5)}, 1, 1 + 1 + -1);

    check_trick("规则 3/6", "天使大于普通牌，且计 +1 分", 0, 3,
                (kusokurae_card_t[]){BAOZI(9), ANGEL, XIANG(5)}, 1, 1 + 1 + -1);

    check_trick("规则 6", "鬼牌先出：吃墩并把正分翻倍", 0, 3,
                (kusokurae_card_t[]){GHOST, ANGEL, BAOZI(5)}, 0, (0 + 1 + 1) * 2);

    check_trick("规则 6", "鬼牌先出：负分同样翻倍", 0, 3,
                (kusokurae_card_t[]){GHOST, XIANG(5), XIANG(3)}, 0, (0 + -1 + -1) * 2);

    check_trick("规则 6", "无鬼牌则不翻倍", 0, 3,
                (kusokurae_card_t[]){BAOZI(9), XIANG(5), XIANG(3)}, 0, 1 + -1 + -1);

    // 规则 3 把鬼牌排在天使之上，但引擎给两者相同的 rank（10），于是按
    // “先出者为大”归了天使。见对话记录中的问题 1。
    check_trick("规则 3", "鬼牌大于天使（后出也应吃墩）", 1, 3,
                (kusokurae_card_t[]){ANGEL, GHOST, BAOZI(5)}, 1, (1 + 0 + 1) * 2);

    check_trick("规则 6", "鬼牌后出时的翻倍", 1, 3,
                (kusokurae_card_t[]){ANGEL, GHOST, XIANG(3)}, 1, (1 + 0 + -1) * 2);

    printf("\n%d 项失败，%d 项已知缺陷，%d 项意外通过\n", failures, xfailures, xpasses);
    return failures ? 1 : 0;
}
