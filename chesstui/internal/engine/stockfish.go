package engine

import (
	"bufio"
	"bytes"
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
)

// ErrNotFound means no Stockfish binary is installed or configured.
var ErrNotFound = errors.New("Stockfish isn’t installed · brew install stockfish, or set stockfish_path")

type Analysis struct {
	Depth    int
	ScoreCP  int
	Mate     int
	PV       []string
	BestMove string
}

type Analyzer interface {
	Analyze(context.Context, string, int) (Analysis, error)
}

type Stockfish struct {
	path string
}

func NewStockfish(configuredPath string) (*Stockfish, error) {
	path, err := findStockfish(configuredPath)
	if err != nil {
		return nil, err
	}
	return &Stockfish{path: path}, nil
}

func (s *Stockfish) Path() string { return s.path }

// Analyze runs a fresh, isolated UCI process. Isolation makes cancellation and cleanup reliable.
func (s *Stockfish) Analyze(ctx context.Context, fen string, depth int) (Analysis, error) {
	if depth < 1 || depth > 40 {
		return Analysis{}, fmt.Errorf("analysis depth %d is outside 1..40", depth)
	}
	return s.run(ctx, fen, nil, fmt.Sprintf("go depth %d", depth))
}

// Play picks a move at a friendly strength. level runs from 1 (weakest) to
// 8 (full strength), mirroring Lichess AI levels.
func (s *Stockfish) Play(ctx context.Context, fen string, level int) (Analysis, error) {
	level = min(8, max(1, level))
	skill := (level - 1) * 20 / 7
	movetime := 50 * level * level // 50 ms at level 1, 3.2 s at level 8
	setup := []string{fmt.Sprintf("setoption name Skill Level value %d", skill)}
	return s.run(ctx, fen, setup, fmt.Sprintf("go movetime %d", movetime))
}

func (s *Stockfish) run(ctx context.Context, fen string, setup []string, goCommand string) (Analysis, error) {
	command := exec.CommandContext(ctx, s.path)
	stdin, err := command.StdinPipe()
	if err != nil {
		return Analysis{}, fmt.Errorf("open Stockfish stdin: %w", err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return Analysis{}, fmt.Errorf("open Stockfish stdout: %w", err)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		return Analysis{}, fmt.Errorf("start Stockfish: %w", err)
	}
	defer func() {
		if command.ProcessState == nil {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
	}()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	if err := sendUCI(stdin, "uci"); err != nil {
		return Analysis{}, err
	}
	if err := readUntil(scanner, "uciok"); err != nil {
		return Analysis{}, withEngineError(err, stderr.String())
	}
	if err := sendUCI(stdin, setup...); err != nil {
		return Analysis{}, err
	}
	if err := sendUCI(stdin, "isready"); err != nil {
		return Analysis{}, err
	}
	if err := readUntil(scanner, "readyok"); err != nil {
		return Analysis{}, withEngineError(err, stderr.String())
	}
	if err := sendUCI(stdin, "ucinewgame", "position fen "+fen, goCommand); err != nil {
		return Analysis{}, err
	}

	result := Analysis{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "info ") {
			parseInfo(line, &result)
			continue
		}
		if strings.HasPrefix(line, "bestmove ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[1] != "(none)" {
				result.BestMove = fields[1]
			}
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return Analysis{}, withEngineError(fmt.Errorf("read Stockfish: %w", err), stderr.String())
	}
	if result.BestMove == "" {
		return Analysis{}, withEngineError(errors.New("Stockfish returned no best move"), stderr.String())
	}
	_ = sendUCI(stdin, "quit")
	_ = stdin.Close()
	if err := command.Wait(); err != nil && ctx.Err() == nil {
		return Analysis{}, withEngineError(fmt.Errorf("Stockfish exited: %w", err), stderr.String())
	}
	return result, ctx.Err()
}

func findStockfish(configuredPath string) (string, error) {
	if configuredPath != "" {
		info, err := os.Stat(configuredPath)
		if err != nil || info.IsDir() {
			return "", fmt.Errorf("configured Stockfish executable is unavailable: %s", configuredPath)
		}
		return configuredPath, nil
	}
	if path, err := exec.LookPath("stockfish"); err == nil {
		return path, nil
	}
	candidates := []string{}
	if runtime.GOOS == "darwin" {
		candidates = []string{"/opt/homebrew/bin/stockfish", "/usr/local/bin/stockfish"}
	}
	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return filepath.Clean(path), nil
		}
	}
	return "", ErrNotFound
}

func sendUCI(writer io.Writer, commands ...string) error {
	for _, command := range commands {
		if _, err := fmt.Fprintln(writer, command); err != nil {
			return fmt.Errorf("send Stockfish command: %w", err)
		}
	}
	return nil
}

func readUntil(scanner *bufio.Scanner, target string) error {
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == target {
			return nil
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read Stockfish handshake: %w", err)
	}
	return fmt.Errorf("Stockfish closed before %s", target)
}

func parseInfo(line string, analysis *Analysis) {
	fields := strings.Fields(line)
	for i := 1; i < len(fields); i++ {
		switch fields[i] {
		case "depth":
			if i+1 < len(fields) {
				analysis.Depth, _ = strconv.Atoi(fields[i+1])
				i++
			}
		case "score":
			if i+2 < len(fields) {
				value, _ := strconv.Atoi(fields[i+2])
				if fields[i+1] == "cp" {
					analysis.ScoreCP = value
					analysis.Mate = 0
				} else if fields[i+1] == "mate" {
					analysis.Mate = value
				}
				i += 2
			}
		case "pv":
			analysis.PV = append(analysis.PV[:0], fields[i+1:]...)
			return
		}
	}
}

func withEngineError(err error, stderr string) error {
	stderr = strings.TrimSpace(stderr)
	if stderr == "" {
		return err
	}
	if len(stderr) > 1024 {
		stderr = stderr[:1024]
	}
	return fmt.Errorf("%w: %s", err, stderr)
}
