// Package app provides the main application UI and logic using Bubble Tea.
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/git"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/security"
	"github.com/chmouel/lazyworktree/internal/theme"
	"github.com/muesli/reflow/wrap"
)

const (
	keyEnter  = "enter"
	keyEsc    = "esc"
	keyEscRaw = "\x1b" // Raw escape byte for terminals that send ESC as a rune

	errBranchEmpty           = "Branch name cannot be empty."
	errNoWorktreeSelected    = "No worktree selected."
	customCommandPlaceholder = "Custom command"
	tmuxSessionLabel         = "tmux session"

	detailsCacheTTL  = 2 * time.Second
	debounceDelay    = 200 * time.Millisecond
	ciCacheTTL       = 30 * time.Second
	defaultDirPerms  = 0o750
	defaultFilePerms = 0o600

	osDarwin  = "darwin"
	osWindows = "windows"
)

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
	refreshCompleteMsg      struct{}
	fetchRemotesCompleteMsg struct{}
	debouncedDetailsMsg     struct {
		selectedIndex int
	}
	tmuxSessionReadyMsg struct {
		sessionName string
		attach      bool
		insideTmux  bool
	}
	cachedWorktreesMsg struct {
		worktrees []*models.WorktreeInfo
	}
	detailsCacheEntry struct {
		statusRaw string
		logRaw    string
		fetchedAt time.Time
	}
	pruneResultMsg struct {
		worktrees []*models.WorktreeInfo
		err       error
		pruned    int
		failed    int
	}
	commitLoadingMsg struct {
		meta commitMeta
	}
	commitLoadedMsg struct {
		meta commitMeta
		stat string
		diff string
	}
	absorbMergeResultMsg struct {
		path   string
		branch string
		err    error
	}
	ciStatusLoadedMsg struct {
		branch string
		checks []*models.CICheck
		err    error
	}
	openPRsLoadedMsg struct {
		prs []*models.PRInfo
		err error
	}
	createFromChangesReadyMsg struct {
		worktree      *models.WorktreeInfo
		currentBranch string
		diff          string // git diff output for branch name generation
	}
)

type commitLogEntry struct {
	sha     string
	message string
}

type commitMeta struct {
	sha     string
	author  string
	email   string
	date    string
	subject string
	body    []string
}

type ciCacheEntry struct {
	checks    []*models.CICheck
	fetchedAt time.Time
}

const (
	minLeftPaneWidth  = 32
	minRightPaneWidth = 32
	mainWorktreeName  = "main"
)

// Model represents the main application model
type Model struct {
	// Configuration
	config *config.AppConfig
	git    *git.Service
	theme  *theme.Theme

	// UI Components
	worktreeTable  table.Model
	statusViewport viewport.Model
	logTable       table.Model
	filterInput    textinput.Model

	// State
	worktrees         []*models.WorktreeInfo
	filteredWts       []*models.WorktreeInfo
	selectedIndex     int
	filterQuery       string
	sortByActive      bool
	prDataLoaded      bool
	repoKey           string
	repoKeyOnce       sync.Once
	currentScreen     screenType
	helpScreen        *HelpScreen
	trustScreen       *TrustScreen
	inputScreen       *InputScreen
	inputSubmit       func(string) (tea.Cmd, bool)
	commitScreen      *CommitScreen
	welcomeScreen     *WelcomeScreen
	paletteScreen     *CommandPaletteScreen
	paletteSubmit     func(string) tea.Cmd
	prSelectionScreen *PRSelectionScreen
	prSelectionSubmit func(*models.PRInfo) tea.Cmd
	listScreen        *ListSelectionScreen
	listSubmit        func(selectionItem) tea.Cmd
	diffScreen        *DiffScreen
	spinner           spinner.Model
	loading           bool
	showingFilter     bool
	focusedPane       int // 0=table, 1=status, 2=log
	windowWidth       int
	windowHeight      int
	infoContent       string
	statusContent     string

	// Cache
	cache           map[string]any
	divergenceCache map[string]string
	notifiedErrors  map[string]bool
	ciCache         map[string]*ciCacheEntry // branch -> CI checks cache
	detailsCache    map[string]*detailsCacheEntry
	worktreesLoaded bool

	// Services
	trustManager *security.TrustManager

	// Context
	ctx    context.Context
	cancel context.CancelFunc

	// Debouncing
	detailUpdateCancel  context.CancelFunc
	pendingDetailsIndex int

	// Confirm screen
	confirmScreen *ConfirmScreen
	confirmAction func() tea.Cmd
	infoScreen    *InfoScreen
	infoAction    tea.Cmd

	// Trust / repo commands
	repoConfig      *config.RepoConfig
	repoConfigPath  string
	pendingCommands []string
	pendingCmdEnv   map[string]string
	pendingCmdCwd   string
	pendingAfter    func() tea.Msg
	pendingTrust    string

	// Log cache for commit detail viewer
	logEntries []commitLogEntry

	// Exit
	selectedPath string
	quitting     bool

	// Command execution
	commandRunner func(string, ...string) *exec.Cmd
	execProcess   func(*exec.Cmd, tea.ExecCallback) tea.Cmd
	startCommand  func(*exec.Cmd) error

	// Debug logging
	debugLogger  *log.Logger
	debugLogFile *os.File
}

// NewModel creates a new application model with the given configuration.
// initialFilter is an optional filter string to apply on startup.
func NewModel(cfg *config.AppConfig, initialFilter string) *Model {
	ctx, cancel := context.WithCancel(context.Background())

	// Load theme
	thm := theme.GetTheme(cfg.Theme)

	var debugLogFile *os.File
	var debugLogger *log.Logger
	var debugMu sync.Mutex
	debugNotified := map[string]bool{}

	if strings.TrimSpace(cfg.DebugLog) != "" {
		logDir := filepath.Dir(cfg.DebugLog)
		if err := os.MkdirAll(logDir, defaultDirPerms); err == nil {
			file, err := os.OpenFile(cfg.DebugLog, os.O_CREATE|os.O_WRONLY|os.O_APPEND, defaultFilePerms)
			if err == nil {
				debugLogFile = file
				debugLogger = log.New(file, "", log.LstdFlags)
				debugLogger.SetFlags(log.LstdFlags | log.Lmicroseconds)
				debugLogger.Printf("debug logging enabled")
			}
		}
	}

	notify := func(message string, severity string) {
		if debugLogger == nil {
			return
		}
		debugMu.Lock()
		defer debugMu.Unlock()
		debugLogger.Printf("[%s] %s", severity, message)
	}
	notifyOnce := func(key string, message string, severity string) {
		if debugLogger == nil {
			return
		}
		debugMu.Lock()
		defer debugMu.Unlock()
		if debugNotified[key] {
			return
		}
		debugNotified[key] = true
		debugLogger.Printf("[%s] %s", severity, message)
	}

	gitService := git.NewService(notify, notifyOnce)
	gitService.SetDebugLogger(debugLogger)
	gitService.SetDeltaPath(cfg.DeltaPath)
	gitService.SetDeltaArgs(cfg.DeltaArgs)
	trustManager := security.NewTrustManager()

	columns := []table.Column{
		{Title: "Name", Width: 20},
		{Title: "Status", Width: 8},
		{Title: "±", Width: 10},
		{Title: "Last Active", Width: 20},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(5),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(thm.BorderDim).
		BorderBottom(true).
		Bold(true).
		Foreground(thm.Cyan)
	s.Selected = s.Selected.
		Foreground(thm.TextFg).
		Background(thm.AccentDim).
		Bold(true)
	t.SetStyles(s)

	statusVp := viewport.New(40, 5)
	statusVp.SetContent("Loading...")

	logColumns := []table.Column{
		{Title: "SHA", Width: 10},
		{Title: "Message", Width: 50},
	}
	logT := table.New(
		table.WithColumns(logColumns),
		table.WithHeight(5),
	)
	logStyles := s
	logT.SetStyles(logStyles)

	filterInput := textinput.New()
	filterInput.Placeholder = "Filter worktrees..."
	filterInput.Width = 50
	filterInput.PromptStyle = lipgloss.NewStyle().Foreground(thm.Accent)
	filterInput.TextStyle = lipgloss.NewStyle().Foreground(thm.TextFg)

	sp := spinner.New()
	sp.Spinner = spinner.Pulse
	sp.Style = lipgloss.NewStyle().Foreground(thm.Accent)

	m := &Model{
		config:          cfg,
		git:             gitService,
		theme:           thm,
		worktreeTable:   t,
		statusViewport:  statusVp,
		logTable:        logT,
		filterInput:     filterInput,
		worktrees:       []*models.WorktreeInfo{},
		filteredWts:     []*models.WorktreeInfo{},
		sortByActive:    cfg.SortByActive,
		filterQuery:     initialFilter,
		cache:           make(map[string]any),
		divergenceCache: make(map[string]string),
		notifiedErrors:  make(map[string]bool),
		ciCache:         make(map[string]*ciCacheEntry),
		detailsCache:    make(map[string]*detailsCacheEntry),
		trustManager:    trustManager,
		ctx:             ctx,
		cancel:          cancel,
		focusedPane:     0,
		infoContent:     errNoWorktreeSelected,
		statusContent:   "Loading...",
		spinner:         sp,
		loading:         true,
		commandRunner:   exec.Command,
		execProcess:     tea.ExecProcess,
		startCommand: func(cmd *exec.Cmd) error {
			return cmd.Start()
		},
		debugLogger:  debugLogger,
		debugLogFile: debugLogFile,
	}

	if initialFilter != "" {
		m.showingFilter = true
		m.filterInput.SetValue(initialFilter)
	}

	return m
}

// Init satisfies the tea.Model interface and starts with no command.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.loadCache(),
		m.refreshWorktrees(),
		m.spinner.Tick,
	)
}

// Update processes Bubble Tea messages and routes them through the app model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.debugf("window: %dx%d", msg.Width, msg.Height)
		m.setWindowSize(msg.Width, msg.Height)
		return m, nil

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		m.debugf("key: %s screen=%s focus=%d filter=%t", msg.String(), screenName(m.currentScreen), m.focusedPane, m.showingFilter)
		if m.currentScreen != screenNone {
			return m.handleScreenKey(msg)
		}
		return m.handleKeyMsg(msg)

	case worktreesLoadedMsg, cachedWorktreesMsg, pruneResultMsg, absorbMergeResultMsg:
		return m.handleWorktreeMessages(msg)

	case openPRsLoadedMsg:
		return m, m.handleOpenPRsLoaded(msg)

	case createFromChangesReadyMsg:
		return m, m.handleCreateFromChangesReady(msg)

	case prDataLoadedMsg, ciStatusLoadedMsg:
		return m.handlePRMessages(msg)

	case statusUpdatedMsg:
		if msg.info != "" {
			m.infoContent = msg.info
		}
		m.statusContent = msg.status
		if msg.log != nil {
			rows := make([]table.Row, 0, len(msg.log))
			for _, entry := range msg.log {
				rows = append(rows, table.Row{entry.sha, entry.message})
			}
			m.logTable.SetRows(rows)
			m.logEntries = msg.log
		}
		// Trigger CI fetch if worktree has a PR and cache is stale
		return m, m.maybeFetchCIStatus()

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

	case tmuxSessionReadyMsg:
		if msg.attach {
			return m, m.attachTmuxSessionCmd(msg.sessionName, msg.insideTmux)
		}
		message := buildTmuxInfoMessage(msg.sessionName, msg.insideTmux)
		m.infoScreen = NewInfoScreen(message, m.theme)
		m.currentScreen = screenInfo
		return m, nil

	case fetchRemotesCompleteMsg:
		m.loading = false
		m.statusContent = "Remotes fetched"
		return m, m.refreshWorktrees()

	case commitLoadingMsg:
		m.currentScreen = screenCommit
		m.commitScreen = NewCommitScreen(msg.meta, "Loading…", "", m.git.UseDelta(), m.theme)
		return m, nil

	case commitLoadedMsg:
		m.commitScreen = NewCommitScreen(msg.meta, msg.stat, msg.diff, m.git.UseDelta(), m.theme)
		m.currentScreen = screenCommit
		return m, nil
	}

	return m, tea.Batch(cmds...)
}

