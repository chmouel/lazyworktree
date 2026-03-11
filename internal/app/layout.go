package app

import (
	"charm.land/bubbles/v2/table"
	"github.com/chmouel/lazyworktree/internal/app/state"
)

// layoutDims holds computed layout dimensions for the UI.
type layoutDims struct {
	width                  int
	height                 int
	headerHeight           int
	footerHeight           int
	filterHeight           int
	bodyHeight             int
	gapX                   int
	gapY                   int
	leftWidth              int
	rightWidth             int
	leftInnerWidth         int
	rightInnerWidth        int
	leftInnerHeight        int
	rightTopHeight         int
	rightMiddleHeight      int
	rightBottomHeight      int
	rightTopInnerHeight    int
	rightMiddleInnerHeight int
	rightBottomInnerHeight int

	// Git status pane visibility
	hasGitStatus bool

	// Notes pane (default layout: left column split)
	hasNotes              bool
	hasAgentSessions      bool
	leftTopHeight         int
	leftMiddleHeight      int
	leftBottomHeight      int
	leftTopInnerHeight    int
	leftMiddleInnerHeight int
	leftBottomInnerHeight int

	// Notes pane (top layout: dedicated row)
	agentRowHeight      int
	agentRowInnerHeight int
	agentRowInnerWidth  int
	notesRowHeight      int
	notesRowInnerHeight int
	notesRowInnerWidth  int

	// Top layout fields
	layoutMode              state.LayoutMode
	topHeight               int
	topInnerWidth           int
	topInnerHeight          int
	bottomHeight            int
	bottomLeftWidth         int
	bottomMiddleWidth       int
	bottomRightWidth        int
	bottomLeftInnerWidth    int
	bottomMiddleInnerWidth  int
	bottomRightInnerWidth   int
	bottomLeftInnerHeight   int
	bottomMiddleInnerHeight int
	bottomRightInnerHeight  int
}

// layoutRatio returns the user-configured ratio for a named pane,
// falling back to the provided default when LayoutSizes is nil or the
// field is unset (zero).
func (m *Model) layoutRatio(pane string, defaultVal float64) float64 {
	ls := m.config.LayoutSizes
	if ls == nil {
		return defaultVal
	}
	var v int
	switch pane {
	case "worktrees":
		v = ls.Worktrees
	case "info":
		v = ls.Info
	case "git_status":
		v = ls.GitStatus
	case "commit":
		v = ls.Commit
	case "notes":
		v = ls.Notes
	case "agent_sessions":
		v = ls.AgentSessions
	}
	if v <= 0 {
		return defaultVal
	}
	return float64(v) / 100.0
}

// normaliseRightRatios returns normalised ratios for info, gitStatus and commit
// panes from user config (or the provided defaults when unconfigured).
func (m *Model) normaliseRightRatios(defInfo, defGitStatus, defCommit float64) (float64, float64, float64) {
	info := m.layoutRatio("info", defInfo)
	gitStatus := m.layoutRatio("git_status", defGitStatus)
	commit := m.layoutRatio("commit", defCommit)
	total := info + gitStatus + commit
	if total <= 0 {
		return defInfo, defGitStatus, defCommit
	}
	return info / total, gitStatus / total, commit / total
}

// setWindowSize updates the window dimensions and applies the layout.
func (m *Model) setWindowSize(width, height int) {
	m.state.view.WindowWidth = width
	m.state.view.WindowHeight = height
	m.state.view.ResizeOffset = 0
	m.applyLayout(m.computeLayout())
}

