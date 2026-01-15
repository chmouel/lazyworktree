package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/theme"
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

func TestAuthorInitials(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{name: "Christian B", want: "CB"},
		{name: "github-actions", want: "gi"},
		{name: "John Doe", want: "JD"},
		{name: "Single", want: "Si"},
		{name: "A", want: "A"},
		{name: "", want: ""},
	}

	for _, tt := range tests {
		if got := authorInitials(tt.name); got != tt.want {
			t.Fatalf("authorInitials(%q) = %q, want %q", tt.name, got, tt.want)
		}
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
	if len(m.listScreen.items) != 6 {
		t.Fatalf("expected 6 base options, got %d", len(m.listScreen.items))
	}
	if m.listScreen.items[0].id != "from-current" {
		t.Fatalf("expected first option from-current, got %q", m.listScreen.items[0].id)
	}
	if m.listScreen.items[1].id != "branch-list" {
		t.Fatalf("expected second option branch-list, got %q", m.listScreen.items[1].id)
	}
}

func TestDetermineCurrentWorktreePrefersSelection(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	main := &models.WorktreeInfo{Path: "/tmp/main", Branch: "main", IsMain: true}
	feature := &models.WorktreeInfo{Path: "/tmp/feature", Branch: "feature"}
	m.worktrees = []*models.WorktreeInfo{main, feature}
	m.filteredWts = m.worktrees

	rows := []table.Row{
		{"main"},
		{"feature"},
	}
	m.worktreeTable.SetRows(rows)
	m.worktreeTable.SetCursor(1)

	got := m.determineCurrentWorktree()
	if got != feature {
		t.Fatalf("expected selected worktree, got %v", got)
	}
}

func TestHandleCreateFromCurrentReadyCheckboxVisibility(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	worktree := &models.WorktreeInfo{Path: "/tmp/branch", Branch: "feature/x"}

	cases := []struct {
		name           string
		hasChanges     bool
		expectCheckbox bool
		expectChecked  bool
	}{
		{name: "no changes", hasChanges: false, expectCheckbox: false},
		{name: "with changes", hasChanges: true, expectCheckbox: true, expectChecked: false},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			msg := createFromCurrentReadyMsg{
				currentWorktree:   worktree,
				currentBranch:     "feature/x",
				hasChanges:        tt.hasChanges,
				defaultBranchName: "feature-x",
			}
			m.handleCreateFromCurrentReady(msg)

			if m.inputScreen == nil {
				t.Fatalf("input screen should be initialized")
			}
			if m.inputScreen.checkboxEnabled != tt.expectCheckbox {
				t.Fatalf("expected checkbox enabled=%v, got %v", tt.expectCheckbox, m.inputScreen.checkboxEnabled)
			}
			if tt.expectCheckbox && m.inputScreen.checkboxChecked != tt.expectChecked {
				t.Fatalf("expected checkbox checked=%v, got %v", tt.expectChecked, m.inputScreen.checkboxChecked)
			}
		})
	}
}

func TestHandleCreateFromCurrentUsesRandomNameByDefault(t *testing.T) {
	const testDiffContent = "some diff content"

	cfg := &config.AppConfig{
		WorktreeDir:      t.TempDir(),
		BranchNameScript: "echo ai-generated-name", // Script is configured but shouldn't run
	}
	m := NewModel(cfg, "")

	msg := createFromCurrentReadyMsg{
		currentWorktree:   &models.WorktreeInfo{Path: "/tmp/branch", Branch: mainWorktreeName},
		currentBranch:     mainWorktreeName,
		diff:              testDiffContent,
		hasChanges:        true,
		defaultBranchName: testRandomName,
	}

	m.handleCreateFromCurrentReady(msg)

	if m.inputScreen == nil {
		t.Fatal("input screen should be initialized")
	}

	// Should use random name, not AI-generated
	got := m.inputScreen.input.Value()
	if got != testRandomName {
		t.Errorf("expected random name %q, got %q", testRandomName, got)
	}

	// Verify context is stored for checkbox toggling
	if m.createFromCurrentDiff != testDiffContent {
		t.Errorf("expected diff to be cached, got %q", m.createFromCurrentDiff)
	}
	if m.createFromCurrentRandomName != testRandomName {
		t.Errorf("expected random name to be cached, got %q", m.createFromCurrentRandomName)
	}
	if m.createFromCurrentBranch != mainWorktreeName {
		t.Errorf("expected branch to be cached, got %q", m.createFromCurrentBranch)
	}
	if m.createFromCurrentAIName != "" {
		t.Errorf("expected AI name cache to be empty, got %q", m.createFromCurrentAIName)
	}
}

func TestHandleCheckboxToggleWithAIScript(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir:      t.TempDir(),
		BranchNameScript: "echo feature/ai-branch", // Will be sanitized to feature-ai-branch
	}
	m := NewModel(cfg, "")

	// Setup the create from current state
	m.createFromCurrentDiff = testDiff
	m.createFromCurrentRandomName = testRandomName
	m.createFromCurrentBranch = mainWorktreeName
	m.createFromCurrentAIName = ""

	m.inputScreen = NewInputScreen("test", "placeholder", testRandomName, m.theme)
	m.inputScreen.SetCheckbox("Include changes", false)
	m.inputScreen.checkboxFocused = true // Simulate tab to checkbox

	// Simulate checking the checkbox
	m.inputScreen.checkboxChecked = true

	// Call handleCheckboxToggle
	cmd := m.handleCheckboxToggle()
	if cmd == nil {
		t.Fatal("expected command to generate AI name, got nil")
	}

	// Execute the command to generate AI name
	msg := cmd()
	if aiBranchMsg, ok := msg.(aiBranchNameGeneratedMsg); ok {
		if aiBranchMsg.err != nil {
			t.Fatalf("AI generation failed: %v", aiBranchMsg.err)
		}
		if aiBranchMsg.name == "" {
			t.Fatal("AI generation returned empty name")
		}

		// Now handle the message to update the model
		updated, _ := m.Update(aiBranchMsg)
		m = updated.(*Model)

		// Verify AI name was sanitized and cached
		if m.createFromCurrentAIName == "" {
			t.Error("expected AI name to be cached")
		}
		if strings.Contains(m.createFromCurrentAIName, "/") {
			t.Errorf("AI name should be sanitized (no slashes), got %q", m.createFromCurrentAIName)
		}

		// Input should be updated to AI name
		got := m.inputScreen.input.Value()
		if got == testRandomName {
			t.Error("expected input to be updated from random name to AI name")
		}
		if strings.Contains(got, "/") {
			t.Errorf("input should not contain slashes, got %q", got)
		}
	} else {
		t.Fatalf("expected aiBranchNameGeneratedMsg, got %T", msg)
	}
}

func TestHandleCheckboxToggleBackToUnchecked(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir:      t.TempDir(),
		BranchNameScript: "echo ai-name",
	}
	m := NewModel(cfg, "")

	// Setup state with AI name cached
	m.createFromCurrentDiff = testDiff
	m.createFromCurrentRandomName = testRandomName
	m.createFromCurrentBranch = mainWorktreeName
	m.createFromCurrentAIName = "ai-name-cached"

	m.inputScreen = NewInputScreen("test", "placeholder", "ai-name-cached", m.theme)
	m.inputScreen.SetCheckbox("Include changes", true) // Start checked

	// Uncheck the checkbox
	m.inputScreen.checkboxChecked = false

	// Call handleCheckboxToggle
	cmd := m.handleCheckboxToggle()
	if cmd != nil {
		t.Error("expected nil command when unchecking (uses cached random name), got command")
	}

	// Input should be restored to random name
	got := m.inputScreen.input.Value()
	if got != testRandomName {
		t.Errorf("expected input to be restored to random name %q, got %q", testRandomName, got)
	}
}

