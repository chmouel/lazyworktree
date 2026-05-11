package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
	writeJSONLLines(
		t, path,
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
	writeJSONLLines(
		t, path,
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

	writeJSONLLines(
		t, filepath.Join(claudeRoot, "project-a", "session-1.jsonl"),
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
	writeJSONLLines(
		t, filepath.Join(piRoot, "project-b", "session-2.jsonl"),
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
	writeJSONLLines(
		t, sessionPath,
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

func TestParseClaudeSessionDecodesHyphenatedFallbackCWD(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "claude.jsonl")
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	writeJSONLLines(
		t, path,
		mustJSONLine(t, map[string]any{
			"type":      "assistant",
			"timestamp": ts,
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "tool_use", "name": "Read"},
				},
			},
		}),
	)

	session, err := parseClaudeSession(path, "-Users-test--team-my-worktree")
	if err != nil {
		t.Fatalf("parseClaudeSession returned error: %v", err)
	}

	if session.CWD != "/Users/test-team/my/worktree" {
		t.Fatalf("expected decoded cwd with preserved hyphen, got %q", session.CWD)
	}
}

func TestAgentSessionServiceRefreshFindsNestedClaudeJSONL(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	claudeRoot := filepath.Join(root, "claude")
	worktreePath := filepath.Join(root, "worktrees", "feature")
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	sessionPath := filepath.Join(claudeRoot, "project-a", "session-1", "subagents", "agent-a.jsonl")

	writeJSONLLines(
		t, sessionPath,
		mustJSONLine(t, map[string]any{
			"type":      "assistant",
			"cwd":       worktreePath,
			"timestamp": ts,
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "tool_use", "name": "Read"},
				},
			},
		}),
	)

	service := NewAgentSessionServiceWithRoots(claudeRoot, "", nil)
	sessions, err := service.Refresh()
	if err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("expected 1 nested Claude session, got %d", len(sessions))
	}
	if sessions[0].JSONLPath != sessionPath {
		t.Fatalf("expected nested session path %q, got %q", sessionPath, sessions[0].JSONLPath)
	}
}

func TestParseClaudeSessionTracksNestedAgentApproval(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	worktreePath := filepath.Join(root, "worktrees", "feature")
	path := filepath.Join(root, "claude.jsonl")
	userTS := time.Now().UTC()
	toolTS := userTS.Add(time.Second)

	writeJSONLLines(
		t, path,
		mustJSONLine(t, map[string]any{
			"type":      "user",
			"cwd":       worktreePath,
			"timestamp": userTS.Format(time.RFC3339Nano),
			"message": map[string]any{
				"role":    "user",
				"content": "Split handlers",
			},
		}),
		mustJSONLine(t, map[string]any{
			"type":      "assistant",
			"cwd":       worktreePath,
			"timestamp": userTS.Format(time.RFC3339Nano),
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "tool_use", "id": "agent-tool", "name": "Agent", "input": map[string]any{"description": "Design app/handlers.go split"}},
				},
			},
		}),
		mustJSONLine(t, map[string]any{
			"type":      "progress",
			"cwd":       worktreePath,
			"timestamp": toolTS.Format(time.RFC3339Nano),
			"data": map[string]any{
				"type":    "agent_progress",
				"agentId": "agent-123",
				"message": map[string]any{
					"type":      "assistant",
					"timestamp": toolTS.Format(time.RFC3339Nano),
					"message": map[string]any{
						"role": "assistant",
						"content": []map[string]any{
							{
								"type": "tool_use",
								"id":   "bash-tool",
								"name": "Bash",
								"input": map[string]any{
									"command":     "grep -n '^func Test' internal/app/handlers_test.go",
									"description": "Analyse test function patterns",
								},
							},
						},
					},
				},
			},
		}),
	)

	session, err := parseClaudeSession(path, "")
	if err != nil {
		t.Fatalf("parseClaudeSession returned error: %v", err)
	}

	if session.Status != models.AgentSessionStatusWaitingApproval {
		t.Fatalf("expected waiting approval status, got %q", session.Status)
	}
	if session.Activity != models.AgentActivityApproval {
		t.Fatalf("expected approval activity, got %q", session.Activity)
	}
	if session.CurrentTool != "Bash" {
		t.Fatalf("expected nested Bash tool to become current tool, got %q", session.CurrentTool)
	}
	if !strings.Contains(session.TaskLabel, "running grep -n") {
		t.Fatalf("expected nested tool command to drive task label, got %q", session.TaskLabel)
	}
}

