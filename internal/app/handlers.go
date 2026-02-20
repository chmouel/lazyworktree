package app

import (
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/app/state"
	"github.com/chmouel/lazyworktree/internal/models"
)

// handleKeyMsg processes keyboard input when not in a modal screen.
func (m *Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	if m.state.view.ShowingSearch {
		return m.handleSearchInput(msg)
	}

	// Handle filter input first - when filtering, only escape/enter should exit
	if m.state.view.ShowingFilter {
		keyStr := msg.String()
		switch m.state.view.FilterTarget {
		case filterTargetWorktrees:
			if keyStr == keyEnter {
				m.state.view.ShowingFilter = false
				m.state.ui.filterInput.Blur()
				m.restoreFocusAfterFilter()
				return m, nil
			}
			if isEscKey(keyStr) || keyStr == keyCtrlC {
				m.state.view.ShowingFilter = false
				m.state.ui.filterInput.Blur()
				m.state.ui.worktreeTable.Focus()
				return m, nil
			}
			if keyStr == "alt+n" || keyStr == "alt+p" {
				return m.handleFilterNavigation(keyStr, true)
			}
			if keyStr == keyUp || keyStr == keyDown || keyStr == keyCtrlK || keyStr == keyCtrlJ {
				return m.handleFilterNavigation(keyStr, false)
			}
			m.state.ui.filterInput, cmd = m.state.ui.filterInput.Update(msg)
			m.setFilterQuery(filterTargetWorktrees, m.state.ui.filterInput.Value())
			m.updateTable()
			return m, cmd
		case filterTargetStatus:
			if keyStr == keyEnter {
				m.state.view.ShowingFilter = false
				m.state.ui.filterInput.Blur()
				m.restoreFocusAfterFilter()
				return m, nil
			}
			if isEscKey(keyStr) || keyStr == keyCtrlC {
				m.state.view.ShowingFilter = false
				m.state.ui.filterInput.Blur()
				m.restoreFocusAfterFilter()
				return m, nil
			}
			m.state.ui.filterInput, cmd = m.state.ui.filterInput.Update(msg)
			m.setFilterQuery(filterTargetStatus, m.state.ui.filterInput.Value())
			m.applyStatusFilter()
			return m, cmd
		case filterTargetLog:
			if keyStr == keyEnter {
				m.state.view.ShowingFilter = false
				m.state.ui.filterInput.Blur()
				m.restoreFocusAfterFilter()
				return m, nil
			}
			if isEscKey(keyStr) || keyStr == keyCtrlC {
				m.state.view.ShowingFilter = false
				m.state.ui.filterInput.Blur()
				m.restoreFocusAfterFilter()
				return m, nil
			}
			m.state.ui.filterInput, cmd = m.state.ui.filterInput.Update(msg)
			m.setFilterQuery(filterTargetLog, m.state.ui.filterInput.Value())
			m.applyLogFilter(false)
			return m, cmd
		}
	}

	// Check for custom commands first - allows users to override built-in keys
	if _, ok := m.config.CustomCommands[msg.String()]; ok {
		return m, m.executeCustomCommand(msg.String())
	}

	return m.handleBuiltInKey(msg)
}

func (m *Model) handleSearchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	keyStr := msg.String()
	if keyStr == keyEnter {
		m.state.view.ShowingSearch = false
		m.state.ui.filterInput.Blur()
		m.restoreFocusAfterSearch()
		return m, nil
	}
	if isEscKey(keyStr) || keyStr == keyCtrlC {
		m.clearSearchQuery()
		m.state.view.ShowingSearch = false
		m.state.ui.filterInput.Blur()
		m.restoreFocusAfterSearch()
		return m, nil
	}

	var cmd tea.Cmd
	m.state.ui.filterInput, cmd = m.state.ui.filterInput.Update(msg)
	query := m.state.ui.filterInput.Value()
	m.setSearchQuery(m.state.view.SearchTarget, query)
	return m, tea.Batch(cmd, m.applySearchQuery(query))
}

func (m *Model) clearSearchQuery() {
	m.setSearchQuery(m.state.view.SearchTarget, "")
	m.state.ui.filterInput.SetValue("")
	m.state.ui.filterInput.CursorEnd()
}

func (m *Model) restoreFocusAfterSearch() {
	switch m.state.view.SearchTarget {
	case searchTargetWorktrees:
		m.state.ui.worktreeTable.Focus()
	case searchTargetLog:
		m.state.ui.logTable.Focus()
	}
}

func (m *Model) restoreFocusAfterFilter() {
	switch m.state.view.FilterTarget {
	case filterTargetWorktrees:
		m.state.ui.worktreeTable.Focus()
	case filterTargetLog:
		m.state.ui.logTable.Focus()
	}
}

