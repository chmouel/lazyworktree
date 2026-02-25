package app

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/chmouel/lazyworktree/internal/app/state"
	"github.com/chmouel/lazyworktree/internal/models"
)

type annotationKeywordSpec struct {
	Canonical string
	Aliases   []string
	NerdIcon  string
	TextIcon  string
}

var annotationKeywordSpecs = []annotationKeywordSpec{
	{
		Canonical: "FIX",
		Aliases:   []string{"FIX", "FIXME", "BUG", "FIXIT", "ISSUE"},
		NerdIcon:  "",
		TextIcon:  "[!]",
	},
	{
		Canonical: "TODO",
		Aliases:   []string{"TODO"},
		NerdIcon:  "",
		TextIcon:  "[ ]",
	},
	{
		Canonical: "DONE",
		Aliases:   []string{"DONE"},
		NerdIcon:  "",
		TextIcon:  "[x]",
	},
	{
		Canonical: "TODO_CHECKBOX",
		Aliases:   []string{"TODO_CHECKBOX"},
		NerdIcon:  " TODO", // U+F0131, checkbox blank circle outline
		TextIcon:  "[ ]",
	},
	{
		Canonical: "DONE_CHECKBOX",
		Aliases:   []string{"DONE_CHECKBOX"},
		NerdIcon:  " DONE", // U+F0134, checkbox marked circle outline
		TextIcon:  "[x]",
	},
	{
		Canonical: "HACK",
		Aliases:   []string{"HACK"},
		NerdIcon:  "",
		TextIcon:  "[~]",
	},
	{
		Canonical: "WARN",
		Aliases:   []string{"WARN", "WARNING", "XXX"},
		NerdIcon:  "",
		TextIcon:  "[!]",
	},
	{
		Canonical: "PERF",
		Aliases:   []string{"PERF", "OPTIM", "PERFORMANCE", "OPTIMIZE"},
		NerdIcon:  "",
		TextIcon:  "[>]",
	},
	{
		Canonical: "NOTE",
		Aliases:   []string{"NOTE", "INFO"},
		NerdIcon:  "",
		TextIcon:  "[i]",
	},
	{
		Canonical: "TEST",
		Aliases:   []string{"TEST", "TESTING", "PASSED", "FAILED"},
		NerdIcon:  "⏲",
		TextIcon:  "[t]",
	},
}

var (
	annotationAliasMap     = buildAnnotationAliasMap()
	annotationKeywordRegex = regexp.MustCompile(buildAnnotationKeywordPattern())
	markdownInlineLinkRe   = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)
	markdownStrongRe       = regexp.MustCompile(`\*\*([^*]+)\*\*|__([^_]+)__`)
	markdownInlineCodeRe   = regexp.MustCompile("`([^`]+)`")
)

func buildAnnotationAliasMap() map[string]annotationKeywordSpec {
	aliases := make(map[string]annotationKeywordSpec, 32)
	for _, spec := range annotationKeywordSpecs {
		for _, alias := range spec.Aliases {
			aliases[alias] = spec
		}
	}
	return aliases
}

func buildAnnotationKeywordPattern() string {
	aliases := make([]string, 0, 32)
	seen := make(map[string]struct{}, 32)
	for _, spec := range annotationKeywordSpecs {
		for _, alias := range spec.Aliases {
			if _, ok := seen[alias]; ok {
				continue
			}
			seen[alias] = struct{}{}
			aliases = append(aliases, regexp.QuoteMeta(alias))
		}
	}
	sort.Slice(aliases, func(i, j int) bool {
		return len(aliases[i]) > len(aliases[j])
	})
	return `\b(` + strings.Join(aliases, "|") + `)\b(:?)`
}

func (m *Model) annotationKeywordIcon(spec annotationKeywordSpec) string {
	iconSet := strings.ToLower(strings.TrimSpace(m.config.IconSet))
	if iconSet == "nerd-font-v3" {
		return spec.NerdIcon
	}
	return spec.TextIcon
}

func (m *Model) annotationKeywordStyle(spec annotationKeywordSpec) lipgloss.Style {
	style := lipgloss.NewStyle().Bold(true)
	switch spec.Canonical {
	case "FIX":
		return style.Foreground(m.theme.ErrorFg)
	case "WARN", "HACK":
		return style.Foreground(m.theme.WarnFg)
	case "DONE":
		return style.Foreground(m.theme.SuccessFg)
	case "TODO", "TODO_CHECKBOX":
		return style.Foreground(m.theme.WarnFg)
	case "DONE_CHECKBOX":
		return style.Foreground(m.theme.SuccessFg)
	case "NOTE":
		return style.Foreground(m.theme.SuccessFg)
	case "PERF", "TEST":
		return style.Foreground(m.theme.Accent)
	default:
		return style.Foreground(m.theme.TextFg)
	}
}

