package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chmouel/lazyworktree/internal/models"
)

func TestParseClaudeSession(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	worktreePath := filepath.Join(root, "worktrees", "feature")
	ts := time.Now().UTC().Format(time.RFC3339Nano)

	path := filepath.Join(root, "claude.jsonl")
	writeJSONLLines(t, path,
		mustJSONLine(t, map[string]any{
			"type":      "user",
			"cwd":       worktreePath,
			"timestamp": ts,
			"message": map[string]any{
				"role":    "user",
				"content": "Polish the agent sessions pane",
			},
		}),
		mustJSONLine(t, map[string]any{
			"type":      "assistant",
			"cwd":       worktreePath,
			"gitBranch": "feature/agent-pane",
			"timestamp": ts,
			"message": map[string]any{
				"role":  "assistant",
				"model": "claude-sonnet-4",
				"content": []map[string]any{
					{"type": "tool_use", "name": "Read", "input": map[string]any{"file_path": filepath.Join(worktreePath, "internal", "app", "app_agents.go")}},
				},
			},
		}),
	)

	session, err := parseClaudeSession(path, "")
	if err != nil {
		t.Fatalf("parseClaudeSession returned error: %v", err)
	}

	if session.Agent != models.AgentKindClaude {
		t.Fatalf("expected Claude agent, got %q", session.Agent)
	}
	if session.CWD != worktreePath {
		t.Fatalf("expected cwd %q, got %q", worktreePath, session.CWD)
	}
	if session.GitBranch != "feature/agent-pane" {
		t.Fatalf("expected git branch to be parsed, got %q", session.GitBranch)
	}
	if session.Model != "claude-sonnet-4" {
		t.Fatalf("expected model to be parsed, got %q", session.Model)
	}
	if session.CurrentTool != "Read" {
		t.Fatalf("expected current tool Read, got %q", session.CurrentTool)
	}
	if session.LastPromptText != "Polish the agent sessions pane" {
		t.Fatalf("expected prompt text to be parsed, got %q", session.LastPromptText)
	}
	if session.LastTargetPath != filepath.Join(worktreePath, "internal", "app", "app_agents.go") {
		t.Fatalf("expected target path to be parsed, got %q", session.LastTargetPath)
	}
	if session.TaskLabel != "reading "+summarizePath(filepath.Join(worktreePath, "internal", "app", "app_agents.go")) {
		t.Fatalf("expected task label from target path, got %q", session.TaskLabel)
	}
	if session.Status != models.AgentSessionStatusExecutingTool {
		t.Fatalf("expected executing-tool status, got %q", session.Status)
	}
	if session.Activity != models.AgentActivityReading {
		t.Fatalf("expected reading activity, got %q", session.Activity)
	}
}

func TestParsePiSession(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	worktreePath := filepath.Join(root, "worktrees", "feature")
	ts := time.Now().UTC().Format(time.RFC3339Nano)

	path := filepath.Join(root, "pi.jsonl")
	writeJSONLLines(t, path,
		mustJSONLine(t, map[string]any{
			"type":      "session",
			"timestamp": ts,
			"cwd":       worktreePath,
		}),
		mustJSONLine(t, map[string]any{
			"type": "session_info",
			"name": "Pane polish",
		}),
		mustJSONLine(t, map[string]any{
			"type":    "model_change",
			"modelId": "pi-4",
		}),
		mustJSONLine(t, map[string]any{
			"type":      "message",
			"timestamp": ts,
			"message": map[string]any{
				"role":    "user",
				"content": "Update the card styling",
			},
		}),
		mustJSONLine(t, map[string]any{
			"type":      "message",
			"timestamp": ts,
			"message": map[string]any{
				"role":  "assistant",
				"model": "pi-4",
				"content": []map[string]any{
					{"type": "toolCall", "name": "write", "arguments": map[string]any{"path": filepath.Join(worktreePath, "internal", "app", "app_agents.go")}},
				},
			},
		}),
	)

	session, err := parsePiSession(path, "")
	if err != nil {
		t.Fatalf("parsePiSession returned error: %v", err)
	}

	if session.Agent != models.AgentKindPi {
		t.Fatalf("expected pi agent, got %q", session.Agent)
	}
	if session.DisplayName != "Pane polish" {
		t.Fatalf("expected display name to be parsed, got %q", session.DisplayName)
	}
	if session.CWD != worktreePath {
		t.Fatalf("expected cwd %q, got %q", worktreePath, session.CWD)
	}
	if session.Model != "pi-4" {
		t.Fatalf("expected model to be parsed, got %q", session.Model)
	}
	if session.CurrentTool != "Write" {
		t.Fatalf("expected current tool Write, got %q", session.CurrentTool)
	}
	if session.LastPromptText != "Update the card styling" {
		t.Fatalf("expected prompt text to be parsed, got %q", session.LastPromptText)
	}
	if session.LastTargetPath != filepath.Join(worktreePath, "internal", "app", "app_agents.go") {
		t.Fatalf("expected target path to be parsed, got %q", session.LastTargetPath)
	}
	if session.TaskLabel != "editing "+summarizePath(filepath.Join(worktreePath, "internal", "app", "app_agents.go")) {
		t.Fatalf("expected task label from target path, got %q", session.TaskLabel)
	}
	if session.Status != models.AgentSessionStatusExecutingTool {
		t.Fatalf("expected executing-tool status, got %q", session.Status)
	}
	if session.Activity != models.AgentActivityWriting {
		t.Fatalf("expected writing activity, got %q", session.Activity)
	}
}

