package app

import "github.com/chmouel/lazyworktree/internal/app/state"

type (
	searchTarget = state.SearchTarget
	filterTarget = state.FilterTarget
)

const (
	searchTargetWorktrees = state.SearchTargetWorktrees
	searchTargetStatus    = state.SearchTargetStatus
	searchTargetLog       = state.SearchTargetLog
)

const (
	filterTargetWorktrees = state.FilterTargetWorktrees
	filterTargetStatus    = state.FilterTargetStatus
	filterTargetLog       = state.FilterTargetLog
)