func (m *Model) renderAnnotationKeywords(line string, valueStyle lipgloss.Style) string {
	matches := annotationKeywordRegex.FindAllStringSubmatchIndex(line, -1)
	if len(matches) == 0 {
		return valueStyle.Render(line)
	}

	var b strings.Builder
	last := 0
	var lastSpec *annotationKeywordSpec
	for _, idx := range matches {
		if len(idx) < 6 {
			continue
		}

		matchStart := idx[0]
		matchEnd := idx[1]
		kwStart := idx[2]
		kwEnd := idx[3]
		colonStart := idx[4]
		colonEnd := idx[5]

		if matchStart > last {
			if lastSpec != nil && lastSpec.Canonical == "DONE" {
				b.WriteString(lipgloss.NewStyle().Foreground(m.theme.MutedFg).Strikethrough(true).Render(line[last:matchStart]))
			} else {
				b.WriteString(valueStyle.Render(line[last:matchStart]))
			}
		}

		alias := line[kwStart:kwEnd]
		spec, ok := annotationAliasMap[alias]
		if !ok {
			b.WriteString(valueStyle.Render(line[matchStart:matchEnd]))
			last = matchEnd
			continue
		}

		token := iconWithSpace(m.annotationKeywordIcon(spec)) + alias
		if colonStart >= 0 && colonEnd > colonStart {
			token += ":"
		}
		b.WriteString(m.annotationKeywordStyle(spec).Render(token))
		lastSpec = &spec
		last = matchEnd
	}

	if last < len(line) {
		if lastSpec != nil && lastSpec.Canonical == "DONE" {
			b.WriteString(lipgloss.NewStyle().Foreground(m.theme.MutedFg).Strikethrough(true).Render(line[last:]))
		} else {
			b.WriteString(valueStyle.Render(line[last:]))
		}
	}
	return b.String()
}

func parseMarkdownHeading(line string) (string, bool) {
	level := 0
	for level < len(line) && line[level] == '#' {
		level++
	}
	if level == 0 || level > 6 || level >= len(line) || line[level] != ' ' {
		return "", false
	}

	return strings.TrimSpace(line[level+1:]), true
}

func parseMarkdownUnorderedList(line string) (int, string, bool) {
	trimmed := strings.TrimLeft(line, " \t")
	if len(trimmed) < 3 {
		return 0, "", false
	}

	marker := trimmed[0]
	if marker != '-' && marker != '*' && marker != '+' {
		return 0, "", false
	}
	if trimmed[1] != ' ' {
		return 0, "", false
	}

	leading := len(line) - len(trimmed)
	return leading / 2, strings.TrimSpace(trimmed[2:]), true
}

// parseMarkdownCheckbox parses markdown checkbox lines like "- [ ] task" or "- [x] task".
// Returns indent level (based on leading spaces / 2), checked status, task text, and ok.
func parseMarkdownCheckbox(line string) (int, bool, string, bool) {
	trimmed := strings.TrimLeft(line, " \t")
	if len(trimmed) < 6 { // Minimum: "- [ ] "
		return 0, false, "", false
	}

	// Check for list marker (-, *, +)
	marker := trimmed[0]
	if marker != '-' && marker != '*' && marker != '+' {
		return 0, false, "", false
	}
	if trimmed[1] != ' ' {
		return 0, false, "", false
	}

	// Check for checkbox pattern "[ ]" or "[x]" / "[X]"
	if len(trimmed) < 6 || trimmed[2] != '[' || trimmed[4] != ']' {
		return 0, false, "", false
	}

	checkChar := trimmed[3]
	var checked bool
	switch checkChar {
	case ' ':
		checked = false
	case 'x', 'X':
		checked = true
	default:
		return 0, false, "", false
	}

	if trimmed[5] != ' ' {
		return 0, false, "", false
	}

	leading := len(line) - len(trimmed)
	text := strings.TrimSpace(trimmed[6:])
	if text == "" {
		text = "(empty task)"
	}

	return leading / 2, checked, text, true
}

func parseMarkdownOrderedList(line string) (int, string, string, bool) {
	trimmed := strings.TrimLeft(line, " \t")
	if len(trimmed) < 4 {
		return 0, "", "", false
	}

	i := 0
	for i < len(trimmed) && trimmed[i] >= '0' && trimmed[i] <= '9' {
		i++
	}
	if i == 0 || i+1 >= len(trimmed) {
		return 0, "", "", false
	}
	if (trimmed[i] != '.' && trimmed[i] != ')') || trimmed[i+1] != ' ' {
		return 0, "", "", false
	}

	leading := len(line) - len(trimmed)
	return leading / 2, trimmed[:i+1], strings.TrimSpace(trimmed[i+2:]), true
}

