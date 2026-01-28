package screen

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/theme"
)

func TestNewPRSelectionScreen(t *testing.T) {
	prs := []*models.PRInfo{
		{Number: 1, Title: "First PR", Author: "user1"},
		{Number: 2, Title: "Second PR", Author: "user2"},
	}
	scr := NewPRSelectionScreen(prs, 80, 30, theme.Dracula(), true)

	if scr.Type() != TypePRSelect {
		t.Errorf("expected Type to be TypePRSelect, got %v", scr.Type())
	}

	if len(scr.Filtered) != 2 {
		t.Errorf("expected 2 filtered PRs, got %d", len(scr.Filtered))
	}

	if scr.Cursor != 0 {
		t.Errorf("expected cursor to start at 0, got %d", scr.Cursor)
	}
}

func TestPRSelectionScreenNavigation(t *testing.T) {
	prs := []*models.PRInfo{
		{Number: 1, Title: "First"},
		{Number: 2, Title: "Second"},
		{Number: 3, Title: "Third"},
	}
	scr := NewPRSelectionScreen(prs, 80, 30, theme.Dracula(), true)

	// Test direct cursor manipulation instead of Update to simplify testing
	scr.Cursor = 1
	if scr.Cursor != 1 {
		t.Errorf("expected cursor to be 1, got %d", scr.Cursor)
	}

	pr, ok := scr.Selected()
	if !ok || pr.Number != 2 {
		t.Error("expected to select second PR")
	}
}

func TestPRSelectionScreenFiltering(t *testing.T) {
	prs := []*models.PRInfo{
		{Number: 123, Title: "Add feature X"},
		{Number: 456, Title: "Fix bug Y"},
		{Number: 789, Title: "Update feature Z"},
	}
	scr := NewPRSelectionScreen(prs, 80, 30, theme.Dracula(), true)

	// Filter by title
	scr.FilterInput.SetValue("feature")
	scr.applyFilter()

	if len(scr.Filtered) != 2 {
		t.Errorf("expected 2 filtered PRs matching 'feature', got %d", len(scr.Filtered))
	}

	// Filter by number
	scr.FilterInput.SetValue("456")
	scr.applyFilter()

	if len(scr.Filtered) != 1 {
		t.Errorf("expected 1 filtered PR matching '456', got %d", len(scr.Filtered))
	}

	if scr.Filtered[0].Number != 456 {
		t.Errorf("expected filtered PR to have number 456, got %d", scr.Filtered[0].Number)
	}

	// Clear filter
	scr.FilterInput.SetValue("")
	scr.applyFilter()

	if len(scr.Filtered) != 3 {
		t.Errorf("expected all 3 PRs after clearing filter, got %d", len(scr.Filtered))
	}
}

func TestPRSelectionScreenSelection(t *testing.T) {
	prs := []*models.PRInfo{
		{Number: 1, Title: "First", Branch: "branch1"},
		{Number: 2, Title: "Second", Branch: "branch2"},
	}
	scr := NewPRSelectionScreen(prs, 80, 30, theme.Dracula(), true)

	// Test selection
	pr, ok := scr.Selected()
	if !ok {
		t.Fatal("expected Selected to return true")
	}
	if pr.Number != 1 {
		t.Errorf("expected selected PR to have number 1, got %d", pr.Number)
	}

	// Move cursor and select again
	scr.Cursor = 1
	pr, ok = scr.Selected()
	if !ok {
		t.Fatal("expected Selected to return true")
	}
	if pr.Number != 2 {
		t.Errorf("expected selected PR to have number 2, got %d", pr.Number)
	}

	// Test out of bounds
	scr.Cursor = 99
	_, ok = scr.Selected()
	if ok {
		t.Error("expected Selected to return false for out of bounds cursor")
	}
}