// handleKeyMsg processes keyboard input when not in a modal screen.
func (m *Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Handle filter input first - when filtering, only escape/enter should exit
	if m.showingFilter {
		keyStr := msg.String()
		if keyStr == keyEnter || isEscKey(keyStr) {
			m.showingFilter = false
			m.filterInput.Blur()
			m.worktreeTable.Focus()
			return m, nil
		}
		m.filterInput, cmd = m.filterInput.Update(msg)
		m.filterQuery = m.filterInput.Value()
		m.updateTable()
		return m, cmd
	}

	// Check for custom commands first - allows users to override built-in keys
	if _, ok := m.config.CustomCommands[msg.String()]; ok {
		return m, m.executeCustomCommand(msg.String())
	}

	return m.handleBuiltInKey(msg)
}

// handleBuiltInKey processes built-in keyboard shortcuts.
func (m *Model) handleBuiltInKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", keyQ:
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

	case "tab", "]":
		m.focusedPane = (m.focusedPane + 1) % 3
		switch m.focusedPane {
		case 0:
			m.worktreeTable.Focus()
		case 2:
			m.logTable.Focus()
		}
		return m, nil

	case "[":
		m.focusedPane = (m.focusedPane - 1 + 3) % 3
		switch m.focusedPane {
		case 0:
			m.worktreeTable.Focus()
		case 2:
			m.logTable.Focus()
		}
		return m, nil

	case "j", "down":
		return m.handleNavigationDown(msg)

	case "k", "up":
		return m.handleNavigationUp(msg)

	case "ctrl+d", " ":
		return m.handlePageDown(msg)

	case "ctrl+u":
		return m.handlePageUp(msg)

	case "pgdown":
		return m.handlePageDown(msg)

	case "pgup":
		return m.handlePageUp(msg)

	case "G":
		if m.focusedPane == 1 {
			m.statusViewport.GotoBottom()
			return m, nil
		}
		return m, nil

	case keyEnter:
		return m.handleEnterKey()

	case "r":
		m.loading = true
		return m, m.refreshWorktrees()

	case "c":
		return m, m.showCreateWorktree()

	case "D":
		return m, m.showDeleteWorktree()

	case "d":
		return m, m.showDiff()

	case "p":
		m.ciCache = make(map[string]*ciCacheEntry)
		if !m.prDataLoaded {
			m.loading = true
			return m, m.fetchPRData()
		}
		return m, m.maybeFetchCIStatus()

	case "R":
		m.loading = true
		m.statusContent = "Fetching remotes..."
		return m, m.fetchRemotes()

	case "f", "/":
		m.showingFilter = true
		m.filterInput.Focus()
		return m, textinput.Blink

	case "s":
		m.sortByActive = !m.sortByActive
		m.updateTable()
		return m, nil

	case "ctrl+p", "P":
		return m, m.showCommandPalette()

	case "?":
		m.currentScreen = screenHelp
		m.helpScreen = NewHelpScreen(m.windowWidth, m.windowHeight, m.config.CustomCommands, m.theme)
		return m, nil

	case "g":
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

	case keyEsc, keyEscRaw:
		if m.currentScreen == screenPalette {
			m.currentScreen = screenNone
			m.paletteScreen = nil
			return m, nil
		}
		return m, nil
	}

	// Handle table input
	if m.focusedPane == 0 {
		var cmd tea.Cmd
		m.worktreeTable, cmd = m.worktreeTable.Update(msg)
		return m, tea.Batch(cmd, m.debouncedUpdateDetailsView())
	}

	return m, nil
}

// handleNavigationDown processes down arrow and 'j' key navigation.
func (m *Model) handleNavigationDown(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

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
}

// handleNavigationUp processes up arrow and 'k' key navigation.
func (m *Model) handleNavigationUp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	cmds := []tea.Cmd{}

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
}

// handlePageDown processes page down navigation.
func (m *Model) handlePageDown(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch m.focusedPane {
	case 1:
		m.statusViewport.HalfPageDown()
		return m, nil
	case 2:
		m.logTable, cmd = m.logTable.Update(msg)
		return m, cmd
	}
	return m, nil
}

// handlePageUp processes page up navigation.
func (m *Model) handlePageUp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch m.focusedPane {
	case 1:
		m.statusViewport.HalfPageUp()
		return m, nil
	case 2:
		m.logTable, cmd = m.logTable.Update(msg)
		return m, cmd
	}
	return m, nil
}

// handleEnterKey processes the Enter key based on focused pane.
func (m *Model) handleEnterKey() (tea.Model, tea.Cmd) {
	switch m.focusedPane {
	case 0:
		// Jump to worktree
		if m.selectedIndex >= 0 && m.selectedIndex < len(m.filteredWts) {
			selectedPath := m.filteredWts[m.selectedIndex].Path
			m.persistLastSelected(selectedPath)
			m.selectedPath = selectedPath
			return m, tea.Quit
		}
	case 2:
		// Open commit view
		return m, m.openCommitView()
	}
	return m, nil
}

// handleWorktreeMessages processes worktree-related messages.
func (m *Model) handleWorktreeMessages(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case worktreesLoadedMsg:
		return m.handleWorktreesLoaded(msg)
	case cachedWorktreesMsg:
		return m.handleCachedWorktrees(msg)
	case pruneResultMsg:
		return m.handlePruneResult(msg)
	case absorbMergeResultMsg:
		return m.handleAbsorbResult(msg)
	default:
		return m, nil
	}
}

// handleWorktreesLoaded processes worktrees loaded message.
func (m *Model) handleWorktreesLoaded(msg worktreesLoadedMsg) (tea.Model, tea.Cmd) {
	m.worktreesLoaded = true
	m.loading = false
	if msg.err != nil {
		m.statusContent = fmt.Sprintf("Error loading worktrees: %v", msg.err)
		return m, nil
	}
	m.worktrees = msg.worktrees
	m.detailsCache = make(map[string]*detailsCacheEntry)
	m.ensureRepoConfig()
	m.updateTable()
	m.saveCache()
	if len(m.worktrees) == 0 {
		cwd, _ := os.Getwd()
		m.welcomeScreen = NewWelcomeScreen(cwd, m.getWorktreeDir(), m.theme)
		m.currentScreen = screenWelcome
		return m, nil
	}
	if m.currentScreen == screenWelcome {
		m.currentScreen = screenNone
		m.welcomeScreen = nil
	}
	if m.config.AutoFetchPRs && !m.prDataLoaded {
		m.loading = true
		return m, m.fetchPRData()
	}
	return m, m.updateDetailsView()
}

// handleCachedWorktrees processes cached worktrees message.
func (m *Model) handleCachedWorktrees(msg cachedWorktreesMsg) (tea.Model, tea.Cmd) {
	if m.worktreesLoaded || len(msg.worktrees) == 0 {
		return m, nil
	}
	m.worktrees = msg.worktrees
	m.updateTable()
	if m.selectedIndex >= 0 && m.selectedIndex < len(m.filteredWts) {
		m.infoContent = m.buildInfoContent(m.filteredWts[m.selectedIndex])
	}
	m.statusContent = "Refreshing worktrees..."
	return m, nil
}

// handlePruneResult processes prune result message.
func (m *Model) handlePruneResult(msg pruneResultMsg) (tea.Model, tea.Cmd) {
	m.loading = false
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

// handleAbsorbResult processes absorb merge result message.
func (m *Model) handleAbsorbResult(msg absorbMergeResultMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.statusContent = msg.err.Error()
		return m, nil
	}
	cmd := m.deleteWorktreeCmd(&models.WorktreeInfo{Path: msg.path, Branch: msg.branch})
	if cmd != nil {
		return m, cmd()
	}
	return m, nil
}

// handlePRMessages processes PR and CI-related messages.
func (m *Model) handlePRMessages(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case prDataLoadedMsg:
		return m.handlePRDataLoaded(msg)
	case ciStatusLoadedMsg:
		return m.handleCIStatusLoaded(msg)
	default:
		return m, nil
	}
}

// handlePRDataLoaded processes PR data loaded message.
func (m *Model) handlePRDataLoaded(msg prDataLoadedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err == nil && msg.prMap != nil {
		for _, wt := range m.worktrees {
			if pr, ok := msg.prMap[wt.Branch]; ok {
				wt.PR = pr
			}
		}
		m.prDataLoaded = true
		// Update columns before rows to include the PR column
		m.updateTableColumns(m.worktreeTable.Width())
		m.updateTable()
		return m, m.updateDetailsView()
	}
	return m, nil
}

// handleCIStatusLoaded processes CI status loaded message.
func (m *Model) handleCIStatusLoaded(msg ciStatusLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err == nil && msg.checks != nil {
		m.ciCache[msg.branch] = &ciCacheEntry{
			checks:    msg.checks,
			fetchedAt: time.Now(),
		}
		// Refresh info content to show CI status
		if m.selectedIndex >= 0 && m.selectedIndex < len(m.filteredWts) {
			wt := m.filteredWts[m.selectedIndex]
			if wt.Branch == msg.branch {
				m.infoContent = m.buildInfoContent(wt)
			}
		}
	}
	return m, nil
}

func screenName(screen screenType) string {
	switch screen {
	case screenNone:
		return "none"
	case screenConfirm:
		return "confirm"
	case screenInfo:
		return "info"
	case screenInput:
		return "input"
	case screenHelp:
		return "help"
	case screenTrust:
		return "trust"
	case screenWelcome:
		return "welcome"
	case screenCommit:
		return "commit"
	case screenPalette:
		return "palette"
	case screenDiff:
		return "diff"
	case screenPRSelect:
		return "pr-select"
	case screenListSelect:
		return "list-select"
	default:
		return "unknown"
	}
}

