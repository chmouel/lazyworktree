package app

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/theme"
)

type screenType int

const (
	screenNone screenType = iota
	screenConfirm
	screenInfo
	screenInput
	screenHelp
	screenTrust
	screenWelcome
	screenCommit
	screenPalette
	screenDiff
	screenPRSelect
	screenListSelect
	screenLoading
	screenCommitFiles

	// Key constants (keyEnter and keyEsc are defined in app.go)
	keyCtrlD = "ctrl+d"
	keyCtrlU = "ctrl+u"
	keyCtrlC = "ctrl+c"
	keyCtrlJ = "ctrl+j"
	keyCtrlK = "ctrl+k"
	keyDown  = "down"
	keyQ     = "q"
	keyUp    = "up"

	// Placeholder text constants
	placeholderFilterFiles = "Filter files..."
)

// ConfirmScreen displays a modal confirmation prompt with Accept/Cancel buttons.
type ConfirmScreen struct {
	message        string
	result         chan bool
	selectedButton int // 0 = Confirm, 1 = Cancel
	thm            *theme.Theme
}

// InfoScreen displays a modal message with an OK button.
type InfoScreen struct {
	message string
	result  chan bool
	thm     *theme.Theme
}

// InputScreen provides a prompt along with a text input and inline validation.
type InputScreen struct {
	prompt              string
	placeholder         string
	value               string
	input               textinput.Model
	errorMsg            string
	boxWidth            int
	result              chan string
	validate            func(string) string
	thm                 *theme.Theme
	fuzzyFinderInput    bool     // Enable fuzzy finder for suggestions
	suggestions         []string // Available suggestions for fuzzy matching
	filteredSuggestions []string
	selectedSuggestion  int
	history             []string // Command history for up/down cycling
	historyIndex        int      // Current position in history (-1 = not browsing)
	originalInput       string   // Store original input when browsing history
}

// HelpScreen renders searchable documentation for the app controls.
type HelpScreen struct {
	viewport    viewport.Model
	width       int
	height      int
	fullText    []string
	searchInput textinput.Model
	searching   bool
	searchQuery string
	thm         *theme.Theme
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
type CommandPaletteScreen struct {
	items        []paletteItem
	filtered     []paletteItem
	filterInput  textinput.Model
	cursor       int
	scrollOffset int
	thm          *theme.Theme
}

type paletteItem struct {
	id          string
	label       string
	description string
}

type selectionItem struct {
	id          string
	label       string
	description string
}

// PRSelectionScreen lets the user pick a PR from a filtered list.
type PRSelectionScreen struct {
	prs          []*models.PRInfo
	filtered     []*models.PRInfo
	filterInput  textinput.Model
	cursor       int
	scrollOffset int
	width        int
	height       int
	thm          *theme.Theme
	showIcons    bool
}

// ListSelectionScreen lets the user pick from a list of options.
type ListSelectionScreen struct {
	items        []selectionItem
	filtered     []selectionItem
	filterInput  textinput.Model
	cursor       int
	scrollOffset int
	width        int
	height       int
	title        string
	placeholder  string
	noResults    string
	thm          *theme.Theme
}

// DiffScreen renders a full commit diff inside a viewport.
type DiffScreen struct {
	title    string
	content  string
	viewport viewport.Model
	thm      *theme.Theme
}

// LoadingScreen displays a modal with a pulsing border and braille spinner.
type LoadingScreen struct {
	message         string
	borderColorIdx  int
	spinnerFrameIdx int
	thm             *theme.Theme
}

// brailleSpinnerFrames are the frames for the braille dots animation.
var brailleSpinnerFrames = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}

// loadingBorderColors returns the color cycle for the pulsing border.
func (s *LoadingScreen) loadingBorderColors() []lipgloss.Color {
	return []lipgloss.Color{
		s.thm.Accent,
		s.thm.SuccessFg,
		s.thm.WarnFg,
		s.thm.Accent,
	}
}

// NewConfirmScreen creates a confirm screen preloaded with a message.
func NewConfirmScreen(message string, thm *theme.Theme) *ConfirmScreen {
	return &ConfirmScreen{
		message:        message,
		result:         make(chan bool, 1),
		selectedButton: 0, // Start with Confirm button focused
		thm:            thm,
	}
}

// NewInfoScreen creates an informational modal with an OK button.
func NewInfoScreen(message string, thm *theme.Theme) *InfoScreen {
	return &InfoScreen{
		message: message,
		result:  make(chan bool, 1),
		thm:     thm,
	}
}

// NewLoadingScreen creates a loading modal with the given message.
func NewLoadingScreen(message string, thm *theme.Theme) *LoadingScreen {
	return &LoadingScreen{
		message:         message,
		borderColorIdx:  0,
		spinnerFrameIdx: 0,
		thm:             thm,
	}
}

// Init implements the tea.Model Init stage for ConfirmScreen.
func (s *ConfirmScreen) Init() tea.Cmd {
	return nil
}

// Init implements the tea.Model Init stage for InfoScreen.
func (s *InfoScreen) Init() tea.Cmd {
	return nil
}

// Update processes keyboard events for the confirmation dialog.
func (s *ConfirmScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	key := keyMsg.String()
	switch key {
	case "tab", "right", "l":
		s.selectedButton = (s.selectedButton + 1) % 2
	case "shift+tab", "left", "h":
		s.selectedButton = (s.selectedButton - 1 + 2) % 2
	case "y", "Y":
		s.result <- true
		return s, tea.Quit
	case "n", "N":
		s.result <- false
		return s, tea.Quit
	case keyEnter:
		if s.selectedButton == 0 {
			s.result <- true
		} else {
			s.result <- false
		}
		return s, tea.Quit
	case keyEsc, keyQ, keyCtrlC:
		s.result <- false
		return s, tea.Quit
	}
	return s, nil
}

// Update processes keyboard events for the info dialog.
func (s *InfoScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	switch keyMsg.String() {
	case keyEnter, keyEsc, keyQ, keyCtrlC:
		s.result <- true
		return s, tea.Quit
	}
	return s, nil
}

// View renders the confirmation UI box with focused button highlighting.
func (s *ConfirmScreen) View() string {
	width := 60
	height := 11

	// Enhanced confirm modal with rounded border and accent color
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.thm.Accent).
		Padding(1, 2).
		Width(width).
		Height(height)

	messageStyle := lipgloss.NewStyle().
		Width(width-4).
		Height(height-6).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(s.thm.TextFg)

	// Enhanced button styling with better visual hierarchy
	// Focused confirm button
	focusedConfirmStyle := lipgloss.NewStyle().
		Width((width-6)/2).
		Align(lipgloss.Center).
		Padding(0, 2). // More padding for pill effect
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(s.thm.ErrorFg).
		Bold(true)

	// Focused cancel button
	focusedCancelStyle := lipgloss.NewStyle().
		Width((width-6)/2).
		Align(lipgloss.Center).
		Padding(0, 2).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(s.thm.Accent).
		Bold(true)

	unfocusedButtonStyle := lipgloss.NewStyle().
		Width((width-6)/2).
		Align(lipgloss.Center).
		Padding(0, 2).
		Foreground(s.thm.MutedFg).
		Background(s.thm.BorderDim)

	var confirmButton, cancelButton string
	if s.selectedButton == 0 {
		// Confirm is focused
		confirmButton = focusedConfirmStyle.Render("[Confirm]")
		cancelButton = unfocusedButtonStyle.Render("[Cancel]")
	} else {
		// Cancel is focused
		confirmButton = unfocusedButtonStyle.Render("[Confirm]")
		cancelButton = focusedCancelStyle.Render("[Cancel]")
	}

	content := fmt.Sprintf("%s\n\n%s  %s",
		messageStyle.Render(s.message),
		confirmButton,
		cancelButton,
	)

	return boxStyle.Render(content)
}

