package models

import "time"

// AgentHookEventSchemaVersion identifies the spool event format.
const AgentHookEventSchemaVersion = "lazyworktree-agent-hook-event-v1"

// AgentHookEvent is a normalised agent lifecycle event written to the spool
// directory by the hidden `agent-event` hook shim and consumed by the TUI.
type AgentHookEvent struct {
	SchemaVersion  string    `json:"schema_version"`
	Agent          AgentKind `json:"agent"`
	HookEventName  string    `json:"hook_event_name"`
	SessionID      string    `json:"session_id"`
	TranscriptPath string    `json:"transcript_path,omitempty"`
	CWD            string    `json:"cwd,omitempty"`
	Model          string    `json:"model,omitempty"`
	Source         string    `json:"source,omitempty"`
	PID            int       `json:"pid,omitempty"`
	Timestamp      time.Time `json:"timestamp"`
}

// Hook event names shared by the Claude Code, Codex CLI, and Copilot CLI
// hook systems. Copilot CLI emits these when its hooks are configured with
// the PascalCase (VS Code compatible) event names.
const (
	AgentHookSessionStart     = "SessionStart"
	AgentHookSessionEnd       = "SessionEnd"
	AgentHookStop             = "Stop"
	AgentHookUserPromptSubmit = "UserPromptSubmit"
	AgentHookWaitingForUser   = "WaitingForUser"
	AgentHookCwdChanged       = "CwdChanged"
)
