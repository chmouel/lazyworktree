package git

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func newRemoteTestService() *Service {
	return NewService(func(string, string) {}, func(string, string, string) {})
}

func TestResolveRemoteName(t *testing.T) {
	t.Run("prefers upstream when present in auto mode", func(t *testing.T) {
		repo := t.TempDir()
		runGit(t, repo, "init")
		runGit(t, repo, "remote", "add", "origin", "https://github.com/fork/repo.git")
		runGit(t, repo, "remote", "add", "upstream", "https://github.com/canonical/repo.git")
		withCwd(t, repo)

		svc := newRemoteTestService()
		assert.Equal(t, "upstream", svc.resolveRemoteName(context.Background()))
		assert.Equal(t, "canonical/repo", svc.ResolveCITargetRepoName(context.Background()))
		assert.Equal(t, "fork/repo", svc.ResolveRepoName(context.Background()))
	})

	t.Run("falls back to origin when no upstream", func(t *testing.T) {
		repo := t.TempDir()
		runGit(t, repo, "init")
		runGit(t, repo, "remote", "add", "origin", "https://github.com/fork/repo.git")
		withCwd(t, repo)

		svc := newRemoteTestService()
		assert.Equal(t, "origin", svc.resolveRemoteName(context.Background()))
		assert.Equal(t, "fork/repo", svc.ResolveCITargetRepoName(context.Background()))
		assert.Equal(t, "fork/repo", svc.ResolveRepoName(context.Background()))
	})

	t.Run("configured remote takes precedence over upstream", func(t *testing.T) {
		repo := t.TempDir()
		runGit(t, repo, "init")
		runGit(t, repo, "remote", "add", "origin", "https://github.com/fork/repo.git")
		runGit(t, repo, "remote", "add", "upstream", "https://github.com/canonical/repo.git")
		runGit(t, repo, "remote", "add", "fork2", "https://github.com/other/repo.git")
		withCwd(t, repo)

		svc := newRemoteTestService()
		svc.SetCIRemote("fork2")
		assert.Equal(t, "fork2", svc.resolveRemoteName(context.Background()))
		assert.Equal(t, "other/repo", svc.ResolveCITargetRepoName(context.Background()))
		assert.Equal(t, "fork/repo", svc.ResolveRepoName(context.Background()))
	})

	t.Run("a remote literally named auto is honoured when set programmatically", func(t *testing.T) {
		// "auto" is normalised to "" (automatic mode) by the config parser, so a
		// user cannot target a remote literally named "auto" via ci_remote config.
		// However, SetCIRemote is called programmatically here to verify that the
		// resolution logic itself correctly handles any non-empty remote name.
		repo := t.TempDir()
		runGit(t, repo, "init")
		runGit(t, repo, "remote", "add", "origin", "https://github.com/fork/repo.git")
		runGit(t, repo, "remote", "add", "upstream", "https://github.com/canonical/repo.git")
		runGit(t, repo, "remote", "add", "auto", "https://github.com/team/repo.git")
		withCwd(t, repo)

		svc := newRemoteTestService()
		svc.SetCIRemote("auto")
		assert.Equal(t, "auto", svc.resolveRemoteName(context.Background()))
		assert.Equal(t, "team/repo", svc.ResolveCITargetRepoName(context.Background()))
		assert.Equal(t, "fork/repo", svc.ResolveRepoName(context.Background()))
	})

	t.Run("configured origin disables upstream preference", func(t *testing.T) {
		repo := t.TempDir()
		runGit(t, repo, "init")
		runGit(t, repo, "remote", "add", "origin", "https://github.com/fork/repo.git")
		runGit(t, repo, "remote", "add", "upstream", "https://github.com/canonical/repo.git")
		withCwd(t, repo)

		svc := newRemoteTestService()
		svc.SetCIRemote("origin")
		assert.Equal(t, "origin", svc.resolveRemoteName(context.Background()))
		assert.Equal(t, "fork/repo", svc.ResolveCITargetRepoName(context.Background()))
		assert.Equal(t, "fork/repo", svc.ResolveRepoName(context.Background()))
	})

	t.Run("configured but missing remote falls back to origin", func(t *testing.T) {
		repo := t.TempDir()
		runGit(t, repo, "init")
		runGit(t, repo, "remote", "add", "origin", "https://github.com/fork/repo.git")
		withCwd(t, repo)

		svc := newRemoteTestService()
		svc.SetCIRemote("upstream")
		assert.Equal(t, "origin", svc.resolveRemoteName(context.Background()))
		assert.Equal(t, "fork/repo", svc.ResolveCITargetRepoName(context.Background()))
	})
}

func TestGHRepoArgs(t *testing.T) {
	t.Run("returns repo flag when targeting non-origin remote", func(t *testing.T) {
		repo := t.TempDir()
		runGit(t, repo, "init")
		runGit(t, repo, "remote", "add", "origin", "https://github.com/fork/repo.git")
		runGit(t, repo, "remote", "add", "upstream", "https://github.com/canonical/repo.git")
		withCwd(t, repo)

		svc := newRemoteTestService()
		assert.Equal(t, []string{"--repo", "canonical/repo"}, svc.ghRepoArgs(context.Background()))
	})

	t.Run("pins origin when explicitly selected", func(t *testing.T) {
		repo := t.TempDir()
		runGit(t, repo, "init")
		runGit(t, repo, "remote", "add", "origin", "https://github.com/fork/repo.git")
		withCwd(t, repo)

		svc := newRemoteTestService()
		svc.SetCIRemote("origin")
		assert.Equal(t, []string{"--repo", "fork/repo"}, svc.ghRepoArgs(context.Background()))
	})

	t.Run("returns nil when auto mode resolves to origin", func(t *testing.T) {
		repo := t.TempDir()
		runGit(t, repo, "init")
		runGit(t, repo, "remote", "add", "origin", "https://github.com/fork/repo.git")
		withCwd(t, repo)

		svc := newRemoteTestService()
		assert.Nil(t, svc.ghRepoArgs(context.Background()))
	})

	t.Run("returns nil when repo name resolves to a local key", func(t *testing.T) {
		repo := t.TempDir()
		runGit(t, repo, "init")
		// A single-segment local path cannot be mapped to an owner/repo pair, so
		// the CI target resolves to a local-* cache key and no --repo flag
		// should be produced even though upstream is the resolved remote.
		runGit(t, repo, "remote", "add", "upstream", "/srv")
		withCwd(t, repo)

		svc := newRemoteTestService()
		assert.Equal(t, "upstream", svc.resolveRemoteName(context.Background()))
		assert.Nil(t, svc.ghRepoArgs(context.Background()))
	})
}
