package multiplexer

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

// fakeHerdrCreateScript checks branches out with git like Herdr does.
// HERDR_FAIL=before fails without a checkout, HERDR_FAIL=after fails once the
// checkout exists, HERDR_ALREADY_OPEN=1 makes worktree open find an existing
// workspace, and HERDR_BUSY=1 reports a command running in the first tab.
const fakeHerdrCreateScript = `
printf '%s\n' "$*" >> "$HERDR_TEST_LOG"
while [ $# -gt 0 ]; do
	case "$1" in
		--session) shift 2; continue;;
		session) echo '{"sessions":[{"name":"default","default":true,"running":true},{"name":"work","running":true}]}'; exit 0;;
		worktree)
			sub=$2; shift 2
			while [ $# -gt 0 ]; do
				case "$1" in
					--cwd) cwd=$2; shift 2;;
					--branch) branch=$2; shift 2;;
					--path) path=$2; shift 2;;
					*) shift;;
				esac
			done
			if [ "$sub" = create ]; then
				if [ "$HERDR_FAIL" = before ]; then echo 'worktree_create_failed' >&2; exit 1; fi
				git -C "$cwd" worktree add -q "$path" "$branch" >/dev/null 2>&1 || exit 1
				if [ "$HERDR_FAIL" = after ]; then echo 'lost reply' >&2; exit 1; fi
			fi
			already=false
			if [ "$sub" = open ] && [ "$HERDR_ALREADY_OPEN" = 1 ]; then already=true; fi
			echo '{"result":{"workspace":{"workspace_id":"w1","label":"feat"},"tab":{"tab_id":"w1:t1"},"root_pane":{"pane_id":"w1:p1"},"already_open":'$already'}}'
			exit 0;;
		pane)
			case "$2" in
				get) echo '{"result":{"pane":{"focused":false}}}';;
				process-info)
					if [ "$HERDR_BUSY" = 1 ]; then
						echo '{"result":{"process_info":{"foreground_process_group_id":2,"shell_pid":1}}}'
					else
						echo '{"result":{"process_info":{"foreground_process_group_id":1,"shell_pid":1}}}'
					fi;;
			esac
			exit 0;;
		tab)
			if [ "$2" = create ]; then echo '{"result":{"tab":{"tab_id":"w1:t2"},"root_pane":{"pane_id":"w1:p2"}}}'; fi
			exit 0;;
		workspace) exit 0;;
	esac
	shift
done
`

type herdrCreateFixture struct {
	repo     string
	wtPath   string
	log      string
	warnings []string
	creator  *HerdrWorktreeCreator
}

func newHerdrCreateFixture(t *testing.T) *herdrCreateFixture {
	t.Helper()
	f := &herdrCreateFixture{repo: t.TempDir(), log: filepath.Join(t.TempDir(), "herdr.log")}
	f.wtPath = filepath.Join(t.TempDir(), "feat")
	t.Setenv("HERDR_TEST_LOG", f.log)
	t.Setenv("HERDR_SESSION", "")
	t.Setenv("HERDR_SOCKET_PATH", "")
	t.Setenv("HERDR_FAIL", "")
	t.Setenv("HERDR_BUSY", "")
	t.Setenv("HERDR_ALREADY_OPEN", "")
	t.Setenv("TMPDIR", t.TempDir())
	f.git(t, "init", "-q", "-b", "main")
	f.git(t, "-c", "commit.gpgsign=false", "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init")
	f.creator = &HerdrWorktreeCreator{
		Bin:  writeFakeHerdr(t, fakeHerdrCreateScript),
		Warn: func(message string) { f.warnings = append(f.warnings, message) },
	}
	return f
}

func (f *herdrCreateFixture) git(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", f.repo}, args...)...) //nolint:gosec // test git invocation
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	return strings.TrimSpace(string(out))
}

func (f *herdrCreateFixture) branchExists(t *testing.T, branch string) bool {
	t.Helper()
	return exec.Command("git", "-C", f.repo, "show-ref", "--verify", "--quiet", "refs/heads/"+branch).Run() == nil //nolint:gosec // test git invocation
}

func (f *herdrCreateFixture) herdrLog(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(f.log)
	if os.IsNotExist(err) {
		return ""
	}
	require.NoError(t, err)
	return string(data)
}

