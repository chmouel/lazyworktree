package app

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

const randomSHA = "abc123"

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
	if m.currentScreen != screenInfo {
		t.Fatalf("expected info screen, got %v", m.currentScreen)
	}
	if m.infoScreen == nil || !strings.Contains(m.infoScreen.message, errNoWorktreeSelected) {
		t.Fatalf("expected info modal with %q, got %#v", errNoWorktreeSelected, m.infoScreen)
	}
}

func TestShowCreateWorktreeStartsWithBasePicker(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	cmd := m.showCreateWorktree()
	if cmd == nil {
		t.Fatal("showCreateWorktree returned nil command")
	}
	if m.currentScreen != screenListSelect {
		t.Fatalf("expected currentScreen screenListSelect, got %v", m.currentScreen)
	}
	if m.listScreen == nil {
		t.Fatal("listScreen should be initialized")
	}
	if m.listScreen.title != "Select base for new worktree" {
		t.Fatalf("unexpected list title: %q", m.listScreen.title)
	}
	if len(m.listScreen.items) != 3 {
		t.Fatalf("expected 3 base options, got %d", len(m.listScreen.items))
	}
	if m.listScreen.items[0].id != "branch-list" {
		t.Fatalf("expected first option branch-list, got %q", m.listScreen.items[0].id)
	}
}

