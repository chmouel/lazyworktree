package app

import (
	"context"
	"time"

	"github.com/chmouel/lazyworktree/internal/models"
)

// loadingState groups all loading/operation-tracking flags on the Model.
type loadingState struct {
	active             bool
	operation          string // tracks which operation is loading ("push", "sync", "rerun", etc.)
	prDataLoaded       bool
	checkMergedAfterPR bool // trigger merged check after PR data refresh
}

// detailsState groups fields related to debounced detail pane updates and click detection.
type detailsState struct {
	currentPath   string
	updateCancel  context.CancelFunc
	pendingIndex  int
	lastArrow     int
	lastLog       int
	lastClickTime time.Time
	lastClickPane int
}

// pendingOpState groups post-operation selection and PR attachment state.
type pendingOpState struct {
	selectPath string
	pr         *models.PRInfo
	prPath     string
}