// add runs the hook like git.Service does; runGit performs the original add.
func (f *herdrCreateFixture) add(t *testing.T, add models.WorktreeAdd) (bool, error) {
	t.Helper()
	if add.Dir == "" {
		add.Dir = f.repo
	}
	ranGit := false
	err := f.creator.AddWorktree(context.Background(), add, func() error {
		ranGit = true
		args := []string{"worktree", "add"}
		if add.NewBranch != "" {
			args = append(args, "-b", add.NewBranch)
		}
		out, err := exec.Command("git", append(append([]string{"-C", add.Dir}, args...), add.Path, add.Ref)...).CombinedOutput() //nolint:gosec // test git invocation
		if err != nil {
			return &exec.ExitError{Stderr: out}
		}
		return nil
	})
	return ranGit, err
}

func TestHerdrWorktreeCreatorCreatesNewBranchThroughHerdr(t *testing.T) {
	f := newHerdrCreateFixture(t)

	ranGit, err := f.add(t, models.WorktreeAdd{Path: f.wtPath, Ref: "main", NewBranch: "feat"})
	require.NoError(t, err)
	assert.False(t, ranGit)
	assert.DirExists(t, f.wtPath)
	assert.Equal(t, f.git(t, "rev-parse", "main"), f.git(t, "rev-parse", "feat"))
	assert.Empty(t, f.warnings)

	log := f.herdrLog(t)
	assert.Contains(t, log, "worktree create --cwd "+f.repo+" --branch feat --path "+f.wtPath+" --label feat --trust-repository --no-focus")
	assert.Contains(t, log, "workspace report-metadata w1 --source lazyworktree --token lazyworktree_id="+HerdrWorktreeKey(f.wtPath))

	// Outside a Herdr client a plain workspace is neither laid out nor focused.
	require.NoError(t, f.creator.FinaliseWorktree(context.Background(), f.wtPath, nil))
	assert.NotContains(t, f.herdrLog(t), "workspace focus")
	_, tracked := f.creator.take(f.wtPath)
	assert.False(t, tracked, "finalising must drop the worktree")
}

