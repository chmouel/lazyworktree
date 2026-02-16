package services

import (
	"context"
	"strings"
	"testing"
)

func TestRunWorktreeNoteScriptSuccess(t *testing.T) {
	t.Parallel()

	note, err := RunWorktreeNoteScript(context.Background(), "cat", WorktreeNoteScriptInput{
		Content: "line one\nline two\n",
		Type:    "issue",
		Number:  42,
		Title:   "Issue title",
		URL:     "https://example.com/issues/42",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if note != "line one\nline two" {
		t.Fatalf("unexpected note: %q", note)
	}
}

func TestRunWorktreeNoteScriptEnv(t *testing.T) {
	t.Parallel()

	script := `printf "%s|%s|%s|%s" "$LAZYWORKTREE_TYPE" "$LAZYWORKTREE_NUMBER" "$LAZYWORKTREE_TITLE" "$LAZYWORKTREE_URL"`
	note, err := RunWorktreeNoteScript(context.Background(), script, WorktreeNoteScriptInput{
		Type:   "pr",
		Number: 123,
		Title:  "Fix bug",
		URL:    "https://example.com/pr/123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if note != "pr|123|Fix bug|https://example.com/pr/123" {
		t.Fatalf("unexpected env output: %q", note)
	}
}

func TestRunWorktreeNoteScriptFailure(t *testing.T) {
	t.Parallel()

	_, err := RunWorktreeNoteScript(context.Background(), "echo boom >&2; exit 1", WorktreeNoteScriptInput{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "worktree note script failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunWorktreeNoteScriptEmpty(t *testing.T) {
	t.Parallel()

	note, err := RunWorktreeNoteScript(context.Background(), "printf '   '", WorktreeNoteScriptInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if note != "" {
		t.Fatalf("expected empty note, got %q", note)
	}
}
