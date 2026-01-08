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
	errPRBranchMissing       = "PR branch information is missing."
	customCommandPlaceholder = "Custom command"
	tmuxSessionLabel         = "tmux session"
	zellijSessionLabel       = "zellij session"
	onExistsAttach           = "attach"
	onExistsKill             = "kill"
	onExistsNew              = "new"
	onExistsSwitch           = "switch"

	detailsCacheTTL  = 2 * time.Second
	debounceDelay    = 200 * time.Millisecond
	ciCacheTTL       = 30 * time.Second
	defaultDirPerms  = 0o750
	defaultFilePerms = 0o600

	osDarwin  = "darwin"
	osWindows = "windows"

	// Visual symbols for enhanced UI
	symbolFilledCircle = "●"

	// Loading messages
	loadingRefreshWorktrees = "Refreshing worktrees..."

	commitMessageMaxLength     = 80
	filterWorktreesPlaceholder = "Filter worktrees..."
)

type (
	errMsg             struct{ err error }
	worktreesLoadedMsg struct {
		worktrees []*models.WorktreeInfo
		err       error
	}
	prDataLoadedMsg struct {
		prMap       map[string]*models.PRInfo
		worktreePRs map[string]*models.PRInfo // keyed by worktree path
		err         error
	}
	statusUpdatedMsg struct {
		info        string
		statusFiles []StatusFile
		log         []commitLogEntry
		path        string
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
	zellijSessionReadyMsg struct {
		sessionName  string
		attach       bool
		insideZellij bool
	}
	cachedWorktreesMsg struct {
		worktrees []*models.WorktreeInfo
	}
	detailsCacheEntry struct {
		statusRaw    string
		logRaw       string
		unpushedSHAs map[string]bool
		unmergedSHAs map[string]bool
		fetchedAt    time.Time
	}
	pruneResultMsg struct {
		worktrees []*models.WorktreeInfo
		err       error
		pruned    int
		failed    int
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
	createFromPRResultMsg struct {
		prNumber   int
		branch     string
		targetPath string
		err        error
	}
	openIssuesLoadedMsg struct {
		issues []*models.IssueInfo
		err    error
	}
	createFromIssueResultMsg struct {
		issueNumber int
		branch      string
		targetPath  string
		err         error
	}
	createFromChangesReadyMsg struct {
		worktree      *models.WorktreeInfo
		currentBranch string
		diff          string // git diff output for branch name generation
	}
	cherryPickResultMsg struct {
		commitSHA      string
		targetWorktree *models.WorktreeInfo
		err            error
	}
	commitFilesLoadedMsg struct {
		sha          string
		worktreePath string
		files        []models.CommitFile
		meta         commitMeta
		err          error
	}
)

type commitLogEntry struct {
	sha            string
	authorInitials string
	message        string
	isUnpushed     bool
	isUnmerged     bool
}

// StatusFile represents a file entry from git status.
type StatusFile struct {
	Filename    string
	Status      string // XY status code (e.g., ".M", "M.", " ?")
	IsUntracked bool
}

// StatusTreeNode represents a node in the status file tree (directory or file).
type StatusTreeNode struct {
	Path        string            // Full path (e.g., "internal/app" or "internal/app/app.go")
	File        *StatusFile       // nil for directories
	Children    []*StatusTreeNode // nil for files
	Compression int               // Number of compressed path segments (e.g., "a/b" = 1)
	depth       int               // Cached depth for rendering
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

	// Merge methods for absorb worktree
	mergeMethodRebase = "rebase"

	// Sort modes for worktree list
	sortModePath         = 0 // Sort by path (alphabetical)
	sortModeLastActive   = 1 // Sort by last commit date
	sortModeLastSwitched = 2 // Sort by last UI access time
)

type searchTarget int

const (
	searchTargetWorktrees searchTarget = iota
	searchTargetStatus
	searchTargetLog
)

type filterTarget int

const (
	filterTargetWorktrees filterTarget = iota
	filterTargetStatus
	filterTargetLog
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
	worktrees            []*models.WorktreeInfo
	filteredWts          []*models.WorktreeInfo
	selectedIndex        int
	filterQuery          string
	statusFilterQuery    string
	logFilterQuery       string
	worktreeSearchQuery  string
	statusSearchQuery    string
	logSearchQuery       string
	sortMode             int // sortModePath, sortModeLastActive, or sortModeLastSwitched
	prDataLoaded         bool
	accessHistory        map[string]int64 // worktree path -> last access timestamp
	repoKey              string
	repoKeyOnce          sync.Once
	currentScreen        screenType
	currentDetailsPath   string
	helpScreen           *HelpScreen
	trustScreen          *TrustScreen
	inputScreen          *InputScreen
	inputSubmit          func(string) (tea.Cmd, bool)
	commitScreen         *CommitScreen
	welcomeScreen        *WelcomeScreen
	paletteScreen        *CommandPaletteScreen
	paletteSubmit        func(string) tea.Cmd
	prSelectionScreen    *PRSelectionScreen
	issueSelectionScreen *IssueSelectionScreen
	issueSelectionSubmit func(*models.IssueInfo) tea.Cmd
	prSelectionSubmit    func(*models.PRInfo) tea.Cmd
	listScreen           *ListSelectionScreen
	listSubmit           func(selectionItem) tea.Cmd
	diffScreen           *DiffScreen
	spinner              spinner.Model
	loading              bool
	showingFilter        bool
	filterTarget         filterTarget
	showingSearch        bool
	searchTarget         searchTarget
	focusedPane          int // 0=table, 1=status, 2=log
	zoomedPane           int // -1 = no zoom, 0/1/2 = which pane is zoomed
	windowWidth          int
	windowHeight         int
	infoContent          string
	statusContent        string
	statusFiles          []StatusFile // parsed list of files from git status (kept for compatibility)
	statusFilesAll       []StatusFile // full list of files from git status
	statusFileIndex      int          // currently selected file index in status pane

	// Status tree view
	statusTree          *StatusTreeNode   // Root of the file tree
	statusTreeFlat      []*StatusTreeNode // Visible nodes after applying collapse state
	statusCollapsedDirs map[string]bool   // Collapsed directory paths
	statusTreeIndex     int               // Current selection in flattened tree

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

	// Post-refresh selection (e.g. after creating worktree)
	pendingSelectWorktreePath string

	// Confirm screen
	confirmScreen *ConfirmScreen
	confirmAction func() tea.Cmd
	infoScreen    *InfoScreen
	infoAction    tea.Cmd
	loadingScreen *LoadingScreen

	// Trust / repo commands
	repoConfig      *config.RepoConfig
	repoConfigPath  string
	pendingCommands []string
	pendingCmdEnv   map[string]string
	pendingCmdCwd   string
	pendingAfter    func() tea.Msg
	pendingTrust    string

	// Log cache for commit detail viewer
	logEntries    []commitLogEntry
	logEntriesAll []commitLogEntry

	// Commit files screen for browsing files in a commit
	commitFilesScreen *CommitFilesScreen

	// Command history for ! command
	commandHistory []string

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
		Foreground(thm.Cyan).
		Background(thm.AccentDim) // Add subtle background to header
	s.Selected = s.Selected.
		Foreground(thm.AccentFg). // Text color that contrasts with Accent background
		Background(thm.Accent).   // Use Accent instead of AccentDim for better visibility
		Bold(true)
	// Add subtle background to unselected cells for better readability
	s.Cell = s.Cell.
		Foreground(thm.TextFg)
	t.SetStyles(s)

	statusVp := viewport.New(40, 5)
	statusVp.SetContent("Loading...")

	logColumns := []table.Column{
		{Title: "SHA", Width: 8},
		{Title: "Au", Width: 2},
		{Title: "Message", Width: 50},
	}
	logT := table.New(
		table.WithColumns(logColumns),
		table.WithHeight(5),
	)
	logStyles := s
	logT.SetStyles(logStyles)

	filterInput := textinput.New()
	filterInput.Placeholder = filterWorktreesPlaceholder
	filterInput.Width = 50
	filterInput.PromptStyle = lipgloss.NewStyle().Foreground(thm.Accent)
	filterInput.TextStyle = lipgloss.NewStyle().Foreground(thm.TextFg)

	sp := spinner.New()
	sp.Spinner = spinner.Pulse
	sp.Style = lipgloss.NewStyle().Foreground(thm.Accent)

	// Convert config sort mode string to int constant
	sortMode := sortModeLastSwitched // default
	switch cfg.SortMode {
	case "path":
		sortMode = sortModePath
	case "active":
		sortMode = sortModeLastActive
	case "switched":
		sortMode = sortModeLastSwitched
	}

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
		sortMode:        sortMode,
		filterQuery:     initialFilter,
		filterTarget:    filterTargetWorktrees,
		searchTarget:    searchTargetWorktrees,
		cache:           make(map[string]any),
		divergenceCache: make(map[string]string),
		notifiedErrors:  make(map[string]bool),
		ciCache:         make(map[string]*ciCacheEntry),
		detailsCache:    make(map[string]*detailsCacheEntry),
		accessHistory:   make(map[string]int64),
		trustManager:    trustManager,
		ctx:             ctx,
		cancel:          cancel,
		focusedPane:     0,
		zoomedPane:      -1,
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
	}
	if cfg.SearchAutoSelect && !m.showingFilter {
		m.showingFilter = true
	}
	if m.showingFilter {
		m.setFilterTarget(filterTargetWorktrees)
		m.filterInput.Focus()
	}

	return m
}

// Init satisfies the tea.Model interface and starts with no command.
func (m *Model) Init() tea.Cmd {
	m.loadCommandHistory()
	m.loadAccessHistory()
	cmds := []tea.Cmd{
		m.loadCache(),
		m.refreshWorktrees(),
		m.spinner.Tick,
	}
	if m.showingFilter {
		cmds = append(cmds, textinput.Blink)
	}
	return tea.Batch(cmds...)
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
		if m.loadingScreen != nil && m.currentScreen == screenLoading {
			m.loadingScreen.Tick()
		}
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

	case openIssuesLoadedMsg:
		return m, m.handleOpenIssuesLoaded(msg)

	case createFromPRResultMsg:
		m.loading = false
		if m.currentScreen == screenLoading {
			m.currentScreen = screenNone
			m.loadingScreen = nil
		}
		if msg.err != nil {
			m.pendingSelectWorktreePath = ""
			m.showInfo(fmt.Sprintf("Failed to create worktree from PR/MR #%d: %v", msg.prNumber, msg.err), nil)
			return m, nil
		}
		env := m.buildCommandEnv(msg.branch, msg.targetPath)
		initCmds := m.collectInitCommands()
		after := func() tea.Msg {
			worktrees, err := m.git.GetWorktrees(m.ctx)
			return worktreesLoadedMsg{worktrees: worktrees, err: err}
		}
		return m, m.runCommandsWithTrust(initCmds, msg.targetPath, env, after)

	case createFromIssueResultMsg:
		m.loading = false
		if m.currentScreen == screenLoading {
			m.currentScreen = screenNone
			m.loadingScreen = nil
		}
		if msg.err != nil {
			m.pendingSelectWorktreePath = ""
			m.showInfo(fmt.Sprintf("Failed to create worktree from issue #%d: %v", msg.issueNumber, msg.err), nil)
			return m, nil
		}
		env := m.buildCommandEnv(msg.branch, msg.targetPath)
		initCmds := m.collectInitCommands()
		after := func() tea.Msg {
			worktrees, err := m.git.GetWorktrees(m.ctx)
			return worktreesLoadedMsg{worktrees: worktrees, err: err}
		}
		return m, m.runCommandsWithTrust(initCmds, msg.targetPath, env, after)

	case createFromChangesReadyMsg:
		return m, m.handleCreateFromChangesReady(msg)

	case prDataLoadedMsg, ciStatusLoadedMsg:
		return m.handlePRMessages(msg)

	case statusUpdatedMsg:
		if msg.info != "" {
			m.infoContent = msg.info
		}
		m.setStatusFiles(msg.statusFiles)
		if msg.log != nil {
			reset := false
			if msg.path != "" && msg.path != m.currentDetailsPath {
				m.currentDetailsPath = msg.path
				reset = true
			}
			m.setLogEntries(msg.log, reset)
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
			m.showInfo(fmt.Sprintf("Error: %v", msg.err), nil)
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
	case zellijSessionReadyMsg:
		if msg.attach && !msg.insideZellij {
			return m, m.attachZellijSessionCmd(msg.sessionName)
		}
		message := buildZellijInfoMessage(msg.sessionName)
		m.infoScreen = NewInfoScreen(message, m.theme)
		m.currentScreen = screenInfo
		return m, nil

	case refreshCompleteMsg:
		return m, m.updateDetailsView()

	case fetchRemotesCompleteMsg:
		m.statusContent = "Remotes fetched"
		// Continue showing loading screen while refreshing worktrees
		if m.loadingScreen != nil {
			m.loadingScreen.message = loadingRefreshWorktrees
		}
		return m, m.refreshWorktrees()

	case cherryPickResultMsg:
		return m, m.handleCherryPickResult(msg)

	case commitFilesLoadedMsg:
		if msg.err != nil {
			m.showInfo(fmt.Sprintf("Failed to load commit files: %v", msg.err), nil)
			return m, nil
		}
		// If only one file, show its diff directly without file picker
		if len(msg.files) == 1 {
			return m, m.showCommitFileDiff(msg.sha, msg.files[0].Filename, msg.worktreePath)
		}
		m.commitFilesScreen = NewCommitFilesScreen(
			msg.sha,
			msg.worktreePath,
			msg.files,
			msg.meta,
			m.windowWidth,
			m.windowHeight,
			m.theme,
			m.config.ShowIcons,
		)
		m.currentScreen = screenCommitFiles
		return m, nil

	}

	return m, tea.Batch(cmds...)
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
	case screenIssueSelect:
		return "issue-select"
	case screenListSelect:
		return "list-select"
	case screenCommitFiles:
		return "commit-files"
	default:
		return "unknown"
	}
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
	case screenIssueSelect:
		if m.issueSelectionScreen != nil {
			return m.overlayPopup(baseView, m.issueSelectionScreen.View(), 2)
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
	case screenLoading:
		if m.loadingScreen != nil {
			return m.overlayPopup(baseView, m.loadingScreen.View(), 5)
		}
	case screenCommitFiles:
		if m.commitFilesScreen != nil {
			return m.overlayPopup(baseView, m.commitFilesScreen.View(), 2)
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
	leftPad := max((baseWidth-popupWidth)/2, 0)

	// "Clear" styling for the background band
	// We use the default terminal background color (reset)
	leftSpace := strings.Repeat(" ", leftPad)
	rightPad := max(baseWidth-popupWidth-leftPad, 0)
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

func (m *Model) inputLabel() string {
	if m.showingSearch {
		return m.searchLabel()
	}
	return m.filterLabel()
}

func (m *Model) searchLabel() string {
	switch m.searchTarget {
	case searchTargetStatus:
		return "🔍 Search Files"
	case searchTargetLog:
		return "🔍 Search Commits"
	default:
		return "🔍 Search Worktrees"
	}
}

func (m *Model) filterLabel() string {
	switch m.filterTarget {
	case filterTargetStatus:
		return "🔍 Filter Files"
	case filterTargetLog:
		return "🔍 Filter Commits"
	default:
		return "🔍 Filter Worktrees"
	}
}

func (m *Model) filterPlaceholder(target filterTarget) string {
	switch target {
	case filterTargetStatus:
		return placeholderFilterFiles
	case filterTargetLog:
		return "Filter commits..."
	default:
		return filterWorktreesPlaceholder
	}
}

func (m *Model) filterQueryForTarget(target filterTarget) string {
	switch target {
	case filterTargetStatus:
		return m.statusFilterQuery
	case filterTargetLog:
		return m.logFilterQuery
	default:
		return m.filterQuery
	}
}

func (m *Model) setFilterQuery(target filterTarget, query string) {
	switch target {
	case filterTargetStatus:
		m.statusFilterQuery = query
	case filterTargetLog:
		m.logFilterQuery = query
	default:
		m.filterQuery = query
	}
}

func (m *Model) hasActiveFilterForPane(paneIndex int) bool {
	switch paneIndex {
	case 0:
		return strings.TrimSpace(m.filterQuery) != ""
	case 1:
		return strings.TrimSpace(m.statusFilterQuery) != ""
	case 2:
		return strings.TrimSpace(m.logFilterQuery) != ""
	}
	return false
}

func (m *Model) setFilterTarget(target filterTarget) {
	m.filterTarget = target
	m.filterInput.Placeholder = m.filterPlaceholder(target)
	m.filterInput.SetValue(m.filterQueryForTarget(target))
	m.filterInput.CursorEnd()
}

func (m *Model) searchPlaceholder(target searchTarget) string {
	switch target {
	case searchTargetStatus:
		return "Search files..."
	case searchTargetLog:
		return "Search commit titles..."
	default:
		return "Search worktrees..."
	}
}

func (m *Model) searchQueryForTarget(target searchTarget) string {
	switch target {
	case searchTargetStatus:
		return m.statusSearchQuery
	case searchTargetLog:
		return m.logSearchQuery
	default:
		return m.worktreeSearchQuery
	}
}

func (m *Model) setSearchQuery(target searchTarget, query string) {
	switch target {
	case searchTargetStatus:
		m.statusSearchQuery = query
	case searchTargetLog:
		m.logSearchQuery = query
	default:
		m.worktreeSearchQuery = query
	}
}

func (m *Model) setSearchTarget(target searchTarget) {
	m.searchTarget = target
	m.filterInput.Placeholder = m.searchPlaceholder(target)
	m.filterInput.SetValue(m.searchQueryForTarget(target))
	m.filterInput.CursorEnd()
}

func (m *Model) startSearch(target searchTarget) tea.Cmd {
	m.showingSearch = true
	m.showingFilter = false
	m.setSearchTarget(target)
	m.filterInput.Focus()
	return textinput.Blink
}

func (m *Model) startFilter(target filterTarget) tea.Cmd {
	m.showingFilter = true
	m.showingSearch = false
	m.setFilterTarget(target)
	m.filterInput.Focus()
	return textinput.Blink
}

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

	// Sort based on current sort mode
	switch m.sortMode {
	case sortModeLastActive:
		sort.Slice(m.filteredWts, func(i, j int) bool {
			return m.filteredWts[i].LastActiveTS > m.filteredWts[j].LastActiveTS
		})
	case sortModeLastSwitched:
		sort.Slice(m.filteredWts, func(i, j int) bool {
			return m.filteredWts[i].LastSwitchedTS > m.filteredWts[j].LastSwitchedTS
		})
	default: // sortModePath
		sort.Slice(m.filteredWts, func(i, j int) bool {
			return m.filteredWts[i].Path < m.filteredWts[j].Path
		})
	}

	// Update table rows
	rows := make([]table.Row, 0, len(m.filteredWts))
	for _, wt := range m.filteredWts {
		name := filepath.Base(wt.Path)
		if wt.IsMain {
			name = " " + mainWorktreeName
		} else {
			name = " " + name
		}

		status := "✓ "
		if wt.Dirty {
			status = "✎ "
		}

		// Build lazygit-style sync status: ↓N↑M, ✓ (in sync), or - (no upstream)
		var abStr string
		switch {
		case !wt.HasUpstream:
			abStr = "-"
		case wt.Ahead == 0 && wt.Behind == 0:
			abStr = "✓ "
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
				prIcon := ""
				if m.config.ShowIcons {
					prIcon = iconWithSpace(iconPR)
				}
				// Use Unicode symbols to indicate PR state
				var stateSymbol string
				switch wt.PR.State {
				case "OPEN":
					stateSymbol = symbolFilledCircle
				case "MERGED":
					stateSymbol = "◆"
				case "CLOSED":
					stateSymbol = "✕"
				default:
					stateSymbol = "?"
				}
				// Right-align PR numbers for consistent column width
				prStr = fmt.Sprintf("%s#%-5d%s", prIcon, wt.PR.Number, stateSymbol)
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
		cursor := max(m.worktreeTable.Cursor(), 0)
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
			m.statusContent = loadingRefreshWorktrees
		}
		return nil
	}
	return func() tea.Msg {
		statusRaw, logRaw, unpushed, unmerged := m.getCachedDetails(wt)

		// Parse log
		logEntries := []commitLogEntry{}
		for line := range strings.SplitSeq(logRaw, "\n") {
			parts := strings.SplitN(line, "\t", 3)
			if len(parts) < 2 {
				continue
			}
			sha := parts[0]
			message := parts[len(parts)-1]
			author := ""
			if len(parts) == 3 {
				author = parts[1]
			}
			logEntries = append(logEntries, commitLogEntry{
				sha:            sha,
				authorInitials: authorInitials(author),
				message:        message,
				isUnpushed:     unpushed[sha],
				isUnmerged:     unmerged[sha],
			})
		}
		return statusUpdatedMsg{
			info:        m.buildInfoContent(wt),
			statusFiles: parseStatusFiles(statusRaw),
			log:         logEntries,
			path:        wt.Path,
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
		// First try the traditional approach (matches by headRefName)
		prMap, err := m.git.FetchPRMap(m.ctx)
		if err != nil {
			return prDataLoadedMsg{prMap: nil, err: err}
		}

		// Also fetch PRs per worktree for cases where local branch differs from remote
		// This handles fork PRs where local branch name doesn't match headRefName
		worktreePRs := make(map[string]*models.PRInfo)
		for _, wt := range m.worktrees {
			// Skip if already matched by headRefName
			if _, ok := prMap[wt.Branch]; ok {
				continue
			}
			// Try to fetch PR for this worktree directly
			if pr := m.git.FetchPRForWorktree(m.ctx, wt.Path); pr != nil {
				worktreePRs[wt.Path] = pr
			}
		}

		return prDataLoadedMsg{
			prMap:       prMap,
			worktreePRs: worktreePRs,
			err:         nil,
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

func (m *Model) showCreateFromIssue() tea.Cmd {
	// Fetch all open issues
	return func() tea.Msg {
		issues, err := m.git.FetchAllOpenIssues(m.ctx)
		return openIssuesLoadedMsg{
			issues: issues,
			err:    err,
		}
	}
}

func (m *Model) showCreateWorktreeFromChanges() tea.Cmd {
	// Check if a worktree is selected
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		m.showInfo(errNoWorktreeSelected, nil)
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

func (m *Model) showCreateFromChangesInput(wt *models.WorktreeInfo, currentBranch, defaultName string) tea.Cmd {
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

func (m *Model) showDeleteFile() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	if len(m.statusTreeFlat) == 0 || m.statusTreeIndex < 0 || m.statusTreeIndex >= len(m.statusTreeFlat) {
		return nil
	}
	node := m.statusTreeFlat[m.statusTreeIndex]
	wt := m.filteredWts[m.selectedIndex]

	if node.IsDir() {
		files := node.CollectFiles()
		if len(files) == 0 {
			return nil
		}
		m.confirmScreen = NewConfirmScreen(fmt.Sprintf("Delete %d file(s) in directory?\n\nDirectory: %s", len(files), node.Path), m.theme)
		m.confirmAction = m.deleteFilesCmd(wt, files)
	} else {
		m.confirmScreen = NewConfirmScreen(fmt.Sprintf("Delete file?\n\nFile: %s", node.File.Filename), m.theme)
		m.confirmAction = m.deleteFilesCmd(wt, []*StatusFile{node.File})
	}
	m.currentScreen = screenConfirm
	return nil
}

func (m *Model) deleteFilesCmd(wt *models.WorktreeInfo, files []*StatusFile) func() tea.Cmd {
	return func() tea.Cmd {
		env := m.buildCommandEnv(wt.Branch, wt.Path)
		envVars := os.Environ()
		for k, v := range env {
			envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
		}

		for _, sf := range files {
			filePath := filepath.Join(wt.Path, sf.Filename)

			if sf.IsUntracked {
				if err := os.Remove(filePath); err != nil {
					return func() tea.Msg { return errMsg{err: err} }
				}
			} else {
				// Restore the file from git (discard all changes)
				cmdStr := fmt.Sprintf("git checkout HEAD -- %s", shellQuote(sf.Filename))
				// #nosec G204 -- command is constructed with quoted filename
				c := m.commandRunner("bash", "-c", cmdStr)
				c.Dir = wt.Path
				c.Env = envVars
				if err := c.Run(); err != nil {
					return func() tea.Msg { return errMsg{err: err} }
				}
			}
		}

		// Clear cache so status pane refreshes
		delete(m.detailsCache, wt.Path)
		return func() tea.Msg { return refreshCompleteMsg{} }
	}
}

func (m *Model) showDiff() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	wt := m.filteredWts[m.selectedIndex]

	// Build environment variables
	env := m.buildCommandEnv(wt.Branch, wt.Path)
	envVars := os.Environ()
	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	// Get pager configuration
	pager := m.pagerCommand()
	pagerEnv := m.pagerEnv(pager)
	pagerCmd := pager
	if pagerEnv != "" {
		pagerCmd = fmt.Sprintf("%s %s", pagerEnv, pager)
	}

	// Build a script that replicates BuildThreePartDiff behavior
	// This shows: 1) Staged changes, 2) Unstaged changes, 3) Untracked files (limited)
	maxUntracked := m.config.MaxUntrackedDiffs
	script := fmt.Sprintf(`
set -e
# Part 1: Staged changes
staged=$(git diff --cached --patch --no-color 2>/dev/null || true)
if [ -n "$staged" ]; then
  echo "=== Staged Changes ==="
  echo "$staged"
  echo
fi

# Part 2: Unstaged changes
unstaged=$(git diff --patch --no-color 2>/dev/null || true)
if [ -n "$unstaged" ]; then
  echo "=== Unstaged Changes ==="
  echo "$unstaged"
  echo
fi

# Part 3: Untracked files (limited to %d)
untracked=$(git status --porcelain 2>/dev/null | grep '^?? ' | cut -d' ' -f2- || true)
if [ -n "$untracked" ]; then
  count=0
  max_count=%d
  total=$(echo "$untracked" | wc -l)
  while IFS= read -r file; do
    [ $count -ge $max_count ] && break
    echo "=== Untracked: $file ==="
    git diff --no-index /dev/null "$file" 2>/dev/null || true
    echo
    count=$((count + 1))
  done <<< "$untracked"

  if [ $total -gt $max_count ]; then
    echo "[...showing $count of $total untracked files]"
  fi
fi
`, maxUntracked, maxUntracked)

	// Pipe through delta if configured, then through pager
	var cmdStr string
	if m.git.UseDelta() {
		deltaArgs := strings.Join(m.config.DeltaArgs, " ")
		cmdStr = fmt.Sprintf("set -o pipefail; (%s) | %s %s | %s", script, m.config.DeltaPath, deltaArgs, pagerCmd)
	} else {
		cmdStr = fmt.Sprintf("set -o pipefail; (%s) | %s", script, pagerCmd)
	}

	// Create command
	// #nosec G204 -- command is constructed from config and controlled inputs
	c := m.commandRunner("bash", "-c", cmdStr)
	c.Dir = wt.Path
	c.Env = envVars

	return m.execProcess(c, func(err error) tea.Msg {
		if err != nil {
			return errMsg{err: err}
		}
		return refreshCompleteMsg{}
	})
}

// showFileDiff shows the diff for a single file in a pager.
func (m *Model) showFileDiff(sf StatusFile) tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	wt := m.filteredWts[m.selectedIndex]

	// Build environment variables
	env := m.buildCommandEnv(wt.Branch, wt.Path)
	envVars := os.Environ()
	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	// Get pager configuration
	pager := m.pagerCommand()
	pagerEnv := m.pagerEnv(pager)
	pagerCmd := pager
	if pagerEnv != "" {
		pagerCmd = fmt.Sprintf("%s %s", pagerEnv, pager)
	}

	// Build script based on file type
	var script string
	// Shell-escape the filename for safe use in shell commands
	escapedFilename := fmt.Sprintf("'%s'", strings.ReplaceAll(sf.Filename, "'", "'\\''"))

	if sf.IsUntracked {
		// For untracked files, show diff against /dev/null
		script = fmt.Sprintf(`
set -e
echo "=== Untracked:" %s "==="
git diff --no-index /dev/null %s 2>/dev/null || true
`, escapedFilename, escapedFilename)
	} else {
		// For tracked files, show both staged and unstaged changes
		script = fmt.Sprintf(`
set -e
# Staged changes for this file
staged=$(git diff --cached --patch --no-color -- %s 2>/dev/null || true)
if [ -n "$staged" ]; then
  echo "=== Staged Changes:" %s "==="
  echo "$staged"
  echo
fi

# Unstaged changes for this file
unstaged=$(git diff --patch --no-color -- %s 2>/dev/null || true)
if [ -n "$unstaged" ]; then
  echo "=== Unstaged Changes:" %s "==="
  echo "$unstaged"
  echo
fi
`, escapedFilename, escapedFilename, escapedFilename, escapedFilename)
	}

	// Pipe through delta if configured, then through pager
	var cmdStr string
	if m.git.UseDelta() {
		deltaArgs := strings.Join(m.config.DeltaArgs, " ")
		cmdStr = fmt.Sprintf("set -o pipefail; (%s) | %s %s | %s", script, m.config.DeltaPath, deltaArgs, pagerCmd)
	} else {
		cmdStr = fmt.Sprintf("set -o pipefail; (%s) | %s", script, pagerCmd)
	}

	// Create command
	// #nosec G204 -- command is constructed from config and controlled inputs
	c := m.commandRunner("bash", "-c", cmdStr)
	c.Dir = wt.Path
	c.Env = envVars

	return m.execProcess(c, func(err error) tea.Msg {
		if err != nil {
			return errMsg{err: err}
		}
		return refreshCompleteMsg{}
	})
}

func (m *Model) openStatusFileInEditor(sf StatusFile) tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	wt := m.filteredWts[m.selectedIndex]

	editor := m.editorCommand()
	if strings.TrimSpace(editor) == "" {
		m.showInfo("No editor configured. Set editor in config or $EDITOR.", nil)
		return nil
	}

	filePath := filepath.Join(wt.Path, sf.Filename)
	if _, err := os.Stat(filePath); err != nil {
		m.showInfo(fmt.Sprintf("Cannot open %s: %v", sf.Filename, err), nil)
		return nil
	}

	env := m.buildCommandEnv(wt.Branch, wt.Path)
	envVars := os.Environ()
	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	cmdStr := fmt.Sprintf("%s %s", editor, shellQuote(sf.Filename))
	// #nosec G204 -- command is constructed from user config and controlled inputs
	c := m.commandRunner("bash", "-c", cmdStr)
	c.Dir = wt.Path
	c.Env = envVars

	return m.execProcess(c, func(err error) tea.Msg {
		if err != nil {
			return errMsg{err: err}
		}
		return refreshCompleteMsg{}
	})
}

func (m *Model) commitAllChanges() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	wt := m.filteredWts[m.selectedIndex]

	env := m.buildCommandEnv(wt.Branch, wt.Path)
	envVars := os.Environ()
	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	// Clear cache so status pane refreshes with latest git status
	delete(m.detailsCache, wt.Path)

	// #nosec G204 -- command is a fixed git command
	c := m.commandRunner("bash", "-c", "git add -A && git commit")
	c.Dir = wt.Path
	c.Env = envVars

	return m.execProcess(c, func(err error) tea.Msg {
		if err != nil {
			return errMsg{err: err}
		}
		return refreshCompleteMsg{}
	})
}

func (m *Model) commitStagedChanges() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	wt := m.filteredWts[m.selectedIndex]

	// Check if there are any staged changes
	hasStagedChanges := false
	for _, sf := range m.statusFilesAll {
		if len(sf.Status) >= 2 {
			x := sf.Status[0] // Staged status
			if x != '.' && x != ' ' {
				hasStagedChanges = true
				break
			}
		}
	}

	if !hasStagedChanges {
		m.showInfo("No staged changes to commit", nil)
		return nil
	}

	env := m.buildCommandEnv(wt.Branch, wt.Path)
	envVars := os.Environ()
	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	// Clear cache so status pane refreshes with latest git status
	delete(m.detailsCache, wt.Path)

	// #nosec G204 -- command is a fixed git command
	c := m.commandRunner("bash", "-c", "git commit")
	c.Dir = wt.Path
	c.Env = envVars

	return m.execProcess(c, func(err error) tea.Msg {
		if err != nil {
			return errMsg{err: err}
		}
		return refreshCompleteMsg{}
	})
}

func (m *Model) stageCurrentFile(sf StatusFile) tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	wt := m.filteredWts[m.selectedIndex]

	// Status is XY format: X=staged, Y=unstaged
	// Examples: "M " = staged, " M" = unstaged, "MM" = both
	if len(sf.Status) < 2 {
		return nil
	}

	x := sf.Status[0] // Staged status
	y := sf.Status[1] // Unstaged status

	var cmdStr string
	hasUnstagedChanges := y != '.' && y != ' '
	hasStagedChanges := x != '.' && x != ' '
	hasNoUnstagedChanges := y == '.' || y == ' '

	switch {
	case hasUnstagedChanges:
		// If there are unstaged changes, stage them
		cmdStr = fmt.Sprintf("git add %s", shellQuote(sf.Filename))
	case hasStagedChanges && hasNoUnstagedChanges:
		// File is fully staged with no unstaged changes, so unstage it
		cmdStr = fmt.Sprintf("git restore --staged %s", shellQuote(sf.Filename))
	default:
		// File is clean or in an unexpected state
		return nil
	}

	env := m.buildCommandEnv(wt.Branch, wt.Path)
	envVars := os.Environ()
	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	// Clear cache so status pane refreshes with latest git status
	delete(m.detailsCache, wt.Path)

	// Run git command in background without suspending the TUI to avoid flicker
	// #nosec G204 -- command is constructed with quoted filename
	c := m.commandRunner("bash", "-c", cmdStr)
	c.Dir = wt.Path
	c.Env = envVars

	return func() tea.Msg {
		if err := c.Run(); err != nil {
			return errMsg{err: err}
		}
		return refreshCompleteMsg{}
	}
}

func (m *Model) stageDirectory(node *StatusTreeNode) tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	wt := m.filteredWts[m.selectedIndex]

	files := node.CollectFiles()
	if len(files) == 0 {
		return nil
	}

	// Check if all files are fully staged (no unstaged changes)
	allStaged := true
	for _, f := range files {
		if len(f.Status) < 2 {
			continue
		}
		y := f.Status[1] // Unstaged status
		if y != '.' && y != ' ' {
			allStaged = false
			break
		}
	}

	// Build file list for git command
	fileArgs := make([]string, 0, len(files))
	for _, f := range files {
		fileArgs = append(fileArgs, shellQuote(f.Filename))
	}
	fileList := strings.Join(fileArgs, " ")

	var cmdStr string
	if allStaged {
		// All files are staged, unstage them all
		cmdStr = fmt.Sprintf("git restore --staged %s", fileList)
	} else {
		// Mixed or all unstaged, stage them all
		cmdStr = fmt.Sprintf("git add %s", fileList)
	}

	env := m.buildCommandEnv(wt.Branch, wt.Path)
	envVars := os.Environ()
	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	// Clear cache so status pane refreshes with latest git status
	delete(m.detailsCache, wt.Path)

	// Run git command in background without suspending the TUI to avoid flicker
	// #nosec G204 -- command is constructed with quoted filenames
	c := m.commandRunner("bash", "-c", cmdStr)
	c.Dir = wt.Path
	c.Env = envVars

	return func() tea.Msg {
		if err := c.Run(); err != nil {
			return errMsg{err: err}
		}
		return refreshCompleteMsg{}
	}
}

