package multiplexer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/chmouel/lazyworktree/internal/config"
)

func TestSanitizeHerdrSessionName(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"wt-feature", "wt-feature"},
		{"wt/feature x", "wt-feature-x"},
		{"wt:branch", "wt-branch"},
		{"...wt...", "...wt..."},
		{".", ""},
		{"..", ""},
		{"héllo", "h-llo"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := SanitizeHerdrSessionName(tt.in); got != tt.want {
			t.Errorf("SanitizeHerdrSessionName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}

	long := strings.Repeat("a", 80)
	if got := SanitizeHerdrSessionName(long); len(got) != herdrMaxSessionNameLen {
		t.Errorf("long name length = %d, want %d", len(got), herdrMaxSessionNameLen)
	}
}

func TestSanitizeHerdrPrefix(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"wt-", "wt-"},
		{"wt:", "wt-"},
		{"wt/feature", "wt-feature"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := SanitizeHerdrPrefix(tt.in); got != tt.want {
			t.Errorf("SanitizeHerdrPrefix(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestHerdrResolvedName(t *testing.T) {
	tests := []struct {
		configured, fallback, want string
		wantErr                    bool
	}{
		{"wt-feature", "wt-fallback", "wt-feature", false},
		{"wt:feature", "wt-fallback", "wt-feature", false},
		{"", "wt-fallback", "wt-fallback", false},
		{"wt/feature x", "wt-fallback", "wt-feature-x", false},
		{"///", "wt-fallback", "", true},
		{"", "///", "", true},
	}
	for _, tt := range tests {
		got, err := HerdrResolvedName("session_name", tt.configured, tt.fallback)
		if tt.wantErr {
			if err == nil {
				t.Errorf("HerdrResolvedName(%q, %q) = %q, want an error", tt.configured, tt.fallback, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("HerdrResolvedName(%q, %q): %v", tt.configured, tt.fallback, err)
			continue
		}
		if got != tt.want {
			t.Errorf("HerdrResolvedName(%q, %q) = %q, want %q", tt.configured, tt.fallback, got, tt.want)
		}
	}
}

func TestBuildHerdrEnvArgsSortsKeys(t *testing.T) {
	args := BuildHerdrEnvArgs(map[string]string{"WORKTREE_PATH": "/w", "AO": "1", "ZO": "2"})
	want := []string{"--env", "AO=1", "--env", "WORKTREE_PATH=/w", "--env", "ZO=2"}
	if strings.Join(args, " ") != strings.Join(want, " ") {
		t.Errorf("env args = %v, want %v", args, want)
	}
}

func TestResolveHerdrWindowsExpandsAndDefaults(t *testing.T) {
	env := map[string]string{"WORKTREE_NAME": "feature", "WORKTREE_PATH": "/w/feature"}
	windows := ResolveHerdrWindows([]config.TmuxWindow{
		{Name: "$WORKTREE_NAME", Command: "echo $WORKTREE_NAME"},
		{Cwd: "$WORKTREE_PATH"},
	}, env, "/fallback")

	if len(windows) != 2 {
		t.Fatalf("got %d windows, want 2", len(windows))
	}
	if windows[0].Name != "feature" || windows[0].Cwd != "/fallback" {
		t.Errorf("unexpected first window: %+v", windows[0])
	}
	if windows[0].Command != "echo $WORKTREE_NAME" {
		t.Errorf("command should not be expanded, got %q", windows[0].Command)
	}
	if windows[1].Name != "window-2" || windows[1].Cwd != "/w/feature" {
		t.Errorf("unexpected second window: %+v", windows[1])
	}
	if windows[1].Command != "" {
		t.Errorf("a tab without a command must keep the native pane shell, got %q", windows[1].Command)
	}
}

func TestDefaultHerdrContainerCommandsOnlyFillsEmptyCommands(t *testing.T) {
	windows := ResolveHerdrWindows([]config.TmuxWindow{
		{Name: "shell"},
		{Name: "test", Command: "go test ./..."},
	}, map[string]string{}, "/w/feature")

	filled := DefaultHerdrContainerCommands(windows)
	if filled[0].Command != containerShellCommand {
		t.Errorf("commandless tab = %q, want the container shell", filled[0].Command)
	}
	if filled[1].Command != "go test ./..." {
		t.Errorf("configured command was overwritten: %q", filled[1].Command)
	}
	if windows[0].Command != "" {
		t.Error("DefaultHerdrContainerCommands must not mutate its input")
	}
}

func TestResolveHerdrWindowsContainerWrapsShellTabs(t *testing.T) {
	windows := DefaultHerdrContainerCommands(ResolveHerdrWindows([]config.TmuxWindow{
		{Name: "shell"},
		{Name: "test", Command: "go test ./..."},
	}, map[string]string{}, "/w/feature"))

	wrapped, err := WrapWindowCommandsForContainer(windows, &config.ContainerCommand{Image: "alpine", Runtime: "echo"}, map[string]string{})
	if err != nil {
		t.Fatalf("wrap: %v", err)
	}
	for _, window := range wrapped {
		if strings.TrimSpace(window.Command) == "" {
			t.Errorf("window %q was left outside the container", window.Name)
		}
		if !strings.Contains(window.Command, "echo") {
			t.Errorf("window %q is not wrapped in the container runtime: %q", window.Name, window.Command)
		}
	}
}

func TestHerdrSessionIsAmbient(t *testing.T) {
	tests := []struct {
		name    string
		ambient string
		socket  string
		session string
		want    bool
	}{
		{name: "empty target inherits the ambient session", ambient: "work", want: true},
		{name: "matching name", ambient: "work", session: "work", want: true},
		{name: "different name", ambient: "work", session: "other", want: false},
		{name: "matching name on a custom socket", ambient: "work", socket: "/tmp/custom.sock", session: "work", want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HERDR_SESSION", tc.ambient)
			t.Setenv("HERDR_SOCKET_PATH", tc.socket)
			if got := HerdrSessionIsAmbient(tc.session); got != tc.want {
				t.Errorf("HerdrSessionIsAmbient(%q) = %v, want %v", tc.session, got, tc.want)
			}
		})
	}
}

func TestHerdrAttachArgs(t *testing.T) {
	tests := []struct {
		session string
		want    string
	}{
		{"", "herdr"},
		{"default", "herdr session attach default"},
		{"wt-feature", "herdr session attach wt-feature"},
	}
	for _, tt := range tests {
		h := &Herdr{Bin: "herdr", Session: tt.session}
		if got := h.InfoCommand(); got != tt.want {
			t.Errorf("InfoCommand() for %q = %q, want %q", tt.session, got, tt.want)
		}
	}
}

func TestHerdrArgsRouteAmbientSessionThroughItsSocket(t *testing.T) {
	t.Setenv("HERDR_SESSION", "")
	t.Setenv("HERDR_SOCKET_PATH", "/tmp/custom-herdr.sock")
	if got := (&Herdr{Session: "default"}).args("workspace", "list"); strings.Join(got, " ") != "workspace list" {
		t.Errorf("explicit default args = %v, want ambient routing without --session", got)
	}

	t.Setenv("HERDR_SESSION", "work")
	if got := (&Herdr{Session: "default"}).args("workspace", "list"); strings.Join(got, " ") != "--session default workspace list" {
		t.Errorf("explicit default args = %v, want --session default workspace list", got)
	}
	if got := (&Herdr{Session: "work"}).args("workspace", "list"); strings.Join(got, " ") != "workspace list" {
		t.Errorf("ambient session args = %v, want workspace list", got)
	}

	t.Setenv("HERDR_SOCKET_PATH", "")
	if got := (&Herdr{Session: "default"}).args("workspace", "list"); strings.Join(got, " ") != "--session default workspace list" {
		t.Errorf("explicit default args without a client socket = %v, want --session default workspace list", got)
	}
	if got := (&Herdr{Session: "work"}).args("workspace", "list"); strings.Join(got, " ") != "--session work workspace list" {
		t.Errorf("explicit ambient session without a client socket = %v, want --session work workspace list", got)
	}
	if got := (&Herdr{}).args("workspace", "list"); strings.Join(got, " ") != "workspace list" {
		t.Errorf("inherited args = %v, want workspace list", got)
	}
}

func TestHerdrDisplaySessionResolvesAmbientSession(t *testing.T) {
	t.Setenv("HERDR_SESSION", "")
	if got := (&Herdr{}).displaySession(); got != herdrDefaultSession {
		t.Errorf("displaySession() = %q, want %q", got, herdrDefaultSession)
	}
	t.Setenv("HERDR_SESSION", "work")
	if got := (&Herdr{}).displaySession(); got != "work" {
		t.Errorf("displaySession() = %q, want work", got)
	}
	if got := (&Herdr{Session: "explicit"}).displaySession(); got != "explicit" {
		t.Errorf("explicit displaySession() = %q, want explicit", got)
	}
}

func writeFakeHerdr(t *testing.T, script string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake herdr uses a POSIX shell script")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "herdr")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"+script), 0o700); err != nil { // #nosec G306 -- test fixture
		t.Fatalf("write fake herdr: %v", err)
	}
	return bin
}

const fakeHerdrScript = `
printf '%s\n' "$*" >> "$HERDR_TEST_LOG"
while [ $# -gt 0 ]; do
	case "$1" in
		--session) shift 2; continue;;
		workspace)
			shift
			case "$1" in
				list) echo '{"result":{"workspaces":[]}}';;
				create) echo '{"result":{"workspace":{"workspace_id":"w1","label":"feat"},"tab":{"tab_id":"w1:t1","label":"1"},"root_pane":{"pane_id":"w1:p1"}}}';;
				focus|close) echo '{}';;
			esac
			exit 0;;
		tab)
			if [ "$2" = "rename" ]; then exit 0; fi
			echo '{"result":{"tab":{"tab_id":"w1:t2"},"root_pane":{"pane_id":"w1:p2"}}}'
			exit 0;;
		pane) exit 0;;
		session)
			shift
			case "$1" in
				list) echo '{"sessions":[{"name":"default","default":true,"running":true}]}';;
				*) exit 0;;
			esac
			exit 0;;
		server) sleep 2; exit 0;;
	esac
	shift
done
`

func TestHerdrCreateWorkspaceRunsTabs(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "herdr.log")
	t.Setenv("HERDR_TEST_LOG", logPath)
	bin := writeFakeHerdr(t, fakeHerdrScript)

	h := &Herdr{Bin: bin, Session: "wt-feat"}
	windows := []ResolvedWindow{
		{Name: "editor", Command: "vim", Cwd: "/w/first"},
		{Name: "tests", Command: "go test ./...", Cwd: "/w"},
	}
	workspace, err := h.CreateWorkspace(context.Background(), "feat", "v1:abc", "/w", windows, map[string]string{"A": "1"}, true)
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}
	if workspace.ID != "w1" {
		t.Errorf("workspace id = %q, want w1", workspace.ID)
	}

	logBytes, err := os.ReadFile(logPath) // #nosec G304 -- test log path
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	log := string(logBytes)
	wants := []string{
		"--session wt-feat workspace create --label feat --cwd /w/first --env A=1 --focus",
		"--session wt-feat workspace report-metadata w1 --source lazyworktree --token lazyworktree_id=v1:abc",
		"--session wt-feat tab rename w1:t1 editor",
		"--session wt-feat pane run w1:p1 vim",
		"--session wt-feat tab create --workspace w1 --label tests --cwd /w --env A=1 --no-focus",
		"--session wt-feat pane run w1:p2 go test ./...",
	}
	for _, want := range wants {
		if !strings.Contains(log, want) {
			t.Errorf("expected log to contain %q, got:\n%s", want, log)
		}
	}
	if strings.Index(log, "report-metadata") > strings.Index(log, "tab rename") {
		t.Errorf("expected the workspace to be tagged before its layout, got:\n%s", log)
	}
}

func TestHerdrCreateWorkspaceTagFailureKeepsWorkspace(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "herdr.log")
	t.Setenv("HERDR_SESSION", "")
	t.Setenv("HERDR_TEST_LOG", logPath)
	bin := writeFakeHerdr(t, strings.Replace(fakeHerdrScript,
		"focus|close) echo '{}';;",
		"focus|close) echo '{}';;\n\t\t\t\treport-metadata) echo 'unknown token' >&2; exit 1;;", 1))

	h := &Herdr{Bin: bin}
	workspace, err := h.CreateWorkspace(context.Background(), "feat", "v1:abc", "/w", []ResolvedWindow{{Name: "shell"}}, nil, true)
	if err == nil || !strings.Contains(err.Error(), "could not be tagged") || !strings.Contains(err.Error(), "w1") {
		t.Fatalf("error = %v, want a tagging error naming the workspace", err)
	}
	if workspace.ID != "w1" {
		t.Errorf("workspace = %+v, want the created workspace", workspace)
	}
	logBytes, _ := os.ReadFile(logPath) // #nosec G304 -- test log path
	if strings.Contains(string(logBytes), "workspace close") || strings.Contains(string(logBytes), "tab rename") {
		t.Errorf("expected no close or layout after a tagging failure, got:\n%s", logBytes)
	}
}

