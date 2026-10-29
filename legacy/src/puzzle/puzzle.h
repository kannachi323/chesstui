#pragma once

#include "common.h"

#define PUZZLE_CSV_MAX 32
#define PUZZLE_PATH_MAX 512

typedef struct {
    int year;
    int day_of_year;
} PuzzleDay;

int puzzle_list_csvs(char out[][PUZZLE_PATH_MAX], int max);
int puzzle_store_load(PuzzleStore *store, const char *filepath);
void puzzle_store_shuffle_random(PuzzleStore *store);
int puzzle_store_next_random(PuzzleStore *store);
PuzzleDay puzzle_current_day(void);
int puzzle_stats_has_solved_day(const Stats *stats, PuzzleDay day);
void puzzle_stats_mark_day_solved(Stats *stats, PuzzleDay day);
