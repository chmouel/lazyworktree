package handlers

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

// DiffRouter consolidates diff routing logic.
type DiffRouter struct {
	Config                *config.AppConfig
	UseGitPager           bool
	CommandRunner         func(context.Context, string, ...string) *exec.Cmd
	Context               context.Context
	ExecProcess           func(*exec.Cmd, tea.ExecCallback) tea.Cmd
	PagerCommand          func() string
	PagerEnv              func(string) string
	FilterWorktreeEnvVars func([]string) []string
	ShellQuote            func(string) string
	ErrorMsg              func(error) tea.Msg
	RefreshMsg            func() tea.Msg
}

// WorktreeDiffParams collects dependencies for a worktree diff.
type WorktreeDiffParams struct {
	Worktree        *models.WorktreeInfo
	StatusFiles     []models.StatusFile
	BuildCommandEnv func(branch, wtPath string) map[string]string
	ShowInfo        func(message string)
}

// FileDiffParams collects dependencies for a status file diff.
type FileDiffParams struct {
	Worktree        *models.WorktreeInfo
	File            models.StatusFile
	BuildCommandEnv func(branch, wtPath string) map[string]string
}

// CommitDiffParams collects dependencies for a commit diff.
type CommitDiffParams struct {
	CommitSHA       string
	Worktree        *models.WorktreeInfo
	BuildCommandEnv func(branch, wtPath string) map[string]string
}

// CommitFileDiffParams collects dependencies for a commit file diff.
type CommitFileDiffParams struct {
	CommitSHA    string
	Filename     string
	WorktreePath string
}

type diffMode int

const (
	diffModeNonInteractive diffMode = iota
	diffModeInteractive
	diffModeVSCode
	diffModeCommand
)

// ShowDiff routes a worktree diff to the configured viewer.
func (r *DiffRouter) ShowDiff(params WorktreeDiffParams) tea.Cmd {
	if params.Worktree == nil {
		return nil
	}
	if len(params.StatusFiles) == 0 {
		if params.ShowInfo != nil {
			params.ShowInfo("No diff to show.")
		}
		return nil
	}

	switch r.mode() {
	case diffModeVSCode:
		return r.showDiffVSCode(params)
	case diffModeCommand:
		return r.showDiffCommand(params)
	case diffModeInteractive:
		return r.showDiffInteractive(params)
	default:
		return r.showDiffNonInteractive(params)
	}
}

