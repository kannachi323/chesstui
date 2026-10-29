package screens

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kannachi323/chesstui/internal/ui/widgets"
)

// Lichess groups everything that needs lichess.org (TV, a player's games
// and studies, online play) in one tab marked beta. Sections switch with
// < and >.
type Lichess struct {
	ctx      Context
	sections []Screen
	labels   []string
	active   int
}

func NewLichess(ctx Context) *Lichess {
	return &Lichess{
		ctx:      ctx,
		sections: []Screen{NewWatch(ctx), NewStudy(ctx), NewPlay(ctx)},
		labels:   []string{"TV", "Games", "Play online"},
	}
}

func (l *Lichess) Init() tea.Cmd { return nil }

func (l *Lichess) Activate() tea.Cmd { return activate(l.sections[l.active]) }

func activate(screen Screen) tea.Cmd {
	if activator, ok := screen.(Activator); ok {
		return activator.Activate()
	}
	return nil
}

// Update forwards results to every section, so a stream keeps flowing while
// another section is showing; keys go to the visible section only.
func (l *Lichess) Update(msg tea.Msg) tea.Cmd {
	if key, ok := keyString(msg); ok {
		current := l.sections[l.active]
		// < and > never appear in a username, a path or a move, so they
		// always switch sections, even from a text field.
		if key == "<" || key == ">" {
			delta := 1
			if key == "<" {
				delta = len(l.sections) - 1
			}
			l.active = (l.active + delta) % len(l.sections)
			return activate(l.sections[l.active])
		}
		return current.Update(msg)
	}
	if _, ok := msg.(tea.PasteMsg); ok {
		return l.sections[l.active].Update(msg)
	}
	var commands []tea.Cmd
	for _, section := range l.sections {
		if command := section.Update(msg); command != nil {
			commands = append(commands, command)
		}
	}
	return tea.Batch(commands...)
}

func (l *Lichess) View(width, height int) string {
	t := l.ctx.Theme
	head := l.ctx.tabs(l.labels, l.active) + "   " + t.Faint().Render("beta")
	body := l.sections[l.active].View(width, max(1, height-2))
	return lipgloss.JoinVertical(lipgloss.Center, head, "", body)
}

func (l *Lichess) Hints() []widgets.Hint {
	return append([]widgets.Hint{{Key: "< >", Label: "section"}}, l.sections[l.active].Hints()...)
}

func (l *Lichess) Status() string { return l.sections[l.active].Status() }

func (l *Lichess) CapturesKey(key string) bool {
	if capturer, ok := l.sections[l.active].(KeyCapturer); ok && capturer.CapturesKey(key) {
		return true
	}
	return key == "<" || key == ">"
}

// QuitWarning holds the app open during a live online game.
func (l *Lichess) QuitWarning() bool {
	for _, section := range l.sections {
		if guard, ok := section.(QuitGuard); ok && guard.QuitWarning() {
			return true
		}
	}
	return false
}
