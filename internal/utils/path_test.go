package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to read home dir: %v", err)
	}

	t.Setenv("LW_TEST_DIR", "/tmp/lw")
	t.Setenv("CUSTOM_VAR", "/custom")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "tilde",
			input:    "~",
			expected: home,
		},
		{
			name:     "tilde nested",
			input:    "~/.config/lazyworktree",
			expected: filepath.Join(home, ".config", "lazyworktree"),
		},
		{
			name:     "tilde worktrees",
			input:    "~/worktrees",
			expected: filepath.Join(home, "worktrees"),
		},
		{
			name:     "env var",
			input:    "$LW_TEST_DIR/path",
			expected: "/tmp/lw/path",
		},
		{
			name:     "custom env var",
			input:    "$CUSTOM_VAR/test",
			expected: "/custom/test",
		},
		{
			name:     "absolute path",
			input:    "/tmp/worktrees",
			expected: "/tmp/worktrees",
		},
		{
			name:     "relative path",
			input:    "relative/path",
			expected: "relative/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExpandPath(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
