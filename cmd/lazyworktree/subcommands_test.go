package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/git"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		{
			name:        "valid from-issue",
			args:        []string{"lazyworktree", "create", "--from-issue", "42"},
			expectError: false,
		},
		{
			name:        "from-issue with from-pr (invalid)",
			args:        []string{"lazyworktree", "create", "--from-issue", "42", "--from-pr", "123"},
			expectError: true,
			errorMsg:    "--from-pr and --from-issue are mutually exclusive",
		},
		{
			name:        "from-issue with from-branch (valid - overrides base)",
			args:        []string{"lazyworktree", "create", "--from-issue", "42", "--from-branch", "develop"},
			expectError: false,
		},
		{
			name:        "from-issue with positional name (invalid)",
			args:        []string{"lazyworktree", "create", "--from-issue", "42", "my-branch"},
			expectError: true,
			errorMsg:    "positional name argument cannot be used with --from-issue",
		},
		{
			name:        "from-issue with with-change (invalid)",
			args:        []string{"lazyworktree", "create", "--from-issue", "42", "--with-change"},
			expectError: true,
			errorMsg:    "--with-change cannot be used with --from-issue",
		},
		{
			name:        "no-workspace with from-pr (valid)",
			args:        []string{"lazyworktree", "create", "--from-pr", "123", "--no-workspace"},
			expectError: false,
		},
		{
			name:        "no-workspace with from-issue (valid)",
			args:        []string{"lazyworktree", "create", "--from-issue", "42", "--no-workspace"},
			expectError: false,
		},
		{
			name:        "no-workspace alone (invalid)",
			args:        []string{"lazyworktree", "create", "--no-workspace"},
			expectError: true,
			errorMsg:    "--no-workspace requires --from-pr, --from-pr-interactive, --from-issue, or --from-issue-interactive",
		},
		{
			name:        "no-workspace with from-branch only (invalid)",
			args:        []string{"lazyworktree", "create", "--from-branch", "main", "--no-workspace"},
			expectError: true,
			errorMsg:    "--no-workspace requires --from-pr, --from-pr-interactive, --from-issue, or --from-issue-interactive",
		},
		{
			name:        "no-workspace with with-change (invalid)",
			args:        []string{"lazyworktree", "create", "--from-pr", "123", "--no-workspace", "--with-change"},
			expectError: true,
			errorMsg:    "--with-change cannot be used with --from-pr",
		},
		{
			name:        "no-workspace with generate (invalid)",
			args:        []string{"lazyworktree", "create", "--from-pr", "123", "--no-workspace", "--generate"},
			expectError: true,
			errorMsg:    "--no-workspace cannot be used with --generate",
		},
		{
			name:        "no-workspace with positional name (invalid)",
			args:        []string{"lazyworktree", "create", "--from-issue", "42", "--no-workspace", "my-branch"},
			expectError: true,
			errorMsg:    "positional name argument cannot be used with --from-issue",
		},
		// --from-issue-interactive tests
		{
			name:        "valid from-issue-interactive",
			args:        []string{"lazyworktree", "create", "--from-issue-interactive"},
			expectError: false,
		},
		{
			name:        "valid from-issue-interactive short flag",
			args:        []string{"lazyworktree", "create", "-I"},
			expectError: false,
		},
		{
			name:        "from-issue-interactive with from-branch (valid)",
			args:        []string{"lazyworktree", "create", "--from-issue-interactive", "--from-branch", "main"},
			expectError: false,
		},
		{
			name:        "from-issue-interactive with no-workspace (valid)",
			args:        []string{"lazyworktree", "create", "--from-issue-interactive", "--no-workspace"},
			expectError: false,
		},
		{
			name:        "from-issue-interactive with from-issue (invalid)",
			args:        []string{"lazyworktree", "create", "--from-issue-interactive", "--from-issue", "42"},
			expectError: true,
			errorMsg:    "--from-issue-interactive and --from-issue are mutually exclusive",
		},
		{
			name:        "from-issue-interactive with from-pr (invalid)",
			args:        []string{"lazyworktree", "create", "--from-issue-interactive", "--from-pr", "123"},
			expectError: true,
			errorMsg:    "--from-issue-interactive and --from-pr are mutually exclusive",
		},
		{
			name:        "from-issue-interactive with positional name (invalid)",
			args:        []string{"lazyworktree", "create", "--from-issue-interactive", "my-branch"},
			expectError: true,
			errorMsg:    "positional name argument cannot be used with --from-issue-interactive",
		},
		{
			name:        "from-issue-interactive with with-change (invalid)",
			args:        []string{"lazyworktree", "create", "--from-issue-interactive", "--with-change"},
			expectError: true,
			errorMsg:    "--with-change cannot be used with --from-issue-interactive",
		},
		// --from-pr-interactive tests
		{
			name:        "valid from-pr-interactive",
			args:        []string{"lazyworktree", "create", "--from-pr-interactive"},
			expectError: false,
		},
		{
			name:        "valid from-pr-interactive short flag",
			args:        []string{"lazyworktree", "create", "-P"},
			expectError: false,
		},
		{
			name:        "from-pr-interactive with no-workspace (valid)",
			args:        []string{"lazyworktree", "create", "--from-pr-interactive", "--no-workspace"},
			expectError: false,
		},
		{
			name:        "from-pr-interactive with from-pr (invalid)",
			args:        []string{"lazyworktree", "create", "--from-pr-interactive", "--from-pr", "123"},
			expectError: true,
			errorMsg:    "--from-pr-interactive and --from-pr are mutually exclusive",
		},
		{
			name:        "from-pr-interactive with from-issue (invalid)",
			args:        []string{"lazyworktree", "create", "--from-pr-interactive", "--from-issue", "42"},
			expectError: true,
			errorMsg:    "--from-pr-interactive and --from-issue are mutually exclusive",
		},
		{
			name:        "from-pr-interactive with from-issue-interactive (invalid)",
			args:        []string{"lazyworktree", "create", "--from-pr-interactive", "--from-issue-interactive"},
			expectError: true,
			errorMsg:    "--from-pr-interactive and --from-issue-interactive are mutually exclusive",
		},
		{
			name:        "from-pr-interactive with from-branch (invalid)",
			args:        []string{"lazyworktree", "create", "--from-pr-interactive", "--from-branch", "main"},
			expectError: true,
			errorMsg:    "--from-pr-interactive and --from-branch are mutually exclusive",
		},
		{
			name:        "from-pr-interactive with positional name (invalid)",
			args:        []string{"lazyworktree", "create", "--from-pr-interactive", "my-branch"},
			expectError: true,
			errorMsg:    "positional name argument cannot be used with --from-pr-interactive",
		},
		{
			name:        "from-pr-interactive with with-change (invalid)",
			args:        []string{"lazyworktree", "create", "--from-pr-interactive", "--with-change"},
			expectError: true,
			errorMsg:    "--with-change cannot be used with --from-pr-interactive",
		},
		{
			name:        "generate with from-pr-interactive (invalid)",
			args:        []string{"lazyworktree", "create", "--generate", "--from-pr-interactive"},
			expectError: true,
			errorMsg:    "--generate flag cannot be used with --from-pr-interactive",
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

func TestHandleListValidation(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "pristine and json together",
			args:        []string{"lazyworktree", "list", "--pristine", "--json"},
			expectError: true,
			errorMsg:    "mutually exclusive",
		},
		{
			name:        "pristine only",
			args:        []string{"lazyworktree", "list", "--pristine"},
			expectError: false,
		},
		{
			name:        "json only",
			args:        []string{"lazyworktree", "list", "--json"},
			expectError: false,
		},
		{
			name:        "default output",
			args:        []string{"lazyworktree", "list"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := listCommand()
			app := &urfavecli.Command{
				Name:     "lazyworktree",
				Commands: []*urfavecli.Command{cmd},
			}

			savedAction := cmd.Action
			cmd.Action = func(ctx context.Context, c *urfavecli.Command) error {
				return validateListFlags(c)
			}

			err := app.Run(context.Background(), tt.args)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
			}

			cmd.Action = savedAction
		})
	}
}

