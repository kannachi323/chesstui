//go:build js

// Command chesstui-web is chesstui compiled to WebAssembly for the landing
// page, where it runs in xterm.js. The page provides a bridge object,
// globalThis.chesstui:
//
//	cols, rows   the terminal size at start
//	dark         whether the page is in dark mode
//	write(bytes) writes program output to the terminal
//	load(key)    reads a string from localStorage, or null
//	save(key, s) writes a string to localStorage
//	exit(error)  called when the program ends
//
// and receives input(text), resize(cols, rows), theme(dark) and stop()
// functions from Go. The engine is chess-api.com; files and Lichess are unavailable.
package main

import (
	"context"
	"image/color"
	"io"
	"log/slog"
	"syscall/js"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/kannachi323/chesstui/internal/app"
	"github.com/kannachi323/chesstui/internal/auth"
	"github.com/kannachi323/chesstui/internal/config"
	"github.com/kannachi323/chesstui/internal/engine"
	"github.com/kannachi323/chesstui/internal/session"
	"github.com/kannachi323/chesstui/internal/storage"
)

const (
	configKey  = "chesstui.config"
	libraryKey = "chesstui.library"
)

func main() {
	bridge := js.Global().Get("chesstui")
	slog.SetDefault(slog.New(slog.DiscardHandler))

	cfg := config.Default()
	cfg.EngineAPI = engine.DefaultAPI
	cfg.AnalysisDepth, cfg.ReviewDepth = 16, 12
	if saved := load(bridge, configKey); saved != "" {
		if decoded, err := config.Decode([]byte(saved), cfg); err == nil {
			cfg = decoded
		}
	}
	cfg.EngineAPI = engine.DefaultAPI
	config.SaveHook = func(data []byte) error {
		bridge.Call("save", configKey, string(data))
		return nil
	}

	store, _ := storage.Open(context.Background(), "")
	store.Restore([]byte(load(bridge, libraryKey)))
	store.OnChange = func(data []byte) { bridge.Call("save", libraryKey, string(data)) }

	paths := config.Paths{
		ConfigFile: "browser storage",
		Database:   "browser storage",
		DataDir:    "browser storage",
	}
	current := session.New(cfg, paths, store, &memoryTokens{})

	input, inputWriter := io.Pipe()
	keys := make(chan string, 4096)
	go func() {
		for text := range keys {
			_, _ = io.WriteString(inputWriter, text)
		}
	}()

	program := tea.NewProgram(app.New(current),
		tea.WithInput(input),
		tea.WithOutput(terminal{bridge}),
		tea.WithWindowSize(bridge.Get("cols").Int(), bridge.Get("rows").Int()),
		tea.WithEnvironment([]string{"TERM=xterm-256color", "COLORTERM=truecolor"}),
		tea.WithColorProfile(colorprofile.TrueColor),
		tea.WithoutSignalHandler(),
	)

	// JS callbacks must not block, so input is queued for a goroutine.
	bridge.Set("input", js.FuncOf(func(_ js.Value, args []js.Value) any {
		select {
		case keys <- args[0].String():
		default:
		}
		return nil
	}))
	bridge.Set("resize", js.FuncOf(func(_ js.Value, args []js.Value) any {
		size := tea.WindowSizeMsg{Width: args[0].Int(), Height: args[1].Int()}
		go func() {
			program.Send(size)
			// xterm.js reflows on resize; repaint rather than diff.
			program.Send(tea.ClearScreen())
		}()
		return nil
	}))
	// xterm.js does not answer background color queries, so the page
	// reports its theme instead.
	sendTheme := func(dark bool) {
		background := color.Color(color.RGBA{0xf6, 0xf4, 0xef, 0xff})
		if dark {
			background = color.RGBA{0x11, 0x12, 0x14, 0xff}
		}
		go program.Send(tea.BackgroundColorMsg{Color: background})
	}
	bridge.Set("theme", js.FuncOf(func(_ js.Value, args []js.Value) any {
		sendTheme(args[0].Bool())
		return nil
	}))
	sendTheme(bridge.Get("dark").Bool())
	bridge.Set("stop", js.FuncOf(func(js.Value, []js.Value) any {
		go program.Quit()
		return nil
	}))

	_, err := program.Run()
	current.Close()
	message := ""
	if err != nil {
		message = err.Error()
	}
	if exit := bridge.Get("exit"); exit.Type() == js.TypeFunction {
		exit.Invoke(message)
	}
}

func load(bridge js.Value, key string) string {
	value := bridge.Call("load", key)
	if value.Type() != js.TypeString {
		return ""
	}
	return value.String()
}

// terminal writes program output to xterm.js as bytes, so a UTF-8
// sequence split across writes is still decoded correctly.
type terminal struct{ bridge js.Value }

func (t terminal) Write(p []byte) (int, error) {
	buffer := js.Global().Get("Uint8Array").New(len(p))
	js.CopyBytesToJS(buffer, p)
	t.bridge.Call("write", buffer)
	return len(p), nil
}

// memoryTokens satisfies the session's token store; the browser build has
// no Lichess sign-in.
type memoryTokens struct{}

func (*memoryTokens) Load() (string, error) { return "", auth.ErrTokenNotFound }
func (*memoryTokens) Save(string) error     { return nil }
func (*memoryTokens) Delete() error         { return nil }
