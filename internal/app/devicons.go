package app

// IconProvider defines the interface for providing icons.
type IconProvider interface {
	GetFileIcon(name string, isDir bool) string
	GetPRIcon() string
	GetIssueIcon() string
	GetCIIcon(conclusion string) string
	GetUIIcon(icon UIIcon) string
}

var currentIconProvider IconProvider = &NerdFontV3Provider{}

// SetIconProvider sets the current icon provider.
func SetIconProvider(p IconProvider) {
	currentIconProvider = p
}

// UIIcon identifies UI-specific icons that follow the selected icon set.
type UIIcon int

// UIIcon values map UI elements to icon set glyphs.
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
	UIIconSearch
	UIIconFilter
	UIIconZoom
	UIIconBot
	UIIconThemeSelect
	UIIconPRSelect
	UIIconIssueSelect
	UIIconWorktreeMain
	UIIconWorktree
	UIIconStatusClean
	UIIconStatusDirty
	UIIconSyncClean
	UIIconAhead
	UIIconBehind
	UIIconArrowLeft
	UIIconArrowRight
)

const (
	nerdFontUIIconHelpTitle         = ""
	nerdFontUIIconNavigation        = ""
	nerdFontUIIconStatusPane        = ""
	nerdFontUIIconLogPane           = ""
	nerdFontUIIconCommitTree        = ""
	nerdFontUIIconWorktreeActions   = ""
	nerdFontUIIconBranchNaming      = ""
	nerdFontUIIconViewingTools      = ""
	nerdFontUIIconRepoOps           = ""
	nerdFontUIIconBackgroundRefresh = ""
	nerdFontUIIconFilterSearch      = ""
	nerdFontUIIconStatusIndicators  = ""
	nerdFontUIIconHelpNavigation    = ""
	nerdFontUIIconShellCompletion   = ""
	nerdFontUIIconConfiguration     = ""
	nerdFontUIIconIconConfiguration = ""
	nerdFontUIIconTip               = ""
	nerdFontUIIconSearch            = ""
	nerdFontUIIconFilter            = ""
	nerdFontUIIconZoom              = ""
	nerdFontUIIconBot               = ""
	nerdFontUIIconThemeSelect       = ""
	nerdFontUIIconWorktreeMain      = ""
	nerdFontUIIconWorktree          = ""
	nerdFontUIIconStatusClean       = ""
	nerdFontUIIconStatusDirty       = ""
	nerdFontUIIconAhead             = "↑"
	nerdFontUIIconBehind            = "↓"
	nerdFontUIIconArrowLeft         = "←"
	nerdFontUIIconArrowRight        = "→"
)

const (
	unicodeUIIconHelpTitle         = "*"
	unicodeUIIconNavigation        = ">"
	unicodeUIIconStatusPane        = "S"
	unicodeUIIconLogPane           = "L"
	unicodeUIIconCommitTree        = "T"
	unicodeUIIconWorktreeActions   = "W"
	unicodeUIIconBranchNaming      = "B"
	unicodeUIIconViewingTools      = "/"
	unicodeUIIconRepoOps           = "R"
	unicodeUIIconBackgroundRefresh = "H"
	unicodeUIIconFilterSearch      = "/"
	unicodeUIIconStatusIndicators  = "I"
	unicodeUIIconHelpNavigation    = "?"
	unicodeUIIconShellCompletion   = "C"
	unicodeUIIconConfiguration     = "C"
	unicodeUIIconIconConfiguration = "I"
	unicodeUIIconTip               = "!"
	unicodeUIIconSearch            = "/"
	unicodeUIIconFilter            = "F"
	unicodeUIIconZoom              = "Z"
	unicodeUIIconBot               = "B"
	unicodeUIIconThemeSelect       = "T"
	unicodeUIIconWorktreeMain      = "M"
	unicodeUIIconWorktree          = "W"
	unicodeUIIconStatusClean       = "C"
	unicodeUIIconStatusDirty       = "D"
	unicodeUIIconAhead             = "↑"
	unicodeUIIconBehind            = "↓"
	unicodeUIIconArrowLeft         = "←"
	unicodeUIIconArrowRight        = "→"
)

// NerdFontV3Provider implements IconProvider for Nerd Font v3.
type NerdFontV3Provider struct{}

// GetFileIcon returns the file icon for the given name and type.
func (p *NerdFontV3Provider) GetFileIcon(name string, isDir bool) string {
	if name == "" {
		return ""
	}
	return lazyGitFileIcon(name, isDir, 3)
}

