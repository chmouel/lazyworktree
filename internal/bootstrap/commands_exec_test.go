package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestShellInvocationForExec(t *testing.T) {
	t.Run("zsh uses login interactive mode", func(t *testing.T) {
		t.Setenv("SHELL", "/bin/zsh")
		shellPath, args := shellInvocationForExec("echo hello")
		if shellPath != "/bin/zsh" {
			t.Fatalf("shell path = %q, want /bin/zsh", shellPath)
		}
		if len(args) != 2 || args[0] != "-ilc" || args[1] != "echo hello" {
			t.Fatalf("args = %v, want [-ilc echo hello]", args)
		}
	})

	t.Run("bash uses interactive mode", func(t *testing.T) {
		t.Setenv("SHELL", "/bin/bash")
		shellPath, args := shellInvocationForExec("echo hello")
		if shellPath != "/bin/bash" {
			t.Fatalf("shell path = %q, want /bin/bash", shellPath)
		}
		if len(args) != 2 || args[0] != "-ic" || args[1] != "echo hello" {
			t.Fatalf("args = %v, want [-ic echo hello]", args)
		}
	})

	t.Run("fallback shell uses login command mode", func(t *testing.T) {
		t.Setenv("SHELL", "")
		shellPath, args := shellInvocationForExec("echo hello")
		if shellPath != "bash" {
			t.Fatalf("shell path = %q, want bash", shellPath)
		}
		if len(args) != 2 || args[0] != "-lc" || args[1] != "echo hello" {
			t.Fatalf("args = %v, want [-lc echo hello]", args)
		}
	})
}

func TestResolveCreateExecCWD(t *testing.T) {
	t.Run("uses created worktree path when workspace exists", func(t *testing.T) {
		worktreePath := filepath.Join(t.TempDir(), "feature")
		if err := os.MkdirAll(worktreePath, 0o750); err != nil {
			t.Fatalf("failed to create worktree path: %v", err)
		}

		cwd, err := resolveCreateExecCWD(worktreePath, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cwd != worktreePath {
			t.Fatalf("cwd = %q, want %q", cwd, worktreePath)
		}
	})

	t.Run("uses current working directory for no-workspace mode", func(t *testing.T) {
		tmpDir := t.TempDir()
		oldWD, err := os.Getwd()
		if err != nil {
			t.Fatalf("failed to get current dir: %v", err)
		}
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("failed to change dir: %v", err)
		}
		t.Cleanup(func() {
			_ = os.Chdir(oldWD)
		})

		cwd, err := resolveCreateExecCWD("ignored", true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		gotInfo, err := os.Stat(cwd)
		if err != nil {
			t.Fatalf("failed to stat cwd %q: %v", cwd, err)
		}
		wantInfo, err := os.Stat(tmpDir)
		if err != nil {
			t.Fatalf("failed to stat tmpDir %q: %v", tmpDir, err)
		}
		if !os.SameFile(gotInfo, wantInfo) {
			t.Fatalf("cwd = %q, want same directory as %q", cwd, tmpDir)
		}
	})
}