// ShowFileDiff shows the diff for a single file.
func (r *DiffRouter) ShowFileDiff(params FileDiffParams) tea.Cmd {
	if params.Worktree == nil {
		return nil
	}

	if r.mode() == diffModeCommand {
		return r.showFileDiffCommand(params)
	}

	// Build environment variables
	env := r.buildCommandEnv(params.BuildCommandEnv, params.Worktree.Branch, params.Worktree.Path)
	envVars := r.envVars(env, false)

	// Get pager configuration
	pager := r.pagerCommand()
	pagerEnv := r.pagerEnv(pager)
	pagerCmd := pager
	if pagerEnv != "" {
		pagerCmd = fmt.Sprintf("%s %s", pagerEnv, pager)
	}

	// Build script based on file type
	var script string
	// Shell-escape the filename for safe use in shell commands
	escapedFilename := fmt.Sprintf("'%s'", strings.ReplaceAll(params.File.Filename, "'", "'\\''"))

	if params.File.IsUntracked {
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
	if r.UseGitPager {
		gitPagerArgs := strings.Join(r.Config.GitPagerArgs, " ")
		cmdStr = fmt.Sprintf("set -o pipefail; (%s) | %s %s | %s", script, r.Config.GitPager, gitPagerArgs, pagerCmd)
	} else {
		cmdStr = fmt.Sprintf("set -o pipefail; (%s) | %s", script, pagerCmd)
	}

	// Create command
	// #nosec G204 -- command is constructed from config and controlled inputs
	c := r.CommandRunner(r.Context, "bash", "-c", cmdStr)
	c.Dir = params.Worktree.Path
	c.Env = envVars

	return r.ExecProcess(c, func(err error) tea.Msg {
		return r.handlePagerExit(err)
	})
}

// ShowCommitDiff routes a commit diff to the configured viewer.
func (r *DiffRouter) ShowCommitDiff(params CommitDiffParams) tea.Cmd {
	if params.Worktree == nil {
		return nil
	}
	switch r.mode() {
	case diffModeVSCode:
		return r.showCommitDiffVSCode(params)
	case diffModeCommand:
		return r.showCommitDiffCommand(params)
	case diffModeInteractive:
		return r.showCommitDiffInteractive(params)
	default:
		return r.showCommitDiffNonInteractive(params)
	}
}

// ShowCommitFileDiff routes a commit file diff to the configured viewer.
func (r *DiffRouter) ShowCommitFileDiff(params CommitFileDiffParams) tea.Cmd {
	switch r.mode() {
	case diffModeVSCode:
		return r.showCommitFileDiffVSCode(params)
	case diffModeCommand:
		return r.showCommitFileDiffCommand(params)
	case diffModeInteractive:
		return r.showCommitFileDiffInteractive(params)
	default:
		return r.showCommitFileDiffNonInteractive(params)
	}
}

func (r *DiffRouter) showDiffInteractive(params WorktreeDiffParams) tea.Cmd {
	// Build environment variables
	env := r.buildCommandEnv(params.BuildCommandEnv, params.Worktree.Branch, params.Worktree.Path)
	envVars := r.envVars(env, false)

	// For interactive mode, just pipe git diff directly to the interactive tool
	// NO piping to less - the interactive tool needs terminal control
	gitPagerArgs := ""
	if len(r.Config.GitPagerArgs) > 0 {
		gitPagerArgs = " " + strings.Join(r.Config.GitPagerArgs, " ")
	}
	cmdStr := fmt.Sprintf("git diff --patch --no-color | %s%s", r.Config.GitPager, gitPagerArgs)

	// #nosec G204 -- command constructed from config and controlled inputs
	c := r.CommandRunner(r.Context, "bash", "-c", cmdStr)
	c.Dir = params.Worktree.Path
	c.Env = envVars

	return r.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return r.errorMsg(err)
		}
		return r.refreshMsg()
	})
}

func (r *DiffRouter) showDiffCommand(params WorktreeDiffParams) tea.Cmd {
	env := r.buildCommandEnv(params.BuildCommandEnv, params.Worktree.Branch, params.Worktree.Path)
	envVars := r.envVars(env, false)

	gitPagerArgs := ""
	if len(r.Config.GitPagerArgs) > 0 {
		gitPagerArgs = " " + strings.Join(r.Config.GitPagerArgs, " ")
	}
	cmdStr := fmt.Sprintf("%s diff%s", r.Config.GitPager, gitPagerArgs)

	// #nosec G204 -- command constructed from config and controlled inputs
	c := r.CommandRunner(r.Context, "bash", "-c", cmdStr)
	c.Dir = params.Worktree.Path
	c.Env = envVars

	return r.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return r.errorMsg(err)
		}
		return r.refreshMsg()
	})
}

func (r *DiffRouter) showFileDiffCommand(params FileDiffParams) tea.Cmd {
	env := r.buildCommandEnv(params.BuildCommandEnv, params.Worktree.Branch, params.Worktree.Path)
	envVars := r.envVars(env, false)

	gitPagerArgs := ""
	if len(r.Config.GitPagerArgs) > 0 {
		gitPagerArgs = " " + strings.Join(r.Config.GitPagerArgs, " ")
	}
	escapedFilename := r.shellQuote(params.File.Filename)
	cmdStr := fmt.Sprintf("%s diff%s --file %s", r.Config.GitPager, gitPagerArgs, escapedFilename)

	// #nosec G204 -- command constructed from config and controlled inputs
	c := r.CommandRunner(r.Context, "bash", "-c", cmdStr)
	c.Dir = params.Worktree.Path
	c.Env = envVars

	return r.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return r.errorMsg(err)
		}
		return r.refreshMsg()
	})
}

