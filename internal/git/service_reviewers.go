package git

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/chmouel/lazyworktree/internal/models"
)

// githubReviewersQuery asks for the latest review submitted by each reviewer.
// totalCount is read alongside the nodes so a pull request with more reviewers
// than we page through still reports an honest count.
const githubReviewersQuery = `query($owner:String!,$name:String!,$number:Int!){
  repository(owner:$owner,name:$name){
    pullRequest(number:$number){
      latestReviews(first:30){
        totalCount
        nodes{
          state
          author{ __typename login avatarUrl(size:64) ... on User{ name } }
        }
      }
    }
  }
}`

// gitlabReviewersQuery reads per-reviewer review state, which the REST API does
// not expose. webUrl comes along so root-relative avatar paths returned by
// self-managed instances can be resolved against the instance origin.
const gitlabReviewersQuery = `query($fullPath:ID!,$iid:String!){
  project(fullPath:$fullPath){
    mergeRequest(iid:$iid){
      webUrl
      reviewers{
        nodes{
          username
          name
          avatarUrl
          bot
          mergeRequestInteraction{ reviewState }
        }
      }
    }
  }
}`

// FetchPRReviewers returns the reviewers who have submitted a review on a
// PR/MR. Hosts other than GitHub and GitLab return no reviewers and no error.
func (s *Service) FetchPRReviewers(ctx context.Context, prNumber int) (*models.PRReviewerSummary, error) {
	if prNumber <= 0 {
		return nil, nil
	}

	switch s.DetectHost(ctx) {
	case gitHostGithub:
		return s.fetchGitHubReviewers(ctx, prNumber)
	case gitHostGitLab:
		return s.fetchGitLabReviewers(ctx, prNumber)
	default:
		return nil, nil
	}
}

func (s *Service) fetchGitHubReviewers(ctx context.Context, prNumber int) (*models.PRReviewerSummary, error) {
	owner, name, ok := s.forgeRepoParts(ctx)
	if !ok {
		return nil, nil
	}

	// owner and name are passed as raw fields: typed fields coerce values to
	// JSON types, which breaks String! for an all-numeric repository name, and
	// they additionally treat a leading @ as "read this file".
	raw, err := s.runForgeJSON(ctx, []string{
		"gh", "api", "graphql",
		"-f", "query=" + githubReviewersQuery,
		"-f", "owner=" + owner,
		"-f", "name=" + name,
		"-F", "number=" + strconv.Itoa(prNumber),
	})
	if err != nil {
		return nil, err
	}
	return parseGitHubReviewers(raw)
}

