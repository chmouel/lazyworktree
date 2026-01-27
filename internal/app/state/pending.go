package state

import (
	tea "github.com/charmbracelet/bubbletea"
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
