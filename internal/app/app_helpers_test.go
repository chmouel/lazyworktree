package app

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/chmouel/lazyworktree/internal/config"
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
