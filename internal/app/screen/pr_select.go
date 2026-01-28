package screen

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/theme"
)

// PRSelectionScreen lets the user pick a PR from a filtered list.
type PRSelectionScreen struct {
	// Data fields
	PRs      []*models.PRInfo
	Filtered []*models.PRInfo

	// UI state
	FilterInput  textinput.Model
	Cursor       int
	ScrollOffset int
	Width        int
	Height       int
	Thm          *theme.Theme
	ShowIcons    bool

	// Callbacks
	OnSelect func(*models.PRInfo) tea.Cmd
	OnCancel func() tea.Cmd
}

// NewPRSelectionScreen builds a PR selection screen with 80% of screen size.
func NewPRSelectionScreen(prs []*models.PRInfo, maxWidth, maxHeight int, thm *theme.Theme, showIcons bool) *PRSelectionScreen {
	// Use 80% of screen size
	width := int(float64(maxWidth) * 0.8)
	height := int(float64(maxHeight) * 0.8)

	// Ensure minimum sizes
	if width < 60 {
		width = 60
	}
	if height < 20 {
		height = 20
	}

	ti := textinput.New()
	ti.Placeholder = "Filter PRs by number or title..."
	ti.CharLimit = 100
	ti.Prompt = "> "
	ti.Focus()
	ti.Width = width - 4 // padding

	return &PRSelectionScreen{
		PRs:          prs,
		Filtered:     prs,
		FilterInput:  ti,
		Cursor:       0,
		ScrollOffset: 0,
		Width:        width,
		Height:       height,
		Thm:          thm,
		ShowIcons:    showIcons,
	}
}

// Type returns the screen type.
func (s *PRSelectionScreen) Type() Type {
	return TypePRSelect
}

// Update handles updates for the PR selection screen.
// Returns nil to signal the screen should close.
func (s *PRSelectionScreen) Update(msg tea.KeyMsg) (Screen, tea.Cmd) {
	var cmd tea.Cmd
	maxVisible := s.Height - 6 // Account for header, input, footer

	keyStr := msg.String()
	switch keyStr {
	case keyEnter:
		if s.OnSelect != nil {
			if pr, ok := s.Selected(); ok {
				return nil, s.OnSelect(pr)
			}
		}
		return nil, nil
	case keyEsc, keyQ, keyCtrlC:
		if s.OnCancel != nil {
			return nil, s.OnCancel()
		}
		return nil, nil
	case "up", "k", "ctrl+k":
		if s.Cursor > 0 {
			s.Cursor--
			if s.Cursor < s.ScrollOffset {
				s.ScrollOffset = s.Cursor
			}
		}
		return s, nil
	case "down", "j", "ctrl+j":
		if s.Cursor < len(s.Filtered)-1 {
			s.Cursor++
			if s.Cursor >= s.ScrollOffset+maxVisible {
				s.ScrollOffset = s.Cursor - maxVisible + 1
			}
		}
		return s, nil
	}

	s.FilterInput, cmd = s.FilterInput.Update(msg)
	s.applyFilter()
	return s, cmd
}

