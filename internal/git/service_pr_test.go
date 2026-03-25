package git

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateWorktreeFromPR(t *testing.T) {
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}
	ctx := context.Background()

	t.Run("create worktree from PR with temporary directory", func(t *testing.T) {
		service := NewService(notify, notifyOnce)
		withCwd(t, t.TempDir())
		targetPath := filepath.Join(t.TempDir(), "test-worktree")

		ok := service.CreateWorktreeFromPR(ctx, 123, "feature-branch", "local-branch", targetPath)
		assert.IsType(t, true, ok)
	})

	t.Run("unknown host uses manual fetch fallback", func(t *testing.T) {
		service := NewService(notify, notifyOnce)
		repo := t.TempDir()
		runGit(t, repo, "init", "-b", "main")
		runGit(t, repo, "config", "user.email", "test@test.com")
		runGit(t, repo, "config", "user.name", "Test User")
		runGit(t, repo, "config", "commit.gpgsign", "false")

		testFile := filepath.Join(repo, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("initial"), 0o600))
		runGit(t, repo, "add", "test.txt")
		runGit(t, repo, "commit", "-m", "initial")

		runGit(t, repo, "checkout", "-b", "feature-branch")
		require.NoError(t, os.WriteFile(testFile, []byte("feature"), 0o600))
		runGit(t, repo, "commit", "-am", "feature commit")
		runGit(t, repo, "checkout", "main")

		workRepo := t.TempDir()
		runGit(t, workRepo, "clone", repo, ".")
		runGit(t, workRepo, "config", "user.email", "test@test.com")
		runGit(t, workRepo, "config", "user.name", "Test User")
		runGit(t, workRepo, "config", "commit.gpgsign", "false")
		runGit(t, workRepo, "remote", "set-url", "origin", "https://gitea.example.com/org/repo.git")
		runGit(t, workRepo, "fetch", repo, "feature-branch:refs/remotes/origin/feature-branch")

		withCwd(t, workRepo)

		targetPath := filepath.Join(t.TempDir(), "pr-worktree")
		ok := service.CreateWorktreeFromPR(ctx, 1, "feature-branch", "local-pr-branch", targetPath)
		assert.False(t, ok)
	})

	t.Run("returns false when not in git repo", func(t *testing.T) {
		service := NewService(notify, notifyOnce)
		tmpDir := t.TempDir()
		withCwd(t, tmpDir)

		targetPath := filepath.Join(tmpDir, "worktree")
		ok := service.CreateWorktreeFromPR(ctx, 1, "feature", "local", targetPath)
		assert.False(t, ok)
	})

	t.Run("returns false for invalid target path", func(t *testing.T) {
		service := NewService(notify, notifyOnce)
		repo := t.TempDir()
		runGit(t, repo, "init")
		runGit(t, repo, "remote", "add", "origin", "https://bitbucket.example.com/org/repo.git")
		withCwd(t, repo)

		invalidPath := "/nonexistent/deeply/nested/path/worktree"
		ok := service.CreateWorktreeFromPR(ctx, 1, "feature", "local", invalidPath)
		assert.False(t, ok)
	})

	t.Run("existing local branch is reset to PR branch before worktree creation", func(t *testing.T) {
		var notifications []string
		service := NewService(
			func(msg, level string) {
				if level == "warning" {
					notifications = append(notifications, msg)
				}
			},
			notifyOnce,
		)
		remoteRepo := t.TempDir()
		runGit(t, remoteRepo, "init", "--bare", "-b", "main")

		setupRepo := t.TempDir()
		runGit(t, setupRepo, "clone", remoteRepo, ".")
		runGit(t, setupRepo, "config", "user.email", "test@test.com")
		runGit(t, setupRepo, "config", "user.name", "Test User")
		runGit(t, setupRepo, "config", "commit.gpgsign", "false")

		testFile := filepath.Join(setupRepo, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("initial"), 0o600))
		runGit(t, setupRepo, "add", "test.txt")
		runGit(t, setupRepo, "commit", "-m", "initial")
		runGit(t, setupRepo, "push", "-u", "origin", "main")

		runGit(t, setupRepo, "checkout", "-b", "feature-branch")
		require.NoError(t, os.WriteFile(testFile, []byte("feature content"), 0o600))
		runGit(t, setupRepo, "commit", "-am", "feature commit")
		runGit(t, setupRepo, "push", "-u", "origin", "feature-branch")
		featureSHA := runGit(t, setupRepo, "rev-parse", "HEAD")

		testRepo := t.TempDir()
		runGit(t, testRepo, "clone", remoteRepo, ".")
		runGit(t, testRepo, "config", "user.email", "test@test.com")
		runGit(t, testRepo, "config", "user.name", "Test User")
		runGit(t, testRepo, "config", "commit.gpgsign", "false")

		runGit(t, testRepo, "checkout", "-b", "feature-branch", "origin/main")
		runGit(t, testRepo, "checkout", "main")

		withCwd(t, testRepo)
		targetPath := filepath.Join(t.TempDir(), "feature-branch")
		ok := service.CreateWorktreeFromPR(ctx, 1, "feature-branch", "feature-branch", targetPath)
		require.True(t, ok)

		gotSHA := runGit(t, testRepo, "rev-parse", "feature-branch")
		assert.Equal(t, featureSHA, gotSHA)
		assert.Equal(t, "feature-branch", runGit(t, targetPath, "rev-parse", "--abbrev-ref", "HEAD"))
		assert.Equal(t, "origin", runGit(t, testRepo, "config", "--get", "branch.feature-branch.remote"))
		assert.Equal(t, "origin", runGit(t, testRepo, "config", "--get", "branch.feature-branch.pushRemote"))
		assert.Equal(t, "refs/heads/feature-branch", runGit(t, testRepo, "config", "--get", "branch.feature-branch.merge"))
		require.NotEmpty(t, notifications)
		assert.Contains(t, notifications[0], "already exists and will be reset")
	})

	t.Run("returns false when PR branch is already attached to another worktree", func(t *testing.T) {
		service := NewService(notify, notifyOnce)
		remoteRepo := t.TempDir()
		runGit(t, remoteRepo, "init", "--bare", "-b", "main")

		setupRepo := t.TempDir()
		runGit(t, setupRepo, "clone", remoteRepo, ".")
		runGit(t, setupRepo, "config", "user.email", "test@test.com")
		runGit(t, setupRepo, "config", "user.name", "Test User")
		runGit(t, setupRepo, "config", "commit.gpgsign", "false")

		testFile := filepath.Join(setupRepo, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("initial"), 0o600))
		runGit(t, setupRepo, "add", "test.txt")
		runGit(t, setupRepo, "commit", "-m", "initial")
		runGit(t, setupRepo, "push", "-u", "origin", "main")

		runGit(t, setupRepo, "checkout", "-b", "feature-branch")
		require.NoError(t, os.WriteFile(testFile, []byte("feature content"), 0o600))
		runGit(t, setupRepo, "commit", "-am", "feature commit")
		runGit(t, setupRepo, "push", "-u", "origin", "feature-branch")

		testRepo := t.TempDir()
		runGit(t, testRepo, "clone", remoteRepo, ".")
		runGit(t, testRepo, "config", "user.email", "test@test.com")
		runGit(t, testRepo, "config", "user.name", "Test User")
		runGit(t, testRepo, "config", "commit.gpgsign", "false")

		attachedPath := filepath.Join(t.TempDir(), "attached-feature")
		runGit(t, testRepo, "worktree", "add", "-b", "feature-branch", attachedPath, "origin/feature-branch")

		withCwd(t, testRepo)
		targetPath := filepath.Join(t.TempDir(), "new-feature-worktree")
		ok := service.CreateWorktreeFromPR(ctx, 1, "feature-branch", "feature-branch", targetPath)
		assert.False(t, ok)
	})

	t.Run("GitLab MR ref from glab api merge_requests succeeds", func(t *testing.T) {
		remoteRepo := t.TempDir()
		runGit(t, remoteRepo, "init", "--bare", "-b", "main")

		setupRepo := t.TempDir()
		runGit(t, setupRepo, "clone", remoteRepo, ".")
		runGit(t, setupRepo, "config", "user.email", "test@test.com")
		runGit(t, setupRepo, "config", "user.name", "Test User")
		runGit(t, setupRepo, "config", "commit.gpgsign", "false")

		testFile := filepath.Join(setupRepo, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("initial"), 0o600))
		runGit(t, setupRepo, "add", "test.txt")
		runGit(t, setupRepo, "commit", "-m", "initial")
		runGit(t, setupRepo, "push", "-u", "origin", "main")

		runGit(t, setupRepo, "checkout", "-b", "feature-branch")
		require.NoError(t, os.WriteFile(testFile, []byte("feature content"), 0o600))
		runGit(t, setupRepo, "commit", "-am", "feature commit")
		runGit(t, setupRepo, "push", "-u", "origin", "feature-branch")
		featureSHA := runGit(t, setupRepo, "rev-parse", "HEAD")

		testRepo := t.TempDir()
		runGit(t, testRepo, "clone", remoteRepo, ".")
		runGit(t, testRepo, "config", "user.email", "test@test.com")
		runGit(t, testRepo, "config", "user.name", "Test User")
		runGit(t, testRepo, "config", "commit.gpgsign", "false")

		stub := "#!/bin/sh\n" +
			"if [ \"$1\" = \"api\" ] && [ \"$2\" = \"merge_requests/1\" ]; then\n" +
			"  echo '{\"sha\":\"" + featureSHA + "\",\"source_branch\":\"feature-branch\"}'\n" +
			"  exit 0\n" +
			"fi\n" +
			"exit 1\n"
		dir := writeStub(t, "glab", stub)
		withStubbedPath(t, dir)

		service := NewService(notify, notifyOnce)
		service.gitHost = gitHostGitLab

		withCwd(t, testRepo)
		targetPath := filepath.Join(t.TempDir(), "feature-branch")
		ok := service.CreateWorktreeFromPR(ctx, 1, "feature-branch", "feature-branch", targetPath)
		require.True(t, ok)

		assert.Equal(t, featureSHA, runGit(t, testRepo, "rev-parse", "feature-branch"))
		assert.Equal(t, "feature-branch", runGit(t, targetPath, "rev-parse", "--abbrev-ref", "HEAD"))
	})

	t.Run("GitLab MR ref uses diff_refs.head_sha when sha missing", func(t *testing.T) {
		remoteRepo := t.TempDir()
		runGit(t, remoteRepo, "init", "--bare", "-b", "main")

		setupRepo := t.TempDir()
		runGit(t, setupRepo, "clone", remoteRepo, ".")
		runGit(t, setupRepo, "config", "user.email", "test@test.com")
		runGit(t, setupRepo, "config", "user.name", "Test User")
		runGit(t, setupRepo, "config", "commit.gpgsign", "false")

		testFile := filepath.Join(setupRepo, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("initial"), 0o600))
		runGit(t, setupRepo, "add", "test.txt")
		runGit(t, setupRepo, "commit", "-m", "initial")
		runGit(t, setupRepo, "push", "-u", "origin", "main")

		runGit(t, setupRepo, "checkout", "-b", "feature-branch")
		require.NoError(t, os.WriteFile(testFile, []byte("feature content"), 0o600))
		runGit(t, setupRepo, "commit", "-am", "feature commit")
		runGit(t, setupRepo, "push", "-u", "origin", "feature-branch")
		featureSHA := runGit(t, setupRepo, "rev-parse", "HEAD")

		testRepo := t.TempDir()
		runGit(t, testRepo, "clone", remoteRepo, ".")
		runGit(t, testRepo, "config", "user.email", "test@test.com")
		runGit(t, testRepo, "config", "user.name", "Test User")
		runGit(t, testRepo, "config", "commit.gpgsign", "false")

		stub := "#!/bin/sh\n" +
			"if [ \"$1\" = \"api\" ] && [ \"$2\" = \"merge_requests/1\" ]; then\n" +
			"  echo '{\"diff_refs\":{\"head_sha\":\"" + featureSHA + "\"},\"source_branch\":\"feature-branch\"}'\n" +
			"  exit 0\n" +
			"fi\n" +
			"exit 1\n"
		dir := writeStub(t, "glab", stub)
		withStubbedPath(t, dir)

		service := NewService(notify, notifyOnce)
		service.gitHost = gitHostGitLab

		withCwd(t, testRepo)
		targetPath := filepath.Join(t.TempDir(), "feature-branch")
		ok := service.CreateWorktreeFromPR(ctx, 1, "feature-branch", "feature-branch", targetPath)
		require.True(t, ok)

		assert.Equal(t, featureSHA, runGit(t, testRepo, "rev-parse", "feature-branch"))
		assert.Equal(t, "feature-branch", runGit(t, targetPath, "rev-parse", "--abbrev-ref", "HEAD"))
	})
}

