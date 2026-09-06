package screen

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/chmouel/lazyworktree/internal/theme"
)

func TestInputScreenPasteInsertsContent(t *testing.T) {
	scr := NewInputScreen("Branch name", "feature/my-branch", "", theme.Dracula(), false)

	next, cmd := scr.Update(tea.PasteMsg{Content: "feature/pasted-branch"})
	require.NotNil(t, next)
	inputScr, ok := next.(*InputScreen)
	require.True(t, ok)
	require.Equal(t, "feature/pasted-branch", inputScr.Input.Value())
	_ = cmd
}

func TestInputScreenPasteInsertsAtCursor(t *testing.T) {
	scr := NewInputScreen("Branch name", "feature/my-branch", "feature/", theme.Dracula(), false)
	scr.Input.CursorEnd()

	next, _ := scr.Update(tea.PasteMsg{Content: "pasted"})
	inputScr, ok := next.(*InputScreen)
	require.True(t, ok)
	require.Equal(t, "feature/pasted", inputScr.Input.Value())
}

func TestInputScreenPasteResetsHistoryBrowsing(t *testing.T) {
	scr := NewInputScreen("Run command", "e.g. make test", "", theme.Dracula(), false)
	scr.SetHistory([]string{"make build", "make test"})

	next, _ := scr.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	inputScr, ok := next.(*InputScreen)
	require.True(t, ok)
	require.Equal(t, 0, inputScr.HistoryIndex)

	next, _ = inputScr.Update(tea.PasteMsg{Content: "pasted command"})
	inputScr, ok = next.(*InputScreen)
	require.True(t, ok)
	require.Equal(t, -1, inputScr.HistoryIndex)
}

func TestInputScreenPasteIgnoredWhenCheckboxFocused(t *testing.T) {
	scr := NewInputScreen("Create from current", "feature/my-branch", "generated-name", theme.Dracula(), false)
	scr.SetCheckbox("Include current file changes", true)
	scr.CheckboxFocused = true

	next, _ := scr.Update(tea.PasteMsg{Content: "pasted-branch-name"})
	inputScr, ok := next.(*InputScreen)
	require.True(t, ok)
	require.Equal(t, "generated-name", inputScr.Input.Value())
	require.True(t, inputScr.CheckboxFocused)
}

func TestInputScreenTypingIgnoredWhenCheckboxFocused(t *testing.T) {
	scr := NewInputScreen("Create from current", "feature/my-branch", "generated-name", theme.Dracula(), false)
	scr.SetCheckbox("Include current file changes", true)
	scr.CheckboxFocused = true

	next, _ := scr.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	inputScr, ok := next.(*InputScreen)
	require.True(t, ok)
	require.Equal(t, "generated-name", inputScr.Input.Value())
}
