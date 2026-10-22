package bootstrap

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/stretchr/testify/require"
)

func TestExecuteKeyActionTmuxHonoursAttach(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake tmux uses a POSIX shell script")
	}
	dir := t.TempDir()
	// #nosec G306 -- test fixture
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tmux"), []byte("#!/bin/sh\nexit 0\n"), 0o700))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	orig := runTmuxScriptFunc
	t.Cleanup(func() { runTmuxScriptFunc = orig })

	scriptErr := errors.New("tmux script failed")
	for _, tc := range []struct {
		name   string
		attach bool
		err    error
	}{
		{name: "attach", attach: true},
		{name: "detached"},
		{name: "failed", err: scriptErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var script string
			runTmuxScriptFunc = func(_ context.Context, s, _ string, env map[string]string) error {
				script = s
				require.NoError(t, os.WriteFile(env["LW_TMUX_SESSION_FILE"], []byte("wt-feat's session.2-2"), 0o600))
				return tc.err
			}

			wt := &models.WorktreeInfo{Path: t.TempDir(), Branch: "feat"}
			cfg := &config.AppConfig{
				WorktreeDir: t.TempDir(),
				CustomCommands: config.CustomCommandsConfig{config.PaneUniversal: {"k": {
					Tmux: &config.TmuxCommand{
						SessionName: "wt-feat",
						OnExists:    "switch",
						Attach:      tc.attach,
						Windows:     []config.TmuxWindow{{Name: "shell"}},
					},
				}}},
			}

			stderr := captureStderr(t, func() {
				err := executeKeyAction(context.Background(), "k", cfg, wt, map[string]string{})
				if tc.err != nil {
					require.ErrorIs(t, err, tc.err)
				} else {
					require.NoError(t, err)
				}
			})
			if tc.attach {
				require.Contains(t, script, `tmux attach -t "$session_id"`)
			} else {
				require.NotContains(t, script, `tmux attach -t "$session_id"`)
				require.NotContains(t, script, `tmux switch-client -t "$session_id"`)
			}
			if !tc.attach && tc.err == nil {
				require.Equal(t, "tmux session \"wt-feat's session.2-2\" is ready. Attach with: tmux attach-session -t '=wt-feat'\"'\"'s session.2-2:'\n", stderr)
			} else {
				require.Empty(t, stderr)
			}
		})
	}
}

func TestExecuteZellijActionHonoursAttach(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake zellij uses a POSIX shell script")
	}
	binDir := t.TempDir()
	fakeZellij := `#!/bin/sh
echo "$*" >> "$LW_TEST_ZELLIJ_LOG"
if [ "$1" = "list-sessions" ]; then
  if [ -f "$LW_TEST_ZELLIJ_STATE" ]; then cat "$LW_TEST_ZELLIJ_STATE"; fi
  exit 0
fi
if [ "$1" = "attach" ] && [ "$2" = "--create-background" ]; then
  if [ -n "$LW_TEST_ZELLIJ_FAIL" ]; then exit 1; fi
  printf '%s [Created now]\n' "$3" > "$LW_TEST_ZELLIJ_STATE"
  exit 0
fi
exit 0
`
	// #nosec G306 -- test fixture
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "zellij"), []byte(fakeZellij), 0o700))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	for _, tc := range []struct {
		name   string
		attach bool
		fail   bool
	}{
		{name: "attach", attach: true},
		{name: "detached"},
		{name: "failed", fail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logPath := filepath.Join(t.TempDir(), "zellij.log")
			t.Setenv("LW_TEST_ZELLIJ_LOG", logPath)
			statePath := filepath.Join(t.TempDir(), "state")
			t.Setenv("LW_TEST_ZELLIJ_STATE", statePath)
			t.Setenv("LW_TEST_ZELLIJ_FAIL", "")
			if tc.fail {
				t.Setenv("LW_TEST_ZELLIJ_FAIL", "1")
			}
			require.NoError(t, os.WriteFile(statePath, []byte("wt-feature's session [Created now]\n"), 0o600))

			wt := &models.WorktreeInfo{Path: t.TempDir(), Branch: "feat"}
			cfg := &config.AppConfig{WorktreeDir: t.TempDir(), SessionPrefix: "wt-"}
			zellijCfg := &config.TmuxCommand{
				SessionName: "wt-feature's session",
				OnExists:    "new",
				Attach:      tc.attach,
				Windows:     []config.TmuxWindow{{Name: "shell"}},
			}

			stderr := captureStderr(t, func() {
				err := executeZellijAction(context.Background(), zellijCfg, nil, cfg, wt, map[string]string{})
				if tc.fail {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			})

			logContent, err := os.ReadFile(logPath) //nolint:gosec
			require.NoError(t, err)
			log := string(logContent)
			require.Contains(t, log, "attach --create-background wt-feature's session-2")
			if tc.attach {
				require.Contains(t, log, "attach wt-feature's session-2")
			} else {
				require.NotContains(t, log, "attach wt-feature's session-2",
					"did not expect an interactive attach when attach is false, got:\n%s", log)
			}
			if !tc.attach && !tc.fail {
				require.Equal(t, "zellij session \"wt-feature's session-2\" is ready. Attach with: zellij attach 'wt-feature'\"'\"'s session-2'\n", stderr)
			} else {
				require.Empty(t, stderr)
			}
		})
	}
}
