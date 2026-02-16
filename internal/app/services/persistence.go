package services

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/utils"
)

const defaultFilePerms = 0o600

// CommandPaletteUsage tracks usage frequency and recency for command palette items.
type CommandPaletteUsage struct {
	ID        string `json:"id"`
	Timestamp int64  `json:"timestamp"`
	Count     int    `json:"count"`
}

// HistoryService persists command and palette history.
type HistoryService interface {
	LoadCommands(repoKey string) []string
	SaveCommands(repoKey string, cmds []string)
	AddCommand(repoKey string, cmd string)
	LoadAccessHistory(repoKey string) map[string]int64
	SaveAccessHistory(repoKey string, history map[string]int64)
	RecordAccess(repoKey string, path string)
	LoadPaletteHistory(repoKey string) []CommandPaletteUsage
	SavePaletteHistory(repoKey string, commands []CommandPaletteUsage)
	AddPaletteUsage(repoKey string, id string)
}

// LoadCache loads worktree data from the cache file.
func LoadCache(repoKey, worktreeDir string) ([]*models.WorktreeInfo, error) {
	cachePath := filepath.Join(worktreeDir, repoKey, models.CacheFilename)
	// #nosec G304 -- cachePath is constructed from vetted directory and constant filename
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, nil
	}

	var payload struct {
		Worktrees []*models.WorktreeInfo `json:"worktrees"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	if len(payload.Worktrees) == 0 {
		return nil, nil
	}
	return payload.Worktrees, nil
}

// SaveCache saves worktree data to the cache file.
func SaveCache(repoKey, worktreeDir string, worktrees []*models.WorktreeInfo) error {
	cachePath := filepath.Join(worktreeDir, repoKey, models.CacheFilename)
	if err := os.MkdirAll(filepath.Dir(cachePath), utils.DefaultDirPerms); err != nil {
		return err
	}

	cacheData := struct {
		Worktrees []*models.WorktreeInfo `json:"worktrees"`
	}{
		Worktrees: worktrees,
	}
	data, err := json.Marshal(cacheData)
	if err != nil {
		return err
	}
	if err := os.WriteFile(cachePath, data, defaultFilePerms); err != nil {
		return err
	}
	return nil
}

// LoadCommandHistory loads command history from file.
func LoadCommandHistory(repoKey, worktreeDir string) ([]string, error) {
	historyPath := filepath.Join(worktreeDir, repoKey, models.CommandHistoryFilename)
	// #nosec G304 -- historyPath is constructed from vetted directory and constant filename
	data, err := os.ReadFile(historyPath)
	if err != nil {
		return []string{}, nil
	}

	var payload struct {
		Commands []string `json:"commands"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return []string{}, err
	}
	if payload.Commands == nil {
		return []string{}, nil
	}
	return payload.Commands, nil
}

// SaveCommandHistory saves command history to file.
func SaveCommandHistory(repoKey, worktreeDir string, commands []string) error {
	historyPath := filepath.Join(worktreeDir, repoKey, models.CommandHistoryFilename)
	if err := os.MkdirAll(filepath.Dir(historyPath), utils.DefaultDirPerms); err != nil {
		return err
	}

	historyData := struct {
		Commands []string `json:"commands"`
	}{
		Commands: commands,
	}
	data, err := json.Marshal(historyData)
	if err != nil {
		return err
	}
	if err := os.WriteFile(historyPath, data, defaultFilePerms); err != nil {
		return err
	}
	return nil
}

// LoadAccessHistory loads access history from file.
func LoadAccessHistory(repoKey, worktreeDir string) (map[string]int64, error) {
	historyPath := filepath.Join(worktreeDir, repoKey, models.AccessHistoryFilename)
	// #nosec G304 -- historyPath is constructed from vetted directory and constant filename
	data, err := os.ReadFile(historyPath)
	if err != nil {
		return nil, nil
	}

	var history map[string]int64
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, err
	}
	if history == nil {
		return map[string]int64{}, nil
	}
	return history, nil
}

