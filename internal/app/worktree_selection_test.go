package app

import (
	"testing"

	"charm.land/bubbles/v2/table"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

func TestDetermineCurrentWorktreePrefersSelection(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	main := &models.WorktreeInfo{Path: "/tmp/main", Branch: "main", IsMain: true}
	feature := &models.WorktreeInfo{Path: "/tmp/feature", Branch: "feature"}
	m.state.data.worktrees = []*models.WorktreeInfo{main, feature}
	m.state.data.filteredWts = m.state.data.worktrees

	rows := []table.Row{
		{"main"},
		{"feature"},
	}
	m.state.ui.worktreeTable.SetRows(rows)
	m.state.ui.worktreeTable.SetCursor(1)

	got := m.determineCurrentWorktree()
	if got != feature {
		t.Fatalf("expected selected worktree, got %v", got)
	}
}
