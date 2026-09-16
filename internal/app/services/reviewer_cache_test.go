package services

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chmouel/lazyworktree/internal/models"
)

func TestPRReviewerCacheFreshness(t *testing.T) {
	t.Parallel()

	t.Run("an empty result is still cached for the full window", func(t *testing.T) {
		t.Parallel()
		cache := NewPRReviewerCache()
		token, ok := cache.MarkFetching("main#1")
		require.True(t, ok)

		cache.Complete(token, &models.PRReviewerSummary{}, nil)

		summary, found := cache.Get("main#1")
		assert.True(t, found)
		assert.Equal(t, 0, summary.Total)
		assert.False(t, cache.ShouldFetch("main#1", time.Minute, time.Second))
	})

	t.Run("an expired result is fetched again", func(t *testing.T) {
		t.Parallel()
		cache := NewPRReviewerCache()
		token, _ := cache.MarkFetching("main#1")
		cache.Complete(token, &models.PRReviewerSummary{Total: 1}, nil)

		assert.True(t, cache.ShouldFetch("main#1", time.Nanosecond, time.Nanosecond))
	})

	t.Run("a key nobody has looked up is fetched", func(t *testing.T) {
		t.Parallel()
		cache := NewPRReviewerCache()
		assert.True(t, cache.ShouldFetch("main#1", time.Minute, time.Second))
	})

	t.Run("a lookup already in flight is not repeated", func(t *testing.T) {
		t.Parallel()
		cache := NewPRReviewerCache()
		_, ok := cache.MarkFetching("main#1")
		require.True(t, ok)

		assert.False(t, cache.ShouldFetch("main#1", time.Minute, time.Second))
		_, ok = cache.MarkFetching("main#1")
		assert.False(t, ok)
	})

	t.Run("completing a lookup makes the key available again", func(t *testing.T) {
		t.Parallel()
		cache := NewPRReviewerCache()
		token, _ := cache.MarkFetching("main#1")
		cache.Complete(token, &models.PRReviewerSummary{Total: 1}, nil)

		_, ok := cache.MarkFetching("main#1")
		assert.True(t, ok)
	})
}

func TestPRReviewerCacheFailures(t *testing.T) {
	t.Parallel()

	t.Run("a failure keeps the result we already had", func(t *testing.T) {
		t.Parallel()
		cache := NewPRReviewerCache()
		first, _ := cache.MarkFetching("main#1")
		cache.Complete(first, &models.PRReviewerSummary{Total: 2}, nil)

		second, _ := cache.MarkFetching("main#1")
		cache.Complete(second, nil, errors.New("gh: not authenticated"))

		summary, found := cache.Get("main#1")
		require.True(t, found)
		assert.Equal(t, 2, summary.Total)
	})

	t.Run("a failure is not mistaken for an empty result", func(t *testing.T) {
		t.Parallel()
		cache := NewPRReviewerCache()
		token, _ := cache.MarkFetching("main#1")
		cache.Complete(token, nil, errors.New("boom"))

		_, found := cache.Get("main#1")
		assert.False(t, found)
	})

	t.Run("a failure is not retried immediately", func(t *testing.T) {
		t.Parallel()
		cache := NewPRReviewerCache()
		token, _ := cache.MarkFetching("main#1")
		cache.Complete(token, nil, errors.New("boom"))

		assert.False(t, cache.ShouldFetch("main#1", time.Minute, time.Minute))
		assert.True(t, cache.ShouldFetch("main#1", time.Minute, time.Nanosecond))
	})
}

