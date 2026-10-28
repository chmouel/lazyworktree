package config

import "strings"

// Herdr modes select whether a custom command manages one workspace or a whole
// named Herdr session.
const (
	HerdrModeWorkspace = "workspace"
	HerdrModeSession   = "session"
)

// HerdrCommand represents a configured Herdr workspace or session layout.
//
// SessionName is the workspace label in workspace mode and the Herdr session
// name in session mode. HerdrSession optionally selects the Herdr session that
// hosts a workspace and is ignored in session mode. WorktreeGroup opens the
// workspace through Herdr's worktree commands so it is grouped under its
// repository.
type HerdrCommand struct {
	Mode          string
	SessionName   string
	HerdrSession  string
	Attach        bool
	OnExists      string
	WorktreeGroup bool
	Windows       []TmuxWindow
}

func parseHerdrCommand(data map[string]any) *HerdrCommand {
	shared := parseTmuxCommand(data)
	cmd := &HerdrCommand{
		Mode:          strings.ToLower(strings.TrimSpace(getString(data, "mode"))),
		SessionName:   shared.SessionName,
		HerdrSession:  getString(data, "herdr_session"),
		Attach:        shared.Attach,
		OnExists:      shared.OnExists,
		WorktreeGroup: coerceBool(data["worktree_group"], true),
		Windows:       shared.Windows,
	}
	if cmd.Mode != HerdrModeSession {
		cmd.Mode = HerdrModeWorkspace
	}
	return cmd
}

// HerdrCreateLayout returns the custom command whose herdr block lays out the
// workspaces Herdr creates for new worktrees when use_herdr_for_workspace_create
// is enabled. The key resolves like a key press in the worktrees pane, so a
// worktrees binding wins over a universal one. A nil command selects Herdr's
// plain worktree workspace; the warning explains why a configured key cannot
// be used.
func (c *AppConfig) HerdrCreateLayout() (*CustomCommand, string) {
	key := strings.TrimSpace(c.HerdrWorkspaceCreateLayout)
	if key == "" {
		return nil, ""
	}
	cmd, ok := c.CustomCommands.Lookup(PaneWorktrees, key)
	switch {
	case !ok:
		return nil, "herdr_workspace_create_layout: custom command " + key + " not found; using a plain Herdr worktree workspace"
	case cmd.Herdr == nil:
		return nil, "herdr_workspace_create_layout: custom command " + key + " has no herdr block; using a plain Herdr worktree workspace"
	case cmd.Herdr.Mode != HerdrModeWorkspace:
		return nil, "herdr_workspace_create_layout: custom command " + key + " uses session mode, which cannot hold a worktree group; using a plain Herdr worktree workspace"
	}
	return cmd, ""
}