// fakeHerdrServerScript answers like a Herdr daemon that is not yet running: the
// workspace API only works once the detached server has created its marker file.
const fakeHerdrServerScript = `
printf '%s\n' "$*" >> "$HERDR_TEST_LOG"
while [ $# -gt 0 ]; do
	case "$1" in
		--session) shift 2; continue;;
		workspace)
			shift
			case "$1" in
				list)
					if [ -f "$HERDR_TEST_LOG.started" ]; then
						echo '{"result":{"workspaces":[]}}'
					else
						echo 'herdr server is not running' >&2
						exit 1
					fi;;
				esac
			exit 0;;
		session)
			shift
			case "$1" in
				list) echo '{"sessions":[{"name":"default","default":true,"running":false}]}';;
				*) exit 0;;
			esac
			exit 0;;
		server) : > "$HERDR_TEST_LOG.started"; sleep 0.2; exit 0;;
	esac
	shift
done
`

func TestHerdrEnsureServerStartsWhenSessionNotRunning(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "herdr.log")
	t.Setenv("HERDR_TEST_LOG", logPath)
	bin := writeFakeHerdr(t, fakeHerdrServerScript)

	h := &Herdr{Bin: bin, Session: "wt-feat"}
	if err := h.EnsureServer(context.Background()); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}
	if _, err := os.Stat(logPath + ".started"); err != nil {
		t.Fatalf("expected the fake server marker file: %v", err)
	}
	logBytes, err := os.ReadFile(logPath) // #nosec G304 -- test log path
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(logBytes), "--session wt-feat server") {
		t.Errorf("expected the server to start in the target session, got:\n%s", string(logBytes))
	}
}

