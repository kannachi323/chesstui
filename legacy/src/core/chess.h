#pragma once

#include "common.h"

#define CHESS_BOARD_SNAPSHOT_SIZE 64
#define CHESS_BOARD_CELL_WIDTH 3
#define CHESS_BOARD_RENDER_LINES 9
#define CHESS_BOARD_RENDER_LINE_MAX 1024

void chess_set_position(const char *fen);
char chess_fen_side(const char *fen);
void chess_apply_uci_move(const char *move);
void chess_print_board(int flipped);
void chess_format_board_lines(int flipped,
                              char out[CHESS_BOARD_RENDER_LINES][CHESS_BOARD_RENDER_LINE_MAX]);
void chess_get_board_snapshot(char out[CHESS_BOARD_SNAPSHOT_SIZE]);
void chess_set_board_snapshot(const char in[CHESS_BOARD_SNAPSHOT_SIZE]);
int chess_split_moves(const char *moves_str, char out[][MOVE_LEN], int max);
int chess_move_matches_expected(const char *input, const char *expected);
