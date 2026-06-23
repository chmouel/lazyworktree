package app

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/chmouel/lazyworktree/internal/app/state"
)

// renderBody renders the main body area with panes.
func (m *Model) renderBody(layout layoutDims) string {
	// Handle zoom mode: only render the zoomed pane (layout agnostic)
	if m.state.view.ZoomedPane >= 0 {
		// If zoomed on git status pane but it's hidden, reset zoom
		if m.state.view.ZoomedPane == paneGitStatus && !layout.hasGitStatus {
			m.state.view.ZoomedPane = -1
		} else {
			switch m.state.view.ZoomedPane {
			case paneWorktrees:
				return m.renderZoomedLeftPane(layout)
			case paneInfo:
				return m.renderZoomedRightTopPane(layout)
			case paneGitStatus:
				return m.renderZoomedRightMiddlePane(layout)
			case paneCommit:
				return m.renderZoomedRightBottomPane(layout)
			case paneNotes:
				return m.renderZoomedNotesPane(layout)
			case paneAgentSessions:
				return m.renderZoomedAgentSessionsPane(layout)
			}
		}
	}

	if layout.layoutMode == state.LayoutTop {
		return m.renderTopLayoutBody(layout)
	}

	left := m.renderLeftPane(layout)
	right := m.renderRightPane(layout)
	gap := lipgloss.NewStyle().
		Width(layout.gapX).
		Render(strings.Repeat(" ", layout.gapX))
	return lipgloss.JoinHorizontal(lipgloss.Top, left, gap, right)
}

// renderLeftPane renders the left pane, stacking worktrees with optional agent sessions and notes.
func (m *Model) renderLeftPane(layout layoutDims) string {
	if layout.hasAgentSessions || layout.hasNotes {
		parts := make([]string, 0, 3)
		wtFocused := m.state.view.FocusedPane == paneWorktrees
		parts = append(parts, m.renderPaneBlock(1, "Worktrees", wtFocused, layout.leftWidth, layout.leftTopHeight, m.state.ui.worktreeTable.View()))
		if layout.hasAgentSessions {
			agentFocused := m.state.view.FocusedPane == paneAgentSessions
			agentBox := m.renderAgentSessionsBox(layout.leftInnerWidth, layout.leftMiddleInnerHeight)
			parts = append(parts, m.renderPaneBlock(6, "Agent Sessions", agentFocused, layout.leftWidth, layout.leftMiddleHeight, agentBox))
		}
		if layout.hasNotes {
			notesFocused := m.state.view.FocusedPane == paneNotes
			notesBox := m.renderNotesBox(layout.leftInnerWidth, layout.leftBottomInnerHeight)
			parts = append(parts, m.renderPaneBlock(5, "Notes", notesFocused, layout.leftWidth, layout.leftBottomHeight, notesBox))
		}
		gap := strings.Repeat("\n", layout.gapY)
		return strings.Join(parts, gap)
	}
	focused := m.state.view.FocusedPane == paneWorktrees
	return m.renderPaneBlock(1, "Worktrees", focused, layout.leftWidth, layout.bodyHeight, m.state.ui.worktreeTable.View())
}

// renderRightPane renders the right pane container (status + git status + commit).
func (m *Model) renderRightPane(layout layoutDims) string {
	top := m.renderRightTopPane(layout)
	bottom := m.renderRightBottomPane(layout)
	gap := strings.Repeat("\n", layout.gapY)
	if !layout.hasGitStatus {
		return lipgloss.JoinVertical(lipgloss.Left, top, gap, bottom)
	}
	middle := m.renderRightMiddlePane(layout)
	return lipgloss.JoinVertical(lipgloss.Left, top, gap, middle, gap, bottom)
}

// renderRightTopPane renders the right top pane (info box only).
func (m *Model) renderRightTopPane(layout layoutDims) string {
	focused := m.state.view.FocusedPane == paneInfo

	infoBox := m.renderInfoBox(layout.rightInnerWidth, layout.rightTopInnerHeight)
	return m.renderPaneBlock(2, "Info", focused, layout.rightWidth, layout.rightTopHeight, infoBox)
}

