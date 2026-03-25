package git

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/chmouel/lazyworktree/internal/models"
)

// authorKeys holds the JSON key names for extracting author info from GitHub vs GitLab payloads.
type authorKeys struct {
	usernameKey string // "login" for GitHub, "username" for GitLab
	isBotKey    string // "is_bot" for GitHub, "bot" for GitLab
}

var (
	githubAuthorKeys = authorKeys{usernameKey: "login", isBotKey: "is_bot"}
	gitlabAuthorKeys = authorKeys{usernameKey: "username", isBotKey: "bot"}
)

// extractAuthor extracts author, authorName, and isBot from a JSON map using host-specific keys.
func extractAuthor(data map[string]any, keys authorKeys) (author, authorName string, isBot bool) {
	authorObj, ok := data["author"].(map[string]any)
	if !ok {
		return "", "", false
	}
	author, _ = authorObj[keys.usernameKey].(string)
	authorName, _ = authorObj["name"].(string)
	isBot, _ = authorObj[keys.isBotKey].(bool)
	return
}

// GetAuthenticatedUsername returns the authenticated forge username for this repository host.
// Returns an empty string when no authenticated username can be resolved.
func (s *Service) GetAuthenticatedUsername(ctx context.Context) string {
	host := s.DetectHost(ctx)
	switch host {
	case gitHostGithub:
		return s.getGitHubAuthenticatedUsername(ctx)
	case gitHostGitLab:
		return s.getGitLabAuthenticatedUsername(ctx)
	default:
		return ""
	}
}

func (s *Service) getGitHubAuthenticatedUsername(ctx context.Context) string {
	username := s.RunGit(ctx, []string{"gh", "api", "user", "--jq", ".login"}, "", []int{0}, true, true)
	return strings.TrimSpace(username)
}

// FetchPRMap gathers PR/MR information via supported host APIs (GitHub or GitLab).
// Returns a map keyed by branch name to PRInfo. Detects the host automatically
// based on the repository's remote URL.
func (s *Service) FetchPRMap(ctx context.Context) (map[string]*models.PRInfo, error) {
	host := s.DetectHost(ctx)

	// Skip PR fetching for repos without GitHub/GitLab remotes
	if host == gitHostUnknown {
		return make(map[string]*models.PRInfo), nil
	}

	if host == gitHostGitLab {
		return s.fetchGitLabPRs(ctx)
	}

	return s.fetchGitHubPRs(ctx)
}

func (s *Service) fetchGitHubPRs(ctx context.Context) (map[string]*models.PRInfo, error) {
	prRaw := s.RunGit(ctx, []string{
		"gh", "pr", "list",
		"--state", "all",
		"--json", "headRefName,state,number,title,body,url,author",
		"--limit", "100",
	}, "", []int{0}, false, false)

	if prRaw == "" {
		return make(map[string]*models.PRInfo), nil
	}

	var prs []map[string]any
	if err := json.Unmarshal([]byte(prRaw), &prs); err != nil {
		key := "pr_json_decode"
		s.notifyOnce(key, fmt.Sprintf("Failed to parse PR data: %v", err), "error")
		return nil, err
	}

	prMap := make(map[string]*models.PRInfo)
	for _, p := range prs {
		headRefName, _ := p["headRefName"].(string)
		state, _ := p["state"].(string)
		number, _ := p["number"].(float64)
		title, _ := p["title"].(string)
		body, _ := p["body"].(string)
		url, _ := p["url"].(string)
		author, authorName, authorIsBot := extractAuthor(p, githubAuthorKeys)

		if headRefName != "" {
			prMap[headRefName] = &models.PRInfo{
				Number:      int(number),
				State:       state,
				Title:       title,
				Body:        body,
				URL:         url,
				Branch:      headRefName,
				Author:      author,
				AuthorName:  authorName,
				AuthorIsBot: authorIsBot,
			}
		}
	}

	return prMap, nil
}

// FetchPRForWorktreeWithError fetches PR info and returns detailed error information.
func (s *Service) FetchPRForWorktreeWithError(ctx context.Context, worktreePath string) (*models.PRInfo, error) {
	host := s.DetectHost(ctx)

	switch host {
	case gitHostGithub:
		return s.fetchGitHubPRForWorktreeWithError(ctx, worktreePath)
	case gitHostGitLab:
		return s.fetchGitLabPRForWorktreeWithError(ctx, worktreePath)
	default:
		return nil, nil
	}
}

