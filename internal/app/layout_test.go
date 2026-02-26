package app

import (
	"testing"

	"github.com/chmouel/lazyworktree/internal/app/state"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestComputeTopLayout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		width       int
		height      int
		focusedPane int
	}{
		{name: "standard terminal", width: 120, height: 40, focusedPane: 0},
		{name: "wide terminal", width: 200, height: 50, focusedPane: 0},
		{name: "narrow terminal", width: 80, height: 24, focusedPane: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := &config.AppConfig{
				WorktreeDir: t.TempDir(),
				Layout:      "top",
			}
			m := NewModel(cfg, "")
			m.state.view.WindowWidth = tt.width
			m.state.view.WindowHeight = tt.height
			m.state.view.FocusedPane = tt.focusedPane
			// Add status files so git status pane is visible (3-way split)
			m.state.data.statusFilesAll = []StatusFile{{Filename: "file.go", Status: ".M"}}

			layout := m.computeLayout()

			assert.Equal(t, state.LayoutTop, layout.layoutMode)
			assert.Equal(t, tt.width, layout.width)
			assert.Equal(t, tt.height, layout.height)

			// Top height + gap + bottom height should equal body height
			assert.Equal(t, layout.bodyHeight, layout.topHeight+layout.gapY+layout.bottomHeight)

			// Bottom left + gaps + bottom middle + bottom right should equal total width
			assert.Equal(t, tt.width, layout.bottomLeftWidth+layout.gapX+layout.bottomMiddleWidth+layout.gapX+layout.bottomRightWidth)

			// Minimum constraints
			assert.GreaterOrEqual(t, layout.topHeight, 4)
			assert.GreaterOrEqual(t, layout.bottomHeight, 6)

			// Inner dimensions should be positive
			assert.Positive(t, layout.topInnerWidth)
			assert.Positive(t, layout.topInnerHeight)
			assert.Positive(t, layout.bottomLeftInnerWidth)
			assert.Positive(t, layout.bottomMiddleInnerWidth)
			assert.Positive(t, layout.bottomRightInnerWidth)
			assert.Positive(t, layout.bottomLeftInnerHeight)
			assert.Positive(t, layout.bottomMiddleInnerHeight)
			assert.Positive(t, layout.bottomRightInnerHeight)
		})
	}
}

func TestComputeTopLayoutFocusDynamic(t *testing.T) {
	t.Parallel()

	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		Layout:      "top",
	}

	tests := []struct {
		name        string
		focusedPane int
	}{
		{name: "worktree focused", focusedPane: 0},
		{name: "status focused", focusedPane: 1},
		{name: "commit focused", focusedPane: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := NewModel(cfg, "")
			m.state.view.WindowWidth = 120
			m.state.view.WindowHeight = 40
			m.state.view.FocusedPane = tt.focusedPane
			// Add status files so git status pane is visible (3-way split)
			m.state.data.statusFilesAll = []StatusFile{{Filename: "file.go", Status: ".M"}}

			layout := m.computeLayout()

			assert.Equal(t, state.LayoutTop, layout.layoutMode)

			// Verify focus-based ratio changes
			switch tt.focusedPane {
			case 0:
				// Worktree focused: top gets more space
				assert.Greater(t, layout.topHeight, layout.bottomHeight/2)
			case 1:
				// Status focused: bottom left (status) should be wider
				assert.Greater(t, layout.bottomLeftWidth, layout.bottomRightWidth)
			case 3:
				// Commit focused: bottom right (commit) should get more space than others
				assert.Greater(t, layout.bottomRightWidth, layout.bottomLeftWidth)
			}
		})
	}
}

func TestApplyLayoutTopMode(t *testing.T) {
	t.Parallel()

	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		Layout:      "top",
	}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40
	// Add status files so git status pane is visible (3-way split)
	m.state.data.statusFilesAll = []StatusFile{{Filename: "file.go", Status: ".M"}}

	layout := m.computeLayout()
	m.applyLayout(layout)

	// Worktree table should use full top width
	assert.Equal(t, layout.topInnerWidth, m.state.ui.worktreeTable.Width())

	// Log table should use bottom right width
	assert.Equal(t, layout.bottomRightInnerWidth, m.state.ui.logTable.Width())
}

