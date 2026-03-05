package multiplexer

import (
	"os"
	"testing"

	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectContainerRuntime(t *testing.T) {
	t.Parallel()
	t.Run("explicit runtime returned as-is", func(t *testing.T) {
		t.Parallel()
		// Use a known binary
		rt, err := DetectContainerRuntime("sh")
		require.NoError(t, err)
		assert.Equal(t, "sh", rt)
	})
	t.Run("explicit runtime not found", func(t *testing.T) {
		t.Parallel()
		_, err := DetectContainerRuntime("nonexistent-binary-xyz")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found in PATH")
	})
}

func TestBuildContainerCommand(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		cfg             *config.ContainerCommand
		command         string
		worktreePath    string
		env             map[string]string
		wantContains    []string
		wantNotContains []string
		wantErr         bool
	}{
		{
			name:         "basic image and command",
			cfg:          &config.ContainerCommand{Image: "golang:1.22", Runtime: "echo"},
			command:      "go test ./...",
			worktreePath: "/home/user/worktrees/feature",
			env:          map[string]string{},
			wantContains: []string{"'echo'", "'run'", "'--rm'", "'golang:1.22'", "go test"},
		},
		{
			name:         "auto-mount worktree to /workspace",
			cfg:          &config.ContainerCommand{Image: "alpine", Runtime: "echo"},
			command:      "ls",
			worktreePath: "/home/user/wt/feat",
			env:          map[string]string{},
			wantContains: []string{"/home/user/wt/feat:/workspace", "-w", "/workspace"},
		},
		{
			name:         "custom working dir",
			cfg:          &config.ContainerCommand{Image: "alpine", Runtime: "echo", WorkingDir: "/src"},
			command:      "make",
			worktreePath: "/wt/feat",
			env:          map[string]string{},
			wantContains: []string{"'/src'", "/wt/feat:/src"},
		},
		{
			name: "explicit mounts",
			cfg: &config.ContainerCommand{
				Image: "alpine", Runtime: "echo",
				Mounts: []config.ContainerMount{
					{Source: "/tmp/cache", Target: "/cache", ReadOnly: true},
				},
			},
			command:      "build",
			worktreePath: "/wt/feat",
			env:          map[string]string{},
			wantContains: []string{"/tmp/cache:/cache:ro", "/wt/feat:/workspace"},
		},
		{
			name: "no duplicate mount when target matches workdir",
			cfg: &config.ContainerCommand{
				Image: "alpine", Runtime: "echo",
				Mounts: []config.ContainerMount{
					{Source: "/custom/path", Target: "/workspace"},
				},
			},
			command:         "test",
			worktreePath:    "/wt/feat",
			env:             map[string]string{},
			wantContains:    []string{"/custom/path:/workspace"},
			wantNotContains: []string{"/wt/feat:/workspace"},
		},
		{
			name:         "env vars forwarded",
			cfg:          &config.ContainerCommand{Image: "alpine", Runtime: "echo"},
			command:      "echo hi",
			worktreePath: "/wt",
			env:          map[string]string{"WORKTREE_NAME": "feat", "WORKTREE_PATH": "/wt"},
			wantContains: []string{"WORKTREE_NAME=feat", "WORKTREE_PATH=/workspace"},
		},
		{
			name:            "WORKTREE_PATH and MAIN_WORKTREE_PATH remapped to container workdir",
			cfg:             &config.ContainerCommand{Image: "alpine", Runtime: "echo", WorkingDir: "/src"},
			command:         "ls",
			worktreePath:    "/home/user/repos/project/feat",
			env:             map[string]string{"WORKTREE_PATH": "/home/user/repos/project/feat", "MAIN_WORKTREE_PATH": "/home/user/repos/project"},
			wantContains:    []string{"WORKTREE_PATH=/src", "MAIN_WORKTREE_PATH=/src"},
			wantNotContains: []string{"WORKTREE_PATH=/home", "MAIN_WORKTREE_PATH=/home"},
		},
		{
			name: "container env vars",
			cfg: &config.ContainerCommand{
				Image: "alpine", Runtime: "echo",
				Env: map[string]string{"NODE_ENV": "test"},
			},
			command:      "npm test",
			worktreePath: "/wt",
			env:          map[string]string{},
			wantContains: []string{"NODE_ENV=test"},
		},
		{
			name: "extra args pass-through",
			cfg: &config.ContainerCommand{
				Image: "alpine", Runtime: "echo",
				ExtraArgs: []string{"--network=host", "--privileged"},
			},
			command:      "make",
			worktreePath: "/wt",
			env:          map[string]string{},
			wantContains: []string{"--network=host", "--privileged"},
		},
		{
			name: "command as entrypoint with args",
			cfg: &config.ContainerCommand{
				Image: "alpine", Runtime: "echo",
				Args: []string{"--flag1", "--flag2"},
			},
			command:         "claude",
			worktreePath:    "/wt",
			env:             map[string]string{},
			wantContains:    []string{"'--entrypoint'", "'claude'", "'alpine'", "'--flag1'", "'--flag2'"},
			wantNotContains: []string{"'sh'", "'-c'"},
		},
		{
			name: "empty command with args uses image default entrypoint",
			cfg: &config.ContainerCommand{
				Image: "alpine", Runtime: "echo",
				Args: []string{"arg1", "arg2"},
			},
			command:         "",
			worktreePath:    "/wt",
			env:             map[string]string{},
			wantContains:    []string{"'alpine'", "'arg1'", "'arg2'"},
			wantNotContains: []string{"--entrypoint"},
		},
		{
			name:            "empty command no args gives image defaults",
			cfg:             &config.ContainerCommand{Image: "alpine", Runtime: "echo"},
			command:         "",
			worktreePath:    "/wt",
			env:             map[string]string{},
			wantContains:    []string{"'alpine'"},
			wantNotContains: []string{"--entrypoint"},
		},
		{
			name:         "empty command gives image only",
			cfg:          &config.ContainerCommand{Image: "alpine", Runtime: "echo"},
			command:      "",
			worktreePath: "/wt",
			env:          map[string]string{},
			wantContains: []string{"'echo'", "'run'", "'--rm'", "'alpine'"},
		},
		{
			name: "tilde in mount source expands to home dir",
			cfg: &config.ContainerCommand{
				Image: "alpine", Runtime: "echo",
				Mounts: []config.ContainerMount{
					{Source: "~/.claude", Target: "/config"},
				},
			},
			command:      "ls",
			worktreePath: "/wt",
			env:          map[string]string{},
			wantContains: []string{os.Getenv("HOME") + "/.claude:/config"},
		},
		{
			name:         "missing runtime binary errors",
			cfg:          &config.ContainerCommand{Image: "alpine", Runtime: "nonexistent-binary-xyz"},
			command:      "ls",
			worktreePath: "/wt",
			env:          map[string]string{},
			wantErr:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := BuildContainerCommand(tt.cfg, tt.command, tt.worktreePath, tt.env, false)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			for _, want := range tt.wantContains {
				assert.Contains(t, got, want, "output %q missing %q", got, want)
			}
			for _, notWant := range tt.wantNotContains {
				assert.NotContains(t, got, notWant, "output %q should not contain %q", got, notWant)
			}
		})
	}
}

