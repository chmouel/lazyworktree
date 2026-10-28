package git

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseWorktreeAdd(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want models.WorktreeAdd
		ok   bool
	}{
		{name: "new branch", args: []string{"git", "worktree", "add", "-b", "feat", "/wt/feat", "main"}, want: models.WorktreeAdd{Path: "/wt/feat", Ref: "main", NewBranch: "feat", Dir: "/repo"}, ok: true},
		{name: "tracking branch", args: []string{"git", "worktree", "add", "-b", "feat", "--track", "/wt/feat", "origin/feat"}, want: models.WorktreeAdd{Path: "/wt/feat", Ref: "origin/feat", NewBranch: "feat", Track: true, Dir: "/repo"}, ok: true},
		{name: "existing branch", args: []string{"git", "worktree", "add", "/wt/feat", "feat"}, want: models.WorktreeAdd{Path: "/wt/feat", Ref: "feat", Dir: "/repo"}, ok: true},
		{name: "relative path uses cwd", args: []string{"git", "worktree", "add", "feat", "feat"}, want: models.WorktreeAdd{Path: "/repo/feat", Ref: "feat", Dir: "/repo"}, ok: true},
		{name: "track without new branch", args: []string{"git", "worktree", "add", "--track", "/wt/feat", "origin/feat"}},
		{name: "unknown flag", args: []string{"git", "worktree", "add", "--detach", "/wt/feat", "main"}},
		{name: "missing ref", args: []string{"git", "worktree", "add", "/wt/feat"}},
		{name: "extra argument", args: []string{"git", "worktree", "add", "-b", "feat", "/wt/feat", "main", "x"}},
		{name: "other command", args: []string{"git", "worktree", "list"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseWorktreeAdd(tt.args, "/repo")
			assert.Equal(t, tt.ok, ok)
			if tt.ok {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestParseWorktreeRemove(t *testing.T) {
	path, ok := parseWorktreeRemove([]string{"git", "worktree", "remove", "--force", "/wt/feat"}, "")
	assert.True(t, ok)
	assert.Equal(t, "/wt/feat", path)

	_, ok = parseWorktreeRemove([]string{"git", "worktree", "remove", "--force", "--force", "/wt/feat"}, "")
	assert.False(t, ok)
}

type recordingWorktreeHook struct {
	adds     []models.WorktreeAdd
	addErr   error
	runGit   bool
	removals []string
	finalise []string
}

func (h *recordingWorktreeHook) AddWorktree(_ context.Context, add models.WorktreeAdd, runGit func() error) error {
	h.adds = append(h.adds, add)
	if h.runGit {
		return runGit()
	}
	return h.addErr
}

func (h *recordingWorktreeHook) WorktreeRemoved(_ context.Context, path string, removed bool) {
	h.removals = append(h.removals, fmt.Sprintf("%s=%v", path, removed))
}

func (h *recordingWorktreeHook) FinaliseWorktree(_ context.Context, path string, _ map[string]string) error {
	h.finalise = append(h.finalise, path)
	return nil
}

func (h *recordingWorktreeHook) ForgetWorktree(string) {}

func TestRunCommandCheckedRoutesWorktreeCommandsThroughHook(t *testing.T) {
	repo := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"-c", "commit.gpgsign=false", "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", args...) //nolint:gosec // test git invocation
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}

	var notified []string
	service := NewService(func(message, _ string) { notified = append(notified, message) }, func(string, string, string) {})
	hook := &recordingWorktreeHook{addErr: errors.New("herdr said no")}
	service.SetWorktreeHook(hook)
	ctx := context.Background()
	wtPath := filepath.Join(t.TempDir(), "feat")

	assert.False(t, service.RunCommandChecked(ctx, []string{"git", "worktree", "add", "-b", "feat", wtPath, "main"}, repo, "Failed to create"))
	require.Len(t, hook.adds, 1)
	assert.Equal(t, models.WorktreeAdd{Path: wtPath, Ref: "main", NewBranch: "feat", Dir: repo}, hook.adds[0])
	assert.Equal(t, []string{"Failed to create: herdr said no"}, notified)

	hook.runGit = true
	assert.True(t, service.RunCommandChecked(ctx, []string{"git", "worktree", "add", "-b", "feat", wtPath, "main"}, repo, "Failed to create"))
	assert.DirExists(t, wtPath)

	assert.True(t, service.RunCommandChecked(ctx, []string{"git", "worktree", "remove", "--force", wtPath}, repo, "Failed to remove"))
	assert.False(t, service.RunCommandChecked(ctx, []string{"git", "worktree", "remove", "--force", wtPath}, repo, "Failed to remove"))
	assert.Equal(t, []string{wtPath + "=true", wtPath + "=false"}, hook.removals)
	assert.Len(t, hook.adds, 2)
}
