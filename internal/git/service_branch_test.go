package git

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetMainBranch(t *testing.T) {
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}

	service := NewService(notify, notifyOnce)
	ctx := context.Background()

	branch := service.GetMainBranch(ctx)
	assert.NotEmpty(t, branch)
	assert.Contains(t, []string{"main", "master"}, branch)
}

func TestGetRemoteURLCachesFirstResult(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("requires sh")
	}

	service := NewService(func(string, string) {}, func(string, string, string) {})
	ctx := context.Background()
	var remoteCalls atomic.Int32

	service.SetCommandRunner(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "git" && strings.Join(args, " ") == "remote get-url origin" {
			remoteCalls.Add(1)
			return exec.CommandContext(ctx, "sh", "-c", "printf '%s' 'git@github.com:org/repo.git'")
		}
		return exec.CommandContext(ctx, "sh", "-c", "printf ''")
	})

	assert.Equal(t, "git@github.com:org/repo.git", service.getRemoteURL(ctx))
	assert.Equal(t, "git@github.com:org/repo.git", service.getRemoteURL(ctx))
	assert.Equal(t, int32(1), remoteCalls.Load())
}

func TestGetRemoteURLCachesEmptyResult(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("requires sh")
	}

	service := NewService(func(string, string) {}, func(string, string, string) {})
	ctx := context.Background()
	var remoteCalls atomic.Int32

	service.SetCommandRunner(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "git" && strings.Join(args, " ") == "remote get-url origin" {
			remoteCalls.Add(1)
			return exec.CommandContext(ctx, "sh", "-c", "printf ''")
		}
		return exec.CommandContext(ctx, "sh", "-c", "printf ''")
	})

	assert.Empty(t, service.getRemoteURL(ctx))
	assert.Empty(t, service.getRemoteURL(ctx))
	assert.Equal(t, int32(1), remoteCalls.Load())
}

func TestGetHeadSHA(t *testing.T) {
	t.Parallel()
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}

	service := NewService(notify, notifyOnce)
	ctx := context.Background()

	t.Run("returns HEAD SHA for valid repo", func(t *testing.T) {
		tmpDir := t.TempDir()
		setupGitRepo(t, tmpDir)

		sha := service.GetHeadSHA(ctx, tmpDir)
		assert.NotEmpty(t, sha)
		assert.Len(t, sha, 40)
	})

	t.Run("returns empty for non-repo directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		sha := service.GetHeadSHA(ctx, tmpDir)
		assert.Empty(t, sha)
	})
}

func TestGetMergedBranches(t *testing.T) {
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}

	service := NewService(notify, notifyOnce)
	ctx := context.Background()
	tmpDir := t.TempDir()

	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()

	require.NoError(t, os.Chdir(tmpDir))

	cmd := exec.Command("git", "init", "-b", "main")
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	require.NoError(t, cmd.Run())
	cmd = exec.Command("git", "config", "user.name", "Test")
	require.NoError(t, cmd.Run())
	cmd = exec.Command("git", "config", "commit.gpgsign", "false")
	require.NoError(t, cmd.Run())

	require.NoError(t, os.WriteFile("file.txt", []byte("initial"), 0o600))
	cmd = exec.Command("git", "add", "file.txt")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "git add failed: %s", string(output))
	cmd = exec.Command("git", "commit", "-m", "initial")
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "git commit failed: %s", string(output))

	cmd = exec.Command("git", "checkout", "-b", "feature-branch")
	require.NoError(t, cmd.Run())
	require.NoError(t, os.WriteFile("feature.txt", []byte("feature"), 0o600))
	cmd = exec.Command("git", "add", "feature.txt")
	require.NoError(t, cmd.Run())
	cmd = exec.Command("git", "commit", "-m", "feature")
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "checkout", "main")
	require.NoError(t, cmd.Run())
	cmd = exec.Command("git", "merge", "feature-branch")
	require.NoError(t, cmd.Run())

	merged := service.GetMergedBranches(ctx, "main")
	assert.Contains(t, merged, "feature-branch")

	cmd = exec.Command("git", "checkout", "-b", "unmerged-branch")
	require.NoError(t, cmd.Run())
	require.NoError(t, os.WriteFile("unmerged.txt", []byte("unmerged"), 0o600))
	cmd = exec.Command("git", "add", "unmerged.txt")
	require.NoError(t, cmd.Run())
	cmd = exec.Command("git", "commit", "-m", "unmerged")
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "checkout", "main")
	require.NoError(t, cmd.Run())

	merged = service.GetMergedBranches(ctx, "main")
	assert.Contains(t, merged, "feature-branch")
	assert.NotContains(t, merged, "unmerged-branch")
}
