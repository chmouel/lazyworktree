package commands

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
)

func TestUpdateShortcutsClearsStaleOwner(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	r.Register(
		CommandAction{ID: "lazygit", Label: "LazyGit", Shortcut: "g", Handler: func() tea.Cmd { return nil }},
		CommandAction{ID: "fetch", Label: "Fetch", Shortcut: "R", Handler: func() tea.Cmd { return nil }},
	)

	// Rebind "g" to "fetch" — "lazygit" should lose its shortcut.
	r.UpdateShortcuts(map[string]string{"g": "fetch"})

	for _, a := range r.Actions() {
		switch a.ID {
		case "lazygit":
			assert.Empty(t, a.Shortcut, "lazygit should have its shortcut cleared")
		case "fetch":
			assert.Equal(t, "g", a.Shortcut, "fetch should now have shortcut g")
		}
	}
}
