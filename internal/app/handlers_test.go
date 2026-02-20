package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/app/services"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

const (
	testFeat         = "feat"
	testGitCmd       = "git"
	testGitPullArg   = "pull"
	testGitPushArg   = "push"
	testRemoteOrigin = "origin"
	testUpstreamRef  = "origin/feature"
	testOtherBranch  = "origin/other"
	testWt1          = "wt1"
	testWt2          = "wt2"
	testReadme       = "README.md"
	testFilterQuery  = "test"
)

func TestHandlePageDownUpOnStatusPane(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1
	m.state.ui.statusViewport = viewport.New(10, 2)
	m.state.ui.statusViewport.SetContent(strings.Repeat("line\n", 10))

	start := m.state.ui.statusViewport.YOffset
	_, _ = m.handlePageDown(tea.KeyMsg{Type: tea.KeyPgDown})
	if m.state.ui.statusViewport.YOffset <= start {
		t.Fatalf("expected YOffset to increase, got %d", m.state.ui.statusViewport.YOffset)
	}

	m.state.ui.statusViewport.YOffset = 2
	_, _ = m.handlePageUp(tea.KeyMsg{Type: tea.KeyPgUp})
	if m.state.ui.statusViewport.YOffset >= 2 {
		t.Fatalf("expected YOffset to decrease, got %d", m.state.ui.statusViewport.YOffset)
	}
}

func TestHandleEnterKeySelectsWorktree(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 0
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: filepath.Join(cfg.WorktreeDir, "wt"), Branch: testFeat},
	}
	m.state.data.selectedIndex = 0

	_, cmd := m.handleEnterKey()
	if m.selectedPath == "" {
		t.Fatal("expected selected path to be set")
	}
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}
}

func TestEnterAfterNavigationUsesHighlightedWorktree(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 0
	m.state.ui.worktreeTable.Focus()

	firstPath := filepath.Join(cfg.WorktreeDir, "wt1")
	secondPath := filepath.Join(cfg.WorktreeDir, "wt2")
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: firstPath, Branch: testFeat},
		{Path: secondPath, Branch: testOtherBranch},
	}
	m.state.ui.worktreeTable.SetRows([]table.Row{
		{"wt1"},
		{"wt2"},
	})
	m.state.ui.worktreeTable.SetCursor(0)
	m.state.data.selectedIndex = 0

	_, _ = m.handleNavigationDown(tea.KeyMsg{Type: tea.KeyDown})
	if m.state.ui.worktreeTable.Cursor() != 1 {
		t.Fatalf("expected cursor to move to 1, got %d", m.state.ui.worktreeTable.Cursor())
	}

	_, cmd := m.handleEnterKey()
	if m.selectedPath != secondPath {
		t.Fatalf("expected selected path %q, got %q", secondPath, m.selectedPath)
	}
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}
}

func TestFilterEnterClosesWithoutSelecting(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		SortMode:    "path",
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 0

	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: filepath.Join(cfg.WorktreeDir, "b-worktree"), Branch: testFeat},
		{Path: filepath.Join(cfg.WorktreeDir, "a-worktree"), Branch: testFeat},
	}
	m.state.services.filter.FilterQuery = testFeat
	m.state.ui.filterInput.SetValue(testFeat)
	m.updateTable()
	m.state.view.ShowingFilter = true
	m.state.ui.filterInput.Focus()
	m.state.ui.worktreeTable.SetCursor(1)
	m.state.data.selectedIndex = 1

	updated, cmd := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyEnter})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	if cmd != nil {
		t.Fatal("expected no command to be returned")
	}
	if m.state.view.ShowingFilter {
		t.Fatal("expected filter to be closed")
	}
	if m.selectedPath != "" {
		t.Fatalf("expected selected path to remain empty, got %q", m.selectedPath)
	}
}

func TestFilterAltNPMovesSelectionAndFills(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		SortMode:    "path",
	}
	m := NewModel(cfg, "")

	wt1Path := filepath.Join(cfg.WorktreeDir, testWt1)
	wt2Path := filepath.Join(cfg.WorktreeDir, testWt2)
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: wt1Path, Branch: "feat-one"},
		{Path: wt2Path, Branch: "feat-two"},
	}
	m.state.services.filter.FilterQuery = testFeat
	m.state.ui.filterInput.SetValue(testFeat)
	m.updateTable()
	m.state.view.ShowingFilter = true
	m.state.ui.filterInput.Focus()
	m.state.ui.worktreeTable.SetCursor(0)
	m.state.data.selectedIndex = 0

	updated, _ := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}, Alt: true})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	if m.state.ui.filterInput.Value() != testWt2 || m.state.services.filter.FilterQuery != testWt2 {
		t.Fatalf("expected filter query to match selected worktree, got %q", m.state.services.filter.FilterQuery)
	}
	if len(m.state.data.filteredWts) != 1 || m.state.data.filteredWts[0].Path != wt2Path {
		t.Fatalf("expected filtered worktree %q, got %v", wt2Path, m.state.data.filteredWts)
	}

	updated, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}, Alt: true})
	updatedModel, ok = updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	if m.state.ui.filterInput.Value() != testWt1 || m.state.services.filter.FilterQuery != testWt1 {
		t.Fatalf("expected filter query to match selected worktree, got %q", m.state.services.filter.FilterQuery)
	}
	if len(m.state.data.filteredWts) != 1 || m.state.data.filteredWts[0].Path != wt1Path {
		t.Fatalf("expected filtered worktree %q, got %v", wt1Path, m.state.data.filteredWts)
	}
}

func TestFilterArrowKeysNavigateWithoutFilling(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		SortMode:    "path",
	}
	m := NewModel(cfg, "")

	wt1Path := filepath.Join(cfg.WorktreeDir, testWt1)
	wt2Path := filepath.Join(cfg.WorktreeDir, testWt2)
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: wt1Path, Branch: "feat-one"},
		{Path: wt2Path, Branch: "feat-two"},
	}
	m.state.services.filter.FilterQuery = testFeat
	m.state.ui.filterInput.SetValue(testFeat)
	m.updateTable()
	m.state.view.ShowingFilter = true
	m.state.ui.filterInput.Focus()
	m.state.ui.worktreeTable.SetCursor(0)
	m.state.data.selectedIndex = 0

	updated, _ := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyDown})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	if m.state.ui.filterInput.Value() != testFeat || m.state.services.filter.FilterQuery != testFeat {
		t.Fatalf("expected filter query unchanged, got %q", m.state.services.filter.FilterQuery)
	}

	updated, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyUp})
	updatedModel, ok = updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	if m.state.ui.filterInput.Value() != testFeat || m.state.services.filter.FilterQuery != testFeat {
		t.Fatalf("expected filter query unchanged, got %q", m.state.services.filter.FilterQuery)
	}
}

func TestFilterEmptyEnterSelectsCurrent(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		SortMode:    "path",
	}
	m := NewModel(cfg, "")

	wt1Path := filepath.Join(cfg.WorktreeDir, testWt1)
	wt2Path := filepath.Join(cfg.WorktreeDir, testWt2)
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: wt1Path, Branch: "feat-one"},
		{Path: wt2Path, Branch: "feat-two"},
	}
	m.state.services.filter.FilterQuery = ""
	m.state.ui.filterInput.SetValue("")
	m.updateTable()
	m.state.view.ShowingFilter = true
	m.state.ui.filterInput.Focus()
	m.state.ui.worktreeTable.SetCursor(1)
	m.state.data.selectedIndex = 1

	updated, _ := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyEnter})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	if m.state.view.ShowingFilter {
		t.Fatal("expected filter to be closed")
	}
	if m.state.data.selectedIndex != 1 {
		t.Fatalf("expected selectedIndex to remain 1, got %d", m.state.data.selectedIndex)
	}
}

func TestFilterCtrlCExitsFilter(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		SortMode:    "path",
	}
	m := NewModel(cfg, "")

	wt1Path := filepath.Join(cfg.WorktreeDir, testWt1)
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: wt1Path, Branch: "feat-one"},
	}
	m.state.services.filter.FilterQuery = "something"
	m.state.ui.filterInput.SetValue("something")
	m.updateTable()
	m.state.view.ShowingFilter = true
	m.state.ui.filterInput.Focus()

	updated, _ := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyCtrlC})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	if m.state.view.ShowingFilter {
		t.Fatal("expected filter to be closed after Ctrl+C")
	}
	if m.state.ui.filterInput.Focused() {
		t.Fatal("expected filter input to be blurred")
	}
}

func TestSearchWorktreeSelectsMatch(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		SortMode:    "path",
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 0

	wt1Path := filepath.Join(cfg.WorktreeDir, "alpha")
	wt2Path := filepath.Join(cfg.WorktreeDir, "beta")
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: wt1Path, Branch: "feat-one"},
		{Path: wt2Path, Branch: "feat-two"},
	}
	m.updateTable()
	m.state.ui.worktreeTable.SetCursor(0)
	m.state.data.selectedIndex = 0

	updated, _ := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	_, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})

	if m.state.ui.worktreeTable.Cursor() != 1 {
		t.Fatalf("expected cursor to move to match, got %d", m.state.ui.worktreeTable.Cursor())
	}
}

func TestFilterStatusNarrowsList(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1
	m.state.ui.statusViewport = viewport.New(40, 10)
	m.setStatusFiles([]StatusFile{
		{Filename: "app.go", Status: ".M"},
		{Filename: "README.md", Status: ".M"},
	})

	updated, _ := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	_, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	_, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	_, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	_, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})

	if len(m.state.data.statusFiles) != 1 {
		t.Fatalf("expected 1 filtered status file, got %d", len(m.state.data.statusFiles))
	}
	if m.state.data.statusFiles[0].Filename != testReadme {
		t.Fatalf("expected %s, got %q", testReadme, m.state.data.statusFiles[0].Filename)
	}
}

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

func TestFilterEnterClosesWithoutSelectingItem(t *testing.T) {
	// Set default provider for testing
	SetIconProvider(&NerdFontV3Provider{})
	cfg := &config.AppConfig{
		WorktreeDir:      t.TempDir(),
		SortMode:         "path",
		SearchAutoSelect: false,
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 0

	wt1Path := filepath.Join(cfg.WorktreeDir, "srv-api")
	wt2Path := filepath.Join(cfg.WorktreeDir, "srv-auth")
	wt3Path := filepath.Join(cfg.WorktreeDir, "srv-worker")
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: wt1Path, Branch: "feature/srv-api"},
		{Path: wt2Path, Branch: "feature/srv-auth"},
		{Path: wt3Path, Branch: "feature/srv-worker"},
	}

	// Apply filter for "srv"
	m.state.services.filter.FilterQuery = "srv"
	m.state.ui.filterInput.SetValue("srv")
	m.updateTable()
	m.state.view.ShowingFilter = true
	m.state.ui.filterInput.Focus()

	// Navigate to the second item (srv-auth)
	m.state.ui.worktreeTable.SetCursor(1)
	m.state.data.selectedIndex = 1

	// Press Enter - should exit filter without selecting
	updated, cmd := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyEnter})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	if cmd != nil {
		t.Fatal("expected no command to be returned")
	}
	if m.state.view.ShowingFilter {
		t.Fatal("expected filter to be closed")
	}
	if m.selectedPath != "" {
		t.Fatalf("expected selected path to remain empty, got %q", m.selectedPath)
	}
}

