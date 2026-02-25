package app

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/chmouel/lazyworktree/internal/app/services"
	"github.com/chmouel/lazyworktree/internal/models"
)

func worktreeNoteKey(path string) string {
	return filepath.Clean(path)
}

func (m *Model) worktreeNoteKey(path string) string {
	notesPath := m.getWorktreeNotesPath()
	if notesPath == "" {
		return worktreeNoteKey(path)
	}
	return services.WorktreeNoteKey(m.getRepoKey(), m.getWorktreeDir(), notesPath, path)
}

func (m *Model) getWorktreeNotesPath() string {
	if m.config == nil {
		return ""
	}
	return strings.TrimSpace(m.config.WorktreeNotesPath)
}

func (m *Model) loadWorktreeNotes() {
	notes, err := services.LoadWorktreeNotes(m.getRepoKey(), m.getWorktreeDir(), m.getWorktreeNotesPath())
	if err != nil {
		m.debugf("failed to parse worktree notes: %v", err)
		return
	}
	if notes == nil {
		notes = map[string]models.WorktreeNote{}
	}
	m.worktreeNotes = notes

	// Auto-migrate per-repo notes to shared file if configured.
	if n, err := services.MigrateRepoNotesToSharedFile(
		m.getRepoKey(), m.getWorktreeDir(), m.getWorktreeNotesPath(),
	); err != nil {
		m.debugf("failed to migrate worktree notes: %v", err)
	} else if n > 0 {
		reloaded, err := services.LoadWorktreeNotes(m.getRepoKey(), m.getWorktreeDir(), m.getWorktreeNotesPath())
		if err == nil && reloaded != nil {
			m.worktreeNotes = reloaded
		}
		m.debugf("migrated %d worktree note(s) to shared file", n)
	}
}

func (m *Model) saveWorktreeNotes() {
	if err := services.SaveWorktreeNotes(m.getRepoKey(), m.getWorktreeDir(), m.getWorktreeNotesPath(), m.worktreeNotes); err != nil {
		m.debugf("failed to write worktree notes: %v", err)
	}
}

func (m *Model) getWorktreeNote(path string) (models.WorktreeNote, bool) {
	if strings.TrimSpace(path) == "" {
		return models.WorktreeNote{}, false
	}
	key := m.worktreeNoteKey(path)
	note, ok := m.worktreeNotes[key]
	if !ok && m.getWorktreeNotesPath() != "" {
		// Backwards compatibility with older absolute-path keys.
		note, ok = m.worktreeNotes[filepath.Clean(path)]
	}
	if !ok {
		return models.WorktreeNote{}, false
	}
	if strings.TrimSpace(note.Note) == "" {
		return models.WorktreeNote{}, false
	}
	return note, true
}

func (m *Model) setWorktreeNote(path, noteText string) {
	if strings.TrimSpace(path) == "" {
		return
	}
	if m.worktreeNotes == nil {
		m.worktreeNotes = make(map[string]models.WorktreeNote)
	}

	trimmed := strings.TrimSpace(noteText)
	key := m.worktreeNoteKey(path)
	if trimmed == "" {
		delete(m.worktreeNotes, key)
		m.saveWorktreeNotes()
		m.refreshSelectedWorktreeNotesPane()
		return
	}

	m.worktreeNotes[key] = models.WorktreeNote{
		Note:      trimmed,
		UpdatedAt: time.Now().Unix(),
	}
	if m.getWorktreeNotesPath() != "" {
		delete(m.worktreeNotes, filepath.Clean(path))
	}
	m.saveWorktreeNotes()
	m.refreshSelectedWorktreeNotesPane()
}

func (m *Model) deleteWorktreeNote(path string) {
	if strings.TrimSpace(path) == "" || len(m.worktreeNotes) == 0 {
		return
	}
	key := m.worktreeNoteKey(path)
	if _, ok := m.worktreeNotes[key]; !ok {
		return
	}
	delete(m.worktreeNotes, key)
	m.saveWorktreeNotes()
}

func (m *Model) migrateWorktreeNote(oldPath, newPath string) {
	if strings.TrimSpace(oldPath) == "" || strings.TrimSpace(newPath) == "" || len(m.worktreeNotes) == 0 {
		return
	}
	oldKey := m.worktreeNoteKey(oldPath)
	note, ok := m.worktreeNotes[oldKey]
	if !ok {
		return
	}

	delete(m.worktreeNotes, oldKey)
	note.UpdatedAt = time.Now().Unix()
	m.worktreeNotes[m.worktreeNoteKey(newPath)] = note
	m.saveWorktreeNotes()
}

func (m *Model) hasNoteForSelectedWorktree() bool {
	wt := m.selectedWorktree()
	if wt == nil {
		return false
	}
	_, ok := m.getWorktreeNote(wt.Path)
	return ok
}

func (m *Model) refreshSelectedWorktreeNotesPane() {
	m.notesContent = m.buildNotesContent(m.selectedWorktree())

	if m.state.view.FocusedPane == 4 && !m.hasNoteForSelectedWorktree() {
		m.state.view.FocusedPane = 0
		m.state.ui.worktreeTable.Focus()
		if m.state.view.ZoomedPane == 4 {
			m.state.view.ZoomedPane = -1
		}
	}
}

func (m *Model) pruneStaleWorktreeNotes(worktrees []*models.WorktreeInfo) {
	if len(m.worktreeNotes) == 0 {
		return
	}

	validPaths := make(map[string]bool, len(worktrees))
	for _, wt := range worktrees {
		if wt == nil || strings.TrimSpace(wt.Path) == "" {
			continue
		}
		validPaths[m.worktreeNoteKey(wt.Path)] = true
		if m.getWorktreeNotesPath() != "" {
			validPaths[filepath.Clean(wt.Path)] = true
		}
	}

	changed := false
	for key := range m.worktreeNotes {
		if !validPaths[key] {
			delete(m.worktreeNotes, key)
			changed = true
		}
	}
	if changed {
		m.saveWorktreeNotes()
	}
}
