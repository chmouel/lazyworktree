package services

import (
	"sync"
	"time"

	"github.com/chmouel/lazyworktree/internal/models"
)

const (
	// DefaultReviewerCacheTTL is how long a successful reviewer lookup is reused.
	DefaultReviewerCacheTTL = 60 * time.Second
	// DefaultReviewerRetryBackoff is how long to wait before retrying a lookup
	// that failed, so a missing CLI or a broken connection is not hammered.
	DefaultReviewerRetryBackoff = 30 * time.Second
)

// ReviewerToken identifies one in-flight reviewer lookup. It is handed out by
// MarkFetching and handed back to Complete, which is how the cache tells a
// current response apart from one that was overtaken by events.
type ReviewerToken struct {
	key        string
	id         uint64
	generation uint64
}

// Key returns the cache key the token was issued for.
func (t ReviewerToken) Key() string { return t.key }

// Valid reports whether the token was issued by MarkFetching.
func (t ReviewerToken) Valid() bool { return t.id != 0 }

// PRReviewerCache stores reviewer lookups for the worktree currently on screen.
type PRReviewerCache interface {
	// Get returns the cached summary for a key, if a lookup has succeeded.
	Get(key string) (*models.PRReviewerSummary, bool)

	// ShouldFetch reports whether a fresh lookup is warranted: no successful
	// result within ttl, no failure within retryBackoff, and nothing in flight.
	ShouldFetch(key string, ttl, retryBackoff time.Duration) bool

	// MarkFetching claims a key for one lookup, returning false when another
	// lookup already holds it.
	MarkFetching(key string) (ReviewerToken, bool)

	// Complete finishes the lookup a token was issued for, releasing the claim
	// and recording the outcome. It does nothing when the token no longer owns
	// the key, or when the cache has been invalidated since the token was
	// issued.
	Complete(token ReviewerToken, summary *models.PRReviewerSummary, err error)

	// Invalidate makes every entry due for a fresh lookup and drops the results
	// of anything still in flight. Summaries already fetched are kept, so a
	// refresh that fails still has last known good reviewers to show.
	Invalidate()
}

type reviewerCacheEntry struct {
	summary   *models.PRReviewerSummary
	ok        bool
	fetchedAt time.Time
	// lastFailedAt is zero unless the most recent attempt failed, so a failure
	// that follows an earlier success still holds off the next attempt.
	lastFailedAt time.Time
}

type prReviewerCache struct {
	mu         sync.Mutex
	entries    map[string]*reviewerCacheEntry
	inFlight   map[string]uint64
	generation uint64
	nextID     uint64
}

// NewPRReviewerCache creates a thread-safe reviewer cache.
func NewPRReviewerCache() PRReviewerCache {
	return &prReviewerCache{
		entries:  make(map[string]*reviewerCacheEntry),
		inFlight: make(map[string]uint64),
	}
}

func (c *prReviewerCache) Get(key string) (*models.PRReviewerSummary, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok || !entry.ok {
		return nil, false
	}
	return entry.summary, true
}

func (c *prReviewerCache) ShouldFetch(key string, ttl, retryBackoff time.Duration) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, busy := c.inFlight[key]; busy {
		return false
	}
	entry, ok := c.entries[key]
	if !ok {
		return true
	}
	if entry.ok && time.Since(entry.fetchedAt) < ttl {
		return false
	}
	if !entry.lastFailedAt.IsZero() && time.Since(entry.lastFailedAt) < retryBackoff {
		return false
	}
	return true
}

func (c *prReviewerCache) MarkFetching(key string) (ReviewerToken, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, busy := c.inFlight[key]; busy {
		return ReviewerToken{}, false
	}
	c.nextID++
	c.inFlight[key] = c.nextID
	return ReviewerToken{key: key, id: c.nextID, generation: c.generation}, true
}

func (c *prReviewerCache) Complete(token ReviewerToken, summary *models.PRReviewerSummary, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !token.Valid() {
		return
	}
	// Releasing the claim and recording the outcome happen together, so a token
	// that has already been completed, or that belongs to a superseded lookup,
	// can neither free another lookup's claim nor write to the entry.
	if id, busy := c.inFlight[token.key]; !busy || id != token.id {
		return
	}
	delete(c.inFlight, token.key)

	// A lookup that was in flight when the cache was cleared carries stale
	// data by definition, so its result is dropped.
	if token.generation != c.generation {
		return
	}

	now := time.Now()
	entry, ok := c.entries[token.key]
	if !ok {
		entry = &reviewerCacheEntry{}
		c.entries[token.key] = entry
	}
	if err != nil {
		// A failed lookup must not erase a result we already have, nor be
		// mistaken for "this change request has no reviewers". Recording when
		// it failed keeps the retry backoff in force even when the entry still
		// holds an earlier success.
		entry.lastFailedAt = now
		return
	}
	entry.summary = summary
	entry.ok = true
	entry.fetchedAt = now
	entry.lastFailedAt = time.Time{}
}

func (c *prReviewerCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Entries are kept but aged out, so the next lookup runs immediately whilst
	// the previous answer remains on screen until a better one arrives.
	for _, entry := range c.entries {
		entry.fetchedAt = time.Time{}
		entry.lastFailedAt = time.Time{}
	}
	c.inFlight = make(map[string]uint64)
	c.generation++
}
