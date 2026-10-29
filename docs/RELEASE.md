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

## Remaining owner steps

1. Rename the GitHub repository from `chess-cli` to `chesstui`, or adjust the release config as described above.
2. Install Stockfish (`brew install stockfish`). Play one local game and analyse one replayed position to confirm the real engine works.
3. Sign in to Lichess from **Play** once. Start an AI game, make a move, and resign, to exercise the Board API end to end with a real token.
4. Optionally run `staticcheck ./...`. It was not installed here.
5. Set up the Homebrew tap, if you want one.
6. Push the branch, open a PR into `main`, and tag `v2.0.0` after it merges.
7. Deploy `web/` to Vercel. `vercel.json` already points at it.
