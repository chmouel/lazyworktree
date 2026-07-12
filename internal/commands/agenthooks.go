package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/shlex"
)

// agentEventShimMarker identifies hook entries installed by lazyworktree.
const agentEventShimMarker = "lazyworktree agent-event"

type agentHookMatcher struct {
	Event   string
	Matcher string
}

// SetupAgentHooksOptions configures the hook installer.
type SetupAgentHooksOptions struct {
	DryRun             bool
	ClaudeSettingsPath string
	CodexHooksPath     string
	CopilotHooksPath   string
	Stdout             io.Writer
}

// SetupAgentHooks installs lazyworktree agent-event hooks into the Claude
// Code, Codex CLI, and Copilot CLI user-level hook configurations. Existing
// settings are preserved; a timestamped backup is written before any
// modification.
func SetupAgentHooks(opts SetupAgentHooksOptions) error {
	out := opts.Stdout
	if out == nil {
		out = os.Stdout
	}
	claudePath := opts.ClaudeSettingsPath
	codexPath := opts.CodexHooksPath
	copilotPath := opts.CopilotHooksPath
	if claudePath == "" || codexPath == "" || copilotPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolve home directory: %w", err)
		}
		if claudePath == "" {
			claudePath = filepath.Join(home, ".claude", "settings.json")
		}
		if codexPath == "" {
			codexHome := strings.TrimSpace(os.Getenv("CODEX_HOME"))
			if codexHome == "" {
				codexHome = filepath.Join(home, ".codex")
			}
			codexPath = filepath.Join(codexHome, "hooks.json")
		}
		if copilotPath == "" {
			copilotHome := strings.TrimSpace(os.Getenv("COPILOT_HOME"))
			if copilotHome == "" {
				copilotHome = filepath.Join(home, ".copilot")
			}
			copilotPath = filepath.Join(copilotHome, "hooks", "lazyworktree.json")
		}
	}
	shim := agentEventShimCommand()

	// Keep state hooks synchronous so their spool order matches agent events.
	if err := installHooksFile(out, opts.DryRun, "Claude Code", claudePath,
		[]string{"SessionStart", "UserPromptSubmit", "Stop", "SessionEnd"},
		shim+" --agent claude", false, []agentHookMatcher{
			{Event: "PreToolUse", Matcher: "AskUserQuestion"},
			{Event: "PostToolUse", Matcher: "AskUserQuestion"},
			{Event: "Elicitation", Matcher: "*"},
			{Event: "ElicitationResult", Matcher: "*"},
		}); err != nil {
		return err
	}
	if err := installHooksFile(out, opts.DryRun, "Codex CLI", codexPath,
		[]string{"SessionStart", "UserPromptSubmit", "Stop"},
		shim+" --agent codex", false, nil); err != nil {
		return err
	}
	if err := installCopilotHooksFile(out, opts.DryRun, copilotPath,
		[]string{"SessionStart", "UserPromptSubmit", "Stop", "SessionEnd"},
		shim+" --agent copilot", []agentHookMatcher{
			{Event: "PreToolUse", Matcher: "AskUserQuestion"},
			{Event: "PostToolUse", Matcher: "AskUserQuestion"},
		}); err != nil {
		return err
	}
	fmt.Fprintln(out, "\nNotes:")
	fmt.Fprintln(out, "- Codex CLI requires approving new hooks with the /hooks command inside a Codex session.")
	fmt.Fprintln(out, "- Copilot CLI loads hook files at startup; restart any running copilot session.")
	return nil
}

// agentEventShimCommand returns the command used to invoke the hook shim. The
// bare name is stable across upgrades, but is safe only when it resolves to
// the executable currently installing the hooks.
func agentEventShimCommand() string {
	executable, err := os.Executable()
	if err != nil {
		return agentEventShimMarker
	}
	pathExecutable, _ := exec.LookPath("lazyworktree")
	sameFile := false
	if pathExecutable != "" {
		executableInfo, executableErr := os.Stat(executable)
		pathInfo, pathErr := os.Stat(pathExecutable)
		sameFile = executableErr == nil && pathErr == nil && os.SameFile(executableInfo, pathInfo)
	}
	return chooseAgentEventShimCommand(executable, pathExecutable, sameFile)
}

func chooseAgentEventShimCommand(executable, pathExecutable string, sameFile bool) string {
	if executable == "" {
		return agentEventShimMarker
	}
	if pathExecutable != "" && sameFile {
		return agentEventShimMarker
	}
	return quoteHookExecutable(executable) + " agent-event"
}

