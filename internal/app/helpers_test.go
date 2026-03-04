package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/utils"
)

func TestGeneratePRWorktreeName(t *testing.T) {
	tests := []struct {
		name     string
		pr       *models.PRInfo
		expected string
	}{
		{
			name: "simple title",
			pr: &models.PRInfo{
				Number: 123,
				Title:  "Add feature",
			},
			expected: "pr-123-add-feature",
		},
		{
			name: "title with special characters",
			pr: &models.PRInfo{
				Number: 2367,
				Title:  "Feat: Add one-per-pipeline comment strategy!",
			},
			expected: "pr-2367-feat-add-one-per-pipeline-comment-strategy",
		},
		{
			name: "long title gets truncated",
			pr: &models.PRInfo{
				Number: 999,
				Title:  "This is a very long title that should be truncated because it exceeds the maximum length limit of one hundred characters total including the pr prefix",
			},
			// Result should be exactly 100 chars or less, and not end with hyphen
			expected: "pr-999-this-is-a-very-long-title-that-should-be-truncated-because-it-exceeds-the-maximum-length-limit",
		},
		{
			name: "title with multiple spaces",
			pr: &models.PRInfo{
				Number: 456,
				Title:  "Fix   multiple    spaces",
			},
			expected: "pr-456-fix-multiple-spaces",
		},
		{
			name: "title with numbers and symbols",
			pr: &models.PRInfo{
				Number: 789,
				Title:  "Update v2.0 API (breaking changes)",
			},
			expected: "pr-789-update-v2-0-api-breaking-changes",
		},
		{
			name: "empty title",
			pr: &models.PRInfo{
				Number: 100,
				Title:  "",
			},
			expected: "pr-100",
		},
		{
			name: "title with only special characters",
			pr: &models.PRInfo{
				Number: 200,
				Title:  "!!!@@@###$$$",
			},
			expected: "pr-200",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use default template format
			result := utils.GeneratePRWorktreeName(tt.pr, "pr-{number}-{title}", "")
			// For the long title test, just verify it's <= 100 chars and doesn't end with hyphen
			if tt.name == "long title gets truncated" {
				if len(result) > 100 {
					t.Errorf("utils.GeneratePRWorktreeName() result length = %d, want <= 100", len(result))
				}
				if strings.HasSuffix(result, "-") {
					t.Errorf("utils.GeneratePRWorktreeName() result ends with hyphen: %q", result)
				}
			} else if result != tt.expected {
				t.Errorf("utils.GeneratePRWorktreeName() = %q, want %q", result, tt.expected)
			}
			// Ensure result is max 100 chars
			if len(result) > 100 {
				t.Errorf("utils.GeneratePRWorktreeName() result length = %d, want <= 100", len(result))
			}
		})
	}

	// Test custom templates
	t.Run("template format without hyphen", func(t *testing.T) {
		pr := &models.PRInfo{
			Number: 123,
			Title:  "Add feature",
		}
		result := utils.GeneratePRWorktreeName(pr, "pr{number}-{title}", "")
		expected := "pr123-add-feature"
		if result != expected {
			t.Errorf("utils.GeneratePRWorktreeName() with custom template = %q, want %q", result, expected)
		}
	})

	t.Run("custom template without prefix", func(t *testing.T) {
		pr := &models.PRInfo{
			Number: 456,
			Title:  "Fix bug",
		}
		result := utils.GeneratePRWorktreeName(pr, "{number}-{title}", "")
		expected := "456-fix-bug"
		if result != expected {
			t.Errorf("utils.GeneratePRWorktreeName() with custom template = %q, want %q", result, expected)
		}
	})

	t.Run("custom prefix", func(t *testing.T) {
		pr := &models.PRInfo{
			Number: 789,
			Title:  "Update docs",
		}
		result := utils.GeneratePRWorktreeName(pr, "pull{number}-{title}", "")
		expected := "pull789-update-docs"
		if result != expected {
			t.Errorf("utils.GeneratePRWorktreeName() with custom prefix = %q, want %q", result, expected)
		}
	})

	t.Run("template with pr_author placeholder", func(t *testing.T) {
		pr := &models.PRInfo{
			Number: 456,
			Title:  "Fix bug",
			Author: "alice",
		}
		result := utils.GeneratePRWorktreeName(pr, "pr-{pr_author}-{number}-{title}", "")
		expected := "pr-alice-456-fix-bug"
		if result != expected {
			t.Errorf("utils.GeneratePRWorktreeName() with pr_author = %q, want %q", result, expected)
		}
	})

	t.Run("template with pr_author containing special chars", func(t *testing.T) {
		pr := &models.PRInfo{
			Number: 789,
			Title:  "Update docs",
			Author: "user@bot",
		}
		result := utils.GeneratePRWorktreeName(pr, "pr-{pr_author}-{number}-{title}", "")
		expected := "pr-user-bot-789-update-docs"
		if result != expected {
			t.Errorf("utils.GeneratePRWorktreeName() with special chars in author = %q, want %q", result, expected)
		}
	})

	t.Run("template with empty pr_author", func(t *testing.T) {
		pr := &models.PRInfo{
			Number: 100,
			Title:  "Feature",
			Author: "",
		}
		result := utils.GeneratePRWorktreeName(pr, "pr-{pr_author}-{number}-{title}", "")
		expected := "pr--100-feature"
		if result != expected {
			t.Errorf("utils.GeneratePRWorktreeName() with empty author = %q, want %q", result, expected)
		}
	})

	t.Run("template with pr_author at start", func(t *testing.T) {
		pr := &models.PRInfo{
			Number: 123,
			Title:  "New feature",
			Author: "bob",
		}
		result := utils.GeneratePRWorktreeName(pr, "{pr_author}-{number}-{title}", "")
		expected := "bob-123-new-feature"
		if result != expected {
			t.Errorf("utils.GeneratePRWorktreeName() with author at start = %q, want %q", result, expected)
		}
	})
}

