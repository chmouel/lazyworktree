package services

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chmouel/lazyworktree/internal/models"
)

const (
	agentActivityTimeout = 30 * time.Second
	agentWaitingTimeout  = 2 * time.Minute
)

type agentSessionCacheEntry struct {
	mtime   time.Time
	session *models.AgentSession
}

// AgentSessionService discovers Claude and pi transcript sessions from disk.
type AgentSessionService struct {
	mu         sync.RWMutex
	cache      map[string]agentSessionCacheEntry
	sessions   []*models.AgentSession
	claudeRoot string
	piRoot     string
	logf       func(string, ...any)
}

// NewAgentSessionService builds a service using the default agent transcript locations.
func NewAgentSessionService(logf func(string, ...any)) *AgentSessionService {
	return NewAgentSessionServiceWithRoots(claudeProjectsDir(), piSessionsDir(), logf)
}

// NewAgentSessionServiceWithRoots builds a service with explicit roots for tests.
func NewAgentSessionServiceWithRoots(claudeRoot, piRoot string, logf func(string, ...any)) *AgentSessionService {
	return &AgentSessionService{
		cache:      make(map[string]agentSessionCacheEntry),
		claudeRoot: claudeRoot,
		piRoot:     piRoot,
		logf:       logf,
	}
}

// WatchRoots returns the directories that should be watched for transcript changes.
func (s *AgentSessionService) WatchRoots() []string {
	roots := make([]string, 0, 2)
	if s.claudeRoot != "" {
		roots = append(roots, s.claudeRoot)
	}
	if s.piRoot != "" {
		roots = append(roots, s.piRoot)
	}
	return roots
}

// Refresh re-discovers all transcript sessions and updates the cache.
func (s *AgentSessionService) Refresh() ([]*models.AgentSession, error) {
	return s.RefreshWithProcesses(nil)
}

// RefreshWithProcesses re-discovers transcript sessions and enriches them with live-process matches.
func (s *AgentSessionService) RefreshWithProcesses(processes []*AgentProcess) ([]*models.AgentSession, error) {
	seen := make(map[string]struct{})
	sessions := make([]*models.AgentSession, 0, 16)

	if claudeSessions, err := s.discoverClaudeSessions(seen); err == nil {
		sessions = append(sessions, claudeSessions...)
	} else if s.logf != nil {
		s.logf("agent sessions: Claude discovery failed: %v", err)
	}

	if piSessions, err := s.discoverPiSessions(seen); err == nil {
		sessions = append(sessions, piSessions...)
	} else if s.logf != nil {
		s.logf("agent sessions: pi discovery failed: %v", err)
	}

	s.pruneCache(seen)
	sessions = matchAgentProcessesToSessions(sessions, processes)
	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].IsOpen != sessions[j].IsOpen {
			return sessions[i].IsOpen
		}
		if agentOpenConfidenceRank(sessions[i].OpenConfidence) != agentOpenConfidenceRank(sessions[j].OpenConfidence) {
			return agentOpenConfidenceRank(sessions[i].OpenConfidence) > agentOpenConfidenceRank(sessions[j].OpenConfidence)
		}
		if sessions[i].LastActivity.Equal(sessions[j].LastActivity) {
			return sessions[i].CWD < sessions[j].CWD
		}
		return sessions[i].LastActivity.After(sessions[j].LastActivity)
	})

	s.mu.Lock()
	s.sessions = sessions
	out := cloneAgentSessions(s.sessions)
	s.mu.Unlock()
	return out, nil
}

// Sessions returns the last discovered sessions.
func (s *AgentSessionService) Sessions() []*models.AgentSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneAgentSessions(s.sessions)
}