func TestShowBranchNameInputUsesDefaultName(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	cmd := m.showBranchNameInput(mainWorktreeName, mainWorktreeName)
	if cmd == nil {
		t.Fatal("showBranchNameInput returned nil command")
	}
	if m.currentScreen != screenInput {
		t.Fatalf("expected currentScreen screenInput, got %v", m.currentScreen)
	}
	if m.inputScreen == nil {
		t.Fatal("inputScreen should be initialized")
	}
	got := m.inputScreen.input.Value()
	if !strings.HasPrefix(got, mainWorktreeName) {
		t.Fatalf("expected default input value to start with %q, got %q", mainWorktreeName, got)
	}
	if got != mainWorktreeName {
		suffix := strings.TrimPrefix(got, mainWorktreeName+"-")
		if suffix == got || suffix == "" {
			t.Fatalf("expected numeric suffix after %q, got %q", mainWorktreeName, got)
		}
		if _, err := strconv.Atoi(suffix); err != nil {
			t.Fatalf("expected numeric suffix after %q, got %q", mainWorktreeName, got)
		}
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

func TestShowCommandPaletteIncludesTmuxCommands(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		CustomCommands: map[string]*config.CustomCommand{
			"t": {
				Description: "Open tmux",
				ShowHelp:    true,
				Tmux: &config.TmuxCommand{
					SessionName: "${REPO_NAME}_wt_$WORKTREE_NAME",
					Attach:      true,
					OnExists:    "switch",
					Windows: []config.TmuxWindow{
						{Name: "shell"},
					},
				},
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
		if item.id == "t" {
			found = true
			if item.label != "Open tmux (t)" {
				t.Errorf("Expected label 'Open tmux (t)', got %q", item.label)
			}
			if item.description != tmuxSessionLabel {
				t.Errorf("Expected description %q, got %q", tmuxSessionLabel, item.description)
			}
			break
		}
	}
	if !found {
		t.Fatal("tmux command item not found in command palette")
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

func TestPagerCommandFallbacksToLess(t *testing.T) {
	if runtime.GOOS == osWindows {
		t.Skip("pager fallback test relies on unix-like PATH lookup")
	}

	originalPath := os.Getenv("PATH")
	originalPager := os.Getenv("PAGER")
	t.Cleanup(func() {
		_ = os.Setenv("PATH", originalPath)
		_ = os.Setenv("PAGER", originalPager)
	})

	tempDir := t.TempDir()
	lessPath := filepath.Join(tempDir, "less")
	if err := os.WriteFile(lessPath, []byte("#!/bin/sh\nexit 0\n"), 0o600); err != nil {
		t.Fatalf("failed to write fake less: %v", err)
	}
	// #nosec G302 -- test requires an executable file on PATH.
	if err := os.Chmod(lessPath, 0o700); err != nil {
		t.Fatalf("failed to chmod fake less: %v", err)
	}

	if err := os.Setenv("PATH", tempDir); err != nil {
		t.Fatalf("failed to set PATH: %v", err)
	}
	if err := os.Unsetenv("PAGER"); err != nil {
		t.Fatalf("failed to unset PAGER: %v", err)
	}

	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	if pager := m.pagerCommand(); pager != "less --use-color --wordwrap -qcR -P 'Press q to exit..'" {
		t.Fatalf("expected fallback pager to be less defaults, got %q", pager)
	}
}

func TestTmuxSessionReadyAttachesDirectly(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)

	updated, cmd := m.Update(tmuxSessionReadyMsg{sessionName: "wt_test", attach: true, insideTmux: false})
	model := updated.(*Model)
	if model.currentScreen != screenNone {
		t.Fatalf("expected no screen change, got %v", model.currentScreen)
	}
	if cmd == nil {
		t.Fatal("expected attach command to be returned")
	}
}

func TestTmuxSessionReadyShowsInfoWhenNotAttaching(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)

	updated, cmd := m.Update(tmuxSessionReadyMsg{sessionName: "wt_test", attach: false, insideTmux: false})
	model := updated.(*Model)
	if model.currentScreen != screenInfo {
		t.Fatalf("expected info screen, got %v", model.currentScreen)
	}
	if model.infoScreen == nil {
		t.Fatal("expected info screen to be created")
	}
	if cmd != nil {
		t.Fatal("expected no command when not attaching")
	}
	if !strings.Contains(model.infoScreen.message, "tmux attach-session -t 'wt_test'") {
		t.Errorf("expected attach message, got %q", model.infoScreen.message)
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
		Branch: mainWorktreeName,
	}

	msg := createFromChangesReadyMsg{
		worktree:      wt,
		currentBranch: mainWorktreeName,
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

func TestCreateFromChangesReadyMsgShowsInfoOnBranchNameScriptError(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir:      t.TempDir(),
		BranchNameScript: "false",
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)

	wt := &models.WorktreeInfo{
		Path:   "/tmp/test-worktree",
		Branch: mainWorktreeName,
	}

	msg := createFromChangesReadyMsg{
		worktree:      wt,
		currentBranch: mainWorktreeName,
		diff:          "diff",
	}

	cmd := m.handleCreateFromChangesReady(msg)
	if cmd != nil {
		t.Fatal("expected no command when showing info screen")
	}
	if m.currentScreen != screenInfo {
		t.Fatalf("expected info screen, got %v", m.currentScreen)
	}
	if m.infoScreen == nil || !strings.Contains(m.infoScreen.message, "Branch name script error") {
		t.Fatalf("expected branch name script error modal, got %#v", m.infoScreen)
	}

	_, action := m.handleScreenKey(tea.KeyMsg{Type: tea.KeyEnter})
	if action != nil {
		_ = action()
	}

	if m.currentScreen != screenInput {
		t.Fatalf("expected input screen, got %v", m.currentScreen)
	}
	if m.inputScreen == nil {
		t.Fatal("expected input screen to be set")
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
	raw := randomSHA
	meta := parseCommitMeta(raw)

	if meta.sha != raw {
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

	if meta.sha != randomSHA {
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

func TestShowAbsorbWorktreeNoSelection(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.selectedIndex = -1 // No selection

	cmd := m.showAbsorbWorktree()
	if cmd != nil {
		t.Error("Expected nil command when no worktree is selected")
	}
}

func TestShowAbsorbWorktreeOnMainWorktree(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	// Set up main worktree
	m.worktrees = []*models.WorktreeInfo{
		{Path: "/path/to/main", Branch: mainWorktreeName, IsMain: true},
	}
	m.filteredWts = m.worktrees
	m.selectedIndex = 0

	cmd := m.showAbsorbWorktree()
	if cmd != nil {
		t.Error("Expected nil command when trying to absorb main worktree")
	}
	if m.currentScreen != screenInfo {
		t.Error("Expected screenInfo to be shown for error")
	}
	if m.infoScreen == nil {
		t.Error("Expected infoScreen to be set")
	}
}

func TestShowAbsorbWorktreeCreatesConfirmScreen(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	// Set up main and feature worktrees
	m.worktrees = []*models.WorktreeInfo{
		{Path: "/path/to/main", Branch: mainWorktreeName, IsMain: true},
		{Path: "/path/to/feature", Branch: "feature-branch", IsMain: false},
	}
	m.filteredWts = m.worktrees
	m.selectedIndex = 1 // Select feature worktree

	cmd := m.showAbsorbWorktree()
	if cmd != nil {
		t.Error("Expected nil command from showAbsorbWorktree")
	}

	// Verify confirm screen was created
	if m.confirmScreen == nil {
		t.Fatal("Expected confirm screen to be created")
	}

	// Verify confirm action was set
	if m.confirmAction == nil {
		t.Fatal("Expected confirm action to be set")
	}

	// Verify current screen is set to confirm
	if m.currentScreen != screenConfirm {
		t.Errorf("Expected currentScreen to be screenConfirm, got %v", m.currentScreen)
	}

	// Verify the confirm message contains the correct information
	if m.confirmScreen.message == "" {
		t.Error("Expected confirm screen message to be set")
	}
	if !strings.Contains(m.confirmScreen.message, "Absorb worktree into main") {
		t.Errorf("Expected confirm message to mention 'Absorb worktree into main', got %q", m.confirmScreen.message)
	}
	if !strings.Contains(m.confirmScreen.message, "feature-branch") {
		t.Errorf("Expected confirm message to mention 'feature-branch', got %q", m.confirmScreen.message)
	}
}

func TestShowAbsorbWorktreeNoMainWorktree(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	// Set up only a feature worktree (no main)
	m.worktrees = []*models.WorktreeInfo{
		{Path: "/path/to/feature", Branch: "feature-branch", IsMain: false},
	}
	m.filteredWts = m.worktrees
	m.selectedIndex = 0

	cmd := m.showAbsorbWorktree()
	if cmd != nil {
		t.Error("Expected nil command when no main worktree exists")
	}
	if m.currentScreen != screenInfo {
		t.Error("Expected screenInfo to be shown for error")
	}
	if m.infoScreen == nil {
		t.Error("Expected infoScreen to be set")
	}
}

func TestShowAbsorbWorktreeOnMainBranch(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	// Set up worktrees where a non-main worktree is on the main branch
	m.worktrees = []*models.WorktreeInfo{
		{Path: "/path/to/main", Branch: mainWorktreeName, IsMain: true},
		{Path: "/path/to/other", Branch: mainWorktreeName, IsMain: false}, // Same branch as main
	}
	m.filteredWts = m.worktrees
	m.selectedIndex = 1 // Select the non-main worktree that's on main branch

	cmd := m.showAbsorbWorktree()
	if cmd != nil {
		t.Error("Expected nil command when worktree is on main branch")
	}
	if m.currentScreen != screenInfo {
		t.Error("Expected screenInfo to be shown for error")
	}
	if m.infoScreen == nil {
		t.Error("Expected infoScreen to be set")
	}
}

func TestShowAbsorbWorktreeDirtyMainWorktree(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	// Set up worktrees where main worktree is dirty
	m.worktrees = []*models.WorktreeInfo{
		{Path: "/path/to/main", Branch: mainWorktreeName, IsMain: true, Dirty: true},
		{Path: "/path/to/feature", Branch: "feature-branch", IsMain: false},
	}
	m.filteredWts = m.worktrees
	m.selectedIndex = 1 // Select the feature worktree

	cmd := m.showAbsorbWorktree()
	if cmd != nil {
		t.Error("Expected nil command when main worktree is dirty")
	}
	if m.currentScreen != screenInfo {
		t.Error("Expected screenInfo to be shown for error")
	}
	if m.infoScreen == nil {
		t.Error("Expected infoScreen to be set")
	}
}

func TestShowCherryPickNotInLogPane(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.focusedPane = 0 // Not in log pane

	cmd := m.showCherryPick()
	if cmd != nil {
		t.Error("Expected nil command when not in log pane")
	}
}

func TestShowCherryPickEmptyLogEntries(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.focusedPane = 2 // Log pane
	m.logEntries = []commitLogEntry{}

	cmd := m.showCherryPick()
	if cmd != nil {
		t.Error("Expected nil command when log entries are empty")
	}
}

func TestShowCherryPickNoOtherWorktrees(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.focusedPane = 2 // Log pane
	m.logEntries = []commitLogEntry{
		{sha: "abc1234", message: "Test commit"},
	}
	m.worktrees = []*models.WorktreeInfo{
		{Path: "/path/to/main", Branch: "main", IsMain: true},
	}
	m.filteredWts = m.worktrees
	m.selectedIndex = 0

	m.showCherryPick()
	if m.currentScreen != screenInfo {
		t.Error("Expected screenInfo to be shown")
	}
	if m.infoScreen == nil || !strings.Contains(m.infoScreen.message, "No other worktrees available") {
		t.Errorf("Expected info message about no worktrees, got: %v", m.infoScreen)
	}
}

func TestShowCherryPickCreatesListSelection(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.focusedPane = 2 // Log pane
	m.logEntries = []commitLogEntry{
		{sha: "abc1234", message: "Test commit"},
	}
	m.worktrees = []*models.WorktreeInfo{
		{Path: "/path/to/main", Branch: "main", IsMain: true},
		{Path: "/path/to/feature", Branch: "feature", IsMain: false},
	}
	m.filteredWts = m.worktrees
	m.selectedIndex = 0

	m.showCherryPick()
	if m.currentScreen != screenListSelect {
		t.Errorf("Expected screenListSelect, got %v", m.currentScreen)
	}
	if m.listScreen == nil {
		t.Fatal("Expected listScreen to be set")
	}
	if !strings.Contains(m.listScreen.title, "Cherry-pick") {
		t.Errorf("Expected cherry-pick in title, got: %s", m.listScreen.title)
	}
	// Should exclude source worktree
	if len(m.listScreen.items) != 1 {
		t.Errorf("Expected 1 target worktree (excluding source), got %d", len(m.listScreen.items))
	}
}

func TestShowCherryPickExcludesSourceWorktree(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.focusedPane = 2
	m.logEntries = []commitLogEntry{
		{sha: "abc1234", message: "Test commit"},
	}
	m.worktrees = []*models.WorktreeInfo{
		{Path: "/path/to/main", Branch: "main", IsMain: true},
		{Path: "/path/to/feature1", Branch: "feature1", IsMain: false},
		{Path: "/path/to/feature2", Branch: "feature2", IsMain: false},
	}
	m.filteredWts = m.worktrees
	m.selectedIndex = 1 // Select feature1

	m.showCherryPick()

	// Should have 2 items (main + feature2, excluding feature1)
	if len(m.listScreen.items) != 2 {
		t.Errorf("Expected 2 target worktrees, got %d", len(m.listScreen.items))
	}

	// Verify feature1 is not in the list
	for _, item := range m.listScreen.items {
		if item.id == "/path/to/feature1" {
			t.Error("Source worktree should be excluded from selection list")
		}
	}
}

func TestShowCherryPickMarksDirtyWorktrees(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.focusedPane = 2
	m.logEntries = []commitLogEntry{
		{sha: "abc1234", message: "Test commit"},
	}
	m.worktrees = []*models.WorktreeInfo{
		{Path: "/path/to/main", Branch: "main", IsMain: true},
		{Path: "/path/to/dirty", Branch: "dirty", IsMain: false, Dirty: true},
	}
	m.filteredWts = m.worktrees
	m.selectedIndex = 0

	m.showCherryPick()

	// Find the dirty worktree item
	var dirtyItem *selectionItem
	for _, item := range m.listScreen.items {
		if item.id == "/path/to/dirty" {
			dirtyItem = &item
			break
		}
	}

	if dirtyItem == nil {
		t.Fatal("Expected dirty worktree in selection list")
	}

	if !strings.Contains(dirtyItem.description, "(has changes)") {
		t.Errorf("Expected '(has changes)' marker in description, got: %s", dirtyItem.description)
	}
}

func TestHandleCherryPickResultSuccess(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	msg := cherryPickResultMsg{
		commitSHA: "abc1234",
		targetWorktree: &models.WorktreeInfo{
			Path:   "/path/to/feature",
			Branch: "feature",
		},
		err: nil,
	}

	cmd := m.handleCherryPickResult(msg)
	if cmd != nil {
		t.Error("Expected nil command from handleCherryPickResult")
	}

	if m.currentScreen != screenInfo {
		t.Error("Expected screenInfo to be shown")
	}

	if m.infoScreen == nil {
		t.Fatal("Expected infoScreen to be set")
	}

	if !strings.Contains(m.infoScreen.message, "Cherry-pick successful") {
		t.Errorf("Expected success message, got: %s", m.infoScreen.message)
	}

	if !strings.Contains(m.infoScreen.message, "abc1234") {
		t.Errorf("Expected commit SHA in message, got: %s", m.infoScreen.message)
	}
}

func TestHandleCherryPickResultError(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	msg := cherryPickResultMsg{
		commitSHA: "abc1234",
		targetWorktree: &models.WorktreeInfo{
			Path:   "/path/to/feature",
			Branch: "feature",
		},
		err: fmt.Errorf("cherry-pick conflicts occurred"),
	}

	cmd := m.handleCherryPickResult(msg)
	if cmd != nil {
		t.Error("Expected nil command from handleCherryPickResult")
	}

	if m.currentScreen != screenInfo {
		t.Error("Expected screenInfo to be shown")
	}

	if m.infoScreen == nil {
		t.Fatal("Expected infoScreen to be set")
	}

	if !strings.Contains(m.infoScreen.message, "Cherry-pick failed") {
		t.Errorf("Expected failure message, got: %s", m.infoScreen.message)
	}

	if !strings.Contains(m.infoScreen.message, "conflicts occurred") {
		t.Errorf("Expected conflict error in message, got: %s", m.infoScreen.message)
	}
}

func TestExpandWithEnv(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		env      map[string]string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			env:      map[string]string{"FOO": "bar"},
			expected: "",
		},
		{
			name:     "no variables",
			input:    "plain text",
			env:      map[string]string{},
			expected: "plain text",
		},
		{
			name:     "single variable",
			input:    "$FOO",
			env:      map[string]string{"FOO": "bar"},
			expected: "bar",
		},
		{
			name:     "variable with braces",
			input:    "${FOO}",
			env:      map[string]string{"FOO": "bar"},
			expected: "bar",
		},
		{
			name:     "multiple variables",
			input:    "$FOO-$BAR",
			env:      map[string]string{"FOO": "hello", "BAR": "world"},
			expected: "hello-world",
		},
		{
			name:     "REPO_NAME and WORKTREE_NAME",
			input:    "${REPO_NAME}_wt_$WORKTREE_NAME",
			env:      map[string]string{"REPO_NAME": "myrepo", "WORKTREE_NAME": "feature"},
			expected: "myrepo_wt_feature",
		},
		{
			name:     "missing variable uses system env",
			input:    "$HOME",
			env:      map[string]string{},
			expected: os.Getenv("HOME"),
		},
		{
			name:     "undefined variable becomes empty",
			input:    "$UNDEFINED_VAR",
			env:      map[string]string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandWithEnv(tt.input, tt.env)
			if result != tt.expected {
				t.Errorf("expandWithEnv(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestResolveTmuxWindows(t *testing.T) {
	tests := []struct {
		name        string
		windows     []config.TmuxWindow
		env         map[string]string
		defaultCwd  string
		expectOk    bool
		expectCount int
	}{
		{
			name:        "empty windows returns false",
			windows:     []config.TmuxWindow{},
			env:         map[string]string{},
			defaultCwd:  "/default",
			expectOk:    false,
			expectCount: 0,
		},
		{
			name: "single window with name",
			windows: []config.TmuxWindow{
				{Name: "shell", Command: "", Cwd: ""},
			},
			env:         map[string]string{},
			defaultCwd:  "/default",
			expectOk:    true,
			expectCount: 1,
		},
		{
			name: "window with env vars in name",
			windows: []config.TmuxWindow{
				{Name: "$WORKTREE_NAME", Command: "", Cwd: ""},
			},
			env:         map[string]string{"WORKTREE_NAME": "feature"},
			defaultCwd:  "/default",
			expectOk:    true,
			expectCount: 1,
		},
		{
			name: "empty name gets auto-generated",
			windows: []config.TmuxWindow{
				{Name: "", Command: "", Cwd: ""},
			},
			env:         map[string]string{},
			defaultCwd:  "/default",
			expectOk:    true,
			expectCount: 1,
		},
		{
			name: "empty cwd uses default",
			windows: []config.TmuxWindow{
				{Name: "shell", Command: "", Cwd: ""},
			},
			env:         map[string]string{},
			defaultCwd:  "/my/path",
			expectOk:    true,
			expectCount: 1,
		},
		{
			name: "multiple windows",
			windows: []config.TmuxWindow{
				{Name: "shell", Command: "", Cwd: ""},
				{Name: "editor", Command: "vim", Cwd: "/custom"},
			},
			env:         map[string]string{},
			defaultCwd:  "/default",
			expectOk:    true,
			expectCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved, ok := resolveTmuxWindows(tt.windows, tt.env, tt.defaultCwd)
			if ok != tt.expectOk {
				t.Errorf("resolveTmuxWindows ok = %v, want %v", ok, tt.expectOk)
			}
			if len(resolved) != tt.expectCount {
				t.Errorf("resolveTmuxWindows count = %d, want %d", len(resolved), tt.expectCount)
			}

			if tt.expectOk && len(tt.windows) > 0 {
				// Check first window specifically
				w := resolved[0]
				if tt.windows[0].Name == "" && w.Name != "window-1" {
					t.Errorf("expected auto-generated name 'window-1', got %q", w.Name)
				}
				if tt.windows[0].Cwd == "" && w.Cwd != tt.defaultCwd {
					t.Errorf("expected default cwd %q, got %q", tt.defaultCwd, w.Cwd)
				}
			}
		})
	}
}

func TestBuildTmuxWindowCommand(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		env      map[string]string
		expected string
	}{
		{
			name:     "empty command uses default shell",
			command:  "",
			env:      map[string]string{},
			expected: "exec ${SHELL:-bash}",
		},
		{
			name:     "simple command",
			command:  "vim",
			env:      map[string]string{},
			expected: "vim",
		},
		{
			name:     "command with env vars",
			command:  "lazygit",
			env:      map[string]string{"FOO": "bar"},
			expected: "export FOO=bar; lazygit",
		},
		{
			name:     "whitespace-only command uses default shell",
			command:  "   ",
			env:      map[string]string{},
			expected: "exec ${SHELL:-bash}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildTmuxWindowCommand(tt.command, tt.env)
			if !strings.Contains(result, tt.command) && tt.command != "" && strings.TrimSpace(tt.command) != "" {
				t.Errorf("buildTmuxWindowCommand(%q) = %q, should contain command", tt.command, result)
			}
			if tt.command == "" || strings.TrimSpace(tt.command) == "" {
				if !strings.Contains(result, "exec ${SHELL:-bash}") {
					t.Errorf("buildTmuxWindowCommand(%q) = %q, should contain default shell", tt.command, result)
				}
			}
		})
	}
}

func TestBuildTmuxScript(t *testing.T) {
	tests := []struct {
		name        string
		sessionName string
		tmuxCfg     *config.TmuxCommand
		windows     []resolvedTmuxWindow
		env         map[string]string
		checkScript func(t *testing.T, script string)
	}{
		{
			name:        "empty windows returns empty script",
			sessionName: "test",
			tmuxCfg: &config.TmuxCommand{
				OnExists: "switch",
				Attach:   false,
			},
			windows: []resolvedTmuxWindow{},
			env:     map[string]string{},
			checkScript: func(t *testing.T, script string) {
				if script != "" {
					t.Errorf("expected empty script for empty windows, got %q", script)
				}
			},
		},
		{
			name:        "single window script",
			sessionName: "myrepo_wt_feature",
			tmuxCfg: &config.TmuxCommand{
				OnExists: "switch",
				Attach:   false,
			},
			windows: []resolvedTmuxWindow{
				{Name: "shell", Command: "exec ${SHELL:-bash}", Cwd: "/path"},
			},
			env: map[string]string{},
			checkScript: func(t *testing.T, script string) {
				if !strings.Contains(script, "session='myrepo_wt_feature'") {
					t.Error("script should contain session name")
				}
				if !strings.Contains(script, "tmux new-session") {
					t.Error("script should create new session")
				}
				if !strings.Contains(script, "-n 'shell'") {
					t.Error("script should create window named 'shell'")
				}
			},
		},
		{
			name:        "on_exists kill mode",
			sessionName: "test",
			tmuxCfg: &config.TmuxCommand{
				OnExists: "kill",
				Attach:   false,
			},
			windows: []resolvedTmuxWindow{
				{Name: "shell", Command: "exec ${SHELL:-bash}", Cwd: "/path"},
			},
			env: map[string]string{},
			checkScript: func(t *testing.T, script string) {
				if !strings.Contains(script, "tmux kill-session") {
					t.Error("script should kill existing session in kill mode")
				}
			},
		},
		{
			name:        "on_exists new mode",
			sessionName: "test",
			tmuxCfg: &config.TmuxCommand{
				OnExists: "new",
				Attach:   false,
			},
			windows: []resolvedTmuxWindow{
				{Name: "shell", Command: "exec ${SHELL:-bash}", Cwd: "/path"},
			},
			env: map[string]string{},
			checkScript: func(t *testing.T, script string) {
				if !strings.Contains(script, "while tmux has-session") {
					t.Error("script should check for incremented session names in new mode")
				}
			},
		},
		{
			name:        "multiple windows",
			sessionName: "test",
			tmuxCfg: &config.TmuxCommand{
				OnExists: "switch",
				Attach:   false,
			},
			windows: []resolvedTmuxWindow{
				{Name: "shell", Command: "exec ${SHELL:-bash}", Cwd: "/path"},
				{Name: "editor", Command: "vim", Cwd: "/path"},
			},
			env: map[string]string{},
			checkScript: func(t *testing.T, script string) {
				if !strings.Contains(script, "tmux new-window") {
					t.Error("script should create additional windows")
				}
				if !strings.Contains(script, "-n 'editor'") {
					t.Error("script should create window named 'editor'")
				}
			},
		},
		{
			name:        "attach inside tmux",
			sessionName: "test",
			tmuxCfg: &config.TmuxCommand{
				OnExists: "switch",
				Attach:   true,
			},
			windows: []resolvedTmuxWindow{
				{Name: "shell", Command: "exec ${SHELL:-bash}", Cwd: "/path"},
			},
			env: map[string]string{},
			checkScript: func(t *testing.T, script string) {
				if !strings.Contains(script, "tmux switch-client") {
					t.Error("script should switch client when inside tmux")
				}
			},
		},
		{
			name:        "env vars in script",
			sessionName: "test",
			tmuxCfg: &config.TmuxCommand{
				OnExists: "switch",
				Attach:   false,
			},
			windows: []resolvedTmuxWindow{
				{Name: "shell", Command: "exec ${SHELL:-bash}", Cwd: "/path"},
			},
			env: map[string]string{"REPO_NAME": "myrepo", "WORKTREE_NAME": "feature"},
			checkScript: func(t *testing.T, script string) {
				if !strings.Contains(script, "tmux set-environment") {
					t.Error("script should set environment variables")
				}
				if !strings.Contains(script, "REPO_NAME") || !strings.Contains(script, "WORKTREE_NAME") {
					t.Error("script should include REPO_NAME and WORKTREE_NAME env vars")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script := buildTmuxScript(tt.sessionName, tt.tmuxCfg, tt.windows, tt.env)
			tt.checkScript(t, script)
		})
	}
}
