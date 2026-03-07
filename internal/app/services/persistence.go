package services

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/utils"
	"github.com/chmouel/lazyworktree/internal/worktreecolor"
)

const (
	defaultFilePerms = 0o600
	// worktreeNameSentinel is a unique placeholder used when computing
	// prefix/suffix from a path template by expanding WORKTREE_NAME.
	worktreeNameSentinel = "\x00SENTINEL\x00"
)

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

// MigrateRepoNotesToSharedFile moves per-repo worktree notes into the shared
// notes file when worktreeNotesPath is configured. Old absolute-path keys are
// converted to repo-relative keys. Entries already present in the shared file
// with a newer UpdatedAt are preserved. The per-repo file is removed on success.
// Returns the number of migrated notes.
func MigrateRepoNotesToSharedFile(repoKey, worktreeDir, worktreeNotesPath string) (int, error) {
	if strings.TrimSpace(worktreeNotesPath) == "" {
		return 0, nil
	}

	repoNotesPath := filepath.Join(worktreeDir, repoKey, models.WorktreeNotesFilename)
	oldNotes, err := loadRepoWorktreeNotes(repoNotesPath)
	if err != nil {
		return 0, err
	}
	if len(oldNotes) == 0 {
		return 0, nil
	}

	allNotes, err := loadSharedWorktreeNotes(repoKey, worktreeNotesPath)
	if err != nil {
		return 0, err
	}

	repoNotes := allNotes[repoKey]
	if repoNotes == nil {
		repoNotes = map[string]models.WorktreeNote{}
	}

	migrated := 0
	for oldKey, note := range oldNotes {
		newKey := WorktreeNoteKey(repoKey, worktreeDir, worktreeNotesPath, oldKey)
		if existing, ok := repoNotes[newKey]; ok && existing.UpdatedAt > note.UpdatedAt {
			continue
		}
		repoNotes[newKey] = note
		migrated++
	}

	if migrated == 0 {
		// All entries already existed with newer timestamps; still clean up.
		_ = os.Remove(repoNotesPath)
		return 0, nil
	}

	allNotes[repoKey] = repoNotes

	if err := os.MkdirAll(filepath.Dir(worktreeNotesPath), utils.DefaultDirPerms); err != nil {
		return 0, err
	}
	data, err := json.Marshal(allNotes)
	if err != nil {
		return 0, err
	}
	if err := os.WriteFile(worktreeNotesPath, data, defaultFilePerms); err != nil {
		return 0, err
	}

	_ = os.Remove(repoNotesPath)
	return migrated, nil
}

// cloneEnvWith returns a copy of env with an additional key=val entry.
func cloneEnvWith(env map[string]string, key, val string) map[string]string {
	out := make(map[string]string, len(env)+1)
	for k, v := range env {
		out[k] = v
	}
	out[key] = val
	return out
}

