package screens

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	chesscore "github.com/kannachi323/chesstui/internal/chess"
	"github.com/kannachi323/chesstui/internal/domain"
	"github.com/kannachi323/chesstui/internal/puzzles"
	"github.com/kannachi323/chesstui/internal/storage"
	"github.com/kannachi323/chesstui/internal/ui/widgets"
)

type puzzleLoadedMsg struct {
	puzzle domain.Puzzle
	err    error
}

type puzzleRecordedMsg struct{ err error }
type puzzleImportedMsg struct {
	progress puzzles.Progress
	err      error
}

type Puzzles struct {
	ctx       Context
	activated bool
	puzzle    domain.Puzzle
	position  *chesscore.Position
	solver    chesscore.Color
	step      int
	input     moveInput
	note      note
	loading   bool
	started   time.Time
	hints     int
	failed    bool
	solved    bool
	importing *textInput
}

func NewPuzzles(ctx Context) *Puzzles { return &Puzzles{ctx: ctx} }
func (p *Puzzles) Init() tea.Cmd      { return nil }

// Activate loads the daily puzzle the first time the screen opens.
func (p *Puzzles) Activate() tea.Cmd {
	if p.activated {
		return nil
	}
	p.activated, p.loading = true, true
	return p.loadDaily
}

func (p *Puzzles) loadDaily() tea.Msg {
	puzzle, err := p.ctx.Session.Client().DailyPuzzle(context.Background())
	return puzzleLoadedMsg{puzzle: puzzle, err: err}
}

func (p *Puzzles) loadNext() tea.Msg {
	puzzle, err := p.ctx.Session.Client().NextPuzzle(context.Background())
	return puzzleLoadedMsg{puzzle: puzzle, err: err}
}

func (p *Puzzles) loadLocal() tea.Msg {
	cfg := p.ctx.Session.Config
	puzzle, err := p.ctx.Session.Store.RandomPuzzle(context.Background(), cfg.PuzzleRatingFloor, cfg.PuzzleRatingCeil)
	if errors.Is(err, storage.ErrNotFound) {
		err = errors.New("No saved puzzles yet · press i to import a Lichess pack")
	}
	return puzzleLoadedMsg{puzzle: puzzle, err: err}
}

// preparePuzzle returns the position the solver faces. Lichess API puzzles
// already start there. Rows from the Lichess CSV database start one move
// earlier: their first move is the opponent's, which sets up the tactic.
func preparePuzzle(puzzle domain.Puzzle) (domain.Puzzle, *chesscore.Position, error) {
	position, err := chesscore.NewPosition(puzzle.FEN)
	if err != nil {
		return puzzle, nil, err
	}
	if puzzle.Source != "lichess" && puzzle.LastMove == "" && len(puzzle.Moves) > 1 {
		if err := position.ApplyUCI(puzzle.Moves[0]); err != nil {
			return puzzle, nil, err
		}
		puzzle.LastMove, puzzle.Moves = puzzle.Moves[0], puzzle.Moves[1:]
	}
	if len(puzzle.Moves) == 0 {
		return puzzle, nil, errors.New("this puzzle has no solution moves")
	}
	return puzzle, position, nil
}

