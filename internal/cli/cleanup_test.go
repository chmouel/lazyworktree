package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCleanupSelection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		selection string
		count     int
		want      []int
		wantErr   string
	}{
		{name: "single", selection: "2", count: 3, want: []int{2}},
		{name: "list and range", selection: "1,3-5", count: 5, want: []int{1, 3, 4, 5}},
		{name: "deduplicates and sorts", selection: "3,1,2-3", count: 3, want: []int{1, 2, 3}},
		{name: "all", selection: "all", count: 3, want: []int{1, 2, 3}},
		{name: "asterisk", selection: "*", count: 2, want: []int{1, 2}},
		{name: "out of range", selection: "4", count: 3, wantErr: "out of range"},
		{name: "backwards range", selection: "3-1", count: 3, wantErr: "invalid cleanup range"},
		{name: "invalid value", selection: "one", count: 3, wantErr: "invalid cleanup selection"},
		{name: "empty list item", selection: "1,,2", count: 3, wantErr: "invalid cleanup selection"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseCleanupSelection(tt.selection, tt.count)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCleanupInteractiveSelection(t *testing.T) {
	t.Parallel()

	worktreeDir := t.TempDir()
	repoDir := filepath.Join(worktreeDir, "repo")
	featurePath := filepath.Join(repoDir, "feature")
	orphanPath := filepath.Join(repoDir, "orphan")
	require.NoError(t, os.MkdirAll(featurePath, 0o750))
	require.NoError(t, os.MkdirAll(orphanPath, 0o750))

	svc := &fakeGitService{
		resolveRepoName:     "repo",
		mainWorktreePath:    "/main",
		mainBranch:          "main",
		mergedBranches:      []string{"feature", "stale"},
		runCommandCheckedOK: true,
		worktrees: []*models.WorktreeInfo{
			{Path: "/main", Branch: "main", IsMain: true},
			{Path: featurePath, Branch: "feature", Dirty: true},
		},
	}
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = worktreeDir
	cfg.DisablePR = true
	cfg.PruneStaleBranches = true

	var stderr bytes.Buffer
	err := Cleanup(context.Background(), svc, cfg, false, strings.NewReader("1,3\n"), &stderr)
	require.NoError(t, err)

	assert.Contains(t, stderr.String(), "[1] worktree feature")
	assert.Contains(t, stderr.String(), "HAS UNCOMMITTED CHANGES")
	assert.Contains(t, stderr.String(), "[2] branch stale")
	assert.Contains(t, stderr.String(), "[3] orphaned directory")
	assert.Contains(t, stderr.String(), "1 merged worktree removed")
	assert.Contains(t, stderr.String(), "1 orphaned directory removed")
	assert.NoDirExists(t, orphanPath)
	assert.True(t, commandWasRun(svc.runCommandCheckedCalls, "git", "worktree", "remove", "--force", featurePath))
	assert.False(t, commandWasRun(svc.runCommandCheckedCalls, "git", "branch", "-D", "stale"))
}

func TestCleanupAllIncludesEveryCandidate(t *testing.T) {
	t.Parallel()

	worktreeDir := t.TempDir()
	repoDir := filepath.Join(worktreeDir, "repo")
	featurePath := filepath.Join(repoDir, "feature")
	orphanPath := filepath.Join(repoDir, "orphan")
	require.NoError(t, os.MkdirAll(featurePath, 0o750))
	require.NoError(t, os.MkdirAll(orphanPath, 0o750))

	svc := &fakeGitService{
		resolveRepoName:     "repo",
		mainWorktreePath:    "/main",
		mainBranch:          "main",
		mergedBranches:      []string{"feature", "stale"},
		runCommandCheckedOK: true,
		worktrees: []*models.WorktreeInfo{
			{Path: "/main", Branch: "main", IsMain: true},
			{Path: featurePath, Branch: "feature"},
		},
	}
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = worktreeDir
	cfg.DisablePR = true
	cfg.PruneStaleBranches = true

	var stderr bytes.Buffer
	err := Cleanup(context.Background(), svc, cfg, true, strings.NewReader(""), &stderr)
	require.NoError(t, err)

	assert.NotContains(t, stderr.String(), "Select items")
	assert.Contains(t, stderr.String(), "1 merged worktree removed")
	assert.Contains(t, stderr.String(), "1 stale branch deleted")
	assert.Contains(t, stderr.String(), "1 orphaned directory removed")
	assert.True(t, commandWasRun(svc.runCommandCheckedCalls, "git", "branch", "-D", "stale"))
	assert.NoDirExists(t, orphanPath)
}

func TestCleanupCancelled(t *testing.T) {
	t.Parallel()

	svc := &fakeGitService{
		resolveRepoName:     "repo",
		mainWorktreePath:    "/main",
		mainBranch:          "main",
		mergedBranches:      []string{"feature"},
		runCommandCheckedOK: true,
		worktrees: []*models.WorktreeInfo{
			{Path: "/main", Branch: "main", IsMain: true},
			{Path: "/worktrees/repo/feature", Branch: "feature"},
		},
	}
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = t.TempDir()
	cfg.DisablePR = true

	var stderr bytes.Buffer
	err := Cleanup(context.Background(), svc, cfg, false, strings.NewReader("\n"), &stderr)
	require.NoError(t, err)
	assert.Contains(t, stderr.String(), "Cleanup cancelled.")
	assert.Empty(t, svc.runCommandCheckedCalls)
}

func TestFindCleanupCandidatesRefreshesMergedPRState(t *testing.T) {
	t.Parallel()

	svc := &fakeGitService{
		resolveRepoName:  "repo",
		mainWorktreePath: "/main",
		mainBranch:       "main",
		prForWorktree:    &models.PRInfo{State: "MERGED"},
		worktrees: []*models.WorktreeInfo{
			{Path: "/main", Branch: "main", IsMain: true},
			{Path: "/worktrees/repo/feature", Branch: "feature"},
		},
	}
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = t.TempDir()

	candidates, _, err := findCleanupCandidates(context.Background(), svc, cfg, &bytes.Buffer{})
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	assert.Equal(t, cleanupWorktree, candidates[0].kind)
	assert.Equal(t, "pr", candidates[0].source)
	assert.Equal(t, 1, svc.prForWorktreeCalls)
}

func TestCleanupReportsFailures(t *testing.T) {
	t.Parallel()

	svc := &fakeGitService{
		resolveRepoName:     "repo",
		mainWorktreePath:    "/main",
		mainBranch:          "main",
		mergedBranches:      []string{"feature"},
		runCommandCheckedOK: false,
		worktrees: []*models.WorktreeInfo{
			{Path: "/main", Branch: "main", IsMain: true},
			{Path: "/worktrees/repo/feature", Branch: "feature"},
		},
	}
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = t.TempDir()
	cfg.DisablePR = true

	var stderr bytes.Buffer
	err := Cleanup(context.Background(), svc, cfg, true, strings.NewReader(""), &stderr)
	require.ErrorContains(t, err, "cleanup completed with 1 failure")
	assert.Contains(t, stderr.String(), "1 failure")
}

func commandWasRun(calls [][]string, want ...string) bool {
	for _, call := range calls {
		if assert.ObjectsAreEqual(want, call) {
			return true
		}
	}
	return false
}