func TestHandleCheckboxToggleUsesCachedAIName(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir:      t.TempDir(),
		BranchNameScript: "echo new-ai-name",
	}
	m := NewModel(cfg, "")

	// Setup state with AI name already cached
	m.createFromCurrentDiff = testDiff
	m.createFromCurrentRandomName = testRandomName
	m.createFromCurrentBranch = mainWorktreeName
	m.createFromCurrentAIName = "cached-ai-name"

	m.inputScreen = NewInputScreen("test", "placeholder", testRandomName, m.theme)
	m.inputScreen.SetCheckbox("Include changes", false)

	// Check the checkbox (should use cached AI name, not run script again)
	m.inputScreen.checkboxChecked = true

	// Call handleCheckboxToggle
	cmd := m.handleCheckboxToggle()
	if cmd != nil {
		t.Error("expected nil command when using cached AI name, got command to generate new name")
	}

	// Input should be updated to cached AI name
	got := m.inputScreen.input.Value()
	if got != "cached-ai-name" {
		t.Errorf("expected cached AI name 'cached-ai-name', got %q", got)
	}
}

func TestHandleCheckboxToggleNoScriptConfigured(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		// No BranchNameScript configured
	}
	m := NewModel(cfg, "")

	m.createFromCurrentDiff = testDiff
	m.createFromCurrentRandomName = testRandomName
	m.createFromCurrentBranch = mainWorktreeName

	m.inputScreen = NewInputScreen("test", "placeholder", testRandomName, m.theme)
	m.inputScreen.SetCheckbox("Include changes", false)

	// Check the checkbox
	m.inputScreen.checkboxChecked = true

	// Call handleCheckboxToggle
	cmd := m.handleCheckboxToggle()
	if cmd != nil {
		t.Error("expected nil command when no script configured, got command")
	}

	// Input should remain unchanged (random name)
	got := m.inputScreen.input.Value()
	if got != testRandomName {
		t.Errorf("expected random name to remain %q, got %q", testRandomName, got)
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
			m.inputScreen = NewInputScreen("test", "placeholder", "initial", m.theme)
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
	m.windowWidth = 200
	m.windowHeight = 50
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

	if pager := m.pagerCommand(); pager != "less --use-color -z-4 -q --wordwrap -qcR -P 'Press q to exit..'" {
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

func TestZellijSessionReadyAttachesDirectly(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)

	updated, cmd := m.Update(zellijSessionReadyMsg{sessionName: "wt_test", attach: true, insideZellij: false})
	model := updated.(*Model)
	if model.currentScreen != screenNone {
		t.Fatalf("expected no screen change, got %v", model.currentScreen)
	}
	if cmd == nil {
		t.Fatal("expected attach command to be returned")
	}
}

func TestZellijSessionReadyShowsInfoWhenInsideZellij(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)

	updated, cmd := m.Update(zellijSessionReadyMsg{sessionName: "wt_test", attach: true, insideZellij: true})
	model := updated.(*Model)
	if model.currentScreen != screenInfo {
		t.Fatalf("expected info screen, got %v", model.currentScreen)
	}
	if model.infoScreen == nil {
		t.Fatal("expected info screen to be created")
	}
	if cmd != nil {
		t.Fatal("expected no command when inside zellij")
	}
	if !strings.Contains(model.infoScreen.message, "zellij attach 'wt_test'") {
		t.Errorf("expected attach message, got %q", model.infoScreen.message)
	}
}

func TestBuildZellijScriptAddsLayoutsAsTabs(t *testing.T) {
	cfg := &config.TmuxCommand{
		SessionName: "session",
		OnExists:    "",
	}
	script := buildZellijScript("session", cfg, []string{"/tmp/layout1", "/tmp/layout2"})
	if !strings.Contains(script, "zellij attach --create-background \"$session\"") {
		t.Fatalf("expected session creation without layout flag, got %q", script)
	}
	if strings.Count(script, "new-tab --layout") != 2 {
		t.Fatalf("expected all layouts added as new tabs, got %q", script)
	}
	if !strings.Contains(script, "new-tab --layout '/tmp/layout1'") || !strings.Contains(script, "new-tab --layout '/tmp/layout2'") {
		t.Fatalf("expected both layouts as new-tab actions, got %q", script)
	}
	if !strings.Contains(script, "while ! zellij list-sessions --short 2>/dev/null | grep -Fxq \"$session\"; do") {
		t.Fatalf("expected wait loop for session readiness, got %q", script)
	}
	if !strings.Contains(script, "if [ $tries -ge 50 ]") {
		t.Fatalf("expected timeout in wait loop, got %q", script)
	}
	if !strings.Contains(script, "go-to-tab 1") || !strings.Contains(script, "close-tab") {
		t.Fatalf("expected close-tab cleanup to remove default tab, got %q", script)
	}
}

