package git

import (
	"context"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewService(t *testing.T) {
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

func TestUseDelta(t *testing.T) {
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}

	service := NewService(notify, notifyOnce)

	// UseDelta should return a boolean
	useDelta := service.UseDelta()
	assert.IsType(t, true, useDelta)
}

func TestSetDeltaPath(t *testing.T) {
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}

	service := NewService(notify, notifyOnce)

	t.Run("empty path disables delta", func(t *testing.T) {
		service.SetDeltaPath("")
		assert.False(t, service.UseDelta())
		assert.Empty(t, service.deltaPath)
	})

	t.Run("custom delta path", func(t *testing.T) {
		service.SetDeltaPath("/custom/path/to/delta")
		assert.Equal(t, "/custom/path/to/delta", service.deltaPath)
	})

	t.Run("whitespace trimmed from path", func(t *testing.T) {
		service.SetDeltaPath("  delta  ")
		assert.Equal(t, "delta", service.deltaPath)
	})
}

func TestSetDebugLogger(t *testing.T) {
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}

	service := NewService(notify, notifyOnce)
	logger := log.New(io.Discard, "", 0)
	service.SetDebugLogger(logger)

	assert.Equal(t, logger, service.debugLogger)
}

func TestSetDeltaArgs(t *testing.T) {
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}

	service := NewService(notify, notifyOnce)

	service.SetDeltaArgs([]string{"--color-only"})
	assert.Equal(t, []string{"--color-only"}, service.deltaArgs)

	args := []string{"--side-by-side"}
	service.SetDeltaArgs(args)
	args[0] = "--changed"
	assert.Equal(t, []string{"--side-by-side"}, service.deltaArgs)

	service.SetDeltaArgs(nil)
	assert.Nil(t, service.deltaArgs)
}

func TestApplyDelta(t *testing.T) {
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}

	service := NewService(notify, notifyOnce)

	t.Run("empty diff returns empty", func(t *testing.T) {
		result := service.ApplyDelta(context.Background(), "")
		assert.Empty(t, result)
	})

	t.Run("diff without delta available", func(t *testing.T) {
		// Temporarily disable delta
		origUseDelta := service.useDelta
		service.useDelta = false
		defer func() { service.useDelta = origUseDelta }()

		diff := "diff --git a/file.txt b/file.txt\n"
		result := service.ApplyDelta(context.Background(), diff)
		assert.Equal(t, diff, result)
	})

	t.Run("diff with delta available", func(t *testing.T) {
		diff := "diff --git a/file.txt b/file.txt\n+added line\n"

		result := service.ApplyDelta(context.Background(), diff)
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

func TestRenameWorktree(t *testing.T) {
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}

	service := NewService(notify, notifyOnce)
	ctx := context.Background()

	t.Run("rename with temporary directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		oldPath := filepath.Join(tmpDir, "old")
		newPath := filepath.Join(tmpDir, "new")

		// Create old directory
		err := os.MkdirAll(oldPath, 0o750)
		require.NoError(t, err)

		// Create a test file in old directory
		testFile := filepath.Join(oldPath, "test.txt")
		err = os.WriteFile(testFile, []byte("test"), 0o600)
		require.NoError(t, err)

		// Rename (this is essentially just a directory move, not a git worktree operation)
		// Note: This will likely fail if git commands are involved, so we're testing basic logic
		ok := service.RenameWorktree(ctx, oldPath, newPath, "old-branch", "new-branch")

		// Even if it returns false due to git errors, we're just testing the function runs
		assert.IsType(t, true, ok)
	})
}

func TestExecuteCommands(t *testing.T) {
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
}

func TestRunGit(t *testing.T) {
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

func TestFetchPRMap(t *testing.T) {
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}

	service := NewService(notify, notifyOnce)
	ctx := context.Background()

	t.Run("fetch PR map without git repository", func(t *testing.T) {
		// This test just verifies the function doesn't panic
		// Behavior varies by git environment (may return error or empty map)
		prMap, err := service.FetchPRMap(ctx)

		// Function should not panic and should return valid types
		// Either error or map (which can be nil or empty)
		if err == nil {
			// prMap can be nil or a valid map - both are acceptable
			if prMap != nil {
				assert.IsType(t, map[string]*models.PRInfo{}, prMap)
			}
		}
	})
}

func TestFetchPRForWorktree(t *testing.T) {
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}

	service := NewService(notify, notifyOnce)
	ctx := context.Background()

	t.Run("fetch PR for non-existent worktree returns nil", func(t *testing.T) {
		// This test verifies the function doesn't panic on invalid path
		pr := service.FetchPRForWorktree(ctx, "/non/existent/path")
		assert.Nil(t, pr)
	})

	t.Run("fetch PR for worktree without PR returns nil", func(t *testing.T) {
		// Create a temp directory that's not a git repo
		tmpDir := t.TempDir()
		pr := service.FetchPRForWorktree(ctx, tmpDir)
		assert.Nil(t, pr)
	})
}

