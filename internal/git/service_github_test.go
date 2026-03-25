package git

import (
	"context"
	"os/exec"
	"testing"

	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchPRMap(t *testing.T) {
	t.Parallel()
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}

	service := NewService(notify, notifyOnce)
	service.SetCommandRunner(func(_ context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("echo")
	})
	ctx := context.Background()

	t.Run("fetch PR map without git repository", func(t *testing.T) {
		// This test just verifies the function doesn't panic.
		// Behaviour varies by git environment (may return error or empty map).
		prMap, err := service.FetchPRMap(ctx)

		if err == nil && prMap != nil {
			assert.IsType(t, map[string]*models.PRInfo{}, prMap)
		}
	})
}

func TestFetchPRMapUnknownHost(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "remote", "add", "origin", "https://gitea.example.com/repo.git")
	withCwd(t, repo)

	service := NewService(func(string, string) {}, func(string, string, string) {})
	prMap, err := service.FetchPRMap(ctx)
	if err != nil {
		t.Fatalf("expected no error for unknown host, got: %v", err)
	}
	if prMap == nil {
		t.Fatal("expected non-nil map for unknown host")
	}
	if len(prMap) != 0 {
		t.Fatalf("expected empty map for unknown host, got %d entries", len(prMap))
	}
}

func TestFetchPRForWorktree(t *testing.T) {
	t.Parallel()
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}

	service := NewService(notify, notifyOnce)
	service.SetCommandRunner(func(_ context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("echo")
	})
	ctx := context.Background()

	t.Run("fetch PR for non-existent worktree returns nil", func(t *testing.T) {
		pr := service.FetchPRForWorktree(ctx, "/non/existent/path")
		assert.Nil(t, pr)
	})

	t.Run("fetch PR for worktree without PR returns nil", func(t *testing.T) {
		tmpDir := t.TempDir()
		pr := service.FetchPRForWorktree(ctx, tmpDir)
		assert.Nil(t, pr)
	})
}

func TestFetchAllOpenPRs(t *testing.T) {
	t.Parallel()
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}

	service := NewService(notify, notifyOnce)
	service.SetCommandRunner(func(_ context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("echo")
	})
	ctx := context.Background()

	t.Run("fetch open PRs without git repository", func(t *testing.T) {
		prs, err := service.FetchAllOpenPRs(ctx)

		if err == nil {
			assert.IsType(t, []*models.PRInfo{}, prs)
		} else {
			assert.Error(t, err)
		}
	})
}

func TestFetchAllOpenIssuesGitHub(t *testing.T) {
	stub := "#!/bin/sh\n" +
		"if [ \"$1\" = \"issue\" ] && [ \"$2\" = \"list\" ]; then\n" +
		"  echo '[{\"number\":1,\"state\":\"open\",\"title\":\"Issue One\",\"body\":\"Description\",\"url\":\"https://github.com/repo/issues/1\",\"author\":{\"login\":\"user1\",\"name\":\"User One\",\"is_bot\":false}},{\"number\":2,\"state\":\"closed\",\"title\":\"Issue Two\",\"body\":\"Description\",\"url\":\"https://github.com/repo/issues/2\",\"author\":{\"login\":\"user2\",\"name\":\"User Two\",\"is_bot\":true}}]'\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 0\n"
	dir := writeStub(t, "gh", stub)
	withStubbedPath(t, dir)

	service := NewService(func(string, string) {}, func(string, string, string) {})
	service.gitHost = gitHostGithub

	issues, err := service.FetchAllOpenIssues(context.Background())
	require.NoError(t, err)
	require.Len(t, issues, 1)

	issue := issues[0]
	assert.Equal(t, 1, issue.Number)
	assert.Equal(t, "open", issue.State)
	assert.Equal(t, "Issue One", issue.Title)
	assert.Equal(t, "Description", issue.Body)
	assert.Equal(t, "https://github.com/repo/issues/1", issue.URL)
	assert.Equal(t, "user1", issue.Author)
	assert.Equal(t, "User One", issue.AuthorName)
	assert.False(t, issue.AuthorIsBot)
}

func TestFetchAllOpenIssuesEmptyResponse(t *testing.T) {
	stub := "#!/bin/sh\n" +
		"if [ \"$1\" = \"issue\" ] && [ \"$2\" = \"list\" ]; then\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 0\n"
	dir := writeStub(t, "gh", stub)
	withStubbedPath(t, dir)

	service := NewService(func(string, string) {}, func(string, string, string) {})
	service.gitHost = gitHostGithub

	issues, err := service.FetchAllOpenIssues(context.Background())
	require.NoError(t, err)
	assert.Empty(t, issues)
}

func TestFetchAllOpenIssuesEmptyArray(t *testing.T) {
	stub := "#!/bin/sh\n" +
		"if [ \"$1\" = \"issue\" ] && [ \"$2\" = \"list\" ]; then\n" +
		"  echo '[]'\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 0\n"
	dir := writeStub(t, "gh", stub)
	withStubbedPath(t, dir)

	service := NewService(func(string, string) {}, func(string, string, string) {})
	service.gitHost = gitHostGithub

	issues, err := service.FetchAllOpenIssues(context.Background())
	require.NoError(t, err)
	assert.Empty(t, issues)
}