// SessionsForWorktree returns sessions whose cwd is the selected worktree or a child directory.
func (s *AgentSessionService) SessionsForWorktree(worktreePath string) []*models.AgentSession {
	base := filepath.Clean(strings.TrimSpace(worktreePath))
	if base == "" {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	matching := make([]*models.AgentSession, 0, len(s.sessions))
	for _, session := range s.sessions {
		if session == nil {
			continue
		}
		cwd := filepath.Clean(strings.TrimSpace(session.CWD))
		if cwd == "" {
			continue
		}
		if cwd == base || strings.HasPrefix(cwd, base+string(filepath.Separator)) {
			matching = append(matching, cloneAgentSession(session))
		}
	}
	return matching
}

func (s *AgentSessionService) discoverSessionsFromDir(
	root string,
	seen map[string]struct{},
	parse func(path, encodedDir string) (*models.AgentSession, error),
) ([]*models.AgentSession, error) {
	if root == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	sessions := make([]*models.AgentSession, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		jsonlFiles, _ := filepath.Glob(filepath.Join(root, entry.Name(), "*.jsonl"))
		for _, path := range jsonlFiles {
			seen[path] = struct{}{}
			session, err := s.cachedSession(path, func() (*models.AgentSession, error) {
				return parse(path, entry.Name())
			})
			if err == nil && session != nil {
				sessions = append(sessions, session)
			}
		}
	}
	return sessions, nil
}

func (s *AgentSessionService) discoverClaudeSessions(seen map[string]struct{}) ([]*models.AgentSession, error) {
	return s.discoverSessionsFromDir(s.claudeRoot, seen, parseClaudeSession)
}

func (s *AgentSessionService) discoverPiSessions(seen map[string]struct{}) ([]*models.AgentSession, error) {
	return s.discoverSessionsFromDir(s.piRoot, seen, parsePiSession)
}

func (s *AgentSessionService) cachedSession(path string, parse func() (*models.AgentSession, error)) (*models.AgentSession, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	mtime := info.ModTime()

	s.mu.RLock()
	entry, ok := s.cache[path]
	s.mu.RUnlock()
	if ok && entry.mtime.Equal(mtime) {
		return cloneAgentSession(entry.session), nil
	}

	session, err := parse()
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, nil
	}

	s.mu.Lock()
	s.cache[path] = agentSessionCacheEntry{mtime: mtime, session: cloneAgentSession(session)}
	s.mu.Unlock()
	return session, nil
}

func (s *AgentSessionService) pruneCache(seen map[string]struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for path := range s.cache {
		if _, ok := seen[path]; !ok {
			delete(s.cache, path)
		}
	}
}

func claudeProjectsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "projects")
}

func piSessionsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".pi", "agent", "sessions")
}

type claudeJSONLEntry struct {
	Type      string              `json:"type"`
	SessionID string              `json:"sessionId"`
	CWD       string              `json:"cwd"`
	GitBranch string              `json:"gitBranch"`
	Timestamp string              `json:"timestamp"`
	Message   *claudeJSONLMessage `json:"message"`
}

type claudeJSONLMessage struct {
	Role        string          `json:"role"`
	Model       string          `json:"model"`
	RawContent  json.RawMessage `json:"content"`
	TextContent string          `json:"-"`
	Content     []contentBlock  `json:"-"`
}

type contentBlock struct {
	Type      string          `json:"type"`
	Name      string          `json:"name"`
	Text      string          `json:"text"`
	Input     json.RawMessage `json:"input"`
	Arguments json.RawMessage `json:"arguments"`
}

func (m *claudeJSONLMessage) parseContent() {
	if len(m.RawContent) == 0 {
		return
	}
	switch m.RawContent[0] {
	case '"':
		_ = json.Unmarshal(m.RawContent, &m.TextContent)
	case '[':
		_ = json.Unmarshal(m.RawContent, &m.Content)
	}
}

