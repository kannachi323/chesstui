// Package storage keeps the local library: games and their analysis, books
// and the reader's place, and puzzles. Native builds use SQLite; the
// browser build keeps the same data in memory and hands snapshots to the
// page to save.
package storage

import (
	"errors"
	"time"
)

var ErrNotFound = errors.New("record not found")

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

// Book is a library entry and the reader's place in it.
type Book struct {
	ID        string
	Title     string
	Path      string // empty for books built into chesstui
	Position  int    // index of the move or block the reader was on
	Bookmarks []int
	OpenedAt  time.Time
}
