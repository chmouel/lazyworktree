package services

import (
	"testing"

	"github.com/chmouel/lazyworktree/internal/models"
)

// mockIconProvider implements CIIconProvider for testing.
type mockIconProvider struct {
	icons map[string]string
}

func (m *mockIconProvider) GetCIIcon(conclusion string) string {
	if m.icons != nil {
		return m.icons[conclusion]
	}
	return ""
}

func TestCIDataService_Sort(t *testing.T) {
	svc := NewCIDataService()

	tests := []struct {
		name     string
		checks   []*models.CICheck
		expected []string // expected order of names
	}{
		{
			name:     "empty checks",
			checks:   []*models.CICheck{},
			expected: []string{},
		},
		{
			name: "all GitHub Actions",
			checks: []*models.CICheck{
				{Name: "build", Link: "https://github.com/owner/repo/actions/runs/123"},
				{Name: "test", Link: "https://github.com/owner/repo/actions/runs/456"},
			},
			expected: []string{"build", "test"},
		},
		{
			name: "all non-GitHub Actions",
			checks: []*models.CICheck{
				{Name: "tekton", Link: "https://tekton.example.com/run/123"},
				{Name: "jenkins", Link: "https://jenkins.example.com/job/456"},
			},
			expected: []string{"tekton", "jenkins"},
		},
		{
			name: "mixed - GitHub Actions should be first",
			checks: []*models.CICheck{
				{Name: "tekton", Link: "https://tekton.example.com/run/123"},
				{Name: "build", Link: "https://github.com/owner/repo/actions/runs/789"},
				{Name: "jenkins", Link: "https://jenkins.example.com/job/456"},
				{Name: "test", Link: "https://github.com/owner/repo/actions/runs/101"},
			},
			expected: []string{"build", "test", "tekton", "jenkins"},
		},
		{
			name: "stable sort preserves order within groups",
			checks: []*models.CICheck{
				{Name: "external1", Link: "https://external.com/1"},
				{Name: "gha1", Link: "https://github.com/owner/repo/actions/runs/1"},
				{Name: "external2", Link: "https://external.com/2"},
				{Name: "gha2", Link: "https://github.com/owner/repo/actions/runs/2"},
			},
			expected: []string{"gha1", "gha2", "external1", "external2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.Sort(tt.checks)
			if len(result) != len(tt.expected) {
				t.Fatalf("Sort() returned %d checks, want %d", len(result), len(tt.expected))
			}
			for i, name := range tt.expected {
				if result[i].Name != name {
					t.Errorf("Sort()[%d].Name = %q, want %q", i, result[i].Name, name)
				}
			}
		})
	}
}

func TestCIDataService_Sort_DoesNotMutateOriginal(t *testing.T) {
	svc := NewCIDataService()

	original := []*models.CICheck{
		{Name: "external", Link: "https://external.com/1"},
		{Name: "gha", Link: "https://github.com/owner/repo/actions/runs/1"},
	}
	originalOrder := []string{original[0].Name, original[1].Name}

	_ = svc.Sort(original)

	// Verify original slice is unchanged
	for i, name := range originalOrder {
		if original[i].Name != name {
			t.Errorf("Original slice was mutated: [%d].Name = %q, want %q", i, original[i].Name, name)
		}
	}
}

