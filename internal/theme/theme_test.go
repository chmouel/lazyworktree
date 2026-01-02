package theme

import "testing"

func TestGetTheme(t *testing.T) {
	got := GetTheme("narna")
	if got == nil {
		t.Fatal("expected theme to be returned")
	}
	if got.Background != Narna().Background {
		t.Fatalf("expected narna background %q, got %q", Narna().Background, got.Background)
	}

	fallback := GetTheme("unknown")
	if fallback.Background != Dracula().Background {
		t.Fatalf("expected Dracula fallback, got %q", fallback.Background)
	}
}

func TestAvailableThemesIncludesDefaults(t *testing.T) {
	themes := AvailableThemes()
	required := map[string]bool{
		"dracula":          false,
		"narna":            false,
		"clean-light":      false,
		"solarized-dark":   false,
		"solarized-light":  false,
		"gruvbox-dark":     false,
		"gruvbox-light":    false,
		"nord":             false,
		"monokai":          false,
		"catppuccin-mocha": false,
	}

	for _, name := range themes {
		if _, ok := required[name]; ok {
			required[name] = true
		}
	}

	for name, seen := range required {
		if !seen {
			t.Fatalf("expected theme %q to be available", name)
		}
	}
}
