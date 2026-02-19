package screen

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/chmouel/lazyworktree/internal/theme"
)

func TestNoteViewScreenType(t *testing.T) {
	s := NewNoteViewScreen("Notes", "line one", 120, 40, theme.Dracula())
	if s.Type() != TypeNoteView {
		t.Fatalf("expected TypeNoteView, got %v", s.Type())
	}
}

func TestNoteViewScreenEditClosesAndCallsCallback(t *testing.T) {
	s := NewNoteViewScreen("Notes", "line one", 120, 40, theme.Dracula())
	called := false
	s.OnEdit = func() tea.Cmd {
		called = true
		return nil
	}

	next, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if next != nil {
		t.Fatal("expected screen to close on edit")
	}
	if !called {
		t.Fatal("expected edit callback to be called")
	}
}

func TestNoteViewScreenScrollKeys(t *testing.T) {
	content := "line 1\nline 2\nline 3\nline 4\nline 5\nline 6\nline 7\nline 8\nline 9\nline 10\nline 11\nline 12\nline 13\nline 14"
	s := NewNoteViewScreen("Notes", content, 80, 18, theme.Dracula())

	start := s.Viewport.YOffset
	next, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	updated := next.(*NoteViewScreen)
	if updated.Viewport.YOffset <= start {
		t.Fatalf("expected y offset to increase after scroll down, start=%d now=%d", start, updated.Viewport.YOffset)
	}

	next, _ = updated.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	updated = next.(*NoteViewScreen)
	if updated.Viewport.YOffset != 0 {
		t.Fatalf("expected y offset to return to top after ctrl+u, got %d", updated.Viewport.YOffset)
	}
}

func TestNoteViewScreenEditExternalClosesAndCallsCallback(t *testing.T) {
	s := NewNoteViewScreen("Notes", "line one", 120, 40, theme.Dracula())
	called := false
	s.OnEditExternal = func() tea.Cmd {
		called = true
		return nil
	}

	next, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}})
	if next != nil {
		t.Fatal("expected screen to close on edit external")
	}
	if !called {
		t.Fatal("expected edit external callback to be called")
	}
}

func TestNoteViewScreenEditExternalNoCallbackIsNoop(t *testing.T) {
	s := NewNoteViewScreen("Notes", "line one", 120, 40, theme.Dracula())

	next, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}})
	if next != s {
		t.Fatal("expected screen to remain when no OnEditExternal callback is set")
	}
}

func TestNoteViewScreenWrapsLongLines(t *testing.T) {
	content := strings.Repeat("a", 240)
	s := NewNoteViewScreen("Notes", content, 90, 20, theme.Dracula())
	if !strings.Contains(s.Viewport.View(), "\n") {
		t.Fatalf("expected wrapped content in viewport view, got %q", s.Viewport.View())
	}
}
