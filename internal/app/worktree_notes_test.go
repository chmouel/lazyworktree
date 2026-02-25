package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

func TestSetAndLoadWorktreeNotes(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.repoKey = testRepoKey

	path := "/tmp/worktrees/feature-a"
	m.setWorktreeNote(path, "line one\nline two")

	m2 := NewModel(cfg, "")
	m2.repoKey = testRepoKey
	m2.loadWorktreeNotes()

	note, ok := m2.getWorktreeNote(path)
	if !ok {
		t.Fatal("expected note to be loaded")
	}
	if note.Note != "line one\nline two" {
		t.Fatalf("unexpected note text: %q", note.Note)
	}
}

func TestSetWorktreeNoteClearsEntryWhenEmpty(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.repoKey = testRepoKey

	path := "/tmp/worktrees/feature-a"
	m.setWorktreeNote(path, "keep me")
	m.setWorktreeNote(path, "   ")

	if _, ok := m.getWorktreeNote(path); ok {
		t.Fatal("expected note to be deleted")
	}
}

func TestMigrateWorktreeNote(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.repoKey = testRepoKey

	oldPath := "/tmp/worktrees/old-branch"
	newPath := "/tmp/worktrees/new-branch"
	m.setWorktreeNote(oldPath, "old note")
	m.migrateWorktreeNote(oldPath, newPath)

	if _, ok := m.getWorktreeNote(oldPath); ok {
		t.Fatal("expected old note to be removed")
	}
	note, ok := m.getWorktreeNote(newPath)
	if !ok {
		t.Fatal("expected note to be migrated")
	}
	if note.Note != "old note" {
		t.Fatalf("unexpected migrated note: %#v", note)
	}
}

func TestPruneStaleWorktreeNotes(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.repoKey = testRepoKey

	keepPath := "/tmp/worktrees/keep"
	dropPath := "/tmp/worktrees/drop"
	m.setWorktreeNote(keepPath, "keep")
	m.setWorktreeNote(dropPath, "drop")

	m.pruneStaleWorktreeNotes([]*models.WorktreeInfo{{Path: keepPath, Branch: "keep"}})

	if _, ok := m.getWorktreeNote(dropPath); ok {
		t.Fatal("expected stale note to be pruned")
	}
	if _, ok := m.getWorktreeNote(keepPath); !ok {
		t.Fatal("expected valid note to remain")
	}
}

