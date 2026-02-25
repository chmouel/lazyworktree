package app

import (
	"path/filepath"
	"strings"
	"testing"

	"charm.land/bubbles/v2/viewport"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/theme"
)

const testNavFilterQuery = "test"

func TestWorkspaceNameTruncation(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		isMain        bool
		maxNameLength int
		expected      string
	}{
		{
			name:          "short name unchanged",
			input:         "my-worktree",
			isMain:        false,
			maxNameLength: 95,
			expected:      " my-worktree",
		},
		{
			name:          "main worktree unchanged",
			input:         "",
			isMain:        true,
			maxNameLength: 95,
			expected:      " main",
		},
		{
			name:          "exactly 95 chars unchanged",
			input:         strings.Repeat("a", 94),
			isMain:        false,
			maxNameLength: 95,
			expected:      " " + strings.Repeat("a", 94),
		},
		{
			name:          "96 chars truncated to 95 plus ellipsis",
			input:         strings.Repeat("a", 95),
			isMain:        false,
			maxNameLength: 95,
			expected:      " " + strings.Repeat("a", 94) + "...",
		},
		{
			name:          "over 100 chars truncated to 95 plus ellipsis",
			input:         strings.Repeat("a", 120),
			isMain:        false,
			maxNameLength: 95,
			expected:      " " + strings.Repeat("a", 94) + "...",
		},
		{
			name:          "unicode characters handled correctly",
			input:         strings.Repeat("😀", 100),
			isMain:        false,
			maxNameLength: 95,
			expected:      " " + strings.Repeat("😀", 94) + "...",
		},
		{
			name:          "mixed ascii and unicode",
			input:         "abc" + strings.Repeat("😀", 100),
			isMain:        false,
			maxNameLength: 95,
			expected:      " abc" + strings.Repeat("😀", 91) + "...",
		},
		{
			name:          "truncation disabled with 0",
			input:         strings.Repeat("a", 200),
			isMain:        false,
			maxNameLength: 0,
			expected:      " " + strings.Repeat("a", 200),
		},
		{
			name:          "custom limit of 50",
			input:         strings.Repeat("a", 100),
			isMain:        false,
			maxNameLength: 50,
			expected:      " " + strings.Repeat("a", 49) + "...",
		},
		{
			name:          "custom limit 50 with unicode",
			input:         strings.Repeat("😀", 100),
			isMain:        false,
			maxNameLength: 50,
			expected:      " " + strings.Repeat("😀", 49) + "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the logic from updateTable() function
			var name string
			if tt.isMain {
				name = " " + mainWorktreeName
			} else {
				name = " " + tt.input
			}

			// Apply truncation logic (matching the implementation in updateTable)
			if tt.maxNameLength > 0 {
				nameRunes := []rune(name)
				if len(nameRunes) > tt.maxNameLength {
					name = string(nameRunes[:tt.maxNameLength]) + "..."
				}
			}

			if name != tt.expected {
				t.Errorf("expected %q, got %q (expected length: %d, got length: %d)",
					tt.expected, name, len([]rune(tt.expected)), len([]rune(name)))
			}
		})
	}
}

func TestRenderPaneTitleBasic(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.theme = theme.Dracula()

	// Test basic title rendering (no filter, no zoom)
	title := m.renderPaneBlock(1, "Worktrees", true, 100, 10, "")

	if !strings.Contains(title, "[1]") {
		t.Error("Expected title to contain pane number [1]")
	}
	if !strings.Contains(title, "Worktrees") {
		t.Error("Expected title to contain pane name")
	}
	if strings.Contains(title, "Filtered") {
		t.Error("Expected no filter indicator")
	}
	if strings.Contains(title, "Zoomed") {
		t.Error("Expected no zoom indicator")
	}
}

func TestRenderPaneTitleWithFilter(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.theme = theme.Dracula()
	m.state.services.filter.FilterQuery = testNavFilterQuery // Activate filter for pane 0
	m.state.view.ShowingFilter = false
	m.state.view.ShowingSearch = false

	title := m.renderPaneBlock(1, "Worktrees", true, 100, 10, "")

	if !strings.Contains(title, "Filtered") {
		t.Error("Expected filter indicator to show 'Filtered'")
	}
	if !strings.Contains(title, "Esc") {
		t.Error("Expected filter indicator to show 'Esc' key")
	}
	if !strings.Contains(title, "Clear") {
		t.Error("Expected filter indicator to show 'Clear' action")
	}
}

func TestRenderPaneTitleWithZoom(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.theme = theme.Dracula()
	m.state.view.ZoomedPane = 0 // Zoom pane 0

	title := m.renderPaneBlock(1, "Worktrees", true, 100, 10, "")

	if !strings.Contains(title, "Zoomed") {
		t.Error("Expected zoom indicator to show 'Zoomed'")
	}
	if !strings.Contains(title, "=") {
		t.Error("Expected zoom indicator to show '=' key")
	}
	if !strings.Contains(title, "Unzoom") {
		t.Error("Expected zoom indicator to show 'Unzoom' action")
	}
}

