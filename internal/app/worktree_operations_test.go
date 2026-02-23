package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

func TestShowCreateWorktreeFromChangesNoSelection(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.data.selectedIndex = -1 // No selection

	cmd := m.showCreateWorktreeFromChanges()
	if cmd != nil {
		t.Error("Expected nil command when no worktree is selected")
	}
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Fatalf("expected info screen, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}
	infoScr := m.state.ui.screenManager.Current().(*appscreen.InfoScreen)
	if !strings.Contains(infoScr.Message, errNoWorktreeSelected) {
		t.Fatalf("expected info modal with %q, got %q", errNoWorktreeSelected, infoScr.Message)
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
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeListSelect {
		t.Fatalf("expected list screen to be active")
	}

	listScreen := m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)
	if listScreen.Title != "Select base for new worktree" {
		t.Fatalf("unexpected list title: %q", listScreen.Title)
	}
	if len(listScreen.Items) != 6 {
		t.Fatalf("expected 6 base options, got %d", len(listScreen.Items))
	}
	if listScreen.Items[0].ID != "from-current" {
		t.Fatalf("expected first option from-current, got %q", listScreen.Items[0].ID)
	}
	if listScreen.Items[1].ID != "branch-list" {
		t.Fatalf("expected second option branch-list, got %q", listScreen.Items[1].ID)
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

			if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInput {
				t.Fatalf("input screen should be initialized")
			}
			inputScr := m.state.ui.screenManager.Current().(*appscreen.InputScreen)
			if inputScr.CheckboxEnabled != tt.expectCheckbox {
				t.Fatalf("expected checkbox enabled=%v, got %v", tt.expectCheckbox, inputScr.CheckboxEnabled)
			}
			if tt.expectCheckbox && inputScr.CheckboxChecked != tt.expectChecked {
				t.Fatalf("expected checkbox checked=%v, got %v", tt.expectChecked, inputScr.CheckboxChecked)
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

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInput {
		t.Fatal("input screen should be initialized")
	}
	inputScr := m.state.ui.screenManager.Current().(*appscreen.InputScreen)

	// Should use random name, not AI-generated
	got := inputScr.Input.Value()
	if got != testRandomName {
		t.Errorf("expected random name %q, got %q", testRandomName, got)
	}

	// Verify context is stored for checkbox toggling
	if m.createFromCurrent.diff != testDiffContent {
		t.Errorf("expected diff to be cached, got %q", m.createFromCurrent.diff)
	}
	if m.createFromCurrent.randomName != testRandomName {
		t.Errorf("expected random name to be cached, got %q", m.createFromCurrent.randomName)
	}
	if m.createFromCurrent.branch != mainWorktreeName {
		t.Errorf("expected branch to be cached, got %q", m.createFromCurrent.branch)
	}
	if m.createFromCurrent.aiName != "" {
		t.Errorf("expected AI name cache to be empty, got %q", m.createFromCurrent.aiName)
	}
}

func TestHandleCheckboxToggleWithAIScript(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir:      t.TempDir(),
		BranchNameScript: "echo feature/ai-branch", // Will be sanitized to feature-ai-branch
	}
	m := NewModel(cfg, "")

	// Setup the create from current state
	m.createFromCurrent.diff = testDiff
	m.createFromCurrent.randomName = testRandomName
	m.createFromCurrent.branch = mainWorktreeName
	m.createFromCurrent.aiName = ""

	inputScr := appscreen.NewInputScreen("test", "placeholder", testRandomName, m.theme, m.config.IconsEnabled())
	inputScr.SetCheckbox("Include changes", false)
	inputScr.CheckboxFocused = true // Simulate tab to checkbox

	// Simulate checking the checkbox
	inputScr.CheckboxChecked = true
	m.createFromCurrent.inputScreen = inputScr

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
		if m.createFromCurrent.aiName == "" {
			t.Error("expected AI name to be cached")
		}
		if strings.Contains(m.createFromCurrent.aiName, "/") {
			t.Errorf("AI name should be sanitized (no slashes), got %q", m.createFromCurrent.aiName)
		}

		// Input should be updated to AI name
		got := m.createFromCurrent.inputScreen.Input.Value()
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
	m.createFromCurrent.diff = testDiff
	m.createFromCurrent.randomName = testRandomName
	m.createFromCurrent.branch = mainWorktreeName
	m.createFromCurrent.aiName = "ai-name-cached"

	inputScr := appscreen.NewInputScreen("test", "placeholder", "ai-name-cached", m.theme, m.config.IconsEnabled())
	inputScr.SetCheckbox("Include changes", true) // Start checked
	m.createFromCurrent.inputScreen = inputScr

	// Uncheck the checkbox
	inputScr.CheckboxChecked = false

	// Call handleCheckboxToggle
	cmd := m.handleCheckboxToggle()
	if cmd != nil {
		t.Error("expected nil command when unchecking (uses cached random name), got command")
	}

	// Input should be restored to random name
	got := inputScr.Input.Value()
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
	m.createFromCurrent.diff = testDiff
	m.createFromCurrent.randomName = testRandomName
	m.createFromCurrent.branch = mainWorktreeName
	m.createFromCurrent.aiName = "cached-ai-name"

	inputScr := appscreen.NewInputScreen("test", "placeholder", testRandomName, m.theme, m.config.IconsEnabled())
	inputScr.SetCheckbox("Include changes", false)
	m.createFromCurrent.inputScreen = inputScr

	// Check the checkbox (should use cached AI name, not run script again)
	inputScr.CheckboxChecked = true

	// Call handleCheckboxToggle
	cmd := m.handleCheckboxToggle()
	if cmd != nil {
		t.Error("expected nil command when using cached AI name, got command to generate new name")
	}

	// Input should be updated to cached AI name
	got := inputScr.Input.Value()
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

	m.createFromCurrent.diff = testDiff
	m.createFromCurrent.randomName = testRandomName
	m.createFromCurrent.branch = mainWorktreeName

	inputScr := appscreen.NewInputScreen("test", "placeholder", testRandomName, m.theme, m.config.IconsEnabled())
	inputScr.SetCheckbox("Include changes", false)
	m.createFromCurrent.inputScreen = inputScr

	// Check the checkbox
	inputScr.CheckboxChecked = true

	// Call handleCheckboxToggle
	cmd := m.handleCheckboxToggle()
	if cmd != nil {
		t.Error("expected nil command when no script configured, got command")
	}

	// Input should remain unchanged (random name)
	got := inputScr.Input.Value()
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
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInput {
		t.Fatal("inputScreen should be initialized")
	}
	inputScr := m.state.ui.screenManager.Current().(*appscreen.InputScreen)

	if inputScr.Prompt != "Create worktree from changes: branch name" {
		t.Errorf("Expected prompt 'Create worktree from changes: branch name', got %q", inputScr.Prompt)
	}

	// Check default value
	if inputScr.Value != "main-changes" {
		t.Errorf("Expected default value 'main-changes', got %q", inputScr.Value)
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
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Fatalf("expected info screen, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}
	infoScr := m.state.ui.screenManager.Current().(*appscreen.InfoScreen)
	if !strings.Contains(infoScr.Message, "Branch name script error") {
		t.Fatalf("expected branch name script error modal, got %q", infoScr.Message)
	}
}

func TestShowAbsorbWorktreeNoSelection(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.data.selectedIndex = -1 // No selection

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
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: "/path/to/main", Branch: mainWorktreeName, IsMain: true},
	}
	m.state.data.filteredWts = m.state.data.worktrees
	m.state.data.selectedIndex = 0

	cmd := m.showAbsorbWorktree()
	if cmd != nil {
		t.Error("Expected nil command when trying to absorb main worktree")
	}
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Errorf("Expected info screen, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}
}

func TestShowAbsorbWorktreeCreatesConfirmScreen(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	// Set up main and feature worktrees
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: "/path/to/main", Branch: mainWorktreeName, IsMain: true},
		{Path: "/path/to/feature", Branch: "feature-branch", IsMain: false},
	}
	m.state.data.filteredWts = m.state.data.worktrees
	m.state.data.selectedIndex = 1 // Select feature worktree

	cmd := m.showAbsorbWorktree()
	if cmd != nil {
		t.Error("Expected nil command from showAbsorbWorktree")
	}

	// Verify confirm screen was created in screen manager
	if !m.state.ui.screenManager.IsActive() {
		t.Fatal("Expected screen manager to be active")
	}
	if m.state.ui.screenManager.Type() != appscreen.TypeConfirm {
		t.Fatalf("Expected confirm screen, got %v", m.state.ui.screenManager.Type())
	}

	confirmScreen, ok := m.state.ui.screenManager.Current().(*appscreen.ConfirmScreen)
	if !ok {
		t.Fatal("Expected confirm screen in screen manager")
	}

	// Verify confirm action was set
	if confirmScreen.OnConfirm == nil {
		t.Fatal("Expected OnConfirm to be set")
	}

	// Verify the confirm message contains the correct information
	if confirmScreen.Message == "" {
		t.Error("Expected confirm screen message to be set")
	}
	if !strings.Contains(confirmScreen.Message, "Absorb worktree into main") {
		t.Errorf("Expected confirm message to mention 'Absorb worktree into main', got %q", confirmScreen.Message)
	}
	if !strings.Contains(confirmScreen.Message, "feature-branch") {
		t.Errorf("Expected confirm message to mention 'feature-branch', got %q", confirmScreen.Message)
	}
}

func TestShowAbsorbWorktreeNoMainWorktree(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	// Set up only a feature worktree (no main)
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: "/path/to/feature", Branch: "feature-branch", IsMain: false},
	}
	m.state.data.filteredWts = m.state.data.worktrees
	m.state.data.selectedIndex = 0

	cmd := m.showAbsorbWorktree()
	if cmd != nil {
		t.Error("Expected nil command when no main worktree exists")
	}
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Errorf("Expected info screen, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}
}