func isMarkdownHorizontalRule(line string) bool {
	trimmed := strings.TrimSpace(line)
	if len(trimmed) < 3 {
		return false
	}

	var marker byte
	count := 0
	for i := 0; i < len(trimmed); i++ {
		ch := trimmed[i]
		if ch == ' ' {
			continue
		}
		if marker == 0 {
			if ch != '-' && ch != '*' && ch != '_' {
				return false
			}
			marker = ch
		}
		if ch != marker {
			return false
		}
		count++
	}

	return count >= 3
}

func (m *Model) renderInlineMarkdown(line string) string {
	codeStyle := lipgloss.NewStyle().Foreground(m.theme.Cyan)
	strongStyle := lipgloss.NewStyle().Bold(true).Foreground(m.theme.TextFg)

	line = markdownInlineCodeRe.ReplaceAllStringFunc(line, func(match string) string {
		parts := markdownInlineCodeRe.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		return codeStyle.Render(parts[1])
	})

	line = markdownStrongRe.ReplaceAllStringFunc(line, func(match string) string {
		parts := markdownStrongRe.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}

		content := parts[1]
		if content == "" {
			content = parts[2]
		}
		return strongStyle.Render(content)
	})

	return markdownInlineLinkRe.ReplaceAllStringFunc(line, func(match string) string {
		parts := markdownInlineLinkRe.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}

		label := strings.TrimSpace(parts[1])
		url := strings.TrimSpace(parts[2])
		if label == "" {
			label = url
		}
		return osc8Hyperlink(label, url)
	})
}

func (m *Model) renderMarkdownNoteLines(noteText string, valueStyle lipgloss.Style) []string {
	normalized := strings.ReplaceAll(noteText, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	rendered := make([]string, 0, len(lines))

	headingStyle := valueStyle.Bold(true).Foreground(m.theme.Accent)
	quoteStyle := valueStyle.Foreground(m.theme.MutedFg)
	codeStyle := valueStyle.Foreground(m.theme.MutedFg)
	ruleStyle := valueStyle.Foreground(m.theme.MutedFg)

	inCodeFence := false
	codeFenceMarker := ""

	for _, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)

		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			marker := trimmed[:3]
			if !inCodeFence {
				inCodeFence = true
				codeFenceMarker = marker
			} else if marker == codeFenceMarker {
				inCodeFence = false
				codeFenceMarker = ""
			}
			continue
		}

		if trimmed == "" {
			rendered = append(rendered, "  ")
			continue
		}

		if inCodeFence {
			codeLine := strings.TrimLeft(rawLine, " \t")
			rendered = append(rendered, "  "+codeStyle.Render(codeLine))
			continue
		}

		if heading, ok := parseMarkdownHeading(trimmed); ok {
			line := m.renderAnnotationKeywords(heading, headingStyle)
			rendered = append(rendered, "  "+m.renderInlineMarkdown(line))
			continue
		}

		if isMarkdownHorizontalRule(trimmed) {
			rendered = append(rendered, "  "+ruleStyle.Render(strings.Repeat("-", 20)))
			continue
		}

		if strings.HasPrefix(trimmed, ">") {
			quoted := strings.TrimSpace(strings.TrimPrefix(trimmed, ">"))
			line := m.renderAnnotationKeywords("| "+quoted, quoteStyle)
			rendered = append(rendered, "  "+m.renderInlineMarkdown(line))
			continue
		}

		// Try checkbox first (more specific than regular list)
		if indent, checked, text, ok := parseMarkdownCheckbox(rawLine); ok {
			// Select spec based on checked status
			var spec annotationKeywordSpec
			if checked {
				spec = annotationKeywordSpecs[4] // DONE_CHECKBOX
			} else {
				spec = annotationKeywordSpecs[3] // TODO_CHECKBOX
			}

			icon := m.annotationKeywordIcon(spec)
			style := m.annotationKeywordStyle(spec)
			indentStr := strings.Repeat("  ", indent)

			iconPart := style.Render(iconWithSpace(icon))
			var textPart string
			if checked {
				textPart = lipgloss.NewStyle().Foreground(m.theme.MutedFg).Strikethrough(true).Render(text)
			} else {
				textPart = valueStyle.Render(text)
			}
			styledLine := iconPart + textPart

			rendered = append(rendered, "  "+indentStr+styledLine)
			continue
		}

		// Fall back to regular unordered list
		if indent, item, ok := parseMarkdownUnorderedList(rawLine); ok {
			prefix := strings.Repeat("  ", indent) + "- "
			line := m.renderAnnotationKeywords(prefix+item, valueStyle)
			rendered = append(rendered, "  "+m.renderInlineMarkdown(line))
			continue
		}

		if indent, marker, item, ok := parseMarkdownOrderedList(rawLine); ok {
			prefix := strings.Repeat("  ", indent) + marker + " "
			line := m.renderAnnotationKeywords(prefix+item, valueStyle)
			rendered = append(rendered, "  "+m.renderInlineMarkdown(line))
			continue
		}

		line := m.renderAnnotationKeywords(strings.TrimLeft(rawLine, " \t"), valueStyle)
		rendered = append(rendered, "  "+m.renderInlineMarkdown(line))
	}

	if len(rendered) == 0 {
		return []string{"  "}
	}

	return rendered
}

