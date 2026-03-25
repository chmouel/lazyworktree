package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/chmouel/lazyworktree/internal/app/state"
	"github.com/chmouel/lazyworktree/internal/config"
)

func (m *Model) handlePaneKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	switch msg.String() {
	case "1":
		targetPane := paneWorktrees
		if m.state.view.FocusedPane == targetPane {
			if m.state.view.ZoomedPane >= 0 {
				m.state.view.ZoomedPane = -1
			} else {
				m.state.view.ZoomedPane = targetPane
			}
		} else {
			m.state.view.ZoomedPane = -1
			m.switchPane(targetPane)
			return m, nil, true
		}
		m.restyleLogRows()
		return m, nil, true
	case "2":
		targetPane := paneInfo
		if m.state.view.FocusedPane == targetPane {
			if m.state.view.ZoomedPane >= 0 {
				m.state.view.ZoomedPane = -1
			} else {
				m.state.view.ZoomedPane = targetPane
			}
		} else {
			m.state.view.ZoomedPane = -1
			m.switchPane(targetPane)
			return m, nil, true
		}
		m.rebuildStatusContentWithHighlight()
		m.restyleLogRows()
		return m, nil, true
	case "3":
		if !m.hasGitStatus() {
			return m, nil, true
		}
		targetPane := paneGitStatus
		if m.state.view.FocusedPane == targetPane {
			if m.state.view.ZoomedPane >= 0 {
				m.state.view.ZoomedPane = -1
			} else {
				m.state.view.ZoomedPane = targetPane
			}
		} else {
			m.state.view.ZoomedPane = -1
			m.switchPane(targetPane)
			return m, nil, true
		}
		m.rebuildStatusContentWithHighlight()
		m.restyleLogRows()
		return m, nil, true
	case "4":
		targetPane := paneCommit
		if m.state.view.FocusedPane == targetPane {
			if m.state.view.ZoomedPane >= 0 {
				m.state.view.ZoomedPane = -1
			} else {
				m.state.view.ZoomedPane = targetPane
			}
		} else {
			m.state.view.ZoomedPane = -1
			m.switchPane(targetPane)
			return m, nil, true
		}
		m.restyleLogRows()
		return m, nil, true
	case "5":
		if !m.hasNoteForSelectedWorktree() {
			return m, nil, true
		}
		targetPane := paneNotes
		if m.state.view.FocusedPane == targetPane {
			if m.state.view.ZoomedPane >= 0 {
				m.state.view.ZoomedPane = -1
			} else {
				m.state.view.ZoomedPane = targetPane
			}
		} else {
			m.state.view.ZoomedPane = -1
			m.switchPane(targetPane)
			return m, nil, true
		}
		m.restyleLogRows()
		return m, nil, true
	case "6":
		if !m.hasAgentSessionsForSelectedWorktree() && !m.hasAnyAgentSessionsForSelectedWorktree() {
			return m, nil, true
		}
		if !m.hasAgentSessionsForSelectedWorktree() && m.hasAnyAgentSessionsForSelectedWorktree() {
			m.state.view.ShowAllAgentSessions = true
			m.refreshSelectedWorktreeAgentSessionsPane()
		}
		targetPane := paneAgentSessions
		if m.state.view.FocusedPane == targetPane {
			if m.state.view.ZoomedPane >= 0 {
				m.state.view.ZoomedPane = -1
			} else {
				m.state.view.ZoomedPane = targetPane
			}
		} else {
			m.state.view.ZoomedPane = -1
			m.switchPane(targetPane)
			return m, nil, true
		}
		m.restyleLogRows()
		return m, nil, true
	case keyTab, "]":
		m.state.view.ZoomedPane = -1
		m.switchPane(m.nextPane(m.state.view.FocusedPane, 1))
		return m, nil, true
	case "[":
		m.state.view.ZoomedPane = -1
		m.switchPane(m.nextPane(m.state.view.FocusedPane, -1))
		return m, nil, true
	case "h":
		m.state.view.ResizeOffset -= resizeStep
		if m.state.view.ResizeOffset < -80 {
			m.state.view.ResizeOffset = -80
		}
		m.applyLayout(m.computeLayout())
		return m, nil, true
	case "l":
		m.state.view.ResizeOffset += resizeStep
		if m.state.view.ResizeOffset > 80 {
			m.state.view.ResizeOffset = 80
		}
		m.applyLayout(m.computeLayout())
		return m, nil, true
	case "L":
		if m.state.view.Layout == state.LayoutDefault {
			m.state.view.Layout = state.LayoutTop
		} else {
			m.state.view.Layout = state.LayoutDefault
		}
		m.state.view.ZoomedPane = -1
		m.state.view.ResizeOffset = 0
		return m, nil, true
	case "=":
		if m.state.view.ZoomedPane >= 0 {
			m.state.view.ZoomedPane = -1
		} else {
			m.state.view.ZoomedPane = m.state.view.FocusedPane
		}
		return m, nil, true
	default:
		return m, nil, false
	}
}

