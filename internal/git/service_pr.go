package git

import (
	"context"
	"encoding/json"
	"fmt"
)

// prRefInfo holds the result of fetching PR/MR ref information from GitHub or GitLab.
type prRefInfo struct {
	headCommit string
	repoURL    string
	remoteName string
	mergeRef   string
}

// fetchPRRefInfo fetches the head commit, repo URL, and merge ref for a PR/MR.
// Returns nil and false if the fetch fails.
func (s *Service) fetchPRRefInfo(ctx context.Context, prNumber int, remoteBranch string) (*prRefInfo, bool) {
	host := s.DetectHost(ctx)
	switch host {
	case gitHostGithub:
		prRaw := s.RunGit(ctx, []string{
			"gh", "pr", "view", fmt.Sprintf("%d", prNumber),
			"--json", "headRefOid,headRepository",
		}, "", []int{0}, true, true)
		if prRaw == "" {
			s.notify(fmt.Sprintf("Failed to get PR #%d info", prNumber), "error")
			return nil, false
		}
		var pr map[string]any
		if err := json.Unmarshal([]byte(prRaw), &pr); err != nil {
			s.notify(fmt.Sprintf("Failed to parse PR #%d data: %v", prNumber, err), "error")
			return nil, false
		}
		headCommit, _ := pr["headRefOid"].(string)
		if headCommit == "" {
			s.notify(fmt.Sprintf("Failed to get PR #%d head commit", prNumber), "error")
			return nil, false
		}
		var repoURL string
		if headRepo, ok := pr["headRepository"].(map[string]any); ok {
			repoURL, _ = headRepo["url"].(string)
		}
		if repoURL == "" {
			repoURL = s.getRemoteURL(ctx)
		}
		if !s.RunCommandChecked(ctx, []string{"git", "fetch", "origin", fmt.Sprintf("refs/pull/%d/head", prNumber)}, "", fmt.Sprintf("Failed to fetch PR #%d", prNumber)) {
			return nil, false
		}
		return &prRefInfo{
			headCommit: headCommit,
			repoURL:    repoURL,
			remoteName: "origin",
			mergeRef:   fmt.Sprintf("refs/pull/%d/head", prNumber),
		}, true

	case gitHostGitLab:
		mrRaw := s.RunGit(ctx, []string{
			"glab", "api", fmt.Sprintf("merge_requests/%d", prNumber),
		}, "", []int{0}, true, true)
		if mrRaw == "" {
			s.notify(fmt.Sprintf("Failed to get MR #%d info", prNumber), "error")
			return nil, false
		}
		var mr map[string]any
		if err := json.Unmarshal([]byte(mrRaw), &mr); err != nil {
			s.notify(fmt.Sprintf("Failed to parse MR #%d data: %v", prNumber, err), "error")
			return nil, false
		}
		headCommit, _ := mr["sha"].(string)
		if headCommit == "" {
			if diffRefs, ok := mr["diff_refs"].(map[string]any); ok {
				headCommit, _ = diffRefs["head_sha"].(string)
			}
		}
		if headCommit == "" {
			s.notify(fmt.Sprintf("Failed to get MR #%d head commit", prNumber), "error")
			return nil, false
		}
		sourceBranch, _ := mr["source_branch"].(string)
		if sourceBranch == "" {
			sourceBranch = remoteBranch
		}
		if sourceBranch == "" {
			s.notify(fmt.Sprintf("Failed to get MR #%d source branch", prNumber), "error")
			return nil, false
		}
		repoURL := s.getRemoteURL(ctx)
		if !s.RunCommandChecked(ctx, []string{"git", "fetch", "origin", sourceBranch}, "", fmt.Sprintf("Failed to fetch MR #%d", prNumber)) {
			return nil, false
		}
		return &prRefInfo{
			headCommit: headCommit,
			repoURL:    repoURL,
			remoteName: "origin",
			mergeRef:   "refs/heads/" + sourceBranch,
		}, true
	}
	return nil, false
}

// CreateWorktreeFromPR creates a worktree from a PR's remote branch.
// It fetches the PR head commit, creates a worktree at that commit with a proper branch,
// and sets up branch tracking configuration (replicating what gh/glab pr checkout does).
func (s *Service) CreateWorktreeFromPR(ctx context.Context, prNumber int, remoteBranch, localBranch, targetPath string) bool {
	host := s.DetectHost(ctx)

	if host != gitHostGithub && host != gitHostGitLab {
		if !s.RunCommandChecked(ctx, []string{"git", "fetch", "origin", remoteBranch}, "", fmt.Sprintf("Failed to fetch remote branch %s", remoteBranch)) {
			return false
		}
		remoteRef := fmt.Sprintf("origin/%s", remoteBranch)
		if !s.syncPRLocalBranch(ctx, localBranch, remoteRef) {
			return false
		}
		if !s.RunCommandChecked(ctx, []string{"git", "worktree", "add", targetPath, localBranch}, "", fmt.Sprintf("Failed to create worktree from PR branch %s", remoteBranch)) {
			return false
		}
		s.configureBranchTracking(ctx, localBranch, targetPath, &prRefInfo{
			remoteName: "origin",
			mergeRef:   "refs/heads/" + remoteBranch,
		})
		return true
	}

	ref, ok := s.fetchPRRefInfo(ctx, prNumber, remoteBranch)
	if !ok {
		return false
	}
	if !s.syncPRLocalBranch(ctx, localBranch, ref.headCommit) {
		return false
	}
	if !s.RunCommandChecked(ctx, []string{"git", "worktree", "add", targetPath, localBranch}, "", fmt.Sprintf("Failed to create worktree at %s", targetPath)) {
		return false
	}
	s.configureBranchTracking(ctx, localBranch, targetPath, ref)
	return true
}

// CheckoutPRBranch checks out a PR branch locally without creating a worktree.
func (s *Service) CheckoutPRBranch(ctx context.Context, prNumber int, remoteBranch, localBranch string) bool {
	host := s.DetectHost(ctx)

	if host != gitHostGithub && host != gitHostGitLab {
		if !s.RunCommandChecked(ctx, []string{"git", "fetch", "origin", remoteBranch}, "", fmt.Sprintf("Failed to fetch remote branch %s", remoteBranch)) {
			return false
		}
		remoteRef := fmt.Sprintf("origin/%s", remoteBranch)
		if !s.syncPRLocalBranch(ctx, localBranch, remoteRef) {
			return false
		}
		s.configureBranchTracking(ctx, localBranch, "", &prRefInfo{
			remoteName: "origin",
			mergeRef:   "refs/heads/" + remoteBranch,
		})
		return s.RunCommandChecked(ctx, []string{"git", "switch", localBranch}, "", fmt.Sprintf("Failed to switch to branch %s", localBranch))
	}

	ref, ok := s.fetchPRRefInfo(ctx, prNumber, remoteBranch)
	if !ok {
		return false
	}
	if !s.syncPRLocalBranch(ctx, localBranch, ref.headCommit) {
		return false
	}
	s.configureBranchTracking(ctx, localBranch, "", ref)
	return s.RunCommandChecked(ctx, []string{"git", "switch", localBranch}, "", fmt.Sprintf("Failed to switch to branch %s", localBranch))
}
