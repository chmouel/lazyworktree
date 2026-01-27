package app

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/chmouel/lazyworktree/internal/models"
)

func (m *Model) inputLabel() string {
	if m.showingSearch {
		return m.searchLabel()
	}
	return m.filterLabel()
}

func (m *Model) searchLabel() string {
	showIcons := m.config.IconsEnabled()
	switch m.searchTarget {
	case searchTargetStatus:
		return labelWithIcon(UIIconSearch, "Search Files", showIcons)
	case searchTargetLog:
		return labelWithIcon(UIIconSearch, "Search Commits", showIcons)
	default:
		return labelWithIcon(UIIconSearch, "Search Worktrees", showIcons)
	}
}

func (m *Model) filterLabel() string {
	showIcons := m.config.IconsEnabled()
	switch m.filterTarget {
	case filterTargetStatus:
		return labelWithIcon(UIIconFilter, "Filter Files", showIcons)
	case filterTargetLog:
		return labelWithIcon(UIIconFilter, "Filter Commits", showIcons)
	default:
		return labelWithIcon(UIIconFilter, "Filter Worktrees", showIcons)
	}
}

func (m *Model) filterPlaceholder(target filterTarget) string {
	switch target {
	case filterTargetStatus:
		return placeholderFilterFiles
	case filterTargetLog:
		return "Filter commits..."
	default:
		return filterWorktreesPlaceholder
	}
}

func (m *Model) filterQueryForTarget(target filterTarget) string {
	switch target {
	case filterTargetStatus:
		return m.statusFilterQuery
	case filterTargetLog:
		return m.logFilterQuery
	default:
		return m.filterQuery
	}
}

func (m *Model) setFilterQuery(target filterTarget, query string) {
	switch target {
	case filterTargetStatus:
		m.statusFilterQuery = query
	case filterTargetLog:
		m.logFilterQuery = query
	default:
		m.filterQuery = query
	}
}

func (m *Model) hasActiveFilterForPane(paneIndex int) bool {
	switch paneIndex {
	case 0:
		return strings.TrimSpace(m.filterQuery) != ""
	case 1:
		return strings.TrimSpace(m.statusFilterQuery) != ""
	case 2:
		return strings.TrimSpace(m.logFilterQuery) != ""
	}
	return false
}

func (m *Model) setFilterTarget(target filterTarget) {
	m.filterTarget = target
	m.filterInput.Placeholder = m.filterPlaceholder(target)
	m.filterInput.SetValue(m.filterQueryForTarget(target))
	m.filterInput.CursorEnd()
}

func (m *Model) searchPlaceholder(target searchTarget) string {
	switch target {
	case searchTargetStatus:
		return searchFiles
	case searchTargetLog:
		return "Search commit titles..."
	default:
		return "Search worktrees..."
	}
}

func (m *Model) searchQueryForTarget(target searchTarget) string {
	switch target {
	case searchTargetStatus:
		return m.statusSearchQuery
	case searchTargetLog:
		return m.logSearchQuery
	default:
		return m.worktreeSearchQuery
	}
}

func (m *Model) setSearchQuery(target searchTarget, query string) {
	switch target {
	case searchTargetStatus:
		m.statusSearchQuery = query
	case searchTargetLog:
		m.logSearchQuery = query
	default:
		m.worktreeSearchQuery = query
	}
}

func (m *Model) setSearchTarget(target searchTarget) {
	m.searchTarget = target
	m.filterInput.Placeholder = m.searchPlaceholder(target)
	m.filterInput.SetValue(m.searchQueryForTarget(target))
	m.filterInput.CursorEnd()
}

func (m *Model) startSearch(target searchTarget) tea.Cmd {
	m.showingSearch = true
	m.showingFilter = false
	m.setSearchTarget(target)
	m.filterInput.Focus()
	return textinput.Blink
}

func (m *Model) startFilter(target filterTarget) tea.Cmd {
	m.showingFilter = true
	m.showingSearch = false
	m.setFilterTarget(target)
	m.filterInput.Focus()
	return textinput.Blink
}

