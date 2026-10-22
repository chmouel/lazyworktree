package bootstrap

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

func writeFakeHerdrCLI(t *testing.T, script string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake herdr uses a POSIX shell script")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "herdr")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"+script), 0o700); err != nil { // #nosec G306 -- test fixture
		t.Fatalf("write fake herdr: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// fakeHerdrCLIScript answers the Herdr CLI calls executeHerdrAction makes. The
// session and workspace lists come from environment variables so each test can
// describe the daemon state it needs.
const fakeHerdrCLIScript = `
printf '%s\n' "${*:-attach}" >> "$HERDR_TEST_LOG"
if [ "$1" = "--session" ]; then
	shift 2
fi
case "$1" in
	session)
		case "$2" in
			list) printf '{"sessions":%s}\n' "${HERDR_TEST_SESSIONS:-[]}";;
			*) exit 0;;
		esac;;
	workspace)
		case "$2" in
			list) printf '{"result":{"workspaces":%s}}\n' "${HERDR_TEST_WORKSPACES:-[]}";;
			create)
				label=""
				prev=""
				for arg in "$@"; do
					[ "$prev" = "--label" ] && label="$arg"
					prev="$arg"
				done
				printf '{"result":{"workspace":{"workspace_id":"w1","label":"%s"},"tab":{"tab_id":"w1:t1"},"root_pane":{"pane_id":"w1:p1"}}}\n' "$label";;
			*) echo '{}';;
		esac;;
	tab)
		if [ "$2" = "rename" ]; then exit 0; fi
		echo '{"result":{"tab":{"tab_id":"w1:t2"},"root_pane":{"pane_id":"w1:p2"}}}';;
	pane) exit 0;;
	server) exit 0;;
esac
exit 0
`

// fakeHerdrPartialRetryScriptCLI creates the workspace on the first attempt and
// then fails the tab rename, leaving a partially configured workspace behind.
const fakeHerdrPartialRetryScriptCLI = `
printf '%s\n' "${*:-attach}" >> "$HERDR_TEST_LOG"
if [ "$1" = "--session" ]; then
	shift 2
fi
case "$1" in
	session)
		case "$2" in
			list) printf '{"sessions":%s}\n' "${HERDR_TEST_SESSIONS:-[]}";;
			*) exit 0;;
		esac;;
	workspace)
		case "$2" in
			list)
				if [ -f "$HERDR_TEST_LOG.created" ]; then
					echo '{"result":{"workspaces":[{"workspace_id":"w1","label":"feat"}]}}'
				else
					echo '{"result":{"workspaces":[]}}'
				fi;;
			create)
				: > "$HERDR_TEST_LOG.created"
				echo '{"result":{"workspace":{"workspace_id":"w1","label":"feat"},"tab":{"tab_id":"w1:t1"},"root_pane":{"pane_id":"w1:p1"}}}';;
			*) echo '{}';;
		esac;;
	tab)
		if [ -f "$HERDR_TEST_LOG.fail" ]; then exit 1; fi
		exit 0;;
	pane|server) exit 0;;
esac
exit 0
`

func readHerdrCLILog(t *testing.T, logPath string) string {
	t.Helper()
	logBytes, err := os.ReadFile(logPath) // #nosec G304 -- test log path
	if err != nil {
		t.Fatalf("read herdr log: %v", err)
	}
	return string(logBytes)
}

func herdrLogIndex(t *testing.T, log, needle string) int {
	t.Helper()
	index := strings.Index(log, needle)
	if index < 0 {
		t.Fatalf("expected %q in the herdr log, got:\n%s", needle, log)
	}
	return index
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}
	os.Stderr = writer
	fn()
	_ = writer.Close()
	os.Stderr = original
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	return string(out)
}

func herdrActionWorktree(t *testing.T) (*models.WorktreeInfo, *config.AppConfig) {
	t.Helper()
	return &models.WorktreeInfo{Path: "/w/feat", Branch: "feat"}, &config.AppConfig{WorktreeDir: t.TempDir()}
}

func TestExecuteHerdrActionFocusesReusedWorkspaceBeforeAttach(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "herdr.log")
	t.Setenv("HERDR_TEST_LOG", logPath)
	t.Setenv("HERDR_SESSION", "")
	t.Setenv("HERDR_SOCKET_PATH", "")
	t.Setenv("HERDR_TEST_SESSIONS", `[{"name":"default","default":true,"running":true}]`)
	t.Setenv("HERDR_TEST_WORKSPACES", `[{"workspace_id":"w9","label":"wt-feat"}]`)
	writeFakeHerdrCLI(t, fakeHerdrCLIScript)

	wt, cfg := herdrActionWorktree(t)
	err := executeHerdrAction(context.Background(), &config.HerdrCommand{
		Mode:        config.HerdrModeWorkspace,
		SessionName: "wt-feat",
		OnExists:    "switch",
		Attach:      true,
		Windows:     []config.TmuxWindow{{Name: "shell"}},
	}, nil, cfg, wt, map[string]string{})
	if err != nil {
		t.Fatalf("executeHerdrAction: %v", err)
	}

	log := readHerdrCLILog(t, logPath)
	if strings.Contains(log, "workspace create") {
		t.Fatalf("expected the existing workspace to be reused, got:\n%s", log)
	}
	focusIndex := herdrLogIndex(t, log, "workspace focus w9")
	attachIndex := herdrLogIndex(t, log, "attach")
	if focusIndex > attachIndex {
		t.Errorf("expected focus before attach, got:\n%s", log)
	}
}

