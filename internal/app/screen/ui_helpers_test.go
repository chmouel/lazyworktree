package screen

import (
	"testing"

	"github.com/chmouel/lazyworktree/internal/theme"
	"github.com/stretchr/testify/assert"
)

type statusIconProvider struct {
	clean string
	dirty string
}

func (p *statusIconProvider) GetPRIcon() string {
	return ""
}

func (p *statusIconProvider) GetIssueIcon() string {
	return ""
}

func (p *statusIconProvider) GetCIIcon(conclusion string) string {
	return ""
}

func (p *statusIconProvider) GetUIIcon(icon UIIcon) string {
	switch icon {
	case UIIconStatusClean:
		return p.clean
	case UIIconStatusDirty:
		return p.dirty
	default:
		return ""
	}
}

func TestStatusIndicatorUsesIconProvider(t *testing.T) {
	prev := currentIconProvider
	t.Cleanup(func() { SetIconProvider(prev) })

	SetIconProvider(&statusIconProvider{clean: "C", dirty: "D"})

	if got := statusIndicator(true, true); got != "C" {
		t.Fatalf("expected clean icon, got %q", got)
	}
	if got := statusIndicator(false, true); got != "D" {
		t.Fatalf("expected dirty icon, got %q", got)
	}
	if got := statusIndicator(true, false); got != " " {
		t.Fatalf("expected clean fallback, got %q", got)
	}
	if got := statusIndicator(false, false); got != "~" {
		t.Fatalf("expected dirty fallback, got %q", got)
	}
}

func TestRenderCIBubble(t *testing.T) {
	t.Parallel()

	thm := theme.Dracula()

	tests := []struct {
		name       string
		conclusion string
		wantIcon   string
	}{
		{"success", "success", "S"},
		{"failure", "failure", "F"},
		{"pending", "pending", "P"},
		{"empty", "", "?"},
		{"skipped", "skipped", "-"},
		{"cancelled", "cancelled", "C"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := renderCIBubble(thm, tt.conclusion, false)

			// Should contain Powerline edges
			assert.Contains(t, result, "\ue0b6", "should have left Powerline edge")
			assert.Contains(t, result, "\ue0b4", "should have right Powerline edge")
			// Should contain the icon text
			assert.Contains(t, result, tt.wantIcon)
		})
	}
}

func TestCIConclusionColors(t *testing.T) {
	t.Parallel()

	thm := theme.Dracula()

	tests := []struct {
		name       string
		conclusion string
		wantBg     string
		wantFg     string
	}{
		{"success uses SuccessFg bg", "success", "SuccessFg", "AccentFg"},
		{"failure uses ErrorFg bg", "failure", "ErrorFg", "AccentFg"},
		{"pending uses WarnFg bg", "pending", "WarnFg", "AccentFg"},
		{"empty uses WarnFg bg", "", "WarnFg", "AccentFg"},
		{"skipped uses BorderDim bg", "skipped", "BorderDim", "TextFg"},
		{"cancelled uses BorderDim bg", "cancelled", "BorderDim", "TextFg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			bg, fg := ciConclusionColors(thm, tt.conclusion)

			// Verify correct theme colours are returned
			switch tt.wantBg {
			case "SuccessFg":
				assert.Equal(t, thm.SuccessFg, bg)
			case "ErrorFg":
				assert.Equal(t, thm.ErrorFg, bg)
			case "WarnFg":
				assert.Equal(t, thm.WarnFg, bg)
			case "BorderDim":
				assert.Equal(t, thm.BorderDim, bg)
			}
			switch tt.wantFg {
			case "AccentFg":
				assert.Equal(t, thm.AccentFg, fg)
			case "TextFg":
				assert.Equal(t, thm.TextFg, fg)
			}
		})
	}
}
