package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPRInfo(t *testing.T) {
	tests := []struct {
		name   string
		prInfo *PRInfo
	}{
		{
			name: "valid PR info",
			prInfo: &PRInfo{
				Number: 123,
				State:  "OPEN",
				Title:  "Test PR",
				URL:    "https://github.com/user/repo/pull/123",
			},
		},
		{
			name: "merged PR",
			prInfo: &PRInfo{
				Number: 456,
				State:  "MERGED",
				Title:  "Merged PR",
				URL:    "https://github.com/user/repo/pull/456",
			},
		},
		{
			name: "closed PR",
			prInfo: &PRInfo{
				Number: 789,
				State:  "CLOSED",
				Title:  "Closed PR",
				URL:    "https://github.com/user/repo/pull/789",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.prInfo)
			assert.Positive(t, tt.prInfo.Number)
			assert.NotEmpty(t, tt.prInfo.State)
			assert.NotEmpty(t, tt.prInfo.Title)
			assert.NotEmpty(t, tt.prInfo.URL)
		})
	}
}

func TestPRInfoEnsureAuthorAvatarURL(t *testing.T) {
	tests := []struct {
		name string
		pr   *PRInfo
		want string
	}{
		{
			name: "backfills github avatar from login",
			pr: &PRInfo{
				Author: "chmouel",
				URL:    "https://github.com/tektoncd/pipelines-as-code/pull/2745",
			},
			want: "https://github.com/chmouel.png?size=64",
		},
		{
			name: "keeps existing avatar url",
			pr: &PRInfo{
				Author:          "chmouel",
				AuthorAvatarURL: "https://example.com/custom.png",
				URL:             "https://github.com/o/r/pull/1",
			},
			want: "https://example.com/custom.png",
		},
		{
			name: "ignores non-github hosts",
			pr: &PRInfo{
				Author: "alice",
				URL:    "https://gitlab.com/o/r/-/merge_requests/1",
			},
			want: "",
		},
		{
			name: "no-op without author login",
			pr: &PRInfo{
				URL: "https://github.com/o/r/pull/1",
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.pr.EnsureAuthorAvatarURL()
			assert.Equal(t, tt.want, tt.pr.AuthorAvatarURL)
		})
	}
}

func TestPRInfoEnsureAuthorAvatarURLNilSafe(t *testing.T) {
	var pr *PRInfo
	assert.NotPanics(t, func() { pr.EnsureAuthorAvatarURL() })
}

func TestWorktreeInfo(t *testing.T) {
	tests := []struct {
		name string
		wt   *WorktreeInfo
	}{
		{
			name: "main worktree clean",
			wt: &WorktreeInfo{
				Path:         "/repo/main",
				Branch:       "main",
				IsMain:       true,
				Dirty:        false,
				Ahead:        0,
				Behind:       0,
				LastActive:   "2024-01-01",
				LastActiveTS: 1704067200,
				PR:           nil,
				Untracked:    0,
				Modified:     0,
				Staged:       0,
				Divergence:   "",
			},
		},
		{
			name: "feature worktree with changes",
			wt: &WorktreeInfo{
				Path:         "/repo/feature",
				Branch:       "feature/test",
				IsMain:       false,
				Dirty:        true,
				Ahead:        3,
				Behind:       1,
				LastActive:   "2024-01-02",
				LastActiveTS: 1704153600,
				PR: &PRInfo{
					Number: 100,
					State:  "OPEN",
					Title:  "Feature PR",
					URL:    "https://github.com/user/repo/pull/100",
				},
				Untracked:  2,
				Modified:   5,
				Staged:     3,
				Divergence: "↑3 ↓1",
			},
		},
		{
			name: "worktree ahead of remote",
			wt: &WorktreeInfo{
				Path:         "/repo/ahead",
				Branch:       "ahead-branch",
				IsMain:       false,
				Dirty:        false,
				Ahead:        5,
				Behind:       0,
				LastActive:   "2024-01-03",
				LastActiveTS: 1704240000,
				Divergence:   "↑5",
			},
		},
		{
			name: "worktree behind remote",
			wt: &WorktreeInfo{
				Path:         "/repo/behind",
				Branch:       "behind-branch",
				IsMain:       false,
				Dirty:        false,
				Ahead:        0,
				Behind:       2,
				LastActive:   "2024-01-04",
				LastActiveTS: 1704326400,
				Divergence:   "↓2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.wt)
			assert.NotEmpty(t, tt.wt.Path)
			assert.NotEmpty(t, tt.wt.Branch)
			assert.GreaterOrEqual(t, tt.wt.Ahead, 0)
			assert.GreaterOrEqual(t, tt.wt.Behind, 0)
			assert.GreaterOrEqual(t, tt.wt.Untracked, 0)
			assert.GreaterOrEqual(t, tt.wt.Modified, 0)
			assert.GreaterOrEqual(t, tt.wt.Staged, 0)

			// Verify dirty flag is consistent with file changes
			if tt.wt.Untracked > 0 || tt.wt.Modified > 0 || tt.wt.Staged > 0 {
				assert.True(t, tt.wt.Dirty, "worktree should be dirty when it has changes")
			}
		})
	}
}

func TestWorktreeNoteIsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		note     WorktreeNote
		expected bool
	}{
		{name: "empty note", note: WorktreeNote{}, expected: true},
		{name: "with text", note: WorktreeNote{Note: "hello"}, expected: false},
		{name: "with icon", note: WorktreeNote{Icon: "🔥"}, expected: false},
		{name: "with color", note: WorktreeNote{Color: "red"}, expected: false},
		{name: "with bold", note: WorktreeNote{Bold: true}, expected: false},
		{name: "with description", note: WorktreeNote{Description: "desc"}, expected: false},
		{name: "with tags", note: WorktreeNote{Tags: []string{"bug"}}, expected: false},
		{name: "with empty tags", note: WorktreeNote{Tags: []string{}}, expected: true},
		{name: "with whitespace tags only", note: WorktreeNote{Tags: []string{" ", "\t"}}, expected: true},
		{name: "whitespace only", note: WorktreeNote{Note: "  ", Icon: " "}, expected: true},
		{name: "updated_at only", note: WorktreeNote{UpdatedAt: 123}, expected: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.note.IsEmpty())
		})
	}
}

func TestNormalizeTags(t *testing.T) {
	tests := []struct {
		name     string
		tags     []string
		expected []string
	}{
		{name: "nil tags", tags: nil, expected: nil},
		{name: "empty tags", tags: []string{}, expected: nil},
		{name: "trim and drop empty", tags: []string{" bug ", "", " frontend ", "   "}, expected: []string{"bug", "frontend"}},
		{name: "preserve order", tags: []string{"urgent", "bug", "frontend"}, expected: []string{"urgent", "bug", "frontend"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, NormalizeTags(tt.tags))
		})
	}
}

func TestConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{
			name:     "last selected filename",
			constant: LastSelectedFilename,
			expected: ".last-selected",
		},
		{
			name:     "cache filename",
			constant: CacheFilename,
			expected: ".worktree-cache.json",
		},
		{
			name:     "worktree notes filename",
			constant: WorktreeNotesFilename,
			expected: ".worktree-notes.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.constant)
			assert.NotEmpty(t, tt.constant)
		})
	}
}
