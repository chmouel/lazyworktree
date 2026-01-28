package services

import (
	"testing"
	"time"

	"github.com/chmouel/lazyworktree/internal/models"
)

func TestCICheckCache_GetSet(t *testing.T) {
	cache := NewCICheckCache()

	// Test Get on empty cache
	checks, fetchedAt, ok := cache.Get("branch1")
	if ok {
		t.Error("Get() on empty cache should return false")
	}
	if checks != nil {
		t.Error("Get() on empty cache should return nil checks")
	}
	if !fetchedAt.IsZero() {
		t.Error("Get() on empty cache should return zero time")
	}

	// Test Set and Get
	testChecks := []*models.CICheck{
		{Name: "build", Conclusion: "success"},
		{Name: "test", Conclusion: "failure"},
	}
	cache.Set("branch1", testChecks)

	checks, fetchedAt, ok = cache.Get("branch1")
	if !ok {
		t.Error("Get() after Set should return true")
	}
	if len(checks) != 2 {
		t.Errorf("Get() returned %d checks, want 2", len(checks))
	}
	if checks[0].Name != "build" || checks[1].Name != "test" {
		t.Error("Get() returned wrong checks")
	}
	if time.Since(fetchedAt) > time.Second {
		t.Error("Get() returned stale fetchedAt time")
	}

	// Test overwriting
	newChecks := []*models.CICheck{
		{Name: "lint", Conclusion: "success"},
	}
	cache.Set("branch1", newChecks)

	checks, _, ok = cache.Get("branch1")
	if !ok {
		t.Error("Get() after overwrite should return true")
	}
	if len(checks) != 1 || checks[0].Name != "lint" {
		t.Error("Get() after overwrite returned wrong checks")
	}
}

func TestCICheckCache_Clear(t *testing.T) {
	cache := NewCICheckCache()

	// Add entries
	cache.Set("branch1", []*models.CICheck{{Name: "check1"}})
	cache.Set("branch2", []*models.CICheck{{Name: "check2"}})

	// Verify entries exist
	_, _, ok1 := cache.Get("branch1")
	_, _, ok2 := cache.Get("branch2")
	if !ok1 || !ok2 {
		t.Error("Entries should exist before Clear")
	}

	// Clear and verify
	cache.Clear()

	_, _, ok1 = cache.Get("branch1")
	_, _, ok2 = cache.Get("branch2")
	if ok1 || ok2 {
		t.Error("Entries should not exist after Clear")
	}
}

func TestCICheckCache_IsFresh(t *testing.T) {
	cache := NewCICheckCache()

	// Test non-existent entry
	if cache.IsFresh("branch1", time.Hour) {
		t.Error("IsFresh() should return false for non-existent entry")
	}

	// Test fresh entry
	cache.Set("branch1", []*models.CICheck{{Name: "check1"}})
	if !cache.IsFresh("branch1", time.Hour) {
		t.Error("IsFresh() should return true for fresh entry with long TTL")
	}

	// Test with very short TTL (entry will be stale immediately)
	if cache.IsFresh("branch1", 0) {
		t.Error("IsFresh() should return false with zero TTL")
	}

	// Test with negative TTL (always stale)
	if cache.IsFresh("branch1", -time.Second) {
		t.Error("IsFresh() should return false with negative TTL")
	}
}

func TestCICheckCache_EmptyChecks(t *testing.T) {
	cache := NewCICheckCache()

	// Set empty checks
	cache.Set("branch1", []*models.CICheck{})

	checks, _, ok := cache.Get("branch1")
	if !ok {
		t.Error("Get() should return true for empty checks slice")
	}
	if checks == nil {
		t.Error("Get() should return empty slice, not nil")
	}
	if len(checks) != 0 {
		t.Errorf("Get() returned %d checks, want 0", len(checks))
	}
}

func TestCICheckCache_NilChecks(t *testing.T) {
	cache := NewCICheckCache()

	// Set nil checks
	cache.Set("branch1", nil)

	checks, _, ok := cache.Get("branch1")
	if !ok {
		t.Error("Get() should return true for nil checks")
	}
	if checks != nil {
		t.Error("Get() should return nil when nil was set")
	}
}

func TestCICheckCache_MultipleBranches(t *testing.T) {
	cache := NewCICheckCache()

	// Add entries for multiple branches
	cache.Set("main", []*models.CICheck{{Name: "main-check"}})
	cache.Set("feature", []*models.CICheck{{Name: "feature-check"}})
	cache.Set("bugfix", []*models.CICheck{{Name: "bugfix-check"}})

	// Verify each branch has its own data
	checks, _, ok := cache.Get("main")
	if !ok || len(checks) != 1 || checks[0].Name != "main-check" {
		t.Error("main branch has wrong data")
	}

	checks, _, ok = cache.Get("feature")
	if !ok || len(checks) != 1 || checks[0].Name != "feature-check" {
		t.Error("feature branch has wrong data")
	}

	checks, _, ok = cache.Get("bugfix")
	if !ok || len(checks) != 1 || checks[0].Name != "bugfix-check" {
		t.Error("bugfix branch has wrong data")
	}

	// Verify non-existent branch
	_, _, ok = cache.Get("nonexistent")
	if ok {
		t.Error("nonexistent branch should not exist")
	}
}
