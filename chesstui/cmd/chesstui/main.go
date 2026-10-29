package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"

	tea "charm.land/bubbletea/v2"
	"github.com/kannachi323/chesstui/internal/app"
	"github.com/kannachi323/chesstui/internal/auth"
	"github.com/kannachi323/chesstui/internal/config"
	"github.com/kannachi323/chesstui/internal/lichess"
	"github.com/kannachi323/chesstui/internal/session"
	"github.com/kannachi323/chesstui/internal/storage"
)

// Set by GoReleaser through -ldflags "-X main.version=… -X main.commit=… -X main.date=…".
var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	showVersion := flag.Bool("version", false, "print the ChessTUI version")
	flag.Parse()
	if *showVersion {
		fmt.Println(versionString())
		return
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "chesstui:", err)
		os.Exit(1)
	}
}

func versionString() string {
	v, c, d := version, commit, date
	// `go install …@vX` builds carry module and VCS data instead of ldflags.
	if info, ok := debug.ReadBuildInfo(); ok {
		if v == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
			v = info.Main.Version
		}
		for _, setting := range info.Settings {
			switch {
			case setting.Key == "vcs.revision" && c == "":
				c = setting.Value
			case setting.Key == "vcs.time" && d == "":
				d = setting.Value
			}
		}
	}
	out := "chesstui " + v
	if len(c) > 7 {
		c = c[:7]
	}
	if c != "" {
		out += " (" + c
		if d != "" {
			out += ", " + d
		}
		out += ")"
	}
	return out
}

func run() error {
	lichess.SetVersion(version)
	paths, err := config.ResolvePaths()
	if err != nil {
		return err
	}
	logFile, err := os.OpenFile(paths.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open log: %w", err)
	}
	defer logFile.Close()
	slog.SetDefault(slog.New(slog.NewJSONHandler(logFile, nil)))

	cfg, err := config.Load(paths.ConfigFile)
	if err != nil {
		return err
	}
	store, err := storage.Open(context.Background(), paths.Database)
	if err != nil {
		return err
	}
	defer store.Close()

	current := session.New(cfg, paths, store, auth.SystemTokenStore{FallbackPath: paths.TokenFile})
	defer current.Close()
	program := tea.NewProgram(app.New(current))
	if _, err := program.Run(); err != nil {
		slog.Error("application stopped", "error", err)
		return fmt.Errorf("run terminal interface: %w", err)
	}
	return nil
}