// handleMouse processes mouse events for scrolling and clicking
func (m *Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Handle mouse scrolling for specific screens
	if m.currentScreen == screenCommit && m.commitScreen != nil {
		if msg.Action == tea.MouseActionPress {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				m.commitScreen.viewport.ScrollUp(3)
				return m, nil
			case tea.MouseButtonWheelDown:
				m.commitScreen.viewport.ScrollDown(3)
				return m, nil
			}
		}
		return m, nil
	}

	// Skip mouse handling when on other modal screens
	if m.currentScreen != screenNone {
		return m, nil
	}

	var cmds []tea.Cmd
	layout := m.computeLayout()

	// Calculate pane boundaries (accounting for header and filter)
	headerOffset := 1
	if m.showingFilter {
		headerOffset = 2
	}

	// Left pane boundaries (worktree table)
	leftX := 0
	leftY := headerOffset
	leftMaxX := layout.leftWidth
	leftMaxY := headerOffset + layout.bodyHeight

	// Right top pane boundaries (info/diff viewport)
	rightX := layout.leftWidth + layout.gapX
	rightTopY := headerOffset
	rightTopMaxX := rightX + layout.rightWidth
	rightTopMaxY := headerOffset + layout.rightTopHeight

	// Right bottom pane boundaries (log table)
	rightBottomY := headerOffset + layout.rightTopHeight + layout.gapY
	rightBottomMaxY := headerOffset + layout.bodyHeight

	// Determine which pane the mouse is in
	mouseX := msg.X
	mouseY := msg.Y

	targetPane := -1
	switch {
	case mouseX >= leftX && mouseX < leftMaxX && mouseY >= leftY && mouseY < leftMaxY:
		targetPane = 0 // Worktree table
	case mouseX >= rightX && mouseX < rightTopMaxX && mouseY >= rightTopY && mouseY < rightTopMaxY:
		targetPane = 1 // Info/Diff viewport
	case mouseX >= rightX && mouseX < rightTopMaxX && mouseY >= rightBottomY && mouseY < rightBottomMaxY:
		targetPane = 2 // Log table
	}

	switch {
	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft:
		// Click to focus pane and select item
		if targetPane >= 0 && targetPane != m.focusedPane {
			m.focusedPane = targetPane
			switch m.focusedPane {
			case 0:
				m.worktreeTable.Focus()
			case 2:
				m.logTable.Focus()
			}
		}

		// Handle clicks within the pane to select items
		if targetPane == 0 && len(m.filteredWts) > 0 {
			// Calculate which row was clicked in the worktree table
			// Account for pane border and title
			relativeY := mouseY - leftY - 4
			if relativeY >= 0 && relativeY < len(m.filteredWts) {
				// Create a key message to move cursor
				for i := 0; i < len(m.filteredWts); i++ {
					if i == relativeY {
						m.worktreeTable.SetCursor(i)
						cmds = append(cmds, m.debouncedUpdateDetailsView())
						break
					}
				}
			}
		} else if targetPane == 2 && len(m.logEntries) > 0 {
			// Calculate which row was clicked in the log table
			relativeY := mouseY - rightBottomY - 4
			if relativeY >= 0 && relativeY < len(m.logEntries) {
				m.logTable.SetCursor(relativeY)
			}
		}

	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonWheelUp:
		switch targetPane {
		case 0:
			// Scroll worktree table up
			m.worktreeTable, _ = m.worktreeTable.Update(tea.KeyMsg{Type: tea.KeyUp})
			cmds = append(cmds, m.debouncedUpdateDetailsView())
		case 1:
			// Scroll viewport up
			m.statusViewport.ScrollUp(3)
		case 2:
			// Scroll log table up
			m.logTable, _ = m.logTable.Update(tea.KeyMsg{Type: tea.KeyUp})
		}

	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonWheelDown:
		switch targetPane {
		case 0:
			// Scroll worktree table down
			m.worktreeTable, _ = m.worktreeTable.Update(tea.KeyMsg{Type: tea.KeyDown})
			cmds = append(cmds, m.debouncedUpdateDetailsView())
		case 1:
			// Scroll viewport down
			m.statusViewport.ScrollDown(3)
		case 2:
			// Scroll log table down
			m.logTable, _ = m.logTable.Update(tea.KeyMsg{Type: tea.KeyDown})
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders the active screen for the Bubble Tea program.
func (m *Model) View() string {
	if m.quitting {
		return ""
	}

	// Wait for window size before rendering full UI
	if m.windowWidth == 0 || m.windowHeight == 0 {
		return "Loading..."
	}

	// Always render base layout first to allow overlays
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

	baseView := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Handle Modal Overlays
	switch m.currentScreen {
	case screenPalette:
		if m.paletteScreen != nil {
			return m.overlayPopup(baseView, m.paletteScreen.View(), 3)
		}
	case screenPRSelect:
		if m.prSelectionScreen != nil {
			return m.overlayPopup(baseView, m.prSelectionScreen.View(), 2)
		}
	case screenListSelect:
		if m.listScreen != nil {
			return m.overlayPopup(baseView, m.listScreen.View(), 2)
		}
	case screenHelp:
		if m.helpScreen != nil {
			// Center the help popup
			// Help screen has fixed/capped size logic in NewHelpScreen/SetSize
			// We can pass 0,0 to use its internal defaults or a specific size
			// In SetSize below we'll ensure it has a good "popup" size
			return m.overlayPopup(baseView, m.helpScreen.View(), 4)
		}
	case screenCommit:
		if m.commitScreen != nil {
			// Resize viewport to fit window
			vpWidth := int(float64(m.windowWidth) * 0.95)
			vpHeight := int(float64(m.windowHeight) * 0.85)
			if vpWidth < 80 {
				vpWidth = 80
			}
			if vpHeight < 20 {
				vpHeight = 20
			}
			m.commitScreen.viewport.Width = vpWidth
			m.commitScreen.viewport.Height = vpHeight
			return m.overlayPopup(baseView, m.commitScreen.View(), 2)
		}
	case screenConfirm:
		if m.confirmScreen != nil {
			return m.overlayPopup(baseView, m.confirmScreen.View(), 5)
		}
	case screenInfo:
		if m.infoScreen != nil {
			return m.overlayPopup(baseView, m.infoScreen.View(), 5)
		}
	case screenInput:
		if m.inputScreen != nil {
			return m.overlayPopup(baseView, m.inputScreen.View(), 5)
		}
	}

	// Handle Full Screen Views (fallback)
	if m.currentScreen != screenNone {
		return m.renderScreen()
	}

	return baseView
}

func (m *Model) overlayPopup(base, popup string, marginTop int) string {
	if base == "" || popup == "" {
		return base
	}

	baseLines := strings.Split(base, "\n")
	popupLines := strings.Split(popup, "\n")

	// Assume fixed width for now or calculate from lines
	if len(baseLines) == 0 {
		return popup
	}

	baseWidth := lipgloss.Width(baseLines[0])
	popupWidth := lipgloss.Width(popupLines[0]) // Assume box is rectangular

	// Calculate left padding to center
	leftPad := (baseWidth - popupWidth) / 2
	if leftPad < 0 {
		leftPad = 0
	}

	// "Clear" styling for the background band
	// We use the default terminal background color (reset)
	leftSpace := strings.Repeat(" ", leftPad)
	rightPad := baseWidth - popupWidth - leftPad
	if rightPad < 0 {
		rightPad = 0
	}
	rightSpace := strings.Repeat(" ", rightPad)

	for i, line := range popupLines {
		row := marginTop + i
		if row >= len(baseLines) {
			break
		}

		// Simply replacing the content in the middle with the popup line
		// The spaces ensure we overwrite underlying text
		baseLines[row] = leftSpace + line + rightSpace
	}

	return strings.Join(baseLines, "\n")
}

// Helper methods

func (m *Model) updateTable() {
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
				name = mainWorktreeName
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
			name = mainWorktreeName
		}

		status := "✔"
		if wt.Dirty {
			status = "✎"
		}

		// Build lazygit-style sync status: ↓N↑M, ✓ (in sync), or - (no upstream)
		var abStr string
		switch {
		case !wt.HasUpstream:
			abStr = "-"
		case wt.Ahead == 0 && wt.Behind == 0:
			abStr = "✓"
		default:
			var parts []string
			if wt.Behind > 0 {
				parts = append(parts, fmt.Sprintf("↓%d", wt.Behind))
			}
			if wt.Ahead > 0 {
				parts = append(parts, fmt.Sprintf("↑%d", wt.Ahead))
			}
			abStr = strings.Join(parts, "")
		}

		row := table.Row{
			name,
			status,
			abStr,
			wt.LastActive,
		}

		// Only include PR column if PR data has been loaded
		if m.prDataLoaded {
			prStr := "-"
			if wt.PR != nil {
				// Use Unicode symbols to indicate PR state
				var stateSymbol string
				switch wt.PR.State {
				case "OPEN":
					stateSymbol = "●"
				case "MERGED":
					stateSymbol = "◆"
				case "CLOSED":
					stateSymbol = "✕"
				default:
					stateSymbol = "?"
				}
				prStr = fmt.Sprintf("#%d %s", wt.PR.Number, stateSymbol)
			}
			row = append(row, prStr)
		}

		rows = append(rows, row)
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

func (m *Model) updateDetailsView() tea.Cmd {
	m.selectedIndex = m.worktreeTable.Cursor()
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}

	wt := m.filteredWts[m.selectedIndex]
	if !m.worktreesLoaded {
		m.infoContent = m.buildInfoContent(wt)
		if m.statusContent == "" || m.statusContent == "Loading..." {
			m.statusContent = "Refreshing worktrees..."
		}
		return nil
	}
	return func() tea.Msg {
		statusRaw, logRaw := m.getCachedDetails(wt)

		// Parse log
		logEntries := []commitLogEntry{}
		for line := range strings.SplitSeq(logRaw, "\n") {
			parts := strings.SplitN(line, "\t", 2)
			if len(parts) == 2 {
				message := parts[1]
				// Truncate long commit messages to 80 characters
				if len(message) > 80 {
					message = message[:80] + "…"
				}
				logEntries = append(logEntries, commitLogEntry{
					sha:     parts[0],
					message: message,
				})
			}
		}
		statusContent := m.buildStatusContent(statusRaw)

		return statusUpdatedMsg{
			info:   m.buildInfoContent(wt),
			status: statusContent,
			log:    logEntries,
		}
	}
}

func (m *Model) debouncedUpdateDetailsView() tea.Cmd {
	// Cancel any existing pending detail update
	if m.detailUpdateCancel != nil {
		m.detailUpdateCancel()
		m.detailUpdateCancel = nil
	}

	// Get current selected index
	m.pendingDetailsIndex = m.worktreeTable.Cursor()
	selectedIndex := m.pendingDetailsIndex

	ctx, cancel := context.WithCancel(context.Background())
	m.detailUpdateCancel = cancel

	return func() tea.Msg {
		timer := time.NewTimer(debounceDelay)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
		}
		return debouncedDetailsMsg{
			selectedIndex: selectedIndex,
		}
	}
}

func (m *Model) refreshWorktrees() tea.Cmd {
	return func() tea.Msg {
		worktrees, err := m.git.GetWorktrees(m.ctx)
		return worktreesLoadedMsg{
			worktrees: worktrees,
			err:       err,
		}
	}
}

func (m *Model) fetchPRData() tea.Cmd {
	return func() tea.Msg {
		prMap, err := m.git.FetchPRMap(m.ctx)
		return prDataLoadedMsg{
			prMap: prMap,
			err:   err,
		}
	}
}

func (m *Model) fetchCIStatus(prNumber int, branch string) tea.Cmd {
	return func() tea.Msg {
		checks, err := m.git.FetchCIStatus(m.ctx, prNumber, branch)
		return ciStatusLoadedMsg{
			branch: branch,
			checks: checks,
			err:    err,
		}
	}
}

// maybeFetchCIStatus triggers CI fetch for current worktree if it has a PR and cache is stale.
func (m *Model) maybeFetchCIStatus() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	wt := m.filteredWts[m.selectedIndex]
	if wt.PR == nil {
		return nil
	}

	// Check cache - skip if fresh (within ciCacheTTL)
	if cached, ok := m.ciCache[wt.Branch]; ok {
		if time.Since(cached.fetchedAt) < ciCacheTTL {
			return nil
		}
	}

	return m.fetchCIStatus(wt.PR.Number, wt.Branch)
}

func (m *Model) fetchRemotes() tea.Cmd {
	return func() tea.Msg {
		m.git.RunGit(m.ctx, []string{"git", "fetch", "--all", "--quiet"}, "", []int{0}, false, false)
		return fetchRemotesCompleteMsg{}
	}
}

func (m *Model) showCreateWorktree() tea.Cmd {
	defaultBase := m.git.GetMainBranch(m.ctx)
	return m.showBaseSelection(defaultBase)
}

func (m *Model) showCreateFromPR() tea.Cmd {
	// Fetch all open PRs
	return func() tea.Msg {
		prs, err := m.git.FetchAllOpenPRs(m.ctx)
		return openPRsLoadedMsg{
			prs: prs,
			err: err,
		}
	}
}

