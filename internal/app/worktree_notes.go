package app

import (
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/app/services"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/worktreecolor"
)

func worktreeNoteKey(path string) string {
	return filepath.Clean(path)
}

func (m *Model) worktreeNoteKey(path string) string {
	if m.getWorktreeNoteType() == config.NoteTypeSplitted {
		return filepath.Base(path)
	}
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

func (m *Model) getWorktreeNoteType() string {
	if m.config == nil {
		return ""
	}
	return strings.TrimSpace(m.config.WorktreeNoteType)
}

func (m *Model) buildNoteEnv() map[string]string {
	wt := m.selectedWorktree()
	branch := ""
	wtPath := ""
	if wt != nil {
		branch = wt.Branch
		wtPath = wt.Path
	}
	return services.BuildCommandEnv(branch, wtPath, m.getRepoKey(), m.getMainWorktreePath())
}

func (m *Model) loadWorktreeNotes() {
	noteType := m.getWorktreeNoteType()
	env := m.buildNoteEnv()
	notes, err := services.LoadWorktreeNotes(m.getRepoKey(), m.getWorktreeDir(), m.getWorktreeNotesPath(), noteType, env)
	if err != nil {
		m.debugf("failed to parse worktree notes: %v", err)
		return
	}
	if notes == nil {
		notes = map[string]models.WorktreeNote{}
	}
	m.worktreeNotes = notes

	// Auto-migrate per-repo notes to shared file if configured (onejson only).
	if noteType != config.NoteTypeSplitted {
		if n, err := services.MigrateRepoNotesToSharedFile(
			m.getRepoKey(), m.getWorktreeDir(), m.getWorktreeNotesPath(),
		); err != nil {
			m.debugf("failed to migrate worktree notes: %v", err)
		} else if n > 0 {
			reloaded, err := services.LoadWorktreeNotes(m.getRepoKey(), m.getWorktreeDir(), m.getWorktreeNotesPath(), noteType, env)
			if err == nil && reloaded != nil {
				m.worktreeNotes = reloaded
			}
			m.debugf("migrated %d worktree note(s) to shared file", n)
		}
	}
}

func (m *Model) saveWorktreeNotes() {
	noteType := m.getWorktreeNoteType()
	env := m.buildNoteEnv()
	if err := services.SaveWorktreeNotes(m.getRepoKey(), m.getWorktreeDir(), m.getWorktreeNotesPath(), noteType, m.worktreeNotes, env); err != nil {
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
	if note.IsEmpty() {
		return models.WorktreeNote{}, false
	}
	return note, true
}

// updateWorktreeNoteField applies updater to the existing note for path, then
// persists the result (or deletes the entry if the updated note is empty).
func (m *Model) updateWorktreeNoteField(path string, updater func(existing models.WorktreeNote) models.WorktreeNote) {
	if strings.TrimSpace(path) == "" {
		return
	}
	if m.worktreeNotes == nil {
		m.worktreeNotes = make(map[string]models.WorktreeNote)
	}

	key := m.worktreeNoteKey(path)
	updated := updater(m.worktreeNotes[key])
	updated.UpdatedAt = time.Now().Unix()

	if updated.IsEmpty() {
		delete(m.worktreeNotes, key)
	} else {
		m.worktreeNotes[key] = updated
		if m.getWorktreeNotesPath() != "" {
			delete(m.worktreeNotes, filepath.Clean(path))
		}
	}
	m.saveWorktreeNotes()
	m.refreshSelectedWorktreeNotesPane()
}

func (m *Model) setWorktreeNote(path, noteText string) {
	trimmed := strings.TrimSpace(noteText)
	m.updateWorktreeNoteField(path, func(existing models.WorktreeNote) models.WorktreeNote {
		existing.Note = trimmed
		return existing
	})
}

func (m *Model) setWorktreeIcon(path, icon string) {
	trimmedIcon := strings.TrimSpace(icon)
	m.updateWorktreeNoteField(path, func(existing models.WorktreeNote) models.WorktreeNote {
		existing.Icon = trimmedIcon
		return existing
	})
}

func (m *Model) toggleWorktreeBold(path string) {
	m.updateWorktreeNoteField(path, func(existing models.WorktreeNote) models.WorktreeNote {
		existing.Bold = !existing.Bold
		return existing
	})
}

func (m *Model) setWorktreeDescription(path, description string) {
	trimmed := strings.TrimSpace(description)
	m.updateWorktreeNoteField(path, func(existing models.WorktreeNote) models.WorktreeNote {
		existing.Description = trimmed
		return existing
	})
}

func (m *Model) setWorktreeTags(path string, tags []string) {
	normalizedTags := models.NormalizeTags(tags)
	m.updateWorktreeNoteField(path, func(existing models.WorktreeNote) models.WorktreeNote {
		existing.Tags = normalizedTags
		return existing
	})
}

func (m *Model) setWorktreeColor(path, color string) {
	trimmedColor := worktreecolor.Normalize(color)
	m.updateWorktreeNoteField(path, func(existing models.WorktreeNote) models.WorktreeNote {
		existing.Color = trimmedColor
		return existing
	})
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
	if m.getWorktreeNoteType() == config.NoteTypeSplitted {
		_ = services.DeleteSplittedNoteFile(m.getWorktreeNotesPath(), key, m.buildNoteEnv())
	}
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
	if m.getWorktreeNoteType() == config.NoteTypeSplitted {
		_ = services.DeleteSplittedNoteFile(m.getWorktreeNotesPath(), oldKey, m.buildNoteEnv())
	}
	note.UpdatedAt = time.Now().Unix()
	m.worktreeNotes[m.worktreeNoteKey(newPath)] = note
	m.saveWorktreeNotes()
}

func (m *Model) hasNoteForSelectedWorktree() bool {
	wt := m.selectedWorktree()
	if wt == nil {
		return false
	}
	note, ok := m.getWorktreeNote(wt.Path)
	return ok && note.Note != ""
}

func (m *Model) refreshSelectedWorktreeNotesPane() {
	m.notesContent = m.buildNotesContent(m.selectedWorktree())

	if m.state.view.FocusedPane == paneNotes && !m.hasNoteForSelectedWorktree() {
		m.state.view.FocusedPane = paneWorktrees
		m.state.ui.worktreeTable.Focus()
		if m.state.view.ZoomedPane == paneNotes {
			m.state.view.ZoomedPane = -1
		}
	}
}

// showSetWorktreeDescription shows an input screen to set a short display label for the selected worktree.
func (m *Model) showSetWorktreeDescription() tea.Cmd {
	wt := m.selectedWorktree()
	if wt == nil {
		return nil
	}

	current := ""
	if note, ok := m.getWorktreeNote(wt.Path); ok {
		current = note.Description
	}

	inputScr := appscreen.NewInputScreen(
		"Set worktree description",
		"Short label for this worktree",
		current,
		m.theme,
		m.config.IconsEnabled(),
	)
	inputScr.OnSubmit = func(value string, _ bool) tea.Cmd {
		m.setWorktreeDescription(wt.Path, value)
		m.updateTable()
		if m.state.data.selectedIndex >= 0 && m.state.data.selectedIndex < len(m.state.data.filteredWts) {
			m.infoContent = m.buildInfoContent(m.state.data.filteredWts[m.state.data.selectedIndex])
		}
		return nil
	}
	inputScr.OnCancel = func() tea.Cmd {
		return nil
	}
	m.state.ui.screenManager.Push(inputScr)
	return textinput.Blink
}

// showSetWorktreeTags shows an input screen to set worktree tags.
func (m *Model) showSetWorktreeTags() tea.Cmd {
	wt := m.selectedWorktree()
	if wt == nil {
		return nil
	}

	current := ""
	currentTags := []string(nil)
	if note, ok := m.getWorktreeNote(wt.Path); ok && len(note.Tags) > 0 {
		current = strings.Join(note.Tags, ", ")
		currentTags = append(currentTags, note.Tags...)
	}

	tagScr := appscreen.NewTagEditorScreen(
		"Set worktree tags",
		currentTags,
		buildTagEditorOptions(m.worktreeTagStats()),
		m.state.view.WindowWidth,
		m.state.view.WindowHeight,
		m.theme,
		m.config.IconsEnabled(),
	)
	tagScr.Input.SetValue(current)
	tagScr.Input.CursorEnd()
	tagScr.OnSubmit = func(tags []string) tea.Cmd {
		m.setWorktreeTags(wt.Path, tags)
		m.updateTable()
		if m.state.data.selectedIndex >= 0 && m.state.data.selectedIndex < len(m.state.data.filteredWts) {
			m.infoContent = m.buildInfoContent(m.state.data.filteredWts[m.state.data.selectedIndex])
		}
		return nil
	}
	tagScr.OnCancel = func() tea.Cmd {
		return nil
	}
	m.state.ui.screenManager.Push(tagScr)
	return textinput.Blink
}

func (m *Model) pruneStaleWorktreeNotes(worktrees []*models.WorktreeInfo) {
	if len(m.worktreeNotes) == 0 {
		return
	}

	isSplitted := m.getWorktreeNoteType() == config.NoteTypeSplitted

	validPaths := make(map[string]bool, len(worktrees))
	for _, wt := range worktrees {
		if wt == nil || strings.TrimSpace(wt.Path) == "" {
			continue
		}
		validPaths[m.worktreeNoteKey(wt.Path)] = true
		if !isSplitted && m.getWorktreeNotesPath() != "" {
			validPaths[filepath.Clean(wt.Path)] = true
		}
	}

	changed := false
	for key := range m.worktreeNotes {
		if !validPaths[key] {
			if isSplitted {
				_ = services.DeleteSplittedNoteFile(m.getWorktreeNotesPath(), key, m.buildNoteEnv())
			}
			delete(m.worktreeNotes, key)
			changed = true
		}
	}
	if changed {
		m.saveWorktreeNotes()
	}
}
