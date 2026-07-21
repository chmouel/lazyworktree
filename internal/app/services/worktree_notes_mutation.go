package services

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

// SaveWorktreeNote stores a single note for a worktree path.
// noteType and env are forwarded to LoadWorktreeNotes/SaveWorktreeNotes.
func SaveWorktreeNote(repoKey, worktreeDir, worktreeNotesPath, noteType, worktreePath, noteText string, env map[string]string) error {
	trimmedPath := strings.TrimSpace(worktreePath)
	trimmedNote := strings.TrimSpace(noteText)
	if trimmedPath == "" || trimmedNote == "" {
		return nil
	}

	notes, err := LoadWorktreeNotes(repoKey, worktreeDir, worktreeNotesPath, noteType, env)
	if err != nil {
		return err
	}
	if notes == nil {
		notes = map[string]models.WorktreeNote{}
	}

	var key string
	if noteType == config.NoteTypeSplitted {
		key = filepath.Base(trimmedPath)
	} else {
		key = WorktreeNoteKey(repoKey, worktreeDir, worktreeNotesPath, trimmedPath)
	}
	if key == "" {
		return nil
	}
	notes[key] = models.WorktreeNote{
		Note:      trimmedNote,
		UpdatedAt: time.Now().Unix(),
	}
	if noteType != config.NoteTypeSplitted && strings.TrimSpace(worktreeNotesPath) != "" {
		// Migrate old full-path keys when switching to shared note storage.
		delete(notes, trimmedPath)
	}
	return SaveWorktreeNoteEntry(repoKey, worktreeDir, worktreeNotesPath, noteType, key, notes, env)
}
