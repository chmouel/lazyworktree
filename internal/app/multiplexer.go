package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

const (
	zellijSessionLabel = "zellij session"
	tmuxSessionLabel   = "tmux session"
)

type (
	zellijSessionReadyMsg struct {
		sessionName  string
		attach       bool
		insideZellij bool
	}
	tmuxSessionReadyMsg struct {
		sessionName string
		attach      bool
		insideTmux  bool
	}
	resolvedTmuxWindow struct {
		Name    string
		Command string
		Cwd     string
	}
)

func cleanupZellijLayouts(paths []string) {
	for _, path := range paths {
		_ = os.Remove(path) //#nosec G703 -- controlled temp file cleanup
	}
}

func buildZellijInfoMessage(sessionName string) string {
	quoted := shellQuote(sessionName)
	return fmt.Sprintf("zellij session ready.\n\nAttach with:\n\n  zellij attach %s", quoted)
}

func (m *Model) attachZellijSessionCmd(sessionName string) tea.Cmd {
	// #nosec G204 -- zellij session name comes from user configuration.
	c := m.commandRunner(m.ctx, "zellij", onExistsAttach, sessionName)
	return m.execProcess(c, func(err error) tea.Msg {
		if err != nil {
			return errMsg{err: err}
		}
		return refreshCompleteMsg{}
	})
}

// getZellijActiveSessions queries zellij for all sessions starting with the configured session prefix
// Returns session names with the prefix stripped, or empty slice if zellij is unavailable.
func (m *Model) getZellijActiveSessions() []string {
	// Check if zellij is available
	if _, err := exec.LookPath("zellij"); err != nil {
		return nil
	}

	// Query zellij for session list
	// #nosec G204 -- static command with format string
	cmd := m.commandRunner(m.ctx, "zellij", "list-sessions", "--short")
	output, err := cmd.Output()
	if err != nil {
		// zellij not running or no sessions
		return nil
	}

	// Parse output and filter for worktree session prefix
	var sessions []string
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, m.config.SessionPrefix) {
			// Strip worktree prefix
			sessionName := strings.TrimPrefix(line, m.config.SessionPrefix)
			if sessionName != "" {
				sessions = append(sessions, sessionName)
			}
		}
	}

	// Sort alphabetically for consistent display
	sort.Strings(sessions)
	return sessions
}

func sanitizeZellijSessionName(name string) string {
	if name == "" {
		return ""
	}
	replacer := strings.NewReplacer("/", "-", "\\", "-", ":", "-")
	return replacer.Replace(name)
}

func buildTmuxInfoMessage(sessionName string, insideTmux bool) string {
	quoted := shellQuote(sessionName)
	if insideTmux {
		return fmt.Sprintf("tmux session ready.\n\nSwitch with:\n\n  tmux switch-client -t %s", quoted)
	}
	return fmt.Sprintf("tmux session ready.\n\nAttach with:\n\n  tmux attach-session -t %s", quoted)
}

func (m *Model) attachTmuxSessionCmd(sessionName string, insideTmux bool) tea.Cmd {
	args := []string{"attach-session", "-t", sessionName}
	if insideTmux {
		args = []string{"switch-client", "-t", sessionName}
	}
	// #nosec G204 -- tmux session name comes from user configuration.
	c := m.commandRunner(m.ctx, "tmux", args...)
	return m.execProcess(c, func(err error) tea.Msg {
		if err != nil {
			return errMsg{err: err}
		}
		return refreshCompleteMsg{}
	})
}