// renderBody renders the main body area with panes.
func (m *Model) renderBody(layout layoutDims) string {
	// Handle zoom mode: only render the zoomed pane (layout agnostic)
	if m.state.view.ZoomedPane >= 0 {
		switch m.state.view.ZoomedPane {
		case 0:
			return m.renderZoomedLeftPane(layout)
		case 1:
			return m.renderZoomedRightTopPane(layout)
		case 2:
			return m.renderZoomedRightMiddlePane(layout)
		case 3:
			return m.renderZoomedRightBottomPane(layout)
		}
	}

	if layout.layoutMode == state.LayoutTop {
		return m.renderTopLayoutBody(layout)
	}

	left := m.renderLeftPane(layout)
	right := m.renderRightPane(layout)
	gap := lipgloss.NewStyle().
		Width(layout.gapX).
		Render(strings.Repeat(" ", layout.gapX))
	return lipgloss.JoinHorizontal(lipgloss.Top, left, gap, right)
}

// renderLeftPane renders the left pane (worktree table).
func (m *Model) renderLeftPane(layout layoutDims) string {
	focused := m.state.view.FocusedPane == 0
	return m.renderPaneBlock(1, "Worktrees", focused, layout.leftWidth, layout.bodyHeight, m.state.ui.worktreeTable.View())
}

// renderRightPane renders the right pane container (status + git status + commit).
func (m *Model) renderRightPane(layout layoutDims) string {
	top := m.renderRightTopPane(layout)
	middle := m.renderRightMiddlePane(layout)
	bottom := m.renderRightBottomPane(layout)
	gap := strings.Repeat("\n", layout.gapY)
	return lipgloss.JoinVertical(lipgloss.Left, top, gap, middle, gap, bottom)
}

// renderRightTopPane renders the right top pane (info box only).
func (m *Model) renderRightTopPane(layout layoutDims) string {
	focused := m.state.view.FocusedPane == 1

	infoBox := m.renderInfoBox(layout.rightInnerWidth, layout.rightTopInnerHeight)
	return m.renderPaneBlock(2, "Info", focused, layout.rightWidth, layout.rightTopHeight, infoBox)
}

// renderInfoBox renders the Info content using a viewport for scrolling.
func (m *Model) renderInfoBox(width, height int) string {
	content := m.infoContent
	if content == "" {
		content = "No data available."
	}

	titleStyle := lipgloss.NewStyle().Foreground(m.theme.MutedFg).Bold(true)
	innerBoxStyle := m.baseInnerBoxStyle()

	// Title takes 1 line
	vpWidth := maxInt(1, width-innerBoxStyle.GetHorizontalFrameSize())
	vpHeight := maxInt(1, height-innerBoxStyle.GetVerticalFrameSize()-1)

	m.state.ui.infoViewport.SetWidth(vpWidth)
	m.state.ui.infoViewport.SetHeight(vpHeight)
	m.state.ui.infoViewport.SetContent(content)

	boxContent := lipgloss.JoinVertical(lipgloss.Left, titleStyle.Render("Info"), m.state.ui.infoViewport.View())

	return innerBoxStyle.
		Width(width).
		Height(height).
		Render(boxContent)
}

// renderRightMiddlePane renders the right middle pane (git status file tree).
func (m *Model) renderRightMiddlePane(layout layoutDims) string {
	focused := m.state.view.FocusedPane == 2

	innerBoxStyle := m.baseInnerBoxStyle()
	statusViewportWidth := maxInt(1, layout.rightInnerWidth-innerBoxStyle.GetHorizontalFrameSize())
	statusViewportHeight := maxInt(1, layout.rightMiddleInnerHeight-innerBoxStyle.GetVerticalFrameSize())
	m.state.ui.statusViewport.SetWidth(statusViewportWidth)
	m.state.ui.statusViewport.SetHeight(statusViewportHeight)
	m.state.ui.statusViewport.SetContent(m.statusContent)
	statusBox := innerBoxStyle.
		Width(layout.rightInnerWidth).
		Height(layout.rightMiddleInnerHeight).
		Render(m.state.ui.statusViewport.View())

	return m.renderPaneBlock(3, "Git Status", focused, layout.rightWidth, layout.rightMiddleHeight, statusBox)
}

// renderRightBottomPane renders the right bottom pane (commit log table).
func (m *Model) renderRightBottomPane(layout layoutDims) string {
	focused := m.state.view.FocusedPane == 3
	return m.renderPaneBlock(4, "Commit", focused, layout.rightWidth, layout.rightBottomHeight, m.state.ui.logTable.View())
}

