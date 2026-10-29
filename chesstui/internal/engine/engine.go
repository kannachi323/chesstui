// Package engine talks UCI to an engine the user already has installed,
// such as Stockfish, Lc0 or Komodo. chesstui ships no engine of its own.
package engine

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ErrNotFound means no UCI engine is installed or configured.
var ErrNotFound = errors.New("No chess engine found · brew install stockfish, or set engine_path")

// Line is one principal variation. Scores are from the side to move.
type Line struct {
	Depth   int
	ScoreCP int
	Mate    int
	PV      []string
}

type Analysis struct {
	Depth    int
	ScoreCP  int
	Mate     int
	PV       []string
	BestMove string
	Lines    []Line // MultiPV lines, best first
}

// Request is a game position: a start FEN ("" for the standard start) and
// the moves played since, so the engine sees repetitions.
type Request struct {
	FEN   string
	Moves []string
}

// Limit bounds one search. Zero fields are ignored.
type Limit struct {
	Depth    int
	MoveTime time.Duration
	MultiPV  int
	Skill    int // 0–20 Stockfish Skill Level; -1 leaves full strength
}

// Options configure the engine process.
type Options struct {
	Path    string
	Threads int
	HashMB  int
}

// Engine is one long-running UCI process. Calls are serialized; the process
// starts on first use and restarts after a crash.
type Engine struct {
	opts Options
	path string

	mu      sync.Mutex
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	lines   chan string
	name    string
	multiPV int
	skill   int
}

// Find resolves the engine binary without starting it.
func Find(configured string) (string, error) {
	if configured != "" {
		if strings.HasPrefix(configured, "~/") {
			if home, err := os.UserHomeDir(); err == nil {
				configured = filepath.Join(home, configured[2:])
			}
		}
		info, err := os.Stat(configured)
		if err != nil || info.IsDir() {
			return "", fmt.Errorf("engine not found at %s · fix engine_path in Settings", configured)
		}
		return configured, nil
	}
	for _, name := range []string{"stockfish"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	candidates := []string{}
	switch runtime.GOOS {
	case "darwin":
		candidates = []string{"/opt/homebrew/bin/stockfish", "/usr/local/bin/stockfish"}
	case "linux":
		candidates = []string{"/usr/games/stockfish", "/usr/bin/stockfish", "/snap/bin/stockfish"}
	}
	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return filepath.Clean(path), nil
		}
	}
	return "", ErrNotFound
}

// New resolves the engine. The process is started lazily.
func New(opts Options) (*Engine, error) {
	path, err := Find(opts.Path)
	if err != nil {
		return nil, err
	}
	return &Engine{opts: opts, path: path, skill: -1, multiPV: 1}, nil
}

// Ensure starts the engine process if it is not running, so Name reports
// the engine's own name.
func (e *Engine) Ensure() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cmd != nil {
		return nil
	}
	return e.start()
}

// Path is the resolved engine binary.
func (e *Engine) Path() string { return e.path }

// Name is the engine's self-reported name, once it has started.
func (e *Engine) Name() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.name != "" {
		return e.name
	}
	return filepath.Base(e.path)
}

func (e *Engine) start() error {
	cmd := exec.Command(e.path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("open engine stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("open engine stdout: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start engine: %w", err)
	}
	lines := make(chan string, 256)
	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			lines <- strings.TrimSpace(scanner.Text())
		}
		close(lines)
	}()
	e.cmd, e.stdin, e.lines, e.multiPV, e.skill = cmd, stdin, lines, 1, -1
	if err := e.send("uci"); err != nil {
		return err
	}
	if err := e.waitFor(context.Background(), 10*time.Second, func(line string) bool {
		if name, ok := strings.CutPrefix(line, "id name "); ok {
			e.name = name
		}
		return line == "uciok"
	}); err != nil {
		e.kill()
		return fmt.Errorf("engine handshake: %w", err)
	}
	if e.opts.Threads > 0 {
		_ = e.send(fmt.Sprintf("setoption name Threads value %d", e.opts.Threads))
	}
	if e.opts.HashMB > 0 {
		_ = e.send(fmt.Sprintf("setoption name Hash value %d", e.opts.HashMB))
	}
	return e.ready()
}

func (e *Engine) send(commands ...string) error {
	for _, command := range commands {
		if _, err := fmt.Fprintln(e.stdin, command); err != nil {
			return fmt.Errorf("send to engine: %w", err)
		}
	}
	return nil
}

func (e *Engine) ready() error {
	if err := e.send("isready"); err != nil {
		return err
	}
	return e.waitFor(context.Background(), 10*time.Second, func(line string) bool { return line == "readyok" })
}

