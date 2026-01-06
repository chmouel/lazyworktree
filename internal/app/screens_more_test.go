package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/theme"
)

func TestHelpScreenSetSizeAndHighlight(t *testing.T) {
	thm := theme.Dracula()
	screen := NewHelpScreen(120, 40, nil, thm)
	screen.SetSize(160, 60)

	if screen.width <= 0 || screen.height <= 0 {
		t.Fatalf("unexpected screen size: %dx%d", screen.width, screen.height)
	}

	line := "Press g to go to top"
	style := lipgloss.NewStyle().Bold(true)
	highlighted := highlightMatches(line, strings.ToLower(line), "g", style)
	if !strings.Contains(highlighted, line) {
		t.Fatalf("expected highlighted output to include original line, got %q", highlighted)
	}
	if highlightMatches(line, strings.ToLower(line), "", style) != line {
		t.Fatal("expected empty query to return original line")
	}
}

func TestPRSelectionScreenUpdate(t *testing.T) {
	prs := []*models.PRInfo{
		{Number: 1, Title: "First"},
		{Number: 2, Title: "Second"},
	}
	screen := NewPRSelectionScreen(prs, 80, 30, theme.Dracula(), true)

	_, _ = screen.Update(tea.KeyMsg{Type: tea.KeyDown})
	if screen.cursor != 1 {
		t.Fatalf("expected cursor to move down, got %d", screen.cursor)
	}

	_, _ = screen.Update(tea.KeyMsg{Type: tea.KeyUp})
	if screen.cursor != 0 {
		t.Fatalf("expected cursor to move up, got %d", screen.cursor)
	}
}

func TestPRSelectionScreenViewIncludesIcon(t *testing.T) {
	prs := []*models.PRInfo{
		{Number: 1, Title: "First"},
	}
	screen := NewPRSelectionScreen(prs, 80, 30, theme.Dracula(), true)

	view := screen.View()
	if !strings.Contains(view, iconPR) {
		t.Fatalf("expected PR selection view to include icon %q, got %q", iconPR, view)
	}
}

func TestListSelectionScreenUpdate(t *testing.T) {
	items := []selectionItem{
		{id: "a", label: "Alpha"},
		{id: "b", label: "Beta"},
	}
	screen := NewListSelectionScreen(items, "Select", "", "", 100, 40, "", theme.Dracula())

	_, _ = screen.Update(tea.KeyMsg{Type: tea.KeyDown})
	if screen.cursor != 1 {
		t.Fatalf("expected cursor to move down, got %d", screen.cursor)
	}

	_, _ = screen.Update(tea.KeyMsg{Type: tea.KeyUp})
	if screen.cursor != 0 {
		t.Fatalf("expected cursor to move up, got %d", screen.cursor)
	}
}

func TestDiffScreenView(t *testing.T) {
	screen := NewDiffScreen("Diff Title", "diff content", theme.Dracula())
	view := screen.View()
	if !strings.Contains(view, "Diff Title") {
		t.Fatalf("expected diff view to include title, got %q", view)
	}
}