func TestFilterNavigationThroughMultipleFilteredItems(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		SortMode:    "path",
	}
	m := NewModel(cfg, "")

	// Create 5 worktrees, 3 of which match "srv" filter
	wt1Path := filepath.Join(cfg.WorktreeDir, "main")
	wt2Path := filepath.Join(cfg.WorktreeDir, "srv-api")
	wt3Path := filepath.Join(cfg.WorktreeDir, "frontend")
	wt4Path := filepath.Join(cfg.WorktreeDir, "srv-auth")
	wt5Path := filepath.Join(cfg.WorktreeDir, "srv-worker")

	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: wt1Path, Branch: "main", IsMain: true},
		{Path: wt2Path, Branch: "feature/srv-api"},
		{Path: wt3Path, Branch: "feature/frontend"},
		{Path: wt4Path, Branch: "feature/srv-auth"},
		{Path: wt5Path, Branch: "feature/srv-worker"},
	}

	// Apply filter for "srv"
	m.state.services.filter.FilterQuery = "srv"
	m.state.ui.filterInput.SetValue("srv")
	m.updateTable()
	m.state.view.ShowingFilter = true
	m.state.ui.filterInput.Focus()
	m.state.ui.worktreeTable.SetCursor(0)
	m.state.data.selectedIndex = 0

	// Verify we have exactly 3 filtered items
	if len(m.state.data.filteredWts) != 3 {
		t.Fatalf("expected 3 filtered items, got %d", len(m.state.data.filteredWts))
	}

	// Navigate down through all filtered items
	for i := 0; i < 2; i++ {
		updated, _ := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyDown})
		updatedModel, ok := updated.(*Model)
		if !ok {
			t.Fatalf("expected updated model, got %T", updated)
		}
		m = updatedModel
	}

	// Should be at the last filtered item (index 2)
	cursor := m.state.ui.worktreeTable.Cursor()
	if cursor != 2 {
		t.Fatalf("expected cursor at index 2, got %d", cursor)
	}

	// Try to navigate down again - should stay at last item
	updated, _ := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyDown})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	cursor = m.state.ui.worktreeTable.Cursor()
	if cursor != 2 {
		t.Fatalf("expected cursor to stay at index 2, got %d", cursor)
	}

	// Navigate back up
	for i := 0; i < 2; i++ {
		updated, _ := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyUp})
		updatedModel, ok := updated.(*Model)
		if !ok {
			t.Fatalf("expected updated model, got %T", updated)
		}
		m = updatedModel
	}

	// Should be at the first filtered item (index 0)
	cursor = m.state.ui.worktreeTable.Cursor()
	if cursor != 0 {
		t.Fatalf("expected cursor at index 0, got %d", cursor)
	}

	// Try to navigate up again - should stay at first item
	updated, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyUp})
	updatedModel, ok = updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	cursor = m.state.ui.worktreeTable.Cursor()
	if cursor != 0 {
		t.Fatalf("expected cursor to stay at index 0, got %d", cursor)
	}
}

// TestStatusFileNavigation tests j/k navigation through status tree items in pane 1.
func TestStatusFileNavigation(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1
	m.state.ui.statusViewport = viewport.New(40, 10)

	// Set up status files using setStatusFiles to build tree
	m.setStatusFiles([]StatusFile{
		{Filename: "file1.go", Status: ".M", IsUntracked: false},
		{Filename: "file2.go", Status: "M.", IsUntracked: false},
		{Filename: "file3.go", Status: " ?", IsUntracked: true},
	})
	m.state.services.statusTree.Index = 0

	// Test navigation down with j
	_, _ = m.handleNavigationDown(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.state.services.statusTree.Index != 1 {
		t.Fatalf("expected statusTreeIndex 1 after j, got %d", m.state.services.statusTree.Index)
	}

	// Test navigation down again
	_, _ = m.handleNavigationDown(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.state.services.statusTree.Index != 2 {
		t.Fatalf("expected statusTreeIndex 2 after second j, got %d", m.state.services.statusTree.Index)
	}

	// Test boundary - should not go past last item
	_, _ = m.handleNavigationDown(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.state.services.statusTree.Index != 2 {
		t.Fatalf("expected statusTreeIndex to stay at 2, got %d", m.state.services.statusTree.Index)
	}

	// Test navigation up with k
	_, _ = m.handleNavigationUp(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.state.services.statusTree.Index != 1 {
		t.Fatalf("expected statusTreeIndex 1 after k, got %d", m.state.services.statusTree.Index)
	}

	// Navigate to first item
	_, _ = m.handleNavigationUp(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.state.services.statusTree.Index != 0 {
		t.Fatalf("expected statusTreeIndex 0 after second k, got %d", m.state.services.statusTree.Index)
	}

	// Test boundary - should not go below 0
	_, _ = m.handleNavigationUp(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.state.services.statusTree.Index != 0 {
		t.Fatalf("expected statusTreeIndex to stay at 0, got %d", m.state.services.statusTree.Index)
	}
}

func TestLogPaneCtrlJMovesNextCommit(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 2
	m.state.ui.logTable.Focus()
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: t.TempDir(), Branch: testFeat},
	}
	m.state.data.selectedIndex = 0
	m.state.data.logEntries = []commitLogEntry{
		{sha: "abc123", authorInitials: "ab", message: "first"},
		{sha: "def456", authorInitials: "de", message: "second"},
	}
	m.state.ui.logTable.SetRows([]table.Row{
		{"abc123", "ab", "first"},
		{"def456", "de", "second"},
	})
	m.state.ui.logTable.SetCursor(0)

	updated, cmd := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyCtrlJ})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	if m.state.ui.logTable.Cursor() != 1 {
		t.Fatalf("expected log cursor at 1, got %d", m.state.ui.logTable.Cursor())
	}
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}
	// The command now returns a commitFilesLoadedMsg instead of calling execProcess
	// since openCommitView shows the files screen first
	msg := cmd()
	if _, ok := msg.(commitFilesLoadedMsg); !ok {
		t.Fatalf("expected commitFilesLoadedMsg, got %T", msg)
	}
}

func TestSearchLogSelectsNextMatch(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 2
	m.state.data.logEntries = []commitLogEntry{
		{sha: "abc123", authorInitials: "ab", message: "Fix bug in parser"},
		{sha: "def456", authorInitials: "de", message: "Add new feature"},
		{sha: "ghi789", authorInitials: "gh", message: "Fix tests"},
	}
	m.state.ui.logTable.SetRows([]table.Row{
		{"abc123", "ab", formatCommitMessage("Fix bug in parser")},
		{"def456", "de", formatCommitMessage("Add new feature")},
		{"ghi789", "gh", formatCommitMessage("Fix tests")},
	})

	updated, _ := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	_, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	_, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	_, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	if m.state.ui.logTable.Cursor() != 0 {
		t.Fatalf("expected first match at cursor 0, got %d", m.state.ui.logTable.Cursor())
	}

	// Confirm search with Enter, then use n to advance to next match
	updated, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyEnter})
	updatedModel, ok = updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	_, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if m.state.ui.logTable.Cursor() != 2 {
		t.Fatalf("expected next match at cursor 2, got %d", m.state.ui.logTable.Cursor())
	}
}

func TestFilterLogNarrowsList(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 2
	m.setLogEntries([]commitLogEntry{
		{sha: "abc123", authorInitials: "ab", message: "Fix bug in parser"},
		{sha: "def456", authorInitials: "de", message: "Add new feature"},
	}, false)

	updated, _ := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	_, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	_, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	_, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	if len(m.state.data.logEntries) != 1 {
		t.Fatalf("expected 1 filtered commit, got %d", len(m.state.data.logEntries))
	}
	if m.state.data.logEntries[0].sha != "abc123" {
		t.Fatalf("expected commit abc123, got %q", m.state.data.logEntries[0].sha)
	}
}

// TestStatusFileNavigationEmptyList tests navigation with no status files.
func TestStatusFileNavigationEmptyList(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1
	m.state.ui.statusViewport = viewport.New(40, 10)
	m.setStatusFiles(nil)
	m.state.services.statusTree.Index = 0

	// Should not panic with empty list
	_, _ = m.handleNavigationDown(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.state.services.statusTree.Index != 0 {
		t.Fatalf("expected statusTreeIndex to stay at 0, got %d", m.state.services.statusTree.Index)
	}

	_, _ = m.handleNavigationUp(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.state.services.statusTree.Index != 0 {
		t.Fatalf("expected statusTreeIndex to stay at 0, got %d", m.state.services.statusTree.Index)
	}
}

// TestStatusFileEnterShowsDiff tests that Enter on pane 1 triggers showFileDiff.
func TestStatusFileEnterShowsDiff(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1
	m.state.ui.statusViewport = viewport.New(40, 10)

	// Set up worktree and status files
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: filepath.Join(cfg.WorktreeDir, "wt1"), Branch: "feature"},
	}
	m.state.data.selectedIndex = 0
	m.setStatusFiles([]StatusFile{
		{Filename: "file1.go", Status: ".M", IsUntracked: false},
		{Filename: "file2.go", Status: "M.", IsUntracked: false},
	})
	m.state.services.statusTree.Index = 1

	// Mock execProcess to capture the command
	var capturedCmd bool
	m.execProcess = func(_ *exec.Cmd, cb tea.ExecCallback) tea.Cmd {
		capturedCmd = true
		return func() tea.Msg { return cb(nil) }
	}

	_, cmd := m.handleEnterKey()
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}

	// Execute the command
	_ = cmd()

	if !capturedCmd {
		t.Fatal("expected execProcess to be called")
	}
}

func TestLogPaneDiffCommandModeUsesCommitRange(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir:         t.TempDir(),
		GitPager:            "lumen",
		GitPagerCommandMode: true,
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 2
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: testWorktreePath, Branch: testFeat},
	}
	m.state.data.selectedIndex = 0
	m.setLogEntries([]commitLogEntry{
		{sha: "abc123", authorInitials: "ab", message: "Fix parser"},
	}, true)

	capture := &commandCapture{}
	m.commandRunner = capture.runner
	m.execProcess = capture.exec

	_, cmd := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}

	if capture.name != testBashCmd {
		t.Fatalf("expected bash command, got %q", capture.name)
	}
	if len(capture.args) != 2 || capture.args[0] != "-c" {
		t.Fatalf("expected bash -c args, got %v", capture.args)
	}
	if !strings.Contains(capture.args[1], "lumen diff abc123^..abc123") {
		t.Fatalf("expected commit range command, got %q", capture.args[1])
	}
}

