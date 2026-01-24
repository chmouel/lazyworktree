package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/utils"
)

// showCreateWorktree shows the base selection screen for creating a new worktree.
func (m *Model) showCreateWorktree() tea.Cmd {
	defaultBase := m.git.GetMainBranch(m.ctx)
	return m.showBaseSelection(defaultBase)
}

// showCreateFromCurrent initiates the "create from current" workflow.
func (m *Model) showCreateFromCurrent() tea.Cmd {
	return func() tea.Msg {
		currentWt := m.determineCurrentWorktree()
		if currentWt == nil {
			return errMsg{err: fmt.Errorf("could not determine current worktree")}
		}

		// Check for changes
		statusRaw := m.git.RunGit(m.ctx, []string{"git", "status", "--porcelain"}, currentWt.Path, []int{0}, true, false)
		hasChanges := strings.TrimSpace(statusRaw) != ""

		// Get current branch
		currentBranch := m.git.RunGit(m.ctx, []string{"git", "rev-parse", "--abbrev-ref", "HEAD"}, currentWt.Path, []int{0}, true, false)
		if currentBranch == "" {
			return errMsg{err: fmt.Errorf("failed to get current branch")}
		}
		currentBranch = strings.TrimSpace(currentBranch)

		// Always generate random name as default
		defaultName := fmt.Sprintf("%s-%s", currentBranch, utils.RandomBranchName())

		// Get diff if changes exist (for later AI generation)
		var diff string
		if hasChanges && m.config.BranchNameScript != "" {
			diff = m.git.RunGit(m.ctx, []string{"git", "diff", "HEAD"}, currentWt.Path, []int{0}, false, true)
		}

		return createFromCurrentReadyMsg{
			currentWorktree:   currentWt,
			currentBranch:     currentBranch,
			diff:              diff,
			hasChanges:        hasChanges,
			defaultBranchName: m.suggestBranchName(defaultName), // Use random name
		}
	}
}

// getCurrentBranchForMenu returns the current branch name for menu display.
// Returns empty string on error (caller should fallback to static label).
func (m *Model) getCurrentBranchForMenu() string {
	currentWt := m.determineCurrentWorktree()
	if currentWt == nil {
		return ""
	}

	branch := m.git.RunGit(
		m.ctx,
		[]string{"git", "rev-parse", "--abbrev-ref", "HEAD"},
		currentWt.Path,
		[]int{0},
		true,
		false,
	)
	return strings.TrimSpace(branch)
}

// showCreateFromPR initiates fetching open PRs for worktree creation.
func (m *Model) showCreateFromPR() tea.Cmd {
	// Fetch all open PRs
	return func() tea.Msg {
		prs, err := m.git.FetchAllOpenPRs(m.ctx)
		return openPRsLoadedMsg{
			prs: prs,
			err: err,
		}
	}
}

// showCreateFromIssue initiates fetching open issues for worktree creation.
func (m *Model) showCreateFromIssue() tea.Cmd {
	// Fetch all open issues
	return func() tea.Msg {
		issues, err := m.git.FetchAllOpenIssues(m.ctx)
		return openIssuesLoadedMsg{
			issues: issues,
			err:    err,
		}
	}
}

// showCreateWorktreeFromChanges initiates creating a worktree from changes in the selected worktree.
func (m *Model) showCreateWorktreeFromChanges() tea.Cmd {
	// Check if a worktree is selected
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		m.showInfo(errNoWorktreeSelected, nil)
		return nil
	}

	wt := m.filteredWts[m.selectedIndex]

	// Check for changes in the selected worktree asynchronously
	return func() tea.Msg {
		statusRaw := m.git.RunGit(m.ctx, []string{"git", "status", "--porcelain"}, wt.Path, []int{0}, true, false)
		if strings.TrimSpace(statusRaw) == "" {
			return errMsg{err: fmt.Errorf("no changes to move")}
		}

		// Get current branch name
		currentBranch := m.git.RunGit(m.ctx, []string{"git", "rev-parse", "--abbrev-ref", "HEAD"}, wt.Path, []int{0}, true, false)
		if currentBranch == "" {
			return errMsg{err: fmt.Errorf("failed to get current branch")}
		}

		// Get diff if branch_name_script is configured
		var diff string
		if m.config.BranchNameScript != "" {
			diff = m.git.RunGit(m.ctx, []string{"git", "diff", "HEAD"}, wt.Path, []int{0}, false, true)
		}

		return createFromChangesReadyMsg{
			worktree:      wt,
			currentBranch: currentBranch,
			diff:          diff,
		}
	}
}

