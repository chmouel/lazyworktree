package cli

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

func TestCreateFromPR_NotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	svc := &fakeGitService{
		resolveRepoName: "repo",
		prs: []*models.PRInfo{
			{Number: 1, Branch: "b1", Title: "one"},
			{Number: 2, Branch: "b2", Title: "two"},
		},
	}

	cfg := &config.AppConfig{WorktreeDir: "/worktrees", PRBranchNameTemplate: "pr-{number}-{title}"}

	if _, err := CreateFromPR(ctx, svc, cfg, 99, true); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreateFromPR_ExistingPath(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	oldStat := osStat
	oldMkdirAll := osMkdirAll
	t.Cleanup(func() {
		osStat = oldStat
		osMkdirAll = oldMkdirAll
	})

	osStat = func(string) (os.FileInfo, error) {
		return nil, nil
	}
	osMkdirAll = func(string, os.FileMode) error {
		return errors.New("should not be called")
	}

	svc := &fakeGitService{
		resolveRepoName: "repo",
		prs: []*models.PRInfo{
			{Number: 1, Branch: "b1", Title: "one"},
		},
	}
	cfg := &config.AppConfig{WorktreeDir: "/worktrees", PRBranchNameTemplate: "pr-{number}-{title}"}

	if _, err := CreateFromPR(ctx, svc, cfg, 1, true); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreateFromPR_MkdirFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	oldStat := osStat
	oldMkdirAll := osMkdirAll
	t.Cleanup(func() {
		osStat = oldStat
		osMkdirAll = oldMkdirAll
	})

	osStat = func(string) (os.FileInfo, error) {
		return nil, os.ErrNotExist
	}
	osMkdirAll = func(string, os.FileMode) error {
		return errors.New("mkdir failed")
	}

	svc := &fakeGitService{
		resolveRepoName: "repo",
		prs: []*models.PRInfo{
			{Number: 1, Branch: "b1", Title: "one"},
		},
	}
	cfg := &config.AppConfig{WorktreeDir: "/worktrees", PRBranchNameTemplate: "pr-{number}-{title}"}

	if _, err := CreateFromPR(ctx, svc, cfg, 1, true); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDeleteWorktree_ListsWhenNoPath(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	svc := &fakeGitService{
		resolveRepoName: "repo",
		worktrees: []*models.WorktreeInfo{
			{Path: "/main", Branch: "main", IsMain: true},
			{Path: "/wt/one", Branch: "one", Dirty: true},
		},
	}
	cfg := &config.AppConfig{WorktreeDir: "/worktrees"}

	if err := DeleteWorktree(ctx, svc, cfg, "", true, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteWorktree_NoWorktrees(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	svc := &fakeGitService{
		resolveRepoName: "repo",
		worktrees: []*models.WorktreeInfo{
			{Path: "/main", Branch: "main", IsMain: true},
		},
	}
	cfg := &config.AppConfig{WorktreeDir: "/worktrees"}

	if err := DeleteWorktree(ctx, svc, cfg, "/wt/does-not-matter", true, true); err == nil {
		t.Fatalf("expected error")
	}
}
