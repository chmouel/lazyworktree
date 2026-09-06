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

// TagEditorOption describes an existing tag that can be toggled in the editor.
type TagEditorOption struct {
	Tag   string
	Count int
}

type tagEditorFocus int

const (
	tagEditorFocusInput tagEditorFocus = iota
	tagEditorFocusList
)

// TagEditorScreen lets the user type free-form tags whilst toggling existing ones.
type TagEditorScreen struct {
	Prompt    string
	Input     textinput.Model
	Available []TagEditorOption
	ErrorMsg  string
	Thm       *theme.Theme
	ShowIcons bool

	OnSubmit func([]string) tea.Cmd
	OnCancel func() tea.Cmd

	Width        int
	Height       int
	Cursor       int
	ScrollOffset int

	focus tagEditorFocus
}

// NewTagEditorScreen creates a modal for editing worktree tags.
func NewTagEditorScreen(prompt string, current []string, available []TagEditorOption, maxWidth, maxHeight int, thm *theme.Theme, showIcons bool) *TagEditorScreen {
	width := 96
	height := 28
	if maxWidth > 0 {
		width = clampInt(int(float64(maxWidth)*0.82), 72, 124)
	}
	if maxHeight > 0 {
		height = clampInt(int(float64(maxHeight)*0.8), 22, 38)
	}

	ti := textinput.New()
	ti.Placeholder = "bug, frontend, urgent"
	ti.SetValue(formatTagEditorValue(current))
	ti.Focus()
	ti.CharLimit = 200
	ti.Prompt = ""
	ti.SetWidth(width - 8)
	styles := ti.Styles()
	styles.Focused.Text = lipgloss.NewStyle().Foreground(thm.TextFg)
	styles.Focused.Placeholder = lipgloss.NewStyle().Foreground(thm.MutedFg).Italic(true)
	styles.Blurred.Text = lipgloss.NewStyle().Foreground(thm.TextFg)
	styles.Blurred.Placeholder = lipgloss.NewStyle().Foreground(thm.MutedFg).Italic(true)
	ti.SetStyles(styles)

	screen := &TagEditorScreen{
		Prompt:    prompt,
		Input:     ti,
		Available: append([]TagEditorOption(nil), available...),
		Thm:       thm,
		ShowIcons: showIcons,
		Width:     width,
		Height:    height,
		Cursor:    0,
		focus:     tagEditorFocusInput,
	}
	if len(screen.Available) == 0 {
		screen.Cursor = -1
	}
	return screen
}

// Type returns the screen type.
func (s *TagEditorScreen) Type() Type {
	return TypeTagEditor
}

// Update handles keyboard input and pasted text for the tag editor.
func (s *TagEditorScreen) Update(raw tea.Msg) (Screen, tea.Cmd) {
	msg, ok := raw.(tea.KeyPressMsg)
	if !ok {
		if s.focus != tagEditorFocusList {
			var cmd tea.Cmd
			s.Input, cmd = s.Input.Update(raw)
			s.ErrorMsg = ""
			return s, cmd
		}
		return s, nil
	}
	keyStr := msg.String()

	switch keyStr {
	case keyTab, keyShiftTab:
		s.toggleFocus()
		return s, nil
	case "ctrl+s":
		return nil, s.submit()
	case keyEsc, keyCtrlC:
		if s.OnCancel != nil {
			return nil, s.OnCancel()
		}
		return nil, nil
	}

	if s.focus == tagEditorFocusList {
		switch keyStr {
		case keyEnter, "space":
			s.toggleSelectedTag()
			return s, nil
		case "up", "k", "ctrl+k":
			s.moveCursor(-1)
			return s, nil
		case "down", "j", "ctrl+j":
			s.moveCursor(1)
			return s, nil
		}
		return s, nil
	}

	if keyStr == keyEnter {
		return nil, s.submit()
	}

	var cmd tea.Cmd
	s.Input, cmd = s.Input.Update(msg)
	s.ErrorMsg = ""
	return s, cmd
}

// View renders the tag editor modal.
func (s *TagEditorScreen) View() string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.Thm.Accent).
		Padding(0, 1).
		Width(s.Width).
		Height(s.Height)

	titleStyle := lipgloss.NewStyle().
		Foreground(s.Thm.Accent).
		Bold(true).
		Width(s.Width - 4).
		Align(lipgloss.Center)

	labelStyle := lipgloss.NewStyle().
		Foreground(s.Thm.MutedFg)
	focusedLabelStyle := lipgloss.NewStyle().
		Foreground(s.Thm.Accent).
		Bold(true)
	inputWrapperStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		Padding(0, 1).
		Width(s.Width - 6)
	listStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		Padding(0, 1).
		Width(s.Width - 6).
		Height(max(4, s.maxVisible()+1))

	if s.focus == tagEditorFocusInput {
		inputWrapperStyle = inputWrapperStyle.BorderForeground(s.Thm.Border)
		listStyle = listStyle.BorderForeground(s.Thm.BorderDim)
	} else {
		inputWrapperStyle = inputWrapperStyle.BorderForeground(s.Thm.BorderDim)
		listStyle = listStyle.BorderForeground(s.Thm.Border)
	}

	inputLabel := labelStyle.Render("Tags")
	existingLabel := labelStyle.Render("Existing tags")
	if s.focus == tagEditorFocusInput {
		inputLabel = focusedLabelStyle.Render("Tags")
	} else {
		existingLabel = focusedLabelStyle.Render("Existing tags")
	}

	contentLines := []string{
		titleStyle.Render(s.Prompt),
		"",
		inputLabel,
		inputWrapperStyle.Render(s.Input.View()),
		"",
		existingLabel,
		listStyle.Render(s.renderList()),
	}

	if s.ErrorMsg != "" {
		errorStyle := lipgloss.NewStyle().Foreground(s.Thm.ErrorFg)
		contentLines = append(contentLines, "", errorStyle.Render(s.ErrorMsg))
	}

	footerStyle := lipgloss.NewStyle().
		Foreground(s.Thm.MutedFg).
		Width(s.Width - 4).
		Align(lipgloss.Center)
	contentLines = append(
		contentLines,
		"",
		footerStyle.Render("Tab switch field • Enter save/toggle • Space toggle tag • j/k move • Esc cancel"),
	)

	return boxStyle.Render(strings.Join(contentLines, "\n"))
}

