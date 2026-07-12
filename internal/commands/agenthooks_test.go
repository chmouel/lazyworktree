package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupHooksTestOpts(t *testing.T, dryRun bool) (SetupAgentHooksOptions, *bytes.Buffer) {
	t.Helper()
	dir := t.TempDir()
	out := &bytes.Buffer{}
	return SetupAgentHooksOptions{
		DryRun:             dryRun,
		ClaudeSettingsPath: filepath.Join(dir, "claude", "settings.json"),
		CodexHooksPath:     filepath.Join(dir, "codex", "hooks.json"),
		CopilotHooksPath:   filepath.Join(dir, "copilot", "hooks", "lazyworktree.json"),
		Stdout:             out,
	}, out
}

func readHooksFile(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // Test path.
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return root
}

func hasMatcherHook(groups []any, matcher, agent string) bool {
	for _, group := range groups {
		groupMap, _ := group.(map[string]any)
		if groupMap["matcher"] != matcher {
			continue
		}
		handlers, _ := groupMap["hooks"].([]any)
		for _, handler := range handlers {
			handlerMap, _ := handler.(map[string]any)
			command, _ := handlerMap["command"].(string)
			if strings.Contains(command, "agent-event") && strings.Contains(command, "--agent "+agent) {
				return true
			}
		}
	}
	return false
}

func matcherHookIsSynchronous(groups []any, matcher string) bool {
	for _, group := range groups {
		groupMap, _ := group.(map[string]any)
		groupMatcher, _ := groupMap["matcher"].(string)
		if groupMatcher != matcher {
			continue
		}
		handlers, _ := groupMap["hooks"].([]any)
		for _, handler := range handlers {
			handlerMap, _ := handler.(map[string]any)
			command, _ := handlerMap["command"].(string)
			if isAgentEventHookCommand(command) {
				_, async := handlerMap["async"]
				return !async
			}
		}
	}
	return false
}

func TestSetupAgentHooksInstallsBothConfigs(t *testing.T) {
	opts, _ := setupHooksTestOpts(t, false)
	if err := SetupAgentHooks(opts); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	claude := readHooksFile(t, opts.ClaudeSettingsPath)
	hooks, _ := claude["hooks"].(map[string]any)
	for _, event := range []string{"SessionStart", "UserPromptSubmit", "Stop", "SessionEnd"} {
		if _, ok := hooks[event]; !ok {
			t.Fatalf("claude settings missing %s hook", event)
		}
	}
	claudeSessionStart, _ := hooks["SessionStart"].([]any)
	if !matcherHookIsSynchronous(claudeSessionStart, "") {
		t.Fatal("claude lifecycle hooks must be synchronous")
	}
	claudePreTool, _ := hooks["PreToolUse"].([]any)
	if !hasMatcherHook(claudePreTool, "AskUserQuestion", "claude") {
		t.Fatal("claude settings missing question-open hook")
	}
	if !matcherHookIsSynchronous(claudePreTool, "AskUserQuestion") {
		t.Fatal("claude question-open hook must be synchronous")
	}
	claudePostTool, _ := hooks["PostToolUse"].([]any)
	if !hasMatcherHook(claudePostTool, "AskUserQuestion", "claude") {
		t.Fatal("claude settings missing question completion hook")
	}
	if !matcherHookIsSynchronous(claudePostTool, "AskUserQuestion") {
		t.Fatal("claude question completion hook must be synchronous")
	}
	for _, event := range []string{"Elicitation", "ElicitationResult"} {
		groups, _ := hooks[event].([]any)
		if !hasMatcherHook(groups, "*", "claude") {
			t.Fatalf("claude settings missing %s hook", event)
		}
	}

	codex := readHooksFile(t, opts.CodexHooksPath)
	codexHooks, _ := codex["hooks"].(map[string]any)
	for _, event := range []string{"SessionStart", "UserPromptSubmit", "Stop"} {
		if _, ok := codexHooks[event]; !ok {
			t.Fatalf("codex hooks missing %s", event)
		}
	}
	if _, ok := codexHooks["SessionEnd"]; ok {
		t.Fatal("codex has no SessionEnd event; must not be installed")
	}

	copilot := readHooksFile(t, opts.CopilotHooksPath)
	if version, ok := copilot["version"].(float64); !ok || version != 1 {
		t.Fatalf("copilot hooks missing version 1, got %v", copilot["version"])
	}
	copilotHooks, _ := copilot["hooks"].(map[string]any)
	for _, event := range []string{"SessionStart", "UserPromptSubmit", "Stop", "SessionEnd"} {
		entries, ok := copilotHooks[event].([]any)
		if !ok || len(entries) != 1 {
			t.Fatalf("copilot hooks missing %s entry", event)
		}
		entry, _ := entries[0].(map[string]any)
		if entry["type"] != "command" {
			t.Fatalf("copilot %s entry has wrong type %v", event, entry["type"])
		}
		command, _ := entry["command"].(string)
		if !strings.Contains(command, "agent-event") || !strings.Contains(command, "--agent copilot") {
			t.Fatalf("copilot %s entry has wrong command %q", event, command)
		}
	}
	copilotPreTool, _ := copilotHooks["PreToolUse"].([]any)
	if len(copilotPreTool) != 1 || copilotPreTool[0].(map[string]any)["matcher"] != "AskUserQuestion" {
		t.Fatal("copilot hooks missing AskUserQuestion question-open hook")
	}
	copilotPostTool, _ := copilotHooks["PostToolUse"].([]any)
	if len(copilotPostTool) != 1 || copilotPostTool[0].(map[string]any)["matcher"] != "AskUserQuestion" {
		t.Fatal("copilot hooks missing AskUserQuestion completion hook")
	}
}

