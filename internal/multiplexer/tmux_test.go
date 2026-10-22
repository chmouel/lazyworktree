package multiplexer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeTmuxSessionName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "simple name",
			input: "feature-branch",
			want:  "feature-branch",
		},
		{
			name:  "name with colon",
			input: "feature:branch",
			want:  "feature-branch",
		},
		{
			name:  "name with slash",
			input: "feature/branch",
			want:  "feature-branch",
		},
		{
			name:  "name with backslash",
			input: "feature\\branch",
			want:  "feature-branch",
		},
		{
			name:  "dot as older tmux releases rewrite it",
			input: "fix v1.2",
			want:  "fix v1_2",
		},
		{
			name:  "name with multiple special chars",
			input: "feature:foo/bar\\baz",
			want:  "feature-foo-bar-baz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeTmuxSessionName(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReadSessionFile(t *testing.T) {
	t.Run("file does not exist", func(t *testing.T) {
		got := ReadSessionFile("/nonexistent/path", "fallback")
		assert.Equal(t, "fallback", got)
	})

	t.Run("file is empty", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "session-")
		require.NoError(t, err)
		defer func() { _ = os.Remove(tmpFile.Name()) }() //#nosec G703 -- test cleanup
		_ = tmpFile.Close()

		got := ReadSessionFile(tmpFile.Name(), "fallback")
		assert.Equal(t, "fallback", got)
	})

	t.Run("file has content", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "session-")
		require.NoError(t, err)
		defer func() { _ = os.Remove(tmpFile.Name()) }() //#nosec G703 -- test cleanup
		_, err = tmpFile.WriteString("my-session")
		require.NoError(t, err)
		_ = tmpFile.Close()

		got := ReadSessionFile(tmpFile.Name(), "fallback")
		assert.Equal(t, "my-session", got)
	})

	t.Run("file has content with whitespace", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "session-")
		require.NoError(t, err)
		defer func() { _ = os.Remove(tmpFile.Name()) }() //#nosec G703 -- test cleanup
		_, err = tmpFile.WriteString("  my-session  \n")
		require.NoError(t, err)
		_ = tmpFile.Close()

		got := ReadSessionFile(tmpFile.Name(), "fallback")
		assert.Equal(t, "my-session", got)
	})
}

func TestResolveTmuxWindows(t *testing.T) {
	env := map[string]string{
		"BRANCH": "feature",
		"PATH":   "/worktree/path",
	}

	t.Run("empty windows", func(t *testing.T) {
		windows, ok := ResolveTmuxWindows([]config.TmuxWindow{}, env, "/default")
		assert.False(t, ok)
		assert.Nil(t, windows)
	})

	t.Run("single window with defaults", func(t *testing.T) {
		windows, ok := ResolveTmuxWindows([]config.TmuxWindow{
			{Name: "main", Command: "vim", Cwd: ""},
		}, env, "/default")
		require.True(t, ok)
		require.Len(t, windows, 1)
		assert.Equal(t, "main", windows[0].Name)
		assert.Contains(t, windows[0].Command, "vim")
		assert.Equal(t, "/default", windows[0].Cwd)
	})

	t.Run("window with env expansion", func(t *testing.T) {
		windows, ok := ResolveTmuxWindows([]config.TmuxWindow{
			{Name: "$BRANCH", Command: "ls", Cwd: "$PATH"},
		}, env, "/default")
		require.True(t, ok)
		require.Len(t, windows, 1)
		assert.Equal(t, "feature", windows[0].Name)
		assert.Equal(t, "/worktree/path", windows[0].Cwd)
	})

	t.Run("window with empty name gets default", func(t *testing.T) {
		windows, ok := ResolveTmuxWindows([]config.TmuxWindow{
			{Name: "", Command: "echo", Cwd: ""},
		}, env, "/default")
		require.True(t, ok)
		require.Len(t, windows, 1)
		assert.Equal(t, "window-1", windows[0].Name)
	})

	t.Run("multiple windows", func(t *testing.T) {
		windows, ok := ResolveTmuxWindows([]config.TmuxWindow{
			{Name: "first", Command: "vim", Cwd: "/first"},
			{Name: "second", Command: "git status", Cwd: "/second"},
		}, env, "/default")
		require.True(t, ok)
		require.Len(t, windows, 2)
		assert.Equal(t, "first", windows[0].Name)
		assert.Equal(t, "second", windows[1].Name)
	})
}