func TestCIDataService_StatusIcon(t *testing.T) {
	svc := NewCIDataService()

	tests := []struct {
		name         string
		status       string
		isDraft      bool
		useIcons     bool
		iconProvider CIIconProvider
		expected     string
	}{
		{
			name:     "draft PR",
			status:   "success",
			isDraft:  true,
			useIcons: true,
			expected: "D",
		},
		{
			name:     "success without icons",
			status:   "success",
			isDraft:  false,
			useIcons: false,
			expected: "S",
		},
		{
			name:     "failure without icons",
			status:   "failure",
			isDraft:  false,
			useIcons: false,
			expected: "F",
		},
		{
			name:     "skipped without icons",
			status:   "skipped",
			isDraft:  false,
			useIcons: false,
			expected: "-",
		},
		{
			name:     "cancelled without icons",
			status:   "cancelled",
			isDraft:  false,
			useIcons: false,
			expected: "C",
		},
		{
			name:     "pending without icons",
			status:   "pending",
			isDraft:  false,
			useIcons: false,
			expected: "P",
		},
		{
			name:     "unknown without icons",
			status:   "unknown",
			isDraft:  false,
			useIcons: false,
			expected: "?",
		},
		{
			name:     "empty status without icons",
			status:   "",
			isDraft:  false,
			useIcons: false,
			expected: "?",
		},
		{
			name:         "success with icons from provider",
			status:       "success",
			isDraft:      false,
			useIcons:     true,
			iconProvider: &mockIconProvider{icons: map[string]string{"success": "✓"}},
			expected:     "✓",
		},
		{
			name:         "icons enabled but provider returns empty",
			status:       "success",
			isDraft:      false,
			useIcons:     true,
			iconProvider: &mockIconProvider{icons: map[string]string{}},
			expected:     "S", // falls back to text
		},
		{
			name:         "icons enabled but nil provider",
			status:       "success",
			isDraft:      false,
			useIcons:     true,
			iconProvider: nil,
			expected:     "S", // falls back to text
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.StatusIcon(tt.status, tt.isDraft, tt.useIcons, tt.iconProvider)
			if result != tt.expected {
				t.Errorf("StatusIcon() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestCIDataService_ExtractRunID(t *testing.T) {
	svc := NewCIDataService()

	tests := []struct {
		name     string
		link     string
		expected string
	}{
		{
			name:     "empty link",
			link:     "",
			expected: "",
		},
		{
			name:     "valid GitHub Actions URL with job",
			link:     "https://github.com/owner/repo/actions/runs/12345678/job/98765432",
			expected: "12345678",
		},
		{
			name:     "valid GitHub Actions URL without job",
			link:     "https://github.com/owner/repo/actions/runs/12345678",
			expected: "12345678",
		},
		{
			name:     "non-GitHub URL",
			link:     "https://tekton.example.com/run/12345",
			expected: "",
		},
		{
			name:     "GitHub URL but not actions",
			link:     "https://github.com/owner/repo/pull/123",
			expected: "",
		},
		{
			name:     "invalid URL",
			link:     "not-a-url",
			expected: "",
		},
		{
			name:     "GitHub Enterprise URL (not supported)",
			link:     "https://github.mycompany.com/owner/repo/actions/runs/999",
			expected: "", // GitHub Enterprise is not supported - must contain "github.com"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.ExtractRunID(tt.link)
			if result != tt.expected {
				t.Errorf("ExtractRunID() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestCIDataService_ExtractJobID(t *testing.T) {
	svc := NewCIDataService()

	tests := []struct {
		name     string
		link     string
		expected string
	}{
		{
			name:     "empty link",
			link:     "",
			expected: "",
		},
		{
			name:     "valid GitHub Actions URL with job",
			link:     "https://github.com/owner/repo/actions/runs/12345678/job/98765432",
			expected: "98765432",
		},
		{
			name:     "GitHub Actions URL without job",
			link:     "https://github.com/owner/repo/actions/runs/12345678",
			expected: "",
		},
		{
			name:     "non-GitHub URL",
			link:     "https://tekton.example.com/job/12345",
			expected: "",
		},
		{
			name:     "invalid URL",
			link:     "not-a-url",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.ExtractJobID(tt.link)
			if result != tt.expected {
				t.Errorf("ExtractJobID() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestCIDataService_ExtractRepo(t *testing.T) {
	svc := NewCIDataService()

	tests := []struct {
		name     string
		link     string
		expected string
	}{
		{
			name:     "empty link",
			link:     "",
			expected: "",
		},
		{
			name:     "valid GitHub URL",
			link:     "https://github.com/owner/repo/actions/runs/12345678",
			expected: "owner/repo",
		},
		{
			name:     "valid GitHub URL with deep path",
			link:     "https://github.com/owner/repo/actions/runs/12345678/job/98765432",
			expected: "owner/repo",
		},
		{
			name:     "GitHub root URL",
			link:     "https://github.com/owner",
			expected: "",
		},
		{
			name:     "non-GitHub URL",
			link:     "https://gitlab.com/owner/repo/pipelines/123",
			expected: "",
		},
		{
			name:     "invalid URL",
			link:     "not-a-url",
			expected: "",
		},
		{
			name:     "GitHub Enterprise URL (not supported)",
			link:     "https://github.mycompany.com/org/project/actions/runs/999",
			expected: "", // GitHub Enterprise is not supported - must contain "github.com"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.ExtractRepo(tt.link)
			if result != tt.expected {
				t.Errorf("ExtractRepo() = %q, want %q", result, tt.expected)
			}
		})
	}
}
