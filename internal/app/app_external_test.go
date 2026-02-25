package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/multiplexer"
)

// windowsMockOutputCmd creates a Windows cmd that outputs multi-line mock data.
// Windows cmd /c echo doesn't interpret \n as newlines, so we chain echo commands with &.
func windowsMockOutputCmd(output string) *exec.Cmd {
	trimmed := strings.TrimRight(output, "\n")
	if trimmed == "" {
		return exec.Command("cmd", "/c", "echo.")
	}
	lines := strings.Split(trimmed, "\n")
	parts := make([]string, 0, len(lines))
	for _, line := range lines {
		parts = append(parts, "echo "+line)
	}
	// #nosec G204 -- test helper with controlled mock data
	return exec.Command("cmd", "/c", strings.Join(parts, "& "))
}

func TestGetTmuxActiveSessions(t *testing.T) {
	tests := []struct {
		name          string
		mockOutput    string
		mockErr       bool
		expectedNames []string
	}{
		{
			name:          "filters wt- sessions and strips prefix",
			mockOutput:    "wt-feature-branch\nother-session\nwt-bugfix\nwt-another-feature\n",
			expectedNames: []string{"another-feature", "bugfix", "feature-branch"}, // sorted
		},
		{
			name:          "handles no wt- sessions",
			mockOutput:    "session1\nsession2\nregular\n",
			expectedNames: nil,
		},
		{
			name:          "handles empty output",
			mockOutput:    "",
			expectedNames: nil,
		},
		{
			name:          "handles only whitespace",
			mockOutput:    "  \n  \n",
			expectedNames: nil,
		},
		{
			name:          "handles single wt- session",
			mockOutput:    "wt-test\n",
			expectedNames: []string{"test"},
		},
		{
			name:          "handles mixed whitespace",
			mockOutput:    "  wt-test1  \nother\n  wt-test2\n",
			expectedNames: []string{"test1", "test2"},
		},
		{
			name:          "command fails (tmux not running)",
			mockErr:       true,
			expectedNames: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.AppConfig{
				WorktreeDir:   t.TempDir(),
				SessionPrefix: "wt-",
			}
			m := NewModel(cfg, "")

			// Mock commandRunner
			m.commandRunner = func(_ context.Context, name string, args ...string) *exec.Cmd {
				if tt.mockErr {
					// Command that will fail
					return exec.Command("false")
				}
				if runtime.GOOS == osWindows {
					return windowsMockOutputCmd(tt.mockOutput)
				}
				// #nosec G204 -- test mock data, not user input
				return exec.Command("printf", "%s", tt.mockOutput)
			}

			got := m.getTmuxActiveSessions()

			if !reflect.DeepEqual(got, tt.expectedNames) {
				t.Fatalf("expected %v, got %v", tt.expectedNames, got)
			}
		})
	}
}

func TestGetTmuxActiveSessionsWithCustomPrefix(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir:   t.TempDir(),
		SessionPrefix: "my-prefix-",
	}
	m := NewModel(cfg, "")

	// Mock commandRunner to return sessions with custom prefix
	m.commandRunner = func(_ context.Context, name string, args ...string) *exec.Cmd {
		mockOutput := "my-prefix-feature\nother-session\nmy-prefix-bugfix\n"
		if runtime.GOOS == osWindows {
			return windowsMockOutputCmd(mockOutput)
		}
		return exec.Command("printf", "%s", mockOutput)
	}

	got := m.getTmuxActiveSessions()
	expected := []string{"bugfix", "feature"}

	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("expected %v, got %v", expected, got)
	}
}

func TestTmuxSessionReadyAttachesDirectly(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)

	updated, cmd := m.Update(tmuxSessionReadyMsg{sessionName: "wt_test", attach: true, insideTmux: false})
	model := updated.(*Model)
	if model.state.ui.screenManager.IsActive() {
		t.Fatalf("expected no screen change, got %v", model.state.ui.screenManager.Type())
	}
	if cmd == nil {
		t.Fatal("expected attach command to be returned")
	}
}

func TestTmuxSessionReadyShowsInfoWhenNotAttaching(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)

	updated, cmd := m.Update(tmuxSessionReadyMsg{sessionName: "wt_test", attach: false, insideTmux: false})
	model := updated.(*Model)
	if !model.state.ui.screenManager.IsActive() || model.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Fatalf("expected info screen, got active=%v type=%v", model.state.ui.screenManager.IsActive(), model.state.ui.screenManager.Type())
	}
	infoScr := model.state.ui.screenManager.Current().(*appscreen.InfoScreen)
	if cmd != nil {
		t.Fatal("expected no command when not attaching")
	}
	if !strings.Contains(infoScr.Message, "tmux attach-session -t 'wt_test'") {
		t.Errorf("expected attach message, got %q", infoScr.Message)
	}
}

