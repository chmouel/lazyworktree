package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

func TestHandleCachedWorktreesUpdatesState(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	// Disable git service to bypass worktree validation in tests
	m.state.services.git = nil
	m.state.data.selectedIndex = 0
	m.state.ui.worktreeTable.SetWidth(80)

	msg := cachedWorktreesMsg{
		worktrees: []*models.WorktreeInfo{
			{Path: filepath.Join(cfg.WorktreeDir, "wt1"), Branch: "main"},
		},
	}

	_, cmd := m.handleCachedWorktrees(msg)
	if cmd != nil {
		t.Fatal("expected no command")
	}
	if len(m.state.data.worktrees) != 1 {
		t.Fatalf("expected worktrees to be set, got %d", len(m.state.data.worktrees))
	}
	if m.statusContent != loadingRefreshWorktrees {
		t.Fatalf("unexpected status content: %q", m.statusContent)
	}
	if !strings.Contains(m.infoContent, "wt1") {
		t.Fatalf("expected info content to include worktree path, got %q", m.infoContent)
	}
}

func TestHandlePRDataLoadedUpdatesTable(t *testing.T) {
	// Set default provider for testing
	SetIconProvider(&NerdFontV3Provider{})
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = t.TempDir()
	m := NewModel(cfg, "")
	m.state.ui.worktreeTable.SetWidth(100)
	m.worktreesLoaded = true
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: filepath.Join(cfg.WorktreeDir, "wt1"), Branch: "feature"},
	}
	m.state.data.filteredWts = m.state.data.worktrees
	m.state.ui.worktreeTable.SetCursor(0)

	msg := prDataLoadedMsg{
		prMap: map[string]*models.PRInfo{
			"feature": {Number: 12, State: "OPEN", Title: "Test PR", URL: "https://example.com"},
		},
	}

	_, cmd := m.handlePRDataLoaded(msg)
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}
	if !m.prDataLoaded {
		t.Fatal("expected prDataLoaded to be true")
	}
	if m.state.data.worktrees[0].PR == nil {
		t.Fatal("expected PR info to be applied to worktree")
	}
	if len(m.state.ui.worktreeTable.Columns()) != 4 {
		t.Fatalf("expected 4 columns after PR data, got %d", len(m.state.ui.worktreeTable.Columns()))
	}
	rows := m.state.ui.worktreeTable.Rows()
	if len(rows) != 1 {
		t.Fatalf("expected 1 row after PR data, got %d", len(rows))
	}
	if len(rows[0]) != 4 {
		t.Fatalf("expected 4 columns in row, got %d", len(rows[0]))
	}
	if !strings.Contains(rows[0][3], getIconPR()) {
		t.Fatalf("expected PR column to include icon %q, got %q", getIconPR(), rows[0][3])
	}
}

func TestUpdateTableHidesPRForMainWorktree(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.prDataLoaded = true
	m.state.data.worktrees = []*models.WorktreeInfo{
		{
			Path:       filepath.Join(cfg.WorktreeDir, "main"),
			Branch:     "main",
			IsMain:     true,
			LastActive: "2024-01-01",
			PR:         &models.PRInfo{Number: 377, State: "OPEN"},
		},
	}
	m.state.data.filteredWts = m.state.data.worktrees

	m.updateTable()

	rows := m.state.ui.worktreeTable.Rows()
	if len(rows) != 1 || len(rows[0]) != 4 {
		t.Fatalf("unexpected row shape: %+v", rows)
	}
	if rows[0][3] != "-" {
		t.Fatalf("expected main worktree PR column to be \"-\", got %q", rows[0][3])
	}
}

func TestHandlePRDataLoadedWithWorktreePRs(t *testing.T) {
	// Set default provider for testing
	SetIconProvider(&NerdFontV3Provider{})
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.ui.worktreeTable.SetWidth(100)
	m.worktreesLoaded = true
	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: wtPath, Branch: "local-branch-name"},
	}
	m.state.data.filteredWts = m.state.data.worktrees
	m.state.ui.worktreeTable.SetCursor(0)

	// Simulate a case where the local branch name differs from the PR's headRefName
	// So prMap won't match, but worktreePRs (from gh pr view) will
	msg := prDataLoadedMsg{
		prMap: map[string]*models.PRInfo{
			"remote-branch-name": {Number: 99, State: "OPEN", Title: "Fork PR", URL: "https://example.com"},
		},
		worktreePRs: map[string]*models.PRInfo{
			wtPath: {Number: 99, State: "OPEN", Title: "Fork PR", URL: "https://example.com"},
		},
	}

	_, cmd := m.handlePRDataLoaded(msg)
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}
	if !m.prDataLoaded {
		t.Fatal("expected prDataLoaded to be true")
	}
	if m.state.data.worktrees[0].PR == nil {
		t.Fatal("expected PR info to be applied to worktree via worktreePRs")
	}
	if m.state.data.worktrees[0].PR.Number != 99 {
		t.Fatalf("expected PR number 99, got %d", m.state.data.worktrees[0].PR.Number)
	}
}

func TestHandleCIStatusLoadedUpdatesCache(t *testing.T) {
	// Set default provider for testing
	SetIconProvider(&NerdFontV3Provider{})
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = t.TempDir()
	m := NewModel(cfg, "")
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{
			Path:   filepath.Join(cfg.WorktreeDir, "wt1"),
			Branch: "feature",
			PR: &models.PRInfo{
				Number: 1,
				State:  "OPEN",
				Title:  "Test",
				URL:    testPRURL,
			},
		},
	}
	m.state.data.selectedIndex = 0

	msg := ciStatusLoadedMsg{
		branch: "feature",
		checks: []*models.CICheck{
			{Name: "build", Status: "completed", Conclusion: "success"},
		},
	}

	_, cmd := m.handleCIStatusLoaded(msg)
	if cmd != nil {
		t.Fatal("expected no command")
	}
	if checks, _, ok := m.cache.ciCache.Get("feature"); !ok || len(checks) != 1 {
		t.Fatalf("expected CI cache to be updated, got %v", checks)
	}
	if !strings.Contains(m.infoContent, "CI Checks:") {
		t.Fatalf("expected info content to include CI checks, got %q", m.infoContent)
	}
	if !strings.Contains(m.infoContent, ciIconForConclusion("success")) {
		t.Fatalf("expected info content to include CI icon %q, got %q", ciIconForConclusion("success"), m.infoContent)
	}
}

