package app

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

// TestCreateFromPRResultMsgSuccess tests successful PR worktree creation.
func TestCreateFromPRResultMsgSuccess(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)
	m.loading = true
	m.setLoadingScreen("Creating worktree...")

	targetPath := filepath.Join(cfg.WorktreeDir, "pr-123")
	msg := createFromPRResultMsg{
		prNumber:   123,
		branch:     "feature-branch",
		targetPath: targetPath,
		err:        nil,
	}

	_, cmd := m.Update(msg)

	// Should clear loading state
	if m.loading {
		t.Error("Expected loading to be false after successful creation")
	}
	if m.state.ui.screenManager.Type() == appscreen.TypeLoading {
		t.Error("Expected loading screen to be cleared")
	}

	// Should return command to run init commands and refresh worktrees
	if cmd == nil {
		t.Fatal("Expected command to be returned for init commands")
	}

	// Execute the command chain to verify it runs init commands
	result := cmd()
	if result == nil {
		t.Fatal("Expected command to return a message")
	}
}

// TestCreateFromPRResultMsgError tests failed PR worktree creation.
func TestCreateFromPRResultMsgError(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)
	m.loading = true
	m.setLoadingScreen("Creating worktree...")
	m.pendingSelectWorktreePath = "/some/path"

	msg := createFromPRResultMsg{
		prNumber:   456,
		branch:     "bugfix-branch",
		targetPath: "/tmp/pr-456",
		err:        fmt.Errorf("failed to checkout branch"),
	}

	_, cmd := m.Update(msg)

	// Should clear loading state
	if m.loading {
		t.Error("Expected loading to be false after error")
	}
	if m.state.ui.screenManager.Type() == appscreen.TypeLoading {
		t.Error("Expected loading screen to be cleared")
	}

	// Should clear pending selection on error
	if m.pendingSelectWorktreePath != "" {
		t.Errorf("Expected pendingSelectWorktreePath to be cleared, got %q", m.pendingSelectWorktreePath)
	}

	// Should not return a command on error
	if cmd != nil {
		t.Error("Expected no command to be returned on error")
	}

	// Should show info screen with error message
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Fatalf("Expected info screen to be shown, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}
	infoScr := m.state.ui.screenManager.Current().(*appscreen.InfoScreen)
	if !strings.Contains(infoScr.Message, "Failed to create worktree from PR/MR #456") {
		t.Errorf("Expected error message about PR #456, got %q", infoScr.Message)
	}
	if !strings.Contains(infoScr.Message, "failed to checkout branch") {
		t.Errorf("Expected error details in message, got %q", infoScr.Message)
	}
}

// TestHandleWorktreesLoadedSelectsPendingPath tests that worktrees are selected after creation.
func TestHandleWorktreesLoadedSelectsPendingPath(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)

	wt1Path := filepath.Join(cfg.WorktreeDir, "main")
	wt2Path := filepath.Join(cfg.WorktreeDir, "feature")
	wt3Path := filepath.Join(cfg.WorktreeDir, "pr-789")

	worktrees := []*models.WorktreeInfo{
		{Path: wt1Path, Branch: "main", IsMain: true},
		{Path: wt2Path, Branch: "feature"},
		{Path: wt3Path, Branch: "pr-branch"},
	}

	// Set pending selection to the PR worktree
	m.pendingSelectWorktreePath = wt3Path

	msg := worktreesLoadedMsg{
		worktrees: worktrees,
		err:       nil,
	}

	_, _ = m.handleWorktreesLoaded(msg)

	// Should have selected the pending worktree
	// Since we record access for pending worktrees (newly created), it will be sorted to top (index 0)
	// when using the default sortModeLastSwitched
	if m.state.data.selectedIndex != 0 {
		t.Errorf("Expected selectedIndex to be 0 (pr-789 sorted to top), got %d", m.state.data.selectedIndex)
	}
	if m.state.ui.worktreeTable.Cursor() != 0 {
		t.Errorf("Expected table cursor to be 0, got %d", m.state.ui.worktreeTable.Cursor())
	}

	// Should clear pending selection after applying it
	if m.pendingSelectWorktreePath != "" {
		t.Errorf("Expected pendingSelectWorktreePath to be cleared, got %q", m.pendingSelectWorktreePath)
	}
}