func readTmuxSessionFile(path, fallback string) string {
	// #nosec G304 -- file path is created by the current process.
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

func buildTmuxScript(sessionName string, tmuxCfg *config.TmuxCommand, windows []resolvedTmuxWindow, env map[string]string) string {
	onExists := strings.ToLower(strings.TrimSpace(tmuxCfg.OnExists))
	switch onExists {
	case onExistsAttach, onExistsKill, onExistsNew, onExistsSwitch:
	default:
		onExists = onExistsSwitch
	}

	var b strings.Builder
	b.WriteString("set -e\n")
	fmt.Fprintf(&b, "session=%s\n", shellQuote(sessionName))
	b.WriteString("base_session=$session\n")
	b.WriteString("if tmux has-session -t \"$session\" 2>/dev/null; then\n")
	switch onExists {
	case onExistsKill:
		b.WriteString("  tmux kill-session -t \"$session\"\n")
	case onExistsNew:
		b.WriteString("  i=2\n")
		b.WriteString("  while tmux has-session -t \"${base_session}-$i\" 2>/dev/null; do i=$((i+1)); done\n")
		b.WriteString("  session=\"${base_session}-$i\"\n")
	default:
		b.WriteString("  :\n")
	}
	b.WriteString("fi\n")
	b.WriteString("if ! tmux has-session -t \"$session\" 2>/dev/null; then\n")
	if len(windows) == 0 {
		return ""
	}
	first := windows[0]
	fmt.Fprintf(&b, "  tmux new-session -d -s \"$session\" -n %s -c %s -- bash -lc %s\n",
		shellQuote(first.Name), shellQuote(first.Cwd), shellQuote(first.Command))

	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(&b, "  tmux set-environment -t \"$session\" %s %s\n", shellQuote(key), shellQuote(env[key]))
	}

	for _, window := range windows[1:] {
		fmt.Fprintf(&b, "  tmux new-window -t \"$session\" -n %s -c %s -- bash -lc %s\n",
			shellQuote(window.Name), shellQuote(window.Cwd), shellQuote(window.Command))
	}
	b.WriteString("fi\n")
	b.WriteString("if [ -n \"${LW_TMUX_SESSION_FILE:-}\" ]; then printf '%s' \"$session\" > \"$LW_TMUX_SESSION_FILE\"; fi\n")

	if tmuxCfg.Attach {
		if onExists == onExistsAttach {
			b.WriteString("tmux attach -t \"$session\" || true\n")
		} else {
			b.WriteString("if [ -n \"$TMUX\" ]; then tmux switch-client -t \"$session\" || true; else tmux attach -t \"$session\" || true; fi\n")
		}
	}
	return b.String()
}

func buildZellijScript(sessionName string, zellijCfg *config.TmuxCommand, layoutPaths []string) string {
	onExists := strings.ToLower(strings.TrimSpace(zellijCfg.OnExists))
	switch onExists {
	case onExistsAttach, onExistsKill, onExistsNew, onExistsSwitch:
	default:
		onExists = onExistsSwitch
	}

	var b strings.Builder
	b.WriteString("set -e\n")
	fmt.Fprintf(&b, "session=%s\n", shellQuote(sessionName))
	b.WriteString("base_session=$session\n")
	b.WriteString("session_exists() {\n")
	b.WriteString("  zellij list-sessions --short --no-formatting 2>/dev/null | grep -Fxq \"$1\"\n")
	b.WriteString("}\n")
	b.WriteString("created=false\n")
	b.WriteString("if session_exists \"$session\"; then\n")
	switch onExists {
	case onExistsKill:
		b.WriteString("  zellij kill-session \"$session\"\n")
	case onExistsNew:
		b.WriteString("  i=2\n")
		b.WriteString("  while session_exists \"${base_session}-$i\"; do i=$((i+1)); done\n")
		b.WriteString("  session=\"${base_session}-$i\"\n")
	default:
		b.WriteString("  :\n")
	}
	b.WriteString("fi\n")
	b.WriteString("if ! session_exists \"$session\"; then\n")
	b.WriteString("  zellij attach --create-background \"$session\"\n")
	b.WriteString("  created=true\n")
	// Wait for session with timeout (5 seconds max)
	b.WriteString("  tries=0\n")
	b.WriteString("  while ! zellij list-sessions --short 2>/dev/null | grep -Fxq \"$session\"; do\n")
	b.WriteString("    sleep 0.1\n")
	b.WriteString("    tries=$((tries+1))\n")
	b.WriteString("    if [ $tries -ge 50 ]; then echo \"Timeout waiting for zellij session\" >&2; exit 1; fi\n")
	b.WriteString("  done\n")
	b.WriteString("fi\n")
	if len(layoutPaths) > 0 {
		b.WriteString("if [ \"$created\" = \"true\" ]; then\n")
		for _, layoutPath := range layoutPaths {
			fmt.Fprintf(&b, "  ZELLIJ_SESSION_NAME=\"$session\" zellij action new-tab --layout %s\n", shellQuote(layoutPath))
		}
		b.WriteString("  ZELLIJ_SESSION_NAME=\"$session\" zellij action go-to-tab 1\n")
		b.WriteString("  ZELLIJ_SESSION_NAME=\"$session\" zellij action close-tab\n")
		b.WriteString("fi\n")
	}
	b.WriteString("if [ -n \"${LW_ZELLIJ_SESSION_FILE:-}\" ]; then printf '%s' \"$session\" > \"$LW_ZELLIJ_SESSION_FILE\"; fi\n")
	return b.String()
}

