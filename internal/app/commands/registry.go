package commands

import tea "github.com/charmbracelet/bubbletea"

const (
	sectionWorktreeActions = "Worktree Actions"
	sectionCreateShortcuts = "Create Menu"
	sectionGitOperations   = "Git Operations"
	sectionStatusPane      = "Status Pane"
	sectionLogPane         = "Log Pane"
	sectionNavigation      = "Navigation"
	sectionSettings        = "Settings"
)

// CommandAction describes a command palette action.
type CommandAction struct {
	ID          string
	Label       string
	Description string
	Section     string
	Shortcut    string // Keyboard shortcut display (e.g., "g")
	Icon        string // Category icon (Nerd Font)
	Handler     func() tea.Cmd
	Available   func() bool
}

// Registry stores command palette actions.
type Registry struct {
	actions []CommandAction
	byID    map[string]CommandAction
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{byID: make(map[string]CommandAction)}
}

// Register adds actions to the registry.
func (r *Registry) Register(actions ...CommandAction) {
	for _, action := range actions {
		r.actions = append(r.actions, action)
		if action.ID != "" {
			r.byID[action.ID] = action
		}
	}
}

// Actions returns the registered actions in order.
func (r *Registry) Actions() []CommandAction {
	return r.actions
}

// Execute runs the handler for an action ID.
func (r *Registry) Execute(id string) tea.Cmd {
	action, ok := r.byID[id]
	if !ok {
		return nil
	}
	if action.Available != nil && !action.Available() {
		return nil
	}
	if action.Handler == nil {
		return nil
	}
	return action.Handler()
}

// WorktreeHandlers holds callbacks for worktree actions.
type WorktreeHandlers struct {
	Create            func() tea.Cmd
	Delete            func() tea.Cmd
	Rename            func() tea.Cmd
	Annotate          func() tea.Cmd
	Absorb            func() tea.Cmd
	Prune             func() tea.Cmd
	CreateFromCurrent func() tea.Cmd
	CreateFromBranch  func() tea.Cmd
	CreateFromCommit  func() tea.Cmd
	CreateFromPR      func() tea.Cmd
	CreateFromIssue   func() tea.Cmd
	CreateFreeform    func() tea.Cmd
}

// Section icons for command palette display.
const (
	IconWorktree   = "" // Nerd Font: git branch
	IconCreate     = "" // Nerd Font: plus
	IconGit        = "" // Nerd Font: git
	IconStatus     = "" // Nerd Font: file-diff
	IconLog        = "" // Nerd Font: history
	IconNavigation = "" // Nerd Font: compass
	IconSettings   = "" // Nerd Font: cog
	IconCustom     = "" // Nerd Font: terminal
	IconMultiplex  = "" // Nerd Font: layers
	IconRecent     = "" // Nerd Font: clock
)

// RegisterWorktreeActions registers worktree-related actions.
func RegisterWorktreeActions(r *Registry, h WorktreeHandlers) {
	r.Register(
		CommandAction{ID: "create", Label: "Create worktree", Description: "Add a new worktree from base branch or PR/MR", Section: sectionWorktreeActions, Shortcut: "c", Icon: IconWorktree, Handler: h.Create},
		CommandAction{ID: "delete", Label: "Delete worktree", Description: "Remove worktree and branch", Section: sectionWorktreeActions, Shortcut: "D", Icon: IconWorktree, Handler: h.Delete},
		CommandAction{ID: "rename", Label: "Rename worktree", Description: "Rename worktree and branch", Section: sectionWorktreeActions, Shortcut: "m", Icon: IconWorktree, Handler: h.Rename},
		CommandAction{ID: "annotate", Label: "Worktree notes", Description: "Add or edit notes for the selected worktree", Section: sectionWorktreeActions, Shortcut: "i", Icon: IconWorktree, Handler: h.Annotate},
		CommandAction{ID: "absorb", Label: "Absorb worktree", Description: "Merge branch into main and remove worktree", Section: sectionWorktreeActions, Shortcut: "A", Icon: IconWorktree, Handler: h.Absorb},
		CommandAction{ID: "prune", Label: "Prune merged", Description: "Remove merged PR worktrees", Section: sectionWorktreeActions, Shortcut: "X", Icon: IconWorktree, Handler: h.Prune},
	)

	r.Register(
		CommandAction{ID: "create-from-current", Label: "Create from current branch", Description: "Create from current branch with or without changes", Section: sectionCreateShortcuts, Icon: IconCreate, Handler: h.CreateFromCurrent},
		CommandAction{ID: "create-from-branch", Label: "Create from branch/tag", Description: "Select a branch, tag, or remote as base", Section: sectionCreateShortcuts, Icon: IconCreate, Handler: h.CreateFromBranch},
		CommandAction{ID: "create-from-commit", Label: "Create from commit", Description: "Choose a branch, then select a specific commit", Section: sectionCreateShortcuts, Icon: IconCreate, Handler: h.CreateFromCommit},
		CommandAction{ID: "create-from-pr", Label: "Create from PR/MR", Description: "Create from a pull/merge request", Section: sectionCreateShortcuts, Icon: IconCreate, Handler: h.CreateFromPR},
		CommandAction{ID: "create-from-issue", Label: "Create from issue", Description: "Create from a GitHub/GitLab issue", Section: sectionCreateShortcuts, Icon: IconCreate, Handler: h.CreateFromIssue},
		CommandAction{ID: "create-freeform", Label: "Create from ref", Description: "Enter a branch, tag, or commit manually", Section: sectionCreateShortcuts, Icon: IconCreate, Handler: h.CreateFreeform},
	)
}

