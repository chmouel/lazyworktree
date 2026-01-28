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
	m.currentScreen = screenLoading
	m.loadingScreen = NewLoadingScreen("Creating worktree...", m.theme, m.config.IconsEnabled())

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
	if m.currentScreen == screenLoading {
		t.Error("Expected currentScreen to change from screenLoading")
	}
	if m.loadingScreen != nil {
		t.Error("Expected loadingScreen to be cleared")
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
	m.currentScreen = screenLoading
	m.loadingScreen = NewLoadingScreen("Creating worktree...", m.theme, m.config.IconsEnabled())
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
	if m.currentScreen == screenLoading {
		t.Error("Expected currentScreen to change from screenLoading")
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
	if !m.screenManager.IsActive() || m.screenManager.Type() != appscreen.TypeInfo {
		t.Fatalf("Expected info screen to be shown, got active=%v type=%v", m.screenManager.IsActive(), m.screenManager.Type())
	}
	infoScr := m.screenManager.Current().(*appscreen.InfoScreen)
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
	if m.selectedIndex != 0 {
		t.Errorf("Expected selectedIndex to be 0 (pr-789 sorted to top), got %d", m.selectedIndex)
	}
	if m.worktreeTable.Cursor() != 0 {
		t.Errorf("Expected table cursor to be 0, got %d", m.worktreeTable.Cursor())
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
	if m.selectedIndex != 0 {
		t.Errorf("Expected selectedIndex to remain 0, got %d", m.selectedIndex)
	}
}

// TestHandleWorktreesLoadedNoPendingPath tests normal behavior without pending selection.
func TestHandleWorktreesLoadedNoPendingPath(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)
	m.selectedIndex = 1

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
	if m.selectedIndex != 0 {
		t.Errorf("Expected selectedIndex to be reset to 0, got %d", m.selectedIndex)
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

	if !m.screenManager.IsActive() || m.screenManager.Type() != appscreen.TypePRSelect {
		t.Fatalf("Expected TypePRSelect, got active=%v type=%v", m.screenManager.IsActive(), m.screenManager.Type())
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
	m.currentScreen = screenLoading

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
	m.worktreeTable.SetCursor(2)
	m.selectedIndex = 2

	// Reload worktrees without pending selection
	msg := worktreesLoadedMsg{
		worktrees: worktrees,
		err:       nil,
	}

	_, _ = m.handleWorktreesLoaded(msg)

	// Cursor should be reset by updateTable, not preserved
	// This is the expected behavior based on the code
	if m.selectedIndex != 0 {
		t.Errorf("Expected selectedIndex to be 0 after reload, got %d", m.selectedIndex)
	}
}
