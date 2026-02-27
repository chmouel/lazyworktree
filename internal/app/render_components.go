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
		repoStr = "   " + repoStyle.Render(repoText)
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

	paneHint := "1-4"
	if !m.hasGitStatus() {
		paneHint = "1-2,4"
	}
	if m.hasNoteForSelectedWorktree() {
		if m.hasGitStatus() {
			paneHint = "1-5"
		} else {
			paneHint = "1-2,4-5"
		}
	}

	switch m.state.view.FocusedPane {
	case 4: // Notes pane
		groups = [][]string{
			{m.renderKeyHint("j/k", "Scroll"), m.renderKeyHint("i", "Edit Note")},
			{m.renderKeyHint("Tab", "Switch Pane")},
			{m.renderKeyHint("q", "Quit"), m.renderKeyHint("?", "Help")},
		}

	case 3: // Commit pane
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

	case 2: // Git Status pane
		actionGroup := []string{m.renderKeyHint("j/k", "Scroll")}
		if len(m.state.data.statusFiles) > 0 {
			actionGroup = append(actionGroup,
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

	case 1: // Info pane (info + CI)
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
		// Show "o" key hint only when current worktree has PR info
		if m.state.data.selectedIndex >= 0 && m.state.data.selectedIndex < len(m.state.data.filteredWts) {
			wt := m.state.data.filteredWts[m.state.data.selectedIndex]
			if wt.PR != nil {
				actionGroup = append(actionGroup, m.renderKeyHint("o", "Open PR"))
			}
		}
		actionGroup = append(actionGroup, m.customFooterHints()...)
		globalGroup := []string{
			m.renderKeyHint("y", "Copy"),
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

	if !m.loading {
		return footerStyle.Width(layout.width).Render(footerContent)
	}
	spinnerView := m.state.ui.spinner.View()
	gap := "  "
	available := maxInt(layout.width-lipgloss.Width(spinnerView)-lipgloss.Width(gap), 0)
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

// renderCIStatusPill renders a CI aggregate status as a powerline pill.
func (m *Model) renderCIStatusPill(conclusion string) string {
	label := ciConclusionLabel(conclusion)
	bg, fg := m.ciConclusionColors(conclusion)
	bubbleStyle := lipgloss.NewStyle().Background(bg).Foreground(fg).Bold(true)
	edgeStyle := lipgloss.NewStyle().Foreground(bg)
	leftEdge := edgeStyle.Render("\ue0b6")
	rightEdge := edgeStyle.Render("\ue0b4")
	return leftEdge + bubbleStyle.Render(" "+label+" ") + rightEdge
}

// ciConclusionLabel maps a CI conclusion to an uppercase display label.
func ciConclusionLabel(conclusion string) string {
	switch conclusion {
	case "success":
		return "SUCCESS"
	case "failure":
		return "FAILED"
	case "pending", "":
		return "PENDING"
	case "skipped":
		return "SKIPPED"
	case "cancelled":
		return "CANCELLED"
	default:
		return strings.ToUpper(conclusion)
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

// ciConclusionColors returns (background, foreground) theme colours for a CI conclusion.
func (m *Model) ciConclusionColors(conclusion string) (color.Color, color.Color) {
	switch conclusion {
	case "success":
		return m.theme.SuccessFg, m.theme.AccentFg
	case "failure":
		return m.theme.ErrorFg, m.theme.AccentFg
	case "pending", "":
		return m.theme.WarnFg, m.theme.AccentFg
	default:
		return m.theme.BorderDim, m.theme.TextFg
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

// renderPRStatePill renders a PR state as a powerline pill.
func (m *Model) renderPRStatePill(state string) string {
	bg, fg := m.prStateColors(state)
	bubbleStyle := lipgloss.NewStyle().Background(bg).Foreground(fg).Bold(true)
	edgeStyle := lipgloss.NewStyle().Foreground(bg)
	leftEdge := edgeStyle.Render("\ue0b6")
	rightEdge := edgeStyle.Render("\ue0b4")
	return leftEdge + bubbleStyle.Render(" "+state+" ") + rightEdge
}

// basePaneStyle returns the base style for panes.
func (m *Model) basePaneStyle() lipgloss.Style {
	return m.renderStyles.baseBoxStyle
}

// baseInnerBoxStyle returns the base style for inner boxes.
func (m *Model) baseInnerBoxStyle() lipgloss.Style {
	return m.renderStyles.baseBoxStyle
}
