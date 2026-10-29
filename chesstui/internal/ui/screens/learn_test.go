package screens

import (
	"strings"
	"testing"

	chesscore "github.com/kannachi323/chesstui/internal/chess"
	"github.com/kannachi323/chesstui/internal/domain"
)

// Every lesson must start from a legal position with a legal answer, and
// mate lessons must really end in mate.
func TestLessonsAreSolvable(t *testing.T) {
	for _, item := range lessons {
		position, err := chesscore.NewPosition(item.FEN)
		if err != nil {
			t.Fatalf("%s: %v", item.Title, err)
		}
		for _, move := range item.Moves {
			clone := position.Clone()
			if err := clone.ApplyUCI(move); err != nil {
				t.Fatalf("%s: %s is illegal: %v", item.Title, move, err)
			}
			if item.Mate && !strings.HasPrefix(clone.Outcome(), "Checkmate") {
				t.Fatalf("%s: %s does not mate", item.Title, move)
			}
		}
	}
}

func TestPreparePuzzlePlaysTheSetupMoveForPacks(t *testing.T) {
	// A Lichess CSV row: the FEN is before the opponent's move.
	puzzle, position, err := preparePuzzle(domainPuzzle("r6k/pp2r2p/4Rp1Q/3p4/8/1N1P2R1/PqP2bPP/7K b - - 0 24", "f2g3 e6e7 b2b1 b3c1 b1c1 h6c1"))
	if err != nil {
		t.Fatal(err)
	}
	if puzzle.LastMove != "f2g3" || len(puzzle.Moves) != 5 || position.Turn() != chesscore.White {
		t.Fatalf("puzzle=%+v turn=%v", puzzle, position.Turn())
	}
}

func TestHumanThemes(t *testing.T) {
	if got := humanThemes([]string{"mateIn2", "short", "backRankMate", "endgame"}, 2); got != "mate in 2 · back rank mate" {
		t.Fatalf("got %q", got)
	}
}

func domainPuzzle(fen, moves string) domain.Puzzle {
	return domain.Puzzle{ID: "00008", FEN: fen, Moves: strings.Fields(moves), Source: "pack.csv"}
}
