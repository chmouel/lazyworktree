package app

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	log "github.com/chmouel/lazyworktree/internal/log"
	"github.com/chmouel/lazyworktree/internal/models"
)

// commandPaletteUsage tracks usage frequency and recency for command palette items.
type commandPaletteUsage struct {
	ID        string `json:"id"`
	Timestamp int64  `json:"timestamp"`
	Count     int    `json:"count"`
}

func (m *Model) debugf(format string, args ...any) {
	log.Printf(format, args...)
}

func (m *Model) pagerCommand() string {
	if m.config != nil {
		if pager := strings.TrimSpace(m.config.Pager); pager != "" {
			return pager
		}
	}
	if pager := strings.TrimSpace(os.Getenv("PAGER")); pager != "" {
		return pager
	}
	if _, err := exec.LookPath("less"); err == nil {
		return "less --use-color -q --wordwrap -qcR -P 'Press q to exit..'"
	}
	if _, err := exec.LookPath("more"); err == nil {
		return "more"
	}
	return "cat"
}

func (m *Model) editorCommand() string {
	if m.config != nil {
		if editor := strings.TrimSpace(m.config.Editor); editor != "" {
			return os.ExpandEnv(editor)
		}
	}
	if editor := strings.TrimSpace(os.Getenv("EDITOR")); editor != "" {
		return editor
	}
	if _, err := exec.LookPath("nvim"); err == nil {
		return "nvim"
	}
	if _, err := exec.LookPath("vi"); err == nil {
		return "vi"
	}
	return ""
}

func (m *Model) pagerEnv(pager string) string {
	if pagerIsLess(pager) {
		return "LESS= LESSHISTFILE=-"
	}
	return ""
}

func pagerIsLess(pager string) bool {
	fields := strings.FieldsSeq(pager)
	for field := range fields {
		if strings.Contains(field, "=") && !strings.HasPrefix(field, "-") && !strings.Contains(field, "/") {
			continue
		}
		return filepath.Base(field) == "less"
	}
	return false
}

func (m *Model) buildCommandEnv(branch, wtPath string) map[string]string {
	return map[string]string{
		"WORKTREE_BRANCH":    branch,
		"MAIN_WORKTREE_PATH": m.git.GetMainWorktreePath(m.ctx),
		"WORKTREE_PATH":      wtPath,
		"WORKTREE_NAME":      filepath.Base(wtPath),
		"REPO_NAME":          m.repoKey,
	}
}

func expandWithEnv(input string, env map[string]string) string {
	if input == "" {
		return ""
	}
	return os.Expand(input, func(key string) string {
		if val, ok := env[key]; ok {
			return val
		}
		return os.Getenv(key)
	})
}

func envMapToList(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	out := make([]string, 0, len(env))
	for key, val := range env {
		out = append(out, fmt.Sprintf("%s=%s", key, val))
	}
	return out
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
	fields := strings.Fields(name)
	if len(fields) == 0 {
		return ""
	}
	if len(fields) == 1 {
		runes := []rune(fields[0])
		if len(runes) <= 2 {
			return string(runes)
		}
		return string(runes[:2])
	}
	first := []rune(fields[0])
	last := []rune(fields[len(fields)-1])
	if len(first) == 0 || len(last) == 0 {
		return ""
	}
	return string([]rune{first[0], last[0]})
}

func parseCommitMeta(raw string) commitMeta {
	parts := strings.Split(raw, "\x1f")
	meta := commitMeta{}
	if len(parts) > 0 {
		meta.sha = parts[0]
	}
	if len(parts) > 1 {
		meta.author = parts[1]
	}
	if len(parts) > 2 {
		meta.email = parts[2]
	}
	if len(parts) > 3 {
		meta.date = parts[3]
	}
	if len(parts) > 4 {
		meta.subject = parts[4]
	}
	if len(parts) > 5 {
		meta.body = strings.Split(parts[5], "\n")
	}
	return meta
}

func sanitizePRURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("PR URL is empty")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid PR URL %q: %w", raw, err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("unsupported URL scheme %q", u.Scheme)
	}

	return u.String(), nil
}

// gitURLToWebURL converts a git remote URL to a web URL.
// Handles both SSH (git@github.com:user/repo.git) and HTTPS (https://github.com/user/repo.git) formats.
func (m *Model) gitURLToWebURL(gitURL string) string {
	gitURL = strings.TrimSpace(gitURL)

	// Remove .git suffix if present
	gitURL = strings.TrimSuffix(gitURL, ".git")

	// Handle SSH format: git@github.com:user/repo
	if strings.HasPrefix(gitURL, "git@") {
		// Extract host and path
		parts := strings.SplitN(gitURL, "@", 2)
		if len(parts) == 2 {
			hostPath := parts[1]
			// Replace : with /
			hostPath = strings.Replace(hostPath, ":", "/", 1)
			return "https://" + hostPath
		}
	}

	// Handle HTTPS format: https://github.com/user/repo
	if strings.HasPrefix(gitURL, "https://") || strings.HasPrefix(gitURL, "http://") {
		return gitURL
	}

	// Handle ssh:// format: ssh://git@github.com/user/repo
	if after, ok := strings.CutPrefix(gitURL, "ssh://"); ok {
		gitURL = after
		// Remove git@ if present
		gitURL = strings.TrimPrefix(gitURL, "git@")
		return "https://" + gitURL
	}

	// Handle git:// format: git://github.com/user/repo
	if strings.HasPrefix(gitURL, "git://") {
		return strings.Replace(gitURL, "git://", "https://", 1)
	}

	return ""
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
		cachePath := filepath.Join(m.getWorktreeDir(), repoKey, models.CacheFilename)
		// #nosec G304 -- cachePath is constructed from vetted worktree directory and constant filename
		data, err := os.ReadFile(cachePath)
		if err != nil {
			return nil
		}

		var payload struct {
			Worktrees []*models.WorktreeInfo `json:"worktrees"`
		}
		if err := json.Unmarshal(data, &payload); err != nil {
			return errMsg{err: err}
		}
		if len(payload.Worktrees) == 0 {
			return nil
		}
		return cachedWorktreesMsg{worktrees: payload.Worktrees}
	}
}

