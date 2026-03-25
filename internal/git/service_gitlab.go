package git

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/chmouel/lazyworktree/internal/models"
)

func (s *Service) getGitLabAuthenticatedUsername(ctx context.Context) string {
	raw := s.RunGit(ctx, []string{"glab", "api", "user"}, "", []int{0}, true, true)
	if raw == "" {
		return ""
	}

	var user map[string]any
	if err := json.Unmarshal([]byte(raw), &user); err != nil {
		return ""
	}
	if username, ok := user["username"].(string); ok {
		return strings.TrimSpace(username)
	}
	if username, ok := user["login"].(string); ok {
		return strings.TrimSpace(username)
	}

	return ""
}

// normalizeGitLabState normalises GitLab state strings to the canonical uppercase form.
// "opened" -> "OPEN", others are uppercased as-is.
func normalizeGitLabState(state string) string {
	state = strings.ToUpper(state)
	if state == "OPENED" {
		return prStateOpen
	}
	return state
}

func (s *Service) fetchGitLabPRs(ctx context.Context) (map[string]*models.PRInfo, error) {
	prRaw := s.RunGit(ctx, []string{"glab", "api", "merge_requests?state=all&per_page=100"}, "", []int{0}, false, false)
	if prRaw == "" {
		return make(map[string]*models.PRInfo), nil
	}

	var prs []map[string]any
	if err := json.Unmarshal([]byte(prRaw), &prs); err != nil {
		key := "pr_json_decode_glab"
		s.notifyOnce(key, fmt.Sprintf("Failed to parse GLAB PR data: %v", err), "error")
		return nil, err
	}

	prMap := make(map[string]*models.PRInfo)
	for _, p := range prs {
		state, _ := p["state"].(string)
		state = normalizeGitLabState(state)

		iid, _ := p["iid"].(float64)
		title, _ := p["title"].(string)
		description, _ := p["description"].(string)
		webURL, _ := p["web_url"].(string)
		sourceBranch, _ := p["source_branch"].(string)
		author, authorName, authorIsBot := extractAuthor(p, gitlabAuthorKeys)
		isDraft, _ := p["draft"].(bool)

		if sourceBranch != "" {
			prMap[sourceBranch] = &models.PRInfo{
				Number:      int(iid),
				State:       state,
				Title:       title,
				Body:        description,
				URL:         webURL,
				Branch:      sourceBranch,
				Author:      author,
				AuthorName:  authorName,
				AuthorIsBot: authorIsBot,
				IsDraft:     isDraft,
			}
		}
	}

	return prMap, nil
}

func (s *Service) fetchGitLabPRForWorktreeWithError(ctx context.Context, worktreePath string) (*models.PRInfo, error) {
	// Run glab mr view with silent=false to capture actual errors
	prRaw := s.RunGit(ctx, []string{
		"glab", "mr", "view",
		"--output", "json",
	}, worktreePath, []int{0, 1}, false, false)

	if prRaw == "" {
		// Check if it's because glab CLI is missing
		if _, err := exec.LookPath("glab"); err != nil {
			return nil, fmt.Errorf("glab CLI not found in PATH")
		}
		// Exit code 1 typically means "no MR found", which is not an error
		return nil, nil
	}

	var pr map[string]any
	if err := json.Unmarshal([]byte(prRaw), &pr); err != nil {
		return nil, fmt.Errorf("failed to parse MR data: %w", err)
	}

	iid, _ := pr["iid"].(float64)
	state, _ := pr["state"].(string)
	state = normalizeGitLabState(state)
	title, _ := pr["title"].(string)
	description, _ := pr["description"].(string)
	webURL, _ := pr["web_url"].(string)
	sourceBranch, _ := pr["source_branch"].(string)
	targetBranch, _ := pr["target_branch"].(string)
	author, authorName, authorIsBot := extractAuthor(pr, gitlabAuthorKeys)
	isDraft, _ := pr["draft"].(bool)

	return &models.PRInfo{
		Number:      int(iid),
		State:       state,
		Title:       title,
		Body:        description,
		URL:         webURL,
		Branch:      sourceBranch,
		BaseBranch:  targetBranch,
		Author:      author,
		AuthorName:  authorName,
		AuthorIsBot: authorIsBot,
		IsDraft:     isDraft,
	}, nil
}

