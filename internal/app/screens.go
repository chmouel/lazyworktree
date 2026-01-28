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
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/theme"
)

type screenType int

const (
	screenNone screenType = iota
	screenConfirm
	screenInfo
	screenInput
	screenTrust
	screenWelcome
	screenCommit
	screenDiff
	screenPRSelect
	screenIssueSelect
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
