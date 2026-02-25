// Package theme provides theme definitions and management for the TUI.
package theme

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// CustomThemeData represents custom theme data without importing config package.
// This avoids circular dependencies.
type CustomThemeData struct {
	Base      string
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

// Theme defines all colors used in the application UI.
type Theme struct {
	Accent    color.Color
	AccentFg  color.Color // Foreground color for text on Accent background
	AccentDim color.Color
	Border    color.Color
	BorderDim color.Color
	MutedFg   color.Color
	TextFg    color.Color
	SuccessFg color.Color
	WarnFg    color.Color
	ErrorFg   color.Color
	Cyan      color.Color
}

// Theme names.
const (
	DraculaName         = "dracula"
	DraculaLightName    = "dracula-light"
	NarnaName           = "narna"
	CleanLightName      = "clean-light"
	SolarizedDarkName   = "solarized-dark"
	SolarizedLightName  = "solarized-light"
	GruvboxDarkName     = "gruvbox-dark"
	GruvboxLightName    = "gruvbox-light"
	NordName            = "nord"
	MonokaiName         = "monokai"
	CatppuccinMochaName = "catppuccin-mocha"
	CatppuccinLatteName = "catppuccin-latte"
	RosePineDawnName    = "rose-pine-dawn"
	OneLightName        = "one-light"
	EverforestLightName = "everforest-light"
	EverforestDarkName  = "everforest-dark"
	ModernName          = "modern"
	TokyoNightName      = "tokyo-night"
	OneDarkName         = "one-dark"
	RosePineName        = "rose-pine"
	AyuMirageName       = "ayu-mirage"
	KanagawaName        = "kanagawa"
)

// Dracula returns the Dracula theme (dark background, vibrant colors).
func Dracula() *Theme {
	return &Theme{
		Accent:    lipgloss.Color("#BD93F9"), // Purple (primary accent)
		AccentFg:  lipgloss.Color("#44475A"), // Dark text on accent
		AccentDim: lipgloss.Color("#44475A"), // Current Line / Selection
		Border:    lipgloss.Color("#44475A"), // Use selection color for subtle borders
		BorderDim: lipgloss.Color("#343746"), // Slightly darker for inactive
		MutedFg:   lipgloss.Color("#6272A4"), // Comment (muted text)
		TextFg:    lipgloss.Color("#F8F8F2"), // Foreground (primary text)
		SuccessFg: lipgloss.Color("#50FA7B"), // Green (success)
		WarnFg:    lipgloss.Color("#FFB86C"), // Orange (warning)
		ErrorFg:   lipgloss.Color("#FF5555"), // Red (error)
		Cyan:      lipgloss.Color("#8BE9FD"), // Cyan (info/secondary)
	}
}

// DraculaLight returns the Dracula theme adapted for light backgrounds.
func DraculaLight() *Theme {
	return &Theme{
		Accent:    lipgloss.Color("#c6dbe5"), // Purple (darker for light bg)
		AccentFg:  lipgloss.Color("#24292F"), // Dark text on accent
		AccentDim: lipgloss.Color("#F3E8FF"), // Light purple wash
		Border:    lipgloss.Color("#D0D7DE"), // Subtle gray border
		BorderDim: lipgloss.Color("#E8E8E8"), // Lighter border
		MutedFg:   lipgloss.Color("#6E7781"), // Muted gray text
		TextFg:    lipgloss.Color("#24292F"), // Dark text
		SuccessFg: lipgloss.Color("#059669"), // Green
		WarnFg:    lipgloss.Color("#D97706"), // Orange
		ErrorFg:   lipgloss.Color("#DC2626"), // Red
		Cyan:      lipgloss.Color("#0891B2"), // Cyan/Teal
	}
}

// Narna returns a balanced dark theme with blue accents.
func Narna() *Theme {
	return &Theme{
		Accent:    lipgloss.Color("#41ADFF"), // Blue accent
		AccentFg:  lipgloss.Color("#0D1117"), // Dark text on accent
		AccentDim: lipgloss.Color("#1A2230"), // Selected rows / panels
		Border:    lipgloss.Color("#30363D"), // Subtle borders
		BorderDim: lipgloss.Color("#20252D"), // Dim borders
		MutedFg:   lipgloss.Color("#8B949E"), // Muted text
		TextFg:    lipgloss.Color("#E6EDF3"), // Primary text
		SuccessFg: lipgloss.Color("#3FB950"), // Success green
		WarnFg:    lipgloss.Color("#E3B341"), // Warning amber
		ErrorFg:   lipgloss.Color("#F47067"), // Soft red
		Cyan:      lipgloss.Color("#7CE0F3"), // Cyan highlights
	}
}

// CleanLight returns a theme optimized for light terminal backgrounds.
func CleanLight() *Theme {
	return &Theme{
		Accent:    lipgloss.Color("#c6dbe5"), // Cyan (matching header)
		AccentFg:  lipgloss.Color("#24292F"), // Dark text on accent
		AccentDim: lipgloss.Color("#DDF4FF"), // Very light blue wash
		Border:    lipgloss.Color("#D0D7DE"), // Subtle cool gray
		BorderDim: lipgloss.Color("#E1E4E8"), // Very subtle divider
		MutedFg:   lipgloss.Color("#6E7781"), // Muted gray text
		TextFg:    lipgloss.Color("#24292F"), // Deep charcoal (softer than black)
		SuccessFg: lipgloss.Color("#1A7F37"), // Success green
		WarnFg:    lipgloss.Color("#9A6700"), // Warning brown/orange
		ErrorFg:   lipgloss.Color("#CF222E"), // Error red
		Cyan:      lipgloss.Color("#0598BC"), // Cyan
	}
}

// CatppuccinLatte returns the Catppuccin Latte theme (Light).
func CatppuccinLatte() *Theme {
	return &Theme{
		Accent:    lipgloss.Color("#1E66F5"), // Blue
		AccentFg:  lipgloss.Color("#FFFFFF"), // White text on accent
		AccentDim: lipgloss.Color("#CCD0DA"), // Surface0
		Border:    lipgloss.Color("#9CA0B0"), // Overlay0
		BorderDim: lipgloss.Color("#BCC0CC"), // Surface1
		MutedFg:   lipgloss.Color("#6C6F85"), // Subtext0
		TextFg:    lipgloss.Color("#4C4F69"), // Text
		SuccessFg: lipgloss.Color("#40A02B"), // Green
		WarnFg:    lipgloss.Color("#DF8E1D"), // Yellow
		ErrorFg:   lipgloss.Color("#D20F39"), // Red
		Cyan:      lipgloss.Color("#04A5E5"), // Sky
	}
}

// RosePineDawn returns the Rosé Pine Dawn theme (Light).
func RosePineDawn() *Theme {
	return &Theme{
		Accent:    lipgloss.Color("#286983"), // Pine
		AccentFg:  lipgloss.Color("#FFFFFF"), // White text on accent
		AccentDim: lipgloss.Color("#DFDAD9"), // Highlight
		Border:    lipgloss.Color("#CECACD"), // Muted (approx)
		BorderDim: lipgloss.Color("#F2E9E1"), // Surface
		MutedFg:   lipgloss.Color("#9893A5"), // Muted
		TextFg:    lipgloss.Color("#575279"), // Text
		SuccessFg: lipgloss.Color("#56949F"), // Foam (used as success/info often)
		WarnFg:    lipgloss.Color("#EA9D34"), // Gold
		ErrorFg:   lipgloss.Color("#B4637A"), // Love
		Cyan:      lipgloss.Color("#907AA9"), // Iris
	}
}

// OneLight returns the Atom One Light theme.
func OneLight() *Theme {
	return &Theme{
		Accent:    lipgloss.Color("#528BFF"), // Blue
		AccentFg:  lipgloss.Color("#FFFFFF"), // White text on accent
		AccentDim: lipgloss.Color("#E5E5E6"), // Light Gray
		Border:    lipgloss.Color("#A0A1A7"), // Muted Gray
		BorderDim: lipgloss.Color("#DBDBDC"), // Light Border
		MutedFg:   lipgloss.Color("#A0A1A7"), // Comments
		TextFg:    lipgloss.Color("#383A42"), // Foreground
		SuccessFg: lipgloss.Color("#50A14F"), // Green
		WarnFg:    lipgloss.Color("#C18401"), // Orange/Gold
		ErrorFg:   lipgloss.Color("#E45649"), // Red
		Cyan:      lipgloss.Color("#0184BC"), // Cyan
	}
}

// EverforestLight returns the Everforest Light theme (Medium).
func EverforestLight() *Theme {
	return &Theme{
		Accent:    lipgloss.Color("#3A94C5"), // Blue
		AccentFg:  lipgloss.Color("#FFFFFF"), // White text on accent
		AccentDim: lipgloss.Color("#EAE4CA"), // Lighter background
		Border:    lipgloss.Color("#C5C1A5"), // Border
		BorderDim: lipgloss.Color("#E0DCC7"), // Light Border
		MutedFg:   lipgloss.Color("#939F91"), // Grey
		TextFg:    lipgloss.Color("#5C6A72"), // Foreground
		SuccessFg: lipgloss.Color("#8DA101"), // Green
		WarnFg:    lipgloss.Color("#DFA000"), // Yellow
		ErrorFg:   lipgloss.Color("#F85552"), // Red
		Cyan:      lipgloss.Color("#3A94C5"), // Blue
	}
}

// EverforestDark returns the Everforest Dark theme (Medium).
func EverforestDark() *Theme {
	return &Theme{
		Accent:    lipgloss.Color("#A7C080"), // Green
		AccentFg:  lipgloss.Color("#2D353B"), // Dark text on accent
		AccentDim: lipgloss.Color("#3D484D"), // Selection
		Border:    lipgloss.Color("#3D484D"), // Border
		BorderDim: lipgloss.Color("#343F44"), // Darker
		MutedFg:   lipgloss.Color("#859289"), // Grey
		TextFg:    lipgloss.Color("#D3C6AA"), // Foreground
		SuccessFg: lipgloss.Color("#A7C080"), // Green
		WarnFg:    lipgloss.Color("#DBBC7F"), // Yellow
		ErrorFg:   lipgloss.Color("#E67E80"), // Red
		Cyan:      lipgloss.Color("#7FBBB3"), // Blue
	}
}

// TokyoNight returns the Tokyo Night theme (Storm).
func TokyoNight() *Theme {
	return &Theme{
		Accent:    lipgloss.Color("#7AA2F7"), // Blue
		AccentFg:  lipgloss.Color("#1A1B26"), // Dark text on accent
		AccentDim: lipgloss.Color("#2F3549"), // Selection
		Border:    lipgloss.Color("#363D59"), // Border
		BorderDim: lipgloss.Color("#2F3549"), // Selection
		MutedFg:   lipgloss.Color("#565F89"), // Comments
		TextFg:    lipgloss.Color("#C0CAF5"), // Foreground
		SuccessFg: lipgloss.Color("#9ECE6A"), // Green
		WarnFg:    lipgloss.Color("#E0AF68"), // Orange
		ErrorFg:   lipgloss.Color("#F7768E"), // Red
		Cyan:      lipgloss.Color("#7DCFFF"), // Cyan
	}
}

// OneDark returns the One Dark theme.
func OneDark() *Theme {
	return &Theme{
		Accent:    lipgloss.Color("#61AFEF"), // Blue
		AccentFg:  lipgloss.Color("#282C34"), // Dark text on accent
		AccentDim: lipgloss.Color("#3E4452"), // Selection
		Border:    lipgloss.Color("#3E4452"), // Border
		BorderDim: lipgloss.Color("#353B45"), // Darker
		MutedFg:   lipgloss.Color("#5C6370"), // Comments
		TextFg:    lipgloss.Color("#ABB2BF"), // Foreground
		SuccessFg: lipgloss.Color("#98C379"), // Green
		WarnFg:    lipgloss.Color("#D19A66"), // Orange
		ErrorFg:   lipgloss.Color("#E06C75"), // Red
		Cyan:      lipgloss.Color("#56B6C2"), // Cyan
	}
}

// RosePine returns the Rosé Pine theme (Dark).
func RosePine() *Theme {
	return &Theme{
		Accent:    lipgloss.Color("#C4A7E7"), // Iris
		AccentFg:  lipgloss.Color("#191724"), // Dark text on accent
		AccentDim: lipgloss.Color("#26233A"), // Selection
		Border:    lipgloss.Color("#403D52"), // Border
		BorderDim: lipgloss.Color("#26233A"), // Selection
		MutedFg:   lipgloss.Color("#6E6A86"), // Muted
		TextFg:    lipgloss.Color("#E0DEF4"), // Foreground
		SuccessFg: lipgloss.Color("#9CCFD8"), // Foam
		WarnFg:    lipgloss.Color("#F6C177"), // Gold
		ErrorFg:   lipgloss.Color("#EB6F92"), // Love
		Cyan:      lipgloss.Color("#31748F"), // Pine
	}
}

// AyuMirage returns the Ayu Mirage theme.
func AyuMirage() *Theme {
	return &Theme{
		Accent:    lipgloss.Color("#FFCC66"), // Orange
		AccentFg:  lipgloss.Color("#212733"), // Dark text on accent
		AccentDim: lipgloss.Color("#2D333F"), // Selection (slightly lighter than bg)
		Border:    lipgloss.Color("#3E4B59"), // Border
		BorderDim: lipgloss.Color("#2D333F"), // Selection
		MutedFg:   lipgloss.Color("#5C6773"), // Comments
		TextFg:    lipgloss.Color("#D9D7CE"), // Foreground
		SuccessFg: lipgloss.Color("#BAE67E"), // Green
		WarnFg:    lipgloss.Color("#FFAE57"), // Orange
		ErrorFg:   lipgloss.Color("#FF3333"), // Red
		Cyan:      lipgloss.Color("#5CCFE6"), // Cyan
	}
}

// Modern returns a sleek, modern dark theme with vibrant accents.
func Modern() *Theme {
	return &Theme{
		Accent:    lipgloss.Color("#8B5CF6"), // Violet 500
		AccentFg:  lipgloss.Color("#18181B"), // Zinc 900 (dark text on accent)
		AccentDim: lipgloss.Color("#27272A"), // Zinc 800
		Border:    lipgloss.Color("#3F3F46"), // Zinc 700
		BorderDim: lipgloss.Color("#27272A"), // Zinc 800
		MutedFg:   lipgloss.Color("#71717A"), // Zinc 500
		TextFg:    lipgloss.Color("#FAFAFA"), // Zinc 50
		SuccessFg: lipgloss.Color("#10B981"), // Emerald 500
		WarnFg:    lipgloss.Color("#F59E0B"), // Amber 500
		ErrorFg:   lipgloss.Color("#EF4444"), // Red 500
		Cyan:      lipgloss.Color("#06B6D4"), // Cyan 500
	}
}

// Kanagawa returns the Kanagawa theme (Wave).
func Kanagawa() *Theme {
	return &Theme{
		Accent:    lipgloss.Color("#7E9CD8"), // Crystal Blue
		AccentFg:  lipgloss.Color("#16161D"), // Summit Black
		AccentDim: lipgloss.Color("#2D4F67"), // Wave Blue 2
		Border:    lipgloss.Color("#727169"), // Fuji Gray
		BorderDim: lipgloss.Color("#223249"), // Wave Blue 1
		MutedFg:   lipgloss.Color("#727169"), // Fuji Gray
		TextFg:    lipgloss.Color("#DCD7BA"), // Fuji White
		SuccessFg: lipgloss.Color("#76946A"), // Autumn Green
		WarnFg:    lipgloss.Color("#C0A36E"), // Ronin Yellow
		ErrorFg:   lipgloss.Color("#C34043"), // Samurai Red
		Cyan:      lipgloss.Color("#7AA89F"), // Wave Aqua 1
	}
}

// SolarizedDark returns the Solarized dark theme.
func SolarizedDark() *Theme {
	return &Theme{
		Accent:    lipgloss.Color("#268BD2"),
		AccentFg:  lipgloss.Color("#FDF6E3"), // Light text on accent
		AccentDim: lipgloss.Color("#073642"),
		Border:    lipgloss.Color("#586E75"),
		BorderDim: lipgloss.Color("#073642"),
		MutedFg:   lipgloss.Color("#586E75"),
		TextFg:    lipgloss.Color("#93A1A1"), // base1
		SuccessFg: lipgloss.Color("#859900"),
		WarnFg:    lipgloss.Color("#B58900"),
		ErrorFg:   lipgloss.Color("#DC322F"),
		Cyan:      lipgloss.Color("#2AA198"),
	}
}

// SolarizedLight returns the Solarized light theme.
func SolarizedLight() *Theme {
	return &Theme{
		Accent:    lipgloss.Color("#268BD2"),
		AccentFg:  lipgloss.Color("#FDF6E3"), // Light text on accent
		AccentDim: lipgloss.Color("#EEE8D5"),
		Border:    lipgloss.Color("#93A1A1"),
		BorderDim: lipgloss.Color("#E4DDC7"),
		MutedFg:   lipgloss.Color("#93A1A1"),
		TextFg:    lipgloss.Color("#586E75"), // base01
		SuccessFg: lipgloss.Color("#859900"),
		WarnFg:    lipgloss.Color("#B58900"),
		ErrorFg:   lipgloss.Color("#DC322F"),
		Cyan:      lipgloss.Color("#2AA198"),
	}
}

// GruvboxDark returns the Gruvbox dark theme.
func GruvboxDark() *Theme {
	return &Theme{
		Accent:    lipgloss.Color("#FABD2F"),
		AccentFg:  lipgloss.Color("#282828"), // Dark text on yellow accent
		AccentDim: lipgloss.Color("#3C3836"),
		Border:    lipgloss.Color("#504945"),
		BorderDim: lipgloss.Color("#3C3836"),
		MutedFg:   lipgloss.Color("#928374"),
		TextFg:    lipgloss.Color("#EBDBB2"),
		SuccessFg: lipgloss.Color("#B8BB26"),
		WarnFg:    lipgloss.Color("#FABD2F"),
		ErrorFg:   lipgloss.Color("#FB4934"),
		Cyan:      lipgloss.Color("#83A598"),
	}
}

// GruvboxLight returns the Gruvbox light theme.
func GruvboxLight() *Theme {
	return &Theme{
		Accent:    lipgloss.Color("#D79921"),
		AccentFg:  lipgloss.Color("#FBF1C7"), // Light text on yellow accent
		AccentDim: lipgloss.Color("#E0CFA9"),
		Border:    lipgloss.Color("#D5C4A1"),
		BorderDim: lipgloss.Color("#C0B58A"),
		MutedFg:   lipgloss.Color("#7C6F64"),
		TextFg:    lipgloss.Color("#3C3836"),
		SuccessFg: lipgloss.Color("#79740E"),
		WarnFg:    lipgloss.Color("#D79921"),
		ErrorFg:   lipgloss.Color("#9D0006"),
		Cyan:      lipgloss.Color("#427B58"),
	}
}

// Nord returns the Nord theme.
func Nord() *Theme {
	return &Theme{
		Accent:    lipgloss.Color("#88C0D0"),
		AccentFg:  lipgloss.Color("#2E3440"), // Dark text on accent
		AccentDim: lipgloss.Color("#3B4252"),
		Border:    lipgloss.Color("#4C566A"),
		BorderDim: lipgloss.Color("#434C5E"),
		MutedFg:   lipgloss.Color("#81A1C1"),
		TextFg:    lipgloss.Color("#E5E9F0"),
		SuccessFg: lipgloss.Color("#A3BE8C"),
		WarnFg:    lipgloss.Color("#EBCB8B"),
		ErrorFg:   lipgloss.Color("#BF616A"),
		Cyan:      lipgloss.Color("#88C0D0"),
	}
}

// Monokai returns the Monokai theme.
func Monokai() *Theme {
	return &Theme{
		Accent:    lipgloss.Color("#A6E22E"),
		AccentFg:  lipgloss.Color("#272822"), // Dark text on green accent
		AccentDim: lipgloss.Color("#3E3D32"),
		Border:    lipgloss.Color("#75715E"),
		BorderDim: lipgloss.Color("#3E3D32"),
		MutedFg:   lipgloss.Color("#75715E"),
		TextFg:    lipgloss.Color("#F8F8F2"),
		SuccessFg: lipgloss.Color("#A6E22E"),
		WarnFg:    lipgloss.Color("#FD971F"),
		ErrorFg:   lipgloss.Color("#F92672"),
		Cyan:      lipgloss.Color("#66D9EF"),
	}
}

// CatppuccinMocha returns the Catppuccin Mocha theme.
func CatppuccinMocha() *Theme {
	return &Theme{
		Accent:    lipgloss.Color("#B4BEFE"),
		AccentFg:  lipgloss.Color("#1E1E2E"), // Dark text on accent
		AccentDim: lipgloss.Color("#313244"),
		Border:    lipgloss.Color("#45475A"),
		BorderDim: lipgloss.Color("#313244"),
		MutedFg:   lipgloss.Color("#6C7086"),
		TextFg:    lipgloss.Color("#CDD6F4"),
		SuccessFg: lipgloss.Color("#A6E3A1"),
		WarnFg:    lipgloss.Color("#F9E2AF"),
		ErrorFg:   lipgloss.Color("#F38BA8"),
		Cyan:      lipgloss.Color("#89DCEB"),
	}
}

// GetTheme returns a theme by name, or Dracula if not found.
func GetTheme(name string) *Theme {
	switch name {
	case DraculaLightName:
		return DraculaLight()
	case NarnaName:
		return Narna()
	case CleanLightName:
		return CleanLight()
	case CatppuccinLatteName:
		return CatppuccinLatte()
	case RosePineDawnName:
		return RosePineDawn()
	case OneLightName:
		return OneLight()
	case EverforestLightName:
		return EverforestLight()
	case EverforestDarkName:
		return EverforestDark()
	case SolarizedDarkName:
		return SolarizedDark()
	case SolarizedLightName:
		return SolarizedLight()
	case GruvboxDarkName:
		return GruvboxDark()
	case GruvboxLightName:
		return GruvboxLight()
	case NordName:
		return Nord()
	case MonokaiName:
		return Monokai()
	case CatppuccinMochaName:
		return CatppuccinMocha()
	case ModernName:
		return Modern()
	case TokyoNightName:
		return TokyoNight()
	case OneDarkName:
		return OneDark()
	case RosePineName:
		return RosePine()
	case AyuMirageName:
		return AyuMirage()
	case KanagawaName:
		return Kanagawa()
	default:
		return Dracula()
	}
}

// DefaultDark returns the default dark theme name.
func DefaultDark() string {
	return RosePineName
}

// DefaultLight returns the default light theme name.
func DefaultLight() string {
	return DraculaLightName
}

// AvailableThemes returns a list of available theme names.
func AvailableThemes() []string {
	return []string{
		DraculaName,
		DraculaLightName,
		NarnaName,
		CleanLightName,
		CatppuccinLatteName,
		RosePineDawnName,
		OneLightName,
		EverforestLightName,
		EverforestDarkName,
		SolarizedDarkName,
		SolarizedLightName,
		GruvboxDarkName,
		GruvboxLightName,
		NordName,
		MonokaiName,
		CatppuccinMochaName,
		ModernName,
		TokyoNightName,
		OneDarkName,
		RosePineName,
		AyuMirageName,
		KanagawaName,
	}
}

// AvailableThemesWithCustoms returns a list of available theme names including custom themes.
func AvailableThemesWithCustoms(customThemes map[string]*CustomThemeData) []string {
	themes := AvailableThemes()
	for name := range customThemes {
		themes = append(themes, name)
	}
	return themes
}

// GetThemeWithCustoms returns a theme by name, checking built-in themes first, then custom themes.
// It handles inheritance recursively and merges base themes with overrides.
func GetThemeWithCustoms(name string, customThemes map[string]*CustomThemeData) *Theme {
	if name == "" {
		return Dracula()
	}

	nameLower := strings.ToLower(name)

	// First check built-in themes
	if isBuiltInTheme(nameLower) {
		return GetTheme(nameLower)
	}

	// Check custom themes
	if customThemes != nil {
		if custom, ok := customThemes[name]; ok {
			return resolveCustomTheme(custom, customThemes, make(map[string]bool))
		}
		// Also try case-insensitive lookup
		for customName, custom := range customThemes {
			if strings.EqualFold(customName, nameLower) {
				return resolveCustomTheme(custom, customThemes, make(map[string]bool))
			}
		}
	}

	// Fallback to Dracula
	return Dracula()
}

// resolveCustomTheme recursively resolves a custom theme, handling inheritance.
func resolveCustomTheme(custom *CustomThemeData, customThemes map[string]*CustomThemeData, visited map[string]bool) *Theme {
	// If no base, create theme from scratch
	if custom.Base == "" {
		return themeFromCustom(custom)
	}

	baseName := strings.ToLower(strings.TrimSpace(custom.Base))

	// Check for circular dependency
	if visited[baseName] {
		return Dracula() // Fallback on circular dependency
	}

	// Try to get base theme from built-in themes first
	baseTheme := GetTheme(baseName)
	if baseTheme == nil || !isBuiltInTheme(baseName) {
		// Check if it's a custom theme
		if baseCustom, ok := customThemes[baseName]; ok {
			visited[baseName] = true
			baseTheme = resolveCustomTheme(baseCustom, customThemes, visited)
		} else {
			// Base doesn't exist, fallback
			return Dracula()
		}
	}

	// Merge base with custom overrides
	return MergeTheme(baseTheme, custom)
}

// isBuiltInTheme checks if a theme name is a built-in theme.
func isBuiltInTheme(name string) bool {
	builtInThemes := AvailableThemes()
	for _, builtIn := range builtInThemes {
		if strings.EqualFold(builtIn, name) {
			return true
		}
	}
	return false
}

// MergeTheme merges a base theme with custom theme overrides.
func MergeTheme(base *Theme, custom *CustomThemeData) *Theme {
	merged := &Theme{
		Accent:    base.Accent,
		AccentFg:  base.AccentFg,
		AccentDim: base.AccentDim,
		Border:    base.Border,
		BorderDim: base.BorderDim,
		MutedFg:   base.MutedFg,
		TextFg:    base.TextFg,
		SuccessFg: base.SuccessFg,
		WarnFg:    base.WarnFg,
		ErrorFg:   base.ErrorFg,
		Cyan:      base.Cyan,
	}

	// Apply overrides from custom theme
	if custom.Accent != "" {
		merged.Accent = lipgloss.Color(custom.Accent)
	}
	if custom.AccentFg != "" {
		merged.AccentFg = lipgloss.Color(custom.AccentFg)
	}
	if custom.AccentDim != "" {
		merged.AccentDim = lipgloss.Color(custom.AccentDim)
	}
	if custom.Border != "" {
		merged.Border = lipgloss.Color(custom.Border)
	}
	if custom.BorderDim != "" {
		merged.BorderDim = lipgloss.Color(custom.BorderDim)
	}
	if custom.MutedFg != "" {
		merged.MutedFg = lipgloss.Color(custom.MutedFg)
	}
	if custom.TextFg != "" {
		merged.TextFg = lipgloss.Color(custom.TextFg)
	}
	if custom.SuccessFg != "" {
		merged.SuccessFg = lipgloss.Color(custom.SuccessFg)
	}
	if custom.WarnFg != "" {
		merged.WarnFg = lipgloss.Color(custom.WarnFg)
	}
	if custom.ErrorFg != "" {
		merged.ErrorFg = lipgloss.Color(custom.ErrorFg)
	}
	if custom.Cyan != "" {
		merged.Cyan = lipgloss.Color(custom.Cyan)
	}

	return merged
}

// themeFromCustom creates a Theme from a CustomThemeData without a base.
func themeFromCustom(custom *CustomThemeData) *Theme {
	return &Theme{
		Accent:    lipgloss.Color(custom.Accent),
		AccentFg:  lipgloss.Color(custom.AccentFg),
		AccentDim: lipgloss.Color(custom.AccentDim),
		Border:    lipgloss.Color(custom.Border),
		BorderDim: lipgloss.Color(custom.BorderDim),
		MutedFg:   lipgloss.Color(custom.MutedFg),
		TextFg:    lipgloss.Color(custom.TextFg),
		SuccessFg: lipgloss.Color(custom.SuccessFg),
		WarnFg:    lipgloss.Color(custom.WarnFg),
		ErrorFg:   lipgloss.Color(custom.ErrorFg),
		Cyan:      lipgloss.Color(custom.Cyan),
	}
}
