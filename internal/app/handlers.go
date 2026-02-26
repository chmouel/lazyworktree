package app

import (
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/app/state"
	"github.com/chmouel/lazyworktree/internal/models"
)

// handleKeyMsg processes keyboard input when not in a modal screen.
func (m *Model) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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
		case filterTargetStatus, filterTargetGitStatus:
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
			m.setFilterQuery(filterTargetGitStatus, m.state.ui.filterInput.Value())
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

func (m *Model) handleSearchInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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
	case 2:
		m.state.services.filter.StatusFilterQuery = ""
		m.state.ui.filterInput.SetValue("")
		m.applyStatusFilter()
	case 3:
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
		m.state.ui.infoViewport.GotoTop()
	case 2:
		if len(m.state.services.statusTree.TreeFlat) > 0 {
			m.state.services.statusTree.Index = 0
			m.rebuildStatusContentWithHighlight()
		}
	case 3:
		m.state.ui.logTable.GotoTop()
	case 4:
		m.state.ui.notesViewport.GotoTop()
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
		m.state.ui.infoViewport.GotoBottom()
	case 2:
		if len(m.state.services.statusTree.TreeFlat) > 0 {
			m.state.services.statusTree.Index = len(m.state.services.statusTree.TreeFlat) - 1
			m.rebuildStatusContentWithHighlight()
		}
	case 3:
		m.state.ui.logTable.GotoBottom()
	case 4:
		m.state.ui.notesViewport.GotoBottom()
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

// nextPane returns the next pane index in the given direction (+1 or -1),
// including pane 4 (Notes) in the cycle when a note exists,
// and excluding pane 2 (Git Status) when the working tree is clean.
func (m *Model) nextPane(current, direction int) int {
	// Build ordered pane list based on visible panes
	hasNotes := m.hasNoteForSelectedWorktree()
	hasGitStatus := m.hasGitStatus()

	panes := make([]int, 0, 5)
	panes = append(panes, 0)
	if hasNotes {
		panes = append(panes, 4)
	}
	panes = append(panes, 1)
	if hasGitStatus {
		panes = append(panes, 2)
	}
	panes = append(panes, 3)

	for i, p := range panes {
		if p == current {
			next := (i + direction + len(panes)) % len(panes)
			return panes[next]
		}
	}
	// Fallback: if current pane not found (e.g. hidden pane), start from 0
	if direction > 0 {
		return panes[0]
	}
	return panes[len(panes)-1]
}

