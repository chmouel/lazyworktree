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
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/theme"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	builtBy = "unknown"
)

func main() {
	var worktreeDir string
	var debugLog string
	var outputSelection string
	var themeName string
	var searchAutoSelect bool
	var showVersion bool
	var showSyntaxThemes bool

	flag.StringVar(&worktreeDir, "worktree-dir", "", "Override the default worktree root directory")
	flag.StringVar(&debugLog, "debug-log", "", "Path to debug log file")
	flag.StringVar(&outputSelection, "output-selection", "", "Write selected worktree path to a file")
	flag.StringVar(&themeName, "theme", "", "Override the UI theme (supported: dracula, narna, clean-light, solarized-dark, solarized-light, gruvbox-dark, gruvbox-light, nord, monokai, catppuccin-mocha)")
	flag.BoolVar(&searchAutoSelect, "search-auto-select", false, "Start with filter focused and select first match on Enter")
	flag.BoolVar(&showVersion, "version", false, "Print version information")
	flag.BoolVar(&showSyntaxThemes, "show-syntax-themes", false, "List available delta syntax themes")
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

	initialFilter := strings.Join(flag.Args(), " ")

	cfg, err := config.LoadConfig("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		cfg = config.DefaultConfig()
	}

	if themeName != "" {
		normalized := config.NormalizeThemeName(themeName)
		if normalized == "" {
			fmt.Fprintf(os.Stderr, "Unknown theme %q\n", themeName)
			os.Exit(1)
		}
		cfg.Theme = normalized
		if !cfg.DeltaArgsSet {
			cfg.DeltaArgs = config.DefaultDeltaArgsForTheme(normalized)
		}
	}
	if searchAutoSelect {
		cfg.SearchAutoSelect = true
	}

	switch {
	case worktreeDir != "":
		expanded, err := expandPath(worktreeDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error expanding worktree-dir: %v\n", err)
			os.Exit(1)
		}
		cfg.WorktreeDir = expanded
	case cfg.WorktreeDir != "":
		expanded, err := expandPath(cfg.WorktreeDir)
		if err == nil {
			cfg.WorktreeDir = expanded
		}
	default:
		home, _ := os.UserHomeDir()
		cfg.WorktreeDir = filepath.Join(home, ".local", "share", "worktrees")
	}

	if debugLog != "" {
		expanded, err := expandPath(debugLog)
		if err == nil {
			cfg.DebugLog = expanded
		} else {
			cfg.DebugLog = debugLog
		}
	}

	model := app.NewModel(cfg, initialFilter)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())

	_, err = p.Run()
	model.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running app: %v\n", err)
		os.Exit(1)
	}

	selectedPath := model.GetSelectedPath()
	if outputSelection != "" {
		expanded, err := expandPath(outputSelection)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error expanding output-selection: %v\n", err)
			os.Exit(1)
		}
		const defaultDirPerms = 0o750
		if err := os.MkdirAll(filepath.Dir(expanded), defaultDirPerms); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating output-selection dir: %v\n", err)
			os.Exit(1)
		}
		data := ""
		if selectedPath != "" {
			data = selectedPath + "\n"
		}
		const defaultFilePerms = 0o600
		if err := os.WriteFile(expanded, []byte(data), defaultFilePerms); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output-selection: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if selectedPath != "" {
		fmt.Println(selectedPath)
	}
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

func printSyntaxThemes() {
	names := theme.AvailableThemes()
	sort.Strings(names)
	fmt.Println("Available syntax themes (delta --syntax-theme defaults):")
	for _, name := range names {
		fmt.Printf("  %-16s -> %s\n", name, config.SyntaxThemeForUITheme(name))
	}
}
