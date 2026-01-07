// Package theme provides theme definitions and management for the TUI.
package theme

import "github.com/charmbracelet/lipgloss"

// Theme defines all colors used in the application UI.
type Theme struct {
	Background lipgloss.Color
	Accent     lipgloss.Color
	AccentDim  lipgloss.Color
	Border     lipgloss.Color
	BorderDim  lipgloss.Color
	MutedFg    lipgloss.Color
	TextFg     lipgloss.Color
	SuccessFg  lipgloss.Color
	WarnFg     lipgloss.Color
	ErrorFg    lipgloss.Color
	Cyan       lipgloss.Color
	Pink       lipgloss.Color
	Yellow     lipgloss.Color
}

// Theme names.
const (
	DraculaName         = "dracula"
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
)

// Dracula returns the Dracula theme (dark background, vibrant colors).
func Dracula() *Theme {
	return &Theme{
		Background: lipgloss.Color("#282A36"), // Background
		Accent:     lipgloss.Color("#BD93F9"), // Purple (primary accent)
		AccentDim:  lipgloss.Color("#44475A"), // Current Line / Selection
		Border:     lipgloss.Color("#6272A4"), // Comment (subtle borders)
		BorderDim:  lipgloss.Color("#44475A"), // Darker borders
		MutedFg:    lipgloss.Color("#6272A4"), // Comment (muted text)
		TextFg:     lipgloss.Color("#F8F8F2"), // Foreground (primary text)
		SuccessFg:  lipgloss.Color("#50FA7B"), // Green (success)
		WarnFg:     lipgloss.Color("#FFB86C"), // Orange (warning)
		ErrorFg:    lipgloss.Color("#FF5555"), // Red (error)
		Cyan:       lipgloss.Color("#8BE9FD"), // Cyan (info/secondary)
		Pink:       lipgloss.Color("#FF79C6"), // Pink (alternative accent)
		Yellow:     lipgloss.Color("#F1FA8C"), // Yellow (alternative highlight)
	}
}

// Narna returns a balanced dark theme with blue accents.
func Narna() *Theme {
	return &Theme{
		Background: lipgloss.Color("#0D1117"), // Charcoal background
		Accent:     lipgloss.Color("#41ADFF"), // Blue accent
		AccentDim:  lipgloss.Color("#1A2230"), // Selected rows / panels
		Border:     lipgloss.Color("#30363D"), // Subtle borders
		BorderDim:  lipgloss.Color("#20252D"), // Dim borders
		MutedFg:    lipgloss.Color("#8B949E"), // Muted text
		TextFg:     lipgloss.Color("#E6EDF3"), // Primary text
		SuccessFg:  lipgloss.Color("#3FB950"), // Success green
		WarnFg:     lipgloss.Color("#E3B341"), // Warning amber
		ErrorFg:    lipgloss.Color("#F47067"), // Soft red
		Cyan:       lipgloss.Color("#7CE0F3"), // Cyan highlights
		Pink:       lipgloss.Color("#D2A8FF"), // Accent purple/pink
		Yellow:     lipgloss.Color("#F2CC60"), // Highlight yellow
	}
}

// CleanLight returns a theme optimized for light terminal backgrounds.
func CleanLight() *Theme {
	return &Theme{
		Background: lipgloss.Color("#FFFFFF"), // Pure White
		Accent:     lipgloss.Color("#0969DA"), // Professional Blue
		AccentDim:  lipgloss.Color("#DDF4FF"), // Very light blue wash
		Border:     lipgloss.Color("#D0D7DE"), // Subtle cool gray
		BorderDim:  lipgloss.Color("#E1E4E8"), // Very subtle divider
		MutedFg:    lipgloss.Color("#6E7781"), // Muted gray text
		TextFg:     lipgloss.Color("#24292F"), // Deep charcoal (softer than black)
		SuccessFg:  lipgloss.Color("#1A7F37"), // Success green
		WarnFg:     lipgloss.Color("#9A6700"), // Warning brown/orange
		ErrorFg:    lipgloss.Color("#CF222E"), // Error red
		Cyan:       lipgloss.Color("#0598BC"), // Cyan
		Pink:       lipgloss.Color("#BF3989"), // Pink
		Yellow:     lipgloss.Color("#D4A72C"), // Yellow
	}
}