func TestHandleCreateOutputSelection(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "output.txt")
	expectedPath := filepath.Join(tmpDir, "repo", "feature")

	oldLoadCLIConfig := loadCLIConfigFunc
	oldNewCLIGitService := newCLIGitServiceFunc
	oldCreateFromBranch := createFromBranchFunc
	oldCreateFromPR := createFromPRFunc
	oldCreateFromIssue := createFromIssueFunc
	oldSelectIssueInteractive := selectIssueInteractiveFunc
	oldSelectPRInteractive := selectPRInteractiveFunc
	oldWriteOutputSelection := writeOutputSelectionFunc
	t.Cleanup(func() {
		loadCLIConfigFunc = oldLoadCLIConfig
		newCLIGitServiceFunc = oldNewCLIGitService
		createFromBranchFunc = oldCreateFromBranch
		createFromPRFunc = oldCreateFromPR
		createFromIssueFunc = oldCreateFromIssue
		selectIssueInteractiveFunc = oldSelectIssueInteractive
		selectPRInteractiveFunc = oldSelectPRInteractive
		writeOutputSelectionFunc = oldWriteOutputSelection
	})

	loadCLIConfigFunc = func(string, string, []string) (*config.AppConfig, error) {
		return &config.AppConfig{WorktreeDir: tmpDir}, nil
	}
	newCLIGitServiceFunc = func(*config.AppConfig) *git.Service {
		return &git.Service{}
	}
	createFromBranchFunc = func(_ context.Context, _ *git.Service, _ *config.AppConfig, _, _ string, _, _ bool) (string, error) {
		return expectedPath, nil
	}
	createFromPRFunc = func(_ context.Context, _ *git.Service, _ *config.AppConfig, _ int, _, _ bool) (string, error) {
		return "", os.ErrInvalid
	}
	createFromIssueFunc = func(_ context.Context, _ *git.Service, _ *config.AppConfig, _ int, _ string, _, _ bool) (string, error) {
		return "", os.ErrInvalid
	}
	selectIssueInteractiveFunc = func(_ context.Context, _ *git.Service) (int, error) {
		return 0, os.ErrInvalid
	}
	selectPRInteractiveFunc = func(_ context.Context, _ *git.Service) (int, error) {
		return 0, os.ErrInvalid
	}
	writeOutputSelectionFunc = writeOutputSelection

	cmd := createCommand()
	app := &urfavecli.Command{
		Name:     "lazyworktree",
		Commands: []*urfavecli.Command{cmd},
	}

	origStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to capture stdout: %v", err)
	}
	os.Stdout = writer
	t.Cleanup(func() {
		_ = writer.Close()
		os.Stdout = origStdout
	})

	args := []string{"lazyworktree", "create", "--from-branch", "main", "--output-selection", outputFile}
	if err := app.Run(context.Background(), args); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = writer.Close()
	// #nosec G304 - test file operations with t.TempDir() are safe
	output, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if string(output) != expectedPath+"\n" {
		t.Fatalf("expected output %q, got %q", expectedPath+"\n", string(output))
	}

	stdoutBytes, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to read stdout: %v", err)
	}
	if strings.TrimSpace(string(stdoutBytes)) != "" {
		t.Fatalf("expected no stdout output, got %q", string(stdoutBytes))
	}
}

