package screen

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/theme"
)

// Additional key constants for commit files screen.
const (
	keyCtrlD = "ctrl+d"
	keyCtrlU = "ctrl+u"
	keyCtrlJ = "ctrl+j"
	keyCtrlK = "ctrl+k"
	keyDown  = "down"
	keyUp    = "up"

	placeholderFilterFiles = "Filter files..."
	searchFiles            = "Search files..."
)

// CommitFileTreeNode represents a node in the commit file tree.
type CommitFileTreeNode struct {
	Path        string
	File        *models.CommitFile // nil for directories
	Children    []*CommitFileTreeNode
	Compression int // Number of compressed path segments
	Depth       int // Cached depth for rendering
}

// IsDir returns true if this node is a directory.
func (n *CommitFileTreeNode) IsDir() bool {
	return n.File == nil
}

// CommitFilesScreen displays files changed in a commit as a collapsible tree.
// Note: Uses CommitMeta from commit.go for metadata display.
type CommitFilesScreen struct {
	CommitSHA     string
	WorktreePath  string
	Files         []models.CommitFile
	AllFiles      []models.CommitFile // Original unfiltered files
	Tree          *CommitFileTreeNode
	TreeFlat      []*CommitFileTreeNode
	CollapsedDirs map[string]bool
	Cursor        int
	ScrollOffset  int
	Width         int
	Height        int
	Thm           *theme.Theme
	ShowIcons     bool

	// Commit metadata
	Meta CommitMeta

	// Filter/search support
	FilterInput   textinput.Model
	ShowingFilter bool
	FilterQuery   string
	ShowingSearch bool
	SearchQuery   string

	// Callbacks
	OnShowFileDiff   func(filename string) tea.Cmd
	OnShowCommitDiff func() tea.Cmd
	OnClose          func() tea.Cmd
}

// NewCommitFilesScreen creates a commit files tree screen.
func NewCommitFilesScreen(sha, wtPath string, files []models.CommitFile, meta CommitMeta, maxWidth, maxHeight int, thm *theme.Theme, showIcons bool) *CommitFilesScreen {
	width := int(float64(maxWidth) * 0.8)
	height := int(float64(maxHeight) * 0.8)
	if width < 60 {
		width = 60
	}
	if height < 20 {
		height = 20
	}

	ti := textinput.New()
	ti.Placeholder = placeholderFilterFiles
	ti.CharLimit = 100
	ti.Prompt = "> "
	ti.Width = width - 6

	screen := &CommitFilesScreen{
		CommitSHA:     sha,
		WorktreePath:  wtPath,
		Files:         files,
		AllFiles:      files,
		CollapsedDirs: make(map[string]bool),
		Cursor:        0,
		ScrollOffset:  0,
		Width:         width,
		Height:        height,
		Thm:           thm,
		ShowIcons:     showIcons,
		Meta:          meta,
		FilterInput:   ti,
	}

	screen.Tree = BuildCommitFileTree(files)
	SortCommitFileTree(screen.Tree)
	for _, child := range screen.Tree.Children {
		CompressCommitFileTree(child)
	}
	screen.RebuildFlat()

	return screen
}

// Type returns TypeCommitFiles to identify this screen.
func (s *CommitFilesScreen) Type() Type {
	return TypeCommitFiles
}

// SetTheme updates the screen's theme.
func (s *CommitFilesScreen) SetTheme(thm *theme.Theme) {
	s.Thm = thm
}

// BuildCommitFileTree constructs a tree from a flat list of commit files.
func BuildCommitFileTree(files []models.CommitFile) *CommitFileTreeNode {
	root := &CommitFileTreeNode{
		Path:     "",
		Children: []*CommitFileTreeNode{},
	}

	for i := range files {
		file := &files[i]
		parts := strings.Split(file.Filename, "/")

		current := root
		for j := range parts {
			isLast := j == len(parts)-1
			partPath := strings.Join(parts[:j+1], "/")

			// Find or create child
			var child *CommitFileTreeNode
			for _, c := range current.Children {
				if c.Path == partPath {
					child = c
					break
				}
			}

			if child == nil {
				if isLast {
					child = &CommitFileTreeNode{
						Path: partPath,
						File: file,
					}
				} else {
					child = &CommitFileTreeNode{
						Path:     partPath,
						Children: []*CommitFileTreeNode{},
					}
				}
				current.Children = append(current.Children, child)
			}
			current = child
		}
	}

	return root
}

