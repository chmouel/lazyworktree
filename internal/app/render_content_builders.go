package app

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/utils"
)

// buildNotesContent builds the notes content string for a worktree.
func (m *Model) buildNotesContent(wt *models.WorktreeInfo) string {
	if wt == nil {
		return ""
	}
	note, ok := m.getWorktreeNote(wt.Path)
	if !ok {
		return ""
	}
	valueStyle := lipgloss.NewStyle().Foreground(m.theme.TextFg)
	lines := m.renderMarkdownNoteLines(note.Note, valueStyle)
	return strings.Join(lines, "\n")
}

// renderNotesBox renders the Notes content using a viewport for scrolling.
func (m *Model) renderNotesBox(width, height int) string {
	content := m.notesContent
	if content == "" {
		content = "No notes."
	}

	innerBoxStyle := m.baseInnerBoxStyle()

	vpWidth := max(1, width-innerBoxStyle.GetHorizontalFrameSize())
	vpHeight := max(1, height-innerBoxStyle.GetVerticalFrameSize())

	m.state.ui.notesViewport.SetWidth(vpWidth)
	m.state.ui.notesViewport.SetHeight(vpHeight)
	m.state.ui.notesViewport.SetContent(utils.WrapANSIContent(content, vpWidth))

	return innerBoxStyle.
		Width(width).
		Height(height).
		Render(m.state.ui.notesViewport.View())
}

func (m *Model) renderAgentSessionsBox(width, height int) string {
	content := m.agentSessionsContent
	if content == "" {
		content = "No agent sessions."
	}

	innerBoxStyle := m.baseInnerBoxStyle()

	vpWidth := max(1, width-innerBoxStyle.GetHorizontalFrameSize())
	vpHeight := max(1, height-innerBoxStyle.GetVerticalFrameSize())

	m.state.ui.agentSessionsViewport.SetWidth(vpWidth)
	m.state.ui.agentSessionsViewport.SetHeight(vpHeight)
	m.state.ui.agentSessionsViewport.SetContent(utils.WrapANSIContent(content, vpWidth))
	m.syncAgentSessionsViewport()

	return innerBoxStyle.
		Width(width).
		Height(height).
		Render(m.state.ui.agentSessionsViewport.View())
}

// infoSectionDivider returns a thin horizontal rule for separating info pane sections.
func (m *Model) infoSectionDivider(width int) string {
	w := width
	if w <= 0 {
		w = 20
	}
	return m.renderStyles.infoDividerStyle.Render(strings.Repeat("─", w))
}