func (m *Model) handleOpenPRsLoaded(msg openPRsLoadedMsg) tea.Cmd {
	if msg.err != nil {
		m.statusContent = fmt.Sprintf("Failed to fetch PRs: %v", msg.err)
		return nil
	}

	if len(msg.prs) == 0 {
		m.statusContent = "No open PRs/MRs found."
		return nil
	}

	// Show PR selection screen
	m.prSelectionScreen = NewPRSelectionScreen(msg.prs, m.windowWidth, m.windowHeight, m.theme)
	m.prSelectionSubmit = func(pr *models.PRInfo) tea.Cmd {
		// Generate worktree name
		generatedName := generatePRWorktreeName(pr)

		// Show input screen with generated name
		m.inputScreen = NewInputScreen(
			fmt.Sprintf("Create worktree from PR #%d", pr.Number),
			"Worktree name",
			generatedName,
			m.theme,
		)
		m.inputSubmit = func(value string) (tea.Cmd, bool) {
			newBranch := strings.TrimSpace(value)
			if newBranch == "" {
				m.inputScreen.errorMsg = errBranchEmpty
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

			// Validate that PR has a branch
			if pr.Branch == "" {
				m.inputScreen.errorMsg = "PR branch information is missing."
				return nil, false
			}

			m.inputScreen.errorMsg = ""
			if err := os.MkdirAll(m.getWorktreeDir(), defaultDirPerms); err != nil {
				return func() tea.Msg { return errMsg{err: fmt.Errorf("failed to create worktree directory: %w", err)} }, true
			}

			// Create worktree from PR branch
			ok := m.git.CreateWorktreeFromPR(m.ctx, pr.Number, pr.Branch, newBranch, targetPath)
			if !ok {
				return func() tea.Msg {
					return errMsg{err: fmt.Errorf("failed to create worktree from PR #%d (branch: %s)", pr.Number, pr.Branch)}
				}, true
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
		m.currentScreen = screenInput
		return textinput.Blink
	}
	m.currentScreen = screenPRSelect
	return textinput.Blink
}

func (m *Model) showCreateWorktreeFromChanges() tea.Cmd {
	// Check if a worktree is selected
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		m.statusContent = errNoWorktreeSelected
		return nil
	}

	wt := m.filteredWts[m.selectedIndex]

	// Check for changes in the selected worktree asynchronously
	return func() tea.Msg {
		statusRaw := m.git.RunGit(m.ctx, []string{"git", "status", "--porcelain"}, wt.Path, []int{0}, true, false)
		if strings.TrimSpace(statusRaw) == "" {
			return errMsg{err: fmt.Errorf("no changes to move")}
		}

		// Get current branch name
		currentBranch := m.git.RunGit(m.ctx, []string{"git", "rev-parse", "--abbrev-ref", "HEAD"}, wt.Path, []int{0}, true, false)
		if currentBranch == "" {
			return errMsg{err: fmt.Errorf("failed to get current branch")}
		}

		// Get diff if branch_name_script is configured
		var diff string
		if m.config.BranchNameScript != "" {
			diff = m.git.RunGit(m.ctx, []string{"git", "diff", "HEAD"}, wt.Path, []int{0}, false, true)
		}

		return createFromChangesReadyMsg{
			worktree:      wt,
			currentBranch: currentBranch,
			diff:          diff,
		}
	}
}

func (m *Model) handleCreateFromChangesReady(msg createFromChangesReadyMsg) tea.Cmd {
	wt := msg.worktree
	currentBranch := msg.currentBranch

	// Generate default name based on current branch
	defaultName := fmt.Sprintf("%s-changes", currentBranch)

	// If branch_name_script is configured, run it to generate a suggested name
	if m.config.BranchNameScript != "" && msg.diff != "" {
		if generatedName, err := runBranchNameScript(m.ctx, m.config.BranchNameScript, msg.diff); err != nil {
			// Log error but continue with default name
			m.statusContent = fmt.Sprintf("Branch name script error: %v", err)
		} else if generatedName != "" {
			defaultName = generatedName
		}
	}

	// Show input screen for worktree name
	m.inputScreen = NewInputScreen("Create worktree from changes: branch name", "feature/my-branch", defaultName, m.theme)
	m.inputSubmit = func(value string) (tea.Cmd, bool) {
		newBranch := strings.TrimSpace(value)
		if newBranch == "" {
			m.inputScreen.errorMsg = errBranchEmpty
			return nil, false
		}

		// Prevent duplicates - check if branch already exists in worktrees
		for _, existingWt := range m.worktrees {
			if existingWt.Branch == newBranch {
				m.inputScreen.errorMsg = fmt.Sprintf("Branch %q already exists.", newBranch)
				return nil, false
			}
		}

		// Check if branch exists in git
		branchRef := m.git.RunGit(m.ctx, []string{"git", "show-ref", fmt.Sprintf("refs/heads/%s", newBranch)}, "", []int{0, 1}, true, true)
		if branchRef != "" {
			// Branch exists
			m.inputScreen.errorMsg = fmt.Sprintf("Branch %q already exists.", newBranch)
			return nil, false
		}

		// Check if worktree path already exists
		targetPath := filepath.Join(m.getWorktreeDir(), newBranch)
		if _, err := os.Stat(targetPath); err == nil {
			m.inputScreen.errorMsg = fmt.Sprintf("Path already exists: %s", targetPath)
			return nil, false
		}

		m.inputScreen.errorMsg = ""
		if err := os.MkdirAll(m.getWorktreeDir(), 0o750); err != nil {
			return func() tea.Msg { return errMsg{err: fmt.Errorf("failed to create worktree directory: %w", err)} }, true
		}

		// Stash changes with descriptive message
		stashMessage := fmt.Sprintf("git-wt-create move-current: %s", newBranch)
		if !m.git.RunCommandChecked(
			m.ctx,
			[]string{"git", "stash", "push", "-u", "-m", stashMessage},
			wt.Path,
			"Failed to create stash for moving changes",
		) {
			return func() tea.Msg { return errMsg{err: fmt.Errorf("failed to create stash for moving changes")} }, true
		}

		// Get the stash ref
		stashRef := m.git.RunGit(m.ctx, []string{"git", "stash", "list", "-1", "--format=%gd"}, wt.Path, []int{0}, true, false)
		if stashRef == "" || !strings.HasPrefix(stashRef, "stash@{") {
			// Try to restore stash if we can't get the ref
			m.git.RunCommandChecked(m.ctx, []string{"git", "stash", "pop"}, wt.Path, "Failed to restore stash")
			return func() tea.Msg { return errMsg{err: fmt.Errorf("failed to get stash reference")} }, true
		}

		// Create the new worktree from current branch
		if !m.git.RunCommandChecked(
			m.ctx,
			[]string{"git", "worktree", "add", "-b", newBranch, targetPath, currentBranch},
			"",
			fmt.Sprintf("Failed to create worktree %s", newBranch),
		) {
			// If worktree creation fails, try to restore the stash
			m.git.RunCommandChecked(m.ctx, []string{"git", "stash", "pop"}, wt.Path, "Failed to restore stash")
			return func() tea.Msg { return errMsg{err: fmt.Errorf("failed to create worktree %s", newBranch)} }, true
		}

		// Apply stash to the new worktree
		if !m.git.RunCommandChecked(
			m.ctx,
			[]string{"git", "stash", "apply", "--index", stashRef},
			targetPath,
			"Failed to apply stash to new worktree",
		) {
			// If stash apply fails, clean up the worktree and try to restore stash to original location
			m.git.RunCommandChecked(m.ctx, []string{"git", "worktree", "remove", "--force", targetPath}, "", "Failed to remove worktree")
			m.git.RunCommandChecked(m.ctx, []string{"git", "stash", "pop"}, wt.Path, "Failed to restore stash")
			return func() tea.Msg { return errMsg{err: fmt.Errorf("failed to apply stash to new worktree")} }, true
		}

		// Drop the stash from the original location
		m.git.RunCommandChecked(m.ctx, []string{"git", "stash", "drop", stashRef}, wt.Path, "Failed to drop stash")

		// Run init commands and refresh
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
	m.currentScreen = screenInput
	return textinput.Blink
}

func (m *Model) showDeleteWorktree() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	wt := m.filteredWts[m.selectedIndex]
	if wt.IsMain {
		return nil
	}
	m.confirmScreen = NewConfirmScreen(fmt.Sprintf("Delete worktree?\n\nPath: %s\nBranch: %s", wt.Path, wt.Branch), m.theme)
	m.confirmAction = m.deleteWorktreeCmd(wt)
	m.currentScreen = screenConfirm
	return nil
}

func (m *Model) showDiff() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	wt := m.filteredWts[m.selectedIndex]
	return func() tea.Msg {
		diff := m.git.BuildThreePartDiff(m.ctx, wt.Path, m.config)
		if strings.TrimSpace(diff) == "" {
			return statusUpdatedMsg{
				info:   m.buildInfoContent(wt),
				status: fmt.Sprintf("No diff for %s.", wt.Branch),
				log:    nil,
			}
		}
		diff = m.git.ApplyDelta(m.ctx, diff)
		m.diffScreen = NewDiffScreen(fmt.Sprintf("Diff for %s", wt.Branch), diff, m.theme)
		m.currentScreen = screenDiff
		return nil
	}
}

func (m *Model) showRenameWorktree() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}

	wt := m.filteredWts[m.selectedIndex]
	if wt.IsMain {
		m.statusContent = "Cannot rename the main worktree."
		return nil
	}

	prompt := fmt.Sprintf("Enter new name for '%s'", wt.Branch)
	m.inputScreen = NewInputScreen(prompt, "New branch name", wt.Branch, m.theme)
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