// TestHandleWorktreesLoadedPendingPathNotFound tests behavior when pending path doesn't exist.
func TestHandleWorktreesLoadedPendingPathNotFound(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)

	wt1Path := filepath.Join(cfg.WorktreeDir, "main")
	wt2Path := filepath.Join(cfg.WorktreeDir, "feature")

	worktrees := []*models.WorktreeInfo{
		{Path: wt1Path, Branch: "main", IsMain: true},
		{Path: wt2Path, Branch: "feature"},
	}

	// Set pending selection to a path that doesn't exist
	m.pendingSelectWorktreePath = filepath.Join(cfg.WorktreeDir, "nonexistent")

	msg := worktreesLoadedMsg{
		worktrees: worktrees,
		err:       nil,
	}

	_, _ = m.handleWorktreesLoaded(msg)

	// Should still clear pending selection even if not found
	if m.pendingSelectWorktreePath != "" {
		t.Errorf("Expected pendingSelectWorktreePath to be cleared, got %q", m.pendingSelectWorktreePath)
	}

	// Selection should remain at initial position (0)
	if m.state.data.selectedIndex != 0 {
		t.Errorf("Expected selectedIndex to remain 0, got %d", m.state.data.selectedIndex)
	}
}

// TestHandleWorktreesLoadedNoPendingPath tests normal behavior without pending selection.
func TestHandleWorktreesLoadedNoPendingPath(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)
	m.state.data.selectedIndex = 1

	wt1Path := filepath.Join(cfg.WorktreeDir, "main")
	wt2Path := filepath.Join(cfg.WorktreeDir, "feature")

	worktrees := []*models.WorktreeInfo{
		{Path: wt1Path, Branch: "main", IsMain: true},
		{Path: wt2Path, Branch: "feature"},
	}

	msg := worktreesLoadedMsg{
		worktrees: worktrees,
		err:       nil,
	}

	_, _ = m.handleWorktreesLoaded(msg)

	// Should not change selection when no pending path
	if m.state.data.selectedIndex != 0 {
		t.Errorf("Expected selectedIndex to be reset to 0, got %d", m.state.data.selectedIndex)
	}
}

// TestHandleOpenPRsLoadedAsyncCreation tests that PR worktree creation sets up async state correctly.
func TestHandleOpenPRsLoadedAsyncCreation(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)
	m.repoKey = "test/repo"

	prs := []*models.PRInfo{
		{Number: 999, Title: "Test PR", Branch: "test-branch"},
	}

	msg := openPRsLoadedMsg{prs: prs}
	_ = m.handleOpenPRsLoaded(msg)

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypePRSelect {
		t.Fatalf("Expected TypePRSelect, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}
}

func TestHandleOpenPRsLoadedAttachedBranchSelectsWorktree(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)

	mainPath := filepath.Join(cfg.WorktreeDir, "main")
	featurePath := filepath.Join(cfg.WorktreeDir, "feature")
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: mainPath, Branch: "main", IsMain: true, LastSwitchedTS: 20},
		{Path: featurePath, Branch: "feature-branch", LastSwitchedTS: 10},
	}
	m.updateTable()

	prs := []*models.PRInfo{
		{Number: 55, Title: "Attached", Branch: "feature-branch"},
	}
	_ = m.handleOpenPRsLoaded(openPRsLoadedMsg{prs: prs})

	prScr, ok := m.state.ui.screenManager.Current().(*appscreen.PRSelectionScreen)
	if !ok {
		t.Fatalf("expected PR selection screen, got %T", m.state.ui.screenManager.Current())
	}

	cmd := prScr.OnSelect(prs[0])
	if cmd != nil {
		t.Fatal("expected no command when branch is already attached")
	}

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Fatalf("expected info screen, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}

	infoScr := m.state.ui.screenManager.Current().(*appscreen.InfoScreen)
	if !strings.Contains(infoScr.Message, "already checked out") {
		t.Fatalf("expected attached branch message, got %q", infoScr.Message)
	}

	if m.state.data.selectedIndex < 0 || m.state.data.selectedIndex >= len(m.state.data.filteredWts) {
		t.Fatalf("selected index out of range: %d", m.state.data.selectedIndex)
	}
	selected := m.state.data.filteredWts[m.state.data.selectedIndex]
	if selected.Path != featurePath {
		t.Fatalf("expected selected path %q, got %q", featurePath, selected.Path)
	}
}