func (p *Puzzles) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case puzzleLoadedMsg:
		p.loading = false
		if msg.err != nil {
			p.note = errNote(msg.err)
			return nil
		}
		puzzle, position, err := preparePuzzle(msg.puzzle)
		if err != nil {
			p.note = errNote(err)
			return nil
		}
		p.puzzle, p.position, p.solver, p.step = puzzle, position, position.Turn(), 0
		p.input, p.hints, p.failed, p.solved, p.started = moveInput{}, 0, false, false, time.Now()
		p.note = plainNote("Find the best move for " + sideName(p.solver))
		store := p.ctx.Session.Store
		original := msg.puzzle
		return func() tea.Msg {
			return puzzleRecordedMsg{err: store.UpsertPuzzle(context.Background(), original)}
		}
	case puzzleRecordedMsg:
		if msg.err != nil {
			p.note = errNote(msg.err)
		}
		return nil
	case puzzleImportedMsg:
		if msg.err != nil {
			p.note = errNote(msg.err)
		} else {
			p.note = okNote("Imported %d puzzles · l to play one", msg.progress.Imported)
		}
		return nil
	}

	key, ok := keyString(msg)
	if !ok {
		return nil
	}
	if p.importing != nil {
		return p.updateImport(key)
	}
	if p.position != nil && !p.solved && !p.loading {
		if move, problem, used := p.input.handle(key, p.position, p.solver == chesscore.Black); used {
			if problem != "" {
				p.note = badNote(problem)
			}
			if move != "" {
				return p.submit(move)
			}
			return nil
		}
	}
	switch key {
	case "t":
		p.loading = true
		return p.loadDaily
	case "n":
		p.loading = true
		return p.loadNext
	case "l":
		p.loading = true
		return p.loadLocal
	case "i":
		p.importing = &textInput{}
	case "space":
		if p.position != nil && !p.solved && p.step < len(p.puzzle.Moves) {
			p.hints++
			move := p.puzzle.Moves[p.step]
			if p.hints == 1 {
				p.input.cursor = move[:2]
				p.note = plainNote("Try the piece on " + move[:2])
			} else {
				p.note = plainNote("The move is " + p.position.SAN(move))
			}
		}
	}
	return nil
}

func (p *Puzzles) updateImport(key string) tea.Cmd {
	switch key {
	case "esc":
		p.importing = nil
	case "enter":
		path := strings.TrimSpace(p.importing.value)
		p.importing = nil
		if path == "" {
			return nil
		}
		p.note = busyNote("Importing " + path)
		store := p.ctx.Session.Store
		return func() tea.Msg {
			progress, err := puzzles.ImportFile(context.Background(), store, path, nil)
			return puzzleImportedMsg{progress: progress, err: err}
		}
	default:
		p.importing.handle(key)
	}
	return nil
}

func (p *Puzzles) submit(move string) tea.Cmd {
	if p.step >= len(p.puzzle.Moves) {
		return nil
	}
	want := strings.ToLower(p.puzzle.Moves[p.step])
	final := p.step == len(p.puzzle.Moves)-1
	accepted := move == want
	if !accepted && final {
		// Any mate is as good as the listed one.
		clone := p.position.Clone()
		accepted = clone.ApplyUCI(move) == nil && strings.HasPrefix(clone.Outcome(), "Checkmate")
	}
	if !accepted {
		p.note = badNote(p.position.SAN(move) + " isn’t it · try again")
		if p.failed {
			return nil
		}
		p.failed = true
		return p.record(false)
	}
	if err := p.position.ApplyUCI(move); err != nil {
		p.note = errNote(err)
		return nil
	}
	p.step++
	// Lichess solutions alternate the solver's move with the forced reply.
	if p.step < len(p.puzzle.Moves) {
		_ = p.position.ApplyUCI(p.puzzle.Moves[p.step])
		p.step++
	}
	if p.step < len(p.puzzle.Moves) {
		p.note = okNote("Good · keep going")
		return nil
	}
	p.solved, p.note = true, okNote("Solved")
	if p.failed {
		return nil
	}
	return p.record(true)
}

func (p *Puzzles) record(solved bool) tea.Cmd {
	attempt := domain.PuzzleAttempt{PuzzleID: p.puzzle.ID, Solved: solved, Hints: p.hints, Elapsed: time.Since(p.started)}
	store := p.ctx.Session.Store
	return func() tea.Msg {
		return puzzleRecordedMsg{err: store.RecordPuzzleAttempt(context.Background(), attempt)}
	}
}

func (p *Puzzles) View(width, height int) string {
	t := p.ctx.Theme
	if p.importing != nil {
		return t.Title().Render("Import a puzzle pack") + "\n\n" + p.ctx.field("path", *p.importing) + "\n\n" +
			t.Faint().Render("Lichess puzzle CSV or .csv.zst · database.lichess.org")
	}
	if p.position == nil {
		if p.loading {
			return t.Muted().Render("… loading today’s puzzle")
		}
		return t.Title().Render("Puzzles") + "\n\n" + p.ctx.note(p.note) + "\n\n" + t.Faint().Render("t today · n next · l saved")
	}
	board := p.input.decorate(widgets.Board{
		Theme: *t, Flipped: p.solver == chesscore.Black, Glyphs: p.ctx.Session.Config.Glyphs, LastMove: lastMove(p.position),
	}, p.position)
	if p.position.Ply() == 0 {
		board.LastMove = p.puzzle.LastMove
	}
	render := func(b widgets.Board) string { return b.Render(p.position) }
	return boardAndSide(board, render, p.side, p.compact, width, height)
}