func (m *Model) showPruneMerged() tea.Cmd {
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

	m.confirmScreen = NewConfirmScreen("Prune merged PR worktrees?\n\n"+strings.Join(lines, "\n"), m.theme)
	m.confirmAction = func() tea.Cmd {
		// Collect terminate commands once (same for all worktrees in this repo)
		terminateCmds := m.collectTerminateCommands()

		// Build the prune routine that runs terminate commands per-worktree
		pruneRoutine := func() tea.Msg {
			pruned := 0
			failed := 0
			for _, wt := range merged {
				// Run terminate commands for each worktree with its environment
				if len(terminateCmds) > 0 {
					env := m.buildCommandEnv(wt.Branch, wt.Path)
					_ = m.git.ExecuteCommands(m.ctx, terminateCmds, wt.Path, env)
				}

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

		// Check trust for repo commands before running
		return m.runCommandsWithTrust(terminateCmds, "", nil, pruneRoutine)
	}
	m.currentScreen = screenConfirm
	return nil
}

// showAbsorbWorktree merges selected branch into main and removes the worktree
func (m *Model) showAbsorbWorktree() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	wt := m.filteredWts[m.selectedIndex]
	if wt.IsMain {
		m.statusContent = "Cannot absorb the main worktree."
		return nil
	}

	mainBranch := m.git.GetMainBranch(m.ctx)

	// Find the main worktree explicitly (don't use fallback)
	var mainPath string
	for _, w := range m.worktrees {
		if w.IsMain {
			mainPath = w.Path
			break
		}
	}
	if mainPath == "" {
		m.statusContent = "Cannot find main worktree."
		return nil
	}

	m.confirmScreen = NewConfirmScreen(fmt.Sprintf("Absorb worktree into %s?\n\nPath: %s\nBranch: %s -> %s", mainBranch, wt.Path, wt.Branch, mainBranch), m.theme)
	m.confirmAction = func() tea.Cmd {
		return func() tea.Msg {
			if !m.git.RunCommandChecked(m.ctx, []string{"git", "-C", mainPath, "merge", "--no-edit", wt.Branch}, "", fmt.Sprintf("Failed to merge %s into %s", wt.Branch, mainBranch)) {
				return absorbMergeResultMsg{
					path:   wt.Path,
					branch: wt.Branch,
					err:    fmt.Errorf("merge failed; resolve conflicts in %s and retry", mainPath),
				}
			}

			return absorbMergeResultMsg{
				path:   wt.Path,
				branch: wt.Branch,
			}
		}
	}
	m.currentScreen = screenConfirm
	return nil
}

func (m *Model) showCommandPalette() tea.Cmd {
	m.debugf("open palette")
	items := []paletteItem{
		{id: "create", label: "Create worktree (c)", description: "Add a new worktree from base branch"},
		{id: "create-from-pr", label: "Create from PR/MR", description: "Create a worktree from a pull/merge request"},
		{id: "create-from-changes", label: "Create from changes", description: "Create a new worktree from current uncommitted changes"},
		{id: "delete", label: "Delete worktree (D)", description: "Remove worktree and branch"},
		{id: "rename", label: "Rename worktree (m)", description: "Rename worktree and branch"},
		{id: "absorb", label: "Absorb worktree (A)", description: "Merge branch into main and remove worktree"},
		{id: "prune", label: "Prune merged (X)", description: "Remove merged PR worktrees"},
		{id: "refresh", label: "Refresh (r)", description: "Reload worktrees"},
		{id: "fetch", label: "Fetch remotes (R)", description: "git fetch --all"},
		{id: "pr", label: "Open PR (o)", description: "Open PR in browser"},
		{id: "help", label: "Help (?)", description: "Show help"},
	}
	items = append(items, m.customPaletteItems()...)

	m.paletteScreen = NewCommandPaletteScreen(items, m.theme)
	m.paletteSubmit = func(action string) tea.Cmd {
		m.debugf("palette action: %s", action)
		if _, ok := m.config.CustomCommands[action]; ok {
			return m.executeCustomCommand(action)
		}
		switch action {
		case "create":
			return m.showCreateWorktree()
		case "create-from-pr":
			return m.showCreateFromPR()
		case "create-from-changes":
			return m.showCreateWorktreeFromChanges()
		case "delete":
			return m.showDeleteWorktree()
		case "rename":
			return m.showRenameWorktree()
		case "absorb":
			return m.showAbsorbWorktree()
		case "prune":
			return m.showPruneMerged()
		case "diff":
			return m.showDiff()
		case "refresh":
			return m.refreshWorktrees()
		case "fetch":
			return m.fetchRemotes()
		case "pr":
			return m.openPR()
		case "help":
			m.currentScreen = screenHelp
			return nil
		}
		return nil
	}
	m.currentScreen = screenPalette
	return textinput.Blink
}

func (m *Model) customPaletteItems() []paletteItem {
	keys := m.customCommandKeys()
	if len(keys) == 0 {
		return nil
	}

	items := make([]paletteItem, 0, len(keys))
	for _, key := range keys {
		cmd := m.config.CustomCommands[key]
		if cmd == nil {
			continue
		}
		label := m.customCommandLabel(cmd, key)
		description := customCommandPlaceholder
		if cmd.Command != "" {
			description = cmd.Command
		} else if cmd.Tmux != nil {
			description = tmuxSessionLabel
		}
		items = append(items, paletteItem{
			id:          key,
			label:       label,
			description: description,
		})
	}

	return items
}

func (m *Model) customCommandKeys() []string {
	if len(m.config.CustomCommands) == 0 {
		return nil
	}

	keys := make([]string, 0, len(m.config.CustomCommands))
	for key, cmd := range m.config.CustomCommands {
		if cmd == nil {
			continue
		}
		if strings.TrimSpace(cmd.Command) == "" && cmd.Tmux == nil {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (m *Model) customCommandLabel(cmd *config.CustomCommand, key string) string {
	label := ""
	if cmd != nil {
		label = strings.TrimSpace(cmd.Description)
		if label == "" {
			label = strings.TrimSpace(cmd.Command)
			if label == "" && cmd.Tmux != nil {
				label = tmuxSessionLabel
			}
		}
	}
	if label == "" {
		label = customCommandPlaceholder
	}
	return fmt.Sprintf("%s (%s)", label, key)
}

func (m *Model) deleteWorktreeCmd(wt *models.WorktreeInfo) func() tea.Cmd {
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

func (m *Model) openLazyGit() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	wt := m.filteredWts[m.selectedIndex]

	c := m.commandRunner("lazygit")
	c.Dir = wt.Path

	return m.execProcess(c, func(err error) tea.Msg {
		if err != nil {
			return errMsg{err: err}
		}
		return refreshCompleteMsg{}
	})
}

func (m *Model) executeCustomCommand(key string) tea.Cmd {
	customCmd, ok := m.config.CustomCommands[key]
	if !ok || customCmd == nil {
		return nil
	}

	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}

	wt := m.filteredWts[m.selectedIndex]

	if customCmd.Tmux != nil {
		return m.openTmuxSession(customCmd.Tmux, wt)
	}

	// Set environment variables
	env := m.buildCommandEnv(wt.Branch, wt.Path)
	envVars := os.Environ()
	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	var c *exec.Cmd
	var cmdStr string
	if customCmd.Wait {
		// Wrap command with a pause prompt when wait is true
		cmdStr = fmt.Sprintf("%s; echo ''; echo 'Press any key to continue...'; read -n 1", customCmd.Command)
	} else {
		cmdStr = customCmd.Command
	}
	// Always run via shell to support pipes, redirects, and shell features
	// #nosec G204 -- command comes from user's own config file
	c = m.commandRunner("bash", "-c", cmdStr)

	c.Dir = wt.Path
	c.Env = envVars

	return m.execProcess(c, func(err error) tea.Msg {
		if err != nil {
			return errMsg{err: err}
		}
		return refreshCompleteMsg{}
	})
}

func (m *Model) openTmuxSession(tmuxCfg *config.TmuxCommand, wt *models.WorktreeInfo) tea.Cmd {
	if tmuxCfg == nil {
		return nil
	}

	env := m.buildCommandEnv(wt.Branch, wt.Path)
	insideTmux := os.Getenv("TMUX") != ""
	sessionName := expandWithEnv(tmuxCfg.SessionName, env)
	if strings.TrimSpace(sessionName) == "" {
		sessionName = fmt.Sprintf("wt:%s", filepath.Base(wt.Path))
	}

	resolved, ok := resolveTmuxWindows(tmuxCfg.Windows, env, wt.Path)
	if !ok {
		return func() tea.Msg {
			return errMsg{err: fmt.Errorf("failed to resolve tmux windows")}
		}
	}

	sessionFile, err := os.CreateTemp("", "lazyworktree-tmux-")
	if err != nil {
		return func() tea.Msg {
			return errMsg{err: err}
		}
	}
	sessionPath := sessionFile.Name()
	if closeErr := sessionFile.Close(); closeErr != nil {
		return func() tea.Msg {
			return errMsg{err: closeErr}
		}
	}

	scriptCfg := *tmuxCfg
	scriptCfg.Attach = false
	env["LW_TMUX_SESSION_FILE"] = sessionPath
	script := buildTmuxScript(sessionName, &scriptCfg, resolved, env)
	// #nosec G204 -- command is built from user-configured tmux session settings.
	c := m.commandRunner("bash", "-lc", script)
	c.Dir = wt.Path
	c.Env = append(os.Environ(), envMapToList(env)...)

	return m.execProcess(c, func(err error) tea.Msg {
		defer func() {
			_ = os.Remove(sessionPath)
		}()
		if err != nil {
			return errMsg{err: err}
		}
		finalSession := readTmuxSessionFile(sessionPath, sessionName)
		return tmuxSessionReadyMsg{
			sessionName: finalSession,
			attach:      tmuxCfg.Attach,
			insideTmux:  insideTmux,
		}
	})
}

func (m *Model) openPR() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	wt := m.filteredWts[m.selectedIndex]
	if wt.PR == nil {
		return nil
	}
	return func() tea.Msg {
		prURL, err := sanitizePRURL(wt.PR.URL)
		if err != nil {
			return errMsg{err: err}
		}

		var cmd *exec.Cmd
		switch runtime.GOOS {
		case osDarwin:
			// #nosec G204 -- the URL is sanitized and only executed directly as a single argument
			cmd = m.commandRunner("open", prURL)
		case osWindows:
			// #nosec G204 -- the URL is sanitized and only executed directly as a single argument
			cmd = m.commandRunner("rundll32", "url.dll,FileProtocolHandler", prURL)
		default:
			// #nosec G204 -- the URL is sanitized and only executed directly as a single argument
			cmd = m.commandRunner("xdg-open", prURL)
		}
		if err := m.startCommand(cmd); err != nil {
			return errMsg{err: err}
		}
		return nil
	}
}

func (m *Model) openCommitView() tea.Cmd {
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

	loadingMeta := commitMeta{
		sha:     entry.sha,
		subject: "Loading commit…",
	}

	fetchCmd := func() tea.Msg {
		metaRaw := m.git.RunGit(m.ctx, []string{"git", "show", "--quiet", "--pretty=format:%H%x1f%an%x1f%ae%x1f%ad%x1f%s%x1f%b", entry.sha}, wt.Path, []int{0}, false, false)
		stat := m.git.RunGit(m.ctx, []string{"git", "show", "--stat", "--pretty=format:", entry.sha}, wt.Path, []int{0}, false, false)
		diff := m.git.RunGit(m.ctx, []string{"git", "show", "--patch", "--pretty=format:", entry.sha}, wt.Path, []int{0}, false, false)
		diff = m.git.ApplyDelta(m.ctx, diff)

		meta := parseCommitMeta(metaRaw)
		return commitLoadedMsg{
			meta: meta,
			stat: stat,
			diff: diff,
		}
	}

	return tea.Batch(
		func() tea.Msg { return commitLoadingMsg{meta: loadingMeta} },
		fetchCmd,
	)
}

func parseCommitMeta(raw string) commitMeta {
	parts := strings.Split(raw, "\x1f")
	meta := commitMeta{}
	if len(parts) > 0 {
		meta.sha = parts[0]
	}
	if len(parts) > 1 {
		meta.author = parts[1]
	}
	if len(parts) > 2 {
		meta.email = parts[2]
	}
	if len(parts) > 3 {
		meta.date = parts[3]
	}
	if len(parts) > 4 {
		meta.subject = parts[4]
	}
	if len(parts) > 5 {
		meta.body = strings.Split(parts[5], "\n")
	}
	return meta
}

func sanitizePRURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("PR URL is empty")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid PR URL %q: %w", raw, err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("unsupported URL scheme %q", u.Scheme)
	}

	return u.String(), nil
}

