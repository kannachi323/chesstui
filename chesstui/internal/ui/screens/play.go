package screens

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	chesscore "github.com/kannachi323/chesstui/internal/chess"
	"github.com/kannachi323/chesstui/internal/engine"
	"github.com/kannachi323/chesstui/internal/lichess"
	"github.com/kannachi323/chesstui/internal/platform"
	"github.com/kannachi323/chesstui/internal/ui/widgets"
)

type loginStartedMsg struct {
	login *lichess.Login
	err   error
}

type loginFinishedMsg struct {
	account lichess.Account
	err     error
}

type gameCreatedMsg struct {
	game      lichess.EventGame
	challenge lichess.Challenge
	err       error
}

type eventStreamMsg struct {
	event lichess.Event
	err   error
}

type boardStreamMsg struct {
	gameID string
	event  lichess.BoardEvent
	retry  time.Duration // > 0: the stream dropped and will reopen
	err    error
}

type moveFinishedMsg struct {
	action string
	err    error
}

type engineMoveMsg struct {
	game     int
	analysis engine.Analysis
	err      error
}

type clockTickMsg struct{ game string }

var clocks = []struct {
	label            string
	limit, increment int
}{{"3+2", 180, 2}, {"5+3", 300, 3}, {"10+0", 600, 0}, {"15+10", 900, 10}, {"30+20", 1800, 20}}

const (
	playAI = iota
	playFriend
	playLocal
	playClock
	playColor
	playAccount
)

var colors = []string{"random", "white", "black"}

type Play struct {
	ctx Context

	selected  int
	level     int
	clock     int
	side      int // index into colors
	note      note
	busy      bool
	username  *textInput
	login     *lichess.Login
	cancelRun context.CancelFunc

	position   *chesscore.Position
	input      moveInput
	gameID     string
	color      string // "white" or "black": the side the user plays
	local      bool
	localGame  int
	finished   bool
	confirming string
	white      lichess.GameUser
	black      lichess.GameUser
	state      lichess.GameState
	stateAt    time.Time

	boardUpdates chan boardStreamMsg
	boardCancel  context.CancelFunc
	eventUpdates chan eventStreamMsg
	eventCancel  context.CancelFunc
	initialFEN   string
}

func NewPlay(ctx Context) *Play {
	return &Play{ctx: ctx, level: 3, clock: 1}
}

func (p *Play) Init() tea.Cmd { return nil }

