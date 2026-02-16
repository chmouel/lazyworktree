package app

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/theme"
	"github.com/chmouel/lazyworktree/internal/utils"
)

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

			m.createFromCurrent.randomName = testFallback

			// Simulate AI name generation message
			msg := aiBranchNameGeneratedMsg{
				name: tt.aiName,
				err:  nil,
			}

			// Setup input screen
			inputScr := appscreen.NewInputScreen("test", "placeholder", "initial", m.theme, m.config.IconsEnabled())
			inputScr.SetCheckbox("Include changes", true)
			m.createFromCurrent.inputScreen = inputScr

			// Handle the AI name generation
			updated, _ := m.Update(msg)
			m = updated.(*Model)

			// Check that the AI name was sanitized
			if !strings.Contains(m.createFromCurrent.aiName, tt.expected) {
				t.Errorf("expected sanitized name to contain %q, got %q", tt.expected, m.createFromCurrent.aiName)
			}
			if strings.Contains(m.createFromCurrent.aiName, "/") {
				t.Errorf("sanitized name should not contain slashes, got %q", m.createFromCurrent.aiName)
			}
		})
	}
}

func TestCacheCleanupOnSubmit(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: "/tmp/main", Branch: mainWorktreeName, IsMain: true},
	}

	// Setup cached state
	m.createFromCurrent.diff = testDiff
	m.createFromCurrent.randomName = testRandomName
	m.createFromCurrent.branch = mainWorktreeName
	m.createFromCurrent.aiName = "ai-cached"

	msg := createFromCurrentReadyMsg{
		currentWorktree:   &models.WorktreeInfo{Path: "/tmp/main", Branch: mainWorktreeName},
		currentBranch:     mainWorktreeName,
		diff:              testDiff,
		hasChanges:        true,
		defaultBranchName: testRandomName,
	}

	m.handleCreateFromCurrentReady(msg)

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInput {
		t.Fatal("input screen should be active")
	}
	inputScr := m.state.ui.screenManager.Current().(*appscreen.InputScreen)
	if inputScr.OnSubmit == nil {
		t.Fatal("OnSubmit callback should be set")
	}

	// Call OnSubmit (which should clear cache)
	// Note: This will fail validation because branch doesn't exist in git, but cache should still be cleared
	inputScr.OnSubmit("new-branch-test", false)

	// Verify cache is cleared
	if m.createFromCurrent.diff != "" {
		t.Errorf("expected diff cache to be cleared, got %q", m.createFromCurrent.diff)
	}
	if m.createFromCurrent.randomName != "" {
		t.Errorf("expected random name cache to be cleared, got %q", m.createFromCurrent.randomName)
	}
	if m.createFromCurrent.aiName != "" {
		t.Errorf("expected AI name cache to be cleared, got %q", m.createFromCurrent.aiName)
	}
	if m.createFromCurrent.branch != "" {
		t.Errorf("expected branch cache to be cleared, got %q", m.createFromCurrent.branch)
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
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInput {
		t.Fatalf("expected input screen active, got type %v", m.state.ui.screenManager.Type())
	}
	inputScr, ok := m.state.ui.screenManager.Current().(*appscreen.InputScreen)
	if !ok {
		t.Fatal("expected InputScreen")
	}
	got := inputScr.Input.Value()
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
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypePalette {
		t.Fatal("expected command palette screen")
	}

	paletteScreen := m.state.ui.screenManager.Current().(*appscreen.CommandPaletteScreen)
	items := paletteScreen.Items
	found := false
	for _, item := range items {
		if item.ID == "x" {
			found = true
			if item.Label != "Run tests (x)" {
				t.Errorf("Expected label 'Run tests (x)', got %q", item.Label)
			}
			if item.Description != "make test" {
				t.Errorf("Expected description 'make test', got %q", item.Description)
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
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypePalette {
		t.Fatal("expected command palette screen")
	}

	paletteScreen := m.state.ui.screenManager.Current().(*appscreen.CommandPaletteScreen)
	items := paletteScreen.Items
	found := false
	for _, item := range items {
		if item.ID == "t" {
			found = true
			if item.Label != "Tmux (t)" {
				t.Errorf("Expected label 'Tmux (t)', got %q", item.Label)
			}
			if item.Description != tmuxSessionLabel {
				t.Errorf("Expected description %q, got %q", tmuxSessionLabel, item.Description)
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
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypePalette {
		t.Fatal("expected command palette screen")
	}

	paletteScreen := m.state.ui.screenManager.Current().(*appscreen.CommandPaletteScreen)
	items := paletteScreen.Items
	found := false
	for _, item := range items {
		if item.ID == "Z" {
			found = true
			if item.Label != "Zellij (Z)" {
				t.Errorf("Expected label 'Zellij (Z)', got %q", item.Label)
			}
			if item.Description != zellijSessionLabel {
				t.Errorf("Expected description %q, got %q", zellijSessionLabel, item.Description)
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

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypePalette {
		t.Fatal("expected command palette screen")
	}

	paletteScreen := m.state.ui.screenManager.Current().(*appscreen.CommandPaletteScreen)
	sectionCount := 0
	for _, item := range paletteScreen.Items {
		if item.IsSection {
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

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypePalette {
		t.Fatal("expected command palette screen")
	}

	paletteScreen := m.state.ui.screenManager.Current().(*appscreen.CommandPaletteScreen)
	if !paletteScreen.Items[0].IsSection {
		t.Error("expected first item to be a section header")
	}
	if paletteScreen.Items[0].Label != "Worktree Actions" {
		t.Errorf("expected first section 'Worktree Actions', got %q", paletteScreen.Items[0].Label)
	}
}

func TestShowCommandPaletteHasAllActions(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.showCommandPalette()

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypePalette {
		t.Fatal("expected command palette screen")
	}

	expectedIDs := []string{
		"create", "delete", "rename", "annotate", "absorb", "prune",
		"create-from-current", "create-from-branch", "create-from-commit",
		"create-from-pr", "create-from-issue", "create-freeform",
		"diff", "refresh", "fetch", "push", "sync", "fetch-pr-data", "pr", "lazygit", "run-command",
		"stage-file", "commit-staged", "commit-all", "edit-file", "delete-file",
		"cherry-pick", "commit-view",
		"zoom-toggle", "filter", "search", "focus-worktrees", "focus-status", "focus-log", "sort-cycle",
		"theme", "help",
	}

	paletteScreen := m.state.ui.screenManager.Current().(*appscreen.CommandPaletteScreen)
	itemIDs := make(map[string]bool)
	for _, item := range paletteScreen.Items {
		if !item.IsSection {
			itemIDs[item.ID] = true
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
	m.state.view.WindowWidth = 200
	m.state.view.WindowHeight = 50
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

	if !m.state.ui.screenManager.IsActive() {
		t.Fatal("expected screen manager to have active screen")
	}

	if m.state.ui.screenManager.Type() != appscreen.TypeListSelect {
		t.Fatalf("expected list selection screen, got %v", m.state.ui.screenManager.Type())
	}

	listScreen, ok := m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)
	if !ok || listScreen == nil {
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

func findSelectionItemByID(items []appscreen.SelectionItem, id string) (appscreen.SelectionItem, bool) {
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return appscreen.SelectionItem{}, false
}

func TestThemeSelectionCancelSaveKeepsAppliedThemeAndClosesFlow(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		Theme:       "dracula",
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)

	m.showThemeSelection()
	listScreen, ok := m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)
	if !ok || listScreen == nil {
		t.Fatal("expected list selection screen")
	}

	item, found := findSelectionItemByID(listScreen.Items, "clean-light")
	if !found {
		t.Fatal("expected clean-light in theme selection list")
	}
	if listScreen.OnCursorChange == nil {
		t.Fatal("expected OnCursorChange callback")
	}
	listScreen.OnCursorChange(item)

	if string(m.theme.Accent) != "#c6dbe5" {
		t.Fatalf("expected clean-light accent after preview, got %v", m.theme.Accent)
	}

	if listScreen.OnSelect == nil {
		t.Fatal("expected OnSelect callback")
	}
	listScreen.OnSelect(item)

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeConfirm {
		t.Fatalf("expected confirm screen, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}

	_, _ = m.handleScreenKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})

	if m.config.Theme != "dracula" {
		t.Fatalf("expected config theme to remain dracula, got %q", m.config.Theme)
	}
	if string(m.theme.Accent) != "#c6dbe5" {
		t.Fatalf("expected selected theme to stay applied, got accent %v", m.theme.Accent)
	}
	if m.state.ui.screenManager.IsActive() {
		t.Fatalf("expected theme flow to close, got active screen type %v", m.state.ui.screenManager.Type())
	}
}

func TestThemeSelectionListCancelRestoresOriginalTheme(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		Theme:       "dracula",
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)

	m.showThemeSelection()
	listScreen, ok := m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)
	if !ok || listScreen == nil {
		t.Fatal("expected list selection screen")
	}

	item, found := findSelectionItemByID(listScreen.Items, "clean-light")
	if !found {
		t.Fatal("expected clean-light in theme selection list")
	}
	listScreen.OnCursorChange(item)
	if string(m.theme.Accent) != "#c6dbe5" {
		t.Fatalf("expected clean-light accent after preview, got %v", m.theme.Accent)
	}

	_, _ = m.handleScreenKey(tea.KeyMsg{Type: tea.KeyEsc})

	if string(m.theme.Accent) != "#BD93F9" {
		t.Fatalf("expected original dracula theme restored, got accent %v", m.theme.Accent)
	}
	if m.state.ui.screenManager.IsActive() {
		t.Fatalf("expected list screen to close, got type %v", m.state.ui.screenManager.Type())
	}
}

func TestThemeSelectionConfirmSavePersistsTheme(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		Theme:       "dracula",
		ConfigPath:  filepath.Join(t.TempDir(), "config.yaml"),
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)

	m.showThemeSelection()
	listScreen, ok := m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)
	if !ok || listScreen == nil {
		t.Fatal("expected list selection screen")
	}

	item, found := findSelectionItemByID(listScreen.Items, "clean-light")
	if !found {
		t.Fatal("expected clean-light in theme selection list")
	}
	listScreen.OnCursorChange(item)
	listScreen.OnSelect(item)

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeConfirm {
		t.Fatalf("expected confirm screen, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}

	_, _ = m.handleScreenKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})

	if m.config.Theme != "clean-light" {
		t.Fatalf("expected config theme to persist as clean-light, got %q", m.config.Theme)
	}
	if string(m.theme.Accent) != "#c6dbe5" {
		t.Fatalf("expected clean-light to stay applied, got accent %v", m.theme.Accent)
	}
	if m.originalTheme != "" {
		t.Fatalf("expected originalTheme to be cleared, got %q", m.originalTheme)
	}
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeListSelect {
		t.Fatalf("expected to return to list selection screen, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}

	_, _ = m.handleScreenKey(tea.KeyMsg{Type: tea.KeyEsc})
	if string(m.theme.Accent) != "#c6dbe5" {
		t.Fatalf("expected persisted theme to remain after exiting picker, got accent %v", m.theme.Accent)
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
	cfg := &config.AppConfig{
		WorktreeDir:     t.TempDir(),
		PaletteMRU:      true,
		PaletteMRULimit: 5,
	}
	m := NewModel(cfg, "")
	m.paletteHistory = []commandPaletteUsage{
		{ID: "refresh", Timestamp: time.Now().Unix(), Count: 5},
		{ID: "create", Timestamp: time.Now().Unix() - 100, Count: 3},
		{ID: "diff", Timestamp: time.Now().Unix() - 200, Count: 2},
	}
	m.state.view.WindowWidth = 100
	m.state.view.WindowHeight = 50

	cmd := m.showCommandPalette()
	if cmd == nil {
		t.Errorf("showCommandPalette should not return nil, got %v", cmd)
	}

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypePalette {
		t.Fatal("expected command palette screen")
	}

	paletteScreen := m.state.ui.screenManager.Current().(*appscreen.CommandPaletteScreen)
	items := paletteScreen.Items

	// Check that MRU section exists and is first
	if len(items) == 0 {
		t.Fatal("palette should have items")
	}

	if !items[0].IsSection || items[0].Label != mruSectionLabel {
		t.Errorf("first item should be 'Recently Used' section, got %+v", items[0])
	}

	// Count occurrences of MRU items
	refreshCount := 0
	createCount := 0
	diffCount := 0
	inMRUSection := false

	for i, item := range items {
		if item.IsSection {
			if item.Label == mruSectionLabel {
				inMRUSection = true
			} else {
				inMRUSection = false
			}
			continue
		}

		if item.ID == testCommandRefresh {
			refreshCount++
			if !inMRUSection {
				t.Errorf("'refresh' found outside MRU section at index %d", i)
			}
		}
		if item.ID == testCommandCreate {
			createCount++
			if !inMRUSection {
				t.Errorf("'create' found outside MRU section at index %d", i)
			}
		}
		if item.ID == "diff" {
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
	cfg := &config.AppConfig{
		WorktreeDir:     t.TempDir(),
		PaletteMRU:      false,
		PaletteMRULimit: 5,
	}
	m := NewModel(cfg, "")
	m.paletteHistory = []commandPaletteUsage{
		{ID: "refresh", Timestamp: time.Now().Unix(), Count: 5},
	}
	m.state.view.WindowWidth = 100
	m.state.view.WindowHeight = 50

	cmd := m.showCommandPalette()
	if cmd == nil {
		t.Errorf("showCommandPalette should not return nil, got %v", cmd)
	}

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypePalette {
		t.Fatal("expected command palette screen")
	}

	paletteScreen := m.state.ui.screenManager.Current().(*appscreen.CommandPaletteScreen)
	items := paletteScreen.Items

	// Should NOT have MRU section when disabled
	for _, item := range items {
		if item.IsSection && item.Label == mruSectionLabel {
			t.Error("MRU section should not appear when palette_mru is false")
		}
	}

	// Items should appear in their original sections
	refreshCount := 0
	for _, item := range items {
		if item.ID == testCommandRefresh {
			refreshCount++
		}
	}

	if refreshCount != 1 {
		t.Errorf("'refresh' should appear exactly once in original section, found %d times", refreshCount)
	}
}

func TestCommandPaletteMRUEmptyHistory(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir:     t.TempDir(),
		PaletteMRU:      true,
		PaletteMRULimit: 5,
	}
	m := NewModel(cfg, "")
	m.paletteHistory = []commandPaletteUsage{}
	m.state.view.WindowWidth = 100
	m.state.view.WindowHeight = 50

	cmd := m.showCommandPalette()
	if cmd == nil {
		t.Errorf("showCommandPalette should not return nil, got %v", cmd)
	}

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypePalette {
		t.Fatal("expected command palette screen")
	}

	paletteScreen := m.state.ui.screenManager.Current().(*appscreen.CommandPaletteScreen)
	items := paletteScreen.Items

	// Should NOT have MRU section when history is empty
	for _, item := range items {
		if item.IsSection && item.Label == mruSectionLabel {
			t.Error("MRU section should not appear when history is empty")
		}
	}
}

func TestShowCherryPickNotInLogPane(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 0 // Not in log pane

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
	m.state.view.FocusedPane = 2 // Log pane
	m.state.data.logEntries = []commitLogEntry{}

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
	m.state.view.FocusedPane = 2 // Log pane
	m.state.data.logEntries = []commitLogEntry{
		{sha: "abc1234", message: "Test commit"},
	}
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: "/path/to/main", Branch: "main", IsMain: true},
	}
	m.state.data.filteredWts = m.state.data.worktrees
	m.state.data.selectedIndex = 0

	m.showCherryPick()
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Error("Expected info screen to be shown")
	}
	infoScr := m.state.ui.screenManager.Current().(*appscreen.InfoScreen)
	if !strings.Contains(infoScr.Message, "No other worktrees available") {
		t.Errorf("Expected info message about no worktrees, got: %v", infoScr.Message)
	}
}

func TestShowCherryPickCreatesListSelection(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 2 // Log pane
	m.state.data.logEntries = []commitLogEntry{
		{sha: "abc1234", message: "Test commit"},
	}
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: "/path/to/main", Branch: "main", IsMain: true},
		{Path: "/path/to/feature", Branch: "feature", IsMain: false},
	}
	m.state.data.filteredWts = m.state.data.worktrees
	m.state.data.selectedIndex = 0

	m.showCherryPick()
	if !m.state.ui.screenManager.IsActive() {
		t.Error("Expected screen manager to have active screen")
	}
	if m.state.ui.screenManager.Type() != appscreen.TypeListSelect {
		t.Errorf("Expected list selection screen, got %v", m.state.ui.screenManager.Type())
	}
	listScreen, ok := m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)
	if !ok || listScreen == nil {
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
	m.state.view.FocusedPane = 2
	m.state.data.logEntries = []commitLogEntry{
		{sha: "abc1234", message: "Test commit"},
	}
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: "/path/to/main", Branch: "main", IsMain: true},
		{Path: "/path/to/feature1", Branch: "feature1", IsMain: false},
		{Path: "/path/to/feature2", Branch: "feature2", IsMain: false},
	}
	m.state.data.filteredWts = m.state.data.worktrees
	m.state.data.selectedIndex = 1 // Select feature1

	m.showCherryPick()

	listScreen := m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)
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
	m.state.view.FocusedPane = 2
	m.state.data.logEntries = []commitLogEntry{
		{sha: "abc1234", message: "Test commit"},
	}
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: "/path/to/main", Branch: "main", IsMain: true},
		{Path: "/path/to/dirty", Branch: "dirty", IsMain: false, Dirty: true},
	}
	m.state.data.filteredWts = m.state.data.worktrees
	m.state.data.selectedIndex = 0

	m.showCherryPick()

	listScreen, ok := m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)
	if !ok || listScreen == nil {
		t.Fatal("Expected listScreen to be set")
	}
	// Find the dirty worktree item
	var dirtyItem appscreen.SelectionItem
	found := false
	for i := range listScreen.Items {
		if listScreen.Items[i].ID == "/path/to/dirty" {
			dirtyItem = listScreen.Items[i]
			found = true
			break
		}
	}

	if !found {
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
	m.state.ui.screenManager.Push(commitScr)
	out := m.View()
	if out == "" {
		t.Fatal("expected commit screen to render")
	}
	m.state.ui.screenManager.Pop()

	confirmScr := appscreen.NewConfirmScreen("Confirm?", m.theme)
	m.state.ui.screenManager.Push(confirmScr)
	if out = m.View(); out == "" {
		t.Fatal("expected confirm screen to render")
	}
	m.state.ui.screenManager.Pop()

	infoScr := appscreen.NewInfoScreen("Info", m.theme)
	m.state.ui.screenManager.Push(infoScr)
	if out = m.View(); out == "" {
		t.Fatal("expected info screen to render")
	}
	m.state.ui.screenManager.Pop()

	// TrustScreen is now managed by screenManager
	trustScr := appscreen.NewTrustScreen("/tmp/.wt.yaml", []string{"cmd"}, m.theme)
	m.state.ui.screenManager.Push(trustScr)
	if out = m.View(); out == "" {
		t.Fatal("expected trust screen to render")
	}
	m.state.ui.screenManager.Pop()

	// WelcomeScreen is now managed by screenManager
	welcomeScr := appscreen.NewWelcomeScreen("/tmp", "/tmp/wt", m.theme)
	m.state.ui.screenManager.Push(welcomeScr)
	if out = m.View(); out == "" {
		t.Fatal("expected welcome screen to render")
	}
	m.state.ui.screenManager.Pop()

	paletteItems := []appscreen.PaletteItem{{ID: "help", Label: "Help"}}
	paletteScr := appscreen.NewCommandPaletteScreen(paletteItems, 100, 40, m.theme)
	m.state.ui.screenManager.Push(paletteScr)
	if out = m.View(); out == "" {
		t.Fatal("expected palette screen to render")
	}
	m.state.ui.screenManager.Pop()

	// No active screen should still render something
	if out = m.View(); out == "" {
		t.Fatal("expected view to render something")
	}

	inputScr := appscreen.NewInputScreen("Prompt", "Placeholder", "value", m.theme, m.config.IconsEnabled())
	m.state.ui.screenManager.Push(inputScr)
	if out = m.View(); out == "" {
		t.Fatal("expected input screen to render")
	}
	m.state.ui.screenManager.Pop()

	listScreen := appscreen.NewListSelectionScreen([]appscreen.SelectionItem{{ID: "a", Label: "A"}}, "Select", "", "", 120, 40, "", m.theme)
	m.state.ui.screenManager.Push(listScreen)
	if out = m.View(); out == "" {
		t.Fatal("expected list selection screen to render")
	}
}

func TestErrMsgShowsInfo(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	_, _ = m.Update(errMsg{err: errors.New("boom")})

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Fatalf("expected info screen, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}
	infoScr := m.state.ui.screenManager.Current().(*appscreen.InfoScreen)
	if !strings.Contains(infoScr.Message, "boom") {
		t.Fatalf("expected info modal to include error, got %q", infoScr.Message)
	}
}
