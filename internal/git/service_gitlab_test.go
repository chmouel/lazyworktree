package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeStub(t *testing.T, name, script string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(script), 0o600); err != nil {
		t.Fatalf("failed to write stub: %v", err)
	}
	// #nosec G302 -- test stub needs executable permissions.
	if err := os.Chmod(path, 0o700); err != nil {
		t.Fatalf("failed to chmod stub: %v", err)
	}
	return dir
}

func withStubbedPath(t *testing.T, dir string) {
	t.Helper()

	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+oldPath)
}

func TestFetchGitLabPRs(t *testing.T) {
	stub := "#!/bin/sh\n" +
		"if [ \"$1\" = \"api\" ]; then\n" +
		"  echo '[{\"iid\":1,\"state\":\"opened\",\"title\":\"One\",\"web_url\":\"https://example.com/1\",\"source_branch\":\"feature\",\"draft\":true},{\"iid\":2,\"state\":\"closed\",\"title\":\"Two\",\"web_url\":\"https://example.com/2\",\"source_branch\":\"closed\",\"draft\":false}]'\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 0\n"
	dir := writeStub(t, "glab", stub)
	withStubbedPath(t, dir)

	service := NewService(func(string, string) {}, func(string, string, string) {})
	prs, err := service.fetchGitLabPRs(context.Background())
	require.NoError(t, err)
	require.NotNil(t, prs)

	pr, ok := prs["feature"]
	require.True(t, ok)
	assert.Equal(t, 1, pr.Number)
	assert.Equal(t, prStateOpen, pr.State)
	assert.Equal(t, "One", pr.Title)
	assert.True(t, pr.IsDraft, "PR map should set IsDraft from GitLab draft field")
}

func TestFetchGitLabOpenPRs(t *testing.T) {
	stub := "#!/bin/sh\n" +
		"if [ \"$1\" = \"api\" ]; then\n" +
		"  echo '[{\"iid\":1,\"state\":\"opened\",\"title\":\"One\",\"web_url\":\"https://example.com/1\",\"source_branch\":\"feature\"},{\"iid\":2,\"state\":\"closed\",\"title\":\"Two\",\"web_url\":\"https://example.com/2\",\"source_branch\":\"closed\"}]'\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 0\n"
	dir := writeStub(t, "glab", stub)
	withStubbedPath(t, dir)

	service := NewService(func(string, string) {}, func(string, string, string) {})
	prs, err := service.fetchGitLabOpenPRs(context.Background())
	require.NoError(t, err)
	require.Len(t, prs, 1)
	assert.Equal(t, "feature", prs[0].Branch)
	assert.Equal(t, prStateOpen, prs[0].State)
}

func TestFetchGitLabCI(t *testing.T) {
	stub := "#!/bin/sh\n" +
		"if [ \"$1\" = \"ci\" ]; then\n" +
		"  echo '{\"jobs\":[{\"name\":\"build\",\"status\":\"success\"},{\"name\":\"test\",\"status\":\"failed\"}]}'\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 0\n"
	dir := writeStub(t, "glab", stub)
	withStubbedPath(t, dir)

	service := NewService(func(string, string) {}, func(string, string, string) {})
	checks, err := service.fetchGitLabCI(context.Background(), "main")
	require.NoError(t, err)
	require.Len(t, checks, 2)
	assert.Equal(t, ciSuccess, checks[0].Conclusion)
	assert.Equal(t, ciFailure, checks[1].Conclusion)
}

func TestFetchGitLabCIFallbackArray(t *testing.T) {
	stub := "#!/bin/sh\n" +
		"if [ \"$1\" = \"ci\" ]; then\n" +
		"  echo '[{\"name\":\"lint\",\"status\":\"skipped\"}]'\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 0\n"
	dir := writeStub(t, "glab", stub)
	withStubbedPath(t, dir)

	service := NewService(func(string, string) {}, func(string, string, string) {})
	checks, err := service.fetchGitLabCI(context.Background(), "main")
	require.NoError(t, err)
	require.Len(t, checks, 1)
	assert.Equal(t, ciSkipped, checks[0].Conclusion)
}

