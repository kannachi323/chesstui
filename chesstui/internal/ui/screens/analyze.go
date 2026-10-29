package screens

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	chesscore "github.com/kannachi323/chesstui/internal/chess"
	"github.com/kannachi323/chesstui/internal/domain"
	"github.com/kannachi323/chesstui/internal/engine"
	"github.com/kannachi323/chesstui/internal/pgn"
	"github.com/kannachi323/chesstui/internal/review"
	"github.com/kannachi323/chesstui/internal/storage"
	"github.com/kannachi323/chesstui/internal/ui/widgets"
)

type libraryLoadedMsg struct {
	games []storage.GameSummary
	err   error
}

type gameOpenedMsg struct {
	id    string
	game  *pgn.Game
	evals map[int]domain.Analysis
	err   error
}

type pgnImportedMsg struct {
	ids []string
	err error
}

type reviewProgressMsg struct {
	id   string
	ply  int
	eval domain.Analysis
}

type reviewDoneMsg struct {
	id  string
	err error
}

type liveEvalMsg struct {
	node     *pgn.Node
	analysis engine.Analysis
	err      error
}

type analyzeSavedMsg struct{ err error }

// Analyze is the game library and the review board: open a game, run an
// engine review over it, step through it, try side lines and keep notes.
type Analyze struct {
	ctx      Context
	games    []storage.GameSummary
	selected int
	loading  bool
	note     note
	prompt   *textInput
	purpose  string // "open" or "comment"
	deleting bool

	id      string
	game    *pgn.Game
	cursor  *pgn.Node
	input   moveInput
	flipped bool
	evals   map[int]domain.Analysis // main-line ply → evaluation, White's view

	reviewing     bool
	reviewDone    int
	reviewCancel  context.CancelFunc
	reviewUpdates chan tea.Msg

	live       bool
	liveCancel context.CancelFunc
	liveNode   *pgn.Node
	liveResult *engine.Analysis
}

func NewAnalyze(ctx Context) *Analyze { return &Analyze{ctx: ctx} }

func (a *Analyze) Init() tea.Cmd { return nil }

// Activate refreshes the library each time the tab opens.
func (a *Analyze) Activate() tea.Cmd { return a.loadLibrary() }

func (a *Analyze) loadLibrary() tea.Cmd {
	store := a.ctx.Session.Store
	return func() tea.Msg {
		games, err := store.ListGames(context.Background(), 500)
		return libraryLoadedMsg{games: games, err: err}
	}
}

// Open loads a stored game by id.
func (a *Analyze) Open(id string) tea.Cmd {
	a.stopWork()
	a.loading = true
	store := a.ctx.Session.Store
	return func() tea.Msg {
		ctx := context.Background()
		text, err := store.GamePGN(ctx, id)
		if err != nil {
			return gameOpenedMsg{err: err}
		}
		games, err := pgn.Parse(text)
		if err != nil {
			return gameOpenedMsg{err: err}
		}
		evals, err := store.GameAnalysis(ctx, id)
		return gameOpenedMsg{id: id, game: games[0], evals: evals, err: err}
	}
}

// ImportPGN stores every game in text and opens the first.
func (a *Analyze) ImportPGN(text, source string) tea.Cmd {
	store := a.ctx.Session.Store
	return func() tea.Msg {
		games, err := pgn.Parse(text)
		if err != nil {
			return pgnImportedMsg{err: err}
		}
		var ids []string
		for _, game := range games {
			if game.Err != nil && len(game.Mainline()) == 0 {
				continue
			}
			if game.Tags["Variant"] != "" && !strings.EqualFold(game.Tags["Variant"], "standard") {
				continue
			}
			written := game.String()
			sum := sha1.Sum([]byte(written))
			id := "pgn-" + hex.EncodeToString(sum[:6])
			record := domain.Game{
				ID: id, Source: source, Variant: "standard", Moves: tipOf(game.Root).Line(),
				White: domain.Player{Name: tagOr(game, "White", "White")}, Black: domain.Player{Name: tagOr(game, "Black", "Black")},
				Result: game.Result, PGN: written,
			}
			if played, err := time.Parse("2006.01.02", game.Tags["Date"]); err == nil {
				record.PlayedAt = played
			}
			if err := store.UpsertGame(context.Background(), record); err != nil {
				return pgnImportedMsg{ids: ids, err: err}
			}
			ids = append(ids, id)
		}
		if len(ids) == 0 {
			return pgnImportedMsg{err: errors.New("no standard games in that PGN")}
		}
		return pgnImportedMsg{ids: ids}
	}
}