func TestStatusFileEditOpensEditor(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		Editor:      "nvim",
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1
	m.state.ui.statusViewport = viewport.New(40, 10)

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	if err := os.MkdirAll(wtPath, 0o700); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}
	filename := "file1.go"
	if err := os.WriteFile(filepath.Join(wtPath, filename), []byte("package main\n"), 0o600); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: wtPath, Branch: "feature"},
	}
	m.state.data.selectedIndex = 0
	m.setStatusFiles([]StatusFile{
		{Filename: filename, Status: ".M", IsUntracked: false},
	})
	m.state.services.statusTree.Index = 0

	var gotCmd *exec.Cmd
	m.execProcess = func(cmd *exec.Cmd, cb tea.ExecCallback) tea.Cmd {
		gotCmd = cmd
		return func() tea.Msg { return cb(nil) }
	}

	_, cmd := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}
	_ = cmd()

	if gotCmd == nil {
		t.Fatal("expected execProcess to be called")
	}
	if gotCmd.Dir != wtPath {
		t.Fatalf("expected worktree dir %q, got %q", wtPath, gotCmd.Dir)
	}
	if len(gotCmd.Args) < 3 || gotCmd.Args[0] != testBashCmd || gotCmd.Args[1] != "-c" {
		t.Fatalf("expected bash -c command, got %v", gotCmd.Args)
	}
	if !strings.Contains(gotCmd.Args[2], "nvim") || !strings.Contains(gotCmd.Args[2], filename) {
		t.Fatalf("expected editor command to include nvim and file, got %q", gotCmd.Args[2])
	}
}

func TestCommitAllChangesFromStatusPane(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1
	m.state.ui.statusViewport = viewport.New(40, 10)

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	if err := os.MkdirAll(wtPath, 0o700); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: wtPath, Branch: "feature"},
	}
	m.state.data.selectedIndex = 0

	var gotCmd *exec.Cmd
	m.execProcess = func(cmd *exec.Cmd, cb tea.ExecCallback) tea.Cmd {
		gotCmd = cmd
		return func() tea.Msg { return cb(nil) }
	}

	_, cmd := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}
	_ = cmd()

	if gotCmd == nil {
		t.Fatal("expected execProcess to be called")
	}
	if gotCmd.Dir != wtPath {
		t.Fatalf("expected worktree dir %q, got %q", wtPath, gotCmd.Dir)
	}
	if len(gotCmd.Args) < 3 || gotCmd.Args[0] != testBashCmd || gotCmd.Args[1] != "-c" {
		t.Fatalf("expected bash -c command, got %v", gotCmd.Args)
	}
	if !strings.Contains(gotCmd.Args[2], "git add -A") || !strings.Contains(gotCmd.Args[2], "git commit") {
		t.Fatalf("expected git add -A && git commit command, got %q", gotCmd.Args[2])
	}
}

func TestCommitAllChangesNotInStatusPane(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 0 // Not status pane

	_, cmd := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	if cmd != nil {
		t.Fatal("expected no command when not in status pane")
	}
}

func TestCommitStagedChangesFromStatusPane(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1
	m.state.ui.statusViewport = viewport.New(40, 10)

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	if err := os.MkdirAll(wtPath, 0o700); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: wtPath, Branch: "feature"},
	}
	m.state.data.selectedIndex = 0

	// Set up staged changes
	m.setStatusFiles([]StatusFile{
		{Filename: "file1.go", Status: "M ", IsUntracked: false}, // Staged modification
	})

	var gotCmd *exec.Cmd
	m.execProcess = func(cmd *exec.Cmd, cb tea.ExecCallback) tea.Cmd {
		gotCmd = cmd
		return func() tea.Msg { return cb(nil) }
	}

	_, cmd := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}
	_ = cmd()

	if gotCmd == nil {
		t.Fatal("expected execProcess to be called")
	}
	if gotCmd.Dir != wtPath {
		t.Fatalf("expected worktree dir %q, got %q", wtPath, gotCmd.Dir)
	}
	if len(gotCmd.Args) < 3 || gotCmd.Args[0] != testBashCmd || gotCmd.Args[1] != "-c" {
		t.Fatalf("expected bash -c command, got %v", gotCmd.Args)
	}
	// Should only run git commit (not git add -A)
	if !strings.Contains(gotCmd.Args[2], "git commit") {
		t.Fatalf("expected git commit command, got %q", gotCmd.Args[2])
	}
	if strings.Contains(gotCmd.Args[2], "git add -A") {
		t.Fatalf("expected no git add -A in command, got %q", gotCmd.Args[2])
	}
}

func TestCommitStagedChangesNoStagedFiles(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1
	m.state.ui.statusViewport = viewport.New(40, 10)

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	if err := os.MkdirAll(wtPath, 0o700); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: wtPath, Branch: "feature"},
	}
	m.state.data.selectedIndex = 0

	// Set up only unstaged changes (no staged)
	m.setStatusFiles([]StatusFile{
		{Filename: "file1.go", Status: " M", IsUntracked: false}, // Unstaged modification
	})

	_, cmd := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if cmd != nil {
		t.Fatal("expected no command when no staged changes")
	}

	// Should show info screen with message
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Fatalf("expected info screen, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}
}

func TestCommitStagedChangesNotInStatusPane(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 0 // Not status pane

	_, cmd := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	// When not in status pane, 'c' should trigger create worktree which returns a command
	if cmd == nil {
		t.Fatal("expected command for create worktree when not in status pane")
	}
}

// Note: The smart PR sync tests that check isBehindBase() require a real git repository
// with actual commits and branches. Those are tested via integration tests.
// Here we test the simpler cases that don't require mocking git internals.

func TestStageUnstagedFile(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1
	m.state.ui.statusViewport = viewport.New(40, 10)

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	if err := os.MkdirAll(wtPath, 0o700); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: wtPath, Branch: "feature"},
	}
	m.state.data.selectedIndex = 0
	m.setStatusFiles([]StatusFile{
		{Filename: "file1.go", Status: " M", IsUntracked: false}, // Unstaged modification
	})
	m.state.services.statusTree.Index = 0

	var gotCmd *exec.Cmd
	m.commandRunner = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		gotCmd = exec.CommandContext(ctx, name, args...) //#nosec G204,G702 -- test mock with controlled args
		return gotCmd
	}

	_, cmd := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}

	if gotCmd == nil {
		t.Fatal("expected commandRunner to be called")
	}
	if gotCmd.Dir != wtPath {
		t.Fatalf("expected worktree dir %q, got %q", wtPath, gotCmd.Dir)
	}
	if len(gotCmd.Args) < 3 || gotCmd.Args[0] != testBashCmd || gotCmd.Args[1] != "-c" {
		t.Fatalf("expected bash -c command, got %v", gotCmd.Args)
	}
	if !strings.Contains(gotCmd.Args[2], "git add") {
		t.Fatalf("expected git add command, got %q", gotCmd.Args[2])
	}
}

func TestUnstageStagedFile(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1
	m.state.ui.statusViewport = viewport.New(40, 10)

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	if err := os.MkdirAll(wtPath, 0o700); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: wtPath, Branch: "feature"},
	}
	m.state.data.selectedIndex = 0
	m.setStatusFiles([]StatusFile{
		{Filename: "file1.go", Status: "M ", IsUntracked: false}, // Staged modification
	})
	m.state.services.statusTree.Index = 0

	var gotCmd *exec.Cmd
	m.commandRunner = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		gotCmd = exec.CommandContext(ctx, name, args...) //#nosec G204,G702 -- test mock with controlled args
		return gotCmd
	}

	_, cmd := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}

	if gotCmd == nil {
		t.Fatal("expected commandRunner to be called")
	}
	if !strings.Contains(gotCmd.Args[2], "git restore --staged") {
		t.Fatalf("expected git restore --staged command, got %q", gotCmd.Args[2])
	}
}

func TestStageMixedStatusFile(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1
	m.state.ui.statusViewport = viewport.New(40, 10)

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	if err := os.MkdirAll(wtPath, 0o700); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: wtPath, Branch: "feature"},
	}
	m.state.data.selectedIndex = 0
	m.setStatusFiles([]StatusFile{
		{Filename: "file1.go", Status: "MM", IsUntracked: false}, // Both staged and unstaged
	})
	m.state.services.statusTree.Index = 0

	var gotCmd *exec.Cmd
	m.commandRunner = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		gotCmd = exec.CommandContext(ctx, name, args...) //#nosec G204,G702 -- test mock with controlled args
		return gotCmd
	}

	_, cmd := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}

	if gotCmd == nil {
		t.Fatal("expected commandRunner to be called")
	}
	if !strings.Contains(gotCmd.Args[2], "git add") {
		t.Fatalf("expected git add command for mixed status, got %q", gotCmd.Args[2])
	}
}

func TestStageFileNotInStatusPane(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 0 // Not status pane
	m.setStatusFiles([]StatusFile{
		{Filename: "file1.go", Status: " M", IsUntracked: false},
	})
	m.state.services.statusTree.Index = 0

	_, cmd := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd != nil {
		t.Fatal("expected no command when not in status pane")
	}
}

func TestStageDirectoryAllUnstaged(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1
	m.state.ui.statusViewport = viewport.New(40, 10)
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: cfg.WorktreeDir, Branch: "feature"},
	}
	m.state.data.selectedIndex = 0

	// Build a tree with a directory containing unstaged files
	m.setStatusFiles([]StatusFile{
		{Filename: "src/file1.go", Status: " M", IsUntracked: false},
		{Filename: "src/file2.go", Status: " M", IsUntracked: false},
	})
	m.state.services.statusTree.Index = 0 // Select the directory

	if len(m.state.services.statusTree.TreeFlat) < 2 || !m.state.services.statusTree.TreeFlat[0].IsDir() {
		t.Fatal("expected directory node at index 0")
	}

	var gotCmd *exec.Cmd
	m.commandRunner = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		gotCmd = exec.CommandContext(ctx, name, args...) //#nosec G204,G702 -- test mock with controlled args
		return gotCmd
	}

	_, cmd := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}

	if gotCmd == nil {
		t.Fatal("expected commandRunner to be called")
	}
	if !strings.Contains(gotCmd.Args[2], "git add") {
		t.Fatalf("expected git add command for unstaged directory, got %q", gotCmd.Args[2])
	}
	// Verify both files are included
	if !strings.Contains(gotCmd.Args[2], "file1.go") || !strings.Contains(gotCmd.Args[2], "file2.go") {
		t.Fatalf("expected both files in git add command, got %q", gotCmd.Args[2])
	}
}

