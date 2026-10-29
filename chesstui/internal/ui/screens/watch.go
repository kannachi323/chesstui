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
	"github.com/kannachi323/chesstui/internal/lichess"
	"github.com/kannachi323/chesstui/internal/ui/widgets"
)

type channelsLoadedMsg struct {
	channels []domain.TVChannel
	err      error
}

type streamersLoadedMsg struct {
	streamers []domain.Streamer
	err       error
}

type broadcastsLoadedMsg struct {
	broadcasts []domain.Broadcast
	err        error
}

type tvUpdateMsg struct {
	stream int
	event  lichess.TVEvent
	retry  time.Duration
	err    error
}

type tvTickMsg struct{ stream int }

type tvPlayer struct {
	name   string
	title  string
	rating int
}

const (
	watchTV = iota
	watchStreams
	watchEvents
)

type Watch struct {
	ctx       Context
	activated bool
	section   int
	selected  [3]int
	loaded    [3]bool
	loading   bool
	note      note

	channels   []domain.TVChannel
	streamers  []domain.Streamer
	broadcasts []domain.Broadcast

	channel  string
	streamID int
	updates  chan tvUpdateMsg
	cancel   context.CancelFunc
	timeline []*chesscore.Position
	moves    []string
	ply      int
	gameID   string
	flipped  bool
	white    tvPlayer
	black    tvPlayer
	clocks   [2]int // seconds, white then black
	clockAt  time.Time
}

func NewWatch(ctx Context) *Watch { return &Watch{ctx: ctx} }
func (w *Watch) Init() tea.Cmd    { return nil }

// Activate loads the channel list and starts the default channel the first
// time Watch opens. Streams and events load when their tab is opened.
func (w *Watch) Activate() tea.Cmd {
	if w.activated {
		return nil
	}
	w.activated = true
	return w.load(watchTV)
}

func (w *Watch) load(section int) tea.Cmd {
	w.loading = true
	client := w.ctx.Session.Client()
	switch section {
	case watchStreams:
		return func() tea.Msg {
			streamers, err := client.LiveStreamers(context.Background())
			return streamersLoadedMsg{streamers, err}
		}
	case watchEvents:
		return func() tea.Msg {
			broadcasts, err := client.TopBroadcasts(context.Background())
			return broadcastsLoadedMsg{broadcasts, err}
		}
	}
	return func() tea.Msg {
		channels, err := client.TVChannels(context.Background())
		return channelsLoadedMsg{channels, err}
	}
}

func (w *Watch) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case channelsLoadedMsg:
		w.loading = false
		if msg.err != nil {
			w.note = errNote(msg.err)
			return nil
		}
		w.channels, w.loaded[watchTV], w.note = msg.channels, true, note{}
		if w.channel == "" {
			for i, channel := range w.channels {
				if channel.Key == w.ctx.Session.Config.DefaultWatch {
					w.selected[watchTV] = i
				}
			}
			if len(w.channels) > 0 {
				return w.startTV(w.channels[w.selected[watchTV]].Key)
			}
		}
		return nil
	case streamersLoadedMsg:
		w.loading = false
		if msg.err != nil {
			w.note = errNote(msg.err)
			return nil
		}
		w.streamers, w.loaded[watchStreams], w.note = msg.streamers, true, note{}
		return nil
	case broadcastsLoadedMsg:
		w.loading = false
		if msg.err != nil {
			w.note = errNote(msg.err)
			return nil
		}
		w.broadcasts, w.loaded[watchEvents], w.note = msg.broadcasts, true, note{}
		return nil
	case tvUpdateMsg:
		if msg.stream != w.streamID {
			return nil
		}
		switch {
		case msg.retry > 0:
			w.note = busyNote(fmt.Sprintf("TV connection lost · retrying in %s", msg.retry.Round(time.Second)))
		case msg.err != nil:
			w.note = errNote(msg.err)
			return nil
		default:
			w.consume(msg.event)
		}
		return w.waitTV()
	case tvTickMsg:
		if msg.stream != w.streamID {
			return nil
		}
		return w.tick()
	case browserOpenedMsg:
		if msg.err != nil {
			w.note = errNote(msg.err)
		}
		return nil
	}

	key, ok := keyString(msg)
	if !ok {
		return nil
	}
	switch key {
	case "]", "tab":
		return w.switchSection(1)
	case "[", "shift+tab":
		return w.switchSection(-1)
	case "up", "k":
		w.selected[w.section] = max(0, w.selected[w.section]-1)
	case "down", "j":
		w.selected[w.section] = max(0, min(w.itemCount()-1, w.selected[w.section]+1))
	case "left", "h":
		w.ply = max(0, w.ply-1)
	case "right", "l":
		w.ply = min(len(w.timeline)-1, w.ply+1)
	case "end", "G":
		w.ply = len(w.timeline) - 1
	case "r":
		return w.load(w.section)
	case "enter":
		return w.activateItem()
	}
	return nil
}

