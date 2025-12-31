package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.NotNil(t, cfg)
	assert.True(t, cfg.SortByActive)
	assert.False(t, cfg.AutoFetchPRs)
	assert.Equal(t, 10, cfg.MaxUntrackedDiffs)
	assert.Equal(t, 200000, cfg.MaxDiffChars)
	assert.Equal(t, []string{"--syntax-theme", "Dracula"}, cfg.DeltaArgs)
	assert.Equal(t, "tofu", cfg.TrustMode)
	assert.Empty(t, cfg.WorktreeDir)
	assert.Empty(t, cfg.InitCommands)
	assert.Empty(t, cfg.TerminateCommands)
	assert.Empty(t, cfg.DebugLog)
	assert.NotNil(t, cfg.CustomCommands)
	assert.Empty(t, cfg.CustomCommands)
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
		name     string
		data     map[string]interface{}
		validate func(*testing.T, *AppConfig)
	}{
		{
			name: "empty config uses defaults",
			data: map[string]interface{}{},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.True(t, cfg.SortByActive)
				assert.False(t, cfg.AutoFetchPRs)
				assert.Equal(t, 10, cfg.MaxUntrackedDiffs)
				assert.Equal(t, 200000, cfg.MaxDiffChars)
				assert.Equal(t, []string{"--syntax-theme", "Dracula"}, cfg.DeltaArgs)
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
				assert.False(t, cfg.SortByActive)
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
			name: "delta_args string",
			data: map[string]interface{}{
				"delta_args": "--syntax-theme Dracula",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, []string{"--syntax-theme", "Dracula"}, cfg.DeltaArgs)
			},
		},
		{
			name: "delta_args list",
			data: map[string]interface{}{
				"delta_args": []interface{}{"--syntax-theme", "Dracula"},
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, []string{"--syntax-theme", "Dracula"}, cfg.DeltaArgs)
			},
		},
		{
			name: "delta_args empty list",
			data: map[string]interface{}{
				"delta_args": []interface{}{},
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Empty(t, cfg.DeltaArgs)
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := parseConfig(tt.data)
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
	t.Run("no config file returns defaults", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", tmpDir)
		configDir := filepath.Join(tmpDir, "lazyworktree")
		configPath := filepath.Join(configDir, "nonexistent.yaml")

		require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o750))

		cfg, err := LoadConfig(configPath)
		require.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.Equal(t, DefaultConfig().SortByActive, cfg.SortByActive)
		assert.Equal(t, DefaultConfig().MaxUntrackedDiffs, cfg.MaxUntrackedDiffs)
		assert.Equal(t, DefaultConfig().DeltaArgs, cfg.DeltaArgs)
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
delta_args:
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
		assert.False(t, cfg.SortByActive)
		assert.True(t, cfg.AutoFetchPRs)
		assert.Equal(t, 20, cfg.MaxUntrackedDiffs)
		assert.Equal(t, 100000, cfg.MaxDiffChars)
		assert.Equal(t, []string{"--syntax-theme", "Dracula"}, cfg.DeltaArgs)
		assert.Equal(t, "always", cfg.TrustMode)
		assert.Equal(t, []string{"echo \"init\""}, cfg.InitCommands)
		assert.Equal(t, []string{"echo \"cleanup\""}, cfg.TerminateCommands)
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
		assert.Equal(t, DefaultConfig().SortByActive, cfg.SortByActive)
	})
}

func TestExpandPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		setup    func()
		cleanup  func()
		validate func(*testing.T, string)
	}{
		{
			name:    "path without tilde",
			input:   "/absolute/path",
			setup:   func() {},
			cleanup: func() {},
			validate: func(t *testing.T, result string) {
				assert.Equal(t, "/absolute/path", result)
			},
		},
		{
			name:    "path with tilde",
			input:   "~/test/path",
			setup:   func() {},
			cleanup: func() {},
			validate: func(t *testing.T, result string) {
				home, _ := os.UserHomeDir()
				assert.Equal(t, filepath.Join(home, "test/path"), result)
			},
		},
		{
			name:    "path with environment variable",
			input:   "$HOME/test",
			setup:   func() {},
			cleanup: func() {},
			validate: func(t *testing.T, result string) {
				home := os.Getenv("HOME")
				assert.Equal(t, filepath.Join(home, "test"), result)
			},
		},
		{
			name:  "path with custom env var",
			input: "$CUSTOM_VAR/test",
			setup: func() {
				_ = os.Setenv("CUSTOM_VAR", "/custom")
			},
			cleanup: func() {
				_ = os.Unsetenv("CUSTOM_VAR")
			},
			validate: func(t *testing.T, result string) {
				assert.Equal(t, "/custom/test", result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			defer tt.cleanup()

			result, err := expandPath(tt.input)
			require.NoError(t, err)
			tt.validate(t, result)
		})
	}
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
					},
				},
			},
			expected: map[string]*CustomCommand{
				"e": {
					Command:     "nvim",
					Description: "Open editor",
					ShowHelp:    true,
					Wait:        true,
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
				"worktree_dir":   "/tmp/worktrees",
				"sort_by_active": true,
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
				assert.True(t, cfg.SortByActive)
				require.Len(t, cfg.CustomCommands, 1)
				assert.Equal(t, "nvim", cfg.CustomCommands["e"].Command)
				assert.Equal(t, "Open editor", cfg.CustomCommands["e"].Description)
				assert.True(t, cfg.CustomCommands["e"].ShowHelp)
				assert.False(t, cfg.CustomCommands["e"].Wait)
			},
		},
		{
			name: "no custom commands",
			input: map[string]interface{}{
				"worktree_dir": "/tmp/worktrees",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Empty(t, cfg.CustomCommands)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := parseConfig(tt.input)
			tt.validate(t, cfg)
		})
	}
}
