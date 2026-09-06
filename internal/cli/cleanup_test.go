package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/git"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanupWorktreeDetectsIgnoredSubmoduleChanges(t *testing.T) {
	t.Parallel()

	for _, source := range []string{".gitmodules", "local"} {
		for _, ignore := range []string{"dirty", "all"} {
			for _, change := range []string{"tracked", "untracked"} {
				t.Run(source+"/"+ignore+"/"+change, func(t *testing.T) {
					t.Parallel()
					ctx := context.Background()
					svc := git.NewService(nil, nil)
					runGit := func(dir string, args ...string) string {
						t.Helper()
						args = append([]string{"git", "-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false"}, args...)
						output, err := svc.RunGitWithCombinedOutput(ctx, args, dir, nil)
						require.NoError(t, err, "%s", output)
						return strings.TrimSpace(string(output))
					}

					subRepo := t.TempDir()
					runGit(subRepo, "init")
					require.NoError(t, os.WriteFile(filepath.Join(subRepo, "tracked.txt"), []byte("original\n"), 0o600))
					runGit(subRepo, "add", ".")
					runGit(subRepo, "commit", "-m", "Initial submodule")

					repo := t.TempDir()
					runGit(repo, "init")
					runGit(repo, "-c", "protocol.file.allow=always", "submodule", "add", subRepo, "sub")
					if source == ".gitmodules" {
						runGit(repo, "config", "-f", ".gitmodules", "submodule.sub.ignore", ignore)
					} else {
						runGit(repo, "config", "--local", "submodule.sub.ignore", ignore)
					}
					runGit(repo, "add", ".")
					runGit(repo, "commit", "-m", "Add submodule")

					dirty, err := cleanupWorktreeHasUncommittedChanges(ctx, svc, repo)
					require.NoError(t, err)
					require.False(t, dirty, "clean submodule should allow cleanup")

					require.NoError(t, os.WriteFile(filepath.Join(repo, "sub", change+".txt"), []byte("unsaved work\n"), 0o600))
					require.Empty(t, runGit(repo, "status", "--porcelain", "--untracked-files=all"), "ordinary status should hide the submodule change")
					dirty, err = cleanupWorktreeHasUncommittedChanges(ctx, svc, repo)
					require.NoError(t, err)
					assert.True(t, dirty, "cleanup must detect hidden submodule changes")
				})
			}
		}
	}
}

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
		runGitCombinedOutputs: map[string][]byte{
			featurePath: []byte("1 .M file.txt\n"),
		},
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
	summary, err := Cleanup(context.Background(), svc, cfg, false, false, strings.NewReader("1,3\n"), &stderr)
	require.NoError(t, err)

	assert.Contains(t, stderr.String(), "[1] worktree feature")
	assert.Contains(t, stderr.String(), "HAS UNCOMMITTED CHANGES (will be skipped)")
	assert.Contains(t, stderr.String(), "[2] branch stale")
	assert.Contains(t, stderr.String(), "[3] orphaned directory")
	assert.Contains(t, stderr.String(), "1 merged worktree skipped")
	assert.Contains(t, stderr.String(), "uncommitted changes")
	assert.Contains(t, stderr.String(), "1 orphaned directory removed")
	assert.NoDirExists(t, orphanPath)
	assert.False(t, commandWasRun(svc.runCommandCheckedCalls, "git", "worktree", "remove", "--force", featurePath))
	assert.False(t, commandWasRun(svc.runCommandCheckedCalls, "git", "branch", "-D", "feature"))
	assert.False(t, commandWasRun(svc.runCommandCheckedCalls, "git", "branch", "-D", "stale"))
	assert.Equal(t, 1, summary.Skipped)
	assert.Equal(t, 1, summary.Orphans)
	require.Len(t, summary.Items, 2)
	skippedItem := findCleanupItem(summary.Items, CleanupKindWorktree)
	require.NotNil(t, skippedItem)
	assert.True(t, skippedItem.Skipped)
	assert.Equal(t, "uncommitted changes", skippedItem.SkipReason)
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
	summary, err := Cleanup(context.Background(), svc, cfg, true, false, strings.NewReader(""), &stderr)
	require.NoError(t, err)

	assert.NotContains(t, stderr.String(), "Select items")
	assert.Contains(t, stderr.String(), "1 merged worktree removed")
	assert.Contains(t, stderr.String(), "1 stale branch deleted")
	assert.Contains(t, stderr.String(), "1 orphaned directory removed")
	assert.True(t, commandWasRun(svc.runCommandCheckedCalls, "git", "branch", "-D", "stale"))
	assert.NoDirExists(t, orphanPath)

	assert.Equal(t, 1, summary.Worktrees)
	assert.Equal(t, 1, summary.Branches)
	assert.Equal(t, 1, summary.Orphans)
	assert.Equal(t, 0, summary.Failures)
	require.Len(t, summary.Items, 3)
	worktreeItem := findCleanupItem(summary.Items, CleanupKindWorktree)
	require.NotNil(t, worktreeItem)
	assert.Equal(t, featurePath, worktreeItem.Path)
	assert.Equal(t, "feature", worktreeItem.Branch)
	assert.True(t, worktreeItem.BranchDeleted)
	assert.False(t, worktreeItem.Failed)
	branchItem := findCleanupItem(summary.Items, CleanupKindBranch)
	require.NotNil(t, branchItem)
	assert.Equal(t, "stale", branchItem.Branch)
}