func (s *Service) fetchGitHubPRForWorktreeWithError(ctx context.Context, worktreePath string) (*models.PRInfo, error) {
	// Run gh pr view with silent=false to capture actual errors
	prRaw := s.RunGit(ctx, []string{
		"gh", "pr", "view",
		"--json", "number,state,title,body,url,headRefName,baseRefName,author",
	}, worktreePath, []int{0, 1}, false, false)

	if prRaw == "" {
		// Check if it's because gh CLI is missing
		if _, err := exec.LookPath("gh"); err != nil {
			return nil, fmt.Errorf("gh CLI not found in PATH")
		}
		// Exit code 1 typically means "no PR found", which is not an error
		return nil, nil
	}

	var pr map[string]any
	if err := json.Unmarshal([]byte(prRaw), &pr); err != nil {
		return nil, fmt.Errorf("failed to parse PR data: %w", err)
	}

	number, _ := pr["number"].(float64)
	state, _ := pr["state"].(string)
	title, _ := pr["title"].(string)
	body, _ := pr["body"].(string)
	url, _ := pr["url"].(string)
	headRefName, _ := pr["headRefName"].(string)
	baseRefName, _ := pr["baseRefName"].(string)
	author, authorName, authorIsBot := extractAuthor(pr, githubAuthorKeys)

	return &models.PRInfo{
		Number:      int(number),
		State:       state,
		Title:       title,
		Body:        body,
		URL:         url,
		Branch:      headRefName,
		BaseBranch:  baseRefName,
		Author:      author,
		AuthorName:  authorName,
		AuthorIsBot: authorIsBot,
	}, nil
}

// FetchPRForWorktree fetches PR info for a specific worktree by running gh/glab in that directory.
// This correctly detects PRs even when the local branch name differs from the remote branch.
// Maintains backward compatibility by swallowing errors.
func (s *Service) FetchPRForWorktree(ctx context.Context, worktreePath string) *models.PRInfo {
	pr, _ := s.FetchPRForWorktreeWithError(ctx, worktreePath)
	return pr
}

// FetchAllOpenPRs fetches all open PRs/MRs and returns them as a slice.
func (s *Service) FetchAllOpenPRs(ctx context.Context) ([]*models.PRInfo, error) {
	host := s.DetectHost(ctx)
	if host == gitHostGitLab {
		return s.fetchGitLabOpenPRs(ctx)
	}

	return s.fetchGitHubOpenPRs(ctx)
}

func (s *Service) fetchGitHubOpenPRs(ctx context.Context) ([]*models.PRInfo, error) {
	prRaw := s.RunGit(ctx, []string{
		"gh", "pr", "list",
		"--state", "open",
		"--json", "headRefName,state,number,title,body,url,author,isDraft,statusCheckRollup",
		"--limit", "100",
	}, "", []int{0}, false, false)

	if prRaw == "" {
		return []*models.PRInfo{}, nil
	}

	var prs []map[string]any
	if err := json.Unmarshal([]byte(prRaw), &prs); err != nil {
		key := "pr_json_decode"
		s.notifyOnce(key, fmt.Sprintf("Failed to parse PR data: %v", err), "error")
		return nil, err
	}

	result := make([]*models.PRInfo, 0, len(prs))
	for _, p := range prs {
		state, _ := p["state"].(string)
		if !strings.EqualFold(state, prStateOpen) {
			continue
		}
		number, _ := p["number"].(float64)
		title, _ := p["title"].(string)
		body, _ := p["body"].(string)
		url, _ := p["url"].(string)
		headRefName, _ := p["headRefName"].(string)
		author, authorName, authorIsBot := extractAuthor(p, githubAuthorKeys)
		isDraft, _ := p["isDraft"].(bool)
		ciStatus := computeCIStatusFromRollup(p["statusCheckRollup"])

		result = append(result, &models.PRInfo{
			Number:      int(number),
			State:       prStateOpen,
			Title:       title,
			Body:        body,
			URL:         url,
			Branch:      headRefName,
			Author:      author,
			AuthorName:  authorName,
			AuthorIsBot: authorIsBot,
			IsDraft:     isDraft,
			CIStatus:    ciStatus,
		})
	}

	return result, nil
}

// FetchPR fetches a single PR by number.
func (s *Service) FetchPR(ctx context.Context, prNumber int) (*models.PRInfo, error) {
	host := s.DetectHost(ctx)
	if host == gitHostGitLab {
		return s.fetchGitLabPR(ctx, prNumber)
	}

	return s.fetchGitHubPR(ctx, prNumber)
}

