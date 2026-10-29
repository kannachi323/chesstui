#pragma once

#include <stddef.h>
#include "account.h"
#include "common.h"
#include "lichess.h"

typedef enum {
    SCREEN_HOME    = 0,
    SCREEN_STATS   = 1,
    SCREEN_ACCOUNT = 2,
    SCREEN_PLAY    = 3,
    SCREEN_COUNT   = 4
} AppScreen;

void cli_clear_screen(void);
void cli_wait_for_enter(void);
const char *cli_select_csv(void);
void cli_format_next_potd_reset(char *buf, size_t bufsz);
void cli_print_menu(const Stats *stats, int solved_count, int total_puzzles, AppScreen screen);
void cli_print_stats(const Stats *stats, int solved_count, int total_puzzles, AppScreen screen);
void cli_print_account(const LichessAccount *acc, AppScreen screen, AccountTab tab);
int  cli_add_account(LichessAccount *out);
void cli_print_play(AppScreen screen, int selected_tc, int has_account);
int cli_play_puzzle(int puzzle_idx, const Puzzle *puzzle, Stats *stats, int is_potd, int total_puzzles, int has_setup_move);
int cli_play_live(const char *token, LichessGame *game);
