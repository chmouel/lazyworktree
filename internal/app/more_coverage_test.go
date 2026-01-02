package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

const (
	featureBranch = "feature"
	testPRURL     = "https://example.com/pr/1"
)

func TestGetSelectedPath(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.selectedPath = "/tmp/selected"

	if got := m.GetSelectedPath(); got != "/tmp/selected" {
		t.Fatalf("expected selected path, got %q", got)
	}
}

func TestEnvMapToList(t *testing.T) {
	env := map[string]string{
		"A": "1",
		"B": "2",
	}

	list := envMapToList(env)
	if len(list) != 2 {
		t.Fatalf("expected two env entries, got %d", len(list))
	}

	values := map[string]bool{}
	for _, entry := range list {
		values[entry] = true
	}

	if !values["A=1"] || !values["B=2"] {
		t.Fatalf("unexpected env list: %v", list)
	}
}

func TestReadTmuxSessionFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "session")

	if got := readTmuxSessionFile(filepath.Join(tmpDir, "missing"), "fallback"); got != "fallback" {
		t.Fatalf("expected fallback on missing file, got %q", got)
	}

	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatalf("write empty file: %v", err)
	}
	if got := readTmuxSessionFile(path, "fallback"); got != "fallback" {
		t.Fatalf("expected fallback on empty file, got %q", got)
	}

	if err := os.WriteFile(path, []byte("session-name\n"), 0o600); err != nil {
		t.Fatalf("write session file: %v", err)
	}
	if got := readTmuxSessionFile(path, "fallback"); got != "session-name" {
		t.Fatalf("expected session-name, got %q", got)
	}
}

func TestCollectInitTerminateCommands(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir:       t.TempDir(),
		InitCommands:      []string{"init-1"},
		TerminateCommands: []string{"term-1"},
	}
	m := NewModel(cfg, "")
	m.repoConfig = &config.RepoConfig{
		InitCommands:      []string{"init-2"},
		TerminateCommands: []string{"term-2"},
	}

	initCmds := m.collectInitCommands()
	if strings.Join(initCmds, ",") != "init-1,init-2" {
		t.Fatalf("unexpected init commands: %v", initCmds)
	}

	termCmds := m.collectTerminateCommands()
	if strings.Join(termCmds, ",") != "term-1,term-2" {
		t.Fatalf("unexpected terminate commands: %v", termCmds)
	}
}

func TestRunCommandsWithTrustNever(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		TrustMode:   "never",
	}
	m := NewModel(cfg, "")

	called := false
	cmd := m.runCommandsWithTrust([]string{"echo hi"}, "", nil, func() tea.Msg {
		called = true
		return nil
	})
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}
	_ = cmd()
	if !called {
		t.Fatal("expected after function to be called")
	}
}

func TestRunCommandsWithTrustTofu(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	trustPath := filepath.Join(t.TempDir(), ".wt.yaml")
	if err := os.WriteFile(trustPath, []byte("commands: []"), 0o600); err != nil {
		t.Fatalf("write trust file: %v", err)
	}
	m.repoConfigPath = trustPath
	m.repoConfig = &config.RepoConfig{}

	cmd := m.runCommandsWithTrust([]string{"echo hi"}, "", nil, nil)
	if cmd != nil {
		t.Fatal("expected no command for trust prompt")
	}
	if m.currentScreen != screenTrust {
		t.Fatalf("expected trust screen, got %v", m.currentScreen)
	}
	if m.trustScreen == nil || len(m.pendingCommands) != 1 {
		t.Fatalf("expected pending commands to be set, got %v", m.pendingCommands)
	}
}

func TestClearPendingTrust(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.pendingCommands = []string{"cmd"}
	m.pendingCmdEnv = map[string]string{"A": "1"}
	m.pendingCmdCwd = "/tmp"
	m.pendingAfter = func() tea.Msg { return nil }
	m.pendingTrust = "/tmp/.wt.yaml"
	m.trustScreen = NewTrustScreen("/tmp/.wt.yaml", []string{"cmd"}, m.theme)

	m.clearPendingTrust()

	if m.pendingCommands != nil || m.pendingCmdEnv != nil || m.pendingCmdCwd != "" || m.pendingAfter != nil || m.pendingTrust != "" || m.trustScreen != nil {
		t.Fatal("expected pending trust state to be cleared")
	}
}