func TestStageDirectoryAllStaged(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1
	m.state.ui.statusViewport = viewport.New(40, 10)
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: cfg.WorktreeDir, Branch: "feature"},
	}
	m.state.data.selectedIndex = 0

	// Build a tree with a directory containing fully staged files
	m.setStatusFiles([]StatusFile{
		{Filename: "src/file1.go", Status: "M ", IsUntracked: false},
		{Filename: "src/file2.go", Status: "A ", IsUntracked: false},
	})
	m.state.services.statusTree.Index = 0 // Select the directory

	if len(m.state.services.statusTree.TreeFlat) < 2 || !m.state.services.statusTree.TreeFlat[0].IsDir() {
		t.Fatal("expected directory node at index 0")
	}

	var gotCmd *exec.Cmd
	m.commandRunner = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		gotCmd = exec.CommandContext(ctx, name, args...) //#nosec G204,G702 -- test mock with controlled args
		return gotCmd
	}

	_, cmd := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}

	if gotCmd == nil {
		t.Fatal("expected commandRunner to be called")
	}
	if !strings.Contains(gotCmd.Args[2], "git restore --staged") {
		t.Fatalf("expected git restore --staged command for fully staged directory, got %q", gotCmd.Args[2])
	}
}

func TestStageDirectoryMixed(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1
	m.state.ui.statusViewport = viewport.New(40, 10)
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: cfg.WorktreeDir, Branch: "feature"},
	}
	m.state.data.selectedIndex = 0

	// Build a tree with a directory containing mixed status files
	m.setStatusFiles([]StatusFile{
		{Filename: "src/file1.go", Status: "M ", IsUntracked: false}, // Staged
		{Filename: "src/file2.go", Status: " M", IsUntracked: false}, // Unstaged
	})
	m.state.services.statusTree.Index = 0 // Select the directory

	if len(m.state.services.statusTree.TreeFlat) < 2 || !m.state.services.statusTree.TreeFlat[0].IsDir() {
		t.Fatal("expected directory node at index 0")
	}

	var gotCmd *exec.Cmd
	m.commandRunner = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		gotCmd = exec.CommandContext(ctx, name, args...) //#nosec G204,G702 -- test mock with controlled args
		return gotCmd
	}

	_, cmd := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}

	if gotCmd == nil {
		t.Fatal("expected commandRunner to be called")
	}
	// Mixed status should stage all files
	if !strings.Contains(gotCmd.Args[2], "git add") {
		t.Fatalf("expected git add command for mixed status directory, got %q", gotCmd.Args[2])
	}
}

func TestCollectFiles(t *testing.T) {
	// Test CollectFiles on a directory node
	dirNode := &StatusTreeNode{
		Path: "src",
		File: nil, // Directory
		Children: []*StatusTreeNode{
			{
				Path: "src/file1.go",
				File: &StatusFile{Filename: "src/file1.go", Status: " M"},
			},
			{
				Path: "src/sub",
				File: nil, // Subdirectory
				Children: []*StatusTreeNode{
					{
						Path: "src/sub/file2.go",
						File: &StatusFile{Filename: "src/sub/file2.go", Status: "M "},
					},
				},
			},
		},
	}

	files := dirNode.CollectFiles()
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}

	// Verify file names
	names := make(map[string]bool)
	for _, f := range files {
		names[f.Filename] = true
	}
	if !names["src/file1.go"] {
		t.Fatal("expected src/file1.go in collected files")
	}
	if !names["src/sub/file2.go"] {
		t.Fatal("expected src/sub/file2.go in collected files")
	}
}

func TestShowDeleteFileNoSelection(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.data.filteredWts = []*models.WorktreeInfo{}
	m.state.data.selectedIndex = -1

	if cmd := m.showDeleteFile(); cmd != nil {
		t.Fatal("expected nil command when no worktree selected")
	}
	if m.state.ui.screenManager.IsActive() && m.state.ui.screenManager.Type() == appscreen.TypeConfirm {
		t.Fatal("expected no confirm screen when no selection")
	}
}

func TestShowDeleteFileNoFiles(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: "/tmp/feat", Branch: featureBranch},
	}
	m.state.data.selectedIndex = 0
	m.state.services.statusTree.TreeFlat = []*StatusTreeNode{}
	m.state.services.statusTree.Index = 0

	if cmd := m.showDeleteFile(); cmd != nil {
		t.Fatal("expected nil command when no files in tree")
	}
	if m.state.ui.screenManager.IsActive() && m.state.ui.screenManager.Type() == appscreen.TypeConfirm {
		t.Fatal("expected no confirm screen when no files")
	}
}

func TestShowDeleteFileSingleFile(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: "/tmp/feat", Branch: featureBranch},
	}
	m.state.data.selectedIndex = 0
	m.state.services.statusTree.TreeFlat = []*StatusTreeNode{
		{
			Path: "file.go",
			File: &StatusFile{Filename: "file.go", Status: " M"},
		},
	}
	m.state.services.statusTree.Index = 0

	if cmd := m.showDeleteFile(); cmd != nil {
		t.Fatal("expected nil command for confirm screen setup")
	}
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeConfirm {
		t.Fatal("expected confirm screen to be set for file deletion")
	}
}

func TestShowDeleteFileDirectory(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: "/tmp/feat", Branch: featureBranch},
	}
	m.state.data.selectedIndex = 0
	m.state.services.statusTree.TreeFlat = []*StatusTreeNode{
		{
			Path: "src",
			File: nil, // Directory
			Children: []*StatusTreeNode{
				{
					Path: "src/file1.go",
					File: &StatusFile{Filename: "src/file1.go", Status: " M"},
				},
				{
					Path: "src/file2.go",
					File: &StatusFile{Filename: "src/file2.go", Status: "M "},
				},
			},
		},
	}
	m.state.services.statusTree.Index = 0

	if cmd := m.showDeleteFile(); cmd != nil {
		t.Fatal("expected nil command for confirm screen setup")
	}
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeConfirm {
		t.Fatal("expected confirm screen to be set for directory deletion")
	}
}

func TestShowDeleteFileEmptyDirectory(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: "/tmp/feat", Branch: featureBranch},
	}
	m.state.data.selectedIndex = 0
	m.state.services.statusTree.TreeFlat = []*StatusTreeNode{
		{
			Path:     "src",
			File:     nil, // Directory
			Children: []*StatusTreeNode{},
		},
	}
	m.state.services.statusTree.Index = 0

	if cmd := m.showDeleteFile(); cmd != nil {
		t.Fatal("expected nil command for empty directory")
	}
	if m.state.ui.screenManager.IsActive() && m.state.ui.screenManager.Type() == appscreen.TypeConfirm {
		t.Fatal("expected no confirm screen for empty directory")
	}
}

// TestStatusFileEnterNoFilesDoesNothing tests Enter with no status files.
func TestStatusFileEnterNoFilesDoesNothing(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1
	m.state.data.statusFiles = nil

	_, cmd := m.handleEnterKey()
	if cmd != nil {
		t.Fatal("expected no command when no status files")
	}
}

// TestBuildStatusContentParsesFiles tests that buildStatusContent parses git status correctly.
func TestBuildStatusContentParsesFiles(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1
	m.state.ui.statusViewport = viewport.New(40, 10)

	// Simulated git status --porcelain=v2 output
	statusRaw := `1 .M N... 100644 100644 100644 abc123 abc123 modified.go
1 M. N... 100644 100644 100644 def456 def456 staged.go
? untracked.txt
1 A. N... 100644 100644 100644 ghi789 ghi789 added.go
1 .D N... 100644 100644 100644 jkl012 jkl012 deleted.go`

	m.setStatusFiles(parseStatusFiles(statusRaw))
	m.rebuildStatusContentWithHighlight()

	if len(m.state.data.statusFiles) != 5 {
		t.Fatalf("expected 5 status files, got %d", len(m.state.data.statusFiles))
	}

	// Check first file (modified)
	if m.state.data.statusFiles[0].Filename != "modified.go" {
		t.Fatalf("expected filename 'modified.go', got %q", m.state.data.statusFiles[0].Filename)
	}
	if m.state.data.statusFiles[0].Status != ".M" {
		t.Fatalf("expected status '.M', got %q", m.state.data.statusFiles[0].Status)
	}
	if m.state.data.statusFiles[0].IsUntracked {
		t.Fatal("expected IsUntracked to be false for modified file")
	}

	// Check untracked file
	if m.state.data.statusFiles[2].Filename != "untracked.txt" {
		t.Fatalf("expected filename 'untracked.txt', got %q", m.state.data.statusFiles[2].Filename)
	}
	if !m.state.data.statusFiles[2].IsUntracked {
		t.Fatal("expected IsUntracked to be true for untracked file")
	}
}

// TestBuildStatusContentCleanTree tests that clean working tree is handled.
func TestBuildStatusContentCleanTree(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1
	m.state.ui.statusViewport = viewport.New(40, 10)
	m.state.data.statusFiles = []StatusFile{{Filename: "old.go", Status: ".M"}}
	m.state.data.statusFileIndex = 5

	m.setStatusFiles(parseStatusFiles(""))
	m.rebuildStatusContentWithHighlight()

	if len(m.state.data.statusFiles) != 0 {
		t.Fatalf("expected 0 status files for clean tree, got %d", len(m.state.data.statusFiles))
	}
	if m.state.data.statusFileIndex != 0 {
		t.Fatalf("expected statusFileIndex reset to 0, got %d", m.state.data.statusFileIndex)
	}
	if !strings.Contains(m.statusContent, "Clean working tree") {
		t.Fatalf("expected 'Clean working tree' in result, got %q", m.statusContent)
	}
}

func TestSearchStatusSelectsMatch(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1
	m.state.ui.statusViewport = viewport.New(40, 10)
	// Note: tree sorts alphabetically, so README.md (R) comes before app.go (a)
	m.setStatusFiles([]StatusFile{
		{Filename: "app.go", Status: ".M"},
		{Filename: "README.md", Status: ".M"},
	})
	m.rebuildStatusContentWithHighlight()

	updated, _ := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	// Search for "app" to find app.go which is at index 1 after sorting
	_, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	_, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	_, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})

	if m.state.services.statusTree.Index != 1 {
		t.Fatalf("expected statusTreeIndex 1, got %d", m.state.services.statusTree.Index)
	}
}

