package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/theme"
)

type commandCapture struct {
	name string
	args []string
	dir  string
	env  []string
}

const testSessionName = "session"

const (
	testWorktreePath = "/tmp/wt"
	testBashCmd      = "bash"
)

func (c *commandCapture) runner(_ context.Context, name string, args ...string) *exec.Cmd {
	c.name = name
	c.args = append([]string{}, args...)
	return exec.Command(name, args...) //#nosec G204,G702 -- test helper with controlled args
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
	m.state.data.filteredWts = []*models.WorktreeInfo{{Path: testWorktreePath}}
	m.state.data.selectedIndex = 0

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
	m.state.data.filteredWts = []*models.WorktreeInfo{{Path: testWorktreePath, Branch: "feat"}}
	m.state.data.selectedIndex = 0

	capture := &commandCapture{}
	m.commandRunner = capture.runner
	m.execProcess = capture.exec

	cmd := m.executeCustomCommand("x")
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}

	if capture.name != testBashCmd {
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

func TestExecuteCustomCommandShowsOutput(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		Pager:       "less --use-color --wordwrap -qcR -P 'Press q to exit..'",
		CustomCommands: map[string]*config.CustomCommand{
			"x": {
				Command:    "echo hello",
				ShowOutput: true,
				Wait:       true,
			},
		},
	}
	m := NewModel(cfg, "")
	m.state.data.filteredWts = []*models.WorktreeInfo{{Path: testWorktreePath, Branch: "feat"}}
	m.state.data.selectedIndex = 0

	capture := &commandCapture{}
	m.commandRunner = capture.runner
	m.execProcess = capture.exec

	cmd := m.executeCustomCommand("x")
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}

	if capture.name != testBashCmd {
		t.Fatalf("expected bash command, got %q", capture.name)
	}
	if len(capture.args) != 2 || capture.args[0] != "-c" {
		t.Fatalf("expected bash -c args, got %v", capture.args)
	}
	if strings.Contains(capture.args[1], "Press any key to continue") {
		t.Fatalf("unexpected wait prompt in output command: %q", capture.args[1])
	}
	if !strings.Contains(capture.args[1], "set -o pipefail") {
		t.Fatalf("expected pipefail in command, got %q", capture.args[1])
	}
	if !strings.Contains(capture.args[1], "LESS= LESSHISTFILE=-") {
		t.Fatalf("expected LESS defaults in command, got %q", capture.args[1])
	}
	if !strings.Contains(capture.args[1], "less --use-color --wordwrap -qcR -P 'Press q to exit..'") {
		t.Fatalf("expected pager in command, got %q", capture.args[1])
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
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{
			Path:   testWorktreePath,
			Branch: "feat",
			PR: &models.PRInfo{
				URL: testPRURL,
			},
		},
	}
	m.state.data.selectedIndex = 0

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
	case osDarwin:
		expected = "open"
	case osWindows:
		expected = "rundll32"
	}
	if capture.name != expected {
		t.Fatalf("expected %q command, got %q", expected, capture.name)
	}
	if runtime.GOOS == osWindows {
		if len(capture.args) < 2 || capture.args[1] != testPRURL {
			t.Fatalf("expected windows URL args, got %v", capture.args)
		}
	} else {
		if len(capture.args) != 1 || capture.args[0] != testPRURL {
			t.Fatalf("expected URL arg, got %v", capture.args)
		}
	}
}

