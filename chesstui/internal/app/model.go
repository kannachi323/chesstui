package app

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kannachi323/chesstui/internal/session"
	"github.com/kannachi323/chesstui/internal/ui/screens"
	"github.com/kannachi323/chesstui/internal/ui/theme"
	"github.com/kannachi323/chesstui/internal/ui/widgets"
)

var labels = []string{"Home", "Play", "Watch", "Puzzles", "Study", "Learn", "Settings"}

// Smallest terminal the layout supports; below it a resize prompt is shown.
const minWidth, minHeight = 40, 12

type Model struct {
	session  *session.Session
	theme    *theme.Theme
	darkTerm bool
	screens  []screens.Screen
	active   int
	nav      int
	focus    focusArea
	mode     editorMode
	command  string
	pending  string
	width    int
	height   int
	help     bool
}

func New(current *session.Session) Model {
	palette := theme.For(current.Config.Theme, true)
	ctx := screens.Context{Session: current, Theme: &palette}
	return Model{
		session:  current,
		theme:    &palette,
		darkTerm: true,
		screens: []screens.Screen{
			screens.NewHome(ctx),
			screens.NewPlay(ctx),
			screens.NewWatch(ctx),
			screens.NewPuzzles(ctx),
			screens.NewStudy(ctx),
			screens.NewLearn(ctx),
			screens.NewSettings(ctx),
		},
	}
}

func (m Model) Init() tea.Cmd {
	commands := []tea.Cmd{tea.RequestBackgroundColor}
	for _, screen := range m.screens {
		commands = append(commands, screen.Init())
	}
	return tea.Batch(commands...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.BackgroundColorMsg:
		m.darkTerm = msg.IsDark()
		*m.theme = theme.For(m.session.Config.Theme, m.darkTerm)
		return m, nil
	case screens.ThemeChangedMsg:
		*m.theme = theme.For(m.session.Config.Theme, m.darkTerm)
		return m, nil
	case screens.OpenScreenMsg:
		m.focus = focusContent
		return m, m.activate(msg.Index)
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	// Commands can finish after the user changes sections. Let every screen
	// inspect non-key messages so its own asynchronous result is never lost.
	commands := make([]tea.Cmd, 0, len(m.screens))
	for _, screen := range m.screens {
		if command := screen.Update(msg); command != nil {
			commands = append(commands, command)
		}
	}
	return m, tea.Batch(commands...)
}

func (m Model) View() tea.View {
	view := tea.NewView(m.render())
	view.AltScreen = true
	view.WindowTitle = "chesstui · " + labels[m.active]
	return view
}

func (m Model) render() string {
	width, height := m.width, m.height
	if width == 0 || height == 0 {
		return ""
	}
	if width < minWidth || height < minHeight {
		message := m.theme.Muted().Render("make the terminal a little larger")
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, fitViewport(message, width, height))
	}
	account := ""
	if current, ok := m.session.Account(); ok {
		account = current.Username
	}
	header := widgets.Header(*m.theme, labels, m.active, m.nav, m.focus == focusNavigation, account, width)
	screen := m.screens[m.active]
	footer := widgets.Footer(*m.theme, screen.Hints(), screen.Status(), width)
	if m.mode == modeCommand {
		footer = widgets.CommandLine(*m.theme, m.command, width)
	}
	// One blank row under the header and above the footer frames the body.
	bodyHeight := max(1, height-4)
	workspaceWidth := min(140, max(1, width-4))
	body := screen.View(workspaceWidth, bodyHeight)
	if m.help {
		body = widgets.HelpPanel(*m.theme, labels[m.active], globalHints, screen.Hints())
	}
	body = fitViewport(body, workspaceWidth, bodyHeight)
	canvas := lipgloss.Place(width, bodyHeight, lipgloss.Center, lipgloss.Center, body)
	return header + "\n\n" + canvas + "\n\n" + footer
}

var globalHints = []widgets.Hint{
	{Key: "1–7", Label: "jump to a section"},
	{Key: "H L", Label: "previous / next section"},
	{Key: "gt gT", Label: "next / previous section"},
	{Key: "tab", Label: "focus the tab bar"},
	{Key: ":", Label: "command, e.g. :watch :q"},
	{Key: "esc", Label: "back to the tab bar"},
	{Key: "?", Label: "close help"},
	{Key: "q", Label: "quit"},
}