// TestRenderStatusFilesHighlighting tests that selected file is highlighted.
func TestRenderStatusFilesHighlighting(t *testing.T) {
	SetIconProvider(&NerdFontV3Provider{})
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = t.TempDir()
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1
	m.state.ui.statusViewport = viewport.New(40, 10)
	m.setStatusFiles([]StatusFile{
		{Filename: "file1.go", Status: ".M", IsUntracked: false},
		{Filename: "file2.go", Status: ".M", IsUntracked: false},
	})
	m.state.services.statusTree.Index = 1

	result := m.renderStatusFiles()

	// The result should contain both filenames
	if !strings.Contains(result, "file1.go") {
		t.Fatalf("expected result to contain 'file1.go', got %q", result)
	}
	if !strings.Contains(result, "file2.go") {
		t.Fatalf("expected result to contain 'file2.go', got %q", result)
	}
	icon := deviconForName("file1.go", false)
	if icon == "" {
		t.Fatalf("expected devicon for file1.go, got empty string")
	}
	if !strings.Contains(result, icon) {
		t.Fatalf("expected result to contain devicon %q, got %q", icon, result)
	}

	// Result should have multiple lines
	lines := strings.Split(result, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
}

func TestRenderStatusFilesIconsDisabled(t *testing.T) {
	SetIconProvider(&NerdFontV3Provider{})
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = t.TempDir()
	cfg.IconSet = "text"
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1
	m.state.ui.statusViewport = viewport.New(40, 10)
	m.setStatusFiles([]StatusFile{
		{Filename: "file1.go", Status: ".M", IsUntracked: false},
	})
	m.state.services.statusTree.Index = 0

	result := m.renderStatusFiles()
	icon := deviconForName("file1.go", false)
	if icon != "" && strings.Contains(result, icon) {
		t.Fatalf("expected icons disabled, got %q in %q", icon, result)
	}
}

// TestStatusTreeIndexClamping tests that statusTreeIndex is clamped to valid range.
func TestStatusTreeIndexClamping(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1
	m.state.ui.statusViewport = viewport.New(40, 10)

	// Set index out of range before parsing
	m.state.services.statusTree.Index = 100

	statusRaw := `1 .M N... 100644 100644 100644 abc123 abc123 file1.go
1 .M N... 100644 100644 100644 abc123 abc123 file2.go`

	m.setStatusFiles(parseStatusFiles(statusRaw))
	m.rebuildStatusContentWithHighlight()

	// Index should be clamped to last valid index
	if m.state.services.statusTree.Index != 1 {
		t.Fatalf("expected statusTreeIndex clamped to 1, got %d", m.state.services.statusTree.Index)
	}

	// Test negative index
	m.state.services.statusTree.Index = -5
	m.setStatusFiles(parseStatusFiles(statusRaw))
	m.rebuildStatusContentWithHighlight()

	if m.state.services.statusTree.Index != 0 {
		t.Fatalf("expected statusTreeIndex clamped to 0, got %d", m.state.services.statusTree.Index)
	}
}

// TestMouseScrollNavigatesFiles tests that mouse scroll navigates tree items in pane 1.
func TestMouseScrollNavigatesFiles(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1
	m.state.ui.statusViewport = viewport.New(40, 10)
	m.state.view.WindowWidth = 100
	m.state.view.WindowHeight = 30

	m.setStatusFiles([]StatusFile{
		{Filename: "file1.go", Status: ".M", IsUntracked: false},
		{Filename: "file2.go", Status: ".M", IsUntracked: false},
		{Filename: "file3.go", Status: ".M", IsUntracked: false},
	})
	m.state.services.statusTree.Index = 0

	// Scroll down should increment index
	msg := tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelDown,
		X:      60, // Right side of screen (pane 1)
		Y:      5,
	}

	_, _ = m.handleMouse(msg)
	if m.state.services.statusTree.Index != 1 {
		t.Fatalf("expected statusTreeIndex 1 after scroll down, got %d", m.state.services.statusTree.Index)
	}

	// Scroll up should decrement index
	msg.Button = tea.MouseButtonWheelUp
	_, _ = m.handleMouse(msg)
	if m.state.services.statusTree.Index != 0 {
		t.Fatalf("expected statusTreeIndex 0 after scroll up, got %d", m.state.services.statusTree.Index)
	}
}

func TestMouseClickSelectsWorktree(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)
	m.state.view.FocusedPane = 0
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: filepath.Join(cfg.WorktreeDir, testWt1), Branch: testFeat},
		{Path: filepath.Join(cfg.WorktreeDir, testWt2), Branch: testOtherBranch},
	}
	m.state.ui.worktreeTable.SetRows([]table.Row{
		{"wt1"},
		{"wt2"},
	})
	m.state.ui.worktreeTable.SetCursor(0)
	m.state.data.selectedIndex = 0

	msg := tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      2,
		Y:      6,
	}

	_, _ = m.handleMouse(msg)
	if m.state.ui.worktreeTable.Cursor() != 1 {
		t.Fatalf("expected cursor to move to 1, got %d", m.state.ui.worktreeTable.Cursor())
	}
	if m.state.data.selectedIndex != 1 {
		t.Fatalf("expected selectedIndex to be 1, got %d", m.state.data.selectedIndex)
	}
}

// TestBuildStatusTreeEmpty tests building tree from empty file list.
func TestBuildStatusTreeEmpty(t *testing.T) {
	tree := services.BuildStatusTree([]StatusFile{})
	if tree == nil {
		t.Fatal("expected non-nil tree root")
	} else if tree.Path != "" {
		t.Errorf("expected empty root path, got %q", tree.Path)
	}
	if len(tree.Children) != 0 {
		t.Errorf("expected no children for empty input, got %d", len(tree.Children))
	}
}

// TestBuildStatusTreeFlatFiles tests tree with files at root level.
func TestBuildStatusTreeFlatFiles(t *testing.T) {
	files := []StatusFile{
		{Filename: "README.md", Status: ".M"},
		{Filename: "main.go", Status: "M."},
	}
	tree := services.BuildStatusTree(files)

	if len(tree.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(tree.Children))
	}

	// Should be sorted alphabetically
	if tree.Children[0].Path != "README.md" {
		t.Errorf("expected first child README.md, got %q", tree.Children[0].Path)
	}
	if tree.Children[1].Path != "main.go" {
		t.Errorf("expected second child main.go, got %q", tree.Children[1].Path)
	}

	// Both should be files, not directories
	for _, child := range tree.Children {
		if child.IsDir() {
			t.Errorf("expected %q to be a file, not directory", child.Path)
		}
		if child.File == nil {
			t.Errorf("expected %q to have File pointer", child.Path)
		}
	}
}

// TestBuildStatusTreeNestedDirs tests tree with nested directory structure.
func TestBuildStatusTreeNestedDirs(t *testing.T) {
	files := []StatusFile{
		{Filename: "internal/app/app.go", Status: ".M"},
		{Filename: "internal/app/handlers.go", Status: ".M"},
		{Filename: "internal/git/git.go", Status: "M."},
		{Filename: "README.md", Status: ".M"},
	}
	tree := services.BuildStatusTree(files)

	// Root should have 2 children: internal (dir) and README.md (file)
	// After compression, internal/app and internal/git are separate
	if len(tree.Children) != 2 {
		t.Fatalf("expected 2 root children, got %d", len(tree.Children))
	}

	// Directories should come before files
	if tree.Children[0].Path != "internal" && !strings.HasPrefix(tree.Children[0].Path, "internal") {
		t.Errorf("expected first child to be internal dir, got %q", tree.Children[0].Path)
	}
	if tree.Children[1].Path != "README.md" {
		t.Errorf("expected second child to be README.md, got %q", tree.Children[1].Path)
	}
}

// TestBuildStatusTreeDirsSortedBeforeFiles tests that directories appear before files.
func TestBuildStatusTreeDirsSortedBeforeFiles(t *testing.T) {
	files := []StatusFile{
		{Filename: "zebra.txt", Status: ".M"},
		{Filename: "aaa/file.go", Status: ".M"},
		{Filename: "alpha.txt", Status: ".M"},
	}
	tree := services.BuildStatusTree(files)

	if len(tree.Children) != 3 {
		t.Fatalf("expected 3 children, got %d", len(tree.Children))
	}

	// First should be the directory (aaa), then files alphabetically
	if !tree.Children[0].IsDir() {
		t.Error("expected first child to be a directory")
	}
	if tree.Children[0].Path != "aaa" {
		t.Errorf("expected first child aaa, got %q", tree.Children[0].Path)
	}
	if tree.Children[1].IsDir() {
		t.Error("expected second child to be a file")
	}
	if tree.Children[2].IsDir() {
		t.Error("expected third child to be a file")
	}
}

// TestCompressStatusTreeSingleChild tests compression of single-child directory chains.
func TestCompressStatusTreeSingleChild(t *testing.T) {
	files := []StatusFile{
		{Filename: "a/b/c/file.go", Status: ".M"},
	}
	tree := services.BuildStatusTree(files)

	// After compression, a/b/c should be one node, not three nested nodes
	flat := services.FlattenStatusTree(tree, map[string]bool{}, 0)

	// Should have: a/b/c (dir) + file.go (file) = 2 nodes
	if len(flat) != 2 {
		t.Fatalf("expected 2 flattened nodes after compression, got %d", len(flat))
	}

	if flat[0].Path != "a/b/c" {
		t.Errorf("expected compressed path a/b/c, got %q", flat[0].Path)
	}
	if !flat[0].IsDir() {
		t.Error("expected first node to be a directory")
	}
	if flat[1].Path != "a/b/c/file.go" {
		t.Errorf("expected file path a/b/c/file.go, got %q", flat[1].Path)
	}
}

// TestFlattenStatusTreeCollapsed tests that collapsed directories hide children.
func TestFlattenStatusTreeCollapsed(t *testing.T) {
	files := []StatusFile{
		{Filename: "dir/file1.go", Status: ".M"},
		{Filename: "dir/file2.go", Status: ".M"},
		{Filename: "root.go", Status: ".M"},
	}
	tree := services.BuildStatusTree(files)

	// Without collapse: should see dir + 2 files + root.go = 4 nodes
	flatOpen := services.FlattenStatusTree(tree, map[string]bool{}, 0)
	if len(flatOpen) != 4 {
		t.Fatalf("expected 4 nodes when expanded, got %d", len(flatOpen))
	}

	// With dir collapsed: should see dir + root.go = 2 nodes
	collapsed := map[string]bool{"dir": true}
	flatClosed := services.FlattenStatusTree(tree, collapsed, 0)
	if len(flatClosed) != 2 {
		t.Fatalf("expected 2 nodes when collapsed, got %d", len(flatClosed))
	}

	if flatClosed[0].Path != "dir" {
		t.Errorf("expected first node to be dir, got %q", flatClosed[0].Path)
	}
	if flatClosed[1].Path != "root.go" {
		t.Errorf("expected second node to be root.go, got %q", flatClosed[1].Path)
	}
}

// TestStatusTreeNodeHelpers tests IsDir and Name helper methods.
func TestStatusTreeNodeHelpers(t *testing.T) {
	fileNode := &StatusTreeNode{
		Path: "internal/app/app.go",
		File: &StatusFile{Filename: "internal/app/app.go", Status: ".M"},
	}
	dirNode := &StatusTreeNode{
		Path:     "internal/app",
		Children: []*StatusTreeNode{},
	}

	if fileNode.IsDir() {
		t.Error("file node should not be a directory")
	}
	if !dirNode.IsDir() {
		t.Error("dir node should be a directory")
	}

	if fileNode.Name() != "app.go" {
		t.Errorf("expected file name app.go, got %q", fileNode.Name())
	}
	if dirNode.Name() != "app" {
		t.Errorf("expected dir name app, got %q", dirNode.Name())
	}
}

