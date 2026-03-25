package git

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// GetMainBranch returns the main branch name for the current repository.
func (s *Service) GetMainBranch(ctx context.Context) string {
	s.mainBranchOnce.Do(func() {
		out := s.RunGit(ctx, []string{"git", "symbolic-ref", "--short", "refs/remotes/origin/HEAD"}, "", []int{0}, true, false)
		if out != "" {
			parts := strings.Split(out, "/")
			if len(parts) > 0 {
				s.mainBranch = parts[len(parts)-1]
			}
		}
		if s.mainBranch == "" {
			s.mainBranch = "main"
		}
	})
	return s.mainBranch
}

func (s *Service) getRemoteURL(ctx context.Context) string {
	s.remoteURLOnce.Do(func() {
		s.remoteURL = strings.TrimSpace(s.RunGit(ctx, []string{"git", "remote", "get-url", "origin"}, "", []int{0}, true, true))
	})
	return s.remoteURL
}

// GetCurrentBranch returns the current branch name from the current working directory.
// Returns an error if not in a git repository or if HEAD is detached.
func (s *Service) GetCurrentBranch(ctx context.Context) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	branchName := s.RunGit(
		ctx,
		[]string{"git", "rev-parse", "--abbrev-ref", "HEAD"},
		cwd,
		[]int{0},
		true,
		false,
	)
	branchName = strings.TrimSpace(branchName)

	if branchName == "" || branchName == "HEAD" {
		return "", fmt.Errorf("not currently on a branch (detached HEAD)")
	}

	return branchName, nil
}

// GetHeadSHA returns the HEAD commit SHA for a worktree path.
func (s *Service) GetHeadSHA(ctx context.Context, worktreePath string) string {
	return s.RunGit(ctx, []string{"git", "rev-parse", "HEAD"}, worktreePath, []int{0}, true, true)
}

// GetMergedBranches returns local branches that have been merged into the specified base branch.
func (s *Service) GetMergedBranches(ctx context.Context, baseBranch string) []string {
	output := s.RunGit(ctx, []string{"git", "branch", "--merged", baseBranch}, "", []int{0}, true, false)
	if output == "" {
		return nil
	}

	var merged []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "* ")
		line = strings.TrimPrefix(line, "+ ")
		line = strings.TrimSpace(line)
		if line == "" || line == baseBranch {
			continue
		}
		merged = append(merged, line)
	}
	return merged
}

func (s *Service) localBranchExists(ctx context.Context, branch string) bool {
	ref := s.RunGit(
		ctx,
		[]string{"git", "show-ref", "--verify", fmt.Sprintf("refs/heads/%s", branch)},
		"",
		[]int{0, 1},
		true,
		true,
	)
	return strings.TrimSpace(ref) != ""
}
