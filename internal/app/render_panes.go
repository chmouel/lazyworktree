package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/chmouel/lazyworktree/internal/app/state"
	"github.com/chmouel/lazyworktree/internal/models"
)

// renderBody renders the main body area with panes.
func (m *Model) renderBody(layout layoutDims) string {
	// Handle zoom mode: only render the zoomed pane (layout agnostic)
	if m.state.view.ZoomedPane >= 0 {
		switch m.state.view.ZoomedPane {
		case 0:
			return m.renderZoomedLeftPane(layout)
		case 1:
			return m.renderZoomedRightTopPane(layout)
		case 2:
			return m.renderZoomedRightBottomPane(layout)
		}
	}

	if layout.layoutMode == state.LayoutTop {
		return m.renderTopLayoutBody(layout)
	}

	left := m.renderLeftPane(layout)
	right := m.renderRightPane(layout)
	gap := lipgloss.NewStyle().
		Width(layout.gapX).
		Render(strings.Repeat(" ", layout.gapX))
	return lipgloss.JoinHorizontal(lipgloss.Top, left, gap, right)
}

// renderLeftPane renders the left pane (worktree table).
func (m *Model) renderLeftPane(layout layoutDims) string {
	title := m.renderPaneTitle(1, "Worktrees", m.state.view.FocusedPane == 0, layout.leftInnerWidth)
	tableView := m.state.ui.worktreeTable.View()
	content := lipgloss.JoinVertical(lipgloss.Left, title, tableView)
	return m.paneStyle(m.state.view.FocusedPane == 0).
		Width(layout.leftWidth).
		Height(layout.bodyHeight).
		MaxHeight(layout.bodyHeight).
		Render(content)
}

// renderRightPane renders the right pane container (status + log).
func (m *Model) renderRightPane(layout layoutDims) string {
	top := m.renderRightTopPane(layout)
	bottom := m.renderRightBottomPane(layout)
	gap := strings.Repeat("\n", layout.gapY)
	return lipgloss.JoinVertical(lipgloss.Left, top, gap, bottom)
}

// renderRightTopPane renders the right top pane (status viewport).
func (m *Model) renderRightTopPane(layout layoutDims) string {
	title := m.renderPaneTitle(2, "Status", m.state.view.FocusedPane == 1, layout.rightInnerWidth)

	// Constrain info box height to prevent overflow when CI checks are numerous
	innerBoxStyle := m.baseInnerBoxStyle()
	minStatusBoxRendered := 3 + innerBoxStyle.GetVerticalFrameSize()
	maxInfoBoxHeight := maxInt(3, layout.rightTopInnerHeight-lipgloss.Height(title)-minStatusBoxRendered)
	infoBox := m.renderInnerBox("Info", m.infoContent, layout.rightInnerWidth, maxInfoBoxHeight)

	statusBoxHeight := maxInt(layout.rightTopInnerHeight-lipgloss.Height(title)-lipgloss.Height(infoBox)-2, 3)
	statusViewportWidth := maxInt(1, layout.rightInnerWidth-innerBoxStyle.GetHorizontalFrameSize())
	statusViewportHeight := maxInt(1, statusBoxHeight-innerBoxStyle.GetVerticalFrameSize())
	m.state.ui.statusViewport.Width = statusViewportWidth
	m.state.ui.statusViewport.Height = statusViewportHeight
	m.state.ui.statusViewport.SetContent(m.statusContent)
	statusBox := innerBoxStyle.
		Width(layout.rightInnerWidth).
		Height(statusBoxHeight).
		Render(m.state.ui.statusViewport.View())

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		infoBox,
		statusBox,
	)
	return m.paneStyle(m.state.view.FocusedPane == 1).
		Width(layout.rightWidth).
		Height(layout.rightTopHeight).
		MaxHeight(layout.rightTopHeight).
		Render(content)
}

// renderRightBottomPane renders the right bottom pane (log table).
func (m *Model) renderRightBottomPane(layout layoutDims) string {
	title := m.renderPaneTitle(3, "Log", m.state.view.FocusedPane == 2, layout.rightInnerWidth)
	content := lipgloss.JoinVertical(lipgloss.Left, title, m.state.ui.logTable.View())
	return m.paneStyle(m.state.view.FocusedPane == 2).
		Width(layout.rightWidth).
		Height(layout.rightBottomHeight).
		MaxHeight(layout.rightBottomHeight).
		Render(content)
}