// View renders the informational UI box with a single OK button.
func (s *InfoScreen) View() string {
	width := 60
	height := 11

	// Enhanced info modal with rounded border
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.thm.Accent).
		Padding(1, 2).
		Width(width).
		Height(height)

	messageStyle := lipgloss.NewStyle().
		Width(width-4).
		Height(height-6).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(s.thm.TextFg)

	// Enhanced button with rounded corners effect
	okStyle := lipgloss.NewStyle().
		Width(width-6).
		Align(lipgloss.Center).
		Padding(0, 2).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(s.thm.Accent).
		Bold(true)

	content := fmt.Sprintf("%s\n\n%s",
		messageStyle.Render(s.message),
		okStyle.Render("[OK]"),
	)

	return boxStyle.Render(content)
}

// Tick advances the loading animation (spinner frame and border color).
func (s *LoadingScreen) Tick() {
	s.spinnerFrameIdx = (s.spinnerFrameIdx + 1) % len(brailleSpinnerFrames)
	colors := s.loadingBorderColors()
	s.borderColorIdx = (s.borderColorIdx + 1) % len(colors)
}

// View renders the loading modal with animated border and braille spinner.
func (s *LoadingScreen) View() string {
	width := 50
	height := 7

	colors := s.loadingBorderColors()
	borderColor := colors[s.borderColorIdx%len(colors)]

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 2).
		Width(width).
		Height(height)

	spinnerFrame := brailleSpinnerFrames[s.spinnerFrameIdx%len(brailleSpinnerFrames)]
	spinnerStyle := lipgloss.NewStyle().
		Foreground(s.thm.Accent).
		Bold(true)

	messageStyle := lipgloss.NewStyle().
		Foreground(s.thm.TextFg)

	contentLine := fmt.Sprintf("%s %s",
		spinnerStyle.Render(spinnerFrame),
		messageStyle.Render(s.message),
	)

	centeredContent := lipgloss.NewStyle().
		Width(width-4).
		Height(height-2).
		Align(lipgloss.Center, lipgloss.Center).
		Render(contentLine)

	return boxStyle.Render(centeredContent)
}

// NewInputScreen builds an input modal with prompt, placeholder, and initial value.
func NewInputScreen(prompt, placeholder, value string, thm *theme.Theme) *InputScreen {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.SetValue(value)
	ti.Focus()
	ti.CharLimit = 200
	ti.Prompt = ""
	ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	// Fixed width to match modal style (60 - padding/border = 52 for input)
	ti.Width = 52

	return &InputScreen{
		prompt:              prompt,
		placeholder:         placeholder,
		value:               value,
		input:               ti,
		errorMsg:            "",
		boxWidth:            60,
		result:              make(chan string, 1),
		thm:                 thm,
		fuzzyFinderInput:    false,
		suggestions:         []string{},
		filteredSuggestions: []string{},
		selectedSuggestion:  0,
		history:             []string{},
		historyIndex:        -1,
		originalInput:       "",
	}
}

// SetValidation adds an optional validation callback for input submission.
func (s *InputScreen) SetValidation(fn func(string) string) {
	s.validate = fn
}

// SetFuzzyFinder enables fuzzy finder with the provided suggestions.
func (s *InputScreen) SetFuzzyFinder(enabled bool, suggestions []string) {
	s.fuzzyFinderInput = enabled
	if enabled && len(suggestions) > 0 {
		s.suggestions = suggestions
		s.filteredSuggestions = suggestions
	}
	s.selectedSuggestion = 0
}

// SetHistory enables bash-style history navigation with up/down arrows.
func (s *InputScreen) SetHistory(history []string) {
	s.history = history
	s.historyIndex = -1
	s.originalInput = ""
}

// Init satisfies tea.Model.Init for the input modal.
func (s *InputScreen) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles keystrokes for the input modal and returns commands on submit.
func (s *InputScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	keyMsg, ok := msg.(tea.KeyMsg)
	if ok {
		switch keyMsg.String() {
		case keyEnter:
			value := s.input.Value()
			// If fuzzy finder is enabled and a suggestion is selected, use it
			if s.fuzzyFinderInput && s.selectedSuggestion >= 0 && s.selectedSuggestion < len(s.filteredSuggestions) {
				value = s.filteredSuggestions[s.selectedSuggestion]
			}
			// Reset history index on submit
			s.historyIndex = -1
			s.result <- value
			return s, tea.Quit
		case keyEsc, keyCtrlC:
			s.result <- ""
			return s, tea.Quit
		case keyDown:
			if s.fuzzyFinderInput && len(s.filteredSuggestions) > 0 {
				s.selectedSuggestion = (s.selectedSuggestion + 1) % len(s.filteredSuggestions)
				return s, nil
			}
		case keyUp:
			if s.fuzzyFinderInput && len(s.filteredSuggestions) > 0 {
				s.selectedSuggestion = (s.selectedSuggestion - 1 + len(s.filteredSuggestions)) % len(s.filteredSuggestions)
				return s, nil
			}
		}

		// If user types something, reset history browsing
		if keyMsg.Type == tea.KeyRunes || keyMsg.Type == tea.KeyBackspace || keyMsg.Type == tea.KeyDelete {
			s.historyIndex = -1
		}
	}

	s.input, cmd = s.input.Update(msg)

	// Update filtered suggestions based on current input
	if s.fuzzyFinderInput && len(s.suggestions) > 0 {
		query := s.input.Value()
		s.filteredSuggestions = filterInputSuggestions(s.suggestions, query)
		if s.selectedSuggestion >= len(s.filteredSuggestions) {
			s.selectedSuggestion = maxInt(0, len(s.filteredSuggestions)-1)
		}
	}

	return s, cmd
}

// View renders the prompt, input field, and error message inside a styled box.
func (s *InputScreen) View() string {
	width := 60

	// Enhanced input modal with rounded border
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.thm.Accent).
		Padding(1, 2).
		Width(width)

	promptStyle := lipgloss.NewStyle().
		Foreground(s.thm.Accent).
		Bold(true).
		Width(width - 6).
		Align(lipgloss.Center)

	inputWrapperStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(s.thm.BorderDim).
		Padding(0, 1).
		Width(width - 6)

	footerStyle := lipgloss.NewStyle().
		Foreground(s.thm.MutedFg).
		Width(width - 6).
		Align(lipgloss.Center).
		MarginTop(1)

	contentLines := []string{
		promptStyle.Render(s.prompt),
		inputWrapperStyle.Render(s.input.View()),
	}

	// Show fuzzy finder suggestions if enabled
	if s.fuzzyFinderInput && len(s.filteredSuggestions) > 0 {
		suggestionsStyle := lipgloss.NewStyle().
			Foreground(s.thm.MutedFg).
			Width(width - 6).
			MarginTop(1)

		selectedStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(s.thm.Accent).
			Padding(0, 1)

		unselectedStyle := lipgloss.NewStyle().
			Foreground(s.thm.TextFg).
			Padding(0, 1)

		suggestions := []string{}
		maxSuggestions := minInt(3, len(s.filteredSuggestions))
		for i := 0; i < maxSuggestions; i++ {
			var item string
			if i == s.selectedSuggestion {
				item = selectedStyle.Render("▸ " + s.filteredSuggestions[i])
			} else {
				item = unselectedStyle.Render("  " + s.filteredSuggestions[i])
			}
			suggestions = append(suggestions, item)
		}

		contentLines = append(contentLines, suggestionsStyle.Render(strings.Join(suggestions, "\n")))
	}

	if s.errorMsg != "" {
		errorStyle := lipgloss.NewStyle().
			Foreground(s.thm.ErrorFg).
			Width(width - 6).
			Align(lipgloss.Center)
		contentLines = append(contentLines, errorStyle.Render(s.errorMsg))
	}

	footerText := "Enter to confirm • Esc to cancel"
	if len(s.history) > 0 {
		footerText = "↑↓ to navigate history • Enter confirm • Esc cancel"
	} else if s.fuzzyFinderInput && len(s.filteredSuggestions) > 0 {
		footerText = "↑↓ to navigate • Enter confirm • Esc cancel"
	}
	contentLines = append(contentLines, footerStyle.Render(footerText))

	content := strings.Join(contentLines, "\n\n")

	return boxStyle.Render(content)
}

