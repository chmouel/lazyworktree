package screen

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/chmouel/lazyworktree/internal/theme"
)

// TaskboardItem represents a task row or a section header in the taskboard.
type TaskboardItem struct {
	ID           string
	WorktreePath string
	WorktreeName string
	Text         string
	Checked      bool

	IsSection    bool
	SectionLabel string
	OpenCount    int
	DoneCount    int
	TotalCount   int
}

// TaskboardScreen displays worktree tasks grouped by worktree.
type TaskboardScreen struct {
	Items    []TaskboardItem
	Filtered []TaskboardItem

	FilterInput  textinput.Model
	FilterActive bool
	Cursor       int
	ScrollOffset int
	Width        int
	Height       int
	Title        string
	NoResults    string
	Thm          *theme.Theme

	OnToggle func(itemID string) tea.Cmd
	OnClose  func() tea.Cmd
	OnAdd    func(worktreePath string) tea.Cmd

	DefaultWorktreePath string
}

// NewTaskboardScreen creates a grouped taskboard modal.
func NewTaskboardScreen(items []TaskboardItem, title string, maxWidth, maxHeight int, thm *theme.Theme) *TaskboardScreen {
	s := &TaskboardScreen{
		Items:     cloneTaskboardItems(items),
		Title:     title,
		NoResults: "No matching tasks.",
		Thm:       thm,
	}
	if strings.TrimSpace(s.Title) == "" {
		s.Title = "Taskboard"
	}
	s.Resize(maxWidth, maxHeight)

	ti := textinput.New()
	ti.Placeholder = "Filter tasks..."
	ti.CharLimit = 100
	ti.Prompt = "/ "
	ti.Blur()
	ti.SetWidth(max(20, s.Width-8))
	s.FilterInput = ti

	s.applyFilter()
	s.Cursor = firstSelectableIndex(s.Filtered)
	return s
}

// Type returns the screen type.
func (s *TaskboardScreen) Type() Type {
	return TypeTaskboard
}

// Resize updates modal dimensions from terminal size.
func (s *TaskboardScreen) Resize(maxWidth, maxHeight int) {
	s.Width = 96
	s.Height = 30
	if maxWidth > 0 {
		s.Width = clampInt(int(float64(maxWidth)*0.85), 72, 130)
	}
	if maxHeight > 0 {
		s.Height = clampInt(int(float64(maxHeight)*0.85), 20, 44)
	}
	if s.FilterInput.Width() > 0 {
		s.FilterInput.SetWidth(max(20, s.Width-8))
	}
}

// SetItems replaces taskboard items and keeps filter/query state.
func (s *TaskboardScreen) SetItems(items []TaskboardItem, preferredID string) {
	s.Items = cloneTaskboardItems(items)
	s.applyFilter()
	if preferredID != "" {
		if idx := s.findFilteredTask(preferredID); idx >= 0 {
			s.Cursor = idx
			s.ensureCursorVisible()
			return
		}
	}
	s.Cursor = firstSelectableIndex(s.Filtered)
	s.ensureCursorVisible()
}

// Update handles keyboard input.
func (s *TaskboardScreen) Update(msg tea.KeyPressMsg) (Screen, tea.Cmd) {
	var cmd tea.Cmd
	keyStr := msg.String()

	if !s.FilterActive {
		switch keyStr {
		case "a":
			if s.OnAdd != nil {
				return s, s.OnAdd(s.currentWorktreePath())
			}
			return s, nil
		case "f":
			s.FilterActive = true
			s.FilterInput.Focus()
			return s, textinput.Blink
		case keyEsc, keyEscRaw, keyQ, keyCtrlC:
			if s.OnClose != nil {
				return nil, s.OnClose()
			}
			return nil, nil
		case keyEnter, "space":
			return s, s.toggleSelected()
		case "up", "k", "ctrl+k":
			s.moveCursor(-1)
			return s, nil
		case "down", "j", "ctrl+j":
			s.moveCursor(1)
			return s, nil
		}
		return s, nil
	}

	switch keyStr {
	case keyEsc:
		s.FilterActive = false
		s.FilterInput.Blur()
		return s, nil
	case keyCtrlC, keyQ:
		if s.OnClose != nil {
			return nil, s.OnClose()
		}
		return nil, nil
	case keyEnter:
		s.FilterActive = false
		s.FilterInput.Blur()
		return s, nil
	case "up", "ctrl+k":
		s.moveCursor(-1)
		return s, nil
	case "down", "ctrl+j":
		s.moveCursor(1)
		return s, nil
	}

	s.FilterInput, cmd = s.FilterInput.Update(msg)
	s.applyFilter()
	s.ensureCursorVisible()
	return s, cmd
}