func TestZellijSessionReadyAttachesDirectly(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)

	updated, cmd := m.Update(zellijSessionReadyMsg{sessionName: "wt_test", attach: true, insideZellij: false})
	model := updated.(*Model)
	if model.state.ui.screenManager.IsActive() {
		t.Fatalf("expected no screen change, got %v", model.state.ui.screenManager.Type())
	}
	if cmd == nil {
		t.Fatal("expected attach command to be returned")
	}
}

func TestZellijPaneCreatedShowsInfo(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)

	updated, cmd := m.Update(zellijPaneCreatedMsg{sessionName: "my-session", direction: "right"})
	model := updated.(*Model)
	if !model.state.ui.screenManager.IsActive() || model.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Fatalf("expected info screen, got active=%v type=%v", model.state.ui.screenManager.IsActive(), model.state.ui.screenManager.Type())
	}
	infoScr := model.state.ui.screenManager.Current().(*appscreen.InfoScreen)
	if cmd != nil {
		t.Fatal("expected no command after pane creation")
	}
	if !strings.Contains(infoScr.Message, "my-session") || !strings.Contains(infoScr.Message, "right") {
		t.Errorf("expected pane created message with session and direction, got %q", infoScr.Message)
	}
}

func TestBuildZellijScriptAddsLayoutsAsTabs(t *testing.T) {
	cfg := &config.TmuxCommand{
		SessionName: "session",
		OnExists:    "",
	}
	script := buildZellijScript("session", cfg, []string{"/tmp/layout1", "/tmp/layout2"})
	if !strings.Contains(script, "zellij attach --create-background \"$session\"") {
		t.Fatalf("expected session creation without layout flag, got %q", script)
	}
	if strings.Count(script, "new-tab --layout") != 2 {
		t.Fatalf("expected all layouts added as new tabs, got %q", script)
	}
	if !strings.Contains(script, "new-tab --layout '/tmp/layout1'") || !strings.Contains(script, "new-tab --layout '/tmp/layout2'") {
		t.Fatalf("expected both layouts as new-tab actions, got %q", script)
	}
	if !strings.Contains(script, "while ! zellij list-sessions --no-formatting 2>/dev/null | grep -v EXITED | sed 's/ \\[.*//' | grep -Fxq \"$session\"; do") {
		t.Fatalf("expected wait loop for session readiness, got %q", script)
	}
	if !strings.Contains(script, "if [ $tries -ge 50 ]") {
		t.Fatalf("expected timeout in wait loop, got %q", script)
	}
	if !strings.Contains(script, "go-to-tab 1") || !strings.Contains(script, "close-tab") {
		t.Fatalf("expected close-tab cleanup to remove default tab, got %q", script)
	}
}

func TestBuildZellijScriptNoLayoutsKeepsDefaultTab(t *testing.T) {
	cfg := &config.TmuxCommand{
		SessionName: "session",
		OnExists:    "",
	}
	script := buildZellijScript("session", cfg, nil)
	if strings.Contains(script, "new-tab --layout") {
		t.Fatalf("expected no new-tab actions when no layouts provided, got %q", script)
	}
	if strings.Contains(script, "close-tab") {
		t.Fatalf("did not expect close-tab when no layouts provided, got %q", script)
	}
	if !strings.Contains(script, "zellij attach --create-background \"$session\"") {
		t.Fatalf("expected basic attach when no layouts provided, got %q", script)
	}
}

func TestBuildTmuxWindowCommand(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		env      map[string]string
		expected string
	}{
		{
			name:     "empty command uses default shell",
			command:  "",
			env:      map[string]string{},
			expected: "exec ${SHELL:-bash}",
		},
		{
			name:     "simple command",
			command:  "vim",
			env:      map[string]string{},
			expected: "vim",
		},
		{
			name:     "command with env vars",
			command:  "lazygit",
			env:      map[string]string{"FOO": "bar"},
			expected: "export FOO=bar; lazygit",
		},
		{
			name:     "whitespace-only command uses default shell",
			command:  "   ",
			env:      map[string]string{},
			expected: "exec ${SHELL:-bash}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildTmuxWindowCommand(tt.command, tt.env)
			if !strings.Contains(result, tt.command) && tt.command != "" && strings.TrimSpace(tt.command) != "" {
				t.Errorf("buildTmuxWindowCommand(%q) = %q, should contain command", tt.command, result)
			}
			if tt.command == "" || strings.TrimSpace(tt.command) == "" {
				if !strings.Contains(result, "exec ${SHELL:-bash}") {
					t.Errorf("buildTmuxWindowCommand(%q) = %q, should contain default shell", tt.command, result)
				}
			}
		})
	}
}

