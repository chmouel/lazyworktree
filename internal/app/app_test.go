package app

import (
	"os"
	"path/filepath"
	"testing"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

const (
	randomSHA          = "abc123"
	testRandomName     = "main-random123"
	testDiff           = "diff"
	testFallback       = "fallback"
	testShell          = "shell"
	mruSectionLabel    = "Recently Used"
	testCommandCreate  = "create"
	testCommandRefresh = "refresh"
	testPRURL          = "https://example.com/pr/1"
	featureBranch      = "feature"
)

func TestHandleMouseDoesNotPanic(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: "/tmp/test",
	}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40

	// Test mouse wheel events
	wheelMsg := tea.MouseWheelMsg{
		Button: tea.MouseWheelUp,
		X:      10,
		Y:      5,
	}

	result, _ := m.handleMouseWheel(wheelMsg)
	if result == nil {
		t.Fatal("handleMouseWheel returned nil model")
	}

	// Test mouse click
	clickMsg := tea.MouseClickMsg{
		Button: tea.MouseLeft,
		X:      10,
		Y:      5,
	}

	result, _ = m.handleMouseClick(clickMsg)
	if result == nil {
		t.Fatal("handleMouseClick returned nil model")
	}
}

func TestPersistLastSelectedWritesFile(t *testing.T) {
	worktreeDir := t.TempDir()
	cfg := &config.AppConfig{
		WorktreeDir: worktreeDir,
	}
	m := NewModel(cfg, "")
	m.repoKey = "example/repo"

	selected := filepath.Join(t.TempDir(), "worktree")
	m.persistLastSelected(selected)

	lastSelectedPath := filepath.Join(worktreeDir, "example", "repo", models.LastSelectedFilename)
	// #nosec G304 -- test reads from a temp dir path we control.
	data, err := os.ReadFile(lastSelectedPath)
	if err != nil {
		t.Fatalf("expected last-selected file to be created: %v", err)
	}
	if got := string(data); got != selected+"\n" {
		t.Fatalf("expected %q, got %q", selected+"\n", got)
	}
}

func TestClosePersistsCurrentSelection(t *testing.T) {
	worktreeDir := t.TempDir()
	cfg := &config.AppConfig{
		WorktreeDir: worktreeDir,
	}
	m := NewModel(cfg, "")
	m.repoKey = "example/repo"

	selected := filepath.Join(t.TempDir(), "worktree")
	m.state.data.filteredWts = []*models.WorktreeInfo{{Path: selected}}
	m.state.ui.worktreeTable.SetRows([]table.Row{{"worktree"}})
	m.state.data.selectedIndex = 0

	m.Close()

	lastSelectedPath := filepath.Join(worktreeDir, "example", "repo", models.LastSelectedFilename)
	// #nosec G304 -- test reads from a temp dir path we control.
	data, err := os.ReadFile(lastSelectedPath)
	if err != nil {
		t.Fatalf("expected last-selected file to be created: %v", err)
	}
	if got := string(data); got != selected+"\n" {
		t.Fatalf("expected %q, got %q", selected+"\n", got)
	}
}

func TestGetSelectedPath(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.selectedPath = "/tmp/selected"

	if got := m.GetSelectedPath(); got != "/tmp/selected" {
		t.Fatalf("expected selected path, got %q", got)
	}
}