func (m *Model) updateTable() {
	// Filter worktrees
	query := strings.ToLower(strings.TrimSpace(m.filterQuery))
	m.filteredWts = []*models.WorktreeInfo{}

	if query == "" {
		m.filteredWts = m.worktrees
	} else {
		hasPathSep := strings.Contains(query, "/")
		for _, wt := range m.worktrees {
			name := filepath.Base(wt.Path)
			if wt.IsMain {
				name = mainWorktreeName
			}
			haystacks := []string{strings.ToLower(name), strings.ToLower(wt.Branch)}
			if hasPathSep {
				haystacks = append(haystacks, strings.ToLower(wt.Path))
			}
			for _, haystack := range haystacks {
				if strings.Contains(haystack, query) {
					m.filteredWts = append(m.filteredWts, wt)
					break
				}
			}
		}
	}

	// Sort based on current sort mode
	switch m.sortMode {
	case sortModeLastActive:
		sort.Slice(m.filteredWts, func(i, j int) bool {
			return m.filteredWts[i].LastActiveTS > m.filteredWts[j].LastActiveTS
		})
	case sortModeLastSwitched:
		sort.Slice(m.filteredWts, func(i, j int) bool {
			return m.filteredWts[i].LastSwitchedTS > m.filteredWts[j].LastSwitchedTS
		})
	default: // sortModePath
		sort.Slice(m.filteredWts, func(i, j int) bool {
			return m.filteredWts[i].Path < m.filteredWts[j].Path
		})
	}

	// Update table rows
	showIcons := m.config.IconsEnabled()
	rows := make([]table.Row, 0, len(m.filteredWts))
	for _, wt := range m.filteredWts {
		name := filepath.Base(wt.Path)
		worktreeIcon := UIIconWorktree
		if wt.IsMain {
			worktreeIcon = UIIconWorktreeMain
			name = mainWorktreeName
		}
		if showIcons {
			name = iconPrefix(worktreeIcon, showIcons) + name
		} else {
			name = " " + name
		}

		// Truncate to configured max length with ellipsis if needed
		if m.config.MaxNameLength > 0 {
			nameRunes := []rune(name)
			if len(nameRunes) > m.config.MaxNameLength {
				name = string(nameRunes[:m.config.MaxNameLength]) + "..."
			}
		}

		statusStr := combinedStatusIndicator(wt.Dirty, wt.HasUpstream, wt.Ahead, wt.Behind, wt.Unpushed, showIcons, m.config.IconSet)

		row := table.Row{
			name,
			statusStr,
			wt.LastActive,
		}

		// Only include PR column if PR data has been loaded
		if m.prDataLoaded {
			prStr := "-"
			if wt.PR != nil {
				prIcon := ""
				if showIcons {
					prIcon = iconWithSpace(getIconPR())
				}
				stateSymbol := prStateIndicator(wt.PR.State, showIcons)
				// Right-align PR numbers for consistent column width
				prStr = fmt.Sprintf("%s#%-5d%s", prIcon, wt.PR.Number, stateSymbol)
			}
			row = append(row, prStr)
		}

		rows = append(rows, row)
	}

	m.worktreeTable.SetRows(rows)
	if len(m.filteredWts) > 0 && m.selectedIndex >= len(m.filteredWts) {
		m.selectedIndex = len(m.filteredWts) - 1
	}
	if len(m.filteredWts) > 0 {
		cursor := max(m.worktreeTable.Cursor(), 0)
		if cursor >= len(m.filteredWts) {
			cursor = len(m.filteredWts) - 1
		}
		m.selectedIndex = cursor
		m.worktreeTable.SetCursor(cursor)
	}
	m.updateWorktreeArrows()
}

// updateWorktreeArrows updates the arrow indicator on the selected row.
func (m *Model) updateWorktreeArrows() {
	rows := m.worktreeTable.Rows()
	cursor := m.worktreeTable.Cursor()
	for i, row := range rows {
		if len(row) > 0 && row[0] != "" {
			runes := []rune(row[0])
			if len(runes) > 0 {
				// Replace first rune with arrow or space
				if i == cursor {
					runes[0] = '›'
				} else {
					runes[0] = ' '
				}
				rows[i][0] = string(runes)
			}
		}
	}
	m.worktreeTable.SetRows(rows)
}

func (m *Model) updateDetailsView() tea.Cmd {
	m.selectedIndex = m.worktreeTable.Cursor()
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}

	// Reset CI check selection when worktree changes
	m.ciCheckIndex = -1

	wt := m.filteredWts[m.selectedIndex]
	if !m.worktreesLoaded {
		m.infoContent = m.buildInfoContent(wt)
		if m.statusContent == "" || m.statusContent == "Loading..." {
			m.statusContent = loadingRefreshWorktrees
		}
		return nil
	}
	return func() tea.Msg {
		statusRaw, logRaw, unpushed, unmerged := m.getCachedDetails(wt)

		// Parse log
		logEntries := []commitLogEntry{}
		for line := range strings.SplitSeq(logRaw, "\n") {
			parts := strings.SplitN(line, "\t", 3)
			if len(parts) < 2 {
				continue
			}
			sha := parts[0]
			message := parts[len(parts)-1]
			author := ""
			if len(parts) == 3 {
				author = parts[1]
			}
			logEntries = append(logEntries, commitLogEntry{
				sha:            sha,
				authorInitials: authorInitials(author),
				message:        message,
				isUnpushed:     unpushed[sha],
				isUnmerged:     unmerged[sha],
			})
		}
		return statusUpdatedMsg{
			info:        m.buildInfoContent(wt),
			statusFiles: parseStatusFiles(statusRaw),
			log:         logEntries,
			path:        wt.Path,
		}
	}
}

