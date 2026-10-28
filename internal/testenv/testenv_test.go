package testenv

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	ClearGitRepoEnv()
	m.Run()
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	full := append([]string{
		"-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false",
	}, args...)
	cmd := exec.Command("git", full...) //nolint:gosec // fixed git binary with test-controlled arguments
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
	return string(out)
}

// Git hooks in a linked worktree export an absolute GIT_DIR. Scratch
// repositories created after ClearGitRepoEnv must not touch that repository.
func TestClearGitRepoEnvProtectsLinkedWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}

	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	runGit(t, root, "init", "-q", repo)
	runGit(t, repo, "commit", "-q", "--allow-empty", "-m", "base")
	runGit(t, repo, "worktree", "add", "-q", "-b", "linked", filepath.Join(root, "linked"))

	snapshot := func() string {
		cfg, err := os.ReadFile(filepath.Join(repo, ".git", "config")) //nolint:gosec // Test-owned temporary path.
		require.NoError(t, err)
		return runGit(t, repo, "for-each-ref") + runGit(t, repo, "worktree", "list", "--porcelain") + string(cfg)
	}
	before := snapshot()

	t.Setenv("GIT_DIR", filepath.Join(repo, ".git", "worktrees", "linked"))
	ClearGitRepoEnv()

	scratch := filepath.Join(root, "scratch")
	runGit(t, root, "init", "-q", scratch)
	runGit(t, scratch, "config", "user.name", "Scratch")
	runGit(t, scratch, "commit", "-q", "--allow-empty", "-m", "scratch")
	runGit(t, scratch, "branch", "feature")
	runGit(t, scratch, "init", "-q", "--bare")

	require.Equal(t, before, snapshot())
}
