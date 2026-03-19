package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/models"
)

const (
	worktreeMetadataDescriptionID = "description"
	worktreeMetadataColorID       = "color"
	worktreeMetadataNotesID       = "notes"
	worktreeMetadataIconID        = "icon"
	worktreeMetadataTagsID        = "tags"
)

func worktreeMetadataDescriptionSummary(note models.WorktreeNote) string {
	if strings.TrimSpace(note.Description) == "" {
		return "Set a description"
	}
	return "Current: " + note.Description
}

func worktreeMetadataColorSummary(note models.WorktreeNote) string {
	switch {
	case note.Color == "" && !note.Bold:
		return "Set a colour"
	case note.Color == "" && note.Bold:
		return "Current: bold only"
	case note.Color != "" && note.Bold:
		return "Current: " + note.Color + ", bold"
	default:
		return "Current: " + note.Color
	}
}

func worktreeMetadataNotesSummary(note models.WorktreeNote) string {
	if strings.TrimSpace(note.Note) == "" {
		return "Add notes"
	}
	return "View existing note"
}

func worktreeMetadataIconSummary(note models.WorktreeNote) string {
	if strings.TrimSpace(note.Icon) == "" {
		return "Set an icon"
	}
	for _, item := range curatedIcons {
		if item.ID == note.Icon {
			return "Current: " + item.Label
		}
	}
	return "Current: " + note.Icon
}

func worktreeMetadataTagsSummary(note models.WorktreeNote) string {
	if len(note.Tags) == 0 {
		return "Set tags"
	}
	return "Current: " + strings.Join(note.Tags, ", ")
}

func (m *Model) buildWorktreeMetadataItems(wt *models.WorktreeInfo) []appscreen.SelectionItem {
	note, _ := m.getWorktreeNote(wt.Path)
	showIcons := m.config.IconsEnabled()
	return []appscreen.SelectionItem{
		{ID: worktreeMetadataDescriptionID, Label: labelWithIcon(UIIconWorktreeDescription, "Description", showIcons), Description: worktreeMetadataDescriptionSummary(note)},
		{ID: worktreeMetadataColorID, Label: labelWithIcon(UIIconWorktreeColour, "Colour", showIcons), Description: worktreeMetadataColorSummary(note)},
		{ID: worktreeMetadataNotesID, Label: labelWithIcon(UIIconWorktreeNotes, "Notes", showIcons), Description: worktreeMetadataNotesSummary(note)},
		{ID: worktreeMetadataIconID, Label: labelWithIcon(UIIconWorktreeIcon, "Icon", showIcons), Description: worktreeMetadataIconSummary(note)},
		{ID: worktreeMetadataTagsID, Label: labelWithIcon(UIIconWorktreeTags, "Tags", showIcons), Description: worktreeMetadataTagsSummary(note)},
	}
}

func (m *Model) showEditWorktreeMetadataMenu() tea.Cmd {
	wt := m.selectedWorktree()
	if wt == nil {
		return nil
	}

	scr := appscreen.NewListSelectionScreen(
		m.buildWorktreeMetadataItems(wt),
		"Edit worktree metadata",
		"Filter metadata fields...",
		"No metadata fields found.",
		m.state.view.WindowWidth,
		m.state.view.WindowHeight,
		"",
		m.theme,
	)

	scr.OnSelect = func(item appscreen.SelectionItem) tea.Cmd {
		switch item.ID {
		case worktreeMetadataDescriptionID:
			return m.showSetWorktreeDescription()
		case worktreeMetadataColorID:
			return m.showSetWorktreeColor()
		case worktreeMetadataNotesID:
			return m.showAnnotateWorktree()
		case worktreeMetadataIconID:
			return m.showSetWorktreeIcon()
		case worktreeMetadataTagsID:
			return m.showSetWorktreeTags()
		default:
			return nil
		}
	}
	scr.OnCancel = func() tea.Cmd {
		return nil
	}

	m.state.ui.screenManager.Push(scr)
	return nil
}
