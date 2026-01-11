package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			name:     "command history filename",
			constant: CommandHistoryFilename,
			expected: ".command-history.json",
		},
		{
			name:     "access history filename",
			constant: AccessHistoryFilename,
			expected: ".worktree-access.json",
		},
		{
			name:     "AI session status filename",
			constant: AISessionStatusFilename,
			expected: ".ai-status.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.constant)
			assert.NotEmpty(t, tt.constant)
		})
	}
}

func TestAISessionStatus(t *testing.T) {
	tests := []struct {
		name     string
		status   *AISessionStatus
		expected string
	}{
		{
			name: "idle status",
			status: &AISessionStatus{
				Status:    "idle",
				UpdatedAt: time.Date(2024, 1, 11, 12, 0, 0, 0, time.UTC),
				Message:   "",
			},
			expected: "idle",
		},
		{
			name: "working status",
			status: &AISessionStatus{
				Status:    "working",
				UpdatedAt: time.Date(2024, 1, 11, 12, 30, 0, 0, time.UTC),
				Message:   "Processing user request",
			},
			expected: "working",
		},
		{
			name: "error status",
			status: &AISessionStatus{
				Status:    "error",
				UpdatedAt: time.Date(2024, 1, 11, 13, 0, 0, 0, time.UTC),
				Message:   "Connection failed",
			},
			expected: "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.status)
			assert.Equal(t, tt.expected, tt.status.Status)
			assert.False(t, tt.status.UpdatedAt.IsZero())
		})
	}
}

func TestAISessionStatusJSONParsing(t *testing.T) {
	tests := []struct {
		name           string
		jsonInput      string
		expectedStatus string
		expectedMsg    string
		expectError    bool
	}{
		{
			name:           "valid idle status",
			jsonInput:      `{"status":"idle","updated_at":"2024-01-11T12:00:00Z","message":""}`,
			expectedStatus: "idle",
			expectedMsg:    "",
			expectError:    false,
		},
		{
			name:           "valid working status with message",
			jsonInput:      `{"status":"working","updated_at":"2024-01-11T12:30:00Z","message":"Processing request"}`,
			expectedStatus: "working",
			expectedMsg:    "Processing request",
			expectError:    false,
		},
		{
			name:           "minimal valid JSON",
			jsonInput:      `{"status":"idle","updated_at":"2024-01-11T12:00:00Z"}`,
			expectedStatus: "idle",
			expectedMsg:    "",
			expectError:    false,
		},
		{
			name:        "invalid JSON",
			jsonInput:   `{invalid json}`,
			expectError: true,
		},
		{
			name:        "empty JSON",
			jsonInput:   `{}`,
			expectError: false, // Empty is valid, just has zero values
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var status AISessionStatus
			err := json.Unmarshal([]byte(tt.jsonInput), &status)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedStatus, status.Status)
				assert.Equal(t, tt.expectedMsg, status.Message)
			}
		})
	}
}

func TestWorktreeInfoWithAISession(t *testing.T) {
	tests := []struct {
		name     string
		wt       *WorktreeInfo
		hasAI    bool
		aiStatus string
	}{
		{
			name: "worktree without AI session",
			wt: &WorktreeInfo{
				Path:      "/repo/main",
				Branch:    "main",
				AISession: nil,
			},
			hasAI:    false,
			aiStatus: "",
		},
		{
			name: "worktree with idle AI session",
			wt: &WorktreeInfo{
				Path:   "/repo/feature",
				Branch: "feature/ai-work",
				AISession: &AISessionStatus{
					Status:    "idle",
					UpdatedAt: time.Now(),
				},
			},
			hasAI:    true,
			aiStatus: "idle",
		},
		{
			name: "worktree with working AI session",
			wt: &WorktreeInfo{
				Path:   "/repo/other",
				Branch: "other-branch",
				AISession: &AISessionStatus{
					Status:    "working",
					UpdatedAt: time.Now(),
					Message:   "Generating code",
				},
			},
			hasAI:    true,
			aiStatus: "working",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.hasAI {
				assert.NotNil(t, tt.wt.AISession)
				assert.Equal(t, tt.aiStatus, tt.wt.AISession.Status)
			} else {
				assert.Nil(t, tt.wt.AISession)
			}
		})
	}
}
