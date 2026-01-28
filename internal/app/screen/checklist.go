package screen

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/chmouel/lazyworktree/internal/theme"
)

// ChecklistItem represents a single item with a checkbox state.
type ChecklistItem struct {
	ID          string
	Label       string
	Description string
	Checked     bool
}

// ChecklistScreen lets the user select multiple items from a list via checkboxes.
type ChecklistScreen struct {
	// Data fields
	Items    []ChecklistItem
	Filtered []ChecklistItem

	// UI state
	FilterInput  textinput.Model
	Cursor       int
	ScrollOffset int
	Width        int
	Height       int
	Title        string
	Placeholder  string
	NoResults    string
	Thm          *theme.Theme

	// Callbacks
	OnSubmit func([]ChecklistItem) tea.Cmd
	OnCancel func() tea.Cmd
}

// NewChecklistScreen builds a checklist screen with 80% of screen size.
func NewChecklistScreen(items []ChecklistItem, title, placeholder, noResults string, maxWidth, maxHeight int, thm *theme.Theme) *ChecklistScreen {
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

	if placeholder == "" {
		placeholder = "Filter..."
	}
	if noResults == "" {
		noResults = "No items found."
	}

	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.CharLimit = 100
	ti.Prompt = "> "
	ti.Focus()
	ti.Width = width - 4 // padding

	cursor := 0
	if len(items) == 0 {
		cursor = -1
	}

	return &ChecklistScreen{
		Items:        items,
		Filtered:     items,
		FilterInput:  ti,
		Cursor:       cursor,
		ScrollOffset: 0,
		Width:        width,
		Height:       height,
		Title:        title,
		Placeholder:  placeholder,
		NoResults:    noResults,
		Thm:          thm,
	}
}

// Type returns the screen type.
func (s *ChecklistScreen) Type() Type {
	return TypeChecklist
}

// Update handles keyboard input for the checklist screen.
// Returns nil to signal the screen should close.
func (s *ChecklistScreen) Update(msg tea.KeyMsg) (Screen, tea.Cmd) {
	var cmd tea.Cmd
	maxVisible := (s.Height - 6) / 2 // Account for header, input, footer; divide by 2 since each item takes 2 lines

	keyStr := msg.String()
	switch keyStr {
	case keyEnter:
		if s.OnSubmit != nil {
			selected := s.SelectedItems()
			return nil, s.OnSubmit(selected)
		}
		return nil, nil
	case keyEsc, keyCtrlC:
		// Clear all selections on cancel
		for i := range s.Items {
			s.Items[i].Checked = false
		}
		s.applyFilter()
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
	case " ":
		// Toggle current item
		if s.Cursor >= 0 && s.Cursor < len(s.Filtered) {
			// Find the item in the original list and toggle it
			id := s.Filtered[s.Cursor].ID
			for i := range s.Items {
				if s.Items[i].ID == id {
					s.Items[i].Checked = !s.Items[i].Checked
					break
				}
			}
			s.applyFilter()
		}
		return s, nil
	case "a":
		// Select all filtered items
		for _, f := range s.Filtered {
			for i := range s.Items {
				if s.Items[i].ID == f.ID {
					s.Items[i].Checked = true
					break
				}
			}
		}
		s.applyFilter()
		return s, nil
	case "n":
		// Deselect all filtered items
		for _, f := range s.Filtered {
			for i := range s.Items {
				if s.Items[i].ID == f.ID {
					s.Items[i].Checked = false
					break
				}
			}
		}
		s.applyFilter()
		return s, nil
	}

	s.FilterInput, cmd = s.FilterInput.Update(msg)
	s.applyFilter()
	return s, cmd
}

func (s *ChecklistScreen) applyFilter() {
	query := strings.ToLower(strings.TrimSpace(s.FilterInput.Value()))
	if query == "" {
		// Rebuild filtered from items to reflect checkbox changes
		s.Filtered = make([]ChecklistItem, len(s.Items))
		copy(s.Filtered, s.Items)
	} else {
		s.Filtered = []ChecklistItem{}
		for _, item := range s.Items {
			labelLower := strings.ToLower(item.Label)
			descLower := strings.ToLower(item.Description)
			idLower := strings.ToLower(item.ID)
			if strings.Contains(labelLower, query) || strings.Contains(descLower, query) || strings.Contains(idLower, query) {
				s.Filtered = append(s.Filtered, item)
			}
		}
	}

	// Reset cursor if needed
	if len(s.Filtered) == 0 {
		s.Cursor = -1
	} else if s.Cursor >= len(s.Filtered) || s.Cursor < 0 {
		s.Cursor = 0
	}
	s.ScrollOffset = 0
}

// SelectedItems returns all checked items.
func (s *ChecklistScreen) SelectedItems() []ChecklistItem {
	var selected []ChecklistItem
	for _, item := range s.Items {
		if item.Checked {
			selected = append(selected, item)
		}
	}
	return selected
}

// View renders the checklist screen.
func (s *ChecklistScreen) View() string {
	maxVisible := (s.Height - 6) / 2 // Account for header, input, footer; divide by 2 since each item takes 2 lines

	// Enhanced checklist modal with rounded border
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
		Render(s.Title)

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

	descStyle := lipgloss.NewStyle().
		Foreground(s.Thm.MutedFg)

	selectedDescStyle := lipgloss.NewStyle().
		Foreground(s.Thm.TextFg)

	noResultsStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(s.Width - 2).
		Foreground(s.Thm.MutedFg).
		Italic(true)

	// Render Input
	inputView := inputStyle.Render(s.FilterInput.View())

	// Render items
	var itemViews []string

	end := min(s.ScrollOffset+maxVisible, len(s.Filtered))
	start := s.ScrollOffset
	if start > end {
		start = end
	}

	for i := start; i < end; i++ {
		item := s.Filtered[i]

		// Checkbox prefix
		checkbox := "[ ] "
		if item.Checked {
			checkbox = "[x] "
		}

		// First line: checkbox + label
		label := checkbox + item.Label
		var line string
		if i == s.Cursor {
			line = selectedStyle.Render(label)
		} else {
			line = itemStyle.Render(label)
		}
		itemViews = append(itemViews, line)

		// Second line: description (indented)
		if item.Description != "" {
			indent := "    " // 4 spaces to align with label after checkbox
			desc := indent + item.Description
			var descLine string
			if i == s.Cursor {
				descLine = selectedStyle.Render(selectedDescStyle.Render(desc))
			} else {
				descLine = itemStyle.Render(descStyle.Render(desc))
			}
			itemViews = append(itemViews, descLine)
		}
	}

	if len(s.Filtered) == 0 {
		itemViews = append(itemViews, noResultsStyle.Render(s.NoResults))
	}

	// Separator
	separator := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(s.Thm.BorderDim).
		Width(s.Width - 2).
		Render("")

	// Count selected
	selectedCount := 0
	for _, item := range s.Items {
		if item.Checked {
			selectedCount++
		}
	}

	// Footer
	footerStyle := lipgloss.NewStyle().
		Foreground(s.Thm.MutedFg).
		Align(lipgloss.Right).
		Width(s.Width - 2).
		PaddingTop(1)
	footer := footerStyle.Render(fmt.Sprintf("%d selected • Space toggle • a/n all/none • Enter confirm • Esc cancel", selectedCount))

	content := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle,
		inputView,
		separator,
		strings.Join(itemViews, "\n"),
		footer,
	)

	return boxStyle.Render(content)
}
