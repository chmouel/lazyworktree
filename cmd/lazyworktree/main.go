// Package main is the entry point for the lazyworktree application.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/chmouel/lazyworktree/internal/app"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/log"
	"github.com/chmouel/lazyworktree/internal/theme"
	"github.com/chmouel/lazyworktree/internal/utils"
	"github.com/urfave/cli/v3"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	builtBy = "unknown"
)

func main() {
	cliApp := &cli.Command{
		Name:                  "lazyworktree",
		Usage:                 "A TUI tool to manage git worktrees",
		Version:               version,
		EnableShellCompletion: true,
		Flags:                 globalFlags(),

		Commands: []*cli.Command{
			wtCreateCommand(),
			wtDeleteCommand(),
		},

		Action: func(ctx context.Context, cmd *cli.Command) error {
			if cmd.Bool("show-syntax-themes") {
				printSyntaxThemes()
				os.Exit(0)
			}
			if cmd.Bool("generate-shell-completion") {
				os.Exit(0)
			}
			return runTUI(ctx, cmd)
		},
		Suggest: true,
	}

	if err := cliApp.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func runTUI(_ context.Context, cmd *cli.Command) error {
	if debugLog := cmd.String("debug-log"); debugLog != "" {
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

	cfg, err := config.LoadConfig(cmd.String("config-file"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		cfg = config.DefaultConfig()
	}

	if cmd.String("debug-log") == "" {
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

	if err := applyThemeConfig(cfg, cmd.String("theme")); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		_ = log.Close()
		return err
	}

	if cmd.Bool("search-auto-select") {
		cfg.SearchAutoSelect = true
	}

	if err := applyWorktreeDirConfig(cfg, cmd.String("worktree-dir")); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		_ = log.Close()
		return err
	}

	if debugLog := cmd.String("debug-log"); debugLog != "" {
		expanded, err := utils.ExpandPath(debugLog)
		if err == nil {
			cfg.DebugLog = expanded
		} else {
			cfg.DebugLog = debugLog
		}
	}

	if configOverrides := cmd.StringSlice("config"); len(configOverrides) > 0 {
		if err := cfg.ApplyCLIOverrides(configOverrides); err != nil {
			fmt.Fprintf(os.Stderr, "Error applying config overrides: %v\n", err)
			_ = log.Close()
			return err
		}
	}

	model := app.NewModel(cfg, "")
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())

	_, err = p.Run()
	model.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running app: %v\n", err)
		_ = log.Close()
		return err
	}

	// Handle output-selection flag
	selectedPath := model.GetSelectedPath()
	if outputSelection := cmd.String("output-selection"); outputSelection != "" {
		expanded, err := utils.ExpandPath(outputSelection)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error expanding output-selection: %v\n", err)
			_ = log.Close()
			return err
		}
		const defaultDirPerms = 0o750
		if err := os.MkdirAll(filepath.Dir(expanded), defaultDirPerms); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating output-selection dir: %v\n", err)
			_ = log.Close()
			return err
		}
		data := ""
		if selectedPath != "" {
			data = selectedPath + "\n"
		}
		const defaultFilePerms = 0o600
		if err := os.WriteFile(expanded, []byte(data), defaultFilePerms); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output-selection: %v\n", err)
			_ = log.Close()
			return err
		}
		_ = log.Close()
		return nil
	}

	// Print selected path if any
	if selectedPath != "" {
		fmt.Println(selectedPath)
	}

	if err := log.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "Error closing debug log: %v\n", err)
	}

	return nil
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

// printSyntaxThemes prints available syntax themes for delta.
func printSyntaxThemes() {
	names := theme.AvailableThemes()
	sort.Strings(names)
	fmt.Println("Available syntax themes (delta --syntax-theme defaults):")
	for _, name := range names {
		fmt.Printf("  %-16s -> %s\n", name, config.SyntaxThemeForUITheme(name))
	}
}

// printVersion prints version information.
func printVersion() {
	v := version
	c := commit
	d := date
	b := builtBy

	if c == "none" || b == "unknown" {
		if info, ok := debug.ReadBuildInfo(); ok {
			if c == "none" {
				for _, setting := range info.Settings {
					if setting.Key == "vcs.revision" {
						c = setting.Value
					}
				}
			}
			if b == "unknown" {
				b = info.GoVersion
			}
		}
	}

	fmt.Printf("lazyworktree version %s\ncommit: %s\nbuilt at: %s\nbuilt by: %s\n", v, c, d, b)
}

// applyThemeConfig applies theme configuration from command line flag.
func applyThemeConfig(cfg *config.AppConfig, themeName string) error {
	if themeName == "" {
		return nil
	}

	normalized := config.NormalizeThemeName(themeName)
	if normalized == "" {
		return fmt.Errorf("unknown theme %q", themeName)
	}

	cfg.Theme = normalized
	if !cfg.GitPagerArgsSet && filepath.Base(cfg.GitPager) == "delta" {
		cfg.GitPagerArgs = config.DefaultDeltaArgsForTheme(normalized)
	}

	return nil
}