func TestShowAbsorbWorktreeOnMainBranch(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	// Set up worktrees where a non-main worktree is on the main branch
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: "/path/to/main", Branch: mainWorktreeName, IsMain: true},
		{Path: "/path/to/other", Branch: mainWorktreeName, IsMain: false}, // Same branch as main
	}
	m.state.data.filteredWts = m.state.data.worktrees
	m.state.data.selectedIndex = 1 // Select the non-main worktree that's on main branch

	cmd := m.showAbsorbWorktree()
	if cmd != nil {
		t.Error("Expected nil command when worktree is on main branch")
	}
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Errorf("Expected info screen, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}
}

func TestShowAbsorbWorktreeDirtyMainWorktree(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	// Set up worktrees where main worktree is dirty
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: "/path/to/main", Branch: mainWorktreeName, IsMain: true, Dirty: true},
		{Path: "/path/to/feature", Branch: "feature-branch", IsMain: false},
	}
	m.state.data.filteredWts = m.state.data.worktrees
	m.state.data.selectedIndex = 1 // Select the feature worktree

	cmd := m.showAbsorbWorktree()
	if cmd != nil {
		t.Error("Expected nil command when main worktree is dirty")
	}
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Errorf("Expected info screen, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}
}

func TestShowDeleteWorktree(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: "/tmp/main", Branch: mainWorktreeName, IsMain: true},
		{Path: "/tmp/feat", Branch: featureBranch},
	}

	m.state.data.selectedIndex = 0
	if cmd := m.showDeleteWorktree(); cmd != nil {
		t.Fatal("expected nil command for main worktree")
	}
	if m.state.ui.screenManager.IsActive() {
		t.Fatal("expected no screen for main worktree")
	}

	m.state.data.selectedIndex = 1
	if cmd := m.showDeleteWorktree(); cmd != nil {
		t.Fatal("expected nil command for confirm screen")
	}
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeConfirm {
		t.Fatal("expected confirm screen to be set")
	}
}

