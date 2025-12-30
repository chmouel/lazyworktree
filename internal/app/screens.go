package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Screen types for modal dialogs
type screenType int

const (
	screenNone screenType = iota
	screenConfirm
	screenInput
	screenHelp
	screenTrust
	screenWelcome
	screenCommit
)

// ConfirmScreen represents a confirmation dialog
type ConfirmScreen struct {
	message string
	result  chan bool
}

// InputScreen represents an input dialog
type InputScreen struct {
	prompt      string
	placeholder string
	value       string
	input       textinput.Model
	errorMsg    string
	boxWidth    int
	result      chan string
}

// HelpScreen represents a help screen
type HelpScreen struct {
	viewport    viewport.Model
	width       int
	height      int
	fullText    []string
	searchInput textinput.Model
	searching   bool
	searchQuery string
}

// TrustScreen represents a trust confirmation screen
type TrustScreen struct {
	filePath string
	commands []string
	viewport viewport.Model
	result   chan string
}

// WelcomeScreen represents a welcome screen
type WelcomeScreen struct {
	currentDir  string
	worktreeDir string
	result      chan bool
}

// CommitScreen represents a commit detail screen
type CommitScreen struct {
	header      string
	diff        string
	useDelta    bool
	viewport    viewport.Model
	headerShown bool
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// NewConfirmScreen creates a new confirmation screen
func NewConfirmScreen(message string) *ConfirmScreen {
	return &ConfirmScreen{
		message: message,
		result:  make(chan bool, 1),
	}
}

// Init initializes the confirm screen
func (s *ConfirmScreen) Init() tea.Cmd {
	return nil
}

// Update handles updates for the confirm screen
func (s *ConfirmScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y", "enter":
			s.result <- true
			return s, tea.Quit
		case "n", "N", "esc", "q":
			s.result <- false
			return s, tea.Quit
		}
	}
	return s, nil
}

// View renders the confirm screen
func (s *ConfirmScreen) View() string {
	width := 60
	height := 11

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(1, 2).
		Width(width).
		Height(height)

	messageStyle := lipgloss.NewStyle().
		Width(width-4).
		Height(height-6).
		Align(lipgloss.Center, lipgloss.Center)

	buttonStyle := lipgloss.NewStyle().
		Width((width-6)/2).
		Align(lipgloss.Center).
		Padding(0, 1)

	confirmButton := buttonStyle.
		Foreground(lipgloss.Color("9")).
		Render("[Confirm]")

	cancelButton := buttonStyle.
		Foreground(lipgloss.Color("4")).
		Render("[Cancel]")

	content := fmt.Sprintf("%s\n\n%s  %s",
		messageStyle.Render(s.message),
		confirmButton,
		cancelButton,
	)

	return boxStyle.Render(content)
}

// NewInputScreen creates a new input screen
func NewInputScreen(prompt, placeholder, value string) *InputScreen {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.SetValue(value)
	ti.Focus()
	ti.CharLimit = 200
	ti.Prompt = ""
	ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	// Compute a comfortable width based on content, bounded to avoid screen overflow
	promptWidth := lipgloss.Width(prompt)
	valueWidth := lipgloss.Width(value)
	boxWidth := maxInt(42, minInt(96, maxInt(promptWidth+8, valueWidth+10)))
	ti.Width = boxWidth - 8

	return &InputScreen{
		prompt:      prompt,
		placeholder: placeholder,
		value:       value,
		input:       ti,
		errorMsg:    "",
		boxWidth:    boxWidth,
		result:      make(chan string, 1),
	}
}

// Init initializes the input screen
func (s *InputScreen) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles updates for the input screen
func (s *InputScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			value := s.input.Value()
			s.result <- value
			return s, tea.Quit
		case "esc":
			s.result <- ""
			return s, tea.Quit
		}
	}

	s.input, cmd = s.input.Update(msg)
	return s, cmd
}

// View renders the input screen
func (s *InputScreen) View() string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("236")).
		Padding(1, 2).
		Width(s.boxWidth).
		Align(lipgloss.Center, lipgloss.Center)

	inputWrapperStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("239")).
		Padding(0, 1).
		Width(s.boxWidth - 6)

	promptStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		MarginBottom(1)

	contentLines := []string{
		promptStyle.Render(s.prompt),
		inputWrapperStyle.Render(s.input.View()),
	}

	if s.errorMsg != "" {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Width(s.boxWidth - 4)
		contentLines = append(contentLines, errorStyle.Render(s.errorMsg))
	}

	content := strings.Join(contentLines, "\n\n")

	return boxStyle.Render(content)
}

