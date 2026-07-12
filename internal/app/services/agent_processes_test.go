package services

import (
	"os/exec"
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
		"404 node node /opt/homebrew/bin/claude --model sonnet",
		"505 zsh zsh -lc claude --print",
		"606 npm npm exec @anthropic-ai/claude-code -- --print",
		"707 bash bash -lc echo claude",
		"808 codex codex --yolo",
	))

	if len(processes) != 7 {
		t.Fatalf("expected 7 agent processes, got %d", len(processes))
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
	for _, idx := range []int{3, 4, 5} {
		if processes[idx].Agent != models.AgentKindClaude || processes[idx].Source != "cli" {
			t.Fatalf("expected wrapped process %d to be Claude CLI, got %#v", idx, processes[idx])
		}
	}
	if processes[6].Agent != models.AgentKindCodex || processes[6].Source != "cli" {
		t.Fatalf("expected Codex CLI process, got %#v", processes[6])
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
	if matched[0].LivenessState != models.AgentSessionLivenessActive {
		t.Fatalf("expected active liveness, got %q", matched[0].LivenessState)
	}
	if matched[0].LivenessSource != models.AgentSessionLivenessSourceExactFile {
		t.Fatalf("expected exact-file source, got %q", matched[0].LivenessSource)
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

	var suspectCount int
	var suspectSession *models.AgentSession
	for _, session := range matched {
		if session.LivenessState == models.AgentSessionLivenessSuspect {
			suspectCount++
			if suspectSession == nil || session.LastActivity.After(suspectSession.LastActivity) {
				suspectSession = session
			}
		}
	}
	if suspectCount != 1 || suspectSession == nil {
		t.Fatalf("expected one recent session to be marked suspect, got %#v", matched)
		return
	}
	if suspectSession.ID != "new" {
		t.Fatalf("expected newest session to win cwd-only heuristic, got %q", suspectSession.ID)
	}
	if suspectSession.IsOpen {
		t.Fatalf("expected cwd-only match to stay closed, got %#v", suspectSession)
	}
	if suspectSession.OpenConfidence != models.AgentOpenConfidenceCWD {
		t.Fatalf("expected cwd confidence, got %q", suspectSession.OpenConfidence)
	}
	if suspectSession.LivenessSource != models.AgentSessionLivenessSourceCWDHeuristic {
		t.Fatalf("expected cwd heuristic source, got %q", suspectSession.LivenessSource)
	}
	if suspectSession.LastObservedAt.IsZero() {
		t.Fatalf("expected cwd heuristic match to refresh LastObservedAt, got %#v", suspectSession)
	}
}

func TestMatchAgentProcessesToSessionsByCWDRefreshesLongRunningSession(t *testing.T) {
	t.Parallel()

	worktreePath := filepath.Clean("/tmp/worktree")
	oldObservation := time.Now().Add(-2 * agentRecentThreshold)
	processes := []*AgentProcess{{
		PID:   303,
		Agent: models.AgentKindClaude,
		CWD:   worktreePath,
	}}
	sessions := []*models.AgentSession{{
		ID:             "long-running",
		Agent:          models.AgentKindClaude,
		CWD:            worktreePath,
		LastActivity:   time.Now().Add(-time.Hour),
		LastObservedAt: oldObservation,
	}}

	matched := matchAgentProcessesToSessions(sessions, processes)
	if len(matched) != 1 {
		t.Fatalf("expected one matched session, got %#v", matched)
	}
	if matched[0].LivenessState != models.AgentSessionLivenessSuspect {
		t.Fatalf("expected long-running cwd match to stay suspect, got %q", matched[0].LivenessState)
	}
	if !matched[0].LastObservedAt.After(oldObservation) {
		t.Fatalf("expected LastObservedAt to refresh, old=%v new=%v", oldObservation, matched[0].LastObservedAt)
	}
}

func TestClassifyAgentProcessIgnoresUnrelatedShellCommand(t *testing.T) {
	t.Parallel()

	agent, source, ok := classifyAgentProcess("bash", "bash -lc echo claude")
	if ok || agent != "" || source != "" {
		t.Fatalf("expected unrelated shell command to be ignored, got %q %q %v", agent, source, ok)
	}
}

func TestClassifyAgentProcessMatchesShellExecWrapper(t *testing.T) {
	t.Parallel()

	agent, source, ok := classifyAgentProcess("bash", "bash -lc 'exec claude --print'")
	if !ok {
		t.Fatal("expected shell exec wrapper to be detected")
	}
	if agent != models.AgentKindClaude || source != "cli" {
		t.Fatalf("expected Claude CLI match, got %q %q", agent, source)
	}
}

func stringsJoin(lines ...string) string {
	return strings.Join(lines, "\n")
}

func TestRefreshReusesLSOFDetailsWhenProcessesUnchanged(t *testing.T) {
	psOut := "101 claude claude --continue"
	lsofOut := "p101\nfcwd\nn/tmp/worktree"
	lsofCalls := 0
	runner := func(name string, args ...string) *exec.Cmd {
		switch name {
		case "ps":
			return exec.Command("printf", "%s", psOut)
		case "lsof":
			lsofCalls++
			return exec.Command("printf", "%s", lsofOut)
		default:
			return exec.Command("true")
		}
	}
	s := NewAgentProcessServiceWithRunner(runner, nil)

	first, err := s.Refresh()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(first) != 1 || first[0].CWD != "/tmp/worktree" {
		t.Fatalf("expected cwd from lsof, got %#v", first)
	}
	if lsofCalls != 1 {
		t.Fatalf("expected 1 lsof call, got %d", lsofCalls)
	}

	second, err := s.Refresh()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lsofCalls != 1 {
		t.Fatalf("expected lsof to be skipped for unchanged processes, got %d calls", lsofCalls)
	}
	if len(second) != 1 || second[0].CWD != "/tmp/worktree" {
		t.Fatalf("expected cached cwd to be reused, got %#v", second)
	}
}

func TestRefreshRerunsLSOFWhenProcessSetChanges(t *testing.T) {
	psOut := "101 claude claude --continue"
	lsofCalls := 0
	runner := func(name string, args ...string) *exec.Cmd {
		switch name {
		case "ps":
			return exec.Command("printf", "%s", psOut)
		case "lsof":
			lsofCalls++
			return exec.Command("printf", "%s", "p101\nfcwd\nn/tmp/worktree")
		default:
			return exec.Command("true")
		}
	}
	s := NewAgentProcessServiceWithRunner(runner, nil)

	if _, err := s.Refresh(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	psOut = "101 claude claude --continue\n202 pi pi --continue"
	if _, err := s.Refresh(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lsofCalls != 2 {
		t.Fatalf("expected lsof to run again after process set change, got %d calls", lsofCalls)
	}
}

func TestRefreshRetriesLSOFAfterFailure(t *testing.T) {
	lsofFails := true
	lsofCalls := 0
	runner := func(name string, args ...string) *exec.Cmd {
		switch name {
		case "ps":
			return exec.Command("printf", "%s", "101 claude claude --continue")
		case "lsof":
			lsofCalls++
			if lsofFails {
				return exec.Command("false")
			}
			return exec.Command("printf", "%s", "p101\nfcwd\nn/tmp/worktree")
		default:
			return exec.Command("true")
		}
	}
	s := NewAgentProcessServiceWithRunner(runner, nil)

	if _, err := s.Refresh(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lsofFails = false
	processes, err := s.Refresh()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lsofCalls != 2 {
		t.Fatalf("expected lsof retry after failure, got %d calls", lsofCalls)
	}
	if len(processes) != 1 || processes[0].CWD != "/tmp/worktree" {
		t.Fatalf("expected cwd after successful retry, got %#v", processes)
	}
}

func TestClassifyAgentProcessCopilot(t *testing.T) {
	cases := []struct {
		command string
		args    string
		want    bool
	}{
		{"copilot", "copilot --banner", true},
		{"/usr/local/bin/copilot", "copilot", true},
		{"node", "node /usr/lib/node_modules/@github/copilot/index.js", true},
		{"node.exe", `node.exe C:\Users\me\AppData\Roaming\npm\node_modules\@github\copilot\index.js`, true},
		{"npx", "npx @github/copilot", true},
		{"node", "node /srv/app/server.js", false},
	}
	for _, tc := range cases {
		kind, source, ok := classifyAgentProcess(tc.command, tc.args)
		if ok != tc.want {
			t.Fatalf("classify(%q, %q) matched=%v, want %v", tc.command, tc.args, ok, tc.want)
		}
		if tc.want && (kind != models.AgentKindCopilot || source != "cli") {
			t.Fatalf("classify(%q, %q) = %v/%v, want copilot/cli", tc.command, tc.args, kind, source)
		}
	}
}

func TestClassifyAgentProcessWindowsCodexWrapper(t *testing.T) {
	args := `node.exe C:\Users\me\AppData\Roaming\npm\node_modules\@openai\codex\bin\codex.js`
	kind, source, ok := classifyAgentProcess("node.exe", args)
	if !ok || kind != models.AgentKindCodex || source != "cli" {
		t.Fatalf("classify Windows Codex wrapper = %v/%v/%v, want codex/cli/true", kind, source, ok)
	}
}