func installHooksFile(
	out io.Writer,
	dryRun bool,
	label, path string,
	events []string,
	command string,
	async bool,
	matcherHooks []agentHookMatcher,
) error {
	root := map[string]any{}
	data, err := os.ReadFile(path) //nolint:gosec // User-level config path.
	switch {
	case err == nil:
		if len(bytes.TrimSpace(data)) > 0 {
			if err := json.Unmarshal(data, &root); err != nil {
				return fmt.Errorf("%s: parse %s: %w", label, path, err)
			}
		}
	case os.IsNotExist(err):
		data = nil
	default:
		return fmt.Errorf("%s: read %s: %w", label, path, err)
	}

	changed := mergeAgentHooks(root, events, command, async)
	for _, hook := range matcherHooks {
		changed = mergeAgentMatcherHook(root, hook.Event, hook.Matcher, command, async) || changed
	}
	if !changed {
		fmt.Fprintf(out, "%s: hooks already installed in %s\n", label, path)
		return nil
	}
	return writeHooksFile(out, dryRun, label, path, data, root)
}

// installCopilotHooksFile installs the shim into a Copilot CLI hook file
// using its native flat entry format. Lifecycle events use PascalCase names
// so the CLI emits VS Code compatible payloads; notification hooks retain
// Copilot's native event name and are normalised by the shim.
func installCopilotHooksFile(
	out io.Writer,
	dryRun bool,
	path string,
	events []string,
	command string,
	matcherHooks []agentHookMatcher,
) error {
	const label = "Copilot CLI"
	root := map[string]any{}
	data, err := os.ReadFile(path) //nolint:gosec // User-level config path.
	switch {
	case err == nil:
		if len(bytes.TrimSpace(data)) > 0 {
			if err := json.Unmarshal(data, &root); err != nil {
				return fmt.Errorf("%s: parse %s: %w", label, path, err)
			}
		}
	case os.IsNotExist(err):
		data = nil
	default:
		return fmt.Errorf("%s: read %s: %w", label, path, err)
	}

	changed := mergeCopilotHooks(root, events, command)
	for _, hook := range matcherHooks {
		changed = mergeCopilotMatcherHook(root, hook.Event, hook.Matcher, command) || changed
	}
	if !changed {
		fmt.Fprintf(out, "%s: hooks already installed in %s\n", label, path)
		return nil
	}
	return writeHooksFile(out, dryRun, label, path, data, root)
}

// writeHooksFile encodes root and writes it to path, backing up any previous
// content first. In dry-run mode it only prints the would-be content.
func writeHooksFile(out io.Writer, dryRun bool, label, path string, previous []byte, root map[string]any) error {
	updated, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("%s: encode %s: %w", label, path, err)
	}
	updated = append(updated, '\n')

	if dryRun {
		fmt.Fprintf(out, "%s: would update %s with:\n%s\n", label, path, updated)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("%s: create config dir: %w", label, err)
	}
	if previous != nil {
		backup := fmt.Sprintf("%s.bak-%s", path, time.Now().Format("20060102-150405"))
		if err := os.WriteFile(backup, previous, 0o600); err != nil {
			return fmt.Errorf("%s: write backup %s: %w", label, backup, err)
		}
		fmt.Fprintf(out, "%s: backed up existing config to %s\n", label, backup)
	}
	if err := writeConfigAtomically(path, updated); err != nil {
		return fmt.Errorf("%s: write %s: %w", label, path, err)
	}
	fmt.Fprintf(out, "%s: installed hooks into %s\n", label, path)
	return nil
}

// mergeCopilotHooks adds or repairs the shim command for each event in the
// Copilot CLI native format: a flat entry list per event with a required
// "version": 1 field at the top level. It reports whether anything changed.
func mergeCopilotHooks(root map[string]any, events []string, command string) bool {
	changed := false
	if !isCopilotHooksVersionOne(root["version"]) {
		root["version"] = 1
		changed = true
	}
	hooks, ok := root["hooks"].(map[string]any)
	if !ok {
		hooks = map[string]any{}
		root["hooks"] = hooks
	}
	for _, event := range events {
		entries, _ := hooks[event].([]any)
		found, updated := updateCopilotHook(entries, command)
		if found {
			changed = changed || updated
			continue
		}
		entries = append(entries, map[string]any{"type": "command", "command": command})
		hooks[event] = entries
		changed = true
	}
	return changed
}

func isCopilotHooksVersionOne(value any) bool {
	switch version := value.(type) {
	case int:
		return version == 1
	case int64:
		return version == 1
	case float64:
		return version == 1
	case json.Number:
		number, err := version.Float64()
		return err == nil && number == 1
	default:
		return false
	}
}