func (w *Watch) switchSection(delta int) tea.Cmd {
	w.section = (w.section + delta + 3) % 3
	if !w.loaded[w.section] && !w.loading {
		return w.load(w.section)
	}
	return nil
}

func (w *Watch) itemCount() int {
	switch w.section {
	case watchTV:
		return len(w.channels)
	case watchStreams:
		return len(w.streamers)
	default:
		return len(w.broadcasts)
	}
}

func (w *Watch) activateItem() tea.Cmd {
	index := w.selected[w.section]
	switch w.section {
	case watchTV:
		if index < len(w.channels) {
			return w.startTV(w.channels[index].Key)
		}
	case watchStreams:
		if index < len(w.streamers) && w.streamers[index].URL != "" {
			return openURL(w.streamers[index].URL)
		}
	case watchEvents:
		if index < len(w.broadcasts) && w.broadcasts[index].URL != "" {
			return openURL(w.broadcasts[index].URL)
		}
	}
	return nil
}

func (w *Watch) startTV(channel string) tea.Cmd {
	if w.cancel != nil {
		w.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	w.streamID++
	streamID := w.streamID
	w.channel, w.timeline, w.moves, w.ply, w.gameID = channel, nil, nil, 0, ""
	w.updates = make(chan tvUpdateMsg, 16)
	updates, client := w.updates, w.ctx.Session.Client()
	send := func(msg tvUpdateMsg) {
		select {
		case updates <- msg:
		case <-ctx.Done():
		}
	}
	go func() {
		err := lichess.Reconnect(ctx, func(ctx context.Context) error {
			return client.StreamTV(ctx, channel, func(event lichess.TVEvent) error {
				send(tvUpdateMsg{stream: streamID, event: event})
				return ctx.Err()
			})
		}, func(wait time.Duration, _ error) { send(tvUpdateMsg{stream: streamID, retry: wait}) })
		send(tvUpdateMsg{stream: streamID, err: err})
	}()
	return tea.Batch(w.waitTV(), w.tick())
}

func (w *Watch) waitTV() tea.Cmd {
	updates := w.updates
	return func() tea.Msg { return <-updates }
}

func (w *Watch) tick() tea.Cmd {
	id := w.streamID
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return tvTickMsg{stream: id} })
}

// consume handles the two TV feed events: "featured" starts a new game with
// its players, and "fen" carries each move.
func (w *Watch) consume(event lichess.TVEvent) {
	data := event.Data
	if event.Type == "featured" {
		w.timeline, w.moves, w.ply, w.gameID = nil, nil, 0, data.ID
		w.flipped = data.Orientation == "black"
		for _, player := range data.Players {
			info := tvPlayer{name: player.User.Name, title: player.User.Title, rating: player.Rating}
			if player.Color == "black" {
				w.black = info
				w.clocks[1] = player.Seconds
			} else {
				w.white = info
				w.clocks[0] = player.Seconds
			}
		}
	} else {
		w.clocks = [2]int{data.WhiteClock, data.BlackClock}
	}
	w.clockAt = time.Now()
	if data.FEN == "" {
		return
	}
	// The TV feed sends only the piece placement and side to move.
	fen := data.FEN
	if len(strings.Fields(fen)) == 1 {
		turn := "w"
		if len(w.timeline) > 0 && w.timeline[len(w.timeline)-1].Turn() == chesscore.White {
			turn = "b"
		}
		fen += " " + turn + " - - 0 1"
	}
	position, err := chesscore.NewPosition(fen)
	if err != nil {
		return
	}
	following := w.ply >= len(w.timeline)-1
	w.timeline = append(w.timeline, position)
	w.moves = append(w.moves, data.LastMove)
	if following {
		w.ply = len(w.timeline) - 1
	}
	w.note = note{}
}

func (w *Watch) View(width, height int) string {
	listWidth := min(40, max(28, width/3))
	if width < 76 {
		return w.tvView(width, height-2) + "\n\n" + w.ctx.tabs([]string{"Channels", "Streams", "Events"}, w.section)
	}
	left := w.listView(listWidth, height)
	right := w.tvView(width-listWidth-4, height)
	return lipgloss.JoinHorizontal(lipgloss.Top, lipgloss.NewStyle().Width(listWidth).Render(left), "    ", right)
}

