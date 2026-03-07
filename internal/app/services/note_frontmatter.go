package services

import (
	"bytes"
	"fmt"

	"github.com/chmouel/lazyworktree/internal/models"
	"gopkg.in/yaml.v3"
)

var frontmatterDelim = []byte("---\n")

type noteFrontmatter struct {
	Icon      string `yaml:"icon,omitempty"`
	Color     string `yaml:"color,omitempty"`
	Bold      bool   `yaml:"bold,omitempty"`
	UpdatedAt int64  `yaml:"updated_at,omitempty"`
}

// ParseNoteFile parses a splitted note file (YAML frontmatter + markdown body).
func ParseNoteFile(data []byte) (models.WorktreeNote, error) {
	if len(data) == 0 {
		return models.WorktreeNote{}, nil
	}

	// Try to split on frontmatter delimiters.
	content := data
	if bytes.HasPrefix(content, frontmatterDelim) {
		content = content[len(frontmatterDelim):]
		end := bytes.Index(content, frontmatterDelim)
		if end < 0 {
			// No closing delimiter — treat whole file as note text.
			return models.WorktreeNote{Note: string(data)}, nil
		}

		fmData := content[:end]
		body := content[end+len(frontmatterDelim):]

		var fm noteFrontmatter
		if err := yaml.Unmarshal(fmData, &fm); err != nil {
			return models.WorktreeNote{}, fmt.Errorf("parsing frontmatter: %w", err)
		}

		return models.WorktreeNote{
			Icon:      fm.Icon,
			Color:     fm.Color,
			Bold:      fm.Bold,
			UpdatedAt: fm.UpdatedAt,
			Note:      string(body),
		}, nil
	}

	// No frontmatter — entire file is note text.
	return models.WorktreeNote{Note: string(data)}, nil
}

// FormatNoteFile serialises a worktree note as YAML frontmatter + markdown body.
func FormatNoteFile(note models.WorktreeNote) []byte {
	var buf bytes.Buffer

	fm := noteFrontmatter{
		Icon:      note.Icon,
		Color:     note.Color,
		Bold:      note.Bold,
		UpdatedAt: note.UpdatedAt,
	}

	// Only write frontmatter if there is metadata to write.
	if fm.Icon != "" || fm.Color != "" || fm.Bold || fm.UpdatedAt != 0 {
		buf.Write(frontmatterDelim)
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		_ = enc.Encode(&fm)
		_ = enc.Close()
		buf.Write(frontmatterDelim)
	}

	if note.Note != "" {
		buf.WriteString(note.Note)
		// Ensure file ends with newline.
		if note.Note[len(note.Note)-1] != '\n' {
			buf.WriteByte('\n')
		}
	}

	return buf.Bytes()
}
