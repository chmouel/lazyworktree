package app

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/stretchr/testify/assert"
)

func newModelForMarkdownTest(t *testing.T) *Model {
	t.Helper()
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	return NewModel(cfg, "")
}

func plainStyle() lipgloss.Style {
	return lipgloss.NewStyle()
}

func TestRenderMarkdownNoteLines_EmptyInput(t *testing.T) {
	t.Parallel()
	m := newModelForMarkdownTest(t)

	lines := m.renderMarkdownNoteLines("", plainStyle())

	// Must return at least one line (blank sentinel)
	assert.NotEmpty(t, lines)
}

func TestRenderMarkdownNoteLines_Heading(t *testing.T) {
	t.Parallel()
	m := newModelForMarkdownTest(t)

	lines := m.renderMarkdownNoteLines("# My Heading", plainStyle())

	combined := strings.Join(lines, "\n")
	assert.Contains(t, combined, "My Heading")
}

func TestRenderMarkdownNoteLines_UncheckedCheckbox(t *testing.T) {
	t.Parallel()
	m := newModelForMarkdownTest(t)

	lines := m.renderMarkdownNoteLines("- [ ] do the thing", plainStyle())

	combined := strings.Join(lines, "\n")
	assert.Contains(t, combined, "do the thing")
}

func TestRenderMarkdownNoteLines_CheckedCheckbox(t *testing.T) {
	t.Parallel()
	m := newModelForMarkdownTest(t)

	lines := m.renderMarkdownNoteLines("- [x] done task", plainStyle())

	combined := strings.Join(lines, "\n")
	assert.Contains(t, combined, "done task")
}

func TestRenderMarkdownNoteLines_CodeFenceSkipsContent(t *testing.T) {
	t.Parallel()
	m := newModelForMarkdownTest(t)

	input := "```\nsome code here\n```\nafter fence"
	lines := m.renderMarkdownNoteLines(input, plainStyle())

	combined := strings.Join(lines, "\n")
	// Code fence delimiters themselves are stripped, but code content is rendered
	assert.Contains(t, combined, "some code here")
	assert.Contains(t, combined, "after fence")
	// The fence markers themselves should not appear
	assert.NotContains(t, combined, "```")
}

func TestRenderMarkdownNoteLines_UnorderedList(t *testing.T) {
	t.Parallel()
	m := newModelForMarkdownTest(t)

	lines := m.renderMarkdownNoteLines("- first item\n- second item", plainStyle())

	combined := strings.Join(lines, "\n")
	assert.Contains(t, combined, "first item")
	assert.Contains(t, combined, "second item")
}

func TestRenderMarkdownNoteLines_OrderedList(t *testing.T) {
	t.Parallel()
	m := newModelForMarkdownTest(t)

	lines := m.renderMarkdownNoteLines("1. alpha\n2. beta", plainStyle())

	combined := strings.Join(lines, "\n")
	assert.Contains(t, combined, "alpha")
	assert.Contains(t, combined, "beta")
}

func TestRenderMarkdownNoteLines_Blockquote(t *testing.T) {
	t.Parallel()
	m := newModelForMarkdownTest(t)

	lines := m.renderMarkdownNoteLines("> quoted text", plainStyle())

	combined := strings.Join(lines, "\n")
	assert.Contains(t, combined, "quoted text")
}

func TestRenderMarkdownNoteLines_HorizontalRule(t *testing.T) {
	t.Parallel()
	m := newModelForMarkdownTest(t)

	lines := m.renderMarkdownNoteLines("---", plainStyle())

	combined := strings.Join(lines, "\n")
	// HR renders as a repeated dash line
	assert.Contains(t, combined, "---")
}

func TestRenderMarkdownNoteLines_AnnotationTODO(t *testing.T) {
	t.Parallel()
	m := newModelForMarkdownTest(t)

	lines := m.renderMarkdownNoteLines("TODO: fix this", plainStyle())

	combined := strings.Join(lines, "\n")
	assert.Contains(t, combined, "TODO")
}