func (s *Service) fetchGitLabOpenPRs(ctx context.Context) ([]*models.PRInfo, error) {
	prRaw := s.RunGit(ctx, []string{"glab", "api", "merge_requests?state=opened&per_page=100"}, "", []int{0}, false, false)
	if prRaw == "" {
		return []*models.PRInfo{}, nil
	}

	var prs []map[string]any
	if err := json.Unmarshal([]byte(prRaw), &prs); err != nil {
		key := "pr_json_decode_glab"
		s.notifyOnce(key, fmt.Sprintf("Failed to parse GLAB PR data: %v", err), "error")
		return nil, err
	}

	result := make([]*models.PRInfo, 0, len(prs))
	for _, p := range prs {
		state, _ := p["state"].(string)
		state = normalizeGitLabState(state)
		if state != prStateOpen {
			continue
		}

		iid, _ := p["iid"].(float64)
		title, _ := p["title"].(string)
		description, _ := p["description"].(string)
		webURL, _ := p["web_url"].(string)
		sourceBranch, _ := p["source_branch"].(string)
		author, authorName, authorIsBot := extractAuthor(p, gitlabAuthorKeys)

		// GitLab uses "draft" field for WIP/draft MRs
		isDraft, _ := p["draft"].(bool)
		// CI status would require additional API calls for GitLab, default to none
		ciStatus := "none"

		result = append(result, &models.PRInfo{
			Number:      int(iid),
			State:       state,
			Title:       title,
			Body:        description,
			URL:         webURL,
			Branch:      sourceBranch,
			Author:      author,
			AuthorName:  authorName,
			AuthorIsBot: authorIsBot,
			IsDraft:     isDraft,
			CIStatus:    ciStatus,
		})
	}

	return result, nil
}

// fetchGitLabPR fetches a single PR (merge request) by number from GitLab.
func (s *Service) fetchGitLabPR(ctx context.Context, prNumber int) (*models.PRInfo, error) {
	prRaw := s.RunGit(ctx, []string{"glab", "api", fmt.Sprintf("merge_requests/%d", prNumber)}, "", []int{0}, false, false)
	if prRaw == "" {
		return nil, fmt.Errorf("PR #%d not found", prNumber)
	}

	var pr map[string]any
	if err := json.Unmarshal([]byte(prRaw), &pr); err != nil {
		key := "pr_json_decode_glab"
		s.notifyOnce(key, fmt.Sprintf("Failed to parse GLAB PR data: %v", err), "error")
		return nil, err
	}

	state, _ := pr["state"].(string)
	state = normalizeGitLabState(state)
	if state != prStateOpen {
		return nil, fmt.Errorf("PR #%d is not open (state: %s)", prNumber, state)
	}

	iid, _ := pr["iid"].(float64)
	title, _ := pr["title"].(string)
	description, _ := pr["description"].(string)
	webURL, _ := pr["web_url"].(string)
	sourceBranch, _ := pr["source_branch"].(string)
	targetBranch, _ := pr["target_branch"].(string)
	author, authorName, authorIsBot := extractAuthor(pr, gitlabAuthorKeys)
	isDraft, _ := pr["draft"].(bool)

	return &models.PRInfo{
		Number:      int(iid),
		State:       state,
		Title:       title,
		Body:        description,
		URL:         webURL,
		Branch:      sourceBranch,
		BaseBranch:  targetBranch,
		Author:      author,
		AuthorName:  authorName,
		AuthorIsBot: authorIsBot,
		IsDraft:     isDraft,
		CIStatus:    "none",
	}, nil
}

func (s *Service) fetchGitLabOpenIssues(ctx context.Context) ([]*models.IssueInfo, error) {
	issueRaw := s.RunGit(ctx, []string{"glab", "api", "issues?state=opened&per_page=100"}, "", []int{0}, false, false)
	if issueRaw == "" {
		return []*models.IssueInfo{}, nil
	}

	var issues []map[string]any
	if err := json.Unmarshal([]byte(issueRaw), &issues); err != nil {
		key := "issue_json_decode_glab"
		s.notifyOnce(key, fmt.Sprintf("Failed to parse GLAB issue data: %v", err), "error")
		return nil, err
	}

	result := make([]*models.IssueInfo, 0, len(issues))
	for _, i := range issues {
		state, _ := i["state"].(string)
		state = normalizeGitLabState(state)
		if state != prStateOpen {
			continue
		}

		iid, _ := i["iid"].(float64)
		title, _ := i["title"].(string)
		description, _ := i["description"].(string)
		webURL, _ := i["web_url"].(string)
		author, authorName, authorIsBot := extractAuthor(i, gitlabAuthorKeys)

		result = append(result, &models.IssueInfo{
			Number:      int(iid),
			State:       "open",
			Title:       title,
			Body:        description,
			URL:         webURL,
			Author:      author,
			AuthorName:  authorName,
			AuthorIsBot: authorIsBot,
		})
	}

	return result, nil
}

