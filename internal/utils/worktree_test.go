package utils

import (
	"testing"

	"github.com/chmouel/lazyworktree/internal/models"
)

func TestSanitizeBranchName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		maxLength int
		want      string
	}{
		{name: "trims and lowercases", input: "  Hello World  ", maxLength: 0, want: "hello-world"},
		{name: "collapses separators", input: "a---b___c", maxLength: 0, want: "a-b-c"},
		{name: "trims hyphens", input: "---a---", maxLength: 0, want: "a"},
		{name: "limits length", input: "abcd-efgh", maxLength: 4, want: "abcd"},
		{name: "trailing hyphen removed after truncation", input: "abcd-efgh", maxLength: 5, want: "abcd"},
		{name: "empty after sanitise", input: "!!!", maxLength: 0, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeBranchName(tt.input, tt.maxLength)
			if got != tt.want {
				t.Fatalf("SanitizeBranchName(%q, %d) = %q, want %q", tt.input, tt.maxLength, got, tt.want)
			}
		})
	}
}

func TestApplyWorktreeTemplate_TruncatesAndTrims(t *testing.T) {
	t.Parallel()

	name := applyWorktreeTemplate("pr-{number}-{title}", []placeholderReplacement{
		{placeholder: "{number}", value: "1"},
		{placeholder: "{title}", value: ""},
	})
	if name != "pr-1" {
		t.Fatalf("applyWorktreeTemplate trimmed result = %q, want %q", name, "pr-1")
	}

	long := applyWorktreeTemplate("x{title}", []placeholderReplacement{
		{placeholder: "{title}", value: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
	})
	if len(long) != 100 {
		t.Fatalf("applyWorktreeTemplate length = %d, want %d", len(long), 100)
	}
}

func TestGeneratePRWorktreeName_UsesGeneratedTitleWhenProvided(t *testing.T) {
	t.Parallel()

	pr := &models.PRInfo{Number: 2, Title: "Original Title", Author: "alice"}
	got := GeneratePRWorktreeName(pr, "pr-{number}-{generated}-{title}", "feat-session")
	want := "pr-2-feat-session-original-title"
	if got != want {
		t.Fatalf("GeneratePRWorktreeName() = %q, want %q", got, want)
	}
}

func TestGenerateIssueWorktreeName_UsesGeneratedTitleWhenProvided(t *testing.T) {
	t.Parallel()

	issue := &models.IssueInfo{Number: 3, Title: "Original Title"}
	got := GenerateIssueWorktreeName(issue, "issue-{number}-{generated}-{title}", "fix-bug")
	want := "issue-3-fix-bug-original-title"
	if got != want {
		t.Fatalf("GenerateIssueWorktreeName() = %q, want %q", got, want)
	}
}
