package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(ctx context.Context, path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	db.SetMaxOpenConns(1)
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=NORMAL",
	} {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("configure sqlite: %w", err)
		}
	}
	store := &Store{db: db}
	if err := store.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Health(ctx context.Context) error {
	if err := s.db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping sqlite database: %w", err)
	}
	return nil
}

func (s *Store) migrate(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS puzzles (
    id TEXT PRIMARY KEY,
    fen TEXT NOT NULL,
    moves TEXT NOT NULL,
    rating INTEGER NOT NULL DEFAULT 0,
    plays INTEGER NOT NULL DEFAULT 0,
    themes TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL,
    game_id TEXT NOT NULL DEFAULT '',
    game_pgn TEXT NOT NULL DEFAULT '',
    initial_ply INTEGER NOT NULL DEFAULT 0,
    last_move TEXT NOT NULL DEFAULT '',
    imported_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS puzzles_rating_idx ON puzzles(rating);
CREATE INDEX IF NOT EXISTS puzzles_source_idx ON puzzles(source);
CREATE TABLE IF NOT EXISTS puzzle_attempts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    puzzle_id TEXT NOT NULL,
    solved INTEGER NOT NULL,
    hints INTEGER NOT NULL DEFAULT 0,
    elapsed_ms INTEGER NOT NULL DEFAULT 0,
    attempted_at TEXT NOT NULL,
    FOREIGN KEY(puzzle_id) REFERENCES puzzles(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS puzzle_attempts_puzzle_idx ON puzzle_attempts(puzzle_id);
CREATE INDEX IF NOT EXISTS puzzle_attempts_time_idx ON puzzle_attempts(attempted_at);
CREATE TABLE IF NOT EXISTS daily_puzzles (
    day TEXT PRIMARY KEY,
    puzzle_id TEXT NOT NULL,
    solved INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY(puzzle_id) REFERENCES puzzles(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS games (
    id TEXT PRIMARY KEY,
    source TEXT NOT NULL,
    variant TEXT NOT NULL DEFAULT 'standard',
    initial_fen TEXT NOT NULL DEFAULT '',
    moves TEXT NOT NULL DEFAULT '',
    white_name TEXT NOT NULL DEFAULT '',
    black_name TEXT NOT NULL DEFAULT '',
    white_rating INTEGER NOT NULL DEFAULT 0,
    black_rating INTEGER NOT NULL DEFAULT 0,
    result TEXT NOT NULL DEFAULT '',
    pgn TEXT NOT NULL DEFAULT '',
    played_at TEXT,
    saved_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS analysis (
    game_id TEXT NOT NULL,
    ply INTEGER NOT NULL,
    depth INTEGER NOT NULL,
    score_cp INTEGER NOT NULL DEFAULT 0,
    mate INTEGER NOT NULL DEFAULT 0,
    pv TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    PRIMARY KEY(game_id, ply, depth),
    FOREIGN KEY(game_id) REFERENCES games(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS books (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    path TEXT NOT NULL DEFAULT '',
    position INTEGER NOT NULL DEFAULT 0,
    bookmarks TEXT NOT NULL DEFAULT '',
    opened_at TEXT NOT NULL
);`
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("apply database schema: %w", err)
	}
	_, err := s.db.ExecContext(ctx,
		"INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES(1, ?)",
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("record schema migration: %w", err)
	}
	return nil
}
