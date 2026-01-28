package app

import (
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/app/state"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/theme"
	"github.com/chmouel/lazyworktree/internal/utils"
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

func TestAIBranchNameSanitization(t *testing.T) {
	tests := []struct {
		name     string
		aiName   string
		expected string
	}{
		{
			name:     "slash in name",
			aiName:   "feature/fix-bug",
			expected: "feature-fix-bug",
		},
		{
			name:     "multiple slashes",
			aiName:   "user/feature/new",
			expected: "user-feature-new",
		},
		{
			name:     "special characters",
			aiName:   "Fix: Add Support!",
			expected: "fix-add-support",
		},
		{
			name:     "spaces",
			aiName:   "my new branch",
			expected: "my-new-branch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.AppConfig{
				WorktreeDir: t.TempDir(),
			}
			m := NewModel(cfg, "")

			m.createFromCurrentRandomName = testFallback

			// Simulate AI name generation message
			msg := aiBranchNameGeneratedMsg{
				name: tt.aiName,
				err:  nil,
			}

			// Setup input screen
			m.inputScreen = NewInputScreen("test", "placeholder", "initial", m.theme, m.config.IconsEnabled())
			m.inputScreen.SetCheckbox("Include changes", true)

			// Handle the AI name generation
			updated, _ := m.Update(msg)
			m = updated.(*Model)

			// Check that the AI name was sanitized
			if !strings.Contains(m.createFromCurrentAIName, tt.expected) {
				t.Errorf("expected sanitized name to contain %q, got %q", tt.expected, m.createFromCurrentAIName)
			}
			if strings.Contains(m.createFromCurrentAIName, "/") {
				t.Errorf("sanitized name should not contain slashes, got %q", m.createFromCurrentAIName)
			}
		})
	}
}

