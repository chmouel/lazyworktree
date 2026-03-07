package app

import (
	"fmt"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/worktreecolor"
)

const (
	worktreeColorNoneID   = "none"
	worktreeColorCustomID = "custom"
	worktreeColorBoldID   = "bold"
)

func buildCuratedColorItems() []appscreen.SelectionItem {
	colors := worktreecolor.CuratedColors()
	maxLen := len("Custom…")
	for _, c := range colors {
		if len(c.Name) > maxLen {
			maxLen = len(c.Name)
		}
	}
	padFmt := fmt.Sprintf("%%-%ds", maxLen)

	items := make([]appscreen.SelectionItem, 0, 3+len(colors))
	items = append(items,
		appscreen.SelectionItem{ID: worktreeColorNoneID, Label: fmt.Sprintf(padFmt, "None"), Description: "Clear worktree colour"},
		appscreen.SelectionItem{ID: worktreeColorCustomID, Label: fmt.Sprintf(padFmt, "Custom…"), Description: "Enter hex, supported name, or 0-255"},
		appscreen.SelectionItem{ID: worktreeColorBoldID, Label: fmt.Sprintf(padFmt, "Bold"), Description: "Toggle bold styling"},
	)
	for _, c := range colors {
		items = append(items, appscreen.SelectionItem{ID: c.Name, Label: fmt.Sprintf(padFmt, c.Name), Description: c.Description})
	}
	return items
}

var curatedColorItems = buildCuratedColorItems()

func worktreeColorInitialSelection(currentColor string) string {
	if currentColor == "" {
		return worktreeColorNoneID
	}
	if worktreecolor.IsCuratedValue(currentColor) {
		return currentColor
	}
	return worktreeColorCustomID
}

func (m *Model) showCustomWorktreeColorInput(path, currentColor string) tea.Cmd {
	inputScr := appscreen.NewInputScreen(
		"Set worktree colour",
		"#ff0000, red, or 214",
		currentColor,
		m.theme,
		m.config.IconsEnabled(),
	)
	inputScr.SetValidation(func(value string) string {
		trimmed := worktreecolor.Normalize(value)
		if trimmed == "" {
			return "Enter a colour value or choose None to clear it."
		}
		if !worktreecolor.IsValid(trimmed) {
			return "Enter a valid hex colour, supported name, or 0-255 index."
		}
		return ""
	})
	inputScr.OnSubmit = func(value string, _ bool) tea.Cmd {
		m.setWorktreeColor(path, value)
		return nil
	}
	inputScr.OnCancel = func() tea.Cmd {
		return nil
	}
	m.state.ui.screenManager.Push(inputScr)
	return textinput.Blink
}

// showSetWorktreeColor shows a picker to select a colour for the current worktree.
func (m *Model) showSetWorktreeColor() tea.Cmd {
	wt := m.selectedWorktree()
	if wt == nil {
		return nil
	}

	currentColor := ""
	currentBold := false
	if note, ok := m.getWorktreeNote(wt.Path); ok {
		currentColor = worktreecolor.Normalize(note.Color)
		currentBold = note.Bold
	}
	initialID := worktreeColorInitialSelection(currentColor)

	items := make([]appscreen.SelectionItem, len(curatedColorItems))
	for i, it := range curatedColorItems {
		items[i] = it
		switch it.ID {
		case worktreeColorBoldID:
			if currentBold {
				items[i].Description = "Toggle bold styling (currently on)"
			} else {
				items[i].Description = "Toggle bold styling (currently off)"
			}
		case worktreeColorNoneID, worktreeColorCustomID:
			// keep as-is
		default:
			if c := worktreecolor.Resolve(it.ID); c != nil {
				items[i].Label = lipgloss.NewStyle().Foreground(c).Render(it.Label)
			}
		}
	}

	scr := appscreen.NewListSelectionScreen(
		items,
		"Set worktree colour",
		"Filter colours or choose Custom…",
		"No matching colours.",
		m.state.view.WindowWidth,
		m.state.view.WindowHeight,
		initialID,
		m.theme,
	)

	scr.OnSelect = func(item appscreen.SelectionItem) tea.Cmd {
		switch item.ID {
		case worktreeColorNoneID:
			m.setWorktreeColor(wt.Path, "")
			return nil
		case worktreeColorCustomID:
			return m.showCustomWorktreeColorInput(wt.Path, currentColor)
		case worktreeColorBoldID:
			m.toggleWorktreeBold(wt.Path)
			return nil
		default:
			m.setWorktreeColor(wt.Path, item.ID)
			return nil
		}
	}
	scr.OnCancel = func() tea.Cmd {
		return nil
	}

	m.state.ui.screenManager.Push(scr)
	return nil
}
