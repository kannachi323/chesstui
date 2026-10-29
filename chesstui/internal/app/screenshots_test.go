package app

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image/color"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	chesscore "github.com/kannachi323/chesstui/internal/chess"
	"github.com/kannachi323/chesstui/internal/config"
	"github.com/kannachi323/chesstui/internal/session"
	"github.com/kannachi323/chesstui/internal/storage"
	"github.com/kannachi323/chesstui/internal/termshot"
)

// go test ./internal/app -run Screenshots -shots ../../../docs/screenshots
var shotsDir = flag.String("shots", "", "write SVG screenshots to this directory")

// tvDelay spaces out fake TV moves so a test can watch them arrive.
var tvDelay time.Duration

// afterUpdate, when set, sees the model after every message drive handles.
var afterUpdate func(Model)

var (
	fakeEngineOnce sync.Once
	fakeEnginePath string
	fakeEngineErr  error
)

// fakeEngine builds testdata/fakeengine once per test run and returns it.
func fakeEngine(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake engine build is POSIX-only")
	}
	fakeEngineOnce.Do(func() {
		dir, err := os.MkdirTemp("", "chesstui-fakeengine")
		if err != nil {
			fakeEngineErr = err
			return
		}
		fakeEnginePath = filepath.Join(dir, "fakefish")
		out, err := exec.Command("go", "build", "-o", fakeEnginePath, "./testdata/fakeengine").CombinedOutput()
		if err != nil {
			fakeEngineErr = fmt.Errorf("%v: %s", err, out)
		}
	})
	if fakeEngineErr != nil {
		t.Fatal(fakeEngineErr)
	}
	return fakeEnginePath
}

type staticTokens struct{ token string }

func (s staticTokens) Load() (string, error) { return s.token, nil }
func (staticTokens) Save(string) error       { return nil }
func (staticTokens) Delete() error           { return nil }