// computeLayout calculates the layout dimensions based on window size and UI state.
func (m *Model) computeLayout() layoutDims {
	width := m.state.view.WindowWidth
	height := m.state.view.WindowHeight
	if width <= 0 {
		width = 120
	}
	if height <= 0 {
		height = 40
	}

	headerHeight := 2
	footerHeight := 1
	filterHeight := 0
	if m.state.view.ShowingFilter || m.state.view.ShowingSearch {
		filterHeight = 1
	}
	gapX := 1
	gapY := 1

	bodyHeight := max(height-headerHeight-footerHeight-filterHeight, 8)

	hasNotes := m.hasNoteForSelectedWorktree()
	hasAgentSessions := m.hasAgentSessionsForSelectedWorktree()
	hasGitStatus := m.hasGitStatus()

	// Handle zoom mode: zoomed pane gets full body area
	if m.state.view.ZoomedPane >= 0 {
		paneFrameX := m.basePaneStyle().GetHorizontalFrameSize()
		paneFrameY := m.basePaneStyle().GetVerticalFrameSize()
		fullWidth := width
		fullInnerWidth := max(1, fullWidth-paneFrameX)
		fullInnerHeight := max(1, bodyHeight-paneFrameY)

		return layoutDims{
			width:                  width,
			height:                 height,
			headerHeight:           headerHeight,
			footerHeight:           footerHeight,
			filterHeight:           filterHeight,
			bodyHeight:             bodyHeight,
			gapX:                   0,
			gapY:                   0,
			hasGitStatus:           hasGitStatus,
			hasNotes:               hasNotes,
			hasAgentSessions:       hasAgentSessions,
			leftWidth:              fullWidth,
			rightWidth:             fullWidth,
			leftInnerWidth:         fullInnerWidth,
			rightInnerWidth:        fullInnerWidth,
			leftInnerHeight:        fullInnerHeight,
			leftTopHeight:          bodyHeight,
			leftTopInnerHeight:     fullInnerHeight,
			leftMiddleHeight:       bodyHeight,
			leftMiddleInnerHeight:  fullInnerHeight,
			leftBottomHeight:       bodyHeight,
			leftBottomInnerHeight:  fullInnerHeight,
			rightTopHeight:         bodyHeight,
			rightMiddleHeight:      bodyHeight,
			rightBottomHeight:      bodyHeight,
			rightTopInnerHeight:    fullInnerHeight,
			rightMiddleInnerHeight: fullInnerHeight,
			rightBottomInnerHeight: fullInnerHeight,
		}
	}

	if m.state.view.Layout == state.LayoutTop {
		return m.computeTopLayoutDims(width, height, headerHeight, footerHeight, filterHeight, bodyHeight, hasGitStatus)
	}

	baseLeftRatio := m.layoutRatio("worktrees", 0.55)
	leftRatio := baseLeftRatio
	switch m.state.view.FocusedPane {
	case paneWorktrees, paneNotes, paneAgentSessions:
		leftRatio = baseLeftRatio * 0.82 // slightly tighter than unfocused
	case paneInfo, paneGitStatus, paneCommit:
		leftRatio = max(0.20, baseLeftRatio*0.45)
	}

	leftWidth := int(float64(width-gapX)*leftRatio) + m.state.view.ResizeOffset
	rightWidth := width - leftWidth - gapX
	if leftWidth < minLeftPaneWidth {
		leftWidth = minLeftPaneWidth
		rightWidth = width - leftWidth - gapX
	}
	if rightWidth < minRightPaneWidth {
		rightWidth = minRightPaneWidth
		leftWidth = width - rightWidth - gapX
	}
	if leftWidth < minLeftPaneWidth {
		leftWidth = minLeftPaneWidth
	}
	if rightWidth < minRightPaneWidth {
		rightWidth = minRightPaneWidth
	}
	if leftWidth+rightWidth+gapX > width {
		rightWidth = width - leftWidth - gapX
	}
	if rightWidth < 0 {
		rightWidth = 0
	}

	paneFrameX := m.basePaneStyle().GetHorizontalFrameSize()
	paneFrameY := m.basePaneStyle().GetVerticalFrameSize()

	var leftTopHeight, leftMiddleHeight, leftBottomHeight int
	var leftTopInnerHeight, leftMiddleInnerHeight, leftBottomInnerHeight int
	stackedPanes := 0
	if hasAgentSessions {
		stackedPanes++
	}
	if hasNotes {
		stackedPanes++
	}
	if stackedPanes == 0 {
		leftTopHeight = bodyHeight
	} else {
		availableHeight := bodyHeight - gapY*stackedPanes
		agentRatio := 0.0
		notesRatio := 0.0
		if hasAgentSessions {
			agentRatio = m.layoutRatio("agent_sessions", 0.20)
			if m.state.view.FocusedPane == paneAgentSessions {
				agentRatio = min(agentRatio+0.10, 0.35)
			}
		}
		if hasNotes {
			notesRatio = m.layoutRatio("notes", 0.30)
			if m.state.view.FocusedPane == paneNotes {
				notesRatio = min(notesRatio+0.10, 0.45)
			}
		}
		switch {
		case hasAgentSessions && hasNotes:
			leftMiddleHeight = max(4, int(float64(availableHeight)*agentRatio))
			leftBottomHeight = max(4, int(float64(availableHeight)*notesRatio))
			leftTopHeight = availableHeight - leftMiddleHeight - leftBottomHeight
		case hasAgentSessions:
			leftMiddleHeight = max(4, int(float64(availableHeight)*agentRatio))
			leftTopHeight = availableHeight - leftMiddleHeight
		default:
			leftBottomHeight = max(4, int(float64(availableHeight)*notesRatio))
			leftTopHeight = availableHeight - leftBottomHeight
		}
		if leftTopHeight < 4 {
			deficit := 4 - leftTopHeight
			leftTopHeight = 4
			if leftBottomHeight > 0 {
				leftBottomHeight = max(4, leftBottomHeight-deficit)
			} else if leftMiddleHeight > 0 {
				leftMiddleHeight = max(4, leftMiddleHeight-deficit)
			}
		}
	}

	// Cap commit pane to notes pane height when notes exist, otherwise use a fixed cap
	commitCap := 11
	if hasNotes && leftBottomHeight > 0 {
		commitCap = leftBottomHeight
	} else if hasAgentSessions && leftMiddleHeight > 0 {
		commitCap = leftMiddleHeight
	}

	// Right column split: 3-way (Info / Git Status / Commit) or 2-way (Info / Commit) when clean
	var rightTopHeight, rightMiddleHeight, rightBottomHeight int
	if hasGitStatus {
		// 3-way vertical split with two gaps
		baseInfo, baseGS, baseCommit := m.normaliseRightRatios(0.30, 0.40, 0.30)
		var topRatio, midRatio float64
		switch m.state.view.FocusedPane {
		case paneInfo: // Status focused — boost info
			topRatio = min(baseInfo+0.20, 0.60)
			midRatio = (1.0 - topRatio) * baseGS / (baseGS + baseCommit)
		case paneGitStatus: // Git Status focused — boost git_status
			midRatio = min(baseGS+0.20, 0.60)
			topRatio = (1.0 - midRatio) * baseInfo / (baseInfo + baseCommit)
		case paneCommit: // Commit focused — boost commit
			botShare := min(baseCommit+0.20, 0.60)
			topRatio = (1.0 - botShare) * baseInfo / (baseInfo + baseGS)
			midRatio = (1.0 - botShare) * baseGS / (baseInfo + baseGS)
		default: // Worktrees focused — use base ratios
			topRatio, midRatio = baseInfo, baseGS
		}

		availableHeight := bodyHeight - gapY*2
		rightTopHeight = max(4, int(float64(availableHeight)*topRatio))
		rightMiddleHeight = max(4, int(float64(availableHeight)*midRatio))
		rightBottomHeight = availableHeight - rightTopHeight - rightMiddleHeight
		if rightBottomHeight > commitCap {
			rightBottomHeight = commitCap
			rightTopHeight = availableHeight - rightMiddleHeight - rightBottomHeight
		} else if rightBottomHeight < 4 {
			rightBottomHeight = 4
			rightMiddleHeight = availableHeight - rightTopHeight - rightBottomHeight
			if rightMiddleHeight < 4 {
				rightMiddleHeight = 4
				rightTopHeight = availableHeight - rightMiddleHeight - rightBottomHeight
			}
		}
	} else {
		// 2-way vertical split (Info / Commit) with one gap
		infoR := m.layoutRatio("info", 0.30)
		commitR := m.layoutRatio("commit", 0.30)
		baseTop := infoR / (infoR + commitR)
		var topRatio float64
		switch m.state.view.FocusedPane {
		case paneInfo:
			topRatio = min(baseTop+0.20, 0.70)
		case paneCommit:
			topRatio = max(baseTop-0.10, 0.25)
		default:
			topRatio = baseTop
		}

		availableHeight := bodyHeight - gapY
		rightTopHeight = max(4, int(float64(availableHeight)*topRatio))
		rightBottomHeight = availableHeight - rightTopHeight
		if rightBottomHeight > commitCap {
			rightBottomHeight = commitCap
			rightTopHeight = availableHeight - rightBottomHeight
		} else if rightBottomHeight < 4 {
			rightBottomHeight = 4
			rightTopHeight = availableHeight - rightBottomHeight
		}
	}

	leftInnerWidth := max(1, leftWidth-paneFrameX)
	rightInnerWidth := max(1, rightWidth-paneFrameX)
	leftInnerHeight := max(1, bodyHeight-paneFrameY)
	rightTopInnerHeight := max(1, rightTopHeight-paneFrameY)
	rightMiddleInnerHeight := max(1, rightMiddleHeight-paneFrameY)
	rightBottomInnerHeight := max(1, rightBottomHeight-paneFrameY)

	// Finish notes inner dimensions
	if hasNotes || hasAgentSessions {
		leftTopInnerHeight = max(1, leftTopHeight-paneFrameY)
		leftMiddleInnerHeight = max(1, leftMiddleHeight-paneFrameY)
		leftBottomInnerHeight = max(1, leftBottomHeight-paneFrameY)
	} else {
		leftTopHeight = bodyHeight
		leftTopInnerHeight = leftInnerHeight
	}

	return layoutDims{
		width:                  width,
		height:                 height,
		headerHeight:           headerHeight,
		footerHeight:           footerHeight,
		filterHeight:           filterHeight,
		bodyHeight:             bodyHeight,
		gapX:                   gapX,
		gapY:                   gapY,
		hasGitStatus:           hasGitStatus,
		hasNotes:               hasNotes,
		hasAgentSessions:       hasAgentSessions,
		leftWidth:              leftWidth,
		rightWidth:             rightWidth,
		leftInnerWidth:         leftInnerWidth,
		rightInnerWidth:        rightInnerWidth,
		leftInnerHeight:        leftInnerHeight,
		leftTopHeight:          leftTopHeight,
		leftMiddleHeight:       leftMiddleHeight,
		leftBottomHeight:       leftBottomHeight,
		leftTopInnerHeight:     leftTopInnerHeight,
		leftMiddleInnerHeight:  leftMiddleInnerHeight,
		leftBottomInnerHeight:  leftBottomInnerHeight,
		rightTopHeight:         rightTopHeight,
		rightMiddleHeight:      rightMiddleHeight,
		rightBottomHeight:      rightBottomHeight,
		rightTopInnerHeight:    rightTopInnerHeight,
		rightMiddleInnerHeight: rightMiddleInnerHeight,
		rightBottomInnerHeight: rightBottomInnerHeight,
	}
}