func TestHandleWorktreesLoadedSuccess(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	wts := []*models.WorktreeInfo{
		{Path: filepath.Join(cfg.WorktreeDir, testWt1), Branch: "main"},
		{Path: filepath.Join(cfg.WorktreeDir, testWt2), Branch: testFeat},
	}

	msg := worktreesLoadedMsg{worktrees: wts, err: nil}
	updated, _ := m.handleWorktreesLoaded(msg)
	updatedModel := updated.(*Model)

	if !updatedModel.worktreesLoaded {
		t.Error("expected worktreesLoaded to be true")
	}
	if updatedModel.loading {
		t.Error("expected loading to be false")
	}
	if len(updatedModel.state.data.worktrees) != 2 {
		t.Fatalf("expected 2 worktrees, got %d", len(updatedModel.state.data.worktrees))
	}
}

func TestHandleWorktreesLoadedError(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	msg := worktreesLoadedMsg{worktrees: nil, err: os.ErrPermission}
	updated, _ := m.handleWorktreesLoaded(msg)
	updatedModel := updated.(*Model)

	if !updatedModel.worktreesLoaded {
		t.Error("expected worktreesLoaded to be true even with error")
	}
	if !updatedModel.state.ui.screenManager.IsActive() || updatedModel.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Error("expected info screen to be shown on error")
	}
}

func TestHandleWorktreesLoadedEmpty(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	msg := worktreesLoadedMsg{worktrees: []*models.WorktreeInfo{}, err: nil}
	updated, _ := m.handleWorktreesLoaded(msg)
	updatedModel := updated.(*Model)

	// WelcomeScreen is now managed by screenManager
	if !updatedModel.state.ui.screenManager.IsActive() {
		t.Error("expected screen manager to be active")
	}
	if updatedModel.state.ui.screenManager.Type() != appscreen.TypeWelcome {
		t.Errorf("expected welcome screen type, got %s", updatedModel.state.ui.screenManager.Type())
	}
}

func TestHandleWorktreesLoadedWithPendingSelection(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	wtPath := filepath.Join(cfg.WorktreeDir, testWt1)
	m.pendingSelectWorktreePath = wtPath

	wts := []*models.WorktreeInfo{
		{Path: wtPath, Branch: "main"},
	}

	msg := worktreesLoadedMsg{worktrees: wts, err: nil}
	updated, _ := m.handleWorktreesLoaded(msg)
	updatedModel := updated.(*Model)

	if updatedModel.pendingSelectWorktreePath != "" {
		t.Error("expected pendingSelectWorktreePath to be cleared")
	}
}

func TestHandleCachedWorktreesLoaded(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	// Disable git service to bypass worktree validation in tests
	m.state.services.git = nil

	wts := []*models.WorktreeInfo{
		{Path: filepath.Join(cfg.WorktreeDir, testWt1), Branch: "main"},
	}

	msg := cachedWorktreesMsg{worktrees: wts}
	updated, _ := m.handleCachedWorktrees(msg)
	updatedModel := updated.(*Model)

	if len(updatedModel.state.data.worktrees) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(updatedModel.state.data.worktrees))
	}
}

func TestHandleCachedWorktreesIgnoredWhenLoaded(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.worktreesLoaded = true

	msg := cachedWorktreesMsg{worktrees: []*models.WorktreeInfo{}}
	updated, _ := m.handleCachedWorktrees(msg)
	updatedModel := updated.(*Model)

	if len(updatedModel.state.data.worktrees) != 0 {
		t.Fatalf("expected 0 worktrees, got %d", len(updatedModel.state.data.worktrees))
	}
}

func TestHandleCachedWorktreesFiltersStaleEntries(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	validPath := filepath.Join(cfg.WorktreeDir, "main")
	mockGitWorktreeList(t, m, validPath)

	wts := []*models.WorktreeInfo{
		{Path: validPath, Branch: "main"},
		{Path: "/nonexistent/path1", Branch: "branch1"},
		{Path: "/nonexistent/path2", Branch: "branch2"},
	}

	msg := cachedWorktreesMsg{worktrees: wts}
	updated, _ := m.handleCachedWorktrees(msg)
	updatedModel := updated.(*Model)

	if len(updatedModel.state.data.worktrees) != 1 {
		t.Fatalf("expected 1 worktree after stale filtering, got %d", len(updatedModel.state.data.worktrees))
	}
	if updatedModel.state.data.worktrees[0].Path != validPath {
		t.Fatalf("expected retained worktree path %q, got %q", validPath, updatedModel.state.data.worktrees[0].Path)
	}
}

func TestHandleCachedWorktreesNormalisesPaths(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	repoPath := filepath.Join(cfg.WorktreeDir, "main")
	mockGitWorktreeList(t, m, repoPath)
	dirtyPath := repoPath + string(os.PathSeparator)
	msg := cachedWorktreesMsg{worktrees: []*models.WorktreeInfo{
		{Path: dirtyPath, Branch: "main"},
	}}
	updated, _ := m.handleCachedWorktrees(msg)
	updatedModel := updated.(*Model)

	if len(updatedModel.state.data.worktrees) != 1 {
		t.Fatalf("expected cached worktree to be retained after normalisation, got %d", len(updatedModel.state.data.worktrees))
	}
}