// showCreateFromChangesInput shows the input screen for creating a worktree from changes.
func (m *Model) showCreateFromChangesInput(wt *models.WorktreeInfo, currentBranch, defaultName string) tea.Cmd {
	// Show input screen for worktree name
	m.inputScreen = NewInputScreen("Create worktree from changes: branch name", "feature/my-branch", defaultName, m.theme, m.config.ShowIcons)
	m.inputSubmit = func(value string, checked bool) (tea.Cmd, bool) {
		newBranch := strings.TrimSpace(value)
		newBranch = sanitizeBranchNameFromTitle(newBranch, "")
		if newBranch == "" {
			m.inputScreen.errorMsg = errBranchEmpty
			return nil, false
		}

		// Prevent duplicates - check if branch already exists in worktrees
		for _, existingWt := range m.worktrees {
			if existingWt.Branch == newBranch {
				m.inputScreen.errorMsg = fmt.Sprintf("Branch %q already exists.", newBranch)
				return nil, false
			}
		}

		// Check if branch exists in git
		branchRef := m.git.RunGit(m.ctx, []string{"git", "show-ref", fmt.Sprintf("refs/heads/%s", newBranch)}, "", []int{0, 1}, true, true)
		if branchRef != "" {
			// Branch exists
			m.inputScreen.errorMsg = fmt.Sprintf("Branch %q already exists.", newBranch)
			return nil, false
		}

		// Check if worktree path already exists
		targetPath := filepath.Join(m.getWorktreeDir(), newBranch)
		if _, err := os.Stat(targetPath); err == nil {
			m.inputScreen.errorMsg = fmt.Sprintf("Path already exists: %s", targetPath)
			return nil, false
		}

		m.inputScreen.errorMsg = ""
		if err := os.MkdirAll(m.getWorktreeDir(), 0o750); err != nil {
			return func() tea.Msg { return errMsg{err: fmt.Errorf("failed to create worktree directory: %w", err)} }, true
		}

		// Stash changes with descriptive message
		stashMessage := fmt.Sprintf("git-wt-create move-current: %s", newBranch)
		if !m.git.RunCommandChecked(
			m.ctx,
			[]string{"git", "stash", "push", "-u", "-m", stashMessage},
			wt.Path,
			"Failed to create stash for moving changes",
		) {
			return func() tea.Msg { return errMsg{err: fmt.Errorf("failed to create stash for moving changes")} }, true
		}

		// Get the stash ref
		stashRef := m.git.RunGit(m.ctx, []string{"git", "stash", "list", "-1", "--format=%gd"}, "", []int{0}, true, false)
		if stashRef == "" || !strings.HasPrefix(stashRef, "stash@{") {
			// Try to restore stash if we can't get the ref
			m.git.RunCommandChecked(m.ctx, []string{"git", "stash", "pop"}, wt.Path, "Failed to restore stash")
			return func() tea.Msg { return errMsg{err: fmt.Errorf("failed to get stash reference")} }, true
		}

		// Create the new worktree from current branch
		if !m.git.RunCommandChecked(
			m.ctx,
			[]string{"git", "worktree", "add", "-b", newBranch, targetPath, currentBranch},
			"",
			fmt.Sprintf("Failed to create worktree %s", newBranch),
		) {
			// If worktree creation fails, try to restore the stash
			m.git.RunCommandChecked(m.ctx, []string{"git", "stash", "pop"}, wt.Path, "Failed to restore stash")
			return func() tea.Msg { return errMsg{err: fmt.Errorf("failed to create worktree %s", newBranch)} }, true
		}

		// Apply stash to the new worktree
		if !m.git.RunCommandChecked(
			m.ctx,
			[]string{"git", "stash", "apply", "--index", stashRef},
			targetPath,
			"Failed to apply stash to new worktree",
		) {
			// If stash apply fails, clean up the worktree and try to restore stash to original location
			m.git.RunCommandChecked(m.ctx, []string{"git", "worktree", "remove", "--force", targetPath}, "", "Failed to remove worktree")
			m.git.RunCommandChecked(m.ctx, []string{"git", "stash", "pop"}, wt.Path, "Failed to restore stash")
			return func() tea.Msg { return errMsg{err: fmt.Errorf("failed to apply stash to new worktree")} }, true
		}

		// Drop the stash from the original location
		m.git.RunCommandChecked(m.ctx, []string{"git", "stash", "drop", stashRef}, wt.Path, "Failed to drop stash")

		// Run init commands and refresh
		env := m.buildCommandEnv(newBranch, targetPath)
		initCmds := m.collectInitCommands()
		after := func() tea.Msg {
			worktrees, err := m.git.GetWorktrees(m.ctx)
			return worktreesLoadedMsg{
				worktrees: worktrees,
				err:       err,
			}
		}
		return m.runCommandsWithTrust(initCmds, targetPath, env, after), true
	}
	m.currentScreen = screenInput
	return textinput.Blink
}

