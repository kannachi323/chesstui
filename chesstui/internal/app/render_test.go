package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kannachi323/chesstui/internal/config"
	"github.com/kannachi323/chesstui/internal/session"
	"github.com/kannachi323/chesstui/internal/storage"
)

func newTestModel(t *testing.T) Model {
	t.Helper()
	store, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "render.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	cfg := config.Default()
	cfg.LichessURL = "http://127.0.0.1:1" // never reach the real Lichess from tests
	return New(session.New(cfg, config.Paths{}, store, memoryTokens{}))
}

// TestEveryScreenFitsEveryTerminal renders each section, with and without the
// help overlay, from below the minimum size up to a wide terminal. The frame
// must fill the terminal exactly and never write past its right edge.
func TestEveryScreenFitsEveryTerminal(t *testing.T) {
	sizes := [][2]int{{1, 1}, {20, 8}, {40, 12}, {60, 20}, {80, 24}, {100, 30}, {120, 40}, {220, 60}}
	model := newTestModel(t)
	for screen := range labels {
		for _, help := range []bool{false, true} {
			for _, size := range sizes {
				m := model
				m.active, m.help = screen, help
				updated, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
				content := updated.(Model).View().Content
				lines := strings.Split(content, "\n")
				if len(lines) != size[1] {
					t.Fatalf("%s %dx%d help=%v: %d lines", labels[screen], size[0], size[1], help, len(lines))
				}
				for i, line := range lines {
					if w := ansi.StringWidth(line); w > size[0] {
						t.Fatalf("%s %dx%d: line %d is %d wide\n%s", labels[screen], size[0], size[1], i, w, ansi.Strip(content))
					}
				}
			}
		}
	}
}

func TestMoveEntryKeepsGlobalKeysAway(t *testing.T) {
	model := newTestModel(t)
	model.width, model.height = 100, 30
	model.active = 5 // Learn: first lesson is e2e4
	model = pressRunes(model, "g")
	if model.pending != "" {
		t.Fatal("g on a board should start a move, not a gt/gT chord")
	}
	model = pressSpecial(model, tea.KeyEscape)
	model = pressRunes(model, "e2e4")
	model = pressSpecial(model, tea.KeyEnter)
	if model.active != 5 {
		t.Fatalf("active = %d", model.active)
	}
	if !strings.Contains(ansi.Strip(model.render()), "nicely done") {
		t.Fatalf("lesson not completed:\n%s", ansi.Strip(model.render()))
	}
}

func TestCursorMovesAPiece(t *testing.T) {
	model := newTestModel(t)
	model.width, model.height = 100, 30
	model.active = 5
	// The cursor starts on e2; pick the pawn, go up two squares, drop.
	for _, key := range []rune{tea.KeyUp, tea.KeyEnter, tea.KeyUp, tea.KeyUp, tea.KeyEnter} {
		model = pressSpecial(model, key)
	}
	if !strings.Contains(ansi.Strip(model.render()), "nicely done") {
		t.Fatalf("cursor move failed:\n%s", ansi.Strip(model.render()))
	}
}

func TestHelpOverlayOpensAndCloses(t *testing.T) {
	model := newTestModel(t)
	model.width, model.height = 100, 30
	model = pressRunes(model, "?")
	if !model.help || !strings.Contains(ansi.Strip(model.render()), "EVERYWHERE") {
		t.Fatal("help did not open")
	}
	model = pressRunes(model, "?")
	if model.help {
		t.Fatal("help did not close")
	}
}
