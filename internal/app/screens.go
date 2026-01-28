package app

import (
	"fmt"
	"math/rand/v2"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/chmouel/lazyworktree/internal/theme"
)

type screenType int

const (
	screenNone screenType = iota
	screenLoading

	// Key constants (keyEnter and keyEsc are defined in app.go)
	keyCtrlD    = "ctrl+d"
	keyCtrlU    = "ctrl+u"
	keyCtrlC    = "ctrl+c"
	keyCtrlJ    = "ctrl+j"
	keyCtrlK    = "ctrl+k"
	keyDown     = "down"
	keyQ        = "q"
	keyUp       = "up"
	keyTab      = "tab"
	keyShiftTab = "shift+tab"

	// Placeholder text constants
	placeholderFilterFiles = "Filter files..."
)

// loadingTips is a list of helpful tips shown during loading.
var loadingTips = []string{
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

// TrustScreen surfaces trust warnings and records commands for a path.
type TrustScreen struct {
	filePath string
	commands []string
	viewport viewport.Model
	result   chan string
	thm      *theme.Theme
}

// WelcomeScreen shows the initial instructions when no worktrees are open.
type WelcomeScreen struct {
	currentDir  string
	worktreeDir string
	result      chan bool
	thm         *theme.Theme
}

// CommitScreen displays metadata, stats, and diff details for a single commit.
type CommitScreen struct {
	meta     commitMeta
	stat     string
	diff     string
	useDelta bool
	viewport viewport.Model
	thm      *theme.Theme
}

// CommandPaletteScreen lets the user pick a command from a filtered list.

type selectionItem struct {
	id          string
	label       string
	description string
}

// ChecklistItem represents a single item with a checkbox state.
type ChecklistItem struct {
	ID          string
	Label       string
	Description string
	Checked     bool
}

// LoadingScreen displays a modal with a spinner and a random tip.
type LoadingScreen struct {
	message        string
	frameIdx       int
	borderColorIdx int
	tip            string
	thm            *theme.Theme
	showIcons      bool
}

// NewLoadingScreen creates a loading modal with the given message.
func NewLoadingScreen(message string, thm *theme.Theme, showIcons bool) *LoadingScreen {
	// Pick a random tip (cryptographic randomness not needed for UI tips)
	tip := loadingTips[rand.IntN(len(loadingTips))] //nolint:gosec

	return &LoadingScreen{
		message:   message,
		tip:       tip,
		thm:       thm,
		showIcons: showIcons,
	}
}

// loadingBorderColors returns the color cycle for the pulsing border.
func (s *LoadingScreen) loadingBorderColors() []lipgloss.Color {
	return []lipgloss.Color{
		s.thm.Accent,
		s.thm.SuccessFg,
		s.thm.WarnFg,
		s.thm.Accent,
	}
}

// Tick advances the loading animation (spinner frame and border colour).
func (s *LoadingScreen) Tick() {
	frames := spinnerFrameSet(s.showIcons)
	s.frameIdx = (s.frameIdx + 1) % len(frames)
	colors := s.loadingBorderColors()
	s.borderColorIdx = (s.borderColorIdx + 1) % len(colors)
}

// View renders the loading modal with spinner, message, and a random tip.
func (s *LoadingScreen) View() string {
	width := 60
	height := 9

	colors := s.loadingBorderColors()
	borderColor := colors[s.borderColorIdx%len(colors)]

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 2).
		Width(width).
		Height(height)

	// Spinner animation
	frames := spinnerFrameSet(s.showIcons)
	spinnerFrame := frames[s.frameIdx%len(frames)]
	spinnerStyle := lipgloss.NewStyle().
		Foreground(s.thm.Accent).
		Bold(true)

	// Message styling
	messageStyle := lipgloss.NewStyle().
		Foreground(s.thm.TextFg).
		Bold(true)

	// Separator line
	separatorStyle := lipgloss.NewStyle().
		Foreground(s.thm.BorderDim)
	separator := separatorStyle.Render(strings.Repeat("─", width-6))

	// Tip styling - truncate to fit on one line
	tipText := s.tip
	maxTipLen := width - 12 // "Tip: " prefix + padding
	if len(tipText) > maxTipLen {
		tipText = tipText[:maxTipLen-3] + "..."
	}
	tipStyle := lipgloss.NewStyle().
		Foreground(s.thm.MutedFg).
		Italic(true)

	// Layout: spinner, message, separator, tip
	content := lipgloss.JoinVertical(lipgloss.Center,
		spinnerStyle.Render(spinnerFrame),
		"",
		messageStyle.Render(s.message),
		"",
		separator,
		tipStyle.Render("Tip: "+tipText),
	)

	centeredContent := lipgloss.NewStyle().
		Width(width-4).
		Height(height-2).
		Align(lipgloss.Center, lipgloss.Center).
		Render(content)

	return boxStyle.Render(centeredContent)
}

