//go:build js

package storage

import (
	"context"
	"testing"

	"github.com/kannachi323/chesstui/internal/domain"
)

func TestMemoryStoreSnapshot(t *testing.T) {
	ctx := context.Background()
	store, _ := Open(ctx, "")
	var saved []byte
	store.OnChange = func(data []byte) { saved = data }
	_ = store.UpsertGame(ctx, domain.Game{ID: "a", Moves: []string{"e2e4"}})
	_ = store.UpsertAnalysis(ctx, domain.Analysis{GameID: "a", Ply: 1, Depth: 12, Score: 30})
	_ = store.UpsertAnalysis(ctx, domain.Analysis{GameID: "a", Ply: 1, Depth: 8, Score: 99})
	_ = store.SaveBook(ctx, Book{ID: "b", Title: "Book", Bookmarks: []int{9, 3}})

	restored, _ := Open(ctx, "")
	restored.Restore(saved)
	games, _ := restored.ListGames(ctx, 10)
	if len(games) != 1 || !games[0].Reviewed {
		t.Fatalf("games %+v", games)
	}
	if pgn, _ := restored.GamePGN(ctx, "a"); pgn != "e2e4 *" {
		t.Fatalf("pgn %q", pgn)
	}
	if evals, _ := restored.GameAnalysis(ctx, "a"); evals[1].Score != 30 {
		t.Fatalf("evals %+v", evals)
	}
	if books, _ := restored.Books(ctx); len(books) != 1 || books[0].Bookmarks[0] != 3 {
		t.Fatalf("books %+v", books)
	}
	restored.Restore([]byte("not json"))
	if games, _ := restored.ListGames(ctx, 10); len(games) != 0 {
		t.Fatal("a damaged snapshot should leave an empty library")
	}
}
