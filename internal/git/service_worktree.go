package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/chmouel/lazyworktree/internal/models"
)

// GetWorktrees parses git worktree metadata and returns the list of worktrees.
// This method concurrently fetches status information for each worktree to improve performance.
// The first worktree in the list is marked as the main worktree.
func (s *Service) GetWorktrees(ctx context.Context) ([]*models.WorktreeInfo, error) {
	rawWts := s.RunGit(ctx, []string{"git", "worktree", "list", "--porcelain"}, "", []int{0}, true, false)
	if rawWts == "" {
		return []*models.WorktreeInfo{}, nil
	}

	type wtData struct {
		path   string
		branch string
		isMain bool
	}

	var wts []wtData
	var currentWt *wtData

	lines := strings.SplitSeq(rawWts, "\n")
	for line := range lines {
		if strings.HasPrefix(line, "worktree ") {
			if currentWt != nil {
				wts = append(wts, *currentWt)
			}
			path := strings.TrimPrefix(line, "worktree ")
			currentWt = &wtData{path: path}
		} else if strings.HasPrefix(line, "branch ") {
			if currentWt != nil {
				branch := strings.TrimPrefix(line, "branch ")
				branch = strings.TrimPrefix(branch, "refs/heads/")
				currentWt.branch = branch
			}
		}
	}
	if currentWt != nil {
		wts = append(wts, *currentWt)
	}

	for i := range wts {
		wts[i].isMain = i == 0
	}

	branchRaw := s.RunGit(ctx, []string{
		"git", "for-each-ref",
		"--format=%(refname:short)|%(committerdate:relative)|%(committerdate:unix)",
		"refs/heads",
	}, "", []int{0}, true, false)

	branchInfo := make(map[string]struct {
		lastActive   string
		lastActiveTS int64
	})

	for line := range strings.SplitSeq(branchRaw, "\n") {
		if strings.Contains(line, "|") {
			parts := strings.Split(line, "|")
			if len(parts) == 3 {
				branch := parts[0]
				lastActive := parts[1]
				lastActiveTS, _ := strconv.ParseInt(parts[2], 10, 64)
				branchInfo[branch] = struct {
					lastActive   string
					lastActiveTS int64
				}{lastActive: lastActive, lastActiveTS: lastActiveTS}
			}
		}
	}

	type result struct {
		wt  *models.WorktreeInfo
		err error
	}

	results := make(chan result, len(wts))
	var wg sync.WaitGroup

	for _, wt := range wts {
		wg.Add(1)
		go func(wtData wtData) {
			defer wg.Done()
			s.acquireSemaphore()
			defer s.releaseSemaphore()

			path := wtData.path
			branch := wtData.branch
			if branch == "" {
				branch = "(detached)"
			}

			statusRaw := s.RunGit(ctx, []string{"git", "status", "--porcelain=v2", "--branch"}, path, []int{0}, true, false)

			ahead := 0
			behind := 0
			hasUpstream := false
			upstreamBranch := ""
			untracked := 0
			modified := 0
			staged := 0

			for _, line := range strings.Split(statusRaw, "\n") {
				switch {
				case strings.HasPrefix(line, "# branch.upstream "):
					hasUpstream = true
					upstreamBranch = strings.TrimPrefix(line, "# branch.upstream ")
				case strings.HasPrefix(line, "# branch.ab "):
					hasUpstream = true
					parts := strings.Fields(line)
					if len(parts) >= 4 {
						aheadStr := strings.TrimPrefix(parts[2], "+")
						behindStr := strings.TrimPrefix(parts[3], "-")
						ahead, _ = strconv.Atoi(aheadStr)
						behind, _ = strconv.Atoi(behindStr)
					}
				case strings.HasPrefix(line, "?"):
					untracked++
				case strings.HasPrefix(line, "1 "), strings.HasPrefix(line, "2 "):
					parts := strings.Fields(line)
					if len(parts) > 1 {
						xy := parts[1]
						if len(xy) >= 2 {
							if xy[0] != '.' {
								staged++
							}
							if xy[1] != '.' {
								modified++
							}
						}
					}
				}
			}

			unpushed := 0
			if !hasUpstream {
				unpushedRaw := s.RunGit(ctx, []string{"git", "rev-list", "-100", "HEAD", "--not", "--remotes"}, path, []int{0}, true, true)
				for _, line := range strings.Split(unpushedRaw, "\n") {
					if strings.TrimSpace(line) != "" {
						unpushed++
					}
				}
			}

			info, exists := branchInfo[branch]
			lastActive := ""
			lastActiveTS := int64(0)
			if exists {
				lastActive = info.lastActive
				lastActiveTS = info.lastActiveTS
			}

			wt := &models.WorktreeInfo{
				Path:           path,
				Branch:         branch,
				IsMain:         wtData.isMain,
				Dirty:          (untracked + modified + staged) > 0,
				Ahead:          ahead,
				Behind:         behind,
				Unpushed:       unpushed,
				HasUpstream:    hasUpstream,
				UpstreamBranch: upstreamBranch,
				LastActive:     lastActive,
				LastActiveTS:   lastActiveTS,
				Untracked:      untracked,
				Modified:       modified,
				Staged:         staged,
			}

			results <- result{wt: wt, err: nil}
		}(wt)
	}

	wg.Wait()
	close(results)

	worktrees := make([]*models.WorktreeInfo, 0, len(wts))
	for r := range results {
		if r.err == nil {
			worktrees = append(worktrees, r.wt)
		}
	}

	return worktrees, nil
}

