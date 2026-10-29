# ChessTUI

ChessTUI is a polished, local-first chess workspace for the terminal. It is written in Go with Bubble Tea and connects directly to Lichess—there is no ChessTUI server and no account password ever passes through the app.

## What works

- Play Lichess AI or challenge another Lichess user through the Board API.
- Play locally against an installed Stockfish engine.
- Watch every Lichess TV channel natively, with a local back/forward position timeline.
- Browse live Lichess streamers and top broadcasts, opening video in the system browser.
- Solve the daily puzzle, request Lichess puzzles, or use a local SQLite collection.
- Stream-import official Lichess CSV and `.zst` puzzle packs without loading them into memory.
- Fetch recent games and studies, replay completed games, and analyze positions with local Stockfish.
- Work through interactive lessons for movement, development, castling, and checkmate.
- Keep puzzle attempts, games, and analysis in a private local database.

Terminal protocols cannot embed browser iframes or safely render Twitch/YouTube video. ChessTUI therefore renders live chess positions itself and opens stream/broadcast video in your browser.

## Install from source

Requirements: Go 1.26 or later. Stockfish is optional and only required for local engine play and completed-game analysis.

```sh
git clone https://github.com/kannachi323/chess-cli.git
cd chess-cli
make build
./build/chesstui
```

On macOS, install Stockfish with `brew install stockfish`. On Linux, use your distribution package. You may also set an explicit `stockfish_path` in the generated config file shown on the Settings screen.

## First run

Press `2`, then `a` to connect Lichess. ChessTUI opens Lichess’s OAuth page and receives the result on a temporary loopback listener. The token is saved to the operating-system credential store, never to the config or SQLite database.

Global keys:

| Key | Action |
|---|---|
| `1`–`7` | Switch Home, Play, Watch, Puzzles, Study, Learn, Settings |
| `h` `j` `k` `l` | Move within the focused region |
| `Tab` / `Shift+Tab` | Alternate navbar and content focus |
| `Ctrl-W k` / `Ctrl-W j` | Focus navbar / content, Vim split-style |
| `gt` / `gT` | Next / previous workspace |
| `H` / `L` | Previous / next workspace |
| `:` | Enter command mode (`:home`, `:watch`, `:tn`, `:tp`, `:help`, `:q`) |
| `Esc` | Return focus to the navbar |
| `?` | Toggle help |
| `q` / `Ctrl+C` | Quit |

The Neovim-style statusline remains attached to the terminal’s final row and shows the current mode, focus region, and contextual controls.

### Board rendering

The board uses native terminal cells rather than HTML. It automatically chooses 6×3, 4×2, or compact 3×1 squares so their physical proportions remain square across normal terminal fonts. Settings offers solid Unicode, classic Unicode, and ASCII piece profiles.

## Local data

ChessTUI follows the XDG directory conventions on Linux and their equivalents on macOS:

- configuration: `chesstui/config.json`
- SQLite library: `chesstui/chesstui.db`
- logs: `chesstui/chesstui.log`
- Lichess token: OS keyring, or the ephemeral `CHESSTUI_LICHESS_TOKEN` environment override

See [Architecture](docs/ARCHITECTURE.md), [Security](SECURITY.md), and the [implementation report](docs/IMPLEMENTATION_REPORT.md).

## Development

```sh
make format
make test
make race
make vet
make run
```

### Website

The keyboard-driven browser demo lives in `web/` and is configured for Vercel from the repository root.

```sh
npm install --prefix web
npm run dev --prefix web
```

The website mirrors ChessTUI's navigation and appearance, uses Lichess's public APIs for TV, broadcasts, streamers, and puzzles, and supports browser-safe Lichess OAuth with PKCE. Authenticated users can create casual Lichess AI games, challenge another user, stream game state, and abort or resign from the browser. Moves can be entered as UCI, SAN, or a resulting FEN, or played entirely in NAV mode by moving a board cursor with `hjkl`, pressing Enter to select a piece, and pressing Enter again on a highlighted legal destination. Features that require local binaries, SQLite, or the operating-system keychain (notably the bundled Stockfish flow) remain in the native app.

The maintained Go module lives in `chesstui/`. The prior C implementation remains in the repository for migration history but is not part of the ChessTUI build.

## Fair play and licensing

ChessTUI never exposes Stockfish to an active online game. Engine use is limited to local games and completed-game study, consistent with Lichess fair-play rules.

ChessTUI is Apache-2.0 licensed. Stockfish is a separate GPL program discovered at runtime and is not bundled. Lichess trademarks and content belong to their respective owners.
