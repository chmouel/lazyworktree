package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/git"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/security"
)

// Message types for the Bubble Tea app
type (
	errMsg             struct{ err error }
	worktreesLoadedMsg struct {
		worktrees []*models.WorktreeInfo
		err       error
	}
	prDataLoadedMsg struct {
		prMap map[string]*models.PRInfo
		err   error
	}
	statusUpdatedMsg struct {
		info   string
		status string
		log    []commitLogEntry
	}
	refreshCompleteMsg  struct{}
	debouncedDetailsMsg struct {
		selectedIndex int
	}
	pruneResultMsg struct {
		worktrees []*models.WorktreeInfo
		err       error
		pruned    int
		failed    int
	}
)

type commitLogEntry struct {
	sha     string
	message string
}

const (
	minLeftPaneWidth  = 32
	minRightPaneWidth = 32
)

var (
	colorAccent    = lipgloss.Color("141") // soft magenta/purple
	colorAccentDim = lipgloss.Color("255") // vibrant cyan
	colorBorder    = lipgloss.Color("241")
	colorBorderDim = lipgloss.Color("238")
	colorMutedFg   = lipgloss.Color("250")
	colorTextFg    = lipgloss.Color("255")
	colorSuccessFg = lipgloss.Color("48")  // vibrant green
	colorWarnFg    = lipgloss.Color("214") // vibrant orange
	colorErrorFg   = lipgloss.Color("196") // vibrant red
)

// AppModel represents the main application model
type AppModel struct {
	// Configuration
	config *config.AppConfig
	git    *git.Service

	// UI Components
	worktreeTable  table.Model
	statusViewport viewport.Model
	logTable       table.Model
	filterInput    textinput.Model

	// State
	worktrees     []*models.WorktreeInfo
	filteredWts   []*models.WorktreeInfo
	selectedIndex int
	filterQuery   string
	sortByActive  bool
	prDataLoaded  bool
	repoKey       string
	currentScreen screenType
	helpScreen    *HelpScreen
	trustScreen   *TrustScreen
	inputScreen   *InputScreen
	inputSubmit   func(string) (tea.Cmd, bool)
	commitScreen  *CommitScreen
	welcomeScreen *WelcomeScreen
	showingFilter bool
	focusedPane   int // 0=table, 1=status, 2=log
	windowWidth   int
	windowHeight  int
	infoContent   string
	statusContent string

	// Cache
	cache           map[string]interface{}
	divergenceCache map[string]string
	notifiedErrors  map[string]bool

	// Services
	trustManager *security.TrustManager

	// Context
	ctx    context.Context
	cancel context.CancelFunc

	// Debouncing
	detailUpdateCancel  context.CancelFunc
	pendingDetailsIndex int

	// Confirm callbacks
	confirmAction  func() tea.Cmd
	confirmMessage string

	// Trust / repo commands
	repoConfig      *config.RepoConfig
	repoConfigPath  string
	pendingCommands []string
	pendingCmdEnv   map[string]string
	pendingCmdCwd   string
	pendingAfter    tea.Cmd
	pendingTrust    string

	// Log cache for commit detail viewer
	logEntries []commitLogEntry

	// Exit
	selectedPath string
	quitting     bool
}

// NewAppModel creates a new application model
func NewAppModel(cfg *config.AppConfig, initialFilter string) *AppModel {
	ctx, cancel := context.WithCancel(context.Background())

	notify := func(msg string, severity string) {
		// Notification handling - can be enhanced later
	}
	notifyOnce := func(key string, msg string, severity string) {
		// One-time notification handling
	}

	gitService := git.NewService(notify, notifyOnce)
	trustManager := security.NewTrustManager()

	// Initialize table
	columns := []table.Column{
		{Title: "Worktree", Width: 20},
		{Title: "Status", Width: 8},
		{Title: "±", Width: 10},
		{Title: "PR", Width: 15},
		{Title: "Last Active", Width: 20},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(5),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		Foreground(colorMutedFg).
		Bold(true)
	s.Cell = s.Cell.Foreground(colorTextFg)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("232")).
		Background(colorAccent).
		Bold(true)
	t.SetStyles(s)

	// Initialize status viewport
	statusVp := viewport.New(40, 5)
	statusVp.SetContent("Loading...")

	// Initialize log table
	logColumns := []table.Column{
		{Title: "SHA", Width: 10},
		{Title: "Message", Width: 50},
	}
	logT := table.New(
		table.WithColumns(logColumns),
		table.WithHeight(5),
	)
	logStyles := s
	logStyles.Header = logStyles.Header.
		Foreground(colorMutedFg).
		Bold(true)
	logStyles.Selected = logStyles.Selected.
		Foreground(colorAccent).
		Background(lipgloss.Color("238")).
		Bold(true)
	logT.SetStyles(logStyles)

	// Initialize filter input
	filterInput := textinput.New()
	filterInput.Placeholder = "Filter worktrees..."
	filterInput.Width = 50

	m := &AppModel{
		config:          cfg,
		git:             gitService,
		worktreeTable:   t,
		statusViewport:  statusVp,
		logTable:        logT,
		filterInput:     filterInput,
		worktrees:       []*models.WorktreeInfo{},
		filteredWts:     []*models.WorktreeInfo{},
		sortByActive:    cfg.SortByActive,
		filterQuery:     initialFilter,
		cache:           make(map[string]interface{}),
		divergenceCache: make(map[string]string),
		notifiedErrors:  make(map[string]bool),
		trustManager:    trustManager,
		ctx:             ctx,
		cancel:          cancel,
		focusedPane:     0,
		infoContent:     "No worktree selected.",
		statusContent:   "Loading...",
	}

	if initialFilter != "" {
		m.showingFilter = true
		m.filterInput.SetValue(initialFilter)
	}

	return m
}

// Init initializes the app
func (m *AppModel) Init() tea.Cmd {
	return tea.Batch(
		m.loadCache(),
		m.refreshWorktrees(),
	)
}

