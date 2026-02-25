package app

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

func TestAuthorInitials(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{name: "Christian B", want: "CB"},
		{name: "github-actions", want: "gi"},
		{name: "John Doe", want: "JD"},
		{name: "Single", want: "Si"},
		{name: "A", want: "A"},
		{name: "", want: ""},
	}

	for _, tt := range tests {
		if got := authorInitials(tt.name); got != tt.want {
			t.Fatalf("authorInitials(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestExpandWithEnv(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		env      map[string]string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			env:      map[string]string{"FOO": "bar"},
			expected: "",
		},
		{
			name:     "no variables",
			input:    "plain text",
			env:      map[string]string{},
			expected: "plain text",
		},
		{
			name:     "single variable",
			input:    "$FOO",
			env:      map[string]string{"FOO": "bar"},
			expected: "bar",
		},
		{
			name:     "variable with braces",
			input:    "${FOO}",
			env:      map[string]string{"FOO": "bar"},
			expected: "bar",
		},
		{
			name:     "multiple variables",
			input:    "$FOO-$BAR",
			env:      map[string]string{"FOO": "hello", "BAR": "world"},
			expected: "hello-world",
		},
		{
			name:     "REPO_NAME and WORKTREE_NAME",
			input:    "${REPO_NAME}_wt_$WORKTREE_NAME",
			env:      map[string]string{"REPO_NAME": "myrepo", "WORKTREE_NAME": "feature"},
			expected: "myrepo_wt_feature",
		},
		{
			name:     "missing variable uses system env",
			input:    "$HOME",
			env:      map[string]string{},
			expected: os.Getenv("HOME"),
		},
		{
			name:     "undefined variable becomes empty",
			input:    "$UNDEFINED_VAR",
			env:      map[string]string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandWithEnv(tt.input, tt.env)
			if result != tt.expected {
				t.Errorf("expandWithEnv(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestEnvMapToList(t *testing.T) {
	env := map[string]string{
		"A": "1",
		"B": "2",
	}

	list := envMapToList(env)
	if len(list) != 2 {
		t.Fatalf("expected two env entries, got %d", len(list))
	}

	values := map[string]bool{}
	for _, entry := range list {
		values[entry] = true
	}

	if !values["A=1"] || !values["B=2"] {
		t.Fatalf("unexpected env list: %v", list)
	}
}

func TestFilterWorktreeEnvVars(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name: "filters worktree vars",
			input: []string{
				"PATH=/usr/bin",
				"WORKTREE_PATH=/tmp/wt",
				"HOME=/home/user",
				"MAIN_WORKTREE_PATH=/main",
				"WORKTREE_BRANCH=feature",
				"EDITOR=vim",
			},
			expected: []string{
				"PATH=/usr/bin",
				"HOME=/home/user",
				"EDITOR=vim",
			},
		},
		{
			name:     "handles empty input",
			input:    []string{},
			expected: []string{},
		},
		{
			name: "handles no worktree vars",
			input: []string{
				"PATH=/usr/bin",
				"HOME=/home/user",
			},
			expected: []string{
				"PATH=/usr/bin",
				"HOME=/home/user",
			},
		},
		{
			name: "filters all worktree vars",
			input: []string{
				"WORKTREE_PATH=/tmp/wt",
				"MAIN_WORKTREE_PATH=/main",
				"WORKTREE_BRANCH=feature",
				"WORKTREE_NAME=my-wt",
				"REPO_NAME=my-repo",
			},
			expected: []string{},
		},
		{
			name: "handles malformed entries gracefully",
			input: []string{
				"PATH=/usr/bin",
				"NOEQUALS",
				"HOME=/home/user",
			},
			expected: []string{
				"PATH=/usr/bin",
				"NOEQUALS",
				"HOME=/home/user",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterWorktreeEnvVars(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("filterWorktreeEnvVars() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestPagerCommandFallbacksToLess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("pager fallback test relies on unix-like PATH lookup")
	}

	originalPath := os.Getenv("PATH")
	originalPager := os.Getenv("PAGER")
	t.Cleanup(func() {
		_ = os.Setenv("PATH", originalPath)
		_ = os.Setenv("PAGER", originalPager)
	})

	tempDir := t.TempDir()
	lessPath := filepath.Join(tempDir, "less")
	if err := os.WriteFile(lessPath, []byte("#!/bin/sh\nexit 0\n"), 0o600); err != nil {
		t.Fatalf("failed to write fake less: %v", err)
	}
	// #nosec G302 -- test requires an executable file on PATH.
	if err := os.Chmod(lessPath, 0o700); err != nil {
		t.Fatalf("failed to chmod fake less: %v", err)
	}

	if err := os.Setenv("PATH", tempDir); err != nil {
		t.Fatalf("failed to set PATH: %v", err)
	}
	if err := os.Unsetenv("PAGER"); err != nil {
		t.Fatalf("failed to unset PAGER: %v", err)
	}

	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	if pager := m.pagerCommand(); pager != "less --use-color -q --wordwrap -qcR -P 'Press q to exit..'" {
		t.Fatalf("expected fallback pager to be less defaults, got %q", pager)
	}
}

func TestFindOrphanedWorktreeDirs(t *testing.T) {
	// Create a temp directory structure
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "test-repo")
	if err := os.MkdirAll(repoDir, 0o750); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}

	// Create some directories (simulating worktrees)
	validDir := filepath.Join(repoDir, "valid-worktree")
	orphanDir := filepath.Join(repoDir, "orphan-dir")
	hiddenDir := filepath.Join(repoDir, ".hidden-dir")

	for _, dir := range []string{validDir, orphanDir, hiddenDir} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	// Create a regular file (should be ignored)
	regularFile := filepath.Join(repoDir, "regular-file.txt")
	if err := os.WriteFile(regularFile, []byte("test"), 0o600); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	cfg := &config.AppConfig{
		WorktreeDir: tempDir,
	}
	m := NewModel(cfg, "")
	m.repoKey = "test-repo"
	mockGitWorktreeList(t, m, validDir)

	orphans := m.findOrphanedWorktreeDirs()

	// validDir is registered in mocked git output and should not be listed.
	// .hidden-dir and regular-file.txt should be ignored as non-orphan candidates.
	if len(orphans) != 1 {
		t.Fatalf("expected 1 orphan, got %d: %v", len(orphans), orphans)
	}
	if orphans[0] != orphanDir {
		t.Fatalf("expected orphan path %q, got %q", orphanDir, orphans[0])
	}
}

func TestFindOrphanedWorktreeDirsNoGitService(t *testing.T) {
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "test-repo")
	if err := os.MkdirAll(repoDir, 0o750); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}

	// Create a directory that would be an orphan
	orphanDir := filepath.Join(repoDir, "orphan-dir")
	if err := os.MkdirAll(orphanDir, 0o750); err != nil {
		t.Fatalf("failed to create orphan dir: %v", err)
	}

	cfg := &config.AppConfig{WorktreeDir: tempDir}
	m := NewModel(cfg, "")
	m.state.services.git = nil // Simulate git unavailable
	m.repoKey = "test-repo"

	orphans := m.findOrphanedWorktreeDirs()
	if orphans != nil {
		t.Fatalf("expected nil orphans when git unavailable, got %v", orphans)
	}
}

func TestNormalizePath(t *testing.T) {
	// Test that normalizePath cleans paths
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "already clean path",
			input:    "/home/user/worktree",
			expected: "/home/user/worktree",
		},
		{
			name:     "path with trailing slash",
			input:    "/home/user/worktree/",
			expected: "/home/user/worktree",
		},
		{
			name:     "path with double slashes",
			input:    "/home//user/worktree",
			expected: "/home/user/worktree",
		},
		{
			name:     "path with dot segments",
			input:    "/home/user/../user/worktree",
			expected: "/home/user/worktree",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizePath(tt.input)
			if result != tt.expected {
				t.Errorf("normalizePath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizePathWithSymlink(t *testing.T) {
	// Create a temp directory with a symlink
	tempDir := t.TempDir()
	realDir := filepath.Join(tempDir, "real-dir")
	symlinkDir := filepath.Join(tempDir, "symlink-dir")

	if err := os.MkdirAll(realDir, 0o750); err != nil {
		t.Fatalf("failed to create real dir: %v", err)
	}

	if err := os.Symlink(realDir, symlinkDir); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	// Both paths should normalize to the same value
	normalizedReal := normalizePath(realDir)
	normalizedSymlink := normalizePath(symlinkDir)

	if normalizedReal != normalizedSymlink {
		t.Errorf("symlink and real path should normalize to same value:\nreal: %q\nsymlink: %q",
			normalizedReal, normalizedSymlink)
	}
}

func TestSaveCacheFiltersInvalidEntries(t *testing.T) {
	// This test verifies that saveCache doesn't crash when git service is unavailable
	// When git service is nil, all worktrees are saved (graceful fallback)

	tempDir := t.TempDir()
	cfg := &config.AppConfig{
		WorktreeDir: tempDir,
	}
	m := NewModel(cfg, "")
	// Disable git service to bypass worktree validation
	m.state.services.git = nil
	m.repoKey = "test-repo"

	// Create the repo directory for cache
	repoDir := filepath.Join(tempDir, "test-repo")
	if err := os.MkdirAll(repoDir, 0o750); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}

	// Add some worktrees (they won't be valid since no git repo)
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: "/nonexistent/path1", Branch: "branch1"},
		{Path: "/nonexistent/path2", Branch: "branch2"},
	}

	// saveCache should not crash even with invalid worktrees
	// Since git service is nil, validation is bypassed and all worktrees are saved
	m.saveCache()

	// Verify cache file exists and has worktrees (fallback behaviour)
	cachePath := filepath.Join(repoDir, ".worktree-cache.json")
	// #nosec G304 -- cachePath is constructed from test temp directory
	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("failed to read cache file: %v", err)
	}

	// Cache should contain worktrees since validation is bypassed
	if !contains(string(data), "branch1") || !contains(string(data), "branch2") {
		t.Fatalf("expected worktrees in cache (validation bypassed), got: %s", string(data))
	}
}

