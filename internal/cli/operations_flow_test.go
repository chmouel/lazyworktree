package cli

import (
	"context"
	"errors"
	"os"
	"strings"
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

	if _, err := CreateFromPR(ctx, svc, cfg, 99, false, true); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreateFromPR_ExistingPath(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	fs := &mockFilesystem{
		statFunc: func(string) (os.FileInfo, error) {
			return nil, nil // path exists
		},
		mkdirAllFunc: func(string, os.FileMode) error {
			return errors.New("should not be called")
		},
	}

	svc := &fakeGitService{
		resolveRepoName: "repo",
		prs: []*models.PRInfo{
			{Number: 1, Branch: "b1", Title: "one"},
		},
	}
	cfg := &config.AppConfig{WorktreeDir: "/worktrees", PRBranchNameTemplate: "pr-{number}-{title}"}

	if _, err := CreateFromPRWithFS(ctx, svc, cfg, 1, false, true, fs); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreateFromPR_MkdirFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	fs := &mockFilesystem{
		statFunc: func(string) (os.FileInfo, error) {
			return nil, os.ErrNotExist
		},
		mkdirAllFunc: func(string, os.FileMode) error {
			return errors.New("mkdir failed")
		},
	}

	svc := &fakeGitService{
		resolveRepoName: "repo",
		prs: []*models.PRInfo{
			{Number: 1, Branch: "b1", Title: "one"},
		},
	}
	cfg := &config.AppConfig{WorktreeDir: "/worktrees", PRBranchNameTemplate: "pr-{number}-{title}"}

	if _, err := CreateFromPRWithFS(ctx, svc, cfg, 1, false, true, fs); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreateFromIssue_NotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	svc := &fakeGitService{
		resolveRepoName: "repo",
		issues: []*models.IssueInfo{
			{Number: 1, Title: "fix bug"},
			{Number: 2, Title: "add feature"},
		},
	}

	cfg := &config.AppConfig{WorktreeDir: "/worktrees", IssueBranchNameTemplate: "issue-{number}-{title}"}

	if _, err := CreateFromIssue(ctx, svc, cfg, 99, "main", false, true); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreateFromIssue_ExistingPath(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	fs := &mockFilesystem{
		statFunc: func(string) (os.FileInfo, error) {
			return nil, nil // path exists
		},
		mkdirAllFunc: func(string, os.FileMode) error {
			return errors.New("should not be called")
		},
	}

	svc := &fakeGitService{
		resolveRepoName: "repo",
		issues: []*models.IssueInfo{
			{Number: 1, Title: "fix bug"},
		},
	}
	cfg := &config.AppConfig{WorktreeDir: "/worktrees", IssueBranchNameTemplate: "issue-{number}-{title}"}

	if _, err := CreateFromIssueWithFS(ctx, svc, cfg, 1, "main", false, true, fs); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreateFromIssue_MkdirFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	fs := &mockFilesystem{
		statFunc: func(string) (os.FileInfo, error) {
			return nil, os.ErrNotExist
		},
		mkdirAllFunc: func(string, os.FileMode) error {
			return errors.New("mkdir failed")
		},
	}

	svc := &fakeGitService{
		resolveRepoName: "repo",
		issues: []*models.IssueInfo{
			{Number: 1, Title: "fix bug"},
		},
	}
	cfg := &config.AppConfig{WorktreeDir: "/worktrees", IssueBranchNameTemplate: "issue-{number}-{title}"}

	if _, err := CreateFromIssueWithFS(ctx, svc, cfg, 1, "main", false, true, fs); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreateFromIssue_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	fs := &mockFilesystem{
		statFunc: func(string) (os.FileInfo, error) {
			return nil, os.ErrNotExist
		},
		mkdirAllFunc: func(string, os.FileMode) error {
			return nil
		},
	}

	svc := &fakeGitService{
		resolveRepoName:     "repo",
		runCommandCheckedOK: true,
		mainWorktreePath:    t.TempDir(),
		issues: []*models.IssueInfo{
			{Number: 42, Title: "implement dark mode"},
		},
	}
	cfg := &config.AppConfig{WorktreeDir: "/worktrees", IssueBranchNameTemplate: "issue-{number}-{title}"}

	path, err := CreateFromIssueWithFS(ctx, svc, cfg, 42, "main", false, true, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedBranch := "issue-42-implement-dark-mode"
	if svc.lastWorktreeAddBranch != expectedBranch {
		t.Fatalf("expected branch %q, got %q", expectedBranch, svc.lastWorktreeAddBranch)
	}

	if !strings.HasSuffix(path, expectedBranch) {
		t.Fatalf("expected path to end with %q, got %q", expectedBranch, path)
	}
}

func TestCreateFromIssue_FetchError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	svc := &fakeGitService{
		resolveRepoName: "repo",
		issuesErr:       errors.New("network error"),
	}
	cfg := &config.AppConfig{WorktreeDir: "/worktrees"}

	_, err := CreateFromIssue(ctx, svc, cfg, 1, "main", false, true)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "failed to fetch issue") {
		t.Fatalf("expected fetch error, got: %v", err)
	}
}

func TestCreateFromIssue_CreateWorktreeFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	fs := &mockFilesystem{
		statFunc: func(string) (os.FileInfo, error) {
			return nil, os.ErrNotExist
		},
		mkdirAllFunc: func(string, os.FileMode) error {
			return nil
		},
	}

	svc := &fakeGitService{
		resolveRepoName:     "repo",
		runCommandCheckedOK: false,
		issues: []*models.IssueInfo{
			{Number: 1, Title: "fix bug"},
		},
	}
	cfg := &config.AppConfig{WorktreeDir: "/worktrees", IssueBranchNameTemplate: "issue-{number}-{title}"}

	_, err := CreateFromIssueWithFS(ctx, svc, cfg, 1, "main", false, true, fs)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "failed to create worktree from issue #1") {
		t.Fatalf("expected creation error, got: %v", err)
	}
}