// Update handles messages
func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.setWindowSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		if m.currentScreen != screenNone {
			// Handle screen-specific keys
			return m.handleScreenKey(msg)
		}

		switch msg.String() {
		case "ctrl+c", "q":
			if m.selectedPath != "" {
				return m, tea.Quit
			}
			m.quitting = true
			return m, tea.Quit

		case "1":
			m.focusedPane = 0
			m.worktreeTable.Focus()
			return m, nil

		case "2":
			m.focusedPane = 1
			return m, nil

		case "3":
			m.focusedPane = 2
			m.logTable.Focus()
			return m, nil

		case "tab":
			m.focusedPane = (m.focusedPane + 1) % 3
			switch m.focusedPane {
			case 0:
				m.worktreeTable.Focus()
			case 2:
				m.logTable.Focus()
			}
			return m, nil

		case "j", "down":
			switch m.focusedPane {
			case 0:
				m.worktreeTable, cmd = m.worktreeTable.Update(msg)
				cmds = append(cmds, cmd)
				cmds = append(cmds, m.debouncedUpdateDetailsView())
			case 1:
				m.statusViewport, cmd = m.statusViewport.Update(msg)
				cmds = append(cmds, cmd)
			default:
				m.logTable, cmd = m.logTable.Update(msg)
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)

		case "k", "up":
			switch m.focusedPane {
			case 0:
				m.worktreeTable, cmd = m.worktreeTable.Update(msg)
				cmds = append(cmds, cmd)
				cmds = append(cmds, m.debouncedUpdateDetailsView())
			case 1:
				m.statusViewport, cmd = m.statusViewport.Update(msg)
				cmds = append(cmds, cmd)
			default:
				m.logTable, cmd = m.logTable.Update(msg)
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)

		case "ctrl+d", " ": // Page down
			switch m.focusedPane {
			case 1:
				m.statusViewport.HalfPageDown()
				return m, nil
			case 2:
				m.logTable, cmd = m.logTable.Update(msg)
				return m, cmd
			}
			return m, nil

		case "ctrl+u": // Page up
			switch m.focusedPane {
			case 1:
				m.statusViewport.HalfPageUp()
				return m, nil
			case 2:
				m.logTable, cmd = m.logTable.Update(msg)
				return m, cmd
			}
			return m, nil

		case "pgdown":
			switch m.focusedPane {
			case 1:
				m.statusViewport.PageDown()
				return m, nil
			case 2:
				m.logTable, cmd = m.logTable.Update(msg)
				return m, cmd
			}
			return m, nil

		case "pgup":
			switch m.focusedPane {
			case 1:
				m.statusViewport.PageUp()
				return m, nil
			case 2:
				m.logTable, cmd = m.logTable.Update(msg)
				return m, cmd
			}
			return m, nil

		case "G": // Go to bottom (shift+g)
			if m.focusedPane == 1 {
				m.statusViewport.GotoBottom()
				return m, nil
			}
			return m, nil

		case "enter":
			switch m.focusedPane {
			case 0:
				// Jump to worktree
				if m.selectedIndex >= 0 && m.selectedIndex < len(m.filteredWts) {
					m.selectedPath = m.filteredWts[m.selectedIndex].Path
					return m, tea.Quit
				}
			case 2:
				// Open commit view
				return m, m.openCommitView()
			}
			return m, nil

		case "r":
			return m, m.refreshWorktrees()

		case "c":
			return m, m.showCreateWorktree()

		case "D":
			return m, m.showDeleteWorktree()

		case "d":
			return m, m.showDiff()

		case "p":
			if !m.prDataLoaded {
				return m, m.fetchPRData()
			}
			return m, nil

		case "f":
			return m, m.fetchRemotes()

		case "s":
			m.sortByActive = !m.sortByActive
			m.updateTable()
			return m, nil

		case "/":
			m.showingFilter = true
			m.filterInput.Focus()
			return m, textinput.Blink

		case "?":
			m.currentScreen = screenHelp
			m.helpScreen = NewHelpScreen(m.windowWidth, m.windowHeight)
			return m, nil

		case "g":
			// If in status pane, go to top, otherwise open lazygit
			if m.focusedPane == 1 {
				m.statusViewport.GotoTop()
				return m, nil
			}
			return m, m.openLazyGit()

		case "o":
			return m, m.openPR()

		case "m":
			return m, m.showRenameWorktree()

		case "A":
			return m, m.showAbsorbWorktree()

		case "X":
			return m, m.showPruneMerged()

		case "esc":
			if m.showingFilter {
				m.showingFilter = false
				m.filterInput.Blur()
				return m, nil
			}
			return m, nil
		}

		// Handle filter input
		if m.showingFilter {
			switch msg.String() {
			case "enter", "esc":
				m.showingFilter = false
				m.filterInput.Blur()
				m.worktreeTable.Focus()
				return m, nil
			default:
				m.filterInput, cmd = m.filterInput.Update(msg)
				m.filterQuery = m.filterInput.Value()
				m.updateTable()
				return m, cmd
			}
		}

		// Handle table input
		if m.focusedPane == 0 {
			m.worktreeTable, cmd = m.worktreeTable.Update(msg)
			return m, tea.Batch(cmd, m.debouncedUpdateDetailsView())
		}

	case worktreesLoadedMsg:
		if msg.err != nil {
			// Handle error
			return m, nil
		}
		m.ensureRepoConfig()
		m.worktrees = msg.worktrees
		m.updateTable()
		m.saveCache()
		if len(m.worktrees) == 0 {
			cwd, _ := os.Getwd()
			m.welcomeScreen = NewWelcomeScreen(cwd, m.getWorktreeDir())
			m.currentScreen = screenWelcome
			return m, nil
		}
		if m.currentScreen == screenWelcome {
			m.currentScreen = screenNone
			m.welcomeScreen = nil
		}
		if m.config.AutoFetchPRs && !m.prDataLoaded {
			return m, m.fetchPRData()
		}
		return m, m.updateDetailsView()

	case prDataLoadedMsg:
		if msg.err == nil && msg.prMap != nil {
			for _, wt := range m.worktrees {
				if pr, ok := msg.prMap[wt.Branch]; ok {
					wt.PR = pr
				}
			}
			m.prDataLoaded = true
			m.updateTable()
		}
		return m, nil

	case statusUpdatedMsg:
		if msg.info != "" {
			m.infoContent = msg.info
		}
		m.statusContent = msg.status
		// Update log table
		rows := make([]table.Row, 0, len(msg.log))
		for _, entry := range msg.log {
			rows = append(rows, table.Row{entry.sha, entry.message})
		}
		m.logTable.SetRows(rows)
		m.logEntries = msg.log
		return m, nil

	case debouncedDetailsMsg:
		// Only update if the index matches and is still valid
		if msg.selectedIndex == m.worktreeTable.Cursor() &&
			msg.selectedIndex >= 0 && msg.selectedIndex < len(m.filteredWts) {
			return m, m.updateDetailsView()
		}
		return m, nil

	case errMsg:
		if msg.err != nil {
			m.statusContent = fmt.Sprintf("Error: %v", msg.err)
		}
		return m, nil

	case pruneResultMsg:
		if msg.err == nil && msg.worktrees != nil {
			m.worktrees = msg.worktrees
			m.updateTable()
			m.saveCache()
		}
		summary := fmt.Sprintf("Pruned %d merged worktrees", msg.pruned)
		if msg.failed > 0 {
			summary = fmt.Sprintf("%s (%d failed)", summary, msg.failed)
		}
		m.statusContent = summary
		return m, nil
	}

	return m, tea.Batch(cmds...)
}

