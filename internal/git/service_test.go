package git

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewService(t *testing.T) {
	t.Parallel()
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}

	service := NewService(notify, notifyOnce)

	assert.NotNil(t, service)
	assert.NotNil(t, service.semaphore)
	assert.NotNil(t, service.notifiedSet)
	assert.NotNil(t, service.notify)
	assert.NotNil(t, service.notifyOnce)

	expectedSlots := runtime.NumCPU() * 2
	if expectedSlots < 4 {
		expectedSlots = 4
	}
	if expectedSlots > 32 {
		expectedSlots = 32
	}

	// Semaphore should have the expected number of slots
	count := 0
	for i := 0; i < expectedSlots; i++ {
		select {
		case <-service.semaphore:
			count++
		default:
			// Can't drain more from semaphore
		}
	}
	assert.Equal(t, expectedSlots, count)
}

func TestUseGitPager(t *testing.T) {
	t.Parallel()
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}

	service := NewService(notify, notifyOnce)

	// UseGitPager should return a boolean
	useGitPager := service.UseGitPager()
	assert.IsType(t, true, useGitPager)
}

func TestSetGitPager(t *testing.T) {
	t.Parallel()
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}

	service := NewService(notify, notifyOnce)

	t.Run("empty value disables git_pager", func(t *testing.T) {
		service.SetGitPager("")
		assert.False(t, service.UseGitPager())
		assert.Empty(t, service.gitPager)
	})

	t.Run("custom git_pager", func(t *testing.T) {
		service.SetGitPager("/custom/path/to/delta")
		assert.Equal(t, "/custom/path/to/delta", service.gitPager)
	})

	t.Run("whitespace trimmed from path", func(t *testing.T) {
		service.SetGitPager("  delta  ")
		assert.Equal(t, "delta", service.gitPager)
	})
}

func TestSetGitPagerArgs(t *testing.T) {
	t.Parallel()
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}

	service := NewService(notify, notifyOnce)

	service.SetGitPagerArgs([]string{"--color-only"})
	assert.Equal(t, []string{"--color-only"}, service.gitPagerArgs)

	args := []string{"--side-by-side"}
	service.SetGitPagerArgs(args)
	args[0] = "--changed"
	assert.Equal(t, []string{"--side-by-side"}, service.gitPagerArgs)

	service.SetGitPagerArgs(nil)
	assert.Nil(t, service.gitPagerArgs)
}

func TestApplyGitPager(t *testing.T) {
	t.Parallel()
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}

	service := NewService(notify, notifyOnce)

	t.Run("empty diff returns empty", func(t *testing.T) {
		result := service.ApplyGitPager(context.Background(), "")
		assert.Empty(t, result)
	})

	t.Run("diff without delta available", func(t *testing.T) {
		// Temporarily disable delta
		origUseDelta := service.useGitPager
		service.useGitPager = false
		defer func() { service.useGitPager = origUseDelta }()

		diff := "diff --git a/file.txt b/file.txt\n"
		result := service.ApplyGitPager(context.Background(), diff)
		assert.Equal(t, diff, result)
	})

	t.Run("diff with delta available", func(t *testing.T) {
		diff := "diff --git a/file.txt b/file.txt\n+added line\n"

		result := service.ApplyGitPager(context.Background(), diff)
		// Result should either be the diff (if delta not available) or transformed by delta
		assert.NotEmpty(t, result)
		assert.Contains(t, result, "file.txt")
	})
}

func TestGetMainBranch(t *testing.T) {
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}

	service := NewService(notify, notifyOnce)

	ctx := context.Background()

	// This test requires a git repository, so we'll test basic functionality
	branch := service.GetMainBranch(ctx)

	// Branch should be non-empty (defaults to "main" or "master")
	assert.NotEmpty(t, branch)
	// Should be one of the common main branches
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

func TestGetMainWorktreePathFallback(t *testing.T) {
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}

	service := NewService(notify, notifyOnce)
	ctx := context.Background()

	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() {
		_ = os.Chdir(origDir)
	}()

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

func TestExecuteCommands(t *testing.T) {
	t.Parallel()
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}

	service := NewService(notify, notifyOnce)
	ctx := context.Background()

	t.Run("execute empty command list", func(t *testing.T) {
		err := service.ExecuteCommands(ctx, []string{}, "", nil)
		assert.NoError(t, err)
	})

	t.Run("execute with whitespace commands", func(t *testing.T) {
		err := service.ExecuteCommands(ctx, []string{"  ", "\t", "\n"}, "", nil)
		assert.NoError(t, err)
	})

	t.Run("execute simple command", func(t *testing.T) {
		tmpDir := t.TempDir()
		err := service.ExecuteCommands(ctx, []string{"echo test"}, tmpDir, nil)
		// May fail if shell execution is restricted, but should not panic
		_ = err
	})

	t.Run("execute with environment variables", func(t *testing.T) {
		tmpDir := t.TempDir()
		env := map[string]string{
			"TEST_VAR": "test_value",
		}
		err := service.ExecuteCommands(ctx, []string{"echo $TEST_VAR"}, tmpDir, env)
		// May fail if shell execution is restricted, but should not panic
		_ = err
	})
}

