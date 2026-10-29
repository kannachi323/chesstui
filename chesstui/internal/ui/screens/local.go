package screens

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	chesscore "github.com/kannachi323/chesstui/internal/chess"
	"github.com/kannachi323/chesstui/internal/domain"
	"github.com/kannachi323/chesstui/internal/engine"
	"github.com/kannachi323/chesstui/internal/pgn"
	"github.com/kannachi323/chesstui/internal/ui/widgets"
)

type localMoveMsg struct {
	game     string
	analysis engine.Analysis
	err      error
}

type localResumeMsg struct {
	id   string
	game *pgn.Game
}

type localSavedMsg struct{ err error }

// OpenAnalyzeMsg asks the app to show a stored game in Analyze.
type OpenAnalyzeMsg struct{ GameID string }

const (
	localStart = iota
	localLevel
	localColor
	localResume
)

// Local is a game against the user's own UCI engine. Every move is saved
// to the library as PGN, so games can be resumed and analysed later.
type Local struct {
	ctx       Context
	activated bool
	selected  int
	level     int
	side      int // index into colors
	note      note

	resumeID   string
	resumeGame *pgn.Game

	id       string
	game     *pgn.Game
	tip      *pgn.Node
	color    string
	finished bool
	busy     bool
	input    moveInput
	started  time.Time
	engine   string
}

func NewLocal(ctx Context) *Local { return &Local{ctx: ctx, level: 3} }

func (l *Local) Init() tea.Cmd { return nil }

// Activate looks for an unfinished game to offer for resuming.
func (l *Local) Activate() tea.Cmd {
	if l.activated {
		return nil
	}
	l.activated = true
	store := l.ctx.Session.Store
	return func() tea.Msg {
		ctx := context.Background()
		games, err := store.ListGames(ctx, 20)
		if err != nil {
			return localResumeMsg{}
		}
		for _, summary := range games {
			if summary.Source != "local" || summary.Result != "*" {
				continue
			}
			text, err := store.GamePGN(ctx, summary.ID)
			if err != nil {
				break
			}
			if games, err := pgn.Parse(text); err == nil && games[0].Err == nil && len(games[0].Mainline()) > 0 {
				return localResumeMsg{id: summary.ID, game: games[0]}
			}
			break
		}
		return localResumeMsg{}
	}
}

func (l *Local) rows() int {
	if l.resumeGame != nil {
		return localResume + 1
	}
	return localColor + 1
}

func (l *Local) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case localResumeMsg:
		l.resumeID, l.resumeGame = msg.id, msg.game
		if msg.game != nil && l.game == nil {
			l.selected = localResume
		}
		return nil
	case localSavedMsg:
		if msg.err != nil {
			l.note = errNote(msg.err)
		}
		return nil
	case localMoveMsg:
		if msg.game != l.id || l.game == nil {
			return nil
		}
		l.busy = false
		if msg.err != nil {
			l.note = errNote(msg.err)
			return nil
		}
		node, err := l.tip.Play(msg.analysis.BestMove)
		if err != nil {
			l.note = errNote(err)
			return nil
		}
		l.tip = node
		return l.afterMove()
	}

	key, ok := keyString(msg)
	if !ok {
		return nil
	}
	if l.game == nil {
		return l.updateMenu(key)
	}
	return l.updateGame(key)
}

func (l *Local) updateMenu(key string) tea.Cmd {
	switch key {
	case "up", "k":
		l.selected = max(0, l.selected-1)
	case "down", "j":
		l.selected = min(l.rows()-1, l.selected+1)
	case "left", "h", "right", "l":
		delta := 1
		if key == "left" || key == "h" {
			delta = -1
		}
		switch l.selected {
		case localLevel:
			l.level = min(8, max(1, l.level+delta))
		case localColor:
			l.side = (l.side + delta + len(colors)) % len(colors)
		}
	case "enter", "space":
		if l.selected == localResume && l.resumeGame != nil {
			return l.resume()
		}
		return l.start()
	}
	return nil
}

func (l *Local) updateGame(key string) tea.Cmd {
	position := l.tip.Position()
	if !l.finished && !l.busy {
		if move, problem, used := l.input.handle(key, position, l.color == "black"); used {
			if problem != "" {
				l.note = badNote(problem)
			}
			if move != "" {
				node, err := l.tip.Play(move)
				if err != nil {
					l.note = errNote(err)
					return nil
				}
				l.tip = node
				return l.afterMove()
			}
			return nil
		}
	}
	switch key {
	case "u":
		if l.busy || l.tip.Parent == nil {
			return nil
		}
		// Take back to the last position where it was your move.
		for l.tip.Parent != nil {
			undone := l.tip
			l.tip = l.tip.Parent
			undone.Delete()
			if sideName(l.tip.Position().Turn()) == l.color {
				break
			}
		}
		l.finished, l.game.Result, l.note = false, "*", plainNote("Move taken back")
		return l.save()
	case "r":
		if l.finished || l.busy {
			return nil
		}
		l.finished = true
		l.game.Result = "1-0"
		if l.color == "white" {
			l.game.Result = "0-1"
		}
		l.note = plainNote("You resigned · enter to analyse")
		return l.save()
	case "enter":
		if l.finished {
			id := l.id
			return func() tea.Msg { return OpenAnalyzeMsg{GameID: id} }
		}
	case "n", "esc":
		if l.busy {
			return nil
		}
		l.game, l.tip, l.input, l.resumeGame = nil, nil, moveInput{}, nil
		l.selected, l.note = localStart, note{}
	}
	return nil
}