func TestBuildTmuxScript(t *testing.T) {
	tests := []struct {
		name        string
		sessionName string
		tmuxCfg     *config.TmuxCommand
		windows     []multiplexer.ResolvedWindow
		env         map[string]string
		checkScript func(t *testing.T, script string)
	}{
		{
			name:        "empty windows returns empty script",
			sessionName: "test",
			tmuxCfg: &config.TmuxCommand{
				OnExists: "switch",
				Attach:   false,
			},
			windows: []multiplexer.ResolvedWindow{},
			env:     map[string]string{},
			checkScript: func(t *testing.T, script string) {
				if script != "" {
					t.Errorf("expected empty script for empty windows, got %q", script)
				}
			},
		},
		{
			name:        "single window script",
			sessionName: "myrepo_wt_feature",
			tmuxCfg: &config.TmuxCommand{
				OnExists: "switch",
				Attach:   false,
			},
			windows: []multiplexer.ResolvedWindow{
				{Name: "shell", Command: "exec ${SHELL:-bash}", Cwd: "/path"},
			},
			env: map[string]string{},
			checkScript: func(t *testing.T, script string) {
				if !strings.Contains(script, "session='myrepo_wt_feature'") {
					t.Error("script should contain session name")
				}
				if !strings.Contains(script, "tmux new-session") {
					t.Error("script should create new session")
				}
				if !strings.Contains(script, "-n 'shell'") {
					t.Error("script should create window named 'shell'")
				}
			},
		},
		{
			name:        "on_exists kill mode",
			sessionName: "test",
			tmuxCfg: &config.TmuxCommand{
				OnExists: "kill",
				Attach:   false,
			},
			windows: []multiplexer.ResolvedWindow{
				{Name: "shell", Command: "exec ${SHELL:-bash}", Cwd: "/path"},
			},
			env: map[string]string{},
			checkScript: func(t *testing.T, script string) {
				if !strings.Contains(script, "tmux kill-session") {
					t.Error("script should kill existing session in kill mode")
				}
			},
		},
		{
			name:        "on_exists new mode",
			sessionName: "test",
			tmuxCfg: &config.TmuxCommand{
				OnExists: "new",
				Attach:   false,
			},
			windows: []multiplexer.ResolvedWindow{
				{Name: "shell", Command: "exec ${SHELL:-bash}", Cwd: "/path"},
			},
			env: map[string]string{},
			checkScript: func(t *testing.T, script string) {
				if !strings.Contains(script, "while tmux has-session") {
					t.Error("script should check for incremented session names in new mode")
				}
			},
		},
		{
			name:        "multiple windows",
			sessionName: "test",
			tmuxCfg: &config.TmuxCommand{
				OnExists: "switch",
				Attach:   false,
			},
			windows: []multiplexer.ResolvedWindow{
				{Name: "shell", Command: "exec ${SHELL:-bash}", Cwd: "/path"},
				{Name: "editor", Command: "vim", Cwd: "/path"},
			},
			env: map[string]string{},
			checkScript: func(t *testing.T, script string) {
				if !strings.Contains(script, "tmux new-window") {
					t.Error("script should create additional windows")
				}
				if !strings.Contains(script, "-n 'editor'") {
					t.Error("script should create window named 'editor'")
				}
			},
		},
		{
			name:        "attach inside tmux",
			sessionName: "test",
			tmuxCfg: &config.TmuxCommand{
				OnExists: "switch",
				Attach:   true,
			},
			windows: []multiplexer.ResolvedWindow{
				{Name: "shell", Command: "exec ${SHELL:-bash}", Cwd: "/path"},
			},
			env: map[string]string{},
			checkScript: func(t *testing.T, script string) {
				if !strings.Contains(script, "tmux switch-client") {
					t.Error("script should switch client when inside tmux")
				}
			},
		},
		{
			name:        "env vars in script",
			sessionName: "test",
			tmuxCfg: &config.TmuxCommand{
				OnExists: "switch",
				Attach:   false,
			},
			windows: []multiplexer.ResolvedWindow{
				{Name: "shell", Command: "exec ${SHELL:-bash}", Cwd: "/path"},
			},
			env: map[string]string{"REPO_NAME": "myrepo", "WORKTREE_NAME": "feature"},
			checkScript: func(t *testing.T, script string) {
				if !strings.Contains(script, "tmux set-environment") {
					t.Error("script should set environment variables")
				}
				if !strings.Contains(script, "REPO_NAME") || !strings.Contains(script, "WORKTREE_NAME") {
					t.Error("script should include REPO_NAME and WORKTREE_NAME env vars")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script := buildTmuxScript(tt.sessionName, tt.tmuxCfg, tt.windows, tt.env)
			tt.checkScript(t, script)
		})
	}
}

func TestResolveTmuxWindows(t *testing.T) {
	tests := []struct {
		name        string
		windows     []config.TmuxWindow
		env         map[string]string
		defaultCwd  string
		expectOk    bool
		expectCount int
	}{
		{
			name:        "empty windows returns false",
			windows:     []config.TmuxWindow{},
			env:         map[string]string{},
			defaultCwd:  "/default",
			expectOk:    false,
			expectCount: 0,
		},
		{
			name: "single window with name",
			windows: []config.TmuxWindow{
				{Name: "shell", Command: "", Cwd: ""},
			},
			env:         map[string]string{},
			defaultCwd:  "/default",
			expectOk:    true,
			expectCount: 1,
		},
		{
			name: "window with env vars in name",
			windows: []config.TmuxWindow{
				{Name: "$WORKTREE_NAME", Command: "", Cwd: ""},
			},
			env:         map[string]string{"WORKTREE_NAME": "feature"},
			defaultCwd:  "/default",
			expectOk:    true,
			expectCount: 1,
		},
		{
			name: "empty name gets auto-generated",
			windows: []config.TmuxWindow{
				{Name: "", Command: "", Cwd: ""},
			},
			env:         map[string]string{},
			defaultCwd:  "/default",
			expectOk:    true,
			expectCount: 1,
		},
		{
			name: "empty cwd uses default",
			windows: []config.TmuxWindow{
				{Name: "shell", Command: "", Cwd: ""},
			},
			env:         map[string]string{},
			defaultCwd:  "/my/path",
			expectOk:    true,
			expectCount: 1,
		},
		{
			name: "multiple windows",
			windows: []config.TmuxWindow{
				{Name: "shell", Command: "", Cwd: ""},
				{Name: "editor", Command: "vim", Cwd: "/custom"},
			},
			env:         map[string]string{},
			defaultCwd:  "/default",
			expectOk:    true,
			expectCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved, ok := resolveTmuxWindows(tt.windows, tt.env, tt.defaultCwd)
			if ok != tt.expectOk {
				t.Errorf("resolveTmuxWindows ok = %v, want %v", ok, tt.expectOk)
			}
			if len(resolved) != tt.expectCount {
				t.Errorf("resolveTmuxWindows count = %d, want %d", len(resolved), tt.expectCount)
			}

			if tt.expectOk && len(tt.windows) > 0 {
				// Check first window specifically
				w := resolved[0]
				if tt.windows[0].Name == "" && w.Name != "window-1" {
					t.Errorf("expected auto-generated name 'window-1', got %q", w.Name)
				}
				if tt.windows[0].Cwd == "" && w.Cwd != tt.defaultCwd {
					t.Errorf("expected default cwd %q, got %q", tt.defaultCwd, w.Cwd)
				}
			}
		})
	}
}