// View renders the taskboard modal.
func (s *TaskboardScreen) View() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(s.Thm.Accent).
		Bold(true).
		Width(s.Width - 4).
		Align(lipgloss.Center)

	filterLabelStyle := lipgloss.NewStyle().Foreground(s.Thm.Cyan)
	filterValueStyle := lipgloss.NewStyle().Foreground(s.Thm.TextFg)
	sectionStyle := lipgloss.NewStyle().
		Foreground(s.Thm.Accent).
		Background(s.Thm.AccentDim).
		Bold(true)
	selectedStyle := lipgloss.NewStyle().
		Foreground(s.Thm.AccentFg).
		Background(s.Thm.Accent)
	taskStyle := lipgloss.NewStyle().Foreground(s.Thm.TextFg)
	doneStyle := lipgloss.NewStyle().Foreground(s.Thm.MutedFg).Strikethrough(true)
	emptyStyle := lipgloss.NewStyle().Foreground(s.Thm.MutedFg)

	contentWidth := s.Width - 6
	maxVisible := max(4, s.Height-8)
	start := s.ScrollOffset
	end := min(len(s.Filtered), start+maxVisible)

	lines := make([]string, 0, maxVisible)
	if len(s.Filtered) == 0 {
		lines = append(lines, emptyStyle.Render(s.NoResults))
	} else {
		for i := start; i < end; i++ {
			item := s.Filtered[i]
			if item.IsSection {
				sectionLine := fmt.Sprintf(" %s  (%d open / %d done / %d total)", item.SectionLabel, item.OpenCount, item.DoneCount, item.TotalCount)
				lines = append(lines, sectionStyle.Width(contentWidth).Render(ansi.Truncate(sectionLine, contentWidth, "")))
				continue
			}

			pointer := " "
			if i == s.Cursor {
				pointer = ">"
			}
			check := "[ ]"
			style := taskStyle
			if item.Checked {
				check = "[x]"
				style = doneStyle
			}
			taskLine := fmt.Sprintf("%s %s %s", pointer, check, item.Text)
			rendered := style.Render(ansi.Truncate(taskLine, contentWidth, ""))
			if i == s.Cursor {
				rendered = selectedStyle.Width(contentWidth).Render(ansi.Truncate(taskLine, contentWidth, ""))
			}
			lines = append(lines, rendered)
		}
	}
	for len(lines) < maxVisible {
		lines = append(lines, "")
	}

	filterLine := ""
	if s.FilterActive {
		filterLine = fmt.Sprintf("%s %s", filterLabelStyle.Render("Filter:"), filterValueStyle.Render(s.FilterInput.View()))
	}
	footerHelp := "a add • f filter • Enter/Space toggle • j/k move • q close"
	if s.FilterActive {
		footerHelp = "Type to filter • Enter apply • Esc stop filter • q close"
	}
	footerStyle := lipgloss.NewStyle().
		Foreground(s.Thm.MutedFg).
		Width(s.Width - 4).
		Align(lipgloss.Center)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.Thm.Accent).
		Padding(0, 1).
		Width(s.Width).
		Height(s.Height)

	body := strings.Join(lines, "\n")
	contentParts := []string{titleStyle.Render(s.Title)}
	if filterLine != "" {
		contentParts = append(contentParts, filterLine)
	}
	contentParts = append(contentParts, body, footerStyle.Render(footerHelp))

	return boxStyle.Render(strings.Join(contentParts, "\n"))
}

func (s *TaskboardScreen) toggleSelected() tea.Cmd {
	if s.Cursor < 0 || s.Cursor >= len(s.Filtered) {
		return nil
	}
	item := s.Filtered[s.Cursor]
	if item.IsSection || item.ID == "" {
		return nil
	}

	s.flipTaskState(item.ID)
	if s.OnToggle != nil {
		return s.OnToggle(item.ID)
	}
	return nil
}

