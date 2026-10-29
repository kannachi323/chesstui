package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kannachi323/chesstui/internal/domain"
)

func (s *Store) UpsertGame(ctx context.Context, game domain.Game) error {
	var playedAt any
	if !game.PlayedAt.IsZero() {
		playedAt = game.PlayedAt.UTC().Format(time.RFC3339)
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO games(id, source, variant, initial_fen, moves, white_name, black_name,
                  white_rating, black_rating, result, pgn, played_at, saved_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    source=excluded.source, variant=excluded.variant, initial_fen=excluded.initial_fen,
    moves=excluded.moves, white_name=excluded.white_name, black_name=excluded.black_name,
    white_rating=excluded.white_rating, black_rating=excluded.black_rating,
    result=excluded.result, pgn=excluded.pgn, played_at=excluded.played_at,
    saved_at=excluded.saved_at`,
		game.ID, game.Source, game.Variant, game.InitialFEN, strings.Join(game.Moves, " "),
		game.White.Name, game.Black.Name, game.White.Rating, game.Black.Rating,
		game.Result, game.PGN, playedAt, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("save game %s: %w", game.ID, err)
	}
	return nil
}

func (s *Store) UpsertAnalysis(ctx context.Context, analysis domain.Analysis) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO analysis(game_id, ply, depth, score_cp, mate, pv, created_at)
VALUES(?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(game_id, ply, depth) DO UPDATE SET
    score_cp=excluded.score_cp, mate=excluded.mate, pv=excluded.pv,
    created_at=excluded.created_at`, analysis.GameID, analysis.Ply, analysis.Depth,
		analysis.Score, analysis.Mate, strings.Join(analysis.PV, " "), time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("save analysis for %s: %w", analysis.GameID, err)
	}
	return nil
}

// GameSummary is a row in the game library.
type GameSummary struct {
	ID       string
	Source   string
	White    string
	Black    string
	Result   string
	PlayedAt time.Time
	SavedAt  time.Time
	Reviewed bool
}

// ListGames returns the newest games first.
func (s *Store) ListGames(ctx context.Context, limit int) ([]GameSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT g.id, g.source, g.white_name, g.black_name, g.result, COALESCE(g.played_at, ''), g.saved_at,
       EXISTS(SELECT 1 FROM analysis a WHERE a.game_id = g.id)
FROM games g ORDER BY COALESCE(g.played_at, g.saved_at) DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list games: %w", err)
	}
	defer rows.Close()
	var games []GameSummary
	for rows.Next() {
		var game GameSummary
		var played, saved string
		if err := rows.Scan(&game.ID, &game.Source, &game.White, &game.Black, &game.Result, &played, &saved, &game.Reviewed); err != nil {
			return nil, fmt.Errorf("scan game: %w", err)
		}
		game.PlayedAt, _ = time.Parse(time.RFC3339, played)
		game.SavedAt, _ = time.Parse(time.RFC3339, saved)
		games = append(games, game)
	}
	return games, rows.Err()
}

// GamePGN returns a stored game's PGN, or ErrNotFound.
func (s *Store) GamePGN(ctx context.Context, id string) (string, error) {
	var pgn, moves string
	err := s.db.QueryRowContext(ctx, "SELECT pgn, moves FROM games WHERE id = ?", id).Scan(&pgn, &moves)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("load game %s: %w", id, err)
	}
	if strings.TrimSpace(pgn) == "" {
		pgn = moves + " *"
	}
	return pgn, nil
}

// DeleteGame removes a game and its analysis.
func (s *Store) DeleteGame(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM games WHERE id = ?", id); err != nil {
		return fmt.Errorf("delete game %s: %w", id, err)
	}
	return nil
}

// GameAnalysis returns the deepest stored evaluation for each ply.
func (s *Store) GameAnalysis(ctx context.Context, id string) (map[int]domain.Analysis, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT ply, depth, score_cp, mate, pv FROM analysis a
WHERE game_id = ? AND depth = (SELECT MAX(depth) FROM analysis b WHERE b.game_id = a.game_id AND b.ply = a.ply)`, id)
	if err != nil {
		return nil, fmt.Errorf("load analysis: %w", err)
	}
	defer rows.Close()
	result := map[int]domain.Analysis{}
	for rows.Next() {
		var item domain.Analysis
		var pv string
		if err := rows.Scan(&item.Ply, &item.Depth, &item.Score, &item.Mate, &pv); err != nil {
			return nil, fmt.Errorf("scan analysis: %w", err)
		}
		item.GameID, item.PV = id, strings.Fields(pv)
		result[item.Ply] = item
	}
	return result, rows.Err()
}

// ClearAnalysis forgets a game's stored evaluations, for example after its
// moves were edited.
func (s *Store) ClearAnalysis(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM analysis WHERE game_id = ?", id)
	return err
}

// UpdateGamePGN stores an edited game's PGN and main-line moves.
func (s *Store) UpdateGamePGN(ctx context.Context, id, pgn string, moves []string, result string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE games SET pgn = ?, moves = ?, result = ?, saved_at = ? WHERE id = ?",
		pgn, strings.Join(moves, " "), result, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("save game %s: %w", id, err)
	}
	return nil
}
