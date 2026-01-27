package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/chmouel/lazyworktree/internal/app/services"
	"github.com/chmouel/lazyworktree/internal/app/util"
	log "github.com/chmouel/lazyworktree/internal/log"
)

// commandPaletteUsage tracks usage frequency and recency for command palette items.
type commandPaletteUsage = services.CommandPaletteUsage

func (m *Model) debugf(format string, args ...any) {
	log.Printf(format, args...)
}

func (m *Model) pagerCommand() string {
	return services.PagerCommand(m.config)
}

func (m *Model) editorCommand() string {
	return services.EditorCommand(m.config)
}

func (m *Model) pagerEnv(pager string) string {
	return services.PagerEnv(pager)
}

func (m *Model) buildCommandEnv(branch, wtPath string) map[string]string {
	return services.BuildCommandEnv(branch, wtPath, m.repoKey, m.git.GetMainWorktreePath(m.ctx))
}

func expandWithEnv(input string, env map[string]string) string {
	return services.ExpandWithEnv(input, env)
}

func envMapToList(env map[string]string) []string {
	return services.EnvMapToList(env)
}

// filterWorktreeEnvVars filters out worktree-specific environment variables
// to prevent duplicates when building command environments.
func filterWorktreeEnvVars(environ []string) []string {
	worktreeVars := map[string]bool{
		"WORKTREE_PATH":      true,
		"MAIN_WORKTREE_PATH": true,
		"WORKTREE_BRANCH":    true,
		"WORKTREE_NAME":      true,
		"REPO_NAME":          true,
	}

	filtered := make([]string, 0, len(environ))
	for _, entry := range environ {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) > 0 && !worktreeVars[parts[0]] {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func exportEnvCommand(env map[string]string) string {
	if len(env) == 0 {
		return ""
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("export %s=%s;", key, shellQuote(env[key])))
	}
	return strings.Join(parts, " ")
}

// isEscKey checks if the key string represents an escape key.
// Some terminals send ESC as "esc" (tea.KeyEsc) while others send it
// as a raw escape byte "\x1b" (ASCII 27).
func isEscKey(keyStr string) bool {
	return keyStr == keyEsc || keyStr == keyEscRaw
}

func formatCommitMessage(message string) string {
	if len(message) <= commitMessageMaxLength {
		return message
	}
	return message[:commitMessageMaxLength] + "…"
}

func authorInitials(name string) string {
	return util.AuthorInitials(name)
}

func parseCommitMeta(raw string) commitMeta {
	parsed := util.ParseCommitMeta(raw)
	return commitMeta{
		sha:     parsed.SHA,
		author:  parsed.Author,
		email:   parsed.Email,
		date:    parsed.Date,
		subject: parsed.Subject,
		body:    parsed.Body,
	}
}

func sanitizePRURL(raw string) (string, error) {
	return util.SanitizePRURL(raw)
}

// gitURLToWebURL converts a git remote URL to a web URL.
// Handles both SSH (git@github.com:user/repo.git) and HTTPS (https://github.com/user/repo.git) formats.
func (m *Model) gitURLToWebURL(gitURL string) string {
	return util.GitURLToWebURL(gitURL)
}

func filterNonEmpty(values []string) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

// loadCache loads worktree data from the cache file.
func (m *Model) loadCache() tea.Cmd {
	return func() tea.Msg {
		repoKey := m.getRepoKey()
		worktrees, err := services.LoadCache(repoKey, m.getWorktreeDir())
		if err != nil {
			return errMsg{err: err}
		}
		if len(worktrees) == 0 {
			return nil
		}
		return cachedWorktreesMsg{worktrees: worktrees}
	}
}

// saveCache saves worktree data to the cache file.
func (m *Model) saveCache() {
	repoKey := m.getRepoKey()
	if err := services.SaveCache(repoKey, m.getWorktreeDir(), m.worktrees); err != nil {
		m.showInfo(fmt.Sprintf("Failed to write cache: %v", err), nil)
	}
}

// loadCommandHistory loads command history from file.
func (m *Model) loadCommandHistory() {
	history, err := services.LoadCommandHistory(m.getRepoKey(), m.getWorktreeDir())
	if err != nil {
		m.debugf("failed to parse command history: %v", err)
	}
	if history == nil {
		history = []string{}
	}
	m.commandHistory = history
}

// saveCommandHistory saves command history to file.
func (m *Model) saveCommandHistory() {
	if err := services.SaveCommandHistory(m.getRepoKey(), m.getWorktreeDir(), m.commandHistory); err != nil {
		m.debugf("failed to write command history: %v", err)
	}
}

// addToCommandHistory adds a command to history and saves it.
func (m *Model) addToCommandHistory(cmd string) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return
	}

	// Remove duplicate if it exists
	filtered := []string{}
	for _, c := range m.commandHistory {
		if c != cmd {
			filtered = append(filtered, c)
		}
	}

	// Add to front (most recent first)
	m.commandHistory = append([]string{cmd}, filtered...)

	// Limit history to 100 entries
	maxHistory := 100
	if len(m.commandHistory) > maxHistory {
		m.commandHistory = m.commandHistory[:maxHistory]
	}

	m.saveCommandHistory()
}

