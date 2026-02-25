package app

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/models"
)

// pushToUpstream pushes the current branch to its upstream.
func (m *Model) pushToUpstream() tea.Cmd {
	wt := m.selectedWorktree()
	if wt == nil {
		m.showInfo(errNoWorktreeSelected, nil)
		return nil
	}
	if hasLocalChanges(wt) {
		m.showInfo("Cannot push while the worktree has local changes.\n\nPlease commit, stash, or discard them first.", nil)
		return nil
	}
	if strings.TrimSpace(wt.Branch) == "" {
		m.showInfo("Cannot push a detached worktree.", nil)
		return nil
	}
	if wt.HasUpstream {
		remote, branch, ok := m.validatedUpstream(wt, "push")
		if !ok {
			return nil
		}
		return m.beginPush(wt, []string{remote, fmt.Sprintf("HEAD:%s", branch)})
	}
	return m.showUpstreamInput(wt, func(remote, branch string) tea.Cmd {
		return m.beginPush(wt, []string{"-u", remote, fmt.Sprintf("HEAD:%s", branch)})
	})
}

// syncWithUpstream synchronises the current branch with its upstream (pull + push).
func (m *Model) syncWithUpstream() tea.Cmd {
	wt := m.selectedWorktree()
	if wt == nil {
		m.showInfo(errNoWorktreeSelected, nil)
		return nil
	}
	if hasLocalChanges(wt) {
		m.showInfo("Cannot synchronise while the worktree has local changes.\n\nPlease commit, stash, or discard them first.", nil)
		return nil
	}
	if strings.TrimSpace(wt.Branch) == "" {
		m.showInfo("Cannot synchronise a detached worktree.", nil)
		return nil
	}

	// Check if this worktree has a PR and if we're behind the base branch
	if wt.PR != nil && wt.PR.BaseBranch != "" {
		if m.isBehindBase(wt) {
			return m.showSyncChoice(wt)
		}
	}

	// Normal sync (pull + push)
	if wt.HasUpstream {
		remote, branch, ok := m.validatedUpstream(wt, "synchronise")
		if !ok {
			return nil
		}
		return m.beginSync(wt, []string{remote, branch}, []string{remote, fmt.Sprintf("HEAD:%s", branch)})
	}
	return m.showUpstreamInput(wt, func(remote, branch string) tea.Cmd {
		return m.beginSync(wt, []string{remote, branch}, []string{"-u", remote, fmt.Sprintf("HEAD:%s", branch)})
	})
}

// beginPush initiates a push operation.
func (m *Model) beginPush(wt *models.WorktreeInfo, args []string) tea.Cmd {
	m.loading = true
	m.loadingOperation = "push"
	m.statusContent = "Pushing to upstream..."
	m.setLoadingScreen("Pushing to upstream...")
	return m.runPush(wt, args)
}

// beginSync initiates a sync operation (pull + push).
func (m *Model) beginSync(wt *models.WorktreeInfo, pullArgs, pushArgs []string) tea.Cmd {
	m.loading = true
	m.loadingOperation = "sync"
	m.statusContent = "Synchronising with upstream..."
	m.setLoadingScreen("Synchronising with upstream...")
	return m.runSync(wt, pullArgs, pushArgs)
}

// isBehindBase checks if the current branch is behind its base branch.
func (m *Model) isBehindBase(wt *models.WorktreeInfo) bool {
	if wt.PR == nil || wt.PR.BaseBranch == "" {
		return false
	}
	// Check if current branch is behind the base branch
	// Use git merge-base to find common ancestor, then check if we're behind
	mergeBase := m.state.services.git.RunGit(m.ctx, []string{
		"git", "merge-base", "HEAD", wt.PR.BaseBranch,
	}, wt.Path, []int{0, 1}, true, false)

	if mergeBase == "" {
		return false
	}

	// Check if there are commits in base that aren't in HEAD
	behindCount := m.state.services.git.RunGit(m.ctx, []string{
		"git", "rev-list", "--count", fmt.Sprintf("HEAD..%s", wt.PR.BaseBranch),
	}, wt.Path, []int{0}, true, false)

	behind, _ := strconv.Atoi(strings.TrimSpace(behindCount))
	return behind > 0
}

