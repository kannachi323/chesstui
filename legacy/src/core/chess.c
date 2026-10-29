#include <ctype.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "chess.h"

static const char *piece_glyph(int idx) {
    switch (idx) {
        case  0: return PIECE_W_PAWN;
        case  1: return PIECE_W_KNIGHT;
        case  2: return PIECE_W_BISHOP;
        case  3: return PIECE_W_ROOK;
        case  4: return PIECE_W_QUEEN;
        case  5: return PIECE_W_KING;
        case  6: return PIECE_B_PAWN;
        case  7: return PIECE_B_KNIGHT;
        case  8: return PIECE_B_BISHOP;
        case  9: return PIECE_B_ROOK;
        case 10: return PIECE_B_QUEEN;
        case 11: return PIECE_B_KING;
        default: return " ";
    }
}

static char board[8][8];

static void append_text(char *dst, size_t dstsz, const char *text) {
    size_t len = strlen(dst);

    if (len >= dstsz - 1) {
        return;
    }

    snprintf(dst + len, dstsz - len, "%s", text);
}

static void append_square(char *dst, size_t dstsz, const char *bg, const char *fg,
                          const char *piece) {
    char square[64];

    if (!piece) {
        snprintf(square, sizeof(square), "%s%*s%s", bg, CHESS_BOARD_CELL_WIDTH, "", RST);
        append_text(dst, dstsz, square);
        return;
    }

    snprintf(square, sizeof(square), "%s%s %s %s", bg, fg, piece, RST);
    append_text(dst, dstsz, square);
}

static int piece_index(char c) {
    const char *map = "PNBRQKpnbrqk";
    const char *p = strchr(map, c);
    return p ? (int)(p - map) : -1;
}

void chess_set_position(const char *fen) {
    memset(board, '.', sizeof(board));

    int rank = 0;
    int file = 0;
    for (const char *p = fen; *p && *p != ' '; p++) {
        if (*p == '/') {
            rank++;
            file = 0;
        } else if (*p >= '1' && *p <= '8') {
            file += *p - '0';
        } else {
            board[rank][file++] = *p;
        }
    }
}

char chess_fen_side(const char *fen) {
    const char *p = strchr(fen, ' ');
    return p ? *(p + 1) : 'w';
}

void chess_apply_uci_move(const char *move) {
    if (strlen(move) < 4) {
        return;
    }

    int f1 = move[0] - 'a';
    int r1 = 7 - (move[1] - '1');
    int f2 = move[2] - 'a';
    int r2 = 7 - (move[3] - '1');
    char piece = board[r1][f1];

    if ((piece == 'K' || piece == 'k') && abs(f2 - f1) == 2) {
        board[r2][f2] = piece;
        board[r1][f1] = '.';
        if (f2 > f1) {
            board[r1][5] = board[r1][7];
            board[r1][7] = '.';
        } else {
            board[r1][3] = board[r1][0];
            board[r1][0] = '.';
        }
        return;
    }

    if ((piece == 'P' || piece == 'p') && f1 != f2 && board[r2][f2] == '.') {
        board[r1][f2] = '.';
    }

    board[r2][f2] = piece;
    board[r1][f1] = '.';

    if (strlen(move) == 5) {
        char promo = move[4];
        board[r2][f2] = (piece == 'P') ? toupper((unsigned char)promo)
                                       : tolower((unsigned char)promo);
    }
}

void chess_format_board_lines(int flipped,
                              char out[CHESS_BOARD_RENDER_LINES][CHESS_BOARD_RENDER_LINE_MAX]) {
    for (int ri = 0; ri < 8; ri++) {
        int rank = flipped ? (7 - ri) : ri;
        int rank_label = flipped ? (ri + 1) : (8 - ri);

        out[ri][0] = '\0';
        snprintf(out[ri], CHESS_BOARD_RENDER_LINE_MAX, " %s%d%s ", DIM, rank_label, RST);
        for (int fi = 0; fi < 8; fi++) {
            int file = flipped ? (7 - fi) : fi;
            int is_light = (rank + file) % 2 == 0;
            const char *bg = is_light ? BG_LIGHT : BG_DARK;
            char c = board[rank][file];

            if (c == '.') {
                append_square(out[ri], CHESS_BOARD_RENDER_LINE_MAX, bg, NULL, NULL);
            } else {
                int idx = piece_index(c);
                const char *fg = (idx < 6) ? FG_WHITE : FG_BLACK;
                append_square(out[ri], CHESS_BOARD_RENDER_LINE_MAX, bg, fg, piece_glyph(idx));
            }
        }
    }

    out[8][0] = '\0';
    snprintf(out[8], CHESS_BOARD_RENDER_LINE_MAX, "   %s", DIM);
    for (int fi = 0; fi < 8; fi++) {
        int file = flipped ? (7 - fi) : fi;
        char label[16];

        snprintf(label, sizeof(label), " %c ", (char) ('a' + file));
        append_text(out[8], CHESS_BOARD_RENDER_LINE_MAX, label);
    }
    append_text(out[8], CHESS_BOARD_RENDER_LINE_MAX, RST);
}

