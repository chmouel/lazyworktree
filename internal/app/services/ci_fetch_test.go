package services

import (
	"context"
	"errors"
	"testing"

	"github.com/chmouel/lazyworktree/internal/models"
)

// mockGitCIProvider implements GitCIProvider for testing.
type mockGitCIProvider struct {
	checks         []*models.CICheck
	commitChecks   []*models.CICheck
	headSHA        string
	isGitHub       bool
	fetchPRErr     error
	fetchCommitErr error
}

func (m *mockGitCIProvider) FetchCIStatus(ctx context.Context, prNumber int, branch string) ([]*models.CICheck, error) {
	return m.checks, m.fetchPRErr
}

func (m *mockGitCIProvider) FetchCIStatusByCommit(ctx context.Context, sha, path string) ([]*models.CICheck, error) {
	return m.commitChecks, m.fetchCommitErr
}

func (m *mockGitCIProvider) GetHeadSHA(ctx context.Context, path string) string {
	return m.headSHA
}

func (m *mockGitCIProvider) IsGitHub(ctx context.Context) bool {
	return m.isGitHub
}

func TestCIFetchService_CreateFetchForPR(t *testing.T) {
	svc := NewCIFetchService()
	ctx := context.Background()

	tests := []struct {
		name       string
		provider   *mockGitCIProvider
		prNumber   int
		branch     string
		wantBranch string
		wantChecks int
		wantErr    bool
	}{
		{
			name: "successful fetch with checks",
			provider: &mockGitCIProvider{
				checks: []*models.CICheck{
					{Name: "build", Conclusion: "success"},
					{Name: "test", Conclusion: "failure"},
				},
			},
			prNumber:   123,
			branch:     "feature",
			wantBranch: "feature",
			wantChecks: 2,
			wantErr:    false,
		},
		{
			name: "successful fetch with no checks",
			provider: &mockGitCIProvider{
				checks: []*models.CICheck{},
			},
			prNumber:   456,
			branch:     "main",
			wantBranch: "main",
			wantChecks: 0,
			wantErr:    false,
		},
		{
			name: "fetch error",
			provider: &mockGitCIProvider{
				fetchPRErr: errors.New("network error"),
			},
			prNumber:   789,
			branch:     "dev",
			wantBranch: "dev",
			wantChecks: 0,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fetchFn := svc.CreateFetchForPR(ctx, tt.provider, tt.prNumber, tt.branch)
			result := fetchFn()

			if result.Branch != tt.wantBranch {
				t.Errorf("Branch = %q, want %q", result.Branch, tt.wantBranch)
			}
			if len(result.Checks) != tt.wantChecks {
				t.Errorf("Checks count = %d, want %d", len(result.Checks), tt.wantChecks)
			}
			if (result.Err != nil) != tt.wantErr {
				t.Errorf("Err = %v, wantErr %v", result.Err, tt.wantErr)
			}
		})
	}
}

func TestCIFetchService_CreateFetchForCommit(t *testing.T) {
	svc := NewCIFetchService()
	ctx := context.Background()

	tests := []struct {
		name         string
		provider     *mockGitCIProvider
		worktreePath string
		branch       string
		wantBranch   string
		wantChecks   int
		wantErr      bool
	}{
		{
			name: "successful fetch with checks",
			provider: &mockGitCIProvider{
				headSHA: "abc123",
				commitChecks: []*models.CICheck{
					{Name: "build", Conclusion: "success"},
				},
			},
			worktreePath: "/path/to/worktree",
			branch:       "feature",
			wantBranch:   "feature",
			wantChecks:   1,
			wantErr:      false,
		},
		{
			name: "empty HEAD SHA returns empty result",
			provider: &mockGitCIProvider{
				headSHA:      "",
				commitChecks: []*models.CICheck{{Name: "should-not-be-returned"}},
			},
			worktreePath: "/path/to/worktree",
			branch:       "detached",
			wantBranch:   "detached",
			wantChecks:   0,
			wantErr:      false,
		},
		{
			name: "fetch error",
			provider: &mockGitCIProvider{
				headSHA:        "def456",
				fetchCommitErr: errors.New("API error"),
			},
			worktreePath: "/path/to/worktree",
			branch:       "error-branch",
			wantBranch:   "error-branch",
			wantChecks:   0,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fetchFn := svc.CreateFetchForCommit(ctx, tt.provider, tt.worktreePath, tt.branch)
			result := fetchFn()

			if result.Branch != tt.wantBranch {
				t.Errorf("Branch = %q, want %q", result.Branch, tt.wantBranch)
			}
			if len(result.Checks) != tt.wantChecks {
				t.Errorf("Checks count = %d, want %d", len(result.Checks), tt.wantChecks)
			}
			if (result.Err != nil) != tt.wantErr {
				t.Errorf("Err = %v, wantErr %v", result.Err, tt.wantErr)
			}
		})
	}
}

func TestCIFetchService_FetchFunctionIsReusable(t *testing.T) {
	svc := NewCIFetchService()
	ctx := context.Background()

	callCount := 0
	provider := &mockGitCIProvider{
		checks: []*models.CICheck{{Name: "check1"}},
	}

	// Create fetch function once
	fetchFn := svc.CreateFetchForPR(ctx, provider, 123, "main")

	// Call it multiple times
	for i := 0; i < 3; i++ {
		result := fetchFn()
		callCount++
		if result.Branch != "main" {
			t.Errorf("Call %d: Branch = %q, want %q", i, result.Branch, "main")
		}
	}

	if callCount != 3 {
		t.Errorf("Expected 3 calls, got %d", callCount)
	}
}
