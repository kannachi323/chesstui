package storage

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/kannachi323/chesstui/internal/domain"
)

func TestPuzzleRoundTrip(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	want := domain.Puzzle{
		ID: "abc", FEN: "8/8/8/8/8/8/8/K6k w - - 0 1",
		Moves: []string{"a1a2"}, Rating: 1200, Themes: []string{"endgame"}, Source: "test",
	}
	if err := store.UpsertPuzzle(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err := store.Puzzle(ctx, want.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != want.ID || got.Rating != want.Rating || len(got.Moves) != 1 {
		t.Fatalf("unexpected puzzle: %#v", got)
	}
}

func TestGameAndAnalysisPersistence(t *testing.T) {
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	game := domain.Game{ID: "g1", Source: "lichess", Variant: "standard", Moves: []string{"e2e4", "e7e5"}, White: domain.Player{Name: "Alice", Rating: 1800}, Black: domain.Player{Name: "Bob", Rating: 1790}, Result: "mate"}
	if err := store.UpsertGame(context.Background(), game); err != nil {
		t.Fatal(err)
	}
	analysis := domain.Analysis{GameID: "g1", Ply: 2, Depth: 16, Score: 23, PV: []string{"g1f3", "b8c6"}}
	if err := store.UpsertAnalysis(context.Background(), analysis); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := store.db.QueryRow("SELECT COUNT(*) FROM analysis WHERE game_id = ?", game.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("analysis count = %d", count)
	}
}
