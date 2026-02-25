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
