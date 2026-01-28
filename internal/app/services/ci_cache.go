package services

import (
	"sync"
	"time"

	"github.com/chmouel/lazyworktree/internal/models"
)

// DefaultCICacheTTL is the default time-to-live for CI cache entries.
const DefaultCICacheTTL = 30 * time.Second

// CICheckCache provides cache management for CI check data.
type CICheckCache interface {
	// Get retrieves cached CI checks for a branch.
	// Returns the checks, fetch time, and whether the entry exists.
	Get(branch string) ([]*models.CICheck, time.Time, bool)

	// Set stores CI checks for a branch with the current timestamp.
	Set(branch string, checks []*models.CICheck)

	// Clear removes all cached entries.
	Clear()

	// IsFresh returns true if the cache entry exists and is within the TTL.
	IsFresh(branch string, ttl time.Duration) bool
}

// ciCacheEntry stores CI checks with their fetch timestamp.
type ciCacheEntry struct {
	checks    []*models.CICheck
	fetchedAt time.Time
}

// ciCheckCache implements CICheckCache with an in-memory map.
type ciCheckCache struct {
	mu      sync.RWMutex
	entries map[string]*ciCacheEntry
}

// NewCICheckCache creates a new thread-safe CI check cache.
func NewCICheckCache() CICheckCache {
	return &ciCheckCache{
		entries: make(map[string]*ciCacheEntry),
	}
}

func (c *ciCheckCache) Get(branch string) ([]*models.CICheck, time.Time, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[branch]
	if !ok {
		return nil, time.Time{}, false
	}
	return entry.checks, entry.fetchedAt, true
}

func (c *ciCheckCache) Set(branch string, checks []*models.CICheck) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[branch] = &ciCacheEntry{
		checks:    checks,
		fetchedAt: time.Now(),
	}
}

func (c *ciCheckCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*ciCacheEntry)
}

func (c *ciCheckCache) IsFresh(branch string, ttl time.Duration) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[branch]
	if !ok {
		return false
	}
	return time.Since(entry.fetchedAt) < ttl
}
