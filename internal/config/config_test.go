package config

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.NotNil(t, cfg)
	assert.Equal(t, "switched", cfg.SortMode)
	assert.False(t, cfg.AutoFetchPRs)
	assert.False(t, cfg.SearchAutoSelect)
	assert.Equal(t, 10, cfg.MaxUntrackedDiffs)
	assert.Equal(t, 200000, cfg.MaxDiffChars)
	assert.Equal(t, []string{"--syntax-theme", "Dracula"}, cfg.GitPagerArgs)
	assert.Equal(t, "delta", cfg.GitPager)
	assert.Equal(t, "tofu", cfg.TrustMode)
	assert.Equal(t, "rebase", cfg.MergeMethod)
	assert.Equal(t, "nerd-font-v3", cfg.IconSet)
	assert.Empty(t, cfg.WorktreeDir)
	assert.Empty(t, cfg.InitCommands)
	assert.Empty(t, cfg.TerminateCommands)
	assert.Empty(t, cfg.DebugLog)
	assert.NotNil(t, cfg.CustomCommands)
	require.Contains(t, cfg.CustomCommands, "t")
	assert.Equal(t, "Tmux", cfg.CustomCommands["t"].Description)
	require.Contains(t, cfg.CustomCommands, "Z")
	assert.Equal(t, "Zellij", cfg.CustomCommands["Z"].Description)
	assert.Empty(t, cfg.BranchNameScript)
}

func TestSyntaxThemeForUITheme(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		inputTheme string
		want       string
	}{
		{name: "default dracula", inputTheme: "dracula", want: "Dracula"},
		{name: "dracula-light", inputTheme: "dracula-light", want: "\"Monokai Extended Light\""},
		{name: "kanagawa", inputTheme: "kanagawa", want: "Dracula"},
		{name: "narna", inputTheme: "narna", want: "\"OneHalfDark\""},
		{name: "clean-light", inputTheme: "clean-light", want: "GitHub"},
		{name: "solarized-dark", inputTheme: "solarized-dark", want: "\"Solarized (dark)\""},
		{name: "solarized-light", inputTheme: "solarized-light", want: "\"Solarized (light)\""},
		{name: "gruvbox-dark", inputTheme: "gruvbox-dark", want: "\"Gruvbox Dark\""},
		{name: "gruvbox-light", inputTheme: "gruvbox-light", want: "\"Gruvbox Light\""},
		{name: "nord", inputTheme: "nord", want: "\"Nord\""},
		{name: "monokai", inputTheme: "monokai", want: "\"Monokai Extended\""},
		{name: "catppuccin-mocha", inputTheme: "catppuccin-mocha", want: "\"Catppuccin Mocha\""},
		{name: "catppuccin-latte", inputTheme: "catppuccin-latte", want: "\"Catppuccin Latte\""},
		{name: "rose-pine-dawn", inputTheme: "rose-pine-dawn", want: "GitHub"},
		{name: "one-light", inputTheme: "one-light", want: "\"OneHalfLight\""},
		{name: "everforest-light", inputTheme: "everforest-light", want: "\"Gruvbox Light\""},
		{name: "unknown falls back to dracula", inputTheme: "unknown", want: "Dracula"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, SyntaxThemeForUITheme(tt.inputTheme))
		})
	}
}

func TestNormalizeThemeName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "lowercase", input: "dracula", want: "dracula"},
		{name: "kanagawa", input: "kanagawa", want: "kanagawa"},
		{name: "uppercase", input: "NARNA", want: "narna"},
		{name: "trimmed whitespace", input: "  clean-light  ", want: "clean-light"},
		{name: "unknown theme", input: "invalid", want: ""},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, NormalizeThemeName(tt.input))
		})
	}
}

