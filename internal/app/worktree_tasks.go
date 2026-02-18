package app

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/models"
)

var (
	markdownTaskLineRE = regexp.MustCompile(`^(\s*[-*+]\s+\[)([ xX])(\]\s*)(.*)$`)
	todoKeywordLineRE  = regexp.MustCompile(`^(\s*)(TODO|DONE)(:?\s*)(.*)$`)
)

type worktreeTaskRef struct {
	ID           string
	WorktreePath string
	LineIndex    int
	Checked      bool
	Text         string
	IsKeyword    bool
}

func (m *Model) showTaskboard() tea.Cmd {
	items, refs := m.buildTaskboardData()
	if len(items) == 0 {
		m.showInfo("No tasks found.\n\nPress i on a worktree and add markdown checkboxes such as:\n- [ ] Review commit log", nil)
		return nil
	}

	taskboard := appscreen.NewTaskboardScreen(
		items,
		"Worktree Taskboard",
		m.state.view.WindowWidth,
		m.state.view.WindowHeight,
		m.theme,
	)

	taskboard.OnToggle = func(itemID string) tea.Cmd {
		ref, ok := refs[itemID]
		if !ok {
			return nil
		}
		if !m.toggleTaskInWorktreeNote(ref) {
			return nil
		}

		m.updateTable()
		if m.state.data.selectedIndex >= 0 && m.state.data.selectedIndex < len(m.state.data.filteredWts) {
			m.infoContent = m.buildInfoContent(m.state.data.filteredWts[m.state.data.selectedIndex])
		}

		nextItems, nextRefs := m.buildTaskboardData()
		refs = nextRefs
		taskboard.SetItems(nextItems, itemID)
		return nil
	}
	taskboard.OnClose = func() tea.Cmd {
		return nil
	}

	m.state.ui.screenManager.Push(taskboard)
	return nil
}

func (m *Model) buildTaskboardData() ([]appscreen.TaskboardItem, map[string]worktreeTaskRef) {
	worktrees := make([]*models.WorktreeInfo, 0, len(m.state.data.worktrees))
	for _, wt := range m.state.data.worktrees {
		if wt == nil || strings.TrimSpace(wt.Path) == "" {
			continue
		}
		worktrees = append(worktrees, wt)
	}

	sort.Slice(worktrees, func(i, j int) bool {
		left := taskboardWorktreeName(worktrees[i])
		right := taskboardWorktreeName(worktrees[j])
		if left == right {
			return worktrees[i].Path < worktrees[j].Path
		}
		return left < right
	})

	items := make([]appscreen.TaskboardItem, 0, 32)
	refs := make(map[string]worktreeTaskRef, 32)

	for _, wt := range worktrees {
		note, ok := m.getWorktreeNote(wt.Path)
		if !ok {
			continue
		}
		taskRefs := extractTaskRefs(wt.Path, note.Note)
		if len(taskRefs) == 0 {
			continue
		}

		openCount := 0
		doneCount := 0
		for _, ref := range taskRefs {
			if ref.Checked {
				doneCount++
			} else {
				openCount++
			}
		}

		items = append(items, appscreen.TaskboardItem{
			IsSection:    true,
			WorktreePath: wt.Path,
			SectionLabel: taskboardWorktreeName(wt),
			OpenCount:    openCount,
			DoneCount:    doneCount,
			TotalCount:   len(taskRefs),
		})

		for _, ref := range taskRefs {
			items = append(items, appscreen.TaskboardItem{
				ID:           ref.ID,
				WorktreePath: ref.WorktreePath,
				WorktreeName: taskboardWorktreeName(wt),
				Text:         ref.Text,
				Checked:      ref.Checked,
			})
			refs[ref.ID] = ref
		}
	}

	return items, refs
}

func taskboardWorktreeName(wt *models.WorktreeInfo) string {
	if wt == nil {
		return ""
	}
	if wt.IsMain {
		return mainWorktreeName
	}
	return filepath.Base(wt.Path)
}

