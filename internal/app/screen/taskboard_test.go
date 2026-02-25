package screen

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/chmouel/lazyworktree/internal/theme"
)

func testTaskboardItems() []TaskboardItem {
	return []TaskboardItem{
		{
			IsSection:    true,
			WorktreePath: "/tmp/wt-a",
			SectionLabel: "wt-a",
			OpenCount:    1,
			DoneCount:    0,
			TotalCount:   1,
		},
		{
			ID:           "task-a-0",
			WorktreePath: "/tmp/wt-a",
			WorktreeName: "wt-a",
			Text:         "Write docs",
			Checked:      false,
		},
		{
			IsSection:    true,
			WorktreePath: "/tmp/wt-b",
			SectionLabel: "wt-b",
			OpenCount:    0,
			DoneCount:    1,
			TotalCount:   1,
		},
		{
			ID:           "task-b-1",
			WorktreePath: "/tmp/wt-b",
			WorktreeName: "wt-b",
			Text:         "Fix parser",
			Checked:      true,
		},
	}
}

func TestTaskboardScreenType(t *testing.T) {
	s := NewTaskboardScreen(testTaskboardItems(), "Taskboard", 120, 40, theme.Dracula())
	if s.Type() != TypeTaskboard {
		t.Fatalf("expected TypeTaskboard, got %v", s.Type())
	}
}

func TestTaskboardScreenNavigationSkipsSections(t *testing.T) {
	s := NewTaskboardScreen(testTaskboardItems(), "Taskboard", 120, 40, theme.Dracula())
	if s.Cursor != 1 {
		t.Fatalf("expected initial cursor at first task index 1, got %d", s.Cursor)
	}

	next, _ := s.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	updated, ok := next.(*TaskboardScreen)
	if !ok {
		t.Fatal("expected taskboard screen")
	}
	if updated.Cursor != 3 {
		t.Fatalf("expected cursor to skip section and land at 3, got %d", updated.Cursor)
	}

	next, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	updated, ok = next.(*TaskboardScreen)
	if !ok {
		t.Fatal("expected taskboard screen")
	}
	if updated.Cursor != 1 {
		t.Fatalf("expected cursor to move back to 1, got %d", updated.Cursor)
	}
}

func TestTaskboardScreenFilter(t *testing.T) {
	s := NewTaskboardScreen(testTaskboardItems(), "Taskboard", 120, 40, theme.Dracula())

	next, _ := s.Update(tea.KeyPressMsg{Code: 'f', Text: string('f')})
	updated := next.(*TaskboardScreen)
	if !updated.FilterActive {
		t.Fatal("expected filter mode to be active")
	}

	next, _ = updated.Update(tea.KeyPressMsg{Code: 'f', Text: string('f')})
	updated = next.(*TaskboardScreen)
	next, _ = updated.Update(tea.KeyPressMsg{Code: 'i', Text: string('i')})
	updated = next.(*TaskboardScreen)
	next, _ = updated.Update(tea.KeyPressMsg{Code: 'x', Text: string('x')})
	updated = next.(*TaskboardScreen)

	if len(updated.Filtered) != 2 {
		t.Fatalf("expected one section + one task after filter, got %d items", len(updated.Filtered))
	}
	if updated.Filtered[1].ID != "task-b-1" {
		t.Fatalf("expected filtered task task-b-1, got %q", updated.Filtered[1].ID)
	}
}

func TestTaskboardScreenToggleCallbackAndState(t *testing.T) {
	s := NewTaskboardScreen(testTaskboardItems(), "Taskboard", 120, 40, theme.Dracula())
	called := false
	var calledID string
	s.OnToggle = func(itemID string) tea.Cmd {
		called = true
		calledID = itemID
		return nil
	}

	next, _ := s.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	updated := next.(*TaskboardScreen)

	if !called {
		t.Fatal("expected toggle callback to be called")
	}
	if calledID != "task-a-0" {
		t.Fatalf("expected callback id task-a-0, got %q", calledID)
	}
	if !updated.Items[1].Checked {
		t.Fatal("expected selected task to be toggled to checked")
	}
	if updated.Items[0].OpenCount != 0 || updated.Items[0].DoneCount != 1 || updated.Items[0].TotalCount != 1 {
		t.Fatalf("expected section counts to update, got open=%d done=%d total=%d",
			updated.Items[0].OpenCount, updated.Items[0].DoneCount, updated.Items[0].TotalCount)
	}
}

func TestTaskboardScreenNoResultsView(t *testing.T) {
	s := NewTaskboardScreen(testTaskboardItems(), "Taskboard", 120, 40, theme.Dracula())
	s.FilterActive = true
	s.FilterInput.SetValue("definitely-not-present")
	s.applyFilter()
	view := s.View()
	if !strings.Contains(view, "No matching tasks.") {
		t.Fatalf("expected empty-state message in view, got %q", view)
	}
}

func TestTaskboardScreenAddKeyTriggersOnAdd(t *testing.T) {
	s := NewTaskboardScreen(testTaskboardItems(), "Taskboard", 120, 40, theme.Dracula())
	called := false
	var calledPath string
	s.OnAdd = func(worktreePath string) tea.Cmd {
		called = true
		calledPath = worktreePath
		return nil
	}

	// Cursor starts at index 1 (first task in wt-a)
	next, _ := s.Update(tea.KeyPressMsg{Code: 'a', Text: string('a')})
	if next == nil {
		t.Fatal("expected screen to remain active")
	}
	if !called {
		t.Fatal("expected OnAdd callback to be called")
	}
	if calledPath != "/tmp/wt-a" {
		t.Fatalf("expected worktree path /tmp/wt-a, got %q", calledPath)
	}
}

func TestTaskboardScreenAddKeyNoopWithoutCallback(t *testing.T) {
	s := NewTaskboardScreen(testTaskboardItems(), "Taskboard", 120, 40, theme.Dracula())
	next, cmd := s.Update(tea.KeyPressMsg{Code: 'a', Text: string('a')})
	if next == nil {
		t.Fatal("expected screen to remain active")
	}
	if cmd != nil {
		t.Fatal("expected nil command when OnAdd is nil")
	}
}

func TestTaskboardScreenEmptyListUsesDefaultWorktreePath(t *testing.T) {
	s := NewTaskboardScreen(nil, "Taskboard", 120, 40, theme.Dracula())
	s.DefaultWorktreePath = "/tmp/default-wt"
	called := false
	var calledPath string
	s.OnAdd = func(worktreePath string) tea.Cmd {
		called = true
		calledPath = worktreePath
		return nil
	}

	s.Update(tea.KeyPressMsg{Code: 'a', Text: string('a')})
	if !called {
		t.Fatal("expected OnAdd callback to be called on empty list")
	}
	if calledPath != "/tmp/default-wt" {
		t.Fatalf("expected default worktree path /tmp/default-wt, got %q", calledPath)
	}
}

func TestTaskboardScreenAddFooterHelpText(t *testing.T) {
	s := NewTaskboardScreen(testTaskboardItems(), "Taskboard", 120, 40, theme.Dracula())
	view := s.View()
	if !strings.Contains(view, "a add") {
		t.Fatal("expected footer to contain 'a add'")
	}
}
