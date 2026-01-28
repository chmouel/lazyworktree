package app

import (
	"strings"
	"testing"

	"github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

const (
	testNoDiffMessage = "No diff to show."
)

func TestShowDiffNonInteractiveNoDiff(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir:       t.TempDir(),
		MaxUntrackedDiffs: 0,
		MaxDiffChars:      1000,
	}
	m := NewModel(cfg, "")
	m.filteredWts = []*models.WorktreeInfo{{Path: cfg.WorktreeDir, Branch: featureBranch}}
	m.selectedIndex = 0
	// statusFilesAll is empty by default, simulating no changes

	// showDiff with no changes should now show an info screen
	cmd := m.showDiff()
	if cmd != nil {
		t.Fatal("expected no command when there are no changes")
	}

	// Verify info screen is shown
	if !m.screenManager.IsActive() {
		t.Fatal("expected screen manager to be active")
	}
	if m.screenManager.Type() != screen.TypeInfo {
		t.Fatalf("expected info screen, got %v", m.screenManager.Type())
	}
	infoScreen, ok := m.screenManager.Current().(*screen.InfoScreen)
	if !ok {
		t.Fatal("expected InfoScreen in screen manager")
	}
	if infoScreen.Message != testNoDiffMessage {
		t.Fatalf("expected message %q, got %q", testNoDiffMessage, infoScreen.Message)
	}
}

func TestShowDiffInteractiveNoDiff(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir:         t.TempDir(),
		GitPager:            "delta",
		GitPagerInteractive: true,
		MaxUntrackedDiffs:   0,
		MaxDiffChars:        1000,
	}
	m := NewModel(cfg, "")
	m.filteredWts = []*models.WorktreeInfo{{Path: cfg.WorktreeDir, Branch: featureBranch}}
	m.selectedIndex = 0
	// statusFilesAll is empty by default, simulating no changes

	cmd := m.showDiff()
	if cmd != nil {
		t.Fatal("expected no command when there are no changes in interactive mode")
	}

	if !m.screenManager.IsActive() {
		t.Fatal("expected screen manager to be active")
	}
	if m.screenManager.Type() != screen.TypeInfo {
		t.Fatalf("expected info screen, got %v", m.screenManager.Type())
	}
	infoScreen, ok := m.screenManager.Current().(*screen.InfoScreen)
	if !ok {
		t.Fatal("expected InfoScreen in screen manager")
	}
	if infoScreen.Message != testNoDiffMessage {
		t.Fatalf("expected message %q, got %q", testNoDiffMessage, infoScreen.Message)
	}
}

func TestShowDiffVSCodeNoDiff(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir:       t.TempDir(),
		GitPager:          "code --wait --diff",
		MaxUntrackedDiffs: 0,
		MaxDiffChars:      1000,
	}
	m := NewModel(cfg, "")
	m.filteredWts = []*models.WorktreeInfo{{Path: cfg.WorktreeDir, Branch: featureBranch}}
	m.selectedIndex = 0
	// statusFilesAll is empty by default, simulating no changes

	cmd := m.showDiff()
	if cmd != nil {
		t.Fatal("expected no command when there are no changes in VSCode mode")
	}

	if !m.screenManager.IsActive() {
		t.Fatal("expected screen manager to be active")
	}
	if m.screenManager.Type() != screen.TypeInfo {
		t.Fatalf("expected info screen, got %v", m.screenManager.Type())
	}
	infoScreen, ok := m.screenManager.Current().(*screen.InfoScreen)
	if !ok {
		t.Fatal("expected InfoScreen in screen manager")
	}
	if infoScreen.Message != testNoDiffMessage {
		t.Fatalf("expected message %q, got %q", testNoDiffMessage, infoScreen.Message)
	}
}

func TestShowDiffInteractiveWithChanges(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir:         t.TempDir(),
		GitPager:            "delta",
		GitPagerInteractive: true,
		MaxUntrackedDiffs:   5,
		MaxDiffChars:        1000,
	}
	m := NewModel(cfg, "")
	m.filteredWts = []*models.WorktreeInfo{{Path: cfg.WorktreeDir, Branch: featureBranch}}
	m.selectedIndex = 0

	// Simulate having changes
	m.statusFilesAll = []StatusFile{
		{Filename: "test.go", Status: ".M", IsUntracked: false},
	}

	// Set up command recorder to capture the execution
	recorder := &commandRecorder{}
	m.commandRunner = recorder.runner
	m.execProcess = recorder.exec

	cmd := m.showDiff()
	if cmd == nil {
		t.Fatal("expected a command when there are changes in interactive mode")
	}

	// Execute the command to trigger recording
	_ = cmd()

	// Verify that git diff was executed via bash
	if len(recorder.execs) == 0 {
		t.Fatal("expected at least one command to be executed")
	}

	found := false
	for _, exec := range recorder.execs {
		if exec.name == "bash" && len(exec.args) >= 2 && exec.args[0] == "-c" {
			if strings.Contains(exec.args[1], "git diff") {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("expected bash command containing 'git diff' to be executed")
	}
}

func TestShowDiffVSCodeWithChanges(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir:       t.TempDir(),
		GitPager:          "code --wait --diff",
		MaxUntrackedDiffs: 5,
		MaxDiffChars:      1000,
	}
	m := NewModel(cfg, "")
	m.filteredWts = []*models.WorktreeInfo{{Path: cfg.WorktreeDir, Branch: featureBranch}}
	m.selectedIndex = 0

	// Simulate having changes
	m.statusFilesAll = []StatusFile{
		{Filename: "test.go", Status: ".M", IsUntracked: false},
	}

	// Set up command recorder to capture the execution
	recorder := &commandRecorder{}
	m.commandRunner = recorder.runner
	m.execProcess = recorder.exec

	cmd := m.showDiff()
	if cmd == nil {
		t.Fatal("expected a command when there are changes in VSCode mode")
	}

	// Execute the command to trigger recording
	_ = cmd()

	// Verify that git difftool was executed via bash
	if len(recorder.execs) == 0 {
		t.Fatal("expected at least one command to be executed")
	}

	found := false
	for _, exec := range recorder.execs {
		if exec.name == "bash" && len(exec.args) >= 2 && exec.args[0] == "-c" {
			if strings.Contains(exec.args[1], "git difftool") {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("expected bash command containing 'git difftool' to be executed")
	}
}
