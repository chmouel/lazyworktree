package screen

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

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

	matchMode   paletteMatchMode
	matchOrder  int
	matchScore  int
	matchSource paletteMatchSource
	matchStart  int
}

type paletteMatchMode int

const (
	paletteMatchModeNone paletteMatchMode = iota
	paletteMatchModeExact
	paletteMatchModePrefix
	paletteMatchModeSubstring
	paletteMatchModeFuzzy
)

type paletteMatchSource int

const (
	paletteMatchSourceNone paletteMatchSource = iota
	paletteMatchSourceLabel
	paletteMatchSourceDescription
	paletteMatchSourceCombined
)

const noPaletteMatchScore = int(^uint(0) >> 1)

type paletteMatch struct {
	mode   paletteMatchMode
	score  int
	source paletteMatchSource
	start  int
}

type paletteSection struct {
	hasHeader bool
	header    PaletteItem
	items     []PaletteItem
	bestScore int
	order     int
}

type textSpan struct {
	start int
	end   int
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
	for i := range items {
		item := items[i]
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
		s.Filtered = s.buildFilteredItems(query)
	}

	s.Cursor = 0
	for s.Cursor < len(s.Filtered) && s.Filtered[s.Cursor].IsSection {
		s.Cursor++
	}
	if s.Cursor >= len(s.Filtered) {
		s.Cursor = max(0, len(s.Filtered)-1)
	}

	s.ScrollOffset = 0
}

func (s *CommandPaletteScreen) buildFilteredItems(query string) []PaletteItem {
	sections := make([]paletteSection, 0, len(s.Items))
	current := paletteSection{bestScore: noPaletteMatchScore}
	hasCurrent := false
	itemOrder := 0
	sectionOrder := 0

	flushSection := func() {
		if !hasCurrent || len(current.items) == 0 {
			return
		}
		sort.SliceStable(current.items, func(i, j int) bool {
			if current.items[i].matchScore != current.items[j].matchScore {
				return current.items[i].matchScore < current.items[j].matchScore
			}
			return current.items[i].matchOrder < current.items[j].matchOrder
		})
		sections = append(sections, current)
	}

	for i := range s.Items {
		item := s.Items[i]
		if item.IsSection {
			flushSection()
			current = paletteSection{
				hasHeader: true,
				header:    item,
				bestScore: noPaletteMatchScore,
				order:     sectionOrder,
			}
			hasCurrent = true
			sectionOrder++
			continue
		}

		if !hasCurrent {
			current = paletteSection{bestScore: noPaletteMatchScore, order: sectionOrder}
			hasCurrent = true
			sectionOrder++
		}

		match, ok := scorePaletteItem(query, item)
		if !ok {
			continue
		}
		match.matchOrder = itemOrder
		itemOrder++
		current.items = append(current.items, match)
		if match.matchScore < current.bestScore {
			current.bestScore = match.matchScore
		}
	}
	flushSection()

	sort.SliceStable(sections, func(i, j int) bool {
		if sections[i].bestScore != sections[j].bestScore {
			return sections[i].bestScore < sections[j].bestScore
		}
		return sections[i].order < sections[j].order
	})

	filtered := make([]PaletteItem, 0, len(s.Items))
	for i := range sections {
		section := sections[i]
		if section.hasHeader {
			filtered = append(filtered, section.header)
		}
		filtered = append(filtered, section.items...)
	}

	return filtered
}

func scorePaletteItem(query string, item PaletteItem) (PaletteItem, bool) {
	label := strings.ToLower(item.Label)
	desc := strings.ToLower(item.Description)
	combined := label + " " + desc

	best := paletteMatch{score: noPaletteMatchScore}

	candidates := []paletteMatch{
		bestTextMatch(label, query, paletteMatchSourceLabel, 0, 100, 200),
		bestTextMatch(desc, query, paletteMatchSourceDescription, 300, 400, 500),
	}

	if score, ok := fuzzyScoreLower(query, label); ok {
		candidates = append(candidates, paletteMatch{
			mode:   paletteMatchModeFuzzy,
			score:  600 + score,
			source: paletteMatchSourceLabel,
		})
	}
	if score, ok := fuzzyScoreLower(query, desc); ok {
		candidates = append(candidates, paletteMatch{
			mode:   paletteMatchModeFuzzy,
			score:  700 + score,
			source: paletteMatchSourceDescription,
		})
	}
	if score, ok := fuzzyScoreLower(query, combined); ok {
		candidates = append(candidates, paletteMatch{
			mode:   paletteMatchModeFuzzy,
			score:  800 + score,
			source: paletteMatchSourceCombined,
		})
	}

	for _, candidate := range candidates {
		if candidate.score < best.score {
			best = candidate
		}
	}

	if best.score == noPaletteMatchScore {
		return PaletteItem{}, false
	}

	item.matchMode = best.mode
	item.matchScore = best.score
	item.matchSource = best.source
	item.matchStart = best.start
	return item, true
}

func bestTextMatch(text, query string, source paletteMatchSource, exactBase, prefixBase, substringBase int) paletteMatch {
	if query == "" || text == "" {
		return paletteMatch{score: noPaletteMatchScore}
	}

	if start := findExactWordMatch(text, query); start >= 0 {
		return paletteMatch{
			mode:   paletteMatchModeExact,
			score:  exactBase + start,
			source: source,
			start:  start,
		}
	}
	if start := findWordPrefixMatch(text, query); start >= 0 {
		return paletteMatch{
			mode:   paletteMatchModePrefix,
			score:  prefixBase + start,
			source: source,
			start:  start,
		}
	}
	if start := strings.Index(text, query); start >= 0 {
		return paletteMatch{
			mode:   paletteMatchModeSubstring,
			score:  substringBase + start,
			source: source,
			start:  start,
		}
	}

	return paletteMatch{score: noPaletteMatchScore}
}

