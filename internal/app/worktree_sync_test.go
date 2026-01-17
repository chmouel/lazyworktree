package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

func TestPushToUpstreamRunsGitPush(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	if err := os.MkdirAll(wtPath, 0o700); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	m.filteredWts = []*models.WorktreeInfo{
		{Path: wtPath, Branch: featureBranch, HasUpstream: true, UpstreamBranch: testUpstreamRef},
	}
	m.selectedIndex = 0

	var gotName string
	var gotArgs []string
	m.commandRunner = func(name string, args ...string) *exec.Cmd {
		gotName = name
		gotArgs = append([]string{}, args...)
		return exec.Command("printf", "")
	}

	_, cmd := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}
	if m.currentScreen != screenLoading {
		t.Fatalf("expected screenLoading, got %v", m.currentScreen)
	}
	if !m.loading || m.loadingScreen == nil {
		t.Fatal("expected loading screen to be set")
	}
	msg := cmd()
	pushMsg, ok := msg.(pushResultMsg)
	if !ok {
		t.Fatalf("expected pushResultMsg, got %T", msg)
	}
	if pushMsg.err != nil {
		t.Fatalf("unexpected push error: %v", pushMsg.err)
	}

	if gotName != testGitCmd {
		t.Fatalf("expected git command, got %q", gotName)
	}
	if len(gotArgs) < 1 || gotArgs[0] != testGitPushArg {
		t.Fatalf("expected git push args, got %v", gotArgs)
	}
	if len(gotArgs) < 3 || gotArgs[1] != testRemoteOrigin || gotArgs[2] != "HEAD:"+featureBranch {
		t.Fatalf("expected git push origin HEAD:%s, got %v", featureBranch, gotArgs)
	}
}

func TestPushToUpstreamPromptsForUpstream(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	if err := os.MkdirAll(wtPath, 0o700); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	m.filteredWts = []*models.WorktreeInfo{
		{Path: wtPath, Branch: featureBranch},
	}
	m.selectedIndex = 0

	_, cmd := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	if cmd == nil {
		t.Fatal("expected input command to be returned")
	}
	if m.currentScreen != screenInput {
		t.Fatalf("expected screenInput, got %v", m.currentScreen)
	}
	if m.inputScreen == nil {
		t.Fatal("expected inputScreen to be set")
	}
	if got := m.inputScreen.input.Value(); got != testUpstreamRef {
		t.Fatalf("expected default upstream %q, got %q", testUpstreamRef, got)
	}

	var gotName string
	var gotArgs []string
	m.commandRunner = func(name string, args ...string) *exec.Cmd {
		gotName = name
		gotArgs = append([]string{}, args...)
		return exec.Command("printf", "")
	}

	pushCmd, closeInput := m.inputSubmit(testUpstreamRef, false)
	if !closeInput {
		t.Fatal("expected input to close after submit")
	}
	if pushCmd == nil {
		t.Fatal("expected push command to be returned")
	}
	if m.currentScreen != screenLoading {
		t.Fatalf("expected screenLoading, got %v", m.currentScreen)
	}
	if !m.loading || m.loadingScreen == nil {
		t.Fatal("expected loading screen to be set")
	}
	msg := pushCmd()
	pushMsg, ok := msg.(pushResultMsg)
	if !ok {
		t.Fatalf("expected pushResultMsg, got %T", msg)
	}
	if pushMsg.err != nil {
		t.Fatalf("unexpected push error: %v", pushMsg.err)
	}

	if gotName != testGitCmd {
		t.Fatalf("expected git command, got %q", gotName)
	}
	if len(gotArgs) < 4 {
		t.Fatalf("expected git push -u args, got %v", gotArgs)
	}
	if gotArgs[0] != testGitPushArg {
		t.Fatalf("expected git push args, got %v", gotArgs)
	}
	if gotArgs[1] != "-u" || gotArgs[2] != testRemoteOrigin || gotArgs[3] != "HEAD:"+featureBranch {
		t.Fatalf("expected git push -u origin HEAD:%s, got %v", featureBranch, gotArgs)
	}
}

func TestPushToUpstreamBlocksWithLocalChanges(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	if err := os.MkdirAll(wtPath, 0o700); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	m.filteredWts = []*models.WorktreeInfo{
		{Path: wtPath, Branch: featureBranch, Dirty: true, Modified: 1},
	}
	m.selectedIndex = 0

	_, cmd := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	if cmd != nil {
		t.Fatal("expected no command when local changes exist")
	}
	if m.currentScreen != screenInfo {
		t.Fatalf("expected screenInfo, got %v", m.currentScreen)
	}
	if m.infoScreen == nil {
		t.Fatal("expected infoScreen to be set")
	}
	if !strings.Contains(m.infoScreen.message, "Cannot push") {
		t.Fatalf("unexpected info message: %q", m.infoScreen.message)
	}
}

