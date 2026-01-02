package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/chmouel/lazyworktree/internal/theme"
)

func TestTrustScreenUpdateAndView(t *testing.T) {
	thm := theme.Dracula()
	screen := NewTrustScreen("/tmp/.wt.yaml", []string{"echo hi"}, thm)

	_, cmd := screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	if cmd == nil {
		t.Fatal("expected quit command for trust")
	}
	select {
	case result := <-screen.result:
		if result != "trust" {
			t.Fatalf("expected trust result, got %q", result)
		}
	default:
		t.Fatal("expected trust result to be sent")
	}

	view := screen.View()
	if !strings.Contains(view, "Trust") {
		t.Fatalf("expected trust screen view to include Trust label, got %q", view)
	}
}

func TestWelcomeScreenUpdateAndView(t *testing.T) {
	thm := theme.Dracula()
	screen := NewWelcomeScreen("/tmp", "/tmp/worktrees", thm)

	_, cmd := screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	if cmd == nil {
		t.Fatal("expected quit command for retry")
	}
	select {
	case result := <-screen.result:
		if !result {
			t.Fatal("expected retry result to be true")
		}
	default:
		t.Fatal("expected retry result to be sent")
	}

	view := screen.View()
	if !strings.Contains(view, "No worktrees found") {
		t.Fatalf("expected welcome view to include message, got %q", view)
	}
}

func TestCommitScreenUpdateAndView(t *testing.T) {
	thm := theme.Dracula()
	meta := commitMeta{
		sha:     "abc123",
		author:  "Test",
		email:   "test@example.com",
		date:    "Mon Jan 1 00:00:00 2024 +0000",
		subject: "Add feature",
	}
	screen := NewCommitScreen(meta, "stat", strings.Repeat("diff\n", 5), false, thm)

	if cmd := screen.Init(); cmd != nil {
		t.Fatal("expected nil init command")
	}

	_, cmd := screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if cmd != nil {
		t.Fatal("expected no command on scroll update")
	}

	view := screen.View()
	if !strings.Contains(view, "Commit:") || !strings.Contains(view, "abc123") {
		t.Fatalf("expected commit view to include metadata, got %q", view)
	}
}
