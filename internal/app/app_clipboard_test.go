package app

import (
	"testing"

	"github.com/chmouel/lazyworktree/internal/app/services"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestYankContextual_Pane0_CopiesPath(t *testing.T) {
	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir()}, "")
	m.state.view.FocusedPane = 0
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: "/tmp/my-worktree", Branch: "feature"},
	}
	m.state.data.selectedIndex = 0

	cmd := m.yankContextual()
	assert.NotNil(t, cmd, "expected a command to be returned")
}

func TestYankContextual_Pane2_CopiesSHA(t *testing.T) {
	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir()}, "")
	m.state.view.FocusedPane = 3
	m.state.data.logEntries = []commitLogEntry{
		{sha: "abc123def", message: "test commit"},
	}

	cmd := m.yankContextual()
	assert.NotNil(t, cmd, "expected a command to be returned")
}

func TestYankContextual_Pane1_CopiesFilePath(t *testing.T) {
	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir()}, "")
	m.state.view.FocusedPane = 2
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: "/tmp/wt", Branch: "main"},
	}
	m.state.data.selectedIndex = 0
	m.state.services.statusTree = services.NewStatusService()
	sf := models.StatusFile{Filename: "cmd/main.go", Status: "M."}
	m.state.services.statusTree.TreeFlat = []*services.StatusTreeNode{
		{Path: "cmd/main.go", File: &sf},
	}
	m.state.services.statusTree.Index = 0

	cmd := m.yankContextual()
	assert.NotNil(t, cmd, "expected a command to be returned")
}

func TestYankContextual_EmptyList_ReturnsNil(t *testing.T) {
	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir()}, "")
	m.state.view.FocusedPane = 0
	m.state.data.filteredWts = []*models.WorktreeInfo{}
	m.state.data.selectedIndex = 0

	cmd := m.yankContextual()
	assert.Nil(t, cmd)
}

func TestYankBranch(t *testing.T) {
	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir()}, "")
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: "/tmp/wt", Branch: "feature-branch"},
	}
	m.state.data.selectedIndex = 0

	cmd := m.yankBranch()
	assert.NotNil(t, cmd, "expected a command to be returned")
}

func TestYankBranch_NoBranch_ReturnsNil(t *testing.T) {
	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir()}, "")
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: "/tmp/wt", Branch: ""},
	}
	m.state.data.selectedIndex = 0

	cmd := m.yankBranch()
	assert.Nil(t, cmd)
}

func TestYankPRURL(t *testing.T) {
	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir()}, "")
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: "/tmp/wt", Branch: "feat", PR: &models.PRInfo{URL: "https://github.com/org/repo/pull/42"}},
	}
	m.state.data.selectedIndex = 0

	cmd := m.yankPRURL()
	assert.NotNil(t, cmd, "expected a command to be returned")
}

func TestYankPRURL_NoPR_ReturnsNil(t *testing.T) {
	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir()}, "")
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: "/tmp/wt", Branch: "feat"},
	}
	m.state.data.selectedIndex = 0

	cmd := m.yankPRURL()
	assert.Nil(t, cmd)
}
