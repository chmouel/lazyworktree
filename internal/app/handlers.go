package app

import (
	"path/filepath"
	"sort"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/chmouel/lazyworktree/internal/models"
)

// handleKeyMsg processes keyboard input when not in a modal screen.
func (m *Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Handle filter input first - when filtering, only escape/enter should exit
	if m.showingFilter {
		keyStr := msg.String()
		if keyStr == keyEnter {
			// If there are filtered items, select the currently highlighted one
			if len(m.filteredWts) > 0 {
				m.showingFilter = false
				m.filterInput.Blur()
				m.worktreeTable.Focus()

				// Always select the currently highlighted item
				cursor := m.worktreeTable.Cursor()
				if cursor >= 0 && cursor < len(m.filteredWts) {
					m.selectedIndex = cursor
				} else {
					// If cursor is invalid, default to first item
					m.selectedIndex = 0
					m.worktreeTable.SetCursor(0)
				}
				return m.handleEnterKey()
			}
			// No filtered items - just close the filter
			m.showingFilter = false
			m.filterInput.Blur()
			m.worktreeTable.Focus()
			return m, nil
		}
		if isEscKey(keyStr) || keyStr == "ctrl+c" {
			m.showingFilter = false
			m.filterInput.Blur()
			m.worktreeTable.Focus()
			return m, nil
		}
		if keyStr == "alt+n" || keyStr == "alt+p" {
			return m.handleFilterNavigation(keyStr, true)
		}
		if keyStr == keyUp || keyStr == keyDown || keyStr == "ctrl+k" || keyStr == "ctrl+j" {
			return m.handleFilterNavigation(keyStr, false)
		}
		m.filterInput, cmd = m.filterInput.Update(msg)
		m.filterQuery = m.filterInput.Value()
		m.updateTable()
		return m, cmd
	}

	// Check for custom commands first - allows users to override built-in keys
	if _, ok := m.config.CustomCommands[msg.String()]; ok {
		return m, m.executeCustomCommand(msg.String())
	}

	return m.handleBuiltInKey(msg)
}

// handleBuiltInKey processes built-in keyboard shortcuts.
func (m *Model) handleBuiltInKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", keyQ:
		if m.selectedPath != "" {
			return m, tea.Quit
		}
		m.quitting = true
		return m, tea.Quit

	case "1":
		m.focusedPane = 0
		m.worktreeTable.Focus()
		return m, nil

	case "2":
		m.focusedPane = 1
		return m, nil

	case "3":
		m.focusedPane = 2
		m.logTable.Focus()
		return m, nil

	case "tab", "]":
		m.focusedPane = (m.focusedPane + 1) % 3
		switch m.focusedPane {
		case 0:
			m.worktreeTable.Focus()
		case 2:
			m.logTable.Focus()
		}
		return m, nil

	case "[":
		m.focusedPane = (m.focusedPane - 1 + 3) % 3
		switch m.focusedPane {
		case 0:
			m.worktreeTable.Focus()
		case 2:
			m.logTable.Focus()
		}
		return m, nil

	case "j", "down":
		return m.handleNavigationDown(msg)

	case "k", "up":
		return m.handleNavigationUp(msg)

	case "ctrl+d", " ":
		return m.handlePageDown(msg)

	case "ctrl+u":
		return m.handlePageUp(msg)

	case "pgdown":
		return m.handlePageDown(msg)

	case "pgup":
		return m.handlePageUp(msg)

	case "G":
		if m.focusedPane == 1 {
			m.statusViewport.GotoBottom()
			return m, nil
		}
		return m, nil

	case keyEnter:
		return m.handleEnterKey()

	case "r":
		m.loading = true
		return m, m.refreshWorktrees()

	case "c":
		return m, m.showCreateWorktree()

	case "D":
		return m, m.showDeleteWorktree()

	case "d":
		return m, m.showDiff()

	case "p":
		m.ciCache = make(map[string]*ciCacheEntry)
		if !m.prDataLoaded {
			m.loading = true
			m.statusContent = "Fetching PR data..."
			return m, m.fetchPRData()
		}
		return m, m.maybeFetchCIStatus()

	case "R":
		m.loading = true
		m.statusContent = "Fetching remotes..."
		return m, m.fetchRemotes()

	case "f", "/":
		m.showingFilter = true
		m.filterInput.Focus()
		return m, textinput.Blink

	case "s":
		m.sortByActive = !m.sortByActive
		m.updateTable()
		return m, nil

	case "ctrl+p", "P":
		return m, m.showCommandPalette()

	case "?":
		m.currentScreen = screenHelp
		m.helpScreen = NewHelpScreen(m.windowWidth, m.windowHeight, m.config.CustomCommands, m.theme)
		return m, nil

	case "g":
		if m.focusedPane == 1 {
			m.statusViewport.GotoTop()
			return m, nil
		}
		return m, m.openLazyGit()

	case "o":
		return m, m.openPR()

	case "m":
		return m, m.showRenameWorktree()

	case "A":
		return m, m.showAbsorbWorktree()

	case "X":
		return m, m.showPruneMerged()

	case "!":
		return m, m.showRunCommand()

	case "C":
		return m, m.showCherryPick()

	case keyEsc, keyEscRaw:
		if m.currentScreen == screenPalette {
			m.currentScreen = screenNone
			m.paletteScreen = nil
			return m, nil
		}
		return m, nil
	}

	// Handle table input
	if m.focusedPane == 0 {
		var cmd tea.Cmd
		m.worktreeTable, cmd = m.worktreeTable.Update(msg)
		return m, tea.Batch(cmd, m.debouncedUpdateDetailsView())
	}

	return m, nil
}

