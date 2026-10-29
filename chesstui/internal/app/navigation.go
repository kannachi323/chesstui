package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/kannachi323/chesstui/internal/ui/screens"
)

type focusArea uint8

const (
	focusContent focusArea = iota
	focusNavigation
)

type editorMode uint8

const (
	modeNormal editorMode = iota
	modeCommand
)

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	quitArmed := m.flash == quitWarning
	m.flash = ""
	if key == "ctrl+c" {
		return m, tea.Quit
	}
	if m.mode == modeCommand {
		return m.updateCommand(msg)
	}

	if m.help {
		// Any key closes help; ? and esc are the documented ones.
		m.help = false
		return m, nil
	}
	if m.focus == focusContent && m.screenCapturesKey(key) {
		return m, m.screens[m.active].Update(msg)
	}
	if command, handled := m.resolvePending(key); handled {
		return m, command
	}

	switch key {
	case ":":
		m.mode, m.command, m.pending = modeCommand, "", ""
		return m, nil
	case "?":
		m.help = true
		return m, nil
	case "q":
		if warning := m.quitWarning(); warning != "" && !quitArmed {
			m.flash = quitWarning
			return m, nil
		}
		return m, tea.Quit
	case "esc":
		if m.focus == focusContent {
			// Let the screen back out of a sub-view first.
			if command := m.screens[m.active].Update(msg); command != nil {
				return m, command
			}
		}
		m.focus, m.nav, m.pending = focusNavigation, m.active, ""
		return m, nil
	case "tab", "shift+tab":
		m.toggleFocus()
		return m, nil
	case "ctrl+w":
		m.pending = "ctrl+w"
		return m, nil
	case "g":
		m.pending = "g"
		return m, nil
	case "H", "shift+h":
		return m, m.switchWorkspace(-1)
	case "L", "shift+l":
		return m, m.switchWorkspace(1)
	}

	if len(key) == 1 && key[0] >= '1' && key[0] <= '7' {
		return m, m.activate(int(key[0] - '1'))
	}
	if m.focus == focusNavigation {
		return m.updateNavigation(key)
	}
	return m, m.screens[m.active].Update(msg)
}

func (m *Model) resolvePending(key string) (tea.Cmd, bool) {
	switch m.pending {
	case "g":
		m.pending = ""
		switch key {
		case "t":
			return m.switchWorkspace(1), true
		case "T", "shift+t":
			return m.switchWorkspace(-1), true
		case "g":
			m.focus, m.nav = focusNavigation, 0
		default:
			return nil, false
		}
		return nil, true
	case "ctrl+w":
		m.pending = ""
		switch key {
		case "k", "h":
			m.focus, m.nav = focusNavigation, m.active
		case "j", "l":
			m.focus = focusContent
		default:
			return nil, false
		}
		return nil, true
	}
	return nil, false
}

func (m Model) updateNavigation(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "h", "k", "left", "up":
		m.nav = (m.nav + len(m.screens) - 1) % len(m.screens)
	case "l", "right", "down":
		m.nav = (m.nav + 1) % len(m.screens)
	case "0", "home":
		m.nav = 0
	case "$", "end":
		m.nav = len(m.screens) - 1
	case "j", "enter", "space":
		m.focus = focusContent
		return m, m.activate(m.nav)
	}
	return m, nil
}

func (m Model) updateCommand(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode, m.command = modeNormal, ""
		return m, nil
	case "backspace":
		m.command = trimLastRune(m.command)
		return m, nil
	case "enter":
		return m.executeCommand()
	}
	if text := msg.Key().Text; text != "" {
		m.command += text
	}
	return m, nil
}

func (m Model) executeCommand() (tea.Model, tea.Cmd) {
	command := strings.TrimSpace(strings.TrimPrefix(m.command, ":"))
	m.mode, m.command = modeNormal, ""
	switch command {
	case "q", "quit", "qa", "qall", "q!":
		if m.quitWarning() != "" && command != "q!" {
			m.flash = "A game is in progress · :q! to quit anyway"
			return m, nil
		}
		return m, tea.Quit
	case "":
		return m, nil
	case "help", "h":
		m.help = true
	case "tn", "tabnext":
		return m, m.switchWorkspace(1)
	case "tp", "tabprevious", "tabprev":
		return m, m.switchWorkspace(-1)
	case "nav", "navigation":
		m.focus, m.nav = focusNavigation, m.active
	case "content":
		m.focus = focusContent
	default:
		for index, label := range labels {
			if strings.EqualFold(command, label) {
				return m, m.activate(index)
			}
		}
		m.flash = "Unknown command :" + command + " · ? for help"
	}
	return m, nil
}

const quitWarning = "A game is in progress · press q again to quit"

// quitWarning asks the screens whether quitting now would abandon something.
func (m Model) quitWarning() string {
	for _, screen := range m.screens {
		if guard, ok := screen.(screens.QuitGuard); ok && guard.QuitWarning() {
			return quitWarning
		}
	}
	return ""
}

// activate shows a screen and gives it a chance to load on first open.
func (m *Model) activate(index int) tea.Cmd {
	if index < 0 || index >= len(m.screens) {
		return nil
	}
	m.active, m.nav, m.help = index, index, false
	if screen, ok := m.screens[index].(screens.Activator); ok {
		return screen.Activate()
	}
	return nil
}

func (m *Model) switchWorkspace(delta int) tea.Cmd {
	return m.activate((m.active + delta + len(m.screens)) % len(m.screens))
}

func (m *Model) toggleFocus() {
	if m.focus == focusContent {
		m.focus, m.nav = focusNavigation, m.active
	} else {
		m.focus = focusContent
	}
}

func (m Model) screenCapturesKey(key string) bool {
	screen, ok := m.screens[m.active].(screens.KeyCapturer)
	return ok && screen.CapturesKey(key)
}

func trimLastRune(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return value
	}
	return string(runes[:len(runes)-1])
}
