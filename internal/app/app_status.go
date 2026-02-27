package app

import (
	"strings"
	"sync"
	"time"

	"charm.land/bubbles/v2/table"
	"github.com/chmouel/lazyworktree/internal/app/services"
	"github.com/chmouel/lazyworktree/internal/models"
)

func (m *Model) updateWorktreeStatus(path string, files []StatusFile) {
	if path == "" {
		return
	}
	var target *models.WorktreeInfo
	for _, wt := range m.state.data.worktrees {
		if wt.Path == path {
			target = wt
			break
		}
	}
	if target == nil {
		return
	}
	staged, modified, untracked := statusCounts(files)
	dirty := staged+modified+untracked > 0
	if target.Dirty == dirty && target.Staged == staged && target.Modified == modified && target.Untracked == untracked {
		return
	}
	target.Dirty = dirty
	target.Staged = staged
	target.Modified = modified
	target.Untracked = untracked
	m.updateTable()
}

func parseStatusFiles(statusRaw string) []StatusFile {
	statusRaw = strings.TrimRight(statusRaw, "\n")
	if strings.TrimSpace(statusRaw) == "" {
		return nil
	}

	// Parse all files into statusFiles
	statusLines := strings.Split(statusRaw, "\n")
	parsedFiles := make([]StatusFile, 0, len(statusLines))
	for _, line := range statusLines {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse git status --porcelain=v2 format
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		var status, filename string
		var isUntracked bool

		switch fields[0] {
		case "1": // Ordinary changed entry: 1 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <path>
			if len(fields) < 9 {
				continue
			}
			status = fields[1] // XY status code (e.g., ".M", "M.", "MM")
			filename = fields[8]
		case "?": // Untracked: ? <path>
			status = " ?" // Single ? with space for alignment
			filename = fields[1]
			isUntracked = true
		case "2": // Renamed/copied: 2 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <X><score> <path><sep><origPath>
			if len(fields) < 10 {
				continue
			}
			status = fields[1]
			filename = fields[9]
		default:
			continue // Skip unhandled entry types
		}

		parsedFiles = append(parsedFiles, StatusFile{
			Filename:    filename,
			Status:      status,
			IsUntracked: isUntracked,
		})
	}

	return parsedFiles
}

func statusCounts(files []StatusFile) (staged, modified, untracked int) {
	for _, file := range files {
		if file.IsUntracked {
			untracked++
			continue
		}
		if file.Status != "" {
			first := file.Status[0]
			if first != '.' && first != ' ' {
				staged++
			}
		}
		if len(file.Status) > 1 {
			second := file.Status[1]
			if second != '.' && second != ' ' {
				modified++
			}
		}
	}
	return staged, modified, untracked
}

func (m *Model) hasGitStatus() bool {
	return len(m.state.data.statusFilesAll) > 0
}

func (m *Model) setStatusFiles(files []StatusFile) {
	m.state.data.statusFilesAll = files

	m.applyStatusFilter()

	// If status became clean and the git status pane is focused, reset focus
	if !m.hasGitStatus() {
		if m.state.view.FocusedPane == 2 {
			m.state.view.FocusedPane = 0
			m.state.ui.worktreeTable.Focus()
		}
		if m.state.view.ZoomedPane == 2 {
			m.state.view.ZoomedPane = -1
		}
	}
}

func (m *Model) applyStatusFilter() {
	query := strings.ToLower(strings.TrimSpace(m.state.services.filter.StatusFilterQuery))
	filtered := m.state.data.statusFilesAll
	if query != "" {
		filtered = make([]StatusFile, 0, len(m.state.data.statusFilesAll))
		for _, sf := range m.state.data.statusFilesAll {
			if strings.Contains(strings.ToLower(sf.Filename), query) {
				filtered = append(filtered, sf)
			}
		}
	}

	// Remember current selection (by path)
	selectedPath := m.state.services.statusTree.SelectedPath()

	// Keep statusFiles for compatibility
	m.state.data.statusFiles = filtered

	// Build tree from filtered files
	m.state.services.statusTree.Tree = services.BuildStatusTree(filtered)
	m.state.services.statusTree.RebuildFlat()

	// Try to restore selection
	m.state.services.statusTree.RestoreSelection(selectedPath)

	// Clamp tree index
	m.state.services.statusTree.ClampIndex()

	// Keep old statusFileIndex in sync for compatibility
	m.state.data.statusFileIndex = m.state.services.statusTree.Index

	m.rebuildStatusContentWithHighlight()
}

func (m *Model) rebuildStatusTreeFlat() {
	m.state.services.statusTree.RebuildFlat()
}

