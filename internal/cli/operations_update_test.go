package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chmouel/lazyworktree/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runGitForUpdateTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	full := append([]string{"-c", "user.name=test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false"}, args...)
	cmd := exec.Command("git", full...) //nolint:gosec // fixed git binary with test-controlled arguments
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	return strings.TrimSpace(string(out))
}

func commitForUpdateTest(t *testing.T, dir, file, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, file), []byte(content), 0o600))
	runGitForUpdateTest(t, dir, "add", file)
	runGitForUpdateTest(t, dir, "commit", "-m", "change "+file)
}

func TestUpdateOnExistingWorktreeRealRepo(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	repo := filepath.Join(root, "repo")
	other := filepath.Join(root, "other")
	worktree := filepath.Join(root, "feature-wt")

	runGitForUpdateTest(t, root, "init", "--bare", "-b", "main", remote)
	require.NoError(t, os.MkdirAll(repo, 0o750))
	runGitForUpdateTest(t, repo, "init", "-b", "main")
	runGitForUpdateTest(t, repo, "remote", "add", "origin", remote)
	commitForUpdateTest(t, repo, "base.txt", "base\n")
	runGitForUpdateTest(t, repo, "push", "origin", "main")
	runGitForUpdateTest(t, repo, "checkout", "-b", "feature")
	commitForUpdateTest(t, repo, "feature.txt", "v1\n")
	runGitForUpdateTest(t, repo, "push", "origin", "feature")
	runGitForUpdateTest(t, repo, "checkout", "main")
	runGitForUpdateTest(t, repo, "worktree", "add", worktree, "feature")

	runGitForUpdateTest(t, root, "clone", "--branch", "feature", remote, other)
	t.Chdir(repo)
	svc := git.NewService(nil, nil)

	t.Run("behind upstream fast-forwards to remote head", func(t *testing.T) {
		commitForUpdateTest(t, other, "feature.txt", "v2\n")
		runGitForUpdateTest(t, other, "push", "origin", "feature")
		remoteHead := strings.Fields(runGitForUpdateTest(t, repo, "ls-remote", "origin", "refs/heads/feature"))[0]

		outPath, err := updateExistingWorktreeToRef(ctx, svc, worktree, "origin/feature", true)
		require.NoError(t, err)
		assert.Equal(t, worktree, outPath)
		assert.Equal(t, remoteHead, runGitForUpdateTest(t, worktree, "rev-parse", "HEAD"))
	})

	t.Run("diverged worktree keeps local commits", func(t *testing.T) {
		commitForUpdateTest(t, worktree, "local.txt", "mine\n")
		localHead := runGitForUpdateTest(t, worktree, "rev-parse", "HEAD")
		commitForUpdateTest(t, other, "feature.txt", "v3\n")
		runGitForUpdateTest(t, other, "push", "origin", "feature")

		outPath, err := updateExistingWorktreeToRef(ctx, svc, worktree, "origin/feature", true)
		require.NoError(t, err)
		assert.Equal(t, worktree, outPath)
		assert.Equal(t, localHead, runGitForUpdateTest(t, worktree, "rev-parse", "HEAD"))
		assert.FileExists(t, filepath.Join(worktree, "local.txt"))
	})
}