// showSyncChoice shows a confirmation dialog for syncing with base branch.
func (m *Model) showSyncChoice(wt *models.WorktreeInfo) tea.Cmd {
	// Store the worktree for later use in confirm/cancel handlers
	savedWt := wt

	confirmScreen := appscreen.NewConfirmScreen(
		fmt.Sprintf("Branch behind %s\n\nUpdate from base branch?\n(This will merge/rebase latest %s into your branch.\nChoose 'No' for normal sync: pull + push)",
			wt.PR.BaseBranch, wt.PR.BaseBranch),
		m.theme,
	)
	confirmScreen.OnConfirm = func() tea.Cmd {
		// User chose YES: update from base
		return m.updateFromBase(savedWt)
	}
	confirmScreen.OnCancel = func() tea.Cmd {
		// User chose NO: do normal sync (pull + push)
		if savedWt.HasUpstream {
			remote, branch, ok := m.validatedUpstream(savedWt, "synchronise")
			if !ok {
				return nil
			}
			return m.beginSync(savedWt, []string{remote, branch}, []string{remote, fmt.Sprintf("HEAD:%s", branch)})
		}
		return m.showUpstreamInput(savedWt, func(remote, branch string) tea.Cmd {
			return m.beginSync(savedWt, []string{remote, branch}, []string{"-u", remote, fmt.Sprintf("HEAD:%s", branch)})
		})
	}
	m.state.ui.screenManager.Push(confirmScreen)
	return nil
}

// updateFromBase updates the branch from its base branch.
func (m *Model) updateFromBase(wt *models.WorktreeInfo) tea.Cmd {
	m.loading = true
	m.loadingOperation = "sync"
	m.statusContent = fmt.Sprintf("Updating from %s...", wt.PR.BaseBranch)
	m.setLoadingScreen(fmt.Sprintf("Updating from %s...", wt.PR.BaseBranch))

	// Use gh pr update-branch with --rebase if merge_method is rebase
	args := []string{"gh", "pr", "update-branch"}
	mergeMethod := strings.TrimSpace(m.config.MergeMethod)
	if mergeMethod == "" {
		mergeMethod = mergeMethodRebase
	}
	if mergeMethod == mergeMethodRebase {
		args = append(args, "--rebase")
	}

	// Clear cache so status pane refreshes
	m.deleteDetailsCache(wt.Path)

	cmd := m.commandRunner(m.ctx, args[0], args[1:]...)
	cmd.Dir = wt.Path

	return func() tea.Msg {
		output, err := cmd.CombinedOutput()
		return syncResultMsg{
			stage:  "update-branch",
			output: strings.TrimSpace(string(output)),
			err:    err,
		}
	}
}

// showUpstreamInput shows an input screen for setting upstream.
func (m *Model) showUpstreamInput(wt *models.WorktreeInfo, onSubmit func(remote, branch string) tea.Cmd) tea.Cmd {
	defaultUpstream := fmt.Sprintf("origin/%s", wt.Branch)
	prompt := fmt.Sprintf("Set upstream for '%s' (remote/branch)", wt.Branch)

	inputScr := appscreen.NewInputScreen(prompt, defaultUpstream, defaultUpstream, m.theme, m.config.IconsEnabled())

	inputScr.OnSubmit = func(value string, _ bool) tea.Cmd {
		remote, branch, ok := parseUpstreamRef(value)
		if !ok {
			inputScr.ErrorMsg = "Please provide upstream as remote/branch."
			return nil
		}
		if branch != wt.Branch {
			inputScr.ErrorMsg = fmt.Sprintf("Upstream branch must match %q.", wt.Branch)
			return nil
		}
		inputScr.ErrorMsg = ""
		return onSubmit(remote, branch)
	}

	inputScr.OnCancel = func() tea.Cmd {
		return nil
	}

	m.state.ui.screenManager.Push(inputScr)
	return textinput.Blink
}

