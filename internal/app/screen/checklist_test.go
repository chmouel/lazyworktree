package screen

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/chmouel/lazyworktree/internal/theme"
)

func TestChecklistScreenFilterToggle(t *testing.T) {
	items := []ChecklistItem{
		{ID: "one", Label: "One"},
		{ID: "two", Label: "Two"},
	}

	scr := NewChecklistScreen(items, "Test", "Filter...", "No items", 80, 30, theme.Dracula())
	if scr.FilterActive {
		t.Fatal("expected filter to be inactive by default")
	}

	next, _ := scr.Update(tea.KeyPressMsg{Code: 'f', Text: string('f')})
	nextScr, ok := next.(*ChecklistScreen)
	if !ok || nextScr == nil {
		t.Fatal("expected Update to return checklist screen after f")
	}
	scr = nextScr
	if !scr.FilterActive {
		t.Fatal("expected filter to be active after f")
	}

	next, _ = scr.Update(tea.KeyPressMsg{Code: 't', Text: string('t')})
	nextScr, ok = next.(*ChecklistScreen)
	if !ok || nextScr == nil {
		t.Fatal("expected Update to return checklist screen after typing")
	}
	scr = nextScr
	if len(scr.Filtered) != 1 || scr.Filtered[0].ID != "two" {
		t.Fatalf("expected filtered results to include only 'two', got %v", scr.Filtered)
	}

	next, _ = scr.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	nextScr, ok = next.(*ChecklistScreen)
	if !ok || nextScr == nil {
		t.Fatal("expected Update to return checklist screen after Esc")
	}
	scr = nextScr
	if scr.FilterActive {
		t.Fatal("expected filter to be inactive after Esc")
	}
	if len(scr.Filtered) != 1 || scr.Filtered[0].ID != "two" {
		t.Fatalf("expected filter to remain applied after Esc, got %v", scr.Filtered)
	}
}
