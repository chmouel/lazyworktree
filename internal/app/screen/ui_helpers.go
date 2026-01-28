package screen

// UIIcon identifies UI-specific icons.
type UIIcon int

// UIIcon constants.
const (
	UIIconHelpTitle UIIcon = iota
	UIIconNavigation
	UIIconStatusPane
	UIIconLogPane
	UIIconCommitTree
	UIIconWorktreeActions
	UIIconBranchNaming
	UIIconViewingTools
	UIIconRepoOps
	UIIconBackgroundRefresh
	UIIconFilterSearch
	UIIconStatusIndicators
	UIIconHelpNavigation
	UIIconShellCompletion
	UIIconConfiguration
	UIIconIconConfiguration
	UIIconTip
	UIIconPRSelect
	UIIconIssueSelect
	UIIconCICheck
	UIIconListSelect
)

type iconProvider interface {
	GetPRIcon() string
	GetIssueIcon() string
	GetCIIcon(conclusion string) string
	GetUIIcon(icon UIIcon) string
}

type defaultIconProvider struct{}

func (p *defaultIconProvider) GetPRIcon() string {
	return "PR"
}

func (p *defaultIconProvider) GetIssueIcon() string {
	return "ISS"
}

func (p *defaultIconProvider) GetCIIcon(conclusion string) string {
	return ""
}

func (p *defaultIconProvider) GetUIIcon(icon UIIcon) string {
	return ""
}

var currentIconProvider iconProvider = &defaultIconProvider{}

// SetIconProvider sets the global icon provider.
func SetIconProvider(provider iconProvider) {
	currentIconProvider = provider
}

func getIconPR() string {
	return currentIconProvider.GetPRIcon()
}

func getIconIssue() string {
	return currentIconProvider.GetIssueIcon()
}

func uiIcon(icon UIIcon) string {
	return currentIconProvider.GetUIIcon(icon)
}

func iconWithSpace(icon string) string {
	if icon == "" {
		return ""
	}
	return icon + " "
}

func iconPrefix(icon UIIcon, showIcons bool) string {
	if !showIcons {
		return ""
	}
	return iconWithSpace(uiIcon(icon))
}

func labelWithIcon(icon UIIcon, label string, showIcons bool) string {
	return iconPrefix(icon, showIcons) + label
}

func statusIndicator(clean, showIcons bool) string {
	if showIcons {
		if clean {
			return " "
		}
		return "~"
	}
	if clean {
		return " "
	}
	return "~"
}

func aheadIndicator(showIcons bool) string {
	if showIcons {
		return "↑"
	}
	return "↑"
}

func behindIndicator(showIcons bool) string {
	if showIcons {
		return "↓"
	}
	return "↓"
}

func arrowUp(showIcons bool) string {
	if !showIcons {
		return "Up"
	}
	return "↑"
}

func arrowDown(showIcons bool) string {
	if !showIcons {
		return "Down"
	}
	return "↓"
}

func arrowLeft(showIcons bool) string {
	if !showIcons {
		return "Left"
	}
	return "←"
}

func arrowRight(showIcons bool) string {
	if !showIcons {
		return "Right"
	}
	return "→"
}

func disclosureIndicator(collapsed, showIcons bool) string {
	if !showIcons {
		if collapsed {
			return ">"
		}
		return "v"
	}
	if collapsed {
		return "▶"
	}
	return "▼"
}

func getCIStatusIcon(ciStatus string, isDraft, showIcons bool) string {
	if isDraft {
		return "D"
	}
	if showIcons {
		if icon := currentIconProvider.GetCIIcon(ciStatus); icon != "" {
			return icon
		}
	}
	switch ciStatus {
	case "success":
		return "S"
	case "failure":
		return "F"
	case "skipped":
		return "-"
	case "cancelled":
		return "C"
	case "pending":
		return "P"
	default:
		return "?"
	}
}
