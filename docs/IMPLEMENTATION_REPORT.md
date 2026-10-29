> Historical: the July 2026 report for the first Go rewrite. For the current state, see the [README](../README.md) and [RELEASE.md](RELEASE.md).

# ChessTUI implementation report

Date: July 16, 2026  
Branch: `codex/chesstui-go-rewrite`

## Outcome

The unfinished C command-line program now has a maintained Go successor in `chesstui/`. The old C tree was intentionally left in place because it contained pre-existing, uncommitted work. The new application builds as `build/chesstui` and is organized as 41 focused Go files; the largest is under 500 lines.

## Delivered

### Terminal experience

- Bubble Tea v2 application shell with alternate-screen rendering and optional mouse tracking.
- Responsive layouts tested at 80×20 and 120×30.
- Exact-height centered editor canvas with a bottom-anchored Neovim-style statusline.
- Normal and Command modes, focusable navbar/content regions, `hjkl`, `gt`/`gT`, `Ctrl-W`, and named `:` workspace commands.
- Seven workspaces: Home, Play, Watch, Puzzles, Study, Learn, and Settings.
- Midnight color system, Unicode or ASCII pieces, responsive cell sizing, coordinates, last-move highlighting, flipped boards, global help, and contextual key guides.
- Framed board geometry adapts between 6×3, 4×2, and 3×1 terminal cells, with solid, classic, and ASCII piece profiles for font compatibility.
- Interactive lessons covering pawn movement, development, castling, and mate.

### Lichess integration

- OAuth Authorization Code + PKCE login through the system browser and a temporary loopback callback.
- Typed clients for account, Board API game/event streams, moves, resign/abort/draw, AI challenges, user challenges, puzzles, TV, streamers, broadcasts, studies, and game export.
- Public TV channel list and native live-board rendering.
- A bounded local TV snapshot timeline with back/forward controls and stream replacement/cancellation.
- Live streamer and broadcast discovery with validated external-browser launch.
- API request serialization, bounded errors, explicit timeouts, rate-limit messages, NDJSON limits, and context cancellation.

### Play

- Lichess AI games at a safe default 5+3 setup.
- Direct challenges to another Lichess username.
- Authoritative online state from Lichess streams; local validation happens before move submission.
- Draw, abort, and confirmed-resign actions.
- Local human-vs-Stockfish play through an isolated UCI process.
- A hard boundary preventing Stockfish from being called by active online-game paths.

### Puzzles and local data

- Daily Lichess puzzle, next online puzzle, and rating-filtered local random puzzle.
- Move validation, hints, forced replies, successful/failed attempt history, elapsed time, and streak statistics.
- Streaming official Lichess CSV and Zstandard puzzle-pack imports in 1,000-row transactions.
- SQLite WAL database for puzzles, attempts, daily metadata, completed games, and per-ply analysis.
- Recent Lichess games cached locally when fetched; Stockfish results persisted by game, ply, and depth.

### Study and analysis

- Fetches the connected account’s recent games and Lichess studies.
- Completed-game board replay with back/forward ply navigation.
- Local Stockfish analysis with evaluation and principal variation.
- Study pages open in the browser; recent games are analyzed natively in the TUI.

### Security and delivery

- OS credential-store abstraction with a development-only environment override; no plaintext token files.
- Atomic `0600` config writes, protected data directories, parameterized SQL, URL scheme validation, and shell-free subprocess execution.
- JSON file logging outside the terminal screen.
- Make targets, GitHub CI with race/vet/build checks, and tag-driven GoReleaser archives for Linux, macOS, and Windows on amd64/arm64.
- Architecture, security, build, usage, fair-play, and licensing documentation.

## Verification performed

- `go test ./...`: passed.
- `go test -race ./...`: passed.
- `go vet ./...`: passed.
- `git diff --check`: passed.
- 14 named regression/integration tests passed, covering chess coordinates and timelines, board orientations, navigation while entering moves, SQLite, puzzle import, Stockfish parsing, API authentication, TV streams, sorting, and rate-limit sanitization.
- GoReleaser v2.15.4 configuration check: passed.
- Local production build and `--version` smoke test: passed.
- Interactive PTY exercise at compact and wide terminal sizes: passed.
- Footer anchoring is regression-tested by asserting that a 50-row terminal renders exactly 50 rows, including oversized child content.
- Real Lichess public service exercise: TV channel discovery, live `best` and variant game streams, timeline updates, daily puzzles, streamer discovery, and broadcast discovery all returned and rendered successfully.

## Environment-dependent verification still required

Two flows require the owner’s local consent or software and were therefore not performed automatically:

1. Complete OAuth in the browser and play a real authenticated game. The endpoint contracts and streaming behavior are covered by tests, but no access token was available or requested during implementation.
2. Install Stockfish and play a full local engine game. No Stockfish binary was installed on this Mac; the UCI adapter and parser were tested independently. ChessTUI reports the missing executable clearly and accepts an explicit configured path.

## Intentional product constraint

A terminal cannot host a browser iframe or provide Twitch/YouTube’s video runtime. ChessTUI renders live Lichess chess natively—with timeline controls—and opens video streams or broadcast pages in the system browser, where the provider’s refresh, seek, captions, and playback controls remain available.
