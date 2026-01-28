package app

import "fmt"

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
	UIIconCICheck
	UIIconWorktreeMain
	UIIconWorktree
	UIIconStatusClean
	UIIconStatusDirty
	UIIconSyncClean
	UIIconAhead
	UIIconBehind
	UIIconArrowLeft
	UIIconArrowRight
	UIIconDisclosureOpen
	UIIconDisclosureClosed
	UIIconSpinnerFilled
	UIIconSpinnerEmpty
	UIIconPRStateOpen
	UIIconPRStateMerged
	UIIconPRStateClosed
	UIIconPRStateUnknown
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
	nerdFontUIIconStatusClean       = "-"
	nerdFontUIIconStatusDirty       = ""
	nerdFontUIIconAhead             = "↑"
	nerdFontUIIconBehind            = "↓"
	nerdFontUIIconArrowLeft         = "←"
	nerdFontUIIconArrowRight        = "→"
	nerdFontUIIconDisclosureOpen    = "▼"
	nerdFontUIIconDisclosureClosed  = "▶"
	nerdFontUIIconSpinnerFilled     = "●"
	nerdFontUIIconSpinnerEmpty      = "◌"
	nerdFontUIIconPRStateOpen       = nerdFontUIIconSpinnerFilled
	nerdFontUIIconPRStateMerged     = "◆"
	nerdFontUIIconPRStateClosed     = "✕"
	nerdFontUIIconPRStateUnknown    = "?"
)