func TestBuildTmuxWindowCommand(t *testing.T) {
	t.Run("empty command", func(t *testing.T) {
		got := BuildTmuxWindowCommand("", map[string]string{})
		assert.Equal(t, "exec ${SHELL:-bash}", got)
	})

	t.Run("command without env", func(t *testing.T) {
		got := BuildTmuxWindowCommand("vim", map[string]string{})
		assert.Equal(t, "vim", got)
	})

	t.Run("command with env", func(t *testing.T) {
		env := map[string]string{"FOO": "bar"}
		got := BuildTmuxWindowCommand("echo test", env)
		assert.Contains(t, got, "export FOO='bar';")
		assert.Contains(t, got, "echo test")
	})

	t.Run("empty command with env", func(t *testing.T) {
		env := map[string]string{"FOO": "bar"}
		got := BuildTmuxWindowCommand("", env)
		assert.Contains(t, got, "export FOO='bar';")
		assert.Contains(t, got, "exec ${SHELL:-bash}")
	})
}

func TestBuildTmuxScript(t *testing.T) {
	windows := []ResolvedWindow{{Name: "main", Command: "vim", Cwd: "/path"}}

	t.Run("empty windows", func(t *testing.T) {
		cfg := &config.TmuxCommand{OnExists: "switch"}
		assert.Empty(t, BuildTmuxScript(TmuxSessionPlan{Name: "test-session"}, cfg, nil, map[string]string{}))
	})

	t.Run("creates by name and addresses the session by ID", func(t *testing.T) {
		cfg := &config.TmuxCommand{OnExists: "switch"}
		script := BuildTmuxScript(TmuxSessionPlan{Name: "wt-feature"}, cfg, windows, map[string]string{"FOO": "bar"})
		assert.Contains(t, script, "session='wt-feature'")
		assert.Contains(t, script, "session_id=$(tmux new-session -d -P -F '#{session_id}' -s \"$(printf '%s' \"$session\" | sed 's/#/##/g')\" -n 'main' -c '/path'")
		assert.Contains(t, script, "tmux set-environment -t \"$session_id\" 'FOO' 'bar'")
		assert.Contains(t, script, "LW_TMUX_SESSION_FILE")
		assert.NotContains(t, script, "kill-session")
		assert.NotContains(t, script, "set-option")
		assert.NotContains(t, script, "tmux attach")
	})

	t.Run("reuses an existing session and tags it", func(t *testing.T) {
		cfg := &config.TmuxCommand{OnExists: "switch", Attach: true}
		plan := TmuxSessionPlan{Name: "old name", ExistingID: "$3", Key: "/tmp/it's $(x)"}
		script := BuildTmuxScript(plan, cfg, windows, map[string]string{})
		assert.Contains(t, script, "session_id='$3'")
		assert.Contains(t, script, "worktree_key='/tmp/it'\"'\"'s $(x)'")
		assert.Contains(t, script, "tmux set-option -t \"$session_id\" @lazyworktree_path \"$worktree_key\"")
		assert.Contains(t, script, "tmux switch-client -t \"$session_id\" || true")
		assert.Contains(t, script, "tmux attach -t \"$session_id\" || true")
	})

	t.Run("kills the planned session before creating", func(t *testing.T) {
		cfg := &config.TmuxCommand{OnExists: "kill"}
		script := BuildTmuxScript(TmuxSessionPlan{Name: "wt-feature", KillID: "$4", KillName: "old"}, cfg, windows, map[string]string{})
		assert.Contains(t, script, "kill_id='$4'")
		assert.Contains(t, script, "tmux kill-session -t \"$kill_id\"")
	})

	t.Run("increments the name for on_exists new", func(t *testing.T) {
		cfg := &config.TmuxCommand{OnExists: "new"}
		script := BuildTmuxScript(TmuxSessionPlan{Name: "wt-feature", Increment: true}, cfg, windows, map[string]string{})
		assert.Contains(t, script, "while tmux has-session -t \"$(tmux_exact_target \"${base_session}-$i\")\"")
	})

	t.Run("adds windows to the created session", func(t *testing.T) {
		cfg := &config.TmuxCommand{OnExists: "switch"}
		multi := append(append([]ResolvedWindow{}, windows...), ResolvedWindow{Name: "second", Command: "git status", Cwd: "/path2"})
		script := BuildTmuxScript(TmuxSessionPlan{Name: "s"}, cfg, multi, map[string]string{})
		assert.Contains(t, script, "tmux new-window -t \"$session_id:\" -n 'second' -c '/path2'")
	})

	t.Run("attach mode attaches in place", func(t *testing.T) {
		cfg := &config.TmuxCommand{OnExists: "attach", Attach: true}
		script := BuildTmuxScript(TmuxSessionPlan{Name: "s"}, cfg, windows, map[string]string{})
		assert.Contains(t, script, "tmux attach -t \"$session_id\" || true")
		assert.NotContains(t, script, "switch-client")
	})
}