func (l *Local) start() tea.Cmd {
	analyzer, err := l.ctx.Session.Engine()
	if err != nil {
		l.note = errNote(err)
		return nil
	}
	color := colors[l.side]
	if color == "random" {
		color = colors[1+time.Now().Nanosecond()%2]
	}
	if err := analyzer.Ensure(); err != nil {
		l.note = errNote(err)
		return nil
	}
	game, _ := pgn.New("")
	l.engine = analyzer.Name()
	opponent := fmt.Sprintf("%s · level %d", l.engine, l.level)
	white, black := "You", opponent
	if color == "black" {
		white, black = opponent, "You"
	}
	now := time.Now()
	game.SetTag("Event", "Casual game")
	game.SetTag("Site", "chesstui")
	game.SetTag("Date", now.Format("2006.01.02"))
	game.SetTag("White", white)
	game.SetTag("Black", black)
	game.SetTag("EngineLevel", fmt.Sprint(l.level))
	l.id = fmt.Sprintf("local-%d", now.UnixNano())
	l.game, l.tip, l.color, l.started = game, game.Root, color, now
	l.finished, l.busy, l.input, l.note = false, false, moveInput{}, plainNote("Your move")
	l.resumeGame = nil
	if color == "black" {
		return tea.Batch(l.save(), l.engineMove())
	}
	return l.save()
}

func (l *Local) resume() tea.Cmd {
	game := l.resumeGame
	l.id, l.game = l.resumeID, game
	l.tip = game.Root
	for len(l.tip.Children) > 0 {
		l.tip = l.tip.Children[0]
	}
	l.color = "white"
	if game.Tags["Black"] == "You" {
		l.color = "black"
	}
	if level := game.Tags["EngineLevel"]; level != "" {
		fmt.Sscan(level, &l.level)
	}
	l.started, l.finished, l.input = time.Now(), false, moveInput{}
	l.resumeGame = nil
	if sideName(l.tip.Position().Turn()) != l.color {
		return l.engineMove()
	}
	l.note = plainNote("Welcome back · your move")
	return nil
}

func (l *Local) engineMove() tea.Cmd {
	analyzer, err := l.ctx.Session.Engine()
	if err != nil {
		l.note = errNote(err)
		return nil
	}
	l.busy, l.note = true, busyNote("Thinking")
	request := engine.Request{FEN: startFEN(l.game), Moves: l.tip.Line()}
	id, level := l.id, l.level
	return func() tea.Msg {
		analysis, err := analyzer.Play(context.Background(), request, level)
		return localMoveMsg{game: id, analysis: analysis, err: err}
	}
}

// startFEN is "" for the standard start so engines get "position startpos".
func startFEN(game *pgn.Game) string {
	if game.Tags["FEN"] == "" {
		return ""
	}
	return game.StartFEN()
}

func (l *Local) afterMove() tea.Cmd {
	position := l.tip.Position()
	if outcome := position.Outcome(); outcome != "" {
		l.finished = true
		switch {
		case strings.Contains(outcome, "white wins"):
			l.game.Result = "1-0"
		case strings.Contains(outcome, "black wins"):
			l.game.Result = "0-1"
		default:
			l.game.Result = "1/2-1/2"
		}
		l.note = plainNote(outcome + " · enter to analyse")
		return l.save()
	}
	if sideName(position.Turn()) != l.color {
		return tea.Batch(l.save(), l.engineMove())
	}
	l.note = plainNote("Your move")
	if position.InCheck() {
		l.note = plainNote("Check · your move")
	}
	return l.save()
}

func (l *Local) save() tea.Cmd {
	white, black := l.game.Tags["White"], l.game.Tags["Black"]
	record := domain.Game{
		ID: l.id, Source: "local", Variant: "standard", Moves: l.tip.Line(),
		White: domain.Player{Name: white}, Black: domain.Player{Name: black},
		Result: l.game.Result, PGN: l.game.String(), PlayedAt: l.started,
	}
	store := l.ctx.Session.Store
	return func() tea.Msg {
		return localSavedMsg{err: store.UpsertGame(context.Background(), record)}
	}
}

