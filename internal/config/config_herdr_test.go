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

func TestHerdrCreateLayout(t *testing.T) {
	worktreesLayout := &CustomCommand{Herdr: &HerdrCommand{Mode: HerdrModeWorkspace}}
	commands := CustomCommandsConfig{
		PaneUniversal: {
			"H": {Herdr: &HerdrCommand{Mode: HerdrModeWorkspace}},
			"S": {Herdr: &HerdrCommand{Mode: HerdrModeSession}},
			"T": {Tmux: &TmuxCommand{}},
			"W": {Herdr: &HerdrCommand{Mode: HerdrModeWorkspace}},
		},
		PaneWorktrees: {"W": worktreesLayout},
		PaneLog:       {"L": {Herdr: &HerdrCommand{Mode: HerdrModeWorkspace}}},
	}
	tests := []struct {
		name        string
		key         string
		wantCommand bool
		wantWarning bool
	}{
		{name: "empty key selects plain mode", key: ""},
		{name: "workspace herdr block", key: "H", wantCommand: true},
		{name: "missing key", key: "X", wantWarning: true},
		{name: "no herdr block", key: "T", wantWarning: true},
		{name: "session mode", key: "S", wantWarning: true},
		{name: "worktrees binding wins", key: "W", wantCommand: true},
		{name: "other pane bindings are ignored", key: "L", wantWarning: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &AppConfig{CustomCommands: commands, HerdrWorkspaceCreateLayout: tt.key}
			cmd, warning := cfg.HerdrCreateLayout()
			if (cmd != nil) != tt.wantCommand {
				t.Errorf("command = %v, want present=%v", cmd, tt.wantCommand)
			}
			if (warning != "") != tt.wantWarning {
				t.Errorf("warning = %q, want present=%v", warning, tt.wantWarning)
			}
			if tt.key == "W" && cmd != worktreesLayout {
				t.Error("the worktrees pane binding must be used, as when pressing the key there")
			}
		})
	}
}

func TestHerdrWorkspaceCreateConfigOverrides(t *testing.T) {
	cfg, err := parseConfig(map[string]any{"herdr_workspace_create_layout": " H "})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UseHerdrForWorkspaceCreate {
		t.Fatal("use_herdr_for_workspace_create must be off by default")
	}
	if cfg.HerdrWorkspaceCreateLayout != "H" {
		t.Fatalf("layout = %q, want H", cfg.HerdrWorkspaceCreateLayout)
	}
	if err := cfg.ApplyCLIOverrides([]string{"lw.use_herdr_for_workspace_create=true", "lw.herdr_workspace_create_layout=G"}); err != nil {
		t.Fatal(err)
	}
	if !cfg.UseHerdrForWorkspaceCreate || cfg.HerdrWorkspaceCreateLayout != "G" {
		t.Fatalf("overrides not applied: use=%v layout=%q", cfg.UseHerdrForWorkspaceCreate, cfg.HerdrWorkspaceCreateLayout)
	}
}