// NewTrustScreen warns the user when a repo config has changed or is untrusted.
func NewTrustScreen(filePath string, commands []string, thm *theme.Theme) *TrustScreen {
	commandsText := strings.Join(commands, "\n")
	question := fmt.Sprintf("The repository config '%s' defines the following commands.\nThis file has changed or hasn't been trusted yet.\nDo you trust these commands to run?", filePath)

	content := fmt.Sprintf("%s\n\n%s", question, commandsText)

	vp := viewport.New(70, 20)
	vp.SetContent(content)

	return &TrustScreen{
		filePath: filePath,
		commands: commands,
		viewport: vp,
		result:   make(chan string, 1),
		thm:      thm,
	}
}

// Init satisfies tea.Model.Init for the trust confirmation screen.
func (s *TrustScreen) Init() tea.Cmd {
	return nil
}

// Update handles trust decisions and delegates viewport input updates.
func (s *TrustScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	keyMsg, ok := msg.(tea.KeyMsg)
	if ok {
		switch keyMsg.String() {
		case "t", "T":
			s.result <- "trust"
			return s, tea.Quit
		case "b", "B":
			s.result <- "block"
			return s, tea.Quit
		case keyEsc, "c", "C", keyCtrlC:
			s.result <- "cancel"
			return s, tea.Quit
		}
	}
	s.viewport, cmd = s.viewport.Update(msg)
	return s, cmd
}

// View renders the trust warning content inside a styled box.
func (s *TrustScreen) View() string {
	width := 70
	height := 25

	// Enhanced trust warning with rounded border and warning color
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.thm.WarnFg). // Use warning color for attention
		Padding(1, 2).
		Width(width).
		Height(height)

	buttonStyle := lipgloss.NewStyle().
		Width(20).
		Align(lipgloss.Center).
		Padding(0, 1).
		Margin(0, 1)

	trustButton := buttonStyle.
		Foreground(s.thm.SuccessFg).
		Render("[Trust & Run]")

	blockButton := buttonStyle.
		Foreground(s.thm.WarnFg).
		Render("[Block (Skip)]")

	cancelButton := buttonStyle.
		Foreground(s.thm.ErrorFg).
		Render("[Cancel Operation]")

	content := fmt.Sprintf("%s\n\n%s  %s  %s",
		s.viewport.View(),
		trustButton,
		blockButton,
		cancelButton,
	)

	return boxStyle.Render(content)
}

// NewWelcomeScreen builds the greeting screen shown when no worktrees exist.
func NewWelcomeScreen(currentDir, worktreeDir string, thm *theme.Theme) *WelcomeScreen {
	return &WelcomeScreen{
		currentDir:  currentDir,
		worktreeDir: worktreeDir,
		result:      make(chan bool, 1),
		thm:         thm,
	}
}

// Init is part of the tea.Model interface for the welcome screen.
func (s *WelcomeScreen) Init() tea.Cmd {
	return nil
}

// Update listens for quit keys on the welcome screen.
func (s *WelcomeScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if ok {
		switch keyMsg.String() {
		case keyQ, "Q", keyEsc, keyCtrlC:
			s.result <- false
			return s, tea.Quit
		}
	}
	return s, nil
}

