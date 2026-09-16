package app

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/chmouel/lazyworktree/internal/app/services"
	"github.com/chmouel/lazyworktree/internal/models"
)

// maxDisplayedReviewers caps how many reviewers are named on the Info pane
// line; the rest are summarised as a trailing count.
const maxDisplayedReviewers = 5

// prReviewersEnabled reports whether reviewers may be fetched and displayed.
func (m *Model) prReviewersEnabled() bool {
	if m == nil || m.config == nil || m.config.DisablePR {
		return false
	}
	return !strings.EqualFold(strings.TrimSpace(m.config.PRReviewers), "never")
}

// prReviewerKey identifies a reviewer lookup. The change request number is part
// of the key because a branch may later carry a different change request, and
// the stale result must not be shown against the new one.
func prReviewerKey(wt *models.WorktreeInfo) string {
	if wt == nil || wt.PR == nil || wt.PR.Number <= 0 {
		return ""
	}
	return fmt.Sprintf("%s#%d", wt.Branch, wt.PR.Number)
}

// prSectionVisible reports whether the Info pane shows PR/MR details for a
// worktree, which is also when fetching reviewers is worthwhile.
func (m *Model) prSectionVisible(wt *models.WorktreeInfo) bool {
	if wt == nil || wt.PR == nil || m.config == nil || m.config.DisablePR {
		return false
	}
	if !wt.IsMain {
		return true
	}
	return wt.PR.State != prStateMerged && wt.PR.State != prStateClosed
}

// reviewersForWorktree returns the cached reviewers for a worktree, if any.
func (m *Model) reviewersForWorktree(wt *models.WorktreeInfo) *models.PRReviewerSummary {
	if !m.prReviewersEnabled() || m.cache.reviewerCache == nil {
		return nil
	}
	key := prReviewerKey(wt)
	if key == "" {
		return nil
	}
	summary, ok := m.cache.reviewerCache.Get(key)
	if !ok {
		return nil
	}
	return summary
}

// clearReviewerCache drops every cached reviewer lookup and invalidates any
// request still in flight.
func (m *Model) clearReviewerCache() {
	if m.cache.reviewerCache != nil {
		m.cache.reviewerCache.Clear()
	}
}

// maybeFetchPRReviewers fetches reviewers for the selected worktree when the
// cached result is missing or stale. Reviews change slowly, so this runs on
// selection and on an explicit refresh rather than on a timer.
func (m *Model) maybeFetchPRReviewers() tea.Cmd {
	if !m.prReviewersEnabled() || m.cache.reviewerCache == nil {
		return nil
	}
	wt := m.selectedWorktree()
	if !m.prSectionVisible(wt) {
		return nil
	}
	if !m.state.services.git.IsGitHubOrGitLab(m.ctx) {
		return nil
	}
	key := prReviewerKey(wt)
	if key == "" {
		return nil
	}
	if !m.cache.reviewerCache.ShouldFetch(key, services.DefaultReviewerCacheTTL, services.DefaultReviewerRetryBackoff) {
		return nil
	}
	token, ok := m.cache.reviewerCache.MarkFetching(key)
	if !ok {
		return nil
	}
	return m.fetchPRReviewersCmd(token, wt.PR.Number)
}

func (m *Model) fetchPRReviewersCmd(token services.ReviewerToken, prNumber int) tea.Cmd {
	return func() tea.Msg {
		summary, err := m.state.services.git.FetchPRReviewers(m.ctx, prNumber)
		return prReviewersLoadedMsg{token: token, summary: summary, err: err}
	}
}

// handlePRReviewersLoaded records a reviewer lookup and refreshes the Info pane
// when the result belongs to the worktree still on screen.
func (m *Model) handlePRReviewersLoaded(msg prReviewersLoadedMsg) (tea.Model, tea.Cmd) {
	if m.cache.reviewerCache == nil {
		return m, nil
	}
	// Completing the token always releases its claim, even when the selection
	// has moved on, so the key never stays wedged as "still fetching".
	m.cache.reviewerCache.Complete(msg.token, msg.summary, msg.err)
	if msg.err != nil {
		m.debugf("reviewer fetch failed for %s: %v", msg.token.Key(), msg.err)
		return m, nil
	}

	wt := m.selectedWorktree()
	if prReviewerKey(wt) != msg.token.Key() {
		return m, nil
	}
	m.infoContent = m.buildInfoContent(wt, m.infoContentWidth)
	return m, m.queuePRAvatarFetches()
}
