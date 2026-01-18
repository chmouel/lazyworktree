// Package main provides CLI parsing functionality for lazyworktree.
package main

import (
	"strings"

	"github.com/alecthomas/kong"
	"github.com/chmouel/lazyworktree/internal/theme"
	kongcompletion "github.com/jotaen/kong-completion"
	"github.com/posener/complete"
)

// CLI represents the main command-line interface structure.
type CLI struct {
	WorktreeDir      string   `help:"Override the default worktree root directory" short:"w"`
	DebugLog         string   `help:"Path to debug log file"`
	OutputSelection  string   `help:"Write selected worktree path to a file"`
	Theme            string   `help:"Override the UI theme" short:"t" predictor:"theme"`
	SearchAutoSelect bool     `help:"Start with filter focused"`
	Version          bool     `help:"Print version information" short:"v"`
	ShowSyntaxThemes bool     `help:"List available delta syntax themes"`
	ConfigFile       string   `help:"Path to configuration file"`
	Config           []string `help:"Override config values (repeatable): --config=lw.key=value" short:"C" predictor:"config" completion-shell-default:"false"`

	WtCreate   *WtCreateCmd              `cmd:"" help:"Create a new worktree"`
	WtDelete   *WtDeleteCmd              `cmd:"" help:"Delete a worktree"`
	Completion kongcompletion.Completion `cmd:"" help:"Generate or run shell completions"`
}

// WtCreateCmd represents the wt-create subcommand.
type WtCreateCmd struct {
	FromBranch string `help:"Create worktree from branch" xor:"source"`
	FromPR     int    `help:"Create worktree from PR number" xor:"source"`
	WithChange bool   `help:"Carry over uncommitted changes to the new worktree (only with --from-branch)"`
	Silent     bool   `help:"Suppress progress messages"`
	BranchName string `arg:"" optional:"" help:"Name for the new worktree/branch (defaults to sanitised source branch name)"`
}

// WtDeleteCmd represents the wt-delete subcommand.
type WtDeleteCmd struct {
	NoBranch     bool   `help:"Skip branch deletion"`
	Silent       bool   `help:"Suppress progress messages"`
	WorktreePath string `arg:"" optional:"" help:"Worktree path/name"`
}

// ParseResult contains the results of parsing CLI arguments.
type ParseResult struct {
	CLI           *CLI
	Parser        *kong.Kong
	Context       *kong.Context
	Command       string
	InitialFilter string
	HasSubcommand bool
}

// NewParser creates a new Kong parser for the CLI.
func NewParser() (*CLI, *kong.Kong, error) {
	cli := &CLI{}
	parser, err := kong.New(cli,
		kong.Name("lazyworktree"),
		kong.Description("A TUI tool to manage git worktrees"),
		kong.UsageOnError(),
	)
	if err != nil {
		return nil, nil, err
	}

	// Set up kong-completion with custom predictors
	// This must happen before parsing so tab completion can be intercepted
	kongcompletion.Register(parser,
		kongcompletion.WithPredictor("theme", complete.PredictSet(theme.AvailableThemes()...)),
		kongcompletion.WithPredictor("config", configPredictor()),
	)

	return cli, parser, nil
}

// ParseArgs parses command-line arguments and returns a ParseResult.
func ParseArgs(args []string) (*ParseResult, error) {
	cli, parser, err := NewParser()
	if err != nil {
		return nil, err
	}

	// Build a map of boolean flags from Kong's model
	booleanFlags := buildBooleanFlagsMap(parser)

	// Helper to check if a flag name (with or without dashes) is a boolean flag
	isBooleanFlag := func(flagArg string) bool {
		// Remove =value if present
		flagName := strings.SplitN(flagArg, "=", 2)[0]
		return booleanFlags[flagName]
	}

	// Check if a subcommand is provided in args
	// Only check in positions where subcommands can appear (not after value flags)
	hasSubcommand := detectSubcommand(args, isBooleanFlag)

	// Extract potential filter args (non-flag, non-subcommand args) before Kong parsing
	initialFilter := extractFilterArgs(args, isBooleanFlag)

	ctx, err := parser.Parse(args)
	var cmd string
	if err != nil {
		// If no subcommand was provided and we get a "missing command" or "unexpected argument" error,
		// treat it as valid and proceed to TUI mode (the argument is likely a filter)
		errStr := err.Error()
		if !hasSubcommand && (strings.Contains(errStr, "expected one of") || strings.Contains(errStr, "unexpected argument")) {
			// This is the "missing command" or "unexpected argument" error - it's OK, we'll launch TUI
			// The flags should still be parsed in the cli struct
			// ctx will be nil, so we skip subcommand handling
			cmd = ""
		} else {
			// Some other error occurred
			return nil, err
		}
	} else {
		// No error, get the command from context
		cmd = ctx.Command()
	}

	return &ParseResult{
		CLI:           cli,
		Parser:        parser,
		Context:       ctx,
		Command:       cmd,
		InitialFilter: initialFilter,
		HasSubcommand: hasSubcommand,
	}, nil
}

