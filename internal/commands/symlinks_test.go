package commands

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSymlinkPath(t *testing.T) {
	t.Run("source does not exist", func(t *testing.T) {
		mainDir := t.TempDir()
		worktreeDir := t.TempDir()

		err := symlinkPath(mainDir, worktreeDir, "nonexistent.txt")
		require.NoError(t, err) // Should return nil without error
	})

	t.Run("create symlink for file", func(t *testing.T) {
		mainDir := t.TempDir()
		worktreeDir := t.TempDir()

		// Create source file in main
		srcFile := filepath.Join(mainDir, "test.txt")
		err := os.WriteFile(srcFile, []byte("test content"), 0o600)
		require.NoError(t, err)

		// Create symlink
		err = symlinkPath(mainDir, worktreeDir, "test.txt")
		require.NoError(t, err)

		// Verify symlink exists and points to correct target
		dstFile := filepath.Join(worktreeDir, "test.txt")
		linkTarget, err := os.Readlink(dstFile)
		require.NoError(t, err)
		assert.Equal(t, srcFile, linkTarget)
	})

	t.Run("create symlink for directory", func(t *testing.T) {
		mainDir := t.TempDir()
		worktreeDir := t.TempDir()

		// Create source directory in main
		srcDir := filepath.Join(mainDir, "testdir")
		err := os.MkdirAll(srcDir, 0o750)
		require.NoError(t, err)

		// Create symlink
		err = symlinkPath(mainDir, worktreeDir, "testdir")
		require.NoError(t, err)

		// Verify symlink exists
		dstDir := filepath.Join(worktreeDir, "testdir")
		linkTarget, err := os.Readlink(dstDir)
		require.NoError(t, err)
		assert.Equal(t, srcDir, linkTarget)
	})

	t.Run("symlink already exists", func(t *testing.T) {
		mainDir := t.TempDir()
		worktreeDir := t.TempDir()

		// Create source file
		srcFile := filepath.Join(mainDir, "test.txt")
		err := os.WriteFile(srcFile, []byte("test"), 0o600)
		require.NoError(t, err)

		// Create symlink first time
		err = symlinkPath(mainDir, worktreeDir, "test.txt")
		require.NoError(t, err)

		// Try creating symlink again - should return nil without error
		err = symlinkPath(mainDir, worktreeDir, "test.txt")
		require.NoError(t, err)
	})

	t.Run("create symlink with nested path", func(t *testing.T) {
		mainDir := t.TempDir()
		worktreeDir := t.TempDir()

		// Create source file in nested directory
		srcFile := filepath.Join(mainDir, "nested", "dir", "test.txt")
		err := os.MkdirAll(filepath.Dir(srcFile), 0o750)
		require.NoError(t, err)
		err = os.WriteFile(srcFile, []byte("nested content"), 0o600)
		require.NoError(t, err)

		// Create symlink
		err = symlinkPath(mainDir, worktreeDir, "nested/dir/test.txt")
		require.NoError(t, err)

		// Verify symlink exists
		dstFile := filepath.Join(worktreeDir, "nested", "dir", "test.txt")
		linkTarget, err := os.Readlink(dstFile)
		require.NoError(t, err)
		assert.Equal(t, srcFile, linkTarget)

		// Verify parent directory was created
		dstDir := filepath.Join(worktreeDir, "nested", "dir")
		info, err := os.Stat(dstDir)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})
}