// View renders the PR selection screen.
func (s *PRSelectionScreen) View() string {
	maxVisible := s.Height - 6 // Account for header, input, footer

	// Enhanced PR selection modal with rounded border
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.Thm.Accent).
		Width(s.Width).
		Padding(0)

	titleStyle := lipgloss.NewStyle().
		Foreground(s.Thm.Accent).
		Bold(true).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(s.Thm.BorderDim).
		Width(s.Width-2).
		Padding(0, 1).
		Render(labelWithIcon(UIIconPRSelect, "Select PR/MR to Create Worktree", s.ShowIcons))

	inputStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(s.Width - 2).
		Foreground(s.Thm.TextFg)

	itemStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(s.Width - 2)

	selectedStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(s.Width - 2).
		Background(s.Thm.Accent).
		Foreground(s.Thm.AccentFg).
		Bold(true)

	noResultsStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(s.Width - 2).
		Foreground(s.Thm.MutedFg).
		Italic(true)

	// Render Input
	inputView := inputStyle.Render(s.FilterInput.View())

	// Render PRs
	var itemViews []string

	end := s.ScrollOffset + maxVisible
	if end > len(s.Filtered) {
		end = len(s.Filtered)
	}
	start := s.ScrollOffset
	if start > end {
		start = end
	}

	// Calculate column widths for display
	// Layout: [icon] #number author CI title
	prNumWidth := 6
	authorWidth := min(12, max(8, (s.Width-30)/5))
	ciWidth := 2
	iconWidth := 0
	if s.ShowIcons {
		iconWidth = 3
	}
	// Title gets remaining space
	titleWidth := s.Width - prNumWidth - authorWidth - ciWidth - iconWidth - 10

	for i := start; i < end; i++ {
		pr := s.Filtered[i]

		// Format PR number
		prNum := fmt.Sprintf("#%-5d", pr.Number)

		// Format author (truncate if needed)
		author := pr.Author
		if len(author) > authorWidth {
			author = author[:authorWidth-1] + "…"
		}
		authorFmt := fmt.Sprintf("%-*s", authorWidth, author)

		// Format CI status icon (draft takes precedence)
		ciIcon := getCIStatusIcon(pr.CIStatus, pr.IsDraft, s.ShowIcons)

		// Format title (truncate if needed)
		title := pr.Title
		if len(title) > titleWidth {
			title = title[:titleWidth-1] + "…"
		}

		// Build the label
		iconPrefix := ""
		if s.ShowIcons {
			iconPrefix = iconWithSpace(getIconPR())
		}
		prLabel := fmt.Sprintf("%s%s %s %s %s", iconPrefix, prNum, authorFmt, ciIcon, title)

		var line string
		if i == s.Cursor {
			line = selectedStyle.Render(prLabel)
		} else {
			// Apply color to CI icon based on status
			line = s.renderPRLine(itemStyle, iconPrefix, prNum, authorFmt, ciIcon, title, pr.CIStatus, pr.IsDraft)
		}
		itemViews = append(itemViews, line)
	}

	if len(s.Filtered) == 0 {
		if len(s.PRs) == 0 {
			itemViews = append(itemViews, noResultsStyle.Render("No open PRs/MRs found."))
		} else {
			itemViews = append(itemViews, noResultsStyle.Render("No PRs match your filter."))
		}
	}

	// Separator
	separator := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(s.Thm.BorderDim).
		Width(s.Width - 2).
		Render("")

	// Footer
	footerStyle := lipgloss.NewStyle().
		Foreground(s.Thm.MutedFg).
		Align(lipgloss.Right).
		Width(s.Width - 2).
		PaddingTop(1)
	footer := footerStyle.Render("Enter to select • Esc to cancel")

	content := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle,
		inputView,
		separator,
		strings.Join(itemViews, "\n"),
		footer,
	)

	return boxStyle.Render(content)
}

// renderPRLine renders a PR line with colored CI status icon.
func (s *PRSelectionScreen) renderPRLine(baseStyle lipgloss.Style, iconPrefix, prNum, author, ciIcon, title, ciStatus string, isDraft bool) string {
	// Style for CI icon based on status
	var ciStyle lipgloss.Style
	if isDraft {
		ciStyle = lipgloss.NewStyle().Foreground(s.Thm.MutedFg)
	} else {
		switch ciStatus {
		case "success":
			ciStyle = lipgloss.NewStyle().Foreground(s.Thm.SuccessFg)
		case "failure":
			ciStyle = lipgloss.NewStyle().Foreground(s.Thm.ErrorFg)
		case "pending":
			ciStyle = lipgloss.NewStyle().Foreground(s.Thm.WarnFg)
		default:
			ciStyle = lipgloss.NewStyle().Foreground(s.Thm.MutedFg)
		}
	}

	// Build the line with colored CI icon
	line := fmt.Sprintf("%s%s %s %s %s", iconPrefix, prNum, author, ciStyle.Render(ciIcon), title)
	return baseStyle.Render(line)
}

// applyFilter filters the PR list based on the current filter input.
func (s *PRSelectionScreen) applyFilter() {
	query := strings.ToLower(strings.TrimSpace(s.FilterInput.Value()))
	if query == "" {
		s.Filtered = s.PRs
	} else {
		s.Filtered = []*models.PRInfo{}
		for _, pr := range s.PRs {
			// Match by number or title
			prNumStr := fmt.Sprintf("%d", pr.Number)
			titleLower := strings.ToLower(pr.Title)
			if strings.Contains(prNumStr, query) || strings.Contains(titleLower, query) {
				s.Filtered = append(s.Filtered, pr)
			}
		}
	}

	// Reset cursor if needed
	if s.Cursor >= len(s.Filtered) {
		s.Cursor = max(0, len(s.Filtered)-1)
	}
	if s.Cursor < 0 && len(s.Filtered) > 0 {
		s.Cursor = 0
	}
	s.ScrollOffset = 0
}

// Selected returns the currently selected PR, if any.
func (s *PRSelectionScreen) Selected() (*models.PRInfo, bool) {
	if s.Cursor < 0 || s.Cursor >= len(s.Filtered) {
		return nil, false
	}
	return s.Filtered[s.Cursor], true
}
