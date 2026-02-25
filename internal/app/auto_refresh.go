package app

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/chmouel/lazyworktree/internal/app/services"
)

func (m *Model) startAutoRefresh() tea.Cmd {
	if m.autoRefreshStarted {
		return nil
	}
	interval := m.autoRefreshInterval()
	if interval <= 0 {
		return nil
	}
	m.autoRefreshStarted = true
	return m.autoRefreshTick()
}

func (m *Model) autoRefreshInterval() time.Duration {
	if m.config == nil || !m.config.AutoRefresh {
		return 0
	}
	if m.config.RefreshIntervalSeconds <= 0 {
		return 0
	}
	interval := time.Duration(m.config.RefreshIntervalSeconds) * time.Second
	if interval < time.Second {
		m.debugf("auto refresh interval too small (%s), clamping to 1s", interval)
		return time.Second
	}
	return interval
}

func (m *Model) autoRefreshTick() tea.Cmd {
	interval := m.autoRefreshInterval()
	if interval <= 0 {
		return nil
	}
	return tea.Tick(interval, func(time.Time) tea.Msg {
		return autoRefreshTickMsg{}
	})
}

func (m *Model) refreshDetails() tea.Cmd {
	if len(m.state.data.filteredWts) == 0 {
		return nil
	}
	idx := m.state.ui.worktreeTable.Cursor()
	if idx < 0 || idx >= len(m.state.data.filteredWts) {
		return nil
	}
	m.deleteDetailsCache(m.state.data.filteredWts[idx].Path)
	return m.updateDetailsView()
}

func (m *Model) startGitWatcher() tea.Cmd {
	if m.state.services.watch != nil && m.state.services.watch.Started {
		return nil
	}
	if m.state.services.watch == nil {
		m.state.services.watch = services.NewGitWatchService(m.state.services.git, m.debugf)
	}
	started, err := m.state.services.watch.Start(m.ctx, m.config)
	if err != nil {
		return func() tea.Msg {
			return errMsg{err: err}
		}
	}
	if !started {
		return nil
	}
	m.autoRefreshStarted = true
	return m.waitForGitWatchEvent()
}

func (m *Model) stopGitWatcher() {
	if m.state.services.watch == nil || !m.state.services.watch.Started {
		return
	}
	m.state.services.watch.Stop()
}

func (m *Model) waitForGitWatchEvent() tea.Cmd {
	if m.state.services.watch == nil {
		return nil
	}
	events := m.state.services.watch.NextEvent()
	if events == nil {
		return nil
	}
	return func() tea.Msg {
		_, ok := <-events
		if !ok {
			return nil
		}
		return gitDirChangedMsg{}
	}
}

func (m *Model) shouldRefreshGitEvent(now time.Time) bool {
	if m.state.services.watch == nil {
		return false
	}
	return m.state.services.watch.ShouldRefresh(now)
}
