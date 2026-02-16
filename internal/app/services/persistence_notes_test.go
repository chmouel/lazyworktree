package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/chmouel/lazyworktree/internal/models"
)

func TestLoadWorktreeNotesMissingFile(t *testing.T) {
	notes, err := LoadWorktreeNotes("repo", t.TempDir(), "")
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

	if err := SaveWorktreeNotes(repoKey, worktreeDir, "", expected); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	got, err := LoadWorktreeNotes(repoKey, worktreeDir, "")
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

	if _, err := LoadWorktreeNotes(repoKey, worktreeDir, ""); err == nil {
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
	if err := SaveWorktreeNotes(repoKey, worktreeDir, "", notes); err != nil {
		t.Fatalf("initial save failed: %v", err)
	}
	if _, err := os.Stat(notesPath); err != nil {
		t.Fatalf("expected notes file to exist, stat failed: %v", err)
	}

	if err := SaveWorktreeNotes(repoKey, worktreeDir, "", map[string]models.WorktreeNote{}); err != nil {
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
	if err := SaveWorktreeNotes(repoKey, worktreeDir, "", notes); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	if _, err := os.Stat(notesPath); !os.IsNotExist(err) {
		t.Fatalf("expected no notes file for whitespace-only note, got err=%v", err)
	}
}

func TestSaveAndLoadWorktreeNotesSharedFile(t *testing.T) {
	worktreeDir := t.TempDir()
	sharedPath := filepath.Join(t.TempDir(), "notes.json")

	repo1Notes := map[string]models.WorktreeNote{
		"feature-a": {Note: "repo1"},
	}
	repo2Notes := map[string]models.WorktreeNote{
		"feature-b": {Note: "repo2"},
	}

	if err := SaveWorktreeNotes("org/repo1", worktreeDir, sharedPath, repo1Notes); err != nil {
		t.Fatalf("save repo1 failed: %v", err)
	}
	if err := SaveWorktreeNotes("org/repo2", worktreeDir, sharedPath, repo2Notes); err != nil {
		t.Fatalf("save repo2 failed: %v", err)
	}

	got1, err := LoadWorktreeNotes("org/repo1", worktreeDir, sharedPath)
	if err != nil {
		t.Fatalf("load repo1 failed: %v", err)
	}
	if !reflect.DeepEqual(repo1Notes, got1) {
		t.Fatalf("repo1 notes mismatch:\nexpected=%#v\ngot=%#v", repo1Notes, got1)
	}

	got2, err := LoadWorktreeNotes("org/repo2", worktreeDir, sharedPath)
	if err != nil {
		t.Fatalf("load repo2 failed: %v", err)
	}
	if !reflect.DeepEqual(repo2Notes, got2) {
		t.Fatalf("repo2 notes mismatch:\nexpected=%#v\ngot=%#v", repo2Notes, got2)
	}
}

func TestSaveWorktreeNotesSharedFileRemovesOnlyOneRepoSection(t *testing.T) {
	worktreeDir := t.TempDir()
	sharedPath := filepath.Join(t.TempDir(), "notes.json")

	if err := SaveWorktreeNotes("org/repo1", worktreeDir, sharedPath, map[string]models.WorktreeNote{
		"feature-a": {Note: "a"},
	}); err != nil {
		t.Fatalf("save repo1 failed: %v", err)
	}
	if err := SaveWorktreeNotes("org/repo2", worktreeDir, sharedPath, map[string]models.WorktreeNote{
		"feature-b": {Note: "b"},
	}); err != nil {
		t.Fatalf("save repo2 failed: %v", err)
	}
	if err := SaveWorktreeNotes("org/repo1", worktreeDir, sharedPath, map[string]models.WorktreeNote{}); err != nil {
		t.Fatalf("clear repo1 failed: %v", err)
	}

	got2, err := LoadWorktreeNotes("org/repo2", worktreeDir, sharedPath)
	if err != nil {
		t.Fatalf("load repo2 failed: %v", err)
	}
	if len(got2) != 1 || got2["feature-b"].Note != "b" {
		t.Fatalf("expected repo2 note to remain, got %#v", got2)
	}

	// Verify underlying JSON still contains repo2 only.
	// #nosec G304 -- sharedPath is a test temp file controlled by the test.
	data, err := os.ReadFile(sharedPath)
	if err != nil {
		t.Fatalf("read shared file failed: %v", err)
	}
	var payload map[string]map[string]models.WorktreeNote
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal shared file failed: %v", err)
	}
	if _, ok := payload["org/repo1"]; ok {
		t.Fatalf("expected repo1 section removed, got %#v", payload)
	}
	if _, ok := payload["org/repo2"]; !ok {
		t.Fatalf("expected repo2 section to exist, got %#v", payload)
	}
}

func TestSaveWorktreeNote(t *testing.T) {
	worktreeDir := t.TempDir()
	repoKey := "repo"
	wtPath := filepath.Join(worktreeDir, "repo", "feature")

	if err := SaveWorktreeNote(repoKey, worktreeDir, "", wtPath, "  generated note  "); err != nil {
		t.Fatalf("save note failed: %v", err)
	}

	notes, err := LoadWorktreeNotes(repoKey, worktreeDir, "")
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

func TestSaveWorktreeNoteSharedPathUsesRelativeKey(t *testing.T) {
	worktreeDir := t.TempDir()
	repoKey := "org/repo"
	sharedPath := filepath.Join(t.TempDir(), "notes.json")
	wtPath := filepath.Join(worktreeDir, "org", "repo", "feature")

	if err := SaveWorktreeNote(repoKey, worktreeDir, sharedPath, wtPath, "  generated note  "); err != nil {
		t.Fatalf("save note failed: %v", err)
	}

	notes, err := LoadWorktreeNotes(repoKey, worktreeDir, sharedPath)
	if err != nil {
		t.Fatalf("load notes failed: %v", err)
	}

	got, ok := notes["feature"]
	if !ok {
		t.Fatalf("expected key %q, got %#v", "feature", notes)
	}
	if got.Note != "generated note" {
		t.Fatalf("unexpected note text: %q", got.Note)
	}
}

func TestWorktreeNoteKeySharedPath(t *testing.T) {
	worktreeDir := t.TempDir()
	repoKey := "org/repo"
	sharedPath := filepath.Join(t.TempDir(), "notes.json")

	wtPath := filepath.Join(worktreeDir, "org", "repo", "feature-a")
	key := WorktreeNoteKey(repoKey, worktreeDir, sharedPath, wtPath)
	if key != "feature-a" {
		t.Fatalf("expected relative key, got %q", key)
	}

	mainPath := filepath.Join(t.TempDir(), "repo")
	mainKey := WorktreeNoteKey(repoKey, worktreeDir, sharedPath, mainPath)
	if mainKey != "repo" {
		t.Fatalf("expected basename key for out-of-tree path, got %q", mainKey)
	}
}

func TestSaveWorktreeNoteEmptyInputNoop(t *testing.T) {
	worktreeDir := t.TempDir()
	repoKey := "repo"
	notesPath := filepath.Join(worktreeDir, repoKey, models.WorktreeNotesFilename)

	if err := SaveWorktreeNote(repoKey, worktreeDir, "", " ", "some text"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := SaveWorktreeNote(repoKey, worktreeDir, "", "/tmp/wt", "   "); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(notesPath); !os.IsNotExist(err) {
		t.Fatalf("expected notes file to be absent, got err=%v", err)
	}
}
