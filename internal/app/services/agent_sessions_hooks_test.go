package services

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/chmouel/lazyworktree/internal/models"
)

func newHookTestServices(t *testing.T) (*AgentSessionService, *AgentHookService) {
	t.Helper()
	root := t.TempDir()
	service := NewAgentSessionServiceWithStore(
		filepath.Join(root, "claude"), filepath.Join(root, "pi"),
		NewTestSessionRegistryStore(filepath.Join(root, "registry.json")), nil,
	)
	hooks := NewAgentHookService(filepath.Join(root, "spool"), nil)
	service.SetHookService(hooks)
	return service, hooks
}

func TestRefreshSynthesisesCodexSessionFromHooks(t *testing.T) {
	service, hooks := newHookTestServices(t)
	hooks.pidAlive = func(pid int) bool { return pid == 999 }
	event := models.AgentHookEvent{
		SchemaVersion: models.AgentHookEventSchemaVersion,
		Agent:         models.AgentKindCodex,
		HookEventName: models.AgentHookUserPromptSubmit,
		SessionID:     "c1",
		CWD:           "/tmp/repo",
		Model:         "gpt-5",
		PID:           999,
		Timestamp:     time.Now(),
	}
	if err := WriteAgentHookEvent(hooks.Dir(), event); err != nil {
		t.Fatalf("write event: %v", err)
	}

	sessions, err := service.Refresh()
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 synthesised session, got %d", len(sessions))
	}
	session := sessions[0]
	if session.Agent != models.AgentKindCodex || session.ID != "c1" || session.CWD != "/tmp/repo" {
		t.Fatalf("unexpected session: %+v", session)
	}
	if session.Title != "Codex session" {
		t.Fatalf("expected Codex title, got %q", session.Title)
	}
	if session.LivenessState != models.AgentSessionLivenessActive ||
		session.LivenessSource != models.AgentSessionLivenessSourceHook ||
		!session.IsOpen || session.PID != 999 {
		t.Fatalf("expected hook-live session, got liveness=%s source=%s open=%v pid=%d",
			session.LivenessState, session.LivenessSource, session.IsOpen, session.PID)
	}
	if session.Status != models.AgentSessionStatusThinking {
		t.Fatalf("expected thinking status, got %s", session.Status)
	}
}

func TestRefreshRestoresHookSessionFromRegistry(t *testing.T) {
	root := t.TempDir()
	registryPath := filepath.Join(root, "registry.json")
	newService := func(spool string) (*AgentSessionService, *AgentHookService) {
		service := NewAgentSessionServiceWithStore(
			filepath.Join(root, "claude"), filepath.Join(root, "pi"),
			NewTestSessionRegistryStore(registryPath), nil,
		)
		hooks := NewAgentHookService(spool, nil)
		service.SetHookService(hooks)
		return service, hooks
	}

	service, hooks := newService(filepath.Join(root, "spool-1"))
	hooks.pidAlive = func(pid int) bool { return pid == 999 }
	event := models.AgentHookEvent{
		SchemaVersion: models.AgentHookEventSchemaVersion,
		Agent:         models.AgentKindCodex,
		HookEventName: models.AgentHookStop,
		SessionID:     "c1",
		CWD:           "/tmp/repo",
		PID:           999,
		Timestamp:     time.Now(),
	}
	if err := WriteAgentHookEvent(hooks.Dir(), event); err != nil {
		t.Fatalf("write event: %v", err)
	}
	if _, err := service.Refresh(); err != nil {
		t.Fatalf("initial refresh failed: %v", err)
	}

	restarted, restartedHooks := newService(filepath.Join(root, "spool-2"))
	restartedHooks.pidAlive = func(pid int) bool { return pid == 999 }
	sessions, err := restarted.Refresh()
	if err != nil {
		t.Fatalf("restart refresh failed: %v", err)
	}
	if len(sessions) != 1 || sessions[0].Agent != models.AgentKindCodex || sessions[0].ID != "c1" {
		t.Fatalf("expected restored Codex session, got %+v", sessions)
	}
	if sessions[0].Title != "Codex session" {
		t.Fatalf("expected restored Codex title, got %q", sessions[0].Title)
	}
	if sessions[0].LivenessState != models.AgentSessionLivenessActive ||
		sessions[0].LivenessSource != models.AgentSessionLivenessSourceHook || !sessions[0].IsOpen {
		t.Fatalf("expected restored Codex session to remain active, got %+v", sessions[0])
	}
	if !restartedHooks.HasLiveSessions() {
		t.Fatal("expected restored hook state to retain live-session tracking")
	}

	deadRestart, deadHooks := newService(filepath.Join(root, "spool-3"))
	deadHooks.pidAlive = func(int) bool { return false }
	deadSessions, err := deadRestart.Refresh()
	if err != nil || len(deadSessions) != 1 {
		t.Fatalf("dead-pid restart refresh: sessions=%+v err=%v", deadSessions, err)
	}
	if deadSessions[0].LivenessState == models.AgentSessionLivenessActive || deadSessions[0].IsOpen {
		t.Fatalf("dead persisted PID must not be restored as active: %+v", deadSessions[0])
	}
}