func (m *Model) debouncedUpdateDetailsView() tea.Cmd {
	// Cancel any existing pending detail update
	if m.detailUpdateCancel != nil {
		m.detailUpdateCancel()
		m.detailUpdateCancel = nil
	}

	// Get current selected index
	m.pendingDetailsIndex = m.worktreeTable.Cursor()
	selectedIndex := m.pendingDetailsIndex

	ctx, cancel := context.WithCancel(context.Background())
	m.detailUpdateCancel = cancel

	return func() tea.Msg {
		timer := time.NewTimer(debounceDelay)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
		}
		return debouncedDetailsMsg{
			selectedIndex: selectedIndex,
		}
	}
}

func (m *Model) refreshWorktrees() tea.Cmd {
	return func() tea.Msg {
		worktrees, err := m.git.GetWorktrees(m.ctx)
		return worktreesLoadedMsg{
			worktrees: worktrees,
			err:       err,
		}
	}
}

func findMatchIndex(count, start int, forward bool, matches func(int) bool) int {
	if count == 0 {
		return -1
	}
	if start < 0 {
		if forward {
			start = 0
		} else {
			start = count - 1
		}
	} else if count > 0 {
		start %= count
	}
	for i := range count {
		var idx int
		if forward {
			idx = (start + i) % count
		} else {
			idx = (start - i + count) % count
		}
		if matches(idx) {
			return idx
		}
	}
	return -1
}

func (m *Model) findWorktreeMatchIndex(query string, start int, forward bool) int {
	lowerQuery := strings.ToLower(strings.TrimSpace(query))
	if lowerQuery == "" {
		return -1
	}
	hasPathSep := strings.Contains(lowerQuery, "/")
	return findMatchIndex(len(m.filteredWts), start, forward, func(i int) bool {
		wt := m.filteredWts[i]
		name := filepath.Base(wt.Path)
		if wt.IsMain {
			name = mainWorktreeName
		}
		if strings.Contains(strings.ToLower(name), lowerQuery) {
			return true
		}
		if strings.Contains(strings.ToLower(wt.Branch), lowerQuery) {
			return true
		}
		return hasPathSep && strings.Contains(strings.ToLower(wt.Path), lowerQuery)
	})
}

func (m *Model) findStatusMatchIndex(query string, start int, forward bool) int {
	lowerQuery := strings.ToLower(strings.TrimSpace(query))
	if lowerQuery == "" {
		return -1
	}
	return findMatchIndex(len(m.statusTreeFlat), start, forward, func(i int) bool {
		return strings.Contains(strings.ToLower(m.statusTreeFlat[i].Path), lowerQuery)
	})
}

func (m *Model) findLogMatchIndex(query string, start int, forward bool) int {
	lowerQuery := strings.ToLower(strings.TrimSpace(query))
	if lowerQuery == "" {
		return -1
	}
	return findMatchIndex(len(m.logEntries), start, forward, func(i int) bool {
		return strings.Contains(strings.ToLower(m.logEntries[i].message), lowerQuery)
	})
}

func (m *Model) applySearchQuery(query string) tea.Cmd {
	switch m.searchTarget {
	case searchTargetStatus:
		if idx := m.findStatusMatchIndex(query, 0, true); idx >= 0 {
			m.statusTreeIndex = idx
			m.rebuildStatusContentWithHighlight()
		}
	case searchTargetLog:
		if idx := m.findLogMatchIndex(query, 0, true); idx >= 0 {
			m.logTable.SetCursor(idx)
		}
	default:
		if idx := m.findWorktreeMatchIndex(query, 0, true); idx >= 0 {
			m.worktreeTable.SetCursor(idx)
			m.selectedIndex = idx
			return m.debouncedUpdateDetailsView()
		}
	}
	return nil
}

func (m *Model) advanceSearchMatch(forward bool) tea.Cmd {
	query := strings.TrimSpace(m.searchQueryForTarget(m.searchTarget))
	if query == "" {
		return nil
	}
	switch m.searchTarget {
	case searchTargetStatus:
		start := m.statusTreeIndex
		if forward {
			start++
		} else {
			start--
		}
		if idx := m.findStatusMatchIndex(query, start, forward); idx >= 0 {
			m.statusTreeIndex = idx
			m.rebuildStatusContentWithHighlight()
		}
	case searchTargetLog:
		start := m.logTable.Cursor()
		if forward {
			start++
		} else {
			start--
		}
		if idx := m.findLogMatchIndex(query, start, forward); idx >= 0 {
			m.logTable.SetCursor(idx)
		}
	default:
		start := m.worktreeTable.Cursor()
		if forward {
			start++
		} else {
			start--
		}
		if idx := m.findWorktreeMatchIndex(query, start, forward); idx >= 0 {
			m.worktreeTable.SetCursor(idx)
			m.selectedIndex = idx
			return m.debouncedUpdateDetailsView()
		}
	}
	return nil
}