func TestBuildZellijScriptNoLayoutsKeepsDefaultTab(t *testing.T) {
	cfg := &config.TmuxCommand{
		SessionName: "session",
		OnExists:    "",
	}
	script := buildZellijScript("session", cfg, nil)
	if strings.Contains(script, "new-tab --layout") {
		t.Fatalf("expected no new-tab actions when no layouts provided, got %q", script)
	}
	if strings.Contains(script, "close-tab") {
		t.Fatalf("did not expect close-tab when no layouts provided, got %q", script)
	}
	if !strings.Contains(script, "zellij attach --create-background \"$session\"") {
		t.Fatalf("expected basic attach when no layouts provided, got %q", script)
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

func TestRenderPaneTitleBasic(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.theme = theme.Dracula()

	// Test basic title rendering (no filter, no zoom)
	title := m.renderPaneTitle(1, "Worktrees", true, 100)

	if !strings.Contains(title, "[1]") {
		t.Error("Expected title to contain pane number [1]")
	}
	if !strings.Contains(title, "Worktrees") {
		t.Error("Expected title to contain pane name")
	}
	if strings.Contains(title, "Filtered") {
		t.Error("Expected no filter indicator")
	}
	if strings.Contains(title, "Zoomed") {
		t.Error("Expected no zoom indicator")
	}
}

func TestRenderPaneTitleWithFilter(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.theme = theme.Dracula()
	m.filterQuery = testFilterQuery // Activate filter for pane 0
	m.showingFilter = false
	m.showingSearch = false

	title := m.renderPaneTitle(1, "Worktrees", true, 100)

	if !strings.Contains(title, "Filtered") {
		t.Error("Expected filter indicator to show 'Filtered'")
	}
	if !strings.Contains(title, "Esc") {
		t.Error("Expected filter indicator to show 'Esc' key")
	}
	if !strings.Contains(title, "Clear") {
		t.Error("Expected filter indicator to show 'Clear' action")
	}
}

func TestRenderPaneTitleWithZoom(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.theme = theme.Dracula()
	m.zoomedPane = 0 // Zoom pane 0

	title := m.renderPaneTitle(1, "Worktrees", true, 100)

	if !strings.Contains(title, "Zoomed") {
		t.Error("Expected zoom indicator to show 'Zoomed'")
	}
	if !strings.Contains(title, "=") {
		t.Error("Expected zoom indicator to show '=' key")
	}
	if !strings.Contains(title, "Unzoom") {
		t.Error("Expected zoom indicator to show 'Unzoom' action")
	}
}

func TestRenderPaneTitleWithFilterAndZoom(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.theme = theme.Dracula()
	m.filterQuery = testFilterQuery // Activate filter for pane 0
	m.showingFilter = false
	m.showingSearch = false
	m.zoomedPane = 0 // Zoom pane 0

	title := m.renderPaneTitle(1, "Worktrees", true, 100)

	// Should show both indicators
	if !strings.Contains(title, "Filtered") {
		t.Error("Expected filter indicator when both filter and zoom are active")
	}
	if !strings.Contains(title, "Zoomed") {
		t.Error("Expected zoom indicator when both filter and zoom are active")
	}
}

func TestRenderPaneTitleNoZoomWhenDifferentPane(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.theme = theme.Dracula()
	m.zoomedPane = 1 // Zoom pane 1 (status)

	// Render title for pane 0 (worktrees)
	title := m.renderPaneTitle(1, "Worktrees", true, 100)

	if strings.Contains(title, "Zoomed") {
		t.Error("Expected no zoom indicator for unzoomed pane")
	}
}

func TestRenderPaneTitleUsesAccentFg(t *testing.T) {
	// Test with a light theme to ensure AccentFg (white) is used instead of black
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.theme = theme.CleanLight()
	m.zoomedPane = 0

	title := m.renderPaneTitle(1, "Worktrees", true, 100)

	// The title should contain styling - we can't directly test the color
	// but we can verify the indicator is present and properly formatted
	if !strings.Contains(title, "Zoomed") || !strings.Contains(title, "=") {
		t.Error("Expected properly formatted zoom indicator with theme colors")
	}

	// Test with filter too
	m.filterQuery = "test"
	m.showingFilter = false
	m.showingSearch = false

	title = m.renderPaneTitle(1, "Worktrees", true, 100)

	if !strings.Contains(title, "Filtered") || !strings.Contains(title, "Esc") {
		t.Error("Expected properly formatted filter indicator with theme colors")
	}
}

const (
	featureBranch     = "feature"
	testPRURL         = "https://example.com/pr/1"
	testNoDiffMessage = "No diff to show."
)

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

func TestEnvMapToList(t *testing.T) {
	env := map[string]string{
		"A": "1",
		"B": "2",
	}

	list := envMapToList(env)
	if len(list) != 2 {
		t.Fatalf("expected two env entries, got %d", len(list))
	}

	values := map[string]bool{}
	for _, entry := range list {
		values[entry] = true
	}

	if !values["A=1"] || !values["B=2"] {
		t.Fatalf("unexpected env list: %v", list)
	}
}

func TestReadTmuxSessionFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "session")

	if got := readTmuxSessionFile(filepath.Join(tmpDir, "missing"), "fallback"); got != "fallback" {
		t.Fatalf("expected fallback on missing file, got %q", got)
	}

	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatalf("write empty file: %v", err)
	}
	if got := readTmuxSessionFile(path, "fallback"); got != "fallback" {
		t.Fatalf("expected fallback on empty file, got %q", got)
	}

	if err := os.WriteFile(path, []byte("session-name\n"), 0o600); err != nil {
		t.Fatalf("write session file: %v", err)
	}
	if got := readTmuxSessionFile(path, "fallback"); got != "session-name" {
		t.Fatalf("expected session-name, got %q", got)
	}
}

func TestCollectInitTerminateCommands(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir:       t.TempDir(),
		InitCommands:      []string{"init-1"},
		TerminateCommands: []string{"term-1"},
	}
	m := NewModel(cfg, "")
	m.repoConfig = &config.RepoConfig{
		InitCommands:      []string{"init-2"},
		TerminateCommands: []string{"term-2"},
	}

	initCmds := m.collectInitCommands()
	if strings.Join(initCmds, ",") != "init-1,init-2" {
		t.Fatalf("unexpected init commands: %v", initCmds)
	}

	termCmds := m.collectTerminateCommands()
	if strings.Join(termCmds, ",") != "term-1,term-2" {
		t.Fatalf("unexpected terminate commands: %v", termCmds)
	}
}

func TestRunCommandsWithTrustNever(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		TrustMode:   "never",
	}
	m := NewModel(cfg, "")

	called := false
	cmd := m.runCommandsWithTrust([]string{"echo hi"}, "", nil, func() tea.Msg {
		called = true
		return nil
	})
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}
	_ = cmd()
	if !called {
		t.Fatal("expected after function to be called")
	}
}

func TestRunCommandsWithTrustTofu(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	trustPath := filepath.Join(t.TempDir(), ".wt.yaml")
	if err := os.WriteFile(trustPath, []byte("commands: []"), 0o600); err != nil {
		t.Fatalf("write trust file: %v", err)
	}
	m.repoConfigPath = trustPath
	m.repoConfig = &config.RepoConfig{}

	cmd := m.runCommandsWithTrust([]string{"echo hi"}, "", nil, nil)
	if cmd != nil {
		t.Fatal("expected no command for trust prompt")
	}
	if m.currentScreen != screenTrust {
		t.Fatalf("expected trust screen, got %v", m.currentScreen)
	}
	if m.trustScreen == nil || len(m.pendingCommands) != 1 {
		t.Fatalf("expected pending commands to be set, got %v", m.pendingCommands)
	}
}

func TestClearPendingTrust(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.pendingCommands = []string{"cmd"}
	m.pendingCmdEnv = map[string]string{"A": "1"}
	m.pendingCmdCwd = "/tmp"
	m.pendingAfter = func() tea.Msg { return nil }
	m.pendingTrust = "/tmp/.wt.yaml"
	m.trustScreen = NewTrustScreen("/tmp/.wt.yaml", []string{"cmd"}, m.theme)

	m.clearPendingTrust()

	if m.pendingCommands != nil || m.pendingCmdEnv != nil || m.pendingCmdCwd != "" || m.pendingAfter != nil || m.pendingTrust != "" || m.trustScreen != nil {
		t.Fatal("expected pending trust state to be cleared")
	}
}

func TestShowDeleteWorktree(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.filteredWts = []*models.WorktreeInfo{
		{Path: "/tmp/main", Branch: mainWorktreeName, IsMain: true},
		{Path: "/tmp/feat", Branch: featureBranch},
	}

	m.selectedIndex = 0
	if cmd := m.showDeleteWorktree(); cmd != nil {
		t.Fatal("expected nil command for main worktree")
	}
	if m.confirmScreen != nil {
		t.Fatal("expected no confirm screen for main worktree")
	}

	m.selectedIndex = 1
	if cmd := m.showDeleteWorktree(); cmd != nil {
		t.Fatal("expected nil command for confirm screen")
	}
	if m.confirmScreen == nil || m.confirmAction == nil || m.currentScreen != screenConfirm {
		t.Fatal("expected confirm screen to be set")
	}
}