// NewHelpScreen initializes help content with the available screen size.
func NewHelpScreen(maxWidth, maxHeight int, customCommands map[string]*config.CustomCommand, thm *theme.Theme) *HelpScreen {
	helpText := `🌲 LazyWorktree Help Guide

**🧭 Navigation**
- j / ↓: Move cursor down
- k / ↑: Move cursor up
- 1 / 2 / 3: Focus Worktree / Status / Log pane
- [ / ]: Previous / Next pane
- Tab: Cycle to next pane
- Enter: Jump to selected worktree (exit and cd)

**📝 Status Pane (when focused)**
- j / k: Navigate files and directories
- Enter: Show diff for selected file in pager
- e: Open selected file in editor
- d: Show full diff (all files) in pager
- s: Stage/unstage selected file or directory
- D: Delete selected file or directory (with confirmation)
- c: Commit staged changes
- C: Stage all changes and commit
- Ctrl+← / →: Jump to previous / next folder
- /: Search file names
- Ctrl+D / Space: Half page down
- Ctrl+U: Half page up
- PageUp / PageDown: Full page up/down
- g / G: Jump to top / bottom

**📜 Log Pane**
- j / k: Move between commits
- Ctrl+J: Next commit and open file tree
- Enter: Open commit file tree (browse changed files)
- C: Cherry-pick commit to another worktree
- /: Search commit titles

**📁 Commit File Tree (viewing files in a commit)**
- j / k: Navigate files and directories
- Enter: Toggle directory or show file diff
- d: Show full commit diff in pager
- f: Filter files by name
- /: Search files (incremental)
- n / N: Next / previous search match
- q / Esc: Return to commit log

**⚡ Worktree Actions**
- c: Create new worktree (from branch, commit, or PR/MR)
- m: Rename selected worktree
- D: Delete selected worktree
- A: Absorb worktree into main (merge + delete)
- X: Prune merged PR worktrees
- !: Run arbitrary command in selected worktree

**🔍 Viewing & Tools**
- d: Full-screen diff viewer
- o: Open PR/MR in browser
- g: Open LazyGit (or go to top in diff pane)
- =: Toggle zoom for focused pane
- : / Ctrl+P: Command Palette
- ?: Show this help

**🔄 Repository Operations**
- r: Refresh worktree list
- R: Fetch all remotes
- p: Fetch PR/MR status from GitHub/GitLab
- s: Cycle sort (Path / Last Active / Last Switched)

**🔎 Filtering & Search**
- f: Filter focused pane
- /: Search focused pane (incremental)
- Alt+N / Alt+P: Move selection and fill filter input
- ↑ / ↓: Move selection (filter active, no fill)
- Ctrl+J / Ctrl+K: Same as above
- Home / End: Jump to first / last item

Search Mode:
- Type: Jump to first matching item
- n / N: Next / previous match
- Enter: Close search
- Esc: Clear search

**📊 Status Indicators**
- ✔: No local changes (clean)
- ✎: Uncommitted changes (dirty)
- ↑N: Ahead of remote by N commits
- ↓N: Behind remote by N commits

**❓ Help Navigation**
- /: Search help (Enter to apply, Esc to clear)
- q / Esc: Close help
- j / k: Scroll up / down
- Ctrl+D / Ctrl+U: Scroll half page down / up

💡 Tip: PR data is not fetched by default for speed.
       Press 'p' to fetch PR information on demand.`

	// Append custom commands section if any exist with show_help=true
	if len(customCommands) > 0 {
		var customKeys []string
		for key, cmd := range customCommands {
			if cmd != nil && cmd.ShowHelp {
				customKeys = append(customKeys, fmt.Sprintf("- %s: %s", key, cmd.Description))
			}
		}

		if len(customKeys) > 0 {
			sort.Strings(customKeys)
			helpText += "\n\n**⚙️ Custom Commands**\n" + strings.Join(customKeys, "\n")
		}
	}

	width := 80
	height := 30
	if maxWidth > 0 {
		width = minInt(100, maxInt(60, int(float64(maxWidth)*0.75)))
	}
	if maxHeight > 0 {
		height = minInt(40, maxInt(20, int(float64(maxHeight)*0.7)))
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
		thm:         thm,
	}

	hs.refreshContent()
	return hs
}

// NewCommandPaletteScreen builds a palette populated with candidate commands.
func NewCommandPaletteScreen(items []paletteItem, thm *theme.Theme) *CommandPaletteScreen {
	ti := textinput.New()
	ti.Placeholder = "Type a command..."
	ti.CharLimit = 100
	ti.Prompt = "> "
	ti.Focus()
	ti.Width = 78 // fits inside 80 width box with padding

	screen := &CommandPaletteScreen{
		items:        items,
		filtered:     items,
		filterInput:  ti,
		cursor:       0,
		scrollOffset: 0,
		thm:          thm,
	}
	return screen
}

// NewDiffScreen displays a commit diff title and body inside a viewport.
func NewDiffScreen(title, diff string, thm *theme.Theme) *DiffScreen {
	vp := viewport.New(100, 40)
	content := title
	if diff != "" {
		content += "\n\n" + diff
	}
	vp.SetContent(content)
	return &DiffScreen{
		title:    title,
		content:  content,
		viewport: vp,
		thm:      thm,
	}
}

// Init prepares the help screen before it starts handling updates.
func (s *HelpScreen) Init() tea.Cmd {
	return nil
}

