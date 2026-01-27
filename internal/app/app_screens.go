package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/theme"
)

func screenName(screen screenType) string {
	switch screen {
	case screenNone:
		return "none"
	case screenConfirm:
		return "confirm"
	case screenInfo:
		return "info"
	case screenInput:
		return "input"
	case screenHelp:
		return "help"
	case screenTrust:
		return "trust"
	case screenWelcome:
		return "welcome"
	case screenCommit:
		return "commit"
	case screenPalette:
		return "palette"
	case screenPRSelect:
		return "pr-select"
	case screenIssueSelect:
		return "issue-select"
	case screenListSelect:
		return "list-select"
	case screenCommitFiles:
		return "commit-files"
	case screenChecklist:
		return "checklist"
	default:
		return "unknown"
	}
}

func (m *Model) showInfo(message string, action tea.Cmd) {
	m.infoScreen = NewInfoScreen(message, m.theme)
	m.infoAction = action
	m.currentScreen = screenInfo
}

func (m *Model) handleScreenKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.debugf("screen key: %s screen=%s", msg.String(), screenName(m.currentScreen))
	switch m.currentScreen {
	case screenHelp:
		if m.helpScreen == nil {
			m.helpScreen = NewHelpScreen(m.windowWidth, m.windowHeight, m.config.CustomCommands, m.theme, m.config.IconsEnabled())
		}
		keyStr := msg.String()
		if keyStr == keyQ || isEscKey(keyStr) {
			// If currently searching, esc clears search; otherwise close help
			if m.helpScreen.searching || m.helpScreen.searchQuery != "" {
				m.helpScreen.searching = false
				m.helpScreen.searchInput.Blur()
				m.helpScreen.searchInput.SetValue("")
				m.helpScreen.searchQuery = ""
				m.helpScreen.refreshContent()
				return m, nil
			}
			m.currentScreen = screenNone
			m.helpScreen = nil
			return m, nil
		}
		hs, cmd := m.helpScreen.Update(msg)
		if updated, ok := hs.(*HelpScreen); ok {
			m.helpScreen = updated
		}
		return m, cmd
	case screenPalette:
		if m.paletteScreen == nil {
			m.currentScreen = screenNone
			return m, nil
		}
		keyStr := msg.String()
		if isEscKey(keyStr) {
			m.currentScreen = screenNone
			m.paletteScreen = nil
			return m, nil
		}
		if keyStr == keyEnter {
			if m.paletteSubmit != nil {
				if action, ok := m.paletteScreen.Selected(); ok {
					cmd := m.paletteSubmit(action)
					if m.currentScreen == screenPalette {
						m.currentScreen = screenNone
					}
					m.paletteScreen = nil
					m.paletteSubmit = nil
					return m, cmd
				}
			}
		}
		ps, cmd := m.paletteScreen.Update(msg)
		if updated, ok := ps.(*CommandPaletteScreen); ok {
			m.paletteScreen = updated
		}
		return m, cmd
	case screenPRSelect:
		if m.prSelectionScreen == nil {
			m.currentScreen = screenNone
			return m, nil
		}
		keyStr := msg.String()
		if isEscKey(keyStr) {
			m.currentScreen = screenNone
			m.prSelectionScreen = nil
			m.prSelectionSubmit = nil
			return m, nil
		}
		if keyStr == keyEnter {
			if m.prSelectionSubmit != nil {
				if pr, ok := m.prSelectionScreen.Selected(); ok {
					cmd := m.prSelectionSubmit(pr)
					// Don't set screenNone here - prSelectionSubmit sets screenInput
					m.prSelectionScreen = nil
					m.prSelectionSubmit = nil
					return m, cmd
				}
			}
		}
		ps, cmd := m.prSelectionScreen.Update(msg)
		if updated, ok := ps.(*PRSelectionScreen); ok {
			m.prSelectionScreen = updated
		}
		return m, cmd
	case screenIssueSelect:
		if m.issueSelectionScreen == nil {
			m.currentScreen = screenNone
			return m, nil
		}
		keyStr := msg.String()
		if isEscKey(keyStr) {
			m.currentScreen = screenNone
			m.issueSelectionScreen = nil
			m.issueSelectionSubmit = nil
			return m, nil
		}
		if keyStr == keyEnter {
			if m.issueSelectionSubmit != nil {
				if issue, ok := m.issueSelectionScreen.Selected(); ok {
					cmd := m.issueSelectionSubmit(issue)
					// Don't set screenNone here - issueSelectionSubmit sets screenInput or screenListSelect
					m.issueSelectionScreen = nil
					m.issueSelectionSubmit = nil
					return m, cmd
				}
			}
		}
		is, cmd := m.issueSelectionScreen.Update(msg)
		if updated, ok := is.(*IssueSelectionScreen); ok {
			m.issueSelectionScreen = updated
		}
		return m, cmd
	case screenListSelect:
		if m.listScreen == nil {
			m.currentScreen = screenNone
			return m, nil
		}
		keyStr := msg.String()
		if isEscKey(keyStr) {
			if m.originalTheme != "" {
				m.UpdateTheme(m.originalTheme)
				m.originalTheme = ""
			}
			m.listScreen = nil
			m.listSubmit = nil
			m.listScreenCIChecks = nil
			m.currentScreen = screenNone
			return m, nil
		}
		// Enter: Open CI job URL in browser (only when viewing CI checks)
		if keyStr == keyEnter && m.listScreenCIChecks != nil {
			if item, ok := m.listScreen.Selected(); ok {
				var idx int
				if _, err := fmt.Sscanf(item.id, "%d", &idx); err == nil && idx >= 0 && idx < len(m.listScreenCIChecks) {
					check := m.listScreenCIChecks[idx]
					return m, m.openURLInBrowser(check.Link)
				}
			}
			return m, nil
		}
		// Ctrl+V: View CI check logs in pager (only when viewing CI checks)
		if keyStr == "ctrl+v" && m.listScreenCIChecks != nil {
			if item, ok := m.listScreen.Selected(); ok {
				var idx int
				if _, err := fmt.Sscanf(item.id, "%d", &idx); err == nil && idx >= 0 && idx < len(m.listScreenCIChecks) {
					check := m.listScreenCIChecks[idx]
					return m, m.showCICheckLog(check)
				}
			}
			return m, nil
		}
		// Ctrl+R: Restart CI job (only when viewing CI checks)
		if keyStr == "ctrl+r" && m.listScreenCIChecks != nil {
			if item, ok := m.listScreen.Selected(); ok {
				var idx int
				if _, err := fmt.Sscanf(item.id, "%d", &idx); err == nil && idx >= 0 && idx < len(m.listScreenCIChecks) {
					check := m.listScreenCIChecks[idx]
					cmd := m.rerunCICheck(check)
					if cmd != nil {
						return m, cmd
					}
				}
			}
			return m, nil
		}
		if keyStr == keyEnter {
			if m.listSubmit != nil {
				if item, ok := m.listScreen.Selected(); ok {
					cmd := m.listSubmit(item)
					return m, cmd
				}
			}
		}
		ls, cmd := m.listScreen.Update(msg)
		if updated, ok := ls.(*ListSelectionScreen); ok {
			m.listScreen = updated
		}
		return m, cmd
	case screenChecklist:
		if m.checklistScreen == nil {
			m.currentScreen = screenNone
			return m, nil
		}
		keyStr := msg.String()
		if isEscKey(keyStr) {
			m.checklistScreen = nil
			m.checklistSubmit = nil
			m.currentScreen = screenNone
			return m, nil
		}
		if keyStr == keyEnter {
			if m.checklistSubmit != nil {
				selected := m.checklistScreen.SelectedItems()
				cmd := m.checklistSubmit(selected)
				m.checklistScreen = nil
				m.checklistSubmit = nil
				m.currentScreen = screenNone
				return m, cmd
			}
		}
		cs, cmd := m.checklistScreen.Update(msg)
		if updated, ok := cs.(*ChecklistScreen); ok {
			m.checklistScreen = updated
		}
		return m, cmd
	case screenCommitFiles:
		if m.commitFilesScreen == nil {
			m.currentScreen = screenNone
			return m, nil
		}
		keyStr := msg.String()

		// If filter or search is active, delegate to screen first
		if m.commitFilesScreen.showingFilter || m.commitFilesScreen.showingSearch {
			cs, cmd := m.commitFilesScreen.Update(msg)
			if updated, ok := cs.(*CommitFilesScreen); ok {
				m.commitFilesScreen = updated
			}
			return m, cmd
		}

		switch keyStr {
		case keyQ, keyCtrlC:
			m.commitFilesScreen = nil
			m.currentScreen = screenNone
			return m, nil
		case keyEsc, keyEscRaw:
			m.commitFilesScreen = nil
			m.currentScreen = screenNone
			return m, nil
		case "f":
			// Start filter mode
			m.commitFilesScreen.showingFilter = true
			m.commitFilesScreen.showingSearch = false
			m.commitFilesScreen.filterInput.Placeholder = placeholderFilterFiles
			m.commitFilesScreen.filterInput.SetValue(m.commitFilesScreen.filterQuery)
			m.commitFilesScreen.filterInput.Focus()
			return m, textinput.Blink
		case "/":
			// Start search mode
			m.commitFilesScreen.showingSearch = true
			m.commitFilesScreen.showingFilter = false
			m.commitFilesScreen.filterInput.Placeholder = searchFiles
			m.commitFilesScreen.filterInput.SetValue("")
			m.commitFilesScreen.searchQuery = ""
			m.commitFilesScreen.filterInput.Focus()
			return m, textinput.Blink
		case "d":
			// Show full commit diff via pager
			sha := m.commitFilesScreen.commitSHA
			wtPath := m.commitFilesScreen.worktreePath
			m.commitFilesScreen = nil
			m.currentScreen = screenNone
			// Find the worktree to pass to showCommitDiff
			var wt *models.WorktreeInfo
			for _, w := range m.filteredWts {
				if w.Path == wtPath {
					wt = w
					break
				}
			}
			if wt != nil {
				return m, m.showCommitDiff(sha, wt)
			}
			return m, nil
		case keyEnter:
			node := m.commitFilesScreen.GetSelectedNode()
			if node == nil {
				return m, nil
			}
			if node.IsDir() {
				m.commitFilesScreen.ToggleCollapse(node.Path)
				return m, nil
			}
			// Show diff for this specific file
			sha := m.commitFilesScreen.commitSHA
			wtPath := m.commitFilesScreen.worktreePath
			return m, m.showCommitFileDiff(sha, node.File.Filename, wtPath)
		}
		// Delegate navigation to screen
		cs, cmd := m.commitFilesScreen.Update(msg)
		if updated, ok := cs.(*CommitFilesScreen); ok {
			m.commitFilesScreen = updated
		}
		return m, cmd
	case screenConfirm:
		if m.confirmScreen != nil {
			_, cmd := m.confirmScreen.Update(msg)
			// Check if the confirm screen sent a result
			select {
			case confirmed := <-m.confirmScreen.result:
				if confirmed {
					// Perform confirmed action (delete, prune, etc.)
					var actionCmd tea.Cmd
					if m.confirmAction != nil {
						actionCmd = m.confirmAction()
					}
					m.confirmScreen = nil
					m.confirmAction = nil
					m.confirmCancel = nil
					if m.currentScreen == screenConfirm {
						m.currentScreen = screenNone
					}
					if actionCmd != nil {
						return m, actionCmd
					}
					return m, nil
				} else {
					// Action cancelled
					var cancelCmd tea.Cmd
					if m.confirmCancel != nil {
						cancelCmd = m.confirmCancel()
					}
					m.confirmScreen = nil
					m.confirmAction = nil
					m.confirmCancel = nil
					m.currentScreen = screenNone
					if cancelCmd != nil {
						return m, cancelCmd
					}
					return m, nil
				}
			default:
				return m, cmd
			}
		}
	case screenInfo:
		if m.infoScreen != nil {
			_, cmd := m.infoScreen.Update(msg)
			select {
			case <-m.infoScreen.result:
				action := m.infoAction
				m.infoScreen = nil
				m.infoAction = nil
				m.currentScreen = screenNone
				if action != nil {
					return m, action
				}
				return m, nil
			default:
				return m, cmd
			}
		}
	case screenWelcome:
		keyStr := msg.String()
		switch {
		case keyStr == "r" || keyStr == "R":
			m.currentScreen = screenNone
			m.welcomeScreen = nil
			return m, m.refreshWorktrees()
		case keyStr == keyQ || keyStr == "Q" || keyStr == "enter" || isEscKey(keyStr):
			m.quitting = true
			m.stopGitWatcher()
			return m, tea.Quit
		}
	case screenTrust:
		if m.trustScreen == nil {
			m.currentScreen = screenNone
			return m, nil
		}
		keyStr := msg.String()
		switch {
		case keyStr == "t" || keyStr == "T":
			if m.pendingTrust != "" {
				_ = m.trustManager.TrustFile(m.pendingTrust)
			}
			cmd := m.runCommands(m.pendingCommands, m.pendingCmdCwd, m.pendingCmdEnv, m.pendingAfter)
			m.clearPendingTrust()
			m.currentScreen = screenNone
			return m, cmd
		case keyStr == "b" || keyStr == "B":
			after := m.pendingAfter
			m.clearPendingTrust()
			m.currentScreen = screenNone
			if after != nil {
				return m, after
			}
			return m, nil
		case keyStr == "c" || keyStr == "C" || isEscKey(keyStr):
			m.clearPendingTrust()
			m.currentScreen = screenNone
			return m, nil
		}
		ts, cmd := m.trustScreen.Update(msg)
		if updated, ok := ts.(*TrustScreen); ok {
			m.trustScreen = updated
		}
		return m, cmd
	case screenCommit:
		if m.commitScreen == nil {
			m.currentScreen = screenNone
			return m, nil
		}
		keyStr := msg.String()
		if keyStr == keyQ || isEscKey(keyStr) {
			m.commitScreen = nil
			m.currentScreen = screenNone
			return m, nil
		}
		cs, cmd := m.commitScreen.Update(msg)
		if updated, ok := cs.(*CommitScreen); ok {
			m.commitScreen = updated
		}
		return m, cmd
	case screenInput:
		if m.inputScreen == nil {
			m.currentScreen = screenNone
			return m, nil
		}

		keyStr := msg.String()
		if isEscKey(keyStr) {
			// Clear cached state on exit
			m.createFromCurrentDiff = ""
			m.createFromCurrentRandomName = ""
			m.createFromCurrentAIName = ""
			m.createFromCurrentBranch = ""

			m.inputScreen = nil
			m.inputSubmit = nil
			m.currentScreen = screenNone
			return m, nil
		}
		if keyStr == keyEnter {
			if m.inputScreen.validate != nil {
				if errMsg := strings.TrimSpace(m.inputScreen.validate(m.inputScreen.input.Value())); errMsg != "" {
					m.inputScreen.errorMsg = errMsg
					return m, nil
				}
				m.inputScreen.errorMsg = ""
			}
			if m.inputSubmit != nil {
				cmd, closeCmd := m.inputSubmit(m.inputScreen.input.Value(), m.inputScreen.checkboxChecked)
				if closeCmd {
					m.inputScreen = nil
					m.inputSubmit = nil
					if m.currentScreen == screenInput {
						m.currentScreen = screenNone
					}
				}
				return m, cmd
			}
		}

		// Handle history navigation with up/down arrows
		if len(m.inputScreen.history) > 0 {
			switch keyStr {
			case "up":
				// Go to previous command in history (older)
				if m.inputScreen.historyIndex == -1 {
					m.inputScreen.originalInput = m.inputScreen.input.Value()
					m.inputScreen.historyIndex = 0
				} else if m.inputScreen.historyIndex < len(m.inputScreen.history)-1 {
					m.inputScreen.historyIndex++
				}
				if m.inputScreen.historyIndex >= 0 && m.inputScreen.historyIndex < len(m.inputScreen.history) {
					m.inputScreen.input.SetValue(m.inputScreen.history[m.inputScreen.historyIndex])
					m.inputScreen.input.CursorEnd()
				}
				return m, nil
			case "down":
				// Go to next command in history (newer)
				if m.inputScreen.historyIndex > 0 {
					m.inputScreen.historyIndex--
					m.inputScreen.input.SetValue(m.inputScreen.history[m.inputScreen.historyIndex])
					m.inputScreen.input.CursorEnd()
				} else if m.inputScreen.historyIndex == 0 {
					m.inputScreen.historyIndex = -1
					m.inputScreen.input.SetValue(m.inputScreen.originalInput)
					m.inputScreen.input.CursorEnd()
				}
				return m, nil
			}
		}

		// Reset history browsing when user types
		if msg.Type == tea.KeyRunes || msg.Type == tea.KeyBackspace || msg.Type == tea.KeyDelete {
			m.inputScreen.historyIndex = -1
		}

		// Store previous checkbox state before update
		prevCheckboxState := m.inputScreen.checkboxChecked

		var cmd tea.Cmd
		_, cmd = m.inputScreen.Update(msg)

		// Detect checkbox state change
		if m.inputScreen.checkboxEnabled && prevCheckboxState != m.inputScreen.checkboxChecked {
			return m, tea.Batch(cmd, m.handleCheckboxToggle())
		}

		return m, cmd
	}
	return m, nil
}

