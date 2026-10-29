package screens

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kannachi323/chesstui/internal/book"
	chesscore "github.com/kannachi323/chesstui/internal/chess"
	"github.com/kannachi323/chesstui/internal/pgn"
	"github.com/kannachi323/chesstui/internal/storage"
	"github.com/kannachi323/chesstui/internal/ui/widgets"
)

type booksLoadedMsg struct {
	books []storage.Book
	err   error
}

type bookOpenedMsg struct {
	entry storage.Book
	book  *book.Book
	err   error
}

type bookSavedMsg struct{ err error }

// ImportToAnalyzeMsg asks the app to copy a game into the library and
// open it in Analyze.
type ImportToAnalyzeMsg struct{ PGN string }

// stop is a position the board can show: a move in a game, the start of a
// game, or a diagram.
type stop struct {
	node    *pgn.Node
	game    *pgn.Game
	fen     string
	chapter int
}

// Read is the book reader. The text scrolls like a pager while a board
// beside it follows the move or diagram being read.
type Read struct {
	ctx     Context
	library []storage.Book
	cursor  int
	note    note
	prompt  *textInput

	entry    storage.Book
	book     *book.Book
	stops    []stop
	focus    int
	top      int
	showTOC  bool
	tocIndex int
	board    bool // shown or hidden by the reader with b
	boardSet bool // false until b is pressed: wide terminals show the board
	width    int
	quiet    bool

	lastHeight int
	lastLines  []readLine
}

type readLine struct {
	text    string
	stops   []int
	chapter int
}

func NewRead(ctx Context) *Read { return &Read{ctx: ctx, board: true} }

func (r *Read) Init() tea.Cmd { return nil }

func (r *Read) Activate() tea.Cmd {
	if r.book != nil {
		return nil
	}
	return r.loadLibrary()
}

func (r *Read) booksDir() string { return filepath.Join(r.ctx.Session.Paths.DataDir, "books") }

// loadLibrary lists saved books plus any book files dropped into the books
// folder, with the starter book always present.
func (r *Read) loadLibrary() tea.Cmd {
	store, dir := r.ctx.Session.Store, r.booksDir()
	return func() tea.Msg {
		books, err := store.Books(context.Background())
		if err != nil {
			return booksLoadedMsg{err: err}
		}
		known := map[string]bool{}
		for _, b := range books {
			known[b.ID] = true
		}
		if !known[book.StarterID] {
			books = append(books, storage.Book{ID: book.StarterID, Title: "First Moves"})
		}
		entries, _ := os.ReadDir(dir)
		for _, entry := range entries {
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			path := filepath.Join(dir, entry.Name())
			if entry.IsDir() || known[path] || (ext != ".pgn" && ext != ".md" && ext != ".markdown") {
				continue
			}
			books = append(books, storage.Book{ID: path, Path: path, Title: strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))})
		}
		return booksLoadedMsg{books: books}
	}
}

func (r *Read) open(entry storage.Book) tea.Cmd {
	return func() tea.Msg {
		var b *book.Book
		var err error
		if entry.ID == book.StarterID {
			b = book.Starter()
		} else {
			b, err = book.Load(entry.Path)
		}
		if err != nil {
			return bookOpenedMsg{err: err}
		}
		entry.Title = b.Title
		return bookOpenedMsg{entry: entry, book: b}
	}
}

func (r *Read) openPath(path string) tea.Cmd {
	return r.open(storage.Book{ID: path, Path: path})
}

func (r *Read) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case OpenBookMsg:
		return r.openPath(msg.Path)
	case tea.PasteMsg:
		if r.prompt != nil {
			r.prompt.paste(msg.Content)
		}
		return nil
	case booksLoadedMsg:
		if msg.err != nil {
			r.note = errNote(msg.err)
			return nil
		}
		r.library = msg.books
		r.cursor = min(r.cursor, max(0, len(r.library)-1))
		return nil
	case bookOpenedMsg:
		if msg.err != nil {
			r.note = errNote(msg.err)
			return nil
		}
		r.entry, r.book = msg.entry, msg.book
		r.stops = collectStops(msg.book)
		r.focus = min(max(0, msg.entry.Position), max(0, len(r.stops)-1))
		r.top, r.showTOC, r.note = -1, false, note{}
		return r.saveEntry()
	case bookSavedMsg:
		if msg.err != nil {
			r.note = errNote(msg.err)
		}
		return nil
	}

	key, ok := keyString(msg)
	if !ok {
		return nil
	}
	if r.prompt != nil {
		switch key {
		case "esc":
			r.prompt = nil
		case "enter":
			path := cleanPath(r.prompt.value)
			r.prompt = nil
			if path != "" {
				return r.openPath(path)
			}
		default:
			r.prompt.handle(key)
		}
		return nil
	}
	if r.book == nil {
		return r.updateLibrary(key)
	}
	if r.showTOC {
		return r.updateTOC(key)
	}
	return r.updateReading(key)
}

