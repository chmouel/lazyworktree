package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appservices "github.com/chmouel/lazyworktree/internal/app/services"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/security"
)

const testRepoName = "testRepoName"

type fakeGitService struct {
	resolveRepoName     string
	worktrees           []*models.WorktreeInfo
	worktreesErr        error
	runGitOutput        map[string]string
	runCommandCheckedOK bool
	renameWorktreeOK    bool
	authUsername        string

	checkedOutPRBranch   bool
	lastCheckoutPRBranch string
	checkoutPRBranchOK   bool
	createdFromPR        bool
	lastPRRemoteBranch   string
	lastPRLocalBranch    string
	lastPRTargetPath     string
	prs                  []*models.PRInfo
	prsErr               error
	fetchPRCalls         int
	prForWorktree        *models.PRInfo
	prForWorktreeErr     error
	prForWorktreeCalls   int

	issues    []*models.IssueInfo
	issuesErr error

	currentBranch    string
	currentBranchErr error
	mainBranch       string
	mergedBranches   []string

	mainWorktreePath       string
	executedCommands       error
	lastExecutedEnv        map[string]string
	lastExecutedCwd        string
	lastExecutedCommands   []string
	lastWorktreeAddPath    string
	lastWorktreeAddBranch  string
	renameWorktreeCalled   bool
	lastRenameOldPath      string
	lastRenameNewPath      string
	lastRenameOldBranch    string
	lastRenameNewBranch    string
	runCommandCheckedCalls [][]string
	runCommandQuietOK      bool
	runCommandQuietCalls   [][]string
	runGitCombinedOutputs  map[string][]byte
	runGitCombinedErrors   map[string]error
	gitArgOutputs          map[string]string
	gitArgErrors           map[string]error
	checkedFailOn          string
}

func (f *fakeGitService) CheckoutPRBranch(_ context.Context, _ int, _, localBranch string) bool {
	f.checkedOutPRBranch = true
	f.lastCheckoutPRBranch = localBranch
	return f.checkoutPRBranchOK
}

func (f *fakeGitService) CreateWorktreeFromPR(_ context.Context, _ int, remoteBranch, localBranch, targetPath string) bool {
	f.lastPRRemoteBranch = remoteBranch
	f.lastPRLocalBranch = localBranch
	f.lastPRTargetPath = targetPath
	return f.createdFromPR
}

func (f *fakeGitService) ExecuteCommands(_ context.Context, commands []string, cwd string, env map[string]string) error {
	f.lastExecutedCommands = append([]string{}, commands...)
	f.lastExecutedCwd = cwd
	f.lastExecutedEnv = env
	return f.executedCommands
}

func (f *fakeGitService) FetchAllOpenIssues(_ context.Context) ([]*models.IssueInfo, error) {
	return f.issues, f.issuesErr
}

func (f *fakeGitService) FetchAllOpenPRs(_ context.Context) ([]*models.PRInfo, error) {
	return f.prs, f.prsErr
}

func (f *fakeGitService) FetchIssue(_ context.Context, issueNumber int) (*models.IssueInfo, error) {
	for _, issue := range f.issues {
		if issue.Number == issueNumber {
			return issue, nil
		}
	}
	if f.issuesErr != nil {
		return nil, f.issuesErr
	}
	return nil, fmt.Errorf("issue #%d not found", issueNumber)
}

func (f *fakeGitService) FetchPR(_ context.Context, prNumber int) (*models.PRInfo, error) {
	f.fetchPRCalls++
	for _, pr := range f.prs {
		if pr.Number == prNumber {
			return pr, nil
		}
	}
	if f.prsErr != nil {
		return nil, f.prsErr
	}
	return nil, fmt.Errorf("PR #%d not found", prNumber)
}

func (f *fakeGitService) FetchPRForWorktreeWithError(_ context.Context, _ string) (*models.PRInfo, error) {
	f.prForWorktreeCalls++
	return f.prForWorktree, f.prForWorktreeErr
}

func (f *fakeGitService) GetCurrentBranch(_ context.Context) (string, error) {
	return f.currentBranch, f.currentBranchErr
}

func (f *fakeGitService) GetAuthenticatedUsername(_ context.Context) string {
	return f.authUsername
}

func (f *fakeGitService) GetMainWorktreePath(_ context.Context) string {
	return f.mainWorktreePath
}

func (f *fakeGitService) GetMainBranch(_ context.Context) string {
	return f.mainBranch
}

func (f *fakeGitService) GetMergedBranches(_ context.Context, _ string) []string {
	return f.mergedBranches
}

func (f *fakeGitService) GetWorktrees(_ context.Context) ([]*models.WorktreeInfo, error) {
	return f.worktrees, f.worktreesErr
}

func (f *fakeGitService) RenameWorktree(_ context.Context, oldPath, newPath, oldBranch, newBranch string) bool {
	f.renameWorktreeCalled = true
	f.lastRenameOldPath = oldPath
	f.lastRenameNewPath = newPath
	f.lastRenameOldBranch = oldBranch
	f.lastRenameNewBranch = newBranch
	return f.renameWorktreeOK
}

func (f *fakeGitService) ResolveRepoName(_ context.Context) string {
	return f.resolveRepoName
}

func (f *fakeGitService) RunCommandChecked(_ context.Context, args []string, _, _ string) bool {
	f.runCommandCheckedCalls = append(f.runCommandCheckedCalls, slices.Clone(args))
	if f.checkedFailOn != "" && strings.Contains(strings.Join(args, " "), f.checkedFailOn) {
		return false
	}
	// Capture worktree add commands for testing
	if len(args) > 2 && args[0] == "git" && args[1] == "worktree" && args[2] == "add" {
		// Find the path in the args (it's before the branch name)
		for i := 3; i < len(args); i++ {
			if args[i] == "-b" && i+2 < len(args) {
				f.lastWorktreeAddBranch = args[i+1]
				f.lastWorktreeAddPath = args[i+2]
				break
			} else if !strings.HasPrefix(args[i], "-") {
				// First non-flag argument after "add" is the path
				f.lastWorktreeAddPath = args[i]
				break
			}
		}
	}
	return f.runCommandCheckedOK
}

func (f *fakeGitService) RunCommandQuiet(_ context.Context, args []string, _ string) bool {
	f.runCommandQuietCalls = append(f.runCommandQuietCalls, slices.Clone(args))
	return f.runCommandQuietOK
}

func (f *fakeGitService) RunGit(_ context.Context, args []string, _ string, _ []int, _, _ bool) string {
	if f.runGitOutput == nil {
		return ""
	}
	return f.runGitOutput[filepath.Join(args...)]
}

