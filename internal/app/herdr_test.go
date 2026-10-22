package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/multiplexer"
)

func writeFakeHerdrBinary(t *testing.T, script string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake herdr uses a POSIX shell script")
	}
	t.Setenv("HERDR_SESSION", "")
	t.Setenv("HERDR_SOCKET_PATH", "")
	dir := t.TempDir()
	bin := filepath.Join(dir, "herdr")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"+script), 0o700); err != nil { // #nosec G306 -- test fixture
		t.Fatalf("write fake herdr: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

const fakeHerdrListScript = `
case "$1" in
	session) echo '{"sessions":[
		{"name":"default","default":true,"running":true},
		{"name":"wt-feature","running":true},
		{"name":"wt-bug","running":false},
		{"name":"other","running":true}
	]}';;
	workspace) echo '{"result":{"workspaces":[
		{"workspace_id":"w1","label":"wt-feature"},
		{"workspace_id":"w2","label":"other"},
		{"workspace_id":"w3","label":"wt:bug"}
	]}}';;
esac
`

func TestHerdrReadyShowsWorkspaceInfoWhenNotAttaching(t *testing.T) {
	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir()}, "")
	m.setWindowSize(120, 40)

	updated, cmd := m.Update(herdrReadyMsg{label: "wt_feature", attach: false, mode: config.HerdrModeWorkspace, session: "wt-feature"})
	model := updated.(*Model)
	if cmd != nil {
		t.Fatal("expected no command when not attaching")
	}
	if !model.state.ui.screenManager.IsActive() || model.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Fatalf("expected info screen, got %v", model.state.ui.screenManager.Type())
	}
	infoScr := model.state.ui.screenManager.Current().(*appscreen.InfoScreen)
	if !strings.Contains(infoScr.Message, "wt_feature") || !strings.Contains(infoScr.Message, "herdr session attach wt-feature") {
		t.Errorf("unexpected info message: %q", infoScr.Message)
	}
}

func TestHerdrWorkspaceInfoNamesAmbientSession(t *testing.T) {
	t.Setenv("HERDR_SESSION", "outer")
	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir()}, "")
	m.setWindowSize(120, 40)

	updated, cmd := m.Update(herdrReadyMsg{label: "wt-feature", attach: false, mode: config.HerdrModeWorkspace})
	model := updated.(*Model)
	if cmd != nil {
		t.Fatal("expected no command when not attaching")
	}
	infoScr := model.state.ui.screenManager.Current().(*appscreen.InfoScreen)
	if !strings.Contains(infoScr.Message, "herdr session attach outer") {
		t.Errorf("info message should name the ambient session, got: %q", infoScr.Message)
	}
}

func TestHerdrReadyShowsSessionInfoWhenNotAttaching(t *testing.T) {
	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir()}, "")
	m.setWindowSize(120, 40)

	updated, cmd := m.Update(herdrReadyMsg{session: "wt_payments", attach: false, mode: config.HerdrModeSession})
	model := updated.(*Model)
	if cmd != nil {
		t.Fatal("expected no command when not attaching")
	}
	infoScr := model.state.ui.screenManager.Current().(*appscreen.InfoScreen)
	if !strings.Contains(infoScr.Message, "herdr session attach") || !strings.Contains(infoScr.Message, "wt_payments") {
		t.Errorf("unexpected info message: %q", infoScr.Message)
	}
}

func TestHerdrReadyInsideSessionShowsNestedInfo(t *testing.T) {
	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir()}, "")
	m.setWindowSize(120, 40)

	updated, cmd := m.Update(herdrReadyMsg{session: "wt_other", attach: true, insideHerdr: true, mode: config.HerdrModeSession})
	model := updated.(*Model)
	if cmd != nil {
		t.Fatal("expected no attach command inside Herdr")
	}
	infoScr := model.state.ui.screenManager.Current().(*appscreen.InfoScreen)
	if !strings.Contains(infoScr.Message, "cannot switch sessions") {
		t.Errorf("unexpected info message: %q", infoScr.Message)
	}
}

func TestHerdrReadyNotAttachingWinsOverInsideHerdrFocus(t *testing.T) {
	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir()}, "")
	m.setWindowSize(120, 40)

	updated, cmd := m.Update(herdrReadyMsg{
		label:       "wt-feature",
		session:     "wt-feature",
		workspaceID: "w1",
		attach:      false,
		insideHerdr: true,
		mode:        config.HerdrModeWorkspace,
	})
	model := updated.(*Model)
	if cmd != nil {
		t.Fatal("expected no focus command when not attaching")
	}
	if !model.state.ui.screenManager.IsActive() || model.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Fatalf("expected info screen, got %v", model.state.ui.screenManager.Type())
	}
	infoScr := model.state.ui.screenManager.Current().(*appscreen.InfoScreen)
	if !strings.Contains(infoScr.Message, "wt-feature") {
		t.Errorf("unexpected info message: %q", infoScr.Message)
	}
}

func TestHerdrReadyAttachesWhenRequested(t *testing.T) {
	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir()}, "")
	m.setWindowSize(120, 40)

	updated, cmd := m.Update(herdrReadyMsg{session: "wt_test", attach: true, mode: config.HerdrModeSession})
	model := updated.(*Model)
	if model.state.ui.screenManager.IsActive() {
		t.Fatalf("expected no screen change, got %v", model.state.ui.screenManager.Type())
	}
	if cmd == nil {
		t.Fatal("expected attach command to be returned")
	}
}

