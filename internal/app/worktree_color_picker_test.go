package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

func TestShowSetWorktreeColorIncludesCustomOption(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)
	m.state.data.filteredWts = []*models.WorktreeInfo{{Path: "/tmp/wt", Branch: "feat"}}
	m.state.data.selectedIndex = 0

	m.showSetWorktreeColor()

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeListSelect {
		t.Fatalf("expected list selection screen, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}
	listScreen := m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)
	if len(listScreen.Items) < 2 {
		t.Fatalf("expected custom picker items, got %d", len(listScreen.Items))
	}
	if listScreen.Items[1].ID != worktreeColorCustomID {
		t.Fatalf("expected second item to be custom option, got %q", listScreen.Items[1].ID)
	}
}

func TestShowSetWorktreeColorCustomInputUsesExistingCustomValue(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)
	path := "/tmp/wt"
	m.state.data.filteredWts = []*models.WorktreeInfo{{Path: path, Branch: "feat"}}
	m.state.data.selectedIndex = 0
	m.setWorktreeColor(path, "#ff0000")

	m.showSetWorktreeColor()
	listScreen := m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)
	customItem := listScreen.Items[1]
	if customItem.ID != worktreeColorCustomID {
		t.Fatalf("expected custom option, got %q", customItem.ID)
	}

	listScreen.OnSelect(customItem)

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInput {
		t.Fatalf("expected custom input screen, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}
	inputScr := m.state.ui.screenManager.Current().(*appscreen.InputScreen)
	if inputScr.Input.Value() != "#ff0000" {
		t.Fatalf("expected existing custom colour in input, got %q", inputScr.Input.Value())
	}
}

func TestCustomWorktreeColorInputRejectsInvalidValues(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	m.showCustomWorktreeColorInput("/tmp/wt", "")

	inputScr := m.state.ui.screenManager.Current().(*appscreen.InputScreen)
	inputScr.Input.SetValue("notacolor")

	next, _ := inputScr.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if next == nil {
		t.Fatal("expected input screen to remain open on invalid value")
	}
	if inputScr.ErrorMsg == "" {
		t.Fatal("expected validation error for invalid custom colour")
	}
}

func TestCustomWorktreeColorInputSavesValidHex(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.repoKey = testRepoKey

	path := "/tmp/wt"
	m.showCustomWorktreeColorInput(path, "")

	inputScr := m.state.ui.screenManager.Current().(*appscreen.InputScreen)
	inputScr.Input.SetValue("#ff0000")

	next, _ := inputScr.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if next != nil {
		t.Fatal("expected screen to close after valid submit")
	}

	note, ok := m.getWorktreeNote(path)
	if !ok {
		t.Fatal("expected note to be stored")
	}
	if note.Color != "#ff0000" {
		t.Fatalf("expected saved colour #ff0000, got %q", note.Color)
	}
}