func TestHandlePruneResultSuccess(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.loading = true

	wts := []*models.WorktreeInfo{
		{Path: filepath.Join(cfg.WorktreeDir, testWt1), Branch: "main"},
	}

	msg := pruneResultMsg{worktrees: wts, pruned: 1, failed: 0, err: nil}
	updated, _ := m.handlePruneResult(msg)
	updatedModel := updated.(*Model)

	if updatedModel.loading {
		t.Error("expected loading to be false")
	}
	if !strings.Contains(updatedModel.statusContent, "Pruned 1") {
		t.Errorf("expected status message to contain 'Pruned 1', got %q", updatedModel.statusContent)
	}
}

func TestHandlePruneResultWithFailures(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	msg := pruneResultMsg{worktrees: []*models.WorktreeInfo{}, pruned: 2, failed: 1, err: nil}
	updated, _ := m.handlePruneResult(msg)
	updatedModel := updated.(*Model)

	if !strings.Contains(updatedModel.statusContent, "1 failed") {
		t.Errorf("expected status to include failed count, got %q", updatedModel.statusContent)
	}
}

func TestHandlePruneResultWithOrphans(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	msg := pruneResultMsg{worktrees: []*models.WorktreeInfo{}, pruned: 1, orphansDeleted: 2, failed: 0, err: nil}
	updated, _ := m.handlePruneResult(msg)
	updatedModel := updated.(*Model)

	if !strings.Contains(updatedModel.statusContent, "Pruned 1") {
		t.Errorf("expected status to include pruned count, got %q", updatedModel.statusContent)
	}
	if !strings.Contains(updatedModel.statusContent, "deleted 2 orphaned") {
		t.Errorf("expected status to include orphans deleted count, got %q", updatedModel.statusContent)
	}
}

func TestHandlePruneResultOnlyOrphans(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	msg := pruneResultMsg{worktrees: []*models.WorktreeInfo{}, pruned: 0, orphansDeleted: 3, failed: 0, err: nil}
	updated, _ := m.handlePruneResult(msg)
	updatedModel := updated.(*Model)

	if strings.Contains(updatedModel.statusContent, "Pruned 0") {
		t.Errorf("should not show 'Pruned 0' when no worktrees pruned, got %q", updatedModel.statusContent)
	}
	if !strings.Contains(updatedModel.statusContent, "deleted 3 orphaned") {
		t.Errorf("expected status to include orphans deleted count, got %q", updatedModel.statusContent)
	}
}

func TestHandlePruneResultStaleBranches(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	msg := pruneResultMsg{worktrees: []*models.WorktreeInfo{}, pruned: 1, branchesDeleted: 3, failed: 0, err: nil}
	updated, _ := m.handlePruneResult(msg)
	updatedModel := updated.(*Model)

	if !strings.Contains(updatedModel.statusContent, "Pruned 1") {
		t.Errorf("expected status to include pruned count, got %q", updatedModel.statusContent)
	}
	if !strings.Contains(updatedModel.statusContent, "deleted 3 stale branches") {
		t.Errorf("expected status to include stale branches count, got %q", updatedModel.statusContent)
	}
}

func TestHandlePruneResultOnlyStaleBranches(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	msg := pruneResultMsg{worktrees: []*models.WorktreeInfo{}, pruned: 0, branchesDeleted: 2, failed: 0, err: nil}
	updated, _ := m.handlePruneResult(msg)
	updatedModel := updated.(*Model)

	if strings.Contains(updatedModel.statusContent, "Pruned 0") {
		t.Errorf("should not show 'Pruned 0' when no worktrees pruned, got %q", updatedModel.statusContent)
	}
	if !strings.Contains(updatedModel.statusContent, "deleted 2 stale branches") {
		t.Errorf("expected status to include stale branches count, got %q", updatedModel.statusContent)
	}
}

func TestHandleAbsorbResultError(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	msg := absorbMergeResultMsg{
		path:   filepath.Join(cfg.WorktreeDir, testWt1),
		branch: "main",
		err:    os.ErrPermission,
	}
	updated, _ := m.handleAbsorbResult(msg)
	updatedModel := updated.(*Model)

	if !updatedModel.state.ui.screenManager.IsActive() || updatedModel.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Errorf("expected info screen, got active=%v type=%v", updatedModel.state.ui.screenManager.IsActive(), updatedModel.state.ui.screenManager.Type())
	}
}

func TestHandlePRDataLoadedSuccess(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: filepath.Join(cfg.WorktreeDir, "pr"), Branch: "pr-123", PR: nil},
	}

	prMap := map[string]*models.PRInfo{
		"pr-123": {Number: 123, Title: "Test PR", Branch: "pr-123"},
	}

	msg := prDataLoadedMsg{prMap: prMap, worktreePRs: nil, err: nil}
	updated, _ := m.handlePRDataLoaded(msg)
	updatedModel := updated.(*Model)

	if !updatedModel.prDataLoaded {
		t.Error("expected prDataLoaded to be true")
	}
	if updatedModel.state.data.worktrees[0].PR == nil {
		t.Error("expected PR to be assigned to worktree")
	}
}

func TestHandlePRDataLoadedError(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	msg := prDataLoadedMsg{prMap: nil, worktreePRs: nil, err: os.ErrPermission}
	updated, cmd := m.handlePRDataLoaded(msg)
	updatedModel := updated.(*Model)

	if updatedModel.prDataLoaded {
		t.Error("expected prDataLoaded to be false on error")
	}
	if cmd != nil {
		t.Error("expected nil command on error")
	}
}

func TestHandleCIStatusLoadedSuccess(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	checks := []*models.CICheck{
		{Name: "build", Conclusion: "success"},
	}

	msg := ciStatusLoadedMsg{branch: "main", checks: checks, err: nil}
	updated, _ := m.handleCIStatusLoaded(msg)
	updatedModel := updated.(*Model)

	if _, _, ok := updatedModel.cache.ciCache.Get("main"); !ok {
		t.Error("expected CI status to be cached")
	}
}

