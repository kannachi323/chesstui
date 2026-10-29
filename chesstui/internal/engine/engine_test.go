package engine

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestParseInfo(t *testing.T) {
	index, line, ok := parseInfo("info depth 18 multipv 2 score cp 37 nodes 42 pv e2e4 e7e5")
	if !ok || index != 1 || line.Depth != 18 || line.ScoreCP != 37 || len(line.PV) != 2 {
		t.Fatalf("index=%d line=%+v ok=%v", index, line, ok)
	}
}

// fakeEngine is a tiny UCI responder, so the process plumbing is tested even
// on machines without Stockfish. It logs every command it receives.
const fakeEngine = `#!/bin/sh
log="$0.log"
while read -r line; do
  echo "$line" >> "$log"
  case "$line" in
    uci) echo "id name FakeFish 1"; echo "uciok" ;;
    isready) echo "readyok" ;;
    go*) echo "info depth 12 multipv 1 score mate 2 pv d1h5 g7g6"; echo "info depth 12 multipv 2 score cp -40 pv g1f3 b8c6"; echo "bestmove d1h5 ponder g7g6" ;;
    quit) exit 0 ;;
  esac
done
`

func fake(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell script as the engine")
	}
	path := filepath.Join(t.TempDir(), "fakefish")
	if err := os.WriteFile(path, []byte(fakeEngine), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEngineSessionSendsTheWholeGame(t *testing.T) {
	path := fake(t)
	engine, err := New(Options{Path: path, Threads: 2, HashMB: 64})
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := engine.Search(ctx, Request{Moves: []string{"e2e4", "e7e5"}}, Limit{Depth: 12, MultiPV: 2})
	if err != nil || result.BestMove != "d1h5" || result.Mate != 2 || len(result.Lines) != 2 || result.Lines[1].ScoreCP != -40 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if engine.Name() != "FakeFish 1" {
		t.Fatalf("name = %q", engine.Name())
	}
	if _, err := engine.Play(ctx, Request{FEN: "8/8/8/8/8/8/4P3/4K2k w - - 0 1"}, 3); err != nil {
		t.Fatal(err)
	}
	engine.Close()
	log, _ := os.ReadFile(path + ".log")
	for _, want := range []string{
		"setoption name Threads value 2", "setoption name Hash value 64", "setoption name MultiPV value 2",
		"position startpos moves e2e4 e7e5", "go depth 12",
		"setoption name Skill Level value 5", "position fen 8/8/8/8/8/8/4P3/4K2k w - - 0 1", "go movetime 450",
	} {
		if !strings.Contains(string(log), want) {
			t.Fatalf("engine never received %q:\n%s", want, log)
		}
	}
	if strings.Count(string(log), "\nuci\n")+strings.Count(string(log)[:4], "uci\n") != 1 {
		t.Fatalf("engine should start once:\n%s", log)
	}
}

func TestMissingEngineIsReported(t *testing.T) {
	if _, err := New(Options{Path: filepath.Join(t.TempDir(), "nope")}); err == nil {
		t.Fatal("expected an error for a missing binary")
	}
}