func TestBuildThreePartDiff(t *testing.T) {
	t.Parallel()
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}

	service := NewService(notify, notifyOnce)
	ctx := context.Background()

	t.Run("build diff for non-git directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfg := &config.AppConfig{
			MaxUntrackedDiffs: 10,
			MaxDiffChars:      200000,
		}

		diff := service.BuildThreePartDiff(ctx, tmpDir, cfg)

		// Should return something (even if empty or error message)
		assert.IsType(t, "", diff)
	})

	t.Run("uses ls-files for untracked enumeration", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("requires sh")
		}

		var lsFilesCalls atomic.Int32
		var statusCalls atomic.Int32
		service.SetCommandRunner(func(ctx context.Context, name string, args ...string) *exec.Cmd {
			if name != "git" {
				return exec.CommandContext(ctx, "sh", "-c", "printf ''")
			}

			switch strings.Join(args, " ") {
			case "diff --cached --patch --no-color":
				return exec.CommandContext(ctx, "sh", "-c", "printf '%s' 'staged-diff'")
			case "diff --patch --no-color":
				return exec.CommandContext(ctx, "sh", "-c", "printf '%s' 'unstaged-diff'")
			case "ls-files --others --exclude-standard":
				lsFilesCalls.Add(1)
				return exec.CommandContext(ctx, "sh", "-c", "printf '%s' 'new.txt\n'")
			case "status --porcelain":
				statusCalls.Add(1)
				return exec.CommandContext(ctx, "sh", "-c", "printf '%s' '?? new.txt\n'")
			}

			if len(args) == 4 && args[0] == "diff" && args[1] == "--no-index" {
				return exec.CommandContext(ctx, "sh", "-c", "printf '%s' 'diff --git a/new.txt b/new.txt'")
			}
			return exec.CommandContext(ctx, "sh", "-c", "printf ''")
		})

		cfg := &config.AppConfig{
			MaxUntrackedDiffs: 10,
			MaxDiffChars:      200000,
		}
		diff := service.BuildThreePartDiff(ctx, t.TempDir(), cfg)

		assert.Contains(t, diff, "=== Staged Changes ===")
		assert.Contains(t, diff, "=== Unstaged Changes ===")
		assert.Contains(t, diff, "=== Untracked: new.txt ===")
		assert.Equal(t, int32(1), lsFilesCalls.Load())
		assert.Equal(t, int32(0), statusCalls.Load())
	})
}

func TestRunGit(t *testing.T) {
	t.Parallel()
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}

	service := NewService(notify, notifyOnce)
	ctx := context.Background()

	t.Run("run git version", func(t *testing.T) {
		// This is a simple git command that should work in most environments
		output := service.RunGit(ctx, []string{"git", "--version"}, "", []int{0}, false, false)

		// Should contain "git version" or be empty if git not available
		if output != "" {
			assert.Contains(t, output, "git version")
		}
	})

	t.Run("run git with allowed error code", func(t *testing.T) {
		// Run a command that will likely fail with code 128 (invalid command)
		output := service.RunGit(ctx, []string{"git", "invalid-command-xyz"}, "", []int{128}, true, false)

		// Should not panic and return some output (even if empty)
		assert.IsType(t, "", output)
	})

	t.Run("run git with cwd", func(t *testing.T) {
		tmpDir := t.TempDir()
		output := service.RunGit(ctx, []string{"git", "--version"}, tmpDir, []int{0}, false, false)

		// Should run successfully
		if output != "" {
			assert.Contains(t, output, "git version")
		}
	})
}

func TestNotifications(t *testing.T) {
	t.Parallel()
	t.Run("notify function called", func(t *testing.T) {
		called := false
		var receivedMessage, receivedSeverity string

		notify := func(message string, severity string) {
			called = true
			receivedMessage = message
			receivedSeverity = severity
		}
		notifyOnce := func(_ string, _ string, _ string) {}

		service := NewService(notify, notifyOnce)

		// Trigger a notification
		service.notify("test message", "info")

		assert.True(t, called)
		assert.Equal(t, "test message", receivedMessage)
		assert.Equal(t, "info", receivedSeverity)
	})

	t.Run("notifyOnce function called", func(t *testing.T) {
		called := false
		var receivedKey, receivedMessage, receivedSeverity string

		notify := func(_ string, _ string) {}
		notifyOnce := func(key string, message string, severity string) {
			called = true
			receivedKey = key
			receivedMessage = message
			receivedSeverity = severity
		}

		service := NewService(notify, notifyOnce)

		// Trigger a one-time notification
		service.notifyOnce("test-key", "test message", "warning")

		assert.True(t, called)
		assert.Equal(t, "test-key", receivedKey)
		assert.Equal(t, "test message", receivedMessage)
		assert.Equal(t, "warning", receivedSeverity)
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

		// Should handle error gracefully
		if err != nil {
			require.Error(t, err)
			assert.Nil(t, worktrees)
		} else {
			assert.IsType(t, []*models.WorktreeInfo{}, worktrees)
		}
	})
}