func TestHandleCIStatusLoadedError(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	msg := ciStatusLoadedMsg{branch: "main", checks: nil, err: os.ErrPermission}
	updated, _ := m.handleCIStatusLoaded(msg)
	updatedModel := updated.(*Model)

	if _, _, ok := updatedModel.cache.ciCache.Get("main"); ok {
		t.Error("expected CI status not to be cached on error")
	}
}

func TestHandleOpenPRsLoadedEmpty(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	msg := openPRsLoadedMsg{prs: []*models.PRInfo{}, err: nil}
	cmd := m.handleOpenPRsLoaded(msg)

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Error("expected info screen to be shown for empty PRs")
	}
	if cmd != nil {
		t.Error("expected nil command")
	}
}

func TestHandleOpenPRsLoadedError(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	msg := openPRsLoadedMsg{prs: nil, err: os.ErrPermission}
	cmd := m.handleOpenPRsLoaded(msg)

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Error("expected info screen to be shown on error")
	}
	if cmd != nil {
		t.Error("expected nil command")
	}
}

func TestHandleOpenIssuesLoadedEmpty(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	msg := openIssuesLoadedMsg{issues: []*models.IssueInfo{}, err: nil}
	cmd := m.handleOpenIssuesLoaded(msg)

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Error("expected info screen to be shown for empty issues")
	}
	if cmd != nil {
		t.Error("expected nil command")
	}
}

func TestHandleOpenIssuesLoadedError(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	msg := openIssuesLoadedMsg{issues: nil, err: os.ErrPermission}
	cmd := m.handleOpenIssuesLoaded(msg)

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Error("expected info screen to be shown on error")
	}
	if cmd != nil {
		t.Error("expected nil command")
	}
}

// TestPRAssignmentsPreservedOnWorktreeRefresh verifies that PR assignments
// are preserved when worktrees are refreshed (e.g., due to git file watcher).
// This is a regression test for a bug where PR data was lost on auto-refresh.

// TestPRAssignmentsPreservedOnWorktreeRefresh verifies that PR assignments
// are preserved when worktrees are refreshed (e.g., due to git file watcher).
// This is a regression test for a bug where PR data was lost on auto-refresh.
func TestPRAssignmentsPreservedOnWorktreeRefresh(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = t.TempDir()
	m := NewModel(cfg, "")
	m.state.ui.worktreeTable.SetWidth(100)
	m.worktreesLoaded = true

	// Create initial worktrees
	wt1 := &models.WorktreeInfo{
		Path:   filepath.Join(cfg.WorktreeDir, "wt1"),
		Branch: "feature-1",
	}
	wt2 := &models.WorktreeInfo{
		Path:   filepath.Join(cfg.WorktreeDir, "wt2"),
		Branch: "feature-2",
	}
	m.state.data.worktrees = []*models.WorktreeInfo{wt1, wt2}
	m.state.data.filteredWts = m.state.data.worktrees

	// Assign PR data to worktrees
	prMsg := prDataLoadedMsg{
		prMap: map[string]*models.PRInfo{
			"feature-1": {Number: 123, State: "OPEN", Title: "PR 123", URL: "https://example.com/123"},
		},
		worktreePRs: map[string]*models.PRInfo{
			wt2.Path: {Number: 456, State: "MERGED", Title: "PR 456", URL: "https://example.com/456"},
		},
		worktreeErrors: map[string]string{},
	}
	m.handlePRDataLoaded(prMsg)

	// Verify PR assignments
	if m.state.data.worktrees[0].PR == nil || m.state.data.worktrees[0].PR.Number != 123 {
		t.Fatal("expected wt1 to have PR #123 assigned")
	}
	if m.state.data.worktrees[1].PR == nil || m.state.data.worktrees[1].PR.Number != 456 {
		t.Fatal("expected wt2 to have PR #456 assigned")
	}
	if m.state.data.worktrees[0].PRFetchStatus != "loaded" {
		t.Fatalf("expected wt1 PRFetchStatus='loaded', got %q", m.state.data.worktrees[0].PRFetchStatus)
	}

	// Simulate worktree refresh (e.g., from git file watcher)
	// This creates NEW WorktreeInfo objects
	refreshedWt1 := &models.WorktreeInfo{
		Path:   filepath.Join(cfg.WorktreeDir, "wt1"),
		Branch: "feature-1",
	}
	refreshedWt2 := &models.WorktreeInfo{
		Path:   filepath.Join(cfg.WorktreeDir, "wt2"),
		Branch: "feature-2",
	}
	refreshMsg := worktreesLoadedMsg{
		worktrees: []*models.WorktreeInfo{refreshedWt1, refreshedWt2},
		err:       nil,
	}
	m.handleWorktreesLoaded(refreshMsg)
	if m.state.data.worktrees[0].PR == nil || m.state.data.worktrees[0].PR.Number != 123 {
		t.Fatal("PR assignment was lost on worktree refresh! wt1 should still have PR #123")
	}
	if m.state.data.worktrees[1].PR == nil || m.state.data.worktrees[1].PR.Number != 456 {
		t.Fatal("PR assignment was lost on worktree refresh! wt2 should still have PR #456")
	}
	if m.state.data.worktrees[0].PRFetchStatus != "loaded" {
		t.Fatalf("PRFetchStatus was lost on refresh! expected 'loaded', got %q", m.state.data.worktrees[0].PRFetchStatus)
	}

	// Verify table row data includes PR info
	rows := m.state.ui.worktreeTable.Rows()
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	// With prDataLoaded=true, rows should have 4 columns including PR
	if len(rows[0]) != 4 {
		t.Fatalf("expected 4 columns in row (including PR), got %d", len(rows[0]))
	}
	// PR column should not be "-"
	prColumn := rows[0][3] // 4th column is PR
	if prColumn == "-" {
		t.Fatal("PR column shows '-' instead of PR data after refresh")
	}
	if !strings.Contains(prColumn, "123") {
		t.Fatalf("PR column should contain '123', got %q", prColumn)
	}
}

