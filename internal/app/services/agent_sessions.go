package services

import (
	"bufio"
	"encoding/json"
	"io/fs"
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
	claudeSessionSchema  = "claude-jsonl-v1"
	piSessionSchema      = "pi-jsonl-v1"
)

type agentSessionCacheEntry struct {
	mtime   time.Time
	session *models.AgentSession
}

// AgentAdapter abstracts per-agent transcript discovery so new agent types can be added without changing core logic.
type AgentAdapter interface {
	Name() string
	WatchRoot() string
	Discover(
		seen map[string]struct{},
		cached func(path string, parse func() (*models.AgentSession, error)) (*models.AgentSession, error),
	) ([]*models.AgentSession, error)
}

type transcriptAgentAdapter struct {
	name  string
	root  string
	parse func(path, encodedDir string) (*models.AgentSession, error)
}

func (a *transcriptAgentAdapter) Name() string {
	if a == nil {
		return ""
	}
	return a.name
}

func (a *transcriptAgentAdapter) WatchRoot() string {
	if a == nil {
		return ""
	}
	return a.root
}

func (a *transcriptAgentAdapter) Discover(
	seen map[string]struct{},
	cached func(path string, parse func() (*models.AgentSession, error)) (*models.AgentSession, error),
) ([]*models.AgentSession, error) {
	if a == nil {
		return nil, nil
	}
	return discoverSessionsFromDir(a.root, seen, a.parse, cached)
}

// AgentSessionService discovers Claude and pi transcript sessions from disk.
type AgentSessionService struct {
	mu         sync.RWMutex
	cache      map[string]agentSessionCacheEntry
	sessions   []*models.AgentSession
	claudeRoot string
	piRoot     string
	adapters   []AgentAdapter
	store      SessionRegistryStore
	hooks      *AgentHookService
	logf       func(string, ...any)
}

// NewAgentSessionService builds a service using the default agent transcript locations.
func NewAgentSessionService(logf func(string, ...any)) *AgentSessionService {
	return NewAgentSessionServiceWithStore(claudeProjectsDir(), piSessionsDir(), newFileSessionRegistryStore(), logf)
}

// NewAgentSessionServiceFromConfig builds a service using config values when non-empty,
// falling back to the default agent transcript locations.
func NewAgentSessionServiceFromConfig(claudeRoot, piRoot string, logf func(string, ...any)) *AgentSessionService {
	if claudeRoot == "" {
		claudeRoot = claudeProjectsDir()
	}
	if piRoot == "" {
		piRoot = piSessionsDir()
	}
	return NewAgentSessionServiceWithStore(claudeRoot, piRoot, newFileSessionRegistryStore(), logf)
}

// NewAgentSessionServiceWithRoots builds a service with explicit roots for tests.
func NewAgentSessionServiceWithRoots(claudeRoot, piRoot string, logf func(string, ...any)) *AgentSessionService {
	return NewAgentSessionServiceWithStore(claudeRoot, piRoot, newFileSessionRegistryStore(), logf)
}

// NewAgentSessionServiceWithStore builds a service with explicit roots and registry storage.
func NewAgentSessionServiceWithStore(claudeRoot, piRoot string, store SessionRegistryStore, logf func(string, ...any)) *AgentSessionService {
	return &AgentSessionService{
		cache:      make(map[string]agentSessionCacheEntry),
		claudeRoot: claudeRoot,
		piRoot:     piRoot,
		adapters: []AgentAdapter{
			&transcriptAgentAdapter{name: "claude", root: claudeRoot, parse: parseClaudeSession},
			&transcriptAgentAdapter{name: "pi", root: piRoot, parse: parsePiSession},
		},
		store: store,
		logf:  logf,
	}
}

// WatchRoots returns the directories that should be watched for transcript changes.
func (s *AgentSessionService) WatchRoots() []string {
	roots := make([]string, 0, 2)
	for _, adapter := range s.adapters {
		if adapter == nil {
			continue
		}
		root := strings.TrimSpace(adapter.WatchRoot())
		if root == "" {
			continue
		}
		roots = append(roots, root)
	}
	return roots
}

// Refresh re-discovers all transcript sessions and updates the cache.
func (s *AgentSessionService) Refresh() ([]*models.AgentSession, error) {
	return s.RefreshWithProcesses(nil)
}