func TestParseTmuxTargets(t *testing.T) {
	output := "$0\t\twt-legacy\n$1\t/repo/feat\tnice name\twith tab\r\nbroken line\n\n$2\t/repo/other\t\n"
	assert.Equal(t, []Target{
		{ID: "$0", Name: "wt-legacy"},
		{ID: "$1", Key: "/repo/feat", Name: "nice name\twith tab"},
	}, ParseTmuxTargets(output))
}

func TestTmuxWorktreeKey(t *testing.T) {
	assert.Equal(t, "/repo/feat", TmuxWorktreeKey("/repo/./feat/"))
	assert.Empty(t, TmuxWorktreeKey("/repo/fe\tat"))
	assert.Empty(t, TmuxWorktreeKey("/repo/fe\nat"))
}

func TestPlanTmuxSession(t *testing.T) {
	const key = "/repo/feat"
	tests := []struct {
		name     string
		targets  []Target
		onExists string
		legacy   []string
		want     TmuxSessionPlan
		wantErr  error
	}{
		{
			name:     "creates when nothing matches",
			targets:  []Target{{ID: "$0", Name: "other"}},
			onExists: "switch",
			want:     TmuxSessionPlan{Name: "wt-feat", Key: key},
		},
		{
			name:     "reuses the tagged session after a template change",
			targets:  []Target{{ID: "$1", Name: "old-name", Key: key}},
			onExists: "switch",
			want:     TmuxSessionPlan{Name: "old-name", ExistingID: "$1", Key: key},
		},
		{
			name:     "reuses and tags a legacy untagged session by name",
			targets:  []Target{{ID: "$1", Name: "wt-feat"}},
			onExists: "attach",
			want:     TmuxSessionPlan{Name: "wt-feat", ExistingID: "$1", Key: key},
		},
		{
			name:     "refuses a name tagged for another worktree",
			targets:  []Target{{ID: "$1", Name: "wt-feat", Key: "/repo/other"}},
			onExists: "switch",
			wantErr:  ErrTargetConflict,
		},
		{
			name:     "new skips matching",
			targets:  []Target{{ID: "$1", Name: "wt-feat", Key: key}},
			onExists: "new",
			want:     TmuxSessionPlan{Name: "wt-feat", Key: key, Increment: true},
		},
		{
			name:     "kill replaces the tagged session under the template name",
			targets:  []Target{{ID: "$1", Name: "old-name", Key: key}},
			onExists: "kill",
			want:     TmuxSessionPlan{Name: "wt-feat", KillID: "$1", KillName: "old-name", Key: key},
		},
		{
			name: "kill refuses when the template name belongs to another worktree",
			targets: []Target{
				{ID: "$1", Name: "old-name", Key: key},
				{ID: "$2", Name: "wt-feat", Key: "/repo/other"},
			},
			onExists: "kill",
			wantErr:  ErrTargetConflict,
		},
		{
			name: "kill refuses an ambiguous match",
			targets: []Target{
				{ID: "$1", Name: "a", Key: key},
				{ID: "$2", Name: "b", Key: key},
			},
			onExists: "kill",
			wantErr:  ErrAmbiguousTarget,
		},
		{
			name:     "reuses an untagged session under a legacy default name",
			targets:  []Target{{ID: "$1", Name: "wt-feat-old"}},
			onExists: "switch",
			legacy:   []string{"wt:feat-old"},
			want:     TmuxSessionPlan{Name: "wt-feat-old", ExistingID: "$1", Key: key},
		},
		{
			name:     "kill refuses when several legacy names match different sessions",
			targets:  []Target{{ID: "$1", Name: "custom-feat"}, {ID: "$2", Name: "wt-feat-old"}},
			onExists: "kill",
			legacy:   []string{"custom-feat", "wt:feat-old"},
			wantErr:  ErrAmbiguousTarget,
		},
		{
			name:     "reuses an untagged session named with a dot by earlier releases",
			targets:  []Target{{ID: "$2", Name: "fix v1.2"}},
			onExists: "switch",
			legacy:   []string{"fix v1.2"},
			want:     TmuxSessionPlan{Name: "fix v1.2", ExistingID: "$2", Key: key},
		},
		{
			name:     "legacy default names never match a tagged session",
			targets:  []Target{{ID: "$1", Name: "wt-feat-old", Key: "/repo/other"}},
			onExists: "switch",
			legacy:   []string{"wt-feat-old"},
			want:     TmuxSessionPlan{Name: "wt-feat", Key: key},
		},
		{
			name: "switch picks the lexically first of ambiguous matches",
			targets: []Target{
				{ID: "$2", Name: "b", Key: key},
				{ID: "$1", Name: "a", Key: key},
			},
			onExists: "switch",
			want:     TmuxSessionPlan{Name: "a", ExistingID: "$1", Key: key},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PlanTmuxSession(tt.targets, key, "wt-feat", tt.onExists, tt.legacy...)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// isolatedTmux points tmux at a private server for the duration of the test.
func isolatedTmux(t *testing.T) func(args ...string) string {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	dir, err := os.MkdirTemp("/tmp", "lwtmux")
	require.NoError(t, err)
	t.Setenv("TMUX_TMPDIR", dir)
	t.Setenv("TMUX", "")
	t.Cleanup(func() {
		_ = exec.Command("tmux", "kill-server").Run()
		_ = os.RemoveAll(dir)
	})
	return func(args ...string) string {
		out, err := exec.Command("tmux", args...).CombinedOutput() //nolint:gosec
		require.NoError(t, err, string(out))
		return string(out)
	}
}

func planTmux(key, name, onExists string, legacyNames ...string) (TmuxSessionPlan, error) {
	out, _ := exec.Command("tmux", "list-sessions", "-F", TmuxListTargetsFormat).Output()
	return PlanTmuxSession(ParseTmuxTargets(string(out)), key, name, onExists, legacyNames...)
}

func runTmuxPlan(t *testing.T, key, name, onExists string) (string, error) {
	t.Helper()
	plan, err := planTmux(key, name, onExists)
	if err != nil {
		return "", err
	}
	return runPlannedTmux(t, plan, onExists)
}

func runPlannedTmux(t *testing.T, plan TmuxSessionPlan, onExists string) (string, error) {
	t.Helper()
	cfg := &config.TmuxCommand{OnExists: onExists}
	sessionFile := filepath.Join(t.TempDir(), "session")
	script := BuildTmuxScript(plan, cfg, []ResolvedWindow{{Name: "shell", Command: "sleep 600", Cwd: t.TempDir()}}, map[string]string{})
	cmd := exec.Command("bash", "-c", script) //nolint:gosec
	cmd.Env = append(os.Environ(), "LW_TMUX_SESSION_FILE="+sessionFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("%w: %s", err, out)
	}
	return ReadSessionFile(sessionFile, ""), nil
}

func TestTmuxScriptAgainstTmux(t *testing.T) {
	tmux := isolatedTmux(t)
	keyA, keyB := "/repo/it's a $(feature)", "/repo/other"

	got, err := runTmuxPlan(t, keyA, "wt-a", "switch")
	require.NoError(t, err)
	assert.Equal(t, "wt-a", got)

	// A changed template still reuses the tagged session and reports its name.
	got, err = runTmuxPlan(t, keyA, "renamed", "switch")
	require.NoError(t, err)
	assert.Equal(t, "wt-a", got)

	_, err = runTmuxPlan(t, keyB, "taken", "switch")
	require.NoError(t, err)

	// Killing A to recreate it under B's name must leave both sessions alone.
	_, err = runTmuxPlan(t, keyA, "taken", "kill")
	require.ErrorIs(t, err, ErrTargetConflict)
	assert.ElementsMatch(t, []Target{
		{ID: "$0", Name: "wt-a", Key: keyA},
		{ID: "$1", Name: "taken", Key: keyB},
	}, ParseTmuxTargets(tmux("list-sessions", "-F", TmuxListTargetsFormat)))

	// Killing replaces the session under its own name.
	got, err = runTmuxPlan(t, keyB, "taken", "kill")
	require.NoError(t, err)
	assert.Equal(t, "taken", got)
	assert.Equal(t, keyB+"\n", tmux("show-options", "-v", "-t", "=taken:", TmuxWorktreeOption))

	got, err = runTmuxPlan(t, keyA, "wt-a", "new")
	require.NoError(t, err)
	assert.Equal(t, "wt-a-2", got)
	assert.Equal(t, keyA+"\n", tmux("show-options", "-v", "-t", "wt-a-2", TmuxWorktreeOption))

	// Pull request titles keep their "#" instead of being read as tmux
	// formats, and "." becomes "_" on every tmux release, including when
	// incremented.
	title := "#12 Fix #S in v1.2 #{session_name}"
	named := "#12 Fix #S in v1_2 #{session_name}"
	for _, want := range []string{named, named + "-2"} {
		got, err = runTmuxPlan(t, "/repo/pr", title, "new")
		require.NoError(t, err)
		assert.Equal(t, want, got)
	}
}

func TestTmuxScriptRechecksStalePlans(t *testing.T) {
	tmux := isolatedTmux(t)
	tmux("new-session", "-d", "-s", "wt-feat", "sleep 600")

	// Both worktrees plan against the same untagged legacy session before
	// either script runs.
	switchPlan, err := planTmux("/repo/b", "b", "switch", "wt-feat")
	require.NoError(t, err)
	require.Equal(t, "$0", switchPlan.ExistingID)
	killPlan, err := planTmux("/repo/b", "b", "kill", "wt-feat")
	require.NoError(t, err)
	require.Equal(t, "$0", killPlan.KillID)

	got, err := runTmuxPlan(t, "/repo/a", "wt-feat", "switch")
	require.NoError(t, err)
	require.Equal(t, "wt-feat", got)

	_, err = runPlannedTmux(t, switchPlan, "switch")
	require.ErrorContains(t, err, "changed since it was listed")
	_, err = runPlannedTmux(t, killPlan, "kill")
	require.ErrorContains(t, err, "changed since it was listed")
	assert.Equal(t, []Target{{ID: "$0", Name: "wt-feat", Key: "/repo/a"}},
		ParseTmuxTargets(tmux("list-sessions", "-F", TmuxListTargetsFormat)))

	// Another worktree takes the replacement name before a stale kill plan
	// runs, so the session it would replace must survive.
	killPlan, err = planTmux("/repo/a", "replacement", "kill")
	require.NoError(t, err)
	require.Equal(t, "$0", killPlan.KillID)
	_, err = runTmuxPlan(t, "/repo/c", "replacement", "switch")
	require.NoError(t, err)
	_, err = runPlannedTmux(t, killPlan, "kill")
	require.ErrorContains(t, err, "changed since it was listed")
	assert.ElementsMatch(t, []Target{
		{ID: "$0", Name: "wt-feat", Key: "/repo/a"},
		{ID: "$1", Name: "replacement", Key: "/repo/c"},
	}, ParseTmuxTargets(tmux("list-sessions", "-F", TmuxListTargetsFormat)))
}

func TestDefaultSessionName(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{
			name: "pull request number and title",
			env:  map[string]string{"LAZYWORKTREE_NUMBER": "12", "LAZYWORKTREE_TITLE": " Fix  the\nthing ", "WORKTREE_NAME": "pr-12"},
			want: "#12 Fix the thing",
		},
		{
			name: "long title cut at a word boundary",
			env:  map[string]string{"LAZYWORKTREE_NUMBER": "2965", "LAZYWORKTREE_TITLE": "Fix queue concurrency follow-ups from 2890 post-merge review"},
			want: "#2965 Fix queue concurrency follow-ups…",
		},
		{
			name: "long title without spaces cut by character",
			env:  map[string]string{"LAZYWORKTREE_NUMBER": "7", "LAZYWORKTREE_TITLE": strings.Repeat("é", 50)},
			want: "#7 " + strings.Repeat("é", 36) + "…",
		},
		{
			name: "worktree name without a title",
			env:  map[string]string{"LAZYWORKTREE_NUMBER": "12", "WORKTREE_NAME": "pr-12"},
			want: "pr-12",
		},
		{
			name: "path base without env",
			env:  map[string]string{},
			want: "feat",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, DefaultSessionName(tt.env, "/repo/feat"))
		})
	}
}