func TestShowDeleteWorktree(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.filteredWts = []*models.WorktreeInfo{
		{Path: "/tmp/main", Branch: mainWorktreeName, IsMain: true},
		{Path: "/tmp/feat", Branch: featureBranch},
	}

	m.selectedIndex = 0
	if cmd := m.showDeleteWorktree(); cmd != nil {
		t.Fatal("expected nil command for main worktree")
	}
	if m.confirmScreen != nil {
		t.Fatal("expected no confirm screen for main worktree")
	}

	m.selectedIndex = 1
	if cmd := m.showDeleteWorktree(); cmd != nil {
		t.Fatal("expected nil command for confirm screen")
	}
	if m.confirmScreen == nil || m.confirmAction == nil || m.currentScreen != screenConfirm {
		t.Fatal("expected confirm screen to be set")
	}
}

func TestShowRenameWorktree(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.filteredWts = []*models.WorktreeInfo{
		{Path: "/tmp/main", Branch: mainWorktreeName, IsMain: true},
		{Path: "/tmp/feat", Branch: featureBranch},
	}

	m.selectedIndex = 0
	if cmd := m.showRenameWorktree(); cmd != nil {
		t.Fatal("expected nil command for main worktree")
	}
	if m.currentScreen != screenInfo {
		t.Fatalf("expected info screen, got %v", m.currentScreen)
	}
	if m.infoScreen == nil || !strings.Contains(m.infoScreen.message, "Cannot rename") {
		t.Fatalf("expected rename warning modal, got %#v", m.infoScreen)
	}

	m.selectedIndex = 1
	if cmd := m.showRenameWorktree(); cmd == nil {
		t.Fatal("expected input screen command")
	}
	if m.inputScreen == nil || m.currentScreen != screenInput {
		t.Fatal("expected input screen to be set")
	}
}

func TestShowPruneMerged(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.worktrees = []*models.WorktreeInfo{
		{Path: "/tmp/main", Branch: mainWorktreeName, IsMain: true},
		{Path: "/tmp/feat", Branch: featureBranch, PR: &models.PRInfo{State: "OPEN"}},
	}

	if cmd := m.showPruneMerged(); cmd != nil {
		t.Fatal("expected nil command when nothing to prune")
	}
	if m.currentScreen != screenInfo {
		t.Fatalf("expected info screen, got %v", m.currentScreen)
	}
	if m.infoScreen == nil || m.infoScreen.message != "No merged PR worktrees to prune." {
		t.Fatalf("unexpected info modal: %#v", m.infoScreen)
	}

	m.worktrees = []*models.WorktreeInfo{
		{Path: "/tmp/main", Branch: mainWorktreeName, IsMain: true},
		{Path: "/tmp/merged", Branch: "merged", PR: &models.PRInfo{State: "MERGED"}},
	}
	if cmd := m.showPruneMerged(); cmd != nil {
		t.Fatal("expected nil command for confirm screen")
	}
	if m.confirmScreen == nil || m.confirmAction == nil || m.currentScreen != screenConfirm {
		t.Fatal("expected confirm screen for prune")
	}
}

func TestHandlePruneResult(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	msg := pruneResultMsg{
		worktrees: []*models.WorktreeInfo{{Path: "/tmp/wt", Branch: featureBranch}},
		pruned:    2,
		failed:    1,
	}
	_, _ = m.handlePruneResult(msg)

	if !strings.Contains(m.statusContent, "Pruned 2 merged worktrees") || !strings.Contains(m.statusContent, "(1 failed)") {
		t.Fatalf("unexpected prune status: %q", m.statusContent)
	}
	if len(m.worktrees) != 1 {
		t.Fatalf("expected worktrees to be updated, got %d", len(m.worktrees))
	}
}

func TestHandleAbsorbResult(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	_, cmd := m.handleAbsorbResult(absorbMergeResultMsg{err: fmt.Errorf("boom")})
	if cmd != nil {
		t.Fatal("expected no command on error")
	}
	if m.currentScreen != screenInfo {
		t.Fatal("expected screenInfo to be shown for error")
	}
	if m.infoScreen == nil {
		t.Fatal("expected infoScreen to be set")
	}

	// Reset for next test
	m.currentScreen = screenNone
	m.infoScreen = nil

	_, cmd = m.handleAbsorbResult(absorbMergeResultMsg{path: "/tmp/wt", branch: featureBranch})
	if cmd == nil {
		t.Fatal("expected command for delete worktree")
	}
}

