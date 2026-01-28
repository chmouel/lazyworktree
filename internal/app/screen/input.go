package screen

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/chmouel/lazyworktree/internal/theme"
)

// InputScreen displays a modal input prompt with optional validation,
// history navigation, and checkbox support.
type InputScreen struct {
	// Core fields
	Prompt      string
	Placeholder string
	Value       string
	Input       textinput.Model
	ErrorMsg    string
	Thm         *theme.Theme
	ShowIcons   bool

	// Validation
	Validate func(string) string

	// Callbacks
	OnSubmit         func(value string, checked bool) tea.Cmd
	OnCancel         func() tea.Cmd
	OnCheckboxToggle func(checked bool) tea.Cmd

	// Checkbox support
	CheckboxEnabled bool
	CheckboxChecked bool
	CheckboxFocused bool
	CheckboxLabel   string

	// History navigation (bash-style up/down)
	History       []string
	HistoryIndex  int    // -1 = not browsing
	OriginalInput string // Store original input when browsing history

	// Internal state
	boxWidth int
}

// NewInputScreen creates an input screen with the given parameters.
func NewInputScreen(prompt, placeholder, value string, thm *theme.Theme, showIcons bool) *InputScreen {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.SetValue(value)
	ti.Focus()
	ti.CharLimit = 200
	ti.Prompt = ""
	ti.TextStyle = lipgloss.NewStyle().Foreground(thm.TextFg)

	// Fixed width to match modal style (60 - padding/border = 52 for input)
	ti.Width = 52

	return &InputScreen{
		Prompt:       prompt,
		Placeholder:  placeholder,
		Value:        value,
		Input:        ti,
		ErrorMsg:     "",
		Thm:          thm,
		ShowIcons:    showIcons,
		boxWidth:     60,
		HistoryIndex: -1,
	}
}

// SetValidation sets a validation function that returns an error message.
func (s *InputScreen) SetValidation(fn func(string) string) {
	s.Validate = fn
}

// SetCheckbox enables a checkbox with the given label and default state.
func (s *InputScreen) SetCheckbox(label string, defaultChecked bool) {
	s.CheckboxEnabled = true
	s.CheckboxLabel = label
	s.CheckboxChecked = defaultChecked
	s.CheckboxFocused = false // Default: input field has focus
}

// SetHistory enables bash-style history navigation with up/down arrows.
func (s *InputScreen) SetHistory(history []string) {
	s.History = history
	s.HistoryIndex = -1
	s.OriginalInput = ""
}

// Type returns the screen type.
func (s *InputScreen) Type() Type {
	return TypeInput
}

// Update handles keyboard input for the input screen.
// Returns nil to signal the screen should be closed.
func (s *InputScreen) Update(msg tea.KeyMsg) (Screen, tea.Cmd) {
	var cmd tea.Cmd
	keyStr := msg.String()

	switch keyStr {
	case keyTab:
		// Switch focus between input and checkbox
		if s.CheckboxEnabled {
			s.CheckboxFocused = !s.CheckboxFocused
			return s, nil
		}

	case keyShiftTab:
		// Switch focus in reverse
		if s.CheckboxEnabled {
			s.CheckboxFocused = !s.CheckboxFocused
			return s, nil
		}

	case " ":
		// Only toggle checkbox if it's enabled AND focused
		if s.CheckboxEnabled && s.CheckboxFocused {
			s.CheckboxChecked = !s.CheckboxChecked
			if s.OnCheckboxToggle != nil {
				return s, s.OnCheckboxToggle(s.CheckboxChecked)
			}
			return s, nil
		}
		// Otherwise, fall through to let textinput handle the space

	case keyEnter:
		value := s.Input.Value()

		// Run validation if set
		if s.Validate != nil {
			if errMsg := strings.TrimSpace(s.Validate(value)); errMsg != "" {
				s.ErrorMsg = errMsg
				return s, nil
			}
			s.ErrorMsg = ""
		}

		// Reset history index on submit
		s.HistoryIndex = -1

		// Call submit callback
		if s.OnSubmit != nil {
			cmd = s.OnSubmit(value, s.CheckboxChecked)
			// If OnSubmit set an error message, stay open
			if s.ErrorMsg != "" {
				return s, cmd
			}
		}
		return nil, cmd

	case keyEsc, keyCtrlC:
		if s.OnCancel != nil {
			return nil, s.OnCancel()
		}
		return nil, nil

	case "up":
		// History navigation - go to previous command (older)
		if len(s.History) > 0 {
			if s.HistoryIndex == -1 {
				s.OriginalInput = s.Input.Value()
				s.HistoryIndex = 0
			} else if s.HistoryIndex < len(s.History)-1 {
				s.HistoryIndex++
			}
			if s.HistoryIndex >= 0 && s.HistoryIndex < len(s.History) {
				s.Input.SetValue(s.History[s.HistoryIndex])
				s.Input.CursorEnd()
			}
			return s, nil
		}

	case "down":
		// History navigation - go to next command (newer)
		if len(s.History) > 0 {
			if s.HistoryIndex > 0 {
				s.HistoryIndex--
				s.Input.SetValue(s.History[s.HistoryIndex])
				s.Input.CursorEnd()
			} else if s.HistoryIndex == 0 {
				s.HistoryIndex = -1
				s.Input.SetValue(s.OriginalInput)
				s.Input.CursorEnd()
			}
			return s, nil
		}
	}

	// Reset history browsing when user types
	if msg.Type == tea.KeyRunes || msg.Type == tea.KeyBackspace || msg.Type == tea.KeyDelete {
		s.HistoryIndex = -1
	}

	// Pass to textinput
	s.Input, cmd = s.Input.Update(msg)
	return s, cmd
}