func TestHerdrRunningReportsSessionState(t *testing.T) {
	t.Setenv("HERDR_SESSION", "")
	t.Setenv("HERDR_TEST_LOG", filepath.Join(t.TempDir(), "herdr.log"))
	bin := writeFakeHerdr(t, fakeHerdrScript)

	if !(&Herdr{Bin: bin}).Running(context.Background()) {
		t.Error("expected default session to be running")
	}
	if (&Herdr{Bin: bin, Session: "wt-other"}).Running(context.Background()) {
		t.Error("expected unknown session to be not running")
	}
}

func TestHerdrRunningUsesAmbientSession(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "herdr.log")
	t.Setenv("HERDR_TEST_LOG", logPath)
	bin := writeFakeHerdr(t, strings.Replace(fakeHerdrScript,
		`{"sessions":[{"name":"default","default":true,"running":true}]}`,
		`{"sessions":[{"name":"work","running":true}]}`, 1))

	t.Setenv("HERDR_SESSION", "work")
	if !(&Herdr{Bin: bin}).Running(context.Background()) {
		t.Error("expected the ambient HERDR_SESSION to be inspected")
	}
	t.Setenv("HERDR_SESSION", "other")
	if (&Herdr{Bin: bin}).Running(context.Background()) {
		t.Error("expected a different ambient session to be reported as not running")
	}
}

