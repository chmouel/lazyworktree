package app

import (
	"context"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

func TestParseCommitMetaComplete(t *testing.T) {
	t.Parallel()
	// Test with complete commit metadata (format: SHA\x1fAuthor\x1fEmail\x1fDate\x1fSubject\x1fBody)
	raw := "d25c6aa6b03f571cf0714e7a56a49053c3bdebf0\x1fChmouel Boudjnah\x1fchmouel@chmouel.com\x1fMon Dec 29 19:33:24 2025 +0100\x1ffeat: Add prune merged worktrees command\x1fIntroduced a new command to automatically identify and prune worktrees\nassociated with merged pull or merge requests. This feature helps maintain a\nclean and organized workspace by removing obsolete worktrees, thereby improving\nefficiency. The command prompts for confirmation before proceeding with the\ndeletion of any identified merged worktrees.\n\nSigned-off-by: Chmouel Boudjnah <chmouel@chmouel.com>"

	meta := parseCommitMeta(raw)

	if meta.sha != "d25c6aa6b03f571cf0714e7a56a49053c3bdebf0" {
		t.Errorf("Expected SHA 'd25c6aa6b03f571cf0714e7a56a49053c3bdebf0', got %q", meta.sha)
	}
	if meta.author != "Chmouel Boudjnah" {
		t.Errorf("Expected author 'Chmouel Boudjnah', got %q", meta.author)
	}
	if meta.email != "chmouel@chmouel.com" {
		t.Errorf("Expected email 'chmouel@chmouel.com', got %q", meta.email)
	}
	if meta.date != "Mon Dec 29 19:33:24 2025 +0100" {
		t.Errorf("Expected date 'Mon Dec 29 19:33:24 2025 +0100', got %q", meta.date)
	}
	if meta.subject != "feat: Add prune merged worktrees command" {
		t.Errorf("Expected subject 'feat: Add prune merged worktrees command', got %q", meta.subject)
	}
	if len(meta.body) == 0 {
		t.Fatal("Expected body to be non-empty")
	}
	bodyText := strings.Join(meta.body, "\n")
	if !strings.Contains(bodyText, "Introduced a new command") {
		t.Errorf("Expected body to contain 'Introduced a new command', got %q", bodyText)
	}
	if !strings.Contains(bodyText, "Signed-off-by") {
		t.Errorf("Expected body to contain 'Signed-off-by', got %q", bodyText)
	}
}

func TestParseCommitMetaMinimal(t *testing.T) {
	t.Parallel()
	// Test with minimal commit metadata (only SHA)
	raw := randomSHA
	meta := parseCommitMeta(raw)

	if meta.sha != raw {
		t.Errorf("Expected SHA 'abc123', got %q", meta.sha)
	}
	if meta.author != "" {
		t.Errorf("Expected empty author, got %q", meta.author)
	}
	if meta.email != "" {
		t.Errorf("Expected empty email, got %q", meta.email)
	}
	if meta.date != "" {
		t.Errorf("Expected empty date, got %q", meta.date)
	}
	if meta.subject != "" {
		t.Errorf("Expected empty subject, got %q", meta.subject)
	}
	if len(meta.body) != 0 {
		t.Errorf("Expected empty body, got %v", meta.body)
	}
}

func TestParseCommitMetaNoBody(t *testing.T) {
	t.Parallel()
	// Test with commit metadata but no body (format: SHA\x1fAuthor\x1fEmail\x1fDate\x1fSubject)
	raw := "abc123\x1fJohn Doe\x1fjohn@example.com\x1fMon Jan 1 00:00:00 2025 +0000\x1ffix: Bug fix"

	meta := parseCommitMeta(raw)

	if meta.sha != randomSHA {
		t.Errorf("Expected SHA 'abc123', got %q", meta.sha)
	}
	if meta.author != "John Doe" {
		t.Errorf("Expected author 'John Doe', got %q", meta.author)
	}
	if meta.email != "john@example.com" {
		t.Errorf("Expected email 'john@example.com', got %q", meta.email)
	}
	if meta.date != "Mon Jan 1 00:00:00 2025 +0000" {
		t.Errorf("Expected date 'Mon Jan 1 00:00:00 2025 +0000', got %q", meta.date)
	}
	if meta.subject != "fix: Bug fix" {
		t.Errorf("Expected subject 'fix: Bug fix', got %q", meta.subject)
	}
	if len(meta.body) != 0 {
		t.Errorf("Expected empty body, got %v", meta.body)
	}
}

func TestTruncateToHeightFromEnd(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		maxLines int
		expected string
	}{
		{
			name:     "fewer lines than max",
			input:    "line1\nline2",
			maxLines: 5,
			expected: "line1\nline2",
		},
		{
			name:     "exactly max lines",
			input:    "line1\nline2\nline3",
			maxLines: 3,
			expected: "line1\nline2\nline3",
		},
		{
			name:     "more lines than max",
			input:    "line1\nline2\nline3\nline4\nline5",
			maxLines: 3,
			expected: "line3\nline4\nline5",
		},
		{
			name:     "single line",
			input:    "single line",
			maxLines: 1,
			expected: "single line",
		},
		{
			name:     "empty string",
			input:    "",
			maxLines: 5,
			expected: "",
		},
		{
			name:     "maxLines zero",
			input:    "line1\nline2",
			maxLines: 0,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateToHeightFromEnd(tt.input, tt.maxLines)
			if result != tt.expected {
				t.Errorf("truncateToHeightFromEnd(%q, %d) = %q, want %q", tt.input, tt.maxLines, result, tt.expected)
			}
		})
	}
}