func TestFetchAllOpenIssuesInvalidJSON(t *testing.T) {
	stub := "#!/bin/sh\n" +
		"if [ \"$1\" = \"issue\" ] && [ \"$2\" = \"list\" ]; then\n" +
		"  echo 'invalid json'\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 0\n"
	dir := writeStub(t, "gh", stub)
	withStubbedPath(t, dir)

	notified := false
	notifyOnce := func(key, msg, severity string) {
		if key == "issue_json_decode" && severity == "error" {
			notified = true
		}
	}

	service := NewService(func(string, string) {}, notifyOnce)
	service.gitHost = gitHostGithub

	issues, err := service.FetchAllOpenIssues(context.Background())
	require.Error(t, err)
	assert.Nil(t, issues)
	assert.True(t, notified, "expected notification for JSON decode error")
}

func TestFetchIssueGitHub(t *testing.T) {
	stub := "#!/bin/sh\n" +
		"if [ \"$1\" = \"issue\" ] && [ \"$2\" = \"view\" ] && [ \"$3\" = \"42\" ]; then\n" +
		"  echo '{\"number\":42,\"state\":\"open\",\"title\":\"Test Issue\",\"body\":\"Test body\",\"url\":\"https://github.com/repo/issues/42\",\"author\":{\"login\":\"testuser\",\"name\":\"Test User\",\"is_bot\":false}}'\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 1\n"
	dir := writeStub(t, "gh", stub)
	withStubbedPath(t, dir)

	service := NewService(func(string, string) {}, func(string, string, string) {})
	service.gitHost = gitHostGithub

	issue, err := service.FetchIssue(context.Background(), 42)
	require.NoError(t, err)
	require.NotNil(t, issue)

	assert.Equal(t, 42, issue.Number)
	assert.Equal(t, "open", issue.State)
	assert.Equal(t, "Test Issue", issue.Title)
	assert.Equal(t, "Test body", issue.Body)
	assert.Equal(t, "https://github.com/repo/issues/42", issue.URL)
	assert.Equal(t, "testuser", issue.Author)
	assert.Equal(t, "Test User", issue.AuthorName)
	assert.False(t, issue.AuthorIsBot)
}

func TestFetchIssueNotFound(t *testing.T) {
	stub := "#!/bin/sh\nexit 0\n"
	dir := writeStub(t, "gh", stub)
	withStubbedPath(t, dir)

	service := NewService(func(string, string) {}, func(string, string, string) {})
	service.gitHost = gitHostGithub

	issue, err := service.FetchIssue(context.Background(), 999)
	require.Error(t, err)
	assert.Nil(t, issue)
	assert.Contains(t, err.Error(), "not found")
}

func TestFetchIssueClosed(t *testing.T) {
	stub := "#!/bin/sh\n" +
		"if [ \"$1\" = \"issue\" ] && [ \"$2\" = \"view\" ] && [ \"$3\" = \"42\" ]; then\n" +
		"  echo '{\"number\":42,\"state\":\"closed\",\"title\":\"Closed Issue\",\"body\":\"Test\",\"url\":\"https://github.com/repo/issues/42\",\"author\":{\"login\":\"user\",\"name\":\"User\",\"is_bot\":false}}'\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 1\n"
	dir := writeStub(t, "gh", stub)
	withStubbedPath(t, dir)

	service := NewService(func(string, string) {}, func(string, string, string) {})
	service.gitHost = gitHostGithub

	issue, err := service.FetchIssue(context.Background(), 42)
	require.Error(t, err)
	assert.Nil(t, issue)
	assert.Contains(t, err.Error(), "not open")
}

func TestFetchPRGitHub(t *testing.T) {
	stub := "#!/bin/sh\n" +
		"if [ \"$1\" = \"pr\" ] && [ \"$2\" = \"view\" ] && [ \"$3\" = \"123\" ]; then\n" +
		"  echo '{\"number\":123,\"state\":\"OPEN\",\"title\":\"Test PR\",\"body\":\"Test body\",\"url\":\"https://github.com/repo/pull/123\",\"headRefName\":\"feature-branch\",\"baseRefName\":\"main\",\"author\":{\"login\":\"testuser\",\"name\":\"Test User\",\"is_bot\":false},\"isDraft\":false,\"statusCheckRollup\":[{\"conclusion\":\"SUCCESS\"}]}'\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 1\n"
	dir := writeStub(t, "gh", stub)
	withStubbedPath(t, dir)

	service := NewService(func(string, string) {}, func(string, string, string) {})
	service.gitHost = gitHostGithub

	pr, err := service.FetchPR(context.Background(), 123)
	require.NoError(t, err)
	require.NotNil(t, pr)

	assert.Equal(t, 123, pr.Number)
	assert.Equal(t, "OPEN", pr.State)
	assert.Equal(t, "Test PR", pr.Title)
	assert.Equal(t, "Test body", pr.Body)
	assert.Equal(t, "https://github.com/repo/pull/123", pr.URL)
	assert.Equal(t, "feature-branch", pr.Branch)
	assert.Equal(t, "main", pr.BaseBranch)
	assert.Equal(t, "testuser", pr.Author)
	assert.Equal(t, "Test User", pr.AuthorName)
	assert.False(t, pr.AuthorIsBot)
	assert.False(t, pr.IsDraft)
}

