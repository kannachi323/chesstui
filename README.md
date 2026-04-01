# chess-cli

A terminal chess puzzle trainer powered by the [Lichess puzzle database](https://database.lichess.org/#puzzles).

```
  ╔══════════════════════════════════════╗
  ║      ♟  c h e s s - c l i  ♟      ║
  ╚══════════════════════════════════════╝
```

## Requirements

- C compiler (gcc or clang)
- `curl`
- `zstd` (for decompressing puzzle packs)

## Build

```sh
make
```

## Getting Puzzles

You need a `puzzles.csv` file in the same directory as the binary. There are two ways to get one.

### Option 1 — Full database (3M+ puzzles)

Downloads the complete Lichess puzzle database (~300 MB compressed):

```sh
make fetch-puzzles
```

This produces a `puzzles.csv` in the current directory.

### Option 2 — Rating-based packs (recommended)

After fetching the full database, split it into smaller packs by rating:

```sh
make split-puzzles
```

This creates a `packs/` directory with five files:

| File | Rating range |
|------|-------------|
| `puzzles-0-1000.csv.zst` | Beginner |
| `puzzles-1000-1500.csv.zst` | Intermediate |
| `puzzles-1500-2000.csv.zst` | Club player |
| `puzzles-2000-2500.csv.zst` | Advanced |
| `puzzles-2500-plus.csv.zst` | Expert |

Decompress the pack you want:

```sh
zstd -d packs/puzzles-1000-1500.csv.zst -o puzzles.csv
```

## Run

```sh
./chess-cli
```

To load a specific puzzle file:

```sh
./chess-cli path/to/puzzles.csv
```

## Menu

| Key | Action |
|-----|--------|
| `1` | Puzzle of the Day |
| `2` | Random puzzle |
| `3` | View stats |
| `q` | Quit |
