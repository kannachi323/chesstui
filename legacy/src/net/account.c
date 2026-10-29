#include <dirent.h>
#include <errno.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>

#include "account.h"
#include "http_client.h"
#include "json.h"

#define LICHESS_ACCOUNT_URL "https://lichess.org/api/account"

static struct json_value_s *obj_get(struct json_object_s *obj, const char *key) {
    for (struct json_object_element_s *e = obj->start; e; e = e->next) {
        if (strcmp(e->name->string, key) == 0) return e->value;
    }
    return NULL;
}

static void tokens_dir(char *buf, size_t bufsz) {
    const char *home = getenv("HOME");
    if (home) {
        snprintf(buf, bufsz, "%s/.chess-cli/tokens", home);
    } else {
        buf[0] = '\0';
    }
}

int account_load(LichessAccount *out) {
    char dir[512];
    tokens_dir(dir, sizeof(dir));
    if (!dir[0]) return 0;

    DIR *d = opendir(dir);
    if (!d) return 0;

    char found_name[ACCOUNT_USERNAME_MAX] = {0};
    struct dirent *entry;
    while ((entry = readdir(d)) != NULL) {
        const char *name = entry->d_name;
        size_t len = strlen(name);
        if (len > 6 && strcmp(name + len - 6, ".token") == 0) {
            size_t ulen = len - 6;
            if (ulen >= ACCOUNT_USERNAME_MAX) ulen = ACCOUNT_USERNAME_MAX - 1;
            memcpy(found_name, name, ulen);
            found_name[ulen] = '\0';
            break;
        }
    }
    closedir(d);

    if (!found_name[0]) return 0;

    char path[512];
    snprintf(path, sizeof(path), "%s/%s.token", dir, found_name);

    FILE *f = fopen(path, "r");
    if (!f) return 0;

    char token[ACCOUNT_TOKEN_MAX];
    if (!fgets(token, sizeof(token), f)) {
        fclose(f);
        return 0;
    }
    fclose(f);

    /* Strip trailing whitespace */
    size_t tlen = strlen(token);
    while (tlen > 0 && (token[tlen - 1] == '\n' || token[tlen - 1] == '\r'
                        || token[tlen - 1] == ' ')) {
        token[--tlen] = '\0';
    }

    snprintf(out->username, sizeof(out->username), "%s", found_name);
    snprintf(out->token,    sizeof(out->token),    "%s", token);
    return 1;
}

int account_save(const char *username, const char *token) {
    const char *home = getenv("HOME");
    if (!home) return 0;

    char chesscli[512];
    snprintf(chesscli, sizeof(chesscli), "%s/.chess-cli", home);
    mkdir(chesscli, 0700);

    char dir[512];
    snprintf(dir, sizeof(dir), "%s/.chess-cli/tokens", home);
    mkdir(dir, 0700);

    char path[512];
    snprintf(path, sizeof(path), "%s/%s.token", dir, username);

    FILE *f = fopen(path, "w");
    if (!f) return 0;
    fprintf(f, "%s\n", token);
    fclose(f);
    return 1;
}

void account_remove(const LichessAccount *acc) {
    const char *home = getenv("HOME");
    if (!home || !acc->username[0]) return;

    char path[512];
    snprintf(path, sizeof(path), "%s/.chess-cli/tokens/%s.token",
             home, acc->username);
    remove(path);
}

int account_fetch(LichessAccount *acc) {
    if (!acc->token[0]) return 0;

    HttpResponse resp = http_get_bearer(LICHESS_ACCOUNT_URL, acc->token);
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

    struct json_string_s *uname = json_value_as_string(obj_get(root, "username"));
    if (uname) {
        snprintf(acc->username, sizeof(acc->username), "%.*s",
                 (int)uname->string_size, uname->string);
    }

    static const char *perf_keys[ACCOUNT_TAB_COUNT] = {
        "puzzle", "bullet", "blitz", "rapid", "classical"
    };

    struct json_value_s  *perfs_val = obj_get(root, "perfs");
    struct json_object_s *perfs = perfs_val ? json_value_as_object(perfs_val) : NULL;
    if (perfs) {
        for (int i = 0; i < ACCOUNT_TAB_COUNT; i++) {
            struct json_value_s  *p_val = obj_get(perfs, perf_keys[i]);
            struct json_object_s *p     = p_val ? json_value_as_object(p_val) : NULL;
            if (!p) continue;
            struct json_value_s  *r_val = obj_get(p, "rating");
            struct json_value_s  *g_val = obj_get(p, "games");
            struct json_number_s *rating = r_val ? json_value_as_number(r_val) : NULL;
            struct json_number_s *games  = g_val ? json_value_as_number(g_val) : NULL;
            if (rating) acc->perfs[i].rating = atoi(rating->number);
            if (games)  acc->perfs[i].games  = atoi(games->number);
        }
    }

    ok = 1;
done:
    free(root_val);
    return ok;
}
