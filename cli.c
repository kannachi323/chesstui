#include <ctype.h>
#include <stdio.h>
#include <string.h>
#include <time.h>

#include "chess.h"
#include "chess_stats.h"
#include "cli.h"
#include "util.h"

void cli_clear_screen(void) {
    printf("\033[2J\033[H");
    fflush(stdout);
}

void cli_wait_for_enter(void) {
    char input[64];

    printf("  %sPress Enter to continue...%s", FG_DIM, RST);
    fflush(stdout);
    if (!fgets(input, sizeof(input), stdin)) {
        clearerr(stdin);
    }
}

static int read_prompt_input(char *input, size_t inputsz) {
    if (!fgets(input, (int)inputsz, stdin)) {
        return 0;
    }

    char *answer = trim(input);

    if (is_ignorable_input(answer) || answer[0] == '\0') {
        return -1;
    }

    return 1;
}

void cli_format_next_potd_reset(char *buf, size_t bufsz) {
    time_t now = time(NULL);
    struct tm reset_tm = *localtime(&now);

    reset_tm.tm_hour = 0;
    reset_tm.tm_min = 0;
    reset_tm.tm_sec = 0;
    reset_tm.tm_mday += 1;
    mktime(&reset_tm);

    strftime(buf, bufsz, "%a %b %d, %I:%M %p %Z", &reset_tm);
}

void cli_print_menu(const Stats *stats, int solved_count, int total_puzzles) {
    printf("\n");
    printf("  %s╔══════════════════════════════════════╗%s\n", FG_TITLE, RST);
    printf("  %s║%s      %s♟  c h e s s - c l i  ♟%s      %s║%s\n",
           FG_TITLE, RST, BOLD, RST, FG_TITLE, RST);
    printf("  %s╚══════════════════════════════════════╝%s\n", FG_TITLE, RST);
    printf("\n");
    printf("    %s[1]%s  Puzzle of the Day\n", BOLD, RST);
    printf("    %s[2]%s  Random Puzzle\n", BOLD, RST);
    printf("    %s[3]%s  Stats\n", BOLD, RST);
    printf("    %s[q]%s  Quit\n", BOLD, RST);
    printf("\n");
    if (stats->total_attempted > 0) {
        printf("    %s🔥 %d streak | %d/%d solved%s\n",
               FG_DIM, stats->current_streak, solved_count, total_puzzles, RST);
    }
    printf("\n");
}

void cli_print_stats(const Stats *stats, int solved_count, int total_puzzles) {
    printf("\n");
    printf("  %s── Stats ──────────────────────────────%s\n\n", FG_TITLE, RST);
    printf("    Current streak:  %s%d%s\n", BOLD, stats->current_streak, RST);
    printf("    Best streak:     %s%d%s\n", BOLD, stats->best_streak, RST);
    printf("    Puzzles solved:  %s%d/%d%s\n", BOLD, solved_count, total_puzzles, RST);
    printf("    Total solved:    %s%d%s\n", BOLD, stats->total_solved, RST);
    printf("    Total attempts:  %s%d%s\n", BOLD, stats->total_attempted, RST);
    if (stats->total_attempted > 0) {
        printf("    Accuracy:        %s%.0f%%%s\n", BOLD,
               100.0 * stats->total_solved / stats->total_attempted, RST);
    }
    printf("\n");
}

