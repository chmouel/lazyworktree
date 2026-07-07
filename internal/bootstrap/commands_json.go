package bootstrap

import "encoding/json"

// flagDescJSON describes a single CLI flag for machine-readable introspection.
type flagDescJSON struct {
	Name    string   `json:"name"`
	Aliases []string `json:"aliases,omitempty"`
	Usage   string   `json:"usage,omitempty"`
	Type    string   `json:"type"`
	Default string   `json:"default,omitempty"`
}

// commandDescJSON describes a CLI command for machine-readable introspection.
type commandDescJSON struct {
	Name        string            `json:"name"`
	Usage       string            `json:"usage,omitempty"`
	ArgsUsage   string            `json:"args_usage,omitempty"`
	Flags       []flagDescJSON    `json:"flags,omitempty"`
	Subcommands []commandDescJSON `json:"subcommands,omitempty"`
}

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

// cleanupItemJSON is a single candidate acted upon during cleanup.
type cleanupItemJSON struct {
	Kind          string `json:"kind"`
	Path          string `json:"path,omitempty"`
	Branch        string `json:"branch,omitempty"`
	Source        string `json:"source,omitempty"`
	BranchDeleted bool   `json:"branch_deleted"`
	Failed        bool   `json:"failed"`
	Error         string `json:"error,omitempty"`
}

// cleanupJSON is the JSON output for the cleanup subcommand.
type cleanupJSON struct {
	Worktrees int               `json:"worktrees"`
	Branches  int               `json:"branches"`
	Orphans   int               `json:"orphans"`
	Failures  int               `json:"failures"`
	Items     []cleanupItemJSON `json:"items"`
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
	Liveness     string `json:"liveness,omitempty"`
	Source       string `json:"source,omitempty"`
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

type jsonErrorEnvelope struct {
	Error jsonError `json:"error"`
}

type jsonError struct {
	Code    string          `json:"code"`
	Message string          `json:"message"`
	Details json.RawMessage `json:"details,omitempty"`
}

type doctorJSON struct {
	Version    string           `json:"version"`
	Build      doctorBuildJSON  `json:"build"`
	Config     doctorConfigJSON `json:"config"`
	Repository doctorRepoJSON   `json:"repository"`
	Tools      doctorToolsJSON  `json:"tools"`
	Checks     doctorChecksJSON `json:"checks"`
}

type doctorBuildJSON struct {
	Commit  string `json:"commit,omitempty"`
	Date    string `json:"date,omitempty"`
	BuiltBy string `json:"built_by,omitempty"`
}

type doctorConfigJSON struct {
	Path        string `json:"path,omitempty"`
	Loaded      bool   `json:"loaded"`
	WorktreeDir string `json:"worktree_dir,omitempty"`
	NoteType    string `json:"note_type,omitempty"`
	NotesPath   string `json:"notes_path,omitempty"`
}

type doctorRepoJSON struct {
	CWD             string `json:"cwd"`
	GitTopLevel     string `json:"git_top_level,omitempty"`
	Repo            string `json:"repo,omitempty"`
	Host            string `json:"host,omitempty"`
	MainWorktree    string `json:"main_worktree,omitempty"`
	CurrentWorktree string `json:"current_worktree,omitempty"`
	InWorktree      bool   `json:"in_worktree"`
	WorktreeCount   int    `json:"worktree_count"`
}

type doctorToolJSON struct {
	Available bool   `json:"available"`
	Path      string `json:"path,omitempty"`
}

type doctorToolsJSON struct {
	Git  doctorToolJSON `json:"git"`
	GH   doctorToolJSON `json:"gh"`
	GLab doctorToolJSON `json:"glab"`
}

type doctorChecksJSON struct {
	CanListWorktrees bool   `json:"can_list_worktrees"`
	ConfigError      string `json:"config_error,omitempty"`
	WorktreeError    string `json:"worktree_error,omitempty"`
}

type machineWorktreeJSON struct {
	Path          string   `json:"path"`
	Name          string   `json:"name"`
	Branch        string   `json:"branch"`
	Repo          string   `json:"repo"`
	IsMain        bool     `json:"is_main"`
	Dirty         bool     `json:"dirty"`
	Ahead         int      `json:"ahead"`
	Behind        int      `json:"behind"`
	Unpushed      int      `json:"unpushed,omitempty"`
	LastActive    string   `json:"last_active,omitempty"`
	Description   string   `json:"description,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	NotePresent   bool     `json:"note_present"`
	NoteUpdatedAt int64    `json:"note_updated_at,omitempty"`
	AgentCount    int      `json:"agent_count"`
	AgentOpen     bool     `json:"agent_open"`
	AgentActivity string   `json:"agent_activity,omitempty"`
}

type machineWorktreeListJSON struct {
	Repo  string                `json:"repo"`
	Count int                   `json:"count"`
	Limit int                   `json:"limit,omitempty"`
	Items []machineWorktreeJSON `json:"items"`
}

type machineWorktreeResolveJSON struct {
	Input      string              `json:"input"`
	ResolvedBy string              `json:"resolved_by"`
	Worktree   machineWorktreeJSON `json:"worktree"`
}

type machineWorktreeContextJSON struct {
	Worktree      machineWorktreeJSON `json:"worktree"`
	Note          *noteShowJSON       `json:"note,omitempty"`
	AgentSessions []agentSessionJSON  `json:"agent_sessions,omitempty"`
}
