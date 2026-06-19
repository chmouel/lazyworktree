package app

import (
	"regexp"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
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
		NerdIcon:  "",
		TextIcon:  "[ ]",
	},
	{
		Canonical: "DONE",
		Aliases:   []string{"DONE"},
		NerdIcon:  "",
		TextIcon:  "[x]",
	},
	{
		Canonical: "TODO_CHECKBOX",
		Aliases:   []string{"TODO_CHECKBOX"},
		NerdIcon:  " TODO", // U+F0131, checkbox blank circle outline
		TextIcon:  "[ ]",
	},
	{
		Canonical: "DONE_CHECKBOX",
		Aliases:   []string{"DONE_CHECKBOX"},
		NerdIcon:  " DONE", // U+F0134, checkbox marked circle outline
		TextIcon:  "[x]",
	},
	{
		Canonical: "HACK",
		Aliases:   []string{"HACK"},
		NerdIcon:  "",
		TextIcon:  "[~]",
	},
	{
		Canonical: "WARN",
		Aliases:   []string{"WARN", "WARNING", "XXX"},
		NerdIcon:  "",
		TextIcon:  "[!]",
	},
	{
		Canonical: "PERF",
		Aliases:   []string{"PERF", "OPTIM", "PERFORMANCE", "OPTIMIZE"},
		NerdIcon:  "",
		TextIcon:  "[>]",
	},
	{
		Canonical: "NOTE",
		Aliases:   []string{"NOTE", "INFO"},
		NerdIcon:  "",
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
	switch spec.Canonical {
	case "FIX":
		return m.renderStyles.annotFixStyle
	case "WARN", "HACK":
		return m.renderStyles.annotWarnStyle
	case "DONE":
		return m.renderStyles.annotDoneStyle
	case "TODO", "TODO_CHECKBOX":
		return m.renderStyles.annotTodoStyle
	case "DONE_CHECKBOX":
		return m.renderStyles.annotDoneStyle
	case "NOTE":
		return m.renderStyles.annotNoteStyle
	case "PERF", "TEST":
		return m.renderStyles.annotPerfStyle
	default:
		return m.renderStyles.annotDefaultStyle
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
				b.WriteString(m.renderStyles.annotStrikeStyle.Render(line[last:matchStart]))
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
			b.WriteString(m.renderStyles.annotStrikeStyle.Render(line[last:]))
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
	codeStyle := m.renderStyles.mdCodeStyle
	strongStyle := m.renderStyles.mdStrongStyle

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
				textPart = m.renderStyles.annotStrikeStyle.Render(text)
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
