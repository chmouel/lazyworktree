package app

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// renderHeader renders the application header.
func (m *Model) renderHeader(layout layoutDims) string {
	showIcons := m.config.IconsEnabled()

	appText := "Lazyworktree"
	if showIcons {
		appText = " " + appText // Tree icon
	}
	appStyle := m.renderStyles.headerAppStyle

	repoKey := strings.TrimSpace(m.repoKey)
	repoStr := ""
	if repoKey != "" && repoKey != "unknown" && !strings.HasPrefix(repoKey, "local-") {
		repoText := repoKey
		if showIcons {
			repoText = " " + repoText // Repo icon
		}
		repoStyle := m.renderStyles.headerRepoStyle
		rendered := repoStyle.Render(repoText)
		if url := m.getRepoWebURL(); url != "" {
			rendered = osc8Hyperlink(rendered, url)
		}
		repoStr = "   " + rendered
	}

	headerStyle := m.renderStyles.headerContainerStyle.Width(layout.width)

	content := appStyle.Render(appText) + repoStr
	return headerStyle.Render(content)
}

// renderFilter renders the filter input bar.
func (m *Model) renderFilter(layout layoutDims) string {
	labelStyle := m.renderStyles.filterLabelStyle
	filterStyle := m.renderStyles.filterContainerStyle
	line := fmt.Sprintf("%s %s", labelStyle.Render(m.inputLabel()), m.state.ui.filterInput.View())
	return filterStyle.Width(layout.width).Render(line)
}