// RefreshWithProcesses re-discovers transcript sessions and enriches them with live-process matches.
func (s *AgentSessionService) RefreshWithProcesses(processes []*AgentProcess) ([]*models.AgentSession, error) {
	previous := map[string]*models.AgentSession{}
	if s.store != nil {
		loaded, err := s.store.Load()
		if err != nil {
			if s.logf != nil {
				s.logf("agent sessions: registry load failed: %v", err)
			}
		} else {
			previous = loaded
		}
	}

	seen := make(map[string]struct{})
	sessions := make([]*models.AgentSession, 0, 16)
	for _, adapter := range s.adapters {
		if adapter == nil {
			continue
		}
		discovered, err := adapter.Discover(seen, s.cachedSession)
		if err != nil {
			if s.logf != nil {
				s.logf("agent sessions: %s discovery failed: %v", adapter.Name(), err)
			}
			continue
		}
		sessions = append(sessions, discovered...)
	}

	s.pruneCache(seen)
	now := time.Now()
	sessions = s.mergeWithRegistryFallbacks(sessions, previous, seen, now)
	var hookStates []AgentHookState
	hooks := s.hookService()
	if hooks != nil {
		hooks.Drain()
		hooks.RestoreSessions(previous)
		hookStates = hooks.States()
		sessions = s.applyHookSessions(sessions, hookStates, now)
	}
	sessions = s.classifySessionLiveness(sessions, processes, previous, now)
	if hooks != nil {
		s.applyHookLiveness(sessions, hooks, hookStates, now)
	}
	sort.Slice(sessions, func(i, j int) bool {
		if agentLivenessRank(sessions[i].LivenessState) != agentLivenessRank(sessions[j].LivenessState) {
			return agentLivenessRank(sessions[i].LivenessState) > agentLivenessRank(sessions[j].LivenessState)
		}
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
	if s.store != nil {
		if err := s.store.Save(sessions); err != nil && s.logf != nil {
			s.logf("agent sessions: registry save failed: %v", err)
		}
	}

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

func discoverSessionsFromDir(
	root string,
	seen map[string]struct{},
	parse func(path, encodedDir string) (*models.AgentSession, error),
	cached func(path string, parse func() (*models.AgentSession, error)) (*models.AgentSession, error),
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
		_ = filepath.WalkDir(filepath.Join(root, entry.Name()), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d == nil || d.IsDir() || filepath.Ext(path) != ".jsonl" {
				return nil
			}
			seen[path] = struct{}{}
			session, err := cached(path, func() (*models.AgentSession, error) {
				return parse(path, entry.Name())
			})
			if err == nil && session != nil {
				sessions = append(sessions, session)
			}
			return nil
		})
	}
	return sessions, nil
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

type claudeJSONLMessage struct {
	Role        string          `json:"role"`
	Model       string          `json:"model"`
	RawContent  json.RawMessage `json:"content"`
	TextContent string          `json:"-"`
	Content     []contentBlock  `json:"-"`
}

type contentBlock struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Text      string          `json:"text"`
	ToolUseID string          `json:"tool_use_id"`
	Input     json.RawMessage `json:"input"`
	Arguments json.RawMessage `json:"arguments"`
}

type claudeEnvelope struct {
	Type      string              `json:"type"`
	CWD       string              `json:"cwd"`
	GitBranch string              `json:"gitBranch"`
	Timestamp string              `json:"timestamp"`
	Message   *claudeJSONLMessage `json:"message"`
	Data      *claudeProgressData `json:"data"`
}

type claudeProgressData struct {
	Type    string                 `json:"type"`
	AgentID string                 `json:"agentId"`
	Message *claudeProgressMessage `json:"message"`
}

type claudeProgressMessage struct {
	Type      string              `json:"type"`
	Timestamp string              `json:"timestamp"`
	Message   *claudeJSONLMessage `json:"message"`
}

type normalizedClaudeEntry struct {
	Type              string
	CWD               string
	GitBranch         string
	Timestamp         string
	Message           *claudeJSONLMessage
	FromAgentProgress bool
}

type pendingClaudeTool struct {
	Block             contentBlock
	Timestamp         time.Time
	Order             int
	FromAgentProgress bool
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
		ID:            strings.TrimSuffix(filepath.Base(path), ".jsonl"),
		Agent:         models.AgentKindClaude,
		JSONLPath:     path,
		LastActivity:  info.ModTime(),
		SchemaVersion: claudeSessionSchema,
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var lastMeaningful *normalizedClaudeEntry
	pendingTools := make(map[string]*pendingClaudeTool)
	pendingToolOrder := 0
	for scanner.Scan() {
		var envelope claudeEnvelope
		if err := json.Unmarshal(scanner.Bytes(), &envelope); err != nil {
			continue
		}

		ts, _ := time.Parse(time.RFC3339Nano, envelope.Timestamp)
		if !ts.IsZero() {
			session.LastActivity = ts
		}
		if session.CWD == "" && envelope.CWD != "" {
			session.CWD = envelope.CWD
		}
		if session.GitBranch == "" && envelope.GitBranch != "" {
			session.GitBranch = envelope.GitBranch
		}
		if envelope.Type == "summary" && !session.LastActivity.IsZero() {
			session.LastSummaryAt = session.LastActivity
		}

		for _, entry := range normalizedClaudeEntries(envelope) {
			if entry.Message != nil {
				entry.Message.parseContent()
			}
			entryTS, _ := time.Parse(time.RFC3339Nano, entry.Timestamp)
			if !entryTS.IsZero() {
				session.LastActivity = entryTS
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
			if entry.Message != nil && !entry.FromAgentProgress {
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
			switch entry.Type {
			case "assistant", "user":
				if !entry.FromAgentProgress {
					copied := entry
					lastMeaningful = &copied
				}
			}
			if entry.Message != nil {
				for i := range entry.Message.Content {
					block := &entry.Message.Content[i]
					switch block.Type {
					case "tool_use":
						if !entry.FromAgentProgress {
							session.LastToolName = block.Name
							session.LastToolAt = entryTS
							if path := extractTargetPath(block.Input); path != "" {
								session.LastTargetPath = path
							}
							if command := extractCommandText(block.Input); command != "" {
								session.LastCommand = command
							}
						}
						if block.ID != "" {
							pendingToolOrder++
							pendingTools[block.ID] = &pendingClaudeTool{
								Block:             *block,
								Timestamp:         entryTS,
								Order:             pendingToolOrder,
								FromAgentProgress: entry.FromAgentProgress,
							}
						}
					case "tool_result":
						if block.ToolUseID != "" {
							delete(pendingTools, block.ToolUseID)
						}
					}
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
			for i := range lastMeaningful.Message.Content {
				block := &lastMeaningful.Message.Content[i]
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
	if pending := newestPendingClaudeTool(pendingTools); pending != nil {
		session.CurrentTool = pending.Block.Name
		session.LastToolName = pending.Block.Name
		session.LastToolAt = pending.Timestamp
		if path := extractTargetPath(pending.Block.Input); path != "" {
			session.LastTargetPath = path
		}
		if command := extractCommandText(pending.Block.Input); command != "" {
			session.LastCommand = command
		}
		if pending.FromAgentProgress {
			session.Status = models.AgentSessionStatusWaitingApproval
		} else {
			session.Status = models.AgentSessionStatusExecutingTool
		}
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
	} else {
		applyAgentStatus(session, role, hasToolUse, toolName, isToolResult)
	}
	session.TaskLabel = deriveAgentTaskLabel(session)
	session.Title = deriveAgentSessionTitle(session)
	session.SessionKey = agentSessionKey(session)
	session.ResumeHint = session.JSONLPath
	return session, nil
}

func normalizedClaudeEntries(envelope claudeEnvelope) []normalizedClaudeEntry {
	entries := make([]normalizedClaudeEntry, 0, 2)
	if envelope.Message != nil && (envelope.Type == "assistant" || envelope.Type == "user") {
		entries = append(entries, normalizedClaudeEntry{
			Type:      envelope.Type,
			CWD:       envelope.CWD,
			GitBranch: envelope.GitBranch,
			Timestamp: envelope.Timestamp,
			Message:   envelope.Message,
		})
	}
	if envelope.Type == "progress" && envelope.Data != nil && envelope.Data.Type == "agent_progress" &&
		envelope.Data.Message != nil && envelope.Data.Message.Message != nil {
		entries = append(entries, normalizedClaudeEntry{
			Type:              envelope.Data.Message.Type,
			CWD:               envelope.CWD,
			GitBranch:         envelope.GitBranch,
			Timestamp:         firstNonEmpty(envelope.Data.Message.Timestamp, envelope.Timestamp),
			Message:           envelope.Data.Message.Message,
			FromAgentProgress: true,
		})
	}
	return entries
}

func newestPendingClaudeTool(pending map[string]*pendingClaudeTool) *pendingClaudeTool {
	var newest *pendingClaudeTool
	for _, tool := range pending {
		if tool == nil {
			continue
		}
		if newest == nil ||
			tool.Timestamp.After(newest.Timestamp) ||
			(tool.Timestamp.Equal(newest.Timestamp) && tool.Order > newest.Order) {
			newest = tool
		}
	}
	return newest
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
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
	var decoded strings.Builder
	decoded.Grow(len(name) + 1)
	for i := 0; i < len(name); i++ {
		switch {
		case name[i] == '-' && i+1 < len(name) && name[i+1] == '-':
			decoded.WriteByte('-')
			i++
		case name[i] == '-':
			decoded.WriteByte(filepath.Separator)
		default:
			decoded.WriteByte(name[i])
		}
	}

	result := decoded.String()
	if result == "" {
		return ""
	}
	if !strings.HasPrefix(result, string(filepath.Separator)) {
		result = string(filepath.Separator) + result
	}
	return filepath.Clean(result)
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
		ID:            strings.TrimSuffix(filepath.Base(path), ".jsonl"),
		Agent:         models.AgentKindPi,
		JSONLPath:     path,
		LastActivity:  info.ModTime(),
		SchemaVersion: piSessionSchema,
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
			blocks := parsePiBlocks(entry.Message.Content)
			for i := range blocks {
				block := &blocks[i]
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
		blocks := parsePiBlocks(lastMessage.Message.Content)
		for i := range blocks {
			block := &blocks[i]
			if block.Type == "toolCall" {
				hasToolUse = true
				toolName = normalizePiToolName(block.Name)
				break
			}
		}
	}
	applyAgentStatus(session, role, hasToolUse, toolName, isToolResult)
	session.TaskLabel = deriveAgentTaskLabel(session)
	session.Title = deriveAgentSessionTitle(session)
	session.SessionKey = agentSessionKey(session)
	session.ResumeHint = session.JSONLPath
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
	for i := range blocks {
		b := &blocks[i]
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
		if status == models.AgentSessionStatusWaitingApproval {
			return models.AgentActivityApproval
		}
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
	if status == models.AgentSessionStatusWaitingApproval {
		if isOpen {
			return models.AgentActivityApproval
		}
		if now.Sub(lastActivity) < agentWaitingTimeout {
			return models.AgentActivityApproval
		}
		return models.AgentActivityIdle
	}
	if status == models.AgentSessionStatusExecutingTool && isOpen {
		return toolActivity(currentTool)
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

func (s *AgentSessionService) mergeWithRegistryFallbacks(
	observed []*models.AgentSession,
	previous map[string]*models.AgentSession,
	seen map[string]struct{},
	now time.Time,
) []*models.AgentSession {
	if len(previous) == 0 {
		return observed
	}

	merged := cloneAgentSessions(observed)
	byPath := make(map[string]struct{}, len(merged))
	for _, session := range merged {
		if session == nil {
			continue
		}
		if session.SessionKey == "" {
			session.SessionKey = agentSessionKey(session)
		}
		session.Title = deriveAgentSessionTitle(session)
		session.ResumeHint = session.JSONLPath
		if prev := previous[session.SessionKey]; prev != nil {
			if session.LastObservedAt.IsZero() {
				session.LastObservedAt = prev.LastObservedAt
			}
			if session.LastObservedAt.Before(prev.LastObservedAt) {
				session.LastObservedAt = prev.LastObservedAt
			}
		}
		if strings.TrimSpace(session.JSONLPath) != "" {
			byPath[filepath.Clean(session.JSONLPath)] = struct{}{}
		}
	}

	for _, session := range previous {
		if session == nil {
			continue
		}
		hookBacked := session.SchemaVersion == models.AgentHookEventSchemaVersion
		if hookBacked {
			observation := sessionObservationTime(session)
			if observation.IsZero() || now.Sub(observation) > agentHookStaleAfter {
				continue
			}
		} else if strings.TrimSpace(session.JSONLPath) == "" {
			continue
		}
		cleanPath := filepath.Clean(session.JSONLPath)
		if !hookBacked {
			if _, ok := seen[cleanPath]; !ok {
				continue
			}
		}
		if _, ok := byPath[cleanPath]; ok && cleanPath != "." {
			continue
		}
		fallback := cloneAgentSession(session)
		fallback.SessionKey = agentSessionKey(fallback)
		fallback.Title = deriveAgentSessionTitle(fallback)
		if strings.TrimSpace(fallback.ResumeHint) == "" {
			fallback.ResumeHint = fallback.JSONLPath
		}
		merged = append(merged, fallback)
	}
	return merged
}

func (s *AgentSessionService) classifySessionLiveness(
	sessions []*models.AgentSession,
	processes []*AgentProcess,
	previous map[string]*models.AgentSession,
	now time.Time,
) []*models.AgentSession {
	matched := cloneAgentSessions(sessions)
	for _, session := range matched {
		if session == nil {
			continue
		}
		if session.SessionKey == "" {
			session.SessionKey = agentSessionKey(session)
		}
		if session.Title == "" {
			session.Title = deriveAgentSessionTitle(session)
		}
		if strings.TrimSpace(session.ResumeHint) == "" {
			session.ResumeHint = session.JSONLPath
		}
		if prev := previous[session.SessionKey]; prev != nil && session.LastObservedAt.Before(prev.LastObservedAt) {
			session.LastObservedAt = prev.LastObservedAt
		}
		if session.LastObservedAt.Before(session.LastActivity) {
			session.LastObservedAt = session.LastActivity
		}
		session.IsOpen = false
		session.OpenConfidence = models.AgentOpenConfidenceNone
		session.LivenessState = models.AgentSessionLivenessInactive
		session.LivenessSource = models.AgentSessionLivenessSourceNone
	}
	if len(matched) == 0 {
		return nil
	}

	processes = cloneAgentProcesses(processes)
	usedProcess := make(map[int]struct{}, len(processes))
	exactMatched := make(map[string]struct{}, len(matched))

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
			session.LivenessState = models.AgentSessionLivenessActive
			session.LivenessSource = models.AgentSessionLivenessSourceExactFile
			session.LastObservedAt = now
			usedProcess[process.PID] = struct{}{}
			exactMatched[session.SessionKey] = struct{}{}
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
			if session == nil || session.Agent != process.Agent {
				continue
			}
			if _, ok := exactMatched[session.SessionKey]; ok {
				continue
			}
			score := agentSessionCWDMatchScore(process.CWD, session.CWD)
			if score == 0 {
				continue
			}
			if bestIndex == -1 ||
				score > bestScore ||
				(score == bestScore && sessionObservationTime(session).After(sessionObservationTime(matched[bestIndex]))) ||
				(score == bestScore && sessionObservationTime(session).Equal(sessionObservationTime(matched[bestIndex])) &&
					session.LastActivity.After(matched[bestIndex].LastActivity)) {
				bestIndex = i
				bestScore = score
			}
		}
		if bestIndex >= 0 {
			candidate := matched[bestIndex]
			candidate.LivenessState = models.AgentSessionLivenessSuspect
			candidate.LivenessSource = models.AgentSessionLivenessSourceCWDHeuristic
			candidate.OpenConfidence = models.AgentOpenConfidenceCWD
			candidate.LastObservedAt = now
			usedProcess[process.PID] = struct{}{}
		}
	}

	for _, session := range matched {
		if session == nil {
			continue
		}
		if session.LivenessState == models.AgentSessionLivenessActive || session.LivenessState == models.AgentSessionLivenessSuspect {
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
			continue
		}

		observation := sessionObservationTime(session)
		if !observation.IsZero() && now.Sub(observation) <= agentRecentThreshold {
			session.LivenessState = models.AgentSessionLivenessRecent
			session.LivenessSource = models.AgentSessionLivenessSourceRegistry
		} else {
			session.LivenessState = models.AgentSessionLivenessInactive
			session.LivenessSource = models.AgentSessionLivenessSourceNone
		}
		session.Activity = resolveAgentActivity(
			session.LastSummaryAt,
			session.LastToolAt,
			session.LastToolName,
			session.CurrentTool,
			false,
			session.Status,
			session.LastActivity,
			now,
		)
	}

	return matched
}

func agentLivenessRank(state models.AgentSessionLiveness) int {
	switch state {
	case models.AgentSessionLivenessActive:
		return 4
	case models.AgentSessionLivenessRecent:
		return 3
	case models.AgentSessionLivenessSuspect:
		return 2
	case models.AgentSessionLivenessInactive:
		return 1
	default:
		return 0
	}
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
	service := &AgentSessionService{}
	return service.classifySessionLiveness(sessions, processes, nil, time.Now())
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
