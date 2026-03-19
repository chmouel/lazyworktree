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
	UIIconWorktreeDescription
	UIIconWorktreeColour
	UIIconWorktreeNotes
	UIIconWorktreeIcon
	UIIconWorktreeTags
	UIIconStatusClean
	UIIconStatusDirty
	UIIconSyncClean
	UIIconAhead
	UIIconUnmerged
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

var nerdFontGlyphs = map[UIIcon]string{
	UIIconHelpTitle:           "",
	UIIconNavigation:          "",
	UIIconStatusPane:          "",
	UIIconLogPane:             "",
	UIIconCommitTree:          "",
	UIIconWorktreeActions:     "",
	UIIconBranchNaming:        "",
	UIIconViewingTools:        "",
	UIIconRepoOps:             "",
	UIIconBackgroundRefresh:   "",
	UIIconFilterSearch:        "",
	UIIconStatusIndicators:    "",
	UIIconHelpNavigation:      "",
	UIIconShellCompletion:     "",
	UIIconConfiguration:       "",
	UIIconIconConfiguration:   "",
	UIIconTip:                 "",
	UIIconSearch:              "",
	UIIconFilter:              "",
	UIIconZoom:                "",
	UIIconBot:                 "",
	UIIconThemeSelect:         "",
	UIIconCICheck:             "",
	UIIconWorktreeMain:        "",
	UIIconWorktree:            "",
	UIIconWorktreeDescription: "󰀬",
	UIIconWorktreeColour:      "󰸌",
	UIIconWorktreeNotes:       "󱞁",
	UIIconWorktreeIcon:        "󰥶",
	UIIconWorktreeTags:        "󰓹",
	UIIconStatusClean:         "-",
	UIIconSyncClean:           "-",
	UIIconStatusDirty:         "",
	UIIconAhead:               "↑",
	UIIconUnmerged:            "★",
	UIIconBehind:              "↓",
	UIIconArrowLeft:           "←",
	UIIconArrowRight:          "→",
	UIIconDisclosureOpen:      "▼",
	UIIconDisclosureClosed:    "▶",
	UIIconSpinnerFilled:       "●",
	UIIconSpinnerEmpty:        "◌",
	UIIconPRStateOpen:         "●",
	UIIconPRStateMerged:       "◆",
	UIIconPRStateClosed:       "✕",
	UIIconPRStateUnknown:      "?",
}

var textGlyphs = map[UIIcon]string{
	UIIconHelpTitle:           "*",
	UIIconNavigation:          ">",
	UIIconStatusPane:          "S",
	UIIconLogPane:             "L",
	UIIconCommitTree:          "T",
	UIIconWorktreeActions:     "W",
	UIIconBranchNaming:        "B",
	UIIconSearch:              "/",
	UIIconViewingTools:        "/",
	UIIconRepoOps:             "R",
	UIIconBackgroundRefresh:   "H",
	UIIconFilterSearch:        "/",
	UIIconStatusIndicators:    "I",
	UIIconHelpNavigation:      "?",
	UIIconShellCompletion:     "C",
	UIIconConfiguration:       "C",
	UIIconIconConfiguration:   "I",
	UIIconTip:                 "!",
	UIIconFilter:              "F",
	UIIconZoom:                "Z",
	UIIconBot:                 "B",
	UIIconThemeSelect:         "T",
	UIIconCICheck:             "C",
	UIIconWorktreeMain:        "M",
	UIIconWorktree:            "W",
	UIIconWorktreeDescription: "D",
	UIIconWorktreeColour:      "C",
	UIIconWorktreeNotes:       "N",
	UIIconWorktreeIcon:        "I",
	UIIconWorktreeTags:        "T",
	UIIconStatusClean:         "C",
	UIIconSyncClean:           "C",
	UIIconStatusDirty:         "D",
	UIIconAhead:               "↑",
	UIIconUnmerged:            "★",
	UIIconBehind:              "↓",
	UIIconArrowLeft:           "←",
	UIIconArrowRight:          "→",
	UIIconDisclosureOpen:      "▼",
	UIIconDisclosureClosed:    "▶",
	UIIconSpinnerFilled:       "●",
	UIIconSpinnerEmpty:        "◌",
	UIIconPRStateOpen:         "●",
	UIIconPRStateMerged:       "◆",
	UIIconPRStateClosed:       "✕",
	UIIconPRStateUnknown:      "?",
}

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
func (p *TextProvider) GetPRIcon() string { return "" }

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
	case UIIconPRSelect:
		return prIcon
	case UIIconIssueSelect:
		return issueIcon
	default:
		return nerdFontGlyphs[icon]
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
	case UIIconWorktreeDescription:
		return "🏷️"
	case UIIconWorktreeColour:
		return "🎨"
	case UIIconWorktreeNotes:
		return "📝"
	case UIIconWorktreeIcon:
		return "✨"
	case UIIconWorktreeTags:
		return "🏷️"
	case UIIconStatusClean, UIIconSyncClean:
		return "✅"
	case UIIconStatusDirty:
		return "📝"
	case UIIconAhead:
		return "⏫"
	case UIIconUnmerged:
		return "⭐"
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
	case UIIconPRSelect:
		return prIcon
	case UIIconIssueSelect:
		return issueIcon
	default:
		return textGlyphs[icon]
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

func unmergedIndicator(showIcons bool) string {
	if showIcons {
		return uiIcon(UIIconUnmerged)
	}
	return "★"
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
func combinedStatusIndicator(dirty, hasUpstream bool, ahead, behind, unpushed int, showIcons bool) string {
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
		result = "  " + syncStr
	default:
		return "  -"
	}

	return result
}
