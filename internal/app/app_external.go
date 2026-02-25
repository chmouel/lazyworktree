package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

// openURLInBrowser opens the given URL in the default browser.
func (m *Model) openURLInBrowser(urlStr string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case osDarwin:
			// #nosec G204 -- the URL is executed directly as a single argument
			cmd = m.commandRunner(m.ctx, "open", urlStr)
		case osWindows:
			// #nosec G204 -- the URL is executed directly as a single argument
			cmd = m.commandRunner(m.ctx, "rundll32", "url.dll,FileProtocolHandler", urlStr)
		default:
			// #nosec G204 -- the URL is executed directly as a single argument
			cmd = m.commandRunner(m.ctx, "xdg-open", urlStr)
		}
		if err := m.startCommand(cmd); err != nil {
			return errMsg{err: err}
		}
		return nil
	}
}

func (m *Model) openPR() tea.Cmd {
	if m.state.data.selectedIndex < 0 || m.state.data.selectedIndex >= len(m.state.data.filteredWts) {
		return nil
	}
	wt := m.state.data.filteredWts[m.state.data.selectedIndex]

	// On main branch with merged/closed/no PR: open root repo in browser
	shouldOpenRepo := wt.IsMain && (wt.PR == nil || wt.PR.State == prStateMerged || wt.PR.State == prStateClosed)

	if shouldOpenRepo {
		return m.openRepoInBrowser()
	}

	// Otherwise, open PR in browser (existing behaviour)
	if wt.PR == nil {
		return nil
	}
	prURL, err := sanitizePRURL(wt.PR.URL)
	if err != nil {
		return func() tea.Msg { return errMsg{err: err} }
	}
	return m.openURLInBrowser(prURL)
}

func (m *Model) openRepoInBrowser() tea.Cmd {
	// Get remote URL
	remoteURL := strings.TrimSpace(m.state.services.git.RunGit(m.ctx, []string{"git", "remote", "get-url", "origin"}, "", []int{0}, true, false))
	if remoteURL == "" {
		return func() tea.Msg {
			return errMsg{err: fmt.Errorf("could not determine repository remote URL")}
		}
	}

	// Convert git URL to web URL
	webURL := m.gitURLToWebURL(remoteURL)
	if webURL == "" {
		return func() tea.Msg {
			return errMsg{err: fmt.Errorf("could not convert git URL to web URL")}
		}
	}

	return m.openURLInBrowser(webURL)
}

func (m *Model) openLazyGit() tea.Cmd {
	if m.state.data.selectedIndex < 0 || m.state.data.selectedIndex >= len(m.state.data.filteredWts) {
		return nil
	}
	wt := m.state.data.filteredWts[m.state.data.selectedIndex]

	c := m.commandRunner(m.ctx, "lazygit")
	c.Dir = wt.Path

	return m.execProcess(c, func(err error) tea.Msg {
		if err != nil {
			return errMsg{err: err}
		}
		return refreshCompleteMsg{}
	})
}

func (m *Model) openStatusFileInEditor(sf StatusFile) tea.Cmd {
	if m.state.data.selectedIndex < 0 || m.state.data.selectedIndex >= len(m.state.data.filteredWts) {
		return nil
	}
	wt := m.state.data.filteredWts[m.state.data.selectedIndex]

	editor := m.editorCommand()
	if strings.TrimSpace(editor) == "" {
		m.showInfo("No editor configured. Set editor in config or $EDITOR.", nil)
		return nil
	}

	filePath := filepath.Join(wt.Path, sf.Filename)
	if _, err := os.Stat(filePath); err != nil {
		m.showInfo(fmt.Sprintf("Cannot open %s: %v", sf.Filename, err), nil)
		return nil
	}

	env := m.buildCommandEnv(wt.Branch, wt.Path)
	envVars := os.Environ()
	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	cmdStr := fmt.Sprintf("%s %s", editor, shellQuote(sf.Filename))
	// #nosec G204 -- command is constructed from user config and controlled inputs
	c := m.commandRunner(m.ctx, "bash", "-c", cmdStr)
	c.Dir = wt.Path
	c.Env = envVars

	return m.execProcess(c, func(err error) tea.Msg {
		if err != nil {
			return errMsg{err: err}
		}
		return refreshCompleteMsg{}
	})
}

