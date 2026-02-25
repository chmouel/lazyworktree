package app

import (
	"fmt"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
)

// yankContextual copies context-aware content based on the focused pane.
func (m *Model) yankContextual() tea.Cmd {
	switch m.state.view.FocusedPane {
	case 0: // Worktrees: copy worktree path
		if m.state.data.selectedIndex >= 0 && m.state.data.selectedIndex < len(m.state.data.filteredWts) {
			wt := m.state.data.filteredWts[m.state.data.selectedIndex]
			m.showInfo(fmt.Sprintf("Copied path: %s", wt.Path), nil)
			return tea.SetClipboard(wt.Path)
		}
	case 1: // Status: copy selected file path
		if len(m.state.services.statusTree.TreeFlat) > 0 &&
			m.state.services.statusTree.Index >= 0 &&
			m.state.services.statusTree.Index < len(m.state.services.statusTree.TreeFlat) {
			node := m.state.services.statusTree.TreeFlat[m.state.services.statusTree.Index]
			if !node.IsDir() && node.File != nil {
				// Build absolute path using worktree path
				var absPath string
				if m.state.data.selectedIndex >= 0 && m.state.data.selectedIndex < len(m.state.data.filteredWts) {
					absPath = filepath.Join(m.state.data.filteredWts[m.state.data.selectedIndex].Path, node.File.Filename)
				} else {
					absPath = node.File.Filename
				}
				m.showInfo(fmt.Sprintf("Copied file: %s", node.File.Filename), nil)
				return tea.SetClipboard(absPath)
			}
		}
	case 2: // Log: copy selected commit SHA
		cursor := m.state.ui.logTable.Cursor()
		if len(m.state.data.logEntries) > 0 && cursor >= 0 && cursor < len(m.state.data.logEntries) {
			sha := m.state.data.logEntries[cursor].sha
			m.showInfo(fmt.Sprintf("Copied SHA: %s", sha), nil)
			return tea.SetClipboard(sha)
		}
	}
	return nil
}

// yankBranch copies the selected worktree's branch name.
func (m *Model) yankBranch() tea.Cmd {
	if m.state.data.selectedIndex >= 0 && m.state.data.selectedIndex < len(m.state.data.filteredWts) {
		wt := m.state.data.filteredWts[m.state.data.selectedIndex]
		if wt.Branch != "" {
			m.showInfo(fmt.Sprintf("Copied branch: %s", wt.Branch), nil)
			return tea.SetClipboard(wt.Branch)
		}
	}
	return nil
}

// yankPRURL copies the selected worktree's PR/MR URL.
func (m *Model) yankPRURL() tea.Cmd {
	if m.state.data.selectedIndex >= 0 && m.state.data.selectedIndex < len(m.state.data.filteredWts) {
		wt := m.state.data.filteredWts[m.state.data.selectedIndex]
		if wt.PR != nil && wt.PR.URL != "" {
			m.showInfo(fmt.Sprintf("Copied PR URL: %s", wt.PR.URL), nil)
			return tea.SetClipboard(wt.PR.URL)
		}
	}
	return nil
}