// Init configures the palette input before Bubble Tea updates begin.
func (s *CommandPaletteScreen) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles scrolling and search input for the help screen.
func (s *HelpScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	keyMsg, ok := msg.(tea.KeyMsg)
	if ok {
		key := keyMsg.String()
		switch key {
		case "/":
			if !s.searching {
				s.searching = true
				s.searchInput.Focus()
				return s, textinput.Blink
			}
		case keyEnter:
			if s.searching {
				s.searchQuery = strings.TrimSpace(s.searchInput.Value())
				s.searching = false
				s.searchInput.Blur()
				s.refreshContent()
				return s, nil
			}
		case keyEsc, keyCtrlC:
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
		newQuery := strings.TrimSpace(s.searchInput.Value())
		if newQuery != s.searchQuery {
			s.searchQuery = newQuery
			s.refreshContent()
		}
		return s, cmd
	}

	if ok {
		key := keyMsg.String()
		switch key {
		case keyCtrlD, " ":
			s.viewport.HalfPageDown()
			return s, nil
		case keyCtrlU:
			s.viewport.HalfPageUp()
			return s, nil
		case "j", keyDown:
			s.viewport.ScrollDown(1)
			return s, nil
		case "k", keyUp:
			s.viewport.ScrollUp(1)
			return s, nil
		}
	}

	s.viewport, cmd = s.viewport.Update(msg)
	return s, cmd
}

// Update handles updates for the command palette
func (s *CommandPaletteScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	maxVisible := 12

	keyMsg, ok := msg.(tea.KeyMsg)
	if ok {
		switch keyMsg.String() {
		case keyEnter:
			return s, tea.Quit
		case keyEsc, keyCtrlC:
			s.cursor = -1
			return s, tea.Quit
		case keyUp:
			if s.cursor > 0 {
				s.cursor--
				if s.cursor < s.scrollOffset {
					s.scrollOffset = s.cursor
				}
			}
			return s, nil
		case keyDown:
			if s.cursor < len(s.filtered)-1 {
				s.cursor++
				if s.cursor >= s.scrollOffset+maxVisible {
					s.scrollOffset = s.cursor - maxVisible + 1
				}
			}
			return s, nil
		}
	}

	s.filterInput, cmd = s.filterInput.Update(msg)
	s.applyFilter()
	return s, cmd
}

func (s *CommandPaletteScreen) applyFilter() {
	s.filtered = filterPaletteItems(s.items, s.filterInput.Value())

	// Reset cursor and scroll offset if list changes
	if s.cursor >= len(s.filtered) {
		s.cursor = maxInt(0, len(s.filtered)-1)
	}
	s.scrollOffset = 0
}

// Selected reports the current palette selection if one exists.
func (s *CommandPaletteScreen) Selected() (string, bool) {
	if s.cursor < 0 || s.cursor >= len(s.filtered) {
		return "", false
	}
	return s.filtered[s.cursor].id, true
}

// NewListSelectionScreen builds a list selection screen with 80% of screen size.
func NewListSelectionScreen(items []selectionItem, title, placeholder, noResults string, maxWidth, maxHeight int, initialID string, thm *theme.Theme) *ListSelectionScreen {
	// Use 80% of screen size
	width := int(float64(maxWidth) * 0.8)
	height := int(float64(maxHeight) * 0.8)

	// Ensure minimum sizes
	if width < 60 {
		width = 60
	}
	if height < 20 {
		height = 20
	}

	if placeholder == "" {
		placeholder = "Filter..."
	}
	if noResults == "" {
		noResults = "No results found."
	}

	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.CharLimit = 100
	ti.Prompt = "> "
	ti.Focus()
	ti.Width = width - 4 // padding

	cursor := 0
	if len(items) == 0 {
		cursor = -1
	}
	if initialID != "" {
		for i, item := range items {
			if item.id == initialID {
				cursor = i
				break
			}
		}
	}

	screen := &ListSelectionScreen{
		items:        items,
		filtered:     items,
		filterInput:  ti,
		cursor:       cursor,
		scrollOffset: 0,
		width:        width,
		height:       height,
		title:        title,
		placeholder:  placeholder,
		noResults:    noResults,
		thm:          thm,
	}
	return screen
}

// NewPRSelectionScreen builds a PR selection screen with 80% of screen size.
func NewPRSelectionScreen(prs []*models.PRInfo, maxWidth, maxHeight int, thm *theme.Theme, showIcons bool) *PRSelectionScreen {
	// Use 80% of screen size
	width := int(float64(maxWidth) * 0.8)
	height := int(float64(maxHeight) * 0.8)

	// Ensure minimum sizes
	if width < 60 {
		width = 60
	}
	if height < 20 {
		height = 20
	}

	ti := textinput.New()
	ti.Placeholder = "Filter PRs by number or title..."
	ti.CharLimit = 100
	ti.Prompt = "> "
	ti.Focus()
	ti.Width = width - 4 // padding

	screen := &PRSelectionScreen{
		prs:          prs,
		filtered:     prs,
		filterInput:  ti,
		cursor:       0,
		scrollOffset: 0,
		width:        width,
		height:       height,
		thm:          thm,
		showIcons:    showIcons,
	}
	return screen
}

// Init configures the PR selection input before Bubble Tea updates begin.
func (s *PRSelectionScreen) Init() tea.Cmd {
	return textinput.Blink
}

// Init configures the list selection input before Bubble Tea updates begin.
func (s *ListSelectionScreen) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles updates for the PR selection screen.
func (s *PRSelectionScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	maxVisible := s.height - 6 // Account for header, input, footer

	keyMsg, ok := msg.(tea.KeyMsg)
	if ok {
		switch keyMsg.String() {
		case keyEnter:
			return s, tea.Quit
		case keyEsc, keyCtrlC:
			s.cursor = -1
			return s, tea.Quit
		case keyUp, keyCtrlK:
			if s.cursor > 0 {
				s.cursor--
				if s.cursor < s.scrollOffset {
					s.scrollOffset = s.cursor
				}
			}
			return s, nil
		case keyDown, keyCtrlJ:
			if s.cursor < len(s.filtered)-1 {
				s.cursor++
				if s.cursor >= s.scrollOffset+maxVisible {
					s.scrollOffset = s.cursor - maxVisible + 1
				}
			}
			return s, nil
		}
	}

	s.filterInput, cmd = s.filterInput.Update(msg)
	s.applyFilter()
	return s, cmd
}

// Update handles updates for the list selection screen.
func (s *ListSelectionScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	maxVisible := s.height - 6 // Account for header, input, footer

	keyMsg, ok := msg.(tea.KeyMsg)
	if ok {
		switch keyMsg.String() {
		case keyEnter:
			return s, tea.Quit
		case keyEsc, keyCtrlC:
			s.cursor = -1
			return s, tea.Quit
		case keyUp, keyCtrlK:
			if s.cursor > 0 {
				s.cursor--
				if s.cursor < s.scrollOffset {
					s.scrollOffset = s.cursor
				}
			}
			return s, nil
		case keyDown, keyCtrlJ:
			if s.cursor < len(s.filtered)-1 {
				s.cursor++
				if s.cursor >= s.scrollOffset+maxVisible {
					s.scrollOffset = s.cursor - maxVisible + 1
				}
			}
			return s, nil
		}
	}

	s.filterInput, cmd = s.filterInput.Update(msg)
	s.applyFilter()
	return s, cmd
}

func (s *PRSelectionScreen) applyFilter() {
	query := strings.ToLower(strings.TrimSpace(s.filterInput.Value()))
	if query == "" {
		s.filtered = s.prs
	} else {
		s.filtered = []*models.PRInfo{}
		for _, pr := range s.prs {
			// Match by number or title
			prNumStr := fmt.Sprintf("%d", pr.Number)
			titleLower := strings.ToLower(pr.Title)
			if strings.Contains(prNumStr, query) || strings.Contains(titleLower, query) {
				s.filtered = append(s.filtered, pr)
			}
		}
	}

	// Reset cursor if needed
	if s.cursor >= len(s.filtered) {
		s.cursor = maxInt(0, len(s.filtered)-1)
	}
	if s.cursor < 0 && len(s.filtered) > 0 {
		s.cursor = 0
	}
	s.scrollOffset = 0
}

func (s *ListSelectionScreen) applyFilter() {
	query := strings.ToLower(strings.TrimSpace(s.filterInput.Value()))
	if query == "" {
		s.filtered = s.items
	} else {
		s.filtered = []selectionItem{}
		for _, item := range s.items {
			labelLower := strings.ToLower(item.label)
			descLower := strings.ToLower(item.description)
			idLower := strings.ToLower(item.id)
			if strings.Contains(labelLower, query) || strings.Contains(descLower, query) || strings.Contains(idLower, query) {
				s.filtered = append(s.filtered, item)
			}
		}
	}

	// Reset cursor if needed
	if len(s.filtered) == 0 {
		s.cursor = -1
	} else if s.cursor >= len(s.filtered) || s.cursor < 0 {
		s.cursor = 0
	}
	s.scrollOffset = 0
}

// Selected returns the currently selected PR, if any.
func (s *PRSelectionScreen) Selected() (*models.PRInfo, bool) {
	if s.cursor < 0 || s.cursor >= len(s.filtered) {
		return nil, false
	}
	return s.filtered[s.cursor], true
}

// Selected returns the currently selected item, if any.
func (s *ListSelectionScreen) Selected() (selectionItem, bool) {
	if s.cursor < 0 || s.cursor >= len(s.filtered) {
		return selectionItem{}, false
	}
	return s.filtered[s.cursor], true
}

// View renders the PR selection screen.
func (s *PRSelectionScreen) View() string {
	maxVisible := s.height - 6 // Account for header, input, footer

	// Enhanced PR selection modal with rounded border
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.thm.Accent).
		Width(s.width).
		Padding(0)

	titleStyle := lipgloss.NewStyle().
		Foreground(s.thm.Accent).
		Bold(true).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(s.thm.BorderDim).
		Width(s.width-2).
		Padding(0, 1).
		Render("🔀 Select PR/MR to Create Worktree")

	inputStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(s.width - 2).
		Foreground(s.thm.TextFg)

	itemStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(s.width - 2)

	selectedStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(s.width - 2).
		Background(s.thm.Accent).
		Foreground(s.thm.TextFg).
		Bold(true)

	noResultsStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(s.width - 2).
		Foreground(s.thm.MutedFg).
		Italic(true)

	// Render Input
	inputView := inputStyle.Render(s.filterInput.View())

	// Render PRs
	var itemViews []string

	end := s.scrollOffset + maxVisible
	if end > len(s.filtered) {
		end = len(s.filtered)
	}
	start := s.scrollOffset
	if start > end {
		start = end
	}

	for i := start; i < end; i++ {
		pr := s.filtered[i]
		iconPrefix := ""
		if s.showIcons {
			iconPrefix = iconWithSpace(iconPR)
		}
		prLabel := fmt.Sprintf("%s#%d: %s", iconPrefix, pr.Number, pr.Title)

		// Truncate if too long
		maxLabelLen := s.width - 10
		if len(prLabel) > maxLabelLen {
			prLabel = prLabel[:maxLabelLen-1] + "…"
		}

		var line string
		if i == s.cursor {
			line = selectedStyle.Render(prLabel)
		} else {
			line = itemStyle.Render(prLabel)
		}
		itemViews = append(itemViews, line)
	}

	if len(s.filtered) == 0 {
		if len(s.prs) == 0 {
			itemViews = append(itemViews, noResultsStyle.Render("No open PRs/MRs found."))
		} else {
			itemViews = append(itemViews, noResultsStyle.Render("No PRs match your filter."))
		}
	}

	// Separator
	separator := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(s.thm.BorderDim).
		Width(s.width - 2).
		Render("")

	// Footer
	footerStyle := lipgloss.NewStyle().
		Foreground(s.thm.MutedFg).
		Align(lipgloss.Right).
		Width(s.width - 2).
		PaddingTop(1)
	footer := footerStyle.Render("Enter to select • Esc to cancel")

	content := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle,
		inputView,
		separator,
		strings.Join(itemViews, "\n"),
		footer,
	)

	return boxStyle.Render(content)
}

