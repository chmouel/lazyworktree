package services

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chmouel/lazyworktree/internal/models"
)

// TestSessionRegistryStoreReusesCacheWhenUnchanged proves Load serves from the
// in-process cache (no disk read / unmarshal) while the file is unchanged: the
// file is made unreadable, so a Load that touched disk would error.
func TestSessionRegistryStoreReusesCacheWhenUnchanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")
	store := NewTestSessionRegistryStore(path)
	sessions := []*models.AgentSession{{
		ID:         "session-a",
		SessionKey: "claude:/tmp/wt/a.jsonl",
		Agent:      models.AgentKindClaude,
		JSONLPath:  "/tmp/wt/a.jsonl",
		Title:      "first",
	}}
	if err := store.Save(sessions); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := os.Chmod(path, 0); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load after Save should hit cache without reading disk, got error: %v", err)
	}
	if got := loaded["claude:/tmp/wt/a.jsonl"]; got == nil || got.Title != "first" {
		t.Fatalf("expected cached session, got %#v", loaded)
	}
}

// TestSessionRegistryStoreReloadsWhenFileChanges ensures an external write
// (different size) invalidates the cache so Load picks up fresh data.
func TestSessionRegistryStoreReloadsWhenFileChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")
	store := NewTestSessionRegistryStore(path)
	if err := store.Save([]*models.AgentSession{{
		ID: "a", SessionKey: "claude:/tmp/wt/a.jsonl", Agent: models.AgentKindClaude, JSONLPath: "/tmp/wt/a.jsonl", Title: "first",
	}}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := store.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	other := NewTestSessionRegistryStore(path)
	if err := other.Save([]*models.AgentSession{
		{ID: "a", SessionKey: "claude:/tmp/wt/a.jsonl", Agent: models.AgentKindClaude, JSONLPath: "/tmp/wt/a.jsonl", Title: "first"},
		{ID: "b", SessionKey: "claude:/tmp/wt/b.jsonl", Agent: models.AgentKindClaude, JSONLPath: "/tmp/wt/b.jsonl", Title: "second"},
	}); err != nil {
		t.Fatalf("other Save: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected reload to pick up 2 sessions, got %d", len(loaded))
	}
}
