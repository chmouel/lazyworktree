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