func TestRefreshHookSessionUpdatesResumedCWD(t *testing.T) {
	service, hooks := newHookTestServices(t)
	hooks.pidAlive = func(pid int) bool { return pid == 999 }
	oldCWD := filepath.Join(t.TempDir(), "old")
	newCWD := filepath.Join(t.TempDir(), "new")
	event := models.AgentHookEvent{
		SchemaVersion: models.AgentHookEventSchemaVersion,
		Agent:         models.AgentKindCodex,
		HookEventName: models.AgentHookSessionStart,
		SessionID:     "c1",
		CWD:           oldCWD,
		PID:           999,
		Timestamp:     time.Now(),
	}
	if err := WriteAgentHookEvent(hooks.Dir(), event); err != nil {
		t.Fatalf("write initial event: %v", err)
	}
	if _, err := service.Refresh(); err != nil {
		t.Fatalf("initial refresh: %v", err)
	}

	event.HookEventName = models.AgentHookCwdChanged
	event.CWD = newCWD
	event.Timestamp = event.Timestamp.Add(time.Second)
	if err := WriteAgentHookEvent(hooks.Dir(), event); err != nil {
		t.Fatalf("write resumed event: %v", err)
	}
	sessions, err := service.Refresh()
	if err != nil || len(sessions) != 1 {
		t.Fatalf("resumed refresh: sessions=%+v err=%v", sessions, err)
	}
	if sessions[0].CWD != newCWD {
		t.Fatalf("cwd = %q, want latest hook cwd %q", sessions[0].CWD, newCWD)
	}
	if sessions[0].SessionKey != agentSessionKey(sessions[0]) {
		t.Fatalf("session key was not updated after cwd change: %q", sessions[0].SessionKey)
	}
}

func TestRefreshHookSessionEndMarksInactive(t *testing.T) {
	service, hooks := newHookTestServices(t)
	hooks.pidAlive = func(int) bool { return true }
	now := time.Now()
	for i, name := range []string{models.AgentHookSessionStart, models.AgentHookSessionEnd} {
		event := models.AgentHookEvent{
			SchemaVersion: models.AgentHookEventSchemaVersion,
			Agent:         models.AgentKindCodex,
			HookEventName: name,
			SessionID:     "c1",
			CWD:           "/tmp/repo",
			PID:           999,
			Timestamp:     now.Add(time.Duration(i) * time.Second),
		}
		if err := WriteAgentHookEvent(hooks.Dir(), event); err != nil {
			t.Fatalf("write event: %v", err)
		}
	}

	sessions, err := service.Refresh()
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	for _, session := range sessions {
		if session.Agent != models.AgentKindCodex {
			continue
		}
		if session.LivenessState == models.AgentSessionLivenessActive || session.IsOpen {
			t.Fatalf("ended session must not be active: %+v", session)
		}
	}
}