func (r *DiffRouter) showCommitDiffCommand(params CommitDiffParams) tea.Cmd {
	env := r.buildCommandEnv(params.BuildCommandEnv, params.Worktree.Branch, params.Worktree.Path)
	envVars := r.envVars(env, false)

	gitPagerArgs := ""
	if len(r.Config.GitPagerArgs) > 0 {
		gitPagerArgs = " " + strings.Join(r.Config.GitPagerArgs, " ")
	}
	commitRef := fmt.Sprintf("%s^..%s", params.CommitSHA, params.CommitSHA)
	cmdStr := fmt.Sprintf("%s diff%s %s", r.Config.GitPager, gitPagerArgs, commitRef)

	// #nosec G204 -- command constructed from config and controlled inputs
	c := r.CommandRunner(r.Context, "bash", "-c", cmdStr)
	c.Dir = params.Worktree.Path
	c.Env = envVars

	return r.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return r.errorMsg(err)
		}
		return r.refreshMsg()
	})
}

func (r *DiffRouter) showCommitFileDiffCommand(params CommitFileDiffParams) tea.Cmd {
	envVars := r.envVars(nil, false)

	gitPagerArgs := ""
	if len(r.Config.GitPagerArgs) > 0 {
		gitPagerArgs = " " + strings.Join(r.Config.GitPagerArgs, " ")
	}
	commitRef := fmt.Sprintf("%s^..%s", params.CommitSHA, params.CommitSHA)
	escapedFilename := r.shellQuote(params.Filename)
	cmdStr := fmt.Sprintf("%s diff%s %s --file %s", r.Config.GitPager, gitPagerArgs, commitRef, escapedFilename)

	// #nosec G204 -- command constructed from config and controlled inputs
	c := r.CommandRunner(r.Context, "bash", "-c", cmdStr)
	c.Dir = params.WorktreePath
	c.Env = envVars

	return r.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return r.errorMsg(err)
		}
		return r.refreshMsg()
	})
}

func (r *DiffRouter) showDiffVSCode(params WorktreeDiffParams) tea.Cmd {
	// Build environment variables
	env := r.buildCommandEnv(params.BuildCommandEnv, params.Worktree.Branch, params.Worktree.Path)
	envVars := r.envVars(env, false)

	// Use git difftool with VS Code - git handles before/after file extraction
	cmdStr := "git difftool --no-prompt --extcmd='code --wait --diff'"

	// #nosec G204 -- command constructed from controlled input
	c := r.CommandRunner(r.Context, "bash", "-c", cmdStr)
	c.Dir = params.Worktree.Path
	c.Env = envVars

	return r.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return r.errorMsg(err)
		}
		return r.refreshMsg()
	})
}

func (r *DiffRouter) showDiffNonInteractive(params WorktreeDiffParams) tea.Cmd {
	// Build environment variables
	env := r.buildCommandEnv(params.BuildCommandEnv, params.Worktree.Branch, params.Worktree.Path)
	envVars := r.envVars(env, false)

	// Get pager configuration
	pager := r.pagerCommand()
	pagerEnv := r.pagerEnv(pager)
	pagerCmd := pager
	if pagerEnv != "" {
		pagerCmd = fmt.Sprintf("%s %s", pagerEnv, pager)
	}

	// Build a script that replicates BuildThreePartDiff behavior
	// This shows: 1) Staged changes, 2) Unstaged changes, 3) Untracked files (limited)
	maxUntracked := r.Config.MaxUntrackedDiffs
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
	if r.UseGitPager {
		gitPagerArgs := strings.Join(r.Config.GitPagerArgs, " ")
		cmdStr = fmt.Sprintf("set -o pipefail; (%s) | %s %s | %s", script, r.Config.GitPager, gitPagerArgs, pagerCmd)
	} else {
		cmdStr = fmt.Sprintf("set -o pipefail; (%s) | %s", script, pagerCmd)
	}

	// Create command
	// #nosec G204 -- command is constructed from config and controlled inputs
	c := r.CommandRunner(r.Context, "bash", "-c", cmdStr)
	c.Dir = params.Worktree.Path
	c.Env = envVars

	return r.ExecProcess(c, func(err error) tea.Msg {
		return r.handlePagerExit(err)
	})
}

