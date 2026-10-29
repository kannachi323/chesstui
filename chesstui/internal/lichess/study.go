package lichess

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

type Study struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

type ExportedGame struct {
	ID        string `json:"id"`
	Rated     bool   `json:"rated"`
	Variant   string `json:"variant"`
	Speed     string `json:"speed"`
	CreatedAt int64  `json:"createdAt"`
	Status    string `json:"status"`
	Winner    string `json:"winner"`
	Moves     string `json:"moves"`
	PGN       string `json:"pgn"`
	Players   struct {
		White GamePlayer `json:"white"`
		Black GamePlayer `json:"black"`
	} `json:"players"`
}

type GamePlayer struct {
	User   LightUser `json:"user"`
	Rating int       `json:"rating"`
}

func (c *Client) Studies(ctx context.Context, username string, handle func(Study) error) error {
	path := "/api/study/by/" + url.PathEscape(username)
	return c.streamNDJSON(ctx, path, func(raw json.RawMessage) error {
		var study Study
		if err := json.Unmarshal(raw, &study); err != nil {
			return fmt.Errorf("decode study metadata: %w", err)
		}
		return handle(study)
	})
}

func (c *Client) ExportStudyPGN(ctx context.Context, studyID string) (string, error) {
	return c.doText(ctx, "GET", "/api/study/"+url.PathEscape(studyID)+".pgn", "application/x-chess-pgn")
}

func (c *Client) RecentGames(ctx context.Context, username string, maximum int, handle func(ExportedGame) error) error {
	query := url.Values{}
	query.Set("max", fmt.Sprintf("%d", maximum))
	query.Set("pgnInJson", "true")
	query.Set("clocks", "true")
	query.Set("opening", "true")
	path := "/api/games/user/" + url.PathEscape(username) + "?" + query.Encode()
	return c.streamNDJSON(ctx, path, func(raw json.RawMessage) error {
		var game ExportedGame
		if err := json.Unmarshal(raw, &game); err != nil {
			return fmt.Errorf("decode exported game: %w", err)
		}
		return handle(game)
	})
}