func TestPushToUpstreamRejectsOtherBranch(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	if err := os.MkdirAll(wtPath, 0o700); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	m.filteredWts = []*models.WorktreeInfo{
		{Path: wtPath, Branch: featureBranch},
	}
	m.selectedIndex = 0

	_, cmd := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	if cmd == nil {
		t.Fatal("expected input command to be returned")
	}
	if m.currentScreen != screenInput {
		t.Fatalf("expected screenInput, got %v", m.currentScreen)
	}

	pushCmd, closeInput := m.inputSubmit(testOtherBranch, false)
	if closeInput {
		t.Fatal("expected input to remain open on invalid branch")
	}
	if pushCmd != nil {
		t.Fatal("expected no command on invalid branch")
	}
	if m.inputScreen == nil || !strings.Contains(m.inputScreen.errorMsg, "Upstream branch must match") {
		t.Fatalf("expected validation error, got %q", m.inputScreen.errorMsg)
	}
}

func TestPushToUpstreamRejectsConfiguredOtherBranch(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	if err := os.MkdirAll(wtPath, 0o700); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	m.filteredWts = []*models.WorktreeInfo{
		{Path: wtPath, Branch: featureBranch, HasUpstream: true, UpstreamBranch: testOtherBranch},
	}
	m.selectedIndex = 0

	_, cmd := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	if cmd != nil {
		t.Fatal("expected no command when upstream is for another branch")
	}
	if m.currentScreen != screenInfo {
		t.Fatalf("expected screenInfo, got %v", m.currentScreen)
	}
	if m.infoScreen == nil || !strings.Contains(m.infoScreen.message, "does not match current branch") {
		t.Fatalf("unexpected info message: %q", m.infoScreen.message)
	}
}

func TestSyncWithUpstreamRunsPullThenPush(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		MergeMethod: "merge",
	}
	m := NewModel(cfg, "")

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	if err := os.MkdirAll(wtPath, 0o700); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	m.filteredWts = []*models.WorktreeInfo{
		{Path: wtPath, Branch: featureBranch, HasUpstream: true, UpstreamBranch: testUpstreamRef},
	}
	m.selectedIndex = 0

	type call struct {
		name string
		args []string
	}
	var calls []call
	m.commandRunner = func(name string, args ...string) *exec.Cmd {
		calls = append(calls, call{name: name, args: append([]string{}, args...)})
		return exec.Command("printf", "")
	}

	_, cmd := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}
	if m.currentScreen != screenLoading {
		t.Fatalf("expected screenLoading, got %v", m.currentScreen)
	}
	if !m.loading || m.loadingScreen == nil {
		t.Fatal("expected loading screen to be set")
	}
	msg := cmd()
	syncMsg, ok := msg.(syncResultMsg)
	if !ok {
		t.Fatalf("expected syncResultMsg, got %T", msg)
	}
	if syncMsg.err != nil {
		t.Fatalf("unexpected sync error: %v", syncMsg.err)
	}

	if len(calls) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(calls))
	}
	if calls[0].name != testGitCmd || len(calls[0].args) < 3 || calls[0].args[0] != testGitPullArg {
		t.Fatalf("expected git pull with upstream first, got %v %v", calls[0].name, calls[0].args)
	}
	if calls[0].args[1] != testRemoteOrigin || calls[0].args[2] != featureBranch {
		t.Fatalf("expected git pull origin %s, got %v", featureBranch, calls[0].args)
	}
	if calls[1].name != testGitCmd || len(calls[1].args) < 1 || calls[1].args[0] != testGitPushArg {
		t.Fatalf("expected git push second, got %v %v", calls[1].name, calls[1].args)
	}
	if len(calls[1].args) < 3 || calls[1].args[1] != testRemoteOrigin || calls[1].args[2] != "HEAD:"+featureBranch {
		t.Fatalf("expected git push origin HEAD:%s, got %v", featureBranch, calls[1].args)
	}
}