func TestGithubBucketToConclusion(t *testing.T) {
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}
	service := NewService(notify, notifyOnce)

	tests := []struct {
		bucket   string
		expected string
	}{
		{"pass", ciSuccess},
		{"PASS", ciSuccess},
		{"fail", ciFailure},
		{"FAIL", ciFailure},
		{"skipping", ciSkipped},
		{"SKIPPING", ciSkipped},
		{"cancel", ciCancelled},
		{"CANCEL", ciCancelled},
		{"pending", ciPending},
		{"PENDING", ciPending},
		{"unknown", "unknown"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.bucket, func(t *testing.T) {
			result := service.githubBucketToConclusion(tt.bucket)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGitlabStatusToConclusion(t *testing.T) {
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}
	service := NewService(notify, notifyOnce)

	tests := []struct {
		status   string
		expected string
	}{
		{"success", ciSuccess},
		{"SUCCESS", ciSuccess},
		{"passed", ciSuccess},
		{"PASSED", ciSuccess},
		{"failed", ciFailure},
		{"FAILED", ciFailure},
		{"canceled", ciCancelled},
		{"cancelled", ciCancelled},
		{"skipped", ciSkipped},
		{"SKIPPED", ciSkipped},
		{"running", ciPending},
		{"pending", ciPending},
		{"created", ciPending},
		{"waiting_for_resource", ciPending},
		{"preparing", ciPending},
		{"unknown", "unknown"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			result := service.gitlabStatusToConclusion(tt.status)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFetchCIStatus(t *testing.T) {
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}
	service := NewService(notify, notifyOnce)
	ctx := context.Background()

	t.Run("fetch CI status without git repository", func(t *testing.T) {
		// This test just verifies the function doesn't panic
		checks, err := service.FetchCIStatus(ctx, 1, "main")

		// Function should not panic
		// Either returns nil checks (unknown host) or error
		if err == nil {
			// checks can be nil - acceptable for unknown host
			if checks != nil {
				assert.IsType(t, []*models.CICheck{}, checks)
			}
		}
	})
}

func TestFetchAllOpenPRs(t *testing.T) {
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}

	service := NewService(notify, notifyOnce)
	ctx := context.Background()

	t.Run("fetch open PRs without git repository", func(t *testing.T) {
		// This will likely fail or return empty, but should not panic
		prs, err := service.FetchAllOpenPRs(ctx)

		// Should return a slice (even if empty) or an error
		if err == nil {
			assert.IsType(t, []*models.PRInfo{}, prs)
		} else {
			// Error is acceptable if gh/glab not available or not in a repo
			assert.Error(t, err)
		}
	})
}

func TestCreateWorktreeFromPR(t *testing.T) {
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}

	service := NewService(notify, notifyOnce)
	ctx := context.Background()

	t.Run("create worktree from PR with temporary directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		targetPath := filepath.Join(tmpDir, "test-worktree")

		// This will likely fail due to missing git repo/PR, but tests the function structure
		ok := service.CreateWorktreeFromPR(ctx, 123, "feature-branch", "local-branch", targetPath)

		// Should return a boolean (even if false due to git errors)
		assert.IsType(t, true, ok)
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
			if got := service.detectHost(ctx); got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestFetchGitHubCIParsesOutput(t *testing.T) {
	ctx := context.Background()
	writeStubCommand(t, "gh", "GH_OUTPUT")
	t.Setenv("GH_OUTPUT", `[{"name":"build","state":"completed","bucket":"pass"}]`)

	service := NewService(func(string, string) {}, func(string, string, string) {})
	checks, err := service.fetchGitHubCI(ctx, 1)
	require.NoError(t, err)
	require.Len(t, checks, 1)
	assert.Equal(t, "build", checks[0].Name)
	assert.Equal(t, "completed", checks[0].Status)
	assert.Equal(t, ciSuccess, checks[0].Conclusion)
}

func TestFetchGitHubCIInvalidJSON(t *testing.T) {
	ctx := context.Background()
	writeStubCommand(t, "gh", "GH_OUTPUT")
	t.Setenv("GH_OUTPUT", "not-json")

	service := NewService(func(string, string) {}, func(string, string, string) {})
	_, err := service.fetchGitHubCI(ctx, 1)
	require.Error(t, err)
}

func TestFetchGitLabCIParsesPipeline(t *testing.T) {
	ctx := context.Background()
	writeStubCommand(t, "glab", "GLAB_OUTPUT")
	t.Setenv("GLAB_OUTPUT", `{"jobs":[{"name":"build","status":"success"},{"name":"lint","status":"failed"}]}`)

	service := NewService(func(string, string) {}, func(string, string, string) {})
	checks, err := service.fetchGitLabCI(ctx, "main")
	require.NoError(t, err)
	require.Len(t, checks, 2)
	assert.Equal(t, "build", checks[0].Name)
	assert.Equal(t, ciSuccess, checks[0].Conclusion)
	assert.Equal(t, "lint", checks[1].Name)
	assert.Equal(t, ciFailure, checks[1].Conclusion)
}

func TestFetchGitLabCIParsesJobArray(t *testing.T) {
	ctx := context.Background()
	writeStubCommand(t, "glab", "GLAB_OUTPUT")
	t.Setenv("GLAB_OUTPUT", `[{"name":"unit","status":"running"}]`)

	service := NewService(func(string, string) {}, func(string, string, string) {})
	checks, err := service.fetchGitLabCI(ctx, "main")
	require.NoError(t, err)
	require.Len(t, checks, 1)
	assert.Equal(t, "unit", checks[0].Name)
	assert.Equal(t, ciPending, checks[0].Conclusion)
}

func TestFetchGitLabCIInvalidJSON(t *testing.T) {
	ctx := context.Background()
	writeStubCommand(t, "glab", "GLAB_OUTPUT")
	t.Setenv("GLAB_OUTPUT", "not-json")

	service := NewService(func(string, string) {}, func(string, string, string) {})
	_, err := service.fetchGitLabCI(ctx, "main")
	require.Error(t, err)
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
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
