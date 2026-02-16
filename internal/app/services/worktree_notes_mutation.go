package services

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/chmouel/lazyworktree/internal/models"
)

// SaveWorktreeNote stores a single note for a worktree path.
func SaveWorktreeNote(repoKey, worktreeDir, worktreePath, noteText string) error {
	trimmedPath := strings.TrimSpace(worktreePath)
	trimmedNote := strings.TrimSpace(noteText)
	if trimmedPath == "" || trimmedNote == "" {
		return nil
	}

	notes, err := LoadWorktreeNotes(repoKey, worktreeDir)
	if err != nil {
		return err
	}
	if notes == nil {
		notes = map[string]models.WorktreeNote{}
	}

	notes[filepath.Clean(trimmedPath)] = models.WorktreeNote{
		Note:      trimmedNote,
		UpdatedAt: time.Now().Unix(),
	}
	return SaveWorktreeNotes(repoKey, worktreeDir, notes)
}
