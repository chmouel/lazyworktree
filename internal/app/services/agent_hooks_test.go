package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chmouel/lazyworktree/internal/models"
)

func TestParseAgentHookPayload(t *testing.T) {
	event, err := ParseAgentHookPayload(models.AgentKindClaude, []byte(`{
		"hook_event_name": "SessionStart",
		"session_id": " abc ",
		"transcript_path": "/tmp/t.jsonl",
		"cwd": "/tmp/repo",
		"model": "opus",
		"source": "startup"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.Agent != models.AgentKindClaude || event.SessionID != "abc" ||
		event.HookEventName != models.AgentHookSessionStart ||
		event.TranscriptPath != "/tmp/t.jsonl" || event.CWD != "/tmp/repo" ||
		event.Model != "opus" || event.Source != "startup" {
		t.Fatalf("unexpected event: %+v", event)
	}
	if event.SchemaVersion != models.AgentHookEventSchemaVersion {
		t.Fatalf("unexpected schema version: %q", event.SchemaVersion)
	}
}

func TestParseAgentHookPayloadErrors(t *testing.T) {
	if _, err := ParseAgentHookPayload(models.AgentKindClaude, []byte("not json")); err == nil {
		t.Fatal("expected decode error")
	}
	if _, err := ParseAgentHookPayload(models.AgentKindClaude, []byte(`{"hook_event_name":"Stop"}`)); err == nil {
		t.Fatal("expected missing session_id error")
	}
}

func TestRecordAgentHookEventSpoolsFile(t *testing.T) {
	dir := t.TempDir()
	payload := strings.NewReader(`{"hook_event_name":"UserPromptSubmit","session_id":"s1","cwd":"/tmp"}`)
	if err := RecordAgentHookEvent(dir, models.AgentKindCodex, payload); err != nil {
		t.Fatalf("record failed: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected one spool file, got %d (err=%v)", len(entries), err)
	}
	name := entries[0].Name()
	if !strings.HasSuffix(name, "-codex-UserPromptSubmit.json") {
		t.Fatalf("unexpected spool file name: %q", name)
	}
}

func TestFindAgentAncestorPIDSkipsHookShell(t *testing.T) {
	processes := map[int]agentHookProcessInfo{
		300: {ParentPID: 200, Command: "lazyworktree", Args: "lazyworktree agent-event --agent codex"},
		200: {ParentPID: 100, Command: "zsh", Args: "zsh -lc lazyworktree agent-event --agent codex"},
		100: {ParentPID: 1, Command: "codex", Args: "codex --yolo"},
	}
	got := findAgentAncestorPID(models.AgentKindCodex, 300, func(pid int) (agentHookProcessInfo, bool) {
		process, ok := processes[pid]
		return process, ok
	})
	if got != 100 {
		t.Fatalf("expected Codex ancestor PID 100, got %d", got)
	}
}

func TestAgentHookProcessArgsResolvesWrappedAgent(t *testing.T) {
	resolvedPID := 0
	got := agentHookProcessArgs(42, "node.exe", func(pid int) string {
		resolvedPID = pid
		return `"C:\Program Files\nodejs\node.exe" C:\Users\me\AppData\Roaming\npm\node_modules\@openai\codex\bin\codex.js`
	})
	if resolvedPID != 42 || !strings.Contains(got, `@openai\codex`) {
		t.Fatalf("wrapped command line was not resolved: pid=%d args=%q", resolvedPID, got)
	}
}

func TestAgentHookProcessArgsSkipsNativeAgent(t *testing.T) {
	called := false
	got := agentHookProcessArgs(42, "codex.exe", func(int) string {
		called = true
		return "unexpected"
	})
	if called || got != "codex.exe" {
		t.Fatalf("native agent should use executable name: called=%v args=%q", called, got)
	}
}

func TestAgentHookServiceDrainAppliesEvents(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	write := func(offset time.Duration, event models.AgentHookEvent) {
		event.SchemaVersion = models.AgentHookEventSchemaVersion
		event.Timestamp = now.Add(offset)
		if err := WriteAgentHookEvent(dir, event); err != nil {
			t.Fatalf("write event: %v", err)
		}
	}
	write(0, models.AgentHookEvent{
		Agent: models.AgentKindClaude, HookEventName: models.AgentHookSessionStart,
		SessionID: "s1", TranscriptPath: "/tmp/s1.jsonl", CWD: "/tmp/repo", Model: "opus", PID: 4242,
	})
	write(time.Second, models.AgentHookEvent{
		Agent: models.AgentKindClaude, HookEventName: models.AgentHookUserPromptSubmit,
		SessionID: "s1", PID: 4242,
	})
	write(2*time.Second, models.AgentHookEvent{
		Agent: models.AgentKindCodex, HookEventName: models.AgentHookSessionStart,
		SessionID: "c1", CWD: "/tmp/repo", PID: 999,
	})

	svc := NewAgentHookService(dir, nil)
	svc.Drain()

	states := svc.States()
	if len(states) != 2 {
		t.Fatalf("expected 2 states, got %d: %+v", len(states), states)
	}
	claude := states[0]
	if claude.Agent != models.AgentKindClaude || claude.SessionID != "s1" ||
		claude.TranscriptPath != "/tmp/s1.jsonl" || claude.CWD != "/tmp/repo" ||
		claude.PID != 4242 || claude.LastEventName != models.AgentHookUserPromptSubmit || claude.Ended {
		t.Fatalf("unexpected claude state: %+v", claude)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("expected spool drained, %d files remain", len(entries))
	}
}

func TestAgentHookServiceSessionEndMarksEnded(t *testing.T) {
	dir := t.TempDir()
	event := models.AgentHookEvent{
		Agent: models.AgentKindClaude, HookEventName: models.AgentHookSessionEnd,
		SessionID: "s1", PID: 4242, Timestamp: time.Now(),
	}
	if err := WriteAgentHookEvent(dir, event); err != nil {
		t.Fatalf("write event: %v", err)
	}
	svc := NewAgentHookService(dir, nil)
	svc.Drain()
	states := svc.States()
	if len(states) != 1 || !states[0].Ended {
		t.Fatalf("expected ended state, got %+v", states)
	}
}

func TestAgentHookServiceDrainSkipsMalformedFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte("nonsense"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := NewAgentHookService(dir, nil)
	svc.Drain()
	if len(svc.States()) != 0 {
		t.Fatal("expected no states from malformed file")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatal("expected malformed file removed")
	}
}

func TestAgentHookServiceHasLiveSessions(t *testing.T) {
	svc := NewAgentHookService(t.TempDir(), nil)
	svc.pidAlive = func(pid int) bool { return pid == 4242 }
	svc.states["claude:s1"] = &AgentHookState{
		Agent: models.AgentKindClaude, SessionID: "s1", PID: 4242, LastEventAt: time.Now(),
	}
	if !svc.HasLiveSessions() {
		t.Fatal("expected live session")
	}
	svc.states["claude:s1"].Ended = true
	if svc.HasLiveSessions() {
		t.Fatal("ended session must not count as live")
	}
	svc.states["claude:s1"].Ended = false
	svc.states["claude:s1"].PID = 1
	if svc.HasLiveSessions() {
		t.Fatal("dead PID must not count as live")
	}
}

func TestAgentHookServicePruneStale(t *testing.T) {
	svc := NewAgentHookService(t.TempDir(), nil)
	svc.states["claude:old"] = &AgentHookState{
		Agent: models.AgentKindClaude, SessionID: "old",
		LastEventAt: time.Now().Add(-2 * agentHookStaleAfter),
	}
	svc.states["claude:ended"] = &AgentHookState{
		Agent: models.AgentKindClaude, SessionID: "ended", Ended: true,
		LastEventAt: time.Now().Add(-2 * agentRecentThreshold),
	}
	svc.Drain()
	if len(svc.States()) != 0 {
		t.Fatalf("expected stale states pruned, got %+v", svc.States())
	}
}

func TestParseAgentHookPayloadCopilot(t *testing.T) {
	// Copilot CLI VS Code compatible payload (PascalCase event config).
	event, err := ParseAgentHookPayload(models.AgentKindCopilot, []byte(`{
		"hook_event_name": "Stop",
		"session_id": "cop-1",
		"timestamp": "2026-07-12T20:00:00Z",
		"cwd": "/tmp/repo",
		"transcript_path": "/tmp/state/session.jsonl",
		"stop_reason": "end_turn"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.Agent != models.AgentKindCopilot || event.SessionID != "cop-1" ||
		event.HookEventName != models.AgentHookStop ||
		event.TranscriptPath != "/tmp/state/session.jsonl" || event.CWD != "/tmp/repo" {
		t.Fatalf("unexpected event: %+v", event)
	}
	if event.Model != "" {
		t.Fatalf("copilot payloads carry no model, got %q", event.Model)
	}
}

func TestParseAgentHookPayloadQuestionLifecycle(t *testing.T) {
	tests := []struct {
		name  string
		agent models.AgentKind
		data  string
		want  string
	}{
		{
			name:  "copilot camelCase elicitation",
			agent: models.AgentKindCopilot,
			data: `{
				"hook_event_name": "Notification",
				"sessionId": "cop-1",
				"notification_type": "elicitation_dialog",
				"cwd": "/tmp/repo"
			}`,
			want: models.AgentHookWaitingForUser,
		},
		{
			name:  "copilot ask_user opened",
			agent: models.AgentKindCopilot,
			data: `{
				"hook_event_name": "PreToolUse",
				"session_id": "cop-1",
				"tool_name": "AskUserQuestion"
			}`,
			want: models.AgentHookWaitingForUser,
		},
		{
			name:  "claude snake_case elicitation",
			agent: models.AgentKindClaude,
			data: `{
				"hook_event_name": "Notification",
				"session_id": "claude-1",
				"notification_type": "agent_needs_input",
				"cwd": "/tmp/repo"
			}`,
			want: models.AgentHookWaitingForUser,
		},
		{
			name:  "claude elicitation response",
			agent: models.AgentKindClaude,
			data: `{
				"hook_event_name": "Notification",
				"session_id": "claude-1",
				"notification_type": "elicitation_response"
			}`,
			want: models.AgentHookUserPromptSubmit,
		},
		{
			name:  "claude MCP elicitation",
			agent: models.AgentKindClaude,
			data: `{
				"hook_event_name": "Elicitation",
				"session_id": "claude-1"
			}`,
			want: models.AgentHookWaitingForUser,
		},
		{
			name:  "claude MCP elicitation result",
			agent: models.AgentKindClaude,
			data: `{
				"hook_event_name": "ElicitationResult",
				"session_id": "claude-1"
			}`,
			want: models.AgentHookUserPromptSubmit,
		},
		{
			name:  "copilot ask_user completed",
			agent: models.AgentKindCopilot,
			data: `{
				"hook_event_name": "PostToolUse",
				"session_id": "cop-1",
				"tool_name": "AskUserQuestion"
			}`,
			want: models.AgentHookUserPromptSubmit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := ParseAgentHookPayload(tt.agent, []byte(tt.data))
			if err != nil {
				t.Fatalf("parse payload: %v", err)
			}
			if event.HookEventName != tt.want {
				t.Fatalf("event name = %q, want %q", event.HookEventName, tt.want)
			}
		})
	}
}

func TestFindAgentAncestorPIDCopilot(t *testing.T) {
	processes := map[int]agentHookProcessInfo{
		300: {ParentPID: 200, Command: "lazyworktree", Args: "lazyworktree agent-event --agent copilot"},
		200: {ParentPID: 100, Command: "bash", Args: "bash -c lazyworktree agent-event --agent copilot"},
		100: {ParentPID: 1, Command: "node", Args: "node /usr/lib/node_modules/@github/copilot/index.js"},
	}
	got := findAgentAncestorPID(models.AgentKindCopilot, 300, func(pid int) (agentHookProcessInfo, bool) {
		process, ok := processes[pid]
		return process, ok
	})
	if got != 100 {
		t.Fatalf("expected Copilot ancestor PID 100, got %d", got)
	}
}
