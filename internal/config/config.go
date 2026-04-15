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

// Note type constants for WorktreeNoteType configuration.
const (
	NoteTypeSplitted = "splitted"
	NoteTypeOneJSON  = "onejson"
)

// CommitConfig defines settings for commit operations.
type CommitConfig struct {
	AutoGenerateCommand string `yaml:"auto_generate_command"`
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
	CustomCommands          CustomCommandsConfig
	Keybindings             KeybindingsConfig
	BranchNameScript        string // Script to generate branch name suggestions from diff
	WorktreeNoteScript      string // Script to generate worktree notes from PR/issue content
	WorktreeNotesPath       string // Optional path to a single shared JSON file for worktree notes
	WorktreeNoteType        string // Note storage type: "onejson" (default) or "splitted"
	Theme                   string // Theme name: see AvailableThemes in internal/theme
	MergeMethod             string // Merge method for absorb: "rebase" or "merge" (default: "rebase")
	FuzzyFinderInput        bool   // Enable fuzzy finder for input suggestions (default: false)
	IconSet                 string // Icon set: "nerd-font-v3", "text" (default: "nerd-font-v3"). Legacy "emoji" and "none" map to "text".
	IssueBranchNameTemplate string // Template for issue branch names with placeholders: {number}, {title} (default: "issue-{number}-{title}")
	PRBranchNameTemplate    string // Template for PR branch names with placeholders: {number}, {title}, {generated}, {pr_author} (default: "pr-{number}-{title}")
	SessionPrefix           string // Prefix for tmux/zellij session names (default: "wt-")
	Layout                  string // Pane arrangement: "default" or "top" (default: "default")
	PruneStaleBranches      bool   // Include merged branches without worktrees in prune (default: false)
	PaletteMRU              bool   // Enable MRU sorting for command palette (default: false)
	PaletteMRULimit         int    // Number of MRU items to show (default: 5)
	AgentSessionClaudeRoot  string // Custom root for Claude transcript discovery (default: ~/.claude/projects)
	AgentSessionPiRoot      string // Custom root for pi transcript discovery (default: ~/.pi/agent/sessions)
	CustomCreateMenus       []*CustomCreateMenu
	CustomThemes            map[string]*CustomTheme // User-defined custom themes
	LayoutSizes             *LayoutSizes            // Configurable pane size weights (nil = use defaults)
	ConfigPath              string                  `yaml:"-"` // Path to the configuration file
	DeprecationWarnings     []string                `yaml:"-"` // Warnings about deprecated config keys detected at load time
	Commit                  CommitConfig            `yaml:"commit"`
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
		Keybindings:             make(KeybindingsConfig),
		CustomCommands: CustomCommandsConfig{
			PaneUniversal: {
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
		},
	}
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
	if autoGenerateCommand, ok := data["commit.auto_generate_command"].(string); ok {
		autoGenerateCommand = strings.TrimSpace(autoGenerateCommand)
		if autoGenerateCommand != "" {
			cfg.Commit.AutoGenerateCommand = autoGenerateCommand
		}
	} else if commitData, ok := data["commit"].(map[string]any); ok {
		if autoGenerateCommand, ok := commitData["auto_generate_command"].(string); ok {
			autoGenerateCommand = strings.TrimSpace(autoGenerateCommand)
			if autoGenerateCommand != "" {
				cfg.Commit.AutoGenerateCommand = autoGenerateCommand
			}
		}
	}

	if agentData, ok := data["agent_sessions"].(map[string]any); ok {
		if claudeRoot, ok := agentData["claude_root"].(string); ok {
			claudeRoot = strings.TrimSpace(claudeRoot)
			if claudeRoot != "" {
				expanded, err := utils.ExpandPath(claudeRoot)
				if err == nil {
					cfg.AgentSessionClaudeRoot = expanded
				}
			}
		}
		if piRoot, ok := agentData["pi_root"].(string); ok {
			piRoot = strings.TrimSpace(piRoot)
			if piRoot != "" {
				expanded, err := utils.ExpandPath(piRoot)
				if err == nil {
					cfg.AgentSessionPiRoot = expanded
				}
			}
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
	cfg.PruneStaleBranches = coerceBool(data["prune_stale_branches"], false)

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

	if worktreeNoteScript, ok := data["worktree_note_script"].(string); ok {
		worktreeNoteScript = strings.TrimSpace(worktreeNoteScript)
		if worktreeNoteScript != "" {
			cfg.WorktreeNoteScript = worktreeNoteScript
		}
	}
	if noteType, ok := data["worktree_note_type"].(string); ok {
		noteType = strings.TrimSpace(noteType)
		if noteType != "" && noteType != NoteTypeOneJSON && noteType != NoteTypeSplitted {
			return nil, fmt.Errorf("invalid worktree_note_type %q: must be %q or %q", noteType, NoteTypeOneJSON, NoteTypeSplitted)
		}
		cfg.WorktreeNoteType = noteType
	}
	if worktreeNotesPath, ok := data["worktree_notes_path"].(string); ok {
		worktreeNotesPath = strings.TrimSpace(worktreeNotesPath)
		if worktreeNotesPath != "" {
			if cfg.WorktreeNoteType == NoteTypeSplitted {
				// Splitted mode: path contains template variables, only expand ~
				if strings.HasPrefix(worktreeNotesPath, "~/") {
					home, herr := os.UserHomeDir()
					if herr == nil {
						worktreeNotesPath = filepath.Join(home, worktreeNotesPath[2:])
					}
				}
				cfg.WorktreeNotesPath = worktreeNotesPath
			} else {
				expanded, err := utils.ExpandPath(worktreeNotesPath)
				if err == nil {
					cfg.WorktreeNotesPath = expanded
				}
			}
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
		parsed, deprecations := parseCustomCommands(data)
		cfg.DeprecationWarnings = append(cfg.DeprecationWarnings, deprecations...)
		for pane, cmds := range parsed {
			if cfg.CustomCommands[pane] == nil {
				cfg.CustomCommands[pane] = make(map[string]*CustomCommand)
			}
			for key, cmd := range cmds {
				cfg.CustomCommands[pane][key] = cmd
			}
		}
	}

	if _, ok := data["keybindings"]; ok {
		cfg.Keybindings = parseKeybindings(data)
	}

	if _, ok := data["custom_create_menus"]; ok {
		cfg.CustomCreateMenus = parseCustomCreateMenus(data)
	}

	if _, ok := data["custom_themes"]; ok {
		cfg.CustomThemes = parseCustomThemes(data)
	}

	if _, ok := data["layout_sizes"]; ok {
		cfg.LayoutSizes = parseLayoutSizes(data)
	}

	return cfg, nil
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
	if overrideCfg.Commit.AutoGenerateCommand != "" {
		cfg.Commit.AutoGenerateCommand = overrideCfg.Commit.AutoGenerateCommand
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
	if overrideCfg.WorktreeNoteScript != "" {
		cfg.WorktreeNoteScript = overrideCfg.WorktreeNoteScript
	}
	if overrideCfg.WorktreeNoteType != "" {
		cfg.WorktreeNoteType = overrideCfg.WorktreeNoteType
	}
	if overrideCfg.WorktreeNotesPath != "" {
		cfg.WorktreeNotesPath = overrideCfg.WorktreeNotesPath
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
	if overrideCfg.AgentSessionClaudeRoot != "" {
		cfg.AgentSessionClaudeRoot = overrideCfg.AgentSessionClaudeRoot
	}
	if overrideCfg.AgentSessionPiRoot != "" {
		cfg.AgentSessionPiRoot = overrideCfg.AgentSessionPiRoot
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
	if _, ok := overrideData["prune_stale_branches"]; ok {
		cfg.PruneStaleBranches = overrideCfg.PruneStaleBranches
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

	if _, ok := overrideData["layout_sizes"]; ok {
		if overrideCfg.LayoutSizes != nil {
			if cfg.LayoutSizes == nil {
				cfg.LayoutSizes = &LayoutSizes{}
			}
			if overrideCfg.LayoutSizes.Worktrees > 0 {
				cfg.LayoutSizes.Worktrees = overrideCfg.LayoutSizes.Worktrees
			}
			if overrideCfg.LayoutSizes.Info > 0 {
				cfg.LayoutSizes.Info = overrideCfg.LayoutSizes.Info
			}
			if overrideCfg.LayoutSizes.GitStatus > 0 {
				cfg.LayoutSizes.GitStatus = overrideCfg.LayoutSizes.GitStatus
			}
			if overrideCfg.LayoutSizes.Commit > 0 {
				cfg.LayoutSizes.Commit = overrideCfg.LayoutSizes.Commit
			}
			if overrideCfg.LayoutSizes.Notes > 0 {
				cfg.LayoutSizes.Notes = overrideCfg.LayoutSizes.Notes
			}
		}
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

	if err := os.WriteFile(path, newData, utils.DefaultFilePerms); err != nil {
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
