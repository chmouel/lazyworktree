package config

import (
	"testing"

	"github.com/chmouel/lazyworktree/internal/theme"
)

func TestParseCustomThemes_ValidWithBase(t *testing.T) {
	data := map[string]any{
		"custom_themes": map[string]any{
			"my-dark": map[string]any{
				"base":    "dracula",
				"accent":  "#FF6B9D",
				"text_fg": "#E8E8E8",
			},
		},
	}

	themes := parseCustomThemes(data)

	if len(themes) != 1 {
		t.Fatalf("expected 1 theme, got %d", len(themes))
	}

	customTheme, ok := themes["my-dark"]
	if !ok {
		t.Fatal("theme 'my-dark' not found")
	}

	if customTheme.Base != "dracula" {
		t.Errorf("expected base 'dracula', got %s", customTheme.Base)
	}
	if customTheme.Accent != "#FF6B9D" {
		t.Errorf("expected accent '#FF6B9D', got %s", customTheme.Accent)
	}
	if customTheme.TextFg != "#E8E8E8" {
		t.Errorf("expected text_fg '#E8E8E8', got %s", customTheme.TextFg)
	}
}

func TestParseCustomThemes_ValidWithoutBase(t *testing.T) {
	data := map[string]any{
		"custom_themes": map[string]any{
			"complete-theme": map[string]any{
				"accent":     "#00FF00",
				"accent_fg":  "#000000",
				"accent_dim": "#2A2A2A",
				"border":     "#3A3A3A",
				"border_dim": "#2A2A2A",
				"muted_fg":   "#888888",
				"text_fg":    "#FFFFFF",
				"success_fg": "#00FF00",
				"warn_fg":    "#FFFF00",
				"error_fg":   "#FF0000",
				"cyan":       "#00FFFF",
			},
		},
	}

	themes := parseCustomThemes(data)

	if len(themes) != 1 {
		t.Fatalf("expected 1 theme, got %d", len(themes))
	}

	customTheme, ok := themes["complete-theme"]
	if !ok {
		t.Fatal("theme 'complete-theme' not found")
	}

	if customTheme.Base != "" {
		t.Errorf("expected empty base, got %s", customTheme.Base)
	}
	if customTheme.Accent != "#00FF00" {
		t.Errorf("expected accent '#00FF00', got %s", customTheme.Accent)
	}
}

func TestParseCustomThemes_MissingRequiredFields(t *testing.T) {
	// Test missing accent
	data := map[string]any{
		"custom_themes": map[string]any{
			"incomplete": map[string]any{
				"text_fg": "#FFFFFF",
				// missing accent and other required fields
			},
		},
	}

	themes := parseCustomThemes(data)
	if len(themes) != 0 {
		t.Errorf("expected 0 themes (invalid), got %d", len(themes))
	}
}

func TestParseCustomThemes_MissingEachField(t *testing.T) {
	requiredFields := []string{
		"accent", "accent_fg", "accent_dim", "border",
		"border_dim", "muted_fg", "text_fg", "success_fg", "warn_fg",
		"error_fg", "cyan",
	}

	completeTheme := map[string]any{
		"accent":     "#00FF00",
		"accent_fg":  "#000000",
		"accent_dim": "#2A2A2A",
		"border":     "#3A3A3A",
		"border_dim": "#2A2A2A",
		"muted_fg":   "#888888",
		"text_fg":    "#FFFFFF",
		"success_fg": "#00FF00",
		"warn_fg":    "#FFFF00",
		"error_fg":   "#FF0000",
		"cyan":       "#00FFFF",
	}

	for _, missingField := range requiredFields {
		t.Run("missing_"+missingField, func(t *testing.T) {
			incompleteTheme := make(map[string]any)
			for k, v := range completeTheme {
				if k != missingField {
					incompleteTheme[k] = v
				}
			}

			data := map[string]any{
				"custom_themes": map[string]any{
					"test-theme": incompleteTheme,
				},
			}

			themes := parseCustomThemes(data)
			if len(themes) != 0 {
				t.Errorf("expected 0 themes when missing %s, got %d", missingField, len(themes))
			}
		})
	}
}