func (r *Read) updateLibrary(key string) tea.Cmd {
	switch key {
	case "up", "k":
		r.cursor = max(0, r.cursor-1)
	case "down", "j":
		r.cursor = max(0, min(len(r.library)-1, r.cursor+1))
	case "enter", "right", "l":
		if r.cursor < len(r.library) {
			return r.open(r.library[r.cursor])
		}
	case "o":
		r.prompt = &textInput{}
	}
	return nil
}

func (r *Read) updateReading(key string) tea.Cmd {
	page := max(1, r.lastHeight-2)
	before := r.focus
	switch key {
	case "right", "n":
		r.focus = min(len(r.stops)-1, r.focus+1)
		r.top = -1
	case "left", "p":
		r.focus = max(0, r.focus-1)
		r.top = -1
	case "down", "j":
		r.scroll(1)
	case "up", "k":
		r.scroll(-1)
	case "space", "pgdown", "f":
		r.scroll(page)
	case "pgup", "shift+space":
		r.scroll(-page)
	case "]", "[":
		r.jumpChapter(key == "]")
	case "t":
		r.showTOC, r.tocIndex = true, r.currentChapter()
	case "m":
		r.toggleBookmark()
		return r.saveEntry()
	case "b":
		r.board, r.boardSet = !r.showBoard(), true
	case "z":
		r.quiet = !r.quiet
	case "a":
		if s := r.currentStop(); s != nil && s.game != nil {
			text := s.game.String()
			return func() tea.Msg { return ImportToAnalyzeMsg{PGN: text} }
		}
		r.note = plainNote("Move to a game to analyse it")
	case "esc", "backspace":
		r.book, r.quiet = nil, false
		return tea.Batch(r.saveEntry(), r.loadLibrary())
	}
	if r.focus != before {
		return r.saveEntry()
	}
	return nil
}

func (r *Read) updateTOC(key string) tea.Cmd {
	entries := r.tocEntries()
	switch key {
	case "up", "k":
		r.tocIndex = max(0, r.tocIndex-1)
	case "down", "j":
		r.tocIndex = max(0, min(len(entries)-1, r.tocIndex+1))
	case "enter":
		if r.tocIndex < len(entries) {
			entry := entries[r.tocIndex]
			r.showTOC = false
			if entry.stop >= 0 {
				r.focus, r.top = entry.stop, -1
				return r.saveEntry()
			}
			r.goToChapter(entry.chapter)
		}
	case "esc", "t":
		r.showTOC = false
	}
	return nil
}

func (r *Read) saveEntry() tea.Cmd {
	if r.book == nil && r.entry.ID == "" {
		return nil
	}
	entry, store := r.entry, r.ctx.Session.Store
	entry.Position = r.focus
	if entry.Title == "" && r.book != nil {
		entry.Title = r.book.Title
	}
	r.entry = entry
	return func() tea.Msg { return bookSavedMsg{err: store.SaveBook(context.Background(), entry)} }
}

func (r *Read) currentStop() *stop {
	if r.focus < 0 || r.focus >= len(r.stops) {
		return nil
	}
	return &r.stops[r.focus]
}

func (r *Read) currentChapter() int {
	if s := r.currentStop(); s != nil {
		return s.chapter
	}
	return 0
}

func (r *Read) toggleBookmark() {
	marks := r.entry.Bookmarks
	for i, mark := range marks {
		if mark == r.focus {
			r.entry.Bookmarks = append(marks[:i:i], marks[i+1:]...)
			r.note = plainNote("Bookmark removed")
			return
		}
	}
	r.entry.Bookmarks = append(marks, r.focus)
	sort.Ints(r.entry.Bookmarks)
	r.note = okNote("Bookmarked · t to see bookmarks")
}

