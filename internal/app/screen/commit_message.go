package screen

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/chmouel/lazyworktree/internal/theme"
)

type commitMessageFocus int

const (
	commitMessageFocusSubject commitMessageFocus = iota
	commitMessageFocusBody
	commitSubjectSoftLimit = 50
)

// CommitMessageScreen displays a commit form with a dedicated subject field and body editor.
type CommitMessageScreen struct {
	Prompt          string
	Placeholder     string
	SubjectInput    textinput.Model
	BodyInput       textarea.Model
	ErrorMsg        string
	Thm             *theme.Theme
	ShowIcons       bool
	HasAutoGenerate bool

	// Validation
	Validate func(string) string

	// Callbacks
	OnSubmit       func(value string) tea.Cmd
	OnCancel       func() tea.Cmd
	OnAutoGenerate func() tea.Cmd
	OnEditExternal func(currentValue string) tea.Cmd

	boxWidth  int
	boxHeight int
	focus     commitMessageFocus
}

// NewCommitMessageScreen creates a commit modal with separate subject and body inputs.
func NewCommitMessageScreen(prompt, placeholder, value string, maxWidth, maxHeight int, thm *theme.Theme, showIcons, hasAutoGenerate bool) *CommitMessageScreen {
	width := 104
	height := 28
	if maxWidth > 0 {
		width = clampInt(int(float64(maxWidth)*0.78), 80, 132)
	}
	if maxHeight > 0 {
		height = clampInt(int(float64(maxHeight)*0.72), 22, 38)
	}

	subject, body := splitCommitMessage(value)

	ti := textinput.New()
	ti.Placeholder = "Subject"
	ti.SetValue(subject)
	ti.Focus()
	ti.CharLimit = 200
	ti.Prompt = ""
	ti.SetWidth(width - 6)
	tiStyles := ti.Styles()
	tiStyles.Focused.Text = lipgloss.NewStyle().Foreground(thm.TextFg)
	tiStyles.Focused.Placeholder = lipgloss.NewStyle().Foreground(thm.MutedFg).Italic(true)
	tiStyles.Blurred.Text = lipgloss.NewStyle().Foreground(thm.TextFg)
	tiStyles.Blurred.Placeholder = lipgloss.NewStyle().Foreground(thm.MutedFg).Italic(true)
	ti.SetStyles(tiStyles)

	ta := textarea.New()
	ta.Placeholder = placeholder
	ta.SetValue(body)
	ta.Prompt = ""
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.SetWidth(width - 4)
	ta.SetHeight(clampInt(height-11, 8, 30))

	taStyles := textarea.DefaultDarkStyles()
	taStyles.Focused.Base = lipgloss.NewStyle().Padding(0, 1)
	taStyles.Focused.Text = lipgloss.NewStyle().Foreground(thm.TextFg)
	taStyles.Focused.Prompt = lipgloss.NewStyle().Foreground(thm.Accent)
	taStyles.Focused.Placeholder = lipgloss.NewStyle().Foreground(thm.MutedFg).Italic(true)
	taStyles.Focused.CursorLine = lipgloss.NewStyle().Foreground(thm.TextFg)
	taStyles.Focused.EndOfBuffer = lipgloss.NewStyle().Foreground(thm.MutedFg)
	taStyles.Blurred = taStyles.Focused
	taStyles.Blurred.Base = lipgloss.NewStyle().Padding(0, 1)
	ta.SetStyles(taStyles)

	screen := &CommitMessageScreen{
		Prompt:          prompt,
		Placeholder:     placeholder,
		SubjectInput:    ti,
		BodyInput:       ta,
		Thm:             thm,
		ShowIcons:       showIcons,
		HasAutoGenerate: hasAutoGenerate,
		boxWidth:        width,
		boxHeight:       height,
		focus:           commitMessageFocusSubject,
	}
	screen.syncFocus()
	screen.applySubjectStyles()
	return screen
}

// SetValidation sets a validation function that returns an error message.
func (s *CommitMessageScreen) SetValidation(fn func(string) string) {
	s.Validate = fn
}

// Type returns the screen type.
func (s *CommitMessageScreen) Type() Type {
	return TypeCommitMessage
}

// Update handles keyboard input and pasted text for the commit message screen.
func (s *CommitMessageScreen) Update(raw tea.Msg) (Screen, tea.Cmd) {
	var cmd tea.Cmd

	if msg, ok := raw.(tea.KeyPressMsg); ok {
		switch msg.String() {
		case keyTab, keyShiftTab:
			s.toggleFocus()
			return s, nil

		case keyEnter:
			if s.focus == commitMessageFocusSubject {
				s.focus = commitMessageFocusBody
				s.syncFocus()
				return s, nil
			}

		case "ctrl+s":
			value := s.Value()
			if strings.TrimSpace(s.SubjectInput.Value()) == "" {
				s.ErrorMsg = "Commit subject cannot be empty."
				return s, nil
			}
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

		case "ctrl+o":
			if s.HasAutoGenerate && s.OnAutoGenerate != nil {
				return s, s.OnAutoGenerate()
			}

		case "ctrl+x":
			if s.OnEditExternal != nil {
				s.ErrorMsg = ""
				return s, s.OnEditExternal(s.Value())
			}
			return s, nil

		case keyEsc, keyCtrlC:
			if s.OnCancel != nil {
				return nil, s.OnCancel()
			}
			return nil, nil
		}
	}

	if s.focus == commitMessageFocusSubject {
		s.SubjectInput, cmd = s.SubjectInput.Update(raw)
	} else {
		s.BodyInput, cmd = s.BodyInput.Update(raw)
	}
	s.applySubjectStyles()
	return s, cmd
}

