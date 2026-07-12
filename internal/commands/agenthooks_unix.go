//go:build !windows

package commands

import "github.com/chmouel/lazyworktree/internal/multiplexer"

func quoteHookExecutable(path string) string {
	return multiplexer.ShellQuote(path)
}