func (m *Model) showCommandPalette() tea.Cmd {
	m.debugf("open palette")
	customItems := m.customPaletteItems()
	items := make([]paletteItem, 0, 40+len(customItems))

	// Build MRU section and track which items are in it
	mruIDs := make(map[string]bool)
	m.debugf("palette MRU: enabled=%v, history_len=%d", m.config.PaletteMRU, len(m.paletteHistory))
	if m.config.PaletteMRU && len(m.paletteHistory) > 0 {
		mruItems := m.buildMRUPaletteItems()
		m.debugf("palette MRU: built %d items", len(mruItems))
		if len(mruItems) > 0 {
			items = append(items, paletteItem{label: "Recently Used", isSection: true})
			items = append(items, mruItems...)
			// Track MRU IDs to exclude from other sections
			for _, item := range mruItems {
				if item.id != "" {
					mruIDs[item.id] = true
				}
			}
		}
	}

	// Helper to add item only if not in MRU
	addItem := func(item paletteItem) {
		if item.id == "" || !mruIDs[item.id] {
			items = append(items, item)
		}
	}

	// Section: Worktree Actions
	items = append(items, paletteItem{label: "Worktree Actions", isSection: true})
	addItem(paletteItem{id: "create", label: "Create worktree (c)", description: "Add a new worktree from base branch or PR/MR"})
	addItem(paletteItem{id: "delete", label: "Delete worktree (D)", description: "Remove worktree and branch"})
	addItem(paletteItem{id: "rename", label: "Rename worktree (m)", description: "Rename worktree and branch"})
	addItem(paletteItem{id: "absorb", label: "Absorb worktree (A)", description: "Merge branch into main and remove worktree"})
	addItem(paletteItem{id: "prune", label: "Prune merged (X)", description: "Remove merged PR worktrees"})

	// Section: Create Shortcuts
	items = append(items, paletteItem{label: "Create Shortcuts", isSection: true})
	addItem(paletteItem{id: "create-from-current", label: "Create worktree from current branch", description: "Create from current branch with or without changes"})
	addItem(paletteItem{id: "create-from-branch", label: "Create worktree from branch/tag", description: "Select a branch, tag, or remote as base"})
	addItem(paletteItem{id: "create-from-commit", label: "Create worktree from commit", description: "Choose a branch, then select a specific commit"})
	addItem(paletteItem{id: "create-from-pr", label: "Create worktree from PR/MR", description: "Create from a pull/merge request"})
	addItem(paletteItem{id: "create-from-issue", label: "Create worktree from issue", description: "Create from a GitHub/GitLab issue"})
	addItem(paletteItem{id: "create-freeform", label: "Create worktree from ref", description: "Enter a branch, tag, or commit manually"})

	// Section: Git Operations
	items = append(items, paletteItem{label: "Git Operations", isSection: true})
	addItem(paletteItem{id: "diff", label: "Show diff (d)", description: "Show diff for current worktree or commit"})
	addItem(paletteItem{id: "refresh", label: "Refresh (r)", description: "Reload worktrees"})
	addItem(paletteItem{id: "fetch", label: "Fetch remotes (R)", description: "git fetch --all"})
	addItem(paletteItem{id: "push", label: "Push to upstream (P)", description: "git push (clean worktree only)"})
	addItem(paletteItem{id: "sync", label: "Synchronise with upstream (S)", description: "git pull, then git push (clean worktree only)"})
	addItem(paletteItem{id: "fetch-pr-data", label: "Fetch PR data (p)", description: "Fetch PR/MR status from GitHub/GitLab"})
	if m.git != nil && m.git.IsGitHub(m.ctx) {
		addItem(paletteItem{id: "ci-checks", label: "View CI checks (v)", description: "View CI check logs for current worktree"})
	}
	addItem(paletteItem{id: "pr", label: "Open PR (o)", description: "Open PR in browser"})
	addItem(paletteItem{id: "lazygit", label: "Open LazyGit (g)", description: "Open LazyGit in selected worktree"})
	addItem(paletteItem{id: "run-command", label: "Run command (!)", description: "Run arbitrary command in worktree"})

	// Section: Status Pane
	items = append(items, paletteItem{label: "Status Pane", isSection: true})
	addItem(paletteItem{id: "stage-file", label: "Stage/unstage file (s)", description: "Stage or unstage selected file"})
	addItem(paletteItem{id: "commit-staged", label: "Commit staged (c)", description: "Commit staged changes"})
	addItem(paletteItem{id: "commit-all", label: "Stage all and commit (C)", description: "Stage all changes and commit"})
	addItem(paletteItem{id: "edit-file", label: "Edit file (e)", description: "Open selected file in editor"})
	addItem(paletteItem{id: "delete-file", label: "Delete file (D)", description: "Delete selected file or directory"})

	// Section: Log Pane
	items = append(items, paletteItem{label: "Log Pane", isSection: true})
	addItem(paletteItem{id: "cherry-pick", label: "Cherry-pick commit (C)", description: "Cherry-pick commit to another worktree"})
	addItem(paletteItem{id: "commit-view", label: "Browse commit files", description: "Browse files changed in selected commit"})

	// Section: Navigation
	items = append(items, paletteItem{label: "Navigation", isSection: true})
	addItem(paletteItem{id: "zoom-toggle", label: "Toggle zoom (=)", description: "Toggle zoom on focused pane"})
	addItem(paletteItem{id: "filter", label: "Filter (f)", description: "Filter items in focused pane"})
	addItem(paletteItem{id: "search", label: "Search (/)", description: "Search items in focused pane"})
	addItem(paletteItem{id: "focus-worktrees", label: "Focus worktrees (1)", description: "Focus worktree pane"})
	addItem(paletteItem{id: "focus-status", label: "Focus status (2)", description: "Focus status pane"})
	addItem(paletteItem{id: "focus-log", label: "Focus log (3)", description: "Focus log pane"})
	addItem(paletteItem{id: "sort-cycle", label: "Cycle sort (s)", description: "Cycle sort mode (path/active/switched)"})

	// Section: Settings
	items = append(items, paletteItem{label: "Settings", isSection: true})
	addItem(paletteItem{id: "theme", label: "Select theme", description: "Change the application theme with live preview"})
	addItem(paletteItem{id: "help", label: "Help (?)", description: "Show help"})

	// Add custom items (filter out MRU duplicates)
	for _, item := range customItems {
		if item.id == "" || !mruIDs[item.id] {
			items = append(items, item)
		}
	}

	m.paletteScreen = NewCommandPaletteScreen(items, m.windowWidth, m.windowHeight, m.theme)
	m.paletteSubmit = func(action string) tea.Cmd {
		m.debugf("palette action: %s", action)

		// Track usage for MRU
		m.addToPaletteHistory(action)

		// Handle tmux active session attachment
		if after, ok := strings.CutPrefix(action, "tmux-attach:"); ok {
			sessionName := after
			insideTmux := os.Getenv("TMUX") != ""
			// Use worktree prefix when attaching (sessions are stored with prefix)
			fullSessionName := m.config.SessionPrefix + sessionName
			return m.attachTmuxSessionCmd(fullSessionName, insideTmux)
		}

		// Handle zellij active session attachment
		if after, ok := strings.CutPrefix(action, "zellij-attach:"); ok {
			sessionName := after
			// Use worktree prefix when attaching (sessions are stored with prefix)
			fullSessionName := m.config.SessionPrefix + sessionName
			return m.attachZellijSessionCmd(fullSessionName)
		}

		if _, ok := m.config.CustomCommands[action]; ok {
			return m.executeCustomCommand(action)
		}
		switch action {
		// Worktree Actions
		case "create":
			return m.showCreateWorktree()
		case "delete":
			return m.showDeleteWorktree()
		case "rename":
			return m.showRenameWorktree()
		case "absorb":
			return m.showAbsorbWorktree()
		case "prune":
			return m.showPruneMerged()

		// Create Menu Shortcuts
		case "create-from-current":
			return m.showCreateFromCurrent()
		case "create-from-branch":
			defaultBase := m.git.GetMainBranch(m.ctx)
			return m.showBranchSelection(
				"Select base branch",
				"Filter branches...",
				"No branches found.",
				defaultBase,
				func(branch string) tea.Cmd {
					suggestedName := stripRemotePrefix(branch)
					return m.showBranchNameInput(branch, suggestedName)
				},
			)
		case "create-from-commit":
			defaultBase := m.git.GetMainBranch(m.ctx)
			return m.showCommitSelection(defaultBase)
		case "create-from-pr":
			return m.showCreateFromPR()
		case "create-from-issue":
			return m.showCreateFromIssue()
		case "create-freeform":
			defaultBase := m.git.GetMainBranch(m.ctx)
			return m.showFreeformBaseInput(defaultBase)

		// Git Operations
		case "diff":
			return m.showDiff()
		case "refresh":
			return m.refreshWorktrees()
		case "fetch":
			return m.fetchRemotes()
		case "push":
			return m.pushToUpstream()
		case "sync":
			return m.syncWithUpstream()
		case "fetch-pr-data":
			m.ciCache = make(map[string]*ciCacheEntry)
			m.prDataLoaded = false
			m.updateTable()
			m.updateTableColumns(m.worktreeTable.Width())
			m.loading = true
			m.statusContent = "Fetching PR data..."
			m.loadingScreen = NewLoadingScreen("Fetching PR data...", m.theme, m.config.IconsEnabled())
			m.currentScreen = screenLoading
			return m.fetchPRData()
		case "pr":
			return m.openPR()
		case "ci-checks":
			return m.openCICheckSelection()
		case "lazygit":
			return m.openLazyGit()
		case "run-command":
			return m.showRunCommand()

		// Status Pane Actions
		case "stage-file":
			if len(m.statusTreeFlat) > 0 && m.statusTreeIndex >= 0 && m.statusTreeIndex < len(m.statusTreeFlat) {
				node := m.statusTreeFlat[m.statusTreeIndex]
				if node.IsDir() {
					return m.stageDirectory(node)
				}
				return m.stageCurrentFile(*node.File)
			}
			return nil
		case "commit-staged":
			return m.commitStagedChanges()
		case "commit-all":
			return m.commitAllChanges()
		case "edit-file":
			if len(m.statusTreeFlat) > 0 && m.statusTreeIndex >= 0 && m.statusTreeIndex < len(m.statusTreeFlat) {
				node := m.statusTreeFlat[m.statusTreeIndex]
				if !node.IsDir() {
					return m.openStatusFileInEditor(*node.File)
				}
			}
			return nil
		case "delete-file":
			return m.showDeleteFile()

		// Log Pane Actions
		case "cherry-pick":
			return m.showCherryPick()
		case "commit-view":
			return m.openCommitView()

		// Navigation & View
		case "zoom-toggle":
			if m.zoomedPane >= 0 {
				m.zoomedPane = -1
			} else {
				m.zoomedPane = m.focusedPane
			}
			return nil
		case "filter":
			target := filterTargetWorktrees
			switch m.focusedPane {
			case 1:
				target = filterTargetStatus
			case 2:
				target = filterTargetLog
			}
			return m.startFilter(target)
		case "search":
			target := searchTargetWorktrees
			switch m.focusedPane {
			case 1:
				target = searchTargetStatus
			case 2:
				target = searchTargetLog
			}
			return m.startSearch(target)
		case "focus-worktrees":
			m.zoomedPane = -1
			m.focusedPane = 0
			m.worktreeTable.Focus()
			return nil
		case "focus-status":
			m.zoomedPane = -1
			m.focusedPane = 1
			m.rebuildStatusContentWithHighlight()
			return nil
		case "focus-log":
			m.zoomedPane = -1
			m.focusedPane = 2
			m.logTable.Focus()
			return nil
		case "sort-cycle":
			m.sortMode = (m.sortMode + 1) % 3
			m.updateTable()
			return nil

		// Settings & Help
		case "theme":
			return m.showThemeSelection()
		case "help":
			m.currentScreen = screenHelp
			return nil
		}
		return nil
	}
	m.currentScreen = screenPalette
	return textinput.Blink
}

