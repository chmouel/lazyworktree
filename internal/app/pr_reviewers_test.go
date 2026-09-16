package app

import (
	"errors"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chmouel/lazyworktree/internal/app/services"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/theme"
)

func reviewerWorktree(t *testing.T, branch string, number int) *models.WorktreeInfo {
	t.Helper()
	return &models.WorktreeInfo{
		Path:   t.TempDir(),
		Branch: branch,
		PR: &models.PRInfo{
			Number: number,
			State:  prStateOpen,
			Title:  "Add feature",
			URL:    "https://github.com/acme/repo/pull/1",
			Author: "alice",
		},
	}
}

// newReviewerModel builds a model with one selected worktree carrying a PR.
func newReviewerModel(t *testing.T, cfg *config.AppConfig, wt *models.WorktreeInfo) *Model {
	t.Helper()
	if cfg.WorktreeDir == "" {
		cfg.WorktreeDir = t.TempDir()
	}
	m := NewModel(cfg, "")
	m.state.data.worktrees = []*models.WorktreeInfo{wt}
	m.state.data.filteredWts = []*models.WorktreeInfo{wt}
	m.state.data.selectedIndex = 0
	return m
}

func cacheReviewers(m *Model, key string, summary *models.PRReviewerSummary) {
	token, _ := m.cache.reviewerCache.MarkFetching(key)
	m.cache.reviewerCache.Complete(token, summary, nil)
}

func TestPRReviewerKey(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "feature#42", prReviewerKey(&models.WorktreeInfo{
		Branch: "feature",
		PR:     &models.PRInfo{Number: 42},
	}))
	assert.Empty(t, prReviewerKey(nil))
	assert.Empty(t, prReviewerKey(&models.WorktreeInfo{Branch: "feature"}))
	assert.Empty(t, prReviewerKey(&models.WorktreeInfo{
		Branch: "feature",
		PR:     &models.PRInfo{Number: 0},
	}))
}

func TestPRReviewersEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  *config.AppConfig
		want bool
	}{
		{"auto is on", &config.AppConfig{PRReviewers: "auto"}, true},
		{"unset is on", &config.AppConfig{}, true},
		{"never is off", &config.AppConfig{PRReviewers: "never"}, false},
		{"case is ignored", &config.AppConfig{PRReviewers: "Never"}, false},
		{"disabling PRs turns it off", &config.AppConfig{PRReviewers: "auto", DisablePR: true}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := &Model{config: tc.cfg}
			assert.Equal(t, tc.want, m.prReviewersEnabled())
		})
	}
}

func TestMaybeFetchPRReviewersGating(t *testing.T) {
	t.Run("nothing is fetched when the option is off", func(t *testing.T) {
		wt := reviewerWorktree(t, "feature", 1)
		m := newReviewerModel(t, &config.AppConfig{PRReviewers: "never"}, wt)
		assert.Nil(t, m.maybeFetchPRReviewers())
	})

	t.Run("nothing is fetched when PRs are disabled", func(t *testing.T) {
		wt := reviewerWorktree(t, "feature", 1)
		m := newReviewerModel(t, &config.AppConfig{DisablePR: true}, wt)
		assert.Nil(t, m.maybeFetchPRReviewers())
	})

	t.Run("nothing is fetched without a change request", func(t *testing.T) {
		wt := &models.WorktreeInfo{Path: t.TempDir(), Branch: "feature"}
		m := newReviewerModel(t, &config.AppConfig{}, wt)
		assert.Nil(t, m.maybeFetchPRReviewers())
	})

	t.Run("nothing is fetched when the PR section is hidden", func(t *testing.T) {
		wt := reviewerWorktree(t, "main", 1)
		wt.IsMain = true
		wt.PR.State = prStateMerged
		m := newReviewerModel(t, &config.AppConfig{}, wt)
		assert.Nil(t, m.maybeFetchPRReviewers())
	})

	t.Run("nothing is fetched while a lookup is in flight", func(t *testing.T) {
		wt := reviewerWorktree(t, "feature", 1)
		m := newReviewerModel(t, &config.AppConfig{}, wt)
		_, ok := m.cache.reviewerCache.MarkFetching("feature#1")
		require.True(t, ok)
		assert.Nil(t, m.maybeFetchPRReviewers())
	})

	t.Run("nothing is fetched when the cached result is fresh", func(t *testing.T) {
		wt := reviewerWorktree(t, "feature", 1)
		m := newReviewerModel(t, &config.AppConfig{}, wt)
		cacheReviewers(m, "feature#1", &models.PRReviewerSummary{Total: 1})
		assert.Nil(t, m.maybeFetchPRReviewers())
	})

	t.Run("nothing is fetched during the backoff after a failure", func(t *testing.T) {
		wt := reviewerWorktree(t, "feature", 1)
		m := newReviewerModel(t, &config.AppConfig{}, wt)
		token, _ := m.cache.reviewerCache.MarkFetching("feature#1")
		m.cache.reviewerCache.Complete(token, nil, errors.New("boom"))
		assert.Nil(t, m.maybeFetchPRReviewers())
	})
}