func parseClaudeSession(path, encodedDir string) (*models.AgentSession, error) {
	//nolint:gosec // Transcript paths come from local agent directories discovered by the application.
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	session := &models.AgentSession{
		ID:           strings.TrimSuffix(filepath.Base(path), ".jsonl"),
		Agent:        models.AgentKindClaude,
		JSONLPath:    path,
		LastActivity: info.ModTime(),
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var lastMeaningful *claudeJSONLEntry
	for scanner.Scan() {
		var entry claudeJSONLEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if entry.Message != nil {
			entry.Message.parseContent()
		}

		ts, _ := time.Parse(time.RFC3339Nano, entry.Timestamp)
		if !ts.IsZero() {
			session.LastActivity = ts
		}
		if session.CWD == "" && entry.CWD != "" {
			session.CWD = entry.CWD
		}
		if session.GitBranch == "" && entry.GitBranch != "" {
			session.GitBranch = entry.GitBranch
		}
		if entry.Message != nil && entry.Message.Model != "" {
			session.Model = entry.Message.Model
		}
		if entry.Message != nil {
			switch entry.Type {
			case "user":
				if text := firstClaudeText(entry.Message); text != "" {
					session.LastPromptText = text
				}
			case "assistant":
				if text := firstClaudeText(entry.Message); text != "" {
					session.LastReplyText = text
				}
			}
		}
		if entry.Type == "summary" && !session.LastActivity.IsZero() {
			session.LastSummaryAt = session.LastActivity
		}
		switch entry.Type {
		case "assistant", "user":
			lastMeaningful = &entry
		}
		if entry.Message != nil {
			for _, block := range entry.Message.Content {
				if block.Type != "tool_use" {
					continue
				}
				session.LastToolName = block.Name
				session.LastToolAt = ts
				if path := extractTargetPath(block.Input); path != "" {
					session.LastTargetPath = path
				}
				if command := extractCommandText(block.Input); command != "" {
					session.LastCommand = command
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if session.CWD == "" {
		session.CWD = decodeClaudeProjectDir(encodedDir)
	}
	var role string
	var hasToolUse, isToolResult bool
	var toolName string
	if lastMeaningful != nil {
		role = lastMeaningful.Type
		if lastMeaningful.Message != nil {
			for _, block := range lastMeaningful.Message.Content {
				if block.Type == "tool_use" {
					hasToolUse = true
					toolName = block.Name
					break
				}
				if block.Type == "tool_result" {
					isToolResult = true
				}
			}
		}
	}
	applyAgentStatus(session, role, hasToolUse, toolName, isToolResult)
	session.TaskLabel = deriveAgentTaskLabel(session)
	return session, nil
}

func applyAgentStatus(session *models.AgentSession, role string, hasToolUse bool, toolName string, isToolResult bool) {
	if session == nil {
		return
	}
	status := models.AgentSessionStatusUnknown
	switch role {
	case "assistant":
		status = models.AgentSessionStatusWaitingForUser
		if hasToolUse {
			status = models.AgentSessionStatusExecutingTool
			session.CurrentTool = toolName
		}
	case "user":
		if isToolResult {
			status = models.AgentSessionStatusProcessingResult
		} else {
			status = models.AgentSessionStatusThinking
		}
	}
	session.Status = status
	session.Activity = resolveAgentActivity(
		session.LastSummaryAt,
		session.LastToolAt,
		session.LastToolName,
		session.CurrentTool,
		session.IsOpen,
		session.Status,
		session.LastActivity,
		time.Now(),
	)
}

func decodeClaudeProjectDir(name string) string {
	if name == "" {
		return ""
	}
	return "/" + strings.ReplaceAll(name, "-", "/")
}

type piEntry struct {
	Type      string     `json:"type"`
	Timestamp string     `json:"timestamp"`
	CWD       string     `json:"cwd"`
	ModelID   string     `json:"modelId"`
	Name      string     `json:"name"`
	Message   *piMessage `json:"message"`
}

type piMessage struct {
	Role    string          `json:"role"`
	Model   string          `json:"model"`
	Content json.RawMessage `json:"content"`
}

func parsePiSession(path, encodedDir string) (*models.AgentSession, error) {
	//nolint:gosec // Transcript paths come from local agent directories discovered by the application.
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	session := &models.AgentSession{
		ID:           strings.TrimSuffix(filepath.Base(path), ".jsonl"),
		Agent:        models.AgentKindPi,
		JSONLPath:    path,
		LastActivity: info.ModTime(),
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var lastMessage *piEntry
	for scanner.Scan() {
		var entry piEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		ts, _ := time.Parse(time.RFC3339Nano, entry.Timestamp)
		if !ts.IsZero() {
			session.LastActivity = ts
		}

		switch entry.Type {
		case "session":
			if session.CWD == "" && entry.CWD != "" {
				session.CWD = entry.CWD
			}
		case "session_info":
			if entry.Name != "" {
				session.DisplayName = entry.Name
			}
		case "model_change":
			if entry.ModelID != "" {
				session.Model = entry.ModelID
			}
		case "compaction":
			session.LastSummaryAt = ts
		case "message":
			if entry.Message == nil {
				continue
			}
			copied := entry
			lastMessage = &copied
			if entry.Message.Model != "" {
				session.Model = entry.Message.Model
			}
			if text := firstPiText(entry.Message); text != "" {
				switch entry.Message.Role {
				case "user":
					session.LastPromptText = text
				case "assistant":
					session.LastReplyText = text
				}
			}
			for _, block := range parsePiBlocks(entry.Message.Content) {
				if block.Type != "toolCall" {
					continue
				}
				session.LastToolName = normalizePiToolName(block.Name)
				session.LastToolAt = ts
				if path := extractTargetPath(block.Arguments); path != "" {
					session.LastTargetPath = path
				}
				if command := extractCommandText(block.Arguments); command != "" {
					session.LastCommand = command
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if session.CWD == "" {
		session.CWD = decodePiSessionDir(encodedDir)
	}
	var role string
	var hasToolUse, isToolResult bool
	var toolName string
	if lastMessage != nil && lastMessage.Message != nil {
		role = lastMessage.Message.Role
		if role == "toolResult" {
			role = "user"
			isToolResult = true
		}
		for _, block := range parsePiBlocks(lastMessage.Message.Content) {
			if block.Type == "toolCall" {
				hasToolUse = true
				toolName = normalizePiToolName(block.Name)
				break
			}
		}
	}
	applyAgentStatus(session, role, hasToolUse, toolName, isToolResult)
	session.TaskLabel = deriveAgentTaskLabel(session)
	return session, nil
}

func parsePiBlocks(raw json.RawMessage) []contentBlock {
	if len(raw) == 0 || raw[0] == '"' {
		return nil
	}
	var blocks []contentBlock
	_ = json.Unmarshal(raw, &blocks)
	return blocks
}

func normalizePiToolName(name string) string {
	switch name {
	case "bash":
		return "Bash"
	case "read":
		return "Read"
	case "write":
		return "Write"
	case "edit":
		return "Edit"
	case "web_search":
		return "WebSearch"
	case "find":
		return "Glob"
	case "process":
		return "Bash"
	case "subagent":
		return "Agent"
	case "lsp":
		return "Grep"
	default:
		if name == "" {
			return ""
		}
		return strings.ToUpper(name[:1]) + name[1:]
	}
}

func decodePiSessionDir(name string) string {
	if strings.HasPrefix(name, "--") && strings.HasSuffix(name, "--") && len(name) > 4 {
		name = name[2 : len(name)-2]
	}
	return "/" + strings.ReplaceAll(name, "-", "/")
}

func scanForText(blocks []contentBlock) string {
	for _, b := range blocks {
		if b.Type == "text" && strings.TrimSpace(b.Text) != "" {
			return compactWhitespace(b.Text)
		}
	}
	return ""
}

func firstClaudeText(message *claudeJSONLMessage) string {
	if message == nil {
		return ""
	}
	if strings.TrimSpace(message.TextContent) != "" {
		return compactWhitespace(message.TextContent)
	}
	return scanForText(message.Content)
}

func firstPiText(message *piMessage) string {
	if message == nil || len(message.Content) == 0 {
		return ""
	}
	if message.Content[0] == '"' {
		var text string
		if err := json.Unmarshal(message.Content, &text); err == nil {
			return compactWhitespace(text)
		}
		return ""
	}
	return scanForText(parsePiBlocks(message.Content))
}

func extractTargetPath(raw json.RawMessage) string {
	var obj struct {
		FilePath string `json:"file_path"`
		Path     string `json:"path"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	if strings.TrimSpace(obj.FilePath) != "" {
		return obj.FilePath
	}
	return obj.Path
}

func extractCommandText(raw json.RawMessage) string {
	var obj struct {
		Command     string `json:"command"`
		Cmd         string `json:"cmd"`
		Commands    string `json:"commands"`
		Code        string `json:"code"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	for _, candidate := range []string{obj.Command, obj.Cmd, obj.Code, obj.Commands, obj.Description} {
		if strings.TrimSpace(candidate) != "" {
			return compactWhitespace(candidate)
		}
	}
	return ""
}

func deriveAgentTaskLabel(session *models.AgentSession) string {
	if session == nil {
		return ""
	}
	if summary := summarizeCommand(session.LastCommand); summary != "" {
		return "running " + summary
	}
	if path := summarizePath(session.LastTargetPath); path != "" {
		switch normalized := normalizeToolAction(session.CurrentTool, session.LastToolName); normalized {
		case "reading":
			return "reading " + path
		case "editing":
			return "editing " + path
		case "searching":
			return "searching " + path
		default:
			return "working on " + path
		}
	}
	if text := summarizeText(session.LastPromptText); text != "" {
		return "working on " + text
	}
	if text := summarizeText(session.LastReplyText); text != "" {
		return "working on " + text
	}
	return ""
}

func normalizeToolAction(currentTool, lastTool string) string {
	tool := strings.TrimSpace(currentTool)
	if tool == "" {
		tool = strings.TrimSpace(lastTool)
	}
	switch tool {
	case "Read":
		return "reading"
	case "Write", "Edit", "NotebookEdit":
		return "editing"
	case "Glob", "Grep":
		return "searching"
	default:
		return ""
	}
}

func summarizePath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	path = filepath.Clean(strings.TrimSpace(path))
	if home, err := os.UserHomeDir(); err == nil {
		home = filepath.Clean(home)
		if path == home {
			path = "~"
		} else if strings.HasPrefix(path, home+string(filepath.Separator)) {
			path = "~" + strings.TrimPrefix(path, home)
		}
	}
	if len(path) > 80 {
		path = "…" + path[len(path)-79:]
	}
	return path
}

func summarizeCommand(command string) string {
	command = compactWhitespace(command)
	if command == "" {
		return ""
	}
	for _, line := range strings.Split(command, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "<parameter") {
			continue
		}
		if len(line) > 80 {
			return line[:79] + "…"
		}
		return line
	}
	return ""
}

func summarizeText(text string) string {
	text = compactWhitespace(text)
	if text == "" {
		return ""
	}
	text = strings.Trim(text, " .,:;!?")
	for _, prefix := range []string{"Please ", "please ", "Could you ", "could you ", "Can you ", "can you "} {
		text = strings.TrimPrefix(text, prefix)
	}
	if len(text) > 72 {
		return text[:71] + "…"
	}
	return text
}

func compactWhitespace(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func resolveAgentActivity(lastSummaryAt, lastToolAt time.Time, lastToolName, currentTool string, isOpen bool, status models.AgentSessionStatus, lastActivity, now time.Time) models.AgentActivity {
	if !lastSummaryAt.IsZero() && now.Sub(lastSummaryAt) < agentActivityTimeout {
		return models.AgentActivityCompacting
	}
	if !lastToolAt.IsZero() && now.Sub(lastToolAt) < agentActivityTimeout {
		return toolActivity(lastToolName)
	}

	if status == models.AgentSessionStatusWaitingForUser {
		if isOpen {
			return models.AgentActivityWaiting
		}
		if now.Sub(lastActivity) < agentWaitingTimeout {
			return models.AgentActivityWaiting
		}
		return models.AgentActivityIdle
	}

	if lastActivity.IsZero() || now.Sub(lastActivity) > agentActivityTimeout {
		return models.AgentActivityIdle
	}

	switch status {
	case models.AgentSessionStatusThinking, models.AgentSessionStatusProcessingResult:
		return models.AgentActivityThinking
	case models.AgentSessionStatusExecutingTool:
		return toolActivity(currentTool)
	default:
		return models.AgentActivityIdle
	}
}

func toolActivity(tool string) models.AgentActivity {
	switch tool {
	case "Read":
		return models.AgentActivityReading
	case "Write", "Edit", "NotebookEdit":
		return models.AgentActivityWriting
	case "Bash":
		return models.AgentActivityRunning
	case "Glob", "Grep":
		return models.AgentActivitySearching
	case "WebFetch", "WebSearch":
		return models.AgentActivityBrowsing
	case "Agent":
		return models.AgentActivitySpawning
	default:
		if tool != "" {
			return models.AgentActivityRunning
		}
		return models.AgentActivityIdle
	}
}

func cloneAgentSessions(in []*models.AgentSession) []*models.AgentSession {
	if len(in) == 0 {
		return nil
	}
	out := make([]*models.AgentSession, 0, len(in))
	for _, session := range in {
		if session == nil {
			continue
		}
		out = append(out, cloneAgentSession(session))
	}
	return out
}

func cloneAgentSession(in *models.AgentSession) *models.AgentSession {
	if in == nil {
		return nil
	}
	copied := *in
	return &copied
}

func agentOpenConfidenceRank(confidence models.AgentOpenConfidence) int {
	switch confidence {
	case models.AgentOpenConfidenceExact:
		return 2
	case models.AgentOpenConfidenceCWD:
		return 1
	default:
		return 0
	}
}

func matchAgentProcessesToSessions(sessions []*models.AgentSession, processes []*AgentProcess) []*models.AgentSession {
	matched := cloneAgentSessions(sessions)
	if len(matched) == 0 {
		return nil
	}
	for _, session := range matched {
		session.IsOpen = false
		session.OpenConfidence = models.AgentOpenConfidenceNone
	}
	if len(processes) == 0 {
		return matched
	}

	processes = cloneAgentProcesses(processes)
	usedProcess := make(map[int]struct{}, len(processes))
	usedSession := make(map[string]struct{}, len(matched))

	for _, process := range processes {
		if process == nil {
			continue
		}
		for _, session := range matched {
			if session == nil || session.Agent != process.Agent || session.JSONLPath == "" {
				continue
			}
			if !processHasOpenFile(process, session.JSONLPath) {
				continue
			}
			session.IsOpen = true
			session.OpenConfidence = models.AgentOpenConfidenceExact
			usedProcess[process.PID] = struct{}{}
			usedSession[session.ID] = struct{}{}
			break
		}
	}

	for _, process := range processes {
		if process == nil {
			continue
		}
		if _, ok := usedProcess[process.PID]; ok {
			continue
		}
		bestIndex := -1
		bestScore := 0
		for i, session := range matched {
			if session == nil || session.Agent != process.Agent || session.ID == "" {
				continue
			}
			if _, ok := usedSession[session.ID]; ok {
				continue
			}
			score := agentSessionCWDMatchScore(process.CWD, session.CWD)
			if score == 0 {
				continue
			}
			if bestIndex == -1 || score > bestScore || (score == bestScore && session.LastActivity.After(matched[bestIndex].LastActivity)) {
				bestIndex = i
				bestScore = score
			}
		}
		if bestIndex >= 0 {
			matched[bestIndex].IsOpen = true
			matched[bestIndex].OpenConfidence = models.AgentOpenConfidenceCWD
			usedSession[matched[bestIndex].ID] = struct{}{}
		}
	}

	now := time.Now()
	for _, session := range matched {
		if session == nil {
			continue
		}
		session.Activity = resolveAgentActivity(
			session.LastSummaryAt,
			session.LastToolAt,
			session.LastToolName,
			session.CurrentTool,
			session.IsOpen,
			session.Status,
			session.LastActivity,
			now,
		)
	}

	return matched
}

func processHasOpenFile(process *AgentProcess, sessionPath string) bool {
	if process == nil || sessionPath == "" {
		return false
	}
	target := filepath.Clean(sessionPath)
	for _, openFile := range process.OpenFiles {
		if filepath.Clean(openFile) == target {
			return true
		}
	}
	return false
}

func agentSessionCWDMatchScore(processCWD, sessionCWD string) int {
	processCWD = filepath.Clean(strings.TrimSpace(processCWD))
	sessionCWD = filepath.Clean(strings.TrimSpace(sessionCWD))
	if processCWD == "" || sessionCWD == "" {
		return 0
	}
	switch {
	case processCWD == sessionCWD:
		return 3
	case strings.HasPrefix(processCWD, sessionCWD+string(filepath.Separator)):
		return 2
	case strings.HasPrefix(sessionCWD, processCWD+string(filepath.Separator)):
		return 1
	default:
		return 0
	}
}
