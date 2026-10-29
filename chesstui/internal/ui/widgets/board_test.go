package widgets

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	chesscore "github.com/kannachi323/chesstui/internal/chess"
	"github.com/kannachi323/chesstui/internal/ui/theme"
)

func TestBoardRendersBothOrientationsAndCoordinates(t *testing.T) {
	position, err := chesscore.NewPosition("")
	if err != nil {
		t.Fatal(err)
	}
	palette := theme.Dark()
	white := ansi.Strip(Board{Theme: palette, Width: 50, Height: 12, Glyphs: "ascii", ShowCoords: true}.Render(position))
	black := ansi.Strip(Board{Theme: palette, Width: 50, Height: 12, Glyphs: "ascii", ShowCoords: true, Flipped: true}.Render(position))
	whiteLines, blackLines := strings.Split(white, "\n"), strings.Split(black, "\n")
	if !strings.HasPrefix(whiteLines[0], "8") || !strings.Contains(whiteLines[len(whiteLines)-1], "a") {
		t.Fatalf("unexpected white orientation:\n%s", white)
	}
	if !strings.HasPrefix(blackLines[0], "1") || !strings.HasPrefix(strings.TrimSpace(blackLines[len(blackLines)-1]), "h") {
		t.Fatalf("unexpected black orientation:\n%s", black)
	}
}

func TestBoardUsesPhysicalSquareGeometry(t *testing.T) {
	position, _ := chesscore.NewPosition("")
	rendered := ansi.Strip(Board{Theme: theme.Dark(), Width: 64, Height: 30, Glyphs: "unicode", ShowCoords: true}.Render(position))
	lines := strings.Split(rendered, "\n")
	if len(lines) != 25 { // 8 ranks × 3 rows + file letters
		t.Fatalf("board height = %d, want 25\n%s", len(lines), rendered)
	}
	wantWidth := len([]rune(lines[0]))
	for index, line := range lines {
		if width := len([]rune(line)); width != wantWidth {
			t.Fatalf("line %d width = %d, want %d", index, width, wantWidth)
		}
	}
}

func TestUnicodePiecesUseConsistentSolidWeight(t *testing.T) {
	if white, black := pieceGlyph('K', "solid"), pieceGlyph('k', "solid"); white != "♚" || black != white {
		t.Fatalf("unicode kings = %q and %q", white, black)
	}
	if white, black := pieceGlyph('K', "classic"), pieceGlyph('k', "classic"); white != "♔" || black != "♚" {
		t.Fatalf("classic kings = %q and %q", white, black)
	}
}

func TestMoveSquaresHandlesPromotion(t *testing.T) {
	from, to := moveSquares("e7e8q")
	if from != "e7" || to != "e8" {
		t.Fatalf("got %s %s", from, to)
	}
}

func TestBoardSizeFitsItsBox(t *testing.T) {
	position, _ := chesscore.NewPosition("")
	for _, box := range [][2]int{{30, 10}, {40, 18}, {60, 26}, {80, 40}, {200, 60}} {
		w, h := BoardSize(box[0], box[1], true)
		rendered := Board{Theme: theme.Dark(), Width: box[0], Height: box[1], ShowCoords: true}.Render(position)
		lines := strings.Split(ansi.Strip(rendered), "\n")
		if len(lines) != h || len([]rune(lines[0])) != w {
			t.Fatalf("box %v: BoardSize says %dx%d, rendered %dx%d", box, w, h, len([]rune(lines[0])), len(lines))
		}
		if box[0] >= 26 && (w > box[0] || h > box[1]) {
			t.Fatalf("box %v: board %dx%d overflows", box, w, h)
		}
	}
}

func TestBoardMarksCheck(t *testing.T) {
	position, _ := chesscore.NewPosition("4k3/8/8/8/8/8/8/4KQ2 b - - 0 1")
	_ = position
	checked, _ := chesscore.NewPosition("4k3/8/8/8/8/8/8/4R1K1 b - - 0 1")
	palette := theme.Dark()
	plain := Board{Theme: palette, Width: 40, Height: 20}.Render(position)
	marked := Board{Theme: palette, Width: 40, Height: 20}.Render(checked)
	if plain == marked || !checked.InCheck() {
		t.Fatal("expected the checked king's square to render differently")
	}
}
