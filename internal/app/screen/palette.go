package screen

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/chmouel/lazyworktree/internal/theme"
)

// PaletteItem represents a command in the palette.
type PaletteItem struct {
	ID          string
	Label       string
	Description string
	IsSection   bool   // Non-selectable section headers
	IsMRU       bool   // Recently used items
	Shortcut    string // Keyboard shortcut display (e.g., "g")
	Icon        string // Category icon (Nerd Font)
}

// CommandPaletteScreen is the command picker modal.
type CommandPaletteScreen struct {
	Items        []PaletteItem
	Filtered     []PaletteItem
	FilterInput  textinput.Model
	FilterActive bool
	Cursor       int
	ScrollOffset int
	Width        int
	Height       int
	Thm          *theme.Theme

	// Callbacks
	OnSelect func(actionID string) tea.Cmd
	OnCancel func() tea.Cmd
}

// NewCommandPaletteScreen builds a command palette screen.
func NewCommandPaletteScreen(items []PaletteItem, maxWidth, maxHeight int, thm *theme.Theme) *CommandPaletteScreen {
	// Calculate palette width: 80% of screen, capped between 60 and 110
	width := int(float64(maxWidth) * 0.8)
	width = max(60, min(110, width))

	ti := textinput.New()
	ti.Placeholder = "Type a command..."
	ti.CharLimit = 100
	ti.Prompt = "  " // Search icon (Nerd Font)
	ti.Focus()
	ti.SetWidth(width - 6) // fits inside box with padding and icon

	// Find first non-section item for initial cursor
	initialCursor := 0
	for i, item := range items {
		if !item.IsSection {
			initialCursor = i
			break
		}
	}

	screen := &CommandPaletteScreen{
		Items:        items,
		Filtered:     items,
		FilterInput:  ti,
		FilterActive: true,
		Cursor:       initialCursor,
		ScrollOffset: 0,
		Width:        width,
		Height:       maxHeight,
		Thm:          thm,
	}
	return screen
}

// Type returns the screen type identifier.
func (s *CommandPaletteScreen) Type() Type {
	return TypePalette
}

// Update handles keyboard input for the command palette.
func (s *CommandPaletteScreen) Update(msg tea.KeyPressMsg) (Screen, tea.Cmd) {
	const maxVisible = 12
	keyStr := msg.String()

	if !s.FilterActive {
		switch keyStr {
		case "f":
			s.FilterActive = true
			s.FilterInput.Focus()
			return s, textinput.Blink
		case "esc", "ctrl+c":
			if s.OnCancel != nil {
				cmd := s.OnCancel()
				return nil, cmd
			}
			return nil, nil
		case "enter":
			if s.Cursor >= 0 && s.Cursor < len(s.Filtered) && !s.Filtered[s.Cursor].IsSection {
				item := s.Filtered[s.Cursor]
				if s.OnSelect != nil {
					cmd := s.OnSelect(item.ID)
					return nil, cmd
				}
			}
			return nil, nil
		case "up", "k", "ctrl+k":
			if s.Cursor > 0 {
				s.Cursor--
				// Skip sections when navigating
				for s.Cursor > 0 && s.Filtered[s.Cursor].IsSection {
					s.Cursor--
				}
				if s.Cursor < s.ScrollOffset {
					s.ScrollOffset = s.Cursor
				}
			}
			return s, nil
		case "down", "j", "ctrl+j":
			if s.Cursor < len(s.Filtered)-1 {
				s.Cursor++
				// Skip sections when navigating
				for s.Cursor < len(s.Filtered)-1 && s.Filtered[s.Cursor].IsSection {
					s.Cursor++
				}
				if s.Cursor >= s.ScrollOffset+maxVisible {
					s.ScrollOffset = s.Cursor - maxVisible + 1
				}
			}
			return s, nil
		}
		return s, nil
	}

	switch keyStr {
	case "esc":
		s.FilterActive = false
		s.FilterInput.Blur()
		return s, nil
	case "ctrl+c":
		if s.OnCancel != nil {
			cmd := s.OnCancel()
			return nil, cmd
		}
		return nil, nil
	case "enter":
		if s.Cursor >= 0 && s.Cursor < len(s.Filtered) && !s.Filtered[s.Cursor].IsSection {
			item := s.Filtered[s.Cursor]
			if s.OnSelect != nil {
				cmd := s.OnSelect(item.ID)
				return nil, cmd
			}
		}
		return nil, nil
	case "up", "ctrl+k":
		if s.Cursor > 0 {
			s.Cursor--
			// Skip sections when navigating
			for s.Cursor > 0 && s.Filtered[s.Cursor].IsSection {
				s.Cursor--
			}
			if s.Cursor < s.ScrollOffset {
				s.ScrollOffset = s.Cursor
			}
		}
		return s, nil
	case "down", "ctrl+j":
		if s.Cursor < len(s.Filtered)-1 {
			s.Cursor++
			// Skip sections when navigating
			for s.Cursor < len(s.Filtered)-1 && s.Filtered[s.Cursor].IsSection {
				s.Cursor++
			}
			if s.Cursor >= s.ScrollOffset+maxVisible {
				s.ScrollOffset = s.Cursor - maxVisible + 1
			}
		}
		return s, nil
	}

	// Update filter input for all other keys
	var cmd tea.Cmd
	s.FilterInput, cmd = s.FilterInput.Update(msg)
	s.applyFilter()
	return s, cmd
}