func TestHerdrReadyWorkspaceInOtherSessionShowsNestedInfo(t *testing.T) {
	t.Setenv("HERDR_SESSION", "outer")
	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir()}, "")
	m.setWindowSize(120, 40)

	updated, cmd := m.Update(herdrReadyMsg{
		label:       "wt-feature",
		session:     "inner",
		workspaceID: "w1",
		attach:      true,
		insideHerdr: true,
		mode:        config.HerdrModeWorkspace,
	})
	model := updated.(*Model)
	if cmd != nil {
		t.Fatal("expected no focus command for a workspace in another session")
	}
	infoScr := model.state.ui.screenManager.Current().(*appscreen.InfoScreen)
	if !strings.Contains(infoScr.Message, "outside the current Herdr session") {
		t.Errorf("expected attach instructions for the target session, got: %q", infoScr.Message)
	}
	if !strings.Contains(infoScr.Message, "herdr session attach 'inner'") {
		t.Errorf("expected the target session in the attach hint, got: %q", infoScr.Message)
	}
}

func TestHerdrReadyWorkspaceInAmbientSessionFocuses(t *testing.T) {
	t.Setenv("HERDR_SESSION", "inner")
	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir()}, "")
	m.setWindowSize(120, 40)

	updated, cmd := m.Update(herdrReadyMsg{
		label:       "wt-feature",
		session:     "inner",
		workspaceID: "w1",
		attach:      true,
		insideHerdr: true,
		mode:        config.HerdrModeWorkspace,
	})
	model := updated.(*Model)
	if model.state.ui.screenManager.IsActive() {
		t.Fatalf("expected no info screen, got %v", model.state.ui.screenManager.Type())
	}
	if cmd == nil {
		t.Fatal("expected the workspace to be focused in place")
	}
}

func TestHerdrAttachCommandQuotesExecutablePath(t *testing.T) {
	got := herdrAttachCommand("/Applications/My Tools/herdr", "wt-feature")
	want := "'/Applications/My Tools/herdr' 'session' 'attach' 'wt-feature'"
	if got != want {
		t.Errorf("attach command = %q, want %q", got, want)
	}
}

func TestFocusReusedHerdrWorkspaceFocusesOtherSession(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "herdr.log")
	t.Setenv("HERDR_TEST_LOG", logPath)
	t.Setenv("HERDR_SESSION", "outer")
	writeFakeHerdrBinary(t, fakeHerdrLayoutScript)

	bin, err := exec.LookPath("herdr")
	if err != nil {
		t.Fatalf("look up fake herdr: %v", err)
	}
	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir()}, "")
	h := &multiplexer.Herdr{Bin: bin, Session: "inner"}
	msg := m.focusReusedHerdrWorkspace(context.Background(), h, &config.HerdrCommand{
		Mode:   config.HerdrModeWorkspace,
		Attach: true,
	}, herdrReadyMsg{label: "wt-feature", session: "inner", workspaceID: "w1", attach: true}, true)
	if failure, isFailure := msg.(errMsg); isFailure {
		t.Fatalf("focus reused workspace: %v", failure.err)
	}
	if log := readHerdrTestLog(t, logPath); !strings.Contains(log, "--session inner workspace focus w1") {
		t.Errorf("expected the workspace to be focused in its own session, got:\n%s", log)
	}
}

func TestHerdrReadyMatchingSessionWithCustomSocketFocuses(t *testing.T) {
	t.Setenv("HERDR_SESSION", "work")
	t.Setenv("HERDR_SOCKET_PATH", "/tmp/custom-herdr.sock")
	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir()}, "")
	m.setWindowSize(120, 40)

	updated, cmd := m.Update(herdrReadyMsg{
		label:       "wt-feature",
		session:     "work",
		workspaceID: "w1",
		attach:      true,
		insideHerdr: true,
		mode:        config.HerdrModeWorkspace,
	})
	model := updated.(*Model)
	if cmd == nil {
		t.Fatal("expected a focus command for the ambient session")
	}
	if _, ok := model.state.ui.screenManager.Current().(*appscreen.InfoScreen); ok {
		t.Error("expected focus, got attach instructions")
	}
}

func TestGetHerdrActiveSessionsFiltersPrefix(t *testing.T) {
	writeFakeHerdrBinary(t, fakeHerdrListScript)
	t.Setenv("HERDR_SESSION", "")

	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir(), SessionPrefix: "wt-"}, "")
	got := m.getHerdrActiveSessions()
	want := []herdrSessionEntry{{Name: "wt-feature", Label: "feature"}}
	if len(got) != len(want) || got[0] != want[0] {
		t.Errorf("sessions = %v, want %v", got, want)
	}
}

func TestGetHerdrWorkspacesFiltersPrefix(t *testing.T) {
	writeFakeHerdrBinary(t, fakeHerdrListScript)
	t.Setenv("HERDR_SESSION", "")

	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir(), SessionPrefix: "wt-"}, "")
	got := m.getHerdrWorkspaces()
	// "wt:bug" sanitises to "wt-bug", so it matches the prefix too.
	if len(got) != 2 || got[0].ID != "w1" || got[1].ID != "w3" {
		t.Errorf("workspaces = %+v, want wt-feature and wt:bug", got)
	}
}

