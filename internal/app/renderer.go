package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/chmouel/lazyworktree/internal/app/screen"
)

// View renders the active screen for the Bubble Tea program.
func (m *Model) View() string {
	if m.quitting {
		return ""
	}

	// Wait for window size before rendering full UI
	if m.state.view.WindowWidth == 0 || m.state.view.WindowHeight == 0 {
		return "Loading..."
	}

	// Always render base layout first to allow overlays
	layout := m.computeLayout()
	m.applyLayout(layout)

	header := m.renderHeader(layout)
	footer := m.renderFooter(layout)
	body := m.renderBody(layout)

	// Truncate body to fit, leaving room for header and footer
	maxBodyLines := m.state.view.WindowHeight - 2 // 1 for header, 1 for footer
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

	// New path: render screens from screen manager
	if m.state.ui.screenManager.IsActive() {
		scr := m.state.ui.screenManager.Current()
		switch scr.Type() {
		case screen.TypeWelcome, screen.TypeTrust:
			// Full-screen replacement for welcome/trust screens
			content := scr.View()
			if m.state.view.WindowWidth > 0 && m.state.view.WindowHeight > 0 {
				return lipgloss.Place(m.state.view.WindowWidth, m.state.view.WindowHeight, lipgloss.Center, lipgloss.Center, content)
			}
			return content
		case screen.TypeCommit:
			// Resize viewport to fit window
			if cs, ok := scr.(*screen.CommitScreen); ok {
				vpWidth := max(80, int(float64(m.state.view.WindowWidth)*0.95))
				vpHeight := max(20, int(float64(m.state.view.WindowHeight)*0.85))
				cs.Viewport.Width = vpWidth
				cs.Viewport.Height = vpHeight
			}
			return m.overlayPopup(baseView, scr.View(), 2)
		case screen.TypeNoteView:
			if ns, ok := scr.(*screen.NoteViewScreen); ok {
				ns.Resize(m.state.view.WindowWidth, m.state.view.WindowHeight)
			}
			return m.overlayPopup(baseView, scr.View(), 2)
		case screen.TypeTaskboard:
			if ts, ok := scr.(*screen.TaskboardScreen); ok {
				ts.Resize(m.state.view.WindowWidth, m.state.view.WindowHeight)
			}
			return m.overlayPopup(baseView, scr.View(), 2)
		case screen.TypePRSelect:
			// PR selection screen with 2-margin popup
			return m.overlayPopup(baseView, scr.View(), 2)
		case screen.TypePalette:
			return m.overlayPopup(baseView, scr.View(), 3)
		default:
			// Default: overlay popup
			return m.overlayPopup(baseView, scr.View(), 3)
		}
	}

	return baseView
}

// overlayPopup overlays a popup on top of the base view, preserving
// the portions of the base that fall outside the popup bounds so that
// underlying box borders remain visible.
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

	for i, line := range popupLines {
		row := marginTop + i
		if row >= len(baseLines) {
			break
		}

		// Preserve left and right portions of the base line using
		// ANSI-aware truncation so box borders stay intact.
		leftPart := ansi.Truncate(baseLines[row], leftPad, "")
		if w := lipgloss.Width(leftPart); w < leftPad {
			leftPart += strings.Repeat(" ", leftPad-w)
		}
		rightPart := ansi.TruncateLeft(baseLines[row], leftPad+popupWidth, "")

		baseLines[row] = leftPart + line + rightPart
	}

	return strings.Join(baseLines, "\n")
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