// TestFlattenStatusTreeDepth tests that depth is correctly calculated.
func TestFlattenStatusTreeDepth(t *testing.T) {
	files := []StatusFile{
		{Filename: "dir/subdir/file.go", Status: ".M"},
		{Filename: "root.go", Status: ".M"},
	}
	tree := services.BuildStatusTree(files)
	flat := services.FlattenStatusTree(tree, map[string]bool{}, 0)

	// After compression: dir/subdir (depth 0), file.go (depth 1), root.go (depth 0)
	if len(flat) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(flat))
	}

	// Root level nodes should have depth 0
	if flat[0].Depth != 0 {
		t.Errorf("expected dir/subdir depth 0, got %d", flat[0].Depth)
	}
	// File inside dir should have depth 1
	if flat[1].Depth != 1 {
		t.Errorf("expected file.go depth 1, got %d", flat[1].Depth)
	}
	// Root file should have depth 0
	if flat[2].Depth != 0 {
		t.Errorf("expected root.go depth 0, got %d", flat[2].Depth)
	}
}

// TestDirectoryToggleUpdatesFlat tests that toggling directory collapse updates flattened list.
func TestDirectoryToggleUpdatesFlat(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1
	m.state.ui.statusViewport = viewport.New(40, 10)
	m.state.view.WindowWidth = 100
	m.state.view.WindowHeight = 30

	m.setStatusFiles([]StatusFile{
		{Filename: "dir/file1.go", Status: ".M"},
		{Filename: "dir/file2.go", Status: ".M"},
	})

	initialCount := len(m.state.services.statusTree.TreeFlat)
	if initialCount != 3 { // dir + 2 files
		t.Fatalf("expected 3 nodes initially, got %d", initialCount)
	}

	// Collapse the directory
	m.state.services.statusTree.CollapsedDirs["dir"] = true
	m.rebuildStatusTreeFlat()

	if len(m.state.services.statusTree.TreeFlat) != 1 { // just the dir
		t.Fatalf("expected 1 node after collapse, got %d", len(m.state.services.statusTree.TreeFlat))
	}

	// Expand again
	m.state.services.statusTree.CollapsedDirs["dir"] = false
	m.rebuildStatusTreeFlat()

	if len(m.state.services.statusTree.TreeFlat) != 3 {
		t.Fatalf("expected 3 nodes after expand, got %d", len(m.state.services.statusTree.TreeFlat))
	}
}

func TestEscClearsWorktreeFilter(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 0
	m.state.services.filter.FilterQuery = testFilterQuery
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: filepath.Join(cfg.WorktreeDir, "test-wt"), Branch: testFeat},
	}
	m.updateTable()

	updated, _ := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyEsc})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}

	if updatedModel.state.services.filter.FilterQuery != "" {
		t.Fatalf("expected filter to be cleared, got %q", updatedModel.state.services.filter.FilterQuery)
	}
}

func TestEscClearsStatusFilter(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1
	m.state.services.filter.StatusFilterQuery = testFilterQuery

	updated, _ := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyEsc})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}

	if updatedModel.state.services.filter.StatusFilterQuery != "" {
		t.Fatalf("expected status filter to be cleared, got %q", updatedModel.state.services.filter.StatusFilterQuery)
	}
}

func TestEscClearsLogFilter(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 2
	m.state.services.filter.LogFilterQuery = testFilterQuery

	updated, _ := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyEsc})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}

	if updatedModel.state.services.filter.LogFilterQuery != "" {
		t.Fatalf("expected log filter to be cleared, got %q", updatedModel.state.services.filter.LogFilterQuery)
	}
}

func TestEscDoesNothingWhenNoFilter(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 0
	m.state.services.filter.FilterQuery = ""

	updated, _ := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyEsc})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}

	if updatedModel.state.services.filter.FilterQuery != "" {
		t.Fatalf("expected filter to remain empty, got %q", updatedModel.state.services.filter.FilterQuery)
	}
}

func TestHasActiveFilterForPane(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	// No filters active
	if m.hasActiveFilterForPane(0) {
		t.Fatal("expected no active filter for pane 0")
	}
	if m.hasActiveFilterForPane(1) {
		t.Fatal("expected no active filter for pane 1")
	}
	if m.hasActiveFilterForPane(2) {
		t.Fatal("expected no active filter for pane 2")
	}

	// Set worktree filter
	m.state.services.filter.FilterQuery = testFilterQuery
	if !m.hasActiveFilterForPane(0) {
		t.Fatal("expected active filter for pane 0")
	}

	// Set status filter
	m.state.services.filter.StatusFilterQuery = testFilterQuery
	if !m.hasActiveFilterForPane(1) {
		t.Fatal("expected active filter for pane 1")
	}

	// Set log filter
	m.state.services.filter.LogFilterQuery = testFilterQuery
	if !m.hasActiveFilterForPane(2) {
		t.Fatal("expected active filter for pane 2")
	}

	// Whitespace-only should not count as active
	m.state.services.filter.FilterQuery = "   "
	if m.hasActiveFilterForPane(0) {
		t.Fatal("expected whitespace-only filter to not be active")
	}
}

func TestZoomPaneToggle(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 0

	if m.state.view.ZoomedPane != -1 {
		t.Fatalf("expected zoomedPane to start at -1, got %d", m.state.view.ZoomedPane)
	}

	// Press = to zoom pane 0
	updated, _ := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'='}})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	if m.state.view.ZoomedPane != 0 {
		t.Fatalf("expected zoomedPane to be 0 after zoom, got %d", m.state.view.ZoomedPane)
	}

	// Press = again to unzoom
	updated, _ = m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'='}})
	updatedModel, ok = updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	if m.state.view.ZoomedPane != -1 {
		t.Fatalf("expected zoomedPane to be -1 after unzoom, got %d", m.state.view.ZoomedPane)
	}
}

func TestZoomPaneExitsOnPaneKeys(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 0
	m.state.view.ZoomedPane = 0

	// Press 2 to switch to pane 2 and exit zoom
	updated, _ := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	if m.state.view.ZoomedPane != -1 {
		t.Fatalf("expected zoomedPane to be -1 after pressing 2, got %d", m.state.view.ZoomedPane)
	}
	if m.state.view.FocusedPane != 1 {
		t.Fatalf("expected focusedPane to be 1, got %d", m.state.view.FocusedPane)
	}
}

func TestZoomPaneExitsOnTabKey(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 0
	m.state.view.ZoomedPane = 0

	// Press tab to cycle panes and exit zoom
	updated, _ := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyTab})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	if m.state.view.ZoomedPane != -1 {
		t.Fatalf("expected zoomedPane to be -1 after pressing tab, got %d", m.state.view.ZoomedPane)
	}
	if m.state.view.FocusedPane != 1 {
		t.Fatalf("expected focusedPane to be 1 after tab, got %d", m.state.view.FocusedPane)
	}
}

func TestZoomPaneExitsOnBracketKey(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1
	m.state.view.ZoomedPane = 1

	// Press [ to cycle back and exit zoom
	updated, _ := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	if m.state.view.ZoomedPane != -1 {
		t.Fatalf("expected zoomedPane to be -1 after pressing [, got %d", m.state.view.ZoomedPane)
	}
	if m.state.view.FocusedPane != 0 {
		t.Fatalf("expected focusedPane to be 0 after [, got %d", m.state.view.FocusedPane)
	}
}

func TestPaneKey1ToggleZoom(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 0
	m.state.view.ZoomedPane = -1

	// Press 1 while on pane 0, not zoomed - should zoom
	updated, _ := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	if m.state.view.FocusedPane != 0 {
		t.Fatalf("expected focusedPane to remain 0, got %d", m.state.view.FocusedPane)
	}
	if m.state.view.ZoomedPane != 0 {
		t.Fatalf("expected zoomedPane to be 0 after toggle, got %d", m.state.view.ZoomedPane)
	}

	// Press 1 again while on pane 0, already zoomed - should unzoom
	updated, _ = m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	updatedModel, ok = updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	if m.state.view.FocusedPane != 0 {
		t.Fatalf("expected focusedPane to remain 0, got %d", m.state.view.FocusedPane)
	}
	if m.state.view.ZoomedPane != -1 {
		t.Fatalf("expected zoomedPane to be -1 after unzoom, got %d", m.state.view.ZoomedPane)
	}
}

func TestPaneKey2ToggleZoom(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1
	m.state.view.ZoomedPane = -1

	// Press 2 while on pane 1, not zoomed - should zoom
	updated, _ := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	if m.state.view.FocusedPane != 1 {
		t.Fatalf("expected focusedPane to remain 1, got %d", m.state.view.FocusedPane)
	}
	if m.state.view.ZoomedPane != 1 {
		t.Fatalf("expected zoomedPane to be 1 after toggle, got %d", m.state.view.ZoomedPane)
	}

	// Press 2 again while on pane 1, already zoomed - should unzoom
	updated, _ = m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	updatedModel, ok = updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	if m.state.view.FocusedPane != 1 {
		t.Fatalf("expected focusedPane to remain 1, got %d", m.state.view.FocusedPane)
	}
	if m.state.view.ZoomedPane != -1 {
		t.Fatalf("expected zoomedPane to be -1 after unzoom, got %d", m.state.view.ZoomedPane)
	}
}

func TestPaneKey3ToggleZoom(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 2
	m.state.view.ZoomedPane = -1

	// Press 3 while on pane 2, not zoomed - should zoom
	updated, _ := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	if m.state.view.FocusedPane != 2 {
		t.Fatalf("expected focusedPane to remain 2, got %d", m.state.view.FocusedPane)
	}
	if m.state.view.ZoomedPane != 2 {
		t.Fatalf("expected zoomedPane to be 2 after toggle, got %d", m.state.view.ZoomedPane)
	}

	// Press 3 again while on pane 2, already zoomed - should unzoom
	updated, _ = m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	updatedModel, ok = updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	if m.state.view.FocusedPane != 2 {
		t.Fatalf("expected focusedPane to remain 2, got %d", m.state.view.FocusedPane)
	}
	if m.state.view.ZoomedPane != -1 {
		t.Fatalf("expected zoomedPane to be -1 after unzoom, got %d", m.state.view.ZoomedPane)
	}
}