func TestHandlePRReviewersLoaded(t *testing.T) {
	t.Run("a result for the selected worktree is cached and shown", func(t *testing.T) {
		wt := reviewerWorktree(t, "feature", 1)
		m := newReviewerModel(t, &config.AppConfig{}, wt)
		token, _ := m.cache.reviewerCache.MarkFetching("feature#1")

		updated, _ := m.handlePRReviewersLoaded(prReviewersLoadedMsg{
			token:   token,
			summary: &models.PRReviewerSummary{Total: 1, Reviewers: []*models.PRReviewer{{Login: "bob", State: models.ReviewStateApproved}}},
		})
		model := updated.(*Model)

		assert.Contains(t, stripTerminalSequences(model.infoContent), "Reviewers: 1")
		assert.Contains(t, stripTerminalSequences(model.infoContent), "@bob")
	})

	t.Run("a result arriving after the selection moved is still cached", func(t *testing.T) {
		wt := reviewerWorktree(t, "feature", 1)
		m := newReviewerModel(t, &config.AppConfig{}, wt)
		token, _ := m.cache.reviewerCache.MarkFetching("other#9")
		m.infoContent = "unchanged"

		updated, _ := m.handlePRReviewersLoaded(prReviewersLoadedMsg{
			token:   token,
			summary: &models.PRReviewerSummary{Total: 2},
		})
		model := updated.(*Model)

		assert.Equal(t, "unchanged", model.infoContent, "the pane was rebuilt for a worktree that is not on screen")
		summary, found := model.cache.reviewerCache.Get("other#9")
		require.True(t, found)
		assert.Equal(t, 2, summary.Total)
		// The claim was released, so that key can be looked up again.
		assert.True(t, model.cache.reviewerCache.ShouldFetch("other#9", 0, 0))
	})

	t.Run("a failure releases the claim and is retried after the backoff", func(t *testing.T) {
		wt := reviewerWorktree(t, "feature", 1)
		m := newReviewerModel(t, &config.AppConfig{}, wt)
		token, _ := m.cache.reviewerCache.MarkFetching("feature#1")

		updated, cmd := m.handlePRReviewersLoaded(prReviewersLoadedMsg{token: token, err: errors.New("boom")})
		model := updated.(*Model)

		assert.Nil(t, cmd)
		_, found := model.cache.reviewerCache.Get("feature#1")
		assert.False(t, found)
		assert.False(t, model.cache.reviewerCache.ShouldFetch("feature#1", time.Minute, time.Minute))
		assert.True(t, model.cache.reviewerCache.ShouldFetch("feature#1", time.Minute, time.Nanosecond))
	})
}

// TestInvalidateReviewerCache covers a refresh: the lookup is due again, but
// the reviewers already on screen stay there until a better answer arrives.
func TestInvalidateReviewerCache(t *testing.T) {
	wt := reviewerWorktree(t, "feature", 1)
	m := newReviewerModel(t, &config.AppConfig{}, wt)
	cacheReviewers(m, "feature#1", &models.PRReviewerSummary{Total: 1})

	m.invalidateReviewerCache()

	summary, found := m.cache.reviewerCache.Get("feature#1")
	require.True(t, found, "the previous answer is kept")
	assert.Equal(t, 1, summary.Total)
	assert.True(t, m.cache.reviewerCache.ShouldFetch("feature#1", time.Minute, time.Minute),
		"the lookup is due again straight away")
}

