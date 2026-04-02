<img width="1200" height="385" alt="b328e384-9886-4a37-b140-0598c9d2c2e6" src="https://github.com/user-attachments/assets/828f2d37-47fc-4f14-9466-dac14eaf6e0c" />

<<<<<<< HEAD
<div align="center">
  
[![Discord](https://img.shields.io/discord/1489013166292734104?logo=discord&label=Discord&logoColor=white&labelColor=black&color=F5F5F5)](https://discord.gg/GqEYhbaM)
[![release](https://img.shields.io/github/v/release/kannachi323/chess-cli?style=flat&labelColor=black&color=F5F5F5)](https://github.com/kannachi323/chess-cli/releases/latest)

</div>

chess-cli lets you play chess directly in your terminal. Compete against yourself, track your improvement over time, and work through thousands of 
  puzzles without ever leaving the command line.                                                                                                     
                                                                                                                                                     
  ## Features                                                                                                                                        
  - Puzzles from the [Lichess puzzle database](https://database.lichess.org/#puzzles) (3M+ puzzles)
  - Daily puzzle of the day                                                                                                                          
  - Random puzzles by rating band (beginner to expert)                                                                                               
  - Stat tracking and leaderboards                                        

## Installation

=======
A terminal chess puzzle trainer built in C, powered by the [Lichess puzzle database](https://database.lichess.org/#puzzles). Sharpen your tactics with over 3 million puzzles ranging from beginner to grandmaster level — all without leaving the command line.

Work through a daily puzzle, grind random puzzles by rating band, and track your progress over time with built-in stats and leaderboards. Puzzle packs are split by rating so you can target exactly the difficulty you want.

**Coming soon:** Claude bot integration — pit LLM agents against puzzles and watch how AI reasons through tactics in real time.

<div align="center">
<pre>
╔══════════════════════════════════════╗
║      ♟  c h e s s - c l i  ♟         ║
╚══════════════════════════════════════╝
</pre>
</div>
>>>>>>> da238aa (fixing stats)

## Requirements

- `C compiler` (gcc, clang, msvc)
- `CURL` (downloading puzzle packs)
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