func TestBuildZellijInfoMessage(t *testing.T) {
	msg := buildZellijInfoMessage("session")
	if !strings.Contains(msg, "zellij attach") {
		t.Fatalf("expected attach message, got %q", msg)
	}
}

func TestBuildTmuxInfoMessage(t *testing.T) {
	msg := buildTmuxInfoMessage("session", true)
	if !strings.Contains(msg, "switch-client") {
		t.Fatalf("expected switch-client message, got %q", msg)
	}
	msg = buildTmuxInfoMessage("session", false)
	if !strings.Contains(msg, "attach-session") {
		t.Fatalf("expected attach-session message, got %q", msg)
	}
}

func TestSanitizeTmuxSessionName(t *testing.T) {
	got := sanitizeTmuxSessionName("wt:feature/branch")
	if got != "wt-feature-branch" {
		t.Fatalf("expected sanitized name, got %q", got)
	}
}

func TestSanitizeZellijSessionName(t *testing.T) {
	got := sanitizeZellijSessionName("owner/repo\\wt:worktree")
	if got != "owner-repo-wt-worktree" {
		t.Fatalf("expected sanitized name, got %q", got)
	}
}

func TestBuildZellijScriptDefaultOnExistsIncludesNoop(t *testing.T) {
	cfg := &config.TmuxCommand{
		SessionName: "session",
		OnExists:    "",
	}
	script := buildZellijScript("session", cfg, []string{"/tmp/layout"})
	if !strings.Contains(script, "if session_exists \"$session\"; then\n  :\nfi\n") {
		t.Fatalf("expected no-op in default on_exists branch, got %q", script)
	}
}

