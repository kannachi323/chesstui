package review

import (
	"math"
	"testing"
)

func TestWinPercent(t *testing.T) {
	if got := (Eval{CP: 0}).WinPercent(); got != 50 {
		t.Fatalf("even = %v", got)
	}
	if got := (Eval{CP: 300}).WinPercent(); math.Abs(got-75.1) > 0.5 {
		t.Fatalf("+3 = %v", got)
	}
	if (Eval{Mate: 3}).WinPercent() != 100 || (Eval{Mate: -1}).WinPercent() != 0 {
		t.Fatal("mate scores")
	}
}

func TestGameMarksBlundersForTheRightSide(t *testing.T) {
	evals := []Eval{
		{CP: 20, Set: true},   // start
		{CP: 30, Set: true},   // 1. white: fine
		{CP: 400, Set: true},  // 1... black blunders
		{CP: 380, Set: true},  // 2. white: fine
		{CP: 380, Set: true},  // 2... black: fine
		{CP: -150, Set: true}, // 3. white blunders
	}
	marks, summary := Game(evals, true)
	if marks[1] != Blunder || marks[4] != Blunder || marks[0] != None || marks[2] != None {
		t.Fatalf("marks = %v", marks)
	}
	if summary[0].Blunders != 1 || summary[1].Blunders != 1 || summary[0].Moves != 3 || summary[1].Moves != 2 {
		t.Fatalf("summary = %+v", summary)
	}
	if summary[0].Accuracy >= 90 || summary[0].Accuracy <= 0 {
		t.Fatalf("white accuracy = %v", summary[0].Accuracy)
	}
}

func TestClassifyThresholds(t *testing.T) {
	for drop, want := range map[float64]Mark{2: None, 6: Inaccuracy, 11: Mistake, 30: Blunder} {
		if got := Classify(drop); got != want {
			t.Fatalf("Classify(%v) = %v", drop, got)
		}
	}
	if MoveAccuracy(0) < 99 || MoveAccuracy(50) > 15 {
		t.Fatal("accuracy curve")
	}
}

func TestSpark(t *testing.T) {
	if got := string(Spark([]float64{0, 50, 100}, 3)); got != "▁▄█" {
		t.Fatalf("spark = %q", got)
	}
	if Column(2, 3, 10) != 9 {
		t.Fatal("column")
	}
}