func findExactWordMatch(text, query string) int {
	for _, span := range wordSpans(text) {
		if text[span.start:span.end] == query {
			return span.start
		}
	}
	return -1
}

func findWordPrefixMatch(text, query string) int {
	for _, span := range wordSpans(text) {
		word := text[span.start:span.end]
		if strings.HasPrefix(word, query) {
			return span.start
		}
	}
	return -1
}

func wordSpans(text string) []textSpan {
	spans := make([]textSpan, 0, strings.Count(text, " ")+1)
	start := -1
	for idx, r := range text {
		if isWordRune(r) {
			if start == -1 {
				start = idx
			}
			continue
		}
		if start >= 0 {
			spans = append(spans, textSpan{start: start, end: idx})
			start = -1
		}
	}
	if start >= 0 {
		spans = append(spans, textSpan{start: start, end: len(text)})
	}
	return spans
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

func fuzzyScoreLower(query, target string) (int, bool) {
	if query == "" {
		return 0, true
	}

	tRunes := []rune(target)
	score := 0
	lastIdx := -1
	searchFrom := 0

	for _, qc := range query {
		found := false
		for i, tc := range tRunes {
			if i < searchFrom {
				continue
			}
			if tc != qc {
				continue
			}
			if lastIdx >= 0 {
				gap := i - lastIdx - 1
				score += gap * 2
				if gap == 0 {
					score--
				}
			} else {
				score += i * 2
			}
			lastIdx = i
			searchFrom = i + 1
			found = true
			break
		}
		if !found {
			return 0, false
		}
	}

	return score, true
}

func (s *CommandPaletteScreen) highlightFuzzyMatches(text, query string, baseStyle lipgloss.Style) string {
	if query == "" {
		return baseStyle.Render(text)
	}

	var result strings.Builder
	textLower := strings.ToLower(text)
	queryLower := strings.ToLower(query)
	pos := 0

	accentStyle := lipgloss.NewStyle().Foreground(s.Thm.Accent).Bold(true)

	for _, qch := range queryLower {
		idx := strings.IndexRune(textLower[pos:], qch)
		if idx == -1 {
			break
		}
		if idx > 0 {
			result.WriteString(baseStyle.Render(text[pos : pos+idx]))
		}
		result.WriteString(accentStyle.Render(string(text[pos+idx])))
		pos += idx + 1
	}
	if pos < len(text) {
		result.WriteString(baseStyle.Render(text[pos:]))
	}
	return result.String()
}

func (s *CommandPaletteScreen) highlightContiguousMatch(text string, start, length int, baseStyle lipgloss.Style) string {
	if start < 0 || start >= len(text) || length <= 0 {
		return baseStyle.Render(text)
	}
	end := min(len(text), start+length)
	accentStyle := lipgloss.NewStyle().Foreground(s.Thm.Accent).Bold(true)
	var result strings.Builder
	if start > 0 {
		result.WriteString(baseStyle.Render(text[:start]))
	}
	result.WriteString(accentStyle.Render(text[start:end]))
	if end < len(text) {
		result.WriteString(baseStyle.Render(text[end:]))
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

	// Selected item with prominent highlight background
	selectedStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(width - 2).
		Foreground(s.Thm.AccentFg).
		Background(s.Thm.Accent).
		Bold(true)

	labelStyle := lipgloss.NewStyle().
		Foreground(s.Thm.TextFg)

	descStyle := lipgloss.NewStyle().
		Foreground(s.Thm.MutedFg)

	noResultsStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(width - 2).
		Foreground(s.Thm.MutedFg).
		Italic(true)

	// Icon style
	iconStyle := lipgloss.NewStyle().
		Foreground(s.Thm.MutedFg)

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
	descWidth := width - labelWidth - 8 // Leave room for icon, padding, and UI elements

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

		if i == s.Cursor {
			// Selected item with prominent highlight
			line := icon + " " + paddedLabel + " " + paddedDesc

			itemViews = append(itemViews, selectedStyle.Render(line))
		} else {
			// Normal item
			styledIcon := iconStyle.Render(icon)
			styledLabel := labelStyle.Render(paddedLabel)
			styledDesc := descStyle.Render(paddedDesc)
			if query != "" {
				switch it.matchSource {
				case paletteMatchSourceLabel:
					if it.matchMode == paletteMatchModeFuzzy {
						styledLabel = s.highlightFuzzyMatches(paddedLabel, query, labelStyle)
					} else {
						styledLabel = s.highlightContiguousMatch(paddedLabel, it.matchStart, len(query), labelStyle)
					}
				case paletteMatchSourceDescription:
					if it.matchMode == paletteMatchModeFuzzy {
						styledDesc = s.highlightFuzzyMatches(paddedDesc, query, descStyle)
					} else {
						styledDesc = s.highlightContiguousMatch(paddedDesc, it.matchStart, len(query), descStyle)
					}
				case paletteMatchSourceCombined:
					styledLabel = s.highlightFuzzyMatches(paddedLabel, query, labelStyle)
					styledDesc = s.highlightFuzzyMatches(paddedDesc, query, descStyle)
				}
			}

			line := styledIcon + " " + styledLabel + " " + styledDesc

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

	footer := lipgloss.JoinHorizontal(
		lipgloss.Top,
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