func (m *Model) executeCustomCommand(key string) tea.Cmd {
	customCmd, ok := m.config.CustomCommands[key]
	if !ok || customCmd == nil {
		return nil
	}

	if m.state.data.selectedIndex < 0 || m.state.data.selectedIndex >= len(m.state.data.filteredWts) {
		return nil
	}

	wt := m.state.data.filteredWts[m.state.data.selectedIndex]

	if customCmd.Zellij != nil {
		return m.openZellijSession(customCmd, wt)
	}

	if customCmd.Tmux != nil {
		return m.openTmuxSession(customCmd, wt)
	}

	if customCmd.NewTab {
		return m.openTerminalTab(customCmd, wt)
	}

	if customCmd.ShowOutput {
		return m.executeCustomCommandWithPager(customCmd, wt)
	}

	// Set environment variables
	env := m.buildCommandEnv(wt.Branch, wt.Path)
	envVars := filterWorktreeEnvVars(os.Environ())
	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	var c *exec.Cmd
	var cmdStr string
	if customCmd.Wait {
		// Wrap command with a pause prompt when wait is true
		cmdStr = fmt.Sprintf("%s; echo ''; echo 'Press any key to continue...'; read -n 1", customCmd.Command)
	} else {
		cmdStr = customCmd.Command
	}
	// Always run via shell to support pipes, redirects, and shell features
	// #nosec G204 -- command comes from user's own config file
	c = m.commandRunner(m.ctx, "bash", "-c", cmdStr)

	c.Dir = wt.Path
	c.Env = envVars

	return m.execProcess(c, func(err error) tea.Msg {
		if err != nil {
			return errMsg{err: err}
		}
		return refreshCompleteMsg{}
	})
}

func (m *Model) executeCustomCommandWithPager(customCmd *config.CustomCommand, wt *models.WorktreeInfo) tea.Cmd {
	env := m.buildCommandEnv(wt.Branch, wt.Path)
	envVars := filterWorktreeEnvVars(os.Environ())
	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	pager := m.pagerCommand()
	pagerEnv := m.pagerEnv(pager)
	pagerCmd := pager
	if pagerEnv != "" {
		pagerCmd = fmt.Sprintf("%s %s", pagerEnv, pager)
	}
	cmdStr := fmt.Sprintf("set -o pipefail; (%s) 2>&1 | %s", customCmd.Command, pagerCmd)
	// Always run via shell to support pipes, redirects, and shell features
	// #nosec G204 -- command comes from user's own config file
	c := m.commandRunner(m.ctx, "bash", "-c", cmdStr)
	c.Dir = wt.Path
	c.Env = envVars

	return m.execProcess(c, func(err error) tea.Msg {
		if err != nil {
			// Ignore exit status 141 (SIGPIPE) which happens when the pager is closed early
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 141 {
				return refreshCompleteMsg{}
			}
			return errMsg{err: err}
		}
		return refreshCompleteMsg{}
	})
}

func (m *Model) executeArbitraryCommand(cmdStr string) tea.Cmd {
	if m.state.data.selectedIndex < 0 || m.state.data.selectedIndex >= len(m.state.data.filteredWts) {
		return nil
	}

	wt := m.state.data.filteredWts[m.state.data.selectedIndex]

	// Build environment variables (same as custom commands)
	env := m.buildCommandEnv(wt.Branch, wt.Path)
	envVars := filterWorktreeEnvVars(os.Environ())
	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	// Get pager configuration
	pager := m.pagerCommand()
	pagerEnv := m.pagerEnv(pager)
	pagerCmd := pager
	if pagerEnv != "" {
		pagerCmd = fmt.Sprintf("%s %s", pagerEnv, pager)
	}

	// Build command string that pipes output through pager
	fullCmdStr := fmt.Sprintf("set -o pipefail; (%s) 2>&1 | %s", cmdStr, pagerCmd)

	// Create command with bash shell
	// #nosec G204 -- command comes from user input in TUI
	c := m.commandRunner(m.ctx, "bash", "-c", fullCmdStr)
	c.Dir = wt.Path
	c.Env = envVars

	return m.execProcess(c, func(err error) tea.Msg {
		if err != nil {
			// Ignore exit status 141 (SIGPIPE) which happens when the pager is closed early
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 141 {
				return refreshCompleteMsg{}
			}
			return errMsg{err: err}
		}
		return refreshCompleteMsg{}
	})
}

func (m *Model) customCommandKeys() []string {
	if len(m.config.CustomCommands) == 0 {
		return nil
	}

	keys := make([]string, 0, len(m.config.CustomCommands))
	for key, cmd := range m.config.CustomCommands {
		if cmd == nil {
			continue
		}
		if strings.TrimSpace(cmd.Command) == "" && cmd.Tmux == nil && cmd.Zellij == nil {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (m *Model) customCommandLabel(cmd *config.CustomCommand, key string) string {
	label := ""
	if cmd != nil {
		label = strings.TrimSpace(cmd.Description)
		if label == "" {
			label = strings.TrimSpace(cmd.Command)
			if label == "" {
				switch {
				case cmd.Zellij != nil:
					label = zellijSessionLabel
				case cmd.Tmux != nil:
					label = tmuxSessionLabel
				case cmd.NewTab:
					label = terminalTabLabel
				}
			}
		}
	}
	if label == "" {
		label = customCommandPlaceholder
	}
	return fmt.Sprintf("%s (%s)", label, key)
}
