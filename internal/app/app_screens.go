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
	"github.com/chmouel/lazyworktree/internal/app/commands"
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
			m.helpScreen = NewHelpScreen(m.view.WindowWidth, m.view.WindowHeight, m.config.CustomCommands, m.theme, m.config.IconsEnabled())
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
			if m.pending.TrustPath != "" {
				_ = m.trustManager.TrustFile(m.pending.TrustPath)
			}
			cmd := m.runCommands(m.pending.Commands, m.pending.CommandCwd, m.pending.CommandEnv, m.pending.After)
			m.clearPendingTrust()
			m.currentScreen = screenNone
			return m, cmd
		case keyStr == "b" || keyStr == "B":
			after := m.pending.After
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
	registry := commands.NewRegistry()
	m.registerPaletteActions(registry)

	m.debugf("palette MRU: enabled=%v, history_len=%d", m.config.PaletteMRU, len(m.paletteHistory))
	paletteItems := commands.BuildPaletteItems(commands.PaletteOptions{
		MRUEnabled:  m.config.PaletteMRU,
		MRULimit:    m.config.PaletteMRULimit,
		History:     m.paletteHistory,
		Actions:     registry.Actions(),
		CustomItems: customItems,
	})

	mruCount := 0
	for _, item := range paletteItems {
		if item.IsMRU {
			mruCount++
		}
	}
	m.debugf("palette MRU: built %d items", mruCount)

	items := toPaletteItems(paletteItems)
	m.paletteScreen = NewCommandPaletteScreen(items, m.view.WindowWidth, m.view.WindowHeight, m.theme)
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
		return registry.Execute(action)
	}
	m.currentScreen = screenPalette
	return textinput.Blink
}

func (m *Model) registerPaletteActions(registry *commands.Registry) {
	commands.RegisterWorktreeActions(registry, commands.WorktreeHandlers{
		Create:            m.showCreateWorktree,
		Delete:            m.showDeleteWorktree,
		Rename:            m.showRenameWorktree,
		Absorb:            m.showAbsorbWorktree,
		Prune:             m.showPruneMerged,
		CreateFromCurrent: m.showCreateFromCurrent,
		CreateFromBranch: func() tea.Cmd {
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
		},
		CreateFromCommit: func() tea.Cmd {
			defaultBase := m.git.GetMainBranch(m.ctx)
			return m.showCommitSelection(defaultBase)
		},
		CreateFromPR:    m.showCreateFromPR,
		CreateFromIssue: m.showCreateFromIssue,
		CreateFreeform: func() tea.Cmd {
			defaultBase := m.git.GetMainBranch(m.ctx)
			return m.showFreeformBaseInput(defaultBase)
		},
	})

	commands.RegisterGitOperations(registry, commands.GitHandlers{
		ShowDiff:    m.showDiff,
		Refresh:     m.refreshWorktrees,
		Fetch:       m.fetchRemotes,
		Push:        m.pushToUpstream,
		Sync:        m.syncWithUpstream,
		FetchPRData: m.fetchPRDataWithState,
		ViewCIChecks: func() tea.Cmd {
			return m.openCICheckSelection()
		},
		CIChecksAvailable: func() bool {
			return m.git != nil && m.git.IsGitHub(m.ctx)
		},
		OpenPR:      m.openPR,
		OpenLazyGit: m.openLazyGit,
		RunCommand:  m.showRunCommand,
	})

	commands.RegisterStatusPaneActions(registry, commands.StatusHandlers{
		StageFile: func() tea.Cmd {
			if len(m.statusTreeFlat) > 0 && m.statusTreeIndex >= 0 && m.statusTreeIndex < len(m.statusTreeFlat) {
				node := m.statusTreeFlat[m.statusTreeIndex]
				if node.IsDir() {
					return m.stageDirectory(node)
				}
				return m.stageCurrentFile(*node.File)
			}
			return nil
		},
		CommitStaged: m.commitStagedChanges,
		CommitAll:    m.commitAllChanges,
		EditFile: func() tea.Cmd {
			if len(m.statusTreeFlat) > 0 && m.statusTreeIndex >= 0 && m.statusTreeIndex < len(m.statusTreeFlat) {
				node := m.statusTreeFlat[m.statusTreeIndex]
				if !node.IsDir() {
					return m.openStatusFileInEditor(*node.File)
				}
			}
			return nil
		},
		DeleteFile: m.showDeleteFile,
	})

	commands.RegisterLogPaneActions(registry, commands.LogHandlers{
		CherryPick: m.showCherryPick,
		CommitView: m.openCommitView,
	})

	commands.RegisterNavigationActions(registry, commands.NavigationHandlers{
		ToggleZoom: func() tea.Cmd {
			if m.view.ZoomedPane >= 0 {
				m.view.ZoomedPane = -1
			} else {
				m.view.ZoomedPane = m.view.FocusedPane
			}
			return nil
		},
		Filter: func() tea.Cmd {
			target := filterTargetWorktrees
			switch m.view.FocusedPane {
			case 1:
				target = filterTargetStatus
			case 2:
				target = filterTargetLog
			}
			return m.startFilter(target)
		},
		Search: func() tea.Cmd {
			target := searchTargetWorktrees
			switch m.view.FocusedPane {
			case 1:
				target = searchTargetStatus
			case 2:
				target = searchTargetLog
			}
			return m.startSearch(target)
		},
		FocusWorktree: func() tea.Cmd {
			m.view.ZoomedPane = -1
			m.view.FocusedPane = 0
			m.worktreeTable.Focus()
			return nil
		},
		FocusStatus: func() tea.Cmd {
			m.view.ZoomedPane = -1
			m.view.FocusedPane = 1
			m.rebuildStatusContentWithHighlight()
			return nil
		},
		FocusLog: func() tea.Cmd {
			m.view.ZoomedPane = -1
			m.view.FocusedPane = 2
			m.logTable.Focus()
			return nil
		},
		SortCycle: func() tea.Cmd {
			m.sortMode = (m.sortMode + 1) % 3
			m.updateTable()
			return nil
		},
	})

	commands.RegisterSettingsActions(registry, commands.SettingsHandlers{
		Theme: m.showThemeSelection,
		Help: func() tea.Cmd {
			m.currentScreen = screenHelp
			return nil
		},
	})
}

