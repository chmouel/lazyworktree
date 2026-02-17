package buildinfo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetAndGetters(t *testing.T) {
	Set("1.2.3", "abc123", "2025-01-01", "ci")

	assert.Equal(t, "1.2.3", Version())
	assert.Equal(t, "abc123", Commit())
	assert.Equal(t, "2025-01-01", Date())
	assert.Equal(t, "ci", BuiltBy())
}

func TestDefaultValues(t *testing.T) {
	Set("dev", "none", "unknown", "unknown")

	assert.Equal(t, "dev", Version())
	assert.Equal(t, "none", Commit())
	assert.Equal(t, "unknown", Date())
	assert.Equal(t, "unknown", BuiltBy())
}

func TestEnrichOverwritesDefaults(t *testing.T) {
	Set("dev", "none", "unknown", "unknown")
	Enrich()

	// After Enrich, builtBy should no longer be "unknown" because
	// runtime/debug.ReadBuildInfo() provides the Go version.
	assert.NotEqual(t, "unknown", BuiltBy(), "expected builtBy to be enriched with Go version")
}

func TestEnrichPreservesExplicitValues(t *testing.T) {
	Set("v1.0.0", "deadbeef", "2025-06-01", "goreleaser")
	Enrich()

	assert.Equal(t, "deadbeef", Commit())
	assert.Equal(t, "goreleaser", BuiltBy())
}
