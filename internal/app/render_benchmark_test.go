package app

import (
	"fmt"
	"path/filepath"
	"testing"

	"charm.land/bubbles/v2/table"
	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

func benchmarkModel(b *testing.B, worktrees, statusFiles, logEntries int) *Model {
	b.Helper()

	cfg := &config.AppConfig{
		WorktreeDir: b.TempDir(),
		Theme:       "dracula",
	}
	m := NewModel(cfg, "")
	m.loading = false
	m.worktreesLoaded = true
	m.state.view.WindowWidth = 180
	m.state.view.WindowHeight = 50

	wts := make([]*models.WorktreeInfo, 0, worktrees)
	for i := range worktrees {
		wts = append(wts, &models.WorktreeInfo{
			Path:       filepath.Join(cfg.WorktreeDir, fmt.Sprintf("wt-%04d", i)),
			Branch:     fmt.Sprintf("feature-%04d", i),
			LastActive: "1h ago",
		})
	}
	m.state.data.worktrees = wts
	m.updateTable()
	if len(wts) > 0 {
		m.state.ui.worktreeTable.SetCursor(0)
		m.updateWorktreeArrows()
	}

	files := make([]StatusFile, 0, statusFiles)
	for i := range statusFiles {
		files = append(files, StatusFile{
			Filename: fmt.Sprintf("pkg/file-%03d.go", i),
			Status:   ".M",
		})
	}
	m.setStatusFiles(files)

	logs := make([]commitLogEntry, 0, logEntries)
	for i := range logEntries {
		logs = append(logs, commitLogEntry{
			sha:            fmt.Sprintf("%040x", i+1),
			authorInitials: "ab",
			message:        fmt.Sprintf("Benchmark commit message %d", i),
		})
	}
	m.setLogEntries(logs, true)

	return m
}

func BenchmarkModelView(b *testing.B) {
	m := benchmarkModel(b, 250, 120, 100)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = m.View()
	}
}

func BenchmarkModelViewWithOverlay(b *testing.B) {
	m := benchmarkModel(b, 250, 120, 100)
	m.state.ui.screenManager.Push(appscreen.NewConfirmScreen("Confirm action?", m.theme))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = m.View()
	}
}

func BenchmarkUpdateWorktreeArrows(b *testing.B) {
	cfg := &config.AppConfig{WorktreeDir: b.TempDir()}
	m := NewModel(cfg, "")

	rows := make([]table.Row, 0, 2000)
	for i := range 2000 {
		rows = append(rows, table.Row{fmt.Sprintf(" row-%04d", i), "", ""})
	}
	m.state.ui.worktreeTable.SetRows(rows)
	m.state.ui.worktreeTable.SetCursor(0)
	m.updateWorktreeArrows()

	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		m.state.ui.worktreeTable.SetCursor(i % len(rows))
		m.updateWorktreeArrows()
	}
}