void chess_print_board(int flipped) {
    char lines[CHESS_BOARD_RENDER_LINES][CHESS_BOARD_RENDER_LINE_MAX];

    chess_format_board_lines(flipped, lines);

    printf("\n");
    for (int i = 0; i < CHESS_BOARD_RENDER_LINES; i++) {
        printf("%s\n", lines[i]);
    }

    printf("\n");
}

void chess_get_board_snapshot(char out[CHESS_BOARD_SNAPSHOT_SIZE]) {
    memcpy(out, board, CHESS_BOARD_SNAPSHOT_SIZE);
}

void chess_set_board_snapshot(const char in[CHESS_BOARD_SNAPSHOT_SIZE]) {
    memcpy(board, in, CHESS_BOARD_SNAPSHOT_SIZE);
}

static void normalize_move(char *dst, const char *src, int dstsz) {
    int j = 0;

    for (int i = 0; src[i] && j < dstsz - 1; i++) {
        char c = (char)tolower((unsigned char)src[i]);
        if (c == 'x' || c == '-' || c == ' ' || c == '=' || c == '+' || c == '#') {
            continue;
        }
        dst[j++] = c;
    }

    dst[j] = '\0';
}

int chess_split_moves(const char *moves_str, char out[][MOVE_LEN], int max) {
    int n = 0;
    const char *p = moves_str;

    while (*p && n < max) {
        while (*p && isspace((unsigned char)*p)) {
            p++;
        }
        if (!*p) {
            break;
        }

        int len = 0;
        while (p[len] && !isspace((unsigned char)p[len]) && len < MOVE_LEN - 1) {
            len++;
        }

        if (len > 0) {
            memcpy(out[n], p, (size_t)len);
            out[n][len] = '\0';
            n++;
        }

        while (*p && !isspace((unsigned char)*p)) {
            p++;
        }
    }

    return n;
}

static int parse_uci_square(const char *move, int offset, int *file, int *rank) {
    char f = move[offset];
    char r = move[offset + 1];

    if (f < 'a' || f > 'h' || r < '1' || r > '8') {
        return 0;
    }

    *file = f - 'a';
    *rank = 7 - (r - '1');
    return 1;
}

static int is_capture_move(const char *move, char piece) {
    int f1;
    int r1;
    int f2;
    int r2;

    if (!parse_uci_square(move, 0, &f1, &r1) || !parse_uci_square(move, 2, &f2, &r2)) {
        return 0;
    }

    if (board[r2][f2] != '.') {
        return 1;
    }

    return (piece == 'P' || piece == 'p') && f1 != f2;
}

static int normalized_equals(const char *lhs, const char *rhs) {
    char a[32];
    char b[32];

    normalize_move(a, lhs, sizeof(a));
    normalize_move(b, rhs, sizeof(b));
    return strcmp(a, b) == 0;
}

int chess_move_matches_expected(const char *input, const char *expected) {
    int f1;
    int r1;
    int f2;
    int r2;
    char src[3];
    char dst[3];
    char candidate[32];
    char piece;
    char piece_letter;
    int capture;
    char promo = '\0';

    if (normalized_equals(input, expected)) {
        return 1;
    }

    if (!parse_uci_square(expected, 0, &f1, &r1) || !parse_uci_square(expected, 2, &f2, &r2)) {
        return 0;
    }

    piece = board[r1][f1];
    if (piece == '.') {
        return 0;
    }

    if ((piece == 'K' || piece == 'k') && abs(f2 - f1) == 2) {
        if (f2 > f1) {
            return normalized_equals(input, "O-O");
        }
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

    if (strlen(expected) >= 5) {
        promo = (char)tolower((unsigned char)expected[4]);
    }

    if (piece == 'P' || piece == 'p') {
        if (capture) {
            if (promo) {
                snprintf(candidate, sizeof(candidate), "%c%s%c", src[0], dst, promo);
                if (normalized_equals(input, candidate)) {
                    return 1;
                }
            } else {
                snprintf(candidate, sizeof(candidate), "%c%s", src[0], dst);
                if (normalized_equals(input, candidate)) {
                    return 1;
                }
            }
        } else {
            if (promo) {
                snprintf(candidate, sizeof(candidate), "%s%c", dst, promo);
                if (normalized_equals(input, candidate)) {
                    return 1;
                }
            } else if (normalized_equals(input, dst)) {
                return 1;
            }
        }
        return 0;
    }

    snprintf(candidate, sizeof(candidate), "%c%s", piece_letter, dst);
    if (normalized_equals(input, candidate)) {
        return 1;
    }

    snprintf(candidate, sizeof(candidate), "%c%c%s", piece_letter, src[0], dst);
    if (normalized_equals(input, candidate)) {
        return 1;
    }

    snprintf(candidate, sizeof(candidate), "%c%c%s", piece_letter, src[1], dst);
    if (normalized_equals(input, candidate)) {
        return 1;
    }

    snprintf(candidate, sizeof(candidate), "%c%s%s", piece_letter, src, dst);
    if (normalized_equals(input, candidate)) {
        return 1;
    }

    return 0;
}