func TestSyncWithUpstreamPromptsForUpstream(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		MergeMethod: "merge",
	}
	m := NewModel(cfg, "")

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	if err := os.MkdirAll(wtPath, 0o700); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	m.filteredWts = []*models.WorktreeInfo{
		{Path: wtPath, Branch: featureBranch},
	}
	m.selectedIndex = 0

	_, cmd := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
	if cmd == nil {
		t.Fatal("expected input command to be returned")
	}
	if m.currentScreen != screenInput {
		t.Fatalf("expected screenInput, got %v", m.currentScreen)
	}
	if m.inputScreen == nil {
		t.Fatal("expected inputScreen to be set")
	}
	if got := m.inputScreen.input.Value(); got != testUpstreamRef {
		t.Fatalf("expected default upstream %q, got %q", testUpstreamRef, got)
	}

	type call struct {
		name string
		args []string
	}
	var calls []call
	m.commandRunner = func(name string, args ...string) *exec.Cmd {
		calls = append(calls, call{name: name, args: append([]string{}, args...)})
		return exec.Command("printf", "")
	}

	syncCmd, closeInput := m.inputSubmit(testUpstreamRef, false)
	if !closeInput {
		t.Fatal("expected input to close after submit")
	}
	if syncCmd == nil {
		t.Fatal("expected sync command to be returned")
	}
	if m.currentScreen != screenLoading {
		t.Fatalf("expected screenLoading, got %v", m.currentScreen)
	}
	if !m.loading || m.loadingScreen == nil {
		t.Fatal("expected loading screen to be set")
	}
	msg := syncCmd()
	syncMsg, ok := msg.(syncResultMsg)
	if !ok {
		t.Fatalf("expected syncResultMsg, got %T", msg)
	}
	if syncMsg.err != nil {
		t.Fatalf("unexpected sync error: %v", syncMsg.err)
	}

	if len(calls) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(calls))
	}
	if calls[0].name != testGitCmd || len(calls[0].args) < 3 || calls[0].args[0] != testGitPullArg {
		t.Fatalf("expected git pull with upstream, got %v %v", calls[0].name, calls[0].args)
	}
	if calls[0].args[1] != testRemoteOrigin || calls[0].args[2] != featureBranch {
		t.Fatalf("expected git pull origin feature, got %v", calls[0].args)
	}
	if calls[1].name != testGitCmd || len(calls[1].args) < 4 || calls[1].args[0] != testGitPushArg {
		t.Fatalf("expected git push with upstream, got %v %v", calls[1].name, calls[1].args)
	}
	if calls[1].args[1] != "-u" || calls[1].args[2] != testRemoteOrigin || calls[1].args[3] != "HEAD:"+featureBranch {
		t.Fatalf("expected git push -u origin HEAD:%s, got %v", featureBranch, calls[1].args)
	}
}

func TestSyncWithUpstreamBlocksWithLocalChanges(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		MergeMethod: "merge",
	}
	m := NewModel(cfg, "")

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	if err := os.MkdirAll(wtPath, 0o700); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	m.filteredWts = []*models.WorktreeInfo{
		{Path: wtPath, Branch: featureBranch, Dirty: true, Modified: 1},
	}
	m.selectedIndex = 0

	_, cmd := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
	if cmd != nil {
		t.Fatal("expected no command when local changes exist")
	}
	if m.currentScreen != screenInfo {
		t.Fatalf("expected screenInfo, got %v", m.currentScreen)
	}
	if m.infoScreen == nil {
		t.Fatal("expected infoScreen to be set")
	}
	if !strings.Contains(m.infoScreen.message, "Cannot synchronise") {
		t.Fatalf("unexpected info message: %q", m.infoScreen.message)
	}
}

func TestSyncWithUpstreamRejectsConfiguredOtherBranch(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		MergeMethod: "merge",
	}
	m := NewModel(cfg, "")

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	if err := os.MkdirAll(wtPath, 0o700); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	m.filteredWts = []*models.WorktreeInfo{
		{Path: wtPath, Branch: featureBranch, HasUpstream: true, UpstreamBranch: testOtherBranch},
	}
	m.selectedIndex = 0

	_, cmd := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
	if cmd != nil {
		t.Fatal("expected no command when upstream is for another branch")
	}
	if m.currentScreen != screenInfo {
		t.Fatalf("expected screenInfo, got %v", m.currentScreen)
	}
	if m.infoScreen == nil || !strings.Contains(m.infoScreen.message, "does not match current branch") {
		t.Fatalf("unexpected info message: %q", m.infoScreen.message)
	}
}

