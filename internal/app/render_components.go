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
	appStyle := lipgloss.NewStyle().
		Foreground(m.theme.Accent).
		Bold(true)

	repoKey := strings.TrimSpace(m.repoKey)
	repoStr := ""
	if repoKey != "" && repoKey != "unknown" && !strings.HasPrefix(repoKey, "local-") {
		repoText := repoKey
		if showIcons {
			repoText = " " + repoText // Repo icon
		}
		repoStyle := lipgloss.NewStyle().
			Foreground(m.theme.TextFg).
			Background(m.theme.BorderDim). // Clean bubble background
			Padding(0, 1)
		repoStr = "   " + repoStyle.Render(repoText)
	}

	headerStyle := lipgloss.NewStyle().
		Background(m.theme.AccentDim).
		Width(layout.width).
		Padding(0, 2).
		Align(lipgloss.Center)

	content := appStyle.Render(appText) + repoStr
	return headerStyle.Render(content)
}

// renderFilter renders the filter input bar.
func (m *Model) renderFilter(layout layoutDims) string {
	labelStyle := lipgloss.NewStyle().
		Foreground(m.theme.AccentFg).
		Background(m.theme.Accent).
		Bold(true).
		Padding(0, 1) // bubble effect
	filterStyle := lipgloss.NewStyle().
		Foreground(m.theme.TextFg).
		Padding(0, 1)
	line := fmt.Sprintf("%s %s", labelStyle.Render(m.inputLabel()), m.state.ui.filterInput.View())
	return filterStyle.Width(layout.width).Render(line)
}

// renderFooter renders the application footer with context-aware hints.
func (m *Model) renderFooter(layout layoutDims) string {
	footerStyle := lipgloss.NewStyle().
		Foreground(m.theme.TextFg).
		Background(m.theme.BorderDim).
		Padding(0, 1)

	// Context-aware hints based on focused pane
	var hints []string

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
		hints = []string{
			m.renderKeyHint("j/k", "Scroll"),
			m.renderKeyHint("i", "Edit Note"),
			m.renderKeyHint("Tab", "Switch Pane"),
			m.renderKeyHint("q", "Quit"),
			m.renderKeyHint("?", "Help"),
		}

	case 3: // Commit pane
		if len(m.state.data.logEntries) > 0 {
			hints = []string{
				m.renderKeyHint("Enter", "View Commit"),
				m.renderKeyHint("C", "Cherry-pick"),
				m.renderKeyHint("j/k", "Navigate"),
				m.renderKeyHint("f", "Filter"),
				m.renderKeyHint("/", "Search"),
				m.renderKeyHint("r", "Refresh"),
				m.renderKeyHint("Tab", "Switch Pane"),
				m.renderKeyHint("q", "Quit"),
				m.renderKeyHint("?", "Help"),
			}
		} else {
			hints = []string{
				m.renderKeyHint("f", "Filter"),
				m.renderKeyHint("/", "Search"),
				m.renderKeyHint("Tab", "Switch Pane"),
				m.renderKeyHint("q", "Quit"),
				m.renderKeyHint("?", "Help"),
			}
		}

	case 2: // Git Status pane
		hints = []string{
			m.renderKeyHint("j/k", "Scroll"),
		}
		if len(m.state.data.statusFiles) > 0 {
			hints = append(hints,
				m.renderKeyHint("Enter", "Show Diff"),
				m.renderKeyHint("e", "Edit File"),
				m.renderKeyHint("s", "Stage"),
			)
		}
		hints = append(hints,
			m.renderKeyHint("f", "Filter"),
			m.renderKeyHint("/", "Search"),
			m.renderKeyHint("Tab", "Switch Pane"),
			m.renderKeyHint("r", "Refresh"),
			m.renderKeyHint("q", "Quit"),
			m.renderKeyHint("?", "Help"),
		)

	case 1: // Info pane (info + CI)
		hints = []string{
			m.renderKeyHint("j/k", "Scroll"),
			m.renderKeyHint("n/p", "CI Checks"),
			m.renderKeyHint("Enter", "Open URL"),
			m.renderKeyHint("Ctrl+v", "CI Logs"),
			m.renderKeyHint("Tab", "Switch Pane"),
			m.renderKeyHint("r", "Refresh"),
			m.renderKeyHint("q", "Quit"),
			m.renderKeyHint("?", "Help"),
		}

	default: // Worktree table (pane 0)
		hints = []string{
			m.renderKeyHint(paneHint, "Pane"),
			m.renderKeyHint("c", "Create"),
			m.renderKeyHint("f", "Filter"),
			m.renderKeyHint("d", "Diff"),
			m.renderKeyHint("D", "Delete"),
			m.renderKeyHint("S", "Sync"),
		}
		// Show "o" key hint only when current worktree has PR info
		if m.state.data.selectedIndex >= 0 && m.state.data.selectedIndex < len(m.state.data.filteredWts) {
			wt := m.state.data.filteredWts[m.state.data.selectedIndex]
			if wt.PR != nil {
				hints = append(hints, m.renderKeyHint("o", "Open PR"))
			}
		}
		hints = append(hints, m.customFooterHints()...)
		hints = append(hints,
			m.renderKeyHint("y", "Copy"),
			m.renderKeyHint("q", "Quit"),
			m.renderKeyHint("?", "Help"),
			m.renderKeyHint("ctrl+p", "Palette"),
		)
	}

	footerContent := strings.Join(hints, "  ")
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
	// Enhanced key hints with bubble/badge styling
	keyStyle := lipgloss.NewStyle().
		Foreground(m.theme.AccentFg).
		Background(m.theme.Accent).
		Bold(true).
		Padding(0, 1) // Add padding for bubble effect
	labelStyle := lipgloss.NewStyle().Foreground(m.theme.Accent)
	return fmt.Sprintf("%s %s", keyStyle.Render(key), labelStyle.Render(label))
}

