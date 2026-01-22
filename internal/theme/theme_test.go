package theme

import "testing"

func TestGetTheme(t *testing.T) {
	for _, name := range AvailableThemes() {
		got := GetTheme(name)
		if got == nil {
			t.Fatalf("expected theme for %q", name)
		}
	}

	fallback := GetTheme("unknown")
	if fallback.Accent != Dracula().Accent {
		t.Fatalf("expected Dracula fallback, got %q", fallback.Accent)
	}
}

func TestAvailableThemesIncludesDefaults(t *testing.T) {
	themes := AvailableThemes()
	required := map[string]bool{
		"dracula":          false,
		"dracula-light":    false,
		"narna":            false,
		"clean-light":      false,
		"catppuccin-latte": false,
		"rose-pine-dawn":   false,
		"one-light":        false,
		"everforest-light": false,
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

func TestDefaults(t *testing.T) {
	if got := DefaultDark(); got != DraculaName {
		t.Errorf("DefaultDark() = %q, want %q", got, DraculaName)
	}
	if got := DefaultLight(); got != DraculaLightName {
		t.Errorf("DefaultLight() = %q, want %q", got, DraculaLightName)
	}
}
