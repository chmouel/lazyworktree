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
	thm := theme.GetThemeWithCustoms(cfg.Theme, config.CustomThemesToThemeDataMap(cfg.CustomThemes))

	// Initialize icon provider based on config
	switch cfg.IconSet {
	case "none":
		SetIconProvider(&TextProvider{})
	case "emoji":
		SetIconProvider(&EmojiProvider{})
	case "text":
		SetIconProvider(&TextProvider{})
	default:
		SetIconProvider(&NerdFontV3Provider{})
	}

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
	s.Selected = s.Selected.Foreground(thm.Accent)
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(thm.BorderDim).
		BorderBottom(true).
		Bold(true).
		Foreground(thm.Cyan).
		Background(thm.AccentDim) // Add subtle background to header
	s.Selected = s.Selected.Bold(true) // Arrow indicator shows selection, no background
	// Don't set Foreground on Cell - let Selected style's foreground take effect
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
			m.config.IconsEnabled(),
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

// Helper methods

func (m *Model) inputLabel() string {
	if m.showingSearch {
		return m.searchLabel()
	}
	return m.filterLabel()
}

func (m *Model) searchLabel() string {
	showIcons := m.config.IconsEnabled()
	switch m.searchTarget {
	case searchTargetStatus:
		return labelWithIcon(UIIconSearch, "Search Files", showIcons)
	case searchTargetLog:
		return labelWithIcon(UIIconSearch, "Search Commits", showIcons)
	default:
		return labelWithIcon(UIIconSearch, "Search Worktrees", showIcons)
	}
}