// generateAIBranchName generates a branch name using the configured AI script.
func (m *Model) generateAIBranchName() tea.Cmd {
	return func() tea.Msg {
		name, err := runBranchNameScript(
			m.ctx,
			m.config.BranchNameScript,
			m.createFromCurrentDiff,
			"diff",
			"",
			"",
			"",
		)
		return aiBranchNameGeneratedMsg{name: name, err: err}
	}
}

// handleCheckboxToggle handles checkbox toggling in the create from current flow.
func (m *Model) handleCheckboxToggle() tea.Cmd {
	if m.createFromCurrentDiff == "" {
		// Not in "create from current" flow, ignore
		return nil
	}

	if m.inputScreen.checkboxChecked {
		// Checkbox was checked: switch to AI name
		if m.createFromCurrentAIName != "" {
			// Use cached AI name
			m.inputScreen.input.SetValue(m.createFromCurrentAIName)
			m.inputScreen.input.CursorEnd()
			return nil
		}

		// Generate AI name if not cached
		if m.config.BranchNameScript != "" && m.createFromCurrentDiff != "" {
			return m.generateAIBranchName()
		}

		// No script configured, keep random name
		return nil
	}

	// Checkbox was unchecked: restore random name
	m.inputScreen.input.SetValue(m.createFromCurrentRandomName)
	m.inputScreen.input.CursorEnd()
	return nil
}

// handleCreateFromCurrentReady handles the createFromCurrentReadyMsg.
func (m *Model) handleCreateFromCurrentReady(msg createFromCurrentReadyMsg) tea.Cmd {
	if msg.currentWorktree == nil {
		m.showInfo("Could not determine current worktree", nil)
		return nil
	}

	// Store context for checkbox toggling
	m.createFromCurrentDiff = msg.diff
	m.createFromCurrentRandomName = msg.defaultBranchName
	m.createFromCurrentBranch = msg.currentBranch
	m.createFromCurrentAIName = "" // Reset cached AI name

	// Show input screen with random name
	m.inputScreen = NewInputScreen("Create from current: branch name", "feature/my-branch", msg.defaultBranchName, m.theme, m.config.ShowIcons)
	if msg.hasChanges {
		m.inputScreen.SetCheckbox("Include current file changes", false)
	}

	// Capture context for closure
	wt := msg.currentWorktree
	currentBranch := msg.currentBranch
	hasChanges := msg.hasChanges

	m.inputSubmit = func(value string, checked bool) (tea.Cmd, bool) {
		newBranch := strings.TrimSpace(value)
		newBranch = sanitizeBranchNameFromTitle(newBranch, "")
		if newBranch == "" {
			m.inputScreen.errorMsg = errBranchEmpty
			return nil, false
		}

		// Validate branch doesn't exist
		if m.branchExistsInWorktrees(newBranch) {
			m.inputScreen.errorMsg = fmt.Sprintf("Branch %q already exists.", newBranch)
			return nil, false
		}

		// Check if branch exists in git
		branchRef := m.git.RunGit(m.ctx, []string{"git", "show-ref", fmt.Sprintf("refs/heads/%s", newBranch)}, "", []int{0, 1}, true, true)
		if branchRef != "" {
			m.inputScreen.errorMsg = fmt.Sprintf("Branch %q already exists.", newBranch)
			return nil, false
		}

		targetPath := filepath.Join(m.getWorktreeDir(), newBranch)
		if m.worktreePathExists(targetPath) {
			m.inputScreen.errorMsg = fmt.Sprintf("Path already exists: %s", targetPath)
			return nil, false
		}

		// Clear cached state
		m.createFromCurrentDiff = ""
		m.createFromCurrentRandomName = ""
		m.createFromCurrentAIName = ""
		m.createFromCurrentBranch = ""

		// Set pending selection so the new worktree is selected after creation
		m.pendingSelectWorktreePath = targetPath

		includeChanges := m.inputScreen.checkboxChecked
		// Only attempt to move changes if checkbox is checked AND there are actual changes
		// This prevents accidentally applying an unrelated existing stash when workspace is clean
		if includeChanges && hasChanges {
			return m.executeCreateWithChanges(wt, currentBranch, newBranch, targetPath), true
		}
		return m.executeCreateWithoutChanges(currentBranch, newBranch, targetPath), true
	}

	m.currentScreen = screenInput
	return textinput.Blink
}