// View renders the UI
func (m *AppModel) View() string {
	if m.quitting {
		return ""
	}

	// Wait for window size before rendering full UI
	if m.windowWidth == 0 || m.windowHeight == 0 {
		return "Loading..."
	}

	if m.currentScreen != screenNone {
		return m.renderScreen()
	}

	layout := m.computeLayout()
	m.applyLayout(layout)

	header := m.renderHeader(layout)
	footer := m.renderFooter(layout)
	body := m.renderBody(layout)

	// Truncate body to fit, leaving room for header and footer
	maxBodyLines := m.windowHeight - 2 // 1 for header, 1 for footer
	if layout.filterHeight > 0 {
		maxBodyLines--
	}
	body = truncateToHeight(body, maxBodyLines)

	sections := []string{header}
	if layout.filterHeight > 0 {
		sections = append(sections, m.renderFilter(layout))
	}
	sections = append(sections, body, footer)
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// Helper methods

func (m *AppModel) updateTable() {
	// Filter worktrees
	query := strings.ToLower(strings.TrimSpace(m.filterQuery))
	m.filteredWts = []*models.WorktreeInfo{}

	if query == "" {
		m.filteredWts = m.worktrees
	} else {
		hasPathSep := strings.Contains(query, "/")
		for _, wt := range m.worktrees {
			name := filepath.Base(wt.Path)
			if wt.IsMain {
				name = "main"
			}
			haystacks := []string{strings.ToLower(name), strings.ToLower(wt.Branch)}
			if hasPathSep {
				haystacks = append(haystacks, strings.ToLower(wt.Path))
			}
			for _, haystack := range haystacks {
				if strings.Contains(haystack, query) {
					m.filteredWts = append(m.filteredWts, wt)
					break
				}
			}
		}
	}

	// Sort
	if m.sortByActive {
		sort.Slice(m.filteredWts, func(i, j int) bool {
			return m.filteredWts[i].LastActiveTS > m.filteredWts[j].LastActiveTS
		})
	} else {
		sort.Slice(m.filteredWts, func(i, j int) bool {
			return m.filteredWts[i].Path < m.filteredWts[j].Path
		})
	}

	// Update table rows
	rows := make([]table.Row, 0, len(m.filteredWts))
	for _, wt := range m.filteredWts {
		name := filepath.Base(wt.Path)
		if wt.IsMain {
			name = "main"
		}

		status := "✔"
		if wt.Dirty {
			status = "✎"
		}

		abStr := ""
		if wt.Ahead > 0 {
			abStr += fmt.Sprintf("↑%d ", wt.Ahead)
		}
		if wt.Behind > 0 {
			abStr += fmt.Sprintf("↓%d ", wt.Behind)
		}
		if abStr == "" {
			abStr = "0"
		}

		prStr := "-"
		if wt.PR != nil {
			stateLetter := wt.PR.State
			if stateLetter != "" {
				stateLetter = stateLetter[:1]
			}
			prStr = fmt.Sprintf("#%d %s", wt.PR.Number, stateLetter)
		}

		rows = append(rows, table.Row{
			name,
			status,
			abStr,
			prStr,
			wt.LastActive,
		})
	}

	m.worktreeTable.SetRows(rows)
	if len(m.filteredWts) > 0 && m.selectedIndex >= len(m.filteredWts) {
		m.selectedIndex = len(m.filteredWts) - 1
	}
	if len(m.filteredWts) > 0 {
		cursor := m.worktreeTable.Cursor()
		if cursor < 0 {
			cursor = 0
		}
		if cursor >= len(m.filteredWts) {
			cursor = len(m.filteredWts) - 1
		}
		m.selectedIndex = cursor
		m.worktreeTable.SetCursor(cursor)
	}
}

func (m *AppModel) updateDetailsView() tea.Cmd {
	m.selectedIndex = m.worktreeTable.Cursor()
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}

	wt := m.filteredWts[m.selectedIndex]
	return func() tea.Msg {
		// Get status
		statusRaw := m.git.RunGit(m.ctx, []string{"git", "status", "--short"}, wt.Path, []int{0}, true, false)
		logRaw := m.git.RunGit(m.ctx, []string{"git", "log", "-20", "--pretty=format:%h%x09%s"}, wt.Path, []int{0}, true, false)

		// Parse log
		logEntries := []commitLogEntry{}
		for _, line := range strings.Split(logRaw, "\n") {
			parts := strings.SplitN(line, "\t", 2)
			if len(parts) == 2 {
				logEntries = append(logEntries, commitLogEntry{
					sha:     parts[0],
					message: parts[1],
				})
			}
		}
		m.logEntries = logEntries

		// Build status content with automatic diff if dirty
		statusContent := m.buildStatusContent(statusRaw)
		if wt.Dirty {
			// Automatically show diff when there are changes
			diff := m.git.BuildThreePartDiff(m.ctx, wt.Path, m.config)
			diff = m.git.ApplyDelta(diff)
			if diff != "" {
				statusContent = statusContent + "\n\n" + diff
			}
		}

		return statusUpdatedMsg{
			info:   m.buildInfoContent(wt),
			status: statusContent,
			log:    logEntries,
		}
	}
}

func (m *AppModel) debouncedUpdateDetailsView() tea.Cmd {
	// Cancel any existing pending detail update
	if m.detailUpdateCancel != nil {
		m.detailUpdateCancel()
	}

	// Get current selected index
	m.pendingDetailsIndex = m.worktreeTable.Cursor()
	selectedIndex := m.pendingDetailsIndex

	return func() tea.Msg {
		// Wait 200ms
		time.Sleep(200 * time.Millisecond)
		return debouncedDetailsMsg{
			selectedIndex: selectedIndex,
		}
	}
}

func (m *AppModel) refreshWorktrees() tea.Cmd {
	return func() tea.Msg {
		worktrees, err := m.git.GetWorktrees(m.ctx)
		return worktreesLoadedMsg{
			worktrees: worktrees,
			err:       err,
		}
	}
}

func (m *AppModel) fetchPRData() tea.Cmd {
	return func() tea.Msg {
		prMap, err := m.git.FetchPRMap(m.ctx)
		return prDataLoadedMsg{
			prMap: prMap,
			err:   err,
		}
	}
}

func (m *AppModel) fetchRemotes() tea.Cmd {
	return func() tea.Msg {
		m.git.RunGit(m.ctx, []string{"git", "fetch", "--all", "--quiet"}, "", []int{0}, false, false)
		return refreshCompleteMsg{}
	}
}

func (m *AppModel) showCreateWorktree() tea.Cmd {
	defaultBase := m.git.GetMainBranch(m.ctx)

	// Stage 1: branch name
	m.inputScreen = NewInputScreen("Create worktree: branch name", "feature/my-branch", "")
	m.inputSubmit = func(value string) (tea.Cmd, bool) {
		newBranch := strings.TrimSpace(value)
		if newBranch == "" {
			m.inputScreen.errorMsg = "Branch name cannot be empty."
			return nil, false
		}

		// Prevent duplicates
		for _, wt := range m.worktrees {
			if wt.Branch == newBranch {
				m.inputScreen.errorMsg = fmt.Sprintf("Branch %q already exists.", newBranch)
				return nil, false
			}
		}

		targetPath := filepath.Join(m.getWorktreeDir(), newBranch)
		if _, err := os.Stat(targetPath); err == nil {
			m.inputScreen.errorMsg = fmt.Sprintf("Path already exists: %s", targetPath)
			return nil, false
		}

		// Stage 2: base branch prompt
		m.inputScreen = NewInputScreen(fmt.Sprintf("Base branch for %q", newBranch), defaultBase, defaultBase)
		m.inputSubmit = func(baseVal string) (tea.Cmd, bool) {
			baseBranch := strings.TrimSpace(baseVal)
			if baseBranch == "" {
				m.inputScreen.errorMsg = "Base branch cannot be empty."
				return nil, false
			}

			m.inputScreen.errorMsg = ""
			if err := os.MkdirAll(m.getWorktreeDir(), 0o755); err != nil {
				return func() tea.Msg { return errMsg{err: fmt.Errorf("failed to create worktree directory: %w", err)} }, true
			}

			ok := m.git.RunCommandChecked(
				m.ctx,
				[]string{"git", "worktree", "add", "-b", newBranch, targetPath, baseBranch},
				"",
				fmt.Sprintf("Failed to create worktree %s", newBranch),
			)
			if !ok {
				return func() tea.Msg { return errMsg{err: fmt.Errorf("failed to create worktree %s", newBranch)} }, true
			}

			env := m.buildCommandEnv(newBranch, targetPath)
			initCmds := m.collectInitCommands()
			after := func() tea.Msg {
				worktrees, err := m.git.GetWorktrees(m.ctx)
				return worktreesLoadedMsg{
					worktrees: worktrees,
					err:       err,
				}
			}
			return m.runCommandsWithTrust(initCmds, targetPath, env, after), true
		}

		return textinput.Blink, false
	}
	m.currentScreen = screenInput
	return textinput.Blink
}