func TestExecuteHerdrActionAttachFalseSkipsFocusAndAttach(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "herdr.log")
	t.Setenv("HERDR_TEST_LOG", logPath)
	t.Setenv("HERDR_SESSION", "")
	t.Setenv("HERDR_SOCKET_PATH", "")
	t.Setenv("HERDR_TEST_SESSIONS", `[{"name":"default","default":true,"running":true}]`)
	t.Setenv("HERDR_TEST_WORKSPACES", `[{"workspace_id":"w9","label":"wt-feat"}]`)
	writeFakeHerdrCLI(t, fakeHerdrCLIScript)

	wt, cfg := herdrActionWorktree(t)
	var runErr error
	stderr := captureStderr(t, func() {
		runErr = executeHerdrAction(context.Background(), &config.HerdrCommand{
			Mode:        config.HerdrModeWorkspace,
			SessionName: "wt-feat",
			OnExists:    "switch",
			Attach:      false,
			Windows:     []config.TmuxWindow{{Name: "shell"}},
		}, nil, cfg, wt, map[string]string{})
	})
	if runErr != nil {
		t.Fatalf("executeHerdrAction: %v", runErr)
	}
	if !strings.Contains(stderr, `Herdr workspace "wt-feat" is ready`) {
		t.Errorf("stderr = %q, want the workspace ready instructions", stderr)
	}
	log := readHerdrCLILog(t, logPath)
	if strings.Contains(log, "workspace focus") {
		t.Errorf("expected no focus when attaching is disabled, got:\n%s", log)
	}
	if strings.Contains(log, "attach") {
		t.Errorf("expected no attach when attaching is disabled, got:\n%s", log)
	}
}

func TestExecuteHerdrActionNestedSessionPrintsInstructions(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "herdr.log")
	t.Setenv("HERDR_TEST_LOG", logPath)
	t.Setenv("HERDR_SESSION", "outer")
	t.Setenv("HERDR_TEST_SESSIONS", `[{"name":"wt-feat","running":true}]`)
	t.Setenv("HERDR_TEST_WORKSPACES", `[{"workspace_id":"w1","label":"feat"}]`)
	writeFakeHerdrCLI(t, fakeHerdrCLIScript)

	wt, cfg := herdrActionWorktree(t)
	var runErr error
	stderr := captureStderr(t, func() {
		runErr = executeHerdrAction(context.Background(), &config.HerdrCommand{
			Mode:        config.HerdrModeSession,
			SessionName: "wt-feat",
			OnExists:    "switch",
			Attach:      true,
			Windows:     []config.TmuxWindow{{Name: "shell"}},
		}, nil, cfg, wt, map[string]string{})
	})
	if runErr != nil {
		t.Fatalf("executeHerdrAction: %v", runErr)
	}
	if !strings.Contains(stderr, "Attach from another terminal") {
		t.Errorf("stderr = %q, want nested attach instructions", stderr)
	}
	log := readHerdrCLILog(t, logPath)
	if strings.Contains(log, "attach") {
		t.Errorf("expected no attach from inside Herdr, got:\n%s", log)
	}
}

