# Security policy

## Design guarantees

- Authentication uses OAuth Authorization Code with PKCE, a random state value, and a temporary loopback listener bound to `127.0.0.1`.
- Access tokens are stored in the operating-system credential manager. Where none exists (for example a headless Linux server), the token falls back to a `0600` file inside a `0700` data directory, and looser permissions are tightened before use. Tokens are never written to logs, config, SQLite, or command arguments.
- Signing out revokes the token on Lichess (`DELETE /api/token`) and removes every local copy.
- The loopback callback ignores requests without the expected `state`, so a stray local request cannot end or hijack a sign-in. The code exchange never carries an existing bearer token.
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
