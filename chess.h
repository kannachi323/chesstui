#ifndef CHESS_H
#define CHESS_H

#include "chess_cli.h"

void chess_set_position(const char *fen);
char chess_fen_side(const char *fen);
void chess_apply_uci_move(const char *move);
void chess_print_board(int flipped);
int chess_split_moves(const char *moves_str, char out[][MOVE_LEN], int max);
int chess_move_matches_expected(const char *input, const char *expected);

#endif