// View renders the input screen.
func (s *InputScreen) View() string {
	width := s.boxWidth

	// Enhanced input modal with rounded border
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.Thm.Accent).
		Padding(1, 2).
		Width(width)

	promptStyle := lipgloss.NewStyle().
		Foreground(s.Thm.Accent).
		Bold(true).
		Width(width - 6).
		Align(lipgloss.Center)

	inputWrapperStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		Padding(0, 1).
		Width(width - 6)

	// Use brighter border when input has focus, dimmer when checkbox is focused
	if s.CheckboxEnabled && s.CheckboxFocused {
		inputWrapperStyle = inputWrapperStyle.BorderForeground(s.Thm.BorderDim)
	} else {
		inputWrapperStyle = inputWrapperStyle.BorderForeground(s.Thm.Border)
	}

	footerStyle := lipgloss.NewStyle().
		Foreground(s.Thm.MutedFg).
		Width(width - 6).
		Align(lipgloss.Center).
		MarginTop(1)

	contentLines := []string{
		promptStyle.Render(s.Prompt),
	}

	// Show checkbox if enabled
	if s.CheckboxEnabled {
		checkbox := "[ ] "
		if s.CheckboxChecked {
			checkbox = "[x] "
		}

		checkboxStyle := lipgloss.NewStyle().
			Width(width - 6).
			MarginTop(1)

		if s.CheckboxFocused {
			// Highlight background when focused
			checkboxStyle = checkboxStyle.
				Background(s.Thm.Accent).
				Foreground(s.Thm.AccentFg).
				Padding(0, 1).
				Bold(true)
		} else {
			// Normal styling when unfocused
			checkboxStyle = checkboxStyle.Foreground(s.Thm.Accent)
		}

		contentLines = append(contentLines, checkboxStyle.Render(checkbox+s.CheckboxLabel))
	}

	contentLines = append(contentLines, inputWrapperStyle.Render(s.Input.View()))

	// Show error message if present
	if s.ErrorMsg != "" {
		errorStyle := lipgloss.NewStyle().
			Foreground(s.Thm.ErrorFg).
			Width(width - 6).
			Align(lipgloss.Center)
		contentLines = append(contentLines, errorStyle.Render(s.ErrorMsg))
	}

	// Footer text varies based on mode
	var footerText string
	switch {
	case s.CheckboxEnabled:
		footerText = "Tab to switch focus • Space to toggle • Enter to confirm • Esc to cancel"
	case len(s.History) > 0:
		footerText = fmt.Sprintf("%s to navigate history • Enter confirm • Esc cancel", arrowPair(s.ShowIcons))
	default:
		footerText = "Enter to confirm • Esc to cancel"
	}
	contentLines = append(contentLines, footerStyle.Render(footerText))

	content := strings.Join(contentLines, "\n\n")

	return boxStyle.Render(content)
}

// arrowPair returns the appropriate arrow indicator based on icon settings.
func arrowPair(showIcons bool) string {
	if !showIcons {
		return "Up/Down"
	}
	// Use Unicode arrows when icons are enabled
	return "↑↓"
}
