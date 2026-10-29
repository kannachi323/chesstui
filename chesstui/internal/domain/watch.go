package domain

type TVChannel struct {
	Key      string
	GameID   string
	Player   Player
	Color    string
	Rating   int
	Position string
}

type Streamer struct {
	ID          string
	Name        string
	Title       string
	Service     string
	Status      string
	Language    string
	Headline    string
	Description string
	URL         string
	ImageURL    string
}

type Broadcast struct {
	ID          string
	Name        string
	Description string
	URL         string
	Live        bool
}