// renderTopLayoutBody renders the body for the top layout mode.
func (m *Model) renderTopLayoutBody(layout layoutDims) string {
	top := m.renderTopPane(layout)
	bottom := m.renderBottomPane(layout)
	gap := strings.Repeat("\n", layout.gapY)
	return lipgloss.JoinVertical(lipgloss.Left, top, gap, bottom)
}

// renderTopPane renders the full-width worktree pane at the top.
func (m *Model) renderTopPane(layout layoutDims) string {
	focused := m.state.view.FocusedPane == 0
	return m.renderPaneBlock(1, "Worktrees", focused, layout.width, layout.topHeight, m.state.ui.worktreeTable.View())
}

// renderBottomPane renders the bottom pane container (status + git status + commit side by side).
func (m *Model) renderBottomPane(layout layoutDims) string {
	left := m.renderBottomLeftPane(layout)
	middle := m.renderBottomMiddlePane(layout)
	right := m.renderBottomRightPane(layout)
	gap := lipgloss.NewStyle().
		Width(layout.gapX).
		Render(strings.Repeat(" ", layout.gapX))
	return lipgloss.JoinHorizontal(lipgloss.Top, left, gap, middle, gap, right)
}

// renderBottomLeftPane renders the status (info) pane in the bottom left of the top layout.
func (m *Model) renderBottomLeftPane(layout layoutDims) string {
	focused := m.state.view.FocusedPane == 1

	infoBox := m.renderInfoBox(layout.bottomLeftInnerWidth, layout.bottomLeftInnerHeight)
	return m.renderPaneBlock(2, "Info", focused, layout.bottomLeftWidth, layout.bottomHeight, infoBox)
}

// renderBottomMiddlePane renders the git status pane in the bottom middle of the top layout.
func (m *Model) renderBottomMiddlePane(layout layoutDims) string {
	focused := m.state.view.FocusedPane == 2

	innerBoxStyle := m.baseInnerBoxStyle()
	statusViewportWidth := maxInt(1, layout.bottomMiddleInnerWidth-innerBoxStyle.GetHorizontalFrameSize())
	statusViewportHeight := maxInt(1, layout.bottomMiddleInnerHeight-innerBoxStyle.GetVerticalFrameSize())
	m.state.ui.statusViewport.SetWidth(statusViewportWidth)
	m.state.ui.statusViewport.SetHeight(statusViewportHeight)
	m.state.ui.statusViewport.SetContent(m.statusContent)
	statusBox := innerBoxStyle.
		Width(layout.bottomMiddleInnerWidth).
		Height(layout.bottomMiddleInnerHeight).
		Render(m.state.ui.statusViewport.View())

	return m.renderPaneBlock(3, "Git Status", focused, layout.bottomMiddleWidth, layout.bottomHeight, statusBox)
}

// renderBottomRightPane renders the commit pane in the bottom right of the top layout.
func (m *Model) renderBottomRightPane(layout layoutDims) string {
	focused := m.state.view.FocusedPane == 3
	return m.renderPaneBlock(4, "Commit", focused, layout.bottomRightWidth, layout.bottomHeight, m.state.ui.logTable.View())
}

// renderZoomedLeftPane renders the zoomed left pane.
func (m *Model) renderZoomedLeftPane(layout layoutDims) string {
	return m.renderPaneBlock(1, "Worktrees", true, layout.leftWidth, layout.bodyHeight, m.state.ui.worktreeTable.View())
}

// renderZoomedRightTopPane renders the zoomed right top pane (info only).
func (m *Model) renderZoomedRightTopPane(layout layoutDims) string {
	infoBox := m.renderInfoBox(layout.rightInnerWidth, layout.rightTopInnerHeight)
	return m.renderPaneBlock(2, "Status", true, layout.rightWidth, layout.bodyHeight, infoBox)
}

// renderZoomedRightMiddlePane renders the zoomed right middle pane (git status file tree).
func (m *Model) renderZoomedRightMiddlePane(layout layoutDims) string {
	innerBoxStyle := m.baseInnerBoxStyle()
	statusViewportWidth := maxInt(1, layout.rightInnerWidth-innerBoxStyle.GetHorizontalFrameSize())
	statusViewportHeight := maxInt(1, layout.rightMiddleInnerHeight-innerBoxStyle.GetVerticalFrameSize())
	m.state.ui.statusViewport.SetWidth(statusViewportWidth)
	m.state.ui.statusViewport.SetHeight(statusViewportHeight)
	m.state.ui.statusViewport.SetContent(m.statusContent)
	statusBox := innerBoxStyle.
		Width(layout.rightInnerWidth).
		Height(layout.rightMiddleInnerHeight).
		Render(m.state.ui.statusViewport.View())

	return m.renderPaneBlock(3, "Git Status", true, layout.rightWidth, layout.bodyHeight, statusBox)
}