func TestShowRenameWorktree(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: "/tmp/main", Branch: mainWorktreeName, IsMain: true},
		{Path: "/tmp/feat", Branch: featureBranch},
	}

	m.state.data.selectedIndex = 0
	if cmd := m.showRenameWorktree(); cmd != nil {
		t.Fatal("expected nil command for main worktree")
	}
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Fatalf("expected info screen, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}
	infoScr := m.state.ui.screenManager.Current().(*appscreen.InfoScreen)
	if !strings.Contains(infoScr.Message, "Cannot rename") {
		t.Fatalf("expected rename warning modal, got %q", infoScr.Message)
	}
	m.state.ui.screenManager.Pop()

	m.state.data.selectedIndex = 1
	if cmd := m.showRenameWorktree(); cmd == nil {
		t.Fatal("expected input screen command")
	}
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInput {
		t.Fatal("expected input screen to be set")
	}
}

func TestShowPruneMerged(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.data.worktrees = []*models.WorktreeInfo{
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
	if m.state.ui.screenManager.Type() != appscreen.TypeLoading {
		t.Fatalf("expected loading screen, got %v", m.state.ui.screenManager.Type())
	}

	// Simulate PR data loaded - this should trigger the actual merged check
	msg := prDataLoadedMsg{prMap: nil, worktreePRs: nil, err: nil}
	updated, _ := m.Update(msg)
	m = updated.(*Model)

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Fatalf("expected info screen, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}
	infoScr := m.state.ui.screenManager.Current().(*appscreen.InfoScreen)
	if infoScr.Message != "No merged worktrees or orphaned directories to prune." {
		t.Fatalf("unexpected info modal: %q", infoScr.Message)
	}

	// Reset and test with a merged PR
	m = NewModel(cfg, "")
	m.state.data.worktrees = []*models.WorktreeInfo{
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

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeChecklist {
		t.Fatalf("expected checklist screen for prune, got active=%v type=%v",
			m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
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
	m.state.data.worktrees = []*models.WorktreeInfo{
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
	if m.state.ui.screenManager.Type() == appscreen.TypeLoading {
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
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: tmpDir, Branch: "main", IsMain: true},
	}
	m.state.data.filteredWts = m.state.data.worktrees

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
	m.state.data.worktrees = []*models.WorktreeInfo{}
	m.state.data.filteredWts = m.state.data.worktrees

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

func TestCreateFromCurrentTargetPathIncludesRepoKey(t *testing.T) {
	worktreeDir := t.TempDir()
	cfg := &config.AppConfig{WorktreeDir: worktreeDir}
	m := NewModel(cfg, "")
	m.repoKey = "myorg/myrepo"

	worktree := &models.WorktreeInfo{Path: "/tmp/branch", Branch: "feature/x"}
	msg := createFromCurrentReadyMsg{
		currentWorktree:   worktree,
		currentBranch:     "feature/x",
		hasChanges:        false,
		defaultBranchName: "feature-x-random",
	}
	m.handleCreateFromCurrentReady(msg)

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInput {
		t.Fatal("expected input screen")
	}
	inputScr := m.state.ui.screenManager.Current().(*appscreen.InputScreen)

	// Create the target path to trigger path-exists validation
	branchName := "test-branch"
	targetDir := filepath.Join(worktreeDir, "myorg", "myrepo", branchName)
	if err := os.MkdirAll(targetDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Submit with the branch name that already exists on disk
	inputScr.OnSubmit(branchName, false)
	if !strings.Contains(inputScr.ErrorMsg, "Path already exists") {
		t.Fatalf("expected path exists error containing org/repo layout, got %q", inputScr.ErrorMsg)
	}
	// Verify the error message path includes the repo key (org/repo)
	if !strings.Contains(inputScr.ErrorMsg, filepath.Join("myorg", "myrepo")) {
		t.Fatalf("target path should include repo key (org/repo layout), got %q", inputScr.ErrorMsg)
	}
}

func TestCreateFromChangesTargetPathIncludesRepoKey(t *testing.T) {
	worktreeDir := t.TempDir()
	cfg := &config.AppConfig{WorktreeDir: worktreeDir}
	m := NewModel(cfg, "")
	m.repoKey = "myorg/myrepo"
	m.setWindowSize(120, 40)

	wt := &models.WorktreeInfo{
		Path:   "/tmp/test-worktree",
		Branch: mainWorktreeName,
	}

	msg := createFromChangesReadyMsg{
		worktree:      wt,
		currentBranch: mainWorktreeName,
	}

	cmd := m.handleCreateFromChangesReady(msg)
	if cmd == nil {
		t.Fatal("expected command")
	}

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInput {
		t.Fatal("expected input screen")
	}
	inputScr := m.state.ui.screenManager.Current().(*appscreen.InputScreen)

	// Create the target path to trigger path-exists validation
	branchName := "changes-branch"
	targetDir := filepath.Join(worktreeDir, "myorg", "myrepo", branchName)
	if err := os.MkdirAll(targetDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Submit with the branch name that already exists on disk
	inputScr.OnSubmit(branchName, false)
	if !strings.Contains(inputScr.ErrorMsg, "Path already exists") {
		t.Fatalf("expected path exists error containing org/repo layout, got %q", inputScr.ErrorMsg)
	}
	if !strings.Contains(inputScr.ErrorMsg, filepath.Join("myorg", "myrepo")) {
		t.Fatalf("target path should include repo key (org/repo layout), got %q", inputScr.ErrorMsg)
	}
}
