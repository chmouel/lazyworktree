package util

import (
	"fmt"
	"net/url"
	"strings"
)

// SanitizePRURL validates and normalises a pull request URL.
func SanitizePRURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("PR URL is empty")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid PR URL %q: %w", raw, err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("unsupported URL scheme %q", u.Scheme)
	}

	return u.String(), nil
}

// GitURLToWebURL converts a git remote URL to a web URL.
// Handles both SSH (git@github.com:user/repo.git) and HTTPS (https://github.com/user/repo.git) formats.
func GitURLToWebURL(gitURL string) string {
	gitURL = strings.TrimSpace(gitURL)

	// Remove .git suffix if present
	gitURL = strings.TrimSuffix(gitURL, ".git")

	// Handle SSH format: git@github.com:user/repo
	if strings.HasPrefix(gitURL, "git@") {
		// Extract host and path
		parts := strings.SplitN(gitURL, "@", 2)
		if len(parts) == 2 {
			hostPath := parts[1]
			// Replace : with /
			hostPath = strings.Replace(hostPath, ":", "/", 1)
			return "https://" + hostPath
		}
	}

	// Handle HTTPS format: https://github.com/user/repo
	if strings.HasPrefix(gitURL, "https://") || strings.HasPrefix(gitURL, "http://") {
		return gitURL
	}

	// Handle ssh:// format: ssh://git@github.com/user/repo
	if after, ok := strings.CutPrefix(gitURL, "ssh://"); ok {
		gitURL = after
		// Remove git@ if present
		gitURL = strings.TrimPrefix(gitURL, "git@")
		return "https://" + gitURL
	}

	// Handle git:// format: git://github.com/user/repo
	if strings.HasPrefix(gitURL, "git://") {
		return strings.Replace(gitURL, "git://", "https://", 1)
	}

	return ""
}