func TestParseCustomThemes_InvalidColorFormat(t *testing.T) {
	testCases := []struct {
		name  string
		color string
	}{
		{"missing_hash", "FF0000"},
		{"wrong_length_short", "#FF"},
		{"wrong_length_long", "#FF000000"},
		{"invalid_chars", "#GG0000"},
		{"empty", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data := map[string]any{
				"custom_themes": map[string]any{
					"test": map[string]any{
						"base":   "dracula",
						"accent": tc.color,
					},
				},
			}

			themes := parseCustomThemes(data)
			// Should either reject the theme or skip invalid color
			// Since we validate colors, invalid themes should be rejected
			if tc.color != "" {
				// Empty color is allowed when base is present (it's just not overriding)
				if len(themes) != 0 {
					t.Errorf("expected theme with invalid color to be rejected, but got %d themes", len(themes))
				}
			}
		})
	}
}

func TestParseCustomThemes_ValidColorFormats(t *testing.T) {
	testCases := []struct {
		name  string
		color string
		valid bool
	}{
		{"hex_6_digits", "#FF0000", true},
		{"hex_3_digits", "#F00", true},
		{"lowercase", "#ff0000", true},
		{"mixed_case", "#Ff0000", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data := map[string]any{
				"custom_themes": map[string]any{
					"test": map[string]any{
						"base":   "dracula",
						"accent": tc.color,
					},
				},
			}

			themes := parseCustomThemes(data)
			if tc.valid {
				if len(themes) != 1 {
					t.Errorf("expected 1 valid theme, got %d", len(themes))
				}
			}
		})
	}
}

func TestParseCustomThemes_ConflictsWithBuiltIn(t *testing.T) {
	builtInThemes := theme.AvailableThemes()
	if len(builtInThemes) == 0 {
		t.Fatal("no built-in themes found")
	}

	conflictName := builtInThemes[0]
	data := map[string]any{
		"custom_themes": map[string]any{
			conflictName: map[string]any{
				"base":   "dracula",
				"accent": "#FF0000",
			},
		},
	}

	themes := parseCustomThemes(data)
	if len(themes) != 0 {
		t.Errorf("expected 0 themes (conflict with built-in), got %d", len(themes))
	}
}

func TestParseCustomThemes_MultipleThemes(t *testing.T) {
	data := map[string]any{
		"custom_themes": map[string]any{
			"theme1": map[string]any{
				"base":   "dracula",
				"accent": "#FF0000",
			},
			"theme2": map[string]any{
				"base":   "narna",
				"accent": "#00FF00",
			},
		},
	}

	themes := parseCustomThemes(data)
	if len(themes) != 2 {
		t.Fatalf("expected 2 themes, got %d", len(themes))
	}

	if _, ok := themes["theme1"]; !ok {
		t.Error("theme1 not found")
	}
	if _, ok := themes["theme2"]; !ok {
		t.Error("theme2 not found")
	}
}

func TestParseCustomThemes_CustomInheritsCustom(t *testing.T) {
	data := map[string]any{
		"custom_themes": map[string]any{
			"base-theme": map[string]any{
				"base":   "dracula",
				"accent": "#FF0000",
			},
			"derived-theme": map[string]any{
				"base":   "base-theme",
				"accent": "#00FF00",
			},
		},
	}

	themes := parseCustomThemes(data)
	if len(themes) != 2 {
		t.Fatalf("expected 2 themes, got %d", len(themes))
	}

	derived, ok := themes["derived-theme"]
	if !ok {
		t.Fatal("derived-theme not found")
	}
	if derived.Base != "base-theme" {
		t.Errorf("expected base 'base-theme', got %s", derived.Base)
	}
}

func TestParseCustomThemes_CircularDependency(t *testing.T) {
	data := map[string]any{
		"custom_themes": map[string]any{
			"theme-a": map[string]any{
				"base":   "theme-b",
				"accent": "#FF0000",
			},
			"theme-b": map[string]any{
				"base":   "theme-a",
				"accent": "#00FF00",
			},
		},
	}

	themes := parseCustomThemes(data)
	// Circular dependencies should be detected and themes rejected
	// Both themes should be rejected due to circular dependency
	if len(themes) > 0 {
		t.Errorf("expected 0 themes (circular dependency), got %d", len(themes))
	}
}

