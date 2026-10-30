package screens

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kannachi323/chesstui/internal/config"
	"github.com/kannachi323/chesstui/internal/ui/widgets"
)

type settingsSavedMsg struct{ err error }
type settingsDisconnectedMsg struct{ err error }

// ThemeChangedMsg asks the app to re-resolve the light/dark theme.
type ThemeChangedMsg struct{}

// TabsChangedMsg asks the app to rebuild its tab bar, for example after the
// Lichess beta was switched on or off.
type TabsChangedMsg struct{}

const (
	settingTheme = iota
	settingGlyphs
	settingEngine
	settingThreads
	settingHash
	settingDepth
	settingReview
	settingBeta
	settingChannel
	settingResign
	settingAccount
	settingCount
)

type Settings struct {
	ctx        Context
	selected   int
	note       note
	confirming bool
	editing    *textInput // engine path
}

func NewSettings(ctx Context) *Settings { return &Settings{ctx: ctx} }
func (s *Settings) Init() tea.Cmd       { return nil }

func (s *Settings) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.PasteMsg:
		if s.editing != nil {
			s.editing.paste(msg.Content)
		}
		return nil
	case settingsSavedMsg:
		if msg.err != nil {
			s.note = errNote(msg.err)
		} else if s.note.text == "" || s.note.tone != bad {
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
	if s.editing != nil {
		switch key {
		case "esc":
			s.editing = nil
		case "enter":
			s.ctx.Session.Config.EnginePath = cleanPath(s.editing.value)
			s.editing = nil
			s.note = note{}
			if _, err := s.ctx.Session.Engine(); err != nil {
				s.note = errNote(err)
			}
			return s.save()
		default:
			s.editing.handle(key)
		}
		return nil
	}
	if key != "enter" {
		s.confirming = false
	}
	switch key {
	case "up", "k":
		s.selected = max(0, s.selected-1)
	case "down", "j":
		s.selected = min(settingCount-1, s.selected+1)
	case "left", "h", "right", "l", "enter", "space":
		switch s.selected {
		case settingAccount:
			return s.signOut(key)
		case settingEngine:
			if api := s.ctx.Session.Config.EngineAPI; api != "" {
				s.note = plainNote("Using the engine at " + api)
				return nil
			}
			if key == "enter" || key == "space" {
				s.editing = &textInput{value: s.ctx.Session.Config.EnginePath}
			}
			return nil
		}
		s.change(key == "left" || key == "h")
		commands := []tea.Cmd{s.save()}
		switch s.selected {
		case settingTheme:
			commands = append(commands, func() tea.Msg { return ThemeChangedMsg{} })
		case settingBeta:
			commands = append(commands, func() tea.Msg { return TabsChangedMsg{} })
		}
		return tea.Batch(commands...)
	}
	return nil
}

func (s *Settings) save() tea.Cmd {
	cfg, path := s.ctx.Session.Config, s.ctx.Session.Paths.ConfigFile
	return func() tea.Msg { return settingsSavedMsg{err: config.Save(path, cfg)} }
}

