package app

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View renders the active screen for the Bubble Tea program.
func (m *Model) View() string {
	if m.quitting {
		return ""
	}

	// Wait for window size before rendering full UI
	if m.view.WindowWidth == 0 || m.view.WindowHeight == 0 {
		return "Loading..."
	}

	// Always render base layout first to allow overlays
	layout := m.computeLayout()
	m.applyLayout(layout)

	header := m.renderHeader(layout)
	footer := m.renderFooter(layout)
	body := m.renderBody(layout)

	// Truncate body to fit, leaving room for header and footer
	maxBodyLines := m.view.WindowHeight - 2 // 1 for header, 1 for footer
	if layout.filterHeight > 0 {
		maxBodyLines--
	}
	body = truncateToHeight(body, maxBodyLines)

	sections := []string{header}
	if layout.filterHeight > 0 {
		sections = append(sections, m.renderFilter(layout))
	}
	sections = append(sections, body, footer)

	baseView := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Handle Modal Overlays
	switch m.currentScreen {
	case screenPalette:
		if m.paletteScreen != nil {
			return m.overlayPopup(baseView, m.paletteScreen.View(), 3)
		}
	case screenPRSelect:
		if m.prSelectionScreen != nil {
			return m.overlayPopup(baseView, m.prSelectionScreen.View(), 2)
		}
	case screenIssueSelect:
		if m.issueSelectionScreen != nil {
			return m.overlayPopup(baseView, m.issueSelectionScreen.View(), 2)
		}
	case screenListSelect:
		if m.listScreen != nil {
			return m.overlayPopup(baseView, m.listScreen.View(), 2)
		}
	case screenChecklist:
		if m.checklistScreen != nil {
			return m.overlayPopup(baseView, m.checklistScreen.View(), 2)
		}
	case screenHelp:
		if m.helpScreen != nil {
			// Center the help popup
			// Help screen has fixed/capped size logic in NewHelpScreen/SetSize
			// We can pass 0,0 to use its internal defaults or a specific size
			// In SetSize below we'll ensure it has a good "popup" size
			return m.overlayPopup(baseView, m.helpScreen.View(), 4)
		}
	case screenCommit:
		if m.commitScreen != nil {
			// Resize viewport to fit window
			vpWidth := int(float64(m.view.WindowWidth) * 0.95)
			vpHeight := int(float64(m.view.WindowHeight) * 0.85)
			if vpWidth < 80 {
				vpWidth = 80
			}
			if vpHeight < 20 {
				vpHeight = 20
			}
			m.commitScreen.viewport.Width = vpWidth
			m.commitScreen.viewport.Height = vpHeight
			return m.overlayPopup(baseView, m.commitScreen.View(), 2)
		}
	case screenConfirm:
		if m.confirmScreen != nil {
			return m.overlayPopup(baseView, m.confirmScreen.View(), 5)
		}
	case screenInfo:
		if m.infoScreen != nil {
			return m.overlayPopup(baseView, m.infoScreen.View(), 5)
		}
	case screenInput:
		if m.inputScreen != nil {
			return m.overlayPopup(baseView, m.inputScreen.View(), 5)
		}
	case screenLoading:
		if m.loadingScreen != nil {
			return m.overlayPopup(baseView, m.loadingScreen.View(), 5)
		}
	case screenCommitFiles:
		if m.commitFilesScreen != nil {
			return m.overlayPopup(baseView, m.commitFilesScreen.View(), 2)
		}
	}

	if m.currentScreen != screenNone {
		return m.renderScreen()
	}

	return baseView
}

// overlayPopup overlays a popup on top of the base view.
func (m *Model) overlayPopup(base, popup string, marginTop int) string {
	if base == "" || popup == "" {
		return base
	}

	baseLines := strings.Split(base, "\n")
	popupLines := strings.Split(popup, "\n")

	if len(baseLines) == 0 {
		return popup
	}

	baseWidth := lipgloss.Width(baseLines[0])
	popupWidth := lipgloss.Width(popupLines[0])

	leftPad := maxInt((baseWidth-popupWidth)/2, 0)
	leftSpace := strings.Repeat(" ", leftPad)
	rightPad := maxInt(baseWidth-popupWidth-leftPad, 0)
	rightSpace := strings.Repeat(" ", rightPad)

	for i, line := range popupLines {
		row := marginTop + i
		if row >= len(baseLines) {
			break
		}

		// Main popup line
		baseLines[row] = leftSpace + line + rightSpace
	}

	return strings.Join(baseLines, "\n")
}

// renderScreen renders special screens that don't use overlays.
func (m *Model) renderScreen() string {
	switch m.currentScreen {
	case screenCommit:
		if m.commitScreen == nil {
			m.commitScreen = NewCommitScreen(commitMeta{}, "", "", m.git.UseGitPager(), m.theme)
		}
		return m.commitScreen.View()
	case screenConfirm:
		if m.confirmScreen != nil {
			return m.confirmScreen.View()
		}
	case screenInfo:
		if m.infoScreen != nil {
			return m.infoScreen.View()
		}
	case screenTrust:
		if m.trustScreen == nil {
			return ""
		}
		return m.trustScreen.View()
	case screenWelcome:
		if m.welcomeScreen == nil {
			cwd, _ := os.Getwd()
			m.welcomeScreen = NewWelcomeScreen(cwd, m.getRepoWorktreeDir(), m.theme)
		}
		content := m.welcomeScreen.View()
		if m.view.WindowWidth > 0 && m.view.WindowHeight > 0 {
			return lipgloss.Place(m.view.WindowWidth, m.view.WindowHeight, lipgloss.Center, lipgloss.Center, content)
		}
		return content
	case screenPalette:
		if m.paletteScreen != nil {
			content := m.paletteScreen.View()
			if m.view.WindowWidth > 0 && m.view.WindowHeight > 0 {
				content = lipgloss.NewStyle().MarginTop(3).Render(content)
				return lipgloss.Place(
					m.view.WindowWidth,
					m.view.WindowHeight,
					lipgloss.Center,
					lipgloss.Top,
					content,
				)
			}
			return content
		}
	case screenInput:
		if m.inputScreen != nil {
			content := m.inputScreen.View()
			if m.view.WindowWidth > 0 && m.view.WindowHeight > 0 {
				return lipgloss.Place(m.view.WindowWidth, m.view.WindowHeight, lipgloss.Center, lipgloss.Center, content)
			}
			return content
		}
	case screenListSelect:
		if m.listScreen != nil {
			return m.listScreen.View()
		}
	}
	return ""
}

// truncateToHeight ensures output doesn't exceed maxLines.
func truncateToHeight(s string, maxLines int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return strings.Join(lines, "\n")
}

// truncateToHeightFromEnd returns the last maxLines lines from the string.
// Useful for git errors where the actual error is at the end.
func truncateToHeightFromEnd(s string, maxLines int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return strings.Join(lines, "\n")
}
