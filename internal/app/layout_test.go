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

func TestCustomLayoutSizes(t *testing.T) {
	t.Parallel()
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		LayoutSizes: &config.LayoutSizes{Worktrees: 70},
	}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40
	m.state.view.FocusedPane = 0

	layoutCustom := m.computeLayout()

	// Now compare with default (nil LayoutSizes)
	cfgDefault := &config.AppConfig{WorktreeDir: t.TempDir()}
	mDefault := NewModel(cfgDefault, "")
	mDefault.state.view.WindowWidth = 120
	mDefault.state.view.WindowHeight = 40
	mDefault.state.view.FocusedPane = 0

	layoutDefault := mDefault.computeLayout()

	// Custom worktrees=70 should give more left space than default
	assert.Greater(t, layoutCustom.leftWidth, layoutDefault.leftWidth,
		"worktrees=70 should produce wider left pane than default")
	// Total width must still be correct
	assert.Equal(t, 120, layoutCustom.leftWidth+layoutCustom.gapX+layoutCustom.rightWidth)
}

func TestCustomLayoutSizesRightColumn(t *testing.T) {
	t.Parallel()
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		LayoutSizes: &config.LayoutSizes{Info: 60, GitStatus: 20, Commit: 20},
	}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40
	m.state.view.FocusedPane = 0
	m.state.data.statusFilesAll = []StatusFile{{Filename: "file.go", Status: ".M"}}

	layout := m.computeLayout()

	assert.True(t, layout.hasGitStatus)
	// Info pane should get more height than git status and commit
	assert.Greater(t, layout.rightTopHeight, layout.rightMiddleHeight,
		"info=60 should produce taller info pane than git_status=20")
	// Heights must sum correctly
	assert.Equal(t, layout.bodyHeight,
		layout.rightTopHeight+layout.gapY+layout.rightMiddleHeight+layout.gapY+layout.rightBottomHeight)
}

func TestCustomLayoutSizesTopLayout(t *testing.T) {
	t.Parallel()
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		Layout:      "top",
		LayoutSizes: &config.LayoutSizes{Worktrees: 50},
	}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40
	m.state.view.FocusedPane = 0
	m.state.data.statusFilesAll = []StatusFile{{Filename: "file.go", Status: ".M"}}

	layoutCustom := m.computeLayout()

	// Compare with default
	cfgDefault := &config.AppConfig{WorktreeDir: t.TempDir(), Layout: "top"}
	mDefault := NewModel(cfgDefault, "")
	mDefault.state.view.WindowWidth = 120
	mDefault.state.view.WindowHeight = 40
	mDefault.state.view.FocusedPane = 0
	mDefault.state.data.statusFilesAll = []StatusFile{{Filename: "file.go", Status: ".M"}}

	layoutDefault := mDefault.computeLayout()

	assert.Equal(t, state.LayoutTop, layoutCustom.layoutMode)
	// worktrees=50 should give more top space than default (30)
	assert.Greater(t, layoutCustom.topHeight, layoutDefault.topHeight,
		"worktrees=50 should produce taller top pane than default 30")
	// Must sum correctly
	assert.Equal(t, layoutCustom.bodyHeight, layoutCustom.topHeight+layoutCustom.gapY+layoutCustom.bottomHeight)
}

func TestCustomLayoutSizesFocusStillWorks(t *testing.T) {
	t.Parallel()
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		LayoutSizes: &config.LayoutSizes{Worktrees: 60},
	}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40

	// Unfocused (pane 0)
	m.state.view.FocusedPane = 0
	layoutUnfocused := m.computeLayout()

	// Right pane focused — left should shrink
	m.state.view.FocusedPane = 1
	layoutFocused := m.computeLayout()

	assert.Less(t, layoutFocused.leftWidth, layoutUnfocused.leftWidth,
		"focusing right pane should shrink left pane even with custom sizes")
	// Both must have valid totals
	assert.Equal(t, 120, layoutUnfocused.leftWidth+layoutUnfocused.gapX+layoutUnfocused.rightWidth)
	assert.Equal(t, 120, layoutFocused.leftWidth+layoutFocused.gapX+layoutFocused.rightWidth)
}

func TestCustomLayoutSizesNilUsesDefaults(t *testing.T) {
	t.Parallel()

	// nil LayoutSizes
	cfg1 := &config.AppConfig{WorktreeDir: t.TempDir()}
	m1 := NewModel(cfg1, "")
	m1.state.view.WindowWidth = 120
	m1.state.view.WindowHeight = 40
	m1.state.view.FocusedPane = 0

	// Explicit defaults matching hardcoded values
	cfg2 := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		LayoutSizes: &config.LayoutSizes{Worktrees: 55, Info: 30, GitStatus: 40, Commit: 30, Notes: 30},
	}
	m2 := NewModel(cfg2, "")
	m2.state.view.WindowWidth = 120
	m2.state.view.WindowHeight = 40
	m2.state.view.FocusedPane = 0

	layout1 := m1.computeLayout()
	layout2 := m2.computeLayout()

	assert.Equal(t, layout1.leftWidth, layout2.leftWidth,
		"nil LayoutSizes should produce same left width as explicit defaults")
	assert.Equal(t, layout1.rightWidth, layout2.rightWidth)
}

