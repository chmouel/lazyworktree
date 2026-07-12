package app

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/chmouel/lazyworktree/internal/app/services"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

func TestStatusUpdatedMsgUpdatesWorktreeStatus(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	m.state.data.worktrees = []*models.WorktreeInfo{{Path: wtPath, Branch: "main"}}
	m.updateTable()

	msg := statusUpdatedMsg{
		statusFiles: []StatusFile{
			{Filename: "staged.txt", Status: "M."},
			{Filename: "modified.txt", Status: ".M"},
			{Filename: "new.txt", Status: " ?", IsUntracked: true},
		},
		path: wtPath,
	}

	_, _ = m.Update(msg)

	wt := m.state.data.worktrees[0]
	if !wt.Dirty {
		t.Fatal("expected worktree to be dirty")
	}
	if wt.Staged != 1 {
		t.Fatalf("expected staged count 1, got %d", wt.Staged)
	}
	if wt.Modified != 1 {
		t.Fatalf("expected modified count 1, got %d", wt.Modified)
	}
	if wt.Untracked != 1 {
		t.Fatalf("expected untracked count 1, got %d", wt.Untracked)
	}
}

func TestRefreshDetails(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	// Test with empty filtered worktrees
	cmd := m.refreshDetails()
	if cmd != nil {
		t.Error("expected nil command for empty worktrees")
	}

	// Test with worktrees
	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	m.state.data.worktrees = []*models.WorktreeInfo{{Path: wtPath, Branch: "main"}}
	m.state.data.filteredWts = m.state.data.worktrees
	m.state.ui.worktreeTable.SetWidth(100)
	m.updateTable()
	m.updateTableColumns(m.state.ui.worktreeTable.Width())

	// Set cursor to valid position
	if len(m.state.ui.worktreeTable.Rows()) > 0 {
		m.state.ui.worktreeTable.SetCursor(0)
		// Add something to cache
		m.resetDetailsCache()
		m.setDetailsCache(wtPath, &detailsCacheEntry{})

		cmd := m.refreshDetails()
		// Command may or may not be nil depending on updateDetailsView implementation
		_ = cmd
		// Cache entry is retained; freshness is governed by the TTL and
		// watcher-driven invalidation rather than the periodic tick.
		if _, ok := m.getDetailsCache(wtPath); !ok {
			t.Error("expected details cache entry to be retained")
		}
	}
}

func TestRefreshDetailsInvalidCursor(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	m.state.data.worktrees = []*models.WorktreeInfo{{Path: wtPath, Branch: "main"}}
	m.state.data.filteredWts = m.state.data.worktrees
	m.state.ui.worktreeTable.SetWidth(100)
	m.updateTable()
	m.updateTableColumns(m.state.ui.worktreeTable.Width())

	// Set cursor to invalid position
	m.state.ui.worktreeTable.SetCursor(999)

	cmd := m.refreshDetails()
	if cmd != nil {
		t.Error("expected nil command for invalid cursor")
	}
}

func TestDetailsCacheConcurrentAccess(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	cacheKey := filepath.Join(cfg.WorktreeDir, "wt1")
	m.resetDetailsCache()
	m.setDetailsCache(cacheKey, &detailsCacheEntry{})

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 2000; j++ {
				_, _ = m.getDetailsCache(cacheKey)
			}
		}()
	}
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 2000; j++ {
				m.setDetailsCache(cacheKey, &detailsCacheEntry{})
				m.deleteDetailsCache(cacheKey)
			}
		}()
	}
	wg.Wait()

	m.setDetailsCache(cacheKey, &detailsCacheEntry{})
	if _, ok := m.getDetailsCache(cacheKey); !ok {
		t.Fatal("expected details cache entry to be set")
	}
}