int cli_play_puzzle(int puzzle_idx, const Puzzle *puzzle, Stats *stats,
                    int is_potd, int total_puzzles) {
    char move_list[MOVE_LIST_MAX][MOVE_LEN];
    int nmoves = chess_split_moves(puzzle->moves, move_list, MOVE_LIST_MAX);
    int setup_moves = (nmoves > 1) ? 1 : 0;
    int expected_idx = setup_moves;
    int total_player_moves = 0;

    if (nmoves <= expected_idx) {
        fprintf(stderr, "Puzzle %s has too few moves\n", puzzle->id);
        return -1;
    }

    chess_set_position(puzzle->fen);
    for (int i = 0; i < setup_moves; i++) {
        chess_apply_uci_move(move_list[i]);
    }

    char side = chess_fen_side(puzzle->fen);
    char player_side = side;
    for (int i = 0; i < setup_moves; i++) {
        player_side = (player_side == 'w') ? 'b' : 'w';
    }
    int flipped = (player_side == 'b');
    for (int i = expected_idx; i < nmoves; i += 2) {
        total_player_moves++;
    }

    stats->total_attempted++;

    for (int step = 0, i = expected_idx; i < nmoves; i += 2, step++) {
        const char *expected = move_list[i];
        char input[64];

        for (;;) {
            cli_clear_screen();

            if (is_potd) {
                printf("\n  %s♟ Puzzle of the Day%s", BOLD, RST);
            } else {
                printf("\n  %s♟ Random Puzzle%s", BOLD, RST);
            }

            printf("  %s#%s  ⭐ %d%s\n", FG_DIM, puzzle->id, puzzle->rating, RST);
            printf("  %s%s to move — find the best move!%s\n",
                   FG_HINT, player_side == 'w' ? "White" : "Black", RST);
            if (strlen(puzzle->themes) > 0) {
                printf("  %sTheme: %s%s\n", FG_DIM, puzzle->themes, RST);
            }
            printf("  %sStep %d/%d%s\n", FG_DIM, step + 1, total_player_moves, RST);

            chess_print_board(flipped);
            printf("  %sYour move%s %s(UCI like e2e4, or SAN like Nf3):%s ",
                   BOLD, RST, FG_DIM, RST);
            fflush(stdout);

            int read_status = read_prompt_input(input, sizeof(input));
            if (read_status == 0) {
                return 0;
            }
            if (read_status < 0) {
                continue;
            }

            char *answer = trim(input);

            if (strcmp(answer, "q") == 0 || strcmp(answer, "quit") == 0) {
                return 0;
            }

            if (strcmp(answer, "hint") == 0) {
                printf("  %sHint:%s the piece starts on %c%c\n",
                       FG_HINT, RST, expected[0], expected[1]);
                printf("  %sPress Enter to try again...%s", FG_DIM, RST);
                fflush(stdout);
                if (!fgets(input, sizeof(input), stdin)) {
                    return 0;
                }
                continue;
            }

            if (!chess_move_matches_expected(answer, expected)) {
                cli_clear_screen();
                printf("\n  %s✗ Not quite.%s\n", FG_FAIL, RST);
                chess_print_board(flipped);
                printf("  %s[r]%s Try again  %s[s]%s Show solution\n\n",
                       BOLD, RST, BOLD, RST);
                printf("  %s>%s ", BOLD, RST);
                fflush(stdout);

                char retry[64];
                if (!fgets(retry, sizeof(retry), stdin)) {
                    return 0;
                }
                char *rc = trim(retry);
                if (rc[0] == 's' || rc[0] == 'S') {
                    cli_clear_screen();
                    printf("\n  %s✗ The best move was %s%s%s\n",
                           FG_FAIL, BOLD, expected, RST);
                    chess_apply_uci_move(expected);
                    chess_print_board(flipped);

                    stats->current_streak = 0;
                    printf("  %s🔥 Streak: %d | Best: %d | Solved: %d/%d%s\n\n",
                           FG_DIM, stats->current_streak, stats->best_streak,
                           stats_count_solved(stats, total_puzzles), total_puzzles,
                           RST);
                    return -1;
                }
                continue;
            }

            cli_clear_screen();
            chess_apply_uci_move(expected);
            break;
        }

        if (i + 1 < nmoves) {
            chess_apply_uci_move(move_list[i + 1]);
        }
    }

    cli_clear_screen();
    printf("\n  %s✓ Correct!%s  You solved the full line.\n", FG_OK, RST);
    chess_print_board(flipped);

    stats->total_solved++;
    if (puzzle_idx >= 0 && puzzle_idx < MAX_PUZZLES) {
        stats->solved[puzzle_idx] = 1;
    }
    stats->current_streak++;
    if (stats->current_streak > stats->best_streak) {
        stats->best_streak = stats->current_streak;
    }

    printf("  %s🔥 Streak: %d | Best: %d | Solved: %d/%d%s\n\n",
           FG_DIM, stats->current_streak, stats->best_streak,
           stats_count_solved(stats, total_puzzles), total_puzzles, RST);

    return 1;
}