func TestNormalizeArgsList(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected []string
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: []string{},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "whitespace only string",
			input:    "   ",
			expected: []string{},
		},
		{
			name:     "string with multiple args",
			input:    "--syntax-theme Dracula --paging=never",
			expected: []string{"--syntax-theme", "Dracula", "--paging=never"},
		},
		{
			name:     "list with multiple args",
			input:    []interface{}{"--syntax-theme", "Dracula"},
			expected: []string{"--syntax-theme", "Dracula"},
		},
		{
			name:     "list with empty elements",
			input:    []interface{}{"--syntax-theme", "", nil, "Dracula"},
			expected: []string{"--syntax-theme", "Dracula"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeArgsList(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizeCommandList(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected []string
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: []string{},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "whitespace only string",
			input:    "   ",
			expected: []string{},
		},
		{
			name:     "single command string",
			input:    "echo hello",
			expected: []string{"echo hello"},
		},
		{
			name:     "trimmed string",
			input:    "  echo hello  ",
			expected: []string{"echo hello"},
		},
		{
			name:     "empty list",
			input:    []interface{}{},
			expected: []string{},
		},
		{
			name:     "list with single command",
			input:    []interface{}{"echo hello"},
			expected: []string{"echo hello"},
		},
		{
			name:     "list with multiple commands",
			input:    []interface{}{"echo hello", "ls -la", "pwd"},
			expected: []string{"echo hello", "ls -la", "pwd"},
		},
		{
			name:     "list with nil elements",
			input:    []interface{}{"echo hello", nil, "pwd"},
			expected: []string{"echo hello", "pwd"},
		},
		{
			name:     "list with empty strings",
			input:    []interface{}{"echo hello", "", "pwd"},
			expected: []string{"echo hello", "pwd"},
		},
		{
			name:     "list with whitespace strings",
			input:    []interface{}{"echo hello", "   ", "pwd"},
			expected: []string{"echo hello", "pwd"},
		},
		{
			name:     "list with trimmed strings",
			input:    []interface{}{"  echo hello  ", "  pwd  "},
			expected: []string{"echo hello", "pwd"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeCommandList(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCoerceBool(t *testing.T) {
	tests := []struct {
		name       string
		input      interface{}
		defaultVal bool
		expected   bool
	}{
		{
			name:       "nil with default true",
			input:      nil,
			defaultVal: true,
			expected:   true,
		},
		{
			name:       "nil with default false",
			input:      nil,
			defaultVal: false,
			expected:   false,
		},
		{
			name:       "bool true",
			input:      true,
			defaultVal: false,
			expected:   true,
		},
		{
			name:       "bool false",
			input:      false,
			defaultVal: true,
			expected:   false,
		},
		{
			name:       "int 1",
			input:      1,
			defaultVal: false,
			expected:   true,
		},
		{
			name:       "int 0",
			input:      0,
			defaultVal: true,
			expected:   false,
		},
		{
			name:       "int non-zero",
			input:      42,
			defaultVal: false,
			expected:   true,
		},
		{
			name:       "string true",
			input:      "true",
			defaultVal: false,
			expected:   true,
		},
		{
			name:       "string false",
			input:      "false",
			defaultVal: true,
			expected:   false,
		},
		{
			name:       "string 1",
			input:      "1",
			defaultVal: false,
			expected:   true,
		},
		{
			name:       "string 0",
			input:      "0",
			defaultVal: true,
			expected:   false,
		},
		{
			name:       "string yes",
			input:      "yes",
			defaultVal: false,
			expected:   true,
		},
		{
			name:       "string no",
			input:      "no",
			defaultVal: true,
			expected:   false,
		},
		{
			name:       "string y",
			input:      "y",
			defaultVal: false,
			expected:   true,
		},
		{
			name:       "string n",
			input:      "n",
			defaultVal: true,
			expected:   false,
		},
		{
			name:       "string on",
			input:      "on",
			defaultVal: false,
			expected:   true,
		},
		{
			name:       "string off",
			input:      "off",
			defaultVal: true,
			expected:   false,
		},
		{
			name:       "string with whitespace",
			input:      "  true  ",
			defaultVal: false,
			expected:   true,
		},
		{
			name:       "string uppercase",
			input:      "TRUE",
			defaultVal: false,
			expected:   true,
		},
		{
			name:       "invalid string",
			input:      "invalid",
			defaultVal: true,
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := coerceBool(tt.input, tt.defaultVal)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCoerceInt(t *testing.T) {
	tests := []struct {
		name       string
		input      interface{}
		defaultVal int
		expected   int
	}{
		{
			name:       "nil with default",
			input:      nil,
			defaultVal: 42,
			expected:   42,
		},
		{
			name:       "int value",
			input:      123,
			defaultVal: 42,
			expected:   123,
		},
		{
			name:       "bool (should return default)",
			input:      true,
			defaultVal: 42,
			expected:   42,
		},
		{
			name:       "string number",
			input:      "123",
			defaultVal: 42,
			expected:   123,
		},
		{
			name:       "string with whitespace",
			input:      "  456  ",
			defaultVal: 42,
			expected:   456,
		},
		{
			name:       "empty string",
			input:      "",
			defaultVal: 42,
			expected:   42,
		},
		{
			name:       "invalid string",
			input:      "abc",
			defaultVal: 42,
			expected:   42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := coerceInt(tt.input, tt.defaultVal)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name        string
		data        map[string]interface{}
		validate    func(*testing.T, *AppConfig)
		expectError bool
		errContains string
	}{
		{
			name: "empty config uses defaults",
			data: map[string]interface{}{},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "switched", cfg.SortMode)
				assert.False(t, cfg.AutoFetchPRs)
				assert.False(t, cfg.SearchAutoSelect)
				assert.Equal(t, 10, cfg.MaxUntrackedDiffs)
				assert.Equal(t, 200000, cfg.MaxDiffChars)
				assert.Equal(t, []string{"--syntax-theme", "Dracula"}, cfg.GitPagerArgs)
				assert.Equal(t, "tofu", cfg.TrustMode)
			},
		},
		{
			name: "worktree_dir",
			data: map[string]interface{}{
				"worktree_dir": "/custom/path",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "/custom/path", cfg.WorktreeDir)
			},
		},
		{
			name: "debug_log",
			data: map[string]interface{}{
				"debug_log": "/tmp/debug.log",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "/tmp/debug.log", cfg.DebugLog)
			},
		},
		{
			name: "init_commands string",
			data: map[string]interface{}{
				"init_commands": "echo hello",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, []string{"echo hello"}, cfg.InitCommands)
			},
		},
		{
			name: "init_commands list",
			data: map[string]interface{}{
				"init_commands": []interface{}{"echo hello", "pwd"},
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, []string{"echo hello", "pwd"}, cfg.InitCommands)
			},
		},
		{
			name: "terminate_commands",
			data: map[string]interface{}{
				"terminate_commands": []interface{}{"cleanup", "exit"},
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, []string{"cleanup", "exit"}, cfg.TerminateCommands)
			},
		},
		{
			name: "sort_by_active false",
			data: map[string]interface{}{
				"sort_by_active": false,
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "path", cfg.SortMode)
			},
		},
		{
			name: "auto_fetch_prs true",
			data: map[string]interface{}{
				"auto_fetch_prs": true,
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.True(t, cfg.AutoFetchPRs)
			},
		},
		{
			name: "search_auto_select true",
			data: map[string]interface{}{
				"search_auto_select": true,
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.True(t, cfg.SearchAutoSelect)
			},
		},
		{
			name: "icon_set nerd-font-v3",
			data: map[string]interface{}{
				"icon_set": "nerd-font-v3",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "nerd-font-v3", cfg.IconSet)
			},
		},
		{
			name: "icon_set none",
			data: map[string]interface{}{
				"icon_set": "none",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "none", cfg.IconSet)
			},
		},
		{
			name: "icon_set empty disables icons",
			data: map[string]interface{}{
				"icon_set": "",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "none", cfg.IconSet)
			},
		},
		{
			name: "icon_set emoji",
			data: map[string]interface{}{
				"icon_set": "emoji",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "emoji", cfg.IconSet)
			},
		},
		{
			name: "icon_set text",
			data: map[string]interface{}{
				"icon_set": "text",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "text", cfg.IconSet)
			},
		},
		{
			name: "icon_set invalid returns error",
			data: map[string]interface{}{
				"icon_set": "invalid",
			},
			expectError: true,
			errContains: "available: nerd-font-v3, emoji, text, none",
		},
		{
			name: "max_untracked_diffs",
			data: map[string]interface{}{
				"max_untracked_diffs": 20,
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, 20, cfg.MaxUntrackedDiffs)
			},
		},
		{
			name: "max_diff_chars",
			data: map[string]interface{}{
				"max_diff_chars": 100000,
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, 100000, cfg.MaxDiffChars)
			},
		},
		{
			name: "git_pager_args string",
			data: map[string]interface{}{
				"git_pager_args": "--syntax-theme Dracula",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.True(t, cfg.GitPagerArgsSet)
				assert.Equal(t, []string{"--syntax-theme", "Dracula"}, cfg.GitPagerArgs)
			},
		},
		{
			name: "git_pager_args list",
			data: map[string]interface{}{
				"git_pager_args": []interface{}{"--syntax-theme", "Dracula"},
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, []string{"--syntax-theme", "Dracula"}, cfg.GitPagerArgs)
			},
		},
		{
			name: "git_pager_args empty list",
			data: map[string]interface{}{
				"git_pager_args": []interface{}{},
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Empty(t, cfg.GitPagerArgs)
			},
		},
		{
			name: "delta_args legacy key",
			data: map[string]interface{}{
				"delta_args": "--syntax-theme Dracula",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.True(t, cfg.GitPagerArgsSet)
				assert.Equal(t, []string{"--syntax-theme", "Dracula"}, cfg.GitPagerArgs)
			},
		},
		{
			name: "non-delta git_pager clears default args",
			data: map[string]interface{}{
				"git_pager": "diff-so-fancy",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "diff-so-fancy", cfg.GitPager)
				assert.Empty(t, cfg.GitPagerArgs)
				assert.False(t, cfg.GitPagerArgsSet)
			},
		},
		{
			name: "theme narna sets default delta args when unset",
			data: map[string]interface{}{
				"theme": "narna",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "narna", cfg.Theme)
				assert.Equal(t, []string{"--syntax-theme", "\"OneHalfDark\""}, cfg.GitPagerArgs)
			},
		},
		{
			name: "theme clean-light sets default delta args when unset",
			data: map[string]interface{}{
				"theme": "clean-light",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "clean-light", cfg.Theme)
				assert.Equal(t, []string{"--syntax-theme", "GitHub"}, cfg.GitPagerArgs)
			},
		},
		{
			name: "theme solarized-dark sets default delta args when unset",
			data: map[string]interface{}{
				"theme": "solarized-dark",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "solarized-dark", cfg.Theme)
				assert.Equal(t, []string{"--syntax-theme", "\"Solarized (dark)\""}, cfg.GitPagerArgs)
			},
		},
		{
			name: "theme solarized-light sets default delta args when unset",
			data: map[string]interface{}{
				"theme": "solarized-light",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "solarized-light", cfg.Theme)
				assert.Equal(t, []string{"--syntax-theme", "\"Solarized (light)\""}, cfg.GitPagerArgs)
			},
		},
		{
			name: "theme gruvbox-dark sets default delta args when unset",
			data: map[string]interface{}{
				"theme": "gruvbox-dark",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "gruvbox-dark", cfg.Theme)
				assert.Equal(t, []string{"--syntax-theme", "\"Gruvbox Dark\""}, cfg.GitPagerArgs)
			},
		},
		{
			name: "theme gruvbox-light sets default delta args when unset",
			data: map[string]interface{}{
				"theme": "gruvbox-light",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "gruvbox-light", cfg.Theme)
				assert.Equal(t, []string{"--syntax-theme", "\"Gruvbox Light\""}, cfg.GitPagerArgs)
			},
		},
		{
			name: "theme nord sets default delta args when unset",
			data: map[string]interface{}{
				"theme": "nord",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "nord", cfg.Theme)
				assert.Equal(t, []string{"--syntax-theme", "\"Nord\""}, cfg.GitPagerArgs)
			},
		},
		{
			name: "theme monokai sets default delta args when unset",
			data: map[string]interface{}{
				"theme": "monokai",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "monokai", cfg.Theme)
				assert.Equal(t, []string{"--syntax-theme", "\"Monokai Extended\""}, cfg.GitPagerArgs)
			},
		},
		{
			name: "theme catppuccin-mocha sets default delta args when unset",
			data: map[string]interface{}{
				"theme": "catppuccin-mocha",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "catppuccin-mocha", cfg.Theme)
				assert.Equal(t, []string{"--syntax-theme", "\"Catppuccin Mocha\""}, cfg.GitPagerArgs)
			},
		},
		{
			name: "theme catppuccin-latte sets default delta args when unset",
			data: map[string]interface{}{
				"theme": "catppuccin-latte",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "catppuccin-latte", cfg.Theme)
				assert.Equal(t, []string{"--syntax-theme", "\"Catppuccin Latte\""}, cfg.GitPagerArgs)
			},
		},
		{
			name: "theme rose-pine-dawn sets default delta args when unset",
			data: map[string]interface{}{
				"theme": "rose-pine-dawn",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "rose-pine-dawn", cfg.Theme)
				assert.Equal(t, []string{"--syntax-theme", "GitHub"}, cfg.GitPagerArgs)
			},
		},
		{
			name: "theme one-light sets default delta args when unset",
			data: map[string]interface{}{
				"theme": "one-light",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "one-light", cfg.Theme)
				assert.Equal(t, []string{"--syntax-theme", "\"OneHalfLight\""}, cfg.GitPagerArgs)
			},
		},
		{
			name: "theme everforest-light sets default delta args when unset",
			data: map[string]interface{}{
				"theme": "everforest-light",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "everforest-light", cfg.Theme)
				assert.Equal(t, []string{"--syntax-theme", "\"Gruvbox Light\""}, cfg.GitPagerArgs)
			},
		},
		{
			name: "custom git_pager_args not overridden by theme",
			data: map[string]interface{}{
				"theme":          "narna",
				"git_pager_args": "--syntax-theme Nord",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "narna", cfg.Theme)
				assert.Equal(t, []string{"--syntax-theme", "Nord"}, cfg.GitPagerArgs)
			},
		},
		{
			name: "custom delta_args legacy key not overridden by theme",
			data: map[string]interface{}{
				"theme":      "narna",
				"delta_args": "--syntax-theme Nord",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "narna", cfg.Theme)
				assert.Equal(t, []string{"--syntax-theme", "Nord"}, cfg.GitPagerArgs)
			},
		},
		{
			name: "negative max_untracked_diffs becomes 0",
			data: map[string]interface{}{
				"max_untracked_diffs": -5,
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, 0, cfg.MaxUntrackedDiffs)
			},
		},
		{
			name: "negative max_diff_chars becomes 0",
			data: map[string]interface{}{
				"max_diff_chars": -1000,
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, 0, cfg.MaxDiffChars)
			},
		},
		{
			name: "trust_mode tofu",
			data: map[string]interface{}{
				"trust_mode": "tofu",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "tofu", cfg.TrustMode)
			},
		},
		{
			name: "trust_mode never",
			data: map[string]interface{}{
				"trust_mode": "never",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "never", cfg.TrustMode)
			},
		},
		{
			name: "trust_mode always",
			data: map[string]interface{}{
				"trust_mode": "always",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "always", cfg.TrustMode)
			},
		},
		{
			name: "trust_mode uppercase converted to lowercase",
			data: map[string]interface{}{
				"trust_mode": "TOFU",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "tofu", cfg.TrustMode)
			},
		},
		{
			name: "invalid trust_mode uses default",
			data: map[string]interface{}{
				"trust_mode": "invalid",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "tofu", cfg.TrustMode)
			},
		},
		{
			name: "branch_name_script",
			data: map[string]interface{}{
				"branch_name_script": "echo feature/test",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "echo feature/test", cfg.BranchNameScript)
			},
		},
		{
			name: "branch_name_script empty string",
			data: map[string]interface{}{
				"branch_name_script": "",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Empty(t, cfg.BranchNameScript)
			},
		},
		{
			name: "branch_name_script with spaces is trimmed",
			data: map[string]interface{}{
				"branch_name_script": "   echo test   ",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "echo test", cfg.BranchNameScript)
			},
		},
		{
			name: "editor config is trimmed",
			data: map[string]interface{}{
				"editor": "  nvim -u NORC  ",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "nvim -u NORC", cfg.Editor)
			},
		},
		{
			name: "merge_method rebase",
			data: map[string]interface{}{
				"merge_method": "rebase",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "rebase", cfg.MergeMethod)
			},
		},
		{
			name: "merge_method merge",
			data: map[string]interface{}{
				"merge_method": "merge",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "merge", cfg.MergeMethod)
			},
		},
		{
			name: "merge_method uppercase converted to lowercase",
			data: map[string]interface{}{
				"merge_method": "REBASE",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "rebase", cfg.MergeMethod)
			},
		},
		{
			name: "invalid merge_method uses default",
			data: map[string]interface{}{
				"merge_method": "invalid",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "rebase", cfg.MergeMethod)
			},
		},
		{
			name: "git_pager default",
			data: map[string]interface{}{},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "delta", cfg.GitPager)
			},
		},
		{
			name: "git_pager custom",
			data: map[string]interface{}{
				"git_pager": "/usr/local/bin/delta",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "/usr/local/bin/delta", cfg.GitPager)
			},
		},
		{
			name: "git_pager empty disables",
			data: map[string]interface{}{
				"git_pager": "",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Empty(t, cfg.GitPager)
			},
		},
		{
			name: "git_pager with whitespace is trimmed",
			data: map[string]interface{}{
				"git_pager": "  delta  ",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "delta", cfg.GitPager)
			},
		},
		{
			name: "delta_path legacy key",
			data: map[string]interface{}{
				"delta_path": "/usr/local/bin/delta",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "/usr/local/bin/delta", cfg.GitPager)
			},
		},
		{
			name: "git_pager non-delta without args clears inherited delta args",
			data: map[string]interface{}{
				"git_pager": "diff-so-fancy",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "diff-so-fancy", cfg.GitPager)
				assert.Nil(t, cfg.GitPagerArgs)
				assert.False(t, cfg.GitPagerArgsSet)
			},
		},
		{
			name: "git_pager non-delta with explicit args uses those args",
			data: map[string]interface{}{
				"git_pager":      "diff-so-fancy",
				"git_pager_args": "--color always",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "diff-so-fancy", cfg.GitPager)
				assert.Equal(t, []string{"--color", "always"}, cfg.GitPagerArgs)
				assert.True(t, cfg.GitPagerArgsSet)
			},
		},
		{
			name: "git_pager_interactive true",
			data: map[string]interface{}{
				"git_pager_interactive": true,
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.True(t, cfg.GitPagerInteractive)
			},
		},
		{
			name: "git_pager_interactive false",
			data: map[string]interface{}{
				"git_pager_interactive": false,
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.False(t, cfg.GitPagerInteractive)
			},
		},
		{
			name: "git_pager_interactive defaults to false",
			data: map[string]interface{}{},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.False(t, cfg.GitPagerInteractive)
			},
		},
		{
			name: "custom_create_menus parsing",
			data: map[string]interface{}{
				"custom_create_menus": []interface{}{
					map[string]interface{}{
						"label":       "From JIRA",
						"description": "Create from JIRA ticket",
						"command":     "jayrah browse SRVKP --choose",
						"interactive": true,
					},
					map[string]interface{}{
						"label":   "Quick create",
						"command": "echo feature-branch",
					},
				},
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				require.Len(t, cfg.CustomCreateMenus, 2)
				assert.Equal(t, "From JIRA", cfg.CustomCreateMenus[0].Label)
				assert.Equal(t, "Create from JIRA ticket", cfg.CustomCreateMenus[0].Description)
				assert.Equal(t, "jayrah browse SRVKP --choose", cfg.CustomCreateMenus[0].Command)
				assert.True(t, cfg.CustomCreateMenus[0].Interactive)
				assert.Equal(t, "Quick create", cfg.CustomCreateMenus[1].Label)
				assert.Empty(t, cfg.CustomCreateMenus[1].Description)
				assert.Equal(t, "echo feature-branch", cfg.CustomCreateMenus[1].Command)
				assert.False(t, cfg.CustomCreateMenus[1].Interactive)
			},
		},
		{
			name: "custom_create_menus with post_command",
			data: map[string]interface{}{
				"custom_create_menus": []interface{}{
					map[string]interface{}{
						"label":            "JIRA with commit",
						"command":          "jayrah browse SRVKP --choose",
						"interactive":      true,
						"post_command":     "git commit --allow-empty -m 'Initial commit'",
						"post_interactive": false,
					},
					map[string]interface{}{
						"label":            "Feature with editor",
						"command":          "echo feature-xyz",
						"post_command":     "$EDITOR README.md",
						"post_interactive": true,
					},
				},
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				require.Len(t, cfg.CustomCreateMenus, 2)
				assert.Equal(t, "JIRA with commit", cfg.CustomCreateMenus[0].Label)
				assert.Equal(t, "git commit --allow-empty -m 'Initial commit'", cfg.CustomCreateMenus[0].PostCommand)
				assert.False(t, cfg.CustomCreateMenus[0].PostInteractive)
				assert.Equal(t, "Feature with editor", cfg.CustomCreateMenus[1].Label)
				assert.Equal(t, "$EDITOR README.md", cfg.CustomCreateMenus[1].PostCommand)
				assert.True(t, cfg.CustomCreateMenus[1].PostInteractive)
			},
		},
		{
			name: "custom_create_menus skips invalid entries",
			data: map[string]interface{}{
				"custom_create_menus": []interface{}{
					map[string]interface{}{
						"label": "No command",
					},
					map[string]interface{}{
						"command": "echo test",
					},
					map[string]interface{}{
						"label":   "Valid",
						"command": "echo valid",
					},
				},
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				require.Len(t, cfg.CustomCreateMenus, 1)
				assert.Equal(t, "Valid", cfg.CustomCreateMenus[0].Label)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := parseConfig(tt.data)
			if tt.expectError {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, cfg)
			tt.validate(t, cfg)
		})
	}
}