func TestLayoutToggle(t *testing.T) {
	t.Parallel()

	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40

	// Default layout
	assert.Equal(t, state.LayoutDefault, m.state.view.Layout)

	layout := m.computeLayout()
	assert.Equal(t, state.LayoutDefault, layout.layoutMode)

	// Toggle to top
	m.state.view.Layout = state.LayoutTop
	layout = m.computeLayout()
	assert.Equal(t, state.LayoutTop, layout.layoutMode)

	// Toggle back to default
	m.state.view.Layout = state.LayoutDefault
	layout = m.computeLayout()
	assert.Equal(t, state.LayoutDefault, layout.layoutMode)
}

func TestDefaultLayoutUnchanged(t *testing.T) {
	t.Parallel()

	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40

	layout := m.computeLayout()

	// Verify default layout still works as before
	assert.Equal(t, state.LayoutDefault, layout.layoutMode)
	assert.Positive(t, layout.leftWidth)
	assert.Positive(t, layout.rightWidth)
	assert.Equal(t, 120, layout.leftWidth+layout.gapX+layout.rightWidth)
}

func TestZoomModeIgnoresTopLayout(t *testing.T) {
	t.Parallel()

	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		Layout:      "top",
	}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40
	m.state.view.ZoomedPane = 0

	layout := m.computeLayout()

	// Zoom mode should return early before top layout computation
	assert.Equal(t, state.LayoutDefault, layout.layoutMode)
	assert.Equal(t, 120, layout.leftWidth)
}

func TestDefaultLayoutWithNotes(t *testing.T) {
	t.Parallel()
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40
	m.state.view.FocusedPane = 0

	wt := &models.WorktreeInfo{Path: "/tmp/wt-layout", Branch: "feat"}
	m.state.data.filteredWts = []*models.WorktreeInfo{wt}
	m.state.data.selectedIndex = 0
	m.worktreeNotes[worktreeNoteKey(wt.Path)] = models.WorktreeNote{Note: "a note"}

	layout := m.computeLayout()

	assert.True(t, layout.hasNotes)
	assert.Positive(t, layout.leftTopHeight)
	assert.Positive(t, layout.leftBottomHeight)
	assert.Positive(t, layout.leftTopInnerHeight)
	assert.Positive(t, layout.leftBottomInnerHeight)

	// Top + gap + bottom should equal body height
	assert.Equal(t, layout.bodyHeight, layout.leftTopHeight+layout.gapY+layout.leftBottomHeight)
}

func TestDefaultLayoutWithoutNotes(t *testing.T) {
	t.Parallel()
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40
	m.state.view.FocusedPane = 0

	wt := &models.WorktreeInfo{Path: "/tmp/wt-no-notes", Branch: "feat"}
	m.state.data.filteredWts = []*models.WorktreeInfo{wt}
	m.state.data.selectedIndex = 0

	layout := m.computeLayout()

	assert.False(t, layout.hasNotes)
	assert.Equal(t, layout.bodyHeight, layout.leftTopHeight)
	assert.Equal(t, 0, layout.leftBottomHeight)
}

func TestTopLayoutWithNotes(t *testing.T) {
	t.Parallel()
	cfg := &config.AppConfig{WorktreeDir: t.TempDir(), Layout: "top"}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40
	m.state.view.FocusedPane = 0
	m.state.view.Layout = state.LayoutTop

	wt := &models.WorktreeInfo{Path: "/tmp/wt-top", Branch: "feat"}
	m.state.data.filteredWts = []*models.WorktreeInfo{wt}
	m.state.data.selectedIndex = 0
	m.worktreeNotes[worktreeNoteKey(wt.Path)] = models.WorktreeNote{Note: "a note"}

	layout := m.computeLayout()

	assert.Equal(t, state.LayoutTop, layout.layoutMode)
	assert.True(t, layout.hasNotes)
	assert.Positive(t, layout.notesRowHeight)
	assert.Positive(t, layout.notesRowInnerHeight)
	assert.Positive(t, layout.notesRowInnerWidth)

	// All vertical sections must sum to the full body height.
	assert.Equal(t, layout.bodyHeight, layout.topHeight+layout.gapY+layout.notesRowHeight+layout.gapY+layout.bottomHeight)
}

