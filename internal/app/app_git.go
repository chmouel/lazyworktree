package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/chmouel/lazyworktree/internal/config"
	log "github.com/chmouel/lazyworktree/internal/log"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/security"
)

func (m *Model) fetchPRData() tea.Cmd {
	return func() tea.Msg {
		// First try the traditional approach (matches by headRefName)
		prMap, err := m.git.FetchPRMap(m.ctx)
		if err != nil {
			return prDataLoadedMsg{prMap: nil, err: err}
		}
		log.Printf("FetchPRMap returned %d PRs", len(prMap))
		for branch, pr := range prMap {
			log.Printf("  prMap[%q] = PR#%d", branch, pr.Number)
		}

		// Also fetch PRs per worktree for cases where local branch differs from remote
		// This handles fork PRs where local branch name doesn't match headRefName
		worktreePRs := make(map[string]*models.PRInfo)
		worktreeErrors := make(map[string]string)
		for _, wt := range m.worktrees {
			log.Printf("Checking worktree: Branch=%q Path=%q", wt.Branch, wt.Path)
			if pr, ok := prMap[wt.Branch]; ok {
				log.Printf("  Found in prMap: PR#%d", pr.Number)
			} else {
				log.Printf("  Not in prMap, will fetch per-worktree")
			}

			// Skip if already matched by headRefName
			if _, ok := prMap[wt.Branch]; ok {
				continue
			}
			// Try to fetch PR for this worktree directly
			pr, fetchErr := m.git.FetchPRForWorktreeWithError(m.ctx, wt.Path)
			if pr != nil {
				worktreePRs[wt.Path] = pr
				log.Printf("  FetchPRForWorktree returned PR#%d", pr.Number)
			}
			if fetchErr != nil {
				worktreeErrors[wt.Path] = fetchErr.Error()
				log.Printf("  FetchPRForWorktree error: %v", fetchErr)
			}
			if pr == nil && fetchErr == nil {
				log.Printf("  FetchPRForWorktree returned nil (no PR)")
			}
		}

		return prDataLoadedMsg{
			prMap:          prMap,
			worktreePRs:    worktreePRs,
			worktreeErrors: worktreeErrors,
			err:            nil,
		}
	}
}

func (m *Model) fetchRemotes() tea.Cmd {
	return func() tea.Msg {
		m.git.RunGit(m.ctx, []string{"git", "fetch", "--all", "--quiet"}, "", []int{0}, false, false)
		return fetchRemotesCompleteMsg{}
	}
}

func (m *Model) showDeleteFile() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	if len(m.statusTreeFlat) == 0 || m.statusTreeIndex < 0 || m.statusTreeIndex >= len(m.statusTreeFlat) {
		return nil
	}
	node := m.statusTreeFlat[m.statusTreeIndex]
	wt := m.filteredWts[m.selectedIndex]

	if node.IsDir() {
		files := node.CollectFiles()
		if len(files) == 0 {
			return nil
		}
		m.confirmScreen = NewConfirmScreen(fmt.Sprintf("Delete %d file(s) in directory?\n\nDirectory: %s", len(files), node.Path), m.theme)
		m.confirmAction = m.deleteFilesCmd(wt, files)
	} else {
		m.confirmScreen = NewConfirmScreen(fmt.Sprintf("Delete file?\n\nFile: %s", node.File.Filename), m.theme)
		m.confirmAction = m.deleteFilesCmd(wt, []*StatusFile{node.File})
	}
	m.currentScreen = screenConfirm
	return nil
}

func (m *Model) deleteFilesCmd(wt *models.WorktreeInfo, files []*StatusFile) func() tea.Cmd {
	return func() tea.Cmd {
		env := m.buildCommandEnv(wt.Branch, wt.Path)
		envVars := os.Environ()
		for k, v := range env {
			envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
		}

		for _, sf := range files {
			filePath := filepath.Join(wt.Path, sf.Filename)

			if sf.IsUntracked {
				if err := os.Remove(filePath); err != nil {
					return func() tea.Msg { return errMsg{err: err} }
				}
			} else {
				// Restore the file from git (discard all changes)
				cmdStr := fmt.Sprintf("git checkout HEAD -- %s", shellQuote(sf.Filename))
				// #nosec G204 -- command is constructed with quoted filename
				c := m.commandRunner("bash", "-c", cmdStr)
				c.Dir = wt.Path
				c.Env = envVars
				if err := c.Run(); err != nil {
					return func() tea.Msg { return errMsg{err: err} }
				}
			}
		}

		// Clear cache so status pane refreshes
		m.deleteDetailsCache(wt.Path)
		return func() tea.Msg { return refreshCompleteMsg{} }
	}
}

