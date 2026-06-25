// Package app provides the main application UI and logic using Bubble Tea.
package app

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
	keyEnter  = "enter"
	keyEsc    = "esc"
	keyEscRaw = "\x1b" // Raw escape byte for terminals that send ESC as a rune
	keyCtrlC  = "ctrl+c"
	keyCtrlG  = "ctrl+g"
	keyCtrlJ  = "ctrl+j"
	keyCtrlK  = "ctrl+k"
	keyDown   = "down"
	keyUp     = "up"
	keyQ      = "q"
	keyTab    = "tab"

	errBranchEmpty           = "Branch name cannot be empty."
	errNoWorktreeSelected    = "No worktree selected."
	errPRBranchMissing       = "PR branch information is missing."
	customCommandPlaceholder = "Custom command"
	onExistsAttach           = "attach"

	detailsCacheTTL  = 2 * time.Second
	debounceDelay    = 200 * time.Millisecond
	ciCacheTTL       = 30 * time.Second
	defaultDirPerms  = utils.DefaultDirPerms
	defaultFilePerms = utils.DefaultFilePerms

	osDarwin  = "darwin"
	osWindows = "windows"

	searchFiles = "Search files..."

	// Loading messages
	loadingRefreshWorktrees = "Refreshing worktrees..."

	prStateOpen   = "OPEN"
	prStateMerged = "MERGED"
	prStateClosed = "CLOSED"
	prStateDraft  = "DRAFT"

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
	agentSessionsUpdatedMsg struct {
		sessions []*models.AgentSession
		err      error
	}
	agentWatchChangedMsg  struct{}
	agentRefreshDueMsg    struct{}
	deprecationWarningMsg struct{}
	debouncedDetailsMsg   struct {
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
		worktrees       []*models.WorktreeInfo
		err             error
		pruned          int
		failed          int
		orphansDeleted  int
		branchesDeleted int
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
	avatarLoadedMsg struct {
		url   string
		image *services.AvatarImage
		err   error
	}
	avatarRegisteredMsg struct {
		url string
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
		pr         *models.PRInfo
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
		noteErr     string
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
	commitExternalEditorResultMsg struct {
		result string
	}
)

type commitLogEntry struct {
	sha            string
	authorName     string
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
	resizeStep        = 4
	mainWorktreeName  = "main"

	paneWorktrees     = 0
	paneInfo          = 1
	paneGitStatus     = 2
	paneCommit        = 3
	paneNotes         = 4
	paneAgentSessions = 5

	// Merge methods for absorb worktree
	mergeMethodRebase = "rebase"
	pullRebaseFlag    = "--rebase=true"

	// Sort modes for worktree list
	sortModePath         = 0 // Sort by path (alphabetical)
	sortModeLastActive   = 1 // Sort by last commit date
	sortModeLastSwitched = 2 // Sort by last UI access time
)

type uiState struct {
	worktreeTable         table.Model
	infoViewport          viewport.Model
	statusViewport        viewport.Model
	notesViewport         viewport.Model
	agentSessionsViewport viewport.Model
	logTable              table.Model
	filterInput           textinput.Model
	spinner               spinner.Model
	spinnerActive         bool // whether the spinner tick loop is running (only while loading)
	screenManager         *screen.Manager
}

type dataState struct {
	worktrees         []*models.WorktreeInfo
	filteredWts       []*models.WorktreeInfo
	selectedIndex     int
	accessHistory     map[string]int64 // worktree path -> last access timestamp
	statusFiles       []StatusFile     // parsed list of files from git status (kept for compatibility)
	statusFilesAll    []StatusFile     // full list of files from git status
	statusFileIndex   int              // currently selected file index in status pane
	agentSessions     []*models.AgentSession
	agentSessionIndex int
	logEntries        []commitLogEntry
	logEntriesAll     []commitLogEntry
}

type servicesState struct {
	git            *git.Service
	worktree       services.WorktreeService
	trustManager   *security.TrustManager
	statusTree     *services.StatusService
	watch          *services.GitWatchService
	agentSessions  *services.AgentSessionService
	agentProcesses *services.AgentProcessService
	agentWatch     *services.AgentWatchService
	filter         *services.FilterService
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
	state                modelState
	sortMode             int // sortModePath, sortModeLastActive, or sortModeLastSwitched
	repoKey              string
	repoKeyOnce          sync.Once
	repoWebURL           string
	repoWebURLOnce       sync.Once
	infoContent          string
	statusContent        string
	notesContent         string
	agentSessionsContent string

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

	// Auto refresh
	autoRefreshStarted bool

	// Trust / repo commands
	repoConfig     *config.RepoConfig
	repoConfigPath string
	pending        *state.PendingState

	// Command history for ! command
	commandHistory []string

	// Per-worktree annotations.
	worktreeNotes map[string]models.WorktreeNote

	// Runtime-only PR/MR author avatar state.
	avatarCache  *services.AvatarCache
	avatarStates map[string]*avatarRuntimeState

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

	// Render style cache (theme-dependent styles reused across frames).
	renderStyles renderStyleCache

	// Grouped state sub-structs
	loading   loadingState
	details   detailsState
	pendingOp pendingOpState
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

	t.SetStyles(buildWorktreeTableStyles(thm, nil, true))

	infoVp := viewport.New(viewport.WithWidth(40), viewport.WithHeight(5))
	infoVp.SoftWrap = true

	statusVp := viewport.New(viewport.WithWidth(40), viewport.WithHeight(5))
	statusVp.SetContent("Loading...")

	notesVp := viewport.New(viewport.WithWidth(40), viewport.WithHeight(5))
	agentSessionsVp := viewport.New(viewport.WithWidth(40), viewport.WithHeight(5))

	logColumns := []table.Column{
		{Title: "SHA", Width: 8},
		{Title: "Au", Width: 2},
		{Title: "Message", Width: 50},
	}
	logT := table.New(
		table.WithColumns(logColumns),
		table.WithHeight(5),
	)
	logStyles := buildWorktreeTableStyles(thm, nil, true)
	logT.SetStyles(logStyles)

	filterInput := textinput.New()
	filterInput.Placeholder = filterWorktreesPlaceholder
	filterInput.SetWidth(50)
	filterStyles := filterInput.Styles()
	filterStyles.Focused.Prompt = lipgloss.NewStyle().Foreground(thm.Accent)
	filterStyles.Blurred.Prompt = lipgloss.NewStyle().Foreground(thm.Accent)
	filterStyles.Focused.Text = lipgloss.NewStyle().Foreground(thm.TextFg)
	filterStyles.Blurred.Text = lipgloss.NewStyle().Foreground(thm.TextFg)
	filterInput.SetStyles(filterStyles)

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
				FilterTarget:    state.FilterTargetWorktrees,
				SearchTarget:    state.SearchTargetWorktrees,
				FocusedPane:     paneWorktrees,
				ZoomedPane:      -1,
				WindowWidth:     80,
				WindowHeight:    24,
				Layout:          layoutMode,
				TerminalFocused: true,
			},
		},
		infoContent:   errNoWorktreeSelected,
		statusContent: "Loading...",
		loading: loadingState{
			active: true,
		},
		details: detailsState{
			lastArrow: -1,
		},
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
	m.avatarCache = services.NewDefaultAvatarCache()
	m.avatarStates = make(map[string]*avatarRuntimeState)

	m.cache.dataCache = make(map[string]any)
	m.cache.divergenceCache = make(map[string]string)
	m.cache.notifiedErrors = make(map[string]bool)
	m.cache.ciCache = services.NewCICheckCache()
	m.cache.detailsCache = make(map[string]*detailsCacheEntry)

	m.state.ui.worktreeTable = t
	m.state.ui.infoViewport = infoVp
	m.state.ui.statusViewport = statusVp
	m.state.ui.notesViewport = notesVp
	m.state.ui.agentSessionsViewport = agentSessionsVp
	m.state.ui.logTable = logT
	m.state.ui.filterInput = filterInput
	m.state.ui.spinner = sp
	m.state.ui.screenManager = screen.NewManager()

	m.state.services.git = gitService
	m.state.services.trustManager = trustManager
	m.state.services.worktree = services.NewWorktreeService(gitService)
	m.state.services.statusTree = services.NewStatusService()
	m.state.services.watch = services.NewGitWatchService(gitService, m.debugf)
	m.state.services.agentSessions = services.NewAgentSessionServiceFromConfig(
		cfg.AgentSessionClaudeRoot, cfg.AgentSessionPiRoot, m.debugf,
	)
	m.state.services.agentProcesses = services.NewAgentProcessService(m.debugf)
	m.state.services.agentWatch = services.NewAgentWatchService(
		m.state.services.agentSessions.WatchRoots(),
		time.Duration(cfg.AgentRefreshDebounceMs)*time.Millisecond,
		m.debugf,
	)
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
	m.state.ui.spinnerActive = true
	cmds := []tea.Cmd{
		m.loadCache(),
		m.refreshWorktrees(),
		m.refreshAgentSessions(),
		m.startAgentWatcher(),
		m.state.ui.spinner.Tick,
	}
	if m.state.view.ShowingFilter {
		cmds = append(cmds, textinput.Blink)
	}
	if len(m.config.DeprecationWarnings) > 0 {
		cmds = append(cmds, func() tea.Msg { return deprecationWarningMsg{} })
	}
	return tea.Batch(cmds...)
}

