package screen

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/chmouel/lazyworktree/internal/theme"
)

func TestListSelectionScreenJKNavigation(t *testing.T) {
	items := []SelectionItem{
		{ID: "one", Label: "One"},
		{ID: "two", Label: "Two"},
		{ID: "three", Label: "Three"},
	}

	scr := NewListSelectionScreen(items, "Test", "Filter...", "No results", 80, 30, "", theme.Dracula())

	if scr.Cursor != 0 {
		t.Fatalf("expected cursor to start at 0, got %d", scr.Cursor)
	}

	next, _ := scr.Update(tea.KeyPressMsg{Code: 'j', Text: string('j')})
	nextScr, ok := next.(*ListSelectionScreen)
	if !ok || nextScr == nil {
		t.Fatal("expected Update to return list selection screen after j")
	}
	scr = nextScr
	if scr.Cursor != 1 {
		t.Fatalf("expected cursor to move to 1 after j, got %d", scr.Cursor)
	}

	next, _ = scr.Update(tea.KeyPressMsg{Code: 'k', Text: string('k')})
	nextScr, ok = next.(*ListSelectionScreen)
	if !ok || nextScr == nil {
		t.Fatal("expected Update to return list selection screen after k")
	}
	scr = nextScr
	if scr.Cursor != 0 {
		t.Fatalf("expected cursor to move back to 0 after k, got %d", scr.Cursor)
	}
}

func TestListSelectionScreenFilterToggle(t *testing.T) {
	items := []SelectionItem{
		{ID: "one", Label: "One"},
		{ID: "two", Label: "Two"},
	}

	scr := NewListSelectionScreen(items, "Test", "Filter...", "No results", 80, 30, "", theme.Dracula())
	if scr.FilterActive {
		t.Fatal("expected filter to be inactive by default")
	}

	next, _ := scr.Update(tea.KeyPressMsg{Code: 'f', Text: string('f')})
	nextScr, ok := next.(*ListSelectionScreen)
	if !ok || nextScr == nil {
		t.Fatal("expected Update to return list selection screen after f")
	}
	scr = nextScr
	if !scr.FilterActive {
		t.Fatal("expected filter to be active after f")
	}

	next, _ = scr.Update(tea.KeyPressMsg{Code: 't', Text: string('t')})
	nextScr, ok = next.(*ListSelectionScreen)
	if !ok || nextScr == nil {
		t.Fatal("expected Update to return list selection screen after typing")
	}
	scr = nextScr
	if len(scr.Filtered) != 1 || scr.Filtered[0].ID != "two" {
		t.Fatalf("expected filtered results to include only 'two', got %v", scr.Filtered)
	}

	next, _ = scr.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	nextScr, ok = next.(*ListSelectionScreen)
	if !ok || nextScr == nil {
		t.Fatal("expected Update to return list selection screen after Esc")
	}
	scr = nextScr
	if scr.FilterActive {
		t.Fatal("expected filter to be inactive after Esc")
	}
	if len(scr.Filtered) != 1 || scr.Filtered[0].ID != "two" {
		t.Fatalf("expected filter to remain applied after Esc, got %v", scr.Filtered)
	}
}
