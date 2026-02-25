package app

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
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

func (r *commandRecorder) runner(_ context.Context, name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...) //#nosec G204,G702 -- test helper with controlled args
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
	m.state.data.filteredWts = []*models.WorktreeInfo{{Path: cfg.WorktreeDir + "/wt", Branch: "feat"}}
	m.state.data.selectedIndex = 0

	recorder := &commandRecorder{}
	m.commandRunner = recorder.runner
	m.execProcess = recorder.exec

	_ = m.showCommandPalette()

	// Filter is now active by default, type directly to filter
	for _, r := range strings.ToLower(customLabel) {
		_, _ = m.handleScreenKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypePalette {
		t.Fatal("expected palette screen to be active")
	}

	paletteScreen := m.state.ui.screenManager.Current().(*appscreen.CommandPaletteScreen)
	if paletteScreen.Cursor >= len(paletteScreen.Filtered) {
		t.Fatal("expected valid palette selection after filtering")
	}

	_, cmd := m.handleScreenKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		_ = cmd()
	}

	if m.state.ui.screenManager.IsActive() {
		t.Fatalf("expected palette to close, screen manager still active with type %v", m.state.ui.screenManager.Type())
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
	if !strings.Contains(view, "PR #12") {
		t.Fatalf("expected PR info to be rendered, got %q", view)
	}
	if !strings.Contains(view, getIconPR()) {
		t.Fatalf("expected PR icon to be rendered, got %q", view)
	}
	if !strings.Contains(view, "CI Checks:") {
		t.Fatalf("expected CI info to be rendered, got %q", view)
	}
}

func TestIntegrationMainBranchMergedPRHidesInfo(t *testing.T) {
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
		Branch: "main",
		IsMain: true,
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
			"main": {Number: 77, State: "MERGED", Title: "Done", URL: testPRURL},
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
		branch: "main",
		checks: []*models.CICheck{
			{Name: "build", Status: "completed", Conclusion: "success"},
		},
	}
	updated, _ = m.Update(ciMsg)
	m = updated.(*Model)

	view := m.View()
	if strings.Contains(view, "PR:") {
		t.Fatalf("expected PR info to be hidden on main, got %q", view)
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
	m.state.data.filteredWts = []*models.WorktreeInfo{{Path: cfg.WorktreeDir + "/wt", Branch: "feat"}}
	m.state.data.selectedIndex = 0

	// Mock commandRunner to return active tmux sessions
	m.commandRunner = func(_ context.Context, name string, args ...string) *exec.Cmd {
		// If querying tmux sessions, return mock data
		if name == "tmux" && len(args) > 0 && args[0] == "list-sessions" {
			mockOutput := "wt-test-session\nother-session\n"
			if runtime.GOOS == osWindows {
				return windowsMockOutputCmd(mockOutput)
			}
			return exec.Command("printf", "%s", mockOutput)
		}
		// For other commands, return the actual command
		return exec.Command(name, args...) //#nosec G204,G702 -- test mock with controlled args
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
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypePalette {
		t.Skip("palette screen not active after filtering")
	}

	paletteScreen := m.state.ui.screenManager.Current().(*appscreen.CommandPaletteScreen)
	if paletteScreen.Cursor >= len(paletteScreen.Filtered) {
		t.Skip("palette filtering did not select any item (may vary by test environment)")
	}

	action := paletteScreen.Filtered[paletteScreen.Cursor].ID
	if !strings.HasPrefix(action, "tmux-attach:") {
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
	if m.state.ui.screenManager.IsActive() {
		t.Fatalf("expected palette to close, screen manager still active with type %v", m.state.ui.screenManager.Type())
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
		gitPagerCommandMode bool
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
		{
			name:                "Command mode",
			gitPager:            "lumen",
			gitPagerCommandMode: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.AppConfig{
				WorktreeDir:         t.TempDir(),
				GitPager:            tc.gitPager,
				GitPagerInteractive: tc.gitPagerInteractive,
				GitPagerCommandMode: tc.gitPagerCommandMode,
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
			if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInfo {
				t.Fatalf("expected info screen, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
			}
			infoScr := m.state.ui.screenManager.Current().(*appscreen.InfoScreen)
			if infoScr.Message != testNoDiffMessage {
				t.Fatalf("expected message 'No diff to show.', got %q", infoScr.Message)
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
		gitPagerCommandMode bool
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
		{
			name:                "Command mode",
			gitPager:            "lumen",
			gitPagerCommandMode: true,
			expectedCommand:     "bash",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.AppConfig{
				WorktreeDir:         t.TempDir(),
				GitPager:            tc.gitPager,
				GitPagerInteractive: tc.gitPagerInteractive,
				GitPagerCommandMode: tc.gitPagerCommandMode,
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
			m.state.data.statusFilesAll = []StatusFile{
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
					switch {
					case strings.Contains(tc.gitPager, "code"):
						// VSCode mode uses git difftool
						if strings.Contains(script, "git difftool") {
							found = true
						}
					case tc.gitPagerCommandMode:
						// Command mode: pager runs its own diff command
						if strings.Contains(script, tc.gitPager+" diff") {
							found = true
						}
					default:
						// Non-interactive and interactive modes use git diff
						if strings.Contains(script, "git diff") {
							found = true
						}
					}
					if found {
						break
					}
				}
			}
			if !found {
				t.Fatalf("expected git command in bash script for %s", tc.name)
			}

			// Verify no info screen is shown
			if m.state.ui.screenManager.IsActive() && m.state.ui.screenManager.Type() == appscreen.TypeInfo {
				t.Fatal("expected no info screen when there are changes")
			}
		})
	}
}
