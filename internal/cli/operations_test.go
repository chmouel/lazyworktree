package cli

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/chmouel/lazyworktree/internal/models"
)

type fakeGitService struct {
	resolveRepoName     string
	worktrees           []*models.WorktreeInfo
	worktreesErr        error
	runGitOutput        map[string]string
	runCommandCheckedOK bool

	createdFromPR bool
	prs           []*models.PRInfo
	prsErr        error

	mainWorktreePath string
	executedCommands error
}

func (f *fakeGitService) CreateWorktreeFromPR(_ context.Context, _ int, _, _, _ string) bool {
	return f.createdFromPR
}

func (f *fakeGitService) ExecuteCommands(_ context.Context, _ []string, _ string, _ map[string]string) error {
	return f.executedCommands
}

func (f *fakeGitService) FetchAllOpenPRs(_ context.Context) ([]*models.PRInfo, error) {
	return f.prs, f.prsErr
}

func (f *fakeGitService) GetMainWorktreePath(_ context.Context) string {
	return f.mainWorktreePath
}

func (f *fakeGitService) GetWorktrees(_ context.Context) ([]*models.WorktreeInfo, error) {
	return f.worktrees, f.worktreesErr
}

func (f *fakeGitService) ResolveRepoName(_ context.Context) string {
	return f.resolveRepoName
}

func (f *fakeGitService) RunCommandChecked(_ context.Context, _ []string, _, _ string) bool {
	return f.runCommandCheckedOK
}

func (f *fakeGitService) RunGit(_ context.Context, args []string, _ string, _ []int, _, _ bool) string {
	if f.runGitOutput == nil {
		return ""
	}
	return f.runGitOutput[filepath.Join(args...)]
}

func TestFindWorktreeByPathOrName(t *testing.T) {
	t.Parallel()

	worktreeDir := "/worktrees"
	repoName := "repo"

	wtFeature := &models.WorktreeInfo{Path: "/worktrees/repo/feature", Branch: "feature"}
	wtBugfix := &models.WorktreeInfo{Path: "/worktrees/repo/bugfix", Branch: "bugfix"}
	worktrees := []*models.WorktreeInfo{wtFeature, wtBugfix}

	tests := []struct {
		name       string
		pathOrName string
		want       *models.WorktreeInfo
		wantErr    bool
	}{
		{name: "exact path match", pathOrName: wtBugfix.Path, want: wtBugfix},
		{name: "branch match", pathOrName: "feature", want: wtFeature},
		{name: "constructed path match", pathOrName: "bugfix", want: wtBugfix},
		{name: "basename match", pathOrName: filepath.Base(wtFeature.Path), want: wtFeature},
		{name: "not found", pathOrName: "nope", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			found, err := findWorktreeByPathOrName(tt.pathOrName, worktrees, worktreeDir, repoName)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(tt.want, found) {
				t.Fatalf("unexpected worktree: want=%#v got=%#v", tt.want, found)
			}
		})
	}
}

func TestBranchExists(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	tests := []struct {
		name   string
		output string
		want   bool
	}{
		{name: "exists", output: "abcd\n", want: true},
		{name: "missing", output: "\n", want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeGitService{
				runGitOutput: map[string]string{
					filepath.Join("git", "rev-parse", "--verify", "mybranch"): tt.output,
				},
			}
			got := branchExists(ctx, svc, "mybranch")
			if got != tt.want {
				t.Fatalf("unexpected result: want=%v got=%v", tt.want, got)
			}
		})
	}
}

func TestBuildCommandEnv(t *testing.T) {
	t.Parallel()

	env := buildCommandEnv("branch", "/wt/path", "/main/path", "repo")
	want := map[string]string{
		"WORKTREE_BRANCH":    "branch",
		"MAIN_WORKTREE_PATH": "/main/path",
		"WORKTREE_PATH":      "/wt/path",
		"WORKTREE_NAME":      "path",
		"REPO_NAME":          "repo",
	}

	if !reflect.DeepEqual(want, env) {
		t.Fatalf("unexpected env: want=%#v got=%#v", want, env)
	}
}
