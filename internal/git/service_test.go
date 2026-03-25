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
