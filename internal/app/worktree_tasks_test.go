package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

func TestParseMarkdownTaskLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantOK   bool
		wantDone bool
		wantText string
	}{
		{name: "unchecked", line: "- [ ] Write docs", wantOK: true, wantDone: false, wantText: "Write docs"},
		{name: "checked upper", line: "* [X] Ship it", wantOK: true, wantDone: true, wantText: "Ship it"},
		{name: "checked lower", line: "+ [x] Merge PR", wantOK: true, wantDone: true, wantText: "Merge PR"},
		{name: "not task", line: "- TODO: plain text tag", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			done, text, ok := parseMarkdownTaskLine(tt.line)
			if ok != tt.wantOK {
				t.Fatalf("ok=%v want=%v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if done != tt.wantDone {
				t.Fatalf("checked=%v want=%v", done, tt.wantDone)
			}
			if text != tt.wantText {
				t.Fatalf("text=%q want=%q", text, tt.wantText)
			}
		})
	}
}

func TestToggleMarkdownTaskLinePreservesFormatting(t *testing.T) {
	note := "## Notes\n  - [ ]   Keep spacing exactly\n- [x] done"
	updated, ok := toggleMarkdownTaskLine(note, 1)
	if !ok {
		t.Fatal("expected toggle to succeed")
	}
	lines := strings.Split(updated, "\n")
	if len(lines) != 3 {
		t.Fatalf("unexpected line count: %d", len(lines))
	}
	if lines[0] != "## Notes" {
		t.Fatalf("expected heading unchanged, got %q", lines[0])
	}
	if lines[1] != "  - [x]   Keep spacing exactly" {
		t.Fatalf("expected only checkbox marker to flip, got %q", lines[1])
	}
	if lines[2] != "- [x] done" {
		t.Fatalf("expected unrelated line unchanged, got %q", lines[2])
	}
}

func TestShowTaskboardNoTasksShowsInfo(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: "/tmp/wt-a", Branch: "feat-a"},
	}
	m.setWorktreeNote("/tmp/wt-a", "Just prose, no checkboxes.")

	cmd := m.showTaskboard()
	if cmd != nil {
		t.Fatal("expected nil command for no-task flow")
	}
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Fatalf("expected info screen, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}
}

func TestShowTaskboardAndToggleTask(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	wtPath := "/tmp/wt-a"
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: wtPath, Branch: "feat-a"},
	}
	m.state.data.filteredWts = m.state.data.worktrees
	m.state.data.selectedIndex = 0
	m.state.view.FocusedPane = 0
	m.setWorktreeNote(wtPath, "- [ ] Write tests\n- [x] done already")

	cmd := m.showTaskboard()
	if cmd != nil {
		t.Fatalf("expected nil command, got %v", cmd)
	}
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeTaskboard {
		t.Fatalf("expected taskboard screen, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}

	updated, _ := m.handleScreenKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = updated.(*Model)

	note, ok := m.getWorktreeNote(wtPath)
	if !ok {
		t.Fatal("expected note to exist")
	}
	if !strings.Contains(note.Note, "- [x] Write tests") {
		t.Fatalf("expected first task to be toggled, got note:\n%s", note.Note)
	}
}

func TestParseTodoKeywordLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantOK   bool
		wantDone bool
		wantText string
	}{
		{name: "todo with colon", line: "TODO: Review the PR", wantOK: true, wantDone: false, wantText: "Review the PR"},
		{name: "todo without colon", line: "TODO fix the parser", wantOK: true, wantDone: false, wantText: "fix the parser"},
		{name: "done with colon", line: "DONE: Set up CI", wantOK: true, wantDone: true, wantText: "Set up CI"},
		{name: "done without colon", line: "DONE shipped it", wantOK: true, wantDone: true, wantText: "shipped it"},
		{name: "leading whitespace", line: "  TODO: indented task", wantOK: true, wantDone: false, wantText: "indented task"},
		{name: "bare todo", line: "TODO", wantOK: true, wantDone: false, wantText: "(untitled task)"},
		{name: "bare done", line: "DONE", wantOK: true, wantDone: true, wantText: "(untitled task)"},
		{name: "bare todo colon", line: "TODO:", wantOK: true, wantDone: false, wantText: "(untitled task)"},
		{name: "lowercase not matched", line: "todo: lowercase", wantOK: false},
		{name: "mid-line not matched", line: "some TODO: text", wantOK: false},
		{name: "checkbox not matched", line: "- [ ] task", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			done, text, ok := parseTodoKeywordLine(tt.line)
			if ok != tt.wantOK {
				t.Fatalf("ok=%v want=%v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if done != tt.wantDone {
				t.Fatalf("checked=%v want=%v", done, tt.wantDone)
			}
			if text != tt.wantText {
				t.Fatalf("text=%q want=%q", text, tt.wantText)
			}
		})
	}
}

