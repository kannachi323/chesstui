package lichess

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

func (c *Client) ChallengeAI(ctx context.Context, level int, options ChallengeOptions) (EventGame, error) {
	form := challengeForm(options)
	form.Set("level", strconv.Itoa(level))
	var game EventGame
	err := c.doJSON(ctx, http.MethodPost, "/api/challenge/ai", form, &game)
	return game, err
}

func (c *Client) ChallengeUser(ctx context.Context, username string, options ChallengeOptions) (Challenge, error) {
	var response struct {
		Challenge Challenge `json:"challenge"`
	}
	err := c.doJSON(ctx, http.MethodPost, "/api/challenge/"+url.PathEscape(username), challengeForm(options), &response)
	return response.Challenge, err
}

func (c *Client) StreamEvents(ctx context.Context, handle func(Event) error) error {
	return c.streamNDJSON(ctx, "/api/stream/event", func(raw json.RawMessage) error {
		var event Event
		if err := json.Unmarshal(raw, &event); err != nil {
			return fmt.Errorf("decode event stream: %w", err)
		}
		return handle(event)
	})
}

func (c *Client) StreamGame(ctx context.Context, gameID string, handle func(BoardEvent) error) error {
	path := "/api/board/game/stream/" + url.PathEscape(gameID)
	return c.streamNDJSON(ctx, path, func(raw json.RawMessage) error {
		var event BoardEvent
		if err := json.Unmarshal(raw, &event); err != nil {
			return fmt.Errorf("decode board stream: %w", err)
		}
		return handle(event)
	})
}

func (c *Client) Move(ctx context.Context, gameID, move string, offerDraw bool) error {
	path := fmt.Sprintf("/api/board/game/%s/move/%s", url.PathEscape(gameID), url.PathEscape(move))
	if offerDraw {
		path += "?offeringDraw=true"
	}
	return c.doJSON(ctx, http.MethodPost, path, url.Values{}, nil)
}

func (c *Client) Abort(ctx context.Context, gameID string) error {
	return c.gameAction(ctx, gameID, "abort")
}

func (c *Client) Resign(ctx context.Context, gameID string) error {
	return c.gameAction(ctx, gameID, "resign")
}

func (c *Client) Draw(ctx context.Context, gameID string, accept bool) error {
	value := "no"
	if accept {
		value = "yes"
	}
	return c.gameAction(ctx, gameID, "draw/"+value)
}

func (c *Client) gameAction(ctx context.Context, gameID, action string) error {
	path := "/api/board/game/" + url.PathEscape(gameID) + "/" + action
	return c.doJSON(ctx, http.MethodPost, path, url.Values{}, nil)
}

func challengeForm(options ChallengeOptions) url.Values {
	form := url.Values{}
	form.Set("rated", strconv.FormatBool(options.Rated))
	if options.Limit > 0 {
		form.Set("clock.limit", strconv.Itoa(options.Limit))
		form.Set("clock.increment", strconv.Itoa(options.Increment))
	} else if options.Days > 0 {
		form.Set("days", strconv.Itoa(options.Days))
	}
	if options.Color != "" {
		form.Set("color", options.Color)
	}
	if options.Variant != "" {
		form.Set("variant", options.Variant)
	}
	if options.FEN != "" {
		form.Set("fen", options.FEN)
	}
	return form
}
