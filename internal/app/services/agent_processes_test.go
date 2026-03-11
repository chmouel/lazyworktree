package services

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chmouel/lazyworktree/internal/models"
)

func TestParseAgentProcessesPS(t *testing.T) {
	t.Parallel()

	processes := parseAgentProcessesPS(stringsJoin(
		"101 claude claude",
		"202 Claude /Applications/Claude.app/Contents/MacOS/Claude",
		"303 pi pi --continue",
		"404 bash bash",
	))

	if len(processes) != 3 {
		t.Fatalf("expected 3 agent processes, got %d", len(processes))
	}
	if processes[0].Agent != models.AgentKindClaude || processes[0].Source != "cli" {
		t.Fatalf("expected first process to be Claude CLI, got %#v", processes[0])
	}
	if processes[1].Agent != models.AgentKindClaude || processes[1].Source != "desktop" {
		t.Fatalf("expected second process to be Claude Desktop, got %#v", processes[1])
	}
	if processes[2].Agent != models.AgentKindPi {
		t.Fatalf("expected third process to be pi, got %#v", processes[2])
	}
}

func TestApplyAgentProcessLSOF(t *testing.T) {
	t.Parallel()

	processes := []*AgentProcess{{PID: 101, Agent: models.AgentKindClaude}}
	applyAgentProcessLSOF(processes, stringsJoin(
		"p101",
		"fcwd",
		"n/tmp/worktree",
		"f12",
		"n/tmp/worktree/.claude/session.jsonl",
	))

	if processes[0].CWD != "/tmp/worktree" {
		t.Fatalf("expected cwd to be populated, got %q", processes[0].CWD)
	}
	if len(processes[0].OpenFiles) != 1 || processes[0].OpenFiles[0] != "/tmp/worktree/.claude/session.jsonl" {
		t.Fatalf("expected open files to be populated, got %#v", processes[0].OpenFiles)
	}
}

func TestMatchAgentProcessesToSessionsByJSONLPath(t *testing.T) {
	t.Parallel()

	sessionPath := "/tmp/worktree/.claude/session.jsonl"
	sessions := []*models.AgentSession{{
		ID:           "session-a",
		Agent:        models.AgentKindClaude,
		JSONLPath:    sessionPath,
		CWD:          "/tmp/worktree",
		LastActivity: time.Now(),
	}}
	processes := []*AgentProcess{{
		PID:       101,
		Agent:     models.AgentKindClaude,
		OpenFiles: []string{sessionPath},
	}}

	matched := matchAgentProcessesToSessions(sessions, processes)
	if len(matched) != 1 || !matched[0].IsOpen {
		t.Fatalf("expected session to be marked open, got %#v", matched)
	}
	if matched[0].OpenConfidence != models.AgentOpenConfidenceExact {
		t.Fatalf("expected exact confidence, got %q", matched[0].OpenConfidence)
	}
}

func TestMatchAgentProcessesToSessionsByCWDPrefersNewest(t *testing.T) {
	t.Parallel()

	worktreePath := filepath.Clean("/tmp/worktree")
	oldTime := time.Now().Add(-time.Hour)
	newTime := time.Now()
	sessions := []*models.AgentSession{
		{
			ID:           "old",
			Agent:        models.AgentKindClaude,
			CWD:          worktreePath,
			LastActivity: oldTime,
		},
		{
			ID:           "new",
			Agent:        models.AgentKindClaude,
			CWD:          worktreePath,
			LastActivity: newTime,
		},
	}
	processes := []*AgentProcess{{
		PID:   202,
		Agent: models.AgentKindClaude,
		CWD:   worktreePath,
	}}

	matched := matchAgentProcessesToSessions(sessions, processes)
	if matched[0].ID != "new" && matched[1].ID != "new" {
		t.Fatalf("expected newest session to remain present, got %#v", matched)
	}

	var openSession *models.AgentSession
	for _, session := range matched {
		if session.IsOpen {
			openSession = session
			break
		}
	}
	if openSession == nil {
		t.Fatalf("expected one session to be marked open, got %#v", matched)
	}
	if openSession.ID != "new" {
		t.Fatalf("expected newest session to win cwd-only match, got %q", openSession.ID)
	}
	if openSession.OpenConfidence != models.AgentOpenConfidenceCWD {
		t.Fatalf("expected cwd confidence, got %q", openSession.OpenConfidence)
	}
}

func stringsJoin(lines ...string) string {
	return strings.Join(lines, "\n")
}