func TestShowDiffNoDiff(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir:       t.TempDir(),
		MaxUntrackedDiffs: 0,
		MaxDiffChars:      1000,
	}
	m := NewModel(cfg, "")
	m.filteredWts = []*models.WorktreeInfo{{Path: cfg.WorktreeDir, Branch: featureBranch}}
	m.selectedIndex = 0

	// showDiff now uses execProcess (like custom commands) which returns an execMsg
	// This is consistent with how custom commands and arbitrary commands work
	cmd := m.showDiff()
	if cmd == nil {
		t.Fatal("expected diff command")
	}

	// The command now returns an execMsg (via execProcess), not nil
	// It will spawn the pager even if diff is empty (consistent with custom commands)
	// We don't test the actual execution here as it would spawn a real process
}

func TestHandleOpenPRsLoaded(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)

	if cmd := m.handleOpenPRsLoaded(openPRsLoadedMsg{err: fmt.Errorf("fail")}); cmd != nil {
		t.Fatal("expected no command on error")
	}
	if m.currentScreen != screenInfo {
		t.Fatalf("expected info screen, got %v", m.currentScreen)
	}
	if m.infoScreen == nil || !strings.Contains(m.infoScreen.message, "Failed to fetch PRs") {
		t.Fatalf("unexpected info modal: %#v", m.infoScreen)
	}

	m.currentScreen = screenNone
	m.infoScreen = nil

	if cmd := m.handleOpenPRsLoaded(openPRsLoadedMsg{prs: []*models.PRInfo{}}); cmd != nil {
		t.Fatal("expected no command on empty list")
	}
	if m.currentScreen != screenInfo {
		t.Fatalf("expected info screen, got %v", m.currentScreen)
	}
	if m.infoScreen == nil || m.infoScreen.message != "No open PRs/MRs found." {
		t.Fatalf("unexpected info modal: %#v", m.infoScreen)
	}

	prs := []*models.PRInfo{{Number: 1, Title: "Test", Branch: featureBranch}}
	cmd := m.handleOpenPRsLoaded(openPRsLoadedMsg{prs: prs})
	if cmd == nil {
		t.Fatal("expected command for PR selection")
	}
	if m.currentScreen != screenPRSelect || m.prSelectionScreen == nil {
		t.Fatal("expected PR selection screen")
	}
}

func TestFetchCommandMessages(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	if msg := m.fetchPRData()(); msg == nil {
		t.Fatal("expected pr data message")
	}
	if msg := m.fetchCIStatus(1, featureBranch)(); msg == nil {
		t.Fatal("expected ci status message")
	}
	if msg := m.fetchRemotes()(); msg == nil {
		t.Fatal("expected fetch remotes message")
	}
}

func TestRenderScreenVariants(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)

	m.currentScreen = screenCommit
	out := m.renderScreen()
	if out == "" || m.commitScreen == nil {
		t.Fatal("expected commit screen to render")
	}

	m.confirmScreen = NewConfirmScreen("Confirm?", m.theme)
	m.currentScreen = screenConfirm
	if out = m.renderScreen(); out == "" {
		t.Fatal("expected confirm screen to render")
	}

	m.infoScreen = NewInfoScreen("Info", m.theme)
	m.currentScreen = screenInfo
	if out = m.renderScreen(); out == "" {
		t.Fatal("expected info screen to render")
	}

	m.trustScreen = NewTrustScreen("/tmp/.wt.yaml", []string{"cmd"}, m.theme)
	m.currentScreen = screenTrust
	if out = m.renderScreen(); out == "" {
		t.Fatal("expected trust screen to render")
	}

	m.welcomeScreen = nil
	m.currentScreen = screenWelcome
	if out = m.renderScreen(); out == "" {
		t.Fatal("expected welcome screen to render")
	}

	m.paletteScreen = NewCommandPaletteScreen([]paletteItem{{id: "help", label: "Help"}}, m.theme)
	m.currentScreen = screenPalette
	if out = m.renderScreen(); out == "" {
		t.Fatal("expected palette screen to render")
	}

	m.diffScreen = NewDiffScreen("Diff", "diff", m.theme)
	m.currentScreen = screenDiff
	if out = m.renderScreen(); out == "" {
		t.Fatal("expected diff screen to render")
	}

	m.inputScreen = NewInputScreen("Prompt", "Placeholder", "value", m.theme)
	m.currentScreen = screenInput
	if out = m.renderScreen(); out == "" {
		t.Fatal("expected input screen to render")
	}

	m.listScreen = NewListSelectionScreen([]selectionItem{{id: "a", label: "A"}}, "Select", "", "", 120, 40, "", m.theme)
	m.currentScreen = screenListSelect
	if out = m.renderScreen(); out == "" {
		t.Fatal("expected list selection screen to render")
	}
}

