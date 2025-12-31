package app

import (
	"bytes"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/chmouel/lazyworktree/internal/config"
)

// TestModelInitialization verifies the model initializes correctly
func TestModelInitialization(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	if m == nil {
		t.Fatal("NewModel returned nil")
	}

	if m.config != cfg {
		t.Error("Model config not set correctly")
	}

	if m.focusedPane != 0 {
		t.Errorf("Expected focusedPane to be 0, got %d", m.focusedPane)
	}

	if m.sortByActive != cfg.SortByActive {
		t.Error("sortByActive not initialized from config")
	}
}

// TestKeyboardNavigation tests basic keyboard navigation
func TestKeyboardNavigation(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	tm := teatest.NewTestModel(
		t,
		NewModel(cfg, ""),
		teatest.WithInitialTermSize(120, 40),
	)

	// Wait for initial load
	time.Sleep(100 * time.Millisecond)

	// Test tab navigation
	tm.Send(tea.KeyMsg{Type: tea.KeyTab})
	time.Sleep(50 * time.Millisecond)

	// Test number keys for pane focus
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})
	time.Sleep(50 * time.Millisecond)

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	time.Sleep(50 * time.Millisecond)

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})
	time.Sleep(50 * time.Millisecond)

	// Quit
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})

	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	// Get final model
	fm := tm.FinalModel(t)
	m, ok := fm.(*Model)
	if !ok {
		t.Fatal("Final model is not *Model type")
	}

	if !m.quitting {
		t.Error("Model should be marked as quitting after 'q' key")
	}
}

// TestFilterInput tests filter functionality
func TestFilterInput(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "test-filter")

	if !m.showingFilter {
		t.Error("Expected showingFilter to be true when initialized with filter")
	}

	if m.filterQuery != "test-filter" {
		t.Errorf("Expected filterQuery to be 'test-filter', got %q", m.filterQuery)
	}

	// Test filter toggle
	tm := teatest.NewTestModel(
		t,
		NewModel(cfg, ""),
		teatest.WithInitialTermSize(120, 40),
	)

	time.Sleep(100 * time.Millisecond)

	// Press 'f' to show filter
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	time.Sleep(50 * time.Millisecond)

	// Type some filter text
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	time.Sleep(50 * time.Millisecond)

	// Press enter to exit filter mode
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	time.Sleep(50 * time.Millisecond)

	// Quit
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})

	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}

// TestSortToggle tests the sort toggle functionality
func TestSortToggle(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir:  t.TempDir(),
		SortByActive: true,
	}
	tm := teatest.NewTestModel(
		t,
		NewModel(cfg, ""),
		teatest.WithInitialTermSize(120, 40),
	)

	time.Sleep(100 * time.Millisecond)

	// Press 's' to toggle sort
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	time.Sleep(50 * time.Millisecond)

	// Press 's' again to toggle back
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	time.Sleep(50 * time.Millisecond)

	// Quit
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})

	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	fm := tm.FinalModel(t)
	m, ok := fm.(*Model)
	if !ok {
		t.Fatal("Final model is not *Model type")
	}

	// Should be back to original state after two toggles
	if m.sortByActive != cfg.SortByActive {
		t.Errorf("Expected sortByActive to be %v after two toggles, got %v", cfg.SortByActive, m.sortByActive)
	}
}

// TestHelpScreen tests the help screen display
func TestHelpScreen(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	tm := teatest.NewTestModel(
		t,
		NewModel(cfg, ""),
		teatest.WithInitialTermSize(120, 40),
	)

	time.Sleep(100 * time.Millisecond)

	// Press '?' to show help
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	time.Sleep(50 * time.Millisecond)

	// Verify output contains help text
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("Help")) || bytes.Contains(bts, []byte("Worktree"))
		},
		teatest.WithCheckInterval(100*time.Millisecond),
		teatest.WithDuration(2*time.Second),
	)

	// Press 'q' to close help
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	time.Sleep(50 * time.Millisecond)

	// Quit the app
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})

	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	fm := tm.FinalModel(t)
	m, ok := fm.(*Model)
	if !ok {
		t.Fatal("Final model is not *Model type")
	}

	// Help screen should be closed
	if m.currentScreen == screenHelp {
		t.Error("Help screen should be closed after pressing 'q'")
	}
}

