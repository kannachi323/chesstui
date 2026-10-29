package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
)

const applicationName = "chesstui"

// Paths contains every filesystem location used by ChessTUI.
type Paths struct {
	ConfigFile string
	DataDir    string
	CacheDir   string
	StateDir   string
	Database   string
	LogFile    string
	// TokenFile is used only when the OS credential store is unavailable.
	TokenFile string
}

// Config contains user-editable local preferences. Secrets are stored separately.
type Config struct {
	Theme             string `json:"theme"` // auto, dark or light
	Glyphs            string `json:"glyphs"`
	LichessURL        string `json:"lichess_url"`
	StockfishPath     string `json:"stockfish_path,omitempty"`
	AnalysisDepth     int    `json:"analysis_depth"`
	DefaultWatch      string `json:"default_watch_channel"`
	MouseEnabled      bool   `json:"mouse_enabled"`
	ConfirmResign     bool   `json:"confirm_resign"`
	PuzzleRatingFloor int    `json:"puzzle_rating_floor"`
	PuzzleRatingCeil  int    `json:"puzzle_rating_ceil"`
}

func Default() Config {
	return Config{
		Theme:             "auto",
		Glyphs:            "solid",
		LichessURL:        "https://lichess.org",
		AnalysisDepth:     16,
		DefaultWatch:      "best",
		MouseEnabled:      true,
		ConfirmResign:     true,
		PuzzleRatingFloor: 0,
		PuzzleRatingCeil:  4000,
	}
}

func ResolvePaths() (Paths, error) {
	configFile, err := xdg.ConfigFile(filepath.Join(applicationName, "config.json"))
	if err != nil {
		return Paths{}, fmt.Errorf("resolve config path: %w", err)
	}
	dataDir := filepath.Join(xdg.DataHome, applicationName)
	cacheDir := filepath.Join(xdg.CacheHome, applicationName)
	stateDir := filepath.Join(xdg.StateHome, applicationName)
	for _, dir := range []string{dataDir, cacheDir, stateDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return Paths{}, fmt.Errorf("create %s: %w", dir, err)
		}
	}
	return Paths{
		ConfigFile: configFile,
		DataDir:    dataDir,
		CacheDir:   cacheDir,
		StateDir:   stateDir,
		Database:   filepath.Join(dataDir, "chesstui.db"),
		LogFile:    filepath.Join(stateDir, "chesstui.log"),
		TokenFile:  filepath.Join(dataDir, "lichess-token"),
	}, nil
}

func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	normalize(&cfg)
	return cfg, nil
}

func Save(path string, cfg Config) error {
	normalize(&cfg)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("protect temporary config: %w", err)
	}
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return fmt.Errorf("write config: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close config: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}

func normalize(cfg *Config) {
	switch cfg.Theme {
	case "auto", "dark", "light":
	default:
		cfg.Theme = "auto"
	}
	switch cfg.Glyphs {
	case "", "unicode":
		cfg.Glyphs = "solid"
	case "solid", "classic", "ascii":
	default:
		cfg.Glyphs = "solid"
	}
	if cfg.LichessURL == "" {
		cfg.LichessURL = Default().LichessURL
	}
	if cfg.AnalysisDepth < 1 || cfg.AnalysisDepth > 40 {
		cfg.AnalysisDepth = Default().AnalysisDepth
	}
	if cfg.PuzzleRatingCeil <= cfg.PuzzleRatingFloor {
		cfg.PuzzleRatingFloor = Default().PuzzleRatingFloor
		cfg.PuzzleRatingCeil = Default().PuzzleRatingCeil
	}
}
