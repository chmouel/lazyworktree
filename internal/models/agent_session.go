package models

import "time"

// AgentKind identifies the coding agent that owns a transcript.
type AgentKind string

const (
	// AgentKindClaude marks a transcript produced by Claude.
	AgentKindClaude AgentKind = "claude"
	// AgentKindPi marks a transcript produced by pi.
	AgentKindPi AgentKind = "pi"
	// AgentKindCodex marks a session produced by the OpenAI Codex CLI.
	AgentKindCodex AgentKind = "codex"
	// AgentKindCopilot marks a session produced by the GitHub Copilot CLI.
	AgentKindCopilot AgentKind = "copilot"
)

// AgentSessionStatus describes the last observable state of a transcript.
type AgentSessionStatus string

const (
	// AgentSessionStatusUnknown means no recent state could be inferred.
	AgentSessionStatusUnknown AgentSessionStatus = "unknown"
	// AgentSessionStatusWaitingForUser means the agent is waiting on input.
	AgentSessionStatusWaitingForUser AgentSessionStatus = "waiting"
	// AgentSessionStatusWaitingApproval means a delegated tool call is waiting for approval or a result.
	AgentSessionStatusWaitingApproval AgentSessionStatus = "approval"
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
	// AgentActivityApproval shows the agent waiting for a tool approval/result.
	AgentActivityApproval AgentActivity = "approval"
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

// AgentSessionLiveness describes how strongly the session appears to still be alive.
type AgentSessionLiveness string

const (
	// AgentSessionLivenessActive means there is current strong proof the session is live.
	AgentSessionLivenessActive AgentSessionLiveness = "active"
	// AgentSessionLivenessRecent means the session was observed recently but is not confirmed active.
	AgentSessionLivenessRecent AgentSessionLiveness = "recent"
	// AgentSessionLivenessSuspect means only heuristic evidence suggests the session may still be live.
	AgentSessionLivenessSuspect AgentSessionLiveness = "suspect"
	// AgentSessionLivenessInactive means there is no current evidence the session is live.
	AgentSessionLivenessInactive AgentSessionLiveness = "inactive"
)

// AgentSessionLivenessSource identifies where the current liveness evidence came from.
type AgentSessionLivenessSource string

const (
	// AgentSessionLivenessSourceNative means the agent itself exposed explicit liveness metadata.
	AgentSessionLivenessSourceNative AgentSessionLivenessSource = "native"
	// AgentSessionLivenessSourceRegistry means the persisted registry supplied the best recent evidence.
	AgentSessionLivenessSourceRegistry AgentSessionLivenessSource = "registry"
	// AgentSessionLivenessSourceExactFile means a live process has the transcript file open.
	AgentSessionLivenessSourceExactFile AgentSessionLivenessSource = "exact_file"
	// AgentSessionLivenessSourceCWDHeuristic means the match came only from a working-directory heuristic.
	AgentSessionLivenessSourceCWDHeuristic AgentSessionLivenessSource = "cwd_heuristic"
	// AgentSessionLivenessSourceHook means an agent lifecycle hook reported the session.
	AgentSessionLivenessSourceHook AgentSessionLivenessSource = "hook"
	// AgentSessionLivenessSourceNone means no evidence was available.
	AgentSessionLivenessSourceNone AgentSessionLivenessSource = "none"
)

// AgentSession summarises a coding-agent session attached to a worktree.
type AgentSession struct {
	ID             string
	SessionKey     string
	Agent          AgentKind
	JSONLPath      string
	CWD            string
	Model          string
	GitBranch      string
	DisplayName    string
	Title          string
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
	LastObservedAt time.Time
	Status         AgentSessionStatus
	Activity       AgentActivity
	LivenessState  AgentSessionLiveness
	LivenessSource AgentSessionLivenessSource
	ResumeHint     string
	SchemaVersion  string
	// PID is the agent process id reported by a lifecycle hook, when known.
	PID            int
	IsOpen         bool
	OpenConfidence AgentOpenConfidence
}
