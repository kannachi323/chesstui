package engine

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/kannachi323/chesstui/internal/chess"
)

func TestRemoteSearch(t *testing.T) {
	var got map[string]any
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests) // retried
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		// chess-api.com scores from White's side: +35 with Black to move.
		_, _ = w.Write([]byte(`{"type":"bestmove","move":"e7e5","depth":12,"centipawns":"35","mate":null,"continuationArr":["g1f3","b8c6"]}`))
	}))
	defer server.Close()

	e, err := New(Options{API: server.URL + "/v1"})
	if err != nil {
		t.Fatal(err)
	}
	if e.Name() != "Stockfish · "+server.Listener.Addr().String() || e.Ensure() != nil {
		t.Fatalf("name %q", e.Name())
	}
	analysis, err := e.Search(context.Background(), Request{Moves: []string{"e2e4"}}, Limit{Depth: 30, MultiPV: 3})
	if err != nil {
		t.Fatal(err)
	}
	if got["fen"] != "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq - 0 1" || got["depth"] != float64(remoteMaxDepth) {
		t.Fatalf("request %v", got)
	}
	if analysis.BestMove != "e7e5" || analysis.ScoreCP != -35 || analysis.Depth != 12 || len(analysis.PV) != 3 || len(analysis.Lines) != 1 {
		t.Fatalf("analysis %+v", analysis)
	}
}

func TestRemoteFinishedPosition(t *testing.T) {
	e, _ := New(Options{API: "http://127.0.0.1:1/v1"}) // never reached
	mated := Request{Moves: []string{"f2f3", "e7e5", "g2g4", "d8h4"}}
	if analysis, err := e.Search(context.Background(), mated, Limit{Depth: 10}); err != nil || analysis.BestMove != "" {
		t.Fatalf("analysis %+v err %v", analysis, err)
	}
}

func TestAPIFENClearsEnPassant(t *testing.T) {
	position, _ := chess.NewPosition("")
	_ = position.ApplyMoves("e2e4 a7a6 e4e5 d7d5")
	if fen := apiFEN(position); !strings.HasSuffix(fen, " w KQkq - 0 3") {
		t.Fatalf("fen %s", fen)
	}
}
