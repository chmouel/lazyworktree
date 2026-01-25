package app

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fsnotify/fsnotify"
)

const gitWatchDebounce = 600 * time.Millisecond

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
	if len(m.filteredWts) == 0 {
		return nil
	}
	idx := m.worktreeTable.Cursor()
	if idx < 0 || idx >= len(m.filteredWts) {
		return nil
	}
	m.deleteDetailsCache(m.filteredWts[idx].Path)
	return m.updateDetailsView()
}

func (m *Model) startGitWatcher() tea.Cmd {
	if m.gitWatchStarted || m.config == nil || !m.config.AutoRefresh {
		return nil
	}
	commonDir := m.resolveGitCommonDir()
	if commonDir == "" {
		m.debugf("auto refresh: unable to resolve git common dir")
		return nil
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return func() tea.Msg {
			return errMsg{err: err}
		}
	}

	m.gitWatchStarted = true
	m.gitWatcher = watcher
	m.gitCommonDir = commonDir
	m.gitWatchEvents = make(chan struct{}, 1)
	m.gitWatchDone = make(chan struct{})
	m.gitWatchPaths = make(map[string]struct{})
	m.gitWatchRoots = []string{
		filepath.Join(commonDir, "refs"),
		filepath.Join(commonDir, "logs"),
		filepath.Join(commonDir, "worktrees"),
	}
	m.addGitWatchDir(commonDir)
	for _, root := range m.gitWatchRoots {
		m.addGitWatchTree(root)
	}

	go m.runGitWatcher()

	return m.waitForGitWatchEvent()
}

func (m *Model) stopGitWatcher() {
	if !m.gitWatchStarted {
		return
	}
	close(m.gitWatchDone)
	m.gitWatchStarted = false
	if m.gitWatcher != nil {
		_ = m.gitWatcher.Close()
	}
}

func (m *Model) waitForGitWatchEvent() tea.Cmd {
	if m.gitWatchEvents == nil || m.gitWatchWaiting {
		return nil
	}
	m.gitWatchWaiting = true
	return func() tea.Msg {
		_, ok := <-m.gitWatchEvents
		if !ok {
			return nil
		}
		return gitDirChangedMsg{}
	}
}

func (m *Model) runGitWatcher() {
	for {
		select {
		case <-m.gitWatchDone:
			return
		case event, ok := <-m.gitWatcher.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
				continue
			}
			if event.Op&fsnotify.Create != 0 {
				m.maybeWatchNewDir(event.Name)
			}
			m.signalGitWatch()
		case err, ok := <-m.gitWatcher.Errors:
			if !ok {
				return
			}
			m.debugf("git watcher error: %v", err)
		}
	}
}

func (m *Model) maybeWatchNewDir(path string) {
	if !m.isUnderGitWatchRoot(path) {
		return
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return
	}
	m.addGitWatchDir(path)
}

func (m *Model) signalGitWatch() {
	select {
	case <-m.gitWatchDone:
		return
	default:
	}
	select {
	case m.gitWatchEvents <- struct{}{}:
	default:
	}
}

func (m *Model) addGitWatchDir(path string) {
	if path == "" {
		return
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return
	}

	m.gitWatchMu.Lock()
	defer m.gitWatchMu.Unlock()

	if _, ok := m.gitWatchPaths[path]; ok {
		return
	}
	if err := m.gitWatcher.Add(path); err != nil {
		m.debugf("git watcher add failed for %s: %v", path, err)
		return
	}
	m.gitWatchPaths[path] = struct{}{}
}

func (m *Model) addGitWatchTree(root string) {
	if root == "" {
		return
	}
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		m.addGitWatchDir(path)
		return nil
	})
}

func (m *Model) isUnderGitWatchRoot(path string) bool {
	if path == "" {
		return false
	}
	for _, root := range m.gitWatchRoots {
		if root == "" {
			continue
		}
		if path == root || strings.HasPrefix(path, root+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func (m *Model) resolveGitCommonDir() string {
	commonDir := strings.TrimSpace(m.git.RunGit(m.ctx, []string{"git", "rev-parse", "--git-common-dir"}, "", []int{0}, true, false))
	if commonDir == "" {
		return ""
	}
	if filepath.IsAbs(commonDir) {
		return commonDir
	}

	repoRoot := strings.TrimSpace(m.git.RunGit(m.ctx, []string{"git", "rev-parse", "--show-toplevel"}, "", []int{0}, true, false))
	if repoRoot != "" {
		return filepath.Join(repoRoot, commonDir)
	}
	if abs, err := filepath.Abs(commonDir); err == nil {
		return abs
	}
	return commonDir
}

func (m *Model) shouldRefreshGitEvent(now time.Time) bool {
	if !m.gitLastRefresh.IsZero() && now.Sub(m.gitLastRefresh) < gitWatchDebounce {
		return false
	}
	m.gitLastRefresh = now
	return true
}
