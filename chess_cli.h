#ifndef CHESS_CLI_H
#define CHESS_CLI_H

#include <stddef.h>

#define MAX_LINE 2048
#define MAX_PUZZLES 1000000
#define FEN_MAX 128
#define MOVES_MAX 256
#define MOVE_LEN 8
#define MOVE_LIST_MAX 32
#define STATS_FILE ".chess-cli-stats"
#define SQ_W 3
#define SQ_H 1

#define RST "\033[0m"
#define BOLD "\033[1m"
#define DIM "\033[2m"

#define BG_LIGHT "\033[48;2;110;140;80m"
#define BG_DARK "\033[48;2;70;95;55m"
#define FG_WHITE "\033[38;2;255;255;255m"
#define FG_BLACK "\033[38;2;30;30;30m"
#define FG_HINT "\033[38;5;208m"
#define FG_OK "\033[38;5;82m"
#define FG_FAIL "\033[38;5;196m"
#define FG_DIM "\033[38;5;245m"
#define FG_TITLE "\033[38;5;75m"

typedef struct {
    char id[16];
    char fen[FEN_MAX];
    char moves[MOVES_MAX];
    int rating;
    char themes[128];
} Puzzle;

typedef struct {
    int current_streak;
    int best_streak;
    int total_solved;
    int total_attempted;
    int last_day;
    int last_year;
    unsigned char solved[MAX_PUZZLES];
} Stats;

typedef struct {
    Puzzle puzzles[MAX_PUZZLES];
    int num_puzzles;
    int random_order[MAX_PUZZLES];
    int random_order_pos;
} PuzzleStore;

#endif
