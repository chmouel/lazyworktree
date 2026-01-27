package commands

import tea "github.com/charmbracelet/bubbletea"

const (
	sectionWorktreeActions = "Worktree Actions"
	sectionCreateShortcuts = "Create Shortcuts"
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
	Absorb            func() tea.Cmd
	Prune             func() tea.Cmd
	CreateFromCurrent func() tea.Cmd
	CreateFromBranch  func() tea.Cmd
	CreateFromCommit  func() tea.Cmd
	CreateFromPR      func() tea.Cmd
	CreateFromIssue   func() tea.Cmd
	CreateFreeform    func() tea.Cmd
}

// RegisterWorktreeActions registers worktree-related actions.
func RegisterWorktreeActions(r *Registry, h WorktreeHandlers) {
	r.Register(
		CommandAction{ID: "create", Label: "Create worktree (c)", Description: "Add a new worktree from base branch or PR/MR", Section: sectionWorktreeActions, Handler: h.Create},
		CommandAction{ID: "delete", Label: "Delete worktree (D)", Description: "Remove worktree and branch", Section: sectionWorktreeActions, Handler: h.Delete},
		CommandAction{ID: "rename", Label: "Rename worktree (m)", Description: "Rename worktree and branch", Section: sectionWorktreeActions, Handler: h.Rename},
		CommandAction{ID: "absorb", Label: "Absorb worktree (A)", Description: "Merge branch into main and remove worktree", Section: sectionWorktreeActions, Handler: h.Absorb},
		CommandAction{ID: "prune", Label: "Prune merged (X)", Description: "Remove merged PR worktrees", Section: sectionWorktreeActions, Handler: h.Prune},
	)

	r.Register(
		CommandAction{ID: "create-from-current", Label: "Create worktree from current branch", Description: "Create from current branch with or without changes", Section: sectionCreateShortcuts, Handler: h.CreateFromCurrent},
		CommandAction{ID: "create-from-branch", Label: "Create worktree from branch/tag", Description: "Select a branch, tag, or remote as base", Section: sectionCreateShortcuts, Handler: h.CreateFromBranch},
		CommandAction{ID: "create-from-commit", Label: "Create worktree from commit", Description: "Choose a branch, then select a specific commit", Section: sectionCreateShortcuts, Handler: h.CreateFromCommit},
		CommandAction{ID: "create-from-pr", Label: "Create worktree from PR/MR", Description: "Create from a pull/merge request", Section: sectionCreateShortcuts, Handler: h.CreateFromPR},
		CommandAction{ID: "create-from-issue", Label: "Create worktree from issue", Description: "Create from a GitHub/GitLab issue", Section: sectionCreateShortcuts, Handler: h.CreateFromIssue},
		CommandAction{ID: "create-freeform", Label: "Create worktree from ref", Description: "Enter a branch, tag, or commit manually", Section: sectionCreateShortcuts, Handler: h.CreateFreeform},
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
		CommandAction{ID: "diff", Label: "Show diff (d)", Description: "Show diff for current worktree or commit", Section: sectionGitOperations, Handler: h.ShowDiff},
		CommandAction{ID: "refresh", Label: "Refresh (r)", Description: "Reload worktrees", Section: sectionGitOperations, Handler: h.Refresh},
		CommandAction{ID: "fetch", Label: "Fetch remotes (R)", Description: "git fetch --all", Section: sectionGitOperations, Handler: h.Fetch},
		CommandAction{ID: "push", Label: "Push to upstream (P)", Description: "git push (clean worktree only)", Section: sectionGitOperations, Handler: h.Push},
		CommandAction{ID: "sync", Label: "Synchronise with upstream (S)", Description: "git pull, then git push (clean worktree only)", Section: sectionGitOperations, Handler: h.Sync},
		CommandAction{ID: "fetch-pr-data", Label: "Fetch PR data (p)", Description: "Fetch PR/MR status from GitHub/GitLab", Section: sectionGitOperations, Handler: h.FetchPRData},
		CommandAction{ID: "ci-checks", Label: "View CI checks (v)", Description: "View CI check logs for current worktree", Section: sectionGitOperations, Handler: h.ViewCIChecks, Available: h.CIChecksAvailable},
		CommandAction{ID: "pr", Label: "Open PR (o)", Description: "Open PR in browser", Section: sectionGitOperations, Handler: h.OpenPR},
		CommandAction{ID: "lazygit", Label: "Open LazyGit (g)", Description: "Open LazyGit in selected worktree", Section: sectionGitOperations, Handler: h.OpenLazyGit},
		CommandAction{ID: "run-command", Label: "Run command (!)", Description: "Run arbitrary command in worktree", Section: sectionGitOperations, Handler: h.RunCommand},
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
		CommandAction{ID: "stage-file", Label: "Stage/unstage file (s)", Description: "Stage or unstage selected file", Section: sectionStatusPane, Handler: h.StageFile},
		CommandAction{ID: "commit-staged", Label: "Commit staged (c)", Description: "Commit staged changes", Section: sectionStatusPane, Handler: h.CommitStaged},
		CommandAction{ID: "commit-all", Label: "Stage all and commit (C)", Description: "Stage all changes and commit", Section: sectionStatusPane, Handler: h.CommitAll},
		CommandAction{ID: "edit-file", Label: "Edit file (e)", Description: "Open selected file in editor", Section: sectionStatusPane, Handler: h.EditFile},
		CommandAction{ID: "delete-file", Label: "Delete selected file or directory", Section: sectionStatusPane, Handler: h.DeleteFile},
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
		CommandAction{ID: "cherry-pick", Label: "Cherry-pick commit (C)", Description: "Cherry-pick commit to another worktree", Section: sectionLogPane, Handler: h.CherryPick},
		CommandAction{ID: "commit-view", Label: "Browse commit files", Description: "Browse files changed in selected commit", Section: sectionLogPane, Handler: h.CommitView},
	)
}

// NavigationHandlers holds callbacks for navigation actions.
type NavigationHandlers struct {
	ToggleZoom    func() tea.Cmd
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
		CommandAction{ID: "zoom-toggle", Label: "Toggle zoom (=)", Description: "Toggle zoom on focused pane", Section: sectionNavigation, Handler: h.ToggleZoom},
		CommandAction{ID: "filter", Label: "Filter (f)", Description: "Filter items in focused pane", Section: sectionNavigation, Handler: h.Filter},
		CommandAction{ID: "search", Label: "Search (/)", Description: "Search items in focused pane", Section: sectionNavigation, Handler: h.Search},
		CommandAction{ID: "focus-worktrees", Label: "Focus worktrees (1)", Description: "Focus worktree pane", Section: sectionNavigation, Handler: h.FocusWorktree},
		CommandAction{ID: "focus-status", Label: "Focus status (2)", Description: "Focus status pane", Section: sectionNavigation, Handler: h.FocusStatus},
		CommandAction{ID: "focus-log", Label: "Focus log (3)", Description: "Focus log pane", Section: sectionNavigation, Handler: h.FocusLog},
		CommandAction{ID: "sort-cycle", Label: "Cycle sort (s)", Description: "Cycle sort mode (path/active/switched)", Section: sectionNavigation, Handler: h.SortCycle},
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
		CommandAction{ID: "theme", Label: "Select theme", Description: "Change the application theme with live preview", Section: sectionSettings, Handler: h.Theme},
		CommandAction{ID: "help", Label: "Help (?)", Description: "Show help", Section: sectionSettings, Handler: h.Help},
	)
}
