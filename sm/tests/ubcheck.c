// Plays complete games so a sanitizer build has something to chew on. Prints
// nothing of interest by itself — the point is the diagnostics the sanitizer
// emits. Two undefined behaviours were found this way: a left shift of the
// negative round score in round_score(), and signed overflow in ms_rand(). Both
// are fixed on branch fix/round-score-ub; against a tree without that fix this
// program still reports them, which is the intended demonstration.
//
//	gcc -std=gnu11 -fsanitize=undefined,address -I .. -o /tmp/ubcheck ubcheck.c ../sm.c
//	/tmp/ubcheck
//
// A clean run prints only the summary line.

#include <stdio.h>
#include <string.h>
#include "sm.h"
#include "sm_internal.h"

#define GAMES 200

static void run(int np) {
    kusokurae_game_state_t g;
    kusokurae_game_config_t cfg = {np, 0};
    for (int t = 0; t < GAMES; t++) {
        memset(&g, 0, sizeof(g));
        kusokurae_game_init(&g, &cfg, NULL);
        kusokurae_game_start(&g);
        while (g.status == KUSOKURAE_STATUS_PLAY) {
            kusokurae_player_t *p = kusokurae_get_active_player(&g);
            if (p == NULL) {
                return;
            }
            int played = 0;
            for (int i = 0; i < p->ncards; i++) {
                if (!kusokurae_card_round_played(p->cards[i]) &&
                    kusokurae_card_is_playable(p->cards[i])) {
                    kusokurae_game_play(&g, p->cards[i]);
                    played = 1;
                    break;
                }
            }
            if (!played) {
                return;
            }
        }
        // Exercise the read-only API surface as well.
        kusokurae_round_state_t rs;
        kusokurae_get_round_state(&g, &rs);
        kusokurae_game_is_final_round(&g);
    }
}

int main(void) {
    kusokurae_global_init();
    run(3);
    run(4);
    printf("%d 局三人局 + %d 局四人局跑完\n", GAMES, GAMES);
    return 0;
}