func TestLoadRepoConfig(t *testing.T) {
	t.Run("empty repo path", func(t *testing.T) {
		cfg, path, err := LoadRepoConfig("")
		require.Error(t, err)
		assert.Nil(t, cfg)
		assert.Empty(t, path)
	})

	t.Run("non-existent .wt file", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfg, path, err := LoadRepoConfig(tmpDir)
		require.NoError(t, err)
		assert.Nil(t, cfg)
		assert.Equal(t, filepath.Join(tmpDir, ".wt"), path)
	})

	t.Run("valid .wt file", func(t *testing.T) {
		tmpDir := t.TempDir()
		wtPath := filepath.Join(tmpDir, ".wt")

		yamlContent := `init_commands:
  - echo "init"
  - pwd
terminate_commands:
  - echo "terminate"
`
		err := os.WriteFile(wtPath, []byte(yamlContent), 0o600)
		require.NoError(t, err)

		cfg, path, err := LoadRepoConfig(tmpDir)
		require.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.Equal(t, wtPath, path)
		assert.Equal(t, wtPath, cfg.Path)
		assert.Equal(t, []string{"echo \"init\"", "pwd"}, cfg.InitCommands)
		assert.Equal(t, []string{"echo \"terminate\""}, cfg.TerminateCommands)
	})

	t.Run("invalid YAML in .wt file", func(t *testing.T) {
		tmpDir := t.TempDir()
		wtPath := filepath.Join(tmpDir, ".wt")

		err := os.WriteFile(wtPath, []byte("invalid: yaml: content: [[["), 0o600)
		require.NoError(t, err)

		cfg, path, err := LoadRepoConfig(tmpDir)
		require.Error(t, err)
		assert.Nil(t, cfg)
		assert.Equal(t, wtPath, path)
	})
}

