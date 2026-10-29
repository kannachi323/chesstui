package lichess

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"

	"github.com/kannachi323/chesstui/internal/domain"
)

type tvGame struct {
	User   LightUser `json:"user"`
	Rating int       `json:"rating"`
	GameID string    `json:"gameId"`
	Color  string    `json:"color"`
}

type TVEvent struct {
	Type string `json:"t"`
	Data struct {
		ID          string `json:"id"`
		Orientation string `json:"orientation"`
		FEN         string `json:"fen"`
		LastMove    string `json:"lm"`
		WhiteClock  int    `json:"wc"`
		BlackClock  int    `json:"bc"`
		Players     []struct {
			Color   string    `json:"color"`
			User    LightUser `json:"user"`
			Rating  int       `json:"rating"`
			Seconds int       `json:"seconds"`
		} `json:"players"`
	} `json:"d"`
}

func (c *Client) TVChannels(ctx context.Context) ([]domain.TVChannel, error) {
	var response map[string]tvGame
	if err := c.doJSON(ctx, http.MethodGet, "/api/tv/channels", nil, &response); err != nil {
		return nil, err
	}
	channels := make([]domain.TVChannel, 0, len(response))
	for key, game := range response {
		channels = append(channels, domain.TVChannel{
			Key: key, GameID: game.GameID, Color: game.Color, Rating: game.Rating,
			Player: domain.Player{ID: game.User.ID, Name: game.User.Name, Title: game.User.Title, Bot: game.User.Bot},
		})
	}
	sort.Slice(channels, func(i, j int) bool { return channels[i].Key < channels[j].Key })
	return channels, nil
}

func (c *Client) StreamTV(ctx context.Context, channel string, handle func(TVEvent) error) error {
	path := "/api/tv/" + url.PathEscape(channel) + "/feed"
	return c.streamNDJSON(ctx, path, func(raw json.RawMessage) error {
		var event TVEvent
		if err := json.Unmarshal(raw, &event); err != nil {
			return fmt.Errorf("decode TV stream: %w", err)
		}
		return handle(event)
	})
}

func (c *Client) LiveStreamers(ctx context.Context) ([]domain.Streamer, error) {
	var response []struct {
		LightUser
		Stream struct {
			Service string `json:"service"`
			Status  string `json:"status"`
			Lang    string `json:"lang"`
		} `json:"stream"`
		Streamer struct {
			Name        string `json:"name"`
			Headline    string `json:"headline"`
			Description string `json:"description"`
			Twitch      string `json:"twitch"`
			YouTube     string `json:"youtube"`
			Image       string `json:"image"`
		} `json:"streamer"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/streamer/live", nil, &response); err != nil {
		return nil, err
	}
	streamers := make([]domain.Streamer, 0, len(response))
	for _, item := range response {
		streamURL := item.Streamer.Twitch
		if item.Stream.Service == "youtube" && item.Streamer.YouTube != "" {
			streamURL = item.Streamer.YouTube
		}
		streamers = append(streamers, domain.Streamer{
			ID: item.ID, Name: item.Streamer.Name, Title: item.Title,
			Service: item.Stream.Service, Status: item.Stream.Status, Language: item.Stream.Lang,
			Headline: item.Streamer.Headline, Description: item.Streamer.Description,
			URL: streamURL, ImageURL: item.Streamer.Image,
		})
	}
	return streamers, nil
}

func (c *Client) TopBroadcasts(ctx context.Context) ([]domain.Broadcast, error) {
	var response struct {
		Active []struct {
			Tour struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				Description string `json:"description"`
				URL         string `json:"url"`
			} `json:"tour"`
			Round struct {
				Ongoing bool `json:"ongoing"`
			} `json:"round"`
		} `json:"active"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/broadcast/top?page=1", nil, &response); err != nil {
		return nil, err
	}
	broadcasts := make([]domain.Broadcast, 0, len(response.Active))
	for _, item := range response.Active {
		broadcasts = append(broadcasts, domain.Broadcast{
			ID: item.Tour.ID, Name: item.Tour.Name, Description: item.Tour.Description,
			URL: item.Tour.URL, Live: item.Round.Ongoing,
		})
	}
	return broadcasts, nil
}

func (c *Client) ExportGamePGN(ctx context.Context, gameID string) (string, error) {
	path := "/game/export/" + url.PathEscape(gameID) + "?literate=false&clocks=true"
	return c.doText(ctx, http.MethodGet, path, "application/x-chess-pgn")
}
