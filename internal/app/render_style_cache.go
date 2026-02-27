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

	footerStyle    lipgloss.Style
	footerSepStyle lipgloss.Style

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

	// Annotation keyword styles
	annotFixStyle     lipgloss.Style
	annotWarnStyle    lipgloss.Style
	annotDoneStyle    lipgloss.Style
	annotTodoStyle    lipgloss.Style
	annotNoteStyle    lipgloss.Style
	annotPerfStyle    lipgloss.Style
	annotDefaultStyle lipgloss.Style
	annotStrikeStyle  lipgloss.Style

	// Markdown inline styles
	mdCodeStyle   lipgloss.Style
	mdStrongStyle lipgloss.Style

	// Info pane section divider
	infoDividerStyle lipgloss.Style

	// Powerline edge styles for pane title bubbles
	paneEdgeFocusedStyle lipgloss.Style
	paneEdgeDimStyle     lipgloss.Style

	// Overlay popup styles
	overlayLeftStyle lipgloss.Style
	overlayLineStyle lipgloss.Style

	unpushedCommitStyle lipgloss.Style
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
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(m.theme.BorderDim),

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
		footerSepStyle: lipgloss.NewStyle().
			Foreground(m.theme.MutedFg).
			Background(m.theme.BorderDim),

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

		annotFixStyle:     lipgloss.NewStyle().Bold(true).Foreground(m.theme.ErrorFg),
		annotWarnStyle:    lipgloss.NewStyle().Bold(true).Foreground(m.theme.WarnFg),
		annotDoneStyle:    lipgloss.NewStyle().Bold(true).Foreground(m.theme.SuccessFg),
		annotTodoStyle:    lipgloss.NewStyle().Bold(true).Foreground(m.theme.WarnFg),
		annotNoteStyle:    lipgloss.NewStyle().Bold(true).Foreground(m.theme.SuccessFg),
		annotPerfStyle:    lipgloss.NewStyle().Bold(true).Foreground(m.theme.Accent),
		annotDefaultStyle: lipgloss.NewStyle().Bold(true).Foreground(m.theme.TextFg),
		annotStrikeStyle:  lipgloss.NewStyle().Foreground(m.theme.MutedFg).Strikethrough(true),

		mdCodeStyle:   lipgloss.NewStyle().Foreground(m.theme.Cyan),
		mdStrongStyle: lipgloss.NewStyle().Bold(true).Foreground(m.theme.TextFg),

		infoDividerStyle: lipgloss.NewStyle().Foreground(m.theme.BorderDim),

		baseBoxStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(m.theme.BorderDim).
			Padding(0, 1),

		paneEdgeFocusedStyle: lipgloss.NewStyle().Foreground(m.theme.Accent),
		paneEdgeDimStyle:     lipgloss.NewStyle().Foreground(m.theme.BorderDim),

		overlayLeftStyle: lipgloss.NewStyle(),
		overlayLineStyle: lipgloss.NewStyle(),

		unpushedCommitStyle: lipgloss.NewStyle().Foreground(m.theme.WarnFg),
	}
}
