// Package theme holds ChessTUI's one colour system. It never paints the
// terminal background, so the same UI reads well on light and dark terminals;
// only foreground tones change between the two variants.
package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

type Theme struct {
	Dark bool

	Text        color.Color
	MutedColor  color.Color
	FaintColor  color.Color
	AccentColor color.Color
	OnFill      color.Color // text drawn on an Accent fill
	Success     color.Color
	Warning     color.Color
	Danger      color.Color

	BoardLight color.Color
	BoardDark  color.Color
	LastLight  color.Color
	LastDark   color.Color
	Check      color.Color
	Cursor     color.Color
	Selected   color.Color
	Hint       color.Color // legal-move dots
	Capture    color.Color // a legal move lands on an enemy piece
	WhitePiece color.Color
	BlackPiece color.Color
}

// Board tones are mid-luminance on purpose: solid white and solid black
// glyphs both keep at least 3:1 contrast on either square colour.
func board(t Theme) Theme {
	t.BoardLight = lipgloss.Color("#A9B497")
	t.BoardDark = lipgloss.Color("#6F8063")
	t.LastLight = lipgloss.Color("#C9C774")
	t.LastDark = lipgloss.Color("#9A9A4A")
	t.Check = lipgloss.Color("#C8574C")
	t.Cursor = lipgloss.Color("#7FA2C2")
	t.Selected = lipgloss.Color("#D6A64F")
	t.Hint = lipgloss.Color("#3E4A38")
	t.Capture = lipgloss.Color("#C98F6B")
	t.WhitePiece = lipgloss.Color("#FFFDF7")
	t.BlackPiece = lipgloss.Color("#16181A")
	return t
}

func Dark() Theme {
	return board(Theme{
		Dark:        true,
		Text:        lipgloss.Color("#E7E4DC"),
		MutedColor:  lipgloss.Color("#8F8C84"),
		FaintColor:  lipgloss.Color("#57554F"),
		AccentColor: lipgloss.Color("#E2B55C"),
		OnFill:      lipgloss.Color("#17150F"),
		Success:     lipgloss.Color("#9CC48A"),
		Warning:     lipgloss.Color("#E2B55C"),
		Danger:      lipgloss.Color("#E5877C"),
	})
}

func Light() Theme {
	return board(Theme{
		Text:        lipgloss.Color("#22211D"),
		MutedColor:  lipgloss.Color("#6B6860"),
		FaintColor:  lipgloss.Color("#AAA69C"),
		AccentColor: lipgloss.Color("#93620A"),
		OnFill:      lipgloss.Color("#FFFDF7"),
		Success:     lipgloss.Color("#3F7A2E"),
		Warning:     lipgloss.Color("#93620A"),
		Danger:      lipgloss.Color("#B23B2E"),
	})
}

// For picks the variant for a terminal background, honouring an explicit
// "light" or "dark" preference from the config file.
func For(preference string, terminalIsDark bool) Theme {
	switch preference {
	case "light":
		return Light()
	case "dark":
		return Dark()
	}
	if terminalIsDark {
		return Dark()
	}
	return Light()
}

func (t Theme) Title() lipgloss.Style  { return lipgloss.NewStyle().Bold(true).Foreground(t.Text) }
func (t Theme) Body() lipgloss.Style   { return lipgloss.NewStyle().Foreground(t.Text) }
func (t Theme) Muted() lipgloss.Style  { return lipgloss.NewStyle().Foreground(t.MutedColor) }
func (t Theme) Faint() lipgloss.Style  { return lipgloss.NewStyle().Foreground(t.FaintColor) }
func (t Theme) Accent() lipgloss.Style { return lipgloss.NewStyle().Foreground(t.AccentColor) }
func (t Theme) Key() lipgloss.Style    { return lipgloss.NewStyle().Bold(true).Foreground(t.Text) }
func (t Theme) Good() lipgloss.Style   { return lipgloss.NewStyle().Foreground(t.Success) }
func (t Theme) Bad() lipgloss.Style    { return lipgloss.NewStyle().Foreground(t.Danger) }

// Pill is a filled accent label, used for the active tab and selections.
func (t Theme) Pill() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(t.OnFill).Background(t.AccentColor).Padding(0, 1)
}

// Panel is the one bordered container style.
func (t Theme) Panel() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.FaintColor).
		Padding(0, 2)
}