func TestUniqueHerdrSessionNameUsesBaseWhenFree(t *testing.T) {
	t.Setenv("HERDR_SESSION", "")
	t.Setenv("HERDR_TEST_LOG", filepath.Join(t.TempDir(), "herdr.log"))
	bin := writeFakeHerdr(t, fakeHerdrScript)

	name, err := UniqueHerdrSessionName(context.Background(), bin, "wt-feat")
	if err != nil {
		t.Fatalf("UniqueHerdrSessionName: %v", err)
	}
	if name != "wt-feat" {
		t.Errorf("name = %q, want wt-feat", name)
	}
}

func TestUniqueHerdrSessionNameRejectsEmptyBase(t *testing.T) {
	t.Setenv("HERDR_SESSION", "")
	t.Setenv("HERDR_TEST_LOG", filepath.Join(t.TempDir(), "herdr.log"))
	bin := writeFakeHerdr(t, fakeHerdrScript)

	if _, err := UniqueHerdrSessionName(context.Background(), bin, ".."); err == nil {
		t.Fatal("expected an error for a name that sanitises to nothing")
	}
}

func TestUniqueHerdrSessionNameSkipsOccupiedSuffixes(t *testing.T) {
	t.Setenv("HERDR_SESSION", "")
	t.Setenv("HERDR_TEST_LOG", filepath.Join(t.TempDir(), "herdr.log"))
	bin := writeFakeHerdr(t, strings.Replace(fakeHerdrScript,
		`{"sessions":[{"name":"default","default":true,"running":true}]}`,
		`{"sessions":[{"name":"wt-feat","running":true},{"name":"wt-feat-2","running":true},{"name":"wt-feat-3","running":false}]}`, 1))

	name, err := UniqueHerdrSessionName(context.Background(), bin, "wt-feat")
	if err != nil {
		t.Fatalf("UniqueHerdrSessionName: %v", err)
	}
	if name != "wt-feat-4" {
		t.Errorf("name = %q, want wt-feat-4", name)
	}
}