// TestPRFetchErrorsPreservedOnWorktreeRefresh verifies that PR fetch errors
// are preserved when worktrees are refreshed, not just successful PR assignments.

// TestPRFetchErrorsPreservedOnWorktreeRefresh verifies that PR fetch errors
// are preserved when worktrees are refreshed, not just successful PR assignments.
func TestPRFetchErrorsPreservedOnWorktreeRefresh(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = t.TempDir()
	m := NewModel(cfg, "")
	m.state.ui.worktreeTable.SetWidth(100)
	m.worktreesLoaded = true

	// Create initial worktrees
	wt1 := &models.WorktreeInfo{
		Path:   filepath.Join(cfg.WorktreeDir, "wt1"),
		Branch: "feature-1",
	}
	m.state.data.worktrees = []*models.WorktreeInfo{wt1}
	m.state.data.filteredWts = m.state.data.worktrees

	// Simulate PR fetch with error
	prMsg := prDataLoadedMsg{
		prMap:       map[string]*models.PRInfo{},
		worktreePRs: map[string]*models.PRInfo{},
		worktreeErrors: map[string]string{
			wt1.Path: "gh CLI not found in PATH",
		},
	}
	m.handlePRDataLoaded(prMsg)

	// Verify error was recorded
	if m.state.data.worktrees[0].PRFetchError != "gh CLI not found in PATH" {
		t.Fatalf("expected PRFetchError to be set, got %q", m.state.data.worktrees[0].PRFetchError)
	}
	if m.state.data.worktrees[0].PRFetchStatus != "error" {
		t.Fatalf("expected PRFetchStatus='error', got %q", m.state.data.worktrees[0].PRFetchStatus)
	}

	// Simulate worktree refresh
	refreshedWt1 := &models.WorktreeInfo{
		Path:   filepath.Join(cfg.WorktreeDir, "wt1"),
		Branch: "feature-1",
	}
	refreshMsg := worktreesLoadedMsg{
		worktrees: []*models.WorktreeInfo{refreshedWt1},
		err:       nil,
	}
	m.handleWorktreesLoaded(refreshMsg)
	if m.state.data.worktrees[0].PRFetchError != "gh CLI not found in PATH" {
		t.Fatalf("PRFetchError was lost on refresh! expected error message, got %q", m.state.data.worktrees[0].PRFetchError)
	}
	if m.state.data.worktrees[0].PRFetchStatus != "error" {
		t.Fatalf("PRFetchStatus was lost on refresh! expected 'error', got %q", m.state.data.worktrees[0].PRFetchStatus)
	}
}

// TestPRStatePreservedOnCachedWorktrees verifies that PR state (PR info, errors, status)
// is preserved when cached worktrees are loaded.

// TestPRStatePreservedOnCachedWorktrees verifies that PR state (PR info, errors, status)
// is preserved when cached worktrees are loaded.
func TestPRStatePreservedOnCachedWorktrees(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = t.TempDir()
	m := NewModel(cfg, "")
	m.state.ui.worktreeTable.SetWidth(100)

	// Create initial worktree with PR data
	wt1 := &models.WorktreeInfo{
		Path:          filepath.Join(cfg.WorktreeDir, "wt1"),
		Branch:        "feature-1",
		PR:            &models.PRInfo{Number: 42, State: "OPEN", Title: "Test PR"},
		PRFetchError:  "some error",
		PRFetchStatus: models.PRFetchStatusError,
	}
	m.state.data.worktrees = []*models.WorktreeInfo{wt1}

	// Simulate cached worktrees load (fresh worktree objects without PR data)
	cachedWt1 := &models.WorktreeInfo{
		Path:   filepath.Join(cfg.WorktreeDir, "wt1"),
		Branch: "feature-1",
	}
	msg := cachedWorktreesMsg{worktrees: []*models.WorktreeInfo{cachedWt1}}
	m.handleCachedWorktrees(msg)

	// Verify PR state was preserved
	if m.state.data.worktrees[0].PR == nil || m.state.data.worktrees[0].PR.Number != 42 {
		t.Fatal("PR info was lost on cached worktree load")
	}
	if m.state.data.worktrees[0].PRFetchError != "some error" {
		t.Fatalf("PRFetchError was lost! expected 'some error', got %q", m.state.data.worktrees[0].PRFetchError)
	}
	if m.state.data.worktrees[0].PRFetchStatus != models.PRFetchStatusError {
		t.Fatalf("PRFetchStatus was lost! expected 'error', got %q", m.state.data.worktrees[0].PRFetchStatus)
	}
}

// TestPRStatePreservedOnPruneResult verifies that PR state (PR info, errors, status)
// is preserved when worktrees are updated after pruning.

