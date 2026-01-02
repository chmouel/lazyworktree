package app

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

const (
	testFeat = "feat"
	testWt1  = "wt1"
	testWt2  = "wt2"
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

func TestFilterEnterSelectsFirstMatch(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir:      t.TempDir(),
		SortByActive:     false,
		SearchAutoSelect: true,
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

	if cmd == nil {
		t.Fatal("expected quit command to be returned")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected quit message, got %T", msg)
	}
	expected := filepath.Join(cfg.WorktreeDir, "a-worktree")
	if m.selectedPath != expected {
		t.Fatalf("expected selected path %q, got %q", expected, m.selectedPath)
	}
}

func TestFilterCtrlJKMovesSelection(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir:  t.TempDir(),
		SortByActive: false,
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

	updated, _ := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyCtrlJ})
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

	updated, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyCtrlK})
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

func TestHandleCachedWorktreesUpdatesState(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	const refreshingStatus = "Refreshing worktrees..."
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
	if m.statusContent != refreshingStatus {
		t.Fatalf("unexpected status content: %q", m.statusContent)
	}
	if !strings.Contains(m.infoContent, "wt1") {
		t.Fatalf("expected info content to include worktree path, got %q", m.infoContent)
	}
}

func TestHandlePRDataLoadedUpdatesTable(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
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
}

func TestHandleCIStatusLoadedUpdatesCache(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
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
}