func TestGetHerdrWorkspacesMatchesSanitizedLabel(t *testing.T) {
	writeFakeHerdrBinary(t, fakeHerdrListScript)
	t.Setenv("HERDR_SESSION", "")

	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir(), SessionPrefix: "wt-"}, "")
	var found bool
	for _, workspace := range m.getHerdrWorkspaces() {
		if workspace.ID == "w3" {
			found = true
		}
	}
	if !found {
		t.Error("expected the wt:bug workspace to be listed once sanitised to wt-bug")
	}
}

func TestGetHerdrSessionsAndWorkspacesWithSanitizedPrefix(t *testing.T) {
	writeFakeHerdrBinary(t, fakeHerdrListScript)
	t.Setenv("HERDR_SESSION", "")

	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir(), SessionPrefix: "wt:"}, "")
	sessions := m.getHerdrActiveSessions()
	if len(sessions) != 1 || sessions[0] != (herdrSessionEntry{Name: "wt-feature", Label: "feature"}) {
		t.Errorf("sessions = %v, want wt-feature displayed as feature", sessions)
	}
	got := m.getHerdrWorkspaces()
	if len(got) != 2 || got[0].ID != "w1" || got[1].ID != "w3" {
		t.Errorf("workspaces = %+v, want wt-feature and wt:bug", got)
	}
}

const fakeHerdrPaletteScript = `
printf '%s\n' "$*" >> "$HERDR_TEST_LOG"
if [ "$1" = "session" ]; then
	echo '{"sessions":[{"name":"default","default":true,"running":true},{"name":"wt-feature","running":true}]}'
	exit 0
fi
if [ "$1" = "workspace" ]; then
	case "$2" in
		list) echo '{"result":{"workspaces":[{"workspace_id":"w1","label":"wt-feature"},{"workspace_id":"w3","label":"wt:bug"},{"workspace_id":"w9","label":"other"}]}}';;
		*) echo '{}';;
	esac
	exit 0
fi
`

func TestCommandPaletteExposesHerdrEntries(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "herdr.log")
	t.Setenv("HERDR_TEST_LOG", logPath)
	writeFakeHerdrBinary(t, fakeHerdrPaletteScript)

	cfg := &config.AppConfig{
		WorktreeDir:   t.TempDir(),
		SessionPrefix: "wt:",
		CustomCommands: config.CustomCommandsConfig{config.PaneUniversal: {
			"H": {
				Description: "Herdr",
				ShowHelp:    true,
				Herdr:       &config.HerdrCommand{Mode: config.HerdrModeWorkspace, Attach: true},
			},
		}},
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)
	if cmd := m.showCommandPalette(); cmd == nil {
		t.Fatal("showCommandPalette returned nil command")
	}

	paletteScreen := m.state.ui.screenManager.Current().(*appscreen.CommandPaletteScreen)
	items := map[string]configItem{}
	sections := map[string]bool{}
	for _, item := range paletteScreen.Items {
		if item.IsSection {
			sections[item.Label] = true
			continue
		}
		items[item.ID] = configItem{label: item.Label, description: item.Description}
	}
	if item, ok := items["H"]; !ok || item.label != "Herdr (H)" || item.description != herdrWorkspaceLabel {
		t.Errorf("custom Herdr palette item = %+v, found=%v", item, ok)
	}
	if !sections["Multiplexer"] {
		t.Error("expected a Multiplexer section")
	}
	if item, ok := items["herdr-workspace:w1"]; !ok || item.label != "wt-feature" {
		t.Errorf("workspace palette item = %+v, found=%v", item, ok)
	}
	if item, ok := items["herdr-workspace:w3"]; !ok || item.label != "wt:bug" {
		t.Errorf("sanitised-prefix workspace item = %+v, found=%v", item, ok)
	}
	if _, ok := items["herdr-workspace:w9"]; ok {
		t.Error("workspace outside the session prefix should not be listed")
	}
	if item, ok := items["herdr-session:wt-feature"]; !ok || item.label != "feature" {
		t.Errorf("session palette item = %+v, found=%v", item, ok)
	}
	if _, ok := items["herdr-session:feature"]; ok {
		t.Error("session palette IDs should keep the full session name so attaching targets it")
	}

	cmd := paletteScreen.OnSelect("herdr-workspace:w1")
	if cmd == nil {
		t.Fatal("expected a focus command from the palette")
	}
	ready, ok := cmd().(herdrReadyMsg)
	if !ok || ready.workspaceID != "w1" {
		t.Fatalf("expected w1 to be focused, got %+v", ready)
	}
	if log := readHerdrTestLog(t, logPath); !strings.Contains(log, "workspace focus w1") {
		t.Errorf("expected the palette selection to focus the workspace, got:\n%s", log)
	}
}

type configItem struct {
	label       string
	description string
}