// View renders the list selection screen.
func (s *ListSelectionScreen) View() string {
	maxVisible := s.height - 6 // Account for header, input, footer

	// Enhanced list selection modal with rounded border
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.thm.Accent).
		Width(s.width).
		Padding(0)

	titleStyle := lipgloss.NewStyle().
		Foreground(s.thm.Accent).
		Bold(true).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(s.thm.BorderDim).
		Width(s.width-2).
		Padding(0, 1).
		Render(s.title)

	inputStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(s.width - 2).
		Foreground(s.thm.TextFg)

	itemStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(s.width - 2)

	selectedStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(s.width - 2).
		Background(s.thm.Accent).
		Foreground(s.thm.TextFg).
		Bold(true)

	descStyle := lipgloss.NewStyle().
		Foreground(s.thm.MutedFg)

	selectedDescStyle := lipgloss.NewStyle().
		Foreground(s.thm.TextFg)

	noResultsStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(s.width - 2).
		Foreground(s.thm.MutedFg).
		Italic(true)

	// Render Input
	inputView := inputStyle.Render(s.filterInput.View())

	// Render items
	var itemViews []string

	end := s.scrollOffset + maxVisible
	if end > len(s.filtered) {
		end = len(s.filtered)
	}
	start := s.scrollOffset
	if start > end {
		start = end
	}

	for i := start; i < end; i++ {
		item := s.filtered[i]
		label := item.label
		if item.description != "" {
			desc := item.description
			if i == s.cursor {
				desc = selectedDescStyle.Render(desc)
			} else {
				desc = descStyle.Render(desc)
			}
			label = fmt.Sprintf("%s  %s", label, desc)
		}

		var line string
		if i == s.cursor {
			line = selectedStyle.Render(label)
		} else {
			line = itemStyle.Render(label)
		}
		itemViews = append(itemViews, line)
	}

	if len(s.filtered) == 0 {
		itemViews = append(itemViews, noResultsStyle.Render(s.noResults))
	}

	// Separator
	separator := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(s.thm.BorderDim).
		Width(s.width - 2).
		Render("")

	// Footer
	footerStyle := lipgloss.NewStyle().
		Foreground(s.thm.MutedFg).
		Align(lipgloss.Right).
		Width(s.width - 2).
		PaddingTop(1)
	footer := footerStyle.Render("Enter to select • Esc to cancel")

	content := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle,
		inputView,
		separator,
		strings.Join(itemViews, "\n"),
		footer,
	)

	return boxStyle.Render(content)
}

// Init sets up the diff viewport state before rendering.
func (s *DiffScreen) Init() tea.Cmd {
	return nil
}

// Update processes navigation keys while the diff is visible.
func (s *DiffScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (s *HelpScreen) refreshContent() {
	content := s.renderContent()
	s.viewport.SetContent(content)
	s.viewport.GotoTop()
}

// SetSize updates the help screen dimensions (useful on terminal resize)
func (s *HelpScreen) SetSize(maxWidth, maxHeight int) {
	width := 80
	height := 30
	if maxWidth > 0 {
		width = minInt(100, maxInt(60, int(float64(maxWidth)*0.75)))
	}
	if maxHeight > 0 {
		height = minInt(40, maxInt(20, int(float64(maxHeight)*0.7)))
	}
	s.width = width
	s.height = height

	// Update viewport size
	// height - 4 for borders/header/footer
	s.viewport.Width = s.width - 2
	s.viewport.Height = maxInt(5, s.height-4)
}

func (s *HelpScreen) renderContent() string {
	lines := s.fullText

	// Apply styling to help content
	styledLines := []string{}
	titleStyle := lipgloss.NewStyle().Foreground(s.thm.Accent).Bold(true)
	keyStyle := lipgloss.NewStyle().Foreground(s.thm.SuccessFg).Bold(true)

	for _, line := range lines {
		// Style section headers (lines that start with ** and end with **)
		if strings.HasPrefix(line, "**") && strings.HasSuffix(line, "**") {
			header := strings.TrimPrefix(strings.TrimSuffix(line, "**"), "**")
			styledLines = append(styledLines, titleStyle.Render("▶ "+header))
			continue
		}

		// Style key bindings (lines starting with "- " and containing ": ")
		if strings.HasPrefix(line, "- ") {
			// Split on ": " (colon + space) to handle keys that contain ":"
			parts := strings.SplitN(line, ": ", 2)
			if len(parts) == 2 {
				keys := strings.TrimPrefix(parts[0], "- ")
				description := parts[1]
				styledLine := "  " + keyStyle.Render(keys) + ": " + description
				styledLines = append(styledLines, styledLine)
				continue
			}
		}

		styledLines = append(styledLines, line)
	}

	// Handle search filtering
	if strings.TrimSpace(s.searchQuery) != "" {
		query := strings.ToLower(strings.TrimSpace(s.searchQuery))
		highlightStyle := lipgloss.NewStyle().Foreground(s.thm.TextFg).Background(s.thm.Accent).Bold(true)
		filteredLines := []string{}
		for _, line := range styledLines {
			lower := strings.ToLower(line)
			if strings.Contains(lower, query) {
				filteredLines = append(filteredLines, highlightMatches(line, lower, query, highlightStyle))
			}
		}

		if len(filteredLines) == 0 {
			return fmt.Sprintf("No help entries match %q", s.searchQuery)
		}
		return strings.Join(filteredLines, "\n")
	}

	return strings.Join(styledLines, "\n")
}

func highlightMatches(line, lowerLine, lowerQuery string, style lipgloss.Style) string {
	if lowerQuery == "" {
		return line
	}

	var b strings.Builder
	searchFrom := 0
	qLen := len(lowerQuery)

	for {
		idx := strings.Index(lowerLine[searchFrom:], lowerQuery)
		if idx < 0 {
			b.WriteString(line[searchFrom:])
			break
		}
		start := searchFrom + idx
		end := start + qLen
		b.WriteString(line[searchFrom:start])
		b.WriteString(style.Render(line[start:end]))
		searchFrom = end
	}

	return b.String()
}

// View renders the help content and search input inside the viewport.
func (s *HelpScreen) View() string {
	content := s.renderContent()

	// Keep viewport sized to available area (minus header/search lines)
	vHeight := maxInt(5, s.height-4) // -4 for borders/header/footer
	s.viewport.Width = s.width - 2   // -2 for borders
	s.viewport.Height = vHeight
	s.viewport.SetContent(content)

	// Enhanced help modal with rounded border
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.thm.Accent).
		Width(s.width).
		Padding(0)

	titleStyle := lipgloss.NewStyle().
		Foreground(s.thm.Accent).
		Bold(true).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(s.thm.BorderDim).
		Width(s.width-2).
		Padding(0, 1).
		Render("❓ Help")

	// Search bar styling
	searchView := ""
	if s.searching || s.searchQuery != "" {
		searchView = lipgloss.NewStyle().
			Width(s.width-2).
			Padding(0, 1).
			Render(s.searchInput.View())

		// Add separator after search
		searchView += "\n" + lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(s.thm.BorderDim).
			Width(s.width-2).
			Render("")
	}

	// Footer
	footerStyle := lipgloss.NewStyle().
		Foreground(s.thm.MutedFg).
		Align(lipgloss.Left).
		Width(s.width - 2).
		PaddingTop(1)
	footer := footerStyle.Render("j/k: scroll • Ctrl+d/u: page • /: search • esc: close")

	// Viewport styling
	vpStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(s.width - 2)

	body := vpStyle.Render(s.viewport.View())

	contentBlock := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle,
		searchView,
		body,
		footer,
	)

	return boxStyle.Render(contentBlock)
}