func (m *Model) showThemeSelection() tea.Cmd {
	m.originalTheme = m.config.Theme
	themes := theme.AvailableThemesWithCustoms(config.CustomThemesToThemeDataMap(m.config.CustomThemes))
	sort.Strings(themes)
	items := make([]selectionItem, 0, len(themes))
	for _, t := range themes {
		items = append(items, selectionItem{id: t, label: t})
	}
	m.listScreen = NewListSelectionScreen(items, labelWithIcon(UIIconThemeSelect, "Select Theme", m.config.IconsEnabled()), "Filter themes...", "", m.windowWidth, m.windowHeight, m.originalTheme, m.theme)
	m.listScreen.onCursorChange = func(item selectionItem) {
		m.UpdateTheme(item.id)
	}
	m.listSubmit = func(item selectionItem) tea.Cmd {
		m.listScreen = nil
		m.listSubmit = nil

		// Ask for confirmation before saving to config
		m.confirmScreen = NewConfirmScreen(fmt.Sprintf("Save theme '%s' to config file?", item.id), m.theme)
		m.confirmAction = func() tea.Cmd {
			m.config.Theme = item.id
			if err := config.SaveConfig(m.config); err != nil {
				m.debugf("failed to save config: %v", err)
			}
			m.originalTheme = ""
			return nil
		}
		m.currentScreen = screenConfirm
		return nil
	}
	m.currentScreen = screenListSelect
	return textinput.Blink
}