// renderFooter renders the application footer with context-aware hints.
func (m *Model) renderFooter(layout layoutDims) string {
	footerStyle := m.renderStyles.footerStyle
	sep := " " + m.renderStyles.footerSepStyle.Render("·") + " "

	// Context-aware hints based on focused pane
	var groups [][]string

	hasGitStatus := m.hasGitStatus()
	hasNotes := m.hasNoteForSelectedWorktree()
	hasAgentSessions := m.hasAgentSessionsForSelectedWorktree()
	paneHint := "1-4"
	switch {
	case hasAgentSessions && hasNotes:
		paneHint = "1-6"
	case hasAgentSessions && hasGitStatus:
		paneHint = "1-4,6"
	case hasAgentSessions:
		paneHint = "1-2,4,6"
	case hasNotes && hasGitStatus:
		paneHint = "1-5"
	case hasNotes:
		paneHint = "1-2,4-5"
	case !hasGitStatus:
		paneHint = "1-2,4"
	}

	switch m.state.view.FocusedPane {
	case paneNotes: // Notes pane
		groups = [][]string{
			{m.renderKeyHint("j/k", "Scroll"), m.renderKeyHint("i", "Edit Note")},
			{m.renderKeyHint("Tab", "Switch Pane")},
			{m.renderKeyHint("q", "Quit"), m.renderKeyHint("?", "Help")},
		}

	case paneAgentSessions:
		groups = [][]string{
			{m.renderKeyHint("j/k", "Navigate"), m.renderKeyHint("Ctrl+D/U", "Page"), m.renderKeyHint("A", "Show All")},
			{m.renderKeyHint("Tab", "Switch Pane"), m.renderKeyHint("6", "Focus Pane")},
			{m.renderKeyHint("q", "Quit"), m.renderKeyHint("?", "Help")},
		}

	case paneCommit: // Commit pane
		if len(m.state.data.logEntries) > 0 {
			groups = [][]string{
				{m.renderKeyHint("Enter", "View Commit"), m.renderKeyHint("C", "Cherry-pick"), m.renderKeyHint("j/k", "Navigate")},
				{m.renderKeyHint("f", "Filter"), m.renderKeyHint("/", "Search"), m.renderKeyHint("r", "Refresh")},
				{m.renderKeyHint("Tab", "Switch Pane"), m.renderKeyHint("q", "Quit"), m.renderKeyHint("?", "Help")},
			}
		} else {
			groups = [][]string{
				{m.renderKeyHint("f", "Filter"), m.renderKeyHint("/", "Search")},
				{m.renderKeyHint("Tab", "Switch Pane"), m.renderKeyHint("q", "Quit"), m.renderKeyHint("?", "Help")},
			}
		}

	case paneGitStatus: // Git Status pane
		actionGroup := []string{m.renderKeyHint("j/k", "Scroll")}
		if len(m.state.data.statusFiles) > 0 {
			actionGroup = append(
				actionGroup,
				m.renderKeyHint("Enter", "Show Diff"),
				m.renderKeyHint("e", "Edit File"),
				m.renderKeyHint("s", "Stage"),
			)
		}
		groups = [][]string{
			actionGroup,
			{m.renderKeyHint("f", "Filter"), m.renderKeyHint("/", "Search"), m.renderKeyHint("r", "Refresh")},
			{m.renderKeyHint("Tab", "Switch Pane"), m.renderKeyHint("q", "Quit"), m.renderKeyHint("?", "Help")},
		}

	case paneInfo: // Info pane (info + CI)
		groups = [][]string{
			{m.renderKeyHint("j/k", "Scroll"), m.renderKeyHint("n/p", "CI Checks"), m.renderKeyHint("Enter", "Open URL"), m.renderKeyHint("Ctrl+v", "CI Logs")},
			{m.renderKeyHint("Tab", "Switch Pane"), m.renderKeyHint("r", "Refresh")},
			{m.renderKeyHint("q", "Quit"), m.renderKeyHint("?", "Help")},
		}

	default: // Worktree table (pane 0)
		navGroup := []string{
			m.renderKeyHint(paneHint, "Pane"),
			m.renderKeyHint("c", "Create"),
			m.renderKeyHint("f", "Filter"),
		}
		actionGroup := []string{
			m.renderKeyHint("d", "Diff"),
			m.renderKeyHint("D", "Delete"),
			m.renderKeyHint("S", "Sync"),
		}
		if m.state.data.selectedIndex >= 0 && m.state.data.selectedIndex < len(m.state.data.filteredWts) {
			wt := m.state.data.filteredWts[m.state.data.selectedIndex]
			openLabel := "Open Branch"
			if wt.IsMain && (wt.PR == nil || wt.PR.State == prStateMerged || wt.PR.State == prStateClosed) {
				openLabel = "Open Repo"
			} else if wt.PR != nil && wt.PR.URL != "" {
				openLabel = "Open PR"
			}
			actionGroup = append(actionGroup, m.renderKeyHint("o", openLabel))
		}
		actionGroup = append(actionGroup, m.customFooterHints()...)
		globalGroup := []string{
			m.renderKeyHint("q", "Quit"),
			m.renderKeyHint("?", "Help"),
			m.renderKeyHint("ctrl+p", "Palette"),
		}
		groups = [][]string{navGroup, actionGroup, globalGroup}
	}

	// Join groups with separator
	groupStrs := make([]string, 0, len(groups))
	for _, g := range groups {
		groupStrs = append(groupStrs, strings.Join(g, "  "))
	}
	footerContent := strings.Join(groupStrs, sep)

	if !m.loading.active {
		return footerStyle.Width(layout.width).Render(footerContent)
	}
	spinnerView := m.state.ui.spinner.View()
	gap := "  "
	available := max(layout.width-lipgloss.Width(spinnerView)-lipgloss.Width(gap), 0)
	footer := footerStyle.Width(available).Render(footerContent)
	return lipgloss.JoinHorizontal(lipgloss.Left, footer, gap, spinnerView)
}

// renderKeyHint renders a single key hint with enhanced styling.
func (m *Model) renderKeyHint(key, label string) string {
	keyStyle := m.renderStyles.keyHintKeyStyle
	labelStyle := m.renderStyles.keyHintLabelStyle
	return fmt.Sprintf("%s %s", keyStyle.Render(key), labelStyle.Render(label))
}

