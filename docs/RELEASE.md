# Releasing ChessTUI

Releases are built by [GoReleaser](https://goreleaser.com) from `.goreleaser.yaml`, and the GitHub Actions workflow in `.github/workflows/release.yml` runs it when a `v*` tag is pushed. The workflow creates a draft release, so nothing becomes public until you press **Publish** on GitHub.

## What a release contains

| Platform | Archive |
|---|---|
| macOS (Apple silicon, Intel) | `chesstui_<version>_darwin_{arm64,amd64}.tar.gz` |
| Linux (x86-64, ARM64) | `chesstui_<version>_linux_{amd64,arm64}.tar.gz` |
| Windows (x86-64) | `chesstui_<version>_windows_amd64.zip` |

Each archive holds the `chesstui` binary, `LICENSE`, `README.md` and `SECURITY.md`. A `checksums.txt` (SHA-256) covers every archive. Binaries are static (`CGO_ENABLED=0`; SQLite is the pure-Go modernc driver) and are built with `-trimpath`.

## Version information

`main.version`, `main.commit` and `main.date` are set through `-ldflags -X`:

```sh
chesstui --version
# chesstui 2.0.0 (eef5bab, 2026-10-29T01:23:39Z)
```

`make build` stamps the output of `git describe`. A plain `go build` falls back to the module and VCS data Go embeds, then to `dev`. The version is also sent in the Lichess `User-Agent` header (`ChessTUI/<version> (+https://github.com/kannachi323/chesstui)`).

## Checklist

1. `make test race vet` pass. `make live` passes too. It makes about eight public Lichess requests, so run it once per release, not in CI.
2. `make shots` regenerates `docs/screenshots/`. Commit any changes.
3. `goreleaser check`, then `make snapshot`. This builds every archive into `dist/` without publishing anything. Try the binary for your own platform.
4. Tag and push:

   ```sh
   git tag -a v2.0.0 -m "ChessTUI 2.0.0"
   git push origin v2.0.0
   ```

5. Open the draft release on GitHub, check the notes and assets, then publish it.

## Homebrew

The `brews` block in `.goreleaser.yaml` is commented out. To turn it on:

1. Create a public repository `kannachi323/homebrew-tap`.
2. Create a fine-grained token with *Contents: read and write* on that repository only. Store it in this repository's secrets as `HOMEBREW_TAP_GITHUB_TOKEN`.
3. Replace the placeholders (homepage, commit e-mail), uncomment the block, and pass the secret to the release step in `release.yml`:

   ```yaml
   env:
     GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
     HOMEBREW_TAP_GITHUB_TOKEN: ${{ secrets.HOMEBREW_TAP_GITHUB_TOKEN }}
   ```

Users then install with `brew install kannachi323/tap/chesstui`.

## Decisions made while preparing this release

These were decided without the owner and are easy to revisit.

- **Branch.** All work is on the local branch `production-ready`, cut from `codex/chesstui-go-rewrite`. The first commit on it is the owner's in-progress work, committed unchanged. Nothing was pushed, tagged, published or deployed.
- **Legacy code.** The C CLI (`src/`, CMake, the curl submodule, `scripts/split-puzzles.sh`, the C `todo.txt`) moved unchanged to `legacy/`, with a README explaining how to build it. `third_party/curl` was a dangling submodule pointer: an empty folder whose `.git` file pointed at a path that no longer exists. It held no code and was removed. The ignored `build/` folder still holds old CMake output and can be deleted locally.
- **Version number.** The existing `v1.0.0` tag is the C CLI. The Go rewrite is a different program, so its first release should be **v2.0.0**.
- **Repository name.** The Go module path, User-Agent and GoReleaser config use `github.com/kannachi323/chesstui`, but the remote is still `chess-cli`. Renaming the repository on GitHub keeps redirects for the old URL. Until then, change `release.github.name` to `chess-cli` if you release first.
- **`go install`.** The module lives in the `chesstui/` subdirectory, so `go install …@version` would need `chesstui/vX.Y.Z` tags. Release archives (and later Homebrew) are the supported install path. The README documents building from source.
- **Windows ARM64** is skipped: there is little demand and it is untested. Remove the `ignore` entry to add it.
- **Draft releases.** `release.draft: true` makes publishing a manual step.
- **Token storage.** The OS keychain is used first (Keychain, Secret Service, Windows Credential Manager). When none is available, for example on a headless Linux server, the token goes to `<data dir>/lichess-token` with mode 0600 in a 0700 directory. Signing out revokes the token on Lichess and deletes both copies.
- **Request etiquette.** Requests are serialized per host. A 429 pauses every request for at least 60 seconds, or longer if `Retry-After` says so. Idempotent GETs retry twice on network errors or 5xx. Sections load the first time they are opened, not at start-up.
- **Theme.** The UI never paints the terminal background. It asks the terminal for its background colour and picks light or dark text to match. Settings can force `dark` or `light`. The old `midnight` value is read as `auto`.
- **Mouse.** Mouse tracking is no longer turned on: nothing in the UI used it, and it blocked text selection. The `mouse_enabled` config key is kept for compatibility.
- **Key bindings.** On a board, letters that can start a move (`a`–`h`, `N B R Q K O`) go to move entry, so screen actions use other keys: `r` resign, `x` abort, `o` offer draw, `space` hint, `t` today's puzzle.
- **Stockfish.** It is not bundled, because it is GPL and a separate program. It is found on `PATH` or through `stockfish_path`. Stockfish was not installed on the build machine, so the engine plumbing is tested against a fake UCI engine. A real Stockfish game and analysis are still unverified (see below).
- **Screenshots** are SVG files rendered from real UI frames against recorded public Lichess responses (`chesstui/internal/app/testdata`), so they always match the code.
- **Web demo.** The keyboard-driven browser demo was replaced by a static landing page (see `web/`). The old demo's OAuth and gameplay code is in git history (commit `deab35f`) if you want it back.

### Final polish pass

- **No all-caps text.** The block-letter `CHESSTUI` wordmark, the `LESSONS` and `LESSON n OF m` labels, and the upper-cased help headings are now sentence case. Home shows a lowercase wordmark beside a starting-position board.
- **One layout rhythm.** The header and footer share a two-cell inset. Every screen opens with one bold title or a tab row. Footer status text is muted, and a warning gets room ahead of key hints.
- **Gaps closed.**
  - Play has a Color option (random, white or black) for Lichess AI, challenges and offline games. As black offline, Stockfish moves first.
  - The Stockfish row says "not installed" instead of failing.
  - Opponent draw offers show up, and `o` accepts them.
  - Esc cancels a sign-in or a pending challenge.
  - During a live online game, `q` (or `:q`) asks again before quitting; `:q!` skips the question.
  - Unknown `:` commands are reported.
  - Pasting works in the username and path fields, and only the visible screen receives it.
  - Puzzle-pack import accepts quoted, escaped or `~/` paths and shows live progress.
  - Settings shows where Stockfish was found and shortens long paths.
- **Landing page.** One typeface, Geist, self-hosted through `@fontsource-variable/geist` with no third-party font request. A 4–128 px spacing scale. The install command wraps only after a slash at 375 px.
- **Before and after.** `docs/screenshots/before/` holds the screenshots from before this pass. The files in `docs/screenshots/` are the current ones.
- **Release check.** `goreleaser check` passed with GoReleaser v2.19.0. A local `--snapshot` run built all five archives and `checksums.txt`, and the darwin/arm64 binary reported its stamped version. The binary was downloaded to a temporary folder, verified against the release checksums and deleted afterwards, so GoReleaser is still not installed.

### Local-first pass

- **Bring your own engine.** chesstui ships no AI. A single long-running UCI process is reused for play, review and live analysis, and receives the whole move list each turn. It works with Stockfish or any UCI engine set in Settings.
- **New sections.**
  - Play now means games against that engine, saved as PGN after every move, resumable, with take-backs.
  - Analyze reviews games and supports side lines and comments.
  - Read is a book reader with an included starter book. Its prose is original; the games in it are public classics.
- **Lichess (beta).** TV, a player's games and studies, and online play share one tab that Settings can hide. Puzzles default to imported packs; the Lichess daily is available while the beta is on.
- **Testing without Stockfish.** A small fake UCI engine in `chesstui/internal/app/testdata/fakeengine` is built during tests, so Play, review and live analysis run end to end and are photographed without Stockfish. It plays and scores by material only, so the evaluations in the screenshots are illustrative.
- **Removed from the old Study screen:** in-place replay and analysis of Lichess games. A game is now copied into the library and opened in Analyze.

## Remaining owner steps

1. Rename the GitHub repository from `chess-cli` to `chesstui`, or adjust the release config as described above.
2. Install Stockfish (`brew install stockfish`). Play one game, review it in Analyze and check the marks look sensible, and try live analysis with space. All of this has only run against the fake test engine.
3. With the Lichess beta on, sign in from **Lichess → Play online** once. Start an AI game, make a move, and resign, to exercise the Board API end to end with a real token.
4. Optionally run `staticcheck ./...`. It was not installed here. Install GoReleaser (`brew install goreleaser`) for future `make snapshot` runs.
5. Set up the Homebrew tap, if you want one.
6. Push the branch, open a PR into `main`, and tag `v2.0.0` after it merges.
7. Deploy `web/` to Vercel. `vercel.json` already points at it.
