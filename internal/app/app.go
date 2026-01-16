// Package app provides the main application UI and logic using Bubble Tea.
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
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
	log "github.com/chmouel/lazyworktree/internal/log"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/security"
	"github.com/chmouel/lazyworktree/internal/theme"
	"github.com/chmouel/lazyworktree/internal/utils"
	"github.com/fsnotify/fsnotify"
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
	defaultDirPerms  = utils.DefaultDirPerms
	defaultFilePerms = 0o600

	osDarwin  = "darwin"
	osWindows = "windows"

	// Visual symbols for enhanced UI
	symbolFilledCircle = "●"

	searchFiles = "Search files..."

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
		prMap          map[string]*models.PRInfo
		worktreePRs    map[string]*models.PRInfo // keyed by worktree path
		worktreeErrors map[string]string         // keyed by worktree path, stores error messages
		err            error
	}
	statusUpdatedMsg struct {
		info        string
		statusFiles []StatusFile
		log         []commitLogEntry
		path        string
	}
	refreshCompleteMsg      struct{}
	fetchRemotesCompleteMsg struct{}
	autoRefreshTickMsg      struct{}
	gitDirChangedMsg        struct{}
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
	worktreeDeletedMsg struct {
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
	pushResultMsg struct {
		output string
		err    error
	}
	syncResultMsg struct {
		stage  string
		output string
		err    error
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
	createFromCurrentReadyMsg struct {
		currentWorktree   *models.WorktreeInfo
		currentBranch     string
		diff              string
		hasChanges        bool
		defaultBranchName string
	}
	cherryPickResultMsg struct {
		commitSHA      string
		targetWorktree *models.WorktreeInfo
		err            error
	}
	aiBranchNameGeneratedMsg struct {
		name string
		err  error
	}
	commitFilesLoadedMsg struct {
		sha          string
		worktreePath string
		files        []models.CommitFile
		meta         commitMeta
		err          error
	}
	customCreateResultMsg struct {
		branchName string
		err        error
	}
	customPostCommandPendingMsg struct {
		targetPath string
		env        map[string]string
	}
	customPostCommandResultMsg struct {
		err error
	}
	loadingProgressMsg struct {
		message string
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
	pullRebaseFlag    = "--rebase=true"

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
	worktrees                 []*models.WorktreeInfo
	filteredWts               []*models.WorktreeInfo
	selectedIndex             int
	filterQuery               string
	statusFilterQuery         string
	logFilterQuery            string
	worktreeSearchQuery       string
	statusSearchQuery         string
	logSearchQuery            string
	sortMode                  int // sortModePath, sortModeLastActive, or sortModeLastSwitched
	prDataLoaded              bool
	checkMergedAfterPRRefresh bool             // Flag to trigger merged check after PR data refresh
	accessHistory             map[string]int64 // worktree path -> last access timestamp
	repoKey                   string
	repoKeyOnce               sync.Once
	currentScreen             screenType
	currentDetailsPath        string
	helpScreen                *HelpScreen
	trustScreen               *TrustScreen
	inputScreen               *InputScreen
	inputSubmit               func(string, bool) (tea.Cmd, bool)
	commitScreen              *CommitScreen
	welcomeScreen             *WelcomeScreen
	paletteScreen             *CommandPaletteScreen
	paletteSubmit             func(string) tea.Cmd
	prSelectionScreen         *PRSelectionScreen
	issueSelectionScreen      *IssueSelectionScreen
	issueSelectionSubmit      func(*models.IssueInfo) tea.Cmd
	prSelectionSubmit         func(*models.PRInfo) tea.Cmd
	listScreen                *ListSelectionScreen
	listSubmit                func(selectionItem) tea.Cmd
	checklistScreen           *ChecklistScreen
	checklistSubmit           func([]ChecklistItem) tea.Cmd
	spinner                   spinner.Model
	loading                   bool
	loadingOperation          string // Tracks what operation is loading (push, sync, etc.)
	showingFilter             bool
	filterTarget              filterTarget
	showingSearch             bool
	searchTarget              searchTarget
	focusedPane               int // 0=table, 1=status, 2=log
	zoomedPane                int // -1 = no zoom, 0/1/2 = which pane is zoomed
	windowWidth               int
	windowHeight              int
	infoContent               string
	statusContent             string
	statusFiles               []StatusFile // parsed list of files from git status (kept for compatibility)
	statusFilesAll            []StatusFile // full list of files from git status
	statusFileIndex           int          // currently selected file index in status pane

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

	// Create from current state
	createFromCurrentDiff       string // Cached diff for AI script
	createFromCurrentRandomName string // Random branch name
	createFromCurrentAIName     string // AI-generated name (cached)
	createFromCurrentBranch     string // Current branch name

	// Services
	trustManager *security.TrustManager

	// Context
	ctx    context.Context
	cancel context.CancelFunc

	// Debouncing
	detailUpdateCancel  context.CancelFunc
	pendingDetailsIndex int

	// Auto refresh
	autoRefreshStarted bool
	gitWatchStarted    bool
	gitWatchWaiting    bool
	gitCommonDir       string
	gitWatchRoots      []string
	gitWatchEvents     chan struct{}
	gitWatchDone       chan struct{}
	gitWatchPaths      map[string]struct{}
	gitWatchMu         sync.Mutex
	gitWatcher         *fsnotify.Watcher
	gitLastRefresh     time.Time

	// Post-refresh selection (e.g. after creating worktree)
	pendingSelectWorktreePath string

	// Confirm screen
	confirmScreen *ConfirmScreen
	confirmAction func() tea.Cmd
	confirmCancel func() tea.Cmd
	infoScreen    *InfoScreen
	infoAction    tea.Cmd
	loadingScreen *LoadingScreen

	// Trust / repo commands
	repoConfig              *config.RepoConfig
	repoConfigPath          string
	pendingCommands         []string
	pendingCmdEnv           map[string]string
	pendingCmdCwd           string
	pendingAfter            func() tea.Msg
	pendingTrust            string
	pendingCustomBranchName string                   // Branch name from custom create command
	pendingCustomBaseRef    string                   // Base ref for custom create (selected before running command)
	pendingCustomMenu       *config.CustomCreateMenu // Menu item for custom create

	// Log cache for commit detail viewer
	logEntries    []commitLogEntry
	logEntriesAll []commitLogEntry

	// Commit files screen for browsing files in a commit
	commitFilesScreen *CommitFilesScreen

	// Command history for ! command
	commandHistory []string

	// Command palette usage history for MRU sorting
	paletteHistory []commandPaletteUsage

	// Original theme before theme selection (for preview rollback)
	originalTheme string

	// Exit
	selectedPath string
	quitting     bool

	// Command execution
	commandRunner func(string, ...string) *exec.Cmd
	execProcess   func(*exec.Cmd, tea.ExecCallback) tea.Cmd
	startCommand  func(*exec.Cmd) error
}

// NewModel creates a new application model with the given configuration.
// initialFilter is an optional filter string to apply on startup.
func NewModel(cfg *config.AppConfig, initialFilter string) *Model {
	ctx, cancel := context.WithCancel(context.Background())

	// Load theme
	thm := theme.GetTheme(cfg.Theme)

	debugNotified := map[string]bool{}
	var debugMu sync.Mutex // Protects debugNotified map

	log.Printf("debug logging enabled")

	notify := func(message string, severity string) {
		log.Printf("[%s] %s", severity, message)
	}
	notifyOnce := func(key string, message string, severity string) {
		debugMu.Lock()
		defer debugMu.Unlock()
		if debugNotified[key] {
			return
		}
		debugNotified[key] = true
		log.Printf("[%s] %s", severity, message)
	}

	gitService := git.NewService(notify, notifyOnce)
	gitService.SetGitPager(cfg.GitPager)
	gitService.SetGitPagerArgs(cfg.GitPagerArgs)
	trustManager := security.NewTrustManager()

	columns := []table.Column{
		{Title: "Name", Width: 20},
		{Title: "Changes", Width: 8},
		{Title: "Status", Width: 7},
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
	sp.Spinner = spinner.MiniDot
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
	m.loadPaletteHistory()
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

	case worktreeDeletedMsg:
		if msg.err != nil {
			// Worktree deletion failed, don't prompt for branch deletion
			return m, nil
		}

		// Worktree deleted successfully, show branch deletion prompt
		m.confirmScreen = NewConfirmScreenWithDefault(
			fmt.Sprintf("Worktree deleted successfully.\n\nDelete branch '%s'?", msg.branch),
			0, // Default to Confirm button (Yes)
			m.theme,
		)
		m.confirmAction = m.deleteBranchCmd(msg.branch)
		m.currentScreen = screenConfirm
		return m, nil

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

	case customCreateResultMsg:
		m.loading = false
		if m.currentScreen == screenLoading {
			m.currentScreen = screenNone
			m.loadingScreen = nil
		}
		if msg.err != nil {
			m.showInfo(fmt.Sprintf("Custom command failed: %v", msg.err), nil)
			return m, nil
		}
		// Store the branch name and show branch name input with the selected base ref
		m.pendingCustomBranchName = msg.branchName
		return m, m.showBranchNameInput(m.pendingCustomBaseRef, msg.branchName)

	case customPostCommandPendingMsg:
		if m.pendingCustomMenu == nil || m.pendingCustomMenu.PostCommand == "" {
			// No post-command, just reload
			worktrees, err := m.git.GetWorktrees(m.ctx)
			return m, func() tea.Msg {
				return worktreesLoadedMsg{worktrees: worktrees, err: err}
			}
		}

		menu := m.pendingCustomMenu
		cmd := menu.PostCommand
		interactive := menu.PostInteractive

		// Clear the pending menu
		m.pendingCustomMenu = nil
		m.pendingCustomBaseRef = ""
		m.pendingCustomBranchName = ""

		// Run the post-command
		if interactive {
			return m, m.executeCustomPostCommandInteractive(cmd, msg.targetPath, msg.env)
		}
		return m, m.executeCustomPostCommand(cmd, msg.targetPath, msg.env)

	case customPostCommandResultMsg:
		m.loading = false
		if m.currentScreen == screenLoading {
			m.currentScreen = screenNone
			m.loadingScreen = nil
		}

		if msg.err != nil {
			// Show error but continue (worktree was already created)
			m.showInfo(fmt.Sprintf("Post-creation command failed: %v", msg.err), nil)
		}

		// Reload worktrees regardless
		worktrees, err := m.git.GetWorktrees(m.ctx)
		return m, func() tea.Msg {
			return worktreesLoadedMsg{worktrees: worktrees, err: err}
		}

	case loadingProgressMsg:
		// Update the loading screen message with progress information
		if m.loadingScreen != nil {
			m.loadingScreen.message = msg.message
		}
		return m, nil

	case createFromChangesReadyMsg:
		return m, m.handleCreateFromChangesReady(msg)

	case createFromCurrentReadyMsg:
		return m, m.handleCreateFromCurrentReady(msg)

	case aiBranchNameGeneratedMsg:
		if msg.err != nil || msg.name == "" {
			// Failed to generate, keep current value
			return m, nil
		}

		// CRITICAL: Sanitize AI-generated name to remove invalid characters like '/'
		// This prevents creating nested directories in worktree path
		sanitizedName := sanitizeBranchNameFromTitle(msg.name, m.createFromCurrentRandomName)

		// Cache the generated name
		suggestedName := m.suggestBranchName(sanitizedName)
		m.createFromCurrentAIName = suggestedName

		// Update input field if checkbox is still checked
		if m.inputScreen != nil && m.inputScreen.checkboxChecked {
			m.inputScreen.input.SetValue(suggestedName)
			m.inputScreen.input.CursorEnd()
		}

		return m, nil

	case prDataLoadedMsg, ciStatusLoadedMsg:
		return m.handlePRMessages(msg)

	case statusUpdatedMsg:
		if msg.info != "" {
			m.infoContent = msg.info
		}
		m.setStatusFiles(msg.statusFiles)
		m.updateWorktreeStatus(msg.path, msg.statusFiles)
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

	case pushResultMsg:
		m.loading = false
		m.loadingOperation = ""
		if m.currentScreen == screenLoading {
			m.currentScreen = screenNone
			m.loadingScreen = nil
		}
		output := strings.TrimSpace(msg.output)
		if msg.err != nil {
			message := fmt.Sprintf("Push failed: %v", msg.err)
			if output != "" {
				message = fmt.Sprintf("Push failed.\n\n%s", truncateToHeightFromEnd(output, 5))
			}
			m.showInfo(message, nil)
			return m, nil
		}
		if output != "" {
			message := fmt.Sprintf("Push completed.\n\n%s", truncateToHeight(output, 3))
			m.showInfo(message, m.updateDetailsView())
			return m, nil
		}
		m.statusContent = "Push completed"
		return m, m.updateDetailsView()

	case syncResultMsg:
		m.loading = false
		m.loadingOperation = ""
		if m.currentScreen == screenLoading {
			m.currentScreen = screenNone
			m.loadingScreen = nil
		}
		output := strings.TrimSpace(msg.output)
		if msg.err != nil {
			heading := "Synchronise failed."
			switch msg.stage {
			case "pull":
				heading = "Pull failed."
			case "push":
				heading = "Push failed."
			}
			message := fmt.Sprintf("%s: %v", heading, msg.err)
			if output != "" {
				message = fmt.Sprintf("%s\n\n%s", heading, truncateToHeightFromEnd(output, 5))
			}
			m.showInfo(message, nil)
			return m, nil
		}
		if output != "" {
			message := fmt.Sprintf("Synchronised.\n\n%s", truncateToHeight(output, 3))
			m.showInfo(message, m.updateDetailsView())
			return m, nil
		}
		m.statusContent = "Synchronised"
		return m, m.updateDetailsView()

	case autoRefreshTickMsg:
		if cmd := m.autoRefreshTick(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		if cmd := m.refreshDetails(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	case gitDirChangedMsg:
		m.gitWatchWaiting = false
		cmds = append(cmds, m.waitForGitWatchEvent())
		if m.shouldRefreshGitEvent(time.Now()) {
			cmds = append(cmds, m.refreshWorktrees())
		}
		return m, tea.Batch(cmds...)

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
	case screenPRSelect:
		return "pr-select"
	case screenIssueSelect:
		return "issue-select"
	case screenListSelect:
		return "list-select"
	case screenCommitFiles:
		return "commit-files"
	case screenChecklist:
		return "checklist"
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
	case screenChecklist:
		if m.checklistScreen != nil {
			return m.overlayPopup(baseView, m.checklistScreen.View(), 2)
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

	if len(baseLines) == 0 {
		return popup
	}

	baseWidth := lipgloss.Width(baseLines[0])
	popupWidth := lipgloss.Width(popupLines[0])

	leftPad := max((baseWidth-popupWidth)/2, 0)
	leftSpace := strings.Repeat(" ", leftPad)
	rightPad := max(baseWidth-popupWidth-leftPad, 0)
	rightSpace := strings.Repeat(" ", rightPad)

	for i, line := range popupLines {
		row := marginTop + i
		if row >= len(baseLines) {
			break
		}

		// Main popup line
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
		return searchFiles
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

		// Truncate to configured max length with ellipsis if needed
		if m.config.MaxNameLength > 0 {
			nameRunes := []rune(name)
			if len(nameRunes) > m.config.MaxNameLength {
				name = string(nameRunes[:m.config.MaxNameLength]) + "..."
			}
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

		// DEBUG: Log what we got from FetchPRMap
		log.Printf("FetchPRMap returned %d PRs", len(prMap))
		for branch, pr := range prMap {
			log.Printf("  prMap[%q] = PR#%d", branch, pr.Number)
		}

		// Also fetch PRs per worktree for cases where local branch differs from remote
		// This handles fork PRs where local branch name doesn't match headRefName
		worktreePRs := make(map[string]*models.PRInfo)
		worktreeErrors := make(map[string]string)
		for _, wt := range m.worktrees {
			// DEBUG: Log branch name and match attempt
			log.Printf("Checking worktree: Branch=%q Path=%q", wt.Branch, wt.Path)
			if pr, ok := prMap[wt.Branch]; ok {
				log.Printf("  Found in prMap: PR#%d", pr.Number)
			} else {
				log.Printf("  Not in prMap, will fetch per-worktree")
			}

			// Skip if already matched by headRefName
			if _, ok := prMap[wt.Branch]; ok {
				continue
			}
			// Try to fetch PR for this worktree directly
			pr, fetchErr := m.git.FetchPRForWorktreeWithError(m.ctx, wt.Path)
			if pr != nil {
				worktreePRs[wt.Path] = pr
				log.Printf("  FetchPRForWorktree returned PR#%d", pr.Number)
			}
			if fetchErr != nil {
				worktreeErrors[wt.Path] = fetchErr.Error()
				log.Printf("  FetchPRForWorktree error: %v", fetchErr)
			}
			if pr == nil && fetchErr == nil {
				log.Printf("  FetchPRForWorktree returned nil (no PR)")
			}
		}

		return prDataLoadedMsg{
			prMap:          prMap,
			worktreePRs:    worktreePRs,
			worktreeErrors: worktreeErrors,
			err:            nil,
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

func (m *Model) pushToUpstream() tea.Cmd {
	wt := m.selectedWorktree()
	if wt == nil {
		m.showInfo(errNoWorktreeSelected, nil)
		return nil
	}
	if hasLocalChanges(wt) {
		m.showInfo("Cannot push while the worktree has local changes.\n\nPlease commit, stash, or discard them first.", nil)
		return nil
	}
	if strings.TrimSpace(wt.Branch) == "" {
		m.showInfo("Cannot push a detached worktree.", nil)
		return nil
	}
	if wt.HasUpstream {
		remote, branch, ok := m.validatedUpstream(wt, "push")
		if !ok {
			return nil
		}
		return m.beginPush(wt, []string{remote, fmt.Sprintf("HEAD:%s", branch)})
	}
	return m.showUpstreamInput(wt, func(remote, branch string) tea.Cmd {
		return m.beginPush(wt, []string{"-u", remote, fmt.Sprintf("HEAD:%s", branch)})
	})
}

func (m *Model) syncWithUpstream() tea.Cmd {
	wt := m.selectedWorktree()
	if wt == nil {
		m.showInfo(errNoWorktreeSelected, nil)
		return nil
	}
	if hasLocalChanges(wt) {
		m.showInfo("Cannot synchronise while the worktree has local changes.\n\nPlease commit, stash, or discard them first.", nil)
		return nil
	}
	if strings.TrimSpace(wt.Branch) == "" {
		m.showInfo("Cannot synchronise a detached worktree.", nil)
		return nil
	}

	// Check if this worktree has a PR and if we're behind the base branch
	if wt.PR != nil && wt.PR.BaseBranch != "" {
		if m.isBehindBase(wt) {
			return m.showSyncChoice(wt)
		}
	}

	// Normal sync (pull + push)
	if wt.HasUpstream {
		remote, branch, ok := m.validatedUpstream(wt, "synchronise")
		if !ok {
			return nil
		}
		return m.beginSync(wt, []string{remote, branch}, []string{remote, fmt.Sprintf("HEAD:%s", branch)})
	}
	return m.showUpstreamInput(wt, func(remote, branch string) tea.Cmd {
		return m.beginSync(wt, []string{remote, branch}, []string{"-u", remote, fmt.Sprintf("HEAD:%s", branch)})
	})
}

func (m *Model) beginPush(wt *models.WorktreeInfo, args []string) tea.Cmd {
	m.loading = true
	m.loadingOperation = "push"
	m.statusContent = "Pushing to upstream..."
	m.loadingScreen = NewLoadingScreen("Pushing to upstream...", m.theme)
	m.currentScreen = screenLoading
	return m.runPush(wt, args)
}

func (m *Model) beginSync(wt *models.WorktreeInfo, pullArgs, pushArgs []string) tea.Cmd {
	m.loading = true
	m.loadingOperation = "sync"
	m.statusContent = "Synchronising with upstream..."
	m.loadingScreen = NewLoadingScreen("Synchronising with upstream...", m.theme)
	m.currentScreen = screenLoading
	return m.runSync(wt, pullArgs, pushArgs)
}

func (m *Model) isBehindBase(wt *models.WorktreeInfo) bool {
	if wt.PR == nil || wt.PR.BaseBranch == "" {
		return false
	}
	// Check if current branch is behind the base branch
	// Use git merge-base to find common ancestor, then check if we're behind
	mergeBase := m.git.RunGit(m.ctx, []string{
		"git", "merge-base", "HEAD", wt.PR.BaseBranch,
	}, wt.Path, []int{0, 1}, true, false)

	if mergeBase == "" {
		return false
	}

	// Check if there are commits in base that aren't in HEAD
	behindCount := m.git.RunGit(m.ctx, []string{
		"git", "rev-list", "--count", fmt.Sprintf("HEAD..%s", wt.PR.BaseBranch),
	}, wt.Path, []int{0}, true, false)

	behind, _ := strconv.Atoi(strings.TrimSpace(behindCount))
	return behind > 0
}

func (m *Model) showSyncChoice(wt *models.WorktreeInfo) tea.Cmd {
	// Store the worktree for later use in confirm/cancel handlers
	savedWt := wt

	m.confirmScreen = NewConfirmScreen(
		fmt.Sprintf("Branch behind %s\n\nUpdate from base branch?\n(This will merge/rebase latest %s into your branch.\nChoose 'No' for normal sync: pull + push)",
			wt.PR.BaseBranch, wt.PR.BaseBranch),
		m.theme,
	)
	m.confirmAction = func() tea.Cmd {
		// User chose YES: update from base
		return m.updateFromBase(savedWt)
	}
	// Store cancel action for normal sync
	m.confirmCancel = func() tea.Cmd {
		// User chose NO: do normal sync (pull + push)
		if savedWt.HasUpstream {
			remote, branch, ok := m.validatedUpstream(savedWt, "synchronise")
			if !ok {
				return nil
			}
			return m.beginSync(savedWt, []string{remote, branch}, []string{remote, fmt.Sprintf("HEAD:%s", branch)})
		}
		return m.showUpstreamInput(savedWt, func(remote, branch string) tea.Cmd {
			return m.beginSync(savedWt, []string{remote, branch}, []string{"-u", remote, fmt.Sprintf("HEAD:%s", branch)})
		})
	}
	m.currentScreen = screenConfirm
	return nil
}

func (m *Model) updateFromBase(wt *models.WorktreeInfo) tea.Cmd {
	m.loading = true
	m.loadingOperation = "sync"
	m.statusContent = fmt.Sprintf("Updating from %s...", wt.PR.BaseBranch)
	m.loadingScreen = NewLoadingScreen(fmt.Sprintf("Updating from %s...", wt.PR.BaseBranch), m.theme)
	m.currentScreen = screenLoading

	// Use gh pr update-branch with --rebase if merge_method is rebase
	args := []string{"gh", "pr", "update-branch"}
	mergeMethod := strings.TrimSpace(m.config.MergeMethod)
	if mergeMethod == "" {
		mergeMethod = mergeMethodRebase
	}
	if mergeMethod == mergeMethodRebase {
		args = append(args, "--rebase")
	}

	// Clear cache so status pane refreshes
	delete(m.detailsCache, wt.Path)

	cmd := m.commandRunner(args[0], args[1:]...)
	cmd.Dir = wt.Path

	return func() tea.Msg {
		output, err := cmd.CombinedOutput()
		return syncResultMsg{
			stage:  "update-branch",
			output: strings.TrimSpace(string(output)),
			err:    err,
		}
	}
}

func (m *Model) showUpstreamInput(wt *models.WorktreeInfo, onSubmit func(remote, branch string) tea.Cmd) tea.Cmd {
	defaultUpstream := fmt.Sprintf("origin/%s", wt.Branch)
	prompt := fmt.Sprintf("Set upstream for '%s' (remote/branch)", wt.Branch)
	m.inputScreen = NewInputScreen(prompt, defaultUpstream, defaultUpstream, m.theme)
	m.inputSubmit = func(value string, checked bool) (tea.Cmd, bool) {
		remote, branch, ok := parseUpstreamRef(value)
		if !ok {
			m.inputScreen.errorMsg = "Please provide upstream as remote/branch."
			return nil, false
		}
		if branch != wt.Branch {
			m.inputScreen.errorMsg = fmt.Sprintf("Upstream branch must match %q.", wt.Branch)
			return nil, false
		}
		m.inputScreen.errorMsg = ""
		return onSubmit(remote, branch), true
	}
	m.currentScreen = screenInput
	return textinput.Blink
}

func (m *Model) validatedUpstream(wt *models.WorktreeInfo, action string) (string, string, bool) {
	upstream := strings.TrimSpace(wt.UpstreamBranch)
	if upstream == "" {
		m.showInfo(fmt.Sprintf("Cannot %s because no upstream is configured.", action), nil)
		return "", "", false
	}
	remote, branch, ok := parseUpstreamRef(upstream)
	if !ok {
		m.showInfo(fmt.Sprintf("Cannot %s because upstream %q is not in remote/branch format.", action, upstream), nil)
		return "", "", false
	}
	if branch != wt.Branch {
		m.showInfo(fmt.Sprintf("Cannot %s because upstream %q does not match current branch %q.", action, upstream, wt.Branch), nil)
		return "", "", false
	}
	return remote, branch, true
}

func parseUpstreamRef(input string) (string, string, bool) {
	value := strings.TrimSpace(input)
	if value == "" {
		return "", "", false
	}
	remote, branch, ok := strings.Cut(value, "/")
	if !ok {
		return "", "", false
	}
	remote = strings.TrimSpace(remote)
	branch = strings.TrimSpace(branch)
	if remote == "" || branch == "" {
		return "", "", false
	}
	return remote, branch, true
}

func hasLocalChanges(wt *models.WorktreeInfo) bool {
	if wt == nil {
		return false
	}
	if wt.Dirty {
		return true
	}
	return wt.Untracked > 0 || wt.Modified > 0 || wt.Staged > 0
}

func (m *Model) runPush(wt *models.WorktreeInfo, args []string) tea.Cmd {
	env := m.buildCommandEnv(wt.Branch, wt.Path)
	envVars := os.Environ()
	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	// Clear cache so status pane refreshes with latest git status
	delete(m.detailsCache, wt.Path)

	cmdArgs := append([]string{"push"}, args...)
	c := m.commandRunner("git", cmdArgs...)
	c.Dir = wt.Path
	c.Env = envVars

	return func() tea.Msg {
		output, err := c.CombinedOutput()
		return pushResultMsg{
			output: strings.TrimSpace(string(output)),
			err:    err,
		}
	}
}

func (m *Model) runSync(wt *models.WorktreeInfo, pullArgs, pushArgs []string) tea.Cmd {
	env := m.buildCommandEnv(wt.Branch, wt.Path)
	envVars := os.Environ()
	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	// Clear cache so status pane refreshes with latest git status
	delete(m.detailsCache, wt.Path)

	pullCmdArgs := append([]string{"pull"}, m.syncPullArgs(pullArgs)...)
	pullCmd := m.commandRunner("git", pullCmdArgs...)
	pullCmd.Dir = wt.Path
	pullCmd.Env = envVars

	return func() tea.Msg {
		pullOutput, pullErr := pullCmd.CombinedOutput()
		pullText := strings.TrimSpace(string(pullOutput))
		if pullErr != nil {
			return syncResultMsg{
				stage:  "pull",
				output: pullText,
				err:    pullErr,
			}
		}

		pushCmdArgs := append([]string{"push"}, pushArgs...)
		pushCmd := m.commandRunner("git", pushCmdArgs...)
		pushCmd.Dir = wt.Path
		pushCmd.Env = envVars

		pushOutput, pushErr := pushCmd.CombinedOutput()
		pushText := strings.TrimSpace(string(pushOutput))
		combined := strings.TrimSpace(strings.Join(filterNonEmpty([]string{pullText, pushText}), "\n"))

		if pushErr != nil {
			return syncResultMsg{
				stage:  "push",
				output: combined,
				err:    pushErr,
			}
		}
		return syncResultMsg{
			output: combined,
			err:    nil,
		}
	}
}

func (m *Model) syncPullArgs(pullArgs []string) []string {
	mergeMethod := strings.TrimSpace(m.config.MergeMethod)
	if mergeMethod == "" {
		mergeMethod = mergeMethodRebase
	}
	if mergeMethod == mergeMethodRebase {
		return append([]string{pullRebaseFlag}, pullArgs...)
	}
	return pullArgs
}

func (m *Model) showCreateWorktree() tea.Cmd {
	defaultBase := m.git.GetMainBranch(m.ctx)
	return m.showBaseSelection(defaultBase)
}

func (m *Model) determineCurrentWorktree() *models.WorktreeInfo {
	if wt := m.selectedWorktree(); wt != nil {
		return wt
	}

	if cwd, err := os.Getwd(); err == nil {
		for _, wt := range m.worktrees {
			if strings.HasPrefix(cwd, wt.Path) {
				return wt
			}
		}
	}

	for _, wt := range m.worktrees {
		if wt.IsMain {
			return wt
		}
	}

	return nil
}

func (m *Model) selectedWorktree() *models.WorktreeInfo {
	indices := []int{m.worktreeTable.Cursor(), m.selectedIndex}
	for _, idx := range indices {
		if wt := m.worktreeAtIndex(idx); wt != nil {
			return wt
		}
	}
	return nil
}

func (m *Model) worktreeAtIndex(idx int) *models.WorktreeInfo {
	if idx < 0 || idx >= len(m.filteredWts) {
		return nil
	}
	return m.filteredWts[idx]
}

func (m *Model) showCreateFromCurrent() tea.Cmd {
	return func() tea.Msg {
		currentWt := m.determineCurrentWorktree()
		if currentWt == nil {
			return errMsg{err: fmt.Errorf("could not determine current worktree")}
		}

		// Check for changes
		statusRaw := m.git.RunGit(m.ctx, []string{"git", "status", "--porcelain"}, currentWt.Path, []int{0}, true, false)
		hasChanges := strings.TrimSpace(statusRaw) != ""

		// Get current branch
		currentBranch := m.git.RunGit(m.ctx, []string{"git", "rev-parse", "--abbrev-ref", "HEAD"}, currentWt.Path, []int{0}, true, false)
		if currentBranch == "" {
			return errMsg{err: fmt.Errorf("failed to get current branch")}
		}
		currentBranch = strings.TrimSpace(currentBranch)

		// Always generate random name as default
		defaultName := fmt.Sprintf("%s-%s", currentBranch, randomBranchName())

		// Get diff if changes exist (for later AI generation)
		var diff string
		if hasChanges && m.config.BranchNameScript != "" {
			diff = m.git.RunGit(m.ctx, []string{"git", "diff", "HEAD"}, currentWt.Path, []int{0}, false, true)
		}

		return createFromCurrentReadyMsg{
			currentWorktree:   currentWt,
			currentBranch:     currentBranch,
			diff:              diff,
			hasChanges:        hasChanges,
			defaultBranchName: m.suggestBranchName(defaultName), // Use random name
		}
	}
}

// getCurrentBranchForMenu returns the current branch name for menu display.
// Returns empty string on error (caller should fallback to static label).
func (m *Model) getCurrentBranchForMenu() string {
	currentWt := m.determineCurrentWorktree()
	if currentWt == nil {
		return ""
	}

	branch := m.git.RunGit(
		m.ctx,
		[]string{"git", "rev-parse", "--abbrev-ref", "HEAD"},
		currentWt.Path,
		[]int{0},
		true,
		false,
	)
	return strings.TrimSpace(branch)
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
	m.inputSubmit = func(value string, checked bool) (tea.Cmd, bool) {
		newBranch := strings.TrimSpace(value)
		newBranch = sanitizeBranchNameFromTitle(newBranch, "")
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

func (m *Model) generateAIBranchName() tea.Cmd {
	return func() tea.Msg {
		name, err := runBranchNameScript(
			m.ctx,
			m.config.BranchNameScript,
			m.createFromCurrentDiff,
			"diff",
			"",
			"",
			"",
		)
		return aiBranchNameGeneratedMsg{name: name, err: err}
	}
}

func (m *Model) handleCheckboxToggle() tea.Cmd {
	if m.createFromCurrentDiff == "" {
		// Not in "create from current" flow, ignore
		return nil
	}

	if m.inputScreen.checkboxChecked {
		// Checkbox was checked: switch to AI name
		if m.createFromCurrentAIName != "" {
			// Use cached AI name
			m.inputScreen.input.SetValue(m.createFromCurrentAIName)
			m.inputScreen.input.CursorEnd()
			return nil
		}

		// Generate AI name if not cached
		if m.config.BranchNameScript != "" && m.createFromCurrentDiff != "" {
			return m.generateAIBranchName()
		}

		// No script configured, keep random name
		return nil
	}

	// Checkbox was unchecked: restore random name
	m.inputScreen.input.SetValue(m.createFromCurrentRandomName)
	m.inputScreen.input.CursorEnd()
	return nil
}

func (m *Model) handleCreateFromCurrentReady(msg createFromCurrentReadyMsg) tea.Cmd {
	if msg.currentWorktree == nil {
		m.showInfo("Could not determine current worktree", nil)
		return nil
	}

	// Store context for checkbox toggling
	m.createFromCurrentDiff = msg.diff
	m.createFromCurrentRandomName = msg.defaultBranchName
	m.createFromCurrentBranch = msg.currentBranch
	m.createFromCurrentAIName = "" // Reset cached AI name

	// Show input screen with random name
	m.inputScreen = NewInputScreen("Create from current: branch name", "feature/my-branch", msg.defaultBranchName, m.theme)
	if msg.hasChanges {
		m.inputScreen.SetCheckbox("Include current file changes", false)
	}

	// Capture context for closure
	wt := msg.currentWorktree
	currentBranch := msg.currentBranch
	hasChanges := msg.hasChanges

	m.inputSubmit = func(value string, checked bool) (tea.Cmd, bool) {
		newBranch := strings.TrimSpace(value)
		newBranch = sanitizeBranchNameFromTitle(newBranch, "")
		if newBranch == "" {
			m.inputScreen.errorMsg = errBranchEmpty
			return nil, false
		}

		// Validate branch doesn't exist
		if m.branchExistsInWorktrees(newBranch) {
			m.inputScreen.errorMsg = fmt.Sprintf("Branch %q already exists.", newBranch)
			return nil, false
		}

		// Check if branch exists in git
		branchRef := m.git.RunGit(m.ctx, []string{"git", "show-ref", fmt.Sprintf("refs/heads/%s", newBranch)}, "", []int{0, 1}, true, true)
		if branchRef != "" {
			m.inputScreen.errorMsg = fmt.Sprintf("Branch %q already exists.", newBranch)
			return nil, false
		}

		targetPath := filepath.Join(m.getWorktreeDir(), newBranch)
		if m.worktreePathExists(targetPath) {
			m.inputScreen.errorMsg = fmt.Sprintf("Path already exists: %s", targetPath)
			return nil, false
		}

		// Clear cached state
		m.createFromCurrentDiff = ""
		m.createFromCurrentRandomName = ""
		m.createFromCurrentAIName = ""
		m.createFromCurrentBranch = ""

		// Set pending selection so the new worktree is selected after creation
		m.pendingSelectWorktreePath = targetPath

		includeChanges := m.inputScreen.checkboxChecked
		// Only attempt to move changes if checkbox is checked AND there are actual changes
		// This prevents accidentally applying an unrelated existing stash when workspace is clean
		if includeChanges && hasChanges {
			return m.executeCreateWithChanges(wt, currentBranch, newBranch, targetPath), true
		}
		return m.executeCreateWithoutChanges(currentBranch, newBranch, targetPath), true
	}

	m.currentScreen = screenInput
	return textinput.Blink
}

func (m *Model) executeCreateWithChanges(wt *models.WorktreeInfo, currentBranch, newBranch, targetPath string) tea.Cmd {
	return func() tea.Msg {
		if err := m.ensureWorktreeDir(m.getWorktreeDir()); err != nil {
			return errMsg{err: err}
		}

		// Stash changes with descriptive message
		prevStashHash := m.git.RunGit(m.ctx, []string{"git", "stash", "list", "-1", "--format=%H"}, wt.Path, []int{0}, true, false)
		stashMessage := fmt.Sprintf("git-wt-create move-current: %s", newBranch)
		if !m.git.RunCommandChecked(
			m.ctx,
			[]string{"git", "stash", "push", "-u", "-m", stashMessage},
			wt.Path,
			"Failed to create stash for moving changes",
		) {
			return errMsg{err: fmt.Errorf("failed to create stash for moving changes")}
		}

		newStashHash := m.git.RunGit(m.ctx, []string{"git", "stash", "list", "-1", "--format=%H"}, wt.Path, []int{0}, true, false)
		if newStashHash == "" || newStashHash == prevStashHash {
			return errMsg{err: fmt.Errorf("failed to create stash for moving changes: no new entry created")}
		}

		// Get the stash ref
		stashRef := m.git.RunGit(m.ctx, []string{"git", "stash", "list", "-1", "--format=%gd"}, wt.Path, []int{0}, true, false)
		if stashRef == "" || !strings.HasPrefix(stashRef, "stash@{") {
			// Try to restore stash if we can't get the ref
			m.git.RunCommandChecked(m.ctx, []string{"git", "stash", "pop"}, wt.Path, "Failed to restore stash")
			return errMsg{err: fmt.Errorf("failed to get stash reference")}
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
			return errMsg{err: fmt.Errorf("failed to create worktree %s", newBranch)}
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
			return errMsg{err: fmt.Errorf("failed to apply stash to new worktree")}
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
		return m.runCommandsWithTrust(initCmds, targetPath, env, after)()
	}
}

func (m *Model) executeCreateWithoutChanges(currentBranch, newBranch, targetPath string) tea.Cmd {
	return func() tea.Msg {
		if err := m.ensureWorktreeDir(m.getWorktreeDir()); err != nil {
			return errMsg{err: err}
		}

		args := []string{"git", "worktree", "add", "-b", newBranch, targetPath, currentBranch}
		if !m.git.RunCommandChecked(m.ctx, args, "", fmt.Sprintf("Failed to create worktree %s", newBranch)) {
			return errMsg{err: fmt.Errorf("failed to create worktree %s", newBranch)}
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
		return m.runCommandsWithTrust(initCmds, targetPath, env, after)()
	}
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
	m.confirmAction = m.deleteWorktreeOnlyCmd(wt)
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
	// Route to appropriate diff viewer based on configuration
	if strings.Contains(m.config.GitPager, "code") {
		return m.showDiffVSCode()
	}
	if m.config.GitPagerInteractive {
		return m.showDiffInteractive()
	}
	return m.showDiffNonInteractive()
}

func (m *Model) showDiffInteractive() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	wt := m.filteredWts[m.selectedIndex]

	// Check if there are any changes to show
	if len(m.statusFilesAll) == 0 {
		m.showInfo("No diff to show.", nil)
		return nil
	}

	// Build environment variables
	env := m.buildCommandEnv(wt.Branch, wt.Path)
	envVars := os.Environ()
	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	// For interactive mode, just pipe git diff directly to the interactive tool
	// NO piping to less - the interactive tool needs terminal control
	gitPagerArgs := ""
	if len(m.config.GitPagerArgs) > 0 {
		gitPagerArgs = " " + strings.Join(m.config.GitPagerArgs, " ")
	}
	cmdStr := fmt.Sprintf("git diff --patch --no-color | %s%s", m.config.GitPager, gitPagerArgs)

	// #nosec G204 -- command constructed from config and controlled inputs
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

func (m *Model) showDiffVSCode() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	wt := m.filteredWts[m.selectedIndex]

	// Check if there are any changes to show
	if len(m.statusFilesAll) == 0 {
		m.showInfo("No diff to show.", nil)
		return nil
	}

	// Build environment variables
	env := m.buildCommandEnv(wt.Branch, wt.Path)
	envVars := os.Environ()
	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	// Use git difftool with VS Code - git handles before/after file extraction
	cmdStr := "git difftool --no-prompt --extcmd='code --wait --diff'"

	// #nosec G204 -- command constructed from controlled input
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

func (m *Model) showDiffNonInteractive() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	wt := m.filteredWts[m.selectedIndex]

	// Check if there are any changes to show
	if len(m.statusFilesAll) == 0 {
		m.showInfo("No diff to show.", nil)
		return nil
	}

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

	// Pipe through git_pager if configured, then through pager
	var cmdStr string
	if m.git.UseGitPager() {
		gitPagerArgs := strings.Join(m.config.GitPagerArgs, " ")
		cmdStr = fmt.Sprintf("set -o pipefail; (%s) | %s %s | %s", script, m.config.GitPager, gitPagerArgs, pagerCmd)
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
			// Ignore exit status 141 (SIGPIPE) which happens when the pager is closed early
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 141 {
				return refreshCompleteMsg{}
			}
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

	// Pipe through git_pager if configured, then through pager
	var cmdStr string
	if m.git.UseGitPager() {
		gitPagerArgs := strings.Join(m.config.GitPagerArgs, " ")
		cmdStr = fmt.Sprintf("set -o pipefail; (%s) | %s %s | %s", script, m.config.GitPager, gitPagerArgs, pagerCmd)
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
			// Ignore exit status 141 (SIGPIPE) which happens when the pager is closed early
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 141 {
				return refreshCompleteMsg{}
			}
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
	m.inputSubmit = func(value string, checked bool) (tea.Cmd, bool) {
		newBranch := strings.TrimSpace(value)
		newBranch = sanitizeBranchNameFromTitle(newBranch, "")
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
	m.inputSubmit = func(value string, checked bool) (tea.Cmd, bool) {
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
	if !m.git.IsGitHubOrGitLab(m.ctx) {
		return m.performMergedWorktreeCheck()
	}

	m.checkMergedAfterPRRefresh = true
	m.ciCache = make(map[string]*ciCacheEntry)
	m.prDataLoaded = false
	m.updateTable()
	m.updateTableColumns(m.worktreeTable.Width())
	m.loading = true
	m.loadingScreen = NewLoadingScreen("Fetching PR data...", m.theme)
	m.currentScreen = screenLoading
	return m.fetchPRData()
}

func (m *Model) performMergedWorktreeCheck() tea.Cmd {
	mainBranch := m.git.GetMainBranch(m.ctx)

	wtBranches := make(map[string]*models.WorktreeInfo)
	for _, wt := range m.worktrees {
		if !wt.IsMain {
			wtBranches[wt.Branch] = wt
		}
	}

	// Track source for each candidate: "pr", "git", or "both"
	type candidate struct {
		wt     *models.WorktreeInfo
		source string
	}
	candidateMap := make(map[string]candidate)

	// 1. PR-based detection (existing logic)
	for _, wt := range m.worktrees {
		if wt.IsMain {
			continue
		}
		if wt.PR != nil && strings.EqualFold(wt.PR.State, "MERGED") {
			candidateMap[wt.Branch] = candidate{wt: wt, source: "pr"}
		}
	}

	// 2. Git-based detection
	mergedBranches := m.git.GetMergedBranches(m.ctx, mainBranch)
	for _, branch := range mergedBranches {
		if wt, exists := wtBranches[branch]; exists {
			if existing, found := candidateMap[branch]; found {
				existing.source = "both"
				candidateMap[branch] = existing
			} else {
				candidateMap[branch] = candidate{wt: wt, source: "git"}
			}
		}
	}

	if len(candidateMap) == 0 {
		m.showInfo("No merged worktrees to prune.", nil)
		return nil
	}

	// Build checklist items (pre-check clean worktrees, uncheck dirty ones)
	items := make([]ChecklistItem, 0, len(candidateMap))
	for branch, info := range candidateMap {
		// Get worktree name from path
		wtName := filepath.Base(info.wt.Path)

		var sourceLabel string
		switch info.source {
		case "pr":
			sourceLabel = "PR merged"
		case "git":
			sourceLabel = "branch merged"
		default:
			sourceLabel = "PR + branch merged"
		}

		desc := fmt.Sprintf("Branch: %s (%s)", branch, sourceLabel)

		// Check for uncommitted changes
		hasDirtyChanges := info.wt.Dirty || info.wt.Untracked > 0 || info.wt.Modified > 0 || info.wt.Staged > 0
		if hasDirtyChanges {
			desc += " - HAS UNCOMMITTED CHANGES!"
		}

		items = append(items, ChecklistItem{
			ID:          branch,
			Label:       wtName,
			Description: desc,
			Checked:     !hasDirtyChanges, // Uncheck dirty worktrees by default
		})
	}

	// Sort items for consistent ordering
	sort.Slice(items, func(i, j int) bool {
		return items[i].Label < items[j].Label
	})

	m.checklistScreen = NewChecklistScreen(
		items,
		"Prune Merged Worktrees",
		"Filter...",
		"No merged worktrees found.",
		m.windowWidth,
		m.windowHeight,
		m.theme,
	)

	m.checklistSubmit = func(selected []ChecklistItem) tea.Cmd {
		if len(selected) == 0 {
			return nil
		}

		// Collect worktrees to prune based on selection
		toPrune := make([]*models.WorktreeInfo, 0, len(selected))
		for _, item := range selected {
			if wt, exists := wtBranches[item.ID]; exists {
				toPrune = append(toPrune, wt)
			}
		}

		// Collect terminate commands once (same for all worktrees in this repo)
		terminateCmds := m.collectTerminateCommands()

		// Build the prune routine that runs terminate commands per-worktree
		pruneRoutine := func() tea.Msg {
			pruned := 0
			failed := 0
			for _, wt := range toPrune {
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
	m.currentScreen = screenChecklist
	return textinput.Blink
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

func (m *Model) buildMRUPaletteItems() []paletteItem {
	if !m.config.PaletteMRU || len(m.paletteHistory) == 0 {
		return nil
	}

	// Build a lookup map of all available palette items
	itemMap := make(map[string]paletteItem)
	customItems := m.customPaletteItems()

	// Add all standard palette items
	standardItems := []paletteItem{
		// Worktree Actions
		{id: "create", label: "Create worktree (c)", description: "Add a new worktree from base branch or PR/MR"},
		{id: "delete", label: "Delete worktree (D)", description: "Remove worktree and branch"},
		{id: "rename", label: "Rename worktree (m)", description: "Rename worktree and branch"},
		{id: "absorb", label: "Absorb worktree (A)", description: "Merge branch into main and remove worktree"},
		{id: "prune", label: "Prune merged (X)", description: "Remove merged PR worktrees"},

		// Create Shortcuts
		{id: "create-from-current", label: "Create worktree from current branch", description: "Create from current branch with or without changes"},
		{id: "create-from-branch", label: "Create worktree from branch/tag", description: "Select a branch, tag, or remote as base"},
		{id: "create-from-commit", label: "Create worktree from commit", description: "Choose a branch, then select a specific commit"},
		{id: "create-from-pr", label: "Create worktree from PR/MR", description: "Create from a pull/merge request"},
		{id: "create-from-issue", label: "Create worktree from issue", description: "Create from a GitHub/GitLab issue"},
		{id: "create-freeform", label: "Create worktree from ref", description: "Enter a branch, tag, or commit manually"},

		// Git Operations
		{id: "diff", label: "Show diff (d)", description: "Show diff for current worktree or commit"},
		{id: "refresh", label: "Refresh (r)", description: "Reload worktrees"},
		{id: "fetch", label: "Fetch remotes (R)", description: "git fetch --all"},
		{id: "push", label: "Push to upstream (P)", description: "git push (clean worktree only)"},
		{id: "sync", label: "Synchronise with upstream (S)", description: "git pull, then git push (clean worktree only)"},
		{id: "fetch-pr-data", label: "Fetch PR data (p)", description: "Fetch PR/MR status from GitHub/GitLab"},
		{id: "pr", label: "Open PR (o)", description: "Open PR in browser"},
		{id: "lazygit", label: "Open LazyGit (g)", description: "Open LazyGit in selected worktree"},
		{id: "run-command", label: "Run command (!)", description: "Run arbitrary command in worktree"},

		// Status Pane
		{id: "stage-file", label: "Stage/unstage file (s)", description: "Stage or unstage selected file"},
		{id: "commit-staged", label: "Commit staged (c)", description: "Commit staged changes"},
		{id: "commit-all", label: "Stage all and commit (C)", description: "Stage all changes and commit"},
		{id: "edit-file", label: "Edit file (e)", description: "Open selected file in editor"},
		{id: "delete-file", label: "Delete file (D)", description: "Delete selected file or directory"},

		// Log Pane
		{id: "cherry-pick", label: "Cherry-pick commit (C)", description: "Cherry-pick commit to another worktree"},
		{id: "commit-view", label: "Browse commit files", description: "Browse files changed in selected commit"},

		// Navigation
		{id: "zoom-toggle", label: "Toggle zoom (=)", description: "Toggle zoom on focused pane"},
		{id: "filter", label: "Filter (f)", description: "Filter items in focused pane"},
		{id: "search", label: "Search (/)", description: "Search items in focused pane"},
		{id: "focus-worktrees", label: "Focus worktrees (1)", description: "Focus worktree pane"},
		{id: "focus-status", label: "Focus status (2)", description: "Focus status pane"},
		{id: "focus-log", label: "Focus log (3)", description: "Focus log pane"},
		{id: "sort-cycle", label: "Cycle sort (s)", description: "Cycle sort mode (path/active/switched)"},

		// Settings
		{id: "theme", label: "Select theme", description: "Change the application theme with live preview"},
		{id: "help", label: "Help (?)", description: "Show help"},
	}

	for _, item := range standardItems {
		if item.id != "" {
			itemMap[item.id] = item
		}
	}

	// Add custom items to the map
	for _, item := range customItems {
		if item.id != "" && !item.isSection {
			itemMap[item.id] = item
		}
	}

	// Build MRU list from history
	mruItems := make([]paletteItem, 0, m.config.PaletteMRULimit)
	for _, usage := range m.paletteHistory {
		if len(mruItems) >= m.config.PaletteMRULimit {
			break
		}

		// Look up the item details
		if item, exists := itemMap[usage.ID]; exists {
			// Mark as MRU and add to list
			item.isMRU = true
			mruItems = append(mruItems, item)
		}
	}

	return mruItems
}

func (m *Model) showCommandPalette() tea.Cmd {
	m.debugf("open palette")
	customItems := m.customPaletteItems()
	items := make([]paletteItem, 0, 40+len(customItems))

	// Build MRU section and track which items are in it
	mruIDs := make(map[string]bool)
	m.debugf("palette MRU: enabled=%v, history_len=%d", m.config.PaletteMRU, len(m.paletteHistory))
	if m.config.PaletteMRU && len(m.paletteHistory) > 0 {
		mruItems := m.buildMRUPaletteItems()
		m.debugf("palette MRU: built %d items", len(mruItems))
		if len(mruItems) > 0 {
			items = append(items, paletteItem{label: "Recently Used", isSection: true})
			items = append(items, mruItems...)
			// Track MRU IDs to exclude from other sections
			for _, item := range mruItems {
				if item.id != "" {
					mruIDs[item.id] = true
				}
			}
		}
	}

	// Helper to add item only if not in MRU
	addItem := func(item paletteItem) {
		if item.id == "" || !mruIDs[item.id] {
			items = append(items, item)
		}
	}

	// Section: Worktree Actions
	items = append(items, paletteItem{label: "Worktree Actions", isSection: true})
	addItem(paletteItem{id: "create", label: "Create worktree (c)", description: "Add a new worktree from base branch or PR/MR"})
	addItem(paletteItem{id: "delete", label: "Delete worktree (D)", description: "Remove worktree and branch"})
	addItem(paletteItem{id: "rename", label: "Rename worktree (m)", description: "Rename worktree and branch"})
	addItem(paletteItem{id: "absorb", label: "Absorb worktree (A)", description: "Merge branch into main and remove worktree"})
	addItem(paletteItem{id: "prune", label: "Prune merged (X)", description: "Remove merged PR worktrees"})

	// Section: Create Shortcuts
	items = append(items, paletteItem{label: "Create Shortcuts", isSection: true})
	addItem(paletteItem{id: "create-from-current", label: "Create worktree from current branch", description: "Create from current branch with or without changes"})
	addItem(paletteItem{id: "create-from-branch", label: "Create worktree from branch/tag", description: "Select a branch, tag, or remote as base"})
	addItem(paletteItem{id: "create-from-commit", label: "Create worktree from commit", description: "Choose a branch, then select a specific commit"})
	addItem(paletteItem{id: "create-from-pr", label: "Create worktree from PR/MR", description: "Create from a pull/merge request"})
	addItem(paletteItem{id: "create-from-issue", label: "Create worktree from issue", description: "Create from a GitHub/GitLab issue"})
	addItem(paletteItem{id: "create-freeform", label: "Create worktree from ref", description: "Enter a branch, tag, or commit manually"})

	// Section: Git Operations
	items = append(items, paletteItem{label: "Git Operations", isSection: true})
	addItem(paletteItem{id: "diff", label: "Show diff (d)", description: "Show diff for current worktree or commit"})
	addItem(paletteItem{id: "refresh", label: "Refresh (r)", description: "Reload worktrees"})
	addItem(paletteItem{id: "fetch", label: "Fetch remotes (R)", description: "git fetch --all"})
	addItem(paletteItem{id: "push", label: "Push to upstream (P)", description: "git push (clean worktree only)"})
	addItem(paletteItem{id: "sync", label: "Synchronise with upstream (S)", description: "git pull, then git push (clean worktree only)"})
	addItem(paletteItem{id: "fetch-pr-data", label: "Fetch PR data (p)", description: "Fetch PR/MR status from GitHub/GitLab"})
	addItem(paletteItem{id: "pr", label: "Open PR (o)", description: "Open PR in browser"})
	addItem(paletteItem{id: "lazygit", label: "Open LazyGit (g)", description: "Open LazyGit in selected worktree"})
	addItem(paletteItem{id: "run-command", label: "Run command (!)", description: "Run arbitrary command in worktree"})

	// Section: Status Pane
	items = append(items, paletteItem{label: "Status Pane", isSection: true})
	addItem(paletteItem{id: "stage-file", label: "Stage/unstage file (s)", description: "Stage or unstage selected file"})
	addItem(paletteItem{id: "commit-staged", label: "Commit staged (c)", description: "Commit staged changes"})
	addItem(paletteItem{id: "commit-all", label: "Stage all and commit (C)", description: "Stage all changes and commit"})
	addItem(paletteItem{id: "edit-file", label: "Edit file (e)", description: "Open selected file in editor"})
	addItem(paletteItem{id: "delete-file", label: "Delete file (D)", description: "Delete selected file or directory"})

	// Section: Log Pane
	items = append(items, paletteItem{label: "Log Pane", isSection: true})
	addItem(paletteItem{id: "cherry-pick", label: "Cherry-pick commit (C)", description: "Cherry-pick commit to another worktree"})
	addItem(paletteItem{id: "commit-view", label: "Browse commit files", description: "Browse files changed in selected commit"})

	// Section: Navigation
	items = append(items, paletteItem{label: "Navigation", isSection: true})
	addItem(paletteItem{id: "zoom-toggle", label: "Toggle zoom (=)", description: "Toggle zoom on focused pane"})
	addItem(paletteItem{id: "filter", label: "Filter (f)", description: "Filter items in focused pane"})
	addItem(paletteItem{id: "search", label: "Search (/)", description: "Search items in focused pane"})
	addItem(paletteItem{id: "focus-worktrees", label: "Focus worktrees (1)", description: "Focus worktree pane"})
	addItem(paletteItem{id: "focus-status", label: "Focus status (2)", description: "Focus status pane"})
	addItem(paletteItem{id: "focus-log", label: "Focus log (3)", description: "Focus log pane"})
	addItem(paletteItem{id: "sort-cycle", label: "Cycle sort (s)", description: "Cycle sort mode (path/active/switched)"})

	// Section: Settings
	items = append(items, paletteItem{label: "Settings", isSection: true})
	addItem(paletteItem{id: "theme", label: "Select theme", description: "Change the application theme with live preview"})
	addItem(paletteItem{id: "help", label: "Help (?)", description: "Show help"})

	// Add custom items (filter out MRU duplicates)
	for _, item := range customItems {
		if item.id == "" || !mruIDs[item.id] {
			items = append(items, item)
		}
	}

	m.paletteScreen = NewCommandPaletteScreen(items, m.windowWidth, m.windowHeight, m.theme)
	m.paletteSubmit = func(action string) tea.Cmd {
		m.debugf("palette action: %s", action)

		// Track usage for MRU
		m.addToPaletteHistory(action)

		// Handle tmux active session attachment
		if strings.HasPrefix(action, "tmux-attach:") {
			sessionName := strings.TrimPrefix(action, "tmux-attach:")
			insideTmux := os.Getenv("TMUX") != ""
			// Use worktree prefix when attaching (sessions are stored with prefix)
			fullSessionName := m.config.SessionPrefix + sessionName
			return m.attachTmuxSessionCmd(fullSessionName, insideTmux)
		}

		// Handle zellij active session attachment
		if strings.HasPrefix(action, "zellij-attach:") {
			sessionName := strings.TrimPrefix(action, "zellij-attach:")
			// Use worktree prefix when attaching (sessions are stored with prefix)
			fullSessionName := m.config.SessionPrefix + sessionName
			return m.attachZellijSessionCmd(fullSessionName)
		}

		if _, ok := m.config.CustomCommands[action]; ok {
			return m.executeCustomCommand(action)
		}
		switch action {
		// Worktree Actions
		case "create":
			return m.showCreateWorktree()
		case "delete":
			return m.showDeleteWorktree()
		case "rename":
			return m.showRenameWorktree()
		case "absorb":
			return m.showAbsorbWorktree()
		case "prune":
			return m.showPruneMerged()

		// Create Menu Shortcuts
		case "create-from-current":
			return m.showCreateFromCurrent()
		case "create-from-branch":
			defaultBase := m.git.GetMainBranch(m.ctx)
			return m.showBranchSelection(
				"Select base branch",
				"Filter branches...",
				"No branches found.",
				defaultBase,
				func(branch string) tea.Cmd {
					suggestedName := stripRemotePrefix(branch)
					return m.showBranchNameInput(branch, suggestedName)
				},
			)
		case "create-from-commit":
			defaultBase := m.git.GetMainBranch(m.ctx)
			return m.showCommitSelection(defaultBase)
		case "create-from-pr":
			return m.showCreateFromPR()
		case "create-from-issue":
			return m.showCreateFromIssue()
		case "create-freeform":
			defaultBase := m.git.GetMainBranch(m.ctx)
			return m.showFreeformBaseInput(defaultBase)

		// Git Operations
		case "diff":
			return m.showDiff()
		case "refresh":
			return m.refreshWorktrees()
		case "fetch":
			return m.fetchRemotes()
		case "push":
			return m.pushToUpstream()
		case "sync":
			return m.syncWithUpstream()
		case "fetch-pr-data":
			m.ciCache = make(map[string]*ciCacheEntry)
			m.prDataLoaded = false
			m.updateTable()
			m.updateTableColumns(m.worktreeTable.Width())
			m.loading = true
			m.statusContent = "Fetching PR data..."
			m.loadingScreen = NewLoadingScreen("Fetching PR data...", m.theme)
			m.currentScreen = screenLoading
			return m.fetchPRData()
		case "pr":
			return m.openPR()
		case "lazygit":
			return m.openLazyGit()
		case "run-command":
			return m.showRunCommand()

		// Status Pane Actions
		case "stage-file":
			if len(m.statusTreeFlat) > 0 && m.statusTreeIndex >= 0 && m.statusTreeIndex < len(m.statusTreeFlat) {
				node := m.statusTreeFlat[m.statusTreeIndex]
				if node.IsDir() {
					return m.stageDirectory(node)
				}
				return m.stageCurrentFile(*node.File)
			}
			return nil
		case "commit-staged":
			return m.commitStagedChanges()
		case "commit-all":
			return m.commitAllChanges()
		case "edit-file":
			if len(m.statusTreeFlat) > 0 && m.statusTreeIndex >= 0 && m.statusTreeIndex < len(m.statusTreeFlat) {
				node := m.statusTreeFlat[m.statusTreeIndex]
				if !node.IsDir() {
					return m.openStatusFileInEditor(*node.File)
				}
			}
			return nil
		case "delete-file":
			return m.showDeleteFile()

		// Log Pane Actions
		case "cherry-pick":
			return m.showCherryPick()
		case "commit-view":
			return m.openCommitView()

		// Navigation & View
		case "zoom-toggle":
			if m.zoomedPane >= 0 {
				m.zoomedPane = -1
			} else {
				m.zoomedPane = m.focusedPane
			}
			return nil
		case "filter":
			target := filterTargetWorktrees
			switch m.focusedPane {
			case 1:
				target = filterTargetStatus
			case 2:
				target = filterTargetLog
			}
			return m.startFilter(target)
		case "search":
			target := searchTargetWorktrees
			switch m.focusedPane {
			case 1:
				target = searchTargetStatus
			case 2:
				target = searchTargetLog
			}
			return m.startSearch(target)
		case "focus-worktrees":
			m.zoomedPane = -1
			m.focusedPane = 0
			m.worktreeTable.Focus()
			return nil
		case "focus-status":
			m.zoomedPane = -1
			m.focusedPane = 1
			m.rebuildStatusContentWithHighlight()
			return nil
		case "focus-log":
			m.zoomedPane = -1
			m.focusedPane = 2
			m.logTable.Focus()
			return nil
		case "sort-cycle":
			m.sortMode = (m.sortMode + 1) % 3
			m.updateTable()
			return nil

		// Settings & Help
		case "theme":
			return m.showThemeSelection()
		case "help":
			m.currentScreen = screenHelp
			return nil
		}
		return nil
	}
	m.currentScreen = screenPalette
	return textinput.Blink
}

func (m *Model) showThemeSelection() tea.Cmd {
	m.originalTheme = m.config.Theme
	themes := theme.AvailableThemes()
	sort.Strings(themes)
	items := make([]selectionItem, 0, len(themes))
	for _, t := range themes {
		items = append(items, selectionItem{id: t, label: t})
	}
	m.listScreen = NewListSelectionScreen(items, "🎨 Select Theme", "Filter themes...", "", m.windowWidth, m.windowHeight, m.originalTheme, m.theme)
	m.listScreen.onCursorChange = func(item selectionItem) {
		m.UpdateTheme(item.id)
	}
	m.listSubmit = func(item selectionItem) tea.Cmd {
		m.listScreen = nil
		m.listSubmit = nil

		// Ask for confirmation before saving to config
		m.confirmScreen = NewConfirmScreen(fmt.Sprintf("Save theme '%s' to config file?", item.id), m.theme)
		m.confirmAction = func() tea.Cmd {
			m.config.Theme = item.id
			if err := config.SaveConfig(m.config); err != nil {
				m.debugf("failed to save config: %v", err)
			}
			m.originalTheme = ""
			return nil
		}
		m.currentScreen = screenConfirm
		return nil
	}
	m.currentScreen = screenListSelect
	return textinput.Blink
}

func (m *Model) customPaletteItems() []paletteItem {
	keys := m.customCommandKeys()
	if len(keys) == 0 {
		return nil
	}

	// Separate commands into categories
	var regularItems, tmuxItems, zellijItems []paletteItem
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
		item := paletteItem{
			id:          key,
			label:       label,
			description: description,
		}
		switch {
		case cmd.Tmux != nil:
			tmuxItems = append(tmuxItems, item)
		case cmd.Zellij != nil:
			zellijItems = append(zellijItems, item)
		default:
			regularItems = append(regularItems, item)
		}
	}

	// Check if tmux/zellij are available
	_, tmuxErr := exec.LookPath("tmux")
	_, zellijErr := exec.LookPath("zellij")
	hasTmux := len(tmuxItems) > 0 && tmuxErr == nil
	hasZellij := len(zellijItems) > 0 && zellijErr == nil

	// Get active tmux sessions
	var activeTmuxSessions []paletteItem
	if tmuxErr == nil {
		sessions := m.getTmuxActiveSessions()
		for _, sessionName := range sessions {
			activeTmuxSessions = append(activeTmuxSessions, paletteItem{
				id:          "tmux-attach:" + sessionName,
				label:       sessionName,
				description: "active tmux session",
			})
		}
	}

	// Get active zellij sessions
	var activeZellijSessions []paletteItem
	if zellijErr == nil {
		sessions := m.getZellijActiveSessions()
		for _, sessionName := range sessions {
			activeZellijSessions = append(activeZellijSessions, paletteItem{
				id:          "zellij-attach:" + sessionName,
				label:       sessionName,
				description: "active zellij session",
			})
		}
	}

	// Build result with sections
	var items []paletteItem
	if len(regularItems) > 0 {
		items = append(items, paletteItem{label: "Custom Commands", isSection: true})
		items = append(items, regularItems...)
	}

	// Multiplexer section for custom tmux/zellij commands
	if hasTmux || hasZellij {
		items = append(items, paletteItem{label: "Multiplexer", isSection: true})
		if hasTmux {
			items = append(items, tmuxItems...)
		}
		if hasZellij {
			items = append(items, zellijItems...)
		}
	}

	// Active Tmux Sessions section (appears after Multiplexer)
	if len(activeTmuxSessions) > 0 {
		items = append(items, paletteItem{label: "Active Tmux Sessions", isSection: true})
		items = append(items, activeTmuxSessions...)
	}

	// Active Zellij Sessions section (appears after Active Tmux Sessions)
	if len(activeZellijSessions) > 0 {
		items = append(items, paletteItem{label: "Active Zellij Sessions", isSection: true})
		items = append(items, activeZellijSessions...)
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

func (m *Model) deleteWorktreeOnlyCmd(wt *models.WorktreeInfo) func() tea.Cmd {
	env := m.buildCommandEnv(wt.Branch, wt.Path)
	terminateCmds := m.collectTerminateCommands()

	afterCmd := func() tea.Msg {
		// Only remove worktree
		success := m.git.RunCommandChecked(
			m.ctx,
			[]string{"git", "worktree", "remove", "--force", wt.Path},
			"",
			fmt.Sprintf("Failed to remove worktree %s", wt.Path),
		)

		if !success {
			return worktreeDeletedMsg{
				path:   wt.Path,
				branch: wt.Branch,
				err:    fmt.Errorf("worktree deletion failed"),
			}
		}

		return worktreeDeletedMsg{
			path:   wt.Path,
			branch: wt.Branch,
			err:    nil,
		}
	}

	return func() tea.Cmd {
		return m.runCommandsWithTrust(terminateCmds, wt.Path, env, afterCmd)
	}
}

func (m *Model) deleteBranchCmd(branch string) func() tea.Cmd {
	return func() tea.Cmd {
		return func() tea.Msg {
			m.git.RunCommandChecked(
				m.ctx,
				[]string{"git", "branch", "-D", branch},
				"",
				fmt.Sprintf("Failed to delete branch %s", branch),
			)

			worktrees, err := m.git.GetWorktrees(m.ctx)
			return worktreesLoadedMsg{
				worktrees: worktrees,
				err:       err,
			}
		}
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
	envVars := filterWorktreeEnvVars(os.Environ())
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
	envVars := filterWorktreeEnvVars(os.Environ())
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
			// Ignore exit status 141 (SIGPIPE) which happens when the pager is closed early
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 141 {
				return refreshCompleteMsg{}
			}
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
	envVars := filterWorktreeEnvVars(os.Environ())
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
			// Ignore exit status 141 (SIGPIPE) which happens when the pager is closed early
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 141 {
				return refreshCompleteMsg{}
			}
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
		sessionName = fmt.Sprintf("%s%s", m.config.SessionPrefix, filepath.Base(wt.Path))
	}
	sessionName = sanitizeTmuxSessionName(sessionName)

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
		sessionName = fmt.Sprintf("%s%s", m.config.SessionPrefix, filepath.Base(wt.Path))
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
	if strings.Contains(m.config.GitPager, "code") {
		return m.showCommitDiffVSCode(commitSHA, wt)
	}
	if m.config.GitPagerInteractive {
		return m.showCommitDiffInteractive(commitSHA, wt)
	}
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

	// Pipe through git_pager if configured, then through pager
	// Note: delta only processes the diff part, so our colorized commit message will pass through
	// Don't use pipefail here as awk might not always match (e.g., if commit format is different)
	var cmdStr string
	if m.git.UseGitPager() {
		gitPagerArgs := strings.Join(m.config.GitPagerArgs, " ")
		cmdStr = fmt.Sprintf("%s | %s %s | %s", gitCmd, m.config.GitPager, gitPagerArgs, pagerCmd)
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
	if strings.Contains(m.config.GitPager, "code") {
		return m.showCommitFileDiffVSCode(commitSHA, filename, worktreePath)
	}
	if m.config.GitPagerInteractive {
		return m.showCommitFileDiffInteractive(commitSHA, filename, worktreePath)
	}
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

	// Pipe through git_pager if configured, then through pager
	var cmdStr string
	if m.git.UseGitPager() {
		gitPagerArgs := strings.Join(m.config.GitPagerArgs, " ")
		cmdStr = fmt.Sprintf("%s | %s %s | %s", gitCmd, m.config.GitPager, gitPagerArgs, pagerCmd)
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

func (m *Model) showCommitDiffInteractive(commitSHA string, wt *models.WorktreeInfo) tea.Cmd {
	// Build environment variables
	env := m.buildCommandEnv(wt.Branch, wt.Path)
	envVars := filterWorktreeEnvVars(os.Environ())
	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	gitPagerArgs := ""
	if len(m.config.GitPagerArgs) > 0 {
		gitPagerArgs = " " + strings.Join(m.config.GitPagerArgs, " ")
	}
	gitCmd := fmt.Sprintf("git show --patch --no-color %s", commitSHA)
	cmdStr := fmt.Sprintf("%s | %s%s", gitCmd, m.config.GitPager, gitPagerArgs)

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

func (m *Model) showCommitFileDiffInteractive(commitSHA, filename, worktreePath string) tea.Cmd {
	// Build environment variables for pager
	envVars := os.Environ()

	gitPagerArgs := ""
	if len(m.config.GitPagerArgs) > 0 {
		gitPagerArgs = " " + strings.Join(m.config.GitPagerArgs, " ")
	}
	gitCmd := fmt.Sprintf("git show --patch --no-color %s -- %q", commitSHA, filename)
	cmdStr := fmt.Sprintf("%s | %s%s", gitCmd, m.config.GitPager, gitPagerArgs)

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

func (m *Model) showCommitDiffVSCode(commitSHA string, wt *models.WorktreeInfo) tea.Cmd {
	// Build environment variables
	env := m.buildCommandEnv(wt.Branch, wt.Path)
	envVars := filterWorktreeEnvVars(os.Environ())
	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	// Use git difftool to compare parent commit with this commit
	cmdStr := fmt.Sprintf("git difftool %s^..%s --no-prompt --extcmd='code --wait --diff'", commitSHA, commitSHA)

	// #nosec G204 -- command constructed from controlled input
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

func (m *Model) showCommitFileDiffVSCode(commitSHA, filename, worktreePath string) tea.Cmd {
	envVars := filterWorktreeEnvVars(os.Environ())
	envVars = append(envVars, fmt.Sprintf("WORKTREE_PATH=%s", worktreePath))

	// Use git difftool to compare the specific file between parent and this commit
	cmdStr := fmt.Sprintf("git difftool %s^..%s --no-prompt --extcmd='code --wait --diff' -- %s",
		commitSHA, commitSHA, shellQuote(filename))

	// #nosec G204 -- command constructed from controlled input
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
					if m.currentScreen == screenPalette {
						m.currentScreen = screenNone
					}
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
			if m.originalTheme != "" {
				m.UpdateTheme(m.originalTheme)
				m.originalTheme = ""
			}
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
	case screenChecklist:
		if m.checklistScreen == nil {
			m.currentScreen = screenNone
			return m, nil
		}
		keyStr := msg.String()
		if isEscKey(keyStr) {
			m.checklistScreen = nil
			m.checklistSubmit = nil
			m.currentScreen = screenNone
			return m, nil
		}
		if keyStr == keyEnter {
			if m.checklistSubmit != nil {
				selected := m.checklistScreen.SelectedItems()
				cmd := m.checklistSubmit(selected)
				m.checklistScreen = nil
				m.checklistSubmit = nil
				m.currentScreen = screenNone
				return m, cmd
			}
		}
		cs, cmd := m.checklistScreen.Update(msg)
		if updated, ok := cs.(*ChecklistScreen); ok {
			m.checklistScreen = updated
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
			m.commitFilesScreen.filterInput.Placeholder = searchFiles
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
					m.confirmCancel = nil
					if m.currentScreen == screenConfirm {
						m.currentScreen = screenNone
					}
					if actionCmd != nil {
						return m, actionCmd
					}
					return m, nil
				} else {
					// Action cancelled
					var cancelCmd tea.Cmd
					if m.confirmCancel != nil {
						cancelCmd = m.confirmCancel()
					}
					m.confirmScreen = nil
					m.confirmAction = nil
					m.confirmCancel = nil
					m.currentScreen = screenNone
					if cancelCmd != nil {
						return m, cancelCmd
					}
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
		case keyStr == keyQ || keyStr == "Q" || keyStr == "enter" || isEscKey(keyStr):
			m.quitting = true
			m.stopGitWatcher()
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
	case screenInput:
		if m.inputScreen == nil {
			m.currentScreen = screenNone
			return m, nil
		}

		keyStr := msg.String()
		if isEscKey(keyStr) {
			// Clear cached state on exit
			m.createFromCurrentDiff = ""
			m.createFromCurrentRandomName = ""
			m.createFromCurrentAIName = ""
			m.createFromCurrentBranch = ""

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
				cmd, closeCmd := m.inputSubmit(m.inputScreen.input.Value(), m.inputScreen.checkboxChecked)
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

		// Store previous checkbox state before update
		prevCheckboxState := m.inputScreen.checkboxChecked

		var cmd tea.Cmd
		_, cmd = m.inputScreen.Update(msg)

		// Detect checkbox state change
		if m.inputScreen.checkboxEnabled && prevCheckboxState != m.inputScreen.checkboxChecked {
			return m, tea.Batch(cmd, m.handleCheckboxToggle())
		}

		return m, cmd
	}
	return m, nil
}

func (m *Model) renderScreen() string {
	switch m.currentScreen {
	case screenCommit:
		if m.commitScreen == nil {
			m.commitScreen = NewCommitScreen(commitMeta{}, "", "", m.git.UseGitPager(), m.theme)
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
		content := m.welcomeScreen.View()
		if m.windowWidth > 0 && m.windowHeight > 0 {
			return lipgloss.Place(m.windowWidth, m.windowHeight, lipgloss.Center, lipgloss.Center, content)
		}
		return content
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

// commandPaletteUsage tracks usage frequency and recency for command palette items.
type commandPaletteUsage struct {
	ID        string `json:"id"`
	Timestamp int64  `json:"timestamp"`
	Count     int    `json:"count"`
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

func (m *Model) loadPaletteHistory() {
	repoKey := m.getRepoKey()
	historyPath := filepath.Join(m.getWorktreeDir(), repoKey, models.CommandPaletteHistoryFilename)
	// #nosec G304 -- historyPath is constructed from vetted worktree directory and constant filename
	data, err := os.ReadFile(historyPath)
	if err != nil {
		m.paletteHistory = []commandPaletteUsage{}
		return
	}

	var payload struct {
		Commands []commandPaletteUsage `json:"commands"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		m.debugf("failed to parse palette history: %v", err)
		m.paletteHistory = []commandPaletteUsage{}
		return
	}

	m.paletteHistory = payload.Commands
	if m.paletteHistory == nil {
		m.paletteHistory = []commandPaletteUsage{}
	}
}

func (m *Model) savePaletteHistory() {
	repoKey := m.getRepoKey()
	historyPath := filepath.Join(m.getWorktreeDir(), repoKey, models.CommandPaletteHistoryFilename)
	if err := os.MkdirAll(filepath.Dir(historyPath), defaultDirPerms); err != nil {
		m.debugf("failed to create palette history dir: %v", err)
		return
	}

	historyData := struct {
		Commands []commandPaletteUsage `json:"commands"`
	}{
		Commands: m.paletteHistory,
	}
	data, _ := json.Marshal(historyData)
	if err := os.WriteFile(historyPath, data, defaultFilePerms); err != nil {
		m.debugf("failed to write palette history: %v", err)
	}
}

func (m *Model) addToPaletteHistory(id string) {
	id = strings.TrimSpace(id)
	if id == "" {
		return
	}

	m.debugf("adding to palette history: %s", id)
	now := time.Now().Unix()

	// Find existing entry and update it
	found := false
	for i, entry := range m.paletteHistory {
		if entry.ID == id {
			m.paletteHistory[i].Timestamp = now
			m.paletteHistory[i].Count++
			// Move to front
			updated := m.paletteHistory[i]
			m.paletteHistory = append([]commandPaletteUsage{updated}, append(m.paletteHistory[:i], m.paletteHistory[i+1:]...)...)
			found = true
			break
		}
	}

	// Add new entry if not found
	if !found {
		m.paletteHistory = append([]commandPaletteUsage{{
			ID:        id,
			Timestamp: now,
			Count:     1,
		}}, m.paletteHistory...)
	}

	// Limit history to 100 entries
	maxHistory := 100
	if len(m.paletteHistory) > maxHistory {
		m.paletteHistory = m.paletteHistory[:maxHistory]
	}

	m.savePaletteHistory()
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
	log.Printf(format, args...)
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

// filterWorktreeEnvVars filters out worktree-specific environment variables
// to prevent duplicates when building command environments.
func filterWorktreeEnvVars(environ []string) []string {
	worktreeVars := map[string]bool{
		"WORKTREE_PATH":      true,
		"MAIN_WORKTREE_PATH": true,
		"WORKTREE_BRANCH":    true,
		"WORKTREE_NAME":      true,
		"REPO_NAME":          true,
	}

	filtered := make([]string, 0, len(environ))
	for _, entry := range environ {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) > 0 && !worktreeVars[parts[0]] {
			filtered = append(filtered, entry)
		}
	}
	return filtered
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
	// Wait for session with timeout (5 seconds max)
	b.WriteString("  tries=0\n")
	b.WriteString("  while ! zellij list-sessions --short 2>/dev/null | grep -Fxq \"$session\"; do\n")
	b.WriteString("    sleep 0.1\n")
	b.WriteString("    tries=$((tries+1))\n")
	b.WriteString("    if [ $tries -ge 50 ]; then echo \"Timeout waiting for zellij session\" >&2; exit 1; fi\n")
	b.WriteString("  done\n")
	b.WriteString("fi\n")
	if len(layoutPaths) > 0 {
		b.WriteString("if [ \"$created\" = \"true\" ]; then\n")
		for _, layoutPath := range layoutPaths {
			b.WriteString(fmt.Sprintf("  ZELLIJ_SESSION_NAME=\"$session\" zellij action new-tab --layout %s\n", shellQuote(layoutPath)))
		}
		b.WriteString("  ZELLIJ_SESSION_NAME=\"$session\" zellij action go-to-tab 1\n")
		b.WriteString("  ZELLIJ_SESSION_NAME=\"$session\" zellij action close-tab\n")
		b.WriteString("fi\n")
	}
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

func sanitizeTmuxSessionName(name string) string {
	if name == "" {
		return ""
	}
	replacer := strings.NewReplacer(":", "-", "/", "-", "\\", "-")
	return replacer.Replace(name)
}

// getTmuxActiveSessions queries tmux for all sessions starting with the configured session prefix
// Returns session names with the prefix stripped, or empty slice if tmux is unavailable.
func (m *Model) getTmuxActiveSessions() []string {
	// Check if tmux is available
	if _, err := exec.LookPath("tmux"); err != nil {
		return nil
	}

	// Query tmux for session list
	// #nosec G204 -- static command with format string
	cmd := m.commandRunner("tmux", "list-sessions", "-F", "#{session_name}")
	output, err := cmd.Output()
	if err != nil {
		// tmux not running or no sessions
		return nil
	}

	// Parse output and filter for worktree session prefix
	var sessions []string
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, m.config.SessionPrefix) {
			// Strip worktree prefix
			sessionName := strings.TrimPrefix(line, m.config.SessionPrefix)
			if sessionName != "" {
				sessions = append(sessions, sessionName)
			}
		}
	}

	// Sort alphabetically for consistent display
	sort.Strings(sessions)
	return sessions
}

// getZellijActiveSessions queries zellij for all sessions starting with the configured session prefix
// Returns session names with the prefix stripped, or empty slice if zellij is unavailable.
func (m *Model) getZellijActiveSessions() []string {
	// Check if zellij is available
	if _, err := exec.LookPath("zellij"); err != nil {
		return nil
	}

	// Query zellij for session list
	// #nosec G204 -- static command with format string
	cmd := m.commandRunner("zellij", "list-sessions", "--short")
	output, err := cmd.Output()
	if err != nil {
		// zellij not running or no sessions
		return nil
	}

	// Parse output and filter for worktree session prefix
	var sessions []string
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, m.config.SessionPrefix) {
			// Strip worktree prefix
			sessionName := strings.TrimPrefix(line, m.config.SessionPrefix)
			if sessionName != "" {
				sessions = append(sessions, sessionName)
			}
		}
	}

	// Sort alphabetically for consistent display
	sort.Strings(sessions)
	return sessions
}

func sanitizeZellijSessionName(name string) string {
	if name == "" {
		return ""
	}
	replacer := strings.NewReplacer("/", "-", "\\", "-", ":", "-")
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
		Foreground(m.theme.AccentFg).
		Bold(true).
		Width(layout.width).
		Padding(0, 2)

	// Add decorative icon to title
	title := "🌲 Lazyworktree"
	repoKey := strings.TrimSpace(m.repoKey)
	content := title
	if repoKey != "" && repoKey != "unknown" && !strings.HasPrefix(repoKey, "local-") {
		content = fmt.Sprintf("%s  •  %s", content, repoKey)
	}

	return headerStyle.Render(content)
}

func (m *Model) renderFilter(layout layoutDims) string {
	labelStyle := lipgloss.NewStyle().
		Foreground(m.theme.AccentFg).
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
			m.renderKeyHint("S", "Sync"),
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

// UpdateTheme updates the application theme and refreshes component styles.
func (m *Model) UpdateTheme(themeName string) {
	thm := theme.GetTheme(themeName)
	m.theme = thm

	// Update table styles
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(thm.BorderDim).
		BorderBottom(true).
		Bold(true).
		Foreground(thm.Cyan).
		Background(thm.AccentDim)
	s.Selected = s.Selected.
		Foreground(thm.AccentFg).
		Background(thm.Accent).
		Bold(true)
	s.Cell = s.Cell.
		Foreground(thm.TextFg)

	m.worktreeTable.SetStyles(s)
	m.logTable.SetStyles(s)

	// Update spinner style
	m.spinner.Style = lipgloss.NewStyle().Foreground(thm.Accent)

	// Update filter input styles
	m.filterInput.PromptStyle = lipgloss.NewStyle().Foreground(thm.Accent)
	m.filterInput.TextStyle = lipgloss.NewStyle().Foreground(thm.TextFg)

	// Update other screens if they exist
	if m.helpScreen != nil {
		m.helpScreen.thm = thm
	}
	if m.confirmScreen != nil {
		m.confirmScreen.thm = thm
	}
	if m.infoScreen != nil {
		m.infoScreen.thm = thm
	}
	if m.loadingScreen != nil {
		m.loadingScreen.thm = thm
	}
	if m.inputScreen != nil {
		m.inputScreen.thm = thm
	}
	if m.paletteScreen != nil {
		m.paletteScreen.thm = thm
	}
	if m.prSelectionScreen != nil {
		m.prSelectionScreen.thm = thm
	}
	if m.issueSelectionScreen != nil {
		m.issueSelectionScreen.thm = thm
	}
	if m.listScreen != nil {
		m.listScreen.thm = thm
	}
	if m.commitFilesScreen != nil {
		m.commitFilesScreen.thm = thm
	}

	// Re-render info content with new theme
	if m.selectedIndex >= 0 && m.selectedIndex < len(m.filteredWts) {
		m.infoContent = m.buildInfoContent(m.filteredWts[m.selectedIndex])
	}
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
		Foreground(m.theme.AccentFg).
		Background(m.theme.Accent).
		Bold(true).
		Padding(0, 1) // Add padding for pill effect
	labelStyle := lipgloss.NewStyle().Foreground(m.theme.MutedFg)
	return fmt.Sprintf("%s %s", keyStyle.Render(key), labelStyle.Render(label))
}

func (m *Model) renderPaneTitle(index int, title string, focused bool, width int) string {
	numStyle := lipgloss.NewStyle().Foreground(m.theme.MutedFg)
	titleStyle := lipgloss.NewStyle().Foreground(m.theme.MutedFg)
	if focused {
		numStyle = numStyle.Foreground(m.theme.Pink).Bold(true)
		titleStyle = titleStyle.Foreground(m.theme.TextFg).Bold(true)
	}
	num := numStyle.Render(fmt.Sprintf("[%d]", index))
	if m.config.ShowIcons {
		num = numStyle.Render(fmt.Sprintf("(%d)", index))
	}
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

	return lipgloss.NewStyle().Width(width).Render(fmt.Sprintf("%s %s%s%s", num, name, filterIndicator, zoomIndicator))
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
		numStyle := lipgloss.NewStyle().Foreground(m.theme.TextFg)
		stateColor := m.theme.SuccessFg // default to success for OPEN
		switch wt.PR.State {
		case "MERGED":
			stateColor = m.theme.Pink
		case "CLOSED":
			stateColor = m.theme.ErrorFg
		}
		stateStyle := lipgloss.NewStyle().Foreground(stateColor)
		// Format: PR: #123 Title [STATE] (matches Python grid layout)
		infoLines = append(infoLines, fmt.Sprintf("%s %s %s [%s]",
			prLabel,
			numStyle.Render(fmt.Sprintf("#%d", wt.PR.Number)),
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
	} else {
		// Show PR status/error when PR is nil
		grayStyle := lipgloss.NewStyle().Foreground(m.theme.MutedFg)
		errorStyle := lipgloss.NewStyle().Foreground(m.theme.ErrorFg)

		infoLines = append(infoLines, "") // blank line

		switch wt.PRFetchStatus {
		case models.PRFetchStatusLoaded:
			// This shouldn't happen (PR is nil but status is loaded) - show debug info
			infoLines = append(infoLines, errorStyle.Render("PR Status: Loaded but nil (bug)"))

		case models.PRFetchStatusError:
			labelStyle := lipgloss.NewStyle().Foreground(m.theme.TextFg).Bold(true)
			infoLines = append(infoLines, labelStyle.Render("PR Status:"))
			infoLines = append(infoLines, errorStyle.Render("  ✗ Fetch failed"))

			// Provide helpful error messages based on error content
			switch {
			case strings.Contains(wt.PRFetchError, "not found") || strings.Contains(wt.PRFetchError, "PATH"):
				infoLines = append(infoLines, grayStyle.Render("  gh/glab CLI not found"))
				infoLines = append(infoLines, grayStyle.Render("  Install from https://cli.github.com"))
			case strings.Contains(wt.PRFetchError, "auth") || strings.Contains(wt.PRFetchError, "401"):
				infoLines = append(infoLines, grayStyle.Render("  Authentication failed"))
				infoLines = append(infoLines, grayStyle.Render("  Run 'gh auth login' or 'glab auth login'"))
			case wt.PRFetchError != "":
				infoLines = append(infoLines, grayStyle.Render(fmt.Sprintf("  %s", wt.PRFetchError)))
			}

		case models.PRFetchStatusNoPR:
			switch {
			case m.prDataLoaded && wt.HasUpstream:
				// Fetch was attempted, no error, no PR found - this is expected
				infoLines = append(infoLines, grayStyle.Render("No PR for this branch"))
			case !m.prDataLoaded:
				// PR data hasn't been fetched yet
				infoLines = append(infoLines, grayStyle.Render("PR data not loaded (press 'p' to fetch)"))
			case !wt.HasUpstream:
				// No upstream, so no PR possible
				infoLines = append(infoLines, grayStyle.Render("Branch has no upstream"))
			}

		case models.PRFetchStatusFetching:
			infoLines = append(infoLines, grayStyle.Render("Fetching PR data..."))

		default:
			// Not fetched yet
			if !m.prDataLoaded {
				infoLines = append(infoLines, grayStyle.Render("Press 'p' to fetch PR data"))
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

func statusCounts(files []StatusFile) (staged, modified, untracked int) {
	for _, file := range files {
		if file.IsUntracked {
			untracked++
			continue
		}
		if file.Status != "" {
			first := file.Status[0]
			if first != '.' && first != ' ' {
				staged++
			}
		}
		if len(file.Status) > 1 {
			second := file.Status[1]
			if second != '.' && second != ' ' {
				modified++
			}
		}
	}
	return staged, modified, untracked
}

func (m *Model) updateWorktreeStatus(path string, files []StatusFile) {
	if path == "" {
		return
	}
	var target *models.WorktreeInfo
	for _, wt := range m.worktrees {
		if wt.Path == path {
			target = wt
			break
		}
	}
	if target == nil {
		return
	}
	staged, modified, untracked := statusCounts(files)
	dirty := staged+modified+untracked > 0
	if target.Dirty == dirty && target.Staged == staged && target.Modified == modified && target.Untracked == untracked {
		return
	}
	target.Dirty = dirty
	target.Staged = staged
	target.Modified = modified
	target.Untracked = untracked
	m.updateTable()
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
	status := 8
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
		{Title: "Changes", Width: status},
		{Title: "Status", Width: ab},
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

// truncateToHeightFromEnd returns the last maxLines lines from the string.
// Useful for git errors where the actual error is at the end.
func truncateToHeightFromEnd(s string, maxLines int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return strings.Join(lines, "\n")
}

func filterNonEmpty(values []string) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			filtered = append(filtered, value)
		}
	}
	return filtered
}
