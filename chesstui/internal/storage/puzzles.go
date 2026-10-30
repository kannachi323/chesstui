//go:build !js

package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/kannachi323/chesstui/internal/domain"
)

const upsertPuzzleSQL = `
INSERT INTO puzzles(id, fen, moves, rating, plays, themes, source, game_id, game_pgn,
                    initial_ply, last_move, imported_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    fen=excluded.fen, moves=excluded.moves, rating=excluded.rating,
    plays=excluded.plays, themes=excluded.themes, source=excluded.source,
    game_id=excluded.game_id, game_pgn=excluded.game_pgn,
    initial_ply=excluded.initial_ply, last_move=excluded.last_move`

func (s *Store) UpsertPuzzle(ctx context.Context, puzzle domain.Puzzle) error {
	_, err := s.db.ExecContext(ctx, upsertPuzzleSQL,
		puzzle.ID, puzzle.FEN, strings.Join(puzzle.Moves, " "), puzzle.Rating,
		puzzle.Plays, strings.Join(puzzle.Themes, " "), puzzle.Source,
		puzzle.GameID, puzzle.GamePGN, puzzle.InitialPly, puzzle.LastMove,
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("upsert puzzle %s: %w", puzzle.ID, err)
	}
	return nil
}

func (s *Store) UpsertPuzzles(ctx context.Context, puzzles []domain.Puzzle) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin puzzle import: %w", err)
	}
	defer tx.Rollback()
	statement, err := tx.PrepareContext(ctx, upsertPuzzleSQL)
	if err != nil {
		return fmt.Errorf("prepare puzzle import: %w", err)
	}
	defer statement.Close()
	now := time.Now().UTC().Format(time.RFC3339)
	for _, puzzle := range puzzles {
		if _, err := statement.ExecContext(ctx,
			puzzle.ID, puzzle.FEN, strings.Join(puzzle.Moves, " "), puzzle.Rating,
			puzzle.Plays, strings.Join(puzzle.Themes, " "), puzzle.Source,
			puzzle.GameID, puzzle.GamePGN, puzzle.InitialPly, puzzle.LastMove, now,
		); err != nil {
			return fmt.Errorf("import puzzle %s: %w", puzzle.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit puzzle import: %w", err)
	}
	return nil
}

func (s *Store) Puzzle(ctx context.Context, id string) (domain.Puzzle, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, fen, moves, rating, plays, themes, source, game_id, game_pgn, initial_ply, last_move
FROM puzzles WHERE id = ?`, id)
	return scanPuzzle(row)
}

func (s *Store) RandomPuzzle(ctx context.Context, minRating, maxRating int) (domain.Puzzle, error) {
	// ORDER BY RANDOM() scans every row, which takes seconds on a full
	// Lichess pack. Seek to a random rowid instead and wrap around once.
	const query = `
SELECT id, fen, moves, rating, plays, themes, source, game_id, game_pgn, initial_ply, last_move
FROM puzzles WHERE rowid >= ? AND rating BETWEEN ? AND ? ORDER BY rowid LIMIT 1`
	var maxRow sql.NullInt64
	if err := s.db.QueryRowContext(ctx, "SELECT MAX(rowid) FROM puzzles").Scan(&maxRow); err != nil {
		return domain.Puzzle{}, fmt.Errorf("find puzzle range: %w", err)
	}
	if !maxRow.Valid {
		return domain.Puzzle{}, ErrNotFound
	}
	start := rand.Int64N(maxRow.Int64) + 1
	puzzle, err := scanPuzzle(s.db.QueryRowContext(ctx, query, start, minRating, maxRating))
	if errors.Is(err, ErrNotFound) {
		puzzle, err = scanPuzzle(s.db.QueryRowContext(ctx, query, 0, minRating, maxRating))
	}
	return puzzle, err
}

func (s *Store) PuzzleCount(ctx context.Context) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM puzzles").Scan(&count); err != nil {
		return 0, fmt.Errorf("count puzzles: %w", err)
	}
	return count, nil
}

func (s *Store) RecordPuzzleAttempt(ctx context.Context, attempt domain.PuzzleAttempt) error {
	when := attempt.Attempted
	if when.IsZero() {
		when = time.Now()
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO puzzle_attempts(puzzle_id, solved, hints, elapsed_ms, attempted_at)
VALUES(?, ?, ?, ?, ?)`, attempt.PuzzleID, attempt.Solved, attempt.Hints,
		attempt.Elapsed.Milliseconds(), when.UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("record puzzle attempt: %w", err)
	}
	return nil
}

func (s *Store) PuzzleStats(ctx context.Context) (domain.PuzzleStats, error) {
	var stats domain.PuzzleStats
	err := s.db.QueryRowContext(ctx, `
SELECT COUNT(*), COALESCE(SUM(solved), 0) FROM puzzle_attempts`).Scan(&stats.Attempted, &stats.Solved)
	if err != nil {
		return domain.PuzzleStats{}, fmt.Errorf("load puzzle stats: %w", err)
	}
	// Streaks are calculated in Go to keep the schema portable and the definition explicit.
	rows, err := s.db.QueryContext(ctx, "SELECT solved FROM puzzle_attempts ORDER BY id")
	if err != nil {
		return domain.PuzzleStats{}, fmt.Errorf("load puzzle attempts: %w", err)
	}
	defer rows.Close()
	current := 0
	for rows.Next() {
		var solved bool
		if err := rows.Scan(&solved); err != nil {
			return domain.PuzzleStats{}, err
		}
		if solved {
			current++
			if current > stats.Best {
				stats.Best = current
			}
		} else {
			current = 0
		}
	}
	stats.Streak = current
	return stats, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanPuzzle(row rowScanner) (domain.Puzzle, error) {
	var puzzle domain.Puzzle
	var moves, themes string
	err := row.Scan(&puzzle.ID, &puzzle.FEN, &moves, &puzzle.Rating, &puzzle.Plays,
		&themes, &puzzle.Source, &puzzle.GameID, &puzzle.GamePGN,
		&puzzle.InitialPly, &puzzle.LastMove)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Puzzle{}, ErrNotFound
	}
	if err != nil {
		return domain.Puzzle{}, fmt.Errorf("scan puzzle: %w", err)
	}
	puzzle.Moves = strings.Fields(moves)
	puzzle.Themes = strings.Fields(themes)
	return puzzle, nil
}