// TestPRStatePreservedOnPruneResult verifies that PR state (PR info, errors, status)
// is preserved when worktrees are updated after pruning.
func TestPRStatePreservedOnPruneResult(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = t.TempDir()
	m := NewModel(cfg, "")
	m.state.ui.worktreeTable.SetWidth(100)

	// Create initial worktrees with PR data
	wt1 := &models.WorktreeInfo{
		Path:          filepath.Join(cfg.WorktreeDir, "wt1"),
		Branch:        "feature-1",
		PR:            &models.PRInfo{Number: 123, State: "OPEN", Title: "Feature PR"},
		PRFetchStatus: models.PRFetchStatusLoaded,
	}
	wt2 := &models.WorktreeInfo{
		Path:          filepath.Join(cfg.WorktreeDir, "wt2"),
		Branch:        "feature-2",
		PRFetchError:  "gh CLI not found",
		PRFetchStatus: models.PRFetchStatusError,
	}
	m.state.data.worktrees = []*models.WorktreeInfo{wt1, wt2}

	// Simulate prune result (fresh worktree objects without PR data)
	prunedWt1 := &models.WorktreeInfo{
		Path:   filepath.Join(cfg.WorktreeDir, "wt1"),
		Branch: "feature-1",
	}
	prunedWt2 := &models.WorktreeInfo{
		Path:   filepath.Join(cfg.WorktreeDir, "wt2"),
		Branch: "feature-2",
	}
	msg := pruneResultMsg{
		worktrees: []*models.WorktreeInfo{prunedWt1, prunedWt2},
		pruned:    0,
		failed:    0,
		err:       nil,
	}
	m.handlePruneResult(msg)

	// Verify PR state was preserved for both worktrees
	if m.state.data.worktrees[0].PR == nil || m.state.data.worktrees[0].PR.Number != 123 {
		t.Fatal("PR info was lost on prune result for wt1")
	}
	if m.state.data.worktrees[0].PRFetchStatus != models.PRFetchStatusLoaded {
		t.Fatalf("PRFetchStatus was lost for wt1! expected 'loaded', got %q", m.state.data.worktrees[0].PRFetchStatus)
	}

	if m.state.data.worktrees[1].PRFetchError != "gh CLI not found" {
		t.Fatalf("PRFetchError was lost for wt2! expected 'gh CLI not found', got %q", m.state.data.worktrees[1].PRFetchError)
	}
	if m.state.data.worktrees[1].PRFetchStatus != models.PRFetchStatusError {
		t.Fatalf("PRFetchStatus was lost for wt2! expected 'error', got %q", m.state.data.worktrees[1].PRFetchStatus)
	}
}

// TestPRRefreshClearsPreservedState verifies that when PR data is explicitly refreshed
// (pressing 'p'), the new PR data properly replaces any previously preserved PR state.

// TestPRRefreshClearsPreservedState verifies that when PR data is explicitly refreshed
// (pressing 'p'), the new PR data properly replaces any previously preserved PR state.
func TestPRRefreshClearsPreservedState(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = t.TempDir()
	m := NewModel(cfg, "")
	m.state.ui.worktreeTable.SetWidth(100)
	m.worktreesLoaded = true

	// Create worktree with OLD PR data
	wt1 := &models.WorktreeInfo{
		Path:          filepath.Join(cfg.WorktreeDir, "wt1"),
		Branch:        "feature-1",
		PR:            &models.PRInfo{Number: 99, State: "OPEN", Title: "Old PR"},
		PRFetchStatus: models.PRFetchStatusLoaded,
	}
	m.state.data.worktrees = []*models.WorktreeInfo{wt1}

	// Simulate worktree refresh that preserves the OLD PR data
	refreshedWt1 := &models.WorktreeInfo{
		Path:   filepath.Join(cfg.WorktreeDir, "wt1"),
		Branch: "feature-1",
	}
	refreshMsg := worktreesLoadedMsg{
		worktrees: []*models.WorktreeInfo{refreshedWt1},
		err:       nil,
	}
	m.handleWorktreesLoaded(refreshMsg)

	// Verify OLD PR data was preserved after worktree refresh
	if m.state.data.worktrees[0].PR == nil || m.state.data.worktrees[0].PR.Number != 99 {
		t.Fatal("PR data should be preserved after worktree refresh")
	}

	// Now simulate PR refresh (pressing 'p') with NEW PR data
	prMsg := prDataLoadedMsg{
		prMap: map[string]*models.PRInfo{
			"feature-1": {Number: 123, State: "OPEN", Title: "New PR"},
		},
		worktreePRs:    map[string]*models.PRInfo{},
		worktreeErrors: map[string]string{},
	}
	m.handlePRDataLoaded(prMsg)
	if m.state.data.worktrees[0].PR == nil {
		t.Fatal("PR data should be set after PR refresh")
	}
	if m.state.data.worktrees[0].PR.Number != 123 {
		t.Fatalf("PR refresh should replace old data! expected PR#123, got PR#%d", m.state.data.worktrees[0].PR.Number)
	}
	if m.state.data.worktrees[0].PR.Title != "New PR" {
		t.Fatalf("PR refresh should replace old data! expected 'New PR', got %q", m.state.data.worktrees[0].PR.Title)
	}
	if m.state.data.worktrees[0].PRFetchStatus != models.PRFetchStatusLoaded {
		t.Fatalf("PRFetchStatus should be 'loaded', got %q", m.state.data.worktrees[0].PRFetchStatus)
	}
}

// TestPRDataResetSyncsRowsAndColumns verifies that when prDataLoaded is reset
// to false (e.g., pressing 'p' to refetch), rows and columns are properly
// synchronized to prevent index out of range panics.

// TestPRDataResetSyncsRowsAndColumns verifies that when prDataLoaded is reset
// to false (e.g., pressing 'p' to refetch), rows and columns are properly
// synchronized to prevent index out of range panics.
func TestPRDataResetSyncsRowsAndColumns(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = t.TempDir()
	m := NewModel(cfg, "")
	m.state.ui.worktreeTable.SetWidth(100)
	m.worktreesLoaded = true
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: filepath.Join(cfg.WorktreeDir, "wt1"), Branch: "feature"},
	}
	m.state.data.filteredWts = m.state.data.worktrees

	// First, load PR data (4 columns)
	prMsg := prDataLoadedMsg{
		prMap: map[string]*models.PRInfo{
			"feature": {Number: 12, State: "OPEN", Title: "Test PR", URL: "https://example.com"},
		},
	}
	m.handlePRDataLoaded(prMsg)

	if !m.prDataLoaded {
		t.Fatal("expected prDataLoaded to be true after loading PR data")
	}
	if len(m.state.ui.worktreeTable.Columns()) != 4 {
		t.Fatalf("expected 4 columns after PR data, got %d", len(m.state.ui.worktreeTable.Columns()))
	}
	rows := m.state.ui.worktreeTable.Rows()
	if len(rows[0]) != 4 {
		t.Fatalf("expected 4 values in row after PR data, got %d", len(rows[0]))
	}

	// Now simulate pressing 'p' to refetch - this should reset to 3 columns
	m.cache.ciCache.Clear()
	m.prDataLoaded = false
	m.updateTable()
	m.updateTableColumns(m.state.ui.worktreeTable.Width())

	if m.prDataLoaded {
		t.Fatal("expected prDataLoaded to be false after reset")
	}
	if len(m.state.ui.worktreeTable.Columns()) != 3 {
		t.Fatalf("expected 3 columns after reset, got %d", len(m.state.ui.worktreeTable.Columns()))
	}
	rows = m.state.ui.worktreeTable.Rows()
	if len(rows[0]) != 3 {
		t.Fatalf("expected 3 values in row after reset, got %d", len(rows[0]))
	}
}

