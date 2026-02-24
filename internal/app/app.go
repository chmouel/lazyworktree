// Package app provides the main application UI and logic using Bubble Tea.
package app

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/app/services"
	"github.com/chmouel/lazyworktree/internal/app/state"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/git"
	log "github.com/chmouel/lazyworktree/internal/log"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/security"
	"github.com/chmouel/lazyworktree/internal/theme"
	"github.com/chmouel/lazyworktree/internal/utils"
)

const (
	keyEnter    = "enter"
	keyEsc      = "esc"
	keyEscRaw   = "\x1b" // Raw escape byte for terminals that send ESC as a rune
	keyCtrlD    = "ctrl+d"
	keyCtrlU    = "ctrl+u"
	keyCtrlC    = "ctrl+c"
	keyCtrlJ    = "ctrl+j"
	keyCtrlK    = "ctrl+k"
	keyDown     = "down"
	keyUp       = "up"
	keyQ        = "q"
	keyTab      = "tab"
	keyShiftTab = "shift+tab"

	errBranchEmpty           = "Branch name cannot be empty."
	errNoWorktreeSelected    = "No worktree selected."
	errPRBranchMissing       = "PR branch information is missing."
	customCommandPlaceholder = "Custom command"
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

	prStateOpen   = "OPEN"
	prStateMerged = "MERGED"
	prStateClosed = "CLOSED"

	commitMessageMaxLength     = 80
	filterWorktreesPlaceholder = "Filter worktrees..."
	placeholderFilterFiles     = "Filter files..."
	worktreeNoteMaxChars       = 4000
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
		worktrees      []*models.WorktreeInfo
		err            error
		pruned         int
		failed         int
		orphansDeleted int
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
	singlePRLoadedMsg struct {
		worktreePath string
		pr           *models.PRInfo
		err          error
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
		note       string
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
		note        string
		err         error
	}
	renameWorktreeResultMsg struct {
		oldPath   string
		newPath   string
		worktrees []*models.WorktreeInfo
		err       error
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
	ciRerunResultMsg struct {
		runURL string
		err    error
	}
	openNoteEditorMsg struct {
		worktreePath string
	}
	openNoteExternalEditorResultMsg struct {
		worktreePath string
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
type StatusFile = models.StatusFile

// StatusTreeNode represents a node in the status file tree (directory or file).
type StatusTreeNode = services.StatusTreeNode

type commitMeta struct {
	sha     string
	author  string
	email   string
	date    string
	subject string
	body    []string
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

type uiState struct {
	worktreeTable  table.Model
	statusViewport viewport.Model
	logTable       table.Model
	filterInput    textinput.Model
	spinner        spinner.Model
	screenManager  *screen.Manager
}

type dataState struct {
	worktrees       []*models.WorktreeInfo
	filteredWts     []*models.WorktreeInfo
	selectedIndex   int
	accessHistory   map[string]int64 // worktree path -> last access timestamp
	statusFiles     []StatusFile     // parsed list of files from git status (kept for compatibility)
	statusFilesAll  []StatusFile     // full list of files from git status
	statusFileIndex int              // currently selected file index in status pane
	logEntries      []commitLogEntry
	logEntriesAll   []commitLogEntry
}

type servicesState struct {
	git          *git.Service
	worktree     services.WorktreeService
	trustManager *security.TrustManager
	statusTree   *services.StatusService
	watch        *services.GitWatchService
	filter       *services.FilterService
}

type modelState struct {
	ui       uiState
	data     dataState
	view     *state.ViewState
	services servicesState
}

// Model represents the main application model
type Model struct {
	// Configuration
	config *config.AppConfig
	theme  *theme.Theme

	// State
	state                     modelState
	sortMode                  int // sortModePath, sortModeLastActive, or sortModeLastSwitched
	prDataLoaded              bool
	checkMergedAfterPRRefresh bool // Flag to trigger merged check after PR data refresh
	repoKey                   string
	repoKeyOnce               sync.Once
	currentDetailsPath        string
	loading                   bool
	loadingOperation          string // Tracks what operation is loading (push, sync, etc.)
	infoContent               string
	statusContent             string

	// Status tree view
	ciCheckIndex int // Current selection in CI checks (-1 = none, 0+ = index)

	// Cache
	cache struct {
		dataCache       map[string]any
		divergenceCache map[string]string
		notifiedErrors  map[string]bool
		ciCache         services.CICheckCache // branch -> CI checks cache
		detailsCache    map[string]*detailsCacheEntry
		detailsCacheMu  sync.RWMutex
	}
	worktreesLoaded bool

	// Create from current state
	createFromCurrent struct {
		diff        string              // Cached diff for AI script
		randomName  string              // Random branch name
		aiName      string              // AI-generated name (cached)
		branch      string              // Current branch name
		inputScreen *screen.InputScreen // Reference for checkbox toggle handling
	}

	// Context
	ctx    context.Context
	cancel context.CancelFunc

	// Debouncing
	detailUpdateCancel  context.CancelFunc
	pendingDetailsIndex int

	// Auto refresh
	autoRefreshStarted bool

	// Post-refresh selection (e.g. after creating worktree)
	pendingSelectWorktreePath string

	// Trust / repo commands
	repoConfig     *config.RepoConfig
	repoConfigPath string
	pending        *state.PendingState

	// Command history for ! command
	commandHistory []string

	// Per-worktree annotations.
	worktreeNotes map[string]models.WorktreeNote

	// Command palette usage history for MRU sorting
	paletteHistory []commandPaletteUsage

	// Original theme before theme selection (for preview rollback)
	originalTheme string

	// Exit
	selectedPath string
	quitting     bool

	// Command execution
	commandRunner func(context.Context, string, ...string) *exec.Cmd
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
	case "text", "emoji", "none":
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

	layoutMode := state.LayoutDefault
	if cfg.Layout == "top" {
		layoutMode = state.LayoutTop
	}

	m := &Model{
		config:   cfg,
		theme:    thm,
		sortMode: sortMode,
		ctx:      ctx,
		cancel:   cancel,
		state: modelState{
			view: &state.ViewState{
				FilterTarget: state.FilterTargetWorktrees,
				SearchTarget: state.SearchTargetWorktrees,
				FocusedPane:  0,
				ZoomedPane:   -1,
				WindowWidth:  80,
				WindowHeight: 24,
				Layout:       layoutMode,
			},
		},
		infoContent:   errNoWorktreeSelected,
		statusContent: "Loading...",
		loading:       true,
		ciCheckIndex:  -1,
		commandRunner: exec.CommandContext,
		execProcess:   tea.ExecProcess,
		startCommand: func(cmd *exec.Cmd) error {
			return cmd.Start()
		},
		pending: &state.PendingState{},
	}

	m.state.data.worktrees = []*models.WorktreeInfo{}
	m.state.data.filteredWts = []*models.WorktreeInfo{}
	m.state.data.accessHistory = make(map[string]int64)
	m.worktreeNotes = make(map[string]models.WorktreeNote)

	m.cache.dataCache = make(map[string]any)
	m.cache.divergenceCache = make(map[string]string)
	m.cache.notifiedErrors = make(map[string]bool)
	m.cache.ciCache = services.NewCICheckCache()
	m.cache.detailsCache = make(map[string]*detailsCacheEntry)

	m.state.ui.worktreeTable = t
	m.state.ui.statusViewport = statusVp
	m.state.ui.logTable = logT
	m.state.ui.filterInput = filterInput
	m.state.ui.spinner = sp
	m.state.ui.screenManager = screen.NewManager()

	m.state.services.git = gitService
	m.state.services.trustManager = trustManager
	m.state.services.worktree = services.NewWorktreeService(gitService)
	m.state.services.statusTree = services.NewStatusService()
	m.state.services.watch = services.NewGitWatchService(gitService, m.debugf)
	m.state.services.filter = services.NewFilterService(initialFilter)

	gitService.SetCommandRunner(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return m.commandRunner(ctx, name, args...)
	})

	if initialFilter != "" {
		m.state.view.ShowingFilter = true
	}
	if cfg.SearchAutoSelect && !m.state.view.ShowingFilter {
		m.state.view.ShowingFilter = true
	}
	if m.state.view.ShowingFilter {
		m.setFilterTarget(state.FilterTargetWorktrees)
		m.state.ui.filterInput.Focus()
	}

	return m
}

// Init satisfies the tea.Model interface and starts with no command.
func (m *Model) Init() tea.Cmd {
	m.loadCommandHistory()
	m.loadAccessHistory()
	m.loadWorktreeNotes()
	m.loadPaletteHistory()
	cmds := []tea.Cmd{
		m.loadCache(),
		m.refreshWorktrees(),
		m.state.ui.spinner.Tick,
	}
	if m.state.view.ShowingFilter {
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
		m.state.ui.spinner, cmd = m.state.ui.spinner.Update(msg)
		if loadingScreen := m.loadingScreen(); loadingScreen != nil {
			loadingScreen.Tick()
		}
		return m, cmd

	case tea.KeyMsg:
		m.debugf("key: %s screen=%s focus=%d filter=%t", msg.String(), m.state.ui.screenManager.Type().String(), m.state.view.FocusedPane, m.state.view.ShowingFilter)
		if m.state.ui.screenManager.IsActive() {
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
		m.deleteWorktreeNote(msg.path)

		// Worktree deleted successfully, show branch deletion prompt
		confirmScreen := screen.NewConfirmScreenWithDefault(
			fmt.Sprintf("Worktree deleted successfully.\n\nDelete branch '%s'?", msg.branch),
			0, // Default to Confirm button (Yes)
			m.theme,
		)
		confirmScreen.OnConfirm = m.deleteBranchCmd(msg.branch)
		confirmScreen.OnCancel = func() tea.Cmd {
			m.state.ui.screenManager.Pop()
			return nil
		}
		m.state.ui.screenManager.Push(confirmScreen)
		return m, nil

	case createFromPRResultMsg:
		m.loading = false
		m.clearLoadingScreen()
		if msg.err != nil {
			m.pendingSelectWorktreePath = ""
			m.showInfo(fmt.Sprintf("Failed to create worktree from PR/MR #%d: %v", msg.prNumber, msg.err), nil)
			return m, nil
		}
		if strings.TrimSpace(msg.note) != "" {
			m.setWorktreeNote(msg.targetPath, msg.note)
		}
		env := m.buildCommandEnv(msg.branch, msg.targetPath)
		initCmds := m.collectInitCommands()
		after := func() tea.Msg {
			worktrees, err := m.state.services.git.GetWorktrees(m.ctx)
			return worktreesLoadedMsg{worktrees: worktrees, err: err}
		}
		return m, m.runCommandsWithTrust(initCmds, msg.targetPath, env, after)

	case createFromIssueResultMsg:
		m.loading = false
		m.clearLoadingScreen()
		if msg.err != nil {
			m.pendingSelectWorktreePath = ""
			m.showInfo(fmt.Sprintf("Failed to create worktree from issue #%d: %v", msg.issueNumber, msg.err), nil)
			return m, nil
		}
		if strings.TrimSpace(msg.note) != "" {
			m.setWorktreeNote(msg.targetPath, msg.note)
		}
		env := m.buildCommandEnv(msg.branch, msg.targetPath)
		initCmds := m.collectInitCommands()
		after := func() tea.Msg {
			worktrees, err := m.state.services.git.GetWorktrees(m.ctx)
			return worktreesLoadedMsg{worktrees: worktrees, err: err}
		}
		return m, m.runCommandsWithTrust(initCmds, msg.targetPath, env, after)

	case renameWorktreeResultMsg:
		if msg.err != nil {
			m.showInfo(fmt.Sprintf("Error: %v", msg.err), nil)
			return m, nil
		}
		m.migrateWorktreeNote(msg.oldPath, msg.newPath)
		return m.handleWorktreesLoaded(worktreesLoadedMsg{
			worktrees: msg.worktrees,
			err:       nil,
		})

	case openNoteEditorMsg:
		return m, m.showWorktreeNoteEditor(msg.worktreePath)

	case openNoteExternalEditorResultMsg:
		m.state.ui.screenManager.Pop()
		existing, _ := m.getWorktreeNote(msg.worktreePath)
		cmd := m.showWorktreeNoteViewer(msg.worktreePath, existing.Note)
		m.updateTable()
		return m, cmd

	case customCreateResultMsg:
		m.loading = false
		m.clearLoadingScreen()
		if msg.err != nil {
			m.showInfo(fmt.Sprintf("Custom command failed: %v", msg.err), nil)
			return m, nil
		}
		// Store the branch name and show branch name input with the selected base ref
		m.pending.CustomBranchName = msg.branchName
		return m, m.showBranchNameInput(m.pending.CustomBaseRef, msg.branchName)

	case customPostCommandPendingMsg:
		if m.pending.CustomMenu == nil || m.pending.CustomMenu.PostCommand == "" {
			// No post-command, just reload
			worktrees, err := m.state.services.git.GetWorktrees(m.ctx)
			return m, func() tea.Msg {
				return worktreesLoadedMsg{worktrees: worktrees, err: err}
			}
		}

		menu := m.pending.CustomMenu
		cmd := menu.PostCommand
		interactive := menu.PostInteractive

		// Clear the pending menu
		m.pending.CustomMenu = nil
		m.pending.CustomBaseRef = ""
		m.pending.CustomBranchName = ""

		// Run the post-command
		if interactive {
			return m, m.executeCustomPostCommandInteractive(cmd, msg.targetPath, msg.env)
		}
		return m, m.executeCustomPostCommand(cmd, msg.targetPath, msg.env)

	case customPostCommandResultMsg:
		m.loading = false
		m.clearLoadingScreen()

		if msg.err != nil {
			// Show error but continue (worktree was already created)
			m.showInfo(fmt.Sprintf("Post-creation command failed: %v", msg.err), nil)
		}

		// Reload worktrees regardless
		worktrees, err := m.state.services.git.GetWorktrees(m.ctx)
		return m, func() tea.Msg {
			return worktreesLoadedMsg{worktrees: worktrees, err: err}
		}

	case loadingProgressMsg:
		// Update the loading screen message with progress information
		m.updateLoadingMessage(msg.message)
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
		sanitizedName := sanitizeBranchNameFromTitle(msg.name, m.createFromCurrent.randomName)

		// Cache the generated name
		suggestedName := m.suggestBranchName(sanitizedName)
		m.createFromCurrent.aiName = suggestedName

		// Update input field if checkbox is still checked
		if m.createFromCurrent.inputScreen != nil && m.createFromCurrent.inputScreen.CheckboxChecked {
			m.createFromCurrent.inputScreen.Input.SetValue(suggestedName)
			m.createFromCurrent.inputScreen.Input.CursorEnd()
		}

		return m, nil

	case prDataLoadedMsg, singlePRLoadedMsg, ciStatusLoadedMsg:
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
		if msg.selectedIndex == m.state.ui.worktreeTable.Cursor() &&
			msg.selectedIndex >= 0 && msg.selectedIndex < len(m.state.data.filteredWts) {
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
		m.showInfo(message, nil)
		return m, nil
	case zellijSessionReadyMsg:
		if msg.attach && !msg.insideZellij {
			return m, m.attachZellijSessionCmd(msg.sessionName)
		}
		message := buildZellijInfoMessage(msg.sessionName)
		m.showInfo(message, nil)
		return m, nil

	case zellijPaneCreatedMsg:
		m.showInfo(fmt.Sprintf("Pane added to session %q (%s).", msg.sessionName, msg.direction), nil)
		return m, nil

	case terminalTabReadyMsg:
		if msg.err != nil {
			m.showInfo(fmt.Sprintf("Terminal tab error: %v", msg.err), nil)
			return m, nil
		}
		return m, nil

	case refreshCompleteMsg:
		return m, m.updateDetailsView()

	case fetchRemotesCompleteMsg:
		m.statusContent = "Remotes fetched"
		// Continue showing loading screen while refreshing worktrees
		m.updateLoadingMessage(loadingRefreshWorktrees)
		return m, m.refreshWorktrees()

	case pushResultMsg:
		m.loading = false
		m.loadingOperation = ""
		m.clearLoadingScreen()
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
		m.clearLoadingScreen()
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
		// Periodically refresh CI status (GitHub only, requires ci_auto_refresh)
		if m.config.CIAutoRefresh && m.state.services.git.IsGitHub(m.ctx) && m.shouldRefreshCI() {
			if cmd := m.maybeFetchCIStatus(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return m, tea.Batch(cmds...)

	case gitDirChangedMsg:
		m.state.services.watch.ResetWaiting()
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
		// Convert commitMeta to screen.CommitMeta
		screenMeta := screen.CommitMeta{
			SHA:     msg.meta.sha,
			Author:  msg.meta.author,
			Email:   msg.meta.email,
			Date:    msg.meta.date,
			Subject: msg.meta.subject,
		}
		commitFilesScr := screen.NewCommitFilesScreen(
			msg.sha,
			msg.worktreePath,
			msg.files,
			screenMeta,
			m.state.view.WindowWidth,
			m.state.view.WindowHeight,
			m.theme,
			m.config.IconsEnabled(),
		)
		// Set callbacks
		sha := msg.sha
		wtPath := msg.worktreePath
		commitFilesScr.OnShowFileDiff = func(filename string) tea.Cmd {
			return m.showCommitFileDiff(sha, filename, wtPath)
		}
		commitFilesScr.OnShowCommitDiff = func() tea.Cmd {
			for _, w := range m.state.data.filteredWts {
				if w.Path == wtPath {
					return m.showCommitDiff(sha, w)
				}
			}
			return nil
		}
		commitFilesScr.OnClose = func() tea.Cmd {
			m.state.ui.screenManager.Pop()
			return nil
		}
		m.state.ui.screenManager.Push(commitFilesScr)
		return m, nil

	case ciRerunResultMsg:
		m.loading = false
		m.loadingOperation = ""
		m.clearLoadingScreen()
		if msg.err != nil {
			m.showInfo(fmt.Sprintf("Failed to restart CI: %v", msg.err), nil)
			return m, nil
		}
		m.showInfo("CI job restarted successfully", nil)
		return m, nil

	}

	return m, tea.Batch(cmds...)
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