func (m *Model) fetchPRDataWithState() tea.Cmd {
	m.ciCache = make(map[string]*ciCacheEntry)
	m.prDataLoaded = false
	m.updateTable()
	m.updateTableColumns(m.worktreeTable.Width())
	m.loading = true
	m.statusContent = "Fetching PR data..."
	m.loadingScreen = NewLoadingScreen("Fetching PR data...", m.theme, m.config.IconsEnabled())
	m.currentScreen = screenLoading
	return m.fetchPRData()
}

func toPaletteItems(items []commands.PaletteItem) []paletteItem {
	if len(items) == 0 {
		return nil
	}
	converted := make([]paletteItem, len(items))
	for i, item := range items {
		converted[i] = paletteItem{
			id:          item.ID,
			label:       item.Label,
			description: item.Description,
			isSection:   item.IsSection,
			isMRU:       item.IsMRU,
		}
	}
	return converted
}

func (m *Model) showThemeSelection() tea.Cmd {
	m.originalTheme = m.config.Theme
	themes := theme.AvailableThemesWithCustoms(config.CustomThemesToThemeDataMap(m.config.CustomThemes))
	sort.Strings(themes)
	items := make([]selectionItem, 0, len(themes))
	for _, t := range themes {
		items = append(items, selectionItem{id: t, label: t})
	}
	m.listScreen = NewListSelectionScreen(items, labelWithIcon(UIIconThemeSelect, "Select Theme", m.config.IconsEnabled()), "Filter themes...", "", m.view.WindowWidth, m.view.WindowHeight, m.originalTheme, m.theme)
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

// UpdateTheme refreshes UI styles for the selected theme.
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
	if m.view.FocusedPane != 2 {
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
	m.listScreen = NewListSelectionScreen(items, title, filterWorktreesPlaceholder, "No worktrees found.", m.view.WindowWidth, m.view.WindowHeight, "", m.theme)
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

func (m *Model) customPaletteItems() []commands.PaletteItem {
	keys := m.customCommandKeys()
	if len(keys) == 0 {
		return nil
	}

	// Separate commands into categories
	var regularItems, tmuxItems, zellijItems []commands.PaletteItem
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
		item := commands.PaletteItem{
			ID:          key,
			Label:       label,
			Description: description,
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
	var activeTmuxSessions []commands.PaletteItem
	if tmuxErr == nil {
		sessions := m.getTmuxActiveSessions()
		for _, sessionName := range sessions {
			activeTmuxSessions = append(activeTmuxSessions, commands.PaletteItem{
				ID:          "tmux-attach:" + sessionName,
				Label:       sessionName,
				Description: "active tmux session",
			})
		}
	}

	// Get active zellij sessions
	var activeZellijSessions []commands.PaletteItem
	if zellijErr == nil {
		sessions := m.getZellijActiveSessions()
		for _, sessionName := range sessions {
			activeZellijSessions = append(activeZellijSessions, commands.PaletteItem{
				ID:          "zellij-attach:" + sessionName,
				Label:       sessionName,
				Description: "active zellij session",
			})
		}
	}

	// Build result with sections
	var items []commands.PaletteItem
	if len(regularItems) > 0 {
		items = append(items, commands.PaletteItem{Label: "Custom Commands", IsSection: true})
		items = append(items, regularItems...)
	}

	// Multiplexer section for custom tmux/zellij commands
	if hasTmux || hasZellij {
		items = append(items, commands.PaletteItem{Label: "Multiplexer", IsSection: true})
		if hasTmux {
			items = append(items, tmuxItems...)
		}
		if hasZellij {
			items = append(items, zellijItems...)
		}
	}

	// Active Tmux Sessions section (appears after Multiplexer)
	if len(activeTmuxSessions) > 0 {
		items = append(items, commands.PaletteItem{Label: "Active Tmux Sessions", IsSection: true})
		items = append(items, activeTmuxSessions...)
	}

	// Active Zellij Sessions section (appears after Active Tmux Sessions)
	if len(activeZellijSessions) > 0 {
		items = append(items, commands.PaletteItem{Label: "Active Zellij Sessions", IsSection: true})
		items = append(items, activeZellijSessions...)
	}

	return items
}