func TestLoadConfig(t *testing.T) {
	// Setup mock to prevent loading real git config from the test repository
	defer func() { gitConfigMock = nil }()
	gitConfigMock = func(args []string, repoPath string) (string, error) {
		// Return empty config for all git config calls in these tests
		return "", nil
	}

	t.Run("no config file returns defaults", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", tmpDir)
		configDir := filepath.Join(tmpDir, "lazyworktree")
		configPath := filepath.Join(configDir, "nonexistent.yaml")

		require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o750))

		cfg, err := LoadConfig(configPath)
		require.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.Equal(t, DefaultConfig().SortMode, cfg.SortMode)
		assert.Equal(t, DefaultConfig().MaxUntrackedDiffs, cfg.MaxUntrackedDiffs)
		assert.Equal(t, DefaultConfig().GitPagerArgs, cfg.GitPagerArgs)
	})

	t.Run("valid config file", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", tmpDir)
		configDir := filepath.Join(tmpDir, "lazyworktree")
		configPath := filepath.Join(configDir, "config.yaml")

		yamlContent := `worktree_dir: /custom/worktrees
sort_by_active: false
auto_fetch_prs: true
max_untracked_diffs: 20
max_diff_chars: 100000
git_pager: delta
git_pager_args:
  - --syntax-theme
  - Dracula
trust_mode: always
init_commands:
  - echo "init"
terminate_commands:
  - echo "cleanup"
`
		require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o750))
		err := os.WriteFile(configPath, []byte(yamlContent), 0o600)
		require.NoError(t, err)

		cfg, err := LoadConfig(configPath)
		require.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.Equal(t, "/custom/worktrees", cfg.WorktreeDir)
		assert.Equal(t, "path", cfg.SortMode)
		assert.True(t, cfg.AutoFetchPRs)
		assert.Equal(t, 20, cfg.MaxUntrackedDiffs)
		assert.Equal(t, 100000, cfg.MaxDiffChars)
		assert.Equal(t, []string{"--syntax-theme", "Dracula"}, cfg.GitPagerArgs)
		assert.Equal(t, "always", cfg.TrustMode)
		assert.Equal(t, []string{"echo \"init\""}, cfg.InitCommands)
		assert.Equal(t, []string{"echo \"cleanup\""}, cfg.TerminateCommands)
	})

	t.Run("non-delta git_pager uses no args", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", tmpDir)
		configDir := filepath.Join(tmpDir, "lazyworktree")
		configPath := filepath.Join(configDir, "config.yaml")

		yamlContent := `git_pager: diff-so-fancy`
		require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o750))
		err := os.WriteFile(configPath, []byte(yamlContent), 0o600)
		require.NoError(t, err)

		cfg, err := LoadConfig(configPath)
		require.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.Equal(t, "diff-so-fancy", cfg.GitPager)
		assert.Empty(t, cfg.GitPagerArgs)
	})

	t.Run("invalid YAML returns defaults", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", tmpDir)
		configDir := filepath.Join(tmpDir, "lazyworktree")
		configPath := filepath.Join(configDir, "config.yaml")

		require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o750))
		err := os.WriteFile(configPath, []byte("invalid: [[["), 0o600)
		require.NoError(t, err)

		cfg, err := LoadConfig(configPath)
		require.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.Equal(t, DefaultConfig().SortMode, cfg.SortMode)
	})
}

