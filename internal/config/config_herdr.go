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