func TestCreateFromIssue_DefaultTemplate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	fs := &mockFilesystem{
		statFunc: func(string) (os.FileInfo, error) {
			return nil, os.ErrNotExist
		},
		mkdirAllFunc: func(string, os.FileMode) error {
			return nil
		},
	}

	svc := &fakeGitService{
		resolveRepoName:     "repo",
		runCommandCheckedOK: true,
		mainWorktreePath:    t.TempDir(),
		issues: []*models.IssueInfo{
			{Number: 10, Title: "add tests"},
		},
	}
	// Empty template — should use default "issue-{number}-{title}"
	cfg := &config.AppConfig{WorktreeDir: "/worktrees"}

	path, err := CreateFromIssueWithFS(ctx, svc, cfg, 10, "develop", false, true, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedBranch := "issue-10-add-tests"
	if svc.lastWorktreeAddBranch != expectedBranch {
		t.Fatalf("expected branch %q, got %q", expectedBranch, svc.lastWorktreeAddBranch)
	}

	if !strings.HasSuffix(path, expectedBranch) {
		t.Fatalf("expected path to end with %q, got %q", expectedBranch, path)
	}
}

func TestCreateFromPR_NoWorkspace_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	svc := &fakeGitService{
		resolveRepoName:    "repo",
		checkoutPRBranchOK: true,
		prs: []*models.PRInfo{
			{Number: 42, Branch: "feature-branch", Title: "add dark mode"},
		},
	}
	cfg := &config.AppConfig{WorktreeDir: "/worktrees", PRBranchNameTemplate: "pr-{number}-{title}"}

	result, err := CreateFromPR(ctx, svc, cfg, 42, true, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedBranch := "pr-42-add-dark-mode"
	if result != expectedBranch {
		t.Fatalf("expected branch name %q, got %q", expectedBranch, result)
	}

	if !svc.checkedOutPRBranch {
		t.Fatal("expected CheckoutPRBranch to be called")
	}
	if svc.lastCheckoutPRBranch != expectedBranch {
		t.Fatalf("expected checkout branch %q, got %q", expectedBranch, svc.lastCheckoutPRBranch)
	}
}

func TestCreateFromPR_NoWorkspace_CheckoutFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	svc := &fakeGitService{
		resolveRepoName:    "repo",
		checkoutPRBranchOK: false,
		prs: []*models.PRInfo{
			{Number: 1, Branch: "b1", Title: "one"},
		},
	}
	cfg := &config.AppConfig{WorktreeDir: "/worktrees", PRBranchNameTemplate: "pr-{number}-{title}"}

	_, err := CreateFromPR(ctx, svc, cfg, 1, true, true)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "failed to checkout branch for PR #1") {
		t.Fatalf("expected checkout error, got: %v", err)
	}
}

func TestCreateFromIssue_NoWorkspace_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	svc := &fakeGitService{
		resolveRepoName:     "repo",
		runCommandCheckedOK: true,
		issues: []*models.IssueInfo{
			{Number: 42, Title: "implement dark mode"},
		},
	}
	cfg := &config.AppConfig{WorktreeDir: "/worktrees", IssueBranchNameTemplate: "issue-{number}-{title}"}

	result, err := CreateFromIssue(ctx, svc, cfg, 42, "main", true, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedBranch := "issue-42-implement-dark-mode"
	if result != expectedBranch {
		t.Fatalf("expected branch name %q, got %q", expectedBranch, result)
	}
}

func TestCreateFromIssue_NoWorkspace_BranchCreateFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	svc := &fakeGitService{
		resolveRepoName:     "repo",
		runCommandCheckedOK: false,
		issues: []*models.IssueInfo{
			{Number: 1, Title: "fix bug"},
		},
	}
	cfg := &config.AppConfig{WorktreeDir: "/worktrees", IssueBranchNameTemplate: "issue-{number}-{title}"}

	_, err := CreateFromIssue(ctx, svc, cfg, 1, "main", true, true)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "failed to create branch from issue #1") {
		t.Fatalf("expected branch creation error, got: %v", err)
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