func TestGitURLToWebURL(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "SSH format",
			input:    "git@github.com:chmouel/lazyworktree.git",
			expected: "https://github.com/chmouel/lazyworktree",
		},
		{
			name:     "SSH format without .git",
			input:    "git@github.com:chmouel/lazyworktree",
			expected: "https://github.com/chmouel/lazyworktree",
		},
		{
			name:     "HTTPS format",
			input:    "https://github.com/chmouel/lazyworktree.git",
			expected: "https://github.com/chmouel/lazyworktree",
		},
		{
			name:     "HTTPS format without .git",
			input:    "https://github.com/chmouel/lazyworktree",
			expected: "https://github.com/chmouel/lazyworktree",
		},
		{
			name:     "ssh:// format",
			input:    "ssh://git@github.com/chmouel/lazyworktree.git",
			expected: "https://github.com/chmouel/lazyworktree",
		},
		{
			name:     "git:// format",
			input:    "git://github.com/chmouel/lazyworktree.git",
			expected: "https://github.com/chmouel/lazyworktree",
		},
		{
			name:     "GitLab SSH",
			input:    "git@gitlab.com:group/project.git",
			expected: "https://gitlab.com/group/project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.gitURLToWebURL(tt.input)
			if result != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, result)
			}
		})
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

	cmd := m.attachTmuxSessionCmd(testSessionName, false)
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}

	if capture.name != "tmux" {
		t.Fatalf("expected tmux command, got %q", capture.name)
	}
	if len(capture.args) != 3 || capture.args[0] != "attach-session" || capture.args[2] != testSessionName {
		t.Fatalf("unexpected tmux args: %v", capture.args)
	}
}

func TestAttachZellijSessionCmdUsesCommandRunner(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	capture := &commandCapture{}
	m.commandRunner = capture.runner
	m.execProcess = capture.exec

	cmd := m.attachZellijSessionCmd(testSessionName)
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}

	if capture.name != "zellij" {
		t.Fatalf("expected zellij command, got %q", capture.name)
	}
	if len(capture.args) != 2 || capture.args[0] != onExistsAttach || capture.args[1] != testSessionName {
		t.Fatalf("unexpected zellij args: %v", capture.args)
	}
}

func TestExecuteArbitraryCommand(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
		Pager:       "less -R",
	}
	m := NewModel(cfg, "")
	m.state.data.filteredWts = []*models.WorktreeInfo{{Path: testWorktreePath, Branch: "feat"}}
	m.state.data.selectedIndex = 0

	capture := &commandCapture{}
	m.commandRunner = capture.runner
	m.execProcess = capture.exec

	cmd := m.executeArbitraryCommand("make test")
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}

	if capture.name != testBashCmd {
		t.Fatalf("expected bash command, got %q", capture.name)
	}
	if len(capture.args) != 2 || capture.args[0] != "-c" {
		t.Fatalf("expected bash -c args, got %v", capture.args)
	}
	if !strings.Contains(capture.args[1], "make test") {
		t.Fatalf("expected command to include 'make test', got %q", capture.args[1])
	}
	if !strings.Contains(capture.args[1], "set -o pipefail") {
		t.Fatalf("expected pipefail in command, got %q", capture.args[1])
	}
	if !strings.Contains(capture.args[1], "less -R") {
		t.Fatalf("expected pager in command, got %q", capture.args[1])
	}
	if capture.dir != testWorktreePath {
		t.Fatalf("expected worktree dir, got %q", capture.dir)
	}
	if value, ok := envValue(capture.env, "WORKTREE_PATH"); !ok || value != testWorktreePath {
		t.Fatalf("expected WORKTREE_PATH in env, got %q (present=%v)", value, ok)
	}
	if value, ok := envValue(capture.env, "WORKTREE_BRANCH"); !ok || value != "feat" {
		t.Fatalf("expected WORKTREE_BRANCH in env, got %q (present=%v)", value, ok)
	}
}

func TestExecuteArbitraryCommandNoSelection(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.data.filteredWts = []*models.WorktreeInfo{}
	m.state.data.selectedIndex = -1

	cmd := m.executeArbitraryCommand("make test")
	if cmd != nil {
		t.Fatal("expected nil command when no worktree selected")
	}
}