func tagOr(game *pgn.Game, key, fallback string) string {
	if value := game.Tags[key]; value != "" && value != "?" {
		return value
	}
	return fallback
}

func tipOf(node *pgn.Node) *pgn.Node {
	for len(node.Children) > 0 {
		node = node.Children[0]
	}
	return node
}

func (a *Analyze) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case OpenAnalyzeMsg:
		return a.Open(msg.GameID)
	case tea.PasteMsg:
		if a.prompt != nil {
			a.prompt.paste(msg.Content)
			return nil
		}
		if a.game == nil && strings.TrimSpace(msg.Content) != "" {
			a.note = busyNote("Importing pasted PGN")
			return a.ImportPGN(msg.Content, "paste")
		}
		return nil
	case libraryLoadedMsg:
		if msg.err != nil {
			a.note = errNote(msg.err)
			return nil
		}
		a.games = msg.games
		a.selected = min(a.selected, max(0, len(a.games)-1))
		return nil
	case pgnImportedMsg:
		if msg.err != nil && len(msg.ids) == 0 {
			a.note = errNote(msg.err)
			return nil
		}
		a.note = okNote("Imported %d game%s", len(msg.ids), plural(len(msg.ids)))
		return tea.Batch(a.loadLibrary(), a.Open(msg.ids[0]))
	case gameOpenedMsg:
		a.loading = false
		if msg.err != nil {
			a.note = errNote(msg.err)
			return nil
		}
		a.id, a.game, a.cursor, a.evals = msg.id, msg.game, msg.game.Root, msg.evals
		a.input, a.liveResult, a.liveNode, a.deleting = moveInput{}, nil, nil, false
		a.flipped = msg.game.Tags["Black"] == "You"
		a.note = note{}
		if msg.game.Err != nil {
			a.note = badNote("Stopped reading at an illegal move · " + msg.game.Err.Error())
		}
		return nil
	case reviewProgressMsg:
		if msg.id != a.id {
			return nil
		}
		a.evals[msg.ply] = msg.eval
		a.reviewDone++
		return a.waitReview()
	case reviewDoneMsg:
		if msg.id != a.id {
			return nil
		}
		a.reviewing, a.reviewUpdates = false, nil
		switch {
		case errors.Is(msg.err, context.Canceled):
			a.note = plainNote("Review paused")
		case msg.err != nil:
			a.note = errNote(msg.err)
		default:
			a.note = okNote("Review complete · j k jump between mistakes")
		}
		return a.loadLibrary()
	case liveEvalMsg:
		if msg.node != a.liveNode {
			return nil
		}
		if msg.err != nil {
			if !errors.Is(msg.err, context.Canceled) {
				a.note, a.live = errNote(msg.err), false
			}
			return nil
		}
		a.liveResult = &msg.analysis
		return nil
	case analyzeSavedMsg:
		if msg.err != nil {
			a.note = errNote(msg.err)
		}
		return nil
	}

	key, ok := keyString(msg)
	if !ok {
		return nil
	}
	if a.prompt != nil {
		return a.updatePrompt(key)
	}
	if a.game == nil {
		return a.updateLibrary(key)
	}
	return a.updateGame(key)
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func (a *Analyze) updatePrompt(key string) tea.Cmd {
	switch key {
	case "esc":
		a.prompt = nil
	case "enter":
		value := a.prompt.value
		purpose := a.purpose
		a.prompt = nil
		if purpose == "comment" {
			a.cursor.Comment = strings.TrimSpace(value)
			return a.save()
		}
		path := cleanPath(value)
		if path == "" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			a.note = errNote(err)
			return nil
		}
		a.note = busyNote("Importing " + filepath.Base(path))
		return a.ImportPGN(string(data), "file")
	default:
		a.prompt.handle(key)
	}
	return nil
}

