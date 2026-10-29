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

func TestGamesAnalysisAndBooks(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.UpsertGame(ctx, domain.Game{ID: "local-1", Source: "local", White: domain.Player{Name: "You"}, Black: domain.Player{Name: "Stockfish"}, Result: "*", PGN: "1. e4 *"}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertAnalysis(ctx, domain.Analysis{GameID: "local-1", Ply: 1, Depth: 12, Score: 30, PV: []string{"e7e5"}}); err != nil {
		t.Fatal(err)
	}
	_ = store.UpsertAnalysis(ctx, domain.Analysis{GameID: "local-1", Ply: 1, Depth: 16, Score: 25})
	games, err := store.ListGames(ctx, 10)
	if err != nil || len(games) != 1 || !games[0].Reviewed || games[0].Black != "Stockfish" {
		t.Fatalf("games=%+v err=%v", games, err)
	}
	analysis, err := store.GameAnalysis(ctx, "local-1")
	if err != nil || analysis[1].Depth != 16 || analysis[1].Score != 25 {
		t.Fatalf("analysis=%+v err=%v", analysis, err)
	}
	if pgn, err := store.GamePGN(ctx, "local-1"); err != nil || pgn != "1. e4 *" {
		t.Fatalf("pgn=%q err=%v", pgn, err)
	}
	if err := store.DeleteGame(ctx, "local-1"); err != nil {
		t.Fatal(err)
	}
	if left, _ := store.GameAnalysis(ctx, "local-1"); len(left) != 0 {
		t.Fatal("analysis should cascade with the game")
	}

	if err := store.SaveBook(ctx, Book{ID: "builtin:starter", Title: "Starter", Position: 7, Bookmarks: []int{9, 3}}); err != nil {
		t.Fatal(err)
	}
	books, err := store.Books(ctx)
	if err != nil || len(books) != 1 || books[0].Position != 7 || len(books[0].Bookmarks) != 2 || books[0].Bookmarks[0] != 3 {
		t.Fatalf("books=%+v err=%v", books, err)
	}
}
