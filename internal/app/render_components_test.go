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

func TestRenderPRStateBadge(t *testing.T) {
	t.Parallel()

	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	tests := []struct {
		name      string
		state     string
		wantLabel string
	}{
		{"open", "OPEN", "Open"},
		{"merged", "MERGED", "Merged"},
		{"closed", "CLOSED", "Closed"},
		{"unknown", "DRAFT", "Draft"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := m.renderPRStateBadge(tt.state, false)
			plain := stripTerminalSequences(result)

			assert.Equal(t, tt.wantLabel, plain)
			assert.NotContains(t, result, "\ue0b6", "badge should not use Powerline edges")
			assert.NotContains(t, result, "\ue0b4", "badge should not use Powerline edges")
		})
	}
}

func TestRenderPRStateBadgeWithIconsUsesLazyGitShape(t *testing.T) {
	t.Parallel()

	cfg := &config.AppConfig{WorktreeDir: t.TempDir(), IconSet: "nerd-font-v3"}
	m := NewModel(cfg, "")

	result := stripTerminalSequences(m.renderPRStateBadge(prStateMerged, cfg.NerdFontIconsEnabled()))

	assert.Equal(t, "\ue0b6 Merged\ue0b4", result)
}

func TestRenderPRStateBadgeTextModeAvoidsNerdFontGlyphs(t *testing.T) {
	t.Parallel()

	cfg := &config.AppConfig{WorktreeDir: t.TempDir(), IconSet: "text"}
	m := NewModel(cfg, "")

	result := stripTerminalSequences(m.renderPRStateBadge(prStateMerged, cfg.NerdFontIconsEnabled()))

	assert.Equal(t, "Merged", result)
	assert.Empty(t, prRemoteIcon("https://github.com/org/repo/pull/42", cfg.NerdFontIconsEnabled()))
}

func TestRenderTagPill(t *testing.T) {
	t.Parallel()

	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	result := m.renderTagPill("bug")
	assert.Contains(t, result, "«bug»", "should render guillemet-wrapped tag")
	assert.NotContains(t, result, "\ue0b6", "should not have Powerline edges")
	assert.NotContains(t, result, "\ue0b4", "should not have Powerline edges")
}

func TestRenderTagPills(t *testing.T) {
	t.Parallel()

	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	result := m.renderTagPills([]string{"bug", "frontend"})
	assert.Contains(t, result, "bug")
	assert.Contains(t, result, "frontend")

	empty := m.renderTagPills(nil)
	assert.Empty(t, empty)
}

func TestRenderPlainTagPills(t *testing.T) {
	t.Parallel()

	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	result := m.renderPlainTagPills([]string{"bug", "frontend"})
	assert.Equal(t, "«bug» «frontend»", result)
	assert.NotContains(t, result, "\x1b[")
}

func TestTagColorDeterminism(t *testing.T) {
	t.Parallel()

	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	// Same tag must always produce the same colour
	c1 := m.tagPillColor("bug")
	c2 := m.tagPillColor("bug")
	assert.Equal(t, c1, c2, "same tag should yield same colour")

	// Different tags can differ (not guaranteed, but "bug" vs "feature" do differ
	// because their byte sums differ mod 6)
	c3 := m.tagPillColor("feature")
	_ = c3 // just ensure it doesn't panic
}

func TestTagColorAvoidsAccentDimSlot(t *testing.T) {
	t.Parallel()

	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	assert.Equal(t, m.theme.TextFg, m.tagPillColor("zaki"))
	assert.NotEqual(t, m.theme.AccentDim, m.tagPillColor("zaki"))
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
