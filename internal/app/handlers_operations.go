package app

import (
	tea "charm.land/bubbletea/v2"
	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
)

// handleBuiltInKey processes built-in keyboard shortcuts.
func (m *Model) handleBuiltInKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if model, cmd, handled := m.handleQuitKey(msg); handled {
		return model, cmd
	}
	if model, cmd, handled := m.handlePaneKey(msg); handled {
		return model, cmd
	}
	if model, cmd, handled := m.handleNavigationKey(msg); handled {
		return model, cmd
	}
	if model, cmd, handled := m.handleSearchKey(msg); handled {
		return model, cmd
	}
	if model, cmd, handled := m.handleFilterKey(msg); handled {
		return model, cmd
	}
	if model, cmd, handled := m.handleOperationKey(msg); handled {
		return model, cmd
	}
	if model, cmd, handled := m.handleCodeNavigationKey(msg); handled {
		return model, cmd
	}

	if m.state.view.FocusedPane == paneWorktrees {
		var cmd tea.Cmd
		m.state.ui.worktreeTable, cmd = m.state.ui.worktreeTable.Update(msg)
		m.syncSelectedIndexFromCursor()
		return m, tea.Batch(cmd, m.debouncedUpdateDetailsView())
	}

	return m, nil
}

func (m *Model) handleQuitKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	switch msg.String() {
	case keyCtrlC, keyQ:
		if m.selectedPath != "" {
			m.stopGitWatcher()
			m.stopAgentWatcher()
			return m, tea.Quit, true
		}
		m.quitting = true
		m.stopGitWatcher()
		m.stopAgentWatcher()
		return m, tea.Quit, true
	default:
		return m, nil, false
	}
}

