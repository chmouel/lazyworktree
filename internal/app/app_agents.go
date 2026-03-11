package app

import (
	"image/color"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/chmouel/lazyworktree/internal/app/services"
	"github.com/chmouel/lazyworktree/internal/models"
)

type agentRenderStyles struct {
	prefix    lipgloss.Style
	title     lipgloss.Style
	muted     lipgloss.Style
	separator lipgloss.Style
}

func (m *Model) agentRenderStyles() agentRenderStyles {
	return agentRenderStyles{
		prefix:    lipgloss.NewStyle().Foreground(m.theme.BorderDim),
		title:     lipgloss.NewStyle().Foreground(m.theme.TextFg).Bold(true),
		muted:     lipgloss.NewStyle().Foreground(m.theme.MutedFg),
		separator: lipgloss.NewStyle().Foreground(m.theme.BorderDim),
	}
}

func (m *Model) refreshAgentSessions() tea.Cmd {
	service := m.state.services.agentSessions
	if service == nil {
		return nil
	}
	processService := m.state.services.agentProcesses
	return func() tea.Msg {
		var processes []*services.AgentProcess
		if processService != nil {
			snapshot, err := processService.Refresh()
			if err != nil {
				m.debugf("agent sessions: process refresh failed: %v", err)
			} else {
				processes = snapshot
			}
		}
		sessions, err := service.RefreshWithProcesses(processes)
		return agentSessionsUpdatedMsg{sessions: sessions, err: err}
	}
}

func (m *Model) startAgentWatcher() tea.Cmd {
	watcher := m.state.services.agentWatch
	if watcher == nil || watcher.Started {
		return nil
	}
	started, err := watcher.Start()
	if err != nil {
		return func() tea.Msg { return errMsg{err: err} }
	}
	if !started {
		return nil
	}
	return m.waitForAgentWatchEvent()
}

func (m *Model) stopAgentWatcher() {
	if watcher := m.state.services.agentWatch; watcher != nil && watcher.Started {
		watcher.Stop()
	}
}

func (m *Model) waitForAgentWatchEvent() tea.Cmd {
	watcher := m.state.services.agentWatch
	if watcher == nil {
		return nil
	}
	events := watcher.NextEvent()
	if events == nil {
		return nil
	}
	return func() tea.Msg {
		_, ok := <-events
		if !ok {
			return nil
		}
		return agentWatchChangedMsg{}
	}
}

func (m *Model) hasAgentSessionsForSelectedWorktree() bool {
	return len(m.agentSessionsForSelectedWorktree()) > 0
}

func (m *Model) hasAnyAgentSessionsForSelectedWorktree() bool {
	return len(m.allAgentSessionsForSelectedWorktree()) > 0
}

func (m *Model) allAgentSessionsForSelectedWorktree() []*models.AgentSession {
	wt := m.selectedWorktree()
	service := m.state.services.agentSessions
	if wt == nil || service == nil {
		return nil
	}
	return service.SessionsForWorktree(wt.Path)
}

func (m *Model) agentSessionsForSelectedWorktree() []*models.AgentSession {
	sessions := m.allAgentSessionsForSelectedWorktree()
	if m.state.view.ShowAllAgentSessions {
		return sessions
	}
	visible := make([]*models.AgentSession, 0, len(sessions))
	for _, session := range sessions {
		if session != nil && session.IsOpen {
			visible = append(visible, session)
		}
	}
	return visible
}

func (m *Model) refreshSelectedWorktreeAgentSessionsPane() {
	selected := m.agentSessionsForSelectedWorktree()
	m.state.data.agentSessions = selected
	if len(selected) == 0 {
		m.state.data.agentSessionIndex = 0
		m.agentSessionsContent = ""
		m.state.ui.agentSessionsViewport.SetYOffset(0)
		if m.state.view.FocusedPane == paneAgentSessions {
			m.state.view.FocusedPane = paneWorktrees
			m.state.ui.worktreeTable.Focus()
			if m.state.view.ZoomedPane == paneAgentSessions {
				m.state.view.ZoomedPane = -1
			}
		}
		return
	}

	if m.state.data.agentSessionIndex >= len(selected) {
		m.state.data.agentSessionIndex = len(selected) - 1
	}
	if m.state.data.agentSessionIndex < 0 {
		m.state.data.agentSessionIndex = 0
	}
	m.agentSessionsContent = m.buildAgentSessionsContent(selected)
	m.state.ui.agentSessionsViewport.SetContent(m.agentSessionsContent)
	m.syncAgentSessionsViewport()
}

func (m *Model) buildAgentSessionsContent(sessions []*models.AgentSession) string {
	if len(sessions) == 0 {
		return ""
	}

	width := m.agentSessionViewportWidth()
	lines := make([]string, 0, len(sessions)*3)

	for i, session := range sessions {
		if session == nil {
			continue
		}
		cardLines := m.renderAgentSessionCard(
			session,
			width,
			i == m.state.data.agentSessionIndex && m.state.view.FocusedPane == paneAgentSessions,
		)
		lines = append(lines, cardLines...)
		if i < len(sessions)-1 {
			lines = append(lines, m.renderAgentSessionSeparator(width))
		}
	}

	return strings.Join(lines, "\n")
}

