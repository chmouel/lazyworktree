package app

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

func TestBuildAgentSessionsContentRendersSessionCards(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	worktreePath := filepath.Join(t.TempDir(), "repo", "feature")
	m.state.data.filteredWts = []*models.WorktreeInfo{{Path: worktreePath, Branch: "feature"}}
	m.state.data.selectedIndex = 0
	m.state.ui.agentSessionsViewport.SetWidth(92)

	sessions := []*models.AgentSession{
		{
			ID:             "claude-open",
			Agent:          models.AgentKindClaude,
			DisplayName:    "Authoring",
			CWD:            filepath.Join(worktreePath, "cmd", "api"),
			TaskLabel:      "editing internal/app/app_agents.go",
			Model:          "claude-sonnet",
			GitBranch:      "feature",
			LastActivity:   time.Now(),
			Activity:       models.AgentActivityWriting,
			IsOpen:         true,
			OpenConfidence: models.AgentOpenConfidenceExact,
		},
		{
			ID:           "pi-offline",
			Agent:        models.AgentKindPi,
			DisplayName:  "Notes tidy",
			CWD:          worktreePath,
			LastActivity: time.Now().Add(-2 * time.Hour),
			Activity:     models.AgentActivityIdle,
			IsOpen:       false,
		},
	}

	content := m.buildAgentSessionsContent(sessions)
	if !strings.Contains(content, "\x1b[") {
		t.Fatal("expected styled output with ANSI sequences")
	}

	plain := ansi.Strip(content)
	for _, want := range []string{
		"Notes tidy",
		"WRITING",
		"IDLE",
		"editing internal/app/app_agents.go",
		"─",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("expected rendered content to contain %q, got %q", want, plain)
		}
	}

	if got := strings.Count(plain, "editing internal/app/app_agents.go"); got != 1 {
		t.Fatalf("expected task label to appear once as the title, got %d occurrences in %q", got, plain)
	}

	for _, unwanted := range []string{"OPEN", "CWD", "OFFLINE", "transcript match", "cwd match", "feature", "cmd/api", "claude-sonnet", "Authoring"} {
		if strings.Contains(plain, unwanted) {
			t.Fatalf("expected rendered content to omit %q, got %q", unwanted, plain)
		}
	}
}

func TestRenderAgentSessionCardSuppressesMetaWhenNarrow(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	worktreePath := filepath.Join(t.TempDir(), "repo", "feature")
	session := &models.AgentSession{
		ID:             "claude-open",
		Agent:          models.AgentKindClaude,
		DisplayName:    "Authoring",
		CWD:            filepath.Join(worktreePath, "cmd", "api"),
		Model:          "claude-sonnet",
		GitBranch:      "feature",
		LastActivity:   time.Now(),
		Activity:       models.AgentActivityWriting,
		IsOpen:         true,
		OpenConfidence: models.AgentOpenConfidenceExact,
	}

	lines := m.renderAgentSessionCard(session, 22, false)
	if len(lines) != 1 {
		t.Fatalf("expected a single compact line for narrow width, got %d", len(lines))
	}

	plain := ansi.Strip(strings.Join(lines, "\n"))
	for _, unwanted := range []string{"transcript match", "cwd match", "OPEN", "CWD"} {
		if strings.Contains(plain, unwanted) {
			t.Fatalf("expected narrow rendering to suppress %q, got %q", unwanted, plain)
		}
	}
}

func TestRenderAgentSessionCardSelectedUsesThinRail(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	worktreePath := filepath.Join(t.TempDir(), "repo", "feature")
	session := &models.AgentSession{
		ID:           "claude-open",
		Agent:        models.AgentKindClaude,
		DisplayName:  "Authoring",
		CWD:          filepath.Join(worktreePath, "cmd", "api"),
		LastActivity: time.Now(),
		Activity:     models.AgentActivityWaiting,
		IsOpen:       true,
	}

	lines := m.renderAgentSessionCard(session, 72, true)
	if len(lines) == 0 {
		t.Fatal("expected selected card output")
	}

	plain := ansi.Strip(lines[0])
	if !strings.HasPrefix(plain, "▏") {
		t.Fatalf("expected selected line to use a thin left rail, got %q", plain)
	}
	if strings.Contains(plain, "OPEN") || strings.Contains(plain, "CWD") {
		t.Fatalf("expected selected line to prioritise activity only, got %q", plain)
	}
}

func TestRenderAgentSessionCardFallsBackToDisplayNameWithoutTaskLabel(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	worktreePath := filepath.Join(t.TempDir(), "repo", "feature")
	session := &models.AgentSession{
		ID:           "claude-open",
		Agent:        models.AgentKindClaude,
		DisplayName:  "Authoring",
		CWD:          filepath.Join(worktreePath, "cmd", "api"),
		Model:        "claude-sonnet",
		LastActivity: time.Now(),
		Activity:     models.AgentActivityWaiting,
		IsOpen:       true,
	}

	lines := m.renderAgentSessionCard(session, 72, false)
	plain := ansi.Strip(strings.Join(lines, "\n"))
	if !strings.Contains(plain, "Authoring") {
		t.Fatalf("expected display name fallback when no task label exists, got %q", plain)
	}
	if strings.Contains(plain, "claude-sonnet") {
		t.Fatalf("expected model to stay hidden, got %q", plain)
	}
}