// applyFilter filters items by the current query.
func (s *CommandPaletteScreen) applyFilter() {
	query := strings.ToLower(strings.TrimSpace(s.FilterInput.Value()))

	if query == "" {
		s.Filtered = s.Items
	} else {
		s.Filtered = make([]PaletteItem, 0, len(s.Items))
		var pendingSection PaletteItem
		hasPendingSection := false

		for _, item := range s.Items {
			if item.IsSection {
				pendingSection = item
				hasPendingSection = true
				continue
			}

			// Fuzzy match: all query characters must appear in order
			label := strings.ToLower(item.Label)
			desc := strings.ToLower(item.Description)
			combined := label + " " + desc

			matched := true
			pos := 0
			for _, ch := range query {
				idx := strings.IndexRune(combined[pos:], ch)
				if idx == -1 {
					matched = false
					break
				}
				pos += idx + 1
			}

			if matched {
				if hasPendingSection {
					s.Filtered = append(s.Filtered, pendingSection)
					hasPendingSection = false
				}
				s.Filtered = append(s.Filtered, item)
			}
		}
	}

	// Reset cursor and scroll offset if list changes
	if s.Cursor >= len(s.Filtered) {
		s.Cursor = max(0, len(s.Filtered)-1)
	}

	// Find first non-section item for cursor
	for s.Cursor < len(s.Filtered) && s.Filtered[s.Cursor].IsSection {
		s.Cursor++
	}
	if s.Cursor >= len(s.Filtered) {
		s.Cursor = 0
	}

	s.ScrollOffset = 0
}

// highlightMatches highlights matching characters in text based on the query.
func (s *CommandPaletteScreen) highlightMatches(text, query string) string {
	if query == "" {
		return text
	}

	var result strings.Builder
	textLower := strings.ToLower(text)
	queryLower := strings.ToLower(query)
	pos := 0

	accentStyle := lipgloss.NewStyle().Foreground(s.Thm.Accent).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(s.Thm.TextFg)

	for _, qch := range queryLower {
		idx := strings.IndexRune(textLower[pos:], qch)
		if idx == -1 {
			break
		}
		// Render unmatched portion
		if idx > 0 {
			result.WriteString(normalStyle.Render(text[pos : pos+idx]))
		}
		// Render matched char
		result.WriteString(accentStyle.Render(string(text[pos+idx])))
		pos += idx + 1
	}
	// Remainder
	if pos < len(text) {
		result.WriteString(normalStyle.Render(text[pos:]))
	}
	return result.String()
}

