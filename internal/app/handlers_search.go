package app

import tea "charm.land/bubbletea/v2"

func (m *Model) handleSearchKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	switch msg.String() {
	case "/":
		target := searchTargetWorktrees
		switch m.state.view.FocusedPane {
		case paneGitStatus:
			target = searchTargetGitStatus
		case paneCommit:
			target = searchTargetLog
		}
		return m, m.startSearch(target), true
	case "n":
		if m.state.view.FocusedPane == paneInfo {
			model, cmd := m.navigateCICheckDown()
			return model, cmd, true
		}
		return m, m.advanceSearchMatch(true), true
	case "N":
		return m, m.advanceSearchMatch(false), true
	case "p":
		if m.state.view.FocusedPane == paneInfo {
			model, cmd := m.navigateCICheckUp()
			return model, cmd, true
		}
		return m, nil, true
	default:
		return m, nil, false
	}
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