func TestParseCustomCommands(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]*CustomCommand
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: map[string]*CustomCommand{},
		},
		{
			name:     "empty map",
			input:    map[string]interface{}{},
			expected: map[string]*CustomCommand{},
		},
		{
			name: "single command with all fields",
			input: map[string]interface{}{
				"custom_commands": map[string]interface{}{
					"e": map[string]interface{}{
						"command":     "nvim",
						"description": "Open editor",
						"show_help":   true,
						"wait":        true,
						"show_output": true,
					},
				},
			},
			expected: map[string]*CustomCommand{
				"e": {
					Command:     "nvim",
					Description: "Open editor",
					ShowHelp:    true,
					Wait:        true,
					ShowOutput:  true,
				},
			},
		},
		{
			name: "multiple commands",
			input: map[string]interface{}{
				"custom_commands": map[string]interface{}{
					"e": map[string]interface{}{
						"command":     "nvim",
						"description": "Open editor",
						"show_help":   true,
					},
					"s": map[string]interface{}{
						"command":     "zsh",
						"description": "Open shell",
						"show_help":   false,
					},
				},
			},
			expected: map[string]*CustomCommand{
				"e": {
					Command:     "nvim",
					Description: "Open editor",
					ShowHelp:    true,
					Wait:        false,
				},
				"s": {
					Command:     "zsh",
					Description: "Open shell",
					ShowHelp:    false,
					Wait:        false,
				},
			},
		},
		{
			name: "command with spaces trimmed",
			input: map[string]interface{}{
				"custom_commands": map[string]interface{}{
					"t": map[string]interface{}{
						"command":     "  make test  ",
						"description": "  Run tests  ",
					},
				},
			},
			expected: map[string]*CustomCommand{
				"t": {
					Command:     "make test",
					Description: "Run tests",
					ShowHelp:    false,
					Wait:        false,
				},
			},
		},
		{
			name: "empty command is skipped",
			input: map[string]interface{}{
				"custom_commands": map[string]interface{}{
					"e": map[string]interface{}{
						"command":     "",
						"description": "Empty command",
					},
				},
			},
			expected: map[string]*CustomCommand{},
		},
		{
			name: "command with only whitespace is skipped",
			input: map[string]interface{}{
				"custom_commands": map[string]interface{}{
					"e": map[string]interface{}{
						"command":     "   ",
						"description": "Whitespace command",
					},
				},
			},
			expected: map[string]*CustomCommand{},
		},
		{
			name: "tmux command with windows",
			input: map[string]interface{}{
				"custom_commands": map[string]interface{}{
					"x": map[string]interface{}{
						"description": "Tmux",
						"show_help":   true,
						"tmux": map[string]interface{}{
							"session_name": "${REPO_NAME}_wt_$WORKTREE_NAME",
							"attach":       false,
							"on_exists":    "kill",
							"windows": []interface{}{
								map[string]interface{}{
									"name":    "shell",
									"command": "zsh",
									"cwd":     "$WORKTREE_PATH",
								},
								map[string]interface{}{
									"name":    "git",
									"command": "lazygit",
								},
							},
						},
					},
				},
			},
			expected: map[string]*CustomCommand{
				"x": {
					Command:     "",
					Description: "Tmux",
					ShowHelp:    true,
					Wait:        false,
					Tmux: &TmuxCommand{
						SessionName: "${REPO_NAME}_wt_$WORKTREE_NAME",
						Attach:      false,
						OnExists:    "kill",
						Windows: []TmuxWindow{
							{Name: "shell", Command: "zsh", Cwd: "$WORKTREE_PATH"},
							{Name: "git", Command: "lazygit", Cwd: ""},
						},
					},
				},
			},
		},
		{
			name: "zellij command with windows",
			input: map[string]interface{}{
				"custom_commands": map[string]interface{}{
					"z": map[string]interface{}{
						"description": "Zellij",
						"show_help":   true,
						"zellij": map[string]interface{}{
							"session_name": "${REPO_NAME}_wt_$WORKTREE_NAME",
							"attach":       false,
							"on_exists":    "kill",
							"windows": []interface{}{
								map[string]interface{}{
									"name":    "shell",
									"command": "zsh",
									"cwd":     "$WORKTREE_PATH",
								},
								map[string]interface{}{
									"name":    "git",
									"command": "lazygit",
								},
							},
						},
					},
				},
			},
			expected: map[string]*CustomCommand{
				"z": {
					Command:     "",
					Description: "Zellij",
					ShowHelp:    true,
					Wait:        false,
					Zellij: &TmuxCommand{
						SessionName: "${REPO_NAME}_wt_$WORKTREE_NAME",
						Attach:      false,
						OnExists:    "kill",
						Windows: []TmuxWindow{
							{Name: "shell", Command: "zsh", Cwd: "$WORKTREE_PATH"},
							{Name: "git", Command: "lazygit", Cwd: ""},
						},
					},
				},
			},
		},
		{
			name: "tmux without windows defaults to shell window",
			input: map[string]interface{}{
				"custom_commands": map[string]interface{}{
					"x": map[string]interface{}{
						"tmux": map[string]interface{}{
							"session_name": "${REPO_NAME}_wt_$WORKTREE_NAME",
						},
					},
				},
			},
			expected: map[string]*CustomCommand{
				"x": {
					Command:     "",
					Description: "",
					ShowHelp:    false,
					Wait:        false,
					Tmux: &TmuxCommand{
						SessionName: "${REPO_NAME}_wt_$WORKTREE_NAME",
						Attach:      true,
						OnExists:    "switch",
						Windows: []TmuxWindow{
							{Name: "shell", Command: "", Cwd: ""},
						},
					},
				},
			},
		},
		{
			name: "invalid type for custom_commands is ignored",
			input: map[string]interface{}{
				"custom_commands": "not a map",
			},
			expected: map[string]*CustomCommand{},
		},
		{
			name: "invalid type for command entry is skipped",
			input: map[string]interface{}{
				"custom_commands": map[string]interface{}{
					"e": "not a map",
					"s": map[string]interface{}{
						"command": "zsh",
					},
				},
			},
			expected: map[string]*CustomCommand{
				"s": {
					Command:     "zsh",
					Description: "",
					ShowHelp:    false,
					Wait:        false,
				},
			},
		},
		{
			name: "boolean coercion for show_help",
			input: map[string]interface{}{
				"custom_commands": map[string]interface{}{
					"a": map[string]interface{}{
						"command":   "cmd1",
						"show_help": "yes",
					},
					"b": map[string]interface{}{
						"command":   "cmd2",
						"show_help": "no",
					},
					"c": map[string]interface{}{
						"command":   "cmd3",
						"show_help": 1,
					},
					"d": map[string]interface{}{
						"command":   "cmd4",
						"show_help": 0,
					},
				},
			},
			expected: map[string]*CustomCommand{
				"a": {Command: "cmd1", ShowHelp: true, Wait: false},
				"b": {Command: "cmd2", ShowHelp: false, Wait: false},
				"c": {Command: "cmd3", ShowHelp: true, Wait: false},
				"d": {Command: "cmd4", ShowHelp: false, Wait: false},
			},
		},
		{
			name: "boolean coercion for wait",
			input: map[string]interface{}{
				"custom_commands": map[string]interface{}{
					"a": map[string]interface{}{
						"command": "cmd1",
						"wait":    "true",
					},
					"b": map[string]interface{}{
						"command": "cmd2",
						"wait":    "false",
					},
					"c": map[string]interface{}{
						"command": "cmd3",
						"wait":    1,
					},
				},
			},
			expected: map[string]*CustomCommand{
				"a": {Command: "cmd1", Wait: true},
				"b": {Command: "cmd2", Wait: false},
				"c": {Command: "cmd3", Wait: true},
			},
		},
		{
			name: "boolean coercion for show_output",
			input: map[string]interface{}{
				"custom_commands": map[string]interface{}{
					"a": map[string]interface{}{
						"command":     "cmd1",
						"show_output": "true",
					},
					"b": map[string]interface{}{
						"command":     "cmd2",
						"show_output": "false",
					},
					"c": map[string]interface{}{
						"command":     "cmd3",
						"show_output": 1,
					},
				},
			},
			expected: map[string]*CustomCommand{
				"a": {Command: "cmd1", ShowOutput: true},
				"b": {Command: "cmd2", ShowOutput: false},
				"c": {Command: "cmd3", ShowOutput: true},
			},
		},
		{
			name: "missing fields use defaults",
			input: map[string]interface{}{
				"custom_commands": map[string]interface{}{
					"e": map[string]interface{}{
						"command": "nvim",
					},
				},
			},
			expected: map[string]*CustomCommand{
				"e": {
					Command:     "nvim",
					Description: "",
					ShowHelp:    false,
					Wait:        false,
				},
			},
		},
		{
			name: "modifier keys (ctrl, alt, etc.)",
			input: map[string]interface{}{
				"custom_commands": map[string]interface{}{
					"ctrl+e": map[string]interface{}{
						"command":     "nvim",
						"description": "Open with Ctrl+E",
					},
					"alt+t": map[string]interface{}{
						"command":     "make test",
						"description": "Test with Alt+T",
					},
					"ctrl+shift+s": map[string]interface{}{
						"command": "git status",
					},
				},
			},
			expected: map[string]*CustomCommand{
				"ctrl+e": {
					Command:     "nvim",
					Description: "Open with Ctrl+E",
					ShowHelp:    false,
					Wait:        false,
				},
				"alt+t": {
					Command:     "make test",
					Description: "Test with Alt+T",
					ShowHelp:    false,
					Wait:        false,
				},
				"ctrl+shift+s": {
					Command:     "git status",
					Description: "",
					ShowHelp:    false,
					Wait:        false,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCustomCommands(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseConfig_CustomCommands(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		validate func(t *testing.T, cfg *AppConfig)
	}{
		{
			name: "custom commands in full config",
			input: map[string]interface{}{
				"worktree_dir": "/tmp/worktrees",
				"sort_mode":    "switched",
				"custom_commands": map[string]interface{}{
					"e": map[string]interface{}{
						"command":     "nvim",
						"description": "Open editor",
						"show_help":   true,
						"wait":        false,
					},
				},
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "/tmp/worktrees", cfg.WorktreeDir)
				assert.Equal(t, "switched", cfg.SortMode)
				require.Len(t, cfg.CustomCommands, 3)
				assert.Equal(t, "nvim", cfg.CustomCommands["e"].Command)
				assert.Equal(t, "Open editor", cfg.CustomCommands["e"].Description)
				assert.True(t, cfg.CustomCommands["e"].ShowHelp)
				assert.False(t, cfg.CustomCommands["e"].Wait)
				require.Contains(t, cfg.CustomCommands, "t")
				require.Contains(t, cfg.CustomCommands, "Z")
			},
		},
		{
			name: "no custom commands",
			input: map[string]interface{}{
				"worktree_dir": "/tmp/worktrees",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				require.Contains(t, cfg.CustomCommands, "t")
				assert.Equal(t, "Tmux", cfg.CustomCommands["t"].Description)
				require.Contains(t, cfg.CustomCommands, "Z")
				assert.Equal(t, "Zellij", cfg.CustomCommands["Z"].Description)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := parseConfig(tt.input)
			require.NoError(t, err)
			tt.validate(t, cfg)
		})
	}
}

func TestParseConfigPager(t *testing.T) {
	input := map[string]interface{}{
		"pager": "less -R",
	}
	cfg, err := parseConfig(input)
	require.NoError(t, err)
	assert.Equal(t, "less -R", cfg.Pager)
}

func TestParseConfigCIScriptPager(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected string
	}{
		{
			name:     "basic value",
			input:    map[string]interface{}{"ci_script_pager": "less -R"},
			expected: "less -R",
		},
		{
			name:     "empty string is ignored",
			input:    map[string]interface{}{"ci_script_pager": ""},
			expected: "",
		},
		{
			name:     "whitespace only is ignored",
			input:    map[string]interface{}{"ci_script_pager": "   "},
			expected: "",
		},
		{
			name:     "whitespace is trimmed",
			input:    map[string]interface{}{"ci_script_pager": "  bat --style=plain  "},
			expected: "bat --style=plain",
		},
		{
			name:     "not set uses default empty",
			input:    map[string]interface{}{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := parseConfig(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, cfg.CIScriptPager)
		})
	}
}

func TestSaveConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "lazyworktree", "config.yaml")

	// Create a file with comments and other fields
	initialContent := `# LazyWorktree Config
theme: dracula
# This is a comment we want to keep
other_field: preserved # Inline comment
`
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(initialContent), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig()
	cfg.ConfigPath = configPath
	cfg.Theme = "narna"

	// Test 1: Update theme while preserving comments
	err := SaveConfig(cfg)
	require.NoError(t, err)

	// #nosec G304
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "# LazyWorktree Config")
	assert.Contains(t, content, "theme: narna")
	assert.Contains(t, content, "# This is a comment we want to keep")
	assert.Contains(t, content, "other_field: preserved # Inline comment")
	assert.NotContains(t, content, "theme: dracula")

	// Test 2: Add theme if missing
	noThemeContent := "other_field: active\n"
	configPath2 := filepath.Join(tmpDir, "config2.yaml")
	if err := os.WriteFile(configPath2, []byte(noThemeContent), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg2 := DefaultConfig()
	cfg2.ConfigPath = configPath2
	cfg2.Theme = "modern"
	err = SaveConfig(cfg2)
	require.NoError(t, err)

	// #nosec G304
	data, err = os.ReadFile(configPath2)
	require.NoError(t, err)
	content = string(data)
	assert.Contains(t, content, "other_field: active")
	assert.Contains(t, content, "theme: modern")

	// Test 3: New file
	configPath3 := filepath.Join(tmpDir, "new", "config3.yaml")
	cfg3 := DefaultConfig()
	cfg3.ConfigPath = configPath3
	cfg3.Theme = "nord"
	err = SaveConfig(cfg3)
	require.NoError(t, err)
	assert.FileExists(t, configPath3)

	// #nosec G304
	data, err = os.ReadFile(configPath3)
	require.NoError(t, err)
	assert.Contains(t, string(data), "theme: nord")
}

