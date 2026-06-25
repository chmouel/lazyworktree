package app

import (
	"testing"

	"charm.land/bubbles/v2/spinner"
	"github.com/chmouel/lazyworktree/internal/config"
)

// TestSpinnerStopsWhenIdle guards against the spinner tick loop running while
// nothing is loading. A perpetual tick re-renders the whole UI ~12 times a
// second and pegs a CPU core, even though the spinner is not even displayed.
func TestSpinnerStopsWhenIdle(t *testing.T) {
	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir()}, "")
	m.loading.active = false
	m.state.ui.spinnerActive = true

	_, cmd := m.Update(spinner.TickMsg{})

	if cmd != nil {
		t.Fatal("idle spinner tick should not schedule another tick")
	}
	if m.state.ui.spinnerActive {
		t.Fatal("spinner should be marked inactive when idle")
	}
}

// TestSpinnerTicksWhileLoading ensures the animation keeps running during a
// loading operation (the tick reschedules itself).
func TestSpinnerTicksWhileLoading(t *testing.T) {
	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir()}, "")
	m.loading.active = true
	m.state.ui.spinnerActive = true

	// Generate a properly-tagged tick message via the spinner's own command.
	msg := m.state.ui.spinner.Tick()
	_, cmd := m.Update(msg)

	if cmd == nil {
		t.Fatal("spinner should keep ticking while loading")
	}
	if !m.state.ui.spinnerActive {
		t.Fatal("spinner should remain active while loading")
	}
}