// TestPRDataLoadedSyncAfterWorktreeReload verifies prDataLoaded is set when PR state is restored.
func TestPRDataLoadedSyncAfterWorktreeReload(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	// Set up initial worktrees with PR data
	initialWts := []*models.WorktreeInfo{
		{Path: filepath.Join(cfg.WorktreeDir, "wt1"), Branch: "feature-1", PR: &models.PRInfo{Number: 42, Title: "Test PR"}},
		{Path: filepath.Join(cfg.WorktreeDir, "wt2"), Branch: "feature-2"},
	}
	m.state.data.worktrees = initialWts
	m.prDataLoaded = true

	m.prDataLoaded = false

	newWts := []*models.WorktreeInfo{
		{Path: filepath.Join(cfg.WorktreeDir, "wt1"), Branch: "feature-1"},
		{Path: filepath.Join(cfg.WorktreeDir, "wt2"), Branch: "feature-2"},
	}
	msg := worktreesLoadedMsg{worktrees: newWts}
	updated, _ := m.handleWorktreesLoaded(msg)
	updatedModel := updated.(*Model)

	// Verify PR state was restored
	var restoredPR *models.PRInfo
	for _, wt := range updatedModel.state.data.worktrees {
		if wt.Path == filepath.Join(cfg.WorktreeDir, "wt1") {
			restoredPR = wt.PR
			break
		}
	}
	if restoredPR == nil {
		t.Fatal("expected PR to be restored to wt1")
	} else if restoredPR.Number != 42 {
		t.Fatalf("expected PR number 42, got %d", restoredPR.Number)
	}

	if !updatedModel.prDataLoaded {
		t.Fatal("expected prDataLoaded to be true after restoring PR state")
	}
}

// TestPRDataLoadedNotSetWhenNoPRData verifies prDataLoaded remains false when no PR data exists.

// TestPRDataLoadedNotSetWhenNoPRData verifies prDataLoaded remains false when no PR data exists.
func TestPRDataLoadedNotSetWhenNoPRData(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	// Set up worktrees without PR data
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: filepath.Join(cfg.WorktreeDir, "wt1"), Branch: "feature-1"},
	}
	m.prDataLoaded = false

	// Load new worktrees without PR data
	newWts := []*models.WorktreeInfo{
		{Path: filepath.Join(cfg.WorktreeDir, "wt1"), Branch: "feature-1"},
	}
	msg := worktreesLoadedMsg{worktrees: newWts}
	updated, _ := m.handleWorktreesLoaded(msg)
	updatedModel := updated.(*Model)

	// prDataLoaded should remain false since no PR data exists
	if updatedModel.prDataLoaded {
		t.Fatal("expected prDataLoaded to remain false when no PR data exists")
	}
}

// TestPRDataLoadedSyncAfterCachedWorktrees verifies prDataLoaded syncs with cached worktrees.

// TestPRDataLoadedSyncAfterCachedWorktrees verifies prDataLoaded syncs with cached worktrees.
func TestPRDataLoadedSyncAfterCachedWorktrees(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	// Disable git service to bypass worktree validation in tests
	m.state.services.git = nil

	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: filepath.Join(cfg.WorktreeDir, "wt1"), Branch: "feature-1", PR: &models.PRInfo{Number: 123}},
	}
	m.prDataLoaded = false
	m.worktreesLoaded = false

	newWts := []*models.WorktreeInfo{
		{Path: filepath.Join(cfg.WorktreeDir, "wt1"), Branch: "feature-1"},
	}
	msg := cachedWorktreesMsg{worktrees: newWts}
	updated, _ := m.handleCachedWorktrees(msg)
	updatedModel := updated.(*Model)

	if !updatedModel.prDataLoaded {
		t.Fatal("expected prDataLoaded to be true after restoring PR state from cache")
	}
}

// TestPRDataLoadedSyncAfterPruneResult verifies prDataLoaded syncs with prune results.

// TestPRDataLoadedSyncAfterPruneResult verifies prDataLoaded syncs with prune results.
func TestPRDataLoadedSyncAfterPruneResult(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.loading = true

	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: filepath.Join(cfg.WorktreeDir, "wt1"), Branch: "feature-1", PR: &models.PRInfo{Number: 456}},
	}
	m.prDataLoaded = false

	newWts := []*models.WorktreeInfo{
		{Path: filepath.Join(cfg.WorktreeDir, "wt1"), Branch: "feature-1"},
	}
	msg := pruneResultMsg{worktrees: newWts, pruned: 1}
	updated, _ := m.handlePruneResult(msg)
	updatedModel := updated.(*Model)

	if !updatedModel.prDataLoaded {
		t.Fatal("expected prDataLoaded to be true after restoring PR state from prune")
	}
}

// TestHandleSinglePRLoadedUpdatesPRData verifies handleSinglePRLoaded updates PR data.

