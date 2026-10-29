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
	"github.com/kannachi323/chesstui/internal/lichess"
	"github.com/kannachi323/chesstui/internal/ui/widgets"
)

type studyLoadedMsg struct {
	username string
	games    []lichess.ExportedGame
	studies  []lichess.Study
	err      error
}

type studyAnalysisMsg struct {
	gameID   string
	ply      int
	analysis engine.Analysis
	err      error
}

const studyGames, studyStudies = 0, 1

// Study browses any public Lichess user's recent games and studies, replays
// a finished game move by move, and analyses positions with local Stockfish.
type Study struct {
	ctx       Context
	activated bool
	username  string
	prompt    *textInput
	section   int
	selected  [2]int
	games     []lichess.ExportedGame
	studies   []lichess.Study
	loading   bool
	note      note

	game     *lichess.ExportedGame
	position *chesscore.Position
	san      []string
	ply      int
	analysis map[int]engine.Analysis
	thinking bool
}

func NewStudy(ctx Context) *Study { return &Study{ctx: ctx} }

func (s *Study) Init() tea.Cmd { return nil }

// Activate opens the signed-in user's library, or asks whose to open.
func (s *Study) Activate() tea.Cmd {
	if s.activated {
		return nil
	}
	s.activated = true
	if account, ok := s.ctx.Session.Account(); ok {
		return s.load(account.Username)
	}
	s.prompt = &textInput{allowed: usernameChar}
	return nil
}

func (s *Study) load(username string) tea.Cmd {
	s.loading, s.note = true, busyNote("Loading "+username+"’s games")
	client, store := s.ctx.Session.Client(), s.ctx.Session.Store
	return func() tea.Msg {
		ctx := context.Background()
		message := studyLoadedMsg{username: username}
		message.err = client.RecentGames(ctx, username, 20, func(game lichess.ExportedGame) error {
			message.games = append(message.games, game)
			return nil
		})
		if message.err == nil {
			message.err = client.Studies(ctx, username, func(study lichess.Study) error {
				message.studies = append(message.studies, study)
				return nil
			})
		}
		for _, game := range message.games {
			stored := domain.Game{
				ID: game.ID, Source: "lichess", Variant: game.Variant, Moves: strings.Fields(game.Moves),
				White:  domain.Player{ID: game.Players.White.User.ID, Name: game.Players.White.User.Name, Rating: game.Players.White.Rating},
				Black:  domain.Player{ID: game.Players.Black.User.ID, Name: game.Players.Black.User.Name, Rating: game.Players.Black.Rating},
				Status: game.Status, Winner: game.Winner, Result: game.Status, PGN: game.PGN,
			}
			if game.CreatedAt > 0 {
				stored.PlayedAt = time.UnixMilli(game.CreatedAt)
			}
			_ = store.UpsertGame(ctx, stored) // the cache is a convenience
		}
		return message
	}
}

func (s *Study) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case studyLoadedMsg:
		s.loading = false
		if msg.err != nil && len(msg.games) == 0 {
			s.note = errNote(msg.err)
			return nil
		}
		s.username, s.games, s.studies, s.selected = msg.username, msg.games, msg.studies, [2]int{}
		s.note = note{}
		if len(s.games) == 0 {
			s.note = plainNote(msg.username + " has no public games yet")
		}
		return nil
	case studyAnalysisMsg:
		s.thinking = false
		if msg.err != nil {
			s.note = errNote(msg.err)
			return nil
		}
		if s.game != nil && msg.gameID == s.game.ID {
			s.analysis[msg.ply] = msg.analysis
			s.note = note{}
			best := msg.analysis
			_ = s.ctx.Session.Store.UpsertAnalysis(context.Background(), domain.Analysis{
				GameID: msg.gameID, Ply: msg.ply, Depth: best.Depth, Score: best.ScoreCP, Mate: best.Mate, PV: best.PV,
			})
		}
		return nil
	case browserOpenedMsg:
		if msg.err != nil {
			s.note = errNote(msg.err)
		}
		return nil
	}

	key, ok := keyString(msg)
	if !ok {
		return nil
	}
	if s.prompt != nil {
		switch key {
		case "esc":
			s.prompt = nil
		case "enter":
			name := strings.TrimSpace(s.prompt.value)
			if name == "" {
				return nil
			}
			s.prompt, s.game, s.position = nil, nil, nil
			return s.load(name)
		default:
			s.prompt.handle(key)
		}
		return nil
	}
	if s.game != nil {
		return s.updateReplay(key)
	}
	switch key {
	case "]", "[", "tab", "shift+tab":
		s.section = 1 - s.section
	case "up", "k":
		s.selected[s.section] = max(0, s.selected[s.section]-1)
	case "down", "j":
		s.selected[s.section] = max(0, min(s.count()-1, s.selected[s.section]+1))
	case "u", "/":
		s.prompt = &textInput{allowed: usernameChar}
	case "r":
		if s.username != "" && !s.loading {
			return s.load(s.username)
		}
	case "enter", "right", "l":
		return s.open()
	}
	return nil
}

