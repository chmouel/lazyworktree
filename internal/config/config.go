// Package config loads application and repository configuration from YAML.
package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// CustomCommand represents a user-defined command binding.
type CustomCommand struct {
	Command     string
	Description string
	ShowHelp    bool
	Wait        bool
}

// AppConfig defines the global lazyworktree configuration options.
type AppConfig struct {
	WorktreeDir       string
	InitCommands      []string
	TerminateCommands []string
	SortByActive      bool
	AutoFetchPRs      bool
	MaxUntrackedDiffs int
	MaxDiffChars      int
	DeltaArgs         []string
	TrustMode         string
	DebugLog          string
	CustomCommands    map[string]*CustomCommand
	BranchNameScript  string // Script to generate branch name suggestions from diff
}

// RepoConfig represents repository-scoped commands from .wt
type RepoConfig struct {
	InitCommands      []string
	TerminateCommands []string
	Path              string
}

// DefaultConfig returns the default configuration values.
func DefaultConfig() *AppConfig {
	return &AppConfig{
		SortByActive:      true,
		AutoFetchPRs:      false,
		MaxUntrackedDiffs: 10,
		MaxDiffChars:      200000,
		DeltaArgs:         []string{"--syntax-theme", "Dracula"},
		TrustMode:         "tofu",
		CustomCommands:    make(map[string]*CustomCommand),
	}
}

// normalizeCommandList converts various input types to a list of command strings
func normalizeCommandList(value any) []string {
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
	case []any:
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

func normalizeArgsList(value any) []string {
	if value == nil {
		return []string{}
	}

	switch v := value.(type) {
	case string:
		text := strings.TrimSpace(v)
		if text == "" {
			return []string{}
		}
		return strings.Fields(text)
	case []any:
		args := []string{}
		for _, item := range v {
			if item == nil {
				continue
			}
			text := strings.TrimSpace(fmt.Sprintf("%v", item))
			if text != "" {
				args = append(args, text)
			}
		}
		return args
	}

	return []string{}
}

func coerceBool(value any, defaultVal bool) bool {
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

func coerceInt(value any, defaultVal int) int {
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

func parseCustomCommands(data map[string]any) map[string]*CustomCommand {
	commands := make(map[string]*CustomCommand)

	raw, ok := data["custom_commands"].(map[string]any)
	if !ok {
		return commands
	}

	for key, val := range raw {
		cmdMap, ok := val.(map[string]any)
		if !ok {
			continue
		}

		cmd := &CustomCommand{}
		if cmdStr, ok := cmdMap["command"].(string); ok {
			cmd.Command = strings.TrimSpace(cmdStr)
		}
		if descStr, ok := cmdMap["description"].(string); ok {
			cmd.Description = strings.TrimSpace(descStr)
		}
		cmd.ShowHelp = coerceBool(cmdMap["show_help"], false)
		cmd.Wait = coerceBool(cmdMap["wait"], false)

		// Only add if command is not empty
		if cmd.Command != "" {
			commands[key] = cmd
		}
	}

	return commands
}

func parseConfig(data map[string]any) *AppConfig {
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
	if _, ok := data["delta_args"]; ok {
		cfg.DeltaArgs = normalizeArgsList(data["delta_args"])
	}

	if trustMode, ok := data["trust_mode"].(string); ok {
		trustMode = strings.ToLower(strings.TrimSpace(trustMode))
		if trustMode == "tofu" || trustMode == "never" || trustMode == "always" {
			cfg.TrustMode = trustMode
		}
	}

	if branchNameScript, ok := data["branch_name_script"].(string); ok {
		branchNameScript = strings.TrimSpace(branchNameScript)
		if branchNameScript != "" {
			cfg.BranchNameScript = branchNameScript
		}
	}

	if cfg.MaxUntrackedDiffs < 0 {
		cfg.MaxUntrackedDiffs = 0
	}
	if cfg.MaxDiffChars < 0 {
		cfg.MaxDiffChars = 0
	}

	cfg.CustomCommands = parseCustomCommands(data)

	return cfg
}

// LoadRepoConfig loads repository-specific commands from .wt in repoPath
func LoadRepoConfig(repoPath string) (*RepoConfig, string, error) {
	if repoPath == "" {
		return nil, "", fmt.Errorf("empty repo path")
	}
	cleanRepoPath := filepath.Clean(repoPath)
	wtPath := filepath.Join(cleanRepoPath, ".wt")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		return nil, wtPath, nil
	}

	if !isPathWithin(cleanRepoPath, wtPath) {
		return nil, "", fmt.Errorf("invalid repo path %q", repoPath)
	}

	dataBytes, err := fs.ReadFile(os.DirFS(cleanRepoPath), ".wt")
	if err != nil {
		return nil, wtPath, fmt.Errorf("failed to read .wt file: %w", err)
	}

	var yamlData map[string]any
	if err := yaml.Unmarshal(dataBytes, &yamlData); err != nil {
		return nil, wtPath, fmt.Errorf("failed to parse .wt file: %w", err)
	}

	cfg := &RepoConfig{
		Path:              wtPath,
		InitCommands:      normalizeCommandList(yamlData["init_commands"]),
		TerminateCommands: normalizeCommandList(yamlData["terminate_commands"]),
	}
	return cfg, wtPath, nil
}

func getConfigDir() string {
	if xdgConfigHome := os.Getenv("XDG_CONFIG_HOME"); xdgConfigHome != "" {
		return xdgConfigHome
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}

// LoadConfig reads the application configuration from a YAML file.
func LoadConfig(configPath string) (*AppConfig, error) {
	configBase := filepath.Join(getConfigDir(), "lazyworktree")
	configBase = filepath.Clean(configBase)

	var paths []string

	if configPath != "" {
		expanded, err := expandPath(configPath)
		if err != nil {
			return DefaultConfig(), err
		}
		absPath, err := filepath.Abs(expanded)
		if err != nil {
			return DefaultConfig(), err
		}
		if !isPathWithin(configBase, absPath) {
			return DefaultConfig(), fmt.Errorf("config path must reside inside %s", configBase)
		}
		paths = []string{absPath}
	} else {
		paths = []string{
			filepath.Join(configBase, "config.yaml"),
			filepath.Join(configBase, "config.yml"),
		}
	}

	for _, path := range paths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}

		// #nosec G304 -- path is constrained to the config directory after validation
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var yamlData map[string]any
		if err := yaml.Unmarshal(data, &yamlData); err != nil {
			return DefaultConfig(), nil
		}

		return parseConfig(yamlData), nil
	}

	return DefaultConfig(), nil
}

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

func isPathWithin(base, target string) bool {
	base = filepath.Clean(base)
	target = filepath.Clean(target)

	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return false
	}
	return true
}