func TestUniqueHerdrSessionNameKeepsSuffixWithinLimit(t *testing.T) {
	t.Setenv("HERDR_SESSION", "")
	for _, size := range []int{63, 64} {
		base := strings.Repeat("a", size)
		stem := base[:herdrMaxSessionNameLen-2]
		sessions := func(names ...string) string {
			entries := make([]string, 0, len(names))
			for _, name := range names {
				entries = append(entries, fmt.Sprintf(`{"name":%q,"running":true}`, name))
			}
			return fmt.Sprintf(`{"sessions":[%s]}`, strings.Join(entries, ","))
		}

		t.Run(fmt.Sprintf("%d-byte base", size), func(t *testing.T) {
			t.Setenv("HERDR_TEST_LOG", filepath.Join(t.TempDir(), "herdr.log"))
			bin := writeFakeHerdr(t, strings.Replace(fakeHerdrScript,
				`{"sessions":[{"name":"default","default":true,"running":true}]}`,
				sessions(base), 1))

			name, err := UniqueHerdrSessionName(context.Background(), bin, base)
			if err != nil {
				t.Fatalf("UniqueHerdrSessionName: %v", err)
			}
			if want := stem + "-2"; name != want {
				t.Errorf("name = %q, want %q", name, want)
			}
			if len(name) != herdrMaxSessionNameLen {
				t.Errorf("name length = %d, want %d", len(name), herdrMaxSessionNameLen)
			}
		})

		t.Run(fmt.Sprintf("%d-byte base with -2 occupied", size), func(t *testing.T) {
			t.Setenv("HERDR_TEST_LOG", filepath.Join(t.TempDir(), "herdr.log"))
			bin := writeFakeHerdr(t, strings.Replace(fakeHerdrScript,
				`{"sessions":[{"name":"default","default":true,"running":true}]}`,
				sessions(base, stem+"-2"), 1))

			name, err := UniqueHerdrSessionName(context.Background(), bin, base)
			if err != nil {
				t.Fatalf("UniqueHerdrSessionName: %v", err)
			}
			if want := stem + "-3"; name != want {
				t.Errorf("name = %q, want %q", name, want)
			}
			if len(name) != herdrMaxSessionNameLen {
				t.Errorf("name length = %d, want %d", len(name), herdrMaxSessionNameLen)
			}
			if name == stem+"-2" {
				t.Errorf("candidate %q must differ from the occupied -2 candidate", name)
			}
		})
	}
}