// scroll moves the page and lets the board follow the first position on it.
func (r *Read) scroll(delta int) {
	lines := r.lastLines
	if len(lines) == 0 {
		return
	}
	height := max(1, r.lastHeight)
	r.top = max(0, min(max(0, len(lines)-height), r.top+delta))
	focusLine := -1
	for i, line := range lines {
		for _, s := range line.stops {
			if s == r.focus {
				focusLine = i
			}
		}
	}
	if focusLine >= r.top && focusLine < r.top+height {
		return
	}
	for i := r.top; i < min(len(lines), r.top+height); i++ {
		if len(lines[i].stops) > 0 {
			r.focus = lines[i].stops[0]
			return
		}
	}
}

func (r *Read) jumpChapter(forward bool) {
	chapter := r.currentChapter()
	if forward {
		chapter++
	} else {
		chapter--
	}
	if chapter >= 0 && chapter < len(r.book.Chapters) {
		r.goToChapter(chapter)
	}
}

// goToChapter scrolls to a chapter and focuses its first position.
func (r *Read) goToChapter(chapter int) {
	for i, s := range r.stops {
		if s.chapter == chapter {
			r.focus = i
			break
		}
	}
	for i, line := range r.lastLines {
		if line.chapter == chapter {
			r.top = i
			return
		}
	}
	r.top = -1
}

type tocEntry struct {
	label   string
	chapter int
	stop    int
	mark    bool
}

func (r *Read) tocEntries() []tocEntry {
	var entries []tocEntry
	for i, chapter := range r.book.Chapters {
		entries = append(entries, tocEntry{label: chapter.Title, chapter: i, stop: -1})
	}
	for _, mark := range r.entry.Bookmarks {
		if mark >= len(r.stops) {
			continue
		}
		s := r.stops[mark]
		label := r.book.Chapters[s.chapter].Title
		if s.node != nil && s.node.Parent != nil {
			full, white := s.game.FullMove()
			dots := "."
			if !s.node.WhiteMoved() {
				dots = "…"
			}
			label += fmt.Sprintf(" · %d%s %s", s.node.MoveNumber(full, white), dots, s.node.SAN)
		}
		entries = append(entries, tocEntry{label: label, chapter: s.chapter, stop: mark, mark: true})
	}
	return entries
}

// collectStops lists every position in reading order.
func collectStops(b *book.Book) []stop {
	var stops []stop
	for c, chapter := range b.Chapters {
		for _, block := range chapter.Blocks {
			switch block.Kind {
			case book.Diagram:
				stops = append(stops, stop{fen: block.FEN, chapter: c})
			case book.Game:
				stops = append(stops, stop{node: block.Game.Root, game: block.Game, chapter: c})
				var walk func(node *pgn.Node)
				walk = func(node *pgn.Node) {
					for len(node.Children) > 0 {
						main := node.Children[0]
						stops = append(stops, stop{node: main, game: block.Game, chapter: c})
						for _, alt := range node.Children[1:] {
							stops = append(stops, stop{node: alt, game: block.Game, chapter: c})
							walk(alt)
						}
						node = main
					}
				}
				walk(block.Game.Root)
			}
		}
	}
	return stops
}

// --- layout ------------------------------------------------------------------

// layout renders the whole book for a text column of width cells.
func (r *Read) layout(width int) []readLine {
	t := r.ctx.Theme
	var lines []readLine
	stopIndex := 0
	add := func(text string, chapter int, stops ...int) {
		lines = append(lines, readLine{text: text, chapter: chapter, stops: stops})
	}
	wrap := func(text string, style lipgloss.Style, chapter int, prefix, hang string) {
		for i, line := range strings.Split(lipgloss.NewStyle().Width(max(10, width-lipgloss.Width(prefix))).Render(markup(text, style, t.Title())), "\n") {
			lead := hang
			if i == 0 {
				lead = prefix
			}
			add(lead+line, chapter)
		}
	}
	for c, chapter := range r.book.Chapters {
		if c > 0 {
			add("", c)
		}
		add(t.Faint().Render(fmt.Sprintf("Chapter %d", c+1)), c)
		add(t.Title().Render(chapter.Title), c)
		add("", c)
		for _, block := range chapter.Blocks {
			switch block.Kind {
			case book.Heading:
				add(t.Title().Render(block.Text), c)
			case book.Paragraph:
				wrap(block.Text, t.Body(), c, "", "")
			case book.Quote:
				wrap(block.Text, t.Muted(), c, t.Faint().Render("│ "), t.Faint().Render("│ "))
			case book.Item:
				wrap(block.Text, t.Body(), c, t.Accent().Render("· "), "  ")
			case book.Diagram:
				marker := t.Accent().Render("◇ ")
				caption := firstNonEmpty(block.Text, "Diagram")
				style := t.Muted()
				if stopIndex == r.focus {
					marker, style = t.Accent().Bold(true).Render("◆ "), t.Accent()
				}
				first := true
				for _, line := range strings.Split(lipgloss.NewStyle().Width(max(10, width-2)).Render(caption), "\n") {
					lead := "  "
					if first {
						lead = marker
					}
					if first {
						add(lead+style.Render(line), c, stopIndex)
					} else {
						add(lead+style.Render(line), c)
					}
					first = false
				}
				stopIndex++
			case book.Game:
				stopIndex = r.layoutGame(block.Game, c, width, stopIndex, &lines)
			}
			add("", c)
		}
	}
	return lines
}