func (m *Model) filterLabel() string {
	showIcons := m.config.IconsEnabled()
	switch m.filterTarget {
	case filterTargetStatus:
		return labelWithIcon(UIIconFilter, "Filter Files", showIcons)
	case filterTargetLog:
		return labelWithIcon(UIIconFilter, "Filter Commits", showIcons)
	default:
		return labelWithIcon(UIIconFilter, "Filter Worktrees", showIcons)
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
	showIcons := m.config.IconsEnabled()
	rows := make([]table.Row, 0, len(m.filteredWts))
	for _, wt := range m.filteredWts {
		name := filepath.Base(wt.Path)
		worktreeIcon := UIIconWorktree
		if wt.IsMain {
			worktreeIcon = UIIconWorktreeMain
			name = mainWorktreeName
		}
		if showIcons {
			name = iconPrefix(worktreeIcon, showIcons) + name
		} else {
			name = " " + name
		}

		// Truncate to configured max length with ellipsis if needed
		if m.config.MaxNameLength > 0 {
			nameRunes := []rune(name)
			if len(nameRunes) > m.config.MaxNameLength {
				name = string(nameRunes[:m.config.MaxNameLength]) + "..."
			}
		}

		statusStr := combinedStatusIndicator(wt.Dirty, wt.HasUpstream, wt.Ahead, wt.Behind, wt.Unpushed, showIcons, m.config.IconSet)

		row := table.Row{
			name,
			statusStr,
			wt.LastActive,
		}

		// Only include PR column if PR data has been loaded
		if m.prDataLoaded {
			prStr := "-"
			if wt.PR != nil {
				prIcon := ""
				if showIcons {
					prIcon = iconWithSpace(getIconPR())
				}
				stateSymbol := prStateIndicator(wt.PR.State, showIcons)
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
	m.updateWorktreeArrows()
}

// updateWorktreeArrows updates the arrow indicator on the selected row.
func (m *Model) updateWorktreeArrows() {
	rows := m.worktreeTable.Rows()
	cursor := m.worktreeTable.Cursor()
	for i, row := range rows {
		if len(row) > 0 && row[0] != "" {
			runes := []rune(row[0])
			if len(runes) > 0 {
				// Replace first rune with arrow or space
				if i == cursor {
					runes[0] = '›'
				} else {
					runes[0] = ' '
				}
				rows[i][0] = string(runes)
			}
		}
	}
	m.worktreeTable.SetRows(rows)
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
		log.Printf("FetchPRMap returned %d PRs", len(prMap))
		for branch, pr := range prMap {
			log.Printf("  prMap[%q] = PR#%d", branch, pr.Number)
		}

		// Also fetch PRs per worktree for cases where local branch differs from remote
		// This handles fork PRs where local branch name doesn't match headRefName
		worktreePRs := make(map[string]*models.PRInfo)
		worktreeErrors := make(map[string]string)
		for _, wt := range m.worktrees {
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
		m.config.IconsEnabled(),
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

// showAbsorbWorktree merges selected branch into main and removes the worktree

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
		{id: "ci-checks", label: "View CI checks (v)", description: "View CI check logs for current worktree"},
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
	if m.git != nil && m.git.IsGitHub(m.ctx) {
		addItem(paletteItem{id: "ci-checks", label: "View CI checks (v)", description: "View CI check logs for current worktree"})
	}
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
			m.loadingScreen = NewLoadingScreen("Fetching PR data...", m.theme, m.config.IconsEnabled())
			m.currentScreen = screenLoading
			return m.fetchPRData()
		case "pr":
			return m.openPR()
		case "ci-checks":
			return m.openCICheckSelection()
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
	themes := theme.AvailableThemesWithCustoms(config.CustomThemesToThemeDataMap(m.config.CustomThemes))
	sort.Strings(themes)
	items := make([]selectionItem, 0, len(themes))
	for _, t := range themes {
		items = append(items, selectionItem{id: t, label: t})
	}
	m.listScreen = NewListSelectionScreen(items, labelWithIcon(UIIconThemeSelect, "Select Theme", m.config.IconsEnabled()), "Filter themes...", "", m.windowWidth, m.windowHeight, m.originalTheme, m.theme)
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

// openURLInBrowser opens the given URL in the default browser.
func (m *Model) openURLInBrowser(urlStr string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case osDarwin:
			// #nosec G204 -- the URL is executed directly as a single argument
			cmd = m.commandRunner("open", urlStr)
		case osWindows:
			// #nosec G204 -- the URL is executed directly as a single argument
			cmd = m.commandRunner("rundll32", "url.dll,FileProtocolHandler", urlStr)
		default:
			// #nosec G204 -- the URL is executed directly as a single argument
			cmd = m.commandRunner("xdg-open", urlStr)
		}
		if err := m.startCommand(cmd); err != nil {
			return errMsg{err: err}
		}
		return nil
	}
}

func (m *Model) openPR() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	wt := m.filteredWts[m.selectedIndex]
	if wt.PR == nil {
		return nil
	}
	prURL, err := sanitizePRURL(wt.PR.URL)
	if err != nil {
		return func() tea.Msg { return errMsg{err: err} }
	}
	return m.openURLInBrowser(prURL)
}

// openCICheckSelection opens a selection screen for CI checks on the current worktree.
func (m *Model) openCICheckSelection() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	wt := m.filteredWts[m.selectedIndex]

	// Get CI checks from cache
	cached, ok := m.ciCache[wt.Branch]
	if !ok || len(cached.checks) == 0 {
		m.showInfo("No CI checks available. Press 'p' to fetch PR data first.", nil)
		return nil
	}

	// Build selection items from CI checks
	items := make([]selectionItem, 0, len(cached.checks))
	for i, check := range cached.checks {
		// Build status indicator
		status := ciIconForConclusion(check.Conclusion)
		if status == "" {
			status = check.Status
		}
		label := fmt.Sprintf("%s %s", status, check.Name)
		items = append(items, selectionItem{
			id:    fmt.Sprintf("%d", i),
			label: label,
		})
	}

	m.listScreen = NewListSelectionScreen(
		items,
		labelWithIcon(UIIconCICheck, "Select CI Check", m.config.IconsEnabled()),
		"Filter checks...",
		"No CI checks found.",
		m.windowWidth,
		m.windowHeight,
		"",
		m.theme,
	)

	// Store checks for later access in submit handler
	checks := cached.checks
	m.listSubmit = func(item selectionItem) tea.Cmd {
		// Keep the selection screen open - don't clear listScreen/listSubmit/currentScreen
		// This allows users to view multiple checks without reopening the selection

		// Parse the index from item.id
		var idx int
		if _, err := fmt.Sscanf(item.id, "%d", &idx); err != nil || idx < 0 || idx >= len(checks) {
			return nil
		}

		return m.showCICheckLog(checks[idx])
	}

	m.currentScreen = screenListSelect
	return textinput.Blink
}

// showCICheckLog opens the CI check log in a pager using gh run view.
// For external CI systems (non-GitHub Actions), it opens the check link in the browser.
func (m *Model) showCICheckLog(check *models.CICheck) tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	wt := m.filteredWts[m.selectedIndex]

	// Extract run ID from the check link
	runID := extractRunIDFromLink(check.Link)
	if runID == "" {
		// Not a GitHub Actions URL - open in browser instead
		if check.Link == "" {
			m.showInfo("No link available for this check.", nil)
			return nil
		}
		return m.openURLInBrowser(check.Link)
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

	// Use --log-failed for failed checks, --log for others
	logFlag := "--log"
	if check.Conclusion == iconFailure {
		logFlag = "--log-failed"
	}

	// Build the command to fetch and display logs
	cmdStr := fmt.Sprintf("set -o pipefail; gh run view %s %s 2>&1 | %s", runID, logFlag, pagerCmd)

	// Create command
	// #nosec G204 -- command is constructed from controlled inputs
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

// extractRunIDFromLink extracts the run ID from a GitHub Actions URL.
// Example URL: https://github.com/owner/repo/actions/runs/12345678/job/98765432
func extractRunIDFromLink(link string) string {
	if link == "" {
		return ""
	}

	parsed, err := url.Parse(link)
	if err != nil {
		return ""
	}

	// Check if it's a GitHub Actions URL
	if !strings.Contains(parsed.Host, "github.com") {
		return ""
	}

	// Path should contain /actions/runs/<run_id>
	parts := strings.Split(parsed.Path, "/")
	for i, part := range parts {
		if part == "runs" && i+1 < len(parts) {
			return parts[i+1]
		}
	}

	return ""
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
			m.helpScreen = NewHelpScreen(m.windowWidth, m.windowHeight, m.config.CustomCommands, m.theme, m.config.IconsEnabled())
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

// UpdateTheme updates the application theme and refreshes component styles.
func (m *Model) UpdateTheme(themeName string) {
	thm := theme.GetThemeWithCustoms(themeName, config.CustomThemesToThemeDataMap(m.config.CustomThemes))
	m.theme = thm

	// Update table styles
	s := table.DefaultStyles()
	s.Selected = s.Selected.Foreground(thm.AccentFg).Background(thm.Accent)
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(thm.BorderDim).
		BorderBottom(true).
		Bold(true).
		Foreground(thm.Cyan).
		Background(thm.AccentDim)
	s.Selected = s.Selected.Bold(true) // Arrow indicator shows selection, no background

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
		initials := authorInitials(entry.authorInitials)
		if entry.isUnpushed || entry.isUnmerged {
			showIcons := m.config.IconsEnabled()
			initials = aheadIndicator(showIcons)
			if showIcons {
				initials = iconWithSpace(initials)
			}
		}

		rows = append(rows, table.Row{sha, initials, msg})
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

func filterNonEmpty(values []string) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			filtered = append(filtered, value)
		}
	}
	return filtered
}
