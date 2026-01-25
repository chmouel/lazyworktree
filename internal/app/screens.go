package app

import (
	"fmt"
	"math/rand/v2"
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
	screenIssueSelect
	screenListSelect
	screenLoading
	screenCommitFiles
	screenChecklist

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
	checkboxEnabled     bool     // Whether checkbox is displayed
	checkboxLabel       string   // Label text for checkbox
	checkboxChecked     bool     // Current checkbox state
	checkboxFocused     bool     // Track whether checkbox has focus (vs input field)
	showIcons           bool
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
	showIcons   bool
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
	width        int
	height       int
	thm          *theme.Theme
}

type paletteItem struct {
	id          string
	label       string
	description string
	isSection   bool
	isMRU       bool
}

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

// ChecklistScreen lets the user select multiple items from a list via checkboxes.
type ChecklistScreen struct {
	items        []ChecklistItem
	filtered     []ChecklistItem
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

// IssueSelectionScreen lets the user pick an issue from a filtered list.
type IssueSelectionScreen struct {
	issues       []*models.IssueInfo
	filtered     []*models.IssueInfo
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

	// Callback for selection change (used for live preview)
	onCursorChange func(selectionItem)
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

// NewConfirmScreen creates a confirm screen preloaded with a message.
func NewConfirmScreen(message string, thm *theme.Theme) *ConfirmScreen {
	return &ConfirmScreen{
		message:        message,
		result:         make(chan bool, 1),
		selectedButton: 0, // Start with Confirm button focused
		thm:            thm,
	}
}

// NewConfirmScreenWithDefault creates a confirmation modal with a specified default button.
func NewConfirmScreenWithDefault(message string, defaultButton int, thm *theme.Theme) *ConfirmScreen {
	return &ConfirmScreen{
		message:        message,
		result:         make(chan bool, 1),
		selectedButton: defaultButton, // Use provided default
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

// NewChecklistScreen creates a multi-select checklist screen.
func NewChecklistScreen(items []ChecklistItem, title, placeholder, noResults string, maxWidth, maxHeight int, thm *theme.Theme) *ChecklistScreen {
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
		noResults = "No items found."
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

	screen := &ChecklistScreen{
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

// Init configures the checklist input before Bubble Tea updates begin.
func (s *ChecklistScreen) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles keyboard events for the checklist screen.
func (s *ChecklistScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	maxVisible := (s.height - 6) / 2 // Account for header, input, footer; divide by 2 since each item takes 2 lines

	keyMsg, ok := msg.(tea.KeyMsg)
	if ok {
		switch keyMsg.String() {
		case keyEnter:
			return s, tea.Quit
		case keyEsc, keyCtrlC:
			// Clear all selections on cancel
			for i := range s.items {
				s.items[i].Checked = false
			}
			s.applyFilter()
			return s, tea.Quit
		case keyUp, "k", keyCtrlK:
			if s.cursor > 0 {
				s.cursor--
				if s.cursor < s.scrollOffset {
					s.scrollOffset = s.cursor
				}
			}
			return s, nil
		case keyDown, "j", keyCtrlJ:
			if s.cursor < len(s.filtered)-1 {
				s.cursor++
				if s.cursor >= s.scrollOffset+maxVisible {
					s.scrollOffset = s.cursor - maxVisible + 1
				}
			}
			return s, nil
		case " ":
			// Toggle current item
			if s.cursor >= 0 && s.cursor < len(s.filtered) {
				// Find the item in the original list and toggle it
				id := s.filtered[s.cursor].ID
				for i := range s.items {
					if s.items[i].ID == id {
						s.items[i].Checked = !s.items[i].Checked
						break
					}
				}
				s.applyFilter()
			}
			return s, nil
		case "a":
			// Select all filtered items
			for _, f := range s.filtered {
				for i := range s.items {
					if s.items[i].ID == f.ID {
						s.items[i].Checked = true
						break
					}
				}
			}
			s.applyFilter()
			return s, nil
		case "n":
			// Deselect all filtered items
			for _, f := range s.filtered {
				for i := range s.items {
					if s.items[i].ID == f.ID {
						s.items[i].Checked = false
						break
					}
				}
			}
			s.applyFilter()
			return s, nil
		}
	}

	s.filterInput, cmd = s.filterInput.Update(msg)
	s.applyFilter()
	return s, cmd
}

func (s *ChecklistScreen) applyFilter() {
	query := strings.ToLower(strings.TrimSpace(s.filterInput.Value()))
	if query == "" {
		// Rebuild filtered from items to reflect checkbox changes
		s.filtered = make([]ChecklistItem, len(s.items))
		copy(s.filtered, s.items)
	} else {
		s.filtered = []ChecklistItem{}
		for _, item := range s.items {
			labelLower := strings.ToLower(item.Label)
			descLower := strings.ToLower(item.Description)
			idLower := strings.ToLower(item.ID)
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

// SelectedItems returns all checked items.
func (s *ChecklistScreen) SelectedItems() []ChecklistItem {
	var selected []ChecklistItem
	for _, item := range s.items {
		if item.Checked {
			selected = append(selected, item)
		}
	}
	return selected
}

// View renders the checklist screen.
func (s *ChecklistScreen) View() string {
	maxVisible := (s.height - 6) / 2 // Account for header, input, footer; divide by 2 since each item takes 2 lines

	// Enhanced checklist modal with rounded border
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
		Foreground(s.thm.AccentFg).
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

	end := min(s.scrollOffset+maxVisible, len(s.filtered))
	start := s.scrollOffset
	if start > end {
		start = end
	}

	for i := start; i < end; i++ {
		item := s.filtered[i]

		// Checkbox prefix
		checkbox := "[ ] "
		if item.Checked {
			checkbox = "[x] "
		}

		// First line: checkbox + label
		label := checkbox + item.Label
		var line string
		if i == s.cursor {
			line = selectedStyle.Render(label)
		} else {
			line = itemStyle.Render(label)
		}
		itemViews = append(itemViews, line)

		// Second line: description (indented)
		if item.Description != "" {
			indent := "    " // 4 spaces to align with label after checkbox
			desc := indent + item.Description
			var descLine string
			if i == s.cursor {
				descLine = selectedStyle.Render(selectedDescStyle.Render(desc))
			} else {
				descLine = itemStyle.Render(descStyle.Render(desc))
			}
			itemViews = append(itemViews, descLine)
		}
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

	// Count selected
	selectedCount := 0
	for _, item := range s.items {
		if item.Checked {
			selectedCount++
		}
	}

	// Footer
	footerStyle := lipgloss.NewStyle().
		Foreground(s.thm.MutedFg).
		Align(lipgloss.Right).
		Width(s.width - 2).
		PaddingTop(1)
	footer := footerStyle.Render(fmt.Sprintf("%d selected • Space toggle • a/n all/none • Enter confirm • Esc cancel", selectedCount))

	content := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle,
		inputView,
		separator,
		strings.Join(itemViews, "\n"),
		footer,
	)

	return boxStyle.Render(content)
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
	case keyTab, "right", "l":
		s.selectedButton = (s.selectedButton + 1) % 2
	case keyShiftTab, "left", "h":
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
		Foreground(s.thm.AccentFg).
		Background(s.thm.ErrorFg).
		Bold(true)

	// Focused cancel button
	focusedCancelStyle := lipgloss.NewStyle().
		Width((width-6)/2).
		Align(lipgloss.Center).
		Padding(0, 2).
		Foreground(s.thm.AccentFg).
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
		Foreground(s.thm.AccentFg).
		Background(s.thm.Accent).
		Bold(true)

	content := fmt.Sprintf("%s\n\n%s",
		messageStyle.Render(s.message),
		okStyle.Render("[OK]"),
	)

	return boxStyle.Render(content)
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

// NewInputScreen builds an input modal with prompt, placeholder, and initial value.
func NewInputScreen(prompt, placeholder, value string, thm *theme.Theme, showIcons bool) *InputScreen {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.SetValue(value)
	ti.Focus()
	ti.CharLimit = 200
	ti.Prompt = ""
	ti.TextStyle = lipgloss.NewStyle().Foreground(thm.TextFg)

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
		showIcons:           showIcons,
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
		case keyTab:
			// Switch focus between input and checkbox
			if s.checkboxEnabled {
				s.checkboxFocused = !s.checkboxFocused
				return s, nil
			}
		case keyShiftTab:
			// Switch focus in reverse
			if s.checkboxEnabled {
				s.checkboxFocused = !s.checkboxFocused
				return s, nil
			}
		case " ":
			// Only toggle checkbox if it's enabled AND focused
			if s.checkboxEnabled && s.checkboxFocused {
				s.checkboxChecked = !s.checkboxChecked
				return s, nil
			}
			// Otherwise, let the textinput handle the space (fall through to textinput.Update)
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
		Padding(0, 1).
		Width(width - 6)

	// Use brighter border when input has focus, dimmer when checkbox is focused
	if s.checkboxEnabled && s.checkboxFocused {
		inputWrapperStyle = inputWrapperStyle.BorderForeground(s.thm.BorderDim)
	} else {
		inputWrapperStyle = inputWrapperStyle.BorderForeground(s.thm.Border)
	}

	footerStyle := lipgloss.NewStyle().
		Foreground(s.thm.MutedFg).
		Width(width - 6).
		Align(lipgloss.Center).
		MarginTop(1)

	contentLines := []string{
		promptStyle.Render(s.prompt),
	}

	// Show checkbox if enabled
	if s.checkboxEnabled {
		checkbox := "[ ] "
		if s.checkboxChecked {
			checkbox = "[x] "
		}

		checkboxStyle := lipgloss.NewStyle().
			Width(width - 6).
			MarginTop(1)

		if s.checkboxFocused {
			// Highlight background when focused (like fuzzy finder selections)
			checkboxStyle = checkboxStyle.
				Background(s.thm.Accent).
				Foreground(s.thm.AccentFg).
				Padding(0, 1).
				Bold(true)
		} else {
			// Normal styling when unfocused
			checkboxStyle = checkboxStyle.Foreground(s.thm.Accent)
		}

		contentLines = append(contentLines, checkboxStyle.Render(checkbox+s.checkboxLabel))
	}

	contentLines = append(contentLines, inputWrapperStyle.Render(s.input.View()))

	// Show fuzzy finder suggestions if enabled
	if s.fuzzyFinderInput && len(s.filteredSuggestions) > 0 {
		suggestionsStyle := lipgloss.NewStyle().
			Foreground(s.thm.MutedFg).
			Width(width - 6).
			MarginTop(1)

		selectedStyle := lipgloss.NewStyle().
			Foreground(s.thm.AccentFg).
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

	var footerText string
	switch {
	case s.checkboxEnabled:
		footerText = "Tab to switch focus • Space to toggle • Enter to confirm • Esc to cancel"
	case len(s.history) > 0:
		footerText = fmt.Sprintf("%s to navigate history • Enter confirm • Esc cancel", arrowPair(s.showIcons))
	case s.fuzzyFinderInput && len(s.filteredSuggestions) > 0:
		footerText = fmt.Sprintf("%s to navigate • Enter confirm • Esc cancel", arrowPair(s.showIcons))
	default:
		footerText = "Enter to confirm • Esc to cancel"
	}
	contentLines = append(contentLines, footerStyle.Render(footerText))

	content := strings.Join(contentLines, "\n\n")

	return boxStyle.Render(content)
}

// NewHelpScreen initializes help content with the available screen size.
func NewHelpScreen(maxWidth, maxHeight int, customCommands map[string]*config.CustomCommand, thm *theme.Theme, showIcons bool) *HelpScreen {
	helpTextTemplate := `{{HELP_TITLE}}LazyWorktree Help Guide

**{{HELP_NAV}}Navigation**
- j / {{ARROW_DOWN}}: Move cursor down
- k / {{ARROW_UP}}: Move cursor up
- 1 / 2 / 3: Switch to pane (or toggle zoom if already focused)
- [ / ]: Previous / Next pane
- Tab: Cycle to next pane
- Enter: Jump to selected worktree (exit and cd)

**{{HELP_STATUS_PANE}}Status Pane (when focused)**
- j / k: Navigate files and directories
- Enter: Show diff for selected file in pager
- e: Open selected file in editor
- d: Show full diff (all files) in pager
- s: Stage/unstage selected file or directory
- D: Delete selected file or directory (with confirmation)
- c: Commit staged changes
- C: Stage all changes and commit
- Ctrl+{{ARROW_LEFT}} / {{ARROW_RIGHT}}: Jump to previous / next folder
- /: Search file names
- Ctrl+D / Space: Half page down
- Ctrl+U: Half page up
- PageUp / PageDown: Full page up/down
- g / G: Jump to top / bottom

**{{HELP_LOG}}Log Pane**
- j / k: Move between commits
- Ctrl+J: Next commit and open file tree
- Enter: Open commit file tree (browse changed files)
- C: Cherry-pick commit to another worktree
- /: Search commit titles

**{{HELP_COMMIT_TREE}}Commit File Tree (viewing files in a commit)**
- j / k: Navigate files and directories
- Enter: Toggle directory or show file diff
- d: Show full commit diff in pager
- f: Filter files by name
- /: Search files (incremental)
- n / N: Next / previous search match
- q / Esc: Return to commit log

**{{HELP_WORKTREE_ACTIONS}}Worktree Actions**
- c: Create new worktree (branch, commit, PR/MR, issue, or custom)
- Create from current: supply an explicit name or leave empty for auto-generation
- Existing local branch: choose to checkout the branch or create a new one based on it
- Tab / Shift+Tab: Move focus to the "Include current file changes" checkbox
- Space: Toggle "Include current file changes"
- m: Rename selected worktree
- D: Delete selected worktree
- A: Absorb worktree into main (merge + delete)
- X: Prune merged worktrees (auto-refreshes PR data, then checks PR/branch merge status)
- !: Run arbitrary command in selected worktree

**{{HELP_BRANCH_NAMING}}Branch Naming**
Special characters in branch names are automatically converted to hyphens for compatibility with Git and terminal multiplexers. Examples:
- "feature.new" {{ARROW_RIGHT}} "feature-new"
- "bug fix here" {{ARROW_RIGHT}} "bug-fix-here"
- "path/to/branch" {{ARROW_RIGHT}} "path-to-branch"
Supported: Letters (a-z, A-Z), numbers (0-9), and hyphens (-). See help for full details.

**{{HELP_VIEWING_TOOLS}}Viewing & Tools**
- d: Full-screen diff viewer
- o: Open PR/MR in browser
- g: Open LazyGit (or go to top in diff pane)
- =: Toggle zoom for focused pane
- : / Ctrl+P: Command Palette
- ?: Show this help

**{{HELP_REPO_OPS}}Repository Operations**
- r: Refresh worktree list
- R: Fetch all remotes
- S: Synchronise with upstream (git pull, then git push, current branch only, requires a clean worktree, honours merge_method)
- P: Push to upstream branch (current branch only, requires a clean worktree, prompts to set upstream when missing)
- p: Fetch PR/MR status from GitHub/GitLab
- v: View CI check logs (select from available checks)
- s: Cycle sort (Path / Last Active / Last Switched)

**{{HELP_BACKGROUND_REFRESH}}Background Refresh**
- Configured via auto_refresh and refresh_interval in the configuration file

**{{HELP_FILTERING_SEARCH}}Filtering & Search**
- f: Filter focused pane
- /: Search focused pane (incremental)
- Alt+N / Alt+P: Move selection and fill filter input
- {{ARROW_UP}} / {{ARROW_DOWN}}: Move selection (filter active, no fill)
- Ctrl+J / Ctrl+K: Same as above
- Home / End: Jump to first / last item

Search Mode:
- Type: Jump to first matching item
- n / N: Next / previous match
- Enter: Close search
- Esc: Clear search

**{{HELP_STATUS_INDICATORS}}Status Indicators**
- -: Clean and synced with remote
- {{STATUS_DIRTY}}: Uncommitted changes (dirty)
- {{STATUS_AHEAD}}N: Ahead of remote by N commits
- {{STATUS_BEHIND}}N: Behind remote by N commits
- {{STATUS_DIRTY}} {{STATUS_AHEAD}}N: Dirty and ahead by N commits

**{{HELP_HELP_NAVIGATION}}Help Navigation**
- /: Search help (Enter to apply, Esc to clear)
- q / Esc: Close help
- j / k: Scroll up / down
- Ctrl+D / Ctrl+U: Scroll half page down / up

**{{HELP_SHELL_COMPLETION}}Shell Completion**
Generate completions: lazyworktree --completion <bash|zsh|fish>

**{{HELP_CONFIGURATION}}Configuration & Overrides**
Configuration is read from multiple sources (in order of precedence):
1. CLI overrides (highest): lazyworktree --config=lw.key=value
2. Git local config: git config --local lw.key value
3. Git global config: git config --global lw.key value
4. YAML file: ~/.config/lazyworktree/config.yaml
5. Built-in defaults (lowest)

Example: lazyworktree --config=lw.theme=nord --config=lw.auto_fetch_prs=true

Custom themes: define custom_themes in the configuration file. Without a base theme, all 11 colour fields are required.

**{{HELP_ICON_CONFIGURATION}}Icon Configuration**
- icon_set: Choose icon set ("nerd-font-v3", "emoji", "text", "none"). Default: "nerd-font-v3". Applies to file icons, UI indicators, and help headings. The text set uses plain labels. Set to "none" or an empty value to suppress icons.

{{HELP_TIP}}Tip: PR data is not fetched by default for speed.
       Press 'p' to fetch PR information on demand.`

	replacer := strings.NewReplacer(
		"{{HELP_TITLE}}", iconPrefix(UIIconHelpTitle, showIcons),
		"{{HELP_NAV}}", iconPrefix(UIIconNavigation, showIcons),
		"{{HELP_STATUS_PANE}}", iconPrefix(UIIconStatusPane, showIcons),
		"{{HELP_LOG}}", iconPrefix(UIIconLogPane, showIcons),
		"{{HELP_COMMIT_TREE}}", iconPrefix(UIIconCommitTree, showIcons),
		"{{HELP_WORKTREE_ACTIONS}}", iconPrefix(UIIconWorktreeActions, showIcons),
		"{{HELP_BRANCH_NAMING}}", iconPrefix(UIIconBranchNaming, showIcons),
		"{{HELP_VIEWING_TOOLS}}", iconPrefix(UIIconViewingTools, showIcons),
		"{{HELP_REPO_OPS}}", iconPrefix(UIIconRepoOps, showIcons),
		"{{HELP_BACKGROUND_REFRESH}}", iconPrefix(UIIconBackgroundRefresh, showIcons),
		"{{HELP_FILTERING_SEARCH}}", iconPrefix(UIIconFilterSearch, showIcons),
		"{{HELP_STATUS_INDICATORS}}", iconPrefix(UIIconStatusIndicators, showIcons),
		"{{HELP_HELP_NAVIGATION}}", iconPrefix(UIIconHelpNavigation, showIcons),
		"{{HELP_SHELL_COMPLETION}}", iconPrefix(UIIconShellCompletion, showIcons),
		"{{HELP_CONFIGURATION}}", iconPrefix(UIIconConfiguration, showIcons),
		"{{HELP_ICON_CONFIGURATION}}", iconPrefix(UIIconIconConfiguration, showIcons),
		"{{HELP_TIP}}", iconPrefix(UIIconTip, showIcons),
		"{{STATUS_CLEAN}}", statusIndicator(true, showIcons),
		"{{STATUS_DIRTY}}", statusIndicator(false, showIcons),
		"{{STATUS_AHEAD}}", aheadIndicator(showIcons),
		"{{STATUS_BEHIND}}", behindIndicator(showIcons),
		"{{ARROW_UP}}", arrowUp(showIcons),
		"{{ARROW_DOWN}}", arrowDown(showIcons),
		"{{ARROW_LEFT}}", arrowLeft(showIcons),
		"{{ARROW_RIGHT}}", arrowRight(showIcons),
	)

	helpText := replacer.Replace(helpTextTemplate)

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
			customTitle := labelWithIcon(UIIconConfiguration, "Custom Commands", showIcons)
			helpText += "\n\n**" + customTitle + "**\n" + strings.Join(customKeys, "\n")
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
		showIcons:   showIcons,
	}

	hs.refreshContent()
	return hs
}

// NewCommandPaletteScreen builds a palette populated with candidate commands.
func NewCommandPaletteScreen(items []paletteItem, maxWidth, maxHeight int, thm *theme.Theme) *CommandPaletteScreen {
	// Calculate palette width: 80% of screen, capped between 60 and 110
	width := int(float64(maxWidth) * 0.8)
	width = max(60, min(110, width))

	ti := textinput.New()
	ti.Placeholder = "Type a command..."
	ti.CharLimit = 100
	ti.Prompt = "> "
	ti.Focus()
	ti.Width = width - 4 // fits inside box with padding

	// Find first non-section item for initial cursor
	initialCursor := 0
	for i, item := range items {
		if !item.isSection {
			initialCursor = i
			break
		}
	}

	screen := &CommandPaletteScreen{
		items:        items,
		filtered:     items,
		filterInput:  ti,
		cursor:       initialCursor,
		scrollOffset: 0,
		width:        width,
		height:       maxHeight,
		thm:          thm,
	}
	return screen
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
				// Skip sections when navigating
				for s.cursor > 0 && s.filtered[s.cursor].isSection {
					s.cursor--
				}
				if s.cursor < s.scrollOffset {
					s.scrollOffset = s.cursor
				}
			}
			return s, nil
		case keyDown:
			if s.cursor < len(s.filtered)-1 {
				s.cursor++
				// Skip sections when navigating
				for s.cursor < len(s.filtered)-1 && s.filtered[s.cursor].isSection {
					s.cursor++
				}
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

	// Find first non-section item for cursor
	for s.cursor < len(s.filtered) && s.filtered[s.cursor].isSection {
		s.cursor++
	}
	if s.cursor >= len(s.filtered) {
		s.cursor = 0
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
				if s.onCursorChange != nil {
					if item, ok := s.Selected(); ok {
						s.onCursorChange(item)
					}
				}
			}
			return s, nil
		case keyDown, keyCtrlJ:
			if s.cursor < len(s.filtered)-1 {
				s.cursor++
				if s.cursor >= s.scrollOffset+maxVisible {
					s.scrollOffset = s.cursor - maxVisible + 1
				}
				if s.onCursorChange != nil {
					if item, ok := s.Selected(); ok {
						s.onCursorChange(item)
					}
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
		Render(labelWithIcon(UIIconPRSelect, "Select PR/MR to Create Worktree", s.showIcons))

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
		Foreground(s.thm.AccentFg).
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

	// Calculate column widths for display
	// Layout: [icon] #number author CI title
	prNumWidth := 6
	authorWidth := min(12, max(8, (s.width-30)/5))
	ciWidth := 2
	iconWidth := 0
	if s.showIcons {
		iconWidth = 3
	}
	// Title gets remaining space
	titleWidth := s.width - prNumWidth - authorWidth - ciWidth - iconWidth - 10

	for i := start; i < end; i++ {
		pr := s.filtered[i]

		// Format PR number
		prNum := fmt.Sprintf("#%-5d", pr.Number)

		// Format author (truncate if needed)
		author := pr.Author
		if len(author) > authorWidth {
			author = author[:authorWidth-1] + "…"
		}
		authorFmt := fmt.Sprintf("%-*s", authorWidth, author)

		// Format CI status icon (draft takes precedence)
		ciIcon := getCIStatusIcon(pr.CIStatus, pr.IsDraft, s.showIcons)

		// Format title (truncate if needed)
		title := pr.Title
		if len(title) > titleWidth {
			title = title[:titleWidth-1] + "…"
		}

		// Build the label
		iconPrefix := ""
		if s.showIcons {
			iconPrefix = iconWithSpace(getIconPR())
		}
		prLabel := fmt.Sprintf("%s%s %s %s %s", iconPrefix, prNum, authorFmt, ciIcon, title)

		var line string
		if i == s.cursor {
			line = selectedStyle.Render(prLabel)
		} else {
			// Apply color to CI icon based on status
			line = s.renderPRLine(itemStyle, iconPrefix, prNum, authorFmt, ciIcon, title, pr.CIStatus, pr.IsDraft)
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

// getCIStatusIcon returns the appropriate icon for CI status.
// Draft PRs show "D" instead of CI status per user preference.
func getCIStatusIcon(ciStatus string, isDraft, showIcons bool) string {
	if isDraft {
		return "D"
	}
	if showIcons {
		if icon := ciIconForConclusion(ciStatus); icon != "" {
			return icon
		}
	}
	switch ciStatus {
	case "success":
		return "S"
	case "failure":
		return "F"
	case "skipped":
		return "-"
	case "cancelled":
		return "C"
	case "pending":
		return "P"
	default:
		return "?"
	}
}

// renderPRLine renders a PR line with colored CI status icon.
func (s *PRSelectionScreen) renderPRLine(baseStyle lipgloss.Style, iconPrefix, prNum, author, ciIcon, title, ciStatus string, isDraft bool) string {
	// Style for CI icon based on status
	var ciStyle lipgloss.Style
	if isDraft {
		ciStyle = lipgloss.NewStyle().Foreground(s.thm.MutedFg)
	} else {
		switch ciStatus {
		case "success":
			ciStyle = lipgloss.NewStyle().Foreground(s.thm.SuccessFg)
		case "failure":
			ciStyle = lipgloss.NewStyle().Foreground(s.thm.ErrorFg)
		case "pending":
			ciStyle = lipgloss.NewStyle().Foreground(s.thm.WarnFg)
		default:
			ciStyle = lipgloss.NewStyle().Foreground(s.thm.MutedFg)
		}
	}

	// Build the line with colored CI icon
	line := fmt.Sprintf("%s%s %s %s %s", iconPrefix, prNum, author, ciStyle.Render(ciIcon), title)
	return baseStyle.Render(line)
}

// NewIssueSelectionScreen builds an issue selection screen with 80% of screen size.
func NewIssueSelectionScreen(issues []*models.IssueInfo, maxWidth, maxHeight int, thm *theme.Theme, showIcons bool) *IssueSelectionScreen {
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
	ti.Placeholder = "Filter issues by number or title..."
	ti.CharLimit = 100
	ti.Prompt = "> "
	ti.Focus()
	ti.Width = width - 4 // padding

	screen := &IssueSelectionScreen{
		issues:       issues,
		filtered:     issues,
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

// Init configures the issue selection input before Bubble Tea updates begin.
func (s *IssueSelectionScreen) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles updates for the issue selection screen.
func (s *IssueSelectionScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (s *IssueSelectionScreen) applyFilter() {
	query := strings.ToLower(strings.TrimSpace(s.filterInput.Value()))
	if query == "" {
		s.filtered = s.issues
	} else {
		s.filtered = []*models.IssueInfo{}
		for _, issue := range s.issues {
			// Match by number or title
			issueNumStr := fmt.Sprintf("%d", issue.Number)
			titleLower := strings.ToLower(issue.Title)
			if strings.Contains(issueNumStr, query) || strings.Contains(titleLower, query) {
				s.filtered = append(s.filtered, issue)
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

// Selected returns the currently selected issue, if any.
func (s *IssueSelectionScreen) Selected() (*models.IssueInfo, bool) {
	if s.cursor < 0 || s.cursor >= len(s.filtered) {
		return nil, false
	}
	return s.filtered[s.cursor], true
}

// View renders the issue selection screen.
func (s *IssueSelectionScreen) View() string {
	maxVisible := s.height - 6 // Account for header, input, footer

	// Enhanced issue selection modal with rounded border
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
		Render(labelWithIcon(UIIconIssueSelect, "Select Issue to Create Worktree", s.showIcons))

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
		Foreground(s.thm.AccentFg).
		Bold(true)

	noResultsStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(s.width - 2).
		Foreground(s.thm.MutedFg).
		Italic(true)

	// Render Input
	inputView := inputStyle.Render(s.filterInput.View())

	// Render Issues
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
		issue := s.filtered[i]
		iconPrefix := ""
		if s.showIcons {
			iconPrefix = iconWithSpace(getIconIssue())
		}
		issueLabel := fmt.Sprintf("%s#%d: %s", iconPrefix, issue.Number, issue.Title)

		// Truncate if too long
		maxLabelLen := s.width - 10
		if len(issueLabel) > maxLabelLen {
			issueLabel = issueLabel[:maxLabelLen-1] + "…"
		}

		var line string
		if i == s.cursor {
			line = selectedStyle.Render(issueLabel)
		} else {
			line = itemStyle.Render(issueLabel)
		}
		itemViews = append(itemViews, line)
	}

	if len(s.filtered) == 0 {
		if len(s.issues) == 0 {
			itemViews = append(itemViews, noResultsStyle.Render("No open issues found."))
		} else {
			itemViews = append(itemViews, noResultsStyle.Render("No issues match your filter."))
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
		Foreground(s.thm.AccentFg).
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
			prefix := disclosureIndicator(false, s.showIcons)
			styledLines = append(styledLines, titleStyle.Render(prefix+" "+header))
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
		highlightStyle := lipgloss.NewStyle().Foreground(s.thm.AccentFg).Background(s.thm.Accent).Bold(true)
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
	width := s.width
	if width == 0 {
		width = 110 // fallback for tests
	}

	// Calculate maxVisible based on available height
	// Reserve: 1 input + 1 separator + 1 footer + 2 border = ~5 lines
	maxVisible := s.height - 5
	maxVisible = max(5, min(20, maxVisible))
	if s.height == 0 {
		maxVisible = 12 // fallback for tests
	}

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
		Foreground(s.thm.AccentFg).
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

	sectionStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(width - 2).
		Foreground(s.thm.Accent).
		Bold(true)

	for i := start; i < end; i++ {
		it := s.filtered[i]

		// Render section headers differently
		if it.isSection {
			itemViews = append(itemViews, sectionStyle.Render("── "+it.label+" ──"))
			continue
		}

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

// SetCheckbox enables a checkbox in the input screen with the given label and default state.
func (s *InputScreen) SetCheckbox(label string, defaultChecked bool) {
	s.checkboxEnabled = true
	s.checkboxLabel = label
	s.checkboxChecked = defaultChecked
	s.checkboxFocused = false // Default: input field has focus
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
	for _, child := range screen.tree.Children {
		compressCommitFileTree(child)
	}
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
	case "f":
		s.showingFilter = true
		s.showingSearch = false
		s.filterInput.Placeholder = placeholderFilterFiles
		s.filterInput.Focus()
		s.filterInput.SetValue(s.filterQuery)
		return s, textinput.Blink
	case "/":
		s.showingSearch = true
		s.showingFilter = false
		s.filterInput.Placeholder = searchFiles
		s.filterInput.Focus()
		s.filterInput.SetValue(s.searchQuery)
		return s, textinput.Blink
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
		Foreground(s.thm.AccentFg).
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
			icon := disclosureIndicator(s.collapsedDirs[node.Path], s.showIcons)
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
		footerText = fmt.Sprintf("%s: navigate • Enter: apply filter • Esc: clear filter", arrowPair(s.showIcons))
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
