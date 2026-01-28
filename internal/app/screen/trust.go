package screen

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/chmouel/lazyworktree/internal/theme"
)

// TrustScreen surfaces trust warnings and records commands for a path.
type TrustScreen struct {
	FilePath string
	Commands []string
	Viewport viewport.Model
	Thm      *theme.Theme

	// Callbacks
	OnTrust  func() tea.Cmd // Called when user trusts the commands
	OnBlock  func() tea.Cmd // Called when user blocks/skips the commands
	OnCancel func() tea.Cmd // Called when user cancels the operation
}

// NewTrustScreen warns the user when a repo config has changed or is untrusted.
func NewTrustScreen(filePath string, commands []string, thm *theme.Theme) *TrustScreen {
	commandsText := strings.Join(commands, "\n")
	question := fmt.Sprintf("The repository config '%s' defines the following commands.\nThis file has changed or hasn't been trusted yet.\nDo you trust these commands to run?", filePath)

	content := fmt.Sprintf("%s\n\n%s", question, commandsText)

	vp := viewport.New(70, 20)
	vp.SetContent(content)

	return &TrustScreen{
		FilePath: filePath,
		Commands: commands,
		Viewport: vp,
		Thm:      thm,
	}
}

// Type returns the screen type.
func (s *TrustScreen) Type() Type {
	return TypeTrust
}

// Update handles trust decisions and delegates viewport input updates.
// Returns nil to signal that the screen should be closed.
func (s *TrustScreen) Update(msg tea.KeyMsg) (Screen, tea.Cmd) {
	keyStr := msg.String()
	switch keyStr {
	case "t", "T":
		if s.OnTrust != nil {
			return nil, s.OnTrust()
		}
		return nil, nil
	case "b", "B":
		if s.OnBlock != nil {
			return nil, s.OnBlock()
		}
		return nil, nil
	case keyEsc, "c", "C", keyCtrlC:
		if s.OnCancel != nil {
			return nil, s.OnCancel()
		}
		return nil, nil
	}
	// Delegate scroll keys to viewport
	var cmd tea.Cmd
	s.Viewport, cmd = s.Viewport.Update(msg)
	return s, cmd
}

// View renders the trust warning content inside a styled box.
func (s *TrustScreen) View() string {
	width := 70
	height := 25

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.Thm.WarnFg).
		Padding(1, 2).
		Width(width).
		Height(height)

	buttonStyle := lipgloss.NewStyle().
		Width(20).
		Align(lipgloss.Center).
		Padding(0, 1).
		Margin(0, 1)

	trustButton := buttonStyle.
		Foreground(s.Thm.SuccessFg).
		Render("[Trust & Run]")

	blockButton := buttonStyle.
		Foreground(s.Thm.WarnFg).
		Render("[Block (Skip)]")

	cancelButton := buttonStyle.
		Foreground(s.Thm.ErrorFg).
		Render("[Cancel Operation]")

	content := fmt.Sprintf("%s\n\n%s  %s  %s",
		s.Viewport.View(),
		trustButton,
		blockButton,
		cancelButton,
	)

	return boxStyle.Render(content)
}

// SetTheme updates the theme for this screen.
func (s *TrustScreen) SetTheme(thm *theme.Theme) {
	s.Thm = thm
}
