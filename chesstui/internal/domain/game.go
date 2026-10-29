package domain

import "time"

type Player struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Title  string `json:"title,omitempty"`
	Rating int    `json:"rating"`
	Color  string `json:"color"`
	Bot    bool   `json:"bot,omitempty"`
}

type Clock struct {
	White     time.Duration
	Black     time.Duration
	Increment time.Duration
}

type Game struct {
	ID         string
	Source     string
	Variant    string
	Rated      bool
	InitialFEN string
	Moves      []string
	White      Player
	Black      Player
	Clock      Clock
	Status     string
	Winner     string
	Result     string
	PGN        string
	PlayedAt   time.Time
}

type Analysis struct {
	GameID string
	Ply    int
	Depth  int
	Score  int
	Mate   int
	PV     []string
}
