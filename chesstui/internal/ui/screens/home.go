package screens

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kannachi323/chesstui/internal/domain"
	"github.com/kannachi323/chesstui/internal/lichess"
	"github.com/kannachi323/chesstui/internal/ui/widgets"
)

type homeLoadedMsg struct {
	account *lichess.Account
	stats   domain.PuzzleStats
	count   int
	err     error
}

// OpenScreenMsg asks the app to switch to a screen by index.
type OpenScreenMsg struct{ Index int }

var homeItems = []struct {
	label, detail string
	screen        int
}{
	{"Play", "Lichess or Stockfish", 1},
	{"Watch", "Lichess TV", 2},
	{"Puzzles", "daily and packs", 3},
	{"Study", "replay and analyse", 4},
	{"Learn", "the basics", 5},
}

type Home struct {
	ctx      Context
	loaded   bool
	selected int
	account  *lichess.Account
	stats    domain.PuzzleStats
	count    int
	err      error
}

func NewHome(ctx Context) *Home { return &Home{ctx: ctx} }

func (h *Home) Init() tea.Cmd { return h.load }

// load reads local stats and, only when a token exists, the Lichess account.
func (h *Home) load() tea.Msg {
	ctx := context.Background()
	message := homeLoadedMsg{}
	if account, ok := h.ctx.Session.Account(); ok {
		message.account = &account
	} else if h.ctx.Session.HasToken() {
		account, err := h.ctx.Session.RefreshAccount(ctx)
		if err == nil {
			message.account = &account
		} else {
			message.err = err
		}
	}
	stats, err := h.ctx.Session.Store.PuzzleStats(ctx)
	if err == nil {
		message.stats = stats
		message.count, err = h.ctx.Session.Store.PuzzleCount(ctx)
	}
	if message.err == nil {
		message.err = err
	}
	return message
}

func (h *Home) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case homeLoadedMsg:
		h.loaded, h.account, h.stats, h.count, h.err = true, msg.account, msg.stats, msg.count, msg.err
		return nil
	case loginFinishedMsg, settingsDisconnectedMsg, puzzleRecordedMsg, puzzleImportedMsg:
		return h.load
	}
	key, ok := keyString(msg)
	if !ok {
		return nil
	}
	switch key {
	case "up", "k":
		h.selected = max(0, h.selected-1)
	case "down", "j":
		h.selected = min(len(homeItems)-1, h.selected+1)
	case "enter", "space", "l", "right":
		index := homeItems[h.selected].screen
		return func() tea.Msg { return OpenScreenMsg{Index: index} }
	}
	return nil
}

func (h *Home) View(width, height int) string {
	t := h.ctx.Theme
	rowWidth := 36
	rows := make([]string, len(homeItems))
	for i, item := range homeItems {
		label := h.ctx.item(item.label, i == h.selected)
		detail := t.Faint().Render(item.detail)
		rows[i] = padRight(label, rowWidth-2-lipgloss.Width(detail)) + detail
	}
	wordmark := t.Accent().Render(widgets.Wordmark)
	if width < 40 || height < 18 {
		wordmark = t.Accent().Bold(true).Render("♞ ") + t.Title().Render("chesstui")
	}
	sections := []string{
		wordmark,
		t.Faint().Render("chess in the terminal, on Lichess"),
		h.ctx.menu(rows, h.selected, len(rows), rowWidth, true),
	}
	if h.loaded {
		stats := fmt.Sprintf("%d solved · streak %d · best %d", h.stats.Solved, h.stats.Streak, h.stats.Best)
		if h.count > 0 {
			stats += fmt.Sprintf(" · %d saved", h.count)
		}
		sections = append(sections, t.Faint().Render(stats))
		if h.err != nil {
			sections = append(sections, h.ctx.note(errNote(h.err)))
		}
	}
	gap := "\n\n"
	if height < 20 {
		gap = "\n"
	}
	block := strings.Join(sections, gap)
	return lipgloss.NewStyle().Width(min(width, 48)).Align(lipgloss.Center).Render(block)
}

func (h *Home) Hints() []widgets.Hint {
	return []widgets.Hint{{Key: "↑↓", Label: "choose"}, {Key: "enter", Label: "open"}, {Key: "1–7", Label: "jump"}}
}

func (h *Home) Status() string { return "" }