// cancelAfterContext reports cancellation only after a fixed number of Err
// calls, which lets a test reach the allocator's per-candidate guard.
type cancelAfterContext struct {
	context.Context
	remaining int
}

func (c *cancelAfterContext) Err() error {
	if c.remaining <= 0 {
		return context.Canceled
	}
	c.remaining--
	return nil
}

func TestUniqueHerdrSessionNameStopsWhenContextCancelsDuringSearch(t *testing.T) {
	t.Setenv("HERDR_SESSION", "")
	t.Setenv("HERDR_TEST_LOG", filepath.Join(t.TempDir(), "herdr.log"))
	bin := writeFakeHerdr(t, strings.Replace(fakeHerdrScript,
		`{"sessions":[{"name":"default","default":true,"running":true}]}`,
		`{"sessions":[{"name":"wt-feat","running":true}]}`, 1))

	ctx := &cancelAfterContext{Context: context.Background(), remaining: 0}
	if _, err := UniqueHerdrSessionName(ctx, bin, "wt-feat"); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestUniqueHerdrSessionNameHonoursCancelledContext(t *testing.T) {
	t.Setenv("HERDR_SESSION", "")
	t.Setenv("HERDR_TEST_LOG", filepath.Join(t.TempDir(), "herdr.log"))
	bin := writeFakeHerdr(t, strings.Replace(fakeHerdrScript,
		`{"sessions":[{"name":"default","default":true,"running":true}]}`,
		`{"sessions":[{"name":"wt-feat","running":true}]}`, 1))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := UniqueHerdrSessionName(ctx, bin, "wt-feat"); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestHerdrResolveWorkspace(t *testing.T) {
	long := strings.Repeat("é", 80)
	tests := []struct {
		name       string
		workspaces string
		label      string
		onExists   string
		legacy     []string
		wantID     string
		wantLabel  string
		wantErr    error
		wantLog    []string
		rejectLog  []string
	}{
		{
			name:       "reuses the tagged workspace under another label",
			workspaces: `[{"workspace_id":"w1","label":"Old name","tokens":{"lazyworktree_id":"v1:me"}}]`,
			label:      "feat",
			wantID:     "w1",
			wantLabel:  "Old name",
			rejectLog:  []string{"report-metadata"},
		},
		{
			name:       "tags a legacy workspace found by label",
			workspaces: `[{"workspace_id":"w1","label":"feat"}]`,
			label:      "feat",
			wantID:     "w1",
			wantLabel:  "feat",
			wantLog:    []string{"workspace report-metadata w1 --source lazyworktree --token lazyworktree_id=v1:me"},
		},
		{
			name:       "tags a legacy workspace created with the sanitised label",
			workspaces: `[{"workspace_id":"w1","label":"wt-feat"}]`,
			label:      "wt:feat",
			wantID:     "w1",
			wantLabel:  "wt-feat",
			wantLog:    []string{"workspace report-metadata w1 --source lazyworktree --token lazyworktree_id=v1:me"},
		},
		{
			name:       "tags a legacy workspace under the old default label",
			workspaces: `[{"workspace_id":"w1","label":"wt-feat"}]`,
			label:      "#12 Fix the thing",
			legacy:     []string{"wt:feat"},
			wantID:     "w1",
			wantLabel:  "wt-feat",
			wantLog:    []string{"workspace report-metadata w1 --source lazyworktree --token lazyworktree_id=v1:me"},
		},
		{
			name:       "kill replaces a legacy workspace with a sanitised label",
			workspaces: `[{"workspace_id":"w1","label":"wt-feat"}]`,
			label:      "wt:feat",
			onExists:   OnExistsKill,
			wantLabel:  "wt:feat",
			wantLog:    []string{"workspace close w1"},
		},
		{
			name:       "label of another worktree is a conflict",
			workspaces: `[{"workspace_id":"w1","label":"feat","tokens":{"lazyworktree_id":"v1:other"}}]`,
			label:      "feat",
			wantErr:    ErrTargetConflict,
		},
		{
			name:       "an untagged duplicate label does not hide another worktree's label",
			workspaces: `[{"workspace_id":"w1","label":"shared"},{"workspace_id":"w2","label":"shared","tokens":{"lazyworktree_id":"v1:other"}}]`,
			label:      "shared",
			wantErr:    ErrTargetConflict,
			rejectLog:  []string{"report-metadata"},
		},
		{
			name:       "kill refuses several workspaces under a legacy label",
			workspaces: `[{"workspace_id":"w1","label":"wt-feat"},{"workspace_id":"w2","label":"wt-feat"}]`,
			label:      "#12 Fix the thing",
			legacy:     []string{"wt:feat"},
			onExists:   OnExistsKill,
			wantErr:    ErrAmbiguousTarget,
			rejectLog:  []string{"workspace close"},
		},
		{
			name:       "nothing matches",
			workspaces: `[{"workspace_id":"w1","label":"other","tokens":null}]`,
			label:      "feat",
			wantLabel:  "feat",
		},
		{
			name:       "kill closes the tagged workspace",
			workspaces: `[{"workspace_id":"w1","label":"Old name","tokens":{"lazyworktree_id":"v1:me"}}]`,
			label:      "feat",
			onExists:   OnExistsKill,
			wantLabel:  "feat",
			wantLog:    []string{"workspace close w1"},
		},
		{
			name:       "kill with the label held by another workspace closes nothing",
			workspaces: `[{"workspace_id":"w1","label":"Old name","tokens":{"lazyworktree_id":"v1:me"}},{"workspace_id":"w2","label":"feat"}]`,
			label:      "feat",
			onExists:   OnExistsKill,
			wantErr:    ErrTargetConflict,
			rejectLog:  []string{"workspace close"},
		},
		{
			name:       "kill leaves an untagged workspace under the requested label alone",
			workspaces: `[{"workspace_id":"w1","label":"feat"}]`,
			label:      "feat",
			onExists:   OnExistsKill,
			wantErr:    ErrTargetConflict,
			rejectLog:  []string{"workspace close"},
		},
		{
			name:       "kill refuses tagged workspaces sharing the requested label",
			workspaces: `[{"workspace_id":"w2","label":"feat","tokens":{"lazyworktree_id":"v1:me"}},{"workspace_id":"w1","label":"feat","tokens":{"lazyworktree_id":"v1:me"}}]`,
			label:      "feat",
			onExists:   OnExistsKill,
			wantErr:    ErrAmbiguousTarget,
			rejectLog:  []string{"workspace close"},
		},
		{
			name:       "kill refuses several tagged workspaces",
			workspaces: `[{"workspace_id":"w1","label":"a","tokens":{"lazyworktree_id":"v1:me"}},{"workspace_id":"w2","label":"b","tokens":{"lazyworktree_id":"v1:me"}}]`,
			label:      "feat",
			onExists:   OnExistsKill,
			wantErr:    ErrAmbiguousTarget,
			rejectLog:  []string{"workspace close"},
		},
		{
			name:       "new appends a suffix without truncating long labels",
			workspaces: fmt.Sprintf(`[{"workspace_id":"w1","label":%q},{"workspace_id":"w2","label":%q}]`, long, long+"-2"),
			label:      long,
			onExists:   OnExistsNew,
			wantLabel:  long + "-3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logPath := filepath.Join(t.TempDir(), "herdr.log")
			t.Setenv("HERDR_SESSION", "")
			t.Setenv("HERDR_TEST_LOG", logPath)
			bin := writeFakeHerdr(t, strings.Replace(fakeHerdrScript,
				`{"result":{"workspaces":[]}}`,
				`{"result":{"workspaces":`+tt.workspaces+`}}`, 1))

			h := &Herdr{Bin: bin}
			existing, label, err := h.ResolveWorkspace(context.Background(), "v1:me", tt.label, tt.onExists, tt.legacy...)
			logBytes, _ := os.ReadFile(logPath) // #nosec G304 -- test log path
			log := string(logBytes)
			for _, reject := range tt.rejectLog {
				if strings.Contains(log, reject) {
					t.Errorf("expected log not to contain %q, got:\n%s", reject, log)
				}
			}
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveWorkspace: %v", err)
			}
			if existing.ID != tt.wantID || label != tt.wantLabel {
				t.Errorf("got (%q, %q), want (%q, %q)", existing.ID, label, tt.wantID, tt.wantLabel)
			}
			for _, want := range tt.wantLog {
				if !strings.Contains(log, want) {
					t.Errorf("expected log to contain %q, got:\n%s", want, log)
				}
			}
		})
	}
}