// executeCreateWithChanges creates a worktree and moves changes from the current worktree.
func (m *Model) executeCreateWithChanges(wt *models.WorktreeInfo, currentBranch, newBranch, targetPath string) tea.Cmd {
	return func() tea.Msg {
		if err := m.ensureWorktreeDir(m.getWorktreeDir()); err != nil {
			return errMsg{err: err}
		}

		// Stash changes with descriptive message
		prevStashHash := m.git.RunGit(m.ctx, []string{"git", "stash", "list", "-1", "--format=%H"}, "", []int{0}, true, false)
		stashMessage := fmt.Sprintf("git-wt-create move-current: %s", newBranch)
		if !m.git.RunCommandChecked(
			m.ctx,
			[]string{"git", "stash", "push", "-u", "-m", stashMessage},
			wt.Path,
			"Failed to create stash for moving changes",
		) {
			return errMsg{err: fmt.Errorf("failed to create stash for moving changes")}
		}

		newStashHash := m.git.RunGit(m.ctx, []string{"git", "stash", "list", "-1", "--format=%H"}, "", []int{0}, true, false)
		if newStashHash == "" || newStashHash == prevStashHash {
			return errMsg{err: fmt.Errorf("failed to create stash for moving changes: no new entry created")}
		}

		// Get the stash ref
		stashRef := m.git.RunGit(m.ctx, []string{"git", "stash", "list", "-1", "--format=%gd"}, "", []int{0}, true, false)
		if stashRef == "" || !strings.HasPrefix(stashRef, "stash@{") {
			// Try to restore stash if we can't get the ref
			m.git.RunCommandChecked(m.ctx, []string{"git", "stash", "pop"}, wt.Path, "Failed to restore stash")
			return errMsg{err: fmt.Errorf("failed to get stash reference")}
		}

		// Create the new worktree from current branch
		if !m.git.RunCommandChecked(
			m.ctx,
			[]string{"git", "worktree", "add", "-b", newBranch, targetPath, currentBranch},
			"",
			fmt.Sprintf("Failed to create worktree %s", newBranch),
		) {
			// If worktree creation fails, try to restore the stash
			m.git.RunCommandChecked(m.ctx, []string{"git", "stash", "pop"}, wt.Path, "Failed to restore stash")
			return errMsg{err: fmt.Errorf("failed to create worktree %s", newBranch)}
		}

		// Apply stash to the new worktree
		if !m.git.RunCommandChecked(
			m.ctx,
			[]string{"git", "stash", "apply", "--index", stashRef},
			targetPath,
			"Failed to apply stash to new worktree",
		) {
			// If stash apply fails, clean up the worktree and try to restore stash to original location
			m.git.RunCommandChecked(m.ctx, []string{"git", "worktree", "remove", "--force", targetPath}, "", "Failed to remove worktree")
			m.git.RunCommandChecked(m.ctx, []string{"git", "stash", "pop"}, wt.Path, "Failed to restore stash")
			return errMsg{err: fmt.Errorf("failed to apply stash to new worktree")}
		}

		// Drop the stash from the original location
		m.git.RunCommandChecked(m.ctx, []string{"git", "stash", "drop", stashRef}, wt.Path, "Failed to drop stash")

		// Run init commands and refresh
		env := m.buildCommandEnv(newBranch, targetPath)
		initCmds := m.collectInitCommands()
		after := func() tea.Msg {
			worktrees, err := m.git.GetWorktrees(m.ctx)
			return worktreesLoadedMsg{
				worktrees: worktrees,
				err:       err,
			}
		}
		return m.runCommandsWithTrust(initCmds, targetPath, env, after)()
	}
}