func TestHerdrWorktreeCreatorResolvesRefInOriginalCheckout(t *testing.T) {
	for _, ref := range []string{"HEAD", "HEAD~1"} {
		t.Run(ref, func(t *testing.T) {
			f := newHerdrCreateFixture(t)
			source := filepath.Join(t.TempDir(), "source")
			f.git(t, "worktree", "add", "-q", "-b", "source", source)
			for range 2 {
				f.git(t, "-C", source, "-c", "commit.gpgsign=false", "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "source")
			}
			want := f.git(t, "-C", source, "rev-parse", ref)
			require.NotEqual(t, want, f.git(t, "rev-parse", "main"))

			ranGit, err := f.add(t, models.WorktreeAdd{Dir: source, Path: f.wtPath, Ref: ref, NewBranch: "feat"})
			require.NoError(t, err)
			assert.False(t, ranGit)
			assert.Equal(t, want, f.git(t, "-C", f.wtPath, "rev-parse", "HEAD"))
			assert.Empty(t, f.warnings)
		})
	}
}

func TestHerdrWorktreeCreatorHandlesRelativePaths(t *testing.T) {
	for _, operation := range []string{"finalise", "forget", "remove"} {
		t.Run(operation, func(t *testing.T) {
			f := newHerdrCreateFixture(t)
			t.Chdir(f.repo)
			t.Setenv("HERDR_SESSION", "work")
			path := filepath.Join(".worktrees", "feat")
			f.wtPath = filepath.Join(f.repo, path)
			f.creator.Layout = &config.CustomCommand{Herdr: &config.HerdrCommand{
				Mode:         config.HerdrModeWorkspace,
				SessionName:  "configured",
				HerdrSession: "work",
				Attach:       true,
				Windows:      []config.TmuxWindow{{Name: "editor", Command: "vim"}},
			}}
			ctx := context.Background()
			ranGit, err := f.add(t, models.WorktreeAdd{Path: f.wtPath, Ref: "main", NewBranch: "feat"})
			require.NoError(t, err)
			require.False(t, ranGit)

			switch operation {
			case "finalise":
				require.NoError(t, f.creator.FinaliseWorktree(ctx, path, nil))
				log := f.herdrLog(t)
				assert.Contains(t, log, "workspace rename w1 configured")
				assert.Contains(t, log, "tab create --workspace w1 --label editor")
				assert.Contains(t, log, "workspace focus w1")
			case "forget":
				f.creator.ForgetWorktree(path)
				require.Len(t, f.warnings, 1)
				assert.Contains(t, f.warnings[0], "keeps its basic setup")
			case "remove":
				f.creator.WorktreeRemoved(ctx, path, false)
				require.Len(t, f.warnings, 1)
				assert.Contains(t, f.warnings[0], "stays open")
				f.creator.WorktreeRemoved(ctx, path, true)
				assert.Contains(t, f.herdrLog(t), "workspace close w1")
			}
			_, tracked := f.creator.lookup(f.wtPath)
			assert.False(t, tracked)
		})
	}
}

func TestHerdrWorktreeCreatorFallsBackToGitWhenHerdrFails(t *testing.T) {
	f := newHerdrCreateFixture(t)
	t.Setenv("HERDR_FAIL", "before")

	ranGit, err := f.add(t, models.WorktreeAdd{Path: f.wtPath, Ref: "main", NewBranch: "feat"})
	require.NoError(t, err)
	assert.True(t, ranGit, "git must create the worktree once Herdr fails")
	assert.DirExists(t, f.wtPath)
	require.Len(t, f.warnings, 1)
	assert.Contains(t, f.warnings[0], "creating it with git")
	_, tracked := f.creator.take(f.wtPath)
	assert.False(t, tracked)
}

func TestHerdrWorktreeCreatorKeepsCheckoutWhenHerdrFailsAfterCreatingIt(t *testing.T) {
	f := newHerdrCreateFixture(t)
	t.Setenv("HERDR_FAIL", "after")

	ranGit, err := f.add(t, models.WorktreeAdd{Path: f.wtPath, Ref: "main", NewBranch: "feat"})
	require.NoError(t, err)
	assert.False(t, ranGit, "an existing checkout must not be added again")
	assert.DirExists(t, f.wtPath)
	assert.True(t, f.branchExists(t, "feat"))
	require.Len(t, f.warnings, 1)
	assert.Contains(t, f.herdrLog(t), "worktree open --cwd "+f.repo+" --path "+f.wtPath)
	_, tracked := f.creator.take(f.wtPath)
	assert.True(t, tracked, "the recovered workspace must still be finalised")
}

func TestHerdrWorktreeCreatorLeavesAlreadyOpenWorkspacesAlone(t *testing.T) {
	f := newHerdrCreateFixture(t)
	t.Setenv("HERDR_FAIL", "after")
	t.Setenv("HERDR_ALREADY_OPEN", "1")
	ctx := context.Background()

	ranGit, err := f.add(t, models.WorktreeAdd{Path: f.wtPath, Ref: "main", NewBranch: "feat"})
	require.NoError(t, err)
	assert.False(t, ranGit)
	require.Len(t, f.warnings, 2)
	assert.Contains(t, f.warnings[1], "already open")

	f.creator.WorktreeRemoved(ctx, f.wtPath, true)
	log := f.herdrLog(t)
	assert.NotContains(t, log, "workspace close", "a workspace lazyworktree cannot prove it created is never closed")
	assert.NotContains(t, log, "report-metadata")
}

func TestHerdrWorktreeCreatorLeavesExistingBranchesToGit(t *testing.T) {
	f := newHerdrCreateFixture(t)
	f.git(t, "branch", "feat")

	ranGit, err := f.add(t, models.WorktreeAdd{Path: f.wtPath, Ref: "main", NewBranch: "feat"})
	require.Error(t, err, "git reports the existing branch")
	assert.True(t, ranGit)
	assert.True(t, f.branchExists(t, "feat"), "a branch lazyworktree did not create must survive")
	assert.NotContains(t, f.herdrLog(t), "worktree create")
}

func TestHerdrWorktreeCreatorChecksOutExistingLocalBranch(t *testing.T) {
	f := newHerdrCreateFixture(t)
	f.git(t, "branch", "feat")

	ranGit, err := f.add(t, models.WorktreeAdd{Path: f.wtPath, Ref: "feat"})
	require.NoError(t, err)
	assert.False(t, ranGit)
	assert.Contains(t, f.herdrLog(t), "worktree create --cwd "+f.repo+" --branch feat")
}

func TestHerdrWorktreeCreatorOpensDetachedCheckouts(t *testing.T) {
	f := newHerdrCreateFixture(t)

	ranGit, err := f.add(t, models.WorktreeAdd{Path: f.wtPath, Ref: f.git(t, "rev-parse", "HEAD")})
	require.NoError(t, err)
	assert.True(t, ranGit, "Herdr cannot check out a detached commit")
	log := f.herdrLog(t)
	assert.NotContains(t, log, "worktree create")
	assert.Contains(t, log, "worktree open --cwd "+f.repo+" --path "+f.wtPath)
}

func TestHerdrWorktreeCreatorClosesRolledBackWorkspaces(t *testing.T) {
	f := newHerdrCreateFixture(t)
	ctx := context.Background()
	_, err := f.add(t, models.WorktreeAdd{Path: f.wtPath, Ref: "main", NewBranch: "feat"})
	require.NoError(t, err)

	f.creator.WorktreeRemoved(ctx, f.wtPath, false)
	assert.NotContains(t, f.herdrLog(t), "workspace close")

	// A failed removal keeps the workspace tracked, so the retry closes it.
	f.creator.WorktreeRemoved(ctx, f.wtPath, true)
	assert.Contains(t, f.herdrLog(t), "workspace close w1")
	require.NoError(t, f.creator.FinaliseWorktree(ctx, f.wtPath, nil))
	assert.NotContains(t, f.herdrLog(t), "workspace rename")
}

func TestHerdrWorktreeCreatorFinalisesLayout(t *testing.T) {
	tests := []struct {
		name      string
		busy      string
		wantLog   []string
		rejectLog []string
	}{
		{
			name:    "replaces an idle first tab",
			wantLog: []string{"tab create --workspace w1 --label editor", "tab close w1:t1", "tab create --workspace w1 --label shell"},
		},
		{
			name:      "keeps a busy first tab",
			busy:      "1",
			wantLog:   []string{"tab create --workspace w1 --label editor", "tab create --workspace w1 --label shell"},
			rejectLog: []string{"tab close"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newHerdrCreateFixture(t)
			t.Setenv("HERDR_BUSY", tt.busy)
			t.Setenv("HERDR_SESSION", "work")
			f.creator.Layout = &config.CustomCommand{Herdr: &config.HerdrCommand{
				Mode:         config.HerdrModeWorkspace,
				SessionName:  "lw-$WORKTREE_NAME",
				HerdrSession: "work",
				Attach:       true,
				Windows:      []config.TmuxWindow{{Name: "editor", Command: "vim"}, {Name: "shell"}},
			}}
			ctx := context.Background()
			_, err := f.add(t, models.WorktreeAdd{Path: f.wtPath, Ref: "main", NewBranch: "feat"})
			require.NoError(t, err)

			env := map[string]string{"WORKTREE_NAME": "feat"}
			require.NoError(t, f.creator.FinaliseWorktree(ctx, f.wtPath, env))
			log := f.herdrLog(t)
			for _, want := range append(tt.wantLog, "workspace rename w1 lw-feat", "workspace focus w1") {
				assert.Contains(t, log, want)
			}
			for _, reject := range tt.rejectLog {
				assert.NotContains(t, log, reject)
			}
			assert.Less(t, strings.Index(log, "worktree create"), strings.Index(log, "workspace focus"))
		})
	}
}

func TestHerdrWorktreeCreatorNoFocusKeepsWorkspaceInBackground(t *testing.T) {
	f := newHerdrCreateFixture(t)
	t.Setenv("HERDR_SESSION", "work")
	f.creator.NoFocus = true
	f.creator.Layout = &config.CustomCommand{Herdr: &config.HerdrCommand{
		Mode:         config.HerdrModeWorkspace,
		HerdrSession: "work",
		Attach:       true,
	}}
	ctx := context.Background()
	_, err := f.add(t, models.WorktreeAdd{Path: f.wtPath, Ref: "main", NewBranch: "feat"})
	require.NoError(t, err)

	require.NoError(t, f.creator.FinaliseWorktree(ctx, f.wtPath, nil))
	assert.NotContains(t, f.herdrLog(t), "workspace focus")
}