func TestHerdrCreateWorkspaceRequiresWorkspaceID(t *testing.T) {
	t.Setenv("HERDR_SESSION", "")
	t.Setenv("HERDR_TEST_LOG", filepath.Join(t.TempDir(), "herdr.log"))
	bin := writeFakeHerdr(t, strings.Replace(fakeHerdrScript,
		`{"result":{"workspace":{"workspace_id":"w1","label":"feat"},"tab":{"tab_id":"w1:t1","label":"1"},"root_pane":{"pane_id":"w1:p1"}}}`,
		`{"result":{"workspace":{"label":"feat"}}}`, 1))

	h := &Herdr{Bin: bin}
	if _, err := h.CreateWorkspace(context.Background(), "feat", "", "/w", []ResolvedWindow{{Name: "shell"}}, nil, true); err == nil || !strings.Contains(err.Error(), "no workspace ID") {
		t.Fatalf("error = %v, want a missing workspace ID error", err)
	}
}

func TestHerdrCreateWorkspaceRequiresTabAndPaneIDs(t *testing.T) {
	tests := []struct {
		name     string
		create   string
		tab      string
		windows  []ResolvedWindow
		wantWord string
	}{
		{
			name:     "missing first tab ID with a configured name",
			create:   `{"result":{"workspace":{"workspace_id":"w1","label":"feat"},"root_pane":{"pane_id":"w1:p1"}}}`,
			tab:      `{"result":{"root_pane":{"pane_id":"w1:p2"}}}`,
			windows:  []ResolvedWindow{{Name: "editor"}},
			wantWord: "no tab ID",
		},
		{
			name:     "missing first root pane ID with a configured command",
			create:   `{"result":{"workspace":{"workspace_id":"w1","label":"feat"},"tab":{"tab_id":"w1:t1"}}}`,
			tab:      `{"result":{"root_pane":{"pane_id":"w1:p2"}}}`,
			windows:  []ResolvedWindow{{Command: "vim"}},
			wantWord: "no root pane ID for the first tab",
		},
		{
			name:     "missing tab root pane ID with a configured command",
			create:   `{"result":{"workspace":{"workspace_id":"w1","label":"feat"},"tab":{"tab_id":"w1:t1"},"root_pane":{"pane_id":"w1:p1"}}}`,
			tab:      `{"result":{"tab":{"tab_id":"w1:t2"}}}`,
			windows:  []ResolvedWindow{{}, {Command: "go test ./..."}},
			wantWord: "no root pane ID",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HERDR_SESSION", "")
			t.Setenv("HERDR_TEST_LOG", filepath.Join(t.TempDir(), "herdr.log"))
			script := strings.Replace(fakeHerdrScript,
				`{"result":{"workspace":{"workspace_id":"w1","label":"feat"},"tab":{"tab_id":"w1:t1","label":"1"},"root_pane":{"pane_id":"w1:p1"}}}`,
				tt.create, 1)
			script = strings.Replace(script,
				`{"result":{"tab":{"tab_id":"w1:t2"},"root_pane":{"pane_id":"w1:p2"}}}`,
				tt.tab, 1)
			bin := writeFakeHerdr(t, script)

			h := &Herdr{Bin: bin}
			_, err := h.CreateWorkspace(context.Background(), "feat", "", "/w", tt.windows, nil, true)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tt.wantWord) {
				t.Errorf("error = %v, want %q", err, tt.wantWord)
			}
			if !strings.Contains(err.Error(), "is partially configured") {
				t.Errorf("error = %v, want a partial configuration message", err)
			}
		})
	}
}
