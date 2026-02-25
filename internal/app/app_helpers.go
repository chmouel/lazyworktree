package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/app/services"
	"github.com/chmouel/lazyworktree/internal/app/util"
	log "github.com/chmouel/lazyworktree/internal/log"
	"github.com/chmouel/lazyworktree/internal/models"
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
	return services.BuildCommandEnv(branch, wtPath, m.repoKey, m.state.services.git.GetMainWorktreePath(m.ctx))
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
// Only valid git worktrees are saved to prevent stale entries.
func (m *Model) saveCache() {
	repoKey := m.getRepoKey()

	// Filter to only valid git worktrees before saving
	// If validPaths is nil, git service is unavailable - save all worktrees
	validPaths := m.getValidWorktreePaths()
	var validWorktrees []*models.WorktreeInfo
	if validPaths == nil {
		validWorktrees = m.state.data.worktrees
	} else {
		validWorktrees = make([]*models.WorktreeInfo, 0, len(m.state.data.worktrees))
		for _, wt := range m.state.data.worktrees {
			if validPaths[normalizePath(wt.Path)] {
				validWorktrees = append(validWorktrees, wt)
			}
		}
	}

	if err := services.SaveCache(repoKey, m.getWorktreeDir(), validWorktrees); err != nil {
		m.showInfo(fmt.Sprintf("Failed to write cache: %v", err), nil)
	}
}

func (m *Model) newLoadingScreen(message string) *appscreen.LoadingScreen {
	operation := appscreen.TipOperationFromContext(m.loadingOperation, message)
	return appscreen.NewLoadingScreen(message, operation, m.theme, spinnerFrameSet(m.config.IconsEnabled()), m.config.IconsEnabled())
}

func (m *Model) setLoadingScreen(message string) {
	m.state.ui.screenManager.Set(m.newLoadingScreen(message))
}

func (m *Model) updateLoadingMessage(message string) {
	if loadingScreen := m.loadingScreen(); loadingScreen != nil {
		loadingScreen.Message = message
	}
}

func (m *Model) loadingScreen() *appscreen.LoadingScreen {
	if m.state.ui.screenManager.Type() != appscreen.TypeLoading {
		return nil
	}
	loadingScreen, _ := m.state.ui.screenManager.Current().(*appscreen.LoadingScreen)
	return loadingScreen
}

func (m *Model) clearLoadingScreen() {
	if m.state.ui.screenManager.Type() == appscreen.TypeLoading {
		m.state.ui.screenManager.Pop()
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
		m.state.data.accessHistory = history
	}
}

// saveAccessHistory saves access history to file.
func (m *Model) saveAccessHistory() {
	if err := services.SaveAccessHistory(m.getRepoKey(), m.getWorktreeDir(), m.state.data.accessHistory); err != nil {
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
	m.state.data.accessHistory[path] = time.Now().Unix()
	m.saveAccessHistory()
}

func (m *Model) getRepoKey() string {
	if m.repoKey != "" {
		return m.repoKey
	}
	m.repoKeyOnce.Do(func() {
		m.repoKey = m.state.services.git.ResolveRepoName(m.ctx)
	})
	return m.repoKey
}

func (m *Model) windowTitle() string {
	title := "Lazyworktree"
	repoKey := strings.TrimSpace(m.repoKey)
	if repoKey != "" && repoKey != "unknown" && !strings.HasPrefix(repoKey, "local-") {
		title += " — " + repoKey
	}
	if m.state.data.selectedIndex >= 0 && m.state.data.selectedIndex < len(m.state.data.filteredWts) {
		wt := m.state.data.filteredWts[m.state.data.selectedIndex]
		if wt.Branch != "" {
			title += " [" + wt.Branch + "]"
		}
	}
	return title
}

func (m *Model) getMainWorktreePath() string {
	for _, wt := range m.state.data.worktrees {
		if wt.IsMain {
			return wt.Path
		}
	}
	if len(m.state.data.worktrees) > 0 {
		return m.state.data.worktrees[0].Path
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

// normalizePath returns a canonical path for comparison.
// Resolves symlinks and cleans the path to prevent false positives
// when comparing worktree paths.
func normalizePath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(resolved)
}

// getValidWorktreePaths returns a set of paths that git recognizes as valid worktrees.
// Returns nil if git service is unavailable or no worktrees found (allows bypass of validation).
func (m *Model) getValidWorktreePaths() map[string]bool {
	if m.state.services.git == nil {
		return nil
	}

	raw := m.state.services.git.RunGit(m.ctx,
		[]string{"git", "worktree", "list", "--porcelain"},
		"", []int{0}, true, false)

	if raw == "" {
		return nil
	}

	paths := make(map[string]bool)
	for line := range strings.SplitSeq(raw, "\n") {
		if path, found := strings.CutPrefix(line, "worktree "); found {
			paths[normalizePath(path)] = true
		}
	}

	// Return nil if no worktrees found (git command failed or empty repo)
	if len(paths) == 0 {
		return nil
	}

	return paths
}

// findOrphanedWorktreeDirs returns directories in the worktree dir that exist on disk
// but are not registered with git worktree.
func (m *Model) findOrphanedWorktreeDirs() []string {
	repoWorktreeDir := m.getRepoWorktreeDir()
	validPaths := m.getValidWorktreePaths()

	// If validPaths is nil, git service is unavailable - can't determine orphans
	if validPaths == nil {
		return nil
	}

	entries, err := os.ReadDir(repoWorktreeDir)
	if err != nil {
		return nil
	}

	var orphans []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// Skip hidden files/dirs
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		fullPath := filepath.Join(repoWorktreeDir, entry.Name())
		normalizedPath := normalizePath(fullPath)
		if !validPaths[normalizedPath] {
			orphans = append(orphans, fullPath) // Store original for display/deletion
		}
	}
	return orphans
}

// GetSelectedPath returns the selected worktree path for shell integration.
// This is used when the application exits to allow the shell to cd into the selected worktree.
func (m *Model) GetSelectedPath() string {
	return m.selectedPath
}
