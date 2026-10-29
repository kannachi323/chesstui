package screens

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kannachi323/chesstui/internal/lichess"
	"github.com/kannachi323/chesstui/internal/session"
	"github.com/kannachi323/chesstui/internal/ui/theme"
	"github.com/kannachi323/chesstui/internal/ui/widgets"
)

type Context struct {
	Session *session.Session
	// Theme is shared so a light/dark switch reaches every screen at once.
	Theme *theme.Theme
}

// Screen is an independently testable section of the application.
type Screen interface {
	Init() tea.Cmd
	Update(tea.Msg) tea.Cmd
	View(width, height int) string
	// Hints are the screen's keys, most useful first. The footer shows as
	// many as fit; the help overlay shows all of them.
	Hints() []widgets.Hint
	// Status is a short note for the right side of the footer, or "".
	Status() string
}

// Activator is implemented by screens that load data the first time they
// are opened, so start-up does not fire every Lichess request at once.
type Activator interface {
	Activate() tea.Cmd
}

// KeyCapturer is implemented by screens that sometimes need keys the app
// would otherwise treat as global shortcuts (text fields, move entry).
type KeyCapturer interface {
	CapturesKey(key string) bool
}

func keyString(msg tea.Msg) (string, bool) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return "", false
	}
	return key.String(), true
}

type tone uint8

const (
	plain tone = iota
	good
	bad
	busy
)

// note is a one-line status message with a tone.
type note struct {
	text string
	tone tone
}

func okNote(format string, args ...any) note { return note{fmt.Sprintf(format, args...), good} }
func busyNote(text string) note              { return note{text, busy} }
func plainNote(text string) note             { return note{text, plain} }
func errNote(err error) note                 { return note{lichess.Describe(err), bad} }
func badNote(text string) note               { return note{text, bad} }

func (c Context) note(n note) string {
	switch n.tone {
	case good:
		return c.Theme.Good().Render("✓ " + n.text)
	case bad:
		return c.Theme.Bad().Render(n.text)
	case busy:
		return c.Theme.Muted().Render("… " + n.text)
	}
	return c.Theme.Muted().Render(n.text)
}

// menu renders a vertical list with a "›" marker on the selected row.
// Rows scroll so the selection stays visible within height rows.
func (c Context) menu(rows []string, selected, height, width int, focused bool) string {
	if len(rows) == 0 {
		return ""
	}
	height = max(1, height)
	start := 0
	if selected >= height {
		start = selected - height + 1
	}
	end := min(len(rows), start+height)
	lines := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		row := ansi.Truncate(rows[i], max(1, width-2), "…")
		if i == selected && focused {
			lines = append(lines, c.Theme.Accent().Bold(true).Render("›")+" "+row)
		} else {
			lines = append(lines, "  "+row)
		}
	}
	return strings.Join(lines, "\n")
}

// item styles a row's main label, bold when the row is selected.
func (c Context) item(label string, selected bool) string {
	if selected {
		return c.Theme.Title().Render(label)
	}
	return c.Theme.Body().Render(label)
}

// tabs renders a row of section names with the active one in the accent.
func (c Context) tabs(labels []string, active int) string {
	parts := make([]string, len(labels))
	for i, label := range labels {
		if i == active {
			parts[i] = c.Theme.Accent().Bold(true).Render(label)
		} else {
			parts[i] = c.Theme.Faint().Render(label)
		}
	}
	return strings.Join(parts, c.Theme.Faint().Render("  ·  "))
}

// boardAndSide places a board with a side panel to its right when there is
// room, or the panel's compact form under the board otherwise. It returns the
// board widget sized for the space left over.
func boardAndSide(board widgets.Board, render func(widgets.Board) string, side func(width int) string, compact func(width int) string, width, height int) string {
	const gap, sideMin = 4, 28
	board.ShowCoords = height >= 12
	boardWidth, _ := widgets.BoardSize(width-gap-sideMin, height, board.ShowCoords)
	if width-gap-boardWidth >= sideMin && width >= 60 {
		board.Width, board.Height = width-gap-sideMin, height
		rendered := render(board)
		sideWidth := min(48, width-gap-lipgloss.Width(rendered))
		return lipgloss.JoinHorizontal(lipgloss.Top, rendered, strings.Repeat(" ", gap), lipgloss.NewStyle().Width(sideWidth).MaxHeight(height).Render(side(sideWidth)))
	}
	below := compact(width)
	board.Width, board.Height = width, max(8, height-lipgloss.Height(below)-1)
	return render(board) + "\n\n" + below
}

