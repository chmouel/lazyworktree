package models

import "time"

// AgentKind identifies the coding agent that owns a transcript.
type AgentKind string

const (
	// AgentKindClaude marks a transcript produced by Claude.
	AgentKindClaude AgentKind = "claude"
	// AgentKindPi marks a transcript produced by pi.
	AgentKindPi AgentKind = "pi"
)

// AgentSessionStatus describes the last observable state of a transcript.
type AgentSessionStatus string

const (
	// AgentSessionStatusUnknown means no recent state could be inferred.
	AgentSessionStatusUnknown AgentSessionStatus = "unknown"
	// AgentSessionStatusWaitingForUser means the agent is waiting on input.
	AgentSessionStatusWaitingForUser AgentSessionStatus = "waiting"
	// AgentSessionStatusThinking means the agent is reasoning about the next step.
	AgentSessionStatusThinking AgentSessionStatus = "thinking"
	// AgentSessionStatusExecutingTool means the agent is currently invoking a tool.
	AgentSessionStatusExecutingTool AgentSessionStatus = "tool"
	// AgentSessionStatusProcessingResult means the agent is digesting a tool result.
	AgentSessionStatusProcessingResult AgentSessionStatus = "processing"
	// AgentSessionStatusIdle means the transcript has gone quiet.
	AgentSessionStatusIdle AgentSessionStatus = "idle"
)

// AgentActivity is the human-friendly label shown in the TUI.
type AgentActivity string

const (
	// AgentActivityIdle shows no recent activity.
	AgentActivityIdle AgentActivity = "idle"
	// AgentActivityWaiting shows the agent waiting for the user.
	AgentActivityWaiting AgentActivity = "waiting"
	// AgentActivityThinking shows the agent reasoning about a response.
	AgentActivityThinking AgentActivity = "thinking"
	// AgentActivityCompacting shows the agent compacting session context.
	AgentActivityCompacting AgentActivity = "compacting"
	// AgentActivityReading shows the agent reading files or data.
	AgentActivityReading AgentActivity = "reading"
	// AgentActivityWriting shows the agent editing files.
	AgentActivityWriting AgentActivity = "writing"
	// AgentActivityRunning shows the agent running a command.
	AgentActivityRunning AgentActivity = "running"
	// AgentActivitySearching shows the agent searching the workspace.
	AgentActivitySearching AgentActivity = "searching"
	// AgentActivityBrowsing shows the agent browsing external or local resources.
	AgentActivityBrowsing AgentActivity = "browsing"
	// AgentActivitySpawning shows the agent launching a sub-agent.
	AgentActivitySpawning AgentActivity = "spawning"
)

// AgentOpenConfidence describes how confidently a live process was matched to a session.
type AgentOpenConfidence string

const (
	// AgentOpenConfidenceNone means no live process match was found.
	AgentOpenConfidenceNone AgentOpenConfidence = "none"
	// AgentOpenConfidenceExact means a live process had the session transcript open.
	AgentOpenConfidenceExact AgentOpenConfidence = "exact"
	// AgentOpenConfidenceCWD means the live process matched by working directory only.
	AgentOpenConfidenceCWD AgentOpenConfidence = "cwd"
)

// AgentSession summarises a Claude or pi transcript attached to a worktree.
type AgentSession struct {
	ID             string
	Agent          AgentKind
	JSONLPath      string
	CWD            string
	Model          string
	GitBranch      string
	DisplayName    string
	LastPromptText string
	LastReplyText  string
	LastTargetPath string
	LastCommand    string
	TaskLabel      string
	CurrentTool    string
	LastToolName   string
	LastToolAt     time.Time
	LastSummaryAt  time.Time
	LastActivity   time.Time
	Status         AgentSessionStatus
	Activity       AgentActivity
	IsOpen         bool
	OpenConfidence AgentOpenConfidence
}
