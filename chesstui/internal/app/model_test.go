package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/kannachi323/chesstui/internal/auth"
	"github.com/kannachi323/chesstui/internal/config"
	"github.com/kannachi323/chesstui/internal/session"
	"github.com/kannachi323/chesstui/internal/storage"
	"github.com/kannachi323/chesstui/internal/ui/widgets"
)

type memoryTokens struct{}

type overflowScreen struct{}

func (overflowScreen) Init() tea.Cmd          { return nil }
func (overflowScreen) Update(tea.Msg) tea.Cmd { return nil }
func (overflowScreen) View(int, int) string   { return strings.Repeat("content\n", 100) }
func (overflowScreen) Status() string         { return "overflow test" }
func (overflowScreen) Hints() []widgets.Hint  { return nil }

func (memoryTokens) Load() (string, error) { return "", auth.ErrTokenNotFound }
func (memoryTokens) Save(string) error     { return nil }
func (memoryTokens) Delete() error         { return nil }

func TestLessonMoveDigitsDoNotTriggerGlobalNavigation(t *testing.T) {
	store, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	current := session.New(config.Default(), config.Paths{}, store, memoryTokens{})
	model := New(current)
	model.active = 5

	for _, key := range "e2e4" {
		updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: key, Text: string(key)}))
		model = updated.(Model)
	}
	if model.active != 5 {
		t.Fatalf("active screen = %d, lesson move triggered navigation", model.active)
	}
}

func TestVimNavigationAndCommandMode(t *testing.T) {
	store, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "navigation.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	current := session.New(config.Default(), config.Paths{}, store, memoryTokens{})
	model := New(current)

	model = pressRunes(model, "gt")
	if model.active != 1 {
		t.Fatalf("gt selected %d, want Play", model.active)
	}
	model = pressRunes(model, "gT")
	if model.active != 0 {
		t.Fatalf("gT selected %d, want Home", model.active)
	}

	model = pressSpecial(model, tea.KeyTab)
	model = pressRunes(model, "ll")
	if model.focus != focusNavigation || model.nav != 2 {
		t.Fatalf("navbar focus=%v cursor=%d", model.focus, model.nav)
	}
	model = pressSpecial(model, tea.KeyEnter)
	if model.active != 2 || model.focus != focusContent {
		t.Fatalf("navbar activation active=%d focus=%v", model.active, model.focus)
	}

	model = pressRunes(model, ":home")
	model = pressSpecial(model, tea.KeyEnter)
	if model.active != 0 || model.mode != modeNormal {
		t.Fatalf(":home active=%d mode=%v", model.active, model.mode)
	}
}

func TestViewFillsTerminalHeight(t *testing.T) {
	store, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "view.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	current := session.New(config.Default(), config.Paths{}, store, memoryTokens{})
	model := New(current)
	model.screens[0] = overflowScreen{}
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 160, Height: 50})
	view := updated.(Model).View()
	if lines := strings.Count(view.Content, "\n") + 1; lines != 50 {
		t.Fatalf("view has %d lines, want exactly 50", lines)
	}
}

func pressRunes(model Model, keys string) Model {
	for _, key := range keys {
		updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: key, Text: string(key)}))
		model = updated.(Model)
	}
	return model
}

func pressSpecial(model Model, key rune) Model {
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: key}))
	return updated.(Model)
}
