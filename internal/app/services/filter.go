package services

import (
	"strings"

	"github.com/chmouel/lazyworktree/internal/app/state"
)

// FilterService stores filter and search queries by target.
type FilterService struct {
	FilterQuery         string
	StatusFilterQuery   string
	LogFilterQuery      string
	WorktreeSearchQuery string
	StatusSearchQuery   string
	LogSearchQuery      string
}

// NewFilterService creates a new FilterService with an optional initial filter.
func NewFilterService(initialFilter string) *FilterService {
	return &FilterService{FilterQuery: initialFilter}
}

// FilterQueryForTarget returns the filter query for the given target.
func (f *FilterService) FilterQueryForTarget(target state.FilterTarget) string {
	switch target {
	case state.FilterTargetStatus, state.FilterTargetGitStatus:
		return f.StatusFilterQuery
	case state.FilterTargetLog:
		return f.LogFilterQuery
	default:
		return f.FilterQuery
	}
}

// SetFilterQuery sets the filter query for the given target.
func (f *FilterService) SetFilterQuery(target state.FilterTarget, query string) {
	switch target {
	case state.FilterTargetStatus, state.FilterTargetGitStatus:
		f.StatusFilterQuery = query
	case state.FilterTargetLog:
		f.LogFilterQuery = query
	default:
		f.FilterQuery = query
	}
}

// SearchQueryForTarget returns the search query for the given target.
func (f *FilterService) SearchQueryForTarget(target state.SearchTarget) string {
	switch target {
	case state.SearchTargetStatus, state.SearchTargetGitStatus:
		return f.StatusSearchQuery
	case state.SearchTargetLog:
		return f.LogSearchQuery
	default:
		return f.WorktreeSearchQuery
	}
}

// SetSearchQuery sets the search query for the given target.
func (f *FilterService) SetSearchQuery(target state.SearchTarget, query string) {
	switch target {
	case state.SearchTargetStatus, state.SearchTargetGitStatus:
		f.StatusSearchQuery = query
	case state.SearchTargetLog:
		f.LogSearchQuery = query
	default:
		f.WorktreeSearchQuery = query
	}
}

// HasActiveFilterForPane reports whether a pane has a non-empty filter applied.
func (f *FilterService) HasActiveFilterForPane(paneIndex int) bool {
	switch paneIndex {
	case 0:
		return strings.TrimSpace(f.FilterQuery) != ""
	case 1:
		// Info-only pane, no filter
		return false
	case 2:
		return strings.TrimSpace(f.StatusFilterQuery) != ""
	case 3:
		return strings.TrimSpace(f.LogFilterQuery) != ""
	}
	return false
}
