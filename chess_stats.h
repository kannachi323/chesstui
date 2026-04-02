#pragma once

#include "common.h"

Stats stats_load(void);
void stats_save(const Stats *stats);
int stats_count_solved(const Stats *stats, int puzzle_count);