func TestErrMsgShowsInfo(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	_, _ = m.Update(errMsg{err: errors.New("boom")})

	if m.currentScreen != screenInfo {
		t.Fatalf("expected info screen, got %v", m.currentScreen)
	}
	if m.infoScreen == nil || !strings.Contains(m.infoScreen.message, "boom") {
		t.Fatalf("expected info modal to include error, got %#v", m.infoScreen)
	}
}

func TestFetchRemotesCompleteTriggersRefresh(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.loading = true

	_, cmd := m.Update(fetchRemotesCompleteMsg{})
	if m.loading {
		t.Fatal("expected loading to be false")
	}
	if m.statusContent != "Remotes fetched" {
		t.Fatalf("unexpected status: %q", m.statusContent)
	}
	if cmd == nil {
		t.Fatal("expected refresh command")
	}
	if msg := cmd(); msg == nil {
		t.Fatal("expected refresh command to return a message")
	}
}

func TestMaybeFetchCIStatus(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.filteredWts = []*models.WorktreeInfo{
		{Branch: featureBranch, PR: &models.PRInfo{Number: 1}},
	}
	m.selectedIndex = 0

	m.ciCache[featureBranch] = &ciCacheEntry{fetchedAt: time.Now()}
	if cmd := m.maybeFetchCIStatus(); cmd != nil {
		t.Fatal("expected no fetch when cache is fresh")
	}

	m.ciCache[featureBranch] = &ciCacheEntry{fetchedAt: time.Now().Add(-ciCacheTTL - time.Second)}
	if cmd := m.maybeFetchCIStatus(); cmd == nil {
		t.Fatal("expected fetch when cache is stale")
	}
}

func TestBuildTmuxInfoMessage(t *testing.T) {
	msg := buildTmuxInfoMessage("session", true)
	if !strings.Contains(msg, "switch-client") {
		t.Fatalf("expected switch-client message, got %q", msg)
	}
	msg = buildTmuxInfoMessage("session", false)
	if !strings.Contains(msg, "attach-session") {
		t.Fatalf("expected attach-session message, got %q", msg)
	}
}

func TestOpenTmuxSession(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	wt := &models.WorktreeInfo{Path: t.TempDir(), Branch: featureBranch}

	if cmd := m.openTmuxSession(nil, wt); cmd != nil {
		t.Fatal("expected nil command for nil tmux config")
	}

	badCfg := &config.TmuxCommand{SessionName: "session"}
	if msg := m.openTmuxSession(badCfg, wt)(); msg == nil {
		t.Fatal("expected error message for empty windows")
	}

	called := false
	m.commandRunner = func(_ string, _ ...string) *exec.Cmd {
		called = true
		return exec.Command("true")
	}
	m.execProcess = func(_ *exec.Cmd, cb tea.ExecCallback) tea.Cmd {
		return func() tea.Msg {
			return cb(nil)
		}
	}

	cfgGood := &config.TmuxCommand{
		SessionName: "session",
		Attach:      true,
		OnExists:    "switch",
		Windows:     []config.TmuxWindow{{Name: "shell"}},
	}
	cmd := m.openTmuxSession(cfgGood, wt)
	if cmd == nil {
		t.Fatal("expected tmux command")
	}
	msg := cmd()
	ready, ok := msg.(tmuxSessionReadyMsg)
	if !ok {
		t.Fatalf("expected tmuxSessionReadyMsg, got %T", msg)
	}
	if !called {
		t.Fatal("expected command runner to be called")
	}
	if ready.sessionName != "session" {
		t.Fatalf("unexpected session name: %q", ready.sessionName)
	}
	if !ready.attach {
		t.Fatal("expected attach to be true")
	}
}
