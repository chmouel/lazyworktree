package git

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/chmouel/lazyworktree/internal/models"
)

// FetchCIStatus fetches CI check statuses for a PR from GitHub or GitLab.
func (s *Service) FetchCIStatus(ctx context.Context, prNumber int, branch string) ([]*models.CICheck, error) {
	host := s.DetectHost(ctx)
	switch host {
	case gitHostGithub:
		return s.fetchGitHubCI(ctx, prNumber)
	case gitHostGitLab:
		return s.fetchGitLabCI(ctx, branch)
	default:
		return nil, nil
	}
}

// FetchCIStatusByCommit fetches CI check statuses for a commit SHA (GitHub only).
// This is used for branches without an associated PR.
func (s *Service) FetchCIStatusByCommit(ctx context.Context, commitSHA, worktreePath string) ([]*models.CICheck, error) {
	if s.DetectHost(ctx) != gitHostGithub {
		return nil, nil
	}

	repoName := s.ResolveRepoName(ctx)
	if repoName == "" || repoName == "unknown" || strings.HasPrefix(repoName, "local-") {
		return nil, nil
	}

	// GitHub Check Runs API: GET /repos/{owner}/{repo}/commits/{ref}/check-runs
	apiPath := fmt.Sprintf("repos/%s/commits/%s/check-runs", repoName, commitSHA)
	out := s.RunGit(ctx, []string{"gh", "api", apiPath, "--jq", ".check_runs"},
		worktreePath, []int{0, 1}, true, true)

	if out == "" {
		return nil, nil
	}

	var checkRuns []struct {
		Name       string `json:"name"`
		Status     string `json:"status"`     // queued, in_progress, completed
		Conclusion string `json:"conclusion"` // success, failure, neutral, cancelled, skipped, timed_out, action_required
		HTMLURL    string `json:"html_url"`
		StartedAt  string `json:"started_at"`
	}

	if err := json.Unmarshal([]byte(out), &checkRuns); err != nil {
		return nil, err
	}

	result := make([]*models.CICheck, 0, len(checkRuns))
	for _, run := range checkRuns {
		conclusion := s.mapGitHubConclusion(run.Status, run.Conclusion)
		var startedAt time.Time
		if run.StartedAt != "" {
			startedAt, _ = time.Parse(time.RFC3339, run.StartedAt)
		}
		result = append(result, &models.CICheck{
			Name:       run.Name,
			Status:     strings.ToLower(run.Status),
			Conclusion: conclusion,
			Link:       run.HTMLURL,
			StartedAt:  startedAt,
		})
	}
	return result, nil
}
