#pragma once

#define ACCOUNT_TOKEN_MAX    128
#define ACCOUNT_USERNAME_MAX  64

typedef enum {
    ACCOUNT_TAB_PUZZLES = 0,
    ACCOUNT_TAB_BULLET,
    ACCOUNT_TAB_BLITZ,
    ACCOUNT_TAB_RAPID,
    ACCOUNT_TAB_CLASSICAL,
    ACCOUNT_TAB_COUNT
} AccountTab;

typedef struct {
    int rating;
    int games;
} PerfStat;

typedef struct {
    char username[ACCOUNT_USERNAME_MAX];
    char token[ACCOUNT_TOKEN_MAX];
    PerfStat perfs[ACCOUNT_TAB_COUNT];
} LichessAccount;

/* Scan ~/.chess-cli/tokens/ and load the first .token file found.
 * Fills username (from filename) and token (from file contents).
 * Returns 1 on success, 0 if no token file exists. */
int account_load(LichessAccount *out);

/* Write token to ~/.chess-cli/tokens/<username>.token.
 * Creates parent directories as needed.
 * Returns 1 on success. */
int account_save(const char *username, const char *token);

/* Delete ~/.chess-cli/tokens/<username>.token. */
void account_remove(const LichessAccount *acc);

/* Fetch account info from the Lichess API using the stored token.
 * Fills username and all perfs[].
 * Returns 1 on success, 0 on any failure. */
int account_fetch(LichessAccount *acc);