func TestIsPathWithin(t *testing.T) {
	base := filepath.Join(t.TempDir(), "base")
	inside := filepath.Join(base, "child")
	outside := filepath.Join(base, "..", "other")

	assert.True(t, isPathWithin(base, base))
	assert.True(t, isPathWithin(base, inside))
	assert.False(t, isPathWithin(base, outside))
}

func TestLoadConfigWithCustomPath(t *testing.T) {
	// Setup mock to prevent loading real git config from the test repository
	defer func() { gitConfigMock = nil }()
	gitConfigMock = func(args []string, repoPath string) (string, error) {
		return "", nil
	}

	tempDir := t.TempDir()

	// Create a custom config file with valid YAML
	customConfigPath := filepath.Join(tempDir, "custom-config.yaml")
	customConfigContent := "theme: narna\n"
	err := os.WriteFile(customConfigPath, []byte(customConfigContent), 0o600)
	require.NoError(t, err)

	// Load config from custom path
	cfg, err := LoadConfig(customConfigPath)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "narna", cfg.Theme)
	assert.Equal(t, customConfigPath, cfg.ConfigPath)
}

func TestLoadConfigWithCustomPathFromAnywhere(t *testing.T) {
	// Setup mock to prevent loading real git config from the test repository
	defer func() { gitConfigMock = nil }()
	gitConfigMock = func(args []string, repoPath string) (string, error) {
		return "", nil
	}

	tempDir := t.TempDir()

	// Create a custom config file outside standard config directory
	customConfigPath := filepath.Join(tempDir, "my-custom-config.yaml")
	customConfigContent := "theme: dracula-light\nauto_fetch_prs: true\n"
	err := os.WriteFile(customConfigPath, []byte(customConfigContent), 0o600)
	require.NoError(t, err)

	// Load config from arbitrary path - should work now since we allow any path
	cfg, err := LoadConfig(customConfigPath)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "dracula-light", cfg.Theme)
	assert.True(t, cfg.AutoFetchPRs)
	assert.Equal(t, customConfigPath, cfg.ConfigPath)
}