const fakeHerdrLayoutScript = `
printf '%s\n' "$*" >> "$HERDR_TEST_LOG"
if [ "$1" = "session" ]; then
	echo '{"sessions":[{"name":"default","default":true,"running":true}]}'
	exit 0
fi
if [ "$1" = "workspace" ]; then
	case "$2" in
		list)
			if [ "${HERDR_TEST_EXISTING:-}" = "1" ]; then
				echo '{"result":{"workspaces":[{"workspace_id":"w9","label":"wt-feat"}]}}'
			elif [ -n "${HERDR_TEST_TAGGED_KEY:-}" ]; then
				echo "{\"result\":{\"workspaces\":[{\"workspace_id\":\"w7\",\"label\":\"Old name\",\"tokens\":{\"lazyworktree_id\":\"$HERDR_TEST_TAGGED_KEY\"}}]}}"
			else
				echo '{"result":{"workspaces":[]}}'
			fi;;
		create)
			label=""
			prev=""
			for arg in "$@"; do
				[ "$prev" = "--label" ] && label="$arg"
				prev="$arg"
			done
			echo "{\"result\":{\"workspace\":{\"workspace_id\":\"w1\",\"label\":\"$label\"},\"tab\":{\"tab_id\":\"w1:t1\"},\"root_pane\":{\"pane_id\":\"w1:p1\"}}}" ;;
		*) echo '{}';;
	esac
	exit 0
fi
echo '{}'
`

func TestCreateHerdrLayoutReusesExistingWorkspace(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "herdr.log")
	t.Setenv("HERDR_TEST_LOG", logPath)
	t.Setenv("HERDR_TEST_EXISTING", "1")
	writeFakeHerdrBinary(t, fakeHerdrLayoutScript)

	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir()}, "")
	msg := m.createHerdrLayout("herdr", &config.HerdrCommand{
		Mode:     config.HerdrModeWorkspace,
		OnExists: "switch",
		Attach:   true,
	}, &models.WorktreeInfo{Path: "/w", Branch: "feat"}, "wt-feat", "", []multiplexer.ResolvedWindow{{Name: "shell"}}, nil, false, false)

	ready, ok := msg.(herdrReadyMsg)
	if !ok {
		t.Fatalf("expected herdrReadyMsg, got %T: %v", msg, msg)
	}
	if ready.workspaceID != "w9" {
		t.Errorf("workspaceID = %q, want w9", ready.workspaceID)
	}
	logBytes, err := os.ReadFile(logPath) // #nosec G304 -- test log path
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if strings.Contains(string(logBytes), "workspace create") {
		t.Errorf("expected no workspace creation, got:\n%s", string(logBytes))
	}
	if !strings.Contains(string(logBytes), "workspace report-metadata w9 --source lazyworktree --token lazyworktree_id="+multiplexer.HerdrWorktreeKey("/w")) {
		t.Errorf("expected the legacy workspace to be tagged, got:\n%s", string(logBytes))
	}
}

func TestCreateHerdrLayoutReusesTaggedWorkspaceAfterTemplateChange(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "herdr.log")
	t.Setenv("HERDR_TEST_LOG", logPath)
	t.Setenv("HERDR_TEST_TAGGED_KEY", multiplexer.HerdrWorktreeKey("/w"))
	writeFakeHerdrBinary(t, fakeHerdrLayoutScript)

	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir()}, "")
	msg := m.createHerdrLayout("herdr", &config.HerdrCommand{
		Mode:     config.HerdrModeWorkspace,
		OnExists: "switch",
		Attach:   true,
	}, &models.WorktreeInfo{Path: "/w", Branch: "feat"}, "wt-feat", "", []multiplexer.ResolvedWindow{{Name: "shell"}}, nil, false, false)

	ready, ok := msg.(herdrReadyMsg)
	if !ok {
		t.Fatalf("expected herdrReadyMsg, got %T: %v", msg, msg)
	}
	if ready.workspaceID != "w7" || ready.label != "Old name" {
		t.Errorf("ready = (%q, %q), want the tagged workspace w7 \"Old name\"", ready.workspaceID, ready.label)
	}
	if log := readHerdrTestLog(t, logPath); strings.Contains(log, "workspace create") {
		t.Errorf("expected no workspace creation, got:\n%s", log)
	}
}

func TestGetHerdrWorkspacesListsTaggedWorkspaces(t *testing.T) {
	key := multiplexer.HerdrWorktreeKey("/repo/feat")
	writeFakeHerdrBinary(t, `echo '{"result":{"workspaces":[{"workspace_id":"w1","label":"My feature","tokens":{"lazyworktree_id":"`+key+`"}},{"workspace_id":"w2","label":"other"}]}}'`)

	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir(), SessionPrefix: "wt-"}, "")
	m.state.data.worktrees = []*models.WorktreeInfo{{Path: "/repo/feat", Branch: "feature/x"}}
	got := m.getHerdrWorkspaces()
	if len(got) != 1 || got[0].ID != "w1" {
		t.Fatalf("workspaces = %+v, want only the tagged one", got)
	}
	if desc := m.describeMultiplexerTarget("herdr workspace", got[0].WorktreeKey(), multiplexer.HerdrWorktreeKey); desc != "herdr workspace for feat (feature/x)" {
		t.Errorf("description = %q", desc)
	}
}

