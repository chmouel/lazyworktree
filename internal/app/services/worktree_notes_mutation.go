package services

import (
	"strings"
	"time"

	"github.com/chmouel/lazyworktree/internal/models"
)

// SaveWorktreeNote stores a single note for a worktree path.
func SaveWorktreeNote(repoKey, worktreeDir, worktreeNotesPath, worktreePath, noteText string) error {
	trimmedPath := strings.TrimSpace(worktreePath)
	trimmedNote := strings.TrimSpace(noteText)
	if trimmedPath == "" || trimmedNote == "" {
		return nil
	}

	notes, err := LoadWorktreeNotes(repoKey, worktreeDir, worktreeNotesPath)
	if err != nil {
		return err
	}
	if notes == nil {
		notes = map[string]models.WorktreeNote{}
	}

	key := WorktreeNoteKey(repoKey, worktreeDir, worktreeNotesPath, trimmedPath)
	if key == "" {
		return nil
	}
	notes[key] = models.WorktreeNote{
		Note:      trimmedNote,
		UpdatedAt: time.Now().Unix(),
	}
	if strings.TrimSpace(worktreeNotesPath) != "" {
		// Migrate old full-path keys when switching to shared note storage.
		delete(notes, trimmedPath)
	}
	return SaveWorktreeNotes(repoKey, worktreeDir, worktreeNotesPath, notes)
}
