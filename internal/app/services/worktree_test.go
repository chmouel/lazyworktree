package services

import (
	"context"
	"testing"

	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockGitService struct {
	mainBranch     string
	mergedBranches []string
}

func (m *mockGitService) RunGit(_ context.Context, _ []string, _ string, _ []int, _, _ bool) string {
	return ""
}

func (m *mockGitService) RunCommandChecked(_ context.Context, _ []string, _, _ string) bool {
	return true
}

func (m *mockGitService) GetMainBranch(_ context.Context) string {
	return m.mainBranch
}

func (m *mockGitService) GetMergedBranches(_ context.Context, _ string) []string {
	return m.mergedBranches
}

func (m *mockGitService) RenameWorktree(_ context.Context, _, _, _, _ string) bool {
	return true
}

func (m *mockGitService) ExecuteCommands(_ context.Context, _ []string, _ string, _ map[string]string) error {
	return nil
}

func (m *mockGitService) RunGitWithCombinedOutput(_ context.Context, _ []string, _ string, _ map[string]string) ([]byte, error) {
	return nil, nil
}

func TestGetPruneCandidatesWithStaleBranches(t *testing.T) {
	git := &mockGitService{
		mainBranch:     "main",
		mergedBranches: []string{"feature-a", "feature-b", "stale-branch"},
	}
	svc := NewWorktreeService(git)

	worktrees := []*models.WorktreeInfo{
		{Path: "/repo/main", Branch: "main", IsMain: true},
		{Path: "/repo/feature-a", Branch: "feature-a"},
	}

	// Without stale branches
	candidates, err := svc.GetPruneCandidates(context.Background(), worktrees, false)
	require.NoError(t, err)

	// Only feature-a should be a candidate (has worktree and is merged)
	assert.Len(t, candidates, 1)
	assert.Equal(t, "feature-a", candidates[0].Branch)
	assert.NotNil(t, candidates[0].Worktree)

	// With stale branches
	candidates, err = svc.GetPruneCandidates(context.Background(), worktrees, true)
	require.NoError(t, err)

	// feature-a (with worktree) + feature-b and stale-branch (branch-only)
	assert.Len(t, candidates, 3)

	branchMap := make(map[string]PruneCandidate)
	for _, c := range candidates {
		branchMap[c.Branch] = c
	}

	assert.NotNil(t, branchMap["feature-a"].Worktree)
	assert.Nil(t, branchMap["feature-b"].Worktree)
	assert.Equal(t, "git", branchMap["feature-b"].Source)
	assert.Nil(t, branchMap["stale-branch"].Worktree)
	assert.Equal(t, "git", branchMap["stale-branch"].Source)
}

func TestGetPruneCandidatesPRMerged(t *testing.T) {
	git := &mockGitService{
		mainBranch:     "main",
		mergedBranches: []string{"feature-a"},
	}
	svc := NewWorktreeService(git)

	worktrees := []*models.WorktreeInfo{
		{Path: "/repo/main", Branch: "main", IsMain: true},
		{Path: "/repo/feature-a", Branch: "feature-a", PR: &models.PRInfo{State: "MERGED"}},
	}

	candidates, err := svc.GetPruneCandidates(context.Background(), worktrees, false)
	require.NoError(t, err)

	assert.Len(t, candidates, 1)
	assert.Equal(t, "both", candidates[0].Source)
	assert.Equal(t, "feature-a", candidates[0].Branch)
}

func TestGetPruneCandidatesExcludesCheckedOutMainWorktreeBranch(t *testing.T) {
	git := &mockGitService{
		mainBranch:     "main",
		mergedBranches: []string{"feature-root", "stale-branch"},
	}
	svc := NewWorktreeService(git)

	worktrees := []*models.WorktreeInfo{
		{Path: "/repo", Branch: "feature-root", IsMain: true},
	}

	candidates, err := svc.GetPruneCandidates(context.Background(), worktrees, true)
	require.NoError(t, err)

	assert.Len(t, candidates, 1)
	assert.Equal(t, "stale-branch", candidates[0].Branch)
	assert.Nil(t, candidates[0].Worktree)
}