func extractTaskRefs(worktreePath, noteText string) []worktreeTaskRef {
	normalized := strings.ReplaceAll(noteText, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	taskRefs := make([]worktreeTaskRef, 0, len(lines))

	for i, line := range lines {
		checked, text, ok := parseMarkdownTaskLine(line)
		isKeyword := false
		if !ok {
			checked, text, ok = parseTodoKeywordLine(line)
			if ok {
				isKeyword = true
			}
		}
		if !ok {
			continue
		}
		id := fmt.Sprintf("%s:%d", filepath.Clean(worktreePath), i)
		taskRefs = append(taskRefs, worktreeTaskRef{
			ID:           id,
			WorktreePath: worktreePath,
			LineIndex:    i,
			Checked:      checked,
			Text:         text,
			IsKeyword:    isKeyword,
		})
	}

	return taskRefs
}

func parseMarkdownTaskLine(line string) (checked bool, text string, ok bool) {
	parts := markdownTaskLineRE.FindStringSubmatch(line)
	if len(parts) != 5 {
		return false, "", false
	}

	checked = strings.EqualFold(parts[2], "x")
	text = strings.TrimSpace(parts[4])
	if text == "" {
		text = "(untitled task)"
	}
	return checked, text, true
}

func (m *Model) toggleTaskInWorktreeNote(ref worktreeTaskRef) bool {
	note, ok := m.getWorktreeNote(ref.WorktreePath)
	if !ok {
		return false
	}

	var next string
	var changed bool
	if ref.IsKeyword {
		next, changed = toggleTodoKeywordLine(note.Note, ref.LineIndex)
	} else {
		next, changed = toggleMarkdownTaskLine(note.Note, ref.LineIndex)
	}
	if !changed {
		return false
	}
	m.setWorktreeNote(ref.WorktreePath, next)
	return true
}

func toggleMarkdownTaskLine(noteText string, lineIndex int) (string, bool) {
	normalized := strings.ReplaceAll(noteText, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	if lineIndex < 0 || lineIndex >= len(lines) {
		return noteText, false
	}

	line := lines[lineIndex]
	idx := markdownTaskLineRE.FindStringSubmatchIndex(line)
	if len(idx) < 6 {
		return noteText, false
	}

	checkStart := idx[4]
	checkEnd := idx[5]
	if checkStart < 0 || checkEnd <= checkStart || checkEnd > len(line) {
		return noteText, false
	}

	replacement := "x"
	if strings.EqualFold(line[checkStart:checkEnd], "x") {
		replacement = " "
	}
	lines[lineIndex] = line[:checkStart] + replacement + line[checkEnd:]
	return strings.Join(lines, "\n"), true
}

func parseTodoKeywordLine(line string) (checked bool, text string, ok bool) {
	parts := todoKeywordLineRE.FindStringSubmatch(line)
	if len(parts) != 5 {
		return false, "", false
	}
	checked = parts[2] == "DONE"
	text = strings.TrimSpace(parts[4])
	if text == "" {
		text = "(untitled task)"
	}
	return checked, text, true
}

func toggleTodoKeywordLine(noteText string, lineIndex int) (string, bool) {
	normalized := strings.ReplaceAll(noteText, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	if lineIndex < 0 || lineIndex >= len(lines) {
		return noteText, false
	}

	line := lines[lineIndex]
	idx := todoKeywordLineRE.FindStringSubmatchIndex(line)
	if len(idx) < 6 {
		return noteText, false
	}

	kwStart := idx[4]
	kwEnd := idx[5]
	keyword := line[kwStart:kwEnd]

	var replacement string
	if keyword == "TODO" {
		replacement = "DONE"
	} else {
		replacement = "TODO"
	}
	lines[lineIndex] = line[:kwStart] + replacement + line[kwEnd:]
	return strings.Join(lines, "\n"), true
}