// CatppuccinLatte returns the Catppuccin Latte theme (Light).
func CatppuccinLatte() *Theme {
	return &Theme{
		Background: lipgloss.Color("#EFF1F5"),
		Accent:     lipgloss.Color("#1E66F5"), // Blue
		AccentDim:  lipgloss.Color("#CCD0DA"), // Surface0
		Border:     lipgloss.Color("#9CA0B0"), // Overlay0
		BorderDim:  lipgloss.Color("#BCC0CC"), // Surface1
		MutedFg:    lipgloss.Color("#6C6F85"), // Subtext0
		TextFg:     lipgloss.Color("#4C4F69"), // Text
		SuccessFg:  lipgloss.Color("#40A02B"), // Green
		WarnFg:     lipgloss.Color("#DF8E1D"), // Yellow
		ErrorFg:    lipgloss.Color("#D20F39"), // Red
		Cyan:       lipgloss.Color("#04A5E5"), // Sky
		Pink:       lipgloss.Color("#EA76CB"), // Pink
		Yellow:     lipgloss.Color("#DF8E1D"), // Yellow
	}
}

// RosePineDawn returns the Rosé Pine Dawn theme (Light).
func RosePineDawn() *Theme {
	return &Theme{
		Background: lipgloss.Color("#FAF4ED"),
		Accent:     lipgloss.Color("#286983"), // Pine
		AccentDim:  lipgloss.Color("#DFDAD9"), // Highlight
		Border:     lipgloss.Color("#CECACD"), // Muted (approx)
		BorderDim:  lipgloss.Color("#F2E9E1"), // Surface
		MutedFg:    lipgloss.Color("#9893A5"), // Muted
		TextFg:     lipgloss.Color("#575279"), // Text
		SuccessFg:  lipgloss.Color("#56949F"), // Foam (used as success/info often)
		WarnFg:     lipgloss.Color("#EA9D34"), // Gold
		ErrorFg:    lipgloss.Color("#B4637A"), // Love
		Cyan:       lipgloss.Color("#907AA9"), // Iris
		Pink:       lipgloss.Color("#B4637A"), // Love
		Yellow:     lipgloss.Color("#EA9D34"), // Gold
	}
}

// OneLight returns the Atom One Light theme.
func OneLight() *Theme {
	return &Theme{
		Background: lipgloss.Color("#FAFAFA"),
		Accent:     lipgloss.Color("#528BFF"), // Blue
		AccentDim:  lipgloss.Color("#E5E5E6"), // Light Gray
		Border:     lipgloss.Color("#A0A1A7"), // Muted Gray
		BorderDim:  lipgloss.Color("#DBDBDC"), // Light Border
		MutedFg:    lipgloss.Color("#A0A1A7"), // Comments
		TextFg:     lipgloss.Color("#383A42"), // Foreground
		SuccessFg:  lipgloss.Color("#50A14F"), // Green
		WarnFg:     lipgloss.Color("#C18401"), // Orange/Gold
		ErrorFg:    lipgloss.Color("#E45649"), // Red
		Cyan:       lipgloss.Color("#0184BC"), // Cyan
		Pink:       lipgloss.Color("#A626A4"), // Purple
		Yellow:     lipgloss.Color("#986801"), // Yellow
	}
}

