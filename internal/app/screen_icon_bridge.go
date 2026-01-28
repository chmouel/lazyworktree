package app

import "github.com/chmouel/lazyworktree/internal/app/screen"

// appIconProviderBridge adapts the app's IconProvider to the screen package's interface.
type appIconProviderBridge struct{}

func (b *appIconProviderBridge) GetPRIcon() string {
	return getIconPR()
}

func (b *appIconProviderBridge) GetIssueIcon() string {
	return getIconIssue()
}

func (b *appIconProviderBridge) GetCIIcon(conclusion string) string {
	return ciIconForConclusion(conclusion)
}

func (b *appIconProviderBridge) GetUIIcon(icon screen.UIIcon) string {
	var appIcon UIIcon
	switch icon {
	case screen.UIIconHelpTitle:
		appIcon = UIIconHelpTitle
	case screen.UIIconNavigation:
		appIcon = UIIconNavigation
	case screen.UIIconStatusPane:
		appIcon = UIIconStatusPane
	case screen.UIIconLogPane:
		appIcon = UIIconLogPane
	case screen.UIIconCommitTree:
		appIcon = UIIconCommitTree
	case screen.UIIconWorktreeActions:
		appIcon = UIIconWorktreeActions
	case screen.UIIconBranchNaming:
		appIcon = UIIconBranchNaming
	case screen.UIIconViewingTools:
		appIcon = UIIconViewingTools
	case screen.UIIconRepoOps:
		appIcon = UIIconRepoOps
	case screen.UIIconBackgroundRefresh:
		appIcon = UIIconBackgroundRefresh
	case screen.UIIconFilterSearch:
		appIcon = UIIconFilterSearch
	case screen.UIIconStatusIndicators:
		appIcon = UIIconStatusIndicators
	case screen.UIIconHelpNavigation:
		appIcon = UIIconHelpNavigation
	case screen.UIIconShellCompletion:
		appIcon = UIIconShellCompletion
	case screen.UIIconConfiguration:
		appIcon = UIIconConfiguration
	case screen.UIIconIconConfiguration:
		appIcon = UIIconIconConfiguration
	case screen.UIIconTip:
		appIcon = UIIconTip
	case screen.UIIconPRSelect:
		appIcon = UIIconPRSelect
	case screen.UIIconIssueSelect:
		appIcon = UIIconIssueSelect
	case screen.UIIconCICheck:
		appIcon = UIIconCICheck
	default:
		return ""
	}
	return uiIcon(appIcon)
}

// init sets up the icon provider bridge for the screen package.
func init() {
	screen.SetIconProvider(&appIconProviderBridge{})
	// Set the devicon provider for the commit files screen
	screen.SetIconProviderFunc(deviconForName)
}