// executeCreateWithoutChanges creates a worktree without moving changes.
func (m *Model) executeCreateWithoutChanges(currentBranch, newBranch, targetPath string) tea.Cmd {
	return func() tea.Msg {
		if err := m.ensureWorktreeDir(m.getWorktreeDir()); err != nil {
			return errMsg{err: err}
		}

		args := []string{"git", "worktree", "add", "-b", newBranch, targetPath, currentBranch}
		if !m.git.RunCommandChecked(m.ctx, args, "", fmt.Sprintf("Failed to create worktree %s", newBranch)) {
			return errMsg{err: fmt.Errorf("failed to create worktree %s", newBranch)}
		}

		env := m.buildCommandEnv(newBranch, targetPath)
		initCmds := m.collectInitCommands()
		after := func() tea.Msg {
			worktrees, err := m.git.GetWorktrees(m.ctx)
			return worktreesLoadedMsg{
				worktrees: worktrees,
				err:       err,
			}
		}
		return m.runCommandsWithTrust(initCmds, targetPath, env, after)()
	}
}

// showDeleteWorktree shows a confirmation dialog for deleting a worktree.
func (m *Model) showDeleteWorktree() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	wt := m.filteredWts[m.selectedIndex]
	if wt.IsMain {
		return nil
	}
	m.confirmScreen = NewConfirmScreen(fmt.Sprintf("Delete worktree?\n\nPath: %s\nBranch: %s", wt.Path, wt.Branch), m.theme)
	m.confirmAction = m.deleteWorktreeOnlyCmd(wt)
	m.currentScreen = screenConfirm
	return nil
}

// showRenameWorktree shows an input screen for renaming a worktree.
func (m *Model) showRenameWorktree() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}

	wt := m.filteredWts[m.selectedIndex]
	if wt.IsMain {
		m.showInfo("Cannot rename the main worktree.", nil)
		return nil
	}

	prompt := fmt.Sprintf("Enter new name for '%s'", wt.Branch)
	m.inputScreen = NewInputScreen(prompt, "New branch name", wt.Branch, m.theme, m.config.ShowIcons)
	m.inputSubmit = func(value string, checked bool) (tea.Cmd, bool) {
		newBranch := strings.TrimSpace(value)
		newBranch = sanitizeBranchNameFromTitle(newBranch, "")
		if newBranch == "" {
			m.inputScreen.errorMsg = "Name cannot be empty."
			return nil, false
		}
		if newBranch == wt.Branch {
			m.inputScreen.errorMsg = "Name must be different from the current branch."
			return nil, false
		}

		parentDir := filepath.Dir(wt.Path)
		newPath := filepath.Join(parentDir, newBranch)
		if _, err := os.Stat(newPath); err == nil {
			m.inputScreen.errorMsg = fmt.Sprintf("Destination already exists: %s", newPath)
			return nil, false
		}

		m.inputScreen.errorMsg = ""
		oldPath := wt.Path
		oldBranch := wt.Branch

		return func() tea.Msg {
			ok := m.git.RenameWorktree(m.ctx, oldPath, newPath, oldBranch, newBranch)
			if !ok {
				return errMsg{err: fmt.Errorf("failed to rename %s to %s", oldBranch, newBranch)}
			}
			worktrees, err := m.git.GetWorktrees(m.ctx)
			return worktreesLoadedMsg{
				worktrees: worktrees,
				err:       err,
			}
		}, true
	}
	m.currentScreen = screenInput
	return textinput.Blink
}