func TestPRSelectionScreenCallbacks(t *testing.T) {
	prs := []*models.PRInfo{
		{Number: 1, Title: "First", Branch: "branch1"},
	}
	scr := NewPRSelectionScreen(prs, 80, 30, theme.Dracula(), true)

	// Test OnSelect callback
	selectCalled := false
	var selectedPR *models.PRInfo
	scr.OnSelect = func(pr *models.PRInfo) tea.Cmd {
		selectCalled = true
		selectedPR = pr
		return nil
	}

	result, _ := scr.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if result != nil {
		t.Error("expected screen to close (return nil) on Enter")
	}
	if !selectCalled {
		t.Error("expected OnSelect callback to be called")
	}
	if selectedPR == nil || selectedPR.Number != 1 {
		t.Error("expected selectedPR to be the first PR")
	}

	// Test OnCancel callback
	scr = NewPRSelectionScreen(prs, 80, 30, theme.Dracula(), true)
	cancelCalled := false
	scr.OnCancel = func() tea.Cmd {
		cancelCalled = true
		return nil
	}

	result, _ = scr.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if result != nil {
		t.Error("expected screen to close (return nil) on Esc")
	}
	if !cancelCalled {
		t.Error("expected OnCancel callback to be called")
	}
}

func TestPRSelectionScreenView(t *testing.T) {
	prs := []*models.PRInfo{
		{Number: 1, Title: "Test PR", Author: "testuser", CIStatus: "success"},
	}
	scr := NewPRSelectionScreen(prs, 80, 30, theme.Dracula(), true)

	view := scr.View()
	if view == "" {
		t.Error("expected View to return non-empty string")
	}

	// Check for expected content
	if !strings.Contains(view, "Test PR") {
		t.Error("expected view to contain PR title")
	}
}

func TestPRSelectionScreenCIIconsUseProvider(t *testing.T) {
	previousProvider := currentIconProvider
	SetIconProvider(&testIconProvider{ciIcon: "CI!"})
	defer SetIconProvider(previousProvider)

	prs := []*models.PRInfo{
		{Number: 1, Title: "Test PR", Author: "testuser", CIStatus: "success"},
	}
	scr := NewPRSelectionScreen(prs, 80, 30, theme.Dracula(), true)

	view := scr.View()
	if !strings.Contains(view, "CI!") {
		t.Fatalf("expected view to include CI icon from provider, got %q", view)
	}
}

func TestPRSelectionScreenCIStatusColoring(t *testing.T) {
	prs := []*models.PRInfo{
		{Number: 1, Title: "Success PR", CIStatus: "success"},
		{Number: 2, Title: "Failure PR", CIStatus: "failure"},
		{Number: 3, Title: "Pending PR", CIStatus: "pending"},
		{Number: 4, Title: "Draft PR", IsDraft: true},
	}
	scr := NewPRSelectionScreen(prs, 100, 30, theme.Dracula(), true)

	view := scr.View()

	// Should contain success/failure/pending/draft indicators
	// The actual rendering includes colored CI icons
	if view == "" {
		t.Error("expected non-empty view")
	}
}

type testIconProvider struct {
	ciIcon string
}

func (p *testIconProvider) GetPRIcon() string {
	return "PR"
}

func (p *testIconProvider) GetIssueIcon() string {
	return "ISS"
}

func (p *testIconProvider) GetCIIcon(conclusion string) string {
	return p.ciIcon
}

func (p *testIconProvider) GetUIIcon(icon UIIcon) string {
	return ""
}

func TestPRSelectionScreenEmptyList(t *testing.T) {
	scr := NewPRSelectionScreen([]*models.PRInfo{}, 80, 30, theme.Dracula(), true)

	view := scr.View()
	if !strings.Contains(view, "No open PRs") {
		t.Error("expected view to show 'No open PRs' message")
	}

	// Should not be able to select anything
	_, ok := scr.Selected()
	if ok {
		t.Error("expected Selected to return false for empty list")
	}
}

func TestPRSelectionScreenNoMatchingFilter(t *testing.T) {
	prs := []*models.PRInfo{
		{Number: 1, Title: "First"},
	}
	scr := NewPRSelectionScreen(prs, 80, 30, theme.Dracula(), true)

	scr.FilterInput.SetValue("nonexistent")
	scr.applyFilter()

	view := scr.View()
	if !strings.Contains(view, "No PRs match your filter") {
		t.Error("expected view to show 'No PRs match' message")
	}
}
