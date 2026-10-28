package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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

func TestDetectHostUsesOriginRemote(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "remote", "add", "origin", "https://gitlab.com/group/repo.git")
	runGit(t, repo, "remote", "add", "upstream", "https://github.com/canonical/repo.git")
	withCwd(t, repo)

	service := NewService(func(string, string) {}, func(string, string, string) {})
	assert.Equal(t, "upstream", service.resolveRemoteName(ctx))
	assert.Equal(t, gitHostGitLab, service.DetectHost(ctx))
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

// TestResolveCITargetHost covers a repository whose CI and PR lookups follow a
// remote other than origin: the forge must be read from that same remote, since
// the repository name already is.
func TestResolveCITargetHost(t *testing.T) {
	ctx := context.Background()

	t.Run("the forge follows the remote the repository name comes from", func(t *testing.T) {
		repo := t.TempDir()
		runGit(t, repo, "init")
		runGit(t, repo, "remote", "add", "origin", "ssh://example.com/mirror.git")
		runGit(t, repo, "remote", "add", "upstream", "https://github.com/canonical/repo.git")
		withCwd(t, repo)

		service := NewService(func(string, string) {}, func(string, string, string) {})
		assert.Equal(t, "canonical/repo", service.ResolveCITargetRepoName(ctx))
		assert.Equal(t, gitHostGithub, service.ResolveCITargetHost(ctx))
		assert.True(t, service.IsCITargetGitHubOrGitLab(ctx))
	})

	t.Run("an unrecognised forge is reported as such", func(t *testing.T) {
		repo := t.TempDir()
		runGit(t, repo, "init")
		runGit(t, repo, "remote", "add", "origin", "https://gitea.example.com/owner/repo.git")
		withCwd(t, repo)

		service := NewService(func(string, string) {}, func(string, string, string) {})
		assert.Equal(t, gitHostUnknown, service.ResolveCITargetHost(ctx))
		assert.False(t, service.IsCITargetGitHubOrGitLab(ctx))
	})
}

func TestDetectHostSelfManagedGitLabKnownToGlab(t *testing.T) {
	ctx := context.Background()
	stub := "#!/bin/sh\n" +
		"if [ \"$1 $2 $3 $4\" = \"config get api_host --host\" ] && [ \"$5\" = \"code.example.com\" ]; then\n" +
		"  echo code.example.com\n" +
		"fi\n" +
		"exit 0\n"
	withStubbedPath(t, writeStub(t, "glab", stub))

	cases := []struct {
		name   string
		remote string
		want   string
	}{
		{name: "known ssh", remote: "git@code.example.com:group/repo.git", want: gitHostGitLab},
		{name: "known https", remote: "https://code.example.com/group/repo.git", want: gitHostGitLab},
		{name: "not configured", remote: "git@other.example.com:group/repo.git", want: gitHostUnknown},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			runGit(t, repo, "init")
			runGit(t, repo, "remote", "add", "origin", tc.remote)
			withCwd(t, repo)

			service := NewService(func(string, string) {}, func(string, string, string) {})
			assert.Equal(t, tc.want, service.DetectHost(ctx))
		})
	}
}

func TestDetectHostSkipsGlabForKnownForges(t *testing.T) {
	ctx := context.Background()
	marker := filepath.Join(t.TempDir(), "glab-called")
	stub := "#!/bin/sh\n" +
		"touch '" + marker + "'\n" +
		"echo github.com\n" +
		"exit 0\n"
	withStubbedPath(t, writeStub(t, "glab", stub))

	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "remote", "add", "origin", "git@github.com:org/repo.git")
	withCwd(t, repo)

	service := NewService(func(string, string) {}, func(string, string, string) {})
	assert.Equal(t, gitHostGithub, service.DetectHost(ctx))
	assert.NoFileExists(t, marker)
}

func TestHostnameFromRemoteURL(t *testing.T) {
	cases := map[string]string{
		"git@Code.Example.com:group/repo.git":        "code.example.com",
		"https://code.example.com/group/repo.git":    "code.example.com",
		"https://user@code.example.com/group/repo":   "code.example.com",
		"ssh://git@code.example.com:2222/group/repo": "code.example.com",
		"":                 "",
		"/local/path/repo": "",
	}
	for remote, want := range cases {
		assert.Equal(t, want, hostnameFromRemoteURL(remote), remote)
	}
}