func (m *Model) rebuildStatusContentWithHighlight() {
	m.statusContent = m.renderStatusFiles()
	m.state.ui.statusViewport.SetContent(m.statusContent)

	if len(m.state.services.statusTree.TreeFlat) == 0 {
		return
	}

	// Auto-scroll to keep selected item visible
	viewportHeight := m.state.ui.statusViewport.Height()
	if viewportHeight > 0 && m.state.services.statusTree.Index >= 0 {
		currentOffset := m.state.ui.statusViewport.YOffset()
		if m.state.services.statusTree.Index < currentOffset {
			m.state.ui.statusViewport.SetYOffset(m.state.services.statusTree.Index)
		} else if m.state.services.statusTree.Index >= currentOffset+viewportHeight {
			m.state.ui.statusViewport.SetYOffset(m.state.services.statusTree.Index - viewportHeight + 1)
		}
	}
}

func (m *Model) setLogEntries(entries []commitLogEntry, reset bool) {
	m.state.data.logEntriesAll = entries
	m.applyLogFilter(reset)
}

// buildLogRow creates a table.Row from a commitLogEntry.
// When styled is true and the entry is unpushed/unmerged, WarnFg colouring is applied.
// When styled is false, cells are left as plain text so the table's Selected style applies cleanly.
func (m *Model) buildLogRow(entry commitLogEntry, styled bool) table.Row {
	sha := entry.sha
	if len(sha) > 7 {
		sha = sha[:7]
	}
	msg := formatCommitMessage(entry.message)
	initials := authorInitials(entry.authorInitials)
	if entry.isUnpushed || entry.isUnmerged {
		showIcons := m.config.IconsEnabled()
		initials = aheadIndicator(showIcons)
		if showIcons {
			initials = iconWithSpace(initials)
		}
		if styled {
			sha = m.renderStyles.unpushedCommitStyle.Render(sha)
			initials = m.renderStyles.unpushedCommitStyle.Render(initials)
			msg = m.renderStyles.unpushedCommitStyle.Render(msg)
		}
	}
	return table.Row{sha, initials, msg}
}

// restyleLogRows swaps WarnFg styling between the previous and current cursor rows
// so the table's Selected highlight always shows cleanly on the cursor row.
// When the log pane is not focused, all unpushed rows keep their WarnFg styling
// since the table's Selected highlight is not visible.
// Follows the same pattern as updateWorktreeArrows.
func (m *Model) restyleLogRows() {
	rows := m.state.ui.logTable.Rows()
	cursor := m.state.ui.logTable.Cursor()

	// When the log pane is not focused, no row needs the plain treatment.
	if m.state.view.FocusedPane != 3 {
		cursor = -1
	}

	if len(rows) == 0 || len(m.state.data.logEntries) == 0 {
		m.lastLogCursor = -1
		return
	}

	if cursor == m.lastLogCursor {
		return
	}

	changed := false
	previous := m.lastLogCursor

	// Restore WarnFg on the old cursor row (it is no longer selected).
	if previous >= 0 && previous < len(rows) && previous < len(m.state.data.logEntries) {
		entry := m.state.data.logEntries[previous]
		if entry.isUnpushed || entry.isUnmerged {
			rows[previous] = m.buildLogRow(entry, true)
			changed = true
		}
	}

	// Strip WarnFg from the new cursor row so Selected style applies.
	if cursor >= 0 && cursor < len(rows) && cursor < len(m.state.data.logEntries) {
		entry := m.state.data.logEntries[cursor]
		if entry.isUnpushed || entry.isUnmerged {
			rows[cursor] = m.buildLogRow(entry, false)
			changed = true
		}
	}

	if changed {
		m.state.ui.logTable.SetRows(rows)
	}
	m.lastLogCursor = cursor
}

func (m *Model) applyLogFilter(reset bool) {
	query := strings.ToLower(strings.TrimSpace(m.state.services.filter.LogFilterQuery))
	filtered := m.state.data.logEntriesAll
	if query != "" {
		filtered = make([]commitLogEntry, 0, len(m.state.data.logEntriesAll))
		for _, entry := range m.state.data.logEntriesAll {
			if strings.Contains(strings.ToLower(entry.message), query) {
				filtered = append(filtered, entry)
			}
		}
	}

	selectedSHA := ""
	if !reset {
		cursor := m.state.ui.logTable.Cursor()
		if cursor >= 0 && cursor < len(m.state.data.logEntries) {
			selectedSHA = m.state.data.logEntries[cursor].sha
		}
	}

	m.state.data.logEntries = filtered
	m.ensureRenderStyles()

	// Determine the target cursor so the cursor row can be left unstyled.
	// When the log pane is not focused, all rows keep their styling.
	targetCursor := -1
	if m.state.view.FocusedPane == 3 {
		targetCursor = 0
		if selectedSHA != "" {
			for i, entry := range filtered {
				if entry.sha == selectedSHA {
					targetCursor = i
					break
				}
			}
		}
	}

	rows := make([]table.Row, 0, len(filtered))
	for i, entry := range filtered {
		// The cursor row gets plain text so the table's Selected style applies cleanly.
		styled := i != targetCursor
		rows = append(rows, m.buildLogRow(entry, styled))
	}
	m.state.ui.logTable.SetRows(rows)

	if selectedSHA != "" {
		for i, entry := range m.state.data.logEntries {
			if entry.sha == selectedSHA {
				m.state.ui.logTable.SetCursor(i)
				m.lastLogCursor = targetCursor
				return
			}
		}
	}
	if len(m.state.data.logEntries) > 0 {
		if m.state.ui.logTable.Cursor() < 0 || m.state.ui.logTable.Cursor() >= len(m.state.data.logEntries) || reset {
			m.state.ui.logTable.SetCursor(0)
		}
	} else {
		m.state.ui.logTable.SetCursor(0)
	}
	m.lastLogCursor = targetCursor
}

