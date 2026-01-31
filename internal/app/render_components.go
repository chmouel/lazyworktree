package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wrap"
)

// renderHeader renders the application header.
func (m *Model) renderHeader(layout layoutDims) string {
	// Create a "toolbar" style header with visual flair
	headerStyle := lipgloss.NewStyle().
		Background(m.theme.AccentDim).
		Foreground(m.theme.TextFg).
		Bold(true).
		Width(layout.width).
		Padding(0, 2).Align(lipgloss.Center)

	// Add decorative icon to title
	title := "Lazyworktree"
	repoKey := strings.TrimSpace(m.repoKey)
	content := title
	if repoKey != "" && repoKey != "unknown" && !strings.HasPrefix(repoKey, "local-") {
		content = fmt.Sprintf("%s  •  %s", content, repoKey)
	}

	return headerStyle.Render(content)
}

// renderFilter renders the filter input bar.
func (m *Model) renderFilter(layout layoutDims) string {
	labelStyle := lipgloss.NewStyle().
		Foreground(m.theme.AccentFg).
		Background(m.theme.Accent).
		Bold(true).
		Padding(0, 1) // Pill effect
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

	switch m.state.view.FocusedPane {
	case 2: // Log pane
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

	case 1: // Status pane
		hints = []string{
			m.renderKeyHint("j/k", "Scroll"),
		}
		if len(m.state.data.statusFiles) > 0 {
			hints = append(hints,
				m.renderKeyHint("Enter", "Show Diff"),
				m.renderKeyHint("e", "Edit File"),
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

	default: // Worktree table (pane 0)
		hints = []string{
			m.renderKeyHint("1-3", "Pane"),
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
	// Enhanced key hints with pill/badge styling
	keyStyle := lipgloss.NewStyle().
		Foreground(m.theme.AccentFg).
		Background(m.theme.Accent).
		Bold(true).
		Padding(0, 1) // Add padding for pill effect
	labelStyle := lipgloss.NewStyle().Foreground(m.theme.Accent)
	return fmt.Sprintf("%s %s", keyStyle.Render(key), labelStyle.Render(label))
}

// renderPaneTitle renders a pane title with focus indicators.
func (m *Model) renderPaneTitle(index int, title string, focused bool, width int) string {
	showIcons := m.config.IconsEnabled()
	numStyle := lipgloss.NewStyle().Foreground(m.theme.MutedFg)
	titleStyle := lipgloss.NewStyle().Foreground(m.theme.MutedFg)
	if focused {
		numStyle = numStyle.Foreground(m.theme.Accent).Bold(true)
		titleStyle = titleStyle.Foreground(m.theme.TextFg).Bold(true)
	}
	num := numStyle.Render(fmt.Sprintf("[%d]", index))
	if showIcons {
		num = numStyle.Render(fmt.Sprintf("(%d)", index))
	}
	name := titleStyle.Render(title)

	filterIndicator := ""
	paneIdx := index - 1 // index is 1-based, panes are 0-based
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

	return lipgloss.NewStyle().Width(width).Render(fmt.Sprintf("%s %s%s%s", num, name, filterIndicator, zoomIndicator))
}

// renderInnerBox renders a bordered inner box with title and content.
func (m *Model) renderInnerBox(title, content string, width, height int) string {
	if content == "" {
		content = "No data available."
	}

	titleStyle := lipgloss.NewStyle().Foreground(m.theme.MutedFg).Bold(true)

	style := m.baseInnerBoxStyle().Width(width)
	if height > 0 {
		style = style.Height(height)
	}

	innerWidth := maxInt(1, width-style.GetHorizontalFrameSize())
	wrappedContent := wrap.String(content, innerWidth)
	boxContent := lipgloss.JoinVertical(lipgloss.Left, titleStyle.Render(title), wrappedContent)

	return style.Render(boxContent)
}

// basePaneStyle returns the base style for panes.
func (m *Model) basePaneStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.BorderDim).
		Padding(0, 1)
}

// paneStyle returns a pane style with focus indication.
func (m *Model) paneStyle(focused bool) lipgloss.Style {
	borderColor := m.theme.BorderDim
	borderStyle := lipgloss.NormalBorder()
	if focused {
		borderColor = m.theme.Accent
		// Use rounded border for focused panes for modern look
		borderStyle = lipgloss.RoundedBorder()
	}
	return lipgloss.NewStyle().
		Border(borderStyle).
		BorderForeground(borderColor).
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
