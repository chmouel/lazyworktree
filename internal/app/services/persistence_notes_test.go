package services

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/chmouel/lazyworktree/internal/models"
)

func TestLoadWorktreeNotesMissingFile(t *testing.T) {
	notes, err := LoadWorktreeNotes("repo", t.TempDir())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(notes) != 0 {
		t.Fatalf("expected empty notes map, got %d entries", len(notes))
	}
}

func TestSaveAndLoadWorktreeNotes(t *testing.T) {
	worktreeDir := t.TempDir()
	repoKey := "repo"
	expected := map[string]models.WorktreeNote{
		"/tmp/worktrees/feat": {
			Note:      "first line\nsecond line",
			UpdatedAt: 1234,
		},
	}

	if err := SaveWorktreeNotes(repoKey, worktreeDir, expected); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	got, err := LoadWorktreeNotes(repoKey, worktreeDir)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if !reflect.DeepEqual(expected, got) {
		t.Fatalf("notes mismatch:\nexpected=%#v\ngot=%#v", expected, got)
	}
}

func TestLoadWorktreeNotesInvalidJSON(t *testing.T) {
	worktreeDir := t.TempDir()
	repoKey := "repo"
	notesPath := filepath.Join(worktreeDir, repoKey, models.WorktreeNotesFilename)

	if err := os.MkdirAll(filepath.Dir(notesPath), 0o750); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(notesPath, []byte("{invalid"), 0o600); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	if _, err := LoadWorktreeNotes(repoKey, worktreeDir); err == nil {
		t.Fatal("expected JSON parsing error")
	}
}

func TestSaveWorktreeNotesRemovesFileWhenEmpty(t *testing.T) {
	worktreeDir := t.TempDir()
	repoKey := "repo"
	notesPath := filepath.Join(worktreeDir, repoKey, models.WorktreeNotesFilename)

	notes := map[string]models.WorktreeNote{
		"/tmp/worktrees/feat": {
			Note:      "keep",
			UpdatedAt: 1234,
		},
	}
	if err := SaveWorktreeNotes(repoKey, worktreeDir, notes); err != nil {
		t.Fatalf("initial save failed: %v", err)
	}
	if _, err := os.Stat(notesPath); err != nil {
		t.Fatalf("expected notes file to exist, stat failed: %v", err)
	}

	if err := SaveWorktreeNotes(repoKey, worktreeDir, map[string]models.WorktreeNote{}); err != nil {
		t.Fatalf("empty save failed: %v", err)
	}
	if _, err := os.Stat(notesPath); !os.IsNotExist(err) {
		t.Fatalf("expected notes file to be removed, got err=%v", err)
	}
}

func TestSaveWorktreeNotesSkipsWhitespaceOnlyNotes(t *testing.T) {
	worktreeDir := t.TempDir()
	repoKey := "repo"
	notesPath := filepath.Join(worktreeDir, repoKey, models.WorktreeNotesFilename)

	notes := map[string]models.WorktreeNote{
		"/tmp/worktrees/feat": {
			Note:      "   \n\t ",
			UpdatedAt: 1234,
		},
	}
	if err := SaveWorktreeNotes(repoKey, worktreeDir, notes); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	if _, err := os.Stat(notesPath); !os.IsNotExist(err) {
		t.Fatalf("expected no notes file for whitespace-only note, got err=%v", err)
	}
}

func TestSaveWorktreeNote(t *testing.T) {
	worktreeDir := t.TempDir()
	repoKey := "repo"
	wtPath := filepath.Join(worktreeDir, "repo", "feature")

	if err := SaveWorktreeNote(repoKey, worktreeDir, wtPath, "  generated note  "); err != nil {
		t.Fatalf("save note failed: %v", err)
	}

	notes, err := LoadWorktreeNotes(repoKey, worktreeDir)
	if err != nil {
		t.Fatalf("load notes failed: %v", err)
	}

	got, ok := notes[wtPath]
	if !ok {
		t.Fatalf("expected note for %q", wtPath)
	}
	if got.Note != "generated note" {
		t.Fatalf("unexpected note text: %q", got.Note)
	}
	if got.UpdatedAt == 0 {
		t.Fatal("expected UpdatedAt to be set")
	}
}

func TestSaveWorktreeNoteEmptyInputNoop(t *testing.T) {
	worktreeDir := t.TempDir()
	repoKey := "repo"
	notesPath := filepath.Join(worktreeDir, repoKey, models.WorktreeNotesFilename)

	if err := SaveWorktreeNote(repoKey, worktreeDir, " ", "some text"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := SaveWorktreeNote(repoKey, worktreeDir, "/tmp/wt", "   "); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(notesPath); !os.IsNotExist(err) {
		t.Fatalf("expected notes file to be absent, got err=%v", err)
	}
}