// EverforestLight returns the Everforest Light theme (Medium).
func EverforestLight() *Theme {
	return &Theme{
		Background: lipgloss.Color("#F3EFDA"),
		Accent:     lipgloss.Color("#3A94C5"), // Blue
		AccentDim:  lipgloss.Color("#EAE4CA"), // Lighter background
		Border:     lipgloss.Color("#C5C1A5"), // Border
		BorderDim:  lipgloss.Color("#E0DCC7"), // Light Border
		MutedFg:    lipgloss.Color("#939F91"), // Grey
		TextFg:     lipgloss.Color("#5C6A72"), // Foreground
		SuccessFg:  lipgloss.Color("#8DA101"), // Green
		WarnFg:     lipgloss.Color("#DFA000"), // Yellow
		ErrorFg:    lipgloss.Color("#F85552"), // Red
		Cyan:       lipgloss.Color("#3A94C5"), // Blue
		Pink:       lipgloss.Color("#D3869B"), // Purple
		Yellow:     lipgloss.Color("#DFA000"), // Yellow
	}
}

// SolarizedDark returns the Solarized dark theme.
func SolarizedDark() *Theme {
	return &Theme{
		Background: lipgloss.Color("#002B36"),
		Accent:     lipgloss.Color("#268BD2"),
		AccentDim:  lipgloss.Color("#073642"),
		Border:     lipgloss.Color("#586E75"),
		BorderDim:  lipgloss.Color("#073642"),
		MutedFg:    lipgloss.Color("#586E75"),
		TextFg:     lipgloss.Color("#EEE8D5"),
		SuccessFg:  lipgloss.Color("#859900"),
		WarnFg:     lipgloss.Color("#B58900"),
		ErrorFg:    lipgloss.Color("#DC322F"),
		Cyan:       lipgloss.Color("#2AA198"),
		Pink:       lipgloss.Color("#D33682"),
		Yellow:     lipgloss.Color("#B58900"),
	}
}

// SolarizedLight returns the Solarized light theme.
func SolarizedLight() *Theme {
	return &Theme{
		Background: lipgloss.Color("#FDF6E3"),
		Accent:     lipgloss.Color("#268BD2"),
		AccentDim:  lipgloss.Color("#EEE8D5"),
		Border:     lipgloss.Color("#93A1A1"),
		BorderDim:  lipgloss.Color("#E4DDC7"),
		MutedFg:    lipgloss.Color("#93A1A1"),
		TextFg:     lipgloss.Color("#073642"),
		SuccessFg:  lipgloss.Color("#859900"),
		WarnFg:     lipgloss.Color("#B58900"),
		ErrorFg:    lipgloss.Color("#DC322F"),
		Cyan:       lipgloss.Color("#2AA198"),
		Pink:       lipgloss.Color("#D33682"),
		Yellow:     lipgloss.Color("#B58900"),
	}
}

// GruvboxDark returns the Gruvbox dark theme.
func GruvboxDark() *Theme {
	return &Theme{
		Background: lipgloss.Color("#282828"),
		Accent:     lipgloss.Color("#FABD2F"),
		AccentDim:  lipgloss.Color("#3C3836"),
		Border:     lipgloss.Color("#504945"),
		BorderDim:  lipgloss.Color("#3C3836"),
		MutedFg:    lipgloss.Color("#928374"),
		TextFg:     lipgloss.Color("#EBDBB2"),
		SuccessFg:  lipgloss.Color("#B8BB26"),
		WarnFg:     lipgloss.Color("#FABD2F"),
		ErrorFg:    lipgloss.Color("#FB4934"),
		Cyan:       lipgloss.Color("#83A598"),
		Pink:       lipgloss.Color("#D3869B"),
		Yellow:     lipgloss.Color("#FABD2F"),
	}
}

// GruvboxLight returns the Gruvbox light theme.
func GruvboxLight() *Theme {
	return &Theme{
		Background: lipgloss.Color("#FBF1C7"),
		Accent:     lipgloss.Color("#D79921"),
		AccentDim:  lipgloss.Color("#E0CFA9"),
		Border:     lipgloss.Color("#D5C4A1"),
		BorderDim:  lipgloss.Color("#C0B58A"),
		MutedFg:    lipgloss.Color("#7C6F64"),
		TextFg:     lipgloss.Color("#3C3836"),
		SuccessFg:  lipgloss.Color("#79740E"),
		WarnFg:     lipgloss.Color("#D79921"),
		ErrorFg:    lipgloss.Color("#9D0006"),
		Cyan:       lipgloss.Color("#427B58"),
		Pink:       lipgloss.Color("#B16286"),
		Yellow:     lipgloss.Color("#D79921"),
	}
}

