package screen

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/chmouel/lazyworktree/internal/theme"
)

// SelectionItem represents a single item in a list selection.
type SelectionItem struct {
	ID          string
	Label       string
	Description string
}

// ListSelectionScreen lets the user pick from a list of options.
type ListSelectionScreen struct {
	// Data fields
	Items    []SelectionItem
	Filtered []SelectionItem

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
	OnSelect       func(SelectionItem) tea.Cmd
	OnCancel       func() tea.Cmd
	OnCursorChange func(SelectionItem) // For live preview

	// Special key handlers (for CI checks)
	OnCtrlV func(SelectionItem) tea.Cmd // Ctrl+V handler (e.g., view logs)
	OnCtrlR func(SelectionItem) tea.Cmd // Ctrl+R handler (e.g., restart)
	OnEnter func(SelectionItem) tea.Cmd // Enter handler (overrides OnSelect if set)

	// Optional additional hint for footer (e.g., "Ctrl+r to restart")
	FooterHint string
}

// NewListSelectionScreen builds a list selection screen with 80% of screen size.
func NewListSelectionScreen(items []SelectionItem, title, placeholder, noResults string, maxWidth, maxHeight int, initialID string, thm *theme.Theme) *ListSelectionScreen {
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
		noResults = "No results found."
	}

	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.CharLimit = 100
	ti.Prompt = "> "
	ti.Focus()
	ti.Width = width - 4

	cursor := 0
	if len(items) == 0 {
		cursor = -1
	}
	if initialID != "" {
		for i, item := range items {
			if item.ID == initialID {
				cursor = i
				break
			}
		}
	}

	scr := &ListSelectionScreen{
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
	return scr
}

// Type returns the screen type.
func (s *ListSelectionScreen) Type() Type {
	return TypeListSelect
}

// Update handles keyboard input and returns nil to signal the screen should close.
func (s *ListSelectionScreen) Update(msg tea.KeyMsg) (Screen, tea.Cmd) {
	var cmd tea.Cmd
	maxVisible := s.Height - 6

	switch msg.String() {
	case "ctrl+v":
		if s.OnCtrlV != nil {
			if item, ok := s.Selected(); ok {
				return s, s.OnCtrlV(item)
			}
		}
		return s, nil
	case "ctrl+r":
		if s.OnCtrlR != nil {
			if item, ok := s.Selected(); ok {
				return s, s.OnCtrlR(item)
			}
		}
		return s, nil
	case "enter":
		if s.OnEnter != nil {
			if item, ok := s.Selected(); ok {
				return s, s.OnEnter(item)
			}
			return s, nil
		}
		if s.OnSelect != nil {
			if item, ok := s.Selected(); ok {
				return nil, s.OnSelect(item)
			}
		}
		return nil, nil
	case "esc", "ctrl+c":
		if s.OnCancel != nil {
			return nil, s.OnCancel()
		}
		return nil, nil
	case "up", "ctrl+k":
		if s.Cursor > 0 {
			s.Cursor--
			if s.Cursor < s.ScrollOffset {
				s.ScrollOffset = s.Cursor
			}
			if s.OnCursorChange != nil {
				if item, ok := s.Selected(); ok {
					s.OnCursorChange(item)
				}
			}
		}
		return s, nil
	case "down", "ctrl+j":
		if s.Cursor < len(s.Filtered)-1 {
			s.Cursor++
			if s.Cursor >= s.ScrollOffset+maxVisible {
				s.ScrollOffset = s.Cursor - maxVisible + 1
			}
			if s.OnCursorChange != nil {
				if item, ok := s.Selected(); ok {
					s.OnCursorChange(item)
				}
			}
		}
		return s, nil
	}

	s.FilterInput, cmd = s.FilterInput.Update(msg)
	s.applyFilter()
	return s, cmd
}

// View renders the list selection screen.
func (s *ListSelectionScreen) View() string {
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

	for i := start; i < end; i++ {
		item := s.Filtered[i]
		label := item.Label
		if item.Description != "" {
			desc := item.Description
			if i == s.Cursor {
				desc = selectedDescStyle.Render(desc)
			} else {
				desc = descStyle.Render(desc)
			}
			label = fmt.Sprintf("%s  %s", label, desc)
		}

		var line string
		if i == s.Cursor {
			line = selectedStyle.Render(ansi.Strip(label))
		} else {
			line = itemStyle.Render(label)
		}
		itemViews = append(itemViews, line)
	}

	if len(s.Filtered) == 0 {
		itemViews = append(itemViews, noResultsStyle.Render(s.NoResults))
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
	footerText := "Enter to select • Esc to cancel"
	if s.FooterHint != "" {
		footerText = s.FooterHint + " • " + footerText
	}
	footer := footerStyle.Render(footerText)

	content := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle,
		inputView,
		separator,
		strings.Join(itemViews, "\n"),
		footer,
	)

	return boxStyle.Render(content)
}

// Selected returns the currently selected item, if any.
func (s *ListSelectionScreen) Selected() (SelectionItem, bool) {
	if s.Cursor < 0 || s.Cursor >= len(s.Filtered) {
		return SelectionItem{}, false
	}
	return s.Filtered[s.Cursor], true
}

func (s *ListSelectionScreen) applyFilter() {
	query := strings.ToLower(strings.TrimSpace(s.FilterInput.Value()))
	if query == "" {
		s.Filtered = s.Items
	} else {
		s.Filtered = []SelectionItem{}
		for _, item := range s.Items {
			labelLower := strings.ToLower(item.Label)
			descLower := strings.ToLower(item.Description)
			idLower := strings.ToLower(item.ID)
			if strings.Contains(labelLower, query) || strings.Contains(descLower, query) || strings.Contains(idLower, query) {
				s.Filtered = append(s.Filtered, item)
			}
		}
	}

	if len(s.Filtered) == 0 {
		s.Cursor = -1
	} else if s.Cursor >= len(s.Filtered) || s.Cursor < 0 {
		s.Cursor = 0
	}
	s.ScrollOffset = 0
}
