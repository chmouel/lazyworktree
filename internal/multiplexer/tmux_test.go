package multiplexer

import (
	"os"
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
	t.Run("empty windows", func(t *testing.T) {
		cfg := &config.TmuxCommand{OnExists: "switch", Attach: false}
		script := BuildTmuxScript("test-session", cfg, []ResolvedWindow{}, map[string]string{})
		assert.Empty(t, script)
	})

	t.Run("basic script with one window", func(t *testing.T) {
		cfg := &config.TmuxCommand{OnExists: "switch", Attach: false}
		windows := []ResolvedWindow{
			{Name: "main", Command: "vim", Cwd: "/path"},
		}
		script := BuildTmuxScript("test-session", cfg, windows, map[string]string{})
		assert.Contains(t, script, "test-session")
		assert.Contains(t, script, "tmux_exact_target() { printf '=%s' \"$1\"; }")
		assert.Contains(t, script, "tmux has-session -t \"$(tmux_exact_target \"$session\")\"")
		assert.Contains(t, script, "tmux new-session")
		assert.Contains(t, script, "tmux new-session -d -s \"$session\"")
		assert.Contains(t, script, "main")
		assert.Contains(t, script, "vim")
		assert.NotContains(t, script, "tmux attach")
	})

	t.Run("script with attach", func(t *testing.T) {
		cfg := &config.TmuxCommand{OnExists: "switch", Attach: true}
		windows := []ResolvedWindow{
			{Name: "main", Command: "vim", Cwd: "/path"},
		}
		script := BuildTmuxScript("test-session", cfg, windows, map[string]string{})
		assert.Contains(t, script, "tmux attach -t \"$(tmux_exact_target \"$session\")\" || true")
		assert.Contains(t, script, "tmux switch-client -t \"$(tmux_exact_target \"$session\")\" || true")
	})

	t.Run("script with multiple windows", func(t *testing.T) {
		cfg := &config.TmuxCommand{OnExists: "switch", Attach: false}
		windows := []ResolvedWindow{
			{Name: "first", Command: "vim", Cwd: "/path1"},
			{Name: "second", Command: "git status", Cwd: "/path2"},
		}
		script := BuildTmuxScript("test-session", cfg, windows, map[string]string{})
		assert.Contains(t, script, "first")
		assert.Contains(t, script, "second")
		assert.Contains(t, script, "tmux new-window")
	})

	t.Run("script with environment variables", func(t *testing.T) {
		cfg := &config.TmuxCommand{OnExists: "switch", Attach: false}
		windows := []ResolvedWindow{
			{Name: "main", Command: "vim", Cwd: "/path"},
		}
		env := map[string]string{"FOO": "bar", "BAZ": "qux"}
		script := BuildTmuxScript("test-session", cfg, windows, env)
		assert.Contains(t, script, "tmux set-environment")
		assert.Contains(t, script, "FOO")
		assert.Contains(t, script, "BAZ")
	})

	t.Run("script with onExists kill", func(t *testing.T) {
		cfg := &config.TmuxCommand{OnExists: "kill", Attach: false}
		windows := []ResolvedWindow{
			{Name: "main", Command: "vim", Cwd: "/path"},
		}
		script := BuildTmuxScript("test-session", cfg, windows, map[string]string{})
		assert.Contains(t, script, "tmux kill-session -t \"$(tmux_exact_target \"$session\")\"")
	})

	t.Run("script with onExists new", func(t *testing.T) {
		cfg := &config.TmuxCommand{OnExists: "new", Attach: false}
		windows := []ResolvedWindow{
			{Name: "main", Command: "vim", Cwd: "/path"},
		}
		script := BuildTmuxScript("test-session", cfg, windows, map[string]string{})
		assert.Contains(t, script, "base_session")
		assert.Contains(t, script, "while tmux has-session -t \"$(tmux_exact_target \"${base_session}-$i\")\"")
	})

	t.Run("script with session file", func(t *testing.T) {
		cfg := &config.TmuxCommand{OnExists: "switch", Attach: false}
		windows := []ResolvedWindow{
			{Name: "main", Command: "vim", Cwd: "/path"},
		}
		script := BuildTmuxScript("test-session", cfg, windows, map[string]string{})
		assert.Contains(t, script, "LW_TMUX_SESSION_FILE")
	})

	t.Run("script uses exact match for prefixed session names", func(t *testing.T) {
		cfg := &config.TmuxCommand{OnExists: "switch", Attach: true}
		windows := []ResolvedWindow{
			{Name: "main", Command: "vim", Cwd: "/path"},
		}
		script := BuildTmuxScript("wt-feature-foo-bar", cfg, windows, map[string]string{})
		assert.Contains(t, script, "session='wt-feature-foo-bar'")
		assert.Contains(t, script, "\"$(tmux_exact_target \"$session\")\"")
		assert.NotContains(t, script, "switch-client -t \"$session\"")
	})
}
