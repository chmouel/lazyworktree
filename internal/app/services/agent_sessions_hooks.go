package services

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/chmouel/lazyworktree/internal/models"
)

// SetHookService attaches a hook spool consumer to the session service.
func (s *AgentSessionService) SetHookService(hooks *AgentHookService) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hooks = hooks
}

func (s *AgentSessionService) hookService() *AgentHookService {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hooks
}

// hookStateMatchesSession reports whether a hook state refers to the given session.
func hookStateMatchesSession(state *AgentHookState, session *models.AgentSession) bool {
	if session == nil || state == nil || session.Agent != state.Agent {
		return false
	}
	if state.TranscriptPath != "" && session.JSONLPath != "" &&
		filepath.Clean(state.TranscriptPath) == filepath.Clean(session.JSONLPath) {
		return true
	}
	return state.SessionID != "" && strings.TrimSpace(session.ID) == state.SessionID
}

// applyHookSessions synthesises sessions for hook-tracked agents that have no
// transcript-backed session yet (e.g. Codex, whose transcript format is unstable).
func (s *AgentSessionService) applyHookSessions(sessions []*models.AgentSession, states []AgentHookState, now time.Time) []*models.AgentSession {
	for i := range states {
		state := &states[i]
		if state.Ended {
			continue
		}
		found := false
		for _, session := range sessions {
			if hookStateMatchesSession(state, session) {
				found = true
				break
			}
		}
		if found {
			continue
		}
		session := &models.AgentSession{
			Agent:          state.Agent,
			ID:             state.SessionID,
			CWD:            state.CWD,
			JSONLPath:      state.TranscriptPath,
			Model:          state.Model,
			PID:            state.PID,
			Status:         hookEventStatus(state.LastEventName),
			LastActivity:   state.LastEventAt,
			LastObservedAt: state.LastEventAt,
			SchemaVersion:  models.AgentHookEventSchemaVersion,
		}
		if session.LastActivity.IsZero() {
			session.LastActivity = now
			session.LastObservedAt = now
		}
		session.SessionKey = agentSessionKey(session)
		session.Title = deriveAgentSessionTitle(session)
		sessions = append(sessions, session)
	}
	return sessions
}

// applyHookLiveness overrides liveness for hook-tracked sessions using PID
// probes, which are far cheaper and more precise than ps/lsof scans.
func (s *AgentSessionService) applyHookLiveness(sessions []*models.AgentSession, hooks *AgentHookService, states []AgentHookState, now time.Time) {
	for i := range states {
		state := &states[i]
		for _, session := range sessions {
			if !hookStateMatchesSession(state, session) {
				continue
			}
			session.PID = state.PID
			if state.LastEventAt.After(session.LastActivity) {
				session.LastActivity = state.LastEventAt
			}
			if state.LastEventAt.After(session.LastObservedAt) {
				session.LastObservedAt = state.LastEventAt
			}
			if state.CWD != "" {
				session.CWD = state.CWD
				session.SessionKey = agentSessionKey(session)
			}
			if state.Model != "" && session.Model == "" {
				session.Model = state.Model
			}
			if status := hookEventStatus(state.LastEventName); status != models.AgentSessionStatusUnknown {
				session.Status = status
			}
			alive := !state.Ended && hooks.PIDAlive(state.PID)
			switch {
			case alive:
				session.IsOpen = true
				session.OpenConfidence = models.AgentOpenConfidenceExact
				session.LivenessState = models.AgentSessionLivenessActive
				session.LivenessSource = models.AgentSessionLivenessSourceHook
				session.LastObservedAt = now
			case state.Ended && session.LivenessSource != models.AgentSessionLivenessSourceExactFile:
				// A SessionEnd event is authoritative unless ps proved otherwise.
				session.IsOpen = false
				session.OpenConfidence = models.AgentOpenConfidenceNone
				session.LivenessState = models.AgentSessionLivenessInactive
				session.LivenessSource = models.AgentSessionLivenessSourceHook
			}
			if activity, ok := hookEventActivity(state.LastEventName); ok {
				session.Activity = activity
			} else {
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
			break
		}
	}
}

func hookEventStatus(eventName string) models.AgentSessionStatus {
	switch eventName {
	case models.AgentHookUserPromptSubmit:
		return models.AgentSessionStatusThinking
	case models.AgentHookStop, models.AgentHookWaitingForUser:
		return models.AgentSessionStatusWaitingForUser
	case models.AgentHookSessionStart, models.AgentHookCwdChanged:
		return models.AgentSessionStatusIdle
	default:
		return models.AgentSessionStatusUnknown
	}
}

func hookEventActivity(eventName string) (models.AgentActivity, bool) {
	switch eventName {
	case models.AgentHookWaitingForUser, models.AgentHookStop:
		return models.AgentActivityWaiting, true
	case models.AgentHookUserPromptSubmit:
		return models.AgentActivityThinking, true
	default:
		return "", false
	}
}
