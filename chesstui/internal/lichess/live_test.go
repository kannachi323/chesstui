//go:build live

package lichess

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestLivePublicEndpoints checks every no-account feature against the real
// lichess.org. It makes eight requests; run it rarely:
//
//	go test -tags live -run Live ./internal/lichess
func TestLivePublicEndpoints(t *testing.T) {
	client := NewClient("https://lichess.org", "")
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	daily, err := client.DailyPuzzle(ctx)
	if err != nil || daily.FEN == "" || len(daily.Moves) == 0 {
		t.Fatalf("daily puzzle: %+v %v", daily, err)
	}
	next, err := client.NextPuzzle(ctx)
	if err != nil || next.ID == "" {
		t.Fatalf("next puzzle: %v", err)
	}
	channels, err := client.TVChannels(ctx)
	if err != nil || len(channels) == 0 {
		t.Fatalf("tv channels: %d %v", len(channels), err)
	}
	streamCtx, stop := context.WithTimeout(ctx, 10*time.Second)
	var first TVEvent
	err = client.StreamTV(streamCtx, "best", func(event TVEvent) error {
		first = event
		return errors.New("got one")
	})
	stop()
	if first.Type != "featured" || first.Data.FEN == "" {
		t.Fatalf("tv feed: %+v %v", first, err)
	}
	games := 0
	if err := client.RecentGames(ctx, "lichess", 3, func(ExportedGame) error { games++; return nil }); err != nil {
		t.Fatalf("games: %v", err)
	}
	studies := 0
	err = client.Studies(ctx, "lichess", func(Study) error {
		studies++
		if studies == 2 {
			return errors.New("enough")
		}
		return nil
	})
	if studies == 0 {
		t.Fatalf("studies: %v", err)
	}
	if _, err := client.LiveStreamers(ctx); err != nil {
		t.Fatalf("streamers: %v", err)
	}
	if _, err := client.TopBroadcasts(ctx); err != nil {
		t.Fatalf("broadcasts: %v", err)
	}
	t.Logf("daily %s (%d), next %s, %d channels, tv %s, %d games, %d studies", daily.ID, daily.Rating, next.ID, len(channels), first.Data.ID, games, studies)
}
