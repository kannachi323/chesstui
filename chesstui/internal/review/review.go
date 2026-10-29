// Package review turns per-move engine evaluations into the things a game
// review shows: winning chances, move marks and accuracy. The formulas follow
// the ones Lichess documents for its own game reports.
package review

import "math"

// Eval is an evaluation from White's point of view.
type Eval struct {
	CP   int
	Mate int // > 0 White mates, < 0 Black mates
	Set  bool
}

// WinPercent is White's winning chance, 0–100.
func (e Eval) WinPercent() float64 {
	if e.Mate > 0 {
		return 100
	}
	if e.Mate < 0 {
		return 0
	}
	cp := math.Max(-1000, math.Min(1000, float64(e.CP)))
	return 50 + 50*(2/(1+math.Exp(-0.00368208*cp))-1)
}

// Mark grades a move by how much winning chance the mover gave away.
type Mark int

const (
	None Mark = iota
	Inaccuracy
	Mistake
	Blunder
)

func (m Mark) Glyph() string {
	return [...]string{"", "?!", "?", "??"}[m]
}

func (m Mark) String() string {
	return [...]string{"", "inaccuracy", "mistake", "blunder"}[m]
}

// Drop is the winning chance, in percentage points, that the mover lost
// between the position before the move and the one after it.
func Drop(before, after Eval, whiteMoved bool) float64 {
	b, a := before.WinPercent(), after.WinPercent()
	if !whiteMoved {
		b, a = 100-b, 100-a
	}
	return math.Max(0, b-a)
}

// Classify grades a drop: 5, 10 and 15 points mark an inaccuracy, a mistake
// and a blunder.
func Classify(drop float64) Mark {
	switch {
	case drop >= 15:
		return Blunder
	case drop >= 10:
		return Mistake
	case drop >= 5:
		return Inaccuracy
	}
	return None
}

// MoveAccuracy maps a drop to 0–100.
func MoveAccuracy(drop float64) float64 {
	accuracy := 103.1668*math.Exp(-0.04354*drop) - 3.1669
	return math.Max(0, math.Min(100, accuracy))
}

// Summary is one side's review.
type Summary struct {
	Accuracy     float64
	Inaccuracies int
	Mistakes     int
	Blunders     int
	Moves        int
}

// Game reviews a main line. evals[i] is the evaluation after i moves;
// whiteFirst says who played move 1. It returns a mark per move and a
// summary per side (White first). Moves without both evaluations are skipped.
func Game(evals []Eval, whiteFirst bool) ([]Mark, [2]Summary) {
	var summaries [2]Summary
	if len(evals) == 0 {
		return nil, summaries
	}
	marks := make([]Mark, len(evals)-1)
	var totals [2]float64
	for i := 1; i < len(evals); i++ {
		if !evals[i-1].Set || !evals[i].Set {
			continue
		}
		white := (i%2 == 1) == whiteFirst
		side := 1
		if white {
			side = 0
		}
		drop := Drop(evals[i-1], evals[i], white)
		marks[i-1] = Classify(drop)
		totals[side] += MoveAccuracy(drop)
		summaries[side].Moves++
		switch marks[i-1] {
		case Inaccuracy:
			summaries[side].Inaccuracies++
		case Mistake:
			summaries[side].Mistakes++
		case Blunder:
			summaries[side].Blunders++
		}
	}
	for side := range summaries {
		if summaries[side].Moves > 0 {
			summaries[side].Accuracy = totals[side] / float64(summaries[side].Moves)
		}
	}
	return marks, summaries
}

// Spark draws values (0–100) as a one-row bar graph of width cells.
func Spark(values []float64, width int) []rune {
	bars := []rune("▁▂▃▄▅▆▇█")
	out := make([]rune, max(0, width))
	if len(values) == 0 {
		for i := range out {
			out[i] = ' '
		}
		return out
	}
	for col := range out {
		index := 0
		if width > 1 {
			index = int(math.Round(float64(col) * float64(len(values)-1) / float64(width-1)))
		}
		level := int(values[index] / 100 * float64(len(bars)-1))
		out[col] = bars[max(0, min(len(bars)-1, level))]
	}
	return out
}

// Column returns the graph column that shows value index i.
func Column(i, count, width int) int {
	if count <= 1 || width <= 1 {
		return 0
	}
	return int(math.Round(float64(i) * float64(width-1) / float64(count-1)))
}