func TestSetupAgentHooksIsIdempotent(t *testing.T) {
	opts, _ := setupHooksTestOpts(t, false)
	if err := SetupAgentHooks(opts); err != nil {
		t.Fatalf("first run failed: %v", err)
	}
	paths := []string{opts.ClaudeSettingsPath, opts.CodexHooksPath, opts.CopilotHooksPath}
	first := make([][]byte, len(paths))
	for i, path := range paths {
		first[i], _ = os.ReadFile(path) //nolint:gosec // Test-owned temporary path.
	}

	opts.Stdout = &bytes.Buffer{}
	if err := SetupAgentHooks(opts); err != nil {
		t.Fatalf("second run failed: %v", err)
	}
	for i, path := range paths {
		second, _ := os.ReadFile(path) //nolint:gosec // Test-owned temporary path.
		if !bytes.Equal(first[i], second) {
			t.Fatalf("second run modified %s", path)
		}
	}
	out := opts.Stdout.(*bytes.Buffer).String()
	if !strings.Contains(out, "already installed") {
		t.Fatalf("expected already-installed notice, got: %s", out)
	}
}

func TestSetupAgentHooksPreservesExistingSettings(t *testing.T) {
	opts, _ := setupHooksTestOpts(t, false)
	if err := os.MkdirAll(filepath.Dir(opts.ClaudeSettingsPath), 0o750); err != nil {
		t.Fatal(err)
	}
	existing := `{"model":"opus","hooks":{"PreToolUse":[{"hooks":[{"type":"command","command":"echo hi"}]}]}}`
	if err := os.WriteFile(opts.ClaudeSettingsPath, []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := SetupAgentHooks(opts); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	root := readHooksFile(t, opts.ClaudeSettingsPath)
	if root["model"] != "opus" {
		t.Fatal("existing top-level settings lost")
	}
	hooks, _ := root["hooks"].(map[string]any)
	if _, ok := hooks["PreToolUse"]; !ok {
		t.Fatal("existing hook entries lost")
	}
	if _, ok := hooks["SessionStart"]; !ok {
		t.Fatal("new hook entries missing")
	}

	backups, _ := filepath.Glob(opts.ClaudeSettingsPath + ".bak-*")
	if len(backups) != 1 {
		t.Fatalf("expected one backup, got %d", len(backups))
	}
}

func TestSetupAgentHooksDryRunWritesNothing(t *testing.T) {
	opts, out := setupHooksTestOpts(t, true)
	if err := SetupAgentHooks(opts); err != nil {
		t.Fatalf("dry run failed: %v", err)
	}
	if _, err := os.Stat(opts.ClaudeSettingsPath); !os.IsNotExist(err) {
		t.Fatal("dry run must not write claude settings")
	}
	if _, err := os.Stat(opts.CodexHooksPath); !os.IsNotExist(err) {
		t.Fatal("dry run must not write codex hooks")
	}
	if _, err := os.Stat(opts.CopilotHooksPath); !os.IsNotExist(err) {
		t.Fatal("dry run must not write copilot hooks")
	}
	if !strings.Contains(out.String(), "would update") {
		t.Fatalf("expected dry-run preview, got: %s", out.String())
	}
}

func TestSetupAgentHooksHonoursCodexHome(t *testing.T) {
	root := t.TempDir()
	codexHome := filepath.Join(root, "custom-codex")
	t.Setenv("CODEX_HOME", codexHome)
	t.Setenv("COPILOT_HOME", filepath.Join(root, "copilot"))
	opts := SetupAgentHooksOptions{
		ClaudeSettingsPath: filepath.Join(root, "claude", "settings.json"),
		Stdout:             &bytes.Buffer{},
	}
	if err := SetupAgentHooks(opts); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(codexHome, "hooks.json")); err != nil {
		t.Fatalf("expected hooks under CODEX_HOME: %v", err)
	}
}