func TestSyncWithUpstreamUsesRebasePull(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		MergeMethod: mergeMethodRebase,
	}
	m := NewModel(cfg, "")

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	if err := os.MkdirAll(wtPath, 0o700); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	m.filteredWts = []*models.WorktreeInfo{
		{Path: wtPath, Branch: featureBranch, HasUpstream: true, UpstreamBranch: testUpstreamRef},
	}
	m.selectedIndex = 0

	type call struct {
		name string
		args []string
	}
	var calls []call
	m.commandRunner = func(name string, args ...string) *exec.Cmd {
		calls = append(calls, call{name: name, args: append([]string{}, args...)})
		return exec.Command("printf", "")
	}

	_, cmd := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}
	_ = cmd()

	if len(calls) < 1 {
		t.Fatal("expected at least one command")
	}
	// Check that pull has --rebase flag
	foundRebase := false
	for _, arg := range calls[0].args {
		if arg == pullRebaseFlag {
			foundRebase = true
			break
		}
	}
	if !foundRebase {
		t.Fatalf("expected --rebase flag in pull args when MergeMethod is rebase, got %v", calls[0].args)
	}
}

func TestSyncWithNoPRDoesNormalSync(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	if err := os.MkdirAll(wtPath, 0o700); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	m.filteredWts = []*models.WorktreeInfo{
		{
			Path:           wtPath,
			Branch:         featureBranch,
			HasUpstream:    true,
			UpstreamBranch: testUpstreamRef,
			PR:             nil, // No PR
		},
	}
	m.selectedIndex = 0

	type call struct {
		name string
		args []string
	}
	var calls []call
	m.commandRunner = func(name string, args ...string) *exec.Cmd {
		calls = append(calls, call{name: name, args: append([]string{}, args...)})
		return exec.Command("printf", "")
	}

	_, cmd := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
	if cmd == nil {
		t.Fatal("expected sync command to be returned")
	}

	_ = cmd()

	// Should do normal sync without checking if behind
	if len(calls) != 2 {
		t.Fatalf("expected 2 commands (pull+push), got %d", len(calls))
	}
}

func TestSyncPREmptyBaseBranchDoesNormalSync(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	if err := os.MkdirAll(wtPath, 0o700); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	m.filteredWts = []*models.WorktreeInfo{
		{
			Path:           wtPath,
			Branch:         featureBranch,
			HasUpstream:    true,
			UpstreamBranch: testUpstreamRef,
			PR: &models.PRInfo{
				Number:     123,
				BaseBranch: "", // Empty base branch
			},
		},
	}
	m.selectedIndex = 0

	type call struct {
		name string
		args []string
	}
	var calls []call
	m.commandRunner = func(name string, args ...string) *exec.Cmd {
		calls = append(calls, call{name: name, args: append([]string{}, args...)})
		return exec.Command("printf", "")
	}

	_, cmd := m.handleBuiltInKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
	if cmd == nil {
		t.Fatal("expected sync command to be returned")
	}

	_ = cmd()

	// Should do normal sync
	if len(calls) != 2 {
		t.Fatalf("expected 2 commands (pull+push), got %d", len(calls))
	}
}

func TestUpdateFromBaseWithRebaseFlag(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		MergeMethod: mergeMethodRebase,
	}
	m := NewModel(cfg, "")

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	if err := os.MkdirAll(wtPath, 0o700); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	wt := &models.WorktreeInfo{
		Path:   wtPath,
		Branch: featureBranch,
		PR: &models.PRInfo{
			Number:     123,
			BaseBranch: "main",
		},
	}

	var gotName string
	var gotArgs []string
	m.commandRunner = func(name string, args ...string) *exec.Cmd {
		gotName = name
		gotArgs = append([]string{}, args...)
		return exec.Command("printf", "")
	}

	cmd := m.updateFromBase(wt)
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}
	if m.currentScreen != screenLoading {
		t.Fatalf("expected screenLoading, got %v", m.currentScreen)
	}

	_ = cmd()

	if gotName != "gh" {
		t.Fatalf("expected gh command, got %q", gotName)
	}
	if len(gotArgs) < 3 || gotArgs[0] != "pr" || gotArgs[1] != "update-branch" {
		t.Fatalf("expected gh pr update-branch, got %v", gotArgs)
	}
	if len(gotArgs) < 3 || gotArgs[2] != "--rebase" {
		t.Fatalf("expected --rebase flag when merge_method is rebase, got %v", gotArgs)
	}
}