// renderInfoBox renders the Info content using a viewport for scrolling.
func (m *Model) renderInfoBox(width, height int) string {
	content := m.infoContent
	if content == "" {
		content = "No data available."
	}

	titleStyle := lipgloss.NewStyle().Foreground(m.theme.MutedFg).Bold(true)
	innerBoxStyle := m.baseInnerBoxStyle()
	title := titleStyle.Render("Info")
	if wt := m.selectedWorktree(); wt != nil && wt.PR != nil && !m.config.DisablePR {
		hidePRDetails := wt.IsMain && (wt.PR.State == prStateMerged || wt.PR.State == prStateClosed)
		if !hidePRDetails {
			showNerdFontIcons := m.config.NerdFontIconsEnabled()
			badge := m.renderPRStateBadge(wt.PR.State, showNerdFontIcons)
			if badge != "" {
				if remoteIcon := prRemoteIcon(wt.PR.URL, showNerdFontIcons); remoteIcon != "" {
					badge = remoteIcon + "  " + badge
				}
				title = lipgloss.JoinHorizontal(lipgloss.Left, title, "  ", badge)
			}
		}
	}

	// Title takes 1 line
	vpWidth := max(1, width-innerBoxStyle.GetHorizontalFrameSize())
	vpHeight := max(1, height-innerBoxStyle.GetVerticalFrameSize()-1)

	m.state.ui.infoViewport.SetWidth(vpWidth)
	m.state.ui.infoViewport.SetHeight(vpHeight)
	m.state.ui.infoViewport.SetContent(content)

	boxContent := lipgloss.JoinVertical(lipgloss.Left, title, m.state.ui.infoViewport.View())

	return innerBoxStyle.
		Width(width).
		Height(height).
		Render(boxContent)
}

// renderRightMiddlePane renders the right middle pane (git status file tree).
func (m *Model) renderRightMiddlePane(layout layoutDims) string {
	focused := m.state.view.FocusedPane == paneGitStatus

	innerBoxStyle := m.baseInnerBoxStyle()
	statusViewportWidth := max(1, layout.rightInnerWidth-innerBoxStyle.GetHorizontalFrameSize())
	statusViewportHeight := max(1, layout.rightMiddleInnerHeight-innerBoxStyle.GetVerticalFrameSize())
	m.state.ui.statusViewport.SetWidth(statusViewportWidth)
	m.state.ui.statusViewport.SetHeight(statusViewportHeight)
	m.state.ui.statusViewport.SetContent(m.statusContent)
	statusBox := innerBoxStyle.
		Width(layout.rightInnerWidth).
		Height(layout.rightMiddleInnerHeight).
		Render(m.state.ui.statusViewport.View())

	return m.renderPaneBlock(3, "Git Status", focused, layout.rightWidth, layout.rightMiddleHeight, statusBox)
}

// renderRightBottomPane renders the right bottom pane (commit log table).
func (m *Model) renderRightBottomPane(layout layoutDims) string {
	focused := m.state.view.FocusedPane == paneCommit
	return m.renderPaneBlock(4, "Commit", focused, layout.rightWidth, layout.rightBottomHeight, m.state.ui.logTable.View())
}

// renderTopLayoutBody renders the body for the top layout mode.
func (m *Model) renderTopLayoutBody(layout layoutDims) string {
	top := m.renderTopPane(layout)
	bottom := m.renderBottomPane(layout)
	gap := strings.Repeat("\n", layout.gapY)
	if layout.hasAgentSessions || layout.hasNotes {
		parts := []string{top}
		if layout.hasAgentSessions {
			agentFocused := m.state.view.FocusedPane == paneAgentSessions
			agentBox := m.renderAgentSessionsBox(layout.agentRowInnerWidth, layout.agentRowInnerHeight)
			parts = append(parts, m.renderPaneBlock(6, "Agent Sessions", agentFocused, layout.width, layout.agentRowHeight, agentBox))
		}
		if layout.hasNotes {
			notesFocused := m.state.view.FocusedPane == paneNotes
			notesBox := m.renderNotesBox(layout.notesRowInnerWidth, layout.notesRowInnerHeight)
			parts = append(parts, m.renderPaneBlock(5, "Notes", notesFocused, layout.width, layout.notesRowHeight, notesBox))
		}
		parts = append(parts, bottom)
		return strings.Join(parts, gap)
	}
	return lipgloss.JoinVertical(lipgloss.Left, top, gap, bottom)
}

// renderTopPane renders the full-width worktree pane at the top.
func (m *Model) renderTopPane(layout layoutDims) string {
	focused := m.state.view.FocusedPane == paneWorktrees
	return m.renderPaneBlock(1, "Worktrees", focused, layout.width, layout.topHeight, m.state.ui.worktreeTable.View())
}

// renderBottomPane renders the bottom pane container (status + git status + commit side by side).
func (m *Model) renderBottomPane(layout layoutDims) string {
	left := m.renderBottomLeftPane(layout)
	right := m.renderBottomRightPane(layout)
	gap := lipgloss.NewStyle().
		Width(layout.gapX).
		Render(strings.Repeat(" ", layout.gapX))
	if !layout.hasGitStatus {
		return lipgloss.JoinHorizontal(lipgloss.Top, left, gap, right)
	}
	middle := m.renderBottomMiddlePane(layout)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, gap, middle, gap, right)
}

