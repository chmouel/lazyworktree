package app

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/chmouel/lazyworktree/internal/models"
)

func (m *Model) showDiff() tea.Cmd {
	// Route to appropriate diff viewer based on configuration
	if strings.Contains(m.config.GitPager, "code") {
		return m.showDiffVSCode()
	}
	if m.config.GitPagerInteractive {
		return m.showDiffInteractive()
	}
	return m.showDiffNonInteractive()
}

func (m *Model) showDiffInteractive() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	wt := m.filteredWts[m.selectedIndex]

	// Check if there are any changes to show
	if len(m.statusFilesAll) == 0 {
		m.showInfo("No diff to show.", nil)
		return nil
	}

	// Build environment variables
	env := m.buildCommandEnv(wt.Branch, wt.Path)
	envVars := os.Environ()
	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	// For interactive mode, just pipe git diff directly to the interactive tool
	// NO piping to less - the interactive tool needs terminal control
	gitPagerArgs := ""
	if len(m.config.GitPagerArgs) > 0 {
		gitPagerArgs = " " + strings.Join(m.config.GitPagerArgs, " ")
	}
	cmdStr := fmt.Sprintf("git diff --patch --no-color | %s%s", m.config.GitPager, gitPagerArgs)

	// #nosec G204 -- command constructed from config and controlled inputs
	c := m.commandRunner("bash", "-c", cmdStr)
	c.Dir = wt.Path
	c.Env = envVars

	return m.execProcess(c, func(err error) tea.Msg {
		if err != nil {
			return errMsg{err: err}
		}
		return refreshCompleteMsg{}
	})
}

func (m *Model) showDiffVSCode() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	wt := m.filteredWts[m.selectedIndex]

	// Check if there are any changes to show
	if len(m.statusFilesAll) == 0 {
		m.showInfo("No diff to show.", nil)
		return nil
	}

	// Build environment variables
	env := m.buildCommandEnv(wt.Branch, wt.Path)
	envVars := os.Environ()
	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	// Use git difftool with VS Code - git handles before/after file extraction
	cmdStr := "git difftool --no-prompt --extcmd='code --wait --diff'"

	// #nosec G204 -- command constructed from controlled input
	c := m.commandRunner("bash", "-c", cmdStr)
	c.Dir = wt.Path
	c.Env = envVars

	return m.execProcess(c, func(err error) tea.Msg {
		if err != nil {
			return errMsg{err: err}
		}
		return refreshCompleteMsg{}
	})
}

func (m *Model) showDiffNonInteractive() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	wt := m.filteredWts[m.selectedIndex]

	// Check if there are any changes to show
	if len(m.statusFilesAll) == 0 {
		m.showInfo("No diff to show.", nil)
		return nil
	}

	// Build environment variables
	env := m.buildCommandEnv(wt.Branch, wt.Path)
	envVars := os.Environ()
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

	// Build a script that replicates BuildThreePartDiff behavior
	// This shows: 1) Staged changes, 2) Unstaged changes, 3) Untracked files (limited)
	maxUntracked := m.config.MaxUntrackedDiffs
	script := fmt.Sprintf(`
	set -e
	# Part 1: Staged changes
	staged=$(git diff --cached --patch --no-color 2>/dev/null || true)
	if [ -n "$staged" ]; then
	  echo "=== Staged Changes ==="
	  echo "$staged"
	  echo
	fi

	# Part 2: Unstaged changes
	unstaged=$(git diff --patch --no-color 2>/dev/null || true)
	if [ -n "$unstaged" ]; then
	  echo "=== Unstaged Changes ==="
	  echo "$unstaged"
	  echo
	fi

	# Part 3: Untracked files (limited to %d)
	untracked=$(git status --porcelain 2>/dev/null | grep '^?? ' | cut -d' ' -f2- || true)
	if [ -n "$untracked" ]; then
	  count=0
	  max_count=%d
	  total=$(echo "$untracked" | wc -l)
	  while IFS= read -r file; do
	    [ $count -ge $max_count ] && break
	    echo "=== Untracked: $file ==="
	    git diff --no-index /dev/null "$file" 2>/dev/null || true
	    echo
	    count=$((count + 1))
	  done <<< "$untracked"

	  if [ $total -gt $max_count ]; then
	    echo "[...showing $count of $total untracked files]"
	  fi
	fi
	`, maxUntracked, maxUntracked)

	// Pipe through git_pager if configured, then through pager
	var cmdStr string
	if m.git.UseGitPager() {
		gitPagerArgs := strings.Join(m.config.GitPagerArgs, " ")
		cmdStr = fmt.Sprintf("set -o pipefail; (%s) | %s %s | %s", script, m.config.GitPager, gitPagerArgs, pagerCmd)
	} else {
		cmdStr = fmt.Sprintf("set -o pipefail; (%s) | %s", script, pagerCmd)
	}

	// Create command
	// #nosec G204 -- command is constructed from config and controlled inputs
	c := m.commandRunner("bash", "-c", cmdStr)
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