// View renders the command palette.
func (s *CommandPaletteScreen) View() string {
	width := s.Width
	if width == 0 {
		width = 110 // fallback for tests
	}

	// Calculate maxVisible based on available height
	// Reserve: 1 input + 1 separator + 1 footer + 2 border = ~5 lines
	maxVisible := s.Height - 5
	if !s.FilterActive {
		maxVisible += 2
	}
	maxVisible = max(5, min(20, maxVisible))
	if s.Height == 0 {
		maxVisible = 12 // fallback for tests
	}

	// Enhanced palette modal with rounded border
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.Thm.Accent).
		Width(width).
		Padding(0)

	inputStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(width - 2).
		Foreground(s.Thm.TextFg)

	// Section header with top and bottom border lines
	sectionStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(width-2).
		Foreground(s.Thm.Accent).
		Bold(true).
		Border(lipgloss.NormalBorder(), true, false, true, false).
		BorderForeground(s.Thm.BorderDim)

	// Normal item style
	itemStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(width - 2)

	// Selected item with accent border and subtle background tint
	selectedStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(width - 4). // Narrower to accommodate left border
		Foreground(s.Thm.TextFg).
		Background(s.Thm.AccentDim).
		Bold(true)

	// Left accent border for selected item
	selectedBorderStyle := lipgloss.NewStyle().
		Foreground(s.Thm.Accent).
		Background(s.Thm.AccentDim).
		Bold(true)

	// Shortcut badge style
	shortcutStyle := lipgloss.NewStyle().
		Foreground(s.Thm.AccentFg).
		Background(s.Thm.Accent).
		Bold(true).
		Padding(0, 1)

	descStyle := lipgloss.NewStyle().
		Foreground(s.Thm.MutedFg)

	selectedDescStyle := lipgloss.NewStyle().
		Foreground(s.Thm.MutedFg)

	noResultsStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(width - 2).
		Foreground(s.Thm.MutedFg).
		Italic(true)

	// Icon style
	iconStyle := lipgloss.NewStyle().
		Foreground(s.Thm.MutedFg)

	selectedIconStyle := lipgloss.NewStyle().
		Foreground(s.Thm.Accent).
		Bold(true)

	// Get current query for highlighting
	query := strings.TrimSpace(s.FilterInput.Value())

	// Render Items
	var itemViews []string

	end := s.ScrollOffset + maxVisible
	if end > len(s.Filtered) {
		end = len(s.Filtered)
	}
	start := s.ScrollOffset
	if start > end {
		start = end
	}

	// Calculate available width for label and description
	// Width breakdown: 2 padding + 2 icon + 1 space + label + desc + shortcut
	labelWidth := 28
	shortcutWidth := 6                                  // space for shortcut badge
	descWidth := width - labelWidth - shortcutWidth - 8 // Leave room for icon, padding, and UI elements

	for i := start; i < end; i++ {
		it := s.Filtered[i]

		// Render section headers with bottom border underline
		if it.IsSection {
			icon := it.Icon
			if icon == "" {
				icon = "" // Default section icon
			}
			leftPart := icon + "  " + it.Label

			itemViews = append(itemViews, sectionStyle.Render(leftPart))
			continue
		}

		// Truncate label if too long
		label := it.Label
		if len(label) > labelWidth {
			label = label[:labelWidth-1] + "…"
		}

		// Truncate description
		desc := it.Description
		if len(desc) > descWidth {
			desc = desc[:descWidth-1] + "…"
		}

		// Build the item icon
		icon := it.Icon
		if icon == "" {
			icon = " " // Space placeholder for alignment
		}

		// Calculate padding for alignment
		paddedLabel := fmt.Sprintf("%-*s", labelWidth, label)
		paddedDesc := fmt.Sprintf("%-*s", descWidth, desc)

		// Render shortcut badge if available
		shortcutBadge := ""
		if it.Shortcut != "" {
			shortcutBadge = " " + shortcutStyle.Render(it.Shortcut)
		}

		if i == s.Cursor {
			// Selected item with left accent border and background tint
			styledIcon := selectedIconStyle.Render(icon)
			styledLabel := paddedLabel
			styledDesc := selectedDescStyle.Render(paddedDesc)

			line := styledIcon + " " + styledLabel + " " + styledDesc + shortcutBadge

			border := selectedBorderStyle.Render("▎")
			itemContent := selectedStyle.Render(line)
			itemViews = append(itemViews, border+itemContent)
		} else {
			// Normal item
			styledIcon := iconStyle.Render(icon)

			// Apply match highlighting when filtering
			styledLabel := paddedLabel
			if query != "" {
				styledLabel = s.highlightMatches(paddedLabel, query)
			}

			styledDesc := descStyle.Render(paddedDesc)

			line := styledIcon + " " + styledLabel + " " + styledDesc + shortcutBadge

			// Add space to align with selected item border
			itemViews = append(itemViews, itemStyle.Render(" "+line))
		}
	}

	if len(s.Filtered) == 0 {
		itemViews = append(itemViews, noResultsStyle.Render("No commands match your filter."))
	}

	// Separator
	separator := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(s.Thm.BorderDim).
		Width(width - 2).
		Render("")

	// Footer with item count and keyboard hints
	countText := fmt.Sprintf("%d of %d", s.Cursor+1, len(s.Filtered))
	if len(s.Filtered) == 0 {
		countText = "No matches"
	}

	// Add scroll indicator
	if len(s.Filtered) > maxVisible {
		switch {
		case s.ScrollOffset > 0 && end < len(s.Filtered):
			countText += " ↕"
		case s.ScrollOffset > 0:
			countText += " ▲"
		case end < len(s.Filtered):
			countText += " ▼"
		}
	}

	hints := "↑↓ navigate • ⏎ select • Esc close"
	if !s.FilterActive {
		hints = "f filter • ↑↓ navigate • ⏎ select • Esc close"
	}

	leftStyle := lipgloss.NewStyle().
		Foreground(s.Thm.MutedFg).
		Width((width - 4) / 2)
	rightStyle := lipgloss.NewStyle().
		Foreground(s.Thm.MutedFg).
		Width((width - 4) / 2).
		Align(lipgloss.Right)

	footer := lipgloss.JoinHorizontal(lipgloss.Top,
		leftStyle.Render(countText),
		rightStyle.Render(hints),
	)

	// Add top padding to footer
	footerWithPadding := lipgloss.NewStyle().PaddingTop(1).Render(footer)

	contentLines := []string{}
	if s.FilterActive {
		inputView := inputStyle.Render(s.FilterInput.View())
		contentLines = append(contentLines, inputView, separator)
	}
	contentLines = append(contentLines, strings.Join(itemViews, "\n"), footerWithPadding)
	content := lipgloss.JoinVertical(lipgloss.Left, contentLines...)

	return boxStyle.Render(content)
}
