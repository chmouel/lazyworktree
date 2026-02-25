// Package screen provides a unified screen management system for modal overlays.
package screen

import (
	tea "charm.land/bubbletea/v2"
)

// Screen represents a modal screen overlay that can handle input and render itself.
type Screen interface {
	// Update processes a key message and returns the updated screen and any command.
	// Returning nil for the Screen signals that this screen should be closed.
	Update(msg tea.KeyPressMsg) (Screen, tea.Cmd)

	// View renders the screen's content.
	View() string

	// Type returns the screen's type identifier.
	Type() Type
}

// Type identifies the kind of screen being displayed.
type Type int

// Screen type constants.
const (
	TypeNone Type = iota
	TypeConfirm
	TypeInfo
	TypeInput
	TypeTextarea
	TypeNoteView
	TypeHelp
	TypeTrust
	TypeWelcome
	TypeCommit
	TypePalette
	TypeDiff
	TypePRSelect
	TypeIssueSelect
	TypeListSelect
	TypeLoading
	TypeCommitFiles
	TypeChecklist
	TypeTaskboard
)

// String returns a human-readable name for the screen type.
func (t Type) String() string {
	switch t {
	case TypeNone:
		return "none"
	case TypeConfirm:
		return "confirm"
	case TypeInfo:
		return "info"
	case TypeInput:
		return "input"
	case TypeTextarea:
		return "textarea"
	case TypeNoteView:
		return "note-view"
	case TypeHelp:
		return "help"
	case TypeTrust:
		return "trust"
	case TypeWelcome:
		return "welcome"
	case TypeCommit:
		return "commit"
	case TypePalette:
		return "palette"
	case TypeDiff:
		return "diff"
	case TypePRSelect:
		return "pr-select"
	case TypeIssueSelect:
		return "issue-select"
	case TypeListSelect:
		return "list-select"
	case TypeLoading:
		return "loading"
	case TypeCommitFiles:
		return "commit-files"
	case TypeChecklist:
		return "checklist"
	case TypeTaskboard:
		return "taskboard"
	default:
		return "unknown"
	}
}
