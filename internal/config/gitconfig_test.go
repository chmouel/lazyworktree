package config

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseGitConfigOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected map[string][]string
		wantErr  bool
	}{
		{
			name: "single values",
			output: `lw.worktree_dir /path/to/dir
lw.auto_fetch_prs true
lw.theme dracula`,
			expected: map[string][]string{
				"worktree_dir":   {"/path/to/dir"},
				"auto_fetch_prs": {"true"},
				"theme":          {"dracula"},
			},
		},
		{
			name: "multi-value keys",
			output: `lw.init_commands link_topsymlinks
lw.init_commands npm install
lw.worktree_dir /path`,
			expected: map[string][]string{
				"init_commands": {"link_topsymlinks", "npm install"},
				"worktree_dir":  {"/path"},
			},
		},
		{
			name: "values with spaces",
			output: `lw.worktree_dir /path/to/my worktrees
lw.git_pager_args --syntax-theme Dracula
lw.editor code --wait`,
			expected: map[string][]string{
				"worktree_dir":   {"/path/to/my worktrees"},
				"git_pager_args": {"--syntax-theme Dracula"},
				"editor":         {"code --wait"},
			},
		},
		{
			name:     "empty output",
			output:   "",
			expected: map[string][]string{},
		},
		{
			name:     "whitespace only",
			output:   "   \n\n  ",
			expected: map[string][]string{},
		},
		{
			name: "hyphens normalised to underscores",
			output: `lw.worktree-dir /path/to/dir
lw.auto-fetch-prs true
lw.init-commands npm install`,
			expected: map[string][]string{
				"worktree_dir":   {"/path/to/dir"},
				"auto_fetch_prs": {"true"},
				"init_commands":  {"npm install"},
			},
		},
		{
			name: "mixed valid and empty lines",
			output: `lw.theme nord

lw.auto_fetch_prs true

`,
			expected: map[string][]string{
				"theme":          {"nord"},
				"auto_fetch_prs": {"true"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseGitConfigOutput(tt.output)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertGitConfigToParseConfig(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string][]string
		expected map[string]any
	}{
		{
			name: "single values",
			input: map[string][]string{
				"worktree_dir":   {"/path/to/dir"},
				"auto_fetch_prs": {"true"},
				"theme":          {"dracula"},
			},
			expected: map[string]any{
				"worktree_dir":   "/path/to/dir",
				"auto_fetch_prs": "true",
				"theme":          "dracula",
			},
		},
		{
			name: "multi-value arrays",
			input: map[string][]string{
				"init_commands": {"cmd1", "cmd2", "cmd3"},
				"theme":         {"nord"},
			},
			expected: map[string]any{
				"init_commands": []any{"cmd1", "cmd2", "cmd3"},
				"theme":         "nord",
			},
		},
		{
			name: "empty values",
			input: map[string][]string{
				"worktree_dir": {},
			},
			expected: map[string]any{},
		},
		{
			name:     "empty map",
			input:    map[string][]string{},
			expected: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertGitConfigToParseConfig(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsInGitRepo(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "empty path",
			path:     "",
			expected: false,
		},
		{
			name:     "current directory (likely a git repo in CI)",
			path:     ".",
			expected: true, // This test file is in a git repo
		},
		{
			name:     "non-existent path",
			path:     "/non/existent/path/12345",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isInGitRepo(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseCLIConfigOverrides(t *testing.T) {
	tests := []struct {
		name      string
		overrides []string
		expected  map[string]any
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "single override",
			overrides: []string{"lw.theme=dracula"},
			expected: map[string]any{
				"theme": "dracula",
			},
		},
		{
			name:      "multiple overrides",
			overrides: []string{"lw.theme=nord", "lw.auto_fetch_prs=true", "lw.worktree_dir=/path"},
			expected: map[string]any{
				"theme":          "nord",
				"auto_fetch_prs": "true",
				"worktree_dir":   "/path",
			},
		},
		{
			name:      "boolean override as string",
			overrides: []string{"lw.auto_fetch_prs=yes"},
			expected: map[string]any{
				"auto_fetch_prs": "yes",
			},
		},
		{
			name:      "value with equals sign",
			overrides: []string{"lw.branch_name_script=awk -F= '{print $2}'"},
			expected: map[string]any{
				"branch_name_script": "awk -F= '{print $2}'",
			},
		},
		{
			name:      "repeated keys become array",
			overrides: []string{"lw.init_commands=cmd1", "lw.init_commands=cmd2", "lw.theme=nord"},
			expected: map[string]any{
				"init_commands": []any{"cmd1", "cmd2"},
				"theme":         "nord",
			},
		},
		{
			name:      "three repeated keys",
			overrides: []string{"lw.init_commands=cmd1", "lw.init_commands=cmd2", "lw.init_commands=cmd3"},
			expected: map[string]any{
				"init_commands": []any{"cmd1", "cmd2", "cmd3"},
			},
		},
		{
			name:      "missing equals sign",
			overrides: []string{"lw.theme"},
			wantErr:   true,
			errMsg:    "invalid config override",
		},
		{
			name:      "missing lw prefix",
			overrides: []string{"theme=dracula"},
			wantErr:   true,
			errMsg:    "config override key must start with 'lw.'",
		},
		{
			name:      "empty key",
			overrides: []string{"lw.=value"},
			wantErr:   true,
			errMsg:    "empty config key",
		},
		{
			name:      "hyphenated keys normalised",
			overrides: []string{"lw.worktree-dir=/path", "lw.auto-fetch-prs=true"},
			expected: map[string]any{
				"worktree_dir":   "/path",
				"auto_fetch_prs": "true",
			},
		},
		{
			name:      "empty value is allowed",
			overrides: []string{"lw.theme="},
			expected: map[string]any{
				"theme": "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseCLIConfigOverrides(tt.overrides)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLoadGitConfigErrorHandling(t *testing.T) {
	// Setup mock
	defer func() { gitConfigMock = nil }()

	gitConfigMock = func(args []string, repoPath string) (string, error) {
		return "", fmt.Errorf("git command failed")
	}

	result, err := loadGitConfig(true, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git command failed")
	assert.Nil(t, result)
}

func TestLoadGitConfig(t *testing.T) {
	// Setup mock
	defer func() { gitConfigMock = nil }()

	tests := []struct {
		name       string
		globalOnly bool
		repoPath   string
		mockOutput string
		mockError  error
		expected   map[string]any
		wantErr    bool
	}{
		{
			name:       "global config with values",
			globalOnly: true,
			repoPath:   "",
			mockOutput: "lw.worktree_dir /global/path\nlw.auto_fetch_prs true\n",
			expected: map[string]any{
				"worktree_dir":   "/global/path",
				"auto_fetch_prs": "true",
			},
		},
		{
			name:       "local config with values",
			globalOnly: false,
			repoPath:   "/repo",
			mockOutput: "lw.theme dracula\nlw.auto_fetch_prs false\n",
			expected: map[string]any{
				"theme":          "dracula",
				"auto_fetch_prs": "false",
			},
		},
		{
			name:       "empty output",
			globalOnly: true,
			repoPath:   "",
			mockOutput: "",
			expected:   map[string]any{},
		},
		{
			name:       "multi-value config",
			globalOnly: true,
			repoPath:   "",
			mockOutput: "lw.init_commands cmd1\nlw.init_commands cmd2\n",
			expected: map[string]any{
				"init_commands": []any{"cmd1", "cmd2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gitConfigMock = func(args []string, repoPath string) (string, error) {
				// Verify correct arguments
				if tt.globalOnly {
					assert.Contains(t, args, "--global")
				} else {
					assert.Contains(t, args, "--local")
				}
				assert.Equal(t, tt.repoPath, repoPath)
				return tt.mockOutput, tt.mockError
			}

			result, err := loadGitConfig(tt.globalOnly, tt.repoPath)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDetermineRepoPath(t *testing.T) {
	tests := []struct {
		name        string
		worktreeDir string
		expectEmpty bool
	}{
		{
			name:        "empty worktree dir, current dir is git repo",
			worktreeDir: "",
			expectEmpty: false, // Test runs in git repo
		},
		{
			name:        "valid worktree dir that is git repo",
			worktreeDir: ".",
			expectEmpty: false,
		},
		{
			name:        "non-existent worktree dir falls back to current dir",
			worktreeDir: "/non/existent/path",
			expectEmpty: false, // Falls back to current dir which is a git repo
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineRepoPath(tt.worktreeDir)
			if tt.expectEmpty {
				assert.Empty(t, result)
			} else {
				assert.NotEmpty(t, result)
			}
		})
	}
}

func TestRunGitConfig(t *testing.T) {
	// Test with real git command (basic smoke test)
	t.Run("real git config call", func(t *testing.T) {
		// This should not error even if no lw.* configs are set
		output, err := runGitConfig([]string{"config", "--global", "--get-regexp", "^lw\\."}, "")
		// Either returns empty string (no config) or actual config
		require.NoError(t, err)
		assert.True(t, output == "" || strings.Contains(output, "lw."))
	})

	t.Run("mock returns output", func(t *testing.T) {
		defer func() { gitConfigMock = nil }()

		gitConfigMock = func(args []string, repoPath string) (string, error) {
			return "lw.theme nord\n", nil
		}

		output, err := runGitConfig([]string{"config"}, "")
		require.NoError(t, err)
		assert.Equal(t, "lw.theme nord\n", output)
	})
}