// computeTopLayoutDims calculates dimensions for the top layout mode
// where worktrees span the full width at top and status+git status+commit sit side-by-side at bottom.
func (m *Model) computeTopLayoutDims(width, height, headerHeight, footerHeight, filterHeight, bodyHeight int, hasGitStatus bool) layoutDims {
	gapX := 1
	gapY := 1

	paneFrameX := m.basePaneStyle().GetHorizontalFrameSize()
	paneFrameY := m.basePaneStyle().GetVerticalFrameSize()

	hasNotes := m.hasNoteForSelectedWorktree()
	hasAgentSessions := m.hasAgentSessionsForSelectedWorktree()

	// Vertical split: top / bottom with focus adjustments.
	baseTopRatio := m.layoutRatio("worktrees", 0.30)
	topRatio := baseTopRatio
	switch m.state.view.FocusedPane {
	case paneWorktrees, paneNotes, paneAgentSessions:
		topRatio = min(baseTopRatio+0.15, 0.60)
	case paneInfo, paneGitStatus, paneCommit:
		topRatio = max(0.20, baseTopRatio*0.45)
	}

	topHeight := max(4, int(float64(bodyHeight-gapY)*topRatio)) + m.state.view.ResizeOffset
	bottomHeight := bodyHeight - topHeight - gapY
	if bottomHeight < 6 {
		bottomHeight = 6
		topHeight = bodyHeight - bottomHeight - gapY
	}
	if topHeight < 4 {
		topHeight = 4
	}

	// Bottom horizontal split: 3-way (Info / Git Status / Commit) or 2-way (Info / Commit)
	var bottomLeftWidth, bottomMiddleWidth, bottomRightWidth int
	if hasGitStatus {
		// 3-way with two gaps
		baseInfo, baseGS, baseCommit := m.normaliseRightRatios(0.30, 0.40, 0.30)
		var leftRatio, midRatio float64
		switch m.state.view.FocusedPane {
		case paneInfo: // Status focused — boost info
			leftRatio = min(baseInfo+0.20, 0.60)
			midRatio = (1.0 - leftRatio) * baseGS / (baseGS + baseCommit)
		case paneGitStatus: // Git Status focused — boost git_status
			midRatio = min(baseGS+0.20, 0.60)
			leftRatio = (1.0 - midRatio) * baseInfo / (baseInfo + baseCommit)
		case paneCommit: // Commit focused — boost commit
			botShare := min(baseCommit+0.20, 0.60)
			leftRatio = (1.0 - botShare) * baseInfo / (baseInfo + baseGS)
			midRatio = (1.0 - botShare) * baseGS / (baseInfo + baseGS)
		default: // Worktrees focused — use base ratios
			leftRatio, midRatio = baseInfo, baseGS
		}

		availableWidth := width - gapX*2
		bottomLeftWidth = max(minLeftPaneWidth, int(float64(availableWidth)*leftRatio))
		bottomMiddleWidth = max(minRightPaneWidth, int(float64(availableWidth)*midRatio))
		bottomRightWidth = availableWidth - bottomLeftWidth - bottomMiddleWidth
		if bottomRightWidth < minRightPaneWidth {
			bottomRightWidth = minRightPaneWidth
			bottomMiddleWidth = availableWidth - bottomLeftWidth - bottomRightWidth
			if bottomMiddleWidth < minRightPaneWidth {
				bottomMiddleWidth = minRightPaneWidth
				bottomLeftWidth = availableWidth - bottomMiddleWidth - bottomRightWidth
			}
		}
		if bottomLeftWidth < minLeftPaneWidth {
			bottomLeftWidth = minLeftPaneWidth
		}

		// Final clamp: ensure widths + gaps do not exceed total width
		totalBottom := bottomLeftWidth + gapX + bottomMiddleWidth + gapX + bottomRightWidth
		if totalBottom > width {
			excess := totalBottom - width
			for excess > 0 && bottomRightWidth > 8 {
				bottomRightWidth--
				excess--
			}
			for excess > 0 && bottomMiddleWidth > 8 {
				bottomMiddleWidth--
				excess--
			}
			for excess > 0 && bottomLeftWidth > 8 {
				bottomLeftWidth--
				excess--
			}
		}
	} else {
		// 2-way with one gap (Info / Commit)
		infoR := m.layoutRatio("info", 0.30)
		commitR := m.layoutRatio("commit", 0.30)
		baseLeft := infoR / (infoR + commitR)
		var leftRatio float64
		switch m.state.view.FocusedPane {
		case paneInfo:
			leftRatio = min(baseLeft+0.10, 0.65)
		case paneCommit:
			leftRatio = max(baseLeft-0.10, 0.30)
		default:
			leftRatio = baseLeft
		}

		availableWidth := width - gapX
		bottomLeftWidth = max(minLeftPaneWidth, int(float64(availableWidth)*leftRatio))
		bottomRightWidth = availableWidth - bottomLeftWidth
		if bottomRightWidth < minRightPaneWidth {
			bottomRightWidth = minRightPaneWidth
			bottomLeftWidth = availableWidth - bottomRightWidth
		}
		if bottomLeftWidth < minLeftPaneWidth {
			bottomLeftWidth = minLeftPaneWidth
		}
	}

	var agentRowHeight, agentRowInnerHeight, agentRowInnerWidth int
	var notesRowHeight, notesRowInnerHeight, notesRowInnerWidth int
	extraRows := 0
	if hasAgentSessions {
		extraRows++
	}
	if hasNotes {
		extraRows++
	}
	if extraRows > 0 {
		remaining := bodyHeight - gapY*(extraRows+1)
		if hasAgentSessions {
			agentRatio := m.layoutRatio("agent_sessions", 0.20) * 0.5
			if m.state.view.FocusedPane == paneAgentSessions {
				agentRatio = min(agentRatio+0.05, 0.25)
			}
			agentRowHeight = max(4, int(float64(bodyHeight)*agentRatio))
			remaining -= agentRowHeight
			agentRowInnerHeight = max(1, agentRowHeight-paneFrameY)
			agentRowInnerWidth = max(1, width-paneFrameX)
		}
		if hasNotes {
			notesRatio := m.layoutRatio("notes", 0.30) * 0.5
			if m.state.view.FocusedPane == paneNotes {
				notesRatio = min(notesRatio+0.05, 0.25)
			}
			notesRowHeight = max(4, int(float64(bodyHeight)*notesRatio))
			remaining -= notesRowHeight
			notesRowInnerHeight = max(1, notesRowHeight-paneFrameY)
			notesRowInnerWidth = max(1, width-paneFrameX)
		}
		topHeight = max(4, int(float64(remaining)*topRatio/(topRatio+(1.0-topRatio))))
		bottomHeight = remaining - topHeight
		if bottomHeight < 6 {
			bottomHeight = 6
			topHeight = remaining - bottomHeight
		}
		if topHeight < 4 {
			topHeight = 4
		}
	}

	topInnerWidth := max(1, width-paneFrameX)
	topInnerHeight := max(1, topHeight-paneFrameY)
	bottomLeftInnerWidth := max(1, bottomLeftWidth-paneFrameX)
	bottomMiddleInnerWidth := max(1, bottomMiddleWidth-paneFrameX)
	bottomRightInnerWidth := max(1, bottomRightWidth-paneFrameX)
	bottomLeftInnerHeight := max(1, bottomHeight-paneFrameY)
	bottomMiddleInnerHeight := max(1, bottomHeight-paneFrameY)
	bottomRightInnerHeight := max(1, bottomHeight-paneFrameY)

	return layoutDims{
		width:            width,
		height:           height,
		headerHeight:     headerHeight,
		footerHeight:     footerHeight,
		filterHeight:     filterHeight,
		bodyHeight:       bodyHeight,
		gapX:             gapX,
		gapY:             gapY,
		layoutMode:       state.LayoutTop,
		hasGitStatus:     hasGitStatus,
		hasNotes:         hasNotes,
		hasAgentSessions: hasAgentSessions,

		agentRowHeight:      agentRowHeight,
		agentRowInnerHeight: agentRowInnerHeight,
		agentRowInnerWidth:  agentRowInnerWidth,
		// Notes row
		notesRowHeight:      notesRowHeight,
		notesRowInnerHeight: notesRowInnerHeight,
		notesRowInnerWidth:  notesRowInnerWidth,

		// Top layout fields
		topHeight:               topHeight,
		topInnerWidth:           topInnerWidth,
		topInnerHeight:          topInnerHeight,
		bottomHeight:            bottomHeight,
		bottomLeftWidth:         bottomLeftWidth,
		bottomMiddleWidth:       bottomMiddleWidth,
		bottomRightWidth:        bottomRightWidth,
		bottomLeftInnerWidth:    bottomLeftInnerWidth,
		bottomMiddleInnerWidth:  bottomMiddleInnerWidth,
		bottomRightInnerWidth:   bottomRightInnerWidth,
		bottomLeftInnerHeight:   bottomLeftInnerHeight,
		bottomMiddleInnerHeight: bottomMiddleInnerHeight,
		bottomRightInnerHeight:  bottomRightInnerHeight,

		// Populate default-layout fields for zoom mode compatibility
		leftWidth:              width,
		rightWidth:             width,
		leftInnerWidth:         topInnerWidth,
		rightInnerWidth:        bottomLeftInnerWidth,
		leftInnerHeight:        topInnerHeight,
		leftTopHeight:          topHeight,
		leftTopInnerHeight:     topInnerHeight,
		leftMiddleHeight:       agentRowHeight,
		leftMiddleInnerHeight:  agentRowInnerHeight,
		leftBottomHeight:       notesRowHeight,
		leftBottomInnerHeight:  notesRowInnerHeight,
		rightTopHeight:         bottomHeight,
		rightMiddleHeight:      bottomHeight,
		rightBottomHeight:      bottomHeight,
		rightTopInnerHeight:    bottomLeftInnerHeight,
		rightMiddleInnerHeight: bottomMiddleInnerHeight,
		rightBottomInnerHeight: bottomRightInnerHeight,
	}
}