func TestExecuteHerdrActionSessionModeCreatesMissingWorkspace(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "herdr.log")
	t.Setenv("HERDR_TEST_LOG", logPath)
	t.Setenv("HERDR_SESSION", "")
	t.Setenv("HERDR_SOCKET_PATH", "")
	t.Setenv("HERDR_TEST_SESSIONS", `[{"name":"wt-feat","running":true}]`)
	t.Setenv("HERDR_TEST_WORKSPACES", `[]`)
	writeFakeHerdrCLI(t, fakeHerdrCLIScript)

	wt, cfg := herdrActionWorktree(t)
	err := executeHerdrAction(context.Background(), &config.HerdrCommand{
		Mode:        config.HerdrModeSession,
		SessionName: "wt-feat",
		OnExists:    "switch",
		Attach:      false,
		Windows:     []config.TmuxWindow{{Name: "shell"}},
	}, nil, cfg, wt, map[string]string{})
	if err != nil {
		t.Fatalf("executeHerdrAction: %v", err)
	}
	log := readHerdrCLILog(t, logPath)
	if !strings.Contains(log, "--session wt-feat workspace create --label feat") {
		t.Errorf("expected the missing workspace layout to be created, got:\n%s", log)
	}
}

func TestExecuteHerdrActionSessionOnExistsNewRespectsNameLimit(t *testing.T) {
	t.Setenv("HERDR_SESSION", "")
	t.Setenv("HERDR_SOCKET_PATH", "")
	for _, size := range []int{63, 64} {
		t.Run(fmt.Sprintf("%d-byte base", size), func(t *testing.T) {
			logPath := filepath.Join(t.TempDir(), "herdr.log")
			t.Setenv("HERDR_TEST_LOG", logPath)
			base := strings.Repeat("a", size)
			stem := base[:62]
			t.Setenv("HERDR_TEST_SESSIONS", `[{"name":"`+base+`","running":true},{"name":"`+stem+`-2","running":true}]`)
			t.Setenv("HERDR_TEST_WORKSPACES", `[]`)
			writeFakeHerdrCLI(t, fakeHerdrCLIScript)

			wt, cfg := herdrActionWorktree(t)
			err := executeHerdrAction(context.Background(), &config.HerdrCommand{
				Mode:        config.HerdrModeSession,
				SessionName: base,
				OnExists:    "new",
				Attach:      false,
				Windows:     []config.TmuxWindow{{Name: "shell"}},
			}, nil, cfg, wt, map[string]string{})
			if err != nil {
				t.Fatalf("executeHerdrAction: %v", err)
			}
			log := readHerdrCLILog(t, logPath)
			want := "--session " + stem + "-3 workspace create"
			if !strings.Contains(log, want) {
				t.Errorf("expected %q in the log, got:\n%s", want, log)
			}
		})
	}
}

func TestExecuteHerdrActionRejectsInvalidConfiguredNames(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.HerdrCommand
		wantErr string
	}{
		{
			name:    "session mode dot-dot session name",
			cfg:     &config.HerdrCommand{Mode: config.HerdrModeSession, SessionName: "..", OnExists: "kill", Attach: true},
			wantErr: "session_name",
		},
		{
			name:    "session mode slash session name",
			cfg:     &config.HerdrCommand{Mode: config.HerdrModeSession, SessionName: "/", Attach: true},
			wantErr: "session_name",
		},
		{
			name:    "workspace mode dot-dot session",
			cfg:     &config.HerdrCommand{Mode: config.HerdrModeWorkspace, HerdrSession: "..", Attach: true},
			wantErr: "herdr_session",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logPath := filepath.Join(t.TempDir(), "herdr.log")
			t.Setenv("HERDR_TEST_LOG", logPath)
			t.Setenv("HERDR_SESSION", "")
			t.Setenv("HERDR_TEST_SESSIONS", `[{"name":"..","running":true}]`)
			writeFakeHerdrCLI(t, fakeHerdrCLIScript)

			tt.cfg.Windows = []config.TmuxWindow{{Name: "shell"}}
			wt, cfg := herdrActionWorktree(t)
			err := executeHerdrAction(context.Background(), tt.cfg, nil, cfg, wt, map[string]string{})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want a %s validation error", err, tt.wantErr)
			}
			if _, statErr := os.Stat(logPath); statErr == nil {
				t.Errorf("expected no herdr command to run, got:\n%s", readHerdrCLILog(t, logPath))
			}
		})
	}
}

