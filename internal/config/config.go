// Package config loads application and repository configuration from YAML.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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

// CustomCreateMenu defines a custom entry in the worktree creation menu.
// The command should output a branch name that will be sanitized and used.
type CustomCreateMenu struct {
	Label           string // Display label in the menu
	Description     string // Help text shown next to label
	Command         string // Shell command that outputs branch name
	Interactive     bool   // Run interactively (TUI suspends, captures stdout via temp file)
	PostCommand     string // Command to run after worktree creation (optional)
	PostInteractive bool   // Run post-command interactively (default: false)
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
	IssueBranchNameTemplate string // Template for issue branch names with placeholders: {number}, {title} (default: "issue-{number}-{title}")
	PRBranchNameTemplate    string // Template for PR branch names with placeholders: {number}, {title} (default: "pr-{number}-{title}")
	CustomCreateMenus       []*CustomCreateMenu
	ConfigPath              string `yaml:"-"` // Path to the configuration file
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
		IssueBranchNameTemplate: "issue-{number}-{title}",
		PRBranchNameTemplate:    "pr-{number}-{title}",
		ShowIcons:               true,
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

func parseConfig(data map[string]any) *AppConfig {
	cfg := DefaultConfig()

	if worktreeDir, ok := data["worktree_dir"].(string); ok {
		expanded, err := expandPath(worktreeDir)
		if err == nil {
			cfg.WorktreeDir = expanded
		}
	}

	if debugLog, ok := data["debug_log"].(string); ok {
		expanded, err := expandPath(debugLog)
		if err == nil {
			cfg.DebugLog = expanded
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

	if issueBranchNameTemplate, ok := data["issue_branch_name_template"].(string); ok {
		issueBranchNameTemplate = strings.TrimSpace(issueBranchNameTemplate)
		if issueBranchNameTemplate != "" {
			cfg.IssueBranchNameTemplate = issueBranchNameTemplate
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

	if _, ok := data["custom_create_menus"]; ok {
		cfg.CustomCreateMenus = parseCustomCreateMenus(data)
	}

	return cfg
}

func parseCustomCommands(data map[string]any) map[string]*CustomCommand {
	raw, ok := data["custom_commands"].(map[string]any)
	if !ok {
		return make(map[string]*CustomCommand)
	}

	cmds := make(map[string]*CustomCommand)
	for key, val := range raw {
		cmdData, ok := val.(map[string]any)
		if !ok {
			continue
		}

		cmd := &CustomCommand{
			Command:     getString(cmdData, "command"),
			Description: getString(cmdData, "description"),
			ShowHelp:    coerceBool(cmdData["show_help"], false),
			Wait:        coerceBool(cmdData["wait"], false),
			ShowOutput:  coerceBool(cmdData["show_output"], false),
		}

		if tmux, ok := cmdData["tmux"].(map[string]any); ok {
			cmd.Tmux = parseTmuxCommand(tmux)
		}
		if zellij, ok := cmdData["zellij"].(map[string]any); ok {
			cmd.Zellij = parseTmuxCommand(zellij)
		}

		if cmd.Command != "" || cmd.Tmux != nil || cmd.Zellij != nil {
			cmds[key] = cmd
		}
	}
	return cmds
}

func parseTmuxCommand(data map[string]any) *TmuxCommand {
	cmd := &TmuxCommand{
		SessionName: getString(data, "session_name"),
		Attach:      coerceBool(data["attach"], true),
		OnExists:    strings.ToLower(getString(data, "on_exists")),
	}
	if cmd.OnExists == "" {
		cmd.OnExists = "switch"
	}

	if windows, ok := data["windows"].([]any); ok {
		for _, w := range windows {
			if wData, ok := w.(map[string]any); ok {
				cmd.Windows = append(cmd.Windows, TmuxWindow{
					Name:    getString(wData, "name"),
					Command: getString(wData, "command"),
					Cwd:     getString(wData, "cwd"),
				})
			}
		}
	}
	if len(cmd.Windows) == 0 {
		cmd.Windows = []TmuxWindow{
			{
				Name:    "shell",
				Command: "",
				Cwd:     "",
			},
		}
	}
	return cmd
}

func parseCustomCreateMenus(data map[string]any) []*CustomCreateMenu {
	raw, ok := data["custom_create_menus"].([]any)
	if !ok {
		return nil
	}

	menus := make([]*CustomCreateMenu, 0, len(raw))
	for _, val := range raw {
		mData, ok := val.(map[string]any)
		if !ok {
			continue
		}

		menu := &CustomCreateMenu{
			Label:           getString(mData, "label"),
			Description:     getString(mData, "description"),
			Command:         getString(mData, "command"),
			Interactive:     coerceBool(mData["interactive"], false),
			PostCommand:     getString(mData, "post_command"),
			PostInteractive: coerceBool(mData["post_interactive"], false),
		}
		if menu.Label != "" && menu.Command != "" {
			menus = append(menus, menu)
		}
	}
	return menus
}

func normalizeCommandList(val any) []string {
	if val == nil {
		return []string{}
	}
	if s, ok := val.(string); ok {
		s = strings.TrimSpace(s)
		if s == "" {
			return []string{}
		}
		return []string{s}
	}
	res := []string{}
	if l, ok := val.([]any); ok {
		for _, v := range l {
			if s, ok := v.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					res = append(res, s)
				}
			}
		}
	}
	return res
}

func normalizeArgsList(val any) []string {
	if s, ok := val.(string); ok {
		s = strings.TrimSpace(s)
		if s == "" {
			return []string{}
		}
		return strings.Fields(s)
	}
	res := []string{}
	if l, ok := val.([]any); ok {
		for _, v := range l {
			if s, ok := v.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					res = append(res, s)
				}
			}
		}
	}
	return res
}

// LoadConfig loads the application configuration from a file.
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
		cfg.ConfigPath = path
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

// SaveConfig writes the configuration back to the file.
// It tries to preserve existing fields by reading the file first.
func SaveConfig(cfg *AppConfig) error {
	path := cfg.ConfigPath
	if path == "" {
		configBase := filepath.Join(getConfigDir(), "lazyworktree")
		path = filepath.Join(configBase, "config.yaml")

		if err := os.MkdirAll(configBase, 0o700); err != nil { // #nosec G301
			return err
		}
	} else {
		// Ensure parent directory of the specific ConfigPath exists if we are saving to a known path
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil { // #nosec G301
			return err
		}
	}

	// #nosec G304
	data, err := os.ReadFile(path)
	var content string
	if err == nil {
		content = string(data)
	}

	// Use regex to replace or add theme: line
	re := regexp.MustCompile(`(?m)^theme:\s*.*$`)
	newThemeLine := fmt.Sprintf("theme: %s", cfg.Theme)

	var newData []byte
	if re.MatchString(content) {
		// Replace existing theme line
		newData = []byte(re.ReplaceAllString(content, newThemeLine))
	} else {
		// Add theme line
		if content != "" && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		newData = []byte(content + newThemeLine + "\n")
	}

	if err := os.WriteFile(path, newData, 0o600); err != nil { // #nosec G306
		return err
	}

	// Update ConfigPath if it was empty so subsequent saves use the same correctly
	if cfg.ConfigPath == "" {
		cfg.ConfigPath = path
	}

	return nil
}

// LoadRepoConfig loads the repository configuration from a .wt file.
func LoadRepoConfig(repoPath string) (*RepoConfig, string, error) {
	if repoPath == "" {
		return nil, "", fmt.Errorf("repo path cannot be empty")
	}

	path := filepath.Join(repoPath, ".wt")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, path, nil
	}

	// #nosec G304 -- path is constructed from safe repo path
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, path, err
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, path, err
	}

	cfg := &RepoConfig{
		Path:              path,
		InitCommands:      normalizeCommandList(raw["init_commands"]),
		TerminateCommands: normalizeCommandList(raw["terminate_commands"]),
	}

	return cfg, path, nil
}

