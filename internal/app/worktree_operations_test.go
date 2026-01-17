package app

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

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
	_ = m.showPruneMerged()

	// Should return performMergedWorktreeCheck (which returns nil for no merged worktrees)
	// or textinput.Blink if there are merged worktrees
	// Key assertion: should NOT trigger loading screen or set checkMergedAfterPRRefresh
	if m.checkMergedAfterPRRefresh {
		t.Fatal("expected checkMergedAfterPRRefresh to be false for unknown host")
	}
	if m.currentScreen == screenLoading {
		t.Fatal("expected no loading screen for unknown host")
	}
}

func TestShowCreateFromCurrent(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	// Create a test git repo
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Initialize git repo
	_ = exec.Command("git", "init", "-b", "main").Run()
	_ = exec.Command("git", "config", "user.email", "test@test.com").Run()
	_ = exec.Command("git", "config", "user.name", "Test").Run()
	_ = exec.Command("git", "config", "commit.gpgsign", "false").Run()
	_ = os.WriteFile("file.txt", []byte("test"), 0o600)
	_ = exec.Command("git", "add", "file.txt").Run()
	_ = exec.Command("git", "commit", "-m", "initial").Run()

	// Set up model with worktree pointing to this repo
	m.worktrees = []*models.WorktreeInfo{
		{Path: tmpDir, Branch: "main", IsMain: true},
	}
	m.filteredWts = m.worktrees

	// Test showCreateFromCurrent
	cmd := m.showCreateFromCurrent()
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}

	msg := cmd()
	switch v := msg.(type) {
	case createFromCurrentReadyMsg:
		if v.currentWorktree == nil {
			t.Error("expected currentWorktree to be set")
		}
		if v.currentBranch == "" {
			t.Error("expected currentBranch to be set")
		}
		if v.defaultBranchName == "" {
			t.Error("expected defaultBranchName to be set")
		}
	case errMsg:
		t.Fatalf("unexpected error: %v", v.err)
	default:
		t.Fatalf("unexpected message type: %T", msg)
	}
}

func TestShowCreateFromCurrentNoWorktree(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	// No worktrees set up
	m.worktrees = []*models.WorktreeInfo{}
	m.filteredWts = m.worktrees

	cmd := m.showCreateFromCurrent()
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}

	msg := cmd()
	errMsg, ok := msg.(errMsg)
	if !ok {
		t.Fatalf("expected errMsg, got %T", msg)
	}
	if errMsg.err == nil {
		t.Error("expected error to be set")
	}
}
