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
	"github.com/chmouel/lazyworktree/internal/utils"
	"gopkg.in/yaml.v3"
)

// CustomCommand represents a user-defined command binding.
type CustomCommand struct {
	Command     string
	Description string
	ShowHelp    bool
	Wait        bool
	ShowOutput  bool
	NewTab      bool // Launch command in a new terminal tab (Kitty, etc.)
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

// CustomTheme represents a user-defined theme that can inherit from built-in or other custom themes.
type CustomTheme struct {
	Base      string // Optional base theme name (built-in or custom)
	Accent    string
	AccentFg  string
	AccentDim string
	Border    string
	BorderDim string
	MutedFg   string
	TextFg    string
	SuccessFg string
	WarnFg    string
	ErrorFg   string
	Cyan      string
}

// AppConfig defines the global lazyworktree configuration options.
type AppConfig struct {
	WorktreeDir             string
	InitCommands            []string
	TerminateCommands       []string
	SortMode                string // Sort mode: "path", "active" (commit date), "switched" (last accessed)
	AutoFetchPRs            bool
	DisablePR               bool // Disable all PR/MR fetching and display
	SearchAutoSelect        bool // Start with filter focused and select first match on Enter.
	MaxUntrackedDiffs       int
	MaxDiffChars            int
	MaxNameLength           int // Maximum length for worktree names in table display (0 disables truncation)
	GitPagerArgs            []string
	GitPagerArgsSet         bool `yaml:"-"`
	GitPager                string
	GitPagerInteractive     bool // Interactive tools need terminal control, skip piping to less
	GitPagerCommandMode     bool // Command-mode tools run their own git commands (e.g. lumen diff)
	TrustMode               string
	DebugLog                string
	Pager                   string
	CIScriptPager           string // Pager for CI check logs, implicitly interactive
	Editor                  string
	AutoRefresh             bool
	CIAutoRefresh           bool // Periodically refresh CI status (GitHub only, uses API rate limits)
	RefreshIntervalSeconds  int
	CustomCommands          map[string]*CustomCommand
	BranchNameScript        string // Script to generate branch name suggestions from diff
	Theme                   string // Theme name: see AvailableThemes in internal/theme
	MergeMethod             string // Merge method for absorb: "rebase" or "merge" (default: "rebase")
	FuzzyFinderInput        bool   // Enable fuzzy finder for input suggestions (default: false)
	IconSet                 string // Icon set: "nerd-font-v3", "text" (default: "nerd-font-v3"). Legacy "emoji" and "none" map to "text".
	IssueBranchNameTemplate string // Template for issue branch names with placeholders: {number}, {title} (default: "issue-{number}-{title}")
	PRBranchNameTemplate    string // Template for PR branch names with placeholders: {number}, {title}, {generated}, {pr_author} (default: "pr-{number}-{title}")
	SessionPrefix           string // Prefix for tmux/zellij session names (default: "wt-")
	Layout                  string // Pane arrangement: "default" or "top" (default: "default")
	PaletteMRU              bool   // Enable MRU sorting for command palette (default: false)
	PaletteMRULimit         int    // Number of MRU items to show (default: 5)
	CustomCreateMenus       []*CustomCreateMenu
	CustomThemes            map[string]*CustomTheme // User-defined custom themes
	ConfigPath              string                  `yaml:"-"` // Path to the configuration file
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
		AutoRefresh:             true,
		RefreshIntervalSeconds:  10,
		SearchAutoSelect:        false,
		MaxUntrackedDiffs:       10,
		MaxDiffChars:            200000,
		MaxNameLength:           95,
		GitPagerArgs:            DefaultDeltaArgsForTheme(theme.DraculaName),
		GitPager:                "delta",
		GitPagerInteractive:     false,
		TrustMode:               "tofu",
		Theme:                   "",
		MergeMethod:             "rebase",
		IssueBranchNameTemplate: "issue-{number}-{title}",
		PRBranchNameTemplate:    "pr-{number}-{title}",
		SessionPrefix:           "wt-",
		Layout:                  "default",
		PaletteMRU:              true,
		PaletteMRULimit:         5,
		IconSet:                 "nerd-font-v3",
		CustomThemes:            make(map[string]*CustomTheme),
		CustomCommands: map[string]*CustomCommand{
			"t": {
				Description: "Tmux",
				ShowHelp:    true,
				Tmux: &TmuxCommand{
					SessionName: "wt:$WORKTREE_NAME",
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
					SessionName: "wt:$WORKTREE_NAME",
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

var iconSetOptions = []string{"nerd-font-v3", "text"}

// IconsEnabled reports whether icon rendering should be enabled for the current icon set.
func (c *AppConfig) IconsEnabled() bool {
	iconSet := strings.ToLower(strings.TrimSpace(c.IconSet))
	return iconSet != ""
}

func iconSetOptionsString() string {
	return strings.Join(iconSetOptions, ", ")
}

func parseConfig(data map[string]any) (*AppConfig, error) {
	cfg := DefaultConfig()

	if worktreeDir, ok := data["worktree_dir"].(string); ok {
		expanded, err := utils.ExpandPath(worktreeDir)
		if err == nil {
			cfg.WorktreeDir = expanded
		}
	}

	if debugLog, ok := data["debug_log"].(string); ok {
		expanded, err := utils.ExpandPath(debugLog)
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
	if ciScriptPager, ok := data["ci_script_pager"].(string); ok {
		ciScriptPager = strings.TrimSpace(ciScriptPager)
		if ciScriptPager != "" {
			cfg.CIScriptPager = ciScriptPager
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
	cfg.DisablePR = coerceBool(data["disable_pr"], false)
	cfg.AutoRefresh = coerceBool(data["auto_refresh"], cfg.AutoRefresh)
	cfg.CIAutoRefresh = coerceBool(data["ci_auto_refresh"], false)
	cfg.RefreshIntervalSeconds = coerceInt(data["refresh_interval"], cfg.RefreshIntervalSeconds)
	cfg.SearchAutoSelect = coerceBool(data["search_auto_select"], false)
	cfg.FuzzyFinderInput = coerceBool(data["fuzzy_finder_input"], false)

	if iconSet, ok := data["icon_set"].(string); ok {
		iconSet = strings.ToLower(strings.TrimSpace(iconSet))
		if iconSet == "" {
			cfg.IconSet = "text"
		} else {
			switch iconSet {
			case "emoji", "none":
				cfg.IconSet = "text"
			case "nerd-font-v3", "text":
				cfg.IconSet = iconSet
			default:
				return nil, fmt.Errorf("invalid icon_set %q (available: %s)", iconSet, iconSetOptionsString())
			}
		}
	}

	cfg.MaxUntrackedDiffs = coerceInt(data["max_untracked_diffs"], 10)
	cfg.MaxDiffChars = coerceInt(data["max_diff_chars"], 200000)
	cfg.MaxNameLength = coerceInt(data["max_name_length"], 95)
	// Diff formatter/pager configuration (new keys: git_pager, git_pager_args)
	if _, ok := data["git_pager_args"]; ok {
		cfg.GitPagerArgs = normalizeArgsList(data["git_pager_args"])
		cfg.GitPagerArgsSet = true
	} else if _, ok := data["delta_args"]; ok {
		// Backwards compatibility
		cfg.GitPagerArgs = normalizeArgsList(data["delta_args"])
		cfg.GitPagerArgsSet = true
	}
	if gitPager, ok := data["git_pager"].(string); ok {
		cfg.GitPager = strings.TrimSpace(gitPager)
	} else if deltaPath, ok := data["delta_path"].(string); ok {
		// Backwards compatibility
		cfg.GitPager = strings.TrimSpace(deltaPath)
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

	if !cfg.GitPagerArgsSet {
		if filepath.Base(cfg.GitPager) == "delta" {
			cfg.GitPagerArgs = DefaultDeltaArgsForTheme(cfg.Theme)
		} else {
			// Clear delta args inherited from DefaultConfig when using non-delta pager
			cfg.GitPagerArgs = nil
		}
	}

	cfg.GitPagerInteractive = coerceBool(data["git_pager_interactive"], false)
	cfg.GitPagerCommandMode = coerceBool(data["git_pager_command_mode"], false)

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

	if sessionPrefix, ok := data["session_prefix"].(string); ok {
		sessionPrefix = strings.TrimSpace(sessionPrefix)
		if sessionPrefix != "" {
			cfg.SessionPrefix = sessionPrefix
		}
	}

	cfg.PaletteMRU = coerceBool(data["palette_mru"], true)
	cfg.PaletteMRULimit = coerceInt(data["palette_mru_limit"], 5)
	if cfg.PaletteMRULimit <= 0 {
		cfg.PaletteMRULimit = 5
	}

	if layout, ok := data["layout"].(string); ok {
		layout = strings.ToLower(strings.TrimSpace(layout))
		if layout == "default" || layout == "top" {
			cfg.Layout = layout
		}
	}

	if cfg.MaxUntrackedDiffs < 0 {
		cfg.MaxUntrackedDiffs = 0
	}
	if cfg.MaxDiffChars < 0 {
		cfg.MaxDiffChars = 0
	}
	if cfg.MaxNameLength < 0 {
		cfg.MaxNameLength = 0
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

	if _, ok := data["custom_themes"]; ok {
		cfg.CustomThemes = parseCustomThemes(data)
	}

	return cfg, nil
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
			NewTab:      coerceBool(cmdData["new_tab"], false),
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

func parseCustomThemes(data map[string]any) map[string]*CustomTheme {
	raw, ok := data["custom_themes"].(map[string]any)
	if !ok {
		return make(map[string]*CustomTheme)
	}

	themes := make(map[string]*CustomTheme)
	builtInThemes := theme.AvailableThemes()
	builtInMap := make(map[string]bool)
	for _, name := range builtInThemes {
		builtInMap[strings.ToLower(name)] = true
	}

	for name, val := range raw {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			continue
		}

		// Check for conflicts with built-in themes
		if builtInMap[name] {
			continue
		}

		themeData, ok := val.(map[string]any)
		if !ok {
			continue
		}

		customTheme := &CustomTheme{
			Base:      strings.TrimSpace(getString(themeData, "base")),
			Accent:    strings.TrimSpace(getString(themeData, "accent")),
			AccentFg:  strings.TrimSpace(getString(themeData, "accent_fg")),
			AccentDim: strings.TrimSpace(getString(themeData, "accent_dim")),
			Border:    strings.TrimSpace(getString(themeData, "border")),
			BorderDim: strings.TrimSpace(getString(themeData, "border_dim")),
			MutedFg:   strings.TrimSpace(getString(themeData, "muted_fg")),
			TextFg:    strings.TrimSpace(getString(themeData, "text_fg")),
			SuccessFg: strings.TrimSpace(getString(themeData, "success_fg")),
			WarnFg:    strings.TrimSpace(getString(themeData, "warn_fg")),
			ErrorFg:   strings.TrimSpace(getString(themeData, "error_fg")),
			Cyan:      strings.TrimSpace(getString(themeData, "cyan")),
		}

		// Validate: if no base, all fields must be present
		if customTheme.Base == "" {
			if err := validateCompleteTheme(customTheme); err != nil {
				continue
			}
		}

		// Validate all color values
		if !validateThemeColors(customTheme) {
			continue
		}

		themes[name] = customTheme
	}

	// Validate inheritance chains (no circular dependencies, base themes exist)
	validatedThemes := make(map[string]*CustomTheme)
	for name, customTheme := range themes {
		if validateThemeInheritance(name, customTheme, themes, builtInMap, make(map[string]bool)) {
			validatedThemes[name] = customTheme
		}
	}

	return validatedThemes
}

// validateCompleteTheme validates that all required fields are present when base is not specified.
func validateCompleteTheme(custom *CustomTheme) error {
	var missing []string

	if custom.Accent == "" {
		missing = append(missing, "accent")
	}
	if custom.AccentFg == "" {
		missing = append(missing, "accent_fg")
	}
	if custom.AccentDim == "" {
		missing = append(missing, "accent_dim")
	}
	if custom.Border == "" {
		missing = append(missing, "border")
	}
	if custom.BorderDim == "" {
		missing = append(missing, "border_dim")
	}
	if custom.MutedFg == "" {
		missing = append(missing, "muted_fg")
	}
	if custom.TextFg == "" {
		missing = append(missing, "text_fg")
	}
	if custom.SuccessFg == "" {
		missing = append(missing, "success_fg")
	}
	if custom.WarnFg == "" {
		missing = append(missing, "warn_fg")
	}
	if custom.ErrorFg == "" {
		missing = append(missing, "error_fg")
	}
	if custom.Cyan == "" {
		missing = append(missing, "cyan")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required fields: %s", strings.Join(missing, ", "))
	}

	return nil
}

// validateThemeColors validates all color hex values in a custom theme.
func validateThemeColors(custom *CustomTheme) bool {
	colors := []string{
		custom.Accent,
		custom.AccentFg,
		custom.AccentDim,
		custom.Border,
		custom.BorderDim,
		custom.MutedFg,
		custom.TextFg,
		custom.SuccessFg,
		custom.WarnFg,
		custom.ErrorFg,
		custom.Cyan,
	}

	for _, color := range colors {
		if color != "" && !validateColorHex(color) {
			return false
		}
	}

	return true
}

// validateColorHex validates hex color format (#RRGGBB or #RGB).
func validateColorHex(color string) bool {
	if color == "" {
		return false
	}

	if color[0] != '#' {
		return false
	}

	hex := color[1:]
	if len(hex) != 3 && len(hex) != 6 {
		return false
	}

	matched, _ := regexp.MatchString("^[0-9A-Fa-f]+$", hex)
	return matched
}

// validateThemeInheritance validates inheritance chains, checking for circular dependencies and ensuring base themes exist.
func validateThemeInheritance(name string, custom *CustomTheme, themes map[string]*CustomTheme, builtInMap, visited map[string]bool) bool {
	if custom.Base == "" {
		return true
	}

	baseName := strings.ToLower(custom.Base)

	// Check for circular dependency
	if visited[baseName] {
		return false
	}

	// Check if base exists
	if builtInMap[baseName] {
		return true
	}

	baseTheme, exists := themes[baseName]
	if !exists {
		return false
	}

	// Recursively validate base theme
	visited[name] = true
	return validateThemeInheritance(baseName, baseTheme, themes, builtInMap, visited)
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

// loadYAMLFile loads YAML config file and returns parsed data.
func loadYAMLFile(configPath string) map[string]any {
	configBase := filepath.Join(getConfigDir(), "lazyworktree")
	configBase = filepath.Clean(configBase)

	var paths []string

	if configPath != "" {
		expanded, err := utils.ExpandPath(configPath)
		if err != nil {
			return nil
		}
		absPath, err := filepath.Abs(expanded)
		if err != nil {
			return nil
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

		// #nosec G304 -- path expanded from user config location or CLI argument
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var yamlData map[string]any
		if err := yaml.Unmarshal(data, &yamlData); err != nil {
			return nil
		}

		return yamlData
	}

	return nil
}

// ApplyCLIOverrides applies CLI config overrides to the configuration.
func (cfg *AppConfig) ApplyCLIOverrides(overrides []string) error {
	if len(overrides) == 0 {
		return nil
	}

	// Parse CLI overrides
	overrideData, err := parseCLIConfigOverrides(overrides)
	if err != nil {
		return err
	}
	overrideCfg, err := parseConfig(overrideData)
	if err != nil {
		return err
	}

	// Apply each non-zero/non-empty field from overrideCfg to cfg
	if overrideCfg.WorktreeDir != "" {
		cfg.WorktreeDir = overrideCfg.WorktreeDir
	}
	if overrideCfg.SortMode != "" {
		cfg.SortMode = overrideCfg.SortMode
	}
	if overrideCfg.Theme != "" {
		cfg.Theme = overrideCfg.Theme
	}
	if overrideCfg.GitPager != "" {
		cfg.GitPager = overrideCfg.GitPager
	}
	if overrideCfg.Pager != "" {
		cfg.Pager = overrideCfg.Pager
	}
	if overrideCfg.CIScriptPager != "" {
		cfg.CIScriptPager = overrideCfg.CIScriptPager
	}
	if overrideCfg.Editor != "" {
		cfg.Editor = overrideCfg.Editor
	}
	if overrideCfg.DebugLog != "" {
		cfg.DebugLog = overrideCfg.DebugLog
	}
	if overrideCfg.TrustMode != "" {
		cfg.TrustMode = overrideCfg.TrustMode
	}
	if overrideCfg.MergeMethod != "" {
		cfg.MergeMethod = overrideCfg.MergeMethod
	}
	if overrideCfg.BranchNameScript != "" {
		cfg.BranchNameScript = overrideCfg.BranchNameScript
	}
	if overrideCfg.IssueBranchNameTemplate != "" {
		cfg.IssueBranchNameTemplate = overrideCfg.IssueBranchNameTemplate
	}
	if overrideCfg.PRBranchNameTemplate != "" {
		cfg.PRBranchNameTemplate = overrideCfg.PRBranchNameTemplate
	}
	if overrideCfg.SessionPrefix != "" {
		cfg.SessionPrefix = overrideCfg.SessionPrefix
	}

	// Arrays - check if they exist in override data
	if _, ok := overrideData["init_commands"]; ok {
		cfg.InitCommands = overrideCfg.InitCommands
	}
	if _, ok := overrideData["terminate_commands"]; ok {
		cfg.TerminateCommands = overrideCfg.TerminateCommands
	}
	if _, ok := overrideData["git_pager_args"]; ok {
		cfg.GitPagerArgs = overrideCfg.GitPagerArgs
		cfg.GitPagerArgsSet = true
	}

	// For booleans and integers, check if they were explicitly set in overrideData
	if _, ok := overrideData["auto_fetch_prs"]; ok {
		cfg.AutoFetchPRs = overrideCfg.AutoFetchPRs
	}
	if _, ok := overrideData["disable_pr"]; ok {
		cfg.DisablePR = overrideCfg.DisablePR
	}
	if _, ok := overrideData["search_auto_select"]; ok {
		cfg.SearchAutoSelect = overrideCfg.SearchAutoSelect
	}
	if _, ok := overrideData["auto_refresh"]; ok {
		cfg.AutoRefresh = overrideCfg.AutoRefresh
	}
	if _, ok := overrideData["ci_auto_refresh"]; ok {
		cfg.CIAutoRefresh = overrideCfg.CIAutoRefresh
	}
	if _, ok := overrideData["git_pager_interactive"]; ok {
		cfg.GitPagerInteractive = overrideCfg.GitPagerInteractive
	}
	if _, ok := overrideData["git_pager_command_mode"]; ok {
		cfg.GitPagerCommandMode = overrideCfg.GitPagerCommandMode
	}
	if _, ok := overrideData["fuzzy_finder_input"]; ok {
		cfg.FuzzyFinderInput = overrideCfg.FuzzyFinderInput
	}
	if _, ok := overrideData["icon_set"]; ok {
		cfg.IconSet = overrideCfg.IconSet
	}
	if _, ok := overrideData["palette_mru"]; ok {
		cfg.PaletteMRU = overrideCfg.PaletteMRU
	}

	if _, ok := overrideData["max_untracked_diffs"]; ok {
		cfg.MaxUntrackedDiffs = overrideCfg.MaxUntrackedDiffs
	}
	if _, ok := overrideData["max_diff_chars"]; ok {
		cfg.MaxDiffChars = overrideCfg.MaxDiffChars
	}
	if _, ok := overrideData["refresh_interval_seconds"]; ok {
		cfg.RefreshIntervalSeconds = overrideCfg.RefreshIntervalSeconds
	}
	if _, ok := overrideData["palette_mru_limit"]; ok {
		cfg.PaletteMRULimit = overrideCfg.PaletteMRULimit
	}

	if _, ok := overrideData["layout"]; ok {
		cfg.Layout = overrideCfg.Layout
	}

	return nil
}

// mergeMaps merges src map into dst map, with src values taking precedence.
func mergeMaps(dst, src map[string]any) {
	for k, v := range src {
		dst[k] = v
	}
}

// LoadConfig loads the application configuration from a file.
func LoadConfig(configPath string) (*AppConfig, error) {
	// Collect all config data maps, then merge and parse once
	mergedData := make(map[string]any)

	// 1. Load YAML config
	var actualConfigPath string
	yamlData := loadYAMLFile(configPath)
	if yamlData != nil {
		mergeMaps(mergedData, yamlData)

		// Determine actual config path
		if configPath != "" {
			expanded, _ := utils.ExpandPath(configPath)
			absPath, _ := filepath.Abs(expanded)
			actualConfigPath = absPath
		} else {
			// Set to default location if it exists
			configBase := filepath.Join(getConfigDir(), "lazyworktree")
			for _, name := range []string{"config.yaml", "config.yml"} {
				path := filepath.Join(configBase, name)
				if _, err := os.Stat(path); err == nil {
					actualConfigPath = path
					break
				}
			}
		}
	}

	// 2. Load and merge git global config (overrides YAML)
	gitGlobalData, err := loadGitConfig(true, "")
	if err == nil && len(gitGlobalData) > 0 {
		mergeMaps(mergedData, gitGlobalData)
	}

	// 3. Determine repo path from merged data so far
	var worktreeDir string
	if wd, ok := mergedData["worktree_dir"].(string); ok {
		worktreeDir = wd
	}

	// 4. Load and merge git local config (overrides git global)
	repoPath := determineRepoPath(worktreeDir)
	if repoPath != "" {
		gitLocalData, err := loadGitConfig(false, repoPath)
		if err == nil && len(gitLocalData) > 0 {
			mergeMaps(mergedData, gitLocalData)
		}
	}

	// 5. Parse the merged data into AppConfig
	cfg, err := parseConfig(mergedData)
	if err != nil {
		return nil, err
	}
	cfg.ConfigPath = actualConfigPath

	// 6. Theme detection (if theme not set from any config source)
	if cfg.Theme == "" {
		detected, err := theme.DetectBackground(500 * time.Millisecond)
		if err == nil {
			cfg.Theme = detected
		} else {
			cfg.Theme = theme.DefaultDark()
		}

		if !cfg.GitPagerArgsSet {
			if filepath.Base(cfg.GitPager) == "delta" {
				cfg.GitPagerArgs = DefaultDeltaArgsForTheme(cfg.Theme)
			} else {
				cfg.GitPagerArgs = nil
			}
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
	case theme.KanagawaName:
		return []string{"--syntax-theme", "Dracula"}
	default:
		return []string{"--syntax-theme", "Dracula"}
	}
}

// NormalizeThemeName returns the normalized theme name if valid, otherwise empty string.
func NormalizeThemeName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	switch name {
	case "dracula", "dracula-light", "narna", "clean-light", "catppuccin-latte", "rose-pine-dawn", "one-light", "everforest-light", "solarized-dark", "solarized-light", "gruvbox-dark", "gruvbox-light", "nord", "monokai", "catppuccin-mocha", "modern", "tokyo-night", "one-dark", "rose-pine", "ayu-mirage", "everforest-dark", "kanagawa":
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

// ToThemeData converts CustomTheme to theme.CustomThemeData to avoid circular dependencies.
func (ct *CustomTheme) ToThemeData() *theme.CustomThemeData {
	if ct == nil {
		return nil
	}
	return &theme.CustomThemeData{
		Base:      ct.Base,
		Accent:    ct.Accent,
		AccentFg:  ct.AccentFg,
		AccentDim: ct.AccentDim,
		Border:    ct.Border,
		BorderDim: ct.BorderDim,
		MutedFg:   ct.MutedFg,
		TextFg:    ct.TextFg,
		SuccessFg: ct.SuccessFg,
		WarnFg:    ct.WarnFg,
		ErrorFg:   ct.ErrorFg,
		Cyan:      ct.Cyan,
	}
}

// CustomThemesToThemeDataMap converts a map of CustomTheme to theme.CustomThemeData.
func CustomThemesToThemeDataMap(customThemes map[string]*CustomTheme) map[string]*theme.CustomThemeData {
	if customThemes == nil {
		return nil
	}
	result := make(map[string]*theme.CustomThemeData)
	for name, ct := range customThemes {
		result[name] = ct.ToThemeData()
	}
	return result
}
