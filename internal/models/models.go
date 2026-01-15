// Package models defines the data objects shared across lazyworktree packages.
package models

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
	BaseBranch  string // Base branch name (baseRefName for GitHub, target_branch for GitLab)
	Author      string // PR/MR author username
	AuthorName  string // PR/MR author full name
	AuthorIsBot bool   // Whether the author is a bot
	IsDraft     bool   // Whether the PR is a draft
	CIStatus    string // Computed CI status: "success", "failure", "pending", "none"
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
	PRFetchError   string // Stores error message if PR fetch failed
	PRFetchStatus  string // "not_fetched", "fetching", "loaded", "error", "no_pr"
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
	// CommandPaletteHistoryFilename stores command palette usage history for MRU sorting.
	CommandPaletteHistoryFilename = ".command-palette-history.json"
)

// PR fetch status values for WorktreeInfo.PRFetchStatus field.
const (
	PRFetchStatusNotFetched = "not_fetched" // PR data has not been fetched yet
	PRFetchStatusFetching   = "fetching"    // PR data is currently being fetched
	PRFetchStatusLoaded     = "loaded"      // PR data was successfully loaded
	PRFetchStatusError      = "error"       // PR fetch encountered an error
	PRFetchStatusNoPR       = "no_pr"       // No PR exists for this branch
)