func (m *Model) UpdateTheme(themeName string) {
	thm := theme.GetThemeWithCustoms(themeName, config.CustomThemesToThemeDataMap(m.config.CustomThemes))
	m.theme = thm

	// Update table styles
	s := table.DefaultStyles()
	s.Selected = s.Selected.Foreground(thm.AccentFg).Background(thm.Accent)
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(thm.BorderDim).
		BorderBottom(true).
		Bold(true).
		Foreground(thm.Cyan).
		Background(thm.AccentDim)
	s.Selected = s.Selected.Bold(true) // Arrow indicator shows selection, no background

	m.worktreeTable.SetStyles(s)
	m.logTable.SetStyles(s)

	// Update spinner style
	m.spinner.Style = lipgloss.NewStyle().Foreground(thm.Accent)

	// Update filter input styles
	m.filterInput.PromptStyle = lipgloss.NewStyle().Foreground(thm.Accent)
	m.filterInput.TextStyle = lipgloss.NewStyle().Foreground(thm.TextFg)

	// Update other screens if they exist
	if m.helpScreen != nil {
		m.helpScreen.thm = thm
	}
	if m.confirmScreen != nil {
		m.confirmScreen.thm = thm
	}
	if m.infoScreen != nil {
		m.infoScreen.thm = thm
	}
	if m.loadingScreen != nil {
		m.loadingScreen.thm = thm
	}
	if m.inputScreen != nil {
		m.inputScreen.thm = thm
	}
	if m.paletteScreen != nil {
		m.paletteScreen.thm = thm
	}
	if m.prSelectionScreen != nil {
		m.prSelectionScreen.thm = thm
	}
	if m.issueSelectionScreen != nil {
		m.issueSelectionScreen.thm = thm
	}
	if m.listScreen != nil {
		m.listScreen.thm = thm
	}
	if m.commitFilesScreen != nil {
		m.commitFilesScreen.thm = thm
	}

	// Re-render info content with new theme
	if m.selectedIndex >= 0 && m.selectedIndex < len(m.filteredWts) {
		m.infoContent = m.buildInfoContent(m.filteredWts[m.selectedIndex])
	}
}