// showFileDiff shows the diff for a single file in a pager.
func (m *Model) showFileDiff(sf StatusFile) tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	wt := m.filteredWts[m.selectedIndex]

	// Build environment variables
	env := m.buildCommandEnv(wt.Branch, wt.Path)
	envVars := os.Environ()
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

	// Build script based on file type
	var script string
	// Shell-escape the filename for safe use in shell commands
	escapedFilename := fmt.Sprintf("'%s'", strings.ReplaceAll(sf.Filename, "'", "'\\''"))

	if sf.IsUntracked {
		// For untracked files, show diff against /dev/null
		script = fmt.Sprintf(`
set -e
echo "=== Untracked:" %s "==="
git diff --no-index /dev/null %s 2>/dev/null || true
`, escapedFilename, escapedFilename)
	} else {
		// For tracked files, show both staged and unstaged changes
		script = fmt.Sprintf(`
set -e
# Staged changes for this file
staged=$(git diff --cached --patch --no-color -- %s 2>/dev/null || true)
if [ -n "$staged" ]; then
  echo "=== Staged Changes:" %s "==="
  echo "$staged"
  echo
fi

# Unstaged changes for this file
unstaged=$(git diff --patch --no-color -- %s 2>/dev/null || true)
if [ -n "$unstaged" ]; then
  echo "=== Unstaged Changes:" %s "==="
  echo "$unstaged"
  echo
fi
`, escapedFilename, escapedFilename, escapedFilename, escapedFilename)
	}

	// Pipe through git_pager if configured, then through pager
	var cmdStr string
	if m.git.UseGitPager() {
		gitPagerArgs := strings.Join(m.config.GitPagerArgs, " ")
		cmdStr = fmt.Sprintf("set -o pipefail; (%s) | %s %s | %s", script, m.config.GitPager, gitPagerArgs, pagerCmd)
	} else {
		cmdStr = fmt.Sprintf("set -o pipefail; (%s) | %s", script, pagerCmd)
	}

	// Create command
	// #nosec G204 -- command is constructed from config and controlled inputs
	c := m.commandRunner("bash", "-c", cmdStr)
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

func (m *Model) showCommitDiff(commitSHA string, wt *models.WorktreeInfo) tea.Cmd {
	if strings.Contains(m.config.GitPager, "code") {
		return m.showCommitDiffVSCode(commitSHA, wt)
	}
	if m.config.GitPagerInteractive {
		return m.showCommitDiffInteractive(commitSHA, wt)
	}
	// Build environment variables
	env := m.buildCommandEnv(wt.Branch, wt.Path)
	envVars := os.Environ()
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

	// Build git show command with colorization
	// --color=always: ensure color codes are passed to delta/pager
	gitCmd := fmt.Sprintf("git show --color=always %s", commitSHA)

	// Pipe through git_pager if configured, then through pager
	// Note: delta only processes the diff part, so our colorized commit message will pass through
	// Don't use pipefail here as awk might not always match (e.g., if commit format is different)
	var cmdStr string
	if m.git.UseGitPager() {
		gitPagerArgs := strings.Join(m.config.GitPagerArgs, " ")
		cmdStr = fmt.Sprintf("%s | %s %s | %s", gitCmd, m.config.GitPager, gitPagerArgs, pagerCmd)
	} else {
		cmdStr = fmt.Sprintf("%s | %s", gitCmd, pagerCmd)
	}

	// Create command
	// #nosec G204 -- command is constructed from config and controlled inputs
	c := m.commandRunner("bash", "-c", cmdStr)
	c.Dir = wt.Path
	c.Env = envVars

	return m.execProcess(c, func(err error) tea.Msg {
		if err != nil {
			return errMsg{err: err}
		}
		return refreshCompleteMsg{}
	})
}