// buildBooleanFlagsMap builds a map of boolean flags from Kong's model.
func buildBooleanFlagsMap(parser *kong.Kong) map[string]bool {
	booleanFlags := make(map[string]bool)
	if parser.Model != nil {
		for _, flag := range parser.Model.Flags {
			if flag.IsBool() {
				booleanFlags["--"+flag.Name] = true
				if flag.Short != 0 {
					booleanFlags["-"+string(flag.Short)] = true
				}
			}
		}
	}
	return booleanFlags
}

// detectSubcommand checks if a subcommand is provided in args.
// It only checks in positions where subcommands can appear (not after value flags).
func detectSubcommand(args []string, isBooleanFlag func(string) bool) bool {
	expectingFlagValue := false
	for i, arg := range args {
		if expectingFlagValue {
			expectingFlagValue = false
			continue
		}
		if strings.HasPrefix(arg, "-") {
			// Check if this is a boolean flag
			if isBooleanFlag(arg) {
				// Boolean flag, doesn't take a value
				continue
			}
			// This flag takes a value, check if next arg is the value
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				expectingFlagValue = true
			}
			continue
		}
		// Check if it's a subcommand (only if we're not expecting a flag value)
		if arg == "wt-create" || arg == "wt-delete" || arg == "completion" {
			return true
		}
	}
	return false
}

// extractFilterArgs extracts potential filter args (non-flag, non-subcommand args).
func extractFilterArgs(args []string, isBooleanFlag func(string) bool) string {
	var filterArgs []string
	expectingFlagValue := false
	for i, arg := range args {
		if expectingFlagValue {
			expectingFlagValue = false
			continue
		}
		// Skip flags
		if strings.HasPrefix(arg, "-") {
			// Check if this flag takes a value (has = in it)
			if strings.Contains(arg, "=") {
				// Flag with =value, already handled
				continue
			}
			// Check if this is a boolean flag
			if isBooleanFlag(arg) {
				// Boolean flag, doesn't take a value, continue
				continue
			}
			// Check if next arg is a value (doesn't start with -)
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				// Next arg might be a flag value, skip it
				expectingFlagValue = true
			}
			continue
		}
		// Check if it's a subcommand
		if arg == "wt-create" || arg == "wt-delete" || arg == "completion" {
			// Skip subcommand and let Kong handle it
			break
		}
		// This is a potential filter arg
		filterArgs = append(filterArgs, arg)
	}
	return strings.Join(filterArgs, " ")
}

// configPredictor returns a predictor function for --config flag completion.
// It suggests config keys in the format "lw.key=value" and appropriate values for each key.
func configPredictor() complete.Predictor {
	return complete.PredictFunc(func(args complete.Args) []string {
		last := args.Last

		// If empty, suggest starting with "lw."
		if last == "" {
			return []string{"lw."}
		}

		// If it doesn't start with "lw.", suggest "lw."
		if !strings.HasPrefix(last, "lw.") {
			return []string{"lw."}
		}

		// Check if there's an "=" sign
		parts := strings.SplitN(last, "=", 2)
		if len(parts) == 1 {
			// No "=" yet, suggest config keys with full "lw.key=" format
			keyPrefix := strings.TrimPrefix(parts[0], "lw.")
			return suggestConfigKeys(keyPrefix)
		}

		// There's an "=", suggest values for the key
		key := strings.TrimPrefix(parts[0], "lw.")
		return suggestConfigValues(key)
	})
}

// suggestConfigKeys returns config key suggestions matching the prefix.
// Returns suggestions in the format "lw.key=" for completion.
func suggestConfigKeys(prefix string) []string {
	allKeys := []string{
		"theme", "worktree_dir", "sort_mode", "auto_fetch_prs", "auto_refresh",
		"refresh_interval", "search_auto_select", "fuzzy_finder_input", "show_icons",
		"max_untracked_diffs", "max_diff_chars", "max_name_length", "git_pager",
		"git_pager_args", "git_pager_interactive", "pager", "editor", "trust_mode",
		"debug_log", "init_commands", "terminate_commands", "merge_method",
		"issue_branch_name_template", "pr_branch_name_template", "branch_name_script",
		"session_prefix", "palette_mru", "palette_mru_limit",
	}

	var matches []string
	for _, key := range allKeys {
		if prefix == "" || strings.HasPrefix(key, prefix) {
			// Return full format "lw.key=" for completion
			matches = append(matches, "lw."+key+"=")
		}
	}
	return matches
}

// suggestConfigValues returns value suggestions for a given config key.
func suggestConfigValues(key string) []string {
	switch key {
	case "theme":
		return theme.AvailableThemes()
	case "sort_mode":
		return []string{"switched", "active", "path"}
	case "merge_method":
		return []string{"rebase", "merge"}
	case "trust_mode":
		return []string{"tofu", "never", "always"}
	case "auto_fetch_prs", "auto_refresh", "search_auto_select", "fuzzy_finder_input",
		"show_icons", "git_pager_interactive", "palette_mru":
		return []string{"true", "false"}
	default:
		// For other keys, return empty to let shell handle file/path completion
		return nil
	}
}