func (m *Model) clearCurrentPaneFilter() (tea.Model, tea.Cmd) {
	switch m.state.view.FocusedPane {
	case 0:
		m.state.services.filter.FilterQuery = ""
		m.state.ui.filterInput.SetValue("")
		m.updateTable()
	case 1:
		m.state.services.filter.StatusFilterQuery = ""
		m.state.ui.filterInput.SetValue("")
		m.applyStatusFilter()
	case 2:
		m.state.services.filter.LogFilterQuery = ""
		m.state.ui.filterInput.SetValue("")
		m.applyLogFilter(false)
	}
	return m, nil
}

func (m *Model) handleGotoTop() (tea.Model, tea.Cmd) {
	switch m.state.view.FocusedPane {
	case 0:
		m.state.ui.worktreeTable.GotoTop()
		m.updateWorktreeArrows()
		m.syncSelectedIndexFromCursor()
		return m, m.debouncedUpdateDetailsView()
	case 1:
		if len(m.state.services.statusTree.TreeFlat) > 0 {
			m.state.services.statusTree.Index = 0
			m.rebuildStatusContentWithHighlight()
		}
	case 2:
		m.state.ui.logTable.GotoTop()
	}
	return m, nil
}

func (m *Model) handleGotoBottom() (tea.Model, tea.Cmd) {
	switch m.state.view.FocusedPane {
	case 0:
		m.state.ui.worktreeTable.GotoBottom()
		m.updateWorktreeArrows()
		m.syncSelectedIndexFromCursor()
		return m, m.debouncedUpdateDetailsView()
	case 1:
		if len(m.state.services.statusTree.TreeFlat) > 0 {
			m.state.services.statusTree.Index = len(m.state.services.statusTree.TreeFlat) - 1
			m.rebuildStatusContentWithHighlight()
		}
	case 2:
		m.state.ui.logTable.GotoBottom()
	}
	return m, nil
}

func (m *Model) handleNextFolder() (tea.Model, tea.Cmd) {
	if len(m.state.services.statusTree.TreeFlat) == 0 {
		return m, nil
	}
	// Find next directory after current position
	for i := m.state.services.statusTree.Index + 1; i < len(m.state.services.statusTree.TreeFlat); i++ {
		if m.state.services.statusTree.TreeFlat[i].IsDir() {
			m.state.services.statusTree.Index = i
			m.rebuildStatusContentWithHighlight()
			return m, nil
		}
	}
	return m, nil
}

func (m *Model) handlePrevFolder() (tea.Model, tea.Cmd) {
	if len(m.state.services.statusTree.TreeFlat) == 0 {
		return m, nil
	}
	// Find previous directory before current position
	for i := m.state.services.statusTree.Index - 1; i >= 0; i-- {
		if m.state.services.statusTree.TreeFlat[i].IsDir() {
			m.state.services.statusTree.Index = i
			m.rebuildStatusContentWithHighlight()
			return m, nil
		}
	}
	return m, nil
}