// showPruneMerged initiates the prune merged worktrees workflow.
func (m *Model) showPruneMerged() tea.Cmd {
	if !m.git.IsGitHubOrGitLab(m.ctx) {
		return m.performMergedWorktreeCheck()
	}

	m.checkMergedAfterPRRefresh = true
	m.ciCache = make(map[string]*ciCacheEntry)
	m.prDataLoaded = false
	m.updateTable()
	m.updateTableColumns(m.worktreeTable.Width())
	m.loading = true
	m.loadingScreen = NewLoadingScreen("Fetching PR data...", m.theme)
	m.currentScreen = screenLoading
	return m.fetchPRData()
}

// performMergedWorktreeCheck checks for merged worktrees and shows a checklist.
func (m *Model) performMergedWorktreeCheck() tea.Cmd {
	mainBranch := m.git.GetMainBranch(m.ctx)

	wtBranches := make(map[string]*models.WorktreeInfo)
	for _, wt := range m.worktrees {
		if !wt.IsMain {
			wtBranches[wt.Branch] = wt
		}
	}

	// Track source for each candidate: "pr", "git", or "both"
	type candidate struct {
		wt     *models.WorktreeInfo
		source string
	}
	candidateMap := make(map[string]candidate)

	// 1. PR-based detection (existing logic)
	for _, wt := range m.worktrees {
		if wt.IsMain {
			continue
		}
		if wt.PR != nil && strings.EqualFold(wt.PR.State, "MERGED") {
			candidateMap[wt.Branch] = candidate{wt: wt, source: "pr"}
		}
	}

	// 2. Git-based detection
	mergedBranches := m.git.GetMergedBranches(m.ctx, mainBranch)
	for _, branch := range mergedBranches {
		if wt, exists := wtBranches[branch]; exists {
			if existing, found := candidateMap[branch]; found {
				existing.source = "both"
				candidateMap[branch] = existing
			} else {
				candidateMap[branch] = candidate{wt: wt, source: "git"}
			}
		}
	}

	if len(candidateMap) == 0 {
		m.showInfo("No merged worktrees to prune.", nil)
		return nil
	}

	// Build checklist items (pre-check clean worktrees, uncheck dirty ones)
	items := make([]ChecklistItem, 0, len(candidateMap))
	for branch, info := range candidateMap {
		// Get worktree name from path
		wtName := filepath.Base(info.wt.Path)

		var sourceLabel string
		switch info.source {
		case "pr":
			sourceLabel = "PR merged"
		case "git":
			sourceLabel = "branch merged"
		default:
			sourceLabel = "PR + branch merged"
		}

		desc := fmt.Sprintf("Branch: %s (%s)", branch, sourceLabel)

		// Check for uncommitted changes
		hasDirtyChanges := info.wt.Dirty || info.wt.Untracked > 0 || info.wt.Modified > 0 || info.wt.Staged > 0
		if hasDirtyChanges {
			desc += " - HAS UNCOMMITTED CHANGES!"
		}

		items = append(items, ChecklistItem{
			ID:          branch,
			Label:       wtName,
			Description: desc,
			Checked:     !hasDirtyChanges, // Uncheck dirty worktrees by default
		})
	}

	// Sort items for consistent ordering
	sort.Slice(items, func(i, j int) bool {
		return items[i].Label < items[j].Label
	})

	m.checklistScreen = NewChecklistScreen(
		items,
		"Prune Merged Worktrees",
		"Filter...",
		"No merged worktrees found.",
		m.windowWidth,
		m.windowHeight,
		m.theme,
	)

	m.checklistSubmit = func(selected []ChecklistItem) tea.Cmd {
		if len(selected) == 0 {
			return nil
		}

		// Collect worktrees to prune based on selection
		toPrune := make([]*models.WorktreeInfo, 0, len(selected))
		for _, item := range selected {
			if wt, exists := wtBranches[item.ID]; exists {
				toPrune = append(toPrune, wt)
			}
		}

		// Collect terminate commands once (same for all worktrees in this repo)
		terminateCmds := m.collectTerminateCommands()

		// Build the prune routine that runs terminate commands per-worktree
		pruneRoutine := func() tea.Msg {
			pruned := 0
			failed := 0
			for _, wt := range toPrune {
				// Run terminate commands for each worktree with its environment
				if len(terminateCmds) > 0 {
					env := m.buildCommandEnv(wt.Branch, wt.Path)
					_ = m.git.ExecuteCommands(m.ctx, terminateCmds, wt.Path, env)
				}

				ok1 := m.git.RunCommandChecked(m.ctx, []string{"git", "worktree", "remove", "--force", wt.Path}, "", fmt.Sprintf("Failed to remove worktree %s", wt.Path))
				ok2 := m.git.RunCommandChecked(m.ctx, []string{"git", "branch", "-D", wt.Branch}, "", fmt.Sprintf("Failed to delete branch %s", wt.Branch))
				if ok1 && ok2 {
					pruned++
				} else {
					failed++
				}
			}
			worktrees, err := m.git.GetWorktrees(m.ctx)
			return pruneResultMsg{
				worktrees: worktrees,
				err:       err,
				pruned:    pruned,
				failed:    failed,
			}
		}

		// Check trust for repo commands before running
		return m.runCommandsWithTrust(terminateCmds, "", nil, pruneRoutine)
	}
	m.currentScreen = screenChecklist
	return textinput.Blink
}