func TestGenerateIssueWorktreeName(t *testing.T) {
	tests := []struct {
		name     string
		issue    *models.IssueInfo
		template string
		expected string
	}{
		{
			name: "simple title with default template",
			issue: &models.IssueInfo{
				Number: 123,
				Title:  "Fix login bug",
			},
			template: "issue-{number}-{title}",
			expected: "issue-123-fix-login-bug",
		},
		{
			name: "title with special characters",
			issue: &models.IssueInfo{
				Number: 456,
				Title:  "Bug: Application crashes on startup!",
			},
			template: "issue-{number}-{title}",
			expected: "issue-456-bug-application-crashes-on-startup",
		},
		{
			name: "custom template format",
			issue: &models.IssueInfo{
				Number: 789,
				Title:  "Add new feature",
			},
			template: "{number}-{title}",
			expected: "789-add-new-feature",
		},
		{
			name: "empty title",
			issue: &models.IssueInfo{
				Number: 100,
				Title:  "",
			},
			template: "issue-{number}-{title}",
			expected: "issue-100",
		},
		{
			name: "long title gets truncated",
			issue: &models.IssueInfo{
				Number: 999,
				Title:  "This is a very long issue title that should be truncated because it exceeds the maximum length limit of one hundred characters",
			},
			template: "issue-{number}-{title}",
			expected: "issue-999-this-is-a-very-long-issue-title-that-should-be-truncated-because-it-exceeds-the-maxi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := utils.GenerateIssueWorktreeName(tt.issue, tt.template, "")
			if tt.name == "long title gets truncated" {
				if len(result) > 100 {
					t.Errorf("utils.GenerateIssueWorktreeName() result length = %d, want <= 100", len(result))
				}
				if strings.HasSuffix(result, "-") {
					t.Errorf("utils.GenerateIssueWorktreeName() result ends with hyphen: %q", result)
				}
			} else if result != tt.expected {
				t.Errorf("utils.GenerateIssueWorktreeName() = %q, want %q", result, tt.expected)
			}
			// Ensure result is max 100 chars
			if len(result) > 100 {
				t.Errorf("utils.GenerateIssueWorktreeName() result length = %d, want <= 100", len(result))
			}
		})
	}
}

