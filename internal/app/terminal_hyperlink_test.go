package app

import (
	"regexp"
	"strings"
	"testing"

	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

var (
	ansiEscapeRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	osc8EscapeRegex = regexp.MustCompile(`\x1b]8;;[^\x1b]*\x1b\\`)
)

func stripANSISequences(s string) string {
	return ansiEscapeRegex.ReplaceAllString(s, "")
}

func stripTerminalSequences(s string) string {
	return stripANSISequences(osc8EscapeRegex.ReplaceAllString(s, ""))
}

func TestBuildInfoContentPRNumberWithURLUsesPlainText(t *testing.T) {
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
	if !strings.Contains(info, "#2446") {
		t.Fatalf("expected PR number text, got %q", info)
	}
	if strings.Contains(info, "\x1b]8;;") {
		t.Fatalf("did not expect OSC-8 hyperlink sequence in PR header, got %q", info)
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
	if strings.Contains(info, "Press 'r' to refresh and fetch PR data") {
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
	if !strings.Contains(info, "Press 'r' to refresh and fetch PR data") {
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

func TestBuildInfoContentAnnotationKeywordsUppercaseWithIconTextSet(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = t.TempDir()
	cfg.IconSet = "text"
	m := NewModel(cfg, "")

	wt := &models.WorktreeInfo{
		Path:   "/tmp/wt",
		Branch: "feature/annotations",
	}
	m.worktreeNotes[worktreeNoteKey(wt.Path)] = models.WorktreeNote{
		Note: "please FIXME now and WARNING: review, TODO later, todo ignored",
	}

	info := m.buildInfoContent(wt)
	plain := stripANSISequences(info)
	if !strings.Contains(plain, "Notes:") {
		t.Fatalf("expected notes section, got %q", plain)
	}
	if !strings.Contains(plain, "[!] FIXME") {
		t.Fatalf("expected FIXME keyword badge, got %q", plain)
	}
	if !strings.Contains(plain, "[!] WARNING:") {
		t.Fatalf("expected WARNING keyword badge with colon, got %q", plain)
	}
	if !strings.Contains(plain, "[ ] TODO") {
		t.Fatalf("expected TODO keyword badge, got %q", plain)
	}
	if !strings.Contains(plain, "todo ignored") {
		t.Fatalf("expected lowercase todo text to remain unchanged, got %q", plain)
	}
	if strings.Contains(plain, "[!] FIX ") || strings.Contains(plain, "[!] WARN:") || strings.Contains(plain, "[ ] todo") {
		t.Fatalf("did not expect canonical rewriting or lowercase highlighting, got %q", plain)
	}
}

func TestBuildInfoContentAnnotationKeywordWordBoundary(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = t.TempDir()
	cfg.IconSet = "text"
	m := NewModel(cfg, "")

	wt := &models.WorktreeInfo{
		Path:   "/tmp/wt-boundary",
		Branch: "feature/annotations-boundary",
	}
	m.worktreeNotes[worktreeNoteKey(wt.Path)] = models.WorktreeNote{
		Note: "methododo should stay untouched",
	}

	info := m.buildInfoContent(wt)
	plain := stripANSISequences(info)
	if strings.Contains(plain, "[ ] TODO") {
		t.Fatalf("did not expect partial word match for TODO, got %q", plain)
	}
	if !strings.Contains(plain, "methododo should stay untouched") {
		t.Fatalf("expected line content to stay unchanged, got %q", plain)
	}
}

func TestBuildInfoContentAnnotationKeywordsNerdFontIcons(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = t.TempDir()
	cfg.IconSet = "nerd-font-v3"
	m := NewModel(cfg, "")

	wt := &models.WorktreeInfo{
		Path:   "/tmp/wt-nerd",
		Branch: "feature/annotations-nerd",
	}
	m.worktreeNotes[worktreeNoteKey(wt.Path)] = models.WorktreeNote{
		Note: "BUG and TEST:",
	}

	info := m.buildInfoContent(wt)
	plain := stripANSISequences(info)
	if !strings.Contains(plain, " BUG") {
		t.Fatalf("expected BUG nerd-font icon badge, got %q", plain)
	}
	if !strings.Contains(plain, "⏲ TEST:") {
		t.Fatalf("expected TEST nerd-font icon badge with colon, got %q", plain)
	}
}

func TestBuildInfoContentNotesRenderMarkdown(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = t.TempDir()
	cfg.IconSet = "text"
	m := NewModel(cfg, "")

	wt := &models.WorktreeInfo{
		Path:   "/tmp/wt-md",
		Branch: "feature/notes-markdown",
	}
	m.worktreeNotes[worktreeNoteKey(wt.Path)] = models.WorktreeNote{
		Note: "# Heading\n**Implementation Notes:**\n- **Problem:** `create_comment` was called repeatedly\n1. second item\n> quoted line\n[docs](https://example.com/docs)\n```go\nTODO: inside code fence\n```",
	}

	info := m.buildInfoContent(wt)
	// Check the hyperlink URL is present (exact OSC8 formatting may vary with lipgloss styling)
	if !strings.Contains(info, "https://example.com/docs") {
		t.Fatalf("expected markdown link URL in output, got %q", info)
	}
	if !strings.Contains(info, osc8OpenPrefix) {
		t.Fatalf("expected OSC8 hyperlink prefix in output, got %q", info)
	}

	plain := stripTerminalSequences(info)
	if strings.Contains(plain, "# Heading") {
		t.Fatalf("expected heading marker to be stripped, got %q", plain)
	}
	if !strings.Contains(plain, "Heading") {
		t.Fatalf("expected heading text to be shown, got %q", plain)
	}
	if strings.Contains(plain, "**Implementation Notes:**") {
		t.Fatalf("expected bold markdown markers to be removed, got %q", plain)
	}
	if !strings.Contains(plain, "Implementation Notes:") {
		t.Fatalf("expected bold markdown text to remain visible, got %q", plain)
	}
	if !strings.Contains(plain, "- Problem: create_comment was called repeatedly") {
		t.Fatalf("expected markdown bullet list item, got %q", plain)
	}
	if strings.Contains(plain, "`create_comment`") {
		t.Fatalf("expected inline code markers to be removed, got %q", plain)
	}
	if !strings.Contains(plain, "create_comment") {
		t.Fatalf("expected inline code content to remain visible, got %q", plain)
	}
	if !strings.Contains(plain, "1. second item") {
		t.Fatalf("expected markdown ordered list item, got %q", plain)
	}
	if !strings.Contains(plain, "| quoted line") {
		t.Fatalf("expected markdown blockquote, got %q", plain)
	}
	if strings.Contains(plain, "[docs](https://example.com/docs)") {
		t.Fatalf("expected markdown link syntax to be removed, got %q", plain)
	}
	if strings.Contains(plain, "```") {
		t.Fatalf("expected code fence markers to be hidden, got %q", plain)
	}
	if !strings.Contains(plain, "TODO: inside code fence") {
		t.Fatalf("expected code fence content to remain visible, got %q", plain)
	}
	if strings.Contains(plain, "[ ] TODO: inside code fence") {
		t.Fatalf("did not expect annotation keyword replacement inside fenced code, got %q", plain)
	}
}