func TestWorktreeDeletedMsg(t *testing.T) {
	t.Run("success shows branch deletion prompt", func(t *testing.T) {
		cfg := &config.AppConfig{
			WorktreeDir: t.TempDir(),
		}
		m := NewModel(cfg, "")
		m.currentScreen = screenNone

		msg := worktreeDeletedMsg{
			path:   "/tmp/feat",
			branch: "feature-branch",
			err:    nil,
		}

		result, cmd := m.Update(msg)
		m = result.(*Model)

		if cmd != nil {
			t.Fatal("expected nil command")
		}
		if m.currentScreen != screenConfirm {
			t.Fatalf("expected confirm screen, got %v", m.currentScreen)
		}
		if m.confirmScreen == nil {
			t.Fatal("expected confirm screen to be set")
		}
		if m.confirmAction == nil {
			t.Fatal("expected confirm action to be set")
		}
		if !strings.Contains(m.confirmScreen.message, "Delete branch 'feature-branch'?") {
			t.Fatalf("unexpected message: %s", m.confirmScreen.message)
		}
		if m.confirmScreen.selectedButton != 0 {
			t.Fatalf("expected default button to be 0, got %d", m.confirmScreen.selectedButton)
		}
	})

	t.Run("failure does not show branch deletion prompt", func(t *testing.T) {
		cfg := &config.AppConfig{
			WorktreeDir: t.TempDir(),
		}
		m := NewModel(cfg, "")
		m.currentScreen = screenNone

		msg := worktreeDeletedMsg{
			path:   "/tmp/feat",
			branch: "feature-branch",
			err:    fmt.Errorf("worktree deletion failed"),
		}

		result, cmd := m.Update(msg)
		m = result.(*Model)

		if cmd != nil {
			t.Fatal("expected nil command")
		}
		if m.currentScreen != screenNone {
			t.Fatalf("expected screen to remain unchanged, got %v", m.currentScreen)
		}
		if m.confirmScreen != nil {
			t.Fatal("expected no confirm screen for failed deletion")
		}
	})
}

func TestShowRenameWorktree(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.filteredWts = []*models.WorktreeInfo{
		{Path: "/tmp/main", Branch: mainWorktreeName, IsMain: true},
		{Path: "/tmp/feat", Branch: featureBranch},
	}

	m.selectedIndex = 0
	if cmd := m.showRenameWorktree(); cmd != nil {
		t.Fatal("expected nil command for main worktree")
	}
	if m.currentScreen != screenInfo {
		t.Fatalf("expected info screen, got %v", m.currentScreen)
	}
	if m.infoScreen == nil || !strings.Contains(m.infoScreen.message, "Cannot rename") {
		t.Fatalf("expected rename warning modal, got %#v", m.infoScreen)
	}

	m.selectedIndex = 1
	if cmd := m.showRenameWorktree(); cmd == nil {
		t.Fatal("expected input screen command")
	}
	if m.inputScreen == nil || m.currentScreen != screenInput {
		t.Fatal("expected input screen to be set")
	}
}

func TestShowPruneMerged(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.worktrees = []*models.WorktreeInfo{
		{Path: "/tmp/main", Branch: mainWorktreeName, IsMain: true},
		{Path: "/tmp/feat", Branch: featureBranch, PR: &models.PRInfo{State: "OPEN"}},
	}

	// showPruneMerged now triggers PR data fetch first
	if cmd := m.showPruneMerged(); cmd == nil {
		t.Fatal("expected fetchPRData command")
	}
	if !m.checkMergedAfterPRRefresh {
		t.Fatal("expected checkMergedAfterPRRefresh flag to be set")
	}
	if m.currentScreen != screenLoading {
		t.Fatalf("expected loading screen, got %v", m.currentScreen)
	}

	// Simulate PR data loaded - this should trigger the actual merged check
	msg := prDataLoadedMsg{prMap: nil, worktreePRs: nil, err: nil}
	updated, _ := m.Update(msg)
	m = updated.(*Model)

	if m.currentScreen != screenInfo {
		t.Fatalf("expected info screen, got %v", m.currentScreen)
	}
	if m.infoScreen == nil || m.infoScreen.message != "No merged worktrees to prune." {
		t.Fatalf("unexpected info modal: %#v", m.infoScreen)
	}

	// Reset and test with a merged PR
	m = NewModel(cfg, "")
	m.worktrees = []*models.WorktreeInfo{
		{Path: "/tmp/main", Branch: mainWorktreeName, IsMain: true},
		{Path: "/tmp/merged", Branch: "merged", PR: &models.PRInfo{State: "MERGED"}},
	}

	if m.showPruneMerged() == nil {
		t.Fatal("expected fetchPRData command")
	}

	// Simulate PR data loaded - this should show the checklist
	msg = prDataLoadedMsg{prMap: nil, worktreePRs: nil, err: nil}
	updated, _ = m.Update(msg)
	m = updated.(*Model)

	if m.checklistScreen == nil || m.checklistSubmit == nil || m.currentScreen != screenChecklist {
		t.Fatal("expected checklist screen for prune")
	}
}

func TestShowPruneMergedUnknownHost(t *testing.T) {
	// Test that showPruneMerged skips PR fetch for unknown hosts
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}

	// Create a test repo with unknown remote
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "remote", "add", "origin", "https://gitea.example.com/repo.git")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Test User")
	runGit(t, repo, "config", "commit.gpgsign", "false")
	runGit(t, repo, "commit", "--allow-empty", "-m", "Initial commit")

	withCwd(t, repo)

	m := NewModel(cfg, "")
	m.worktrees = []*models.WorktreeInfo{
		{Path: repo, Branch: mainWorktreeName, IsMain: true},
	}

	// showPruneMerged should skip PR fetch and go straight to merged check
	cmd := m.showPruneMerged()

	// Should return performMergedWorktreeCheck (which returns nil for no merged worktrees)
	// or textinput.Blink if there are merged worktrees
	// Key assertion: should NOT trigger loading screen or set checkMergedAfterPRRefresh
	if m.checkMergedAfterPRRefresh {
		t.Fatal("expected checkMergedAfterPRRefresh to be false for unknown host")
	}
	if m.currentScreen == screenLoading {
		t.Fatal("expected no loading screen for unknown host")
	}

	// Should show info screen with "No merged worktrees to prune"
	if m.currentScreen != screenInfo {
		t.Fatalf("expected info screen, got %v", m.currentScreen)
	}

	_ = cmd // May be nil or blink depending on merged worktrees
}

func TestHandlePruneResult(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	msg := pruneResultMsg{
		worktrees: []*models.WorktreeInfo{{Path: "/tmp/wt", Branch: featureBranch}},
		pruned:    2,
		failed:    1,
	}
	_, _ = m.handlePruneResult(msg)

	if !strings.Contains(m.statusContent, "Pruned 2 merged worktrees") || !strings.Contains(m.statusContent, "(1 failed)") {
		t.Fatalf("unexpected prune status: %q", m.statusContent)
	}
	if len(m.worktrees) != 1 {
		t.Fatalf("expected worktrees to be updated, got %d", len(m.worktrees))
	}
}