func (f *fakeGitService) RunGitWithCombinedOutput(_ context.Context, args []string, cwd string, _ map[string]string) ([]byte, error) {
	key := filepath.Join(args...)
	if err, ok := f.gitArgErrors[key]; ok {
		return nil, err
	}
	if out, ok := f.gitArgOutputs[key]; ok {
		return []byte(out), nil
	}
	if f.runGitCombinedErrors != nil {
		if err, ok := f.runGitCombinedErrors[cwd]; ok {
			return nil, err
		}
	}
	if f.runGitCombinedOutputs != nil {
		return f.runGitCombinedOutputs[cwd], nil
	}
	return nil, nil
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// mockFilesystem implements OSFilesystem for testing.
type mockFilesystem struct {
	statFunc     func(name string) (os.FileInfo, error)
	mkdirAllFunc func(path string, perm os.FileMode) error
	getwdFunc    func() (string, error)
}

func (m *mockFilesystem) Stat(name string) (os.FileInfo, error) {
	if m.statFunc != nil {
		return m.statFunc(name)
	}
	return os.Stat(name)
}

func (m *mockFilesystem) MkdirAll(path string, perm os.FileMode) error {
	if m.mkdirAllFunc != nil {
		return m.mkdirAllFunc(path, perm)
	}
	return os.MkdirAll(path, perm)
}

func (m *mockFilesystem) Getwd() (string, error) {
	if m.getwdFunc != nil {
		return m.getwdFunc()
	}
	return os.Getwd()
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
		{name: "prefix path match (cwd inside worktree)", pathOrName: "/worktrees/repo/feature/src/pkg", want: wtFeature},
		{name: "not found", pathOrName: "nope", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			found, err := FindWorktreeByPathOrName(tt.pathOrName, worktrees, worktreeDir, repoName, "")
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

func TestResolveWorktreeBaseDir(t *testing.T) {
	t.Parallel()

	globalWorktreeDir := filepath.Join(string(filepath.Separator), "home", "user", ".local", "share", "worktrees")
	mainWorktreePath := filepath.Join(string(filepath.Separator), "home", "user", "code", "myrepo")
	repoLocalWorktreeDir := filepath.Join(mainWorktreePath, ".worktrees")
	worktreesRootDir := filepath.Join(string(filepath.Separator), "worktrees")
	similarPrefixWorktreeDir := filepath.Join(string(filepath.Separator), "home", "user", "code", "myrepo-other", ".worktrees")

	tests := []struct {
		name             string
		worktreeDir      string
		mainWorktreePath string
		repoName         string
		want             string
	}{
		{
			name:             "global mode: absolute dir outside repo",
			worktreeDir:      globalWorktreeDir,
			mainWorktreePath: mainWorktreePath,
			repoName:         "org-myrepo",
			want:             filepath.Join(globalWorktreeDir, "org-myrepo"),
		},
		{
			name:             "repo-local mode: dir inside main worktree",
			worktreeDir:      repoLocalWorktreeDir,
			mainWorktreePath: mainWorktreePath,
			repoName:         "org-myrepo",
			want:             repoLocalWorktreeDir,
		},
		{
			name:             "repo-local mode: dir equals main worktree path",
			worktreeDir:      mainWorktreePath,
			mainWorktreePath: mainWorktreePath,
			repoName:         "org-myrepo",
			want:             mainWorktreePath,
		},
		{
			name:             "empty mainWorktreePath falls back to global",
			worktreeDir:      worktreesRootDir,
			mainWorktreePath: "",
			repoName:         "myrepo",
			want:             filepath.Join(worktreesRootDir, "myrepo"),
		},
		{
			name:             "similar prefix does not trigger repo-local",
			worktreeDir:      similarPrefixWorktreeDir,
			mainWorktreePath: mainWorktreePath,
			repoName:         "org-myrepo",
			want:             filepath.Join(similarPrefixWorktreeDir, "org-myrepo"),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := resolveWorktreeBaseDir(tt.worktreeDir, tt.mainWorktreePath, tt.repoName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsRepoLocal(t *testing.T) {
	t.Parallel()

	mainWorktreePath := filepath.Join(string(filepath.Separator), "home", "user", "code", "myrepo")

	tests := []struct {
		name             string
		worktreeDir      string
		mainWorktreePath string
		want             bool
	}{
		{
			name:             "inside main worktree",
			worktreeDir:      filepath.Join(mainWorktreePath, ".worktrees"),
			mainWorktreePath: mainWorktreePath,
			want:             true,
		},
		{
			name:             "equals main worktree path",
			worktreeDir:      mainWorktreePath,
			mainWorktreePath: mainWorktreePath,
			want:             true,
		},
		{
			name:             "outside main worktree",
			worktreeDir:      filepath.Join(string(filepath.Separator), "home", "user", ".local", "share", "worktrees"),
			mainWorktreePath: mainWorktreePath,
			want:             false,
		},
		{
			name:             "similar prefix is not repo-local",
			worktreeDir:      filepath.Join(string(filepath.Separator), "home", "user", "code", "myrepo-other", ".worktrees"),
			mainWorktreePath: mainWorktreePath,
			want:             false,
		},
		{
			name:             "empty mainWorktreePath",
			worktreeDir:      filepath.Join(string(filepath.Separator), "worktrees"),
			mainWorktreePath: "",
			want:             false,
		},
		{
			name:             "empty worktreeDir",
			worktreeDir:      "",
			mainWorktreePath: mainWorktreePath,
			want:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := IsRepoLocal(tt.worktreeDir, tt.mainWorktreePath)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsRepoLocalWithRelativeRepoLocalDir(t *testing.T) {
	tmpDir := t.TempDir()
	mainWorktreePath := filepath.Join(tmpDir, "repo")
	require.NoError(t, os.MkdirAll(mainWorktreePath, 0o750))
	t.Chdir(mainWorktreePath)

	assert.True(t, IsRepoLocal(".worktrees", mainWorktreePath))
	assert.Equal(t, ".worktrees", resolveWorktreeBaseDir(".worktrees", mainWorktreePath, "org-myrepo"))
}

func TestIsRepoLocalWithRelativeDirOutsideRepo(t *testing.T) {
	tmpDir := t.TempDir()
	parentDir := filepath.Join(tmpDir, "parent")
	mainWorktreePath := filepath.Join(parentDir, "repo")
	require.NoError(t, os.MkdirAll(mainWorktreePath, 0o750))
	t.Chdir(mainWorktreePath)

	assert.False(t, IsRepoLocal("../shared-worktrees", mainWorktreePath))
}

func TestIsRepoLocalWithRelativeDirWhenWorkingDirUnavailable(t *testing.T) {
	tmpDir := t.TempDir()
	mainWorktreePath := filepath.Join(tmpDir, "repo")
	require.NoError(t, os.MkdirAll(mainWorktreePath, 0o750))

	missingDir := filepath.Join(tmpDir, "deleted-cwd")
	require.NoError(t, os.MkdirAll(missingDir, 0o750))
	t.Chdir(missingDir)
	require.NoError(t, os.RemoveAll(missingDir))

	assert.False(t, IsRepoLocal(".worktrees", mainWorktreePath))
}

func TestFindWorktreeByPathOrNameRepoLocal(t *testing.T) {
	t.Parallel()

	mainPath := filepath.Join(string(filepath.Separator), "home", "user", "code", "myrepo")
	worktreeDir := filepath.Join(mainPath, ".worktrees")
	repoName := "org-myrepo"

	wtFeature := &models.WorktreeInfo{Path: filepath.Join(worktreeDir, "feature"), Branch: "feature"}
	worktrees := []*models.WorktreeInfo{wtFeature}

	found, err := FindWorktreeByPathOrName("feature", worktrees, worktreeDir, repoName, mainPath)
	require.NoError(t, err)
	assert.Equal(t, wtFeature, found)
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
		"WORKTREE_BRANCH":             "branch",
		"MAIN_WORKTREE_PATH":          "/main/path",
		"WORKTREE_PATH":               "/wt/path",
		"WORKTREE_NAME":               "path",
		"REPO_NAME":                   "repo",
		"REPO_OWNER":                  "",
		"REPO_REPONAME":               "repo",
		"LAZYWORKTREE_TYPE":           "",
		"LAZYWORKTREE_NUMBER":         "",
		"LAZYWORKTREE_TEMPLATE":       "",
		"LAZYWORKTREE_SUGGESTED_NAME": "",
		"LAZYWORKTREE_TITLE":          "",
		"LAZYWORKTREE_URL":            "",
		"LAZYWORKTREE_DESCRIPTION":    "",
	}

	if !reflect.DeepEqual(want, env) {
		t.Fatalf("unexpected env: want=%#v got=%#v", want, env)
	}
}

func TestCreateFromPRWithFSDisabledPRDoesNotFetch(t *testing.T) {
	t.Parallel()

	svc := &fakeGitService{}
	cfg := &config.AppConfig{DisablePR: true, WorktreeDir: t.TempDir()}

	_, err := CreateFromPRWithFS(context.Background(), svc, cfg, 42, false, true, DefaultFS)
	if err == nil {
		t.Fatal("expected error when PR integration is disabled")
	}
	if !strings.Contains(err.Error(), "PR/MR integration is disabled") {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.fetchPRCalls != 0 {
		t.Fatalf("expected no PR fetch when disabled, got %d", svc.fetchPRCalls)
	}
}

func updateFakeOutputs(targetPath, status, counts string) map[string]string {
	remoteFeatureCommitRef := "refs/remotes/origin/feature^{commit}"
	return map[string]string{
		filepath.Join("git", "rev-parse", "--verify", "feature"):                         "abc123\n",
		filepath.Join("git", "rev-parse", "--show-toplevel"):                             targetPath + "\n",
		filepath.Join("git", "rev-parse", "--path-format=absolute", "--git-common-dir"):  "/repo/.git\n",
		filepath.Join("git", "status", "--porcelain", "--untracked-files=all"):           status,
		filepath.Join("git", "rev-parse", "--verify", "--quiet", remoteFeatureCommitRef): "abc123\n",
		filepath.Join("git", "rev-parse", "--verify", "--quiet", "FETCH_HEAD^{commit}"):  "def456\n",
		filepath.Join("git", "rev-list", "--left-right", "--count", "HEAD...abc123"):     counts,
		filepath.Join("git", "rev-list", "--left-right", "--count", "HEAD...def456"):     counts,
	}
}

func countCalls(calls [][]string, substr string) int {
	n := 0
	for _, call := range calls {
		if strings.Contains(strings.Join(call, " "), substr) {
			n++
		}
	}
	return n
}

func TestUpdateOnExistingBranch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	remoteFeatureCommitRef := "refs/remotes/origin/feature^{commit}"
	statusKey := filepath.Join("git", "status", "--porcelain", "--untracked-files=all")
	revListKey := filepath.Join("git", "rev-list", "--left-right", "--count", "HEAD...abc123")
	resolveKey := filepath.Join("git", "rev-parse", "--verify", "--quiet", remoteFeatureCommitRef)
	topKey := filepath.Join("git", "rev-parse", "--show-toplevel")

	tests := []struct {
		name        string
		status      string
		counts      string
		mutate      func(*fakeGitService)
		checkedOK   bool
		checkedFail string
		wantFetch   bool
		wantMerge   bool
		wantErr     string
	}{
		{name: "up to date is a no-op", counts: "0 0\n", checkedOK: true, wantFetch: true},
		{name: "behind only fast-forwards", counts: "0 2\n", checkedOK: true, wantFetch: true, wantMerge: true},
		{name: "ahead only is left untouched", counts: "1 0\n", checkedOK: true, wantFetch: true},
		{name: "diverged is left untouched", counts: "1 2\n", checkedOK: true, wantFetch: true},
		{name: "dirty is left untouched without fetching", status: " M dirty.go\n", checkedOK: true},
		{name: "untracked is left untouched without fetching", status: "?? new.go\n", checkedOK: true},
		{
			name:      "status failure errors without fetching",
			checkedOK: true,
			mutate: func(f *fakeGitService) {
				f.gitArgErrors = map[string]error{statusKey: errors.New("boom")}
			},
			wantErr: "failed to inspect worktree",
		},
		{name: "fetch failure errors", counts: "0 1\n", checkedOK: true, checkedFail: "fetch", wantFetch: true, wantErr: "failed to fetch feature from origin"},
		{
			name:      "unresolvable target errors",
			counts:    "0 1\n",
			checkedOK: true,
			wantFetch: true,
			mutate: func(f *fakeGitService) {
				delete(f.gitArgOutputs, resolveKey)
			},
			wantErr: "failed to resolve",
		},
		{
			name:      "rev-list failure errors",
			checkedOK: true,
			wantFetch: true,
			mutate: func(f *fakeGitService) {
				f.gitArgErrors = map[string]error{revListKey: errors.New("boom")}
			},
			wantErr: "failed to compare worktree",
		},
		{name: "malformed rev-list output errors", counts: "garbage\n", checkedOK: true, wantFetch: true, wantErr: "failed to compare worktree"},
		{name: "merge failure errors", counts: "0 1\n", checkedOK: true, checkedFail: "merge", wantFetch: true, wantMerge: true, wantErr: "failed to fast-forward"},
		{
			name:      "non-root directory errors before fetching",
			counts:    "0 1\n",
			checkedOK: true,
			mutate: func(f *fakeGitService) {
				f.gitArgOutputs[topKey] = "/elsewhere\n"
			},
			wantErr: "is not the root of a git worktree",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()
			repoName := testRepoName
			worktreeName := "feature-wt"
			targetPath := filepath.Join(tmpDir, repoName, worktreeName)
			require.NoError(t, os.MkdirAll(targetPath, 0o750))

			svc := &fakeGitService{
				resolveRepoName:     repoName,
				runGitOutput:        map[string]string{filepath.Join("git", "rev-parse", "--verify", "feature"): "abc123\n"},
				runCommandCheckedOK: tt.checkedOK,
				checkedFailOn:       tt.checkedFail,
				gitArgOutputs:       updateFakeOutputs(targetPath, tt.status, tt.counts),
			}
			if tt.mutate != nil {
				tt.mutate(svc)
			}

			cfg := &config.AppConfig{WorktreeDir: tmpDir, UpdateOnExisting: true}
			outputPath, err := CreateFromBranch(ctx, svc, cfg, "feature", worktreeName, false, true)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, targetPath, outputPath)
			}
			assert.Equal(t, tt.wantFetch, countCalls(svc.runCommandCheckedCalls, "fetch") > 0, "fetch called")
			assert.Equal(t, tt.wantMerge, countCalls(svc.runCommandCheckedCalls, "merge") > 0, "merge called")
			if tt.wantMerge {
				assert.Contains(t, svc.runCommandCheckedCalls[len(svc.runCommandCheckedCalls)-1], "abc123")
			}
		})
	}
}

func TestUpdateOnExistingPR(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	statusKey := filepath.Join("git", "status", "--porcelain", "--untracked-files=all")

	newPRSvc := func(tmpDir, targetPath, prBranch, status, counts string) *fakeGitService {
		return &fakeGitService{
			resolveRepoName:     testRepoName,
			mainWorktreePath:    filepath.Join(tmpDir, "main"),
			runCommandCheckedOK: true,
			runCommandQuietOK:   true,
			prs: []*models.PRInfo{
				{Number: 42, Title: "Fix bug", Branch: prBranch},
			},
			worktrees: []*models.WorktreeInfo{
				{Path: targetPath, Branch: prBranch},
			},
			gitArgOutputs: updateFakeOutputs(targetPath, status, counts),
		}
	}

	t.Run("behind only fast-forwards to PR head", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		targetPath := filepath.Join(tmpDir, testRepoName, "pr-42-fix-bug")
		require.NoError(t, os.MkdirAll(targetPath, 0o750))

		svc := newPRSvc(tmpDir, targetPath, "fix-bug", "", "0 1\n")
		cfg := &config.AppConfig{WorktreeDir: tmpDir, UpdateOnExisting: true}

		outputPath, err := CreateFromPRWithFS(ctx, svc, cfg, 42, false, true, DefaultFS)
		require.NoError(t, err)
		assert.Equal(t, targetPath, outputPath)
		require.NotEmpty(t, svc.runCommandQuietCalls)
		assert.Equal(t, []string{"git", "fetch", "origin", "refs/pull/42/head"}, svc.runCommandQuietCalls[0])
		assert.Equal(t, 1, countCalls(svc.runCommandCheckedCalls, "merge"))
	})

	t.Run("dirty worktree is left untouched without fetching", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		targetPath := filepath.Join(tmpDir, testRepoName, "pr-42-fix-bug")
		require.NoError(t, os.MkdirAll(targetPath, 0o750))

		svc := newPRSvc(tmpDir, targetPath, "fix-bug", " M dirty.go\n", "0 1\n")
		cfg := &config.AppConfig{WorktreeDir: tmpDir, UpdateOnExisting: true}

		outputPath, err := CreateFromPRWithFS(ctx, svc, cfg, 42, false, true, DefaultFS)
		require.NoError(t, err)
		assert.Equal(t, targetPath, outputPath)
		assert.Empty(t, svc.runCommandQuietCalls)
		assert.Empty(t, svc.runCommandCheckedCalls)
	})

	t.Run("ahead worktree is left untouched", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		targetPath := filepath.Join(tmpDir, testRepoName, "pr-42-fix-bug")
		require.NoError(t, os.MkdirAll(targetPath, 0o750))

		svc := newPRSvc(tmpDir, targetPath, "fix-bug", "", "2 0\n")
		cfg := &config.AppConfig{WorktreeDir: tmpDir, UpdateOnExisting: true}

		outputPath, err := CreateFromPRWithFS(ctx, svc, cfg, 42, false, true, DefaultFS)
		require.NoError(t, err)
		assert.Equal(t, targetPath, outputPath)
		assert.Equal(t, 0, countCalls(svc.runCommandCheckedCalls, "merge"))
	})

	t.Run("status failure errors", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		targetPath := filepath.Join(tmpDir, testRepoName, "pr-42-fix-bug")
		require.NoError(t, os.MkdirAll(targetPath, 0o750))

		svc := newPRSvc(tmpDir, targetPath, "fix-bug", "", "0 1\n")
		svc.gitArgErrors = map[string]error{statusKey: errors.New("boom")}
		cfg := &config.AppConfig{WorktreeDir: tmpDir, UpdateOnExisting: true}

		_, err := CreateFromPRWithFS(ctx, svc, cfg, 42, false, true, DefaultFS)
		require.ErrorContains(t, err, "failed to inspect worktree")
		assert.Empty(t, svc.runCommandQuietCalls)
	})

	t.Run("fork PR falls back to branch fetch when refs/pull is unavailable", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		targetPath := filepath.Join(tmpDir, testRepoName, "pr-42-fix-bug")
		require.NoError(t, os.MkdirAll(targetPath, 0o750))

		svc := newPRSvc(tmpDir, targetPath, "fork-branch", "", "0 1\n")
		svc.runCommandQuietOK = false
		cfg := &config.AppConfig{WorktreeDir: tmpDir, UpdateOnExisting: true}

		outputPath, err := CreateFromPRWithFS(ctx, svc, cfg, 42, false, true, DefaultFS)
		require.NoError(t, err)
		assert.Equal(t, targetPath, outputPath)
		assert.Equal(t, 1, countCalls(svc.runCommandCheckedCalls, "fork-branch"))
		assert.Equal(t, 1, countCalls(svc.runCommandCheckedCalls, "merge"))
	})
}

func TestUpdateOnExistingIssue(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("returns existing path without fetch", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoName := testRepoName
		branchName := "issue-10-add-feature"
		targetPath := filepath.Join(tmpDir, repoName, branchName)
		require.NoError(t, os.MkdirAll(targetPath, 0o750))

		svc := &fakeGitService{
			resolveRepoName:     repoName,
			mainWorktreePath:    filepath.Join(tmpDir, "main"),
			runCommandCheckedOK: true,
			issues: []*models.IssueInfo{
				{Number: 10, Title: "Add feature"},
			},
		}

		cfg := &config.AppConfig{
			WorktreeDir:      tmpDir,
			UpdateOnExisting: true,
		}

		outputPath, err := CreateFromIssueWithFS(ctx, svc, cfg, 10, "main", false, true, DefaultFS)
		require.NoError(t, err)
		assert.Equal(t, targetPath, outputPath)
	})
}