func (a *Analyze) updateLibrary(key string) tea.Cmd {
	if key != "d" {
		a.deleting = false
	}
	switch key {
	case "up", "k":
		a.selected = max(0, a.selected-1)
	case "down", "j":
		a.selected = max(0, min(len(a.games)-1, a.selected+1))
	case "enter", "right", "l":
		if a.selected < len(a.games) {
			return a.Open(a.games[a.selected].ID)
		}
	case "o":
		a.prompt, a.purpose = &textInput{}, "open"
	case "n":
		return a.newBoard()
	case "d":
		if a.selected >= len(a.games) {
			return nil
		}
		if !a.deleting {
			a.deleting, a.note = true, plainNote("Press d again to delete this game")
			return nil
		}
		a.deleting = false
		id, store := a.games[a.selected].ID, a.ctx.Session.Store
		a.note = plainNote("Deleted")
		return tea.Sequence(func() tea.Msg { _ = store.DeleteGame(context.Background(), id); return nil }, a.loadLibrary())
	}
	return nil
}

// newBoard starts an empty analysis board saved like any other game.
func (a *Analyze) newBoard() tea.Cmd {
	game, _ := pgn.New("")
	now := time.Now()
	game.SetTag("Event", "Analysis board")
	game.SetTag("Date", now.Format("2006.01.02"))
	record := domain.Game{ID: fmt.Sprintf("board-%d", now.UnixNano()), Source: "board", Variant: "standard",
		White: domain.Player{Name: "White"}, Black: domain.Player{Name: "Black"}, Result: "*", PGN: game.String(), PlayedAt: now}
	store := a.ctx.Session.Store
	return func() tea.Msg {
		if err := store.UpsertGame(context.Background(), record); err != nil {
			return gameOpenedMsg{err: err}
		}
		return gameOpenedMsg{id: record.ID, game: game, evals: map[int]domain.Analysis{}}
	}
}

func (a *Analyze) updateGame(key string) tea.Cmd {
	switch key {
	case "up", "down", "left", "right":
		// Arrows walk the game here, so they never reach the move cursor.
	default:
		if move, problem, used := a.input.handle(key, a.cursor.Position(), a.flipped); used {
			if problem != "" {
				a.note = badNote(problem)
			}
			if move != "" {
				return a.play(move)
			}
			return nil
		}
	}
	switch key {
	case "left":
		if a.cursor.Parent != nil {
			a.cursor = a.cursor.Parent
		}
	case "right":
		if len(a.cursor.Children) > 0 {
			a.cursor = a.cursor.Children[0]
		}
	case "up", "down":
		a.switchLine(key == "down")
	case "home":
		a.cursor = a.game.Root
	case "end":
		a.cursor = tipOf(a.cursor)
	case "j", "k":
		a.jumpToMistake(key == "j")
	case "space":
		a.live = !a.live
		if !a.live {
			a.stopLive()
			return nil
		}
		a.stopReview()
		return a.liveSearch()
	case "r":
		if a.reviewing {
			a.stopReview()
			return nil
		}
		a.stopLive()
		return a.startReview()
	case "c":
		if a.cursor.Parent != nil {
			a.prompt, a.purpose = &textInput{value: a.cursor.Comment}, "comment"
		}
		return nil
	case "x":
		if a.cursor.Parent == nil {
			return nil
		}
		removed := a.cursor
		a.cursor = removed.Parent
		mainline := onMainline(removed)
		removed.Delete()
		if mainline {
			a.trimEvals()
		}
		a.note = plainNote("Move deleted")
		return a.save()
	case "p":
		if a.cursor.Parent != nil && !onMainline(a.cursor) {
			for node := a.cursor; node.Parent != nil; node = node.Parent {
				node.Promote()
			}
			a.evals = map[int]domain.Analysis{}
			id, store := a.id, a.ctx.Session.Store
			a.note = plainNote("Line promoted · review again for new marks")
			return tea.Batch(a.save(), func() tea.Msg { _ = store.ClearAnalysis(context.Background(), id); return nil })
		}
	case "v":
		a.flipped = !a.flipped
	case "y":
		a.note = okNote("PGN copied")
		return tea.SetClipboard(a.game.String())
	case "w":
		dir := filepath.Join(a.ctx.Session.Paths.DataDir, "exports")
		path := filepath.Join(dir, a.id+".pgn")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			a.note = errNote(err)
			return nil
		}
		if err := os.WriteFile(path, []byte(a.game.String()), 0o600); err != nil {
			a.note = errNote(err)
			return nil
		}
		a.note = okNote("Saved %s", shortPath(path, 60))
		return nil
	case "esc", "backspace":
		a.stopWork()
		a.game, a.cursor = nil, nil
		return a.loadLibrary()
	}
	if a.live {
		return a.liveSearch()
	}
	return nil
}