func TestRenderReviewersLine(t *testing.T) {
	t.Run("the line is omitted when there is nothing to report", func(t *testing.T) {
		wt := reviewerWorktree(t, "feature", 1)
		m := newReviewerModel(t, &config.AppConfig{}, wt)

		assert.Empty(t, m.renderReviewersLine(wt, 60))

		cacheReviewers(m, "feature#1", &models.PRReviewerSummary{})
		assert.Empty(t, m.renderReviewersLine(wt, 60))
	})

	t.Run("the line is omitted when the option is off", func(t *testing.T) {
		wt := reviewerWorktree(t, "feature", 1)
		m := newReviewerModel(t, &config.AppConfig{PRReviewers: "never"}, wt)
		cacheReviewers(m, "feature#1", &models.PRReviewerSummary{
			Total:     1,
			Reviewers: []*models.PRReviewer{{Login: "bob", State: models.ReviewStateApproved}},
		})

		assert.Empty(t, m.renderReviewersLine(wt, 60))
		assert.NotContains(t, stripTerminalSequences(m.buildInfoContent(wt, 60)), "Reviewers:")
	})

	t.Run("states and the bot marker are rendered", func(t *testing.T) {
		wt := reviewerWorktree(t, "feature", 1)
		m := newReviewerModel(t, &config.AppConfig{IconSet: "text"}, wt)
		cacheReviewers(m, "feature#1", &models.PRReviewerSummary{
			Total: 3,
			Reviewers: []*models.PRReviewer{
				{Login: "alice", State: models.ReviewStateApproved},
				{Login: "bob", State: models.ReviewStateChangesRequested},
				{Login: "copilot", State: models.ReviewStateCommented, IsBot: true},
			},
		})

		line := stripTerminalSequences(m.renderReviewersLine(wt, 120))

		assert.Contains(t, line, "Reviewers: 3")
		assert.Contains(t, line, "@alice +")
		assert.Contains(t, line, "@bob !")
		assert.Contains(t, line, "B @copilot ~")
	})

	t.Run("the reported count is shown even when identities are missing", func(t *testing.T) {
		wt := reviewerWorktree(t, "feature", 1)
		m := newReviewerModel(t, &config.AppConfig{IconSet: "text"}, wt)
		cacheReviewers(m, "feature#1", &models.PRReviewerSummary{Total: 4, Reviewers: nil})

		assert.Contains(t, stripTerminalSequences(m.renderReviewersLine(wt, 120)), "Reviewers: 4")
	})

	t.Run("more than five reviewers are summarised", func(t *testing.T) {
		wt := reviewerWorktree(t, "feature", 1)
		m := newReviewerModel(t, &config.AppConfig{IconSet: "text"}, wt)
		reviewers := make([]*models.PRReviewer, 0, 8)
		for _, login := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
			reviewers = append(reviewers, &models.PRReviewer{Login: login, State: models.ReviewStateApproved})
		}
		cacheReviewers(m, "feature#1", &models.PRReviewerSummary{Total: 8, Reviewers: reviewers})

		line := stripTerminalSequences(m.renderReviewersLine(wt, 200))

		assert.Contains(t, line, "Reviewers: 8")
		assert.Contains(t, line, "+3")
		assert.NotContains(t, line, "@f")
	})

	t.Run("a long login is truncated", func(t *testing.T) {
		wt := reviewerWorktree(t, "feature", 1)
		m := newReviewerModel(t, &config.AppConfig{IconSet: "text"}, wt)
		cacheReviewers(m, "feature#1", &models.PRReviewerSummary{
			Total:     1,
			Reviewers: []*models.PRReviewer{{Login: strings.Repeat("x", 40), State: models.ReviewStateApproved}},
		})

		line := stripTerminalSequences(m.renderReviewersLine(wt, 200))

		assert.Contains(t, line, "…")
		assert.NotContains(t, line, strings.Repeat("x", 20))
	})

	t.Run("the line never outgrows the pane", func(t *testing.T) {
		reviewers := make([]*models.PRReviewer, 0, 5)
		for _, login := range []string{"alexandra", "bartholomew", "constantine", "demetrius", "evangeline"} {
			reviewers = append(reviewers, &models.PRReviewer{Login: login, State: models.ReviewStateApproved})
		}

		for _, width := range []int{20, 30, 48, 80} {
			wt := reviewerWorktree(t, "feature", 1)
			m := newReviewerModel(t, &config.AppConfig{IconSet: "text"}, wt)
			cacheReviewers(m, "feature#1", &models.PRReviewerSummary{Total: 5, Reviewers: reviewers})

			line := m.renderReviewersLine(wt, width)
			assert.LessOrEqual(t, lipgloss.Width(line), width, "width %d overflowed", width)
		}
	})

	t.Run("a narrow pane still reports the count", func(t *testing.T) {
		wt := reviewerWorktree(t, "feature", 1)
		m := newReviewerModel(t, &config.AppConfig{IconSet: "text"}, wt)
		cacheReviewers(m, "feature#1", &models.PRReviewerSummary{
			Total:     2,
			Reviewers: []*models.PRReviewer{{Login: "alexandrina", State: models.ReviewStateApproved}, {Login: "bartholomew", State: models.ReviewStateApproved}},
		})

		line := stripTerminalSequences(m.renderReviewersLine(wt, 16))

		assert.Contains(t, line, "Reviewers: 2")
	})
}

