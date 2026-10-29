#pragma once

#include "common.h"

#define LICHESS_TC_COUNT  6
#define LICHESS_MOVES_MAX 4096

typedef struct {
    const char *label;
    const char *speed;   /* "bullet", "blitz", "rapid", "classical" */
    int         limit;   /* seconds */
    int         increment;
} TimeControl;

typedef struct {
    char id[16];
    char url[128];
    char color[8];       /* "white", "black", "random" */
} LichessSeek;

typedef struct {
    char id[16];
    char my_color[8];              /* "white" or "black" */
    char white_id[64];
    char black_id[64];
    char moves[LICHESS_MOVES_MAX]; /* space-separated UCI, updated per event */
    int  wtime;                    /* ms */
    int  btime;                    /* ms */
    int  winc;                     /* ms */
    int  binc;                     /* ms */
    char status[32];               /* "started", "mate", "resign", "draw", etc. */
    char winner[8];                /* "white", "black", or "" */
} LichessGame;

extern const TimeControl lichess_time_controls[LICHESS_TC_COUNT];

int lichess_fetch_daily_puzzle(Puzzle *out);
int lichess_create_seek(const char *token, const TimeControl *tc, LichessSeek *out);

/* Stream /api/stream/event until gameStart; fills game->id and game->my_color.
 * timeout_ms: total wait time (e.g. 600000 = 10 min). Returns 1 on success. */
int lichess_wait_for_game(const char *token, LichessGame *game, int timeout_ms);

/* Parse one NDJSON line from /api/board/game/stream/{id}.
 * Updates game fields. Returns 1 if the line was recognized. */
int lichess_parse_game_event(const char *line, LichessGame *game);

/* POST a UCI move. Returns 1 on HTTP 200. */
int lichess_send_move(const char *token, const char *game_id, const char *uci);

/* POST resign. Returns 1 on HTTP 200. */
int lichess_resign(const char *token, const char *game_id);
