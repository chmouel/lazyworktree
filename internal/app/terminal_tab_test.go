package app

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

func TestKittyLauncherName(t *testing.T) {
	launcher := &KittyLauncher{}
	if launcher.Name() != "Kitty" {
		t.Errorf("expected 'Kitty', got %q", launcher.Name())
	}
}

func TestKittyLauncherIsAvailable(t *testing.T) {
	launcher := &KittyLauncher{}

	t.Run("not available when env not set", func(t *testing.T) {
		t.Setenv("KITTY_WINDOW_ID", "")
		if launcher.IsAvailable() {
			t.Error("expected IsAvailable to return false when KITTY_WINDOW_ID is empty")
		}
	})

	t.Run("available when env is set", func(t *testing.T) {
		t.Setenv("KITTY_WINDOW_ID", "123")
		if !launcher.IsAvailable() {
			t.Error("expected IsAvailable to return true when KITTY_WINDOW_ID is set")
		}
	})
}

func TestKittyLauncherLaunch(t *testing.T) {
	var capturedName string
	var capturedArgs []string

	launcher := &KittyLauncher{
		commandRunner: func(_ context.Context, name string, args ...string) *exec.Cmd {
			capturedName = name
			capturedArgs = args
			return exec.Command("true")
		},
	}

	title, err := launcher.Launch(context.Background(), "claude", "/path/to/worktree", "Claude Code", map[string]string{
		"WORKTREE_NAME": "feature",
		"REPO_NAME":     "myrepo",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if title != "Claude Code" {
		t.Errorf("expected title 'Claude Code', got %q", title)
	}

	if capturedName != "kitty" {
		t.Errorf("expected command 'kitty', got %q", capturedName)
	}

	// Check required args are present
	argsStr := strings.Join(capturedArgs, " ")
	if !strings.Contains(argsStr, "@") {
		t.Error("expected '@' in args")
	}
	if !strings.Contains(argsStr, "launch") {
		t.Error("expected 'launch' in args")
	}
	if !strings.Contains(argsStr, "--type=tab") {
		t.Error("expected '--type=tab' in args")
	}
	if !strings.Contains(argsStr, "--cwd=/path/to/worktree") {
		t.Error("expected '--cwd=/path/to/worktree' in args")
	}
	if !strings.Contains(argsStr, "--tab-title=Claude Code") {
		t.Error("expected '--tab-title=Claude Code' in args")
	}
	// Env vars should be present (order may vary)
	if !strings.Contains(argsStr, "--env=WORKTREE_NAME=feature") {
		t.Error("expected WORKTREE_NAME env var in args")
	}
	if !strings.Contains(argsStr, "--env=REPO_NAME=myrepo") {
		t.Error("expected REPO_NAME env var in args")
	}
	if !strings.Contains(argsStr, "bash -lc claude") {
		t.Error("expected 'bash -lc claude' in args")
	}
}

func TestKittyLauncherLaunchError(t *testing.T) {
	launcher := &KittyLauncher{
		commandRunner: func(_ context.Context, name string, args ...string) *exec.Cmd {
			return exec.Command("false") // Will fail
		},
	}

	_, err := launcher.Launch(context.Background(), "cmd", "/path", "Title", nil)
	if err == nil {
		t.Error("expected error when command fails")
	}
	if !strings.Contains(err.Error(), "failed to launch Kitty tab") {
		t.Errorf("expected 'failed to launch Kitty tab' in error, got %v", err)
	}
}

func TestWezTermLauncherName(t *testing.T) {
	launcher := &WezTermLauncher{}
	if launcher.Name() != "WezTerm" {
		t.Errorf("expected 'WezTerm', got %q", launcher.Name())
	}
}

func TestWezTermLauncherIsAvailable(t *testing.T) {
	launcher := &WezTermLauncher{}

	t.Run("not available when env not set", func(t *testing.T) {
		t.Setenv("WEZTERM_PANE", "")
		t.Setenv("WEZTERM_UNIX_SOCKET", "")
		if launcher.IsAvailable() {
			t.Error("expected IsAvailable to return false when WEZTERM env vars are empty")
		}
	})

	t.Run("available when WEZTERM_PANE is set", func(t *testing.T) {
		t.Setenv("WEZTERM_PANE", "1")
		t.Setenv("WEZTERM_UNIX_SOCKET", "")
		if !launcher.IsAvailable() {
			t.Error("expected IsAvailable to return true when WEZTERM_PANE is set")
		}
	})

	t.Run("available when WEZTERM_UNIX_SOCKET is set", func(t *testing.T) {
		t.Setenv("WEZTERM_PANE", "")
		t.Setenv("WEZTERM_UNIX_SOCKET", "/tmp/wezterm.sock")
		if !launcher.IsAvailable() {
			t.Error("expected IsAvailable to return true when WEZTERM_UNIX_SOCKET is set")
		}
	})
}

func TestWezTermLauncherLaunch(t *testing.T) {
	var capturedNames []string
	var capturedArgs [][]string

	launcher := &WezTermLauncher{
		commandRunner: func(_ context.Context, name string, args ...string) *exec.Cmd {
			capturedNames = append(capturedNames, name)
			capturedArgs = append(capturedArgs, append([]string(nil), args...))
			if len(capturedNames) == 1 {
				return exec.Command("sh", "-c", "printf 42")
			}
			return exec.Command("true")
		},
	}

	title, err := launcher.Launch(context.Background(), "claude", "/path/to/worktree", "Claude Code", map[string]string{
		"WORKTREE_NAME": "feature",
		"REPO_NAME":     "myrepo",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if title != "Claude Code" {
		t.Errorf("expected title 'Claude Code', got %q", title)
	}

	if len(capturedNames) != 2 {
		t.Fatalf("expected 2 wezterm calls, got %d", len(capturedNames))
	}
	if capturedNames[0] != "wezterm" {
		t.Errorf("expected command 'wezterm', got %q", capturedNames[0])
	}
	if capturedNames[1] != "wezterm" {
		t.Errorf("expected command 'wezterm', got %q", capturedNames[1])
	}

	argsStr := strings.Join(capturedArgs[0], " ")
	if !strings.Contains(argsStr, "cli spawn") {
		t.Error("expected 'cli spawn' in args")
	}
	if !strings.Contains(argsStr, "--cwd /path/to/worktree") {
		t.Error("expected '--cwd /path/to/worktree' in args")
	}
	if !strings.Contains(argsStr, "env") {
		t.Error("expected 'env' in args")
	}
	if !strings.Contains(argsStr, "WORKTREE_NAME=feature") {
		t.Error("expected WORKTREE_NAME env var in args")
	}
	if !strings.Contains(argsStr, "REPO_NAME=myrepo") {
		t.Error("expected REPO_NAME env var in args")
	}
	if !strings.Contains(argsStr, "bash -lc claude") {
		t.Error("expected 'bash -lc claude' in args")
	}

	argsStr = strings.Join(capturedArgs[1], " ")
	if !strings.Contains(argsStr, "cli set-tab-title") {
		t.Error("expected 'cli set-tab-title' in args")
	}
	if !strings.Contains(argsStr, "--pane-id 42") {
		t.Error("expected '--pane-id 42' in args")
	}
	if !strings.Contains(argsStr, "Claude Code") {
		t.Error("expected tab title in args")
	}
}

func TestWezTermLauncherLaunchError(t *testing.T) {
	launcher := &WezTermLauncher{
		commandRunner: func(_ context.Context, name string, args ...string) *exec.Cmd {
			return exec.Command("false") // Will fail
		},
	}

	_, err := launcher.Launch(context.Background(), "cmd", "/path", "Title", nil)
	if err == nil {
		t.Error("expected error when command fails")
	}
	if !strings.Contains(err.Error(), "failed to launch WezTerm tab") {
		t.Errorf("expected 'failed to launch WezTerm tab' in error, got %v", err)
	}
}

func TestITermLauncherName(t *testing.T) {
	launcher := &ITermLauncher{}
	if launcher.Name() != "iTerm" {
		t.Errorf("expected 'iTerm', got %q", launcher.Name())
	}
}

func TestITermLauncherIsAvailable(t *testing.T) {
	launcher := &ITermLauncher{}

	t.Run("not available when env not set", func(t *testing.T) {
		t.Setenv("ITERM_SESSION_ID", "")
		t.Setenv("TERM_PROGRAM", "")
		if launcher.IsAvailable() {
			t.Error("expected IsAvailable to return false when iTerm env vars are empty")
		}
	})

	t.Run("available when ITERM_SESSION_ID is set", func(t *testing.T) {
		t.Setenv("ITERM_SESSION_ID", "w0t0p0")
		t.Setenv("TERM_PROGRAM", "")
		if !launcher.IsAvailable() {
			t.Error("expected IsAvailable to return true when ITERM_SESSION_ID is set")
		}
	})

	t.Run("available when TERM_PROGRAM is iTerm.app", func(t *testing.T) {
		t.Setenv("ITERM_SESSION_ID", "")
		t.Setenv("TERM_PROGRAM", "iTerm.app")
		if !launcher.IsAvailable() {
			t.Error("expected IsAvailable to return true when TERM_PROGRAM is iTerm.app")
		}
	})
}

func TestITermLauncherLaunch(t *testing.T) {
	var capturedName string
	var capturedArgs []string

	launcher := &ITermLauncher{
		commandRunner: func(_ context.Context, name string, args ...string) *exec.Cmd {
			capturedName = name
			capturedArgs = args
			return exec.Command("true")
		},
	}

	title, err := launcher.Launch(context.Background(), "claude", "/path/to/worktree", "Claude Code", map[string]string{
		"WORKTREE_NAME": "feature",
		"REPO_NAME":     "myrepo",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if title != "Claude Code" {
		t.Errorf("expected title 'Claude Code', got %q", title)
	}

	if capturedName != "osascript" {
		t.Errorf("expected command 'osascript', got %q", capturedName)
	}

	if len(capturedArgs) < 5 {
		t.Fatalf("expected at least 5 args, got %d", len(capturedArgs))
	}
	if capturedArgs[0] != "-e" {
		t.Errorf("expected '-e' arg, got %q", capturedArgs[0])
	}
	if capturedArgs[2] != "--" {
		t.Errorf("expected '--' arg, got %q", capturedArgs[2])
	}
	if !strings.Contains(capturedArgs[1], "tell application \"iTerm\"") {
		t.Error("expected iTerm AppleScript in args")
	}

	expectedCmd := buildShellCommand("claude", "/path/to/worktree", map[string]string{
		"WORKTREE_NAME": "feature",
		"REPO_NAME":     "myrepo",
	})
	if capturedArgs[3] != expectedCmd {
		t.Errorf("expected command %q, got %q", expectedCmd, capturedArgs[3])
	}
	if capturedArgs[4] != "Claude Code" {
		t.Errorf("expected title arg 'Claude Code', got %q", capturedArgs[4])
	}
}

func TestITermLauncherLaunchError(t *testing.T) {
	launcher := &ITermLauncher{
		commandRunner: func(_ context.Context, name string, args ...string) *exec.Cmd {
			return exec.Command("false") // Will fail
		},
	}

	_, err := launcher.Launch(context.Background(), "cmd", "/path", "Title", nil)
	if err == nil {
		t.Error("expected error when command fails")
	}
	if !strings.Contains(err.Error(), "failed to launch iTerm tab") {
		t.Errorf("expected 'failed to launch iTerm tab' in error, got %v", err)
	}
}

func TestDetectTerminalLauncher(t *testing.T) {
	runner := func(_ context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("true")
	}

	t.Run("detects Kitty when available", func(t *testing.T) {
		t.Setenv("KITTY_WINDOW_ID", "123")
		launcher := detectTerminalLauncher(runner)
		if launcher == nil {
			t.Fatal("expected launcher to be detected when KITTY_WINDOW_ID is set")
		}
		if launcher.Name() != "Kitty" {
			t.Errorf("expected Kitty launcher, got %q", launcher.Name())
		}
	})

	t.Run("detects WezTerm when available", func(t *testing.T) {
		t.Setenv("KITTY_WINDOW_ID", "")
		t.Setenv("WEZTERM_PANE", "1")
		t.Setenv("ITERM_SESSION_ID", "")
		t.Setenv("TERM_PROGRAM", "")
		launcher := detectTerminalLauncher(runner)
		if launcher == nil {
			t.Fatal("expected launcher to be detected when WEZTERM_PANE is set")
		}
		if launcher.Name() != "WezTerm" {
			t.Errorf("expected WezTerm launcher, got %q", launcher.Name())
		}
	})

	t.Run("detects iTerm when available", func(t *testing.T) {
		t.Setenv("KITTY_WINDOW_ID", "")
		t.Setenv("WEZTERM_PANE", "")
		t.Setenv("WEZTERM_UNIX_SOCKET", "")
		t.Setenv("ITERM_SESSION_ID", "w0t0p0")
		t.Setenv("TERM_PROGRAM", "")
		launcher := detectTerminalLauncher(runner)
		if launcher == nil {
			t.Fatal("expected launcher to be detected when ITERM_SESSION_ID is set")
		}
		if launcher.Name() != "iTerm" {
			t.Errorf("expected iTerm launcher, got %q", launcher.Name())
		}
	})

	t.Run("returns nil when no terminal available", func(t *testing.T) {
		t.Setenv("KITTY_WINDOW_ID", "")
		t.Setenv("WEZTERM_PANE", "")
		t.Setenv("WEZTERM_UNIX_SOCKET", "")
		t.Setenv("ITERM_SESSION_ID", "")
		t.Setenv("TERM_PROGRAM", "")
		launcher := detectTerminalLauncher(runner)
		if launcher != nil {
			t.Errorf("expected nil launcher when no terminal is detected, got %v", launcher)
		}
	})
}

func TestBuildTerminalTabInfoMessage(t *testing.T) {
	msg := buildTerminalTabInfoMessage("Kitty", "Claude Code")
	expected := "Command launched in new Kitty tab: Claude Code"
	if msg != expected {
		t.Errorf("expected %q, got %q", expected, msg)
	}
}

func TestOpenTerminalTabNilCommand(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	wt := &models.WorktreeInfo{Path: t.TempDir(), Branch: "feature"}

	cmd := m.openTerminalTab(nil, wt)
	if cmd != nil {
		t.Error("expected nil command for nil customCmd")
	}
}

func TestOpenTerminalTabEmptyCommand(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	wt := &models.WorktreeInfo{Path: t.TempDir(), Branch: "feature"}

	customCmd := &config.CustomCommand{Command: ""}
	cmd := m.openTerminalTab(customCmd, wt)
	if cmd != nil {
		t.Error("expected nil command for empty command string")
	}
}

func TestOpenTerminalTabNoTerminal(t *testing.T) {
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("WEZTERM_PANE", "")
	t.Setenv("WEZTERM_UNIX_SOCKET", "")
	t.Setenv("ITERM_SESSION_ID", "")
	t.Setenv("TERM_PROGRAM", "")

	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	wt := &models.WorktreeInfo{Path: t.TempDir(), Branch: "feature"}

	customCmd := &config.CustomCommand{Command: "claude", Description: "Claude"}
	cmd := m.openTerminalTab(customCmd, wt)
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}

	msg := cmd()
	readyMsg, ok := msg.(terminalTabReadyMsg)
	if !ok {
		t.Fatalf("expected terminalTabReadyMsg, got %T", msg)
	}
	if readyMsg.err == nil {
		t.Error("expected error when no terminal is detected")
	}
	if !strings.Contains(readyMsg.err.Error(), "no supported terminal detected") {
		t.Errorf("expected 'no supported terminal detected' in error, got %v", readyMsg.err)
	}
	if !strings.Contains(readyMsg.err.Error(), "Kitty, WezTerm, or iTerm") {
		t.Errorf("expected 'Kitty, WezTerm, or iTerm' in error, got %v", readyMsg.err)
	}
}

func TestOpenTerminalTabSuccess(t *testing.T) {
	t.Setenv("KITTY_WINDOW_ID", "123")
	t.Setenv("WEZTERM_PANE", "")
	t.Setenv("WEZTERM_UNIX_SOCKET", "")

	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.commandRunner = func(_ context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("true")
	}

	wt := &models.WorktreeInfo{Path: t.TempDir(), Branch: "feature"}
	customCmd := &config.CustomCommand{Command: "claude", Description: "Claude Code"}

	cmd := m.openTerminalTab(customCmd, wt)
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}

	msg := cmd()
	readyMsg, ok := msg.(terminalTabReadyMsg)
	if !ok {
		t.Fatalf("expected terminalTabReadyMsg, got %T", msg)
	}
	if readyMsg.err != nil {
		t.Errorf("unexpected error: %v", readyMsg.err)
	}
	if readyMsg.terminalName != "Kitty" {
		t.Errorf("expected terminal name 'Kitty', got %q", readyMsg.terminalName)
	}
	if readyMsg.tabTitle != "Claude Code" {
		t.Errorf("expected tab title 'Claude Code', got %q", readyMsg.tabTitle)
	}
}

func TestOpenTerminalTabSuccessWezTerm(t *testing.T) {
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("WEZTERM_PANE", "1")
	t.Setenv("ITERM_SESSION_ID", "")
	t.Setenv("TERM_PROGRAM", "")

	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.commandRunner = func(_ context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("true")
	}

	wt := &models.WorktreeInfo{Path: t.TempDir(), Branch: "feature"}
	customCmd := &config.CustomCommand{Command: "claude", Description: "Claude Code"}

	cmd := m.openTerminalTab(customCmd, wt)
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}

	msg := cmd()
	readyMsg, ok := msg.(terminalTabReadyMsg)
	if !ok {
		t.Fatalf("expected terminalTabReadyMsg, got %T", msg)
	}
	if readyMsg.err != nil {
		t.Errorf("unexpected error: %v", readyMsg.err)
	}
	if readyMsg.terminalName != "WezTerm" {
		t.Errorf("expected terminal name 'WezTerm', got %q", readyMsg.terminalName)
	}
	if readyMsg.tabTitle != "Claude Code" {
		t.Errorf("expected tab title 'Claude Code', got %q", readyMsg.tabTitle)
	}
}

func TestOpenTerminalTabSuccessITerm(t *testing.T) {
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("WEZTERM_PANE", "")
	t.Setenv("WEZTERM_UNIX_SOCKET", "")
	t.Setenv("ITERM_SESSION_ID", "w0t0p0")
	t.Setenv("TERM_PROGRAM", "")

	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.commandRunner = func(_ context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("true")
	}

	wt := &models.WorktreeInfo{Path: t.TempDir(), Branch: "feature"}
	customCmd := &config.CustomCommand{Command: "claude", Description: "Claude Code"}

	cmd := m.openTerminalTab(customCmd, wt)
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}

	msg := cmd()
	readyMsg, ok := msg.(terminalTabReadyMsg)
	if !ok {
		t.Fatalf("expected terminalTabReadyMsg, got %T", msg)
	}
	if readyMsg.err != nil {
		t.Errorf("unexpected error: %v", readyMsg.err)
	}
	if readyMsg.terminalName != "iTerm" {
		t.Errorf("expected terminal name 'iTerm', got %q", readyMsg.terminalName)
	}
	if readyMsg.tabTitle != "Claude Code" {
		t.Errorf("expected tab title 'Claude Code', got %q", readyMsg.tabTitle)
	}
}

func TestOpenTerminalTabUsesWorktreeNameWhenNoDescription(t *testing.T) {
	t.Setenv("KITTY_WINDOW_ID", "123")
	t.Setenv("WEZTERM_PANE", "")
	t.Setenv("WEZTERM_UNIX_SOCKET", "")

	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.commandRunner = func(_ context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("true")
	}

	tmpDir := t.TempDir()
	wt := &models.WorktreeInfo{Path: tmpDir, Branch: "feature"}
	customCmd := &config.CustomCommand{Command: "claude", Description: ""}

	cmd := m.openTerminalTab(customCmd, wt)
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}

	msg := cmd()
	readyMsg, ok := msg.(terminalTabReadyMsg)
	if !ok {
		t.Fatalf("expected terminalTabReadyMsg, got %T", msg)
	}

	// When no description, should use filepath.Base(wt.Path)
	if readyMsg.tabTitle == "" {
		t.Error("expected non-empty tab title")
	}
}

func TestTerminalTabReadyMsgHandling(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)

	// Test error case
	updated, _ := m.Update(terminalTabReadyMsg{err: os.ErrNotExist})
	model := updated.(*Model)
	if !model.state.ui.screenManager.IsActive() {
		t.Error("expected info screen to be shown for error")
	}

	// Reset
	m = NewModel(cfg, "")
	m.setWindowSize(120, 40)

	// Test success case
	updated, _ = m.Update(terminalTabReadyMsg{terminalName: "Kitty", tabTitle: "Test Tab"})
	model = updated.(*Model)
	if !model.state.ui.screenManager.IsActive() {
		t.Error("expected info screen to be shown for success")
	}
}
