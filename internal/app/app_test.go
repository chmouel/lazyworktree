package app

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

func TestFilterPaletteItemsEmptyQueryReturnsAll(t *testing.T) {
	items := []paletteItem{
		{id: "create", label: "Create worktree", description: "Add a new worktree"},
		{id: "delete", label: "Delete worktree", description: "Remove worktree"},
		{id: "help", label: "Help", description: "Show help"},
	}

	got := filterPaletteItems(items, "")
	if !reflect.DeepEqual(got, items) {
		t.Fatalf("expected items to be unchanged, got %#v", got)
	}
}

func TestFilterPaletteItemsPrefersLabelMatches(t *testing.T) {
	items := []paletteItem{
		{id: "desc", label: "Delete worktree", description: "Create new worktree"},
		{id: "label", label: "Create worktree", description: "Remove worktree"},
		{id: "help", label: "Help", description: "Show help"},
	}

	got := filterPaletteItems(items, "create")
	if len(got) < 2 {
		t.Fatalf("expected at least two matches, got %d", len(got))
	}
	if got[0].id != "label" {
		t.Fatalf("expected label match first, got %q", got[0].id)
	}
	if got[1].id != "desc" {
		t.Fatalf("expected description match second, got %q", got[1].id)
	}
}

func TestFuzzyScoreLowerMissingChars(t *testing.T) {
	if _, ok := fuzzyScoreLower("zz", "create worktree"); ok {
		t.Fatalf("expected fuzzy match to fail")
	}
}

func TestHandleMouseDoesNotPanic(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: "/tmp/test",
	}
	m := NewModel(cfg, "")
	m.windowWidth = 120
	m.windowHeight = 40

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

func TestShowCommandPaletteIncludesCreateFromChanges(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)

	// Show command palette
	cmd := m.showCommandPalette()
	if cmd == nil {
		t.Fatal("showCommandPalette returned nil command")
	}

	// Check that palette screen was created
	if m.paletteScreen == nil {
		t.Fatal("paletteScreen should be initialized")
	}

	// Check that palette includes create-from-changes
	items := m.paletteScreen.items
	found := false
	for _, item := range items {
		if item.id == "create-from-changes" {
			found = true
			if item.label != "Create from changes" {
				t.Errorf("Expected label 'Create from changes', got %q", item.label)
			}
			if item.description != "Create a new worktree from current uncommitted changes" {
				t.Errorf("Expected description 'Create a new worktree from current uncommitted changes', got %q", item.description)
			}
			break
		}
	}
	if !found {
		t.Fatal("create-from-changes item not found in command palette")
	}
}

func TestShowCreateWorktreeFromChangesNoSelection(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.selectedIndex = -1 // No selection

	cmd := m.showCreateWorktreeFromChanges()
	if cmd != nil {
		t.Error("Expected nil command when no worktree is selected")
	}
	if m.statusContent != errNoWorktreeSelected {
		t.Errorf("Expected status %q, got %q", errNoWorktreeSelected, m.statusContent)
	}
}

func TestShowCommandPaletteIncludesCustomCommands(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		CustomCommands: map[string]*config.CustomCommand{
			"x": {
				Command:     "make test",
				Description: "Run tests",
				ShowHelp:    true,
			},
		},
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)

	cmd := m.showCommandPalette()
	if cmd == nil {
		t.Fatal("showCommandPalette returned nil command")
	}
	if m.paletteScreen == nil {
		t.Fatal("paletteScreen should be initialized")
	}

	items := m.paletteScreen.items
	found := false
	for _, item := range items {
		if item.id == "x" {
			found = true
			if item.label != "Run tests (x)" {
				t.Errorf("Expected label 'Run tests (x)', got %q", item.label)
			}
			if item.description != "make test" {
				t.Errorf("Expected description 'make test', got %q", item.description)
			}
			break
		}
	}
	if !found {
		t.Fatal("custom command item not found in command palette")
	}
}

func TestRenderFooterIncludesCustomHelpHints(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		CustomCommands: map[string]*config.CustomCommand{
			"x": {
				Command:     "make test",
				Description: "Run tests",
				ShowHelp:    true,
			},
		},
	}
	m := NewModel(cfg, "")
	layout := m.computeLayout()
	footer := m.renderFooter(layout)

	if !strings.Contains(footer, "Run tests") {
		t.Fatalf("expected footer to include custom command label, got %q", footer)
	}
}

func TestCreateFromChangesReadyMsg(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)

	// Create a mock worktree
	wt := &models.WorktreeInfo{
		Path:   "/tmp/test-worktree",
		Branch: "main",
	}

	msg := createFromChangesReadyMsg{
		worktree:      wt,
		currentBranch: "main",
	}

	// Handle the message
	cmd := m.handleCreateFromChangesReady(msg)
	if cmd == nil {
		t.Fatal("handleCreateFromChangesReady returned nil command")
	}

	// Check that input screen was set up
	if m.inputScreen == nil {
		t.Fatal("inputScreen should be initialized")
	}

	if m.inputScreen.prompt != "Create worktree from changes: branch name" {
		t.Errorf("Expected prompt 'Create worktree from changes: branch name', got %q", m.inputScreen.prompt)
	}

	// Check default value
	if m.inputScreen.value != "main-changes" {
		t.Errorf("Expected default value 'main-changes', got %q", m.inputScreen.value)
	}

	if m.currentScreen != screenInput {
		t.Errorf("Expected currentScreen to be screenInput, got %v", m.currentScreen)
	}
}
