/*
 * chess-cli — Terminal Chess Puzzles
 *
 * Build:  gcc -o chess-cli main.c -std=c11
 * Usage:  ./chess-cli [puzzles.csv path]
 *
 * Puzzles: Lichess open database (CC0), bundled in puzzles.csv
 * Format:  PuzzleId,FEN,Moves,Rating,Themes
 *          - FEN is the position BEFORE opponent's move
 *          - Moves: first move is opponent's, rest is the solution
 */

#define _POSIX_C_SOURCE 200809L

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <unistd.h>
#include <ctype.h>
#include <signal.h>

/* ── Constants ─────────────────────────────────────────────── */

#define MAX_LINE    2048
#define MAX_PUZZLES 200
#define FEN_MAX     128
#define MOVES_MAX   256
#define MOVE_LEN    8
#define MOVE_LIST_MAX 32
#define STATS_FILE  ".chess-cli-stats"
#define SQ_W        3
#define SQ_H        1

/* ANSI */
#define RST   "\033[0m"
#define BOLD  "\033[1m"
#define DIM   "\033[2m"

#define BG_LIGHT "\033[48;2;110;140;80m"
#define BG_DARK  "\033[48;2;70;95;55m"
#define FG_WHITE "\033[38;2;255;255;255m"
#define FG_BLACK "\033[38;2;30;30;30m"
#define FG_HINT  "\033[38;5;208m"
#define FG_OK    "\033[38;5;82m"
#define FG_FAIL  "\033[38;5;196m"
#define FG_DIM   "\033[38;5;245m"
#define FG_TITLE "\033[38;5;75m"

/* Preferred fonts for chess Unicode glyphs, in order */
static const char *chess_fonts[] = {
    "JetBrains Mono",
    "JetBrainsMono Nerd Font",
    "Fira Code",
    "Noto Sans Mono",
    "DejaVu Sans Mono",
    NULL
};

/* ── Kitty terminal control ────────────────────────────────── */

static int g_kitty = 0;
static char g_orig_font[256] = {0};
static int g_font_changed = 0;
static int g_alt_screen = 0;

static int read_cmd(const char *cmd, char *buf, int bufsz) {
    FILE *fp = popen(cmd, "r");
    if (!fp) return -1;
    buf[0] = '\0';
    if (fgets(buf, bufsz, fp)) {
        char *nl = strchr(buf, '\n');
        if (nl) *nl = '\0';
    }
    return pclose(fp);
}

static int font_available(const char *name) {
    char cmd[512];
    snprintf(cmd, sizeof(cmd), "fc-list \"%s\" 2>/dev/null | head -1", name);
    char buf[256];
    read_cmd(cmd, buf, sizeof(buf));
    return buf[0] != '\0';
}

static void kitty_restore(void) {
    if (!g_kitty) return;
    if (g_font_changed && g_orig_font[0]) {
        char cmd[512];
        snprintf(cmd, sizeof(cmd),
                 "kitty @ set-font-family \"%s\" 2>/dev/null", g_orig_font);
        system(cmd);
    }
}

static void terminal_restore(void) {
    if (g_alt_screen) {
        printf("\033[?1049l");
        fflush(stdout);
        g_alt_screen = 0;
    }
    kitty_restore();
}

static void signal_handler(int sig) {
    terminal_restore();
    signal(sig, SIG_DFL);
    raise(sig);
}

static void terminal_setup(void) {
    if (!isatty(STDOUT_FILENO))
        return;

    printf("\033[?1049h");
    fflush(stdout);
    g_alt_screen = 1;
}

