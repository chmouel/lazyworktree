// Package config loads application and repository configuration from YAML.
package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chmouel/lazyworktree/internal/theme"
	"gopkg.in/yaml.v3"
)

// CustomCommand represents a user-defined command binding.
type CustomCommand struct {
	Command     string
	Description string
	ShowHelp    bool
	Wait        bool
	ShowOutput  bool
	Tmux        *TmuxCommand
	Zellij      *TmuxCommand
}

// TmuxCommand represents a configured tmux session layout.
type TmuxCommand struct {
	SessionName string
	Attach      bool
	OnExists    string
	Windows     []TmuxWindow
}

// TmuxWindow represents a tmux window configuration.
type TmuxWindow struct {
	Name    string
	Command string
	Cwd     string
}

// AppConfig defines the global lazyworktree configuration options.
type AppConfig struct {
	WorktreeDir             string
	InitCommands            []string
	TerminateCommands       []string
	SortMode                string // Sort mode: "path", "active" (commit date), "switched" (last accessed)
	AutoFetchPRs            bool
	SearchAutoSelect        bool // Start with filter focused and select first match on Enter.
	MaxUntrackedDiffs       int
	MaxDiffChars            int
	DeltaArgs               []string
	DeltaArgsSet            bool `yaml:"-"`
	DeltaPath               string
	TrustMode               string
	DebugLog                string
	Pager                   string
	Editor                  string
	CustomCommands          map[string]*CustomCommand
	BranchNameScript        string // Script to generate branch name suggestions from diff
	Theme                   string // Theme name: see AvailableThemes in internal/theme
	MergeMethod             string // Merge method for absorb: "rebase" or "merge" (default: "rebase")
	FuzzyFinderInput        bool   // Enable fuzzy finder for input suggestions (default: false)
	ShowIcons               bool   // Render Nerd Font icons in file trees and PR views (default: true)
	IssuePrefix             string // Prefix for issue branch names (default: "issue")
	IssueBranchNameTemplate string // Template for issue branch names with placeholders: {prefix}, {number}, {title} (default: "{prefix}-{number}-{title}")
	PRPrefix                string // Prefix for PR branch names (default: "pr")
	PRBranchNameTemplate    string // Template for PR branch names with placeholders: {prefix}, {number}, {title} (default: "{prefix}-{number}-{title}")
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
		SortMode:                "switched",
		AutoFetchPRs:            false,
		SearchAutoSelect:        false,
		MaxUntrackedDiffs:       10,
		MaxDiffChars:            200000,
		DeltaArgs:               DefaultDeltaArgsForTheme(theme.DraculaName),
		DeltaPath:               "delta",
		TrustMode:               "tofu",
		Theme:                   "",
		MergeMethod:             "rebase",
		FuzzyFinderInput:        false,
		ShowIcons:               true,
		IssuePrefix:             "issue",
		IssueBranchNameTemplate: "{prefix}-{number}-{title}",
		PRPrefix:                "pr",
		PRBranchNameTemplate:    "{prefix}-{number}-{title}",
		CustomCommands: map[string]*CustomCommand{
			"t": {
				Description: "Tmux",
				ShowHelp:    true,
				Tmux: &TmuxCommand{
					SessionName: "${REPO_NAME}_wt_$WORKTREE_NAME",
					Attach:      true,
					OnExists:    "switch",
					Windows: []TmuxWindow{
						{Name: "shell"},
					},
				},
			},
			"Z": {
				Description: "Zellij",
				ShowHelp:    false,
				Zellij: &TmuxCommand{
					SessionName: "${REPO_NAME}_wt_$WORKTREE_NAME",
					Attach:      true,
					OnExists:    "switch",
					Windows: []TmuxWindow{
						{Name: "shell"},
					},
				},
			},
		},
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

func parseTmuxCommand(data map[string]any) *TmuxCommand {
	tmux := &TmuxCommand{
		SessionName: "${REPO_NAME}_wt_$WORKTREE_NAME",
		Attach:      true,
		OnExists:    "switch",
	}

	if sessionName, ok := data["session_name"].(string); ok {
		sessionName = strings.TrimSpace(sessionName)
		if sessionName != "" {
			tmux.SessionName = sessionName
		}
	}

	if onExists, ok := data["on_exists"].(string); ok {
		onExists = strings.ToLower(strings.TrimSpace(onExists))
		switch onExists {
		case "switch", "attach", "kill", "new":
			tmux.OnExists = onExists
		}
	}

	tmux.Attach = coerceBool(data["attach"], true)

	var windows []TmuxWindow
	if rawWindows, ok := data["windows"].([]any); ok {
		windows = make([]TmuxWindow, 0, len(rawWindows))
		for _, item := range rawWindows {
			windowMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			window := TmuxWindow{}
			if name, ok := windowMap["name"].(string); ok {
				window.Name = strings.TrimSpace(name)
			}
			if cmd, ok := windowMap["command"].(string); ok {
				window.Command = strings.TrimSpace(cmd)
			}
			if cwd, ok := windowMap["cwd"].(string); ok {
				window.Cwd = strings.TrimSpace(cwd)
			}
			if window.Name == "" && window.Command == "" && window.Cwd == "" {
				continue
			}
			windows = append(windows, window)
		}
	}

	if len(windows) == 0 {
		windows = []TmuxWindow{{Name: "shell"}}
	}

	tmux.Windows = windows
	return tmux
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
		cmd.ShowOutput = coerceBool(cmdMap["show_output"], false)
		if tmuxRaw, ok := cmdMap["tmux"].(map[string]any); ok {
			cmd.Tmux = parseTmuxCommand(tmuxRaw)
		}
		if zellijRaw, ok := cmdMap["zellij"].(map[string]any); ok {
			cmd.Zellij = parseTmuxCommand(zellijRaw)
		}

		// Only add if command is not empty or tmux config is present
		if cmd.Command != "" || cmd.Tmux != nil || cmd.Zellij != nil {
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
	if pager, ok := data["pager"].(string); ok {
		pager = strings.TrimSpace(pager)
		if pager != "" {
			cfg.Pager = pager
		}
	}
	if editor, ok := data["editor"].(string); ok {
		editor = strings.TrimSpace(editor)
		if editor != "" {
			cfg.Editor = editor
		}
	}

	cfg.InitCommands = normalizeCommandList(data["init_commands"])
	cfg.TerminateCommands = normalizeCommandList(data["terminate_commands"])

	// Handle sort_mode with backwards compatibility for sort_by_active
	if sortMode, ok := data["sort_mode"].(string); ok {
		sortMode = strings.ToLower(strings.TrimSpace(sortMode))
		switch sortMode {
		case "path", "active", "switched":
			cfg.SortMode = sortMode
		}
	} else if _, hasOld := data["sort_by_active"]; hasOld {
		// Backwards compatibility: sort_by_active: true -> "active", false -> "path"
		if coerceBool(data["sort_by_active"], true) {
			cfg.SortMode = "active"
		} else {
			cfg.SortMode = "path"
		}
	}

	cfg.AutoFetchPRs = coerceBool(data["auto_fetch_prs"], false)
	cfg.SearchAutoSelect = coerceBool(data["search_auto_select"], false)
	cfg.FuzzyFinderInput = coerceBool(data["fuzzy_finder_input"], false)
	cfg.ShowIcons = coerceBool(data["show_icons"], cfg.ShowIcons)
	cfg.MaxUntrackedDiffs = coerceInt(data["max_untracked_diffs"], 10)
	cfg.MaxDiffChars = coerceInt(data["max_diff_chars"], 200000)
	if _, ok := data["delta_args"]; ok {
		cfg.DeltaArgs = normalizeArgsList(data["delta_args"])
		cfg.DeltaArgsSet = true
	}
	if deltaPath, ok := data["delta_path"].(string); ok {
		cfg.DeltaPath = strings.TrimSpace(deltaPath)
	}

	if trustMode, ok := data["trust_mode"].(string); ok {
		trustMode = strings.ToLower(strings.TrimSpace(trustMode))
		if trustMode == "tofu" || trustMode == "never" || trustMode == "always" {
			cfg.TrustMode = trustMode
		}
	}

	if themeName, ok := data["theme"].(string); ok {
		if normalized := NormalizeThemeName(themeName); normalized != "" {
			cfg.Theme = normalized
		}
	}

	if !cfg.DeltaArgsSet {
		cfg.DeltaArgs = DefaultDeltaArgsForTheme(cfg.Theme)
	}

	if branchNameScript, ok := data["branch_name_script"].(string); ok {
		branchNameScript = strings.TrimSpace(branchNameScript)
		if branchNameScript != "" {
			cfg.BranchNameScript = branchNameScript
		}
	}

	if issuePrefix, ok := data["issue_prefix"].(string); ok {
		issuePrefix = strings.TrimSpace(issuePrefix)
		if issuePrefix != "" {
			cfg.IssuePrefix = issuePrefix
		}
	}

	if issueBranchNameTemplate, ok := data["issue_branch_name_template"].(string); ok {
		issueBranchNameTemplate = strings.TrimSpace(issueBranchNameTemplate)
		if issueBranchNameTemplate != "" {
			cfg.IssueBranchNameTemplate = issueBranchNameTemplate
		}
	}

	if prPrefix, ok := data["pr_prefix"].(string); ok {
		prPrefix = strings.TrimSpace(prPrefix)
		if prPrefix != "" {
			cfg.PRPrefix = prPrefix
		}
	}

	if prBranchNameTemplate, ok := data["pr_branch_name_template"].(string); ok {
		prBranchNameTemplate = strings.TrimSpace(prBranchNameTemplate)
		if prBranchNameTemplate != "" {
			cfg.PRBranchNameTemplate = prBranchNameTemplate
		}
	}

	if mergeMethod, ok := data["merge_method"].(string); ok {
		mergeMethod = strings.ToLower(strings.TrimSpace(mergeMethod))
		if mergeMethod == "rebase" || mergeMethod == "merge" {
			cfg.MergeMethod = mergeMethod
		}
	}

	if cfg.MaxUntrackedDiffs < 0 {
		cfg.MaxUntrackedDiffs = 0
	}
	if cfg.MaxDiffChars < 0 {
		cfg.MaxDiffChars = 0
	}

	if _, ok := data["custom_commands"]; ok {
		customCommands := parseCustomCommands(data)
		for key, cmd := range customCommands {
			cfg.CustomCommands[key] = cmd
		}
	}

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

	var cfg *AppConfig

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

		cfg = parseConfig(yamlData)
		break
	}

	if cfg == nil {
		cfg = DefaultConfig()
	}

	if cfg.Theme == "" {
		detected, err := theme.DetectBackground(500 * time.Millisecond)
		if err == nil {
			cfg.Theme = detected
		} else {
			cfg.Theme = theme.DefaultDark()
		}

		if !cfg.DeltaArgsSet {
			cfg.DeltaArgs = DefaultDeltaArgsForTheme(cfg.Theme)
		}
	}

	return cfg, nil
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

// DefaultDeltaArgsForTheme returns the default delta arguments for a given theme.
func DefaultDeltaArgsForTheme(themeName string) []string {
	switch themeName {
	case theme.DraculaLightName:
		return []string{"--syntax-theme", "\"Monokai Extended Light\""}
	case theme.NarnaName:
		return []string{"--syntax-theme", "\"OneHalfDark\""}
	case theme.CleanLightName:
		return []string{"--syntax-theme", "GitHub"}
	case theme.SolarizedDarkName:
		return []string{"--syntax-theme", "\"Solarized (dark)\""}
	case theme.SolarizedLightName:
		return []string{"--syntax-theme", "\"Solarized (light)\""}
	case theme.GruvboxDarkName:
		return []string{"--syntax-theme", "\"Gruvbox Dark\""}
	case theme.GruvboxLightName:
		return []string{"--syntax-theme", "\"Gruvbox Light\""}
	case theme.NordName:
		return []string{"--syntax-theme", "\"Nord\""}
	case theme.MonokaiName:
		return []string{"--syntax-theme", "\"Monokai Extended\""}
	case theme.CatppuccinMochaName:
		return []string{"--syntax-theme", "\"Catppuccin Mocha\""}
	case theme.CatppuccinLatteName:
		return []string{"--syntax-theme", "\"Catppuccin Latte\""}
	case theme.RosePineDawnName:
		return []string{"--syntax-theme", "GitHub"}
	case theme.OneLightName:
		return []string{"--syntax-theme", "\"OneHalfLight\""}
	case theme.EverforestLightName:
		return []string{"--syntax-theme", "\"Gruvbox Light\""}
	default:
		return []string{"--syntax-theme", "Dracula"}
	}
}

// SyntaxThemeForUITheme returns the default delta syntax theme for a UI theme.
func SyntaxThemeForUITheme(themeName string) string {
	args := DefaultDeltaArgsForTheme(themeName)
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--syntax-theme" {
			return args[i+1]
		}
	}
	return ""
}

// NormalizeThemeName returns the canonical theme name if it is supported.
func NormalizeThemeName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	switch name {
	case theme.DraculaName,
		theme.DraculaLightName,
		theme.NarnaName,
		theme.CleanLightName,
		theme.CatppuccinLatteName,
		theme.RosePineDawnName,
		theme.OneLightName,
		theme.EverforestLightName,
		theme.SolarizedDarkName,
		theme.SolarizedLightName,
		theme.GruvboxDarkName,
		theme.GruvboxLightName,
		theme.NordName,
		theme.MonokaiName,
		theme.CatppuccinMochaName:
		return name
	default:
		return ""
	}
}
