package screen

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/chmouel/lazyworktree/internal/theme"
)

// Key constants for navigation.
const (
	keyEnter    = "enter"
	keyEsc      = "esc"
	keyEscRaw   = "\x1b" // Raw escape byte for terminals that send ESC as a rune
	keyTab      = "tab"
	keyShiftTab = "shift+tab"
	keyQ        = "q"
	keyCtrlC    = "ctrl+c"
)

// ConfirmScreen displays a modal confirmation prompt with Accept/Cancel buttons.
type ConfirmScreen struct {
	Message        string
	SelectedButton int // 0 = Confirm, 1 = Cancel
	Thm            *theme.Theme

	// Callbacks
	OnConfirm func() tea.Cmd
	OnCancel  func() tea.Cmd
}

// NewConfirmScreen creates a confirm screen preloaded with a message.
func NewConfirmScreen(message string, thm *theme.Theme) *ConfirmScreen {
	return &ConfirmScreen{
		Message:        message,
		SelectedButton: 0, // Start with Confirm button focused
		Thm:            thm,
	}
}

// NewConfirmScreenWithDefault creates a confirmation modal with a specified default button.
func NewConfirmScreenWithDefault(message string, defaultButton int, thm *theme.Theme) *ConfirmScreen {
	return &ConfirmScreen{
		Message:        message,
		SelectedButton: defaultButton,
		Thm:            thm,
	}
}

// Type returns the screen type.
func (s *ConfirmScreen) Type() Type {
	return TypeConfirm
}

// Update processes keyboard events for the confirmation dialog.
// Returns nil to signal that the screen should be closed.
func (s *ConfirmScreen) Update(msg tea.KeyMsg) (Screen, tea.Cmd) {
	key := msg.String()
	switch key {
	case keyTab, "right", "l":
		s.SelectedButton = (s.SelectedButton + 1) % 2
	case keyShiftTab, "left", "h":
		s.SelectedButton = (s.SelectedButton - 1 + 2) % 2
	case "y", "Y":
		if s.OnConfirm != nil {
			return nil, s.OnConfirm()
		}
		return nil, nil
	case "n", "N":
		if s.OnCancel != nil {
			return nil, s.OnCancel()
		}
		return nil, nil
	case keyEnter:
		if s.SelectedButton == 0 {
			if s.OnConfirm != nil {
				return nil, s.OnConfirm()
			}
		} else {
			if s.OnCancel != nil {
				return nil, s.OnCancel()
			}
		}
		return nil, nil
	case keyEsc, keyQ, keyCtrlC:
		if s.OnCancel != nil {
			return nil, s.OnCancel()
		}
		return nil, nil
	}
	return s, nil
}

// View renders the confirmation UI box with focused button highlighting.
func (s *ConfirmScreen) View() string {
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

	focusedConfirmStyle := lipgloss.NewStyle().
		Width((width-6)/2).
		Align(lipgloss.Center).
		Padding(0, 2).
		Foreground(s.Thm.AccentFg).
		Background(s.Thm.ErrorFg).
		Bold(true)

	focusedCancelStyle := lipgloss.NewStyle().
		Width((width-6)/2).
		Align(lipgloss.Center).
		Padding(0, 2).
		Foreground(s.Thm.AccentFg).
		Background(s.Thm.Accent).
		Bold(true)

	unfocusedButtonStyle := lipgloss.NewStyle().
		Width((width-6)/2).
		Align(lipgloss.Center).
		Padding(0, 2).
		Foreground(s.Thm.MutedFg).
		Background(s.Thm.BorderDim)

	var confirmButton, cancelButton string
	if s.SelectedButton == 0 {
		confirmButton = focusedConfirmStyle.Render("[Confirm]")
		cancelButton = unfocusedButtonStyle.Render("[Cancel]")
	} else {
		confirmButton = unfocusedButtonStyle.Render("[Confirm]")
		cancelButton = focusedCancelStyle.Render("[Cancel]")
	}

	content := fmt.Sprintf("%s\n\n%s  %s",
		messageStyle.Render(s.Message),
		confirmButton,
		cancelButton,
	)

	return boxStyle.Render(content)
}

// SetTheme updates the theme for this screen.
func (s *ConfirmScreen) SetTheme(thm *theme.Theme) {
	s.Thm = thm
}