func TestFetchPRNotFound(t *testing.T) {
	stub := "#!/bin/sh\nexit 0\n"
	dir := writeStub(t, "gh", stub)
	withStubbedPath(t, dir)

	service := NewService(func(string, string) {}, func(string, string, string) {})
	service.gitHost = gitHostGithub

	pr, err := service.FetchPR(context.Background(), 999)
	require.Error(t, err)
	assert.Nil(t, pr)
	assert.Contains(t, err.Error(), "not found")
}

func TestFetchPRClosed(t *testing.T) {
	stub := "#!/bin/sh\n" +
		"if [ \"$1\" = \"pr\" ] && [ \"$2\" = \"view\" ] && [ \"$3\" = \"123\" ]; then\n" +
		"  echo '{\"number\":123,\"state\":\"CLOSED\",\"title\":\"Closed PR\",\"body\":\"Test\",\"url\":\"https://github.com/repo/pull/123\",\"headRefName\":\"feature\",\"baseRefName\":\"main\",\"author\":{\"login\":\"user\",\"name\":\"User\",\"is_bot\":false},\"isDraft\":false}'\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 1\n"
	dir := writeStub(t, "gh", stub)
	withStubbedPath(t, dir)

	service := NewService(func(string, string) {}, func(string, string, string) {})
	service.gitHost = gitHostGithub

	pr, err := service.FetchPR(context.Background(), 123)
	require.Error(t, err)
	assert.Nil(t, pr)
	assert.Contains(t, err.Error(), "not open")
}

func TestGetAuthenticatedUsernameGitHub(t *testing.T) {
	stub := "#!/bin/sh\n" +
		"if [ \"$1\" = \"api\" ] && [ \"$2\" = \"user\" ]; then\n" +
		"  echo 'octocat'\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 1\n"
	dir := writeStub(t, "gh", stub)
	withStubbedPath(t, dir)

	service := NewService(func(string, string) {}, func(string, string, string) {})
	service.gitHost = gitHostGithub

	assert.Equal(t, "octocat", service.GetAuthenticatedUsername(context.Background()))
}

func TestGithubBucketToConclusion(t *testing.T) {
	t.Parallel()
	service := NewService(func(string, string) {}, func(string, string, string) {})

	tests := []struct {
		bucket   string
		expected string
	}{
		{"pass", ciSuccess},
		{"PASS", ciSuccess},
		{"fail", ciFailure},
		{"FAIL", ciFailure},
		{"skipping", ciSkipped},
		{"SKIPPING", ciSkipped},
		{"cancel", ciCancelled},
		{"CANCEL", ciCancelled},
		{"pending", ciPending},
		{"PENDING", ciPending},
		{"unknown", "unknown"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.bucket, func(t *testing.T) {
			result := service.githubBucketToConclusion(tt.bucket)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestComputeCIStatusFromRollup(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		rollup   any
		expected string
	}{
		{name: "nil rollup", rollup: nil, expected: "none"},
		{name: "empty rollup", rollup: []any{}, expected: "none"},
		{
			name: "all success",
			rollup: []any{
				map[string]any{"conclusion": "SUCCESS", "status": "COMPLETED"},
				map[string]any{"conclusion": "SUCCESS", "status": "COMPLETED"},
			},
			expected: "success",
		},
		{
			name: "one failure",
			rollup: []any{
				map[string]any{"conclusion": "SUCCESS", "status": "COMPLETED"},
				map[string]any{"conclusion": "FAILURE", "status": "COMPLETED"},
			},
			expected: "failure",
		},
		{
			name: "cancelled counts as failure",
			rollup: []any{
				map[string]any{"conclusion": "CANCELLED", "status": "COMPLETED"},
			},
			expected: "failure",
		},
		{
			name: "pending status",
			rollup: []any{
				map[string]any{"conclusion": "", "status": "IN_PROGRESS"},
			},
			expected: "pending",
		},
		{
			name: "mixed success and pending",
			rollup: []any{
				map[string]any{"conclusion": "SUCCESS", "status": "COMPLETED"},
				map[string]any{"conclusion": "", "status": "QUEUED"},
			},
			expected: "pending",
		},
		{
			name: "failure takes precedence over pending",
			rollup: []any{
				map[string]any{"conclusion": "FAILURE", "status": "COMPLETED"},
				map[string]any{"conclusion": "", "status": "IN_PROGRESS"},
			},
			expected: "failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, computeCIStatusFromRollup(tt.rollup))
		})
	}
}
