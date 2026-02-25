package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/chmouel/lazyworktree/internal/config"
	log "github.com/chmouel/lazyworktree/internal/log"
	"github.com/chmouel/lazyworktree/internal/models"
)

// CommandRunner is a function type for creating exec.Cmd instances.
type CommandRunner func(ctx context.Context, name string, args ...string) *exec.Cmd

// TerminalTabLauncher launches commands in new terminal tabs.
type TerminalTabLauncher interface {
	// Name returns the terminal name for display.
	Name() string
	// IsAvailable checks if running inside this terminal.
	IsAvailable() bool
	// Launch opens a new tab with the given command.
	// Returns the tab title on success.
	Launch(ctx context.Context, cmd, cwd, title string, env map[string]string) (string, error)
}

func debugf(format string, args ...any) {
	log.Printf(format, args...)
}

// KittyLauncher implements TerminalTabLauncher for Kitty terminal.
type KittyLauncher struct {
	commandRunner CommandRunner
}

// Name returns "Kitty".
func (k *KittyLauncher) Name() string { return "Kitty" }

// IsAvailable checks if running inside Kitty terminal.
func (k *KittyLauncher) IsAvailable() bool {
	return os.Getenv("KITTY_WINDOW_ID") != ""
}

// Launch opens a new Kitty tab with the given command.
func (k *KittyLauncher) Launch(ctx context.Context, cmd, cwd, title string, env map[string]string) (string, error) {
	args := []string{"@", "launch", "--type=tab", "--cwd=" + cwd, "--tab-title=" + title}
	for key, val := range env {
		args = append(args, "--env="+key+"="+val)
	}
	args = append(args, "--", "bash", "-lc", cmd)

	debugf("kitty %s", strings.Join(args, " "))
	c := k.commandRunner(ctx, "kitty", args...)
	output, err := c.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to launch Kitty tab: %w (%s)", err, string(output))
	}
	return title, nil
}

// WezTermLauncher implements TerminalTabLauncher for WezTerm terminal.
type WezTermLauncher struct {
	commandRunner CommandRunner
}

// Name returns "WezTerm".
func (w *WezTermLauncher) Name() string { return "WezTerm" }

// IsAvailable checks if running inside WezTerm.
func (w *WezTermLauncher) IsAvailable() bool {
	return os.Getenv("WEZTERM_PANE") != "" || os.Getenv("WEZTERM_UNIX_SOCKET") != ""
}

// Launch opens a new WezTerm tab with the given command.
func (w *WezTermLauncher) Launch(ctx context.Context, cmd, cwd, title string, env map[string]string) (string, error) {
	args := []string{"cli", "spawn", "--cwd", cwd, "--"}
	if len(env) > 0 {
		args = append(args, "env")
		for key, val := range env {
			args = append(args, fmt.Sprintf("%s=%s", key, val))
		}
	}
	args = append(args, "bash", "-lc", cmd)

	c := w.commandRunner(ctx, "wezterm", args...)
	output, err := c.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to launch WezTerm tab: %w (%s)", err, string(output))
	}

	if title != "" {
		fields := strings.Fields(string(output))
		if len(fields) > 0 {
			if err := w.setTabTitle(ctx, fields[0], title); err != nil {
				// Best-effort: tab is already created, so ignore title failures.
				return title, nil
			}
		}
	}

	return title, nil
}

func (w *WezTermLauncher) setTabTitle(ctx context.Context, paneID, title string) error {
	args := []string{"cli", "set-tab-title", "--pane-id", paneID, title}
	c := w.commandRunner(ctx, "wezterm", args...)
	output, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set WezTerm tab title: %w (%s)", err, string(output))
	}
	return nil
}

// ITermLauncher implements TerminalTabLauncher for iTerm.
type ITermLauncher struct {
	commandRunner CommandRunner
}

// Name returns "iTerm".
func (i *ITermLauncher) Name() string { return "iTerm" }

// IsAvailable checks if running inside iTerm.
func (i *ITermLauncher) IsAvailable() bool {
	return os.Getenv("ITERM_SESSION_ID") != "" || os.Getenv("TERM_PROGRAM") == "iTerm.app"
}

