package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

func TestFetchRemotesCompleteTriggersRefresh(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.loading = true
	m.loadingScreen = NewLoadingScreen("Fetching remotes...", m.theme, m.config.IconsEnabled())

	_, cmd := m.Update(fetchRemotesCompleteMsg{})
	// loading stays true while refreshing worktrees
	if !m.loading {
		t.Fatal("expected loading to stay true during worktree refresh")
	}
	if m.statusContent != "Remotes fetched" {
		t.Fatalf("unexpected status: %q", m.statusContent)
	}
	// loading screen message should be updated to show refresh phase
	if m.loadingScreen == nil || m.loadingScreen.message != loadingRefreshWorktrees {
		t.Fatalf("expected loading screen message to be %q", loadingRefreshWorktrees)
	}
	if cmd == nil {
		t.Fatal("expected refresh command")
	}
	if msg := cmd(); msg == nil {
		t.Fatal("expected refresh command to return a message")
	}
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
	if !m.screenManager.IsActive() || m.screenManager.Type() != appscreen.TypeInfo {
		t.Fatalf("expected info screen, got active=%v type=%v", m.screenManager.IsActive(), m.screenManager.Type())
	}
	infoScr := m.screenManager.Current().(*appscreen.InfoScreen)
	if !strings.Contains(infoScr.Message, "Failed to fetch PRs") {
		t.Fatalf("unexpected info modal: %q", infoScr.Message)
	}

	m.screenManager.Pop()

	if cmd := m.handleOpenPRsLoaded(openPRsLoadedMsg{prs: []*models.PRInfo{}}); cmd != nil {
		t.Fatal("expected no command on empty list")
	}
	if !m.screenManager.IsActive() || m.screenManager.Type() != appscreen.TypeInfo {
		t.Fatalf("expected info screen, got active=%v type=%v", m.screenManager.IsActive(), m.screenManager.Type())
	}
	infoScr2 := m.screenManager.Current().(*appscreen.InfoScreen)
	if infoScr2.Message != "No open PRs/MRs found." {
		t.Fatalf("unexpected info modal: %q", infoScr2.Message)
	}

	m.screenManager.Pop()

	prs := []*models.PRInfo{{Number: 1, Title: "Test", Branch: featureBranch}}
	cmd := m.handleOpenPRsLoaded(openPRsLoadedMsg{prs: prs})
	if cmd == nil {
		t.Fatal("expected command for PR selection")
	}
	// Check screen manager instead of legacy currentScreen field
	if !m.screenManager.IsActive() || m.screenManager.Type() != appscreen.TypePRSelect {
		t.Fatalf("expected PR selection screen, got active=%v type=%v", m.screenManager.IsActive(), m.screenManager.Type())
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

func TestMaybeFetchCIStatus(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.filteredWts = []*models.WorktreeInfo{
		{Branch: featureBranch, PR: &models.PRInfo{Number: 1}},
	}
	m.selectedIndex = 0

	m.ciCache.Set(featureBranch, nil)
	if cmd := m.maybeFetchCIStatus(); cmd != nil {
		t.Fatal("expected no fetch when cache is fresh")
	}

	// Wait for cache to become stale (use a very short sleep to ensure it's stale)
	// Note: The IsFresh check uses time.Since(fetchedAt) < ciCacheTTL
	// Since we just set it, we need to wait or use a custom cache for testing
	// For simplicity, we'll just test that fresh cache blocks fetching
}

func TestMaybeFetchCIStatusNonPRBranch(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	// Branch without a PR
	m.filteredWts = []*models.WorktreeInfo{
		{Branch: featureBranch, Path: "/tmp/worktree", PR: nil},
	}
	m.selectedIndex = 0

	// Note: On a GitHub repo (like the test environment), maybeFetchCIStatus
	// will return a command to fetch CI by commit. On non-GitHub repos, it returns nil.
	// Either behaviour is valid depending on the test environment.

	// With cache set and fresh, should not fetch regardless of host
	m.ciCache.Set(featureBranch, nil)
	cmd := m.maybeFetchCIStatus()
	if cmd != nil {
		t.Fatal("expected no fetch when cache is fresh for non-PR branch")
	}

	// With stale cache, should return command on GitHub host (if detected)
	m.ciCache.Clear()
	// We don't check cmd here as it depends on whether the test runs in a GitHub repo
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
	if !m.screenManager.IsActive() || m.screenManager.Type() != appscreen.TypeInfo {
		t.Fatalf("expected info screen, got active=%v type=%v", m.screenManager.IsActive(), m.screenManager.Type())
	}

	// Reset for next test
	m.screenManager.Pop()

	_, cmd = m.handleAbsorbResult(absorbMergeResultMsg{path: "/tmp/wt", branch: featureBranch})
	if cmd == nil {
		t.Fatal("expected command for delete worktree")
	}
}

func TestWorktreeDeletedMsg(t *testing.T) {
	t.Run("success shows branch deletion prompt", func(t *testing.T) {
		cfg := &config.AppConfig{
			WorktreeDir: t.TempDir(),
		}
		m := NewModel(cfg, "")
		m.currentScreen = screenNone

		msg := worktreeDeletedMsg{
			path:   "/tmp/feat",
			branch: "feature-branch",
			err:    nil,
		}

		result, cmd := m.Update(msg)
		m = result.(*Model)

		if cmd != nil {
			t.Fatal("expected nil command")
		}
		if !m.screenManager.IsActive() || m.screenManager.Type() != appscreen.TypeConfirm {
			t.Fatal("expected confirm screen to be active")
		}
		confirmScreen, ok := m.screenManager.Current().(*appscreen.ConfirmScreen)
		if !ok {
			t.Fatal("expected confirm screen in screen manager")
		}
		if confirmScreen.OnConfirm == nil {
			t.Fatal("expected OnConfirm to be set")
		}
		if !strings.Contains(confirmScreen.Message, "Delete branch 'feature-branch'?") {
			t.Fatalf("unexpected message: %s", confirmScreen.Message)
		}
		if confirmScreen.SelectedButton != 0 {
			t.Fatalf("expected default button to be 0, got %d", confirmScreen.SelectedButton)
		}
	})

	t.Run("failure does not show branch deletion prompt", func(t *testing.T) {
		cfg := &config.AppConfig{
			WorktreeDir: t.TempDir(),
		}
		m := NewModel(cfg, "")
		m.currentScreen = screenNone

		msg := worktreeDeletedMsg{
			path:   "/tmp/feat",
			branch: "feature-branch",
			err:    fmt.Errorf("worktree deletion failed"),
		}

		result, cmd := m.Update(msg)
		m = result.(*Model)

		if cmd != nil {
			t.Fatal("expected nil command")
		}
		if m.currentScreen != screenNone {
			t.Fatalf("expected screen to remain unchanged, got %v", m.currentScreen)
		}
		if m.screenManager.IsActive() {
			t.Fatal("expected no screen for failed deletion")
		}
	})
}

func TestHandleCherryPickResultSuccess(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	msg := cherryPickResultMsg{
		commitSHA: "abc1234",
		targetWorktree: &models.WorktreeInfo{
			Path:   "/path/to/feature",
			Branch: "feature",
		},
		err: nil,
	}

	cmd := m.handleCherryPickResult(msg)
	if cmd != nil {
		t.Error("Expected nil command from handleCherryPickResult")
	}

	if !m.screenManager.IsActive() || m.screenManager.Type() != appscreen.TypeInfo {
		t.Errorf("Expected info screen, got active=%v type=%v", m.screenManager.IsActive(), m.screenManager.Type())
	}

	infoScr := m.screenManager.Current().(*appscreen.InfoScreen)
	if !strings.Contains(infoScr.Message, "Cherry-pick successful") {
		t.Errorf("Expected success message, got: %s", infoScr.Message)
	}

	if !strings.Contains(infoScr.Message, "abc1234") {
		t.Errorf("Expected commit SHA in message, got: %s", infoScr.Message)
	}
}

func TestHandleCherryPickResultError(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	msg := cherryPickResultMsg{
		commitSHA: "abc1234",
		targetWorktree: &models.WorktreeInfo{
			Path:   "/path/to/feature",
			Branch: "feature",
		},
		err: fmt.Errorf("cherry-pick conflicts occurred"),
	}

	cmd := m.handleCherryPickResult(msg)
	if cmd != nil {
		t.Error("Expected nil command from handleCherryPickResult")
	}

	if !m.screenManager.IsActive() || m.screenManager.Type() != appscreen.TypeInfo {
		t.Errorf("Expected info screen, got active=%v type=%v", m.screenManager.IsActive(), m.screenManager.Type())
	}

	infoScr := m.screenManager.Current().(*appscreen.InfoScreen)
	if !strings.Contains(infoScr.Message, "Cherry-pick failed") {
		t.Errorf("Expected failure message, got: %s", infoScr.Message)
	}

	if !strings.Contains(infoScr.Message, "conflicts occurred") {
		t.Errorf("Expected conflict error in message, got: %s", infoScr.Message)
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
	// TrustScreen is now managed by screenManager
	if !m.screenManager.IsActive() || m.screenManager.Type() != appscreen.TypeTrust {
		t.Fatalf("expected trust screen via screenManager, got %v", m.screenManager.Type())
	}
	if len(m.pending.Commands) != 1 {
		t.Fatalf("expected pending commands to be set, got %v", m.pending.Commands)
	}
}

func TestClearPendingTrust(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.pending.Commands = []string{"cmd"}
	m.pending.CommandEnv = map[string]string{"A": "1"}
	m.pending.CommandCwd = "/tmp"
	m.pending.After = func() tea.Msg { return nil }
	m.pending.TrustPath = "/tmp/.wt.yaml"
	// TrustScreen is now managed by screenManager
	m.screenManager.Push(appscreen.NewTrustScreen("/tmp/.wt.yaml", []string{"cmd"}, m.theme))

	m.clearPendingTrust()

	if m.pending.Commands != nil || m.pending.CommandEnv != nil || m.pending.CommandCwd != "" || m.pending.After != nil || m.pending.TrustPath != "" {
		t.Fatal("expected pending trust state to be cleared")
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