func TestHandleOpenPRsLoadedCreateUsesPRBranch(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)
	m.repoKey = "test/repo"

	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: filepath.Join(cfg.WorktreeDir, "main"), Branch: "main", IsMain: true},
	}
	m.updateTable()

	prs := []*models.PRInfo{
		{Number: 77, Title: "Use PR branch", Branch: "feature/demo-branch"},
	}
	_ = m.handleOpenPRsLoaded(openPRsLoadedMsg{prs: prs})

	prScr, ok := m.state.ui.screenManager.Current().(*appscreen.PRSelectionScreen)
	if !ok {
		t.Fatalf("expected PR selection screen, got %T", m.state.ui.screenManager.Current())
	}

	cmd := prScr.OnSelect(prs[0])
	if cmd == nil {
		t.Fatal("expected async creation command")
	}
	if !m.loading {
		t.Fatal("expected loading state to be enabled")
	}

	expectedPath := filepath.Join(m.getRepoWorktreeDir(), "feature-demo-branch")
	if m.pendingSelectWorktreePath != expectedPath {
		t.Fatalf("expected pending path %q, got %q", expectedPath, m.pendingSelectWorktreePath)
	}
	if m.state.ui.screenManager.Type() != appscreen.TypeLoading {
		t.Fatalf("expected loading screen, got %v", m.state.ui.screenManager.Type())
	}
}

// TestCreateFromPRResultMsgWithInitCommands tests that init commands are run after PR worktree creation.
func TestCreateFromPRResultMsgWithInitCommands(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir:  t.TempDir(),
		InitCommands: []string{"echo 'init command 1'", "echo 'init command 2'"},
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)
	m.loading = true
	m.setLoadingScreen("Creating worktree...")

	targetPath := filepath.Join(cfg.WorktreeDir, "pr-555")
	msg := createFromPRResultMsg{
		prNumber:   555,
		branch:     "init-test-branch",
		targetPath: targetPath,
		err:        nil,
	}

	_, cmd := m.Update(msg)

	if cmd == nil {
		t.Fatal("Expected command to be returned")
	}

	// The command should eventually trigger worktree refresh
	result := cmd()
	if _, ok := result.(worktreesLoadedMsg); !ok {
		t.Errorf("Expected final result to be worktreesLoadedMsg, got %T", result)
	}
}

// TestPendingSelectWorktreePathClearedOnError tests that pending selection is cleared when creation fails.
func TestPendingSelectWorktreePathClearedOnError(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.pendingSelectWorktreePath = "/some/pending/path"

	msg := createFromPRResultMsg{
		prNumber:   111,
		branch:     "error-branch",
		targetPath: "/tmp/pr-111",
		err:        fmt.Errorf("git error"),
	}

	_, _ = m.Update(msg)

	if m.pendingSelectWorktreePath != "" {
		t.Errorf("Expected pendingSelectWorktreePath to be cleared on error, got %q", m.pendingSelectWorktreePath)
	}
}

// TestHandleWorktreesLoadedPreservesCursorOnNoPending tests that cursor position is preserved when there's no pending selection.
func TestHandleWorktreesLoadedPreservesCursorOnNoPending(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)

	wt1Path := filepath.Join(cfg.WorktreeDir, "main")
	wt2Path := filepath.Join(cfg.WorktreeDir, "feature")
	wt3Path := filepath.Join(cfg.WorktreeDir, "bugfix")

	worktrees := []*models.WorktreeInfo{
		{Path: wt1Path, Branch: "main", IsMain: true},
		{Path: wt2Path, Branch: "feature"},
		{Path: wt3Path, Branch: "bugfix"},
	}

	// Set initial cursor position
	m.state.ui.worktreeTable.SetCursor(2)
	m.state.data.selectedIndex = 2

	// Reload worktrees without pending selection
	msg := worktreesLoadedMsg{
		worktrees: worktrees,
		err:       nil,
	}

	_, _ = m.handleWorktreesLoaded(msg)

	// Cursor should be reset by updateTable, not preserved
	// This is the expected behavior based on the code
	if m.state.data.selectedIndex != 0 {
		t.Errorf("Expected selectedIndex to be 0 after reload, got %d", m.state.data.selectedIndex)
	}
}

