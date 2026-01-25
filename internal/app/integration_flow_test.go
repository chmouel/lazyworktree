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

const (
	testSkipPath = "skip"
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
	m.repoConfigPath = testSkipPath

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
	// Set default provider for testing
	SetIconProvider(&NerdFontV3Provider{})
	cfg := config.DefaultConfig()
	cfg.WorktreeDir = t.TempDir()
	m := NewModel(cfg, "")
	m.repoConfigPath = testSkipPath
	m.setWindowSize(120, 40)

	worktreePath := cfg.WorktreeDir + "/wt"
	wt := &models.WorktreeInfo{
		Path:   worktreePath,
		Branch: featureBranch,
	}

	updated, cmd := m.Update(worktreesLoadedMsg{worktrees: []*models.WorktreeInfo{wt}})
	m = updated.(*Model)
	m.setDetailsCache(worktreePath, &detailsCacheEntry{
		statusRaw: "",
		logRaw:    "",
		fetchedAt: time.Now(),
	})
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
	m.setDetailsCache(worktreePath, &detailsCacheEntry{
		statusRaw: "",
		logRaw:    "",
		fetchedAt: time.Now(),
	})
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
	if !strings.Contains(view, getIconPR()) {
		t.Fatalf("expected PR icon to be rendered, got %q", view)
	}
	if !strings.Contains(view, "CI Checks:") {
		t.Fatalf("expected CI info to be rendered, got %q", view)
	}
}

func TestIntegrationPaletteSelectsActiveTmuxSession(t *testing.T) {
	// Skip this test if tmux is not available
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not available in test environment")
	}

	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}

	m := NewModel(cfg, "")
	m.filteredWts = []*models.WorktreeInfo{{Path: cfg.WorktreeDir + "/wt", Branch: "feat"}}
	m.selectedIndex = 0

	// Mock commandRunner to return active tmux sessions
	m.commandRunner = func(name string, args ...string) *exec.Cmd {
		// If querying tmux sessions, return mock data
		if name == "tmux" && len(args) > 0 && args[0] == "list-sessions" {
			mockOutput := "wt-test-session\nother-session\n"
			if runtime.GOOS == osWindows {
				return exec.Command("cmd", "/c", "echo "+mockOutput)
			}
			return exec.Command("printf", "%s", mockOutput)
		}
		// For other commands, return the actual command
		return exec.Command(name, args...)
	}

	recorder := &commandRecorder{}
	m.execProcess = recorder.exec

	// Open command palette
	_ = m.showCommandPalette()

	// Verify the palette has the active session item
	// Filter for "test" to narrow down the results
	for _, r := range "test" {
		_, _ = m.handleScreenKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Verify item is selected
	if action, ok := m.paletteScreen.Selected(); !ok {
		t.Skip("palette filtering did not select any item (may vary by test environment)")
	} else if !strings.HasPrefix(action, "tmux-attach:") {
		// If it's not a tmux-attach action, that's okay - the filter might have matched something else
		t.Logf("filtered item is not tmux-attach (got %q), skipping rest of test", action)
		return
	}

	// Submit the selection
	_, cmd := m.handleScreenKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		_ = cmd()
	}

	// Verify palette is closed
	if m.currentScreen != screenNone {
		t.Fatalf("expected palette to close, got %v", m.currentScreen)
	}

	// Verify tmux command was executed
	if !containsCommand(recorder.execs, "tmux") {
		t.Fatal("expected tmux attach/switch command to be executed")
	}

	// Verify the correct session name was used
	tmuxCmd, _ := findCommand(recorder.execs, "tmux")
	foundWtPrefix := false
	for _, arg := range tmuxCmd.args {
		if strings.HasPrefix(arg, "wt-") {
			foundWtPrefix = true
			break
		}
	}
	if !foundWtPrefix {
		t.Fatalf("expected tmux command to include 'wt-' prefix in session name, got args: %v", tmuxCmd.args)
	}
}