func TestIsUnderGitWatchRoot(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	// Set up git watch roots
	m.state.services.watch.Roots = []string{
		"/tmp/git/refs",
		"/tmp/git/logs",
		"/tmp/git/worktrees",
	}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "path under refs root",
			path:     "/tmp/git/refs/heads/main",
			expected: true,
		},
		{
			name:     "path under logs root",
			path:     "/tmp/git/logs/refs/heads/main",
			expected: true,
		},
		{
			name:     "path under worktrees root",
			path:     "/tmp/git/worktrees/wt1/HEAD",
			expected: true,
		},
		{
			name:     "path not under any root",
			path:     "/tmp/other/path",
			expected: false,
		},
		{
			name:     "exact match with root",
			path:     "/tmp/git/refs",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.state.services.watch.IsUnderRoot(tt.path)
			if result != tt.expected {
				t.Errorf("isUnderGitWatchRoot(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestMaybeWatchNewDir(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	// Set up git watch roots
	watchRoot := t.TempDir()
	m.state.services.watch.Roots = []string{watchRoot}
	m.state.services.watch.Paths = make(map[string]struct{})

	// Test with path not under watch root (should return early)
	otherPath := filepath.Join(t.TempDir(), "other")
	if err := os.MkdirAll(otherPath, 0o750); err != nil { //nolint:gosec // test directory permissions
		t.Fatalf("failed to create test dir: %v", err)
	}
	m.state.services.watch.MaybeWatchNewDir(otherPath)
	// Should return early without calling addGitWatchDir

	// Test with non-directory (should return early after stat)
	filePath := filepath.Join(watchRoot, "file.txt")
	if err := os.WriteFile(filePath, []byte("test"), 0o600); err != nil { //nolint:gosec // test file permissions
		t.Fatalf("failed to create test file: %v", err)
	}
	m.state.services.watch.MaybeWatchNewDir(filePath)
	// Should return early because it's not a directory

	// Note: Testing with actual directory would require initializing the watcher,
	// which is complex. The function logic is tested above (early returns).
}

func TestSignalGitWatch(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	// Set up git watch channels
	m.state.services.watch.Events = make(chan struct{}, 1)
	m.state.services.watch.Done = make(chan struct{})

	// Signal should send to channel
	m.state.services.watch.Signal()

	// Verify event was sent (non-blocking check)
	select {
	case <-m.state.services.watch.Events:
		// Good, event was sent
	default:
		t.Error("expected event to be sent to gitWatchEvents channel")
	}

	// Test with closed done channel
	close(m.state.services.watch.Done)
	m.state.services.watch.Signal()
	// Should return early without sending
}

func TestShouldRefreshGitEventDebounce(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	now := time.Now()
	if !m.state.services.watch.ShouldRefresh(now) {
		t.Fatal("expected first refresh to pass")
	}
	if m.state.services.watch.ShouldRefresh(now.Add(services.GitWatchDebounce / 2)) {
		t.Fatal("expected debounce to block refresh")
	}
	if !m.state.services.watch.ShouldRefresh(now.Add(services.GitWatchDebounce + time.Millisecond)) {
		t.Fatal("expected refresh after debounce window")
	}
}

func TestAutoRefreshTickSuspendsWhenUnfocused(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir(), AutoRefresh: true, RefreshIntervalSeconds: 10}
	m := NewModel(cfg, "")
	m.autoRefreshStarted = true
	m.state.view.TerminalFocused = false

	_, cmd := m.Update(autoRefreshTickMsg{})
	if cmd != nil {
		t.Fatal("expected no command while unfocused")
	}
	if m.autoRefreshStarted {
		t.Fatal("expected tick loop to suspend while unfocused")
	}

	_, cmd = m.Update(tea.FocusMsg{})
	if !m.autoRefreshStarted {
		t.Fatal("expected tick loop to resume on focus")
	}
	if cmd == nil {
		t.Fatal("expected focus to schedule work")
	}
}

func TestAutoRefreshTickKeepsRunningWhenFocused(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir(), AutoRefresh: true, RefreshIntervalSeconds: 10}
	m := NewModel(cfg, "")
	m.autoRefreshStarted = true
	m.state.view.TerminalFocused = true

	_, cmd := m.Update(autoRefreshTickMsg{})
	if cmd == nil {
		t.Fatal("expected the focused tick to reschedule itself")
	}
	if !m.autoRefreshStarted {
		t.Fatal("expected tick loop to stay running while focused")
	}
}