// GetPRIcon returns the PR icon.
func (p *NerdFontV3Provider) GetPRIcon() string { return "" }

// GetIssueIcon returns the issue icon.
func (p *NerdFontV3Provider) GetIssueIcon() string { return "󰄱" }

const (
	iconSuccess   = "success"
	iconFailure   = "failure"
	iconSkipped   = "skipped"
	iconCancelled = "cancelled"
	iconPending   = "pending"
)

// GetCIIcon returns the CI status icon for the given conclusion.
func (p *NerdFontV3Provider) GetCIIcon(conclusion string) string {
	switch conclusion {
	case iconSuccess:
		return ""
	case iconFailure:
		return ""
	case iconSkipped:
		return ""
	case iconCancelled:
		return ""
	case iconPending, "":
		return ""
	default:
		return ""
	}
}

// GetUIIcon returns the UI icon for the given identifier.
func (p *NerdFontV3Provider) GetUIIcon(icon UIIcon) string {
	return nerdFontUIIcon(icon, p.GetPRIcon(), p.GetIssueIcon())
}

// NerdFontV2Provider implements IconProvider for Nerd Font v2.
type NerdFontV2Provider struct{}

// GetFileIcon returns the file icon for the given name and type.
func (p *NerdFontV2Provider) GetFileIcon(name string, isDir bool) string {
	if name == "" {
		return ""
	}
	return lazyGitFileIcon(name, isDir, 2)
}

// GetPRIcon returns the PR icon.
func (p *NerdFontV2Provider) GetPRIcon() string { return "" }

// GetIssueIcon returns the issue icon.
func (p *NerdFontV2Provider) GetIssueIcon() string { return "" }

// GetCIIcon returns the CI status icon for the given conclusion.
func (p *NerdFontV2Provider) GetCIIcon(conclusion string) string {
	switch conclusion {
	case iconSuccess:
		return ""
	case iconFailure:
		return ""
	case iconSkipped:
		return ""
	case iconCancelled:
		return ""
	case iconPending, "":
		return ""
	default:
		return ""
	}
}

// GetUIIcon returns the UI icon for the given identifier.
func (p *NerdFontV2Provider) GetUIIcon(icon UIIcon) string {
	return nerdFontUIIcon(icon, p.GetPRIcon(), p.GetIssueIcon())
}

// EmojiProvider implements IconProvider using emojis.
type EmojiProvider struct{}

// GetFileIcon returns the file icon for the given name and type.
func (p *EmojiProvider) GetFileIcon(name string, isDir bool) string {
	if isDir {
		return "📁"
	}
	return "📄"
}

// GetPRIcon returns the PR icon.
func (p *EmojiProvider) GetPRIcon() string { return "🔀" }

// GetIssueIcon returns the issue icon.
func (p *EmojiProvider) GetIssueIcon() string { return "🐛" }

// GetCIIcon returns the CI status icon for the given conclusion.
func (p *EmojiProvider) GetCIIcon(conclusion string) string {
	switch conclusion {
	case iconSuccess:
		return "✅"
	case iconFailure:
		return "❌"
	case iconSkipped:
		return "⏭️"
	case iconCancelled:
		return "🚫"
	case iconPending, "":
		return "⏳"
	default:
		return "❓"
	}
}

// GetUIIcon returns the UI icon for the given identifier.
func (p *EmojiProvider) GetUIIcon(icon UIIcon) string {
	return emojiUIIcon(icon, p.GetPRIcon(), p.GetIssueIcon())
}

// UnicodeProvider implements IconProvider using Unicode characters.
type UnicodeProvider struct{}

// GetFileIcon returns the file icon for the given name and type.
func (p *UnicodeProvider) GetFileIcon(name string, isDir bool) string {
	if isDir {
		return "/"
	}
	return ""
}

// GetPRIcon returns the PR icon.
func (p *UnicodeProvider) GetPRIcon() string { return "[PR]" }

// GetIssueIcon returns the issue icon.
func (p *UnicodeProvider) GetIssueIcon() string { return "[I]" }

// GetCIIcon returns the CI status icon for the given conclusion.
func (p *UnicodeProvider) GetCIIcon(conclusion string) string {
	switch conclusion {
	case iconSuccess:
		return "✓"
	case iconFailure:
		return "✗"
	case iconSkipped:
		return "-"
	case iconCancelled:
		return "⊘"
	case iconPending, "":
		return "●"
	default:
		return "?"
	}
}

