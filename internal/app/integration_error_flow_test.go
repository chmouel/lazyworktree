package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

func TestIntegrationOpenPRsErrors(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	updated, _ := m.Update(openPRsLoadedMsg{err: errors.New("boom")})
	m = updated.(*Model)
	if m.currentScreen != screenInfo {
		t.Fatalf("expected info screen, got %v", m.currentScreen)
	}
	if m.infoScreen == nil || !strings.Contains(m.infoScreen.message, "Failed to fetch PRs") {
		t.Fatalf("expected fetch error modal, got %#v", m.infoScreen)
	}

	m.currentScreen = screenNone
	m.infoScreen = nil

	updated, _ = m.Update(openPRsLoadedMsg{prs: []*models.PRInfo{}})
	m = updated.(*Model)
	if m.currentScreen != screenInfo {
		t.Fatalf("expected info screen, got %v", m.currentScreen)
	}
	if m.infoScreen == nil || !strings.Contains(m.infoScreen.message, "No open PRs") {
		t.Fatalf("unexpected info modal: %#v", m.infoScreen)
	}
}

func TestIntegrationCreateFromPRValidationErrors(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.view.WindowWidth = 120
	m.view.WindowHeight = 40

	missingBranch := &models.PRInfo{Number: 1, Title: "Add feature"}
	updated, _ := m.Update(openPRsLoadedMsg{prs: []*models.PRInfo{missingBranch}})
	m = updated.(*Model)
	if !m.screenManager.IsActive() || m.screenManager.Type() != screen.TypePRSelect {
		t.Fatalf("expected PR selection screen, got active=%v type=%v", m.screenManager.IsActive(), m.screenManager.Type())
	}

	prScreen := m.screenManager.Current().(*screen.PRSelectionScreen)
	if prScreen == nil {
		t.Fatal("expected PRSelectionScreen to be set")
	}

	if prScreen.OnSelect == nil {
		t.Fatal("expected OnSelect callback to be set")
	}
	cmd := prScreen.OnSelect(missingBranch)
	if cmd != nil {
		updated, _ = m.Update(cmd())
		m = updated.(*Model)
	}

	if !m.screenManager.IsActive() || m.screenManager.Type() != screen.TypeInput {
		t.Fatal("expected input screen for PR selection")
	}
	inputScr := m.screenManager.Current().(*screen.InputScreen)
	inputScr.OnSubmit("pr1-add-feature", false)
	if inputScr.ErrorMsg != errPRBranchMissing {
		t.Fatalf("unexpected error: %q", inputScr.ErrorMsg)
	}

	withBranch := &models.PRInfo{Number: 2, Title: "Add tests", Branch: featureBranch}
	updated, _ = m.Update(openPRsLoadedMsg{prs: []*models.PRInfo{withBranch}})
	m = updated.(*Model)

	duplicateBranch := "my-feature"
	m.worktrees = []*models.WorktreeInfo{{Branch: duplicateBranch}}

	prScreen = m.screenManager.Current().(*screen.PRSelectionScreen)
	cmd = prScreen.OnSelect(withBranch)
	if cmd != nil {
		updated, _ = m.Update(cmd())
		m = updated.(*Model)
	}

	inputScr = m.screenManager.Current().(*screen.InputScreen)
	inputScr.OnSubmit(duplicateBranch, false)
	if !strings.Contains(inputScr.ErrorMsg, "already exists") {
		t.Fatalf("unexpected error: %q", inputScr.ErrorMsg)
	}

	existsBranch := "exists"
	if err := os.MkdirAll(filepath.Join(m.getRepoWorktreeDir(), existsBranch), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	m.worktrees = nil

	updated, _ = m.Update(openPRsLoadedMsg{prs: []*models.PRInfo{withBranch}})
	m = updated.(*Model)

	prScreen = m.screenManager.Current().(*screen.PRSelectionScreen)
	cmd = prScreen.OnSelect(withBranch)
	if cmd != nil {
		updated, _ = m.Update(cmd())
		m = updated.(*Model)
	}

	inputScr = m.screenManager.Current().(*screen.InputScreen)
	inputScr.OnSubmit(existsBranch, false)
	if !strings.Contains(inputScr.ErrorMsg, "Path already exists") {
		t.Fatalf("unexpected error: %q", inputScr.ErrorMsg)
	}
}

func TestIntegrationPRAndCIErrorPaths(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.loading = true
	m.worktrees = []*models.WorktreeInfo{{Branch: featureBranch}}

	updated, _ := m.Update(prDataLoadedMsg{err: errors.New("boom")})
	m = updated.(*Model)
	if m.loading {
		t.Fatal("expected loading to be false")
	}
	if m.prDataLoaded {
		t.Fatal("expected prDataLoaded to remain false")
	}
	if m.worktrees[0].PR != nil {
		t.Fatal("expected PR data to remain unset")
	}

	m.filteredWts = []*models.WorktreeInfo{{Branch: featureBranch}}
	m.selectedIndex = 0
	m.infoContent = "before"
	updated, _ = m.Update(ciStatusLoadedMsg{branch: featureBranch, err: errors.New("boom")})
	m = updated.(*Model)
	if m.infoContent != "before" {
		t.Fatalf("expected infoContent to remain unchanged, got %q", m.infoContent)
	}
	if _, ok := m.ciCache[featureBranch]; ok {
		t.Fatal("expected CI cache to remain empty on error")
	}
}
