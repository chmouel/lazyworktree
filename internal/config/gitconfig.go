package config

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// gitConfigMock allows tests to mock git config output.
var gitConfigMock func(args []string, repoPath string) (string, error)

// runGitConfig executes git config command and returns raw output.
func runGitConfig(args []string, repoPath string) (string, error) {
	if gitConfigMock != nil {
		return gitConfigMock(args, repoPath)
	}

	cmd := exec.Command("git", args...) //#nosec G204 -- controlled git command execution
	if repoPath != "" {
		cmd.Dir = repoPath
	}

	output, err := cmd.Output()
	if err != nil {
		// git config returns exit code 1 when key not found (not an error)
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", nil
		}
		return "", err
	}
	return string(output), nil
}

// parseGitConfigOutput parses git config output into multi-value map.
// Input format: "lw.worktree_dir /path/to/dir\nlw.auto_fetch_prs true\n"
func parseGitConfigOutput(output string) (map[string][]string, error) {
	configMap := make(map[string][]string)
	if output == "" {
		return configMap, nil
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		// Parse "lw.worktree_dir /path/to/dir"
		// Using SplitN with 2 to handle values containing spaces
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimPrefix(parts[0], "lw.")
		value := parts[1]

		// Git config can have multi-values for same key
		configMap[key] = append(configMap[key], value)
	}

	return configMap, nil
}

// convertGitConfigToParseConfig converts to format expected by parseConfig().
func convertGitConfigToParseConfig(gitCfg map[string][]string) map[string]any {
	result := make(map[string]any)

	for key, values := range gitCfg {
		if len(values) == 0 {
			continue
		}

		// Multi-value keys become arrays (e.g., init_commands)
		// parseConfig expects []any, not []string
		if len(values) > 1 {
			anySlice := make([]any, len(values))
			for i, v := range values {
				anySlice[i] = v
			}
			result[key] = anySlice
			continue
		}

		// Single value - keep as string, coerceBool/coerceInt will handle conversion
		result[key] = values[0]
	}

	return result
}

// loadGitConfig reads git config values and returns map for parseConfig.
func loadGitConfig(globalOnly bool, repoPath string) (map[string]any, error) {
	args := []string{"config", "--get-regexp", "^lw\\."}

	if globalOnly {
		args = append(args, "--global")
	} else {
		args = append(args, "--local")
	}

	output, err := runGitConfig(args, repoPath)
	if err != nil {
		return nil, err
	}

	if output == "" {
		return make(map[string]any), nil
	}

	gitCfg, err := parseGitConfigOutput(output)
	if err != nil {
		return nil, err
	}

	return convertGitConfigToParseConfig(gitCfg), nil
}

// isInGitRepo checks if path is in a git repository.
func isInGitRepo(path string) bool {
	if path == "" {
		return false
	}
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = path
	return cmd.Run() == nil
}

// determineRepoPath returns repo path for local git config lookup.
func determineRepoPath(worktreeDir string) string {
	// Try worktreeDir if specified.
	if worktreeDir != "" && isInGitRepo(worktreeDir) {
		return worktreeDir
	}

	// Fall back to current directory.
	if wd, err := os.Getwd(); err == nil && isInGitRepo(wd) {
		return wd
	}

	return ""
}

// parseCLIConfigOverrides parses --config=lw.key=value format.
// Returns a map suitable for parseConfig().
func parseCLIConfigOverrides(overrides []string) (map[string]any, error) {
	result := make(map[string]any)
	keyCount := make(map[string]int)

	for _, override := range overrides {
		// Parse "lw.key=value" format
		parts := strings.SplitN(override, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid config override: %q, expected format: lw.key=value (note: use = not space)", override)
		}

		fullKey := parts[0]
		value := parts[1]

		// Ensure key starts with "lw."
		if !strings.HasPrefix(fullKey, "lw.") {
			return nil, fmt.Errorf("config override key must start with 'lw.': %q", fullKey)
		}

		key := strings.TrimPrefix(fullKey, "lw.")
		if key == "" {
			return nil, fmt.Errorf("empty config key in override: %q", override)
		}

		// Handle multi-value keys (if same key appears multiple times)
		// parseConfig expects []any, not []string
		keyCount[key]++
		if keyCount[key] > 1 {
			// Convert to array on second occurrence
			if keyCount[key] == 2 {
				firstValue := result[key].(string)
				result[key] = []any{firstValue, value}
			} else {
				result[key] = append(result[key].([]any), value)
			}
		} else {
			result[key] = value
		}
	}

	return result, nil
}