func TestRenderReviewerAvatarBadge(t *testing.T) {
	t.Run("a loaded reviewer avatar is drawn", func(t *testing.T) {
		wt := reviewerWorktree(t, "feature", 1)
		m := newReviewerModel(t, &config.AppConfig{AvatarBadges: "always", IconSet: "text"}, wt)
		avatarURL := "https://example.com/bob.png"
		m.avatarStates[avatarURL] = &avatarRuntimeState{
			status:     avatarStateLoaded,
			registered: true,
			image:      &services.AvatarImage{URL: avatarURL, Key: "bob", PNG: []byte("png")},
		}
		cacheReviewers(m, "feature#1", &models.PRReviewerSummary{
			Total:     1,
			Reviewers: []*models.PRReviewer{{Login: "bob", AvatarURL: avatarURL, State: models.ReviewStateApproved}},
		})

		line := m.renderReviewersLine(wt, 120)

		assert.Contains(t, line, kittyPlaceholderRune)
		assert.Contains(t, stripTerminalSequences(line), "@bob")
	})

	t.Run("an avatar that has not arrived leaves the name alone", func(t *testing.T) {
		wt := reviewerWorktree(t, "feature", 1)
		m := newReviewerModel(t, &config.AppConfig{AvatarBadges: "always", IconSet: "text"}, wt)
		cacheReviewers(m, "feature#1", &models.PRReviewerSummary{
			Total:     1,
			Reviewers: []*models.PRReviewer{{Login: "bob", AvatarURL: "https://example.com/bob.png", State: models.ReviewStateApproved}},
		})

		line := m.renderReviewersLine(wt, 120)

		assert.NotContains(t, line, kittyPlaceholderRune)
		assert.Contains(t, stripTerminalSequences(line), "@bob")
	})

	t.Run("reviewer avatars are queued for download", func(t *testing.T) {
		wt := reviewerWorktree(t, "feature", 1)
		m := newReviewerModel(t, &config.AppConfig{AvatarBadges: "always"}, wt)
		cacheReviewers(m, "feature#1", &models.PRReviewerSummary{
			Total: 2,
			Reviewers: []*models.PRReviewer{
				{Login: "bob", AvatarURL: "https://example.com/bob.png"},
				{Login: "copilot", AvatarURL: "https://example.com/copilot.png", IsBot: true},
			},
		})

		require.NotNil(t, m.queuePRAvatarFetches())

		assert.Contains(t, m.avatarStates, "https://example.com/bob.png")
		assert.Contains(t, m.avatarStates, "https://example.com/copilot.png")
	})

	t.Run("a late reviewer avatar refreshes the pane", func(t *testing.T) {
		wt := reviewerWorktree(t, "feature", 1)
		m := newReviewerModel(t, &config.AppConfig{AvatarBadges: "always", IconSet: "text"}, wt)
		avatarURL := "https://example.com/bob.png"
		m.avatarStates[avatarURL] = &avatarRuntimeState{
			status: avatarStateLoaded,
			image:  &services.AvatarImage{URL: avatarURL, Key: "bob", PNG: []byte("png")},
		}
		cacheReviewers(m, "feature#1", &models.PRReviewerSummary{
			Total:     1,
			Reviewers: []*models.PRReviewer{{Login: "bob", AvatarURL: avatarURL, State: models.ReviewStateApproved}},
		})
		m.infoContentWidth = 120
		m.infoContent = "stale"

		updated, _ := m.handleAvatarRegistered(avatarRegisteredMsg{url: avatarURL})

		assert.Contains(t, updated.(*Model).infoContent, kittyPlaceholderRune)
	})
}

