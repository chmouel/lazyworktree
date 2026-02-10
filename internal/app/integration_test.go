package app

import (
	"bytes"
	"os"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

// TestModelInitialization verifies the model initializes correctly
func TestModelInitialization(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	if m == nil {
		t.Fatal("NewModel returned nil")
	} else if m.config != cfg {
		t.Error("Model config not set correctly")
	}

	if m.state.view.FocusedPane != 0 {
		t.Errorf("Expected focusedPane to be 0, got %d", m.state.view.FocusedPane)
	}

	// sortMode is now an int: 0=path, 1=active, 2=switched
	// Default is switched (2) when config.SortMode is "switched"
	expectedSortMode := sortModeLastSwitched
	switch cfg.SortMode {
	case "path":
		expectedSortMode = sortModePath
	case "active":
		expectedSortMode = sortModeLastActive
	}
	if m.sortMode != expectedSortMode {
		t.Errorf("sortMode not initialized from config: got %d, expected %d", m.sortMode, expectedSortMode)
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

	if !m.state.view.ShowingFilter {
		t.Error("Expected showingFilter to be true when initialized with filter")
	}

	if m.state.services.filter.FilterQuery != "test-filter" {
		t.Errorf("Expected filterQuery to be 'test-filter', got %q", m.state.services.filter.FilterQuery)
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

func TestSearchAutoSelectStartsFocused(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir:      t.TempDir(),
		SearchAutoSelect: true,
	}
	m := NewModel(cfg, "")

	if !m.state.view.ShowingFilter {
		t.Error("Expected showingFilter to be true when search auto-select is enabled")
	}
	if !m.state.ui.filterInput.Focused() {
		t.Error("Expected filter input to be focused when search auto-select is enabled")
	}
}

// TestSortCycle tests the sort cycle functionality
func TestSortCycle(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		SortMode:    "switched",
	}
	tm := teatest.NewTestModel(
		t,
		NewModel(cfg, ""),
		teatest.WithInitialTermSize(120, 40),
	)

	time.Sleep(100 * time.Millisecond)

	// Press 's' three times to cycle through all modes and back to original
	// switched (2) -> path (0) -> active (1) -> switched (2)
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	time.Sleep(50 * time.Millisecond)

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	time.Sleep(50 * time.Millisecond)

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

	// Should be back to original state after three cycles
	if m.sortMode != sortModeLastSwitched {
		t.Errorf("Expected sortMode to be %d after three cycles, got %d", sortModeLastSwitched, m.sortMode)
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

	// Help screen should be closed (screen manager should be inactive)
	if m.state.ui.screenManager.IsActive() && m.state.ui.screenManager.Type() == appscreen.TypeHelp {
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

	if updatedModel.state.view.WindowWidth != 100 {
		t.Errorf("Expected windowWidth to be 100, got %d", updatedModel.state.view.WindowWidth)
	}

	if updatedModel.state.view.WindowHeight != 30 {
		t.Errorf("Expected windowHeight to be 30, got %d", updatedModel.state.view.WindowHeight)
	}
}

// TestVerySmallTerminalSize tests handling of very small terminal sizes
func TestVerySmallTerminalSize(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	// Send a very small window size message that previously caused a panic
	msg := tea.WindowSizeMsg{
		Width:  10,
		Height: 5,
	}

	// This should not panic
	newModel, _ := m.Update(msg)
	updatedModel, ok := newModel.(*Model)
	if !ok {
		t.Fatal("Update returned wrong type")
	}

	// Verify the model was updated
	if updatedModel.state.view.WindowWidth != 10 {
		t.Errorf("Expected windowWidth to be 10, got %d", updatedModel.state.view.WindowWidth)
	}

	if updatedModel.state.view.WindowHeight != 5 {
		t.Errorf("Expected windowHeight to be 5, got %d", updatedModel.state.view.WindowHeight)
	}

	// Try to render the view - this previously caused slice bounds panic
	view := updatedModel.View()
	if view == "" {
		t.Error("Expected View() to return non-empty string even with tiny window")
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

	// First Esc exits filter mode (filter is active by default)
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	time.Sleep(50 * time.Millisecond)

	// Second Esc closes the palette
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

	if m.state.ui.screenManager.IsActive() && m.state.ui.screenManager.Type() == appscreen.TypePalette {
		t.Error("Command palette should be closed after pressing escape twice")
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

// TestCommitScreenEscapeKey tests that ESC key closes the commit screen
func TestCommitScreenEscapeKey(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	// Set up the commit screen via screen manager
	commitScr := appscreen.NewCommitScreen(appscreen.CommitMeta{SHA: "abc123"}, "stat", "diff", false, m.theme)
	m.state.ui.screenManager.Push(commitScr)

	// Simulate pressing ESC
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}

	// Log what we're testing
	t.Logf("ESC key string: %q", escMsg.String())
	t.Logf("keyEsc constant: %q", keyEsc)

	// Check if the key matches
	if escMsg.String() != keyEsc {
		t.Errorf("ESC key string %q doesn't match keyEsc %q", escMsg.String(), keyEsc)
	}

	// Call handleScreenKey
	newModel, _ := m.handleScreenKey(escMsg)
	updatedModel := newModel.(*Model)

	// Verify the commit screen was closed via screen manager
	if updatedModel.state.ui.screenManager.IsActive() {
		t.Errorf("Expected screen manager to be inactive, got type %v", updatedModel.state.ui.screenManager.Type())
	}
}

// TestCommitScreenRawEscapeKey tests that raw ESC byte (0x1b) also closes the commit screen
// Some terminals send ESC as a raw byte rather than the special tea.KeyEsc type
func TestCommitScreenRawEscapeKey(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	// Set up the commit screen via screen manager
	commitScr := appscreen.NewCommitScreen(appscreen.CommitMeta{SHA: "abc123"}, "stat", "diff", false, m.theme)
	m.state.ui.screenManager.Push(commitScr)

	// Simulate pressing ESC as a raw rune (how some terminals send it)
	rawEscMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{0x1b}}

	// Log what we're testing
	t.Logf("Raw ESC key string: %q", rawEscMsg.String())
	t.Logf("keyEscRaw constant: %q", keyEscRaw)

	// Call handleScreenKey
	newModel, _ := m.handleScreenKey(rawEscMsg)
	updatedModel := newModel.(*Model)

	// Verify the commit screen was closed via screen manager
	if updatedModel.state.ui.screenManager.IsActive() {
		t.Errorf("Expected screen manager to be inactive, got type %v", updatedModel.state.ui.screenManager.Type())
	}
}

// TestWorktreeLoadingFlow tests the complete flow from cache to loaded state
func TestWorktreeLoadingFlow(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	hasRepo := hasGitRepo(t)
	if !hasRepo {
		m.state.services.git = nil
	}

	// First, load cached worktrees
	cachedMsg := cachedWorktreesMsg{worktrees: []*models.WorktreeInfo{
		{Path: "/tmp/wt1", Branch: "main"},
	}}
	updated, _ := m.handleCachedWorktrees(cachedMsg)
	m = updated.(*Model)

	if hasRepo {
		if len(m.state.data.worktrees) != 0 {
			t.Fatalf("expected cached worktrees to be filtered, got %d", len(m.state.data.worktrees))
		}
	} else {
		if len(m.state.data.worktrees) != 1 {
			t.Fatalf("expected cached worktrees without git validation, got %d", len(m.state.data.worktrees))
		}
	}
	if m.worktreesLoaded {
		t.Error("expected worktreesLoaded to be false after cached load")
	}

	// Then load actual worktrees
	loadedMsg := worktreesLoadedMsg{
		worktrees: []*models.WorktreeInfo{
			{Path: "/tmp/wt1", Branch: "main"},
			{Path: "/tmp/wt2", Branch: "feat"},
		},
		err: nil,
	}
	updated, _ = m.handleWorktreesLoaded(loadedMsg)
	m = updated.(*Model)

	if !m.worktreesLoaded {
		t.Error("expected worktreesLoaded to be true after load")
	}
	if len(m.state.data.worktrees) != 2 {
		t.Fatalf("expected 2 worktrees after load, got %d", len(m.state.data.worktrees))
	}
}

// TestPRFetchingFlow tests PR data loading flow
func TestPRFetchingFlow(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: "/tmp/pr-123", Branch: "pr-123", PR: nil},
		{Path: "/tmp/main", Branch: "main", PR: nil},
	}

	prMap := map[string]*models.PRInfo{
		"pr-123": {Number: 123, Title: "Test PR", Branch: "pr-123"},
	}

	msg := prDataLoadedMsg{prMap: prMap, worktreePRs: nil, err: nil}
	updated, _ := m.handlePRDataLoaded(msg)
	m = updated.(*Model)

	if !m.prDataLoaded {
		t.Error("expected prDataLoaded to be true")
	}
	if m.state.data.worktrees[0].PR == nil {
		t.Error("expected PR data to be assigned to worktree")
	}
	if m.state.data.worktrees[0].PR.Number != 123 {
		t.Errorf("expected PR number 123, got %d", m.state.data.worktrees[0].PR.Number)
	}
}

// TestCIStatusCaching tests CI status loading and caching
func TestCIStatusCaching(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	// Load CI status for a branch
	checks := []*models.CICheck{
		{Name: "build", Conclusion: "success"},
		{Name: "test", Conclusion: "success"},
	}
	msg := ciStatusLoadedMsg{branch: "main", checks: checks, err: nil}
	updated, _ := m.handleCIStatusLoaded(msg)
	m = updated.(*Model)

	// Verify it's cached
	if cached, _, ok := m.cache.ciCache.Get("main"); !ok {
		t.Fatal("expected CI status to be cached for 'main' branch")
	} else if len(cached) != 2 {
		t.Errorf("expected 2 checks, got %d", len(cached))
	}

	// Load different branch and verify first one is still cached
	checks2 := []*models.CICheck{
		{Name: "build", Conclusion: "failure"},
	}
	msg2 := ciStatusLoadedMsg{branch: "dev", checks: checks2, err: nil}
	updated, _ = m.handleCIStatusLoaded(msg2)
	m = updated.(*Model)

	if _, _, ok := m.cache.ciCache.Get("main"); !ok {
		t.Error("expected 'main' branch to still be cached")
	}
	if _, _, ok := m.cache.ciCache.Get("dev"); !ok {
		t.Error("expected 'dev' branch to be cached")
	}
}

// TestMultipleErrorHandling tests proper error handling across message types
func TestMultipleErrorHandling(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	// Test worktrees load error
	loadMsg := worktreesLoadedMsg{worktrees: nil, err: os.ErrPermission}
	updated, _ := m.handleWorktreesLoaded(loadMsg)
	m = updated.(*Model)
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Error("expected error to show info screen")
	}

	// Test PR data error
	m.state.ui.screenManager.Pop()
	prMsg := prDataLoadedMsg{prMap: nil, worktreePRs: nil, err: os.ErrPermission}
	updated, _ = m.handlePRDataLoaded(prMsg)
	m = updated.(*Model)
	if m.prDataLoaded {
		t.Error("expected prDataLoaded to be false on error")
	}

	// Test CI status error
	m.cache.ciCache.Clear()
	ciMsg := ciStatusLoadedMsg{branch: "main", checks: nil, err: os.ErrPermission}
	updated, _ = m.handleCIStatusLoaded(ciMsg)
	m = updated.(*Model)
	if _, _, ok := m.cache.ciCache.Get("main"); ok {
		t.Error("expected CI cache to not be updated on error")
	}
}
