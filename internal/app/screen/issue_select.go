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

// IssueSelectionScreen lets the user pick an issue from a filtered list.
type IssueSelectionScreen struct {
	Issues       []*models.IssueInfo
	Filtered     []*models.IssueInfo
	FilterInput  textinput.Model
	Cursor       int
	ScrollOffset int
	Width        int
	Height       int
	Thm          *theme.Theme
	ShowIcons    bool

	OnSelect func(*models.IssueInfo) tea.Cmd
	OnCancel func() tea.Cmd
}

// NewIssueSelectionScreen builds an issue selection screen with 80% of screen size.
func NewIssueSelectionScreen(issues []*models.IssueInfo, maxWidth, maxHeight int, thm *theme.Theme, showIcons bool) *IssueSelectionScreen {
	width := int(float64(maxWidth) * 0.8)
	height := int(float64(maxHeight) * 0.8)

	if width < 60 {
		width = 60
	}
	if height < 20 {
		height = 20
	}

	ti := textinput.New()
	ti.Placeholder = "Filter issues by number or title..."
	ti.CharLimit = 100
	ti.Prompt = "> "
	ti.Focus()
	ti.Width = width - 4

	return &IssueSelectionScreen{
		Issues:       issues,
		Filtered:     issues,
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
func (s *IssueSelectionScreen) Type() Type {
	return TypeIssueSelect
}

// Update handles keyboard input and returns nil to signal the screen should close.
func (s *IssueSelectionScreen) Update(msg tea.KeyMsg) (Screen, tea.Cmd) {
	var cmd tea.Cmd
	maxVisible := s.Height - 6

	keyStr := msg.String()
	switch keyStr {
	case keyEnter:
		if s.OnSelect != nil {
			if issue, ok := s.Selected(); ok {
				return nil, s.OnSelect(issue)
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

// View renders the issue selection screen.
func (s *IssueSelectionScreen) View() string {
	maxVisible := s.Height - 6

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
		Render(labelWithIcon(UIIconIssueSelect, "Select Issue to Create Worktree", s.ShowIcons))

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

	inputView := inputStyle.Render(s.FilterInput.View())

	var itemViews []string

	end := s.ScrollOffset + maxVisible
	if end > len(s.Filtered) {
		end = len(s.Filtered)
	}
	start := s.ScrollOffset
	if start > end {
		start = end
	}

	issueNumWidth := 6
	iconWidth := 0
	if s.ShowIcons {
		iconWidth = 3
	}
	titleWidth := s.Width - issueNumWidth - iconWidth - 10

	for i := start; i < end; i++ {
		issue := s.Filtered[i]

		issueNum := fmt.Sprintf("#%-5d", issue.Number)

		title := issue.Title
		if len(title) > titleWidth {
			title = title[:titleWidth-1] + "…"
		}

		iconPrefix := ""
		if s.ShowIcons {
			iconPrefix = iconWithSpace(getIconIssue())
		}
		issueLabel := fmt.Sprintf("%s%s %s", iconPrefix, issueNum, title)

		var line string
		if i == s.Cursor {
			line = selectedStyle.Render(issueLabel)
		} else {
			line = itemStyle.Render(issueLabel)
		}
		itemViews = append(itemViews, line)
	}

	if len(s.Filtered) == 0 {
		if len(s.Issues) == 0 {
			itemViews = append(itemViews, noResultsStyle.Render("No open issues found."))
		} else {
			itemViews = append(itemViews, noResultsStyle.Render("No issues match your filter."))
		}
	}

	separator := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(s.Thm.BorderDim).
		Width(s.Width - 2).
		Render("")

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

func (s *IssueSelectionScreen) applyFilter() {
	query := strings.ToLower(strings.TrimSpace(s.FilterInput.Value()))
	if query == "" {
		s.Filtered = s.Issues
	} else {
		s.Filtered = []*models.IssueInfo{}
		for _, issue := range s.Issues {
			issueNumStr := fmt.Sprintf("%d", issue.Number)
			titleLower := strings.ToLower(issue.Title)
			if strings.Contains(issueNumStr, query) || strings.Contains(titleLower, query) {
				s.Filtered = append(s.Filtered, issue)
			}
		}
	}

	if s.Cursor >= len(s.Filtered) {
		s.Cursor = max(0, len(s.Filtered)-1)
	}
	if s.Cursor < 0 && len(s.Filtered) > 0 {
		s.Cursor = 0
	}
	s.ScrollOffset = 0
}

// Selected returns the currently selected issue, if any.
func (s *IssueSelectionScreen) Selected() (*models.IssueInfo, bool) {
	if s.Cursor < 0 || s.Cursor >= len(s.Filtered) {
		return nil, false
	}
	return s.Filtered[s.Cursor], true
}