// layoutGame flows a game's moves, comments and side lines like book text.
func (r *Read) layoutGame(game *pgn.Game, chapter, width, stopIndex int, lines *[]readLine) int {
	t := r.ctx.Theme
	type word struct {
		text  string
		style lipgloss.Style
		stop  int
		glue  bool // no space before
	}
	var words []word
	headerStyle := t.Faint()
	if stopIndex == r.focus {
		headerStyle = t.Accent()
	}
	header := game.Title()
	if date := game.Tags["Date"]; len(date) >= 4 && date[:4] != "????" {
		header += " · " + date[:4]
	}
	*lines = append(*lines, readLine{text: headerStyle.Render("♟ " + header), chapter: chapter, stops: []int{stopIndex}})
	stopIndex++

	full, startWhite := game.FullMove()
	comment := func(text string, side bool) {
		style := t.Muted()
		if side {
			style = t.Faint()
		}
		for _, w := range strings.Fields(text) {
			words = append(words, word{text: w, style: style, stop: -1})
		}
	}
	var walk func(node *pgn.Node, side, forceNumber bool)
	emit := func(node *pgn.Node, side, forceNumber bool) {
		if node.Pre != "" {
			comment(node.Pre, side)
			forceNumber = true
		}
		number := node.MoveNumber(full, startWhite)
		style := t.Title()
		if side {
			style = t.Muted()
		}
		if stopIndex == r.focus {
			style = lipgloss.NewStyle().Bold(true).Foreground(t.OnFill).Background(t.AccentColor)
		}
		text := node.SAN
		for _, nag := range node.NAGs {
			text += pgn.Glyph(nag)
		}
		switch {
		case node.WhiteMoved():
			words = append(words, word{text: fmt.Sprintf("%d.", number), style: t.Faint(), stop: -1})
		case forceNumber:
			words = append(words, word{text: fmt.Sprintf("%d…", number), style: t.Faint(), stop: -1})
		}
		words = append(words, word{text: text, style: style, stop: stopIndex})
		stopIndex++
		if node.Comment != "" {
			comment(node.Comment, side)
		}
	}
	walk = func(node *pgn.Node, side, forceNumber bool) {
		for len(node.Children) > 0 {
			main := node.Children[0]
			emit(main, side, forceNumber)
			forceNumber = main.Comment != ""
			for _, alt := range node.Children[1:] {
				words = append(words, word{text: "(", style: t.Faint(), stop: -1})
				emit(alt, true, true)
				walk(alt, true, alt.Comment != "")
				words = append(words, word{text: ")", style: t.Faint(), stop: -1, glue: true})
				forceNumber = true
			}
			node = main
		}
	}
	walk(game.Root, false, !startWhite)
	if game.Result != "" && game.Result != "*" {
		words = append(words, word{text: game.Result, style: t.Faint(), stop: -1})
	}

	var line strings.Builder
	lineWidth := 0
	var lineStops []int
	flush := func() {
		*lines = append(*lines, readLine{text: line.String(), chapter: chapter, stops: lineStops})
		line.Reset()
		lineWidth, lineStops = 0, nil
	}
	afterOpen := false
	for _, w := range words {
		size := lipgloss.Width(w.text)
		if lineWidth > 0 && lineWidth+1+size > width {
			flush()
			afterOpen = false
		}
		if lineWidth > 0 && !w.glue && !afterOpen {
			line.WriteByte(' ')
			lineWidth++
		}
		line.WriteString(w.style.Render(w.text))
		lineWidth += size
		afterOpen = w.text == "("
		if w.stop >= 0 {
			lineStops = append(lineStops, w.stop)
		}
	}
	if lineWidth > 0 {
		flush()
	}
	return stopIndex
}

