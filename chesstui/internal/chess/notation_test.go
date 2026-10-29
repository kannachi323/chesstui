package chess

import (
	"strings"
	"testing"
)

func TestSANMoves(t *testing.T) {
	position, _ := NewPosition("")
	if err := position.ApplyMoves("e2e4 e7e5 g1f3 b8c6 f1b5 a7a6 b5c6 d7c6 e1g1 f7f6 f3e5 f6e5 d1h5 e8e7 h5e5 e7f7 f1e1 g8f6 e5e8"); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(position.SANMoves(), " ")
	want := "e4 e5 Nf3 Nc6 Bb5 a6 Bxc6 dxc6 O-O f6 Nxe5 fxe5 Qh5+ Ke7 Qxe5+ Kf7 Re1 Nf6 Qe8+"
	if got != want {
		t.Fatalf("SAN = %q\nwant  %q", got, want)
	}
	if position.KingSquare(Black) != "f7" {
		t.Fatalf("black king = %s", position.KingSquare(Black))
	}
}

func TestSANMarksMate(t *testing.T) {
	position, _ := NewPosition("")
	_ = position.ApplyMoves("e2e4 e7e5 f1c4 b8c6 d1h5 g8f6 h5f7")
	if moves := position.SANMoves(); moves[len(moves)-1] != "Qxf7#" {
		t.Fatalf("moves = %v", moves)
	}
	if position.Outcome() != "Checkmate · white wins" {
		t.Fatalf("outcome = %q", position.Outcome())
	}
}

func TestSANDisambiguatesAndPromotes(t *testing.T) {
	position, _ := NewPosition("4k3/P7/8/8/8/8/4K3/R6R w - - 0 1")
	if san := position.SAN("a1d1"); san != "Rad1" {
		t.Fatalf("a1d1 = %s", san)
	}
	if san := position.SAN("a7a8q"); san != "a8=Q+" {
		t.Fatalf("a7a8q = %s", san)
	}
}

func TestResolveMoveAcceptsUCIAndSAN(t *testing.T) {
	position, _ := NewPosition("")
	for input, want := range map[string]string{"e2e4": "e2e4", "e4": "e2e4", "Nf3": "g1f3", "nf3": "g1f3", "zz": ""} {
		got, ok := position.ResolveMove(input)
		if got != want || ok != (want != "") {
			t.Fatalf("ResolveMove(%q) = %q, %v", input, got, ok)
		}
	}
	promotion, _ := NewPosition("4k3/P7/8/8/8/8/8/4K3 w - - 0 1")
	if got, _ := promotion.ResolveMove("a7a8"); got != "a7a8q" {
		t.Fatalf("auto-queen = %q", got)
	}
}

func TestApplyNotationReadsLichessSAN(t *testing.T) {
	position, _ := NewPosition("")
	if err := position.ApplyNotation("e4 e5 Nf3 Nc6 Bb5 a6 O-O"); err != nil {
		t.Fatal(err)
	}
	if moves := position.Moves(); moves[len(moves)-1] != "e1g1" {
		t.Fatalf("moves = %v", moves)
	}
}