// validatedUpstream validates and parses the upstream reference.
func (m *Model) validatedUpstream(wt *models.WorktreeInfo, action string) (string, string, bool) {
	upstream := strings.TrimSpace(wt.UpstreamBranch)
	if upstream == "" {
		m.showInfo(fmt.Sprintf("Cannot %s because no upstream is configured.", action), nil)
		return "", "", false
	}
	remote, branch, ok := parseUpstreamRef(upstream)
	if !ok {
		m.showInfo(fmt.Sprintf("Cannot %s because upstream %q is not in remote/branch format.", action, upstream), nil)
		return "", "", false
	}
	if branch != wt.Branch {
		m.showInfo(fmt.Sprintf("Cannot %s because upstream %q does not match current branch %q.", action, upstream, wt.Branch), nil)
		return "", "", false
	}
	return remote, branch, true
}

// parseUpstreamRef parses a remote/branch string into remote and branch components.
func parseUpstreamRef(input string) (string, string, bool) {
	value := strings.TrimSpace(input)
	if value == "" {
		return "", "", false
	}
	remote, branch, ok := strings.Cut(value, "/")
	if !ok {
		return "", "", false
	}
	remote = strings.TrimSpace(remote)
	branch = strings.TrimSpace(branch)
	if remote == "" || branch == "" {
		return "", "", false
	}
	return remote, branch, true
}

// hasLocalChanges checks if a worktree has local uncommitted changes.
func hasLocalChanges(wt *models.WorktreeInfo) bool {
	if wt == nil {
		return false
	}
	if wt.Dirty {
		return true
	}
	return wt.Untracked > 0 || wt.Modified > 0 || wt.Staged > 0
}

// runPush executes a git push command.
func (m *Model) runPush(wt *models.WorktreeInfo, args []string) tea.Cmd {
	env := m.buildCommandEnv(wt.Branch, wt.Path)
	envVars := os.Environ()
	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	// Clear cache so status pane refreshes with latest git status
	m.deleteDetailsCache(wt.Path)

	cmdArgs := append([]string{"push"}, args...)
	c := m.commandRunner(m.ctx, "git", cmdArgs...)
	c.Dir = wt.Path
	c.Env = envVars

	return func() tea.Msg {
		output, err := c.CombinedOutput()
		return pushResultMsg{
			output: strings.TrimSpace(string(output)),
			err:    err,
		}
	}
}

// runSync executes a git pull followed by push.
func (m *Model) runSync(wt *models.WorktreeInfo, pullArgs, pushArgs []string) tea.Cmd {
	env := m.buildCommandEnv(wt.Branch, wt.Path)
	envVars := os.Environ()
	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	// Clear cache so status pane refreshes with latest git status
	m.deleteDetailsCache(wt.Path)

	pullCmdArgs := append([]string{"pull"}, m.syncPullArgs(pullArgs)...)
	pullCmd := m.commandRunner(m.ctx, "git", pullCmdArgs...)
	pullCmd.Dir = wt.Path
	pullCmd.Env = envVars

	return func() tea.Msg {
		pullOutput, pullErr := pullCmd.CombinedOutput()
		pullText := strings.TrimSpace(string(pullOutput))
		if pullErr != nil {
			return syncResultMsg{
				stage:  "pull",
				output: pullText,
				err:    pullErr,
			}
		}

		pushCmdArgs := append([]string{"push"}, pushArgs...)
		pushCmd := m.commandRunner(m.ctx, "git", pushCmdArgs...)
		pushCmd.Dir = wt.Path
		pushCmd.Env = envVars

		pushOutput, pushErr := pushCmd.CombinedOutput()
		pushText := strings.TrimSpace(string(pushOutput))
		combined := strings.TrimSpace(strings.Join(filterNonEmpty([]string{pullText, pushText}), "\n"))

		if pushErr != nil {
			return syncResultMsg{
				stage:  "push",
				output: combined,
				err:    pushErr,
			}
		}
		return syncResultMsg{
			output: combined,
			err:    nil,
		}
	}
}

// syncPullArgs adds merge method flags to pull arguments.
func (m *Model) syncPullArgs(pullArgs []string) []string {
	mergeMethod := strings.TrimSpace(m.config.MergeMethod)
	if mergeMethod == "" {
		mergeMethod = mergeMethodRebase
	}
	if mergeMethod == mergeMethodRebase {
		return append(pullArgs, pullRebaseFlag)
	}
	return pullArgs
}
