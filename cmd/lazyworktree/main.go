// Package main is the entry point for the lazyworktree application.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/chmouel/lazyworktree/internal/app"
	"github.com/chmouel/lazyworktree/internal/completion"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/log"
	"github.com/chmouel/lazyworktree/internal/theme"
	"github.com/chmouel/lazyworktree/internal/utils"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	builtBy = "unknown"
)

// configOverrides is a custom flag type for repeatable --config flags.
type configOverrides []string

func (c *configOverrides) String() string {
	return strings.Join(*c, ",")
}

func (c *configOverrides) Set(value string) error {
	*c = append(*c, value)
	return nil
}

func main() {
	var worktreeDir string
	var debugLog string
	var outputSelection string
	var themeName string
	var searchAutoSelect bool
	var showVersion bool
	var showSyntaxThemes bool
	var completionShell string
	var configFile string
	var configOverrideList configOverrides

	flag.StringVar(&worktreeDir, "worktree-dir", "", "Override the default worktree root directory")
	flag.StringVar(&debugLog, "debug-log", "", "Path to debug log file")
	flag.StringVar(&outputSelection, "output-selection", "", "Write selected worktree path to a file")
	flag.StringVar(&themeName, "theme", "", themeHelpText())
	flag.BoolVar(&searchAutoSelect, "search-auto-select", false, "Start with filter focused")
	flag.BoolVar(&showVersion, "version", false, "Print version information")
	flag.BoolVar(&showSyntaxThemes, "show-syntax-themes", false, "List available delta syntax themes")
	flag.StringVar(&completionShell, "completion", "", "Generate shell completion script (bash, zsh, fish)")
	flag.StringVar(&configFile, "config-file", "", "Path to configuration file")
	flag.Var(&configOverrideList, "config", "Override config values (repeatable): --config=lw.key=value")

	// Custom usage message to include subcommands
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [OPTIONS] [SUBCOMMAND]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "A TUI tool to manage git worktrees\n\n")
		fmt.Fprintf(os.Stderr, "Subcommands:\n")
		fmt.Fprintf(os.Stderr, "  wt-create    Create a new worktree\n")
		fmt.Fprintf(os.Stderr, "  wt-delete    Delete a worktree\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nRun '%s SUBCOMMAND --help' for more information on a subcommand.\n", os.Args[0])
	}

	flag.Parse()

	if showVersion {
		if commit == "none" || builtBy == "unknown" {
			if info, ok := debug.ReadBuildInfo(); ok {
				if commit == "none" {
					for _, setting := range info.Settings {
						if setting.Key == "vcs.revision" {
							commit = setting.Value
						}
					}
				}
				if builtBy == "unknown" {
					builtBy = info.GoVersion
				}
			}
		}

		fmt.Printf("lazyworktree version %s\ncommit: %s\nbuilt at: %s\nbuilt by: %s\n", version, commit, date, builtBy)
		return
	}
	if showSyntaxThemes {
		printSyntaxThemes()
		return
	}
	if completionShell != "" {
		if err := printCompletion(completionShell); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating completion: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Get subcommand (first positional argument)
	args := flag.Args()
	if len(args) > 0 {
		subcommand := args[0]
		switch subcommand {
		case "wt-create":
			handleWtCreate(args[1:], worktreeDir, configFile, configOverrideList)
			return
		case "wt-delete":
			handleWtDelete(args[1:], worktreeDir, configFile, configOverrideList)
			return
		}
		// If not a recognized subcommand, treat as initial filter for TUI
	}

	initialFilter := strings.Join(args, " ")

	// Set up debug logging before loading config, so debug output is captured
	if debugLog != "" {
		expanded, err := utils.ExpandPath(debugLog)
		if err == nil {
			if err := log.SetFile(expanded); err != nil {
				fmt.Fprintf(os.Stderr, "Error opening debug log file %q: %v\n", expanded, err)
			}
		} else {
			if err := log.SetFile(debugLog); err != nil {
				fmt.Fprintf(os.Stderr, "Error opening debug log file %q: %v\n", debugLog, err)
			}
		}
	}

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		cfg = config.DefaultConfig()
	}

	// If debug log wasn't set via flag, check if it's in the config
	// If it is, enable logging. If not, disable logging and discard buffer.
	if debugLog == "" {
		if cfg.DebugLog != "" {
			expanded, err := utils.ExpandPath(cfg.DebugLog)
			path := cfg.DebugLog
			if err == nil {
				path = expanded
			}
			if err := log.SetFile(path); err != nil {
				fmt.Fprintf(os.Stderr, "Error opening debug log file from config %q: %v\n", path, err)
			}
		} else {
			// No debug log configured, discard any buffered logs
			_ = log.SetFile("")
		}
	}

	if themeName != "" {
		normalized := config.NormalizeThemeName(themeName)
		if normalized == "" {
			fmt.Fprintf(os.Stderr, "Unknown theme %q\n", themeName)
			_ = log.Close()
			os.Exit(1)
		}
		cfg.Theme = normalized
		if !cfg.GitPagerArgsSet && filepath.Base(cfg.GitPager) == "delta" {
			cfg.GitPagerArgs = config.DefaultDeltaArgsForTheme(normalized)
		}
	}
	if searchAutoSelect {
		cfg.SearchAutoSelect = true
	}

	switch {
	case worktreeDir != "":
		expanded, err := utils.ExpandPath(worktreeDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error expanding worktree-dir: %v\n", err)
			_ = log.Close()
			os.Exit(1)
		}
		cfg.WorktreeDir = expanded
	case cfg.WorktreeDir != "":
		expanded, err := utils.ExpandPath(cfg.WorktreeDir)
		if err == nil {
			cfg.WorktreeDir = expanded
		}
	default:
		home, _ := os.UserHomeDir()
		cfg.WorktreeDir = filepath.Join(home, ".local", "share", "worktrees")
	}

	if debugLog != "" {
		expanded, err := utils.ExpandPath(debugLog)
		if err == nil {
			cfg.DebugLog = expanded
		} else {
			cfg.DebugLog = debugLog
		}
	}

	// Apply CLI config overrides (highest precedence)
	if len(configOverrideList) > 0 {
		if err := cfg.ApplyCLIOverrides(configOverrideList); err != nil {
			fmt.Fprintf(os.Stderr, "Error applying config overrides: %v\n", err)
			_ = log.Close()
			os.Exit(1)
		}
	}

	model := app.NewModel(cfg, initialFilter)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())

	_, err = p.Run()
	model.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running app: %v\n", err)
		_ = log.Close()
		os.Exit(1)
	}

	selectedPath := model.GetSelectedPath()
	if outputSelection != "" {
		expanded, err := utils.ExpandPath(outputSelection)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error expanding output-selection: %v\n", err)
			_ = log.Close()
			os.Exit(1)
		}
		const defaultDirPerms = 0o750
		if err := os.MkdirAll(filepath.Dir(expanded), defaultDirPerms); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating output-selection dir: %v\n", err)
			_ = log.Close()
			os.Exit(1)
		}
		data := ""
		if selectedPath != "" {
			data = selectedPath + "\n"
		}
		const defaultFilePerms = 0o600
		if err := os.WriteFile(expanded, []byte(data), defaultFilePerms); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output-selection: %v\n", err)
			_ = log.Close()
			os.Exit(1)
		}
		return
	}
	if selectedPath != "" {
		fmt.Println(selectedPath)
	}
	if err := log.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "Error closing debug log: %v\n", err)
	}
}