// SortCommitFileTree sorts nodes: directories first, then files, alphabetically.
func SortCommitFileTree(node *CommitFileTreeNode) {
	if node == nil || len(node.Children) == 0 {
		return
	}

	sort.Slice(node.Children, func(i, j int) bool {
		iIsDir := node.Children[i].IsDir()
		jIsDir := node.Children[j].IsDir()
		if iIsDir != jIsDir {
			return iIsDir
		}
		return node.Children[i].Path < node.Children[j].Path
	})

	for _, child := range node.Children {
		SortCommitFileTree(child)
	}
}

// CompressCommitFileTree compresses single-child directory chains.
func CompressCommitFileTree(node *CommitFileTreeNode) {
	if node == nil {
		return
	}

	for _, child := range node.Children {
		CompressCommitFileTree(child)
	}

	if node.IsDir() && len(node.Children) == 1 {
		child := node.Children[0]
		if child.IsDir() {
			node.Compression++
			node.Compression += child.Compression
			node.Children = child.Children
		}
	}
}

// RebuildFlat rebuilds the flat list from the tree respecting collapsed state.
func (s *CommitFilesScreen) RebuildFlat() {
	s.TreeFlat = []*CommitFileTreeNode{}
	s.flattenTree(s.Tree, 0)
}

// ApplyFilter filters the files list and rebuilds the tree.
func (s *CommitFilesScreen) ApplyFilter() {
	query := strings.ToLower(strings.TrimSpace(s.FilterQuery))
	if query == "" {
		s.Files = s.AllFiles
	} else {
		s.Files = nil
		for _, f := range s.AllFiles {
			if strings.Contains(strings.ToLower(f.Filename), query) {
				s.Files = append(s.Files, f)
			}
		}
	}

	// Rebuild tree from filtered files
	s.Tree = BuildCommitFileTree(s.Files)
	SortCommitFileTree(s.Tree)
	CompressCommitFileTree(s.Tree)
	s.RebuildFlat()

	// Clamp cursor
	if s.Cursor >= len(s.TreeFlat) {
		s.Cursor = maxInt(0, len(s.TreeFlat)-1)
	}
	s.ScrollOffset = 0
}

// SearchNext finds the next match for the search query.
func (s *CommitFilesScreen) SearchNext(forward bool) {
	if s.SearchQuery == "" || len(s.TreeFlat) == 0 {
		return
	}

	query := strings.ToLower(s.SearchQuery)
	start := s.Cursor
	n := len(s.TreeFlat)

	for i := 1; i <= n; i++ {
		var idx int
		if forward {
			idx = (start + i) % n
		} else {
			idx = (start - i + n) % n
		}

		node := s.TreeFlat[idx]
		name := node.Path
		if parts := strings.Split(node.Path, "/"); len(parts) > 0 {
			name = parts[len(parts)-1]
		}

		if strings.Contains(strings.ToLower(name), query) {
			s.Cursor = idx
			// Adjust scroll offset
			maxVisible := s.Height - 8
			if s.Cursor < s.ScrollOffset {
				s.ScrollOffset = s.Cursor
			} else if s.Cursor >= s.ScrollOffset+maxVisible {
				s.ScrollOffset = s.Cursor - maxVisible + 1
			}
			return
		}
	}
}

func (s *CommitFilesScreen) flattenTree(node *CommitFileTreeNode, depth int) {
	if node == nil {
		return
	}

	for _, child := range node.Children {
		child.Depth = depth
		s.TreeFlat = append(s.TreeFlat, child)

		if child.IsDir() && !s.CollapsedDirs[child.Path] {
			s.flattenTree(child, depth+1)
		}
	}
}

// GetSelectedNode returns the currently selected node.
func (s *CommitFilesScreen) GetSelectedNode() *CommitFileTreeNode {
	if s.Cursor < 0 || s.Cursor >= len(s.TreeFlat) {
		return nil
	}
	return s.TreeFlat[s.Cursor]
}

// ToggleCollapse toggles the collapse state of a directory.
func (s *CommitFilesScreen) ToggleCollapse(path string) {
	s.CollapsedDirs[path] = !s.CollapsedDirs[path]
	s.RebuildFlat()
	if s.Cursor >= len(s.TreeFlat) {
		s.Cursor = maxInt(0, len(s.TreeFlat)-1)
	}
}