func parseGitHubReviewers(raw []byte) (*models.PRReviewerSummary, error) {
	var payload struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
		Data struct {
			Repository struct {
				PullRequest struct {
					LatestReviews struct {
						TotalCount int `json:"totalCount"`
						Nodes      []struct {
							State  string `json:"state"`
							Author *struct {
								Typename  string `json:"__typename"`
								Login     string `json:"login"`
								AvatarURL string `json:"avatarUrl"`
								Name      string `json:"name"`
							} `json:"author"`
						} `json:"nodes"`
					} `json:"latestReviews"`
				} `json:"pullRequest"`
			} `json:"repository"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("parse reviewer data: %w", err)
	}
	if len(payload.Errors) > 0 {
		return nil, fmt.Errorf("fetch reviewers: %s", payload.Errors[0].Message)
	}

	reviews := payload.Data.Repository.PullRequest.LatestReviews
	summary := &models.PRReviewerSummary{
		Total:     reviews.TotalCount,
		Reviewers: make([]*models.PRReviewer, 0, len(reviews.Nodes)),
	}
	for _, node := range reviews.Nodes {
		// A review by a since-deleted account has no author, yet it was still
		// submitted, so it keeps its place in the total.
		if node.Author == nil || strings.TrimSpace(node.Author.Login) == "" {
			continue
		}
		state := normaliseGitHubReviewState(node.State)
		if state == "" {
			continue
		}
		summary.Reviewers = append(summary.Reviewers, &models.PRReviewer{
			Login:     node.Author.Login,
			Name:      node.Author.Name,
			AvatarURL: sanitiseAvatarURL(node.Author.AvatarURL, ""),
			IsBot:     node.Author.Typename == "Bot",
			State:     state,
		})
	}
	if summary.Total < len(summary.Reviewers) {
		summary.Total = len(summary.Reviewers)
	}
	return summary, nil
}

func normaliseGitHubReviewState(state string) string {
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case models.ReviewStateApproved:
		return models.ReviewStateApproved
	case models.ReviewStateChangesRequested:
		return models.ReviewStateChangesRequested
	case models.ReviewStateCommented:
		return models.ReviewStateCommented
	case models.ReviewStateDismissed:
		return models.ReviewStateDismissed
	case "PENDING":
		// Not submitted yet, and only ever visible to its own author.
		return ""
	default:
		return models.ReviewStateCommented
	}
}

func (s *Service) fetchGitLabReviewers(ctx context.Context, prNumber int) (*models.PRReviewerSummary, error) {
	fullPath := s.ResolveCITargetRepoName(ctx)
	if !usableForgeRepoName(fullPath) {
		return nil, nil
	}

	// iid is declared String! by GitLab, so it must be a raw field; a typed
	// field would be sent as a JSON number and rejected by the schema.
	raw, err := s.runForgeJSON(ctx, []string{
		"glab", "api", "graphql",
		"-f", "query=" + gitlabReviewersQuery,
		"-f", "fullPath=" + fullPath,
		"-f", "iid=" + strconv.Itoa(prNumber),
	})
	if err != nil {
		return nil, err
	}
	return parseGitLabReviewers(raw)
}

func parseGitLabReviewers(raw []byte) (*models.PRReviewerSummary, error) {
	var payload struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
		Data struct {
			Project struct {
				MergeRequest struct {
					WebURL    string `json:"webUrl"`
					Reviewers struct {
						Nodes []struct {
							Username                string `json:"username"`
							Name                    string `json:"name"`
							AvatarURL               string `json:"avatarUrl"`
							Bot                     bool   `json:"bot"`
							MergeRequestInteraction *struct {
								ReviewState string `json:"reviewState"`
							} `json:"mergeRequestInteraction"`
						} `json:"nodes"`
					} `json:"reviewers"`
				} `json:"mergeRequest"`
			} `json:"project"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("parse reviewer data: %w", err)
	}
	if len(payload.Errors) > 0 {
		return nil, fmt.Errorf("fetch reviewers: %s", payload.Errors[0].Message)
	}

	mr := payload.Data.Project.MergeRequest
	summary := &models.PRReviewerSummary{
		Reviewers: make([]*models.PRReviewer, 0, len(mr.Reviewers.Nodes)),
	}
	for _, node := range mr.Reviewers.Nodes {
		if node.MergeRequestInteraction == nil {
			continue
		}
		state, submitted := gitlabReviewState(node.MergeRequestInteraction.ReviewState)
		if !submitted {
			continue
		}
		if strings.TrimSpace(node.Username) == "" {
			continue
		}
		summary.Reviewers = append(summary.Reviewers, &models.PRReviewer{
			Login:     node.Username,
			Name:      node.Name,
			AvatarURL: sanitiseAvatarURL(node.AvatarURL, mr.WebURL),
			IsBot:     node.Bot,
			State:     state,
		})
	}
	summary.Total = len(summary.Reviewers)
	return summary, nil
}

// gitlabReviewState maps GitLab's review state to ours, reporting whether a
// review was actually submitted. Reviewers who have yet to submit, and states
// we do not recognise, are left out rather than shown under a guessed state.
func gitlabReviewState(state string) (string, bool) {
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case "APPROVED":
		return models.ReviewStateApproved, true
	case "REQUESTED_CHANGES":
		return models.ReviewStateChangesRequested, true
	case "REVIEWED":
		return models.ReviewStateCommented, true
	case "UNAPPROVED":
		return models.ReviewStateDismissed, true
	default:
		return "", false
	}
}

// forgeRepoParts splits the CI target repository into owner and name.
func (s *Service) forgeRepoParts(ctx context.Context) (owner, name string, ok bool) {
	repo := s.ResolveCITargetRepoName(ctx)
	if !usableForgeRepoName(repo) {
		return "", "", false
	}
	owner, name, found := strings.Cut(repo, "/")
	if !found || owner == "" || name == "" {
		return "", "", false
	}
	return owner, name, true
}

func usableForgeRepoName(repo string) bool {
	repo = strings.TrimSpace(repo)
	return repo != "" && repo != gitHostUnknown && !strings.HasPrefix(repo, "local-")
}

// sanitiseAvatarURL keeps only avatar URLs we are willing to fetch: absolute
// HTTPS ones, plus the root-relative paths self-managed GitLab returns, which
// are resolved against the merge request's own origin. Anything else yields an
// empty string, so the reviewer is shown without a picture.
func sanitiseAvatarURL(rawURL, baseURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	// Scheme-relative URLs would silently adopt our scheme and point at an
	// arbitrary host, so they are rejected outright.
	if strings.HasPrefix(rawURL, "//") {
		return ""
	}

	if strings.HasPrefix(rawURL, "/") {
		base, err := url.Parse(strings.TrimSpace(baseURL))
		if err != nil || base.Scheme != "https" || base.Host == "" {
			return ""
		}
		ref, err := url.Parse(rawURL)
		if err != nil {
			return ""
		}
		return base.ResolveReference(ref).String()
	}

	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return ""
	}
	return parsed.String()
}