func TestOpenTmuxSession(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	wt := &models.WorktreeInfo{Path: t.TempDir(), Branch: "feature"}

	if cmd := m.openTmuxSession(nil, wt); cmd != nil {
		t.Fatal("expected nil command for nil tmux config")
	}

	badCfg := &config.TmuxCommand{SessionName: "session"}
	if msg := m.openTmuxSession(&config.CustomCommand{Tmux: badCfg}, wt)(); msg == nil {
		t.Fatal("expected error message for empty windows")
	}

	called := false
	m.commandRunner = func(_ context.Context, _ string, _ ...string) *exec.Cmd {
		called = true
		return exec.Command("true")
	}
	m.execProcess = func(_ *exec.Cmd, cb tea.ExecCallback) tea.Cmd {
		return func() tea.Msg {
			return cb(nil)
		}
	}

	cfgGood := &config.TmuxCommand{
		SessionName: "session",
		Attach:      true,
		OnExists:    "switch",
		Windows:     []config.TmuxWindow{{Name: "shell"}},
	}
	cmd := m.openTmuxSession(&config.CustomCommand{Tmux: cfgGood}, wt)
	if cmd == nil {
		t.Fatal("expected tmux command")
	}
	msg := cmd()
	ready, ok := msg.(tmuxSessionReadyMsg)
	if !ok {
		t.Fatalf("expected tmuxSessionReadyMsg, got %T", msg)
	}
	if !called {
		t.Fatal("expected command runner to be called")
	}
	if ready.sessionName != "session" {
		t.Fatalf("unexpected session name: %q", ready.sessionName)
	}
	if !ready.attach {
		t.Fatal("expected attach to be true")
	}

	// new_tab: should return terminalTabReadyMsg instead of tmuxSessionReadyMsg
	// and must NOT invoke execProcess (which would suspend the TUI).
	execProcessCalled := false
	m.execProcess = func(_ *exec.Cmd, _ tea.ExecCallback) tea.Cmd {
		execProcessCalled = true
		return nil
	}
	multiWindowCfg := &config.TmuxCommand{
		SessionName: "test-session",
		Attach:      true,
		OnExists:    "switch",
		Windows: []config.TmuxWindow{
			{Name: "editor", Command: "vim"},
			{Name: "build", Command: "make watch"},
			{Name: "git", Command: "gitu"},
		},
	}
	newTabCmd := m.openTmuxSession(&config.CustomCommand{
		Tmux:        multiWindowCfg,
		NewTab:      true,
		Description: "TMUX",
	}, wt)
	if newTabCmd == nil {
		t.Fatal("expected command for new_tab tmux")
	}
	tabMsg := newTabCmd()
	if _, ok := tabMsg.(terminalTabReadyMsg); !ok {
		t.Fatalf("expected terminalTabReadyMsg for new_tab, got %T", tabMsg)
	}
	if execProcessCalled {
		t.Fatal("execProcess must not be called when new_tab is set")
	}
}

func TestOpenTmuxSessionNewTabClearsTmuxEnv(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-1000/default,123,0")
	t.Setenv("TMUX_PANE", "%1")
	t.Setenv("KITTY_WINDOW_ID", "123")
	t.Setenv("WEZTERM_PANE", "")
	t.Setenv("WEZTERM_UNIX_SOCKET", "")

	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	wt := &models.WorktreeInfo{Path: t.TempDir(), Branch: "feature"}

	var kittyArgs []string
	m.commandRunner = func(_ context.Context, name string, args ...string) *exec.Cmd {
		if name == "kitty" {
			kittyArgs = append([]string{}, args...)
		}
		return exec.Command("true")
	}

	tmuxCfg := &config.TmuxCommand{
		SessionName: "session",
		Attach:      true,
		OnExists:    "switch",
		Windows:     []config.TmuxWindow{{Name: "shell"}},
	}
	cmd := m.openTmuxSession(&config.CustomCommand{
		Tmux:        tmuxCfg,
		NewTab:      true,
		Description: "TMUX",
	}, wt)
	if cmd == nil {
		t.Fatal("expected tmux new_tab command")
	}
	msg := cmd()
	ready, ok := msg.(terminalTabReadyMsg)
	if !ok {
		t.Fatalf("expected terminalTabReadyMsg, got %T", msg)
	}
	if ready.err != nil {
		t.Fatalf("expected no launch error, got %v", ready.err)
	}
	if len(kittyArgs) == 0 {
		t.Fatal("expected kitty launcher to be invoked")
	}
	argsStr := strings.Join(kittyArgs, " ")
	expectedTitle := filepath.Base(wt.Path)
	if !strings.Contains(argsStr, "--tab-title="+expectedTitle) {
		t.Fatalf("expected tab title %q, got args %q", expectedTitle, argsStr)
	}

	launchCmd := kittyArgs[len(kittyArgs)-1]
	var scriptPath string
	for _, part := range strings.Split(launchCmd, "'") {
		if strings.Contains(part, "lazyworktree-tab-") && strings.HasSuffix(part, ".sh") {
			scriptPath = part
			break
		}
	}
	if scriptPath == "" {
		t.Fatalf("expected script path in launch command, got %q", launchCmd)
	}
	defer func() { _ = os.Remove(scriptPath) }()

	content, err := os.ReadFile(scriptPath) //nolint:gosec
	if err != nil {
		t.Fatalf("failed to read generated script: %v", err)
	}
	if !strings.HasPrefix(string(content), "unset TMUX TMUX_PANE\n") {
		t.Fatalf("expected script to clear tmux env, got:\n%s", string(content))
	}
}