func TestFetchGitLabOpenIssues(t *testing.T) {
	stub := "#!/bin/sh\n" +
		"if [ \"$1\" = \"api\" ] && [ \"$2\" = \"issues?state=opened&per_page=100\" ]; then\n" +
		"  echo '[{\"iid\":1,\"state\":\"opened\",\"title\":\"Issue One\",\"description\":\"Description one\",\"web_url\":\"https://example.com/issues/1\",\"author\":{\"username\":\"user1\",\"name\":\"User One\",\"bot\":false}},{\"iid\":2,\"state\":\"closed\",\"title\":\"Issue Two\",\"description\":\"Description two\",\"web_url\":\"https://example.com/issues/2\",\"author\":{\"username\":\"user2\",\"name\":\"User Two\",\"bot\":true}}]'\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 0\n"
	dir := writeStub(t, "glab", stub)
	withStubbedPath(t, dir)

	service := NewService(func(string, string) {}, func(string, string, string) {})
	issues, err := service.fetchGitLabOpenIssues(context.Background())
	require.NoError(t, err)
	require.Len(t, issues, 1)

	issue := issues[0]
	assert.Equal(t, 1, issue.Number)
	assert.Equal(t, "open", issue.State)
	assert.Equal(t, "Issue One", issue.Title)
	assert.Equal(t, "Description one", issue.Body)
	assert.Equal(t, "https://example.com/issues/1", issue.URL)
	assert.Equal(t, "user1", issue.Author)
	assert.Equal(t, "User One", issue.AuthorName)
	assert.False(t, issue.AuthorIsBot)
}

func TestFetchGitLabOpenIssuesEmptyResponse(t *testing.T) {
	stub := "#!/bin/sh\n" +
		"if [ \"$1\" = \"api\" ] && [ \"$2\" = \"issues?state=opened&per_page=100\" ]; then\n" +
		"  echo '[]'\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 0\n"
	dir := writeStub(t, "glab", stub)
	withStubbedPath(t, dir)

	service := NewService(func(string, string) {}, func(string, string, string) {})
	issues, err := service.fetchGitLabOpenIssues(context.Background())
	require.NoError(t, err)
	assert.Empty(t, issues)
}

func TestFetchGitLabOpenIssuesInvalidJSON(t *testing.T) {
	stub := "#!/bin/sh\n" +
		"if [ \"$1\" = \"api\" ] && [ \"$2\" = \"issues?state=opened&per_page=100\" ]; then\n" +
		"  echo 'invalid json'\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 0\n"
	dir := writeStub(t, "glab", stub)
	withStubbedPath(t, dir)

	notified := false
	notifyOnce := func(key, msg, severity string) {
		if key == "issue_json_decode_glab" && severity == "error" {
			notified = true
		}
	}

	service := NewService(func(string, string) {}, notifyOnce)
	issues, err := service.fetchGitLabOpenIssues(context.Background())
	require.Error(t, err)
	assert.Nil(t, issues)
	assert.True(t, notified, "expected notification for JSON decode error")
}

func TestFetchAllOpenIssuesGitLab(t *testing.T) {
	stub := "#!/bin/sh\n" +
		"if [ \"$1\" = \"api\" ] && [ \"$2\" = \"issues?state=opened&per_page=100\" ]; then\n" +
		"  echo '[{\"iid\":1,\"state\":\"opened\",\"title\":\"Issue One\",\"description\":\"Description\",\"web_url\":\"https://gitlab.com/repo/issues/1\",\"author\":{\"username\":\"user1\",\"name\":\"User One\",\"bot\":false}}]'\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 0\n"
	dir := writeStub(t, "glab", stub)
	withStubbedPath(t, dir)

	service := NewService(func(string, string) {}, func(string, string, string) {})
	service.gitHost = gitHostGitLab

	issues, err := service.FetchAllOpenIssues(context.Background())
	require.NoError(t, err)
	require.Len(t, issues, 1)

	issue := issues[0]
	assert.Equal(t, 1, issue.Number)
	assert.Equal(t, "open", issue.State)
	assert.Equal(t, "Issue One", issue.Title)
}

func TestFetchIssueGitLab(t *testing.T) {
	stub := "#!/bin/sh\n" +
		"if [ \"$1\" = \"api\" ] && [ \"$2\" = \"issues/42\" ]; then\n" +
		"  echo '{\"iid\":42,\"state\":\"opened\",\"title\":\"Test Issue\",\"description\":\"Test description\",\"web_url\":\"https://gitlab.com/repo/issues/42\",\"author\":{\"username\":\"testuser\",\"name\":\"Test User\",\"bot\":false}}'\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 1\n"
	dir := writeStub(t, "glab", stub)
	withStubbedPath(t, dir)

	service := NewService(func(string, string) {}, func(string, string, string) {})
	service.gitHost = gitHostGitLab

	issue, err := service.FetchIssue(context.Background(), 42)
	require.NoError(t, err)
	require.NotNil(t, issue)

	assert.Equal(t, 42, issue.Number)
	assert.Equal(t, "open", issue.State)
	assert.Equal(t, "Test Issue", issue.Title)
	assert.Equal(t, "Test description", issue.Body)
}