// GitHandlers holds callbacks for git operations.
type GitHandlers struct {
	ShowDiff          func() tea.Cmd
	Refresh           func() tea.Cmd
	Fetch             func() tea.Cmd
	Push              func() tea.Cmd
	Sync              func() tea.Cmd
	FetchPRData       func() tea.Cmd
	ViewCIChecks      func() tea.Cmd
	CIChecksAvailable func() bool
	OpenPR            func() tea.Cmd
	OpenLazyGit       func() tea.Cmd
	RunCommand        func() tea.Cmd
}

// RegisterGitOperations registers git operations.
func RegisterGitOperations(r *Registry, h GitHandlers) {
	r.Register(
		CommandAction{ID: "diff", Label: "Show diff", Description: "Show diff for current worktree or commit", Section: sectionGitOperations, Shortcut: "d", Icon: IconGit, Handler: h.ShowDiff},
		CommandAction{ID: "refresh", Label: "Refresh", Description: "Reload worktrees", Section: sectionGitOperations, Shortcut: "r", Icon: IconGit, Handler: h.Refresh},
		CommandAction{ID: "fetch", Label: "Fetch remotes", Description: "git fetch --all", Section: sectionGitOperations, Shortcut: "R", Icon: IconGit, Handler: h.Fetch},
		CommandAction{ID: "push", Label: "Push to upstream", Description: "git push (clean worktree only)", Section: sectionGitOperations, Shortcut: "P", Icon: IconGit, Handler: h.Push},
		CommandAction{ID: "sync", Label: "Synchronise with upstream", Description: "git pull, then git push (clean worktree only)", Section: sectionGitOperations, Shortcut: "S", Icon: IconGit, Handler: h.Sync},
		CommandAction{ID: "fetch-pr-data", Label: "Fetch PR data", Description: "Fetch PR/MR status from GitHub/GitLab", Section: sectionGitOperations, Shortcut: "p", Icon: IconGit, Handler: h.FetchPRData},
		CommandAction{ID: "ci-checks", Label: "View CI checks", Description: "View CI check logs for current worktree", Section: sectionGitOperations, Shortcut: "v", Icon: IconGit, Handler: h.ViewCIChecks, Available: h.CIChecksAvailable},
		CommandAction{ID: "pr", Label: "Open PR", Description: "Open PR in browser", Section: sectionGitOperations, Shortcut: "o", Icon: IconGit, Handler: h.OpenPR},
		CommandAction{ID: "lazygit", Label: "Open LazyGit", Description: "Open LazyGit in selected worktree", Section: sectionGitOperations, Shortcut: "g", Icon: IconGit, Handler: h.OpenLazyGit},
		CommandAction{ID: "run-command", Label: "Run command", Description: "Run arbitrary command in worktree", Section: sectionGitOperations, Shortcut: "!", Icon: IconGit, Handler: h.RunCommand},
	)
}

// StatusHandlers holds callbacks for status pane actions.
type StatusHandlers struct {
	StageFile    func() tea.Cmd
	CommitStaged func() tea.Cmd
	CommitAll    func() tea.Cmd
	EditFile     func() tea.Cmd
	DeleteFile   func() tea.Cmd
}