func TestToggleTodoKeywordLine(t *testing.T) {
	tests := []struct {
		name     string
		note     string
		line     int
		wantLine string
		wantOK   bool
	}{
		{
			name:     "todo to done",
			note:     "TODO: Review PR",
			line:     0,
			wantLine: "DONE: Review PR",
			wantOK:   true,
		},
		{
			name:     "done to todo",
			note:     "DONE: Review PR",
			line:     0,
			wantLine: "TODO: Review PR",
			wantOK:   true,
		},
		{
			name:     "preserves indentation",
			note:     "  TODO: indented",
			line:     0,
			wantLine: "  DONE: indented",
			wantOK:   true,
		},
		{
			name:     "preserves no colon",
			note:     "TODO fix it",
			line:     0,
			wantLine: "DONE fix it",
			wantOK:   true,
		},
		{
			name:     "preserves surrounding lines",
			note:     "# Heading\nTODO: task\n- [ ] checkbox",
			line:     1,
			wantLine: "DONE: task",
			wantOK:   true,
		},
		{
			name:   "out of range",
			note:   "TODO: task",
			line:   5,
			wantOK: false,
		},
		{
			name:   "non-keyword line",
			note:   "plain text",
			line:   0,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := toggleTodoKeywordLine(tt.note, tt.line)
			if ok != tt.wantOK {
				t.Fatalf("ok=%v want=%v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			lines := strings.Split(result, "\n")
			if lines[tt.line] != tt.wantLine {
				t.Fatalf("got %q want %q", lines[tt.line], tt.wantLine)
			}
		})
	}
}

func TestExtractTaskRefsMixedItems(t *testing.T) {
	note := "TODO: Review the PR\n- [ ] Write tests\nDONE: Set up CI\n- [x] Merge PR\nTODO fix the parser"
	refs := extractTaskRefs("/tmp/wt", note)
	if len(refs) != 5 {
		t.Fatalf("expected 5 refs, got %d", len(refs))
	}

	// Verify order matches line order
	expected := []struct {
		text      string
		checked   bool
		isKeyword bool
	}{
		{"Review the PR", false, true},
		{"Write tests", false, false},
		{"Set up CI", true, true},
		{"Merge PR", true, false},
		{"fix the parser", false, true},
	}

	for i, exp := range expected {
		if refs[i].Text != exp.text {
			t.Fatalf("ref[%d] text=%q want=%q", i, refs[i].Text, exp.text)
		}
		if refs[i].Checked != exp.checked {
			t.Fatalf("ref[%d] checked=%v want=%v", i, refs[i].Checked, exp.checked)
		}
		if refs[i].IsKeyword != exp.isKeyword {
			t.Fatalf("ref[%d] isKeyword=%v want=%v", i, refs[i].IsKeyword, exp.isKeyword)
		}
	}
}

func TestShowTaskboardWithTodoLines(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	wtPath := "/tmp/wt-a"
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: wtPath, Branch: "feat-a"},
	}
	m.state.data.filteredWts = m.state.data.worktrees
	m.state.data.selectedIndex = 0
	m.setWorktreeNote(wtPath, "TODO: Review the PR\n- [ ] Write tests\nDONE: Set up CI")

	cmd := m.showTaskboard()
	if cmd != nil {
		t.Fatalf("expected nil command, got %v", cmd)
	}
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeTaskboard {
		t.Fatalf("expected taskboard screen, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}

	// Toggle the first item (TODO keyword) via space key
	updated, _ := m.handleScreenKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = updated.(*Model)

	note, ok := m.getWorktreeNote(wtPath)
	if !ok {
		t.Fatal("expected note to exist")
	}
	if !strings.Contains(note.Note, "DONE: Review the PR") {
		t.Fatalf("expected TODO toggled to DONE, got note:\n%s", note.Note)
	}
	// Verify checkbox line is unchanged
	if !strings.Contains(note.Note, "- [ ] Write tests") {
		t.Fatalf("expected checkbox line unchanged, got note:\n%s", note.Note)
	}
}

func TestHandleBuiltInKeyTaskboardShortcut(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	wtPath := "/tmp/wt-a"
	m.state.data.worktrees = []*models.WorktreeInfo{{Path: wtPath, Branch: "feat-a"}}
	m.setWorktreeNote(wtPath, "- [ ] Open taskboard")

	_, _ = m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'T'}})
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeTaskboard {
		t.Fatalf("expected taskboard screen on T shortcut, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}
}
