// Package models defines the data objects shared across lazyworktree packages.
package models

import "time"

// CommitFile represents a file changed in a commit.
type CommitFile struct {
	Filename   string
	ChangeType string // A=Added, M=Modified, D=Deleted, R=Renamed, C=Copied
	OldPath    string // For renames: the original path
}

// PRInfo captures the relevant metadata for a pull request.
type PRInfo struct {
	Number      int
	State       string
	Title       string
	Body        string // For branch_name_script input
	URL         string
	Branch      string // Branch name (headRefName for GitHub, source_branch for GitLab)
	Author      string // PR/MR author username
	AuthorName  string // PR/MR author full name
	AuthorIsBot bool   // Whether the author is a bot
}

// IssueInfo captures the relevant metadata for an issue.
type IssueInfo struct {
	Number      int
	State       string
	Title       string
	Body        string // For branch_name_script input
	URL         string
	Author      string // Issue author username
	AuthorName  string // Issue author full name
	AuthorIsBot bool   // Whether the author is a bot
}

// CICheck represents a single CI check/job status.
type CICheck struct {
	Name       string // Name of the check/job
	Status     string // Status: "completed", "in_progress", "queued", "pending"
	Conclusion string // Conclusion: "success", "failure", "skipped", "cancelled", etc.
}

// AISessionStatus represents the current state of an AI session for a worktree.
// Status is written by the AI tool via hooks to a file in the worktree root.
type AISessionStatus struct {
	Status    string    `json:"status"`     // Status: "idle", "working", "error"
	UpdatedAt time.Time `json:"updated_at"` // When the status was last updated
	Message   string    `json:"message"`    // Optional status message
}

// WorktreeInfo summarizes the information for a git worktree.
type WorktreeInfo struct {
	Path           string
	Branch         string
	IsMain         bool
	Dirty          bool
	Ahead          int
	Behind         int
	HasUpstream    bool
	UpstreamBranch string // The upstream branch name (e.g., "origin/main" or "chmouel/feature-branch")
	LastActive     string
	LastActiveTS   int64
	LastSwitchedTS int64 // Unix timestamp of last UI access/switch
	PR             *PRInfo
	AISession      *AISessionStatus // AI session status (nil if no session)
	Untracked      int
	Modified       int
	Staged         int
	Divergence     string
}

const (
	// LastSelectedFilename stores the last worktree selection for a repo.
	LastSelectedFilename = ".last-selected"
	// CacheFilename stores cached worktree metadata for faster loads.
	CacheFilename = ".worktree-cache.json"
	// CommandHistoryFilename stores the command history for the ! command.
	CommandHistoryFilename = ".command-history.json"
	// AccessHistoryFilename stores worktree access timestamps for sorting.
	AccessHistoryFilename = ".worktree-access.json"
	// AISessionStatusFilename stores the AI session status in the worktree root.
	AISessionStatusFilename = ".ai-status.json"
)