func (m *Model) showCommitFileDiff(commitSHA, filename, worktreePath string) tea.Cmd {
	if strings.Contains(m.config.GitPager, "code") {
		return m.showCommitFileDiffVSCode(commitSHA, filename, worktreePath)
	}
	if m.config.GitPagerInteractive {
		return m.showCommitFileDiffInteractive(commitSHA, filename, worktreePath)
	}
	// Build environment variables for pager
	envVars := os.Environ()

	// Get pager configuration
	pager := m.pagerCommand()
	pagerEnv := m.pagerEnv(pager)
	pagerCmd := pager
	if pagerEnv != "" {
		pagerCmd = fmt.Sprintf("%s %s", pagerEnv, pager)
	}

	// Build git show command for specific file with colorization
	gitCmd := fmt.Sprintf("git show --color=always %s -- %q", commitSHA, filename)

	// Pipe through git_pager if configured, then through pager
	var cmdStr string
	if m.git.UseGitPager() {
		gitPagerArgs := strings.Join(m.config.GitPagerArgs, " ")
		cmdStr = fmt.Sprintf("%s | %s %s | %s", gitCmd, m.config.GitPager, gitPagerArgs, pagerCmd)
	} else {
		cmdStr = fmt.Sprintf("%s | %s", gitCmd, pagerCmd)
	}

	// Create command
	// #nosec G204 -- command is constructed from config and controlled inputs
	c := m.commandRunner("bash", "-c", cmdStr)
	c.Dir = worktreePath
	c.Env = envVars

	return m.execProcess(c, func(err error) tea.Msg {
		if err != nil {
			return errMsg{err: err}
		}
		return refreshCompleteMsg{}
	})
}

func (m *Model) showCommitDiffInteractive(commitSHA string, wt *models.WorktreeInfo) tea.Cmd {
	// Build environment variables
	env := m.buildCommandEnv(wt.Branch, wt.Path)
	envVars := filterWorktreeEnvVars(os.Environ())
	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	gitPagerArgs := ""
	if len(m.config.GitPagerArgs) > 0 {
		gitPagerArgs = " " + strings.Join(m.config.GitPagerArgs, " ")
	}
	gitCmd := fmt.Sprintf("git show --patch --no-color %s", commitSHA)
	cmdStr := fmt.Sprintf("%s | %s%s", gitCmd, m.config.GitPager, gitPagerArgs)

	c := m.commandRunner("bash", "-c", cmdStr)
	c.Dir = wt.Path
	c.Env = envVars

	return m.execProcess(c, func(err error) tea.Msg {
		if err != nil {
			return errMsg{err: err}
		}
		return refreshCompleteMsg{}
	})
}

func (m *Model) showCommitFileDiffInteractive(commitSHA, filename, worktreePath string) tea.Cmd {
	// Build environment variables for pager
	envVars := os.Environ()

	gitPagerArgs := ""
	if len(m.config.GitPagerArgs) > 0 {
		gitPagerArgs = " " + strings.Join(m.config.GitPagerArgs, " ")
	}
	gitCmd := fmt.Sprintf("git show --patch --no-color %s -- %q", commitSHA, filename)
	cmdStr := fmt.Sprintf("%s | %s%s", gitCmd, m.config.GitPager, gitPagerArgs)

	c := m.commandRunner("bash", "-c", cmdStr)
	c.Dir = worktreePath
	c.Env = envVars

	return m.execProcess(c, func(err error) tea.Msg {
		if err != nil {
			return errMsg{err: err}
		}
		return refreshCompleteMsg{}
	})
}

func (m *Model) showCommitDiffVSCode(commitSHA string, wt *models.WorktreeInfo) tea.Cmd {
	// Build environment variables
	env := m.buildCommandEnv(wt.Branch, wt.Path)
	envVars := filterWorktreeEnvVars(os.Environ())
	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	// Use git difftool to compare parent commit with this commit
	cmdStr := fmt.Sprintf("git difftool %s^..%s --no-prompt --extcmd='code --wait --diff'", commitSHA, commitSHA)

	// #nosec G204 -- command constructed from controlled input
	c := m.commandRunner("bash", "-c", cmdStr)
	c.Dir = wt.Path
	c.Env = envVars

	return m.execProcess(c, func(err error) tea.Msg {
		if err != nil {
			return errMsg{err: err}
		}
		return refreshCompleteMsg{}
	})
}

func (m *Model) showCommitFileDiffVSCode(commitSHA, filename, worktreePath string) tea.Cmd {
	envVars := filterWorktreeEnvVars(os.Environ())
	envVars = append(envVars, fmt.Sprintf("WORKTREE_PATH=%s", worktreePath))

	// Use git difftool to compare the specific file between parent and this commit
	cmdStr := fmt.Sprintf("git difftool %s^..%s --no-prompt --extcmd='code --wait --diff' -- %s",
		commitSHA, commitSHA, shellQuote(filename))

	// #nosec G204 -- command constructed from controlled input
	c := m.commandRunner("bash", "-c", cmdStr)
	c.Dir = worktreePath
	c.Env = envVars

	return m.execProcess(c, func(err error) tea.Msg {
		if err != nil {
			return errMsg{err: err}
		}
		return refreshCompleteMsg{}
	})
}
