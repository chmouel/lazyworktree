package services

import (
	"encoding/json"
	"os"
	"path/filepath"

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