func TestCreateHerdrLayoutCreatesIncrementedWorkspace(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "herdr.log")
	t.Setenv("HERDR_TEST_LOG", logPath)
	t.Setenv("HERDR_TEST_EXISTING", "1")
	writeFakeHerdrBinary(t, fakeHerdrLayoutScript)

	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir()}, "")
	msg := m.createHerdrLayout("herdr", &config.HerdrCommand{
		Mode:     config.HerdrModeWorkspace,
		OnExists: "new",
		Attach:   true,
	}, &models.WorktreeInfo{Path: "/w", Branch: "feat"}, "wt-feat", "", []multiplexer.ResolvedWindow{{Name: "shell"}}, nil, false, false)

	ready, ok := msg.(herdrReadyMsg)
	if !ok {
		t.Fatalf("expected herdrReadyMsg, got %T: %v", msg, msg)
	}
	if ready.label != "wt-feat-2" {
		t.Errorf("label = %q, want wt-feat-2", ready.label)
	}
	logBytes, err := os.ReadFile(logPath) // #nosec G304 -- test log path
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(logBytes), "workspace create --label wt-feat-2") {
		t.Errorf("expected incremented workspace creation, got:\n%s", string(logBytes))
	}
}

const fakeHerdrSessionScript = `
printf '%s\n' "$*" >> "$HERDR_TEST_LOG"
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
			create) echo '{"result":{"workspace":{"workspace_id":"w1","label":"feat"},"tab":{"tab_id":"w1:t1"},"root_pane":{"pane_id":"w1:p1"}}}';;
			*) echo '{}';;
		esac;;
	pane|tab|server) exit 0;;
esac
exit 0
`

func readHerdrTestLog(t *testing.T, logPath string) string {
	t.Helper()
	logBytes, err := os.ReadFile(logPath) // #nosec G304 -- test log path
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	return string(logBytes)
}

func TestCreateHerdrLayoutNestedSessionCreatesWorkspaceAndShowsAttachInstructions(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "herdr.log")
	t.Setenv("HERDR_TEST_LOG", logPath)
	t.Setenv("HERDR_TEST_SESSIONS", `[{"name":"wt-feat","running":true}]`)
	writeFakeHerdrBinary(t, fakeHerdrSessionScript)

	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir()}, "")
	m.setWindowSize(120, 40)

	msg := m.createHerdrLayout("herdr", &config.HerdrCommand{
		Mode:     config.HerdrModeSession,
		OnExists: "switch",
		Attach:   true,
	}, &models.WorktreeInfo{Path: "/w", Branch: "feat"}, "wt-feat", "wt-feat", []multiplexer.ResolvedWindow{{Name: "shell"}}, nil, true, false)

	ready, ok := msg.(herdrReadyMsg)
	if !ok {
		t.Fatalf("expected herdrReadyMsg, got %T: %v", msg, msg)
	}
	if ready.session != "wt-feat" || !ready.insideHerdr || !ready.attach {
		t.Errorf("unexpected ready message: %+v", ready)
	}
	if ready.workspaceID != "w1" {
		t.Errorf("workspaceID = %q, want w1", ready.workspaceID)
	}

	cmd := m.handleHerdrReady(ready)
	if cmd != nil {
		t.Fatal("expected no command when nested in Herdr")
	}
	infoScr, ok := m.state.ui.screenManager.Current().(*appscreen.InfoScreen)
	if !ok {
		t.Fatalf("expected info screen, got %v", m.state.ui.screenManager.Type())
	}
	if !strings.Contains(infoScr.Message, "cannot switch sessions") {
		t.Errorf("unexpected info message: %q", infoScr.Message)
	}
	if log := readHerdrTestLog(t, logPath); !strings.Contains(log, "--session wt-feat workspace create") {
		t.Errorf("expected the workspace layout to be created in the running session, got:\n%s", log)
	}
}

func TestCreateHerdrLayoutReusedWorkspaceFocusesBeforeAttach(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "herdr.log")
	t.Setenv("HERDR_TEST_LOG", logPath)
	t.Setenv("HERDR_TEST_EXISTING", "1")
	writeFakeHerdrBinary(t, fakeHerdrLayoutScript)

	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir()}, "")
	m.commandRunner = func(_ context.Context, name string, args ...string) *exec.Cmd {
		appendHerdrTestLog(t, logPath, "attach "+name+" "+strings.Join(args, " "))
		return exec.Command("true")
	}
	m.execProcess = func(_ *exec.Cmd, cb tea.ExecCallback) tea.Cmd {
		return func() tea.Msg { return cb(nil) }
	}

	msg := m.createHerdrLayout("herdr", &config.HerdrCommand{
		Mode:     config.HerdrModeWorkspace,
		OnExists: "switch",
		Attach:   true,
	}, &models.WorktreeInfo{Path: "/w", Branch: "feat"}, "wt-feat", "", []multiplexer.ResolvedWindow{{Name: "shell"}}, nil, false, false)

	ready, ok := msg.(herdrReadyMsg)
	if !ok {
		t.Fatalf("expected herdrReadyMsg, got %T: %v", msg, msg)
	}
	if ready.workspaceID != "w9" {
		t.Fatalf("workspaceID = %q, want w9", ready.workspaceID)
	}

	cmd := m.handleHerdrReady(ready)
	if cmd == nil {
		t.Fatal("expected an attach command")
	}
	if msg := cmd(); msg == nil {
		t.Fatal("expected the attach command to report back")
	}

	log := readHerdrTestLog(t, logPath)
	focusIndex := strings.Index(log, "workspace focus w9")
	attachIndex := strings.Index(log, "attach herdr")
	if focusIndex < 0 {
		t.Fatalf("expected the reused workspace to be focused, got:\n%s", log)
	}
	if attachIndex < 0 {
		t.Fatalf("expected an attach call, got:\n%s", log)
	}
	if focusIndex > attachIndex {
		t.Errorf("expected focus before attach, got:\n%s", log)
	}
}

