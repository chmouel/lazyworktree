package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
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

	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: wtPath, Branch: featureBranch, HasUpstream: true, UpstreamBranch: testUpstreamRef},
	}
	m.state.data.selectedIndex = 0

	var gotName string
	var gotArgs []string
	m.commandRunner = func(_ context.Context, name string, args ...string) *exec.Cmd {
		if name == "git" && len(args) > 0 && args[0] == "worktree" {
			return exec.Command("printf", "")
		}
		gotName = name
		gotArgs = append([]string{}, args...)
		return exec.Command("printf", "")
	}

	_, cmd := m.handleBuiltInKey(tea.KeyPressMsg{Code: 'P', Text: string('P')})
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}
	if !m.loading {
		t.Fatal("expected loading to be true")
	}
	if m.state.ui.screenManager.Type() != appscreen.TypeLoading {
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

	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: wtPath, Branch: featureBranch},
	}
	m.state.data.selectedIndex = 0

	_, cmd := m.handleBuiltInKey(tea.KeyPressMsg{Code: 'P', Text: string('P')})
	if cmd == nil {
		t.Fatal("expected input command to be returned")
	}
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInput {
		t.Fatalf("expected input screen, got %v", m.state.ui.screenManager.Type())
	}
	inputScr := m.state.ui.screenManager.Current().(*appscreen.InputScreen)
	if got := inputScr.Input.Value(); got != testUpstreamRef {
		t.Fatalf("expected default upstream %q, got %q", testUpstreamRef, got)
	}

	var gotName string
	var gotArgs []string
	m.commandRunner = func(_ context.Context, name string, args ...string) *exec.Cmd {
		if name == "git" && len(args) > 0 && args[0] == "worktree" {
			return exec.Command("printf", "")
		}
		gotName = name
		gotArgs = append([]string{}, args...)
		return exec.Command("printf", "")
	}

	pushCmd := inputScr.OnSubmit(testUpstreamRef, false)
	if pushCmd == nil {
		t.Fatal("expected push command to be returned")
	}
	if !m.loading {
		t.Fatal("expected loading to be true")
	}
	if m.state.ui.screenManager.Type() != appscreen.TypeLoading {
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

	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: wtPath, Branch: featureBranch, Dirty: true, Modified: 1},
	}
	m.state.data.selectedIndex = 0

	_, cmd := m.handleBuiltInKey(tea.KeyPressMsg{Code: 'P', Text: string('P')})
	if cmd != nil {
		t.Fatal("expected no command when local changes exist")
	}
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Fatalf("expected info screen, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}
	infoScr := m.state.ui.screenManager.Current().(*appscreen.InfoScreen)
	if !strings.Contains(infoScr.Message, "Cannot push") {
		t.Fatalf("unexpected info message: %q", infoScr.Message)
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

	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: wtPath, Branch: featureBranch},
	}
	m.state.data.selectedIndex = 0

	_, cmd := m.handleBuiltInKey(tea.KeyPressMsg{Code: 'P', Text: string('P')})
	if cmd == nil {
		t.Fatal("expected input command to be returned")
	}
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInput {
		t.Fatalf("expected input screen, got %v", m.state.ui.screenManager.Type())
	}

	inputScr := m.state.ui.screenManager.Current().(*appscreen.InputScreen)
	pushCmd := inputScr.OnSubmit(testOtherBranch, false)
	if pushCmd != nil {
		t.Fatal("expected no command on invalid branch")
	}
	if !strings.Contains(inputScr.ErrorMsg, "Upstream branch must match") {
		t.Fatalf("expected validation error, got %q", inputScr.ErrorMsg)
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

	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: wtPath, Branch: featureBranch, HasUpstream: true, UpstreamBranch: testOtherBranch},
	}
	m.state.data.selectedIndex = 0

	_, cmd := m.handleBuiltInKey(tea.KeyPressMsg{Code: 'P', Text: string('P')})
	if cmd != nil {
		t.Fatal("expected no command when upstream is for another branch")
	}
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Fatalf("expected info screen, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}
	infoScr := m.state.ui.screenManager.Current().(*appscreen.InfoScreen)
	if !strings.Contains(infoScr.Message, "does not match current branch") {
		t.Fatalf("unexpected info message: %q", infoScr.Message)
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

	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: wtPath, Branch: featureBranch, HasUpstream: true, UpstreamBranch: testUpstreamRef},
	}
	m.state.data.selectedIndex = 0

	type call struct {
		name string
		args []string
	}
	var calls []call
	m.commandRunner = func(_ context.Context, name string, args ...string) *exec.Cmd {
		if name == "git" && len(args) > 0 && args[0] == "worktree" {
			return exec.Command("printf", "")
		}
		calls = append(calls, call{name: name, args: append([]string{}, args...)})
		return exec.Command("printf", "")
	}

	_, cmd := m.handleBuiltInKey(tea.KeyPressMsg{Code: 'S', Text: string('S')})
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}
	if !m.loading {
		t.Fatal("expected loading to be true")
	}
	if m.state.ui.screenManager.Type() != appscreen.TypeLoading {
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

	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: wtPath, Branch: featureBranch},
	}
	m.state.data.selectedIndex = 0

	_, cmd := m.handleBuiltInKey(tea.KeyPressMsg{Code: 'S', Text: string('S')})
	if cmd == nil {
		t.Fatal("expected input command to be returned")
	}
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInput {
		t.Fatalf("expected input screen, got %v", m.state.ui.screenManager.Type())
	}
	inputScr := m.state.ui.screenManager.Current().(*appscreen.InputScreen)
	if got := inputScr.Input.Value(); got != testUpstreamRef {
		t.Fatalf("expected default upstream %q, got %q", testUpstreamRef, got)
	}

	type call struct {
		name string
		args []string
	}
	var calls []call
	m.commandRunner = func(_ context.Context, name string, args ...string) *exec.Cmd {
		if name == "git" && len(args) > 0 && args[0] == "worktree" {
			return exec.Command("printf", "")
		}
		calls = append(calls, call{name: name, args: append([]string{}, args...)})
		return exec.Command("printf", "")
	}

	syncCmd := inputScr.OnSubmit(testUpstreamRef, false)
	if syncCmd == nil {
		t.Fatal("expected sync command to be returned")
	}
	if !m.loading {
		t.Fatal("expected loading to be true")
	}
	if m.state.ui.screenManager.Type() != appscreen.TypeLoading {
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

	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: wtPath, Branch: featureBranch, Dirty: true, Modified: 1},
	}
	m.state.data.selectedIndex = 0

	_, cmd := m.handleBuiltInKey(tea.KeyPressMsg{Code: 'S', Text: string('S')})
	if cmd != nil {
		t.Fatal("expected no command when local changes exist")
	}
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Fatalf("expected info screen, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}
	infoScr := m.state.ui.screenManager.Current().(*appscreen.InfoScreen)
	if !strings.Contains(infoScr.Message, "Cannot synchronise") {
		t.Fatalf("unexpected info message: %q", infoScr.Message)
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

	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: wtPath, Branch: featureBranch, HasUpstream: true, UpstreamBranch: testOtherBranch},
	}
	m.state.data.selectedIndex = 0

	_, cmd := m.handleBuiltInKey(tea.KeyPressMsg{Code: 'S', Text: string('S')})
	if cmd != nil {
		t.Fatal("expected no command when upstream is for another branch")
	}
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Fatalf("expected info screen, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}
	infoScr := m.state.ui.screenManager.Current().(*appscreen.InfoScreen)
	if !strings.Contains(infoScr.Message, "does not match current branch") {
		t.Fatalf("unexpected info message: %q", infoScr.Message)
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

	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: wtPath, Branch: featureBranch, HasUpstream: true, UpstreamBranch: testUpstreamRef},
	}
	m.state.data.selectedIndex = 0

	type call struct {
		name string
		args []string
	}
	var calls []call
	m.commandRunner = func(_ context.Context, name string, args ...string) *exec.Cmd {
		if name == "git" && len(args) > 0 && args[0] == "worktree" {
			return exec.Command("printf", "")
		}
		calls = append(calls, call{name: name, args: append([]string{}, args...)})
		return exec.Command("printf", "")
	}

	_, cmd := m.handleBuiltInKey(tea.KeyPressMsg{Code: 'S', Text: string('S')})
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

	m.state.data.filteredWts = []*models.WorktreeInfo{
		{
			Path:           wtPath,
			Branch:         featureBranch,
			HasUpstream:    true,
			UpstreamBranch: testUpstreamRef,
			PR:             nil, // No PR
		},
	}
	m.state.data.selectedIndex = 0

	type call struct {
		name string
		args []string
	}
	var calls []call
	m.commandRunner = func(_ context.Context, name string, args ...string) *exec.Cmd {
		if name == "git" && len(args) > 0 && args[0] == "worktree" {
			return exec.Command("printf", "")
		}
		calls = append(calls, call{name: name, args: append([]string{}, args...)})
		return exec.Command("printf", "")
	}

	_, cmd := m.handleBuiltInKey(tea.KeyPressMsg{Code: 'S', Text: string('S')})
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

	m.state.data.filteredWts = []*models.WorktreeInfo{
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
	m.state.data.selectedIndex = 0

	type call struct {
		name string
		args []string
	}
	var calls []call
	m.commandRunner = func(_ context.Context, name string, args ...string) *exec.Cmd {
		if name == "git" && len(args) > 0 && args[0] == "worktree" {
			return exec.Command("printf", "")
		}
		calls = append(calls, call{name: name, args: append([]string{}, args...)})
		return exec.Command("printf", "")
	}

	_, cmd := m.handleBuiltInKey(tea.KeyPressMsg{Code: 'S', Text: string('S')})
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
	m.commandRunner = func(_ context.Context, name string, args ...string) *exec.Cmd {
		if name == "git" && len(args) > 0 && args[0] == "worktree" {
			return exec.Command("printf", "")
		}
		gotName = name
		gotArgs = append([]string{}, args...)
		return exec.Command("printf", "")
	}

	cmd := m.updateFromBase(wt)
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}
	if m.state.ui.screenManager.Type() != appscreen.TypeLoading {
		t.Fatalf("expected loading screen, got %v", m.state.ui.screenManager.Type())
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
	m.commandRunner = func(_ context.Context, name string, args ...string) *exec.Cmd {
		if name == "git" && len(args) > 0 && args[0] == "worktree" {
			return exec.Command("printf", "")
		}
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

	if !m.state.ui.screenManager.IsActive() {
		t.Fatal("expected screen manager to be active")
	}
	if m.state.ui.screenManager.Type() != appscreen.TypeConfirm {
		t.Fatalf("expected confirm screen, got %v", m.state.ui.screenManager.Type())
	}
	confirmScreen, ok := m.state.ui.screenManager.Current().(*appscreen.ConfirmScreen)
	if !ok {
		t.Fatal("expected confirm screen in screen manager")
	}
	if confirmScreen.OnConfirm == nil {
		t.Fatal("expected OnConfirm to be set")
	}
	if confirmScreen.OnCancel == nil {
		t.Fatal("expected OnCancel to be set")
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
	m.commandRunner = func(_ context.Context, name string, args ...string) *exec.Cmd {
		if name == "git" && len(args) > 0 && args[0] == "worktree" {
			return exec.Command("printf", "")
		}
		gotName = name
		gotArgs = append([]string{}, args...)
		return exec.Command("printf", "")
	}

	// Simulate user pressing YES (y key)
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'y', Text: string('y')})

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

	// Screen manager should have popped the confirm screen after the callback
	// (In practice, the callback would be invoked by the Update handler which pops the screen)
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
	m.commandRunner = func(_ context.Context, name string, args ...string) *exec.Cmd {
		if name == "git" && len(args) > 0 && args[0] == "worktree" {
			return exec.Command("printf", "")
		}
		calls = append(calls, call{name: name, args: append([]string{}, args...)})
		return exec.Command("printf", "")
	}

	// Simulate user pressing NO (n key)
	newModel, cmd := m.Update(tea.KeyPressMsg{Code: 'n', Text: string('n')})
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