func buildZellijTabLayout(window resolvedTmuxWindow) string {
	var b strings.Builder
	b.WriteString("layout {\n")
	fmt.Fprintf(&b, "    tab name=%s {\n", kdlQuote(window.Name))
	b.WriteString("        pane {\n")
	if window.Cwd != "" {
		fmt.Fprintf(&b, "            cwd %s\n", kdlQuote(window.Cwd))
	}
	fmt.Fprintf(&b, "            command %s\n", kdlQuote("bash"))
	fmt.Fprintf(&b, "            args %s %s\n", kdlQuote("-lc"), kdlQuote(window.Command))
	b.WriteString("        }\n")
	b.WriteString("    }\n")
	b.WriteString("}\n")
	return b.String()
}

func writeZellijLayouts(windows []resolvedTmuxWindow) ([]string, error) {
	paths := make([]string, 0, len(windows))
	for _, window := range windows {
		layoutFile, err := os.CreateTemp("", "lazyworktree-zellij-layout-")
		if err != nil {
			cleanupZellijLayouts(paths)
			return nil, err
		}
		if _, err := layoutFile.WriteString(buildZellijTabLayout(window)); err != nil {
			_ = layoutFile.Close()
			_ = os.Remove(layoutFile.Name()) //#nosec G703 -- controlled temp file cleanup
			cleanupZellijLayouts(paths)
			return nil, err
		}
		if err := layoutFile.Close(); err != nil {
			_ = os.Remove(layoutFile.Name()) //#nosec G703 -- controlled temp file cleanup
			cleanupZellijLayouts(paths)
			return nil, err
		}
		paths = append(paths, layoutFile.Name())
	}
	return paths, nil
}

func kdlQuote(input string) string {
	escaped := strings.ReplaceAll(input, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
	return "\"" + escaped + "\""
}

func sanitizeTmuxSessionName(name string) string {
	if name == "" {
		return ""
	}
	replacer := strings.NewReplacer(":", "-", "/", "-", "\\", "-")
	return replacer.Replace(name)
}

// getTmuxActiveSessions queries tmux for all sessions starting with the configured session prefix
// Returns session names with the prefix stripped, or empty slice if tmux is unavailable.
func (m *Model) getTmuxActiveSessions() []string {
	// Check if tmux is available
	if _, err := exec.LookPath("tmux"); err != nil {
		return nil
	}

	// Query tmux for session list
	// #nosec G204 -- static command with format string
	cmd := m.commandRunner(m.ctx, "tmux", "list-sessions", "-F", "#{session_name}")
	output, err := cmd.Output()
	if err != nil {
		// tmux not running or no sessions
		return nil
	}

	// Parse output and filter for worktree session prefix
	var sessions []string
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, m.config.SessionPrefix) {
			// Strip worktree prefix
			sessionName := strings.TrimPrefix(line, m.config.SessionPrefix)
			if sessionName != "" {
				sessions = append(sessions, sessionName)
			}
		}
	}

	// Sort alphabetically for consistent display
	sort.Strings(sessions)
	return sessions
}

func resolveTmuxWindows(windows []config.TmuxWindow, env map[string]string, defaultCwd string) ([]resolvedTmuxWindow, bool) {
	if len(windows) == 0 {
		return nil, false
	}
	resolved := make([]resolvedTmuxWindow, 0, len(windows))
	for i, window := range windows {
		name := strings.TrimSpace(expandWithEnv(window.Name, env))
		if name == "" {
			name = fmt.Sprintf("window-%d", i+1)
		}
		cwd := strings.TrimSpace(expandWithEnv(window.Cwd, env))
		if cwd == "" {
			cwd = defaultCwd
		}
		command := strings.TrimSpace(window.Command)
		command = buildTmuxWindowCommand(command, env)
		resolved = append(resolved, resolvedTmuxWindow{
			Name:    name,
			Command: command,
			Cwd:     cwd,
		})
	}
	return resolved, true
}

func buildTmuxWindowCommand(command string, env map[string]string) string {
	prefix := exportEnvCommand(env)
	if prefix != "" {
		prefix += " "
	}
	if strings.TrimSpace(command) == "" {
		return prefix + "exec ${SHELL:-bash}"
	}
	return prefix + command
}

