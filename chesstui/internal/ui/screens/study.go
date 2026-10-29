package screens

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kannachi323/chesstui/internal/domain"
	"github.com/kannachi323/chesstui/internal/lichess"
	"github.com/kannachi323/chesstui/internal/pgn"
	"github.com/kannachi323/chesstui/internal/ui/widgets"
)

type studyLoadedMsg struct {
	username string
	games    []lichess.ExportedGame
	studies  []lichess.Study
	err      error
}

type studyImportedMsg struct {
	gameID string
	err    error
}

type studyBookMsg struct {
	path string
	err  error
}

const studyGames, studyStudies = 0, 1

// Study browses any public Lichess player's games and studies. A game is
// copied into the local library and opened in Analyze; a study opens as a
// book in Read.
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
	client := s.ctx.Session.Client()
	return func() tea.Msg {
		ctx := context.Background()
		message := studyLoadedMsg{username: username}
		message.err = client.RecentGames(ctx, username, 30, func(game lichess.ExportedGame) error {
			message.games = append(message.games, game)
			return nil
		})
		if message.err == nil {
			message.err = client.Studies(ctx, username, func(study lichess.Study) error {
				message.studies = append(message.studies, study)
				return nil
			})
		}
		return message
	}
}

func (s *Study) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.PasteMsg:
		if s.prompt != nil {
			s.prompt.paste(msg.Content)
		}
		return nil
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
	case studyImportedMsg:
		s.loading = false
		if msg.err != nil {
			s.note = errNote(msg.err)
			return nil
		}
		s.note = okNote("Copied to your library")
		id := msg.gameID
		return func() tea.Msg { return OpenAnalyzeMsg{GameID: id} }
	case studyBookMsg:
		s.loading = false
		if msg.err != nil {
			s.note = errNote(msg.err)
			return nil
		}
		s.note = okNote("Saved as a book")
		path := msg.path
		return func() tea.Msg { return OpenBookMsg{Path: path} }
	}
	key, ok := keyString(msg)
	if !ok || s.loading {
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
			s.prompt = nil
			return s.load(name)
		default:
			s.prompt.handle(key)
		}
		return nil
	}
	switch key {
	case "]", "[":
		s.section = 1 - s.section
	case "up", "k":
		s.selected[s.section] = max(0, s.selected[s.section]-1)
	case "down", "j":
		s.selected[s.section] = max(0, min(s.count()-1, s.selected[s.section]+1))
	case "u", "/":
		s.prompt = &textInput{allowed: usernameChar}
	case "r":
		if s.username != "" {
			return s.load(s.username)
		}
	case "o":
		return s.openInBrowser()
	case "enter":
		return s.importSelected()
	}
	return nil
}

func (s *Study) count() int {
	if s.section == studyGames {
		return len(s.games)
	}
	return len(s.studies)
}

func (s *Study) openInBrowser() tea.Cmd {
	base := strings.TrimRight(s.ctx.Session.Config.LichessURL, "/")
	index := s.selected[s.section]
	if s.section == studyStudies && index < len(s.studies) {
		return openURL(base + "/study/" + s.studies[index].ID)
	}
	if s.section == studyGames && index < len(s.games) {
		return openURL(base + "/" + s.games[index].ID)
	}
	return nil
}

// importSelected copies a game into the library (and opens it in Analyze),
// or downloads a study's PGN into the books folder (and opens it in Read).
func (s *Study) importSelected() tea.Cmd {
	index := s.selected[s.section]
	store, client := s.ctx.Session.Store, s.ctx.Session.Client()
	if s.section == studyStudies {
		if index >= len(s.studies) {
			return nil
		}
		study := s.studies[index]
		dir := filepath.Join(s.ctx.Session.Paths.DataDir, "books")
		s.loading, s.note = true, busyNote("Downloading "+study.Name)
		return func() tea.Msg {
			text, err := client.ExportStudyPGN(context.Background(), study.ID)
			if err != nil {
				return studyBookMsg{err: err}
			}
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return studyBookMsg{err: err}
			}
			path := filepath.Join(dir, slug(study.Name)+"-"+study.ID+".pgn")
			if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
				return studyBookMsg{err: err}
			}
			return studyBookMsg{path: path}
		}
	}
	if index >= len(s.games) {
		return nil
	}
	game := s.games[index]
	if game.Variant != "" && game.Variant != "standard" {
		s.note = badNote("Only standard chess can be analysed")
		return nil
	}
	s.loading = true
	return func() tea.Msg {
		text := game.PGN
		if strings.TrimSpace(text) == "" {
			text = game.Moves + " *"
		}
		parsed, err := pgn.Parse(text)
		if err != nil {
			return studyImportedMsg{err: err}
		}
		if parsed[0].Err != nil {
			return studyImportedMsg{err: parsed[0].Err}
		}
		record := domain.Game{
			ID: "lichess-" + game.ID, Source: "lichess", Variant: "standard", Moves: lastNode(parsed[0]).Line(),
			White:  domain.Player{ID: game.Players.White.User.ID, Name: firstNonEmpty(game.Players.White.User.Name, "Anonymous"), Rating: game.Players.White.Rating},
			Black:  domain.Player{ID: game.Players.Black.User.ID, Name: firstNonEmpty(game.Players.Black.User.Name, "Anonymous"), Rating: game.Players.Black.Rating},
			Result: parsed[0].Result, PGN: parsed[0].String(),
		}
		if game.CreatedAt > 0 {
			record.PlayedAt = time.UnixMilli(game.CreatedAt)
		}
		return studyImportedMsg{gameID: record.ID, err: store.UpsertGame(context.Background(), record)}
	}
}

func lastNode(game *pgn.Game) *pgn.Node {
	node := game.Root
	for len(node.Children) > 0 {
		node = node.Children[0]
	}
	return node
}

func slug(text string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case b.Len() > 0 && !strings.HasSuffix(b.String(), "-"):
			b.WriteByte('-')
		}
		if b.Len() >= 40 {
			break
		}
	}
	return strings.Trim(b.String(), "-")
}

func (s *Study) View(width, height int) string {
	t := s.ctx.Theme
	if s.prompt != nil {
		lines := []string{t.Title().Render("Open a player’s games"), "", s.ctx.field("Lichess username", *s.prompt), "",
			t.Faint().Render("any public player · try DrNykterstein or penguingim1")}
		if s.note.text != "" {
			lines = append(lines, "", s.ctx.note(s.note))
		}
		return strings.Join(lines, "\n")
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

func (s *Study) Hints() []widgets.Hint {
	if s.prompt != nil {
		return []widgets.Hint{{Key: "enter", Label: "open"}, {Key: "esc", Label: "back"}}
	}
	action := "analyse"
	if s.section == studyStudies {
		action = "read"
	}
	return []widgets.Hint{{Key: "↑↓", Label: "select"}, {Key: "enter", Label: action}, {Key: "[ ]", Label: "games/studies"}, {Key: "u", Label: "player"}, {Key: "o", Label: "open on lichess"}}
}

func (s *Study) Status() string { return "" }

func (s *Study) CapturesKey(key string) bool { return s.prompt != nil }
