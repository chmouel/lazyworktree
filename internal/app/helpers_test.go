package app

import (
	"context"
	"strings"
	"testing"

	"github.com/chmouel/lazyworktree/internal/models"
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
			result := generatePRWorktreeName(tt.pr, "pr-{number}-{title}", "")
			// For the long title test, just verify it's <= 100 chars and doesn't end with hyphen
			if tt.name == "long title gets truncated" {
				if len(result) > 100 {
					t.Errorf("generatePRWorktreeName() result length = %d, want <= 100", len(result))
				}
				if strings.HasSuffix(result, "-") {
					t.Errorf("generatePRWorktreeName() result ends with hyphen: %q", result)
				}
			} else if result != tt.expected {
				t.Errorf("generatePRWorktreeName() = %q, want %q", result, tt.expected)
			}
			// Ensure result is max 100 chars
			if len(result) > 100 {
				t.Errorf("generatePRWorktreeName() result length = %d, want <= 100", len(result))
			}
		})
	}

	// Test custom templates
	t.Run("template format without hyphen", func(t *testing.T) {
		pr := &models.PRInfo{
			Number: 123,
			Title:  "Add feature",
		}
		result := generatePRWorktreeName(pr, "pr{number}-{title}", "")
		expected := "pr123-add-feature"
		if result != expected {
			t.Errorf("generatePRWorktreeName() with custom template = %q, want %q", result, expected)
		}
	})

	t.Run("custom template without prefix", func(t *testing.T) {
		pr := &models.PRInfo{
			Number: 456,
			Title:  "Fix bug",
		}
		result := generatePRWorktreeName(pr, "{number}-{title}", "")
		expected := "456-fix-bug"
		if result != expected {
			t.Errorf("generatePRWorktreeName() with custom template = %q, want %q", result, expected)
		}
	})

	t.Run("custom prefix", func(t *testing.T) {
		pr := &models.PRInfo{
			Number: 789,
			Title:  "Update docs",
		}
		result := generatePRWorktreeName(pr, "pull{number}-{title}", "")
		expected := "pull789-update-docs"
		if result != expected {
			t.Errorf("generatePRWorktreeName() with custom prefix = %q, want %q", result, expected)
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
			result := generateIssueWorktreeName(tt.issue, tt.template, "")
			if tt.name == "long title gets truncated" {
				if len(result) > 100 {
					t.Errorf("generateIssueWorktreeName() result length = %d, want <= 100", len(result))
				}
				if strings.HasSuffix(result, "-") {
					t.Errorf("generateIssueWorktreeName() result ends with hyphen: %q", result)
				}
			} else if result != tt.expected {
				t.Errorf("generateIssueWorktreeName() = %q, want %q", result, tt.expected)
			}
			// Ensure result is max 100 chars
			if len(result) > 100 {
				t.Errorf("generateIssueWorktreeName() result length = %d, want <= 100", len(result))
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
			result := generatePRWorktreeName(tt.pr, tt.template, tt.generatedTitle)
			if tt.name == "long {generated} title gets truncated" {
				if len(result) > 100 {
					t.Errorf("generatePRWorktreeName() result length = %d, want <= 100", len(result))
				}
				if strings.HasSuffix(result, "-") {
					t.Errorf("generatePRWorktreeName() result ends with hyphen: %q", result)
				}
			} else if result != tt.expected {
				t.Errorf("generatePRWorktreeName() = %q, want %q", result, tt.expected)
			}
			// Ensure result is max 100 chars
			if len(result) > 100 {
				t.Errorf("generatePRWorktreeName() result length = %d, want <= 100", len(result))
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
			result := generateIssueWorktreeName(tt.issue, tt.template, tt.generatedTitle)
			if result != tt.expected {
				t.Errorf("generateIssueWorktreeName() = %q, want %q", result, tt.expected)
			}
			// Ensure result is max 100 chars
			if len(result) > 100 {
				t.Errorf("generateIssueWorktreeName() result length = %d, want <= 100", len(result))
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

func TestPaletteMatchScore(t *testing.T) {
	tests := []struct {
		name        string
		query       string // will be lowercased in test
		item        paletteItem
		wantOk      bool
		labelBetter bool        // if true, label match should have better (lower) score than description
		compareItem paletteItem // item to compare score with
		exactBetter bool        // if true, exact match should have better score than fuzzy
	}{
		{
			name:   "empty query",
			query:  "",
			item:   paletteItem{label: "test", description: "description"},
			wantOk: true,
		},
		{
			name:   "exact label match",
			query:  "test",
			item:   paletteItem{label: "test", description: "something"},
			wantOk: true,
		},
		{
			name:   "fuzzy label match",
			query:  "ts",
			item:   paletteItem{label: "test", description: "something"},
			wantOk: true,
		},
		{
			name:   "exact description match",
			query:  "desc",
			item:   paletteItem{label: "label", description: "description"},
			wantOk: true,
		},
		{
			name:   "fuzzy description match",
			query:  "dsc",
			item:   paletteItem{label: "label", description: "description"},
			wantOk: true,
		},
		{
			name:        "both label and description match - label wins",
			query:       "test",
			item:        paletteItem{label: "test", description: "test description"},
			compareItem: paletteItem{label: "xyz", description: "test description"},
			wantOk:      true,
			labelBetter: true,
		},
		{
			name:   "no match",
			query:  "xyz",
			item:   paletteItem{label: "test", description: "description"},
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queryLower := strings.ToLower(tt.query)
			gotScore, gotOk := paletteMatchScore(queryLower, tt.item)
			if gotOk != tt.wantOk {
				t.Errorf("paletteMatchScore() ok = %v, want %v", gotOk, tt.wantOk)
			}
			if tt.labelBetter && tt.compareItem.label != "" {
				compareScore, compareOk := paletteMatchScore(queryLower, tt.compareItem)
				if !compareOk {
					t.Errorf("paletteMatchScore() comparison item should match")
				}
				if gotScore >= compareScore {
					t.Errorf("paletteMatchScore() label match should have better (lower) score: got %d, compare %d", gotScore, compareScore)
				}
			}
			if tt.exactBetter && tt.compareItem.label != "" {
				compareScore, compareOk := paletteMatchScore(queryLower, tt.compareItem)
				if !compareOk {
					t.Errorf("paletteMatchScore() comparison item should match")
				}
				if gotScore >= compareScore {
					t.Errorf("paletteMatchScore() exact match should have better (lower) score: got %d, compare %d", gotScore, compareScore)
				}
			}
		})
	}
}

func TestFilterPaletteItems(t *testing.T) {
	items := []paletteItem{
		{id: "1", label: "test item", description: "first test"},
		{id: "2", label: "example", description: "second example"},
		{id: "3", label: "sample", description: "third sample"},
		{id: "4", label: "demo", description: "test description"},
		{id: "5", label: "xyz", description: "no match"},
	}

	tests := []struct {
		name    string
		query   string
		wantIDs []string
		wantLen int
	}{
		{
			name:    "empty query returns all",
			query:   "",
			wantIDs: []string{"1", "2", "3", "4", "5"},
			wantLen: 5,
		},
		{
			name:    "whitespace only query returns all",
			query:   "   ",
			wantIDs: []string{"1", "2", "3", "4", "5"},
			wantLen: 5,
		},
		{
			name:    "exact label match",
			query:   "test",
			wantIDs: []string{"1", "4"}, // "test item" and "test description"
			wantLen: 2,
		},
		{
			name:    "fuzzy match",
			query:   "ex",
			wantIDs: []string{"2"}, // "example" matches
			wantLen: 1,
		},
		{
			name:    "no matches",
			query:   "zzzzzz",
			wantIDs: []string{},
			wantLen: 0,
		},
		{
			name:    "case insensitive",
			query:   "TEST",
			wantIDs: []string{"1", "4"},
			wantLen: 2,
		},
		{
			name:    "description match",
			query:   "description",
			wantIDs: []string{"4"},
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterPaletteItems(items, tt.query)
			if len(got) != tt.wantLen {
				t.Errorf("filterPaletteItems() len = %v, want %v", len(got), tt.wantLen)
			}
			gotIDs := make([]string, len(got))
			for i, item := range got {
				gotIDs[i] = item.id
			}
			// Check that all expected IDs are present (order may vary due to scoring)
			if tt.wantLen > 0 {
				for _, wantID := range tt.wantIDs {
					found := false
					for _, gotID := range gotIDs {
						if gotID == wantID {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("filterPaletteItems() missing expected ID %q, got IDs: %v", wantID, gotIDs)
					}
				}
			}
			// Verify results are sorted by score (lower is better)
			if len(got) > 1 {
				for i := 0; i < len(got)-1; i++ {
					score1, _ := paletteMatchScore(strings.ToLower(strings.TrimSpace(tt.query)), got[i])
					score2, _ := paletteMatchScore(strings.ToLower(strings.TrimSpace(tt.query)), got[i+1])
					if score1 > score2 {
						t.Errorf("filterPaletteItems() items not sorted correctly: item %d has score %d, item %d has score %d", i, score1, i+1, score2)
					}
				}
			}
		})
	}
}

func TestFilterPaletteItemsSkipsSections(t *testing.T) {
	items := []paletteItem{
		{label: "Section", isSection: true},
		{id: "create", label: "Create", description: "Create item"},
		{id: "delete", label: "Delete", description: "Delete item"},
	}
	result := filterPaletteItems(items, "cre")
	if len(result) != 1 {
		t.Errorf("expected 1 result, got %d", len(result))
	}
	if result[0].id != "create" {
		t.Errorf("expected 'create', got %q", result[0].id)
	}
}

func TestFilterPaletteItemsIncludesSectionsWhenEmpty(t *testing.T) {
	items := []paletteItem{
		{label: "Section", isSection: true},
		{id: "create", label: "Create", description: "Create item"},
	}
	result := filterPaletteItems(items, "")
	if len(result) != 2 {
		t.Errorf("expected 2 results, got %d", len(result))
	}
	if !result[0].isSection {
		t.Error("expected first item to be a section")
	}
}

func TestFilterPaletteItemsWithOnlySections(t *testing.T) {
	items := []paletteItem{
		{label: "Section 1", isSection: true},
		{label: "Section 2", isSection: true},
	}
	result := filterPaletteItems(items, "sec")
	if len(result) != 0 {
		t.Errorf("expected 0 results, got %d", len(result))
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
