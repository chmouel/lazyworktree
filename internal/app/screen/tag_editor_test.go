package screen

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/chmouel/lazyworktree/internal/theme"
)

func TestTagEditorScreenTogglesExistingTagIntoInput(t *testing.T) {
	scr := NewTagEditorScreen(
		"Set worktree tags",
		[]string{"bug"},
		[]TagEditorOption{
			{Tag: "bug", Count: 2},
			{Tag: "frontend", Count: 1},
		},
		120,
		40,
		theme.Dracula(),
		false,
	)

	next, _ := scr.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	tagScr, ok := next.(*TagEditorScreen)
	if !ok || tagScr == nil {
		t.Fatal("expected tag editor screen after Tab")
	}
	scr = tagScr

	next, _ = scr.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	tagScr, ok = next.(*TagEditorScreen)
	if !ok || tagScr == nil {
		t.Fatal("expected tag editor screen after Down")
	}
	scr = tagScr

	next, _ = scr.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	tagScr, ok = next.(*TagEditorScreen)
	if !ok || tagScr == nil {
		t.Fatal("expected tag editor screen after Space")
	}
	scr = tagScr

	if got := scr.Input.Value(); got != "bug, frontend" {
		t.Fatalf("expected toggled tags in input, got %q", got)
	}
}

func TestTagEditorScreenEnterSubmitsCurrentTags(t *testing.T) {
	scr := NewTagEditorScreen(
		"Set worktree tags",
		[]string{"bug"},
		nil,
		120,
		40,
		theme.Dracula(),
		false,
	)

	var submitted []string
	scr.OnSubmit = func(tags []string) tea.Cmd {
		submitted = append([]string(nil), tags...)
		return nil
	}

	scr.Input.SetValue("bug, frontend, urgent")
	next, _ := scr.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if next != nil {
		t.Fatal("expected tag editor screen to close on submit")
	}
	if got := strings.Join(submitted, ","); got != "bug,frontend,urgent" {
		t.Fatalf("unexpected submitted tags: %q", got)
	}
}

func TestTagEditorScreenPasteGoesToFocusedInput(t *testing.T) {
	scr := NewTagEditorScreen(
		"Set worktree tags",
		nil,
		[]TagEditorOption{{Tag: "bug", Count: 2}},
		120,
		40,
		theme.Dracula(),
		false,
	)

	// Input is focused by default.
	next, _ := scr.Update(tea.PasteMsg{Content: "pasted-tag"})
	tagScr, ok := next.(*TagEditorScreen)
	if !ok || tagScr == nil {
		t.Fatal("expected tag editor screen after paste")
	}
	if got := tagScr.Input.Value(); got != "pasted-tag" {
		t.Fatalf("expected pasted content in input, got %q", got)
	}
}

func TestTagEditorScreenPasteIgnoredWhenListFocused(t *testing.T) {
	scr := NewTagEditorScreen(
		"Set worktree tags",
		nil,
		[]TagEditorOption{{Tag: "bug", Count: 2}},
		120,
		40,
		theme.Dracula(),
		false,
	)

	next, _ := scr.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	tagScr, ok := next.(*TagEditorScreen)
	if !ok || tagScr == nil {
		t.Fatal("expected tag editor screen after Tab")
	}

	next, _ = tagScr.Update(tea.PasteMsg{Content: "pasted-tag"})
	tagScr, ok = next.(*TagEditorScreen)
	if !ok || tagScr == nil {
		t.Fatal("expected tag editor screen after paste")
	}
	if got := tagScr.Input.Value(); got != "" {
		t.Fatalf("expected paste to be ignored while list is focused, got %q", got)
	}
}
