package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCustomCommands(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected CustomCommandsConfig
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: CustomCommandsConfig{},
		},
		{
			name:     "empty map",
			input:    map[string]interface{}{},
			expected: CustomCommandsConfig{},
		},
		{
			name: "single command with all fields",
			input: map[string]interface{}{
				"custom_commands": map[string]interface{}{
					PaneUniversal: map[string]interface{}{
						"e": map[string]interface{}{
							"command":     "nvim",
							"description": "Open editor",
							"show_help":   true,
							"wait":        true,
							"show_output": true,
						},
					},
				},
			},
			expected: CustomCommandsConfig{
				PaneUniversal: {
					"e": {
						Command:     "nvim",
						Description: "Open editor",
						ShowHelp:    true,
						Wait:        true,
						ShowOutput:  true,
					},
				},
			},
		},
		{
			name: "multiple commands",
			input: map[string]interface{}{
				"custom_commands": map[string]interface{}{
					PaneUniversal: map[string]interface{}{
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
			},
			expected: CustomCommandsConfig{
				PaneUniversal: {
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
		},
		{
			name: "command with spaces trimmed",
			input: map[string]interface{}{
				"custom_commands": map[string]interface{}{
					PaneUniversal: map[string]interface{}{
						"t": map[string]interface{}{
							"command":     "  make test  ",
							"description": "  Run tests  ",
						},
					},
				},
			},
			expected: CustomCommandsConfig{
				PaneUniversal: {
					"t": {
						Command:     "make test",
						Description: "Run tests",
						ShowHelp:    false,
						Wait:        false,
					},
				},
			},
		},
		{
			name: "empty command is skipped",
			input: map[string]interface{}{
				"custom_commands": map[string]interface{}{
					PaneUniversal: map[string]interface{}{
						"e": map[string]interface{}{
							"command":     "",
							"description": "Empty command",
						},
					},
				},
			},
			expected: CustomCommandsConfig{},
		},
		{
			name: "command with only whitespace is skipped",
			input: map[string]interface{}{
				"custom_commands": map[string]interface{}{
					PaneUniversal: map[string]interface{}{
						"e": map[string]interface{}{
							"command":     "   ",
							"description": "Whitespace command",
						},
					},
				},
			},
			expected: CustomCommandsConfig{},
		},
		{
			name: "tmux command with windows",
			input: map[string]interface{}{
				"custom_commands": map[string]interface{}{
					PaneUniversal: map[string]interface{}{
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
			},
			expected: CustomCommandsConfig{
				PaneUniversal: {
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
		},
		{
			name: "zellij command with windows",
			input: map[string]interface{}{
				"custom_commands": map[string]interface{}{
					PaneUniversal: map[string]interface{}{
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
			},
			expected: CustomCommandsConfig{
				PaneUniversal: {
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
		},
		{
			name: "tmux without windows defaults to shell window",
			input: map[string]interface{}{
				"custom_commands": map[string]interface{}{
					PaneUniversal: map[string]interface{}{
						"x": map[string]interface{}{
							"tmux": map[string]interface{}{
								"session_name": "${REPO_NAME}_wt_$WORKTREE_NAME",
							},
						},
					},
				},
			},
			expected: CustomCommandsConfig{
				PaneUniversal: {
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
		},
		{
			name: "invalid type for custom_commands is ignored",
			input: map[string]interface{}{
				"custom_commands": "not a map",
			},
			expected: CustomCommandsConfig{},
		},
		{
			name: "invalid type for command entry is skipped",
			input: map[string]interface{}{
				"custom_commands": map[string]interface{}{
					PaneUniversal: map[string]interface{}{
						"e": "not a map",
						"s": map[string]interface{}{
							"command": "zsh",
						},
					},
				},
			},
			expected: CustomCommandsConfig{
				PaneUniversal: {
					"s": {
						Command:     "zsh",
						Description: "",
						ShowHelp:    false,
						Wait:        false,
					},
				},
			},
		},
		{
			name: "boolean coercion for show_help",
			input: map[string]interface{}{
				"custom_commands": map[string]interface{}{
					PaneUniversal: map[string]interface{}{
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
			},
			expected: CustomCommandsConfig{
				PaneUniversal: {
					"a": {Command: "cmd1", ShowHelp: true, Wait: false},
					"b": {Command: "cmd2", ShowHelp: false, Wait: false},
					"c": {Command: "cmd3", ShowHelp: true, Wait: false},
					"d": {Command: "cmd4", ShowHelp: false, Wait: false},
				},
			},
		},
		{
			name: "boolean coercion for wait",
			input: map[string]interface{}{
				"custom_commands": map[string]interface{}{
					PaneUniversal: map[string]interface{}{
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
			},
			expected: CustomCommandsConfig{
				PaneUniversal: {
					"a": {Command: "cmd1", Wait: true},
					"b": {Command: "cmd2", Wait: false},
					"c": {Command: "cmd3", Wait: true},
				},
			},
		},
		{
			name: "boolean coercion for show_output",
			input: map[string]interface{}{
				"custom_commands": map[string]interface{}{
					PaneUniversal: map[string]interface{}{
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
			},
			expected: CustomCommandsConfig{
				PaneUniversal: {
					"a": {Command: "cmd1", ShowOutput: true},
					"b": {Command: "cmd2", ShowOutput: false},
					"c": {Command: "cmd3", ShowOutput: true},
				},
			},
		},
		{
			name: "missing fields use defaults",
			input: map[string]interface{}{
				"custom_commands": map[string]interface{}{
					PaneUniversal: map[string]interface{}{
						"e": map[string]interface{}{
							"command": "nvim",
						},
					},
				},
			},
			expected: CustomCommandsConfig{
				PaneUniversal: {
					"e": {
						Command:     "nvim",
						Description: "",
						ShowHelp:    false,
						Wait:        false,
					},
				},
			},
		},
		{
			name: "modifier keys (ctrl, alt, etc.)",
			input: map[string]interface{}{
				"custom_commands": map[string]interface{}{
					PaneUniversal: map[string]interface{}{
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
			},
			expected: CustomCommandsConfig{
				PaneUniversal: {
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
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _ := parseCustomCommands(tt.input)
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
					PaneUniversal: map[string]interface{}{
						"e": map[string]interface{}{
							"command":     "nvim",
							"description": "Open editor",
							"show_help":   true,
							"wait":        false,
						},
					},
				},
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "/tmp/worktrees", cfg.WorktreeDir)
				assert.Equal(t, "switched", cfg.SortMode)
				require.Len(t, cfg.CustomCommands[PaneUniversal], 4)
				assert.Equal(t, "nvim", cfg.CustomCommands[PaneUniversal]["e"].Command)
				assert.Equal(t, "Open editor", cfg.CustomCommands[PaneUniversal]["e"].Description)
				assert.True(t, cfg.CustomCommands[PaneUniversal]["e"].ShowHelp)
				assert.False(t, cfg.CustomCommands[PaneUniversal]["e"].Wait)
				require.Contains(t, cfg.CustomCommands[PaneUniversal], "t")
				require.Contains(t, cfg.CustomCommands[PaneUniversal], "Z")
				require.Contains(t, cfg.CustomCommands[PaneUniversal], "H")
			},
		},
		{
			name: "no custom commands",
			input: map[string]interface{}{
				"worktree_dir": "/tmp/worktrees",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				require.Contains(t, cfg.CustomCommands[PaneUniversal], "t")
				assert.Equal(t, "Tmux", cfg.CustomCommands[PaneUniversal]["t"].Description)
				require.Contains(t, cfg.CustomCommands[PaneUniversal], "Z")
				assert.Equal(t, "Zellij", cfg.CustomCommands[PaneUniversal]["Z"].Description)
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

func TestCustomCommandKeyHelpers(t *testing.T) {
	t.Parallel()

	assert.True(t, IsPaletteOnlyCommandKey("_review"))
	assert.False(t, IsPaletteOnlyCommandKey("r"))
	assert.Equal(t, "review", PaletteOnlyCommandName("_review"))
	assert.Empty(t, PaletteOnlyCommandName("r"))
	assert.False(t, CustomCommandHasKeyBinding("_review"))
	assert.True(t, CustomCommandHasKeyBinding("r"))
}

func TestParseCustomCommandsOldFlatFormatMigration(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"custom_commands": map[string]any{
			"x": map[string]any{
				"command": "make test",
			},
		},
	}
	cmds, warnings := parseCustomCommands(input)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "old flat format")
	require.Contains(t, cmds[PaneUniversal], "x")
	assert.Equal(t, "make test", cmds[PaneUniversal]["x"].Command)
}

func TestParseCustomCommandsNewFormatNoWarning(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"custom_commands": map[string]any{
			PaneUniversal: map[string]any{
				"x": map[string]any{
					"command": "make test",
				},
			},
		},
	}
	cmds, warnings := parseCustomCommands(input)
	assert.Empty(t, warnings)
	require.Contains(t, cmds[PaneUniversal], "x")
	assert.Equal(t, "make test", cmds[PaneUniversal]["x"].Command)
}