func (m *Model) handleScreenKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.debugf("screen key: %s screen=%s", msg.String(), screenName(m.currentScreen))
	switch m.currentScreen {
	case screenHelp:
		if m.helpScreen == nil {
			m.helpScreen = NewHelpScreen(m.windowWidth, m.windowHeight, m.config.CustomCommands, m.theme)
		}
		keyStr := msg.String()
		if keyStr == keyQ || isEscKey(keyStr) {
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
	case screenPalette:
		if m.paletteScreen == nil {
			m.currentScreen = screenNone
			return m, nil
		}
		keyStr := msg.String()
		if isEscKey(keyStr) {
			m.currentScreen = screenNone
			m.paletteScreen = nil
			return m, nil
		}
		if keyStr == keyEnter {
			if m.paletteSubmit != nil {
				if action, ok := m.paletteScreen.Selected(); ok {
					cmd := m.paletteSubmit(action)
					m.currentScreen = screenNone
					m.paletteScreen = nil
					m.paletteSubmit = nil
					return m, cmd
				}
			}
		}
		ps, cmd := m.paletteScreen.Update(msg)
		if updated, ok := ps.(*CommandPaletteScreen); ok {
			m.paletteScreen = updated
		}
		return m, cmd
	case screenPRSelect:
		if m.prSelectionScreen == nil {
			m.currentScreen = screenNone
			return m, nil
		}
		keyStr := msg.String()
		if isEscKey(keyStr) {
			m.currentScreen = screenNone
			m.prSelectionScreen = nil
			m.prSelectionSubmit = nil
			return m, nil
		}
		if keyStr == keyEnter {
			if m.prSelectionSubmit != nil {
				if pr, ok := m.prSelectionScreen.Selected(); ok {
					cmd := m.prSelectionSubmit(pr)
					// Don't set screenNone here - prSelectionSubmit sets screenInput
					m.prSelectionScreen = nil
					m.prSelectionSubmit = nil
					return m, cmd
				}
			}
		}
		ps, cmd := m.prSelectionScreen.Update(msg)
		if updated, ok := ps.(*PRSelectionScreen); ok {
			m.prSelectionScreen = updated
		}
		return m, cmd
	case screenListSelect:
		if m.listScreen == nil {
			m.currentScreen = screenNone
			return m, nil
		}
		keyStr := msg.String()
		if isEscKey(keyStr) {
			m.listScreen = nil
			m.listSubmit = nil
			m.currentScreen = screenNone
			return m, nil
		}
		if keyStr == keyEnter {
			if m.listSubmit != nil {
				if item, ok := m.listScreen.Selected(); ok {
					cmd := m.listSubmit(item)
					return m, cmd
				}
			}
		}
		ls, cmd := m.listScreen.Update(msg)
		if updated, ok := ls.(*ListSelectionScreen); ok {
			m.listScreen = updated
		}
		return m, cmd
	case screenConfirm:
		if m.confirmScreen != nil {
			_, cmd := m.confirmScreen.Update(msg)
			// Check if the confirm screen sent a result
			select {
			case confirmed := <-m.confirmScreen.result:
				if confirmed {
					// Perform confirmed action (delete, prune, etc.)
					var actionCmd tea.Cmd
					if m.confirmAction != nil {
						actionCmd = m.confirmAction()
					}
					m.confirmScreen = nil
					m.confirmAction = nil
					if m.currentScreen == screenConfirm {
						m.currentScreen = screenNone
					}
					if actionCmd != nil {
						return m, actionCmd
					}
					return m, nil
				} else {
					// Action cancelled
					m.confirmScreen = nil
					m.confirmAction = nil
					m.currentScreen = screenNone
					return m, nil
				}
			default:
				return m, cmd
			}
		}
	case screenInfo:
		if m.infoScreen != nil {
			_, cmd := m.infoScreen.Update(msg)
			select {
			case <-m.infoScreen.result:
				action := m.infoAction
				m.infoScreen = nil
				m.infoAction = nil
				m.currentScreen = screenNone
				if action != nil {
					return m, action
				}
				return m, nil
			default:
				return m, cmd
			}
		}
	case screenWelcome:
		keyStr := msg.String()
		switch {
		case keyStr == "r" || keyStr == "R":
			m.currentScreen = screenNone
			m.welcomeScreen = nil
			return m, m.refreshWorktrees()
		case keyStr == keyQ || keyStr == "Q" || isEscKey(keyStr):
			m.quitting = true
			return m, tea.Quit
		}
	case screenTrust:
		if m.trustScreen == nil {
			m.currentScreen = screenNone
			return m, nil
		}
		keyStr := msg.String()
		switch {
		case keyStr == "t" || keyStr == "T":
			if m.pendingTrust != "" {
				_ = m.trustManager.TrustFile(m.pendingTrust)
			}
			cmd := m.runCommands(m.pendingCommands, m.pendingCmdCwd, m.pendingCmdEnv, m.pendingAfter)
			m.clearPendingTrust()
			m.currentScreen = screenNone
			return m, cmd
		case keyStr == "b" || keyStr == "B":
			after := m.pendingAfter
			m.clearPendingTrust()
			m.currentScreen = screenNone
			if after != nil {
				return m, after
			}
			return m, nil
		case keyStr == "c" || keyStr == "C" || isEscKey(keyStr):
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
		keyStr := msg.String()
		if keyStr == keyQ || isEscKey(keyStr) {
			m.commitScreen = nil
			m.currentScreen = screenNone
			return m, nil
		}
		cs, cmd := m.commitScreen.Update(msg)
		if updated, ok := cs.(*CommitScreen); ok {
			m.commitScreen = updated
		}
		return m, cmd
	case screenDiff:
		if m.diffScreen == nil {
			m.currentScreen = screenNone
			return m, nil
		}
		keyStr := msg.String()
		if keyStr == keyQ || isEscKey(keyStr) {
			m.diffScreen = nil
			m.currentScreen = screenNone
			return m, nil
		}
		ds, cmd := m.diffScreen.Update(msg)
		if updated, ok := ds.(*DiffScreen); ok {
			m.diffScreen = updated
		}
		return m, cmd
	case screenInput:
		if m.inputScreen == nil {
			m.currentScreen = screenNone
			return m, nil
		}

		keyStr := msg.String()
		if isEscKey(keyStr) {
			m.inputScreen = nil
			m.inputSubmit = nil
			m.currentScreen = screenNone
			return m, nil
		}
		if keyStr == keyEnter {
			if m.inputScreen.validate != nil {
				if errMsg := strings.TrimSpace(m.inputScreen.validate(m.inputScreen.input.Value())); errMsg != "" {
					m.inputScreen.errorMsg = errMsg
					return m, nil
				}
				m.inputScreen.errorMsg = ""
			}
			if m.inputSubmit != nil {
				cmd, closeCmd := m.inputSubmit(m.inputScreen.input.Value())
				if closeCmd {
					m.inputScreen = nil
					m.inputSubmit = nil
					if m.currentScreen == screenInput {
						m.currentScreen = screenNone
					}
				}
				return m, cmd
			}
		}

		var cmd tea.Cmd
		m.inputScreen.input, cmd = m.inputScreen.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *Model) renderScreen() string {
	switch m.currentScreen {
	case screenCommit:
		if m.commitScreen == nil {
			m.commitScreen = NewCommitScreen(commitMeta{}, "", "", m.git.UseDelta(), m.theme)
		}
		return m.commitScreen.View()
	case screenConfirm:
		if m.confirmScreen != nil {
			return m.confirmScreen.View()
		}
	case screenInfo:
		if m.infoScreen != nil {
			return m.infoScreen.View()
		}
	case screenTrust:
		if m.trustScreen == nil {
			return ""
		}
		return m.trustScreen.View()
	case screenWelcome:
		if m.welcomeScreen == nil {
			cwd, _ := os.Getwd()
			m.welcomeScreen = NewWelcomeScreen(cwd, m.getWorktreeDir(), m.theme)
		}
		return m.welcomeScreen.View()
	case screenPalette:
		if m.paletteScreen != nil {
			content := m.paletteScreen.View()
			if m.windowWidth > 0 && m.windowHeight > 0 {
				content = lipgloss.NewStyle().MarginTop(3).Render(content)
				return lipgloss.Place(
					m.windowWidth,
					m.windowHeight,
					lipgloss.Center,
					lipgloss.Top,
					content,
				)
			}
			return content
		}
	case screenDiff:
		if m.diffScreen != nil {
			return m.diffScreen.View()
		}
	case screenInput:
		if m.inputScreen != nil {
			content := m.inputScreen.View()
			if m.windowWidth > 0 && m.windowHeight > 0 {
				return lipgloss.Place(m.windowWidth, m.windowHeight, lipgloss.Center, lipgloss.Center, content)
			}
			return content
		}
	case screenListSelect:
		if m.listScreen != nil {
			return m.listScreen.View()
		}
	}
	return ""
}

func (m *Model) loadCache() tea.Cmd {
	return func() tea.Msg {
		repoKey := m.getRepoKey()
		cachePath := filepath.Join(m.getWorktreeDir(), repoKey, models.CacheFilename)
		// #nosec G304 -- cachePath is constructed from vetted worktree directory and constant filename
		data, err := os.ReadFile(cachePath)
		if err != nil {
			return nil
		}

		var payload struct {
			Worktrees []*models.WorktreeInfo `json:"worktrees"`
		}
		if err := json.Unmarshal(data, &payload); err != nil {
			return errMsg{err: err}
		}
		if len(payload.Worktrees) == 0 {
			return nil
		}
		return cachedWorktreesMsg{worktrees: payload.Worktrees}
	}
}

func (m *Model) saveCache() {
	repoKey := m.getRepoKey()
	cachePath := filepath.Join(m.getWorktreeDir(), repoKey, models.CacheFilename)
	if err := os.MkdirAll(filepath.Dir(cachePath), defaultDirPerms); err != nil {
		m.statusContent = fmt.Sprintf("Failed to create cache dir: %v", err)
		return
	}

	cacheData := struct {
		Worktrees []*models.WorktreeInfo `json:"worktrees"`
	}{
		Worktrees: m.worktrees,
	}
	data, _ := json.Marshal(cacheData)
	if err := os.WriteFile(cachePath, data, defaultFilePerms); err != nil {
		m.statusContent = fmt.Sprintf("Failed to write cache: %v", err)
	}
}

func (m *Model) getRepoKey() string {
	if m.repoKey != "" {
		return m.repoKey
	}
	m.repoKeyOnce.Do(func() {
		m.repoKey = m.git.ResolveRepoName(m.ctx)
	})
	return m.repoKey
}

func (m *Model) getCachedDetails(wt *models.WorktreeInfo) (string, string) {
	if wt == nil || strings.TrimSpace(wt.Path) == "" {
		return "", ""
	}

	cacheKey := wt.Path
	if cached, ok := m.detailsCache[cacheKey]; ok {
		if time.Since(cached.fetchedAt) < detailsCacheTTL {
			return cached.statusRaw, cached.logRaw
		}
	}

	// Get status (using porcelain format for reliable machine parsing)
	statusRaw := m.git.RunGit(m.ctx, []string{"git", "status", "--porcelain=v2"}, wt.Path, []int{0}, true, false)
	logRaw := m.git.RunGit(m.ctx, []string{"git", "log", "-50", "--pretty=format:%h%x09%s"}, wt.Path, []int{0}, true, false)

	m.detailsCache[cacheKey] = &detailsCacheEntry{
		statusRaw: statusRaw,
		logRaw:    logRaw,
		fetchedAt: time.Now(),
	}

	return statusRaw, logRaw
}

func (m *Model) getMainWorktreePath() string {
	for _, wt := range m.worktrees {
		if wt.IsMain {
			return wt.Path
		}
	}
	if len(m.worktrees) > 0 {
		return m.worktrees[0].Path
	}
	return ""
}

func (m *Model) getWorktreeDir() string {
	if m.config.WorktreeDir != "" {
		return m.config.WorktreeDir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "worktrees")
}

// GetSelectedPath returns the selected worktree path for shell integration.
// This is used when the application exits to allow the shell to cd into the selected worktree.
func (m *Model) GetSelectedPath() string {
	return m.selectedPath
}

func (m *Model) debugf(format string, args ...any) {
	if m.debugLogger == nil {
		return
	}
	m.debugLogger.Printf(format, args...)
}

// isEscKey checks if the key string represents an escape key.
// Some terminals send ESC as "esc" (tea.KeyEsc) while others send it
// as a raw escape byte "\x1b" (ASCII 27).
func isEscKey(keyStr string) bool {
	return keyStr == keyEsc || keyStr == keyEscRaw
}

func (m *Model) persistCurrentSelection() {
	idx := m.selectedIndex
	if idx < 0 || idx >= len(m.filteredWts) {
		idx = m.worktreeTable.Cursor()
	}
	if idx < 0 || idx >= len(m.filteredWts) {
		return
	}
	m.persistLastSelected(m.filteredWts[idx].Path)
}

func (m *Model) persistLastSelected(path string) {
	if strings.TrimSpace(path) == "" {
		return
	}
	m.debugf("persist last-selected: %s", path)
	repoKey := m.getRepoKey()
	lastSelectedPath := filepath.Join(m.getWorktreeDir(), repoKey, models.LastSelectedFilename)
	if err := os.MkdirAll(filepath.Dir(lastSelectedPath), defaultDirPerms); err != nil {
		return
	}
	_ = os.WriteFile(lastSelectedPath, []byte(path+"\n"), defaultFilePerms)
}

// Close releases background resources including canceling contexts and timers.
// It also persists the current selection for the next session.
func (m *Model) Close() {
	m.persistCurrentSelection()
	m.debugf("close")
	if m.debugLogFile != nil {
		_ = m.debugLogFile.Close()
	}
	if m.detailUpdateCancel != nil {
		m.detailUpdateCancel()
	}
	if m.cancel != nil {
		m.cancel()
	}
}

func (m *Model) buildCommandEnv(branch, wtPath string) map[string]string {
	return map[string]string{
		"WORKTREE_BRANCH":    branch,
		"MAIN_WORKTREE_PATH": m.git.GetMainWorktreePath(m.ctx),
		"WORKTREE_PATH":      wtPath,
		"WORKTREE_NAME":      filepath.Base(wtPath),
		"REPO_NAME":          m.repoKey,
	}
}

type resolvedTmuxWindow struct {
	Name    string
	Command string
	Cwd     string
}

func expandWithEnv(input string, env map[string]string) string {
	if input == "" {
		return ""
	}
	return os.Expand(input, func(key string) string {
		if val, ok := env[key]; ok {
			return val
		}
		return os.Getenv(key)
	})
}

func envMapToList(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	out := make([]string, 0, len(env))
	for key, val := range env {
		out = append(out, fmt.Sprintf("%s=%s", key, val))
	}
	return out
}

func resolveTmuxWindows(windows []config.TmuxWindow, env map[string]string, defaultCwd string) ([]resolvedTmuxWindow, bool) {
	if len(windows) == 0 {
		return nil, false
	}
	resolved := make([]resolvedTmuxWindow, 0, len(windows))
	for i, window := range windows {
		name := strings.TrimSpace(expandWithEnv(window.Name, env))
		if name == "" {
			name = fmt.Sprintf("window-%d", i+1)
		}
		cwd := strings.TrimSpace(expandWithEnv(window.Cwd, env))
		if cwd == "" {
			cwd = defaultCwd
		}
		command := strings.TrimSpace(window.Command)
		command = buildTmuxWindowCommand(command, env)
		resolved = append(resolved, resolvedTmuxWindow{
			Name:    name,
			Command: command,
			Cwd:     cwd,
		})
	}
	return resolved, true
}

func buildTmuxWindowCommand(command string, env map[string]string) string {
	prefix := exportEnvCommand(env)
	if prefix != "" {
		prefix += " "
	}
	if strings.TrimSpace(command) == "" {
		return prefix + "exec ${SHELL:-bash}"
	}
	return prefix + command
}

func exportEnvCommand(env map[string]string) string {
	if len(env) == 0 {
		return ""
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("export %s=%s;", key, shellQuote(env[key])))
	}
	return strings.Join(parts, " ")
}

func buildTmuxScript(sessionName string, tmuxCfg *config.TmuxCommand, windows []resolvedTmuxWindow, env map[string]string) string {
	onExists := strings.ToLower(strings.TrimSpace(tmuxCfg.OnExists))
	switch onExists {
	case "attach", "kill", "new", "switch":
	default:
		onExists = "switch"
	}

	var b strings.Builder
	b.WriteString("set -e\n")
	b.WriteString(fmt.Sprintf("session=%s\n", shellQuote(sessionName)))
	b.WriteString("base_session=$session\n")
	b.WriteString("if tmux has-session -t \"$session\" 2>/dev/null; then\n")
	switch onExists {
	case "kill":
		b.WriteString("  tmux kill-session -t \"$session\"\n")
	case "new":
		b.WriteString("  i=2\n")
		b.WriteString("  while tmux has-session -t \"${base_session}-$i\" 2>/dev/null; do i=$((i+1)); done\n")
		b.WriteString("  session=\"${base_session}-$i\"\n")
	default:
		b.WriteString("  :\n")
	}
	b.WriteString("fi\n")
	b.WriteString("if ! tmux has-session -t \"$session\" 2>/dev/null; then\n")
	if len(windows) == 0 {
		return ""
	}
	first := windows[0]
	b.WriteString(fmt.Sprintf("  tmux new-session -d -s \"$session\" -n %s -c %s -- bash -lc %s\n",
		shellQuote(first.Name), shellQuote(first.Cwd), shellQuote(first.Command)))

	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		b.WriteString(fmt.Sprintf("  tmux set-environment -t \"$session\" %s %s\n", shellQuote(key), shellQuote(env[key])))
	}

	for _, window := range windows[1:] {
		b.WriteString(fmt.Sprintf("  tmux new-window -t \"$session\" -n %s -c %s -- bash -lc %s\n",
			shellQuote(window.Name), shellQuote(window.Cwd), shellQuote(window.Command)))
	}
	b.WriteString("fi\n")
	b.WriteString("if [ -n \"${LW_TMUX_SESSION_FILE:-}\" ]; then printf '%s' \"$session\" > \"$LW_TMUX_SESSION_FILE\"; fi\n")

	if tmuxCfg.Attach {
		if onExists == "attach" {
			b.WriteString("tmux attach -t \"$session\" || true\n")
		} else {
			b.WriteString("if [ -n \"$TMUX\" ]; then tmux switch-client -t \"$session\" || true; else tmux attach -t \"$session\" || true; fi\n")
		}
	}
	return b.String()
}