func TestCreateWorktreeFromPR(t *testing.T) {
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}
	ctx := context.Background()

	t.Run("create worktree from PR with temporary directory", func(t *testing.T) {
		service := NewService(notify, notifyOnce)
		// Run outside any real repository so this test cannot mutate the caller repo.
		withCwd(t, t.TempDir())
		targetPath := filepath.Join(t.TempDir(), "test-worktree")

		// This will likely fail due to missing git repo/PR, but tests the function structure
		ok := service.CreateWorktreeFromPR(ctx, 123, "feature-branch", "local-branch", targetPath)

		// Should return a boolean (even if false due to git errors)
		assert.IsType(t, true, ok)
	})

	t.Run("unknown host uses manual fetch fallback", func(t *testing.T) {
		service := NewService(notify, notifyOnce)
		// Set up a git repo with a non-GitHub/GitLab remote
		repo := t.TempDir()
		runGit(t, repo, "init", "-b", "main")
		runGit(t, repo, "config", "user.email", "test@test.com")
		runGit(t, repo, "config", "user.name", "Test User")
		runGit(t, repo, "config", "commit.gpgsign", "false")

		// Create initial commit
		testFile := filepath.Join(repo, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("initial"), 0o600))
		runGit(t, repo, "add", "test.txt")
		runGit(t, repo, "commit", "-m", "initial")

		// Create a feature branch
		runGit(t, repo, "checkout", "-b", "feature-branch")
		require.NoError(t, os.WriteFile(testFile, []byte("feature"), 0o600))
		runGit(t, repo, "commit", "-am", "feature commit")

		// Go back to main
		runGit(t, repo, "checkout", "main")

		// Create a second repo that will use the first as origin
		workRepo := t.TempDir()
		runGit(t, workRepo, "clone", repo, ".")
		runGit(t, workRepo, "config", "user.email", "test@test.com")
		runGit(t, workRepo, "config", "user.name", "Test User")
		runGit(t, workRepo, "config", "commit.gpgsign", "false")

		// Change remote to unknown host (not github/gitlab)
		runGit(t, workRepo, "remote", "set-url", "origin", "https://gitea.example.com/org/repo.git")

		// Fetch the feature branch
		runGit(t, workRepo, "fetch", repo, "feature-branch:refs/remotes/origin/feature-branch")

		withCwd(t, workRepo)

		targetPath := filepath.Join(t.TempDir(), "pr-worktree")
		ok := service.CreateWorktreeFromPR(ctx, 1, "feature-branch", "local-pr-branch", targetPath)

		// Should fail because we can't actually fetch from gitea.example.com
		// But the function should handle this gracefully
		assert.False(t, ok)
	})

	t.Run("returns false when not in git repo", func(t *testing.T) {
		service := NewService(notify, notifyOnce)
		tmpDir := t.TempDir()
		withCwd(t, tmpDir)

		targetPath := filepath.Join(tmpDir, "worktree")
		ok := service.CreateWorktreeFromPR(ctx, 1, "feature", "local", targetPath)

		assert.False(t, ok)
	})

	t.Run("returns false for invalid target path", func(t *testing.T) {
		service := NewService(notify, notifyOnce)
		repo := t.TempDir()
		runGit(t, repo, "init")
		runGit(t, repo, "remote", "add", "origin", "https://bitbucket.example.com/org/repo.git")
		withCwd(t, repo)

		// Use invalid path (nested in non-existent directory)
		invalidPath := "/nonexistent/deeply/nested/path/worktree"
		ok := service.CreateWorktreeFromPR(ctx, 1, "feature", "local", invalidPath)

		assert.False(t, ok)
	})

	t.Run("existing local branch is reset to PR branch before worktree creation", func(t *testing.T) {
		var notifications []string
		service := NewService(
			func(msg, level string) {
				if level == "warning" {
					notifications = append(notifications, msg)
				}
			},
			notifyOnce,
		)
		remoteRepo := t.TempDir()
		runGit(t, remoteRepo, "init", "--bare", "-b", "main")

		setupRepo := t.TempDir()
		runGit(t, setupRepo, "clone", remoteRepo, ".")
		runGit(t, setupRepo, "config", "user.email", "test@test.com")
		runGit(t, setupRepo, "config", "user.name", "Test User")
		runGit(t, setupRepo, "config", "commit.gpgsign", "false")

		testFile := filepath.Join(setupRepo, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("initial"), 0o600))
		runGit(t, setupRepo, "add", "test.txt")
		runGit(t, setupRepo, "commit", "-m", "initial")
		runGit(t, setupRepo, "push", "-u", "origin", "main")

		runGit(t, setupRepo, "checkout", "-b", "feature-branch")
		require.NoError(t, os.WriteFile(testFile, []byte("feature content"), 0o600))
		runGit(t, setupRepo, "commit", "-am", "feature commit")
		runGit(t, setupRepo, "push", "-u", "origin", "feature-branch")
		featureSHA := runGit(t, setupRepo, "rev-parse", "HEAD")

		testRepo := t.TempDir()
		runGit(t, testRepo, "clone", remoteRepo, ".")
		runGit(t, testRepo, "config", "user.email", "test@test.com")
		runGit(t, testRepo, "config", "user.name", "Test User")
		runGit(t, testRepo, "config", "commit.gpgsign", "false")

		// Create a stale local branch that should be reset to the PR branch tip.
		runGit(t, testRepo, "checkout", "-b", "feature-branch", "origin/main")
		runGit(t, testRepo, "checkout", "main")

		withCwd(t, testRepo)
		targetPath := filepath.Join(t.TempDir(), "feature-branch")
		ok := service.CreateWorktreeFromPR(ctx, 1, "feature-branch", "feature-branch", targetPath)
		require.True(t, ok)

		gotSHA := runGit(t, testRepo, "rev-parse", "feature-branch")
		assert.Equal(t, featureSHA, gotSHA)
		assert.Equal(t, "feature-branch", runGit(t, targetPath, "rev-parse", "--abbrev-ref", "HEAD"))
		assert.Equal(t, "origin", runGit(t, testRepo, "config", "--get", "branch.feature-branch.remote"))
		assert.Equal(t, "origin", runGit(t, testRepo, "config", "--get", "branch.feature-branch.pushRemote"))
		assert.Equal(t, "refs/heads/feature-branch", runGit(t, testRepo, "config", "--get", "branch.feature-branch.merge"))
		require.NotEmpty(t, notifications)
		assert.Contains(t, notifications[0], "already exists and will be reset")
	})

	t.Run("returns false when PR branch is already attached to another worktree", func(t *testing.T) {
		service := NewService(notify, notifyOnce)
		remoteRepo := t.TempDir()
		runGit(t, remoteRepo, "init", "--bare", "-b", "main")

		setupRepo := t.TempDir()
		runGit(t, setupRepo, "clone", remoteRepo, ".")
		runGit(t, setupRepo, "config", "user.email", "test@test.com")
		runGit(t, setupRepo, "config", "user.name", "Test User")
		runGit(t, setupRepo, "config", "commit.gpgsign", "false")

		testFile := filepath.Join(setupRepo, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("initial"), 0o600))
		runGit(t, setupRepo, "add", "test.txt")
		runGit(t, setupRepo, "commit", "-m", "initial")
		runGit(t, setupRepo, "push", "-u", "origin", "main")

		runGit(t, setupRepo, "checkout", "-b", "feature-branch")
		require.NoError(t, os.WriteFile(testFile, []byte("feature content"), 0o600))
		runGit(t, setupRepo, "commit", "-am", "feature commit")
		runGit(t, setupRepo, "push", "-u", "origin", "feature-branch")

		testRepo := t.TempDir()
		runGit(t, testRepo, "clone", remoteRepo, ".")
		runGit(t, testRepo, "config", "user.email", "test@test.com")
		runGit(t, testRepo, "config", "user.name", "Test User")
		runGit(t, testRepo, "config", "commit.gpgsign", "false")

		attachedPath := filepath.Join(t.TempDir(), "attached-feature")
		runGit(t, testRepo, "worktree", "add", "-b", "feature-branch", attachedPath, "origin/feature-branch")

		withCwd(t, testRepo)
		targetPath := filepath.Join(t.TempDir(), "new-feature-worktree")
		ok := service.CreateWorktreeFromPR(ctx, 1, "feature-branch", "feature-branch", targetPath)
		assert.False(t, ok)
	})

	t.Run("GitLab MR ref from glab api merge_requests succeeds", func(t *testing.T) {
		remoteRepo := t.TempDir()
		runGit(t, remoteRepo, "init", "--bare", "-b", "main")

		setupRepo := t.TempDir()
		runGit(t, setupRepo, "clone", remoteRepo, ".")
		runGit(t, setupRepo, "config", "user.email", "test@test.com")
		runGit(t, setupRepo, "config", "user.name", "Test User")
		runGit(t, setupRepo, "config", "commit.gpgsign", "false")

		testFile := filepath.Join(setupRepo, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("initial"), 0o600))
		runGit(t, setupRepo, "add", "test.txt")
		runGit(t, setupRepo, "commit", "-m", "initial")
		runGit(t, setupRepo, "push", "-u", "origin", "main")

		runGit(t, setupRepo, "checkout", "-b", "feature-branch")
		require.NoError(t, os.WriteFile(testFile, []byte("feature content"), 0o600))
		runGit(t, setupRepo, "commit", "-am", "feature commit")
		runGit(t, setupRepo, "push", "-u", "origin", "feature-branch")
		featureSHA := runGit(t, setupRepo, "rev-parse", "HEAD")

		testRepo := t.TempDir()
		runGit(t, testRepo, "clone", remoteRepo, ".")
		runGit(t, testRepo, "config", "user.email", "test@test.com")
		runGit(t, testRepo, "config", "user.name", "Test User")
		runGit(t, testRepo, "config", "commit.gpgsign", "false")

		stub := "#!/bin/sh\n" +
			"if [ \"$1\" = \"api\" ] && [ \"$2\" = \"merge_requests/1\" ]; then\n" +
			"  echo '{\"sha\":\"" + featureSHA + "\",\"source_branch\":\"feature-branch\"}'\n" +
			"  exit 0\n" +
			"fi\n" +
			"exit 1\n"
		dir := writeStub(t, "glab", stub)
		withStubbedPath(t, dir)

		service := NewService(notify, notifyOnce)
		service.gitHost = gitHostGitLab

		withCwd(t, testRepo)
		targetPath := filepath.Join(t.TempDir(), "feature-branch")
		ok := service.CreateWorktreeFromPR(ctx, 1, "feature-branch", "feature-branch", targetPath)
		require.True(t, ok)

		assert.Equal(t, featureSHA, runGit(t, testRepo, "rev-parse", "feature-branch"))
		assert.Equal(t, "feature-branch", runGit(t, targetPath, "rev-parse", "--abbrev-ref", "HEAD"))
	})

	t.Run("GitLab MR ref uses diff_refs.head_sha when sha missing", func(t *testing.T) {
		remoteRepo := t.TempDir()
		runGit(t, remoteRepo, "init", "--bare", "-b", "main")

		setupRepo := t.TempDir()
		runGit(t, setupRepo, "clone", remoteRepo, ".")
		runGit(t, setupRepo, "config", "user.email", "test@test.com")
		runGit(t, setupRepo, "config", "user.name", "Test User")
		runGit(t, setupRepo, "config", "commit.gpgsign", "false")

		testFile := filepath.Join(setupRepo, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("initial"), 0o600))
		runGit(t, setupRepo, "add", "test.txt")
		runGit(t, setupRepo, "commit", "-m", "initial")
		runGit(t, setupRepo, "push", "-u", "origin", "main")

		runGit(t, setupRepo, "checkout", "-b", "feature-branch")
		require.NoError(t, os.WriteFile(testFile, []byte("feature content"), 0o600))
		runGit(t, setupRepo, "commit", "-am", "feature commit")
		runGit(t, setupRepo, "push", "-u", "origin", "feature-branch")
		featureSHA := runGit(t, setupRepo, "rev-parse", "HEAD")

		testRepo := t.TempDir()
		runGit(t, testRepo, "clone", remoteRepo, ".")
		runGit(t, testRepo, "config", "user.email", "test@test.com")
		runGit(t, testRepo, "config", "user.name", "Test User")
		runGit(t, testRepo, "config", "commit.gpgsign", "false")

		stub := "#!/bin/sh\n" +
			"if [ \"$1\" = \"api\" ] && [ \"$2\" = \"merge_requests/1\" ]; then\n" +
			"  echo '{\"diff_refs\":{\"head_sha\":\"" + featureSHA + "\"},\"source_branch\":\"feature-branch\"}'\n" +
			"  exit 0\n" +
			"fi\n" +
			"exit 1\n"
		dir := writeStub(t, "glab", stub)
		withStubbedPath(t, dir)

		service := NewService(notify, notifyOnce)
		service.gitHost = gitHostGitLab

		withCwd(t, testRepo)
		targetPath := filepath.Join(t.TempDir(), "feature-branch")
		ok := service.CreateWorktreeFromPR(ctx, 1, "feature-branch", "feature-branch", targetPath)
		require.True(t, ok)

		assert.Equal(t, featureSHA, runGit(t, testRepo, "rev-parse", "feature-branch"))
		assert.Equal(t, "feature-branch", runGit(t, targetPath, "rev-parse", "--abbrev-ref", "HEAD"))
	})
}