func TestUpdateFromBaseWithoutRebaseFlag(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		MergeMethod: "merge",
	}
	m := NewModel(cfg, "")

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	if err := os.MkdirAll(wtPath, 0o700); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	wt := &models.WorktreeInfo{
		Path:   wtPath,
		Branch: featureBranch,
		PR: &models.PRInfo{
			Number:     123,
			BaseBranch: "main",
		},
	}

	var gotName string
	var gotArgs []string
	m.commandRunner = func(name string, args ...string) *exec.Cmd {
		gotName = name
		gotArgs = append([]string{}, args...)
		return exec.Command("printf", "")
	}

	cmd := m.updateFromBase(wt)
	_ = cmd()

	if gotName != "gh" {
		t.Fatalf("expected gh command, got %q", gotName)
	}
	if len(gotArgs) != 2 || gotArgs[0] != "pr" || gotArgs[1] != "update-branch" {
		t.Fatalf("expected gh pr update-branch without --rebase, got %v", gotArgs)
	}
}

func TestShowSyncChoiceCreatesConfirmScreen(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	if err := os.MkdirAll(wtPath, 0o700); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	wt := &models.WorktreeInfo{
		Path:   wtPath,
		Branch: featureBranch,
		PR: &models.PRInfo{
			Number:     123,
			BaseBranch: "main",
		},
	}

	cmd := m.showSyncChoice(wt)
	if cmd != nil {
		t.Fatal("expected showSyncChoice to return nil (no immediate command)")
	}

	if m.currentScreen != screenConfirm {
		t.Fatalf("expected screenConfirm, got %v", m.currentScreen)
	}
	if m.confirmScreen == nil {
		t.Fatal("expected confirmScreen to be created")
	}
	if m.confirmAction == nil {
		t.Fatal("expected confirmAction to be set")
	}
	if m.confirmCancel == nil {
		t.Fatal("expected confirmCancel to be set")
	}
}

func TestConfirmYesCallsUpdateFromBase(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		MergeMethod: mergeMethodRebase,
	}
	m := NewModel(cfg, "")

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	if err := os.MkdirAll(wtPath, 0o700); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	wt := &models.WorktreeInfo{
		Path:   wtPath,
		Branch: featureBranch,
		PR: &models.PRInfo{
			Number:     123,
			BaseBranch: "main",
		},
	}

	// Set up confirm screen
	_ = m.showSyncChoice(wt)

	var gotName string
	var gotArgs []string
	m.commandRunner = func(name string, args ...string) *exec.Cmd {
		gotName = name
		gotArgs = append([]string{}, args...)
		return exec.Command("printf", "")
	}

	// Simulate user pressing YES (y key)
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = newModel.(*Model)

	if cmd == nil {
		t.Fatal("expected command to be returned from confirmAction")
	}
	_ = cmd()

	// Verify gh pr update-branch was called with --rebase
	if gotName != "gh" {
		t.Fatalf("expected gh command, got %q", gotName)
	}
	if len(gotArgs) < 3 || gotArgs[0] != "pr" || gotArgs[1] != "update-branch" || gotArgs[2] != "--rebase" {
		t.Fatalf("expected gh pr update-branch --rebase, got %v", gotArgs)
	}

	// Verify confirm screen was cleared
	if m.confirmScreen != nil {
		t.Fatal("expected confirmScreen to be cleared")
	}
	if m.confirmAction != nil {
		t.Fatal("expected confirmAction to be cleared")
	}
	if m.confirmCancel != nil {
		t.Fatal("expected confirmCancel to be cleared")
	}
}

func TestConfirmNoDoesNormalSync(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	if err := os.MkdirAll(wtPath, 0o700); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	wt := &models.WorktreeInfo{
		Path:           wtPath,
		Branch:         featureBranch,
		HasUpstream:    true,
		UpstreamBranch: testUpstreamRef,
		PR: &models.PRInfo{
			Number:     123,
			BaseBranch: "main",
		},
	}

	// Set up confirm screen
	_ = m.showSyncChoice(wt)

	type call struct {
		name string
		args []string
	}
	var calls []call
	m.commandRunner = func(name string, args ...string) *exec.Cmd {
		calls = append(calls, call{name: name, args: append([]string{}, args...)})
		return exec.Command("printf", "")
	}

	// Simulate user pressing NO (n key)
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	_ = newModel.(*Model)

	if cmd == nil {
		t.Fatal("expected command to be returned from confirmCancel")
	}
	_ = cmd()

	// Verify normal sync was performed (pull + push)
	if len(calls) != 2 {
		t.Fatalf("expected 2 commands (pull+push), got %d", len(calls))
	}
	if calls[0].name != testGitCmd || len(calls[0].args) < 1 || calls[0].args[0] != testGitPullArg {
		t.Fatalf("expected git pull, got %v %v", calls[0].name, calls[0].args)
	}
	if calls[1].name != testGitCmd || len(calls[1].args) < 1 || calls[1].args[0] != testGitPushArg {
		t.Fatalf("expected git push, got %v %v", calls[1].name, calls[1].args)
	}
}
