package widgets

import (
	"strings"

	"charm.land/lipgloss/v2"
	chesscore "github.com/kannachi323/chesstui/internal/chess"
	"github.com/kannachi323/chesstui/internal/ui/theme"
)

type Board struct {
	Theme      theme.Theme
	Width      int
	Height     int
	Flipped    bool
	Glyphs     string
	LastMove   string
	Cursor     string   // keyboard cursor square
	Selected   string   // picked-up piece
	Targets    []string // legal destinations of Selected
	Suggest    string   // an engine move to show, in UCI
	ShowCoords bool
}

// Square sizes in terminal cells. Terminal cells are about twice as tall as
// they are wide, so w = 2h keeps squares square. 3×1 is the compact fallback.
var squareSizes = [][2]int{{6, 3}, {4, 2}, {3, 1}}

// BoardSize returns the rendered width and height for a board that must fit
// in width × height cells.
func BoardSize(width, height int, coords bool) (int, int) {
	cw, ch, gutter := geometry(width, height, coords)
	rows := ch * 8
	if coords {
		rows++
	}
	return gutter + cw*8, rows
}

func geometry(width, height int, coords bool) (cellWidth, cellHeight, gutter int) {
	extraRows := 0
	if coords {
		gutter, extraRows = 2, 1
	}
	for _, size := range squareSizes {
		if size[0]*8+gutter <= width && size[1]*8+extraRows <= height {
			return size[0], size[1], gutter
		}
	}
	return 3, 1, gutter
}

func (b Board) Render(position *chesscore.Position) string {
	if position == nil {
		return b.Theme.Faint().Render("waiting for a position")
	}
	cellWidth, cellHeight, gutter := geometry(b.Width, b.Height, b.ShowCoords)
	files := orderedIndices(b.Flipped)
	ranks := orderedIndices(!b.Flipped)
	from, to := moveSquares(b.LastMove)
	suggestFrom, suggestTo := moveSquares(b.Suggest)
	check := ""
	if position.InCheck() {
		check = position.KingSquare(position.Turn())
	}
	targets := make(map[string]bool, len(b.Targets))
	for _, target := range b.Targets {
		targets[target] = true
	}
	glyphRow := (cellHeight - 1) / 2

	rows := make([]string, 0, cellHeight*8+1)
	for _, rank := range ranks {
		for cellRow := range cellHeight {
			var line strings.Builder
			if gutter > 0 {
				label := " "
				if cellRow == glyphRow {
					label = string(rune('1' + rank))
				}
				line.WriteString(b.Theme.Faint().Render(label + " "))
			}
			for _, file := range files {
				square := chesscore.SquareName(file, rank)
				piece := position.PieceAt(file, rank)
				style := b.squareStyle(file, rank, square, piece, from, to, check, cellWidth)
				if square == suggestFrom || square == suggestTo {
					style = style.Background(b.Theme.Suggest)
				}
				glyph := " "
				if cellRow == glyphRow {
					switch {
					case piece.Rune != 0:
						glyph = pieceGlyph(piece.Rune, b.Glyphs)
					case targets[square]:
						glyph = "•"
						style = style.Foreground(b.Theme.Hint)
					}
				}
				if targets[square] && piece.Rune != 0 {
					style = style.Background(b.Theme.Capture)
				}
				line.WriteString(style.Render(glyph))
			}
			rows = append(rows, line.String())
		}
	}
	if b.ShowCoords {
		labels := make([]string, 0, len(files))
		for _, file := range files {
			labels = append(labels, lipgloss.PlaceHorizontal(cellWidth, lipgloss.Center, string(rune('a'+file))))
		}
		rows = append(rows, b.Theme.Faint().Render(strings.Repeat(" ", gutter)+strings.Join(labels, "")))
	}
	return strings.Join(rows, "\n")
}

func (b Board) squareStyle(file, rank int, square string, piece chesscore.Piece, from, to, check string, width int) lipgloss.Style {
	light := (file+rank)%2 == 1
	background := b.Theme.BoardDark
	if light {
		background = b.Theme.BoardLight
	}
	switch {
	case square == b.Cursor:
		background = b.Theme.Cursor
	case square == b.Selected:
		background = b.Theme.Selected
	case square == check:
		background = b.Theme.Check
	case square == from || square == to:
		background = b.Theme.LastDark
		if light {
			background = b.Theme.LastLight
		}
	}
	foreground := b.Theme.WhitePiece
	if piece.Color == chesscore.Black {
		foreground = b.Theme.BlackPiece
	}
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).
		Background(background).Foreground(foreground).Bold(true)
}

func moveSquares(move string) (string, string) {
	move = strings.TrimSpace(strings.ToLower(move))
	if len(move) < 4 {
		return "", ""
	}
	return move[:2], move[2:4]
}

func orderedIndices(reverse bool) []int {
	values := make([]int, 8)
	for i := range values {
		if reverse {
			values[i] = 7 - i
		} else {
			values[i] = i
		}
	}
	return values
}

func pieceGlyph(piece rune, profile string) string {
	if piece == 0 {
		return " "
	}
	if profile == "ascii" {
		return string(piece)
	}
	if profile == "classic" {
		classic := map[rune]rune{
			'K': '♔', 'Q': '♕', 'R': '♖', 'B': '♗', 'N': '♘', 'P': '♙',
			'k': '♚', 'q': '♛', 'r': '♜', 'b': '♝', 'n': '♞', 'p': '♟',
		}
		if glyph, ok := classic[piece]; ok {
			return string(glyph)
		}
	}
	// The same solid-weight forms are used for both sides; color communicates
	// ownership. This avoids the thin outline/solid mismatch of the classic
	// Unicode white and black sets on many terminal fonts.
	solid := map[rune]rune{
		'K': '♚', 'Q': '♛', 'R': '♜', 'B': '♝', 'N': '♞', 'P': '♟',
		'k': '♚', 'q': '♛', 'r': '♜', 'b': '♝', 'n': '♞', 'p': '♟',
	}
	if glyph, ok := solid[piece]; ok {
		return string(glyph)
	}
	return string(piece)
}
