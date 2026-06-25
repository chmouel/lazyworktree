package services

import (
	"strings"
	"testing"
)

func TestBuildCommandEnvWithContext(t *testing.T) {
	t.Parallel()

	env := BuildCommandEnvWithContext("feature/demo", "/tmp/repo/demo", "owner/repo", "/tmp/repo", LazyWorktreeContext{
		Type:          "pr",
		Number:        "123",
		Template:      "pr-{number}-{title}",
		SuggestedName: "pr-123-demo",
		Title:         "Demo PR",
		URL:           "https://example.com/pull/123",
		Description:   "Demo body",
	})

	want := map[string]string{
		EnvWorktreeBranch:            "feature/demo",
		EnvMainWorktreePath:          "/tmp/repo",
		EnvWorktreePath:              "/tmp/repo/demo",
		EnvWorktreeName:              "demo",
		EnvRepoName:                  "owner/repo",
		EnvRepoOwner:                 "owner",
		EnvRepoRepoName:              "repo",
		EnvLazyWorktreeType:          "pr",
		EnvLazyWorktreeNumber:        "123",
		EnvLazyWorktreeTemplate:      "pr-{number}-{title}",
		EnvLazyWorktreeSuggestedName: "pr-123-demo",
		EnvLazyWorktreeTitle:         "Demo PR",
		EnvLazyWorktreeURL:           "https://example.com/pull/123",
		EnvLazyWorktreeDescription:   "Demo body",
	}
	for key, wantVal := range want {
		if got := env[key]; got != wantVal {
			t.Fatalf("env[%s] = %q, want %q", key, got, wantVal)
		}
	}
}

func TestBuildCommandEnvWithEmptyContext(t *testing.T) {
	t.Parallel()

	env := BuildCommandEnv("", "", "repo", "/main")
	if got := env[EnvWorktreeName]; got != "" {
		t.Fatalf("expected empty worktree name for empty path, got %q", got)
	}
	if got := env[EnvLazyWorktreeNumber]; got != "" {
		t.Fatalf("expected empty lazyworktree number, got %q", got)
	}
	if got := env[EnvRepoRepoName]; got != "repo" {
		t.Fatalf("expected repo component, got %q", got)
	}
}

func TestAppendCommandEnvFiltersManagedValues(t *testing.T) {
	t.Parallel()

	parent := []string{
		"PATH=/usr/bin",
		"WORKTREE_PATH=/stale",
		"LAZYWORKTREE_NUMBER=999",
		"REPO_OWNER=stale",
		"HOME=/home/user",
	}
	env := BuildCommandEnvWithContext("branch", "/wt/path", "owner/repo", "/main", LazyWorktreeContext{Number: "42"})

	got := AppendCommandEnv(parent, env)
	values := map[string]string{}
	for _, entry := range got {
		key, val, ok := strings.Cut(entry, "=")
		if ok {
			values[key] = val
		}
	}

	if values["PATH"] != "/usr/bin" || values["HOME"] != "/home/user" {
		t.Fatalf("expected unmanaged parent env to be preserved, got %v", got)
	}
	if values[EnvWorktreePath] != "/wt/path" {
		t.Fatalf("expected generated worktree path, got %q", values[EnvWorktreePath])
	}
	if values[EnvLazyWorktreeNumber] != "42" {
		t.Fatalf("expected generated lazyworktree number, got %q", values[EnvLazyWorktreeNumber])
	}
	if values[EnvRepoOwner] != "owner" {
		t.Fatalf("expected generated repo owner, got %q", values[EnvRepoOwner])
	}
}