func (r *DiffRouter) showCommitDiffNonInteractive(params CommitDiffParams) tea.Cmd {
	// Build environment variables
	env := r.buildCommandEnv(params.BuildCommandEnv, params.Worktree.Branch, params.Worktree.Path)
	envVars := r.envVars(env, false)

	// Get pager configuration
	pager := r.pagerCommand()
	pagerEnv := r.pagerEnv(pager)
	pagerCmd := pager
	if pagerEnv != "" {
		pagerCmd = fmt.Sprintf("%s %s", pagerEnv, pager)
	}

	// Build git show command with colorization
	// --color=always: ensure color codes are passed to delta/pager
	gitCmd := fmt.Sprintf("git show --color=always %s", params.CommitSHA)

	// Pipe through git_pager if configured, then through pager
	// Note: delta only processes the diff part, so our colorized commit message will pass through
	// Don't use pipefail here as awk might not always match (e.g., if commit format is different)
	var cmdStr string
	if r.UseGitPager {
		gitPagerArgs := strings.Join(r.Config.GitPagerArgs, " ")
		cmdStr = fmt.Sprintf("%s | %s %s | %s", gitCmd, r.Config.GitPager, gitPagerArgs, pagerCmd)
	} else {
		cmdStr = fmt.Sprintf("%s | %s", gitCmd, pagerCmd)
	}

	// Create command
	// #nosec G204 -- command is constructed from config and controlled inputs
	c := r.CommandRunner(r.Context, "bash", "-c", cmdStr)
	c.Dir = params.Worktree.Path
	c.Env = envVars

	return r.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return r.errorMsg(err)
		}
		return r.refreshMsg()
	})
}

func (r *DiffRouter) showCommitFileDiffNonInteractive(params CommitFileDiffParams) tea.Cmd {
	// Build environment variables for pager
	envVars := r.envVars(nil, false)

	// Get pager configuration
	pager := r.pagerCommand()
	pagerEnv := r.pagerEnv(pager)
	pagerCmd := pager
	if pagerEnv != "" {
		pagerCmd = fmt.Sprintf("%s %s", pagerEnv, pager)
	}

	// Build git show command for specific file with colorization
	gitCmd := fmt.Sprintf("git show --color=always %s -- %q", params.CommitSHA, params.Filename)

	// Pipe through git_pager if configured, then through pager
	var cmdStr string
	if r.UseGitPager {
		gitPagerArgs := strings.Join(r.Config.GitPagerArgs, " ")
		cmdStr = fmt.Sprintf("%s | %s %s | %s", gitCmd, r.Config.GitPager, gitPagerArgs, pagerCmd)
	} else {
		cmdStr = fmt.Sprintf("%s | %s", gitCmd, pagerCmd)
	}

	// Create command
	// #nosec G204 -- command is constructed from config and controlled inputs
	c := r.CommandRunner(r.Context, "bash", "-c", cmdStr)
	c.Dir = params.WorktreePath
	c.Env = envVars

	return r.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return r.errorMsg(err)
		}
		return r.refreshMsg()
	})
}

func (r *DiffRouter) showCommitDiffInteractive(params CommitDiffParams) tea.Cmd {
	// Build environment variables
	env := r.buildCommandEnv(params.BuildCommandEnv, params.Worktree.Branch, params.Worktree.Path)
	envVars := r.envVars(env, true)

	gitPagerArgs := ""
	if len(r.Config.GitPagerArgs) > 0 {
		gitPagerArgs = " " + strings.Join(r.Config.GitPagerArgs, " ")
	}
	gitCmd := fmt.Sprintf("git show --patch --no-color %s", params.CommitSHA)
	cmdStr := fmt.Sprintf("%s | %s%s", gitCmd, r.Config.GitPager, gitPagerArgs)

	c := r.CommandRunner(r.Context, "bash", "-c", cmdStr)
	c.Dir = params.Worktree.Path
	c.Env = envVars

	return r.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return r.errorMsg(err)
		}
		return r.refreshMsg()
	})
}