func (s *Settings) signOut(key string) tea.Cmd {
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

func step(value, delta, low, high int, back bool) int {
	if back {
		return max(low, value-delta)
	}
	return min(high, value+delta)
}

func (s *Settings) change(back bool) {
	cfg := &s.ctx.Session.Config
	switch s.selected {
	case settingTheme:
		cfg.Theme = cycle([]string{"auto", "dark", "light"}, cfg.Theme, back)
	case settingGlyphs:
		cfg.Glyphs = cycle([]string{"solid", "classic", "ascii"}, cfg.Glyphs, back)
	case settingThreads:
		cfg.EngineThreads = step(cfg.EngineThreads, 1, 1, 64, back)
	case settingHash:
		if back {
			cfg.EngineHashMB = max(16, cfg.EngineHashMB/2)
		} else {
			cfg.EngineHashMB = min(4096, cfg.EngineHashMB*2)
		}
	case settingDepth:
		cfg.AnalysisDepth = step(cfg.AnalysisDepth, 2, 8, 40, back)
	case settingReview:
		cfg.ReviewDepth = step(cfg.ReviewDepth, 2, 6, 30, back)
	case settingBeta:
		cfg.LichessBeta = !cfg.LichessBeta && !config.Browser
	case settingChannel:
		cfg.DefaultWatch = cycle([]string{"best", "bullet", "blitz", "rapid", "classical", "chess960", "bot"}, cfg.DefaultWatch, back)
	case settingResign:
		cfg.ConfirmResign = !cfg.ConfirmResign
	}
}

func (s *Settings) View(width, height int) string {
	t := s.ctx.Theme
	cfg := s.ctx.Session.Config
	onOff := map[bool]string{true: "on", false: "off"}
	rowWidth := min(60, width)

	engineValue := "not found"
	if found, err := s.ctx.Session.Engine(); err == nil {
		engineValue = found.Name()
	}
	account := "not signed in"
	if current, ok := s.ctx.Session.Account(); ok {
		account = current.Username + " · sign out"
	} else if s.ctx.Session.HasToken() {
		account = "signed in · sign out"
	}
	// Rows that do nothing in the current setup are dimmed: Lichess rows
	// with the beta off, process settings when an HTTP engine is in use.
	remote := cfg.EngineAPI != ""
	type item struct {
		label, value string
		off          bool
	}
	items := []item{
		{"Theme", cfg.Theme, false},
		{"Pieces", cfg.Glyphs + "  " + sample(cfg.Glyphs), false},
		{"Engine", engineValue, false},
		{"Engine threads", fmt.Sprint(cfg.EngineThreads), remote},
		{"Engine memory", fmt.Sprintf("%d MB", cfg.EngineHashMB), remote},
		{"Analysis depth", fmt.Sprint(cfg.AnalysisDepth), false},
		{"Review depth", fmt.Sprint(cfg.ReviewDepth), false},
		{"Lichess beta", betaValue(cfg.LichessBeta), false},
		{"TV channel", channelLabel(cfg.DefaultWatch), !cfg.LichessBeta},
		{"Confirm resign", onOff[cfg.ConfirmResign], !cfg.LichessBeta},
		{"Lichess account", account, !cfg.LichessBeta},
	}
	rows := make([]string, len(items))
	for i, it := range items {
		value := t.Muted().Render(it.value)
		if i == s.selected {
			value = t.Accent().Render(it.value)
		}
		label := s.ctx.item(it.label, i == s.selected)
		if it.off {
			label, value = t.Faint().Render(it.label), t.Faint().Render(it.value)
		}
		rows[i] = padRight(label, rowWidth-2-lipgloss.Width(value)) + value
	}
	lines := []string{t.Title().Render("Settings"), "", s.ctx.menu(rows, s.selected, len(rows), rowWidth, true)}
	if s.editing != nil {
		lines = append(lines, "", s.ctx.field("Engine path", *s.editing), t.Faint().Render("any UCI engine · empty finds Stockfish · enter save · esc cancel"))
	} else if s.note.text != "" {
		lines = append(lines, "", s.ctx.note(s.note))
	}
	if height >= 24 && s.editing == nil {
		enginePath := "brew install stockfish, or set a path above"
		if found, err := s.ctx.Session.Engine(); err == nil {
			enginePath = found.Path()
		}
		info := func(label, value string) string {
			return t.Faint().Render(padRight(label, 9) + shortPath(value, rowWidth-11))
		}
		lines = append(lines, "",
			info("Engine", enginePath),
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
	if s.editing != nil {
		return []widgets.Hint{{Key: "enter", Label: "save"}, {Key: "esc", Label: "cancel"}}
	}
	return []widgets.Hint{{Key: "↑↓", Label: "select"}, {Key: "←→", Label: "change"}, {Key: "enter", Label: "edit"}}
}

func (s *Settings) Status() string { return "changes save automatically" }

func (s *Settings) CapturesKey(string) bool { return s.editing != nil }

func betaValue(on bool) string {
	switch {
	case config.Browser:
		return "in the terminal app"
	case on:
		return "on"
	}
	return "off"
}