func (s *Study) updateReplay(key string) tea.Cmd {
	last := s.position.Ply()
	switch key {
	case "left", "h":
		s.ply = max(0, s.ply-1)
	case "right", "l":
		s.ply = min(last, s.ply+1)
	case "up", "k":
		s.ply = max(0, s.ply-10)
	case "down", "j":
		s.ply = min(last, s.ply+10)
	case "home", "0":
		s.ply = 0
	case "end", "G":
		s.ply = last
	case "a":
		return s.analyze()
	case "o":
		return openURL(strings.TrimRight(s.ctx.Session.Config.LichessURL, "/") + "/" + s.game.ID)
	case "esc", "backspace", "q":
		s.game, s.position = nil, nil
	}
	return nil
}

func (s *Study) count() int {
	if s.section == studyGames {
		return len(s.games)
	}
	return len(s.studies)
}

func (s *Study) open() tea.Cmd {
	index := s.selected[s.section]
	if s.section == studyStudies {
		if index >= len(s.studies) {
			return nil
		}
		return openURL(strings.TrimRight(s.ctx.Session.Config.LichessURL, "/") + "/study/" + s.studies[index].ID)
	}
	if index >= len(s.games) {
		return nil
	}
	game := s.games[index]
	position, err := chesscore.NewPosition("")
	if err == nil {
		err = position.ApplyNotation(game.Moves)
	}
	if err != nil || game.Variant != "standard" && game.Variant != "" {
		s.note = badNote("Replay supports standard chess only")
		return nil
	}
	s.game, s.position, s.ply, s.san = &game, position, 0, position.SANMoves()
	s.analysis, s.note = map[int]engine.Analysis{}, note{}
	return nil
}

func (s *Study) analyze() tea.Cmd {
	if s.thinking {
		return nil
	}
	position, err := s.position.AtPly(s.ply)
	if err != nil {
		s.note = errNote(err)
		return nil
	}
	analyzer, err := engine.NewStockfish(s.ctx.Session.Config.StockfishPath)
	if err != nil {
		s.note = errNote(err)
		return nil
	}
	s.thinking, s.note = true, busyNote("Stockfish is thinking")
	fen, depth, ply, id := position.FEN(), s.ctx.Session.Config.AnalysisDepth, s.ply, s.game.ID
	white := position.Turn() == chesscore.White
	return func() tea.Msg {
		analysis, err := analyzer.Analyze(context.Background(), fen, depth)
		// UCI scores are from the side to move; show them from White's view.
		if !white {
			analysis.ScoreCP, analysis.Mate = -analysis.ScoreCP, -analysis.Mate
		}
		return studyAnalysisMsg{gameID: id, ply: ply, analysis: analysis, err: err}
	}
}

func (s *Study) View(width, height int) string {
	t := s.ctx.Theme
	if s.prompt != nil {
		lines := []string{t.Title().Render("Open a Lichess library"), "", s.ctx.field("username", *s.prompt), "",
			t.Faint().Render("any public player · try DrNykterstein or penguingim1")}
		if s.note.text != "" {
			lines = append(lines, "", s.ctx.note(s.note))
		}
		return strings.Join(lines, "\n")
	}
	if s.game != nil {
		return s.replayView(width, height)
	}
	if s.username == "" {
		if s.note.text != "" {
			return s.ctx.note(s.note)
		}
		return t.Faint().Render("press u to open a player’s games")
	}
	return s.libraryView(min(width, 84), height)
}

func (s *Study) libraryView(width, height int) string {
	t := s.ctx.Theme
	head := t.Title().Render(s.username) + "   " + s.ctx.tabs([]string{fmt.Sprintf("Games %d", len(s.games)), fmt.Sprintf("Studies %d", len(s.studies))}, s.section)
	rows := []string{}
	if s.section == studyGames {
		for i, game := range s.games {
			rows = append(rows, s.gameRow(game, width-2, i == s.selected[studyGames]))
		}
	} else {
		for i, study := range s.studies {
			when := ""
			if study.UpdatedAt > 0 {
				when = time.UnixMilli(study.UpdatedAt).Format("Jan 2006")
			}
			rows = append(rows, padRight(s.ctx.item(study.Name, i == s.selected[studyStudies]), width-12)+t.Faint().Render(when))
		}
	}
	body := s.ctx.menu(rows, s.selected[s.section], height-4, width, true)
	if len(rows) == 0 {
		body = t.Faint().Render("  nothing here")
	}
	out := head + "\n\n" + body
	if s.note.text != "" {
		out += "\n\n" + s.ctx.note(s.note)
	}
	return lipgloss.NewStyle().Width(width).Render(out)
}

