package multiplexer

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/chmouel/lazyworktree/internal/app/services"
	"github.com/chmouel/lazyworktree/internal/config"
)

// SanitizeTmuxSessionName removes invalid characters from a tmux session name.
func SanitizeTmuxSessionName(name string) string {
	if name == "" {
		return ""
	}
	replacer := strings.NewReplacer(":", "-", "/", "-", "\\", "-")
	return replacer.Replace(name)
}

// ReadSessionFile reads the session name from a file, returning fallback if the file cannot be read or is empty.
func ReadSessionFile(path, fallback string) string {
	// #nosec G304 G703 -- file path is created by the current process
	data, err := os.ReadFile(path)
	if err != nil {
		return fallback
	}
	value := strings.TrimSpace(string(data))
	if value == "" {
		return fallback
	}
	return value
}

// ResolveTmuxWindows resolves window configurations by expanding environment variables.
// Returns the resolved windows and a boolean indicating success.
func ResolveTmuxWindows(windows []config.TmuxWindow, env map[string]string, defaultCwd string) ([]ResolvedWindow, bool) {
	if len(windows) == 0 {
		return nil, false
	}
	resolved := make([]ResolvedWindow, 0, len(windows))
	for i, window := range windows {
		name := strings.TrimSpace(services.ExpandWithEnv(window.Name, env))
		if name == "" {
			name = fmt.Sprintf("window-%d", i+1)
		}
		cwd := strings.TrimSpace(services.ExpandWithEnv(window.Cwd, env))
		if cwd == "" {
			cwd = defaultCwd
		}
		command := strings.TrimSpace(window.Command)
		command = BuildTmuxWindowCommand(command, env)
		resolved = append(resolved, ResolvedWindow{
			Name:    name,
			Command: command,
			Cwd:     cwd,
		})
	}
	return resolved, true
}

// defaultWindowShellCommand keeps a window alive with the user's shell when no
// command is configured. Multiplexers share it so empty windows still receive a
// command that container wrapping can replace.
const defaultWindowShellCommand = "exec ${SHELL:-bash}"

// BuildTmuxWindowCommand builds the command string for a tmux window with environment exports.
func BuildTmuxWindowCommand(command string, env map[string]string) string {
	prefix := ExportEnvCommand(env)
	if prefix != "" {
		prefix += " "
	}
	if strings.TrimSpace(command) == "" {
		return prefix + defaultWindowShellCommand
	}
	return prefix + command
}

// TmuxWorktreeOption is the tmux session user option holding the worktree tag.
const TmuxWorktreeOption = "@lazyworktree_path"

// TmuxListTargetsFormat lists sessions as ID, worktree tag and name. The name
// comes last so it may contain tabs; tags never do (see TmuxWorktreeKey).
const TmuxListTargetsFormat = "#{session_id}\t#{" + TmuxWorktreeOption + "}\t#{session_name}"

// TmuxWorktreeKey returns the tag recorded on a tmux session for a worktree,
// or an empty string when the path cannot be stored in the list format.
func TmuxWorktreeKey(path string) string {
	key := WorktreeKey(path)
	if strings.ContainsAny(key, "\t\n") {
		return ""
	}
	return key
}

// ParseTmuxTargets parses `tmux list-sessions -F TmuxListTargetsFormat` output.
func ParseTmuxTargets(output string) []Target {
	var targets []Target
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSuffix(line, "\r")
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) != 3 || fields[0] == "" || fields[2] == "" {
			continue
		}
		targets = append(targets, Target{ID: fields[0], Key: fields[1], Name: fields[2]})
	}
	return targets
}

// TmuxSessionPlan is the session a tmux script reuses or creates.
type TmuxSessionPlan struct {
	// Name is the session to create, or the actual name of the reused session.
	Name string
	// ExistingID is the session to reuse; empty creates Name.
	ExistingID string
	// KillID is a session to kill before creating Name.
	KillID string
	// KillName is the name KillID had when it was listed.
	KillName string
	// Increment adds a numeric suffix when Name is taken (on_exists: new).
	Increment bool
	// Key is the worktree tag to record; empty skips tagging.
	Key string
}

// NormalizeOnExists returns a supported on_exists value, defaulting to switch.
func NormalizeOnExists(onExists string) string {
	onExists = strings.ToLower(strings.TrimSpace(onExists))
	switch onExists {
	case OnExistsAttach, OnExistsKill, OnExistsNew, OnExistsSwitch:
		return onExists
	default:
		return OnExistsSwitch
	}
}

// PlanTmuxSession decides which session a worktree's tmux command reuses,
// replaces or creates, given the current sessions.
func PlanTmuxSession(targets []Target, key, name, onExists string, legacyNames ...string) (TmuxSessionPlan, error) {
	plan := TmuxSessionPlan{Name: name, Key: key}
	sanitised := make([]string, 0, len(legacyNames))
	for _, legacy := range legacyNames {
		sanitised = append(sanitised, SanitizeTmuxSessionName(legacy))
	}
	legacyNames = sanitised
	switch NormalizeOnExists(onExists) {
	case OnExistsNew:
		plan.Increment = true
		return plan, nil
	case OnExistsKill:
		resolution, err := ResolveForKill(targets, key, name, legacyNames...)
		if err != nil {
			return TmuxSessionPlan{}, err
		}
		if resolution.Found {
			plan.KillID = resolution.Target.ID
			plan.KillName = resolution.Target.Name
		}
		return plan, nil
	default:
		resolution, err := ResolveTarget(targets, key, name, legacyNames...)
		if err != nil {
			return TmuxSessionPlan{}, err
		}
		if resolution.Found {
			plan.Name = resolution.Target.Name
			plan.ExistingID = resolution.Target.ID
		}
		return plan, nil
	}
}