func TestLoadConfigWithNonexistentPathFallsBack(t *testing.T) {
	// Try to load from a non-existent path
	cfg, err := LoadConfig("/this/path/does/not/exist/config.yaml")
	require.NoError(t, err)
	// Should return default config when file doesn't exist
	assert.NotNil(t, cfg)
	// Theme will be auto-detected or set to default
	assert.NotEmpty(t, cfg.Theme)
}

func TestMaxNameLengthConfig(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]interface{}
		expected int
	}{
		{
			name:     "default value",
			data:     map[string]interface{}{},
			expected: 95,
		},
		{
			name:     "custom value",
			data:     map[string]interface{}{"max_name_length": 50},
			expected: 50,
		},
		{
			name:     "disabled with 0",
			data:     map[string]interface{}{"max_name_length": 0},
			expected: 0,
		},
		{
			name:     "negative treated as 0",
			data:     map[string]interface{}{"max_name_length": -10},
			expected: 0,
		},
		{
			name:     "string coerced to int",
			data:     map[string]interface{}{"max_name_length": "75"},
			expected: 75,
		},
		{
			name:     "large value",
			data:     map[string]interface{}{"max_name_length": 200},
			expected: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := parseConfig(tt.data)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, cfg.MaxNameLength, "MaxNameLength mismatch")
		})
	}
}