// Update handles key events for the commit files screen.
func (s *CommitFilesScreen) Update(msg tea.KeyMsg) (Screen, tea.Cmd) {
	maxVisible := s.Height - 8 // Account for header, footer, borders
	keyStr := msg.String()

	// Handle filter mode
	if s.ShowingFilter {
		switch keyStr {
		case keyEnter:
			s.ShowingFilter = false
			s.FilterInput.Blur()
			return s, nil
		case keyEsc, keyCtrlC:
			s.ShowingFilter = false
			s.FilterQuery = ""
			s.FilterInput.SetValue("")
			s.FilterInput.Blur()
			s.ApplyFilter()
			return s, nil
		case keyUp, keyCtrlK:
			if s.Cursor > 0 {
				s.Cursor--
				if s.Cursor < s.ScrollOffset {
					s.ScrollOffset = s.Cursor
				}
			}
			return s, nil
		case keyDown, keyCtrlJ:
			if s.Cursor < len(s.TreeFlat)-1 {
				s.Cursor++
				if s.Cursor >= s.ScrollOffset+maxVisible {
					s.ScrollOffset = s.Cursor - maxVisible + 1
				}
			}
			return s, nil
		}

		// Update filter input
		var cmd tea.Cmd
		s.FilterInput, cmd = s.FilterInput.Update(msg)
		newQuery := s.FilterInput.Value()
		if newQuery != s.FilterQuery {
			s.FilterQuery = newQuery
			s.ApplyFilter()
		}
		return s, cmd
	}

	// Handle search mode
	if s.ShowingSearch {
		switch keyStr {
		case keyEnter:
			s.ShowingSearch = false
			s.FilterInput.Blur()
			return s, nil
		case keyEsc, keyCtrlC:
			s.ShowingSearch = false
			s.SearchQuery = ""
			s.FilterInput.SetValue("")
			s.FilterInput.Blur()
			return s, nil
		case "n":
			s.SearchNext(true)
			return s, nil
		case "N":
			s.SearchNext(false)
			return s, nil
		}

		// Update search input and jump to first match
		var cmd tea.Cmd
		s.FilterInput, cmd = s.FilterInput.Update(msg)
		newQuery := s.FilterInput.Value()
		if newQuery != s.SearchQuery {
			s.SearchQuery = newQuery
			// Jump to first match from current position
			if s.SearchQuery != "" {
				s.SearchNext(true)
			}
		}
		return s, cmd
	}

	// Normal navigation and actions
	switch keyStr {
	case "q", keyCtrlC:
		if s.OnClose != nil {
			return nil, s.OnClose()
		}
		return nil, nil
	case keyEsc:
		if s.OnClose != nil {
			return nil, s.OnClose()
		}
		return nil, nil
	case "f":
		s.ShowingFilter = true
		s.ShowingSearch = false
		s.FilterInput.Placeholder = placeholderFilterFiles
		s.FilterInput.Focus()
		s.FilterInput.SetValue(s.FilterQuery)
		return s, textinput.Blink
	case "/":
		s.ShowingSearch = true
		s.ShowingFilter = false
		s.FilterInput.Placeholder = searchFiles
		s.FilterInput.Focus()
		s.FilterInput.SetValue(s.SearchQuery)
		return s, textinput.Blink
	case "d":
		if s.OnShowCommitDiff != nil {
			return nil, s.OnShowCommitDiff()
		}
		return nil, nil
	case keyEnter:
		node := s.GetSelectedNode()
		if node == nil {
			return s, nil
		}
		if node.IsDir() {
			s.ToggleCollapse(node.Path)
			return s, nil
		}
		// Show diff for this specific file
		if s.OnShowFileDiff != nil {
			return s, s.OnShowFileDiff(node.File.Filename)
		}
		return s, nil
	case "j", keyDown:
		if s.Cursor < len(s.TreeFlat)-1 {
			s.Cursor++
			if s.Cursor >= s.ScrollOffset+maxVisible {
				s.ScrollOffset = s.Cursor - maxVisible + 1
			}
		}
	case "k", keyUp:
		if s.Cursor > 0 {
			s.Cursor--
			if s.Cursor < s.ScrollOffset {
				s.ScrollOffset = s.Cursor
			}
		}
	case keyCtrlD, " ":
		s.Cursor = minInt(s.Cursor+maxVisible/2, len(s.TreeFlat)-1)
		if s.Cursor >= s.ScrollOffset+maxVisible {
			s.ScrollOffset = s.Cursor - maxVisible + 1
		}
	case keyCtrlU:
		s.Cursor = maxInt(s.Cursor-maxVisible/2, 0)
		if s.Cursor < s.ScrollOffset {
			s.ScrollOffset = s.Cursor
		}
	case "g":
		s.Cursor = 0
		s.ScrollOffset = 0
	case "G":
		s.Cursor = maxInt(0, len(s.TreeFlat)-1)
		if s.Cursor >= maxVisible {
			s.ScrollOffset = s.Cursor - maxVisible + 1
		}
	case "n":
		if s.SearchQuery != "" {
			s.SearchNext(true)
		}
	case "N":
		if s.SearchQuery != "" {
			s.SearchNext(false)
		}
	}

	return s, nil
}