func (m *Model) handleNavigationKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	switch msg.String() {
	case "j", "down":
		model, cmd := m.handleNavigationDown(msg)
		return model, cmd, true
	case "k", "up":
		model, cmd := m.handleNavigationUp(msg)
		return model, cmd, true
	case keyCtrlJ:
		if m.state.view.FocusedPane == paneGitStatus && len(m.state.services.statusTree.TreeFlat) > 0 {
			if m.state.services.statusTree.Index < len(m.state.services.statusTree.TreeFlat)-1 {
				m.state.services.statusTree.Index++
			}
			m.rebuildStatusContentWithHighlight()
			node := m.state.services.statusTree.TreeFlat[m.state.services.statusTree.Index]
			if !node.IsDir() {
				return m, m.showFileDiff(*node.File), true
			}
			return m, nil, true
		}
		if m.state.view.FocusedPane == paneCommit {
			prevCursor := m.state.ui.logTable.Cursor()
			_, moveCmd := m.handleNavigationDown(tea.KeyPressMsg{Code: tea.KeyDown})
			if m.state.ui.logTable.Cursor() == prevCursor {
				return m, moveCmd, true
			}
			return m, tea.Batch(moveCmd, m.openCommitView()), true
		}
		if m.state.view.FocusedPane == paneNotes {
			m.state.ui.notesViewport.ScrollDown(1)
			return m, nil, true
		}
		if m.state.view.FocusedPane == paneAgentSessions {
			if m.state.data.agentSessionIndex < len(m.state.data.agentSessions)-1 {
				m.state.data.agentSessionIndex++
			}
			m.refreshSelectedWorktreeAgentSessionsPane()
			return m, nil, true
		}
		return m, nil, true
	case keyCtrlK:
		if m.state.view.FocusedPane == paneGitStatus && len(m.state.services.statusTree.TreeFlat) > 0 {
			if m.state.services.statusTree.Index > 0 {
				m.state.services.statusTree.Index--
			}
			m.rebuildStatusContentWithHighlight()
			node := m.state.services.statusTree.TreeFlat[m.state.services.statusTree.Index]
			if !node.IsDir() {
				return m, m.showFileDiff(*node.File), true
			}
			return m, nil, true
		}
		if m.state.view.FocusedPane == paneNotes {
			m.state.ui.notesViewport.ScrollUp(1)
			return m, nil, true
		}
		if m.state.view.FocusedPane == paneAgentSessions {
			if m.state.data.agentSessionIndex > 0 {
				m.state.data.agentSessionIndex--
			}
			m.refreshSelectedWorktreeAgentSessionsPane()
			return m, nil, true
		}
		return m, nil, true
	case "ctrl+d", "space", "pgdown":
		model, cmd := m.handlePageDown(msg)
		return model, cmd, true
	case "ctrl+u", "pgup":
		model, cmd := m.handlePageUp(msg)
		return model, cmd, true
	case "G":
		if m.state.view.FocusedPane == paneInfo {
			m.state.ui.infoViewport.GotoBottom()
			return m, nil, true
		}
		if m.state.view.FocusedPane == paneGitStatus {
			m.state.ui.statusViewport.GotoBottom()
			if len(m.state.services.statusTree.TreeFlat) > 0 {
				m.state.services.statusTree.Index = len(m.state.services.statusTree.TreeFlat) - 1
			}
			return m, nil, true
		}
		if m.state.view.FocusedPane == paneNotes {
			m.state.ui.notesViewport.GotoBottom()
			return m, nil, true
		}
		if m.state.view.FocusedPane == paneAgentSessions {
			if len(m.state.data.agentSessions) > 0 {
				m.state.data.agentSessionIndex = len(m.state.data.agentSessions) - 1
			}
			m.refreshSelectedWorktreeAgentSessionsPane()
			return m, nil, true
		}
		return m, nil, true
	default:
		return m, nil, false
	}
}