// waitFor reads lines until match returns true.
func (e *Engine) waitFor(ctx context.Context, timeout time.Duration, match func(string) bool) error {
	deadline := time.After(timeout)
	for {
		select {
		case line, ok := <-e.lines:
			if !ok {
				return errors.New("engine exited")
			}
			if match(line) {
				return nil
			}
		case <-deadline:
			return errors.New("engine did not answer in time")
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (e *Engine) kill() {
	if e.cmd != nil && e.cmd.Process != nil {
		_ = e.cmd.Process.Kill()
		_ = e.cmd.Wait()
	}
	e.cmd, e.stdin, e.lines = nil, nil, nil
}

// Close quits the engine process.
func (e *Engine) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cmd != nil {
		_ = e.send("quit")
		done := make(chan struct{})
		go func() { _ = e.cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(time.Second):
			_ = e.cmd.Process.Kill()
		}
		e.cmd = nil
	}
}

// Search runs one search on the position. Cancelling ctx stops the engine
// and returns the best result found so far with ctx's error.
func (e *Engine) Search(ctx context.Context, request Request, limit Limit) (Analysis, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cmd == nil {
		if err := e.start(); err != nil {
			return Analysis{}, err
		}
	}
	analysis, err := e.search(ctx, request, limit)
	if err != nil && ctx.Err() == nil {
		e.kill() // a confused engine is restarted on the next call
	}
	return analysis, err
}

func (e *Engine) search(ctx context.Context, request Request, limit Limit) (Analysis, error) {
	multiPV := max(1, limit.MultiPV)
	if multiPV != e.multiPV {
		if err := e.send(fmt.Sprintf("setoption name MultiPV value %d", multiPV)); err != nil {
			return Analysis{}, err
		}
		e.multiPV = multiPV
	}
	skill := limit.Skill
	if skill == 0 {
		skill = -1
	}
	if skill != e.skill {
		value := skill
		if value < 0 {
			value = 20
		}
		if err := e.send(fmt.Sprintf("setoption name Skill Level value %d", value)); err != nil {
			return Analysis{}, err
		}
		e.skill = skill
	}
	position := "position startpos"
	if request.FEN != "" && request.FEN != "startpos" {
		position = "position fen " + request.FEN
	}
	if len(request.Moves) > 0 {
		position += " moves " + strings.Join(request.Moves, " ")
	}
	goCommand := "go"
	if limit.Depth > 0 {
		goCommand += fmt.Sprintf(" depth %d", limit.Depth)
	}
	if limit.MoveTime > 0 {
		goCommand += fmt.Sprintf(" movetime %d", limit.MoveTime.Milliseconds())
	}
	if goCommand == "go" {
		goCommand = "go depth 18"
	}
	if err := e.send(position, goCommand); err != nil {
		return Analysis{}, err
	}

	lines := make([]Line, multiPV)
	result := Analysis{}
	stopped := false
	for {
		select {
		case line, ok := <-e.lines:
			if !ok {
				return result, errors.New("engine exited during search")
			}
			if strings.HasPrefix(line, "info ") {
				if index, parsed, ok := parseInfo(line); ok && index < len(lines) {
					lines[index] = parsed
				}
				continue
			}
			if !strings.HasPrefix(line, "bestmove") {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[1] != "(none)" {
				result.BestMove = fields[1]
			}
			for _, l := range lines {
				if len(l.PV) > 0 {
					result.Lines = append(result.Lines, l)
				}
			}
			if len(result.Lines) > 0 {
				best := result.Lines[0]
				result.Depth, result.ScoreCP, result.Mate, result.PV = best.Depth, best.ScoreCP, best.Mate, best.PV
			}
			if result.BestMove == "" && len(result.PV) > 0 {
				result.BestMove = result.PV[0]
			}
			if stopped {
				return result, ctx.Err()
			}
			return result, nil
		case <-ctx.Done():
			if !stopped {
				stopped = true
				_ = e.send("stop")
			}
		}
	}
}

// parseInfo reads one "info" line into a Line and its MultiPV index.
func parseInfo(line string) (int, Line, bool) {
	fields := strings.Fields(line)
	var parsed Line
	index, hasScore := 0, false
	for i := 1; i < len(fields); i++ {
		switch fields[i] {
		case "depth":
			if i+1 < len(fields) {
				parsed.Depth, _ = strconv.Atoi(fields[i+1])
				i++
			}
		case "multipv":
			if i+1 < len(fields) {
				n, _ := strconv.Atoi(fields[i+1])
				index = max(0, n-1)
				i++
			}
		case "score":
			if i+2 < len(fields) {
				value, _ := strconv.Atoi(fields[i+2])
				if fields[i+1] == "cp" {
					parsed.ScoreCP = value
				} else if fields[i+1] == "mate" {
					parsed.Mate = value
				}
				hasScore = true
				i += 2
			}
		case "pv":
			parsed.PV = append([]string(nil), fields[i+1:]...)
			i = len(fields)
		}
	}
	return index, parsed, hasScore && len(parsed.PV) > 0
}

// Analyze is a convenience for a fixed-depth search of one FEN.
func (e *Engine) Analyze(ctx context.Context, fen string, depth int) (Analysis, error) {
	if depth < 1 || depth > 40 {
		return Analysis{}, fmt.Errorf("analysis depth %d is outside 1..40", depth)
	}
	return e.Search(ctx, Request{FEN: fen}, Limit{Depth: depth})
}

// Play picks a move at a friendly strength. level runs from 1 (weakest) to
// 8 (full strength), mirroring Lichess AI levels.
func (e *Engine) Play(ctx context.Context, request Request, level int) (Analysis, error) {
	level = min(8, max(1, level))
	limit := Limit{MoveTime: time.Duration(50*level*level) * time.Millisecond, Skill: (level - 1) * 20 / 7}
	if level == 8 {
		limit.Skill = -1
	}
	return e.Search(ctx, request, limit)
}

// WhiteScore converts a side-to-move score to White's point of view.
func WhiteScore(cp, mate int, whiteToMove bool) (int, int) {
	if whiteToMove {
		return cp, mate
	}
	return -cp, -mate
}