// SaveAccessHistory saves access history to file.
func SaveAccessHistory(repoKey, worktreeDir string, history map[string]int64) error {
	historyPath := filepath.Join(worktreeDir, repoKey, models.AccessHistoryFilename)
	if err := os.MkdirAll(filepath.Dir(historyPath), utils.DefaultDirPerms); err != nil {
		return err
	}
	data, err := json.Marshal(history)
	if err != nil {
		return err
	}
	if err := os.WriteFile(historyPath, data, defaultFilePerms); err != nil {
		return err
	}
	return nil
}

// LoadPaletteHistory loads palette usage history from file.
func LoadPaletteHistory(repoKey, worktreeDir string) ([]CommandPaletteUsage, error) {
	historyPath := filepath.Join(worktreeDir, repoKey, models.CommandPaletteHistoryFilename)
	// #nosec G304 -- historyPath is constructed from vetted directory and constant filename
	data, err := os.ReadFile(historyPath)
	if err != nil {
		return []CommandPaletteUsage{}, nil
	}

	var payload struct {
		Commands []CommandPaletteUsage `json:"commands"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return []CommandPaletteUsage{}, err
	}
	if payload.Commands == nil {
		return []CommandPaletteUsage{}, nil
	}
	return payload.Commands, nil
}

// SavePaletteHistory saves palette usage history to file.
func SavePaletteHistory(repoKey, worktreeDir string, commands []CommandPaletteUsage) error {
	historyPath := filepath.Join(worktreeDir, repoKey, models.CommandPaletteHistoryFilename)
	if err := os.MkdirAll(filepath.Dir(historyPath), utils.DefaultDirPerms); err != nil {
		return err
	}

	historyData := struct {
		Commands []CommandPaletteUsage `json:"commands"`
	}{
		Commands: commands,
	}
	data, err := json.Marshal(historyData)
	if err != nil {
		return err
	}
	if err := os.WriteFile(historyPath, data, defaultFilePerms); err != nil {
		return err
	}
	return nil
}

// WorktreeNoteKey returns the storage key for a worktree note.
//
// Default mode stores notes in per-repo files and uses full worktree paths as keys.
// Shared-file mode (worktreeNotesPath set) uses repo-relative keys for cross-system sync.
func WorktreeNoteKey(repoKey, worktreeDir, worktreeNotesPath, worktreePath string) string {
	trimmedPath := strings.TrimSpace(worktreePath)
	if trimmedPath == "" {
		return ""
	}

	cleanPath := filepath.Clean(trimmedPath)
	if strings.TrimSpace(worktreeNotesPath) == "" {
		return cleanPath
	}

	repoRoot := filepath.Clean(filepath.Join(worktreeDir, repoKey))
	if rel, ok := relativePathWithin(repoRoot, cleanPath); ok {
		if rel == "." {
			return filepath.Base(cleanPath)
		}
		return filepath.ToSlash(rel)
	}

	worktreeRoot := filepath.Clean(worktreeDir)
	if rel, ok := relativePathWithin(worktreeRoot, cleanPath); ok {
		rel = filepath.ToSlash(rel)
		repoPrefix := filepath.ToSlash(strings.Trim(repoKey, string(filepath.Separator)))
		if repoPrefix != "" {
			if rel == repoPrefix {
				return filepath.Base(cleanPath)
			}
			if strings.HasPrefix(rel, repoPrefix+"/") {
				rel = strings.TrimPrefix(rel, repoPrefix+"/")
			}
		}
		if rel != "." && rel != "" {
			return rel
		}
	}

	return filepath.Base(cleanPath)
}

// LoadWorktreeNotes loads worktree notes from file.
func LoadWorktreeNotes(repoKey, worktreeDir, worktreeNotesPath string) (map[string]models.WorktreeNote, error) {
	if strings.TrimSpace(worktreeNotesPath) == "" {
		return loadRepoWorktreeNotes(filepath.Join(worktreeDir, repoKey, models.WorktreeNotesFilename))
	}

	allNotes, err := loadSharedWorktreeNotes(repoKey, worktreeNotesPath)
	if err != nil {
		return nil, err
	}
	repoNotes, ok := allNotes[repoKey]
	if !ok || repoNotes == nil {
		return map[string]models.WorktreeNote{}, nil
	}
	return repoNotes, nil
}

// SaveWorktreeNotes saves worktree notes to file.
func SaveWorktreeNotes(repoKey, worktreeDir, worktreeNotesPath string, notes map[string]models.WorktreeNote) error {
	normalized := normalizeWorktreeNotes(notes)

	if strings.TrimSpace(worktreeNotesPath) == "" {
		return saveRepoWorktreeNotes(filepath.Join(worktreeDir, repoKey, models.WorktreeNotesFilename), normalized)
	}

	allNotes, err := loadSharedWorktreeNotes(repoKey, worktreeNotesPath)
	if err != nil {
		return err
	}

	if len(normalized) == 0 {
		delete(allNotes, repoKey)
	} else {
		allNotes[repoKey] = normalized
	}

	if len(allNotes) == 0 {
		if err := os.Remove(worktreeNotesPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(worktreeNotesPath), utils.DefaultDirPerms); err != nil {
		return err
	}

	data, err := json.Marshal(allNotes)
	if err != nil {
		return err
	}
	if err := os.WriteFile(worktreeNotesPath, data, defaultFilePerms); err != nil {
		return err
	}
	return nil
}

func normalizeWorktreeNotes(notes map[string]models.WorktreeNote) map[string]models.WorktreeNote {
	if notes == nil {
		return map[string]models.WorktreeNote{}
	}

	normalized := make(map[string]models.WorktreeNote, len(notes))
	for noteKey, note := range notes {
		trimmed := strings.TrimSpace(note.Note)
		if trimmed == "" {
			continue
		}
		note.Note = trimmed
		normalized[noteKey] = note
	}
	return normalized
}

func loadRepoWorktreeNotes(notesPath string) (map[string]models.WorktreeNote, error) {
	// #nosec G304 -- notesPath is constructed from vetted directory and constant filename
	data, err := os.ReadFile(notesPath)
	if err != nil {
		return map[string]models.WorktreeNote{}, nil
	}

	var notes map[string]models.WorktreeNote
	if err := json.Unmarshal(data, &notes); err != nil {
		return nil, err
	}
	if notes == nil {
		return map[string]models.WorktreeNote{}, nil
	}
	return notes, nil
}

func saveRepoWorktreeNotes(notesPath string, notes map[string]models.WorktreeNote) error {
	if len(notes) == 0 {
		if err := os.Remove(notesPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(notesPath), utils.DefaultDirPerms); err != nil {
		return err
	}

	data, err := json.Marshal(notes)
	if err != nil {
		return err
	}
	if err := os.WriteFile(notesPath, data, defaultFilePerms); err != nil {
		return err
	}
	return nil
}

func loadSharedWorktreeNotes(repoKey, worktreeNotesPath string) (map[string]map[string]models.WorktreeNote, error) {
	// #nosec G304 -- path is user-configured and intentionally read for persistence
	data, err := os.ReadFile(worktreeNotesPath)
	if err != nil {
		return map[string]map[string]models.WorktreeNote{}, nil
	}

	var allNotes map[string]map[string]models.WorktreeNote
	if err := json.Unmarshal(data, &allNotes); err == nil {
		if allNotes == nil {
			return map[string]map[string]models.WorktreeNote{}, nil
		}
		return allNotes, nil
	} else {
		// Backwards compatibility: if the shared file contains a legacy single-repo payload,
		// treat it as the current repository's notes.
		var legacy map[string]models.WorktreeNote
		if legacyErr := json.Unmarshal(data, &legacy); legacyErr == nil {
			if legacy == nil {
				return map[string]map[string]models.WorktreeNote{}, nil
			}
			return map[string]map[string]models.WorktreeNote{
				repoKey: legacy,
			}, nil
		}
		return nil, err
	}
}

func relativePathWithin(base, target string) (string, bool) {
	base = filepath.Clean(base)
	target = filepath.Clean(target)

	rel, err := filepath.Rel(base, target)
	if err != nil {
		return "", false
	}
	if rel == "." {
		return ".", true
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}
