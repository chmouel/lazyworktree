package app

import (
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/chmouel/lazyworktree/internal/models"
)

func (m *Model) updateWorktreeStatus(path string, files []StatusFile) {
	if path == "" {
		return
	}
	var target *models.WorktreeInfo
	for _, wt := range m.worktrees {
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

func (m *Model) setStatusFiles(files []StatusFile) {
	m.statusFilesAll = files

	// Initialize collapsed dirs map if needed
	if m.statusCollapsedDirs == nil {
		m.statusCollapsedDirs = make(map[string]bool)
	}

	m.applyStatusFilter()
}

func (m *Model) applyStatusFilter() {
	query := strings.ToLower(strings.TrimSpace(m.statusFilterQuery))
	filtered := m.statusFilesAll
	if query != "" {
		filtered = make([]StatusFile, 0, len(m.statusFilesAll))
		for _, sf := range m.statusFilesAll {
			if strings.Contains(strings.ToLower(sf.Filename), query) {
				filtered = append(filtered, sf)
			}
		}
	}

	// Remember current selection (by path)
	selectedPath := ""
	if m.statusTreeIndex >= 0 && m.statusTreeIndex < len(m.statusTreeFlat) {
		selectedPath = m.statusTreeFlat[m.statusTreeIndex].Path
	}

	// Keep statusFiles for compatibility
	m.statusFiles = filtered

	// Build tree from filtered files
	m.statusTree = buildStatusTree(filtered)
	m.rebuildStatusTreeFlat()

	// Try to restore selection
	if selectedPath != "" {
		for i, node := range m.statusTreeFlat {
			if node.Path == selectedPath {
				m.statusTreeIndex = i
				break
			}
		}
	}

	// Clamp tree index
	if m.statusTreeIndex < 0 {
		m.statusTreeIndex = 0
	}
	if len(m.statusTreeFlat) > 0 && m.statusTreeIndex >= len(m.statusTreeFlat) {
		m.statusTreeIndex = len(m.statusTreeFlat) - 1
	}
	if len(m.statusTreeFlat) == 0 {
		m.statusTreeIndex = 0
	}

	// Keep old statusFileIndex in sync for compatibility
	m.statusFileIndex = m.statusTreeIndex

	m.rebuildStatusContentWithHighlight()
}

// buildStatusTree builds a tree structure from a flat list of files.
// Files are grouped by directory, with directories sorted before files.
func buildStatusTree(files []StatusFile) *StatusTreeNode {
	if len(files) == 0 {
		return &StatusTreeNode{Path: "", Children: nil}
	}

	root := &StatusTreeNode{Path: "", Children: make([]*StatusTreeNode, 0)}
	childrenByPath := make(map[string]*StatusTreeNode)

	for i := range files {
		file := &files[i]
		parts := strings.Split(file.Filename, "/")

		current := root
		for j := range parts {
			isFile := j == len(parts)-1
			pathSoFar := strings.Join(parts[:j+1], "/")

			if existing, ok := childrenByPath[pathSoFar]; ok {
				current = existing
				continue
			}

			var newNode *StatusTreeNode
			if isFile {
				newNode = &StatusTreeNode{
					Path: pathSoFar,
					File: file,
				}
			} else {
				newNode = &StatusTreeNode{
					Path:     pathSoFar,
					Children: make([]*StatusTreeNode, 0),
				}
			}
			current.Children = append(current.Children, newNode)
			childrenByPath[pathSoFar] = newNode
			current = newNode
		}
	}

	sortStatusTree(root)
	compressStatusTree(root)
	return root
}

// sortStatusTree sorts tree nodes: directories first, then alphabetically.
func sortStatusTree(node *StatusTreeNode) {
	if node == nil || node.Children == nil {
		return
	}

	sort.Slice(node.Children, func(i, j int) bool {
		iIsDir := node.Children[i].File == nil
		jIsDir := node.Children[j].File == nil
		if iIsDir != jIsDir {
			return iIsDir // directories first
		}
		return node.Children[i].Path < node.Children[j].Path
	})

	for _, child := range node.Children {
		sortStatusTree(child)
	}
}

// compressStatusTree squashes single-child directory chains (e.g., a/b/c becomes one node).
func compressStatusTree(node *StatusTreeNode) {
	if node == nil {
		return
	}

	for _, child := range node.Children {
		compressStatusTree(child)
	}

	// Compress children that are single-child directories
	for i, child := range node.Children {
		for child.File == nil && len(child.Children) == 1 && child.Children[0].File == nil {
			grandchild := child.Children[0]
			grandchild.Compression = child.Compression + 1
			node.Children[i] = grandchild
			child = grandchild
		}
	}
}

// flattenStatusTree returns visible nodes respecting collapsed state.
func flattenStatusTree(node *StatusTreeNode, collapsed map[string]bool, depth int) []*StatusTreeNode {
	if node == nil {
		return nil
	}

	result := make([]*StatusTreeNode, 0)

	// Skip root node itself but process its children
	if node.Path != "" {
		nodeCopy := *node
		nodeCopy.depth = depth
		result = append(result, &nodeCopy)

		// If collapsed, don't include children
		if collapsed[node.Path] {
			return result
		}
	}

	if node.Children != nil {
		childDepth := depth
		if node.Path != "" {
			childDepth = depth + 1
		}
		for _, child := range node.Children {
			result = append(result, flattenStatusTree(child, collapsed, childDepth)...)
		}
	}

	return result
}

// IsDir returns true if this node is a directory.
func (n *StatusTreeNode) IsDir() bool {
	return n.File == nil
}

// Name returns the display name for this node.
func (n *StatusTreeNode) Name() string {
	return filepath.Base(n.Path)
}

// CollectFiles recursively collects all StatusFile pointers from this node and its children.
func (n *StatusTreeNode) CollectFiles() []*StatusFile {
	var files []*StatusFile
	if n.File != nil {
		files = append(files, n.File)
	}
	for _, child := range n.Children {
		files = append(files, child.CollectFiles()...)
	}
	return files
}

func (m *Model) rebuildStatusTreeFlat() {
	if m.statusCollapsedDirs == nil {
		m.statusCollapsedDirs = make(map[string]bool)
	}
	m.statusTreeFlat = flattenStatusTree(m.statusTree, m.statusCollapsedDirs, 0)
}

func (m *Model) rebuildStatusContentWithHighlight() {
	m.statusContent = m.renderStatusFiles()
	m.statusViewport.SetContent(m.statusContent)

	if len(m.statusTreeFlat) == 0 {
		return
	}

	// Auto-scroll to keep selected item visible
	viewportHeight := m.statusViewport.Height
	if viewportHeight > 0 && m.statusTreeIndex >= 0 {
		currentOffset := m.statusViewport.YOffset
		if m.statusTreeIndex < currentOffset {
			m.statusViewport.SetYOffset(m.statusTreeIndex)
		} else if m.statusTreeIndex >= currentOffset+viewportHeight {
			m.statusViewport.SetYOffset(m.statusTreeIndex - viewportHeight + 1)
		}
	}
}

func (m *Model) setLogEntries(entries []commitLogEntry, reset bool) {
	m.logEntriesAll = entries
	m.applyLogFilter(reset)
}

func (m *Model) applyLogFilter(reset bool) {
	query := strings.ToLower(strings.TrimSpace(m.logFilterQuery))
	filtered := m.logEntriesAll
	if query != "" {
		filtered = make([]commitLogEntry, 0, len(m.logEntriesAll))
		for _, entry := range m.logEntriesAll {
			if strings.Contains(strings.ToLower(entry.message), query) {
				filtered = append(filtered, entry)
			}
		}
	}

	selectedSHA := ""
	if !reset {
		cursor := m.logTable.Cursor()
		if cursor >= 0 && cursor < len(m.logEntries) {
			selectedSHA = m.logEntries[cursor].sha
		}
	}

	m.logEntries = filtered
	rows := make([]table.Row, 0, len(filtered))
	for _, entry := range filtered {
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
		}

		rows = append(rows, table.Row{sha, initials, msg})
	}
	m.logTable.SetRows(rows)

	if selectedSHA != "" {
		for i, entry := range m.logEntries {
			if entry.sha == selectedSHA {
				m.logTable.SetCursor(i)
				return
			}
		}
	}
	if len(m.logEntries) > 0 {
		if m.logTable.Cursor() < 0 || m.logTable.Cursor() >= len(m.logEntries) || reset {
			m.logTable.SetCursor(0)
		}
	} else {
		m.logTable.SetCursor(0)
	}
}

func (m *Model) getDetailsCache(cacheKey string) (*detailsCacheEntry, bool) {
	m.detailsCacheMu.RLock()
	defer m.detailsCacheMu.RUnlock()
	cached, ok := m.detailsCache[cacheKey]
	return cached, ok
}

func (m *Model) setDetailsCache(cacheKey string, entry *detailsCacheEntry) {
	m.detailsCacheMu.Lock()
	defer m.detailsCacheMu.Unlock()
	if m.detailsCache == nil {
		m.detailsCache = make(map[string]*detailsCacheEntry)
	}
	m.detailsCache[cacheKey] = entry
}

func (m *Model) deleteDetailsCache(cacheKey string) {
	m.detailsCacheMu.Lock()
	defer m.detailsCacheMu.Unlock()
	delete(m.detailsCache, cacheKey)
}

func (m *Model) resetDetailsCache() {
	m.detailsCacheMu.Lock()
	defer m.detailsCacheMu.Unlock()
	m.detailsCache = make(map[string]*detailsCacheEntry)
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

	// Get status (using porcelain format for reliable machine parsing)
	statusRaw := m.git.RunGit(m.ctx, []string{"git", "status", "--porcelain=v2"}, wt.Path, []int{0}, true, false)
	// Use %H for full SHA to ensure reliable matching
	logRaw := m.git.RunGit(m.ctx, []string{"git", "log", "-50", "--pretty=format:%H%x09%an%x09%s"}, wt.Path, []int{0}, true, false)

	// Get unpushed SHAs (commits not on any remote)
	unpushedRaw := m.git.RunGit(m.ctx, []string{"git", "rev-list", "-100", "HEAD", "--not", "--remotes"}, wt.Path, []int{0}, true, false)
	unpushedSHAs := make(map[string]bool)
	for sha := range strings.SplitSeq(unpushedRaw, "\n") {
		if s := strings.TrimSpace(sha); s != "" {
			unpushedSHAs[s] = true
		}
	}

	// Get unmerged SHAs (commits not in main branch)
	mainBranch := m.git.GetMainBranch(m.ctx)
	unmergedSHAs := make(map[string]bool)
	if mainBranch != "" {
		unmergedRaw := m.git.RunGit(m.ctx, []string{"git", "rev-list", "-100", "HEAD", "^" + mainBranch}, wt.Path, []int{0}, true, false)
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