func TestWindowTitle(t *testing.T) {
	tests := []struct {
		name     string
		repoKey  string
		wts      []*models.WorktreeInfo
		selIdx   int
		expected string
	}{
		{
			name:     "no repo key",
			repoKey:  "",
			expected: "Lazyworktree",
		},
		{
			name:     "with repo key",
			repoKey:  "org/repo",
			expected: "Lazyworktree — org/repo",
		},
		{
			name:    "with repo key and selected branch",
			repoKey: "org/repo",
			wts: []*models.WorktreeInfo{
				{Path: "/tmp/wt", Branch: "feature-x"},
			},
			selIdx:   0,
			expected: "Lazyworktree — org/repo [feature-x]",
		},
		{
			name:     "local repo key excluded",
			repoKey:  "local-abc",
			expected: "Lazyworktree",
		},
		{
			name:     "unknown repo key excluded",
			repoKey:  "unknown",
			expected: "Lazyworktree",
		},
		{
			name:    "selected index out of range",
			repoKey: "org/repo",
			wts:     []*models.WorktreeInfo{},
			selIdx:  5,
			expected: "Lazyworktree — org/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir()}, "")
			m.repoKey = tt.repoKey
			m.state.data.filteredWts = tt.wts
			m.state.data.selectedIndex = tt.selIdx

			got := m.windowTitle()
			if got != tt.expected {
				t.Errorf("windowTitle() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || s != "" && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
