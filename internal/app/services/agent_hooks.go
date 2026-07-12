package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chmouel/lazyworktree/internal/models"
)

const (
	// agentHookMaxPayloadBytes caps the size of a single hook payload read from stdin.
	agentHookMaxPayloadBytes = 256 * 1024
	// agentHookStaleAfter prunes hook states with no events for this long.
	agentHookStaleAfter = 24 * time.Hour
	// agentHookSpoolFileStaleAfter prunes unconsumed spool files older than this.
	agentHookSpoolFileStaleAfter = 24 * time.Hour
)

// AgentHookSpoolDir returns the directory where hook shims spool events.
func AgentHookSpoolDir() string {
	if xdgStateHome := os.Getenv("XDG_STATE_HOME"); xdgStateHome != "" {
		return filepath.Join(xdgStateHome, "lazyworktree", "agent-events")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".local", "state", "lazyworktree", "agent-events")
	}
	return filepath.Join(home, ".local", "state", "lazyworktree", "agent-events")
}

// ParseAgentHookPayload converts a raw hook stdin payload into a spool event.
func ParseAgentHookPayload(agent models.AgentKind, data []byte) (models.AgentHookEvent, error) {
	var payload struct {
		HookEventName       string `json:"hook_event_name"`
		SessionID           string `json:"session_id"`
		SessionIDCamel      string `json:"sessionId"`
		TranscriptPath      string `json:"transcript_path"`
		TranscriptPathCamel string `json:"transcriptPath"`
		CWD                 string `json:"cwd"`
		Model               string `json:"model"`
		Source              string `json:"source"`
		NotificationType    string `json:"notification_type"`
		NotificationCamel   string `json:"notificationType"`
		ToolName            string `json:"tool_name"`
		ToolNameCamel       string `json:"toolName"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return models.AgentHookEvent{}, fmt.Errorf("decode hook payload: %w", err)
	}
	sessionID := strings.TrimSpace(firstNonEmpty(payload.SessionID, payload.SessionIDCamel))
	if sessionID == "" {
		return models.AgentHookEvent{}, errors.New("hook payload missing session_id")
	}
	eventName := normalizeAgentHookEventName(
		payload.HookEventName,
		firstNonEmpty(payload.NotificationType, payload.NotificationCamel),
		firstNonEmpty(payload.ToolName, payload.ToolNameCamel),
	)
	return models.AgentHookEvent{
		SchemaVersion:  models.AgentHookEventSchemaVersion,
		Agent:          agent,
		HookEventName:  eventName,
		SessionID:      sessionID,
		TranscriptPath: strings.TrimSpace(firstNonEmpty(payload.TranscriptPath, payload.TranscriptPathCamel)),
		CWD:            strings.TrimSpace(payload.CWD),
		Model:          strings.TrimSpace(payload.Model),
		Source:         strings.TrimSpace(payload.Source),
	}, nil
}

func normalizeAgentHookEventName(eventName, notificationType, toolName string) string {
	eventName = strings.TrimSpace(eventName)
	notificationType = strings.TrimSpace(notificationType)
	toolName = strings.TrimSpace(toolName)
	switch eventName {
	case "PreToolUse":
		if toolName == "ask_user" || toolName == "AskUserQuestion" {
			return models.AgentHookWaitingForUser
		}
	case "Notification":
		switch notificationType {
		case "elicitation_dialog", "agent_needs_input":
			return models.AgentHookWaitingForUser
		case "elicitation_complete", "elicitation_response":
			return models.AgentHookUserPromptSubmit
		}
	case "PostToolUse":
		if toolName == "ask_user" || toolName == "AskUserQuestion" {
			return models.AgentHookUserPromptSubmit
		}
	case "Elicitation":
		return models.AgentHookWaitingForUser
	case "ElicitationResult":
		return models.AgentHookUserPromptSubmit
	}
	return eventName
}

type agentHookProcessInfo struct {
	ParentPID int
	Command   string
	Args      string
}

type agentHookProcessLookup func(pid int) (agentHookProcessInfo, bool)

func agentHookProcessArgs(pid int, command string, resolve func(int) string) string {
	base := strings.TrimSuffix(strings.ToLower(filepath.Base(strings.TrimSpace(command))), ".exe")
	if base == "node" || base == "bun" {
		if args := strings.TrimSpace(resolve(pid)); args != "" {
			return args
		}
	}
	return filepath.Base(command)
}

func findAgentAncestorPID(agent models.AgentKind, startPID int, lookup agentHookProcessLookup) int {
	pid := startPID
	for range 32 {
		if pid <= 1 {
			return 0
		}
		process, ok := lookup(pid)
		if !ok {
			return 0
		}
		kind, _, matched := classifyAgentProcess(process.Command, process.Args)
		if matched && kind == agent {
			return pid
		}
		if process.ParentPID <= 1 || process.ParentPID == pid {
			return 0
		}
		pid = process.ParentPID
	}
	return 0
}

// RecordAgentHookEvent reads a hook payload from r and spools it into dir.
// Hook commands run through an intermediate shell, so resolve the matching
// agent ancestor rather than recording the shell PID.
func RecordAgentHookEvent(dir string, agent models.AgentKind, r io.Reader) error {
	data, err := io.ReadAll(io.LimitReader(r, agentHookMaxPayloadBytes))
	if err != nil {
		return fmt.Errorf("read hook payload: %w", err)
	}
	event, err := ParseAgentHookPayload(agent, data)
	if err != nil {
		return err
	}
	event.PID = agentHookProcessPID(agent)
	event.Timestamp = time.Now()
	return WriteAgentHookEvent(dir, event)
}

// WriteAgentHookEvent atomically writes one event into the spool directory.
func WriteAgentHookEvent(dir string, event models.AgentHookEvent) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create spool dir: %w", err)
	}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode hook event: %w", err)
	}
	name := fmt.Sprintf("%020d-%s-%s.json", event.Timestamp.UnixNano(), event.Agent, sanitizeHookFileComponent(event.HookEventName))
	return writeAtomically(filepath.Join(dir, name), data)
}

func sanitizeHookFileComponent(s string) string {
	if s == "" {
		return "event"
	}
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, s)
}

// AgentHookState is the accumulated view of one hook-reported session.
type AgentHookState struct {
	Agent          models.AgentKind
	SessionID      string
	TranscriptPath string
	CWD            string
	Model          string
	Source         string
	PID            int
	LastEventName  string
	LastEventAt    time.Time
	Ended          bool
}

// AgentHookService drains spooled hook events and tracks per-session state.
type AgentHookService struct {
	mu       sync.Mutex
	dir      string
	states   map[string]*AgentHookState
	pidAlive func(pid int) bool
	logf     func(format string, args ...any)
}

// NewAgentHookService creates a hook service reading events from dir.
func NewAgentHookService(dir string, logf func(format string, args ...any)) *AgentHookService {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	return &AgentHookService{
		dir:      dir,
		states:   map[string]*AgentHookState{},
		pidAlive: agentHookPIDAlive,
		logf:     logf,
	}
}

// Dir returns the spool directory this service consumes.
func (s *AgentHookService) Dir() string {
	return s.dir
}

// EnsureDir creates the hook spool before the watcher starts so the first
// event is observable even when the directory did not previously exist.
func (s *AgentHookService) EnsureDir() error {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return fmt.Errorf("create spool dir: %w", err)
	}
	if err := os.Chmod(s.dir, 0o700); err != nil { //nolint:gosec // The spool is a directory and needs owner traversal.
		return fmt.Errorf("secure spool dir: %w", err)
	}
	return nil
}

// Drain consumes all pending spool files and folds them into session states.
func (s *AgentHookService) Drain() {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			s.logf("agent hooks: read spool dir: %v", err)
		}
		return
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, name := range names {
		path := filepath.Join(s.dir, name)
		data, err := os.ReadFile(path) //nolint:gosec // Spool path is app-controlled.
		if err != nil {
			s.logf("agent hooks: read %s: %v", name, err)
			continue
		}
		var event models.AgentHookEvent
		if err := json.Unmarshal(data, &event); err != nil {
			s.logf("agent hooks: decode %s: %v", name, err)
			s.removeSpoolFile(path, name)
			continue
		}
		s.applyLocked(event)
		s.removeSpoolFile(path, name)
	}
	s.pruneLocked(time.Now())
}

func (s *AgentHookService) removeSpoolFile(path, name string) {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		s.logf("agent hooks: remove %s: %v", name, err)
		// Avoid re-processing files that cannot be deleted forever.
		if info, statErr := os.Stat(path); statErr == nil && time.Since(info.ModTime()) > agentHookSpoolFileStaleAfter {
			s.logf("agent hooks: stale undeletable spool file %s", name)
		}
	}
}

func (s *AgentHookService) applyLocked(event models.AgentHookEvent) {
	if event.SessionID == "" || event.Agent == "" {
		return
	}
	key := string(event.Agent) + ":" + event.SessionID
	st, ok := s.states[key]
	if !ok {
		st = &AgentHookState{Agent: event.Agent, SessionID: event.SessionID}
		s.states[key] = st
	}
	if event.TranscriptPath != "" {
		st.TranscriptPath = event.TranscriptPath
	}
	if event.CWD != "" {
		st.CWD = event.CWD
	}
	if event.Model != "" {
		st.Model = event.Model
	}
	if event.Source != "" {
		st.Source = event.Source
	}
	if event.PID > 0 {
		st.PID = event.PID
	}
	if !event.Timestamp.IsZero() && event.Timestamp.After(st.LastEventAt) {
		st.LastEventAt = event.Timestamp
	}
	st.LastEventName = event.HookEventName
	switch event.HookEventName {
	case models.AgentHookSessionEnd:
		st.Ended = true
	case models.AgentHookSessionStart, models.AgentHookUserPromptSubmit, models.AgentHookWaitingForUser,
		models.AgentHookStop, models.AgentHookCwdChanged:
		st.Ended = false
	}
}

func (s *AgentHookService) pruneLocked(now time.Time) {
	for key, st := range s.states {
		if now.Sub(st.LastEventAt) > agentHookStaleAfter {
			delete(s.states, key)
			continue
		}
		if st.Ended && now.Sub(st.LastEventAt) > agentRecentThreshold {
			delete(s.states, key)
		}
	}
}

// States returns a snapshot of the current hook-tracked session states.
func (s *AgentHookService) States() []AgentHookState {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]AgentHookState, 0, len(s.states))
	for _, st := range s.states {
		out = append(out, *st)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Agent != out[j].Agent {
			return out[i].Agent < out[j].Agent
		}
		return out[i].SessionID < out[j].SessionID
	})
	return out
}

// RestoreSessions rehydrates live hook state from the persisted session
// registry after a restart. Pending hook events always take precedence.
func (s *AgentHookService) RestoreSessions(previous map[string]*models.AgentSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for _, session := range previous {
		observation := sessionObservationTime(session)
		if session == nil || session.SchemaVersion != models.AgentHookEventSchemaVersion ||
			session.LivenessSource != models.AgentSessionLivenessSourceHook ||
			session.LivenessState != models.AgentSessionLivenessActive || !session.IsOpen ||
			session.PID <= 0 || strings.TrimSpace(session.ID) == "" || observation.IsZero() ||
			now.Sub(observation) > agentHookStaleAfter || !s.pidAlive(session.PID) {
			continue
		}
		found := false
		for _, state := range s.states {
			if hookStateMatchesSession(state, session) {
				found = true
				break
			}
		}
		if found {
			continue
		}
		sessionID := strings.TrimSpace(session.ID)
		key := string(session.Agent) + ":" + sessionID
		s.states[key] = &AgentHookState{
			Agent:          session.Agent,
			SessionID:      sessionID,
			TranscriptPath: session.JSONLPath,
			CWD:            session.CWD,
			Model:          session.Model,
			PID:            session.PID,
			LastEventAt:    observation,
		}
	}
}

// HasLiveSessions reports whether any hook-tracked session still has a live process.
func (s *AgentHookService) HasLiveSessions() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, st := range s.states {
		if !st.Ended && st.PID > 0 && s.pidAlive(st.PID) {
			return true
		}
	}
	return false
}

// PIDAlive reports whether the given hook-reported PID is still running.
func (s *AgentHookService) PIDAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	s.mu.Lock()
	alive := s.pidAlive
	s.mu.Unlock()
	return alive(pid)
}