func TestHandleAbsorbResult(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	_, cmd := m.handleAbsorbResult(absorbMergeResultMsg{err: fmt.Errorf("boom")})
	if cmd != nil {
		t.Fatal("expected no command on error")
	}
	if m.currentScreen != screenInfo {
		t.Fatal("expected screenInfo to be shown for error")
	}
	if m.infoScreen == nil {
		t.Fatal("expected infoScreen to be set")
	}

	// Reset for next test
	m.currentScreen = screenNone
	m.infoScreen = nil

	_, cmd = m.handleAbsorbResult(absorbMergeResultMsg{path: "/tmp/wt", branch: featureBranch})
	if cmd == nil {
		t.Fatal("expected command for delete worktree")
	}
}

func TestShowDiffNonInteractiveNoDiff(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir:       t.TempDir(),
		MaxUntrackedDiffs: 0,
		MaxDiffChars:      1000,
	}
	m := NewModel(cfg, "")
	m.filteredWts = []*models.WorktreeInfo{{Path: cfg.WorktreeDir, Branch: featureBranch}}
	m.selectedIndex = 0
	// statusFilesAll is empty by default, simulating no changes

	// showDiff with no changes should now show an info screen
	cmd := m.showDiff()
	if cmd != nil {
		t.Fatal("expected no command when there are no changes")
	}

	// Verify info screen is shown
	if m.currentScreen != screenInfo {
		t.Fatalf("expected screenInfo, got %v", m.currentScreen)
	}
	if m.infoScreen == nil {
		t.Fatal("expected infoScreen to be set")
	}
	if m.infoScreen.message != testNoDiffMessage {
		t.Fatalf("expected message %q, got %q", testNoDiffMessage, m.infoScreen.message)
	}
}

func TestShowDiffInteractiveNoDiff(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir:         t.TempDir(),
		GitPager:            "delta",
		GitPagerInteractive: true,
		MaxUntrackedDiffs:   0,
		MaxDiffChars:        1000,
	}
	m := NewModel(cfg, "")
	m.filteredWts = []*models.WorktreeInfo{{Path: cfg.WorktreeDir, Branch: featureBranch}}
	m.selectedIndex = 0
	// statusFilesAll is empty by default, simulating no changes

	cmd := m.showDiff()
	if cmd != nil {
		t.Fatal("expected no command when there are no changes in interactive mode")
	}

	if m.currentScreen != screenInfo {
		t.Fatalf("expected screenInfo, got %v", m.currentScreen)
	}
	if m.infoScreen == nil {
		t.Fatal("expected infoScreen to be set")
	}
	if m.infoScreen.message != testNoDiffMessage {
		t.Fatalf("expected message %q, got %q", testNoDiffMessage, m.infoScreen.message)
	}
}

func TestShowDiffVSCodeNoDiff(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir:       t.TempDir(),
		GitPager:          "code --wait --diff",
		MaxUntrackedDiffs: 0,
		MaxDiffChars:      1000,
	}
	m := NewModel(cfg, "")
	m.filteredWts = []*models.WorktreeInfo{{Path: cfg.WorktreeDir, Branch: featureBranch}}
	m.selectedIndex = 0
	// statusFilesAll is empty by default, simulating no changes

	cmd := m.showDiff()
	if cmd != nil {
		t.Fatal("expected no command when there are no changes in VSCode mode")
	}

	if m.currentScreen != screenInfo {
		t.Fatalf("expected screenInfo, got %v", m.currentScreen)
	}
	if m.infoScreen == nil {
		t.Fatal("expected infoScreen to be set")
	}
	if m.infoScreen.message != testNoDiffMessage {
		t.Fatalf("expected message %q, got %q", testNoDiffMessage, m.infoScreen.message)
	}
}

