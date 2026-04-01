#ifndef CHESS_STATS_H
#define CHESS_STATS_H

#include "chess_cli.h"

Stats stats_load(void);
void stats_save(const Stats *stats);
int stats_count_solved(const Stats *stats, int puzzle_count);

#endif
