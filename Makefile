CC ?= gcc
CFLAGS ?= -std=c11 -Wall -Wextra -pedantic

SRCS = main.c cli.c chess.c chess_stats.c puzzle.c terminal.c util.c
OBJS = $(SRCS:.c=.o)

chess-cli: $(OBJS)
	$(CC) $(CFLAGS) -o $@ $(OBJS)

.PHONY: clean fetch-puzzles split-puzzles

clean:
	rm -f $(OBJS) chess-cli

fetch-puzzles:
	@echo "Downloading full Lichess puzzle database..."
	curl -L -o lichess_db_puzzle.csv.zst \
		https://database.lichess.org/lichess_db_puzzle.csv.zst
	zstd -d --rm lichess_db_puzzle.csv.zst -o puzzles.csv
	@echo "Downloaded $$(( $$(wc -l < puzzles.csv) - 1 )) puzzles"

split-puzzles: puzzles.csv
	./split-puzzles.sh puzzles.csv packs