func TestSetupAgentHooksHonoursCopilotHome(t *testing.T) {
	root := t.TempDir()
	copilotHome := filepath.Join(root, "custom-copilot")
	t.Setenv("COPILOT_HOME", copilotHome)
	t.Setenv("CODEX_HOME", filepath.Join(root, "codex"))
	opts := SetupAgentHooksOptions{
		ClaudeSettingsPath: filepath.Join(root, "claude", "settings.json"),
		Stdout:             &bytes.Buffer{},
	}
	if err := SetupAgentHooks(opts); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(copilotHome, "hooks", "lazyworktree.json")); err != nil {
		t.Fatalf("expected hooks under COPILOT_HOME: %v", err)
	}
}

func TestChooseAgentEventShimCommand(t *testing.T) {
	if got := chooseAgentEventShimCommand("/current/lazyworktree", "/on/path/lazyworktree", true); got != agentEventShimMarker {
		t.Fatalf("expected stable bare command for matching PATH executable, got %q", got)
	}

	executable := filepath.Join(string(filepath.Separator), "tmp", "Lazy Worktree", "lazyworktree")
	want := quoteHookExecutable(executable) + " agent-event"
	if got := chooseAgentEventShimCommand(executable, "/old/lazyworktree", false); got != want {
		t.Fatalf("expected quoted current executable %q, got %q", want, got)
	}
}

func TestMergeAgentHooksRepairsExistingCommand(t *testing.T) {
	handler := map[string]any{
		"type":    "command",
		"command": "/old/lazyworktree agent-event --agent codex",
		"async":   true,
	}
	root := map[string]any{
		"hooks": map[string]any{
			"Stop": []any{map[string]any{"hooks": []any{handler}}},
		},
	}
	command := "'/new/Lazy Worktree/lazyworktree' agent-event --agent codex"
	if changed := mergeAgentHooks(root, []string{"Stop"}, command, false); !changed {
		t.Fatal("expected stale hook command to be repaired")
	}
	if got := handler["command"]; got != command {
		t.Fatalf("expected repaired command %q, got %q", command, got)
	}
	if _, ok := handler["async"]; ok {
		t.Fatal("expected unsupported async setting to be removed")
	}
	if changed := mergeAgentHooks(root, []string{"Stop"}, command, false); changed {
		t.Fatal("expected repaired hook to be idempotent")
	}
}

