package git

import (
	"context"
	"os/exec"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

// captureRunner records every *exec.Cmd it hands back so a test can inspect the
// environment lazyworktree prepares for a subprocess. It points the command at
// `true` so RunGit can execute it harmlessly.
func captureRunner(captured *[]*exec.Cmd) func(ctx context.Context, name string, args ...string) *exec.Cmd {
	return func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		c := exec.CommandContext(ctx, "true")
		*captured = append(*captured, c)
		return c
	}
}

func envHas(cmd *exec.Cmd, kv string) bool {
	return slices.Contains(cmd.Env, kv)
}

// Git reads run for display must never take optional locks, so lazyworktree's
// background status/diff polling can't create .git/index.lock and race a git
// command (e.g. an agent's commit) in the same worktree.
func TestRunGitDisablesOptionalLocks(t *testing.T) {
	svc := NewService(func(string, string) {}, func(string, string, string) {})
	var captured []*exec.Cmd
	svc.SetCommandRunner(captureRunner(&captured))

	svc.RunGit(context.Background(), []string{"git", "status", "--porcelain"}, "", []int{0}, true, false)

	require.Len(t, captured, 1)
	require.True(t, envHas(captured[0], "GIT_OPTIONAL_LOCKS=0"),
		"git subprocess env should contain GIT_OPTIONAL_LOCKS=0, got %v", captured[0].Env)
}

// The combined-output path is used for worktree writes; it must keep the
// optional-locks setting from prepareAllowedCommand while still layering the
// caller's own env on top.
func TestRunGitWithCombinedOutputKeepsOptionalLocks(t *testing.T) {
	svc := NewService(func(string, string) {}, func(string, string, string) {})
	var captured []*exec.Cmd
	svc.SetCommandRunner(captureRunner(&captured))

	_, _ = svc.RunGitWithCombinedOutput(context.Background(),
		[]string{"git", "pull"}, "", map[string]string{"GIT_TERMINAL_PROMPT": "0"})

	require.Len(t, captured, 1)
	require.True(t, envHas(captured[0], "GIT_OPTIONAL_LOCKS=0"),
		"combined-output git env should retain GIT_OPTIONAL_LOCKS=0, got %v", captured[0].Env)
	require.True(t, envHas(captured[0], "GIT_TERMINAL_PROMPT=0"),
		"caller-supplied env should still be present, got %v", captured[0].Env)
}

// gh/glab are not git and must not be forced into the git lock setting.
func TestNonGitCommandsNotForcedOptionalLocks(t *testing.T) {
	svc := NewService(func(string, string) {}, func(string, string, string) {})
	var captured []*exec.Cmd
	svc.SetCommandRunner(captureRunner(&captured))

	svc.RunGit(context.Background(), []string{"gh", "pr", "list"}, "", []int{0}, true, true)

	require.Len(t, captured, 1)
	require.False(t, envHas(captured[0], "GIT_OPTIONAL_LOCKS=0"),
		"non-git command env should not be modified, got %v", captured[0].Env)
}