func (s *Service) fetchGitHubPR(ctx context.Context, prNumber int) (*models.PRInfo, error) {
	prRaw := s.RunGit(ctx, []string{
		"gh", "pr", "view", strconv.Itoa(prNumber),
		"--json", "headRefName,baseRefName,state,number,title,body,url,author,isDraft,statusCheckRollup",
	}, "", []int{0}, false, false)

	if prRaw == "" {
		return nil, fmt.Errorf("PR #%d not found", prNumber)
	}

	var pr map[string]any
	if err := json.Unmarshal([]byte(prRaw), &pr); err != nil {
		key := "pr_json_decode"
		s.notifyOnce(key, fmt.Sprintf("Failed to parse PR data: %v", err), "error")
		return nil, err
	}

	state, _ := pr["state"].(string)
	if !strings.EqualFold(state, prStateOpen) {
		return nil, fmt.Errorf("PR #%d is not open (state: %s)", prNumber, state)
	}

	number, _ := pr["number"].(float64)
	title, _ := pr["title"].(string)
	body, _ := pr["body"].(string)
	url, _ := pr["url"].(string)
	headRefName, _ := pr["headRefName"].(string)
	baseRefName, _ := pr["baseRefName"].(string)
	author, authorName, authorIsBot := extractAuthor(pr, githubAuthorKeys)
	isDraft, _ := pr["isDraft"].(bool)
	ciStatus := computeCIStatusFromRollup(pr["statusCheckRollup"])

	return &models.PRInfo{
		Number:      int(number),
		State:       prStateOpen,
		Title:       title,
		Body:        body,
		URL:         url,
		Branch:      headRefName,
		BaseBranch:  baseRefName,
		Author:      author,
		AuthorName:  authorName,
		AuthorIsBot: authorIsBot,
		IsDraft:     isDraft,
		CIStatus:    ciStatus,
	}, nil
}

// FetchAllOpenIssues fetches all open issues and returns them as a slice.
func (s *Service) FetchAllOpenIssues(ctx context.Context) ([]*models.IssueInfo, error) {
	host := s.DetectHost(ctx)
	if host == gitHostGitLab {
		return s.fetchGitLabOpenIssues(ctx)
	}

	return s.fetchGitHubOpenIssues(ctx)
}

func (s *Service) fetchGitHubOpenIssues(ctx context.Context) ([]*models.IssueInfo, error) {
	issueRaw := s.RunGit(ctx, []string{
		"gh", "issue", "list",
		"--state", "open",
		"--json", "number,state,title,body,url,author",
		"--limit", "100",
	}, "", []int{0}, false, false)

	if issueRaw == "" {
		return []*models.IssueInfo{}, nil
	}

	var issues []map[string]any
	if err := json.Unmarshal([]byte(issueRaw), &issues); err != nil {
		key := "issue_json_decode"
		s.notifyOnce(key, fmt.Sprintf("Failed to parse issue data: %v", err), "error")
		return nil, err
	}

	result := make([]*models.IssueInfo, 0, len(issues))
	for _, i := range issues {
		state, _ := i["state"].(string)
		if !strings.EqualFold(state, "open") {
			continue
		}
		number, _ := i["number"].(float64)
		title, _ := i["title"].(string)
		body, _ := i["body"].(string)
		url, _ := i["url"].(string)
		author, authorName, authorIsBot := extractAuthor(i, githubAuthorKeys)

		result = append(result, &models.IssueInfo{
			Number:      int(number),
			State:       "open",
			Title:       title,
			Body:        body,
			URL:         url,
			Author:      author,
			AuthorName:  authorName,
			AuthorIsBot: authorIsBot,
		})
	}

	return result, nil
}

// FetchIssue fetches a single issue by number.
func (s *Service) FetchIssue(ctx context.Context, issueNumber int) (*models.IssueInfo, error) {
	host := s.DetectHost(ctx)
	if host == gitHostGitLab {
		return s.fetchGitLabIssue(ctx, issueNumber)
	}

	return s.fetchGitHubIssue(ctx, issueNumber)
}

