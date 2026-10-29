package screens

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kannachi323/chesstui/internal/config"
	"github.com/kannachi323/chesstui/internal/engine"
	"github.com/kannachi323/chesstui/internal/ui/widgets"
)

type settingsSavedMsg struct{ err error }
type settingsDisconnectedMsg struct{ err error }

// ThemeChangedMsg asks the app to re-resolve the light/dark theme.
type ThemeChangedMsg struct{}

const (
	settingTheme = iota
	settingGlyphs
	settingResign
	settingDepth
	settingChannel
	settingAccount
)

type Settings struct {
	ctx        Context
	selected   int
	note       note
	confirming bool
}

func NewSettings(ctx Context) *Settings { return &Settings{ctx: ctx} }
func (s *Settings) Init() tea.Cmd       { return nil }

func (s *Settings) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case settingsSavedMsg:
		if msg.err != nil {
			s.note = errNote(msg.err)
		} else {
			s.note = okNote("Saved")
		}
		return nil
	case settingsDisconnectedMsg:
		if msg.err != nil {
			s.note = errNote(msg.err)
		} else {
			s.note = okNote("Signed out · token revoked and removed")
		}
		return nil
	}
	key, ok := keyString(msg)
	if !ok {
		return nil
	}
	if key != "enter" {
		s.confirming = false
	}
	switch key {
	case "up", "k":
		s.selected = max(0, s.selected-1)
	case "down", "j":
		s.selected = min(settingAccount, s.selected+1)
	case "left", "h", "right", "l", "enter", "space":
		if s.selected == settingAccount {
			if key != "enter" || !s.ctx.Session.HasToken() {
				return nil
			}
			if !s.confirming {
				s.confirming, s.note = true, plainNote("Press enter again to sign out")
				return nil
			}
			s.confirming = false
			session := s.ctx.Session
			return func() tea.Msg { return settingsDisconnectedMsg{err: session.Disconnect(context.Background())} }
		}
		s.change(key == "left" || key == "h")
		cfg, path := s.ctx.Session.Config, s.ctx.Session.Paths.ConfigFile
		save := func() tea.Msg { return settingsSavedMsg{err: config.Save(path, cfg)} }
		if s.selected == settingTheme {
			return tea.Batch(save, func() tea.Msg { return ThemeChangedMsg{} })
		}
		return save
	}
	return nil
}

func cycle(options []string, current string, back bool) string {
	index := 0
	for i, option := range options {
		if option == current {
			index = i
		}
	}
	step := 1
	if back {
		step = len(options) - 1
	}
	return options[(index+step)%len(options)]
}

func (s *Settings) change(back bool) {
	cfg := &s.ctx.Session.Config
	switch s.selected {
	case settingTheme:
		cfg.Theme = cycle([]string{"auto", "dark", "light"}, cfg.Theme, back)
	case settingGlyphs:
		cfg.Glyphs = cycle([]string{"solid", "classic", "ascii"}, cfg.Glyphs, back)
	case settingResign:
		cfg.ConfirmResign = !cfg.ConfirmResign
	case settingDepth:
		if back {
			cfg.AnalysisDepth = max(8, cfg.AnalysisDepth-2)
		} else {
			cfg.AnalysisDepth = min(30, cfg.AnalysisDepth+2)
		}
	case settingChannel:
		cfg.DefaultWatch = cycle([]string{"best", "bullet", "blitz", "rapid", "classical", "chess960", "bot"}, cfg.DefaultWatch, back)
	}
}

func (s *Settings) View(width, height int) string {
	t := s.ctx.Theme
	cfg := s.ctx.Session.Config
	onOff := map[bool]string{true: "on", false: "off"}
	account := "not signed in"
	if current, ok := s.ctx.Session.Account(); ok {
		account = current.Username + " · sign out"
	} else if s.ctx.Session.HasToken() {
		account = "signed in · sign out"
	}
	rowWidth := min(56, width)
	items := []struct{ label, value string }{
		{"Theme", cfg.Theme},
		{"Pieces", cfg.Glyphs + "  " + sample(cfg.Glyphs)},
		{"Confirm resign", onOff[cfg.ConfirmResign]},
		{"Analysis depth", fmt.Sprint(cfg.AnalysisDepth)},
		{"TV channel", channelLabel(cfg.DefaultWatch)},
		{"Lichess", account},
	}
	rows := make([]string, len(items))
	for i, item := range items {
		value := t.Muted().Render(item.value)
		if i == s.selected {
			value = t.Accent().Render(item.value)
		}
		rows[i] = padRight(s.ctx.item(item.label, i == s.selected), rowWidth-2-lipgloss.Width(value)) + value
	}
	lines := []string{t.Title().Render("Settings"), "", s.ctx.menu(rows, s.selected, len(rows), rowWidth, true)}
	if s.note.text != "" {
		lines = append(lines, "", s.ctx.note(s.note))
	}
	if height >= 16 {
		stockfish := "not installed · brew install stockfish"
		if found, err := engine.NewStockfish(cfg.StockfishPath); err == nil {
			stockfish = found.Path()
		}
		info := func(label, value string) string {
			return t.Faint().Render(padRight(label, 11) + shortPath(value, rowWidth-13))
		}
		lines = append(lines, "", "",
			info("Stockfish", stockfish),
			info("Config", s.ctx.Session.Paths.ConfigFile),
			info("Library", s.ctx.Session.Paths.Database))
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(strings.Join(lines, "\n"))
}

// shortPath abbreviates the home directory and trims the front of long
// paths so the end, which names the file, stays visible.
func shortPath(path string, width int) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(path, home) {
		path = "~" + strings.TrimPrefix(path, home)
	}
	runes := []rune(path)
	if width > 1 && len(runes) > width {
		path = "…" + string(runes[len(runes)-width+1:])
	}
	return path
}

func sample(profile string) string {
	switch profile {
	case "classic":
		return "♔ ♚"
	case "ascii":
		return "K k"
	}
	return "♚"
}

func (s *Settings) Hints() []widgets.Hint {
	return []widgets.Hint{{Key: "↑↓", Label: "select"}, {Key: "←→", Label: "change"}, {Key: "enter", Label: "toggle"}}
}

func (s *Settings) Status() string { return "changes save automatically" }