// renderPaneBlock renders a pane block with the title embedded in the top border.
func (m *Model) renderPaneBlock(index int, title string, focused bool, width, height int, innerContent string) string {
	border := lipgloss.RoundedBorder()
	borderColor := m.theme.BorderDim
	if focused {
		borderColor = m.theme.Accent
	}

	contentStyle := lipgloss.NewStyle().
		Border(border).
		BorderTop(false).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(width).
		Height(height - 1).
		MaxHeight(height - 1)

	styledContent := contentStyle.Render(innerContent)

	showIcons := m.config.IconsEnabled()
	numStr := fmt.Sprintf("[%d]", index)
	if showIcons {
		numStr = fmt.Sprintf("(%d)", index)
	}

	bubbleStyle := m.renderStyles.paneBubbleDimStyle
	edgeStyle := m.renderStyles.paneEdgeDimStyle
	if focused {
		bubbleStyle = m.renderStyles.paneBubbleFocusedStyle
		edgeStyle = m.renderStyles.paneEdgeFocusedStyle
	}
	leftEdge := edgeStyle.Render("")
	rightEdge := edgeStyle.Render("")
	titleText := fmt.Sprintf(" %s %s ", numStr, title)

	filterIndicator := ""
	paneIdx := index - 1
	if !m.state.view.ShowingFilter && !m.state.view.ShowingSearch && m.hasActiveFilterForPane(paneIdx) {
		filterIndicator = fmt.Sprintf(" %s%s  %s %s",
			iconPrefix(UIIconFilter, showIcons),
			m.renderStyles.paneFilterTextStyle.Render("Filtered"),
			m.renderStyles.paneIndicatorKeyStyle.Render("Esc"),
			m.renderStyles.paneMutedTextStyle.Render("Clear"))
	}

	zoomIndicator := ""
	if m.state.view.ZoomedPane == paneIdx {
		zoomIndicator = fmt.Sprintf(" %s%s  %s %s",
			iconPrefix(UIIconZoom, showIcons),
			m.renderStyles.paneZoomTextStyle.Render("Zoomed"),
			m.renderStyles.paneIndicatorKeyStyle.Render("="),
			m.renderStyles.paneMutedTextStyle.Render("Unzoom"))
	}

	borderStyle := edgeStyle
	borderLine := borderStyle.Render(border.Top)

	styledTitleBlock := borderLine + leftEdge + bubbleStyle.Render(titleText) + rightEdge + borderLine

	topLeft := border.TopLeft
	topRight := border.TopRight
	topLine := border.Top

	usedWidth := lipgloss.Width(topLeft) + lipgloss.Width(styledTitleBlock) + lipgloss.Width(filterIndicator) + lipgloss.Width(zoomIndicator) + lipgloss.Width(topRight)
	remaining := max(width-usedWidth, 0)

	styledTopLeft := borderStyle.Render(topLeft)
	styledTopRight := borderStyle.Render(topRight)
	styledRemaining := borderStyle.Render(strings.Repeat(topLine, remaining))

	finalTopBorder := styledTopLeft + styledTitleBlock + filterIndicator + zoomIndicator + styledRemaining + styledTopRight

	return lipgloss.JoinVertical(lipgloss.Left, finalTopBorder, styledContent)
}

// renderCIStatusChip renders the aggregate CI state as a lightweight inline chip.
func (m *Model) renderCIStatusChip(conclusion string, showIcons bool) string {
	statusConclusion := conclusion
	if statusConclusion == "" {
		statusConclusion = "pending"
	}
	statusStyle := m.ciIconStyle(statusConclusion).Bold(true)
	icon := getCIStatusIcon(statusConclusion, false, showIcons)
	label := ciConclusionDisplayLabel(statusConclusion)
	if icon == "" {
		return statusStyle.Render(label)
	}
	return fmt.Sprintf("%s %s", statusStyle.Render(icon), statusStyle.Render(label))
}

// tagPillColor returns a deterministic theme colour for a tag string.
func (m *Model) tagPillColor(tag string) color.Color {
	palette := []color.Color{
		m.theme.Accent,
		m.theme.SuccessFg,
		m.theme.WarnFg,
		m.theme.ErrorFg,
		m.theme.Cyan,
		m.theme.TextFg,
	}
	// Simple hash: sum of bytes mod palette length.
	var h uint32
	for i := range len(tag) {
		h += uint32(tag[i])
	}
	return palette[int(h)%len(palette)]
}

// renderTagPill renders a single tag as a bracketed label with foreground colour.
func (m *Model) renderTagPill(tag string) string {
	c := m.tagPillColor(tag)
	style := lipgloss.NewStyle().Foreground(c).Bold(true)
	return style.Render("«" + tag + "»")
}

// renderPlainTagPill renders a single tag as plain badge text so table
// selection styling can remain readable.
func (m *Model) renderPlainTagPill(tag string) string {
	return "«" + tag + "»"
}

func joinTagPills(tags []string, render func(string) string) string {
	if len(tags) == 0 {
		return ""
	}
	pills := make([]string, len(tags))
	for i, tag := range tags {
		pills[i] = render(tag)
	}
	return strings.Join(pills, " ")
}

// renderTagPills renders all tags as space-separated coloured pill badges.
func (m *Model) renderTagPills(tags []string) string {
	return joinTagPills(tags, m.renderTagPill)
}

