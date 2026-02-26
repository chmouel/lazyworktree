package screen

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/chmouel/lazyworktree/internal/theme"
)

func TestTextareaScreenType(t *testing.T) {
	s := NewTextareaScreen("Prompt", "Placeholder", "", 120, 40, theme.Dracula(), false)
	if s.Type() != TypeTextarea {
		t.Fatalf("expected TypeTextarea, got %v", s.Type())
	}
}

func TestTextareaScreenCtrlSSubmit(t *testing.T) {
	s := NewTextareaScreen("Prompt", "Placeholder", "hello", 120, 40, theme.Dracula(), false)
	called := false
	var gotValue string
	s.OnSubmit = func(value string) tea.Cmd {
		called = true
		gotValue = value
		return nil
	}

	next, _ := s.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if next != nil {
		t.Fatal("expected screen to close on Ctrl+S")
	}
	if !called {
		t.Fatal("expected submit callback to be called")
	}
	if gotValue != "hello" {
		t.Fatalf("expected value %q, got %q", "hello", gotValue)
	}
}

func TestTextareaScreenCtrlXEditExternal(t *testing.T) {
	s := NewTextareaScreen("Prompt", "Placeholder", "some notes", 120, 40, theme.Dracula(), false)
	called := false
	var gotValue string
	s.OnEditExternal = func(currentValue string) tea.Cmd {
		called = true
		gotValue = currentValue
		return nil
	}

	next, _ := s.Update(tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl})
	if next != nil {
		t.Fatal("expected screen to close on Ctrl+X")
	}
	if !called {
		t.Fatal("expected OnEditExternal callback to be called")
	}
	if gotValue != "some notes" {
		t.Fatalf("expected value %q, got %q", "some notes", gotValue)
	}
}

func TestTextareaScreenCtrlXNoCallback(t *testing.T) {
	s := NewTextareaScreen("Prompt", "Placeholder", "hello", 120, 40, theme.Dracula(), false)
	// OnEditExternal is nil — Ctrl+X should be a no-op (screen stays open)
	next, _ := s.Update(tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl})
	if next == nil {
		t.Fatal("expected screen to stay open when OnEditExternal is nil")
	}
}

func TestTextareaScreenFooterShowsEditorWhenCallback(t *testing.T) {
	s := NewTextareaScreen("Prompt", "Placeholder", "", 120, 40, theme.Dracula(), false)

	// Without callback, footer should not mention Ctrl+X
	view := s.View()
	if contains(view, "Ctrl+X") {
		t.Fatal("footer should not mention Ctrl+X when OnEditExternal is nil")
	}

	// With callback, footer should mention Ctrl+X
	s.OnEditExternal = func(string) tea.Cmd { return nil }
	view = s.View()
	if !contains(view, "Ctrl+X") {
		t.Fatal("footer should mention Ctrl+X when OnEditExternal is set")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestTextareaScreenEnterAddsNewLine(t *testing.T) {
	s := NewTextareaScreen("Prompt", "Placeholder", "hello", 120, 40, theme.Dracula(), false)

	next, _ := s.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if next == nil {
		t.Fatal("expected screen to stay open on Enter")
	}

	updated := next.(*TextareaScreen)
	if updated.Input.Value() != "hello\n" {
		t.Fatalf("expected newline to be inserted, got %q", updated.Input.Value())
	}
}
