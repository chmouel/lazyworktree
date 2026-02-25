package theme

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestGetThemeWithCustoms_BuiltInTheme(t *testing.T) {
	customThemes := make(map[string]*CustomThemeData)

	// Test that built-in themes still work
	thm := GetThemeWithCustoms("dracula", customThemes)
	requireTheme(t, thm)

	// Verify it's actually Dracula theme
	if thm.Accent != lipgloss.Color("#BD93F9") {
		t.Errorf("expected Dracula accent, got %s", thm.Accent)
	}
}

func TestGetThemeWithCustoms_CustomThemeWithBase(t *testing.T) {
	customThemes := map[string]*CustomThemeData{
		"my-theme": {
			Base:   "dracula",
			Accent: "#FF6B9D",
		},
	}

	thm := GetThemeWithCustoms("my-theme", customThemes)
	requireTheme(t, thm)

	// Should have custom accent
	if thm.Accent != lipgloss.Color("#FF6B9D") {
		t.Errorf("expected custom accent #FF6B9D, got %s", thm.Accent)
	}

	// Other fields should be from Dracula
	if thm.TextFg != lipgloss.Color("#F8F8F2") {
		t.Errorf("expected Dracula text_fg, got %s", thm.TextFg)
	}
}

func TestGetThemeWithCustoms_CustomThemeWithoutBase(t *testing.T) {
	customThemes := map[string]*CustomThemeData{
		"complete-theme": {
			Accent:    "#00FF00",
			AccentFg:  "#000000",
			AccentDim: "#2A2A2A",
			Border:    "#3A3A3A",
			BorderDim: "#2A2A2A",
			MutedFg:   "#888888",
			TextFg:    "#FFFFFF",
			SuccessFg: "#00FF00",
			WarnFg:    "#FFFF00",
			ErrorFg:   "#FF0000",
			Cyan:      "#00FFFF",
		},
	}

	thm := GetThemeWithCustoms("complete-theme", customThemes)
	requireTheme(t, thm)

	if thm.Accent != lipgloss.Color("#00FF00") {
		t.Errorf("expected accent #00FF00, got %s", thm.Accent)
	}
}

func TestGetThemeWithCustoms_CustomInheritsCustom(t *testing.T) {
	customThemes := map[string]*CustomThemeData{
		"base-custom": {
			Base:   "dracula",
			Accent: "#FF0000",
		},
		"derived": {
			Base:   "base-custom",
			Accent: "#00FF00",
		},
	}

	thm := GetThemeWithCustoms("derived", customThemes)
	requireTheme(t, thm)

	// Should have derived accent (overrides base-custom)
	if thm.Accent != lipgloss.Color("#00FF00") {
		t.Errorf("expected derived accent #00FF00, got %s", thm.Accent)
	}

	// Should inherit other fields from Dracula
	if thm.TextFg != lipgloss.Color("#F8F8F2") {
		t.Errorf("expected Dracula text_fg, got %s", thm.TextFg)
	}
}

func TestGetThemeWithCustoms_MultiLevelInheritance(t *testing.T) {
	customThemes := map[string]*CustomThemeData{
		"level1": {
			Base:   "dracula",
			Accent: "#FF0000",
		},
		"level2": {
			Base:     "level1",
			AccentFg: "#FFFFFF",
		},
		"level3": {
			Base:   "level2",
			Accent: "#00FF00",
		},
	}

	thm := GetThemeWithCustoms("level3", customThemes)
	requireTheme(t, thm)

	// Should have level3 accent
	if thm.Accent != lipgloss.Color("#00FF00") {
		t.Errorf("expected level3 accent #00FF00, got %s", thm.Accent)
	}

	// Should have level2 accent_fg
	if thm.AccentFg != lipgloss.Color("#FFFFFF") {
		t.Errorf("expected level2 accent_fg #FFFFFF, got %s", thm.AccentFg)
	}

	// Should inherit other fields from Dracula
	if thm.TextFg != lipgloss.Color("#F8F8F2") {
		t.Errorf("expected Dracula text_fg, got %s", thm.TextFg)
	}
}

func TestGetThemeWithCustoms_UnknownTheme(t *testing.T) {
	customThemes := make(map[string]*CustomThemeData)

	thm := GetThemeWithCustoms("nonexistent", customThemes)
	requireTheme(t, thm)

	// Should fallback to Dracula
	if thm.Accent != lipgloss.Color("#BD93F9") {
		t.Errorf("expected Dracula fallback, got %s", thm.Accent)
	}
}

func TestGetThemeWithCustoms_EmptyName(t *testing.T) {
	customThemes := make(map[string]*CustomThemeData)

	thm := GetThemeWithCustoms("", customThemes)
	requireTheme(t, thm)

	// Should fallback to Dracula
	if thm.Accent != lipgloss.Color("#BD93F9") {
		t.Errorf("expected Dracula fallback, got %s", thm.Accent)
	}
}

