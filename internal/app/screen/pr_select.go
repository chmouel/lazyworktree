package screen

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
	FilterActive bool
	Cursor       int
	ScrollOffset int
	Width        int
	Height       int
	Thm          *theme.Theme
	ShowIcons    bool

	// AttachedBranches maps branch names to worktree names for branches already checked out
	AttachedBranches map[string]string

	// StatusMessage shows temporary feedback (e.g., when trying to select attached PR)
	StatusMessage string

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
	ti.Blur()
	ti.SetWidth(width - 4) // padding

	return &PRSelectionScreen{
		PRs:          prs,
		Filtered:     prs,
		FilterInput:  ti,
		FilterActive: false,
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
func (s *PRSelectionScreen) Update(msg tea.KeyPressMsg) (Screen, tea.Cmd) {
	var cmd tea.Cmd
	maxVisible := s.Height - 6 // Account for header, input, footer
	if !s.FilterActive {
		maxVisible += 2
	}

	keyStr := msg.String()
	if !s.FilterActive {
		switch keyStr {
		case "f":
			s.FilterActive = true
			s.FilterInput.Focus()
			return s, textinput.Blink
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
			s.StatusMessage = "" // Clear status on navigation
			if s.Cursor > 0 {
				s.Cursor--
				if s.Cursor < s.ScrollOffset {
					s.ScrollOffset = s.Cursor
				}
			}
			return s, nil
		case "down", "j", "ctrl+j":
			s.StatusMessage = "" // Clear status on navigation
			if s.Cursor < len(s.Filtered)-1 {
				s.Cursor++
				if s.Cursor >= s.ScrollOffset+maxVisible {
					s.ScrollOffset = s.Cursor - maxVisible + 1
				}
			}
			return s, nil
		}
		return s, nil
	}

	switch keyStr {
	case keyEsc:
		s.FilterActive = false
		s.FilterInput.Blur()
		return s, nil
	case keyEnter:
		if s.OnSelect != nil {
			if pr, ok := s.Selected(); ok {
				return nil, s.OnSelect(pr)
			}
		}
		return nil, nil
	case keyQ, keyCtrlC:
		if s.OnCancel != nil {
			return nil, s.OnCancel()
		}
		return nil, nil
	case "up", "ctrl+k":
		s.StatusMessage = "" // Clear status on navigation
		if s.Cursor > 0 {
			s.Cursor--
			if s.Cursor < s.ScrollOffset {
				s.ScrollOffset = s.Cursor
			}
		}
		return s, nil
	case "down", "ctrl+j":
		s.StatusMessage = "" // Clear status on navigation
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
	if !s.FilterActive {
		maxVisible += 2
	}

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
	ciWidth := 6
	iconWidth := 0
	if s.ShowIcons {
		iconWidth = 3
	}
	// Title gets remaining space
	titleWidth := s.Width - prNumWidth - authorWidth - ciWidth - iconWidth - 10

	for i := start; i < end; i++ {
		pr := s.Filtered[i]

		// Check if this PR's branch is already attached to a worktree
		wtName, isAttached := s.isAttached(pr)

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

		// Format title (truncate if needed, with suffix for attached PRs)
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

		var line string
		if i == s.Cursor {
			if isAttached {
				line = disabledSelectedStyle.Render(prLabel)
			} else {
				line = selectedStyle.Render(prLabel)
			}
		} else {
			if isAttached {
				line = disabledStyle.Render(prLabel)
			} else {
				// Apply color to CI icon based on status
				line = s.renderPRLine(itemStyle, iconPrefix, prNum, authorFmt, ciIcon, title, pr.CIStatus, pr.IsDraft)
			}
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

	// Footer with optional status message
	footerStyle := lipgloss.NewStyle().
		Foreground(s.Thm.MutedFg).
		Align(lipgloss.Right).
		Width(s.Width - 2).
		PaddingTop(1)
	footerText := "j/k to move • f to filter • Enter to select • Esc to cancel"
	if s.FilterActive {
		footerText = "Esc to return • Enter to select"
	}
	if s.StatusMessage != "" {
		statusStyle := lipgloss.NewStyle().Foreground(s.Thm.WarnFg)
		footerText = statusStyle.Render(s.StatusMessage) + "  •  " + footerText
	}
	footer := footerStyle.Render(footerText)

	contentLines := []string{titleStyle}
	if s.FilterActive {
		inputView := inputStyle.Render(s.FilterInput.View())
		contentLines = append(contentLines, inputView, separator)
	}
	contentLines = append(contentLines, strings.Join(itemViews, "\n"), footer)
	content := lipgloss.JoinVertical(lipgloss.Left, contentLines...)

	return boxStyle.Render(content)
}

// renderPRLine renders a PR line with bubble-styled CI status icon.
func (s *PRSelectionScreen) renderPRLine(baseStyle lipgloss.Style, iconPrefix, prNum, author, _, title, ciStatus string, isDraft bool) string {
	// For draft PRs, show a muted "D" without a bubble
	var ciBubble string
	if isDraft {
		ciBubble = lipgloss.NewStyle().Foreground(s.Thm.MutedFg).Render("D")
	} else {
		ciBubble = renderCIBubble(s.Thm, ciStatus, s.ShowIcons)
	}

	line := fmt.Sprintf("%s%s %s %s %s", iconPrefix, prNum, author, ciBubble, title)
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

// isAttached checks if a PR's branch is already checked out in a worktree.
// Returns the worktree name and true if attached, empty string and false otherwise.
func (s *PRSelectionScreen) isAttached(pr *models.PRInfo) (string, bool) {
	if s.AttachedBranches == nil {
		return "", false
	}
	wtName, ok := s.AttachedBranches[pr.Branch]
	return wtName, ok
}