func TestBuildContainerCommandInteractive(t *testing.T) {
	t.Parallel()
	t.Run("interactive true adds -it flags", func(t *testing.T) {
		t.Parallel()
		cfg := &config.ContainerCommand{Image: "alpine", Runtime: "echo"}
		got, err := BuildContainerCommand(cfg, "bash", "/wt", map[string]string{}, true)
		require.NoError(t, err)
		assert.Contains(t, got, "'-it'")
	})
	t.Run("interactive false omits -it flags", func(t *testing.T) {
		t.Parallel()
		cfg := &config.ContainerCommand{Image: "alpine", Runtime: "echo"}
		got, err := BuildContainerCommand(cfg, "cat /etc/os-release", "/wt", map[string]string{}, false)
		require.NoError(t, err)
		assert.NotContains(t, got, "-it")
	})
	t.Run("cfg Interactive true adds -it even when parameter is false", func(t *testing.T) {
		t.Parallel()
		cfg := &config.ContainerCommand{Image: "alpine", Runtime: "echo", Interactive: true}
		got, err := BuildContainerCommand(cfg, "bash", "/wt", map[string]string{}, false)
		require.NoError(t, err)
		assert.Contains(t, got, "'-it'")
	})
	t.Run("command as entrypoint with empty args", func(t *testing.T) {
		t.Parallel()
		cfg := &config.ContainerCommand{Image: "alpine", Runtime: "echo"}
		got, err := BuildContainerCommand(cfg, "bash", "/wt", map[string]string{}, false)
		require.NoError(t, err)
		assert.Contains(t, got, "'--entrypoint'")
		assert.Contains(t, got, "'bash'")
		assert.Contains(t, got, "'alpine'")
	})
}

func TestWrapWindowCommandsForContainer(t *testing.T) {
	t.Parallel()
	t.Run("nil container passes through", func(t *testing.T) {
		t.Parallel()
		windows := []ResolvedWindow{{Name: "main", Command: "bash", Cwd: "/wt"}}
		got, err := WrapWindowCommandsForContainer(windows, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, "bash", got[0].Command)
	})
	t.Run("wraps non-empty commands", func(t *testing.T) {
		t.Parallel()
		cfg := &config.ContainerCommand{Image: "alpine", Runtime: "echo"}
		windows := []ResolvedWindow{
			{Name: "test", Command: "go test", Cwd: "/wt/feat"},
			{Name: "shell", Command: "", Cwd: "/wt/feat"},
		}
		got, err := WrapWindowCommandsForContainer(windows, cfg, map[string]string{})
		require.NoError(t, err)
		assert.Contains(t, got[0].Command, "'echo'")
		assert.Empty(t, got[1].Command)
	})
	t.Run("preserves window metadata", func(t *testing.T) {
		t.Parallel()
		cfg := &config.ContainerCommand{Image: "alpine", Runtime: "echo"}
		windows := []ResolvedWindow{
			{Name: "build", Command: "make", Cwd: "/wt/feat"},
		}
		got, err := WrapWindowCommandsForContainer(windows, cfg, map[string]string{})
		require.NoError(t, err)
		assert.Equal(t, "build", got[0].Name)
		assert.Equal(t, "/wt/feat", got[0].Cwd)
	})
	t.Run("runtime error propagated", func(t *testing.T) {
		t.Parallel()
		cfg := &config.ContainerCommand{Image: "alpine", Runtime: "nonexistent-binary-xyz"}
		windows := []ResolvedWindow{
			{Name: "test", Command: "ls", Cwd: "/wt"},
		}
		_, err := WrapWindowCommandsForContainer(windows, cfg, map[string]string{})
		require.Error(t, err)
	})
}
