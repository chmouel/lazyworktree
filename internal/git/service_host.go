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
		remoteURL := s.getOriginRemoteURL(ctx)
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

func repoNameFromRemoteURL(remoteURL string) string {
	var repoName string

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

	return strings.TrimSuffix(repoName, ".git")
}

// resolveRepoNameFromRemoteURL resolves the repository name from a specific remote URL,
// falling back to gh/glab/local discovery when the URL cannot be parsed.
func (s *Service) resolveRepoNameFromRemoteURL(ctx context.Context, remoteURL string) string {
	repoName := repoNameFromRemoteURL(remoteURL)

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

// ResolveRepoName returns the repository identifier for caching and local state.
// It uses the origin remote when available so CI/PR remote selection does not
// change the repository identity used by worktrees, notes, and caches.
func (s *Service) ResolveRepoName(ctx context.Context) string {
	return s.resolveRepoNameFromRemoteURL(ctx, s.getOriginRemoteURL(ctx))
}

// ResolveCITargetRepoName returns the repository identifier targeted by CI/PR queries.
func (s *Service) ResolveCITargetRepoName(ctx context.Context) string {
	return s.resolveRepoNameFromRemoteURL(ctx, s.getRemoteURL(ctx))
}

// ghRepoArgs returns "--repo <owner/repo>" flags targeting the resolved CI/PR
// remote, so gh queries the intended repository (e.g. upstream) rather than
// whatever gh would default to. It preserves gh's own default resolution when
// automatic mode resolves to origin, but pins origin explicitly when the user
// requested it.
func (s *Service) ghRepoArgs(ctx context.Context) []string {
	repo := s.ResolveCITargetRepoName(ctx)
	if repo == "" || repo == "unknown" || strings.HasPrefix(repo, "local-") {
		return nil
	}
	if s.ciRemote == "" && s.resolveRemoteName(ctx) == "origin" {
		return nil
	}
	return []string{"--repo", repo}
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