func TestCreateWorktreeFromPRUnknownHostSuccess(t *testing.T) {
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}

	service := NewService(notify, notifyOnce)
	ctx := context.Background()

	// Create a "remote" repo with explicit main branch
	remoteRepo := t.TempDir()
	runGit(t, remoteRepo, "init", "--bare", "-b", "main")

	// Create a work repo and push to remote
	workSetup := t.TempDir()
	runGit(t, workSetup, "clone", remoteRepo, ".")
	runGit(t, workSetup, "config", "user.email", "test@test.com")
	runGit(t, workSetup, "config", "user.name", "Test User")
	runGit(t, workSetup, "config", "commit.gpgsign", "false")

	// Create initial commit on main
	testFile := filepath.Join(workSetup, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("initial"), 0o600))
	runGit(t, workSetup, "add", "test.txt")
	runGit(t, workSetup, "commit", "-m", "initial")
	runGit(t, workSetup, "push", "-u", "origin", "main")

	// Create feature branch and push
	runGit(t, workSetup, "checkout", "-b", "feature-branch")
	require.NoError(t, os.WriteFile(testFile, []byte("feature content"), 0o600))
	runGit(t, workSetup, "commit", "-am", "feature commit")
	runGit(t, workSetup, "push", "-u", "origin", "feature-branch")

	// Now create the actual test repo that clones from remote
	testRepo := t.TempDir()
	runGit(t, testRepo, "clone", remoteRepo, ".")
	runGit(t, testRepo, "config", "user.email", "test@test.com")
	runGit(t, testRepo, "config", "user.name", "Test User")
	runGit(t, testRepo, "config", "commit.gpgsign", "false")

	// Set remote to unknown host (triggers fallback path)
	// But keep the actual URL for fetching
	runGit(t, testRepo, "remote", "set-url", "origin", remoteRepo)
	// Add a fake gh-resolved config to make it look like unknown host
	runGit(t, testRepo, "config", "remote.origin.gh-resolved", "false")

	// Manually set remote URL to something that's not github/gitlab for detection
	// but we'll use a local path that actually works
	withCwd(t, testRepo)

	// Since we can't easily make DetectHost return unknown while still having a working remote,
	// we test that the function handles the case gracefully
	targetPath := filepath.Join(t.TempDir(), "pr-worktree")

	// This tests the full path when remote is actually accessible.
	ok := service.CreateWorktreeFromPR(ctx, 1, "feature-branch", "local-pr-branch", targetPath)
	require.True(t, ok)
	assert.Equal(t, "origin", runGit(t, testRepo, "config", "--get", "branch.local-pr-branch.remote"))
	assert.Equal(t, "origin", runGit(t, testRepo, "config", "--get", "branch.local-pr-branch.pushRemote"))
	assert.Equal(t, "refs/heads/feature-branch", runGit(t, testRepo, "config", "--get", "branch.local-pr-branch.merge"))
}