func TestShowAnnotateWorktreeOpensTextareaWhenNoNote(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = 0
	m.state.data.filteredWts = []*models.WorktreeInfo{{Path: "/tmp/wt", Branch: "feat"}}
	m.state.data.selectedIndex = 0

	cmd := m.showAnnotateWorktree()
	if cmd == nil {
		t.Fatal("expected blink command")
	}
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeTextarea {
		t.Fatalf("expected textarea screen, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}
}

func TestShowAnnotateWorktreeOpensViewerWhenNoteExists(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	path := "/tmp/wt"
	m.state.view.FocusedPane = 0
	m.state.data.filteredWts = []*models.WorktreeInfo{{Path: path, Branch: "feat"}}
	m.state.data.selectedIndex = 0
	m.setWorktreeNote(path, "existing note")

	cmd := m.showAnnotateWorktree()
	if cmd != nil {
		t.Fatal("expected no blink command when opening viewer")
	}
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeNoteView {
		t.Fatalf("expected note viewer screen, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}
}

func TestHandleBuiltInKeyAnnotate(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	path := "/tmp/wt"
	m.state.data.filteredWts = []*models.WorktreeInfo{{Path: path, Branch: "feat"}}
	m.state.data.selectedIndex = 0

	m.state.view.FocusedPane = 1
	_, _ = m.handleBuiltInKey(tea.KeyPressMsg{Code: 'i', Text: string('i')})
	if m.state.ui.screenManager.IsActive() {
		t.Fatal("did not expect screen when pane is not worktree pane")
	}

	m.state.view.FocusedPane = 0
	_, _ = m.handleBuiltInKey(tea.KeyPressMsg{Code: 'i', Text: string('i')})
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeTextarea {
		t.Fatalf("expected textarea screen, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}

	m.state.ui.screenManager.Pop()
	m.setWorktreeNote(path, "existing note")
	_, _ = m.handleBuiltInKey(tea.KeyPressMsg{Code: 'i', Text: string('i')})
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeNoteView {
		t.Fatalf("expected note viewer screen, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}
}

func TestAnnotateWorktreeCtrlSSaves(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	path := "/tmp/wt"

	m.state.view.FocusedPane = 0
	m.state.data.filteredWts = []*models.WorktreeInfo{{Path: path, Branch: "feat"}}
	m.state.data.selectedIndex = 0

	_ = m.showAnnotateWorktree()
	scr := m.state.ui.screenManager.Current().(*appscreen.TextareaScreen)
	scr.Input.SetValue("one line\ntwo line")

	updated, _ := m.handleScreenKey(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	m = updated.(*Model)

	if m.state.ui.screenManager.IsActive() {
		t.Fatal("expected annotate modal to close on save")
	}
	note, ok := m.getWorktreeNote(path)
	if !ok {
		t.Fatal("expected note to be saved")
	}
	if note.Note != "one line\ntwo line" {
		t.Fatalf("unexpected saved note: %q", note.Note)
	}
}

func TestAnnotateWorktreeViewerEOpensEditor(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	path := "/tmp/wt"
	m.state.view.FocusedPane = 0
	m.state.data.filteredWts = []*models.WorktreeInfo{{Path: path, Branch: "feat"}}
	m.state.data.selectedIndex = 0
	m.setWorktreeNote(path, "one line\ntwo line")

	_ = m.showAnnotateWorktree()
	if m.state.ui.screenManager.Type() != appscreen.TypeNoteView {
		t.Fatalf("expected note viewer, got %v", m.state.ui.screenManager.Type())
	}

	updated, cmd := m.handleScreenKey(tea.KeyPressMsg{Code: 'e', Text: string('e')})
	m = updated.(*Model)

	if m.state.ui.screenManager.IsActive() {
		t.Fatal("expected viewer to close before processing edit message")
	}
	if cmd == nil {
		t.Fatal("expected edit command")
	}

	msg := cmd()
	updated, _ = m.Update(msg)
	m = updated.(*Model)

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeTextarea {
		t.Fatalf("expected textarea screen after edit, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}

	scr := m.state.ui.screenManager.Current().(*appscreen.TextareaScreen)
	if scr.Input.Value() != "one line\ntwo line" {
		t.Fatalf("expected textarea to be prefilled with note, got %q", scr.Input.Value())
	}
}

func TestUpdateRenameWorktreeResultMigratesNote(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	oldPath := "/tmp/wt-old"
	newPath := "/tmp/wt-new"

	m.state.data.worktrees = []*models.WorktreeInfo{{Path: oldPath, Branch: "old"}}
	m.setWorktreeNote(oldPath, "rename me")

	updated, _ := m.Update(renameWorktreeResultMsg{
		oldPath: oldPath,
		newPath: newPath,
		worktrees: []*models.WorktreeInfo{
			{Path: newPath, Branch: "new"},
		},
	})
	m = updated.(*Model)

	if _, ok := m.getWorktreeNote(oldPath); ok {
		t.Fatal("expected old path note to be removed")
	}
	if _, ok := m.getWorktreeNote(newPath); !ok {
		t.Fatal("expected note to move to new path")
	}
}

func TestUpdateTableHidesNoteIconForEmptyNote(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		IconSet:     "text",
	}
	m := NewModel(cfg, "")
	m.repoKey = testRepoKey
	wtPath := filepath.Join(cfg.WorktreeDir, "empty-note")
	notesPath := filepath.Join(cfg.WorktreeDir, testRepoKey, models.WorktreeNotesFilename)
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: wtPath, Branch: "feat"},
	}
	m.state.data.filteredWts = m.state.data.worktrees
	m.state.data.selectedIndex = 0

	m.setWorktreeNote(wtPath, "non-empty")
	if _, err := os.Stat(notesPath); err != nil {
		t.Fatalf("expected notes file to exist before clearing note, got %v", err)
	}

	m.setWorktreeNote(wtPath, "   ")
	m.updateTable()

	rows := m.state.ui.worktreeTable.Rows()
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if strings.Contains(rows[0][0], "[N]") {
		t.Fatalf("expected no note icon for empty note, got %q", rows[0][0])
	}
	if _, err := os.Stat(notesPath); !os.IsNotExist(err) {
		t.Fatalf("expected notes file to be removed when all notes are empty, got err=%v", err)
	}
}

func TestSetAndLoadWorktreeNotesSharedFileUsesRelativeKeys(t *testing.T) {
	worktreeDir := t.TempDir()
	sharedPath := filepath.Join(t.TempDir(), "shared-notes.json")
	cfg := &config.AppConfig{
		WorktreeDir:       worktreeDir,
		WorktreeNotesPath: sharedPath,
	}
	repoKey := "org/repo"

	m := NewModel(cfg, "")
	m.repoKey = repoKey
	wtPath := filepath.Join(worktreeDir, "org", "repo", "feature-a")
	m.setWorktreeNote(wtPath, "shared note")

	// #nosec G304 -- sharedPath is a test temp file controlled by the test.
	data, err := os.ReadFile(sharedPath)
	if err != nil {
		t.Fatalf("read shared notes failed: %v", err)
	}

	var payload map[string]map[string]models.WorktreeNote
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal shared notes failed: %v", err)
	}

	repoNotes, ok := payload[repoKey]
	if !ok {
		t.Fatalf("expected repo section %q in shared file", repoKey)
	}
	if _, ok := repoNotes[wtPath]; ok {
		t.Fatalf("did not expect absolute key %q in shared file", wtPath)
	}
	if note := repoNotes["feature-a"]; note.Note != "shared note" {
		t.Fatalf("unexpected shared note payload: %#v", repoNotes)
	}

	m2 := NewModel(cfg, "")
	m2.repoKey = repoKey
	m2.loadWorktreeNotes()
	note, ok := m2.getWorktreeNote(wtPath)
	if !ok {
		t.Fatal("expected note to load from shared file")
	}
	if note.Note != "shared note" {
		t.Fatalf("unexpected note text: %q", note.Note)
	}
}