// SyntaxThemeForUITheme returns the syntax theme name for a given TUI theme.
func SyntaxThemeForUITheme(themeName string) string {
	args := DefaultDeltaArgsForTheme(themeName)
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--syntax-theme" {
			return args[i+1]
		}
	}
	return "Dracula"
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
	case theme.CatppuccinLatteName:
		return []string{"--syntax-theme", "\"Catppuccin Latte\""}
	case theme.RosePineDawnName:
		return []string{"--syntax-theme", "GitHub"}
	case theme.OneLightName:
		return []string{"--syntax-theme", "\"OneHalfLight\""}
	case theme.EverforestLightName:
		return []string{"--syntax-theme", "\"Gruvbox Light\""}
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
	case theme.ModernName:
		return []string{"--syntax-theme", "Dracula"}
	case theme.TokyoNightName:
		return []string{"--syntax-theme", "Dracula"}
	case theme.OneDarkName:
		return []string{"--syntax-theme", "\"OneHalfDark\""}
	case theme.RosePineName:
		return []string{"--syntax-theme", "Dracula"}
	case theme.AyuMirageName:
		return []string{"--syntax-theme", "Dracula"}
	case theme.EverforestDarkName:
		return []string{"--syntax-theme", "Dracula"}
	default:
		return []string{"--syntax-theme", "Dracula"}
	}
}

// NormalizeThemeName returns the normalized theme name if valid, otherwise empty string.
func NormalizeThemeName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	switch name {
	case "dracula", "dracula-light", "narna", "clean-light", "catppuccin-latte", "rose-pine-dawn", "one-light", "everforest-light", "solarized-dark", "solarized-light", "gruvbox-dark", "gruvbox-light", "nord", "monokai", "catppuccin-mocha", "modern", "tokyo-night", "one-dark", "rose-pine", "ayu-mirage", "everforest-dark":
		return name
	}
	return ""
}

func getConfigDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return xdg
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}

func coerceBool(v any, def bool) bool {
	if v == nil {
		return def
	}
	if b, ok := v.(bool); ok {
		return b
	}
	if s, ok := v.(string); ok {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "true" || s == "1" || s == "yes" || s == "y" || s == "on" {
			return true
		}
		if s == "false" || s == "0" || s == "no" || s == "n" || s == "off" {
			return false
		}
	}
	if i, ok := v.(int); ok {
		return i != 0
	}
	return def
}

func coerceInt(v any, def int) int {
	if v == nil {
		return def
	}
	if i, ok := v.(int); ok {
		return i
	}
	if s, ok := v.(string); ok {
		s = strings.TrimSpace(s)
		i, err := strconv.Atoi(s)
		if err == nil {
			return i
		}
	}
	return def
}

func getString(data map[string]any, key string) string {
	if v, ok := data[key]; ok && v != nil {
		return strings.TrimSpace(fmt.Sprint(v))
	}
	return ""
}