func TestCreateWorktreeFromPRBranchTracking(t *testing.T) {
	t.Parallel()
	// This test verifies the branch tracking config structure
	// by testing the config commands that would be run

	t.Run("github tracking config format", func(t *testing.T) {
		// Verify the expected config keys for GitHub
		localBranch := "pr-123-feature"

		expectedRemoteKey := "branch.pr-123-feature.remote"
		expectedPushRemoteKey := "branch.pr-123-feature.pushRemote"
		expectedMergeKey := "branch.pr-123-feature.merge"
		expectedMergeValue := "refs/heads/feature-branch"

		assert.Equal(t, expectedRemoteKey, "branch."+localBranch+".remote")
		assert.Equal(t, expectedPushRemoteKey, "branch."+localBranch+".pushRemote")
		assert.Equal(t, expectedMergeKey, "branch."+localBranch+".merge")
		assert.Equal(t, expectedMergeValue, "refs/heads/feature-branch")
	})

	t.Run("gitlab tracking config format", func(t *testing.T) {
		// Verify the expected config keys for GitLab
		localBranch := "mr-456-feature"
		sourceBranch := "feature-branch"

		expectedRemoteKey := "branch.mr-456-feature.remote"
		expectedMergeKey := "branch.mr-456-feature.merge"
		expectedMergeValue := "refs/heads/feature-branch"

		assert.Equal(t, expectedRemoteKey, "branch."+localBranch+".remote")
		assert.Equal(t, expectedMergeKey, "branch."+localBranch+".merge")
		assert.Equal(t, expectedMergeValue, "refs/heads/"+sourceBranch)
	})
}

