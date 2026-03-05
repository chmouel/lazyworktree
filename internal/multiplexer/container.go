package multiplexer

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/log"
	"github.com/chmouel/lazyworktree/internal/utils"
)

// DetectContainerRuntime returns the container runtime binary to use.
// If runtime is non-empty, it validates that the binary exists.
// Otherwise prefers podman, falls back to docker.
func DetectContainerRuntime(runtime string) (string, error) {
	if runtime != "" {
		if _, err := exec.LookPath(runtime); err != nil {
			return "", fmt.Errorf("container runtime %q not found in PATH", runtime)
		}
		return runtime, nil
	}
	if _, err := exec.LookPath("podman"); err == nil {
		return "podman", nil
	}
	if _, err := exec.LookPath("docker"); err == nil {
		return "docker", nil
	}
	return "", fmt.Errorf("no container runtime found: install docker or podman")
}

// BuildContainerCommand wraps a shell command in a container run invocation.
// The worktree path is auto-mounted to the working directory (default /workspace).
// Environment variables from env are forwarded into the container via -e flags.
// When interactive is true, -it flags are added for TTY allocation.
func BuildContainerCommand(cfg *config.ContainerCommand, command, worktreePath string, env map[string]string, interactive bool) (string, error) {
	runtime, err := DetectContainerRuntime(cfg.Runtime)
	if err != nil {
		return "", err
	}
	log.Printf("container: detected runtime %q", runtime)

	// Honour interactive from either the call site or the config field
	interactive = interactive || cfg.Interactive

	var args []string
	args = append(args, runtime, "run", "--rm")

	if interactive {
		log.Printf("container: interactive mode enabled")
		args = append(args, "-it")
	}

	workDir := cfg.WorkingDir
	if workDir == "" {
		workDir = "/workspace"
	}
	if cfg.Entrypoint != "" {
		log.Printf("container: entrypoint override %q", cfg.Entrypoint)
		args = append(args, "--entrypoint", cfg.Entrypoint)
	}

	args = append(args, "-w", workDir)

	// Auto-mount worktree unless user already mounts the working dir
	hasWorkDirMount := false
	for _, m := range cfg.Mounts {
		if m.Target == workDir {
			hasWorkDirMount = true
		}
	}
	if !hasWorkDirMount && worktreePath != "" {
		args = append(args, "-v", worktreePath+":"+workDir)
	}

	// User-specified mounts
	for _, m := range cfg.Mounts {
		source, err := utils.ExpandPath(m.Source)
		if err != nil {
			return "", fmt.Errorf("expanding mount source %q: %w", m.Source, err)
		}
		mountStr := source + ":" + m.Target
		if m.ReadOnly {
			mountStr += ":ro"
		}
		args = append(args, "-v", mountStr)
	}

	// Forward env vars (sorted for determinism)
	envKeys := make([]string, 0, len(env))
	for k := range env {
		envKeys = append(envKeys, k)
	}
	sort.Strings(envKeys)
	for _, k := range envKeys {
		args = append(args, "-e", k+"="+env[k])
	}

	// Container-specific env vars (sorted for determinism)
	if len(cfg.Env) > 0 {
		cfgKeys := make([]string, 0, len(cfg.Env))
		for k := range cfg.Env {
			cfgKeys = append(cfgKeys, k)
		}
		sort.Strings(cfgKeys)
		for _, k := range cfgKeys {
			args = append(args, "-e", k+"="+cfg.Env[k])
		}
	}

	// Extra args pass-through
	args = append(args, cfg.ExtraArgs...)

	// Image
	args = append(args, cfg.Image)

	// Command
	if command != "" {
		args = append(args, "sh", "-c", command)
	}

	// Shell-quote each arg
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = ShellQuote(a)
	}
	result := strings.Join(quoted, " ")
	log.Printf("container: final command: %s", result)
	return result, nil
}

// WrapWindowCommandsForContainer wraps each window's command in a container
// invocation. Windows with empty commands are left unchanged.
func WrapWindowCommandsForContainer(windows []ResolvedWindow, containerCfg *config.ContainerCommand, env map[string]string) ([]ResolvedWindow, error) {
	if containerCfg == nil {
		return windows, nil
	}
	wrapped := make([]ResolvedWindow, len(windows))
	for i, w := range windows {
		wrapped[i] = w
		if w.Command != "" {
			containerCmd, err := BuildContainerCommand(containerCfg, w.Command, w.Cwd, env, true)
			if err != nil {
				return nil, err
			}
			wrapped[i].Command = containerCmd
		}
	}
	return wrapped, nil
}