func TestLinkTopSymlinks(t *testing.T) {
	t.Run("missing main path", func(t *testing.T) {
		worktreeDir := t.TempDir()
		statusFunc := func(_ context.Context, _ string) string {
			return ""
		}

		err := LinkTopSymlinks(context.Background(), "", worktreeDir, statusFunc)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing paths")
	})

	t.Run("missing worktree path", func(t *testing.T) {
		mainDir := t.TempDir()
		statusFunc := func(_ context.Context, _ string) string {
			return ""
		}

		err := LinkTopSymlinks(context.Background(), mainDir, "", statusFunc)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing paths")
	})

	t.Run("symlink untracked files", func(t *testing.T) {
		mainDir := t.TempDir()
		worktreeDir := t.TempDir()

		// Create untracked files in main
		file1 := filepath.Join(mainDir, "untracked1.txt")
		err := os.WriteFile(file1, []byte("content1"), 0o600)
		require.NoError(t, err)

		file2 := filepath.Join(mainDir, "untracked2.txt")
		err = os.WriteFile(file2, []byte("content2"), 0o600)
		require.NoError(t, err)

		statusFunc := func(_ context.Context, _ string) string {
			return "?? untracked1.txt\n?? untracked2.txt\n M tracked.txt"
		}

		err = LinkTopSymlinks(context.Background(), mainDir, worktreeDir, statusFunc)
		require.NoError(t, err)

		// Verify symlinks were created
		link1 := filepath.Join(worktreeDir, "untracked1.txt")
		target1, err := os.Readlink(link1)
		require.NoError(t, err)
		assert.Equal(t, file1, target1)

		link2 := filepath.Join(worktreeDir, "untracked2.txt")
		target2, err := os.Readlink(link2)
		require.NoError(t, err)
		assert.Equal(t, file2, target2)
	})

	t.Run("symlink untracked files with spaces from porcelain z output", func(t *testing.T) {
		mainDir := t.TempDir()
		worktreeDir := t.TempDir()

		fileWithSpaces := filepath.Join(mainDir, "my file.txt")
		err := os.WriteFile(fileWithSpaces, []byte("content"), 0o600)
		require.NoError(t, err)

		statusFunc := func(_ context.Context, _ string) string {
			return "?? my file.txt\x00"
		}

		err = LinkTopSymlinks(context.Background(), mainDir, worktreeDir, statusFunc)
		require.NoError(t, err)

		link := filepath.Join(worktreeDir, "my file.txt")
		target, err := os.Readlink(link)
		require.NoError(t, err)
		assert.Equal(t, fileWithSpaces, target)
	})

	t.Run("symlink multiple porcelain z records with spaced names", func(t *testing.T) {
		mainDir := t.TempDir()
		worktreeDir := t.TempDir()

		untracked := filepath.Join(mainDir, "first file.txt")
		err := os.WriteFile(untracked, []byte("content"), 0o600)
		require.NoError(t, err)

		ignored := filepath.Join(mainDir, "ignored file.log")
		err = os.WriteFile(ignored, []byte("content"), 0o600)
		require.NoError(t, err)

		statusFunc := func(_ context.Context, _ string) string {
			return "?? first file.txt\x00!! ignored file.log\x00"
		}

		err = LinkTopSymlinks(context.Background(), mainDir, worktreeDir, statusFunc)
		require.NoError(t, err)

		untrackedLink := filepath.Join(worktreeDir, "first file.txt")
		untrackedTarget, err := os.Readlink(untrackedLink)
		require.NoError(t, err)
		assert.Equal(t, untracked, untrackedTarget)

		ignoredLink := filepath.Join(worktreeDir, "ignored file.log")
		ignoredTarget, err := os.Readlink(ignoredLink)
		require.NoError(t, err)
		assert.Equal(t, ignored, ignoredTarget)
	})

	t.Run("symlink ignored files", func(t *testing.T) {
		mainDir := t.TempDir()
		worktreeDir := t.TempDir()

		// Create ignored file in main
		ignoredFile := filepath.Join(mainDir, "ignored.log")
		err := os.WriteFile(ignoredFile, []byte("log content"), 0o600)
		require.NoError(t, err)

		statusFunc := func(_ context.Context, _ string) string {
			return "!! ignored.log"
		}

		err = LinkTopSymlinks(context.Background(), mainDir, worktreeDir, statusFunc)
		require.NoError(t, err)

		// Verify symlink was created
		link := filepath.Join(worktreeDir, "ignored.log")
		target, err := os.Readlink(link)
		require.NoError(t, err)
		assert.Equal(t, ignoredFile, target)
	})

	t.Run("symlink editor configs", func(t *testing.T) {
		mainDir := t.TempDir()
		worktreeDir := t.TempDir()

		// Create traditional editor config directories (.vscode, .idea, .cursor)
		editorDirs := []string{".vscode", ".idea", ".cursor"}
		for _, dir := range editorDirs {
			dirPath := filepath.Join(mainDir, dir)
			err := os.MkdirAll(dirPath, 0o750)
			require.NoError(t, err)

			// Add a file in each directory
			configFile := filepath.Join(dirPath, "settings.json")
			err = os.WriteFile(configFile, []byte("{}"), 0o600)
			require.NoError(t, err)
		}

		// Create .claude directory with settings.local.json
		claudeDir := filepath.Join(mainDir, ".claude")
		err := os.MkdirAll(claudeDir, 0o750)
		require.NoError(t, err)
		settingsLocalPath := filepath.Join(claudeDir, "settings.local.json")
		err = os.WriteFile(settingsLocalPath, []byte("{}"), 0o600)
		require.NoError(t, err)

		// Add another file that should NOT be symlinked
		settingsPath := filepath.Join(claudeDir, "settings.json")
		err = os.WriteFile(settingsPath, []byte("{}"), 0o600)
		require.NoError(t, err)

		statusFunc := func(_ context.Context, _ string) string {
			return ""
		}

		err = LinkTopSymlinks(context.Background(), mainDir, worktreeDir, statusFunc)
		require.NoError(t, err)

		// Verify symlinks were created for traditional editor dirs
		for _, dir := range editorDirs {
			link := filepath.Join(worktreeDir, dir)
			target, err := os.Readlink(link)
			require.NoError(t, err)
			assert.Equal(t, filepath.Join(mainDir, dir), target)
		}

		// Verify .claude directory exists and is NOT a symlink
		claudeDirWorktree := filepath.Join(worktreeDir, ".claude")
		info, err := os.Lstat(claudeDirWorktree)
		require.NoError(t, err)
		assert.True(t, info.IsDir(), ".claude should be a real directory, not a symlink")

		// Verify settings.local.json IS a symlink
		settingsLocalWorktree := filepath.Join(worktreeDir, ".claude", "settings.local.json")
		target, err := os.Readlink(settingsLocalWorktree)
		require.NoError(t, err)
		assert.Equal(t, settingsLocalPath, target)

		// Verify settings.json is NOT symlinked (remains isolated)
		settingsWorktree := filepath.Join(worktreeDir, ".claude", "settings.json")
		_, err = os.Lstat(settingsWorktree)
		require.Error(t, err, "settings.json should not exist in worktree")
	})

	t.Run("skip empty editor directories", func(t *testing.T) {
		mainDir := t.TempDir()
		worktreeDir := t.TempDir()

		// Create empty .vscode directory
		emptyVSCode := filepath.Join(mainDir, ".vscode")
		err := os.MkdirAll(emptyVSCode, 0o750)
		require.NoError(t, err)

		// Create non-empty .idea directory
		ideaDir := filepath.Join(mainDir, ".idea")
		err = os.MkdirAll(ideaDir, 0o750)
		require.NoError(t, err)
		configFile := filepath.Join(ideaDir, "workspace.xml")
		err = os.WriteFile(configFile, []byte("<xml/>"), 0o600)
		require.NoError(t, err)

		// Create empty .cursor directory
		emptyCursor := filepath.Join(mainDir, ".cursor")
		err = os.MkdirAll(emptyCursor, 0o750)
		require.NoError(t, err)

		statusFunc := func(_ context.Context, _ string) string {
			return ""
		}

		err = LinkTopSymlinks(context.Background(), mainDir, worktreeDir, statusFunc)
		require.NoError(t, err)

		// Verify empty .vscode was NOT symlinked
		vscodeLink := filepath.Join(worktreeDir, ".vscode")
		_, err = os.Lstat(vscodeLink)
		require.Error(t, err, "empty .vscode should not be symlinked")

		// Verify non-empty .idea WAS symlinked
		ideaLink := filepath.Join(worktreeDir, ".idea")
		target, err := os.Readlink(ideaLink)
		require.NoError(t, err)
		assert.Equal(t, ideaDir, target)

		// Verify empty .cursor was NOT symlinked
		cursorLink := filepath.Join(worktreeDir, ".cursor")
		_, err = os.Lstat(cursorLink)
		require.Error(t, err, "empty .cursor should not be symlinked")
	})

	t.Run("symlink editor directories with subdirectories", func(t *testing.T) {
		mainDir := t.TempDir()
		worktreeDir := t.TempDir()

		// Create .vscode with subdirectory but no files
		vscodeDir := filepath.Join(mainDir, ".vscode")
		subDir := filepath.Join(vscodeDir, "extensions")
		err := os.MkdirAll(subDir, 0o750)
		require.NoError(t, err)

		statusFunc := func(_ context.Context, _ string) string {
			return ""
		}

		err = LinkTopSymlinks(context.Background(), mainDir, worktreeDir, statusFunc)
		require.NoError(t, err)

		// Verify .vscode WAS symlinked (has subdirectory)
		vscodeLink := filepath.Join(worktreeDir, ".vscode")
		target, err := os.Readlink(vscodeLink)
		require.NoError(t, err)
		assert.Equal(t, vscodeDir, target)
	})

	t.Run("create tmp directory", func(t *testing.T) {
		mainDir := t.TempDir()
		worktreeDir := t.TempDir()

		statusFunc := func(_ context.Context, _ string) string {
			return ""
		}

		err := LinkTopSymlinks(context.Background(), mainDir, worktreeDir, statusFunc)
		require.NoError(t, err)

		// Verify tmp directory was created
		tmpDir := filepath.Join(worktreeDir, "tmp")
		info, err := os.Stat(tmpDir)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("direnv allow with .envrc", func(t *testing.T) {
		mainDir := t.TempDir()
		worktreeDir := t.TempDir()

		// Create .envrc file
		envrcPath := filepath.Join(worktreeDir, ".envrc")
		err := os.WriteFile(envrcPath, []byte("export TEST=1"), 0o600)
		require.NoError(t, err)

		statusFunc := func(_ context.Context, _ string) string {
			return ""
		}

		// This will attempt to run direnv allow, which may not exist
		// The function doesn't return error if direnv fails (best-effort)
		err = LinkTopSymlinks(context.Background(), mainDir, worktreeDir, statusFunc)
		require.NoError(t, err)
	})

	t.Run("handle empty status lines", func(t *testing.T) {
		mainDir := t.TempDir()
		worktreeDir := t.TempDir()

		statusFunc := func(_ context.Context, _ string) string {
			return "?? file.txt\n\n?? \n   \n"
		}

		// Create only the valid file
		file := filepath.Join(mainDir, "file.txt")
		err := os.WriteFile(file, []byte("content"), 0o600)
		require.NoError(t, err)

		err = LinkTopSymlinks(context.Background(), mainDir, worktreeDir, statusFunc)
		require.NoError(t, err)

		// Only valid file should have symlink
		link := filepath.Join(worktreeDir, "file.txt")
		_, err = os.Readlink(link)
		require.NoError(t, err)
	})

	t.Run("handle status with short lines", func(t *testing.T) {
		mainDir := t.TempDir()
		worktreeDir := t.TempDir()

		statusFunc := func(_ context.Context, _ string) string {
			return "?? file.txt\n??\nM\n   "
		}

		file := filepath.Join(mainDir, "file.txt")
		err := os.WriteFile(file, []byte("content"), 0o600)
		require.NoError(t, err)

		err = LinkTopSymlinks(context.Background(), mainDir, worktreeDir, statusFunc)
		require.NoError(t, err)

		link := filepath.Join(worktreeDir, "file.txt")
		_, err = os.Readlink(link)
		require.NoError(t, err)
	})

	t.Run("skip non-untracked files", func(t *testing.T) {
		mainDir := t.TempDir()
		worktreeDir := t.TempDir()

		// Create files
		tracked := filepath.Join(mainDir, "tracked.txt")
		err := os.WriteFile(tracked, []byte("tracked"), 0o600)
		require.NoError(t, err)

		modified := filepath.Join(mainDir, "modified.txt")
		err = os.WriteFile(modified, []byte("modified"), 0o600)
		require.NoError(t, err)

		statusFunc := func(_ context.Context, _ string) string {
			return " M tracked.txt\n M modified.txt\nA  added.txt"
		}

		err = LinkTopSymlinks(context.Background(), mainDir, worktreeDir, statusFunc)
		require.NoError(t, err)

		// Verify no symlinks created for tracked/modified files
		_, err = os.Readlink(filepath.Join(worktreeDir, "tracked.txt"))
		require.Error(t, err) // Should not exist

		_, err = os.Readlink(filepath.Join(worktreeDir, "modified.txt"))
		require.Error(t, err) // Should not exist
	})

	t.Run("skip nested untracked files", func(t *testing.T) {
		mainDir := t.TempDir()
		worktreeDir := t.TempDir()

		// Create nested untracked directory
		nestedDir := filepath.Join(mainDir, "docs", "resources")
		err := os.MkdirAll(nestedDir, 0o750)
		require.NoError(t, err)

		// Create top-level untracked directory
		topDir := filepath.Join(mainDir, "bin")
		err = os.MkdirAll(topDir, 0o750)
		require.NoError(t, err)

		statusFunc := func(_ context.Context, _ string) string {
			return "?? bin\n?? docs/resources"
		}

		err = LinkTopSymlinks(context.Background(), mainDir, worktreeDir, statusFunc)
		require.NoError(t, err)

		// Top-level should be symlinked
		link := filepath.Join(worktreeDir, "bin")
		_, err = os.Readlink(link)
		require.NoError(t, err, "top-level 'bin' should be symlinked")

		// Nested should NOT be symlinked
		nestedLink := filepath.Join(worktreeDir, "docs", "resources")
		_, err = os.Stat(nestedLink)
		require.Error(t, err, "nested 'docs/resources' should not be symlinked")
	})

	t.Run("claude directory not created when settings.local.json missing", func(t *testing.T) {
		mainDir := t.TempDir()
		worktreeDir := t.TempDir()

		statusFunc := func(_ context.Context, _ string) string {
			return ""
		}

		err := LinkTopSymlinks(context.Background(), mainDir, worktreeDir, statusFunc)
		require.NoError(t, err)

		// .claude directory should not exist in worktree
		claudeDir := filepath.Join(worktreeDir, ".claude")
		_, err = os.Stat(claudeDir)
		require.Error(t, err, ".claude directory should not be created when settings.local.json doesn't exist")
	})

	t.Run("claude directory not created when .claude dir doesn't exist", func(t *testing.T) {
		mainDir := t.TempDir()
		worktreeDir := t.TempDir()

		statusFunc := func(_ context.Context, _ string) string {
			return ""
		}

		err := LinkTopSymlinks(context.Background(), mainDir, worktreeDir, statusFunc)
		require.NoError(t, err)

		// .claude directory should not exist in worktree
		claudeDir := filepath.Join(worktreeDir, ".claude")
		_, err = os.Stat(claudeDir)
		require.Error(t, err, ".claude directory should not be created when .claude doesn't exist in main")
	})

	t.Run("claude directory not created when .claude exists but no settings.local.json", func(t *testing.T) {
		mainDir := t.TempDir()
		worktreeDir := t.TempDir()

		// Create .claude directory without settings.local.json
		claudeDir := filepath.Join(mainDir, ".claude")
		err := os.MkdirAll(claudeDir, 0o750)
		require.NoError(t, err)

		// Add only settings.json (not settings.local.json)
		settingsPath := filepath.Join(claudeDir, "settings.json")
		err = os.WriteFile(settingsPath, []byte("{}"), 0o600)
		require.NoError(t, err)

		statusFunc := func(_ context.Context, _ string) string {
			return ""
		}

		err = LinkTopSymlinks(context.Background(), mainDir, worktreeDir, statusFunc)
		require.NoError(t, err)

		// .claude directory should not exist in worktree
		claudeDirWorktree := filepath.Join(worktreeDir, ".claude")
		_, err = os.Stat(claudeDirWorktree)
		require.Error(t, err, ".claude directory should not be created when settings.local.json doesn't exist")
	})

	t.Run("claude directory created and settings.local.json symlinked when it exists", func(t *testing.T) {
		mainDir := t.TempDir()
		worktreeDir := t.TempDir()

		// Create .claude directory with settings.local.json
		claudeDir := filepath.Join(mainDir, ".claude")
		err := os.MkdirAll(claudeDir, 0o750)
		require.NoError(t, err)

		settingsLocalPath := filepath.Join(claudeDir, "settings.local.json")
		err = os.WriteFile(settingsLocalPath, []byte(`{"key": "value"}`), 0o600)
		require.NoError(t, err)

		statusFunc := func(_ context.Context, _ string) string {
			return ""
		}

		err = LinkTopSymlinks(context.Background(), mainDir, worktreeDir, statusFunc)
		require.NoError(t, err)

		// Verify .claude directory exists and is a real directory
		claudeDirWorktree := filepath.Join(worktreeDir, ".claude")
		info, err := os.Lstat(claudeDirWorktree)
		require.NoError(t, err)
		assert.True(t, info.IsDir(), ".claude should be a real directory")

		// Verify settings.local.json is a symlink
		settingsLocalWorktree := filepath.Join(worktreeDir, ".claude", "settings.local.json")
		target, err := os.Readlink(settingsLocalWorktree)
		require.NoError(t, err)
		assert.Equal(t, settingsLocalPath, target)
	})

	t.Run("claude directory handling is idempotent", func(t *testing.T) {
		mainDir := t.TempDir()
		worktreeDir := t.TempDir()

		// Create .claude directory with settings.local.json
		claudeDir := filepath.Join(mainDir, ".claude")
		err := os.MkdirAll(claudeDir, 0o750)
		require.NoError(t, err)

		settingsLocalPath := filepath.Join(claudeDir, "settings.local.json")
		err = os.WriteFile(settingsLocalPath, []byte("{}"), 0o600)
		require.NoError(t, err)

		statusFunc := func(_ context.Context, _ string) string {
			return ""
		}

		// Run LinkTopSymlinks twice
		err = LinkTopSymlinks(context.Background(), mainDir, worktreeDir, statusFunc)
		require.NoError(t, err)

		err = LinkTopSymlinks(context.Background(), mainDir, worktreeDir, statusFunc)
		require.NoError(t, err)

		// Verify .claude directory still exists and settings.local.json is still a symlink
		claudeDirWorktree := filepath.Join(worktreeDir, ".claude")
		info, err := os.Lstat(claudeDirWorktree)
		require.NoError(t, err)
		assert.True(t, info.IsDir())

		settingsLocalWorktree := filepath.Join(worktreeDir, ".claude", "settings.local.json")
		target, err := os.Readlink(settingsLocalWorktree)
		require.NoError(t, err)
		assert.Equal(t, settingsLocalPath, target)
	})
}