// markup renders **bold** spans in a paragraph.
func markup(text string, body, bold lipgloss.Style) string {
	parts := strings.Split(text, "**")
	var out strings.Builder
	for i, part := range parts {
		if i%2 == 1 {
			out.WriteString(bold.Render(part))
		} else {
			out.WriteString(body.Render(part))
		}
	}
	return out.String()
}

// --- views -------------------------------------------------------------------

func (r *Read) View(width, height int) string {
	if r.prompt != nil {
		t := r.ctx.Theme
		return t.Title().Render("Open a book") + "\n\n" + r.ctx.field("Path", *r.prompt) + "\n\n" +
			t.Faint().Render(".pgn or .md · or drop files into "+shortPath(r.booksDir(), 50))
	}
	if r.book == nil {
		return r.libraryView(width, height)
	}
	r.width = width
	if r.showTOC {
		return r.tocView(width, height)
	}

	textWidth := min(68, width)
	boardWidth, boardHeight := 0, 0
	sideBySide := r.showBoard() && width >= 80
	if sideBySide {
		textWidth = min(68, width-30)
		boardWidth, boardHeight = widgets.BoardSize(width-textWidth-4, height-2, height >= 14)
		textWidth = min(68, width-boardWidth-4)
	}
	textHeight := height
	var stacked string
	if r.showBoard() && !sideBySide {
		stacked = r.boardView(width, min(height/2, 11), false)
		textHeight = max(3, height-lipgloss.Height(stacked)-1)
	}

	lines := r.layout(textWidth)
	r.lastLines, r.lastHeight = lines, textHeight
	focusLine := 0
	for i, line := range lines {
		for _, s := range line.stops {
			if s == r.focus {
				focusLine = i
			}
		}
	}
	if r.top < 0 || focusLine < r.top || focusLine >= r.top+textHeight {
		r.top = max(0, min(max(0, len(lines)-textHeight), focusLine-textHeight/3))
	}
	end := min(len(lines), r.top+textHeight)
	visible := make([]string, 0, textHeight)
	for _, line := range lines[r.top:end] {
		visible = append(visible, line.text)
	}
	text := lipgloss.NewStyle().Width(textWidth).Height(textHeight).Render(strings.Join(visible, "\n"))

	switch {
	case sideBySide:
		board := r.boardView(boardWidth, boardHeight+2, true)
		return lipgloss.JoinHorizontal(lipgloss.Top, text, "    ", board)
	case stacked != "":
		return stacked + "\n" + text
	}
	return text
}

// showBoard is the reader's choice, or by default whether the terminal is
// wide enough to keep the board beside the text.
func (r *Read) showBoard() bool {
	if r.boardSet {
		return r.board
	}
	return r.width == 0 || r.width >= 80
}

func (r *Read) boardView(width, height int, withCaption bool) string {
	t := r.ctx.Theme
	s := r.currentStop()
	if s == nil {
		return t.Faint().Render("no diagrams in this book yet")
	}
	var position *chesscore.Position
	lastMove := ""
	if s.node != nil {
		position, lastMove = s.node.Position(), s.node.UCI
	} else {
		var err error
		if position, err = chesscore.NewPosition(s.fen); err != nil {
			return t.Bad().Render("bad diagram")
		}
	}
	flipped := false
	if s.game == nil {
		flipped = position.Turn() == chesscore.Black
	}
	boardHeight := height
	if withCaption {
		boardHeight = height - 2
	}
	rendered := widgets.Board{Theme: *t, Width: width, Height: boardHeight, Flipped: flipped, Glyphs: r.ctx.Session.Config.Glyphs, LastMove: lastMove, ShowCoords: boardHeight >= 12}.Render(position)
	if !withCaption {
		return rendered
	}
	caption := "White to move"
	if position.Turn() == chesscore.Black {
		caption = "Black to move"
	}
	if outcome := position.Outcome(); outcome != "" {
		caption = outcome
	}
	progress := fmt.Sprintf("%d / %d", r.focus+1, len(r.stops))
	bar := padRight(t.Muted().Render(caption), max(1, lipgloss.Width(rendered)-lipgloss.Width(progress))) + t.Faint().Render(progress)
	return rendered + "\n\n" + bar
}