func TestGeneratePRWorktreeNameWithGenerated(t *testing.T) {
	tests := []struct {
		name           string
		pr             *models.PRInfo
		template       string
		generatedTitle string
		expected       string
	}{
		{
			name: "{generated} with AI title provided",
			pr: &models.PRInfo{
				Number: 123,
				Title:  "Add new feature",
			},
			template:       "pr-{number}-{generated}",
			generatedTitle: "feat-session-manager",
			expected:       "pr-123-feat-session-manager",
		},
		{
			name: "{generated} falls back to {title} when empty",
			pr: &models.PRInfo{
				Number: 456,
				Title:  "Fix login bug",
			},
			template:       "pr-{number}-{generated}",
			generatedTitle: "",
			expected:       "pr-456-fix-login-bug",
		},
		{
			name: "both {title} and {generated} in template",
			pr: &models.PRInfo{
				Number: 789,
				Title:  "Update documentation",
			},
			template:       "pr-{number}-{generated}-from-{title}",
			generatedTitle: "docs-update",
			expected:       "pr-789-docs-update-from-update-documentation",
		},
		{
			name: "{title} still works without {generated}",
			pr: &models.PRInfo{
				Number: 100,
				Title:  "Refactor code",
			},
			template:       "pr-{number}-{title}",
			generatedTitle: "refactor-core",
			expected:       "pr-100-refactor-code",
		},
		{
			name: "{generated} with special characters",
			pr: &models.PRInfo{
				Number: 200,
				Title:  "Original Title!",
			},
			template:       "pr-{number}-{generated}",
			generatedTitle: "Feat: Add AI Support!",
			expected:       "pr-200-feat-add-ai-support",
		},
		{
			name: "long {generated} title gets truncated",
			pr: &models.PRInfo{
				Number: 999,
				Title:  "Short",
			},
			template:       "pr-{number}-{generated}",
			generatedTitle: "This is a very long generated title that should be truncated because it exceeds the maximum length",
			expected:       "pr-999-this-is-a-very-long-generated-title-that-should-be-truncated-because-it-exceeds-the",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := utils.GeneratePRWorktreeName(tt.pr, tt.template, tt.generatedTitle)
			if tt.name == "long {generated} title gets truncated" {
				if len(result) > 100 {
					t.Errorf("utils.GeneratePRWorktreeName() result length = %d, want <= 100", len(result))
				}
				if strings.HasSuffix(result, "-") {
					t.Errorf("utils.GeneratePRWorktreeName() result ends with hyphen: %q", result)
				}
			} else if result != tt.expected {
				t.Errorf("utils.GeneratePRWorktreeName() = %q, want %q", result, tt.expected)
			}
			// Ensure result is max 100 chars
			if len(result) > 100 {
				t.Errorf("utils.GeneratePRWorktreeName() result length = %d, want <= 100", len(result))
			}
		})
	}
}

func TestGenerateIssueWorktreeNameWithGenerated(t *testing.T) {
	tests := []struct {
		name           string
		issue          *models.IssueInfo
		template       string
		generatedTitle string
		expected       string
	}{
		{
			name: "{generated} with AI title provided",
			issue: &models.IssueInfo{
				Number: 123,
				Title:  "Bug in login system",
			},
			template:       "issue-{number}-{generated}",
			generatedTitle: "fix-auth-bug",
			expected:       "issue-123-fix-auth-bug",
		},
		{
			name: "{generated} falls back to {title} when empty",
			issue: &models.IssueInfo{
				Number: 456,
				Title:  "Feature request",
			},
			template:       "issue-{number}-{generated}",
			generatedTitle: "",
			expected:       "issue-456-feature-request",
		},
		{
			name: "both {title} and {generated} in template",
			issue: &models.IssueInfo{
				Number: 789,
				Title:  "Performance issue",
			},
			template:       "issue-{number}-{generated}-{title}",
			generatedTitle: "perf-opt",
			expected:       "issue-789-perf-opt-performance-issue",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := utils.GenerateIssueWorktreeName(tt.issue, tt.template, tt.generatedTitle)
			if result != tt.expected {
				t.Errorf("utils.GenerateIssueWorktreeName() = %q, want %q", result, tt.expected)
			}
			// Ensure result is max 100 chars
			if len(result) > 100 {
				t.Errorf("utils.GenerateIssueWorktreeName() result length = %d, want <= 100", len(result))
			}
		})
	}
}