func TestFetchPRGitLab(t *testing.T) {
	stub := "#!/bin/sh\n" +
		"if [ \"$1\" = \"api\" ] && [ \"$2\" = \"merge_requests/123\" ]; then\n" +
		"  echo '{\"iid\":123,\"state\":\"opened\",\"title\":\"Test MR\",\"description\":\"Test description\",\"web_url\":\"https://gitlab.com/repo/merge_requests/123\",\"source_branch\":\"feature\",\"target_branch\":\"main\",\"author\":{\"username\":\"testuser\",\"name\":\"Test User\",\"bot\":false},\"draft\":false}'\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 1\n"
	dir := writeStub(t, "glab", stub)
	withStubbedPath(t, dir)

	service := NewService(func(string, string) {}, func(string, string, string) {})
	service.gitHost = gitHostGitLab

	pr, err := service.FetchPR(context.Background(), 123)
	require.NoError(t, err)
	require.NotNil(t, pr)

	assert.Equal(t, 123, pr.Number)
	assert.Equal(t, "OPEN", pr.State)
	assert.Equal(t, "Test MR", pr.Title)
	assert.Equal(t, "feature", pr.Branch)
	assert.Equal(t, "main", pr.BaseBranch)
}

func TestGetAuthenticatedUsernameGitLab(t *testing.T) {
	stub := "#!/bin/sh\n" +
		"if [ \"$1\" = \"api\" ] && [ \"$2\" = \"user\" ]; then\n" +
		"  echo '{\"username\":\"alice\"}'\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 1\n"
	dir := writeStub(t, "glab", stub)
	withStubbedPath(t, dir)

	service := NewService(func(string, string) {}, func(string, string, string) {})
	service.gitHost = gitHostGitLab

	assert.Equal(t, "alice", service.GetAuthenticatedUsername(context.Background()))
}

func TestGetAuthenticatedUsernameGitLabInvalidJSON(t *testing.T) {
	stub := "#!/bin/sh\n" +
		"if [ \"$1\" = \"api\" ] && [ \"$2\" = \"user\" ]; then\n" +
		"  echo '{invalid'\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 1\n"
	dir := writeStub(t, "glab", stub)
	withStubbedPath(t, dir)

	service := NewService(func(string, string) {}, func(string, string, string) {})
	service.gitHost = gitHostGitLab

	assert.Empty(t, service.GetAuthenticatedUsername(context.Background()))
}

func TestGitlabStatusToConclusion(t *testing.T) {
	t.Parallel()
	service := NewService(func(string, string) {}, func(string, string, string) {})

	tests := []struct {
		status   string
		expected string
	}{
		{"success", ciSuccess},
		{"SUCCESS", ciSuccess},
		{"passed", ciSuccess},
		{"PASSED", ciSuccess},
		{"failed", ciFailure},
		{"FAILED", ciFailure},
		{"canceled", ciCancelled},
		{"cancelled", ciCancelled},
		{"skipped", ciSkipped},
		{"SKIPPED", ciSkipped},
		{"running", ciPending},
		{"pending", ciPending},
		{"created", ciPending},
		{"waiting_for_resource", ciPending},
		{"preparing", ciPending},
		{"unknown", "unknown"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			assert.Equal(t, tt.expected, service.gitlabStatusToConclusion(tt.status))
		})
	}
}

func TestFetchGitLabCIParsesPipeline(t *testing.T) {
	ctx := context.Background()
	writeStubCommand(t, "glab", "GLAB_OUTPUT")
	t.Setenv("GLAB_OUTPUT", `{"jobs":[{"name":"build","status":"success"},{"name":"lint","status":"failed"}]}`)

	service := NewService(func(string, string) {}, func(string, string, string) {})
	checks, err := service.fetchGitLabCI(ctx, "main")
	require.NoError(t, err)
	require.Len(t, checks, 2)
	assert.Equal(t, "build", checks[0].Name)
	assert.Equal(t, ciSuccess, checks[0].Conclusion)
	assert.Equal(t, "lint", checks[1].Name)
	assert.Equal(t, ciFailure, checks[1].Conclusion)
}

func TestFetchGitLabCIParsesJobArray(t *testing.T) {
	ctx := context.Background()
	writeStubCommand(t, "glab", "GLAB_OUTPUT")
	t.Setenv("GLAB_OUTPUT", `[{"name":"unit","status":"running"}]`)

	service := NewService(func(string, string) {}, func(string, string, string) {})
	checks, err := service.fetchGitLabCI(ctx, "main")
	require.NoError(t, err)
	require.Len(t, checks, 1)
	assert.Equal(t, "unit", checks[0].Name)
	assert.Equal(t, ciPending, checks[0].Conclusion)
}

func TestFetchGitLabCIInvalidJSON(t *testing.T) {
	ctx := context.Background()
	writeStubCommand(t, "glab", "GLAB_OUTPUT")
	t.Setenv("GLAB_OUTPUT", "not-json")

	service := NewService(func(string, string) {}, func(string, string, string) {})
	_, err := service.fetchGitLabCI(ctx, "main")
	require.Error(t, err)
}
