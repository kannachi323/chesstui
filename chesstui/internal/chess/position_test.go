package chess

import "testing"

func TestPositionAppliesLegalMoves(t *testing.T) {
	position, err := NewPosition("")
	if err != nil {
		t.Fatal(err)
	}
	for _, move := range []string{"e2e4", "e7e5", "g1f3"} {
		if err := position.ApplyUCI(move); err != nil {
			t.Fatalf("apply %s: %v", move, err)
		}
	}
	if got := position.Ply(); got != 3 {
		t.Fatalf("ply = %d, want 3", got)
	}
	if err := position.ApplyUCI("e1e8"); err == nil {
		t.Fatal("expected illegal move to fail")
	}
}

func TestPositionTimeline(t *testing.T) {
	position, _ := NewPosition("")
	_ = position.ApplyMoves("e2e4 e7e5 g1f3")
	previous, err := position.AtPly(1)
	if err != nil {
		t.Fatal(err)
	}
	if previous.Ply() != 1 || previous.Turn() != Black {
		t.Fatalf("unexpected timeline state: ply=%d turn=%v", previous.Ply(), previous.Turn())
	}
}

func TestPieceAtUsesChessCoordinates(t *testing.T) {
	position, err := NewPosition("")
	if err != nil {
		t.Fatal(err)
	}
	if piece := position.PieceAt(4, 0); piece.Rune != 'K' || piece.Color != White {
		t.Fatalf("e1 = %#v, want white king", piece)
	}
	if piece := position.PieceAt(4, 7); piece.Rune != 'k' || piece.Color != Black {
		t.Fatalf("e8 = %#v, want black king", piece)
	}
	if piece := position.PieceAt(4, 3); piece.Rune != 0 {
		t.Fatalf("e4 = %#v, want empty", piece)
	}
}
