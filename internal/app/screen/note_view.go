package screen

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/chmouel/lazyworktree/internal/theme"
	"github.com/muesli/reflow/wrap"
)

// NoteViewScreen displays rendered worktree notes in a pager-like modal.
type NoteViewScreen struct {
	Title    string
	Content  string
	Viewport viewport.Model
	Width    int
	Height   int
	Thm      *theme.Theme

	OnEdit         func() tea.Cmd
	OnEditExternal func() tea.Cmd
	OnClose        func() tea.Cmd
}

// NewNoteViewScreen creates a scrollable notes viewer modal.
func NewNoteViewScreen(title, content string, maxWidth, maxHeight int, thm *theme.Theme) *NoteViewScreen {
	s := &NoteViewScreen{
		Title: title,
		Thm:   thm,
	}
	s.Resize(maxWidth, maxHeight)
	if strings.TrimSpace(content) == "" {
		content = "  "
	}
	s.Content = content
	s.setViewportContent()
	return s
}

// Type returns the screen type.
func (s *NoteViewScreen) Type() Type {
	return TypeNoteView
}

// Resize updates modal and viewport dimensions based on terminal size.
func (s *NoteViewScreen) Resize(maxWidth, maxHeight int) {
	s.Width = 96
	s.Height = 30
	if maxWidth > 0 {
		s.Width = clampInt(int(float64(maxWidth)*0.8), 70, 120)
	}
	if maxHeight > 0 {
		s.Height = clampInt(int(float64(maxHeight)*0.8), 18, 42)
	}
	s.Viewport.SetWidth(maxInt(1, s.Width-6))
	s.Viewport.SetHeight(maxInt(3, s.Height-6))
	s.setViewportContent()
}

// Update handles navigation, close, and edit actions.
func (s *NoteViewScreen) Update(msg tea.KeyPressMsg) (Screen, tea.Cmd) {
	switch msg.String() {
	case keyQ, keyEsc, keyEscRaw, keyCtrlC:
		if s.OnClose != nil {
			return nil, s.OnClose()
		}
		return nil, nil
	case "e":
		if s.OnEdit != nil {
			return nil, s.OnEdit()
		}
		return s, nil
	case "E":
		if s.OnEditExternal != nil {
			return nil, s.OnEditExternal()
		}
		return s, nil
	case "j", "down":
		s.Viewport.ScrollDown(1)
		return s, nil
	case "k", "up":
		s.Viewport.ScrollUp(1)
		return s, nil
	case "ctrl+d", "space":
		s.Viewport.HalfPageDown()
		return s, nil
	case "ctrl+u":
		s.Viewport.HalfPageUp()
		return s, nil
	case "g":
		s.Viewport.GotoTop()
		return s, nil
	case "G":
		s.Viewport.GotoBottom()
		return s, nil
	}

	var cmd tea.Cmd
	s.Viewport, cmd = s.Viewport.Update(msg)
	return s, cmd
}

// View renders the notes viewer modal.
func (s *NoteViewScreen) View() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(s.Thm.Accent).
		Bold(true).
		Width(s.Width - 4).
		Align(lipgloss.Center)

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

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render(s.Title),
		s.Viewport.View(),
		footerStyle.Render("e edit • E editor • q close • j/k scroll • Ctrl+D/U half page • g/G top/bottom"),
	)

	return boxStyle.Render(content)
}

// SetTheme updates the screen theme.
func (s *NoteViewScreen) SetTheme(thm *theme.Theme) {
	s.Thm = thm
}

func (s *NoteViewScreen) setViewportContent() {
	if s.Viewport.Width() <= 0 {
		return
	}
	s.Viewport.SetContent(wrap.String(s.Content, s.Viewport.Width()))
}
