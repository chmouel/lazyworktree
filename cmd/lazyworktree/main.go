// Package main is the entry point for the lazyworktree application.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/chmouel/lazyworktree/internal/app"
	"github.com/chmouel/lazyworktree/internal/config"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	var worktreeDir string
	var debugLog string
	var outputSelection string
	var showVersion bool

	flag.StringVar(&worktreeDir, "worktree-dir", "", "Override the default worktree root directory")
	flag.StringVar(&debugLog, "debug-log", "", "Path to debug log file")
	flag.StringVar(&outputSelection, "output-selection", "", "Write selected worktree path to a file")
	flag.BoolVar(&showVersion, "version", false, "Print version information")
	flag.Parse()

	if showVersion {
		fmt.Printf("lazyworktree version %s\ncommit: %s\nbuilt at: %s\n", version, commit, date)
		return
	}

	initialFilter := strings.Join(flag.Args(), " ")

	cfg, err := config.LoadConfig("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		cfg = config.DefaultConfig()
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
