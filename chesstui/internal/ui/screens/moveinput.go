package screens

import (
	"strings"

	chesscore "github.com/kannachi323/chesstui/internal/chess"
	"github.com/kannachi323/chesstui/internal/ui/widgets"
)

// moveInput is the shared way to enter a move on any board: type UCI or SAN
// (e2e4, Nf3, O-O), or walk a cursor with the arrow keys and press enter
// once on the piece and once on its destination.
type moveInput struct {
	text     string
	cursor   string
	selected string
}

const moveStart = "abcdefghNBRQKO"
const moveChars = "abcdefgh12345678NBRQKOnrqkx+#=-"

// capturesKey reports whether key belongs to move entry rather than to the
// app's global shortcuts (g starts "gt", for example, but is also a file).
func (m *moveInput) capturesKey(key string) bool {
	return m.text != "" || (len(key) == 1 && strings.Contains(moveStart, key))
}

// active reports whether there is anything for esc to clear.
func (m *moveInput) active() bool { return m.text != "" || m.selected != "" || m.cursor != "" }

func (m *moveInput) reset() { *m = moveInput{} }

// handle consumes key. It returns a legal UCI move when one is complete,
// an error note for an illegal attempt, and whether the key was used.
func (m *moveInput) handle(key string, position *chesscore.Position, flipped bool) (move string, problem string, used bool) {
	if position == nil {
		return "", "", false
	}
	switch key {
	case "up", "down", "left", "right":
		m.moveCursor(key, flipped)
		return "", "", true
	case "esc":
		if !m.active() {
			return "", "", false
		}
		m.reset()
		return "", "", true
	case "backspace":
		if m.text == "" {
			return "", "", false
		}
		m.text = trimLastRune(m.text)
		return "", "", true
	case "enter", "space":
		if m.text != "" {
			text := m.text
			m.text = ""
			if resolved, ok := position.ResolveMove(text); ok {
				m.selected = ""
				return resolved, "", true
			}
			return "", "“" + text + "” isn’t a legal move here", true
		}
		if m.cursor == "" {
			return "", "", key == "enter"
		}
		return m.pick(position)
	}
	if len(key) == 1 && (m.text != "" && strings.Contains(moveChars, key) || strings.Contains(moveStart, key)) {
		if len(m.text) < 8 {
			m.text += key
		}
		return "", "", true
	}
	return "", "", false
}

func (m *moveInput) pick(position *chesscore.Position) (string, string, bool) {
	file, rank, _ := chesscore.ParseSquare(m.cursor)
	piece := position.PieceAt(file, rank)
	own := piece.Rune != 0 && piece.Color == position.Turn()
	if m.selected == "" || own {
		if own && m.cursor != m.selected {
			m.selected = m.cursor
		} else {
			m.selected = ""
		}
		return "", "", true
	}
	from, to := m.selected, m.cursor
	m.selected = ""
	for _, candidate := range position.LegalMoves() {
		if candidate == from+to || candidate == from+to+"q" {
			return candidate, "", true
		}
	}
	return "", "", true
}

func (m *moveInput) moveCursor(key string, flipped bool) {
	if m.cursor == "" {
		m.cursor = "e2"
		if flipped {
			m.cursor = "e7"
		}
		return
	}
	file, rank, _ := chesscore.ParseSquare(m.cursor)
	dx, dy := 0, 0
	switch key {
	case "up":
		dy = 1
	case "down":
		dy = -1
	case "left":
		dx = -1
	case "right":
		dx = 1
	}
	if flipped {
		dx, dy = -dx, -dy
	}
	file, rank = min(7, max(0, file+dx)), min(7, max(0, rank+dy))
	m.cursor = chesscore.SquareName(file, rank)
}

// decorate copies cursor, selection and legal targets onto a board.
func (m *moveInput) decorate(board widgets.Board, position *chesscore.Position) widgets.Board {
	board.Cursor, board.Selected = m.cursor, m.selected
	if m.selected != "" && position != nil {
		for _, move := range position.LegalMoves() {
			if strings.HasPrefix(move, m.selected) {
				board.Targets = append(board.Targets, move[2:4])
			}
		}
	}
	return board
}

// prompt renders the input line under a board.
func (m *moveInput) prompt(ctx Context) string {
	caret := ctx.Theme.Accent().Render("▏")
	if m.text == "" {
		return ctx.Theme.Accent().Render("›") + " " + caret + ctx.Theme.Faint().Render("type a move or use the arrows")
	}
	return ctx.Theme.Accent().Render("›") + " " + ctx.Theme.Body().Render(m.text) + caret
}

func moveHints() []widgets.Hint {
	return []widgets.Hint{{Key: "e4 Nf3", Label: "type a move"}, {Key: "←↑↓→", Label: "cursor"}, {Key: "enter", Label: "pick/drop"}}
}