func (a *Analyze) play(move string) tea.Cmd {
	node, err := a.cursor.Play(move)
	if err != nil {
		a.note = errNote(err)
		return nil
	}
	a.cursor = node
	a.note = note{}
	commands := []tea.Cmd{a.save()}
	if a.live {
		commands = append(commands, a.liveSearch())
	}
	return tea.Batch(commands...)
}

func onMainline(node *pgn.Node) bool {
	for ; node.Parent != nil; node = node.Parent {
		if node.Parent.Children[0] != node {
			return false
		}
	}
	return true
}

// switchLine moves to the previous or next alternative of the current move.
func (a *Analyze) switchLine(next bool) {
	if a.cursor.Parent == nil {
		return
	}
	siblings := a.cursor.Parent.Children
	for i, sibling := range siblings {
		if sibling != a.cursor {
			continue
		}
		if next && i+1 < len(siblings) {
			a.cursor = siblings[i+1]
		} else if !next && i > 0 {
			a.cursor = siblings[i-1]
		}
		return
	}
}

func (a *Analyze) trimEvals() {
	plies := len(a.game.Mainline())
	for ply := range a.evals {
		if ply > plies {
			delete(a.evals, ply)
		}
	}
}

func (a *Analyze) save() tea.Cmd {
	id, text, store := a.id, a.game.String(), a.ctx.Session.Store
	moves, result := tipOf(a.game.Root).Line(), a.game.Result
	return func() tea.Msg {
		return analyzeSavedMsg{err: store.UpdateGamePGN(context.Background(), id, text, moves, result)}
	}
}

// --- engine work -----------------------------------------------------------

func (a *Analyze) stopWork() {
	a.stopReview()
	a.stopLive()
	a.live = false
}

func (a *Analyze) stopLive() {
	if a.liveCancel != nil {
		a.liveCancel()
		a.liveCancel = nil
	}
}

func (a *Analyze) stopReview() {
	if a.reviewCancel != nil {
		a.reviewCancel()
		a.reviewCancel = nil
	}
}