func (m *Model) agentSessionViewportWidth() int {
	width := m.state.ui.agentSessionsViewport.Width()
	if width > 0 {
		return width
	}
	return 60
}

func (m *Model) syncAgentSessionsViewport() {
	if len(m.state.data.agentSessions) == 0 {
		m.state.ui.agentSessionsViewport.SetYOffset(0)
		return
	}

	viewportHeight := m.state.ui.agentSessionsViewport.Height()
	if viewportHeight <= 0 {
		return
	}

	selectedLine := m.state.data.agentSessionIndex * 2
	currentOffset := m.state.ui.agentSessionsViewport.YOffset()
	if selectedLine < currentOffset {
		m.state.ui.agentSessionsViewport.SetYOffset(selectedLine)
		return
	}
	if selectedLine >= currentOffset+viewportHeight {
		m.state.ui.agentSessionsViewport.SetYOffset(selectedLine - viewportHeight + 1)
	}
}

func (m *Model) renderAgentSessionCard(session *models.AgentSession, width int, selected bool) []string {
	styles := m.agentRenderStyles()
	prefix := " "
	prefixWidth := lipgloss.Width(prefix)
	prefixStyle := styles.prefix
	titleStyle := styles.title
	if selected {
		prefix = "▏"
		prefixStyle = lipgloss.NewStyle().Foreground(m.theme.Accent)
	}

	contentWidth := max(12, width-prefixWidth)
	marker := m.renderAgentSessionMarker(session)
	right := m.renderAgentSessionRight(session)
	plainRight := ansi.Strip(right)
	title := m.agentSessionTitle(session)
	titlePrefix := marker + " " + title
	if right != "" {
		titleWidth := max(8, contentWidth-lipgloss.Width(plainRight)-1)
		titlePrefix = ansi.Truncate(titlePrefix, titleWidth, "…")
	}
	line1 := titleStyle.Render(titlePrefix)
	if right != "" {
		gapWidth := max(1, contentWidth-lipgloss.Width(ansi.Strip(titlePrefix))-lipgloss.Width(plainRight))
		line1 += strings.Repeat(" ", gapWidth) + right
	}
	line1 = prefixStyle.Render(prefix) + line1

	if !session.IsOpen {
		return []string{m.mutedPaneStyle().Width(width).Render(line1)}
	}
	return []string{line1}
}

func (m *Model) renderAgentSessionSeparator(width int) string {
	return m.agentRenderStyles().separator.Render(strings.Repeat("─", max(6, width-1)))
}

func (m *Model) renderAgentSessionMarker(session *models.AgentSession) string {
	letter := "C"
	fg := m.theme.Accent
	if session != nil && session.Agent == models.AgentKindPi {
		letter = "P"
		fg = m.theme.Cyan
	} else if m.config != nil && strings.EqualFold(strings.TrimSpace(m.config.IconSet), "nerd-font-v3") {
		letter = "✻"
	}
	return lipgloss.NewStyle().Foreground(fg).Bold(true).Render(letter)
}

func (m *Model) renderAgentSessionRight(session *models.AgentSession) string {
	if session == nil {
		return ""
	}
	styles := m.agentRenderStyles()
	parts := []string{
		m.renderAgentSessionActivityBadge(session),
		styles.muted.Render(formatRelativeTime(session.LastActivity)),
	}
	return strings.Join(parts, " ")
}

func (m *Model) renderAgentSessionActivityBadge(session *models.AgentSession) string {
	if session == nil {
		return ""
	}
	bg := m.theme.Accent
	switch session.Activity {
	case models.AgentActivityIdle:
		bg = m.theme.BorderDim
	case models.AgentActivityWaiting:
		bg = m.theme.Cyan
	case models.AgentActivityThinking, models.AgentActivityCompacting:
		bg = m.theme.Accent
	case models.AgentActivityReading, models.AgentActivitySearching, models.AgentActivityBrowsing:
		bg = m.theme.Cyan
	case models.AgentActivityWriting, models.AgentActivityRunning:
		bg = m.theme.WarnFg
	case models.AgentActivitySpawning:
		bg = m.theme.SuccessFg
	}
	return m.renderAgentSessionBadge(string(session.Activity), bg, m.theme.AccentFg)
}

func (m *Model) renderAgentSessionBadge(label string, bg, fg color.Color) string {
	badgeStyle := lipgloss.NewStyle().
		Background(bg).
		Foreground(fg).
		Bold(true).
		Padding(0, 1)
	return badgeStyle.Render(strings.ToUpper(label))
}

func (m *Model) agentSessionTitle(session *models.AgentSession) string {
	if session == nil {
		return ""
	}

	if strings.TrimSpace(session.TaskLabel) != "" {
		return session.TaskLabel
	}
	if strings.TrimSpace(session.DisplayName) != "" {
		return session.DisplayName
	}
	if session.Agent == models.AgentKindPi {
		return "pi session"
	}
	return "Claude session"
}

func (m *Model) mutedPaneStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(m.theme.MutedFg)
}