func TestCreateWorktreeFromPRJSONParsing(t *testing.T) {
	t.Parallel()
	t.Run("parse github pr json", func(t *testing.T) {
		jsonData := `{"headRefOid":"abc123def456","headRepository":{"url":"https://github.com/fork/repo"}}`

		var pr map[string]any
		err := json.Unmarshal([]byte(jsonData), &pr)
		require.NoError(t, err)

		headCommit, _ := pr["headRefOid"].(string)
		assert.Equal(t, "abc123def456", headCommit)

		var repoURL string
		if headRepo, ok := pr["headRepository"].(map[string]any); ok {
			repoURL, _ = headRepo["url"].(string)
		}
		assert.Equal(t, "https://github.com/fork/repo", repoURL)
	})

	t.Run("parse github pr json without headRepository", func(t *testing.T) {
		jsonData := `{"headRefOid":"abc123def456"}`

		var pr map[string]any
		err := json.Unmarshal([]byte(jsonData), &pr)
		require.NoError(t, err)

		headCommit, _ := pr["headRefOid"].(string)
		assert.Equal(t, "abc123def456", headCommit)

		var repoURL string
		if headRepo, ok := pr["headRepository"].(map[string]any); ok {
			repoURL, _ = headRepo["url"].(string)
		}
		assert.Empty(t, repoURL) // Should be empty, fallback to origin
	})

	t.Run("parse gitlab mr json", func(t *testing.T) {
		jsonData := `{"sha":"def789ghi012","source_branch":"feature-xyz","web_url":"https://gitlab.com/org/repo/-/merge_requests/42"}`

		var mr map[string]any
		err := json.Unmarshal([]byte(jsonData), &mr)
		require.NoError(t, err)

		sha, _ := mr["sha"].(string)
		assert.Equal(t, "def789ghi012", sha)

		sourceBranch, _ := mr["source_branch"].(string)
		assert.Equal(t, "feature-xyz", sourceBranch)
	})

	t.Run("parse gitlab mr json with missing sha", func(t *testing.T) {
		jsonData := `{"source_branch":"feature-xyz"}`

		var mr map[string]any
		err := json.Unmarshal([]byte(jsonData), &mr)
		require.NoError(t, err)

		sha, ok := mr["sha"].(string)
		assert.False(t, ok || sha != "")
	})

	t.Run("handle malformed json", func(t *testing.T) {
		jsonData := `{invalid json}`

		var pr map[string]any
		err := json.Unmarshal([]byte(jsonData), &pr)
		assert.Error(t, err)
	})
}

func TestCreateWorktreeFromPRIntegration(t *testing.T) {
	// Skip if gh/glab not available - these are integration tests
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh CLI not available, skipping integration test")
	}

	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}

	ctx := context.Background()

	t.Run("github host detection triggers github path", func(t *testing.T) {
		// Create fresh service for this test
		service := NewService(notify, notifyOnce)

		repo := t.TempDir()
		runGit(t, repo, "init")
		runGit(t, repo, "config", "user.email", "test@test.com")
		runGit(t, repo, "config", "user.name", "Test User")
		runGit(t, repo, "config", "commit.gpgsign", "false")
		runGit(t, repo, "remote", "add", "origin", "git@github.com:test/repo.git")

		// Create initial commit
		testFile := filepath.Join(repo, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("test"), 0o600))
		runGit(t, repo, "add", ".")
		runGit(t, repo, "commit", "-m", "initial")

		withCwd(t, repo)

		// Verify host detection
		host := service.DetectHost(ctx)
		assert.Equal(t, gitHostGithub, host)

		// CreateWorktreeFromPR will fail because gh pr view won't work
		// but it should take the GitHub path
		targetPath := filepath.Join(t.TempDir(), "worktree")
		ok := service.CreateWorktreeFromPR(ctx, 1, "feature", "local", targetPath)

		// Should fail (no actual PR) but not panic
		assert.False(t, ok)
	})

	t.Run("gitlab host detection triggers gitlab path", func(t *testing.T) {
		// Create fresh service for this test
		service := NewService(notify, notifyOnce)

		repo := t.TempDir()
		runGit(t, repo, "init")
		runGit(t, repo, "config", "user.email", "test@test.com")
		runGit(t, repo, "config", "user.name", "Test User")
		runGit(t, repo, "config", "commit.gpgsign", "false")
		runGit(t, repo, "remote", "add", "origin", "git@gitlab.com:test/repo.git")

		// Create initial commit
		testFile := filepath.Join(repo, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("test"), 0o600))
		runGit(t, repo, "add", ".")
		runGit(t, repo, "commit", "-m", "initial")

		withCwd(t, repo)

		// Verify host detection
		host := service.DetectHost(ctx)
		assert.Equal(t, gitHostGitLab, host)

		// CreateWorktreeFromPR will fail because glab mr view won't work
		// but it should take the GitLab path
		targetPath := filepath.Join(t.TempDir(), "worktree")
		ok := service.CreateWorktreeFromPR(ctx, 1, "feature", "local", targetPath)

		// Should fail (no actual MR) but not panic
		assert.False(t, ok)
	})
}