// GetMainWorktreePath returns the path of the main worktree.
func (s *Service) GetMainWorktreePath(ctx context.Context) string {
	s.mainWorktreePathOnce.Do(func() {
		rawWts := s.RunGit(ctx, []string{"git", "worktree", "list", "--porcelain"}, "", []int{0}, true, false)
		for _, line := range strings.Split(rawWts, "\n") {
			if strings.HasPrefix(line, "worktree ") {
				s.mainWorktreePath = strings.TrimPrefix(line, "worktree ")
				break
			}
		}
		if s.mainWorktreePath == "" {
			s.mainWorktreePath, _ = os.Getwd()
		}
	})
	return s.mainWorktreePath
}

// RenameWorktree moves a worktree and renames its branch only when the
// worktree directory name matches the old branch name.
func (s *Service) RenameWorktree(ctx context.Context, oldPath, newPath, oldBranch, newBranch string) bool {
	if !s.RunCommandChecked(ctx, []string{"git", "worktree", "move", oldPath, newPath}, "", fmt.Sprintf("Failed to move worktree from %s to %s", oldPath, newPath)) {
		return false
	}

	if filepath.Base(oldPath) == oldBranch {
		if !s.RunCommandChecked(ctx, []string{"git", "branch", "-m", oldBranch, newBranch}, newPath, fmt.Sprintf("Failed to rename branch from %s to %s", oldBranch, newBranch)) {
			return false
		}
	}

	return true
}

// configureBranchTracking sets upstream tracking for a local branch.
func (s *Service) configureBranchTracking(ctx context.Context, localBranch, cwd string, ref *prRefInfo) {
	if ref == nil || ref.mergeRef == "" || localBranch == "" {
		return
	}
	remoteName := strings.TrimSpace(ref.remoteName)
	if remoteName == "" {
		remoteName = "origin"
	}
	s.RunGit(ctx, []string{"git", "config", fmt.Sprintf("branch.%s.remote", localBranch), remoteName}, cwd, []int{0}, true, true)
	s.RunGit(ctx, []string{"git", "config", fmt.Sprintf("branch.%s.pushRemote", localBranch), remoteName}, cwd, []int{0}, true, true)
	s.RunGit(ctx, []string{"git", "config", fmt.Sprintf("branch.%s.merge", localBranch), ref.mergeRef}, cwd, []int{0}, true, true)
}

func (s *Service) findWorktreePathForBranch(ctx context.Context, branch string) (string, bool) {
	rawWts := s.RunGit(ctx, []string{"git", "worktree", "list", "--porcelain"}, "", []int{0}, true, true)
	if rawWts == "" {
		return "", false
	}

	currentPath := ""
	for _, line := range strings.Split(rawWts, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			currentPath = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "branch "):
			wtBranch := strings.TrimPrefix(line, "branch ")
			wtBranch = strings.TrimPrefix(wtBranch, "refs/heads/")
			if wtBranch == branch {
				return currentPath, true
			}
		}
	}

	return "", false
}

func (s *Service) syncPRLocalBranch(ctx context.Context, localBranch, targetRef string) bool {
	if localBranch == "" || targetRef == "" {
		s.notify("PR branch information is missing", "error")
		return false
	}

	if path, attached := s.findWorktreePathForBranch(ctx, localBranch); attached {
		s.notify(fmt.Sprintf("Branch %q is already checked out in worktree %q", localBranch, path), "error")
		return false
	}

	if s.localBranchExists(ctx, localBranch) {
		s.notify(fmt.Sprintf("Warning: local branch %q already exists and will be reset to PR head", localBranch), "warning")
		return s.RunCommandChecked(
			ctx,
			[]string{"git", "branch", "-f", localBranch, targetRef},
			"",
			fmt.Sprintf("Failed to reset branch %s", localBranch),
		)
	}

	return s.RunCommandChecked(
		ctx,
		[]string{"git", "branch", localBranch, targetRef},
		"",
		fmt.Sprintf("Failed to create branch %s", localBranch),
	)
}

// CherryPickCommit applies a commit to a target worktree.
// Returns true on success, false on failure (including conflicts).
func (s *Service) CherryPickCommit(ctx context.Context, commitSHA, targetPath string) (bool, error) {
	statusRaw := s.RunGit(ctx, []string{"git", "status", "--porcelain"}, targetPath, []int{0}, true, false)
	if strings.TrimSpace(statusRaw) != "" {
		return false, fmt.Errorf("target worktree has uncommitted changes")
	}

	cmd, err := s.prepareAllowedCommand(ctx, []string{"git", "cherry-pick", commitSHA})
	if err != nil {
		return false, err
	}
	cmd.Dir = targetPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))

		s.RunCommandChecked(ctx, []string{"git", "cherry-pick", "--abort"}, targetPath, "Failed to abort cherry-pick")

		if strings.Contains(detail, "conflict") || strings.Contains(detail, "CONFLICT") {
			return false, fmt.Errorf("cherry-pick conflicts occurred: %s", detail)
		}
		return false, fmt.Errorf("cherry-pick failed: %s", detail)
	}

	return true, nil
}
