package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chmouel/lazyworktree/internal/models"
)

// WorktreeService handles high-level worktree lifecycle and synchronization operations.
type WorktreeService interface {
	// Create creates a new worktree.
	Create(ctx context.Context, opts CreateOptions) error

	// CreateFromChanges moves changes from an existing worktree to a new one.
	CreateFromChanges(ctx context.Context, opts CreateFromChangesOptions) error

	// Delete removes a worktree and optionally its branch.
	Delete(ctx context.Context, path, branch string, deleteBranch bool) error

	// Rename moves a worktree and conditionally renames its branch.
	Rename(ctx context.Context, oldPath, newPath, oldBranch, newBranch string) error

	// Push pushes a worktree's branch to its upstream.
	Push(ctx context.Context, wt *models.WorktreeInfo, args []string, env map[string]string) (string, error)

	// Sync synchronizes a worktree's branch with its upstream (pull + push).
	Sync(ctx context.Context, wt *models.WorktreeInfo, pullArgs, pushArgs []string, env map[string]string) (string, error)

	// UpdateFromBase updates a branch from its PR base branch.
	UpdateFromBase(ctx context.Context, wt *models.WorktreeInfo, mergeMethod string, env map[string]string) (string, error)

	// Absorb merges or rebases a worktree into the main branch.
	Absorb(ctx context.Context, wt *models.WorktreeInfo, mainWorktree *models.WorktreeInfo, mergeMethod string) error

	// GetPruneCandidates identifies worktrees and stale branches that have been merged and are candidates for pruning.
	GetPruneCandidates(ctx context.Context, worktrees []*models.WorktreeInfo, includeStaleBranches bool) ([]PruneCandidate, error)

	// ExecuteCommands runs a list of shell commands in the specified directory.
	ExecuteCommands(ctx context.Context, commands []string, cwd string, env map[string]string) error
}

// CreateOptions contains parameters for worktree creation.
type CreateOptions struct {
	Branch     string
	TargetPath string
	BaseRef    string
	Env        map[string]string
}

// CreateFromChangesOptions contains parameters for creating a worktree from existing changes.
type CreateFromChangesOptions struct {
	SourcePath    string
	NewBranch     string
	TargetPath    string
	CurrentBranch string
	Env           map[string]string
}

// PruneCandidate represents a worktree or stale branch that is a candidate for pruning.
type PruneCandidate struct {
	Worktree *models.WorktreeInfo // nil for branch-only candidates
	Branch   string               // branch name (always set)
	Source   string               // "pr", "git", or "both"
}

// GitService defines the subset of git operations needed by WorktreeService.
type GitService interface {
	RunGit(ctx context.Context, args []string, cwd string, okReturncodes []int, strip, silent bool) string
	RunCommandChecked(ctx context.Context, args []string, cwd, errorPrefix string) bool
	GetMainBranch(ctx context.Context) string
	GetMergedBranches(ctx context.Context, baseBranch string) []string
	RenameWorktree(ctx context.Context, oldPath, newPath, oldBranch, newBranch string) bool
	ExecuteCommands(ctx context.Context, cmdList []string, cwd string, env map[string]string) error
	RunGitWithCombinedOutput(ctx context.Context, args []string, cwd string, env map[string]string) ([]byte, error)
}

type worktreeService struct {
	git GitService
}

// NewWorktreeService creates a new WorktreeService.
func NewWorktreeService(git GitService) WorktreeService {
	return &worktreeService{
		git: git,
	}
}

func (s *worktreeService) Create(ctx context.Context, opts CreateOptions) error {
	if err := os.MkdirAll(filepath.Dir(opts.TargetPath), 0o750); err != nil {
		return fmt.Errorf("failed to create worktree directory: %w", err)
	}

	args := []string{"git", "worktree", "add", "-b", opts.Branch, opts.TargetPath, opts.BaseRef}
	if !s.git.RunCommandChecked(ctx, args, "", fmt.Sprintf("Failed to create worktree %s", opts.Branch)) {
		return fmt.Errorf("failed to create worktree %s", opts.Branch)
	}

	return nil
}