func TestExecuteHerdrActionRejectsDerivedEmptyWorkspaceLabel(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "herdr.log")
	t.Setenv("HERDR_TEST_LOG", logPath)
	t.Setenv("HERDR_SESSION", "")
	writeFakeHerdrCLI(t, fakeHerdrCLIScript)

	wt := &models.WorktreeInfo{Path: "/w/+", Branch: "feat"}
	err := executeHerdrAction(context.Background(), &config.HerdrCommand{
		Mode:        config.HerdrModeSession,
		SessionName: "...",
		Attach:      true,
		Windows:     []config.TmuxWindow{{Name: "shell"}},
	}, nil, &config.AppConfig{WorktreeDir: t.TempDir()}, wt, map[string]string{})
	if err == nil || !strings.Contains(err.Error(), "workspace label") {
		t.Fatalf("error = %v, want a workspace label validation error", err)
	}
	if _, statErr := os.Stat(logPath); statErr == nil {
		t.Errorf("expected no herdr command to run, got:\n%s", readHerdrCLILog(t, logPath))
	}
}

func TestExecuteHerdrActionAcceptsDotOnlyValidName(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "herdr.log")
	t.Setenv("HERDR_TEST_LOG", logPath)
	t.Setenv("HERDR_SESSION", "")
	t.Setenv("HERDR_SOCKET_PATH", "")
	t.Setenv("HERDR_TEST_SESSIONS", `[]`)
	writeFakeHerdrCLI(t, fakeHerdrCLIScript)

	wt, cfg := herdrActionWorktree(t)
	err := executeHerdrAction(context.Background(), &config.HerdrCommand{
		Mode:        config.HerdrModeSession,
		SessionName: "...",
		Attach:      false,
		Windows:     []config.TmuxWindow{{Name: "shell"}},
	}, nil, cfg, wt, map[string]string{})
	if err != nil {
		t.Fatalf("executeHerdrAction: %v", err)
	}
	if log := readHerdrCLILog(t, logPath); !strings.Contains(log, "--session ... workspace create") {
		t.Errorf("expected the dot-only name to be used, got:\n%s", log)
	}
}

func TestExecuteHerdrActionRetriesAfterPartialFailure(t *testing.T) {
	for _, mode := range []string{config.HerdrModeWorkspace, config.HerdrModeSession} {
		t.Run(mode, func(t *testing.T) {
			logPath := filepath.Join(t.TempDir(), "herdr.log")
			t.Setenv("HERDR_TEST_LOG", logPath)
			t.Setenv("HERDR_SESSION", "")
			t.Setenv("HERDR_SOCKET_PATH", "")
			t.Setenv("HERDR_TEST_SESSIONS", `[{"name":"feat","running":true}]`)
			writeFakeHerdrCLI(t, fakeHerdrPartialRetryScriptCLI)

			failMarker := logPath + ".fail"
			if err := os.WriteFile(failMarker, nil, 0o600); err != nil {
				t.Fatalf("write fail marker: %v", err)
			}

			wt, cfg := herdrActionWorktree(t)
			herdrCfg := &config.HerdrCommand{
				Mode:        mode,
				SessionName: "feat",
				OnExists:    "switch",
				Attach:      false,
				Windows:     []config.TmuxWindow{{Name: "shell"}},
			}
			firstErr := executeHerdrAction(context.Background(), herdrCfg, nil, cfg, wt, map[string]string{})
			if firstErr == nil || !strings.Contains(firstErr.Error(), "is partially configured") {
				t.Fatalf("first attempt error = %v, want a partial configuration error", firstErr)
			}

			if err := os.Remove(failMarker); err != nil {
				t.Fatalf("remove fail marker: %v", err)
			}
			if err := executeHerdrAction(context.Background(), herdrCfg, nil, cfg, wt, map[string]string{}); err != nil {
				t.Fatalf("retry: %v", err)
			}
			log := readHerdrCLILog(t, logPath)
			if got := strings.Count(log, "workspace create"); got != 1 {
				t.Errorf("workspace create ran %d times, want 1:\n%s", got, log)
			}
		})
	}
}
