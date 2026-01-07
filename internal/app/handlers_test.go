package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

const (
	testFeat        = "feat"
	testWt1         = "wt1"
	testWt2         = "wt2"
	testReadme      = "README.md"
	testFilterQuery = "test"
)

func TestHandlePageDownUpOnStatusPane(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.focusedPane = 1
	m.statusViewport = viewport.New(10, 2)
	m.statusViewport.SetContent(strings.Repeat("line\n", 10))

	start := m.statusViewport.YOffset
	_, _ = m.handlePageDown(tea.KeyMsg{Type: tea.KeyPgDown})
	if m.statusViewport.YOffset <= start {
		t.Fatalf("expected YOffset to increase, got %d", m.statusViewport.YOffset)
	}

	m.statusViewport.YOffset = 2
	_, _ = m.handlePageUp(tea.KeyMsg{Type: tea.KeyPgUp})
	if m.statusViewport.YOffset >= 2 {
		t.Fatalf("expected YOffset to decrease, got %d", m.statusViewport.YOffset)
	}
}

func TestHandleEnterKeySelectsWorktree(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.focusedPane = 0
	m.filteredWts = []*models.WorktreeInfo{
		{Path: filepath.Join(cfg.WorktreeDir, "wt"), Branch: testFeat},
	}
	m.selectedIndex = 0

	_, cmd := m.handleEnterKey()
	if m.selectedPath == "" {
		t.Fatal("expected selected path to be set")
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
	m.focusedPane = 0

	m.worktrees = []*models.WorktreeInfo{
		{Path: filepath.Join(cfg.WorktreeDir, "b-worktree"), Branch: testFeat},
		{Path: filepath.Join(cfg.WorktreeDir, "a-worktree"), Branch: testFeat},
	}
	m.filterQuery = testFeat
	m.filterInput.SetValue(testFeat)
	m.updateTable()
	m.showingFilter = true
	m.filterInput.Focus()
	m.worktreeTable.SetCursor(1)
	m.selectedIndex = 1

	updated, cmd := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyEnter})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	if cmd != nil {
		t.Fatal("expected no command to be returned")
	}
	if m.showingFilter {
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
	m.worktrees = []*models.WorktreeInfo{
		{Path: wt1Path, Branch: "feat-one"},
		{Path: wt2Path, Branch: "feat-two"},
	}
	m.filterQuery = testFeat
	m.filterInput.SetValue(testFeat)
	m.updateTable()
	m.showingFilter = true
	m.filterInput.Focus()
	m.worktreeTable.SetCursor(0)
	m.selectedIndex = 0

	updated, _ := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}, Alt: true})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	if m.filterInput.Value() != testWt2 || m.filterQuery != testWt2 {
		t.Fatalf("expected filter query to match selected worktree, got %q", m.filterQuery)
	}
	if len(m.filteredWts) != 1 || m.filteredWts[0].Path != wt2Path {
		t.Fatalf("expected filtered worktree %q, got %v", wt2Path, m.filteredWts)
	}

	updated, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}, Alt: true})
	updatedModel, ok = updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	if m.filterInput.Value() != testWt1 || m.filterQuery != testWt1 {
		t.Fatalf("expected filter query to match selected worktree, got %q", m.filterQuery)
	}
	if len(m.filteredWts) != 1 || m.filteredWts[0].Path != wt1Path {
		t.Fatalf("expected filtered worktree %q, got %v", wt1Path, m.filteredWts)
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
	m.worktrees = []*models.WorktreeInfo{
		{Path: wt1Path, Branch: "feat-one"},
		{Path: wt2Path, Branch: "feat-two"},
	}
	m.filterQuery = testFeat
	m.filterInput.SetValue(testFeat)
	m.updateTable()
	m.showingFilter = true
	m.filterInput.Focus()
	m.worktreeTable.SetCursor(0)
	m.selectedIndex = 0

	updated, _ := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyDown})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	if m.filterInput.Value() != testFeat || m.filterQuery != testFeat {
		t.Fatalf("expected filter query unchanged, got %q", m.filterQuery)
	}

	updated, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyUp})
	updatedModel, ok = updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	if m.filterInput.Value() != testFeat || m.filterQuery != testFeat {
		t.Fatalf("expected filter query unchanged, got %q", m.filterQuery)
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
	m.worktrees = []*models.WorktreeInfo{
		{Path: wt1Path, Branch: "feat-one"},
		{Path: wt2Path, Branch: "feat-two"},
	}
	m.filterQuery = ""
	m.filterInput.SetValue("")
	m.updateTable()
	m.showingFilter = true
	m.filterInput.Focus()
	m.worktreeTable.SetCursor(1)
	m.selectedIndex = 1

	updated, _ := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyEnter})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	if m.showingFilter {
		t.Fatal("expected filter to be closed")
	}
	if m.selectedIndex != 1 {
		t.Fatalf("expected selectedIndex to remain 1, got %d", m.selectedIndex)
	}
}

