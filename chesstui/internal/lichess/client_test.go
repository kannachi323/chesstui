package lichess

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kannachi323/chesstui/internal/domain"
)

func TestClientAddsAuthenticationAndParsesDailyPuzzle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/puzzle/daily" {
			t.Fatalf("path = %s", request.URL.Path)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("authorization = %q", got)
		}
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, `{"game":{"id":"g1","pgn":"1. e4"},"puzzle":{"id":"p1","rating":1800,"plays":42,"solution":["e2e4"],"themes":["opening"],"fen":"8/8/8/8/8/8/4P3/4K2k w - - 0 1","lastMove":"a7a6","initialPly":2}}`)
	}))
	defer server.Close()

	puzzle, err := NewClient(server.URL, "secret").DailyPuzzle(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if puzzle.ID != "p1" || puzzle.Rating != 1800 || len(puzzle.Moves) != 1 {
		t.Fatalf("unexpected puzzle: %#v", puzzle)
	}
}

func TestClientParsesTVStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprintln(writer, `{"t":"featured","d":{"id":"one","fen":"8/8/8/8/8/8/4P3/4K2k w - - 0 1"}}`)
		fmt.Fprintln(writer)
		fmt.Fprintln(writer, `{"t":"fen","d":{"id":"one","fen":"8/8/8/8/4P3/8/8/4K2k b - - 0 1","lm":"e2e4"}}`)
	}))
	defer server.Close()

	var events []TVEvent
	err := NewClient(server.URL, "").StreamTV(context.Background(), "best", func(event TVEvent) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[1].Data.LastMove != "e2e4" {
		t.Fatalf("events = %#v", events)
	}
}

func TestClientReportsRateLimitWithoutLeakingLargeBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Retry-After", "90")
		writer.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(writer, strings.Repeat("private", 1000))
	}))
	defer server.Close()

	_, err := NewClient(server.URL, "").TVChannels(context.Background())
	var rate *RateLimitError
	if !errors.As(err, &rate) {
		t.Fatalf("error = %v", err)
	}
	if wait := rate.Wait(); wait < 85*time.Second || strings.Contains(Describe(err), "private") {
		t.Fatalf("unsafe or unclear error: %v (%s)", err, wait)
	}
}

func TestTVChannelsAreStableAndSorted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(writer, `{"rapid":{"user":{"id":"r","name":"Rapid"},"rating":1900,"gameId":"two"},"best":{"user":{"id":"b","name":"Best"},"rating":2400,"gameId":"one"}}`)
	}))
	defer server.Close()

	channels, err := NewClient(server.URL, "").TVChannels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []domain.TVChannel{{Key: "best", GameID: "one", Player: domain.Player{ID: "b", Name: "Best"}, Rating: 2400}, {Key: "rapid", GameID: "two", Player: domain.Player{ID: "r", Name: "Rapid"}, Rating: 1900}}
	if fmt.Sprint(channels) != fmt.Sprint(want) {
		t.Fatalf("channels = %#v, want %#v", channels, want)
	}
}