func (p *Play) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.PasteMsg:
		if p.username != nil {
			p.username.paste(msg.Content)
		}
		return nil
	case loginStartedMsg:
		if msg.err != nil {
			p.busy, p.note = false, errNote(msg.err)
			return nil
		}
		p.login = msg.login
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		p.cancelRun = cancel
		login := msg.login
		session := p.ctx.Session
		return tea.Batch(openURL(login.URL), func() tea.Msg {
			defer cancel()
			token, err := login.Wait(ctx)
			if err != nil {
				return loginFinishedMsg{err: err}
			}
			account, err := session.Connect(ctx, token)
			return loginFinishedMsg{account: account, err: err}
		})
	case loginFinishedMsg:
		p.busy, p.login = false, nil
		switch {
		case errors.Is(msg.err, context.Canceled):
			p.note = plainNote("Sign-in cancelled")
		case errors.Is(msg.err, context.DeadlineExceeded):
			p.note = badNote("Sign-in timed out · try again")
		case msg.err != nil:
			p.note = errNote(msg.err)
		default:
			p.note = okNote("Signed in as %s", msg.account.Username)
			p.selected = playAI
		}
		return nil
	case browserOpenedMsg:
		if msg.err != nil && p.login != nil {
			p.note = plainNote("Couldn’t open a browser · press c to copy the link")
		}
		return nil
	case gameCreatedMsg:
		p.busy = false
		if msg.err != nil {
			p.note = errNote(msg.err)
			return nil
		}
		if id := firstNonEmpty(msg.game.ID, msg.game.GameID); id != "" {
			return p.startOnlineGame(id, firstNonEmpty(msg.game.Color, msg.game.Player))
		}
		p.busy, p.note = true, busyNote("Challenge sent · waiting for "+firstNonEmpty(msg.challenge.ID, "your opponent"))
		return p.startEventStream()
	case eventStreamMsg:
		if msg.err != nil {
			if !errors.Is(msg.err, context.Canceled) {
				p.busy, p.note = false, errNote(msg.err)
			}
			return nil
		}
		switch msg.event.Type {
		case "gameStart":
			p.stopEvents()
			game := msg.event.Game
			return p.startOnlineGame(firstNonEmpty(game.GameID, game.ID), game.Color)
		case "challengeDeclined":
			p.stopEvents()
			p.busy, p.note = false, plainNote("Challenge declined")
			return nil
		}
		return p.waitEvent()
	case boardStreamMsg:
		if msg.gameID != p.gameID {
			return nil
		}
		switch {
		case msg.retry > 0:
			p.note = busyNote(fmt.Sprintf("Connection lost · reconnecting in %s", msg.retry.Round(time.Second)))
		case msg.err != nil:
			if !errors.Is(msg.err, context.Canceled) && !errors.Is(msg.err, lichess.ErrStreamDone) {
				p.note = errNote(msg.err)
			}
			return nil
		default:
			p.consumeBoardEvent(msg.event)
		}
		return p.waitBoard()
	case clockTickMsg:
		if msg.game != p.gameID || p.finished || p.position == nil {
			return nil
		}
		return p.tick()
	case moveFinishedMsg:
		p.busy = false
		if msg.err != nil {
			p.note = errNote(msg.err)
		} else if msg.action == "draw" {
			p.note = plainNote("Draw offered")
		}
		return nil
	case engineMoveMsg:
		if msg.game != p.localGame || p.position == nil {
			return nil
		}
		p.busy = false
		if msg.err != nil {
			p.note = errNote(msg.err)
			return nil
		}
		if err := p.position.ApplyUCI(msg.analysis.BestMove); err != nil {
			p.note = errNote(err)
			return nil
		}
		p.afterLocalMove()
		return nil
	}

	key, ok := keyString(msg)
	if !ok {
		return nil
	}
	if p.username != nil {
		return p.updateUsername(key)
	}
	if p.login != nil {
		switch key {
		case "esc":
			if p.cancelRun != nil {
				p.cancelRun()
			}
		case "c":
			p.note = plainNote("Sign-in link copied")
			return tea.SetClipboard(p.login.URL)
		case "o":
			return openURL(p.login.URL)
		}
		return nil
	}
	if p.position == nil {
		return p.updateMenu(key)
	}
	return p.updateGame(key)
}

func (p *Play) updateMenu(key string) tea.Cmd {
	if p.busy {
		if key == "esc" && p.eventCancel != nil {
			p.stopEvents()
			p.busy, p.note = false, plainNote("Stopped waiting")
		}
		return nil
	}
	signedIn := p.ctx.Session.HasToken()
	switch key {
	case "up", "k":
		p.selected = max(0, p.selected-1)
	case "down", "j":
		p.selected = min(playAccount, p.selected+1)
	case "left", "h", "right", "l":
		delta := 1
		if key == "left" || key == "h" {
			delta = -1
		}
		switch p.selected {
		case playAI, playLocal:
			p.level = min(8, max(1, p.level+delta))
		case playClock:
			p.clock = (p.clock + delta + len(clocks)) % len(clocks)
		case playColor:
			p.side = (p.side + delta + len(colors)) % len(colors)
		}
	case "enter", "space":
		switch p.selected {
		case playAI:
			if !signedIn {
				p.selected, p.note = playAccount, plainNote("Sign in to play on Lichess")
				return nil
			}
			p.busy, p.note = true, busyNote(fmt.Sprintf("Starting a %s game with Lichess AI level %d", clocks[p.clock].label, p.level))
			return p.challengeAI()
		case playFriend:
			if !signedIn {
				p.selected, p.note = playAccount, plainNote("Sign in to challenge a player")
				return nil
			}
			p.username, p.note = &textInput{allowed: usernameChar}, note{}
		case playLocal:
			return p.startLocal()
		case playAccount:
			if signedIn {
				p.note = plainNote("Sign out from Settings")
				return nil
			}
			p.busy, p.note = true, busyNote("Opening Lichess sign-in")
			client := p.ctx.Session.Client()
			return func() tea.Msg {
				login, err := client.BeginLogin(lichess.DefaultScopes)
				return loginStartedMsg{login: login, err: err}
			}
		}
	}
	return nil
}

