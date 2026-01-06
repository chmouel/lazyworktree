package app

import (
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

type recordedCommand struct {
	name string
	args []string
	dir  string
}

type commandRecorder struct {
	execs  []recordedCommand
	starts []recordedCommand
}

func (r *commandRecorder) runner(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

func (r *commandRecorder) exec(cmd *exec.Cmd, _ tea.ExecCallback) tea.Cmd {
	r.execs = append(r.execs, recordedCommand{
		name: cmd.Args[0],
		args: append([]string{}, cmd.Args[1:]...),
		dir:  cmd.Dir,
	})
	return func() tea.Msg { return nil }
}

func (r *commandRecorder) start(cmd *exec.Cmd) error {
	r.starts = append(r.starts, recordedCommand{
		name: cmd.Args[0],
		args: append([]string{}, cmd.Args[1:]...),
		dir:  cmd.Dir,
	})
	return nil
}

func containsCommand(cmds []recordedCommand, name string) bool {
	for _, cmd := range cmds {
		if cmd.name == name {
			return true
		}
	}
	return false
}

func findCommand(cmds []recordedCommand, name string) (recordedCommand, bool) {
	for _, cmd := range cmds {
		if cmd.name == name {
			return cmd, true
		}
	}
	return recordedCommand{}, false
}

func TestIntegrationKeyBindingsTriggerCommands(t *testing.T) {
	const (
		customKey     = "x"
		customCommand = "echo ok"
	)

	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		CustomCommands: map[string]*config.CustomCommand{
			customKey: {
				Command: customCommand,
			},
		},
	}

	m := NewModel(cfg, "")
	m.repoConfigPath = "skip"

	worktreePath := cfg.WorktreeDir + "/wt"
	wt := &models.WorktreeInfo{
		Path:   worktreePath,
		Branch: featureBranch,
		PR: &models.PRInfo{
			URL: testPRURL,
		},
	}

	updated, _ := m.Update(worktreesLoadedMsg{worktrees: []*models.WorktreeInfo{wt}})
	m = updated.(*Model)

	recorder := &commandRecorder{}
	m.commandRunner = recorder.runner
	m.execProcess = recorder.exec
	m.startCommand = recorder.start

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if cmd != nil {
		_ = cmd()
	}

	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	if cmd != nil {
		_ = cmd()
	}

	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(customKey)})
	if cmd != nil {
		_ = cmd()
	}

	if !containsCommand(recorder.execs, "lazygit") {
		t.Fatalf("expected lazygit command to be executed, got %+v", recorder.execs)
	}
	if !containsCommand(recorder.execs, "bash") {
		t.Fatalf("expected bash command to be executed, got %+v", recorder.execs)
	}

	expectedOpen := "xdg-open"
	switch runtime.GOOS {
	case osDarwin:
		expectedOpen = "open"
	case osWindows:
		expectedOpen = "rundll32"
	}
	openCmd, ok := findCommand(recorder.starts, expectedOpen)
	if !ok {
		t.Fatalf("expected %q to be started, got %+v", expectedOpen, recorder.starts)
	}
	if runtime.GOOS == osWindows {
		if len(openCmd.args) < 2 || openCmd.args[1] != testPRURL {
			t.Fatalf("unexpected windows opener args: %+v", openCmd.args)
		}
	} else {
		if len(openCmd.args) != 1 || openCmd.args[0] != testPRURL {
			t.Fatalf("unexpected opener args: %+v", openCmd.args)
		}
	}
}

func TestIntegrationPaletteSelectsCustomCommand(t *testing.T) {
	const (
		customKey     = "x"
		customCommand = "echo run"
		customLabel   = "Run tests"
	)

	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		CustomCommands: map[string]*config.CustomCommand{
			customKey: {
				Command:     customCommand,
				Description: customLabel,
			},
		},
	}

	m := NewModel(cfg, "")
	m.filteredWts = []*models.WorktreeInfo{{Path: cfg.WorktreeDir + "/wt", Branch: "feat"}}
	m.selectedIndex = 0

	recorder := &commandRecorder{}
	m.commandRunner = recorder.runner
	m.execProcess = recorder.exec

	_ = m.showCommandPalette()

	for _, r := range strings.ToLower(customLabel) {
		_, _ = m.handleScreenKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	if _, ok := m.paletteScreen.Selected(); !ok {
		t.Fatal("expected palette selection after filtering")
	}

	_, cmd := m.handleScreenKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		_ = cmd()
	}

	if m.currentScreen != screenNone {
		t.Fatalf("expected palette to close, got %v", m.currentScreen)
	}
	if !containsCommand(recorder.execs, "bash") {
		t.Fatalf("expected bash command to be executed, got %+v", recorder.execs)
	}
}

func TestIntegrationPRAndCIFlowUpdatesView(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = t.TempDir()
	m := NewModel(cfg, "")
	m.repoConfigPath = "skip"
	m.setWindowSize(120, 40)

	worktreePath := cfg.WorktreeDir + "/wt"
	wt := &models.WorktreeInfo{
		Path:   worktreePath,
		Branch: featureBranch,
	}

	updated, cmd := m.Update(worktreesLoadedMsg{worktrees: []*models.WorktreeInfo{wt}})
	m = updated.(*Model)
	m.detailsCache[worktreePath] = &detailsCacheEntry{
		statusRaw: "",
		logRaw:    "",
		fetchedAt: time.Now(),
	}
	if cmd != nil {
		if msg := cmd(); msg != nil {
			updated, _ = m.Update(msg)
			m = updated.(*Model)
		}
	}

	prMsg := prDataLoadedMsg{
		prMap: map[string]*models.PRInfo{
			featureBranch: {Number: 12, State: "OPEN", Title: "Test", URL: testPRURL},
		},
	}
	updated, cmd = m.Update(prMsg)
	m = updated.(*Model)
	m.detailsCache[worktreePath] = &detailsCacheEntry{
		statusRaw: "",
		logRaw:    "",
		fetchedAt: time.Now(),
	}
	if cmd != nil {
		if msg := cmd(); msg != nil {
			updated, _ = m.Update(msg)
			m = updated.(*Model)
		}
	}

	ciMsg := ciStatusLoadedMsg{
		branch: featureBranch,
		checks: []*models.CICheck{
			{Name: "build", Status: "completed", Conclusion: "success"},
		},
	}
	updated, _ = m.Update(ciMsg)
	m = updated.(*Model)

	view := m.View()
	if !strings.Contains(view, "PR:") {
		t.Fatalf("expected PR info to be rendered, got %q", view)
	}
	if !strings.Contains(view, iconPR) {
		t.Fatalf("expected PR icon to be rendered, got %q", view)
	}
	if !strings.Contains(view, "CI Checks:") {
		t.Fatalf("expected CI info to be rendered, got %q", view)
	}
}