func TestMergeTheme_PartialOverrides(t *testing.T) {
	base := Dracula()
	custom := &CustomThemeData{
		Accent: "#FF6B9D",
		TextFg: "#E8E8E8",
	}

	merged := MergeTheme(base, custom)

	// Overridden fields
	if merged.Accent != lipgloss.Color("#FF6B9D") {
		t.Errorf("expected custom accent, got %s", merged.Accent)
	}
	if merged.TextFg != lipgloss.Color("#E8E8E8") {
		t.Errorf("expected custom text_fg, got %s", merged.TextFg)
	}

	// Non-overridden fields should be from base
	if merged.SuccessFg != base.SuccessFg {
		t.Errorf("expected base success_fg, got %s", merged.SuccessFg)
	}
}

func TestMergeTheme_AllFieldsOverridden(t *testing.T) {
	base := Dracula()
	custom := &CustomThemeData{
		Accent:    "#00FF00",
		AccentFg:  "#000000",
		AccentDim: "#2A2A2A",
		Border:    "#3A3A3A",
		BorderDim: "#2A2A2A",
		MutedFg:   "#888888",
		TextFg:    "#FFFFFF",
		SuccessFg: "#00FF00",
		WarnFg:    "#FFFF00",
		ErrorFg:   "#FF0000",
		Cyan:      "#00FFFF",
	}

	merged := MergeTheme(base, custom)

	if merged.Accent != lipgloss.Color("#00FF00") {
		t.Errorf("expected custom accent, got %s", merged.Accent)
	}
	if merged.TextFg != lipgloss.Color("#FFFFFF") {
		t.Errorf("expected custom text_fg, got %s", merged.TextFg)
	}
}

func TestMergeTheme_NoOverrides(t *testing.T) {
	base := Dracula()
	custom := &CustomThemeData{
		Base: "dracula",
		// No color overrides
	}

	merged := MergeTheme(base, custom)

	// Should be identical to base
	if merged.Accent != base.Accent {
		t.Errorf("expected base accent, got %s", merged.Accent)
	}
}

func TestAvailableThemesWithCustoms(t *testing.T) {
	customThemes := map[string]*CustomThemeData{
		"custom1": {Base: "dracula"},
		"custom2": {Base: "narna"},
	}

	themes := AvailableThemesWithCustoms(customThemes)

	// Should include built-in themes
	builtInCount := len(AvailableThemes())
	if len(themes) < builtInCount {
		t.Errorf("expected at least %d themes (built-in), got %d", builtInCount, len(themes))
	}

	// Should include custom themes
	hasCustom1 := false
	hasCustom2 := false
	for _, name := range themes {
		if name == "custom1" {
			hasCustom1 = true
		}
		if name == "custom2" {
			hasCustom2 = true
		}
	}

	if !hasCustom1 {
		t.Error("custom1 not found in available themes")
	}
	if !hasCustom2 {
		t.Error("custom2 not found in available themes")
	}
}

func TestAvailableThemesWithCustoms_Empty(t *testing.T) {
	customThemes := make(map[string]*CustomThemeData)

	themes := AvailableThemesWithCustoms(customThemes)
	builtInThemes := AvailableThemes()

	if len(themes) != len(builtInThemes) {
		t.Errorf("expected %d themes (built-in only), got %d", len(builtInThemes), len(themes))
	}
}

func TestIsBuiltInTheme(t *testing.T) {
	builtInThemes := AvailableThemes()

	for _, name := range builtInThemes {
		if !isBuiltInTheme(name) {
			t.Errorf("expected %s to be recognized as built-in", name)
		}
		if !isBuiltInTheme(strings.ToLower(name)) {
			t.Errorf("expected %s (lowercase) to be recognized as built-in", name)
		}
	}

	if isBuiltInTheme("nonexistent") {
		t.Error("expected nonexistent theme to not be built-in")
	}
}

func requireTheme(t *testing.T, thm *Theme) {
	t.Helper()
	if thm == nil {
		t.Fatal("expected theme, got nil")
	}
}

func TestThemeFromCustom(t *testing.T) {
	custom := &CustomThemeData{
		Accent:    "#00FF00",
		AccentFg:  "#000000",
		AccentDim: "#2A2A2A",
		Border:    "#3A3A3A",
		BorderDim: "#2A2A2A",
		MutedFg:   "#888888",
		TextFg:    "#FFFFFF",
		SuccessFg: "#00FF00",
		WarnFg:    "#FFFF00",
		ErrorFg:   "#FF0000",
		Cyan:      "#00FFFF",
	}

	thm := themeFromCustom(custom)

	if thm.Accent != lipgloss.Color("#00FF00") {
		t.Errorf("expected accent #00FF00, got %s", thm.Accent)
	}
	if thm.TextFg != lipgloss.Color("#FFFFFF") {
		t.Errorf("expected text_fg #FFFFFF, got %s", thm.TextFg)
	}
}
