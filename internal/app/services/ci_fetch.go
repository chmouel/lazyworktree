package services

import (
	"context"

	"github.com/chmouel/lazyworktree/internal/models"
)

// GitCIProvider defines the interface for git service CI operations.
// This interface enables dependency injection for testing CI-related functionality.
type GitCIProvider interface {
	// FetchCIStatus fetches CI status for a PR.
	FetchCIStatus(ctx context.Context, prNumber int, branch string) ([]*models.CICheck, error)

	// FetchCIStatusByCommit fetches CI status for a commit SHA.
	FetchCIStatusByCommit(ctx context.Context, sha, path string) ([]*models.CICheck, error)

	// GetHeadSHA returns the HEAD commit SHA for a worktree path.
	GetHeadSHA(ctx context.Context, path string) string

	// IsGitHub returns true if the repository is hosted on GitHub.
	IsGitHub(ctx context.Context) bool
}

// CIFetchService creates commands for fetching CI status.
// This service abstracts the creation of fetch operations.
type CIFetchService interface {
	// CreateFetchForPR creates a fetch function for PR-based CI status.
	// The returned function can be called to perform the fetch and return a result.
	CreateFetchForPR(ctx context.Context, git GitCIProvider, prNumber int, branch string) func() CIFetchResult

	// CreateFetchForCommit creates a fetch function for commit-based CI status.
	// The returned function can be called to perform the fetch and return a result.
	CreateFetchForCommit(ctx context.Context, git GitCIProvider, worktreePath, branch string) func() CIFetchResult
}

// CIFetchResult contains the result of a CI status fetch operation.
type CIFetchResult struct {
	Branch string
	Checks []*models.CICheck
	Err    error
}

type ciFetchService struct{}

// NewCIFetchService creates a new CIFetchService.
func NewCIFetchService() CIFetchService {
	return &ciFetchService{}
}

func (s *ciFetchService) CreateFetchForPR(ctx context.Context, git GitCIProvider, prNumber int, branch string) func() CIFetchResult {
	return func() CIFetchResult {
		checks, err := git.FetchCIStatus(ctx, prNumber, branch)
		return CIFetchResult{
			Branch: branch,
			Checks: checks,
			Err:    err,
		}
	}
}

func (s *ciFetchService) CreateFetchForCommit(ctx context.Context, git GitCIProvider, worktreePath, branch string) func() CIFetchResult {
	return func() CIFetchResult {
		commitSHA := git.GetHeadSHA(ctx, worktreePath)
		if commitSHA == "" {
			return CIFetchResult{Branch: branch, Checks: nil, Err: nil}
		}
		checks, err := git.FetchCIStatusByCommit(ctx, commitSHA, worktreePath)
		return CIFetchResult{Branch: branch, Checks: checks, Err: err}
	}
}
