package puzzles

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kannachi323/chesstui/internal/domain"
)

type memoryStore struct{ puzzles []domain.Puzzle }

func (s *memoryStore) UpsertPuzzles(_ context.Context, puzzles []domain.Puzzle) error {
	s.puzzles = append(s.puzzles, puzzles...)
	return nil
}

func TestImportCSV(t *testing.T) {
	path := filepath.Join(t.TempDir(), "puzzles.csv")
	data := "PuzzleId,FEN,Moves,Rating,RatingDeviation,Popularity,NbPlays,Themes\n" +
		"abc,8/8/8/8/8/8/8/K6k w - - 0 1,a1a2 h1h2,1200,80,100,10,endgame short\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	store := &memoryStore{}
	progress, err := ImportFile(context.Background(), store, path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if progress.Imported != 1 || len(store.puzzles) != 1 {
		t.Fatalf("unexpected import: %#v %#v", progress, store.puzzles)
	}
}