func (s *worktreeService) CreateFromChanges(ctx context.Context, opts CreateFromChangesOptions) error {
	if err := os.MkdirAll(filepath.Dir(opts.TargetPath), 0o750); err != nil {
		return fmt.Errorf("failed to create worktree directory: %w", err)
	}

	// Stash changes with descriptive message
	stashMessage := fmt.Sprintf("git-wt-create move-current: %s", opts.NewBranch)
	if !s.git.RunCommandChecked(
		ctx,
		[]string{"git", "stash", "push", "-u", "-m", stashMessage},
		opts.SourcePath,
		"Failed to create stash for moving changes",
	) {
		return fmt.Errorf("failed to create stash for moving changes")
	}

	// Get the stash ref
	stashRef := s.git.RunGit(ctx, []string{"git", "stash", "list", "-1", "--format=%gd"}, "", []int{0}, true, false)
	if stashRef == "" || !strings.HasPrefix(stashRef, "stash@{") {
		// Try to restore stash if we can't get the ref
		s.git.RunCommandChecked(ctx, []string{"git", "stash", "pop"}, opts.SourcePath, "Failed to restore stash")
		return fmt.Errorf("failed to get stash reference")
	}

	// Create the new worktree from current branch
	if !s.git.RunCommandChecked(
		ctx,
		[]string{"git", "worktree", "add", "-b", opts.NewBranch, opts.TargetPath, opts.CurrentBranch},
		"",
		fmt.Sprintf("Failed to create worktree %s", opts.NewBranch),
	) {
		// If worktree creation fails, try to restore the stash
		s.git.RunCommandChecked(ctx, []string{"git", "stash", "pop"}, opts.SourcePath, "Failed to restore stash")
		return fmt.Errorf("failed to create worktree %s", opts.NewBranch)
	}

	// Apply stash to the new worktree
	if !s.git.RunCommandChecked(
		ctx,
		[]string{"git", "stash", "apply", "--index", stashRef},
		opts.TargetPath,
		"Failed to apply stash to new worktree",
	) {
		// If stash apply fails, clean up the worktree and try to restore stash to original location
		s.git.RunCommandChecked(ctx, []string{"git", "worktree", "remove", "--force", opts.TargetPath}, "", "Failed to remove worktree")
		s.git.RunCommandChecked(ctx, []string{"git", "stash", "pop"}, opts.SourcePath, "Failed to restore stash")
		return fmt.Errorf("failed to apply stash to new worktree")
	}

	// Drop the stash from the original location
	s.git.RunCommandChecked(ctx, []string{"git", "stash", "drop", stashRef}, opts.SourcePath, "Failed to drop stash")

	return nil
}

func (s *worktreeService) Delete(ctx context.Context, path, branch string, deleteBranch bool) error {
	if path != "" {
		if !s.git.RunCommandChecked(ctx, []string{"git", "worktree", "remove", "--force", path}, "", fmt.Sprintf("Failed to remove worktree %s", path)) {
			return fmt.Errorf("failed to remove worktree %s", path)
		}
	}

	if deleteBranch && branch != "" {
		if !s.git.RunCommandChecked(ctx, []string{"git", "branch", "-D", branch}, "", fmt.Sprintf("Failed to delete branch %s", branch)) {
			return fmt.Errorf("failed to delete branch %s", branch)
		}
	}

	return nil
}

func (s *worktreeService) Rename(ctx context.Context, oldPath, newPath, oldBranch, newBranch string) error {
	if !s.git.RenameWorktree(ctx, oldPath, newPath, oldBranch, newBranch) {
		return fmt.Errorf("failed to rename %s to %s", oldBranch, newBranch)
	}
	return nil
}

func (s *worktreeService) Push(ctx context.Context, wt *models.WorktreeInfo, args []string, env map[string]string) (string, error) {
	fullArgs := append([]string{"git", "push"}, args...)
	out, err := s.git.RunGitWithCombinedOutput(ctx, fullArgs, wt.Path, env)
	return strings.TrimSpace(string(out)), err
}

func (s *worktreeService) Sync(ctx context.Context, wt *models.WorktreeInfo, pullArgs, pushArgs []string, env map[string]string) (string, error) {
	pullFullArgs := append([]string{"git", "pull"}, pullArgs...)
	pullOut, err := s.git.RunGitWithCombinedOutput(ctx, pullFullArgs, wt.Path, env)
	if err != nil {
		return strings.TrimSpace(string(pullOut)), fmt.Errorf("pull failed: %w", err)
	}

	pushFullArgs := append([]string{"git", "push"}, pushArgs...)
	pushOut, err := s.git.RunGitWithCombinedOutput(ctx, pushFullArgs, wt.Path, env)
	combined := strings.TrimSpace(string(pullOut) + "\n" + string(pushOut))
	if err != nil {
		return combined, fmt.Errorf("push failed: %w", err)
	}

	return combined, nil
}

