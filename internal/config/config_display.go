package config

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/chmouel/lazyworktree/internal/theme"
)

// deltaSyntaxThemeFlag is the delta CLI flag for setting the syntax highlight theme.
const deltaSyntaxThemeFlag = "--syntax-theme"

// LayoutSizes holds user-configurable pane size weights.
// Values are relative weights (1–100) that get normalised at computation time.
// A nil LayoutSizes means the hardcoded defaults are used.
type LayoutSizes struct {
	Worktrees     int // Main pane width (default layout) or height (top layout)
	Info          int // Info pane share of secondary area
	GitStatus     int // Git status pane share (when visible)
	Commit        int // Commit log pane share
	Notes         int // Notes pane share (when visible)
	AgentSessions int // Agent sessions pane share (when visible)
}

// CustomTheme represents a user-defined theme that can inherit from built-in or other custom themes.
type CustomTheme struct {
	Base      string // Optional base theme name (built-in or custom)
	Accent    string
	AccentFg  string
	AccentDim string
	Border    string
	BorderDim string
	MutedFg   string
	TextFg    string
	SuccessFg string
	WarnFg    string
	ErrorFg   string
	Cyan      string
}

var iconSetOptions = []string{"nerd-font-v3", "text"}

// IconsEnabled reports whether icon rendering should be enabled for the current icon set.
func (c *AppConfig) IconsEnabled() bool {
	iconSet := strings.ToLower(strings.TrimSpace(c.IconSet))
	return iconSet != ""
}

func iconSetOptionsString() string {
	return strings.Join(iconSetOptions, ", ")
}

func parseCustomThemes(data map[string]any) map[string]*CustomTheme {
	raw, ok := data["custom_themes"].(map[string]any)
	if !ok {
		return make(map[string]*CustomTheme)
	}

	themes := make(map[string]*CustomTheme)
	builtInThemes := theme.AvailableThemes()
	builtInMap := make(map[string]bool)
	for _, name := range builtInThemes {
		builtInMap[strings.ToLower(name)] = true
	}

	for name, val := range raw {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			continue
		}

		if builtInMap[name] {
			continue
		}

		themeData, ok := val.(map[string]any)
		if !ok {
			continue
		}

		customTheme := &CustomTheme{
			Base:      strings.TrimSpace(getString(themeData, "base")),
			Accent:    strings.TrimSpace(getString(themeData, "accent")),
			AccentFg:  strings.TrimSpace(getString(themeData, "accent_fg")),
			AccentDim: strings.TrimSpace(getString(themeData, "accent_dim")),
			Border:    strings.TrimSpace(getString(themeData, "border")),
			BorderDim: strings.TrimSpace(getString(themeData, "border_dim")),
			MutedFg:   strings.TrimSpace(getString(themeData, "muted_fg")),
			TextFg:    strings.TrimSpace(getString(themeData, "text_fg")),
			SuccessFg: strings.TrimSpace(getString(themeData, "success_fg")),
			WarnFg:    strings.TrimSpace(getString(themeData, "warn_fg")),
			ErrorFg:   strings.TrimSpace(getString(themeData, "error_fg")),
			Cyan:      strings.TrimSpace(getString(themeData, "cyan")),
		}

		if customTheme.Base == "" {
			if err := validateCompleteTheme(customTheme); err != nil {
				continue
			}
		}

		if !validateThemeColors(customTheme) {
			continue
		}

		themes[name] = customTheme
	}

	validatedThemes := make(map[string]*CustomTheme)
	for name, customTheme := range themes {
		if validateThemeInheritance(name, customTheme, themes, builtInMap, make(map[string]bool)) {
			validatedThemes[name] = customTheme
		}
	}

	return validatedThemes
}

