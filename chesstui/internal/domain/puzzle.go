package domain

import "time"

type Puzzle struct {
	ID         string
	FEN        string
	Moves      []string
	Rating     int
	Plays      int
	Themes     []string
	Source     string
	GameID     string
	GamePGN    string
	InitialPly int
	LastMove   string
}

type PuzzleAttempt struct {
	PuzzleID  string
	Solved    bool
	Hints     int
	Elapsed   time.Duration
	Attempted time.Time
}

type PuzzleStats struct {
	Attempted int
	Solved    int
	Streak    int
	Best      int
}