func TestCreateWorktreeFromPRUnknownHostSuccess(t *testing.T) {
	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}

	service := NewService(notify, notifyOnce)
	ctx := context.Background()

	remoteRepo := t.TempDir()
	runGit(t, remoteRepo, "init", "--bare", "-b", "main")

	workSetup := t.TempDir()
	runGit(t, workSetup, "clone", remoteRepo, ".")
	runGit(t, workSetup, "config", "user.email", "test@test.com")
	runGit(t, workSetup, "config", "user.name", "Test User")
	runGit(t, workSetup, "config", "commit.gpgsign", "false")

	testFile := filepath.Join(workSetup, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("initial"), 0o600))
	runGit(t, workSetup, "add", "test.txt")
	runGit(t, workSetup, "commit", "-m", "initial")
	runGit(t, workSetup, "push", "-u", "origin", "main")

	runGit(t, workSetup, "checkout", "-b", "feature-branch")
	require.NoError(t, os.WriteFile(testFile, []byte("feature content"), 0o600))
	runGit(t, workSetup, "commit", "-am", "feature commit")
	runGit(t, workSetup, "push", "-u", "origin", "feature-branch")

	testRepo := t.TempDir()
	runGit(t, testRepo, "clone", remoteRepo, ".")
	runGit(t, testRepo, "config", "user.email", "test@test.com")
	runGit(t, testRepo, "config", "user.name", "Test User")
	runGit(t, testRepo, "config", "commit.gpgsign", "false")

	runGit(t, testRepo, "remote", "set-url", "origin", remoteRepo)
	runGit(t, testRepo, "config", "remote.origin.gh-resolved", "false")

	withCwd(t, testRepo)

	targetPath := filepath.Join(t.TempDir(), "pr-worktree")
	ok := service.CreateWorktreeFromPR(ctx, 1, "feature-branch", "local-pr-branch", targetPath)
	require.True(t, ok)
	assert.Equal(t, "origin", runGit(t, testRepo, "config", "--get", "branch.local-pr-branch.remote"))
	assert.Equal(t, "origin", runGit(t, testRepo, "config", "--get", "branch.local-pr-branch.pushRemote"))
	assert.Equal(t, "refs/heads/feature-branch", runGit(t, testRepo, "config", "--get", "branch.local-pr-branch.merge"))
}

