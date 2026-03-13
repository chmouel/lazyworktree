package bootstrap

// JSON response types for CLI output.
// All mutating commands support a --json flag that emits one of these types to stdout.
// Progress and diagnostic messages continue to go to stderr.

// createJSON is the JSON output for the create subcommand.
type createJSON struct {
	Path        string   `json:"path"`
	Name        string   `json:"name"`
	Branch      string   `json:"branch"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// deleteJSON is the JSON output for the delete subcommand.
type deleteJSON struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	BranchDeleted bool   `json:"branch_deleted"`
}

// renameJSON is the JSON output for the rename subcommand.
type renameJSON struct {
	OldName string `json:"old_name"`
	OldPath string `json:"old_path"`
	NewName string `json:"new_name"`
	NewPath string `json:"new_path"`
}

// noteShowJSON is the JSON output for the note show subcommand.
type noteShowJSON struct {
	WorktreeName string   `json:"worktree_name"`
	Path         string   `json:"path"`
	Note         string   `json:"note,omitempty"`
	Description  string   `json:"description,omitempty"`
	Icon         string   `json:"icon,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	UpdatedAt    int64    `json:"updated_at,omitempty"`
}

// execJSON is the JSON output for the exec subcommand in command mode.
// ExitCode is the exit code of the child process; lazyworktree itself exits 0 regardless.
type execJSON struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Command  string `json:"command"`
	ExitCode int    `json:"exit_code"`
}

// agentSessionJSON is the JSON representation of an agent session within list output.
type agentSessionJSON struct {
	ID           string `json:"id"`
	Agent        string `json:"agent"`
	Status       string `json:"status"`
	Activity     string `json:"activity"`
	IsOpen       bool   `json:"is_open"`
	LastActivity string `json:"last_activity,omitempty"`
	TaskLabel    string `json:"task_label,omitempty"`
	Model        string `json:"model,omitempty"`
}

// worktreeJSONExtended is the enriched JSON output for the list subcommand.
// It extends the base worktree fields with note metadata and agent session information.
type worktreeJSONExtended struct {
	Path          string             `json:"path"`
	Name          string             `json:"name"`
	Branch        string             `json:"branch"`
	IsMain        bool               `json:"is_main"`
	Dirty         bool               `json:"dirty"`
	Ahead         int                `json:"ahead"`
	Behind        int                `json:"behind"`
	Unpushed      int                `json:"unpushed,omitempty"`
	LastActive    string             `json:"last_active"`
	Description   string             `json:"description,omitempty"`
	Tags          []string           `json:"tags,omitempty"`
	NotePresent   bool               `json:"note_present"`
	NoteUpdatedAt int64              `json:"note_updated_at,omitempty"`
	AgentSessions []agentSessionJSON `json:"agent_sessions,omitempty"`
	AgentOpen     bool               `json:"agent_open"`
	AgentActivity string             `json:"agent_activity,omitempty"`
	AgentCount    int                `json:"agent_count"`
}