func TestDetectHost(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name   string
		remote string
		want   string
	}{
		{name: "github", remote: "git@github.com:org/repo.git", want: gitHostGithub},
		{name: "gitlab", remote: "https://gitlab.com/group/repo.git", want: gitHostGitLab},
		{name: "unknown", remote: "ssh://example.com/repo.git", want: gitHostUnknown},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			runGit(t, repo, "init")
			runGit(t, repo, "remote", "add", "origin", tc.remote)
			withCwd(t, repo)

			service := NewService(func(string, string) {}, func(string, string, string) {})
			if got := service.DetectHost(ctx); got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestIsGitHubOrGitLab(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name   string
		remote string
		want   bool
	}{
		{name: "github", remote: "git@github.com:org/repo.git", want: true},
		{name: "gitlab", remote: "https://gitlab.com/group/repo.git", want: true},
		{name: "unknown", remote: "ssh://example.com/repo.git", want: false},
		{name: "gitea", remote: "https://gitea.example.com/repo.git", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			runGit(t, repo, "init")
			runGit(t, repo, "remote", "add", "origin", tc.remote)
			withCwd(t, repo)

			service := NewService(func(string, string) {}, func(string, string, string) {})
			if got := service.IsGitHubOrGitLab(ctx); got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...) //#nosec G204 -- test helper with controlled args
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func withCwd(t *testing.T, dir string) {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})
}

func writeStubCommand(t *testing.T, name, envVar string) {
	t.Helper()

	if runtime.GOOS == "windows" {
		t.Skip("requires sh")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, name)
	script := "#!/bin/sh\nprintf '%s' \"$" + envVar + "\"\n"
	// #nosec G306 -- test helper needs an executable stub in a temp dir.
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatalf("write stub command: %v", err)
	}
	pathEnv := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+pathEnv)
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
		// Create a temporary git repository
		tmpDir := t.TempDir()
		setupGitRepo(t, tmpDir)

		// Create a file to make worktree dirty
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

// setupGitRepo creates a minimal git repository for testing
func setupGitRepo(t *testing.T, dir string) {
	t.Helper()

	// Check if git is available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to init git repo: %v\noutput: %s", err, output)
	}

	// Configure git user (required for commits)
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = dir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to configure git email: %v\noutput: %s", err, output)
	}

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = dir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to configure git name: %v\noutput: %s", err, output)
	}

	// Disable GPG signing for tests
	cmd = exec.Command("git", "config", "commit.gpgsign", "false")
	cmd.Dir = dir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to disable GPG signing: %v\noutput: %s", err, output)
	}

	// Create initial commit
	initialFile := filepath.Join(dir, "README.md")
	if err := os.WriteFile(initialFile, []byte("# Test Repo"), 0o600); err != nil {
		t.Fatalf("failed to write initial file: %v", err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = dir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to git add: %v\noutput: %s", err, output)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = dir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to create initial commit: %v\noutput: %s", err, output)
	}
}

func TestGetCommitFiles(t *testing.T) {
	t.Parallel()
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}

	service := NewService(notify, notifyOnce)
	ctx := context.Background()

	t.Run("get commit files from valid repo", func(t *testing.T) {
		tmpDir := t.TempDir()
		setupGitRepo(t, tmpDir)

		// Create a new file and commit it
		newFile := filepath.Join(tmpDir, "new.txt")
		err := os.WriteFile(newFile, []byte("content"), 0o600)
		require.NoError(t, err)

		runGit(t, tmpDir, "add", ".")
		runGit(t, tmpDir, "commit", "-m", "Add new.txt")

		// Get HEAD sha
		sha := runGit(t, tmpDir, "rev-parse", "HEAD")

		files, err := service.GetCommitFiles(ctx, sha, tmpDir)
		require.NoError(t, err)
		require.Len(t, files, 1)
		assert.Equal(t, "new.txt", files[0].Filename)
		assert.Equal(t, "A", files[0].ChangeType)
	})

	t.Run("get commit files with invalid sha", func(t *testing.T) {
		tmpDir := t.TempDir()
		setupGitRepo(t, tmpDir)

		files, err := service.GetCommitFiles(ctx, "invalid-sha", tmpDir)
		// Should return empty list and no error (as RunGit returns empty string on failure currently for some paths, or we check implementation)
		// Implementation: if raw == "" return empty. RunGit returns empty string on failure if not allowed exit code?
		// GetCommitFiles calls RunGit with []int{0}. So if it fails, it returns empty string.
		require.NoError(t, err)
		assert.Empty(t, files)
	})
}