// renderBottomLeftPane renders the status (info) pane in the bottom left of the top layout.
func (m *Model) renderBottomLeftPane(layout layoutDims) string {
	focused := m.state.view.FocusedPane == paneInfo

	infoBox := m.renderInfoBox(layout.bottomLeftInnerWidth, layout.bottomLeftInnerHeight)
	return m.renderPaneBlock(2, "Info", focused, layout.bottomLeftWidth, layout.bottomHeight, infoBox)
}

// renderBottomMiddlePane renders the git status pane in the bottom middle of the top layout.
func (m *Model) renderBottomMiddlePane(layout layoutDims) string {
	focused := m.state.view.FocusedPane == paneGitStatus

	innerBoxStyle := m.baseInnerBoxStyle()
	statusViewportWidth := max(1, layout.bottomMiddleInnerWidth-innerBoxStyle.GetHorizontalFrameSize())
	statusViewportHeight := max(1, layout.bottomMiddleInnerHeight-innerBoxStyle.GetVerticalFrameSize())
	m.state.ui.statusViewport.SetWidth(statusViewportWidth)
	m.state.ui.statusViewport.SetHeight(statusViewportHeight)
	m.state.ui.statusViewport.SetContent(m.statusContent)
	statusBox := innerBoxStyle.
		Width(layout.bottomMiddleInnerWidth).
		Height(layout.bottomMiddleInnerHeight).
		Render(m.state.ui.statusViewport.View())

	return m.renderPaneBlock(3, "Git Status", focused, layout.bottomMiddleWidth, layout.bottomHeight, statusBox)
}

// renderBottomRightPane renders the commit pane in the bottom right of the top layout.
func (m *Model) renderBottomRightPane(layout layoutDims) string {
	focused := m.state.view.FocusedPane == paneCommit
	return m.renderPaneBlock(4, "Commit", focused, layout.bottomRightWidth, layout.bottomHeight, m.state.ui.logTable.View())
}

// renderZoomedLeftPane renders the zoomed left pane.
func (m *Model) renderZoomedLeftPane(layout layoutDims) string {
	return m.renderPaneBlock(1, "Worktrees", true, layout.leftWidth, layout.bodyHeight, m.state.ui.worktreeTable.View())
}

// renderZoomedRightTopPane renders the zoomed right top pane (info only).
func (m *Model) renderZoomedRightTopPane(layout layoutDims) string {
	infoBox := m.renderInfoBox(layout.rightInnerWidth, layout.rightTopInnerHeight)
	return m.renderPaneBlock(2, "Status", true, layout.rightWidth, layout.bodyHeight, infoBox)
}

// renderZoomedRightMiddlePane renders the zoomed right middle pane (git status file tree).
func (m *Model) renderZoomedRightMiddlePane(layout layoutDims) string {
	innerBoxStyle := m.baseInnerBoxStyle()
	statusViewportWidth := max(1, layout.rightInnerWidth-innerBoxStyle.GetHorizontalFrameSize())
	statusViewportHeight := max(1, layout.rightMiddleInnerHeight-innerBoxStyle.GetVerticalFrameSize())
	m.state.ui.statusViewport.SetWidth(statusViewportWidth)
	m.state.ui.statusViewport.SetHeight(statusViewportHeight)
	m.state.ui.statusViewport.SetContent(m.statusContent)
	statusBox := innerBoxStyle.
		Width(layout.rightInnerWidth).
		Height(layout.rightMiddleInnerHeight).
		Render(m.state.ui.statusViewport.View())

	return m.renderPaneBlock(3, "Git Status", true, layout.rightWidth, layout.bodyHeight, statusBox)
}

// renderZoomedRightBottomPane renders the zoomed right bottom pane (commit log).
func (m *Model) renderZoomedRightBottomPane(layout layoutDims) string {
	return m.renderPaneBlock(4, "Commit", true, layout.rightWidth, layout.bodyHeight, m.state.ui.logTable.View())
}

// renderZoomedNotesPane renders the zoomed notes pane.
func (m *Model) renderZoomedNotesPane(layout layoutDims) string {
	notesBox := m.renderNotesBox(layout.leftInnerWidth, layout.leftTopInnerHeight)
	return m.renderPaneBlock(5, "Notes", true, layout.leftWidth, layout.bodyHeight, notesBox)
}

// renderZoomedAgentSessionsPane renders the zoomed agent sessions pane.
func (m *Model) renderZoomedAgentSessionsPane(layout layoutDims) string {
	agentBox := m.renderAgentSessionsBox(layout.leftInnerWidth, layout.leftTopInnerHeight)
	return m.renderPaneBlock(6, "Agent Sessions", true, layout.leftWidth, layout.bodyHeight, agentBox)
}