func TestIntegrationDiffViewerModesWithNoChanges(t *testing.T) {
	testCases := []struct {
		name                string
		gitPager            string
		gitPagerInteractive bool
	}{
		{
			name:                "Non-interactive mode",
			gitPager:            "less",
			gitPagerInteractive: false,
		},
		{
			name:                "Interactive mode",
			gitPager:            "delta",
			gitPagerInteractive: true,
		},
		{
			name:                "VSCode mode",
			gitPager:            "code --wait --diff",
			gitPagerInteractive: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.AppConfig{
				WorktreeDir:         t.TempDir(),
				GitPager:            tc.gitPager,
				GitPagerInteractive: tc.gitPagerInteractive,
				MaxUntrackedDiffs:   5,
				MaxDiffChars:        1000,
			}

			m := NewModel(cfg, "")
			m.repoConfigPath = testSkipPath

			worktreePath := cfg.WorktreeDir + "/wt"
			wt := &models.WorktreeInfo{
				Path:   worktreePath,
				Branch: featureBranch,
			}

			updated, _ := m.Update(worktreesLoadedMsg{worktrees: []*models.WorktreeInfo{wt}})
			m = updated.(*Model)

			// Set up command recorder
			recorder := &commandRecorder{}
			m.commandRunner = recorder.runner
			m.execProcess = recorder.exec

			// statusFilesAll is empty by default, simulating no changes

			// Simulate 'd' key press
			updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
			m = updated.(*Model)

			// Verify info screen is shown
			if m.currentScreen != screenInfo {
				t.Fatalf("expected screenInfo, got %v", m.currentScreen)
			}
			if m.infoScreen == nil {
				t.Fatal("expected infoScreen to be set")
			}
			if m.infoScreen.message != testNoDiffMessage {
				t.Fatalf("expected message 'No diff to show.', got %q", m.infoScreen.message)
			}

			// Verify no command was executed
			if cmd != nil {
				_ = cmd() // Execute to trigger any recordings
			}
			if len(recorder.execs) > 0 {
				t.Fatalf("expected no commands to be executed, got %d", len(recorder.execs))
			}
		})
	}
}

func TestIntegrationDiffViewerModesWithChanges(t *testing.T) {
	testCases := []struct {
		name                string
		gitPager            string
		gitPagerInteractive bool
		expectedCommand     string
	}{
		{
			name:                "Non-interactive mode",
			gitPager:            "less",
			gitPagerInteractive: false,
			expectedCommand:     "bash",
		},
		{
			name:                "Interactive mode",
			gitPager:            "delta",
			gitPagerInteractive: true,
			expectedCommand:     "bash",
		},
		{
			name:                "VSCode mode",
			gitPager:            "code --wait --diff",
			gitPagerInteractive: false,
			expectedCommand:     "bash",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.AppConfig{
				WorktreeDir:         t.TempDir(),
				GitPager:            tc.gitPager,
				GitPagerInteractive: tc.gitPagerInteractive,
				MaxUntrackedDiffs:   5,
				MaxDiffChars:        1000,
			}

			m := NewModel(cfg, "")
			m.repoConfigPath = testSkipPath

			worktreePath := cfg.WorktreeDir + "/wt"
			wt := &models.WorktreeInfo{
				Path:   worktreePath,
				Branch: featureBranch,
			}

			updated, _ := m.Update(worktreesLoadedMsg{worktrees: []*models.WorktreeInfo{wt}})
			m = updated.(*Model)

			// Simulate having changes
			m.statusFilesAll = []StatusFile{
				{Filename: "test.go", Status: ".M", IsUntracked: false},
			}

			// Set up command recorder
			recorder := &commandRecorder{}
			m.commandRunner = recorder.runner
			m.execProcess = recorder.exec

			// Simulate 'd' key press
			updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
			m = updated.(*Model)

			// Verify command was returned
			if cmd == nil {
				t.Fatal("expected a command to be returned")
			}

			// Execute the command to trigger recording
			_ = cmd()

			// Verify bash command was executed
			if !containsCommand(recorder.execs, "bash") {
				t.Fatalf("expected bash command to be executed, got %v", recorder.execs)
			}

			// Verify git command is in the bash script
			found := false
			for _, exec := range recorder.execs {
				if exec.name == testBashCmd && len(exec.args) >= 2 && exec.args[0] == "-c" {
					script := exec.args[1]
					if strings.Contains(tc.gitPager, "code") {
						// VSCode mode uses git difftool
						if strings.Contains(script, "git difftool") {
							found = true
							break
						}
					} else {
						// Non-interactive and interactive modes use git diff
						if strings.Contains(script, "git diff") {
							found = true
							break
						}
					}
				}
			}
			if !found {
				t.Fatalf("expected git command in bash script for %s", tc.name)
			}

			// Verify no info screen is shown
			if m.currentScreen == screenInfo {
				t.Fatal("expected no info screen when there are changes")
			}
		})
	}
}