func TestRenderMarkdownNoteLines_AnnotationFIX(t *testing.T) {
	t.Parallel()
	m := newModelForMarkdownTest(t)

	lines := m.renderMarkdownNoteLines("FIX: broken logic", plainStyle())

	combined := strings.Join(lines, "\n")
	assert.Contains(t, combined, "FIX")
}

func TestRenderMarkdownNoteLines_AnnotationDONE(t *testing.T) {
	t.Parallel()
	m := newModelForMarkdownTest(t)

	lines := m.renderMarkdownNoteLines("DONE: already finished", plainStyle())

	combined := strings.Join(lines, "\n")
	assert.Contains(t, combined, "DONE")
}

func TestRenderMarkdownNoteLines_MultipleLines(t *testing.T) {
	t.Parallel()
	m := newModelForMarkdownTest(t)

	input := "line one\nline two\nline three"
	lines := m.renderMarkdownNoteLines(input, plainStyle())

	assert.Len(t, lines, 3)
}

func TestRenderMarkdownNoteLines_BlankLinesPreserved(t *testing.T) {
	t.Parallel()
	m := newModelForMarkdownTest(t)

	input := "first\n\nthird"
	lines := m.renderMarkdownNoteLines(input, plainStyle())

	// blank line becomes a single-space sentinel line
	assert.Len(t, lines, 3)
	assert.Equal(t, "  ", lines[1])
}

func TestRenderMarkdownNoteLines_WindowsLineEndings(t *testing.T) {
	t.Parallel()
	m := newModelForMarkdownTest(t)

	lines := m.renderMarkdownNoteLines("a\r\nb\r\nc", plainStyle())

	combined := strings.Join(lines, "\n")
	assert.Contains(t, combined, "a")
	assert.Contains(t, combined, "b")
	assert.Contains(t, combined, "c")
}

func TestParseMarkdownHeading(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input  string
		want   string
		wantOk bool
	}{
		{"# Heading 1", "Heading 1", true},
		{"## Heading 2", "Heading 2", true},
		{"###### Heading 6", "Heading 6", true},
		{"####### Too deep", "", false},
		{"#NoSpace", "", false},
		{"Not a heading", "", false},
		{"", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got, ok := parseMarkdownHeading(tt.input)
			assert.Equal(t, tt.wantOk, ok)
			if ok {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestParseMarkdownCheckbox(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input     string
		wantOk    bool
		wantCheck bool
		wantText  string
	}{
		{"- [ ] unchecked task", true, false, "unchecked task"},
		{"- [x] checked task", true, true, "checked task"},
		{"- [X] also checked", true, true, "also checked"},
		{"- [ ] ", true, false, "(empty task)"},
		{"* [ ] asterisk marker", true, false, "asterisk marker"},
		{"- not a checkbox", false, false, ""},
		{"  - [ ] indented", true, false, "indented"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			_, checked, text, ok := parseMarkdownCheckbox(tt.input)
			assert.Equal(t, tt.wantOk, ok)
			if ok {
				assert.Equal(t, tt.wantCheck, checked)
				assert.Equal(t, tt.wantText, text)
			}
		})
	}
}

func TestIsMarkdownHorizontalRule(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  bool
	}{
		{"---", true},
		{"***", true},
		{"___", true},
		{"- - -", true},
		{"----", true},
		{"--", false},
		{"", false},
		{"abc", false},
		{"-*-", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, isMarkdownHorizontalRule(tt.input))
		})
	}
}

func TestParseMarkdownUnorderedList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input      string
		wantOk     bool
		wantIndent int
		wantItem   string
	}{
		{"- item", true, 0, "item"},
		{"* item", true, 0, "item"},
		{"+ item", true, 0, "item"},
		{"  - nested", true, 1, "nested"},
		{"-no space", false, 0, ""},
		{"not a list", false, 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			indent, item, ok := parseMarkdownUnorderedList(tt.input)
			assert.Equal(t, tt.wantOk, ok)
			if ok {
				assert.Equal(t, tt.wantIndent, indent)
				assert.Equal(t, tt.wantItem, item)
			}
		})
	}
}
