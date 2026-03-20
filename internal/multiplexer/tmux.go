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

// BuildTmuxWindowCommand builds the command string for a tmux window with environment exports.
func BuildTmuxWindowCommand(command string, env map[string]string) string {
	prefix := ExportEnvCommand(env)
	if prefix != "" {
		prefix += " "
	}
	if strings.TrimSpace(command) == "" {
		return prefix + "exec ${SHELL:-bash}"
	}
	return prefix + command
}

// BuildTmuxScript generates a shell script that creates or attaches to a tmux session.
// The script handles session existence based on tmuxCfg.OnExists and optionally attaches if tmuxCfg.Attach is true.
func BuildTmuxScript(sessionName string, tmuxCfg *config.TmuxCommand, windows []ResolvedWindow, env map[string]string) string {
	onExists := strings.ToLower(strings.TrimSpace(tmuxCfg.OnExists))
	switch onExists {
	case OnExistsAttach, OnExistsKill, OnExistsNew, OnExistsSwitch:
	default:
		onExists = OnExistsSwitch
	}

	var b strings.Builder
	b.WriteString("set -e\n")
	fmt.Fprintf(&b, "session=%s\n", ShellQuote(sessionName))
	b.WriteString("base_session=$session\n")
	b.WriteString("tmux_exact_target() { printf '=%s' \"$1\"; }\n")
	b.WriteString("if tmux has-session -t \"$(tmux_exact_target \"$session\")\" 2>/dev/null; then\n")
	switch onExists {
	case OnExistsKill:
		b.WriteString("  tmux kill-session -t \"$(tmux_exact_target \"$session\")\"\n")
	case OnExistsNew:
		b.WriteString("  i=2\n")
		b.WriteString("  while tmux has-session -t \"$(tmux_exact_target \"${base_session}-$i\")\" 2>/dev/null; do i=$((i+1)); done\n")
		b.WriteString("  session=\"${base_session}-$i\"\n")
	default:
		b.WriteString("  :\n")
	}
	b.WriteString("fi\n")
	b.WriteString("if ! tmux has-session -t \"$(tmux_exact_target \"$session\")\" 2>/dev/null; then\n")
	if len(windows) == 0 {
		return ""
	}
	first := windows[0]
	fmt.Fprintf(&b, "  tmux new-session -d -s \"$session\" -n %s -c %s -- bash -lc %s\n",
		ShellQuote(first.Name), ShellQuote(first.Cwd), ShellQuote(first.Command))

	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(&b, "  tmux set-environment -t \"$session\" %s %s\n", ShellQuote(key), ShellQuote(env[key]))
	}

	for _, window := range windows[1:] { //#nosec G602 -- slice bounds checked above (len(windows)==0 returns empty string)
		fmt.Fprintf(&b, "  tmux new-window -t \"$session\" -n %s -c %s -- bash -lc %s\n",
			ShellQuote(window.Name), ShellQuote(window.Cwd), ShellQuote(window.Command))
	}
	b.WriteString("fi\n")
	b.WriteString("if [ -n \"${LW_TMUX_SESSION_FILE:-}\" ]; then printf '%s' \"$session\" > \"$LW_TMUX_SESSION_FILE\"; fi\n")

	if tmuxCfg.Attach {
		if onExists == OnExistsAttach {
			b.WriteString("tmux attach -t \"$(tmux_exact_target \"$session\")\" || true\n")
		} else {
			b.WriteString("if [ -n \"$TMUX\" ]; then tmux switch-client -t \"$(tmux_exact_target \"$session\")\" || true; else tmux attach -t \"$(tmux_exact_target \"$session\")\" || true; fi\n")
		}
	}
	return b.String()
}