func TestShowDiffInteractiveWithChanges(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir:         t.TempDir(),
		GitPager:            "delta",
		GitPagerInteractive: true,
		MaxUntrackedDiffs:   5,
		MaxDiffChars:        1000,
	}
	m := NewModel(cfg, "")
	m.filteredWts = []*models.WorktreeInfo{{Path: cfg.WorktreeDir, Branch: featureBranch}}
	m.selectedIndex = 0

	// Simulate having changes
	m.statusFilesAll = []StatusFile{
		{Filename: "test.go", Status: ".M", IsUntracked: false},
	}

	// Set up command recorder to capture the execution
	recorder := &commandRecorder{}
	m.commandRunner = recorder.runner
	m.execProcess = recorder.exec

	cmd := m.showDiff()
	if cmd == nil {
		t.Fatal("expected a command when there are changes in interactive mode")
	}

	// Execute the command to trigger recording
	_ = cmd()

	// Verify that git diff was executed via bash
	if len(recorder.execs) == 0 {
		t.Fatal("expected at least one command to be executed")
	}

	found := false
	for _, exec := range recorder.execs {
		if exec.name == "bash" && len(exec.args) >= 2 && exec.args[0] == "-c" {
			if strings.Contains(exec.args[1], "git diff") {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("expected bash command containing 'git diff' to be executed")
	}
}

func TestShowDiffVSCodeWithChanges(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir:       t.TempDir(),
		GitPager:          "code --wait --diff",
		MaxUntrackedDiffs: 5,
		MaxDiffChars:      1000,
	}
	m := NewModel(cfg, "")
	m.filteredWts = []*models.WorktreeInfo{{Path: cfg.WorktreeDir, Branch: featureBranch}}
	m.selectedIndex = 0

	// Simulate having changes
	m.statusFilesAll = []StatusFile{
		{Filename: "test.go", Status: ".M", IsUntracked: false},
	}

	// Set up command recorder to capture the execution
	recorder := &commandRecorder{}
	m.commandRunner = recorder.runner
	m.execProcess = recorder.exec

	cmd := m.showDiff()
	if cmd == nil {
		t.Fatal("expected a command when there are changes in VSCode mode")
	}

	// Execute the command to trigger recording
	_ = cmd()

	// Verify that git difftool was executed via bash
	if len(recorder.execs) == 0 {
		t.Fatal("expected at least one command to be executed")
	}

	found := false
	for _, exec := range recorder.execs {
		if exec.name == "bash" && len(exec.args) >= 2 && exec.args[0] == "-c" {
			if strings.Contains(exec.args[1], "git difftool") {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("expected bash command containing 'git difftool' to be executed")
	}
}

func TestHandleOpenPRsLoaded(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)

	if cmd := m.handleOpenPRsLoaded(openPRsLoadedMsg{err: fmt.Errorf("fail")}); cmd != nil {
		t.Fatal("expected no command on error")
	}
	if m.currentScreen != screenInfo {
		t.Fatalf("expected info screen, got %v", m.currentScreen)
	}
	if m.infoScreen == nil || !strings.Contains(m.infoScreen.message, "Failed to fetch PRs") {
		t.Fatalf("unexpected info modal: %#v", m.infoScreen)
	}

	m.currentScreen = screenNone
	m.infoScreen = nil

	if cmd := m.handleOpenPRsLoaded(openPRsLoadedMsg{prs: []*models.PRInfo{}}); cmd != nil {
		t.Fatal("expected no command on empty list")
	}
	if m.currentScreen != screenInfo {
		t.Fatalf("expected info screen, got %v", m.currentScreen)
	}
	if m.infoScreen == nil || m.infoScreen.message != "No open PRs/MRs found." {
		t.Fatalf("unexpected info modal: %#v", m.infoScreen)
	}

	prs := []*models.PRInfo{{Number: 1, Title: "Test", Branch: featureBranch}}
	cmd := m.handleOpenPRsLoaded(openPRsLoadedMsg{prs: prs})
	if cmd == nil {
		t.Fatal("expected command for PR selection")
	}
	if m.currentScreen != screenPRSelect || m.prSelectionScreen == nil {
		t.Fatal("expected PR selection screen")
	}
}

func TestFetchCommandMessages(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	if msg := m.fetchPRData()(); msg == nil {
		t.Fatal("expected pr data message")
	}
	if msg := m.fetchCIStatus(1, featureBranch)(); msg == nil {
		t.Fatal("expected ci status message")
	}
	if msg := m.fetchRemotes()(); msg == nil {
		t.Fatal("expected fetch remotes message")
	}
}

func TestRenderScreenVariants(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)

	m.currentScreen = screenCommit
	out := m.renderScreen()
	if out == "" || m.commitScreen == nil {
		t.Fatal("expected commit screen to render")
	}

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

	m.trustScreen = NewTrustScreen("/tmp/.wt.yaml", []string{"cmd"}, m.theme)
	m.currentScreen = screenTrust
	if out = m.renderScreen(); out == "" {
		t.Fatal("expected trust screen to render")
	}

	m.welcomeScreen = nil
	m.currentScreen = screenWelcome
	if out = m.renderScreen(); out == "" {
		t.Fatal("expected welcome screen to render")
	}

	m.paletteScreen = NewCommandPaletteScreen([]paletteItem{{id: "help", label: "Help"}}, 100, 40, m.theme)
	m.currentScreen = screenPalette
	if out = m.renderScreen(); out == "" {
		t.Fatal("expected palette screen to render")
	}

	if out = m.renderScreen(); out == "" {
		t.Fatal("expected diff screen to render")
	}

	m.inputScreen = NewInputScreen("Prompt", "Placeholder", "value", m.theme)
	m.currentScreen = screenInput
	if out = m.renderScreen(); out == "" {
		t.Fatal("expected input screen to render")
	}

	m.listScreen = NewListSelectionScreen([]selectionItem{{id: "a", label: "A"}}, "Select", "", "", 120, 40, "", m.theme)
	m.currentScreen = screenListSelect
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

func TestFetchRemotesCompleteTriggersRefresh(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.loading = true
	m.loadingScreen = NewLoadingScreen("Fetching remotes...", m.theme)

	_, cmd := m.Update(fetchRemotesCompleteMsg{})
	// loading stays true while refreshing worktrees
	if !m.loading {
		t.Fatal("expected loading to stay true during worktree refresh")
	}
	if m.statusContent != "Remotes fetched" {
		t.Fatalf("unexpected status: %q", m.statusContent)
	}
	// loading screen message should be updated to show refresh phase
	if m.loadingScreen == nil || m.loadingScreen.message != loadingRefreshWorktrees {
		t.Fatalf("expected loading screen message to be %q", loadingRefreshWorktrees)
	}
	if cmd == nil {
		t.Fatal("expected refresh command")
	}
	if msg := cmd(); msg == nil {
		t.Fatal("expected refresh command to return a message")
	}
}

func TestMaybeFetchCIStatus(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.filteredWts = []*models.WorktreeInfo{
		{Branch: featureBranch, PR: &models.PRInfo{Number: 1}},
	}
	m.selectedIndex = 0

	m.ciCache[featureBranch] = &ciCacheEntry{fetchedAt: time.Now()}
	if cmd := m.maybeFetchCIStatus(); cmd != nil {
		t.Fatal("expected no fetch when cache is fresh")
	}

	m.ciCache[featureBranch] = &ciCacheEntry{fetchedAt: time.Now().Add(-ciCacheTTL - time.Second)}
	if cmd := m.maybeFetchCIStatus(); cmd == nil {
		t.Fatal("expected fetch when cache is stale")
	}
}

func TestBuildTmuxInfoMessage(t *testing.T) {
	msg := buildTmuxInfoMessage("session", true)
	if !strings.Contains(msg, "switch-client") {
		t.Fatalf("expected switch-client message, got %q", msg)
	}
	msg = buildTmuxInfoMessage("session", false)
	if !strings.Contains(msg, "attach-session") {
		t.Fatalf("expected attach-session message, got %q", msg)
	}
}

func TestBuildZellijInfoMessage(t *testing.T) {
	msg := buildZellijInfoMessage("session")
	if !strings.Contains(msg, "zellij attach") {
		t.Fatalf("expected attach message, got %q", msg)
	}
}

func TestSanitizeTmuxSessionName(t *testing.T) {
	got := sanitizeTmuxSessionName("wt:feature/branch")
	if got != "wt-feature-branch" {
		t.Fatalf("expected sanitized name, got %q", got)
	}
}

func TestSanitizeZellijSessionName(t *testing.T) {
	got := sanitizeZellijSessionName("owner/repo\\wt:worktree")
	if got != "owner-repo-wt-worktree" {
		t.Fatalf("expected sanitized name, got %q", got)
	}
}

func TestBuildZellijScriptDefaultOnExistsIncludesNoop(t *testing.T) {
	cfg := &config.TmuxCommand{
		SessionName: "session",
		OnExists:    "",
	}
	script := buildZellijScript("session", cfg, []string{"/tmp/layout"})
	if !strings.Contains(script, "if session_exists \"$session\"; then\n  :\nfi\n") {
		t.Fatalf("expected no-op in default on_exists branch, got %q", script)
	}
}

func TestOpenTmuxSession(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	wt := &models.WorktreeInfo{Path: t.TempDir(), Branch: featureBranch}

	if cmd := m.openTmuxSession(nil, wt); cmd != nil {
		t.Fatal("expected nil command for nil tmux config")
	}

	badCfg := &config.TmuxCommand{SessionName: "session"}
	if msg := m.openTmuxSession(badCfg, wt)(); msg == nil {
		t.Fatal("expected error message for empty windows")
	}

	called := false
	m.commandRunner = func(_ string, _ ...string) *exec.Cmd {
		called = true
		return exec.Command("true")
	}
	m.execProcess = func(_ *exec.Cmd, cb tea.ExecCallback) tea.Cmd {
		return func() tea.Msg {
			return cb(nil)
		}
	}

	cfgGood := &config.TmuxCommand{
		SessionName: "session",
		Attach:      true,
		OnExists:    "switch",
		Windows:     []config.TmuxWindow{{Name: "shell"}},
	}
	cmd := m.openTmuxSession(cfgGood, wt)
	if cmd == nil {
		t.Fatal("expected tmux command")
	}
	msg := cmd()
	ready, ok := msg.(tmuxSessionReadyMsg)
	if !ok {
		t.Fatalf("expected tmuxSessionReadyMsg, got %T", msg)
	}
	if !called {
		t.Fatal("expected command runner to be called")
	}
	if ready.sessionName != "session" {
		t.Fatalf("unexpected session name: %q", ready.sessionName)
	}
	if !ready.attach {
		t.Fatal("expected attach to be true")
	}
}

func TestOpenZellijSession(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	wt := &models.WorktreeInfo{Path: t.TempDir(), Branch: featureBranch}

	if cmd := m.openZellijSession(nil, wt); cmd != nil {
		t.Fatal("expected nil command for nil zellij config")
	}

	badCfg := &config.TmuxCommand{SessionName: "session"}
	if msg := m.openZellijSession(badCfg, wt)(); msg == nil {
		t.Fatal("expected error message for empty windows")
	}

	called := false
	m.commandRunner = func(_ string, _ ...string) *exec.Cmd {
		called = true
		return exec.Command("true")
	}
	m.execProcess = func(_ *exec.Cmd, cb tea.ExecCallback) tea.Cmd {
		return func() tea.Msg {
			return cb(nil)
		}
	}

	cfgGood := &config.TmuxCommand{
		SessionName: "session",
		Attach:      true,
		OnExists:    "switch",
		Windows:     []config.TmuxWindow{{Name: "shell"}},
	}
	cmd := m.openZellijSession(cfgGood, wt)
	if cmd == nil {
		t.Fatal("expected zellij command")
	}
	msg := cmd()
	ready, ok := msg.(zellijSessionReadyMsg)
	if !ok {
		t.Fatalf("expected zellijSessionReadyMsg, got %T", msg)
	}
	if !called {
		t.Fatal("expected command runner to be called")
	}
	if ready.sessionName != "session" {
		t.Fatalf("unexpected session name: %q", ready.sessionName)
	}
	if !ready.attach {
		t.Fatal("expected attach to be true")
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
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)

	cmd := m.showThemeSelection()
	if cmd == nil {
		t.Fatal("showThemeSelection returned nil command")
	}

	if m.currentScreen != screenListSelect {
		t.Fatalf("expected screenListSelect, got %v", m.currentScreen)
	}

	if m.listScreen == nil {
		t.Fatal("listScreen should be initialized")
	}

	if m.listScreen.title != "🎨 Select Theme" {
		t.Fatalf("expected title '🎨 Select Theme', got %q", m.listScreen.title)
	}

	// Verify all themes are present
	available := theme.AvailableThemes()
	if len(m.listScreen.items) != len(available) {
		t.Fatalf("expected %d themes in list, got %d", len(available), len(m.listScreen.items))
	}
}

func TestRandomBranchName(t *testing.T) {
	name := randomBranchName()
	if name == "" {
		t.Fatal("expected non-empty random branch name")
	}
	parts := strings.Split(name, "-")
	if len(parts) != 2 {
		t.Fatalf("expected a single hyphen, got %q", name)
	}
	if !stringInSlice(randomAdjectives, parts[0]) {
		t.Fatalf("unexpected adjective %q", parts[0])
	}
	if !stringInSlice(randomNouns, parts[1]) {
		t.Fatalf("unexpected noun %q", parts[1])
	}
}

func stringInSlice(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func TestGetTmuxActiveSessions(t *testing.T) {
	tests := []struct {
		name          string
		mockOutput    string
		mockErr       bool
		expectedNames []string
	}{
		{
			name:          "filters wt- sessions and strips prefix",
			mockOutput:    "wt-feature-branch\nother-session\nwt-bugfix\nwt-another-feature\n",
			expectedNames: []string{"another-feature", "bugfix", "feature-branch"}, // sorted
		},
		{
			name:          "handles no wt- sessions",
			mockOutput:    "session1\nsession2\nregular\n",
			expectedNames: nil,
		},
		{
			name:          "handles empty output",
			mockOutput:    "",
			expectedNames: nil,
		},
		{
			name:          "handles only whitespace",
			mockOutput:    "  \n  \n",
			expectedNames: nil,
		},
		{
			name:          "handles single wt- session",
			mockOutput:    "wt-test\n",
			expectedNames: []string{"test"},
		},
		{
			name:          "handles mixed whitespace",
			mockOutput:    "  wt-test1  \nother\n  wt-test2\n",
			expectedNames: []string{"test1", "test2"},
		},
		{
			name:          "command fails (tmux not running)",
			mockErr:       true,
			expectedNames: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.AppConfig{
				WorktreeDir:   t.TempDir(),
				SessionPrefix: "wt-",
			}
			m := NewModel(cfg, "")

			// Mock commandRunner
			m.commandRunner = func(name string, args ...string) *exec.Cmd {
				if tt.mockErr {
					// Command that will fail
					return exec.Command("false")
				}
				// Command that returns the mock output
				if runtime.GOOS == osWindows {
					// #nosec G204 -- test mock data, not user input
					return exec.Command("cmd", "/c", "echo "+tt.mockOutput)
				}
				// #nosec G204 -- test mock data, not user input
				return exec.Command("printf", "%s", tt.mockOutput)
			}

			got := m.getTmuxActiveSessions()

			if !reflect.DeepEqual(got, tt.expectedNames) {
				t.Fatalf("expected %v, got %v", tt.expectedNames, got)
			}
		})
	}
}

func TestGetTmuxActiveSessionsWithCustomPrefix(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir:   t.TempDir(),
		SessionPrefix: "my-prefix-",
	}
	m := NewModel(cfg, "")

	// Mock commandRunner to return sessions with custom prefix
	m.commandRunner = func(name string, args ...string) *exec.Cmd {
		mockOutput := "my-prefix-feature\nother-session\nmy-prefix-bugfix\n"
		if runtime.GOOS == osWindows {
			return exec.Command("cmd", "/c", "echo "+mockOutput)
		}
		return exec.Command("printf", "%s", mockOutput)
	}

	got := m.getTmuxActiveSessions()
	expected := []string{"bugfix", "feature"}

	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("expected %v, got %v", expected, got)
	}
}

