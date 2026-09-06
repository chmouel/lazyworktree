package screen

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/theme"
)

// PRSelectionScreen lets the user pick a PR from a filtered list.
// It embeds a ListSelectionScreen and adds PR-specific rendering.
type PRSelectionScreen struct {
	*ListSelectionScreen

	// PR data
	PRs   []*models.PRInfo
	prMap map[string]*models.PRInfo

	// AttachedBranches maps branch names to worktree names for branches already checked out
	AttachedBranches map[string]string

	ShowIcons bool

	// OnSelectPR is called when a PR is selected (enter key)
	OnSelectPR func(*models.PRInfo) tea.Cmd
}

// NewPRSelectionScreen builds a PR selection screen with 80% of screen size.
func NewPRSelectionScreen(prs []*models.PRInfo, maxWidth, maxHeight int, thm *theme.Theme, showIcons bool) *PRSelectionScreen {
	items := make([]SelectionItem, len(prs))
	prMap := make(map[string]*models.PRInfo, len(prs))
	for i, pr := range prs {
		id := fmt.Sprintf("%d", pr.Number)
		items[i] = SelectionItem{
			ID:          id,
			Label:       pr.Title,
			Description: id,
		}
		prMap[id] = pr
	}

	list := NewListSelectionScreen(
		items,
		labelWithIcon(UIIconPRSelect, "Select PR/MR to Create Worktree", showIcons),
		"Filter PRs by number or title...",
		"No PRs match your filter.",
		maxWidth, maxHeight,
		"",
		thm,
	)
	list.EmptyMessage = "No open PRs/MRs found."
	list.FilterFields = func(item SelectionItem) []string {
		return []string{item.Description, item.Label, item.ID}
	}

	s := &PRSelectionScreen{
		ListSelectionScreen: list,
		PRs:                 prs,
		prMap:               prMap,
		ShowIcons:           showIcons,
	}

	list.RenderItem = s.renderItem
	list.OnSelect = func(item SelectionItem) tea.Cmd {
		if s.OnSelectPR != nil {
			if pr, ok := s.prMap[item.ID]; ok {
				return s.OnSelectPR(pr)
			}
		}
		return nil
	}

	return s
}

// Type returns the screen type.
func (s *PRSelectionScreen) Type() Type {
	return TypePRSelect
}

// Update handles updates for the PR selection screen.
func (s *PRSelectionScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	// PR screen accepts 'q' as quit in both filtered and unfiltered modes
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok && keyMsg.String() == keyQ {
		if s.OnCancel != nil {
			return nil, s.OnCancel()
		}
		return nil, nil
	}
	result, cmd := s.ListSelectionScreen.Update(msg)
	if result == nil {
		return nil, cmd
	}
	return s, cmd
}

// SelectedPR returns the currently selected PR, if any.
func (s *PRSelectionScreen) SelectedPR() (*models.PRInfo, bool) {
	item, ok := s.Selected()
	if !ok {
		return nil, false
	}
	pr, exists := s.prMap[item.ID]
	return pr, exists
}

// FilteredPRs returns the currently filtered PR list.
func (s *PRSelectionScreen) FilteredPRs() []*models.PRInfo {
	result := make([]*models.PRInfo, 0, len(s.Filtered))
	for _, item := range s.Filtered {
		if pr, ok := s.prMap[item.ID]; ok {
			result = append(result, pr)
		}
	}
	return result
}

// isAttached checks if a PR's branch is already checked out in a worktree.
func (s *PRSelectionScreen) isAttached(pr *models.PRInfo) (string, bool) {
	if s.AttachedBranches == nil {
		return "", false
	}
	wtName, ok := s.AttachedBranches[pr.Branch]
	return wtName, ok
}

// renderItem renders a single PR item line with CI bubbles, author, and attached-branch dimming.
func (s *PRSelectionScreen) renderItem(item SelectionItem, _ int, isCursor bool) string {
	pr := s.prMap[item.ID]

	itemStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(s.Width - 2)

	selectedStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(s.Width - 2).
		Background(s.Thm.Accent).
		Foreground(s.Thm.AccentFg).
		Bold(true)

	disabledStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(s.Width - 2).
		Foreground(s.Thm.MutedFg).
		Italic(true)

	disabledSelectedStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(s.Width - 2).
		Background(s.Thm.BorderDim).
		Foreground(s.Thm.MutedFg).
		Italic(true)

	wtName, isAttached := s.isAttached(pr)

	// Column widths
	prNumWidth := 6
	authorWidth := min(12, max(8, (s.Width-30)/5))
	ciWidth := 6
	iconWidth := 0
	if s.ShowIcons {
		iconWidth = 3
	}
	titleWidth := s.Width - prNumWidth - authorWidth - ciWidth - iconWidth - 10

	// Format PR number
	prNum := fmt.Sprintf("#%-5d", pr.Number)

	// Format author
	author := pr.Author
	if len(author) > authorWidth {
		author = author[:authorWidth-1] + "…"
	}
	authorFmt := fmt.Sprintf("%-*s", authorWidth, author)

	// Format CI status icon
	ciIcon := getCIStatusIcon(pr.CIStatus, pr.IsDraft, s.ShowIcons)

	// Format title with attached suffix
	title := pr.Title
	suffix := ""
	availableTitleWidth := titleWidth
	if isAttached {
		suffix = fmt.Sprintf(" (in: %s)", wtName)
		availableTitleWidth = titleWidth - len(suffix)
		if availableTitleWidth < 10 {
			availableTitleWidth = 10
		}
	}
	if len(title) > availableTitleWidth {
		title = title[:availableTitleWidth-1] + "…"
	}
	if isAttached {
		title += suffix
	}

	// Build the label
	iconPrefix := ""
	if s.ShowIcons {
		iconPrefix = iconWithSpace(getIconPR())
	}
	prLabel := fmt.Sprintf("%s%s %s %s %s", iconPrefix, prNum, authorFmt, ciIcon, title)

	if isCursor {
		if isAttached {
			return disabledSelectedStyle.Render(prLabel)
		}
		return selectedStyle.Render(prLabel)
	}
	if isAttached {
		return disabledStyle.Render(prLabel)
	}
	return s.renderPRLine(itemStyle, iconPrefix, prNum, authorFmt, ciIcon, title, pr.CIStatus, pr.IsDraft)
}

// renderPRLine renders a PR line with bubble-styled CI status icon.
func (s *PRSelectionScreen) renderPRLine(baseStyle lipgloss.Style, iconPrefix, prNum, author, _, title, ciStatus string, isDraft bool) string {
	var ciBubble string
	if isDraft {
		ciBubble = lipgloss.NewStyle().Foreground(s.Thm.MutedFg).Render("D")
	} else {
		ciBubble = renderCIBubble(s.Thm, ciStatus, s.ShowIcons)
	}

	line := fmt.Sprintf("%s%s %s %s %s", iconPrefix, prNum, author, ciBubble, title)
	return baseStyle.Render(line)
}