func (l *Local) View(width, height int) string {
	if l.game == nil {
		return l.menuView(width)
	}
	position := l.tip.Position()
	board := l.input.decorate(widgets.Board{
		Theme: *l.ctx.Theme, Flipped: l.color == "black", Glyphs: l.ctx.Session.Config.Glyphs, LastMove: l.tip.UCI,
	}, position)
	render := func(b widgets.Board) string { return b.Render(position) }
	return boardAndSide(board, render, func(w int) string { return l.sidePanel(w, height) }, l.compact, width, height)
}

func (l *Local) players(width int) (top, bottom string) {
	white, black := l.game.Tags["White"], l.game.Tags["Black"]
	turnWhite := l.tip.Position().Turn() == chesscore.White && !l.finished
	w := l.ctx.playerLine(true, white, 0, "", turnWhite, width)
	b := l.ctx.playerLine(false, black, 0, "", !turnWhite && !l.finished, width)
	if l.color == "black" {
		return w, b
	}
	return b, w
}

func (l *Local) sidePanel(width, height int) string {
	top, bottom := l.players(width)
	var sans []string
	for _, node := range l.game.Mainline() {
		sans = append(sans, node.SAN)
	}
	list := l.ctx.moveList(sans, len(sans)-1, max(1, height-9), 1, false)
	if len(sans) == 0 {
		list = l.ctx.Theme.Faint().Render("  no moves yet")
	}
	parts := []string{top, "", list, "", bottom, ""}
	if !l.finished {
		parts = append(parts, l.input.prompt(l.ctx))
	}
	if l.note.text != "" {
		parts = append(parts, l.ctx.note(l.note))
	}
	return strings.Join(parts, "\n")
}

func (l *Local) compact(width int) string {
	top, bottom := l.players(width / 2)
	lines := []string{bottom + "  " + top}
	if !l.finished {
		lines = append(lines, l.input.prompt(l.ctx))
	}
	if l.note.text != "" {
		lines = append(lines, l.ctx.note(l.note))
	}
	return strings.Join(lines, "\n")
}

func (l *Local) menuView(width int) string {
	t := l.ctx.Theme
	rowWidth := min(52, width)
	value := func(text string, adjustable bool) string {
		if adjustable {
			return t.Faint().Render("‹ ") + t.Accent().Render(text) + t.Faint().Render(" ›")
		}
		return t.Faint().Render(text)
	}
	row := func(label, right string) string {
		gap := max(2, rowWidth-2-lipgloss.Width(label)-lipgloss.Width(right))
		return label + strings.Repeat(" ", gap) + right
	}
	engineName := t.Faint().Render("no engine found")
	if found, err := l.ctx.Session.Engine(); err == nil {
		engineName = t.Faint().Render(found.Name())
	}
	rows := []string{
		row(l.ctx.item("New game", l.selected == localStart), engineName),
		row(l.ctx.item("Strength", l.selected == localLevel), value(fmt.Sprintf("level %d", l.level), l.selected == localLevel)),
		row(l.ctx.item("Color", l.selected == localColor), value(colors[l.side], l.selected == localColor)),
	}
	if l.resumeGame != nil {
		moves := len(l.resumeGame.Mainline())
		rows = append(rows, row(l.ctx.item("Resume last game", l.selected == localResume), value(fmt.Sprintf("%d moves in", (moves+1)/2), false)))
	}
	out := t.Title().Render("Play") + "\n\n" + l.ctx.menu(rows, l.selected, len(rows), rowWidth, true)
	if l.note.text != "" {
		out += "\n\n" + l.ctx.note(l.note)
	} else if _, err := l.ctx.Session.Engine(); err != nil {
		out += "\n\n" + l.ctx.note(errNote(err))
	}
	return lipgloss.NewStyle().Width(rowWidth).Render(out)
}

func (l *Local) Hints() []widgets.Hint {
	switch {
	case l.game == nil:
		return []widgets.Hint{{Key: "↑↓", Label: "choose"}, {Key: "←→", Label: "adjust"}, {Key: "enter", Label: "start"}}
	case l.finished:
		return []widgets.Hint{{Key: "enter", Label: "analyse"}, {Key: "u", Label: "take back"}, {Key: "n", Label: "new game"}}
	}
	return append(moveHints(), widgets.Hint{Key: "u", Label: "take back"}, widgets.Hint{Key: "r", Label: "resign"}, widgets.Hint{Key: "n", Label: "new game"})
}

func (l *Local) Status() string {
	if l.game != nil {
		return "saved to your library"
	}
	return ""
}

func (l *Local) CapturesKey(key string) bool {
	if l.game == nil {
		return false
	}
	if key == "esc" {
		return true
	}
	return !l.finished && l.input.capturesKey(key)
}
