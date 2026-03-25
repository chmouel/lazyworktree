package git

import (
	"context"
	"os/exec"
	"testing"

	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchGitHubCIParsesOutput(t *testing.T) {
	ctx := context.Background()
	writeStubCommand(t, "gh", "GH_OUTPUT")
	t.Setenv("GH_OUTPUT", `[{"name":"build","state":"completed","bucket":"pass"}]`)

	service := NewService(func(string, string) {}, func(string, string, string) {})
	checks, err := service.fetchGitHubCI(ctx, 1)
	require.NoError(t, err)
	require.Len(t, checks, 1)
	assert.Equal(t, "build", checks[0].Name)
	assert.Equal(t, "completed", checks[0].Status)
	assert.Equal(t, ciSuccess, checks[0].Conclusion)
}

func TestFetchGitHubCIInvalidJSON(t *testing.T) {
	ctx := context.Background()
	writeStubCommand(t, "gh", "GH_OUTPUT")
	t.Setenv("GH_OUTPUT", "not-json")

	service := NewService(func(string, string) {}, func(string, string, string) {})
	_, err := service.fetchGitHubCI(ctx, 1)
	require.Error(t, err)
}

func TestFetchCIStatus(t *testing.T) {
	t.Parallel()
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}
	service := NewService(notify, notifyOnce)
	service.SetCommandRunner(func(_ context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("echo")
	})
	ctx := context.Background()

	t.Run("fetch CI status without git repository", func(t *testing.T) {
		checks, err := service.FetchCIStatus(ctx, 1, "main")

		if err == nil && checks != nil {
			assert.IsType(t, []*models.CICheck{}, checks)
		}
	})
}

func TestFetchCIStatusByCommit(t *testing.T) {
	ctx := context.Background()

	t.Run("returns nil for non-github host", func(t *testing.T) {
		repo := t.TempDir()
		runGit(t, repo, "init")
		runGit(t, repo, "remote", "add", "origin", "git@gitlab.com:org/repo.git")
		withCwd(t, repo)

		service := NewService(func(string, string) {}, func(string, string, string) {})
		checks, err := service.FetchCIStatusByCommit(ctx, "abc123", repo)

		require.NoError(t, err)
		assert.Nil(t, checks)
	})

	t.Run("returns nil for unknown repo", func(t *testing.T) {
		repo := t.TempDir()
		runGit(t, repo, "init")
		withCwd(t, repo)

		service := NewService(func(string, string) {}, func(string, string, string) {})
		service.gitHost = gitHostGithub
		checks, err := service.FetchCIStatusByCommit(ctx, "abc123", repo)

		require.NoError(t, err)
		assert.Nil(t, checks)
	})

	t.Run("parses github api response correctly", func(t *testing.T) {
		stub := "#!/bin/sh\n" +
			"echo '[{\"name\":\"build\",\"status\":\"completed\",\"conclusion\":\"success\",\"html_url\":\"https://github.com/run/1\",\"started_at\":\"2024-01-15T14:00:00Z\"},{\"name\":\"test\",\"status\":\"in_progress\",\"conclusion\":\"\",\"html_url\":\"https://github.com/run/2\",\"started_at\":\"2024-01-15T14:01:00Z\"}]'\n" +
			"exit 0\n"
		dir := writeStub(t, "gh", stub)
		withStubbedPath(t, dir)

		repo := t.TempDir()
		runGit(t, repo, "init")
		runGit(t, repo, "remote", "add", "origin", "git@github.com:org/repo.git")
		withCwd(t, repo)

		service := NewService(func(string, string) {}, func(string, string, string) {})
		checks, err := service.FetchCIStatusByCommit(ctx, "abc123", repo)

		require.NoError(t, err)
		require.Len(t, checks, 2)
		assert.Equal(t, "build", checks[0].Name)
		assert.Equal(t, "completed", checks[0].Status)
		assert.Equal(t, "success", checks[0].Conclusion)
		assert.Equal(t, "https://github.com/run/1", checks[0].Link)
		assert.Equal(t, "test", checks[1].Name)
		assert.Equal(t, "in_progress", checks[1].Status)
		assert.Equal(t, "pending", checks[1].Conclusion)
	})
}

func TestMapGitHubConclusion(t *testing.T) {
	t.Parallel()
	service := NewService(func(string, string) {}, func(string, string, string) {})

	tests := []struct {
		status     string
		conclusion string
		expected   string
	}{
		{"queued", "", "pending"},
		{"in_progress", "", "pending"},
		{"completed", "success", "success"},
		{"completed", "failure", "failure"},
		{"completed", "neutral", "skipped"},
		{"completed", "skipped", "skipped"},
		{"completed", "cancelled", "cancelled"},
		{"completed", "timed_out", "cancelled"},
		{"completed", "action_required", "cancelled"},
		{"completed", "unknown_value", "unknown_value"},
	}

	for _, tt := range tests {
		t.Run(tt.status+"/"+tt.conclusion, func(t *testing.T) {
			assert.Equal(t, tt.expected, service.mapGitHubConclusion(tt.status, tt.conclusion))
		})
	}
}
