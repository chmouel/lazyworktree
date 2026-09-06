package screen

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/chmouel/lazyworktree/internal/theme"
)

func TestCommitMessageScreenType(t *testing.T) {
	s := NewCommitMessageScreen("Commit", "Body", "", 120, 40, theme.Dracula(), false, false)
	if s.Type() != TypeCommitMessage {
		t.Fatalf("expected TypeCommitMessage, got %v", s.Type())
	}
}

func TestCommitMessageScreenUsesLargerModalSizing(t *testing.T) {
	s := NewCommitMessageScreen("Commit", "Body", "", 120, 40, theme.Dracula(), false, false)
	if s.boxWidth != 93 {
		t.Fatalf("expected wider modal, got width %d", s.boxWidth)
	}
	if s.boxHeight != 28 {
		t.Fatalf("expected taller modal, got height %d", s.boxHeight)
	}
}

func TestCommitMessageScreenCtrlSSubmitsCombinedValue(t *testing.T) {
	s := NewCommitMessageScreen("Commit", "Body", "", 120, 40, theme.Dracula(), false, false)
	s.SubjectInput.SetValue("subject line")
	s.BodyInput.SetValue("body line")

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
	if gotValue != "subject line\n\nbody line" {
		t.Fatalf("expected combined value, got %q", gotValue)
	}
}

func TestCommitMessageScreenRequiresSubject(t *testing.T) {
	s := NewCommitMessageScreen("Commit", "Body", "", 120, 40, theme.Dracula(), false, false)
	s.BodyInput.SetValue("body only")

	next, _ := s.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if next == nil {
		t.Fatal("expected screen to stay open when subject is empty")
	}
	if s.ErrorMsg == "" {
		t.Fatal("expected validation error for empty subject")
	}
}

func TestCommitMessageScreenEnterMovesFocusToBody(t *testing.T) {
	s := NewCommitMessageScreen("Commit", "Body", "", 120, 40, theme.Dracula(), false, false)

	next, _ := s.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if next == nil {
		t.Fatal("expected screen to stay open on Enter in subject")
	}
	updated := next.(*CommitMessageScreen)
	if updated.focus != commitMessageFocusBody {
		t.Fatalf("expected focus to move to body, got %v", updated.focus)
	}
}

func TestCommitMessageScreenPasteGoesToSubjectByDefault(t *testing.T) {
	s := NewCommitMessageScreen("Commit", "Body", "", 120, 40, theme.Dracula(), false, false)

	next, _ := s.Update(tea.PasteMsg{Content: "pasted subject"})
	updated, ok := next.(*CommitMessageScreen)
	if !ok || updated == nil {
		t.Fatal("expected commit message screen after paste")
	}
	if got := updated.SubjectInput.Value(); got != "pasted subject" {
		t.Fatalf("expected pasted content in subject, got %q", got)
	}
	if updated.BodyInput.Value() != "" {
		t.Fatalf("expected body to stay empty, got %q", updated.BodyInput.Value())
	}
}

func TestCommitMessageScreenPasteGoesToBodyAfterFocusMoves(t *testing.T) {
	s := NewCommitMessageScreen("Commit", "Body", "", 120, 40, theme.Dracula(), false, false)

	next, _ := s.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := next.(*CommitMessageScreen)

	next, _ = updated.Update(tea.PasteMsg{Content: "pasted body"})
	updated, ok := next.(*CommitMessageScreen)
	if !ok || updated == nil {
		t.Fatal("expected commit message screen after paste")
	}
	if got := updated.BodyInput.Value(); got != "pasted body" {
		t.Fatalf("expected pasted content in body, got %q", got)
	}
	if updated.SubjectInput.Value() != "" {
		t.Fatalf("expected subject to stay empty, got %q", updated.SubjectInput.Value())
	}
}

func TestCommitMessageScreenCtrlXEditExternal(t *testing.T) {
	s := NewCommitMessageScreen("Commit", "Body", "subject\n\nbody", 120, 40, theme.Dracula(), false, false)
	called := false
	var gotValue string
	s.OnEditExternal = func(currentValue string) tea.Cmd {
		called = true
		gotValue = currentValue
		return nil
	}

	next, _ := s.Update(tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl})
	if next == nil {
		t.Fatal("expected commit screen to stay open on Ctrl+X")
	}
	if !called {
		t.Fatal("expected external editor callback to be called")
	}
	if gotValue != "subject\n\nbody" {
		t.Fatalf("expected combined draft value, got %q", gotValue)
	}
}

