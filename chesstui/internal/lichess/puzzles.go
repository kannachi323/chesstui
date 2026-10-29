package lichess

import (
	"context"
	"net/http"

	"github.com/kannachi323/chesstui/internal/domain"
)

type puzzleResponse struct {
	Game struct {
		ID      string `json:"id"`
		PGN     string `json:"pgn"`
		Clock   string `json:"clock"`
		Rated   bool   `json:"rated"`
		Players []struct {
			Name   string `json:"name"`
			ID     string `json:"id"`
			Color  string `json:"color"`
			Rating int    `json:"rating"`
		} `json:"players"`
	} `json:"game"`
	Puzzle struct {
		ID         string   `json:"id"`
		Rating     int      `json:"rating"`
		Plays      int      `json:"plays"`
		Solution   []string `json:"solution"`
		Themes     []string `json:"themes"`
		FEN        string   `json:"fen"`
		LastMove   string   `json:"lastMove"`
		InitialPly int      `json:"initialPly"`
	} `json:"puzzle"`
}

func (c *Client) DailyPuzzle(ctx context.Context) (domain.Puzzle, error) {
	return c.fetchPuzzle(ctx, "/api/puzzle/daily")
}

func (c *Client) NextPuzzle(ctx context.Context) (domain.Puzzle, error) {
	return c.fetchPuzzle(ctx, "/api/puzzle/next")
}

func (c *Client) Puzzle(ctx context.Context, id string) (domain.Puzzle, error) {
	return c.fetchPuzzle(ctx, "/api/puzzle/"+id)
}

func (c *Client) fetchPuzzle(ctx context.Context, path string) (domain.Puzzle, error) {
	var response puzzleResponse
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return domain.Puzzle{}, err
	}
	return domain.Puzzle{
		ID:         response.Puzzle.ID,
		FEN:        response.Puzzle.FEN,
		Moves:      response.Puzzle.Solution,
		Rating:     response.Puzzle.Rating,
		Plays:      response.Puzzle.Plays,
		Themes:     response.Puzzle.Themes,
		Source:     "lichess",
		GameID:     response.Game.ID,
		GamePGN:    response.Game.PGN,
		InitialPly: response.Puzzle.InitialPly,
		LastMove:   response.Puzzle.LastMove,
	}, nil
}