// applyLayout applies the computed layout dimensions to UI components.
func (m *Model) applyLayout(layout layoutDims) {
	titleHeight := 1
	tableHeaderHeight := 1 // bubbles table has its own header

	if layout.layoutMode == state.LayoutTop && m.state.view.ZoomedPane < 0 {
		// Top layout: worktree uses full width at top, commit uses bottom right
		tableHeight := max(3, layout.topInnerHeight-titleHeight-tableHeaderHeight-2)
		m.state.ui.worktreeTable.SetWidth(layout.topInnerWidth)
		m.state.ui.worktreeTable.SetHeight(tableHeight)
		m.updateTableColumns(layout.topInnerWidth)

		// Pane title is the top border, already accounted for in paneFrameY.
		// Safety margin of 2 prevents overflow at small sizes.
		logHeight := max(3, layout.bottomRightInnerHeight-2)
		m.state.ui.logTable.SetWidth(layout.bottomRightInnerWidth)
		m.state.ui.logTable.SetHeight(logHeight)
		m.updateLogColumns(layout.bottomRightInnerWidth)
	} else {
		// Default layout or zoom mode
		// Subtract 2 extra lines for safety margin
		// Minimum height of 3 is required to prevent viewport slice bounds panic
		wtInnerHeight := layout.leftInnerHeight
		if (layout.hasNotes || layout.hasAgentSessions) && m.state.view.ZoomedPane < 0 {
			wtInnerHeight = layout.leftTopInnerHeight
		}
		tableHeight := max(3, wtInnerHeight-titleHeight-tableHeaderHeight-2)
		m.state.ui.worktreeTable.SetWidth(layout.leftInnerWidth)
		m.state.ui.worktreeTable.SetHeight(tableHeight)
		m.updateTableColumns(layout.leftInnerWidth)

		// Pane title is the top border, already accounted for in paneFrameY.
		// Safety margin of 2 prevents overflow at small sizes.
		logHeight := max(3, layout.rightBottomInnerHeight-2)
		m.state.ui.logTable.SetWidth(layout.rightInnerWidth)
		m.state.ui.logTable.SetHeight(logHeight)
		m.updateLogColumns(layout.rightInnerWidth)
	}

	m.state.ui.filterInput.SetWidth(max(20, layout.width-18))
}

