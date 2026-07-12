package git

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

const helperProcessEnv = "GO_WANT_GIT_SERVICE_HELPER_PROCESS"

func TestGitServiceHelperProcess(*testing.T) {
	if os.Getenv(helperProcessEnv) != "1" {
		return
	}
	os.Exit(0)
}

func helperProcessRunner(captured **exec.Cmd, initialEnv ...string) func(context.Context, string, ...string) *exec.Cmd {
	return func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestGitServiceHelperProcess", "--") // #nosec G204,G702 -- the test binary and arguments are controlled
		cmd.Env = append([]string{helperProcessEnv + "=1"}, initialEnv...)
		*captured = cmd
		return cmd
	}
}

func TestPrepareAllowedCommandOverridesInheritedOptionalLocks(t *testing.T) {
	t.Setenv("GIT_OPTIONAL_LOCKS", "1")
	service := NewService(func(string, string) {}, func(string, string, string) {})
	service.SetCommandRunner(func(context.Context, string, ...string) *exec.Cmd {
		return &exec.Cmd{}
	})

	cmd, err := service.prepareAllowedCommand(context.Background(), []string{"git", "status"}, nil)

	require.NoError(t, err)
	require.Equal(t, []string{"0"}, envValues(cmd.Env, "GIT_OPTIONAL_LOCKS"))
}

func envValues(env []string, key string) []string {
	prefix := key + "="
	values := make([]string, 0, 1)
	for _, entry := range env {
		if len(entry) >= len(prefix) && entry[:len(prefix)] == prefix {
			values = append(values, entry[len(prefix):])
		}
	}
	return values
}

func TestRunGitDisablesOptionalLocks(t *testing.T) {
	service := NewService(func(string, string) {}, func(string, string, string) {})
	var captured *exec.Cmd
	service.SetCommandRunner(helperProcessRunner(
		&captured,
		"RUNNER_ENV=preserved",
		"GIT_OPTIONAL_LOCKS=1",
	))

	service.RunGit(context.Background(), []string{"git", "status", "--porcelain"}, "", []int{0}, true, false)

	require.NotNil(t, captured)
	require.Equal(t, []string{"0"}, envValues(captured.Env, "GIT_OPTIONAL_LOCKS"))
	require.Equal(t, []string{"preserved"}, envValues(captured.Env, "RUNNER_ENV"))
}

func TestRunGitWithCombinedOutputForcesOptionalLocks(t *testing.T) {
	service := NewService(func(string, string) {}, func(string, string, string) {})
	var captured *exec.Cmd
	service.SetCommandRunner(helperProcessRunner(
		&captured,
		"RUNNER_ENV=preserved",
		"GIT_OPTIONAL_LOCKS=true",
	))

	_, err := service.RunGitWithCombinedOutput(
		context.Background(),
		[]string{"git", "pull"},
		"",
		map[string]string{
			"CALLER_ENV":         "preserved",
			"GIT_OPTIONAL_LOCKS": "1",
		},
	)

	require.NoError(t, err)
	require.NotNil(t, captured)
	require.Equal(t, []string{"0"}, envValues(captured.Env, "GIT_OPTIONAL_LOCKS"))
	require.Equal(t, []string{"preserved"}, envValues(captured.Env, "RUNNER_ENV"))
	require.Equal(t, []string{"preserved"}, envValues(captured.Env, "CALLER_ENV"))
}

func TestNonGitCommandsKeepOptionalLocksEnvironment(t *testing.T) {
	service := NewService(func(string, string) {}, func(string, string, string) {})
	var captured *exec.Cmd
	service.SetCommandRunner(helperProcessRunner(&captured, "GIT_OPTIONAL_LOCKS=1"))

	service.RunGit(context.Background(), []string{"gh", "pr", "list"}, "", []int{0}, true, true)

	require.NotNil(t, captured)
	require.Equal(t, []string{"1"}, envValues(captured.Env, "GIT_OPTIONAL_LOCKS"))
}
