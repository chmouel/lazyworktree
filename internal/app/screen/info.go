package screen

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/chmouel/lazyworktree/internal/theme"
)

// InfoScreen displays a modal message with an OK button.
type InfoScreen struct {
	Message string
	Thm     *theme.Theme

	// Callback
	OnClose func() tea.Cmd
}

// NewInfoScreen creates an informational modal with an OK button.
func NewInfoScreen(message string, thm *theme.Theme) *InfoScreen {
	return &InfoScreen{
		Message: message,
		Thm:     thm,
	}
}

// Type returns the screen type.
func (s *InfoScreen) Type() Type {
	return TypeInfo
}

// Update processes keyboard events for the info dialog.
// Returns nil to signal that the screen should be closed.
func (s *InfoScreen) Update(msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch msg.String() {
	case keyEnter, keyEsc, keyQ, keyCtrlC:
		if s.OnClose != nil {
			return nil, s.OnClose()
		}
		return nil, nil
	}
	return s, nil
}

// View renders the informational UI box with a single OK button.
func (s *InfoScreen) View() string {
	width := 60
	height := 11

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.Thm.Accent).
		Padding(1, 2).
		Width(width).
		Height(height)

	messageStyle := lipgloss.NewStyle().
		Width(width-4).
		Height(height-6).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(s.Thm.TextFg)

	okStyle := lipgloss.NewStyle().
		Width(width-6).
		Align(lipgloss.Center).
		Padding(0, 2).
		Foreground(s.Thm.AccentFg).
		Background(s.Thm.Accent).
		Bold(true)

	content := fmt.Sprintf("%s\n\n%s",
		messageStyle.Render(s.Message),
		okStyle.Render("[OK]"),
	)

	return boxStyle.Render(content)
}

// SetTheme updates the theme for this screen.
func (s *InfoScreen) SetTheme(thm *theme.Theme) {
	s.Thm = thm
}