func TestRenderAgentSessionCardUsesGenericFallbackWithoutTaskOrDisplayName(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	worktreePath := filepath.Join(t.TempDir(), "repo", "feature")
	session := &models.AgentSession{
		ID:           "claude-open",
		Agent:        models.AgentKindClaude,
		CWD:          worktreePath,
		LastActivity: time.Now(),
		Activity:     models.AgentActivityWaiting,
		IsOpen:       true,
	}

	lines := m.renderAgentSessionCard(session, 72, false)
	plain := ansi.Strip(strings.Join(lines, "\n"))
	if !strings.Contains(plain, "Claude session") {
		t.Fatalf("expected generic session fallback title, got %q", plain)
	}
}

func TestRenderAgentSessionCardShowsApprovalBadge(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	session := &models.AgentSession{
		ID:           "claude-open",
		Agent:        models.AgentKindClaude,
		DisplayName:  "Authoring",
		LastActivity: time.Now(),
		Activity:     models.AgentActivityApproval,
		IsOpen:       true,
	}

	lines := m.renderAgentSessionCard(session, 72, false)
	plain := ansi.Strip(strings.Join(lines, "\n"))
	if !strings.Contains(plain, "APPROVAL") {
		t.Fatalf("expected approval badge, got %q", plain)
	}
}

func TestAgentSessionsForSelectedWorktreeIncludesSuspectByDefault(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	worktreePath := filepath.Join(t.TempDir(), "repo", "feature")
	m.state.data.filteredWts = []*models.WorktreeInfo{{Path: worktreePath, Branch: "feature"}}
	m.state.data.selectedIndex = 0
	m.state.ui.agentSessionsViewport.SetWidth(92)

	suspect := &models.AgentSession{
		ID:             "claude-suspect",
		Agent:          models.AgentKindClaude,
		DisplayName:    "Authoring",
		CWD:            worktreePath,
		Title:          "editing internal/app/app_agents.go",
		LastActivity:   time.Now(),
		Activity:       models.AgentActivityWriting,
		LivenessState:  models.AgentSessionLivenessSuspect,
		LivenessSource: models.AgentSessionLivenessSourceCWDHeuristic,
		OpenConfidence: models.AgentOpenConfidenceCWD,
	}

	m.state.view.ShowAllAgentSessions = false
	visible := []*models.AgentSession{suspect}
	content := m.buildAgentSessionsContent(visible)
	plain := ansi.Strip(content)
	if !strings.Contains(plain, "CWD") {
		t.Fatalf("expected suspect session badge to render by default, got %q", plain)
	}
}

func TestRenderAgentSessionMarkerUsesNerdFontGlyphForClaude(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir(), IconSet: "nerd-font-v3"}
	m := NewModel(cfg, "")

	marker := ansi.Strip(m.renderAgentSessionMarker(&models.AgentSession{Agent: models.AgentKindClaude}))
	if marker != "✻" {
		t.Fatalf("expected nerd font Claude marker, got %q", marker)
	}
}

func TestRenderAgentSessionMarkerUsesTextGlyphForClaude(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir(), IconSet: "text"}
	m := NewModel(cfg, "")

	marker := ansi.Strip(m.renderAgentSessionMarker(&models.AgentSession{Agent: models.AgentKindClaude}))
	if marker != "C" {
		t.Fatalf("expected text Claude marker, got %q", marker)
	}
}

func TestAgentSessionsEqual(t *testing.T) {
	now := time.Now()
	a := &models.AgentSession{ID: "one", CWD: "/tmp/wt", Status: models.AgentSessionStatusWaitingForUser, LastActivity: now}
	b := &models.AgentSession{ID: "one", CWD: "/tmp/wt", Status: models.AgentSessionStatusWaitingForUser, LastActivity: now}
	c := &models.AgentSession{ID: "two", CWD: "/tmp/wt"}

	if !agentSessionsEqual(nil, nil) {
		t.Fatal("expected nil slices to be equal")
	}
	if !agentSessionsEqual([]*models.AgentSession{a}, []*models.AgentSession{b}) {
		t.Fatal("expected identical session values to be equal")
	}
	if agentSessionsEqual([]*models.AgentSession{a}, []*models.AgentSession{c}) {
		t.Fatal("expected differing sessions to be unequal")
	}
	if agentSessionsEqual([]*models.AgentSession{a}, nil) {
		t.Fatal("expected differing lengths to be unequal")
	}
	if !agentSessionsEqual([]*models.AgentSession{nil}, []*models.AgentSession{nil}) {
		t.Fatal("expected nil entries to be equal")
	}
	if agentSessionsEqual([]*models.AgentSession{nil}, []*models.AgentSession{a}) {
		t.Fatal("expected nil versus value entry to be unequal")
	}
}

func TestAgentSessionsUpdatedMsgSkipsUnchangedSnapshot(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	current := []*models.AgentSession{{ID: "one", CWD: "/tmp/wt"}}
	m.state.data.agentSessionsSnapshot = current

	clone := []*models.AgentSession{{ID: "one", CWD: "/tmp/wt"}}
	_, _ = m.Update(agentSessionsUpdatedMsg{sessions: clone})
	if &m.state.data.agentSessionsSnapshot[0] != &current[0] {
		t.Fatal("expected unchanged snapshot to keep existing state")
	}

	changed := []*models.AgentSession{{ID: "two", CWD: "/tmp/wt"}}
	_, _ = m.Update(agentSessionsUpdatedMsg{sessions: changed})
	if m.state.data.agentSessionsSnapshot[0].ID != "two" {
		t.Fatal("expected changed snapshot to replace state")
	}
}
