# ChessTUI architecture

ChessTUI is a single local process. Bubble Tea owns UI state and dispatches typed messages; screens depend on a shared session containing configuration, the Lichess client, the credential abstraction, and SQLite storage.

```mermaid
flowchart LR
    UI["Bubble Tea app shell"] --> Screens["Independent screen models"]
    Screens --> Session["Shared session"]
    Session --> API["Typed Lichess client"]
    Session --> DB["SQLite repository"]
    Session --> Keyring["OS credential store"]
    Screens --> Rules["Chess rules adapter"]
    Screens --> UCI["Stockfish UCI adapter"]
    API --> Lichess["lichess.org"]
    UCI --> Engine["Local Stockfish process"]
```

## Package boundaries

- `cmd/chesstui`: composition root, logging, process lifecycle.
- `internal/app`: global navigation, window layout, asynchronous message routing.
- `internal/ui/screens`: small, stateful Bubble Tea screen models.
- `internal/ui/widgets` and `theme`: reusable rendering with no network or persistence work.
- `internal/lichess`: endpoint-specific typed API methods, OAuth PKCE, NDJSON streams, rate-limit handling.
- `internal/chess`: a narrow rules/timeline adapter around `chess-on-go` so the library can be replaced without rewriting screens.
- `internal/engine`: an `Analyzer` interface and isolated UCI Stockfish processes.
- `internal/storage`: SQLite schema and repositories for puzzles, attempts, games, and analysis.
- `internal/puzzles`: bounded-memory CSV/Zstandard importer.
- `internal/auth`, `config`, `platform`, `session`: infrastructure and application services.

## Concurrency model

Bubble Tea remains the sole writer of screen state. Network and engine operations run as commands and return typed messages. Lichess streams use bounded channels and cancellable contexts. The API client serializes ordinary requests to respect Lichess rate guidance while streaming endpoints use independent connections.

SQLite uses WAL mode, foreign keys, a busy timeout, and one writer connection. Puzzle imports commit batches of 1,000 records so multi-million-row packs remain memory-bounded and recoverable.

## Online game boundary

Online positions arrive only from the Lichess Board API. Submitted moves are locally validated, posted to Lichess, and not committed to UI state until Lichess streams the authoritative game state. The Stockfish dependency is only reachable from local play and completed-game study paths.