const (
	textUIIconHelpTitle         = "*"
	textUIIconNavigation        = ">"
	textUIIconStatusPane        = "S"
	textUIIconLogPane           = "L"
	textUIIconCommitTree        = "T"
	textUIIconWorktreeActions   = "W"
	textUIIconBranchNaming      = "B"
	textUIIconSearch            = "/"
	textUIIconViewingTools      = textUIIconSearch
	textUIIconRepoOps           = "R"
	textUIIconBackgroundRefresh = "H"
	textUIIconFilterSearch      = textUIIconSearch
	textUIIconStatusIndicators  = "I"
	textUIIconHelpNavigation    = "?"
	textUIIconShellCompletion   = "C"
	textUIIconConfiguration     = textUIIconShellCompletion
	textUIIconIconConfiguration = textUIIconStatusIndicators
	textUIIconTip               = "!"
	textUIIconFilter            = "F"
	textUIIconZoom              = "Z"
	textUIIconBot               = textUIIconBranchNaming
	textUIIconThemeSelect       = "T"
	textUIIconWorktreeMain      = "M"
	textUIIconWorktree          = textUIIconWorktreeActions
	textUIIconStatusClean       = textUIIconShellCompletion
	textUIIconStatusDirty       = "D"
	textUIIconAhead             = "↑"
	textUIIconBehind            = "↓"
	textUIIconArrowLeft         = "←"
	textUIIconArrowRight        = "→"
	textUIIconDisclosureOpen    = "▼"
	textUIIconDisclosureClosed  = "▶"
	textUIIconSpinnerFilled     = "●"
	textUIIconSpinnerEmpty      = "◌"
	textUIIconPRStateOpen       = textUIIconSpinnerFilled
	textUIIconPRStateMerged     = "◆"
	textUIIconPRStateClosed     = "✕"
	textUIIconPRStateUnknown    = "?"
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

// TextProvider implements IconProvider using simple Unicode-safe characters.
type TextProvider struct{}

// GetFileIcon returns the file icon for the given name and type.
func (p *TextProvider) GetFileIcon(name string, isDir bool) string {
	if name == "" {
		return ""
	}
	if isDir {
		return "/"
	}
	return ""
}

// GetPRIcon returns the PR icon.
func (p *TextProvider) GetPRIcon() string { return "[PR]" }

// GetIssueIcon returns the issue icon.
func (p *TextProvider) GetIssueIcon() string { return "[I]" }

// GetCIIcon returns the CI status icon for the given conclusion.
func (p *TextProvider) GetCIIcon(conclusion string) string {
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
func (p *TextProvider) GetUIIcon(icon UIIcon) string {
	return textUIIcon(icon, p.GetPRIcon(), p.GetIssueIcon())
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
	case UIIconCICheck:
		return "" // CI/workflow icon
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
	case UIIconDisclosureOpen:
		return nerdFontUIIconDisclosureOpen
	case UIIconDisclosureClosed:
		return nerdFontUIIconDisclosureClosed
	case UIIconSpinnerFilled:
		return nerdFontUIIconSpinnerFilled
	case UIIconSpinnerEmpty:
		return nerdFontUIIconSpinnerEmpty
	case UIIconPRStateOpen:
		return nerdFontUIIconPRStateOpen
	case UIIconPRStateMerged:
		return nerdFontUIIconPRStateMerged
	case UIIconPRStateClosed:
		return nerdFontUIIconPRStateClosed
	case UIIconPRStateUnknown:
		return nerdFontUIIconPRStateUnknown
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
	case UIIconCICheck:
		return "⚙️" // CI/workflow icon
	case UIIconWorktreeMain:
		return "🌳"
	case UIIconWorktree:
		return "📁"
	case UIIconStatusClean, UIIconSyncClean:
		return "✅"
	case UIIconStatusDirty:
		return "📝"
	case UIIconAhead:
		return "⏫"
	case UIIconBehind:
		return "⏬"
	case UIIconArrowLeft:
		return "⬅️"
	case UIIconArrowRight:
		return "➡️"
	case UIIconDisclosureOpen:
		return "▼"
	case UIIconDisclosureClosed:
		return "▶"
	case UIIconSpinnerFilled:
		return "●"
	case UIIconSpinnerEmpty:
		return "◌"
	case UIIconPRStateOpen:
		return "🟢"
	case UIIconPRStateMerged:
		return "✅"
	case UIIconPRStateClosed:
		return "❌"
	case UIIconPRStateUnknown:
		return "❓"
	default:
		return ""
	}
}

func textUIIcon(icon UIIcon, prIcon, issueIcon string) string {
	switch icon {
	case UIIconHelpTitle:
		return textUIIconHelpTitle
	case UIIconNavigation:
		return textUIIconNavigation
	case UIIconStatusPane:
		return textUIIconStatusPane
	case UIIconLogPane:
		return textUIIconLogPane
	case UIIconCommitTree:
		return textUIIconCommitTree
	case UIIconWorktreeActions:
		return textUIIconWorktreeActions
	case UIIconBranchNaming:
		return textUIIconBranchNaming
	case UIIconViewingTools:
		return textUIIconViewingTools
	case UIIconRepoOps:
		return textUIIconRepoOps
	case UIIconBackgroundRefresh:
		return textUIIconBackgroundRefresh
	case UIIconFilterSearch:
		return textUIIconFilterSearch
	case UIIconStatusIndicators:
		return textUIIconStatusIndicators
	case UIIconHelpNavigation:
		return textUIIconHelpNavigation
	case UIIconShellCompletion:
		return textUIIconShellCompletion
	case UIIconConfiguration:
		return textUIIconConfiguration
	case UIIconIconConfiguration:
		return textUIIconIconConfiguration
	case UIIconTip:
		return textUIIconTip
	case UIIconSearch:
		return textUIIconSearch
	case UIIconFilter:
		return textUIIconFilter
	case UIIconZoom:
		return textUIIconZoom
	case UIIconBot:
		return textUIIconBot
	case UIIconThemeSelect:
		return textUIIconThemeSelect
	case UIIconPRSelect:
		return prIcon
	case UIIconIssueSelect:
		return issueIcon
	case UIIconCICheck:
		return "C" // CI/workflow icon
	case UIIconWorktreeMain:
		return textUIIconWorktreeMain
	case UIIconWorktree:
		return textUIIconWorktree
	case UIIconStatusClean, UIIconSyncClean:
		return textUIIconStatusClean
	case UIIconStatusDirty:
		return textUIIconStatusDirty
	case UIIconAhead:
		return textUIIconAhead
	case UIIconBehind:
		return textUIIconBehind
	case UIIconArrowLeft:
		return textUIIconArrowLeft
	case UIIconArrowRight:
		return textUIIconArrowRight
	case UIIconDisclosureOpen:
		return textUIIconDisclosureOpen
	case UIIconDisclosureClosed:
		return textUIIconDisclosureClosed
	case UIIconSpinnerFilled:
		return textUIIconSpinnerFilled
	case UIIconSpinnerEmpty:
		return textUIIconSpinnerEmpty
	case UIIconPRStateOpen:
		return textUIIconPRStateOpen
	case UIIconPRStateMerged:
		return textUIIconPRStateMerged
	case UIIconPRStateClosed:
		return textUIIconPRStateClosed
	case UIIconPRStateUnknown:
		return textUIIconPRStateUnknown
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
		return " "
	}
	return "~"
}

func syncIndicator(showIcons bool) string {
	if showIcons {
		return uiIcon(UIIconSyncClean)
	}
	return "-"
}

func aheadIndicator(showIcons bool) string {
	if showIcons {
		return uiIcon(UIIconAhead)
	}
	return "↑"
}

func behindIndicator(showIcons bool) string {
	if showIcons {
		return uiIcon(UIIconBehind)
	}
	return "↓"
}

func disclosureIndicator(collapsed, showIcons bool) string {
	if !showIcons {
		if collapsed {
			return ">"
		}
		return "v"
	}
	if collapsed {
		return uiIcon(UIIconDisclosureClosed)
	}
	return uiIcon(UIIconDisclosureOpen)
}

func spinnerFrameSet(showIcons bool) []string {
	if !showIcons {
		return []string{"...", ".. ", ".  "}
	}
	filled := uiIcon(UIIconSpinnerFilled)
	empty := uiIcon(UIIconSpinnerEmpty)
	if filled == "" || empty == "" {
		return []string{"...", ".. ", ".  "}
	}
	return []string{
		fmt.Sprintf("%s %s %s", filled, filled, empty),
		fmt.Sprintf("%s %s %s", filled, empty, filled),
		fmt.Sprintf("%s %s %s", empty, filled, filled),
	}
}

func prStateIndicator(state string, showIcons bool) string {
	if !showIcons {
		switch state {
		case "OPEN":
			return "O"
		case "MERGED":
			return "M"
		case "CLOSED":
			return "C"
		default:
			return "?"
		}
	}
	switch state {
	case "OPEN":
		return uiIcon(UIIconPRStateOpen)
	case "MERGED":
		return uiIcon(UIIconPRStateMerged)
	case "CLOSED":
		return uiIcon(UIIconPRStateClosed)
	default:
		return uiIcon(UIIconPRStateUnknown)
	}
}

func iconWithSpace(icon string) string {
	if icon == "" {
		return ""
	}
	return icon + " "
}

// combinedStatusIndicator returns a combined dirty + sync status string.
// Returns "-" when clean and synced, otherwise shows dirty indicator and/or ahead/behind counts.
func combinedStatusIndicator(dirty, hasUpstream bool, ahead, behind, unpushed int, showIcons bool, iconSet string) string {
	// Build dirty indicator
	var dirtyStr string
	if dirty {
		if showIcons {
			dirtyStr = uiIcon(UIIconStatusDirty)
		} else {
			dirtyStr = "~"
		}
	}

	// Build sync/ahead/behind indicator
	var syncStr string
	switch {
	case !hasUpstream:
		if unpushed > 0 {
			syncStr = fmt.Sprintf("%s%d", aheadIndicator(showIcons), unpushed)
		}
	case ahead == 0 && behind == 0:
		// Synced with upstream, no indicator needed
	default:
		if behind > 0 {
			syncStr += fmt.Sprintf("%s%d", behindIndicator(showIcons), behind)
		}
		if ahead > 0 {
			syncStr += fmt.Sprintf("%s%d", aheadIndicator(showIcons), ahead)
		}
	}

	// Combine the indicators with a space between dirty and sync if both present
	var result string
	switch {
	case dirtyStr != "" && syncStr != "":
		result = dirtyStr + " " + syncStr
	case dirtyStr != "":
		result = dirtyStr + " -"
	case syncStr != "":
		if iconSet == "emoji" {
			result = "   " + syncStr
		} else {
			result = "  " + syncStr
		}
	default:
		if iconSet == "emoji" {
			return "   -"
		} else {
			return "  -"
		}
	}

	return result
}
