package storage

import (
	"context"
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
