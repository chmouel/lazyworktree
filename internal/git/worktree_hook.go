package git

import (
	"context"
	"path/filepath"

	"github.com/chmouel/lazyworktree/internal/models"
)

// WorktreeHook lets an integration create worktrees in place of git and follow
// what happens to them until their setup is complete.
type WorktreeHook interface {
	// AddWorktree creates the worktree. runGit runs the original git command,
	// for cases the hook leaves to git; its error carries git's output.
	AddWorktree(ctx context.Context, add models.WorktreeAdd, runGit func() error) error
	// WorktreeRemoved reports a `git worktree remove` of path and whether it
	// succeeded, so a rolled-back worktree can be cleaned up.
	WorktreeRemoved(ctx context.Context, path string, removed bool)
	// FinaliseWorktree completes a new worktree once its init commands are done.
	FinaliseWorktree(ctx context.Context, path string, env map[string]string) error
	// ForgetWorktree stops tracking a new worktree that keeps its basic setup.
	ForgetWorktree(path string)
}

// SetWorktreeHook installs hook for later worktree commands. A nil hook leaves
// them to git.
func (s *Service) SetWorktreeHook(hook WorktreeHook) {
	s.worktreeHook = hook
}

// WorktreeHook returns the installed worktree hook, if any.
func (s *Service) WorktreeHook() WorktreeHook {
	return s.worktreeHook
}

// parseWorktreeAdd recognises `git worktree add [-b NEW [--track]] PATH REF`,
// the only forms lazyworktree issues. Anything else is left to git.
func parseWorktreeAdd(args []string, cwd string) (models.WorktreeAdd, bool) {
	if len(args) < 5 || args[0] != "git" || args[1] != "worktree" || args[2] != "add" {
		return models.WorktreeAdd{}, false
	}
	rest := args[3:]
	add := models.WorktreeAdd{Dir: cwd}
	if rest[0] == "-b" {
		if len(rest) < 2 || rest[1] == "" {
			return models.WorktreeAdd{}, false
		}
		add.NewBranch = rest[1]
		rest = rest[2:]
		if len(rest) > 0 && rest[0] == "--track" {
			add.Track = true
			rest = rest[1:]
		}
	}
	if len(rest) != 2 || rest[0] == "" || rest[1] == "" || rest[0][0] == '-' || rest[1][0] == '-' {
		return models.WorktreeAdd{}, false
	}
	add.Path = absWorktreePath(rest[0], cwd)
	add.Ref = rest[1]
	return add, add.Path != ""
}

// parseWorktreeRemove recognises `git worktree remove [--force] PATH`.
func parseWorktreeRemove(args []string, cwd string) (string, bool) {
	if len(args) < 4 || args[0] != "git" || args[1] != "worktree" || args[2] != "remove" {
		return "", false
	}
	rest := args[3:]
	if rest[0] == "--force" {
		rest = rest[1:]
	}
	if len(rest) != 1 || rest[0] == "" || rest[0][0] == '-' {
		return "", false
	}
	path := absWorktreePath(rest[0], cwd)
	return path, path != ""
}

func absWorktreePath(path, cwd string) string {
	if !filepath.IsAbs(path) && cwd != "" {
		path = filepath.Join(cwd, path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	return abs
}