func TestFuzzyScoreLower(t *testing.T) {
	tests := []struct {
		name          string
		query         string // will be lowercased in test
		target        string // will be lowercased in test
		wantOk        bool
		wantScoreLess bool   // if true, check that closer match has lower score
		compareWith   string // target to compare score with (will be lowercased)
	}{
		{
			name:   "empty query",
			query:  "",
			target: "anything",
			wantOk: true,
		},
		{
			name:   "exact match at start",
			query:  "test",
			target: "test string",
			wantOk: true,
		},
		{
			name:   "exact match in middle",
			query:  "test",
			target: "some test string",
			wantOk: true,
		},
		{
			name:   "fuzzy match with gaps",
			query:  "abc",
			target: "a b c",
			wantOk: true,
		},
		{
			name:   "consecutive characters",
			query:  "ab",
			target: "abc",
			wantOk: true,
		},
		{
			name:   "no match",
			query:  "xyz",
			target: "abc",
			wantOk: false,
		},
		{
			name:   "query longer than target",
			query:  "abcdef",
			target: "abc",
			wantOk: false,
		},
		{
			name:   "unicode characters",
			query:  "café",
			target: "café test",
			wantOk: true,
		},
		{
			name:          "closer match has lower score",
			query:         "test",
			target:        "test string",
			compareWith:   "some test string",
			wantOk:        true,
			wantScoreLess: true,
		},
		{
			name:          "consecutive chars score lower than gaps",
			query:         "ab",
			target:        "abc",
			compareWith:   "a b c",
			wantOk:        true,
			wantScoreLess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queryLower := strings.ToLower(tt.query)
			targetLower := strings.ToLower(tt.target)
			gotScore, gotOk := fuzzyScoreLower(queryLower, targetLower)
			if gotOk != tt.wantOk {
				t.Errorf("fuzzyScoreLower() ok = %v, want %v", gotOk, tt.wantOk)
			}
			if tt.wantScoreLess && tt.compareWith != "" {
				compareLower := strings.ToLower(tt.compareWith)
				compareScore, compareOk := fuzzyScoreLower(queryLower, compareLower)
				if !compareOk {
					t.Errorf("fuzzyScoreLower() comparison target should match")
				}
				if gotScore >= compareScore {
					t.Errorf("fuzzyScoreLower() closer match should have lower score: got %d, compare %d", gotScore, compareScore)
				}
			}
		})
	}
}