func TestParseCustomThemes_SelfReference(t *testing.T) {
	data := map[string]any{
		"custom_themes": map[string]any{
			"self-ref": map[string]any{
				"base":   "self-ref",
				"accent": "#FF0000",
			},
		},
	}

	themes := parseCustomThemes(data)
	if len(themes) != 0 {
		t.Errorf("expected 0 themes (self-reference), got %d", len(themes))
	}
}

func TestParseCustomThemes_MissingBaseTheme(t *testing.T) {
	data := map[string]any{
		"custom_themes": map[string]any{
			"invalid": map[string]any{
				"base":   "nonexistent-theme",
				"accent": "#FF0000",
			},
		},
	}

	themes := parseCustomThemes(data)
	if len(themes) != 0 {
		t.Errorf("expected 0 themes (missing base), got %d", len(themes))
	}
}

func TestParseCustomThemes_EmptyMap(t *testing.T) {
	data := map[string]any{
		"custom_themes": map[string]any{},
	}

	themes := parseCustomThemes(data)
	if len(themes) != 0 {
		t.Errorf("expected 0 themes, got %d", len(themes))
	}
}

func TestParseCustomThemes_InvalidStructure(t *testing.T) {
	testCases := []struct {
		name string
		data map[string]any
	}{
		{"not_a_map", map[string]any{"custom_themes": "not a map"}},
		{"nil", map[string]any{"custom_themes": nil}},
		{"array", map[string]any{"custom_themes": []any{}}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			themes := parseCustomThemes(tc.data)
			if len(themes) != 0 {
				t.Errorf("expected 0 themes for invalid structure, got %d", len(themes))
			}
		})
	}
}

func TestValidateColorHex(t *testing.T) {
	validColors := []string{
		"#FF0000",
		"#ff0000",
		"#Ff0000",
		"#F00",
		"#f00",
		"#123456",
		"#000000",
		"#FFFFFF",
	}

	invalidColors := []string{
		"FF0000",    // missing #
		"#FF",       // too short
		"#FF000000", // too long
		"#GG0000",   // invalid chars
		"#FF00GG",   // invalid chars
		"",          // empty
		"#",         // just hash
		"##FF0000",  // double hash
	}

	for _, color := range validColors {
		if !validateColorHex(color) {
			t.Errorf("expected %s to be valid", color)
		}
	}

	for _, color := range invalidColors {
		if validateColorHex(color) {
			t.Errorf("expected %s to be invalid", color)
		}
	}
}

func TestValidateCompleteTheme(t *testing.T) {
	completeTheme := &CustomTheme{
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

	if err := validateCompleteTheme(completeTheme); err != nil {
		t.Errorf("expected complete theme to be valid, got error: %v", err)
	}

	// Test missing each field
	fields := []struct {
		name   string
		setter func(*CustomTheme)
	}{
		{"accent", func(ct *CustomTheme) { ct.Accent = "" }},
		{"accent_fg", func(ct *CustomTheme) { ct.AccentFg = "" }},
		{"accent_dim", func(ct *CustomTheme) { ct.AccentDim = "" }},
		{"border", func(ct *CustomTheme) { ct.Border = "" }},
		{"border_dim", func(ct *CustomTheme) { ct.BorderDim = "" }},
		{"muted_fg", func(ct *CustomTheme) { ct.MutedFg = "" }},
		{"text_fg", func(ct *CustomTheme) { ct.TextFg = "" }},
		{"success_fg", func(ct *CustomTheme) { ct.SuccessFg = "" }},
		{"warn_fg", func(ct *CustomTheme) { ct.WarnFg = "" }},
		{"error_fg", func(ct *CustomTheme) { ct.ErrorFg = "" }},
		{"cyan", func(ct *CustomTheme) { ct.Cyan = "" }},
	}

	for _, field := range fields {
		t.Run("missing_"+field.name, func(t *testing.T) {
			incomplete := *completeTheme
			field.setter(&incomplete)
			if err := validateCompleteTheme(&incomplete); err == nil {
				t.Errorf("expected error when missing %s, got nil", field.name)
			}
		})
	}
}