// View displays the palette items and current filter text.
func (s *CommandPaletteScreen) View() string {
	width := 110
	maxVisible := 12

	// Enhanced palette modal with rounded border
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.thm.Accent).
		Width(width).
		Padding(0)

	inputStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(width - 2).
		Foreground(s.thm.TextFg)

	itemStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(width - 2)

	selectedStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(width - 2).
		Background(s.thm.Accent).
		Foreground(s.thm.TextFg).
		Bold(true)

	descStyle := lipgloss.NewStyle().
		Foreground(s.thm.MutedFg)

	selectedDescStyle := lipgloss.NewStyle().
		Foreground(s.thm.TextFg)

	noResultsStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(width - 2).
		Foreground(s.thm.MutedFg).
		Italic(true)

	// Render Input
	inputView := inputStyle.Render(s.filterInput.View())

	// Render Items
	var itemViews []string

	end := s.scrollOffset + maxVisible
	if end > len(s.filtered) {
		end = len(s.filtered)
	}
	start := s.scrollOffset
	if start > end {
		start = end // Should not happen if logic is correct
	}

	for i := start; i < end; i++ {
		it := s.filtered[i]

		// Truncate label if too long
		label := it.label
		desc := it.description

		// Pad label to align descriptions somewhat
		labelPad := 45
		if len(label) > labelPad {
			label = label[:labelPad-1] + "…"
		}
		paddedLabel := fmt.Sprintf("%-45s", label)

		var line string
		if i == s.cursor {
			line = fmt.Sprintf("%s %s", paddedLabel, selectedDescStyle.Render(desc))
			itemViews = append(itemViews, selectedStyle.Render(line))
		} else {
			line = fmt.Sprintf("%s %s", paddedLabel, descStyle.Render(desc))
			itemViews = append(itemViews, itemStyle.Render(line))
		}
	}

	if len(s.filtered) == 0 {
		itemViews = append(itemViews, noResultsStyle.Render("No commands match your filter."))
	}

	// Separator
	separator := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(s.thm.BorderDim).
		Width(width - 2).
		Render("")

	// Footer
	footerStyle := lipgloss.NewStyle().
		Foreground(s.thm.MutedFg).
		Align(lipgloss.Right).
		Width(width - 2).
		PaddingTop(1)
	footer := footerStyle.Render("esc to close")

	content := lipgloss.JoinVertical(lipgloss.Left,
		inputView,
		separator,
		strings.Join(itemViews, "\n"),
		footer,
	)

	return boxStyle.Render(content)
}

// View renders the diff text inside a scrollable viewport.
func (s *DiffScreen) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(s.thm.Accent)
	title := titleStyle.Render("📄 " + s.title)

	content := lipgloss.JoinVertical(lipgloss.Left, title, "", s.viewport.View())
	// Enhanced diff view with rounded border
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.thm.Accent).
		Padding(1, 2).
		Width(maxInt(80, s.viewport.Width)).
		Render(content)
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

// Update listens for retry or quit keys on the welcome screen.
func (s *WelcomeScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if ok {
		switch keyMsg.String() {
		case "r", "R":
			s.result <- true
			return s, tea.Quit
		case keyQ, "Q", keyEsc, keyCtrlC:
			s.result <- false
			return s, tea.Quit
		}
	}
	return s, nil
}