func (m *Model) getDetailsCache(cacheKey string) (*detailsCacheEntry, bool) {
	m.cache.detailsCacheMu.RLock()
	defer m.cache.detailsCacheMu.RUnlock()
	cached, ok := m.cache.detailsCache[cacheKey]
	return cached, ok
}

func (m *Model) setDetailsCache(cacheKey string, entry *detailsCacheEntry) {
	m.cache.detailsCacheMu.Lock()
	defer m.cache.detailsCacheMu.Unlock()
	if m.cache.detailsCache == nil {
		m.cache.detailsCache = make(map[string]*detailsCacheEntry)
	}
	m.cache.detailsCache[cacheKey] = entry
}

func (m *Model) deleteDetailsCache(cacheKey string) {
	m.cache.detailsCacheMu.Lock()
	defer m.cache.detailsCacheMu.Unlock()
	delete(m.cache.detailsCache, cacheKey)
}

func (m *Model) resetDetailsCache() {
	m.cache.detailsCacheMu.Lock()
	defer m.cache.detailsCacheMu.Unlock()
	m.cache.detailsCache = make(map[string]*detailsCacheEntry)
}

func (m *Model) getCachedDetails(wt *models.WorktreeInfo) (string, string, map[string]bool, map[string]bool) {
	if wt == nil || strings.TrimSpace(wt.Path) == "" {
		return "", "", nil, nil
	}

	cacheKey := wt.Path
	if cached, ok := m.getDetailsCache(cacheKey); ok {
		if time.Since(cached.fetchedAt) < detailsCacheTTL {
			return cached.statusRaw, cached.logRaw, cached.unpushedSHAs, cached.unmergedSHAs
		}
	}

	mainBranch := m.state.services.git.GetMainBranch(m.ctx)

	var statusRaw, logRaw, unpushedRaw, unmergedRaw string
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Get status (using porcelain format for reliable machine parsing)
		statusRaw = m.state.services.git.RunGit(m.ctx, []string{"git", "status", "--porcelain=v2"}, wt.Path, []int{0}, true, false)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Use %H for full SHA to ensure reliable matching
		logRaw = m.state.services.git.RunGit(m.ctx, []string{"git", "log", "-50", "--pretty=format:%H%x09%an%x09%s"}, wt.Path, []int{0}, true, false)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Get unpushed SHAs (commits not on any remote)
		unpushedRaw = m.state.services.git.RunGit(m.ctx, []string{"git", "rev-list", "-100", "HEAD", "--not", "--remotes"}, wt.Path, []int{0}, true, false)
	}()
	if mainBranch != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Get unmerged SHAs (commits not in main branch)
			unmergedRaw = m.state.services.git.RunGit(m.ctx, []string{"git", "rev-list", "-100", "HEAD", "^" + mainBranch}, wt.Path, []int{0}, true, false)
		}()
	}
	wg.Wait()

	unpushedSHAs := make(map[string]bool)
	for sha := range strings.SplitSeq(unpushedRaw, "\n") {
		if s := strings.TrimSpace(sha); s != "" {
			unpushedSHAs[s] = true
		}
	}

	unmergedSHAs := make(map[string]bool)
	if mainBranch != "" {
		for sha := range strings.SplitSeq(unmergedRaw, "\n") {
			if s := strings.TrimSpace(sha); s != "" {
				unmergedSHAs[s] = true
			}
		}
	}

	m.setDetailsCache(cacheKey, &detailsCacheEntry{
		statusRaw:    statusRaw,
		logRaw:       logRaw,
		unpushedSHAs: unpushedSHAs,
		unmergedSHAs: unmergedSHAs,
		fetchedAt:    time.Now(),
	})

	return statusRaw, logRaw, unpushedSHAs, unmergedSHAs
}