func TestHandleCreateOutputSelectionFailureLeavesFile(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "output.txt")
	const filePerms = 0o600
	if err := os.WriteFile(outputFile, []byte("existing\n"), filePerms); err != nil {
		t.Fatalf("failed to seed output file: %v", err)
	}

	oldLoadCLIConfig := loadCLIConfigFunc
	oldNewCLIGitService := newCLIGitServiceFunc
	oldCreateFromBranch := createFromBranchFunc
	oldCreateFromPR := createFromPRFunc
	oldCreateFromIssue := createFromIssueFunc
	oldSelectIssueInteractive := selectIssueInteractiveFunc
	oldSelectPRInteractive := selectPRInteractiveFunc
	oldWriteOutputSelection := writeOutputSelectionFunc
	t.Cleanup(func() {
		loadCLIConfigFunc = oldLoadCLIConfig
		newCLIGitServiceFunc = oldNewCLIGitService
		createFromBranchFunc = oldCreateFromBranch
		createFromPRFunc = oldCreateFromPR
		createFromIssueFunc = oldCreateFromIssue
		selectIssueInteractiveFunc = oldSelectIssueInteractive
		selectPRInteractiveFunc = oldSelectPRInteractive
		writeOutputSelectionFunc = oldWriteOutputSelection
	})

	loadCLIConfigFunc = func(string, string, []string) (*config.AppConfig, error) {
		return &config.AppConfig{WorktreeDir: tmpDir}, nil
	}
	newCLIGitServiceFunc = func(*config.AppConfig) *git.Service {
		return &git.Service{}
	}
	createFromBranchFunc = func(_ context.Context, _ *git.Service, _ *config.AppConfig, _, _ string, _, _ bool) (string, error) {
		return "", os.ErrInvalid
	}
	createFromPRFunc = func(_ context.Context, _ *git.Service, _ *config.AppConfig, _ int, _, _ bool) (string, error) {
		return "", os.ErrInvalid
	}
	createFromIssueFunc = func(_ context.Context, _ *git.Service, _ *config.AppConfig, _ int, _ string, _, _ bool) (string, error) {
		return "", os.ErrInvalid
	}
	selectIssueInteractiveFunc = func(_ context.Context, _ *git.Service) (int, error) {
		return 0, os.ErrInvalid
	}
	selectPRInteractiveFunc = func(_ context.Context, _ *git.Service) (int, error) {
		return 0, os.ErrInvalid
	}
	writeOutputSelectionFunc = writeOutputSelection

	cmd := createCommand()
	app := &urfavecli.Command{
		Name:     "lazyworktree",
		Commands: []*urfavecli.Command{cmd},
	}

	args := []string{"lazyworktree", "create", "--from-branch", "main", "--output-selection", outputFile}
	if err := app.Run(context.Background(), args); err == nil {
		t.Fatal("expected error")
	}

	// #nosec G304 - test file operations with t.TempDir() are safe
	output, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if string(output) != "existing\n" {
		t.Fatalf("expected output file to remain unchanged, got %q", string(output))
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

func TestListCommandFlags(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		pristine bool
		json     bool
	}{
		{
			name:     "default flags (verbose table)",
			args:     []string{"lazyworktree", "list"},
			pristine: false,
			json:     false,
		},
		{
			name:     "pristine flag",
			args:     []string{"lazyworktree", "list", "--pristine"},
			pristine: true,
			json:     false,
		},
		{
			name:     "pristine short flag",
			args:     []string{"lazyworktree", "list", "-p"},
			pristine: true,
			json:     false,
		},
		{
			name:     "json flag",
			args:     []string{"lazyworktree", "list", "--json"},
			pristine: false,
			json:     true,
		},
		{
			name:     "ls alias",
			args:     []string{"lazyworktree", "ls"},
			pristine: false,
			json:     false,
		},
		{
			name:     "ls alias with pristine",
			args:     []string{"lazyworktree", "ls", "-p"},
			pristine: true,
			json:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := listCommand()
			var capturedPristine, capturedJSON bool

			cmd.Action = func(_ context.Context, c *urfavecli.Command) error {
				capturedPristine = c.Bool("pristine")
				capturedJSON = c.Bool("json")
				return nil
			}

			app := &urfavecli.Command{
				Name:     "lazyworktree",
				Commands: []*urfavecli.Command{cmd},
			}

			err := app.Run(context.Background(), tt.args)
			require.NoError(t, err)
			assert.Equal(t, tt.pristine, capturedPristine, "pristine flag mismatch")
			assert.Equal(t, tt.json, capturedJSON, "json flag mismatch")
		})
	}
}