func TestCustomLayoutSizesNotes(t *testing.T) {
	t.Parallel()
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		LayoutSizes: &config.LayoutSizes{Notes: 50},
	}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40
	m.state.view.FocusedPane = 0

	wt := &models.WorktreeInfo{Path: "/tmp/wt-notes-custom", Branch: "feat"}
	m.state.data.filteredWts = []*models.WorktreeInfo{wt}
	m.state.data.selectedIndex = 0
	m.worktreeNotes[worktreeNoteKey(wt.Path)] = models.WorktreeNote{Note: "a note"}

	layoutCustom := m.computeLayout()

	// Compare with default
	cfgDefault := &config.AppConfig{WorktreeDir: t.TempDir()}
	mDefault := NewModel(cfgDefault, "")
	mDefault.state.view.WindowWidth = 120
	mDefault.state.view.WindowHeight = 40
	mDefault.state.view.FocusedPane = 0
	mDefault.state.data.filteredWts = []*models.WorktreeInfo{wt}
	mDefault.state.data.selectedIndex = 0
	mDefault.worktreeNotes[worktreeNoteKey(wt.Path)] = models.WorktreeNote{Note: "a note"}

	layoutDefault := mDefault.computeLayout()

	assert.True(t, layoutCustom.hasNotes)
	assert.Greater(t, layoutCustom.leftBottomHeight, layoutDefault.leftBottomHeight,
		"notes=50 should give bigger notes pane than default 30")
	assert.Equal(t, layoutCustom.bodyHeight,
		layoutCustom.leftTopHeight+layoutCustom.gapY+layoutCustom.leftBottomHeight)
}

func TestResizeOffsetWidensLeftPane(t *testing.T) {
	t.Parallel()
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40
	m.state.view.FocusedPane = 0

	layoutBase := m.computeLayout()

	m.state.view.ResizeOffset = 12
	layoutGrown := m.computeLayout()

	assert.Greater(t, layoutGrown.leftWidth, layoutBase.leftWidth,
		"positive offset should widen left pane")
	assert.Equal(t, 120, layoutGrown.leftWidth+layoutGrown.gapX+layoutGrown.rightWidth)
}

func TestResizeOffsetShrinksLeftPane(t *testing.T) {
	t.Parallel()
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40
	m.state.view.FocusedPane = 0

	layoutBase := m.computeLayout()

	m.state.view.ResizeOffset = -12
	layoutShrunk := m.computeLayout()

	assert.Less(t, layoutShrunk.leftWidth, layoutBase.leftWidth,
		"negative offset should shrink left pane")
	assert.Equal(t, 120, layoutShrunk.leftWidth+layoutShrunk.gapX+layoutShrunk.rightWidth)
}

func TestResizeOffsetRespectsMinWidths(t *testing.T) {
	t.Parallel()
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40
	m.state.view.FocusedPane = 0

	// Extreme negative offset — clamping should keep both panes above minimum
	m.state.view.ResizeOffset = -80
	layout := m.computeLayout()
	assert.GreaterOrEqual(t, layout.leftWidth, minLeftPaneWidth)
	assert.GreaterOrEqual(t, layout.rightWidth, minRightPaneWidth)
}

func TestResizeOffsetResetsOnWindowResize(t *testing.T) {
	t.Parallel()
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40
	m.state.view.ResizeOffset = 20

	m.setWindowSize(100, 30)

	assert.Equal(t, 0, m.state.view.ResizeOffset)
	assert.Equal(t, 100, m.state.view.WindowWidth)
	assert.Equal(t, 30, m.state.view.WindowHeight)
}

func TestResizeOffsetTopLayoutAffectsTopHeight(t *testing.T) {
	t.Parallel()
	cfg := &config.AppConfig{WorktreeDir: t.TempDir(), Layout: "top"}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40
	m.state.view.FocusedPane = 0
	m.state.data.statusFilesAll = []StatusFile{{Filename: "file.go", Status: ".M"}}

	layoutBase := m.computeLayout()

	m.state.view.ResizeOffset = 8
	layoutGrown := m.computeLayout()

	assert.Greater(t, layoutGrown.topHeight, layoutBase.topHeight,
		"positive offset should increase top height in top layout")
	assert.Equal(t, layoutGrown.bodyHeight, layoutGrown.topHeight+layoutGrown.gapY+layoutGrown.bottomHeight)
}