// Update processes Bubble Tea messages and routes them through the app model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	wasLoading := m.loading.active
	model, cmd := m.updateModel(msg)
	mm, ok := model.(*Model)
	if !ok {
		return model, cmd
	}
	// Restart the loading spinner only on the transition into a loading state.
	// The tick loop self-stops when idle (see spinner.TickMsg); without that gate
	// it re-renders the whole UI ~12 times a second forever and pegs a core.
	//
	// The not-loading -> loading transition is required, not just !spinnerActive:
	// a freshly built Model is loading.active before Init runs the first Tick, so
	// gating on !spinnerActive alone would inject a spurious tick on the first
	// message of any such model. In production Init sets both flags together and
	// the loop only stops while idle, so loading-without-a-live-loop never occurs
	// outside that pre-Init window.
	if !wasLoading && mm.loading.active && !mm.state.ui.spinnerActive {
		mm.state.ui.spinnerActive = true
		cmd = tea.Batch(cmd, mm.state.ui.spinner.Tick)
	}
	return mm, cmd
}

func (m *Model) updateModel(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.debugf("window: %dx%d", msg.Width, msg.Height)
		m.setWindowSize(msg.Width, msg.Height)
		return m, nil

	case tea.FocusMsg:
		m.state.view.TerminalFocused = true
		m.debugf("terminal focused")
		return m, m.refreshWorktrees()

	case tea.BlurMsg:
		m.state.view.TerminalFocused = false
		m.debugf("terminal blurred")
		return m, nil

	case tea.ColorProfileMsg:
		m.debugf("colour profile: %s", msg.Profile)
		return m, nil

	case tea.MouseClickMsg:
		return m.handleMouseClick(msg)

	case tea.MouseWheelMsg:
		return m.handleMouseWheel(msg)

	case spinner.TickMsg:
		if !m.loading.active && m.loadingScreen() == nil {
			// Idle: stop the tick loop instead of re-rendering forever. The
			// Update wrapper restarts it when a loading operation begins.
			m.state.ui.spinnerActive = false
			return m, nil
		}
		m.state.ui.spinner, cmd = m.state.ui.spinner.Update(msg)
		if loadingScreen := m.loadingScreen(); loadingScreen != nil {
			loadingScreen.Tick()
		}
		return m, cmd

	case tea.KeyPressMsg:
		m.debugf("key: %s screen=%s focus=%d filter=%t", msg.String(), m.state.ui.screenManager.Type().String(), m.state.view.FocusedPane, m.state.view.ShowingFilter)
		if handledModel, handledCmd, handled := m.handleGlobalKey(msg); handled {
			return handledModel, handledCmd
		}
		if m.state.ui.screenManager.IsActive() {
			return m.handleScreenKey(msg)
		}
		return m.handleKeyMsg(msg)

	case worktreesLoadedMsg, cachedWorktreesMsg, pruneResultMsg, absorbMergeResultMsg:
		return m.handleWorktreeMessages(msg)

	case agentSessionsUpdatedMsg:
		if msg.err == nil {
			m.state.data.agentSessions = msg.sessions
			m.refreshSelectedWorktreeAgentSessionsPane()
		}
		return m, nil

	case agentWatchChangedMsg:
		if m.state.services.agentWatch != nil {
			m.state.services.agentWatch.ResetWaiting()
		}
		cmds = append(cmds, m.waitForAgentWatchEvent())
		if cmd := m.planAgentRefresh(time.Now()); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	case agentRefreshDueMsg:
		if m.state.services.agentWatch != nil {
			m.state.services.agentWatch.TrailingRefreshFired(time.Now())
		}
		if cmd := m.refreshAgentSessions(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	case openPRsLoadedMsg:
		return m, m.handleOpenPRsLoaded(msg)

	case openIssuesLoadedMsg:
		return m, m.handleOpenIssuesLoaded(msg)

	case autoGenerateResultMsg:
		if scr, ok := m.state.ui.screenManager.Current().(*screen.CommitMessageScreen); ok {
			scr.SetGeneratedValue(msg.result)
		} else {
			m.statusContent = "Auto-generated commit message is ready, but the commit screen is no longer active."
			m.debugf("auto-generate completed while commit screen was not active")
		}
		return m, nil

	case commitExternalEditorResultMsg:
		if scr, ok := m.state.ui.screenManager.Current().(*screen.CommitMessageScreen); ok {
			scr.SetValue(msg.result)
		} else {
			m.statusContent = "Edited commit draft is ready, but the commit screen is no longer active."
			m.debugf("commit editor completed while commit screen was not active")
		}
		return m, nil

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
		m.loading.active = false
		m.clearLoadingScreen()
		if msg.err != nil {
			m.pendingOp.selectPath = ""
			m.pendingOp.pr = nil
			m.pendingOp.prPath = ""
			m.showInfo(fmt.Sprintf("Failed to create worktree from %s #%d: %v", changeRequestLabel(msg.pr), msg.prNumber, msg.err), nil)
			return m, nil
		}
		m.pendingOp.pr = msg.pr
		m.pendingOp.prPath = msg.targetPath
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
		m.loading.active = false
		m.clearLoadingScreen()
		if msg.err != nil {
			m.pendingOp.selectPath = ""
			m.showInfo(fmt.Sprintf("Failed to create worktree from issue #%d: %v", msg.issueNumber, msg.err), nil)
			return m, nil
		}
		if strings.TrimSpace(msg.note) != "" {
			m.setWorktreeNote(msg.targetPath, msg.note)
		}
		if msg.noteErr != "" {
			m.statusContent = msg.noteErr
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
		m.loading.active = false
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
		m.loading.active = false
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

	case avatarLoadedMsg:
		return m.handleAvatarLoaded(msg)

	case avatarRegisteredMsg:
		return m.handleAvatarRegistered(msg)

	case statusUpdatedMsg:
		if msg.info != "" {
			m.infoContent = msg.info
		}
		m.setStatusFiles(msg.statusFiles)
		m.updateWorktreeStatus(msg.path, msg.statusFiles)
		if msg.log != nil {
			reset := false
			if msg.path != "" && msg.path != m.details.currentPath {
				m.details.currentPath = msg.path
				reset = true
			}
			m.setLogEntries(msg.log, reset)
		}
		m.refreshSelectedWorktreeNotesPane()
		m.refreshSelectedWorktreeAgentSessionsPane()
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
		m.loading.active = false
		m.loading.operation = ""
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
		m.loading.active = false
		m.loading.operation = ""
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
		// Keep scheduling ticks but skip git work when terminal is unfocused
		if cmd := m.autoRefreshTick(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		if !m.state.view.TerminalFocused {
			return m, tea.Batch(cmds...)
		}
		if cmd := m.refreshDetails(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		if cmd := m.refreshAgentSessions(); cmd != nil {
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
		m.loading.active = false
		m.loading.operation = ""
		m.clearLoadingScreen()
		if msg.err != nil {
			m.showInfo(fmt.Sprintf("Failed to restart CI: %v", msg.err), nil)
			return m, nil
		}
		m.showInfo("CI job restarted successfully", nil)
		return m, nil

	case deprecationWarningMsg:
		warning := "Configuration update required:\n\n" +
			strings.Join(m.config.DeprecationWarnings, "\n") +
			"\n\nSee config.example.yaml or documentation for the new format."
		m.showInfo(warning, nil)
		return m, nil

	}

	return m, tea.Batch(cmds...)
}

// Close releases background resources including canceling contexts and timers.
// It also persists the current selection for the next session.
func (m *Model) Close() {
	m.persistCurrentSelection()
	m.debugf("close")
	if m.details.updateCancel != nil {
		m.details.updateCancel()
	}
	if m.cancel != nil {
		m.cancel()
	}
}
