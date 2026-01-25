package main

import (
	"context"
	"testing"

	urfavecli "github.com/urfave/cli/v3"
)

func TestHandleCreateValidation(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "both flags specified",
			args:        []string{"lazyworktree", "create", "--from-branch", "main", "--from-pr", "123"},
			expectError: true,
			errorMsg:    "mutually exclusive",
		},
		{
			name:        "valid from-branch",
			args:        []string{"lazyworktree", "create", "--from-branch", "main"},
			expectError: false,
		},
		{
			name:        "valid from-pr",
			args:        []string{"lazyworktree", "create", "--from-pr", "123"},
			expectError: false,
		},
		{
			name:        "valid from-branch with with-change",
			args:        []string{"lazyworktree", "create", "--from-branch", "main", "--with-change"},
			expectError: false,
		},
		{
			name:        "valid from-branch with branch name",
			args:        []string{"lazyworktree", "create", "--from-branch", "main", "feature-1"},
			expectError: false,
		},
		{
			name:        "branch name with from-pr",
			args:        []string{"lazyworktree", "create", "--from-pr", "123", "my-branch"},
			expectError: true,
			errorMsg:    "positional name argument cannot be used with --from-pr",
		},
		{
			name:        "from-branch with branch name and with-change",
			args:        []string{"lazyworktree", "create", "--from-branch", "main", "feature-1", "--with-change"},
			expectError: false,
		},
		{
			name:        "no arguments (would use current branch in real scenario)",
			args:        []string{"lazyworktree", "create"},
			expectError: false, // Validation won't error, runtime will check current branch
		},
		{
			name:        "branch name only (current branch + explicit name)",
			args:        []string{"lazyworktree", "create", "my-feature"},
			expectError: false,
		},
		{
			name:        "with-change only (current branch + changes)",
			args:        []string{"lazyworktree", "create", "--with-change"},
			expectError: false,
		},
		{
			name:        "branch name and with-change (current branch + explicit name + changes)",
			args:        []string{"lazyworktree", "create", "my-feature", "--with-change"},
			expectError: false,
		},
		{
			name:        "from-pr with with-change (invalid)",
			args:        []string{"lazyworktree", "create", "--from-pr", "123", "--with-change"},
			expectError: true,
			errorMsg:    "--with-change cannot be used with --from-pr",
		},
		{
			name:        "generate flag (valid)",
			args:        []string{"lazyworktree", "create", "--generate"},
			expectError: false,
		},
		{
			name:        "generate flag with from-branch (valid)",
			args:        []string{"lazyworktree", "create", "--from-branch", "main", "--generate"},
			expectError: false,
		},
		{
			name:        "generate flag with positional name (invalid)",
			args:        []string{"lazyworktree", "create", "--generate", "my-feature"},
			expectError: true,
			errorMsg:    "--generate flag cannot be used with a positional name argument",
		},
		{
			name:        "generate flag with positional name and from-branch (invalid)",
			args:        []string{"lazyworktree", "create", "--from-branch", "main", "--generate", "my-feature"},
			expectError: true,
			errorMsg:    "--generate flag cannot be used with a positional name argument",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test app with just the create command
			// The validation is now part of the Action function
			cmd := createCommand()

			app := &urfavecli.Command{
				Name:     "lazyworktree",
				Commands: []*urfavecli.Command{cmd},
			}

			// Capture validation errors without executing the full action
			savedAction := cmd.Action
			cmd.Action = func(ctx context.Context, c *urfavecli.Command) error {
				// Run validation only
				if err := validateCreateFlags(ctx, c); err != nil {
					return err
				}
				return nil
			}

			err := app.Run(context.Background(), tt.args)

			if tt.expectError && err == nil {
				t.Error("expected validation error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Restore original action
			cmd.Action = savedAction
		})
	}
}

func TestHandleDeleteFlags(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		noBranch bool
		silent   bool
		worktree string
	}{
		{
			name:     "default flags",
			args:     []string{"lazyworktree", "delete"},
			noBranch: false,
			silent:   false,
		},
		{
			name:     "no-branch flag",
			args:     []string{"lazyworktree", "delete", "--no-branch"},
			noBranch: true,
			silent:   false,
		},
		{
			name:     "silent flag",
			args:     []string{"lazyworktree", "delete", "--silent"},
			noBranch: false,
			silent:   true,
		},
		{
			name:     "worktree path",
			args:     []string{"lazyworktree", "delete", "/path/to/worktree"},
			noBranch: false,
			silent:   false,
			worktree: "/path/to/worktree",
		},
		{
			name:     "all flags and path",
			args:     []string{"lazyworktree", "delete", "--no-branch", "--silent", "/path/to/worktree"},
			noBranch: true,
			silent:   true,
			worktree: "/path/to/worktree",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test app with just the delete command
			// We override the Action to capture and check flag values
			cmd := deleteCommand()
			var capturedNoBranch, capturedSilent bool
			var capturedWorktree string

			cmd.Action = func(ctx context.Context, c *urfavecli.Command) error {
				capturedNoBranch = c.Bool("no-branch")
				capturedSilent = c.Bool("silent")
				if c.NArg() > 0 {
					capturedWorktree = c.Args().Get(0)
				}
				return nil
			}

			app := &urfavecli.Command{
				Name:     "lazyworktree",
				Commands: []*urfavecli.Command{cmd},
			}

			if err := app.Run(context.Background(), tt.args); err != nil {
				t.Fatalf("unexpected parse error: %v", err)
			}

			if capturedNoBranch != tt.noBranch {
				t.Errorf("noBranch = %v, want %v", capturedNoBranch, tt.noBranch)
			}
			if capturedSilent != tt.silent {
				t.Errorf("silent = %v, want %v", capturedSilent, tt.silent)
			}
			if capturedWorktree != tt.worktree {
				t.Errorf("worktreePath = %q, want %q", capturedWorktree, tt.worktree)
			}
		})
	}
}
