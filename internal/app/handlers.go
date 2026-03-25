package app

import (
	tea "charm.land/bubbletea/v2"
	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/config"
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
			m.state.ui.filterInput, cmd = m.state.ui.filterInput.Update(msg)
			m.setFilterQuery(filterTargetWorktrees, m.state.ui.filterInput.Value())
			m.updateTable()
			return m, cmd
		case filterTargetStatus, filterTargetGitStatus:
			if keyStr == keyEnter || isEscKey(keyStr) || keyStr == keyCtrlC {
				m.exitFilter()
				return m, nil
			}
			m.state.ui.filterInput, cmd = m.state.ui.filterInput.Update(msg)
			m.setFilterQuery(filterTargetGitStatus, m.state.ui.filterInput.Value())
			m.applyStatusFilter()
			return m, cmd
		case filterTargetLog:
			if keyStr == keyEnter || isEscKey(keyStr) || keyStr == keyCtrlC {
				m.exitFilter()
				return m, nil
			}
			m.state.ui.filterInput, cmd = m.state.ui.filterInput.Update(msg)
			m.setFilterQuery(filterTargetLog, m.state.ui.filterInput.Value())
			m.applyLogFilter(false)
			return m, cmd
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