func TestShowCommitDiffInteractiveUsesConfiguredPager(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir:         t.TempDir(),
		GitPager:            "tig",
		GitPagerArgs:        []string{"--foo"},
		GitPagerInteractive: true,
	}
	m := NewModel(cfg, "")
	wt := &models.WorktreeInfo{Path: testWorktreePath, Branch: "feat"}

	capture := &commandCapture{}
	m.commandRunner = capture.runner
	m.execProcess = capture.exec

	if cmd := m.showCommitDiff("abc123", wt); cmd == nil {
		t.Fatal("expected diff command")
	}

	if capture.name != testBashCmd {
		t.Fatalf("expected bash command, got %q", capture.name)
	}
	if len(capture.args) != 2 || capture.args[0] != "-c" {
		t.Fatalf("expected bash -c args, got %v", capture.args)
	}
	cmdStr := capture.args[1]
	if !strings.Contains(cmdStr, "git show --patch --no-color abc123") {
		t.Fatalf("expected git show in command, got %q", cmdStr)
	}
	if !strings.Contains(cmdStr, "| tig --foo") {
		t.Fatalf("expected interactive pager in command, got %q", cmdStr)
	}
	if strings.Contains(cmdStr, "less") {
		t.Fatalf("did not expect non-interactive pager in command, got %q", cmdStr)
	}
	if capture.dir != testWorktreePath {
		t.Fatalf("expected worktree dir, got %q", capture.dir)
	}
	if value, ok := envValue(capture.env, "WORKTREE_PATH"); !ok || value != testWorktreePath {
		t.Fatalf("expected WORKTREE_PATH in env, got %q (present=%v)", value, ok)
	}
}

func TestShowCommitFileDiffInteractiveUsesConfiguredPager(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir:         t.TempDir(),
		GitPager:            "tig",
		GitPagerArgs:        []string{"--foo"},
		GitPagerInteractive: true,
	}
	m := NewModel(cfg, "")

	capture := &commandCapture{}
	m.commandRunner = capture.runner
	m.execProcess = capture.exec

	if cmd := m.showCommitFileDiff("abc123", "file.txt", testWorktreePath); cmd == nil {
		t.Fatal("expected file diff command")
	}

	if capture.name != testBashCmd {
		t.Fatalf("expected bash command, got %q", capture.name)
	}
	if len(capture.args) != 2 || capture.args[0] != "-c" {
		t.Fatalf("expected bash -c args, got %v", capture.args)
	}
	cmdStr := capture.args[1]
	if !strings.Contains(cmdStr, "git show --patch --no-color abc123 -- \"file.txt\"") {
		t.Fatalf("expected git show file diff, got %q", cmdStr)
	}
	if !strings.Contains(cmdStr, "| tig --foo") {
		t.Fatalf("expected interactive pager in file diff command, got %q", cmdStr)
	}
	if capture.dir != testWorktreePath {
		t.Fatalf("expected worktree dir, got %q", capture.dir)
	}
}

func TestShowCommitDiffCommandModeUsesOwnDiff(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir:         t.TempDir(),
		GitPager:            "lumen",
		GitPagerArgs:        []string{"--theme", "dark"},
		GitPagerCommandMode: true,
	}
	m := NewModel(cfg, "")
	wt := &models.WorktreeInfo{Path: testWorktreePath, Branch: "feat"}

	capture := &commandCapture{}
	m.commandRunner = capture.runner
	m.execProcess = capture.exec

	if cmd := m.showCommitDiff("abc123", wt); cmd == nil {
		t.Fatal("expected diff command")
	}

	if capture.name != testBashCmd {
		t.Fatalf("expected bash command, got %q", capture.name)
	}
	if len(capture.args) != 2 || capture.args[0] != "-c" {
		t.Fatalf("expected bash -c args, got %v", capture.args)
	}
	cmdStr := capture.args[1]
	if !strings.Contains(cmdStr, "lumen diff --theme dark abc123^..abc123") {
		t.Fatalf("expected lumen diff with commit range, got %q", cmdStr)
	}
	if strings.Contains(cmdStr, "|") {
		t.Fatalf("command mode should not use pipes, got %q", cmdStr)
	}
	if capture.dir != testWorktreePath {
		t.Fatalf("expected worktree dir, got %q", capture.dir)
	}
}