func buildTmuxInfoMessage(sessionName string, insideTmux bool) string {
	quoted := shellQuote(sessionName)
	if insideTmux {
		return fmt.Sprintf("tmux session ready.\n\nSwitch with:\n\n  tmux switch-client -t %s", quoted)
	}
	return fmt.Sprintf("tmux session ready.\n\nAttach with:\n\n  tmux attach-session -t %s", quoted)
}

func (m *Model) attachTmuxSessionCmd(sessionName string, insideTmux bool) tea.Cmd {
	args := []string{"attach-session", "-t", sessionName}
	if insideTmux {
		args = []string{"switch-client", "-t", sessionName}
	}
	// #nosec G204 -- tmux session name comes from user configuration.
	c := m.commandRunner("tmux", args...)
	return m.execProcess(c, func(err error) tea.Msg {
		if err != nil {
			return errMsg{err: err}
		}
		return refreshCompleteMsg{}
	})
}

func readTmuxSessionFile(path, fallback string) string {
	// #nosec G304 -- file path is created by the current process.
	data, err := os.ReadFile(path)
	if err != nil {
		return fallback
	}
	value := strings.TrimSpace(string(data))
	if value == "" {
		return fallback
	}
	return value
}

func shellQuote(input string) string {
	if input == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(input, "'", "'\"'\"'") + "'"
}

func (m *Model) collectInitCommands() []string {
	cmds := []string{}
	cmds = append(cmds, m.config.InitCommands...)
	if m.repoConfig != nil {
		cmds = append(cmds, m.repoConfig.InitCommands...)
	}
	return cmds
}

func (m *Model) collectTerminateCommands() []string {
	cmds := []string{}
	cmds = append(cmds, m.config.TerminateCommands...)
	if m.repoConfig != nil {
		cmds = append(cmds, m.repoConfig.TerminateCommands...)
	}
	return cmds
}

func (m *Model) runCommandsWithTrust(cmds []string, cwd string, env map[string]string, after func() tea.Msg) tea.Cmd {
	if len(cmds) == 0 {
		if after == nil {
			return nil
		}
		return after
	}

	trustMode := strings.ToLower(strings.TrimSpace(m.config.TrustMode))
	// If trust mode set to never, skip repo commands
	if trustMode == "never" {
		if after == nil {
			return nil
		}
		return after
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
		m.pendingAfter = after
		m.pendingTrust = trustPath
		m.trustScreen = NewTrustScreen(trustPath, cmds, m.theme)
		m.currentScreen = screenTrust
	}
	return nil
}

func (m *Model) runCommands(cmds []string, cwd string, env map[string]string, after func() tea.Msg) tea.Cmd {
	return func() tea.Msg {
		if err := m.git.ExecuteCommands(m.ctx, cmds, cwd, env); err != nil {
			// Still refresh UI even if commands failed, so user sees current state
			if after != nil {
				return after()
			}
			return errMsg{err: err}
		}
		if after != nil {
			return after()
		}
		return nil
	}
}

func (m *Model) clearPendingTrust() {
	m.pendingCommands = nil
	m.pendingCmdEnv = nil
	m.pendingCmdCwd = ""
	m.pendingAfter = nil
	m.pendingTrust = ""
	m.trustScreen = nil
}

