package app

import (
	"charm.land/lipgloss/v2"
	"github.com/chmouel/lazyworktree/internal/theme"
)

type renderStyleCache struct {
	theme *theme.Theme

	headerAppStyle       lipgloss.Style
	headerRepoStyle      lipgloss.Style
	headerContainerStyle lipgloss.Style

	filterLabelStyle     lipgloss.Style
	filterContainerStyle lipgloss.Style

	footerStyle lipgloss.Style

	keyHintKeyStyle   lipgloss.Style
	keyHintLabelStyle lipgloss.Style

	paneIndicatorKeyStyle  lipgloss.Style
	paneFilterTextStyle    lipgloss.Style
	paneZoomTextStyle      lipgloss.Style
	paneMutedTextStyle     lipgloss.Style
	paneBubbleFocusedStyle lipgloss.Style
	paneBubbleDimStyle     lipgloss.Style

	ciIconSuccessStyle lipgloss.Style
	ciIconFailureStyle lipgloss.Style
	ciIconPendingStyle lipgloss.Style
	ciIconDefaultStyle lipgloss.Style

	baseBoxStyle lipgloss.Style
}

func (m *Model) invalidateRenderStyleCache() {
	m.renderStyles = renderStyleCache{}
}

func (m *Model) ensureRenderStyles() {
	if m.theme == nil {
		return
	}
	if m.renderStyles.theme == m.theme {
		return
	}

	m.renderStyles = renderStyleCache{
		theme: m.theme,

		headerAppStyle: lipgloss.NewStyle().
			Foreground(m.theme.Accent).
			Bold(true),
		headerRepoStyle: lipgloss.NewStyle().
			Foreground(m.theme.TextFg).
			Background(m.theme.BorderDim).
			Padding(0, 1),
		headerContainerStyle: lipgloss.NewStyle().
			Background(m.theme.AccentDim).
			Padding(0, 2).
			Align(lipgloss.Center),

		filterLabelStyle: lipgloss.NewStyle().
			Foreground(m.theme.AccentFg).
			Background(m.theme.Accent).
			Bold(true).
			Padding(0, 1),
		filterContainerStyle: lipgloss.NewStyle().
			Foreground(m.theme.TextFg).
			Padding(0, 1),

		footerStyle: lipgloss.NewStyle().
			Foreground(m.theme.TextFg).
			Background(m.theme.BorderDim).
			Padding(0, 1),

		keyHintKeyStyle: lipgloss.NewStyle().
			Foreground(m.theme.AccentFg).
			Background(m.theme.Accent).
			Bold(true).
			Padding(0, 1),
		keyHintLabelStyle: lipgloss.NewStyle().Foreground(m.theme.Accent),

		paneIndicatorKeyStyle: lipgloss.NewStyle().
			Foreground(m.theme.AccentFg).
			Background(m.theme.Accent).
			Bold(true).
			Padding(0, 1),
		paneFilterTextStyle: lipgloss.NewStyle().Foreground(m.theme.WarnFg).Italic(true),
		paneZoomTextStyle:   lipgloss.NewStyle().Foreground(m.theme.Accent).Italic(true),
		paneMutedTextStyle:  lipgloss.NewStyle().Foreground(m.theme.MutedFg),
		paneBubbleFocusedStyle: lipgloss.NewStyle().
			Background(m.theme.Accent).
			Foreground(m.theme.AccentFg).
			Bold(true),
		paneBubbleDimStyle: lipgloss.NewStyle().
			Background(m.theme.BorderDim).
			Foreground(m.theme.TextFg),

		ciIconSuccessStyle: lipgloss.NewStyle().Foreground(m.theme.SuccessFg),
		ciIconFailureStyle: lipgloss.NewStyle().Foreground(m.theme.ErrorFg),
		ciIconPendingStyle: lipgloss.NewStyle().Foreground(m.theme.WarnFg),
		ciIconDefaultStyle: lipgloss.NewStyle().Foreground(m.theme.MutedFg),

		baseBoxStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(m.theme.BorderDim).
			Padding(0, 1),
	}
}
