package screen

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chmouel/lazyworktree/internal/theme"
)

func TestCommandPaletteFilterToggle(t *testing.T) {
	items := []PaletteItem{
		{ID: "alpha", Label: "Alpha"},
		{ID: "beta", Label: "Beta"},
	}

	scr := NewCommandPaletteScreen(items, 80, 24, theme.Dracula())
	if !scr.FilterActive {
		t.Fatal("expected filter to be active by default")
	}

	// Type directly to filter (filter is already active)
	next, _ := scr.Update(tea.KeyPressMsg{Code: 'b', Text: string('b')})
	nextScr, ok := next.(*CommandPaletteScreen)
	if !ok || nextScr == nil {
		t.Fatal("expected Update to return command palette screen after typing")
	}
	scr = nextScr
	if len(scr.Filtered) != 1 || scr.Filtered[0].ID != "beta" {
		t.Fatalf("expected filtered results to include only 'beta', got %v", scr.Filtered)
	}

	// Esc exits filter mode but preserves filter text
	next, _ = scr.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	nextScr, ok = next.(*CommandPaletteScreen)
	if !ok || nextScr == nil {
		t.Fatal("expected Update to return command palette screen after Esc")
	}
	scr = nextScr
	if scr.FilterActive {
		t.Fatal("expected filter to be inactive after Esc")
	}
	if len(scr.Filtered) != 1 || scr.Filtered[0].ID != "beta" {
		t.Fatalf("expected filter to remain applied after Esc, got %v", scr.Filtered)
	}
}

func TestCommandPaletteViewWithIcons(t *testing.T) {
	items := []PaletteItem{
		{Label: "Worktree Actions", IsSection: true, Icon: ""},
		{ID: "create", Label: "Create worktree", Description: "Add a new worktree", Shortcut: "c", Icon: ""},
		{ID: "delete", Label: "Delete worktree", Description: "Remove worktree", Shortcut: "D", Icon: ""},
	}

	scr := NewCommandPaletteScreen(items, 100, 24, theme.Dracula())
	view := scr.View()

	// Verify the view contains expected elements
	assert.Contains(t, view, "Worktree Actions", "should contain section header")
	assert.Contains(t, view, "Create worktree", "should contain item label")
	assert.Contains(t, view, "of", "should contain item count")
}

func TestCommandPaletteViewWithMRU(t *testing.T) {
	items := []PaletteItem{
		{Label: "Recently Used", IsSection: true, Icon: ""},
		{ID: "recent-action", Label: "Recent Action", Description: "A recently used action", IsMRU: true, Icon: ""},
		{Label: "Git Operations", IsSection: true, Icon: ""},
		{ID: "refresh", Label: "Refresh", Description: "Reload worktrees", Shortcut: "r", Icon: ""},
	}

	scr := NewCommandPaletteScreen(items, 100, 24, theme.Dracula())
	view := scr.View()

	assert.Contains(t, view, "Recently Used", "should contain MRU section")
	assert.Contains(t, view, "Recent Action", "should contain MRU item")
}

func TestCommandPaletteFilterHidesEmptySections(t *testing.T) {
	items := []PaletteItem{
		{Label: "Worktree Actions", IsSection: true},
		{ID: "create", Label: "Create worktree", Description: "Add a new worktree"},
		{Label: "Status Pane", IsSection: true},
		{ID: "refresh-status", Label: "Refresh status", Description: "Reload status pane"},
		{Label: "Navigation", IsSection: true},
		{ID: "focus-worktrees", Label: "Focus worktrees", Description: "Focus worktree pane"},
	}

	scr := NewCommandPaletteScreen(items, 100, 24, theme.Dracula())
	next, _ := scr.Update(tea.KeyPressMsg{Code: 'w', Text: string('w')})
	nextScr, ok := next.(*CommandPaletteScreen)
	require.True(t, ok)
	require.NotNil(t, nextScr)

	require.Len(t, nextScr.Filtered, 4)
	assert.Equal(t, "Worktree Actions", nextScr.Filtered[0].Label)
	assert.Equal(t, "create", nextScr.Filtered[1].ID)
	assert.Equal(t, "Navigation", nextScr.Filtered[2].Label)
	assert.Equal(t, "focus-worktrees", nextScr.Filtered[3].ID)
}

func TestCommandPaletteHighlightMatches(t *testing.T) {
	items := []PaletteItem{
		{ID: "create", Label: "Create worktree", Description: "Add a new worktree", Icon: ""},
	}

	scr := NewCommandPaletteScreen(items, 100, 24, theme.Dracula())

	// Test highlight function directly
	result := scr.highlightMatches("Create worktree", "cre")
	require.NotEmpty(t, result, "highlighted result should not be empty")

	// With empty query, should return original text
	result = scr.highlightMatches("Create worktree", "")
	assert.Equal(t, "Create worktree", result, "empty query should return original text")
}

func TestCommandPaletteScrollIndicators(t *testing.T) {
	// Create enough items to require scrolling
	items := make([]PaletteItem, 20)
	for i := range items {
		items[i] = PaletteItem{
			ID:    "item-" + string(rune('a'+i)),
			Label: "Item " + string(rune('A'+i)),
		}
	}

	scr := NewCommandPaletteScreen(items, 100, 15, theme.Dracula())
	view := scr.View()

	// Should show scroll down indicator
	assert.True(t, strings.Contains(view, "▼") || strings.Contains(view, "↕"),
		"should show scroll indicator when items exceed visible area")
}

func TestCommandPaletteFooterFormat(t *testing.T) {
	items := []PaletteItem{
		{ID: "alpha", Label: "Alpha"},
		{ID: "beta", Label: "Beta"},
		{ID: "gamma", Label: "Gamma"},
	}

	scr := NewCommandPaletteScreen(items, 100, 24, theme.Dracula())
	view := scr.View()

	// Footer should contain item count and navigation hints
	assert.Contains(t, view, "1 of 3", "should show item count")
	assert.Contains(t, view, "navigate", "should contain navigation hint")
	assert.Contains(t, view, "Esc", "should contain escape hint")
}