func (s *Service) fetchGitHubIssue(ctx context.Context, issueNumber int) (*models.IssueInfo, error) {
	issueRaw := s.RunGit(ctx, []string{
		"gh", "issue", "view", strconv.Itoa(issueNumber),
		"--json", "number,state,title,body,url,author",
	}, "", []int{0}, false, false)

	if issueRaw == "" {
		return nil, fmt.Errorf("issue #%d not found", issueNumber)
	}

	var issue map[string]any
	if err := json.Unmarshal([]byte(issueRaw), &issue); err != nil {
		key := "issue_json_decode"
		s.notifyOnce(key, fmt.Sprintf("Failed to parse issue data: %v", err), "error")
		return nil, err
	}

	state, _ := issue["state"].(string)
	if !strings.EqualFold(state, "open") {
		return nil, fmt.Errorf("issue #%d is not open (state: %s)", issueNumber, state)
	}

	number, _ := issue["number"].(float64)
	title, _ := issue["title"].(string)
	body, _ := issue["body"].(string)
	url, _ := issue["url"].(string)
	author, authorName, authorIsBot := extractAuthor(issue, githubAuthorKeys)

	return &models.IssueInfo{
		Number:      int(number),
		State:       "open",
		Title:       title,
		Body:        body,
		URL:         url,
		Author:      author,
		AuthorName:  authorName,
		AuthorIsBot: authorIsBot,
	}, nil
}

// mapGitHubConclusion maps GitHub check run status and conclusion to our internal format.
func (s *Service) mapGitHubConclusion(status, conclusion string) string {
	// If still in progress, return pending
	if status == "queued" || status == "in_progress" {
		return ciPending
	}
	// Map conclusion values
	switch strings.ToLower(conclusion) {
	case "success":
		return ciSuccess
	case "failure":
		return ciFailure
	case "neutral", "skipped":
		return ciSkipped
	case "cancelled", "timed_out", "action_required":
		return ciCancelled
	default:
		return conclusion
	}
}

func (s *Service) fetchGitHubCI(ctx context.Context, prNumber int) ([]*models.CICheck, error) {
	// Use gh pr checks to get CI status
	out := s.RunGit(ctx, []string{
		"gh", "pr", "checks", fmt.Sprintf("%d", prNumber),
		"--json", "name,state,bucket,link,startedAt",
	}, "", []int{0, 1, 8}, true, true) // exit code 8 = checks pending

	if out == "" {
		return nil, nil
	}

	var checks []struct {
		Name      string `json:"name"`
		State     string `json:"state"`
		Bucket    string `json:"bucket"`    // pass, fail, pending, skipping, cancel
		Link      string `json:"link"`      // URL to the check details
		StartedAt string `json:"startedAt"` // ISO 8601 format from GH CLI
	}

	if err := json.Unmarshal([]byte(out), &checks); err != nil {
		return nil, err
	}

	result := make([]*models.CICheck, 0, len(checks))
	for _, c := range checks {
		// Map bucket to our conclusion format
		conclusion := s.githubBucketToConclusion(c.Bucket)
		var startedAt time.Time
		if c.StartedAt != "" {
			startedAt, _ = time.Parse(time.RFC3339, c.StartedAt)
		}
		result = append(result, &models.CICheck{
			Name:       c.Name,
			Status:     strings.ToLower(c.State),
			Conclusion: conclusion,
			Link:       c.Link,
			StartedAt:  startedAt,
		})
	}
	return result, nil
}

func (s *Service) githubBucketToConclusion(bucket string) string {
	switch strings.ToLower(bucket) {
	case "pass":
		return ciSuccess
	case "fail":
		return ciFailure
	case "skipping":
		return ciSkipped
	case "cancel":
		return ciCancelled
	case "pending":
		return ciPending
	default:
		return bucket
	}
}

// computeCIStatusFromRollup computes overall CI status from GitHub statusCheckRollup data.
// Returns "success", "failure", "pending", or "none".
func computeCIStatusFromRollup(rollup any) string {
	checks, ok := rollup.([]any)
	if !ok || len(checks) == 0 {
		return "none"
	}

	hasFailure := false
	hasPending := false

	for _, check := range checks {
		checkMap, ok := check.(map[string]any)
		if !ok {
			continue
		}

		conclusion, _ := checkMap["conclusion"].(string)
		status, _ := checkMap["status"].(string)

		// Check for failure states
		switch strings.ToUpper(conclusion) {
		case "FAILURE", "CANCELLED", "TIMED_OUT", "ACTION_REQUIRED":
			hasFailure = true
		}

		// Check for pending states
		if status != "" && !strings.EqualFold(status, "COMPLETED") {
			hasPending = true
		}
	}

	if hasFailure {
		return "failure"
	}
	if hasPending {
		return "pending"
	}
	return "success"
}
