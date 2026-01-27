package app

import (
	"os/exec"
	"reflect"
	"runtime"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

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
			m.commandRunner = func(name string, args ...string) *exec.Cmd {
				if tt.mockErr {
					// Command that will fail
					return exec.Command("false")
				}
				// Command that returns the mock output
				if runtime.GOOS == osWindows {
					// #nosec G204 -- test mock data, not user input
					return exec.Command("cmd", "/c", "echo "+tt.mockOutput)
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
	m.commandRunner = func(name string, args ...string) *exec.Cmd {
		mockOutput := "my-prefix-feature\nother-session\nmy-prefix-bugfix\n"
		if runtime.GOOS == osWindows {
			return exec.Command("cmd", "/c", "echo "+mockOutput)
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
	if model.currentScreen != screenNone {
		t.Fatalf("expected no screen change, got %v", model.currentScreen)
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
	if model.currentScreen != screenInfo {
		t.Fatalf("expected info screen, got %v", model.currentScreen)
	}
	if model.infoScreen == nil {
		t.Fatal("expected info screen to be created")
	}
	if cmd != nil {
		t.Fatal("expected no command when not attaching")
	}
	if !strings.Contains(model.infoScreen.message, "tmux attach-session -t 'wt_test'") {
		t.Errorf("expected attach message, got %q", model.infoScreen.message)
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
	if model.currentScreen != screenNone {
		t.Fatalf("expected no screen change, got %v", model.currentScreen)
	}
	if cmd == nil {
		t.Fatal("expected attach command to be returned")
	}
}

func TestZellijSessionReadyShowsInfoWhenInsideZellij(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)

	updated, cmd := m.Update(zellijSessionReadyMsg{sessionName: "wt_test", attach: true, insideZellij: true})
	model := updated.(*Model)
	if model.currentScreen != screenInfo {
		t.Fatalf("expected info screen, got %v", model.currentScreen)
	}
	if model.infoScreen == nil {
		t.Fatal("expected info screen to be created")
	}
	if cmd != nil {
		t.Fatal("expected no command when inside zellij")
	}
	if !strings.Contains(model.infoScreen.message, "zellij attach 'wt_test'") {
		t.Errorf("expected attach message, got %q", model.infoScreen.message)
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
	if !strings.Contains(script, "while ! zellij list-sessions --short 2>/dev/null | grep -Fxq \"$session\"; do") {
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
		windows     []resolvedTmuxWindow
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
			windows: []resolvedTmuxWindow{},
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
			windows: []resolvedTmuxWindow{
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
			windows: []resolvedTmuxWindow{
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
			windows: []resolvedTmuxWindow{
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
			windows: []resolvedTmuxWindow{
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
			windows: []resolvedTmuxWindow{
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
			windows: []resolvedTmuxWindow{
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
	if msg := m.openTmuxSession(badCfg, wt)(); msg == nil {
		t.Fatal("expected error message for empty windows")
	}

	called := false
	m.commandRunner = func(_ string, _ ...string) *exec.Cmd {
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
	cmd := m.openTmuxSession(cfgGood, wt)
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
}

func TestOpenZellijSession(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	wt := &models.WorktreeInfo{Path: t.TempDir(), Branch: "feature"}

	if cmd := m.openZellijSession(nil, wt); cmd != nil {
		t.Fatal("expected nil command for nil zellij config")
	}

	badCfg := &config.TmuxCommand{SessionName: "session"}
	if msg := m.openZellijSession(badCfg, wt)(); msg == nil {
		t.Fatal("expected error message for empty windows")
	}

	called := false
	m.commandRunner = func(_ string, _ ...string) *exec.Cmd {
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
	cmd := m.openZellijSession(cfgGood, wt)
	if cmd == nil {
		t.Fatal("expected zellij command")
	}
	msg := cmd()
	ready, ok := msg.(zellijSessionReadyMsg)
	if !ok {
		t.Fatalf("expected zellijSessionReadyMsg, got %T", msg)
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
}
