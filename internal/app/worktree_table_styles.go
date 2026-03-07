package app

import (
	"image/color"

	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"
	"github.com/chmouel/lazyworktree/internal/theme"
	"github.com/chmouel/lazyworktree/internal/worktreecolor"
)

func buildWorktreeTableStyles(thm *theme.Theme, selectedColor color.Color, bold bool) table.Styles {
	s := table.DefaultStyles()
	if selectedColor != nil {
		s.Selected = s.Selected.Foreground(selectedColor)
	} else {
		s.Selected = s.Selected.Foreground(thm.Accent)
	}
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(thm.BorderDim).
		BorderBottom(true).
		Bold(true).
		Foreground(thm.Cyan).
		Background(thm.AccentDim)
	s.Selected = s.Selected.Bold(bold)
	return s
}

func (m *Model) updateWorktreeTableStyles() {
	var selectedColor color.Color
	bold := true
	if wt := m.selectedWorktree(); wt != nil {
		if note, ok := m.getWorktreeNote(wt.Path); ok {
			if note.Color != "" {
				selectedColor = worktreecolor.Resolve(note.Color)
			}
			bold = note.Bold
		}
	}
	m.state.ui.worktreeTable.SetStyles(buildWorktreeTableStyles(m.theme, selectedColor, bold))
}
