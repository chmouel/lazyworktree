package commands

import tea "charm.land/bubbletea/v2"

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

// UpdateShortcuts overwrites action Shortcut fields with user-configured keys.
// It first clears any existing action that held the same shortcut key to
// prevent duplicate display in the palette.
func (r *Registry) UpdateShortcuts(keybindings map[string]string) {
	// Build reverse maps: shortcut → index and id → index for O(1) updates.
	shortcutOwner := make(map[string]int) // key → index in r.actions
	idIndex := make(map[string]int)       // action ID → index in r.actions
	for i, a := range r.actions {
		if a.Shortcut != "" {
			shortcutOwner[a.Shortcut] = i
		}
		if a.ID != "" {
			idIndex[a.ID] = i
		}
	}

	for key, actionID := range keybindings {
		action, ok := r.byID[actionID]
		if !ok {
			continue
		}

		// Clear old owner of this shortcut key (if any and different action).
		if oldIdx, exists := shortcutOwner[key]; exists && r.actions[oldIdx].ID != actionID {
			r.actions[oldIdx].Shortcut = ""
			r.byID[r.actions[oldIdx].ID] = r.actions[oldIdx]
		}

		action.Shortcut = key
		r.byID[actionID] = action
		if idx, ok := idIndex[actionID]; ok {
			r.actions[idx].Shortcut = key
			shortcutOwner[key] = idx
		}
	}
}

