/*
 * chess-cli — Terminal Chess Puzzles
 *
 * Author: kannachi323
 * Usage:  ./chess-cli
 */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

#include "chess_stats.h"
#include "cli.h"
#include "puzzle.h"
#include "terminal.h"
#include "util.h"

int main(int argc, char **argv) {
    terminal_setup();
    atexit(terminal_restore);

    static PuzzleStore store = {0};
    const char *csv_path = puzzle_find_csv_path(argv[0], argc > 1 ? argv[1] : NULL);
    if (puzzle_store_load(&store, csv_path) <= 0) {
        fprintf(stderr, "Failed to load puzzles from %s\n", csv_path);
        return 1;
    }

    srand((unsigned)time(NULL));
    Stats stats = stats_load();
    puzzle_store_shuffle_random(&store);

    char input[64];

    while (1) {
        int solved_count = stats_count_solved(&stats, store.num_puzzles);

        cli_clear_screen();
        cli_print_menu(&stats, solved_count, store.num_puzzles);
        printf("  %s>%s ", BOLD, RST);
        fflush(stdout);

        if (!fgets(input, sizeof(input), stdin)) {
            break;
        }

        char *choice = trim(input);
        if (is_ignorable_input(choice) || choice[0] == '\0') {
            continue;
        }

        if (strcmp(choice, "q") == 0 || strcmp(choice, "quit") == 0) {
            break;
        }

        if (strcmp(choice, "1") == 0) {
            PuzzleDay day = puzzle_current_day();

            if (puzzle_stats_has_solved_day(&stats, day)) {
                char reset_at[64];
                cli_format_next_potd_reset(reset_at, sizeof(reset_at));
                cli_clear_screen();
                printf("\n  %sPOTD already solved, come back tomorrow!%s\n", FG_DIM, RST);
                printf("  %sResets at %s%s\n\n", FG_DIM, reset_at, RST);
                cli_wait_for_enter();
                continue;
            }

            int potd_idx = puzzle_store_current_potd_index(&store, day);
            int result = cli_play_puzzle(potd_idx, &store.puzzles[potd_idx], &stats,
                                         1, store.num_puzzles);
            if (result == 1) {
                puzzle_stats_mark_day_solved(&stats, day);
            }
            if (result != 0) {
                cli_wait_for_enter();
            }
            stats_save(&stats);
        } else if (strcmp(choice, "2") == 0) {
            int keep_going = 1;

            while (keep_going) {
                int idx = puzzle_store_next_random(&store);
                if (idx < 0) {
                    break;
                }

                int result = cli_play_puzzle(idx, &store.puzzles[idx], &stats,
                                             0, store.num_puzzles);
                stats_save(&stats);
                if (result == 0) {
                    break;
                }

                printf("  %sAnother? [y/n]:%s ", BOLD, RST);
                fflush(stdout);
                if (!fgets(input, sizeof(input), stdin)) {
                    keep_going = 0;
                    break;
                }

                char *yn = trim(input);
                if (is_ignorable_input(yn) || yn[0] == '\0') {
                    continue;
                }

                if (yn[0] != 'y' && yn[0] != 'Y') {
                    keep_going = 0;
                }
            }
        } else if (strcmp(choice, "3") == 0) {
            cli_clear_screen();
            cli_print_stats(&stats, solved_count, store.num_puzzles);
            cli_wait_for_enter();
        } else {
            printf("\n  %sPress 1, 2, 3, or q%s\n", FG_DIM, RST);
        }
    }

    printf("\n  %sGoodbye!%s\n\n", FG_DIM, RST);
    return 0;
}

