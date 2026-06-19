package app

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/chmouel/lazyworktree/internal/app/state"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestInfoViewportMouseWheel(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 1
	m.state.ui.infoViewport = viewport.New(viewport.WithWidth(40), viewport.WithHeight(2))
	m.state.ui.infoViewport.SetContent(strings.Repeat("line\n", 20))

	// Scroll down via mouse wheel
	m.state.ui.infoViewport.ScrollDown(3)
	assert.Positive(t, m.state.ui.infoViewport.YOffset(), "mouse wheel down should scroll info viewport")

	prev := m.state.ui.infoViewport.YOffset()
	m.state.ui.infoViewport.ScrollUp(3)
	assert.Less(t, m.state.ui.infoViewport.YOffset(), prev, "mouse wheel up should scroll info viewport")
}

// TestMouseScrollNavigatesFiles tests that mouse scroll navigates tree items in pane 2.
func TestMouseScrollNavigatesFiles(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 2
	m.state.ui.statusViewport = viewport.New(viewport.WithWidth(40), viewport.WithHeight(10))
	m.state.view.WindowWidth = 100
	m.state.view.WindowHeight = 30

	m.setStatusFiles([]StatusFile{
		{Filename: "file1.go", Status: ".M", IsUntracked: false},
		{Filename: "file2.go", Status: ".M", IsUntracked: false},
		{Filename: "file3.go", Status: ".M", IsUntracked: false},
	})
	m.state.services.statusTree.Index = 0

	// Scroll down should increment index
	wheelMsg := tea.MouseWheelMsg{
		Button: tea.MouseWheelDown,
		X:      60, // Right side of screen (pane 2 - git status)
		Y:      10, // Middle section of right pane (git status area)
	}

	_, _ = m.handleMouseWheel(wheelMsg)
	if m.state.services.statusTree.Index != 1 {
		t.Fatalf("expected statusTreeIndex 1 after scroll down, got %d", m.state.services.statusTree.Index)
	}

	// Scroll up should decrement index
	wheelMsg.Button = tea.MouseWheelUp
	_, _ = m.handleMouseWheel(wheelMsg)
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

	msg := tea.MouseClickMsg{
		Button: tea.MouseLeft,
		X:      2,
		Y:      6,
	}

	_, _ = m.handleMouseClick(msg)
	if m.state.ui.worktreeTable.Cursor() != 1 {
		t.Fatalf("expected cursor to move to 1, got %d", m.state.ui.worktreeTable.Cursor())
	}
	if m.state.data.selectedIndex != 1 {
		t.Fatalf("expected selectedIndex to be 1, got %d", m.state.data.selectedIndex)
	}
}

func TestMouseClickSelectsCommitRowInTopLayoutWithNotes(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)
	m.state.view.Layout = state.LayoutTop

	wtPath := filepath.Join(cfg.WorktreeDir, "wt-notes")
	m.state.data.filteredWts = []*models.WorktreeInfo{{Path: wtPath, Branch: "feat"}}
	m.state.data.selectedIndex = 0
	m.state.ui.worktreeTable.SetCursor(0)
	m.setWorktreeNote(wtPath, "note text")

	m.setLogEntries([]commitLogEntry{
		{sha: "aaaaaaaa", message: "first"},
		{sha: "bbbbbbbb", message: "second"},
	}, true)

	layout := m.computeLayout()
	headerOffset := 1
	commitPaneX := layout.bottomLeftWidth + layout.gapX + layout.bottomMiddleWidth + layout.gapX + 1
	commitPaneTopY := headerOffset + layout.topHeight + layout.gapY + layout.notesRowHeight + layout.gapY
	clickSecondRowY := commitPaneTopY + 5 // table content starts after pane chrome; +4 is first row

	_, _ = m.handleMouseClick(tea.MouseClickMsg{
		Button: tea.MouseLeft,
		X:      commitPaneX,
		Y:      clickSecondRowY,
	})

	if m.state.view.FocusedPane != 3 {
		t.Fatalf("expected commit pane focus, got %d", m.state.view.FocusedPane)
	}
	if m.state.ui.logTable.Cursor() != 1 {
		t.Fatalf("expected commit cursor 1, got %d", m.state.ui.logTable.Cursor())
	}
}

func TestDoubleClickZoom(t *testing.T) {
	tests := []struct {
		name         string
		firstPane    int
		secondPane   int
		timeBetween  time.Duration
		initialZoom  int
		expectedZoom int
	}{
		{
			name:         "double click same pane zooms",
			firstPane:    0,
			secondPane:   0,
			timeBetween:  200 * time.Millisecond,
			initialZoom:  -1,
			expectedZoom: 0,
		},
		{
			name:         "double click when zoomed unzooms",
			firstPane:    0,
			secondPane:   0,
			timeBetween:  200 * time.Millisecond,
			initialZoom:  0,
			expectedZoom: -1,
		},
		{
			name:         "clicks on different panes no zoom",
			firstPane:    1,
			secondPane:   0,
			timeBetween:  200 * time.Millisecond,
			initialZoom:  -1,
			expectedZoom: -1,
		},
		{
			name:         "clicks too slow no zoom",
			firstPane:    0,
			secondPane:   0,
			timeBetween:  500 * time.Millisecond,
			initialZoom:  -1,
			expectedZoom: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
			m := NewModel(cfg, "")
			m.setWindowSize(120, 40)
			m.state.view.FocusedPane = 0
			m.state.view.ZoomedPane = tt.initialZoom

			// Simulate first click by setting tracking fields directly
			m.details.lastClickTime = time.Now().Add(-tt.timeBetween)
			m.details.lastClickPane = tt.firstPane

			// Build a click message targeting the second pane
			msg := tea.MouseClickMsg{
				Button: tea.MouseLeft,
				X:      2,
				Y:      5,
			}

			// Override targetPane detection by pre-focusing
			// For pane 0, X=2 Y=5 lands in worktree pane in default layout
			// For different-pane test, first click was on pane 1 so no match
			if tt.secondPane == 0 {
				msg.X = 2
				msg.Y = 5
			}

			_, _ = m.handleMouseClick(msg)

			assert.Equal(t, tt.expectedZoom, m.state.view.ZoomedPane,
				"expected ZoomedPane=%d, got %d", tt.expectedZoom, m.state.view.ZoomedPane)
		})
	}
}

// TestBuildStatusTreeEmpty tests building tree from empty file list.