func TestTruncateDisplay(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "alice", truncateDisplay("alice", 16))
	assert.Equal(t, "alice", truncateDisplay("  alice  ", 16))
	assert.Equal(t, "ali…", truncateDisplay("alice", 4))
	assert.Equal(t, "alice", truncateDisplay("alice", 0), "a non-positive width means no limit")
	assert.LessOrEqual(t, lipgloss.Width(truncateDisplay(strings.Repeat("x", 50), 16)), 16)
}

func TestInfoContentRebuildsOnWidthChange(t *testing.T) {
	longReviewers := []*models.PRReviewer{
		{Login: "alexandrina", State: models.ReviewStateApproved},
		{Login: "bartholomew", State: models.ReviewStateApproved},
		{Login: "constantine", State: models.ReviewStateApproved},
		{Login: "demetrius", State: models.ReviewStateApproved},
		{Login: "evangeline", State: models.ReviewStateApproved},
	}

	newWideModel := func(t *testing.T) (*Model, *models.WorktreeInfo) {
		t.Helper()
		wt := reviewerWorktree(t, "feature", 1)
		m := newReviewerModel(t, &config.AppConfig{IconSet: "text"}, wt)
		cacheReviewers(m, "feature#1", &models.PRReviewerSummary{Total: 5, Reviewers: longReviewers})
		m.setWindowSize(200, 50)
		require.Contains(t, stripTerminalSequences(m.infoContent), "@evangeline")
		return m, wt
	}

	t.Run("resizing the terminal", func(t *testing.T) {
		m, _ := newWideModel(t)
		m.setWindowSize(70, 50)
		assert.NotContains(t, stripTerminalSequences(m.infoContent), "@evangeline")
	})

	t.Run("resizing the pane with h", func(t *testing.T) {
		m, _ := newWideModel(t)
		for range 6 {
			m.state.view.ResizeOffset += resizeStep
			m.applyLayout(m.computeLayout())
		}
		assert.NotContains(t, stripTerminalSequences(m.infoContent), "@evangeline")
	})

	t.Run("zooming a pane", func(t *testing.T) {
		m, _ := newWideModel(t)
		before := m.infoContentWidth
		m.state.view.ZoomedPane = paneInfo
		m.applyLayout(m.computeLayout())
		if m.infoContentWidth != before {
			assert.Equal(t, m.buildInfoContent(m.selectedWorktree(), m.infoContentWidth), m.infoContent)
		}
	})
}

func TestBuildInfoContentDoesNotMutateTheModel(t *testing.T) {
	wt := reviewerWorktree(t, "feature", 1)
	m := newReviewerModel(t, &config.AppConfig{IconSet: "text"}, wt)
	cacheReviewers(m, "feature#1", &models.PRReviewerSummary{
		Total:     1,
		Reviewers: []*models.PRReviewer{{Login: "bob", State: models.ReviewStateApproved}},
	})
	m.infoContent = "sentinel"
	m.infoContentWidth = 42

	// buildInfoContent runs inside a background command, so it must leave the
	// model untouched.
	out := m.buildInfoContent(wt, 80)

	assert.NotEmpty(t, out)
	assert.Equal(t, "sentinel", m.infoContent)
	assert.Equal(t, 42, m.infoContentWidth)
}