// GetUIIcon returns the UI icon for the given identifier.
func (p *UnicodeProvider) GetUIIcon(icon UIIcon) string {
	return unicodeUIIcon(icon, p.GetPRIcon(), p.GetIssueIcon())
}

func nerdFontUIIcon(icon UIIcon, prIcon, issueIcon string) string {
	switch icon {
	case UIIconHelpTitle:
		return nerdFontUIIconHelpTitle
	case UIIconNavigation:
		return nerdFontUIIconNavigation
	case UIIconStatusPane:
		return nerdFontUIIconStatusPane
	case UIIconLogPane:
		return nerdFontUIIconLogPane
	case UIIconCommitTree:
		return nerdFontUIIconCommitTree
	case UIIconWorktreeActions:
		return nerdFontUIIconWorktreeActions
	case UIIconBranchNaming:
		return nerdFontUIIconBranchNaming
	case UIIconViewingTools:
		return nerdFontUIIconViewingTools
	case UIIconRepoOps:
		return nerdFontUIIconRepoOps
	case UIIconBackgroundRefresh:
		return nerdFontUIIconBackgroundRefresh
	case UIIconFilterSearch:
		return nerdFontUIIconFilterSearch
	case UIIconStatusIndicators:
		return nerdFontUIIconStatusIndicators
	case UIIconHelpNavigation:
		return nerdFontUIIconHelpNavigation
	case UIIconShellCompletion:
		return nerdFontUIIconShellCompletion
	case UIIconConfiguration:
		return nerdFontUIIconConfiguration
	case UIIconIconConfiguration:
		return nerdFontUIIconIconConfiguration
	case UIIconTip:
		return nerdFontUIIconTip
	case UIIconSearch:
		return nerdFontUIIconSearch
	case UIIconFilter:
		return nerdFontUIIconFilter
	case UIIconZoom:
		return nerdFontUIIconZoom
	case UIIconBot:
		return nerdFontUIIconBot
	case UIIconThemeSelect:
		return nerdFontUIIconThemeSelect
	case UIIconPRSelect:
		return prIcon
	case UIIconIssueSelect:
		return issueIcon
	case UIIconWorktreeMain:
		return nerdFontUIIconWorktreeMain
	case UIIconWorktree:
		return nerdFontUIIconWorktree
	case UIIconStatusClean, UIIconSyncClean:
		return nerdFontUIIconStatusClean
	case UIIconStatusDirty:
		return nerdFontUIIconStatusDirty
	case UIIconAhead:
		return nerdFontUIIconAhead
	case UIIconBehind:
		return nerdFontUIIconBehind
	case UIIconArrowLeft:
		return nerdFontUIIconArrowLeft
	case UIIconArrowRight:
		return nerdFontUIIconArrowRight
	default:
		return ""
	}
}

func emojiUIIcon(icon UIIcon, prIcon, issueIcon string) string {
	switch icon {
	case UIIconHelpTitle:
		return "🌲"
	case UIIconNavigation:
		return "🧭"
	case UIIconStatusPane:
		return "📝"
	case UIIconLogPane:
		return "📜"
	case UIIconCommitTree:
		return "📁"
	case UIIconWorktreeActions:
		return "⚡"
	case UIIconBranchNaming:
		return "📝"
	case UIIconViewingTools:
		return "🔍"
	case UIIconRepoOps:
		return "🔄"
	case UIIconBackgroundRefresh:
		return "🕰"
	case UIIconFilterSearch:
		return "🔎"
	case UIIconStatusIndicators:
		return "📊"
	case UIIconHelpNavigation:
		return "❓"
	case UIIconShellCompletion:
		return "🔧"
	case UIIconConfiguration:
		return "⚙️"
	case UIIconIconConfiguration:
		return "🎨"
	case UIIconTip:
		return "💡"
	case UIIconSearch:
		return "🔍"
	case UIIconFilter:
		return "🔍"
	case UIIconZoom:
		return "🔎"
	case UIIconBot:
		return "🤖"
	case UIIconThemeSelect:
		return "🎨"
	case UIIconPRSelect:
		return prIcon
	case UIIconIssueSelect:
		return issueIcon
	case UIIconWorktreeMain:
		return "🌳"
	case UIIconWorktree:
		return "📁"
	case UIIconStatusClean, UIIconSyncClean:
		return "✅"
	case UIIconStatusDirty:
		return "✎"
	case UIIconAhead:
		return "⬆️"
	case UIIconBehind:
		return "⬇️"
	case UIIconArrowLeft:
		return "⬅️"
	case UIIconArrowRight:
		return "➡️"
	default:
		return ""
	}
}