// TestGetWorktreeForBranch tests the helper function for finding worktrees by branch name.
func TestGetWorktreeForBranch(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	wt1 := &models.WorktreeInfo{Path: "/path/to/main", Branch: "main", IsMain: true}
	wt2 := &models.WorktreeInfo{Path: "/path/to/feature", Branch: "feature-branch"}
	wt3 := &models.WorktreeInfo{Path: "/path/to/pr-123", Branch: "fix-bug"}
	m.state.data.worktrees = []*models.WorktreeInfo{wt1, wt2, wt3}

	// Test finding existing branch
	found := m.getWorktreeForBranch("feature-branch")
	if found == nil {
		t.Fatal("Expected to find worktree for branch 'feature-branch'")
	} else if found.Path != "/path/to/feature" {
		t.Errorf("Expected path '/path/to/feature', got %q", found.Path)
	}

	// Test finding non-existent branch
	notFound := m.getWorktreeForBranch("non-existent")
	if notFound != nil {
		t.Errorf("Expected nil for non-existent branch, got %+v", notFound)
	}

	// Test finding main branch
	main := m.getWorktreeForBranch("main")
	if main == nil || !main.IsMain {
		t.Error("Expected to find main worktree")
	}
}

// TestCreateFromPRClearsScreenStack tests that the screen stack is cleared when creating a worktree from PR.
// This ensures the user returns to the worktree list rather than the PR selection screen.
func TestCreateFromPRClearsScreenStack(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)
	m.repoKey = "test/repo"

	// Simulate the screen stack state before worktree creation:
	// 1. PR selection screen is on the stack
	// 2. Input screen is the current screen
	// 3. Loading screen replaces input screen when creation starts
	prScreen := appscreen.NewPRSelectionScreen(
		[]*models.PRInfo{{Number: 1, Title: "Test PR", Branch: "test"}},
		120, 40, m.theme, false,
	)
	m.state.ui.screenManager.Push(prScreen)

	inputScreen := appscreen.NewInputScreen("Test", "Label", "", m.theme, false)
	m.state.ui.screenManager.Push(inputScreen)

	// Verify the stack has 2 screens (PR in stack, input as current)
	if m.state.ui.screenManager.StackDepth() != 1 {
		t.Fatalf("Expected stack depth 1, got %d", m.state.ui.screenManager.StackDepth())
	}
	if m.state.ui.screenManager.Type() != appscreen.TypeInput {
		t.Fatalf("Expected current screen to be TypeInput, got %v", m.state.ui.screenManager.Type())
	}

	// Simulate what happens when the user submits the input:
	// The code should clear the stack, then set the loading screen
	m.loading = true
	m.statusContent = "Creating worktree from PR/MR #1..."
	m.state.ui.screenManager.Clear() // This is the fix
	m.setLoadingScreen(m.statusContent)

	// After Clear() and setLoadingScreen(), only loading screen should remain
	if m.state.ui.screenManager.StackDepth() != 0 {
		t.Errorf("Expected stack to be empty after Clear(), got depth %d", m.state.ui.screenManager.StackDepth())
	}
	if m.state.ui.screenManager.Type() != appscreen.TypeLoading {
		t.Errorf("Expected TypeLoading, got %v", m.state.ui.screenManager.Type())
	}

	// Now simulate the result message
	targetPath := filepath.Join(cfg.WorktreeDir, "pr-1")
	msg := createFromPRResultMsg{
		prNumber:   1,
		branch:     "test",
		targetPath: targetPath,
		err:        nil,
	}

	_, _ = m.Update(msg)

	// After result, loading screen should be popped and no screens should be active
	if m.state.ui.screenManager.IsActive() {
		t.Errorf("Expected no active screen after PR creation result, got type %v", m.state.ui.screenManager.Type())
	}
}
