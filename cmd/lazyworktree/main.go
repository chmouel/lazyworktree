// Package main is the entry point for the lazyworktree application.
package main

import (
	"os"

	"github.com/chmouel/lazyworktree/internal/bootstrap"
	"github.com/chmouel/lazyworktree/internal/buildinfo"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	builtBy = "unknown"
)

func main() {
	buildinfo.Set(version, commit, date, builtBy)
	os.Exit(bootstrap.Run(os.Args))
}