func TestRefreshHookPromptReplacesWaitingStatus(t *testing.T) {
	service, hooks := newHookTestServices(t)
	hooks.pidAlive = func(pid int) bool { return pid == 999 }
	stopAt := time.Now().Add(-time.Minute)
	write := func(name string, timestamp time.Time) {
		t.Helper()
		if err := WriteAgentHookEvent(hooks.Dir(), models.AgentHookEvent{
			SchemaVersion: models.AgentHookEventSchemaVersion,
			Agent:         models.AgentKindCodex,
			HookEventName: name,
			SessionID:     "c1",
			CWD:           "/tmp/repo",
			PID:           999,
			Timestamp:     timestamp,
		}); err != nil {
			t.Fatalf("write %s event: %v", name, err)
		}
	}

	write(models.AgentHookStop, stopAt)
	sessions, err := service.Refresh()
	if err != nil || len(sessions) != 1 {
		t.Fatalf("refresh stop event: sessions=%+v err=%v", sessions, err)
	}
	if sessions[0].Status != models.AgentSessionStatusWaitingForUser {
		t.Fatalf("expected waiting status, got %s", sessions[0].Status)
	}

	promptAt := time.Now()
	write(models.AgentHookUserPromptSubmit, promptAt)
	sessions, err = service.Refresh()
	if err != nil || len(sessions) != 1 {
		t.Fatalf("refresh prompt event: sessions=%+v err=%v", sessions, err)
	}
	if sessions[0].Status != models.AgentSessionStatusThinking || sessions[0].Activity != models.AgentActivityThinking {
		t.Fatalf("expected thinking status/activity, got status=%s activity=%s", sessions[0].Status, sessions[0].Activity)
	}
	if sessions[0].LastActivity.Before(promptAt) {
		t.Fatalf("expected prompt hook to refresh activity time, got %v before %v", sessions[0].LastActivity, promptAt)
	}
}

func TestRefreshHookDeadPIDKeepsClassifierVerdict(t *testing.T) {
	service, hooks := newHookTestServices(t)
	hooks.pidAlive = func(int) bool { return false }
	event := models.AgentHookEvent{
		SchemaVersion: models.AgentHookEventSchemaVersion,
		Agent:         models.AgentKindCodex,
		HookEventName: models.AgentHookSessionStart,
		SessionID:     "c1",
		CWD:           "/tmp/repo",
		PID:           999,
		Timestamp:     time.Now(),
	}
	if err := WriteAgentHookEvent(hooks.Dir(), event); err != nil {
		t.Fatalf("write event: %v", err)
	}

	sessions, err := service.Refresh()
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].LivenessState == models.AgentSessionLivenessActive {
		t.Fatalf("dead PID must not be active: %+v", sessions[0])
	}
}

func TestHookStateMatchesSession(t *testing.T) {
	session := &models.AgentSession{
		Agent: models.AgentKindClaude, ID: "s1", JSONLPath: "/tmp/a/../t.jsonl",
	}
	byPath := AgentHookState{Agent: models.AgentKindClaude, SessionID: "other", TranscriptPath: "/tmp/t.jsonl"}
	if !hookStateMatchesSession(&byPath, session) {
		t.Fatal("expected transcript path match")
	}
	byID := AgentHookState{Agent: models.AgentKindClaude, SessionID: "s1"}
	if !hookStateMatchesSession(&byID, session) {
		t.Fatal("expected session-id match")
	}
	wrongAgent := AgentHookState{Agent: models.AgentKindCodex, SessionID: "s1"}
	if hookStateMatchesSession(&wrongAgent, session) {
		t.Fatal("agent mismatch must not match")
	}
}

