package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetMainWorktreePathFallback(t *testing.T) {
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}

	service := NewService(notify, notifyOnce)
	ctx := context.Background()

	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()

	tmpDir := t.TempDir()
	require.NoError(t, os.Chdir(tmpDir))

	path := service.GetMainWorktreePath(ctx)
	expected, err := filepath.EvalSymlinks(tmpDir)
	require.NoError(t, err)
	actual, err := filepath.EvalSymlinks(path)
	require.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestGetMainWorktreePathCachesResult(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("requires sh")
	}

	service := NewService(func(string, string) {}, func(string, string, string) {})
	ctx := context.Background()
	var listCalls atomic.Int32

	service.SetCommandRunner(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "git" && strings.Join(args, " ") == "worktree list --porcelain" {
			listCalls.Add(1)
			return exec.CommandContext(ctx, "sh", "-c", "printf '%s' 'worktree /tmp/main\nbranch refs/heads/main\n'")
		}
		return exec.CommandContext(ctx, "sh", "-c", "printf ''")
	})

	assert.Equal(t, "/tmp/main", service.GetMainWorktreePath(ctx))
	assert.Equal(t, "/tmp/main", service.GetMainWorktreePath(ctx))
	assert.Equal(t, int32(1), listCalls.Load())
}

func TestRenameWorktree(t *testing.T) {
	t.Parallel()
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}

	service := NewService(notify, notifyOnce)
	ctx := context.Background()

	t.Run("renames branch when worktree name equals branch", func(t *testing.T) {
		tmpDir := t.TempDir()
		oldPath := filepath.Join(tmpDir, "feature")
		newPath := filepath.Join(tmpDir, "new-feature")
		require.NoError(t, os.MkdirAll(newPath, 0o750))

		var commands [][]string
		service.SetCommandRunner(func(ctx context.Context, name string, args ...string) *exec.Cmd {
			commands = append(commands, append([]string{name}, args...))
			return exec.CommandContext(ctx, "sh", "-c", "exit 0")
		})

		ok := service.RenameWorktree(ctx, oldPath, newPath, "feature", "new-feature")
		require.True(t, ok)
		require.Len(t, commands, 2)
		assert.Equal(t, []string{"git", "worktree", "move", oldPath, newPath}, commands[0])
		assert.Equal(t, []string{"git", "branch", "-m", "feature", "new-feature"}, commands[1])
	})

	t.Run("skips branch rename when worktree name differs from branch", func(t *testing.T) {
		tmpDir := t.TempDir()
		oldPath := filepath.Join(tmpDir, "worktree-custom-name")
		newPath := filepath.Join(tmpDir, "new-worktree-name")

		var commands [][]string
		service.SetCommandRunner(func(ctx context.Context, name string, args ...string) *exec.Cmd {
			commands = append(commands, append([]string{name}, args...))
			return exec.CommandContext(ctx, "sh", "-c", "exit 0")
		})

		ok := service.RenameWorktree(ctx, oldPath, newPath, "feature", "new-worktree-name")
		require.True(t, ok)
		require.Len(t, commands, 1)
		assert.Equal(t, []string{"git", "worktree", "move", oldPath, newPath}, commands[0])
	})
}

func TestWorktreeOperations(t *testing.T) {
	t.Parallel()
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}

	service := NewService(notify, notifyOnce)
	ctx := context.Background()

	t.Run("get worktrees from non-git directory", func(t *testing.T) {
		worktrees, err := service.GetWorktrees(ctx)
		if err != nil {
			require.Error(t, err)
			assert.Nil(t, worktrees)
		} else {
			assert.IsType(t, []*models.WorktreeInfo{}, worktrees)
		}
	})
}

func TestCherryPickCommit(t *testing.T) {
	t.Parallel()
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}

	service := NewService(notify, notifyOnce)
	ctx := context.Background()

	t.Run("cherry-pick to non-existent directory fails", func(t *testing.T) {
		success, err := service.CherryPickCommit(ctx, "abc1234", "/nonexistent/path")
		assert.False(t, success)
		assert.Error(t, err)
	})

	t.Run("cherry-pick with empty commit SHA", func(t *testing.T) {
		tmpDir := t.TempDir()
		success, err := service.CherryPickCommit(ctx, "", tmpDir)
		assert.False(t, success)
		assert.Error(t, err)
	})

	t.Run("cherry-pick detects dirty worktree", func(t *testing.T) {
		tmpDir := t.TempDir()
		setupGitRepo(t, tmpDir)

		dirtyFile := filepath.Join(tmpDir, "dirty.txt")
		err := os.WriteFile(dirtyFile, []byte("uncommitted changes"), 0o600)
		require.NoError(t, err)

		success, err := service.CherryPickCommit(ctx, "abc1234", tmpDir)
		assert.False(t, success)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "uncommitted changes")
	})

	t.Run("cherry-pick with invalid commit SHA", func(t *testing.T) {
		tmpDir := t.TempDir()
		setupGitRepo(t, tmpDir)

		success, err := service.CherryPickCommit(ctx, "invalid-sha", tmpDir)
		assert.False(t, success)
		assert.Error(t, err)
	})
}