// handleBuiltInKey processes built-in keyboard shortcuts.
func (m *Model) handleBuiltInKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case keyCtrlC, keyQ:
		if m.selectedPath != "" {
			m.stopGitWatcher()
			return m, tea.Quit
		}
		m.quitting = true
		m.stopGitWatcher()
		return m, tea.Quit

	case "1":
		targetPane := 0
		if m.state.view.FocusedPane == targetPane {
			// Already on this pane - toggle zoom
			if m.state.view.ZoomedPane >= 0 {
				m.state.view.ZoomedPane = -1 // unzoom
			} else {
				m.state.view.ZoomedPane = targetPane // zoom
			}
		} else {
			// Switching to different pane - exit zoom and switch
			m.state.view.ZoomedPane = -1
			wasPane1 := m.state.view.FocusedPane == 1
			if wasPane1 {
				m.ciCheckIndex = -1
			}
			m.state.view.FocusedPane = targetPane
			m.state.ui.worktreeTable.Focus()
			if wasPane1 {
				m.rebuildStatusContentWithHighlight()
			}
		}
		return m, nil

	case "2":
		targetPane := 1
		if m.state.view.FocusedPane == targetPane {
			// Already on this pane - toggle zoom
			if m.state.view.ZoomedPane >= 0 {
				m.state.view.ZoomedPane = -1 // unzoom
			} else {
				m.state.view.ZoomedPane = targetPane // zoom
			}
		} else {
			// Switching to different pane - exit zoom and switch
			m.state.view.ZoomedPane = -1
			m.state.view.FocusedPane = targetPane
		}
		// Always rebuild status for pane 1 (important for highlighting)
		m.rebuildStatusContentWithHighlight()
		return m, nil

	case "3":
		targetPane := 2
		if m.state.view.FocusedPane == targetPane {
			// Already on this pane - toggle zoom
			if m.state.view.ZoomedPane >= 0 {
				m.state.view.ZoomedPane = -1 // unzoom
			} else {
				m.state.view.ZoomedPane = targetPane // zoom
			}
		} else {
			// Switching to different pane - exit zoom and switch
			m.state.view.ZoomedPane = -1
			wasPane1 := m.state.view.FocusedPane == 1
			if wasPane1 {
				m.ciCheckIndex = -1
			}
			m.state.view.FocusedPane = targetPane
			m.state.ui.logTable.Focus()
			if wasPane1 {
				m.rebuildStatusContentWithHighlight()
			}
		}
		return m, nil

	case keyTab, "]":
		m.state.view.ZoomedPane = -1 // exit zoom mode
		wasPane1 := m.state.view.FocusedPane == 1
		m.state.view.FocusedPane = (m.state.view.FocusedPane + 1) % 3
		if wasPane1 && m.state.view.FocusedPane != 1 {
			m.ciCheckIndex = -1
		}
		switch m.state.view.FocusedPane {
		case 0:
			m.state.ui.worktreeTable.Focus()
		case 2:
			m.state.ui.logTable.Focus()
		}
		if wasPane1 || m.state.view.FocusedPane == 1 {
			m.rebuildStatusContentWithHighlight()
		}
		return m, nil

	case "[":
		m.state.view.ZoomedPane = -1 // exit zoom mode
		wasPane1 := m.state.view.FocusedPane == 1
		m.state.view.FocusedPane = (m.state.view.FocusedPane - 1 + 3) % 3
		if wasPane1 && m.state.view.FocusedPane != 1 {
			m.ciCheckIndex = -1
		}
		switch m.state.view.FocusedPane {
		case 0:
			m.state.ui.worktreeTable.Focus()
		case 2:
			m.state.ui.logTable.Focus()
		}
		if wasPane1 || m.state.view.FocusedPane == 1 {
			m.rebuildStatusContentWithHighlight()
		}
		return m, nil

	case "h":
		if m.state.view.Layout == state.LayoutTop {
			// Top layout: h navigates left among bottom panes, or up to top
			switch m.state.view.FocusedPane {
			case 1:
				m.state.view.ZoomedPane = -1
				m.state.view.FocusedPane = 0
				m.state.ui.worktreeTable.Focus()
				m.ciCheckIndex = -1
				m.rebuildStatusContentWithHighlight()
			case 2:
				m.state.view.ZoomedPane = -1
				m.state.view.FocusedPane = 1
				m.rebuildStatusContentWithHighlight()
			}
		} else if m.state.view.FocusedPane != 0 {
			// Default layout: navigate left - go to pane 0 if not already there
			m.state.view.ZoomedPane = -1
			wasPane1 := m.state.view.FocusedPane == 1
			m.state.view.FocusedPane = 0
			m.state.ui.worktreeTable.Focus()
			if wasPane1 {
				m.ciCheckIndex = -1
				m.rebuildStatusContentWithHighlight()
			}
		}
		return m, nil

	case "l":
		if m.state.view.Layout == state.LayoutTop {
			// Top layout: l navigates right among panes
			switch m.state.view.FocusedPane {
			case 0:
				m.state.view.ZoomedPane = -1
				m.state.view.FocusedPane = 1
				m.rebuildStatusContentWithHighlight()
			case 1:
				m.state.view.ZoomedPane = -1
				m.state.view.FocusedPane = 2
				m.ciCheckIndex = -1
				m.state.ui.logTable.Focus()
			}
		} else {
			// Default layout: navigate right - cycle through right panels (1, 2)
			switch m.state.view.FocusedPane {
			case 0:
				m.state.view.ZoomedPane = -1
				m.state.view.FocusedPane = 1
				m.rebuildStatusContentWithHighlight()
			case 1:
				m.state.view.ZoomedPane = -1
				m.state.view.FocusedPane = 2
				m.ciCheckIndex = -1
				m.state.ui.logTable.Focus()
			default:
				m.state.view.ZoomedPane = -1
				m.state.view.FocusedPane = 1
				m.rebuildStatusContentWithHighlight()
			}
		}
		return m, nil

	case "j", "down":
		return m.handleNavigationDown(msg)

	case "k", "up":
		return m.handleNavigationUp(msg)

	case keyCtrlJ:
		if m.state.view.FocusedPane == 1 && len(m.state.services.statusTree.TreeFlat) > 0 {
			if m.state.services.statusTree.Index < len(m.state.services.statusTree.TreeFlat)-1 {
				m.state.services.statusTree.Index++
			}
			m.rebuildStatusContentWithHighlight()
			node := m.state.services.statusTree.TreeFlat[m.state.services.statusTree.Index]
			if !node.IsDir() {
				return m, m.showFileDiff(*node.File)
			}
			return m, nil
		}
		if m.state.view.FocusedPane == 2 {
			prevCursor := m.state.ui.logTable.Cursor()
			_, moveCmd := m.handleNavigationDown(tea.KeyMsg{Type: tea.KeyDown})
			if m.state.ui.logTable.Cursor() == prevCursor {
				return m, moveCmd
			}
			return m, tea.Batch(moveCmd, m.openCommitView())
		}
		return m, nil

	case keyCtrlK:
		if m.state.view.FocusedPane == 1 && len(m.state.services.statusTree.TreeFlat) > 0 {
			if m.state.services.statusTree.Index > 0 {
				m.state.services.statusTree.Index--
			}
			m.rebuildStatusContentWithHighlight()
			node := m.state.services.statusTree.TreeFlat[m.state.services.statusTree.Index]
			if !node.IsDir() {
				return m, m.showFileDiff(*node.File)
			}
			return m, nil
		}
		return m, nil

	case "ctrl+d", " ":
		return m.handlePageDown(msg)

	case "ctrl+u":
		return m.handlePageUp(msg)

	case "pgdown":
		return m.handlePageDown(msg)

	case "pgup":
		return m.handlePageUp(msg)

	case "G":
		if m.state.view.FocusedPane == 1 {
			m.state.ui.statusViewport.GotoBottom()
			if len(m.state.services.statusTree.TreeFlat) > 0 {
				m.state.services.statusTree.Index = len(m.state.services.statusTree.TreeFlat) - 1
			}
			return m, nil
		}
		return m, nil

	case keyEnter:
		return m.handleEnterKey()

	case "r":
		m.loading = true
		m.setLoadingScreen(loadingRefreshWorktrees)
		cmds := []tea.Cmd{m.refreshWorktrees()}

		// Also refresh PR/CI for current worktree if GitHub/GitLab (unless PR disabled)
		if !m.config.DisablePR && m.state.services.git.IsGitHubOrGitLab(m.ctx) {
			m.cache.ciCache.Clear()
			if cmd := m.refreshCurrentWorktreePR(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return m, tea.Batch(cmds...)

	case "c":
		if m.state.view.FocusedPane == 1 {
			return m, m.commitStagedChanges()
		}
		return m, m.showCreateWorktree()

	case "D":
		if m.state.view.FocusedPane == 1 {
			return m, m.showDeleteFile()
		}
		return m, m.showDeleteWorktree()

	case "d":
		// If in log pane (bottom right), show commit diff
		if m.state.view.FocusedPane == 2 {
			cursor := m.state.ui.logTable.Cursor()
			if len(m.state.data.logEntries) > 0 && cursor >= 0 && cursor < len(m.state.data.logEntries) {
				if m.state.data.selectedIndex >= 0 && m.state.data.selectedIndex < len(m.state.data.filteredWts) {
					commitSHA := m.state.data.logEntries[cursor].sha
					wt := m.state.data.filteredWts[m.state.data.selectedIndex]
					return m, m.showCommitDiff(commitSHA, wt)
				}
			}
			return m, nil
		}
		// Otherwise show worktree diff
		return m, m.showDiff()

	case "e":
		if m.state.view.FocusedPane == 1 && len(m.state.services.statusTree.TreeFlat) > 0 && m.state.services.statusTree.Index >= 0 && m.state.services.statusTree.Index < len(m.state.services.statusTree.TreeFlat) {
			node := m.state.services.statusTree.TreeFlat[m.state.services.statusTree.Index]
			if !node.IsDir() {
				return m, m.openStatusFileInEditor(*node.File)
			}
		}
		return m, nil

	case "v":
		// Open CI check selection from any pane
		return m, m.openCICheckSelection()

	case "ctrl+v":
		// View CI check logs in pager when a CI check is selected in status screen
		if m.state.view.FocusedPane == 1 {
			ciChecks, hasCIChecks := m.getCIChecksForCurrentWorktree()
			if hasCIChecks && m.ciCheckIndex >= 0 && m.ciCheckIndex < len(ciChecks) {
				check := ciChecks[m.ciCheckIndex]
				return m, m.showCICheckLog(check)
			}
		}
		return m, nil

	case "P":
		return m, m.pushToUpstream()

	case "S":
		return m, m.syncWithUpstream()

	case "R":
		m.loading = true
		m.statusContent = "Fetching remotes..."
		m.setLoadingScreen("Fetching remotes...")
		return m, m.fetchRemotes()

	case "f":
		target := filterTargetWorktrees
		switch m.state.view.FocusedPane {
		case 1:
			target = filterTargetStatus
		case 2:
			target = filterTargetLog
		}
		return m, m.startFilter(target)

	case "/":
		target := searchTargetWorktrees
		switch m.state.view.FocusedPane {
		case 1:
			target = searchTargetStatus
		case 2:
			target = searchTargetLog
		}
		return m, m.startSearch(target)

	case "n":
		return m, m.advanceSearchMatch(true)
	case "N":
		return m, m.advanceSearchMatch(false)

	case "s":
		// In status pane: stage/unstage selected file or directory
		if m.state.view.FocusedPane == 1 && len(m.state.services.statusTree.TreeFlat) > 0 && m.state.services.statusTree.Index >= 0 && m.state.services.statusTree.Index < len(m.state.services.statusTree.TreeFlat) {
			node := m.state.services.statusTree.TreeFlat[m.state.services.statusTree.Index]
			if node.IsDir() {
				return m, m.stageDirectory(node)
			}
			return m, m.stageCurrentFile(*node.File)
		}
		// Otherwise: cycle through sort modes: path -> active -> switched -> path
		m.sortMode = (m.sortMode + 1) % 3
		m.updateTable()
		return m, nil

	case "ctrl+p", ":":
		return m, m.showCommandPalette()

	case "?":
		helpScreen := appscreen.NewHelpScreen(m.state.view.WindowWidth, m.state.view.WindowHeight, m.config.CustomCommands, m.theme, m.config.IconsEnabled())
		m.state.ui.screenManager.Push(helpScreen)
		return m, nil

	case "g":
		return m, m.openLazyGit()

	case "o":
		return m, m.openPR()

	case "m":
		return m, m.showRenameWorktree()

	case "i":
		return m, m.showAnnotateWorktree()

	case "T":
		return m, m.showTaskboard()

	case "A":
		return m, m.showAbsorbWorktree()

	case "X":
		return m, m.showPruneMerged()

	case "!":
		return m, m.showRunCommand()

	case "C":
		if m.state.view.FocusedPane == 1 {
			return m, m.commitAllChanges()
		}
		return m, m.showCherryPick()

	case "L":
		if m.state.view.Layout == state.LayoutDefault {
			m.state.view.Layout = state.LayoutTop
		} else {
			m.state.view.Layout = state.LayoutDefault
		}
		m.state.view.ZoomedPane = -1
		return m, nil

	case "=":
		if m.state.view.ZoomedPane >= 0 {
			m.state.view.ZoomedPane = -1 // unzoom
		} else {
			m.state.view.ZoomedPane = m.state.view.FocusedPane // zoom current pane
		}
		return m, nil

	case keyEsc, keyEscRaw:
		if m.hasActiveFilterForPane(m.state.view.FocusedPane) {
			return m.clearCurrentPaneFilter()
		}
		return m, nil
	}

	// Handle Home/End keys for all panes
	if msg.Type == tea.KeyHome {
		return m.handleGotoTop()
	}
	if msg.Type == tea.KeyEnd {
		return m.handleGotoBottom()
	}

	// Handle Ctrl+Left/Right for folder navigation in status pane
	if m.state.view.FocusedPane == 1 {
		if msg.Type == tea.KeyCtrlLeft {
			return m.handlePrevFolder()
		}
		if msg.Type == tea.KeyCtrlRight {
			return m.handleNextFolder()
		}
	}

	// Handle table input
	if m.state.view.FocusedPane == 0 {
		var cmd tea.Cmd
		m.state.ui.worktreeTable, cmd = m.state.ui.worktreeTable.Update(msg)
		m.syncSelectedIndexFromCursor()
		return m, tea.Batch(cmd, m.debouncedUpdateDetailsView())
	}

	return m, nil
}