func TestParseClaudeSessionIgnoresChildProgressForParentSummary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	worktreePath := filepath.Join(root, "worktrees", "feature")
	path := filepath.Join(root, "claude.jsonl")
	now := time.Now().UTC()

	writeJSONLLines(
		t, path,
		mustJSONLine(t, map[string]any{
			"type":      "user",
			"cwd":       worktreePath,
			"timestamp": now.Format(time.RFC3339Nano),
			"message": map[string]any{
				"role":    "user",
				"content": "Split handlers",
			},
		}),
		mustJSONLine(t, map[string]any{
			"type":      "progress",
			"cwd":       worktreePath,
			"timestamp": now.Add(time.Second).Format(time.RFC3339Nano),
			"data": map[string]any{
				"type":    "agent_progress",
				"agentId": "agent-123",
				"message": map[string]any{
					"type":      "assistant",
					"timestamp": now.Add(time.Second).Format(time.RFC3339Nano),
					"message": map[string]any{
						"role":    "assistant",
						"content": "Child progress update",
					},
				},
			},
		}),
	)

	session, err := parseClaudeSession(path, "")
	if err != nil {
		t.Fatalf("parseClaudeSession returned error: %v", err)
	}

	if session.LastPromptText != "Split handlers" {
		t.Fatalf("expected parent prompt text to remain visible, got %q", session.LastPromptText)
	}
	if session.LastReplyText != "" {
		t.Fatalf("expected child progress not to populate parent reply text, got %q", session.LastReplyText)
	}
	if session.Status != models.AgentSessionStatusThinking {
		t.Fatalf("expected parent status to remain thinking, got %q", session.Status)
	}
	if session.TaskLabel != "working on Split handlers" {
		t.Fatalf("expected task label from parent prompt, got %q", session.TaskLabel)
	}
}

func TestParseClaudeSessionNestedToolResultClearsApproval(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	worktreePath := filepath.Join(root, "worktrees", "feature")
	path := filepath.Join(root, "claude.jsonl")
	now := time.Now().UTC()

	writeJSONLLines(
		t, path,
		mustJSONLine(t, map[string]any{
			"type":      "user",
			"cwd":       worktreePath,
			"timestamp": now.Format(time.RFC3339Nano),
			"message": map[string]any{
				"role":    "user",
				"content": "Split handlers",
			},
		}),
		mustJSONLine(t, map[string]any{
			"type":      "progress",
			"cwd":       worktreePath,
			"timestamp": now.Add(time.Second).Format(time.RFC3339Nano),
			"data": map[string]any{
				"type":    "agent_progress",
				"agentId": "agent-123",
				"message": map[string]any{
					"type":      "assistant",
					"timestamp": now.Add(time.Second).Format(time.RFC3339Nano),
					"message": map[string]any{
						"role": "assistant",
						"content": []map[string]any{
							{"type": "tool_use", "id": "bash-tool", "name": "Bash", "input": map[string]any{"command": "grep -n '^func Test' internal/app/handlers_test.go"}},
						},
					},
				},
			},
		}),
		mustJSONLine(t, map[string]any{
			"type":      "progress",
			"cwd":       worktreePath,
			"timestamp": now.Add(2 * time.Second).Format(time.RFC3339Nano),
			"data": map[string]any{
				"type":    "agent_progress",
				"agentId": "agent-123",
				"message": map[string]any{
					"type":      "user",
					"timestamp": now.Add(2 * time.Second).Format(time.RFC3339Nano),
					"message": map[string]any{
						"role": "user",
						"content": []map[string]any{
							{"type": "tool_result", "tool_use_id": "bash-tool", "content": "ok", "is_error": false},
						},
					},
				},
			},
		}),
	)

	session, err := parseClaudeSession(path, "")
	if err != nil {
		t.Fatalf("parseClaudeSession returned error: %v", err)
	}

	if session.Status == models.AgentSessionStatusWaitingApproval {
		t.Fatalf("expected approval status to clear after tool result, got %q", session.Status)
	}
	if session.Activity == models.AgentActivityApproval {
		t.Fatalf("expected approval activity to clear after tool result, got %q", session.Activity)
	}
	if session.Status != models.AgentSessionStatusThinking {
		t.Fatalf("expected parent status to fall back to thinking, got %q", session.Status)
	}
	if session.TaskLabel != "working on Split handlers" {
		t.Fatalf("expected task label to fall back to parent prompt, got %q", session.TaskLabel)
	}
}

func TestParseClaudeSessionChoosesLatestPendingToolDeterministically(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	worktreePath := filepath.Join(root, "worktrees", "feature")
	path := filepath.Join(root, "claude.jsonl")
	now := time.Now().UTC()

	writeJSONLLines(
		t, path,
		mustJSONLine(t, map[string]any{
			"type":      "assistant",
			"cwd":       worktreePath,
			"timestamp": now.Format(time.RFC3339Nano),
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{
						"type":  "tool_use",
						"id":    "read-tool",
						"name":  "Read",
						"input": map[string]any{"file_path": filepath.Join(worktreePath, "first.go")},
					},
					{
						"type":  "tool_use",
						"id":    "write-tool",
						"name":  "Write",
						"input": map[string]any{"file_path": filepath.Join(worktreePath, "second.go")},
					},
				},
			},
		}),
	)

	session, err := parseClaudeSession(path, "")
	if err != nil {
		t.Fatalf("parseClaudeSession returned error: %v", err)
	}

	if session.CurrentTool != "Write" {
		t.Fatalf("expected last same-timestamp pending tool to win, got %q", session.CurrentTool)
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

func TestResolveAgentActivityKeepsOpenExecutingToolActive(t *testing.T) {
	t.Parallel()

	now := time.Now()
	activity := resolveAgentActivity(
		time.Time{},
		now.Add(-10*time.Minute),
		"Bash",
		"Bash",
		true,
		models.AgentSessionStatusExecutingTool,
		now.Add(-10*time.Minute),
		now,
	)

	if activity != models.AgentActivityRunning {
		t.Fatalf("expected open executing tool to remain running, got %q", activity)
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