func TestOpenZellijSession(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)
	wt := &models.WorktreeInfo{Path: t.TempDir(), Branch: "feature"}

	if cmd := m.openZellijSession(nil, wt); cmd != nil {
		t.Fatal("expected nil command for nil zellij config")
	}

	if _, err := exec.LookPath("zellij"); err != nil {
		t.Skip("zellij not available in test environment")
	}

	// Ensure we are not inside a zellij session for this test
	t.Setenv("ZELLIJ", "")
	t.Setenv("ZELLIJ_SESSION_NAME", "")

	cfgGood := &config.TmuxCommand{
		SessionName: "session",
		Attach:      true,
		OnExists:    "switch",
		Windows:     []config.TmuxWindow{{Name: "shell"}},
	}

	// No sessions: mock commandRunner to return empty list, then verify execProcess is used
	// (zellijAttachNewSessionCmd uses execProcess for zellij attach --create)
	m.commandRunner = func(_ context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.Command("true")
	}
	var execProcessCmd *exec.Cmd
	m.execProcess = func(c *exec.Cmd, _ tea.ExecCallback) tea.Cmd {
		execProcessCmd = c
		return func() tea.Msg { return nil }
	}
	cmd := m.openZellijSession(&config.CustomCommand{Zellij: cfgGood}, wt)
	if cmd == nil {
		t.Fatal("expected command for no-sessions case (zellijAttachNewSessionCmd)")
	}
	if execProcessCmd == nil {
		t.Fatal("expected execProcess to be called for new session creation")
	}

	// With sessions: mock commandRunner to return sessions, should push session picker
	m.commandRunner = func(_ context.Context, _ string, args ...string) *exec.Cmd {
		// list-sessions returns existing sessions
		if len(args) > 0 && args[0] == "list-sessions" {
			if runtime.GOOS == osWindows {
				return windowsMockOutputCmd("existing-session [Created 1h ago]\n")
			}
			return exec.Command("printf", "%s", "existing-session [Created 1h ago]\n")
		}
		return exec.Command("true")
	}
	cmd = m.openZellijSession(&config.CustomCommand{Zellij: cfgGood}, wt)
	if cmd != nil {
		t.Fatal("expected nil command (session picker pushed to screen manager)")
	}
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeListSelect {
		t.Fatalf("expected list selection screen (session picker), got active=%v type=%v",
			m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}
	listScr := m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)
	if listScr.Title != "Select zellij session" {
		t.Fatalf("expected session picker, got title %q", listScr.Title)
	}

	// Simulate selecting a session: should push direction picker
	selectCmd := listScr.OnSelect(appscreen.SelectionItem{ID: "existing-session", Label: "existing-session"})
	if selectCmd != nil {
		t.Fatal("expected nil command from session selection (direction picker pushed)")
	}
	if m.state.ui.screenManager.Type() != appscreen.TypeListSelect {
		t.Fatalf("expected direction picker screen, got type=%v", m.state.ui.screenManager.Type())
	}
	dirScr := m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)
	if dirScr.Title != "Select pane direction" {
		t.Fatalf("expected direction picker, got title %q", dirScr.Title)
	}

	// Pop both screens for the next subtest
	m.state.ui.screenManager.Pop()
	m.state.ui.screenManager.Pop()

	// new_tab: should return terminalTabReadyMsg instead of pushing a screen
	// and must NOT invoke execProcess (which would suspend the TUI).
	execProcessCalled := false
	m.execProcess = func(_ *exec.Cmd, _ tea.ExecCallback) tea.Cmd {
		execProcessCalled = true
		return nil
	}
	newTabCmd := m.openZellijSession(&config.CustomCommand{Zellij: cfgGood, NewTab: true}, wt)
	if newTabCmd == nil {
		t.Fatal("expected command for new_tab zellij")
	}
	tabMsg := newTabCmd()
	if _, ok := tabMsg.(terminalTabReadyMsg); !ok {
		t.Fatalf("expected terminalTabReadyMsg for new_tab, got %T", tabMsg)
	}
	if execProcessCalled {
		t.Fatal("execProcess must not be called when new_tab is set")
	}
}

func TestZellijAttachNewSessionCmd(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	var capturedCmd *exec.Cmd
	m.execProcess = func(c *exec.Cmd, cb tea.ExecCallback) tea.Cmd {
		capturedCmd = c
		return func() tea.Msg { return cb(nil) }
	}
	m.commandRunner = func(_ context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command(name, args...) //#nosec G204,G702 -- test mock with controlled args
	}

	cmd := m.zellijAttachNewSessionCmd("my-session", "/tmp/worktree")
	if cmd == nil {
		t.Fatal("expected command")
	}
	if capturedCmd == nil {
		t.Fatal("expected execProcess to be called")
	}
	if capturedCmd.Args[0] != "zellij" || capturedCmd.Args[1] != "attach" || capturedCmd.Args[2] != "--create" || capturedCmd.Args[3] != "my-session" {
		t.Fatalf("expected zellij attach --create my-session, got %v", capturedCmd.Args)
	}
	if capturedCmd.Dir != "/tmp/worktree" {
		t.Fatalf("expected Dir to be /tmp/worktree, got %q", capturedCmd.Dir)
	}
}