// LoadWorktreeNotes loads worktree notes from file.
// When noteType is "splitted", notes are loaded from individual files matching
// the pathTemplate with env variables expanded. For other types (including empty),
// the existing onejson behaviour is used.
func LoadWorktreeNotes(repoKey, worktreeDir, worktreeNotesPath, noteType string, env map[string]string) (map[string]models.WorktreeNote, error) {
	if noteType == config.NoteTypeSplitted {
		return loadSplittedWorktreeNotes(worktreeNotesPath, env)
	}

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
// When noteType is "splitted", notes are saved as individual files. For other
// types (including empty), the existing onejson behaviour is used.
func SaveWorktreeNotes(repoKey, worktreeDir, worktreeNotesPath, noteType string, notes map[string]models.WorktreeNote, env map[string]string) error {
	if noteType == config.NoteTypeSplitted {
		return saveSplittedWorktreeNotes(worktreeNotesPath, notes, env)
	}

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
		note.Note = strings.TrimSpace(note.Note)
		note.Icon = strings.TrimSpace(note.Icon)
		note.Color = worktreecolor.Normalize(note.Color)
		if note.IsEmpty() {
			continue
		}
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

// loadSplittedWorktreeNotes discovers individual note files matching a path template.
func loadSplittedWorktreeNotes(pathTemplate string, env map[string]string) (map[string]models.WorktreeNote, error) {
	matches, err := findSplittedNotePaths(pathTemplate, env)
	if err != nil {
		return nil, err
	}

	notes := make(map[string]models.WorktreeNote, len(matches))
	for wtName, match := range matches {
		// #nosec G304 -- match comes from filepath.Glob on user-configured template
		data, rerr := os.ReadFile(match)
		if rerr != nil {
			continue
		}
		note, perr := ParseNoteFile(data)
		if perr != nil {
			continue
		}
		// Trim the note body for consistency with onejson normalisation.
		note.Note = strings.TrimSpace(note.Note)
		if note.IsEmpty() {
			continue
		}
		notes[wtName] = note
	}
	return notes, nil
}

func findSplittedNotePaths(pathTemplate string, env map[string]string) (map[string]string, error) {
	// Expand all variables, substituting WORKTREE_NAME with a glob wildcard.
	pattern := ExpandWithEnv(pathTemplate, cloneEnvWith(env, "WORKTREE_NAME", "*"))

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	// Compute prefix/suffix once from the template — they don't depend on the matched path.
	sentinelExpanded := ExpandWithEnv(pathTemplate, cloneEnvWith(env, "WORKTREE_NAME", worktreeNameSentinel))
	sentinelParts := strings.SplitN(sentinelExpanded, worktreeNameSentinel, 2)
	var prefix, suffix string
	if len(sentinelParts) == 2 {
		prefix, suffix = sentinelParts[0], sentinelParts[1]
	}

	paths := make(map[string]string, len(matches))
	for _, match := range matches {
		// Derive the worktree name from the precomputed prefix/suffix.
		wtName := extractWorktreeNameFromParts(match, prefix, suffix)
		if wtName == "" {
			continue
		}
		paths[wtName] = match
	}
	return paths, nil
}

// extractWorktreeNameFromParts extracts the worktree name from a matched path
// given a precomputed prefix and suffix derived from the path template.
func extractWorktreeNameFromParts(matched, prefix, suffix string) string {
	if prefix == "" && suffix == "" {
		return filepath.Base(filepath.Dir(matched))
	}
	if !strings.HasPrefix(matched, prefix) {
		return ""
	}
	rest := matched[len(prefix):]
	if suffix != "" && strings.HasSuffix(rest, suffix) {
		rest = rest[:len(rest)-len(suffix)]
	}
	return rest
}

// saveSplittedWorktreeNotes saves individual note files for each worktree.
func saveSplittedWorktreeNotes(pathTemplate string, notes map[string]models.WorktreeNote, env map[string]string) error {
	normalized := normalizeWorktreeNotes(notes)

	existingPaths, err := findSplittedNotePaths(pathTemplate, env)
	if err != nil {
		return err
	}
	for wtName, notePath := range existingPaths {
		if _, ok := normalized[wtName]; ok {
			continue
		}
		if err := os.Remove(notePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}

	// Write each note as an individual file.
	for wtName, note := range normalized {
		notePath := ExpandWithEnv(pathTemplate, cloneEnvWith(env, "WORKTREE_NAME", wtName))

		if err := os.MkdirAll(filepath.Dir(notePath), utils.DefaultDirPerms); err != nil {
			return err
		}
		data := FormatNoteFile(note)
		if err := os.WriteFile(notePath, data, defaultFilePerms); err != nil {
			return err
		}
	}

	return nil
}

// DeleteSplittedNoteFile removes the note file for a specific worktree.
func DeleteSplittedNoteFile(pathTemplate, wtName string, env map[string]string) error {
	notePath := ExpandWithEnv(pathTemplate, cloneEnvWith(env, "WORKTREE_NAME", wtName))

	if err := os.Remove(notePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
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