// Tags returns the current normalized tags from the input value.
func (s *TagEditorScreen) Tags() []string {
	return parseTagEditorValue(s.Input.Value())
}

func (s *TagEditorScreen) submit() tea.Cmd {
	s.ErrorMsg = ""
	if s.OnSubmit != nil {
		return s.OnSubmit(s.Tags())
	}
	return nil
}

func (s *TagEditorScreen) toggleFocus() {
	if s.focus == tagEditorFocusInput {
		s.focus = tagEditorFocusList
		s.Input.Blur()
		return
	}
	s.focus = tagEditorFocusInput
	s.Input.Focus()
}

func (s *TagEditorScreen) moveCursor(delta int) {
	if len(s.Available) == 0 {
		s.Cursor = -1
		return
	}
	s.Cursor = clampInt(s.Cursor+delta, 0, len(s.Available)-1)
	if s.Cursor < s.ScrollOffset {
		s.ScrollOffset = s.Cursor
	}
	maxVisible := s.maxVisible()
	if s.Cursor >= s.ScrollOffset+maxVisible {
		s.ScrollOffset = s.Cursor - maxVisible + 1
	}
}

func (s *TagEditorScreen) toggleSelectedTag() {
	if s.Cursor < 0 || s.Cursor >= len(s.Available) {
		return
	}
	tags := s.Tags()
	tag := s.Available[s.Cursor].Tag
	if hasTagEditorValue(tags, tag) {
		tags = removeTagEditorValue(tags, tag)
	} else {
		tags = append(tags, tag)
	}
	s.Input.SetValue(formatTagEditorValue(tags))
	s.Input.CursorEnd()
	s.ErrorMsg = ""
}

func (s *TagEditorScreen) renderList() string {
	maxVisible := s.maxVisible()
	if len(s.Available) == 0 {
		return lipgloss.NewStyle().
			Foreground(s.Thm.MutedFg).
			Italic(true).
			Render("No existing tags yet. Type a new one above.")
	}

	selected := make(map[string]struct{}, len(s.Available))
	for _, tag := range s.Tags() {
		selected[strings.ToLower(tag)] = struct{}{}
	}

	itemStyle := lipgloss.NewStyle().Width(s.Width - 10)
	selectedStyle := lipgloss.NewStyle().
		Width(s.Width - 10).
		Background(s.Thm.Accent).
		Foreground(s.Thm.AccentFg).
		Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(s.Thm.MutedFg)

	start := min(s.ScrollOffset, len(s.Available))
	end := min(len(s.Available), start+maxVisible)
	lines := make([]string, 0, maxVisible)
	for i := start; i < end; i++ {
		item := s.Available[i]
		check := "[ ]"
		if _, ok := selected[strings.ToLower(item.Tag)]; ok {
			check = "[x]"
		}
		pointer := " "
		if i == s.Cursor {
			pointer = ">"
		}
		line := fmt.Sprintf("%s %s %s", pointer, check, item.Tag)
		if item.Count > 0 {
			suffix := fmt.Sprintf(" (%d)", item.Count)
			contentWidth := max(0, s.Width-10-len([]rune(suffix)))
			line = ansi.Truncate(line, contentWidth, "") + descStyle.Render(suffix)
		}
		if i == s.Cursor && s.focus == tagEditorFocusList {
			lines = append(lines, selectedStyle.Render(ansi.Truncate(line, s.Width-10, "")))
			continue
		}
		lines = append(lines, itemStyle.Render(ansi.Truncate(line, s.Width-10, "")))
	}
	for len(lines) < maxVisible {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (s *TagEditorScreen) maxVisible() int {
	return max(4, s.Height-16)
}

func parseTagEditorValue(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		tags = append(tags, trimmed)
	}
	if len(tags) == 0 {
		return nil
	}
	return tags
}

func formatTagEditorValue(tags []string) string {
	return strings.Join(parseTagEditorValue(strings.Join(tags, ",")), ", ")
}

func hasTagEditorValue(tags []string, tag string) bool {
	for _, existing := range tags {
		if strings.EqualFold(existing, tag) {
			return true
		}
	}
	return false
}

func removeTagEditorValue(tags []string, tag string) []string {
	filtered := tags[:0]
	for _, existing := range tags {
		if strings.EqualFold(existing, tag) {
			continue
		}
		filtered = append(filtered, existing)
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}
