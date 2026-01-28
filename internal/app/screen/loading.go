package screen

import (
	"math/rand/v2"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/chmouel/lazyworktree/internal/theme"
)

// LoadingTips is a list of helpful tips shown during loading.
var LoadingTips = []string{
	"Press '?' to view the help guide anytime.",
	"Use '/' to search in almost any list view.",
	"Press 'c' to create a worktree from a branch, PR, or issue.",
	"Use 'D' to delete a worktree (and optionally its branch).",
	"Press 'g' to open LazyGit in the current worktree.",
	"Switch between panes using '1', '2', or '3'.",
	"Zoom into a pane with '='.",
	"Press ':' or 'Ctrl+P' to open the Command Palette.",
	"Use 'r' to refresh the worktree list manually.",
	"Press 's' to cycle sorting modes.",
	"Use 'o' to open the related PR/MR in your browser.",
	"Press 'R' to fetch all remotes.",
	"Use 'S' to synchronise with upstream (pull then push).",
	"Press 'P' to push to the upstream branch.",
	"Use 'p' to fetch PR/MR status from GitHub or GitLab.",
	"Press 'f' to filter the focused pane.",
	"Use Tab to cycle to the next pane.",
	"Press 'A' to absorb a worktree into main (merge + delete).",
	"Use 'X' to prune merged worktrees automatically.",
	"Press '!' to run an arbitrary command in the selected worktree.",
	"Use 'm' to rename a worktree.",
	"In the Status pane, press 'e' to open a file in your editor.",
	"In the Status pane, press 's' to stage or unstage files.",
	"In the Log pane, press 'C' to cherry-pick a commit to another worktree.",
	"Press Enter on a worktree to jump there and cd into it.",
	"Generate shell completions with: lazyworktree --completion <shell>.",
}

// LoadingScreen displays a modal with a spinner and a random tip.
type LoadingScreen struct {
	Message        string
	FrameIdx       int
	BorderColorIdx int
	Tip            string
	Thm            *theme.Theme
	SpinnerFrames  []string
	ShowIcons      bool
}

// DefaultSpinnerFrames returns the text-only spinner frames.
func DefaultSpinnerFrames() []string {
	return []string{"...", ".. ", ".  "}
}

// NewLoadingScreen creates a loading modal with the given message.
// spinnerFrames should be provided by the caller; if nil, text fallback is used.
func NewLoadingScreen(message string, thm *theme.Theme, spinnerFrames []string) *LoadingScreen {
	frames := spinnerFrames
	if len(frames) == 0 {
		frames = DefaultSpinnerFrames()
	}

	// Pick a random tip (cryptographic randomness not needed for UI tips)
	tip := LoadingTips[rand.IntN(len(LoadingTips))] //nolint:gosec

	return &LoadingScreen{
		Message:       message,
		Tip:           tip,
		Thm:           thm,
		SpinnerFrames: frames,
	}
}

// Type returns the screen type.
func (s *LoadingScreen) Type() Type {
	return TypeLoading
}

// Update handles key events. Loading screen does not respond to keys.
func (s *LoadingScreen) Update(msg tea.KeyMsg) (Screen, tea.Cmd) {
	// Loading screen ignores key input
	return s, nil
}

// loadingBorderColors returns the colour cycle for the pulsing border.
func (s *LoadingScreen) loadingBorderColors() []lipgloss.Color {
	return []lipgloss.Color{
		s.Thm.Accent,
		s.Thm.SuccessFg,
		s.Thm.WarnFg,
		s.Thm.Accent,
	}
}

// Tick advances the loading animation (spinner frame and border colour).
func (s *LoadingScreen) Tick() {
	s.FrameIdx = (s.FrameIdx + 1) % len(s.SpinnerFrames)
	colours := s.loadingBorderColors()
	s.BorderColorIdx = (s.BorderColorIdx + 1) % len(colours)
}

// View renders the loading modal with spinner, message, and a random tip.
func (s *LoadingScreen) View() string {
	width := 60
	height := 9

	colours := s.loadingBorderColors()
	borderColour := colours[s.BorderColorIdx%len(colours)]

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColour).
		Padding(1, 2).
		Width(width).
		Height(height)

	// Spinner animation
	spinnerFrame := s.SpinnerFrames[s.FrameIdx%len(s.SpinnerFrames)]
	spinnerStyle := lipgloss.NewStyle().
		Foreground(s.Thm.Accent).
		Bold(true)

	// Message styling
	messageStyle := lipgloss.NewStyle().
		Foreground(s.Thm.TextFg).
		Bold(true)

	// Separator line
	separatorStyle := lipgloss.NewStyle().
		Foreground(s.Thm.BorderDim)
	separator := separatorStyle.Render(strings.Repeat("-", width-6))

	// Tip styling - truncate to fit on one line
	tipText := s.Tip
	maxTipLen := width - 12 // "Tip: " prefix + padding
	if len(tipText) > maxTipLen {
		tipText = tipText[:maxTipLen-3] + "..."
	}
	tipStyle := lipgloss.NewStyle().
		Foreground(s.Thm.MutedFg).
		Italic(true)

	// Layout: spinner, message, separator, tip
	content := lipgloss.JoinVertical(lipgloss.Center,
		spinnerStyle.Render(spinnerFrame),
		"",
		messageStyle.Render(s.Message),
		separator,
		tipStyle.Render("Tip: "+tipText),
	)

	return boxStyle.Render(content)
}

// SetTheme updates the theme for this screen.
func (s *LoadingScreen) SetTheme(thm *theme.Theme) {
	s.Thm = thm
}

// SetSpinnerFrames updates the spinner frames.
func (s *LoadingScreen) SetSpinnerFrames(frames []string) {
	if len(frames) > 0 {
		s.SpinnerFrames = frames
	}
}
