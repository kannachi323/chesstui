package engine

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestParseInfo(t *testing.T) {
	var result Analysis
	parseInfo("info depth 18 score cp 37 nodes 42 pv e2e4 e7e5", &result)
	if result.Depth != 18 || result.ScoreCP != 37 || len(result.PV) != 2 {
		t.Fatalf("unexpected analysis: %#v", result)
	}
}

// fakeEngine is a tiny UCI responder so the process plumbing is tested even
// on machines without Stockfish.
const fakeEngine = `#!/bin/sh
while read -r line; do
  case "$line" in
    uci) echo "id name Fake"; echo "uciok" ;;
    isready) echo "readyok" ;;
    go*) echo "info depth 12 score mate 2 pv d1h5 g7g6"; echo "bestmove d1h5 ponder g7g6" ;;
    quit) exit 0 ;;
  esac
done
`

func TestStockfishProcessRoundTrip(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell script as the engine")
	}
	path := filepath.Join(t.TempDir(), "stockfish")
	if err := os.WriteFile(path, []byte(fakeEngine), 0o755); err != nil {
		t.Fatal(err)
	}
	engine, err := NewStockfish(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for name, run := range map[string]func() (Analysis, error){
		"analyze": func() (Analysis, error) { return engine.Analyze(ctx, "startpos", 12) },
		"play":    func() (Analysis, error) { return engine.Play(ctx, "startpos", 3) },
	} {
		result, err := run()
		if err != nil || result.BestMove != "d1h5" || result.Mate != 2 || result.Depth != 12 {
			t.Fatalf("%s: %+v err=%v", name, result, err)
		}
	}
}

func TestMissingStockfishIsReported(t *testing.T) {
	if _, err := NewStockfish(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("expected an error for a missing binary")
	}
}