// renderPaneBlock renders a pane block with the title embedded in the top border.
func (m *Model) renderPaneBlock(index int, title string, focused bool, width, height int, innerContent string) string {
	border := lipgloss.NormalBorder()
	borderColor := m.theme.BorderDim
	if focused {
		border = lipgloss.RoundedBorder()
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

	// bubble styling for title
	bubbleBg := m.theme.BorderDim
	bubbleFg := m.theme.TextFg
	isBold := false

	if focused {
		bubbleBg = m.theme.Accent
		bubbleFg = m.theme.AccentFg
		isBold = true
	}

	bubbleStyle := lipgloss.NewStyle().Background(bubbleBg).Foreground(bubbleFg).Bold(isBold)
	leftEdge := lipgloss.NewStyle().Foreground(bubbleBg).Render("")
	rightEdge := lipgloss.NewStyle().Foreground(bubbleBg).Render("")
	titleText := fmt.Sprintf(" %s %s ", numStr, title)

	filterIndicator := ""
	paneIdx := index - 1
	if !m.state.view.ShowingFilter && !m.state.view.ShowingSearch && m.hasActiveFilterForPane(paneIdx) {
		filteredStyle := lipgloss.NewStyle().Foreground(m.theme.WarnFg).Italic(true)
		keyStyle := lipgloss.NewStyle().
			Foreground(m.theme.AccentFg).
			Background(m.theme.Accent).
			Bold(true).
			Padding(0, 1)
		filterIndicator = fmt.Sprintf(" %s%s  %s %s",
			iconPrefix(UIIconFilter, showIcons),
			filteredStyle.Render("Filtered"),
			keyStyle.Render("Esc"),
			lipgloss.NewStyle().Foreground(m.theme.MutedFg).Render("Clear"))
	}

	zoomIndicator := ""
	if m.state.view.ZoomedPane == paneIdx {
		zoomedStyle := lipgloss.NewStyle().Foreground(m.theme.Accent).Italic(true)
		keyStyle := lipgloss.NewStyle().
			Foreground(m.theme.AccentFg).
			Background(m.theme.Accent).
			Bold(true).
			Padding(0, 1)
		zoomIndicator = fmt.Sprintf(" %s%s  %s %s",
			iconPrefix(UIIconZoom, showIcons),
			zoomedStyle.Render("Zoomed"),
			keyStyle.Render("="),
			lipgloss.NewStyle().Foreground(m.theme.MutedFg).Render("Unzoom"))
	}

	borderStyle := lipgloss.NewStyle().Foreground(borderColor)
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
	leftEdge := lipgloss.NewStyle().Foreground(bg).Render("\ue0b6")
	rightEdge := lipgloss.NewStyle().Foreground(bg).Render("\ue0b4")
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
		return lipgloss.NewStyle().Foreground(m.theme.SuccessFg)
	case "failure":
		return lipgloss.NewStyle().Foreground(m.theme.ErrorFg)
	case "pending", "":
		return lipgloss.NewStyle().Foreground(m.theme.WarnFg)
	default: // skipped, cancelled, etc.
		return lipgloss.NewStyle().Foreground(m.theme.MutedFg)
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
	leftEdge := lipgloss.NewStyle().Foreground(bg).Render("\ue0b6")
	rightEdge := lipgloss.NewStyle().Foreground(bg).Render("\ue0b4")
	return leftEdge + bubbleStyle.Render(" "+state+" ") + rightEdge
}

// basePaneStyle returns the base style for panes.
func (m *Model) basePaneStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.BorderDim).
		Padding(0, 1)
}

// baseInnerBoxStyle returns the base style for inner boxes.
func (m *Model) baseInnerBoxStyle() lipgloss.Style {
	// Use rounded border for inner boxes for softer appearance
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.BorderDim).
		Padding(0, 1)
}