func (m *Model) ensureRepoConfig() {
	if m.repoConfig != nil || m.repoConfigPath != "" {
		return
	}
	mainPath := m.getMainWorktreePath()
	if mainPath == "" {
		mainPath = m.git.GetMainWorktreePath(m.ctx)
	}
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

func (m *Model) setWindowSize(width, height int) {
	m.windowWidth = width
	m.windowHeight = height
	m.applyLayout(m.computeLayout())
}

func (m *Model) computeLayout() layoutDims {
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

	paneFrameX := m.basePaneStyle().GetHorizontalFrameSize()
	paneFrameY := m.basePaneStyle().GetVerticalFrameSize()

	leftInnerWidth := maxInt(1, leftWidth-paneFrameX)
	rightInnerWidth := maxInt(1, rightWidth-paneFrameX)
	leftInnerHeight := maxInt(1, bodyHeight-paneFrameY)
	rightTopInnerHeight := maxInt(1, rightTopHeight-paneFrameY)
	rightBottomInnerHeight := maxInt(1, rightBottomHeight-paneFrameY)

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

func (m *Model) applyLayout(layout layoutDims) {
	titleHeight := 1
	tableHeaderHeight := 1 // bubbles table has its own header

	// Subtract 2 extra lines for safety margin
	tableHeight := maxInt(1, layout.leftInnerHeight-titleHeight-tableHeaderHeight-2)
	m.worktreeTable.SetWidth(layout.leftInnerWidth)
	m.worktreeTable.SetHeight(tableHeight)
	m.updateTableColumns(layout.leftInnerWidth)

	logHeight := maxInt(1, layout.rightBottomInnerHeight-titleHeight-tableHeaderHeight-2)
	m.logTable.SetWidth(layout.rightInnerWidth)
	m.logTable.SetHeight(logHeight)
	m.updateLogColumns(layout.rightInnerWidth)

	m.filterInput.Width = maxInt(20, layout.width-18)
}

func (m *Model) renderHeader(layout layoutDims) string {
	// Create a "toolbar" style header
	headerStyle := lipgloss.NewStyle().
		Background(m.theme.Accent).
		Foreground(m.theme.AccentDim).
		Bold(true).
		Width(layout.width).
		Padding(0, 1)

	title := "Git Worktree Status"
	repoStyle := lipgloss.NewStyle().Italic(true).Foreground(m.theme.AccentDim)
	repoKey := strings.TrimSpace(m.repoKey)
	content := title
	if repoKey != "" && repoKey != "unknown" && !strings.HasPrefix(repoKey, "local-") {
		content = fmt.Sprintf("%s - %s", content, repoStyle.Render(repoKey))
	}

	return headerStyle.Render(content)
}

func (m *Model) renderFilter(layout layoutDims) string {
	labelStyle := lipgloss.NewStyle().Foreground(m.theme.Accent).Bold(true)
	filterStyle := lipgloss.NewStyle().
		Foreground(m.theme.TextFg).
		Padding(0, 1)
	line := fmt.Sprintf("%s %s", labelStyle.Render("/ Filter:"), m.filterInput.View())
	return filterStyle.Width(layout.width).Render(line)
}

func (m *Model) renderBody(layout layoutDims) string {
	left := m.renderLeftPane(layout)
	right := m.renderRightPane(layout)
	gap := lipgloss.NewStyle().
		Width(layout.gapX).
		Render(strings.Repeat(" ", layout.gapX))
	return lipgloss.JoinHorizontal(lipgloss.Top, left, gap, right)
}

func (m *Model) renderLeftPane(layout layoutDims) string {
	title := m.renderPaneTitle(1, "Worktrees", m.focusedPane == 0, layout.leftInnerWidth)
	tableView := m.worktreeTable.View()
	content := lipgloss.JoinVertical(lipgloss.Left, title, tableView)
	return m.paneStyle(m.focusedPane == 0).
		Width(layout.leftWidth).
		Height(layout.bodyHeight).
		Render(content)
}

func (m *Model) renderRightPane(layout layoutDims) string {
	top := m.renderRightTopPane(layout)
	bottom := m.renderRightBottomPane(layout)
	gap := strings.Repeat("\n", layout.gapY)
	return lipgloss.JoinVertical(lipgloss.Left, top, gap, bottom)
}

func (m *Model) renderRightTopPane(layout layoutDims) string {
	title := m.renderPaneTitle(2, "Info/Diff", m.focusedPane == 1, layout.rightInnerWidth)
	infoBox := m.renderInnerBox("Info", m.infoContent, layout.rightInnerWidth, 0)

	innerBoxStyle := m.baseInnerBoxStyle()
	statusBoxHeight := layout.rightTopInnerHeight - lipgloss.Height(title) - lipgloss.Height(infoBox) - 2
	if statusBoxHeight < 3 {
		statusBoxHeight = 3
	}
	statusViewportWidth := maxInt(1, layout.rightInnerWidth-innerBoxStyle.GetHorizontalFrameSize())
	statusViewportHeight := maxInt(1, statusBoxHeight-innerBoxStyle.GetVerticalFrameSize())
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
	return m.paneStyle(m.focusedPane == 1).
		Width(layout.rightWidth).
		Height(layout.rightTopHeight).
		Render(content)
}

func (m *Model) renderRightBottomPane(layout layoutDims) string {
	title := m.renderPaneTitle(3, "Log", m.focusedPane == 2, layout.rightInnerWidth)
	content := lipgloss.JoinVertical(lipgloss.Left, title, m.logTable.View())
	return m.paneStyle(m.focusedPane == 2).
		Width(layout.rightWidth).
		Height(layout.rightBottomHeight).
		Render(content)
}

func (m *Model) renderFooter(layout layoutDims) string {
	footerStyle := lipgloss.NewStyle().
		Foreground(m.theme.MutedFg).
		Background(m.theme.Background).
		Padding(0, 1)
	hints := []string{
		m.renderKeyHint("1-3", "Pane Focus"),
		m.renderKeyHint("g", "LazyGit"),
		m.renderKeyHint("r", "Refresh"),
		m.renderKeyHint("d", "Diff"),
		m.renderKeyHint("p", "PR Info"),
	}
	// Show "o" key hint only when current worktree has PR info
	if m.selectedIndex >= 0 && m.selectedIndex < len(m.filteredWts) {
		wt := m.filteredWts[m.selectedIndex]
		if wt.PR != nil {
			hints = append(hints, m.renderKeyHint("o", "Open PR"))
		}
	}
	hints = append(hints, m.customFooterHints()...)
	hints = append(hints,
		m.renderKeyHint("D", "Delete"),
		m.renderKeyHint("/", "Filter"),
		m.renderKeyHint("q", "Quit"),
		m.renderKeyHint("?", "Help"),
		m.renderKeyHint("ctrl+p", "Palette"),
	)
	footerContent := strings.Join(hints, "  ")
	if !m.loading {
		return footerStyle.Width(layout.width).Render(footerContent)
	}
	spinnerView := m.spinner.View()
	gap := "  "
	available := layout.width - lipgloss.Width(spinnerView) - lipgloss.Width(gap)
	if available < 0 {
		available = 0
	}
	footer := footerStyle.Width(available).Render(footerContent)
	return lipgloss.JoinHorizontal(lipgloss.Left, footer, gap, spinnerView)
}

func (m *Model) customFooterHints() []string {
	keys := m.customCommandKeys()
	if len(keys) == 0 {
		return nil
	}

	hints := make([]string, 0, len(keys))
	for _, key := range keys {
		cmd := m.config.CustomCommands[key]
		if cmd == nil || !cmd.ShowHelp {
			continue
		}
		label := strings.TrimSpace(cmd.Description)
		if label == "" {
			label = strings.TrimSpace(cmd.Command)
		}
		if label == "" {
			label = customCommandPlaceholder
		}
		hints = append(hints, m.renderKeyHint(key, label))
	}
	return hints
}

func (m *Model) renderKeyHint(key, label string) string {
	keyStyle := lipgloss.NewStyle().Foreground(m.theme.Accent).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(m.theme.MutedFg)
	return fmt.Sprintf("%s %s", keyStyle.Render(key), labelStyle.Render(label))
}

func (m *Model) renderPaneTitle(index int, title string, focused bool, width int) string {
	numStyle := lipgloss.NewStyle().Foreground(m.theme.MutedFg)
	titleStyle := lipgloss.NewStyle().Foreground(m.theme.MutedFg)
	if focused {
		numStyle = numStyle.Foreground(m.theme.Accent).Bold(true)
		titleStyle = titleStyle.Foreground(m.theme.TextFg)
	}
	num := numStyle.Render(fmt.Sprintf("[%d]", index))
	name := titleStyle.Render(title)
	return lipgloss.NewStyle().Width(width).Render(fmt.Sprintf("%s %s", num, name))
}

func (m *Model) renderInnerBox(title, content string, width, height int) string {
	if content == "" {
		content = "No data available."
	}

	titleStyle := lipgloss.NewStyle().Foreground(m.theme.MutedFg).Bold(true)

	style := m.baseInnerBoxStyle().Width(width)
	if height > 0 {
		style = style.Height(height)
	}

	innerWidth := maxInt(1, width-style.GetHorizontalFrameSize())
	wrappedContent := wrap.String(content, innerWidth)
	boxContent := lipgloss.JoinVertical(lipgloss.Left, titleStyle.Render(title), wrappedContent)

	return style.Render(boxContent)
}

func (m *Model) buildInfoContent(wt *models.WorktreeInfo) string {
	if wt == nil {
		return errNoWorktreeSelected
	}

	labelStyle := lipgloss.NewStyle().Foreground(m.theme.Cyan).Bold(true)
	valueStyle := lipgloss.NewStyle().Foreground(m.theme.TextFg)

	infoLines := []string{
		fmt.Sprintf("%s %s", labelStyle.Render("Path:"), valueStyle.Render(wt.Path)),
		fmt.Sprintf("%s %s", labelStyle.Render("Branch:"), valueStyle.Render(wt.Branch)),
	}
	if wt.Divergence != "" {
		// Colorize arrows to match Python: cyan ↑, red ↓
		coloredDiv := strings.ReplaceAll(wt.Divergence, "↑", lipgloss.NewStyle().Foreground(m.theme.Cyan).Render("↑"))
		coloredDiv = strings.ReplaceAll(coloredDiv, "↓", lipgloss.NewStyle().Foreground(m.theme.ErrorFg).Render("↓"))
		infoLines = append(infoLines, fmt.Sprintf("%s %s", labelStyle.Render("Divergence:"), coloredDiv))
	}
	if wt.PR != nil {
		// Match Python: white number, colored state (green=OPEN, magenta=MERGED, red=else)
		prLabelStyle := lipgloss.NewStyle().Foreground(m.theme.Pink).Bold(true) // Pink for PR prominence
		whiteStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("15"))      // white
		stateColor := lipgloss.Color("2")                                       // green for OPEN
		switch wt.PR.State {
		case "MERGED":
			stateColor = lipgloss.Color("5") // magenta
		case "CLOSED":
			stateColor = lipgloss.Color("1") // red
		}
		stateStyle := lipgloss.NewStyle().Foreground(stateColor)
		// Format: PR: #123 Title [STATE] (matches Python grid layout)
		infoLines = append(infoLines, fmt.Sprintf("%s %s %s [%s]",
			prLabelStyle.Render("PR:"),
			whiteStyle.Render(fmt.Sprintf("#%d", wt.PR.Number)),
			wt.PR.Title,
			stateStyle.Render(wt.PR.State)))
		// URL styled with cyan for consistency
		urlStyle := lipgloss.NewStyle().Foreground(m.theme.Cyan).Underline(true)
		infoLines = append(infoLines, fmt.Sprintf("     %s", urlStyle.Render(wt.PR.URL)))

		// CI status from cache
		if cached, ok := m.ciCache[wt.Branch]; ok && len(cached.checks) > 0 {
			infoLines = append(infoLines, "") // blank line before CI
			infoLines = append(infoLines, labelStyle.Render("CI Checks:"))

			greenStyle := lipgloss.NewStyle().Foreground(m.theme.SuccessFg)
			redStyle := lipgloss.NewStyle().Foreground(m.theme.ErrorFg)
			yellowStyle := lipgloss.NewStyle().Foreground(m.theme.WarnFg)
			grayStyle := lipgloss.NewStyle().Foreground(m.theme.MutedFg)

			for _, check := range cached.checks {
				var symbol, styledSymbol string
				switch check.Conclusion {
				case "success":
					symbol = "✓"
					styledSymbol = greenStyle.Render(symbol)
				case "failure":
					symbol = "✗"
					styledSymbol = redStyle.Render(symbol)
				case "skipped":
					symbol = "○"
					styledSymbol = grayStyle.Render(symbol)
				case "cancelled":
					symbol = "⊘"
					styledSymbol = grayStyle.Render(symbol)
				case "pending", "":
					symbol = "●"
					styledSymbol = yellowStyle.Render(symbol)
				default:
					symbol = "?"
					styledSymbol = grayStyle.Render(symbol)
				}
				infoLines = append(infoLines, fmt.Sprintf("  %s %s", styledSymbol, check.Name))
			}
		}
	}
	return strings.Join(infoLines, "\n")
}

func (m *Model) buildStatusContent(statusRaw string) string {
	statusRaw = strings.TrimRight(statusRaw, "\n")
	if strings.TrimSpace(statusRaw) == "" {
		return lipgloss.NewStyle().Foreground(m.theme.SuccessFg).Render("Clean working tree")
	}

	modifiedStyle := lipgloss.NewStyle().Foreground(m.theme.WarnFg)
	addedStyle := lipgloss.NewStyle().Foreground(m.theme.SuccessFg)
	deletedStyle := lipgloss.NewStyle().Foreground(m.theme.ErrorFg)
	untrackedStyle := lipgloss.NewStyle().Foreground(m.theme.Yellow)

	lines := []string{}
	for _, line := range strings.Split(statusRaw, "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse git status --porcelain=v2 format
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		var status, filename string

		switch fields[0] {
		case "1": // Ordinary changed entry: 1 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <path>
			if len(fields) < 9 {
				continue
			}
			status = fields[1] // XY status code (e.g., ".M", "M.", "MM")
			filename = fields[8]
		case "?": // Untracked: ? <path>
			status = " ?" // Single ? with space for alignment
			filename = fields[1]
		case "2": // Renamed/copied: 2 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <X><score> <path><sep><origPath>
			if len(fields) < 10 {
				continue
			}
			status = fields[1]
			filename = fields[9]
		default:
			continue // Skip unhandled entry types
		}

		// Determine style based on status code
		var style lipgloss.Style
		x := status[0] // index status
		y := status[1] // working tree status
		switch {
		case status == " ?":
			style = untrackedStyle
		case x == 'D' || y == 'D':
			style = deletedStyle
		case x == 'A' || y == 'A':
			style = addedStyle
		case x == 'M' || y == 'M' || x == '.' || y == '.':
			style = modifiedStyle
		case x == 'R' || y == 'R':
			style = modifiedStyle
		default:
			style = lipgloss.NewStyle()
		}

		// Convert porcelain dots to spaces for display (. means "unchanged")
		displayStatus := strings.ReplaceAll(status, ".", " ")

		// Format: "XY filename" with proper alignment
		// Render each status character separately to avoid ANSI codes wrapping spaces
		var statusRendered strings.Builder
		for _, char := range displayStatus {
			if char == ' ' {
				statusRendered.WriteString(" ")
			} else {
				statusRendered.WriteString(style.Render(string(char)))
			}
		}

		formatted := fmt.Sprintf("%s %s", statusRendered.String(), filename)
		lines = append(lines, formatted)
	}
	return strings.Join(lines, "\n")
}

func (m *Model) updateTableColumns(totalWidth int) {
	status := 6
	ab := 7
	last := 15

	// Only include PR column width if PR data has been loaded
	pr := 0
	if m.prDataLoaded {
		pr = 12
	}

	// The table library handles separators internally (3 spaces per separator)
	// So we need to account for them: (numColumns - 1) * 3
	numColumns := 4
	if m.prDataLoaded {
		numColumns = 5
	}
	separatorSpace := (numColumns - 1) * 3

	worktree := maxInt(12, totalWidth-status-ab-last-pr-separatorSpace)
	excess := worktree + status + ab + pr + last + separatorSpace - totalWidth
	for excess > 0 && last > 10 {
		last--
		excess--
	}
	if m.prDataLoaded {
		for excess > 0 && pr > 8 {
			pr--
			excess--
		}
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
		worktree = maxInt(6, worktree-excess)
	}

	// Final adjustment: ensure column widths + separators sum exactly to totalWidth
	actualTotal := worktree + status + ab + last + pr + separatorSpace
	if actualTotal < totalWidth {
		// Distribute remaining space to the worktree column
		worktree += (totalWidth - actualTotal)
	} else if actualTotal > totalWidth {
		// Remove excess from worktree column
		worktree = maxInt(6, worktree-(actualTotal-totalWidth))
	}

	columns := []table.Column{
		{Title: "Worktree", Width: worktree},
		{Title: "Status", Width: status},
		{Title: "±", Width: ab},
		{Title: "Last Active", Width: last},
	}

	if m.prDataLoaded {
		columns = append(columns, table.Column{Title: "PR", Width: pr})
	}

	m.worktreeTable.SetColumns(columns)
}

func (m *Model) updateLogColumns(totalWidth int) {
	sha := 8

	// The table library handles separators internally (3 spaces per separator)
	// 2 columns = 1 separator = 3 spaces
	separatorSpace := 3

	message := maxInt(10, totalWidth-sha-separatorSpace)

	// Final adjustment: ensure column widths + separator space sum exactly to totalWidth
	actualTotal := sha + message + separatorSpace
	if actualTotal < totalWidth {
		message += (totalWidth - actualTotal)
	} else if actualTotal > totalWidth {
		message = maxInt(10, message-(actualTotal-totalWidth))
	}

	m.logTable.SetColumns([]table.Column{
		{Title: "SHA", Width: sha},
		{Title: "Message", Width: message},
	})
}

func (m *Model) basePaneStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.ThickBorder()).
		BorderForeground(m.theme.BorderDim).
		Padding(0, 1)
}

func (m *Model) paneStyle(focused bool) lipgloss.Style {
	borderColor := m.theme.BorderDim
	if focused {
		borderColor = m.theme.Accent
	}
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(borderColor).
		Padding(0, 1)
}

func (m *Model) baseInnerBoxStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.ThickBorder()).
		BorderForeground(m.theme.BorderDim).
		Padding(0, 1)
}

// truncateToHeight ensures output doesn't exceed maxLines
func truncateToHeight(s string, maxLines int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return strings.Join(lines, "\n")
}
