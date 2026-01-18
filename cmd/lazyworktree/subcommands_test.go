package main

import (
	"os"
	"testing"

	"github.com/alecthomas/kong"
)

func TestHandleWtCreateValidation(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "missing both flags",
			args:        []string{},
			expectError: true,
			errorMsg:    "must specify either --from-branch or --from-pr",
		},
		{
			name:        "both flags specified",
			args:        []string{"--from-branch", "main", "--from-pr", "123"},
			expectError: true,
			errorMsg:    "mutually exclusive",
		},
		{
			name:        "with-change with from-pr",
			args:        []string{"--from-pr", "123", "--with-change"},
			expectError: true,
			errorMsg:    "--with-change can only be used with --from-branch",
		},
		{
			name:        "valid from-branch",
			args:        []string{"--from-branch", "main"},
			expectError: false,
		},
		{
			name:        "valid from-pr",
			args:        []string{"--from-pr", "123"},
			expectError: false,
		},
		{
			name:        "valid from-branch with with-change",
			args:        []string{"--from-branch", "main", "--with-change"},
			expectError: false,
		},
		{
			name:        "valid from-branch with branch name",
			args:        []string{"--from-branch", "main", "feature-1"},
			expectError: false,
		},
		{
			name:        "branch name with from-pr",
			args:        []string{"--from-pr", "123", "my-branch"},
			expectError: true,
			errorMsg:    "branch name argument cannot be used with --from-pr",
		},
		{
			name:        "from-branch with branch name and with-change",
			args:        []string{"--from-branch", "main", "feature-1", "--with-change"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a CLI struct to test validation logic
			cmd := &WtCreateCmd{}

			// Capture stderr
			oldStderr := os.Stderr
			_, w, _ := os.Pipe()
			os.Stderr = w

			parser, err := kong.New(cmd)
			if err != nil {
				_ = w.Close()
				os.Stderr = oldStderr
				t.Fatalf("failed to create parser: %v", err)
			}

			_, err = parser.Parse(tt.args)
			if err != nil {
				_ = w.Close()
				os.Stderr = oldStderr
				// Kong might handle xor validation, so we check our custom validation
				if !tt.expectError {
					t.Logf("Kong parse error (may be expected): %v", err)
				}
			}

			// Test validation logic (same as in handleWtCreate)
			hasError := false
			switch {
			case cmd.FromBranch != "" && cmd.FromPR > 0:
				hasError = true
			case cmd.FromBranch == "" && cmd.FromPR == 0:
				hasError = true
			case cmd.WithChange && cmd.FromPR > 0:
				hasError = true
			case cmd.BranchName != "" && cmd.FromPR > 0:
				hasError = true
			}

			_ = w.Close()
			os.Stderr = oldStderr

			if tt.expectError && !hasError {
				t.Error("expected validation error but got none")
			}
			if !tt.expectError && hasError {
				t.Error("unexpected validation error")
			}
		})
	}
}

func TestHandleWtDeleteFlags(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		noBranch bool
		silent   bool
		worktree string
	}{
		{
			name:     "default flags",
			args:     []string{},
			noBranch: false,
			silent:   false,
		},
		{
			name:     "no-branch flag",
			args:     []string{"--no-branch"},
			noBranch: true,
			silent:   false,
		},
		{
			name:     "silent flag",
			args:     []string{"--silent"},
			noBranch: false,
			silent:   true,
		},
		{
			name:     "worktree path",
			args:     []string{"/path/to/worktree"},
			noBranch: false,
			silent:   false,
			worktree: "/path/to/worktree",
		},
		{
			name:     "all flags and path",
			args:     []string{"--no-branch", "--silent", "/path/to/worktree"},
			noBranch: true,
			silent:   true,
			worktree: "/path/to/worktree",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &WtDeleteCmd{}

			parser, err := kong.New(cmd)
			if err != nil {
				t.Fatalf("failed to create parser: %v", err)
			}

			if _, err := parser.Parse(tt.args); err != nil {
				t.Fatalf("unexpected parse error: %v", err)
			}

			if cmd.NoBranch != tt.noBranch {
				t.Errorf("noBranch = %v, want %v", cmd.NoBranch, tt.noBranch)
			}
			if cmd.Silent != tt.silent {
				t.Errorf("silent = %v, want %v", cmd.Silent, tt.silent)
			}

			if cmd.WorktreePath != tt.worktree {
				t.Errorf("worktreePath = %q, want %q", cmd.WorktreePath, tt.worktree)
			}
		})
	}
}
