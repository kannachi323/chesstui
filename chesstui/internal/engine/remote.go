package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kannachi323/chesstui/internal/chess"
)

// DefaultAPI is chess-api.com, which runs Stockfish on its own servers and
// answers browsers from any origin. The web demo uses it, and anyone can set
// engine_api to it instead of installing an engine.
const DefaultAPI = "https://chess-api.com/v1"

// remoteMaxDepth is the deepest search chess-api.com runs for free.
const remoteMaxDepth = 18

var remoteClient = &http.Client{Timeout: 30 * time.Second}

// remoteReply is the part of chess-api.com's answer chesstui reads. Its
// scores are from White's point of view.
type remoteReply struct {
	Type         string   `json:"type"`
	Text         string   `json:"text"`
	Move         string   `json:"move"`
	Depth        flexInt  `json:"depth"`
	Centipawns   flexInt  `json:"centipawns"`
	Mate         flexInt  `json:"mate"`
	Continuation []string `json:"continuationArr"`
}

// flexInt reads a JSON number, a numeric string or null.
type flexInt int

func (n *flexInt) UnmarshalJSON(data []byte) error {
	text := strings.Trim(string(data), `"`)
	if text == "null" || text == "" {
		*n = 0
		return nil
	}
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return err
	}
	*n = flexInt(value)
	return nil
}

// searchRemote asks the HTTP engine for one position. The API takes a FEN,
// so the moves are played out here first. It returns a single line, since
// the REST endpoint has no MultiPV.
func (e *Engine) searchRemote(ctx context.Context, request Request, limit Limit) (Analysis, error) {
	position, err := chess.NewPosition(request.FEN)
	if err != nil {
		return Analysis{}, err
	}
	for _, move := range request.Moves {
		if err := position.ApplyUCI(move); err != nil {
			return Analysis{}, fmt.Errorf("replay %s: %w", move, err)
		}
	}
	if position.Terminal() {
		return Analysis{}, nil
	}
	depth := limit.Depth
	if depth <= 0 {
		depth = 12
	}
	body, _ := json.Marshal(map[string]any{"fen": apiFEN(position), "depth": min(depth, remoteMaxDepth)})

	var reply remoteReply
	for attempt := 0; ; attempt++ {
		reply, err = e.post(ctx, body)
		var busy errBusy
		if !errors.As(err, &busy) || attempt == 2 {
			break
		}
		select {
		case <-time.After(time.Duration(attempt+1) * time.Second):
		case <-ctx.Done():
			return Analysis{}, ctx.Err()
		}
	}
	if err != nil {
		return Analysis{}, err
	}
	if reply.Move == "" {
		return Analysis{}, fmt.Errorf("engine API: %s", strings.TrimSpace(reply.Text))
	}
	cp, mate := WhiteScore(int(reply.Centipawns), int(reply.Mate), position.Turn() == chess.White)
	pv := append([]string{reply.Move}, reply.Continuation...)
	line := Line{Depth: int(reply.Depth), ScoreCP: cp, Mate: mate, PV: pv}
	return Analysis{Depth: line.Depth, ScoreCP: cp, Mate: mate, PV: pv, BestMove: reply.Move, Lines: []Line{line}}, nil
}

// apiFEN clears the en passant square, because chess-api.com rejects any
// FEN that has one. The cost is that the API never suggests an en passant
// capture.
func apiFEN(position *chess.Position) string {
	fields := strings.Fields(position.FEN())
	if len(fields) >= 4 {
		fields[3] = "-"
	}
	return strings.Join(fields, " ")
}

type errBusy struct{ status int }

func (e errBusy) Error() string {
	return "The engine API is busy · try again in a moment"
}

func (e *Engine) post(ctx context.Context, body []byte) (remoteReply, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.api, bytes.NewReader(body))
	if err != nil {
		return remoteReply{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "chesstui (+https://github.com/kannachi323/chesstui)")
	res, err := remoteClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return remoteReply{}, ctx.Err()
		}
		return remoteReply{}, fmt.Errorf("Can't reach the engine API · check your connection")
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusTooManyRequests || res.StatusCode >= 500 {
		return remoteReply{}, errBusy{res.StatusCode}
	}
	if res.StatusCode != http.StatusOK {
		return remoteReply{}, fmt.Errorf("engine API answered %s", res.Status)
	}
	var reply remoteReply
	if err := json.NewDecoder(res.Body).Decode(&reply); err != nil {
		return remoteReply{}, fmt.Errorf("read engine API answer: %w", err)
	}
	return reply, nil
}

// remoteLevels maps Play strengths to search depth and the chance of a
// random move, since the API has no Skill Level.
var remoteLevels = [9]struct {
	depth  int
	random float64
}{{}, {1, 0.4}, {2, 0.3}, {3, 0.2}, {5, 0.1}, {7, 0.05}, {10, 0}, {13, 0}, {18, 0}}

func (e *Engine) playRemote(ctx context.Context, request Request, level int) (Analysis, error) {
	setting := remoteLevels[level]
	if rand.Float64() < setting.random {
		position, err := chess.NewPosition(request.FEN)
		if err == nil && position.ApplyMoves(strings.Join(request.Moves, " ")) == nil {
			if moves := position.LegalMoves(); len(moves) > 0 {
				return Analysis{BestMove: moves[rand.IntN(len(moves))]}, nil
			}
		}
	}
	return e.Search(ctx, request, Limit{Depth: setting.depth})
}