func TestFilterCtrlCExitsFilter(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		SortMode:    "path",
	}
	m := NewModel(cfg, "")

	wt1Path := filepath.Join(cfg.WorktreeDir, testWt1)
	m.worktrees = []*models.WorktreeInfo{
		{Path: wt1Path, Branch: "feat-one"},
	}
	m.filterQuery = "something"
	m.filterInput.SetValue("something")
	m.updateTable()
	m.showingFilter = true
	m.filterInput.Focus()

	updated, _ := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyCtrlC})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	if m.showingFilter {
		t.Fatal("expected filter to be closed after Ctrl+C")
	}
	if m.filterInput.Focused() {
		t.Fatal("expected filter input to be blurred")
	}
}

func TestSearchWorktreeSelectsMatch(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		SortMode:    "path",
	}
	m := NewModel(cfg, "")
	m.focusedPane = 0

	wt1Path := filepath.Join(cfg.WorktreeDir, "alpha")
	wt2Path := filepath.Join(cfg.WorktreeDir, "beta")
	m.worktrees = []*models.WorktreeInfo{
		{Path: wt1Path, Branch: "feat-one"},
		{Path: wt2Path, Branch: "feat-two"},
	}
	m.updateTable()
	m.worktreeTable.SetCursor(0)
	m.selectedIndex = 0

	updated, _ := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	_, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})

	if m.worktreeTable.Cursor() != 1 {
		t.Fatalf("expected cursor to move to match, got %d", m.worktreeTable.Cursor())
	}
}

func TestFilterStatusNarrowsList(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.focusedPane = 1
	m.statusViewport = viewport.New(40, 10)
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

	if len(m.statusFiles) != 1 {
		t.Fatalf("expected 1 filtered status file, got %d", len(m.statusFiles))
	}
	if m.statusFiles[0].Filename != testReadme {
		t.Fatalf("expected %s, got %q", testReadme, m.statusFiles[0].Filename)
	}
}

func TestHandleCachedWorktreesUpdatesState(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.selectedIndex = 0
	m.worktreeTable.SetWidth(80)

	msg := cachedWorktreesMsg{
		worktrees: []*models.WorktreeInfo{
			{Path: filepath.Join(cfg.WorktreeDir, "wt1"), Branch: "main"},
		},
	}

	_, cmd := m.handleCachedWorktrees(msg)
	if cmd != nil {
		t.Fatal("expected no command")
	}
	if len(m.worktrees) != 1 {
		t.Fatalf("expected worktrees to be set, got %d", len(m.worktrees))
	}
	if m.statusContent != loadingRefreshWorktrees {
		t.Fatalf("unexpected status content: %q", m.statusContent)
	}
	if !strings.Contains(m.infoContent, "wt1") {
		t.Fatalf("expected info content to include worktree path, got %q", m.infoContent)
	}
}

func TestHandlePRDataLoadedUpdatesTable(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = t.TempDir()
	m := NewModel(cfg, "")
	m.worktreeTable.SetWidth(100)
	m.worktreesLoaded = true
	m.worktrees = []*models.WorktreeInfo{
		{Path: filepath.Join(cfg.WorktreeDir, "wt1"), Branch: "feature"},
	}
	m.filteredWts = m.worktrees
	m.worktreeTable.SetCursor(0)

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
	if m.worktrees[0].PR == nil {
		t.Fatal("expected PR info to be applied to worktree")
	}
	if len(m.worktreeTable.Columns()) != 5 {
		t.Fatalf("expected 5 columns after PR data, got %d", len(m.worktreeTable.Columns()))
	}
	rows := m.worktreeTable.Rows()
	if len(rows) != 1 {
		t.Fatalf("expected 1 row after PR data, got %d", len(rows))
	}
	if len(rows[0]) != 5 {
		t.Fatalf("expected 5 columns in row, got %d", len(rows[0]))
	}
	if !strings.Contains(rows[0][4], iconPR) {
		t.Fatalf("expected PR column to include icon %q, got %q", iconPR, rows[0][4])
	}
}

func TestHandlePRDataLoadedOmitsIconWhenDisabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = t.TempDir()
	cfg.ShowIcons = false
	m := NewModel(cfg, "")
	m.worktreeTable.SetWidth(100)
	m.worktreesLoaded = true
	m.worktrees = []*models.WorktreeInfo{
		{Path: filepath.Join(cfg.WorktreeDir, "wt1"), Branch: "feature"},
	}
	m.filteredWts = m.worktrees
	m.worktreeTable.SetCursor(0)

	msg := prDataLoadedMsg{
		prMap: map[string]*models.PRInfo{
			"feature": {Number: 12, State: "OPEN", Title: "Test PR", URL: "https://example.com"},
		},
	}

	_, cmd := m.handlePRDataLoaded(msg)
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}

	rows := m.worktreeTable.Rows()
	if len(rows) != 1 || len(rows[0]) != 5 {
		t.Fatalf("unexpected row shape: %+v", rows)
	}
	if strings.Contains(rows[0][4], iconPR) {
		t.Fatalf("expected PR icon to be omitted, got %q", rows[0][4])
	}
}

