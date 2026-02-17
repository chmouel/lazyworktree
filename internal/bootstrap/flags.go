// Package bootstrap provides CLI flag definitions for lazyworktree.
package bootstrap

import (
	urfavecli "github.com/urfave/cli/v3"
)

// globalFlags returns all global flags for the application.
// Note: --version is provided automatically by urfave/cli via App.Version
func globalFlags() []urfavecli.Flag {
	return []urfavecli.Flag{
		&urfavecli.StringFlag{
			Name:    "worktree-dir",
			Aliases: []string{"w"},
			Usage:   "Override the default worktree root directory",
		},
		&urfavecli.StringFlag{
			Name:  "debug-log",
			Usage: "Path to debug log file",
		},
		&urfavecli.StringFlag{
			Name:  "output-selection",
			Usage: "Write selected worktree path to a file",
		},
		&urfavecli.StringFlag{
			Name:    "theme",
			Aliases: []string{"t"},
			Usage:   "Override the UI theme",
		},
		&urfavecli.BoolFlag{
			Name:  "search-auto-select",
			Usage: "Start with filter focused",
		},
		&urfavecli.BoolFlag{
			Name:  "show-syntax-themes",
			Usage: "List available delta syntax themes",
		},
		&urfavecli.StringFlag{
			Name:  "config-file",
			Usage: "Path to configuration file",
		},
		&urfavecli.StringSliceFlag{
			Name:    "config",
			Aliases: []string{"C"},
			Usage:   "Override config values (repeatable): --config=lw.key=value",
		},
	}
}
