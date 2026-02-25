package app

import (
	"testing"

	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestRenderCIStatusPill(t *testing.T) {
	t.Parallel()

	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	tests := []struct {
		name       string
		conclusion string
		wantLabel  string
	}{
		{"success", "success", "SUCCESS"},
		{"failure", "failure", "FAILED"},
		{"pending", "pending", "PENDING"},
		{"empty treated as pending", "", "PENDING"},
		{"skipped", "skipped", "SKIPPED"},
		{"cancelled", "cancelled", "CANCELLED"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := m.renderCIStatusPill(tt.conclusion)

			// Should contain Powerline edges
			assert.Contains(t, result, "\ue0b6", "should have left Powerline edge")
			assert.Contains(t, result, "\ue0b4", "should have right Powerline edge")
			// Should contain the text label
			assert.Contains(t, result, tt.wantLabel)
		})
	}
}

func TestRenderPRStatePill(t *testing.T) {
	t.Parallel()

	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	tests := []struct {
		name      string
		state     string
		wantLabel string
	}{
		{"open", "OPEN", "OPEN"},
		{"merged", "MERGED", "MERGED"},
		{"closed", "CLOSED", "CLOSED"},
		{"unknown", "DRAFT", "DRAFT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := m.renderPRStatePill(tt.state)

			// Should contain Powerline edges
			assert.Contains(t, result, "\ue0b6", "should have left Powerline edge")
			assert.Contains(t, result, "\ue0b4", "should have right Powerline edge")
			// Should contain the text label
			assert.Contains(t, result, tt.wantLabel)
		})
	}
}

func TestCIConclusionLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		conclusion string
		want       string
	}{
		{"success", "success", "SUCCESS"},
		{"failure", "failure", "FAILED"},
		{"pending", "pending", "PENDING"},
		{"empty", "", "PENDING"},
		{"skipped", "skipped", "SKIPPED"},
		{"cancelled", "cancelled", "CANCELLED"},
		{"unknown", "foobar", "FOOBAR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, ciConclusionLabel(tt.conclusion))
		})
	}
}
