#include <stdlib.h>
#include <stdio.h>
#include <string.h>

#include "json.h"
#include "http_client.h"
#include "lichess.h"

#define LICHESS_DAILY_URL  "https://lichess.org/api/puzzle/daily"
#define LICHESS_SEEK_URL   "https://lichess.org/api/challenge/open"

const TimeControl lichess_time_controls[LICHESS_TC_COUNT] = {
    { "Bullet    1+0",  "bullet",     60,  0 },
    { "Bullet    2+1",  "bullet",    120,  1 },
    { "Blitz     3+0",  "blitz",     180,  0 },
    { "Blitz     5+0",  "blitz",     300,  0 },
    { "Rapid    10+0",  "rapid",     600,  0 },
    { "Classical 30+0", "classical", 1800, 0 },
};

static struct json_value_s *obj_get(struct json_object_s *obj, const char *key) {
    for (struct json_object_element_s *e = obj->start; e; e = e->next) {
        if (strcmp(e->name->string, key) == 0) return e->value;
    }
    return NULL;    
}

static void join_string_array(struct json_array_s *arr, char *buf, size_t bufsz) {
    size_t pos = 0;
    for (struct json_array_element_s *e = arr->start; e; e = e->next) {
        struct json_string_s *s = json_value_as_string(e->value);
        if (!s) continue;
        if (pos > 0 && pos < bufsz - 1) buf[pos++] = ' ';
        size_t copy = s->string_size;
        if (pos + copy >= bufsz) copy = bufsz - pos - 1;
        memcpy(buf + pos, s->string, copy);
        pos += copy;
    }
    buf[pos] = '\0';
}

int lichess_fetch_daily_puzzle(Puzzle *out) {
    HttpResponse resp = http_get(LICHESS_DAILY_URL);
    if (resp.status_code != 200 || !resp.body) {
        http_response_free(&resp);
        return 0;
    }

    struct json_value_s *root_val = json_parse(resp.body, resp.body_len);
    http_response_free(&resp);
    if (!root_val) return 0;

    int ok = 0;

    struct json_object_s *root = json_value_as_object(root_val);
    if (!root) goto done;

    struct json_value_s *pval = obj_get(root, "puzzle");
    if (!pval) goto done;
    struct json_object_s *puzzle = json_value_as_object(pval);
    if (!puzzle) goto done;

    struct json_string_s *id  = json_value_as_string(obj_get(puzzle, "id"));
    struct json_string_s *fen = json_value_as_string(obj_get(puzzle, "fen"));
    struct json_number_s *rat = json_value_as_number(obj_get(puzzle, "rating"));
    struct json_array_s  *sol = json_value_as_array (obj_get(puzzle, "solution"));
    struct json_array_s  *thm = json_value_as_array (obj_get(puzzle, "themes"));

    if (!id || !fen || !sol) goto done;

    snprintf(out->id,  sizeof(out->id),  "%.*s", (int)id->string_size,  id->string);
    snprintf(out->fen, sizeof(out->fen), "%.*s", (int)fen->string_size, fen->string);
    out->rating = rat ? atoi(rat->number) : 0;

    /* The API's fen is already the position after lastMove (player to move).
     * solution[0] is the player's first move — no setup move needed. */
    join_string_array(sol, out->moves, sizeof(out->moves));

    if (thm) join_string_array(thm, out->themes, sizeof(out->themes));

    ok = 1;
done:
    free(root_val);
    return ok;
}