func unicodeUIIcon(icon UIIcon, prIcon, issueIcon string) string {
	switch icon {
	case UIIconHelpTitle:
		return unicodeUIIconHelpTitle
	case UIIconNavigation:
		return unicodeUIIconNavigation
	case UIIconStatusPane:
		return unicodeUIIconStatusPane
	case UIIconLogPane:
		return unicodeUIIconLogPane
	case UIIconCommitTree:
		return unicodeUIIconCommitTree
	case UIIconWorktreeActions:
		return unicodeUIIconWorktreeActions
	case UIIconBranchNaming:
		return unicodeUIIconBranchNaming
	case UIIconViewingTools:
		return unicodeUIIconViewingTools
	case UIIconRepoOps:
		return unicodeUIIconRepoOps
	case UIIconBackgroundRefresh:
		return unicodeUIIconBackgroundRefresh
	case UIIconFilterSearch:
		return unicodeUIIconFilterSearch
	case UIIconStatusIndicators:
		return unicodeUIIconStatusIndicators
	case UIIconHelpNavigation:
		return unicodeUIIconHelpNavigation
	case UIIconShellCompletion:
		return unicodeUIIconShellCompletion
	case UIIconConfiguration:
		return unicodeUIIconConfiguration
	case UIIconIconConfiguration:
		return unicodeUIIconIconConfiguration
	case UIIconTip:
		return unicodeUIIconTip
	case UIIconSearch:
		return unicodeUIIconSearch
	case UIIconFilter:
		return unicodeUIIconFilter
	case UIIconZoom:
		return unicodeUIIconZoom
	case UIIconBot:
		return unicodeUIIconBot
	case UIIconThemeSelect:
		return unicodeUIIconThemeSelect
	case UIIconPRSelect:
		return prIcon
	case UIIconIssueSelect:
		return issueIcon
	case UIIconWorktreeMain:
		return unicodeUIIconWorktreeMain
	case UIIconWorktree:
		return unicodeUIIconWorktree
	case UIIconStatusClean, UIIconSyncClean:
		return unicodeUIIconStatusClean
	case UIIconStatusDirty:
		return unicodeUIIconStatusDirty
	case UIIconAhead:
		return unicodeUIIconAhead
	case UIIconBehind:
		return unicodeUIIconBehind
	case UIIconArrowLeft:
		return unicodeUIIconArrowLeft
	case UIIconArrowRight:
		return unicodeUIIconArrowRight
	default:
		return ""
	}
}

// Wrappers for backward compatibility and ease of use

func deviconForName(name string, isDir bool) string {
	return currentIconProvider.GetFileIcon(name, isDir)
}

func ciIconForConclusion(conclusion string) string {
	return currentIconProvider.GetCIIcon(conclusion)
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
			return uiIcon(UIIconStatusClean)
		}
		return uiIcon(UIIconStatusDirty)
	}
	if clean {
		return "C"
	}
	return "D"
}

func syncIndicator(showIcons bool) string {
	if showIcons {
		return uiIcon(UIIconSyncClean)
	}
	return "OK"
}

func aheadIndicator(showIcons bool) string {
	if showIcons {
		return uiIcon(UIIconAhead)
	}
	return "A"
}

func behindIndicator(showIcons bool) string {
	if showIcons {
		return uiIcon(UIIconBehind)
	}
	return "B"
}

func arrowUp(showIcons bool) string {
	if !showIcons {
		return "Up"
	}
	return uiIcon(UIIconAhead)
}

func arrowDown(showIcons bool) string {
	if !showIcons {
		return "Down"
	}
	return uiIcon(UIIconBehind)
}

func arrowLeft(showIcons bool) string {
	if !showIcons {
		return "Left"
	}
	return uiIcon(UIIconArrowLeft)
}

func arrowRight(showIcons bool) string {
	if !showIcons {
		return "Right"
	}
	return uiIcon(UIIconArrowRight)
}

func arrowPair(showIcons bool) string {
	if !showIcons {
		return "Up/Down"
	}
	return arrowUp(true) + arrowDown(true)
}

func iconWithSpace(icon string) string {
	if icon == "" {
		return ""
	}
	return icon + " "
}