func (s *TaskboardScreen) currentWorktreePath() string {
	if s.Cursor >= 0 && s.Cursor < len(s.Filtered) && s.Filtered[s.Cursor].WorktreePath != "" {
		return s.Filtered[s.Cursor].WorktreePath
	}
	for _, item := range s.Filtered {
		if item.WorktreePath != "" {
			return item.WorktreePath
		}
	}
	return s.DefaultWorktreePath
}

func (s *TaskboardScreen) flipTaskState(taskID string) {
	for i := range s.Items {
		if s.Items[i].ID == taskID {
			s.Items[i].Checked = !s.Items[i].Checked
		}
	}
	s.recomputeSectionCounts()
	s.applyFilter()
}

func (s *TaskboardScreen) recomputeSectionCounts() {
	openCounts := map[string]int{}
	doneCounts := map[string]int{}
	totalCounts := map[string]int{}

	for _, item := range s.Items {
		if item.IsSection {
			continue
		}
		totalCounts[item.WorktreePath]++
		if item.Checked {
			doneCounts[item.WorktreePath]++
		} else {
			openCounts[item.WorktreePath]++
		}
	}

	for i := range s.Items {
		if !s.Items[i].IsSection {
			continue
		}
		path := s.Items[i].WorktreePath
		s.Items[i].OpenCount = openCounts[path]
		s.Items[i].DoneCount = doneCounts[path]
		s.Items[i].TotalCount = totalCounts[path]
	}
}

func (s *TaskboardScreen) moveCursor(delta int) {
	if len(s.Filtered) == 0 {
		s.Cursor = -1
		return
	}
	if s.Cursor < 0 {
		s.Cursor = firstSelectableIndex(s.Filtered)
		s.ensureCursorVisible()
		return
	}

	for i := s.Cursor + delta; i >= 0 && i < len(s.Filtered); i += delta {
		if !s.Filtered[i].IsSection {
			s.Cursor = i
			s.ensureCursorVisible()
			return
		}
	}
}

func (s *TaskboardScreen) ensureCursorVisible() {
	maxVisible := max(4, s.Height-8)
	if s.Cursor < 0 {
		s.ScrollOffset = 0
		return
	}
	if s.Cursor < s.ScrollOffset {
		s.ScrollOffset = s.Cursor
	}
	if s.Cursor >= s.ScrollOffset+maxVisible {
		s.ScrollOffset = s.Cursor - maxVisible + 1
	}
}

func (s *TaskboardScreen) findFilteredTask(taskID string) int {
	for i, item := range s.Filtered {
		if item.IsSection {
			continue
		}
		if item.ID == taskID {
			return i
		}
	}
	return -1
}

func (s *TaskboardScreen) applyFilter() {
	query := strings.ToLower(strings.TrimSpace(s.FilterInput.Value()))
	if query == "" {
		s.Filtered = cloneTaskboardItems(s.Items)
	} else {
		filtered := make([]TaskboardItem, 0, len(s.Items))
		var pending TaskboardItem
		hasPending := false
		for _, item := range s.Items {
			if item.IsSection {
				pending = item
				hasPending = true
				continue
			}
			haystack := strings.ToLower(item.Text + " " + item.WorktreeName)
			if !strings.Contains(haystack, query) {
				continue
			}
			if hasPending {
				filtered = append(filtered, pending)
				hasPending = false
			}
			filtered = append(filtered, item)
		}
		s.Filtered = filtered
	}

	if s.Cursor >= len(s.Filtered) || s.Cursor < 0 || (len(s.Filtered) > 0 && s.Filtered[s.Cursor].IsSection) {
		s.Cursor = firstSelectableIndex(s.Filtered)
	}
}

func firstSelectableIndex(items []TaskboardItem) int {
	for i, item := range items {
		if !item.IsSection {
			return i
		}
	}
	return -1
}

func cloneTaskboardItems(items []TaskboardItem) []TaskboardItem {
	cloned := make([]TaskboardItem, len(items))
	copy(cloned, items)
	return cloned
}