func TestCleanupAllSkipsDirtyWorktreeAndContinues(t *testing.T) {
	t.Parallel()

	worktreeDir := t.TempDir()
	repoDir := filepath.Join(worktreeDir, "repo")
	cleanPath := filepath.Join(repoDir, "clean")
	featurePath := filepath.Join(repoDir, "feature")
	orphanPath := filepath.Join(repoDir, "orphan")
	require.NoError(t, os.MkdirAll(cleanPath, 0o750))
	require.NoError(t, os.MkdirAll(featurePath, 0o750))
	require.NoError(t, os.MkdirAll(orphanPath, 0o750))

	svc := &fakeGitService{
		resolveRepoName:     "repo",
		mainWorktreePath:    "/main",
		mainBranch:          "main",
		mergedBranches:      []string{"clean", "feature", "stale"},
		runCommandCheckedOK: true,
		runGitCombinedOutputs: map[string][]byte{
			featurePath: []byte("?? untracked.txt\n"),
		},
		worktrees: []*models.WorktreeInfo{
			{Path: "/main", Branch: "main", IsMain: true},
			{Path: cleanPath, Branch: "clean"},
			{Path: featurePath, Branch: "feature"},
		},
	}
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = worktreeDir
	cfg.DisablePR = true
	cfg.PruneStaleBranches = true

	var stderr bytes.Buffer
	summary, err := Cleanup(context.Background(), svc, cfg, true, false, strings.NewReader(""), &stderr)
	require.NoError(t, err)

	assert.Contains(t, stderr.String(), "1 merged worktree removed")
	assert.Contains(t, stderr.String(), "1 merged worktree skipped")
	assert.Contains(t, stderr.String(), "1 stale branch deleted")
	assert.Contains(t, stderr.String(), "1 orphaned directory removed")
	assert.Contains(t, stderr.String(), "  • clean\n")
	assert.Contains(t, stderr.String(), "  • feature (uncommitted changes)")
	assert.True(t, commandWasRun(svc.runCommandCheckedCalls, "git", "worktree", "remove", "--force", cleanPath))
	assert.False(t, commandWasRun(svc.runCommandCheckedCalls, "git", "worktree", "remove", "--force", featurePath))
	assert.False(t, commandWasRun(svc.runCommandCheckedCalls, "git", "branch", "-D", "feature"))
	assert.True(t, commandWasRun(svc.runCommandCheckedCalls, "git", "branch", "-D", "stale"))
	assert.NoDirExists(t, orphanPath)
	assert.Equal(t, 1, summary.Worktrees)
	assert.Equal(t, 1, summary.Branches)
	assert.Equal(t, 1, summary.Orphans)
	assert.Equal(t, 1, summary.Skipped)
	assert.Equal(t, 0, summary.Failures)
}

func TestCleanupRechecksWorktreeStatusBeforeRemoval(t *testing.T) {
	t.Parallel()

	worktreeDir := t.TempDir()
	repoDir := filepath.Join(worktreeDir, "repo")
	featurePath := filepath.Join(repoDir, "feature")
	require.NoError(t, os.MkdirAll(featurePath, 0o750))

	svc := &fakeGitService{
		resolveRepoName:     "repo",
		mainWorktreePath:    "/main",
		mainBranch:          "main",
		mergedBranches:      []string{"feature"},
		runCommandCheckedOK: true,
		runGitCombinedOutputs: map[string][]byte{
			featurePath: []byte("1 .M changed-after-discovery.txt\n"),
		},
		worktrees: []*models.WorktreeInfo{
			{Path: "/main", Branch: "main", IsMain: true},
			{Path: featurePath, Branch: "feature"},
		},
	}
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = worktreeDir
	cfg.DisablePR = true

	var stderr bytes.Buffer
	summary, err := Cleanup(context.Background(), svc, cfg, true, false, strings.NewReader(""), &stderr)
	require.NoError(t, err)

	assert.Contains(t, stderr.String(), "1 merged worktree skipped")
	assert.False(t, commandWasRun(svc.runCommandCheckedCalls, "git", "worktree", "remove", "--force", featurePath))
	assert.Equal(t, 1, summary.Skipped)
	assert.Equal(t, 0, summary.Failures)
}