func TestShowFileDiffCommandModeUsesFileFlag(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir:         t.TempDir(),
		GitPager:            "lumen",
		GitPagerArgs:        []string{"--theme", "dark"},
		GitPagerCommandMode: true,
	}
	m := NewModel(cfg, "")
	m.state.data.filteredWts = []*models.WorktreeInfo{{Path: testWorktreePath, Branch: "feat"}}
	m.state.data.selectedIndex = 0

	capture := &commandCapture{}
	m.commandRunner = capture.runner
	m.execProcess = capture.exec

	if cmd := m.showFileDiff(StatusFile{Filename: "file.txt"}); cmd == nil {
		t.Fatal("expected file diff command")
	}

	if capture.name != testBashCmd {
		t.Fatalf("expected bash command, got %q", capture.name)
	}
	if len(capture.args) != 2 || capture.args[0] != "-c" {
		t.Fatalf("expected bash -c args, got %v", capture.args)
	}
	cmdStr := capture.args[1]
	if !strings.Contains(cmdStr, "lumen diff --theme dark --file 'file.txt'") {
		t.Fatalf("expected lumen diff with --file, got %q", cmdStr)
	}
	if strings.Contains(cmdStr, "|") {
		t.Fatalf("command mode should not use pipes, got %q", cmdStr)
	}
	if capture.dir != testWorktreePath {
		t.Fatalf("expected worktree dir, got %q", capture.dir)
	}
}

func TestShowCommitFileDiffCommandModeUsesOwnDiff(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir:         t.TempDir(),
		GitPager:            "lumen",
		GitPagerArgs:        []string{"--theme", "dark"},
		GitPagerCommandMode: true,
	}
	m := NewModel(cfg, "")

	capture := &commandCapture{}
	m.commandRunner = capture.runner
	m.execProcess = capture.exec

	if cmd := m.showCommitFileDiff("abc123", "file.txt", testWorktreePath); cmd == nil {
		t.Fatal("expected file diff command")
	}

	if capture.name != testBashCmd {
		t.Fatalf("expected bash command, got %q", capture.name)
	}
	if len(capture.args) != 2 || capture.args[0] != "-c" {
		t.Fatalf("expected bash -c args, got %v", capture.args)
	}
	cmdStr := capture.args[1]
	if !strings.Contains(cmdStr, "lumen diff --theme dark abc123^..abc123 --file 'file.txt'") {
		t.Fatalf("expected lumen diff with commit range and file, got %q", cmdStr)
	}
	if strings.Contains(cmdStr, "|") {
		t.Fatalf("command mode should not use pipes, got %q", cmdStr)
	}
	if capture.dir != testWorktreePath {
		t.Fatalf("expected worktree dir, got %q", capture.dir)
	}
}

func TestShowCommitDiffVSCodeUsesGitDifftool(t *testing.T) {
	repoDir, branch := setupCommitDiffRepo(t)
	cfg := &config.AppConfig{
		WorktreeDir: repoDir,
		GitPager:    "code",
	}
	m := NewModel(cfg, "")
	wt := &models.WorktreeInfo{Path: repoDir, Branch: branch}

	capture := &commandCapture{}
	m.commandRunner = capture.runner
	m.execProcess = capture.exec

	cmd := m.showCommitDiff("abc123", wt)
	if cmd == nil {
		t.Fatal("expected diff command")
	}

	if capture.name != testBashCmd {
		t.Fatalf("expected bash command, got %q", capture.name)
	}
	if len(capture.args) != 2 || capture.args[0] != "-c" {
		t.Fatalf("expected bash -c args, got %v", capture.args)
	}
	cmdStr := capture.args[1]
	if !strings.Contains(cmdStr, "git difftool abc123^..abc123") {
		t.Fatalf("expected git difftool with commit range, got %q", cmdStr)
	}
	if !strings.Contains(cmdStr, "--no-prompt") {
		t.Fatalf("expected --no-prompt flag, got %q", cmdStr)
	}
	if !strings.Contains(cmdStr, "--extcmd='code --wait --diff'") {
		t.Fatalf("expected extcmd with code --wait --diff, got %q", cmdStr)
	}
	if capture.dir != repoDir {
		t.Fatalf("expected worktree dir, got %q", capture.dir)
	}
	if value, ok := envValue(capture.env, "WORKTREE_PATH"); !ok || value != repoDir {
		t.Fatalf("expected WORKTREE_PATH in env, got %q (present=%v)", value, ok)
	}
}

