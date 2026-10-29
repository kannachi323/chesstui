package screens

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	chesscore "github.com/kannachi323/chesstui/internal/chess"
	"github.com/kannachi323/chesstui/internal/ui/widgets"
)

type lesson struct {
	Title string
	Goal  string
	FEN   string
	Moves []string // accepted answers in UCI
	Mate  bool     // any checkmating move is also accepted
	Last  string   // the move that led here, highlighted on the board
}

var lessons = []lesson{
	{"Claim the center", "Push the king’s pawn two squares.", "4k3/8/8/8/8/8/4P3/4K3 w - - 0 1", []string{"e2e4"}, false, ""},
	{"Develop a knight", "Bring the g8 knight toward the center.", "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq - 0 1", []string{"g8f6"}, false, "e2e4"},
	{"Castle", "Tuck the king away on the kingside.", "r3k2r/pppq1ppp/2npbn2/8/8/2NPBN2/PPPQ1PPP/R3K2R w KQkq - 0 1", []string{"e1g1"}, false, ""},
	{"En passant", "Capture the pawn that just jumped past you.", "4k3/8/8/3pP3/8/8/8/4K3 w - d6 0 1", []string{"e5d6"}, false, "d7d5"},
	{"Promote", "Walk the pawn to the last rank and make a queen.", "8/P7/8/8/8/8/k7/4K3 w - - 0 1", []string{"a7a8q"}, false, ""},
	{"Knight fork", "Attack the king and the rook at once.", "r3k3/8/8/1N6/8/8/8/4K3 w - - 0 1", []string{"b5c7"}, false, ""},
	{"Back-rank mate", "The king is trapped by its own pawns.", "6k1/5ppp/8/8/8/8/8/R5K1 w - - 0 1", []string{"a1a8"}, true, ""},
	{"Queen and king", "Deliver mate with the queen, guarded by your king.", "7k/5K2/8/6Q1/8/8/8/8 w - - 0 1", []string{"g5g7"}, true, ""},
}

type Learn struct {
	ctx      Context
	selected int
	position *chesscore.Position
	input    moveInput
	note     note
	done     map[int]bool
}

func NewLearn(ctx Context) *Learn {
	l := &Learn{ctx: ctx, done: map[int]bool{}}
	l.reset()
	return l
}

func (l *Learn) Init() tea.Cmd { return nil }

func (l *Learn) Update(msg tea.Msg) tea.Cmd {
	key, ok := keyString(msg)
	if !ok {
		return nil
	}
	solved := l.done[l.selected] && l.position.Ply() > 0
	if !solved {
		if move, problem, used := l.input.handle(key, l.position, l.position.Turn() == chesscore.Black); used {
			if problem != "" {
				l.note = badNote(problem)
			}
			if move != "" {
				l.submit(move)
			}
			return nil
		}
	}
	switch key {
	case "k":
		if l.selected > 0 {
			l.selected--
			l.reset()
		}
	case "j", "n":
		if l.selected < len(lessons)-1 {
			l.selected++
			l.reset()
		}
	case "r":
		l.reset()
	}
	return nil
}

func (l *Learn) reset() {
	l.position, _ = chesscore.NewPosition(lessons[l.selected].FEN)
	l.input, l.note = moveInput{}, note{}
}

func (l *Learn) submit(move string) {
	current := lessons[l.selected]
	accepted := false
	for _, want := range current.Moves {
		accepted = accepted || move == want
	}
	clone := l.position.Clone()
	if err := clone.ApplyUCI(move); err != nil {
		l.note = errNote(err)
		return
	}
	if !accepted && current.Mate {
		accepted = strings.HasPrefix(clone.Outcome(), "Checkmate")
	}
	if !accepted {
		l.note = badNote(l.position.SAN(move) + " is legal, but not the idea · try again")
		return
	}
	san := l.position.SAN(move)
	l.position, l.input = clone, moveInput{}
	l.done[l.selected] = true
	l.note = okNote("%s · nicely done", san)
	if l.selected < len(lessons)-1 {
		l.note.text += " · n next"
	}
}

func (l *Learn) View(width, height int) string {
	t := l.ctx.Theme
	rows := make([]string, len(lessons))
	for i, item := range lessons {
		mark := t.Faint().Render("○ ")
		if l.done[i] {
			mark = t.Good().Render("● ")
		}
		rows[i] = mark + l.ctx.item(item.Title, i == l.selected)
	}
	current := lessons[l.selected]
	board := l.input.decorate(widgets.Board{
		Theme: *t, Flipped: chesscore.NewPositionTurn(current.FEN) == chesscore.Black, Glyphs: l.ctx.Session.Config.Glyphs, LastMove: firstNonEmpty(lastMove(l.position), current.Last),
	}, l.position)
	side := func(w int) string {
		lines := []string{t.Faint().Render(fmt.Sprintf("Lesson %d of %d", l.selected+1, len(lessons))), t.Title().Render(current.Title), "",
			lipgloss.NewStyle().Width(w).Foreground(t.MutedColor).Render(current.Goal), ""}
		if !(l.done[l.selected] && l.position.Ply() > 0) {
			lines = append(lines, l.input.prompt(l.ctx))
		}
		if l.note.text != "" {
			lines = append(lines, lipgloss.NewStyle().Width(w).Render(l.ctx.note(l.note)))
		}
		return strings.Join(lines, "\n")
	}
	compact := func(w int) string {
		lines := []string{t.Title().Render(current.Title) + "  " + t.Muted().Render(current.Goal), l.input.prompt(l.ctx)}
		if l.note.text != "" {
			lines = append(lines, l.ctx.note(l.note))
		}
		return strings.Join(lines, "\n")
	}
	render := func(b widgets.Board) string { return b.Render(l.position) }
	listWidth := 22
	if width < 100 {
		return boardAndSide(board, render, side, compact, width, height)
	}
	list := lipgloss.NewStyle().Width(listWidth).Render(t.Title().Render("Lessons") + "\n\n" + l.ctx.menu(rows, l.selected, height-2, listWidth, true))
	return lipgloss.JoinHorizontal(lipgloss.Top, list, "    ", boardAndSide(board, render, side, compact, width-listWidth-4, height))
}

func (l *Learn) Hints() []widgets.Hint {
	return append(moveHints(), widgets.Hint{Key: "j/k", Label: "lesson"}, widgets.Hint{Key: "r", Label: "reset"})
}

func (l *Learn) Status() string {
	return fmt.Sprintf("%d of %d lessons done", len(l.done), len(lessons))
}

func (l *Learn) CapturesKey(key string) bool { return l.input.capturesKey(key) }