// renderZoomedRightBottomPane renders the zoomed right bottom pane (commit log).
func (m *Model) renderZoomedRightBottomPane(layout layoutDims) string {
	return m.renderPaneBlock(4, "Commit", true, layout.rightWidth, layout.bodyHeight, m.state.ui.logTable.View())
}

// buildInfoContent builds the info content string for a worktree.
func (m *Model) buildInfoContent(wt *models.WorktreeInfo) string {
	if wt == nil {
		return errNoWorktreeSelected
	}
	// Consider any worktree on the same branch as the main worktree as a main-branch view.
	isMainBranch := wt.IsMain
	if !isMainBranch {
		for _, candidate := range m.state.data.worktrees {
			if candidate != nil && candidate.IsMain && candidate.Branch != "" && wt.Branch == candidate.Branch {
				isMainBranch = true
				break
			}
		}
	}

	labelStyle := lipgloss.NewStyle().Foreground(m.theme.Cyan).Bold(true)
	valueStyle := lipgloss.NewStyle().Foreground(m.theme.TextFg)
	sectionStyle := lipgloss.NewStyle().Foreground(m.theme.Accent).Bold(true)
	keyWidth := lipgloss.Width("Last Accessed:")
	keyStyle := labelStyle.Width(keyWidth)

	addField := func(lines []string, key, value string) []string {
		return append(lines, fmt.Sprintf("%s %s", keyStyle.Render(key), value))
	}

	infoLines := make([]string, 0, 32)
	infoLines = addField(infoLines, "Path:", valueStyle.Render(wt.Path))
	infoLines = addField(infoLines, "Branch:", valueStyle.Render(wt.Branch))

	if wt.LastSwitchedTS > 0 {
		accessTime := time.Unix(wt.LastSwitchedTS, 0)
		relTime := formatRelativeTime(accessTime)
		infoLines = addField(infoLines, "Last Accessed:", valueStyle.Render(relTime))
	}
	if wt.Ahead > 0 || wt.Behind > 0 {
		aheadStyle := lipgloss.NewStyle().Foreground(m.theme.Cyan)
		behindStyle := lipgloss.NewStyle().Foreground(m.theme.ErrorFg)
		parts := make([]string, 0, 2)
		if wt.Ahead > 0 {
			parts = append(parts, aheadStyle.Render(fmt.Sprintf("%s%d", aheadIndicator(m.config.IconsEnabled()), wt.Ahead)))
		}
		if wt.Behind > 0 {
			parts = append(parts, behindStyle.Render(fmt.Sprintf("%s%d", behindIndicator(m.config.IconsEnabled()), wt.Behind)))
		}
		infoLines = addField(infoLines, "Divergence:", strings.Join(parts, " "))
	}
	hidePRDetails := wt.PR != nil && wt.IsMain && (wt.PR.State == prStateMerged || wt.PR.State == prStateClosed)
	if wt.PR != nil && !hidePRDetails && !m.config.DisablePR {
		authorText := wt.PR.Author
		renderStyle := lipgloss.NewStyle().Foreground(m.theme.TextFg).Bold(true)
		if wt.PR.Author != "" {
			if wt.PR.AuthorName != "" {
				authorText = fmt.Sprintf("@%s", wt.PR.Author)
			} else {
				authorText = wt.PR.Author
			}
			if wt.PR.AuthorIsBot {
				authorText = iconPrefix(UIIconBot, m.config.IconsEnabled()) + authorText
			}
			authorText = renderStyle.Render(authorText)
		}
		prLabelStyle := lipgloss.NewStyle().Foreground(m.theme.Accent).Bold(true) // Accent for PR prominence
		prNumber := fmt.Sprintf("#%d", wt.PR.Number)
		prNumber = renderStyle.Render(prNumber)
		prPrefix := fmt.Sprintf("PR %s by %s %s", prNumber, authorText, m.renderPRStatePill(wt.PR.State))
		if m.config.IconsEnabled() {
			prPrefix = iconWithSpace(getIconPR()) + prPrefix
		}
		infoLines = append(infoLines, "")
		infoLines = append(infoLines, prLabelStyle.Render(prPrefix))
		infoLines = append(infoLines, fmt.Sprintf("  %s ", wt.PR.Title))
		// Author line with bot indicator if applicable
		// // URL styled with cyan for consistency
		infoLines = append(infoLines, fmt.Sprintf("  %s", wt.PR.URL))
	} else if wt.PR == nil && !m.config.DisablePR && wt.HasUpstream {
		// Show PR status/error when PR is nil
		grayStyle := lipgloss.NewStyle().Foreground(m.theme.MutedFg)
		errorStyle := lipgloss.NewStyle().Foreground(m.theme.ErrorFg)
		prLabelStyle := lipgloss.NewStyle().Foreground(m.theme.Accent).Bold(true)
		prPrefix := "PR:"
		if m.config.IconsEnabled() {
			prPrefix = iconWithSpace(getIconPR()) + prPrefix
		}

		infoLines = append(infoLines, "")
		infoLines = append(infoLines, prLabelStyle.Render(prPrefix))

		switch wt.PRFetchStatus {
		case models.PRFetchStatusLoaded:
			// This shouldn't happen (PR is nil but status is loaded) - show debug info
			infoLines = append(infoLines, errorStyle.Render("  PR Status: Loaded but nil (bug)"))

		case models.PRFetchStatusError:
			infoLines = append(infoLines, valueStyle.Bold(true).Render("  PR Status:"))
			infoLines = append(infoLines, errorStyle.Render("    Fetch failed"))

			// Provide helpful error messages based on error content
			switch {
			case strings.Contains(wt.PRFetchError, "not found") || strings.Contains(wt.PRFetchError, "PATH"):
				infoLines = append(infoLines, grayStyle.Render("    gh/glab CLI not found"))
				infoLines = append(infoLines, grayStyle.Render("    Install from https://cli.github.com"))
			case strings.Contains(wt.PRFetchError, "auth") || strings.Contains(wt.PRFetchError, "401"):
				infoLines = append(infoLines, grayStyle.Render("    Authentication failed"))
				infoLines = append(infoLines, grayStyle.Render("    Run 'gh auth login' or 'glab auth login'"))
			case wt.PRFetchError != "":
				infoLines = append(infoLines, grayStyle.Render(fmt.Sprintf("    %s", wt.PRFetchError)))
			}

		case models.PRFetchStatusNoPR:
			if m.prDataLoaded {
				// Fetch was attempted, no error, no PR found - this is expected
				infoLines = append(infoLines, grayStyle.Render("  No PR for this branch"))
			}

		case models.PRFetchStatusFetching:
			infoLines = append(infoLines, grayStyle.Render("  Fetching PR data..."))

		default:
			// Not fetched yet
			if !m.prDataLoaded {
				if isMainBranch {
					infoLines = append(infoLines, grayStyle.Render("  Main branch usually has no PR"))
				} else {
					infoLines = append(infoLines, grayStyle.Render("  Press 'r' to refresh and fetch PR data"))
				}
			}
		}
	}

	// CI status from cache (shown for all branches with cached checks, not just PRs)
	if !m.config.DisablePR {
		if cachedChecks, _, ok := m.cache.ciCache.Get(wt.Branch); ok && len(cachedChecks) > 0 {
			infoLines = append(infoLines, "") // blank line before CI

			// Summary pill next to CI Checks heading
			aggregate := aggregateCIConclusion(cachedChecks)
			summaryPill := m.renderCIStatusPill(aggregate)
			infoLines = append(infoLines, sectionStyle.Render("CI Checks:")+" "+summaryPill)

			selectedStyle := lipgloss.NewStyle().
				Foreground(m.theme.AccentFg).
				Background(m.theme.Accent).
				Bold(true)

			checks := sortCIChecks(cachedChecks)
			for i, check := range checks {
				isSelected := m.state.view.FocusedPane == 1 && m.ciCheckIndex >= 0 && i == m.ciCheckIndex

				symbol := getCIStatusIcon(check.Conclusion, false, m.config.IconsEnabled())
				var line string
				if isSelected {
					line = fmt.Sprintf("  %s %s", symbol, check.Name)
					line = selectedStyle.Render(line)
				} else {
					iconStyle := m.ciIconStyle(check.Conclusion)
					line = fmt.Sprintf("  %s %s", iconStyle.Render(symbol), check.Name)
				}
				infoLines = append(infoLines, line)
			}
		}
	}

	if note, ok := m.getWorktreeNote(wt.Path); ok {
		infoLines = append(infoLines, "")
		infoLines = append(infoLines, sectionStyle.Render("Notes:"))
		infoLines = append(infoLines, m.renderMarkdownNoteLines(note.Note, valueStyle)...)
	}

	return strings.Join(infoLines, "\n")
}