func TestShowCommitFileDiffVSCodeUsesGitDifftool(t *testing.T) {
	repoDir, _ := setupCommitDiffRepo(t)
	cfg := &config.AppConfig{
		WorktreeDir: repoDir,
		GitPager:    "code",
	}
	m := NewModel(cfg, "")

	capture := &commandCapture{}
	m.commandRunner = capture.runner
	m.execProcess = capture.exec

	cmd := m.showCommitFileDiff("abc123", "file.txt", repoDir)
	if cmd == nil {
		t.Fatal("expected file diff command")
	}

	if capture.name != testBashCmd {
		t.Fatalf("expected bash command, got %q", capture.name)
	}
	if len(capture.args) != 2 || capture.args[0] != "-c" {
		t.Fatalf("expected bash -c args, got %v", capture.args)
	}
	cmdStr := capture.args[1]
	if !strings.Contains(cmdStr, "git difftool abc123^..abc123") {
		t.Fatalf("expected git difftool with commit range, got %q", cmdStr)
	}
	if !strings.Contains(cmdStr, "--no-prompt") {
		t.Fatalf("expected --no-prompt flag, got %q", cmdStr)
	}
	if !strings.Contains(cmdStr, "--extcmd='code --wait --diff'") {
		t.Fatalf("expected extcmd with code --wait --diff, got %q", cmdStr)
	}
	if !strings.Contains(cmdStr, "-- 'file.txt'") {
		t.Fatalf("expected file specifier, got %q", cmdStr)
	}
	if capture.dir != repoDir {
		t.Fatalf("expected worktree dir, got %q", capture.dir)
	}
	if value, ok := envValue(capture.env, "WORKTREE_PATH"); !ok || value != repoDir {
		t.Fatalf("expected WORKTREE_PATH in env, got %q (present=%v)", value, ok)
	}
}

func setupCommitDiffRepo(t *testing.T) (string, string) {
	t.Helper()
	repoDir := t.TempDir()
	runGitCommand(t, repoDir, "init")
	runGitCommand(t, repoDir, "config", "user.email", "test@example.com")
	runGitCommand(t, repoDir, "config", "user.name", "lazyworktree")
	if err := os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("line\n"), 0o600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	runGitCommand(t, repoDir, "add", "file.txt")
	runGitCommand(t, repoDir, "-c", "commit.gpgsign=false", "commit", "-m", "initial")
	branch := strings.TrimSpace(runGitCommand(t, repoDir, "rev-parse", "--abbrev-ref", "HEAD"))
	return repoDir, branch
}

func runGitCommand(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...) //#nosec G204 -- test helper with controlled args
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return string(out)
}

func TestOpenURLInBrowserUsesCommandRunner(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	capture := &commandCapture{}
	m.commandRunner = capture.runner
	m.startCommand = capture.start

	testURL := "https://example.com/ci/logs"
	cmd := m.openURLInBrowser(testURL)
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}
	_ = cmd()

	expected := "xdg-open"
	switch runtime.GOOS {
	case osDarwin:
		expected = "open"
	case osWindows:
		expected = "rundll32"
	}
	if capture.name != expected {
		t.Fatalf("expected %q command, got %q", expected, capture.name)
	}
	if runtime.GOOS == osWindows {
		if len(capture.args) < 2 || capture.args[1] != testURL {
			t.Fatalf("expected windows URL args, got %v", capture.args)
		}
	} else {
		if len(capture.args) != 1 || capture.args[0] != testURL {
			t.Fatalf("expected URL arg, got %v", capture.args)
		}
	}
}

func TestShowCICheckLogExternalCIOpensInBrowser(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{
			Path:   testWorktreePath,
			Branch: "feat",
		},
	}
	m.state.data.selectedIndex = 0

	capture := &commandCapture{}
	m.commandRunner = capture.runner
	m.startCommand = capture.start

	// External CI link (not a GitHub Actions URL)
	externalLink := "https://console.tekton.dev/pipelines/runs/12345"
	check := &models.CICheck{
		Name: "tekton-build",
		Link: externalLink,
	}

	cmd := m.showCICheckLog(check)
	if cmd == nil {
		t.Fatal("expected command to be returned for external CI link")
	}
	_ = cmd()

	// Should open in browser
	expected := "xdg-open"
	switch runtime.GOOS {
	case osDarwin:
		expected = "open"
	case osWindows:
		expected = "rundll32"
	}
	if capture.name != expected {
		t.Fatalf("expected %q command for external CI, got %q", expected, capture.name)
	}
	if runtime.GOOS == osWindows {
		if len(capture.args) < 2 || capture.args[1] != externalLink {
			t.Fatalf("expected windows URL args, got %v", capture.args)
		}
	} else {
		if len(capture.args) != 1 || capture.args[0] != externalLink {
			t.Fatalf("expected URL arg, got %v", capture.args)
		}
	}
}

