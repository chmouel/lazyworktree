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
		Background: lipgloss.Color("#FFFFFF"), // White
		Accent:     lipgloss.Color("#3F7F9B"), // Toned-down cyan blue
		AccentDim:  lipgloss.Color("#EEF4F7"), // Very light blue-gray
		Border:     lipgloss.Color("#D3DEE6"), // Cool light gray with blue hint
		BorderDim:  lipgloss.Color("#E7EFF4"), // Subtle blue-tinted divider
		MutedFg:    lipgloss.Color("#556B78"), // Soft blue-gray text
		TextFg:     lipgloss.Color("#000000"), // Black
		SuccessFg:  lipgloss.Color("#1A7F37"), // Natural green
		WarnFg:     lipgloss.Color("#C2410C"), // Muted orange
		ErrorFg:    lipgloss.Color("#B91C1C"), // Muted red
		Cyan:       lipgloss.Color("#2F8FA3"), // Clear but restrained cyan
		Pink:       lipgloss.Color("#C0266D"), // Controlled pink
		Yellow:     lipgloss.Color("#B45309"), // Warm amber
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
	case "narna":
		return Narna()
	case "clean-light":
		return CleanLight()
	case "solarized-dark":
		return SolarizedDark()
	case "solarized-light":
		return SolarizedLight()
	case "gruvbox-dark":
		return GruvboxDark()
	case "gruvbox-light":
		return GruvboxLight()
	case "nord":
		return Nord()
	case "monokai":
		return Monokai()
	case "catppuccin-mocha":
		return CatppuccinMocha()
	default:
		return Dracula()
	}
}

// AvailableThemes returns a list of available theme names.
func AvailableThemes() []string {
	return []string{
		"dracula",
		"narna",
		"clean-light",
		"solarized-dark",
		"solarized-light",
		"gruvbox-dark",
		"gruvbox-light",
		"nord",
		"monokai",
		"catppuccin-mocha",
	}
}