// NewHelpScreen creates a new help screen sized to the current window
func NewHelpScreen(maxWidth, maxHeight int) *HelpScreen {
	helpText := `# Git Worktree Status Help

**Navigation**
- j / Down: Move cursor down
- k / Up: Move cursor up
- 1: Focus Worktree pane
- 2: Focus Info/Diff pane
- 3: Focus Log pane
- Enter: Jump to selected worktree (exit and cd)
- Tab: Cycle focus (table → status → log)

**Diff/Status Pane Navigation (when focused)**
- j/k: Line up/down
- Ctrl+D / Space: Half page down
- Ctrl+U: Half page up
- PageDown / PageUp: Full page up/down
- g: Go to top
- G: Go to bottom

**Log Pane**
- j / k: Move between commits
- Enter: Open commit details and diff

**Actions**
- c: Create new worktree
- d: Manually refresh diff (diffs auto-show when worktree is dirty)
- D: Delete selected worktree
- f: Fetch all remotes
- p: Fetch PR status from GitHub
- r: Refresh list
- s: Sort (toggle Name/Last Active)
- /: Filter worktrees
- g: Open LazyGit (or go to top if in diff pane)
- ?: Show this help

**Status Indicators**
- ✔ Clean: No local changes
- ✎ Dirty: Uncommitted changes
- ↑N: Ahead of remote by N commits
- ↓N: Behind remote by N commits

**Performance Note**
PR data is not fetched by default for speed.
Press p to fetch PR information from GitHub.`

	width := 80
	height := 30
	if maxWidth > 0 {
		width = minInt(120, maxInt(60, maxWidth))
	}
	if maxHeight > 0 {
		height = minInt(60, maxInt(25, maxHeight))
	}

	vp := viewport.New(width, maxInt(5, height-3))
	fullLines := strings.Split(helpText, "\n")

	ti := textinput.New()
	ti.Placeholder = "Search help (/ to start, Enter to apply, Esc to clear)"
	ti.CharLimit = 64
	ti.Prompt = "/ "
	ti.SetValue("")
	ti.Blur()
	ti.Width = maxInt(20, width-6)

	hs := &HelpScreen{
		viewport:    vp,
		width:       width,
		height:      height,
		fullText:    fullLines,
		searchInput: ti,
	}

	hs.refreshContent()
	return hs
}

// Init initializes the help screen
func (s *HelpScreen) Init() tea.Cmd {
	return nil
}

// Update handles updates for the help screen, including search
func (s *HelpScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "/":
			if !s.searching {
				s.searching = true
				s.searchInput.Focus()
				return s, textinput.Blink
			}
		case "enter":
			if s.searching {
				s.searchQuery = strings.TrimSpace(s.searchInput.Value())
				s.searching = false
				s.searchInput.Blur()
				s.refreshContent()
				return s, nil
			}
		case "esc":
			if s.searching || s.searchQuery != "" {
				s.searching = false
				s.searchInput.SetValue("")
				s.searchQuery = ""
				s.searchInput.Blur()
				s.refreshContent()
				return s, nil
			}
		}
	}

	if s.searching {
		s.searchInput, cmd = s.searchInput.Update(msg)
		return s, cmd
	}

	s.viewport, cmd = s.viewport.Update(msg)
	return s, cmd
}

func (s *HelpScreen) refreshContent() {
	content := s.renderContent()
	s.viewport.SetContent(content)
	s.viewport.GotoTop()
}

// SetSize updates the help screen dimensions (useful on terminal resize)
func (s *HelpScreen) SetSize(width, height int) {
	if width > 0 {
		s.width = width
	}
	if height > 0 {
		s.height = height
	}
	s.viewport.Width = s.width
	s.viewport.Height = maxInt(5, s.height-2)
}

func (s *HelpScreen) renderContent() string {
	if strings.TrimSpace(s.searchQuery) == "" {
		return strings.Join(s.fullText, "\n")
	}

	query := strings.ToLower(s.searchQuery)
	lineStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("60")).Bold(true)
	lines := []string{}
	for _, line := range s.fullText {
		if strings.Contains(strings.ToLower(line), query) {
			lines = append(lines, lineStyle.Render(line))
		}
	}

	if len(lines) == 0 {
		return fmt.Sprintf("No help entries match %q", s.searchQuery)
	}
	return strings.Join(lines, "\n")
}

// View renders the help screen as a full-screen overlay
func (s *HelpScreen) View() string {
	content := s.renderContent()

	// Keep viewport sized to available area (minus header/search lines)
	vHeight := maxInt(5, s.height-2)
	s.viewport.Width = s.width
	s.viewport.Height = vHeight
	s.viewport.SetContent(content)

	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("60")).Bold(true).Padding(0, 1)
	title := titleStyle.Render(" Help — / search • q/esc close ")

	searchLine := s.searchInput.View()

	lines := []string{title}
	if searchLine != "" {
		lines = append(lines, searchLine)
	}
	lines = append(lines, s.viewport.View())

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// NewTrustScreen creates a new trust screen
func NewTrustScreen(filePath string, commands []string) *TrustScreen {
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
	}
}

