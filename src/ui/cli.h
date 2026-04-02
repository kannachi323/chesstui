#pragma once

#include <stddef.h>
#include "common.h"

void cli_clear_screen(void);
void cli_wait_for_enter(void);
void cli_format_next_potd_reset(char *buf, size_t bufsz);
void cli_print_menu(const Stats *stats, int solved_count, int total_puzzles);
void cli_print_stats(const Stats *stats, int solved_count, int total_puzzles);
int cli_play_puzzle(int puzzle_idx, const Puzzle *puzzle, Stats *stats, int is_potd, int total_puzzles);