func TestCreateHerdrLayoutReusedWorkspaceSkipsFocusWhenNotAttaching(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "herdr.log")
	t.Setenv("HERDR_TEST_LOG", logPath)
	t.Setenv("HERDR_TEST_EXISTING", "1")
	writeFakeHerdrBinary(t, fakeHerdrLayoutScript)

	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir()}, "")
	m.setWindowSize(120, 40)
	msg := m.createHerdrLayout("herdr", &config.HerdrCommand{
		Mode:     config.HerdrModeWorkspace,
		OnExists: "switch",
		Attach:   false,
	}, &models.WorktreeInfo{Path: "/w", Branch: "feat"}, "wt-feat", "", []multiplexer.ResolvedWindow{{Name: "shell"}}, nil, false, false)

	if _, ok := msg.(herdrReadyMsg); !ok {
		t.Fatalf("expected herdrReadyMsg, got %T: %v", msg, msg)
	}
	if log := readHerdrTestLog(t, logPath); strings.Contains(log, "workspace focus") {
		t.Errorf("expected no focus when attaching is disabled, got:\n%s", log)
	}
}

func TestCreateHerdrLayoutSessionReusesExistingWorkspace(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "herdr.log")
	t.Setenv("HERDR_TEST_LOG", logPath)
	t.Setenv("HERDR_TEST_SESSIONS", `[{"name":"feat","running":true}]`)
	t.Setenv("HERDR_TEST_WORKSPACES", `[{"workspace_id":"w7","label":"feat"}]`)
	writeFakeHerdrBinary(t, fakeHerdrSessionScript)

	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir()}, "")
	m.commandRunner = func(_ context.Context, name string, args ...string) *exec.Cmd {
		appendHerdrTestLog(t, logPath, "attach "+name+" "+strings.Join(args, " "))
		return exec.Command("true")
	}
	m.execProcess = func(_ *exec.Cmd, cb tea.ExecCallback) tea.Cmd {
		return func() tea.Msg { return cb(nil) }
	}

	msg := m.createHerdrLayout("herdr", &config.HerdrCommand{
		Mode:     config.HerdrModeSession,
		OnExists: "switch",
		Attach:   true,
	}, &models.WorktreeInfo{Path: "/w/feat", Branch: "feat"}, "feat", "feat", []multiplexer.ResolvedWindow{{Name: "shell"}}, nil, false, false)

	ready, ok := msg.(herdrReadyMsg)
	if !ok {
		t.Fatalf("expected herdrReadyMsg, got %T: %v", msg, msg)
	}
	if ready.workspaceID != "w7" {
		t.Errorf("workspaceID = %q, want w7", ready.workspaceID)
	}
	log := readHerdrTestLog(t, logPath)
	if strings.Contains(log, "workspace create") {
		t.Errorf("expected the existing workspace to be reused, got:\n%s", log)
	}
	if !strings.Contains(log, "workspace focus w7") {
		t.Errorf("expected the reused workspace to be focused before attaching, got:\n%s", log)
	}
}

func TestCreateHerdrLayoutSessionCreatesWorkspaceInRunningSession(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "herdr.log")
	t.Setenv("HERDR_TEST_LOG", logPath)
	t.Setenv("HERDR_TEST_SESSIONS", `[{"name":"wt-feat","running":true}]`)
	writeFakeHerdrBinary(t, fakeHerdrSessionScript)

	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir()}, "")
	msg := m.createHerdrLayout("herdr", &config.HerdrCommand{
		Mode:     config.HerdrModeSession,
		OnExists: "switch",
		Attach:   false,
	}, &models.WorktreeInfo{Path: "/w", Branch: "feat"}, "wt-feat", "wt-feat", []multiplexer.ResolvedWindow{{Name: "shell"}}, nil, false, false)

	ready, ok := msg.(herdrReadyMsg)
	if !ok {
		t.Fatalf("expected herdrReadyMsg, got %T: %v", msg, msg)
	}
	if ready.session != "wt-feat" || ready.workspaceID != "w1" {
		t.Errorf("unexpected ready message: %+v", ready)
	}
	if log := readHerdrTestLog(t, logPath); !strings.Contains(log, "--session wt-feat workspace create") {
		t.Errorf("expected the missing workspace to be created, got:\n%s", log)
	}
}

func TestCreateHerdrLayoutSessionOnExistsKill(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "herdr.log")
	t.Setenv("HERDR_TEST_LOG", logPath)
	t.Setenv("HERDR_TEST_SESSIONS", `[{"name":"wt-feat","running":true}]`)
	writeFakeHerdrBinary(t, fakeHerdrSessionScript)

	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir()}, "")
	msg := m.createHerdrLayout("herdr", &config.HerdrCommand{
		Mode:     config.HerdrModeSession,
		OnExists: "kill",
		Attach:   true,
	}, &models.WorktreeInfo{Path: "/w", Branch: "feat"}, "wt-feat", "wt-feat", []multiplexer.ResolvedWindow{{Name: "shell"}}, nil, false, false)

	ready, ok := msg.(herdrReadyMsg)
	if !ok {
		t.Fatalf("expected herdrReadyMsg, got %T: %v", msg, msg)
	}
	if ready.session != "wt-feat" || ready.workspaceID != "w1" {
		t.Errorf("unexpected ready message: %+v", ready)
	}
	log := readHerdrTestLog(t, logPath)
	if !strings.Contains(log, "session stop wt-feat") {
		t.Errorf("expected the running session to be stopped, got:\n%s", log)
	}
	if !strings.Contains(log, "--session wt-feat workspace create") {
		t.Errorf("expected creation to proceed in the same session, got:\n%s", log)
	}
}