func TestNotesPaneFocusIncreasesSize(t *testing.T) {
	t.Parallel()
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40

	wt := &models.WorktreeInfo{Path: "/tmp/wt-focus", Branch: "feat"}
	m.state.data.filteredWts = []*models.WorktreeInfo{wt}
	m.state.data.selectedIndex = 0
	m.worktreeNotes[worktreeNoteKey(wt.Path)] = models.WorktreeNote{Note: "a note"}

	m.state.view.FocusedPane = 0
	layoutUnfocused := m.computeLayout()

	m.state.view.FocusedPane = 4
	layoutFocused := m.computeLayout()

	assert.Greater(t, layoutFocused.leftBottomHeight, layoutUnfocused.leftBottomHeight,
		"notes pane should be larger when focused")
}

func TestDefaultLayoutWithoutGitStatus(t *testing.T) {
	t.Parallel()
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40
	m.state.view.FocusedPane = 0

	layout := m.computeLayout()

	assert.False(t, layout.hasGitStatus)
	// 2-way split: rightMiddleHeight should be 0
	assert.Equal(t, 0, layout.rightMiddleHeight)
	// Top + gap + bottom should equal body height (one gap only)
	assert.Equal(t, layout.bodyHeight, layout.rightTopHeight+layout.gapY+layout.rightBottomHeight)
}

func TestDefaultLayoutWithGitStatus(t *testing.T) {
	t.Parallel()
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40
	m.state.view.FocusedPane = 0
	m.state.data.statusFilesAll = []StatusFile{{Filename: "file.go", Status: ".M"}}

	layout := m.computeLayout()

	assert.True(t, layout.hasGitStatus)
	assert.Positive(t, layout.rightMiddleHeight)
	// 3-way split: top + gap + middle + gap + bottom should equal body height
	assert.Equal(t, layout.bodyHeight, layout.rightTopHeight+layout.gapY+layout.rightMiddleHeight+layout.gapY+layout.rightBottomHeight)
}

func TestTopLayoutWithoutGitStatus(t *testing.T) {
	t.Parallel()
	cfg := &config.AppConfig{WorktreeDir: t.TempDir(), Layout: "top"}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40
	m.state.view.FocusedPane = 0

	layout := m.computeLayout()

	assert.Equal(t, state.LayoutTop, layout.layoutMode)
	assert.False(t, layout.hasGitStatus)
	// 2-way split: bottomMiddleWidth should be 0
	assert.Equal(t, 0, layout.bottomMiddleWidth)
	// Left + gap + right should equal total width (one gap)
	assert.Equal(t, 120, layout.bottomLeftWidth+layout.gapX+layout.bottomRightWidth)
}

func TestTopLayoutWithGitStatus(t *testing.T) {
	t.Parallel()
	cfg := &config.AppConfig{WorktreeDir: t.TempDir(), Layout: "top"}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40
	m.state.view.FocusedPane = 0
	m.state.data.statusFilesAll = []StatusFile{{Filename: "file.go", Status: ".M"}}

	layout := m.computeLayout()

	assert.Equal(t, state.LayoutTop, layout.layoutMode)
	assert.True(t, layout.hasGitStatus)
	assert.Positive(t, layout.bottomMiddleWidth)
	// 3-way split: left + gap + middle + gap + right should equal total width
	assert.Equal(t, 120, layout.bottomLeftWidth+layout.gapX+layout.bottomMiddleWidth+layout.gapX+layout.bottomRightWidth)
}
