package app

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/table"
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

func TestParseCommitMetaComplete(t *testing.T) {
	// Test with complete commit metadata (format: SHA\x1fAuthor\x1fEmail\x1fDate\x1fSubject\x1fBody)
	raw := "d25c6aa6b03f571cf0714e7a56a49053c3bdebf0\x1fChmouel Boudjnah\x1fchmouel@chmouel.com\x1fMon Dec 29 19:33:24 2025 +0100\x1ffeat: Add prune merged worktrees command\x1fIntroduced a new command to automatically identify and prune worktrees\nassociated with merged pull or merge requests. This feature helps maintain a\nclean and organized workspace by removing obsolete worktrees, thereby improving\nefficiency. The command prompts for confirmation before proceeding with the\ndeletion of any identified merged worktrees.\n\nSigned-off-by: Chmouel Boudjnah <chmouel@chmouel.com>"

	meta := parseCommitMeta(raw)

	if meta.sha != "d25c6aa6b03f571cf0714e7a56a49053c3bdebf0" {
		t.Errorf("Expected SHA 'd25c6aa6b03f571cf0714e7a56a49053c3bdebf0', got %q", meta.sha)
	}
	if meta.author != "Chmouel Boudjnah" {
		t.Errorf("Expected author 'Chmouel Boudjnah', got %q", meta.author)
	}
	if meta.email != "chmouel@chmouel.com" {
		t.Errorf("Expected email 'chmouel@chmouel.com', got %q", meta.email)
	}
	if meta.date != "Mon Dec 29 19:33:24 2025 +0100" {
		t.Errorf("Expected date 'Mon Dec 29 19:33:24 2025 +0100', got %q", meta.date)
	}
	if meta.subject != "feat: Add prune merged worktrees command" {
		t.Errorf("Expected subject 'feat: Add prune merged worktrees command', got %q", meta.subject)
	}
	if len(meta.body) == 0 {
		t.Fatal("Expected body to be non-empty")
	}
	bodyText := strings.Join(meta.body, "\n")
	if !strings.Contains(bodyText, "Introduced a new command") {
		t.Errorf("Expected body to contain 'Introduced a new command', got %q", bodyText)
	}
	if !strings.Contains(bodyText, "Signed-off-by") {
		t.Errorf("Expected body to contain 'Signed-off-by', got %q", bodyText)
	}
}

func TestParseCommitMetaMinimal(t *testing.T) {
	// Test with minimal commit metadata (only SHA)
	raw := "abc123"

	meta := parseCommitMeta(raw)

	if meta.sha != "abc123" {
		t.Errorf("Expected SHA 'abc123', got %q", meta.sha)
	}
	if meta.author != "" {
		t.Errorf("Expected empty author, got %q", meta.author)
	}
	if meta.email != "" {
		t.Errorf("Expected empty email, got %q", meta.email)
	}
	if meta.date != "" {
		t.Errorf("Expected empty date, got %q", meta.date)
	}
	if meta.subject != "" {
		t.Errorf("Expected empty subject, got %q", meta.subject)
	}
	if len(meta.body) != 0 {
		t.Errorf("Expected empty body, got %v", meta.body)
	}
}

func TestParseCommitMetaNoBody(t *testing.T) {
	// Test with commit metadata but no body (format: SHA\x1fAuthor\x1fEmail\x1fDate\x1fSubject)
	raw := "abc123\x1fJohn Doe\x1fjohn@example.com\x1fMon Jan 1 00:00:00 2025 +0000\x1ffix: Bug fix"

	meta := parseCommitMeta(raw)

	if meta.sha != "abc123" {
		t.Errorf("Expected SHA 'abc123', got %q", meta.sha)
	}
	if meta.author != "John Doe" {
		t.Errorf("Expected author 'John Doe', got %q", meta.author)
	}
	if meta.email != "john@example.com" {
		t.Errorf("Expected email 'john@example.com', got %q", meta.email)
	}
	if meta.date != "Mon Jan 1 00:00:00 2025 +0000" {
		t.Errorf("Expected date 'Mon Jan 1 00:00:00 2025 +0000', got %q", meta.date)
	}
	if meta.subject != "fix: Bug fix" {
		t.Errorf("Expected subject 'fix: Bug fix', got %q", meta.subject)
	}
	if len(meta.body) != 0 {
		t.Errorf("Expected empty body, got %v", meta.body)
	}
}
