package widgets

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kannachi323/chesstui/internal/ui/theme"
)

const Wordmark = `█▀▀ █ █ █▀▀ █▀▀ █▀▀ ▀█▀ █ █ █
█   █▀█ █▀▀ ▀▀█ ▀▀█  █  █ █ █
▀▀▀ ▀ ▀ ▀▀▀ ▀▀▀ ▀▀▀  ▀  ▀▀▀ ▀`

// Hint is one key and what it does, shown in the footer and the help overlay.
type Hint struct {
	Key   string
	Label string
}

// Header is one row: brand, tabs, and the Lichess connection state.
func Header(t theme.Theme, labels []string, active, cursor int, navFocused bool, account string, width int) string {
	brand := t.Accent().Bold(true).Render("♞") + " " + t.Title().Render("chesstui")
	right := t.Faint().Render("○ guest")
	if account != "" {
		right = t.Good().Render("●") + " " + t.Muted().Render(account)
	}
	tab := func(i int, label string) string {
		switch {
		case navFocused && i == cursor:
			return t.Pill().Render(label)
		case i == active:
			return lipgloss.NewStyle().Bold(true).Foreground(t.AccentColor).Padding(0, 1).Render(label)
		default:
			return t.Muted().Padding(0, 1).Render(label)
		}
	}
	tabs := make([]string, len(labels))
	for i, label := range labels {
		tabs[i] = tab(i, label)
	}
	nav := strings.Join(tabs, "")
	// Narrow terminals: keep the active tab readable and number the rest.
	if lipgloss.Width(brand)+lipgloss.Width(nav)+lipgloss.Width(right)+4 > width {
		for i := range labels {
			if i != active && !(navFocused && i == cursor) {
				tabs[i] = tab(i, string(rune('1'+i)))
			}
		}
		nav = strings.Join(tabs, "")
	}
	if lipgloss.Width(brand)+lipgloss.Width(nav)+lipgloss.Width(right)+4 > width {
		right = ""
	}
	if lipgloss.Width(brand)+lipgloss.Width(nav)+2 > width {
		brand = t.Accent().Bold(true).Render("♞")
	}
	return spread(brand, nav, right, width)
}

// Footer shows contextual key hints on the left and a status note on the
// right. Hints that do not fit are dropped from the end; "? help" stays.
func Footer(t theme.Theme, hints []Hint, status string, width int) string {
	width = max(1, width-1) // never write the last cell; some terminals wrap
	help := renderHint(t, Hint{"?", "help"})
	parts := make([]string, 0, len(hints))
	used := lipgloss.Width(help)
	for _, hint := range hints {
		rendered := renderHint(t, hint)
		if used+lipgloss.Width(rendered)+3 > width*3/4 && len(parts) > 0 {
			break
		}
		parts = append(parts, rendered)
		used += lipgloss.Width(rendered) + 3
	}
	parts = append(parts, help)
	left := " " + strings.Join(parts, "   ")
	room := width - lipgloss.Width(left) - 3
	if room < 8 || status == "" {
		return ansi.Truncate(left, width, "")
	}
	return spread(left, "", ansi.Truncate(status, room, "…")+" ", width)
}

// CommandLine replaces the footer while typing a ":" command.
func CommandLine(t theme.Theme, command string, width int) string {
	line := " " + t.Accent().Bold(true).Render(":") + t.Body().Render(command) + t.Accent().Render("▏")
	tail := t.Faint().Render("enter run · esc cancel ")
	return spread(line, "", tail, max(1, width-1))
}

func renderHint(t theme.Theme, hint Hint) string {
	return t.Key().Render(hint.Key) + " " + t.Muted().Render(hint.Label)
}

// HelpPanel renders the "?" overlay: global keys, then the screen's own.
func HelpPanel(t theme.Theme, title string, global, local []Hint) string {
	column := func(heading string, hints []Hint) string {
		keyWidth := 0
		for _, hint := range hints {
			keyWidth = max(keyWidth, lipgloss.Width(hint.Key))
		}
		lines := []string{t.Faint().Render(strings.ToUpper(heading)), ""}
		for _, hint := range hints {
			lines = append(lines, t.Key().Width(keyWidth+2).Render(hint.Key)+t.Muted().Render(hint.Label))
		}
		return strings.Join(lines, "\n")
	}
	body := column(title, local)
	if len(local) == 0 {
		body = column("Everywhere", global)
	} else {
		body = lipgloss.JoinHorizontal(lipgloss.Top, body, "      ", column("Everywhere", global))
	}
	return t.Panel().Padding(1, 3).Render(body)
}

// spread lays out left, centre and right segments across width cells.
func spread(left, center, right string, width int) string {
	lw, cw, rw := lipgloss.Width(left), lipgloss.Width(center), lipgloss.Width(right)
	if center == "" {
		gap := max(1, width-lw-rw)
		return ansi.Truncate(left+strings.Repeat(" ", gap)+right, width, "")
	}
	start := max(lw+2, (width-cw)/2)
	if start+cw+rw+1 > width {
		start = max(lw+2, width-cw-rw-1)
	}
	line := left + strings.Repeat(" ", start-lw) + center
	gap := max(1, width-lipgloss.Width(line)-rw)
	return ansi.Truncate(line+strings.Repeat(" ", gap)+right, width, "")
}
