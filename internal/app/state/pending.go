package state

import (
	tea "charm.land/bubbletea/v2"
	"github.com/chmouel/lazyworktree/internal/config"
)

// PendingState keeps deferred command and UI input state.
type PendingState struct {
	Commands         []string
	CommandEnv       map[string]string
	CommandCwd       string
	After            func() tea.Msg
	TrustPath        string
	CustomBranchName string
	CustomBaseRef    string
	CustomMenu       *config.CustomCreateMenu
}