// renderPlainTagPills renders tags without inline ANSI styling.
func (m *Model) renderPlainTagPills(tags []string) string {
	return joinTagPills(tags, m.renderPlainTagPill)
}

// ciConclusionDisplayLabel maps a CI conclusion to a compact display label.
func ciConclusionDisplayLabel(conclusion string) string {
	switch conclusion {
	case "success":
		return "Passed"
	case "failure":
		return "Failed"
	case "pending", "":
		return "Pending"
	case "skipped":
		return "Skipped"
	case "cancelled":
		return "Cancelled"
	default:
		return strings.ToUpper(conclusion[:1]) + strings.ToLower(conclusion[1:])
	}
}

// ciIconStyle returns a foreground-only style for a CI conclusion icon.
func (m *Model) ciIconStyle(conclusion string) lipgloss.Style {
	switch conclusion {
	case "success":
		return m.renderStyles.ciIconSuccessStyle
	case "failure":
		return m.renderStyles.ciIconFailureStyle
	case "pending", "":
		return m.renderStyles.ciIconPendingStyle
	default: // skipped, cancelled, etc.
		return m.renderStyles.ciIconDefaultStyle
	}
}

// prStateColors returns (background, foreground) theme colours for a PR state.
func (m *Model) prStateColors(state string) (color.Color, color.Color) {
	switch state {
	case prStateOpen:
		return m.theme.SuccessFg, m.theme.AccentFg
	case prStateMerged:
		return m.theme.Accent, m.theme.AccentFg
	case prStateClosed:
		return m.theme.ErrorFg, m.theme.AccentFg
	default:
		return m.theme.BorderDim, m.theme.TextFg
	}
}

// prStateCompactStyle returns the foreground-only style used in compact table cells.
func (m *Model) prStateCompactStyle(state string) lipgloss.Style {
	bg, _ := m.prStateColors(state)
	return lipgloss.NewStyle().Foreground(bg)
}

// renderPRStateBadge renders a compact badge for PR/MR state.
func (m *Model) renderPRStateBadge(state string, showNerdFontIcons bool) string {
	label := prStateText(state, showNerdFontIcons)
	if label == "" {
		return ""
	}
	bg, fg := m.prStateColors(state)
	if showNerdFontIcons {
		edgeStyle := lipgloss.NewStyle().Foreground(bg)
		badgeStyle := lipgloss.NewStyle().
			Background(bg).
			Foreground(fg).
			Bold(true)
		return edgeStyle.Render("\ue0b6") + badgeStyle.Render(label) + edgeStyle.Render("\ue0b4")
	}
	return lipgloss.NewStyle().
		Foreground(fg).
		Bold(true).
		Render(label)
}

func prStateText(state string, showNerdFontIcons bool) string {
	var icon, label string
	switch state {
	case prStateOpen:
		icon, label = " ", "Open"
	case prStateMerged:
		icon, label = " ", "Merged"
	case prStateClosed:
		icon, label = " ", "Closed"
	case prStateDraft:
		icon, label = " ", "Draft"
	default:
		return ""
	}
	if showNerdFontIcons {
		return icon + label
	}
	return label
}

var remoteIconTable = []struct {
	domain string
	icon   string
}{
	{"github.com", "\ue709"},
	{"bitbucket.org", "\ue703"},
	{"gitlab.com", "\uf296"},
	{"dev.azure.com", "\U000f0805"},
	{"codeberg.org", "\uf330"},
	{"git.FreeBSD.org", "\uf30c"},
	{"gitlab.archlinux.org", "\uf303"},
	{"gitlab.freedesktop.org", "\uf360"},
	{"gitlab.gnome.org", "\uf361"},
	{"gnu.org", "\ue779"},
	{"invent.kde.org", "\uf373"},
	{"kernel.org", "\uf31a"},
	{"salsa.debian.org", "\uf306"},
	{"sr.ht", "\uf1db"},
}

func prRemoteIcon(url string, showNerdFontIcons bool) string {
	if !showNerdFontIcons {
		return ""
	}
	for _, r := range remoteIconTable {
		if strings.Contains(url, r.domain) {
			return r.icon
		}
	}
	return "\U000f02a2"
}

// basePaneStyle returns the base style for panes.
func (m *Model) basePaneStyle() lipgloss.Style {
	return m.renderStyles.baseBoxStyle
}

// baseInnerBoxStyle returns the base style for inner boxes.
func (m *Model) baseInnerBoxStyle() lipgloss.Style {
	return m.renderStyles.baseBoxStyle
}
