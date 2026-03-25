package app

import (
	"time"

	tea "charm.land/bubbletea/v2"
	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/app/state"
)

// handleMouseClick processes mouse click events for pane focus and item selection.
func (m *Model) handleMouseClick(msg tea.MouseClickMsg) (tea.Model, tea.Cmd) {
	if m.state.ui.screenManager.IsActive() {
		return m, nil
	}

	if msg.Button != tea.MouseLeft {
		return m, nil
	}

	var cmds []tea.Cmd
	layout := m.computeLayout()

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

		agentY := topMaxY + layout.gapY
		agentMaxY := agentY + layout.agentRowHeight

		notesY := agentY
		if layout.hasAgentSessions {
			notesY = agentMaxY + layout.gapY
		}
		notesMaxY := notesY + layout.notesRowHeight

		var bottomY int
		switch {
		case layout.hasNotes:
			bottomY = notesMaxY + layout.gapY
		case layout.hasAgentSessions:
			bottomY = agentMaxY + layout.gapY
		default:
			bottomY = topMaxY + layout.gapY
		}
		bottomMaxY := headerOffset + layout.bodyHeight
		bottomLeftMaxX := layout.bottomLeftWidth
		bottomMiddleX := layout.bottomLeftWidth + layout.gapX
		bottomMiddleMaxX := bottomMiddleX + layout.bottomMiddleWidth
		bottomRightX := bottomMiddleMaxX + layout.gapX

		switch {
		case mouseY >= topY && mouseY < topMaxY:
			targetPane = paneWorktrees
		case layout.hasAgentSessions && mouseY >= agentY && mouseY < agentMaxY:
			targetPane = paneAgentSessions
		case layout.hasNotes && mouseY >= notesY && mouseY < notesMaxY:
			targetPane = paneNotes
		case mouseX < bottomLeftMaxX && mouseY >= bottomY && mouseY < bottomMaxY:
			targetPane = paneInfo
		case mouseX >= bottomMiddleX && mouseX < bottomMiddleMaxX && mouseY >= bottomY && mouseY < bottomMaxY:
			targetPane = paneGitStatus
		case mouseX >= bottomRightX && mouseY >= bottomY && mouseY < bottomMaxY:
			targetPane = paneCommit
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

		if (layout.hasAgentSessions || layout.hasNotes) && mouseX < leftMaxX {
			leftTopY := headerOffset
			leftTopMaxY := headerOffset + layout.leftTopHeight
			leftMiddleY := leftTopMaxY + layout.gapY
			leftMiddleMaxY := leftMiddleY + layout.leftMiddleHeight
			leftBottomY := leftMiddleY
			if layout.hasAgentSessions {
				leftBottomY = leftMiddleMaxY + layout.gapY
			}
			leftBottomMaxY := headerOffset + layout.bodyHeight

			switch {
			case mouseY >= leftTopY && mouseY < leftTopMaxY:
				targetPane = paneWorktrees
			case layout.hasAgentSessions && mouseY >= leftMiddleY && mouseY < leftMiddleMaxY:
				targetPane = paneAgentSessions
			case layout.hasNotes && mouseY >= leftBottomY && mouseY < leftBottomMaxY:
				targetPane = paneNotes
			}
		} else {
			switch {
			case mouseX < leftMaxX && mouseY >= headerOffset && mouseY < headerOffset+layout.bodyHeight:
				targetPane = paneWorktrees
			case mouseX >= rightX && mouseX < rightTopMaxX && mouseY >= rightTopY && mouseY < rightTopMaxY:
				targetPane = paneInfo
			case mouseX >= rightX && mouseX < rightTopMaxX && mouseY >= rightMiddleY && mouseY < rightMiddleMaxY:
				targetPane = paneGitStatus
			case mouseX >= rightX && mouseX < rightTopMaxX && mouseY >= rightBottomY && mouseY < rightBottomMaxY:
				targetPane = paneCommit
			}
		}
	}

	if targetPane >= 0 {
		now := time.Now()
		if targetPane == m.lastClickPane && now.Sub(m.lastClickTime) < 400*time.Millisecond {
			if m.state.view.ZoomedPane >= 0 {
				m.state.view.ZoomedPane = -1
			} else {
				m.state.view.ZoomedPane = targetPane
			}
			m.lastClickTime = time.Time{}
		} else {
			m.lastClickTime = now
		}
		m.lastClickPane = targetPane
	}

	if targetPane >= 0 && targetPane != m.state.view.FocusedPane {
		m.switchPane(targetPane)
	}

	if targetPane == paneWorktrees && len(m.state.data.filteredWts) > 0 {
		paneTopY := headerOffset
		relativeY := mouseY - paneTopY - 4
		if relativeY >= 0 && relativeY < len(m.state.data.filteredWts) {
			m.state.ui.worktreeTable.SetCursor(relativeY)
			m.state.data.selectedIndex = relativeY
			m.updateWorktreeArrows()
			cmds = append(cmds, m.debouncedUpdateDetailsView())
		}
	} else if targetPane == paneCommit && len(m.state.data.logEntries) > 0 {
		var logPaneTopY int
		if layout.layoutMode == state.LayoutTop {
			logPaneTopY = headerOffset + layout.topHeight + layout.gapY
			if layout.hasAgentSessions {
				logPaneTopY += layout.agentRowHeight + layout.gapY
			}
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

	if m.state.ui.screenManager.IsActive() {
		return m, nil
	}

	var cmds []tea.Cmd
	layout := m.computeLayout()

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

		agentY := topMaxY + layout.gapY
		agentMaxY := agentY + layout.agentRowHeight
		notesY := agentY
		if layout.hasAgentSessions {
			notesY = agentMaxY + layout.gapY
		}
		notesMaxY := notesY + layout.notesRowHeight

		var bottomY int
		switch {
		case layout.hasNotes:
			bottomY = notesMaxY + layout.gapY
		case layout.hasAgentSessions:
			bottomY = agentMaxY + layout.gapY
		default:
			bottomY = topMaxY + layout.gapY
		}
		bottomMaxY := headerOffset + layout.bodyHeight
		bottomLeftMaxX := layout.bottomLeftWidth
		bottomMiddleX := layout.bottomLeftWidth + layout.gapX
		bottomMiddleMaxX := bottomMiddleX + layout.bottomMiddleWidth
		bottomRightX := bottomMiddleMaxX + layout.gapX

		switch {
		case mouseY >= topY && mouseY < topMaxY:
			targetPane = paneWorktrees
		case layout.hasAgentSessions && mouseY >= agentY && mouseY < agentMaxY:
			targetPane = paneAgentSessions
		case layout.hasNotes && mouseY >= notesY && mouseY < notesMaxY:
			targetPane = paneNotes
		case mouseX < bottomLeftMaxX && mouseY >= bottomY && mouseY < bottomMaxY:
			targetPane = paneInfo
		case mouseX >= bottomMiddleX && mouseX < bottomMiddleMaxX && mouseY >= bottomY && mouseY < bottomMaxY:
			targetPane = paneGitStatus
		case mouseX >= bottomRightX && mouseY >= bottomY && mouseY < bottomMaxY:
			targetPane = paneCommit
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

		if (layout.hasAgentSessions || layout.hasNotes) && mouseX < leftMaxX {
			leftTopY := headerOffset
			leftTopMaxY := headerOffset + layout.leftTopHeight
			leftMiddleY := leftTopMaxY + layout.gapY
			leftMiddleMaxY := leftMiddleY + layout.leftMiddleHeight
			leftBottomY := leftMiddleY
			if layout.hasAgentSessions {
				leftBottomY = leftMiddleMaxY + layout.gapY
			}
			leftBottomMaxY := headerOffset + layout.bodyHeight

			switch {
			case mouseY >= leftTopY && mouseY < leftTopMaxY:
				targetPane = paneWorktrees
			case layout.hasAgentSessions && mouseY >= leftMiddleY && mouseY < leftMiddleMaxY:
				targetPane = paneAgentSessions
			case layout.hasNotes && mouseY >= leftBottomY && mouseY < leftBottomMaxY:
				targetPane = paneNotes
			}
		} else {
			switch {
			case mouseX < leftMaxX && mouseY >= headerOffset && mouseY < headerOffset+layout.bodyHeight:
				targetPane = paneWorktrees
			case mouseX >= rightX && mouseX < rightTopMaxX && mouseY >= rightTopY && mouseY < rightTopMaxY:
				targetPane = paneInfo
			case mouseX >= rightX && mouseX < rightTopMaxX && mouseY >= rightMiddleY && mouseY < rightMiddleMaxY:
				targetPane = paneGitStatus
			case mouseX >= rightX && mouseX < rightTopMaxX && mouseY >= rightBottomY && mouseY < rightBottomMaxY:
				targetPane = paneCommit
			}
		}
	}

	switch msg.Button {
	case tea.MouseWheelUp:
		switch targetPane {
		case paneWorktrees:
			m.state.ui.worktreeTable, _ = m.state.ui.worktreeTable.Update(tea.KeyPressMsg{Code: tea.KeyUp})
			m.updateWorktreeArrows()
			m.syncSelectedIndexFromCursor()
			cmds = append(cmds, m.debouncedUpdateDetailsView())
		case paneInfo:
			m.state.ui.infoViewport.ScrollUp(3)
		case paneGitStatus:
			if len(m.state.services.statusTree.TreeFlat) > 0 && m.state.services.statusTree.Index > 0 {
				m.state.services.statusTree.Index--
				m.rebuildStatusContentWithHighlight()
			}
		case paneCommit:
			m.state.ui.logTable, _ = m.state.ui.logTable.Update(tea.KeyPressMsg{Code: tea.KeyUp})
			m.restyleLogRows()
		case paneNotes:
			m.state.ui.notesViewport.ScrollUp(3)
		case paneAgentSessions:
			m.state.data.agentSessionIndex = max(0, m.state.data.agentSessionIndex-1)
			m.refreshSelectedWorktreeAgentSessionsPane()
		}
	case tea.MouseWheelDown:
		switch targetPane {
		case paneWorktrees:
			m.state.ui.worktreeTable, _ = m.state.ui.worktreeTable.Update(tea.KeyPressMsg{Code: tea.KeyDown})
			m.updateWorktreeArrows()
			m.syncSelectedIndexFromCursor()
			cmds = append(cmds, m.debouncedUpdateDetailsView())
		case paneInfo:
			m.state.ui.infoViewport.ScrollDown(3)
		case paneGitStatus:
			if len(m.state.services.statusTree.TreeFlat) > 0 && m.state.services.statusTree.Index < len(m.state.services.statusTree.TreeFlat)-1 {
				m.state.services.statusTree.Index++
				m.rebuildStatusContentWithHighlight()
			}
		case paneCommit:
			m.state.ui.logTable, _ = m.state.ui.logTable.Update(tea.KeyPressMsg{Code: tea.KeyDown})
			m.restyleLogRows()
		case paneNotes:
			m.state.ui.notesViewport.ScrollDown(3)
		case paneAgentSessions:
			if m.state.data.agentSessionIndex < len(m.state.data.agentSessions)-1 {
				m.state.data.agentSessionIndex++
			}
			m.refreshSelectedWorktreeAgentSessionsPane()
		}
	}

	return m, tea.Batch(cmds...)
}