func TestShowCICheckLogEmptyLinkShowsInfo(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{
			Path:   testWorktreePath,
			Branch: "feat",
		},
	}
	m.state.data.selectedIndex = 0

	check := &models.CICheck{
		Name: "some-check",
		Link: "",
	}

	cmd := m.showCICheckLog(check)
	if cmd != nil {
		t.Fatal("expected nil command for empty link")
	}
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Fatalf("expected info screen, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}
	infoScr := m.state.ui.screenManager.Current().(*appscreen.InfoScreen)
	if !strings.Contains(infoScr.Message, "No link available") {
		t.Fatalf("expected info message about no link, got %q", infoScr.Message)
	}
}

func TestExtractRunIDFromLink(t *testing.T) {
	tests := []struct {
		name     string
		link     string
		expected string
	}{
		{
			name:     "GitHub Actions URL with job",
			link:     "https://github.com/owner/repo/actions/runs/12345678/job/98765432",
			expected: "12345678",
		},
		{
			name:     "GitHub Actions URL without job",
			link:     "https://github.com/owner/repo/actions/runs/12345678",
			expected: "12345678",
		},
		{
			name:     "External CI URL",
			link:     "https://console.tekton.dev/pipelines/runs/12345",
			expected: "",
		},
		{
			name:     "Empty link",
			link:     "",
			expected: "",
		},
		{
			name:     "Invalid URL",
			link:     "not-a-url",
			expected: "",
		},
		{
			name:     "GitHub URL without runs",
			link:     "https://github.com/owner/repo/actions",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractRunIDFromLink(tt.link)
			if result != tt.expected {
				t.Errorf("extractRunIDFromLink(%q) = %q, want %q", tt.link, result, tt.expected)
			}
		})
	}
}

func TestOpenCICheckSelectionNoChecks(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{
			Path:   testWorktreePath,
			Branch: "feat",
		},
	}
	m.state.data.selectedIndex = 0

	// No CI checks in cache
	cmd := m.openCICheckSelection()
	if cmd != nil {
		t.Fatal("expected nil command when no CI checks available")
	}
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Fatalf("expected info screen, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}
	infoScr := m.state.ui.screenManager.Current().(*appscreen.InfoScreen)
	if !strings.Contains(infoScr.Message, "No CI checks available") {
		t.Fatalf("expected info message about no CI checks, got %q", infoScr.Message)
	}
}

func TestOpenCICheckSelectionWithChecks(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{
			Path:   testWorktreePath,
			Branch: "feat",
		},
	}
	m.state.data.selectedIndex = 0
	m.setWindowSize(120, 40)

	// Add CI checks to cache
	m.cache.ciCache.Set("feat", []*models.CICheck{
		{Name: "build", Conclusion: "success", Link: "https://github.com/owner/repo/actions/runs/123"},
		{Name: "test", Conclusion: "failure", Link: "https://tekton.dev/runs/456"},
	})

	cmd := m.openCICheckSelection()
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeListSelect {
		t.Fatalf("expected list screen to be active")
	}
	listScreen := m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)
	if listScreen == nil {
		t.Fatal("expected listScreen to be set")
	}
}

