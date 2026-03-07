package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chmouel/lazyworktree/internal/models"
)

func TestLoadWorktreeNotesMissingFile(t *testing.T) {
	notes, err := LoadWorktreeNotes("repo", t.TempDir(), "", "", nil)
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

	if err := SaveWorktreeNotes(repoKey, worktreeDir, "", "", expected, nil); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	got, err := LoadWorktreeNotes(repoKey, worktreeDir, "", "", nil)
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

	if _, err := LoadWorktreeNotes(repoKey, worktreeDir, "", "", nil); err == nil {
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
	if err := SaveWorktreeNotes(repoKey, worktreeDir, "", "", notes, nil); err != nil {
		t.Fatalf("initial save failed: %v", err)
	}
	if _, err := os.Stat(notesPath); err != nil {
		t.Fatalf("expected notes file to exist, stat failed: %v", err)
	}

	if err := SaveWorktreeNotes(repoKey, worktreeDir, "", "", map[string]models.WorktreeNote{}, nil); err != nil {
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
	if err := SaveWorktreeNotes(repoKey, worktreeDir, "", "", notes, nil); err != nil {
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

	if err := SaveWorktreeNotes("org/repo1", worktreeDir, sharedPath, "", repo1Notes, nil); err != nil {
		t.Fatalf("save repo1 failed: %v", err)
	}
	if err := SaveWorktreeNotes("org/repo2", worktreeDir, sharedPath, "", repo2Notes, nil); err != nil {
		t.Fatalf("save repo2 failed: %v", err)
	}

	got1, err := LoadWorktreeNotes("org/repo1", worktreeDir, sharedPath, "", nil)
	if err != nil {
		t.Fatalf("load repo1 failed: %v", err)
	}
	if !reflect.DeepEqual(repo1Notes, got1) {
		t.Fatalf("repo1 notes mismatch:\nexpected=%#v\ngot=%#v", repo1Notes, got1)
	}

	got2, err := LoadWorktreeNotes("org/repo2", worktreeDir, sharedPath, "", nil)
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

	if err := SaveWorktreeNotes("org/repo1", worktreeDir, sharedPath, "", map[string]models.WorktreeNote{
		"feature-a": {Note: "a"},
	}, nil); err != nil {
		t.Fatalf("save repo1 failed: %v", err)
	}
	if err := SaveWorktreeNotes("org/repo2", worktreeDir, sharedPath, "", map[string]models.WorktreeNote{
		"feature-b": {Note: "b"},
	}, nil); err != nil {
		t.Fatalf("save repo2 failed: %v", err)
	}
	if err := SaveWorktreeNotes("org/repo1", worktreeDir, sharedPath, "", map[string]models.WorktreeNote{}, nil); err != nil {
		t.Fatalf("clear repo1 failed: %v", err)
	}

	got2, err := LoadWorktreeNotes("org/repo2", worktreeDir, sharedPath, "", nil)
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

	if err := SaveWorktreeNote(repoKey, worktreeDir, "", "", wtPath, "  generated note  ", nil); err != nil {
		t.Fatalf("save note failed: %v", err)
	}

	notes, err := LoadWorktreeNotes(repoKey, worktreeDir, "", "", nil)
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

	if err := SaveWorktreeNote(repoKey, worktreeDir, sharedPath, "", wtPath, "  generated note  ", nil); err != nil {
		t.Fatalf("save note failed: %v", err)
	}

	notes, err := LoadWorktreeNotes(repoKey, worktreeDir, sharedPath, "", nil)
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

	if err := SaveWorktreeNote(repoKey, worktreeDir, "", "", " ", "some text", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := SaveWorktreeNote(repoKey, worktreeDir, "", "", "/tmp/wt", "   ", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(notesPath); !os.IsNotExist(err) {
		t.Fatalf("expected notes file to be absent, got err=%v", err)
	}
}

func TestMigrateRepoNotesToSharedFile(t *testing.T) {
	worktreeDir := t.TempDir()
	repoKey := "org/repo"
	sharedPath := filepath.Join(t.TempDir(), "shared-notes.json")

	// Create per-repo notes file with absolute-path keys.
	wtPath := filepath.Join(worktreeDir, "org", "repo", "feature-x")
	oldNotes := map[string]models.WorktreeNote{
		wtPath: {Note: "migrate me", UpdatedAt: 100},
	}
	if err := SaveWorktreeNotes(repoKey, worktreeDir, "", "", oldNotes, nil); err != nil {
		t.Fatalf("save per-repo notes failed: %v", err)
	}

	n, err := MigrateRepoNotesToSharedFile(repoKey, worktreeDir, sharedPath)
	if err != nil {
		t.Fatalf("migration failed: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 migrated note, got %d", n)
	}

	// Per-repo file should be deleted.
	repoNotesPath := filepath.Join(worktreeDir, repoKey, models.WorktreeNotesFilename)
	if _, err := os.Stat(repoNotesPath); !os.IsNotExist(err) {
		t.Fatalf("expected per-repo file removed, got err=%v", err)
	}

	// Note should be readable from shared file with relative key.
	notes, err := LoadWorktreeNotes(repoKey, worktreeDir, sharedPath, "", nil)
	if err != nil {
		t.Fatalf("load shared notes failed: %v", err)
	}
	got, ok := notes["feature-x"]
	if !ok {
		t.Fatalf("expected key %q in shared notes, got %#v", "feature-x", notes)
	}
	if got.Note != "migrate me" {
		t.Fatalf("unexpected note text: %q", got.Note)
	}
}

func TestMigrateRepoNotesToSharedFileNoOp(t *testing.T) {
	worktreeDir := t.TempDir()
	repoKey := "org/repo"
	sharedPath := filepath.Join(t.TempDir(), "shared-notes.json")

	// No per-repo file exists.
	n, err := MigrateRepoNotesToSharedFile(repoKey, worktreeDir, sharedPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 migrated notes, got %d", n)
	}
}

func TestMigrateRepoNotesToSharedFileConflict(t *testing.T) {
	worktreeDir := t.TempDir()
	repoKey := "org/repo"
	sharedPath := filepath.Join(t.TempDir(), "shared-notes.json")

	wtPath := filepath.Join(worktreeDir, "org", "repo", "feature-y")

	// Write a newer note to the shared file first.
	sharedNotes := map[string]models.WorktreeNote{
		"feature-y": {Note: "newer shared note", UpdatedAt: 500},
	}
	if err := SaveWorktreeNotes(repoKey, worktreeDir, sharedPath, "", sharedNotes, nil); err != nil {
		t.Fatalf("save shared notes failed: %v", err)
	}

	// Create per-repo notes with an older timestamp.
	oldNotes := map[string]models.WorktreeNote{
		wtPath: {Note: "older repo note", UpdatedAt: 100},
	}
	if err := SaveWorktreeNotes(repoKey, worktreeDir, "", "", oldNotes, nil); err != nil {
		t.Fatalf("save per-repo notes failed: %v", err)
	}

	n, err := MigrateRepoNotesToSharedFile(repoKey, worktreeDir, sharedPath)
	if err != nil {
		t.Fatalf("migration failed: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 migrated (conflict), got %d", n)
	}

	// Shared file should still have the newer note.
	notes, err := LoadWorktreeNotes(repoKey, worktreeDir, sharedPath, "", nil)
	if err != nil {
		t.Fatalf("load shared notes failed: %v", err)
	}
	got := notes["feature-y"]
	if got.Note != "newer shared note" {
		t.Fatalf("expected newer note preserved, got %q", got.Note)
	}
	if got.UpdatedAt != 500 {
		t.Fatalf("expected UpdatedAt=500, got %d", got.UpdatedAt)
	}

	// Per-repo file should still be cleaned up.
	repoNotesPath := filepath.Join(worktreeDir, repoKey, models.WorktreeNotesFilename)
	if _, err := os.Stat(repoNotesPath); !os.IsNotExist(err) {
		t.Fatalf("expected per-repo file removed, got err=%v", err)
	}
}

func TestMigrateRepoNotesToSharedFileNoSharedPath(t *testing.T) {
	worktreeDir := t.TempDir()
	repoKey := "org/repo"

	n, err := MigrateRepoNotesToSharedFile(repoKey, worktreeDir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 migrated notes, got %d", n)
	}
}

func TestSplittedWorktreeNotesRoundTrip(t *testing.T) {
	baseDir := t.TempDir()
	pathTemplate := filepath.Join(baseDir, "$REPO_OWNER", "$REPO_REPONAME", "$WORKTREE_NAME", "note.md")
	env := map[string]string{
		"REPO_OWNER":    "myorg",
		"REPO_REPONAME": "myrepo",
	}

	notes := map[string]models.WorktreeNote{
		"feature-a": {Note: "hello world", Icon: "⚡", UpdatedAt: 1709740800},
		"feature-b": {Note: "another note", UpdatedAt: 1709740900},
	}

	if err := SaveWorktreeNotes("myorg/myrepo", "", pathTemplate, "splitted", notes, env); err != nil {
		t.Fatalf("save splitted failed: %v", err)
	}

	// Verify files exist on disk.
	noteFileA := filepath.Join(baseDir, "myorg", "myrepo", "feature-a", "note.md")
	if _, err := os.Stat(noteFileA); err != nil {
		t.Fatalf("expected note file at %s: %v", noteFileA, err)
	}

	got, err := LoadWorktreeNotes("myorg/myrepo", "", pathTemplate, "splitted", env)
	if err != nil {
		t.Fatalf("load splitted failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 notes, got %d: %#v", len(got), got)
	}
	if got["feature-a"].Icon != "⚡" {
		t.Fatalf("expected icon ⚡, got %q", got["feature-a"].Icon)
	}
	if got["feature-b"].Note != "another note" {
		t.Fatalf("unexpected note text: %q", got["feature-b"].Note)
	}
}

func TestSplittedWorktreeNotesRetainsColorOnly(t *testing.T) {
	baseDir := t.TempDir()
	pathTemplate := filepath.Join(baseDir, "$WORKTREE_NAME", "note.md")
	env := map[string]string{}

	notes := map[string]models.WorktreeNote{
		"color-only": {Color: "red", UpdatedAt: 1709740800},
	}
	if err := SaveWorktreeNotes("repo", "", pathTemplate, "splitted", notes, env); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	got, err := LoadWorktreeNotes("repo", "", pathTemplate, "splitted", env)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 note, got %d", len(got))
	}
	if got["color-only"].Color != "red" {
		t.Fatalf("expected color red, got %q", got["color-only"].Color)
	}
}

func TestSplittedWorktreeNotesDeletesEmptyNote(t *testing.T) {
	baseDir := t.TempDir()
	pathTemplate := filepath.Join(baseDir, "$WORKTREE_NAME", "note.md")
	env := map[string]string{}

	notes := map[string]models.WorktreeNote{
		"feat": {Note: "keep me", UpdatedAt: 100},
	}
	if err := SaveWorktreeNotes("repo", "", pathTemplate, "splitted", notes, env); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	noteFile := filepath.Join(baseDir, "feat", "note.md")
	if _, err := os.Stat(noteFile); err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}

	// Delete the note file.
	if err := DeleteSplittedNoteFile(pathTemplate, "feat", env); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if _, err := os.Stat(noteFile); !os.IsNotExist(err) {
		t.Fatalf("expected file removed, got err=%v", err)
	}
}

func TestSplittedWorktreeNotesSavingEmptyNotesSkips(t *testing.T) {
	baseDir := t.TempDir()
	pathTemplate := filepath.Join(baseDir, "$WORKTREE_NAME", "note.md")
	env := map[string]string{}

	notes := map[string]models.WorktreeNote{
		"feat": {Note: "   ", Icon: ""},
	}
	if err := SaveWorktreeNotes("repo", "", pathTemplate, "splitted", notes, env); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	noteFile := filepath.Join(baseDir, "feat", "note.md")
	if _, err := os.Stat(noteFile); !os.IsNotExist(err) {
		t.Fatalf("expected no file for whitespace-only note, got err=%v", err)
	}
}

func TestSplittedWorktreeNotesSaveRemovesStaleFiles(t *testing.T) {
	baseDir := t.TempDir()
	pathTemplate := filepath.Join(baseDir, "$WORKTREE_NAME", "note.md")
	env := map[string]string{}

	require.NoError(t, SaveWorktreeNotes("repo", "", pathTemplate, "splitted", map[string]models.WorktreeNote{
		"feat-a": {Note: "keep", UpdatedAt: 1},
		"feat-b": {Note: "remove", UpdatedAt: 1},
	}, env))

	require.NoError(t, SaveWorktreeNotes("repo", "", pathTemplate, "splitted", map[string]models.WorktreeNote{
		"feat-a": {Note: "keep", UpdatedAt: 2},
	}, env))

	if _, err := os.Stat(filepath.Join(baseDir, "feat-a", "note.md")); err != nil {
		t.Fatalf("expected feat-a note to remain: %v", err)
	}
	if _, err := os.Stat(filepath.Join(baseDir, "feat-b", "note.md")); !os.IsNotExist(err) {
		t.Fatalf("expected feat-b note removed, got err=%v", err)
	}
}

func TestSplitRepoKey(t *testing.T) {
	tests := []struct {
		input     string
		wantOwner string
		wantRepo  string
	}{
		{"owner/repo", "owner", "repo"},
		{"org/sub/repo", "org", "sub/repo"},
		{"local-abc123", "", "local-abc123"},
		{"", "", ""},
	}
	for _, tt := range tests {
		owner, repo := SplitRepoKey(tt.input)
		if owner != tt.wantOwner || repo != tt.wantRepo {
			t.Errorf("SplitRepoKey(%q) = (%q, %q), want (%q, %q)", tt.input, owner, repo, tt.wantOwner, tt.wantRepo)
		}
	}
}

func TestExtractWorktreeNameFromParts(t *testing.T) {
	env := map[string]string{
		"REPO_OWNER":    "org",
		"REPO_REPONAME": "repo",
	}
	tmpl := "/notes/$REPO_OWNER/$REPO_REPONAME/$WORKTREE_NAME/note.md"
	expanded := ExpandWithEnv(tmpl, cloneEnvWith(env, "WORKTREE_NAME", worktreeNameSentinel))
	parts := strings.SplitN(expanded, worktreeNameSentinel, 2)
	require.Len(t, parts, 2, "sentinel should split template into prefix and suffix")

	name := extractWorktreeNameFromParts("/notes/org/repo/feature-x/note.md", parts[0], parts[1])
	if name != "feature-x" {
		t.Fatalf("expected feature-x, got %q", name)
	}
}
