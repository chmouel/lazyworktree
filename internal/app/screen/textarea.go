package screen

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/chmouel/lazyworktree/internal/theme"
)

// TextareaScreen displays a modal multiline input.
type TextareaScreen struct {
	Prompt      string
	Placeholder string
	Input       textarea.Model
	ErrorMsg    string
	Thm         *theme.Theme
	ShowIcons   bool

	// Validation
	Validate func(string) string

	// Callbacks
	OnSubmit       func(value string) tea.Cmd
	OnCancel       func() tea.Cmd
	OnEditExternal func(currentValue string) tea.Cmd

	boxWidth  int
	boxHeight int
}

// NewTextareaScreen creates a multiline input modal sized relative to the terminal.
func NewTextareaScreen(prompt, placeholder, value string, maxWidth, maxHeight int, thm *theme.Theme, showIcons bool) *TextareaScreen {
	width := 88
	height := 22
	if maxWidth > 0 {
		width = clampInt(int(float64(maxWidth)*0.62), 62, 100)
	}
	if maxHeight > 0 {
		height = clampInt(int(float64(maxHeight)*0.50), 15, 28)
	}

	ta := textarea.New()
	ta.Placeholder = placeholder
	ta.SetValue(value)
	ta.Prompt = ""
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.SetWidth(width - 4)
	ta.SetHeight(clampInt(height-7, 4, 30))
	ta.Focus()

	taStyles := textarea.DefaultDarkStyles()
	taStyles.Focused.Base = lipgloss.NewStyle().
		Padding(0, 1)
	taStyles.Focused.Text = lipgloss.NewStyle().Foreground(thm.TextFg)
	taStyles.Focused.Prompt = lipgloss.NewStyle().Foreground(thm.Accent)
	taStyles.Focused.Placeholder = lipgloss.NewStyle().Foreground(thm.MutedFg).Italic(true)
	taStyles.Focused.CursorLine = lipgloss.NewStyle().Foreground(thm.TextFg)
	taStyles.Focused.EndOfBuffer = lipgloss.NewStyle().Foreground(thm.MutedFg)
	taStyles.Blurred = taStyles.Focused
	taStyles.Blurred.Base = lipgloss.NewStyle().Padding(0, 1)
	ta.SetStyles(taStyles)

	return &TextareaScreen{
		Prompt:      prompt,
		Placeholder: placeholder,
		Input:       ta,
		Thm:         thm,
		ShowIcons:   showIcons,
		boxWidth:    width,
		boxHeight:   height,
	}
}

// SetValidation sets a validation function that returns an error message.
func (s *TextareaScreen) SetValidation(fn func(string) string) {
	s.Validate = fn
}

// Type returns the screen type.
func (s *TextareaScreen) Type() Type {
	return TypeTextarea
}

// Update handles keyboard input for the textarea screen.
// Returns nil to signal the screen should be closed.
func (s *TextareaScreen) Update(msg tea.KeyPressMsg) (Screen, tea.Cmd) {
	var cmd tea.Cmd
	keyStr := msg.String()

	switch keyStr {
	case "ctrl+s":
		value := s.Input.Value()
		if s.Validate != nil {
			if errMsg := strings.TrimSpace(s.Validate(value)); errMsg != "" {
				s.ErrorMsg = errMsg
				return s, nil
			}
		}
		s.ErrorMsg = ""
		if s.OnSubmit != nil {
			cmd = s.OnSubmit(value)
			if s.ErrorMsg != "" {
				return s, cmd
			}
		}
		return nil, cmd

	case "ctrl+x":
		if s.OnEditExternal != nil {
			return nil, s.OnEditExternal(s.Input.Value())
		}
		return s, nil

	case keyEsc, keyCtrlC:
		if s.OnCancel != nil {
			return nil, s.OnCancel()
		}
		return nil, nil
	}

	s.Input, cmd = s.Input.Update(msg)
	return s, cmd
}

// View renders the multiline input screen.
func (s *TextareaScreen) View() string {
	width := s.boxWidth
	height := s.boxHeight

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.Thm.Accent).
		Padding(0, 1).
		Width(width).
		Height(height)

	promptStyle := lipgloss.NewStyle().
		Foreground(s.Thm.Accent).
		Bold(true)

	footerStyle := lipgloss.NewStyle().
		Foreground(s.Thm.MutedFg)

	contentLines := []string{
		promptStyle.Render(s.Prompt),
		"",
	}

	contentLines = append(contentLines, s.Input.View())

	if s.ErrorMsg != "" {
		errorStyle := lipgloss.NewStyle().
			Foreground(s.Thm.ErrorFg)
		contentLines = append(contentLines, errorStyle.Render(s.ErrorMsg))
	}

	footer := "Ctrl+S save • Esc cancel • Enter newline"
	if s.OnEditExternal != nil {
		footer = "Ctrl+S save • Ctrl+X editor • Esc cancel • Enter newline"
	}
	contentLines = append(contentLines, "", footerStyle.Render(footer))

	return boxStyle.Render(strings.Join(contentLines, "\n"))
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