// validateCompleteTheme validates that all required fields are present when base is not specified.
func validateCompleteTheme(custom *CustomTheme) error {
	var missing []string

	if custom.Accent == "" {
		missing = append(missing, "accent")
	}
	if custom.AccentFg == "" {
		missing = append(missing, "accent_fg")
	}
	if custom.AccentDim == "" {
		missing = append(missing, "accent_dim")
	}
	if custom.Border == "" {
		missing = append(missing, "border")
	}
	if custom.BorderDim == "" {
		missing = append(missing, "border_dim")
	}
	if custom.MutedFg == "" {
		missing = append(missing, "muted_fg")
	}
	if custom.TextFg == "" {
		missing = append(missing, "text_fg")
	}
	if custom.SuccessFg == "" {
		missing = append(missing, "success_fg")
	}
	if custom.WarnFg == "" {
		missing = append(missing, "warn_fg")
	}
	if custom.ErrorFg == "" {
		missing = append(missing, "error_fg")
	}
	if custom.Cyan == "" {
		missing = append(missing, "cyan")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required fields: %s", strings.Join(missing, ", "))
	}

	return nil
}

// validateThemeColors validates all color hex values in a custom theme.
func validateThemeColors(custom *CustomTheme) bool {
	colors := []string{
		custom.Accent,
		custom.AccentFg,
		custom.AccentDim,
		custom.Border,
		custom.BorderDim,
		custom.MutedFg,
		custom.TextFg,
		custom.SuccessFg,
		custom.WarnFg,
		custom.ErrorFg,
		custom.Cyan,
	}

	for _, color := range colors {
		if color != "" && !validateColorHex(color) {
			return false
		}
	}

	return true
}

// validateColorHex validates hex color format (#RRGGBB or #RGB).
func validateColorHex(color string) bool {
	if color == "" {
		return false
	}

	if color[0] != '#' {
		return false
	}

	hex := color[1:]
	if len(hex) != 3 && len(hex) != 6 {
		return false
	}

	matched, _ := regexp.MatchString("^[0-9A-Fa-f]+$", hex)
	return matched
}

// validateThemeInheritance validates inheritance chains, checking for circular dependencies and ensuring base themes exist.
func validateThemeInheritance(name string, custom *CustomTheme, themes map[string]*CustomTheme, builtInMap, visited map[string]bool) bool {
	if custom.Base == "" {
		return true
	}

	baseName := strings.ToLower(custom.Base)

	if visited[baseName] {
		return false
	}

	if builtInMap[baseName] {
		return true
	}

	baseTheme, exists := themes[baseName]
	if !exists {
		return false
	}

	visited[name] = true
	return validateThemeInheritance(baseName, baseTheme, themes, builtInMap, visited)
}

// clampLayoutSize clamps a layout size value to the range 1–100.
func clampLayoutSize(val, def int) int {
	if val <= 0 {
		return def
	}
	if val > 100 {
		return 100
	}
	return val
}

func parseLayoutSizes(data map[string]any) *LayoutSizes {
	raw, ok := data["layout_sizes"].(map[string]any)
	if !ok {
		return nil
	}

	ls := &LayoutSizes{
		Worktrees:     0,
		Info:          0,
		GitStatus:     0,
		Commit:        0,
		Notes:         0,
		AgentSessions: 0,
	}

	if v, exists := raw["worktrees"]; exists {
		ls.Worktrees = clampLayoutSize(coerceInt(v, 0), 0)
	}
	if v, exists := raw["info"]; exists {
		ls.Info = clampLayoutSize(coerceInt(v, 0), 0)
	}
	if v, exists := raw["git_status"]; exists {
		ls.GitStatus = clampLayoutSize(coerceInt(v, 0), 0)
	}
	if v, exists := raw["commit"]; exists {
		ls.Commit = clampLayoutSize(coerceInt(v, 0), 0)
	}
	if v, exists := raw["notes"]; exists {
		ls.Notes = clampLayoutSize(coerceInt(v, 0), 0)
	}
	if v, exists := raw["agent_sessions"]; exists {
		ls.AgentSessions = clampLayoutSize(coerceInt(v, 0), 0)
	}

	return ls
}

// SyntaxThemeForUITheme returns the syntax theme name for a given TUI theme.
func SyntaxThemeForUITheme(themeName string) string {
	args := DefaultDeltaArgsForTheme(themeName)
	for i := 0; i < len(args)-1; i++ {
		if args[i] == deltaSyntaxThemeFlag {
			return args[i+1]
		}
	}
	return "Dracula"
}

