package app

import (
	tea "charm.land/bubbletea/v2"
	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/config"
)

// handlePasteMsg routes bracketed-paste content to the focused text input:
// a modal screen, the search box, or the filter box. Paste with nothing
// focused is dropped, like a stray key press.
func (m *Model) handlePasteMsg(msg tea.PasteMsg) (tea.Model, tea.Cmd) {
	if m.state.ui.screenManager.IsActive() {
		return m.handleScreenKey(msg)
	}

	if m.state.view.ShowingSearch {
		return m.handleSearchInput(msg)
	}

	if m.state.view.ShowingFilter {
		return m.applyFilterInputUpdate(msg)
	}

	return m, nil
}

// applyFilterInputUpdate feeds a message into the filter box and reapplies
// the resulting query to whichever pane is currently being filtered.
func (m *Model) applyFilterInputUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.state.ui.filterInput, cmd = m.state.ui.filterInput.Update(msg)
	switch m.state.view.FilterTarget {
	case filterTargetWorktrees:
		m.setFilterQuery(filterTargetWorktrees, m.state.ui.filterInput.Value())
		m.updateTable()
	case filterTargetStatus, filterTargetGitStatus:
		m.setFilterQuery(filterTargetGitStatus, m.state.ui.filterInput.Value())
		m.applyStatusFilter()
	case filterTargetLog:
		m.setFilterQuery(filterTargetLog, m.state.ui.filterInput.Value())
		m.applyLogFilter(false)
	}
	return m, cmd
}

// handleKeyMsg processes keyboard input when not in a modal screen.
func (m *Model) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.state.view.ShowingSearch {
		return m.handleSearchInput(msg)
	}

	// Handle filter input first - when filtering, only escape/enter should exit
	if m.state.view.ShowingFilter {
		keyStr := msg.String()
		switch m.state.view.FilterTarget {
		case filterTargetWorktrees:
			if keyStr == keyEnter || isEscKey(keyStr) || keyStr == keyCtrlC {
				m.exitFilter()
				return m, nil
			}
			if keyStr == "alt+n" || keyStr == "alt+p" {
				return m.handleFilterNavigation(keyStr, true)
			}
			if keyStr == keyUp || keyStr == keyDown || keyStr == keyCtrlK || keyStr == keyCtrlJ {
				return m.handleFilterNavigation(keyStr, false)
			}
			return m.applyFilterInputUpdate(msg)
		case filterTargetStatus, filterTargetGitStatus, filterTargetLog:
			if keyStr == keyEnter || isEscKey(keyStr) || keyStr == keyCtrlC {
				m.exitFilter()
				return m, nil
			}
			return m.applyFilterInputUpdate(msg)
		}
	}

	paneName := paneIndexToName(m.state.view.FocusedPane)

	// Pane-specific keybinding → universal keybinding
	if actionID, ok := m.config.Keybindings.Lookup(paneName, msg.String()); ok {
		return m, m.executeRegistryAction(actionID)
	}

	// Pane-specific custom command → universal custom command
	if cmd, ok := m.config.CustomCommands.Lookup(paneName, msg.String()); ok && config.CustomCommandHasKeyBinding(msg.String()) {
		return m, m.executeCustomCommandDirect(cmd)
	}

	return m.handleBuiltInKey(msg)
}

func (m *Model) handleGlobalKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	switch msg.String() {
	case keyCtrlG:
		if m.state.ui.screenManager.Type() == appscreen.TypeCommitMessage {
			return m, nil, true
		}
		return m, m.commitStagedChanges(), true
	default:
		return m, nil, false
	}
}