// renderTopLayoutBody renders the body for the top layout mode.
func (m *Model) renderTopLayoutBody(layout layoutDims) string {
	top := m.renderTopPane(layout)
	bottom := m.renderBottomPane(layout)
	gap := strings.Repeat("\n", layout.gapY)
	return lipgloss.JoinVertical(lipgloss.Left, top, gap, bottom)
}

// renderTopPane renders the full-width worktree pane at the top.
func (m *Model) renderTopPane(layout layoutDims) string {
	title := m.renderPaneTitle(1, "Worktrees", m.state.view.FocusedPane == 0, layout.topInnerWidth)
	tableView := m.state.ui.worktreeTable.View()
	content := lipgloss.JoinVertical(lipgloss.Left, title, tableView)
	return m.paneStyle(m.state.view.FocusedPane == 0).
		Width(layout.width).
		Height(layout.topHeight).
		MaxHeight(layout.topHeight).
		Render(content)
}

// renderBottomPane renders the bottom pane container (status + log side by side).
func (m *Model) renderBottomPane(layout layoutDims) string {
	left := m.renderBottomLeftPane(layout)
	right := m.renderBottomRightPane(layout)
	gap := lipgloss.NewStyle().
		Width(layout.gapX).
		Render(strings.Repeat(" ", layout.gapX))
	return lipgloss.JoinHorizontal(lipgloss.Top, left, gap, right)
}

// renderBottomLeftPane renders the status pane in the bottom left of the top layout.
func (m *Model) renderBottomLeftPane(layout layoutDims) string {
	title := m.renderPaneTitle(2, "Status", m.state.view.FocusedPane == 1, layout.bottomLeftInnerWidth)

	innerBoxStyle := m.baseInnerBoxStyle()
	minStatusBoxRendered := 3 + innerBoxStyle.GetVerticalFrameSize()
	maxInfoBoxHeight := maxInt(3, layout.bottomLeftInnerHeight-lipgloss.Height(title)-minStatusBoxRendered)
	infoBox := m.renderInnerBox("Info", m.infoContent, layout.bottomLeftInnerWidth, maxInfoBoxHeight)

	statusBoxHeight := maxInt(layout.bottomLeftInnerHeight-lipgloss.Height(title)-lipgloss.Height(infoBox)-2, 3)
	statusViewportWidth := maxInt(1, layout.bottomLeftInnerWidth-innerBoxStyle.GetHorizontalFrameSize())
	statusViewportHeight := maxInt(1, statusBoxHeight-innerBoxStyle.GetVerticalFrameSize())
	m.state.ui.statusViewport.Width = statusViewportWidth
	m.state.ui.statusViewport.Height = statusViewportHeight
	m.state.ui.statusViewport.SetContent(m.statusContent)
	statusBox := innerBoxStyle.
		Width(layout.bottomLeftInnerWidth).
		Height(statusBoxHeight).
		Render(m.state.ui.statusViewport.View())

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		infoBox,
		statusBox,
	)
	return m.paneStyle(m.state.view.FocusedPane == 1).
		Width(layout.bottomLeftWidth).
		Height(layout.bottomHeight).
		MaxHeight(layout.bottomHeight).
		Render(content)
}

// renderBottomRightPane renders the log pane in the bottom right of the top layout.
func (m *Model) renderBottomRightPane(layout layoutDims) string {
	title := m.renderPaneTitle(3, "Log", m.state.view.FocusedPane == 2, layout.bottomRightInnerWidth)
	content := lipgloss.JoinVertical(lipgloss.Left, title, m.state.ui.logTable.View())
	return m.paneStyle(m.state.view.FocusedPane == 2).
		Width(layout.bottomRightWidth).
		Height(layout.bottomHeight).
		MaxHeight(layout.bottomHeight).
		Render(content)
}

// renderZoomedLeftPane renders the zoomed left pane.
func (m *Model) renderZoomedLeftPane(layout layoutDims) string {
	title := m.renderPaneTitle(1, "Worktrees", true, layout.leftInnerWidth)
	tableView := m.state.ui.worktreeTable.View()
	content := lipgloss.JoinVertical(lipgloss.Left, title, tableView)
	return m.paneStyle(true).
		Width(layout.leftWidth).
		Height(layout.bodyHeight).
		MaxHeight(layout.bodyHeight).
		Render(content)
}