// DefaultDeltaArgsForTheme returns the default delta arguments for a given theme.
func DefaultDeltaArgsForTheme(themeName string) []string {
	switch themeName {
	case theme.DraculaLightName:
		return []string{deltaSyntaxThemeFlag, "\"Monokai Extended Light\""}
	case theme.NarnaName:
		return []string{deltaSyntaxThemeFlag, "\"OneHalfDark\""}
	case theme.CleanLightName:
		return []string{deltaSyntaxThemeFlag, "GitHub"}
	case theme.CatppuccinLatteName:
		return []string{deltaSyntaxThemeFlag, "\"Catppuccin Latte\""}
	case theme.RosePineDawnName:
		return []string{deltaSyntaxThemeFlag, "GitHub"}
	case theme.OneLightName:
		return []string{deltaSyntaxThemeFlag, "\"OneHalfLight\""}
	case theme.EverforestLightName:
		return []string{deltaSyntaxThemeFlag, "\"Gruvbox Light\""}
	case theme.SolarizedDarkName:
		return []string{deltaSyntaxThemeFlag, "\"Solarized (dark)\""}
	case theme.SolarizedLightName:
		return []string{deltaSyntaxThemeFlag, "\"Solarized (light)\""}
	case theme.GruvboxDarkName:
		return []string{deltaSyntaxThemeFlag, "\"Gruvbox Dark\""}
	case theme.GruvboxLightName:
		return []string{deltaSyntaxThemeFlag, "\"Gruvbox Light\""}
	case theme.NordName:
		return []string{deltaSyntaxThemeFlag, "\"Nord\""}
	case theme.MonokaiName:
		return []string{deltaSyntaxThemeFlag, "\"Monokai Extended\""}
	case theme.CatppuccinMochaName:
		return []string{deltaSyntaxThemeFlag, "\"Catppuccin Mocha\""}
	case theme.ModernName:
		return []string{deltaSyntaxThemeFlag, "Dracula"}
	case theme.TokyoNightName:
		return []string{deltaSyntaxThemeFlag, "Dracula"}
	case theme.OneDarkName:
		return []string{deltaSyntaxThemeFlag, "\"OneHalfDark\""}
	case theme.RosePineName:
		return []string{deltaSyntaxThemeFlag, "Dracula"}
	case theme.AyuMirageName:
		return []string{deltaSyntaxThemeFlag, "Dracula"}
	case theme.EverforestDarkName:
		return []string{deltaSyntaxThemeFlag, "Dracula"}
	case theme.KanagawaName:
		return []string{deltaSyntaxThemeFlag, "Dracula"}
	default:
		return []string{deltaSyntaxThemeFlag, "Dracula"}
	}
}

// NormalizeThemeName returns the normalized theme name if valid, otherwise empty string.
func NormalizeThemeName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	switch name {
	case "dracula", "dracula-light", "narna", "clean-light", "catppuccin-latte", "rose-pine-dawn", "one-light", "everforest-light", "solarized-dark", "solarized-light", "gruvbox-dark", "gruvbox-light", "nord", "monokai", "catppuccin-mocha", "modern", "tokyo-night", "one-dark", "rose-pine", "ayu-mirage", "everforest-dark", "kanagawa":
		return name
	}
	return ""
}

// ToThemeData converts CustomTheme to theme.CustomThemeData to avoid circular dependencies.
func (ct *CustomTheme) ToThemeData() *theme.CustomThemeData {
	if ct == nil {
		return nil
	}
	return &theme.CustomThemeData{
		Base:      ct.Base,
		Accent:    ct.Accent,
		AccentFg:  ct.AccentFg,
		AccentDim: ct.AccentDim,
		Border:    ct.Border,
		BorderDim: ct.BorderDim,
		MutedFg:   ct.MutedFg,
		TextFg:    ct.TextFg,
		SuccessFg: ct.SuccessFg,
		WarnFg:    ct.WarnFg,
		ErrorFg:   ct.ErrorFg,
		Cyan:      ct.Cyan,
	}
}

// CustomThemesToThemeDataMap converts a map of CustomTheme to theme.CustomThemeData.
func CustomThemesToThemeDataMap(customThemes map[string]*CustomTheme) map[string]*theme.CustomThemeData {
	if customThemes == nil {
		return nil
	}
	result := make(map[string]*theme.CustomThemeData)
	for name, ct := range customThemes {
		result[name] = ct.ToThemeData()
	}
	return result
}