func TestZellijCreateExternalPaneCmd(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	var capturedArgs []string
	m.commandRunner = func(_ context.Context, name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		c := exec.Command("true") //#nosec G204 -- test mock
		return c
	}

	t.Setenv("SHELL", "/bin/zsh")
	cmd := m.zellijCreateExternalPaneCmd("my-session", "right", "/tmp/worktree")
	if cmd == nil {
		t.Fatal("expected command")
	}
	msg := cmd()

	// Verify the command args
	expectedArgs := []string{"zellij", "action", "new-pane", "--direction", "right", "--cwd", "/tmp/worktree", "--", "/bin/zsh"}
	if !reflect.DeepEqual(capturedArgs, expectedArgs) {
		t.Fatalf("expected args %v, got %v", expectedArgs, capturedArgs)
	}

	// On success, should return zellijPaneCreatedMsg (no attach, TUI stays active)
	paneMsg, ok := msg.(zellijPaneCreatedMsg)
	if !ok {
		t.Fatalf("expected zellijPaneCreatedMsg, got %T", msg)
	}
	if paneMsg.sessionName != "my-session" {
		t.Fatalf("expected sessionName 'my-session', got %q", paneMsg.sessionName)
	}
	if paneMsg.direction != "right" {
		t.Fatalf("expected direction 'right', got %q", paneMsg.direction)
	}
}

func TestGetAllZellijSessions(t *testing.T) {
	tests := []struct {
		name          string
		mockOutput    string
		mockErr       bool
		expectedNames []string
	}{
		{
			name:          "returns all active sessions sorted",
			mockOutput:    "gamma [Created 1h ago]\nalpha [Created 2h ago]\nbeta [Created 30m ago]\n",
			expectedNames: []string{"alpha", "beta", "gamma"},
		},
		{
			name:          "filters EXITED sessions",
			mockOutput:    "active-session [Created 1h ago]\nexited-one [Created 2h ago] (EXITED - attach to resurrect)\nanother-active [Created 30m ago]\n",
			expectedNames: []string{"active-session", "another-active"},
		},
		{
			name:          "handles empty output",
			mockOutput:    "",
			expectedNames: nil,
		},
		{
			name:          "handles only whitespace",
			mockOutput:    "  \n  \n",
			expectedNames: nil,
		},
		{
			name:          "handles single session",
			mockOutput:    "my-session [Created 5m ago]\n",
			expectedNames: []string{"my-session"},
		},
		{
			name:          "command fails",
			mockErr:       true,
			expectedNames: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
			m := NewModel(cfg, "")

			m.commandRunner = func(_ context.Context, _ string, _ ...string) *exec.Cmd {
				if tt.mockErr {
					return exec.Command("false")
				}
				if runtime.GOOS == osWindows {
					// #nosec G204 -- test mock data
					return windowsMockOutputCmd(tt.mockOutput)
				}
				// #nosec G204 -- test mock data
				return exec.Command("printf", "%s", tt.mockOutput)
			}

			got := m.getAllZellijSessions()
			if !reflect.DeepEqual(got, tt.expectedNames) {
				t.Fatalf("expected %v, got %v", tt.expectedNames, got)
			}
		})
	}
}

func TestShowZellijPaneSelectorSingleSession(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)
	wt := &models.WorktreeInfo{Path: t.TempDir(), Branch: "feature"}

	// Mock: return a single session
	m.commandRunner = func(_ context.Context, _ string, _ ...string) *exec.Cmd {
		if runtime.GOOS == osWindows {
			return windowsMockOutputCmd("only-session [Created 1h ago]\n")
		}
		return exec.Command("printf", "%s", "only-session [Created 1h ago]\n")
	}

	cmd := m.showZellijPaneSelector(wt)
	if cmd != nil {
		t.Fatal("expected nil command from showZellijPaneSelector")
	}

	// Should push the direction picker directly (skip session picker)
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeListSelect {
		t.Fatalf("expected list selection screen (direction picker), got active=%v type=%v",
			m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}
	listScr := m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)
	if listScr.Title != "Select pane direction" {
		t.Fatalf("expected direction picker, got title %q", listScr.Title)
	}
	if len(listScr.Items) != 2 {
		t.Fatalf("expected 2 direction items, got %d", len(listScr.Items))
	}
}

