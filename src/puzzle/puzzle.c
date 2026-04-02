#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <unistd.h>

#include "puzzle.h"
#include "util.h"

const char *puzzle_find_csv_path(const char *argv0, const char *arg) {
    static char path[512];

    if (arg) {
        snprintf(path, sizeof(path), "%s", arg);
        return path;
    }

    const char *slash = strrchr(argv0, '/');
    if (slash) {
        int dirlen = (int)(slash - argv0);
        snprintf(path, sizeof(path), "%.*s/puzzles.csv", dirlen, argv0);
    } else {
        snprintf(path, sizeof(path), "puzzles.csv");
    }

    if (access(path, R_OK) == 0) {
        return path;
    }

    snprintf(path, sizeof(path), "puzzles.csv");
    return path;
}

int puzzle_store_load(PuzzleStore *store, const char *filepath) {
    FILE *f = fopen(filepath, "r");
    if (!f) {
        fprintf(stderr, "Cannot open %s\n", filepath);
        return -1;
    }

    char line[MAX_LINE];
    if (!fgets(line, sizeof(line), f)) {
        fclose(f);
        return -1;
    }

    store->num_puzzles = 0;
    while (fgets(line, sizeof(line), f) && store->num_puzzles < MAX_PUZZLES) {
        Puzzle *p = &store->puzzles[store->num_puzzles];
        char *fields[10] = {0};
        int nfields = 0;
        int in_quotes = 0;
        char *field_start = line;

        for (char *cur = line; *cur; cur++) {
            if (*cur == '"') {
                in_quotes = !in_quotes;
            } else if (*cur == ',' && !in_quotes && nfields < 9) {
                *cur = '\0';
                fields[nfields++] = field_start;
                field_start = cur + 1;
            }
        }
        fields[nfields++] = field_start;

        if (nfields < 5) {
            continue;
        }

        /* Lichess CSV: id,fen,moves,rating,...,themes(7),... */
        char *themes_field = (nfields >= 8) ? fields[7] : fields[4];

        for (int fi = 0; fi < nfields; fi++) {
            if (!fields[fi]) fields[fi] = "";
        }

        strncpy(p->id, trim(fields[0]), sizeof(p->id) - 1);
        p->id[sizeof(p->id) - 1] = '\0';
        strncpy(p->fen, trim(fields[1]), sizeof(p->fen) - 1);
        p->fen[sizeof(p->fen) - 1] = '\0';
        strncpy(p->moves, trim(fields[2]), sizeof(p->moves) - 1);
        p->moves[sizeof(p->moves) - 1] = '\0';
        p->rating = atoi(trim(fields[3]));
        strncpy(p->themes, trim(themes_field), sizeof(p->themes) - 1);
        p->themes[sizeof(p->themes) - 1] = '\0';

        store->num_puzzles++;
    }

    fclose(f);
    return store->num_puzzles;
}

void puzzle_store_shuffle_random(PuzzleStore *store) {
    for (int i = 0; i < store->num_puzzles; i++) {
        store->random_order[i] = i;
    }

    for (int i = store->num_puzzles - 1; i > 0; i--) {
        int j = rand() % (i + 1);
        int tmp = store->random_order[i];
        store->random_order[i] = store->random_order[j];
        store->random_order[j] = tmp;
    }

    store->random_order_pos = 0;
}

int puzzle_store_next_random(PuzzleStore *store) {
    if (store->num_puzzles <= 0) {
        return -1;
    }

    if (store->random_order_pos >= store->num_puzzles) {
        puzzle_store_shuffle_random(store);
    }

    return store->random_order[store->random_order_pos++];
}

int puzzle_store_potd_index(const PuzzleStore *store, int year, int day_of_year) {
    if (store->num_puzzles <= 0) {
        return -1;
    }

    return (year * 366 + day_of_year) % store->num_puzzles;
}

PuzzleDay puzzle_current_day(void) {
    time_t now = time(NULL);
    struct tm *local_now;
    PuzzleDay day = {0};

    local_now = localtime(&now);
    if (local_now == NULL) {
        return day;
    }

    day.year = local_now->tm_year + 1900;
    day.day_of_year = local_now->tm_yday;
    return day;
}

int puzzle_store_current_potd_index(const PuzzleStore *store, PuzzleDay day) {
    return puzzle_store_potd_index(store, day.year, day.day_of_year);
}

int puzzle_stats_has_solved_day(const Stats *stats, PuzzleDay day) {
    return stats->last_day == day.day_of_year && stats->last_year == day.year;
}

void puzzle_stats_mark_day_solved(Stats *stats, PuzzleDay day) {
    stats->last_day = day.day_of_year;
    stats->last_year = day.year;
}
