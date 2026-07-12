//go:build windows

package commands

import "strings"

func quoteHookExecutable(path string) string {
	return `"` + strings.ReplaceAll(path, `"`, `""`) + `"`
}