func (m *AppModel) showDeleteWorktree() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	wt := m.filteredWts[m.selectedIndex]
	if wt.IsMain {
		return nil
	}
	m.currentScreen = screenConfirm
	m.confirmAction = m.deleteWorktreeCmd(wt)
	m.confirmMessage = fmt.Sprintf("Delete worktree?\n\nPath: %s\nBranch: %s", wt.Path, wt.Branch)
	return nil
}

func (m *AppModel) showDiff() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	wt := m.filteredWts[m.selectedIndex]
	return func() tea.Msg {
		// Build three-part diff (staged + unstaged + untracked)
		diff := m.git.BuildThreePartDiff(m.ctx, wt.Path, m.config)
		// Apply delta if available
		diff = m.git.ApplyDelta(diff)
		return statusUpdatedMsg{
			info:   m.buildInfoContent(wt),
			status: fmt.Sprintf("Diff for %s:\n\n%s", wt.Branch, diff),
			log:    []commitLogEntry{},
		}
	}
}

func (m *AppModel) showRenameWorktree() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}

	wt := m.filteredWts[m.selectedIndex]
	if wt.IsMain {
		m.statusContent = "Cannot rename the main worktree."
		return nil
	}

	prompt := fmt.Sprintf("Enter new name for '%s'", wt.Branch)
	m.inputScreen = NewInputScreen(prompt, "New branch name", wt.Branch)
	m.inputSubmit = func(value string) (tea.Cmd, bool) {
		newBranch := strings.TrimSpace(value)
		if newBranch == "" {
			m.inputScreen.errorMsg = "Name cannot be empty."
			return nil, false
		}
		if newBranch == wt.Branch {
			m.inputScreen.errorMsg = "Name must be different from the current branch."
			return nil, false
		}

		parentDir := filepath.Dir(wt.Path)
		newPath := filepath.Join(parentDir, newBranch)
		if _, err := os.Stat(newPath); err == nil {
			m.inputScreen.errorMsg = fmt.Sprintf("Destination already exists: %s", newPath)
			return nil, false
		}

		m.inputScreen.errorMsg = ""
		oldPath := wt.Path
		oldBranch := wt.Branch

		return func() tea.Msg {
			ok := m.git.RenameWorktree(m.ctx, oldPath, newPath, oldBranch, newBranch)
			if !ok {
				return errMsg{err: fmt.Errorf("failed to rename %s to %s", oldBranch, newBranch)}
			}
			worktrees, err := m.git.GetWorktrees(m.ctx)
			return worktreesLoadedMsg{
				worktrees: worktrees,
				err:       err,
			}
		}, true
	}
	m.currentScreen = screenInput
	return textinput.Blink
}

func (m *AppModel) showPruneMerged() tea.Cmd {
	merged := []*models.WorktreeInfo{}
	for _, wt := range m.worktrees {
		if wt.IsMain {
			continue
		}
		if wt.PR != nil && strings.EqualFold(wt.PR.State, "MERGED") {
			merged = append(merged, wt)
		}
	}

	if len(merged) == 0 {
		m.statusContent = "No merged PR worktrees to prune."
		return nil
	}

	// Build confirmation message (truncate if long)
	lines := []string{}
	limit := len(merged)
	if limit > 10 {
		limit = 10
	}
	for i := 0; i < limit; i++ {
		lines = append(lines, fmt.Sprintf("- %s (%s)", merged[i].Path, merged[i].Branch))
	}
	if len(merged) > limit {
		lines = append(lines, fmt.Sprintf("...and %d more", len(merged)-limit))
	}

	m.confirmMessage = "Prune merged PR worktrees?\n\n" + strings.Join(lines, "\n")
	m.confirmAction = func() tea.Cmd {
		return func() tea.Msg {
			pruned := 0
			failed := 0
			for _, wt := range merged {
				ok1 := m.git.RunCommandChecked(m.ctx, []string{"git", "worktree", "remove", "--force", wt.Path}, "", fmt.Sprintf("Failed to remove worktree %s", wt.Path))
				ok2 := m.git.RunCommandChecked(m.ctx, []string{"git", "branch", "-D", wt.Branch}, "", fmt.Sprintf("Failed to delete branch %s", wt.Branch))
				if ok1 && ok2 {
					pruned++
				} else {
					failed++
				}
			}
			worktrees, err := m.git.GetWorktrees(m.ctx)
			return pruneResultMsg{
				worktrees: worktrees,
				err:       err,
				pruned:    pruned,
				failed:    failed,
			}
		}
	}
	m.currentScreen = screenConfirm
	return nil
}

// showAbsorbWorktree merges selected branch into main and removes the worktree
func (m *AppModel) showAbsorbWorktree() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	wt := m.filteredWts[m.selectedIndex]
	if wt.IsMain {
		m.statusContent = "Cannot absorb the main worktree."
		return nil
	}

	mainBranch := m.git.GetMainBranch(m.ctx)
	m.confirmMessage = fmt.Sprintf("Absorb worktree into %s?\n\nPath: %s\nBranch: %s -> %s", mainBranch, wt.Path, wt.Branch, mainBranch)
	m.confirmAction = func() tea.Cmd {
		return func() tea.Msg {
			ok := m.git.RunCommandChecked(m.ctx, []string{"git", "-C", wt.Path, "checkout", mainBranch}, "", fmt.Sprintf("Failed to checkout %s", mainBranch))
			if ok {
				_ = m.git.RunCommandChecked(m.ctx, []string{"git", "-C", wt.Path, "merge", "--no-edit", wt.Branch}, "", fmt.Sprintf("Failed to merge %s into %s", wt.Branch, mainBranch))
			}

			env := m.buildCommandEnv(wt.Branch, wt.Path)
			terminateCmds := m.collectTerminateCommands()
			after := func() tea.Msg {
				m.git.RunCommandChecked(m.ctx, []string{"git", "worktree", "remove", "--force", wt.Path}, "", fmt.Sprintf("Failed to remove worktree %s", wt.Path))
				m.git.RunCommandChecked(m.ctx, []string{"git", "branch", "-D", wt.Branch}, "", fmt.Sprintf("Failed to delete branch %s", wt.Branch))

				worktrees, err := m.git.GetWorktrees(m.ctx)
				return worktreesLoadedMsg{
					worktrees: worktrees,
					err:       err,
				}
			}
			cmd := m.runCommandsWithTrust(terminateCmds, wt.Path, env, after)
			if cmd != nil {
				return cmd()
			}
			return nil
		}
	}
	m.currentScreen = screenConfirm
	return nil
}

