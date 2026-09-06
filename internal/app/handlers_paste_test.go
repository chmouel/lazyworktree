package app

import (
	"testing"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
)

func TestHandlePasteMsgFillsFilterInput(t *testing.T) {
	m := newTestModel(t)
	m.state.view.ShowingFilter = true
	m.state.view.FilterTarget = filterTargetWorktrees
	m.state.ui.filterInput.Focus()

	updated, _ := m.handlePasteMsg(tea.PasteMsg{Content: "pasted-query"})
	m, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}

	if got := m.state.ui.filterInput.Value(); got != "pasted-query" {
		t.Fatalf("expected filter input to contain pasted content, got %q", got)
	}
	if m.state.services.filter.FilterQuery != "pasted-query" {
		t.Fatalf("expected filter query to be updated, got %q", m.state.services.filter.FilterQuery)
	}
}

func TestHandlePasteMsgFillsSearchInput(t *testing.T) {
	m := newTestModel(t)
	m.state.view.ShowingSearch = true
	m.state.view.SearchTarget = searchTargetWorktrees
	m.state.ui.filterInput.Focus()

	updated, _ := m.handlePasteMsg(tea.PasteMsg{Content: "pasted-search"})
	m, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}

	if got := m.state.ui.filterInput.Value(); got != "pasted-search" {
		t.Fatalf("expected search input to contain pasted content, got %q", got)
	}
}

func TestHandlePasteMsgRoutesToActiveScreen(t *testing.T) {
	m := newTestModel(t)
	inputScr := appscreen.NewInputScreen("Branch name", "feature/my-branch", "", m.theme, m.config.IconsEnabled())
	m.state.ui.screenManager.Push(inputScr)

	updated, _ := m.handlePasteMsg(tea.PasteMsg{Content: "feature/pasted-branch"})
	m, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}

	scr, ok := m.state.ui.screenManager.Current().(*appscreen.InputScreen)
	if !ok {
		t.Fatalf("expected input screen to remain active, got %T", m.state.ui.screenManager.Current())
	}
	if got := scr.Input.Value(); got != "feature/pasted-branch" {
		t.Fatalf("expected pasted content in screen input, got %q", got)
	}
}

// TestPasteMsgThroughUpdateDispatcherFillsWorktreeFilter drives paste through
// updateModel to cover the app.go dispatch wiring, not just handlePasteMsg.
func TestPasteMsgThroughUpdateDispatcherFillsWorktreeFilter(t *testing.T) {
	m := newTestModel(t)
	m.state.view.FocusedPane = 0

	updated, _ := m.handleKeyMsg(tea.KeyPressMsg{Code: 'f', Text: "f"})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	updated, _ = m.Update(tea.PasteMsg{Content: "pasted-query"})
	m, ok = updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}

	if got := m.state.ui.filterInput.Value(); got != "pasted-query" {
		t.Fatalf("expected filter input to contain pasted content, got %q", got)
	}
	if m.state.services.filter.FilterQuery != "pasted-query" {
		t.Fatalf("expected filter query to be updated, got %q", m.state.services.filter.FilterQuery)
	}
}

func TestHandlePasteMsgNarrowsStatusFilter(t *testing.T) {
	m := newTestModel(t)
	m.state.view.FocusedPane = 2
	m.state.ui.statusViewport = viewport.New(viewport.WithWidth(40), viewport.WithHeight(10))
	m.setStatusFiles([]StatusFile{
		{Filename: "app.go", Status: ".M"},
		{Filename: testReadme, Status: ".M"},
	})

	updated, _ := m.handleKeyMsg(tea.KeyPressMsg{Code: 'f', Text: "f"})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	updated, _ = m.handlePasteMsg(tea.PasteMsg{Content: "read"})
	m, ok = updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}

	if len(m.state.data.statusFiles) != 1 {
		t.Fatalf("expected 1 filtered status file, got %d", len(m.state.data.statusFiles))
	}
	if m.state.data.statusFiles[0].Filename != testReadme {
		t.Fatalf("expected %s, got %q", testReadme, m.state.data.statusFiles[0].Filename)
	}
}

func TestHandlePasteMsgNarrowsLogFilter(t *testing.T) {
	m := newTestModel(t)
	m.state.view.FocusedPane = 3
	m.setLogEntries([]commitLogEntry{
		{sha: "abc123", authorInitials: "ab", message: "Fix bug in parser"},
		{sha: "def456", authorInitials: "de", message: "Add new feature"},
	}, false)

	updated, _ := m.handleKeyMsg(tea.KeyPressMsg{Code: 'f', Text: "f"})
	updatedModel, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	m = updatedModel

	updated, _ = m.handlePasteMsg(tea.PasteMsg{Content: "fix"})
	m, ok = updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}

	if len(m.state.data.logEntries) != 1 {
		t.Fatalf("expected 1 filtered commit, got %d", len(m.state.data.logEntries))
	}
	if m.state.data.logEntries[0].sha != "abc123" {
		t.Fatalf("expected commit abc123, got %q", m.state.data.logEntries[0].sha)
	}
}

func TestHandlePasteMsgIgnoredWhenNothingFocused(t *testing.T) {
	m := newTestModel(t)

	updated, cmd := m.handlePasteMsg(tea.PasteMsg{Content: "stray paste"})
	m, ok := updated.(*Model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	if cmd != nil {
		t.Fatal("expected no command when nothing is focused")
	}
	if m.state.ui.filterInput.Value() != "" {
		t.Fatalf("expected filter input to remain empty, got %q", m.state.ui.filterInput.Value())
	}
}