func TestRefreshSynthesisesCopilotSessionFromHooks(t *testing.T) {
	// Refresh() passes no process snapshot, matching the default
	// configuration where the deprecated ps/lsof scan is disabled: hook
	// events alone must provide session state and liveness.
	service, hooks := newHookTestServices(t)
	hooks.pidAlive = func(pid int) bool { return pid == 424 }
	event := models.AgentHookEvent{
		SchemaVersion: models.AgentHookEventSchemaVersion,
		Agent:         models.AgentKindCopilot,
		HookEventName: models.AgentHookUserPromptSubmit,
		SessionID:     "cop-1",
		CWD:           "/tmp/repo",
		PID:           424,
		Timestamp:     time.Now(),
	}
	if err := WriteAgentHookEvent(hooks.Dir(), event); err != nil {
		t.Fatalf("write event: %v", err)
	}

	sessions, err := service.Refresh()
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 synthesised session, got %d", len(sessions))
	}
	session := sessions[0]
	if session.Agent != models.AgentKindCopilot || session.ID != "cop-1" {
		t.Fatalf("unexpected session: %+v", session)
	}
	if session.Title != "Copilot session" {
		t.Fatalf("expected Copilot title, got %q", session.Title)
	}
	if session.LivenessState != models.AgentSessionLivenessActive ||
		session.LivenessSource != models.AgentSessionLivenessSourceHook ||
		!session.IsOpen || session.PID != 424 {
		t.Fatalf("expected hook-live session, got liveness=%s source=%s open=%v pid=%d",
			session.LivenessState, session.LivenessSource, session.IsOpen, session.PID)
	}
}

func TestRefreshQuestionLifecycleChangesHookStatus(t *testing.T) {
	service, hooks := newHookTestServices(t)
	hooks.pidAlive = func(pid int) bool { return pid == 424 }
	write := func(name string, timestamp time.Time) {
		t.Helper()
		if err := WriteAgentHookEvent(hooks.Dir(), models.AgentHookEvent{
			SchemaVersion: models.AgentHookEventSchemaVersion,
			Agent:         models.AgentKindCopilot,
			HookEventName: name,
			SessionID:     "cop-1",
			CWD:           "/tmp/repo",
			PID:           424,
			Timestamp:     timestamp,
		}); err != nil {
			t.Fatalf("write %s event: %v", name, err)
		}
	}

	waitingAt := time.Now()
	write(models.AgentHookWaitingForUser, waitingAt)
	sessions, err := service.Refresh()
	if err != nil || len(sessions) != 1 {
		t.Fatalf("refresh waiting event: sessions=%+v err=%v", sessions, err)
	}
	if sessions[0].Status != models.AgentSessionStatusWaitingForUser {
		t.Fatalf("status = %s, want waiting", sessions[0].Status)
	}
	if sessions[0].Activity != models.AgentActivityWaiting {
		t.Fatalf("activity = %s, want waiting", sessions[0].Activity)
	}

	write(models.AgentHookUserPromptSubmit, waitingAt.Add(time.Second))
	sessions, err = service.Refresh()
	if err != nil || len(sessions) != 1 {
		t.Fatalf("refresh response event: sessions=%+v err=%v", sessions, err)
	}
	if sessions[0].Status != models.AgentSessionStatusThinking {
		t.Fatalf("status = %s, want thinking", sessions[0].Status)
	}
	if sessions[0].Activity != models.AgentActivityThinking {
		t.Fatalf("activity = %s, want thinking", sessions[0].Activity)
	}
}

func TestHookQuestionActivityOverridesRecentTool(t *testing.T) {
	now := time.Now()
	tests := []struct {
		event string
		want  models.AgentActivity
	}{
		{event: models.AgentHookWaitingForUser, want: models.AgentActivityWaiting},
		{event: models.AgentHookUserPromptSubmit, want: models.AgentActivityThinking},
	}

	for _, tt := range tests {
		t.Run(tt.event, func(t *testing.T) {
			service := &AgentSessionService{}
			hooks := NewAgentHookService(t.TempDir(), nil)
			hooks.pidAlive = func(pid int) bool { return pid == 424 }
			session := &models.AgentSession{
				Agent:        models.AgentKindCopilot,
				ID:           "cop-1",
				LastToolAt:   now,
				LastToolName: "Bash",
				LastActivity: now,
			}
			state := AgentHookState{
				Agent:         models.AgentKindCopilot,
				SessionID:     "cop-1",
				PID:           424,
				LastEventName: tt.event,
				LastEventAt:   now,
			}

			service.applyHookLiveness(
				[]*models.AgentSession{session},
				hooks,
				[]AgentHookState{state},
				now,
			)
			if session.Activity != tt.want {
				t.Fatalf("activity = %s, want %s", session.Activity, tt.want)
			}
		})
	}
}