func TestCreateWorktreeFromPRBranchTracking(t *testing.T) {
	t.Parallel()

	t.Run("github tracking config format", func(t *testing.T) {
		localBranch := "pr-123-feature"
		sourceBranch := "feature-branch"
		assert.Equal(t, "branch.pr-123-feature.remote", "branch."+localBranch+".remote")
		assert.Equal(t, "branch.pr-123-feature.pushRemote", "branch."+localBranch+".pushRemote")
		assert.Equal(t, "branch.pr-123-feature.merge", "branch."+localBranch+".merge")
		assert.Equal(t, "refs/heads/feature-branch", "refs/heads/"+sourceBranch)
	})

	t.Run("gitlab tracking config format", func(t *testing.T) {
		localBranch := "mr-456-feature"
		sourceBranch := "feature-branch"
		assert.Equal(t, "branch.mr-456-feature.remote", "branch."+localBranch+".remote")
		assert.Equal(t, "branch.mr-456-feature.merge", "branch."+localBranch+".merge")
		assert.Equal(t, "refs/heads/feature-branch", "refs/heads/"+sourceBranch)
	})
}

func TestCreateWorktreeFromPRJSONParsing(t *testing.T) {
	t.Parallel()

	t.Run("parse github pr json", func(t *testing.T) {
		jsonData := `{"headRefOid":"abc123def456","headRepository":{"url":"https://github.com/fork/repo"}}`

		var pr map[string]any
		err := json.Unmarshal([]byte(jsonData), &pr)
		require.NoError(t, err)

		headCommit, _ := pr["headRefOid"].(string)
		assert.Equal(t, "abc123def456", headCommit)

		var repoURL string
		if headRepo, ok := pr["headRepository"].(map[string]any); ok {
			repoURL, _ = headRepo["url"].(string)
		}
		assert.Equal(t, "https://github.com/fork/repo", repoURL)
	})

	t.Run("parse github pr json without headRepository", func(t *testing.T) {
		jsonData := `{"headRefOid":"abc123def456"}`

		var pr map[string]any
		err := json.Unmarshal([]byte(jsonData), &pr)
		require.NoError(t, err)

		headCommit, _ := pr["headRefOid"].(string)
		assert.Equal(t, "abc123def456", headCommit)

		var repoURL string
		if headRepo, ok := pr["headRepository"].(map[string]any); ok {
			repoURL, _ = headRepo["url"].(string)
		}
		assert.Empty(t, repoURL)
	})

	t.Run("parse gitlab mr json", func(t *testing.T) {
		jsonData := `{"sha":"def789ghi012","source_branch":"feature-xyz","web_url":"https://gitlab.com/org/repo/-/merge_requests/42"}`

		var mr map[string]any
		err := json.Unmarshal([]byte(jsonData), &mr)
		require.NoError(t, err)

		sha, _ := mr["sha"].(string)
		assert.Equal(t, "def789ghi012", sha)

		sourceBranch, _ := mr["source_branch"].(string)
		assert.Equal(t, "feature-xyz", sourceBranch)
	})

	t.Run("parse gitlab mr json with missing sha", func(t *testing.T) {
		jsonData := `{"source_branch":"feature-xyz"}`

		var mr map[string]any
		err := json.Unmarshal([]byte(jsonData), &mr)
		require.NoError(t, err)

		sha, ok := mr["sha"].(string)
		assert.False(t, ok || sha != "")
	})

	t.Run("handle malformed json", func(t *testing.T) {
		var pr map[string]any
		assert.Error(t, json.Unmarshal([]byte(`{invalid json}`), &pr))
	})
}