func TestHandlePRDataLoadedWithWorktreePRs(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.worktreeTable.SetWidth(100)
	m.worktreesLoaded = true
	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	m.worktrees = []*models.WorktreeInfo{
		{Path: wtPath, Branch: "local-branch-name"},
	}
	m.filteredWts = m.worktrees
	m.worktreeTable.SetCursor(0)

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
	if m.worktrees[0].PR == nil {
		t.Fatal("expected PR info to be applied to worktree via worktreePRs")
	}
	if m.worktrees[0].PR.Number != 99 {
		t.Fatalf("expected PR number 99, got %d", m.worktrees[0].PR.Number)
	}
}

func TestHandleCIStatusLoadedUpdatesCache(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = t.TempDir()
	m := NewModel(cfg, "")
	m.filteredWts = []*models.WorktreeInfo{
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
	m.selectedIndex = 0

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
	if entry, ok := m.ciCache["feature"]; !ok || len(entry.checks) != 1 {
		t.Fatalf("expected CI cache to be updated, got %v", entry)
	}
	if !strings.Contains(m.infoContent, "CI Checks:") {
		t.Fatalf("expected info content to include CI checks, got %q", m.infoContent)
	}
	if !strings.Contains(m.infoContent, iconCISuccess) {
		t.Fatalf("expected info content to include CI icon %q, got %q", iconCISuccess, m.infoContent)
	}
}

func TestHandleCIStatusLoadedOmitsIconWhenDisabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = t.TempDir()
	cfg.ShowIcons = false
	m := NewModel(cfg, "")
	m.filteredWts = []*models.WorktreeInfo{
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
	m.selectedIndex = 0

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
	if !strings.Contains(m.infoContent, "CI Checks:") {
		t.Fatalf("expected info content to include CI checks, got %q", m.infoContent)
	}
	if strings.Contains(m.infoContent, iconCISuccess) {
		t.Fatalf("expected CI icon to be omitted, got %q", m.infoContent)
	}
}

func TestFilterEnterClosesWithoutSelectingItem(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir:      t.TempDir(),
		SortMode:         "path",
		SearchAutoSelect: false,
	}
	m := NewModel(cfg, "")
	m.focusedPane = 0

	wt1Path := filepath.Join(cfg.WorktreeDir, "srv-api")
	wt2Path := filepath.Join(cfg.WorktreeDir, "srv-auth")
	wt3Path := filepath.Join(cfg.WorktreeDir, "srv-worker")
	m.worktrees = []*models.WorktreeInfo{
		{Path: wt1Path, Branch: "feature/srv-api"},
		{Path: wt2Path, Branch: "feature/srv-auth"},
		{Path: wt3Path, Branch: "feature/srv-worker"},
	}

	// Apply filter for "srv"
	m.filterQuery = "srv"
	m.filterInput.SetValue("srv")
	m.updateTable()
	m.showingFilter = true
	m.filterInput.Focus()

	// Navigate to the second item (srv-auth)
	m.worktreeTable.SetCursor(1)
	m.selectedIndex = 1

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
	if m.showingFilter {
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

	m.worktrees = []*models.WorktreeInfo{
		{Path: wt1Path, Branch: "main", IsMain: true},
		{Path: wt2Path, Branch: "feature/srv-api"},
		{Path: wt3Path, Branch: "feature/frontend"},
		{Path: wt4Path, Branch: "feature/srv-auth"},
		{Path: wt5Path, Branch: "feature/srv-worker"},
	}

	// Apply filter for "srv"
	m.filterQuery = "srv"
	m.filterInput.SetValue("srv")
	m.updateTable()
	m.showingFilter = true
	m.filterInput.Focus()
	m.worktreeTable.SetCursor(0)
	m.selectedIndex = 0

	// Verify we have exactly 3 filtered items
	if len(m.filteredWts) != 3 {
		t.Fatalf("expected 3 filtered items, got %d", len(m.filteredWts))
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
	cursor := m.worktreeTable.Cursor()
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

	cursor = m.worktreeTable.Cursor()
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
	cursor = m.worktreeTable.Cursor()
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

	cursor = m.worktreeTable.Cursor()
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
	m.focusedPane = 1
	m.statusViewport = viewport.New(40, 10)

	// Set up status files using setStatusFiles to build tree
	m.setStatusFiles([]StatusFile{
		{Filename: "file1.go", Status: ".M", IsUntracked: false},
		{Filename: "file2.go", Status: "M.", IsUntracked: false},
		{Filename: "file3.go", Status: " ?", IsUntracked: true},
	})
	m.statusTreeIndex = 0

	// Test navigation down with j
	_, _ = m.handleNavigationDown(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.statusTreeIndex != 1 {
		t.Fatalf("expected statusTreeIndex 1 after j, got %d", m.statusTreeIndex)
	}

	// Test navigation down again
	_, _ = m.handleNavigationDown(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.statusTreeIndex != 2 {
		t.Fatalf("expected statusTreeIndex 2 after second j, got %d", m.statusTreeIndex)
	}

	// Test boundary - should not go past last item
	_, _ = m.handleNavigationDown(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.statusTreeIndex != 2 {
		t.Fatalf("expected statusTreeIndex to stay at 2, got %d", m.statusTreeIndex)
	}

	// Test navigation up with k
	_, _ = m.handleNavigationUp(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.statusTreeIndex != 1 {
		t.Fatalf("expected statusTreeIndex 1 after k, got %d", m.statusTreeIndex)
	}

	// Navigate to first item
	_, _ = m.handleNavigationUp(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.statusTreeIndex != 0 {
		t.Fatalf("expected statusTreeIndex 0 after second k, got %d", m.statusTreeIndex)
	}

	// Test boundary - should not go below 0
	_, _ = m.handleNavigationUp(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.statusTreeIndex != 0 {
		t.Fatalf("expected statusTreeIndex to stay at 0, got %d", m.statusTreeIndex)
	}
}

func TestLogPaneCtrlJMovesNextCommit(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.focusedPane = 2
	m.logTable.Focus()
	m.filteredWts = []*models.WorktreeInfo{
		{Path: t.TempDir(), Branch: testFeat},
	}
	m.selectedIndex = 0
	m.logEntries = []commitLogEntry{
		{sha: "abc123", authorInitials: "ab", message: "first"},
		{sha: "def456", authorInitials: "de", message: "second"},
	}
	m.logTable.SetRows([]table.Row{
		{"abc123", "ab", "first"},
		{"def456", "de", "second"},
	})
	m.logTable.SetCursor(0)

	updated, cmd := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyCtrlJ})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	if m.logTable.Cursor() != 1 {
		t.Fatalf("expected log cursor at 1, got %d", m.logTable.Cursor())
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
	m.focusedPane = 2
	m.logEntries = []commitLogEntry{
		{sha: "abc123", authorInitials: "ab", message: "Fix bug in parser"},
		{sha: "def456", authorInitials: "de", message: "Add new feature"},
		{sha: "ghi789", authorInitials: "gh", message: "Fix tests"},
	}
	m.logTable.SetRows([]table.Row{
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

	if m.logTable.Cursor() != 0 {
		t.Fatalf("expected first match at cursor 0, got %d", m.logTable.Cursor())
	}

	_, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if m.logTable.Cursor() != 2 {
		t.Fatalf("expected next match at cursor 2, got %d", m.logTable.Cursor())
	}
}

func TestFilterLogNarrowsList(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.focusedPane = 2
	m.setLogEntries([]commitLogEntry{
		{sha: "abc123", authorInitials: "ab", message: "Fix bug in parser"},
		{sha: "def456", authorInitials: "de", message: "Add new feature"},
	})

	updated, _ := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	_, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	_, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	_, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	if len(m.logEntries) != 1 {
		t.Fatalf("expected 1 filtered commit, got %d", len(m.logEntries))
	}
	if m.logEntries[0].sha != "abc123" {
		t.Fatalf("expected commit abc123, got %q", m.logEntries[0].sha)
	}
}

// TestStatusFileNavigationEmptyList tests navigation with no status files.
func TestStatusFileNavigationEmptyList(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.focusedPane = 1
	m.statusViewport = viewport.New(40, 10)
	m.setStatusFiles(nil)
	m.statusTreeIndex = 0

	// Should not panic with empty list
	_, _ = m.handleNavigationDown(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.statusTreeIndex != 0 {
		t.Fatalf("expected statusTreeIndex to stay at 0, got %d", m.statusTreeIndex)
	}

	_, _ = m.handleNavigationUp(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.statusTreeIndex != 0 {
		t.Fatalf("expected statusTreeIndex to stay at 0, got %d", m.statusTreeIndex)
	}
}

// TestStatusFileEnterShowsDiff tests that Enter on pane 1 triggers showFileDiff.
func TestStatusFileEnterShowsDiff(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.focusedPane = 1
	m.statusViewport = viewport.New(40, 10)

	// Set up worktree and status files
	m.filteredWts = []*models.WorktreeInfo{
		{Path: filepath.Join(cfg.WorktreeDir, "wt1"), Branch: "feature"},
	}
	m.selectedIndex = 0
	m.setStatusFiles([]StatusFile{
		{Filename: "file1.go", Status: ".M", IsUntracked: false},
		{Filename: "file2.go", Status: "M.", IsUntracked: false},
	})
	m.statusTreeIndex = 1

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

func TestStatusFileEditOpensEditor(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		Editor:      "nvim",
	}
	m := NewModel(cfg, "")
	m.focusedPane = 1
	m.statusViewport = viewport.New(40, 10)

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	if err := os.MkdirAll(wtPath, 0o700); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}
	filename := "file1.go"
	if err := os.WriteFile(filepath.Join(wtPath, filename), []byte("package main\n"), 0o600); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	m.filteredWts = []*models.WorktreeInfo{
		{Path: wtPath, Branch: "feature"},
	}
	m.selectedIndex = 0
	m.setStatusFiles([]StatusFile{
		{Filename: filename, Status: ".M", IsUntracked: false},
	})
	m.statusTreeIndex = 0

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
	m.focusedPane = 1
	m.statusViewport = viewport.New(40, 10)

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	if err := os.MkdirAll(wtPath, 0o700); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	m.filteredWts = []*models.WorktreeInfo{
		{Path: wtPath, Branch: "feature"},
	}
	m.selectedIndex = 0

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
	m.focusedPane = 0 // Not status pane

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
	m.focusedPane = 1
	m.statusViewport = viewport.New(40, 10)

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	if err := os.MkdirAll(wtPath, 0o700); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	m.filteredWts = []*models.WorktreeInfo{
		{Path: wtPath, Branch: "feature"},
	}
	m.selectedIndex = 0

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
	m.focusedPane = 1
	m.statusViewport = viewport.New(40, 10)

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	if err := os.MkdirAll(wtPath, 0o700); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	m.filteredWts = []*models.WorktreeInfo{
		{Path: wtPath, Branch: "feature"},
	}
	m.selectedIndex = 0

	// Set up only unstaged changes (no staged)
	m.setStatusFiles([]StatusFile{
		{Filename: "file1.go", Status: " M", IsUntracked: false}, // Unstaged modification
	})

	_, cmd := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if cmd != nil {
		t.Fatal("expected no command when no staged changes")
	}

	// Should show info screen with message
	if m.currentScreen != screenInfo {
		t.Fatalf("expected screenInfo, got %v", m.currentScreen)
	}
	if m.infoScreen == nil {
		t.Fatal("expected infoScreen to be set")
	}
}

func TestCommitStagedChangesNotInStatusPane(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.focusedPane = 0 // Not status pane

	_, cmd := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	// When not in status pane, 'c' should trigger create worktree which returns a command
	if cmd == nil {
		t.Fatal("expected command for create worktree when not in status pane")
	}
}

func TestStageUnstagedFile(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.focusedPane = 1
	m.statusViewport = viewport.New(40, 10)

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	if err := os.MkdirAll(wtPath, 0o700); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	m.filteredWts = []*models.WorktreeInfo{
		{Path: wtPath, Branch: "feature"},
	}
	m.selectedIndex = 0
	m.setStatusFiles([]StatusFile{
		{Filename: "file1.go", Status: " M", IsUntracked: false}, // Unstaged modification
	})
	m.statusTreeIndex = 0

	var gotCmd *exec.Cmd
	m.commandRunner = func(name string, args ...string) *exec.Cmd {
		gotCmd = exec.Command(name, args...)
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
	m.focusedPane = 1
	m.statusViewport = viewport.New(40, 10)

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	if err := os.MkdirAll(wtPath, 0o700); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	m.filteredWts = []*models.WorktreeInfo{
		{Path: wtPath, Branch: "feature"},
	}
	m.selectedIndex = 0
	m.setStatusFiles([]StatusFile{
		{Filename: "file1.go", Status: "M ", IsUntracked: false}, // Staged modification
	})
	m.statusTreeIndex = 0

	var gotCmd *exec.Cmd
	m.commandRunner = func(name string, args ...string) *exec.Cmd {
		gotCmd = exec.Command(name, args...)
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
	m.focusedPane = 1
	m.statusViewport = viewport.New(40, 10)

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	if err := os.MkdirAll(wtPath, 0o700); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	m.filteredWts = []*models.WorktreeInfo{
		{Path: wtPath, Branch: "feature"},
	}
	m.selectedIndex = 0
	m.setStatusFiles([]StatusFile{
		{Filename: "file1.go", Status: "MM", IsUntracked: false}, // Both staged and unstaged
	})
	m.statusTreeIndex = 0

	var gotCmd *exec.Cmd
	m.commandRunner = func(name string, args ...string) *exec.Cmd {
		gotCmd = exec.Command(name, args...)
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
	m.focusedPane = 0 // Not status pane
	m.setStatusFiles([]StatusFile{
		{Filename: "file1.go", Status: " M", IsUntracked: false},
	})
	m.statusTreeIndex = 0

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
	m.focusedPane = 1
	m.statusViewport = viewport.New(40, 10)
	m.filteredWts = []*models.WorktreeInfo{
		{Path: cfg.WorktreeDir, Branch: "feature"},
	}
	m.selectedIndex = 0

	// Build a tree with a directory containing unstaged files
	m.setStatusFiles([]StatusFile{
		{Filename: "src/file1.go", Status: " M", IsUntracked: false},
		{Filename: "src/file2.go", Status: " M", IsUntracked: false},
	})
	m.statusTreeIndex = 0 // Select the directory

	if len(m.statusTreeFlat) < 2 || !m.statusTreeFlat[0].IsDir() {
		t.Fatal("expected directory node at index 0")
	}

	var gotCmd *exec.Cmd
	m.commandRunner = func(name string, args ...string) *exec.Cmd {
		gotCmd = exec.Command(name, args...)
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
	m.focusedPane = 1
	m.statusViewport = viewport.New(40, 10)
	m.filteredWts = []*models.WorktreeInfo{
		{Path: cfg.WorktreeDir, Branch: "feature"},
	}
	m.selectedIndex = 0

	// Build a tree with a directory containing fully staged files
	m.setStatusFiles([]StatusFile{
		{Filename: "src/file1.go", Status: "M ", IsUntracked: false},
		{Filename: "src/file2.go", Status: "A ", IsUntracked: false},
	})
	m.statusTreeIndex = 0 // Select the directory

	if len(m.statusTreeFlat) < 2 || !m.statusTreeFlat[0].IsDir() {
		t.Fatal("expected directory node at index 0")
	}

	var gotCmd *exec.Cmd
	m.commandRunner = func(name string, args ...string) *exec.Cmd {
		gotCmd = exec.Command(name, args...)
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
	m.focusedPane = 1
	m.statusViewport = viewport.New(40, 10)
	m.filteredWts = []*models.WorktreeInfo{
		{Path: cfg.WorktreeDir, Branch: "feature"},
	}
	m.selectedIndex = 0

	// Build a tree with a directory containing mixed status files
	m.setStatusFiles([]StatusFile{
		{Filename: "src/file1.go", Status: "M ", IsUntracked: false}, // Staged
		{Filename: "src/file2.go", Status: " M", IsUntracked: false}, // Unstaged
	})
	m.statusTreeIndex = 0 // Select the directory

	if len(m.statusTreeFlat) < 2 || !m.statusTreeFlat[0].IsDir() {
		t.Fatal("expected directory node at index 0")
	}

	var gotCmd *exec.Cmd
	m.commandRunner = func(name string, args ...string) *exec.Cmd {
		gotCmd = exec.Command(name, args...)
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

// TestStatusFileEnterNoFilesDoesNothing tests Enter with no status files.
func TestStatusFileEnterNoFilesDoesNothing(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.focusedPane = 1
	m.statusFiles = nil

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
	m.focusedPane = 1
	m.statusViewport = viewport.New(40, 10)

	// Simulated git status --porcelain=v2 output
	statusRaw := `1 .M N... 100644 100644 100644 abc123 abc123 modified.go
1 M. N... 100644 100644 100644 def456 def456 staged.go
? untracked.txt
1 A. N... 100644 100644 100644 ghi789 ghi789 added.go
1 .D N... 100644 100644 100644 jkl012 jkl012 deleted.go`

	_ = m.buildStatusContent(statusRaw)

	if len(m.statusFiles) != 5 {
		t.Fatalf("expected 5 status files, got %d", len(m.statusFiles))
	}

	// Check first file (modified)
	if m.statusFiles[0].Filename != "modified.go" {
		t.Fatalf("expected filename 'modified.go', got %q", m.statusFiles[0].Filename)
	}
	if m.statusFiles[0].Status != ".M" {
		t.Fatalf("expected status '.M', got %q", m.statusFiles[0].Status)
	}
	if m.statusFiles[0].IsUntracked {
		t.Fatal("expected IsUntracked to be false for modified file")
	}

	// Check untracked file
	if m.statusFiles[2].Filename != "untracked.txt" {
		t.Fatalf("expected filename 'untracked.txt', got %q", m.statusFiles[2].Filename)
	}
	if !m.statusFiles[2].IsUntracked {
		t.Fatal("expected IsUntracked to be true for untracked file")
	}
}

// TestBuildStatusContentCleanTree tests that clean working tree is handled.
func TestBuildStatusContentCleanTree(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.focusedPane = 1
	m.statusViewport = viewport.New(40, 10)
	m.statusFiles = []StatusFile{{Filename: "old.go", Status: ".M"}}
	m.statusFileIndex = 5

	result := m.buildStatusContent("")

	if len(m.statusFiles) != 0 {
		t.Fatalf("expected 0 status files for clean tree, got %d", len(m.statusFiles))
	}
	if m.statusFileIndex != 0 {
		t.Fatalf("expected statusFileIndex reset to 0, got %d", m.statusFileIndex)
	}
	if !strings.Contains(result, "Clean working tree") {
		t.Fatalf("expected 'Clean working tree' in result, got %q", result)
	}
}

func TestSearchStatusSelectsMatch(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.focusedPane = 1
	m.statusViewport = viewport.New(40, 10)
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

	if m.statusTreeIndex != 1 {
		t.Fatalf("expected statusTreeIndex 1, got %d", m.statusTreeIndex)
	}
}

// TestRenderStatusFilesHighlighting tests that selected file is highlighted.
func TestRenderStatusFilesHighlighting(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = t.TempDir()
	m := NewModel(cfg, "")
	m.focusedPane = 1
	m.statusViewport = viewport.New(40, 10)
	m.setStatusFiles([]StatusFile{
		{Filename: "file1.go", Status: ".M", IsUntracked: false},
		{Filename: "file2.go", Status: ".M", IsUntracked: false},
	})
	m.statusTreeIndex = 1

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
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = t.TempDir()
	cfg.ShowIcons = false
	m := NewModel(cfg, "")
	m.focusedPane = 1
	m.statusViewport = viewport.New(40, 10)
	m.setStatusFiles([]StatusFile{
		{Filename: "file1.go", Status: ".M", IsUntracked: false},
	})
	m.statusTreeIndex = 0

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
	m.focusedPane = 1
	m.statusViewport = viewport.New(40, 10)

	// Set index out of range before parsing
	m.statusTreeIndex = 100

	statusRaw := `1 .M N... 100644 100644 100644 abc123 abc123 file1.go
1 .M N... 100644 100644 100644 abc123 abc123 file2.go`

	_ = m.buildStatusContent(statusRaw)

	// Index should be clamped to last valid index
	if m.statusTreeIndex != 1 {
		t.Fatalf("expected statusTreeIndex clamped to 1, got %d", m.statusTreeIndex)
	}

	// Test negative index
	m.statusTreeIndex = -5
	_ = m.buildStatusContent(statusRaw)

	if m.statusTreeIndex != 0 {
		t.Fatalf("expected statusTreeIndex clamped to 0, got %d", m.statusTreeIndex)
	}
}

// TestMouseScrollNavigatesFiles tests that mouse scroll navigates tree items in pane 1.
func TestMouseScrollNavigatesFiles(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.focusedPane = 1
	m.statusViewport = viewport.New(40, 10)
	m.windowWidth = 100
	m.windowHeight = 30

	m.setStatusFiles([]StatusFile{
		{Filename: "file1.go", Status: ".M", IsUntracked: false},
		{Filename: "file2.go", Status: ".M", IsUntracked: false},
		{Filename: "file3.go", Status: ".M", IsUntracked: false},
	})
	m.statusTreeIndex = 0

	// Scroll down should increment index
	msg := tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelDown,
		X:      60, // Right side of screen (pane 1)
		Y:      5,
	}

	_, _ = m.handleMouse(msg)
	if m.statusTreeIndex != 1 {
		t.Fatalf("expected statusTreeIndex 1 after scroll down, got %d", m.statusTreeIndex)
	}

	// Scroll up should decrement index
	msg.Button = tea.MouseButtonWheelUp
	_, _ = m.handleMouse(msg)
	if m.statusTreeIndex != 0 {
		t.Fatalf("expected statusTreeIndex 0 after scroll up, got %d", m.statusTreeIndex)
	}
}

// TestBuildStatusTreeEmpty tests building tree from empty file list.
func TestBuildStatusTreeEmpty(t *testing.T) {
	tree := buildStatusTree([]StatusFile{})
	if tree == nil {
		t.Fatal("expected non-nil tree root")
	}
	if tree.Path != "" {
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
	tree := buildStatusTree(files)

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
	tree := buildStatusTree(files)

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
	tree := buildStatusTree(files)

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
	tree := buildStatusTree(files)

	// After compression, a/b/c should be one node, not three nested nodes
	flat := flattenStatusTree(tree, map[string]bool{}, 0)

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
	tree := buildStatusTree(files)

	// Without collapse: should see dir + 2 files + root.go = 4 nodes
	flatOpen := flattenStatusTree(tree, map[string]bool{}, 0)
	if len(flatOpen) != 4 {
		t.Fatalf("expected 4 nodes when expanded, got %d", len(flatOpen))
	}

	// With dir collapsed: should see dir + root.go = 2 nodes
	collapsed := map[string]bool{"dir": true}
	flatClosed := flattenStatusTree(tree, collapsed, 0)
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
	tree := buildStatusTree(files)
	flat := flattenStatusTree(tree, map[string]bool{}, 0)

	// After compression: dir/subdir (depth 0), file.go (depth 1), root.go (depth 0)
	if len(flat) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(flat))
	}

	// Root level nodes should have depth 0
	if flat[0].depth != 0 {
		t.Errorf("expected dir/subdir depth 0, got %d", flat[0].depth)
	}
	// File inside dir should have depth 1
	if flat[1].depth != 1 {
		t.Errorf("expected file.go depth 1, got %d", flat[1].depth)
	}
	// Root file should have depth 0
	if flat[2].depth != 0 {
		t.Errorf("expected root.go depth 0, got %d", flat[2].depth)
	}
}

// TestDirectoryToggleUpdatesFlat tests that toggling directory collapse updates flattened list.
func TestDirectoryToggleUpdatesFlat(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.focusedPane = 1
	m.statusViewport = viewport.New(40, 10)
	m.windowWidth = 100
	m.windowHeight = 30

	m.setStatusFiles([]StatusFile{
		{Filename: "dir/file1.go", Status: ".M"},
		{Filename: "dir/file2.go", Status: ".M"},
	})

	initialCount := len(m.statusTreeFlat)
	if initialCount != 3 { // dir + 2 files
		t.Fatalf("expected 3 nodes initially, got %d", initialCount)
	}

	// Collapse the directory
	m.statusCollapsedDirs["dir"] = true
	m.rebuildStatusTreeFlat()

	if len(m.statusTreeFlat) != 1 { // just the dir
		t.Fatalf("expected 1 node after collapse, got %d", len(m.statusTreeFlat))
	}

	// Expand again
	m.statusCollapsedDirs["dir"] = false
	m.rebuildStatusTreeFlat()

	if len(m.statusTreeFlat) != 3 {
		t.Fatalf("expected 3 nodes after expand, got %d", len(m.statusTreeFlat))
	}
}

func TestEscClearsWorktreeFilter(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.focusedPane = 0
	m.filterQuery = testFilterQuery
	m.worktrees = []*models.WorktreeInfo{
		{Path: filepath.Join(cfg.WorktreeDir, "test-wt"), Branch: testFeat},
	}
	m.updateTable()

	updated, _ := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyEsc})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}

	if updatedModel.filterQuery != "" {
		t.Fatalf("expected filter to be cleared, got %q", updatedModel.filterQuery)
	}
}

func TestEscClearsStatusFilter(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.focusedPane = 1
	m.statusFilterQuery = testFilterQuery

	updated, _ := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyEsc})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}

	if updatedModel.statusFilterQuery != "" {
		t.Fatalf("expected status filter to be cleared, got %q", updatedModel.statusFilterQuery)
	}
}

func TestEscClearsLogFilter(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.focusedPane = 2
	m.logFilterQuery = testFilterQuery

	updated, _ := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyEsc})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}

	if updatedModel.logFilterQuery != "" {
		t.Fatalf("expected log filter to be cleared, got %q", updatedModel.logFilterQuery)
	}
}

func TestEscDoesNothingWhenNoFilter(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.focusedPane = 0
	m.filterQuery = ""

	updated, _ := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyEsc})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}

	if updatedModel.filterQuery != "" {
		t.Fatalf("expected filter to remain empty, got %q", updatedModel.filterQuery)
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
	m.filterQuery = testFilterQuery
	if !m.hasActiveFilterForPane(0) {
		t.Fatal("expected active filter for pane 0")
	}

	// Set status filter
	m.statusFilterQuery = testFilterQuery
	if !m.hasActiveFilterForPane(1) {
		t.Fatal("expected active filter for pane 1")
	}

	// Set log filter
	m.logFilterQuery = testFilterQuery
	if !m.hasActiveFilterForPane(2) {
		t.Fatal("expected active filter for pane 2")
	}

	// Whitespace-only should not count as active
	m.filterQuery = "   "
	if m.hasActiveFilterForPane(0) {
		t.Fatal("expected whitespace-only filter to not be active")
	}
}

func TestZoomPaneToggle(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.focusedPane = 0

	if m.zoomedPane != -1 {
		t.Fatalf("expected zoomedPane to start at -1, got %d", m.zoomedPane)
	}

	// Press = to zoom pane 0
	updated, _ := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'='}})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	if m.zoomedPane != 0 {
		t.Fatalf("expected zoomedPane to be 0 after zoom, got %d", m.zoomedPane)
	}

	// Press = again to unzoom
	updated, _ = m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'='}})
	updatedModel, ok = updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	if m.zoomedPane != -1 {
		t.Fatalf("expected zoomedPane to be -1 after unzoom, got %d", m.zoomedPane)
	}
}