// View renders the commit message screen.
func (s *CommitMessageScreen) View() string {
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
	labelStyle := lipgloss.NewStyle().
		Foreground(s.Thm.MutedFg)
	focusedLabelStyle := lipgloss.NewStyle().
		Foreground(s.Thm.Accent).
		Bold(true)
	footerStyle := lipgloss.NewStyle().
		Foreground(s.Thm.MutedFg)
	subjectCountStyle := lipgloss.NewStyle().Foreground(s.Thm.MutedFg)
	s.applySubjectStyles()
	if s.subjectTooLong() {
		subjectCountStyle = lipgloss.NewStyle().Foreground(s.Thm.ErrorFg)
	}

	subjectLabel := labelStyle.Render("Subject")
	bodyLabel := labelStyle.Render("Body")
	if s.focus == commitMessageFocusSubject {
		subjectLabel = focusedLabelStyle.Render("Subject")
	} else {
		bodyLabel = focusedLabelStyle.Render("Body")
	}

	contentLines := []string{
		promptStyle.Render(s.Prompt),
		"",
		subjectLabel,
		lipgloss.NewStyle().Padding(0, 1).Render(s.SubjectInput.View()),
		subjectCountStyle.Render(fmt.Sprintf("%d/%d", utf8.RuneCountInString(s.SubjectInput.Value()), commitSubjectSoftLimit)),
		"",
		bodyLabel,
		s.BodyInput.View(),
	}

	if s.ErrorMsg != "" {
		errorStyle := lipgloss.NewStyle().
			Foreground(s.Thm.ErrorFg)
		contentLines = append(contentLines, errorStyle.Render(s.ErrorMsg))
	}

	footerParts := []string{"Tab switch field", "Ctrl+S save"}
	if s.HasAutoGenerate {
		footerParts = append(footerParts, "Ctrl+O auto-generate")
	}
	if s.OnEditExternal != nil {
		footerParts = append(footerParts, "Ctrl+X editor")
	}
	footerParts = append(footerParts, "Esc cancel", "Enter moves to body / adds newline")
	contentLines = append(contentLines, "", footerStyle.Render(strings.Join(footerParts, " • ")))

	return boxStyle.Render(strings.Join(contentLines, "\n"))
}

// Value returns the combined commit message.
func (s *CommitMessageScreen) Value() string {
	subject := strings.TrimSpace(s.SubjectInput.Value())
	body := s.BodyInput.Value()
	if strings.TrimSpace(body) == "" {
		return subject
	}
	return subject + "\n\n" + body
}

// SetValue updates the subject and body from a combined commit message.
func (s *CommitMessageScreen) SetValue(value string) {
	subject, body := splitCommitMessage(value)
	s.SubjectInput.SetValue(subject)
	s.BodyInput.SetValue(body)
}

// SetGeneratedValue updates the subject and body using the generator contract:
// line 1 is the subject and line 3 onwards form the body.
func (s *CommitMessageScreen) SetGeneratedValue(value string) {
	subject, body := parseGeneratedCommitMessage(value)
	s.SubjectInput.SetValue(subject)
	s.BodyInput.SetValue(body)
}

// SetErrorMsg displays an error message on the screen.
func (s *CommitMessageScreen) SetErrorMsg(msg string) {
	s.ErrorMsg = msg
}

func (s *CommitMessageScreen) toggleFocus() {
	if s.focus == commitMessageFocusSubject {
		s.focus = commitMessageFocusBody
	} else {
		s.focus = commitMessageFocusSubject
	}
	s.syncFocus()
}

func (s *CommitMessageScreen) syncFocus() {
	if s.focus == commitMessageFocusSubject {
		s.SubjectInput.Focus()
		s.BodyInput.Blur()
		return
	}
	s.SubjectInput.Blur()
	s.BodyInput.Focus()
}

func splitCommitMessage(raw string) (string, string) {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	normalized = strings.TrimRight(normalized, "\n")
	if normalized == "" {
		return "", ""
	}
	subject, body, found := strings.Cut(normalized, "\n")
	if !found {
		return subject, ""
	}
	body = strings.TrimPrefix(body, "\n")
	return subject, body
}

func parseGeneratedCommitMessage(raw string) (string, string) {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	normalized = strings.TrimRight(normalized, "\n")
	if normalized == "" {
		return "", ""
	}

	lines := strings.Split(normalized, "\n")
	subject := strings.TrimSpace(lines[0])
	if len(lines) < 3 {
		return subject, ""
	}
	return subject, strings.Join(lines[2:], "\n")
}

func (s *CommitMessageScreen) subjectTooLong() bool {
	return utf8.RuneCountInString(s.SubjectInput.Value()) > commitSubjectSoftLimit
}

func (s *CommitMessageScreen) applySubjectStyles() {
	styles := s.SubjectInput.Styles()
	textColour := s.Thm.TextFg
	if s.subjectTooLong() {
		textColour = s.Thm.ErrorFg
	}
	styles.Focused.Text = lipgloss.NewStyle().Foreground(textColour)
	styles.Blurred.Text = lipgloss.NewStyle().Foreground(textColour)
	styles.Focused.Placeholder = lipgloss.NewStyle().Foreground(s.Thm.MutedFg).Italic(true)
	styles.Blurred.Placeholder = lipgloss.NewStyle().Foreground(s.Thm.MutedFg).Italic(true)
	s.SubjectInput.SetStyles(styles)
}