func (m *Model) showRunCommand() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}

	m.currentScreen = screenInput
	m.inputScreen = NewInputScreen(
		"Run command in worktree",
		"e.g., make test, npm install, etc.",
		"",
		m.theme,
		m.config.IconsEnabled(),
	)
	// Enable bash-style history navigation with up/down arrows
	// Always set history, even if empty - it will populate as commands are added
	m.inputScreen.SetHistory(m.commandHistory)
	m.inputSubmit = func(value string, checked bool) (tea.Cmd, bool) {
		cmdStr := strings.TrimSpace(value)
		if cmdStr == "" {
			return nil, true // Close without running
		}
		// Add command to history
		m.addToCommandHistory(cmdStr)
		return m.executeArbitraryCommand(cmdStr), true
	}
	return nil
}

func (m *Model) customFooterHints() []string {
	keys := m.customCommandKeys()
	if len(keys) == 0 {
		return nil
	}

	hints := make([]string, 0, len(keys))
	for _, key := range keys {
		cmd := m.config.CustomCommands[key]
		if cmd == nil || !cmd.ShowHelp {
			continue
		}
		label := strings.TrimSpace(cmd.Description)
		if label == "" {
			label = strings.TrimSpace(cmd.Command)
		}
		if label == "" {
			label = customCommandPlaceholder
		}
		hints = append(hints, m.renderKeyHint(key, label))
	}
	return hints
}

