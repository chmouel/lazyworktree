package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
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
	m.view.WindowWidth = 120
	m.view.WindowHeight = 40

	// Test mouse wheel events
	mouseMsg := tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelUp,
		X:      10,
		Y:      5,
	}

	result, _ := m.handleMouse(mouseMsg)
	if result == nil {
		t.Fatal("handleMouse returned nil model")
	}

	// Test mouse click
	mouseMsg = tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      10,
		Y:      5,
	}

	result, _ = m.handleMouse(mouseMsg)
	if result == nil {
		t.Fatal("handleMouse returned nil model")
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
	m.filteredWts = []*models.WorktreeInfo{{Path: selected}}
	m.worktreeTable.SetRows([]table.Row{{"worktree"}})
	m.selectedIndex = 0

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
