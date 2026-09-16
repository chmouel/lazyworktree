package app

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/chmouel/lazyworktree/internal/models"
)

const (
	// maxReviewerLoginWidth caps a single reviewer name so one very long login
	// cannot crowd out everybody else.
	maxReviewerLoginWidth = 16
	// defaultInfoContentWidth is the assumed pane width before the first
	// layout has been applied.
	defaultInfoContentWidth = 60
)

// renderReviewersLine renders the reviewer summary for a worktree's change
// request, or an empty string when there is nothing to report. Entries are
// added only whilst they fit the pane, with the remainder shown as a count.
func (m *Model) renderReviewersLine(wt *models.WorktreeInfo, contentWidth int) string {
	summary := m.reviewersForWorktree(wt)
	if summary == nil || summary.Total <= 0 {
		return ""
	}

	labelStyle := lipgloss.NewStyle().Foreground(m.theme.TextFg).Bold(true)
	label := fmt.Sprintf("  %s %d", labelStyle.Render("Reviewers:"), summary.Total)

	available := contentWidth
	if available <= 0 {
		available = defaultInfoContentWidth
	}

	// The remainder counts against the forge-reported total, so reviewers the
	// forge did not name — a deleted account, or one beyond the page we asked
	// for — are still accounted for rather than quietly dropped.
	suffixFor := func(shown int) string {
		if hidden := summary.Total - shown; hidden > 0 {
			return fmt.Sprintf("  +%d", hidden)
		}
		return ""
	}

	line := label
	shown := 0
	for _, reviewer := range summary.Reviewers {
		if shown >= maxDisplayedReviewers {
			break
		}
		entry := "  " + m.renderReviewerEntry(reviewer)
		// Keep room for the remainder this entry would leave behind.
		reserved := lipgloss.Width(suffixFor(shown + 1))
		if lipgloss.Width(line)+lipgloss.Width(entry)+reserved > available {
			break
		}
		line += entry
		shown++
	}

	// A pane too narrow even for the remainder keeps the count on its own,
	// rather than overflowing the pane.
	if suffix := suffixFor(shown); suffix != "" && lipgloss.Width(line)+lipgloss.Width(suffix) <= available {
		line += suffix
	}
	return line
}

// renderReviewerEntry renders one reviewer: their avatar when we have one, a
// bot marker when they are not a person, their name, and their review state.
func (m *Model) renderReviewerEntry(reviewer *models.PRReviewer) string {
	if reviewer == nil {
		return ""
	}

	var b strings.Builder
	if badge := m.renderAvatarBadgeForURL(reviewer.AvatarURL); badge != "" {
		b.WriteString(badge)
		b.WriteString(" ")
	}
	// A bot keeps its marker even once its avatar arrives, so it is never
	// mistaken for a person.
	if reviewer.IsBot {
		b.WriteString(iconPrefix(UIIconBot, m.config.IconsEnabled()))
	}

	name := truncateDisplay(reviewer.Login, maxReviewerLoginWidth)
	b.WriteString(lipgloss.NewStyle().Foreground(m.theme.TextFg).Render("@" + name))

	if glyph := m.renderReviewStateGlyph(reviewer.State); glyph != "" {
		b.WriteString(" ")
		b.WriteString(glyph)
	}
	return b.String()
}

// renderReviewStateGlyph renders the outcome of a review in theme colours.
func (m *Model) renderReviewStateGlyph(state string) string {
	var icon UIIcon
	var fallback string
	var colour color.Color
	switch state {
	case models.ReviewStateApproved:
		icon, fallback, colour = UIIconReviewApproved, "+", m.theme.SuccessFg
	case models.ReviewStateChangesRequested:
		icon, fallback, colour = UIIconReviewChangesRequested, "!", m.theme.ErrorFg
	case models.ReviewStateCommented, models.ReviewStateDismissed:
		icon, fallback, colour = UIIconReviewCommented, "~", m.theme.MutedFg
	default:
		return ""
	}
	glyph := ""
	if m.config.IconsEnabled() {
		glyph = uiIcon(icon)
	}
	if glyph == "" {
		glyph = fallback
	}
	return lipgloss.NewStyle().Foreground(colour).Render(glyph)
}

// truncateDisplay shortens text to width display cells, marking the cut. A
// non-positive width means no limit.
func truncateDisplay(text string, width int) string {
	text = strings.TrimSpace(text)
	if width <= 0 || lipgloss.Width(text) <= width {
		return text
	}
	runes := []rune(text)
	for len(runes) > 0 {
		candidate := string(runes) + "…"
		if lipgloss.Width(candidate) <= width {
			return candidate
		}
		runes = runes[:len(runes)-1]
	}
	return ""
}
