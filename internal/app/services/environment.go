package services

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/chmouel/lazyworktree/internal/models"
)

// Environment variable names managed by LazyWorktree command execution.
const (
	EnvWorktreeBranch            = "WORKTREE_BRANCH"
	EnvMainWorktreePath          = "MAIN_WORKTREE_PATH"
	EnvWorktreePath              = "WORKTREE_PATH"
	EnvWorktreeName              = "WORKTREE_NAME"
	EnvRepoName                  = "REPO_NAME"
	EnvRepoOwner                 = "REPO_OWNER"
	EnvRepoRepoName              = "REPO_REPONAME"
	EnvLazyWorktreeType          = "LAZYWORKTREE_TYPE"
	EnvLazyWorktreeNumber        = "LAZYWORKTREE_NUMBER"
	EnvLazyWorktreeTemplate      = "LAZYWORKTREE_TEMPLATE"
	EnvLazyWorktreeSuggestedName = "LAZYWORKTREE_SUGGESTED_NAME"
	EnvLazyWorktreeTitle         = "LAZYWORKTREE_TITLE"
	EnvLazyWorktreeURL           = "LAZYWORKTREE_URL"
	EnvLazyWorktreeDescription   = "LAZYWORKTREE_DESCRIPTION"
)

var managedCommandEnvKeys = map[string]struct{}{
	EnvWorktreeBranch:            {},
	EnvMainWorktreePath:          {},
	EnvWorktreePath:              {},
	EnvWorktreeName:              {},
	EnvRepoName:                  {},
	EnvRepoOwner:                 {},
	EnvRepoRepoName:              {},
	EnvLazyWorktreeType:          {},
	EnvLazyWorktreeNumber:        {},
	EnvLazyWorktreeTemplate:      {},
	EnvLazyWorktreeSuggestedName: {},
	EnvLazyWorktreeTitle:         {},
	EnvLazyWorktreeURL:           {},
	EnvLazyWorktreeDescription:   {},
}

// LazyWorktreeContext holds optional source metadata exposed to command env.
type LazyWorktreeContext struct {
	Type          string
	Number        string
	Template      string
	SuggestedName string
	Title         string
	URL           string
	Description   string
}

// BuildCommandEnv builds environment variables for worktree commands.
func BuildCommandEnv(branch, wtPath, repoKey, mainWorktreePath string) map[string]string {
	return BuildCommandEnvWithContext(branch, wtPath, repoKey, mainWorktreePath, LazyWorktreeContext{})
}

// BuildCommandEnvWithContext builds environment variables for worktree commands,
// including managed LAZYWORKTREE_* keys. Contextual values are empty when the
// source metadata is unavailable.
func BuildCommandEnvWithContext(branch, wtPath, repoKey, mainWorktreePath string, lazyCtx LazyWorktreeContext) map[string]string {
	owner, repo := SplitRepoKey(repoKey)
	return map[string]string{
		EnvWorktreeBranch:            branch,
		EnvMainWorktreePath:          mainWorktreePath,
		EnvWorktreePath:              wtPath,
		EnvWorktreeName:              worktreeName(wtPath),
		EnvRepoName:                  repoKey,
		EnvRepoOwner:                 owner,
		EnvRepoRepoName:              repo,
		EnvLazyWorktreeType:          lazyCtx.Type,
		EnvLazyWorktreeNumber:        lazyCtx.Number,
		EnvLazyWorktreeTemplate:      lazyCtx.Template,
		EnvLazyWorktreeSuggestedName: lazyCtx.SuggestedName,
		EnvLazyWorktreeTitle:         lazyCtx.Title,
		EnvLazyWorktreeURL:           lazyCtx.URL,
		EnvLazyWorktreeDescription:   lazyCtx.Description,
	}
}

// LazyWorktreeContextFromPR returns command context for PR/MR-backed worktrees.
func LazyWorktreeContextFromPR(pr *models.PRInfo, template, suggestedName string) LazyWorktreeContext {
	if pr == nil {
		return LazyWorktreeContext{}
	}
	return LazyWorktreeContext{
		Type:          "pr",
		Number:        strconv.Itoa(pr.Number),
		Template:      template,
		SuggestedName: suggestedName,
		Title:         pr.Title,
		URL:           pr.URL,
		Description:   pr.Body,
	}
}

// LazyWorktreeContextFromIssue returns command context for issue-backed creation.
func LazyWorktreeContextFromIssue(issue *models.IssueInfo, template, suggestedName string) LazyWorktreeContext {
	if issue == nil {
		return LazyWorktreeContext{}
	}
	return LazyWorktreeContext{
		Type:          "issue",
		Number:        strconv.Itoa(issue.Number),
		Template:      template,
		SuggestedName: suggestedName,
		Title:         issue.Title,
		URL:           issue.URL,
		Description:   issue.Body,
	}
}

// SplitRepoKey splits a repository key like "owner/repo" into owner and repo.
// If the key contains no slash, owner is empty and repo is the full key.
func SplitRepoKey(repoKey string) (owner, repo string) {
	if i := strings.IndexByte(repoKey, '/'); i >= 0 {
		return repoKey[:i], repoKey[i+1:]
	}
	return "", repoKey
}

// ExpandWithEnv expands environment variables using the provided map first.
func ExpandWithEnv(input string, env map[string]string) string {
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

// EnvMapToList converts environment variables to KEY=VALUE pairs.
func EnvMapToList(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, fmt.Sprintf("%s=%s", key, env[key]))
	}
	return out
}

// IsManagedCommandEnvKey reports whether key is controlled by lazyworktree.
func IsManagedCommandEnvKey(key string) bool {
	_, ok := managedCommandEnvKeys[key]
	return ok
}

// FilterManagedCommandEnv removes lazyworktree-managed variables from environ.
func FilterManagedCommandEnv(environ []string) []string {
	filtered := make([]string, 0, len(environ))
	for _, entry := range environ {
		key, _, _ := strings.Cut(entry, "=")
		if IsManagedCommandEnvKey(key) {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

// AppendCommandEnv appends generated command env after filtering stale managed
// values from the parent environment.
func AppendCommandEnv(environ []string, env map[string]string) []string {
	return append(FilterManagedCommandEnv(environ), EnvMapToList(env)...)
}

func worktreeName(wtPath string) string {
	if strings.TrimSpace(wtPath) == "" {
		return ""
	}
	return filepath.Base(wtPath)
}