func TestMinInt(t *testing.T) {
	tests := []struct {
		name string
		a    int
		b    int
		want int
	}{
		{"a less than b", 1, 2, 1},
		{"b less than a", 5, 3, 3},
		{"equal values", 4, 4, 4},
		{"negative values", -5, -3, -5},
		{"mixed signs", -2, 5, -2},
		{"zero and positive", 0, 10, 0},
		{"zero and negative", 0, -10, -10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := minInt(tt.a, tt.b); got != tt.want {
				t.Errorf("minInt(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestMaxInt(t *testing.T) {
	tests := []struct {
		name string
		a    int
		b    int
		want int
	}{
		{"a greater than b", 5, 2, 5},
		{"b greater than a", 1, 3, 3},
		{"equal values", 4, 4, 4},
		{"negative values", -5, -3, -3},
		{"mixed signs", -2, 5, 5},
		{"zero and positive", 0, 10, 10},
		{"zero and negative", 0, -10, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := maxInt(tt.a, tt.b); got != tt.want {
				t.Errorf("maxInt(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestRunBranchNameScript(t *testing.T) {
	ctx := t.Context()

	tests := []struct {
		name    string
		script  string
		diff    string
		want    string
		wantErr bool
	}{
		{
			name:    "empty script returns empty",
			script:  "",
			diff:    "some diff",
			want:    "",
			wantErr: false,
		},
		{
			name:    "simple echo script",
			script:  "echo feature/test-branch",
			diff:    "some diff",
			want:    "feature/test-branch",
			wantErr: false,
		},
		{
			name:    "script with trailing newline",
			script:  "printf 'feature/branch'",
			diff:    "some diff",
			want:    "feature/branch",
			wantErr: false,
		},
		{
			name:    "script outputs multiple lines, only first is used",
			script:  "printf 'first-branch\nsecond-line'",
			diff:    "some diff",
			want:    "first-branch",
			wantErr: false,
		},
		{
			name:    "script receives diff on stdin",
			script:  "cat | head -c 10",
			diff:    "diff --git a/file.txt",
			want:    "diff --git",
			wantErr: false,
		},
		{
			name:    "script returns empty output",
			script:  "echo ''",
			diff:    "some diff",
			want:    "",
			wantErr: false,
		},
		{
			name:    "script that fails",
			script:  "exit 1",
			diff:    "some diff",
			want:    "",
			wantErr: true,
		},
		{
			name:    "script with whitespace output",
			script:  "echo '  feature/trimmed  '",
			diff:    "some diff",
			want:    "feature/trimmed",
			wantErr: false,
		},
		{
			name:    "command not found",
			script:  "nonexistent_command_xyz123",
			diff:    "some diff",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := runBranchNameScript(ctx, tt.script, tt.diff, "diff", "", "", "")
			if (err != nil) != tt.wantErr {
				t.Errorf("runBranchNameScript() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("runBranchNameScript() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRunBranchNameScriptWithEnvironmentVariables(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		scriptType     string
		number         string
		template       string
		suggestedName  string
		expectedOutput string
	}{
		{
			name:           "PR context with all variables",
			scriptType:     "pr",
			number:         "123",
			template:       "pr-{number}-{title}",
			suggestedName:  "pr-123-fix-bug",
			expectedOutput: "pr|123|pr-{number}-{title}|pr-123-fix-bug",
		},
		{
			name:           "issue context with all variables",
			scriptType:     "issue",
			number:         "456",
			template:       "issue-{number}-{title}",
			suggestedName:  "issue-456-add-feature",
			expectedOutput: "issue|456|issue-{number}-{title}|issue-456-add-feature",
		},
		{
			name:           "diff context with empty variables",
			scriptType:     "diff",
			number:         "",
			template:       "",
			suggestedName:  "",
			expectedOutput: "diff|||",
		},
		{
			name:           "script uses suggested name directly",
			scriptType:     "pr",
			number:         "789",
			template:       "pr-{number}-{title}",
			suggestedName:  "pr-789-update-docs",
			expectedOutput: "pr-789-update-docs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test script that echoes environment variables
			script := `echo "$LAZYWORKTREE_TYPE|$LAZYWORKTREE_NUMBER|$LAZYWORKTREE_TEMPLATE|$LAZYWORKTREE_SUGGESTED_NAME"`
			if tt.name == "script uses suggested name directly" {
				script = "echo $LAZYWORKTREE_SUGGESTED_NAME"
			}

			got, err := runBranchNameScript(ctx, script, "test content", tt.scriptType, tt.number, tt.template, tt.suggestedName)
			if err != nil {
				t.Errorf("runBranchNameScript() error = %v", err)
				return
			}
			if got != tt.expectedOutput {
				t.Errorf("runBranchNameScript() = %q, want %q", got, tt.expectedOutput)
			}
		})
	}
}

func TestFormatCreateFromCurrentLabel(t *testing.T) {
	tests := []struct {
		name     string
		branch   string
		expected string
	}{
		{
			name:     "empty branch returns base label",
			branch:   "",
			expected: "Create from current",
		},
		{
			name:     "short branch name",
			branch:   "main",
			expected: "Create from current (main)",
		},
		{
			name:     "medium branch name",
			branch:   "feature/add-new-feature",
			expected: "Create from current (feature/add-new-feature)",
		},
		{
			name:     "branch name at exactly 58 chars total",
			branch:   "feature/this-is-exactly-thirty-eight-chars",
			expected: "Create from current (feature/this-is-exactly-thirty-eight-chars)",
		},
		{
			name:     "long branch name gets truncated",
			branch:   "feature/this-is-a-very-long-branch-name-that-exceeds-the-seventy-eight-character-limit-and-should-be-truncated",
			expected: "Create from current (feature/this-is-a-very-long-branch-name-that-exceeds-t...",
		},
		{
			name:     "branch name with special characters",
			branch:   "feature/fix-bug-#123",
			expected: "Create from current (feature/fix-bug-#123)",
		},
		{
			name:     "detached HEAD",
			branch:   "HEAD",
			expected: "Create from current (HEAD)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatCreateFromCurrentLabel(tt.branch)
			if result != tt.expected {
				t.Errorf("formatCreateFromCurrentLabel(%q) = %q, want %q", tt.branch, result, tt.expected)
			}
			// Verify result is at most 78 chars
			if len(result) > 78 {
				t.Errorf("formatCreateFromCurrentLabel(%q) result length = %d, want <= 78", tt.branch, len(result))
			}
		})
	}
}

func TestFilterInputSuggestions(t *testing.T) {
	tests := []struct {
		name        string
		suggestions []string
		query       string
		expected    []string
	}{
		{
			name:        "empty query returns all suggestions",
			suggestions: []string{"apple", "banana", "cherry"},
			query:       "",
			expected:    []string{"apple", "banana", "cherry"},
		},
		{
			name:        "query with whitespace is trimmed",
			suggestions: []string{"apple", "banana", "cherry"},
			query:       "  app  ",
			expected:    []string{"apple"},
		},
		{
			name:        "exact match",
			suggestions: []string{"apple", "banana", "cherry"},
			query:       "apple",
			expected:    []string{"apple"},
		},
		{
			name:        "prefix match",
			suggestions: []string{"apple", "application", "banana"},
			query:       "app",
			expected:    []string{"apple", "application"},
		},
		{
			name:        "case insensitive match",
			suggestions: []string{"Apple", "BANANA", "cherry"},
			query:       "app",
			expected:    []string{"Apple"},
		},
		{
			name:        "fuzzy match",
			suggestions: []string{"apple", "banana", "cherry", "apricot"},
			query:       "apl",
			expected:    []string{"apple"}, // fuzzyScoreLower may only match "apple" for "apl"
		},
		{
			name:        "no matches returns empty",
			suggestions: []string{"apple", "banana", "cherry"},
			query:       "xyz",
			expected:    []string{},
		},
		{
			name:        "empty suggestions",
			suggestions: []string{},
			query:       "app",
			expected:    []string{},
		},
		{
			name:        "suggestions sorted by score",
			suggestions: []string{"application", "apple", "apricot"},
			query:       "ap",
			expected:    []string{"apple", "application", "apricot"}, // All should match, verify all are present
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterInputSuggestions(tt.suggestions, tt.query)
			if len(result) != len(tt.expected) {
				t.Errorf("filterInputSuggestions() length = %d, want %d, got %v", len(result), len(tt.expected), result)
				return
			}
			// Verify all expected items are present (order may vary for same scores)
			for _, expected := range tt.expected {
				found := false
				for _, r := range result {
					if r == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("filterInputSuggestions() missing expected item %q, got %v", expected, result)
				}
			}
		})
	}
}

func TestFormatRelativeTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{
			name:     "just now",
			time:     now.Add(-30 * time.Second),
			expected: "just now",
		},
		{
			name:     "1 minute ago",
			time:     now.Add(-1 * time.Minute),
			expected: "1 minute ago",
		},
		{
			name:     "multiple minutes ago",
			time:     now.Add(-5 * time.Minute),
			expected: "5 minutes ago",
		},
		{
			name:     "59 minutes ago",
			time:     now.Add(-59 * time.Minute),
			expected: "59 minutes ago",
		},
		{
			name:     "1 hour ago",
			time:     now.Add(-1 * time.Hour),
			expected: "1 hour ago",
		},
		{
			name:     "multiple hours ago",
			time:     now.Add(-5 * time.Hour),
			expected: "5 hours ago",
		},
		{
			name:     "23 hours ago",
			time:     now.Add(-23 * time.Hour),
			expected: "23 hours ago",
		},
		{
			name:     "yesterday",
			time:     now.Add(-1 * 24 * time.Hour),
			expected: "yesterday",
		},
		{
			name:     "multiple days ago",
			time:     now.Add(-3 * 24 * time.Hour),
			expected: "3 days ago",
		},
		{
			name:     "6 days ago",
			time:     now.Add(-6 * 24 * time.Hour),
			expected: "6 days ago",
		},
		{
			name:     "1 week ago",
			time:     now.Add(-7 * 24 * time.Hour),
			expected: "1 week ago",
		},
		{
			name:     "multiple weeks ago",
			time:     now.Add(-14 * 24 * time.Hour),
			expected: "2 weeks ago",
		},
		{
			name:     "4 weeks ago",
			time:     now.Add(-28 * 24 * time.Hour),
			expected: "4 weeks ago",
		},
		{
			name:     "more than 30 days ago",
			time:     now.Add(-60 * 24 * time.Hour),
			expected: "", // Will be formatted as date
		},
		{
			name:     "far in the past",
			time:     now.Add(-365 * 24 * time.Hour),
			expected: "", // Will be formatted as date
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatRelativeTime(tt.time)
			if tt.expected != "" {
				if result != tt.expected {
					t.Errorf("formatRelativeTime() = %q, want %q", result, tt.expected)
				}
			} else {
				// For dates, verify it's formatted as "Jan 2, 2006"
				if result == "" {
					t.Error("formatRelativeTime() returned empty string for date format")
				}
				// Verify it contains a year (basic check)
				if !strings.Contains(result, "200") && !strings.Contains(result, "202") && !strings.Contains(result, "199") {
					t.Errorf("formatRelativeTime() = %q, expected date format", result)
				}
			}
		})
	}
}