func TestZoomPaneExitsOnPaneKeys(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.focusedPane = 0
	m.zoomedPane = 0

	// Press 2 to switch to pane 2 and exit zoom
	updated, _ := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	if m.zoomedPane != -1 {
		t.Fatalf("expected zoomedPane to be -1 after pressing 2, got %d", m.zoomedPane)
	}
	if m.focusedPane != 1 {
		t.Fatalf("expected focusedPane to be 1, got %d", m.focusedPane)
	}
}

func TestZoomPaneExitsOnTabKey(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.focusedPane = 0
	m.zoomedPane = 0

	// Press tab to cycle panes and exit zoom
	updated, _ := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyTab})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	if m.zoomedPane != -1 {
		t.Fatalf("expected zoomedPane to be -1 after pressing tab, got %d", m.zoomedPane)
	}
	if m.focusedPane != 1 {
		t.Fatalf("expected focusedPane to be 1 after tab, got %d", m.focusedPane)
	}
}

func TestZoomPaneExitsOnBracketKey(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.focusedPane = 1
	m.zoomedPane = 1

	// Press [ to cycle back and exit zoom
	updated, _ := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	if m.zoomedPane != -1 {
		t.Fatalf("expected zoomedPane to be -1 after pressing [, got %d", m.zoomedPane)
	}
	if m.focusedPane != 0 {
		t.Fatalf("expected focusedPane to be 0 after [, got %d", m.focusedPane)
	}
}