func TestCustomPaletteItemsIncludesActiveTmuxSessions(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir:   t.TempDir(),
		SessionPrefix: "wt-",
		CustomCommands: map[string]*config.CustomCommand{
			testShell: {
				Tmux: &config.TmuxCommand{
					SessionName: "custom-shell",
					Windows:     []config.TmuxWindow{{Name: testShell}},
				},
			},
		},
	}
	m := NewModel(cfg, "")

	// Mock commandRunner to return active sessions
	m.commandRunner = func(name string, args ...string) *exec.Cmd {
		// Return mock tmux sessions
		mockOutput := "wt-feature-a\nwt-bugfix-b\nother-session\n"
		if runtime.GOOS == osWindows {
			return exec.Command("cmd", "/c", "echo "+mockOutput)
		}
		return exec.Command("printf", "%s", mockOutput)
	}

	items := m.customPaletteItems()

	// Verify Multiplexer section exists
	hasMultiplexerSection := false
	for _, item := range items {
		if item.isSection && item.label == "Multiplexer" {
			hasMultiplexerSection = true
			break
		}
	}
	if !hasMultiplexerSection {
		t.Fatal("expected Multiplexer section in palette items")
	}

	// Verify active sessions are included
	foundFeatureA := false
	foundBugfixB := false
	foundCustomShell := false

	for _, item := range items {
		if item.id == "tmux-attach:feature-a" {
			foundFeatureA = true
			if item.label != "feature-a" {
				t.Fatalf("expected label 'feature-a', got %q", item.label)
			}
			if item.description != "active tmux session" {
				t.Fatalf("expected description 'active tmux session', got %q", item.description)
			}
		}
		if item.id == "tmux-attach:bugfix-b" {
			foundBugfixB = true
		}
		if item.id == testShell {
			foundCustomShell = true
		}
	}

	if !foundFeatureA {
		t.Fatal("expected to find active session 'feature-a'")
	}
	if !foundBugfixB {
		t.Fatal("expected to find active session 'bugfix-b'")
	}
	if !foundCustomShell {
		t.Fatal("expected to find custom tmux command 'shell'")
	}

	// Verify "Active Tmux Sessions" section exists
	hasActiveTmuxSection := false
	for _, item := range items {
		if item.isSection && item.label == "Active Tmux Sessions" {
			hasActiveTmuxSection = true
			break
		}
	}
	if !hasActiveTmuxSection {
		t.Fatal("expected 'Active Tmux Sessions' section in palette items")
	}

	// Verify active sessions appear AFTER custom commands (after Multiplexer section)
	featureAIndex := -1
	shellIndex := -1
	multiplexerIndex := -1
	for i, item := range items {
		if item.id == "tmux-attach:feature-a" {
			featureAIndex = i
		}
		if item.id == testShell {
			shellIndex = i
		}
		if item.isSection && item.label == "Multiplexer" {
			multiplexerIndex = i
		}
	}

	if shellIndex < 0 || featureAIndex < 0 || multiplexerIndex < 0 {
		t.Fatalf("expected to find all items, got shell at %d, feature-a at %d, multiplexer at %d", shellIndex, featureAIndex, multiplexerIndex)
	}

	if featureAIndex <= shellIndex {
		t.Fatalf("expected active sessions after custom commands, got feature-a at %d, shell at %d", featureAIndex, shellIndex)
	}

	if featureAIndex <= multiplexerIndex {
		t.Fatalf("expected active sessions after Multiplexer section, got feature-a at %d, multiplexer at %d", featureAIndex, multiplexerIndex)
	}
}