func (m *Model) showCherryPick() tea.Cmd {
	// Validate: log pane must be focused
	if m.focusedPane != 2 {
		return nil
	}

	// Validate: commit must be selected
	if len(m.logEntries) == 0 {
		return nil
	}

	cursor := m.logTable.Cursor()
	if cursor < 0 || cursor >= len(m.logEntries) {
		return nil
	}

	// Get source worktree and commit
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	sourceWorktree := m.filteredWts[m.selectedIndex]
	selectedCommit := m.logEntries[cursor]

	// Build worktree selection items (exclude source worktree)
	items := make([]selectionItem, 0, len(m.worktrees)-1)
	for _, wt := range m.worktrees {
		if wt.Path == sourceWorktree.Path {
			continue // Skip source worktree
		}

		name := filepath.Base(wt.Path)
		if wt.IsMain {
			name = "main"
		}

		desc := wt.Branch
		if wt.Dirty {
			desc += " (has changes)"
		}

		items = append(items, selectionItem{
			id:          wt.Path,
			label:       name,
			description: desc,
		})
	}

	// Check if no other worktrees available
	if len(items) == 0 {
		m.showInfo("No other worktrees available for cherry-pick.", nil)
		return nil
	}

	// Show worktree selection screen
	title := fmt.Sprintf("Cherry-pick %s to worktree", selectedCommit.sha)
	m.listScreen = NewListSelectionScreen(items, title, filterWorktreesPlaceholder, "No worktrees found.", m.windowWidth, m.windowHeight, "", m.theme)
	m.listSubmit = func(item selectionItem) tea.Cmd {
		// Find target worktree by path
		var targetWorktree *models.WorktreeInfo
		for _, wt := range m.worktrees {
			if wt.Path == item.id {
				targetWorktree = wt
				break
			}
		}

		if targetWorktree == nil {
			return func() tea.Msg {
				return errMsg{err: fmt.Errorf("target worktree not found")}
			}
		}

		// Clear list selection
		m.listScreen = nil
		m.listSubmit = nil
		m.currentScreen = screenNone

		// Execute cherry-pick
		return m.executeCherryPick(selectedCommit.sha, targetWorktree)
	}

	m.currentScreen = screenListSelect
	return textinput.Blink
}

