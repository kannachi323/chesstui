#include <stdio.h>
#include <stdlib.h>

#include "chess_stats.h"

static const char* stats_path(void) {
    static char path[512];
    const char *home = getenv("HOME");

    if (home) {
        snprintf(path, sizeof(path), "%s/%s", home, STATS_FILE);
    } else {
        snprintf(path, sizeof(path), "%s", STATS_FILE);
    }

    return path;
}

Stats stats_load(void) {
    Stats stats = {0};
    FILE *f = fopen(stats_path(), "r");

    if (f) {
        char line[MAX_LINE];

        if (fgets(line, sizeof(line), f)) {
            sscanf(line, "%d %d %d %d %d %d",
                   &stats.current_streak, &stats.best_streak,
                   &stats.total_solved, &stats.total_attempted,
                   &stats.last_day, &stats.last_year);
        }

        if (fgets(line, sizeof(line), f)) {
            for (int i = 0; i < MAX_PUZZLES && line[i]; i++) {
                if (line[i] == '0' || line[i] == '1') {
                    stats.solved[i] = (unsigned char)(line[i] - '0');
                }
            }
        }

        fclose(f);
    }

    return stats;
}

void stats_save(const Stats *stats) {
    FILE *f = fopen(stats_path(), "w");
    if (!f) {
        return;
    }

    fprintf(f, "%d %d %d %d %d %d\n",
            stats->current_streak, stats->best_streak,
            stats->total_solved, stats->total_attempted,
            stats->last_day, stats->last_year);

    for (int i = 0; i < MAX_PUZZLES; i++) {
        fputc(stats->solved[i] ? '1' : '0', f);
    }
    fputc('\n', f);

    fclose(f);
}

int stats_count_solved(const Stats *stats, int puzzle_count) {
    int count = 0;

    for (int i = 0; i < puzzle_count; i++) {
        count += stats->solved[i] ? 1 : 0;
    }

    return count;
}
