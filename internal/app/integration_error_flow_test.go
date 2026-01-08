package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	m.windowWidth = 120
	m.windowHeight = 40

	missingBranch := &models.PRInfo{Number: 1, Title: "Add feature"}
	updated, _ := m.Update(openPRsLoadedMsg{prs: []*models.PRInfo{missingBranch}})
	m = updated.(*Model)
	if m.currentScreen != screenPRSelect {
		t.Fatalf("expected PR selection screen, got %v", m.currentScreen)
	}

	m.prSelectionSubmit(missingBranch)
	if m.currentScreen != screenInput || m.inputScreen == nil {
		t.Fatal("expected input screen for PR selection")
	}
	if _, ok := m.inputSubmit("pr1-add-feature"); ok {
		t.Fatal("expected missing branch validation to fail")
	}
	if m.inputScreen.errorMsg != errPRBranchMissing {
		t.Fatalf("unexpected error: %q", m.inputScreen.errorMsg)
	}

	withBranch := &models.PRInfo{Number: 2, Title: "Add tests", Branch: featureBranch}
	updated, _ = m.Update(openPRsLoadedMsg{prs: []*models.PRInfo{withBranch}})
	m = updated.(*Model)

	// Test that duplicate branch names are rejected (user enters a branch name that already exists)
	duplicateBranch := "my-feature"
	m.worktrees = []*models.WorktreeInfo{{Branch: duplicateBranch}}
	m.prSelectionSubmit(withBranch)
	if _, ok := m.inputSubmit(duplicateBranch); ok {
		t.Fatal("expected duplicate branch to be rejected")
	}
	if !strings.Contains(m.inputScreen.errorMsg, "already exists") {
		t.Fatalf("unexpected error: %q", m.inputScreen.errorMsg)
	}

	existsBranch := "exists"
	if err := os.MkdirAll(filepath.Join(m.getRepoWorktreeDir(), existsBranch), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	m.worktrees = nil
	m.prSelectionSubmit(withBranch)
	if _, ok := m.inputSubmit(existsBranch); ok {
		t.Fatal("expected existing path to be rejected")
	}
	if !strings.Contains(m.inputScreen.errorMsg, "Path already exists") {
		t.Fatalf("unexpected error: %q", m.inputScreen.errorMsg)
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