int lichess_create_seek(const char *token, const TimeControl *tc, LichessSeek *out) {
    char body[128];
    snprintf(body, sizeof(body),
             "clock.limit=%d&clock.increment=%d&rated=true",
             tc->limit, tc->increment);

    HttpResponse resp = http_post_bearer(LICHESS_SEEK_URL, token, body);
    if (resp.status_code != 200 || !resp.body) {
        http_response_free(&resp);
        return 0;
    }

    struct json_value_s *root_val = json_parse(resp.body, resp.body_len);
    http_response_free(&resp);
    if (!root_val) return 0;

    int ok = 0;
    struct json_object_s *root = json_value_as_object(root_val);
    if (!root) goto done;

    struct json_value_s  *challenge_val = obj_get(root, "challenge");
    if (!challenge_val) goto done;
    struct json_object_s *challenge = json_value_as_object(challenge_val);
    if (!challenge) goto done;

    struct json_value_s *id_val    = obj_get(challenge, "id");
    struct json_value_s *url_val   = obj_get(challenge, "url");
    struct json_value_s *color_val = obj_get(challenge, "color");
    if (!id_val || !url_val) goto done;

    struct json_string_s *id    = json_value_as_string(id_val);
    struct json_string_s *url   = json_value_as_string(url_val);
    struct json_string_s *color = color_val ? json_value_as_string(color_val) : NULL;

    if (!id || !url) goto done;

    snprintf(out->id,    sizeof(out->id),    "%.*s", (int)id->string_size,    id->string);
    snprintf(out->url,   sizeof(out->url),   "%.*s", (int)url->string_size,   url->string);
    if (color)
        snprintf(out->color, sizeof(out->color), "%.*s", (int)color->string_size, color->string);

    ok = 1;
done:
    free(root_val);
    return ok;
}

/* ---- Live game streaming ----------------------------------------------- */

#define LICHESS_EVENTS_URL  "https://lichess.org/api/stream/event"
#define LICHESS_GAME_STREAM "https://lichess.org/api/board/game/stream/%s"
#define LICHESS_MOVE_URL    "https://lichess.org/api/board/game/%s/move/%s"
#define LICHESS_RESIGN_URL  "https://lichess.org/api/board/game/%s/resign"

typedef struct { LichessGame *game; int found; } WaitCtx;

static void wait_event_cb(const char *line, void *ud) {
    WaitCtx *ctx = (WaitCtx *)ud;
    if (ctx->found) return;

    struct json_value_s *root_val = json_parse(line, strlen(line));
    if (!root_val) return;

    struct json_object_s *root = json_value_as_object(root_val);
    if (!root) goto done;

    struct json_value_s  *type_val = obj_get(root, "type");
    struct json_string_s *type     = type_val ? json_value_as_string(type_val) : NULL;
    if (!type || strcmp(type->string, "gameStart") != 0) goto done;

    struct json_value_s  *game_val = obj_get(root, "game");
    struct json_object_s *game_obj = game_val ? json_value_as_object(game_val) : NULL;
    if (!game_obj) goto done;

    struct json_value_s  *id_val    = obj_get(game_obj, "id");
    struct json_value_s  *color_val = obj_get(game_obj, "color");
    struct json_string_s *id        = id_val    ? json_value_as_string(id_val)    : NULL;
    struct json_string_s *clr       = color_val ? json_value_as_string(color_val) : NULL;
    if (!id) goto done;

    snprintf(ctx->game->id, sizeof(ctx->game->id), "%.*s",
             (int)id->string_size, id->string);
    if (clr)
        snprintf(ctx->game->my_color, sizeof(ctx->game->my_color), "%.*s",
                 (int)clr->string_size, clr->string);
    ctx->found = 1;

done:
    free(root_val);
}

int lichess_wait_for_game(const char *token, LichessGame *game, int timeout_ms) {
    HttpStream *s = http_stream_open(LICHESS_EVENTS_URL, token);
    if (!s) return 0;

    WaitCtx ctx = { game, 0 };
    int elapsed = 0;

    while (!ctx.found && elapsed < timeout_ms) {
        int still = http_stream_tick(s, 200, wait_event_cb, &ctx, NULL);
        elapsed += 200;
        if (!still) break;
    }

    http_stream_close(s);
    return ctx.found;
}