func TestShowZellijPaneSelectorMultipleSessions(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)
	wt := &models.WorktreeInfo{Path: t.TempDir(), Branch: "feature"}

	t.Setenv("ZELLIJ_SESSION_NAME", "session-b")

	// Mock: return multiple sessions
	m.commandRunner = func(_ context.Context, _ string, _ ...string) *exec.Cmd {
		if runtime.GOOS == osWindows {
			return windowsMockOutputCmd("session-a [Created 1h ago]\nsession-b [Created 2h ago]\nsession-c [Created 30m ago]")
		}
		return exec.Command("printf", "%s", "session-a [Created 1h ago]\nsession-b [Created 2h ago]\nsession-c [Created 30m ago]\n")
	}

	cmd := m.showZellijPaneSelector(wt)
	if cmd != nil {
		t.Fatal("expected nil command from showZellijPaneSelector")
	}

	// Should push the session picker
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeListSelect {
		t.Fatalf("expected list selection screen (session picker), got active=%v type=%v",
			m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}
	listScr := m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)
	if listScr.Title != "Select zellij session" {
		t.Fatalf("expected session picker, got title %q", listScr.Title)
	}
	if len(listScr.Items) != 3 {
		t.Fatalf("expected 3 session items, got %d", len(listScr.Items))
	}
	// Current session should be pre-selected
	if listScr.Cursor != 1 {
		t.Fatalf("expected cursor at index 1 (session-b), got %d", listScr.Cursor)
	}
}

func TestZellijNewPaneCmdSuccess(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	var capturedArgs []string
	m.commandRunner = func(_ context.Context, name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		return exec.Command("true")
	}

	cmd := m.zellijNewPaneCmd("my-session", "right", "/tmp/worktree")
	msg := cmd()
	paneMsg, ok := msg.(zellijPaneCreatedMsg)
	if !ok {
		t.Fatalf("expected zellijPaneCreatedMsg, got %T: %v", msg, msg)
	}
	if paneMsg.sessionName != "my-session" {
		t.Fatalf("expected session name %q, got %q", "my-session", paneMsg.sessionName)
	}
	if paneMsg.direction != "right" {
		t.Fatalf("expected direction %q, got %q", "right", paneMsg.direction)
	}

	// Verify captured command arguments include explicit shell via -- $SHELL
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "bash"
	}
	expected := []string{"zellij", "action", "new-pane", "--direction", "right", "--cwd", "/tmp/worktree", "--", shell}
	if !reflect.DeepEqual(capturedArgs, expected) {
		t.Fatalf("expected args %v, got %v", expected, capturedArgs)
	}
}

func TestOpenZellijSessionZellijNotInstalled(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)
	wt := &models.WorktreeInfo{Path: t.TempDir(), Branch: "feature"}

	// Override PATH so zellij cannot be found
	t.Setenv("PATH", t.TempDir())

	zellijCfg := &config.TmuxCommand{
		SessionName: "session",
		Windows:     []config.TmuxWindow{{Name: "shell"}},
	}
	cmd := m.openZellijSession(&config.CustomCommand{Zellij: zellijCfg}, wt)
	if cmd != nil {
		t.Fatal("expected nil command when zellij is not installed")
	}

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Fatalf("expected info screen, got active=%v type=%v",
			m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}
	infoScr := m.state.ui.screenManager.Current().(*appscreen.InfoScreen)
	if !strings.Contains(infoScr.Message, "zellij is not installed") {
		t.Errorf("expected 'not installed' message, got %q", infoScr.Message)
	}
}

func TestOpenZellijSessionInsideZellijShowsPaneSelector(t *testing.T) {
	if _, err := exec.LookPath("zellij"); err != nil {
		t.Skip("zellij not available in test environment")
	}

	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)
	wt := &models.WorktreeInfo{Path: t.TempDir(), Branch: "feature"}

	// Simulate being inside a zellij session
	t.Setenv("ZELLIJ", "0")
	t.Setenv("ZELLIJ_SESSION_NAME", "current-session")

	// Mock: return the current session
	m.commandRunner = func(_ context.Context, _ string, _ ...string) *exec.Cmd {
		if runtime.GOOS == osWindows {
			return windowsMockOutputCmd("current-session [Created 1h ago]\n")
		}
		return exec.Command("printf", "%s", "current-session [Created 1h ago]\n")
	}

	zellijCfg := &config.TmuxCommand{
		SessionName: "session",
		Windows:     []config.TmuxWindow{{Name: "shell"}},
	}
	cmd := m.openZellijSession(&config.CustomCommand{Zellij: zellijCfg}, wt)
	if cmd != nil {
		t.Fatal("expected nil command (pane selector pushed to screen)")
	}

	// Should show direction picker directly (single session)
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeListSelect {
		t.Fatalf("expected list selection screen, got active=%v type=%v",
			m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}
	listScr := m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)
	if listScr.Title != "Select pane direction" {
		t.Fatalf("expected direction picker, got title %q", listScr.Title)
	}
}
