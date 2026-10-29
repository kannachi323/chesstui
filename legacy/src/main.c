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
#include <locale.h>

#include "account.h"
#include "chess_stats.h"
#include "cli.h"
#include "lichess.h"
#include "puzzle.h"
#include "terminal.h"
#include "util.h"

static void handle_potd(Stats *stats, int total_puzzles) {
    PuzzleDay day = puzzle_current_day();

    if (puzzle_stats_has_solved_day(stats, day)) {
        char reset_at[64];
        cli_format_next_potd_reset(reset_at, sizeof(reset_at));
        cli_clear_screen();
        printf("\n  %sPOTD already solved, come back tomorrow!%s\n", FG_DIM, RST);
        printf("  %sResets at %s%s\n\n", FG_DIM, reset_at, RST);
        cli_wait_for_enter();
        return;
    }

    Puzzle live = {0};
    if (!lichess_fetch_daily_puzzle(&live)) {
        cli_clear_screen();
        printf("\n  %sUnable to fetch Puzzle of the Day.%s\n\n", FG_DIM, RST);
        cli_wait_for_enter();
        return;
    }

    int result = cli_play_puzzle(-1, &live, stats, 1, total_puzzles, 0);
    if (result == 1) {
        puzzle_stats_mark_day_solved(stats, day);
    }
    if (result != 0) {
        cli_wait_for_enter();
    }
    stats_save(stats);
}

static void handle_puzzles(PuzzleStore *store, Stats *stats) {
    if (store->num_puzzles == 0) {
        const char *csv_path = cli_select_csv();
        if (!csv_path) {
            return;
        }
        if (puzzle_store_load(store, csv_path) <= 0) {
            cli_clear_screen();
            printf("\n  %sFailed to load puzzles from %s%s\n\n", FG_FAIL, csv_path, RST);
            cli_wait_for_enter();
            return;
        }
        puzzle_store_shuffle_random(store);
    }

    char input[64];
    for (;;) {
        int idx = puzzle_store_next_random(store);
        if (idx < 0) {
            break;
        }

        int result = cli_play_puzzle(idx, &store->puzzles[idx], stats,
                                     0, store->num_puzzles, 1);
        stats_save(stats);
        if (result == 0) {
            break;
        }

        printf("  %sAnother? [y/n]:%s ", BOLD, RST);
        fflush(stdout);
        if (!fgets(input, sizeof(input), stdin)) {
            break;
        }

        char *yn = trim(input);
        if (yn[0] != 'y' && yn[0] != 'Y') {
            break;
        }
    }
}

int main(void) {
    setlocale(LC_CTYPE, "");
    terminal_setup();
    atexit(terminal_restore);

    static PuzzleStore store = {0};
    srand((unsigned)time(NULL));
    Stats stats = stats_load();

    LichessAccount account = {0};
    int account_fetched = 0;
    AccountTab account_tab = ACCOUNT_TAB_PUZZLES;
    account_load(&account);

    int play_selected = 0;
    AppScreen screen = SCREEN_HOME;
    terminal_enable_raw_mode();

    for (;;) {
        int solved_count = stats_count_solved(&stats, store.num_puzzles);
        cli_clear_screen();

        if (screen == SCREEN_ACCOUNT && account.username[0] && !account_fetched) {
            terminal_disable_raw_mode();
            printf("\n  %sFetching account data...%s\n", FG_DIM, RST);
            fflush(stdout);
            account_fetch(&account);
            account_fetched = 1;
            terminal_enable_raw_mode();
            cli_clear_screen();
        }

        if (screen == SCREEN_HOME) {
            cli_print_menu(&stats, solved_count, store.num_puzzles, screen);
        } else if (screen == SCREEN_STATS) {
            cli_print_stats(&stats, solved_count, store.num_puzzles, screen);
        } else if (screen == SCREEN_ACCOUNT) {
            cli_print_account(&account, screen, account_tab);
        } else {
            cli_print_play(screen, play_selected, account.username[0] != '\0');
        }

        int key = terminal_read_key();
        if (key == TERMINAL_KEY_UNKNOWN) {
            continue;
        }

        if (key == TERMINAL_KEY_LEFT) {
            if ((int)screen > 0) screen--;
            continue;
        }
        if (key == TERMINAL_KEY_RIGHT) {
            if ((int)screen < SCREEN_COUNT - 1) screen++;
            continue;
        }
        if (key == TERMINAL_KEY_UP) {
            if (screen == SCREEN_ACCOUNT && (int)account_tab > 0)
                account_tab--;
            else if (screen == SCREEN_PLAY && play_selected > 0)
                play_selected--;
            continue;
        }
        if (key == TERMINAL_KEY_DOWN) {
            if (screen == SCREEN_ACCOUNT && (int)account_tab < ACCOUNT_TAB_COUNT - 1)
                account_tab++;
            else if (screen == SCREEN_PLAY && play_selected < LICHESS_TC_COUNT - 1)
                play_selected++;
            continue;
        }

        if (key == 'q' || key == 'Q') {
            break;
        }

        if (screen == SCREEN_HOME) {
            if (key == '1') {
                terminal_disable_raw_mode();
                handle_potd(&stats, store.num_puzzles);
                terminal_enable_raw_mode();
            } else if (key == '2') {
                terminal_disable_raw_mode();
                handle_puzzles(&store, &stats);
                terminal_enable_raw_mode();
            }
        } else if (screen == SCREEN_PLAY) {
            if (key == TERMINAL_KEY_ENTER && account.username[0]) {
                terminal_disable_raw_mode();
                cli_clear_screen();
                printf("\n  %sCreating challenge...%s\n\n", FG_DIM, RST);
                fflush(stdout);

                LichessSeek seek = {0};
                const TimeControl *tc = &lichess_time_controls[play_selected];
                if (lichess_create_seek(account.token, tc, &seek)) {
                    printf("  %s✓ Challenge created!%s\n\n", FG_OK, RST);
                    printf("    %s%s%s\n\n", BOLD, seek.url, RST);
                    printf("  %sShare the link or open it to play.%s\n", FG_DIM, RST);
                    printf("  %sWaiting for opponent (q to cancel)...%s\n\n",
                           FG_DIM, RST);
                    fflush(stdout);

                    LichessGame live_game = {0};
                    if (lichess_wait_for_game(account.token, &live_game, 600000)) {
                        cli_play_live(account.token, &live_game);
                    } else {
                        printf("  %sTimed out waiting for opponent.%s\n\n",
                               FG_FAIL, RST);
                        cli_wait_for_enter();
                    }
                } else {
                    printf("  %sFailed to create challenge.%s\n\n", FG_FAIL, RST);
                    cli_wait_for_enter();
                }
                terminal_enable_raw_mode();
            }
        } else if (screen == SCREEN_ACCOUNT) {
            if (key == 'a' || key == 'A') {
                if (!account.username[0]) {
                    terminal_disable_raw_mode();
                    if (cli_add_account(&account)) {
                        account_fetched = 1;
                    }
                    terminal_enable_raw_mode();
                }
            } else if (key == 'x' || key == 'X') {
                if (account.username[0]) {
                    account_remove(&account);
                    memset(&account, 0, sizeof(account));
                    account_fetched = 0;
                }
            }
        }
    }

    terminal_disable_raw_mode();
    printf("\n  %sGoodbye!%s\n\n", FG_DIM, RST);
    return 0;
}