func TestGetCachedDetailsCachesResults(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	wt := &models.WorktreeInfo{Path: t.TempDir()}

	var mu sync.Mutex
	callCounts := map[string]int{}
	// #nosec G702 -- test helper with fixed command arguments
	m.state.services.git.SetCommandRunner(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		key := name + " " + strings.Join(args, " ")
		mu.Lock()
		callCounts[key]++
		mu.Unlock()

		switch key {
		case "git symbolic-ref --short refs/remotes/origin/HEAD":
			return exec.CommandContext(ctx, "echo", "-n", "origin/main") //nolint:gosec
		case "git status --porcelain=v2":
			return exec.CommandContext(ctx, "echo", "-n", "") //nolint:gosec
		case "git log -50 --pretty=format:%H%x09%an%x09%s":
			return exec.CommandContext(ctx, "echo", "-n", "abc123\talice\tCommit title") //nolint:gosec
		case "git rev-list -100 HEAD --not --remotes":
			return exec.CommandContext(ctx, "echo", "-n", "unpushedsha") //nolint:gosec
		case "git rev-list -100 HEAD ^main":
			return exec.CommandContext(ctx, "echo", "-n", "unmergedsha") //nolint:gosec
		default:
			return exec.CommandContext(ctx, "echo", "-n", "") //nolint:gosec
		}
	})

	statusRaw, logRaw, unpushedSHAs, unmergedSHAs := m.getCachedDetails(wt)
	if statusRaw != "" {
		t.Fatalf("expected empty status raw, got %q", statusRaw)
	}
	if logRaw == "" {
		t.Fatal("expected log raw to be populated")
	}
	if !unpushedSHAs["unpushedsha"] {
		t.Fatalf("expected unpushed SHA to be tracked, got %v", unpushedSHAs)
	}
	if !unmergedSHAs["unmergedsha"] {
		t.Fatalf("expected unmerged SHA to be tracked, got %v", unmergedSHAs)
	}

	_, _, _, _ = m.getCachedDetails(wt)

	mu.Lock()
	defer mu.Unlock()
	if callCounts["git symbolic-ref --short refs/remotes/origin/HEAD"] != 1 {
		t.Fatalf("expected GetMainBranch git call once, got %d", callCounts["git symbolic-ref --short refs/remotes/origin/HEAD"])
	}
	if callCounts["git status --porcelain=v2"] != 1 {
		t.Fatalf("expected status git call once, got %d", callCounts["git status --porcelain=v2"])
	}
	if callCounts["git log -50 --pretty=format:%H%x09%an%x09%s"] != 1 {
		t.Fatalf("expected log git call once, got %d", callCounts["git log -50 --pretty=format:%H%x09%an%x09%s"])
	}
	if callCounts["git rev-list -100 HEAD --not --remotes"] != 1 {
		t.Fatalf("expected unpushed git call once, got %d", callCounts["git rev-list -100 HEAD --not --remotes"])
	}
	if callCounts["git rev-list -100 HEAD ^main"] != 1 {
		t.Fatalf("expected unmerged git call once, got %d", callCounts["git rev-list -100 HEAD ^main"])
	}
}
