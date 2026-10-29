package lichess

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func fastRetries(t *testing.T) {
	t.Helper()
	saved := retryDelays
	retryDelays = []time.Duration{time.Millisecond, time.Millisecond}
	t.Cleanup(func() { retryDelays = saved })
}

func TestRateLimitPausesEveryClientForTheHost(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if got := r.Header.Get("User-Agent"); !strings.HasPrefix(got, "ChessTUI/") {
			t.Errorf("User-Agent = %q", got)
		}
		w.Header().Set("Retry-After", "5")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	_, err := NewClient(server.URL, "").DailyPuzzle(context.Background())
	var rate *RateLimitError
	if !errors.As(err, &rate) {
		t.Fatalf("err = %v, want RateLimitError", err)
	}
	if wait := rate.Wait(); wait < 55*time.Second {
		t.Fatalf("pause = %s, want at least a minute as Lichess asks", wait)
	}
	// A fresh client, as after signing in, must honour the same pause.
	if _, err := NewClient(server.URL, "token").TVChannels(context.Background()); !errors.As(err, &rate) {
		t.Fatalf("second client err = %v", err)
	}
	if hits.Load() != 1 {
		t.Fatalf("server hit %d times during the pause", hits.Load())
	}
	if !strings.Contains(Describe(err), "pause") {
		t.Fatalf("Describe = %q", Describe(err))
	}
}

func TestGetRetriesServerErrorsButPostDoesNot(t *testing.T) {
	fastRetries(t)
	var gets, posts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posts.Add(1)
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		if gets.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		fmt.Fprint(w, `{"puzzle":{"id":"p1","fen":"8/8/8/8/8/8/4P3/4K2k w - - 0 1","solution":["e2e4"]}}`)
	}))
	defer server.Close()
	client := NewClient(server.URL, "t")
	if puzzle, err := client.DailyPuzzle(context.Background()); err != nil || puzzle.ID != "p1" {
		t.Fatalf("puzzle=%+v err=%v", puzzle, err)
	}
	if err := client.Move(context.Background(), "g", "e2e4", false); err == nil {
		t.Fatal("move should fail")
	}
	if gets.Load() != 3 || posts.Load() != 1 {
		t.Fatalf("gets=%d posts=%d, want 3 and 1", gets.Load(), posts.Load())
	}
}

func TestOfflineErrorsAreDescribed(t *testing.T) {
	fastRetries(t)
	server := httptest.NewServer(http.NotFoundHandler())
	address := server.URL
	server.Close() // nothing listens here any more
	_, err := NewClient(address, "").TVChannels(context.Background())
	if err == nil || Describe(err) != "Offline · check your connection" {
		t.Fatalf("err=%v describe=%q", err, Describe(err))
	}
}

func TestReconnectBacksOffAndStopsOnPermanentErrors(t *testing.T) {
	calls := 0
	waits := []time.Duration{}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := Reconnect(ctx, func(context.Context) error {
		calls++
		if calls < 3 {
			return &APIError{StatusCode: 502}
		}
		return &APIError{StatusCode: 404, Message: "gone"}
	}, func(wait time.Duration, _ error) { waits = append(waits, wait) })
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 404 || calls != 3 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
	if len(waits) != 2 || waits[1] <= waits[0] {
		t.Fatalf("waits = %v, want growing backoff", waits)
	}
}

// TestPKCELoginExchangesVerifier plays the browser and Lichess: it follows
// the authorize URL back to the loopback listener and checks that the token
// request carries a verifier matching the S256 challenge.
func TestPKCELoginExchangesVerifier(t *testing.T) {
	var challenge string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/token" || r.Method != http.MethodPost {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			return
		}
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
		if base64.RawURLEncoding.EncodeToString(sum[:]) != challenge || r.PostForm.Get("code") != "abc" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":"invalid_grant"}`)
			return
		}
		if r.Header.Get("Authorization") != "" {
			t.Error("token exchange must not send an old bearer token")
		}
		fmt.Fprint(w, `{"access_token":"lio_test","token_type":"Bearer"}`)
	}))
	defer server.Close()

	login, err := NewClient(server.URL, "old-token").BeginLogin(DefaultScopes)
	if err != nil {
		t.Fatal(err)
	}
	authorize, _ := url.Parse(login.URL)
	query := authorize.Query()
	challenge = query.Get("code_challenge")
	if query.Get("code_challenge_method") != "S256" || !strings.HasPrefix(query.Get("redirect_uri"), "http://127.0.0.1:") {
		t.Fatalf("authorize query = %v", query)
	}

	// A stray request with the wrong state must not end the login.
	stray, err := http.Get(query.Get("redirect_uri") + "?state=wrong&code=evil")
	if err != nil {
		t.Fatal(err)
	}
	stray.Body.Close()
	if stray.StatusCode != http.StatusBadRequest {
		t.Fatalf("stray callback status = %d", stray.StatusCode)
	}
	callback, err := http.Get(query.Get("redirect_uri") + "?state=" + url.QueryEscape(query.Get("state")) + "&code=abc")
	if err != nil {
		t.Fatal(err)
	}
	callback.Body.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	token, err := login.Wait(ctx)
	if err != nil || token != "lio_test" {
		t.Fatalf("token=%q err=%v", token, err)
	}
}

func TestPKCELoginCanBeCancelled(t *testing.T) {
	login, err := NewClient("http://127.0.0.1:1", "").BeginLogin(DefaultScopes)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := login.Wait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	login.Close()
}