func (r *Read) tocView(width, height int) string {
	t := r.ctx.Theme
	entries := r.tocEntries()
	rowWidth := min(60, width)
	rows := make([]string, len(entries))
	current := r.currentChapter()
	for i, entry := range entries {
		label := r.ctx.item(entry.label, i == r.tocIndex)
		prefix := t.Faint().Render(fmt.Sprintf("%2d  ", entry.chapter+1))
		if entry.mark {
			prefix = t.Accent().Render(" ◆  ")
		} else if entry.chapter == current {
			prefix = t.Accent().Render(fmt.Sprintf("%2d  ", entry.chapter+1))
		}
		rows[i] = prefix + ansi.Truncate(label, rowWidth-6, "…")
	}
	title := t.Title().Render(r.book.Title)
	if r.book.Author != "" {
		title += t.Faint().Render(" · " + r.book.Author)
	}
	return lipgloss.NewStyle().Width(rowWidth).Render(title + "\n\n" + r.ctx.menu(rows, r.tocIndex, max(3, height-3), rowWidth, true))
}

func (r *Read) libraryView(width, height int) string {
	t := r.ctx.Theme
	rowWidth := min(72, width)
	rows := make([]string, len(r.library))
	for i, entry := range r.library {
		meta := "built in"
		switch {
		case entry.ID == book.StarterID && !entry.OpenedAt.IsZero():
			meta = "read " + entry.OpenedAt.Local().Format("Jan 2")
		case !entry.OpenedAt.IsZero():
			meta = "read " + entry.OpenedAt.Local().Format("Jan 2")
		case entry.Path != "":
			meta = "new"
		}
		if len(entry.Bookmarks) > 0 {
			meta = fmt.Sprintf("%d bookmark%s · %s", len(entry.Bookmarks), plural(len(entry.Bookmarks)), meta)
		}
		title := ansi.Truncate(firstNonEmpty(entry.Title, filepath.Base(entry.Path)), rowWidth-24, "…")
		rows[i] = padRight(r.ctx.item(title, i == r.cursor), rowWidth-2-lipgloss.Width(meta)) + t.Faint().Render(meta)
	}
	lines := []string{t.Title().Render("Read"), t.Muted().Render("Chess books for a terminal split."), "", r.ctx.menu(rows, r.cursor, max(3, height-8), rowWidth, true)}
	lines = append(lines, "", t.Faint().Render("Add .pgn or .md books with o, or drop them into "+shortPath(r.booksDir(), 40)))
	if r.note.text != "" {
		lines = append(lines, "", r.ctx.note(r.note))
	}
	return lipgloss.NewStyle().Width(rowWidth).Render(strings.Join(lines, "\n"))
}

func (r *Read) Hints() []widgets.Hint {
	switch {
	case r.prompt != nil:
		return []widgets.Hint{{Key: "enter", Label: "open"}, {Key: "esc", Label: "back"}}
	case r.book == nil:
		return []widgets.Hint{{Key: "↑↓", Label: "choose"}, {Key: "enter", Label: "read"}, {Key: "o", Label: "open a file"}}
	case r.showTOC:
		return []widgets.Hint{{Key: "↑↓", Label: "choose"}, {Key: "enter", Label: "go"}, {Key: "esc", Label: "close"}}
	}
	return []widgets.Hint{
		{Key: "←→", Label: "moves"}, {Key: "j k", Label: "scroll"}, {Key: "space", Label: "page"}, {Key: "t", Label: "contents"},
		{Key: "[ ]", Label: "chapters"}, {Key: "m", Label: "bookmark"}, {Key: "b", Label: "board"}, {Key: "z", Label: "quiet"},
		{Key: "a", Label: "analyse game"}, {Key: "esc", Label: "library"},
	}
}

func (r *Read) Status() string {
	if r.book == nil {
		return ""
	}
	return r.book.Title
}

func (r *Read) CapturesKey(key string) bool {
	if r.prompt != nil {
		return true
	}
	return r.book != nil && (key == "esc" || r.showTOC)
}

// Fullscreen hides the app's header and footer in quiet mode.
func (r *Read) Fullscreen() bool { return r.book != nil && r.quiet }