// TestHandleSinglePRLoadedUpdatesPRData verifies handleSinglePRLoaded updates PR data.
func TestHandleSinglePRLoadedUpdatesPRData(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: wtPath, Branch: "feature-1"},
	}
	m.state.data.filteredWts = m.state.data.worktrees
	m.state.data.selectedIndex = 0
	m.prDataLoaded = false

	msg := singlePRLoadedMsg{
		worktreePath: wtPath,
		pr:           &models.PRInfo{Number: 123, Title: "Test PR"},
		err:          nil,
	}
	updated, _ := m.handleSinglePRLoaded(msg)
	updatedModel := updated.(*Model)

	// Verify PR was set on the worktree
	if updatedModel.state.data.worktrees[0].PR == nil {
		t.Fatal("expected PR to be set on worktree")
	}
	if updatedModel.state.data.worktrees[0].PR.Number != 123 {
		t.Fatalf("expected PR number 123, got %d", updatedModel.state.data.worktrees[0].PR.Number)
	}
	if updatedModel.state.data.worktrees[0].PRFetchStatus != models.PRFetchStatusLoaded {
		t.Fatalf("expected PRFetchStatus to be Loaded, got %v", updatedModel.state.data.worktrees[0].PRFetchStatus)
	}
	if !updatedModel.prDataLoaded {
		t.Fatal("expected prDataLoaded to be true after receiving PR data")
	}
}

// TestHandleSinglePRLoadedHandlesError verifies handleSinglePRLoaded handles fetch errors.

// TestHandleSinglePRLoadedHandlesError verifies handleSinglePRLoaded handles fetch errors.
func TestHandleSinglePRLoadedHandlesError(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: wtPath, Branch: "feature-1"},
	}
	m.state.data.filteredWts = m.state.data.worktrees
	m.state.data.selectedIndex = 0

	msg := singlePRLoadedMsg{
		worktreePath: wtPath,
		pr:           nil,
		err:          context.DeadlineExceeded,
	}
	updated, _ := m.handleSinglePRLoaded(msg)
	updatedModel := updated.(*Model)

	if updatedModel.state.data.worktrees[0].PRFetchStatus != models.PRFetchStatusError {
		t.Fatalf("expected PRFetchStatus to be Error, got %v", updatedModel.state.data.worktrees[0].PRFetchStatus)
	}
	if updatedModel.state.data.worktrees[0].PRFetchError == "" {
		t.Fatal("expected PRFetchError to be set")
	}
}

// TestHandleSinglePRLoadedNoPR verifies handleSinglePRLoaded handles no PR case.

// TestHandleSinglePRLoadedNoPR verifies handleSinglePRLoaded handles no PR case.
func TestHandleSinglePRLoadedNoPR(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: wtPath, Branch: "feature-1"},
	}
	m.state.data.filteredWts = m.state.data.worktrees
	m.state.data.selectedIndex = 0

	msg := singlePRLoadedMsg{
		worktreePath: wtPath,
		pr:           nil,
		err:          nil,
	}
	updated, _ := m.handleSinglePRLoaded(msg)
	updatedModel := updated.(*Model)

	if updatedModel.state.data.worktrees[0].PRFetchStatus != models.PRFetchStatusNoPR {
		t.Fatalf("expected PRFetchStatus to be NoPR, got %v", updatedModel.state.data.worktrees[0].PRFetchStatus)
	}
	if updatedModel.state.data.worktrees[0].PR != nil {
		t.Fatal("expected PR to be nil")
	}
}

func TestDisablePRFetchPRDataReturnsNil(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		DisablePR:   true,
	}
	m := NewModel(cfg, "")

	cmd := m.fetchPRData()
	if cmd != nil {
		t.Error("expected fetchPRData to return nil when DisablePR is true")
	}
}

func TestDisablePRRefreshCurrentWorktreePRReturnsNil(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		DisablePR:   true,
	}
	m := NewModel(cfg, "")
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: filepath.Join(cfg.WorktreeDir, "test"), Branch: "test"},
	}
	m.state.data.filteredWts = m.state.data.worktrees
	m.state.data.selectedIndex = 0

	cmd := m.refreshCurrentWorktreePR()
	if cmd != nil {
		t.Error("expected refreshCurrentWorktreePR to return nil when DisablePR is true")
	}
}

func TestDisablePRMaybeFetchCIStatusReturnsNil(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		DisablePR:   true,
	}
	m := NewModel(cfg, "")
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: filepath.Join(cfg.WorktreeDir, "test"), Branch: "test"},
	}
	m.state.data.filteredWts = m.state.data.worktrees
	m.state.data.selectedIndex = 0

	cmd := m.maybeFetchCIStatus()
	if cmd != nil {
		t.Error("expected maybeFetchCIStatus to return nil when DisablePR is true")
	}
}

func TestDisablePRShouldRefreshCIReturnsFalse(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		DisablePR:   true,
	}
	m := NewModel(cfg, "")
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: filepath.Join(cfg.WorktreeDir, "test"), Branch: "test"},
	}
	m.state.data.filteredWts = m.state.data.worktrees
	m.state.data.selectedIndex = 0

	if m.shouldRefreshCI() {
		t.Error("expected shouldRefreshCI to return false when DisablePR is true")
	}
}

func TestDisablePRTableColumnsExcludesPRColumn(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		DisablePR:   true,
	}
	m := NewModel(cfg, "")
	m.prDataLoaded = true // Even with PR data loaded, DisablePR should hide column

	m.updateTableColumns(100)

	columns := m.state.ui.worktreeTable.Columns()
	for _, col := range columns {
		if col.Title == "PR" {
			t.Error("expected PR column to be excluded when DisablePR is true")
		}
	}
}

func TestDisablePRTableRowsExcludesPRColumn(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		DisablePR:   true,
	}
	m := NewModel(cfg, "")
	m.prDataLoaded = true
	m.state.data.worktrees = []*models.WorktreeInfo{
		{
			Path:       filepath.Join(cfg.WorktreeDir, "test"),
			Branch:     "test",
			LastActive: "2024-01-01",
			PR:         &models.PRInfo{Number: 123},
		},
	}
	m.state.data.filteredWts = m.state.data.worktrees

	m.updateTable()

	rows := m.state.ui.worktreeTable.Rows()
	if len(rows) > 0 && len(rows[0]) > 3 {
		t.Errorf("expected 3 columns when DisablePR is true, got %d", len(rows[0]))
	}
}