func (s *worktreeService) UpdateFromBase(ctx context.Context, wt *models.WorktreeInfo, mergeMethod string, env map[string]string) (string, error) {
	args := []string{"gh", "pr", "update-branch"}
	if mergeMethod == "rebase" {
		args = append(args, "--rebase")
	}

	out, err := s.git.RunGitWithCombinedOutput(ctx, args, wt.Path, env)
	return strings.TrimSpace(string(out)), err
}

func (s *worktreeService) Absorb(ctx context.Context, wt, mainWorktree *models.WorktreeInfo, mergeMethod string) error {
	mainBranch := s.git.GetMainBranch(ctx)
	mainPath := mainWorktree.Path

	if mergeMethod == "rebase" {
		// Rebase: first rebase the feature branch onto main, then fast-forward main
		if !s.git.RunCommandChecked(ctx, []string{"git", "-C", wt.Path, "rebase", mainBranch}, "", fmt.Sprintf("Failed to rebase %s onto %s", wt.Branch, mainBranch)) {
			return fmt.Errorf("rebase failed; resolve conflicts in %s and retry", wt.Path)
		}
		// Fast-forward main to the rebased branch
		if !s.git.RunCommandChecked(ctx, []string{"git", "-C", mainPath, "merge", "--ff-only", wt.Branch}, "", fmt.Sprintf("Failed to fast-forward %s to %s", mainBranch, wt.Branch)) {
			return fmt.Errorf("fast-forward failed; the branch may have diverged")
		}
	} else if !s.git.RunCommandChecked(ctx, []string{"git", "-C", mainPath, "merge", "--no-edit", wt.Branch}, "", fmt.Sprintf("Failed to merge %s into %s", wt.Branch, mainBranch)) {
		return fmt.Errorf("merge failed; resolve conflicts in %s and retry", mainPath)
	}

	return nil
}

func (s *worktreeService) GetPruneCandidates(ctx context.Context, worktrees []*models.WorktreeInfo, includeStaleBranches bool) ([]PruneCandidate, error) {
	mainBranch := s.git.GetMainBranch(ctx)

	wtBranches := make(map[string]*models.WorktreeInfo)
	checkedOutBranches := make(map[string]struct{})
	for _, wt := range worktrees {
		if wt.Branch != "" {
			checkedOutBranches[wt.Branch] = struct{}{}
		}
		if !wt.IsMain {
			wtBranches[wt.Branch] = wt
		}
	}

	candidateMap := make(map[string]PruneCandidate)

	// 1. PR-based detection
	for _, wt := range worktrees {
		if wt.IsMain {
			continue
		}
		if wt.PR != nil && strings.EqualFold(wt.PR.State, "MERGED") {
			candidateMap[wt.Branch] = PruneCandidate{Worktree: wt, Branch: wt.Branch, Source: "pr"}
		}
	}

	// 2. Git-based detection
	mergedBranches := s.git.GetMergedBranches(ctx, mainBranch)
	for _, branch := range mergedBranches {
		if wt, exists := wtBranches[branch]; exists {
			if existing, found := candidateMap[branch]; found {
				existing.Source = "both"
				candidateMap[branch] = existing
			} else {
				candidateMap[branch] = PruneCandidate{Worktree: wt, Branch: branch, Source: "git"}
			}
		}
	}

	// 3. Branch-only detection: merged branches with no worktree
	if includeStaleBranches {
		for _, branch := range mergedBranches {
			if _, checkedOut := checkedOutBranches[branch]; checkedOut {
				continue
			}
			if _, exists := wtBranches[branch]; !exists {
				if _, found := candidateMap[branch]; !found {
					candidateMap[branch] = PruneCandidate{Worktree: nil, Branch: branch, Source: "git"}
				}
			}
		}
	}

	candidates := make([]PruneCandidate, 0, len(candidateMap))
	for _, c := range candidateMap {
		candidates = append(candidates, c)
	}

	return candidates, nil
}

func (s *worktreeService) ExecuteCommands(ctx context.Context, commands []string, cwd string, env map[string]string) error {
	return s.git.ExecuteCommands(ctx, commands, cwd, env)
}
