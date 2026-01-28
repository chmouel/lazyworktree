package screen

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/chmouel/lazyworktree/internal/theme"
)

// CommitMeta holds metadata for a commit.
type CommitMeta struct {
	SHA     string
	Author  string
	Email   string
	Date    string
	Subject string
	Body    []string
}

// CommitScreen displays metadata, stats, and diff details for a single commit.
type CommitScreen struct {
	Meta     CommitMeta
	Stat     string
	Diff     string
	UseDelta bool
	Viewport viewport.Model
	Thm      *theme.Theme
}

// NewCommitScreen configures the commit detail viewer for the selected SHA.
func NewCommitScreen(meta CommitMeta, stat, diff string, useDelta bool, thm *theme.Theme) *CommitScreen {
	vp := viewport.New(110, 60)

	s := &CommitScreen{
		Meta:     meta,
		Stat:     stat,
		Diff:     diff,
		UseDelta: useDelta,
		Viewport: vp,
		Thm:      thm,
	}

	s.setViewportContent()
	return s
}

// Type returns the screen type.
func (s *CommitScreen) Type() Type {
	return TypeCommit
}

// Update handles scrolling and closing events for the commit screen.
// Returns nil to signal that the screen should be closed.
func (s *CommitScreen) Update(msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch msg.String() {
	case keyQ, keyEsc, keyEscRaw, keyCtrlC:
		return nil, nil
	case "j", "down":
		s.Viewport.ScrollDown(1)
		return s, nil
	case "k", "up":
		s.Viewport.ScrollUp(1)
		return s, nil
	case "ctrl+d", " ":
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

func (s *CommitScreen) setViewportContent() {
	s.Viewport.SetContent(s.buildBody())
}

func (s *CommitScreen) buildBody() string {
	var parts []string
	parts = append(parts, s.renderHeader())
	if strings.TrimSpace(s.Stat) != "" {
		parts = append(parts, s.Stat)
	}
	if strings.TrimSpace(s.Diff) != "" {
		parts = append(parts, s.Diff)
	}
	return strings.Join(parts, "\n\n")
}

func (s *CommitScreen) renderHeader() string {
	label := lipgloss.NewStyle().Foreground(s.Thm.MutedFg).Bold(true)
	value := lipgloss.NewStyle().Foreground(s.Thm.TextFg)
	subjectStyle := lipgloss.NewStyle().Bold(true).Foreground(s.Thm.Accent)
	bodyStyle := lipgloss.NewStyle().Foreground(s.Thm.MutedFg)

	lines := []string{
		fmt.Sprintf("%s %s", label.Render("Commit:"), value.Render(s.Meta.SHA)),
		fmt.Sprintf("%s %s <%s>", label.Render("Author:"), value.Render(s.Meta.Author), value.Render(s.Meta.Email)),
		fmt.Sprintf("%s %s", label.Render("Date:"), value.Render(s.Meta.Date)),
	}
	if s.Meta.Subject != "" {
		lines = append(lines, "")
		lines = append(lines, subjectStyle.Render(s.Meta.Subject))
	}
	if len(s.Meta.Body) > 0 {
		for _, l := range s.Meta.Body {
			if strings.TrimSpace(l) == "" {
				lines = append(lines, "")
				continue
			}
			lines = append(lines, bodyStyle.Render(l))
		}
	}

	header := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Padding(0, 1).
		Render(header)
}

// View renders the commit screen.
func (s *CommitScreen) View() string {
	width := max(100, s.Viewport.Width)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.Thm.Accent).
		Padding(0, 1).
		Width(width)

	return boxStyle.Render(s.Viewport.View())
}

// SetTheme updates the theme for this screen.
func (s *CommitScreen) SetTheme(thm *theme.Theme) {
	s.Thm = thm
}
