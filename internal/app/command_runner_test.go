package app

import (
	"os/exec"
	"runtime"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

type commandCapture struct {
	name string
	args []string
	dir  string
	env  []string
}

const (
	testWorktreePath = "/tmp/wt"
	windowsOS        = "windows"
)

func (c *commandCapture) runner(name string, args ...string) *exec.Cmd {
	c.name = name
	c.args = append([]string{}, args...)
	return exec.Command(name, args...)
}

func (c *commandCapture) exec(cmd *exec.Cmd, _ tea.ExecCallback) tea.Cmd {
	c.dir = cmd.Dir
	c.env = append([]string{}, cmd.Env...)
	return func() tea.Msg { return nil }
}

func (c *commandCapture) start(cmd *exec.Cmd) error {
	c.dir = cmd.Dir
	c.env = append([]string{}, cmd.Env...)
	return nil
}

func envValue(env []string, key string) (string, bool) {
	for _, entry := range env {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) == 2 && parts[0] == key {
			return parts[1], true
		}
	}
	return "", false
}

func TestOpenLazyGitUsesCommandRunner(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.filteredWts = []*models.WorktreeInfo{{Path: testWorktreePath}}
	m.selectedIndex = 0

	capture := &commandCapture{}
	m.commandRunner = capture.runner
	m.execProcess = capture.exec

	cmd := m.openLazyGit()
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}

	if capture.name != "lazygit" {
		t.Fatalf("expected lazygit command, got %q", capture.name)
	}
	if len(capture.args) != 0 {
		t.Fatalf("expected no args, got %v", capture.args)
	}
	if capture.dir != testWorktreePath {
		t.Fatalf("expected worktree dir, got %q", capture.dir)
	}
}

func TestExecuteCustomCommandUsesCommandRunner(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		CustomCommands: map[string]*config.CustomCommand{
			"x": {
				Command: "echo hello",
				Wait:    true,
			},
		},
	}
	m := NewModel(cfg, "")
	m.filteredWts = []*models.WorktreeInfo{{Path: testWorktreePath, Branch: "feat"}}
	m.selectedIndex = 0

	capture := &commandCapture{}
	m.commandRunner = capture.runner
	m.execProcess = capture.exec

	cmd := m.executeCustomCommand("x")
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}

	if capture.name != "bash" {
		t.Fatalf("expected bash command, got %q", capture.name)
	}
	if len(capture.args) != 2 || capture.args[0] != "-c" {
		t.Fatalf("expected bash -c args, got %v", capture.args)
	}
	if !strings.Contains(capture.args[1], "echo hello") {
		t.Fatalf("expected command to include custom command, got %q", capture.args[1])
	}
	if !strings.Contains(capture.args[1], "Press any key to continue") {
		t.Fatalf("expected wait prompt, got %q", capture.args[1])
	}
	if capture.dir != testWorktreePath {
		t.Fatalf("expected worktree dir, got %q", capture.dir)
	}
	if value, ok := envValue(capture.env, "WORKTREE_PATH"); !ok || value != testWorktreePath {
		t.Fatalf("expected WORKTREE_PATH in env, got %q (present=%v)", value, ok)
	}
}

func TestOpenPRUsesCommandRunner(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.filteredWts = []*models.WorktreeInfo{
		{
			Path:   testWorktreePath,
			Branch: "feat",
			PR: &models.PRInfo{
				URL: "https://example.com/pr/1",
			},
		},
	}
	m.selectedIndex = 0

	capture := &commandCapture{}
	m.commandRunner = capture.runner
	m.startCommand = capture.start

	cmd := m.openPR()
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}
	_ = cmd()

	expected := "xdg-open"
	switch runtime.GOOS {
	case "darwin":
		expected = "open"
	case windowsOS:
		expected = "rundll32"
	}
	if capture.name != expected {
		t.Fatalf("expected %q command, got %q", expected, capture.name)
	}
	if runtime.GOOS == windowsOS {
		if len(capture.args) < 2 || capture.args[1] != "https://example.com/pr/1" {
			t.Fatalf("expected windows URL args, got %v", capture.args)
		}
	} else {
		if len(capture.args) != 1 || capture.args[0] != "https://example.com/pr/1" {
			t.Fatalf("expected URL arg, got %v", capture.args)
		}
	}
}

func TestAttachTmuxSessionCmdUsesCommandRunner(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	capture := &commandCapture{}
	m.commandRunner = capture.runner
	m.execProcess = capture.exec

	cmd := m.attachTmuxSessionCmd("session", false)
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}

	if capture.name != "tmux" {
		t.Fatalf("expected tmux command, got %q", capture.name)
	}
	if len(capture.args) != 3 || capture.args[0] != "attach-session" || capture.args[2] != "session" {
		t.Fatalf("unexpected tmux args: %v", capture.args)
	}
}