// Nord returns the Nord theme.
func Nord() *Theme {
	return &Theme{
		Background: lipgloss.Color("#2E3440"),
		Accent:     lipgloss.Color("#88C0D0"),
		AccentDim:  lipgloss.Color("#3B4252"),
		Border:     lipgloss.Color("#4C566A"),
		BorderDim:  lipgloss.Color("#434C5E"),
		MutedFg:    lipgloss.Color("#81A1C1"),
		TextFg:     lipgloss.Color("#E5E9F0"),
		SuccessFg:  lipgloss.Color("#A3BE8C"),
		WarnFg:     lipgloss.Color("#EBCB8B"),
		ErrorFg:    lipgloss.Color("#BF616A"),
		Cyan:       lipgloss.Color("#88C0D0"),
		Pink:       lipgloss.Color("#B48EAD"),
		Yellow:     lipgloss.Color("#EBCB8B"),
	}
}

// Monokai returns the Monokai theme.
func Monokai() *Theme {
	return &Theme{
		Background: lipgloss.Color("#272822"),
		Accent:     lipgloss.Color("#A6E22E"),
		AccentDim:  lipgloss.Color("#3E3D32"),
		Border:     lipgloss.Color("#75715E"),
		BorderDim:  lipgloss.Color("#3E3D32"),
		MutedFg:    lipgloss.Color("#75715E"),
		TextFg:     lipgloss.Color("#F8F8F2"),
		SuccessFg:  lipgloss.Color("#A6E22E"),
		WarnFg:     lipgloss.Color("#FD971F"),
		ErrorFg:    lipgloss.Color("#F92672"),
		Cyan:       lipgloss.Color("#66D9EF"),
		Pink:       lipgloss.Color("#F92672"),
		Yellow:     lipgloss.Color("#E6DB74"),
	}
}

// CatppuccinMocha returns the Catppuccin Mocha theme.
func CatppuccinMocha() *Theme {
	return &Theme{
		Background: lipgloss.Color("#1E1E2E"),
		Accent:     lipgloss.Color("#B4BEFE"),
		AccentDim:  lipgloss.Color("#313244"),
		Border:     lipgloss.Color("#45475A"),
		BorderDim:  lipgloss.Color("#313244"),
		MutedFg:    lipgloss.Color("#6C7086"),
		TextFg:     lipgloss.Color("#CDD6F4"),
		SuccessFg:  lipgloss.Color("#A6E3A1"),
		WarnFg:     lipgloss.Color("#F9E2AF"),
		ErrorFg:    lipgloss.Color("#F38BA8"),
		Cyan:       lipgloss.Color("#89DCEB"),
		Pink:       lipgloss.Color("#F5C2E7"),
		Yellow:     lipgloss.Color("#F9E2AF"),
	}
}

// GetTheme returns a theme by name, or Dracula if not found.
func GetTheme(name string) *Theme {
	switch name {
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
	default:
		return Dracula()
	}
}

// IsLight returns true if the theme is a light theme.
func IsLight(name string) bool {
	switch name {
	case CleanLightName, SolarizedLightName, GruvboxLightName, CatppuccinLatteName, RosePineDawnName, OneLightName, EverforestLightName:
		return true
	default:
		return false
	}
}

// DefaultDark returns the default dark theme name.
func DefaultDark() string {
	return DraculaName
}

// DefaultLight returns the default light theme name.
func DefaultLight() string {
	return CleanLightName
}

// AvailableThemes returns a list of available theme names.
func AvailableThemes() []string {
	return []string{
		DraculaName,
		NarnaName,
		CleanLightName,
		CatppuccinLatteName,
		RosePineDawnName,
		OneLightName,
		EverforestLightName,
		SolarizedDarkName,
		SolarizedLightName,
		GruvboxDarkName,
		GruvboxLightName,
		NordName,
		MonokaiName,
		CatppuccinMochaName,
	}
}
