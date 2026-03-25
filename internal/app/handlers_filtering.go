package app

import (
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/chmouel/lazyworktree/internal/models"
)

func (m *Model) handleFilterKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	switch msg.String() {
	case "f":
		target := filterTargetWorktrees
		switch m.state.view.FocusedPane {
		case paneGitStatus:
			target = filterTargetGitStatus
		case paneCommit:
			target = filterTargetLog
		}
		return m, m.startFilter(target), true
	case keyEsc, keyEscRaw:
		if m.hasActiveFilterForPane(m.state.view.FocusedPane) {
			model, cmd := m.clearCurrentPaneFilter()
			return model, cmd, true
		}
		return m, nil, true
	default:
		return m, nil, false
	}
}

func (m *Model) exitFilter() {
	m.state.view.ShowingFilter = false
	m.state.ui.filterInput.Blur()
	m.restoreFocusAfterFilter()
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
	case paneWorktrees:
		m.state.services.filter.FilterQuery = ""
		m.state.ui.filterInput.SetValue("")
		m.updateTable()
	case paneGitStatus:
		m.state.services.filter.StatusFilterQuery = ""
		m.state.ui.filterInput.SetValue("")
		m.applyStatusFilter()
	case paneCommit:
		m.state.services.filter.LogFilterQuery = ""
		m.state.ui.filterInput.SetValue("")
		m.applyLogFilter(false)
	}
	return m, nil
}

func (m *Model) handleFilterNavigation(keyStr string, fillInput bool) (tea.Model, tea.Cmd) {
	var workList []*models.WorktreeInfo
	if fillInput {
		workList = make([]*models.WorktreeInfo, len(m.state.data.worktrees))
		copy(workList, m.state.data.worktrees)
		sortWorktrees(workList, m.sortMode)
	} else {
		workList = m.state.data.filteredWts
	}

	if len(workList) == 0 {
		return m, nil
	}

	currentPath := ""
	if !fillInput {
		currentIndex := m.state.ui.worktreeTable.Cursor()
		if currentIndex >= 0 && currentIndex < len(m.state.data.filteredWts) {
			currentPath = m.state.data.filteredWts[currentIndex].Path
		}
	} else {
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
