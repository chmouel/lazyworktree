package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/utils"
)

// showCreateWorktree shows the base selection screen for creating a new worktree.
func (m *Model) showCreateWorktree() tea.Cmd {
	defaultBase := m.state.services.git.GetMainBranch(m.ctx)
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
		statusRaw := m.state.services.git.RunGit(m.ctx, []string{"git", "status", "--porcelain"}, currentWt.Path, []int{0}, true, false)
		hasChanges := strings.TrimSpace(statusRaw) != ""

		// Get current branch
		currentBranch := m.state.services.git.RunGit(m.ctx, []string{"git", "rev-parse", "--abbrev-ref", "HEAD"}, currentWt.Path, []int{0}, true, false)
		if currentBranch == "" {
			return errMsg{err: fmt.Errorf("failed to get current branch")}
		}
		currentBranch = strings.TrimSpace(currentBranch)

		// Always generate random name as default
		defaultName := fmt.Sprintf("%s-%s", currentBranch, utils.RandomBranchName())

		// Get diff if changes exist (for later AI generation)
		var diff string
		if hasChanges && m.config.BranchNameScript != "" {
			diff = m.state.services.git.RunGit(m.ctx, []string{"git", "diff", "HEAD"}, currentWt.Path, []int{0}, false, true)
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

	branch := m.state.services.git.RunGit(
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
	if m.config.DisablePR {
		m.showInfo("PR/MR display is disabled in configuration", nil)
		return nil
	}
	// Fetch all open PRs
	return func() tea.Msg {
		prs, err := m.state.services.git.FetchAllOpenPRs(m.ctx)
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
		issues, err := m.state.services.git.FetchAllOpenIssues(m.ctx)
		return openIssuesLoadedMsg{
			issues: issues,
			err:    err,
		}
	}
}

// showCreateWorktreeFromChanges initiates creating a worktree from changes in the selected worktree.
func (m *Model) showCreateWorktreeFromChanges() tea.Cmd {
	// Check if a worktree is selected
	if m.state.data.selectedIndex < 0 || m.state.data.selectedIndex >= len(m.state.data.filteredWts) {
		m.showInfo(errNoWorktreeSelected, nil)
		return nil
	}

	wt := m.state.data.filteredWts[m.state.data.selectedIndex]

	// Check for changes in the selected worktree asynchronously
	return func() tea.Msg {
		statusRaw := m.state.services.git.RunGit(m.ctx, []string{"git", "status", "--porcelain"}, wt.Path, []int{0}, true, false)
		if strings.TrimSpace(statusRaw) == "" {
			return errMsg{err: fmt.Errorf("no changes to move")}
		}

		// Get current branch name
		currentBranch := m.state.services.git.RunGit(m.ctx, []string{"git", "rev-parse", "--abbrev-ref", "HEAD"}, wt.Path, []int{0}, true, false)
		if currentBranch == "" {
			return errMsg{err: fmt.Errorf("failed to get current branch")}
		}

		// Get diff if branch_name_script is configured
		var diff string
		if m.config.BranchNameScript != "" {
			diff = m.state.services.git.RunGit(m.ctx, []string{"git", "diff", "HEAD"}, wt.Path, []int{0}, false, true)
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
	inputScr := appscreen.NewInputScreen("Create worktree from changes: branch name", "feature/my-branch", defaultName, m.theme, m.config.IconsEnabled())

	inputScr.OnSubmit = func(value string, _ bool) tea.Cmd {
		newBranch := strings.TrimSpace(value)
		newBranch = sanitizeBranchNameFromTitle(newBranch, "")
		if newBranch == "" {
			inputScr.ErrorMsg = errBranchEmpty
			return nil
		}

		// Prevent duplicates - check if branch already exists in worktrees
		for _, existingWt := range m.state.data.worktrees {
			if existingWt.Branch == newBranch {
				inputScr.ErrorMsg = fmt.Sprintf("Branch %q already exists.", newBranch)
				return nil
			}
		}

		// Check if branch exists in git
		branchRef := m.state.services.git.RunGit(m.ctx, []string{"git", "show-ref", fmt.Sprintf("refs/heads/%s", newBranch)}, "", []int{0, 1}, true, true)
		if branchRef != "" {
			// Branch exists
			inputScr.ErrorMsg = fmt.Sprintf("Branch %q already exists.", newBranch)
			return nil
		}

		// Check if worktree path already exists
		targetPath := filepath.Join(m.getRepoWorktreeDir(), newBranch)
		if _, err := os.Stat(targetPath); err == nil {
			inputScr.ErrorMsg = fmt.Sprintf("Path already exists: %s", targetPath)
			return nil
		}

		inputScr.ErrorMsg = ""
		if err := os.MkdirAll(m.getRepoWorktreeDir(), 0o750); err != nil {
			return func() tea.Msg { return errMsg{err: fmt.Errorf("failed to create worktree directory: %w", err)} }
		}

		// Stash changes with descriptive message
		stashMessage := fmt.Sprintf("git-wt-create move-current: %s", newBranch)
		if !m.state.services.git.RunCommandChecked(
			m.ctx,
			[]string{"git", "stash", "push", "-u", "-m", stashMessage},
			wt.Path,
			"Failed to create stash for moving changes",
		) {
			return func() tea.Msg { return errMsg{err: fmt.Errorf("failed to create stash for moving changes")} }
		}

		// Get the stash ref
		stashRef := strings.TrimSpace(m.state.services.git.RunGit(m.ctx, []string{"git", "stash", "list", "-1", "--format=%gd"}, "", []int{0}, true, false))
		if stashRef == "" || !strings.HasPrefix(stashRef, "stash@{") {
			// Try to restore stash if we can't get the ref
			m.state.services.git.RunCommandChecked(m.ctx, []string{"git", "stash", "pop"}, wt.Path, "Failed to restore stash")
			return func() tea.Msg { return errMsg{err: fmt.Errorf("failed to get stash reference")} }
		}

		// Create the new worktree from current branch
		if !m.state.services.git.RunCommandChecked(
			m.ctx,
			[]string{"git", "worktree", "add", "-b", newBranch, targetPath, currentBranch},
			"",
			fmt.Sprintf("Failed to create worktree %s", newBranch),
		) {
			// If worktree creation fails, try to restore the stash
			m.state.services.git.RunCommandChecked(m.ctx, []string{"git", "stash", "pop"}, wt.Path, "Failed to restore stash")
			return func() tea.Msg { return errMsg{err: fmt.Errorf("failed to create worktree %s", newBranch)} }
		}

		// Apply stash to the new worktree
		if !m.state.services.git.RunCommandChecked(
			m.ctx,
			[]string{"git", "stash", "apply", "--index", stashRef},
			targetPath,
			"Failed to apply stash to new worktree",
		) {
			// If stash apply fails, clean up the worktree and try to restore stash to original location
			m.state.services.git.RunCommandChecked(m.ctx, []string{"git", "worktree", "remove", "--force", targetPath}, "", "Failed to remove worktree")
			m.state.services.git.RunCommandChecked(m.ctx, []string{"git", "stash", "pop"}, wt.Path, "Failed to restore stash")
			return func() tea.Msg { return errMsg{err: fmt.Errorf("failed to apply stash to new worktree")} }
		}

		// Drop the stash from the original location
		m.state.services.git.RunCommandChecked(m.ctx, []string{"git", "stash", "drop", stashRef}, wt.Path, "Failed to drop stash")

		// Run init commands and refresh
		env := m.buildCommandEnv(newBranch, targetPath)
		initCmds := m.collectInitCommands()
		after := func() tea.Msg {
			worktrees, err := m.state.services.git.GetWorktrees(m.ctx)
			return worktreesLoadedMsg{
				worktrees: worktrees,
				err:       err,
			}
		}
		return m.runCommandsWithTrust(initCmds, targetPath, env, after)
	}

	inputScr.OnCancel = func() tea.Cmd {
		return nil
	}

	m.state.ui.screenManager.Push(inputScr)
	return textinput.Blink
}

// generateAIBranchName generates a branch name using the configured AI script.
func (m *Model) generateAIBranchName() tea.Cmd {
	return func() tea.Msg {
		name, err := runBranchNameScript(
			m.ctx,
			m.config.BranchNameScript,
			m.createFromCurrent.diff,
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
	if m.createFromCurrent.diff == "" || m.createFromCurrent.inputScreen == nil {
		// Not in "create from current" flow, ignore
		return nil
	}

	inputScr := m.createFromCurrent.inputScreen

	if inputScr.CheckboxChecked {
		// Checkbox was checked: switch to AI name
		if m.createFromCurrent.aiName != "" {
			// Use cached AI name
			inputScr.Input.SetValue(m.createFromCurrent.aiName)
			inputScr.Input.CursorEnd()
			return nil
		}

		// Generate AI name if not cached
		if m.config.BranchNameScript != "" && m.createFromCurrent.diff != "" {
			return m.generateAIBranchName()
		}

		// No script configured, keep random name
		return nil
	}

	// Checkbox was unchecked: restore random name
	inputScr.Input.SetValue(m.createFromCurrent.randomName)
	inputScr.Input.CursorEnd()
	return nil
}

// handleCreateFromCurrentReady handles the createFromCurrentReadyMsg.
func (m *Model) handleCreateFromCurrentReady(msg createFromCurrentReadyMsg) tea.Cmd {
	if msg.currentWorktree == nil {
		m.showInfo("Could not determine current worktree", nil)
		return nil
	}

	// Store context for checkbox toggling
	m.createFromCurrent.diff = msg.diff
	m.createFromCurrent.randomName = msg.defaultBranchName
	m.createFromCurrent.branch = msg.currentBranch
	m.createFromCurrent.aiName = "" // Reset cached AI name

	// Show input screen with random name
	inputScr := appscreen.NewInputScreen("Create from current: branch name", "feature/my-branch", msg.defaultBranchName, m.theme, m.config.IconsEnabled())
	if msg.hasChanges {
		inputScr.SetCheckbox("Include current file changes", false)
	}

	// Store reference for checkbox toggle handling
	m.createFromCurrent.inputScreen = inputScr

	// Capture context for closure
	wt := msg.currentWorktree
	currentBranch := msg.currentBranch
	hasChanges := msg.hasChanges

	inputScr.OnSubmit = func(value string, checked bool) tea.Cmd {
		newBranch := strings.TrimSpace(value)
		newBranch = sanitizeBranchNameFromTitle(newBranch, "")
		if newBranch == "" {
			inputScr.ErrorMsg = errBranchEmpty
			return nil
		}

		// Validate branch doesn't exist
		if m.branchExistsInWorktrees(newBranch) {
			inputScr.ErrorMsg = fmt.Sprintf("Branch %q already exists.", newBranch)
			return nil
		}

		// Check if branch exists in git
		branchRef := m.state.services.git.RunGit(m.ctx, []string{"git", "show-ref", fmt.Sprintf("refs/heads/%s", newBranch)}, "", []int{0, 1}, true, true)
		if branchRef != "" {
			inputScr.ErrorMsg = fmt.Sprintf("Branch %q already exists.", newBranch)
			return nil
		}

		targetPath := filepath.Join(m.getRepoWorktreeDir(), newBranch)
		if m.worktreePathExists(targetPath) {
			inputScr.ErrorMsg = fmt.Sprintf("Path already exists: %s", targetPath)
			return nil
		}

		// Clear cached state
		m.createFromCurrent.diff = ""
		m.createFromCurrent.randomName = ""
		m.createFromCurrent.aiName = ""
		m.createFromCurrent.branch = ""
		m.createFromCurrent.inputScreen = nil

		// Set pending selection so the new worktree is selected after creation
		m.pendingSelectWorktreePath = targetPath

		// Only attempt to move changes if checkbox is checked AND there are actual changes
		// This prevents accidentally applying an unrelated existing stash when workspace is clean
		if checked && hasChanges {
			return m.executeCreateWithChanges(wt, currentBranch, newBranch, targetPath)
		}
		return m.executeCreateWithoutChanges(currentBranch, newBranch, targetPath)
	}

	inputScr.OnCancel = func() tea.Cmd {
		// Clear cached state on cancel
		m.createFromCurrent.diff = ""
		m.createFromCurrent.randomName = ""
		m.createFromCurrent.aiName = ""
		m.createFromCurrent.branch = ""
		m.createFromCurrent.inputScreen = nil
		return nil
	}

	inputScr.OnCheckboxToggle = func(checked bool) tea.Cmd {
		return m.handleCheckboxToggle()
	}

	m.state.ui.screenManager.Push(inputScr)
	return textinput.Blink
}

// executeCreateWithChanges creates a worktree and moves changes from the current worktree.
func (m *Model) executeCreateWithChanges(wt *models.WorktreeInfo, currentBranch, newBranch, targetPath string) tea.Cmd {
	return func() tea.Msg {
		if err := m.ensureWorktreeDir(m.getRepoWorktreeDir()); err != nil {
			return errMsg{err: err}
		}

		// Stash changes with descriptive message
		prevStashHash := m.state.services.git.RunGit(m.ctx, []string{"git", "stash", "list", "-1", "--format=%H"}, "", []int{0}, true, false)
		stashMessage := fmt.Sprintf("git-wt-create move-current: %s", newBranch)
		if !m.state.services.git.RunCommandChecked(
			m.ctx,
			[]string{"git", "stash", "push", "-u", "-m", stashMessage},
			wt.Path,
			"Failed to create stash for moving changes",
		) {
			return errMsg{err: fmt.Errorf("failed to create stash for moving changes")}
		}

		newStashHash := m.state.services.git.RunGit(m.ctx, []string{"git", "stash", "list", "-1", "--format=%H"}, "", []int{0}, true, false)
		if newStashHash == "" || newStashHash == prevStashHash {
			return errMsg{err: fmt.Errorf("failed to create stash for moving changes: no new entry created")}
		}

		// Get the stash ref
		stashRef := strings.TrimSpace(m.state.services.git.RunGit(m.ctx, []string{"git", "stash", "list", "-1", "--format=%gd"}, "", []int{0}, true, false))
		if stashRef == "" || !strings.HasPrefix(stashRef, "stash@{") {
			// Try to restore stash if we can't get the ref
			m.state.services.git.RunCommandChecked(m.ctx, []string{"git", "stash", "pop"}, wt.Path, "Failed to restore stash")
			return errMsg{err: fmt.Errorf("failed to get stash reference")}
		}

		// Create the new worktree from current branch
		if !m.state.services.git.RunCommandChecked(
			m.ctx,
			[]string{"git", "worktree", "add", "-b", newBranch, targetPath, currentBranch},
			"",
			fmt.Sprintf("Failed to create worktree %s", newBranch),
		) {
			// If worktree creation fails, try to restore the stash
			m.state.services.git.RunCommandChecked(m.ctx, []string{"git", "stash", "pop"}, wt.Path, "Failed to restore stash")
			return errMsg{err: fmt.Errorf("failed to create worktree %s", newBranch)}
		}

		// Apply stash to the new worktree
		if !m.state.services.git.RunCommandChecked(
			m.ctx,
			[]string{"git", "stash", "apply", "--index", stashRef},
			targetPath,
			"Failed to apply stash to new worktree",
		) {
			// If stash apply fails, clean up the worktree and try to restore stash to original location
			m.state.services.git.RunCommandChecked(m.ctx, []string{"git", "worktree", "remove", "--force", targetPath}, "", "Failed to remove worktree")
			m.state.services.git.RunCommandChecked(m.ctx, []string{"git", "stash", "pop"}, wt.Path, "Failed to restore stash")
			return errMsg{err: fmt.Errorf("failed to apply stash to new worktree")}
		}

		// Drop the stash from the original location
		m.state.services.git.RunCommandChecked(m.ctx, []string{"git", "stash", "drop", stashRef}, wt.Path, "Failed to drop stash")

		// Run init commands and refresh
		env := m.buildCommandEnv(newBranch, targetPath)
		initCmds := m.collectInitCommands()
		after := func() tea.Msg {
			worktrees, err := m.state.services.git.GetWorktrees(m.ctx)
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
		if err := m.ensureWorktreeDir(m.getRepoWorktreeDir()); err != nil {
			return errMsg{err: err}
		}

		args := []string{"git", "worktree", "add", "-b", newBranch, targetPath, currentBranch}
		if !m.state.services.git.RunCommandChecked(m.ctx, args, "", fmt.Sprintf("Failed to create worktree %s", newBranch)) {
			return errMsg{err: fmt.Errorf("failed to create worktree %s", newBranch)}
		}

		env := m.buildCommandEnv(newBranch, targetPath)
		initCmds := m.collectInitCommands()
		after := func() tea.Msg {
			worktrees, err := m.state.services.git.GetWorktrees(m.ctx)
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
	if m.state.data.selectedIndex < 0 || m.state.data.selectedIndex >= len(m.state.data.filteredWts) {
		return nil
	}
	wt := m.state.data.filteredWts[m.state.data.selectedIndex]
	if wt.IsMain {
		return nil
	}
	confirmScreen := appscreen.NewConfirmScreen(fmt.Sprintf("Delete worktree?\n\nPath: %s\nBranch: %s", wt.Path, wt.Branch), m.theme)
	confirmScreen.OnConfirm = m.deleteWorktreeOnlyCmd(wt)
	m.state.ui.screenManager.Push(confirmScreen)
	return nil
}

// showRenameWorktree shows an input screen for renaming a worktree.
func (m *Model) showRenameWorktree() tea.Cmd {
	if m.state.data.selectedIndex < 0 || m.state.data.selectedIndex >= len(m.state.data.filteredWts) {
		return nil
	}

	wt := m.state.data.filteredWts[m.state.data.selectedIndex]
	if wt.IsMain {
		m.showInfo("Cannot rename the main worktree.", nil)
		return nil
	}

	prompt := fmt.Sprintf("Enter new name for '%s'", wt.Branch)
	inputScr := appscreen.NewInputScreen(prompt, "New branch name", wt.Branch, m.theme, m.config.IconsEnabled())

	inputScr.OnSubmit = func(value string, _ bool) tea.Cmd {
		newBranch := strings.TrimSpace(value)
		newBranch = sanitizeBranchNameFromTitle(newBranch, "")
		if newBranch == "" {
			inputScr.ErrorMsg = "Name cannot be empty."
			return nil
		}
		if newBranch == wt.Branch {
			inputScr.ErrorMsg = "Name must be different from the current branch."
			return nil
		}

		parentDir := filepath.Dir(wt.Path)
		newPath := filepath.Join(parentDir, newBranch)
		if _, err := os.Stat(newPath); err == nil {
			inputScr.ErrorMsg = fmt.Sprintf("Destination already exists: %s", newPath)
			return nil
		}

		inputScr.ErrorMsg = ""
		oldPath := wt.Path
		oldBranch := wt.Branch

		return func() tea.Msg {
			ok := m.state.services.git.RenameWorktree(m.ctx, oldPath, newPath, oldBranch, newBranch)
			if !ok {
				return renameWorktreeResultMsg{
					oldPath: oldPath,
					newPath: newPath,
					err:     fmt.Errorf("failed to rename %s to %s", oldBranch, newBranch),
				}
			}
			worktrees, err := m.state.services.git.GetWorktrees(m.ctx)
			return renameWorktreeResultMsg{
				oldPath:   oldPath,
				newPath:   newPath,
				worktrees: worktrees,
				err:       err,
			}
		}
	}

	inputScr.OnCancel = func() tea.Cmd {
		return nil
	}

	m.state.ui.screenManager.Push(inputScr)
	return textinput.Blink
}

// showAnnotateWorktree opens notes for the selected worktree.
func (m *Model) showAnnotateWorktree() tea.Cmd {
	if m.state.view.FocusedPane != 0 {
		return nil
	}
	if m.state.data.selectedIndex < 0 || m.state.data.selectedIndex >= len(m.state.data.filteredWts) {
		m.showInfo(errNoWorktreeSelected, nil)
		return nil
	}

	wt := m.state.data.filteredWts[m.state.data.selectedIndex]
	existing, hasNote := m.getWorktreeNote(wt.Path)
	if hasNote {
		return m.showWorktreeNoteViewer(wt.Path, existing.Note)
	}
	return m.showWorktreeNoteEditor(wt.Path)
}

func (m *Model) showWorktreeNoteViewer(worktreePath, noteText string) tea.Cmd {
	valueStyle := lipgloss.NewStyle().Foreground(m.theme.TextFg)
	content := strings.Join(m.renderMarkdownNoteLines(noteText, valueStyle), "\n")
	viewer := appscreen.NewNoteViewScreen(
		"Worktree notes",
		content,
		m.state.view.WindowWidth,
		m.state.view.WindowHeight,
		m.theme,
	)
	viewer.OnEdit = func() tea.Cmd {
		return func() tea.Msg {
			return openNoteEditorMsg{worktreePath: worktreePath}
		}
	}
	viewer.OnEditExternal = func() tea.Cmd {
		return m.openNoteInExternalEditor(worktreePath, noteText)
	}
	m.state.ui.screenManager.Push(viewer)
	return nil
}

func (m *Model) openNoteInExternalEditor(worktreePath, noteText string) tea.Cmd {
	editor := m.editorCommand()
	if strings.TrimSpace(editor) == "" {
		m.showInfo("No editor configured. Set editor in config or $EDITOR.", nil)
		return nil
	}

	tmpFile, err := os.CreateTemp("", "lwt-note-*.md")
	if err != nil {
		m.showInfo(fmt.Sprintf("Failed to create temp file: %v", err), nil)
		return nil
	}
	tmpPath := tmpFile.Name()
	if _, err := tmpFile.WriteString(noteText); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath) //nolint:gosec // tmpPath is from os.CreateTemp, not user input
		m.showInfo(fmt.Sprintf("Failed to write temp file: %v", err), nil)
		return nil
	}
	_ = tmpFile.Close()

	cmdStr := fmt.Sprintf("%s %s", editor, shellQuote(tmpPath))
	// #nosec G204 -- command is constructed from user config and controlled inputs
	c := m.commandRunner(m.ctx, "bash", "-c", cmdStr)

	return m.execProcess(c, func(err error) tea.Msg {
		defer func() { _ = os.Remove(tmpPath) }()
		if err != nil {
			return errMsg{err: err}
		}
		// #nosec G304 -- tmpPath is created by os.CreateTemp above, not user-controlled
		content, readErr := os.ReadFile(tmpPath)
		if readErr != nil {
			return errMsg{err: readErr}
		}
		m.setWorktreeNote(worktreePath, string(content))
		return openNoteExternalEditorResultMsg{worktreePath: worktreePath}
	})
}

func (m *Model) showWorktreeNoteEditor(worktreePath string) tea.Cmd {
	if strings.TrimSpace(worktreePath) == "" {
		return nil
	}
	existing, _ := m.getWorktreeNote(worktreePath)
	textareaScr := appscreen.NewTextareaScreen(
		"Worktree notes",
		"Add notes for this worktree...",
		existing.Note,
		m.state.view.WindowWidth,
		m.state.view.WindowHeight,
		m.theme,
		m.config.IconsEnabled(),
	)
	textareaScr.SetValidation(func(value string) string {
		noteText := strings.TrimSpace(value)
		if len([]rune(noteText)) > worktreeNoteMaxChars {
			return fmt.Sprintf("Note is too long (%d/%d characters).", len([]rune(noteText)), worktreeNoteMaxChars)
		}
		return ""
	})
	textareaScr.OnSubmit = func(value string) tea.Cmd {
		m.setWorktreeNote(worktreePath, value)
		m.updateTable()
		if m.state.data.selectedIndex >= 0 && m.state.data.selectedIndex < len(m.state.data.filteredWts) {
			m.infoContent = m.buildInfoContent(m.state.data.filteredWts[m.state.data.selectedIndex])
		}
		return nil
	}
	textareaScr.OnCancel = func() tea.Cmd {
		return nil
	}

	m.state.ui.screenManager.Push(textareaScr)
	return textarea.Blink
}

// showPruneMerged initiates the prune merged worktrees workflow.
func (m *Model) showPruneMerged() tea.Cmd {
	if !m.state.services.git.IsGitHubOrGitLab(m.ctx) {
		return m.performMergedWorktreeCheck()
	}

	m.checkMergedAfterPRRefresh = true
	m.cache.ciCache.Clear()
	m.prDataLoaded = false
	m.updateTable()
	m.updateTableColumns(m.state.ui.worktreeTable.Width())
	m.loading = true
	m.setLoadingScreen("Fetching PR data...")
	return m.fetchPRData()
}

// performMergedWorktreeCheck checks for merged worktrees and shows a checklist.
func (m *Model) performMergedWorktreeCheck() tea.Cmd {
	mainBranch := m.state.services.git.GetMainBranch(m.ctx)

	wtBranches := make(map[string]*models.WorktreeInfo)
	for _, wt := range m.state.data.worktrees {
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
	for _, wt := range m.state.data.worktrees {
		if wt.IsMain {
			continue
		}
		if wt.PR != nil && strings.EqualFold(wt.PR.State, "MERGED") {
			candidateMap[wt.Branch] = candidate{wt: wt, source: "pr"}
		}
	}

	// 2. Git-based detection
	mergedBranches := m.state.services.git.GetMergedBranches(m.ctx, mainBranch)
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

	// 3. Detect orphaned directories (exist on disk but not in git worktree list)
	orphanedDirs := m.findOrphanedWorktreeDirs()

	if len(candidateMap) == 0 && len(orphanedDirs) == 0 {
		m.showInfo("No merged worktrees or orphaned directories to prune.", nil)
		return nil
	}

	// Build checklist items (pre-check clean worktrees, uncheck dirty ones)
	items := make([]appscreen.ChecklistItem, 0, len(candidateMap)+len(orphanedDirs))

	// Add merged worktrees
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

		items = append(items, appscreen.ChecklistItem{
			ID:          branch,
			Label:       wtName,
			Description: desc,
			Checked:     !hasDirtyChanges, // Uncheck dirty worktrees by default
		})
	}

	// Add orphaned directories with special prefix to distinguish them
	for _, orphanPath := range orphanedDirs {
		dirName := filepath.Base(orphanPath)
		items = append(items, appscreen.ChecklistItem{
			ID:          "orphan:" + orphanPath,
			Label:       dirName,
			Description: fmt.Sprintf("Orphaned directory: %s (not in git worktree list)", orphanPath),
			Checked:     false, // Require explicit selection for deletion
		})
	}

	// Sort items for consistent ordering
	sort.Slice(items, func(i, j int) bool {
		return items[i].Label < items[j].Label
	})

	checkScreen := appscreen.NewChecklistScreen(
		items,
		"Prune Merged Worktrees & Orphans",
		"Filter...",
		"No merged worktrees or orphaned directories found.",
		m.state.view.WindowWidth,
		m.state.view.WindowHeight,
		m.theme,
	)

	checkScreen.OnSubmit = func(selected []appscreen.ChecklistItem) tea.Cmd {
		if len(selected) == 0 {
			return nil
		}

		// Separate worktrees from orphaned directories
		toPrune := make([]*models.WorktreeInfo, 0, len(selected))
		orphansToDelete := make([]string, 0)
		for _, item := range selected {
			if orphanPath, isOrphan := strings.CutPrefix(item.ID, "orphan:"); isOrphan {
				orphansToDelete = append(orphansToDelete, orphanPath)
			} else if wt, exists := wtBranches[item.ID]; exists {
				toPrune = append(toPrune, wt)
			}
		}

		// Collect terminate commands once (same for all worktrees in this repo)
		terminateCmds := m.collectTerminateCommands()

		// Build the prune routine that runs terminate commands per-worktree
		pruneRoutine := func() tea.Msg {
			// First, run git worktree prune to clean up git's internal tracking
			m.state.services.git.RunGit(m.ctx, []string{"git", "worktree", "prune"}, "", []int{0}, true, true)

			pruned := 0
			failed := 0
			orphansDeleted := 0

			// Prune merged worktrees
			for _, wt := range toPrune {
				// Run terminate commands for each worktree with its environment
				if len(terminateCmds) > 0 {
					env := m.buildCommandEnv(wt.Branch, wt.Path)
					_ = m.state.services.git.ExecuteCommands(m.ctx, terminateCmds, wt.Path, env)
				}

				ok1 := m.state.services.git.RunCommandChecked(m.ctx, []string{"git", "worktree", "remove", "--force", wt.Path}, "", fmt.Sprintf("Failed to remove worktree %s", wt.Path))
				ok2 := m.state.services.git.RunCommandChecked(m.ctx, []string{"git", "branch", "-D", wt.Branch}, "", fmt.Sprintf("Failed to delete branch %s", wt.Branch))
				if ok1 && ok2 {
					pruned++
				} else {
					failed++
				}
			}

			// Delete orphaned directories
			// Re-fetch valid paths to ensure we have current state
			validPaths := m.getValidWorktreePaths()
			repoDir := m.getRepoWorktreeDir()

			for _, orphanPath := range orphansToDelete {
				// Re-validate: skip if now registered with git
				if validPaths != nil {
					normalizedPath := normalizePath(orphanPath)
					if validPaths[normalizedPath] {
						// Path is now a valid worktree, skip deletion
						continue
					}
				}

				// Verify path is still within expected repo directory bounds
				if !strings.HasPrefix(orphanPath, repoDir) {
					failed++
					continue
				}

				if err := os.RemoveAll(orphanPath); err != nil {
					failed++
				} else {
					orphansDeleted++
				}
			}

			worktrees, err := m.state.services.git.GetWorktrees(m.ctx)
			return pruneResultMsg{
				worktrees:      worktrees,
				err:            err,
				pruned:         pruned,
				failed:         failed,
				orphansDeleted: orphansDeleted,
			}
		}

		// Check trust for repo commands before running
		return m.runCommandsWithTrust(terminateCmds, "", nil, pruneRoutine)
	}

	checkScreen.OnCancel = func() tea.Cmd {
		return nil
	}

	m.state.ui.screenManager.Push(checkScreen)
	return textinput.Blink
}

// showAbsorbWorktree shows a confirmation dialog for absorbing a worktree into main.
func (m *Model) showAbsorbWorktree() tea.Cmd {
	if m.state.data.selectedIndex < 0 || m.state.data.selectedIndex >= len(m.state.data.filteredWts) {
		return nil
	}
	wt := m.state.data.filteredWts[m.state.data.selectedIndex]
	if wt.IsMain {
		m.showInfo("Cannot absorb the main worktree.", nil)
		return nil
	}

	mainBranch := m.state.services.git.GetMainBranch(m.ctx)

	// Prevent absorbing if the selected worktree is on the main branch
	if wt.Branch == mainBranch {
		m.showInfo(fmt.Sprintf("Cannot absorb: worktree is on the main branch (%s).", mainBranch), nil)
		return nil
	}

	// Find the main worktree explicitly (don't use fallback)
	var mainWorktree *models.WorktreeInfo
	for _, w := range m.state.data.worktrees {
		if w.IsMain {
			mainWorktree = w
			break
		}
	}
	if mainWorktree == nil {
		m.showInfo("Cannot find main worktree.", nil)
		return nil
	}

	// Check if main worktree has uncommitted changes
	if mainWorktree.Dirty {
		m.showInfo(fmt.Sprintf("Cannot absorb: main worktree has uncommitted changes.\n\nCommit or stash changes in:\n%s", mainWorktree.Path), nil)
		return nil
	}

	mainPath := mainWorktree.Path
	mergeMethod := m.config.MergeMethod
	if mergeMethod == "" {
		mergeMethod = mergeMethodRebase
	}

	confirmScreen := appscreen.NewConfirmScreen(fmt.Sprintf("Absorb worktree into %s (%s)?\n\nPath: %s\nBranch: %s -> %s", mainBranch, mergeMethod, wt.Path, wt.Branch, mainBranch), m.theme)
	confirmScreen.OnConfirm = func() tea.Cmd {
		return func() tea.Msg {
			if mergeMethod == mergeMethodRebase {
				// Rebase: first rebase the feature branch onto main, then fast-forward main
				if !m.state.services.git.RunCommandChecked(m.ctx, []string{"git", "-C", wt.Path, "rebase", mainBranch}, "", fmt.Sprintf("Failed to rebase %s onto %s", wt.Branch, mainBranch)) {
					return absorbMergeResultMsg{
						path:   wt.Path,
						branch: wt.Branch,
						err:    fmt.Errorf("rebase failed; resolve conflicts in %s and retry", wt.Path),
					}
				}
				// Fast-forward main to the rebased branch
				if !m.state.services.git.RunCommandChecked(m.ctx, []string{"git", "-C", mainPath, "merge", "--ff-only", wt.Branch}, "", fmt.Sprintf("Failed to fast-forward %s to %s", mainBranch, wt.Branch)) {
					return absorbMergeResultMsg{
						path:   wt.Path,
						branch: wt.Branch,
						err:    fmt.Errorf("fast-forward failed; the branch may have diverged"),
					}
				}
			} else if !m.state.services.git.RunCommandChecked(m.ctx, []string{"git", "-C", mainPath, "merge", "--no-edit", wt.Branch}, "", fmt.Sprintf("Failed to merge %s into %s", wt.Branch, mainBranch)) {
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
	m.state.ui.screenManager.Push(confirmScreen)
	return nil
}

// deleteWorktreeCmd returns a command function that deletes a worktree and its branch.
func (m *Model) deleteWorktreeCmd(wt *models.WorktreeInfo) func() tea.Cmd {
	env := m.buildCommandEnv(wt.Branch, wt.Path)
	terminateCmds := m.collectTerminateCommands()
	afterCmd := func() tea.Msg {
		m.state.services.git.RunCommandChecked(m.ctx, []string{"git", "worktree", "remove", "--force", wt.Path}, "", fmt.Sprintf("Failed to remove worktree %s", wt.Path))
		m.state.services.git.RunCommandChecked(m.ctx, []string{"git", "branch", "-D", wt.Branch}, "", fmt.Sprintf("Failed to delete branch %s", wt.Branch))

		worktrees, err := m.state.services.git.GetWorktrees(m.ctx)
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
		success := m.state.services.git.RunCommandChecked(
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
			m.state.services.git.RunCommandChecked(
				m.ctx,
				[]string{"git", "branch", "-D", branch},
				"",
				fmt.Sprintf("Failed to delete branch %s", branch),
			)

			worktrees, err := m.state.services.git.GetWorktrees(m.ctx)
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
