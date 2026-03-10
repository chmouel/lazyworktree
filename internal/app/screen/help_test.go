package screen

import (
	"strings"
	"testing"

	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/theme"
)

func TestHelpScreenCustomCommandsShowPaletteOnlyEntries(t *testing.T) {
	help := NewHelpScreen(120, 40, config.CustomCommandsConfig{
		config.PaneUniversal: {
			"x": {
				Description: "Run tests",
				ShowHelp:    true,
			},
			"_review": {
				Description: "Review",
				ShowHelp:    true,
			},
		},
	}, nil, theme.Dracula(), false)

	text := strings.Join(help.FullText, "\n")
	if !strings.Contains(text, "- x: Run tests") {
		t.Fatalf("expected keyed custom command in help text, got %q", text)
	}
	if !strings.Contains(text, "- review: Review (command palette only)") {
		t.Fatalf("expected palette-only custom command in help text, got %q", text)
	}
}