func TestCICheckSelectionColouredIcons(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.theme = theme.Dracula()
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: testWorktreePath, Branch: "feat"},
	}
	m.state.data.selectedIndex = 0
	m.setWindowSize(120, 40)

	m.cache.ciCache.Set("feat", []*models.CICheck{
		{Name: "build", Conclusion: "success"},
		{Name: "test", Conclusion: "failure"},
		{Name: "lint", Conclusion: "skipped"},
		{Name: "deploy", Conclusion: "pending"},
	})

	m.openCICheckSelection()

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeListSelect {
		t.Fatal("expected list screen to be active")
	}
	listScreen := m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)
	if len(listScreen.Items) != 4 {
		t.Fatalf("expected 4 items, got %d", len(listScreen.Items))
	}

	expectedNames := []string{"build", "test", "lint", "deploy"}
	for i, item := range listScreen.Items {
		if !strings.Contains(item.Label, expectedNames[i]) {
			t.Errorf("expected label to contain %q, got %q", expectedNames[i], item.Label)
		}
	}
}

func TestExtractJobIDFromLink(t *testing.T) {
	tests := []struct {
		name     string
		link     string
		expected string
	}{
		{
			name:     "GitHub Actions URL with job",
			link:     "https://github.com/owner/repo/actions/runs/12345678/job/98765432",
			expected: "98765432",
		},
		{
			name:     "GitHub Actions URL without job",
			link:     "https://github.com/owner/repo/actions/runs/12345678",
			expected: "",
		},
		{
			name:     "External CI URL",
			link:     "https://console.tekton.dev/pipelines/runs/12345",
			expected: "",
		},
		{
			name:     "Empty link",
			link:     "",
			expected: "",
		},
		{
			name:     "Invalid URL",
			link:     "not-a-url",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractJobIDFromLink(tt.link)
			if result != tt.expected {
				t.Errorf("extractJobIDFromLink(%q) = %q, want %q", tt.link, result, tt.expected)
			}
		})
	}
}

func TestExtractRepoFromLink(t *testing.T) {
	tests := []struct {
		name     string
		link     string
		expected string
	}{
		{
			name:     "GitHub Actions URL",
			link:     "https://github.com/owner/repo/actions/runs/12345678/job/98765432",
			expected: "owner/repo",
		},
		{
			name:     "GitHub repo URL",
			link:     "https://github.com/myorg/myrepo",
			expected: "myorg/myrepo",
		},
		{
			name:     "External CI URL",
			link:     "https://console.tekton.dev/pipelines/runs/12345",
			expected: "",
		},
		{
			name:     "Empty link",
			link:     "",
			expected: "",
		},
		{
			name:     "Invalid URL",
			link:     "not-a-url",
			expected: "",
		},
		{
			name:     "GitHub URL with only owner",
			link:     "https://github.com/owner",
			expected: "",
		},
		{
			name:     "GitHub URL with trailing slash",
			link:     "https://github.com/owner/repo/",
			expected: "owner/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractRepoFromLink(tt.link)
			if result != tt.expected {
				t.Errorf("extractRepoFromLink(%q) = %q, want %q", tt.link, result, tt.expected)
			}
		})
	}
}

func TestOpenCICheckSelectionStoresChecks(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")
	m.state.data.filteredWts = []*models.WorktreeInfo{
		{Path: testWorktreePath, Branch: "feat"},
	}
	m.state.data.selectedIndex = 0
	m.setWindowSize(120, 40)

	checks := []*models.CICheck{
		{Name: "build", Conclusion: "success", Link: "https://github.com/owner/repo/actions/runs/123/job/456"},
		{Name: "test", Conclusion: "failure", Link: "https://github.com/owner/repo/actions/runs/123/job/789"},
	}
	m.cache.ciCache.Set("feat", checks)

	m.openCICheckSelection()

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeListSelect {
		t.Fatal("expected list screen to be active")
	}
	listScreen := m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)
	if len(listScreen.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(listScreen.Items))
	}
	if listScreen.Items[0].ID != "0" {
		t.Errorf("expected first item ID to be '0' (index), got %q", listScreen.Items[0].ID)
	}
	if !strings.Contains(listScreen.Items[0].Label, "build") {
		t.Errorf("expected first item label to contain 'build', got %q", listScreen.Items[0].Label)
	}
}

func TestClearListSelectionClearsCIChecks(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	m.clearListSelection()

	if m.state.ui.screenManager.IsActive() {
		t.Fatalf("expected no screen, got %v", m.state.ui.screenManager.Type())
	}
}
