//go:build js

package storage

import (
	"cmp"
	"context"
	"encoding/json"
	"math/rand/v2"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/kannachi323/chesstui/internal/domain"
)

// Store is the browser build's library. SQLite does not run in WebAssembly,
// so the data lives in memory, and every change is passed to OnChange as a
// JSON snapshot for the page to keep in localStorage.
type Store struct {
	mu       sync.Mutex
	data     snapshot
	OnChange func([]byte)
}

type snapshot struct {
	Games    map[string]storedGame              `json:"games"`
	Analysis map[string]map[int]domain.Analysis `json:"analysis"`
	Books    map[string]Book                    `json:"books"`
	Puzzles  []domain.Puzzle                    `json:"puzzles"`
	Attempts []domain.PuzzleAttempt             `json:"attempts"`
}

type storedGame struct {
	domain.Game
	SavedAt time.Time
}

// Open starts an empty library; path is ignored. Restore loads a snapshot.
func Open(context.Context, string) (*Store, error) {
	s := &Store{}
	s.reset()
	return s, nil
}

func (s *Store) reset() {
	s.data = snapshot{
		Games:    map[string]storedGame{},
		Analysis: map[string]map[int]domain.Analysis{},
		Books:    map[string]Book{},
	}
}

// Restore replaces the library with a snapshot from OnChange. A damaged
// snapshot leaves the library empty rather than failing to start.
func (s *Store) Restore(data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reset()
	var loaded snapshot
	if json.Unmarshal(data, &loaded) != nil {
		return
	}
	if loaded.Games != nil {
		s.data.Games = loaded.Games
	}
	if loaded.Analysis != nil {
		s.data.Analysis = loaded.Analysis
	}
	if loaded.Books != nil {
		s.data.Books = loaded.Books
	}
	s.data.Puzzles, s.data.Attempts = loaded.Puzzles, loaded.Attempts
}

// changed runs with s.mu held.
func (s *Store) changed() {
	if s.OnChange == nil {
		return
	}
	if data, err := json.Marshal(s.data); err == nil {
		s.OnChange(data)
	}
}

func (s *Store) Close() error                 { return nil }
func (s *Store) Health(context.Context) error { return nil }

func (s *Store) UpsertGame(_ context.Context, game domain.Game) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Games[game.ID] = storedGame{Game: game, SavedAt: time.Now().UTC()}
	s.changed()
	return nil
}

func (s *Store) UpsertAnalysis(_ context.Context, analysis domain.Analysis) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	plies := s.data.Analysis[analysis.GameID]
	if plies == nil {
		plies = map[int]domain.Analysis{}
		s.data.Analysis[analysis.GameID] = plies
	}
	// Keep the deepest evaluation per ply, as the SQLite store returns.
	if old, ok := plies[analysis.Ply]; !ok || analysis.Depth >= old.Depth {
		plies[analysis.Ply] = analysis
	}
	s.changed()
	return nil
}

// ListGames returns the newest games first.
func (s *Store) ListGames(_ context.Context, limit int) ([]GameSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	games := make([]GameSummary, 0, len(s.data.Games))
	for _, game := range s.data.Games {
		games = append(games, GameSummary{
			ID: game.ID, Source: game.Source, White: game.White.Name, Black: game.Black.Name,
			Result: game.Result, PlayedAt: game.PlayedAt, SavedAt: game.SavedAt,
			Reviewed: len(s.data.Analysis[game.ID]) > 0,
		})
	}
	when := func(game GameSummary) time.Time {
		if game.PlayedAt.IsZero() {
			return game.SavedAt
		}
		return game.PlayedAt
	}
	slices.SortFunc(games, func(a, b GameSummary) int { return when(b).Compare(when(a)) })
	if limit > 0 && len(games) > limit {
		games = games[:limit]
	}
	return games, nil
}

// GamePGN returns a stored game's PGN, or ErrNotFound.
func (s *Store) GamePGN(_ context.Context, id string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	game, ok := s.data.Games[id]
	if !ok {
		return "", ErrNotFound
	}
	if strings.TrimSpace(game.PGN) == "" {
		return strings.Join(game.Moves, " ") + " *", nil
	}
	return game.PGN, nil
}

