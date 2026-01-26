package app

import (
	"github.com/charmbracelet/bubbles/table"
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
	rightBottomHeight      int
	rightTopInnerHeight    int
	rightBottomInnerHeight int
}

// setWindowSize updates the window dimensions and applies the layout.
func (m *Model) setWindowSize(width, height int) {
	m.windowWidth = width
	m.windowHeight = height
	m.applyLayout(m.computeLayout())
}

// computeLayout calculates the layout dimensions based on window size and UI state.
func (m *Model) computeLayout() layoutDims {
	width := m.windowWidth
	height := m.windowHeight
	if width <= 0 {
		width = 120
	}
	if height <= 0 {
		height = 40
	}

	headerHeight := 1
	footerHeight := 1
	filterHeight := 0
	if m.showingFilter || m.showingSearch {
		filterHeight = 1
	}
	gapX := 1
	gapY := 1

	bodyHeight := maxInt(height-headerHeight-footerHeight-filterHeight, 8)

	// Handle zoom mode: zoomed pane gets full body area
	if m.zoomedPane >= 0 {
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
			leftWidth:              fullWidth,
			rightWidth:             fullWidth,
			leftInnerWidth:         fullInnerWidth,
			rightInnerWidth:        fullInnerWidth,
			leftInnerHeight:        fullInnerHeight,
			rightTopHeight:         bodyHeight,
			rightBottomHeight:      bodyHeight,
			rightTopInnerHeight:    fullInnerHeight,
			rightBottomInnerHeight: fullInnerHeight,
		}
	}

	leftRatio := 0.55
	switch m.focusedPane {
	case 0:
		leftRatio = 0.45
	case 1, 2:
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

	topRatio := 0.70
	switch m.focusedPane {
	case 1: // Status focused → give more height to top pane
		topRatio = 0.50
	case 2: // Log focused → give more height to bottom pane
		topRatio = 0.50
	}

	rightTopHeight := maxInt(int(float64(bodyHeight-gapY)*topRatio), 6)
	rightBottomHeight := bodyHeight - rightTopHeight - gapY
	if rightBottomHeight < 4 {
		rightBottomHeight = 4
		rightTopHeight = bodyHeight - rightBottomHeight - gapY
	}

	paneFrameX := m.basePaneStyle().GetHorizontalFrameSize()
	paneFrameY := m.basePaneStyle().GetVerticalFrameSize()

	leftInnerWidth := maxInt(1, leftWidth-paneFrameX)
	rightInnerWidth := maxInt(1, rightWidth-paneFrameX)
	leftInnerHeight := maxInt(1, bodyHeight-paneFrameY)
	rightTopInnerHeight := maxInt(1, rightTopHeight-paneFrameY)
	rightBottomInnerHeight := maxInt(1, rightBottomHeight-paneFrameY)

	return layoutDims{
		width:                  width,
		height:                 height,
		headerHeight:           headerHeight,
		footerHeight:           footerHeight,
		filterHeight:           filterHeight,
		bodyHeight:             bodyHeight,
		gapX:                   gapX,
		gapY:                   gapY,
		leftWidth:              leftWidth,
		rightWidth:             rightWidth,
		leftInnerWidth:         leftInnerWidth,
		rightInnerWidth:        rightInnerWidth,
		leftInnerHeight:        leftInnerHeight,
		rightTopHeight:         rightTopHeight,
		rightBottomHeight:      rightBottomHeight,
		rightTopInnerHeight:    rightTopInnerHeight,
		rightBottomInnerHeight: rightBottomInnerHeight,
	}
}

// applyLayout applies the computed layout dimensions to UI components.
func (m *Model) applyLayout(layout layoutDims) {
	titleHeight := 1
	tableHeaderHeight := 1 // bubbles table has its own header

	// Subtract 2 extra lines for safety margin
	// Minimum height of 3 is required to prevent viewport slice bounds panic
	tableHeight := maxInt(3, layout.leftInnerHeight-titleHeight-tableHeaderHeight-2)
	m.worktreeTable.SetWidth(layout.leftInnerWidth)
	m.worktreeTable.SetHeight(tableHeight)
	m.updateTableColumns(layout.leftInnerWidth)

	logHeight := maxInt(3, layout.rightBottomInnerHeight-titleHeight-tableHeaderHeight-2)
	m.logTable.SetWidth(layout.rightInnerWidth)
	m.logTable.SetHeight(logHeight)
	m.updateLogColumns(layout.rightInnerWidth)

	m.filterInput.Width = maxInt(20, layout.width-18)
}

// updateTableColumns updates the worktree table column widths based on available space.
func (m *Model) updateTableColumns(totalWidth int) {
	status := 10
	last := 15

	// Only include PR column width if PR data has been loaded
	pr := 0
	if m.prDataLoaded {
		pr = 12
	}

	// The table library handles separators internally (3 spaces per separator)
	// So we need to account for them: (numColumns - 1) * 3
	numColumns := 3
	if m.prDataLoaded {
		numColumns = 4
	}
	separatorSpace := (numColumns - 1) * 3

	worktree := maxInt(12, totalWidth-status-last-pr-separatorSpace)
	excess := worktree + status + pr + last + separatorSpace - totalWidth
	for excess > 0 && last > 10 {
		last--
		excess--
	}
	if m.prDataLoaded {
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

	if m.prDataLoaded {
		columns = append(columns, table.Column{Title: "PR", Width: pr})
	}

	m.worktreeTable.SetColumns(columns)
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

	m.logTable.SetColumns([]table.Column{
		{Title: "SHA", Width: sha},
		{Title: "Au", Width: author},
		{Title: "Message", Width: message},
	})
}