// fakeLichess serves recorded public API responses from testdata, plus a
// TV feed and a board stream replayed from one recorded game.
func fakeLichess(t *testing.T) *httptest.Server {
	t.Helper()
	read := func(name string) []byte {
		data, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	var game struct {
		ID      string `json:"id"`
		Moves   string `json:"moves"`
		Players struct {
			White, Black struct {
				User   struct{ Name, Title string }
				Rating int
			}
		}
	}
	firstGame := strings.SplitN(string(read("games.ndjson")), "\n", 2)[0]
	if err := json.Unmarshal([]byte(firstGame), &game); err != nil {
		t.Fatal(err)
	}
	replay, _ := chesscore.NewPosition("")
	if err := replay.ApplyNotation(game.Moves); err != nil {
		t.Fatal(err)
	}
	moves := replay.Moves()[:41]

	mux := http.NewServeMux()
	serve := func(path, file, contentType string) {
		mux.HandleFunc(path, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", contentType)
			_, _ = w.Write(read(file))
		})
	}
	serve("/api/puzzle/daily", "puzzle-daily.json", "application/json")
	serve("/api/puzzle/next", "puzzle-daily.json", "application/json")
	serve("/api/tv/channels", "tv-channels.json", "application/json")
	serve("/api/games/user/", "games.ndjson", "application/x-ndjson")
	serve("/api/study/by/", "studies.ndjson", "application/x-ndjson")
	mux.HandleFunc("/api/streamer/live", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "[]") })
	mux.HandleFunc("/api/broadcast/top", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, `{"active":[]}`) })
	mux.HandleFunc("/api/account", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		fmt.Fprint(w, `{"id":"kannachi323","username":"kannachi323"}`)
	})
	mux.HandleFunc("/api/challenge/ai", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"id":"aiGame01","color":"white"}`)
	})
	mux.HandleFunc("/api/board/game/stream/", func(w http.ResponseWriter, r *http.Request) {
		played := strings.Join(moves[:24], " ")
		fmt.Fprintf(w, `{"type":"gameFull","id":"aiGame01","initialFen":"startpos","white":{"id":"kannachi323","name":"kannachi323","rating":1840},"black":{"aiLevel":5},"state":{"type":"gameState","moves":%q,"wtime":241000,"btime":187000,"status":"started"}}`+"\n", played)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})
	mux.HandleFunc("/api/tv/", func(w http.ResponseWriter, r *http.Request) {
		position, _ := chesscore.NewPosition("")
		white, black := game.Players.White, game.Players.Black
		fmt.Fprintf(w, `{"t":"featured","d":{"id":%q,"orientation":"white","players":[{"color":"white","user":{"name":%q,"title":%q},"rating":%d,"seconds":180},{"color":"black","user":{"name":%q,"title":%q},"rating":%d,"seconds":180}],"fen":%q}}`+"\n",
			game.ID, white.User.Name, white.User.Title, white.Rating, black.User.Name, black.User.Title, black.Rating, position.FEN())
		for i, move := range moves {
			_ = position.ApplyUCI(move)
			fmt.Fprintf(w, `{"t":"fen","d":{"fen":%q,"lm":%q,"wc":%d,"bc":%d}}`+"\n", position.FEN(), move, 180-i*2, 176-i*3)
			if tvDelay > 0 {
				w.(http.Flusher).Flush()
				time.Sleep(tvDelay)
			}
		}
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})
	server := httptest.NewServer(mux)
	t.Cleanup(func() { server.CloseClientConnections(); server.Close() })
	return server
}

// run executes one command, expanding batches. Commands that block (ticks,
// idle streams) are abandoned after a short wait.
func run(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	result := make(chan tea.Msg, 1)
	go func() { result <- cmd() }()
	select {
	case msg := <-result:
		if batch, ok := msg.(tea.BatchMsg); ok {
			var out []tea.Msg
			for _, inner := range batch {
				out = append(out, run(inner)...)
			}
			return out
		}
		if msg == nil {
			return nil
		}
		return []tea.Msg{msg}
	case <-time.After(700 * time.Millisecond):
		return nil
	}
}

// drive feeds msg to the model and keeps processing the messages its
// commands produce until the queue settles.
func drive(m Model, msgs ...tea.Msg) Model {
	queue := append([]tea.Msg(nil), msgs...)
	for steps := 0; len(queue) > 0 && steps < 400; steps++ {
		msg := queue[0]
		queue = queue[1:]
		updated, cmd := m.Update(msg)
		m = updated.(Model)
		_ = m.render() // like the real loop: a frame is drawn after each update
		if afterUpdate != nil {
			afterUpdate(m)
		}
		queue = append(queue, run(cmd)...)
	}
	return m
}

func key(text string) tea.Msg {
	special := map[string]rune{"enter": tea.KeyEnter, "up": tea.KeyUp, "down": tea.KeyDown, "left": tea.KeyLeft, "right": tea.KeyRight, "space": tea.KeySpace, "end": tea.KeyEnd, "esc": tea.KeyEscape}
	if code, ok := special[text]; ok {
		return tea.KeyPressMsg(tea.Key{Code: code})
	}
	r := []rune(text)[0]
	return tea.KeyPressMsg(tea.Key{Code: r, Text: text})
}

type scene struct {
	name          string
	width, height int
	light         bool
	token         string
	keys          []string // "paste:<file>" pastes a testdata file
	want          []string
	engine        bool // use the fake UCI engine
	card          bool // also render a tighter 84x26 frame for the website
}

func TestScreenshots(t *testing.T) {
	if testing.Short() {
		t.Skip("drives the whole app against a fake Lichess")
	}
	server := fakeLichess(t)
	scenes := []scene{
		{name: "home", width: 100, height: 30, want: []string{"Play", "Analyze", "Read"}},
		{name: "play", card: true, width: 120, height: 36, engine: true,
			keys: []string{"2", "down", "down", "right", "up", "up", "enter", "e", "4", "enter", "N", "f", "3", "enter", "B", "c", "4", "enter"},
			want: []string{"FakeFish", "saved to your library"}},
		{name: "analyze", card: true, width: 120, height: 36, engine: true,
			keys: []string{"3", "paste:opera.pgn", "r", "end", "left", "left", "space"},
			want: []string{"Paul Morphy", "Qb8+", "%"}},
		{name: "analyze-library", width: 100, height: 30, engine: true,
			keys: []string{"3", "paste:opera.pgn", "r", "esc"},
			want: []string{"Paul Morphy", "reviewed"}},
		{name: "read", card: true, width: 130, height: 38, keys: append([]string{"4", "enter"}, "right", "right", "right", "right", "right", "right", "right", "right", "right", "right", "right", "right"),
			want: []string{"Paul Morphy", "13 / 65"}},
		{name: "read-split-60x30", width: 60, height: 30, keys: []string{"4", "enter", "right", "right", "right", "b"},
			want: []string{"Philidor"}},
		{name: "learn-light", card: true, width: 110, height: 32, light: true, keys: []string{"6", "j", "j", "j"}, want: []string{"En passant"}},
		{name: "puzzle", width: 110, height: 32, keys: []string{"5", "space", "enter"}, want: []string{"Puzzle", "to move"}},
		{name: "help", width: 100, height: 30, keys: []string{"3", "paste:opera.pgn", "?"}, want: []string{"Everywhere", "review"}},
		{name: "compact-80x24", width: 80, height: 24, engine: true, keys: []string{"3", "paste:opera.pgn", "r", "end"}, want: []string{"Rd8#"}},
		{name: "play-menu-80x24", width: 80, height: 24, keys: []string{"2"}, want: []string{"New game", "Strength", "no engine found"}},
		{name: "settings", width: 100, height: 32, keys: []string{"8"}, want: []string{"Settings", "Engine", "Lichess beta"}},
		{name: "wide-180x50", width: 180, height: 50, keys: []string{"4", "enter", "right", "right"}, want: []string{"Morphy"}},
		{name: "lichess-tv", width: 120, height: 36, keys: []string{"7"}, want: []string{"DrNykterstein", "live", "beta"}},
		{name: "lichess-play", width: 120, height: 36, token: "lio_fake", keys: []string{"7", ">", ">", "enter"}, want: []string{"Lichess AI · 5", "kannachi323"}},
	}
	for _, base := range scenes {
		variants := []scene{base}
		if base.card && *shotsDir != "" {
			card := base
			card.name, card.width, card.height, card.want = base.name+"-card", 84, 26, nil
			variants = append(variants, card)
		}
		for _, sc := range variants {
			t.Run(sc.name, func(t *testing.T) {
				store, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "shots.db"))
				if err != nil {
					t.Fatal(err)
				}
				defer store.Close()
				cfg := config.Default()
				cfg.LichessURL = server.URL
				// Pin the engine so shots match on machines with or without Stockfish.
				cfg.EnginePath = filepath.Join(t.TempDir(), "no-engine")
				if sc.engine {
					cfg.EnginePath = fakeEngine(t)
				}
				var tokens interface {
					Load() (string, error)
					Save(string) error
					Delete() error
				} = memoryTokens{}
				if sc.token != "" {
					tokens = staticTokens{sc.token}
				}
				current := session.New(cfg, config.Paths{ConfigFile: filepath.Join(t.TempDir(), "config.json"), Database: "~/.local/share/chesstui/chesstui.db", DataDir: t.TempDir()}, store, tokens)
				defer current.Close()
				model := New(current)
				background := tea.BackgroundColorMsg{Color: color.RGBA{0x11, 0x12, 0x14, 0xff}}
				if sc.light {
					background = tea.BackgroundColorMsg{Color: color.RGBA{0xfa, 0xf9, 0xf6, 0xff}}
				}
				model = drive(model, tea.WindowSizeMsg{Width: sc.width, Height: sc.height}, background)
				model = drive(model, run(model.Init())...)
				for _, k := range sc.keys {
					if file, ok := strings.CutPrefix(k, "paste:"); ok {
						data, err := os.ReadFile(filepath.Join("testdata", file))
						if err != nil {
							t.Fatal(err)
						}
						model = drive(model, tea.PasteMsg{Content: string(data)})
						continue
					}
					model = drive(model, key(k))
				}
				frame := model.render()
				plain := ansi.Strip(frame)
				for _, want := range sc.want {
					if !strings.Contains(plain, want) {
						t.Fatalf("frame missing %q:\n%s", want, plain)
					}
				}
				if *shotsDir == "" {
					return
				}
				opts := termshot.Options{Background: "#111214", Foreground: "#e7e4dc"}
				if sc.light {
					opts = termshot.Options{Background: "#faf9f6", Foreground: "#22211d"}
				}
				if err := os.MkdirAll(*shotsDir, 0o755); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(*shotsDir, sc.name+".svg")
				if err := os.WriteFile(path, []byte(termshot.SVG(frame, opts)), 0o644); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

// TestDailyPuzzleSolvesEndToEnd types the recorded solution into the
// Puzzles screen and expects the solved state and a stored attempt.
func TestDailyPuzzleSolvesEndToEnd(t *testing.T) {
	server := fakeLichess(t)
	var daily struct {
		Puzzle struct{ Solution []string } `json:"puzzle"`
	}
	data, _ := os.ReadFile(filepath.Join("testdata", "puzzle-daily.json"))
	if err := json.Unmarshal(data, &daily); err != nil {
		t.Fatal(err)
	}
	store, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "solve.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	cfg := config.Default()
	cfg.LichessURL = server.URL
	model := New(session.New(cfg, config.Paths{}, store, memoryTokens{}))
	model = drive(model, tea.WindowSizeMsg{Width: 100, Height: 30}, key("5"))
	for i := 0; i < len(daily.Puzzle.Solution); i += 2 {
		for _, r := range daily.Puzzle.Solution[i] {
			model = drive(model, key(string(r)))
		}
		model = drive(model, key("enter"))
	}
	if plain := ansi.Strip(model.render()); !strings.Contains(plain, "Solved") {
		t.Fatalf("puzzle not solved:\n%s", plain)
	}
	stats, err := store.PuzzleStats(context.Background())
	if err != nil || stats.Solved != 1 {
		t.Fatalf("stats=%+v err=%v", stats, err)
	}
}

// TestHeroFrames records the reader stepping through the Opera game, for
// the animated hero on the website. It only runs when writing shots.
func TestHeroFrames(t *testing.T) {
	if *shotsDir == "" {
		t.Skip("run with -shots")
	}
	store, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "hero.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	cfg := config.Default()
	cfg.LichessURL = "http://127.0.0.1:1"
	model := New(session.New(cfg, config.Paths{DataDir: t.TempDir()}, store, memoryTokens{}))
	model = drive(model, tea.WindowSizeMsg{Width: 120, Height: 32}, tea.BackgroundColorMsg{Color: color.RGBA{0x11, 0x12, 0x14, 0xff}})
	model = drive(model, key("4"), key("enter"))
	dir := filepath.Join(*shotsDir, "hero")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := range 16 {
		model = drive(model, key("right"))
		svg := termshot.SVG(model.render(), termshot.Options{Background: "#111214", Foreground: "#e7e4dc"})
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%02d.svg", i)), []byte(svg), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if plain := ansi.Strip(model.render()); !strings.Contains(plain, "Morphy") {
		t.Fatalf("hero frame:\n%s", plain)
	}
}

func TestQuitAsksTwiceDuringAnOnlineGame(t *testing.T) {
	server := fakeLichess(t)
	store, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "quit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	cfg := config.Default()
	cfg.LichessURL = server.URL
	model := New(session.New(cfg, config.Paths{}, store, staticTokens{"lio_fake"}))
	model = drive(model, tea.WindowSizeMsg{Width: 120, Height: 36})
	model = drive(model, key("7"), key(">"), key(">"), key("enter"))
	updated, cmd := model.Update(key("q"))
	model = updated.(Model)
	if cmd != nil || !strings.Contains(ansi.Strip(model.render()), "press q again") {
		t.Fatalf("first q should warn during a live game (cmd=%v):\n%s", cmd != nil, ansi.Strip(model.render()))
	}
	if _, cmd = model.Update(key("q")); cmd == nil {
		t.Fatal("second q should quit")
	}
}