func TestPaneKeyCrossPaneSwitching(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 0
	m.state.view.ZoomedPane = 0

	// Press 2 while on pane 0 (zoomed) - should switch to pane 1 and exit zoom
	updated, _ := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	if m.state.view.FocusedPane != 1 {
		t.Fatalf("expected focusedPane to be 1, got %d", m.state.view.FocusedPane)
	}
	if m.state.view.ZoomedPane != -1 {
		t.Fatalf("expected zoomedPane to be -1 after switching panes, got %d", m.state.view.ZoomedPane)
	}

	// Now press 3 while on pane 1 (not zoomed) - should switch to pane 2 and remain unzoomed
	updated, _ = m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	updatedModel, ok = updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	if m.state.view.FocusedPane != 2 {
		t.Fatalf("expected focusedPane to be 2, got %d", m.state.view.FocusedPane)
	}
	if m.state.view.ZoomedPane != -1 {
		t.Fatalf("expected zoomedPane to remain -1, got %d", m.state.view.ZoomedPane)
	}

	// Press 1 while on pane 2 (not zoomed) - should switch to pane 0
	updated, _ = m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	updatedModel, ok = updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	if m.state.view.FocusedPane != 0 {
		t.Fatalf("expected focusedPane to be 0, got %d", m.state.view.FocusedPane)
	}
	if m.state.view.ZoomedPane != -1 {
		t.Fatalf("expected zoomedPane to remain -1, got %d", m.state.view.ZoomedPane)
	}
}

// Message handler tests

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

func TestClearSearchQuery(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.view.SearchTarget = searchTargetWorktrees
	m.setSearchQuery(searchTargetWorktrees, "test query")
	m.state.ui.filterInput.SetValue("test")

	m.clearSearchQuery()

	if m.state.services.filter.WorktreeSearchQuery != "" {
		t.Errorf("expected worktreeSearchQuery to be empty, got %q", m.state.services.filter.WorktreeSearchQuery)
	}
	if m.state.ui.filterInput.Value() != "" {
		t.Errorf("expected filterInput to be empty, got %q", m.state.ui.filterInput.Value())
	}
}

func TestRestoreFocusAfterSearch(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	t.Run("restore focus to worktrees", func(t *testing.T) {
		m.state.view.SearchTarget = searchTargetWorktrees
		m.restoreFocusAfterSearch()
		if !m.state.ui.worktreeTable.Focused() {
			t.Error("expected worktreeTable to be focused")
		}
	})

	t.Run("restore focus to log", func(t *testing.T) {
		m.state.view.SearchTarget = searchTargetLog
		m.restoreFocusAfterSearch()
		if !m.state.ui.logTable.Focused() {
			t.Error("expected logTable to be focused")
		}
	})
}

func TestRestoreFocusAfterFilter(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	t.Run("restore focus to worktrees", func(t *testing.T) {
		m.state.view.FilterTarget = filterTargetWorktrees
		m.restoreFocusAfterFilter()
		if !m.state.ui.worktreeTable.Focused() {
			t.Error("expected worktreeTable to be focused")
		}
	})

	t.Run("restore focus to log", func(t *testing.T) {
		m.state.view.FilterTarget = filterTargetLog
		m.restoreFocusAfterFilter()
		if !m.state.ui.logTable.Focused() {
			t.Error("expected logTable to be focused")
		}
	})
}

func TestHandleGotoTop(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	t.Run("goto top on worktree pane", func(t *testing.T) {
		m.state.view.FocusedPane = 0
		m.state.data.worktrees = []*models.WorktreeInfo{
			{Path: filepath.Join(cfg.WorktreeDir, "wt1"), Branch: "branch1"},
			{Path: filepath.Join(cfg.WorktreeDir, "wt2"), Branch: "branch2"},
			{Path: filepath.Join(cfg.WorktreeDir, "wt3"), Branch: "branch3"},
		}
		m.state.ui.worktreeTable.SetWidth(100)
		m.updateTable()
		m.updateTableColumns(m.state.ui.worktreeTable.Width())
		if len(m.state.ui.worktreeTable.Rows()) == 0 {
			t.Fatal("table has no rows after updateTable")
		}
		m.state.ui.worktreeTable.SetCursor(2)
		_, cmd := m.handleGotoTop()
		if m.state.ui.worktreeTable.Cursor() != 0 {
			t.Errorf("expected cursor at top (0), got %d", m.state.ui.worktreeTable.Cursor())
		}
		if cmd == nil {
			t.Error("expected command to be returned")
		}
	})

	t.Run("goto top on status pane", func(t *testing.T) {
		m.state.view.FocusedPane = 1
		m.state.services.statusTree.TreeFlat = []*StatusTreeNode{
			{Path: "file1.txt", File: &StatusFile{Filename: "file1.txt"}},
			{Path: "dir1", Children: []*StatusTreeNode{}},
			{Path: "file2.txt", File: &StatusFile{Filename: "file2.txt"}},
		}
		m.state.services.statusTree.Index = 2
		_, cmd := m.handleGotoTop()
		if m.state.services.statusTree.Index != 0 {
			t.Errorf("expected statusTreeIndex to be 0, got %d", m.state.services.statusTree.Index)
		}
		if cmd != nil {
			t.Error("expected nil command for status pane")
		}
	})

	t.Run("goto top on log pane", func(t *testing.T) {
		m.state.view.FocusedPane = 2
		m.state.data.logEntries = []commitLogEntry{
			{sha: "abc123", message: "commit 1"},
			{sha: "def456", message: "commit 2"},
			{sha: "ghi789", message: "commit 3"},
		}
		m.setLogEntries(m.state.data.logEntries, false)
		m.updateLogColumns(m.state.ui.logTable.Width())
		m.state.ui.logTable.SetCursor(2)
		_, cmd := m.handleGotoTop()
		if m.state.ui.logTable.Cursor() != 0 {
			t.Errorf("expected cursor at top, got %d", m.state.ui.logTable.Cursor())
		}
		// Command may or may not be nil - just verify the cursor moved
		_ = cmd
	})
}

func TestHandleGotoBottom(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	t.Run("goto bottom on worktree pane", func(t *testing.T) {
		m.state.view.FocusedPane = 0
		m.state.ui.worktreeTable.SetCursor(0)
		m.state.data.filteredWts = []*models.WorktreeInfo{
			{Path: "wt1", Branch: "branch1"},
			{Path: "wt2", Branch: "branch2"},
			{Path: "wt3", Branch: "branch3"},
		}
		m.updateTable()
		_, cmd := m.handleGotoBottom()
		expectedBottom := len(m.state.data.filteredWts) - 1
		if m.state.ui.worktreeTable.Cursor() != expectedBottom {
			t.Errorf("expected cursor at bottom (%d), got %d", expectedBottom, m.state.ui.worktreeTable.Cursor())
		}
		if cmd == nil {
			t.Error("expected command to be returned")
		}
	})

	t.Run("goto bottom on status pane", func(t *testing.T) {
		m.state.view.FocusedPane = 1
		m.state.services.statusTree.TreeFlat = []*StatusTreeNode{
			{Path: "file1.txt", File: &StatusFile{Filename: "file1.txt"}},
			{Path: "dir1", Children: []*StatusTreeNode{}},
			{Path: "file2.txt", File: &StatusFile{Filename: "file2.txt"}},
		}
		m.state.services.statusTree.Index = 0
		_, cmd := m.handleGotoBottom()
		expectedBottom := len(m.state.services.statusTree.TreeFlat) - 1
		if m.state.services.statusTree.Index != expectedBottom {
			t.Errorf("expected statusTreeIndex to be %d, got %d", expectedBottom, m.state.services.statusTree.Index)
		}
		// Command may or may not be nil - just verify the index changed
		_ = cmd
	})

	t.Run("goto bottom on empty status pane", func(t *testing.T) {
		m.state.view.FocusedPane = 1
		m.state.services.statusTree.TreeFlat = []*StatusTreeNode{}
		m.state.services.statusTree.Index = 0
		_, cmd := m.handleGotoBottom()
		if m.state.services.statusTree.Index != 0 {
			t.Errorf("expected statusTreeIndex to remain 0, got %d", m.state.services.statusTree.Index)
		}
		_ = cmd
	})

	t.Run("goto bottom on log pane", func(t *testing.T) {
		m.state.view.FocusedPane = 2
		m.state.ui.logTable.SetCursor(0)
		m.state.data.logEntries = []commitLogEntry{
			{sha: "abc123", message: "commit 1"},
			{sha: "def456", message: "commit 2"},
			{sha: "ghi789", message: "commit 3"},
		}
		m.setLogEntries(m.state.data.logEntries, false)
		m.updateLogColumns(m.state.ui.logTable.Width())
		_, cmd := m.handleGotoBottom()
		expectedBottom := len(m.state.data.logEntries) - 1
		if m.state.ui.logTable.Cursor() != expectedBottom {
			t.Errorf("expected cursor at bottom (%d), got %d", expectedBottom, m.state.ui.logTable.Cursor())
		}
		// Command may or may not be nil - just verify the cursor moved
		_ = cmd
	})
}

func TestHandleNextFolder(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1

	t.Run("empty status tree", func(t *testing.T) {
		m.state.services.statusTree.TreeFlat = []*StatusTreeNode{}
		m.state.services.statusTree.Index = 0
		_, cmd := m.handleNextFolder()
		if cmd != nil {
			t.Error("expected nil command for empty tree")
		}
	})

	t.Run("find next folder", func(t *testing.T) {
		m.state.services.statusTree.TreeFlat = []*StatusTreeNode{
			{Path: "file1.txt", File: &StatusFile{Filename: "file1.txt"}},
			{Path: "dir1", Children: []*StatusTreeNode{}},
			{Path: "file2.txt", File: &StatusFile{Filename: "file2.txt"}},
			{Path: "dir2", Children: []*StatusTreeNode{}},
		}
		m.state.services.statusTree.Index = 0
		_, cmd := m.handleNextFolder()
		if m.state.services.statusTree.Index != 1 {
			t.Errorf("expected statusTreeIndex to be 1 (dir1), got %d", m.state.services.statusTree.Index)
		}
		if cmd != nil {
			t.Error("expected nil command")
		}
	})

	t.Run("no next folder found", func(t *testing.T) {
		m.state.services.statusTree.TreeFlat = []*StatusTreeNode{
			{Path: "file1.txt", File: &StatusFile{Filename: "file1.txt"}},
			{Path: "dir1", Children: []*StatusTreeNode{}},
		}
		m.state.services.statusTree.Index = 1 // Already at last folder
		_, cmd := m.handleNextFolder()
		if m.state.services.statusTree.Index != 1 {
			t.Errorf("expected statusTreeIndex to remain 1, got %d", m.state.services.statusTree.Index)
		}
		if cmd != nil {
			t.Error("expected nil command")
		}
	})
}