func (m *Model) showCommitFilesScreen(commitSHA, worktreePath string) tea.Cmd {
	return func() tea.Msg {
		files, err := m.git.GetCommitFiles(m.ctx, commitSHA, worktreePath)
		if err != nil {
			return errMsg{err: err}
		}
		// Fetch commit metadata
		metaRaw := m.git.RunGit(
			m.ctx,
			[]string{
				"git", "log", "-1",
				"--pretty=format:%H%x1f%an%x1f%ae%x1f%ad%x1f%s%x1f%b",
				commitSHA,
			},
			worktreePath,
			[]int{0},
			true,
			false,
		)
		meta := parseCommitMeta(metaRaw)
		// Ensure SHA is set even if parsing fails
		if meta.sha == "" {
			meta.sha = commitSHA
		}
		return commitFilesLoadedMsg{
			sha:          commitSHA,
			worktreePath: worktreePath,
			files:        files,
			meta:         meta,
		}
	}
}

func (m *Model) openCommitView() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	if len(m.logEntries) == 0 {
		return nil
	}

	cursor := m.logTable.Cursor()
	if cursor < 0 || cursor >= len(m.logEntries) {
		return nil
	}
	entry := m.logEntries[cursor]
	wt := m.filteredWts[m.selectedIndex]

	return m.showCommitFilesScreen(entry.sha, wt.Path)
}

func (m *Model) persistCurrentSelection() {
	idx := m.selectedIndex
	if idx < 0 || idx >= len(m.filteredWts) {
		idx = m.worktreeTable.Cursor()
	}
	if idx < 0 || idx >= len(m.filteredWts) {
		return
	}
	m.persistLastSelected(m.filteredWts[idx].Path)
}

func (m *Model) persistLastSelected(path string) {
	if strings.TrimSpace(path) == "" {
		return
	}
	m.debugf("persist last-selected: %s", path)
	repoKey := m.getRepoKey()
	lastSelectedPath := filepath.Join(m.getWorktreeDir(), repoKey, models.LastSelectedFilename)
	if err := os.MkdirAll(filepath.Dir(lastSelectedPath), defaultDirPerms); err != nil {
		return
	}
	_ = os.WriteFile(lastSelectedPath, []byte(path+"\n"), defaultFilePerms)
	m.recordAccess(path)
}

