<img width="1200" height="264" alt="image" src="https://github.com/user-attachments/assets/fd0e0ffb-b70b-4bd4-9377-8737a19a94fa" />

<div align="center">
  
[![Discord](https://img.shields.io/discord/1489013166292734104?logo=discord&label=Discord&logoColor=white&labelColor=black&color=F5F5F5)](https://discord.gg/GqEYhbaM)
[![release](https://img.shields.io/github/v/release/kannachi323/chess-cli?style=flat&labelColor=black&color=F5F5F5)](https://github.com/kannachi323/chess-cli/releases/latest)
</div>

`chess-cli` lets you play chess directly in your terminal. Play against your friends online or challenge yourself against Stockfish and other strong AI opponents. Work through millions of puzzles to sharpen your tactical vision.                                                                      
                                                                                                                                                     
## Features      
Check out [Features](./FEATURES.md) for a full list of features.

- Play online games connected to your Lichess account. 
- Puzzles from the [Lichess puzzle database](https://database.lichess.org/#puzzles) (5.8M+ puzzles)
- Daily puzzle of the day                                                                                                                          
- Random puzzles by rating band (0-2500+ elo)                                                                                               
- Stat tracking and leaderboards (**coming soon**)


## Installation
`chess-cli` is designed to stay simple and lightweight. If you want the fastest setup, use the quick install command.
If you prefer compiling it yourself, you can build it from source with CMake. Be sure to also check out [Getting Puzzles](#getting-puzzles) for more information on downloading puzzles.

### Quick install
```sh
curl -fsSL https://raw.githubusercontent.com/kannachi323/chess-cli/main/install.sh | sh
```

### Install from package
Pre-built packages for Windows, macOS, and Linux are found on the [Releases](https://github.com/kannachi323/chess-cli/releases) page.

You can also install via package managers. See [INSTALL.md](./INSTALL.md) for details.

### Build from source
> **Note**: You will need a C compiler (`gcc`, `clang`, or `msvc`), `curl`, `zstd`
```sh
git clone https://github.com/kannachi323/chess-cli
cd chess-cli
mkdir build && cd build
cmake ..
make
```
> **Note**: If you want to execute `chess-cli` from any directory, you must properly add to your environment `$PATH`.

## Getting Puzzles
In order to play puzzles, you will download puzzles and store them locally at `~/.chess-cli/puzzles`. Be sure to check out the [Discord](https://discord.gg/hyFpBQkp) for
updates on puzzle packs. They will be always be updated with the [Lichess puzzle database](https://database.lichess.org/#puzzles)

The puzzles are split into different skill levels using the [Lichess elo system](https://lichess.org/page/rating-systems#lichess):

| Pack | Rating |
|------|--------|
| `puzzles-0-1000.csv.zst` | 0-1000 |
| `puzzles-1000-1500.csv.zst` | 1000-1500 |
| `puzzles-1500-2000.csv.zst` | 1500-2000 |
| `puzzles-2000-2500.csv.zst` | 2000-2500 |
| `puzzles-2500-plus.csv.zst` | 2500+ |

Download the puzzles file using the following format `curl -LO https://github.com/kannachi323/chess-cli/releases/download/<version>/puzzles-1000-1500.csv.zst`
where `<version>` follows the format `vX.X.X`.
```sh
# Example: download the intermediate pack
curl -LO https://github.com/<you>/chess-cli/releases/download/v1.0.0/puzzles-1000-1500.csv.zst
zstd -d puzzles-1000-1500.csv.zst -o puzzles-1000-1500.csv
mkdir ~/.chess-cli/puzzles
mv puzzles-1000-1500.csv ~/.chess-cli/puzzles
```
## Usage
Run from the directory containing the `chess-cli` binary, or add it to your `$PATH`
```sh
./chess-cli
```
That's it. Everything else will be guided through prompts and menu selection.