func TestHandlePrevFolder(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1

	t.Run("empty status tree", func(t *testing.T) {
		m.state.services.statusTree.TreeFlat = []*StatusTreeNode{}
		m.state.services.statusTree.Index = 0
		_, cmd := m.handlePrevFolder()
		if cmd != nil {
			t.Error("expected nil command for empty tree")
		}
	})

	t.Run("find previous folder", func(t *testing.T) {
		m.state.services.statusTree.TreeFlat = []*StatusTreeNode{
			{Path: "dir1", Children: []*StatusTreeNode{}},
			{Path: "file1.txt", File: &StatusFile{Filename: "file1.txt"}},
			{Path: "dir2", Children: []*StatusTreeNode{}},
			{Path: "file2.txt", File: &StatusFile{Filename: "file2.txt"}},
		}
		m.state.services.statusTree.Index = 3
		_, cmd := m.handlePrevFolder()
		if m.state.services.statusTree.Index != 2 {
			t.Errorf("expected statusTreeIndex to be 2 (dir2), got %d", m.state.services.statusTree.Index)
		}
		if cmd != nil {
			t.Error("expected nil command")
		}
	})

	t.Run("no previous folder found", func(t *testing.T) {
		m.state.services.statusTree.TreeFlat = []*StatusTreeNode{
			{Path: "dir1", Children: []*StatusTreeNode{}},
			{Path: "file1.txt", File: &StatusFile{Filename: "file1.txt"}},
		}
		m.state.services.statusTree.Index = 0 // Already at first folder
		_, cmd := m.handlePrevFolder()
		if m.state.services.statusTree.Index != 0 {
			t.Errorf("expected statusTreeIndex to remain 0, got %d", m.state.services.statusTree.Index)
		}
		if cmd != nil {
			t.Error("expected nil command")
		}
	})
}

func TestCICheckNavigationDown(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1
	m.state.data.selectedIndex = 0
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: testWorktreePath, Branch: "feat"},
	}

	// Add CI checks to cache
	checks := []*models.CICheck{
		{Name: "build", Conclusion: "success", Link: "https://github.com/owner/repo/actions/runs/123"},
		{Name: "test", Conclusion: "failure", Link: "https://github.com/owner/repo/actions/runs/456"},
		{Name: "lint", Conclusion: "success", Link: "https://github.com/owner/repo/actions/runs/789"},
	}
	m.cache.ciCache.Set("feat", checks)

	// Start navigating CI checks
	m.ciCheckIndex = 0

	// Navigate down
	_, _ = m.handleNavigationDown(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.ciCheckIndex != 1 {
		t.Fatalf("expected ciCheckIndex 1 after j, got %d", m.ciCheckIndex)
	}

	// Navigate down again
	_, _ = m.handleNavigationDown(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.ciCheckIndex != 2 {
		t.Fatalf("expected ciCheckIndex 2 after second j, got %d", m.ciCheckIndex)
	}

	// At last CI check, should wrap to file tree
	m.setStatusFiles([]StatusFile{
		{Filename: "file1.go", Status: ".M", IsUntracked: false},
	})
	_, _ = m.handleNavigationDown(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.ciCheckIndex != -1 {
		t.Fatalf("expected ciCheckIndex -1 after wrapping, got %d", m.ciCheckIndex)
	}
	if m.state.services.statusTree.Index != 0 {
		t.Fatalf("expected statusTreeIndex 0 after wrapping, got %d", m.state.services.statusTree.Index)
	}
}

func TestCICheckNavigationUp(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1
	m.state.data.selectedIndex = 0
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: testWorktreePath, Branch: "feat"},
	}

	// Add CI checks to cache
	checks := []*models.CICheck{
		{Name: "build", Conclusion: "success", Link: "https://github.com/owner/repo/actions/runs/123"},
		{Name: "test", Conclusion: "failure", Link: "https://github.com/owner/repo/actions/runs/456"},
		{Name: "lint", Conclusion: "success", Link: "https://github.com/owner/repo/actions/runs/789"},
	}
	m.cache.ciCache.Set("feat", checks)

	// Set up file tree
	m.setStatusFiles([]StatusFile{
		{Filename: "file1.go", Status: ".M", IsUntracked: false},
	})
	m.state.services.statusTree.Index = 0

	// Start at first CI check
	m.ciCheckIndex = 1

	// Navigate up
	_, _ = m.handleNavigationUp(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.ciCheckIndex != 0 {
		t.Fatalf("expected ciCheckIndex 0 after k, got %d", m.ciCheckIndex)
	}

	// At first CI check, should stay at 0
	_, _ = m.handleNavigationUp(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.ciCheckIndex != 0 {
		t.Fatalf("expected ciCheckIndex to stay at 0, got %d", m.ciCheckIndex)
	}

	// Test wrapping from file tree to CI checks
	m.ciCheckIndex = -1
	m.state.services.statusTree.Index = 0
	_, _ = m.handleNavigationUp(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.ciCheckIndex != 2 {
		t.Fatalf("expected ciCheckIndex 2 after wrapping from file tree, got %d", m.ciCheckIndex)
	}
}

func TestCICheckEnterOpensURL(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1
	m.state.data.selectedIndex = 0
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: testWorktreePath, Branch: "feat"},
	}

	// Add CI checks to cache
	checkURL := "https://github.com/owner/repo/actions/runs/123"
	checks := []*models.CICheck{
		{Name: "build", Conclusion: "success", Link: checkURL},
	}
	m.cache.ciCache.Set("feat", checks)
	m.ciCheckIndex = 0

	// Capture command
	var capturedCmd *exec.Cmd
	m.commandRunner = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		capturedCmd = exec.CommandContext(ctx, name, args...) //#nosec G204,G702 -- test mock with controlled args
		return capturedCmd
	}
	m.startCommand = func(cmd *exec.Cmd) error {
		return nil
	}

	// Press Enter
	_, cmd := m.handleEnterKey()
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}

	// Execute command to capture URL
	_ = cmd()

	// Verify the command was to open URL (platform-specific)
	if capturedCmd == nil || len(capturedCmd.Args) == 0 {
		t.Fatal("expected command to be executed")
	}
	// Check for common browser commands
	validCommands := []string{"open", "xdg-open", "rundll32"}
	isValid := false
	for _, validCmd := range validCommands {
		if capturedCmd.Args[0] == validCmd {
			isValid = true
			break
		}
	}
	if !isValid {
		t.Fatalf("expected browser command, got %v", capturedCmd.Args)
	}
}

func TestCICheckCtrlVShowsLogs(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1
	m.state.data.selectedIndex = 0
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: testWorktreePath, Branch: "feat"},
	}

	// Add CI checks to cache
	checkURL := "https://github.com/owner/repo/actions/runs/123"
	checks := []*models.CICheck{
		{Name: "build", Conclusion: "success", Link: checkURL},
	}
	m.cache.ciCache.Set("feat", checks)
	m.ciCheckIndex = 0

	// Capture command
	var capturedCmd *exec.Cmd
	m.commandRunner = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		capturedCmd = exec.CommandContext(ctx, name, args...) //#nosec G204,G702 -- test mock with controlled args
		return capturedCmd
	}
	m.startCommand = func(cmd *exec.Cmd) error {
		return nil
	}

	// Press Ctrl+v
	_, cmd := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyCtrlV})
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}

	// Execute command
	_ = cmd()

	// Verify the command contains gh run view
	if capturedCmd == nil || len(capturedCmd.Args) < 2 {
		t.Fatal("expected command with args")
	}
	if capturedCmd.Args[0] != "bash" {
		t.Fatalf("expected bash command, got %s", capturedCmd.Args[0])
	}
	if !strings.Contains(capturedCmd.Args[2], "gh run view") {
		t.Fatalf("expected gh run view in command, got %s", capturedCmd.Args[2])
	}
}

func TestCICheckSelectionResetOnWorktreeChange(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.data.selectedIndex = 0
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: testWorktreePath, Branch: "feat"},
		{Path: "/other/path", Branch: "other"},
	}

	// Add CI checks to cache
	checks := []*models.CICheck{
		{Name: "build", Conclusion: "success", Link: "https://github.com/owner/repo/actions/runs/123"},
	}
	m.cache.ciCache.Set("feat", checks)
	m.ciCheckIndex = 0

	// Change worktree selection
	m.state.data.selectedIndex = 1
	m.updateDetailsView()

	// CI check selection should be reset
	if m.ciCheckIndex != -1 {
		t.Fatalf("expected ciCheckIndex -1 after worktree change, got %d", m.ciCheckIndex)
	}
}

func TestCICheckSelectionResetOnPaneSwitch(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1
	m.state.data.selectedIndex = 0
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: testWorktreePath, Branch: "feat"},
	}

	// Add CI checks to cache
	checks := []*models.CICheck{
		{Name: "build", Conclusion: "success", Link: "https://github.com/owner/repo/actions/runs/123"},
	}
	m.cache.ciCache.Set("feat", checks)
	m.ciCheckIndex = 0

	// Switch to pane 0
	_, _ = m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})

	// CI check selection should be reset
	if m.ciCheckIndex != -1 {
		t.Fatalf("expected ciCheckIndex -1 after pane switch, got %d", m.ciCheckIndex)
	}
}

func TestCICheckSelectionResetWhenUnavailable(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1
	m.state.data.selectedIndex = 0
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: testWorktreePath, Branch: "feat"},
	}

	// Add CI checks to cache
	checks := []*models.CICheck{
		{Name: "build", Conclusion: "success", Link: "https://github.com/owner/repo/actions/runs/123"},
	}
	m.cache.ciCache.Set("feat", checks)
	m.ciCheckIndex = 5 // Out of bounds

	// Navigate - should reset invalid index
	_, _ = m.handleNavigationDown(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.ciCheckIndex != -1 {
		t.Fatalf("expected ciCheckIndex -1 after invalid index, got %d", m.ciCheckIndex)
	}

	// Clear CI cache
	m.cache.ciCache.Clear()
	m.ciCheckIndex = 0

	// Navigate - should reset when no CI checks available
	_, _ = m.handleNavigationDown(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.ciCheckIndex != -1 {
		t.Fatalf("expected ciCheckIndex -1 when no CI checks, got %d", m.ciCheckIndex)
	}
}

func TestCICheckNavigationWithNoChecks(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1
	m.state.ui.statusViewport = viewport.New(40, 10)
	m.state.data.selectedIndex = 0
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: testWorktreePath, Branch: "feat"},
	}

	// No CI checks in cache
	m.setStatusFiles([]StatusFile{
		{Filename: "file1.go", Status: ".M", IsUntracked: false},
		{Filename: "file2.go", Status: "M.", IsUntracked: false},
	})
	m.state.services.statusTree.Index = 0

	// Navigation should work on file tree
	_, _ = m.handleNavigationDown(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.state.services.statusTree.Index != 1 {
		t.Fatalf("expected statusTreeIndex 1, got %d", m.state.services.statusTree.Index)
	}
	if m.ciCheckIndex != -1 {
		t.Fatalf("expected ciCheckIndex -1, got %d", m.ciCheckIndex)
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