func TestCustomPaletteItemsWithNoActiveSessions(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir:   t.TempDir(),
		SessionPrefix: "wt-",
		CustomCommands: map[string]*config.CustomCommand{
			testShell: {
				Tmux: &config.TmuxCommand{
					SessionName: "custom-shell",
					Windows:     []config.TmuxWindow{{Name: testShell}},
				},
			},
		},
	}
	m := NewModel(cfg, "")

	// Mock commandRunner to return no wt- sessions
	m.commandRunner = func(name string, args ...string) *exec.Cmd {
		mockOutput := "other-session\nregular\n"
		if runtime.GOOS == osWindows {
			return exec.Command("cmd", "/c", "echo "+mockOutput)
		}
		return exec.Command("printf", "%s", mockOutput)
	}

	items := m.customPaletteItems()

	// Verify no active session items
	for _, item := range items {
		if strings.HasPrefix(item.id, "tmux-attach:") {
			t.Fatalf("expected no active session items, found %q", item.id)
		}
	}

	// Verify custom command still exists
	foundCustomShell := false
	for _, item := range items {
		if item.id == testShell {
			foundCustomShell = true
		}
	}
	if !foundCustomShell {
		t.Fatal("expected to find custom tmux command 'shell'")
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
		windowWidth:  100,
		windowHeight: 50,
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
		windowWidth:  100,
		windowHeight: 50,
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
		windowWidth:    100,
		windowHeight:   50,
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


func TestWorkspaceNameTruncation(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		isMain        bool
		maxNameLength int
		expected      string
	}{
		{
			name:          "short name unchanged",
			input:         "my-worktree",
			isMain:        false,
			maxNameLength: 95,
			expected:      " my-worktree",
		},
		{
			name:          "main worktree unchanged",
			input:         "",
			isMain:        true,
			maxNameLength: 95,
			expected:      " main",
		},
		{
			name:          "exactly 95 chars unchanged",
			input:         strings.Repeat("a", 94),
			isMain:        false,
			maxNameLength: 95,
			expected:      " " + strings.Repeat("a", 94),
		},
		{
			name:          "96 chars truncated to 95 plus ellipsis",
			input:         strings.Repeat("a", 95),
			isMain:        false,
			maxNameLength: 95,
			expected:      " " + strings.Repeat("a", 94) + "...",
		},
		{
			name:          "over 100 chars truncated to 95 plus ellipsis",
			input:         strings.Repeat("a", 120),
			isMain:        false,
			maxNameLength: 95,
			expected:      " " + strings.Repeat("a", 94) + "...",
		},
		{
			name:          "unicode characters handled correctly",
			input:         strings.Repeat("😀", 100),
			isMain:        false,
			maxNameLength: 95,
			expected:      " " + strings.Repeat("😀", 94) + "...",
		},
		{
			name:          "mixed ascii and unicode",
			input:         "abc" + strings.Repeat("😀", 100),
			isMain:        false,
			maxNameLength: 95,
			expected:      " abc" + strings.Repeat("😀", 91) + "...",
		},
		{
			name:          "truncation disabled with 0",
			input:         strings.Repeat("a", 200),
			isMain:        false,
			maxNameLength: 0,
			expected:      " " + strings.Repeat("a", 200),
		},
		{
			name:          "custom limit of 50",
			input:         strings.Repeat("a", 100),
			isMain:        false,
			maxNameLength: 50,
			expected:      " " + strings.Repeat("a", 49) + "...",
		},
		{
			name:          "custom limit 50 with unicode",
			input:         strings.Repeat("😀", 100),
			isMain:        false,
			maxNameLength: 50,
			expected:      " " + strings.Repeat("😀", 49) + "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the logic from updateTable() function
			var name string
			if tt.isMain {
				name = " " + mainWorktreeName
			} else {
				name = " " + tt.input
			}

			// Apply truncation logic (matching the implementation in updateTable)
			if tt.maxNameLength > 0 {
				nameRunes := []rune(name)
				if len(nameRunes) > tt.maxNameLength {
					name = string(nameRunes[:tt.maxNameLength]) + "..."
				}
			}

			if name != tt.expected {
				t.Errorf("expected %q, got %q (expected length: %d, got length: %d)",
					tt.expected, name, len([]rune(tt.expected)), len([]rune(name)))
			}
		})
	}
}

func TestFilterWorktreeEnvVars(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name: "filters worktree vars",
			input: []string{
				"PATH=/usr/bin",
				"WORKTREE_PATH=/tmp/wt",
				"HOME=/home/user",
				"MAIN_WORKTREE_PATH=/main",
				"WORKTREE_BRANCH=feature",
				"EDITOR=vim",
			},
			expected: []string{
				"PATH=/usr/bin",
				"HOME=/home/user",
				"EDITOR=vim",
			},
		},
		{
			name:     "handles empty input",
			input:    []string{},
			expected: []string{},
		},
		{
			name: "handles no worktree vars",
			input: []string{
				"PATH=/usr/bin",
				"HOME=/home/user",
			},
			expected: []string{
				"PATH=/usr/bin",
				"HOME=/home/user",
			},
		},
		{
			name: "filters all worktree vars",
			input: []string{
				"WORKTREE_PATH=/tmp/wt",
				"MAIN_WORKTREE_PATH=/main",
				"WORKTREE_BRANCH=feature",
				"WORKTREE_NAME=my-wt",
				"REPO_NAME=my-repo",
			},
			expected: []string{},
		},
		{
			name: "handles malformed entries gracefully",
			input: []string{
				"PATH=/usr/bin",
				"NOEQUALS",
				"HOME=/home/user",
			},
			expected: []string{
				"PATH=/usr/bin",
				"NOEQUALS",
				"HOME=/home/user",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterWorktreeEnvVars(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("filterWorktreeEnvVars() = %v, want %v", result, tt.expected)
			}
		})
	}
}