// saveCache saves worktree data to the cache file.
func (m *Model) saveCache() {
	repoKey := m.getRepoKey()
	cachePath := filepath.Join(m.getWorktreeDir(), repoKey, models.CacheFilename)
	if err := os.MkdirAll(filepath.Dir(cachePath), defaultDirPerms); err != nil {
		m.showInfo(fmt.Sprintf("Failed to create cache dir: %v", err), nil)
		return
	}

	cacheData := struct {
		Worktrees []*models.WorktreeInfo `json:"worktrees"`
	}{
		Worktrees: m.worktrees,
	}
	data, _ := json.Marshal(cacheData)
	if err := os.WriteFile(cachePath, data, defaultFilePerms); err != nil {
		m.showInfo(fmt.Sprintf("Failed to write cache: %v", err), nil)
	}
}

// loadCommandHistory loads command history from file.
func (m *Model) loadCommandHistory() {
	repoKey := m.getRepoKey()
	historyPath := filepath.Join(m.getWorktreeDir(), repoKey, models.CommandHistoryFilename)
	// #nosec G304 -- historyPath is constructed from vetted worktree directory and constant filename
	data, err := os.ReadFile(historyPath)
	if err != nil {
		// No history file yet, that's fine
		m.commandHistory = []string{}
		return
	}

	var payload struct {
		Commands []string `json:"commands"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		m.debugf("failed to parse command history: %v", err)
		m.commandHistory = []string{}
		return
	}

	m.commandHistory = payload.Commands
	if m.commandHistory == nil {
		m.commandHistory = []string{}
	}
}

// saveCommandHistory saves command history to file.
func (m *Model) saveCommandHistory() {
	repoKey := m.getRepoKey()
	historyPath := filepath.Join(m.getWorktreeDir(), repoKey, models.CommandHistoryFilename)
	if err := os.MkdirAll(filepath.Dir(historyPath), defaultDirPerms); err != nil {
		m.debugf("failed to create history dir: %v", err)
		return
	}

	historyData := struct {
		Commands []string `json:"commands"`
	}{
		Commands: m.commandHistory,
	}
	data, _ := json.Marshal(historyData)
	if err := os.WriteFile(historyPath, data, defaultFilePerms); err != nil {
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
	repoKey := m.getRepoKey()
	historyPath := filepath.Join(m.getWorktreeDir(), repoKey, models.AccessHistoryFilename)
	// #nosec G304 -- path is constructed from known safe components
	data, err := os.ReadFile(historyPath)
	if err != nil {
		return
	}
	var history map[string]int64
	if err := json.Unmarshal(data, &history); err != nil {
		m.debugf("failed to parse access history: %v", err)
		return
	}
	m.accessHistory = history
}

// saveAccessHistory saves access history to file.
func (m *Model) saveAccessHistory() {
	repoKey := m.getRepoKey()
	historyPath := filepath.Join(m.getWorktreeDir(), repoKey, models.AccessHistoryFilename)
	if err := os.MkdirAll(filepath.Dir(historyPath), defaultDirPerms); err != nil {
		m.debugf("failed to create access history dir: %v", err)
		return
	}
	data, _ := json.Marshal(m.accessHistory)
	if err := os.WriteFile(historyPath, data, defaultFilePerms); err != nil {
		m.debugf("failed to write access history: %v", err)
	}
}

// loadPaletteHistory loads palette usage history from file.
func (m *Model) loadPaletteHistory() {
	repoKey := m.getRepoKey()
	historyPath := filepath.Join(m.getWorktreeDir(), repoKey, models.CommandPaletteHistoryFilename)
	// #nosec G304 -- historyPath is constructed from vetted worktree directory and constant filename
	data, err := os.ReadFile(historyPath)
	if err != nil {
		m.paletteHistory = []commandPaletteUsage{}
		return
	}

	var payload struct {
		Commands []commandPaletteUsage `json:"commands"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		m.debugf("failed to parse palette history: %v", err)
		m.paletteHistory = []commandPaletteUsage{}
		return
	}

	m.paletteHistory = payload.Commands
	if m.paletteHistory == nil {
		m.paletteHistory = []commandPaletteUsage{}
	}
}

// savePaletteHistory saves palette usage history to file.
func (m *Model) savePaletteHistory() {
	repoKey := m.getRepoKey()
	historyPath := filepath.Join(m.getWorktreeDir(), repoKey, models.CommandPaletteHistoryFilename)
	if err := os.MkdirAll(filepath.Dir(historyPath), defaultDirPerms); err != nil {
		m.debugf("failed to create palette history dir: %v", err)
		return
	}

	historyData := struct {
		Commands []commandPaletteUsage `json:"commands"`
	}{
		Commands: m.paletteHistory,
	}
	data, _ := json.Marshal(historyData)
	if err := os.WriteFile(historyPath, data, defaultFilePerms); err != nil {
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
