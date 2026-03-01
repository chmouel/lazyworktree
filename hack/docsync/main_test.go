package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGlobalFlags(t *testing.T) {
	path := filepath.Join("..", "..", "internal", "bootstrap", "flags.go")
	flags, err := parseGlobalFlags(path)
	if err != nil {
		t.Fatalf("parseGlobalFlags returned error: %v", err)
	}

	required := []string{"config", "config-file", "debug-log", "output-selection", "search-auto-select", "show-syntax-themes", "theme", "worktree-dir"}
	for _, name := range required {
		if !containsFlag(flags, name) {
			t.Fatalf("expected global flag %q", name)
		}
	}
}

func TestParseCommands(t *testing.T) {
	path := filepath.Join("..", "..", "internal", "bootstrap", "commands.go")
	commands, err := parseCommands(path)
	if err != nil {
		t.Fatalf("parseCommands returned error: %v", err)
	}

	required := []string{"list", "create", "delete", "rename", "exec"}
	for _, name := range required {
		if !containsCommand(commands, name) {
			t.Fatalf("expected command %q", name)
		}
	}

	listCmd := mustGetCommand(commands, "list", t)
	if len(listCmd.Aliases) != 1 || listCmd.Aliases[0] != "ls" {
		t.Fatalf("expected list command alias ls, got %#v", listCmd.Aliases)
	}

	createCmd := mustGetCommand(commands, "create", t)
	createFlags := []string{"from-branch", "from-issue", "from-pr", "generate", "no-workspace", "query", "with-change"}
	for _, flag := range createFlags {
		if !containsFlag(createCmd.Flags, flag) {
			t.Fatalf("expected create flag %q", flag)
		}
	}
}

func TestParseConfigKeys(t *testing.T) {
	path := filepath.Join("..", "..", "internal", "config", "config.go")
	keys, err := parseConfigKeys(path)
	if err != nil {
		t.Fatalf("parseConfigKeys returned error: %v", err)
	}

	required := []string{"worktree_dir", "theme", "custom_commands", "custom_create_menus", "custom_themes", "git_pager", "icon_set", "sort_mode", "sort_by_active", "trust_mode"}
	for _, key := range required {
		if !containsConfigKey(keys, key) {
			t.Fatalf("expected config key %q", key)
		}
	}

	if containsConfigKey(keys, "attach") {
		t.Fatalf("unexpected nested key %q in top-level config keys", "attach")
	}
}

func TestRenderCLIFlagsPageContainsValidationRules(t *testing.T) {
	content := renderCLIFlagsPage(
		[]flagSpec{{Name: "worktree-dir", Kind: "string", Usage: "..."}},
		[]commandSpec{{Name: "list", Flags: []flagSpec{{Name: "json", Kind: "bool", Usage: "..."}}}},
	)
	if !contains(content, "Validation Rules") {
		t.Fatalf("expected Validation Rules section in generated CLI flags page")
	}
}

func containsFlag(flags []flagSpec, name string) bool {
	for _, flag := range flags {
		if flag.Name == name {
			return true
		}
	}
	return false
}

func containsCommand(commands []commandSpec, name string) bool {
	for _, command := range commands {
		if command.Name == name {
			return true
		}
	}
	return false
}

func containsConfigKey(keys []configKeySpec, key string) bool {
	for _, item := range keys {
		if item.Key == key {
			return true
		}
	}
	return false
}

func mustGetCommand(commands []commandSpec, name string, t *testing.T) commandSpec {
	t.Helper()
	for _, command := range commands {
		if command.Name == name {
			return command
		}
	}
	t.Fatalf("missing command %q", name)
	return commandSpec{}
}

func contains(content, needle string) bool {
	return strings.Contains(content, needle)
}