// moveList renders SAN moves as numbered pairs, keeping the latest visible.
func (c Context) moveList(moves []string, current, rows, firstMoveNumber int, blackFirst bool) string {
	if len(moves) == 0 || rows <= 0 {
		return ""
	}
	type pair struct{ number, white, black int }
	pairs := []pair{}
	index, number := 0, firstMoveNumber
	if blackFirst {
		pairs = append(pairs, pair{number, -1, 0})
		index, number = 1, number+1
	}
	for ; index < len(moves); index += 2 {
		p := pair{number, index, -1}
		if index+1 < len(moves) {
			p.black = index + 1
		}
		pairs = append(pairs, p)
		number++
	}
	if len(pairs) > rows {
		// Keep the current move in view, a little above the bottom.
		end := len(pairs)
		if current >= 0 && current < len(moves)-1 {
			row := 0
			for i, p := range pairs {
				if p.white == current || p.black == current {
					row = i
				}
			}
			end = min(len(pairs), max(rows, row+rows/2+1))
		}
		pairs = pairs[end-rows : end]
	}
	cell := func(i int) string {
		if i < 0 || i >= len(moves) {
			return fmt.Sprintf("%-8s", "")
		}
		text := fmt.Sprintf("%-8s", moves[i])
		if i == current {
			return c.Theme.Accent().Bold(true).Render(text)
		}
		return c.Theme.Body().Render(text)
	}
	lines := make([]string, 0, len(pairs))
	for _, p := range pairs {
		if p.white == -1 {
			lines = append(lines, c.Theme.Faint().Render(fmt.Sprintf("%3d. ", p.number))+cell(-1)+cell(0))
			continue
		}
		lines = append(lines, c.Theme.Faint().Render(fmt.Sprintf("%3d. ", p.number))+cell(p.white)+cell(p.black))
	}
	return strings.Join(lines, "\n")
}

// playerLine renders "● Name  1850        04:59".
func (c Context) playerLine(white bool, name string, rating int, clock string, toMove bool, width int) string {
	dot := c.Theme.Faint().Render("○")
	if white {
		dot = c.Theme.Body().Render("●")
	}
	left := dot + " " + c.Theme.Title().Render(name)
	if rating > 0 {
		left += " " + c.Theme.Muted().Render(fmt.Sprint(rating))
	}
	right := ""
	if clock != "" {
		style := c.Theme.Muted()
		if toMove {
			style = c.Theme.Accent().Bold(true)
		}
		right = style.Render(clock)
	}
	gap := max(1, width-lipgloss.Width(left)-lipgloss.Width(right))
	return left + strings.Repeat(" ", gap) + right
}

func trimLastRune(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return value
	}
	return string(runes[:len(runes)-1])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// textInput is a minimal single-line field for usernames and paths.
type textInput struct {
	value   string
	allowed func(string) bool
}

func (t *textInput) handle(key string) bool {
	switch key {
	case "backspace":
		t.value = trimLastRune(t.value)
	case "space":
		if t.allowed == nil || t.allowed(" ") {
			t.value += " "
		}
	default:
		if len([]rune(key)) != 1 || (t.allowed != nil && !t.allowed(key)) {
			return false
		}
		t.value += key
	}
	return true
}

func (c Context) field(label string, t textInput) string {
	return c.Theme.Muted().Render(label) + " " + c.Theme.Body().Render(t.value) + c.Theme.Accent().Render("▏")
}

func usernameChar(key string) bool {
	return len(key) == 1 && strings.Contains("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-", key)
}