// View renders the welcome dialog with guidance and action buttons.
func (s *WelcomeScreen) View() string {
	width := 70
	height := 20

	// Enhanced welcome screen with rounded border
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.thm.Accent).
		Padding(1, 2).
		Width(width).
		Height(height)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(s.thm.Accent).
		Align(lipgloss.Center).
		MarginBottom(1)

	welcomeIcon := "👋"

	messageStyle := lipgloss.NewStyle().
		Align(lipgloss.Center).
		MarginBottom(2)

	buttonStyle := lipgloss.NewStyle().
		Width(20).
		Align(lipgloss.Center).
		Padding(0, 1).
		Margin(0, 1)

	quitButton := buttonStyle.
		Foreground(s.thm.ErrorFg).
		Render("[Quit]")

	retryButton := buttonStyle.
		Foreground(s.thm.Accent).
		Render("[Retry]")

	message := fmt.Sprintf(" No worktrees found.\n\n Current Directory: %s\n Worktree Root: %s\n\nPlease ensure you are in a git repository or the configured worktree root.\nYou may need to initialize a repository or configure 'worktree_dir' in config.",
		s.currentDir,
		s.worktreeDir,
	)

	content := fmt.Sprintf("%s\n%s\n\n%s  %s",
		titleStyle.Render(welcomeIcon+" Welcome to LazyWorktree"),
		messageStyle.Render(message),
		quitButton,
		retryButton,
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

// CommitFileTreeNode represents a node in the commit file tree.
type CommitFileTreeNode struct {
	Path        string
	File        *models.CommitFile // nil for directories
	Children    []*CommitFileTreeNode
	Compression int // Number of compressed path segments
	depth       int // Cached depth for rendering
}

// IsDir returns true if this node is a directory.
func (n *CommitFileTreeNode) IsDir() bool {
	return n.File == nil
}

// CommitFilesScreen displays files changed in a commit as a collapsible tree.
type CommitFilesScreen struct {
	commitSHA     string
	worktreePath  string
	files         []models.CommitFile
	allFiles      []models.CommitFile // Original unfiltered files
	tree          *CommitFileTreeNode
	treeFlat      []*CommitFileTreeNode
	collapsedDirs map[string]bool
	cursor        int
	scrollOffset  int
	width         int
	height        int
	thm           *theme.Theme
	showIcons     bool
	// Commit metadata
	commitMeta commitMeta
	// Filter/search support
	filterInput   textinput.Model
	showingFilter bool
	filterQuery   string
	showingSearch bool
	searchQuery   string
}

// NewCommitFilesScreen creates a commit files tree screen.
func NewCommitFilesScreen(sha, wtPath string, files []models.CommitFile, meta commitMeta, maxWidth, maxHeight int, thm *theme.Theme, showIcons bool) *CommitFilesScreen {
	width := int(float64(maxWidth) * 0.8)
	height := int(float64(maxHeight) * 0.8)
	if width < 60 {
		width = 60
	}
	if height < 20 {
		height = 20
	}

	ti := textinput.New()
	ti.Placeholder = placeholderFilterFiles
	ti.CharLimit = 100
	ti.Prompt = "> "
	ti.Width = width - 6

	screen := &CommitFilesScreen{
		commitSHA:     sha,
		worktreePath:  wtPath,
		files:         files,
		allFiles:      files,
		collapsedDirs: make(map[string]bool),
		cursor:        0,
		scrollOffset:  0,
		width:         width,
		height:        height,
		thm:           thm,
		showIcons:     showIcons,
		commitMeta:    meta,
		filterInput:   ti,
	}

	screen.tree = buildCommitFileTree(files)
	sortCommitFileTree(screen.tree)
	compressCommitFileTree(screen.tree)
	screen.rebuildFlat()

	return screen
}

// buildCommitFileTree constructs a tree from a flat list of commit files.
func buildCommitFileTree(files []models.CommitFile) *CommitFileTreeNode {
	root := &CommitFileTreeNode{
		Path:     "",
		Children: []*CommitFileTreeNode{},
	}

	for i := range files {
		file := &files[i]
		parts := strings.Split(file.Filename, "/")

		current := root
		for j := range parts {
			isLast := j == len(parts)-1
			partPath := strings.Join(parts[:j+1], "/")

			// Find or create child
			var child *CommitFileTreeNode
			for _, c := range current.Children {
				if c.Path == partPath {
					child = c
					break
				}
			}

			if child == nil {
				if isLast {
					child = &CommitFileTreeNode{
						Path: partPath,
						File: file,
					}
				} else {
					child = &CommitFileTreeNode{
						Path:     partPath,
						Children: []*CommitFileTreeNode{},
					}
				}
				current.Children = append(current.Children, child)
			}
			current = child
		}
	}

	return root
}

// sortCommitFileTree sorts nodes: directories first, then files, alphabetically.
func sortCommitFileTree(node *CommitFileTreeNode) {
	if node == nil || len(node.Children) == 0 {
		return
	}

	sort.Slice(node.Children, func(i, j int) bool {
		iIsDir := node.Children[i].IsDir()
		jIsDir := node.Children[j].IsDir()
		if iIsDir != jIsDir {
			return iIsDir
		}
		return node.Children[i].Path < node.Children[j].Path
	})

	for _, child := range node.Children {
		sortCommitFileTree(child)
	}
}

// compressCommitFileTree compresses single-child directory chains.
func compressCommitFileTree(node *CommitFileTreeNode) {
	if node == nil {
		return
	}

	for _, child := range node.Children {
		compressCommitFileTree(child)
	}

	if node.IsDir() && len(node.Children) == 1 {
		child := node.Children[0]
		if child.IsDir() {
			node.Compression++
			node.Compression += child.Compression
			node.Children = child.Children
		}
	}
}

// rebuildFlat rebuilds the flat list from the tree respecting collapsed state.
func (s *CommitFilesScreen) rebuildFlat() {
	s.treeFlat = []*CommitFileTreeNode{}
	s.flattenTree(s.tree, 0)
}

// applyFilter filters the files list and rebuilds the tree.
func (s *CommitFilesScreen) applyFilter() {
	query := strings.ToLower(strings.TrimSpace(s.filterQuery))
	if query == "" {
		s.files = s.allFiles
	} else {
		s.files = nil
		for _, f := range s.allFiles {
			if strings.Contains(strings.ToLower(f.Filename), query) {
				s.files = append(s.files, f)
			}
		}
	}

	// Rebuild tree from filtered files
	s.tree = buildCommitFileTree(s.files)
	sortCommitFileTree(s.tree)
	compressCommitFileTree(s.tree)
	s.rebuildFlat()

	// Clamp cursor
	if s.cursor >= len(s.treeFlat) {
		s.cursor = maxInt(0, len(s.treeFlat)-1)
	}
	s.scrollOffset = 0
}

// searchNext finds the next match for the search query.
func (s *CommitFilesScreen) searchNext(forward bool) {
	if s.searchQuery == "" || len(s.treeFlat) == 0 {
		return
	}

	query := strings.ToLower(s.searchQuery)
	start := s.cursor
	n := len(s.treeFlat)

	for i := 1; i <= n; i++ {
		var idx int
		if forward {
			idx = (start + i) % n
		} else {
			idx = (start - i + n) % n
		}

		node := s.treeFlat[idx]
		name := node.Path
		if parts := strings.Split(node.Path, "/"); len(parts) > 0 {
			name = parts[len(parts)-1]
		}

		if strings.Contains(strings.ToLower(name), query) {
			s.cursor = idx
			// Adjust scroll offset
			maxVisible := s.height - 8
			if s.cursor < s.scrollOffset {
				s.scrollOffset = s.cursor
			} else if s.cursor >= s.scrollOffset+maxVisible {
				s.scrollOffset = s.cursor - maxVisible + 1
			}
			return
		}
	}
}

func (s *CommitFilesScreen) flattenTree(node *CommitFileTreeNode, depth int) {
	if node == nil {
		return
	}

	for _, child := range node.Children {
		child.depth = depth
		s.treeFlat = append(s.treeFlat, child)

		if child.IsDir() && !s.collapsedDirs[child.Path] {
			s.flattenTree(child, depth+1)
		}
	}
}

// GetSelectedNode returns the currently selected node.
func (s *CommitFilesScreen) GetSelectedNode() *CommitFileTreeNode {
	if s.cursor < 0 || s.cursor >= len(s.treeFlat) {
		return nil
	}
	return s.treeFlat[s.cursor]
}

// ToggleCollapse toggles the collapse state of a directory.
func (s *CommitFilesScreen) ToggleCollapse(path string) {
	s.collapsedDirs[path] = !s.collapsedDirs[path]
	s.rebuildFlat()
	if s.cursor >= len(s.treeFlat) {
		s.cursor = maxInt(0, len(s.treeFlat)-1)
	}
}

// Init implements tea.Model.
func (s *CommitFilesScreen) Init() tea.Cmd {
	return nil
}

// Update handles key events for the commit files screen.
func (s *CommitFilesScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}

	maxVisible := s.height - 8 // Account for header, footer, borders
	keyStr := keyMsg.String()

	// Handle filter mode
	if s.showingFilter {
		switch keyStr {
		case keyEnter:
			s.showingFilter = false
			s.filterInput.Blur()
			return s, nil
		case keyEsc, keyCtrlC:
			s.showingFilter = false
			s.filterQuery = ""
			s.filterInput.SetValue("")
			s.filterInput.Blur()
			s.applyFilter()
			return s, nil
		case keyUp, keyCtrlK:
			if s.cursor > 0 {
				s.cursor--
				if s.cursor < s.scrollOffset {
					s.scrollOffset = s.cursor
				}
			}
			return s, nil
		case keyDown, keyCtrlJ:
			if s.cursor < len(s.treeFlat)-1 {
				s.cursor++
				if s.cursor >= s.scrollOffset+maxVisible {
					s.scrollOffset = s.cursor - maxVisible + 1
				}
			}
			return s, nil
		}

		// Update filter input
		var cmd tea.Cmd
		s.filterInput, cmd = s.filterInput.Update(msg)
		newQuery := s.filterInput.Value()
		if newQuery != s.filterQuery {
			s.filterQuery = newQuery
			s.applyFilter()
		}
		return s, cmd
	}

	// Handle search mode
	if s.showingSearch {
		switch keyStr {
		case keyEnter:
			s.showingSearch = false
			s.filterInput.Blur()
			return s, nil
		case keyEsc, keyCtrlC:
			s.showingSearch = false
			s.searchQuery = ""
			s.filterInput.SetValue("")
			s.filterInput.Blur()
			return s, nil
		case "n":
			s.searchNext(true)
			return s, nil
		case "N":
			s.searchNext(false)
			return s, nil
		}

		// Update search input and jump to first match
		var cmd tea.Cmd
		s.filterInput, cmd = s.filterInput.Update(msg)
		newQuery := s.filterInput.Value()
		if newQuery != s.searchQuery {
			s.searchQuery = newQuery
			// Jump to first match from current position
			if s.searchQuery != "" {
				s.searchNext(true)
			}
		}
		return s, cmd
	}

	// Normal navigation
	switch keyStr {
	case "j", keyDown:
		if s.cursor < len(s.treeFlat)-1 {
			s.cursor++
			if s.cursor >= s.scrollOffset+maxVisible {
				s.scrollOffset = s.cursor - maxVisible + 1
			}
		}
	case "k", keyUp:
		if s.cursor > 0 {
			s.cursor--
			if s.cursor < s.scrollOffset {
				s.scrollOffset = s.cursor
			}
		}
	case keyCtrlD, " ":
		s.cursor = minInt(s.cursor+maxVisible/2, len(s.treeFlat)-1)
		if s.cursor >= s.scrollOffset+maxVisible {
			s.scrollOffset = s.cursor - maxVisible + 1
		}
	case keyCtrlU:
		s.cursor = maxInt(s.cursor-maxVisible/2, 0)
		if s.cursor < s.scrollOffset {
			s.scrollOffset = s.cursor
		}
	case "g":
		s.cursor = 0
		s.scrollOffset = 0
	case "G":
		s.cursor = maxInt(0, len(s.treeFlat)-1)
		if s.cursor >= maxVisible {
			s.scrollOffset = s.cursor - maxVisible + 1
		}
	case "n":
		if s.searchQuery != "" {
			s.searchNext(true)
		}
	case "N":
		if s.searchQuery != "" {
			s.searchNext(false)
		}
	}

	return s, nil
}