static void kitty_setup(void) {
    const char *kitty_pid = getenv("KITTY_PID");
    if (!kitty_pid) return;

    g_kitty = 1;

    /* save current font */
    read_cmd("kitty @ get-font-family 2>/dev/null | head -1",
             g_orig_font, sizeof(g_orig_font));

    /* try a font with good chess glyphs */
    for (int i = 0; chess_fonts[i]; i++) {
        if (font_available(chess_fonts[i])) {
            char cmd[512];
            snprintf(cmd, sizeof(cmd),
                     "kitty @ set-font-family \"%s\" 2>/dev/null", chess_fonts[i]);
            if (system(cmd) == 0) {
                g_font_changed = 1;
                break;
            }
        }
    }

    /* restore on any exit */
    signal(SIGINT, signal_handler);
    signal(SIGTERM, signal_handler);
    signal(SIGHUP, signal_handler);
}

/* Unicode chess pieces indexed by "PNBRQKpnbrqk" */
static const char *piece_unicode[] = {
    "♙", "♘", "♗", "♖", "♕", "♔",
    "♟", "♞", "♝", "♜", "♛", "♚",
};

/* ── Data types ────────────────────────────────────────────── */

typedef struct {
    char id[16];
    char fen[FEN_MAX];
    char moves[MOVES_MAX];
    int  rating;
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

/* ── Board ─────────────────────────────────────────────────── */

static char board[8][8];

static int piece_index(char c) {
    const char *map = "PNBRQKpnbrqk";
    const char *p = strchr(map, c);
    return p ? (int)(p - map) : -1;
}

static void fen_to_board(const char *fen) {
    memset(board, '.', sizeof(board));
    int rank = 0, file = 0;
    for (const char *p = fen; *p && *p != ' '; p++) {
        if (*p == '/') { rank++; file = 0; }
        else if (*p >= '1' && *p <= '8') file += *p - '0';
        else board[rank][file++] = *p;
    }
}

static char fen_side(const char *fen) {
    const char *p = strchr(fen, ' ');
    return p ? *(p + 1) : 'w';
}

static void apply_uci_move(const char *move) {
    if (strlen(move) < 4) return;
    int f1 = move[0] - 'a', r1 = 7 - (move[1] - '1');
    int f2 = move[2] - 'a', r2 = 7 - (move[3] - '1');
    char piece = board[r1][f1];

    if ((piece == 'K' || piece == 'k') && abs(f2 - f1) == 2) {
        board[r2][f2] = piece;
        board[r1][f1] = '.';
        if (f2 > f1) { board[r1][5] = board[r1][7]; board[r1][7] = '.'; }
        else          { board[r1][3] = board[r1][0]; board[r1][0] = '.'; }
        return;
    }

    if ((piece == 'P' || piece == 'p') && f1 != f2 && board[r2][f2] == '.')
        board[r1][f2] = '.';

    board[r2][f2] = piece;
    board[r1][f1] = '.';

    if (strlen(move) == 5) {
        char promo = move[4];
        board[r2][f2] = (piece == 'P') ? toupper(promo) : tolower(promo);
    }
}

/* ── Rendering ─────────────────────────────────────────────── */

static void print_board(int flipped) {
    printf("\n");
    for (int ri = 0; ri < 8; ri++) {
        int rank = flipped ? (7 - ri) : ri;
        int rank_label = flipped ? (ri + 1) : (8 - ri);

        printf(" %s%d%s ", DIM, rank_label, RST);
        for (int fi = 0; fi < 8; fi++) {
            int file = flipped ? (7 - fi) : fi;
            int is_light = (rank + file) % 2 == 0;
            const char *bg = is_light ? BG_LIGHT : BG_DARK;
            char c = board[rank][file];

            if (c == '.') {
                printf("%s   %s", bg, RST);
            } else {
                int idx = piece_index(c);
                const char *fg = (idx < 6) ? FG_WHITE : FG_BLACK;
                printf("%s%s %s %s", bg, fg, piece_unicode[idx], RST);
            }
        }
        printf("\n");
    }
    if (flipped)
        printf("    %sh  g  f  e  d  c  b  a%s\n", DIM, RST);
    else
        printf("    %sa  b  c  d  e  f  g  h%s\n", DIM, RST);
    printf("\n");
}

/* ── Puzzle loading ────────────────────────────────────────── */

static Puzzle puzzles[MAX_PUZZLES];
static int num_puzzles = 0;
static int random_order[MAX_PUZZLES];
static int random_order_pos = 0;

static char *trim(char *s);
static void normalize_move(char *dst, const char *src, int dstsz);

static char *find_puzzles_csv(const char *argv0, const char *arg) {
    static char path[512];
    if (arg) { snprintf(path, sizeof(path), "%s", arg); return path; }

    const char *slash = strrchr(argv0, '/');
    if (slash) {
        int dirlen = (int)(slash - argv0);
        snprintf(path, sizeof(path), "%.*s/puzzles.csv", dirlen, argv0);
    } else {
        snprintf(path, sizeof(path), "puzzles.csv");
    }
    if (access(path, R_OK) == 0) return path;

    snprintf(path, sizeof(path), "puzzles.csv");
    return path;
}

static int load_puzzles(const char *filepath) {
    FILE *f = fopen(filepath, "r");
    if (!f) { fprintf(stderr, "Cannot open %s\n", filepath); return -1; }

    char line[MAX_LINE];
    if (!fgets(line, sizeof(line), f)) { fclose(f); return -1; }

    num_puzzles = 0;
    while (fgets(line, sizeof(line), f) && num_puzzles < MAX_PUZZLES) {
        Puzzle *p = &puzzles[num_puzzles];
        char *fields[5] = {0};
        int nfields = 0;
        int in_quotes = 0;
        char *field_start = line;

        for (char *cur = line; *cur; cur++) {
            if (*cur == '"') {
                in_quotes = !in_quotes;
            } else if (*cur == ',' && !in_quotes && nfields < 4) {
                *cur = '\0';
                fields[nfields++] = field_start;
                field_start = cur + 1;
            }
        }
        fields[nfields++] = field_start;

        if (nfields != 5) continue;

        fields[0] = fields[0] ? fields[0] : "";
        fields[1] = fields[1] ? fields[1] : "";
        fields[2] = fields[2] ? fields[2] : "";
        fields[3] = fields[3] ? fields[3] : "";
        fields[4] = fields[4] ? fields[4] : "";

        strncpy(p->id, trim(fields[0]), sizeof(p->id) - 1);
        p->id[sizeof(p->id) - 1] = '\0';
        strncpy(p->fen, trim(fields[1]), sizeof(p->fen) - 1);
        p->fen[sizeof(p->fen) - 1] = '\0';
        strncpy(p->moves, trim(fields[2]), sizeof(p->moves) - 1);
        p->moves[sizeof(p->moves) - 1] = '\0';
        p->rating = atoi(trim(fields[3]));
        strncpy(p->themes, trim(fields[4]), sizeof(p->themes) - 1);
        p->themes[sizeof(p->themes) - 1] = '\0';
        num_puzzles++;
    }
    fclose(f);
    return num_puzzles;
}

/* ── Stats ─────────────────────────────────────────────────── */

static char *stats_path(void) {
    static char path[512];
    const char *home = getenv("HOME");
    if (home) snprintf(path, sizeof(path), "%s/%s", home, STATS_FILE);
    else      snprintf(path, sizeof(path), "%s", STATS_FILE);
    return path;
}

static Stats load_stats(void) {
    Stats s = {0};
    FILE *f = fopen(stats_path(), "r");
    if (f) {
        char line[MAX_LINE];

        if (fgets(line, sizeof(line), f)) {
            sscanf(line, "%d %d %d %d %d %d",
                   &s.current_streak, &s.best_streak,
                   &s.total_solved, &s.total_attempted,
                   &s.last_day, &s.last_year);
        }

        if (fgets(line, sizeof(line), f)) {
            for (int i = 0; i < MAX_PUZZLES && line[i]; i++) {
                if (line[i] == '0' || line[i] == '1')
                    s.solved[i] = (unsigned char)(line[i] - '0');
            }
        }
        fclose(f);
    }
    return s;
}

static void save_stats(Stats *s) {
    FILE *f = fopen(stats_path(), "w");
    if (f) {
        fprintf(f, "%d %d %d %d %d %d\n",
                s->current_streak, s->best_streak,
                s->total_solved, s->total_attempted,
                s->last_day, s->last_year);
        for (int i = 0; i < MAX_PUZZLES; i++)
            fputc(s->solved[i] ? '1' : '0', f);
        fputc('\n', f);
        fclose(f);
    }
}

/* ── Helpers ───────────────────────────────────────────────── */

static char *trim(char *s) {
    while (*s && isspace(*s)) s++;
    if (*s == '\0') return s;
    char *end = s + strlen(s) - 1;
    while (end > s && isspace(*end)) *end-- = '\0';
    return s;
}

static void normalize_move(char *dst, const char *src, int dstsz) {
    int j = 0;
    for (int i = 0; src[i] && j < dstsz - 1; i++) {
        char c = tolower(src[i]);
        if (c == 'x' || c == '-' || c == ' ' || c == '=' || c == '+' || c == '#')
            continue;
        dst[j++] = c;
    }
    dst[j] = '\0';
}

static int split_moves(const char *moves_str, char out[][MOVE_LEN], int max) {
    int n = 0;
    const char *p = moves_str;

    while (*p && n < max) {
        while (*p && isspace((unsigned char)*p))
            p++;
        if (!*p)
            break;

        int len = 0;
        while (p[len] && !isspace((unsigned char)p[len]) && len < MOVE_LEN - 1)
            len++;

        if (len > 0) {
            memcpy(out[n], p, len);
            out[n][len] = '\0';
            n++;
        }

        while (*p && !isspace((unsigned char)*p))
            p++;
    }

    return n;
}

static int parse_uci_square(const char *move, int offset, int *file, int *rank) {
    char f = move[offset];
    char r = move[offset + 1];

    if (f < 'a' || f > 'h' || r < '1' || r > '8')
        return 0;

    *file = f - 'a';
    *rank = 7 - (r - '1');
    return 1;
}

static int is_capture_move(const char *move, char piece) {
    int f1, r1, f2, r2;

    if (!parse_uci_square(move, 0, &f1, &r1) || !parse_uci_square(move, 2, &f2, &r2))
        return 0;

    if (board[r2][f2] != '.')
        return 1;

    return (piece == 'P' || piece == 'p') && f1 != f2;
}

static int normalized_equals(const char *lhs, const char *rhs) {
    char a[32];
    char b[32];

    normalize_move(a, lhs, sizeof(a));
    normalize_move(b, rhs, sizeof(b));
    return strcmp(a, b) == 0;
}

static void clear_screen(void) {
    printf("\033[2J\033[H");
    fflush(stdout);
}

static int count_solved_puzzles(const Stats *stats) {
    int count = 0;

    for (int i = 0; i < num_puzzles; i++)
        count += stats->solved[i] ? 1 : 0;

    return count;
}

static void wait_for_enter(void) {
    char input[64];

    printf("  %sPress Enter to continue...%s", FG_DIM, RST);
    fflush(stdout);
    if (!fgets(input, sizeof(input), stdin))
        clearerr(stdin);
}

static void format_next_potd_reset(char *buf, size_t bufsz) {
    time_t now = time(NULL);
    struct tm reset_tm = *localtime(&now);

    reset_tm.tm_hour = 0;
    reset_tm.tm_min = 0;
    reset_tm.tm_sec = 0;
    reset_tm.tm_mday += 1;
    mktime(&reset_tm);

    strftime(buf, bufsz, "%a %b %d, %I:%M %p %Z", &reset_tm);
}

static void shuffle_random_order(void) {
    for (int i = 0; i < num_puzzles; i++)
        random_order[i] = i;

    for (int i = num_puzzles - 1; i > 0; i--) {
        int j = rand() % (i + 1);
        int tmp = random_order[i];
        random_order[i] = random_order[j];
        random_order[j] = tmp;
    }

    random_order_pos = 0;
}

static int next_random_puzzle_index(void) {
    if (num_puzzles <= 0)
        return -1;

    if (random_order_pos >= num_puzzles)
        shuffle_random_order();

    return random_order[random_order_pos++];
}

static int move_matches_expected(const char *input, const char *expected) {
    int f1, r1, f2, r2;
    char src[3], dst[3], candidate[32];
    char piece, piece_letter;
    int capture;
    char promo = '\0';

    if (normalized_equals(input, expected))
        return 1;

    if (!parse_uci_square(expected, 0, &f1, &r1) || !parse_uci_square(expected, 2, &f2, &r2))
        return 0;

    piece = board[r1][f1];
    if (piece == '.')
        return 0;

    if ((piece == 'K' || piece == 'k') && abs(f2 - f1) == 2) {
        if (f2 > f1)
            return normalized_equals(input, "O-O");
        return normalized_equals(input, "O-O-O");
    }

    src[0] = (char)('a' + f1);
    src[1] = (char)('1' + (7 - r1));
    src[2] = '\0';
    dst[0] = (char)('a' + f2);
    dst[1] = (char)('1' + (7 - r2));
    dst[2] = '\0';
    capture = is_capture_move(expected, piece);
    piece_letter = (char)toupper((unsigned char)piece);

    if (strlen(expected) >= 5)
        promo = (char)tolower((unsigned char)expected[4]);

    if (piece == 'P' || piece == 'p') {
        if (capture) {
            if (promo) {
                snprintf(candidate, sizeof(candidate), "%c%s%c", src[0], dst, promo);
                if (normalized_equals(input, candidate))
                    return 1;
            } else {
                snprintf(candidate, sizeof(candidate), "%c%s", src[0], dst);
                if (normalized_equals(input, candidate))
                    return 1;
            }
        } else {
            if (promo) {
                snprintf(candidate, sizeof(candidate), "%s%c", dst, promo);
                if (normalized_equals(input, candidate))
                    return 1;
            } else if (normalized_equals(input, dst)) {
                return 1;
            }
        }
        return 0;
    }

    snprintf(candidate, sizeof(candidate), "%c%s", piece_letter, dst);
    if (normalized_equals(input, candidate))
        return 1;

    snprintf(candidate, sizeof(candidate), "%c%c%s", piece_letter, src[0], dst);
    if (normalized_equals(input, candidate))
        return 1;

    snprintf(candidate, sizeof(candidate), "%c%c%s", piece_letter, src[1], dst);
    if (normalized_equals(input, candidate))
        return 1;

    snprintf(candidate, sizeof(candidate), "%c%s%s", piece_letter, src, dst);
    if (normalized_equals(input, candidate))
        return 1;

    (void)capture;
    return 0;
}

/* ── Play a single puzzle ──────────────────────────────────── */
/* Returns: 1 = correct, -1 = wrong, 0 = user quit */

static int play_puzzle(int puzzle_idx, Puzzle *puz, Stats *stats, int is_potd) {
    char move_list[MOVE_LIST_MAX][MOVE_LEN];
    int nmoves = split_moves(puz->moves, move_list, MOVE_LIST_MAX);
    int setup_moves = (nmoves > 1) ? 1 : 0;
    int expected_idx = setup_moves;
    int total_player_moves = 0;

    if (nmoves <= expected_idx) {
        fprintf(stderr, "Puzzle %s has too few moves\n", puz->id);
        return -1;
    }

    fen_to_board(puz->fen);
    for (int i = 0; i < setup_moves; i++)
        apply_uci_move(move_list[i]);

    char side = fen_side(puz->fen);
    char player_side = side;
    for (int i = 0; i < setup_moves; i++)
        player_side = (player_side == 'w') ? 'b' : 'w';
    int flipped = (player_side == 'b');
    for (int i = expected_idx; i < nmoves; i += 2)
        total_player_moves++;

    stats->total_attempted++;

    for (int step = 0, i = expected_idx; i < nmoves; i += 2, step++) {
        const char *expected = move_list[i];
        char input[64];
        char normalized[16];

        for (;;) {
            clear_screen();

            if (is_potd)
                printf("\n  %s♟ Puzzle of the Day%s", BOLD, RST);
            else
                printf("\n  %s♟ Random Puzzle%s", BOLD, RST);

            printf("  %s#%s  ⭐ %d%s\n", FG_DIM, puz->id, puz->rating, RST);
            printf("  %s%s to move — find the best move!%s\n",
                   FG_HINT, player_side == 'w' ? "White" : "Black", RST);
            if (strlen(puz->themes) > 0)
                printf("  %sTheme: %s%s\n", FG_DIM, puz->themes, RST);
            printf("  %sStep %d/%d%s\n", FG_DIM, step + 1, total_player_moves, RST);

            print_board(flipped);
            printf("  %sYour move%s %s(UCI like e2e4, or SAN like Nf3):%s ",
                   BOLD, RST, FG_DIM, RST);
            fflush(stdout);

            if (!fgets(input, sizeof(input), stdin))
                return 0;

            char *answer = trim(input);
            normalize_move(normalized, answer, sizeof(normalized));

            if (strcmp(normalized, "q") == 0 || strcmp(normalized, "quit") == 0)
                return 0;

            if (strcmp(normalized, "hint") == 0) {
                printf("  %sHint:%s the piece starts on %c%c\n",
                       FG_HINT, RST, expected[0], expected[1]);
                printf("  %sPress Enter to try again...%s", FG_DIM, RST);
                fflush(stdout);
                if (!fgets(input, sizeof(input), stdin))
                    return 0;
                continue;
            }

            if (!move_matches_expected(answer, expected)) {
                clear_screen();
                printf("\n  %s✗ Not quite.%s  The best move was %s%s%s\n",
                       FG_FAIL, RST, BOLD, expected, RST);
                apply_uci_move(expected);
                print_board(flipped);

                stats->current_streak = 0;
                printf("  %s🔥 Streak: %d | Best: %d | Solved: %d/%d%s\n\n",
                       FG_DIM, stats->current_streak, stats->best_streak,
                       count_solved_puzzles(stats), num_puzzles, RST);
                return -1;
            }

            clear_screen();
            apply_uci_move(expected);
            break;
        }

        if (i + 1 < nmoves)
            apply_uci_move(move_list[i + 1]);
    }

    clear_screen();
    printf("\n  %s✓ Correct!%s  You solved the full line.\n", FG_OK, RST);
    print_board(flipped);

    stats->total_solved++;
    if (puzzle_idx >= 0 && puzzle_idx < MAX_PUZZLES)
        stats->solved[puzzle_idx] = 1;
    stats->current_streak++;
    if (stats->current_streak > stats->best_streak)
        stats->best_streak = stats->current_streak;

    printf("  %s🔥 Streak: %d | Best: %d | Solved: %d/%d%s\n\n",
           FG_DIM, stats->current_streak, stats->best_streak,
           count_solved_puzzles(stats), num_puzzles, RST);

    return 1;
}

/* ── Menu ──────────────────────────────────────────────────── */

static void print_menu(Stats *stats) {
    printf("\n");
    printf("  %s╔══════════════════════════════════════╗%s\n", FG_TITLE, RST);
    printf("  %s║%s      %s♟  c h e s s - c l i  ♟%s      %s║%s\n",
           FG_TITLE, RST, BOLD, RST, FG_TITLE, RST);
    printf("  %s╚══════════════════════════════════════╝%s\n", FG_TITLE, RST);
    printf("\n");
    printf("    %s[1]%s  Puzzle of the Day\n", BOLD, RST);
    printf("    %s[2]%s  Random Puzzle\n", BOLD, RST);
    printf("    %s[3]%s  Stats\n", BOLD, RST);
    printf("    %s[q]%s  Quit\n", BOLD, RST);
    printf("\n");
    if (stats->total_attempted > 0)
        printf("    %s🔥 %d streak | %d/%d solved%s\n",
               FG_DIM, stats->current_streak,
               count_solved_puzzles(stats), num_puzzles, RST);
    printf("\n");
}

static void print_stats(Stats *stats) {
    printf("\n");
    printf("  %s── Stats ──────────────────────────────%s\n\n", FG_TITLE, RST);
    printf("    Current streak:    %s%d%s\n", BOLD, stats->current_streak, RST);
    printf("    Best streak:       %s%d%s\n", BOLD, stats->best_streak, RST);
    printf("    Puzzles solved:    %s%d/%d%s\n", BOLD,
           count_solved_puzzles(stats), num_puzzles, RST);
    printf("    Solve events:      %s%d%s\n", BOLD, stats->total_solved, RST);
    printf("    Puzzles attempted: %s%d%s\n", BOLD, stats->total_attempted, RST);
    if (stats->total_attempted > 0)
        printf("    Accuracy:          %s%.0f%%%s\n", BOLD,
               100.0 * stats->total_solved / stats->total_attempted, RST);
    printf("\n");
}

/* ── Main ──────────────────────────────────────────────────── */

int main(int argc, char **argv) {
    terminal_setup();
    atexit(terminal_restore);
    kitty_setup();

    char *csv_path = find_puzzles_csv(argv[0], argc > 1 ? argv[1] : NULL);
    if (load_puzzles(csv_path) <= 0) {
        fprintf(stderr, "Failed to load puzzles from %s\n", csv_path);
        return 1;
    }

    srand((unsigned)time(NULL));
    Stats stats = load_stats();
    shuffle_random_order();

    time_t now = time(NULL);
    struct tm *t = localtime(&now);
    int day_of_year = t->tm_yday;
    int year = t->tm_year + 1900;

    char input[64];

    while (1) {
        clear_screen();
        print_menu(&stats);
        printf("  %s>%s ", BOLD, RST);
        fflush(stdout);

        if (!fgets(input, sizeof(input), stdin)) break;
        char *choice = trim(input);

        if (strcmp(choice, "q") == 0 || strcmp(choice, "quit") == 0)
            break;

        if (strcmp(choice, "1") == 0) {
            if (stats.last_day == day_of_year && stats.last_year == year) {
                char reset_at[64];
                format_next_potd_reset(reset_at, sizeof(reset_at));
                clear_screen();
                printf("\n  %sPOTD already solved, come back tomorrow!%s\n", FG_DIM, RST);
                printf("  %sResets at %s%s\n\n", FG_DIM, reset_at, RST);
                wait_for_enter();
                continue;
            }
            int potd_idx = (year * 366 + day_of_year) % num_puzzles;
            int result = play_puzzle(potd_idx, &puzzles[potd_idx], &stats, 1);
            if (result == 1) {
                stats.last_day = day_of_year;
                stats.last_year = year;
            }
            save_stats(&stats);

        } else if (strcmp(choice, "2") == 0) {
            int keep_going = 1;
            while (keep_going) {
                int idx = next_random_puzzle_index();
                if (idx < 0)
                    break;
                int result = play_puzzle(idx, &puzzles[idx], &stats, 0);
                save_stats(&stats);
                if (result == 0) break;

                printf("  %sAnother? [y/n]:%s ", BOLD, RST);
                fflush(stdout);
                if (!fgets(input, sizeof(input), stdin)) { keep_going = 0; break; }
                char *yn = trim(input);
                if (yn[0] != 'y' && yn[0] != 'Y') keep_going = 0;
            }

        } else if (strcmp(choice, "3") == 0) {
            clear_screen();
            print_stats(&stats);
            wait_for_enter();

        } else {
            printf("\n  %sPress 1, 2, 3, or q%s\n", FG_DIM, RST);
        }
    }

    printf("\n  %sGoodbye!%s\n\n", FG_DIM, RST);
    return 0;
}