func TestCleanupSkipsWorktreeWhenStatusCannotBeVerified(t *testing.T) {
	t.Parallel()

	worktreeDir := t.TempDir()
	repoDir := filepath.Join(worktreeDir, "repo")
	featurePath := filepath.Join(repoDir, "feature")
	require.NoError(t, os.MkdirAll(featurePath, 0o750))

	statusErr := errors.New("status unavailable")
	svc := &fakeGitService{
		resolveRepoName:     "repo",
		mainWorktreePath:    "/main",
		mainBranch:          "main",
		mergedBranches:      []string{"feature"},
		runCommandCheckedOK: true,
		runGitCombinedErrors: map[string]error{
			featurePath: statusErr,
		},
		worktrees: []*models.WorktreeInfo{
			{Path: "/main", Branch: "main", IsMain: true},
			{Path: featurePath, Branch: "feature"},
		},
	}
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = worktreeDir
	cfg.DisablePR = true

	var stderr bytes.Buffer
	summary, err := Cleanup(context.Background(), svc, cfg, true, false, strings.NewReader(""), &stderr)
	require.NoError(t, err)

	assert.Contains(t, stderr.String(), "status could not be verified")
	assert.False(t, commandWasRun(svc.runCommandCheckedCalls, "git", "worktree", "remove", "--force", featurePath))
	assert.Equal(t, 1, summary.Skipped)
	assert.Equal(t, "could not verify worktree status", summary.Items[0].SkipReason)
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
	_, err := Cleanup(context.Background(), svc, cfg, false, false, strings.NewReader("\n"), &stderr)
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
	summary, err := Cleanup(context.Background(), svc, cfg, true, false, strings.NewReader(""), &stderr)
	require.ErrorContains(t, err, "cleanup completed with 1 failure")
	assert.Contains(t, stderr.String(), "1 failure")

	require.Len(t, summary.Items, 1)
	assert.True(t, summary.Items[0].Failed)
	assert.NotEmpty(t, summary.Items[0].Error)
	assert.Equal(t, 1, summary.Failures)
}

func TestCleanupJSONRequiresAll(t *testing.T) {
	t.Parallel()

	svc := &fakeGitService{
		resolveRepoName:  "repo",
		mainWorktreePath: "/main",
		mainBranch:       "main",
	}
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = t.TempDir()
	cfg.DisablePR = true

	var stderr bytes.Buffer
	_, err := Cleanup(context.Background(), svc, cfg, false, true, strings.NewReader(""), &stderr)
	require.ErrorContains(t, err, "--json requires --all")
	assert.Empty(t, svc.runCommandCheckedCalls)
}

func TestCleanupJSONSuppressesSummaryAndTerminateNoise(t *testing.T) {
	t.Parallel()

	worktreeDir := t.TempDir()
	repoDir := filepath.Join(worktreeDir, "repo")
	featurePath := filepath.Join(repoDir, "feature")
	require.NoError(t, os.MkdirAll(featurePath, 0o750))

	svc := &fakeGitService{
		resolveRepoName:     "repo",
		mainWorktreePath:    "/main",
		mainBranch:          "main",
		mergedBranches:      []string{"feature"},
		runCommandCheckedOK: true,
		worktrees: []*models.WorktreeInfo{
			{Path: "/main", Branch: "main", IsMain: true},
			{Path: featurePath, Branch: "feature"},
		},
	}
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = worktreeDir
	cfg.DisablePR = true
	cfg.TerminateCommands = []string{"true"}

	var stderr bytes.Buffer
	summary, err := Cleanup(context.Background(), svc, cfg, true, true, strings.NewReader(""), &stderr)
	require.NoError(t, err)

	assert.NotContains(t, stderr.String(), "Cleanup complete")
	assert.NotContains(t, stderr.String(), "Running terminate commands")

	require.Len(t, summary.Items, 1)
	assert.Equal(t, CleanupKindWorktree, summary.Items[0].Kind)
	assert.Equal(t, "feature", summary.Items[0].Branch)
	assert.Equal(t, featurePath, summary.Items[0].Path)
	assert.True(t, summary.Items[0].BranchDeleted)
}

func findCleanupItem(items []CleanupItem, kind string) *CleanupItem {
	for i := range items {
		if items[i].Kind == kind {
			return &items[i]
		}
	}
	return nil
}

func commandWasRun(calls [][]string, want ...string) bool {
	for _, call := range calls {
		if assert.ObjectsAreEqual(want, call) {
			return true
		}
	}
	return false
}