// DeleteGame removes a game and its analysis.
func (s *Store) DeleteGame(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data.Games, id)
	delete(s.data.Analysis, id)
	s.changed()
	return nil
}

// GameAnalysis returns the deepest stored evaluation for each ply.
func (s *Store) GameAnalysis(_ context.Context, id string) (map[int]domain.Analysis, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := map[int]domain.Analysis{}
	for ply, eval := range s.data.Analysis[id] {
		result[ply] = eval
	}
	return result, nil
}

func (s *Store) ClearAnalysis(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data.Analysis, id)
	s.changed()
	return nil
}

// UpdateGamePGN stores an edited game's PGN and main-line moves.
func (s *Store) UpdateGamePGN(_ context.Context, id, pgn string, moves []string, result string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	game, ok := s.data.Games[id]
	if !ok {
		return nil // UPDATE of a missing row is a no-op in SQLite too
	}
	game.PGN, game.Moves, game.Result, game.SavedAt = pgn, moves, result, time.Now().UTC()
	s.data.Games[id] = game
	s.changed()
	return nil
}

func (s *Store) SaveBook(_ context.Context, book Book) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	book.Bookmarks = slices.Sorted(slices.Values(book.Bookmarks))
	book.OpenedAt = time.Now().UTC()
	s.data.Books[book.ID] = book
	s.changed()
	return nil
}

// Books returns the library, most recently opened first.
func (s *Store) Books(context.Context) ([]Book, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	books := make([]Book, 0, len(s.data.Books))
	for _, book := range s.data.Books {
		books = append(books, book)
	}
	slices.SortFunc(books, func(a, b Book) int { return b.OpenedAt.Compare(a.OpenedAt) })
	return books, nil
}

func (s *Store) DeleteBook(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data.Books, id)
	s.changed()
	return nil
}

func (s *Store) UpsertPuzzle(ctx context.Context, puzzle domain.Puzzle) error {
	return s.UpsertPuzzles(ctx, []domain.Puzzle{puzzle})
}

func (s *Store) UpsertPuzzles(_ context.Context, puzzles []domain.Puzzle) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, puzzle := range puzzles {
		if i := s.puzzleIndex(puzzle.ID); i >= 0 {
			s.data.Puzzles[i] = puzzle
		} else {
			s.data.Puzzles = append(s.data.Puzzles, puzzle)
		}
	}
	s.changed()
	return nil
}

func (s *Store) puzzleIndex(id string) int {
	return slices.IndexFunc(s.data.Puzzles, func(p domain.Puzzle) bool { return p.ID == id })
}

func (s *Store) Puzzle(_ context.Context, id string) (domain.Puzzle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i := s.puzzleIndex(id); i >= 0 {
		return s.data.Puzzles[i], nil
	}
	return domain.Puzzle{}, ErrNotFound
}

func (s *Store) RandomPuzzle(_ context.Context, minRating, maxRating int) (domain.Puzzle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var matches []domain.Puzzle
	for _, puzzle := range s.data.Puzzles {
		if puzzle.Rating >= minRating && puzzle.Rating <= maxRating {
			matches = append(matches, puzzle)
		}
	}
	if len(matches) == 0 {
		return domain.Puzzle{}, ErrNotFound
	}
	return matches[rand.IntN(len(matches))], nil
}

func (s *Store) PuzzleCount(context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.data.Puzzles), nil
}

func (s *Store) RecordPuzzleAttempt(_ context.Context, attempt domain.PuzzleAttempt) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	attempt.Attempted = cmp.Or(attempt.Attempted, time.Now())
	s.data.Attempts = append(s.data.Attempts, attempt)
	s.changed()
	return nil
}

func (s *Store) PuzzleStats(context.Context) (domain.PuzzleStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var stats domain.PuzzleStats
	current := 0
	for _, attempt := range s.data.Attempts {
		stats.Attempted++
		if attempt.Solved {
			stats.Solved++
			current++
			stats.Best = max(stats.Best, current)
		} else {
			current = 0
		}
	}
	stats.Streak = current
	return stats, nil
}
