#include <cstdio>
#include <ctime>
#include "sm.h"
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

#ifndef WHATEVER_YOU_WANT_TO_INDICATE_CGO

int main(int argc, char *argv[]) {
    test_init();

    kusokurae_game_config_t cfg = { 3 };
    kusokurae_game_state_t g;
    kusokurae_game_init(&g, &cfg);

    test_start(&g);
    std::printf("\n%dP has the ghost\n", g.ghost_holder_index + 1);
}

#endif // WHATEVER_YOU_WANT_TO_INDICATE_CGO