func (m *Model) handleOperationKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	switch msg.String() {
	case keyEnter:
		model, cmd := m.handleEnterKey()
		return model, cmd, true
	case "r":
		m.loading = true
		m.setLoadingScreen(loadingRefreshWorktrees)
		cmds := []tea.Cmd{m.refreshWorktrees()}
		if !m.config.DisablePR && m.state.services.git.IsGitHubOrGitLab(m.ctx) {
			m.cache.ciCache.Clear()
			if cmd := m.refreshCurrentWorktreePR(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return m, tea.Batch(cmds...), true
	case "c":
		if m.state.view.FocusedPane == paneGitStatus {
			return m, m.commitStagedChanges(), true
		}
		return m, m.showCreateWorktree(), true
	case keyCtrlG:
		return m, m.commitStagedChanges(), true
	case "D":
		if m.state.view.FocusedPane == paneGitStatus {
			return m, m.showDeleteFile(), true
		}
		return m, m.showDeleteWorktree(), true
	case "d":
		if m.state.view.FocusedPane == paneCommit {
			cursor := m.state.ui.logTable.Cursor()
			if len(m.state.data.logEntries) > 0 && cursor >= 0 && cursor < len(m.state.data.logEntries) {
				if m.state.data.selectedIndex >= 0 && m.state.data.selectedIndex < len(m.state.data.filteredWts) {
					commitSHA := m.state.data.logEntries[cursor].sha
					wt := m.state.data.filteredWts[m.state.data.selectedIndex]
					return m, m.showCommitDiff(commitSHA, wt), true
				}
			}
			return m, nil, true
		}
		return m, m.showDiff(), true
	case "e":
		if m.state.view.FocusedPane == paneWorktrees {
			return m, m.showEditWorktreeMetadataMenu(), true
		}
		if m.state.view.FocusedPane == paneGitStatus && len(m.state.services.statusTree.TreeFlat) > 0 && m.state.services.statusTree.Index >= 0 && m.state.services.statusTree.Index < len(m.state.services.statusTree.TreeFlat) {
			node := m.state.services.statusTree.TreeFlat[m.state.services.statusTree.Index]
			if !node.IsDir() {
				return m, m.openStatusFileInEditor(*node.File), true
			}
		}
		return m, nil, true
	case "v":
		return m, m.openCICheckSelection(), true
	case "ctrl+v":
		if m.state.view.FocusedPane == paneInfo {
			ciChecks, hasCIChecks := m.getCIChecksForCurrentWorktree()
			if hasCIChecks && m.ciCheckIndex >= 0 && m.ciCheckIndex < len(ciChecks) {
				check := ciChecks[m.ciCheckIndex]
				return m, m.showCICheckLog(check), true
			}
		}
		return m, nil, true
	case "P":
		return m, m.pushToUpstream(), true
	case "S":
		return m, m.syncWithUpstream(), true
	case "R":
		m.loading = true
		m.statusContent = "Fetching remotes..."
		m.setLoadingScreen("Fetching remotes...")
		return m, m.fetchRemotes(), true
	case "s":
		if m.state.view.FocusedPane == paneGitStatus && len(m.state.services.statusTree.TreeFlat) > 0 && m.state.services.statusTree.Index >= 0 && m.state.services.statusTree.Index < len(m.state.services.statusTree.TreeFlat) {
			node := m.state.services.statusTree.TreeFlat[m.state.services.statusTree.Index]
			if node.IsDir() {
				return m, m.stageDirectory(node), true
			}
			return m, m.stageCurrentFile(*node.File), true
		}
		m.sortMode = (m.sortMode + 1) % 3
		m.updateTable()
		return m, nil, true
	case "ctrl+p", ":":
		return m, m.showCommandPalette(), true
	case "?":
		helpScreen := appscreen.NewHelpScreen(m.state.view.WindowWidth, m.state.view.WindowHeight, m.config.CustomCommands, m.config.Keybindings, m.theme, m.config.IconsEnabled())
		m.state.ui.screenManager.Push(helpScreen)
		return m, nil, true
	case "g":
		return m, m.openLazyGit(), true
	case "o":
		if m.state.view.FocusedPane == paneCommit {
			cursor := m.state.ui.logTable.Cursor()
			if len(m.state.data.logEntries) > 0 && cursor >= 0 && cursor < len(m.state.data.logEntries) {
				sha := m.state.data.logEntries[cursor].sha
				return m, m.openCommitInBrowser(sha), true
			}
			return m, nil, true
		}
		return m, m.openPR(), true
	case "m":
		return m, m.showRenameWorktree(), true
	case "i":
		if m.state.view.FocusedPane == paneNotes {
			return m, m.showAnnotateWorktree(), true
		}
		return m, nil, true
	case "T":
		return m, m.showTaskboard(), true
	case "A":
		if m.state.view.FocusedPane == paneAgentSessions {
			m.state.view.ShowAllAgentSessions = !m.state.view.ShowAllAgentSessions
			m.refreshSelectedWorktreeAgentSessionsPane()
			return m, nil, true
		}
		return m, m.showAbsorbWorktree(), true
	case "X":
		return m, m.showPruneMerged(), true
	case "!":
		return m, m.showRunCommand(), true
	case "C":
		if m.state.view.FocusedPane == paneGitStatus {
			return m, m.commitAllChanges(), true
		}
		return m, m.showCherryPick(), true
	case "y":
		return m, m.yankContextual(), true
	case "Y":
		return m, m.yankBranch(), true
	default:
		return m, nil, false
	}
}

// handleEnterKey processes the Enter key based on focused pane.
func (m *Model) handleEnterKey() (tea.Model, tea.Cmd) {
	switch m.state.view.FocusedPane {
	case paneWorktrees:
		if m.state.data.selectedIndex >= 0 && m.state.data.selectedIndex < len(m.state.data.filteredWts) {
			selectedPath := m.state.data.filteredWts[m.state.data.selectedIndex].Path
			m.persistLastSelected(selectedPath)
			m.selectedPath = selectedPath
			m.stopGitWatcher()
			m.stopAgentWatcher()
			return m, tea.Quit
		}
	case paneInfo:
		ciChecks, hasCIChecks := m.getCIChecksForCurrentWorktree()
		if hasCIChecks && m.ciCheckIndex >= 0 && m.ciCheckIndex < len(ciChecks) {
			check := ciChecks[m.ciCheckIndex]
			if check.Link != "" {
				return m, m.openURLInBrowser(check.Link)
			}
		}
	case paneGitStatus:
		if len(m.state.services.statusTree.TreeFlat) > 0 && m.state.services.statusTree.Index >= 0 && m.state.services.statusTree.Index < len(m.state.services.statusTree.TreeFlat) {
			node := m.state.services.statusTree.TreeFlat[m.state.services.statusTree.Index]
			if node.IsDir() {
				m.state.services.statusTree.ToggleCollapse(node.Path)
				m.rebuildStatusContentWithHighlight()
				return m, nil
			}
			return m, m.showFileDiff(*node.File)
		}
	case paneCommit:
		return m, m.openCommitView()
	}
	return m, nil
}
