package services

import (
	"net/url"
	"sort"
	"strings"

	"github.com/chmouel/lazyworktree/internal/models"
)

// CIDataService provides stateless CI check operations.
type CIDataService interface {
	// Sort returns a copy of checks sorted with GitHub Actions first.
	Sort(checks []*models.CICheck) []*models.CICheck

	// StatusIcon returns the appropriate icon for CI status.
	// Draft PRs show "D" instead of CI status.
	StatusIcon(status string, isDraft, useIcons bool, iconProvider CIIconProvider) string

	// ExtractRunID extracts the run ID from a GitHub Actions URL.
	// Example: https://github.com/owner/repo/actions/runs/12345678/job/98765432 -> 12345678
	ExtractRunID(link string) string

	// ExtractJobID extracts the job ID from a GitHub Actions URL.
	// Example: https://github.com/owner/repo/actions/runs/12345678/job/98765432 -> 98765432
	ExtractJobID(link string) string

	// ExtractRepo extracts the owner/repo from a GitHub URL.
	// Example: https://github.com/owner/repo/actions/runs/12345678 -> owner/repo
	ExtractRepo(link string) string
}

// CIIconProvider provides CI status icons.
type CIIconProvider interface {
	GetCIIcon(conclusion string) string
}

type ciDataService struct{}

// NewCIDataService creates a new CIDataService.
func NewCIDataService() CIDataService {
	return &ciDataService{}
}

func (s *ciDataService) Sort(checks []*models.CICheck) []*models.CICheck {
	sorted := make([]*models.CICheck, len(checks))
	copy(sorted, checks)
	sort.SliceStable(sorted, func(i, j int) bool {
		// GitHub Actions links contain "/actions/"
		iIsGHA := strings.Contains(sorted[i].Link, "/actions/")
		jIsGHA := strings.Contains(sorted[j].Link, "/actions/")
		if iIsGHA != jIsGHA {
			return iIsGHA // GitHub Actions first
		}
		return false // Preserve original order within each group
	})
	return sorted
}

func (s *ciDataService) StatusIcon(status string, isDraft, useIcons bool, iconProvider CIIconProvider) string {
	if isDraft {
		return "D"
	}
	if useIcons && iconProvider != nil {
		if icon := iconProvider.GetCIIcon(status); icon != "" {
			return icon
		}
	}
	switch status {
	case "success":
		return "S"
	case "failure":
		return "F"
	case "skipped":
		return "-"
	case "cancelled":
		return "C"
	case "pending":
		return "P"
	default:
		return "?"
	}
}

func (s *ciDataService) ExtractRunID(link string) string {
	if link == "" {
		return ""
	}

	parsed, err := url.Parse(link)
	if err != nil {
		return ""
	}

	// Check if it's a GitHub Actions URL
	if !strings.Contains(parsed.Host, "github.com") {
		return ""
	}

	// Path should contain /actions/runs/<run_id>
	parts := strings.Split(parsed.Path, "/")
	for i, part := range parts {
		if part == "runs" && i+1 < len(parts) {
			return parts[i+1]
		}
	}

	return ""
}

func (s *ciDataService) ExtractJobID(link string) string {
	if link == "" {
		return ""
	}

	parsed, err := url.Parse(link)
	if err != nil {
		return ""
	}

	// Check if it's a GitHub Actions URL
	if !strings.Contains(parsed.Host, "github.com") {
		return ""
	}

	// Path should contain /job/<job_id>
	parts := strings.Split(parsed.Path, "/")
	for i, part := range parts {
		if part == "job" && i+1 < len(parts) {
			return parts[i+1]
		}
	}

	return ""
}

func (s *ciDataService) ExtractRepo(link string) string {
	if link == "" {
		return ""
	}

	parsed, err := url.Parse(link)
	if err != nil {
		return ""
	}

	// Check if it's a GitHub URL
	if !strings.Contains(parsed.Host, "github.com") {
		return ""
	}

	// Path should be /owner/repo/...
	parts := strings.Split(strings.TrimPrefix(parsed.Path, "/"), "/")
	if len(parts) >= 2 {
		return parts[0] + "/" + parts[1]
	}

	return ""
}
