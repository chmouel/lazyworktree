// Package buildinfo centralises build metadata for the lazyworktree binary.
// The linker injects values into cmd/lazyworktree/main.go; main() calls Set()
// to forward them here so every other package can query them.
package buildinfo

import "runtime/debug"

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	builtBy = "unknown"
)

// Set stores the build metadata received from linker-injected variables.
func Set(v, c, d, b string) {
	version = v
	commit = c
	date = d
	builtBy = b
}

// Version returns the build version string.
func Version() string { return version }

// Commit returns the build commit hash.
func Commit() string { return commit }

// Date returns the build date string.
func Date() string { return date }

// BuiltBy returns the build agent string.
func BuiltBy() string { return builtBy }

// Enrich fills missing metadata from runtime/debug.ReadBuildInfo().
// It overwrites commit when it equals "none" and builtBy when it
// equals "unknown", using VCS revision and Go version respectively.
func Enrich() {
	if commit != "none" && builtBy != "unknown" {
		return
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}

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