func (s *Study) gameRow(game lichess.ExportedGame, width int, selected bool) string {
	t := s.ctx.Theme
	white, black := game.Players.White, game.Players.Black
	name := func(player lichess.GamePlayer) string {
		return firstNonEmpty(player.User.Name, "Anonymous")
	}
	result := "½–½"
	switch game.Winner {
	case "white":
		result = "1–0"
	case "black":
		result = "0–1"
	}
	if game.Status == "started" {
		result = "live"
	}
	meta := game.Speed
	if game.CreatedAt > 0 {
		meta += " · " + time.UnixMilli(game.CreatedAt).Format("Jan 2")
	}
	players := s.ctx.item(name(white), selected) + t.Faint().Render(" vs ") + s.ctx.item(name(black), selected)
	return padRight(padRight(players, max(20, width-26))+t.Muted().Render(result), max(26, width-17)) + t.Faint().Render(meta)
}

func (s *Study) replayView(width, height int) string {
	position, err := s.position.AtPly(s.ply)
	if err != nil {
		return s.ctx.note(errNote(err))
	}
	last := ""
	if s.ply > 0 {
		last = s.position.Moves()[s.ply-1]
	}
	board := widgets.Board{Theme: *s.ctx.Theme, Glyphs: s.ctx.Session.Config.Glyphs, LastMove: last}
	render := func(b widgets.Board) string { return b.Render(position) }
	return boardAndSide(board, render, func(w int) string { return s.replaySide(w, height) }, s.replayCompact, width, height)
}

func (s *Study) replaySide(width, height int) string {
	t := s.ctx.Theme
	white, black := s.game.Players.White, s.game.Players.Black
	lines := []string{
		s.ctx.playerLine(false, firstNonEmpty(black.User.Name, "Black"), black.Rating, "", false, width),
		"",
		s.ctx.moveList(s.san, s.ply-1, max(1, height-11), 1, false),
		"",
		s.ctx.playerLine(true, firstNonEmpty(white.User.Name, "White"), white.Rating, "", false, width),
		"",
		t.Faint().Render(fmt.Sprintf("move %d of %d · %s", (s.ply+1)/2, (len(s.san)+1)/2, s.game.Status)),
	}
	lines = append(lines, s.evalLines(width)...)
	if s.note.text != "" {
		lines = append(lines, s.ctx.note(s.note))
	}
	return strings.Join(lines, "\n")
}

func (s *Study) replayCompact(width int) string {
	lines := []string{s.ctx.Theme.Faint().Render(fmt.Sprintf("move %d of %d", (s.ply+1)/2, (len(s.san)+1)/2))}
	lines = append(lines, s.evalLines(width)...)
	return strings.Join(lines, "\n")
}

// evalLines draws a horizontal evaluation bar and the engine's line.
func (s *Study) evalLines(width int) []string {
	analysis, ok := s.analysis[s.ply]
	if !ok {
		return nil
	}
	t := s.ctx.Theme
	barWidth := max(10, min(30, width-8))
	share := 0.5
	switch {
	case analysis.Mate > 0:
		share = 1
	case analysis.Mate < 0:
		share = 0
	default:
		share = 0.5 + float64(max(-600, min(600, analysis.ScoreCP)))/1200
	}
	filled := int(share * float64(barWidth))
	bar := lipgloss.NewStyle().Foreground(t.WhitePiece).Render(strings.Repeat("█", filled)) +
		lipgloss.NewStyle().Foreground(t.FaintColor).Render(strings.Repeat("█", barWidth-filled))
	pv := []string{}
	if position, err := s.position.AtPly(s.ply); err == nil {
		for _, move := range analysis.PV[:min(6, len(analysis.PV))] {
			pv = append(pv, position.SAN(move))
			if position.ApplyUCI(move) != nil {
				break
			}
		}
	}
	return []string{"", bar + "  " + t.Accent().Bold(true).Render(formatScore(analysis)),
		t.Muted().Render(strings.Join(pv, " ")) + t.Faint().Render(fmt.Sprintf("  d%d", analysis.Depth))}
}

func (s *Study) Hints() []widgets.Hint {
	switch {
	case s.prompt != nil:
		return []widgets.Hint{{Key: "enter", Label: "open"}, {Key: "esc", Label: "back"}}
	case s.game != nil:
		return []widgets.Hint{{Key: "←→", Label: "step"}, {Key: "↑↓", Label: "±10"}, {Key: "a", Label: "analyse"}, {Key: "o", Label: "open on lichess"}, {Key: "backspace", Label: "back"}}
	}
	return []widgets.Hint{{Key: "↑↓", Label: "select"}, {Key: "enter", Label: "replay"}, {Key: "[ ]", Label: "games/studies"}, {Key: "u", Label: "player"}, {Key: "r", Label: "refresh"}}
}

func (s *Study) Status() string {
	if s.game != nil {
		return "lichess.org/" + s.game.ID
	}
	return ""
}

func (s *Study) CapturesKey(key string) bool {
	return s.prompt != nil || (s.game != nil && (key == "q" || key == "0" || key == "G"))
}