func mergeCopilotMatcherHook(root map[string]any, event, matcher, command string) bool {
	hooks, ok := root["hooks"].(map[string]any)
	if !ok {
		hooks = map[string]any{}
		root["hooks"] = hooks
	}
	entries, _ := hooks[event].([]any)
	for _, entry := range entries {
		entryMap, ok := entry.(map[string]any)
		if !ok || entryMap["matcher"] != matcher {
			continue
		}
		for _, field := range []string{"command", "bash", "powershell"} {
			cmd, _ := entryMap[field].(string)
			if !isAgentEventHookCommand(cmd) {
				continue
			}
			if cmd == command {
				return false
			}
			entryMap[field] = command
			return true
		}
	}
	hooks[event] = append(entries, map[string]any{
		"type":    "command",
		"matcher": matcher,
		"command": command,
	})
	return true
}

func updateCopilotHook(entries []any, command string) (bool, bool) {
	for _, entry := range entries {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		for _, field := range []string{"command", "bash", "powershell"} {
			cmd, _ := entryMap[field].(string)
			if !isAgentEventHookCommand(cmd) {
				continue
			}
			changed := false
			if cmd != command {
				entryMap[field] = command
				changed = true
			}
			return true, changed
		}
	}
	return false, false
}

// mergeAgentHooks adds or repairs the shim command for each event. It reports
// whether anything changed.
func mergeAgentHooks(root map[string]any, events []string, command string, async bool) bool {
	hooks, ok := root["hooks"].(map[string]any)
	if !ok {
		hooks = map[string]any{}
		root["hooks"] = hooks
	}
	changed := false
	for _, event := range events {
		groups, _ := hooks[event].([]any)
		found, updated := updateAgentHook(groups, command, async)
		if found {
			changed = changed || updated
			continue
		}
		handler := map[string]any{"type": "command", "command": command}
		if async {
			handler["async"] = true
		}
		groups = append(groups, map[string]any{"hooks": []any{handler}})
		hooks[event] = groups
		changed = true
	}
	return changed
}

func mergeAgentMatcherHook(root map[string]any, event, matcher, command string, async bool) bool {
	hooks, ok := root["hooks"].(map[string]any)
	if !ok {
		hooks = map[string]any{}
		root["hooks"] = hooks
	}
	groups, _ := hooks[event].([]any)
	for _, group := range groups {
		groupMap, ok := group.(map[string]any)
		if !ok || groupMap["matcher"] != matcher {
			continue
		}
		if found, updated := updateAgentHook([]any{group}, command, async); found {
			return updated
		}
		handler := map[string]any{"type": "command", "command": command}
		if async {
			handler["async"] = true
		}
		handlers, _ := groupMap["hooks"].([]any)
		groupMap["hooks"] = append(handlers, handler)
		return true
	}
	handler := map[string]any{"type": "command", "command": command}
	if async {
		handler["async"] = true
	}
	hooks[event] = append(groups, map[string]any{
		"matcher": matcher,
		"hooks":   []any{handler},
	})
	return true
}

func updateAgentHook(groups []any, command string, async bool) (bool, bool) {
	for _, group := range groups {
		groupMap, ok := group.(map[string]any)
		if !ok {
			continue
		}
		handlers, _ := groupMap["hooks"].([]any)
		for _, handler := range handlers {
			handlerMap, ok := handler.(map[string]any)
			if !ok {
				continue
			}
			cmd, _ := handlerMap["command"].(string)
			if isAgentEventHookCommand(cmd) {
				changed := false
				if cmd != command {
					handlerMap["command"] = command
					changed = true
				}
				if async {
					if current, ok := handlerMap["async"].(bool); !ok || !current {
						handlerMap["async"] = true
						changed = true
					}
				} else if _, ok := handlerMap["async"]; ok {
					delete(handlerMap, "async")
					changed = true
				}
				return true, changed
			}
		}
	}
	return false, false
}

func writeConfigAtomically(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func isAgentEventHookCommand(command string) bool {
	parts, err := shlex.Split(command)
	if err != nil || len(parts) < 2 {
		return false
	}
	executable := filepath.Base(parts[0])
	if strings.EqualFold(filepath.Ext(executable), ".exe") {
		executable = strings.TrimSuffix(executable, filepath.Ext(executable))
	}
	if parts[1] != "agent-event" {
		return false
	}
	if strings.EqualFold(executable, "lazyworktree") {
		return true
	}
	current, err := os.Executable()
	if err == nil && strings.EqualFold(filepath.Clean(parts[0]), filepath.Clean(current)) {
		return true
	}
	return false
}