// applyWorktreeDirConfig applies the worktree directory configuration.
// This ensures the same path expansion logic is used in both TUI and CLI modes.
func applyWorktreeDirConfig(cfg *config.AppConfig, worktreeDirFlag string) error {
	switch {
	case worktreeDirFlag != "":
		expanded, err := utils.ExpandPath(worktreeDirFlag)
		if err != nil {
			return fmt.Errorf("error expanding worktree-dir: %w", err)
		}
		cfg.WorktreeDir = expanded
	case cfg.WorktreeDir != "":
		expanded, err := utils.ExpandPath(cfg.WorktreeDir)
		if err == nil {
			cfg.WorktreeDir = expanded
		}
	default:
		home, _ := os.UserHomeDir()
		cfg.WorktreeDir = filepath.Join(home, ".local", "share", "worktrees")
	}
	return nil
}

func printSyntaxThemes() {
	names := theme.AvailableThemes()
	sort.Strings(names)
	fmt.Println("Available syntax themes (delta --syntax-theme defaults):")
	for _, name := range names {
		fmt.Printf("  %-16s -> %s\n", name, config.SyntaxThemeForUITheme(name))
	}
}

func themeHelpText() string {
	names := theme.AvailableThemes()
	sort.Strings(names)
	return fmt.Sprintf("Override the UI theme (supported: %s)", strings.Join(names, ", "))
}

func printCompletion(shell string) error {
	script, err := completion.Generate(shell)
	if err != nil {
		return err
	}
	fmt.Println(script)
	return nil
}