// handleBuiltInKey processes built-in keyboard shortcuts.
func (m *Model) handleBuiltInKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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
			if m.state.view.ZoomedPane >= 0 {
				m.state.view.ZoomedPane = -1
			} else {
				m.state.view.ZoomedPane = targetPane
			}
		} else {
			m.state.view.ZoomedPane = -1
			m.state.view.FocusedPane = targetPane
		}
		m.rebuildStatusContentWithHighlight()
		return m, nil

	case "3":
		if !m.hasGitStatus() {
			return m, nil
		}
		targetPane := 2
		if m.state.view.FocusedPane == targetPane {
			if m.state.view.ZoomedPane >= 0 {
				m.state.view.ZoomedPane = -1
			} else {
				m.state.view.ZoomedPane = targetPane
			}
		} else {
			m.state.view.ZoomedPane = -1
			wasPane1 := m.state.view.FocusedPane == 1
			if wasPane1 {
				m.ciCheckIndex = -1
			}
			m.state.view.FocusedPane = targetPane
		}
		m.rebuildStatusContentWithHighlight()
		return m, nil

	case "4":
		targetPane := 3
		if m.state.view.FocusedPane == targetPane {
			if m.state.view.ZoomedPane >= 0 {
				m.state.view.ZoomedPane = -1
			} else {
				m.state.view.ZoomedPane = targetPane
			}
		} else {
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

	case "5":
		if !m.hasNoteForSelectedWorktree() {
			return m, nil
		}
		targetPane := 4
		if m.state.view.FocusedPane == targetPane {
			if m.state.view.ZoomedPane >= 0 {
				m.state.view.ZoomedPane = -1
			} else {
				m.state.view.ZoomedPane = targetPane
			}
		} else {
			m.state.view.ZoomedPane = -1
			wasPane1 := m.state.view.FocusedPane == 1
			if wasPane1 {
				m.ciCheckIndex = -1
			}
			m.state.view.FocusedPane = targetPane
			if wasPane1 {
				m.rebuildStatusContentWithHighlight()
			}
		}
		return m, nil

	case keyTab, "]":
		m.state.view.ZoomedPane = -1
		wasPane1 := m.state.view.FocusedPane == 1
		m.state.view.FocusedPane = m.nextPane(m.state.view.FocusedPane, 1)
		if wasPane1 && m.state.view.FocusedPane != 1 {
			m.ciCheckIndex = -1
		}
		switch m.state.view.FocusedPane {
		case 0:
			m.state.ui.worktreeTable.Focus()
		case 3:
			m.state.ui.logTable.Focus()
		}
		if wasPane1 || m.state.view.FocusedPane == 1 || m.state.view.FocusedPane == 2 {
			m.rebuildStatusContentWithHighlight()
		}
		return m, nil

	case "[":
		m.state.view.ZoomedPane = -1
		wasPane1 := m.state.view.FocusedPane == 1
		m.state.view.FocusedPane = m.nextPane(m.state.view.FocusedPane, -1)
		if wasPane1 && m.state.view.FocusedPane != 1 {
			m.ciCheckIndex = -1
		}
		switch m.state.view.FocusedPane {
		case 0:
			m.state.ui.worktreeTable.Focus()
		case 3:
			m.state.ui.logTable.Focus()
		}
		if wasPane1 || m.state.view.FocusedPane == 1 || m.state.view.FocusedPane == 2 {
			m.rebuildStatusContentWithHighlight()
		}
		return m, nil

	case "h":
		switch {
		case m.state.view.Layout == state.LayoutTop:
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
			case 3:
				m.state.view.ZoomedPane = -1
				if m.hasGitStatus() {
					m.state.view.FocusedPane = 2
				} else {
					m.state.view.FocusedPane = 1
				}
				m.rebuildStatusContentWithHighlight()
			case 4:
				// Notes in top layout: go to worktrees (above)
				m.state.view.ZoomedPane = -1
				m.state.view.FocusedPane = 0
				m.state.ui.worktreeTable.Focus()
			}
		case m.state.view.FocusedPane == 0 || m.state.view.FocusedPane == 4:
			// Default layout: worktrees and notes are already leftmost, no-op
		default:
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
				m.ciCheckIndex = -1
				if m.hasGitStatus() {
					m.state.view.FocusedPane = 2
					m.rebuildStatusContentWithHighlight()
				} else {
					m.state.view.FocusedPane = 3
					m.state.ui.logTable.Focus()
				}
			case 2:
				m.state.view.ZoomedPane = -1
				m.state.view.FocusedPane = 3
				m.state.ui.logTable.Focus()
			case 4:
				// Notes in top layout: go right to first bottom pane
				m.state.view.ZoomedPane = -1
				m.state.view.FocusedPane = 1
				m.rebuildStatusContentWithHighlight()
			}
		} else {
			// Default layout: navigate right - cycle through right panels (1, 2, 3)
			switch m.state.view.FocusedPane {
			case 0:
				m.state.view.ZoomedPane = -1
				m.state.view.FocusedPane = 1
				m.rebuildStatusContentWithHighlight()
			case 1:
				m.state.view.ZoomedPane = -1
				m.ciCheckIndex = -1
				if m.hasGitStatus() {
					m.state.view.FocusedPane = 2
					m.rebuildStatusContentWithHighlight()
				} else {
					m.state.view.FocusedPane = 3
					m.state.ui.logTable.Focus()
				}
			case 2:
				m.state.view.ZoomedPane = -1
				m.state.view.FocusedPane = 3
				m.state.ui.logTable.Focus()
			case 4:
				// Notes in default layout: go right to commit pane (same vertical level)
				m.state.view.ZoomedPane = -1
				m.state.view.FocusedPane = 3
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
		if m.state.view.FocusedPane == 2 && len(m.state.services.statusTree.TreeFlat) > 0 {
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
		if m.state.view.FocusedPane == 3 {
			prevCursor := m.state.ui.logTable.Cursor()
			_, moveCmd := m.handleNavigationDown(tea.KeyPressMsg{Code: tea.KeyDown})
			if m.state.ui.logTable.Cursor() == prevCursor {
				return m, moveCmd
			}
			return m, tea.Batch(moveCmd, m.openCommitView())
		}
		if m.state.view.FocusedPane == 4 {
			m.state.ui.notesViewport.ScrollDown(1)
			return m, nil
		}
		return m, nil

	case keyCtrlK:
		if m.state.view.FocusedPane == 2 && len(m.state.services.statusTree.TreeFlat) > 0 {
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
		if m.state.view.FocusedPane == 4 {
			m.state.ui.notesViewport.ScrollUp(1)
			return m, nil
		}
		return m, nil

	case "ctrl+d", "space":
		return m.handlePageDown(msg)

	case "ctrl+u":
		return m.handlePageUp(msg)

	case "pgdown":
		return m.handlePageDown(msg)

	case "pgup":
		return m.handlePageUp(msg)

	case "G":
		if m.state.view.FocusedPane == 1 {
			m.state.ui.infoViewport.GotoBottom()
			return m, nil
		}
		if m.state.view.FocusedPane == 2 {
			m.state.ui.statusViewport.GotoBottom()
			if len(m.state.services.statusTree.TreeFlat) > 0 {
				m.state.services.statusTree.Index = len(m.state.services.statusTree.TreeFlat) - 1
			}
			return m, nil
		}
		if m.state.view.FocusedPane == 4 {
			m.state.ui.notesViewport.GotoBottom()
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
		if m.state.view.FocusedPane == 2 {
			return m, m.commitStagedChanges()
		}
		return m, m.showCreateWorktree()

	case "D":
		if m.state.view.FocusedPane == 2 {
			return m, m.showDeleteFile()
		}
		return m, m.showDeleteWorktree()

	case "d":
		// If in commit pane, show commit diff
		if m.state.view.FocusedPane == 3 {
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
		if m.state.view.FocusedPane == 2 && len(m.state.services.statusTree.TreeFlat) > 0 && m.state.services.statusTree.Index >= 0 && m.state.services.statusTree.Index < len(m.state.services.statusTree.TreeFlat) {
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
		case 2:
			target = filterTargetGitStatus
		case 3:
			target = filterTargetLog
		}
		return m, m.startFilter(target)

	case "/":
		target := searchTargetWorktrees
		switch m.state.view.FocusedPane {
		case 2:
			target = searchTargetGitStatus
		case 3:
			target = searchTargetLog
		}
		return m, m.startSearch(target)

	case "n":
		if m.state.view.FocusedPane == 1 {
			return m.navigateCICheckDown()
		}
		return m, m.advanceSearchMatch(true)
	case "N":
		return m, m.advanceSearchMatch(false)
	case "p":
		if m.state.view.FocusedPane == 1 {
			return m.navigateCICheckUp()
		}
		return m, nil

	case "s":
		// In git status pane: stage/unstage selected file or directory
		if m.state.view.FocusedPane == 2 && len(m.state.services.statusTree.TreeFlat) > 0 && m.state.services.statusTree.Index >= 0 && m.state.services.statusTree.Index < len(m.state.services.statusTree.TreeFlat) {
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
		if m.state.view.FocusedPane == 2 {
			return m, m.commitAllChanges()
		}
		return m, m.showCherryPick()

	case "y":
		return m, m.yankContextual()

	case "Y":
		return m, m.yankBranch()

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
	if msg.Code == tea.KeyHome {
		return m.handleGotoTop()
	}
	if msg.Code == tea.KeyEnd {
		return m.handleGotoBottom()
	}

	// Handle Ctrl+Left/Right for folder navigation in git status pane
	if m.state.view.FocusedPane == 2 {
		if msg.Code == tea.KeyLeft && msg.Mod == tea.ModCtrl {
			return m.handlePrevFolder()
		}
		if msg.Code == tea.KeyRight && msg.Mod == tea.ModCtrl {
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
func (m *Model) handleNavigationDown(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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
		m.state.ui.infoViewport.ScrollDown(1)
	case 2:
		// Navigate through status tree items (git status pane)
		if len(m.state.services.statusTree.TreeFlat) > 0 {
			if m.state.services.statusTree.Index < len(m.state.services.statusTree.TreeFlat)-1 {
				m.state.services.statusTree.Index++
			}
			m.rebuildStatusContentWithHighlight()
		}
	case 4:
		m.state.ui.notesViewport.ScrollDown(1)
	default:
		m.state.ui.logTable, cmd = m.state.ui.logTable.Update(keyMsg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

// handleNavigationUp processes up arrow and 'k' key navigation.
func (m *Model) handleNavigationUp(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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
		m.state.ui.infoViewport.ScrollUp(1)
	case 2:
		// Navigate through status tree items (git status pane)
		if len(m.state.services.statusTree.TreeFlat) > 0 {
			if m.state.services.statusTree.Index > 0 {
				m.state.services.statusTree.Index--
			}
			m.rebuildStatusContentWithHighlight()
		}
	case 4:
		m.state.ui.notesViewport.ScrollUp(1)
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
func (m *Model) handlePageDown(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch m.state.view.FocusedPane {
	case 1:
		m.state.ui.infoViewport.HalfPageDown()
		return m, nil
	case 2:
		m.state.ui.statusViewport.HalfPageDown()
		return m, nil
	case 3:
		m.state.ui.logTable, cmd = m.state.ui.logTable.Update(msg)
		return m, cmd
	case 4:
		m.state.ui.notesViewport.HalfPageDown()
		return m, nil
	}
	return m, nil
}

// handlePageUp processes page up navigation.
func (m *Model) handlePageUp(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch m.state.view.FocusedPane {
	case 1:
		m.state.ui.infoViewport.HalfPageUp()
		return m, nil
	case 2:
		m.state.ui.statusViewport.HalfPageUp()
		return m, nil
	case 3:
		m.state.ui.logTable, cmd = m.state.ui.logTable.Update(msg)
		return m, cmd
	case 4:
		m.state.ui.notesViewport.HalfPageUp()
		return m, nil
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
		// Check if a CI check is selected in the Status (info) pane
		ciChecks, hasCIChecks := m.getCIChecksForCurrentWorktree()
		if hasCIChecks && m.ciCheckIndex >= 0 && m.ciCheckIndex < len(ciChecks) {
			check := ciChecks[m.ciCheckIndex]
			if check.Link != "" {
				return m, m.openURLInBrowser(check.Link)
			}
		}
	case 2:
		// Handle Enter on status tree items (git status pane)
		if len(m.state.services.statusTree.TreeFlat) > 0 && m.state.services.statusTree.Index >= 0 && m.state.services.statusTree.Index < len(m.state.services.statusTree.TreeFlat) {
			node := m.state.services.statusTree.TreeFlat[m.state.services.statusTree.Index]
			if node.IsDir() {
				m.state.services.statusTree.ToggleCollapse(node.Path)
				m.rebuildStatusContentWithHighlight()
				return m, nil
			}
			return m, m.showFileDiff(*node.File)
		}
	case 3:
		// Open commit view
		return m, m.openCommitView()
	}
	return m, nil
}

// handleMouseClick processes mouse click events for pane focus and item selection.
func (m *Model) handleMouseClick(msg tea.MouseClickMsg) (tea.Model, tea.Cmd) {
	// Skip mouse handling when on modal screens
	if m.state.ui.screenManager.IsActive() {
		return m, nil
	}

	if msg.Button != tea.MouseLeft {
		return m, nil
	}

	var cmds []tea.Cmd
	layout := m.computeLayout()

	// Calculate pane boundaries (accounting for header and filter)
	headerOffset := 1
	if m.state.view.ShowingFilter {
		headerOffset = 2
	}

	mouseX := msg.Mouse().X
	mouseY := msg.Mouse().Y
	targetPane := -1

	if layout.layoutMode == state.LayoutTop {
		// Top layout: worktree at top (full width), optional notes row, status+git status+commit side-by-side at bottom
		topY := headerOffset
		topMaxY := headerOffset + layout.topHeight

		notesY := topMaxY + layout.gapY
		notesMaxY := notesY + layout.notesRowHeight

		var bottomY int
		if layout.hasNotes {
			bottomY = notesMaxY + layout.gapY
		} else {
			bottomY = topMaxY + layout.gapY
		}
		bottomMaxY := headerOffset + layout.bodyHeight
		bottomLeftMaxX := layout.bottomLeftWidth
		bottomMiddleX := layout.bottomLeftWidth + layout.gapX
		bottomMiddleMaxX := bottomMiddleX + layout.bottomMiddleWidth
		bottomRightX := bottomMiddleMaxX + layout.gapX

		switch {
		case mouseY >= topY && mouseY < topMaxY:
			targetPane = 0
		case layout.hasNotes && mouseY >= notesY && mouseY < notesMaxY:
			targetPane = 4
		case mouseX < bottomLeftMaxX && mouseY >= bottomY && mouseY < bottomMaxY:
			targetPane = 1
		case mouseX >= bottomMiddleX && mouseX < bottomMiddleMaxX && mouseY >= bottomY && mouseY < bottomMaxY:
			targetPane = 2
		case mouseX >= bottomRightX && mouseY >= bottomY && mouseY < bottomMaxY:
			targetPane = 3
		}
	} else {
		// Default layout: worktree on left (optionally split with notes), status+git status+commit stacked on right
		leftMaxX := layout.leftWidth

		rightX := layout.leftWidth + layout.gapX
		rightTopY := headerOffset
		rightTopMaxX := rightX + layout.rightWidth
		rightTopMaxY := headerOffset + layout.rightTopHeight

		rightMiddleY := rightTopMaxY + layout.gapY
		rightMiddleMaxY := rightMiddleY + layout.rightMiddleHeight

		rightBottomY := rightMiddleMaxY + layout.gapY
		rightBottomMaxY := headerOffset + layout.bodyHeight

		if layout.hasNotes && mouseX < leftMaxX {
			// Left column is split: top = worktrees, bottom = notes
			leftTopY := headerOffset
			leftTopMaxY := headerOffset + layout.leftTopHeight
			leftBottomY := leftTopMaxY + layout.gapY
			leftBottomMaxY := headerOffset + layout.bodyHeight

			switch {
			case mouseY >= leftTopY && mouseY < leftTopMaxY:
				targetPane = 0
			case mouseY >= leftBottomY && mouseY < leftBottomMaxY:
				targetPane = 4
			}
		} else {
			switch {
			case mouseX < leftMaxX && mouseY >= headerOffset && mouseY < headerOffset+layout.bodyHeight:
				targetPane = 0
			case mouseX >= rightX && mouseX < rightTopMaxX && mouseY >= rightTopY && mouseY < rightTopMaxY:
				targetPane = 1
			case mouseX >= rightX && mouseX < rightTopMaxX && mouseY >= rightMiddleY && mouseY < rightMiddleMaxY:
				targetPane = 2
			case mouseX >= rightX && mouseX < rightTopMaxX && mouseY >= rightBottomY && mouseY < rightBottomMaxY:
				targetPane = 3
			}
		}
	}

	// Click to focus pane and select item
	if targetPane >= 0 && targetPane != m.state.view.FocusedPane {
		m.state.view.FocusedPane = targetPane
		switch m.state.view.FocusedPane {
		case 0:
			m.state.ui.worktreeTable.Focus()
		case 3:
			m.state.ui.logTable.Focus()
		}
	}

	// Handle clicks within the pane to select items
	if targetPane == 0 && len(m.state.data.filteredWts) > 0 {
		paneTopY := headerOffset
		relativeY := mouseY - paneTopY - 4
		if relativeY >= 0 && relativeY < len(m.state.data.filteredWts) {
			m.state.ui.worktreeTable.SetCursor(relativeY)
			m.state.data.selectedIndex = relativeY
			m.updateWorktreeArrows()
			cmds = append(cmds, m.debouncedUpdateDetailsView())
		}
	} else if targetPane == 3 && len(m.state.data.logEntries) > 0 {
		var logPaneTopY int
		if layout.layoutMode == state.LayoutTop {
			logPaneTopY = headerOffset + layout.topHeight + layout.gapY
			if layout.hasNotes {
				logPaneTopY += layout.notesRowHeight + layout.gapY
			}
		} else {
			logPaneTopY = headerOffset + layout.rightTopHeight + layout.gapY + layout.rightMiddleHeight + layout.gapY
		}
		relativeY := mouseY - logPaneTopY - 4
		if relativeY >= 0 && relativeY < len(m.state.data.logEntries) {
			m.state.ui.logTable.SetCursor(relativeY)
		}
	}

	return m, tea.Batch(cmds...)
}

// handleMouseWheel processes mouse wheel scroll events.
func (m *Model) handleMouseWheel(msg tea.MouseWheelMsg) (tea.Model, tea.Cmd) {
	// Handle mouse scrolling for CommitScreen via screen manager
	if m.state.ui.screenManager.Type() == appscreen.TypeCommit {
		if cs, ok := m.state.ui.screenManager.Current().(*appscreen.CommitScreen); ok {
			switch msg.Button {
			case tea.MouseWheelUp:
				cs.Viewport.ScrollUp(3)
				return m, nil
			case tea.MouseWheelDown:
				cs.Viewport.ScrollDown(3)
				return m, nil
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

	mouseX := msg.Mouse().X
	mouseY := msg.Mouse().Y
	targetPane := -1

	if layout.layoutMode == state.LayoutTop {
		topY := headerOffset
		topMaxY := headerOffset + layout.topHeight

		notesY := topMaxY + layout.gapY
		notesMaxY := notesY + layout.notesRowHeight

		var bottomY int
		if layout.hasNotes {
			bottomY = notesMaxY + layout.gapY
		} else {
			bottomY = topMaxY + layout.gapY
		}
		bottomMaxY := headerOffset + layout.bodyHeight
		bottomLeftMaxX := layout.bottomLeftWidth
		bottomMiddleX := layout.bottomLeftWidth + layout.gapX
		bottomMiddleMaxX := bottomMiddleX + layout.bottomMiddleWidth
		bottomRightX := bottomMiddleMaxX + layout.gapX

		switch {
		case mouseY >= topY && mouseY < topMaxY:
			targetPane = 0
		case layout.hasNotes && mouseY >= notesY && mouseY < notesMaxY:
			targetPane = 4
		case mouseX < bottomLeftMaxX && mouseY >= bottomY && mouseY < bottomMaxY:
			targetPane = 1
		case mouseX >= bottomMiddleX && mouseX < bottomMiddleMaxX && mouseY >= bottomY && mouseY < bottomMaxY:
			targetPane = 2
		case mouseX >= bottomRightX && mouseY >= bottomY && mouseY < bottomMaxY:
			targetPane = 3
		}
	} else {
		leftMaxX := layout.leftWidth
		rightX := layout.leftWidth + layout.gapX
		rightTopY := headerOffset
		rightTopMaxX := rightX + layout.rightWidth
		rightTopMaxY := headerOffset + layout.rightTopHeight
		rightMiddleY := rightTopMaxY + layout.gapY
		rightMiddleMaxY := rightMiddleY + layout.rightMiddleHeight
		rightBottomY := rightMiddleMaxY + layout.gapY
		rightBottomMaxY := headerOffset + layout.bodyHeight

		if layout.hasNotes && mouseX < leftMaxX {
			leftTopY := headerOffset
			leftTopMaxY := headerOffset + layout.leftTopHeight
			leftBottomY := leftTopMaxY + layout.gapY
			leftBottomMaxY := headerOffset + layout.bodyHeight

			switch {
			case mouseY >= leftTopY && mouseY < leftTopMaxY:
				targetPane = 0
			case mouseY >= leftBottomY && mouseY < leftBottomMaxY:
				targetPane = 4
			}
		} else {
			switch {
			case mouseX < leftMaxX && mouseY >= headerOffset && mouseY < headerOffset+layout.bodyHeight:
				targetPane = 0
			case mouseX >= rightX && mouseX < rightTopMaxX && mouseY >= rightTopY && mouseY < rightTopMaxY:
				targetPane = 1
			case mouseX >= rightX && mouseX < rightTopMaxX && mouseY >= rightMiddleY && mouseY < rightMiddleMaxY:
				targetPane = 2
			case mouseX >= rightX && mouseX < rightTopMaxX && mouseY >= rightBottomY && mouseY < rightBottomMaxY:
				targetPane = 3
			}
		}
	}

	switch msg.Button {
	case tea.MouseWheelUp:
		switch targetPane {
		case 0:
			m.state.ui.worktreeTable, _ = m.state.ui.worktreeTable.Update(tea.KeyPressMsg{Code: tea.KeyUp})
			m.updateWorktreeArrows()
			m.syncSelectedIndexFromCursor()
			cmds = append(cmds, m.debouncedUpdateDetailsView())
		case 1:
			m.state.ui.infoViewport.ScrollUp(3)
		case 2:
			// Navigate up through tree items in git status pane
			if len(m.state.services.statusTree.TreeFlat) > 0 && m.state.services.statusTree.Index > 0 {
				m.state.services.statusTree.Index--
				m.rebuildStatusContentWithHighlight()
			}
		case 3:
			m.state.ui.logTable, _ = m.state.ui.logTable.Update(tea.KeyPressMsg{Code: tea.KeyUp})
		case 4:
			m.state.ui.notesViewport.ScrollUp(3)
		}

	case tea.MouseWheelDown:
		switch targetPane {
		case 0:
			m.state.ui.worktreeTable, _ = m.state.ui.worktreeTable.Update(tea.KeyPressMsg{Code: tea.KeyDown})
			m.updateWorktreeArrows()
			m.syncSelectedIndexFromCursor()
			cmds = append(cmds, m.debouncedUpdateDetailsView())
		case 1:
			m.state.ui.infoViewport.ScrollDown(3)
		case 2:
			// Navigate down through tree items in git status pane
			if len(m.state.services.statusTree.TreeFlat) > 0 && m.state.services.statusTree.Index < len(m.state.services.statusTree.TreeFlat)-1 {
				m.state.services.statusTree.Index++
				m.rebuildStatusContentWithHighlight()
			}
		case 3:
			m.state.ui.logTable, _ = m.state.ui.logTable.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		case 4:
			m.state.ui.notesViewport.ScrollDown(3)
		}
	}

	return m, tea.Batch(cmds...)
}

// navigateCICheckDown moves the CI check selection to the next check.
func (m *Model) navigateCICheckDown() (tea.Model, tea.Cmd) {
	ciChecks, hasCIChecks := m.getCIChecksForCurrentWorktree()
	if !hasCIChecks {
		return m, nil
	}
	if m.ciCheckIndex >= len(ciChecks) {
		m.ciCheckIndex = -1
		m.infoContent = m.buildInfoContent(m.state.data.filteredWts[m.state.data.selectedIndex])
	}
	if m.ciCheckIndex == -1 {
		m.ciCheckIndex = 0
	} else if m.ciCheckIndex < len(ciChecks)-1 {
		m.ciCheckIndex++
	}
	m.infoContent = m.buildInfoContent(m.state.data.filteredWts[m.state.data.selectedIndex])
	return m, nil
}

// navigateCICheckUp moves the CI check selection to the previous check.
func (m *Model) navigateCICheckUp() (tea.Model, tea.Cmd) {
	ciChecks, hasCIChecks := m.getCIChecksForCurrentWorktree()
	if !hasCIChecks {
		return m, nil
	}
	if m.ciCheckIndex >= len(ciChecks) {
		m.ciCheckIndex = -1
		m.infoContent = m.buildInfoContent(m.state.data.filteredWts[m.state.data.selectedIndex])
	}
	switch {
	case m.ciCheckIndex > 0:
		m.ciCheckIndex--
	case m.ciCheckIndex == -1:
		m.ciCheckIndex = len(ciChecks) - 1
	}
	m.infoContent = m.buildInfoContent(m.state.data.filteredWts[m.state.data.selectedIndex])
	return m, nil
}