func (m *Model) commitAllChanges() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	wt := m.filteredWts[m.selectedIndex]

	env := m.buildCommandEnv(wt.Branch, wt.Path)
	envVars := os.Environ()
	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	// Clear cache so status pane refreshes with latest git status
	m.deleteDetailsCache(wt.Path)

	// #nosec G204 -- command is a fixed git command
	c := m.commandRunner("bash", "-c", "git add -A && git commit")
	c.Dir = wt.Path
	c.Env = envVars

	return m.execProcess(c, func(err error) tea.Msg {
		if err != nil {
			return errMsg{err: err}
		}
		return refreshCompleteMsg{}
	})
}

func (m *Model) commitStagedChanges() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	wt := m.filteredWts[m.selectedIndex]

	// Check if there are any staged changes
	hasStagedChanges := false
	for _, sf := range m.statusFilesAll {
		if len(sf.Status) >= 2 {
			x := sf.Status[0] // Staged status
			if x != '.' && x != ' ' {
				hasStagedChanges = true
				break
			}
		}
	}

	if !hasStagedChanges {
		m.showInfo("No staged changes to commit", nil)
		return nil
	}

	env := m.buildCommandEnv(wt.Branch, wt.Path)
	envVars := os.Environ()
	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	// Clear cache so status pane refreshes with latest git status
	m.deleteDetailsCache(wt.Path)

	// #nosec G204 -- command is a fixed git command
	c := m.commandRunner("bash", "-c", "git commit")
	c.Dir = wt.Path
	c.Env = envVars

	return m.execProcess(c, func(err error) tea.Msg {
		if err != nil {
			return errMsg{err: err}
		}
		return refreshCompleteMsg{}
	})
}

func (m *Model) stageCurrentFile(sf StatusFile) tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	wt := m.filteredWts[m.selectedIndex]

	// Status is XY format: X=staged, Y=unstaged
	// Examples: "M " = staged, " M" = unstaged, "MM" = both
	if len(sf.Status) < 2 {
		return nil
	}

	x := sf.Status[0] // Staged status
	y := sf.Status[1] // Unstaged status

	var cmdStr string
	hasUnstagedChanges := y != '.' && y != ' '
	hasStagedChanges := x != '.' && x != ' '
	hasNoUnstagedChanges := y == '.' || y == ' '

	switch {
	case hasUnstagedChanges:
		// If there are unstaged changes, stage them
		cmdStr = fmt.Sprintf("git add %s", shellQuote(sf.Filename))
	case hasStagedChanges && hasNoUnstagedChanges:
		// File is fully staged with no unstaged changes, so unstage it
		cmdStr = fmt.Sprintf("git restore --staged %s", shellQuote(sf.Filename))
	default:
		// File is clean or in an unexpected state
		return nil
	}

	env := m.buildCommandEnv(wt.Branch, wt.Path)
	envVars := os.Environ()
	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	// Clear cache so status pane refreshes with latest git status
	m.deleteDetailsCache(wt.Path)

	// Run git command in background without suspending the TUI to avoid flicker
	// #nosec G204 -- command is constructed with quoted filename
	c := m.commandRunner("bash", "-c", cmdStr)
	c.Dir = wt.Path
	c.Env = envVars

	return func() tea.Msg {
		if err := c.Run(); err != nil {
			return errMsg{err: err}
		}
		return refreshCompleteMsg{}
	}
}

