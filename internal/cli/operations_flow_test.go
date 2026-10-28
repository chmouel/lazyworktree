package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	appservices "github.com/chmouel/lazyworktree/internal/app/services"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	cfg := &config.AppConfig{WorktreeDir: "/worktrees"}

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
	cfg := &config.AppConfig{WorktreeDir: "/worktrees"}

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
	cfg := &config.AppConfig{WorktreeDir: "/worktrees"}

	if _, err := CreateFromPRWithFS(ctx, svc, cfg, 1, false, true, fs); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreateFromPR_SuccessWithWorktreeNoteScript(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	worktreeDir := t.TempDir()

	fs := &mockFilesystem{
		statFunc: func(string) (os.FileInfo, error) {
			return nil, os.ErrNotExist
		},
		mkdirAllFunc: func(string, os.FileMode) error {
			return nil
		},
	}

	svc := &fakeGitService{
		resolveRepoName:  "repo",
		createdFromPR:    true,
		mainWorktreePath: t.TempDir(),
		prs: []*models.PRInfo{
			{Number: 42, Branch: "feature-branch", Title: "Add feature", Body: "Body text", URL: "https://example.com/pr/42"},
		},
	}
	cfg := &config.AppConfig{
		WorktreeDir:        worktreeDir,
		WorktreeNoteScript: `printf 'note-%s' "$LAZYWORKTREE_NUMBER"`,
	}

	path, err := CreateFromPRWithFS(ctx, svc, cfg, 42, false, true, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	notes, err := appservices.LoadWorktreeNotes("repo", worktreeDir, "", "", nil)
	if err != nil {
		t.Fatalf("failed to load notes: %v", err)
	}
	note, ok := notes[path]
	if !ok {
		t.Fatalf("expected note for %q", path)
	}
	if note.Note != "note-42" {
		t.Fatalf("unexpected note content: %q", note.Note)
	}
}

func TestCreateFromPR_SuccessWithWorktreeNoteScriptSharedPath(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	worktreeDir := t.TempDir()
	sharedNotesPath := filepath.Join(t.TempDir(), "shared-worktree-notes.json")

	fs := &mockFilesystem{
		statFunc: func(string) (os.FileInfo, error) {
			return nil, os.ErrNotExist
		},
		mkdirAllFunc: func(string, os.FileMode) error {
			return nil
		},
	}

	svc := &fakeGitService{
		resolveRepoName:  "repo",
		createdFromPR:    true,
		mainWorktreePath: t.TempDir(),
		prs: []*models.PRInfo{
			{Number: 42, Branch: "feature-branch", Title: "Add feature", Body: "Body text", URL: "https://example.com/pr/42"},
		},
	}
	cfg := &config.AppConfig{
		WorktreeDir:        worktreeDir,
		WorktreeNoteScript: `printf 'note-%s' "$LAZYWORKTREE_NUMBER"`,
		WorktreeNotesPath:  sharedNotesPath,
	}

	path, err := CreateFromPRWithFS(ctx, svc, cfg, 42, false, true, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	notes, err := appservices.LoadWorktreeNotes("repo", worktreeDir, sharedNotesPath, "", nil)
	if err != nil {
		t.Fatalf("failed to load notes: %v", err)
	}
	relativeKey := filepath.Base(path)
	note, ok := notes[relativeKey]
	if !ok {
		t.Fatalf("expected note for key %q, notes=%#v", relativeKey, notes)
	}
	if note.Note != "note-42" {
		t.Fatalf("unexpected note content: %q", note.Note)
	}
	if _, ok := notes[path]; ok {
		t.Fatalf("did not expect absolute path key %q in shared notes payload", path)
	}
}

func TestCreateFromPR_WorktreeNoteScriptFailureNonFatal(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	worktreeDir := t.TempDir()

	fs := &mockFilesystem{
		statFunc: func(string) (os.FileInfo, error) {
			return nil, os.ErrNotExist
		},
		mkdirAllFunc: func(string, os.FileMode) error {
			return nil
		},
	}

	svc := &fakeGitService{
		resolveRepoName:  "repo",
		createdFromPR:    true,
		mainWorktreePath: t.TempDir(),
		prs: []*models.PRInfo{
			{Number: 7, Branch: "feature-branch", Title: "Add feature", Body: "Body text", URL: "https://example.com/pr/7"},
		},
	}
	cfg := &config.AppConfig{
		WorktreeDir:        worktreeDir,
		WorktreeNoteScript: "echo boom >&2; exit 1",
	}

	path, err := CreateFromPRWithFS(ctx, svc, cfg, 7, false, true, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := filepath.Base(path); got != "pr-7-add-feature" {
		t.Fatalf("unexpected worktree name: %q", got)
	}

	notes, err := appservices.LoadWorktreeNotes("repo", worktreeDir, "", "", nil)
	if err != nil {
		t.Fatalf("failed to load notes: %v", err)
	}
	if len(notes) != 0 {
		t.Fatalf("expected no notes, got %#v", notes)
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

func TestCreateFromIssue_SuccessWithWorktreeNoteScript(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	worktreeDir := t.TempDir()

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
			{Number: 10, Title: "add tests", Body: "body", URL: "https://example.com/issues/10"},
		},
	}
	cfg := &config.AppConfig{
		WorktreeDir:             worktreeDir,
		IssueBranchNameTemplate: "issue-{number}-{title}",
		WorktreeNoteScript:      `printf 'issue-%s-%s' "$LAZYWORKTREE_NUMBER" "$LAZYWORKTREE_TYPE"`,
	}

	path, err := CreateFromIssueWithFS(ctx, svc, cfg, 10, "develop", false, true, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	notes, err := appservices.LoadWorktreeNotes("repo", worktreeDir, "", "", nil)
	if err != nil {
		t.Fatalf("failed to load notes: %v", err)
	}
	note, ok := notes[path]
	if !ok {
		t.Fatalf("expected note for %q", path)
	}
	if note.Note != "issue-10-issue" {
		t.Fatalf("unexpected note content: %q", note.Note)
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
	cfg := &config.AppConfig{WorktreeDir: "/worktrees"}

	result, err := CreateFromPR(ctx, svc, cfg, 42, true, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedBranch := "feature-branch"
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
	cfg := &config.AppConfig{WorktreeDir: "/worktrees"}

	_, err := CreateFromPR(ctx, svc, cfg, 1, true, true)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "failed to checkout branch for PR #1") {
		t.Fatalf("expected checkout error, got: %v", err)
	}
}

func TestCreateFromPR_BranchAlreadyAttached(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	svc := &fakeGitService{
		resolveRepoName: "repo",
		worktrees: []*models.WorktreeInfo{
			{Path: "/worktrees/repo/feature-branch", Branch: "feature-branch"},
		},
		prs: []*models.PRInfo{
			{Number: 7, Branch: "feature-branch", Title: "already attached"},
		},
	}
	cfg := &config.AppConfig{WorktreeDir: "/worktrees"}

	outputPath, err := CreateFromPR(ctx, svc, cfg, 7, false, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outputPath != "/worktrees/repo/feature-branch" {
		t.Fatalf("expected existing worktree path, got: %q", outputPath)
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

func TestCreateFromIssue_NoWorkspaceSkipsWorktreeNoteScript(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	worktreeDir := t.TempDir()

	svc := &fakeGitService{
		resolveRepoName:     "repo",
		runCommandCheckedOK: true,
		issues: []*models.IssueInfo{
			{Number: 42, Title: "implement dark mode", Body: "body", URL: "https://example.com/issues/42"},
		},
	}
	cfg := &config.AppConfig{
		WorktreeDir:             worktreeDir,
		IssueBranchNameTemplate: "issue-{number}-{title}",
		WorktreeNoteScript:      "cat",
	}

	_, err := CreateFromIssue(ctx, svc, cfg, 42, "main", true, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	notesPath := filepath.Join(worktreeDir, "repo", models.WorktreeNotesFilename)
	if _, err := os.Stat(notesPath); !os.IsNotExist(err) {
		t.Fatalf("expected no notes file, got err=%v", err)
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

func TestCreateFromIssue_ReusesExistingBranch(t *testing.T) {
	t.Parallel()

	const branch = "issue-7-fix-bug"
	showRefKey := filepath.Join("git", "show-ref", "--verify", "refs/heads/"+branch)
	target := func(root string) string { return filepath.Join(root, "repo", branch) }
	elsewhere := func(root string) string { return filepath.Join(root, "elsewhere", branch) }

	tests := []struct {
		name        string
		noWorkspace bool
		// attachedAt returns where the branch is checked out, or nil when it is not.
		attachedAt func(root string) string
		// staleAttached leaves the attached worktree directory missing.
		staleAttached bool
		failOn        string
		wantPath      func(root string) string
		wantCalls     func(root string) [][]string
		wantErr       string
	}{
		{
			name:     "detached branch gets a new worktree without -b",
			wantPath: target,
			wantCalls: func(root string) [][]string {
				return [][]string{{"git", "worktree", "add", target(root), branch}}
			},
		},
		{
			name:       "branch checked out elsewhere returns that worktree",
			attachedAt: elsewhere,
			wantPath:   elsewhere,
		},
		{
			name:       "branch checked out at the target path returns it without -U",
			attachedAt: target,
			wantPath:   target,
		},
		{
			name:          "missing attached worktree asks for a prune",
			attachedAt:    elsewhere,
			staleAttached: true,
			wantErr:       "which no longer exists; run \"git worktree prune\"",
		},
		{
			name:        "no-workspace switches without creating the branch",
			noWorkspace: true,
			wantPath:    func(string) string { return branch },
			wantCalls: func(string) [][]string {
				return [][]string{{"git", "switch", branch}}
			},
		},
		{
			name:        "no-workspace names the worktree holding the branch",
			noWorkspace: true,
			attachedAt:  elsewhere,
			failOn:      "switch",
			wantCalls: func(string) [][]string {
				return [][]string{{"git", "switch", branch}}
			},
			wantErr: `branch "issue-7-fix-bug" is already checked out in worktree`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			root := t.TempDir()

			svc := &fakeGitService{
				resolveRepoName:     "repo",
				mainWorktreePath:    filepath.Join(root, "main"),
				runCommandCheckedOK: true,
				checkedFailOn:       tt.failOn,
				runGitOutput:        map[string]string{showRefKey: "abc123 refs/heads/" + branch + "\n"},
				issues:              []*models.IssueInfo{{Number: 7, Title: "fix bug"}},
			}
			if tt.attachedAt != nil {
				attached := tt.attachedAt(root)
				svc.worktrees = []*models.WorktreeInfo{{Path: attached, Branch: branch}}
				if !tt.staleAttached {
					require.NoError(t, os.MkdirAll(attached, 0o750))
				}
			}
			cfg := &config.AppConfig{WorktreeDir: root}

			got, err := CreateFromIssueWithFS(ctx, svc, cfg, 7, "main", tt.noWorkspace, true, DefaultFS)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantPath(root), got)
			}
			var wantCalls [][]string
			if tt.wantCalls != nil {
				wantCalls = tt.wantCalls(root)
			}
			assert.Equal(t, wantCalls, svc.runCommandCheckedCalls)
		})
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

func TestRenameWorktree(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := &config.AppConfig{WorktreeDir: "/worktrees"}

	t.Run("renames selected worktree", func(t *testing.T) {
		svc := &fakeGitService{
			resolveRepoName:  "repo",
			renameWorktreeOK: true,
			worktrees: []*models.WorktreeInfo{
				{Path: "/main", Branch: "main", IsMain: true},
				{Path: "/worktrees/repo/feature", Branch: "feature"},
			},
		}

		err := RenameWorktree(ctx, svc, cfg, "feature", "new-feature", true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !svc.renameWorktreeCalled {
			t.Fatalf("expected rename to be called")
		}
		if svc.lastRenameOldPath != "/worktrees/repo/feature" {
			t.Fatalf("unexpected old path: %q", svc.lastRenameOldPath)
		}
		if svc.lastRenameNewPath != "/worktrees/repo/new-feature" {
			t.Fatalf("unexpected new path: %q", svc.lastRenameNewPath)
		}
		if svc.lastRenameOldBranch != "feature" {
			t.Fatalf("unexpected old branch: %q", svc.lastRenameOldBranch)
		}
		if svc.lastRenameNewBranch != "new-feature" {
			t.Fatalf("unexpected new branch: %q", svc.lastRenameNewBranch)
		}
	})

	t.Run("renames worktree by full path (simulating cwd resolution)", func(t *testing.T) {
		svc := &fakeGitService{
			resolveRepoName:  "repo",
			renameWorktreeOK: true,
			worktrees: []*models.WorktreeInfo{
				{Path: "/main", Branch: "main", IsMain: true},
				{Path: "/worktrees/repo/feature", Branch: "feature"},
			},
		}

		err := RenameWorktree(ctx, svc, cfg, "/worktrees/repo/feature", "new-feature", true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !svc.renameWorktreeCalled {
			t.Fatalf("expected rename to be called")
		}
		if svc.lastRenameOldPath != "/worktrees/repo/feature" {
			t.Fatalf("unexpected old path: %q", svc.lastRenameOldPath)
		}
		if svc.lastRenameNewPath != "/worktrees/repo/new-feature" {
			t.Fatalf("unexpected new path: %q", svc.lastRenameNewPath)
		}
	})

	t.Run("rejects empty new name", func(t *testing.T) {
		svc := &fakeGitService{
			resolveRepoName: "repo",
			worktrees: []*models.WorktreeInfo{
				{Path: "/main", Branch: "main", IsMain: true},
				{Path: "/worktrees/repo/feature", Branch: "feature"},
			},
		}

		err := RenameWorktree(ctx, svc, cfg, "feature", "   ", true)
		if err == nil {
			t.Fatalf("expected error")
		}
		if !strings.Contains(err.Error(), "new name is required") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("fails when git rename fails", func(t *testing.T) {
		svc := &fakeGitService{
			resolveRepoName:  "repo",
			renameWorktreeOK: false,
			worktrees: []*models.WorktreeInfo{
				{Path: "/main", Branch: "main", IsMain: true},
				{Path: "/worktrees/repo/feature", Branch: "feature"},
			},
		}

		err := RenameWorktree(ctx, svc, cfg, "feature", "new-feature", true)
		if err == nil {
			t.Fatalf("expected error")
		}
		if !strings.Contains(err.Error(), "failed to rename worktree") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