// BuildTmuxScript builds a script that carries out plan. Sessions are always
// addressed by ID once known, and creation fails rather than reusing a session
// that took the name in the meantime.
func BuildTmuxScript(plan TmuxSessionPlan, tmuxCfg *config.TmuxCommand, windows []ResolvedWindow, env map[string]string) string {
	if len(windows) == 0 {
		return ""
	}
	onExists := NormalizeOnExists(tmuxCfg.OnExists)

	var b strings.Builder
	b.WriteString("set -e\n")
	fmt.Fprintf(&b, "session=%s\n", ShellQuote(plan.Name))
	fmt.Fprintf(&b, "session_id=%s\n", ShellQuote(plan.ExistingID))
	// The trailing colon stops tmux reading a "." in the name as a pane separator.
	b.WriteString("tmux_exact_target() { printf '=%s:' \"$1\"; }\n")
	// Sessions may change between listing and running the script, and IDs are
	// reused after a server restart, so check the name and owner again before
	// reusing, tagging or killing a listed session.
	fmt.Fprintf(&b, "worktree_key=%s\n", ShellQuote(plan.Key))
	b.WriteString("tmux_still_ours() {\n")
	b.WriteString("  [ \"$(tmux display-message -p -t \"$1\" '#{session_name}' 2>/dev/null)\" = \"$2\" ] || return 1\n")
	fmt.Fprintf(&b, "  owner=$(tmux display-message -p -t \"$1\" '#{%s}' 2>/dev/null)\n", TmuxWorktreeOption)
	b.WriteString("  [ -z \"$owner\" ] || [ -z \"$worktree_key\" ] || [ \"$owner\" = \"$worktree_key\" ]\n")
	b.WriteString("}\n")
	b.WriteString("tmux_changed() { printf 'lazyworktree: tmux session %s changed since it was listed, try again\\n' \"$1\" >&2; exit 1; }\n")
	b.WriteString("if [ -n \"$session_id\" ]; then\n")
	b.WriteString("  if ! tmux has-session -t \"$session_id\" 2>/dev/null; then session_id=\n")
	b.WriteString("  elif ! tmux_still_ours \"$session_id\" \"$session\"; then tmux_changed \"$session\"; fi\n")
	b.WriteString("fi\n")
	if plan.KillID != "" {
		fmt.Fprintf(&b, "kill_id=%s\n", ShellQuote(plan.KillID))
		fmt.Fprintf(&b, "kill_name=%s\n", ShellQuote(plan.KillName))
		b.WriteString("if tmux has-session -t \"$kill_id\" 2>/dev/null; then\n")
		b.WriteString("  tmux_still_ours \"$kill_id\" \"$kill_name\" || tmux_changed \"$kill_name\"\n")
		b.WriteString("  holder=$(tmux display-message -p -t \"$(tmux_exact_target \"$session\")\" '#{session_id}' 2>/dev/null || true)\n")
		b.WriteString("  if [ -n \"$holder\" ] && [ \"$holder\" != \"$kill_id\" ]; then tmux_changed \"$session\"; fi\n")
		b.WriteString("  tmux kill-session -t \"$kill_id\"\n")
		b.WriteString("fi\n")
	}
	if plan.Increment {
		b.WriteString("base_session=$session\n")
		b.WriteString("if tmux has-session -t \"$(tmux_exact_target \"$session\")\" 2>/dev/null; then\n")
		b.WriteString("  i=2\n")
		b.WriteString("  while tmux has-session -t \"$(tmux_exact_target \"${base_session}-$i\")\" 2>/dev/null; do i=$((i+1)); done\n")
		b.WriteString("  session=\"${base_session}-$i\"\n")
		b.WriteString("fi\n")
	}

	b.WriteString("if [ -z \"$session_id\" ]; then\n")
	first := windows[0]
	fmt.Fprintf(&b, "  session_id=$(tmux new-session -d -P -F '#{session_id}' -s \"$(printf '%%s' \"$session\" | sed 's/#/##/g')\" -n %s -c %s -- bash -lc %s)\n",
		ShellQuote(first.Name), ShellQuote(first.Cwd), ShellQuote(first.Command))

	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(&b, "  tmux set-environment -t \"$session_id\" %s %s\n", ShellQuote(key), ShellQuote(env[key]))
	}

	for _, window := range windows[1:] {
		fmt.Fprintf(&b, "  tmux new-window -t \"$session_id:\" -n %s -c %s -- bash -lc %s\n",
			ShellQuote(window.Name), ShellQuote(window.Cwd), ShellQuote(window.Command))
	}
	b.WriteString("fi\n")
	if plan.Key != "" {
		fmt.Fprintf(&b, "tmux set-option -t \"$session_id\" %s \"$worktree_key\"\n", TmuxWorktreeOption)
	}
	b.WriteString("session=$(tmux display-message -p -t \"$session_id\" '#{session_name}')\n")
	b.WriteString("if [ -n \"${LW_TMUX_SESSION_FILE:-}\" ]; then printf '%s' \"$session\" > \"$LW_TMUX_SESSION_FILE\"; fi\n")

	if tmuxCfg.Attach {
		if onExists == OnExistsAttach {
			b.WriteString("tmux attach -t \"$session_id\" || true\n")
		} else {
			b.WriteString("if [ -n \"$TMUX\" ]; then tmux switch-client -t \"$session_id\" || true; else tmux attach -t \"$session_id\" || true; fi\n")
		}
	}
	return b.String()
}