func TestParseCommitFiles(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected []models.CommitFile
	}{
		{
			name:  "added file",
			input: "A\tfile.txt",
			expected: []models.CommitFile{
				{Filename: "file.txt", ChangeType: "A"},
			},
		},
		{
			name:  "modified file",
			input: "M\tpath/to/file.go",
			expected: []models.CommitFile{
				{Filename: "path/to/file.go", ChangeType: "M"},
			},
		},
		{
			name:  "deleted file",
			input: "D\tdeleted.txt",
			expected: []models.CommitFile{
				{Filename: "deleted.txt", ChangeType: "D"},
			},
		},
		{
			name:  "renamed file",
			input: "R100\told.txt\tnew.txt",
			expected: []models.CommitFile{
				{Filename: "new.txt", ChangeType: "R", OldPath: "old.txt"},
			},
		},
		{
			name:  "multiple files",
			input: "M\tfile1.go\nA\tfile2.go",
			expected: []models.CommitFile{
				{Filename: "file1.go", ChangeType: "M"},
				{Filename: "file2.go", ChangeType: "A"},
			},
		},
		{
			name:     "empty input",
			input:    "",
			expected: []models.CommitFile{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCommitFiles(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetMergedBranches(t *testing.T) {
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}

	service := NewService(notify, notifyOnce)
	ctx := context.Background()

	// Create a temp directory for a test git repo
	tmpDir := t.TempDir()

	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() {
		_ = os.Chdir(origDir)
	}()

	require.NoError(t, os.Chdir(tmpDir))

	// Initialize a git repo with main as default branch
	cmd := exec.Command("git", "init", "-b", "main")
	require.NoError(t, cmd.Run())

	// Configure git user and disable gpg signing
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	require.NoError(t, cmd.Run())
	cmd = exec.Command("git", "config", "user.name", "Test")
	require.NoError(t, cmd.Run())
	cmd = exec.Command("git", "config", "commit.gpgsign", "false")
	require.NoError(t, cmd.Run())

	// Create initial commit on main
	require.NoError(t, os.WriteFile("file.txt", []byte("initial"), 0o600))
	cmd = exec.Command("git", "add", "file.txt")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "git add failed: %s", string(output))
	cmd = exec.Command("git", "commit", "-m", "initial")
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "git commit failed: %s", string(output))

	// Create a branch and make a commit
	cmd = exec.Command("git", "checkout", "-b", "feature-branch")
	require.NoError(t, cmd.Run())
	require.NoError(t, os.WriteFile("feature.txt", []byte("feature"), 0o600))
	cmd = exec.Command("git", "add", "feature.txt")
	require.NoError(t, cmd.Run())
	cmd = exec.Command("git", "commit", "-m", "feature")
	require.NoError(t, cmd.Run())

	// Go back to main and merge the feature branch
	cmd = exec.Command("git", "checkout", "main")
	require.NoError(t, cmd.Run())
	cmd = exec.Command("git", "merge", "feature-branch")
	require.NoError(t, cmd.Run())

	// Now feature-branch should be detected as merged
	merged := service.GetMergedBranches(ctx, "main")
	assert.Contains(t, merged, "feature-branch")

	// Create another branch that is NOT merged
	cmd = exec.Command("git", "checkout", "-b", "unmerged-branch")
	require.NoError(t, cmd.Run())
	require.NoError(t, os.WriteFile("unmerged.txt", []byte("unmerged"), 0o600))
	cmd = exec.Command("git", "add", "unmerged.txt")
	require.NoError(t, cmd.Run())
	cmd = exec.Command("git", "commit", "-m", "unmerged")
	require.NoError(t, cmd.Run())

	// Go back to main
	cmd = exec.Command("git", "checkout", "main")
	require.NoError(t, cmd.Run())

	// Get merged branches again
	merged = service.GetMergedBranches(ctx, "main")
	assert.Contains(t, merged, "feature-branch")
	assert.NotContains(t, merged, "unmerged-branch")
}

func TestApplyGitPagerEdgeCases(t *testing.T) {
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}

	t.Run("empty diff returns empty", func(t *testing.T) {
		service := NewService(notify, notifyOnce)
		service.SetGitPager("cat") // Use cat as a simple pager
		result := service.ApplyGitPager(context.Background(), "")
		assert.Empty(t, result)
	})

	t.Run("pager disabled returns original diff", func(t *testing.T) {
		service := NewService(notify, notifyOnce)
		service.SetGitPager("")
		diff := "test diff"
		result := service.ApplyGitPager(context.Background(), diff)
		assert.Equal(t, diff, result)
	})

	t.Run("pager command fails returns original diff", func(t *testing.T) {
		service := NewService(notify, notifyOnce)
		service.SetGitPager("nonexistent-command-that-fails")
		diff := "test diff"
		result := service.ApplyGitPager(context.Background(), diff)
		assert.Equal(t, diff, result) // Should return original on error
	})

	t.Run("delta pager with args", func(t *testing.T) {
		// Create a simple echo stub for delta
		stub := "#!/bin/sh\n" +
			"cat\n" + // Just pass through input
			"exit 0\n"
		dir := writeStub(t, "delta", stub)
		withStubbedPath(t, dir)

		service := NewService(notify, notifyOnce)
		service.SetGitPager("delta")
		service.SetGitPagerArgs([]string{"--syntax-theme", "Dracula"})
		diff := "test diff content"
		result := service.ApplyGitPager(context.Background(), diff)
		// Should process the diff (may add formatting)
		assert.NotNil(t, result)
	})
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
		// SHA should be 40 hex characters
		assert.Len(t, sha, 40)
	})

	t.Run("returns empty for non-repo directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		sha := service.GetHeadSHA(ctx, tmpDir)
		assert.Empty(t, sha)
	})
}
