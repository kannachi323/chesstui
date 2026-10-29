# Roadmap: local first

chesstui is a terminal chess desk for working on your own games. You play a local engine, review the games afterwards, and read chess books in a terminal split while you work. Lichess features stay in the app, grouped in one tab marked beta.

## Principles

- **Bring your own engine.** chesstui runs no AI of its own. It talks UCI to any engine the user already has (Stockfish, Lc0, Komodo). Each turn it sends the starting position and the full move list (`position startpos moves …`), so the engine sees the whole game, repetitions included.
- **Local data.** Games, analysis, books and reading progress live in the local SQLite library. Games move in and out as PGN.
- **Lichess is optional.** TV, online play, the daily puzzle and game import sit in one *Lichess (beta)* tab, and a setting hides it. Nothing else depends on the network.

## Phases

| Phase | Scope | Status |
|---|---|---|
| 1 | Engine setup: detection, path, threads, hash, persistent UCI session. Local games saved as PGN after every move, and resumable. Navigation reshaped, with Lichess grouped into a beta tab. | Done |
| 2 | PGN reader and writer that keep comments, side lines and annotation marks (`$n`, `!?`). Reads multi-game files and writes the tree back. | Done |
| 3 | Analyze: game library, PGN open and paste, engine review with an evaluation graph, move marks and accuracy, the engine's top lines, side lines and comments, PGN export. | Done |
| 4 | Read: annotated PGN and Markdown books, a board that follows the text, narrow and quiet modes, bookmarks, an included starter book. | Done |
| 5 | README, landing page and screenshots rewritten around local play. | Done |

## Later

- Remote engines behind an "engine URL" setting, off by default.
- Lichess cloud eval as an analysis fallback in the beta tab.
- An EPUB text import for the reader. PDFs stay out of scope, because terminals cannot show their diagrams.
