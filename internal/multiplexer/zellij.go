package multiplexer

import (
	"fmt"
	"os"
	"strings"

	"github.com/chmouel/lazyworktree/internal/config"
)

// SanitizeZellijSessionName removes invalid characters from a zellij session name.
func SanitizeZellijSessionName(name string) string {
	if name == "" {
		return ""
	}
	replacer := strings.NewReplacer("/", "-", "\\", "-", ":", "-")
	return replacer.Replace(name)
}

// KdlQuote quotes a string for use in KDL (Zellij layout format).
func KdlQuote(input string) string {
	escaped := strings.ReplaceAll(input, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
	return "\"" + escaped + "\""
}

// BuildZellijTabLayout generates a KDL layout for a single zellij tab.
func BuildZellijTabLayout(window ResolvedWindow) string {
	var b strings.Builder
	b.WriteString("layout {\n")
	fmt.Fprintf(&b, "    tab name=%s {\n", KdlQuote(window.Name))
	b.WriteString("        pane {\n")
	if window.Cwd != "" {
		fmt.Fprintf(&b, "            cwd %s\n", KdlQuote(window.Cwd))
	}
	fmt.Fprintf(&b, "            command %s\n", KdlQuote("bash"))
	fmt.Fprintf(&b, "            args %s %s\n", KdlQuote("-lc"), KdlQuote(window.Command))
	b.WriteString("        }\n")
	b.WriteString("    }\n")
	b.WriteString("}\n")
	return b.String()
}

// WriteZellijLayouts creates temporary layout files for each window.
// Returns the list of created file paths and an error if any operation failed.
// Caller is responsible for cleanup using CleanupZellijLayouts.
func WriteZellijLayouts(windows []ResolvedWindow) ([]string, error) {
	paths := make([]string, 0, len(windows))
	for _, window := range windows {
		layoutFile, err := os.CreateTemp("", "lazyworktree-zellij-layout-")
		if err != nil {
			CleanupZellijLayouts(paths)
			return nil, err
		}
		if _, err := layoutFile.WriteString(BuildZellijTabLayout(window)); err != nil {
			_ = layoutFile.Close()
			_ = os.Remove(layoutFile.Name()) //#nosec G703 -- controlled temp file cleanup
			CleanupZellijLayouts(paths)
			return nil, err
		}
		if err := layoutFile.Close(); err != nil {
			_ = os.Remove(layoutFile.Name()) //#nosec G703 -- controlled temp file cleanup
			CleanupZellijLayouts(paths)
			return nil, err
		}
		paths = append(paths, layoutFile.Name())
	}
	return paths, nil
}

// CleanupZellijLayouts removes temporary layout files.
func CleanupZellijLayouts(paths []string) {
	for _, path := range paths {
		_ = os.Remove(path) //#nosec G703 -- controlled temp file cleanup
	}
}

// BuildZellijScript generates a shell script that creates or attaches to a zellij session.
// The script handles session existence based on zellijCfg.OnExists and creates tabs from layoutPaths.
func BuildZellijScript(sessionName string, zellijCfg *config.TmuxCommand, layoutPaths []string) string {
	onExists := strings.ToLower(strings.TrimSpace(zellijCfg.OnExists))
	switch onExists {
	case OnExistsAttach, OnExistsKill, OnExistsNew, OnExistsSwitch:
	default:
		onExists = OnExistsSwitch
	}

	var b strings.Builder
	b.WriteString("set -e\n")
	fmt.Fprintf(&b, "session=%s\n", ShellQuote(sessionName))
	b.WriteString("base_session=$session\n")
	b.WriteString("session_exists() {\n")
	b.WriteString("  zellij list-sessions --short --no-formatting 2>/dev/null | grep -v EXITED | grep -Fxq \"$1\"\n")
	b.WriteString("}\n")
	b.WriteString("created=false\n")
	b.WriteString("if session_exists \"$session\"; then\n")
	switch onExists {
	case OnExistsKill:
		b.WriteString("  zellij kill-session \"$session\"\n")
	case OnExistsNew:
		b.WriteString("  i=2\n")
		b.WriteString("  while session_exists \"${base_session}-$i\"; do i=$((i+1)); done\n")
		b.WriteString("  session=\"${base_session}-$i\"\n")
	default:
		b.WriteString("  :\n")
	}
	b.WriteString("fi\n")
	b.WriteString("if ! session_exists \"$session\"; then\n")
	b.WriteString("  zellij attach --create-background \"$session\"\n")
	b.WriteString("  created=true\n")
	// Wait for session with timeout (5 seconds max)
	b.WriteString("  tries=0\n")
	b.WriteString("  while ! zellij list-sessions --short --no-formatting 2>/dev/null | grep -v EXITED | grep -Fxq \"$session\"; do\n")
	b.WriteString("    sleep 0.1\n")
	b.WriteString("    tries=$((tries+1))\n")
	b.WriteString("    if [ $tries -ge 50 ]; then echo \"Timeout waiting for zellij session\" >&2; exit 1; fi\n")
	b.WriteString("  done\n")
	b.WriteString("fi\n")
	if len(layoutPaths) > 0 {
		b.WriteString("if [ \"$created\" = \"true\" ]; then\n")
		for _, layoutPath := range layoutPaths {
			fmt.Fprintf(&b, "  ZELLIJ_SESSION_NAME=\"$session\" zellij action new-tab --layout %s\n", ShellQuote(layoutPath))
		}
		b.WriteString("  ZELLIJ_SESSION_NAME=\"$session\" zellij action go-to-tab 1\n")
		b.WriteString("  ZELLIJ_SESSION_NAME=\"$session\" zellij action close-tab\n")
		b.WriteString("fi\n")
	}
	b.WriteString("if [ -n \"${LW_ZELLIJ_SESSION_FILE:-}\" ]; then printf '%s' \"$session\" > \"$LW_ZELLIJ_SESSION_FILE\"; fi\n")
	return b.String()
}