func (m *Model) handleCodeNavigationKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	if msg.Code == tea.KeyHome {
		model, cmd := m.handleGotoTop()
		return model, cmd, true
	}
	if msg.Code == tea.KeyEnd {
		model, cmd := m.handleGotoBottom()
		return model, cmd, true
	}
	if m.state.view.FocusedPane == paneGitStatus {
		if msg.Code == tea.KeyLeft && msg.Mod == tea.ModCtrl {
			model, cmd := m.handlePrevFolder()
			return model, cmd, true
		}
		if msg.Code == tea.KeyRight && msg.Mod == tea.ModCtrl {
			model, cmd := m.handleNextFolder()
			return model, cmd, true
		}
	}
	return m, nil, false
}

func (m *Model) handleGotoTop() (tea.Model, tea.Cmd) {
	switch m.state.view.FocusedPane {
	case paneWorktrees:
		m.state.ui.worktreeTable.GotoTop()
		m.updateWorktreeArrows()
		m.syncSelectedIndexFromCursor()
		return m, m.debouncedUpdateDetailsView()
	case paneInfo:
		m.state.ui.infoViewport.GotoTop()
	case paneGitStatus:
		if len(m.state.services.statusTree.TreeFlat) > 0 {
			m.state.services.statusTree.Index = 0
			m.rebuildStatusContentWithHighlight()
		}
	case paneCommit:
		m.state.ui.logTable.GotoTop()
	case paneNotes:
		m.state.ui.notesViewport.GotoTop()
	case paneAgentSessions:
		m.state.data.agentSessionIndex = 0
		m.refreshSelectedWorktreeAgentSessionsPane()
	}
	return m, nil
}

func (m *Model) handleGotoBottom() (tea.Model, tea.Cmd) {
	switch m.state.view.FocusedPane {
	case paneWorktrees:
		m.state.ui.worktreeTable.GotoBottom()
		m.updateWorktreeArrows()
		m.syncSelectedIndexFromCursor()
		return m, m.debouncedUpdateDetailsView()
	case paneInfo:
		m.state.ui.infoViewport.GotoBottom()
	case paneGitStatus:
		if len(m.state.services.statusTree.TreeFlat) > 0 {
			m.state.services.statusTree.Index = len(m.state.services.statusTree.TreeFlat) - 1
			m.rebuildStatusContentWithHighlight()
		}
	case paneCommit:
		m.state.ui.logTable.GotoBottom()
	case paneNotes:
		m.state.ui.notesViewport.GotoBottom()
	case paneAgentSessions:
		if len(m.state.data.agentSessions) > 0 {
			m.state.data.agentSessionIndex = len(m.state.data.agentSessions) - 1
		}
		m.refreshSelectedWorktreeAgentSessionsPane()
	}
	return m, nil
}

func (m *Model) handleNextFolder() (tea.Model, tea.Cmd) {
	if len(m.state.services.statusTree.TreeFlat) == 0 {
		return m, nil
	}
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
	hasNotes := m.hasNoteForSelectedWorktree()
	hasGitStatus := m.hasGitStatus()
	hasAgentSessions := m.hasAgentSessionsForSelectedWorktree()

	panes := make([]int, 0, 6)
	panes = append(panes, paneWorktrees, paneInfo)
	if hasGitStatus {
		panes = append(panes, paneGitStatus)
	}
	panes = append(panes, paneCommit)
	if hasNotes {
		panes = append(panes, paneNotes)
	}
	if hasAgentSessions {
		panes = append(panes, paneAgentSessions)
	}

	for i, p := range panes {
		if p == current {
			next := (i + direction + len(panes)) % len(panes)
			return panes[next]
		}
	}
	if direction > 0 {
		return panes[0]
	}
	return panes[len(panes)-1]
}

// switchPane updates pane focus and refreshes any cached pane content whose
// rendering depends on focus state.
func (m *Model) switchPane(targetPane int) {
	previousPane := m.state.view.FocusedPane
	if previousPane == targetPane {
		return
	}

	if previousPane == paneInfo && targetPane != paneInfo {
		m.ciCheckIndex = -1
	}

	m.state.view.FocusedPane = targetPane

	switch targetPane {
	case paneWorktrees:
		m.state.ui.worktreeTable.Focus()
	case paneCommit:
		m.state.ui.logTable.Focus()
	}

	if previousPane == paneInfo || targetPane == paneInfo || previousPane == paneGitStatus || targetPane == paneGitStatus {
		m.rebuildStatusContentWithHighlight()
	}
	if previousPane == paneAgentSessions || targetPane == paneAgentSessions {
		m.refreshSelectedWorktreeAgentSessionsPane()
	}
	m.restyleLogRows()
}