func (m *AppModel) deleteWorktreeCmd(wt *models.WorktreeInfo) func() tea.Cmd {
	env := m.buildCommandEnv(wt.Branch, wt.Path)
	terminateCmds := m.collectTerminateCommands()
	afterCmd := func() tea.Msg {
		m.git.RunCommandChecked(m.ctx, []string{"git", "worktree", "remove", "--force", wt.Path}, "", fmt.Sprintf("Failed to remove worktree %s", wt.Path))
		m.git.RunCommandChecked(m.ctx, []string{"git", "branch", "-D", wt.Branch}, "", fmt.Sprintf("Failed to delete branch %s", wt.Branch))

		worktrees, err := m.git.GetWorktrees(m.ctx)
		return worktreesLoadedMsg{
			worktrees: worktrees,
			err:       err,
		}
	}

	return func() tea.Cmd {
		return m.runCommandsWithTrust(terminateCmds, wt.Path, env, afterCmd)
	}
}

func (m *AppModel) openLazyGit() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	wt := m.filteredWts[m.selectedIndex]
	return func() tea.Msg {
		cmd := exec.Command("lazygit")
		cmd.Dir = wt.Path
		if err := cmd.Run(); err != nil {
			return errMsg{err: err}
		}
		return refreshCompleteMsg{}
	}
}

func (m *AppModel) openPR() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	wt := m.filteredWts[m.selectedIndex]
	if wt.PR == nil {
		return nil
	}
	return func() tea.Msg {
		if err := exec.Command("xdg-open", wt.PR.URL).Start(); err != nil {
			return errMsg{err: err}
		}
		return nil
	}
}

func (m *AppModel) openCommitView() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	if len(m.logEntries) == 0 {
		return nil
	}

	cursor := m.logTable.Cursor()
	if cursor < 0 || cursor >= len(m.logEntries) {
		return nil
	}
	entry := m.logEntries[cursor]
	wt := m.filteredWts[m.selectedIndex]

	return func() tea.Msg {
		header := m.git.RunGit(m.ctx, []string{"git", "show", "--quiet", "--pretty=fuller", entry.sha}, wt.Path, []int{0}, false, false)
		diff := m.git.RunGit(m.ctx, []string{"git", "show", "--patch", entry.sha}, wt.Path, []int{0}, false, false)
		diff = m.git.ApplyDelta(diff)

		m.currentScreen = screenCommit
		m.commitScreen = NewCommitScreen(header, diff, m.git.UseDelta())
		return nil
	}
}

func (m *AppModel) handleScreenKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.currentScreen {
	case screenHelp:
		if m.helpScreen == nil {
			m.helpScreen = NewHelpScreen(m.windowWidth, m.windowHeight)
		}
		if msg.String() == "q" || msg.String() == "esc" {
			// If currently searching, esc clears search; otherwise close help
			if m.helpScreen.searching || m.helpScreen.searchQuery != "" {
				m.helpScreen.searching = false
				m.helpScreen.searchInput.Blur()
				m.helpScreen.searchInput.SetValue("")
				m.helpScreen.searchQuery = ""
				m.helpScreen.refreshContent()
				return m, nil
			}
			m.currentScreen = screenNone
			m.helpScreen = nil
			return m, nil
		}
		hs, cmd := m.helpScreen.Update(msg)
		if updated, ok := hs.(*HelpScreen); ok {
			m.helpScreen = updated
		}
		return m, cmd
	case screenConfirm:
		if msg.String() == "y" || msg.String() == "enter" {
			// Perform confirmed action (delete, prune, etc.)
			var cmd tea.Cmd
			if m.confirmAction != nil {
				cmd = m.confirmAction()
			}
			m.currentScreen = screenNone
			m.confirmAction = nil
			m.confirmMessage = ""
			if cmd != nil {
				return m, cmd
			}
			return m, nil
		} else if msg.String() == "n" || msg.String() == "esc" {
			m.currentScreen = screenNone
			m.confirmAction = nil
			m.confirmMessage = ""
			return m, nil
		}
	case screenWelcome:
		switch msg.String() {
		case "r", "R":
			m.currentScreen = screenNone
			m.welcomeScreen = nil
			return m, m.refreshWorktrees()
		case "q", "Q", "esc":
			m.quitting = true
			return m, tea.Quit
		}
	case screenTrust:
		if m.trustScreen == nil {
			m.currentScreen = screenNone
			return m, nil
		}
		switch msg.String() {
		case "t", "T":
			if m.pendingTrust != "" {
				_ = m.trustManager.TrustFile(m.pendingTrust)
			}
			var after tea.Msg
			if m.pendingAfter != nil {
				after = m.pendingAfter()
			}
			cmd := m.runCommands(m.pendingCommands, m.pendingCmdCwd, m.pendingCmdEnv, after)
			m.clearPendingTrust()
			m.currentScreen = screenNone
			return m, cmd
		case "b", "B":
			after := m.pendingAfter
			m.clearPendingTrust()
			m.currentScreen = screenNone
			if after != nil {
				return m, after
			}
			return m, nil
		case "c", "C", "esc":
			m.clearPendingTrust()
			m.currentScreen = screenNone
			return m, nil
		}
		ts, cmd := m.trustScreen.Update(msg)
		if updated, ok := ts.(*TrustScreen); ok {
			m.trustScreen = updated
		}
		return m, cmd
	case screenCommit:
		if m.commitScreen == nil {
			m.currentScreen = screenNone
			return m, nil
		}
		switch msg.String() {
		case "q", "esc":
			m.commitScreen = nil
			m.currentScreen = screenNone
			return m, nil
		}
		cs, cmd := m.commitScreen.Update(msg)
		if updated, ok := cs.(*CommitScreen); ok {
			m.commitScreen = updated
		}
		return m, cmd
	case screenInput:
		if m.inputScreen == nil {
			m.currentScreen = screenNone
			return m, nil
		}

		switch msg.String() {
		case "enter":
			if m.inputSubmit != nil {
				cmd, close := m.inputSubmit(m.inputScreen.input.Value())
				if close {
					m.inputScreen = nil
					m.inputSubmit = nil
					m.currentScreen = screenNone
				}
				return m, cmd
			}
		case "esc":
			m.inputScreen = nil
			m.inputSubmit = nil
			m.currentScreen = screenNone
			return m, nil
		}

		var cmd tea.Cmd
		m.inputScreen.input, cmd = m.inputScreen.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *AppModel) renderScreen() string {
	switch m.currentScreen {
	case screenHelp:
		if m.helpScreen == nil {
			m.helpScreen = NewHelpScreen(m.windowWidth, m.windowHeight)
		}
		m.helpScreen.SetSize(m.windowWidth, m.windowHeight)
		return m.helpScreen.View()
	case screenCommit:
		if m.commitScreen == nil {
			m.commitScreen = NewCommitScreen("", "", m.git.UseDelta())
		}
		return m.commitScreen.View()
	case screenConfirm:
		msg := m.confirmMessage
		if msg == "" && m.selectedIndex >= 0 && m.selectedIndex < len(m.filteredWts) {
			wt := m.filteredWts[m.selectedIndex]
			msg = fmt.Sprintf("Delete worktree?\n\nPath: %s\nBranch: %s", wt.Path, wt.Branch)
		}
		confirmScreen := NewConfirmScreen(msg)
		return confirmScreen.View()
	case screenTrust:
		if m.trustScreen == nil {
			return ""
		}
		return m.trustScreen.View()
	case screenWelcome:
		if m.welcomeScreen == nil {
			cwd, _ := os.Getwd()
			m.welcomeScreen = NewWelcomeScreen(cwd, m.getWorktreeDir())
		}
		return m.welcomeScreen.View()
	case screenInput:
		if m.inputScreen != nil {
			content := m.inputScreen.View()
			if m.windowWidth > 0 && m.windowHeight > 0 {
				return lipgloss.Place(m.windowWidth, m.windowHeight, lipgloss.Center, lipgloss.Center, content)
			}
			return content
		}
	}
	return ""
}

