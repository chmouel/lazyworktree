package screen

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/chmouel/lazyworktree/internal/theme"
)

// PaletteItem represents a command in the palette.
type PaletteItem struct {
	ID          string
	Label       string
	Description string
	IsSection   bool // Non-selectable section headers
	IsMRU       bool // Recently used items
}

// CommandPaletteScreen is the command picker modal.
type CommandPaletteScreen struct {
	Items        []PaletteItem
	Filtered     []PaletteItem
	FilterInput  textinput.Model
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
	ti.Prompt = "> "
	ti.Focus()
	ti.Width = width - 4 // fits inside box with padding

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
func (s *CommandPaletteScreen) Update(msg tea.KeyMsg) (Screen, tea.Cmd) {
	const maxVisible = 12
	keyStr := msg.String()

	switch keyStr {
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
		for _, item := range s.Items {
			// Always include sections for visual grouping
			if item.IsSection {
				s.Filtered = append(s.Filtered, item)
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

// View renders the command palette.
func (s *CommandPaletteScreen) View() string {
	width := s.Width
	if width == 0 {
		width = 110 // fallback for tests
	}

	// Calculate maxVisible based on available height
	// Reserve: 1 input + 1 separator + 1 footer + 2 border = ~5 lines
	maxVisible := s.Height - 5
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

	itemStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(width - 2)

	selectedStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(width - 2).
		Background(s.Thm.Accent).
		Foreground(s.Thm.AccentFg).
		Bold(true)

	descStyle := lipgloss.NewStyle().
		Foreground(s.Thm.MutedFg)

	selectedDescStyle := lipgloss.NewStyle().
		Foreground(s.Thm.TextFg)

	noResultsStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(width - 2).
		Foreground(s.Thm.MutedFg).
		Italic(true)

	sectionStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(width - 2).
		Foreground(s.Thm.Accent).
		Bold(true)

	// Render Input
	inputView := inputStyle.Render(s.FilterInput.View())

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

	for i := start; i < end; i++ {
		it := s.Filtered[i]

		// Render section headers differently
		if it.IsSection {
			itemViews = append(itemViews, sectionStyle.Render("── "+it.Label+" ──"))
			continue
		}

		// Truncate label if too long
		label := it.Label
		desc := it.Description

		// Pad label to align descriptions somewhat
		labelPad := 45
		if len(label) > labelPad {
			label = label[:labelPad-1] + "…"
		}
		paddedLabel := fmt.Sprintf("%-45s", label)

		var line string
		if i == s.Cursor {
			line = fmt.Sprintf("%s %s", paddedLabel, selectedDescStyle.Render(desc))
			itemViews = append(itemViews, selectedStyle.Render(line))
		} else {
			line = fmt.Sprintf("%s %s", paddedLabel, descStyle.Render(desc))
			itemViews = append(itemViews, itemStyle.Render(line))
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

	// Footer
	footerStyle := lipgloss.NewStyle().
		Foreground(s.Thm.MutedFg).
		Align(lipgloss.Right).
		Width(width - 2).
		PaddingTop(1)
	footer := footerStyle.Render("esc to close")

	content := lipgloss.JoinVertical(lipgloss.Left,
		inputView,
		separator,
		strings.Join(itemViews, "\n"),
		footer,
	)

	return boxStyle.Render(content)
}