// loadAccessHistory loads access history from file.
func (m *Model) loadAccessHistory() {
	history, err := services.LoadAccessHistory(m.getRepoKey(), m.getWorktreeDir())
	if err != nil {
		m.debugf("failed to parse access history: %v", err)
		return
	}
	if history != nil {
		m.accessHistory = history
	}
}

// saveAccessHistory saves access history to file.
func (m *Model) saveAccessHistory() {
	if err := services.SaveAccessHistory(m.getRepoKey(), m.getWorktreeDir(), m.accessHistory); err != nil {
		m.debugf("failed to write access history: %v", err)
	}
}

// loadPaletteHistory loads palette usage history from file.
func (m *Model) loadPaletteHistory() {
	history, err := services.LoadPaletteHistory(m.getRepoKey(), m.getWorktreeDir())
	if err != nil {
		m.debugf("failed to parse palette history: %v", err)
	}
	if history == nil {
		history = []commandPaletteUsage{}
	}
	m.paletteHistory = history
}

// savePaletteHistory saves palette usage history to file.
func (m *Model) savePaletteHistory() {
	if err := services.SavePaletteHistory(m.getRepoKey(), m.getWorktreeDir(), m.paletteHistory); err != nil {
		m.debugf("failed to write palette history: %v", err)
	}
}

// addToPaletteHistory adds a command usage to palette history and saves it.
func (m *Model) addToPaletteHistory(id string) {
	id = strings.TrimSpace(id)
	if id == "" {
		return
	}

	m.debugf("adding to palette history: %s", id)
	now := time.Now().Unix()

	// Find existing entry and update it
	found := false
	for i, entry := range m.paletteHistory {
		if entry.ID == id {
			m.paletteHistory[i].Timestamp = now
			m.paletteHistory[i].Count++
			// Move to front
			updated := m.paletteHistory[i]
			m.paletteHistory = append([]commandPaletteUsage{updated}, append(m.paletteHistory[:i], m.paletteHistory[i+1:]...)...)
			found = true
			break
		}
	}

	// Add new entry if not found
	if !found {
		m.paletteHistory = append([]commandPaletteUsage{{
			ID:        id,
			Timestamp: now,
			Count:     1,
		}}, m.paletteHistory...)
	}

	// Limit history to 100 entries
	maxHistory := 100
	if len(m.paletteHistory) > maxHistory {
		m.paletteHistory = m.paletteHistory[:maxHistory]
	}

	m.savePaletteHistory()
}

// recordAccess updates the access timestamp for a worktree path.
func (m *Model) recordAccess(path string) {
	if path == "" {
		return
	}
	m.accessHistory[path] = time.Now().Unix()
	m.saveAccessHistory()
}

func (m *Model) getRepoKey() string {
	if m.repoKey != "" {
		return m.repoKey
	}
	m.repoKeyOnce.Do(func() {
		m.repoKey = m.git.ResolveRepoName(m.ctx)
	})
	return m.repoKey
}

func (m *Model) getMainWorktreePath() string {
	for _, wt := range m.worktrees {
		if wt.IsMain {
			return wt.Path
		}
	}
	if len(m.worktrees) > 0 {
		return m.worktrees[0].Path
	}
	return ""
}

func (m *Model) getWorktreeDir() string {
	if m.config.WorktreeDir != "" {
		return m.config.WorktreeDir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "worktrees")
}

func (m *Model) getRepoWorktreeDir() string {
	return filepath.Join(m.getWorktreeDir(), m.getRepoKey())
}

// GetSelectedPath returns the selected worktree path for shell integration.
// This is used when the application exits to allow the shell to cd into the selected worktree.
func (m *Model) GetSelectedPath() string {
	return m.selectedPath
}