func (m *AppModel) loadCache() tea.Cmd {
	return func() tea.Msg {
		repoKey := m.getRepoKey()
		cachePath := filepath.Join(m.getWorktreeDir(), repoKey, models.CacheFilename)
		data, err := os.ReadFile(cachePath)
		if err != nil {
			return nil
		}
		if err := json.Unmarshal(data, &m.cache); err != nil {
			return errMsg{err: err}
		}
		return nil
	}
}

func (m *AppModel) saveCache() {
	repoKey := m.getRepoKey()
	cachePath := filepath.Join(m.getWorktreeDir(), repoKey, models.CacheFilename)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		m.statusContent = fmt.Sprintf("Failed to create cache dir: %v", err)
		return
	}

	cacheData := map[string]interface{}{
		"worktrees": m.worktrees,
	}
	data, _ := json.Marshal(cacheData)
	if err := os.WriteFile(cachePath, data, 0o644); err != nil {
		m.statusContent = fmt.Sprintf("Failed to write cache: %v", err)
	}
}

func (m *AppModel) getRepoKey() string {
	if m.repoKey != "" {
		return m.repoKey
	}
	m.repoKey = m.git.ResolveRepoName(m.ctx)
	return m.repoKey
}

func (m *AppModel) getWorktreeDir() string {
	if m.config.WorktreeDir != "" {
		return m.config.WorktreeDir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "worktrees")
}

// GetSelectedPath returns the selected worktree path (for shell integration)
func (m *AppModel) GetSelectedPath() string {
	return m.selectedPath
}

func (m *AppModel) buildCommandEnv(branch, wtPath string) map[string]string {
	return map[string]string{
		"WORKTREE_BRANCH":    branch,
		"MAIN_WORKTREE_PATH": m.git.GetMainWorktreePath(m.ctx),
		"WORKTREE_PATH":      wtPath,
		"WORKTREE_NAME":      filepath.Base(wtPath),
	}
}

func (m *AppModel) collectInitCommands() []string {
	cmds := []string{}
	cmds = append(cmds, m.config.InitCommands...)
	if m.repoConfig != nil {
		cmds = append(cmds, m.repoConfig.InitCommands...)
	}
	return cmds
}

func (m *AppModel) collectTerminateCommands() []string {
	cmds := []string{}
	cmds = append(cmds, m.config.TerminateCommands...)
	if m.repoConfig != nil {
		cmds = append(cmds, m.repoConfig.TerminateCommands...)
	}
	return cmds
}

func (m *AppModel) runCommandsWithTrust(cmds []string, cwd string, env map[string]string, after tea.Msg) tea.Cmd {
	if len(cmds) == 0 {
		if after == nil {
			return nil
		}
		return func() tea.Msg { return after }
	}

	trustMode := strings.ToLower(strings.TrimSpace(m.config.TrustMode))
	// If trust mode set to never, skip repo commands
	if trustMode == "never" {
		if after == nil {
			return nil
		}
		return func() tea.Msg { return after }
	}

	// Determine trust status if repo config exists
	trustPath := m.repoConfigPath
	status := security.TrustStatusTrusted
	if m.repoConfig != nil && trustPath != "" {
		status = m.trustManager.CheckTrust(trustPath)
	}

	if trustMode == "always" || status == security.TrustStatusTrusted {
		return m.runCommands(cmds, cwd, env, after)
	}

	// TOFU: prompt user
	if trustPath != "" {
		m.pendingCommands = cmds
		m.pendingCmdEnv = env
		m.pendingCmdCwd = cwd
		m.pendingAfter = func() tea.Msg { return after }
		m.pendingTrust = trustPath
		m.trustScreen = NewTrustScreen(trustPath, cmds)
		m.currentScreen = screenTrust
	}
	return nil
}

func (m *AppModel) runCommands(cmds []string, cwd string, env map[string]string, after tea.Msg) tea.Cmd {
	return func() tea.Msg {
		if err := m.git.ExecuteCommands(m.ctx, cmds, cwd, env); err != nil {
			return errMsg{err: err}
		}
		if after != nil {
			return after
		}
		return nil
	}
}

func (m *AppModel) clearPendingTrust() {
	m.pendingCommands = nil
	m.pendingCmdEnv = nil
	m.pendingCmdCwd = ""
	m.pendingAfter = nil
	m.pendingTrust = ""
	m.trustScreen = nil
}

func (m *AppModel) ensureRepoConfig() {
	if m.repoConfig != nil || m.repoConfigPath != "" {
		return
	}
	mainPath := m.git.GetMainWorktreePath(m.ctx)
	repoCfg, cfgPath, err := config.LoadRepoConfig(mainPath)
	if err != nil {
		m.statusContent = fmt.Sprintf("Failed to load .wt: %v", err)
		return
	}
	m.repoConfigPath = cfgPath
	m.repoConfig = repoCfg
}

type layoutDims struct {
	width                  int
	height                 int
	headerHeight           int
	footerHeight           int
	filterHeight           int
	bodyHeight             int
	gapX                   int
	gapY                   int
	leftWidth              int
	rightWidth             int
	leftInnerWidth         int
	rightInnerWidth        int
	leftInnerHeight        int
	rightTopHeight         int
	rightBottomHeight      int
	rightTopInnerHeight    int
	rightBottomInnerHeight int
}

func (m *AppModel) setWindowSize(width, height int) {
	m.windowWidth = width
	m.windowHeight = height
	m.applyLayout(m.computeLayout())
}