// Integration tests for git config precedence

func TestLoadConfigWithGitGlobalConfig(t *testing.T) {
	// Setup mock
	defer func() { gitConfigMock = nil }()

	gitConfigMock = func(args []string, repoPath string) (string, error) {
		if slices.Contains(args, "--global") {
			return "lw.worktree_dir /git/global/path\nlw.auto_fetch_prs true\nlw.theme nord\n", nil
		}
		return "", nil
	}

	cfg, err := LoadConfig("")
	require.NoError(t, err)

	// Values from git global config should be applied
	assert.Equal(t, "/git/global/path", cfg.WorktreeDir)
	assert.True(t, cfg.AutoFetchPRs)
	assert.Equal(t, "nord", cfg.Theme)
}

func TestLoadConfigGitLocalOverridesGitGlobal(t *testing.T) {
	// Setup mock
	defer func() { gitConfigMock = nil }()

	gitConfigMock = func(args []string, repoPath string) (string, error) {
		if slices.Contains(args, "--global") {
			return "lw.theme nord\nlw.auto_fetch_prs true\nlw.worktree_dir /global/path\n", nil
		}
		if slices.Contains(args, "--local") {
			// Local overrides theme and auto_fetch_prs
			return "lw.theme dracula\nlw.auto_fetch_prs false\n", nil
		}
		return "", nil
	}

	cfg, err := LoadConfig("")
	require.NoError(t, err)

	// Local git config overrides global
	assert.Equal(t, "dracula", cfg.Theme)
	assert.False(t, cfg.AutoFetchPRs)
	// WorktreeDir from global (not overridden by local)
	assert.Equal(t, "/global/path", cfg.WorktreeDir)
}

func TestLoadConfigGitConfigMultiValue(t *testing.T) {
	// Setup mock
	defer func() { gitConfigMock = nil }()

	gitConfigMock = func(args []string, repoPath string) (string, error) {
		if slices.Contains(args, "--global") {
			return "lw.init_commands link_topsymlinks\nlw.init_commands npm install\nlw.init_commands make setup\n", nil
		}
		return "", nil
	}

	cfg, err := LoadConfig("")
	require.NoError(t, err)

	// Multi-value config should be parsed as array
	assert.Equal(t, []string{"link_topsymlinks", "npm install", "make setup"}, cfg.InitCommands)
}

func TestApplyCLIOverrides(t *testing.T) {
	cfg := DefaultConfig()

	// Apply CLI overrides
	overrides := []string{
		"lw.theme=gruvbox-dark",
		"lw.auto_fetch_prs=true",
		"lw.max_diff_chars=500000",
	}

	err := cfg.ApplyCLIOverrides(overrides)
	require.NoError(t, err)

	assert.Equal(t, "gruvbox-dark", cfg.Theme)
	assert.True(t, cfg.AutoFetchPRs)
	assert.Equal(t, 500000, cfg.MaxDiffChars)
}

func TestApplyCLIOverridesMultiValue(t *testing.T) {
	cfg := DefaultConfig()

	// Apply CLI overrides with repeated keys
	overrides := []string{
		"lw.init_commands=echo first",
		"lw.init_commands=echo second",
		"lw.theme=nord",
	}

	err := cfg.ApplyCLIOverrides(overrides)
	require.NoError(t, err)

	assert.Equal(t, []string{"echo first", "echo second"}, cfg.InitCommands)
	assert.Equal(t, "nord", cfg.Theme)
}

func TestApplyCLIOverridesInvalidFormat(t *testing.T) {
	cfg := DefaultConfig()

	// Invalid format (missing equals)
	overrides := []string{"lw.theme"}
	err := cfg.ApplyCLIOverrides(overrides)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid config override")

	// Invalid format (missing lw prefix)
	overrides = []string{"theme=nord"}
	err = cfg.ApplyCLIOverrides(overrides)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config override key must start with 'lw.'")
}

func TestConfigPrecedenceFullStack(t *testing.T) {
	// Test full precedence: CLI override > git local > git global > YAML > defaults
	defer func() { gitConfigMock = nil }()

	// Create temp YAML config
	tempDir := t.TempDir()
	yamlPath := filepath.Join(tempDir, "config.yaml")
	yamlContent := "theme: dracula\nworktree_dir: /yaml/path\nauto_fetch_prs: false\nmax_diff_chars: 100000\n"
	err := os.WriteFile(yamlPath, []byte(yamlContent), 0o600)
	require.NoError(t, err)

	// Mock git config
	gitConfigMock = func(args []string, repoPath string) (string, error) {
		if slices.Contains(args, "--global") {
			return "lw.theme nord\nlw.auto_fetch_prs true\nlw.max_untracked_diffs 20\n", nil
		}
		if slices.Contains(args, "--local") {
			return "lw.theme gruvbox-dark\nlw.max_diff_chars 200000\n", nil
		}
		return "", nil
	}

	// Load config with YAML
	cfg, err := LoadConfig(yamlPath)
	require.NoError(t, err)

	// Verify precedence:
	// - theme: gruvbox-dark (git local wins over git global and YAML)
	assert.Equal(t, "gruvbox-dark", cfg.Theme)

	// - worktree_dir: /yaml/path (from YAML, not overridden by git)
	assert.Equal(t, "/yaml/path", cfg.WorktreeDir)

	// - auto_fetch_prs: true (git global wins over YAML)
	assert.True(t, cfg.AutoFetchPRs)

	// - max_diff_chars: 200000 (git local wins over YAML)
	assert.Equal(t, 200000, cfg.MaxDiffChars)

	// - max_untracked_diffs: 20 (from git global)
	assert.Equal(t, 20, cfg.MaxUntrackedDiffs)

	// Now apply CLI overrides (highest precedence)
	cliOverrides := []string{
		"lw.theme=tokyo-night",
		"lw.auto_fetch_prs=false",
	}
	err = cfg.ApplyCLIOverrides(cliOverrides)
	require.NoError(t, err)

	// CLI overrides win
	assert.Equal(t, "tokyo-night", cfg.Theme)
	assert.False(t, cfg.AutoFetchPRs)
	// Other values unchanged
	assert.Equal(t, "/yaml/path", cfg.WorktreeDir)
	assert.Equal(t, 200000, cfg.MaxDiffChars)
}