// View renders the commit files screen.
func (s *CommitFilesScreen) View() string {
	// Calculate header height: title (1) + metadata (variable) + stats (1) + filter/search (1 if active) + footer (1) + borders (2)
	headerHeight := 5 // title + stats + footer + borders
	if s.commitMeta.sha != "" || s.commitMeta.author != "" || s.commitMeta.date != "" || s.commitMeta.subject != "" {
		// Estimate metadata height: commit line + author line + date line + blank + subject = ~5 lines
		metaHeight := 1 // commit line
		if s.commitMeta.author != "" {
			metaHeight++
		}
		if s.commitMeta.date != "" {
			metaHeight++
		}
		if s.commitMeta.subject != "" {
			metaHeight += 2 // blank + subject
		}
		headerHeight += metaHeight
	}
	if s.showingFilter || s.showingSearch {
		headerHeight++
	}
	maxVisible := s.height - headerHeight

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.thm.Accent).
		Width(s.width).
		Height(s.height).
		Padding(0)

	titleStyle := lipgloss.NewStyle().
		Foreground(s.thm.Accent).
		Bold(true).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(s.thm.BorderDim).
		Width(s.width-2).
		Padding(0, 1)

	shortSHA := s.commitSHA
	if len(shortSHA) > 8 {
		shortSHA = shortSHA[:8]
	}
	title := titleStyle.Render(fmt.Sprintf("Files in commit %s", shortSHA))

	// Render commit metadata
	metaStyle := lipgloss.NewStyle().
		Foreground(s.thm.MutedFg).
		Width(s.width-2).
		Padding(0, 1).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(s.thm.BorderDim)
	labelStyle := lipgloss.NewStyle().Foreground(s.thm.MutedFg).Bold(true)
	valueStyle := lipgloss.NewStyle().Foreground(s.thm.TextFg)
	subjectStyle := lipgloss.NewStyle().Bold(true).Foreground(s.thm.Accent)

	var metaLines []string
	if s.commitMeta.sha != "" {
		metaLines = append(metaLines, fmt.Sprintf("%s %s", labelStyle.Render("Commit:"), valueStyle.Render(s.commitMeta.sha)))
	}
	if s.commitMeta.author != "" {
		authorLine := fmt.Sprintf("%s %s", labelStyle.Render("Author:"), valueStyle.Render(s.commitMeta.author))
		if s.commitMeta.email != "" {
			authorLine += fmt.Sprintf(" <%s>", valueStyle.Render(s.commitMeta.email))
		}
		metaLines = append(metaLines, authorLine)
	}
	if s.commitMeta.date != "" {
		metaLines = append(metaLines, fmt.Sprintf("%s %s", labelStyle.Render("Date:"), valueStyle.Render(s.commitMeta.date)))
	}
	if s.commitMeta.subject != "" {
		if len(metaLines) > 0 {
			metaLines = append(metaLines, "")
		}
		metaLines = append(metaLines, subjectStyle.Render(s.commitMeta.subject))
	}
	commitMetaSection := ""
	if len(metaLines) > 0 {
		commitMetaSection = metaStyle.Render(strings.Join(metaLines, "\n"))
	}

	// Render file tree
	var itemViews []string

	end := s.scrollOffset + maxVisible
	if end > len(s.treeFlat) {
		end = len(s.treeFlat)
	}
	start := s.scrollOffset
	if start > end {
		start = end
	}

	itemStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(s.width - 2)

	// Inline highlight style - no width, no padding
	highlightStyle := lipgloss.NewStyle().
		Background(s.thm.Accent).
		Foreground(s.thm.TextFg).
		Bold(true)

	dirStyle := lipgloss.NewStyle().
		Foreground(s.thm.Accent)

	fileStyle := lipgloss.NewStyle().
		Foreground(s.thm.TextFg)

	changeTypeStyle := lipgloss.NewStyle().
		Foreground(s.thm.MutedFg)

	noFilesStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(s.width - 2).
		Foreground(s.thm.MutedFg).
		Italic(true)

	for i := start; i < end; i++ {
		node := s.treeFlat[i]
		indent := strings.Repeat("  ", node.depth)
		isSelected := i == s.cursor
		iconName := node.Path
		if parts := strings.Split(node.Path, "/"); len(parts) > 0 {
			iconName = parts[len(parts)-1]
		}
		devicon := ""
		if s.showIcons {
			devicon = iconWithSpace(deviconForName(iconName, node.IsDir()))
		}

		var label string
		if node.IsDir() {
			icon := "▼"
			if s.collapsedDirs[node.Path] {
				icon = "▶"
			}
			// Show just the last part of the path for display
			displayPath := node.Path
			if parts := strings.Split(node.Path, "/"); len(parts) > 0 {
				displayPath = parts[len(parts)-1]
				if node.Compression > 0 && len(parts) > node.Compression {
					displayPath = strings.Join(parts[len(parts)-node.Compression-1:], "/")
				}
			}
			displayLabel := devicon + displayPath
			// Apply highlight only to directory name
			if isSelected {
				label = fmt.Sprintf("%s%s %s/", indent, icon, highlightStyle.Render(displayLabel))
			} else {
				label = fmt.Sprintf("%s%s %s/", indent, icon, dirStyle.Render(displayLabel))
			}
		} else {
			// Show just the filename
			displayName := node.Path
			if parts := strings.Split(node.Path, "/"); len(parts) > 0 {
				displayName = parts[len(parts)-1]
			}
			displayLabel := devicon + displayName
			changeIndicator := ""
			if node.File != nil {
				switch node.File.ChangeType {
				case "A":
					changeIndicator = changeTypeStyle.Render(" [+]")
				case "D":
					changeIndicator = changeTypeStyle.Render(" [-]")
				case "M":
					changeIndicator = changeTypeStyle.Render(" [~]")
				case "R":
					changeIndicator = changeTypeStyle.Render(" [R]")
				case "C":
					changeIndicator = changeTypeStyle.Render(" [C]")
				}
			}
			// Apply highlight only to filename
			if isSelected {
				label = fmt.Sprintf("%s  %s%s", indent, highlightStyle.Render(displayLabel), changeIndicator)
			} else {
				label = fmt.Sprintf("%s  %s%s", indent, fileStyle.Render(displayLabel), changeIndicator)
			}
		}

		itemViews = append(itemViews, itemStyle.Render(label))
	}

	if len(s.treeFlat) == 0 {
		itemViews = append(itemViews, noFilesStyle.Render("No files in this commit."))
	}

	// Footer
	footerStyle := lipgloss.NewStyle().
		Foreground(s.thm.MutedFg).
		Width(s.width-2).
		Padding(0, 1).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(s.thm.BorderDim)

	footerText := "j/k: navigate • Enter: toggle/view diff • d: full diff • f: filter • /: search • q: close"
	if s.showingFilter {
		footerText = "↑↓: navigate • Enter: apply filter • Esc: clear filter"
	} else if s.showingSearch {
		footerText = "n/N: next/prev match • Enter: close search • Esc: clear search"
	}
	footer := footerStyle.Render(footerText)

	// Stats line
	statsStyle := lipgloss.NewStyle().
		Foreground(s.thm.MutedFg).
		Width(s.width-2).
		Padding(0, 1).
		Align(lipgloss.Right)

	statsText := fmt.Sprintf("%d files", len(s.files))
	if s.filterQuery != "" {
		statsText = fmt.Sprintf("%d/%d files (filtered)", len(s.files), len(s.allFiles))
	}
	stats := statsStyle.Render(statsText)

	// Build content sections
	sections := []string{title}
	if commitMetaSection != "" {
		sections = append(sections, commitMetaSection)
	}

	// Add filter/search input if active
	if s.showingFilter || s.showingSearch {
		inputStyle := lipgloss.NewStyle().
			Padding(0, 1).
			Width(s.width - 2).
			Foreground(s.thm.TextFg)
		sections = append(sections, inputStyle.Render(s.filterInput.View()))
	}

	sections = append(sections, stats, strings.Join(itemViews, "\n"), footer)

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	return boxStyle.Render(content)
}