func (m *AppModel) computeLayout() layoutDims {
	width := m.windowWidth
	height := m.windowHeight
	if width <= 0 {
		width = 120
	}
	if height <= 0 {
		height = 40
	}

	headerHeight := 1
	footerHeight := 1
	filterHeight := 0
	if m.showingFilter {
		filterHeight = 1
	}
	gapX := 1
	gapY := 1

	bodyHeight := height - headerHeight - footerHeight - filterHeight
	if bodyHeight < 8 {
		bodyHeight = 8
	}

	leftRatio := 0.55
	switch m.focusedPane {
	case 0:
		leftRatio = 0.60
	case 1, 2:
		leftRatio = 0.20
	}

	leftWidth := int(float64(width-gapX) * leftRatio)
	rightWidth := width - leftWidth - gapX
	if leftWidth < minLeftPaneWidth {
		leftWidth = minLeftPaneWidth
		rightWidth = width - leftWidth - gapX
	}
	if rightWidth < minRightPaneWidth {
		rightWidth = minRightPaneWidth
		leftWidth = width - rightWidth - gapX
	}
	if leftWidth < minLeftPaneWidth {
		leftWidth = minLeftPaneWidth
	}
	if rightWidth < minRightPaneWidth {
		rightWidth = minRightPaneWidth
	}
	if leftWidth+rightWidth+gapX > width {
		rightWidth = width - leftWidth - gapX
	}
	if rightWidth < 0 {
		rightWidth = 0
	}

	topRatio := 0.70
	switch m.focusedPane {
	case 1: // Info/Diff focused → give more height to top pane
		topRatio = 0.82
	case 2: // Log focused → give more height to bottom pane
		topRatio = 0.30
	}

	rightTopHeight := int(float64(bodyHeight-gapY) * topRatio)
	if rightTopHeight < 6 {
		rightTopHeight = 6
	}
	rightBottomHeight := bodyHeight - rightTopHeight - gapY
	if rightBottomHeight < 4 {
		rightBottomHeight = 4
		rightTopHeight = bodyHeight - rightBottomHeight - gapY
	}

	paneFrameX := basePaneStyle().GetHorizontalFrameSize()
	paneFrameY := basePaneStyle().GetVerticalFrameSize()

	leftInnerWidth := max(1, leftWidth-paneFrameX)
	rightInnerWidth := max(1, rightWidth-paneFrameX)
	leftInnerHeight := max(1, bodyHeight-paneFrameY)
	rightTopInnerHeight := max(1, rightTopHeight-paneFrameY)
	rightBottomInnerHeight := max(1, rightBottomHeight-paneFrameY)

	return layoutDims{
		width:                  width,
		height:                 height,
		headerHeight:           headerHeight,
		footerHeight:           footerHeight,
		filterHeight:           filterHeight,
		bodyHeight:             bodyHeight,
		gapX:                   gapX,
		gapY:                   gapY,
		leftWidth:              leftWidth,
		rightWidth:             rightWidth,
		leftInnerWidth:         leftInnerWidth,
		rightInnerWidth:        rightInnerWidth,
		leftInnerHeight:        leftInnerHeight,
		rightTopHeight:         rightTopHeight,
		rightBottomHeight:      rightBottomHeight,
		rightTopInnerHeight:    rightTopInnerHeight,
		rightBottomInnerHeight: rightBottomInnerHeight,
	}
}

func (m *AppModel) applyLayout(layout layoutDims) {
	titleHeight := 1
	tableHeaderHeight := 1 // bubbles table has its own header

	// Subtract 2 extra lines for safety margin
	tableHeight := max(1, layout.leftInnerHeight-titleHeight-tableHeaderHeight-2)
	m.worktreeTable.SetWidth(layout.leftInnerWidth)
	m.worktreeTable.SetHeight(tableHeight)
	m.updateTableColumns(layout.leftInnerWidth)

	logHeight := max(1, layout.rightBottomInnerHeight-titleHeight-tableHeaderHeight-2)
	m.logTable.SetWidth(layout.rightInnerWidth)
	m.logTable.SetHeight(logHeight)
	m.updateLogColumns(layout.rightInnerWidth)

	m.filterInput.Width = max(20, layout.width-18)
}

func (m *AppModel) renderHeader(layout layoutDims) string {
	headerStyle := lipgloss.NewStyle().
		Foreground(colorAccent).
		Bold(true).
		Width(layout.width).
		Align(lipgloss.Center)
	return headerStyle.Render("─── Git Worktree Status ───")
}

func (m *AppModel) renderFilter(layout layoutDims) string {
	labelStyle := lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	filterStyle := lipgloss.NewStyle().
		Foreground(colorTextFg).
		Padding(0, 1)
	line := fmt.Sprintf("%s %s", labelStyle.Render("/ Filter:"), m.filterInput.View())
	return filterStyle.Width(layout.width).Render(line)
}

func (m *AppModel) renderBody(layout layoutDims) string {
	left := m.renderLeftPane(layout)
	right := m.renderRightPane(layout)
	gap := lipgloss.NewStyle().
		Width(layout.gapX).
		Render(strings.Repeat(" ", layout.gapX))
	return lipgloss.JoinHorizontal(lipgloss.Top, left, gap, right)
}

func (m *AppModel) renderLeftPane(layout layoutDims) string {
	title := m.renderPaneTitle(1, "Worktrees", m.focusedPane == 0, layout.leftInnerWidth)
	tableView := m.worktreeTable.View()
	content := lipgloss.JoinVertical(lipgloss.Left, title, tableView)
	return paneStyle(m.focusedPane == 0).
		Width(layout.leftWidth).
		Height(layout.bodyHeight).
		Render(content)
}

func (m *AppModel) renderRightPane(layout layoutDims) string {
	top := m.renderRightTopPane(layout)
	bottom := m.renderRightBottomPane(layout)
	gap := strings.Repeat("\n", layout.gapY)
	return lipgloss.JoinVertical(lipgloss.Left, top, gap, bottom)
}

func (m *AppModel) renderRightTopPane(layout layoutDims) string {
	title := m.renderPaneTitle(2, "Info/Diff", m.focusedPane == 1, layout.rightInnerWidth)
	infoBox := m.renderInnerBox("Info", m.infoContent, layout.rightInnerWidth, 0)

	innerBoxStyle := baseInnerBoxStyle()
	statusBoxHeight := layout.rightTopInnerHeight - lipgloss.Height(title) - lipgloss.Height(infoBox) - 2
	if statusBoxHeight < 3 {
		statusBoxHeight = 3
	}
	statusViewportWidth := max(1, layout.rightInnerWidth-innerBoxStyle.GetHorizontalFrameSize())
	statusViewportHeight := max(1, statusBoxHeight-innerBoxStyle.GetVerticalFrameSize())
	m.statusViewport.Width = statusViewportWidth
	m.statusViewport.Height = statusViewportHeight
	m.statusViewport.SetContent(m.statusContent)
	statusBox := innerBoxStyle.
		Width(layout.rightInnerWidth).
		Height(statusBoxHeight).
		Render(m.statusViewport.View())

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		infoBox,
		statusBox,
	)
	return paneStyle(m.focusedPane == 1).
		Width(layout.rightWidth).
		Height(layout.rightTopHeight).
		Render(content)
}

func (m *AppModel) renderRightBottomPane(layout layoutDims) string {
	title := m.renderPaneTitle(3, "Log", m.focusedPane == 2, layout.rightInnerWidth)
	content := lipgloss.JoinVertical(lipgloss.Left, title, m.logTable.View())
	return paneStyle(m.focusedPane == 2).
		Width(layout.rightWidth).
		Height(layout.rightBottomHeight).
		Render(content)
}