// paneIndexToName maps a focused pane index to its canonical name for keybinding lookup.
func paneIndexToName(pane int) string {
	switch pane {
	case paneWorktrees:
		return config.PaneWorktrees
	case paneInfo:
		return config.PaneInfo
	case paneGitStatus:
		return config.PaneStatus
	case paneCommit:
		return config.PaneLog
	case paneNotes:
		return config.PaneNotes
	case paneAgentSessions:
		return config.PaneAgentSessions
	default:
		return config.PaneUniversal
	}
}

// handleNavigationDown processes down arrow and 'j' key navigation.
func (m *Model) handleNavigationDown(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	keyMsg := msg
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch m.state.view.FocusedPane {
	case paneWorktrees:
		m.state.ui.worktreeTable, cmd = m.state.ui.worktreeTable.Update(keyMsg)
		m.updateWorktreeArrows()
		m.syncSelectedIndexFromCursor()
		cmds = append(cmds, cmd)
		cmds = append(cmds, m.debouncedUpdateDetailsView())
	case paneInfo:
		m.state.ui.infoViewport.ScrollDown(1)
	case paneGitStatus:
		if len(m.state.services.statusTree.TreeFlat) > 0 {
			if m.state.services.statusTree.Index < len(m.state.services.statusTree.TreeFlat)-1 {
				m.state.services.statusTree.Index++
			}
			m.rebuildStatusContentWithHighlight()
		}
	case paneNotes:
		m.state.ui.notesViewport.ScrollDown(1)
	case paneAgentSessions:
		if m.state.data.agentSessionIndex < len(m.state.data.agentSessions)-1 {
			m.state.data.agentSessionIndex++
		}
		m.refreshSelectedWorktreeAgentSessionsPane()
	default:
		m.state.ui.logTable, cmd = m.state.ui.logTable.Update(keyMsg)
		m.restyleLogRows()
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

// handleNavigationUp processes up arrow and 'k' key navigation.
func (m *Model) handleNavigationUp(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	cmds := []tea.Cmd{}

	switch m.state.view.FocusedPane {
	case paneWorktrees:
		m.state.ui.worktreeTable, cmd = m.state.ui.worktreeTable.Update(msg)
		m.updateWorktreeArrows()
		m.syncSelectedIndexFromCursor()
		cmds = append(cmds, cmd)
		cmds = append(cmds, m.debouncedUpdateDetailsView())
	case paneInfo:
		m.state.ui.infoViewport.ScrollUp(1)
	case paneGitStatus:
		if len(m.state.services.statusTree.TreeFlat) > 0 {
			if m.state.services.statusTree.Index > 0 {
				m.state.services.statusTree.Index--
			}
			m.rebuildStatusContentWithHighlight()
		}
	case paneNotes:
		m.state.ui.notesViewport.ScrollUp(1)
	case paneAgentSessions:
		if m.state.data.agentSessionIndex > 0 {
			m.state.data.agentSessionIndex--
		}
		m.refreshSelectedWorktreeAgentSessionsPane()
	default:
		m.state.ui.logTable, cmd = m.state.ui.logTable.Update(msg)
		m.restyleLogRows()
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

// handlePageDown processes page down navigation.
func (m *Model) handlePageDown(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch m.state.view.FocusedPane {
	case paneInfo:
		m.state.ui.infoViewport.HalfPageDown()
		return m, nil
	case paneGitStatus:
		m.state.ui.statusViewport.HalfPageDown()
		return m, nil
	case paneCommit:
		m.state.ui.logTable, cmd = m.state.ui.logTable.Update(msg)
		m.restyleLogRows()
		return m, cmd
	case paneNotes:
		m.state.ui.notesViewport.HalfPageDown()
		return m, nil
	case paneAgentSessions:
		if len(m.state.data.agentSessions) > 0 {
			m.state.data.agentSessionIndex = min(len(m.state.data.agentSessions)-1, m.state.data.agentSessionIndex+3)
		}
		m.refreshSelectedWorktreeAgentSessionsPane()
		return m, nil
	}
	return m, nil
}

// handlePageUp processes page up navigation.
func (m *Model) handlePageUp(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch m.state.view.FocusedPane {
	case paneInfo:
		m.state.ui.infoViewport.HalfPageUp()
		return m, nil
	case paneGitStatus:
		m.state.ui.statusViewport.HalfPageUp()
		return m, nil
	case paneCommit:
		m.state.ui.logTable, cmd = m.state.ui.logTable.Update(msg)
		m.restyleLogRows()
		return m, cmd
	case paneNotes:
		m.state.ui.notesViewport.HalfPageUp()
		return m, nil
	case paneAgentSessions:
		m.state.data.agentSessionIndex = max(0, m.state.data.agentSessionIndex-3)
		m.refreshSelectedWorktreeAgentSessionsPane()
		return m, nil
	}
	return m, nil
}
