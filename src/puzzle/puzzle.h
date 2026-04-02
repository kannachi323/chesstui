#pragma once

#include "common.h"

typedef struct {
    int year;
    int day_of_year;
} PuzzleDay;

const char *puzzle_find_csv_path(const char *argv0, const char *arg);
int puzzle_store_load(PuzzleStore *store, const char *filepath);
void puzzle_store_shuffle_random(PuzzleStore *store);
int puzzle_store_next_random(PuzzleStore *store);
int puzzle_store_potd_index(const PuzzleStore *store, int year, int day_of_year);
PuzzleDay puzzle_current_day(void);
int puzzle_store_current_potd_index(const PuzzleStore *store, PuzzleDay day);
int puzzle_stats_has_solved_day(const Stats *stats, PuzzleDay day);
void puzzle_stats_mark_day_solved(Stats *stats, PuzzleDay day);
