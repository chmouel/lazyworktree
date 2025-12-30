package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// AppConfig represents the application configuration
type AppConfig struct {
	WorktreeDir       string
	InitCommands      []string
	TerminateCommands []string
	SortByActive      bool
	AutoFetchPRs      bool
	MaxUntrackedDiffs int
	MaxDiffChars      int
	TrustMode         string
	DebugLog          string
}

// RepoConfig represents repository-scoped commands from .wt
type RepoConfig struct {
	InitCommands      []string
	TerminateCommands []string
	Path              string
}

// DefaultConfig returns a new AppConfig with default values
func DefaultConfig() *AppConfig {
	return &AppConfig{
		SortByActive:      true,
		AutoFetchPRs:      false,
		MaxUntrackedDiffs: 10,
		MaxDiffChars:      200000,
		TrustMode:         "tofu",
	}
}

// normalizeCommandList converts various input types to a list of command strings
func normalizeCommandList(value interface{}) []string {
	if value == nil {
		return []string{}
	}

	switch v := value.(type) {
	case string:
		text := strings.TrimSpace(v)
		if text == "" {
			return []string{}
		}
		return []string{text}
	case []interface{}:
		commands := []string{}
		for _, item := range v {
			if item == nil {
				continue
			}
			text := strings.TrimSpace(fmt.Sprintf("%v", item))
			if text != "" {
				commands = append(commands, text)
			}
		}
		return commands
	}
	return []string{}
}

// coerceBool converts various types to bool
func coerceBool(value interface{}, defaultVal bool) bool {
	if value == nil {
		return defaultVal
	}

	switch v := value.(type) {
	case bool:
		return v
	case int:
		return v != 0
	case string:
		text := strings.ToLower(strings.TrimSpace(v))
		switch text {
		case "1", "true", "yes", "y", "on":
			return true
		case "0", "false", "no", "n", "off":
			return false
		}
	}
	return defaultVal
}

// coerceInt converts various types to int
func coerceInt(value interface{}, defaultVal int) int {
	if value == nil {
		return defaultVal
	}

	switch v := value.(type) {
	case bool:
		return defaultVal
	case int:
		return v
	case string:
		text := strings.TrimSpace(v)
		if text == "" {
			return defaultVal
		}
		if i, err := strconv.Atoi(text); err == nil {
			return i
		}
	}
	return defaultVal
}

// parseConfig parses YAML data into AppConfig
func parseConfig(data map[string]interface{}) *AppConfig {
	cfg := DefaultConfig()

	if worktreeDir, ok := data["worktree_dir"].(string); ok {
		worktreeDir = strings.TrimSpace(worktreeDir)
		if worktreeDir != "" {
			cfg.WorktreeDir = worktreeDir
		}
	}

	if debugLog, ok := data["debug_log"].(string); ok {
		debugLog = strings.TrimSpace(debugLog)
		if debugLog != "" {
			cfg.DebugLog = debugLog
		}
	}

	cfg.InitCommands = normalizeCommandList(data["init_commands"])
	cfg.TerminateCommands = normalizeCommandList(data["terminate_commands"])
	cfg.SortByActive = coerceBool(data["sort_by_active"], true)
	cfg.AutoFetchPRs = coerceBool(data["auto_fetch_prs"], false)
	cfg.MaxUntrackedDiffs = coerceInt(data["max_untracked_diffs"], 10)
	cfg.MaxDiffChars = coerceInt(data["max_diff_chars"], 200000)

	if trustMode, ok := data["trust_mode"].(string); ok {
		trustMode = strings.ToLower(strings.TrimSpace(trustMode))
		if trustMode == "tofu" || trustMode == "never" || trustMode == "always" {
			cfg.TrustMode = trustMode
		}
	}

	if cfg.MaxUntrackedDiffs < 0 {
		cfg.MaxUntrackedDiffs = 0
	}
	if cfg.MaxDiffChars < 0 {
		cfg.MaxDiffChars = 0
	}

	return cfg
}

// LoadRepoConfig loads repository-specific commands from .wt in repoPath
func LoadRepoConfig(repoPath string) (*RepoConfig, string, error) {
	if repoPath == "" {
		return nil, "", fmt.Errorf("empty repo path")
	}
	wtPath := filepath.Join(repoPath, ".wt")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		return nil, wtPath, nil
	}

	dataBytes, err := os.ReadFile(wtPath)
	if err != nil {
		return nil, wtPath, err
	}

	var yamlData map[string]interface{}
	if err := yaml.Unmarshal(dataBytes, &yamlData); err != nil {
		return nil, wtPath, err
	}

	cfg := &RepoConfig{
		Path:              wtPath,
		InitCommands:      normalizeCommandList(yamlData["init_commands"]),
		TerminateCommands: normalizeCommandList(yamlData["terminate_commands"]),
	}
	return cfg, wtPath, nil
}

// getConfigDir returns the XDG config directory
func getConfigDir() string {
	if xdgConfigHome := os.Getenv("XDG_CONFIG_HOME"); xdgConfigHome != "" {
		return xdgConfigHome
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}

// LoadConfig loads configuration from file
func LoadConfig(configPath string) (*AppConfig, error) {
	var paths []string

	if configPath != "" {
		expanded, err := expandPath(configPath)
		if err != nil {
			return DefaultConfig(), err
		}
		paths = []string{expanded}
	} else {
		configDir := getConfigDir()
		paths = []string{
			filepath.Join(configDir, "lazyworktree", "config.yaml"),
			filepath.Join(configDir, "lazyworktree", "config.yml"),
		}
	}

	for _, path := range paths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}

		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var yamlData map[string]interface{}
		if err := yaml.Unmarshal(data, &yamlData); err != nil {
			return DefaultConfig(), nil
		}

		return parseConfig(yamlData), nil
	}

	return DefaultConfig(), nil
}

// expandPath expands ~ and environment variables in a path
func expandPath(path string) (string, error) {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, path[1:])
	}
	return os.ExpandEnv(path), nil
}