func TestPRReviewerCacheTokens(t *testing.T) {
	t.Parallel()

	t.Run("an unissued token changes nothing", func(t *testing.T) {
		t.Parallel()
		cache := NewPRReviewerCache()
		cache.Complete(ReviewerToken{}, &models.PRReviewerSummary{Total: 5}, nil)

		_, found := cache.Get("main#1")
		assert.False(t, found)
	})

	t.Run("a token cannot be completed twice", func(t *testing.T) {
		t.Parallel()
		cache := NewPRReviewerCache()
		token, _ := cache.MarkFetching("main#1")
		cache.Complete(token, &models.PRReviewerSummary{Total: 1}, nil)

		next, ok := cache.MarkFetching("main#1")
		require.True(t, ok)

		// The stale token must not write to the entry, nor release the claim
		// the new lookup now holds.
		cache.Complete(token, &models.PRReviewerSummary{Total: 99}, nil)

		summary, _ := cache.Get("main#1")
		assert.Equal(t, 1, summary.Total)
		_, ok = cache.MarkFetching("main#1")
		assert.False(t, ok, "the stale token released a claim it did not own")

		cache.Complete(next, &models.PRReviewerSummary{Total: 3}, nil)
		summary, _ = cache.Get("main#1")
		assert.Equal(t, 3, summary.Total)
	})

	t.Run("a lookup that survives a clear cannot write stale data", func(t *testing.T) {
		t.Parallel()
		cache := NewPRReviewerCache()
		first, _ := cache.MarkFetching("main#1")

		cache.Invalidate()

		second, ok := cache.MarkFetching("main#1")
		require.True(t, ok)

		cache.Complete(first, &models.PRReviewerSummary{Total: 99}, nil)
		_, found := cache.Get("main#1")
		assert.False(t, found, "a superseded lookup wrote to the cache")

		cache.Complete(second, &models.PRReviewerSummary{Total: 4}, nil)
		summary, found := cache.Get("main#1")
		require.True(t, found)
		assert.Equal(t, 4, summary.Total)
	})

	t.Run("invalidating keeps the answer but makes the lookup due", func(t *testing.T) {
		t.Parallel()
		cache := NewPRReviewerCache()
		token, _ := cache.MarkFetching("main#1")
		cache.Complete(token, &models.PRReviewerSummary{Total: 1}, nil)

		cache.Invalidate()

		summary, found := cache.Get("main#1")
		require.True(t, found, "a refresh must not blank the pane whilst it runs")
		assert.Equal(t, 1, summary.Total)
		assert.True(t, cache.ShouldFetch("main#1", time.Minute, time.Minute))
	})

	t.Run("a refresh that fails falls back on the previous answer", func(t *testing.T) {
		t.Parallel()
		cache := NewPRReviewerCache()
		token, _ := cache.MarkFetching("main#1")
		cache.Complete(token, &models.PRReviewerSummary{Total: 2}, nil)

		cache.Invalidate()

		retry, ok := cache.MarkFetching("main#1")
		require.True(t, ok)
		cache.Complete(retry, nil, errors.New("forge unreachable"))

		summary, found := cache.Get("main#1")
		require.True(t, found, "the reviewers stay on screen when the refresh fails")
		assert.Equal(t, 2, summary.Total)
	})

	t.Run("keys are independent", func(t *testing.T) {
		t.Parallel()
		cache := NewPRReviewerCache()
		token, _ := cache.MarkFetching("main#1")
		cache.Complete(token, &models.PRReviewerSummary{Total: 1}, nil)

		_, found := cache.Get("main#2")
		assert.False(t, found)
		assert.True(t, cache.ShouldFetch("main#2", time.Minute, time.Minute))
	})
}

func TestReviewerTokenKey(t *testing.T) {
	t.Parallel()

	cache := NewPRReviewerCache()
	token, _ := cache.MarkFetching("feature/x#12")
	assert.Equal(t, "feature/x#12", token.Key())
	assert.True(t, token.Valid())
	assert.False(t, ReviewerToken{}.Valid())
}

// TestReviewerCacheBacksOffAfterStaleRevalidation covers a failure that follows
// an earlier success: the good result is kept, but the backoff must still hold
// off the next attempt, or a forge that is down is hammered once per tick.
func TestReviewerCacheBacksOffAfterStaleRevalidation(t *testing.T) {
	t.Parallel()

	cache := NewPRReviewerCache()
	summary := &models.PRReviewerSummary{Total: 1}

	token, ok := cache.MarkFetching("feature#1")
	require.True(t, ok)
	cache.Complete(token, summary, nil)

	// The success has aged past its freshness window, so it is revalidated.
	require.True(t, cache.ShouldFetch("feature#1", time.Nanosecond, time.Minute))

	retry, ok := cache.MarkFetching("feature#1")
	require.True(t, ok)
	cache.Complete(retry, nil, errors.New("forge unreachable"))

	got, cached := cache.Get("feature#1")
	require.True(t, cached, "the earlier result is kept")
	assert.Equal(t, summary, got)
	assert.False(t, cache.ShouldFetch("feature#1", time.Nanosecond, time.Minute),
		"the backoff must hold even though an earlier success is still cached")
	assert.True(t, cache.ShouldFetch("feature#1", time.Nanosecond, time.Nanosecond),
		"once the backoff lapses the lookup is retried")
}
