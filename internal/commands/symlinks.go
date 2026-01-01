// Package commands provides utility helpers for workspace-related shell commands.
package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// LinkTopSymlinks creates symlinks for untracked/ignored files and editor configs from main to target worktree.
// This is a built-in automation command that:
// - Symlinks all untracked and ignored files from the root of the main worktree (excluding subdirectories)
// - Symlinks common editor configurations (.vscode, .idea, .cursor, .claude)
// - Ensures a tmp/ directory exists in the new worktree
// - Automatically runs direnv allow if a .envrc file is present
// statusFunc is used to get git status for detecting untracked/ignored files.
func LinkTopSymlinks(ctx context.Context, mainPath, worktreePath string, statusFunc func(context.Context, string) string) error {
	if mainPath == "" || worktreePath == "" {
		return fmt.Errorf("missing paths for link_topsymlinks")
	}

	status := statusFunc(ctx, mainPath)
	for _, line := range strings.Split(status, "\n") {
		if len(line) < 4 {
			continue
		}
		if !strings.HasPrefix(line, "?? ") && !strings.HasPrefix(line, "!! ") {
			continue
		}
		rel := strings.TrimSpace(line[3:])
		if rel == "" {
			continue
		}
		if err := symlinkPath(mainPath, worktreePath, rel); err != nil {
			return fmt.Errorf("failed to symlink %s: %w", rel, err)
		}
	}

	for _, name := range []string{".vscode", ".idea", ".cursor", ".claude"} {
		if err := symlinkPath(mainPath, worktreePath, name); err != nil {
			return fmt.Errorf("failed to symlink %s: %w", name, err)
		}
	}

	if err := os.MkdirAll(filepath.Join(worktreePath, "tmp"), 0o750); err != nil {
		return fmt.Errorf("failed to create tmp directory: %w", err)
	}

	envrcPath := filepath.Join(worktreePath, ".envrc")
	if _, err := os.Stat(envrcPath); err == nil {
		cmd := exec.CommandContext(ctx, "direnv", "allow")
		cmd.Dir = worktreePath
		_ = cmd.Run() // best-effort
	}

	return nil
}

func symlinkPath(mainPath, worktreePath, rel string) error {
	src := filepath.Join(mainPath, rel)
	if _, err := os.Stat(src); err != nil {
		return nil
	}

	dst := filepath.Join(worktreePath, rel)
	if _, err := os.Lstat(dst); err == nil {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", dst, err)
	}

	_ = os.Remove(dst)
	if err := os.Symlink(src, dst); err != nil {
		return fmt.Errorf("failed to create symlink %s -> %s: %w", dst, src, err)
	}
	return nil
}