// TestWindowResize tests window resize handling
func TestWindowResize(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	// Send window size message
	msg := tea.WindowSizeMsg{
		Width:  100,
		Height: 30,
	}

	newModel, _ := m.Update(msg)
	updatedModel, ok := newModel.(*Model)
	if !ok {
		t.Fatal("Update returned wrong type")
	}

	if updatedModel.windowWidth != 100 {
		t.Errorf("Expected windowWidth to be 100, got %d", updatedModel.windowWidth)
	}

	if updatedModel.windowHeight != 30 {
		t.Errorf("Expected windowHeight to be 30, got %d", updatedModel.windowHeight)
	}
}

// TestCommandPalette tests command palette functionality
func TestCommandPalette(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	tm := teatest.NewTestModel(
		t,
		NewModel(cfg, ""),
		teatest.WithInitialTermSize(120, 40),
	)

	time.Sleep(100 * time.Millisecond)

	// Press 'ctrl+p' to show command palette
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlP})
	time.Sleep(50 * time.Millisecond)

	// Press escape to close palette
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	time.Sleep(50 * time.Millisecond)

	// Quit
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})

	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	fm := tm.FinalModel(t)
	m, ok := fm.(*Model)
	if !ok {
		t.Fatal("Final model is not *Model type")
	}

	if m.currentScreen == screenPalette {
		t.Error("Command palette should be closed after pressing escape")
	}
}

// TestViewRendering tests that the View method doesn't panic and produces output
func TestViewRendering(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	// Set window size first (View needs this)
	m.setWindowSize(120, 40)

	// Call View - should not panic
	view := m.View()
	if view == "" {
		t.Error("View() returned empty string")
	}

	if !bytes.Contains([]byte(view), []byte("Worktree")) {
		t.Error("View should contain 'Worktree' text")
	}
}

// TestArrowKeyNavigation tests arrow key navigation in different panes
func TestArrowKeyNavigation(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	tm := teatest.NewTestModel(
		t,
		NewModel(cfg, ""),
		teatest.WithInitialTermSize(120, 40),
	)

	time.Sleep(100 * time.Millisecond)

	// Test down/up arrows in worktree table
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	time.Sleep(50 * time.Millisecond)

	tm.Send(tea.KeyMsg{Type: tea.KeyUp})
	time.Sleep(50 * time.Millisecond)

	// Switch to pane 2 (viewport)
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	time.Sleep(50 * time.Millisecond)

	// Test scrolling in viewport
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	time.Sleep(50 * time.Millisecond)

	tm.Send(tea.KeyMsg{Type: tea.KeyUp})
	time.Sleep(50 * time.Millisecond)

	// Quit
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})

	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}

// TestMouseEvents tests that mouse events don't cause panics
func TestMouseEvents(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)

	// Test mouse wheel up
	mouseMsg := tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelUp,
		X:      10,
		Y:      5,
	}

	_, _ = m.Update(mouseMsg)

	// Test mouse wheel down
	mouseMsg = tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelDown,
		X:      10,
		Y:      5,
	}

	_, _ = m.Update(mouseMsg)

	// Test mouse click
	mouseMsg = tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      10,
		Y:      5,
	}

	_, _ = m.Update(mouseMsg)
}

// TestCleanup tests that the Close method properly cleans up resources
func TestCleanup(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	// Should not panic
	m.Close()

	// Should be safe to call multiple times
	m.Close()
}

// TestCommandPaletteCreateFromChanges tests that the create-from-changes command appears in the palette
func TestCommandPaletteCreateFromChanges(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	tm := teatest.NewTestModel(
		t,
		NewModel(cfg, ""),
		teatest.WithInitialTermSize(120, 40),
	)

	time.Sleep(100 * time.Millisecond)

	// Press 'ctrl+p' to show command palette
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlP})
	time.Sleep(50 * time.Millisecond)

	// Verify output contains the create-from-changes option
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("Create from changes")) ||
				bytes.Contains(bts, []byte("create-from-changes"))
		},
		teatest.WithCheckInterval(100*time.Millisecond),
		teatest.WithDuration(2*time.Second),
	)

	// Press escape to close palette
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	time.Sleep(50 * time.Millisecond)

	// Quit
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})

	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	fm := tm.FinalModel(t)
	m, ok := fm.(*Model)
	if !ok {
		t.Fatal("Final model is not *Model type")
	}

	if m.currentScreen == screenPalette {
		t.Error("Command palette should be closed after pressing escape")
	}
}