func TestMergeAgentHooksDoesNotRewriteUnrelatedAgentEvent(t *testing.T) {
	handler := map[string]any{
		"type":    "command",
		"command": "other-tool agent-event --agent codex",
	}
	root := map[string]any{
		"hooks": map[string]any{
			"Stop": []any{map[string]any{"hooks": []any{handler}}},
		},
	}
	command := "lazyworktree agent-event --agent codex"
	if changed := mergeAgentHooks(root, []string{"Stop"}, command, false); !changed {
		t.Fatal("expected lazyworktree hook to be added")
	}
	if got := handler["command"]; got != "other-tool agent-event --agent codex" {
		t.Fatalf("unrelated hook was rewritten to %q", got)
	}
}

func TestMergeCopilotHooksRepairsExistingCommand(t *testing.T) {
	entry := map[string]any{
		"type":    "command",
		"command": "/old/lazyworktree agent-event --agent copilot",
	}
	root := map[string]any{
		"version": float64(1),
		"hooks": map[string]any{
			"Stop": []any{entry},
		},
	}
	command := "'/new/Lazy Worktree/lazyworktree' agent-event --agent copilot"
	if changed := mergeCopilotHooks(root, []string{"Stop"}, command); !changed {
		t.Fatal("expected stale hook command to be repaired")
	}
	if got := entry["command"]; got != command {
		t.Fatalf("expected repaired command %q, got %q", command, got)
	}
	if changed := mergeCopilotHooks(root, []string{"Stop"}, command); changed {
		t.Fatal("expected repaired hook to be idempotent")
	}
}

func TestMergeCopilotHooksPreservesForeignEntries(t *testing.T) {
	foreign := map[string]any{
		"type":    "command",
		"command": "other-tool agent-event --agent copilot",
	}
	root := map[string]any{
		"version": float64(1),
		"hooks": map[string]any{
			"Stop": []any{foreign},
		},
	}
	command := "lazyworktree agent-event --agent copilot"
	if changed := mergeCopilotHooks(root, []string{"Stop"}, command); !changed {
		t.Fatal("expected lazyworktree hook to be added")
	}
	if got := foreign["command"]; got != "other-tool agent-event --agent copilot" {
		t.Fatalf("foreign hook was rewritten to %q", got)
	}
	entries, _ := root["hooks"].(map[string]any)["Stop"].([]any)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}

func TestMergeCopilotHooksSetsVersion(t *testing.T) {
	root := map[string]any{}
	command := "lazyworktree agent-event --agent copilot"
	if changed := mergeCopilotHooks(root, []string{"SessionStart"}, command); !changed {
		t.Fatal("expected change on empty config")
	}
	if root["version"] != 1 {
		t.Fatalf("expected version 1, got %v", root["version"])
	}
	if changed := mergeCopilotHooks(root, []string{"SessionStart"}, command); changed {
		t.Fatal("expected repeated merge on the same map to be idempotent")
	}
}

func TestIsCopilotHooksVersionOne(t *testing.T) {
	for _, version := range []any{1, int64(1), float64(1), json.Number("1")} {
		if !isCopilotHooksVersionOne(version) {
			t.Fatalf("expected version %T(%v) to be accepted", version, version)
		}
	}
	for _, version := range []any{nil, 2, float64(1.5), json.Number("invalid"), "1"} {
		if isCopilotHooksVersionOne(version) {
			t.Fatalf("expected version %T(%v) to be rejected", version, version)
		}
	}
}

func TestSetupAgentHooksEmptyConfigFile(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"zero bytes", ""},
		{"whitespace only", "  \n\t\n  "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, _ := setupHooksTestOpts(t, false)
			for _, path := range []string{opts.ClaudeSettingsPath, opts.CodexHooksPath, opts.CopilotHooksPath} {
				if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(tt.content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := SetupAgentHooks(opts); err != nil {
				t.Fatalf("setup with %s config failed: %v", tt.name, err)
			}
			claude := readHooksFile(t, opts.ClaudeSettingsPath)
			hooks, _ := claude["hooks"].(map[string]any)
			if _, ok := hooks["SessionStart"]; !ok {
				t.Fatal("hooks not installed into empty claude config")
			}
		})
	}
}

func TestIsAgentEventHookCommandCopilot(t *testing.T) {
	if !isAgentEventHookCommand("/some/path/lazyworktree agent-event --agent copilot") {
		t.Fatal("expected copilot shim command to be recognised")
	}
}
