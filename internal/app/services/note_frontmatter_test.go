package services

import (
	"testing"

	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseNoteFileRoundTrip(t *testing.T) {
	note := models.WorktreeNote{
		Icon:      "⚡",
		UpdatedAt: 1709740800,
		Note:      "The note content here.\nSupports **markdown**.",
	}
	data := FormatNoteFile(note)
	got, err := ParseNoteFile(data)
	require.NoError(t, err)
	assert.Equal(t, note.Icon, got.Icon)
	assert.Equal(t, note.UpdatedAt, got.UpdatedAt)
	assert.Equal(t, note.Note+"\n", got.Note)
}

func TestParseNoteFileRoundTripWithColor(t *testing.T) {
	note := models.WorktreeNote{
		Icon:      "🔥",
		Color:     "red",
		UpdatedAt: 1709740800,
		Note:      "Coloured note.",
	}
	data := FormatNoteFile(note)
	got, err := ParseNoteFile(data)
	require.NoError(t, err)
	assert.Equal(t, note.Icon, got.Icon)
	assert.Equal(t, note.Color, got.Color)
	assert.Equal(t, note.UpdatedAt, got.UpdatedAt)
	assert.Equal(t, note.Note+"\n", got.Note)
}

func TestParseNoteFileEmptyIcon(t *testing.T) {
	note := models.WorktreeNote{
		UpdatedAt: 1709740800,
		Note:      "Just text.",
	}
	data := FormatNoteFile(note)
	got, err := ParseNoteFile(data)
	require.NoError(t, err)
	assert.Empty(t, got.Icon)
	assert.Equal(t, note.UpdatedAt, got.UpdatedAt)
}

func TestParseNoteFileEmptyBody(t *testing.T) {
	note := models.WorktreeNote{
		Icon:      "🔥",
		UpdatedAt: 1709740800,
	}
	data := FormatNoteFile(note)
	got, err := ParseNoteFile(data)
	require.NoError(t, err)
	assert.Equal(t, "🔥", got.Icon)
	assert.Empty(t, got.Note)
}

func TestParseNoteFileNoFrontmatter(t *testing.T) {
	raw := []byte("Just plain text\nwith newlines.")
	got, err := ParseNoteFile(raw)
	require.NoError(t, err)
	assert.Empty(t, got.Icon)
	assert.Equal(t, int64(0), got.UpdatedAt)
	assert.Equal(t, "Just plain text\nwith newlines.", got.Note)
}

func TestParseNoteFileEmpty(t *testing.T) {
	got, err := ParseNoteFile(nil)
	require.NoError(t, err)
	assert.Equal(t, models.WorktreeNote{}, got)
}

func TestParseNoteFileMalformedFrontmatter(t *testing.T) {
	raw := []byte("---\n: invalid yaml [[\n---\nbody\n")
	_, err := ParseNoteFile(raw)
	assert.Error(t, err)
}

func TestParseNoteFileNoClosingDelimiter(t *testing.T) {
	raw := []byte("---\nicon: 🔥\nsome text without closing\n")
	got, err := ParseNoteFile(raw)
	require.NoError(t, err)
	assert.Equal(t, string(raw), got.Note)
}

func TestFormatNoteFileNoMetadata(t *testing.T) {
	note := models.WorktreeNote{Note: "plain note"}
	data := FormatNoteFile(note)
	assert.NotContains(t, string(data), "---")
	assert.Contains(t, string(data), "plain note")
}

func TestParseNoteFileRoundTripWithDescription(t *testing.T) {
	note := models.WorktreeNote{
		Icon:        "⚡",
		Description: "Fix auth flow",
		UpdatedAt:   1709740800,
		Note:        "Some details.",
	}
	data := FormatNoteFile(note)
	got, err := ParseNoteFile(data)
	require.NoError(t, err)
	assert.Equal(t, note.Icon, got.Icon)
	assert.Equal(t, note.Description, got.Description)
	assert.Equal(t, note.UpdatedAt, got.UpdatedAt)
	assert.Equal(t, note.Note+"\n", got.Note)
}

func TestFormatNoteFileDescriptionOnly(t *testing.T) {
	note := models.WorktreeNote{Description: "My description", UpdatedAt: 1}
	data := FormatNoteFile(note)
	assert.Contains(t, string(data), "description:")
	assert.Contains(t, string(data), "My description")
	got, err := ParseNoteFile(data)
	require.NoError(t, err)
	assert.Equal(t, "My description", got.Description)
}

func TestFormatNoteFileColorOnly(t *testing.T) {
	note := models.WorktreeNote{Color: "#ff0000", UpdatedAt: 1}
	data := FormatNoteFile(note)
	assert.Contains(t, string(data), "color:")
	assert.Contains(t, string(data), "#ff0000")
	got, err := ParseNoteFile(data)
	require.NoError(t, err)
	assert.Equal(t, "#ff0000", got.Color)
}