// handleNavigationDown processes down arrow and 'j' key navigation.
func (m *Model) handleNavigationDown(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	keyMsg := msg
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch m.focusedPane {
	case 0:
		m.worktreeTable, cmd = m.worktreeTable.Update(keyMsg)
		cmds = append(cmds, cmd)
		cmds = append(cmds, m.debouncedUpdateDetailsView())
	case 1:
		m.statusViewport, cmd = m.statusViewport.Update(keyMsg)
		cmds = append(cmds, cmd)
	default:
		m.logTable, cmd = m.logTable.Update(keyMsg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

// handleNavigationUp processes up arrow and 'k' key navigation.
func (m *Model) handleNavigationUp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	cmds := []tea.Cmd{}

	switch m.focusedPane {
	case 0:
		m.worktreeTable, cmd = m.worktreeTable.Update(msg)
		cmds = append(cmds, cmd)
		cmds = append(cmds, m.debouncedUpdateDetailsView())
	case 1:
		m.statusViewport, cmd = m.statusViewport.Update(msg)
		cmds = append(cmds, cmd)
	default:
		m.logTable, cmd = m.logTable.Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m *Model) handleFilterNavigation(keyStr string, fillInput bool) (tea.Model, tea.Cmd) {
	// When fillInput is true (Alt+n/Alt+p), navigate through all worktrees
	// When fillInput is false (Up/Down), navigate through filtered worktrees only
	var workList []*models.WorktreeInfo
	if fillInput {
		// Alt+n/Alt+p: navigate through all worktrees (sorted)
		workList = make([]*models.WorktreeInfo, len(m.worktrees))
		copy(workList, m.worktrees)
		if m.sortByActive {
			sort.Slice(workList, func(i, j int) bool {
				return workList[i].LastActiveTS > workList[j].LastActiveTS
			})
		} else {
			sort.Slice(workList, func(i, j int) bool {
				return workList[i].Path < workList[j].Path
			})
		}
	} else {
		// Up/Down: navigate through filtered worktrees
		workList = m.filteredWts
	}

	if len(workList) == 0 {
		return m, nil
	}

	// Find current position
	currentPath := ""
	if !fillInput {
		// For filtered navigation, use table cursor
		currentIndex := m.worktreeTable.Cursor()
		if currentIndex >= 0 && currentIndex < len(m.filteredWts) {
			currentPath = m.filteredWts[currentIndex].Path
		}
	} else {
		// For all-worktree navigation, find current selection
		if m.selectedIndex >= 0 && m.selectedIndex < len(m.filteredWts) {
			currentPath = m.filteredWts[m.selectedIndex].Path
		}
		if currentPath == "" {
			cursor := m.worktreeTable.Cursor()
			if cursor >= 0 && cursor < len(m.filteredWts) {
				currentPath = m.filteredWts[cursor].Path
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
	m.filterInput.SetValue(name)
	m.filterInput.CursorEnd()
	m.filterQuery = name
	m.updateTable()
}

func (m *Model) selectFilteredWorktree(path string) {
	if path == "" {
		return
	}
	for i, wt := range m.filteredWts {
		if wt.Path == path {
			m.worktreeTable.SetCursor(i)
			m.selectedIndex = i
			return
		}
	}
}

// handlePageDown processes page down navigation.
func (m *Model) handlePageDown(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch m.focusedPane {
	case 1:
		m.statusViewport.HalfPageDown()
		return m, nil
	case 2:
		m.logTable, cmd = m.logTable.Update(msg)
		return m, cmd
	}
	return m, nil
}

// handlePageUp processes page up navigation.
func (m *Model) handlePageUp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch m.focusedPane {
	case 1:
		m.statusViewport.HalfPageUp()
		return m, nil
	case 2:
		m.logTable, cmd = m.logTable.Update(msg)
		return m, cmd
	}
	return m, nil
}

// handleEnterKey processes the Enter key based on focused pane.
func (m *Model) handleEnterKey() (tea.Model, tea.Cmd) {
	switch m.focusedPane {
	case 0:
		// Jump to worktree
		if m.selectedIndex >= 0 && m.selectedIndex < len(m.filteredWts) {
			selectedPath := m.filteredWts[m.selectedIndex].Path
			m.persistLastSelected(selectedPath)
			m.selectedPath = selectedPath
			return m, tea.Quit
		}
	case 2:
		// Open commit view
		return m, m.openCommitView()
	}
	return m, nil
}

// handleMouse processes mouse events for scrolling and clicking
func (m *Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Handle mouse scrolling for specific screens
	if m.currentScreen == screenCommit && m.commitScreen != nil {
		if msg.Action == tea.MouseActionPress {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				m.commitScreen.viewport.ScrollUp(3)
				return m, nil
			case tea.MouseButtonWheelDown:
				m.commitScreen.viewport.ScrollDown(3)
				return m, nil
			}
		}
		return m, nil
	}

	// Skip mouse handling when on other modal screens
	if m.currentScreen != screenNone {
		return m, nil
	}

	var cmds []tea.Cmd
	layout := m.computeLayout()

	// Calculate pane boundaries (accounting for header and filter)
	headerOffset := 1
	if m.showingFilter {
		headerOffset = 2
	}

	// Left pane boundaries (worktree table)
	leftX := 0
	leftY := headerOffset
	leftMaxX := layout.leftWidth
	leftMaxY := headerOffset + layout.bodyHeight

	// Right top pane boundaries (info/diff viewport)
	rightX := layout.leftWidth + layout.gapX
	rightTopY := headerOffset
	rightTopMaxX := rightX + layout.rightWidth
	rightTopMaxY := headerOffset + layout.rightTopHeight

	// Right bottom pane boundaries (log table)
	rightBottomY := headerOffset + layout.rightTopHeight + layout.gapY
	rightBottomMaxY := headerOffset + layout.bodyHeight

	// Determine which pane the mouse is in
	mouseX := msg.X
	mouseY := msg.Y

	targetPane := -1
	switch {
	case mouseX >= leftX && mouseX < leftMaxX && mouseY >= leftY && mouseY < leftMaxY:
		targetPane = 0 // Worktree table
	case mouseX >= rightX && mouseX < rightTopMaxX && mouseY >= rightTopY && mouseY < rightTopMaxY:
		targetPane = 1 // Info/Diff viewport
	case mouseX >= rightX && mouseX < rightTopMaxX && mouseY >= rightBottomY && mouseY < rightBottomMaxY:
		targetPane = 2 // Log table
	}

	switch {
	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft:
		// Click to focus pane and select item
		if targetPane >= 0 && targetPane != m.focusedPane {
			m.focusedPane = targetPane
			switch m.focusedPane {
			case 0:
				m.worktreeTable.Focus()
			case 2:
				m.logTable.Focus()
			}
		}

		// Handle clicks within the pane to select items
		if targetPane == 0 && len(m.filteredWts) > 0 {
			// Calculate which row was clicked in the worktree table
			// Account for pane border and title
			relativeY := mouseY - leftY - 4
			if relativeY >= 0 && relativeY < len(m.filteredWts) {
				// Create a key message to move cursor
				for i := 0; i < len(m.filteredWts); i++ {
					if i == relativeY {
						m.worktreeTable.SetCursor(i)
						cmds = append(cmds, m.debouncedUpdateDetailsView())
						break
					}
				}
			}
		} else if targetPane == 2 && len(m.logEntries) > 0 {
			// Calculate which row was clicked in the log table
			relativeY := mouseY - rightBottomY - 4
			if relativeY >= 0 && relativeY < len(m.logEntries) {
				m.logTable.SetCursor(relativeY)
			}
		}

	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonWheelUp:
		switch targetPane {
		case 0:
			// Scroll worktree table up
			m.worktreeTable, _ = m.worktreeTable.Update(tea.KeyMsg{Type: tea.KeyUp})
			cmds = append(cmds, m.debouncedUpdateDetailsView())
		case 1:
			// Scroll viewport up
			m.statusViewport.ScrollUp(3)
		case 2:
			// Scroll log table up
			m.logTable, _ = m.logTable.Update(tea.KeyMsg{Type: tea.KeyUp})
		}

	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonWheelDown:
		switch targetPane {
		case 0:
			// Scroll worktree table down
			m.worktreeTable, _ = m.worktreeTable.Update(tea.KeyMsg{Type: tea.KeyDown})
			cmds = append(cmds, m.debouncedUpdateDetailsView())
		case 1:
			// Scroll viewport down
			m.statusViewport.ScrollDown(3)
		case 2:
			// Scroll log table down
			m.logTable, _ = m.logTable.Update(tea.KeyMsg{Type: tea.KeyDown})
		}
	}

	return m, tea.Batch(cmds...)
}