func (m *Model) buildMRUPaletteItems() []paletteItem {
	if !m.config.PaletteMRU || len(m.paletteHistory) == 0 {
		return nil
	}

	// Build a lookup map of all available palette items
	itemMap := make(map[string]paletteItem)
	customItems := m.customPaletteItems()

	// Add all standard palette items
	standardItems := []paletteItem{
		// Worktree Actions
		{id: "create", label: "Create worktree (c)", description: "Add a new worktree from base branch or PR/MR"},
		{id: "delete", label: "Delete worktree (D)", description: "Remove worktree and branch"},
		{id: "rename", label: "Rename worktree (m)", description: "Rename worktree and branch"},
		{id: "absorb", label: "Absorb worktree (A)", description: "Merge branch into main and remove worktree"},
		{id: "prune", label: "Prune merged (X)", description: "Remove merged PR worktrees"},

		// Create Shortcuts
		{id: "create-from-current", label: "Create worktree from current branch", description: "Create from current branch with or without changes"},
		{id: "create-from-branch", label: "Create worktree from branch/tag", description: "Select a branch, tag, or remote as base"},
		{id: "create-from-commit", label: "Create worktree from commit", description: "Choose a branch, then select a specific commit"},
		{id: "create-from-pr", label: "Create worktree from PR/MR", description: "Create from a pull/merge request"},
		{id: "create-from-issue", label: "Create worktree from issue", description: "Create from a GitHub/GitLab issue"},
		{id: "create-freeform", label: "Create worktree from ref", description: "Enter a branch, tag, or commit manually"},

		// Git Operations
		{id: "diff", label: "Show diff (d)", description: "Show diff for current worktree or commit"},
		{id: "refresh", label: "Refresh (r)", description: "Reload worktrees"},
		{id: "fetch", label: "Fetch remotes (R)", description: "git fetch --all"},
		{id: "push", label: "Push to upstream (P)", description: "git push (clean worktree only)"},
		{id: "sync", label: "Synchronise with upstream (S)", description: "git pull, then git push (clean worktree only)"},
		{id: "fetch-pr-data", label: "Fetch PR data (p)", description: "Fetch PR/MR status from GitHub/GitLab"},
		{id: "ci-checks", label: "View CI checks (v)", description: "View CI check logs for current worktree"},
		{id: "pr", label: "Open PR (o)", description: "Open PR in browser"},
		{id: "lazygit", label: "Open LazyGit (g)", description: "Open LazyGit in selected worktree"},
		{id: "run-command", label: "Run command (!)", description: "Run arbitrary command in worktree"},

		// Status Pane
		{id: "stage-file", label: "Stage/unstage file (s)", description: "Stage or unstage selected file"},
		{id: "commit-staged", label: "Commit staged (c)", description: "Commit staged changes"},
		{id: "commit-all", label: "Stage all and commit (C)", description: "Stage all changes and commit"},
		{id: "edit-file", label: "Edit file (e)", description: "Open selected file in editor"},
		{id: "delete-file", label: "Delete file (D)", description: "Delete selected file or directory"},

		// Log Pane
		{id: "cherry-pick", label: "Cherry-pick commit (C)", description: "Cherry-pick commit to another worktree"},
		{id: "commit-view", label: "Browse commit files", description: "Browse files changed in selected commit"},

		// Navigation
		{id: "zoom-toggle", label: "Toggle zoom (=)", description: "Toggle zoom on focused pane"},
		{id: "filter", label: "Filter (f)", description: "Filter items in focused pane"},
		{id: "search", label: "Search (/)", description: "Search items in focused pane"},
		{id: "focus-worktrees", label: "Focus worktrees (1)", description: "Focus worktree pane"},
		{id: "focus-status", label: "Focus status (2)", description: "Focus status pane"},
		{id: "focus-log", label: "Focus log (3)", description: "Focus log pane"},
		{id: "sort-cycle", label: "Cycle sort (s)", description: "Cycle sort mode (path/active/switched)"},

		// Settings
		{id: "theme", label: "Select theme", description: "Change the application theme with live preview"},
		{id: "help", label: "Help (?)", description: "Show help"},
	}

	for _, item := range standardItems {
		if item.id != "" {
			itemMap[item.id] = item
		}
	}

	// Add custom items to the map
	for _, item := range customItems {
		if item.id != "" && !item.isSection {
			itemMap[item.id] = item
		}
	}

	// Build MRU list from history
	mruItems := make([]paletteItem, 0, m.config.PaletteMRULimit)
	for _, usage := range m.paletteHistory {
		if len(mruItems) >= m.config.PaletteMRULimit {
			break
		}

		// Look up the item details
		if item, exists := itemMap[usage.ID]; exists {
			// Mark as MRU and add to list
			item.isMRU = true
			mruItems = append(mruItems, item)
		}
	}

	return mruItems
}

func (m *Model) customPaletteItems() []paletteItem {
	keys := m.customCommandKeys()
	if len(keys) == 0 {
		return nil
	}

	// Separate commands into categories
	var regularItems, tmuxItems, zellijItems []paletteItem
	for _, key := range keys {
		cmd := m.config.CustomCommands[key]
		if cmd == nil {
			continue
		}
		label := m.customCommandLabel(cmd, key)
		description := customCommandPlaceholder
		switch {
		case cmd.Command != "":
			description = cmd.Command
		case cmd.Zellij != nil:
			description = zellijSessionLabel
		case cmd.Tmux != nil:
			description = tmuxSessionLabel
		}
		item := paletteItem{
			id:          key,
			label:       label,
			description: description,
		}
		switch {
		case cmd.Tmux != nil:
			tmuxItems = append(tmuxItems, item)
		case cmd.Zellij != nil:
			zellijItems = append(zellijItems, item)
		default:
			regularItems = append(regularItems, item)
		}
	}

	// Check if tmux/zellij are available
	_, tmuxErr := exec.LookPath("tmux")
	_, zellijErr := exec.LookPath("zellij")
	hasTmux := len(tmuxItems) > 0 && tmuxErr == nil
	hasZellij := len(zellijItems) > 0 && zellijErr == nil

	// Get active tmux sessions
	var activeTmuxSessions []paletteItem
	if tmuxErr == nil {
		sessions := m.getTmuxActiveSessions()
		for _, sessionName := range sessions {
			activeTmuxSessions = append(activeTmuxSessions, paletteItem{
				id:          "tmux-attach:" + sessionName,
				label:       sessionName,
				description: "active tmux session",
			})
		}
	}

	// Get active zellij sessions
	var activeZellijSessions []paletteItem
	if zellijErr == nil {
		sessions := m.getZellijActiveSessions()
		for _, sessionName := range sessions {
			activeZellijSessions = append(activeZellijSessions, paletteItem{
				id:          "zellij-attach:" + sessionName,
				label:       sessionName,
				description: "active zellij session",
			})
		}
	}

	// Build result with sections
	var items []paletteItem
	if len(regularItems) > 0 {
		items = append(items, paletteItem{label: "Custom Commands", isSection: true})
		items = append(items, regularItems...)
	}

	// Multiplexer section for custom tmux/zellij commands
	if hasTmux || hasZellij {
		items = append(items, paletteItem{label: "Multiplexer", isSection: true})
		if hasTmux {
			items = append(items, tmuxItems...)
		}
		if hasZellij {
			items = append(items, zellijItems...)
		}
	}

	// Active Tmux Sessions section (appears after Multiplexer)
	if len(activeTmuxSessions) > 0 {
		items = append(items, paletteItem{label: "Active Tmux Sessions", isSection: true})
		items = append(items, activeTmuxSessions...)
	}

	// Active Zellij Sessions section (appears after Active Tmux Sessions)
	if len(activeZellijSessions) > 0 {
		items = append(items, paletteItem{label: "Active Zellij Sessions", isSection: true})
		items = append(items, activeZellijSessions...)
	}

	return items
}