// Launch opens a new iTerm tab with the given command.
func (i *ITermLauncher) Launch(ctx context.Context, cmd, cwd, title string, env map[string]string) (string, error) {
	script := `on run argv
set cmd to item 1 of argv
set tabTitle to item 2 of argv
tell application "iTerm"
	activate
	if (count of windows) = 0 then
		set targetWindow to (create window with default profile)
	else
		set targetWindow to current window
		tell targetWindow to create tab with default profile
	end if
	tell current session of targetWindow
		write text cmd
		if tabTitle is not "" then
			set name to tabTitle
		end if
	end tell
end tell
end run`

	command := buildShellCommand(cmd, cwd, env)
	args := []string{"-e", script, "--", command, title}
	c := i.commandRunner(ctx, "osascript", args...)
	output, err := c.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to launch iTerm tab: %w (%s)", err, string(output))
	}
	return title, nil
}

func buildShellCommand(cmd, cwd string, env map[string]string) string {
	command := cmd
	if len(env) > 0 {
		pairs := make([]string, 0, len(env))
		keys := make([]string, 0, len(env))
		for key := range env {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			val := env[key]
			pairs = append(pairs, fmt.Sprintf("%s=%s", key, shellQuote(val)))
		}
		command = strings.Join(pairs, " ") + " " + cmd
	}
	script := fmt.Sprintf("cd %s && %s", shellQuote(cwd), command)
	return "bash -lc " + shellQuote(script)
}

// detectTerminalLauncher returns the first available terminal launcher.
func detectTerminalLauncher(runner CommandRunner) TerminalTabLauncher {
	launchers := []TerminalTabLauncher{
		&KittyLauncher{commandRunner: runner},
		&WezTermLauncher{commandRunner: runner},
		&ITermLauncher{commandRunner: runner},
		// Future: &GhosttyLauncher{}, etc.
	}
	for _, l := range launchers {
		if l.IsAvailable() {
			return l
		}
	}
	return nil
}

const terminalTabLabel = "terminal tab"

type terminalTabReadyMsg struct {
	terminalName string
	tabTitle     string
	err          error
}

// writeCommandScript writes a command string to a self-deleting temp script
// file. This is used when the command contains newlines, since terminal remote
// control protocols (Kitty, WezTerm) may not handle multi-line positional
// arguments correctly.
func writeCommandScript(cmd string) (string, error) {
	f, err := os.CreateTemp("", "lazyworktree-tab-*.sh")
	if err != nil {
		return "", fmt.Errorf("failed to create temp script: %w", err)
	}
	path := f.Name()
	if _, err := f.WriteString(cmd); err != nil {
		_ = f.Close()
		_ = os.Remove(path) //nolint:gosec
		return "", fmt.Errorf("failed to write temp script: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path) //nolint:gosec
		return "", fmt.Errorf("failed to close temp script: %w", err)
	}
	if err := os.Chmod(path, 0o700); err != nil { //nolint:gosec // script must be executable to run in the new tab
		_ = os.Remove(path) //nolint:gosec
		return "", fmt.Errorf("failed to chmod temp script: %w", err)
	}
	return path, nil
}

func (m *Model) openTerminalTab(customCmd *config.CustomCommand, wt *models.WorktreeInfo) tea.Cmd {
	if customCmd == nil || customCmd.Command == "" {
		return nil
	}

	launcher := detectTerminalLauncher(m.commandRunner)
	if launcher == nil {
		return func() tea.Msg {
			return terminalTabReadyMsg{err: fmt.Errorf("no supported terminal detected (Kitty, WezTerm, or iTerm required)")}
		}
	}

	env := m.buildCommandEnv(wt.Branch, wt.Path)
	// Forward the current PATH so tools (tmux, zellij, etc.) available to
	// lazyworktree are also reachable in the new tab.
	if p := os.Getenv("PATH"); p != "" {
		env["PATH"] = p
	}
	title := customCmd.Description
	if title == "" {
		title = filepath.Base(wt.Path)
	}

	// When the command contains newlines (e.g. generated tmux/zellij scripts),
	// write it to a temp file so the launcher always receives a short,
	// single-line command. Terminal remote control protocols may truncate or
	// misparse multi-line positional arguments.
	cmd := customCmd.Command
	if strings.Contains(cmd, "\n") {
		scriptPath, err := writeCommandScript(cmd)
		if err != nil {
			return func() tea.Msg {
				return terminalTabReadyMsg{err: err}
			}
		}

		cmd = fmt.Sprintf("cd %s && bash %s; rm -f %s", shellQuote(wt.Path), shellQuote(scriptPath), shellQuote(scriptPath))
	}

	return func() tea.Msg {
		tabTitle, err := launcher.Launch(m.ctx, cmd, wt.Path, title, env)
		return terminalTabReadyMsg{
			terminalName: launcher.Name(),
			tabTitle:     tabTitle,
			err:          err,
		}
	}
}
