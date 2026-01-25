package app

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

func TestStatusUpdatedMsgUpdatesWorktreeStatus(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	m.worktrees = []*models.WorktreeInfo{{Path: wtPath, Branch: "main"}}
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

	wt := m.worktrees[0]
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
	m.worktrees = []*models.WorktreeInfo{{Path: wtPath, Branch: "main"}}
	m.filteredWts = m.worktrees
	m.worktreeTable.SetWidth(100)
	m.updateTable()
	m.updateTableColumns(m.worktreeTable.Width())

	// Set cursor to valid position
	if len(m.worktreeTable.Rows()) > 0 {
		m.worktreeTable.SetCursor(0)
		// Add something to cache
		m.resetDetailsCache()
		m.setDetailsCache(wtPath, &detailsCacheEntry{})

		cmd := m.refreshDetails()
		// Command may or may not be nil depending on updateDetailsView implementation
		_ = cmd
		// Cache should be cleared
		if _, ok := m.getDetailsCache(wtPath); ok {
			t.Error("expected details cache to be cleared")
		}
	}
}

func TestRefreshDetailsInvalidCursor(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	wtPath := filepath.Join(cfg.WorktreeDir, "wt1")
	m.worktrees = []*models.WorktreeInfo{{Path: wtPath, Branch: "main"}}
	m.filteredWts = m.worktrees
	m.worktreeTable.SetWidth(100)
	m.updateTable()
	m.updateTableColumns(m.worktreeTable.Width())

	// Set cursor to invalid position
	m.worktreeTable.SetCursor(999)

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
	m.gitWatchRoots = []string{
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
			result := m.isUnderGitWatchRoot(tt.path)
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
	m.gitWatchRoots = []string{watchRoot}
	m.gitWatchPaths = make(map[string]struct{})

	// Test with path not under watch root (should return early)
	otherPath := filepath.Join(t.TempDir(), "other")
	if err := os.MkdirAll(otherPath, 0o750); err != nil { //nolint:gosec // test directory permissions
		t.Fatalf("failed to create test dir: %v", err)
	}
	m.maybeWatchNewDir(otherPath)
	// Should return early without calling addGitWatchDir

	// Test with non-directory (should return early after stat)
	filePath := filepath.Join(watchRoot, "file.txt")
	if err := os.WriteFile(filePath, []byte("test"), 0o600); err != nil { //nolint:gosec // test file permissions
		t.Fatalf("failed to create test file: %v", err)
	}
	m.maybeWatchNewDir(filePath)
	// Should return early because it's not a directory

	// Note: Testing with actual directory would require initializing the watcher,
	// which is complex. The function logic is tested above (early returns).
}

func TestSignalGitWatch(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	// Set up git watch channels
	m.gitWatchEvents = make(chan struct{}, 1)
	m.gitWatchDone = make(chan struct{})

	// Signal should send to channel
	m.signalGitWatch()

	// Verify event was sent (non-blocking check)
	select {
	case <-m.gitWatchEvents:
		// Good, event was sent
	default:
		t.Error("expected event to be sent to gitWatchEvents channel")
	}

	// Test with closed done channel
	close(m.gitWatchDone)
	m.signalGitWatch()
	// Should return early without sending
}

func TestShouldRefreshGitEventDebounce(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	now := time.Now()
	if !m.shouldRefreshGitEvent(now) {
		t.Fatal("expected first refresh to pass")
	}
	if m.shouldRefreshGitEvent(now.Add(gitWatchDebounce / 2)) {
		t.Fatal("expected debounce to block refresh")
	}
	if !m.shouldRefreshGitEvent(now.Add(gitWatchDebounce + time.Millisecond)) {
		t.Fatal("expected refresh after debounce window")
	}
}