func (m *Model) openTmuxSession(tmuxCfg *config.TmuxCommand, wt *models.WorktreeInfo) tea.Cmd {
	if tmuxCfg == nil {
		return nil
	}

	env := m.buildCommandEnv(wt.Branch, wt.Path)
	insideTmux := os.Getenv("TMUX") != ""
	sessionName := expandWithEnv(tmuxCfg.SessionName, env)
	if strings.TrimSpace(sessionName) == "" {
		sessionName = fmt.Sprintf("%s%s", m.config.SessionPrefix, filepath.Base(wt.Path))
	}
	sessionName = sanitizeTmuxSessionName(sessionName)

	resolved, ok := resolveTmuxWindows(tmuxCfg.Windows, env, wt.Path)
	if !ok {
		return func() tea.Msg {
			return errMsg{err: fmt.Errorf("failed to resolve tmux windows")}
		}
	}

	sessionFile, err := os.CreateTemp("", "lazyworktree-tmux-")
	if err != nil {
		return func() tea.Msg {
			return errMsg{err: err}
		}
	}
	sessionPath := sessionFile.Name()
	if closeErr := sessionFile.Close(); closeErr != nil {
		return func() tea.Msg {
			return errMsg{err: closeErr}
		}
	}

	scriptCfg := *tmuxCfg
	scriptCfg.Attach = false
	env["LW_TMUX_SESSION_FILE"] = sessionPath
	script := buildTmuxScript(sessionName, &scriptCfg, resolved, env)
	// #nosec G204 -- command is built from user-configured tmux session settings.
	c := m.commandRunner(m.ctx, "bash", "-lc", script)
	c.Dir = wt.Path
	c.Env = append(os.Environ(), envMapToList(env)...)

	return m.execProcess(c, func(err error) tea.Msg {
		defer func() {
			_ = os.Remove(sessionPath)
		}()
		if err != nil {
			return errMsg{err: err}
		}
		finalSession := readTmuxSessionFile(sessionPath, sessionName)
		return tmuxSessionReadyMsg{
			sessionName: finalSession,
			attach:      tmuxCfg.Attach,
			insideTmux:  insideTmux,
		}
	})
}

func (m *Model) openZellijSession(zellijCfg *config.TmuxCommand, wt *models.WorktreeInfo) tea.Cmd {
	if zellijCfg == nil {
		return nil
	}

	env := m.buildCommandEnv(wt.Branch, wt.Path)
	insideZellij := os.Getenv("ZELLIJ") != "" || os.Getenv("ZELLIJ_SESSION_NAME") != ""
	sessionName := strings.TrimSpace(expandWithEnv(zellijCfg.SessionName, env))
	if sessionName == "" {
		sessionName = fmt.Sprintf("%s%s", m.config.SessionPrefix, filepath.Base(wt.Path))
	}
	sessionName = sanitizeZellijSessionName(sessionName)

	resolved, ok := resolveTmuxWindows(zellijCfg.Windows, env, wt.Path)
	if !ok {
		return func() tea.Msg {
			return errMsg{err: fmt.Errorf("failed to resolve zellij windows")}
		}
	}

	layoutPaths, err := writeZellijLayouts(resolved)
	if err != nil {
		return func() tea.Msg {
			return errMsg{err: err}
		}
	}

	sessionFile, err := os.CreateTemp("", "lazyworktree-zellij-")
	if err != nil {
		cleanupZellijLayouts(layoutPaths)
		return func() tea.Msg {
			return errMsg{err: err}
		}
	}
	sessionPath := sessionFile.Name()
	if closeErr := sessionFile.Close(); closeErr != nil {
		cleanupZellijLayouts(layoutPaths)
		return func() tea.Msg {
			return errMsg{err: closeErr}
		}
	}

	scriptCfg := *zellijCfg
	scriptCfg.Attach = false
	env["LW_ZELLIJ_SESSION_FILE"] = sessionPath
	script := buildZellijScript(sessionName, &scriptCfg, layoutPaths)
	// #nosec G204 -- command is built from user-configured zellij session settings.
	c := m.commandRunner(m.ctx, "bash", "-lc", script)
	c.Dir = wt.Path
	c.Env = append(os.Environ(), envMapToList(env)...)

	return m.execProcess(c, func(err error) tea.Msg {
		defer func() {
			_ = os.Remove(sessionPath)
			cleanupZellijLayouts(layoutPaths)
		}()
		if err != nil {
			return errMsg{err: err}
		}
		finalSession := readTmuxSessionFile(sessionPath, sessionName)
		return zellijSessionReadyMsg{
			sessionName:  finalSession,
			attach:       zellijCfg.Attach,
			insideZellij: insideZellij,
		}
	})
}