func (w *Watch) listView(width, height int) string {
	t := w.ctx.Theme
	rows := []string{}
	empty := "nothing live right now"
	switch w.section {
	case watchTV:
		for i, item := range w.channels {
			name := item.Player.Name
			if item.Player.Title != "" {
				name = item.Player.Title + " " + name
			}
			rows = append(rows, padRight(w.ctx.item(channelLabel(item.Key), i == w.selected[watchTV]), 18)+t.Muted().Render(name))
		}
	case watchStreams:
		for i, item := range w.streamers {
			rows = append(rows, padRight(w.ctx.item(item.Name, i == w.selected[watchStreams]), 18)+t.Faint().Render(item.Service))
		}
	case watchEvents:
		for i, item := range w.broadcasts {
			dot := t.Faint().Render("○ ")
			if item.Live {
				dot = t.Bad().Render("● ")
			}
			rows = append(rows, dot+w.ctx.item(item.Name, i == w.selected[watchEvents]))
		}
	}
	body := w.ctx.menu(rows, w.selected[w.section], height-4, width, true)
	switch {
	case w.loading && len(rows) == 0:
		body = t.Faint().Render("  loading…")
	case len(rows) == 0:
		body = t.Faint().Render("  " + empty)
	}
	out := w.ctx.tabs([]string{"Channels", "Streams", "Events"}, w.section) + "\n\n" + body
	if w.note.text != "" {
		out += "\n\n" + w.ctx.note(w.note)
	}
	return out
}

func (w *Watch) tvView(width, height int) string {
	t := w.ctx.Theme
	if len(w.timeline) == 0 {
		if w.note.text != "" && w.channel == "" {
			return w.ctx.note(w.note)
		}
		return t.Faint().Render("… tuning in to Lichess TV")
	}
	position := w.timeline[w.ply]
	top, bottom := w.white, w.black
	topWhite := false
	if !w.flipped {
		top, bottom, topWhite = w.black, w.white, false
	} else {
		topWhite = true
	}
	_, boardHeight := widgets.BoardSize(width, height-4, height >= 14)
	board := widgets.Board{
		Theme: *t, Width: width, Height: boardHeight, Flipped: w.flipped, Glyphs: w.ctx.Session.Config.Glyphs,
		LastMove: w.moves[w.ply], ShowCoords: height >= 14,
	}.Render(position)
	boardWidth := lipgloss.Width(board)
	live := w.ply == len(w.timeline)-1
	line := func(player tvPlayer, white bool) string {
		name := player.name
		if player.title != "" {
			name = player.title + " " + name
		}
		clock := w.clock(white, live)
		toMove := live && (position.Turn() == chesscore.White) == white
		return w.ctx.playerLine(white, firstNonEmpty(name, "Anonymous"), player.rating, clock, toMove, boardWidth)
	}
	status := t.Bad().Render("●") + t.Faint().Render(" live · "+channelLabel(w.channel))
	if !live {
		status = t.Faint().Render(fmt.Sprintf("move %d of %d · end to go live", w.ply+1, len(w.timeline)))
	}
	return strings.Join([]string{line(top, topWhite), board, line(bottom, !topWhite), status}, "\n")
}

func (w *Watch) clock(white, live bool) string {
	if w.clockAt.IsZero() {
		return ""
	}
	seconds := w.clocks[1]
	if white {
		seconds = w.clocks[0]
	}
	if live && len(w.timeline) > 0 {
		position := w.timeline[len(w.timeline)-1]
		if (position.Turn() == chesscore.White) == white && len(w.timeline) > 1 {
			seconds -= int(time.Since(w.clockAt).Seconds())
		}
	}
	return formatClock(max(0, seconds) * 1000)
}

func channelLabel(key string) string {
	labels := map[string]string{"best": "Top rated", "ultraBullet": "UltraBullet", "kingOfTheHill": "King of the Hill", "threeCheck": "Three-check", "racingKings": "Racing Kings", "computer": "Computer", "bot": "Bot", "chess960": "Chess960"}
	if label, ok := labels[key]; ok {
		return label
	}
	if key == "" {
		return ""
	}
	return strings.ToUpper(key[:1]) + key[1:]
}

func padRight(text string, width int) string {
	return text + strings.Repeat(" ", max(1, width-lipgloss.Width(text)))
}

func (w *Watch) Hints() []widgets.Hint {
	hints := []widgets.Hint{{Key: "[ ]", Label: "list"}, {Key: "↑↓", Label: "select"}}
	if w.section == watchTV {
		hints = append(hints, widgets.Hint{Key: "enter", Label: "watch"}, widgets.Hint{Key: "←→", Label: "rewind"}, widgets.Hint{Key: "end", Label: "live"})
	} else {
		hints = append(hints, widgets.Hint{Key: "enter", Label: "open in browser"})
	}
	return append(hints, widgets.Hint{Key: "r", Label: "refresh"})
}

func (w *Watch) Status() string {
	if w.gameID != "" {
		return "lichess.org/" + w.gameID
	}
	return ""
}