// KnownActionIDs returns all registered action IDs.
func (r *Registry) KnownActionIDs() []string {
	ids := make([]string, 0, len(r.byID))
	for id := range r.byID {
		ids = append(ids, id)
	}
	return ids
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
	EditMetadata      func() tea.Cmd
	Annotate          func() tea.Cmd
	SetIcon           func() tea.Cmd
	SetColor          func() tea.Cmd
	SetDescription    func() tea.Cmd
	SetTags           func() tea.Cmd
	BrowseTags        func() tea.Cmd
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

func wtAction(id, label, desc, shortcut string, handler func() tea.Cmd) CommandAction {
	return CommandAction{
		ID:          id,
		Label:       label,
		Description: desc,
		Section:     sectionWorktreeActions,
		Shortcut:    shortcut,
		Icon:        IconWorktree,
		Handler:     handler,
	}
}

func createAction(id, label, desc string, handler func() tea.Cmd) CommandAction {
	return CommandAction{
		ID:          id,
		Label:       label,
		Description: desc,
		Section:     sectionCreateShortcuts,
		Icon:        IconCreate,
		Handler:     handler,
	}
}

// RegisterWorktreeActions registers worktree-related actions.
func RegisterWorktreeActions(r *Registry, h WorktreeHandlers) {
	r.Register(
		wtAction("worktree-create", "Create worktree", "Add a new worktree from base branch or PR/MR", "c", h.Create),
		wtAction("worktree-delete", "Delete worktree", "Remove worktree and branch", "D", h.Delete),
		wtAction("worktree-rename", "Rename worktree", "Rename worktree (and branch when names match)", "m", h.Rename),
		wtAction("worktree-edit-metadata", "Edit worktree metadata", "Choose description, colour, notes, icon, or tags for the selected worktree", "e", h.EditMetadata),
		wtAction("worktree-annotate", "Worktree notes", "View or edit notes for the selected worktree", "", h.Annotate),
		wtAction("worktree-set-icon", "Set worktree icon", "Choose a custom icon for the selected worktree", "", h.SetIcon),
		wtAction("worktree-set-color", "Set worktree colour", "Choose a colour for the selected worktree name", "", h.SetColor),
		wtAction("worktree-set-description", "Set worktree description", "Set a short label replacing the directory name in the list", "", h.SetDescription),
		wtAction("worktree-set-tags", "Set worktree tags", "Type tags or toggle existing labels in one editor", "", h.SetTags),
		wtAction("worktree-browse-tags", "Browse by worktree tags", "Browse worktrees by existing tags and apply an exact tag filter", "", h.BrowseTags),
		wtAction("worktree-absorb", "Absorb worktree", "Merge branch into main and remove worktree", "A", h.Absorb),
		wtAction("worktree-prune", "Prune merged", "Remove merged PR worktrees", "X", h.Prune),
	)

	r.Register(
		createAction("worktree-create-from-current", "Create from current branch", "Create from current branch with or without changes", h.CreateFromCurrent),
		createAction("worktree-create-from-branch", "Create from branch/tag", "Select a branch, tag, or remote as base", h.CreateFromBranch),
		createAction("worktree-create-from-commit", "Create from commit", "Choose a branch, then select a specific commit", h.CreateFromCommit),
		createAction("worktree-create-from-pr", "Create from PR/MR", "Create from a pull/merge request", h.CreateFromPR),
		createAction("worktree-create-from-issue", "Create from issue", "Create from a GitHub/GitLab issue", h.CreateFromIssue),
		createAction("worktree-create-freeform", "Create from ref", "Enter a branch, tag, or commit manually", h.CreateFreeform),
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
		CommandAction{ID: "git-diff", Label: "Show diff", Description: "Show diff for current worktree or commit", Section: sectionGitOperations, Shortcut: "d", Icon: IconGit, Handler: h.ShowDiff},
		CommandAction{ID: "git-refresh", Label: "Refresh", Description: "Reload worktrees", Section: sectionGitOperations, Shortcut: "r", Icon: IconGit, Handler: h.Refresh},
		CommandAction{ID: "git-fetch", Label: "Fetch remotes", Description: "git fetch --all", Section: sectionGitOperations, Shortcut: "R", Icon: IconGit, Handler: h.Fetch},
		CommandAction{ID: "git-push", Label: "Push to upstream", Description: "git push (clean worktree only)", Section: sectionGitOperations, Shortcut: "P", Icon: IconGit, Handler: h.Push},
		CommandAction{ID: "git-sync", Label: "Synchronise with upstream", Description: "git pull, then git push (clean worktree only)", Section: sectionGitOperations, Shortcut: "S", Icon: IconGit, Handler: h.Sync},
		CommandAction{ID: "git-fetch-pr-data", Label: "Fetch PR data", Description: "Fetch PR/MR status from GitHub/GitLab", Section: sectionGitOperations, Shortcut: "p", Icon: IconGit, Handler: h.FetchPRData},
		CommandAction{ID: "git-ci-checks", Label: "View CI checks", Description: "View CI check logs for current worktree", Section: sectionGitOperations, Shortcut: "v", Icon: IconGit, Handler: h.ViewCIChecks, Available: h.CIChecksAvailable},
		CommandAction{ID: "git-pr", Label: "Open in browser", Description: "Open PR, branch, or repo in browser", Section: sectionGitOperations, Shortcut: "o", Icon: IconGit, Handler: h.OpenPR},
		CommandAction{ID: "git-lazygit", Label: "Open LazyGit", Description: "Open LazyGit in selected worktree", Section: sectionGitOperations, Shortcut: "g", Icon: IconGit, Handler: h.OpenLazyGit},
		CommandAction{ID: "git-run-command", Label: "Run command", Description: "Run arbitrary shell command in worktree", Section: sectionGitOperations, Shortcut: "!", Icon: IconGit, Handler: h.RunCommand},
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
		CommandAction{ID: "status-stage-file", Label: "Stage/unstage file", Description: "Stage or unstage selected file", Section: sectionStatusPane, Shortcut: "s", Icon: IconStatus, Handler: h.StageFile},
		CommandAction{ID: "status-commit-staged", Label: "Open commit screen", Description: "Open the commit screen for staged changes (or prompt to stage all)", Section: sectionStatusPane, Shortcut: "c", Icon: IconStatus, Handler: h.CommitStaged},
		CommandAction{ID: "status-commit-all", Label: "Commit changes using git editor", Description: "Commit using git editor", Section: sectionStatusPane, Shortcut: "C", Icon: IconStatus, Handler: h.CommitAll},
		CommandAction{ID: "status-edit-file", Label: "Edit file", Description: "Open selected file in editor", Section: sectionStatusPane, Shortcut: "e", Icon: IconStatus, Handler: h.EditFile},
		CommandAction{ID: "status-delete-file", Label: "Delete selected file or directory", Section: sectionStatusPane, Icon: IconStatus, Handler: h.DeleteFile},
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
		CommandAction{ID: "log-cherry-pick", Label: "Cherry-pick commit", Description: "Cherry-pick commit to another worktree", Section: sectionLogPane, Shortcut: "C", Icon: IconLog, Handler: h.CherryPick},
		CommandAction{ID: "log-commit-view", Label: "Browse commit files", Description: "Browse files changed in selected commit", Section: sectionLogPane, Icon: IconLog, Handler: h.CommitView},
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
		CommandAction{ID: "nav-zoom-toggle", Label: "Toggle zoom", Description: "Toggle zoom on focused pane", Section: sectionNavigation, Shortcut: "=", Icon: IconNavigation, Handler: h.ToggleZoom},
		CommandAction{ID: "nav-toggle-layout", Label: "Toggle layout", Description: "Switch between default and top layout", Section: sectionNavigation, Shortcut: "L", Icon: IconNavigation, Handler: h.ToggleLayout},
		CommandAction{ID: "nav-filter", Label: "Filter", Description: "Filter items in focused pane", Section: sectionNavigation, Shortcut: "f", Icon: IconNavigation, Handler: h.Filter},
		CommandAction{ID: "nav-search", Label: "Search", Description: "Search items in focused pane", Section: sectionNavigation, Shortcut: "/", Icon: IconNavigation, Handler: h.Search},
		CommandAction{ID: "nav-focus-worktrees", Label: "Focus worktrees", Description: "Focus worktree pane", Section: sectionNavigation, Shortcut: "1", Icon: IconNavigation, Handler: h.FocusWorktree},
		CommandAction{ID: "nav-focus-status", Label: "Focus status", Description: "Focus status pane", Section: sectionNavigation, Shortcut: "2", Icon: IconNavigation, Handler: h.FocusStatus},
		CommandAction{ID: "nav-focus-log", Label: "Focus log", Description: "Focus log pane", Section: sectionNavigation, Shortcut: "3", Icon: IconNavigation, Handler: h.FocusLog},
		CommandAction{ID: "nav-sort-cycle", Label: "Cycle sort", Description: "Cycle sort mode (path/active/switched)", Section: sectionNavigation, Shortcut: "s", Icon: IconNavigation, Handler: h.SortCycle},
	)
}

// ClipboardHandlers holds callbacks for clipboard actions.
type ClipboardHandlers struct {
	CopyPath   func() tea.Cmd
	CopyBranch func() tea.Cmd
	CopyPRURL  func() tea.Cmd
}

// RegisterClipboardActions registers clipboard actions.
func RegisterClipboardActions(r *Registry, h ClipboardHandlers) {
	r.Register(
		CommandAction{ID: "nav-copy-path", Label: "Copy path / file / SHA", Description: "Copy context-aware content (path, file, or commit SHA)", Section: sectionNavigation, Shortcut: "y", Icon: IconNavigation, Handler: h.CopyPath},
		CommandAction{ID: "nav-copy-branch", Label: "Copy branch name", Description: "Copy selected worktree branch name", Section: sectionNavigation, Shortcut: "Y", Icon: IconNavigation, Handler: h.CopyBranch},
		CommandAction{ID: "nav-copy-pr-url", Label: "Copy PR/MR URL", Description: "Copy selected worktree PR/MR URL", Section: sectionNavigation, Icon: IconNavigation, Handler: h.CopyPRURL},
	)
}

// SettingsHandlers holds callbacks for settings actions.
type SettingsHandlers struct {
	Theme     func() tea.Cmd
	Help      func() tea.Cmd
	Taskboard func() tea.Cmd
}

// RegisterSettingsActions registers settings actions.
func RegisterSettingsActions(r *Registry, h SettingsHandlers) {
	r.Register(
		CommandAction{ID: "settings-theme", Label: "Select theme", Description: "Change the application theme with live preview", Section: sectionSettings, Icon: IconSettings, Handler: h.Theme},
		CommandAction{ID: "settings-taskboard", Label: "Taskboard", Description: "Browse and toggle worktree tasks", Section: sectionSettings, Shortcut: "T", Icon: IconSettings, Handler: h.Taskboard},
		CommandAction{ID: "settings-help", Label: "Help", Description: "Show help", Section: sectionSettings, Shortcut: "?", Icon: IconSettings, Handler: h.Help},
	)
}
