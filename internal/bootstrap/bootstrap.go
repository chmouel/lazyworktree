// Package bootstrap wires the CLI graph and launches the TUI.
package bootstrap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/chmouel/lazyworktree/internal/app"
	"github.com/chmouel/lazyworktree/internal/buildinfo"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/log"
	"github.com/chmouel/lazyworktree/internal/theme"
	"github.com/chmouel/lazyworktree/internal/utils"
	"github.com/urfave/cli/v3"
)

// Run constructs the CLI application and executes it.
// It returns an exit code suitable for os.Exit.
func Run(args []string) int {
	cliApp := &cli.Command{
		Name:                  "lazyworktree",
		Usage:                 "A TUI tool to manage git worktrees",
		Version:               buildinfo.Version(),
		EnableShellCompletion: true,
		Flags:                 globalFlags(),

		Commands: []*cli.Command{
			createCommand(),
			renameCommand(),
			deleteCommand(),
			listCommand(),
		},

		Action: func(ctx context.Context, cmd *cli.Command) error {
			// Handle completion when invoked as "lazyworktree -- --generate-shell-completion"
			// (zsh completion script passes this when user types "lazyworktree --<TAB>")
			if slices.Contains(args, "--generate-shell-completion") {
				outputAllFlags(cmd)
				return nil
			}
			if cmd.Bool("show-syntax-themes") {
				printSyntaxThemes()
				os.Exit(0)
			}
			return runTUI(ctx, cmd)
		},
		Suggest: true,
		ShellComplete: func(ctx context.Context, cmd *cli.Command) {
			argsLen := len(args)
			lastArg := ""
			if argsLen > 1 {
				lastArg = args[argsLen-2]
			}

			// Handle the "--" case by treating it as a prefix for all flags
			if lastArg == "--" {
				outputAllFlags(cmd)
				return
			}

			// Delegate to default handler for other cases
			cli.DefaultCompleteWithFlags(ctx, cmd)
		},
	}

	// Update theme flag with available themes list
	for _, flag := range cliApp.Flags {
		if strFlag, ok := flag.(*cli.StringFlag); ok && strFlag.Name == "theme" {
			themes := theme.AvailableThemes()
			strFlag.Usage = fmt.Sprintf("Override the UI theme (%s)", formatThemeList(themes))
			break
		}
	}

	if err := cliApp.Run(context.Background(), args); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	return 0
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

// formatThemeList formats theme names as a comma-separated string.
func formatThemeList(themes []string) string {
	return strings.Join(themes, ", ")
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

// outputAllFlags prints all visible flags in completion format.
// Used when handling "lazyworktree -- --generate-shell-completion".
func outputAllFlags(cmd *cli.Command) {
	for _, flag := range cmd.Flags {
		if bf, ok := flag.(*cli.BoolFlag); ok && bf.Hidden {
			continue
		}
		if sf, ok := flag.(*cli.StringFlag); ok && sf.Hidden {
			continue
		}
		name := flag.Names()[0]
		usage := ""
		if df, ok := flag.(cli.DocGenerationFlag); ok {
			usage = df.GetUsage()
		}
		prefix := "--"
		if len(name) == 1 {
			prefix = "-"
		}
		if usage != "" {
			fmt.Printf("%s%s:%s\n", prefix, name, usage)
		} else {
			fmt.Printf("%s%s\n", prefix, name)
		}
	}
}