func (p *Play) updateGame(key string) tea.Cmd {
	flipped := p.color == "black"
	if !p.finished && !p.busy {
		if move, problem, used := p.input.handle(key, p.position, flipped); used {
			p.confirming = ""
			if problem != "" {
				p.note = badNote(problem)
			}
			if move != "" {
				return p.submitMove(move)
			}
			return nil
		}
	}
	switch key {
	case "esc", "n":
		if p.finished || p.local {
			p.resetGame()
		}
	case "r":
		if p.finished {
			return nil
		}
		if p.local {
			p.resetGame()
			p.note = plainNote("You resigned")
			return nil
		}
		if p.ctx.Session.Config.ConfirmResign && p.confirming != "resign" {
			p.confirming, p.note = "resign", plainNote("Press r again to resign")
			return nil
		}
		p.confirming, p.busy = "", true
		return p.gameAction("resign")
	case "x":
		if !p.local && !p.finished {
			p.busy = true
			return p.gameAction("abort")
		}
	case "o":
		if !p.local && !p.finished {
			p.busy = true
			return p.gameAction("draw")
		}
	}
	return nil
}

func (p *Play) challengeAI() tea.Cmd {
	clock, level, client, color := clocks[p.clock], p.level, p.ctx.Session.Client(), colors[p.side]
	return func() tea.Msg {
		options := lichess.ChallengeOptions{Limit: clock.limit, Increment: clock.increment, Color: color, Variant: "standard"}
		game, err := client.ChallengeAI(context.Background(), level, options)
		return gameCreatedMsg{game: game, err: err}
	}
}

func (p *Play) updateUsername(key string) tea.Cmd {
	switch key {
	case "esc":
		p.username = nil
	case "enter":
		username := strings.TrimSpace(p.username.value)
		p.username = nil
		if username == "" {
			return nil
		}
		clock, client, color := clocks[p.clock], p.ctx.Session.Client(), colors[p.side]
		p.busy, p.note = true, busyNote("Challenging "+username+" · esc to stop waiting")
		return func() tea.Msg {
			options := lichess.ChallengeOptions{Limit: clock.limit, Increment: clock.increment, Color: color, Variant: "standard"}
			challenge, err := client.ChallengeUser(context.Background(), username, options)
			return gameCreatedMsg{challenge: challenge, err: err}
		}
	default:
		p.username.handle(key)
	}
	return nil
}