// renderZoomedRightTopPane renders the zoomed right top pane.
func (m *Model) renderZoomedRightTopPane(layout layoutDims) string {
	title := m.renderPaneTitle(2, "Status", true, layout.rightInnerWidth)

	// Constrain info box height to prevent overflow when CI checks are numerous
	innerBoxStyle := m.baseInnerBoxStyle()
	minStatusBoxRendered := 3 + innerBoxStyle.GetVerticalFrameSize()
	maxInfoBoxHeight := maxInt(3, layout.rightTopInnerHeight-lipgloss.Height(title)-minStatusBoxRendered)
	infoBox := m.renderInnerBox("Info", m.infoContent, layout.rightInnerWidth, maxInfoBoxHeight)

	statusBoxHeight := maxInt(layout.rightTopInnerHeight-lipgloss.Height(title)-lipgloss.Height(infoBox)-2, 3)
	statusViewportWidth := maxInt(1, layout.rightInnerWidth-innerBoxStyle.GetHorizontalFrameSize())
	statusViewportHeight := maxInt(1, statusBoxHeight-innerBoxStyle.GetVerticalFrameSize())
	m.state.ui.statusViewport.Width = statusViewportWidth
	m.state.ui.statusViewport.Height = statusViewportHeight
	m.state.ui.statusViewport.SetContent(m.statusContent)
	statusBox := innerBoxStyle.
		Width(layout.rightInnerWidth).
		Height(statusBoxHeight).
		Render(m.state.ui.statusViewport.View())

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		infoBox,
		statusBox,
	)
	return m.paneStyle(true).
		Width(layout.rightWidth).
		Height(layout.bodyHeight).
		MaxHeight(layout.bodyHeight).
		Render(content)
}

// renderZoomedRightBottomPane renders the zoomed right bottom pane.
func (m *Model) renderZoomedRightBottomPane(layout layoutDims) string {
	title := m.renderPaneTitle(3, "Log", true, layout.rightInnerWidth)
	content := lipgloss.JoinVertical(lipgloss.Left, title, m.state.ui.logTable.View())
	return m.paneStyle(true).
		Width(layout.rightWidth).
		Height(layout.bodyHeight).
		MaxHeight(layout.bodyHeight).
		Render(content)
}

