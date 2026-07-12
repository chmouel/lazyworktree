// Package main is the entry point for the lazyworktree application.
package main

import (
	"fmt"
	"os"
	"runtime/pprof"

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
	stopProfile := startCPUProfile()
	code := bootstrap.Run(os.Args)
	stopProfile()
	os.Exit(code)
}

// startCPUProfile enables CPU profiling when LAZYWORKTREE_PPROF names a file
// path. This is a hidden diagnostic hook for performance investigations.
func startCPUProfile() func() {
	path := os.Getenv("LAZYWORKTREE_PPROF")
	if path == "" {
		return func() {}
	}
	f, err := os.Create(path) // #nosec G304 -- operator-supplied diagnostics path
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to create CPU profile %q: %v\n", path, err)
		return func() {}
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to start CPU profile: %v\n", err)
		_ = f.Close()
		return func() {}
	}
	return func() {
		pprof.StopCPUProfile()
		_ = f.Close()
	}
}
