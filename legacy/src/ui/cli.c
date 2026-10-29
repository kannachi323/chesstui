#include <ctype.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <unistd.h>

#include "chess.h"
#include "chess_stats.h"
#include "cli.h"
#include "http_client.h"
#include "lichess.h"
#include "puzzle.h"
#include "terminal.h"
#include "util.h"

#define REPLAY_FRAME_MAX (MOVE_LIST_MAX + 2)
#define REPLAY_LABEL_LEN 32
#define REPLAY_STATUS_LEN 128
#define REPLAY_INPUT_LEN 64
#define PLAYER_MOVE_DELAY_MS 450
#define OPPONENT_MOVE_DELAY_MS 700

typedef struct {
    char board[CHESS_BOARD_SNAPSHOT_SIZE];
    char label[REPLAY_LABEL_LEN];
} ReplayFrame;

void cli_clear_screen(void) {
    printf("\033[H\033[J");
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

const char *cli_select_csv(void) {
    static char paths[PUZZLE_CSV_MAX][PUZZLE_PATH_MAX];
    static char selected[PUZZLE_PATH_MAX];
    char input[64];

    int count = puzzle_list_csvs(paths, PUZZLE_CSV_MAX);

    if (count == 0) {
        cli_clear_screen();
        printf("\n  %sNo puzzle sets found.%s\n", FG_FAIL, RST);
        printf("  %sAdd .csv files to ~/.chess-cli/puzzles/%s\n\n", FG_DIM, RST);
        return NULL;
    }

    if (count == 1) {
        snprintf(selected, sizeof(selected), "%s", paths[0]);
        return selected;
    }

    for (;;) {
        cli_clear_screen();
        printf("\n  %sSelect a puzzle set:%s\n\n", BOLD, RST);
        for (int i = 0; i < count; i++) {
            const char *slash = strrchr(paths[i], '/');
            const char *name = slash ? slash + 1 : paths[i];
            char display[PUZZLE_PATH_MAX];
            strncpy(display, name, sizeof(display) - 1);
            display[sizeof(display) - 1] = '\0';
            size_t dlen = strlen(display);
            if (dlen > 4 && strcmp(display + dlen - 4, ".csv") == 0) {
                display[dlen - 4] = '\0';
            }
            printf("    %s[%d]%s  %s\n", BOLD, i + 1, RST, display);
        }
        printf("\n  %s>%s ", BOLD, RST);
        fflush(stdout);

        if (!fgets(input, sizeof(input), stdin)) {
            return NULL;
        }

        char *choice = trim(input);
        if (is_ignorable_input(choice) || choice[0] == '\0') {
            continue;
        }
        if (strcmp(choice, "q") == 0 || strcmp(choice, "quit") == 0) {
            return NULL;
        }

        int idx = atoi(choice) - 1;
        if (idx >= 0 && idx < count) {
            snprintf(selected, sizeof(selected), "%s", paths[idx]);
            return selected;
        }
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

static void replay_push_frame(ReplayFrame frames[], int *count, const char *label) {
    if (*count >= REPLAY_FRAME_MAX) {
        return;
    }

    chess_get_board_snapshot(frames[*count].board);
    snprintf(frames[*count].label, sizeof(frames[*count].label), "%s", label);
    (*count)++;
}

static int cli_is_jump_command(const char *input, int *target_idx) {
    int value = 0;

    if (input[0] != 'j' && input[0] != 'J') {
        return 0;
    }
    if (input[1] == '\0') {
        return 0;
    }

    for (int i = 1; input[i] != '\0'; i++) {
        if (!isdigit((unsigned char) input[i])) {
            return 0;
        }
        value = value * 10 + (input[i] - '0');
    }

    *target_idx = value;
    return 1;
}

static void cli_render_puzzle_view(const Puzzle *puzzle, char player_side,
                                   int flipped,
                                   const ReplayFrame frames[], int replay_idx,
                                   int live_idx, const char *input,
                                   const char *status, int show_help) {
    cli_clear_screen();

    printf("\n  %s⭐ %d%s  %s%s to move%s\n",
           FG_DIM, puzzle->rating, RST,
           FG_HINT, player_side == 'w' ? "White" : "Black", RST);

    if (replay_idx == live_idx) {
        printf("  %sLive board%s\n", FG_DIM, RST);
    } else {
        printf("  %sReplay %d/%d: %s%s\n",
               FG_DIM, replay_idx + 1, live_idx + 1, frames[replay_idx].label, RST);
    }

    chess_set_board_snapshot(frames[replay_idx].board);
    chess_print_board(flipped);

    if (status && status[0] != '\0') {
        printf("  %s%s%s\n", FG_DIM, status, RST);
    }

    if (show_help) {
        printf("  %s←/→%s replay  %sj<n>%s jump  %shint%s hint  %sq%s quit\n",
               BOLD, RST, BOLD, RST, BOLD, RST, BOLD, RST);
    } else {
        printf("  %s? for controls%s\n", FG_DIM, RST);
    }

    printf("  Move: %s", input);
    fflush(stdout);
}


static int cli_read_replayable_input(const Puzzle *puzzle, char player_side,
                                     int flipped,
                                     const ReplayFrame frames[], int live_idx,
                                     const char *status, char *input, size_t inputsz) {
    size_t len = 0;
    int replay_idx = live_idx;
    int show_help = 0;
    char status_buf[REPLAY_STATUS_LEN];

    input[0] = '\0';
    snprintf(status_buf, sizeof(status_buf), "%s", status ? status : "");

    if (!terminal_enable_raw_mode()) {
        chess_set_board_snapshot(frames[live_idx].board);
        return read_prompt_input(input, inputsz);
    }

    for (;;) {
        int key;

        cli_render_puzzle_view(puzzle, player_side, flipped,
                               frames, replay_idx, live_idx, input, status_buf, show_help);

        key = terminal_read_key();
        if (key == TERMINAL_KEY_UNKNOWN) {
            continue;
        }
        if (key == TERMINAL_KEY_LEFT) {
            if (replay_idx > 0) {
                replay_idx--;
            }
            continue;
        }
        if (key == TERMINAL_KEY_RIGHT) {
            if (replay_idx < live_idx) {
                replay_idx++;
            }
            continue;
        }
        if (key == TERMINAL_KEY_BACKSPACE) {
            if (len > 0) {
                len--;
                input[len] = '\0';
            }
            if (replay_idx != live_idx) {
                replay_idx = live_idx;
            }
            continue;
        }
        if (key == TERMINAL_KEY_ENTER) {
            int jump_target = 0;

            if (cli_is_jump_command(input, &jump_target)) {
                if (jump_target >= 0 && jump_target <= live_idx) {
                    replay_idx = jump_target;
                    snprintf(status_buf, sizeof(status_buf), "Jumped to move %d.", jump_target);
                } else {
                    snprintf(status_buf, sizeof(status_buf), "Move %d is out of range.", jump_target);
                }
                len = 0;
                input[0] = '\0';
                continue;
            }

            terminal_disable_raw_mode();
            chess_set_board_snapshot(frames[live_idx].board);
            if (len == 0 || is_ignorable_input(input)) {
                return -1;
            }
            return 1;
        }
        if (key == '?') {
            show_help = !show_help;
            continue;
        }
        if (key == 3 || key == 4) {
            terminal_disable_raw_mode();
            chess_set_board_snapshot(frames[live_idx].board);
            return 0;
        }
        if (key >= 32 && key <= 126) {
            if (replay_idx != live_idx) {
                replay_idx = live_idx;
            }
            if (len + 1 < inputsz) {
                input[len++] = (char) key;
                input[len] = '\0';
            }
        }
    }
}

static void cli_show_replay_summary(const Puzzle *puzzle, int is_potd, int flipped,
                                    const ReplayFrame frames[], int frame_count,
                                    const char *headline, const char *summary) {
    int replay_idx = frame_count - 1;

    if (frame_count <= 0) {
        return;
    }

    if (!terminal_enable_raw_mode()) {
        return;
    }

    for (;;) {
        int key;

        cli_clear_screen();
        printf("\n  %s%s%s\n", BOLD, headline, RST);
        if (is_potd) {
            printf("  %sPuzzle of the Day%s", FG_DIM, RST);
        } else {
            printf("  %sRandom Puzzle%s", FG_DIM, RST);
        }
        printf("  %s#%s  ⭐ %d%s\n", FG_DIM, puzzle->id, puzzle->rating, RST);
        printf("  %s%s%s\n", FG_DIM, summary, RST);
        printf("  %sReplay %d/%d: %s%s\n",
               FG_DIM, replay_idx + 1, frame_count, frames[replay_idx].label, RST);

        chess_set_board_snapshot(frames[replay_idx].board);
        chess_print_board(flipped);
        printf("  %sControls:%s \342\206\220/\342\206\222 replay  j<num> jump  Enter continue\n",
               FG_DIM, RST);
        fflush(stdout);

        key = terminal_read_key();
        if (key == TERMINAL_KEY_LEFT) {
            if (replay_idx > 0) {
                replay_idx--;
            }
            continue;
        }
        if (key == TERMINAL_KEY_RIGHT) {
            if (replay_idx + 1 < frame_count) {
                replay_idx++;
            }
            continue;
        }
        if (key == 'j' || key == 'J') {
            char input[REPLAY_INPUT_LEN];
            int len = 0;

            input[len++] = (char) key;
            input[len] = '\0';
            for (;;) {
                cli_clear_screen();
                printf("\n  %s%s%s\n", BOLD, headline, RST);
                if (is_potd) {
                    printf("  %sPuzzle of the Day%s", FG_DIM, RST);
                } else {
                    printf("  %sRandom Puzzle%s", FG_DIM, RST);
                }
                printf("  %s#%s  ⭐ %d%s\n", FG_DIM, puzzle->id, puzzle->rating, RST);
                printf("  %s%s%s\n", FG_DIM, summary, RST);
                printf("  %sReplay %d/%d: %s%s\n",
                       FG_DIM, replay_idx + 1, frame_count, frames[replay_idx].label, RST);
                chess_set_board_snapshot(frames[replay_idx].board);
                chess_print_board(flipped);
                printf("  %sJump:%s %s\n", FG_DIM, RST, input);
                fflush(stdout);

                key = terminal_read_key();
                if (key == TERMINAL_KEY_ENTER) {
                    int jump_target = 0;

                    if (cli_is_jump_command(input, &jump_target)
                        && jump_target >= 0 && jump_target < frame_count) {
                        replay_idx = jump_target;
                    }
                    break;
                }
                if (key == TERMINAL_KEY_BACKSPACE) {
                    if (len > 1) {
                        len--;
                        input[len] = '\0';
                    }
                    continue;
                }
                if (key >= 32 && key <= 126 && len + 1 < (int) sizeof(input)) {
                    input[len++] = (char) key;
                    input[len] = '\0';
                }
            }
            continue;
        }
        if (key == TERMINAL_KEY_ENTER || key == 'q' || key == 'Q') {
            break;
        }
    }

    terminal_disable_raw_mode();
    chess_set_board_snapshot(frames[frame_count - 1].board);
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

static void cli_print_screen_tabs(AppScreen current) {
    static const char *labels[SCREEN_COUNT] = { "Home", "Stats", "Account", "Play" };
    printf("  ");
    for (int i = 0; i < SCREEN_COUNT; i++) {
        if (i == (int)current) {
            printf("%s%s%s", BOLD, labels[i], RST);
        } else {
            printf("%s%s%s", FG_DIM, labels[i], RST);
        }
        if (i < SCREEN_COUNT - 1) {
            printf("  ");
        }
    }
    printf("  %s← →%s\n", FG_DIM, RST);
}

void cli_print_menu(const Stats *stats, int solved_count, int total_puzzles,
                    AppScreen screen) {
    printf("\n");
    printf("  %s╔══════════════════════════════════════╗%s\n", FG_TITLE, RST);
    printf("  %s║%s      %s♟  c h e s s - c l i  ♟%s      %s║%s\n",
           FG_TITLE, RST, BOLD, RST, FG_TITLE, RST);
    printf("  %s╚══════════════════════════════════════╝%s\n", FG_TITLE, RST);
    printf("\n");
    cli_print_screen_tabs(screen);
    printf("\n");
    printf("    %s[1]%s  Puzzle of the Day\n", BOLD, RST);
    printf("    %s[2]%s  Puzzles\n", BOLD, RST);
    printf("    %s[q]%s  Quit\n", BOLD, RST);
    printf("\n");
    if (stats->total_attempted > 0) {
        printf("    %s🔥 %d streak | %d/%d solved%s\n",
               FG_DIM, stats->current_streak, solved_count, total_puzzles, RST);
    }
    printf("\n");
}

void cli_print_stats(const Stats *stats, int solved_count, int total_puzzles,
                     AppScreen screen) {
    printf("\n");
    printf("  %s╔══════════════════════════════════════╗%s\n", FG_TITLE, RST);
    printf("  %s║%s      %s♟  c h e s s - c l i  ♟%s      %s║%s\n",
           FG_TITLE, RST, BOLD, RST, FG_TITLE, RST);
    printf("  %s╚══════════════════════════════════════╝%s\n", FG_TITLE, RST);
    printf("\n");
    cli_print_screen_tabs(screen);
    printf("\n");
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

void cli_print_account(const LichessAccount *acc, AppScreen screen, AccountTab tab) {
    static const char *tab_labels[ACCOUNT_TAB_COUNT] = {
        "Puzzles", "Bullet", "Blitz", "Rapid", "Classical"
    };

    printf("\n");
    printf("  %s╔══════════════════════════════════════╗%s\n", FG_TITLE, RST);
    printf("  %s║%s      %s♟  c h e s s - c l i  ♟%s      %s║%s\n",
           FG_TITLE, RST, BOLD, RST, FG_TITLE, RST);
    printf("  %s╚══════════════════════════════════════╝%s\n", FG_TITLE, RST);
    printf("\n");
    cli_print_screen_tabs(screen);
    printf("\n");

    if (!acc || !acc->username[0]) {
        printf("  %sNo Lichess account linked.%s\n\n", FG_DIM, RST);
        printf("    %s[a]%s  Add account\n", BOLD, RST);
        printf("\n");
        return;
    }

    printf("  %s♟  %s%s%s\n\n", FG_HINT, BOLD, acc->username, RST);

    /* Sub-tab bar */
    printf("  ");
    for (int i = 0; i < ACCOUNT_TAB_COUNT; i++) {
        if (i == (int)tab) {
            printf("%s%s%s", BOLD, tab_labels[i], RST);
        } else {
            printf("%s%s%s", FG_DIM, tab_labels[i], RST);
        }
        if (i < ACCOUNT_TAB_COUNT - 1) printf("  ");
    }
    printf("  %s↑↓%s\n\n", FG_DIM, RST);

    /* Stats for current tab */
    const PerfStat *p = &acc->perfs[tab];
    if (p->games == 0) {
        printf("    %sNo games played.%s\n", FG_DIM, RST);
    } else {
        printf("    Rating:   %s%d%s\n", BOLD, p->rating, RST);
        printf("    Games:    %s%d%s\n", BOLD, p->games, RST);
    }

    printf("\n");
    printf("    %s[x]%s  Remove account\n", BOLD, RST);
    printf("\n");
}

int cli_add_account(LichessAccount *out) {
    char input[ACCOUNT_TOKEN_MAX];

    cli_clear_screen();
    printf("\n  %sAdd Lichess Account%s\n\n", BOLD, RST);
    printf("  %sGenerate a token at:%s\n", FG_DIM, RST);
    printf("  %slichess.org/account/oauth/token%s\n\n", FG_HINT, RST);
    printf("  Token: ");
    fflush(stdout);

    if (!fgets(input, sizeof(input), stdin)) {
        return 0;
    }

    char *tok = trim(input);
    if (!tok || tok[0] == '\0') {
        return 0;
    }

    snprintf(out->token, sizeof(out->token), "%s", tok);
    out->username[0] = '\0';
    memset(out->perfs, 0, sizeof(out->perfs));

    cli_clear_screen();
    printf("\n  %sConnecting...%s\n\n", FG_DIM, RST);
    fflush(stdout);

    if (!account_fetch(out) || !out->username[0]) {
        printf("  %sInvalid token or network error.%s\n\n", FG_FAIL, RST);
        out->token[0] = '\0';
        cli_wait_for_enter();
        return 0;
    }

    if (!account_save(out->username, out->token)) {
        printf("  %sFailed to save token.%s\n\n", FG_FAIL, RST);
        cli_wait_for_enter();
        return 0;
    }

    printf("  %s✓ Linked as %s%s%s\n\n", FG_OK, BOLD, out->username, RST);
    cli_wait_for_enter();
    return 1;
}

void cli_print_play(AppScreen screen, int selected_tc, int has_account) {
    printf("\n");
    printf("  %s╔══════════════════════════════════════╗%s\n", FG_TITLE, RST);
    printf("  %s║%s      %s♟  c h e s s - c l i  ♟%s      %s║%s\n",
           FG_TITLE, RST, BOLD, RST, FG_TITLE, RST);
    printf("  %s╚══════════════════════════════════════╝%s\n", FG_TITLE, RST);
    printf("\n");
    cli_print_screen_tabs(screen);
    printf("\n");

    if (!has_account) {
        printf("  %sNo Lichess account linked.%s\n", FG_DIM, RST);
        printf("  %sGo to Account to add your token.%s\n\n", FG_DIM, RST);
        return;
    }

    printf("  %sSelect a time control:%s\n\n", FG_DIM, RST);

    for (int i = 0; i < LICHESS_TC_COUNT; i++) {
        if (i == selected_tc) {
            printf("  %s▶  %s%s\n", BOLD, lichess_time_controls[i].label, RST);
        } else {
            printf("  %s   %s%s\n", FG_DIM, lichess_time_controls[i].label, RST);
        }
    }

    printf("\n  %s↑↓ select   Enter seek%s\n", FG_DIM, RST);
    printf("\n");
}

int cli_play_puzzle(int puzzle_idx, const Puzzle *puzzle, Stats *stats,
                    int is_potd, int total_puzzles, int has_setup_move) {
    char move_list[MOVE_LIST_MAX][MOVE_LEN];
    ReplayFrame frames[REPLAY_FRAME_MAX];
    char status[REPLAY_STATUS_LEN];
    int nmoves = chess_split_moves(puzzle->moves, move_list, MOVE_LIST_MAX);
    int setup_moves = has_setup_move ? 1 : 0;
    int expected_idx = setup_moves;
    int frame_count = 0;

    if (nmoves <= expected_idx) {
        fprintf(stderr, "Puzzle %s has too few moves\n", puzzle->id);
        return -1;
    }

    chess_set_position(puzzle->fen);
    for (int i = 0; i < setup_moves; i++) {
        chess_apply_uci_move(move_list[i]);
    }
    replay_push_frame(frames, &frame_count, "Start");

    char side = chess_fen_side(puzzle->fen);
    char player_side = side;
    for (int i = 0; i < setup_moves; i++) {
        player_side = (player_side == 'w') ? 'b' : 'w';
    }
    int flipped = (player_side == 'b');

    stats->total_attempted++;
    status[0] = '\0';

    for (int i = expected_idx; i < nmoves; i += 2) {
        const char *expected = move_list[i];
        char input[REPLAY_INPUT_LEN];

        for (;;) {
            int read_status = cli_read_replayable_input(puzzle, player_side,
                                                        flipped,
                                                        frames, frame_count - 1, status,
                                                        input, sizeof(input));
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
                snprintf(status, sizeof(status), "Hint: the piece starts on %c%c",
                         expected[0], expected[1]);
                chess_set_board_snapshot(frames[frame_count - 1].board);
                printf("  %sPress Enter to try again...%s", FG_DIM, RST);
                fflush(stdout);
                if (!fgets(input, sizeof(input), stdin)) {
                    return 0;
                }
                continue;
            }

            chess_set_board_snapshot(frames[frame_count - 1].board);
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
                    replay_push_frame(frames, &frame_count, expected);
                    chess_print_board(flipped);

                    stats->current_streak = 0;
                    printf("  %s🔥 Streak: %d | Best: %d | Solved: %d/%d%s\n\n",
                           FG_DIM, stats->current_streak, stats->best_streak,
                           stats_count_solved(stats, total_puzzles), total_puzzles,
                           RST);
                    cli_show_replay_summary(puzzle, is_potd, flipped, frames, frame_count,
                                            "\342\234\227 Solution shown",
                                            "Use replay to review the line.");
                    return -1;
                }
                snprintf(status, sizeof(status), "Try another move.");
                continue;
            }

            chess_apply_uci_move(expected);
            replay_push_frame(frames, &frame_count, expected);
            snprintf(status, sizeof(status), "Played %s", expected);
            cli_render_puzzle_view(puzzle, player_side, flipped,
                                   frames, frame_count - 1, frame_count - 1,
                                   "", status, 0);
            terminal_sleep_ms(PLAYER_MOVE_DELAY_MS);
            break;
        }

        if (i + 1 < nmoves) {
            chess_apply_uci_move(move_list[i + 1]);
            replay_push_frame(frames, &frame_count, move_list[i + 1]);
            snprintf(status, sizeof(status), "Reply %s", move_list[i + 1]);
            cli_render_puzzle_view(puzzle, player_side, flipped,
                                   frames, frame_count - 1, frame_count - 1,
                                   "", status, 0);
            terminal_sleep_ms(OPPONENT_MOVE_DELAY_MS);
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

    cli_show_replay_summary(puzzle, is_potd, flipped, frames, frame_count,
                            "\342\234\223 Puzzle solved",
                            "Use replay to scroll through the full sequence.");

    return 1;
}

/* ---- Live game --------------------------------------------------------- */

static int live_move_count(const char *moves) {
    if (!moves || !moves[0]) return 0;
    int n = 1;
    for (const char *p = moves; *p; p++)
        if (*p == ' ') n++;
    return n;
}

static int live_is_my_turn(const LichessGame *game) {
    int n = game->moves[0] ? live_move_count(game->moves) : 0;
    int my_white = strcmp(game->my_color, "white") == 0;
    return my_white ? (n % 2 == 0) : (n % 2 == 1);
}

static void live_apply_moves(const char *moves) {
    if (!moves || !moves[0]) return;
    char buf[LICHESS_MOVES_MAX];
    strncpy(buf, moves, sizeof(buf) - 1);
    buf[sizeof(buf) - 1] = '\0';
    char *tok = strtok(buf, " ");
    while (tok) {
        chess_apply_uci_move(tok);
        tok = strtok(NULL, " ");
    }
}

static void live_render(const LichessGame *game, int flipped,
                        const char *move_input, const char *status, int my_turn) {
    cli_clear_screen();

    int n         = game->moves[0] ? live_move_count(game->moves) : 0;
    int white_turn = (n % 2 == 0);
    int wtime_s   = game->wtime / 1000;
    int btime_s   = game->btime / 1000;

    const char *top_name  = flipped ? (game->white_id[0] ? game->white_id : "opponent")
                                    : (game->black_id[0] ? game->black_id : "opponent");
    const char *bot_name  = flipped ? (game->black_id[0] ? game->black_id : "you")
                                    : (game->white_id[0] ? game->white_id : "you");
    int top_secs  = flipped ? wtime_s : btime_s;
    int bot_secs  = flipped ? btime_s : wtime_s;

    printf("\n  %s%s%s  %s%02d:%02d%s\n",
           FG_DIM, top_name, RST,
           FG_DIM, top_secs / 60, top_secs % 60, RST);
    printf("  %s%s to move%s\n",
           FG_HINT, white_turn ? "White" : "Black", RST);

    chess_print_board(flipped);

    printf("  %s%s%s  %s%02d:%02d%s\n",
           BOLD, bot_name, RST,
           BOLD, bot_secs / 60, bot_secs % 60, RST);

    if (status && status[0])
        printf("  %s%s%s\n", FG_DIM, status, RST);

    if (my_turn)
        printf("  Move: %s", move_input);
    else
        printf("  %sWaiting for opponent...%s", FG_DIM, RST);

    fflush(stdout);
}

typedef struct { LichessGame *game; int updated; } LiveCtx;

static void live_line_cb(const char *line, void *ud) {
    LiveCtx *ctx = (LiveCtx *)ud;
    if (lichess_parse_game_event(line, ctx->game))
        ctx->updated = 1;
}

int cli_play_live(const char *token, LichessGame *game) {
    char stream_url[256];
    snprintf(stream_url, sizeof(stream_url),
             "https://lichess.org/api/board/game/stream/%s", game->id);

    HttpStream *s = http_stream_open(stream_url, token);
    if (!s) return 0;

    int  flipped    = strcmp(game->my_color, "black") == 0;
    char move_input[16] = {0};
    int  move_len   = 0;
    char status[128] = {0};
    int  needs_redraw = 1;

    chess_set_position("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1");

    terminal_enable_raw_mode();

    for (;;) {
        LiveCtx ctx = { game, 0 };
        int stdin_ready = 0;
        int active = http_stream_tick(s, 100, live_line_cb, &ctx, &stdin_ready);

        if (ctx.updated) {
            chess_set_position(
                "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1");
            live_apply_moves(game->moves);
            needs_redraw = 1;
        }

        /* Check game over */
        if (game->status[0] && strcmp(game->status, "started") != 0)
            break;

        /* Handle keyboard */
        if (stdin_ready) {
            char c = 0;
            if (read(STDIN_FILENO, &c, 1) == 1) {
                needs_redraw = 1;
                if (c == '\r' || c == '\n') {
                    if (move_len > 0 && live_is_my_turn(game)) {
                        move_input[move_len] = '\0';
                        if (lichess_send_move(token, game->id, move_input)) {
                            snprintf(status, sizeof(status), "Sent: %s", move_input);
                        } else {
                            snprintf(status, sizeof(status),
                                     "Illegal move: %s", move_input);
                        }
                        move_len = 0;
                        move_input[0] = '\0';
                    }
                } else if (c == 127 || c == 8) {
                    if (move_len > 0) move_input[--move_len] = '\0';
                } else if ((c == 'q' || c == 'Q') && move_len == 0) {
                    lichess_resign(token, game->id);
                    snprintf(status, sizeof(status), "You resigned.");
                    break;
                } else if (c >= 32 && c < 127 &&
                           move_len < (int)sizeof(move_input) - 1) {
                    move_input[move_len++] = c;
                    move_input[move_len]   = '\0';
                }
            }
        }

        if (needs_redraw) {
            live_render(game, flipped, move_input, status, live_is_my_turn(game));
            needs_redraw = 0;
        }

        if (!active) break;
    }

    terminal_disable_raw_mode();
    http_stream_close(s);

    /* Final result screen */
    cli_clear_screen();
    printf("\n  %sGame over%s\n\n", BOLD, RST);
    chess_print_board(flipped);

    if (game->winner[0]) {
        int i_won = strcmp(game->winner, game->my_color) == 0;
        printf("  %s%s%s  (%s)\n\n",
               i_won ? FG_OK : FG_FAIL,
               i_won ? "You won!" : "You lost.",
               RST, game->status);
    } else if (game->status[0]) {
        printf("  %sDraw%s  (%s)\n\n", FG_DIM, RST, game->status);
    }

    cli_wait_for_enter();
    return 1;
}