// buildInfoContent builds the info content string for a worktree.
func (m *Model) buildInfoContent(wt *models.WorktreeInfo) string {
	if wt == nil {
		return errNoWorktreeSelected
	}

	labelStyle := lipgloss.NewStyle().Foreground(m.theme.Cyan).Bold(true)
	valueStyle := lipgloss.NewStyle().Foreground(m.theme.TextFg)
	sectionStyle := lipgloss.NewStyle().Foreground(m.theme.Accent).Bold(true)
	keyWidth := lipgloss.Width("Last Accessed:")
	keyStyle := labelStyle.Width(keyWidth)

	addField := func(lines []string, key, value string) []string {
		return append(lines, fmt.Sprintf("%s %s", keyStyle.Render(key), value))
	}

	infoLines := make([]string, 0, 32)
	infoLines = addField(infoLines, "Path:", valueStyle.Render(wt.Path))
	infoLines = addField(infoLines, "Branch:", valueStyle.Render(wt.Branch))

	if wt.LastSwitchedTS > 0 {
		accessTime := time.Unix(wt.LastSwitchedTS, 0)
		relTime := formatRelativeTime(accessTime)
		infoLines = addField(infoLines, "Last Accessed:", valueStyle.Render(relTime))
	}
	if wt.Ahead > 0 || wt.Behind > 0 {
		aheadStyle := lipgloss.NewStyle().Foreground(m.theme.Cyan)
		behindStyle := lipgloss.NewStyle().Foreground(m.theme.ErrorFg)
		parts := make([]string, 0, 2)
		if wt.Ahead > 0 {
			parts = append(parts, aheadStyle.Render(fmt.Sprintf("%s%d", aheadIndicator(m.config.IconsEnabled()), wt.Ahead)))
		}
		if wt.Behind > 0 {
			parts = append(parts, behindStyle.Render(fmt.Sprintf("%s%d", behindIndicator(m.config.IconsEnabled()), wt.Behind)))
		}
		infoLines = addField(infoLines, "Divergence:", strings.Join(parts, " "))
	}
	if note, ok := m.getWorktreeNote(wt.Path); ok {
		infoLines = append(infoLines, "")
		infoLines = append(infoLines, sectionStyle.Render("Annotation:"))
		for _, line := range strings.Split(note.Note, "\n") {
			infoLines = append(infoLines, "  "+valueStyle.Render(line))
		}
	}
	hidePRDetails := wt.PR != nil && wt.IsMain && (wt.PR.State == prStateMerged || wt.PR.State == prStateClosed)
	if wt.PR != nil && !hidePRDetails && !m.config.DisablePR {
		prLabelStyle := lipgloss.NewStyle().Foreground(m.theme.Accent).Bold(true) // Accent for PR prominence
		prPrefix := "PR:"
		if m.config.IconsEnabled() {
			prPrefix = iconWithSpace(getIconPR()) + prPrefix
		}
		numStyle := lipgloss.NewStyle().Foreground(m.theme.TextFg)
		stateColor := m.theme.SuccessFg // default to success for OPEN
		switch wt.PR.State {
		case prStateMerged:
			stateColor = m.theme.Accent
		case prStateClosed:
			stateColor = m.theme.ErrorFg
		}
		stateStyle := lipgloss.NewStyle().Foreground(stateColor)
		prNumber := fmt.Sprintf("#%d", wt.PR.Number)
		prNumber = osc8Hyperlink(prNumber, wt.PR.URL)
		infoLines = append(infoLines, "")
		infoLines = append(infoLines, prLabelStyle.Render(prPrefix))
		infoLines = append(infoLines, fmt.Sprintf("  %s %s [%s]",
			numStyle.Render(prNumber),
			wt.PR.Title,
			stateStyle.Render(wt.PR.State)))
		// Author line with bot indicator if applicable
		if wt.PR.Author != "" {
			grayStyle := lipgloss.NewStyle().Foreground(m.theme.MutedFg)
			var authorText string
			if wt.PR.AuthorName != "" {
				authorText = fmt.Sprintf("%s (@%s)", wt.PR.AuthorName, wt.PR.Author)
			} else {
				authorText = wt.PR.Author
			}
			if wt.PR.AuthorIsBot {
				authorText = iconPrefix(UIIconBot, m.config.IconsEnabled()) + authorText
			}
			infoLines = append(infoLines, fmt.Sprintf("  by %s", grayStyle.Render(authorText)))
		}
		// URL styled with cyan for consistency
		urlStyle := lipgloss.NewStyle().Foreground(m.theme.Cyan).Underline(true)
		infoLines = append(infoLines, fmt.Sprintf("  %s", urlStyle.Render(wt.PR.URL)))
	} else if wt.PR == nil && !m.config.DisablePR {
		// Show PR status/error when PR is nil
		grayStyle := lipgloss.NewStyle().Foreground(m.theme.MutedFg)
		errorStyle := lipgloss.NewStyle().Foreground(m.theme.ErrorFg)
		prLabelStyle := lipgloss.NewStyle().Foreground(m.theme.Accent).Bold(true)
		prPrefix := "PR:"
		if m.config.IconsEnabled() {
			prPrefix = iconWithSpace(getIconPR()) + prPrefix
		}

		infoLines = append(infoLines, "")
		infoLines = append(infoLines, prLabelStyle.Render(prPrefix))

		switch wt.PRFetchStatus {
		case models.PRFetchStatusLoaded:
			// This shouldn't happen (PR is nil but status is loaded) - show debug info
			infoLines = append(infoLines, errorStyle.Render("  PR Status: Loaded but nil (bug)"))

		case models.PRFetchStatusError:
			infoLines = append(infoLines, valueStyle.Bold(true).Render("  PR Status:"))
			infoLines = append(infoLines, errorStyle.Render("    Fetch failed"))

			// Provide helpful error messages based on error content
			switch {
			case strings.Contains(wt.PRFetchError, "not found") || strings.Contains(wt.PRFetchError, "PATH"):
				infoLines = append(infoLines, grayStyle.Render("    gh/glab CLI not found"))
				infoLines = append(infoLines, grayStyle.Render("    Install from https://cli.github.com"))
			case strings.Contains(wt.PRFetchError, "auth") || strings.Contains(wt.PRFetchError, "401"):
				infoLines = append(infoLines, grayStyle.Render("    Authentication failed"))
				infoLines = append(infoLines, grayStyle.Render("    Run 'gh auth login' or 'glab auth login'"))
			case wt.PRFetchError != "":
				infoLines = append(infoLines, grayStyle.Render(fmt.Sprintf("    %s", wt.PRFetchError)))
			}

		case models.PRFetchStatusNoPR:
			switch {
			case m.prDataLoaded && wt.HasUpstream:
				// Fetch was attempted, no error, no PR found - this is expected
				infoLines = append(infoLines, grayStyle.Render("  No PR for this branch"))
			case !wt.HasUpstream:
				// No upstream, so no PR possible
				infoLines = append(infoLines, grayStyle.Render("  Branch has no upstream"))
			}

		case models.PRFetchStatusFetching:
			infoLines = append(infoLines, grayStyle.Render("  Fetching PR data..."))

		default:
			// Not fetched yet
			if !m.prDataLoaded {
				infoLines = append(infoLines, grayStyle.Render("  Press 'p' to fetch PR data"))
			}
		}
	}

	// CI status from cache (shown for all branches with cached checks, not just PRs)
	if !m.config.DisablePR {
		if cachedChecks, _, ok := m.cache.ciCache.Get(wt.Branch); ok && len(cachedChecks) > 0 {
			infoLines = append(infoLines, "") // blank line before CI
			infoLines = append(infoLines, sectionStyle.Render("CI Checks:"))

			greenStyle := lipgloss.NewStyle().Foreground(m.theme.SuccessFg)
			redStyle := lipgloss.NewStyle().Foreground(m.theme.ErrorFg)
			yellowStyle := lipgloss.NewStyle().Foreground(m.theme.WarnFg)
			grayStyle := lipgloss.NewStyle().Foreground(m.theme.MutedFg)
			selectedStyle := lipgloss.NewStyle().
				Foreground(m.theme.AccentFg).
				Background(m.theme.Accent).
				Bold(true)

			checks := sortCIChecks(cachedChecks)
			for i, check := range checks {
				symbol := getCIStatusIcon(check.Conclusion, false, m.config.IconsEnabled())
				isSelected := m.state.view.FocusedPane == 1 && m.ciCheckIndex >= 0 && i == m.ciCheckIndex

				var line string
				if isSelected {
					// When selected, apply selection style to entire line
					line = fmt.Sprintf("  %s %s", symbol, check.Name)
					line = selectedStyle.Render(line)
				} else {
					// When not selected, apply conclusion color to icon only
					var iconStyle lipgloss.Style
					switch check.Conclusion {
					case "success":
						iconStyle = greenStyle
					case "failure":
						iconStyle = redStyle
					case "skipped":
						iconStyle = grayStyle
					case "cancelled":
						iconStyle = grayStyle
					case "pending", "":
						iconStyle = yellowStyle
					default:
						iconStyle = grayStyle
					}
					line = fmt.Sprintf("  %s %s", iconStyle.Render(symbol), check.Name)
				}
				infoLines = append(infoLines, line)
			}
		}
	}

	return strings.Join(infoLines, "\n")
}

