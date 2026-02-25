package state

// SearchTarget describes where search input is applied.
type SearchTarget int

// Search target options.
const (
	SearchTargetWorktrees SearchTarget = iota
	SearchTargetStatus
	SearchTargetLog
)

// FilterTarget describes which list the filter applies to.
type FilterTarget int

// Filter target options.
const (
	FilterTargetWorktrees FilterTarget = iota
	FilterTargetStatus
	FilterTargetLog
)

// LayoutMode describes the pane arrangement.
type LayoutMode int

// Layout mode options.
const (
	LayoutDefault LayoutMode = iota
	LayoutTop
)

// ViewState holds UI-related state for the model.
type ViewState struct {
	ShowingFilter    bool
	FilterTarget     FilterTarget
	ShowingSearch    bool
	SearchTarget     SearchTarget
	FocusedPane      int
	ZoomedPane       int
	WindowWidth      int
	WindowHeight     int
	Layout           LayoutMode
	TerminalFocused  bool
}