func (m *Model) showRenameWorktree() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}

	wt := m.filteredWts[m.selectedIndex]
	if wt.IsMain {
		m.showInfo("Cannot rename the main worktree.", nil)
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

func (m *Model) showRunCommand() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}

	m.currentScreen = screenInput
	m.inputScreen = NewInputScreen(
		"Run command in worktree",
		"e.g., make test, npm install, etc.",
		"",
		m.theme,
	)
	// Enable bash-style history navigation with up/down arrows
	// Always set history, even if empty - it will populate as commands are added
	m.inputScreen.SetHistory(m.commandHistory)
	m.inputSubmit = func(value string) (tea.Cmd, bool) {
		cmdStr := strings.TrimSpace(value)
		if cmdStr == "" {
			return nil, true // Close without running
		}
		// Add command to history
		m.addToCommandHistory(cmdStr)
		return m.executeArbitraryCommand(cmdStr), true
	}
	return nil
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
		m.showInfo("No merged PR worktrees to prune.", nil)
		return nil
	}

	// Build confirmation message (truncate if long)
	lines := []string{}
	limit := min(len(merged), 10)
	for i := range limit {
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
		m.infoScreen = NewInfoScreen("Cannot absorb the main worktree.", m.theme)
		m.currentScreen = screenInfo
		return nil
	}

	mainBranch := m.git.GetMainBranch(m.ctx)

	// Prevent absorbing if the selected worktree is on the main branch
	if wt.Branch == mainBranch {
		m.infoScreen = NewInfoScreen(
			fmt.Sprintf("Cannot absorb: worktree is on the main branch (%s).", mainBranch),
			m.theme,
		)
		m.currentScreen = screenInfo
		return nil
	}

	// Find the main worktree explicitly (don't use fallback)
	var mainWorktree *models.WorktreeInfo
	for _, w := range m.worktrees {
		if w.IsMain {
			mainWorktree = w
			break
		}
	}
	if mainWorktree == nil {
		m.infoScreen = NewInfoScreen("Cannot find main worktree.", m.theme)
		m.currentScreen = screenInfo
		return nil
	}

	// Check if main worktree has uncommitted changes
	if mainWorktree.Dirty {
		m.infoScreen = NewInfoScreen(
			fmt.Sprintf("Cannot absorb: main worktree has uncommitted changes.\n\nCommit or stash changes in:\n%s", mainWorktree.Path),
			m.theme,
		)
		m.currentScreen = screenInfo
		return nil
	}

	mainPath := mainWorktree.Path
	mergeMethod := m.config.MergeMethod
	if mergeMethod == "" {
		mergeMethod = mergeMethodRebase
	}

	m.confirmScreen = NewConfirmScreen(fmt.Sprintf("Absorb worktree into %s (%s)?\n\nPath: %s\nBranch: %s -> %s", mainBranch, mergeMethod, wt.Path, wt.Branch, mainBranch), m.theme)
	m.confirmAction = func() tea.Cmd {
		return func() tea.Msg {
			if mergeMethod == mergeMethodRebase {
				// Rebase: first rebase the feature branch onto main, then fast-forward main
				if !m.git.RunCommandChecked(m.ctx, []string{"git", "-C", wt.Path, "rebase", mainBranch}, "", fmt.Sprintf("Failed to rebase %s onto %s", wt.Branch, mainBranch)) {
					return absorbMergeResultMsg{
						path:   wt.Path,
						branch: wt.Branch,
						err:    fmt.Errorf("rebase failed; resolve conflicts in %s and retry", wt.Path),
					}
				}
				// Fast-forward main to the rebased branch
				if !m.git.RunCommandChecked(m.ctx, []string{"git", "-C", mainPath, "merge", "--ff-only", wt.Branch}, "", fmt.Sprintf("Failed to fast-forward %s to %s", mainBranch, wt.Branch)) {
					return absorbMergeResultMsg{
						path:   wt.Path,
						branch: wt.Branch,
						err:    fmt.Errorf("fast-forward failed; the branch may have diverged"),
					}
				}
			} else if !m.git.RunCommandChecked(m.ctx, []string{"git", "-C", mainPath, "merge", "--no-edit", wt.Branch}, "", fmt.Sprintf("Failed to merge %s into %s", wt.Branch, mainBranch)) {
				// Merge: traditional merge
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
	customItems := m.customPaletteItems()
	items := make([]paletteItem, 0, 10+len(customItems))
	items = append(items,
		paletteItem{id: "create", label: "Create worktree (c)", description: "Add a new worktree from base branch or PR/MR"},
		paletteItem{id: "create-from-changes", label: "Create from changes", description: "Create a new worktree from current uncommitted changes"},
		paletteItem{id: "delete", label: "Delete worktree (D)", description: "Remove worktree and branch"},
		paletteItem{id: "rename", label: "Rename worktree (m)", description: "Rename worktree and branch"},
		paletteItem{id: "absorb", label: "Absorb worktree (A)", description: "Merge branch into main and remove worktree"},
		paletteItem{id: "prune", label: "Prune merged (X)", description: "Remove merged PR worktrees"},
		paletteItem{id: "refresh", label: "Refresh (r)", description: "Reload worktrees"},
		paletteItem{id: "fetch", label: "Fetch remotes (R)", description: "git fetch --all"},
		paletteItem{id: "pr", label: "Open PR (o)", description: "Open PR in browser"},
		paletteItem{id: "help", label: "Help (?)", description: "Show help"},
	)
	items = append(items, customItems...)

	m.paletteScreen = NewCommandPaletteScreen(items, m.theme)
	m.paletteSubmit = func(action string) tea.Cmd {
		m.debugf("palette action: %s", action)
		if _, ok := m.config.CustomCommands[action]; ok {
			return m.executeCustomCommand(action)
		}
		switch action {
		case "create":
			return m.showCreateWorktree()
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
		switch {
		case cmd.Command != "":
			description = cmd.Command
		case cmd.Zellij != nil:
			description = zellijSessionLabel
		case cmd.Tmux != nil:
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
		if strings.TrimSpace(cmd.Command) == "" && cmd.Tmux == nil && cmd.Zellij == nil {
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
			if label == "" {
				if cmd.Zellij != nil {
					label = zellijSessionLabel
				} else if cmd.Tmux != nil {
					label = tmuxSessionLabel
				}
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

	if customCmd.Zellij != nil {
		return m.openZellijSession(customCmd.Zellij, wt)
	}

	if customCmd.Tmux != nil {
		return m.openTmuxSession(customCmd.Tmux, wt)
	}

	if customCmd.ShowOutput {
		return m.executeCustomCommandWithPager(customCmd, wt)
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

func (m *Model) executeCustomCommandWithPager(customCmd *config.CustomCommand, wt *models.WorktreeInfo) tea.Cmd {
	env := m.buildCommandEnv(wt.Branch, wt.Path)
	envVars := os.Environ()
	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	pager := m.pagerCommand()
	pagerEnv := m.pagerEnv(pager)
	pagerCmd := pager
	if pagerEnv != "" {
		pagerCmd = fmt.Sprintf("%s %s", pagerEnv, pager)
	}
	cmdStr := fmt.Sprintf("set -o pipefail; (%s) 2>&1 | %s", customCmd.Command, pagerCmd)
	// Always run via shell to support pipes, redirects, and shell features
	// #nosec G204 -- command comes from user's own config file
	c := m.commandRunner("bash", "-c", cmdStr)
	c.Dir = wt.Path
	c.Env = envVars

	return m.execProcess(c, func(err error) tea.Msg {
		if err != nil {
			return errMsg{err: err}
		}
		return refreshCompleteMsg{}
	})
}

func (m *Model) executeArbitraryCommand(cmdStr string) tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}

	wt := m.filteredWts[m.selectedIndex]

	// Build environment variables (same as custom commands)
	env := m.buildCommandEnv(wt.Branch, wt.Path)
	envVars := os.Environ()
	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	// Get pager configuration
	pager := m.pagerCommand()
	pagerEnv := m.pagerEnv(pager)
	pagerCmd := pager
	if pagerEnv != "" {
		pagerCmd = fmt.Sprintf("%s %s", pagerEnv, pager)
	}

	// Build command string that pipes output through pager
	fullCmdStr := fmt.Sprintf("set -o pipefail; (%s) 2>&1 | %s", cmdStr, pagerCmd)

	// Create command with bash shell
	// #nosec G204 -- command comes from user input in TUI
	c := m.commandRunner("bash", "-c", fullCmdStr)
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

func (m *Model) openZellijSession(zellijCfg *config.TmuxCommand, wt *models.WorktreeInfo) tea.Cmd {
	if zellijCfg == nil {
		return nil
	}

	env := m.buildCommandEnv(wt.Branch, wt.Path)
	insideZellij := os.Getenv("ZELLIJ") != "" || os.Getenv("ZELLIJ_SESSION_NAME") != ""
	sessionName := strings.TrimSpace(expandWithEnv(zellijCfg.SessionName, env))
	if sessionName == "" {
		sessionName = fmt.Sprintf("wt:%s", filepath.Base(wt.Path))
	}
	sessionName = sanitizeZellijSessionName(sessionName)

	resolved, ok := resolveTmuxWindows(zellijCfg.Windows, env, wt.Path)
	if !ok {
		return func() tea.Msg {
			return errMsg{err: fmt.Errorf("failed to resolve zellij windows")}
		}
	}

	layoutPaths, err := writeZellijLayouts(resolved)
	if err != nil {
		return func() tea.Msg {
			return errMsg{err: err}
		}
	}

	sessionFile, err := os.CreateTemp("", "lazyworktree-zellij-")
	if err != nil {
		cleanupZellijLayouts(layoutPaths)
		return func() tea.Msg {
			return errMsg{err: err}
		}
	}
	sessionPath := sessionFile.Name()
	if closeErr := sessionFile.Close(); closeErr != nil {
		cleanupZellijLayouts(layoutPaths)
		return func() tea.Msg {
			return errMsg{err: closeErr}
		}
	}

	scriptCfg := *zellijCfg
	scriptCfg.Attach = false
	env["LW_ZELLIJ_SESSION_FILE"] = sessionPath
	script := buildZellijScript(sessionName, &scriptCfg, layoutPaths)
	// #nosec G204 -- command is built from user-configured zellij session settings.
	c := m.commandRunner("bash", "-lc", script)
	c.Dir = wt.Path
	c.Env = append(os.Environ(), envMapToList(env)...)

	return m.execProcess(c, func(err error) tea.Msg {
		defer func() {
			_ = os.Remove(sessionPath)
			cleanupZellijLayouts(layoutPaths)
		}()
		if err != nil {
			return errMsg{err: err}
		}
		finalSession := readTmuxSessionFile(sessionPath, sessionName)
		return zellijSessionReadyMsg{
			sessionName:  finalSession,
			attach:       zellijCfg.Attach,
			insideZellij: insideZellij,
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

func (m *Model) showCherryPick() tea.Cmd {
	// Validate: log pane must be focused
	if m.focusedPane != 2 {
		return nil
	}

	// Validate: commit must be selected
	if len(m.logEntries) == 0 {
		return nil
	}

	cursor := m.logTable.Cursor()
	if cursor < 0 || cursor >= len(m.logEntries) {
		return nil
	}

	// Get source worktree and commit
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	sourceWorktree := m.filteredWts[m.selectedIndex]
	selectedCommit := m.logEntries[cursor]

	// Build worktree selection items (exclude source worktree)
	items := make([]selectionItem, 0, len(m.worktrees)-1)
	for _, wt := range m.worktrees {
		if wt.Path == sourceWorktree.Path {
			continue // Skip source worktree
		}

		name := filepath.Base(wt.Path)
		if wt.IsMain {
			name = "main"
		}

		desc := wt.Branch
		if wt.Dirty {
			desc += " (has changes)"
		}

		items = append(items, selectionItem{
			id:          wt.Path,
			label:       name,
			description: desc,
		})
	}

	// Check if no other worktrees available
	if len(items) == 0 {
		m.showInfo("No other worktrees available for cherry-pick.", nil)
		return nil
	}

	// Show worktree selection screen
	title := fmt.Sprintf("Cherry-pick %s to worktree", selectedCommit.sha)
	m.listScreen = NewListSelectionScreen(items, title, filterWorktreesPlaceholder, "No worktrees found.", m.windowWidth, m.windowHeight, "", m.theme)
	m.listSubmit = func(item selectionItem) tea.Cmd {
		// Find target worktree by path
		var targetWorktree *models.WorktreeInfo
		for _, wt := range m.worktrees {
			if wt.Path == item.id {
				targetWorktree = wt
				break
			}
		}

		if targetWorktree == nil {
			return func() tea.Msg {
				return errMsg{err: fmt.Errorf("target worktree not found")}
			}
		}

		// Clear list selection
		m.listScreen = nil
		m.listSubmit = nil
		m.currentScreen = screenNone

		// Execute cherry-pick
		return m.executeCherryPick(selectedCommit.sha, targetWorktree)
	}

	m.currentScreen = screenListSelect
	return textinput.Blink
}

func (m *Model) executeCherryPick(commitSHA string, targetWorktree *models.WorktreeInfo) tea.Cmd {
	return func() tea.Msg {
		_, err := m.git.CherryPickCommit(m.ctx, commitSHA, targetWorktree.Path)
		return cherryPickResultMsg{
			commitSHA:      commitSHA,
			targetWorktree: targetWorktree,
			err:            err,
		}
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

	return m.showCommitFilesScreen(entry.sha, wt.Path)
}

func (m *Model) showCommitFilesScreen(commitSHA, worktreePath string) tea.Cmd {
	return func() tea.Msg {
		files, err := m.git.GetCommitFiles(m.ctx, commitSHA, worktreePath)
		if err != nil {
			return errMsg{err: err}
		}
		// Fetch commit metadata
		metaRaw := m.git.RunGit(
			m.ctx,
			[]string{
				"git", "log", "-1",
				"--pretty=format:%H%x1f%an%x1f%ae%x1f%ad%x1f%s%x1f%b",
				commitSHA,
			},
			worktreePath,
			[]int{0},
			true,
			false,
		)
		meta := parseCommitMeta(metaRaw)
		// Ensure SHA is set even if parsing fails
		if meta.sha == "" {
			meta.sha = commitSHA
		}
		return commitFilesLoadedMsg{
			sha:          commitSHA,
			worktreePath: worktreePath,
			files:        files,
			meta:         meta,
		}
	}
}

func (m *Model) showCommitDiff(commitSHA string, wt *models.WorktreeInfo) tea.Cmd {
	// Build environment variables
	env := m.buildCommandEnv(wt.Branch, wt.Path)
	envVars := os.Environ()
	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	// Get pager configuration
	pager := m.pagerCommand()
	pagerEnv := m.pagerEnv(pager)
	pagerCmd := pager
	if pagerEnv != "" {
		pagerCmd = fmt.Sprintf("%s %s", pagerEnv, pager)
	}

	// Build git show command with colorization
	// --color=always: ensure color codes are passed to delta/pager
	gitCmd := fmt.Sprintf("git show --color=always %s", commitSHA)

	// Pipe through delta if configured, then through pager
	// Note: delta only processes the diff part, so our colorized commit message will pass through
	// Don't use pipefail here as awk might not always match (e.g., if commit format is different)
	var cmdStr string
	if m.git.UseDelta() {
		deltaArgs := strings.Join(m.config.DeltaArgs, " ")
		cmdStr = fmt.Sprintf("%s | %s %s | %s", gitCmd, m.config.DeltaPath, deltaArgs, pagerCmd)
	} else {
		cmdStr = fmt.Sprintf("%s | %s", gitCmd, pagerCmd)
	}

	// Create command
	// #nosec G204 -- command is constructed from config and controlled inputs
	c := m.commandRunner("bash", "-c", cmdStr)
	c.Dir = wt.Path
	c.Env = envVars

	return m.execProcess(c, func(err error) tea.Msg {
		if err != nil {
			return errMsg{err: err}
		}
		return refreshCompleteMsg{}
	})
}

func (m *Model) showCommitFileDiff(commitSHA, filename, worktreePath string) tea.Cmd {
	// Build environment variables for pager
	envVars := os.Environ()

	// Get pager configuration
	pager := m.pagerCommand()
	pagerEnv := m.pagerEnv(pager)
	pagerCmd := pager
	if pagerEnv != "" {
		pagerCmd = fmt.Sprintf("%s %s", pagerEnv, pager)
	}

	// Build git show command for specific file with colorization
	gitCmd := fmt.Sprintf("git show --color=always %s -- %q", commitSHA, filename)

	// Pipe through delta if configured, then through pager
	var cmdStr string
	if m.git.UseDelta() {
		deltaArgs := strings.Join(m.config.DeltaArgs, " ")
		cmdStr = fmt.Sprintf("%s | %s %s | %s", gitCmd, m.config.DeltaPath, deltaArgs, pagerCmd)
	} else {
		cmdStr = fmt.Sprintf("%s | %s", gitCmd, pagerCmd)
	}

	// Create command
	// #nosec G204 -- command is constructed from config and controlled inputs
	c := m.commandRunner("bash", "-c", cmdStr)
	c.Dir = worktreePath
	c.Env = envVars

	return m.execProcess(c, func(err error) tea.Msg {
		if err != nil {
			return errMsg{err: err}
		}
		return refreshCompleteMsg{}
	})
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
	case screenIssueSelect:
		if m.issueSelectionScreen == nil {
			m.currentScreen = screenNone
			return m, nil
		}
		keyStr := msg.String()
		if isEscKey(keyStr) {
			m.currentScreen = screenNone
			m.issueSelectionScreen = nil
			m.issueSelectionSubmit = nil
			return m, nil
		}
		if keyStr == keyEnter {
			if m.issueSelectionSubmit != nil {
				if issue, ok := m.issueSelectionScreen.Selected(); ok {
					cmd := m.issueSelectionSubmit(issue)
					// Don't set screenNone here - issueSelectionSubmit sets screenInput or screenListSelect
					m.issueSelectionScreen = nil
					m.issueSelectionSubmit = nil
					return m, cmd
				}
			}
		}
		is, cmd := m.issueSelectionScreen.Update(msg)
		if updated, ok := is.(*IssueSelectionScreen); ok {
			m.issueSelectionScreen = updated
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
	case screenCommitFiles:
		if m.commitFilesScreen == nil {
			m.currentScreen = screenNone
			return m, nil
		}
		keyStr := msg.String()

		// If filter or search is active, delegate to screen first
		if m.commitFilesScreen.showingFilter || m.commitFilesScreen.showingSearch {
			cs, cmd := m.commitFilesScreen.Update(msg)
			if updated, ok := cs.(*CommitFilesScreen); ok {
				m.commitFilesScreen = updated
			}
			return m, cmd
		}

		switch keyStr {
		case keyQ, keyCtrlC:
			m.commitFilesScreen = nil
			m.currentScreen = screenNone
			return m, nil
		case keyEsc, keyEscRaw:
			m.commitFilesScreen = nil
			m.currentScreen = screenNone
			return m, nil
		case "f":
			// Start filter mode
			m.commitFilesScreen.showingFilter = true
			m.commitFilesScreen.showingSearch = false
			m.commitFilesScreen.filterInput.Placeholder = placeholderFilterFiles
			m.commitFilesScreen.filterInput.SetValue(m.commitFilesScreen.filterQuery)
			m.commitFilesScreen.filterInput.Focus()
			return m, textinput.Blink
		case "/":
			// Start search mode
			m.commitFilesScreen.showingSearch = true
			m.commitFilesScreen.showingFilter = false
			m.commitFilesScreen.filterInput.Placeholder = "Search files..."
			m.commitFilesScreen.filterInput.SetValue("")
			m.commitFilesScreen.searchQuery = ""
			m.commitFilesScreen.filterInput.Focus()
			return m, textinput.Blink
		case "d":
			// Show full commit diff via pager
			sha := m.commitFilesScreen.commitSHA
			wtPath := m.commitFilesScreen.worktreePath
			m.commitFilesScreen = nil
			m.currentScreen = screenNone
			// Find the worktree to pass to showCommitDiff
			var wt *models.WorktreeInfo
			for _, w := range m.filteredWts {
				if w.Path == wtPath {
					wt = w
					break
				}
			}
			if wt != nil {
				return m, m.showCommitDiff(sha, wt)
			}
			return m, nil
		case keyEnter:
			node := m.commitFilesScreen.GetSelectedNode()
			if node == nil {
				return m, nil
			}
			if node.IsDir() {
				m.commitFilesScreen.ToggleCollapse(node.Path)
				return m, nil
			}
			// Show diff for this specific file
			sha := m.commitFilesScreen.commitSHA
			wtPath := m.commitFilesScreen.worktreePath
			return m, m.showCommitFileDiff(sha, node.File.Filename, wtPath)
		}
		// Delegate navigation to screen
		cs, cmd := m.commitFilesScreen.Update(msg)
		if updated, ok := cs.(*CommitFilesScreen); ok {
			m.commitFilesScreen = updated
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

		// Handle history navigation with up/down arrows
		if len(m.inputScreen.history) > 0 {
			switch keyStr {
			case "up":
				// Go to previous command in history (older)
				if m.inputScreen.historyIndex == -1 {
					m.inputScreen.originalInput = m.inputScreen.input.Value()
					m.inputScreen.historyIndex = 0
				} else if m.inputScreen.historyIndex < len(m.inputScreen.history)-1 {
					m.inputScreen.historyIndex++
				}
				if m.inputScreen.historyIndex >= 0 && m.inputScreen.historyIndex < len(m.inputScreen.history) {
					m.inputScreen.input.SetValue(m.inputScreen.history[m.inputScreen.historyIndex])
					m.inputScreen.input.CursorEnd()
				}
				return m, nil
			case "down":
				// Go to next command in history (newer)
				if m.inputScreen.historyIndex > 0 {
					m.inputScreen.historyIndex--
					m.inputScreen.input.SetValue(m.inputScreen.history[m.inputScreen.historyIndex])
					m.inputScreen.input.CursorEnd()
				} else if m.inputScreen.historyIndex == 0 {
					m.inputScreen.historyIndex = -1
					m.inputScreen.input.SetValue(m.inputScreen.originalInput)
					m.inputScreen.input.CursorEnd()
				}
				return m, nil
			}
		}

		// Reset history browsing when user types
		if msg.Type == tea.KeyRunes || msg.Type == tea.KeyBackspace || msg.Type == tea.KeyDelete {
			m.inputScreen.historyIndex = -1
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
			m.welcomeScreen = NewWelcomeScreen(cwd, m.getRepoWorktreeDir(), m.theme)
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
		m.showInfo(fmt.Sprintf("Failed to create cache dir: %v", err), nil)
		return
	}

	cacheData := struct {
		Worktrees []*models.WorktreeInfo `json:"worktrees"`
	}{
		Worktrees: m.worktrees,
	}
	data, _ := json.Marshal(cacheData)
	if err := os.WriteFile(cachePath, data, defaultFilePerms); err != nil {
		m.showInfo(fmt.Sprintf("Failed to write cache: %v", err), nil)
	}
}

func (m *Model) loadCommandHistory() {
	repoKey := m.getRepoKey()
	historyPath := filepath.Join(m.getWorktreeDir(), repoKey, models.CommandHistoryFilename)
	// #nosec G304 -- historyPath is constructed from vetted worktree directory and constant filename
	data, err := os.ReadFile(historyPath)
	if err != nil {
		// No history file yet, that's fine
		m.commandHistory = []string{}
		return
	}

	var payload struct {
		Commands []string `json:"commands"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		m.debugf("failed to parse command history: %v", err)
		m.commandHistory = []string{}
		return
	}

	m.commandHistory = payload.Commands
	if m.commandHistory == nil {
		m.commandHistory = []string{}
	}
}

func (m *Model) saveCommandHistory() {
	repoKey := m.getRepoKey()
	historyPath := filepath.Join(m.getWorktreeDir(), repoKey, models.CommandHistoryFilename)
	if err := os.MkdirAll(filepath.Dir(historyPath), defaultDirPerms); err != nil {
		m.debugf("failed to create history dir: %v", err)
		return
	}

	historyData := struct {
		Commands []string `json:"commands"`
	}{
		Commands: m.commandHistory,
	}
	data, _ := json.Marshal(historyData)
	if err := os.WriteFile(historyPath, data, defaultFilePerms); err != nil {
		m.debugf("failed to write command history: %v", err)
	}
}

func (m *Model) addToCommandHistory(cmd string) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return
	}

	// Remove duplicate if it exists
	filtered := []string{}
	for _, c := range m.commandHistory {
		if c != cmd {
			filtered = append(filtered, c)
		}
	}

	// Add to front (most recent first)
	m.commandHistory = append([]string{cmd}, filtered...)

	// Limit history to 100 entries
	maxHistory := 100
	if len(m.commandHistory) > maxHistory {
		m.commandHistory = m.commandHistory[:maxHistory]
	}

	m.saveCommandHistory()
}

func (m *Model) loadAccessHistory() {
	repoKey := m.getRepoKey()
	historyPath := filepath.Join(m.getWorktreeDir(), repoKey, models.AccessHistoryFilename)
	// #nosec G304 -- path is constructed from known safe components
	data, err := os.ReadFile(historyPath)
	if err != nil {
		return
	}
	var history map[string]int64
	if err := json.Unmarshal(data, &history); err != nil {
		m.debugf("failed to parse access history: %v", err)
		return
	}
	m.accessHistory = history
}

func (m *Model) saveAccessHistory() {
	repoKey := m.getRepoKey()
	historyPath := filepath.Join(m.getWorktreeDir(), repoKey, models.AccessHistoryFilename)
	if err := os.MkdirAll(filepath.Dir(historyPath), defaultDirPerms); err != nil {
		m.debugf("failed to create access history dir: %v", err)
		return
	}
	data, _ := json.Marshal(m.accessHistory)
	if err := os.WriteFile(historyPath, data, defaultFilePerms); err != nil {
		m.debugf("failed to write access history: %v", err)
	}
}

func (m *Model) recordAccess(path string) {
	if path == "" {
		return
	}
	m.accessHistory[path] = time.Now().Unix()
	m.saveAccessHistory()
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

func (m *Model) getCachedDetails(wt *models.WorktreeInfo) (string, string, map[string]bool, map[string]bool) {
	if wt == nil || strings.TrimSpace(wt.Path) == "" {
		return "", "", nil, nil
	}

	cacheKey := wt.Path
	if cached, ok := m.detailsCache[cacheKey]; ok {
		if time.Since(cached.fetchedAt) < detailsCacheTTL {
			return cached.statusRaw, cached.logRaw, cached.unpushedSHAs, cached.unmergedSHAs
		}
	}

	// Get status (using porcelain format for reliable machine parsing)
	statusRaw := m.git.RunGit(m.ctx, []string{"git", "status", "--porcelain=v2"}, wt.Path, []int{0}, true, false)
	// Use %H for full SHA to ensure reliable matching
	logRaw := m.git.RunGit(m.ctx, []string{"git", "log", "-50", "--pretty=format:%H%x09%an%x09%s"}, wt.Path, []int{0}, true, false)

	// Get unpushed SHAs (commits not on any remote)
	unpushedRaw := m.git.RunGit(m.ctx, []string{"git", "rev-list", "-100", "HEAD", "--not", "--remotes"}, wt.Path, []int{0}, true, false)
	unpushedSHAs := make(map[string]bool)
	for _, sha := range strings.Split(unpushedRaw, "\n") {
		if s := strings.TrimSpace(sha); s != "" {
			unpushedSHAs[s] = true
		}
	}

	// Get unmerged SHAs (commits not in main branch)
	mainBranch := m.git.GetMainBranch(m.ctx)
	unmergedSHAs := make(map[string]bool)
	if mainBranch != "" {
		unmergedRaw := m.git.RunGit(m.ctx, []string{"git", "rev-list", "-100", "HEAD", "^" + mainBranch}, wt.Path, []int{0}, true, false)
		for _, sha := range strings.Split(unmergedRaw, "\n") {
			if s := strings.TrimSpace(sha); s != "" {
				unmergedSHAs[s] = true
			}
		}
	}

	m.detailsCache[cacheKey] = &detailsCacheEntry{
		statusRaw:    statusRaw,
		logRaw:       logRaw,
		unpushedSHAs: unpushedSHAs,
		unmergedSHAs: unmergedSHAs,
		fetchedAt:    time.Now(),
	}

	return statusRaw, logRaw, unpushedSHAs, unmergedSHAs
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

func (m *Model) getRepoWorktreeDir() string {
	return filepath.Join(m.getWorktreeDir(), m.getRepoKey())
}

func (m *Model) pagerCommand() string {
	if m.config != nil {
		if pager := strings.TrimSpace(m.config.Pager); pager != "" {
			return pager
		}
	}
	if pager := strings.TrimSpace(os.Getenv("PAGER")); pager != "" {
		return pager
	}
	if _, err := exec.LookPath("less"); err == nil {
		return "less --use-color -z-4 -q --wordwrap -qcR -P 'Press q to exit..'"
	}
	if _, err := exec.LookPath("more"); err == nil {
		return "more"
	}
	return "cat"
}

func (m *Model) editorCommand() string {
	if m.config != nil {
		if editor := strings.TrimSpace(m.config.Editor); editor != "" {
			return os.ExpandEnv(editor)
		}
	}
	if editor := strings.TrimSpace(os.Getenv("EDITOR")); editor != "" {
		return editor
	}
	if _, err := exec.LookPath("nvim"); err == nil {
		return "nvim"
	}
	if _, err := exec.LookPath("vi"); err == nil {
		return "vi"
	}
	return ""
}

func (m *Model) pagerEnv(pager string) string {
	if pagerIsLess(pager) {
		return "LESS= LESSHISTFILE=-"
	}
	return ""
}

func pagerIsLess(pager string) bool {
	fields := strings.FieldsSeq(pager)
	for field := range fields {
		if strings.Contains(field, "=") && !strings.HasPrefix(field, "-") && !strings.Contains(field, "/") {
			continue
		}
		return filepath.Base(field) == "less"
	}
	return false
}

// GetSelectedPath returns the selected worktree path for shell integration.
// This is used when the application exits to allow the shell to cd into the selected worktree.
func (m *Model) GetSelectedPath() string {
	return m.selectedPath
}

func (m *Model) showInfo(message string, action tea.Cmd) {
	m.infoScreen = NewInfoScreen(message, m.theme)
	m.infoAction = action
	m.currentScreen = screenInfo
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
	m.recordAccess(path)
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
	case onExistsAttach, onExistsKill, onExistsNew, onExistsSwitch:
	default:
		onExists = onExistsSwitch
	}

	var b strings.Builder
	b.WriteString("set -e\n")
	b.WriteString(fmt.Sprintf("session=%s\n", shellQuote(sessionName)))
	b.WriteString("base_session=$session\n")
	b.WriteString("if tmux has-session -t \"$session\" 2>/dev/null; then\n")
	switch onExists {
	case onExistsKill:
		b.WriteString("  tmux kill-session -t \"$session\"\n")
	case onExistsNew:
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
		if onExists == onExistsAttach {
			b.WriteString("tmux attach -t \"$session\" || true\n")
		} else {
			b.WriteString("if [ -n \"$TMUX\" ]; then tmux switch-client -t \"$session\" || true; else tmux attach -t \"$session\" || true; fi\n")
		}
	}
	return b.String()
}

func buildZellijScript(sessionName string, zellijCfg *config.TmuxCommand, layoutPaths []string) string {
	onExists := strings.ToLower(strings.TrimSpace(zellijCfg.OnExists))
	switch onExists {
	case onExistsAttach, onExistsKill, onExistsNew, onExistsSwitch:
	default:
		onExists = onExistsSwitch
	}

	var b strings.Builder
	b.WriteString("set -e\n")
	b.WriteString(fmt.Sprintf("session=%s\n", shellQuote(sessionName)))
	b.WriteString("base_session=$session\n")
	b.WriteString("session_exists() {\n")
	b.WriteString("  zellij list-sessions --short --no-formatting 2>/dev/null | grep -Fxq \"$1\"\n")
	b.WriteString("}\n")
	b.WriteString("created=false\n")
	b.WriteString("if session_exists \"$session\"; then\n")
	switch onExists {
	case onExistsKill:
		b.WriteString("  zellij kill-session \"$session\"\n")
	case onExistsNew:
		b.WriteString("  i=2\n")
		b.WriteString("  while session_exists \"${base_session}-$i\"; do i=$((i+1)); done\n")
		b.WriteString("  session=\"${base_session}-$i\"\n")
	default:
		b.WriteString("  :\n")
	}
	b.WriteString("fi\n")
	b.WriteString("if ! session_exists \"$session\"; then\n")
	b.WriteString("  zellij attach --create-background \"$session\"\n")
	b.WriteString("  created=true\n")
	b.WriteString("fi\n")
	b.WriteString("if [ \"$created\" = \"true\" ]; then\n")
	for _, layoutPath := range layoutPaths {
		b.WriteString(fmt.Sprintf("  ZELLIJ_SESSION_NAME=\"$session\" zellij action new-tab --layout %s\n", shellQuote(layoutPath)))
	}
	b.WriteString("  ZELLIJ_SESSION_NAME=\"$session\" zellij action go-to-tab 1\n")
	b.WriteString("  ZELLIJ_SESSION_NAME=\"$session\" zellij action close-tab\n")
	b.WriteString("fi\n")
	b.WriteString("if [ -n \"${LW_ZELLIJ_SESSION_FILE:-}\" ]; then printf '%s' \"$session\" > \"$LW_ZELLIJ_SESSION_FILE\"; fi\n")
	return b.String()
}

func buildZellijTabLayout(window resolvedTmuxWindow) string {
	var b strings.Builder
	b.WriteString("layout {\n")
	b.WriteString(fmt.Sprintf("    tab name=%s {\n", kdlQuote(window.Name)))
	b.WriteString("        pane {\n")
	if window.Cwd != "" {
		b.WriteString(fmt.Sprintf("            cwd %s\n", kdlQuote(window.Cwd)))
	}
	b.WriteString(fmt.Sprintf("            command %s\n", kdlQuote("bash")))
	b.WriteString(fmt.Sprintf("            args %s %s\n", kdlQuote("-lc"), kdlQuote(window.Command)))
	b.WriteString("        }\n")
	b.WriteString("    }\n")
	b.WriteString("}\n")
	return b.String()
}

func writeZellijLayouts(windows []resolvedTmuxWindow) ([]string, error) {
	paths := make([]string, 0, len(windows))
	for _, window := range windows {
		layoutFile, err := os.CreateTemp("", "lazyworktree-zellij-layout-")
		if err != nil {
			cleanupZellijLayouts(paths)
			return nil, err
		}
		if _, err := layoutFile.WriteString(buildZellijTabLayout(window)); err != nil {
			_ = layoutFile.Close()
			_ = os.Remove(layoutFile.Name())
			cleanupZellijLayouts(paths)
			return nil, err
		}
		if err := layoutFile.Close(); err != nil {
			_ = os.Remove(layoutFile.Name())
			cleanupZellijLayouts(paths)
			return nil, err
		}
		paths = append(paths, layoutFile.Name())
	}
	return paths, nil
}

func cleanupZellijLayouts(paths []string) {
	for _, path := range paths {
		_ = os.Remove(path)
	}
}

func buildTmuxInfoMessage(sessionName string, insideTmux bool) string {
	quoted := shellQuote(sessionName)
	if insideTmux {
		return fmt.Sprintf("tmux session ready.\n\nSwitch with:\n\n  tmux switch-client -t %s", quoted)
	}
	return fmt.Sprintf("tmux session ready.\n\nAttach with:\n\n  tmux attach-session -t %s", quoted)
}

func buildZellijInfoMessage(sessionName string) string {
	quoted := shellQuote(sessionName)
	return fmt.Sprintf("zellij session ready.\n\nAttach with:\n\n  zellij attach %s", quoted)
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

func (m *Model) attachZellijSessionCmd(sessionName string) tea.Cmd {
	// #nosec G204 -- zellij session name comes from user configuration.
	c := m.commandRunner("zellij", onExistsAttach, sessionName)
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

func kdlQuote(input string) string {
	escaped := strings.ReplaceAll(input, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
	return "\"" + escaped + "\""
}

func sanitizeZellijSessionName(name string) string {
	if name == "" {
		return ""
	}
	replacer := strings.NewReplacer("/", "-", "\\", "-")
	return replacer.Replace(name)
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
		m.showInfo(fmt.Sprintf("Failed to load .wt: %v", err), nil)
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
	if m.showingFilter || m.showingSearch {
		filterHeight = 1
	}
	gapX := 1
	gapY := 1

	bodyHeight := max(height-headerHeight-footerHeight-filterHeight, 8)

	// Handle zoom mode: zoomed pane gets full body area
	if m.zoomedPane >= 0 {
		paneFrameX := m.basePaneStyle().GetHorizontalFrameSize()
		paneFrameY := m.basePaneStyle().GetVerticalFrameSize()
		fullWidth := width
		fullInnerWidth := maxInt(1, fullWidth-paneFrameX)
		fullInnerHeight := maxInt(1, bodyHeight-paneFrameY)

		return layoutDims{
			width:                  width,
			height:                 height,
			headerHeight:           headerHeight,
			footerHeight:           footerHeight,
			filterHeight:           filterHeight,
			bodyHeight:             bodyHeight,
			gapX:                   0,
			gapY:                   0,
			leftWidth:              fullWidth,
			rightWidth:             fullWidth,
			leftInnerWidth:         fullInnerWidth,
			rightInnerWidth:        fullInnerWidth,
			leftInnerHeight:        fullInnerHeight,
			rightTopHeight:         bodyHeight,
			rightBottomHeight:      bodyHeight,
			rightTopInnerHeight:    fullInnerHeight,
			rightBottomInnerHeight: fullInnerHeight,
		}
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
	case 1: // Status focused → give more height to top pane
		topRatio = 0.82
	case 2: // Log focused → give more height to bottom pane
		topRatio = 0.30
	}

	rightTopHeight := max(int(float64(bodyHeight-gapY)*topRatio), 6)
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
	// Minimum height of 3 is required to prevent viewport slice bounds panic
	tableHeight := maxInt(3, layout.leftInnerHeight-titleHeight-tableHeaderHeight-2)
	m.worktreeTable.SetWidth(layout.leftInnerWidth)
	m.worktreeTable.SetHeight(tableHeight)
	m.updateTableColumns(layout.leftInnerWidth)

	logHeight := maxInt(3, layout.rightBottomInnerHeight-titleHeight-tableHeaderHeight-2)
	m.logTable.SetWidth(layout.rightInnerWidth)
	m.logTable.SetHeight(logHeight)
	m.updateLogColumns(layout.rightInnerWidth)

	m.filterInput.Width = maxInt(20, layout.width-18)
}

func (m *Model) renderHeader(layout layoutDims) string {
	// Create a "toolbar" style header with visual flair
	headerStyle := lipgloss.NewStyle().
		Background(m.theme.Accent).
		Foreground(lipgloss.Color("#FFFFFF")). // Pure white for better contrast
		Bold(true).
		Width(layout.width).
		Padding(0, 2) // Increased padding for breathing room

	// Add decorative icon to title
	title := "🌲 Lazy Worktree Manager"
	repoKey := strings.TrimSpace(m.repoKey)
	content := title
	if repoKey != "" && repoKey != "unknown" && !strings.HasPrefix(repoKey, "local-") {
		content = fmt.Sprintf("%s  •  %s", content, repoKey)
	}

	return headerStyle.Render(content)
}

func (m *Model) renderFilter(layout layoutDims) string {
	// Enhanced filter bar with visual flair
	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(m.theme.Accent).
		Bold(true).
		Padding(0, 1) // Pill effect
	filterStyle := lipgloss.NewStyle().
		Foreground(m.theme.TextFg).
		Padding(0, 1)
	line := fmt.Sprintf("%s %s", labelStyle.Render(m.inputLabel()), m.filterInput.View())
	return filterStyle.Width(layout.width).Render(line)
}

func (m *Model) renderBody(layout layoutDims) string {
	// Handle zoom mode: only render the zoomed pane
	if m.zoomedPane >= 0 {
		switch m.zoomedPane {
		case 0:
			return m.renderZoomedLeftPane(layout)
		case 1:
			return m.renderZoomedRightTopPane(layout)
		case 2:
			return m.renderZoomedRightBottomPane(layout)
		}
	}

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
	title := m.renderPaneTitle(2, "Status", m.focusedPane == 1, layout.rightInnerWidth)
	infoBox := m.renderInnerBox("Info", m.infoContent, layout.rightInnerWidth, 0)

	innerBoxStyle := m.baseInnerBoxStyle()
	statusBoxHeight := max(layout.rightTopInnerHeight-lipgloss.Height(title)-lipgloss.Height(infoBox)-2, 3)
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

func (m *Model) renderZoomedLeftPane(layout layoutDims) string {
	title := m.renderPaneTitle(1, "Worktrees", true, layout.leftInnerWidth)
	tableView := m.worktreeTable.View()
	content := lipgloss.JoinVertical(lipgloss.Left, title, tableView)
	return m.paneStyle(true).
		Width(layout.leftWidth).
		Height(layout.bodyHeight).
		Render(content)
}

func (m *Model) renderZoomedRightTopPane(layout layoutDims) string {
	title := m.renderPaneTitle(2, "Status", true, layout.rightInnerWidth)
	infoBox := m.renderInnerBox("Info", m.infoContent, layout.rightInnerWidth, 0)

	innerBoxStyle := m.baseInnerBoxStyle()
	statusBoxHeight := max(layout.rightTopInnerHeight-lipgloss.Height(title)-lipgloss.Height(infoBox)-2, 3)
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
	return m.paneStyle(true).
		Width(layout.rightWidth).
		Height(layout.bodyHeight).
		Render(content)
}

func (m *Model) renderZoomedRightBottomPane(layout layoutDims) string {
	title := m.renderPaneTitle(3, "Log", true, layout.rightInnerWidth)
	content := lipgloss.JoinVertical(lipgloss.Left, title, m.logTable.View())
	return m.paneStyle(true).
		Width(layout.rightWidth).
		Height(layout.bodyHeight).
		Render(content)
}

func (m *Model) renderFooter(layout layoutDims) string {
	footerStyle := lipgloss.NewStyle().
		Foreground(m.theme.TextFg).
		Background(m.theme.BorderDim).
		Padding(0, 1)

	// Context-aware hints based on focused pane
	var hints []string

	switch m.focusedPane {
	case 2: // Log pane
		if len(m.logEntries) > 0 {
			hints = []string{
				m.renderKeyHint("Enter", "View Commit"),
				m.renderKeyHint("C", "Cherry-pick"),
				m.renderKeyHint("j/k", "Navigate"),
				m.renderKeyHint("f", "Filter"),
				m.renderKeyHint("/", "Search"),
				m.renderKeyHint("r", "Refresh"),
				m.renderKeyHint("Tab", "Switch Pane"),
				m.renderKeyHint("q", "Quit"),
				m.renderKeyHint("?", "Help"),
			}
		} else {
			hints = []string{
				m.renderKeyHint("f", "Filter"),
				m.renderKeyHint("/", "Search"),
				m.renderKeyHint("Tab", "Switch Pane"),
				m.renderKeyHint("q", "Quit"),
				m.renderKeyHint("?", "Help"),
			}
		}

	case 1: // Status pane
		hints = []string{
			m.renderKeyHint("j/k", "Scroll"),
		}
		if len(m.statusFiles) > 0 {
			hints = append(hints,
				m.renderKeyHint("Enter", "Show Diff"),
				m.renderKeyHint("e", "Edit File"),
			)
		}
		hints = append(hints,
			m.renderKeyHint("f", "Filter"),
			m.renderKeyHint("/", "Search"),
			m.renderKeyHint("Tab", "Switch Pane"),
			m.renderKeyHint("r", "Refresh"),
			m.renderKeyHint("q", "Quit"),
			m.renderKeyHint("?", "Help"),
		)

	default: // Worktree table (pane 0)
		sortName := "Path"
		switch m.sortMode {
		case sortModeLastActive:
			sortName = "Active"
		case sortModeLastSwitched:
			sortName = "Switched"
		}
		hints = []string{
			m.renderKeyHint("1-3", "Pane"),
			m.renderKeyHint("f", "Filter"),
			m.renderKeyHint("/", "Search"),
			m.renderKeyHint("s", sortName),
			m.renderKeyHint("d", "Diff"),
			m.renderKeyHint("D", "Delete"),
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
			m.renderKeyHint("q", "Quit"),
			m.renderKeyHint("?", "Help"),
			m.renderKeyHint("ctrl+p", "Palette"),
		)
	}

	footerContent := strings.Join(hints, "  ")
	if !m.loading {
		return footerStyle.Width(layout.width).Render(footerContent)
	}
	spinnerView := m.spinner.View()
	gap := "  "
	available := max(layout.width-lipgloss.Width(spinnerView)-lipgloss.Width(gap), 0)
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
	// Enhanced key hints with pill/badge styling
	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#000000")).
		Background(m.theme.Accent).
		Bold(true).
		Padding(0, 1) // Add padding for pill effect
	labelStyle := lipgloss.NewStyle().Foreground(m.theme.MutedFg)
	return fmt.Sprintf("%s %s", keyStyle.Render(key), labelStyle.Render(label))
}

func (m *Model) renderPaneTitle(index int, title string, focused bool, width int) string {
	numStyle := lipgloss.NewStyle().Foreground(m.theme.MutedFg)
	titleStyle := lipgloss.NewStyle().Foreground(m.theme.MutedFg)
	indicator := "○" // Hollow circle for unfocused
	if focused {
		numStyle = numStyle.Foreground(m.theme.Accent).Bold(true)
		titleStyle = titleStyle.Foreground(m.theme.TextFg).Bold(true)
		indicator = symbolFilledCircle // Filled circle for focused
	}
	num := numStyle.Render(fmt.Sprintf("[%d]", index))
	focusIndicator := numStyle.Render(indicator)
	name := titleStyle.Render(title)

	filterIndicator := ""
	paneIdx := index - 1 // index is 1-based, panes are 0-based
	if !m.showingFilter && !m.showingSearch && m.hasActiveFilterForPane(paneIdx) {
		filteredStyle := lipgloss.NewStyle().Foreground(m.theme.WarnFg).Italic(true)
		keyStyle := lipgloss.NewStyle().
			Foreground(m.theme.AccentFg).
			Background(m.theme.Accent).
			Bold(true).
			Padding(0, 1)
		filterIndicator = fmt.Sprintf(" 🔍 %s  %s %s",
			filteredStyle.Render("Filtered"),
			keyStyle.Render("Esc"),
			lipgloss.NewStyle().Foreground(m.theme.MutedFg).Render("Clear"))
	}

	zoomIndicator := ""
	if m.zoomedPane == paneIdx {
		zoomedStyle := lipgloss.NewStyle().Foreground(m.theme.Accent).Italic(true)
		keyStyle := lipgloss.NewStyle().
			Foreground(m.theme.AccentFg).
			Background(m.theme.Accent).
			Bold(true).
			Padding(0, 1)
		zoomIndicator = fmt.Sprintf(" %s %s  %s %s",
			"🔎",
			zoomedStyle.Render("Zoomed"),
			keyStyle.Render("="),
			lipgloss.NewStyle().Foreground(m.theme.MutedFg).Render("Unzoom"))
	}

	return lipgloss.NewStyle().Width(width).Render(fmt.Sprintf("%s %s %s%s%s", focusIndicator, num, name, filterIndicator, zoomIndicator))
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
	if wt.LastSwitchedTS > 0 {
		accessTime := time.Unix(wt.LastSwitchedTS, 0)
		relTime := formatRelativeTime(accessTime)
		infoLines = append(infoLines, fmt.Sprintf("%s %s", labelStyle.Render("Last Accessed:"), valueStyle.Render(relTime)))
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
		prPrefix := "PR:"
		if m.config.ShowIcons {
			prPrefix = iconWithSpace(iconPR) + prPrefix
		}
		prLabel := prLabelStyle.Render(prPrefix)
		whiteStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("15")) // white
		stateColor := lipgloss.Color("2")                                  // green for OPEN
		switch wt.PR.State {
		case "MERGED":
			stateColor = lipgloss.Color("5") // magenta
		case "CLOSED":
			stateColor = lipgloss.Color("1") // red
		}
		stateStyle := lipgloss.NewStyle().Foreground(stateColor)
		// Format: PR: #123 Title [STATE] (matches Python grid layout)
		infoLines = append(infoLines, fmt.Sprintf("%s %s %s [%s]",
			prLabel,
			whiteStyle.Render(fmt.Sprintf("#%d", wt.PR.Number)),
			wt.PR.Title,
			stateStyle.Render(wt.PR.State)))
		// Author line with bot indicator if applicable
		if wt.PR.Author != "" {
			grayStyle := lipgloss.NewStyle().Foreground(m.theme.MutedFg)
			var authorText string
			if wt.PR.AuthorName != "" {
				authorText = fmt.Sprintf("%s (@%s)", wt.PR.AuthorName, wt.PR.Author)
			} else {
				authorText = wt.PR.Author
			}
			if wt.PR.AuthorIsBot {
				authorText = "🤖 " + authorText
			}
			infoLines = append(infoLines, fmt.Sprintf("     by %s", grayStyle.Render(authorText)))
		}
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
				var symbol string
				var style lipgloss.Style
				switch check.Conclusion {
				case "success":
					symbol = "✓"
					style = greenStyle
				case "failure":
					symbol = "✗"
					style = redStyle
				case "skipped":
					symbol = "○"
					style = grayStyle
				case "cancelled":
					symbol = "⊘"
					style = grayStyle
				case "pending", "":
					symbol = symbolFilledCircle
					style = yellowStyle
				default:
					symbol = "?"
					style = grayStyle
				}
				if m.config.ShowIcons {
					if icon := ciIconForConclusion(check.Conclusion); icon != "" {
						symbol = icon
					}
				}
				infoLines = append(infoLines, fmt.Sprintf("  %s %s", style.Render(symbol), check.Name))
			}
		}
	}
	return strings.Join(infoLines, "\n")
}

func parseStatusFiles(statusRaw string) []StatusFile {
	statusRaw = strings.TrimRight(statusRaw, "\n")
	if strings.TrimSpace(statusRaw) == "" {
		return nil
	}

	// Parse all files into statusFiles
	statusLines := strings.Split(statusRaw, "\n")
	parsedFiles := make([]StatusFile, 0, len(statusLines))
	for _, line := range statusLines {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse git status --porcelain=v2 format
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		var status, filename string
		var isUntracked bool

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
			isUntracked = true
		case "2": // Renamed/copied: 2 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <X><score> <path><sep><origPath>
			if len(fields) < 10 {
				continue
			}
			status = fields[1]
			filename = fields[9]
		default:
			continue // Skip unhandled entry types
		}

		parsedFiles = append(parsedFiles, StatusFile{
			Filename:    filename,
			Status:      status,
			IsUntracked: isUntracked,
		})
	}

	return parsedFiles
}

// buildStatusTree builds a tree structure from a flat list of files.
// Files are grouped by directory, with directories sorted before files.
func buildStatusTree(files []StatusFile) *StatusTreeNode {
	if len(files) == 0 {
		return &StatusTreeNode{Path: "", Children: nil}
	}

	root := &StatusTreeNode{Path: "", Children: make([]*StatusTreeNode, 0)}
	childrenByPath := make(map[string]*StatusTreeNode)

	for i := range files {
		file := &files[i]
		parts := strings.Split(file.Filename, "/")

		current := root
		for j := 0; j < len(parts); j++ {
			isFile := j == len(parts)-1
			pathSoFar := strings.Join(parts[:j+1], "/")

			if existing, ok := childrenByPath[pathSoFar]; ok {
				current = existing
				continue
			}

			var newNode *StatusTreeNode
			if isFile {
				newNode = &StatusTreeNode{
					Path: pathSoFar,
					File: file,
				}
			} else {
				newNode = &StatusTreeNode{
					Path:     pathSoFar,
					Children: make([]*StatusTreeNode, 0),
				}
			}
			current.Children = append(current.Children, newNode)
			childrenByPath[pathSoFar] = newNode
			current = newNode
		}
	}

	sortStatusTree(root)
	compressStatusTree(root)
	return root
}

// sortStatusTree sorts tree nodes: directories first, then alphabetically.
func sortStatusTree(node *StatusTreeNode) {
	if node == nil || node.Children == nil {
		return
	}

	sort.Slice(node.Children, func(i, j int) bool {
		iIsDir := node.Children[i].File == nil
		jIsDir := node.Children[j].File == nil
		if iIsDir != jIsDir {
			return iIsDir // directories first
		}
		return node.Children[i].Path < node.Children[j].Path
	})

	for _, child := range node.Children {
		sortStatusTree(child)
	}
}

// compressStatusTree squashes single-child directory chains (e.g., a/b/c becomes one node).
func compressStatusTree(node *StatusTreeNode) {
	if node == nil {
		return
	}

	for _, child := range node.Children {
		compressStatusTree(child)
	}

	// Compress children that are single-child directories
	for i, child := range node.Children {
		for child.File == nil && len(child.Children) == 1 && child.Children[0].File == nil {
			grandchild := child.Children[0]
			grandchild.Compression = child.Compression + 1
			node.Children[i] = grandchild
			child = grandchild
		}
	}
}

// flattenStatusTree returns visible nodes respecting collapsed state.
func flattenStatusTree(node *StatusTreeNode, collapsed map[string]bool, depth int) []*StatusTreeNode {
	if node == nil {
		return nil
	}

	result := make([]*StatusTreeNode, 0)

	// Skip root node itself but process its children
	if node.Path != "" {
		nodeCopy := *node
		nodeCopy.depth = depth
		result = append(result, &nodeCopy)

		// If collapsed, don't include children
		if collapsed[node.Path] {
			return result
		}
	}

	if node.Children != nil {
		childDepth := depth
		if node.Path != "" {
			childDepth = depth + 1
		}
		for _, child := range node.Children {
			result = append(result, flattenStatusTree(child, collapsed, childDepth)...)
		}
	}

	return result
}

// IsDir returns true if this node is a directory.
func (n *StatusTreeNode) IsDir() bool {
	return n.File == nil
}

// Name returns the display name for this node.
func (n *StatusTreeNode) Name() string {
	return filepath.Base(n.Path)
}

// CollectFiles recursively collects all StatusFile pointers from this node and its children.
func (n *StatusTreeNode) CollectFiles() []*StatusFile {
	var files []*StatusFile
	if n.File != nil {
		files = append(files, n.File)
	}
	for _, child := range n.Children {
		files = append(files, child.CollectFiles()...)
	}
	return files
}

func (m *Model) buildStatusContent(statusRaw string) string {
	m.setStatusFiles(parseStatusFiles(statusRaw))
	return m.statusContent
}

func (m *Model) setStatusFiles(files []StatusFile) {
	m.statusFilesAll = files

	// Initialize collapsed dirs map if needed
	if m.statusCollapsedDirs == nil {
		m.statusCollapsedDirs = make(map[string]bool)
	}

	m.applyStatusFilter()
}

func (m *Model) applyStatusFilter() {
	query := strings.ToLower(strings.TrimSpace(m.statusFilterQuery))
	filtered := m.statusFilesAll
	if query != "" {
		filtered = make([]StatusFile, 0, len(m.statusFilesAll))
		for _, sf := range m.statusFilesAll {
			if strings.Contains(strings.ToLower(sf.Filename), query) {
				filtered = append(filtered, sf)
			}
		}
	}

	// Remember current selection (by path)
	selectedPath := ""
	if m.statusTreeIndex >= 0 && m.statusTreeIndex < len(m.statusTreeFlat) {
		selectedPath = m.statusTreeFlat[m.statusTreeIndex].Path
	}

	// Keep statusFiles for compatibility
	m.statusFiles = filtered

	// Build tree from filtered files
	m.statusTree = buildStatusTree(filtered)
	m.rebuildStatusTreeFlat()

	// Try to restore selection
	if selectedPath != "" {
		for i, node := range m.statusTreeFlat {
			if node.Path == selectedPath {
				m.statusTreeIndex = i
				break
			}
		}
	}

	// Clamp tree index
	if m.statusTreeIndex < 0 {
		m.statusTreeIndex = 0
	}
	if len(m.statusTreeFlat) > 0 && m.statusTreeIndex >= len(m.statusTreeFlat) {
		m.statusTreeIndex = len(m.statusTreeFlat) - 1
	}
	if len(m.statusTreeFlat) == 0 {
		m.statusTreeIndex = 0
	}

	// Keep old statusFileIndex in sync for compatibility
	m.statusFileIndex = m.statusTreeIndex

	m.rebuildStatusContentWithHighlight()
}

// rebuildStatusTreeFlat rebuilds the flattened tree view from the tree structure.
func (m *Model) rebuildStatusTreeFlat() {
	if m.statusCollapsedDirs == nil {
		m.statusCollapsedDirs = make(map[string]bool)
	}
	m.statusTreeFlat = flattenStatusTree(m.statusTree, m.statusCollapsedDirs, 0)
}

func formatCommitMessage(message string) string {
	if len(message) <= commitMessageMaxLength {
		return message
	}
	return message[:commitMessageMaxLength] + "…"
}

func authorInitials(name string) string {
	fields := strings.Fields(name)
	if len(fields) == 0 {
		return ""
	}
	if len(fields) == 1 {
		runes := []rune(fields[0])
		if len(runes) <= 2 {
			return string(runes)
		}
		return string(runes[:2])
	}
	first := []rune(fields[0])
	last := []rune(fields[len(fields)-1])
	if len(first) == 0 || len(last) == 0 {
		return ""
	}
	return string([]rune{first[0], last[0]})
}

func (m *Model) setLogEntries(entries []commitLogEntry, reset bool) {
	m.logEntriesAll = entries
	m.applyLogFilter(reset)
}

func (m *Model) applyLogFilter(reset bool) {
	query := strings.ToLower(strings.TrimSpace(m.logFilterQuery))
	filtered := m.logEntriesAll
	if query != "" {
		filtered = make([]commitLogEntry, 0, len(m.logEntriesAll))
		for _, entry := range m.logEntriesAll {
			if strings.Contains(strings.ToLower(entry.message), query) {
				filtered = append(filtered, entry)
			}
		}
	}

	selectedSHA := ""
	if !reset {
		cursor := m.logTable.Cursor()
		if cursor >= 0 && cursor < len(m.logEntries) {
			selectedSHA = m.logEntries[cursor].sha
		}
	}

	m.logEntries = filtered
	rows := make([]table.Row, 0, len(filtered))
	for _, entry := range filtered {
		sha := entry.sha
		if len(sha) > 7 {
			sha = sha[:7]
		}
		msg := formatCommitMessage(entry.message)
		if entry.isUnpushed {
			msg = lipgloss.NewStyle().Foreground(m.theme.WarnFg).Render("⬆ " + msg)
		} else if entry.isUnmerged {
			msg = lipgloss.NewStyle().Foreground(m.theme.Accent).Render(msg)
		}
		rows = append(rows, table.Row{sha, entry.authorInitials, msg})
	}
	m.logTable.SetRows(rows)

	if selectedSHA != "" {
		for i, entry := range m.logEntries {
			if entry.sha == selectedSHA {
				m.logTable.SetCursor(i)
				return
			}
		}
	}
	if len(m.logEntries) > 0 {
		if m.logTable.Cursor() < 0 || m.logTable.Cursor() >= len(m.logEntries) || reset {
			m.logTable.SetCursor(0)
		}
	} else {
		m.logTable.SetCursor(0)
	}
}

func findMatchIndex(count, start int, forward bool, matches func(int) bool) int {
	if count == 0 {
		return -1
	}
	if start < 0 {
		if forward {
			start = 0
		} else {
			start = count - 1
		}
	} else if count > 0 {
		start %= count
	}
	for i := 0; i < count; i++ {
		var idx int
		if forward {
			idx = (start + i) % count
		} else {
			idx = (start - i + count) % count
		}
		if matches(idx) {
			return idx
		}
	}
	return -1
}

func (m *Model) findWorktreeMatchIndex(query string, start int, forward bool) int {
	lowerQuery := strings.ToLower(strings.TrimSpace(query))
	if lowerQuery == "" {
		return -1
	}
	hasPathSep := strings.Contains(lowerQuery, "/")
	return findMatchIndex(len(m.filteredWts), start, forward, func(i int) bool {
		wt := m.filteredWts[i]
		name := filepath.Base(wt.Path)
		if wt.IsMain {
			name = mainWorktreeName
		}
		if strings.Contains(strings.ToLower(name), lowerQuery) {
			return true
		}
		if strings.Contains(strings.ToLower(wt.Branch), lowerQuery) {
			return true
		}
		return hasPathSep && strings.Contains(strings.ToLower(wt.Path), lowerQuery)
	})
}

func (m *Model) findStatusMatchIndex(query string, start int, forward bool) int {
	lowerQuery := strings.ToLower(strings.TrimSpace(query))
	if lowerQuery == "" {
		return -1
	}
	return findMatchIndex(len(m.statusTreeFlat), start, forward, func(i int) bool {
		return strings.Contains(strings.ToLower(m.statusTreeFlat[i].Path), lowerQuery)
	})
}

func (m *Model) findLogMatchIndex(query string, start int, forward bool) int {
	lowerQuery := strings.ToLower(strings.TrimSpace(query))
	if lowerQuery == "" {
		return -1
	}
	return findMatchIndex(len(m.logEntries), start, forward, func(i int) bool {
		return strings.Contains(strings.ToLower(m.logEntries[i].message), lowerQuery)
	})
}

func (m *Model) applySearchQuery(query string) tea.Cmd {
	switch m.searchTarget {
	case searchTargetStatus:
		if idx := m.findStatusMatchIndex(query, 0, true); idx >= 0 {
			m.statusTreeIndex = idx
			m.rebuildStatusContentWithHighlight()
		}
	case searchTargetLog:
		if idx := m.findLogMatchIndex(query, 0, true); idx >= 0 {
			m.logTable.SetCursor(idx)
		}
	default:
		if idx := m.findWorktreeMatchIndex(query, 0, true); idx >= 0 {
			m.worktreeTable.SetCursor(idx)
			m.selectedIndex = idx
			return m.debouncedUpdateDetailsView()
		}
	}
	return nil
}

func (m *Model) advanceSearchMatch(forward bool) tea.Cmd {
	query := strings.TrimSpace(m.searchQueryForTarget(m.searchTarget))
	if query == "" {
		return nil
	}
	switch m.searchTarget {
	case searchTargetStatus:
		start := m.statusTreeIndex
		if forward {
			start++
		} else {
			start--
		}
		if idx := m.findStatusMatchIndex(query, start, forward); idx >= 0 {
			m.statusTreeIndex = idx
			m.rebuildStatusContentWithHighlight()
		}
	case searchTargetLog:
		start := m.logTable.Cursor()
		if forward {
			start++
		} else {
			start--
		}
		if idx := m.findLogMatchIndex(query, start, forward); idx >= 0 {
			m.logTable.SetCursor(idx)
		}
	default:
		start := m.worktreeTable.Cursor()
		if forward {
			start++
		} else {
			start--
		}
		if idx := m.findWorktreeMatchIndex(query, start, forward); idx >= 0 {
			m.worktreeTable.SetCursor(idx)
			m.selectedIndex = idx
			return m.debouncedUpdateDetailsView()
		}
	}
	return nil
}

// formatStatusDisplay converts a git status code (XY format) to a user-friendly display string.
// X = staged status, Y = unstaged status
// Examples: "M " -> "S ", " M" -> "M", "MM" -> "SM", " ?" -> "?"
func formatStatusDisplay(status string) string {
	if len(status) < 2 {
		return status
	}

	x := status[0] // Staged status
	y := status[1] // Unstaged status

	// Special case for untracked files
	if status == " ?" {
		return "?"
	}

	// Build display string
	var display [2]rune

	// First character: show staged status as S for modifications, or original for add/delete
	// #nosec G602 -- array size is 2, index is 0 and 1, always within bounds after len check
	switch x {
	case 'M':
		display[0] = 'S' // Staged modification
	case '.', ' ':
		display[0] = ' ' // No staged changes
	default:
		display[0] = rune(x) // A, D, R, C, etc.
	}

	// Second character: show unstaged status
	// #nosec G602 -- array size is 2, index is 0 and 1, always within bounds after len check
	switch y {
	case '.', ' ':
		display[1] = ' ' // No unstaged changes
	default:
		display[1] = rune(y) // M, A, D, R, C, etc.
	}

	return string(display[:])
}

// renderStatusFiles renders the status file list with current selection highlighted.
func (m *Model) renderStatusFiles() string {
	if len(m.statusTreeFlat) == 0 {
		if len(m.statusFilesAll) == 0 {
			return lipgloss.NewStyle().Foreground(m.theme.SuccessFg).Render("Clean working tree")
		}
		if strings.TrimSpace(m.statusFilterQuery) != "" {
			return lipgloss.NewStyle().Foreground(m.theme.MutedFg).Render(
				fmt.Sprintf("No files match %q", strings.TrimSpace(m.statusFilterQuery)),
			)
		}
		return lipgloss.NewStyle().Foreground(m.theme.MutedFg).Render("No files to display")
	}

	modifiedStyle := lipgloss.NewStyle().Foreground(m.theme.WarnFg)
	addedStyle := lipgloss.NewStyle().Foreground(m.theme.SuccessFg)
	deletedStyle := lipgloss.NewStyle().Foreground(m.theme.ErrorFg)
	untrackedStyle := lipgloss.NewStyle().Foreground(m.theme.Yellow)
	stagedStyle := lipgloss.NewStyle().Foreground(m.theme.Cyan)
	dirStyle := lipgloss.NewStyle().Foreground(m.theme.MutedFg)
	selectedStyle := lipgloss.NewStyle().
		Foreground(m.theme.AccentFg).
		Background(m.theme.Accent).
		Bold(true)

	viewportWidth := m.statusViewport.Width

	lines := make([]string, 0, len(m.statusTreeFlat))
	for i, node := range m.statusTreeFlat {
		indent := strings.Repeat("  ", node.depth)

		var lineContent string
		var fileIcon string
		if node.IsDir() {
			// Directory line: "  ▼ dirname" or "  ▶ dirname"
			expandIcon := "▼"
			if m.statusCollapsedDirs[node.Path] {
				expandIcon = "▶"
			}
			dirIcon := ""
			if m.config.ShowIcons {
				dirIcon = iconWithSpace(deviconForName(node.Name(), true))
			}
			lineContent = fmt.Sprintf("%s%s %s%s", indent, expandIcon, dirIcon, node.Path)
		} else {
			// File line: "    M  filename" or "    S  filename" for staged
			status := node.File.Status
			displayStatus := formatStatusDisplay(status)
			if m.config.ShowIcons {
				fileIcon = iconWithSpace(deviconForName(node.Name(), false))
			}
			lineContent = fmt.Sprintf("%s  %s %s%s", indent, displayStatus, fileIcon, node.Name())
		}

		// Apply styling based on selection and node type
		switch {
		case m.focusedPane == 1 && i == m.statusTreeIndex:
			if viewportWidth > 0 && len(lineContent) < viewportWidth {
				lineContent += strings.Repeat(" ", viewportWidth-len(lineContent))
			}
			lines = append(lines, selectedStyle.Render(lineContent))
		case node.IsDir():
			lines = append(lines, dirStyle.Render(lineContent))
		default:
			// Color based on file status - apply different colors for staged vs unstaged
			status := node.File.Status
			if len(status) < 2 {
				lines = append(lines, lineContent)
				continue
			}

			// Special case for untracked files
			if status == " ?" {
				displayStatus := formatStatusDisplay(status)
				formatted := fmt.Sprintf("%s  %s %s%s", indent, untrackedStyle.Render(displayStatus), fileIcon, node.Name())
				lines = append(lines, formatted)
				continue
			}

			x := status[0] // Staged status
			y := status[1] // Unstaged status
			displayStatus := formatStatusDisplay(status)

			// Render each character with appropriate color based on position
			var statusRendered strings.Builder
			for i, char := range displayStatus {
				if char == ' ' {
					statusRendered.WriteString(" ")
					continue
				}

				var style lipgloss.Style
				if i == 0 {
					// First character is staged (X position)
					switch x {
					case 'M':
						style = stagedStyle // Cyan for staged modifications
					case 'A':
						style = addedStyle // Green for staged additions
					case 'D':
						style = deletedStyle // Red for staged deletions
					case 'R', 'C':
						style = stagedStyle // Cyan for staged renames/copies
					default:
						style = lipgloss.NewStyle()
					}
				} else {
					// Second character is unstaged (Y position)
					switch y {
					case 'M':
						style = modifiedStyle // Orange for unstaged modifications
					case 'A':
						style = addedStyle // Green for unstaged additions
					case 'D':
						style = deletedStyle // Red for unstaged deletions
					case 'R', 'C':
						style = modifiedStyle // Orange for unstaged renames/copies
					default:
						style = lipgloss.NewStyle()
					}
				}
				statusRendered.WriteString(style.Render(string(char)))
			}
			formatted := fmt.Sprintf("%s  %s %s%s", indent, statusRendered.String(), fileIcon, node.Name())
			lines = append(lines, formatted)
		}
	}
	return strings.Join(lines, "\n")
}

// rebuildStatusContentWithHighlight re-renders the status content with current selection highlighted.
func (m *Model) rebuildStatusContentWithHighlight() {
	m.statusContent = m.renderStatusFiles()
	m.statusViewport.SetContent(m.statusContent)

	if len(m.statusTreeFlat) == 0 {
		return
	}

	// Auto-scroll to keep selected item visible
	viewportHeight := m.statusViewport.Height
	if viewportHeight > 0 && m.statusTreeIndex >= 0 {
		currentOffset := m.statusViewport.YOffset
		if m.statusTreeIndex < currentOffset {
			m.statusViewport.SetYOffset(m.statusTreeIndex)
		} else if m.statusTreeIndex >= currentOffset+viewportHeight {
			m.statusViewport.SetYOffset(m.statusTreeIndex - viewportHeight + 1)
		}
	}
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
		{Title: "Name", Width: worktree},
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
	author := 2

	// The table library handles separators internally (3 spaces per separator)
	// 3 columns = 2 separators = 6 spaces
	separatorSpace := 6

	message := maxInt(10, totalWidth-sha-author-separatorSpace)

	// Final adjustment: ensure column widths + separator space sum exactly to totalWidth
	actualTotal := sha + author + message + separatorSpace
	if actualTotal < totalWidth {
		message += (totalWidth - actualTotal)
	} else if actualTotal > totalWidth {
		message = maxInt(10, message-(actualTotal-totalWidth))
	}

	m.logTable.SetColumns([]table.Column{
		{Title: "SHA", Width: sha},
		{Title: "Au", Width: author},
		{Title: "Message", Width: message},
	})
}

func (m *Model) basePaneStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.BorderDim).
		Padding(0, 1)
}

func (m *Model) paneStyle(focused bool) lipgloss.Style {
	borderColor := m.theme.BorderDim
	borderStyle := lipgloss.NormalBorder()
	if focused {
		borderColor = m.theme.Accent
		// Use rounded border for focused panes for modern look
		borderStyle = lipgloss.RoundedBorder()
	}
	return lipgloss.NewStyle().
		Border(borderStyle).
		BorderForeground(borderColor).
		Padding(0, 1)
}

func (m *Model) baseInnerBoxStyle() lipgloss.Style {
	// Use rounded border for inner boxes for softer appearance
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
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