// showAbsorbWorktree shows a confirmation dialog for absorbing a worktree into main.
func (m *Model) showAbsorbWorktree() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	wt := m.filteredWts[m.selectedIndex]
	if wt.IsMain {
		m.infoScreen = NewInfoScreen("Cannot absorb the main worktree.", m.theme)
		m.currentScreen = screenInfo
		return nil
	}

	mainBranch := m.git.GetMainBranch(m.ctx)

	// Prevent absorbing if the selected worktree is on the main branch
	if wt.Branch == mainBranch {
		m.infoScreen = NewInfoScreen(
			fmt.Sprintf("Cannot absorb: worktree is on the main branch (%s).", mainBranch),
			m.theme,
		)
		m.currentScreen = screenInfo
		return nil
	}

	// Find the main worktree explicitly (don't use fallback)
	var mainWorktree *models.WorktreeInfo
	for _, w := range m.worktrees {
		if w.IsMain {
			mainWorktree = w
			break
		}
	}
	if mainWorktree == nil {
		m.infoScreen = NewInfoScreen("Cannot find main worktree.", m.theme)
		m.currentScreen = screenInfo
		return nil
	}

	// Check if main worktree has uncommitted changes
	if mainWorktree.Dirty {
		m.infoScreen = NewInfoScreen(
			fmt.Sprintf("Cannot absorb: main worktree has uncommitted changes.\n\nCommit or stash changes in:\n%s", mainWorktree.Path),
			m.theme,
		)
		m.currentScreen = screenInfo
		return nil
	}

	mainPath := mainWorktree.Path
	mergeMethod := m.config.MergeMethod
	if mergeMethod == "" {
		mergeMethod = mergeMethodRebase
	}

	m.confirmScreen = NewConfirmScreen(fmt.Sprintf("Absorb worktree into %s (%s)?\n\nPath: %s\nBranch: %s -> %s", mainBranch, mergeMethod, wt.Path, wt.Branch, mainBranch), m.theme)
	m.confirmAction = func() tea.Cmd {
		return func() tea.Msg {
			if mergeMethod == mergeMethodRebase {
				// Rebase: first rebase the feature branch onto main, then fast-forward main
				if !m.git.RunCommandChecked(m.ctx, []string{"git", "-C", wt.Path, "rebase", mainBranch}, "", fmt.Sprintf("Failed to rebase %s onto %s", wt.Branch, mainBranch)) {
					return absorbMergeResultMsg{
						path:   wt.Path,
						branch: wt.Branch,
						err:    fmt.Errorf("rebase failed; resolve conflicts in %s and retry", wt.Path),
					}
				}
				// Fast-forward main to the rebased branch
				if !m.git.RunCommandChecked(m.ctx, []string{"git", "-C", mainPath, "merge", "--ff-only", wt.Branch}, "", fmt.Sprintf("Failed to fast-forward %s to %s", mainBranch, wt.Branch)) {
					return absorbMergeResultMsg{
						path:   wt.Path,
						branch: wt.Branch,
						err:    fmt.Errorf("fast-forward failed; the branch may have diverged"),
					}
				}
			} else if !m.git.RunCommandChecked(m.ctx, []string{"git", "-C", mainPath, "merge", "--no-edit", wt.Branch}, "", fmt.Sprintf("Failed to merge %s into %s", wt.Branch, mainBranch)) {
				// Merge: traditional merge
				return absorbMergeResultMsg{
					path:   wt.Path,
					branch: wt.Branch,
					err:    fmt.Errorf("merge failed; resolve conflicts in %s and retry", mainPath),
				}
			}

			return absorbMergeResultMsg{
				path:   wt.Path,
				branch: wt.Branch,
			}
		}
	}
	m.currentScreen = screenConfirm
	return nil
}

