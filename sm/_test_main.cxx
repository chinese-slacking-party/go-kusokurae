// Standalone driver for the C engine, independent of the Go bindings.
// The leading underscore keeps the go tool from compiling this file into
// package sm: cgo provides no macro to detect itself, and a file the tool
// ignores needs no such guard. It also keeps a C++ toolchain out of the
// requirements for building the Go package.
//
//	gcc -std=gnu11   -I sm -c sm/sm.c -o /tmp/sm.o
//	g++ -std=gnu++11 -I sm -o /tmp/kusokurae_test sm/_test_main.cxx /tmp/sm.o

#include <cstdio>
#include <cstring>
#include <ctime>

#include "sm_internal.h"

void print_card(kusokurae_card_t *p) {
    if (p == NULL) {
        std::printf("NULL\n");
        return;
    }
    if (p->display_order == 0) {
        std::printf("Zero card\n");
        return;
    }

    // If the card is played (not in hand any longer), mark it
    int rp = kusokurae_card_round_played(*p);
    if (rp > 0) {
        std::printf("(%d)~", rp);
    }

    // Print suit and rank
    switch (p->suit) {
    case KUSOKURAE_SUIT_OTHER:
        std::printf("Ghost\n");
        return;
    case KUSOKURAE_SUIT_BAOZI:
        std::printf("Peace ");
        break;
    case KUSOKURAE_SUIT_YOUTIAO:
        std::printf("Calm ");
        break;
    case KUSOKURAE_SUIT_XIANG:
        std::printf("Anger ");
        break;
    default:
        std::printf("Invalid suit %d\n", p->suit);
        return;
    }
    std::printf("%d\n", p->rank);
}

void test_init() {
    std::printf("Starting kusokurae engine test @ %ld...\n\n", time(0));
    std::printf("Sizes of data structures:\n");
    std::printf("kusokurae_game_config_t: %lu\n", sizeof(kusokurae_game_config_t));
    std::printf("kusokurae_card_t: %lu\n", sizeof(kusokurae_card_t));
    std::printf("kusokurae_player_t: %lu\n", sizeof(kusokurae_player_t));
    std::printf("kusokurae_game_state_t: %lu\n", sizeof(kusokurae_game_state_t));
    std::printf("kusokurae_round_state_t: %lu\n", sizeof(kusokurae_round_state_t));
    kusokurae_global_init();
}

void print_all_card_slots(kusokurae_game_state_t *g) {
    int i, j;
    for (i = 0; i < KUSOKURAE_MAX_PLAYERS && i < g->cfg.np; i++) {
        std::printf("\n%dP's cards:\n", i + 1);
        for (j = 0; j < KUSOKURAE_MAX_HAND_CARDS && j < g->players[i].ncards; j++) {
            print_card(&g->players[i].cards[j]);
        }
    }
}

void test_start(kusokurae_game_state_t *g) {
    //print_all_card_slots(g);
    kusokurae_game_start(g);
    print_all_card_slots(g);
}

void dummy_state_cb(kusokurae_game_state_t *self, int32_t newstate, void *userdata) {
    std::printf("dummy_state_cb(%p, %d, %p)\n", self, newstate, userdata);
}

// Plays the game out to the end, always choosing the first playable card.
static void play_out(kusokurae_game_state_t *g) {
    while (g->status == KUSOKURAE_STATUS_PLAY) {
        kusokurae_player_t *p = kusokurae_get_active_player(g);
        if (p == NULL) {
            return;
        }
        for (int i = 0; i < p->ncards; i++) {
            if (!kusokurae_card_round_played(p->cards[i]) &&
                kusokurae_card_is_playable(p->cards[i])) {
                kusokurae_game_play(g, p->cards[i]);
                break;
            }
        }
    }
}

// kusokurae_game_start() must clear the previous tally: sm.h documents starting
// a new game on a finished state, and the caller's struct is not required to be
// zeroed in the first place. Returns the number of failures.
static int test_restart_resets_score() {
    int failures = 0;
    // Empty braces rather than { 3 }: the config carries a reserved
    // array now, and a partial brace list makes -Wextra complain in C++.
    kusokurae_game_config_t cfg = {};
    cfg.np = 3;
    kusokurae_game_callbacks_t cbs = { NULL, NULL };

    // Case 1: a caller-supplied struct full of garbage.
    kusokurae_game_state_t g;
    std::memset(&g, 0xCC, sizeof(g));
    kusokurae_game_init(&g, &cfg, &cbs);
    kusokurae_game_start(&g);
    for (int i = 0; i < cfg.np; i++) {
        if (g.players[i].score != 0 || g.players[i].cards_taken != 0) {
            std::printf("FAIL: fresh start, %dP score=%d cards_taken=%d, want 0/0\n",
                        i + 1, g.players[i].score, g.players[i].cards_taken);
            failures++;
        }
    }

    // Case 2: a second game started on the finished state of the first.
    play_out(&g);
    int carried = 0;
    for (int i = 0; i < cfg.np; i++) {
        carried += g.players[i].score;
    }
    kusokurae_game_start(&g);
    for (int i = 0; i < cfg.np; i++) {
        if (g.players[i].score != 0 || g.players[i].cards_taken != 0) {
            std::printf("FAIL: restart carried over, %dP score=%d cards_taken=%d, want 0/0\n",
                        i + 1, g.players[i].score, g.players[i].cards_taken);
            failures++;
        }
    }
    std::printf("test_restart_resets_score: %s (first game totalled %d)\n",
                failures ? "FAIL" : "ok", carried);
    return failures;
}

int main(int argc, char *argv[]) {
    test_init();

    // Empty braces rather than { 3 }: the config carries a reserved
    // array now, and a partial brace list makes -Wextra complain in C++.
    kusokurae_game_config_t cfg = {};
    cfg.np = 3;
    kusokurae_game_state_t g;
    kusokurae_game_callbacks_t cbs = { &g, &dummy_state_cb };
    kusokurae_game_init(&g, &cfg, &cbs);

    test_start(&g);
    std::printf("\n%dP has the ghost\n", g.ghost_holder_index[0] + 1);

    std::printf("\n");
    return test_restart_resets_score() ? 1 : 0;
}