func (r *DiffRouter) showCommitFileDiffInteractive(params CommitFileDiffParams) tea.Cmd {
	// Build environment variables for pager
	envVars := r.envVars(nil, false)

	gitPagerArgs := ""
	if len(r.Config.GitPagerArgs) > 0 {
		gitPagerArgs = " " + strings.Join(r.Config.GitPagerArgs, " ")
	}
	gitCmd := fmt.Sprintf("git show --patch --no-color %s -- %q", params.CommitSHA, params.Filename)
	cmdStr := fmt.Sprintf("%s | %s%s", gitCmd, r.Config.GitPager, gitPagerArgs)

	c := r.CommandRunner(r.Context, "bash", "-c", cmdStr)
	c.Dir = params.WorktreePath
	c.Env = envVars

	return r.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return r.errorMsg(err)
		}
		return r.refreshMsg()
	})
}

func (r *DiffRouter) showCommitDiffVSCode(params CommitDiffParams) tea.Cmd {
	// Build environment variables
	env := r.buildCommandEnv(params.BuildCommandEnv, params.Worktree.Branch, params.Worktree.Path)
	envVars := r.envVars(env, true)

	// Use git difftool to compare parent commit with this commit
	cmdStr := fmt.Sprintf("git difftool %s^..%s --no-prompt --extcmd='code --wait --diff'", params.CommitSHA, params.CommitSHA)

	// #nosec G204 -- command constructed from controlled input
	c := r.CommandRunner(r.Context, "bash", "-c", cmdStr)
	c.Dir = params.Worktree.Path
	c.Env = envVars

	return r.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return r.errorMsg(err)
		}
		return r.refreshMsg()
	})
}

func (r *DiffRouter) showCommitFileDiffVSCode(params CommitFileDiffParams) tea.Cmd {
	envVars := r.envVars(nil, true)
	envVars = append(envVars, fmt.Sprintf("WORKTREE_PATH=%s", params.WorktreePath))

	// Use git difftool to compare the specific file between parent and this commit
	cmdStr := fmt.Sprintf("git difftool %s^..%s --no-prompt --extcmd='code --wait --diff' -- %s",
		params.CommitSHA, params.CommitSHA, r.shellQuote(params.Filename))

	// #nosec G204 -- command constructed from controlled input
	c := r.CommandRunner(r.Context, "bash", "-c", cmdStr)
	c.Dir = params.WorktreePath
	c.Env = envVars

	return r.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return r.errorMsg(err)
		}
		return r.refreshMsg()
	})
}

func (r *DiffRouter) mode() diffMode {
	if r.Config == nil {
		return diffModeNonInteractive
	}
	if strings.Contains(r.Config.GitPager, "code") {
		return diffModeVSCode
	}
	if r.Config.GitPagerCommandMode {
		return diffModeCommand
	}
	if r.Config.GitPagerInteractive {
		return diffModeInteractive
	}
	return diffModeNonInteractive
}

func (r *DiffRouter) envVars(env map[string]string, filter bool) []string {
	var envVars []string
	if filter && r.FilterWorktreeEnvVars != nil {
		envVars = r.FilterWorktreeEnvVars(os.Environ())
	} else {
		envVars = os.Environ()
	}
	for key, val := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", key, val))
	}
	return envVars
}

func (r *DiffRouter) buildCommandEnv(build func(branch, wtPath string) map[string]string, branch, path string) map[string]string {
	if build == nil {
		return map[string]string{}
	}
	return build(branch, path)
}

func (r *DiffRouter) pagerCommand() string {
	if r.PagerCommand == nil {
		return ""
	}
	return r.PagerCommand()
}

func (r *DiffRouter) pagerEnv(pager string) string {
	if r.PagerEnv == nil {
		return ""
	}
	return r.PagerEnv(pager)
}

func (r *DiffRouter) shellQuote(input string) string {
	if r.ShellQuote != nil {
		return r.ShellQuote(input)
	}
	if input == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(input, "'", "'\"'\"'") + "'"
}

func (r *DiffRouter) errorMsg(err error) tea.Msg {
	if r.ErrorMsg == nil {
		return err
	}
	return r.ErrorMsg(err)
}

func (r *DiffRouter) refreshMsg() tea.Msg {
	if r.RefreshMsg == nil {
		return nil
	}
	return r.RefreshMsg()
}

func (r *DiffRouter) handlePagerExit(err error) tea.Msg {
	if err == nil {
		return r.refreshMsg()
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 141 {
		return r.refreshMsg()
	}
	return r.errorMsg(err)
}
