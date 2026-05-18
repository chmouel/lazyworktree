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
// - Symlinks non-empty editor configurations (.vscode, .idea, .cursor, .claude/settings.local.json)
// - Ensures a tmp/ directory exists in the new worktree
// - Automatically runs direnv allow if a .envrc file is present
// statusFunc is used to get git status for detecting untracked/ignored files.
func LinkTopSymlinks(ctx context.Context, mainPath, worktreePath string, statusFunc func(context.Context, string) string) error {
	if mainPath == "" || worktreePath == "" {
		return fmt.Errorf("missing paths for link_topsymlinks")
	}

	status := statusFunc(ctx, mainPath)
	for _, rel := range linkableStatusPaths(status) {
		// Only symlink top-level items, skip nested paths
		if strings.Contains(rel, "/") {
			continue
		}
		if err := symlinkPath(mainPath, worktreePath, rel); err != nil {
			return fmt.Errorf("failed to symlink %s: %w", rel, err)
		}
	}

	for _, name := range []string{".vscode", ".idea", ".cursor"} {
		src := filepath.Join(mainPath, name)

		// Check if directory exists
		info, err := os.Stat(src)
		if err != nil {
			continue // Directory doesn't exist, skip
		}

		// Skip if it's not a directory
		if !info.IsDir() {
			continue
		}

		// Skip empty directories
		isEmpty, err := isEmptyDir(src)
		if err != nil {
			continue // Can't read directory, skip
		}
		if isEmpty {
			continue
		}

		// Directory exists and is not empty, symlink it
		if err := symlinkPath(mainPath, worktreePath, name); err != nil {
			return fmt.Errorf("failed to symlink %s: %w", name, err)
		}
	}

	// Special handling for .claude: create directory and symlink settings file
	if err := createClaudeDirectory(mainPath, worktreePath); err != nil {
		return fmt.Errorf("failed to setup .claude directory: %w", err)
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

func linkableStatusPaths(status string) []string {
	records := strings.Split(status, "\n")
	if strings.ContainsRune(status, '\x00') {
		records = strings.Split(strings.TrimRight(status, "\x00"), "\x00")
	}

	paths := make([]string, 0, len(records))
	for _, record := range records {
		record = strings.TrimSuffix(record, "\r")
		if len(record) < 4 {
			continue
		}
		if !strings.HasPrefix(record, "?? ") && !strings.HasPrefix(record, "!! ") {
			continue
		}
		paths = append(paths, record[3:])
	}
	return paths
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

func createClaudeDirectory(mainPath, worktreePath string) error {
	// Check if settings.local.json exists in main worktree
	settingsPath := filepath.Join(mainPath, ".claude", "settings.local.json")
	if _, err := os.Stat(settingsPath); err != nil {
		// settings.local.json doesn't exist, skip creating .claude directory
		return nil
	}

	// Create .claude directory in new worktree (not a symlink)
	claudeDir := filepath.Join(worktreePath, ".claude")
	if err := os.MkdirAll(claudeDir, 0o750); err != nil {
		return fmt.Errorf("failed to create .claude directory: %w", err)
	}

	// Symlink only settings.local.json
	if err := symlinkPath(mainPath, worktreePath, ".claude/settings.local.json"); err != nil {
		return fmt.Errorf("failed to symlink settings.local.json: %w", err)
	}

	return nil
}

func isEmptyDir(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}