func (m *Model) stageDirectory(node *StatusTreeNode) tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	wt := m.filteredWts[m.selectedIndex]

	files := node.CollectFiles()
	if len(files) == 0 {
		return nil
	}

	// Check if all files are fully staged (no unstaged changes)
	allStaged := true
	for _, f := range files {
		if len(f.Status) < 2 {
			continue
		}
		y := f.Status[1] // Unstaged status
		if y != '.' && y != ' ' {
			allStaged = false
			break
		}
	}

	// Build file list for git command
	fileArgs := make([]string, 0, len(files))
	for _, f := range files {
		fileArgs = append(fileArgs, shellQuote(f.Filename))
	}
	fileList := strings.Join(fileArgs, " ")

	var cmdStr string
	if allStaged {
		// All files are staged, unstage them all
		cmdStr = fmt.Sprintf("git restore --staged %s", fileList)
	} else {
		// Mixed or all unstaged, stage them all
		cmdStr = fmt.Sprintf("git add %s", fileList)
	}

	env := m.buildCommandEnv(wt.Branch, wt.Path)
	envVars := os.Environ()
	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	// Clear cache so status pane refreshes with latest git status
	m.deleteDetailsCache(wt.Path)

	// Run git command in background without suspending the TUI to avoid flicker
	// #nosec G204 -- command is constructed with quoted filenames
	c := m.commandRunner("bash", "-c", cmdStr)
	c.Dir = wt.Path
	c.Env = envVars

	return func() tea.Msg {
		if err := c.Run(); err != nil {
			return errMsg{err: err}
		}
		return refreshCompleteMsg{}
	}
}

func (m *Model) executeCherryPick(commitSHA string, targetWorktree *models.WorktreeInfo) tea.Cmd {
	return func() tea.Msg {
		_, err := m.git.CherryPickCommit(m.ctx, commitSHA, targetWorktree.Path)
		return cherryPickResultMsg{
			commitSHA:      commitSHA,
			targetWorktree: targetWorktree,
			err:            err,
		}
	}
}

func (m *Model) collectInitCommands() []string {
	cmds := []string{}
	cmds = append(cmds, m.config.InitCommands...)
	if m.repoConfig != nil {
		cmds = append(cmds, m.repoConfig.InitCommands...)
	}
	return cmds
}

func (m *Model) collectTerminateCommands() []string {
	cmds := []string{}
	cmds = append(cmds, m.config.TerminateCommands...)
	if m.repoConfig != nil {
		cmds = append(cmds, m.repoConfig.TerminateCommands...)
	}
	return cmds
}

func (m *Model) runCommandsWithTrust(cmds []string, cwd string, env map[string]string, after func() tea.Msg) tea.Cmd {
	if len(cmds) == 0 {
		if after == nil {
			return nil
		}
		return after
	}

	trustMode := strings.ToLower(strings.TrimSpace(m.config.TrustMode))
	// If trust mode set to never, skip repo commands
	if trustMode == "never" {
		if after == nil {
			return nil
		}
		return after
	}

	// Determine trust status if repo config exists
	trustPath := m.repoConfigPath
	status := security.TrustStatusTrusted
	if m.repoConfig != nil && trustPath != "" {
		status = m.trustManager.CheckTrust(trustPath)
	}

	if trustMode == "always" || status == security.TrustStatusTrusted {
		return m.runCommands(cmds, cwd, env, after)
	}

	// TOFU: prompt user
	if trustPath != "" {
		m.pendingCommands = cmds
		m.pendingCmdEnv = env
		m.pendingCmdCwd = cwd
		m.pendingAfter = after
		m.pendingTrust = trustPath
		m.trustScreen = NewTrustScreen(trustPath, cmds, m.theme)
		m.currentScreen = screenTrust
	}
	return nil
}

func (m *Model) runCommands(cmds []string, cwd string, env map[string]string, after func() tea.Msg) tea.Cmd {
	return func() tea.Msg {
		if err := m.git.ExecuteCommands(m.ctx, cmds, cwd, env); err != nil {
			// Still refresh UI even if commands failed, so user sees current state
			if after != nil {
				return after()
			}
			return errMsg{err: err}
		}
		if after != nil {
			return after()
		}
		return nil
	}
}

func (m *Model) clearPendingTrust() {
	m.pendingCommands = nil
	m.pendingCmdEnv = nil
	m.pendingCmdCwd = ""
	m.pendingAfter = nil
	m.pendingTrust = ""
	m.trustScreen = nil
}

func (m *Model) ensureRepoConfig() {
	if m.repoConfig != nil || m.repoConfigPath != "" {
		return
	}
	mainPath := m.getMainWorktreePath()
	if mainPath == "" {
		mainPath = m.git.GetMainWorktreePath(m.ctx)
	}
	repoCfg, cfgPath, err := config.LoadRepoConfig(mainPath)
	if err != nil {
		m.showInfo(fmt.Sprintf("Failed to load .wt: %v", err), nil)
		return
	}
	m.repoConfigPath = cfgPath
	m.repoConfig = repoCfg
}