// aggregateCIConclusion computes the overall CI status from a slice of checks.
// Priority: failure > pending > success > skipped/cancelled.
func aggregateCIConclusion(checks []*models.CICheck) string {
	hasSuccess := false
	hasPending := false
	for _, c := range checks {
		switch c.Conclusion {
		case "failure":
			return "failure"
		case "pending", "":
			hasPending = true
		case "success":
			hasSuccess = true
		}
	}
	if hasPending {
		return "pending"
	}
	if hasSuccess {
		return "success"
	}
	return "skipped"
}

// renderStatusFiles renders the status file list with current selection highlighted.
func (m *Model) renderStatusFiles() string {
	if len(m.state.services.statusTree.TreeFlat) == 0 {
		if len(m.state.data.statusFilesAll) == 0 {
			return lipgloss.NewStyle().Foreground(m.theme.SuccessFg).Render("Clean working tree")
		}
		if strings.TrimSpace(m.state.services.filter.StatusFilterQuery) != "" {
			return lipgloss.NewStyle().Foreground(m.theme.MutedFg).Render(
				fmt.Sprintf("No files match %q", strings.TrimSpace(m.state.services.filter.StatusFilterQuery)),
			)
		}
		return lipgloss.NewStyle().Foreground(m.theme.MutedFg).Render("No files to display")
	}

	modifiedStyle := lipgloss.NewStyle().Foreground(m.theme.WarnFg)
	addedStyle := lipgloss.NewStyle().Foreground(m.theme.SuccessFg)
	deletedStyle := lipgloss.NewStyle().Foreground(m.theme.ErrorFg)
	untrackedStyle := lipgloss.NewStyle().Foreground(m.theme.WarnFg)
	stagedStyle := lipgloss.NewStyle().Foreground(m.theme.Cyan)
	dirStyle := lipgloss.NewStyle().Foreground(m.theme.MutedFg)
	selectedStyle := lipgloss.NewStyle().
		Foreground(m.theme.AccentFg).
		Background(m.theme.Accent).
		Bold(true)

	viewportWidth := m.state.ui.statusViewport.Width()
	showIcons := m.config.IconsEnabled()

	lines := make([]string, 0, len(m.state.services.statusTree.TreeFlat))
	for i, node := range m.state.services.statusTree.TreeFlat {
		indent := strings.Repeat("  ", node.Depth)

		var lineContent string
		var fileIcon string
		if node.IsDir() {
			// Directory line: "  ▼ dirname" or "  ▶ dirname"
			expandIcon := disclosureIndicator(m.state.services.statusTree.CollapsedDirs[node.Path], showIcons)
			dirIcon := ""
			if showIcons {
				dirIcon = iconWithSpace(deviconForName(node.Name(), true))
			}
			lineContent = fmt.Sprintf("%s%s %s%s", indent, expandIcon, dirIcon, node.Path)
		} else {
			// File line: "    M  filename" or "    S  filename" for staged
			status := node.File.Status
			displayStatus := formatStatusDisplay(status)
			if showIcons {
				fileIcon = iconWithSpace(deviconForName(node.Name(), false))
			}
			lineContent = fmt.Sprintf("%s  %s %s%s", indent, displayStatus, fileIcon, node.Name())
		}

		// Apply styling based on selection and node type
		switch {
		case m.state.view.FocusedPane == 2 && m.ciCheckIndex < 0 && i == m.state.services.statusTree.Index:
			if viewportWidth > 0 && len(lineContent) < viewportWidth {
				lineContent += strings.Repeat(" ", viewportWidth-len(lineContent))
			}
			lines = append(lines, selectedStyle.Render(lineContent))
		case node.IsDir():
			lines = append(lines, dirStyle.Render(lineContent))
		default:
			// Color based on file status - apply different colors for staged vs unstaged
			status := node.File.Status
			if len(status) < 2 {
				lines = append(lines, lineContent)
				continue
			}

			// Special case for untracked files
			if status == " ?" {
				displayStatus := formatStatusDisplay(status)
				formatted := fmt.Sprintf("%s  %s %s%s", indent, untrackedStyle.Render(displayStatus), fileIcon, node.Name())
				lines = append(lines, formatted)
				continue
			}

			x := status[0] // Staged status
			y := status[1] // Unstaged status
			displayStatus := formatStatusDisplay(status)

			// Render each character with appropriate color based on position
			var statusRendered strings.Builder
			for i, char := range displayStatus {
				if char == ' ' {
					statusRendered.WriteString(" ")
					continue
				}

				var style lipgloss.Style
				if i == 0 {
					// First character is staged (X position)
					switch x {
					case 'M':
						style = stagedStyle // Cyan for staged modifications
					case 'A':
						style = addedStyle // Green for staged additions
					case 'D':
						style = deletedStyle // Red for staged deletions
					case 'R', 'C':
						style = stagedStyle // Cyan for staged renames/copies
					default:
						style = lipgloss.NewStyle()
					}
				} else {
					// Second character is unstaged (Y position)
					switch y {
					case 'M':
						style = modifiedStyle // Orange for unstaged modifications
					case 'A':
						style = addedStyle // Green for unstaged additions
					case 'D':
						style = deletedStyle // Red for unstaged deletions
					case 'R', 'C':
						style = modifiedStyle // Orange for unstaged renames/copies
					default:
						style = lipgloss.NewStyle()
					}
				}
				statusRendered.WriteString(style.Render(string(char)))
			}
			formatted := fmt.Sprintf("%s  %s %s%s", indent, statusRendered.String(), fileIcon, node.Name())
			lines = append(lines, formatted)
		}
	}
	return strings.Join(lines, "\n")
}