func TestBuildStatusString(t *testing.T) {
	tests := []struct {
		name     string
		wt       *models.WorktreeInfo
		expected string
	}{
		{
			name:     "clean worktree",
			wt:       &models.WorktreeInfo{Dirty: false, HasUpstream: true},
			expected: "✓",
		},
		{
			name:     "dirty worktree",
			wt:       &models.WorktreeInfo{Dirty: true, HasUpstream: true},
			expected: "~",
		},
		{
			name:     "ahead only",
			wt:       &models.WorktreeInfo{Dirty: false, HasUpstream: true, Ahead: 3},
			expected: "✓↑3",
		},
		{
			name:     "behind only",
			wt:       &models.WorktreeInfo{Dirty: false, HasUpstream: true, Behind: 2},
			expected: "✓↓2",
		},
		{
			name:     "ahead and behind",
			wt:       &models.WorktreeInfo{Dirty: false, HasUpstream: true, Ahead: 3, Behind: 2},
			expected: "✓↓2↑3",
		},
		{
			name:     "dirty with ahead and behind",
			wt:       &models.WorktreeInfo{Dirty: true, HasUpstream: true, Ahead: 1, Behind: 5},
			expected: "~↓5↑1",
		},
		{
			name:     "unpushed without upstream",
			wt:       &models.WorktreeInfo{Dirty: false, HasUpstream: false, Unpushed: 4},
			expected: "✓?4",
		},
		{
			name:     "unpushed with upstream is ignored",
			wt:       &models.WorktreeInfo{Dirty: false, HasUpstream: true, Unpushed: 4},
			expected: "✓",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildStatusString(tt.wt)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOutputListJSON(t *testing.T) {
	worktrees := []*models.WorktreeInfo{
		{
			Path:        "/home/user/worktrees/main",
			Branch:      "main",
			IsMain:      true,
			Dirty:       false,
			Ahead:       0,
			Behind:      0,
			HasUpstream: true,
			LastActive:  "2 hours ago",
		},
		{
			Path:        "/home/user/worktrees/feature-x",
			Branch:      "feature/x",
			IsMain:      false,
			Dirty:       true,
			Ahead:       3,
			Behind:      1,
			HasUpstream: true,
			LastActive:  "5 mins ago",
		},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = oldStdout
		_ = w.Close()
		_ = r.Close()
	})

	err = outputListJSON(worktrees)
	require.NoError(t, err)

	_ = w.Close()

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	var result []worktreeJSON
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)

	assert.Len(t, result, 2)
	assert.Equal(t, "/home/user/worktrees/main", result[0].Path)
	assert.Equal(t, "main", result[0].Name)
	assert.Equal(t, "main", result[0].Branch)
	assert.True(t, result[0].IsMain)
	assert.False(t, result[0].Dirty)

	assert.Equal(t, "/home/user/worktrees/feature-x", result[1].Path)
	assert.Equal(t, "feature-x", result[1].Name)
	assert.Equal(t, "feature/x", result[1].Branch)
	assert.False(t, result[1].IsMain)
	assert.True(t, result[1].Dirty)
	assert.Equal(t, 3, result[1].Ahead)
	assert.Equal(t, 1, result[1].Behind)
}

func TestOutputListVerbose(t *testing.T) {
	worktrees := []*models.WorktreeInfo{
		{
			Path:        "/home/user/worktrees/main",
			Branch:      "main",
			IsMain:      true,
			Dirty:       false,
			HasUpstream: true,
			LastActive:  "2 hours ago",
		},
		{
			Path:        "/home/user/worktrees/feature-x",
			Branch:      "feature/x",
			IsMain:      false,
			Dirty:       true,
			Ahead:       3,
			Behind:      2,
			HasUpstream: true,
			LastActive:  "5 mins ago",
		},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = oldStdout
		_ = w.Close()
		_ = r.Close()
	})

	err = outputListVerbose(worktrees)
	require.NoError(t, err)

	_ = w.Close()

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Verify header
	assert.Contains(t, output, "NAME")
	assert.Contains(t, output, "BRANCH")
	assert.Contains(t, output, "STATUS")
	assert.Contains(t, output, "LAST ACTIVE")
	assert.Contains(t, output, "PATH")

	// Verify content
	assert.Contains(t, output, "main")
	assert.Contains(t, output, "feature-x")
	assert.Contains(t, output, "feature/x")
	assert.Contains(t, output, "2 hours ago")
	assert.Contains(t, output, "5 mins ago")

	// Verify paths
	assert.Contains(t, output, "/home/user/worktrees/main")
	assert.Contains(t, output, "/home/user/worktrees/feature-x")

	// Verify status indicators
	assert.Contains(t, output, "✓")  // clean
	assert.Contains(t, output, "~")  // dirty
	assert.Contains(t, output, "↑3") // ahead
	assert.Contains(t, output, "↓2") // behind
}

func TestSortWorktreesByPath(t *testing.T) {
	worktrees := []*models.WorktreeInfo{
		{Path: "/worktrees/zeta"},
		{Path: "/worktrees/alpha"},
		{Path: "/worktrees/beta"},
	}

	sortWorktreesByPath(worktrees)

	assert.Equal(t, "/worktrees/alpha", worktrees[0].Path)
	assert.Equal(t, "/worktrees/beta", worktrees[1].Path)
	assert.Equal(t, "/worktrees/zeta", worktrees[2].Path)
}
