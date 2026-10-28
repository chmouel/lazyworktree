package config

import "testing"

func TestParseHerdrCommandDefaults(t *testing.T) {
	cmds, warnings := parseCustomCommands(map[string]any{
		"custom_commands": map[string]any{
			PaneUniversal: map[string]any{
				"H": map[string]any{
					"herdr": map[string]any{
						"session_name": "wt:$WORKTREE_NAME",
						"attach":       true,
						"windows": []any{
							map[string]any{"name": "editor", "command": "vim"},
						},
					},
				},
			},
		},
	})
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	cmd := cmds[PaneUniversal]["H"]
	if cmd == nil || cmd.Herdr == nil {
		t.Fatal("expected herdr command to be parsed")
	}
	if cmd.Herdr.Mode != HerdrModeWorkspace {
		t.Errorf("mode = %q, want %q", cmd.Herdr.Mode, HerdrModeWorkspace)
	}
	if cmd.Herdr.OnExists != "switch" {
		t.Errorf("on_exists = %q, want switch", cmd.Herdr.OnExists)
	}
	if !cmd.Herdr.Attach {
		t.Error("expected attach to be true")
	}
	if len(cmd.Herdr.Windows) != 1 || cmd.Herdr.Windows[0].Name != "editor" {
		t.Errorf("unexpected windows: %+v", cmd.Herdr.Windows)
	}
}

func TestParseHerdrCommandSessionMode(t *testing.T) {
	cmds, _ := parseCustomCommands(map[string]any{
		"custom_commands": map[string]any{
			PaneUniversal: map[string]any{
				"H": map[string]any{
					"herdr": map[string]any{
						"mode":          "SESSION",
						"herdr_session": "team",
					},
				},
			},
		},
	})
	cmd := cmds[PaneUniversal]["H"]
	if cmd == nil || cmd.Herdr == nil {
		t.Fatal("expected herdr command to be parsed")
	}
	if cmd.Herdr.Mode != HerdrModeSession {
		t.Errorf("mode = %q, want %q", cmd.Herdr.Mode, HerdrModeSession)
	}
	if cmd.Herdr.HerdrSession != "team" {
		t.Errorf("herdr_session = %q, want team", cmd.Herdr.HerdrSession)
	}
	if len(cmd.Herdr.Windows) != 1 || cmd.Herdr.Windows[0].Name != "shell" {
		t.Errorf("expected default shell window, got %+v", cmd.Herdr.Windows)
	}
}

func TestParseHerdrCommandUnknownModeFallsBackToWorkspace(t *testing.T) {
	cmds, _ := parseCustomCommands(map[string]any{
		"custom_commands": map[string]any{
			PaneUniversal: map[string]any{
				"H": map[string]any{
					"herdr": map[string]any{"mode": "galaxy"},
				},
			},
		},
	})
	cmd := cmds[PaneUniversal]["H"]
	if cmd == nil || cmd.Herdr == nil {
		t.Fatal("expected herdr command to be parsed")
	}
	if cmd.Herdr.Mode != HerdrModeWorkspace {
		t.Errorf("mode = %q, want %q", cmd.Herdr.Mode, HerdrModeWorkspace)
	}
}

func TestDefaultConfigIncludesHerdrCommand(t *testing.T) {
	cmd, ok := DefaultConfig().CustomCommands[PaneUniversal]["H"]
	if !ok || cmd.Herdr == nil {
		t.Fatal("expected default H command to configure herdr")
	}
	if cmd.Herdr.Mode != HerdrModeWorkspace {
		t.Errorf("default mode = %q, want %q", cmd.Herdr.Mode, HerdrModeWorkspace)
	}
	if !cmd.ShowHelp {
		t.Error("expected default herdr command to show in help")
	}
}

func TestParseHerdrCommandWorktreeGroup(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  bool
	}{
		{name: "defaults to grouping", value: nil, want: true},
		{name: "can be disabled", value: false, want: false},
		{name: "accepts string booleans", value: "false", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			herdr := map[string]any{}
			if tt.value != nil {
				herdr["worktree_group"] = tt.value
			}
			cmds, _ := parseCustomCommands(map[string]any{
				"custom_commands": map[string]any{
					PaneUniversal: map[string]any{"H": map[string]any{"herdr": herdr}},
				},
			})
			cmd := cmds[PaneUniversal]["H"]
			if cmd == nil || cmd.Herdr == nil {
				t.Fatal("expected herdr command to be parsed")
			}
			if cmd.Herdr.WorktreeGroup != tt.want {
				t.Errorf("worktree_group = %v, want %v", cmd.Herdr.WorktreeGroup, tt.want)
			}
		})
	}
}
