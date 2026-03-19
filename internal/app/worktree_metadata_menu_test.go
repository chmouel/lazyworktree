package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

func setupMetadataTestModel(t *testing.T) *Model {
	t.Helper()
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.setWindowSize(120, 40)
	m.state.data.filteredWts = []*models.WorktreeInfo{{Path: testWorktreePath, Branch: "feat"}}
	m.state.data.selectedIndex = 0
	return m
}

func TestShowEditWorktreeMetadataMenuListsExpectedItems(t *testing.T) {
	m := setupMetadataTestModel(t)
	path := testWorktreePath
	m.setWorktreeDescription(path, "Fix auth flow")
	m.setWorktreeColor(path, "red")
	m.toggleWorktreeBold(path)
	m.setWorktreeNote(path, "existing note")
	m.setWorktreeIcon(path, "🚀")
	m.setWorktreeTags(path, []string{"bug", "urgent"})

	m.showEditWorktreeMetadataMenu()

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeListSelect {
		t.Fatalf("expected list selection screen, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}

	listScreen := m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)
	expected := []struct {
		id          string
		label       string
		description string
	}{
		{worktreeMetadataDescriptionID, "Description", "Current: Fix auth flow"},
		{worktreeMetadataColorID, "Colour", "Current: red, bold"},
		{worktreeMetadataNotesID, "Notes", "View existing note"},
		{worktreeMetadataIconID, "Icon", "Current: 🚀 Release / Launch"},
		{worktreeMetadataTagsID, "Tags", "Current: bug, urgent"},
	}

	if len(listScreen.Items) != len(expected) {
		t.Fatalf("expected %d metadata items, got %d", len(expected), len(listScreen.Items))
	}

	for i, want := range expected {
		got := listScreen.Items[i]
		if got.ID != want.id || got.Label != want.label || got.Description != want.description {
			t.Fatalf("item %d mismatch: got %#v want id=%q label=%q desc=%q", i, got, want.id, want.label, want.description)
		}
	}
}

func TestShowEditWorktreeMetadataMenuUsesActionCopyWhenUnset(t *testing.T) {
	m := setupMetadataTestModel(t)

	m.showEditWorktreeMetadataMenu()

	listScreen := m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)
	expected := []string{
		"Set a description",
		"Set a colour",
		"Add notes",
		"Set an icon",
		"Set tags",
	}

	for i, want := range expected {
		if listScreen.Items[i].Description != want {
			t.Fatalf("item %d description mismatch: got %q want %q", i, listScreen.Items[i].Description, want)
		}
	}
}

func TestShowEditWorktreeMetadataMenuAddsIconsWhenEnabled(t *testing.T) {
	m := setupMetadataTestModel(t)
	m.config.IconSet = "nerdfont"

	previousProvider := currentIconProvider
	SetIconProvider(&NerdFontV3Provider{})
	defer SetIconProvider(previousProvider)

	m.showEditWorktreeMetadataMenu()

	listScreen := m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)
	plainLabels := []string{"Description", "Colour", "Notes", "Icon", "Tags"}
	for i, plain := range plainLabels {
		if !strings.HasSuffix(listScreen.Items[i].Label, plain) {
			t.Fatalf("item %d label mismatch: got %q want suffix %q", i, listScreen.Items[i].Label, plain)
		}
		if listScreen.Items[i].Label == plain {
			t.Fatalf("expected icon-prefixed label for %q", plain)
		}
	}
}

func TestHandleBuiltInKeyEOpensMetadataMenuInWorktreePane(t *testing.T) {
	m := setupMetadataTestModel(t)
	m.state.view.FocusedPane = paneWorktrees

	_, cmd := m.handleBuiltInKey(tea.KeyPressMsg{Code: 'e', Text: string('e')})
	if cmd != nil {
		t.Fatal("expected metadata menu to open without deferred command")
	}
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeListSelect {
		t.Fatalf("expected metadata menu screen, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}
}

func TestHandleBuiltInKeyEPreservesGitStatusEditor(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.view.FocusedPane = paneGitStatus

	_, cmd := m.handleBuiltInKey(tea.KeyPressMsg{Code: 'e', Text: string('e')})
	if cmd != nil {
		t.Fatal("expected nil command without a selected file")
	}
	if m.state.ui.screenManager.IsActive() {
		t.Fatal("did not expect metadata menu in git status pane")
	}
}

func TestMetadataMenuSelectionOpensExpectedScreen(t *testing.T) {
	tests := []struct {
		name           string
		itemID         string
		seed           func(*Model, string)
		expectedScreen appscreen.Type
		expectedTitle  string
	}{
		{
			name:           "description",
			itemID:         worktreeMetadataDescriptionID,
			expectedScreen: appscreen.TypeInput,
			expectedTitle:  "Set worktree description",
		},
		{
			name:           "colour",
			itemID:         worktreeMetadataColorID,
			expectedScreen: appscreen.TypeListSelect,
			expectedTitle:  "Set worktree colour",
		},
		{
			name:           "notes without existing note",
			itemID:         worktreeMetadataNotesID,
			expectedScreen: appscreen.TypeTextarea,
		},
		{
			name:   "notes with existing note",
			itemID: worktreeMetadataNotesID,
			seed: func(m *Model, path string) {
				m.setWorktreeNote(path, "existing note")
			},
			expectedScreen: appscreen.TypeNoteView,
		},
		{
			name:           "icon",
			itemID:         worktreeMetadataIconID,
			expectedScreen: appscreen.TypeListSelect,
			expectedTitle:  "Set worktree icon",
		},
		{
			name:           "tags",
			itemID:         worktreeMetadataTagsID,
			expectedScreen: appscreen.TypeTagEditor,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := setupMetadataTestModel(t)
			if tt.seed != nil {
				tt.seed(m, testWorktreePath)
			}

			m.showEditWorktreeMetadataMenu()
			listScreen := m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)
			for listScreen.Cursor < len(listScreen.Items)-1 && listScreen.Items[listScreen.Cursor].ID != tt.itemID {
				listScreen.Cursor++
			}
			if listScreen.Items[listScreen.Cursor].ID != tt.itemID {
				t.Fatalf("did not find menu item %q", tt.itemID)
			}

			_, _ = m.handleScreenKey(tea.KeyPressMsg{Code: tea.KeyEnter})
			if !m.state.ui.screenManager.IsActive() {
				t.Fatal("expected screen to remain active after selection")
			}
			if m.state.ui.screenManager.Type() != tt.expectedScreen {
				t.Fatalf("expected screen %v, got %v", tt.expectedScreen, m.state.ui.screenManager.Type())
			}
			if tt.expectedTitle != "" && tt.expectedScreen == appscreen.TypeListSelect {
				next := m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)
				if next.Title != tt.expectedTitle {
					t.Fatalf("expected list selection title %q, got %q", tt.expectedTitle, next.Title)
				}
			}
		})
	}
}
