// Command fakeengine is a deterministic stand-in for a UCI engine, used by
// tests and screenshots on machines without Stockfish. It scores positions
// by material and plays the best capture it can see, else its first move.
package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"

	chesscore "github.com/kannachi323/chesstui/internal/chess"
)

var values = map[rune]int{'p': 100, 'n': 300, 'b': 320, 'r': 500, 'q': 900}

func material(position *chesscore.Position) int {
	score := 0
	for rank := range 8 {
		for file := range 8 {
			piece := position.PieceAt(file, rank)
			value := values[unicode.ToLower(piece.Rune)]
			if piece.Color == chesscore.White {
				score += value
			} else {
				score -= value
			}
		}
	}
	return score
}

// sideScore is material from the side to move's view after move.
func sideScore(position *chesscore.Position, move string) (int, bool) {
	after := position.Clone()
	if after.ApplyUCI(move) != nil {
		return 0, false
	}
	if strings.HasPrefix(after.Outcome(), "Checkmate") {
		return 100000, true
	}
	score := material(after)
	if position.Turn() == chesscore.Black {
		score = -score
	}
	// Assume the best recapture: subtract the most valuable piece the
	// opponent could take next.
	worst := 0
	for _, reply := range after.LegalMoves() {
		next := after.Clone()
		if next.ApplyUCI(reply) != nil {
			continue
		}
		swing := material(next) - material(after)
		if position.Turn() == chesscore.White {
			swing = -swing
		}
		if strings.HasPrefix(next.Outcome(), "Checkmate") {
			swing = 100000
		}
		worst = max(worst, swing)
	}
	return score - worst, true
}

func main() {
	position, _ := chesscore.NewPosition("")
	multiPV := 1
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		fields := strings.Fields(line)
		switch {
		case line == "uci":
			fmt.Println("id name FakeFish 1")
			fmt.Println("uciok")
		case line == "isready":
			fmt.Println("readyok")
		case strings.HasPrefix(line, "setoption name MultiPV value"):
			fmt.Sscan(fields[len(fields)-1], &multiPV)
		case strings.HasPrefix(line, "position"):
			fen, rest := "", fields[1:]
			if rest[0] == "fen" {
				end := len(rest)
				for i, f := range rest {
					if f == "moves" {
						end = i
						break
					}
				}
				fen, rest = strings.Join(rest[1:end], " "), rest[end:]
			} else {
				rest = rest[1:]
			}
			position, _ = chesscore.NewPosition(fen)
			if len(rest) > 0 && rest[0] == "moves" {
				_ = position.ApplyMoves(strings.Join(rest[1:], " "))
			}
		case strings.HasPrefix(line, "go"):
			type scored struct {
				move  string
				score int
			}
			var moves []scored
			for _, move := range position.LegalMoves() {
				if score, ok := sideScore(position, move); ok {
					moves = append(moves, scored{move, score})
				}
			}
			sort.SliceStable(moves, func(i, j int) bool { return moves[i].score > moves[j].score })
			if len(moves) == 0 {
				fmt.Println("bestmove (none)")
				continue
			}
			for i := 0; i < multiPV && i < len(moves); i++ {
				pv := moves[i].move
				after := position.Clone()
				_ = after.ApplyUCI(pv)
				if replies := after.LegalMoves(); len(replies) > 0 {
					pv += " " + replies[0]
				}
				switch score := moves[i].score; {
				case score >= 50000:
					fmt.Printf("info depth 14 multipv %d score mate 1 pv %s\n", i+1, moves[i].move)
				case score <= -50000:
					fmt.Printf("info depth 14 multipv %d score mate -1 pv %s\n", i+1, pv)
				default:
					fmt.Printf("info depth 14 multipv %d score cp %d pv %s\n", i+1, min(1500, max(-1500, score+15)), pv)
				}
			}
			fmt.Printf("bestmove %s\n", moves[0].move)
		case line == "quit":
			return
		}
	}
}