func TestAgentSessionServiceSessionsForWorktree(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	claudeRoot := filepath.Join(root, "claude")
	piRoot := filepath.Join(root, "pi")
	worktreePath := filepath.Join(root, "worktrees", "feature")
	otherWorktree := filepath.Join(root, "worktrees", "other")
	now := time.Now().UTC().Format(time.RFC3339Nano)

	writeJSONLLines(t, filepath.Join(claudeRoot, "project-a", "session-1.jsonl"),
		mustJSONLine(t, map[string]any{
			"type":      "assistant",
			"cwd":       filepath.Join(worktreePath, "subdir"),
			"timestamp": now,
			"message": map[string]any{
				"role":  "assistant",
				"model": "claude-sonnet-4",
				"content": []map[string]any{
					{"type": "tool_use", "name": "Read"},
				},
			},
		}),
	)
	writeJSONLLines(t, filepath.Join(piRoot, "project-b", "session-2.jsonl"),
		mustJSONLine(t, map[string]any{
			"type":      "session",
			"timestamp": now,
			"cwd":       otherWorktree,
		}),
	)

	service := NewAgentSessionServiceWithRoots(claudeRoot, piRoot, nil)
	if _, err := service.Refresh(); err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}

	matching := service.SessionsForWorktree(worktreePath)
	if len(matching) != 1 {
		t.Fatalf("expected 1 matching session, got %d", len(matching))
	}
	if matching[0].Agent != models.AgentKindClaude {
		t.Fatalf("expected Claude match, got %q", matching[0].Agent)
	}
}

func TestAgentSessionServiceRefreshInvalidatesCache(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	claudeRoot := filepath.Join(root, "claude")
	sessionPath := filepath.Join(claudeRoot, "project-a", "session-1.jsonl")
	worktreePath := filepath.Join(root, "worktrees", "feature")
	now := time.Now().UTC()

	writeJSONLLines(t, sessionPath,
		mustJSONLine(t, map[string]any{
			"type":      "assistant",
			"cwd":       worktreePath,
			"timestamp": now.Format(time.RFC3339Nano),
			"message": map[string]any{
				"role":  "assistant",
				"model": "claude-sonnet-4",
				"content": []map[string]any{
					{"type": "tool_use", "name": "Read"},
				},
			},
		}),
	)

	service := NewAgentSessionServiceWithRoots(claudeRoot, "", nil)
	first, err := service.Refresh()
	if err != nil {
		t.Fatalf("first Refresh returned error: %v", err)
	}
	if len(first) != 1 || first[0].CurrentTool != "Read" {
		t.Fatalf("expected first refresh to parse Read tool, got %#v", first)
	}

	later := now.Add(2 * time.Second)
	writeJSONLLines(t, sessionPath,
		mustJSONLine(t, map[string]any{
			"type":      "assistant",
			"cwd":       worktreePath,
			"timestamp": later.Format(time.RFC3339Nano),
			"message": map[string]any{
				"role":  "assistant",
				"model": "claude-sonnet-4",
				"content": []map[string]any{
					{"type": "tool_use", "name": "Write"},
				},
			},
		}),
	)
	if err := os.Chtimes(sessionPath, later, later); err != nil {
		t.Fatalf("Chtimes returned error: %v", err)
	}

	second, err := service.Refresh()
	if err != nil {
		t.Fatalf("second Refresh returned error: %v", err)
	}
	if len(second) != 1 || second[0].CurrentTool != "Write" {
		t.Fatalf("expected cache invalidation to pick up Write tool, got %#v", second)
	}
}

func TestResolveAgentActivityKeepsWaitingForOpenSession(t *testing.T) {
	t.Parallel()

	now := time.Now()
	activity := resolveAgentActivity(
		time.Time{},
		time.Time{},
		"",
		"",
		true,
		models.AgentSessionStatusWaitingForUser,
		now.Add(-10*time.Minute),
		now,
	)

	if activity != models.AgentActivityWaiting {
		t.Fatalf("expected open waiting session to remain waiting, got %q", activity)
	}
}

func TestResolveAgentActivityDowngradesClosedWaitingSessionToIdle(t *testing.T) {
	t.Parallel()

	now := time.Now()
	activity := resolveAgentActivity(
		time.Time{},
		time.Time{},
		"",
		"",
		false,
		models.AgentSessionStatusWaitingForUser,
		now.Add(-10*time.Minute),
		now,
	)

	if activity != models.AgentActivityIdle {
		t.Fatalf("expected closed waiting session to decay to idle, got %q", activity)
	}
}

func TestDeriveAgentTaskLabelPrefersCommand(t *testing.T) {
	t.Parallel()

	session := &models.AgentSession{
		LastCommand:    "go test ./internal/app/...",
		LastTargetPath: "/tmp/project/internal/app/app_agents.go",
		LastPromptText: "Fix the agent pane",
	}

	if got := deriveAgentTaskLabel(session); got != "running go test ./internal/app/..." {
		t.Fatalf("expected command-based task label, got %q", got)
	}
}

func writeJSONLLines(t *testing.T, path string, lines ...string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	data := []byte{}
	for _, line := range lines {
		data = append(data, []byte(line+"\n")...)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
}

func mustJSONLine(t *testing.T, v any) string {
	t.Helper()

	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	return string(data)
}