// View renders the welcome dialog with guidance and action buttons.
func (s *WelcomeScreen) View() string {
	width := 60
	height := 15

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(s.thm.Accent).
		Padding(2, 4).
		Width(width).
		Height(height).
		Align(lipgloss.Center, lipgloss.Center)

	titleStyle := lipgloss.NewStyle().
		Foreground(s.thm.Accent).
		Bold(true).
		MarginBottom(1).
		Underline(true)

	warningStyle := lipgloss.NewStyle().
		Foreground(s.thm.WarnFg).
		Bold(true)

	textStyle := lipgloss.NewStyle().
		Foreground(s.thm.MutedFg).
		Italic(true)

	buttonStyle := lipgloss.NewStyle().
		Foreground(s.thm.AccentFg).
		Background(s.thm.Accent).
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

// NewCommitScreen configures the commit detail viewer for the selected SHA.
func NewCommitScreen(meta commitMeta, stat, diff string, useDelta bool, thm *theme.Theme) *CommitScreen {
	vp := viewport.New(110, 60)

	screen := &CommitScreen{
		meta:     meta,
		stat:     stat,
		diff:     diff,
		useDelta: useDelta,
		viewport: vp,
		thm:      thm,
	}

	screen.setViewportContent()
	return screen
}

// Init satisfies tea.Model.Init for the commit detail view.
func (s *CommitScreen) Init() tea.Cmd {
	return nil
}

// Update handles scrolling and closing events for the commit screen.
func (s *CommitScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	keyMsg, ok := msg.(tea.KeyMsg)
	if ok {
		switch keyMsg.String() {
		case keyQ, keyEsc, keyCtrlC:
			return s, tea.Quit
		case "j", keyDown:
			s.viewport.ScrollDown(1)
			return s, nil
		case "k", keyUp:
			s.viewport.ScrollUp(1)
			return s, nil
		case keyCtrlD, " ":
			s.viewport.HalfPageDown()
			return s, nil
		case keyCtrlU:
			s.viewport.HalfPageUp()
			return s, nil
		case "g":
			s.viewport.GotoTop()
			return s, nil
		case "G":
			s.viewport.GotoBottom()
			return s, nil
		}
	}
	s.viewport, cmd = s.viewport.Update(msg)
	return s, cmd
}

func (s *CommitScreen) setViewportContent() {
	s.viewport.SetContent(s.buildBody())
}

func (s *CommitScreen) buildBody() string {
	parts := []string{}
	parts = append(parts, s.renderHeader())
	if strings.TrimSpace(s.stat) != "" {
		parts = append(parts, s.stat)
	}
	if strings.TrimSpace(s.diff) != "" {
		parts = append(parts, s.diff)
	}
	return strings.Join(parts, "\n\n")
}

func (s *CommitScreen) renderHeader() string {
	label := lipgloss.NewStyle().Foreground(s.thm.MutedFg).Bold(true)
	value := lipgloss.NewStyle().Foreground(s.thm.TextFg)
	subjectStyle := lipgloss.NewStyle().Bold(true).Foreground(s.thm.Accent)
	bodyStyle := lipgloss.NewStyle().Foreground(s.thm.MutedFg)

	lines := []string{
		fmt.Sprintf("%s %s", label.Render("Commit:"), value.Render(s.meta.sha)),
		fmt.Sprintf("%s %s <%s>", label.Render("Author:"), value.Render(s.meta.author), value.Render(s.meta.email)),
		fmt.Sprintf("%s %s", label.Render("Date:"), value.Render(s.meta.date)),
	}
	if s.meta.subject != "" {
		lines = append(lines, "")
		lines = append(lines, subjectStyle.Render(s.meta.subject))
	}
	if len(s.meta.body) > 0 {
		for _, l := range s.meta.body {
			if strings.TrimSpace(l) == "" {
				lines = append(lines, "")
				continue
			}
			lines = append(lines, bodyStyle.Render(l))
		}
	}

	header := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Padding(0, 1).
		Render(header)
}

// View renders the commit screen
func (s *CommitScreen) View() string {
	width := maxInt(100, s.viewport.Width)

	// Enhanced commit view with rounded border
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.thm.Accent).
		Padding(0, 1).
		Width(width)

	return boxStyle.Render(s.viewport.View())
}