// RegisterStatusPaneActions registers status pane actions.
func RegisterStatusPaneActions(r *Registry, h StatusHandlers) {
	r.Register(
		CommandAction{ID: "stage-file", Label: "Stage/unstage file", Description: "Stage or unstage selected file", Section: sectionStatusPane, Shortcut: "s", Icon: IconStatus, Handler: h.StageFile},
		CommandAction{ID: "commit-staged", Label: "Commit staged", Description: "Commit staged changes", Section: sectionStatusPane, Shortcut: "c", Icon: IconStatus, Handler: h.CommitStaged},
		CommandAction{ID: "commit-all", Label: "Stage all and commit", Description: "Stage all changes and commit", Section: sectionStatusPane, Shortcut: "C", Icon: IconStatus, Handler: h.CommitAll},
		CommandAction{ID: "edit-file", Label: "Edit file", Description: "Open selected file in editor", Section: sectionStatusPane, Shortcut: "e", Icon: IconStatus, Handler: h.EditFile},
		CommandAction{ID: "delete-file", Label: "Delete selected file or directory", Section: sectionStatusPane, Icon: IconStatus, Handler: h.DeleteFile},
	)
}

// LogHandlers holds callbacks for log pane actions.
type LogHandlers struct {
	CherryPick func() tea.Cmd
	CommitView func() tea.Cmd
}

// RegisterLogPaneActions registers log pane actions.
func RegisterLogPaneActions(r *Registry, h LogHandlers) {
	r.Register(
		CommandAction{ID: "cherry-pick", Label: "Cherry-pick commit", Description: "Cherry-pick commit to another worktree", Section: sectionLogPane, Shortcut: "C", Icon: IconLog, Handler: h.CherryPick},
		CommandAction{ID: "commit-view", Label: "Browse commit files", Description: "Browse files changed in selected commit", Section: sectionLogPane, Icon: IconLog, Handler: h.CommitView},
	)
}

// NavigationHandlers holds callbacks for navigation actions.
type NavigationHandlers struct {
	ToggleZoom    func() tea.Cmd
	ToggleLayout  func() tea.Cmd
	Filter        func() tea.Cmd
	Search        func() tea.Cmd
	FocusWorktree func() tea.Cmd
	FocusStatus   func() tea.Cmd
	FocusLog      func() tea.Cmd
	SortCycle     func() tea.Cmd
}

// RegisterNavigationActions registers navigation actions.
func RegisterNavigationActions(r *Registry, h NavigationHandlers) {
	r.Register(
		CommandAction{ID: "zoom-toggle", Label: "Toggle zoom", Description: "Toggle zoom on focused pane", Section: sectionNavigation, Shortcut: "=", Icon: IconNavigation, Handler: h.ToggleZoom},
		CommandAction{ID: "toggle-layout", Label: "Toggle layout", Description: "Switch between default and top layout", Section: sectionNavigation, Shortcut: "L", Icon: IconNavigation, Handler: h.ToggleLayout},
		CommandAction{ID: "filter", Label: "Filter", Description: "Filter items in focused pane", Section: sectionNavigation, Shortcut: "f", Icon: IconNavigation, Handler: h.Filter},
		CommandAction{ID: "search", Label: "Search", Description: "Search items in focused pane", Section: sectionNavigation, Shortcut: "/", Icon: IconNavigation, Handler: h.Search},
		CommandAction{ID: "focus-worktrees", Label: "Focus worktrees", Description: "Focus worktree pane", Section: sectionNavigation, Shortcut: "1", Icon: IconNavigation, Handler: h.FocusWorktree},
		CommandAction{ID: "focus-status", Label: "Focus status", Description: "Focus status pane", Section: sectionNavigation, Shortcut: "2", Icon: IconNavigation, Handler: h.FocusStatus},
		CommandAction{ID: "focus-log", Label: "Focus log", Description: "Focus log pane", Section: sectionNavigation, Shortcut: "3", Icon: IconNavigation, Handler: h.FocusLog},
		CommandAction{ID: "sort-cycle", Label: "Cycle sort", Description: "Cycle sort mode (path/active/switched)", Section: sectionNavigation, Shortcut: "s", Icon: IconNavigation, Handler: h.SortCycle},
	)
}

// SettingsHandlers holds callbacks for settings actions.
type SettingsHandlers struct {
	Theme func() tea.Cmd
	Help  func() tea.Cmd
}

// RegisterSettingsActions registers settings actions.
func RegisterSettingsActions(r *Registry, h SettingsHandlers) {
	r.Register(
		CommandAction{ID: "theme", Label: "Select theme", Description: "Change the application theme with live preview", Section: sectionSettings, Icon: IconSettings, Handler: h.Theme},
		CommandAction{ID: "help", Label: "Help", Description: "Show help", Section: sectionSettings, Shortcut: "?", Icon: IconSettings, Handler: h.Help},
	)
}