func TestRenderReviewStateGlyph(t *testing.T) {
	t.Parallel()

	thm := theme.GetTheme("dracula")
	nerd := &Model{config: &config.AppConfig{IconSet: "nerd-font-v3"}, theme: thm}
	text := &Model{config: &config.AppConfig{IconSet: "text"}, theme: thm}
	off := &Model{config: &config.AppConfig{}, theme: thm}

	for _, state := range []string{
		models.ReviewStateApproved,
		models.ReviewStateChangesRequested,
		models.ReviewStateCommented,
		models.ReviewStateDismissed,
	} {
		assert.NotEmpty(t, stripTerminalSequences(nerd.renderReviewStateGlyph(state)), state)
		assert.NotEmpty(t, stripTerminalSequences(text.renderReviewStateGlyph(state)), state)
		// With icons off, the glyph falls back to plain ASCII.
		glyph := stripTerminalSequences(off.renderReviewStateGlyph(state))
		assert.Contains(t, []string{"+", "!", "~"}, glyph, state)
	}

	assert.Empty(t, nerd.renderReviewStateGlyph("SOMETHING_ELSE"))
	assert.Empty(t, nerd.renderReviewStateGlyph(""))
}

// TestUpdateDispatchesPRReviewersLoaded guards the wiring rather than the
// handler: an earlier version handled the message correctly but never received
// it, because the message type was missing from the top-level switch.
func TestUpdateDispatchesPRReviewersLoaded(t *testing.T) {
	t.Run("a successful lookup reaches the Info pane", func(t *testing.T) {
		wt := reviewerWorktree(t, "feature", 7)
		m := newReviewerModel(t, &config.AppConfig{IconSet: "text"}, wt)
		m.infoContentWidth = 120
		m.infoContent = "stale"
		token, ok := m.cache.reviewerCache.MarkFetching("feature#7")
		require.True(t, ok)

		updated, _ := m.Update(prReviewersLoadedMsg{
			token: token,
			summary: &models.PRReviewerSummary{
				Total:     1,
				Reviewers: []*models.PRReviewer{{Login: "alice", State: models.ReviewStateApproved}},
			},
		})

		content := stripTerminalSequences(updated.(*Model).infoContent)
		assert.Contains(t, content, "Reviewers:")
		assert.Contains(t, content, "@alice")

		summary, cached := m.cache.reviewerCache.Get("feature#7")
		require.True(t, cached, "the result must be cached")
		assert.Equal(t, 1, summary.Total)
	})

	t.Run("a failed lookup releases its claim", func(t *testing.T) {
		wt := reviewerWorktree(t, "feature", 7)
		m := newReviewerModel(t, &config.AppConfig{IconSet: "text"}, wt)
		token, ok := m.cache.reviewerCache.MarkFetching("feature#7")
		require.True(t, ok)

		m.Update(prReviewersLoadedMsg{token: token, err: errors.New("no credentials")})

		next, ok := m.cache.reviewerCache.MarkFetching("feature#7")
		assert.True(t, ok, "the key must not stay wedged as still fetching")
		m.cache.reviewerCache.Complete(next, nil, nil)
	})
}

