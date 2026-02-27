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
	leftTopHeight         int
	leftBottomHeight      int
	leftTopInnerHeight    int
	leftBottomInnerHeight int

	// Notes pane (top layout: dedicated row)
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

// setWindowSize updates the window dimensions and applies the layout.
func (m *Model) setWindowSize(width, height int) {
	m.state.view.WindowWidth = width
	m.state.view.WindowHeight = height
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

	bodyHeight := maxInt(height-headerHeight-footerHeight-filterHeight, 8)

	hasNotes := m.hasNoteForSelectedWorktree()
	hasGitStatus := m.hasGitStatus()

	// Handle zoom mode: zoomed pane gets full body area
	if m.state.view.ZoomedPane >= 0 {
		paneFrameX := m.basePaneStyle().GetHorizontalFrameSize()
		paneFrameY := m.basePaneStyle().GetVerticalFrameSize()
		fullWidth := width
		fullInnerWidth := maxInt(1, fullWidth-paneFrameX)
		fullInnerHeight := maxInt(1, bodyHeight-paneFrameY)

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
			leftWidth:              fullWidth,
			rightWidth:             fullWidth,
			leftInnerWidth:         fullInnerWidth,
			rightInnerWidth:        fullInnerWidth,
			leftInnerHeight:        fullInnerHeight,
			leftTopHeight:          bodyHeight,
			leftTopInnerHeight:     fullInnerHeight,
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

	leftRatio := 0.55
	switch m.state.view.FocusedPane {
	case 0, 4:
		leftRatio = 0.45
	case 1, 2, 3:
		leftRatio = 0.20
	}

	leftWidth := int(float64(width-gapX) * leftRatio)
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

	// Compute notes pane height first so the commit pane can match it
	var leftTopHeight, leftBottomHeight, leftTopInnerHeight, leftBottomInnerHeight int
	if hasNotes {
		notesRatio := 0.30
		if m.state.view.FocusedPane == 4 {
			notesRatio = 0.50
		}
		leftBottomHeight = maxInt(4, int(float64(bodyHeight-gapY)*notesRatio))
		leftTopHeight = bodyHeight - leftBottomHeight - gapY
		if leftTopHeight < 4 {
			leftTopHeight = 4
			leftBottomHeight = bodyHeight - leftTopHeight - gapY
		}
	}

	// Cap commit pane to notes pane height when notes exist, otherwise use a fixed cap
	commitCap := 11
	if hasNotes {
		commitCap = leftBottomHeight
	}

	// Right column split: 3-way (Info / Git Status / Commit) or 2-way (Info / Commit) when clean
	var rightTopHeight, rightMiddleHeight, rightBottomHeight int
	if hasGitStatus {
		// 3-way vertical split with two gaps
		var topRatio, midRatio float64
		switch m.state.view.FocusedPane {
		case 1: // Status focused
			topRatio, midRatio = 0.50, 0.30
		case 2: // Git Status focused
			topRatio, midRatio = 0.20, 0.60
		case 3: // Commit focused
			topRatio, midRatio = 0.20, 0.20
		default: // Worktrees focused
			topRatio, midRatio = 0.30, 0.40
		}

		availableHeight := bodyHeight - gapY*2
		rightTopHeight = maxInt(4, int(float64(availableHeight)*topRatio))
		rightMiddleHeight = maxInt(4, int(float64(availableHeight)*midRatio))
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
		var topRatio float64
		switch m.state.view.FocusedPane {
		case 1:
			topRatio = 0.60
		case 3:
			topRatio = 0.30
		default:
			topRatio = 0.40
		}

		availableHeight := bodyHeight - gapY
		rightTopHeight = maxInt(4, int(float64(availableHeight)*topRatio))
		rightBottomHeight = availableHeight - rightTopHeight
		if rightBottomHeight > commitCap {
			rightBottomHeight = commitCap
			rightTopHeight = availableHeight - rightBottomHeight
		} else if rightBottomHeight < 4 {
			rightBottomHeight = 4
			rightTopHeight = availableHeight - rightBottomHeight
		}
	}

	leftInnerWidth := maxInt(1, leftWidth-paneFrameX)
	rightInnerWidth := maxInt(1, rightWidth-paneFrameX)
	leftInnerHeight := maxInt(1, bodyHeight-paneFrameY)
	rightTopInnerHeight := maxInt(1, rightTopHeight-paneFrameY)
	rightMiddleInnerHeight := maxInt(1, rightMiddleHeight-paneFrameY)
	rightBottomInnerHeight := maxInt(1, rightBottomHeight-paneFrameY)

	// Finish notes inner dimensions
	if hasNotes {
		leftTopInnerHeight = maxInt(1, leftTopHeight-paneFrameY)
		leftBottomInnerHeight = maxInt(1, leftBottomHeight-paneFrameY)
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
		leftWidth:              leftWidth,
		rightWidth:             rightWidth,
		leftInnerWidth:         leftInnerWidth,
		rightInnerWidth:        rightInnerWidth,
		leftInnerHeight:        leftInnerHeight,
		leftTopHeight:          leftTopHeight,
		leftBottomHeight:       leftBottomHeight,
		leftTopInnerHeight:     leftTopInnerHeight,
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

	// Vertical split: top 30% / bottom 70% with focus adjustments
	topRatio := 0.30
	switch m.state.view.FocusedPane {
	case 0, 4:
		topRatio = 0.45
	case 1, 2, 3:
		topRatio = 0.20
	}

	topHeight := maxInt(4, int(float64(bodyHeight-gapY)*topRatio))
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
		var leftRatio, midRatio float64
		switch m.state.view.FocusedPane {
		case 1: // Status focused
			leftRatio, midRatio = 0.50, 0.30
		case 2: // Git Status focused
			leftRatio, midRatio = 0.20, 0.60
		case 3: // Commit focused
			leftRatio, midRatio = 0.20, 0.20
		default: // Worktrees focused
			leftRatio, midRatio = 0.30, 0.40
		}

		availableWidth := width - gapX*2
		bottomLeftWidth = maxInt(minLeftPaneWidth, int(float64(availableWidth)*leftRatio))
		bottomMiddleWidth = maxInt(minRightPaneWidth, int(float64(availableWidth)*midRatio))
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
		var leftRatio float64
		switch m.state.view.FocusedPane {
		case 1:
			leftRatio = 0.55
		case 3:
			leftRatio = 0.35
		default:
			leftRatio = 0.45
		}

		availableWidth := width - gapX
		bottomLeftWidth = maxInt(minLeftPaneWidth, int(float64(availableWidth)*leftRatio))
		bottomRightWidth = availableWidth - bottomLeftWidth
		if bottomRightWidth < minRightPaneWidth {
			bottomRightWidth = minRightPaneWidth
			bottomLeftWidth = availableWidth - bottomRightWidth
		}
		if bottomLeftWidth < minLeftPaneWidth {
			bottomLeftWidth = minLeftPaneWidth
		}
	}

	// Notes row in top layout: insert between worktrees and bottom panes
	var notesRowHeight, notesRowInnerHeight, notesRowInnerWidth int
	if hasNotes {
		notesRatio := 0.15
		if m.state.view.FocusedPane == 4 {
			notesRatio = 0.25
		}
		notesRowHeight = maxInt(4, int(float64(bodyHeight)*notesRatio))
		// Re-budget: top + gap + notes + gap + bottom = bodyHeight
		remaining := bodyHeight - notesRowHeight - gapY*2
		topHeight = maxInt(4, int(float64(remaining)*topRatio/(topRatio+(1.0-topRatio))))
		bottomHeight = remaining - topHeight
		if bottomHeight < 6 {
			bottomHeight = 6
			topHeight = remaining - bottomHeight
		}
		if topHeight < 4 {
			topHeight = 4
		}
		notesRowInnerHeight = maxInt(1, notesRowHeight-paneFrameY)
		notesRowInnerWidth = maxInt(1, width-paneFrameX)
	}

	topInnerWidth := maxInt(1, width-paneFrameX)
	topInnerHeight := maxInt(1, topHeight-paneFrameY)
	bottomLeftInnerWidth := maxInt(1, bottomLeftWidth-paneFrameX)
	bottomMiddleInnerWidth := maxInt(1, bottomMiddleWidth-paneFrameX)
	bottomRightInnerWidth := maxInt(1, bottomRightWidth-paneFrameX)
	bottomLeftInnerHeight := maxInt(1, bottomHeight-paneFrameY)
	bottomMiddleInnerHeight := maxInt(1, bottomHeight-paneFrameY)
	bottomRightInnerHeight := maxInt(1, bottomHeight-paneFrameY)

	return layoutDims{
		width:        width,
		height:       height,
		headerHeight: headerHeight,
		footerHeight: footerHeight,
		filterHeight: filterHeight,
		bodyHeight:   bodyHeight,
		gapX:         gapX,
		gapY:         gapY,
		layoutMode:   state.LayoutTop,
		hasGitStatus: hasGitStatus,
		hasNotes:     hasNotes,

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
		tableHeight := maxInt(3, layout.topInnerHeight-titleHeight-tableHeaderHeight-2)
		m.state.ui.worktreeTable.SetWidth(layout.topInnerWidth)
		m.state.ui.worktreeTable.SetHeight(tableHeight)
		m.updateTableColumns(layout.topInnerWidth)

		// Pane title is the top border, already accounted for in paneFrameY.
		// Safety margin of 2 prevents overflow at small sizes.
		logHeight := maxInt(3, layout.bottomRightInnerHeight-2)
		m.state.ui.logTable.SetWidth(layout.bottomRightInnerWidth)
		m.state.ui.logTable.SetHeight(logHeight)
		m.updateLogColumns(layout.bottomRightInnerWidth)
	} else {
		// Default layout or zoom mode
		// Subtract 2 extra lines for safety margin
		// Minimum height of 3 is required to prevent viewport slice bounds panic
		wtInnerHeight := layout.leftInnerHeight
		if layout.hasNotes && m.state.view.ZoomedPane < 0 {
			wtInnerHeight = layout.leftTopInnerHeight
		}
		tableHeight := maxInt(3, wtInnerHeight-titleHeight-tableHeaderHeight-2)
		m.state.ui.worktreeTable.SetWidth(layout.leftInnerWidth)
		m.state.ui.worktreeTable.SetHeight(tableHeight)
		m.updateTableColumns(layout.leftInnerWidth)

		// Pane title is the top border, already accounted for in paneFrameY.
		// Safety margin of 2 prevents overflow at small sizes.
		logHeight := maxInt(3, layout.rightBottomInnerHeight-2)
		m.state.ui.logTable.SetWidth(layout.rightInnerWidth)
		m.state.ui.logTable.SetHeight(logHeight)
		m.updateLogColumns(layout.rightInnerWidth)
	}

	m.state.ui.filterInput.SetWidth(maxInt(20, layout.width-18))
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

	worktree := maxInt(12, totalWidth-status-last-pr-separatorSpace)
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
		worktree = maxInt(6, worktree-excess)
	}

	// Final adjustment: ensure column widths + separators sum exactly to totalWidth
	actualTotal := worktree + status + last + pr + separatorSpace
	if actualTotal < totalWidth {
		// Distribute remaining space to the worktree column
		worktree += (totalWidth - actualTotal)
	} else if actualTotal > totalWidth {
		// Remove excess from worktree column
		worktree = maxInt(6, worktree-(actualTotal-totalWidth))
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

	message := maxInt(10, totalWidth-sha-author-separatorSpace)

	// Final adjustment: ensure column widths + separator space sum exactly to totalWidth
	actualTotal := sha + author + message + separatorSpace
	if actualTotal < totalWidth {
		message += (totalWidth - actualTotal)
	} else if actualTotal > totalWidth {
		message = maxInt(10, message-(actualTotal-totalWidth))
	}

	m.state.ui.logTable.SetColumns([]table.Column{
		{Title: "SHA", Width: sha},
		{Title: "Au", Width: author},
		{Title: "Message", Width: message},
	})
}
