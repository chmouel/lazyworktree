package services

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chmouel/lazyworktree/internal/models"
)

func TestSessionRegistryStoreRoundTrip(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "registry.json")
	store := NewTestSessionRegistryStore(path)
	now := time.Now().UTC().Round(time.Second)
	sessions := []*models.AgentSession{{
		ID:             "session-a",
		SessionKey:     "claude:/tmp/worktree/session.jsonl",
		Agent:          models.AgentKindClaude,
		JSONLPath:      "/tmp/worktree/session.jsonl",
		Title:          "editing internal/app/app_agents.go",
		LastActivity:   now,
		LastObservedAt: now,
		LivenessState:  models.AgentSessionLivenessRecent,
		LivenessSource: models.AgentSessionLivenessSourceRegistry,
	}}

	if err := store.Save(sessions); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	got, ok := loaded["claude:/tmp/worktree/session.jsonl"]
	if !ok || got == nil {
		t.Fatalf("expected stored session to be present, got %#v", loaded)
	}
	if got.Title != sessions[0].Title {
		t.Fatalf("expected title %q, got %q", sessions[0].Title, got.Title)
	}
	if got.LivenessState != models.AgentSessionLivenessRecent {
		t.Fatalf("expected recent liveness, got %q", got.LivenessState)
	}
}

func TestAgentSessionServiceFallsBackToRegistryWhenParsingFails(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	claudeRoot := filepath.Join(root, "claude")
	sessionPath := filepath.Join(claudeRoot, "project-a", "session-1.jsonl")
	worktreePath := filepath.Join(root, "worktrees", "feature")
	now := time.Now().UTC()

	writeJSONLLines(
		t, sessionPath,
		mustJSONLine(t, map[string]any{
			"type":      "assistant",
			"cwd":       worktreePath,
			"timestamp": now.Format(time.RFC3339Nano),
			"message": map[string]any{
				"role":  "assistant",
				"model": "claude-sonnet-4",
				"content": []map[string]any{
					{"type": "text", "text": "Done"},
				},
			},
		}),
	)

	service := NewAgentSessionServiceWithStore(
		claudeRoot,
		"",
		NewTestSessionRegistryStore(filepath.Join(root, "registry.json")),
		nil,
	)
	first, err := service.Refresh()
	if err != nil {
		t.Fatalf("first Refresh returned error: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("expected one parsed session, got %#v", first)
	}

	later := now.Add(2 * time.Second)
	if err := os.WriteFile(sessionPath, []byte("{not-json}\n"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := os.Chtimes(sessionPath, later, later); err != nil {
		t.Fatalf("Chtimes returned error: %v", err)
	}

	second, err := service.Refresh()
	if err != nil {
		t.Fatalf("second Refresh returned error: %v", err)
	}
	if len(second) != 1 {
		t.Fatalf("expected registry fallback session, got %#v", second)
	}
	if second[0].JSONLPath != sessionPath {
		t.Fatalf("expected fallback to preserve JSONL path %q, got %q", sessionPath, second[0].JSONLPath)
	}
	if second[0].LivenessState != models.AgentSessionLivenessRecent {
		t.Fatalf("expected fallback session to stay recent, got %q", second[0].LivenessState)
	}
	if second[0].LivenessSource != models.AgentSessionLivenessSourceRegistry {
		t.Fatalf("expected registry fallback source, got %q", second[0].LivenessSource)
	}
}