func (a *Analyze) liveSearch() tea.Cmd {
	analyzer, err := a.ctx.Session.Engine()
	if err != nil {
		a.note, a.live = errNote(err), false
		return nil
	}
	a.stopLive()
	if a.liveNode == a.cursor && a.liveResult != nil {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.liveCancel, a.liveNode, a.liveResult = cancel, a.cursor, nil
	node := a.cursor
	request := engine.Request{FEN: startFEN(a.game), Moves: node.Line()}
	limit := engine.Limit{Depth: a.ctx.Session.Config.AnalysisDepth, MultiPV: 3}
	return func() tea.Msg {
		analysis, err := analyzer.Search(ctx, request, limit)
		return liveEvalMsg{node: node, analysis: analysis, err: err}
	}
}

func (a *Analyze) startReview() tea.Cmd {
	analyzer, err := a.ctx.Session.Engine()
	if err != nil {
		a.note = errNote(err)
		return nil
	}
	mainline := a.game.Mainline()
	depth := a.ctx.Session.Config.ReviewDepth
	todo := []int{}
	for ply := 0; ply <= len(mainline); ply++ {
		if eval, ok := a.evals[ply]; !ok || eval.Depth < depth {
			todo = append(todo, ply)
		}
	}
	if len(todo) == 0 {
		a.note = okNote("Already reviewed at depth %d", depth)
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.reviewCancel, a.reviewing, a.reviewDone = cancel, true, len(a.evals)
	updates := make(chan tea.Msg, 4)
	a.reviewUpdates = updates
	id, start, store := a.id, startFEN(a.game), a.ctx.Session.Store
	nodes := append([]*pgn.Node{a.game.Root}, mainline...)
	a.note = note{}
	go func() {
		send := func(msg tea.Msg) bool {
			select {
			case updates <- msg:
				return true
			case <-ctx.Done():
				return false
			}
		}
		for _, ply := range todo {
			node := nodes[ply]
			eval, err := evaluate(ctx, analyzer, start, node, depth)
			if err != nil {
				updates <- reviewDoneMsg{id: id, err: err}
				return
			}
			eval.GameID, eval.Ply = id, ply
			_ = store.UpsertAnalysis(context.Background(), eval)
			if !send(reviewProgressMsg{id: id, ply: ply, eval: eval}) {
				updates <- reviewDoneMsg{id: id, err: ctx.Err()}
				return
			}
		}
		updates <- reviewDoneMsg{id: id}
	}()
	return a.waitReview()
}

func (a *Analyze) waitReview() tea.Cmd {
	updates := a.reviewUpdates
	if updates == nil {
		return nil
	}
	return func() tea.Msg { return <-updates }
}

// evaluate scores the position after node from White's point of view.
// Finished positions are scored without asking the engine.
func evaluate(ctx context.Context, analyzer *engine.Engine, start string, node *pgn.Node, depth int) (domain.Analysis, error) {
	position := node.Position()
	white := position.Turn() == chesscore.White
	switch outcome := position.Outcome(); {
	case strings.HasPrefix(outcome, "Checkmate"):
		score := 10000
		if white {
			score = -10000 // White is to move and mated
		}
		return domain.Analysis{Depth: depth, Score: score}, nil
	case outcome != "":
		return domain.Analysis{Depth: depth}, nil
	}
	analysis, err := analyzer.Search(ctx, engine.Request{FEN: start, Moves: node.Line()}, engine.Limit{Depth: depth})
	if err != nil {
		return domain.Analysis{}, err
	}
	cp, mate := engine.WhiteScore(analysis.ScoreCP, analysis.Mate, white)
	return domain.Analysis{Depth: analysis.Depth, Score: cp, Mate: mate, PV: analysis.PV}, nil
}

// --- review data -------------------------------------------------------------

func (a *Analyze) reviewData() ([]review.Eval, []review.Mark, [2]review.Summary) {
	mainline := a.game.Mainline()
	evals := make([]review.Eval, len(mainline)+1)
	for ply := range evals {
		if eval, ok := a.evals[ply]; ok {
			evals[ply] = review.Eval{CP: eval.Score, Mate: eval.Mate, Set: true}
		}
	}
	_, whiteFirst := a.game.FullMove()
	marks, summaries := review.Game(evals, whiteFirst)
	// Playing the engine's own first choice is never a mistake.
	for i, node := range mainline {
		if before, ok := a.evals[i]; ok && len(before.PV) > 0 && before.PV[0] == node.UCI && i < len(marks) {
			marks[i] = review.None
		}
	}
	return evals, marks, summaries
}

func (a *Analyze) jumpToMistake(forward bool) {
	_, marks, _ := a.reviewData()
	mainline := a.game.Mainline()
	current := -1
	for i, node := range mainline {
		if node == a.cursor {
			current = i
		}
	}
	if forward {
		for i := current + 1; i < len(marks); i++ {
			if marks[i] >= review.Mistake {
				a.cursor = mainline[i]
				return
			}
		}
	} else {
		for i := min(current, len(marks)) - 1; i >= 0; i-- {
			if marks[i] >= review.Mistake {
				a.cursor = mainline[i]
				return
			}
		}
	}
	a.note = plainNote("No more mistakes that way")
}

// currentEval is the best evaluation known for the cursor position.
func (a *Analyze) currentEval() (domain.Analysis, []engine.Line, bool) {
	if a.liveResult != nil && a.liveNode == a.cursor {
		white := a.cursor.Position().Turn() == chesscore.White
		cp, mate := engine.WhiteScore(a.liveResult.ScoreCP, a.liveResult.Mate, white)
		return domain.Analysis{Depth: a.liveResult.Depth, Score: cp, Mate: mate, PV: a.liveResult.PV}, a.liveResult.Lines, true
	}
	if onMainline(a.cursor) {
		if eval, ok := a.evals[a.cursor.Ply]; ok {
			return eval, nil, true
		}
	}
	return domain.Analysis{}, nil, false
}

// --- views -------------------------------------------------------------------

func (a *Analyze) View(width, height int) string {
	if a.game == nil {
		return a.libraryView(width, height)
	}
	position := a.cursor.Position()
	board := a.input.decorate(widgets.Board{
		Theme: *a.ctx.Theme, Flipped: a.flipped, Glyphs: a.ctx.Session.Config.Glyphs, LastMove: a.cursor.UCI,
	}, position)
	if eval, _, ok := a.currentEval(); ok && len(eval.PV) > 0 {
		board.Suggest = eval.PV[0]
	}
	render := func(b widgets.Board) string { return b.Render(position) }
	return boardAndSide(board, render, func(w int) string { return a.side(w, height) }, a.compact, width, height)
}

func (a *Analyze) libraryView(width, height int) string {
	t := a.ctx.Theme
	rowWidth := min(84, width)
	if a.prompt != nil {
		return t.Title().Render("Open a PGN file") + "\n\n" + a.ctx.field("Path", *a.prompt) + "\n\n" +
			t.Faint().Render("drop a file here, or paste PGN text on the library")
	}
	head := t.Title().Render("Analyze") + "   " + t.Faint().Render(fmt.Sprintf("%d game%s", len(a.games), plural(len(a.games))))
	if len(a.games) == 0 {
		lines := []string{head, "",
			t.Muted().Render("Your games appear here after you play one, import a PGN,"),
			t.Muted().Render("or copy one from Lichess."), "",
			t.Faint().Render("o open a PGN file · paste PGN text · n new board")}
		if a.note.text != "" {
			lines = append(lines, "", a.ctx.note(a.note))
		}
		return strings.Join(lines, "\n")
	}
	rows := make([]string, len(a.games))
	for i, game := range a.games {
		players := a.ctx.item(game.White, i == a.selected) + t.Faint().Render(" – ") + a.ctx.item(game.Black, i == a.selected)
		when := game.PlayedAt
		if when.IsZero() {
			when = game.SavedAt
		}
		meta := sourceLabel(game.Source) + " · " + when.Format("Jan 2")
		if game.Reviewed {
			meta = "reviewed · " + meta
		}
		result := strings.NewReplacer("1/2-1/2", "½–½", "1-0", "1–0", "0-1", "0–1", "*", "…").Replace(game.Result)
		left := padRight(ansi.Truncate(players, max(10, rowWidth-36), "…"), max(12, rowWidth-34)) + t.Muted().Render(padRight(result, 6))
		rows[i] = padRight(left, max(20, rowWidth-2-lipgloss.Width(meta))) + t.Faint().Render(meta)
	}
	out := head + "\n\n" + a.ctx.menu(rows, a.selected, max(3, height-6), rowWidth, true)
	if a.note.text != "" {
		out += "\n\n" + a.ctx.note(a.note)
	}
	return lipgloss.NewStyle().Width(rowWidth).Render(out)
}

func sourceLabel(source string) string {
	switch source {
	case "local":
		return "played here"
	case "lichess":
		return "lichess"
	case "board":
		return "board"
	}
	return "pgn"
}

func (a *Analyze) side(width, height int) string {
	white, black := tagOr(a.game, "White", "White"), tagOr(a.game, "Black", "Black")
	evals, marks, summaries := a.reviewData()
	top, bottom := a.ctx.playerLine(false, black, 0, accuracyText(summaries[1]), false, width), a.ctx.playerLine(true, white, 0, accuracyText(summaries[0]), false, width)
	if a.flipped {
		top, bottom = bottom, top
	}

	lines := []string{top, ""}
	lines = append(lines, a.evalLine(width))
	if graph := a.graph(evals, marks, width); graph != "" {
		lines = append(lines, graph)
	}
	lines = append(lines, "")
	footer := a.footerLines(width)
	listRows := max(2, height-len(lines)-len(footer)-3)
	lines = append(lines, a.movetext(marks, width, listRows), "", bottom)
	lines = append(lines, footer...)
	return strings.Join(lines, "\n")
}

func accuracyText(summary review.Summary) string {
	if summary.Moves == 0 {
		return ""
	}
	return fmt.Sprintf("%.0f%%", summary.Accuracy)
}

func (a *Analyze) evalLine(width int) string {
	t := a.ctx.Theme
	eval, _, ok := a.currentEval()
	label := t.Faint().Render("no evaluation · space for engine, r to review")
	switch {
	case a.reviewing:
		total := len(a.game.Mainline()) + 1
		label = t.Muted().Render(fmt.Sprintf("… reviewing %d of %d", min(a.reviewDone, total), total))
	case a.live && !ok:
		label = t.Muted().Render("… thinking")
	case ok:
		score := formatScore(engine.Analysis{ScoreCP: eval.Score, Mate: eval.Mate})
		label = t.Accent().Bold(true).Render(score) + t.Faint().Render(fmt.Sprintf("  depth %d", eval.Depth))
		if a.live {
			label += t.Faint().Render(" · live")
		}
	}
	return ansi.Truncate(label, width, "…")
}

// graph is the evaluation over the game, with the current move marked.
func (a *Analyze) graph(evals []review.Eval, marks []review.Mark, width int) string {
	values, known := make([]float64, len(evals)), 0
	last := 50.0
	for i, eval := range evals {
		if eval.Set {
			last = eval.WinPercent()
			known++
		}
		values[i] = last
	}
	if known < 2 {
		return ""
	}
	bars := review.Spark(values, width)
	current := -1
	if onMainline(a.cursor) {
		current = review.Column(a.cursor.Ply, len(values), width)
	}
	bad := map[int]bool{}
	for i, mark := range marks {
		if mark >= review.Mistake {
			bad[review.Column(i+1, len(values), width)] = true
		}
	}
	th := a.ctx.Theme
	var out strings.Builder
	for col, bar := range bars {
		style := th.Faint()
		switch {
		case col == current:
			style = th.Accent()
		case bad[col]:
			style = th.Bad()
		}
		out.WriteString(style.Render(string(bar)))
	}
	return out.String()
}

func (a *Analyze) footerLines(width int) []string {
	th := a.ctx.Theme
	var lines []string
	if a.cursor.Pre != "" {
		lines = append(lines, "", wrapText(th.Muted(), a.cursor.Pre, width))
	}
	if a.cursor.Comment != "" {
		lines = append(lines, "", wrapText(th.Muted(), a.cursor.Comment, width))
	}
	_, engineLines, _ := a.currentEval()
	if len(engineLines) > 0 {
		lines = append(lines, "")
		white := a.cursor.Position().Turn() == chesscore.White
		for _, line := range engineLines {
			cp, mate := engine.WhiteScore(line.ScoreCP, line.Mate, white)
			score := th.Muted().Render(padRight(formatScore(engine.Analysis{ScoreCP: cp, Mate: mate}), 7))
			lines = append(lines, ansi.Truncate(score+th.Body().Render(sanLine(a.cursor, line.PV, 8)), width, "…"))
		}
	}
	if a.prompt != nil && a.purpose == "comment" {
		lines = append(lines, "", a.ctx.field("Comment", *a.prompt))
	} else {
		lines = append(lines, "", a.input.prompt(a.ctx))
	}
	if a.note.text != "" {
		lines = append(lines, a.ctx.note(a.note))
	}
	return lines
}

// sanLine renders up to limit moves of a UCI line in SAN from node.
func sanLine(node *pgn.Node, pv []string, limit int) string {
	position := node.Position()
	var words []string
	for i, move := range pv {
		if i == limit {
			break
		}
		number := ""
		if position.Turn() == chesscore.White {
			number = fmt.Sprintf("%d.", fullMoveNumber(position.FEN()))
		} else if i == 0 {
			number = fmt.Sprintf("%d…", fullMoveNumber(position.FEN()))
		}
		words = append(words, number+position.SAN(move))
		if position.ApplyUCI(move) != nil {
			break
		}
	}
	return strings.Join(words, " ")
}

func fullMoveNumber(fen string) int {
	fields := strings.Fields(fen)
	n := 1
	if len(fields) >= 6 {
		fmt.Sscan(fields[5], &n)
	}
	return n
}

func wrapText(style lipgloss.Style, text string, width int) string {
	return style.Width(max(10, width)).Render(text)
}

type moveToken struct {
	text string
	node *pgn.Node
	side bool // inside a side line
}

// movetext lays the game out like a book: main line with side lines in
// brackets, wrapped to width, scrolled so the current move is visible.
func (a *Analyze) movetext(marks []review.Mark, width, rows int) string {
	th := a.ctx.Theme
	markOf := map[*pgn.Node]review.Mark{}
	for i, node := range a.game.Mainline() {
		if i < len(marks) {
			markOf[node] = marks[i]
		}
	}
	full, startWhite := a.game.FullMove()
	var tokens []moveToken
	var walk func(node *pgn.Node, side bool, forceNumber bool)
	emit := func(node *pgn.Node, side, forceNumber bool) {
		number := node.MoveNumber(full, startWhite)
		if node.WhiteMoved() {
			tokens = append(tokens, moveToken{text: fmt.Sprintf("%d.", number), side: side})
		} else if forceNumber {
			tokens = append(tokens, moveToken{text: fmt.Sprintf("%d…", number), side: side})
		}
		text := node.SAN
		for _, nag := range node.NAGs {
			text += pgn.Glyph(nag)
		}
		tokens = append(tokens, moveToken{text: text, node: node, side: side})
	}
	walk = func(node *pgn.Node, side bool, forceNumber bool) {
		for len(node.Children) > 0 {
			main := node.Children[0]
			emit(main, side, forceNumber)
			forceNumber = false
			for _, alt := range node.Children[1:] {
				tokens = append(tokens, moveToken{text: "(", side: true})
				emit(alt, true, true)
				walk(alt, true, false)
				tokens = append(tokens, moveToken{text: ")", side: true})
				forceNumber = true
			}
			node = main
		}
	}
	walk(a.game.Root, false, !startWhite)
	if len(tokens) == 0 {
		return th.Faint().Render("no moves yet · type one to start")
	}

	var lines []string
	var line strings.Builder
	lineWidth, cursorLine := 0, 0
	for _, token := range tokens {
		text := token.text
		style := th.Body()
		switch {
		case token.node == a.cursor && token.node != nil:
			style = th.Accent().Bold(true)
		case token.side:
			style = th.Muted()
		case token.node == nil:
			style = th.Faint()
		}
		rendered := style.Render(text)
		if mark := markOf[token.node]; mark != review.None {
			markStyle := th.Accent()
			if mark >= review.Mistake {
				markStyle = th.Bad()
			}
			rendered += markStyle.Render(mark.Glyph())
			text += mark.Glyph()
		}
		w := lipgloss.Width(text)
		joined := token.text == ")" || strings.HasSuffix(lastRuneString(line.String()), "(")
		if lineWidth > 0 && lineWidth+1+w > width {
			lines = append(lines, line.String())
			line.Reset()
			lineWidth = 0
		}
		if lineWidth > 0 && !joined {
			line.WriteByte(' ')
			lineWidth++
		}
		line.WriteString(rendered)
		lineWidth += w
		if token.node == a.cursor && token.node != nil {
			cursorLine = len(lines)
		}
	}
	lines = append(lines, line.String())
	start := 0
	if len(lines) > rows {
		start = max(0, min(len(lines)-rows, cursorLine-rows/2))
	}
	end := min(len(lines), start+rows)
	return strings.Join(lines[start:end], "\n")
}

func lastRuneString(s string) string {
	plain := ansi.Strip(s)
	if plain == "" {
		return ""
	}
	r := []rune(plain)
	return string(r[len(r)-1])
}

func (a *Analyze) compact(width int) string {
	lines := []string{a.evalLine(width)}
	if a.cursor.Parent != nil {
		lines = append(lines, a.ctx.Theme.Muted().Render(fmt.Sprintf("%d. %s", a.cursor.MoveNumber(a.game.FullMove()), a.cursor.SAN)))
	}
	lines = append(lines, a.input.prompt(a.ctx))
	if a.note.text != "" {
		lines = append(lines, a.ctx.note(a.note))
	}
	return strings.Join(lines, "\n")
}

func (a *Analyze) Hints() []widgets.Hint {
	switch {
	case a.prompt != nil:
		return []widgets.Hint{{Key: "enter", Label: "save"}, {Key: "esc", Label: "cancel"}}
	case a.game == nil:
		return []widgets.Hint{{Key: "↑↓", Label: "select"}, {Key: "enter", Label: "open"}, {Key: "o", Label: "open PGN"}, {Key: "n", Label: "new board"}, {Key: "d", Label: "delete"}}
	}
	return []widgets.Hint{
		{Key: "←→", Label: "step"}, {Key: "↑↓", Label: "side lines"}, {Key: "r", Label: "review"}, {Key: "space", Label: "engine"},
		{Key: "j k", Label: "mistakes"}, {Key: "e4 Nf3", Label: "try a move"}, {Key: "c", Label: "comment"},
		{Key: "x", Label: "delete move"}, {Key: "p", Label: "promote line"}, {Key: "v", Label: "flip"},
		{Key: "y", Label: "copy PGN"}, {Key: "w", Label: "write PGN file"}, {Key: "esc", Label: "library"},
	}
}

func (a *Analyze) Status() string {
	if a.game != nil {
		return a.game.Title()
	}
	return ""
}

func (a *Analyze) CapturesKey(key string) bool {
	if a.prompt != nil {
		return true
	}
	if a.game == nil {
		return false
	}
	return key == "esc" || a.input.capturesKey(key)
}