func TestCacheCleanupOnSubmit(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.worktrees = []*models.WorktreeInfo{
		{Path: "/tmp/main", Branch: mainWorktreeName, IsMain: true},
	}

	// Setup cached state
	m.createFromCurrentDiff = testDiff
	m.createFromCurrentRandomName = testRandomName
	m.createFromCurrentBranch = mainWorktreeName
	m.createFromCurrentAIName = "ai-cached"

	msg := createFromCurrentReadyMsg{
		currentWorktree:   &models.WorktreeInfo{Path: "/tmp/main", Branch: mainWorktreeName},
		currentBranch:     mainWorktreeName,
		diff:              testDiff,
		hasChanges:        true,
		defaultBranchName: testRandomName,
	}

	m.handleCreateFromCurrentReady(msg)

	if m.inputSubmit == nil {
		t.Fatal("inputSubmit callback should be set")
	}

	// Call inputSubmit (which should clear cache)
	// Note: This will fail validation because branch doesn't exist in git, but cache should still be cleared
	m.inputSubmit("new-branch-test", false)

	// Verify cache is cleared
	if m.createFromCurrentDiff != "" {
		t.Errorf("expected diff cache to be cleared, got %q", m.createFromCurrentDiff)
	}
	if m.createFromCurrentRandomName != "" {
		t.Errorf("expected random name cache to be cleared, got %q", m.createFromCurrentRandomName)
	}
	if m.createFromCurrentAIName != "" {
		t.Errorf("expected AI name cache to be cleared, got %q", m.createFromCurrentAIName)
	}
	if m.createFromCurrentBranch != "" {
		t.Errorf("expected branch cache to be cleared, got %q", m.createFromCurrentBranch)
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
	// Skip this test if tmux is not available
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not available in test environment")
	}

	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		CustomCommands: map[string]*config.CustomCommand{
			"t": {
				Description: "Tmux",
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
			if item.label != "Tmux (t)" {
				t.Errorf("Expected label 'Tmux (t)', got %q", item.label)
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

func TestShowCommandPaletteIncludesZellijCommands(t *testing.T) {
	// Skip this test if zellij is not available
	if _, err := exec.LookPath("zellij"); err != nil {
		t.Skip("zellij not available in test environment")
	}

	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		CustomCommands: map[string]*config.CustomCommand{
			"Z": {
				Description: "Zellij",
				ShowHelp:    true,
				Zellij: &config.TmuxCommand{
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
		if item.id == "Z" {
			found = true
			if item.label != "Zellij (Z)" {
				t.Errorf("Expected label 'Zellij (Z)', got %q", item.label)
			}
			if item.description != zellijSessionLabel {
				t.Errorf("Expected description %q, got %q", zellijSessionLabel, item.description)
			}
			break
		}
	}
	if !found {
		t.Fatal("zellij command item not found in command palette")
	}
}

func TestShowCommandPaletteHasSectionHeaders(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.showCommandPalette()

	sectionCount := 0
	for _, item := range m.paletteScreen.items {
		if item.isSection {
			sectionCount++
		}
	}

	if sectionCount != 7 {
		t.Errorf("expected 7 sections, got %d", sectionCount)
	}
}

func TestShowCommandPaletteFirstItemIsSection(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.showCommandPalette()

	if !m.paletteScreen.items[0].isSection {
		t.Error("expected first item to be a section header")
	}
	if m.paletteScreen.items[0].label != "Worktree Actions" {
		t.Errorf("expected first section 'Worktree Actions', got %q", m.paletteScreen.items[0].label)
	}
}

func TestShowCommandPaletteHasAllActions(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.showCommandPalette()

	expectedIDs := []string{
		"create", "delete", "rename", "absorb", "prune",
		"create-from-current", "create-from-branch", "create-from-commit",
		"create-from-pr", "create-from-issue", "create-freeform",
		"diff", "refresh", "fetch", "push", "sync", "fetch-pr-data", "pr", "lazygit", "run-command",
		"stage-file", "commit-staged", "commit-all", "edit-file", "delete-file",
		"cherry-pick", "commit-view",
		"zoom-toggle", "filter", "search", "focus-worktrees", "focus-status", "focus-log", "sort-cycle",
		"theme", "help",
	}

	itemIDs := make(map[string]bool)
	for _, item := range m.paletteScreen.items {
		if !item.isSection {
			itemIDs[item.id] = true
		}
	}

	for _, expectedID := range expectedIDs {
		if !itemIDs[expectedID] {
			t.Errorf("expected palette item %q not found", expectedID)
		}
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
	m.view.WindowWidth = 200
	m.view.WindowHeight = 50
	layout := m.computeLayout()
	footer := m.renderFooter(layout)

	if !strings.Contains(footer, "Run tests") {
		t.Fatalf("expected footer to include custom command label, got %q", footer)
	}
}

func TestUpdateTheme(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		Theme:       "dracula",
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)

	// Verify initial theme (Dracula accent is #BD93F9)
	if string(m.theme.Accent) != "#BD93F9" {
		t.Fatalf("expected initial dracula accent, got %v", m.theme.Accent)
	}

	// Update to clean-light (Clean-Light accent is #c6dbe5)
	m.UpdateTheme("clean-light")
	if string(m.theme.Accent) != "#c6dbe5" {
		t.Fatalf("expected clean-light accent, got %v", m.theme.Accent)
	}
}

func TestShowThemeSelection(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		IconSet:     "nerd-font-v3",
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)

	cmd := m.showThemeSelection()
	if cmd == nil {
		t.Fatal("showThemeSelection returned nil command")
	}

	if !m.screenManager.IsActive() {
		t.Fatal("expected screen manager to have active screen")
	}

	if m.screenManager.Type() != appscreen.TypeListSelect {
		t.Fatalf("expected list selection screen, got %v", m.screenManager.Type())
	}

	listScreen := m.screenManager.Current().(*appscreen.ListSelectionScreen)
	if listScreen == nil {
		t.Fatal("listScreen should be initialized")
	}

	expectedTitle := labelWithIcon(UIIconThemeSelect, "Select Theme", m.config.IconsEnabled())
	if listScreen.Title != expectedTitle {
		t.Fatalf("expected title %q, got %q", expectedTitle, listScreen.Title)
	}

	// Verify all themes are present
	available := theme.AvailableThemes()
	if len(listScreen.Items) != len(available) {
		t.Fatalf("expected %d themes in list, got %d", len(available), len(listScreen.Items))
	}
}

func TestRandomBranchName(t *testing.T) {
	name := utils.RandomBranchName()
	if name == "" {
		t.Fatal("expected non-empty random branch name")
	}
	parts := strings.Split(name, "-")
	if len(parts) != 2 {
		t.Fatalf("expected format 'adjective-noun' with a single hyphen, got %q", name)
	}
	// Verify both parts are non-empty alphabetic strings
	for i, part := range parts {
		if part == "" {
			t.Fatalf("part %d is empty in %q", i, name)
		}
		for _, c := range part {
			if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') {
				t.Fatalf("part %d contains non-alphabetic character in %q", i, name)
			}
		}
	}
}

func TestCommandPaletteMRUDeduplication(t *testing.T) {
	m := &Model{
		config: &config.AppConfig{
			PaletteMRU:      true,
			PaletteMRULimit: 5,
		},
		paletteHistory: []commandPaletteUsage{
			{ID: "refresh", Timestamp: time.Now().Unix(), Count: 5},
			{ID: "create", Timestamp: time.Now().Unix() - 100, Count: 3},
			{ID: "diff", Timestamp: time.Now().Unix() - 200, Count: 2},
		},
		view: &state.ViewState{
			WindowWidth:  100,
			WindowHeight: 50,
		},
	}

	cmd := m.showCommandPalette()
	if cmd == nil {
		t.Errorf("showCommandPalette should not return nil, got %v", cmd)
	}

	if m.paletteScreen == nil {
		t.Fatal("paletteScreen should be set")
	}

	items := m.paletteScreen.items

	// Check that MRU section exists and is first
	if len(items) == 0 {
		t.Fatal("palette should have items")
	}

	if !items[0].isSection || items[0].label != mruSectionLabel {
		t.Errorf("first item should be 'Recently Used' section, got %+v", items[0])
	}

	// Count occurrences of MRU items
	refreshCount := 0
	createCount := 0
	diffCount := 0
	inMRUSection := false

	for i, item := range items {
		if item.isSection {
			if item.label == mruSectionLabel {
				inMRUSection = true
			} else {
				inMRUSection = false
			}
			continue
		}

		if item.id == testCommandRefresh {
			refreshCount++
			if !inMRUSection {
				t.Errorf("'refresh' found outside MRU section at index %d", i)
			}
		}
		if item.id == testCommandCreate {
			createCount++
			if !inMRUSection {
				t.Errorf("'create' found outside MRU section at index %d", i)
			}
		}
		if item.id == "diff" {
			diffCount++
			if !inMRUSection {
				t.Errorf("'diff' found outside MRU section at index %d", i)
			}
		}
	}

	// Each MRU item should appear exactly once (only in MRU section)
	if refreshCount != 1 {
		t.Errorf("'refresh' should appear exactly once, found %d times", refreshCount)
	}
	if createCount != 1 {
		t.Errorf("'create' should appear exactly once, found %d times", createCount)
	}
	if diffCount != 1 {
		t.Errorf("'diff' should appear exactly once, found %d times", diffCount)
	}
}

func TestCommandPaletteMRUDisabled(t *testing.T) {
	m := &Model{
		config: &config.AppConfig{
			PaletteMRU:      false,
			PaletteMRULimit: 5,
		},
		paletteHistory: []commandPaletteUsage{
			{ID: "refresh", Timestamp: time.Now().Unix(), Count: 5},
		},
		view: &state.ViewState{
			WindowWidth:  100,
			WindowHeight: 50,
		},
	}

	cmd := m.showCommandPalette()
	if cmd == nil {
		t.Errorf("showCommandPalette should not return nil, got %v", cmd)
	}

	items := m.paletteScreen.items

	// Should NOT have MRU section when disabled
	for _, item := range items {
		if item.isSection && item.label == mruSectionLabel {
			t.Error("MRU section should not appear when palette_mru is false")
		}
	}

	// Items should appear in their original sections
	refreshCount := 0
	for _, item := range items {
		if item.id == testCommandRefresh {
			refreshCount++
		}
	}

	if refreshCount != 1 {
		t.Errorf("'refresh' should appear exactly once in original section, found %d times", refreshCount)
	}
}

func TestCommandPaletteMRUEmptyHistory(t *testing.T) {
	m := &Model{
		config: &config.AppConfig{
			PaletteMRU:      true,
			PaletteMRULimit: 5,
		},
		paletteHistory: []commandPaletteUsage{},
		view: &state.ViewState{
			WindowWidth:  100,
			WindowHeight: 50,
		},
	}

	cmd := m.showCommandPalette()
	if cmd == nil {
		t.Errorf("showCommandPalette should not return nil, got %v", cmd)
	}

	items := m.paletteScreen.items

	// Should NOT have MRU section when history is empty
	for _, item := range items {
		if item.isSection && item.label == mruSectionLabel {
			t.Error("MRU section should not appear when history is empty")
		}
	}
}

func TestFilterPaletteItemsSkipsMRU(t *testing.T) {
	items := []paletteItem{
		{label: mruSectionLabel, isSection: true},
		{id: "refresh", label: "Refresh (r)", description: "Reload worktrees", isMRU: true},
		{label: "Git Operations", isSection: true},
		{id: "refresh", label: "Refresh (r)", description: "Reload worktrees"},
		{id: "diff", label: "Show diff (d)", description: "Show diff"},
	}

	filtered := filterPaletteItems(items, "ref")

	// Should skip MRU items and sections during filtering
	if len(filtered) != 1 {
		t.Errorf("expected 1 filtered item (refresh from Git Operations), got %d", len(filtered))
		for i, item := range filtered {
			t.Logf("  [%d] id=%s label=%s isMRU=%v isSection=%v", i, item.id, item.label, item.isMRU, item.isSection)
		}
	}

	if len(filtered) > 0 && filtered[0].isMRU {
		t.Error("filtered item should not be marked as MRU")
	}
}

func TestShowCherryPickNotInLogPane(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.view.FocusedPane = 0 // Not in log pane

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
	m.view.FocusedPane = 2 // Log pane
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
	m.view.FocusedPane = 2 // Log pane
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
	m.view.FocusedPane = 2 // Log pane
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
	if !m.screenManager.IsActive() {
		t.Error("Expected screen manager to have active screen")
	}
	if m.screenManager.Type() != appscreen.TypeListSelect {
		t.Errorf("Expected list selection screen, got %v", m.screenManager.Type())
	}
	listScreen := m.screenManager.Current().(*appscreen.ListSelectionScreen)
	if listScreen == nil {
		t.Fatal("Expected listScreen to be set")
	}
	if !strings.Contains(listScreen.Title, "Cherry-pick") {
		t.Errorf("Expected cherry-pick in title, got: %s", listScreen.Title)
	}
	// Should exclude source worktree
	if len(listScreen.Items) != 1 {
		t.Errorf("Expected 1 target worktree (excluding source), got %d", len(listScreen.Items))
	}
}

func TestShowCherryPickExcludesSourceWorktree(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.view.FocusedPane = 2
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

	listScreen := m.screenManager.Current().(*appscreen.ListSelectionScreen)
	// Should have 2 items (main + feature2, excluding feature1)
	if len(listScreen.Items) != 2 {
		t.Errorf("Expected 2 target worktrees, got %d", len(listScreen.Items))
	}

	// Verify feature1 is not in the list
	for _, item := range listScreen.Items {
		if item.ID == "/path/to/feature1" {
			t.Error("Source worktree should be excluded from selection list")
		}
	}
}

func TestShowCherryPickMarksDirtyWorktrees(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.view.FocusedPane = 2
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

	listScreen := m.screenManager.Current().(*appscreen.ListSelectionScreen)
	// Find the dirty worktree item
	var dirtyItem *appscreen.SelectionItem
	for i := range listScreen.Items {
		if listScreen.Items[i].ID == "/path/to/dirty" {
			dirtyItem = &listScreen.Items[i]
			break
		}
	}

	if dirtyItem == nil {
		t.Fatal("Expected dirty worktree in selection list")
	}

	if !strings.Contains(dirtyItem.Description, "(has changes)") {
		t.Errorf("Expected '(has changes)' marker in description, got: %s", dirtyItem.Description)
	}
}

func TestRenderScreenVariants(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)

	// CommitScreen is now managed by screenManager
	commitScr := appscreen.NewCommitScreen(appscreen.CommitMeta{SHA: "abc123"}, "stat", "diff", false, m.theme)
	m.screenManager.Push(commitScr)
	out := m.View()
	if out == "" {
		t.Fatal("expected commit screen to render")
	}
	m.screenManager.Pop()

	m.confirmScreen = NewConfirmScreen("Confirm?", m.theme)
	m.currentScreen = screenConfirm
	if out = m.renderScreen(); out == "" {
		t.Fatal("expected confirm screen to render")
	}

	m.infoScreen = NewInfoScreen("Info", m.theme)
	m.currentScreen = screenInfo
	if out = m.renderScreen(); out == "" {
		t.Fatal("expected info screen to render")
	}

	// TrustScreen is now managed by screenManager
	trustScr := appscreen.NewTrustScreen("/tmp/.wt.yaml", []string{"cmd"}, m.theme)
	m.screenManager.Push(trustScr)
	if out = m.View(); out == "" {
		t.Fatal("expected trust screen to render")
	}
	m.screenManager.Pop()

	// WelcomeScreen is now managed by screenManager
	welcomeScr := appscreen.NewWelcomeScreen("/tmp", "/tmp/wt", m.theme)
	m.screenManager.Push(welcomeScr)
	if out = m.View(); out == "" {
		t.Fatal("expected welcome screen to render")
	}
	m.screenManager.Pop()

	m.paletteScreen = NewCommandPaletteScreen([]paletteItem{{id: "help", label: "Help"}}, 100, 40, m.theme)
	m.currentScreen = screenPalette
	if out = m.renderScreen(); out == "" {
		t.Fatal("expected palette screen to render")
	}

	if out = m.renderScreen(); out == "" {
		t.Fatal("expected diff screen to render")
	}

	m.inputScreen = NewInputScreen("Prompt", "Placeholder", "value", m.theme, m.config.IconsEnabled())
	m.currentScreen = screenInput
	if out = m.renderScreen(); out == "" {
		t.Fatal("expected input screen to render")
	}

	listScreen := appscreen.NewListSelectionScreen([]appscreen.SelectionItem{{ID: "a", Label: "A"}}, "Select", "", "", 120, 40, "", m.theme)
	m.screenManager.Push(listScreen)
	if out = m.renderScreen(); out == "" {
		t.Fatal("expected list selection screen to render")
	}
}

func TestErrMsgShowsInfo(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	_, _ = m.Update(errMsg{err: errors.New("boom")})

	if m.currentScreen != screenInfo {
		t.Fatalf("expected info screen, got %v", m.currentScreen)
	}
	if m.infoScreen == nil || !strings.Contains(m.infoScreen.message, "boom") {
		t.Fatalf("expected info modal to include error, got %#v", m.infoScreen)
	}
}