// fetchGitLabIssue fetches a single issue by number from GitLab.
func (s *Service) fetchGitLabIssue(ctx context.Context, issueNumber int) (*models.IssueInfo, error) {
	issueRaw := s.RunGit(ctx, []string{"glab", "api", fmt.Sprintf("issues/%d", issueNumber)}, "", []int{0}, false, false)
	if issueRaw == "" {
		return nil, fmt.Errorf("issue #%d not found", issueNumber)
	}

	var issue map[string]any
	if err := json.Unmarshal([]byte(issueRaw), &issue); err != nil {
		key := "issue_json_decode_glab"
		s.notifyOnce(key, fmt.Sprintf("Failed to parse GLAB issue data: %v", err), "error")
		return nil, err
	}

	state, _ := issue["state"].(string)
	state = normalizeGitLabState(state)
	if state != prStateOpen {
		return nil, fmt.Errorf("issue #%d is not open (state: %s)", issueNumber, state)
	}

	iid, _ := issue["iid"].(float64)
	title, _ := issue["title"].(string)
	description, _ := issue["description"].(string)
	webURL, _ := issue["web_url"].(string)
	author, authorName, authorIsBot := extractAuthor(issue, gitlabAuthorKeys)

	return &models.IssueInfo{
		Number:      int(iid),
		State:       "open",
		Title:       title,
		Body:        description,
		URL:         webURL,
		Author:      author,
		AuthorName:  authorName,
		AuthorIsBot: authorIsBot,
	}, nil
}

func (s *Service) fetchGitLabCI(ctx context.Context, branch string) ([]*models.CICheck, error) {
	// Use glab ci status to get pipeline jobs
	out := s.RunGit(ctx, []string{
		"glab", "ci", "status", "--branch", branch, "--output", "json",
	}, "", []int{0, 1}, true, true)

	if out == "" {
		return nil, nil
	}

	var pipeline struct {
		Jobs []struct {
			Name      string `json:"name"`
			Status    string `json:"status"`
			WebURL    string `json:"web_url"`
			StartedAt string `json:"started_at"` // ISO 8601 format from GitLab
		} `json:"jobs"`
	}

	if err := json.Unmarshal([]byte(out), &pipeline); err != nil {
		// Try parsing as array of jobs directly
		var jobs []struct {
			Name      string `json:"name"`
			Status    string `json:"status"`
			WebURL    string `json:"web_url"`
			StartedAt string `json:"started_at"` // ISO 8601 format from GitLab
		}
		if err2 := json.Unmarshal([]byte(out), &jobs); err2 != nil {
			return nil, err
		}
		result := make([]*models.CICheck, 0, len(jobs))
		for _, j := range jobs {
			var startedAt time.Time
			if j.StartedAt != "" {
				startedAt, _ = time.Parse(time.RFC3339, j.StartedAt)
			}
			result = append(result, &models.CICheck{
				Name:       j.Name,
				Status:     strings.ToLower(j.Status),
				Conclusion: s.gitlabStatusToConclusion(j.Status),
				Link:       j.WebURL,
				StartedAt:  startedAt,
			})
		}
		return result, nil
	}

	result := make([]*models.CICheck, 0, len(pipeline.Jobs))
	for _, j := range pipeline.Jobs {
		var startedAt time.Time
		if j.StartedAt != "" {
			startedAt, _ = time.Parse(time.RFC3339, j.StartedAt)
		}
		result = append(result, &models.CICheck{
			Name:       j.Name,
			Status:     strings.ToLower(j.Status),
			Conclusion: s.gitlabStatusToConclusion(j.Status),
			Link:       j.WebURL,
			StartedAt:  startedAt,
		})
	}
	return result, nil
}

func (s *Service) gitlabStatusToConclusion(status string) string {
	switch strings.ToLower(status) {
	case "success", "passed":
		return ciSuccess
	case "failed":
		return ciFailure
	case "canceled", "cancelled":
		return ciCancelled
	case "skipped":
		return ciSkipped
	case "running", "pending", "created", "waiting_for_resource", "preparing":
		return ciPending
	default:
		return status
	}
}