func (p *Puzzles) title() string {
	t := p.ctx.Theme
	label := "Puzzle"
	if p.puzzle.Source != "lichess" {
		label = "Saved puzzle"
	}
	title := t.Title().Render(label)
	if p.puzzle.Rating > 0 {
		title += "  " + t.Accent().Render(fmt.Sprint(p.puzzle.Rating))
	}
	if p.loading {
		title += "  " + t.Faint().Render("loading…")
	}
	return title
}

func (p *Puzzles) side(width int) string {
	t := p.ctx.Theme
	lines := []string{p.title(), t.Faint().Render("#" + p.puzzle.ID)}
	if themes := humanThemes(p.puzzle.Themes, 3); themes != "" {
		lines = append(lines, "", t.Muted().Render(themes))
	}
	turn := "White to move"
	if p.solver == chesscore.Black {
		turn = "Black to move"
	}
	lines = append(lines, "", t.Body().Render(turn), "")
	progress := p.progress()
	if progress != "" {
		lines = append(lines, progress, "")
	}
	if !p.solved {
		lines = append(lines, p.input.prompt(p.ctx))
	}
	if p.note.text != "" {
		lines = append(lines, p.ctx.note(p.note))
	}
	return lipgloss.NewStyle().Width(width).Render(strings.Join(lines, "\n"))
}

func (p *Puzzles) compact(width int) string {
	lines := []string{p.title()}
	if !p.solved {
		lines = append(lines, p.input.prompt(p.ctx))
	}
	if p.note.text != "" {
		lines = append(lines, p.ctx.note(p.note))
	}
	return strings.Join(lines, "\n")
}

// progress draws one pip per solver move: done, current, to come.
func (p *Puzzles) progress() string {
	total := (len(p.puzzle.Moves) + 1) / 2
	done := (p.step + 1) / 2
	if total <= 1 {
		return ""
	}
	pips := make([]string, total)
	for i := range pips {
		switch {
		case i < done:
			pips[i] = p.ctx.Theme.Good().Render("●")
		case i == done && !p.solved:
			pips[i] = p.ctx.Theme.Accent().Render("●")
		default:
			pips[i] = p.ctx.Theme.Faint().Render("○")
		}
	}
	return strings.Join(pips, " ")
}

var camelBoundary = regexp.MustCompile(`([a-z])([A-Z0-9])`)

// humanThemes turns Lichess theme keys such as "mateIn3" into "mate in 3".
func humanThemes(themes []string, limit int) string {
	skip := map[string]bool{"short": true, "long": true, "veryLong": true, "oneMove": true, "master": true, "masterVsMaster": true, "superGM": true}
	words := []string{}
	for _, theme := range themes {
		if skip[theme] {
			continue
		}
		words = append(words, strings.ToLower(camelBoundary.ReplaceAllString(theme, "$1 $2")))
		if len(words) == limit {
			break
		}
	}
	return strings.Join(words, " · ")
}

func (p *Puzzles) Hints() []widgets.Hint {
	if p.importing != nil {
		return []widgets.Hint{{Key: "enter", Label: "import"}, {Key: "esc", Label: "back"}}
	}
	actions := []widgets.Hint{{Key: "n", Label: "next"}, {Key: "t", Label: "today"}, {Key: "l", Label: "saved"}, {Key: "i", Label: "import pack"}}
	if p.position == nil || p.solved {
		return actions
	}
	return append(append(moveHints(), widgets.Hint{Key: "space", Label: "hint"}), actions...)
}

func (p *Puzzles) Status() string {
	if p.puzzle.ID == "" {
		return ""
	}
	return "lichess.org/training/" + p.puzzle.ID
}

func (p *Puzzles) CapturesKey(key string) bool {
	if p.importing != nil {
		return true
	}
	return p.position != nil && !p.solved && p.input.capturesKey(key)
}
