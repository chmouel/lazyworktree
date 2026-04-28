package git

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectHost(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name   string
		remote string
		want   string
	}{
		{name: "github", remote: "git@github.com:org/repo.git", want: gitHostGithub},
		{name: "gitlab", remote: "https://gitlab.com/group/repo.git", want: gitHostGitLab},
		{name: "unknown", remote: "ssh://example.com/repo.git", want: gitHostUnknown},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			runGit(t, repo, "init")
			runGit(t, repo, "remote", "add", "origin", tc.remote)
			withCwd(t, repo)

			service := NewService(func(string, string) {}, func(string, string, string) {})
			if got := service.DetectHost(ctx); got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestIsGitHubOrGitLab(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name   string
		remote string
		want   bool
	}{
		{name: "github", remote: "git@github.com:org/repo.git", want: true},
		{name: "gitlab", remote: "https://gitlab.com/group/repo.git", want: true},
		{name: "unknown", remote: "ssh://example.com/repo.git", want: false},
		{name: "gitea", remote: "https://gitea.example.com/repo.git", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			runGit(t, repo, "init")
			runGit(t, repo, "remote", "add", "origin", tc.remote)
			withCwd(t, repo)

			service := NewService(func(string, string) {}, func(string, string, string) {})
			if got := service.IsGitHubOrGitLab(ctx); got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestResolveRepoName(t *testing.T) {
	t.Run("resolve from github remote url with .git", func(t *testing.T) {
		tmpDir := t.TempDir()

		cmd := exec.Command("git", "init")
		cmd.Dir = tmpDir
		require.NoError(t, cmd.Run())

		cmd = exec.Command("git", "remote", "add", "origin", "https://github.com/owner/repo.git")
		cmd.Dir = tmpDir
		require.NoError(t, cmd.Run())

		oldWd, err := os.Getwd()
		require.NoError(t, err)
		defer func() { _ = os.Chdir(oldWd) }()

		require.NoError(t, os.Chdir(tmpDir))

		service := NewService(func(string, string) {}, func(string, string, string) {})
		assert.Equal(t, "owner/repo", service.ResolveRepoName(context.Background()))
	})

	t.Run("resolve from gitlab remote url with .git", func(t *testing.T) {
		tmpDir := t.TempDir()

		cmd := exec.Command("git", "init")
		cmd.Dir = tmpDir
		require.NoError(t, cmd.Run())

		cmd = exec.Command("git", "remote", "add", "origin", "https://gitlab.com/group/subgroup/project.git")
		cmd.Dir = tmpDir
		require.NoError(t, cmd.Run())

		oldWd, err := os.Getwd()
		require.NoError(t, err)
		defer func() { _ = os.Chdir(oldWd) }()
		require.NoError(t, os.Chdir(tmpDir))

		service := NewService(func(string, string) {}, func(string, string, string) {})
		assert.Equal(t, "group/subgroup/project", service.ResolveRepoName(context.Background()))
	})

	t.Run("resolve from remote url without .git", func(t *testing.T) {
		tmpDir := t.TempDir()

		cmd := exec.Command("git", "init")
		cmd.Dir = tmpDir
		require.NoError(t, cmd.Run())

		cmd = exec.Command("git", "remote", "add", "origin", "https://github.com/owner/repo")
		cmd.Dir = tmpDir
		require.NoError(t, cmd.Run())

		oldWd, err := os.Getwd()
		require.NoError(t, err)
		defer func() { _ = os.Chdir(oldWd) }()
		require.NoError(t, os.Chdir(tmpDir))

		service := NewService(func(string, string) {}, func(string, string, string) {})
		assert.Equal(t, "owner/repo", service.ResolveRepoName(context.Background()))
	})

	t.Run("url-encoded remote segments are decoded", func(t *testing.T) {
		cases := []struct {
			name   string
			remote string
			want   string
		}{
			{name: "space in org", remote: "https://gitea.example.com/Company%20A/myrepo.git", want: "Company A/myrepo"},
			{name: "encoded slash in github org", remote: "https://github.com/org%2Fname/repo.git", want: "org/name/repo"},
			{name: "no encoding", remote: "git@github.com:owner/repo.git", want: "owner/repo"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				repo := t.TempDir()
				runGit(t, repo, "init")
				runGit(t, repo, "remote", "add", "origin", tc.remote)
				withCwd(t, repo)

				service := NewService(func(string, string) {}, func(string, string, string) {})
				assert.Equal(t, tc.want, service.ResolveRepoName(context.Background()))
			})
		}
	})

	t.Run("resolve local key when no remote is configured", func(t *testing.T) {
		tmpDir := t.TempDir()

		cmd := exec.Command("git", "init")
		cmd.Dir = tmpDir
		require.NoError(t, cmd.Run())

		oldWd, err := os.Getwd()
		require.NoError(t, err)
		defer func() { _ = os.Chdir(oldWd) }()
		require.NoError(t, os.Chdir(tmpDir))

		service := NewService(func(string, string) {}, func(string, string, string) {})
		top := service.RunGit(context.Background(), []string{"git", "rev-parse", "--show-toplevel"}, "", []int{0}, true, true)
		require.NotEmpty(t, top)

		assert.Equal(t, localRepoKey(top), service.ResolveRepoName(context.Background()))
	})
}