func (p *Play) startOnlineGame(id, color string) tea.Cmd {
	p.stopBoard()
	p.gameID, p.color, p.local, p.finished, p.busy = id, color, false, false, false
	p.note, p.input, p.confirming = plainNote("Connecting to the game"), moveInput{}, ""
	p.initialFEN, p.white, p.black, p.state = "startpos", lichess.GameUser{}, lichess.GameUser{}, lichess.GameState{}
	p.position, _ = chesscore.NewPosition("")

	ctx, cancel := context.WithCancel(context.Background())
	p.boardCancel = cancel
	p.boardUpdates = make(chan boardStreamMsg, 16)
	updates, client := p.boardUpdates, p.ctx.Session.Client()
	send := func(msg boardStreamMsg) error {
		select {
		case updates <- msg:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	go func() {
		err := lichess.Reconnect(ctx, func(ctx context.Context) error {
			return client.StreamGame(ctx, id, func(event lichess.BoardEvent) error {
				if err := send(boardStreamMsg{gameID: id, event: event}); err != nil {
					return err
				}
				state := event.GameState
				if event.Type == "gameFull" {
					state = event.State
				}
				if gameOver(state.Status) {
					return lichess.ErrStreamDone
				}
				return nil
			})
		}, func(wait time.Duration, _ error) { _ = send(boardStreamMsg{gameID: id, retry: wait}) })
		_ = send(boardStreamMsg{gameID: id, err: err})
	}()
	return tea.Batch(p.waitBoard(), p.tick())
}

func (p *Play) startEventStream() tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	p.eventCancel = cancel
	p.eventUpdates = make(chan eventStreamMsg, 8)
	updates, client := p.eventUpdates, p.ctx.Session.Client()
	go func() {
		err := client.StreamEvents(ctx, func(event lichess.Event) error {
			select {
			case updates <- eventStreamMsg{event: event}:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		if err == nil {
			err = errors.New("Lichess closed the event stream")
		}
		select {
		case updates <- eventStreamMsg{err: err}:
		case <-ctx.Done():
		}
	}()
	return p.waitEvent()
}

func (p *Play) waitBoard() tea.Cmd {
	updates := p.boardUpdates
	return func() tea.Msg { return <-updates }
}

func (p *Play) waitEvent() tea.Cmd {
	updates := p.eventUpdates
	return func() tea.Msg { return <-updates }
}

func (p *Play) stopEvents() {
	if p.eventCancel != nil {
		p.eventCancel()
		p.eventCancel = nil
	}
}

func (p *Play) stopBoard() {
	if p.boardCancel != nil {
		p.boardCancel()
		p.boardCancel = nil
	}
}

func (p *Play) tick() tea.Cmd {
	id := p.gameID
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return clockTickMsg{game: id} })
}

func gameOver(status string) bool {
	return status != "" && status != "started" && status != "created"
}

func (p *Play) consumeBoardEvent(event lichess.BoardEvent) {
	state := event.GameState
	if event.Type == "gameFull" {
		p.initialFEN, p.white, p.black = event.InitialFEN, event.White, event.Black
		state = event.State
	} else if event.Type != "gameState" {
		return // chatLine, opponentGone: nothing to draw
	}
	fen := p.initialFEN
	if fen == "startpos" {
		fen = ""
	}
	position, err := chesscore.NewPosition(fen)
	if err == nil {
		err = position.ApplyMoves(state.Moves)
	}
	if err != nil {
		p.note = errNote(err)
		return
	}
	p.position, p.state, p.stateAt = position, state, time.Now()
	switch {
	case gameOver(state.Status):
		p.finished = true
		p.note = plainNote(describeResult(state.Status, state.Winner, p.color))
	case p.color == "white" && state.BDraw, p.color == "black" && state.WDraw:
		p.note = plainNote("Draw offered · o to accept")
	case p.color != "" && sideName(position.Turn()) == p.color:
		p.note = plainNote("Your move")
	default:
		p.note = note{}
	}
}

func describeResult(status, winner, you string) string {
	reason := map[string]string{
		"mate": "Checkmate", "resign": "Resignation", "outoftime": "Time out", "timeout": "Opponent left",
		"draw": "Draw", "stalemate": "Stalemate", "aborted": "Game aborted", "noStart": "Game didn’t start",
	}[status]
	if reason == "" {
		reason = status
	}
	switch {
	case winner == "":
		return reason
	case winner == you:
		return reason + " · you win"
	default:
		return reason + " · you lose"
	}
}

func sideName(color chesscore.Color) string {
	if color == chesscore.Black {
		return "black"
	}
	return "white"
}

func (p *Play) startLocal() tea.Cmd {
	if _, err := engine.NewStockfish(p.ctx.Session.Config.StockfishPath); err != nil {
		p.note = errNote(err)
		return nil
	}
	p.stopBoard()
	p.localGame++
	p.position, _ = chesscore.NewPosition("")
	color := colors[p.side]
	if color == "random" {
		color = colors[1+time.Now().Nanosecond()%2]
	}
	p.local, p.finished, p.color, p.gameID = true, false, color, ""
	p.input, p.state = moveInput{}, lichess.GameState{}
	you, engineUser := lichess.GameUser{Name: "You"}, lichess.GameUser{Name: "Stockfish", AILevel: p.level}
	p.white, p.black = you, engineUser
	if color == "black" {
		p.white, p.black = engineUser, you
		return p.engineMove()
	}
	p.note = plainNote("Your move")
	return nil
}

// engineMove asks local Stockfish for its reply in the current position.
func (p *Play) engineMove() tea.Cmd {
	analyzer, err := engine.NewStockfish(p.ctx.Session.Config.StockfishPath)
	if err != nil {
		p.note = errNote(err)
		return nil
	}
	p.busy, p.note = true, busyNote("Stockfish is thinking")
	fen, level, game := p.position.FEN(), p.level, p.localGame
	return func() tea.Msg {
		analysis, err := analyzer.Play(context.Background(), fen, level)
		return engineMoveMsg{game: game, analysis: analysis, err: err}
	}
}

func (p *Play) submitMove(move string) tea.Cmd {
	if !p.local {
		if sideName(p.position.Turn()) != p.color {
			p.note = plainNote("Wait for your opponent’s move")
			return nil
		}
		p.busy = true
		gameID, client := p.gameID, p.ctx.Session.Client()
		return func() tea.Msg {
			return moveFinishedMsg{action: "move", err: client.Move(context.Background(), gameID, move, false)}
		}
	}
	if err := p.position.ApplyUCI(move); err != nil {
		p.note = errNote(err)
		return nil
	}
	if p.afterLocalMove() {
		return nil
	}
	return p.engineMove()
}

// afterLocalMove updates the note and reports whether the game ended.
func (p *Play) afterLocalMove() bool {
	if outcome := p.position.Outcome(); outcome != "" {
		p.finished, p.note = true, plainNote(outcome)
		return true
	}
	p.note = plainNote("Your move")
	if p.position.InCheck() {
		p.note = plainNote("Check · your move")
	}
	return false
}

func (p *Play) gameAction(action string) tea.Cmd {
	gameID, client := p.gameID, p.ctx.Session.Client()
	return func() tea.Msg {
		var err error
		switch action {
		case "resign":
			err = client.Resign(context.Background(), gameID)
		case "draw":
			err = client.Draw(context.Background(), gameID, true)
		case "abort":
			err = client.Abort(context.Background(), gameID)
		}
		return moveFinishedMsg{action: action, err: err}
	}
}

func (p *Play) resetGame() {
	p.stopBoard()
	p.localGame++
	p.position, p.gameID, p.local, p.finished, p.busy = nil, "", false, false, false
	p.input, p.note = moveInput{}, note{}
}

func (p *Play) View(width, height int) string {
	if p.position == nil {
		return p.menuView(width)
	}
	flipped := p.color == "black"
	board := p.input.decorate(widgets.Board{
		Theme: *p.ctx.Theme, Flipped: flipped, Glyphs: p.ctx.Session.Config.Glyphs, LastMove: lastMove(p.position),
	}, p.position)
	render := func(b widgets.Board) string { return b.Render(p.position) }
	return boardAndSide(board, render, func(w int) string { return p.sideView(w, height) }, p.compactView, width, height)
}

func (p *Play) players() (top, bottom string) {
	white := p.playerName(p.white, "White")
	black := p.playerName(p.black, "Black")
	whiteClock, blackClock := p.clockText(true), p.clockText(false)
	turnWhite := p.position.Turn() == chesscore.White && !p.finished
	w := func(width int) string {
		return p.ctx.playerLine(true, white, p.white.Rating, whiteClock, turnWhite, width)
	}
	b := func(width int) string {
		return p.ctx.playerLine(false, black, p.black.Rating, blackClock, !turnWhite && !p.finished, width)
	}
	if p.color == "black" {
		return w(30), b(30)
	}
	return b(30), w(30)
}

func (p *Play) sideView(width, height int) string {
	top, bottom := p.players()
	top = lipgloss.NewStyle().Width(width).Render(top)
	moves := p.position.SANMoves()
	listRows := max(1, height-8)
	parts := []string{top, "", p.ctx.moveList(moves, len(moves)-1, listRows, 1, false)}
	if len(moves) == 0 {
		parts[2] = p.ctx.Theme.Faint().Render("  no moves yet")
	}
	parts = append(parts, "", bottom, "")
	if !p.finished {
		parts = append(parts, p.input.prompt(p.ctx))
	}
	if p.note.text != "" {
		parts = append(parts, p.ctx.note(p.note))
	}
	if !p.local && !p.finished {
		parts = append(parts, p.ctx.Theme.Faint().Render("engine help is off in online games"))
	}
	return strings.Join(parts, "\n")
}

func (p *Play) compactView(width int) string {
	top, bottom := p.players()
	lines := []string{bottom + "   " + p.ctx.Theme.Faint().Render("vs") + "   " + top}
	if !p.finished {
		lines = append(lines, p.input.prompt(p.ctx))
	}
	if p.note.text != "" {
		lines = append(lines, p.ctx.note(p.note))
	}
	return strings.Join(lines, "\n")
}

func (p *Play) playerName(user lichess.GameUser, fallback string) string {
	switch {
	case user.AILevel > 0 && user.Name == "":
		return fmt.Sprintf("Lichess AI · %d", user.AILevel)
	case user.AILevel > 0:
		return fmt.Sprintf("%s · %d", user.Name, user.AILevel)
	}
	return firstNonEmpty(user.Name, user.ID, fallback)
}

func (p *Play) clockText(white bool) string {
	if p.local || p.state.WTime == 0 && p.state.BTime == 0 {
		return ""
	}
	ms := p.state.BTime
	if white {
		ms = p.state.WTime
	}
	// Lichess sends clocks with each move; count down the side to move.
	moving := (p.position.Turn() == chesscore.White) == white
	if moving && !p.finished && p.position.Ply() >= 2 {
		ms -= int(time.Since(p.stateAt).Milliseconds())
	}
	return formatClock(ms)
}

func (p *Play) menuView(width int) string {
	t := p.ctx.Theme
	if p.login != nil {
		lines := []string{
			t.Title().Render("Sign in with Lichess"), "",
			t.Muted().Render("Approve chesstui in the browser tab that just opened."),
			t.Muted().Render("Your password never touches this app."), "",
			t.Faint().Render("o reopen · c copy link · esc cancel"),
		}
		return strings.Join(lines, "\n")
	}
	if p.username != nil {
		return t.Title().Render("Challenge a player") + "\n\n" + p.ctx.field("Lichess username", *p.username) + "\n\n" +
			t.Faint().Render(clocks[p.clock].label+" · casual · enter send · esc back")
	}
	account := t.Faint().Render("not signed in")
	if current, ok := p.ctx.Session.Account(); ok {
		account = t.Muted().Render(current.Username)
	} else if p.ctx.Session.HasToken() {
		account = t.Muted().Render("signed in")
	}
	needsLichess := func(label string, index int) string {
		if p.ctx.Session.HasToken() || index == p.selected {
			return p.ctx.item(label, index == p.selected)
		}
		return t.Muted().Render(label)
	}
	value := func(text string, adjustable bool) string {
		if adjustable {
			return t.Faint().Render("‹ ") + t.Accent().Render(text) + t.Faint().Render(" ›")
		}
		return t.Faint().Render(text)
	}
	level := fmt.Sprintf("level %d", p.level)
	engineValue := value(level, p.selected == playLocal)
	if _, err := engine.NewStockfish(p.ctx.Session.Config.StockfishPath); err != nil {
		engineValue = t.Faint().Render("not installed")
	}
	rowWidth := min(52, width)
	row := func(label, right string) string {
		gap := max(2, rowWidth-2-lipgloss.Width(label)-lipgloss.Width(right))
		return label + strings.Repeat(" ", gap) + right
	}
	rows := []string{
		row(needsLichess("Lichess AI", playAI), value(level, p.selected == playAI)),
		row(needsLichess("Challenge a player", playFriend), value("by username", false)),
		row(p.ctx.item("Stockfish, offline", p.selected == playLocal), engineValue),
		row(p.ctx.item("Clock", p.selected == playClock), value(clocks[p.clock].label, p.selected == playClock)),
		row(p.ctx.item("Color", p.selected == playColor), value(colors[p.side], p.selected == playColor)),
		row(p.ctx.item("Lichess account", p.selected == playAccount), account),
	}
	out := t.Title().Render("Play") + "\n\n" + p.ctx.menu(rows, p.selected, len(rows), rowWidth, true)
	if p.note.text != "" {
		out += "\n\n" + p.ctx.note(p.note)
	}
	return lipgloss.NewStyle().Width(rowWidth).Render(out)
}

func (p *Play) Hints() []widgets.Hint {
	switch {
	case p.login != nil:
		return []widgets.Hint{{Key: "o", Label: "open"}, {Key: "c", Label: "copy link"}, {Key: "esc", Label: "cancel"}}
	case p.username != nil:
		return []widgets.Hint{{Key: "enter", Label: "send"}, {Key: "esc", Label: "back"}}
	case p.position == nil:
		return []widgets.Hint{{Key: "↑↓", Label: "choose"}, {Key: "←→", Label: "adjust"}, {Key: "enter", Label: "start"}}
	case p.finished:
		return []widgets.Hint{{Key: "n", Label: "new game"}}
	case p.local:
		return append(moveHints(), widgets.Hint{Key: "r", Label: "resign"}, widgets.Hint{Key: "n", Label: "new game"})
	}
	return append(moveHints(), widgets.Hint{Key: "o", Label: "offer draw"}, widgets.Hint{Key: "x", Label: "abort"}, widgets.Hint{Key: "r", Label: "resign"})
}

func (p *Play) Status() string {
	if p.position != nil && !p.local {
		return "lichess.org/" + p.gameID
	}
	return ""
}

func (p *Play) CapturesKey(key string) bool {
	if p.username != nil {
		return true
	}
	// Esc cancels a sign-in or a pending challenge rather than leaving.
	if key == "esc" && (p.login != nil || p.busy && p.position == nil) {
		return true
	}
	return p.position != nil && !p.finished && p.input.capturesKey(key)
}

// QuitWarning holds the app open for a second q during a live online game.
func (p *Play) QuitWarning() bool {
	return p.position != nil && !p.local && !p.finished
}

func lastMove(position *chesscore.Position) string {
	moves := position.Moves()
	if len(moves) == 0 {
		return ""
	}
	return moves[len(moves)-1]
}

func formatClock(milliseconds int) string {
	if milliseconds <= 0 {
		return "0:00"
	}
	seconds := milliseconds / 1000
	if seconds < 10 {
		return fmt.Sprintf("0:%02d.%d", seconds, milliseconds%1000/100)
	}
	if seconds >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", seconds/3600, seconds%3600/60, seconds%60)
	}
	return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
}

func formatScore(analysis engine.Analysis) string {
	if analysis.Mate != 0 {
		return fmt.Sprintf("#%d", analysis.Mate)
	}
	return fmt.Sprintf("%+.1f", float64(analysis.ScoreCP)/100)
}

func openURL(rawURL string) tea.Cmd {
	return func() tea.Msg {
		return browserOpenedMsg{err: platform.OpenBrowser(context.Background(), rawURL)}
	}
}

type browserOpenedMsg struct{ err error }