// renderStatusFiles renders the status file list with current selection highlighted.
func (m *Model) renderStatusFiles() string {
	if len(m.state.services.statusTree.TreeFlat) == 0 {
		if len(m.state.data.statusFilesAll) == 0 {
			return lipgloss.NewStyle().Foreground(m.theme.SuccessFg).Render("Clean working tree")
		}
		if strings.TrimSpace(m.state.services.filter.StatusFilterQuery) != "" {
			return lipgloss.NewStyle().Foreground(m.theme.MutedFg).Render(
				fmt.Sprintf("No files match %q", strings.TrimSpace(m.state.services.filter.StatusFilterQuery)),
			)
		}
		return lipgloss.NewStyle().Foreground(m.theme.MutedFg).Render("No files to display")
	}

	modifiedStyle := lipgloss.NewStyle().Foreground(m.theme.WarnFg)
	addedStyle := lipgloss.NewStyle().Foreground(m.theme.SuccessFg)
	deletedStyle := lipgloss.NewStyle().Foreground(m.theme.ErrorFg)
	untrackedStyle := lipgloss.NewStyle().Foreground(m.theme.WarnFg)
	stagedStyle := lipgloss.NewStyle().Foreground(m.theme.Cyan)
	dirStyle := lipgloss.NewStyle().Foreground(m.theme.MutedFg)
	selectedStyle := lipgloss.NewStyle().
		Foreground(m.theme.AccentFg).
		Background(m.theme.Accent).
		Bold(true)

	viewportWidth := m.state.ui.statusViewport.Width
	showIcons := m.config.IconsEnabled()

	lines := make([]string, 0, len(m.state.services.statusTree.TreeFlat))
	for i, node := range m.state.services.statusTree.TreeFlat {
		indent := strings.Repeat("  ", node.Depth)

		var lineContent string
		var fileIcon string
		if node.IsDir() {
			// Directory line: "  ▼ dirname" or "  ▶ dirname"
			expandIcon := disclosureIndicator(m.state.services.statusTree.CollapsedDirs[node.Path], showIcons)
			dirIcon := ""
			if showIcons {
				dirIcon = iconWithSpace(deviconForName(node.Name(), true))
			}
			lineContent = fmt.Sprintf("%s%s %s%s", indent, expandIcon, dirIcon, node.Path)
		} else {
			// File line: "    M  filename" or "    S  filename" for staged
			status := node.File.Status
			displayStatus := formatStatusDisplay(status)
			if showIcons {
				fileIcon = iconWithSpace(deviconForName(node.Name(), false))
			}
			lineContent = fmt.Sprintf("%s  %s %s%s", indent, displayStatus, fileIcon, node.Name())
		}

		// Apply styling based on selection and node type
		switch {
		case m.state.view.FocusedPane == 1 && m.ciCheckIndex < 0 && i == m.state.services.statusTree.Index:
			if viewportWidth > 0 && len(lineContent) < viewportWidth {
				lineContent += strings.Repeat(" ", viewportWidth-len(lineContent))
			}
			lines = append(lines, selectedStyle.Render(lineContent))
		case node.IsDir():
			lines = append(lines, dirStyle.Render(lineContent))
		default:
			// Color based on file status - apply different colors for staged vs unstaged
			status := node.File.Status
			if len(status) < 2 {
				lines = append(lines, lineContent)
				continue
			}

			// Special case for untracked files
			if status == " ?" {
				displayStatus := formatStatusDisplay(status)
				formatted := fmt.Sprintf("%s  %s %s%s", indent, untrackedStyle.Render(displayStatus), fileIcon, node.Name())
				lines = append(lines, formatted)
				continue
			}

			x := status[0] // Staged status
			y := status[1] // Unstaged status
			displayStatus := formatStatusDisplay(status)

			// Render each character with appropriate color based on position
			var statusRendered strings.Builder
			for i, char := range displayStatus {
				if char == ' ' {
					statusRendered.WriteString(" ")
					continue
				}

				var style lipgloss.Style
				if i == 0 {
					// First character is staged (X position)
					switch x {
					case 'M':
						style = stagedStyle // Cyan for staged modifications
					case 'A':
						style = addedStyle // Green for staged additions
					case 'D':
						style = deletedStyle // Red for staged deletions
					case 'R', 'C':
						style = stagedStyle // Cyan for staged renames/copies
					default:
						style = lipgloss.NewStyle()
					}
				} else {
					// Second character is unstaged (Y position)
					switch y {
					case 'M':
						style = modifiedStyle // Orange for unstaged modifications
					case 'A':
						style = addedStyle // Green for unstaged additions
					case 'D':
						style = deletedStyle // Red for unstaged deletions
					case 'R', 'C':
						style = modifiedStyle // Orange for unstaged renames/copies
					default:
						style = lipgloss.NewStyle()
					}
				}
				statusRendered.WriteString(style.Render(string(char)))
			}
			formatted := fmt.Sprintf("%s  %s %s%s", indent, statusRendered.String(), fileIcon, node.Name())
			lines = append(lines, formatted)
		}
	}
	return strings.Join(lines, "\n")
}
