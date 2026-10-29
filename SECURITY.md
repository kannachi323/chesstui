# Security policy

## Design guarantees

- Authentication uses OAuth Authorization Code with PKCE, a random state value, and a temporary loopback listener bound to `127.0.0.1`.
- Access tokens are stored in the operating-system credential manager. They are not written to logs, config, SQLite, or command arguments.
- Browser launching accepts only `http` and `https` URLs and invokes the platform opener without a shell.
- API errors are bounded before display; streams cap individual NDJSON records at 2 MiB.
- Config writes use a protected temporary file, `fsync`, and atomic replacement.
- SQLite enables foreign keys, WAL, a busy timeout, and parameterized statements.
- Stockfish is executed directly, never through a shell, and every analysis process is cancellable and isolated.
- Engine analysis is unavailable in active online Lichess games.

## Lichess permissions

ChessTUI requests `board:play`, `challenge:read`, `challenge:write`, `study:read`, `puzzle:read`, and `preference:read`. It does not request email, messaging, team administration, or account-management scopes.

Set `CHESSTUI_LICHESS_TOKEN` only for short-lived development sessions; environment variables can be visible to other tooling on the same machine.

## Reporting a vulnerability

Please open a private GitHub security advisory for the repository. Do not include access tokens, private game data, database files, or unredacted logs in a public issue.