func TestCommitMessageScreenSetValueSplitsSubjectAndBody(t *testing.T) {
	s := NewCommitMessageScreen("Commit", "Body", "", 120, 40, theme.Dracula(), false, false)
	s.SetValue("subject line\n\nbody line")

	if s.SubjectInput.Value() != "subject line" {
		t.Fatalf("expected subject to be set, got %q", s.SubjectInput.Value())
	}
	if s.BodyInput.Value() != "body line" {
		t.Fatalf("expected body to be set, got %q", s.BodyInput.Value())
	}
}

func TestCommitMessageScreenSetValuePreservesBodyWithoutBlankSeparator(t *testing.T) {
	s := NewCommitMessageScreen("Commit", "Body", "", 120, 40, theme.Dracula(), false, false)
	s.SetValue("subject line\nbody line\nextra detail")

	if s.SubjectInput.Value() != "subject line" {
		t.Fatalf("expected subject to be set, got %q", s.SubjectInput.Value())
	}
	if s.BodyInput.Value() != "body line\nextra detail" {
		t.Fatalf("expected remaining text to stay in the body, got %q", s.BodyInput.Value())
	}
}

func TestCommitMessageScreenSetGeneratedValueUsesThirdLineAsBodyStart(t *testing.T) {
	s := NewCommitMessageScreen("Commit", "Body", "", 120, 40, theme.Dracula(), false, false)
	s.SetGeneratedValue("subject line\n\nbody line\nextra detail")

	if s.SubjectInput.Value() != "subject line" {
		t.Fatalf("expected subject to be set, got %q", s.SubjectInput.Value())
	}
	if s.BodyInput.Value() != "body line\nextra detail" {
		t.Fatalf("expected body to start from third line, got %q", s.BodyInput.Value())
	}
}

func TestCommitMessageScreenSetGeneratedValueDropsBodyWithoutThirdLine(t *testing.T) {
	s := NewCommitMessageScreen("Commit", "Body", "", 120, 40, theme.Dracula(), false, false)
	s.SetGeneratedValue("subject line\nbody line only")

	if s.SubjectInput.Value() != "subject line" {
		t.Fatalf("expected subject to be set, got %q", s.SubjectInput.Value())
	}
	if s.BodyInput.Value() != "" {
		t.Fatalf("expected empty body when generator output has no third line, got %q", s.BodyInput.Value())
	}
}

func TestCommitMessageScreenCtrlOAutoGenerate(t *testing.T) {
	s := NewCommitMessageScreen("Commit", "Body", "", 120, 40, theme.Dracula(), false, true)
	called := false
	s.OnAutoGenerate = func() tea.Cmd {
		called = true
		return nil
	}

	next, _ := s.Update(tea.KeyPressMsg{Code: 'o', Mod: tea.ModCtrl})
	if next == nil {
		t.Fatal("expected commit screen to stay open on Ctrl+O")
	}
	if !called {
		t.Fatal("expected auto-generate callback to be called")
	}
}

func TestCommitMessageScreenFooterShowsOptionalShortcuts(t *testing.T) {
	s := NewCommitMessageScreen("Commit", "Body", "", 120, 40, theme.Dracula(), false, true)

	view := s.View()
	if contains(view, "Ctrl+X") {
		t.Fatal("footer should not mention Ctrl+X without external editor callback")
	}
	if !contains(view, "Ctrl+O") {
		t.Fatal("footer should mention Ctrl+O when auto-generate is enabled")
	}

	s.OnEditExternal = func(string) tea.Cmd { return nil }
	view = s.View()
	if !contains(view, "Ctrl+X") {
		t.Fatal("footer should mention Ctrl+X when external editor callback is set")
	}
}

func TestCommitMessageScreenSubjectTooLongThreshold(t *testing.T) {
	s := NewCommitMessageScreen("Commit", "Body", "", 120, 40, theme.Dracula(), false, false)
	s.SubjectInput.SetValue(strings.Repeat("a", commitSubjectSoftLimit))
	if s.subjectTooLong() {
		t.Fatal("subject should not be too long at the limit")
	}

	s.SubjectInput.SetValue(strings.Repeat("a", commitSubjectSoftLimit+1))
	if !s.subjectTooLong() {
		t.Fatal("subject should be too long past the limit")
	}
}

func TestCommitMessageScreenViewTurnsSubjectRedPastLimit(t *testing.T) {
	s := NewCommitMessageScreen("Commit", "Body", "", 120, 40, theme.Dracula(), false, false)
	s.SubjectInput.SetValue(strings.Repeat("a", commitSubjectSoftLimit+1))

	view := s.View()
	expectedColour := lipgloss.NewStyle().Foreground(theme.Dracula().ErrorFg).Render(strings.Repeat("a", commitSubjectSoftLimit+1))
	if !contains(view, expectedColour) {
		t.Fatal("expected subject line to be rendered with error colour when over limit")
	}
	if !contains(view, "51/50") {
		t.Fatal("expected subject character count to be shown")
	}
}