static void parse_game_state(struct json_object_s *state, LichessGame *game) {
    struct json_value_s *moves_v  = obj_get(state, "moves");
    struct json_value_s *wtime_v  = obj_get(state, "wtime");
    struct json_value_s *btime_v  = obj_get(state, "btime");
    struct json_value_s *winc_v   = obj_get(state, "winc");
    struct json_value_s *binc_v   = obj_get(state, "binc");
    struct json_value_s *status_v = obj_get(state, "status");
    struct json_value_s *winner_v = obj_get(state, "winner");

    struct json_string_s *moves  = moves_v  ? json_value_as_string(moves_v)  : NULL;
    struct json_number_s *wtime  = wtime_v  ? json_value_as_number(wtime_v)  : NULL;
    struct json_number_s *btime  = btime_v  ? json_value_as_number(btime_v)  : NULL;
    struct json_number_s *winc   = winc_v   ? json_value_as_number(winc_v)   : NULL;
    struct json_number_s *binc   = binc_v   ? json_value_as_number(binc_v)   : NULL;
    struct json_string_s *status = status_v ? json_value_as_string(status_v) : NULL;
    struct json_string_s *winner = winner_v ? json_value_as_string(winner_v) : NULL;

    if (moves)  snprintf(game->moves,  sizeof(game->moves),  "%.*s",
                         (int)moves->string_size,  moves->string);
    if (wtime)  game->wtime  = atoi(wtime->number);
    if (btime)  game->btime  = atoi(btime->number);
    if (winc)   game->winc   = atoi(winc->number);
    if (binc)   game->binc   = atoi(binc->number);
    if (status) snprintf(game->status, sizeof(game->status), "%.*s",
                         (int)status->string_size, status->string);
    if (winner) snprintf(game->winner, sizeof(game->winner), "%.*s",
                         (int)winner->string_size, winner->string);
}

int lichess_parse_game_event(const char *line, LichessGame *game) {
    if (!line || !line[0]) return 0;

    struct json_value_s *root_val = json_parse(line, strlen(line));
    if (!root_val) return 0;

    int ok = 0;
    struct json_object_s *root = json_value_as_object(root_val);
    if (!root) goto done;

    struct json_value_s  *type_val = obj_get(root, "type");
    struct json_string_s *type     = type_val ? json_value_as_string(type_val) : NULL;
    if (!type) goto done;

    if (strcmp(type->string, "gameFull") == 0) {
        struct json_value_s  *white_v   = obj_get(root, "white");
        struct json_value_s  *black_v   = obj_get(root, "black");
        struct json_object_s *white_obj = white_v ? json_value_as_object(white_v) : NULL;
        struct json_object_s *black_obj = black_v ? json_value_as_object(black_v) : NULL;
        if (white_obj) {
            struct json_value_s  *wid_v = obj_get(white_obj, "id");
            struct json_string_s *wid   = wid_v ? json_value_as_string(wid_v) : NULL;
            if (wid) snprintf(game->white_id, sizeof(game->white_id), "%.*s",
                              (int)wid->string_size, wid->string);
        }
        if (black_obj) {
            struct json_value_s  *bid_v = obj_get(black_obj, "id");
            struct json_string_s *bid   = bid_v ? json_value_as_string(bid_v) : NULL;
            if (bid) snprintf(game->black_id, sizeof(game->black_id), "%.*s",
                              (int)bid->string_size, bid->string);
        }
        struct json_value_s  *state_v   = obj_get(root, "state");
        struct json_object_s *state_obj = state_v ? json_value_as_object(state_v) : NULL;
        if (state_obj) parse_game_state(state_obj, game);
        ok = 1;

    } else if (strcmp(type->string, "gameState") == 0) {
        parse_game_state(root, game);
        ok = 1;
    }

done:
    free(root_val);
    return ok;
}

int lichess_send_move(const char *token, const char *game_id, const char *uci) {
    char url[256];
    snprintf(url, sizeof(url), LICHESS_MOVE_URL, game_id, uci);
    HttpResponse resp = http_post_bearer(url, token, "");
    int ok = (resp.status_code == 200);
    http_response_free(&resp);
    return ok;
}

int lichess_resign(const char *token, const char *game_id) {
    char url[256];
    snprintf(url, sizeof(url), LICHESS_RESIGN_URL, game_id);
    HttpResponse resp = http_post_bearer(url, token, "");
    int ok = (resp.status_code == 200);
    http_response_free(&resp);
    return ok;
}