func TestRenderReviewersLineRemainder(t *testing.T) {
	newModel := func(t *testing.T, wt *models.WorktreeInfo) *Model {
		t.Helper()
		return newReviewerModel(t, &config.AppConfig{IconSet: "text"}, wt)
	}

	t.Run("the remainder counts reviewers the forge did not name", func(t *testing.T) {
		wt := reviewerWorktree(t, "feature", 1)
		m := newModel(t, wt)
		// Nine reviews submitted, one identity resolved: the eight unnamed
		// reviewers must still be reported.
		cacheReviewers(m, "feature#1", &models.PRReviewerSummary{
			Total:     9,
			Reviewers: []*models.PRReviewer{{Login: "alice", State: models.ReviewStateApproved}},
		})

		line := stripTerminalSequences(m.renderReviewersLine(wt, 120))

		assert.Contains(t, line, "Reviewers: 9")
		assert.Contains(t, line, "@alice")
		assert.Contains(t, line, "+8")
	})

	t.Run("the remainder is dropped rather than overflowing a narrow pane", func(t *testing.T) {
		wt := reviewerWorktree(t, "feature", 1)
		m := newModel(t, wt)
		reviewers := make([]*models.PRReviewer, 0, 8)
		for _, login := range []string{"alice", "bob", "carol", "dave", "erin", "frank", "grace", "heidi"} {
			reviewers = append(reviewers, &models.PRReviewer{Login: login, State: models.ReviewStateApproved})
		}
		cacheReviewers(m, "feature#1", &models.PRReviewerSummary{Total: 8, Reviewers: reviewers})

		for width := 14; width <= 80; width++ {
			line := m.renderReviewersLine(wt, width)
			assert.LessOrEqualf(t, lipgloss.Width(line), width, "line overflows at width %d: %q", width, line)
		}
	})

	t.Run("a pane too narrow for anybody still reports the count", func(t *testing.T) {
		wt := reviewerWorktree(t, "feature", 1)
		m := newModel(t, wt)
		cacheReviewers(m, "feature#1", &models.PRReviewerSummary{
			Total:     3,
			Reviewers: []*models.PRReviewer{{Login: "alice", State: models.ReviewStateApproved}},
		})

		line := stripTerminalSequences(m.renderReviewersLine(wt, 16))

		assert.Contains(t, line, "Reviewers: 3")
		assert.NotContains(t, line, "@alice")
	})
}

// TestRenderReviewerEntryBotKeepsMarker mirrors the author treatment: an avatar
// tells you who reviewed, the marker tells you it was not a person.
func TestRenderReviewerEntryBotKeepsMarker(t *testing.T) {
	wt := reviewerWorktree(t, "feature", 1)
	m := newReviewerModel(t, &config.AppConfig{AvatarBadges: "always", IconSet: "text"}, wt)
	avatarURL := "https://example.com/copilot.png"
	m.avatarStates[avatarURL] = &avatarRuntimeState{
		status:     avatarStateLoaded,
		registered: true,
		image:      &services.AvatarImage{URL: avatarURL, Key: "copilot", PNG: []byte("png")},
	}

	entry := m.renderReviewerEntry(&models.PRReviewer{
		Login:     "copilot",
		AvatarURL: avatarURL,
		IsBot:     true,
		State:     models.ReviewStateCommented,
	})

	assert.Contains(t, entry, kittyPlaceholderRune, "the avatar is drawn")
	assert.Contains(t, stripTerminalSequences(entry), uiIcon(UIIconBot), "the bot marker is kept alongside it")
}

// TestStatusUpdateGuardsStaleWidth covers info content laid out in the
// background for a pane width that has since changed.
func TestStatusUpdateGuardsStaleWidth(t *testing.T) {
	t.Run("content built for the current width is used as is", func(t *testing.T) {
		wt := reviewerWorktree(t, "feature", 1)
		m := newReviewerModel(t, &config.AppConfig{IconSet: "text"}, wt)
		m.infoContentWidth = 80

		updated, _ := m.Update(statusUpdatedMsg{info: "built at 80", infoWidth: 80, path: wt.Path})

		assert.Equal(t, "built at 80", updated.(*Model).infoContent)
	})

	t.Run("content built for another width is rebuilt", func(t *testing.T) {
		wt := reviewerWorktree(t, "feature", 1)
		m := newReviewerModel(t, &config.AppConfig{IconSet: "text"}, wt)
		m.infoContentWidth = 120
		cacheReviewers(m, "feature#1", &models.PRReviewerSummary{
			Total:     1,
			Reviewers: []*models.PRReviewer{{Login: "alice", State: models.ReviewStateApproved}},
		})

		updated, _ := m.Update(statusUpdatedMsg{info: "built at 40", infoWidth: 40, path: wt.Path})

		content := stripTerminalSequences(updated.(*Model).infoContent)
		assert.NotEqual(t, "built at 40", content)
		assert.Contains(t, content, "@alice")
	})
}
