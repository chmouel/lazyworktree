package app

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/utils"
)

func TestIntegrationOpenPRsErrors(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	updated, _ := m.Update(openPRsLoadedMsg{err: errors.New("boom")})
	m = updated.(*Model)
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Fatalf("expected info screen, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}
	infoScr := m.state.ui.screenManager.Current().(*appscreen.InfoScreen)
	if !strings.Contains(infoScr.Message, "Failed to fetch PRs") {
		t.Fatalf("expected fetch error modal, got %q", infoScr.Message)
	}

	m.state.ui.screenManager.Pop()

	updated, _ = m.Update(openPRsLoadedMsg{prs: []*models.PRInfo{}})
	m = updated.(*Model)
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Fatalf("expected info screen, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}
	infoScr2 := m.state.ui.screenManager.Current().(*appscreen.InfoScreen)
	if !strings.Contains(infoScr2.Message, "No open PRs") {
		t.Fatalf("unexpected info modal: %q", infoScr2.Message)
	}
}

func TestIntegrationCreateFromPRValidationErrors(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40
	m.state.services.git.SetCommandRunner(func(_ context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.Command("bash", "-lc", "exit 1")
	})

	missingBranch := &models.PRInfo{Number: 1, Title: "Add feature"}
	updated, _ := m.Update(openPRsLoadedMsg{prs: []*models.PRInfo{missingBranch}})
	m = updated.(*Model)
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypePRSelect {
		t.Fatalf("expected PR selection screen, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}

	prScreen, ok := m.state.ui.screenManager.Current().(*appscreen.PRSelectionScreen)
	if !ok || prScreen == nil {
		t.Fatal("expected PRSelectionScreen to be set")
	}

	if prScreen.OnSelect == nil {
		t.Fatal("expected OnSelect callback to be set")
	}
	cmd := prScreen.OnSelect(missingBranch)
	if cmd != nil {
		t.Fatal("expected missing-branch PR selection to fail immediately")
	}
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Fatal("expected info screen for missing PR branch")
	}
	infoScr := m.state.ui.screenManager.Current().(*appscreen.InfoScreen)
	if !strings.Contains(infoScr.Message, errPRBranchMissing) {
		t.Fatalf("unexpected error: %q", infoScr.Message)
	}

	withBranch := &models.PRInfo{Number: 2, Title: "Add tests", Branch: featureBranch}
	updated, _ = m.Update(openPRsLoadedMsg{prs: []*models.PRInfo{withBranch}})
	m = updated.(*Model)

	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: filepath.Join(cfg.WorktreeDir, "main"), Branch: "main", IsMain: true},
		{Path: filepath.Join(cfg.WorktreeDir, "attached"), Branch: featureBranch},
	}
	m.updateTable()

	prScreen = m.state.ui.screenManager.Current().(*appscreen.PRSelectionScreen)
	cmd = prScreen.OnSelect(withBranch)
	if cmd != nil {
		t.Fatal("expected attached-branch PR selection to fail immediately")
	}
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Fatal("expected info screen for attached PR branch")
	}
	infoScr = m.state.ui.screenManager.Current().(*appscreen.InfoScreen)
	if !strings.Contains(infoScr.Message, "already checked out") {
		t.Fatalf("unexpected error: %q", infoScr.Message)
	}

	worktreeName := utils.GeneratePRWorktreeName(withBranch, "pr-{number}-{title}", "")
	if err := os.MkdirAll(filepath.Join(m.getRepoWorktreeDir(), worktreeName), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	m.state.data.worktrees = nil

	updated, _ = m.Update(openPRsLoadedMsg{prs: []*models.PRInfo{withBranch}})
	m = updated.(*Model)

	prScreen = m.state.ui.screenManager.Current().(*appscreen.PRSelectionScreen)
	cmd = prScreen.OnSelect(withBranch)
	if cmd != nil {
		t.Fatal("expected existing-path PR selection to fail immediately")
	}
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Fatal("expected info screen for existing path")
	}
	infoScr = m.state.ui.screenManager.Current().(*appscreen.InfoScreen)
	if !strings.Contains(infoScr.Message, "Path already exists") {
		t.Fatalf("unexpected error: %q", infoScr.Message)
	}
}

func TestIntegrationPRAndCIErrorPaths(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.loading = true
	m.state.data.worktrees = []*models.WorktreeInfo{{Branch: featureBranch}}

	updated, _ := m.Update(prDataLoadedMsg{err: errors.New("boom")})
	m = updated.(*Model)
	if m.loading {
		t.Fatal("expected loading to be false")
	}
	if m.prDataLoaded {
		t.Fatal("expected prDataLoaded to remain false")
	}
	if m.state.data.worktrees[0].PR != nil {
		t.Fatal("expected PR data to remain unset")
	}

	m.state.data.filteredWts = []*models.WorktreeInfo{{Branch: featureBranch}}
	m.state.data.selectedIndex = 0
	m.infoContent = "before"
	updated, _ = m.Update(ciStatusLoadedMsg{branch: featureBranch, err: errors.New("boom")})
	m = updated.(*Model)
	if m.infoContent != "before" {
		t.Fatalf("expected infoContent to remain unchanged, got %q", m.infoContent)
	}
	if _, _, ok := m.cache.ciCache.Get(featureBranch); ok {
		t.Fatal("expected CI cache to remain empty on error")
	}
}
