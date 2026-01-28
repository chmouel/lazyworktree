package screen

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/chmouel/lazyworktree/internal/theme"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	if m.IsActive() {
		t.Error("expected new manager to have no active screen")
	}
	if m.Type() != TypeNone {
		t.Errorf("expected TypeNone, got %v", m.Type())
	}
}

func TestManagerPushPop(t *testing.T) {
	m := NewManager()
	thm := theme.Dracula()

	// Push a confirm screen
	confirm := NewConfirmScreen("test", thm)
	m.Push(confirm)

	if !m.IsActive() {
		t.Error("expected manager to be active after push")
	}
	if m.Type() != TypeConfirm {
		t.Errorf("expected TypeConfirm, got %v", m.Type())
	}
	if m.Current() != confirm {
		t.Error("expected current to be the pushed screen")
	}

	// Push an info screen
	info := NewInfoScreen("info", thm)
	m.Push(info)

	if m.Type() != TypeInfo {
		t.Errorf("expected TypeInfo, got %v", m.Type())
	}
	if m.StackDepth() != 1 {
		t.Errorf("expected stack depth 1, got %d", m.StackDepth())
	}

	// Pop the info screen
	popped := m.Pop()
	if popped != info {
		t.Error("expected to pop the info screen")
	}
	if m.Type() != TypeConfirm {
		t.Errorf("expected TypeConfirm after pop, got %v", m.Type())
	}

	// Pop the confirm screen
	popped = m.Pop()
	if popped != confirm {
		t.Error("expected to pop the confirm screen")
	}
	if m.IsActive() {
		t.Error("expected manager to be inactive after popping all screens")
	}
}

func TestManagerClear(t *testing.T) {
	m := NewManager()
	thm := theme.Dracula()

	m.Push(NewConfirmScreen("test", thm))
	m.Push(NewInfoScreen("info", thm))

	if !m.IsActive() {
		t.Error("expected manager to be active")
	}

	m.Clear()

	if m.IsActive() {
		t.Error("expected manager to be inactive after clear")
	}
	if m.StackDepth() != 0 {
		t.Errorf("expected stack depth 0, got %d", m.StackDepth())
	}
}

func TestManagerSet(t *testing.T) {
	m := NewManager()
	thm := theme.Dracula()

	confirm := NewConfirmScreen("test", thm)
	info := NewInfoScreen("info", thm)

	m.Push(confirm)
	m.Set(info)

	// Set should replace current without affecting stack
	if m.Current() != info {
		t.Error("expected current to be info after Set")
	}
	if m.StackDepth() != 0 {
		t.Errorf("expected stack depth 0 after Set, got %d", m.StackDepth())
	}
}

func TestConfirmScreenUpdate(t *testing.T) {
	thm := theme.Dracula()
	s := NewConfirmScreen("test", thm)

	// Test tab navigation
	updated, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	if updated.(*ConfirmScreen).SelectedButton != 1 {
		t.Error("expected button to move right")
	}

	// Test 'y' key
	s = NewConfirmScreen("test", thm)
	confirmCalled := false
	s.OnConfirm = func() tea.Cmd {
		confirmCalled = true
		return nil
	}
	updated, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if updated != nil {
		t.Error("expected nil screen after confirm")
	}
	if !confirmCalled {
		t.Error("expected OnConfirm to be called for 'y' key")
	}

	// Test 'n' key
	s = NewConfirmScreen("test", thm)
	cancelCalled := false
	s.OnCancel = func() tea.Cmd {
		cancelCalled = true
		return nil
	}
	updated, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if updated != nil {
		t.Error("expected nil screen after cancel")
	}
	if !cancelCalled {
		t.Error("expected OnCancel to be called for 'n' key")
	}
}

func TestInfoScreenUpdate(t *testing.T) {
	thm := theme.Dracula()
	s := NewInfoScreen("test", thm)

	// Test enter key
	closeCalled := false
	s.OnClose = func() tea.Cmd {
		closeCalled = true
		return nil
	}
	updated, _ := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if updated != nil {
		t.Error("expected nil screen after enter")
	}
	if !closeCalled {
		t.Error("expected OnClose to be called")
	}
}

func TestLoadingScreenTick(t *testing.T) {
	thm := theme.Dracula()
	s := NewLoadingScreen("Loading...", thm, nil)

	if s.FrameIdx != 0 {
		t.Errorf("expected initial frame index 0, got %d", s.FrameIdx)
	}

	s.Tick()
	if s.FrameIdx != 1 {
		t.Errorf("expected frame index 1 after tick, got %d", s.FrameIdx)
	}
}

func TestLoadingScreenDoesNotRespondToKeys(t *testing.T) {
	thm := theme.Dracula()
	s := NewLoadingScreen("Loading...", thm, nil)

	updated, _ := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if updated != s {
		t.Error("expected loading screen to return itself on key input")
	}
}

func TestTypeString(t *testing.T) {
	tests := []struct {
		t        Type
		expected string
	}{
		{TypeNone, "none"},
		{TypeConfirm, "confirm"},
		{TypeInfo, "info"},
		{TypeLoading, "loading"},
		{TypeHelp, "help"},
		{Type(999), "unknown"},
	}

	for _, tc := range tests {
		if got := tc.t.String(); got != tc.expected {
			t.Errorf("Type(%d).String() = %q, want %q", tc.t, got, tc.expected)
		}
	}
}