// deleteWorktreeCmd returns a command function that deletes a worktree and its branch.
func (m *Model) deleteWorktreeCmd(wt *models.WorktreeInfo) func() tea.Cmd {
	env := m.buildCommandEnv(wt.Branch, wt.Path)
	terminateCmds := m.collectTerminateCommands()
	afterCmd := func() tea.Msg {
		m.git.RunCommandChecked(m.ctx, []string{"git", "worktree", "remove", "--force", wt.Path}, "", fmt.Sprintf("Failed to remove worktree %s", wt.Path))
		m.git.RunCommandChecked(m.ctx, []string{"git", "branch", "-D", wt.Branch}, "", fmt.Sprintf("Failed to delete branch %s", wt.Branch))

		worktrees, err := m.git.GetWorktrees(m.ctx)
		return worktreesLoadedMsg{
			worktrees: worktrees,
			err:       err,
		}
	}

	return func() tea.Cmd {
		return m.runCommandsWithTrust(terminateCmds, wt.Path, env, afterCmd)
	}
}

// deleteWorktreeOnlyCmd returns a command function that deletes only the worktree (not the branch).
func (m *Model) deleteWorktreeOnlyCmd(wt *models.WorktreeInfo) func() tea.Cmd {
	env := m.buildCommandEnv(wt.Branch, wt.Path)
	terminateCmds := m.collectTerminateCommands()

	afterCmd := func() tea.Msg {
		// Only remove worktree
		success := m.git.RunCommandChecked(
			m.ctx,
			[]string{"git", "worktree", "remove", "--force", wt.Path},
			"",
			fmt.Sprintf("Failed to remove worktree %s", wt.Path),
		)

		if !success {
			return worktreeDeletedMsg{
				path:   wt.Path,
				branch: wt.Branch,
				err:    fmt.Errorf("worktree deletion failed"),
			}
		}

		return worktreeDeletedMsg{
			path:   wt.Path,
			branch: wt.Branch,
			err:    nil,
		}
	}

	return func() tea.Cmd {
		return m.runCommandsWithTrust(terminateCmds, wt.Path, env, afterCmd)
	}
}

// deleteBranchCmd returns a command function that deletes a branch.
func (m *Model) deleteBranchCmd(branch string) func() tea.Cmd {
	return func() tea.Cmd {
		return func() tea.Msg {
			m.git.RunCommandChecked(
				m.ctx,
				[]string{"git", "branch", "-D", branch},
				"",
				fmt.Sprintf("Failed to delete branch %s", branch),
			)

			worktrees, err := m.git.GetWorktrees(m.ctx)
			return worktreesLoadedMsg{
				worktrees: worktrees,
				err:       err,
			}
		}
	}
}

// shellQuote quotes a string for safe use in shell commands.
func shellQuote(input string) string {
	if input == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(input, "'", "'\"'\"'") + "'"
}