func TestRenderPaneTitleWithFilterAndZoom(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.theme = theme.Dracula()
	m.state.services.filter.FilterQuery = testNavFilterQuery // Activate filter for pane 0
	m.state.view.ShowingFilter = false
	m.state.view.ShowingSearch = false
	m.state.view.ZoomedPane = 0 // Zoom pane 0

	title := m.renderPaneBlock(1, "Worktrees", true, 100, 10, "")

	// Should show both indicators
	if !strings.Contains(title, "Filtered") {
		t.Error("Expected filter indicator when both filter and zoom are active")
	}
	if !strings.Contains(title, "Zoomed") {
		t.Error("Expected zoom indicator when both filter and zoom are active")
	}
}

func TestRenderPaneTitleNoZoomWhenDifferentPane(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.theme = theme.Dracula()
	m.state.view.ZoomedPane = 1 // Zoom pane 1 (status)

	// Render title for pane 0 (worktrees)
	title := m.renderPaneBlock(1, "Worktrees", true, 100, 10, "")

	if strings.Contains(title, "Zoomed") {
		t.Error("Expected no zoom indicator for unzoomed pane")
	}
}

func TestRenderPaneTitleUsesAccentFg(t *testing.T) {
	// Test with a light theme to ensure AccentFg (white) is used instead of black
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.theme = theme.CleanLight()
	m.state.view.ZoomedPane = 0

	title := m.renderPaneBlock(1, "Worktrees", true, 100, 10, "")

	// The title should contain styling - we can't directly test the color
	// but we can verify the indicator is present and properly formatted
	if !strings.Contains(title, "Zoomed") || !strings.Contains(title, "=") {
		t.Error("Expected properly formatted zoom indicator with theme colors")
	}

	// Test with filter too
	m.state.services.filter.FilterQuery = "test"
	m.state.view.ShowingFilter = false
	m.state.view.ShowingSearch = false

	title = m.renderPaneBlock(1, "Worktrees", true, 100, 10, "")

	if !strings.Contains(title, "Filtered") || !strings.Contains(title, "Esc") {
		t.Error("Expected properly formatted filter indicator with theme colors")
	}
}

func TestRenderZoomedLeftPane(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 100
	m.state.view.WindowHeight = 40
	m.state.view.ZoomedPane = 0

	// Set up worktree table
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: filepath.Join(cfg.WorktreeDir, "wt1"), Branch: "branch1"},
	}
	m.state.data.filteredWts = m.state.data.worktrees
	m.state.ui.worktreeTable.SetWidth(80)
	m.updateTable()
	m.updateTableColumns(m.state.ui.worktreeTable.Width())

	layout := layoutDims{
		leftWidth:      80,
		leftInnerWidth: 78,
		bodyHeight:     30,
	}

	result := m.renderZoomedLeftPane(layout)
	if result == "" {
		t.Error("expected non-empty render result")
	}
	if !strings.Contains(result, "Worktrees") {
		t.Error("expected render to contain 'Worktrees' title")
	}
}

func TestRenderZoomedRightTopPane(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 100
	m.state.view.WindowHeight = 40
	m.state.view.ZoomedPane = 1

	m.infoContent = "Test info"
	m.statusContent = "Test status content"
	m.state.ui.statusViewport = viewport.New(viewport.WithWidth(50), viewport.WithHeight(20))

	layout := layoutDims{
		rightWidth:          80,
		rightInnerWidth:     78,
		rightTopInnerHeight: 30,
		bodyHeight:          30,
	}

	result := m.renderZoomedRightTopPane(layout)
	if result == "" {
		t.Error("expected non-empty render result")
	}
	if !strings.Contains(result, "Status") {
		t.Error("expected render to contain 'Status' title")
	}
}

func TestRenderZoomedRightBottomPane(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 100
	m.state.view.WindowHeight = 40
	m.state.view.ZoomedPane = 3

	m.state.data.logEntries = []commitLogEntry{
		{sha: "abc123", message: "commit 1"},
		{sha: "def456", message: "commit 2"},
	}
	m.setLogEntries(m.state.data.logEntries, false)
	m.state.ui.logTable.SetWidth(80)
	m.updateLogColumns(m.state.ui.logTable.Width())

	layout := layoutDims{
		rightWidth:      80,
		rightInnerWidth: 78,
		bodyHeight:      30,
	}

	result := m.renderZoomedRightBottomPane(layout)
	if result == "" {
		t.Error("expected non-empty render result")
	}
	if !strings.Contains(result, "Commit") {
		t.Error("expected render to contain 'Commit' title")
	}
}