func TestCreateWorktreeFromPRIntegration(t *testing.T) {
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh CLI not available, skipping integration test")
	}

	notify := func(_ string, _ string) {}
	notifyOnce := func(_ string, _ string, _ string) {}
	ctx := context.Background()

	t.Run("github host detection triggers github path", func(t *testing.T) {
		service := NewService(notify, notifyOnce)

		repo := t.TempDir()
		runGit(t, repo, "init")
		runGit(t, repo, "config", "user.email", "test@test.com")
		runGit(t, repo, "config", "user.name", "Test User")
		runGit(t, repo, "config", "commit.gpgsign", "false")
		runGit(t, repo, "remote", "add", "origin", "git@github.com:test/repo.git")

		testFile := filepath.Join(repo, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("test"), 0o600))
		runGit(t, repo, "add", ".")
		runGit(t, repo, "commit", "-m", "initial")

		withCwd(t, repo)
		assert.Equal(t, gitHostGithub, service.DetectHost(ctx))

		targetPath := filepath.Join(t.TempDir(), "worktree")
		ok := service.CreateWorktreeFromPR(ctx, 1, "feature", "local", targetPath)
		assert.False(t, ok)
	})

	t.Run("gitlab host detection triggers gitlab path", func(t *testing.T) {
		service := NewService(notify, notifyOnce)

		repo := t.TempDir()
		runGit(t, repo, "init")
		runGit(t, repo, "config", "user.email", "test@test.com")
		runGit(t, repo, "config", "user.name", "Test User")
		runGit(t, repo, "config", "commit.gpgsign", "false")
		runGit(t, repo, "remote", "add", "origin", "git@gitlab.com:test/repo.git")

		testFile := filepath.Join(repo, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("test"), 0o600))
		runGit(t, repo, "add", ".")
		runGit(t, repo, "commit", "-m", "initial")

		withCwd(t, repo)
		assert.Equal(t, gitHostGitLab, service.DetectHost(ctx))

		targetPath := filepath.Join(t.TempDir(), "worktree")
		ok := service.CreateWorktreeFromPR(ctx, 1, "feature", "local", targetPath)
		assert.False(t, ok)
	})
}