// updateTableColumns updates the worktree table column widths based on available space.
func (m *Model) updateTableColumns(totalWidth int) {
	status := 10
	last := 15

	// Only include PR column width if PR data has been loaded and PR is not disabled
	showPRColumn := m.prDataLoaded && !m.config.DisablePR
	pr := 0
	if showPRColumn {
		pr = 12
	}

	// The table library handles separators internally (3 spaces per separator)
	// So we need to account for them: (numColumns - 1) * 3
	numColumns := 3
	if showPRColumn {
		numColumns = 4
	}
	separatorSpace := (numColumns - 1) * 3

	worktree := max(12, totalWidth-status-last-pr-separatorSpace)
	excess := worktree + status + pr + last + separatorSpace - totalWidth
	for excess > 0 && last > 10 {
		last--
		excess--
	}
	if showPRColumn {
		for excess > 0 && pr > 8 {
			pr--
			excess--
		}
	}
	for excess > 0 && worktree > 12 {
		worktree--
		excess--
	}
	for excess > 0 && status > 6 {
		status--
		excess--
	}
	if excess > 0 {
		worktree = max(6, worktree-excess)
	}

	// Final adjustment: ensure column widths + separators sum exactly to totalWidth
	actualTotal := worktree + status + last + pr + separatorSpace
	if actualTotal < totalWidth {
		// Distribute remaining space to the worktree column
		worktree += (totalWidth - actualTotal)
	} else if actualTotal > totalWidth {
		// Remove excess from worktree column
		worktree = max(6, worktree-(actualTotal-totalWidth))
	}

	columns := []table.Column{
		{Title: "Name", Width: worktree},
		{Title: "Status", Width: status},
		{Title: "Last Active", Width: last},
	}

	if showPRColumn {
		columns = append(columns, table.Column{Title: "PR", Width: pr})
	}

	m.state.ui.worktreeTable.SetColumns(columns)
}

// updateLogColumns updates the log table column widths based on available space.
func (m *Model) updateLogColumns(totalWidth int) {
	sha := 8
	author := 2

	// The table library handles separators internally (3 spaces per separator)
	// 3 columns = 2 separators = 6 spaces
	separatorSpace := 6

	message := max(10, totalWidth-sha-author-separatorSpace)

	// Final adjustment: ensure column widths + separator space sum exactly to totalWidth
	actualTotal := sha + author + message + separatorSpace
	if actualTotal < totalWidth {
		message += (totalWidth - actualTotal)
	} else if actualTotal > totalWidth {
		message = max(10, message-(actualTotal-totalWidth))
	}

	m.state.ui.logTable.SetColumns([]table.Column{
		{Title: "SHA", Width: sha},
		{Title: "Au", Width: author},
		{Title: "Message", Width: message},
	})
}
