#include <cstdio>
#include "sm.h"

void print_card(kusokurae_card_t *p) {
    if (p == NULL) {
        std::printf("NULL\n");
        return;
    }
    if (p->display_order == 0) {
        std::printf("Zero card\n");
        return;
    }
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

#ifndef WHATEVER_YOU_WANT_TO_INDICATE_CGO

int main(int argc, char *argv[]) {
    std::printf("Starting kusokurae engine test...\n\n");
    kusokurae_global_init();
    
    kusokurae_game_config_t cfg = { 4 };
    kusokurae_game_state_t g;
    kusokurae_game_init(&g, &cfg);
    kusokurae_game_start(&g);
}

#endif // WHATEVER_YOU_WANT_TO_INDICATE_CGO
