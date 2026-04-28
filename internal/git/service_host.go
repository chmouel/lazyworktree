package git

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// DetectHost detects the git host (github, gitlab, or unknown)
func (s *Service) DetectHost(ctx context.Context) string {
	s.gitHostOnce.Do(func() {
		// Allow tests to pre-seed gitHost directly on the struct.
		if s.gitHost != "" {
			return
		}
		s.gitHost = gitHostUnknown
		remoteURL := s.getRemoteURL(ctx)
		if remoteURL != "" {
			re := regexp.MustCompile(`(?:git@|https?://|ssh://|git://)(?:[^@]+@)?([^/:]+)`)
			matches := re.FindStringSubmatch(remoteURL)
			if len(matches) > 1 {
				hostname := strings.ToLower(matches[1])
				if strings.Contains(hostname, gitHostGitLab) {
					s.gitHost = gitHostGitLab
				}
				if strings.Contains(hostname, gitHostGithub) {
					s.gitHost = gitHostGithub
				}
			}
		}
	})
	return s.gitHost
}

// IsGitHubOrGitLab returns true if the repository is connected to GitHub or GitLab.
func (s *Service) IsGitHubOrGitLab(ctx context.Context) bool {
	host := s.DetectHost(ctx)
	return host == gitHostGithub || host == gitHostGitLab
}

// IsGitHub returns true if the repository is connected to GitHub.
func (s *Service) IsGitHub(ctx context.Context) bool {
	return s.DetectHost(ctx) == gitHostGithub
}

// ResolveRepoName resolves the repository name using various methods.
// ResolveRepoName returns the repository identifier for caching purposes.
func (s *Service) ResolveRepoName(ctx context.Context) string {
	var repoName string

	remoteURL := s.getRemoteURL(ctx)

	if remoteURL != "" {
		if strings.Contains(remoteURL, "github.com") {
			re := regexp.MustCompile(`github\.com[:/](.+)(?:\.git)?$`)
			matches := re.FindStringSubmatch(remoteURL)
			if len(matches) > 1 {
				repoName = matches[1]
			}
		} else if strings.Contains(remoteURL, "gitlab.com") {
			re := regexp.MustCompile(`gitlab\.com[:/](.+)(?:\.git)?$`)
			matches := re.FindStringSubmatch(remoteURL)
			if len(matches) > 1 {
				repoName = matches[1]
			}
		}
	}

	if repoName == "" {
		if out := s.RunGit(ctx, []string{"gh", "repo", "view", "--json", "nameWithOwner", "-q", ".nameWithOwner"}, "", []int{0}, true, true); out != "" {
			repoName = out
		}
	}

	if repoName == "" {
		if out := s.RunGit(ctx, []string{"glab", "repo", "view", "-F", "json"}, "", []int{0}, false, true); out != "" {
			var data map[string]any
			if err := json.Unmarshal([]byte(out), &data); err == nil {
				if path, ok := data["path_with_namespace"].(string); ok {
					repoName = path
				}
			}
		}
	}

	if repoName == "" && remoteURL != "" {
		re := regexp.MustCompile(`[:/]([^/]+/[^/]+)(?:\.git)?$`)
		matches := re.FindStringSubmatch(remoteURL)
		if len(matches) > 1 {
			repoName = matches[1]
		}
	}

	if repoName == "" {
		if out := s.RunGit(ctx, []string{"git", "rev-parse", "--show-toplevel"}, "", []int{0}, true, true); out != "" {
			repoName = localRepoKey(out)
		}
	}

	if repoName == "" {
		return "unknown"
	}

	repoName = strings.TrimSuffix(repoName, ".git")
	if decoded, err := url.PathUnescape(repoName); err == nil {
		repoName = decoded
	}
	return repoName
}

// localRepoKey builds a stable, compact cache key when no remote name is available.
func localRepoKey(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(path))
	return fmt.Sprintf("local-%x", sum[:8])
}