func (m *AppModel) renderFooter(layout layoutDims) string {
	footerStyle := lipgloss.NewStyle().
		Foreground(colorMutedFg).
		Padding(0, 1)
	hints := []string{
		m.renderKeyHint("q", "Quit"),
		m.renderKeyHint("g", "LazyGit"),
		m.renderKeyHint("r", "Refresh"),
		m.renderKeyHint("p", "PR Info"),
		m.renderKeyHint("d", "Diff"),
		m.renderKeyHint("D", "Delete"),
		m.renderKeyHint("/", "Filter"),
		m.renderKeyHint("?", "Help"),
		m.renderKeyHint("tab", "Next Pane"),
	}
	return footerStyle.Width(layout.width).Render(strings.Join(hints, "  "))
}

func (m *AppModel) renderKeyHint(key, label string) string {
	keyStyle := lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(colorMutedFg)
	return fmt.Sprintf("%s %s", keyStyle.Render(key), labelStyle.Render(label))
}

func (m *AppModel) renderPaneTitle(index int, title string, focused bool, width int) string {
	numStyle := lipgloss.NewStyle().Foreground(colorAccentDim)
	titleStyle := lipgloss.NewStyle().Foreground(colorMutedFg)
	if focused {
		numStyle = numStyle.Foreground(colorAccent).Bold(true)
		titleStyle = titleStyle.Foreground(colorTextFg)
	}
	num := numStyle.Render(fmt.Sprintf("[%d]", index))
	name := titleStyle.Render(title)
	return lipgloss.NewStyle().Width(width).Render(fmt.Sprintf("%s %s", num, name))
}

func (m *AppModel) renderInnerBox(title, content string, width, height int) string {
	if content == "" {
		content = "No data available."
	}
	titleStyle := lipgloss.NewStyle().Foreground(colorMutedFg).Bold(true)
	boxContent := lipgloss.JoinVertical(lipgloss.Left, titleStyle.Render(title), content)
	style := baseInnerBoxStyle().Width(width)
	if height > 0 {
		style = style.Height(height)
	}
	return style.Render(boxContent)
}

func (m *AppModel) buildInfoContent(wt *models.WorktreeInfo) string {
	if wt == nil {
		return "No worktree selected."
	}

	labelStyle := lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	valueStyle := lipgloss.NewStyle().Foreground(colorTextFg)
	mutedStyle := lipgloss.NewStyle().Foreground(colorMutedFg)

	infoLines := []string{
		fmt.Sprintf("%s %s", labelStyle.Render("Path:"), valueStyle.Render(wt.Path)),
		fmt.Sprintf("%s %s", labelStyle.Render("Branch:"), valueStyle.Render(wt.Branch)),
	}
	if wt.Divergence != "" {
		infoLines = append(infoLines, fmt.Sprintf("%s %s", labelStyle.Render("Divergence:"), valueStyle.Render(wt.Divergence)))
	}
	if wt.PR != nil {
		stateColor := colorSuccessFg
		switch wt.PR.State {
		case "MERGED":
			stateColor = lipgloss.Color("171")
		case "CLOSED":
			stateColor = colorErrorFg
		}
		stateStyle := lipgloss.NewStyle().Foreground(stateColor).Bold(true)
		infoLines = append(infoLines, fmt.Sprintf("%s #%d %s [%s]", labelStyle.Render("PR:"), wt.PR.Number, valueStyle.Render(wt.PR.Title), stateStyle.Render(wt.PR.State)))
		infoLines = append(infoLines, fmt.Sprintf("%s %s", labelStyle.Render("URL:"), mutedStyle.Render(wt.PR.URL)))
	}
	return strings.Join(infoLines, "\n")
}

func (m *AppModel) buildStatusContent(statusRaw string) string {
	statusRaw = strings.TrimRight(statusRaw, "\n")
	if strings.TrimSpace(statusRaw) == "" {
		return lipgloss.NewStyle().Foreground(colorSuccessFg).Render("Clean working tree")
	}

	modifiedStyle := lipgloss.NewStyle().Foreground(colorWarnFg)
	addedStyle := lipgloss.NewStyle().Foreground(colorSuccessFg)
	deletedStyle := lipgloss.NewStyle().Foreground(colorErrorFg)
	untrackedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))

	lines := []string{}
	for _, line := range strings.Split(statusRaw, "\n") {
		if line == "" {
			continue
		}

		// Parse git status --short format (XY filename)
		if len(line) < 3 {
			lines = append(lines, line)
			continue
		}

		status := line[:2]
		filename := strings.TrimSpace(line[2:])

		// Format based on status
		var formatted string
		switch {
		case strings.HasPrefix(status, "M"):
			formatted = fmt.Sprintf("%s %s", modifiedStyle.Render("M "), filename)
		case strings.HasPrefix(status, "A"):
			formatted = fmt.Sprintf("%s %s", addedStyle.Render("A "), filename)
		case strings.HasPrefix(status, "D"):
			formatted = fmt.Sprintf("%s %s", deletedStyle.Render("D "), filename)
		case strings.HasPrefix(status, "??"):
			formatted = fmt.Sprintf("%s %s", untrackedStyle.Render("??"), filename)
		case strings.HasPrefix(status, "R"):
			formatted = fmt.Sprintf("%s %s", modifiedStyle.Render("R "), filename)
		default:
			formatted = line
		}

		lines = append(lines, formatted)
	}
	return strings.Join(lines, "\n")
}

func (m *AppModel) updateTableColumns(totalWidth int) {
	worktree := max(12, totalWidth-44)
	status := 6
	ab := 7
	pr := 12
	last := 15
	excess := worktree + status + ab + pr + last - totalWidth
	for excess > 0 && last > 10 {
		last--
		excess--
	}
	for excess > 0 && pr > 8 {
		pr--
		excess--
	}
	for excess > 0 && worktree > 12 {
		worktree--
		excess--
	}
	for excess > 0 && status > 4 {
		status--
		excess--
	}
	for excess > 0 && ab > 5 {
		ab--
		excess--
	}
	if excess > 0 {
		worktree = max(6, worktree-excess)
	}

	m.worktreeTable.SetColumns([]table.Column{
		{Title: "Worktree", Width: worktree},
		{Title: "Status", Width: status},
		{Title: "±", Width: ab},
		{Title: "PR", Width: pr},
		{Title: "Last Active", Width: last},
	})
}

func (m *AppModel) updateLogColumns(totalWidth int) {
	sha := 8
	message := max(10, totalWidth-sha)
	m.logTable.SetColumns([]table.Column{
		{Title: "SHA", Width: sha},
		{Title: "Message", Width: message},
	})
}

func basePaneStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(colorBorder).
		Padding(0, 1)
}

func paneStyle(focused bool) lipgloss.Style {
	borderColor := colorBorderDim
	if focused {
		borderColor = colorAccent
	}
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(borderColor).
		Padding(0, 1)
}

func baseInnerBoxStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorderDim).
		Padding(0, 1)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// truncateToHeight ensures output doesn't exceed maxLines
func truncateToHeight(s string, maxLines int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return strings.Join(lines, "\n")
}