// Init initializes the trust screen
func (s *TrustScreen) Init() tea.Cmd {
	return nil
}

// Update handles updates for the trust screen
func (s *TrustScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "t", "T":
			s.result <- "trust"
			return s, tea.Quit
		case "b", "B":
			s.result <- "block"
			return s, tea.Quit
		case "esc", "c", "C":
			s.result <- "cancel"
			return s, tea.Quit
		}
	}
	s.viewport, cmd = s.viewport.Update(msg)
	return s, cmd
}

// View renders the trust screen
func (s *TrustScreen) View() string {
	width := 70
	height := 25

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(1, 2).
		Width(width).
		Height(height)

	buttonStyle := lipgloss.NewStyle().
		Width(20).
		Align(lipgloss.Center).
		Padding(0, 1).
		Margin(0, 1)

	trustButton := buttonStyle.
		Foreground(lipgloss.Color("2")).
		Render("[Trust & Run]")

	blockButton := buttonStyle.
		Foreground(lipgloss.Color("3")).
		Render("[Block (Skip)]")

	cancelButton := buttonStyle.
		Foreground(lipgloss.Color("1")).
		Render("[Cancel Operation]")

	content := fmt.Sprintf("%s\n\n%s  %s  %s",
		s.viewport.View(),
		trustButton,
		blockButton,
		cancelButton,
	)

	return boxStyle.Render(content)
}

// NewWelcomeScreen creates a new welcome screen
func NewWelcomeScreen(currentDir, worktreeDir string) *WelcomeScreen {
	return &WelcomeScreen{
		currentDir:  currentDir,
		worktreeDir: worktreeDir,
		result:      make(chan bool, 1),
	}
}

// Init initializes the welcome screen
func (s *WelcomeScreen) Init() tea.Cmd {
	return nil
}

// Update handles updates for the welcome screen
func (s *WelcomeScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "r", "R":
			s.result <- true
			return s, tea.Quit
		case "q", "Q", "esc":
			s.result <- false
			return s, tea.Quit
		}
	}
	return s, nil
}

// View renders the welcome screen
func (s *WelcomeScreen) View() string {
	width := 70
	height := 20

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("6")).
		Padding(1, 2).
		Width(width).
		Height(height)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("6")).
		Align(lipgloss.Center).
		MarginBottom(1)

	messageStyle := lipgloss.NewStyle().
		Align(lipgloss.Center).
		MarginBottom(2)

	buttonStyle := lipgloss.NewStyle().
		Width(20).
		Align(lipgloss.Center).
		Padding(0, 1).
		Margin(0, 1)

	quitButton := buttonStyle.
		Foreground(lipgloss.Color("1")).
		Render("[Quit]")

	retryButton := buttonStyle.
		Foreground(lipgloss.Color("4")).
		Render("[Retry]")

	message := fmt.Sprintf("No worktrees found.\n\nCurrent Directory: %s\nWorktree Root: %s\n\nPlease ensure you are in a git repository or the configured worktree root.\nYou may need to initialize a repository or configure 'worktree_dir' in config.",
		s.currentDir,
		s.worktreeDir,
	)

	content := fmt.Sprintf("%s\n%s\n\n%s  %s",
		titleStyle.Render("Welcome to LazyWorktree"),
		messageStyle.Render(message),
		quitButton,
		retryButton,
	)

	return boxStyle.Render(content)
}

// NewCommitScreen creates a new commit detail screen
func NewCommitScreen(header, diff string, useDelta bool) *CommitScreen {
	content := fmt.Sprintf("%s\n\n%s", header, diff)
	vp := viewport.New(95, 95)
	vp.SetContent(content)

	return &CommitScreen{
		header:      header,
		diff:        diff,
		useDelta:    useDelta,
		viewport:    vp,
		headerShown: true,
	}
}

// Init initializes the commit screen
func (s *CommitScreen) Init() tea.Cmd {
	return nil
}

// Update handles updates for the commit screen
func (s *CommitScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			return s, tea.Quit
		case "j", "down":
			s.viewport.ScrollDown(1)
			return s, nil
		case "k", "up":
			s.viewport.ScrollUp(1)
			return s, nil
		case "ctrl+d", " ":
			s.viewport.HalfPageDown()
			return s, nil
		case "ctrl+u":
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

// View renders the commit screen
func (s *CommitScreen) View() string {
	width := 95
	height := 95

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1).
		Width(width).
		Height(height)

	return boxStyle.Render(s.viewport.View())
}
