package app

import (
	"strings"
	"testing"

	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

func TestBuildInfoContentPRNumberUsesOSCHyperlink(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = t.TempDir()
	m := NewModel(cfg, "")

	wt := &models.WorktreeInfo{
		Path:   "/tmp/wt",
		Branch: "feature/hyperlink",
		PR: &models.PRInfo{
			Number: 2446,
			State:  "OPEN",
			Title:  "Clickable PR number",
			URL:    "https://example.com/org/repo/pull/2446",
		},
	}

	info := m.buildInfoContent(wt)
	if !strings.Contains(info, osc8Hyperlink("#2446", wt.PR.URL)) {
		t.Fatalf("expected OSC-8 hyperlink for PR number, got %q", info)
	}
}

func TestBuildInfoContentPRNumberWithoutURLUsesPlainText(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = t.TempDir()
	m := NewModel(cfg, "")

	wt := &models.WorktreeInfo{
		Path:   "/tmp/wt",
		Branch: "feature/plain-number",
		PR: &models.PRInfo{
			Number: 88,
			State:  "OPEN",
			Title:  "No URL available",
		},
	}

	info := m.buildInfoContent(wt)
	if !strings.Contains(info, "#88") {
		t.Fatalf("expected plain PR number, got %q", info)
	}
	if strings.Contains(info, "\x1b]8;;") {
		t.Fatalf("did not expect OSC-8 hyperlink sequence without PR URL, got %q", info)
	}
}

func TestOSC8HyperlinkEmptyURLReturnsPlainText(t *testing.T) {
	got := osc8Hyperlink("#123", "   ")
	if got != "#123" {
		t.Fatalf("expected plain text for empty URL, got %q", got)
	}
}

func TestBuildInfoContentMainBranchWithoutPRHidesFetchHint(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = t.TempDir()
	m := NewModel(cfg, "")
	m.prDataLoaded = false

	mainWt := &models.WorktreeInfo{
		Path:        "/tmp/main",
		Branch:      "main",
		IsMain:      true,
		HasUpstream: true,
	}
	m.state.data.worktrees = []*models.WorktreeInfo{mainWt}

	info := m.buildInfoContent(mainWt)
	if strings.Contains(info, "Press 'p' to fetch PR data") {
		t.Fatalf("did not expect fetch hint on main branch, got %q", info)
	}
	if !strings.Contains(info, "Main branch usually has no PR") {
		t.Fatalf("expected main-branch message, got %q", info)
	}
}

func TestBuildInfoContentFeatureBranchShowsFetchHint(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = t.TempDir()
	m := NewModel(cfg, "")
	m.prDataLoaded = false

	mainWt := &models.WorktreeInfo{
		Path:        "/tmp/main",
		Branch:      "main",
		IsMain:      true,
		HasUpstream: true,
	}
	featureWt := &models.WorktreeInfo{
		Path:        "/tmp/feature",
		Branch:      "feature/test",
		IsMain:      false,
		HasUpstream: true,
	}
	m.state.data.worktrees = []*models.WorktreeInfo{mainWt, featureWt}

	info := m.buildInfoContent(featureWt)
	if !strings.Contains(info, "Press 'p' to fetch PR data") {
		t.Fatalf("expected fetch hint for feature branch, got %q", info)
	}
}

func TestBuildInfoContentNoUpstreamHidesPRSection(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = t.TempDir()
	m := NewModel(cfg, "")
	m.prDataLoaded = true

	wt := &models.WorktreeInfo{
		Path:        "/tmp/no-upstream",
		Branch:      "local-only",
		HasUpstream: false,
		PR:          nil,
	}
	m.state.data.worktrees = []*models.WorktreeInfo{wt}

	info := m.buildInfoContent(wt)
	if strings.Contains(info, "PR:") {
		t.Fatalf("did not expect PR section for branch without upstream, got %q", info)
	}
	if strings.Contains(info, "Branch has no upstream") {
		t.Fatalf("did not expect no-upstream PR message, got %q", info)
	}
}
