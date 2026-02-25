package screen

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/theme"
)

func TestIssueSelectionScreenFilterToggle(t *testing.T) {
	issues := []*models.IssueInfo{
		{Number: 1, Title: "First"},
		{Number: 2, Title: "Second"},
	}

	scr := NewIssueSelectionScreen(issues, 80, 30, theme.Dracula(), true)
	if scr.FilterActive {
		t.Fatal("expected filter to be inactive by default")
	}

	next, _ := scr.Update(tea.KeyPressMsg{Code: 'f', Text: string('f')})
	nextScr, ok := next.(*IssueSelectionScreen)
	if !ok || nextScr == nil {
		t.Fatal("expected Update to return issue selection screen after f")
	}
	scr = nextScr
	if !scr.FilterActive {
		t.Fatal("expected filter to be active after f")
	}

	next, _ = scr.Update(tea.KeyPressMsg{Code: '2', Text: string('2')})
	nextScr, ok = next.(*IssueSelectionScreen)
	if !ok || nextScr == nil {
		t.Fatal("expected Update to return issue selection screen after typing")
	}
	scr = nextScr
	if len(scr.Filtered) != 1 || scr.Filtered[0].Number != 2 {
		t.Fatalf("expected filtered results to include only #2, got %v", scr.Filtered)
	}

	next, _ = scr.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	nextScr, ok = next.(*IssueSelectionScreen)
	if !ok || nextScr == nil {
		t.Fatal("expected Update to return issue selection screen after Esc")
	}
	scr = nextScr
	if scr.FilterActive {
		t.Fatal("expected filter to be inactive after Esc")
	}
	if len(scr.Filtered) != 1 || scr.Filtered[0].Number != 2 {
		t.Fatalf("expected filter to remain applied after Esc, got %v", scr.Filtered)
	}
}