func TestCheckTrust(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tmpDir := t.TempDir()
	wtFile := filepath.Join(tmpDir, ".wt")

	tests := []struct {
		name        string
		trustMode   string
		trustStatus security.TrustStatus
		wantErr     bool
	}{
		{
			name:        "trust mode always",
			trustMode:   "always",
			trustStatus: security.TrustStatusUntrusted,
			wantErr:     false,
		},
		{
			name:        "trust mode never",
			trustMode:   "never",
			trustStatus: security.TrustStatusUntrusted,
			wantErr:     true,
		},
		{
			name:        "tofu mode trusted",
			trustMode:   "tofu",
			trustStatus: security.TrustStatusTrusted,
			wantErr:     false,
		},
		{
			name:        "tofu mode untrusted",
			trustMode:   "tofu",
			trustStatus: security.TrustStatusUntrusted,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.AppConfig{
				TrustMode: tt.trustMode,
			}

			// Set up trust status if needed
			if tt.trustMode == "tofu" {
				if tt.trustStatus == security.TrustStatusTrusted {
					tm := security.NewTrustManager()
					_ = tm.TrustFile(wtFile)
				} else {
					// For untrusted, create a file that exists but isn't trusted
					untrustedFile := filepath.Join(tmpDir, "untrusted.wt")
					if err := os.WriteFile(untrustedFile, []byte("test"), 0o600); err != nil {
						t.Fatalf("failed to create untrusted file: %v", err)
					}
					wtFile = untrustedFile
				}
			}

			err := checkTrust(ctx, cfg, wtFile)
			if tt.wantErr && err == nil {
				t.Fatal("expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestRunInitCommands(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tmpDir := t.TempDir()
	mainPath := tmpDir
	wtPath := filepath.Join(tmpDir, "worktree")

	cfg := &config.AppConfig{
		InitCommands: []string{"echo init1", "echo init2"},
	}

	svc := &fakeGitService{
		mainWorktreePath: mainPath,
		resolveRepoName:  testRepoName,
		executedCommands: nil, // Success
	}

	err := runInitCommands(ctx, svc, cfg, "branch", wtPath, appservices.LazyWorktreeContext{}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := svc.lastExecutedEnv["LAZYWORKTREE_NUMBER"]; got != "" {
		t.Fatalf("expected empty lazyworktree number without context, got %q", got)
	}
	if got := svc.lastExecutedEnv["REPO_REPONAME"]; got != testRepoName {
		t.Fatalf("expected repo name component, got %q", got)
	}

	lazyCtx := appservices.LazyWorktreeContext{
		Type:        "issue",
		Number:      "42",
		Title:       "Issue title",
		URL:         "https://example.com/issues/42",
		Description: "Issue body",
	}
	err = runInitCommands(ctx, svc, cfg, "branch", wtPath, lazyCtx, false)
	if err != nil {
		t.Fatalf("unexpected error with context: %v", err)
	}
	if got := svc.lastExecutedEnv["LAZYWORKTREE_NUMBER"]; got != "42" {
		t.Fatalf("expected contextual lazyworktree number, got %q", got)
	}
	if got := svc.lastExecutedEnv["LAZYWORKTREE_TITLE"]; got != "Issue title" {
		t.Fatalf("expected contextual lazyworktree title, got %q", got)
	}

	// Test with no commands
	cfg2 := &config.AppConfig{
		InitCommands: []string{},
	}
	err = runInitCommands(ctx, svc, cfg2, "branch", wtPath, appservices.LazyWorktreeContext{}, false)
	if err != nil {
		t.Fatalf("unexpected error with no commands: %v", err)
	}
}

func TestRunTerminateCommands(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tmpDir := t.TempDir()
	mainPath := tmpDir
	wtPath := filepath.Join(tmpDir, "worktree")

	cfg := &config.AppConfig{
		TerminateCommands: []string{"echo terminate1"},
	}

	svc := &fakeGitService{
		mainWorktreePath: mainPath,
		resolveRepoName:  testRepoName,
		executedCommands: nil, // Success
	}

	err := runTerminateCommands(ctx, svc, cfg, "branch", wtPath, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test with no commands
	cfg2 := &config.AppConfig{
		TerminateCommands: []string{},
	}
	called := false
	err = runTerminateCommands(ctx, svc, cfg2, "branch", wtPath, func() appservices.LazyWorktreeContext {
		called = true
		return appservices.LazyWorktreeContext{Type: "pr"}
	}, false)
	if err != nil {
		t.Fatalf("unexpected error with no commands: %v", err)
	}
	if called {
		t.Fatal("expected lazy context provider not to run when no terminate commands exist")
	}
}

func TestLazyWorktreeContextForWorktreeFetchesPR(t *testing.T) {
	t.Parallel()

	wt := &models.WorktreeInfo{Path: "/tmp/wt", Branch: "feature"}
	svc := &fakeGitService{
		prForWorktree: &models.PRInfo{
			Number: 77,
			Title:  "PR title",
			Body:   "PR body",
			URL:    "https://example.com/pull/77",
		},
	}

	ctx := lazyWorktreeContextForWorktree(context.Background(), svc, wt)
	if got := ctx.Type; got != "pr" {
		t.Fatalf("expected pr context, got %q", got)
	}
	if got := ctx.Number; got != "77" {
		t.Fatalf("expected PR number, got %q", got)
	}
	if wt.PR == nil || wt.PR.Number != 77 {
		t.Fatalf("expected fetched PR to be cached on worktree, got %#v", wt.PR)
	}
}

func TestGetCurrentWorktreeWithChanges(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tmpDir := t.TempDir()
	wtPath := filepath.Join(tmpDir, "worktree")

	worktrees := []*models.WorktreeInfo{
		{Path: wtPath, Branch: "main"},
	}

	svc := &fakeGitService{
		worktrees: worktrees,
		runGitOutput: map[string]string{
			filepath.Join("git", "status", "--porcelain"): "M file.txt\n",
		},
	}

	// Use mock filesystem
	fs := &mockFilesystem{
		getwdFunc: func() (string, error) {
			return wtPath, nil
		},
	}

	wt, hasChanges, err := getCurrentWorktreeWithChangesFS(ctx, svc, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wt == nil {
		t.Fatal("expected worktree to be found")
	}
	if !hasChanges {
		t.Error("expected changes to be detected")
	}

	// Test with no changes
	svc2 := &fakeGitService{
		worktrees: worktrees,
		runGitOutput: map[string]string{
			filepath.Join("git", "status", "--porcelain"): "",
		},
	}

	wt2, hasChanges2, err := getCurrentWorktreeWithChangesFS(ctx, svc2, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wt2 == nil {
		t.Fatal("expected worktree to be found")
	}
	if hasChanges2 {
		t.Error("expected no changes to be detected")
	}
}

func TestCreateFromBranchWorktreeNameDiffersFromSource(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sourceBranch := "main"
	worktreeName := "strava"
	showRef := filepath.Join("git", "show-ref", "--verify", "refs/heads/"+worktreeName)

	newSvc := func(t *testing.T, existing bool, worktrees []*models.WorktreeInfo) *fakeGitService {
		t.Helper()
		mainPath := filepath.Join(t.TempDir(), "main")
		if err := os.MkdirAll(mainPath, 0o750); err != nil {
			t.Fatalf("failed to create main path: %v", err)
		}
		runGitOutput := map[string]string{
			filepath.Join("git", "rev-parse", "--verify", sourceBranch):              "abc123\n",
			filepath.Join("git", "show-ref", "--verify", "refs/heads/"+sourceBranch): "abc123\n",
		}
		if existing {
			runGitOutput[showRef] = "def456\n"
		}
		return &fakeGitService{
			resolveRepoName:     testRepoName,
			mainWorktreePath:    mainPath,
			runCommandCheckedOK: true,
			runGitOutput:        runGitOutput,
			worktrees:           worktrees,
		}
	}

	t.Run("creates new branch when name is free", func(t *testing.T) {
		svc := newSvc(t, false, nil)
		cfg := &config.AppConfig{WorktreeDir: t.TempDir(), InitCommands: []string{}}

		if _, err := CreateFromBranch(ctx, svc, cfg, sourceBranch, worktreeName, false, true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		last := svc.runCommandCheckedCalls[len(svc.runCommandCheckedCalls)-1]
		assert.Contains(t, last, "-b")
		assert.Contains(t, last, worktreeName)
		assert.Equal(t, sourceBranch, last[len(last)-1])
	})

	t.Run("reuses existing local branch instead of -b", func(t *testing.T) {
		svc := newSvc(t, true, nil)
		cfg := &config.AppConfig{WorktreeDir: t.TempDir(), InitCommands: []string{}}

		if _, err := CreateFromBranch(ctx, svc, cfg, sourceBranch, worktreeName, false, true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		last := svc.runCommandCheckedCalls[len(svc.runCommandCheckedCalls)-1]
		assert.NotContains(t, last, "-b")
		assert.Equal(t, worktreeName, last[len(last)-1])
	})

	t.Run("fails when existing branch is checked out elsewhere", func(t *testing.T) {
		otherPath := filepath.Join(t.TempDir(), "elsewhere")
		svc := newSvc(t, true, []*models.WorktreeInfo{{Path: otherPath, Branch: worktreeName}})
		cfg := &config.AppConfig{WorktreeDir: t.TempDir(), InitCommands: []string{}}

		_, err := CreateFromBranch(ctx, svc, cfg, sourceBranch, worktreeName, false, true)
		if err == nil {
			t.Fatal("expected error for branch already checked out")
		}
		assert.Contains(t, err.Error(), otherPath)
		for _, call := range svc.runCommandCheckedCalls {
			assert.NotContains(t, call, "add")
		}
	})
}

func TestCreateFromBranchDirectCheckout(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	revParse := func(ref string) string { return filepath.Join("git", "rev-parse", "--verify", ref) }
	localRef := func(b string) string { return filepath.Join("git", "show-ref", "--verify", "refs/heads/"+b) }
	remoteRef := func(r string) string { return filepath.Join("git", "show-ref", "--verify", "refs/remotes/"+r) }

	setup := func(t *testing.T, refs []string, worktrees []*models.WorktreeInfo) (*fakeGitService, *config.AppConfig, string) {
		t.Helper()
		runGitOutput := map[string]string{}
		for _, ref := range refs {
			runGitOutput[ref] = "abc123\n"
		}
		cfg := &config.AppConfig{WorktreeDir: t.TempDir(), InitCommands: []string{}}
		return &fakeGitService{
			resolveRepoName:     testRepoName,
			mainWorktreePath:    t.TempDir(),
			runCommandCheckedOK: true,
			runGitOutput:        runGitOutput,
			worktrees:           worktrees,
		}, cfg, filepath.Join(cfg.WorktreeDir, testRepoName)
	}
	worktreeAdd := func(t *testing.T, svc *fakeGitService) []string {
		t.Helper()
		for _, call := range svc.runCommandCheckedCalls {
			if len(call) > 2 && call[1] == "worktree" && call[2] == "add" {
				return call
			}
		}
		t.Fatal("no git worktree add call recorded")
		return nil
	}

	t.Run("checks out free local branch as is", func(t *testing.T) {
		t.Parallel()
		svc, cfg, base := setup(t, []string{revParse("feature/x"), localRef("feature/x")}, nil)

		out, err := CreateFromBranch(ctx, svc, cfg, "feature/x", "", false, true)
		require.NoError(t, err)
		want := filepath.Join(base, "feature-x")
		assert.Equal(t, want, out)
		assert.Equal(t, []string{"git", "worktree", "add", want, "feature/x"}, worktreeAdd(t, svc))
	})

	t.Run("falls back to new branch when attached", func(t *testing.T) {
		t.Parallel()
		attached := []*models.WorktreeInfo{{Path: "/elsewhere", Branch: "feature"}}
		svc, cfg, _ := setup(t, []string{revParse("feature"), localRef("feature")}, attached)

		out, err := CreateFromBranch(ctx, svc, cfg, "feature", "", false, true)
		require.NoError(t, err)
		name := filepath.Base(out)
		assert.True(t, strings.HasPrefix(name, "feature-"), "unexpected name %q", name)
		assert.Equal(t, []string{"git", "worktree", "add", "-b", name, out, "feature"}, worktreeAdd(t, svc))
	})

	t.Run("creates tracking branch from remote", func(t *testing.T) {
		t.Parallel()
		svc, cfg, base := setup(t, []string{revParse("origin/foo"), remoteRef("origin/foo")}, nil)

		out, err := CreateFromBranch(ctx, svc, cfg, "origin/foo", "", false, true)
		require.NoError(t, err)
		want := filepath.Join(base, "foo")
		assert.Equal(t, want, out)
		assert.Equal(t, []string{"git", "worktree", "add", "-b", "foo", "--track", want, "origin/foo"}, worktreeAdd(t, svc))
	})

	t.Run("reuses local branch matching remote", func(t *testing.T) {
		t.Parallel()
		svc, cfg, base := setup(t, []string{revParse("origin/foo"), remoteRef("origin/foo"), localRef("foo")}, nil)

		out, err := CreateFromBranch(ctx, svc, cfg, "origin/foo", "", false, true)
		require.NoError(t, err)
		want := filepath.Join(base, "foo")
		assert.Equal(t, want, out)
		assert.Equal(t, []string{"git", "worktree", "add", want, "foo"}, worktreeAdd(t, svc))
	})

	t.Run("fails when target path exists", func(t *testing.T) {
		t.Parallel()
		svc, cfg, base := setup(t, []string{revParse("feature"), localRef("feature")}, nil)
		require.NoError(t, os.MkdirAll(filepath.Join(base, "feature"), 0o750))

		_, err := CreateFromBranch(ctx, svc, cfg, "feature", "", false, true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "path already exists")
		assert.Empty(t, svc.runCommandCheckedCalls)
	})

	t.Run("with change checks out branch and applies stash", func(t *testing.T) {
		t.Parallel()
		current := &models.WorktreeInfo{Path: t.TempDir(), Branch: "main"}
		inner, cfg, base := setup(t, []string{revParse("feature"), localRef("feature")}, []*models.WorktreeInfo{current})
		inner.runGitOutput[filepath.Join("git", "status", "--porcelain")] = " M file.go\n"
		inner.runGitOutput[filepath.Join("git", "stash", "list", "-1", "--format=%gd")] = "stash@{0}\n"
		svc := &stashSequenceGitService{fakeGitService: inner}
		fs := &mockFilesystem{getwdFunc: func() (string, error) { return current.Path, nil }}

		out, err := CreateFromBranchWithFS(ctx, svc, cfg, "feature", "", true, true, fs)
		require.NoError(t, err)
		want := filepath.Join(base, "feature")
		assert.Equal(t, want, out)
		assert.Equal(t, []string{"git", "worktree", "add", want, "feature"}, worktreeAdd(t, inner))
	})
}

// stashSequenceGitService reports a new stash entry after the first lookup so
// that the --with-change flow sees a successful stash push.
type stashSequenceGitService struct {
	*fakeGitService
	stashLookups int
}

func (s *stashSequenceGitService) RunGit(ctx context.Context, args []string, cwd string, okReturncodes []int, strip, silent bool) string {
	if slices.Equal(args, []string{"git", "stash", "list", "-1", "--format=%H"}) {
		s.stashLookups++
		if s.stashLookups > 1 {
			return "newstash"
		}
		return ""
	}
	return s.fakeGitService.RunGit(ctx, args, cwd, okReturncodes, strip, silent)
}

func TestCreateFromBranch(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tmpDir := t.TempDir()
	cfg := &config.AppConfig{
		WorktreeDir:  tmpDir,
		InitCommands: []string{},
	}

	t.Run("branch does not exist", func(t *testing.T) {
		svc := &fakeGitService{
			resolveRepoName: testRepoName,
			runGitOutput: map[string]string{
				filepath.Join("git", "rev-parse", "--verify", "nonexistent"): "",
			},
		}

		_, err := CreateFromBranch(ctx, svc, cfg, "nonexistent", "", false, false)
		if err == nil {
			t.Fatal("expected error for nonexistent branch")
		}
	})

	t.Run("path already exists", func(t *testing.T) {
		repoName := testRepoName
		branchName := "existing"
		worktreeName := "existing-wt"
		targetPath := filepath.Join(tmpDir, repoName, worktreeName)

		// Create the path
		if err := os.MkdirAll(targetPath, 0o750); err != nil {
			t.Fatalf("failed to create path: %v", err)
		}

		svc := &fakeGitService{
			resolveRepoName: repoName,
			runGitOutput: map[string]string{
				filepath.Join("git", "rev-parse", "--verify", branchName): "abc123\n",
			},
		}

		// Provide explicit worktreeName to avoid random generation
		_, err := CreateFromBranch(ctx, svc, cfg, branchName, worktreeName, false, false)
		if err == nil {
			t.Fatal("expected error for existing path")
		}
	})

	t.Run("successful creation", func(t *testing.T) {
		repoName := testRepoName
		branchName := "new-branch"
		mainPath := filepath.Join(tmpDir, "main")
		if err := os.MkdirAll(mainPath, 0o750); err != nil {
			t.Fatalf("failed to create main path: %v", err)
		}

		svc := &fakeGitService{
			resolveRepoName:     repoName,
			mainWorktreePath:    mainPath,
			runCommandCheckedOK: true,
			runGitOutput: map[string]string{
				filepath.Join("git", "rev-parse", "--verify", branchName):              "abc123\n",
				filepath.Join("git", "show-ref", "--verify", "refs/heads/"+branchName): "abc123\n",
			},
		}

		outputPath, err := CreateFromBranch(ctx, svc, cfg, branchName, "", false, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if outputPath == "" {
			t.Fatal("expected output path to be returned")
		}
	})

	t.Run("explicit branch name provided", func(t *testing.T) {
		repoName := testRepoName
		sourceBranch := "main"
		worktreeName := "feature-1"
		mainPath := filepath.Join(tmpDir, "main")
		if err := os.MkdirAll(mainPath, 0o750); err != nil {
			t.Fatalf("failed to create main path: %v", err)
		}

		svc := &fakeGitService{
			resolveRepoName:     repoName,
			mainWorktreePath:    mainPath,
			runCommandCheckedOK: true,
			runGitOutput: map[string]string{
				filepath.Join("git", "rev-parse", "--verify", sourceBranch):              "abc123\n",
				filepath.Join("git", "show-ref", "--verify", "refs/heads/"+sourceBranch): "abc123\n",
			},
		}

		outputPath, err := CreateFromBranch(ctx, svc, cfg, sourceBranch, worktreeName, false, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if outputPath == "" {
			t.Fatal("expected output path to be returned")
		}

		// Verify the worktree was created with the explicit name
		expectedPath := filepath.Join(tmpDir, repoName, worktreeName)
		if svc.lastWorktreeAddPath != expectedPath {
			t.Errorf("expected path %q, got %q", expectedPath, svc.lastWorktreeAddPath)
		}
	})

	t.Run("explicit branch name gets sanitised", func(t *testing.T) {
		repoName := testRepoName
		sourceBranch := "main"
		worktreeName := "Feature@#123!"
		mainPath := filepath.Join(tmpDir, "main")
		if err := os.MkdirAll(mainPath, 0o750); err != nil {
			t.Fatalf("failed to create main path: %v", err)
		}

		svc := &fakeGitService{
			resolveRepoName:     repoName,
			mainWorktreePath:    mainPath,
			runCommandCheckedOK: true,
			runGitOutput: map[string]string{
				filepath.Join("git", "rev-parse", "--verify", sourceBranch):              "abc123\n",
				filepath.Join("git", "show-ref", "--verify", "refs/heads/"+sourceBranch): "abc123\n",
			},
		}

		outputPath, err := CreateFromBranch(ctx, svc, cfg, sourceBranch, worktreeName, false, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if outputPath == "" {
			t.Fatal("expected output path to be returned")
		}

		// Verify the name was sanitised
		expectedPath := filepath.Join(tmpDir, repoName, "feature-123")
		if svc.lastWorktreeAddPath != expectedPath {
			t.Errorf("expected sanitised path %q, got %q", expectedPath, svc.lastWorktreeAddPath)
		}
	})

	t.Run("invalid branch name all special chars", func(t *testing.T) {
		repoName := testRepoName
		sourceBranch := "main"
		worktreeName := "@#$%^&*()"

		svc := &fakeGitService{
			resolveRepoName: repoName,
			runGitOutput: map[string]string{
				filepath.Join("git", "rev-parse", "--verify", sourceBranch): "abc123\n",
			},
		}

		_, err := CreateFromBranch(ctx, svc, cfg, sourceBranch, worktreeName, false, true)
		if err == nil {
			t.Fatal("expected error for invalid worktree name")
		}
		if !contains(err.Error(), "invalid worktree name") {
			t.Errorf("expected 'invalid worktree name' error, got: %v", err)
		}
	})
}

func TestDeleteWorktree(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tmpDir := t.TempDir()
	cfg := &config.AppConfig{
		WorktreeDir:       tmpDir,
		TerminateCommands: []string{},
	}

	t.Run("worktree not found", func(t *testing.T) {
		svc := &fakeGitService{
			resolveRepoName: testRepoName,
			worktrees:       []*models.WorktreeInfo{},
			worktreesErr:    nil,
		}

		err := DeleteWorktree(ctx, svc, cfg, "nonexistent", true, false)
		if err == nil {
			t.Fatal("expected error for nonexistent worktree")
		}
	})

	t.Run("successful deletion", func(t *testing.T) {
		wtPath := filepath.Join(tmpDir, testRepoName, "worktree")
		worktrees := []*models.WorktreeInfo{
			{Path: wtPath, Branch: "worktree"},
		}

		svc := &fakeGitService{
			resolveRepoName:     testRepoName,
			worktrees:           worktrees,
			runCommandCheckedOK: true,
		}

		err := DeleteWorktree(ctx, svc, cfg, "worktree", true, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestNoteShow(t *testing.T) {
	ctx := context.Background()

	t.Run("prints note text for worktree found by name", func(t *testing.T) {
		tmpDir := t.TempDir()
		wtPath := filepath.Join(tmpDir, "my-feature")
		require.NoError(t, os.MkdirAll(wtPath, 0o750))

		cfg := &config.AppConfig{
			WorktreeDir: tmpDir,
		}

		worktrees := []*models.WorktreeInfo{
			{Path: wtPath, Branch: "my-feature", IsMain: false},
			{Path: filepath.Join(tmpDir, "main"), Branch: "main", IsMain: true},
		}

		svc := &fakeGitService{
			resolveRepoName: testRepoName,
			worktrees:       worktrees,
		}

		// Pre-populate note via SaveWorktreeNotes
		key := appservices.WorktreeNoteKey(testRepoName, tmpDir, "", wtPath)
		notes := map[string]models.WorktreeNote{
			key: {Note: "hello from note", UpdatedAt: 1},
		}
		require.NoError(t, appservices.SaveWorktreeNotes(testRepoName, tmpDir, "", "", notes, nil))

		// Capture stdout
		oldStdout := os.Stdout
		r, w, pipeErr := os.Pipe()
		require.NoError(t, pipeErr)
		os.Stdout = w

		err := NoteShow(ctx, svc, cfg, "my-feature")

		_ = w.Close()
		os.Stdout = oldStdout

		require.NoError(t, err)

		out, copyErr := io.ReadAll(r)
		require.NoError(t, copyErr)

		assert.Contains(t, string(out), "hello from note")
	})

	t.Run("returns nil when no note exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		wtPath := filepath.Join(tmpDir, "my-feature")
		require.NoError(t, os.MkdirAll(wtPath, 0o750))

		cfg := &config.AppConfig{
			WorktreeDir: tmpDir,
		}

		worktrees := []*models.WorktreeInfo{
			{Path: wtPath, Branch: "my-feature", IsMain: false},
			{Path: filepath.Join(tmpDir, "main"), Branch: "main", IsMain: true},
		}

		svc := &fakeGitService{
			resolveRepoName: testRepoName,
			worktrees:       worktrees,
		}

		err := NoteShow(ctx, svc, cfg, "my-feature")
		assert.NoError(t, err)
	})

	t.Run("falls back to legacy absolute path key in shared notes", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoDir := filepath.Join(tmpDir, testRepoName)
		wtPath := filepath.Join(repoDir, "my-feature")
		require.NoError(t, os.MkdirAll(wtPath, 0o750))

		sharedNotesPath := filepath.Join(tmpDir, "shared-notes.json")
		payload := map[string]map[string]models.WorktreeNote{
			testRepoName: {
				filepath.Clean(wtPath): {Note: "legacy shared note", UpdatedAt: 1},
			},
		}
		data, err := json.Marshal(payload)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(sharedNotesPath, data, 0o600))

		cfg := &config.AppConfig{
			WorktreeDir:       tmpDir,
			WorktreeNotesPath: sharedNotesPath,
		}

		worktrees := []*models.WorktreeInfo{
			{Path: wtPath, Branch: "my-feature", IsMain: false},
			{Path: filepath.Join(repoDir, "main"), Branch: "main", IsMain: true},
		}

		svc := &fakeGitService{
			resolveRepoName: testRepoName,
			worktrees:       worktrees,
		}

		oldStdout := os.Stdout
		r, w, pipeErr := os.Pipe()
		require.NoError(t, pipeErr)
		os.Stdout = w

		err = NoteShow(ctx, svc, cfg, "my-feature")

		_ = w.Close()
		os.Stdout = oldStdout

		require.NoError(t, err)

		out, copyErr := io.ReadAll(r)
		require.NoError(t, copyErr)
		assert.Contains(t, string(out), "legacy shared note")
	})

	t.Run("errors when worktree not found", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfg := &config.AppConfig{
			WorktreeDir: tmpDir,
		}

		svc := &fakeGitService{
			resolveRepoName: testRepoName,
			worktrees:       []*models.WorktreeInfo{},
		}

		err := NoteShow(ctx, svc, cfg, "nonexistent")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "worktree not found")
	})
}

func TestNoteEdit(t *testing.T) {
	ctx := context.Background()

	t.Run("edit from stdin", func(t *testing.T) {
		tmpDir := t.TempDir()
		wtPath := filepath.Join(tmpDir, "my-feature")
		require.NoError(t, os.MkdirAll(wtPath, 0o750))

		cfg := &config.AppConfig{
			WorktreeDir: tmpDir,
		}

		worktrees := []*models.WorktreeInfo{
			{Path: wtPath, Branch: "my-feature", IsMain: false},
			{Path: filepath.Join(tmpDir, "main"), Branch: "main", IsMain: true},
		}

		svc := &fakeGitService{
			resolveRepoName: testRepoName,
			worktrees:       worktrees,
		}

		// Set up stdin with note content
		oldStdin := os.Stdin
		r, w, pipeErr := os.Pipe()
		require.NoError(t, pipeErr)
		_, _ = w.WriteString("---\nicon: rocket\n---\nnew note from stdin\n")
		_ = w.Close()
		os.Stdin = r

		err := NoteEdit(ctx, svc, cfg, "my-feature", "-")
		os.Stdin = oldStdin

		require.NoError(t, err)

		// Verify note was saved
		key := appservices.WorktreeNoteKey(testRepoName, tmpDir, "", wtPath)
		notes, loadErr := appservices.LoadWorktreeNotes(testRepoName, tmpDir, "", "", nil)
		require.NoError(t, loadErr)
		assert.Equal(t, "new note from stdin", notes[key].Note)
		assert.Equal(t, "rocket", notes[key].Icon)
	})

	t.Run("edit from file", func(t *testing.T) {
		tmpDir := t.TempDir()
		wtPath := filepath.Join(tmpDir, "my-feature")
		require.NoError(t, os.MkdirAll(wtPath, 0o750))

		cfg := &config.AppConfig{
			WorktreeDir: tmpDir,
		}

		worktrees := []*models.WorktreeInfo{
			{Path: wtPath, Branch: "my-feature", IsMain: false},
			{Path: filepath.Join(tmpDir, "main"), Branch: "main", IsMain: true},
		}

		svc := &fakeGitService{
			resolveRepoName: testRepoName,
			worktrees:       worktrees,
		}

		// Write a note file
		noteFile := filepath.Join(tmpDir, "input-note.md")
		require.NoError(t, os.WriteFile(noteFile, []byte("note from file\n"), 0o600))

		err := NoteEdit(ctx, svc, cfg, "my-feature", noteFile)
		require.NoError(t, err)

		// Verify note was saved
		key := appservices.WorktreeNoteKey(testRepoName, tmpDir, "", wtPath)
		notes, loadErr := appservices.LoadWorktreeNotes(testRepoName, tmpDir, "", "", nil)
		require.NoError(t, loadErr)
		assert.Equal(t, "note from file", notes[key].Note)
	})

	t.Run("empty input clears note", func(t *testing.T) {
		tmpDir := t.TempDir()
		wtPath := filepath.Join(tmpDir, "my-feature")
		require.NoError(t, os.MkdirAll(wtPath, 0o750))

		cfg := &config.AppConfig{
			WorktreeDir: tmpDir,
		}

		worktrees := []*models.WorktreeInfo{
			{Path: wtPath, Branch: "my-feature", IsMain: false},
			{Path: filepath.Join(tmpDir, "main"), Branch: "main", IsMain: true},
		}

		svc := &fakeGitService{
			resolveRepoName: testRepoName,
			worktrees:       worktrees,
		}

		// Pre-populate a note
		key := appservices.WorktreeNoteKey(testRepoName, tmpDir, "", wtPath)
		notes := map[string]models.WorktreeNote{
			key: {Note: "existing note", UpdatedAt: 1},
		}
		require.NoError(t, appservices.SaveWorktreeNotes(testRepoName, tmpDir, "", "", notes, nil))

		// Send empty stdin
		oldStdin := os.Stdin
		r, w, pipeErr := os.Pipe()
		require.NoError(t, pipeErr)
		_ = w.Close()
		os.Stdin = r

		err := NoteEdit(ctx, svc, cfg, "my-feature", "-")
		os.Stdin = oldStdin
		require.NoError(t, err)

		// Verify note was cleared
		notes, loadErr := appservices.LoadWorktreeNotes(testRepoName, tmpDir, "", "", nil)
		require.NoError(t, loadErr)
		_, exists := notes[key]
		assert.False(t, exists)
	})

	t.Run("empty input removes splitted note file", func(t *testing.T) {
		tmpDir := t.TempDir()
		wtPath := filepath.Join(tmpDir, "my-feature")
		require.NoError(t, os.MkdirAll(wtPath, 0o750))

		notesPath := filepath.Join(tmpDir, "notes", "${WORKTREE_NAME}.md")
		cfg := &config.AppConfig{
			WorktreeDir:       tmpDir,
			WorktreeNoteType:  config.NoteTypeSplitted,
			WorktreeNotesPath: notesPath,
		}

		worktrees := []*models.WorktreeInfo{
			{Path: wtPath, Branch: "my-feature", IsMain: false},
			{Path: filepath.Join(tmpDir, "main"), Branch: "main", IsMain: true},
		}

		svc := &fakeGitService{
			resolveRepoName: testRepoName,
			worktrees:       worktrees,
		}

		require.NoError(t, appservices.SaveWorktreeNotes(testRepoName, tmpDir, notesPath, config.NoteTypeSplitted, map[string]models.WorktreeNote{
			"my-feature": {Note: "existing note", UpdatedAt: 1},
		}, nil))

		noteFile := filepath.Join(tmpDir, "notes", "my-feature.md")
		if _, err := os.Stat(noteFile); err != nil {
			t.Fatalf("expected note file to exist: %v", err)
		}

		oldStdin := os.Stdin
		r, w, pipeErr := os.Pipe()
		require.NoError(t, pipeErr)
		_ = w.Close()
		os.Stdin = r

		err := NoteEdit(ctx, svc, cfg, "my-feature", "-")
		os.Stdin = oldStdin
		require.NoError(t, err)

		if _, err := os.Stat(noteFile); !os.IsNotExist(err) {
			t.Fatalf("expected note file removed, got err=%v", err)
		}
	})

	t.Run("edit in editor honours configured editor", func(t *testing.T) {
		tmpDir := t.TempDir()
		wtPath := filepath.Join(tmpDir, "my-feature")
		require.NoError(t, os.MkdirAll(wtPath, 0o750))

		editorScript := filepath.Join(tmpDir, "editor.sh")
		require.NoError(t, os.WriteFile(editorScript, []byte("#!/bin/sh\nlast=''\nfor arg in \"$@\"; do last=\"$arg\"; done\nprintf 'edited from config\\n' > \"$last\"\n"), 0o600))

		t.Setenv("EDITOR", "")
		t.Setenv("VISUAL", "")

		cfg := &config.AppConfig{
			WorktreeDir: tmpDir,
			Editor:      "sh " + editorScript,
		}

		worktrees := []*models.WorktreeInfo{
			{Path: wtPath, Branch: "my-feature", IsMain: false},
			{Path: filepath.Join(tmpDir, "main"), Branch: "main", IsMain: true},
		}

		svc := &fakeGitService{
			resolveRepoName: testRepoName,
			worktrees:       worktrees,
		}

		err := NoteEdit(ctx, svc, cfg, "my-feature", "")
		require.NoError(t, err)

		key := appservices.WorktreeNoteKey(testRepoName, tmpDir, "", wtPath)
		notes, loadErr := appservices.LoadWorktreeNotes(testRepoName, tmpDir, "", "", nil)
		require.NoError(t, loadErr)
		assert.Equal(t, "edited from config", notes[key].Note)
	})

	t.Run("edit in editor parses editor command flags", func(t *testing.T) {
		tmpDir := t.TempDir()
		wtPath := filepath.Join(tmpDir, "my-feature")
		require.NoError(t, os.MkdirAll(wtPath, 0o750))

		editorScript := filepath.Join(tmpDir, "editor-with-flags.sh")
		require.NoError(t, os.WriteFile(editorScript, []byte("#!/bin/sh\nlast=''\nfor arg in \"$@\"; do last=\"$arg\"; done\nprintf '%s\\n' '---' 'icon: rocket' '---' 'edited with flags' > \"$last\"\n"), 0o600))

		t.Setenv("EDITOR", "sh "+editorScript+" --wait")
		t.Setenv("VISUAL", "")

		cfg := &config.AppConfig{
			WorktreeDir: tmpDir,
		}

		worktrees := []*models.WorktreeInfo{
			{Path: wtPath, Branch: "my-feature", IsMain: false},
			{Path: filepath.Join(tmpDir, "main"), Branch: "main", IsMain: true},
		}

		svc := &fakeGitService{
			resolveRepoName: testRepoName,
			worktrees:       worktrees,
		}

		err := NoteEdit(ctx, svc, cfg, "my-feature", "")
		require.NoError(t, err)

		key := appservices.WorktreeNoteKey(testRepoName, tmpDir, "", wtPath)
		notes, loadErr := appservices.LoadWorktreeNotes(testRepoName, tmpDir, "", "", nil)
		require.NoError(t, loadErr)
		assert.Equal(t, "edited with flags", notes[key].Note)
		assert.Equal(t, "rocket", notes[key].Icon)
	})

	t.Run("editing legacy shared note migrates key", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoDir := filepath.Join(tmpDir, testRepoName)
		wtPath := filepath.Join(repoDir, "my-feature")
		require.NoError(t, os.MkdirAll(wtPath, 0o750))

		sharedNotesPath := filepath.Join(tmpDir, "shared-notes.json")
		payload := map[string]map[string]models.WorktreeNote{
			testRepoName: {
				filepath.Clean(wtPath): {Note: "legacy note", UpdatedAt: 1},
			},
		}
		data, err := json.Marshal(payload)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(sharedNotesPath, data, 0o600))

		cfg := &config.AppConfig{
			WorktreeDir:       tmpDir,
			WorktreeNotesPath: sharedNotesPath,
		}

		worktrees := []*models.WorktreeInfo{
			{Path: wtPath, Branch: "my-feature", IsMain: false},
			{Path: filepath.Join(repoDir, "main"), Branch: "main", IsMain: true},
		}

		svc := &fakeGitService{
			resolveRepoName: testRepoName,
			worktrees:       worktrees,
		}

		noteFile := filepath.Join(tmpDir, "input-note.md")
		require.NoError(t, os.WriteFile(noteFile, []byte("updated shared note\n"), 0o600))

		err = NoteEdit(ctx, svc, cfg, "my-feature", noteFile)
		require.NoError(t, err)

		notes, loadErr := appservices.LoadWorktreeNotes(testRepoName, tmpDir, sharedNotesPath, "", nil)
		require.NoError(t, loadErr)

		relativeKey := appservices.WorktreeNoteKey(testRepoName, tmpDir, sharedNotesPath, wtPath)
		assert.Equal(t, "updated shared note", notes[relativeKey].Note)
		_, hasLegacy := notes[filepath.Clean(wtPath)]
		assert.False(t, hasLegacy)
	})

	t.Run("errors on nonexistent input file", func(t *testing.T) {
		tmpDir := t.TempDir()
		wtPath := filepath.Join(tmpDir, "my-feature")
		require.NoError(t, os.MkdirAll(wtPath, 0o750))

		cfg := &config.AppConfig{
			WorktreeDir: tmpDir,
		}

		worktrees := []*models.WorktreeInfo{
			{Path: wtPath, Branch: "my-feature", IsMain: false},
			{Path: filepath.Join(tmpDir, "main"), Branch: "main", IsMain: true},
		}

		svc := &fakeGitService{
			resolveRepoName: testRepoName,
			worktrees:       worktrees,
		}

		err := NoteEdit(ctx, svc, cfg, "my-feature", "/nonexistent/file.md")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read file")
	})
}

func TestResolveNoteContext(t *testing.T) {
	ctx := context.Background()

	t.Run("resolves from cwd", func(t *testing.T) {
		tmpDir := t.TempDir()
		wtPath := filepath.Join(tmpDir, "my-feature")
		require.NoError(t, os.MkdirAll(wtPath, 0o750))

		// Resolve symlinks to match os.Getwd() behaviour
		wtPath, err := filepath.EvalSymlinks(wtPath)
		require.NoError(t, err)
		tmpDir, err = filepath.EvalSymlinks(tmpDir)
		require.NoError(t, err)

		// Create the directory for worktree notes
		notesDir := filepath.Join(tmpDir, testRepoName)
		require.NoError(t, os.MkdirAll(notesDir, 0o750))

		cfg := &config.AppConfig{
			WorktreeDir: tmpDir,
		}

		worktrees := []*models.WorktreeInfo{
			{Path: wtPath, Branch: "my-feature", IsMain: false},
			{Path: filepath.Join(tmpDir, "main"), Branch: "main", IsMain: true},
		}

		svc := &fakeGitService{
			resolveRepoName: testRepoName,
			worktrees:       worktrees,
		}

		// Change to the worktree dir
		oldDir, cwdErr := os.Getwd()
		require.NoError(t, cwdErr)
		defer func() { _ = os.Chdir(oldDir) }()
		require.NoError(t, os.Chdir(wtPath))

		nc, err := resolveNoteContext(ctx, svc, cfg, "")
		require.NoError(t, err)
		assert.Equal(t, wtPath, nc.worktree.Path)
		assert.NotEmpty(t, nc.key)
	})

	t.Run("errors when cwd is not a worktree", func(t *testing.T) {
		tmpDir := t.TempDir()

		cfg := &config.AppConfig{
			WorktreeDir: tmpDir,
		}

		svc := &fakeGitService{
			resolveRepoName: testRepoName,
			worktrees:       []*models.WorktreeInfo{},
		}

		oldDir, cwdErr := os.Getwd()
		require.NoError(t, cwdErr)
		defer func() { _ = os.Chdir(oldDir) }()
		require.NoError(t, os.Chdir(tmpDir))

		_, err := resolveNoteContext(ctx, svc, cfg, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not inside a known worktree")
	})

	t.Run("splitted note type uses basename key", func(t *testing.T) {
		tmpDir := t.TempDir()
		wtPath := filepath.Join(tmpDir, "my-feature")
		require.NoError(t, os.MkdirAll(wtPath, 0o750))

		cfg := &config.AppConfig{
			WorktreeDir:       tmpDir,
			WorktreeNoteType:  config.NoteTypeSplitted,
			WorktreeNotesPath: filepath.Join(tmpDir, "notes", "${WORKTREE_NAME}.md"),
		}

		worktrees := []*models.WorktreeInfo{
			{Path: wtPath, Branch: "my-feature", IsMain: false},
			{Path: filepath.Join(tmpDir, "main"), Branch: "main", IsMain: true},
		}

		svc := &fakeGitService{
			resolveRepoName: testRepoName,
			worktrees:       worktrees,
		}

		nc, err := resolveNoteContext(ctx, svc, cfg, "my-feature")
		require.NoError(t, err)
		assert.Equal(t, "my-feature", nc.key)
	})
}