// View renders the commit files screen.
func (s *CommitFilesScreen) View() string {
	// Calculate header height: title (1) + metadata (variable) + stats (1) + filter/search (1 if active) + footer (1) + borders (2)
	headerHeight := 5 // title + stats + footer + borders
	if s.Meta.SHA != "" || s.Meta.Author != "" || s.Meta.Date != "" || s.Meta.Subject != "" {
		// Estimate metadata height: commit line + author line + date line + blank + subject = ~5 lines
		metaHeight := 1 // commit line
		if s.Meta.Author != "" {
			metaHeight++
		}
		if s.Meta.Date != "" {
			metaHeight++
		}
		if s.Meta.Subject != "" {
			metaHeight += 2 // blank + subject
		}
		headerHeight += metaHeight
	}
	if s.ShowingFilter || s.ShowingSearch {
		headerHeight++
	}
	maxVisible := s.Height - headerHeight

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.Thm.Accent).
		Width(s.Width).
		Height(s.Height).
		Padding(0)

	titleStyle := lipgloss.NewStyle().
		Foreground(s.Thm.Accent).
		Bold(true).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(s.Thm.BorderDim).
		Width(s.Width-2).
		Padding(0, 1)

	shortSHA := s.CommitSHA
	if len(shortSHA) > 8 {
		shortSHA = shortSHA[:8]
	}
	title := titleStyle.Render(fmt.Sprintf("Files in commit %s", shortSHA))

	// Render commit metadata
	metaStyle := lipgloss.NewStyle().
		Foreground(s.Thm.MutedFg).
		Width(s.Width-2).
		Padding(0, 1).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(s.Thm.BorderDim)
	labelStyle := lipgloss.NewStyle().Foreground(s.Thm.MutedFg).Bold(true)
	valueStyle := lipgloss.NewStyle().Foreground(s.Thm.TextFg)
	subjectStyle := lipgloss.NewStyle().Bold(true).Foreground(s.Thm.Accent)

	var metaLines []string
	if s.Meta.SHA != "" {
		metaLines = append(metaLines, fmt.Sprintf("%s %s", labelStyle.Render("Commit:"), valueStyle.Render(s.Meta.SHA)))
	}
	if s.Meta.Author != "" {
		authorLine := fmt.Sprintf("%s %s", labelStyle.Render("Author:"), valueStyle.Render(s.Meta.Author))
		if s.Meta.Email != "" {
			authorLine += fmt.Sprintf(" <%s>", valueStyle.Render(s.Meta.Email))
		}
		metaLines = append(metaLines, authorLine)
	}
	if s.Meta.Date != "" {
		metaLines = append(metaLines, fmt.Sprintf("%s %s", labelStyle.Render("Date:"), valueStyle.Render(s.Meta.Date)))
	}
	if s.Meta.Subject != "" {
		if len(metaLines) > 0 {
			metaLines = append(metaLines, "")
		}
		metaLines = append(metaLines, subjectStyle.Render(s.Meta.Subject))
	}
	commitMetaSection := ""
	if len(metaLines) > 0 {
		commitMetaSection = metaStyle.Render(strings.Join(metaLines, "\n"))
	}

	// Render file tree
	var itemViews []string

	end := min(s.ScrollOffset+maxVisible, len(s.TreeFlat))
	start := min(s.ScrollOffset, end)

	itemStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(s.Width - 2)

	// Inline highlight style - no width, no padding
	highlightStyle := lipgloss.NewStyle().
		Background(s.Thm.Accent).
		Foreground(s.Thm.AccentFg).
		Bold(true)

	dirStyle := lipgloss.NewStyle().
		Foreground(s.Thm.Accent)

	fileStyle := lipgloss.NewStyle().
		Foreground(s.Thm.TextFg)

	changeTypeStyle := lipgloss.NewStyle().
		Foreground(s.Thm.MutedFg)

	noFilesStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(s.Width - 2).
		Foreground(s.Thm.MutedFg).
		Italic(true)

	for i := start; i < end; i++ {
		node := s.TreeFlat[i]
		indent := strings.Repeat("  ", node.Depth)
		isSelected := i == s.Cursor
		iconName := node.Path
		if parts := strings.Split(node.Path, "/"); len(parts) > 0 {
			iconName = parts[len(parts)-1]
		}
		devicon := ""
		if s.ShowIcons {
			devicon = iconWithSpace(getDevicon(iconName, node.IsDir()))
		}

		var label string
		if node.IsDir() {
			icon := disclosureIndicator(s.CollapsedDirs[node.Path], s.ShowIcons)
			// Show just the last part of the path for display
			displayPath := node.Path
			if parts := strings.Split(node.Path, "/"); len(parts) > 0 {
				displayPath = parts[len(parts)-1]
				if node.Compression > 0 && len(parts) > node.Compression {
					displayPath = strings.Join(parts[len(parts)-node.Compression-1:], "/")
				}
			}
			displayLabel := devicon + displayPath
			// Apply highlight only to directory name
			if isSelected {
				label = fmt.Sprintf("%s%s %s/", indent, icon, highlightStyle.Render(displayLabel))
			} else {
				label = fmt.Sprintf("%s%s %s/", indent, icon, dirStyle.Render(displayLabel))
			}
		} else {
			// Show just the filename
			displayName := node.Path
			if parts := strings.Split(node.Path, "/"); len(parts) > 0 {
				displayName = parts[len(parts)-1]
			}
			displayLabel := devicon + displayName
			changeIndicator := ""
			if node.File != nil {
				switch node.File.ChangeType {
				case "A":
					changeIndicator = changeTypeStyle.Render(" [+]")
				case "D":
					changeIndicator = changeTypeStyle.Render(" [-]")
				case "M":
					changeIndicator = changeTypeStyle.Render(" [~]")
				case "R":
					changeIndicator = changeTypeStyle.Render(" [R]")
				case "C":
					changeIndicator = changeTypeStyle.Render(" [C]")
				}
			}
			// Apply highlight only to filename
			if isSelected {
				label = fmt.Sprintf("%s  %s%s", indent, highlightStyle.Render(displayLabel), changeIndicator)
			} else {
				label = fmt.Sprintf("%s  %s%s", indent, fileStyle.Render(displayLabel), changeIndicator)
			}
		}

		itemViews = append(itemViews, itemStyle.Render(label))
	}

	if len(s.TreeFlat) == 0 {
		itemViews = append(itemViews, noFilesStyle.Render("No files in this commit."))
	}

	// Footer
	footerStyle := lipgloss.NewStyle().
		Foreground(s.Thm.MutedFg).
		Width(s.Width-2).
		Padding(0, 1).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(s.Thm.BorderDim)

	footerText := "j/k: navigate • Enter: toggle/view diff • d: full diff • f: filter • /: search • q: close"
	if s.ShowingFilter {
		footerText = fmt.Sprintf("%s: navigate • Enter: apply filter • Esc: clear filter", arrowPair(s.ShowIcons))
	} else if s.ShowingSearch {
		footerText = "n/N: next/prev match • Enter: close search • Esc: clear search"
	}
	footer := footerStyle.Render(footerText)

	// Stats line
	statsStyle := lipgloss.NewStyle().
		Foreground(s.Thm.MutedFg).
		Width(s.Width-2).
		Padding(0, 1).
		Align(lipgloss.Right)

	statsText := fmt.Sprintf("%d files", len(s.Files))
	if s.FilterQuery != "" {
		statsText = fmt.Sprintf("%d/%d files (filtered)", len(s.Files), len(s.AllFiles))
	}
	stats := statsStyle.Render(statsText)

	// Build content sections
	sections := []string{title}
	if commitMetaSection != "" {
		sections = append(sections, commitMetaSection)
	}

	// Add filter/search input if active
	if s.ShowingFilter || s.ShowingSearch {
		inputStyle := lipgloss.NewStyle().
			Padding(0, 1).
			Width(s.Width - 2).
			Foreground(s.Thm.TextFg)
		sections = append(sections, inputStyle.Render(s.FilterInput.View()))
	}

	sections = append(sections, stats, strings.Join(itemViews, "\n"), footer)

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	return boxStyle.Render(content)
}

// DeviconFunc is a function type for getting file icons.
type DeviconFunc func(name string, isDir bool) string

// IconProviderFunc allows injecting the devicon lookup function.
var IconProviderFunc DeviconFunc

// SetIconProviderFunc sets the function used to get file icons.
func SetIconProviderFunc(fn DeviconFunc) {
	IconProviderFunc = fn
}

// getDevicon returns the icon for a file or directory.
func getDevicon(name string, isDir bool) string {
	if IconProviderFunc != nil {
		return IconProviderFunc(name, isDir)
	}
	return ""
}