func TestCreateHerdrLayoutSessionOnExistsNew(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "herdr.log")
	t.Setenv("HERDR_TEST_LOG", logPath)
	t.Setenv("HERDR_TEST_SESSIONS", `[{"name":"wt-feat","running":true}]`)
	writeFakeHerdrBinary(t, fakeHerdrSessionScript)

	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir()}, "")
	msg := m.createHerdrLayout("herdr", &config.HerdrCommand{
		Mode:     config.HerdrModeSession,
		OnExists: "new",
		Attach:   true,
	}, &models.WorktreeInfo{Path: "/w", Branch: "feat"}, "wt-feat", "wt-feat", []multiplexer.ResolvedWindow{{Name: "shell"}}, nil, false, false)

	ready, ok := msg.(herdrReadyMsg)
	if !ok {
		t.Fatalf("expected herdrReadyMsg, got %T: %v", msg, msg)
	}
	if ready.session != "wt-feat-2" {
		t.Errorf("session = %q, want wt-feat-2", ready.session)
	}
	log := readHerdrTestLog(t, logPath)
	if !strings.Contains(log, "--session wt-feat-2 workspace create") {
		t.Errorf("expected creation in the incremented session, got:\n%s", log)
	}
}

const fakeHerdrPartialRetryScript = `
printf '%s\n' "$*" >> "$HERDR_TEST_LOG"
if [ "$1" = "--session" ]; then
	shift 2
fi
case "$1" in
	session)
		case "$2" in
			list) echo '{"sessions":[{"name":"feat","running":true}]}';;
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

func TestCreateHerdrLayoutRetriesAfterPartialFailure(t *testing.T) {
	for _, mode := range []string{config.HerdrModeWorkspace, config.HerdrModeSession} {
		t.Run(mode, func(t *testing.T) {
			logPath := filepath.Join(t.TempDir(), "herdr.log")
			t.Setenv("HERDR_TEST_LOG", logPath)
			writeFakeHerdrBinary(t, fakeHerdrPartialRetryScript)

			failMarker := logPath + ".fail"
			if err := os.WriteFile(failMarker, nil, 0o600); err != nil {
				t.Fatalf("write fail marker: %v", err)
			}

			m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir()}, "")
			cfg := &config.HerdrCommand{Mode: mode, OnExists: "switch", Attach: false}
			wt := &models.WorktreeInfo{Path: "/w/feat", Branch: "feat"}
			windows := []multiplexer.ResolvedWindow{{Name: "shell"}}

			first := m.createHerdrLayout("herdr", cfg, wt, "feat", "feat", windows, nil, false, false)
			errMsgValue, ok := first.(errMsg)
			if !ok {
				t.Fatalf("expected the first attempt to fail, got %T: %v", first, first)
			}
			if !strings.Contains(errMsgValue.err.Error(), "is partially configured") {
				t.Errorf("first error = %v, want a partial configuration message", errMsgValue.err)
			}

			if err := os.Remove(failMarker); err != nil {
				t.Fatalf("remove fail marker: %v", err)
			}
			second := m.createHerdrLayout("herdr", cfg, wt, "feat", "feat", windows, nil, false, false)
			ready, ok := second.(herdrReadyMsg)
			if !ok {
				t.Fatalf("expected the retry to succeed, got %T: %v", second, second)
			}
			if ready.workspaceID != "w1" {
				t.Errorf("workspaceID = %q, want w1", ready.workspaceID)
			}
			log := readHerdrTestLog(t, logPath)
			if got := strings.Count(log, "workspace create"); got != 1 {
				t.Errorf("workspace create ran %d times, want 1:\n%s", got, log)
			}
		})
	}
}

func TestOpenHerdrSessionRejectsInvalidConfiguredNames(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.HerdrCommand
		path string
		want string
	}{
		{
			name: "session mode dot-dot session name",
			cfg:  &config.HerdrCommand{Mode: config.HerdrModeSession, SessionName: "..", Attach: true},
			want: "session_name",
		},
		{
			name: "session mode slash session name",
			cfg:  &config.HerdrCommand{Mode: config.HerdrModeSession, SessionName: "/", Attach: true},
			want: "session_name",
		},
		{
			name: "workspace mode dot-dot session",
			cfg:  &config.HerdrCommand{Mode: config.HerdrModeWorkspace, HerdrSession: "..", Attach: true},
			want: "herdr_session",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logPath := filepath.Join(t.TempDir(), "herdr.log")
			t.Setenv("HERDR_TEST_LOG", logPath)
			writeFakeHerdrBinary(t, fakeHerdrSessionScript)

			path := tt.path
			if path == "" {
				path = "/w/feat"
			}
			m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir()}, "")
			cmd := m.openHerdrSession(&config.CustomCommand{Herdr: tt.cfg}, &models.WorktreeInfo{Path: path, Branch: "feat"})
			if cmd == nil {
				t.Fatal("expected an error command")
			}
			if _, ok := cmd().(errMsg); !ok {
				t.Fatal("expected an errMsg")
			}
			if _, err := os.Stat(logPath); err == nil {
				t.Errorf("expected no herdr command to run, got:\n%s", readHerdrTestLog(t, logPath))
			}
		})
	}
}

func TestOpenHerdrSessionAcceptsDotOnlyValidName(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "herdr.log")
	t.Setenv("HERDR_TEST_LOG", logPath)
	t.Setenv("HERDR_TEST_SESSIONS", `[]`)
	writeFakeHerdrBinary(t, fakeHerdrSessionScript)

	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir()}, "")
	cmd := m.openHerdrSession(&config.CustomCommand{Herdr: &config.HerdrCommand{
		Mode:        config.HerdrModeSession,
		SessionName: "...",
		Attach:      false,
		Windows:     []config.TmuxWindow{{Name: "shell"}},
	}}, &models.WorktreeInfo{Path: "/w/feat", Branch: "feat"})
	if cmd == nil {
		t.Fatal("expected a command")
	}
	ready, ok := cmd().(herdrReadyMsg)
	if !ok {
		t.Fatal("expected herdrReadyMsg for the valid name")
	}
	if ready.session != "..." {
		t.Errorf("session = %q, want ...", ready.session)
	}
}

func TestOpenHerdrSessionFallsBackToPrefixAndBasename(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "herdr.log")
	t.Setenv("HERDR_TEST_LOG", logPath)
	t.Setenv("HERDR_TEST_SESSIONS", `[]`)
	writeFakeHerdrBinary(t, fakeHerdrSessionScript)

	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir(), SessionPrefix: "wt-"}, "")
	cmd := m.openHerdrSession(&config.CustomCommand{Herdr: &config.HerdrCommand{
		Mode:    config.HerdrModeSession,
		Attach:  false,
		Windows: []config.TmuxWindow{{Name: "shell"}},
	}}, &models.WorktreeInfo{Path: "/w/feat", Branch: "feat"})
	if cmd == nil {
		t.Fatal("expected a command")
	}
	ready, ok := cmd().(herdrReadyMsg)
	if !ok {
		t.Fatal("expected herdrReadyMsg")
	}
	if ready.session != "wt-feat" {
		t.Errorf("session = %q, want wt-feat", ready.session)
	}
}

func TestOpenHerdrWorkspaceCmdFocusesPaletteWorkspace(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "herdr.log")
	t.Setenv("HERDR_TEST_LOG", logPath)
	writeFakeHerdrBinary(t, fakeHerdrLayoutScript)

	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir()}, "")
	cmd := m.openHerdrWorkspaceCmd("w9")
	if cmd == nil {
		t.Fatal("expected a focus command")
	}
	ready, ok := cmd().(herdrReadyMsg)
	if !ok {
		t.Fatalf("expected herdrReadyMsg, got %T", cmd())
	}
	if ready.workspaceID != "w9" || !ready.attach {
		t.Errorf("unexpected ready message: %+v", ready)
	}
	if log := readHerdrTestLog(t, logPath); !strings.Contains(log, "workspace focus w9") {
		t.Errorf("expected the palette workspace to be focused, got:\n%s", log)
	}
}

func TestOpenHerdrSessionByNameCmdKeepsDiscoveredName(t *testing.T) {
	t.Setenv("HERDR_SESSION", "outer")
	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir(), SessionPrefix: "wt-"}, "")
	m.setWindowSize(120, 40)

	cmd := m.openHerdrSessionByNameCmd("wt-feature")
	if cmd != nil {
		t.Fatal("expected no nested attach command")
	}
	infoScr, ok := m.state.ui.screenManager.Current().(*appscreen.InfoScreen)
	if !ok {
		t.Fatalf("expected info screen, got %v", m.state.ui.screenManager.Type())
	}
	if !strings.Contains(infoScr.Message, "wt-feature") || strings.Contains(infoScr.Message, "wt-wt-feature") {
		t.Errorf("expected the discovered session name without a second prefix, got: %q", infoScr.Message)
	}
}

func TestOpenHerdrSessionByNameCmdAttachesDiscoveredName(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "herdr.log")
	t.Setenv("HERDR_TEST_LOG", logPath)
	writeFakeHerdrBinary(t, fakeHerdrLayoutScript)

	m := NewModel(&config.AppConfig{WorktreeDir: t.TempDir(), SessionPrefix: "wt:"}, "")
	m.commandRunner = func(_ context.Context, name string, args ...string) *exec.Cmd {
		appendHerdrTestLog(t, logPath, "attach "+name+" "+strings.Join(args, " "))
		return exec.Command("true")
	}
	m.execProcess = func(_ *exec.Cmd, cb tea.ExecCallback) tea.Cmd {
		return func() tea.Msg { return cb(nil) }
	}

	cmd := m.openHerdrSessionByNameCmd("wt-feature")
	if cmd == nil {
		t.Fatal("expected an attach command")
	}
	_ = cmd()
	if log := readHerdrTestLog(t, logPath); !strings.Contains(log, "attach herdr session attach wt-feature") {
		t.Errorf("expected the discovered session to be attached unchanged, got:\n%s", log)
	}
}

func appendHerdrTestLog(t *testing.T, logPath, line string) {
	t.Helper()
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) // #nosec G304 -- test log path
	if err != nil {
		t.Fatalf("open log: %v", err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString(line + "\n"); err != nil {
		t.Fatalf("append log: %v", err)
	}
}