// handleNavigationDown processes down arrow and 'j' key navigation.
func (m *Model) handleNavigationDown(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	keyMsg := msg
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch m.state.view.FocusedPane {
	case 0:
		m.state.ui.worktreeTable, cmd = m.state.ui.worktreeTable.Update(keyMsg)
		m.updateWorktreeArrows()
		m.syncSelectedIndexFromCursor()
		cmds = append(cmds, cmd)
		cmds = append(cmds, m.debouncedUpdateDetailsView())
	case 1:
		// Check if we're navigating CI checks
		ciChecks, hasCIChecks := m.getCIChecksForCurrentWorktree()
		wasReset := false
		// Reset if CI checks are no longer available or index is out of bounds
		if !hasCIChecks || (hasCIChecks && m.ciCheckIndex >= len(ciChecks)) {
			if m.ciCheckIndex >= 0 {
				m.ciCheckIndex = -1
				m.infoContent = m.buildInfoContent(m.state.data.filteredWts[m.state.data.selectedIndex])
				wasReset = true
			}
		}

		switch {
		case len(m.state.services.statusTree.TreeFlat) == 0 && hasCIChecks && m.ciCheckIndex == -1 && !wasReset:
			// No commit changes but CI checks are available, navigate to first CI check
			m.ciCheckIndex = 0
			m.infoContent = m.buildInfoContent(m.state.data.filteredWts[m.state.data.selectedIndex])
			m.rebuildStatusContentWithHighlight()
		case hasCIChecks && m.ciCheckIndex >= 0:
			// Navigate CI checks
			if m.ciCheckIndex < len(ciChecks)-1 {
				m.ciCheckIndex++
				m.infoContent = m.buildInfoContent(m.state.data.filteredWts[m.state.data.selectedIndex])
				m.rebuildStatusContentWithHighlight()
			} else {
				// At last CI check, wrap to file tree
				m.ciCheckIndex = -1
				m.infoContent = m.buildInfoContent(m.state.data.filteredWts[m.state.data.selectedIndex])
				if len(m.state.services.statusTree.TreeFlat) > 0 {
					m.state.services.statusTree.Index = 0
					m.rebuildStatusContentWithHighlight()
				}
			}
		default:
			// Navigate through status tree items
			if len(m.state.services.statusTree.TreeFlat) > 0 {
				if m.state.services.statusTree.Index < len(m.state.services.statusTree.TreeFlat)-1 {
					m.state.services.statusTree.Index++
				}
				m.rebuildStatusContentWithHighlight()
			}
		}
	default:
		m.state.ui.logTable, cmd = m.state.ui.logTable.Update(keyMsg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

// handleNavigationUp processes up arrow and 'k' key navigation.
func (m *Model) handleNavigationUp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	cmds := []tea.Cmd{}

	switch m.state.view.FocusedPane {
	case 0:
		m.state.ui.worktreeTable, cmd = m.state.ui.worktreeTable.Update(msg)
		m.updateWorktreeArrows()
		m.syncSelectedIndexFromCursor()
		cmds = append(cmds, cmd)
		cmds = append(cmds, m.debouncedUpdateDetailsView())
	case 1:
		// Check if we're navigating CI checks
		ciChecks, hasCIChecks := m.getCIChecksForCurrentWorktree()
		wasReset := false
		// Reset if CI checks are no longer available or index is out of bounds
		if !hasCIChecks || (hasCIChecks && m.ciCheckIndex >= len(ciChecks)) {
			if m.ciCheckIndex >= 0 {
				m.ciCheckIndex = -1
				m.infoContent = m.buildInfoContent(m.state.data.filteredWts[m.state.data.selectedIndex])
				wasReset = true
			}
		}
		switch {
		case hasCIChecks && m.ciCheckIndex >= 0:
			// Navigate CI checks
			if m.ciCheckIndex > 0 {
				m.ciCheckIndex--
				m.infoContent = m.buildInfoContent(m.state.data.filteredWts[m.state.data.selectedIndex])
				m.rebuildStatusContentWithHighlight()
			}
		case hasCIChecks && m.ciCheckIndex == -1 && len(m.state.services.statusTree.TreeFlat) == 0 && !wasReset:
			// No commit changes but CI checks available, wrap to last CI check
			m.ciCheckIndex = len(ciChecks) - 1
			m.infoContent = m.buildInfoContent(m.state.data.filteredWts[m.state.data.selectedIndex])
			m.rebuildStatusContentWithHighlight()
		case hasCIChecks && m.ciCheckIndex == -1 && m.state.services.statusTree.Index == 0:
			// At top of file tree, wrap to last CI check
			m.ciCheckIndex = len(ciChecks) - 1
			m.infoContent = m.buildInfoContent(m.state.data.filteredWts[m.state.data.selectedIndex])
			m.rebuildStatusContentWithHighlight()
		default:
			// Navigate through status tree items
			if len(m.state.services.statusTree.TreeFlat) > 0 {
				if m.state.services.statusTree.Index > 0 {
					m.state.services.statusTree.Index--
				}
				m.rebuildStatusContentWithHighlight()
			}
		}
	default:
		m.state.ui.logTable, cmd = m.state.ui.logTable.Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m *Model) handleFilterNavigation(keyStr string, fillInput bool) (tea.Model, tea.Cmd) {
	// When fillInput is true (Alt+n/Alt+p), navigate through all worktrees
	// When fillInput is false (Up/Down), navigate through filtered worktrees only
	var workList []*models.WorktreeInfo
	if fillInput {
		// Alt+n/Alt+p: navigate through all worktrees (sorted)
		workList = make([]*models.WorktreeInfo, len(m.state.data.worktrees))
		copy(workList, m.state.data.worktrees)
		sortWorktrees(workList, m.sortMode)
	} else {
		// Up/Down: navigate through filtered worktrees
		workList = m.state.data.filteredWts
	}

	if len(workList) == 0 {
		return m, nil
	}

	// Find current position
	currentPath := ""
	if !fillInput {
		// For filtered navigation, use table cursor
		currentIndex := m.state.ui.worktreeTable.Cursor()
		if currentIndex >= 0 && currentIndex < len(m.state.data.filteredWts) {
			currentPath = m.state.data.filteredWts[currentIndex].Path
		}
	} else {
		// For all-worktree navigation, find current selection
		if m.state.data.selectedIndex >= 0 && m.state.data.selectedIndex < len(m.state.data.filteredWts) {
			currentPath = m.state.data.filteredWts[m.state.data.selectedIndex].Path
		}
		if currentPath == "" {
			cursor := m.state.ui.worktreeTable.Cursor()
			if cursor >= 0 && cursor < len(m.state.data.filteredWts) {
				currentPath = m.state.data.filteredWts[cursor].Path
			}
		}
	}

	currentIndex := -1
	if currentPath != "" {
		for i, wt := range workList {
			if wt.Path == currentPath {
				currentIndex = i
				break
			}
		}
	}

	targetIndex := currentIndex
	switch keyStr {
	case "alt+n", keyDown, "ctrl+j":
		if currentIndex == -1 {
			targetIndex = 0
		} else if currentIndex < len(workList)-1 {
			targetIndex = currentIndex + 1
		}
	case "alt+p", keyUp, "ctrl+k":
		if currentIndex == -1 {
			targetIndex = len(workList) - 1
		} else if currentIndex > 0 {
			targetIndex = currentIndex - 1
		}
	default:
		return m, nil
	}
	if targetIndex < 0 || targetIndex >= len(workList) {
		return m, nil
	}

	target := workList[targetIndex]
	if fillInput {
		m.setFilterToWorktree(target)
	}
	m.selectFilteredWorktree(target.Path)
	return m, m.debouncedUpdateDetailsView()
}

func (m *Model) setFilterToWorktree(wt *models.WorktreeInfo) {
	if wt == nil {
		return
	}
	name := filepath.Base(wt.Path)
	if wt.IsMain {
		name = mainWorktreeName
	}
	m.state.ui.filterInput.SetValue(name)
	m.state.ui.filterInput.CursorEnd()
	m.state.services.filter.FilterQuery = name
	m.updateTable()
}

func (m *Model) selectFilteredWorktree(path string) {
	if path == "" {
		return
	}
	for i, wt := range m.state.data.filteredWts {
		if wt.Path == path {
			m.state.ui.worktreeTable.SetCursor(i)
			m.updateWorktreeArrows()
			m.state.data.selectedIndex = i
			return
		}
	}
}

// handlePageDown processes page down navigation.
func (m *Model) handlePageDown(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch m.state.view.FocusedPane {
	case 1:
		m.state.ui.statusViewport.HalfPageDown()
		return m, nil
	case 2:
		m.state.ui.logTable, cmd = m.state.ui.logTable.Update(msg)
		return m, cmd
	}
	return m, nil
}

// handlePageUp processes page up navigation.
func (m *Model) handlePageUp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch m.state.view.FocusedPane {
	case 1:
		m.state.ui.statusViewport.HalfPageUp()
		return m, nil
	case 2:
		m.state.ui.logTable, cmd = m.state.ui.logTable.Update(msg)
		return m, cmd
	}
	return m, nil
}

// handleEnterKey processes the Enter key based on focused pane.
func (m *Model) handleEnterKey() (tea.Model, tea.Cmd) {
	switch m.state.view.FocusedPane {
	case 0:
		// Jump to worktree
		if m.state.data.selectedIndex >= 0 && m.state.data.selectedIndex < len(m.state.data.filteredWts) {
			selectedPath := m.state.data.filteredWts[m.state.data.selectedIndex].Path
			m.persistLastSelected(selectedPath)
			m.selectedPath = selectedPath
			m.stopGitWatcher()
			return m, tea.Quit
		}
	case 1:
		// Check if a CI check is selected
		ciChecks, hasCIChecks := m.getCIChecksForCurrentWorktree()
		switch {
		case hasCIChecks && m.ciCheckIndex >= 0 && m.ciCheckIndex < len(ciChecks):
			// Open selected CI check URL
			check := ciChecks[m.ciCheckIndex]
			if check.Link != "" {
				return m, m.openURLInBrowser(check.Link)
			}
			return m, nil
		case len(m.state.services.statusTree.TreeFlat) > 0 && m.state.services.statusTree.Index >= 0 && m.state.services.statusTree.Index < len(m.state.services.statusTree.TreeFlat):
			// Handle Enter on status tree items
			node := m.state.services.statusTree.TreeFlat[m.state.services.statusTree.Index]
			if node.IsDir() {
				// Toggle collapse state for directories
				m.state.services.statusTree.ToggleCollapse(node.Path)
				m.rebuildStatusContentWithHighlight()
				return m, nil
			}
			// Show diff for files
			return m, m.showFileDiff(*node.File)
		}
	case 2:
		// Open commit view
		return m, m.openCommitView()
	}
	return m, nil
}

// handleMouse processes mouse events for scrolling and clicking
func (m *Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Handle mouse scrolling for CommitScreen via screen manager
	if m.state.ui.screenManager.Type() == appscreen.TypeCommit {
		if cs, ok := m.state.ui.screenManager.Current().(*appscreen.CommitScreen); ok {
			if msg.Action == tea.MouseActionPress {
				switch msg.Button {
				case tea.MouseButtonWheelUp:
					cs.Viewport.ScrollUp(3)
					return m, nil
				case tea.MouseButtonWheelDown:
					cs.Viewport.ScrollDown(3)
					return m, nil
				}
			}
		}
		return m, nil
	}

	// Skip mouse handling when on other modal screens
	if m.state.ui.screenManager.IsActive() {
		return m, nil
	}

	var cmds []tea.Cmd
	layout := m.computeLayout()

	// Calculate pane boundaries (accounting for header and filter)
	headerOffset := 1
	if m.state.view.ShowingFilter {
		headerOffset = 2
	}

	mouseX := msg.X
	mouseY := msg.Y
	targetPane := -1

	if layout.layoutMode == state.LayoutTop {
		// Top layout: worktree at top (full width), status+log side-by-side at bottom
		topY := headerOffset
		topMaxY := headerOffset + layout.topHeight

		bottomY := headerOffset + layout.topHeight + layout.gapY
		bottomMaxY := headerOffset + layout.bodyHeight
		bottomLeftMaxX := layout.bottomLeftWidth
		bottomRightX := layout.bottomLeftWidth + layout.gapX

		switch {
		case mouseY >= topY && mouseY < topMaxY:
			targetPane = 0
		case mouseX < bottomLeftMaxX && mouseY >= bottomY && mouseY < bottomMaxY:
			targetPane = 1
		case mouseX >= bottomRightX && mouseY >= bottomY && mouseY < bottomMaxY:
			targetPane = 2
		}
	} else {
		// Default layout: worktree on left, status+log stacked on right
		leftMaxX := layout.leftWidth
		leftY := headerOffset
		leftMaxY := headerOffset + layout.bodyHeight

		rightX := layout.leftWidth + layout.gapX
		rightTopY := headerOffset
		rightTopMaxX := rightX + layout.rightWidth
		rightTopMaxY := headerOffset + layout.rightTopHeight

		rightBottomY := headerOffset + layout.rightTopHeight + layout.gapY
		rightBottomMaxY := headerOffset + layout.bodyHeight

		switch {
		case mouseX < leftMaxX && mouseY >= leftY && mouseY < leftMaxY:
			targetPane = 0
		case mouseX >= rightX && mouseX < rightTopMaxX && mouseY >= rightTopY && mouseY < rightTopMaxY:
			targetPane = 1
		case mouseX >= rightX && mouseX < rightTopMaxX && mouseY >= rightBottomY && mouseY < rightBottomMaxY:
			targetPane = 2
		}
	}

	switch {
	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft:
		// Click to focus pane and select item
		if targetPane >= 0 && targetPane != m.state.view.FocusedPane {
			m.state.view.FocusedPane = targetPane
			switch m.state.view.FocusedPane {
			case 0:
				m.state.ui.worktreeTable.Focus()
			case 2:
				m.state.ui.logTable.Focus()
			}
		}

		// Handle clicks within the pane to select items
		if targetPane == 0 && len(m.state.data.filteredWts) > 0 {
			// Calculate which row was clicked in the worktree table
			// Account for pane border and title
			paneTopY := headerOffset
			relativeY := mouseY - paneTopY - 4
			if relativeY >= 0 && relativeY < len(m.state.data.filteredWts) {
				m.state.ui.worktreeTable.SetCursor(relativeY)
				m.state.data.selectedIndex = relativeY
				m.updateWorktreeArrows()
				cmds = append(cmds, m.debouncedUpdateDetailsView())
			}
		} else if targetPane == 2 && len(m.state.data.logEntries) > 0 {
			// Calculate which row was clicked in the log table
			var logPaneTopY int
			if layout.layoutMode == state.LayoutTop {
				logPaneTopY = headerOffset + layout.topHeight + layout.gapY
			} else {
				logPaneTopY = headerOffset + layout.rightTopHeight + layout.gapY
			}
			relativeY := mouseY - logPaneTopY - 4
			if relativeY >= 0 && relativeY < len(m.state.data.logEntries) {
				m.state.ui.logTable.SetCursor(relativeY)
			}
		}

	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonWheelUp:
		switch targetPane {
		case 0:
			// Scroll worktree table up
			m.state.ui.worktreeTable, _ = m.state.ui.worktreeTable.Update(tea.KeyMsg{Type: tea.KeyUp})
			m.updateWorktreeArrows()
			m.syncSelectedIndexFromCursor()
			cmds = append(cmds, m.debouncedUpdateDetailsView())
		case 1:
			// Navigate up through tree items
			if len(m.state.services.statusTree.TreeFlat) > 0 && m.state.services.statusTree.Index > 0 {
				m.state.services.statusTree.Index--
				m.rebuildStatusContentWithHighlight()
			}
		case 2:
			// Scroll log table up
			m.state.ui.logTable, _ = m.state.ui.logTable.Update(tea.KeyMsg{Type: tea.KeyUp})
		}

	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonWheelDown:
		switch targetPane {
		case 0:
			// Scroll worktree table down
			m.state.ui.worktreeTable, _ = m.state.ui.worktreeTable.Update(tea.KeyMsg{Type: tea.KeyDown})
			m.updateWorktreeArrows()
			m.syncSelectedIndexFromCursor()
			cmds = append(cmds, m.debouncedUpdateDetailsView())
		case 1:
			// Navigate down through tree items
			if len(m.state.services.statusTree.TreeFlat) > 0 && m.state.services.statusTree.Index < len(m.state.services.statusTree.TreeFlat)-1 {
				m.state.services.statusTree.Index++
				m.rebuildStatusContentWithHighlight()
			}
		case 2:
			// Scroll log table down
			m.state.ui.logTable, _ = m.state.ui.logTable.Update(tea.KeyMsg{Type: tea.KeyDown})
		}
	}

	return m, tea.Batch(cmds...)
}
