package screen

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/chmouel/lazyworktree/internal/theme"
)

// WelcomeScreen shows the initial instructions when no worktrees are open.
type WelcomeScreen struct {
	CurrentDir  string
	WorktreeDir string
	Thm         *theme.Theme

	// Callbacks
	OnRefresh func() tea.Cmd // Called when user presses 'r' to refresh
	OnQuit    func() tea.Cmd // Called when user wants to quit
}

// NewWelcomeScreen builds the greeting screen shown when no worktrees exist.
func NewWelcomeScreen(currentDir, worktreeDir string, thm *theme.Theme) *WelcomeScreen {
	return &WelcomeScreen{
		CurrentDir:  currentDir,
		WorktreeDir: worktreeDir,
		Thm:         thm,
	}
}

// Type returns the screen type.
func (s *WelcomeScreen) Type() Type {
	return TypeWelcome
}

// Update processes keyboard events for the welcome screen.
// Returns nil to signal that the screen should be closed.
func (s *WelcomeScreen) Update(msg tea.KeyMsg) (Screen, tea.Cmd) {
	keyStr := msg.String()
	switch keyStr {
	case "r", "R":
		if s.OnRefresh != nil {
			return nil, s.OnRefresh()
		}
		return nil, nil
	case keyQ, "Q", keyEnter, keyEsc, keyCtrlC:
		if s.OnQuit != nil {
			return nil, s.OnQuit()
		}
		return nil, tea.Quit
	}
	return s, nil
}

// View renders the welcome dialog with guidance and action buttons.
func (s *WelcomeScreen) View() string {
	width := 60
	height := 15

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(s.Thm.Accent).
		Padding(2, 4).
		Width(width).
		Height(height).
		Align(lipgloss.Center, lipgloss.Center)

	titleStyle := lipgloss.NewStyle().
		Foreground(s.Thm.Accent).
		Bold(true).
		MarginBottom(1).
		Underline(true)

	warningStyle := lipgloss.NewStyle().
		Foreground(s.Thm.WarnFg).
		Bold(true)

	textStyle := lipgloss.NewStyle().
		Foreground(s.Thm.MutedFg).
		Italic(true)

	buttonStyle := lipgloss.NewStyle().
		Foreground(s.Thm.AccentFg).
		Background(s.Thm.Accent).
		Padding(0, 1).
		MarginTop(1).
		Bold(true)

	content := lipgloss.JoinVertical(lipgloss.Center,
		titleStyle.Render("LazyWorktree"),
		"",
		fmt.Sprintf("%s  %s", warningStyle.Render("⚠"), warningStyle.Render("No worktrees found")),
		"",
		textStyle.Render("Please ensure you are in a git repository."),
		"",
		buttonStyle.Render("[Q/Enter] Quit"),
	)

	return boxStyle.Render(content)
}

// SetTheme updates the theme for this screen.
func (s *WelcomeScreen) SetTheme(thm *theme.Theme) {
	s.Thm = thm
}