// buildInfoContent builds the info content string for a worktree.
func (m *Model) buildInfoContent(wt *models.WorktreeInfo) string {
	if wt == nil {
		return errNoWorktreeSelected
	}
	// Consider any worktree on the same branch as the main worktree as a main-branch view.
	isMainBranch := wt.IsMain
	if !isMainBranch {
		for _, candidate := range m.state.data.worktrees {
			if candidate != nil && candidate.IsMain && candidate.Branch != "" && wt.Branch == candidate.Branch {
				isMainBranch = true
				break
			}
		}
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
	if note, ok := m.getWorktreeNote(wt.Path); ok {
		if note.Description != "" {
			infoLines = addField(infoLines, "Description:", valueStyle.Render(note.Description))
		}
		if normalizedTags := models.NormalizeTags(note.Tags); len(normalizedTags) > 0 {
			infoLines = addField(infoLines, "Tags:", m.renderTagPills(normalizedTags))
		}
	}

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
	hidePRDetails := wt.PR != nil && wt.IsMain && (wt.PR.State == prStateMerged || wt.PR.State == prStateClosed)
	if wt.PR != nil && !hidePRDetails && !m.config.DisablePR {
		authorText := wt.PR.Author
		renderStyle := lipgloss.NewStyle().Foreground(m.theme.TextFg).Bold(true)
		if wt.PR.Author != "" {
			if wt.PR.AuthorName != "" {
				authorText = fmt.Sprintf("@%s", wt.PR.Author)
			} else {
				authorText = wt.PR.Author
			}
			if wt.PR.AuthorIsBot {
				authorText = iconPrefix(UIIconBot, m.config.IconsEnabled()) + authorText
			}
			authorText = renderStyle.Render(authorText)
		}
		prLabelStyle := lipgloss.NewStyle().Foreground(m.theme.Accent).Bold(true) // Accent for PR prominence
		prNumber := fmt.Sprintf("#%d", wt.PR.Number)
		prNumber = renderStyle.Render(prNumber)
		prPrefix := fmt.Sprintf("PR/MR %s by %s", prNumber, authorText)
		if m.config.IconsEnabled() {
			prPrefix = iconWithSpace(getIconPR()) + prPrefix
		}
		infoLines = append(infoLines, m.infoSectionDivider(30))
		infoLines = append(infoLines, prLabelStyle.Render(prPrefix))
		infoLines = append(infoLines, fmt.Sprintf("  %s ", wt.PR.Title))
		// // URL styled with cyan for consistency
		infoLines = append(infoLines, fmt.Sprintf("  %s", wt.PR.URL))
	} else if wt.PR == nil && !m.config.DisablePR && wt.HasUpstream {
		// Skip the PR section entirely when there is nothing actionable to show:
		// - confirmed no PR exists, or
		// - on the main branch before any fetch has completed (avoids noise)
		skipPRSection := wt.PRFetchStatus == models.PRFetchStatusNoPR ||
			(isMainBranch && !m.loading.prDataLoaded &&
				wt.PRFetchStatus != models.PRFetchStatusFetching &&
				wt.PRFetchStatus != models.PRFetchStatusError &&
				wt.PRFetchStatus != models.PRFetchStatusLoaded)
		if !skipPRSection {
			grayStyle := lipgloss.NewStyle().Foreground(m.theme.MutedFg)
			errorStyle := lipgloss.NewStyle().Foreground(m.theme.ErrorFg)
			prLabelStyle := lipgloss.NewStyle().Foreground(m.theme.Accent).Bold(true)
			prPrefix := "PR:"
			if m.config.IconsEnabled() {
				prPrefix = iconWithSpace(getIconPR()) + prPrefix
			}

			infoLines = append(infoLines, m.infoSectionDivider(30))
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

			case models.PRFetchStatusFetching:
				infoLines = append(infoLines, grayStyle.Render("  Fetching PR data..."))

			default:
				// Not fetched yet (non-main branch)
				if !m.loading.prDataLoaded {
					infoLines = append(infoLines, grayStyle.Render("  Press 'r' to refresh and fetch PR data"))
				}
			}
		}
	}

	// CI status from cache (shown for all branches with cached checks, not just PRs)
	if !m.config.DisablePR {
		if cachedChecks, _, ok := m.cache.ciCache.Get(wt.Branch); ok && len(cachedChecks) > 0 {
			infoLines = append(infoLines, m.infoSectionDivider(30))

			// Summary chip next to CI Checks heading
			aggregate := aggregateCIConclusion(cachedChecks)
			summaryChip := m.renderCIStatusChip(aggregate, m.config.IconsEnabled())
			infoLines = append(infoLines, sectionStyle.Render("CI Checks:")+" "+summaryChip)

			selectedStyle := lipgloss.NewStyle().
				Foreground(m.theme.AccentFg).
				Background(m.theme.Accent).
				Bold(true)

			checks := sortCIChecks(cachedChecks)
			for i, check := range checks {
				isSelected := m.state.view.FocusedPane == paneInfo && m.ciCheckIndex >= 0 && i == m.ciCheckIndex

				symbol := getCIStatusIcon(check.Conclusion, false, m.config.IconsEnabled())
				var line string
				if isSelected {
					line = fmt.Sprintf("  %s %s", symbol, check.Name)
					line = selectedStyle.Render(line)
				} else {
					iconStyle := m.ciIconStyle(check.Conclusion)
					line = fmt.Sprintf("  %s %s", iconStyle.Render(symbol), check.Name)
				}
				infoLines = append(infoLines, line)
			}
		}
	}

	return strings.Join(infoLines, "\n")
}

// aggregateCIConclusion computes the overall CI status from a slice of checks.
// Priority: failure > pending > success > skipped/cancelled.
func aggregateCIConclusion(checks []*models.CICheck) string {
	hasSuccess := false
	hasPending := false
	for _, c := range checks {
		switch c.Conclusion {
		case "failure":
			return "failure"
		case "pending", "":
			hasPending = true
		case "success":
			hasSuccess = true
		}
	}
	if hasPending {
		return "pending"
	}
	if hasSuccess {
		return "success"
	}
	return "skipped"
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

	viewportWidth := m.state.ui.statusViewport.Width()
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
		case m.state.view.FocusedPane == paneGitStatus && m.ciCheckIndex < 0 && i == m.state.services.statusTree.Index:
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
