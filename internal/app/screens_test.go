package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/theme"
)

func TestTrustScreenUpdateAndView(t *testing.T) {
	thm := theme.Dracula()
	screen := NewTrustScreen("/tmp/.wt.yaml", []string{"echo hi"}, thm)

	_, cmd := screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	if cmd == nil {
		t.Fatal("expected quit command for trust")
	}
	select {
	case result := <-screen.result:
		if result != "trust" {
			t.Fatalf("expected trust result, got %q", result)
		}
	default:
		t.Fatal("expected trust result to be sent")
	}

	view := screen.View()
	if !strings.Contains(view, "Trust") {
		t.Fatalf("expected trust screen view to include Trust label, got %q", view)
	}
}

func TestWelcomeScreenUpdateAndView(t *testing.T) {
	thm := theme.Dracula()
	screen := NewWelcomeScreen("/tmp", "/tmp/worktrees", thm)

	_, cmd := screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("expected quit command for quit key")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("expected quit command to return tea.QuitMsg")
	}
	select {
	case result := <-screen.result:
		if result {
			t.Fatal("expected quit result to be false")
		}
	default:
		t.Fatal("expected quit result to be sent")
	}

	view := screen.View()
	if !strings.Contains(view, "No worktrees found") {
		t.Fatalf("expected welcome view to include message, got %q", view)
	}
}

func TestCommitScreenUpdateAndView(t *testing.T) {
	thm := theme.Dracula()
	meta := commitMeta{
		sha:     "abc123",
		author:  "Test",
		email:   "test@example.com",
		date:    "Mon Jan 1 00:00:00 2024 +0000",
		subject: "Add feature",
	}
	screen := NewCommitScreen(meta, "stat", strings.Repeat("diff\n", 5), false, thm)

	if cmd := screen.Init(); cmd != nil {
		t.Fatal("expected nil init command")
	}

	_, cmd := screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if cmd != nil {
		t.Fatal("expected no command on scroll update")
	}

	view := screen.View()
	if !strings.Contains(view, "Commit:") || !strings.Contains(view, "abc123") {
		t.Fatalf("expected commit view to include metadata, got %q", view)
	}
}

func TestNewCommitFilesScreen(t *testing.T) {
	files := []models.CommitFile{
		{Filename: "cmd/main.go", ChangeType: "M"},
		{Filename: "internal/app.go", ChangeType: "A"},
	}
	meta := commitMeta{sha: "123456"}
	thm := theme.Dracula()
	screen := NewCommitFilesScreen("123456", "/tmp", files, meta, 100, 40, thm, false)

	if screen.commitSHA != "123456" {
		t.Errorf("expected sha 123456, got %s", screen.commitSHA)
	}
	if len(screen.files) != 2 {
		t.Errorf("expected 2 files, got %d", len(screen.files))
	}
	if screen.tree == nil {
		t.Fatal("expected tree to be built")
	}
}

func TestBuildCommitFileTree(t *testing.T) {
	files := []models.CommitFile{
		{Filename: "a/b/c.go", ChangeType: "M"},
		{Filename: "a/d.go", ChangeType: "A"},
		{Filename: "e.go", ChangeType: "D"},
	}
	tree := buildCommitFileTree(files)

	// Root children should be "a" and "e.go"
	if len(tree.Children) != 2 {
		t.Errorf("expected 2 root children, got %d", len(tree.Children))
	}
}

func TestSortCommitFileTree(t *testing.T) {
	files := []models.CommitFile{
		{Filename: "b.go", ChangeType: "M"},
		{Filename: "a/c.go", ChangeType: "M"},
	}
	tree := buildCommitFileTree(files)
	sortCommitFileTree(tree)

	// "a" (dir) should come before "b.go" (file)
	if tree.Children[0].Path != "a" {
		t.Errorf("expected 'a' first, got %s", tree.Children[0].Path)
	}
	if tree.Children[1].Path != "b.go" {
		t.Errorf("expected 'b.go' second, got %s", tree.Children[1].Path)
	}
}

func TestCompressCommitFileTree(t *testing.T) {
	files := []models.CommitFile{
		{Filename: "a/b/c/d.go", ChangeType: "M"},
	}
	tree := buildCommitFileTree(files)
	// tree is Root -> a -> b -> c -> d.go
	// We want to test compression logic on child 'a'
	nodeA := tree.Children[0]
	compressCommitFileTree(nodeA)

	if len(nodeA.Children) != 1 {
		t.Fatalf("expected 1 child for a, got %d", len(nodeA.Children))
	}
	if nodeA.Children[0].Path != "a/b/c/d.go" {
		t.Errorf("expected child path 'a/b/c/d.go', got %s", nodeA.Children[0].Path)
	}
	if nodeA.Compression != 2 {
		t.Errorf("expected compression 2, got %d", nodeA.Compression)
	}
}

func TestCommitFilesScreen_FlatRebuild(t *testing.T) {
	files := []models.CommitFile{
		{Filename: "dir/file.go", ChangeType: "M"},
	}
	thm := theme.Dracula()
	screen := NewCommitFilesScreen("123", "", files, commitMeta{}, 100, 40, thm, false)

	// With NewCommitFilesScreen not compressing root, we expect [dir, file.go]
	if len(screen.treeFlat) != 2 {
		t.Errorf("expected 2 items in flat list, got %d", len(screen.treeFlat))
	}

	// Collapse "dir"
	screen.ToggleCollapse("dir")
	// flat: [dir]
	if len(screen.treeFlat) != 1 {
		t.Errorf("expected 1 item in flat list after collapse, got %d", len(screen.treeFlat))
	}

	screen.ToggleCollapse("dir")
	if len(screen.treeFlat) != 2 {
		t.Errorf("expected 2 items in flat list after expand, got %d", len(screen.treeFlat))
	}
}

func TestCommitFilesScreen_ApplyFilter(t *testing.T) {
	files := []models.CommitFile{
		{Filename: "foo.go", ChangeType: "M"},
		{Filename: "bar.go", ChangeType: "M"},
	}
	thm := theme.Dracula()
	screen := NewCommitFilesScreen("123", "", files, commitMeta{}, 100, 40, thm, false)

	screen.filterQuery = "foo"
	screen.applyFilter()

	if len(screen.treeFlat) != 1 {
		t.Errorf("expected 1 item after filter, got %d", len(screen.treeFlat))
	}
	if screen.treeFlat[0].Path != "foo.go" {
		t.Errorf("expected 'foo.go', got %s", screen.treeFlat[0].Path)
	}

	screen.filterQuery = ""
	screen.applyFilter()
	if len(screen.treeFlat) != 2 {
		t.Errorf("expected 2 items after clearing filter, got %d", len(screen.treeFlat))
	}
}

func TestCommitFilesScreen_SearchNext(t *testing.T) {
	files := []models.CommitFile{
		{Filename: "a.go", ChangeType: "M"},
		{Filename: "b.go", ChangeType: "M"},
		{Filename: "c.go", ChangeType: "M"},
	}
	thm := theme.Dracula()
	screen := NewCommitFilesScreen("123", "", files, commitMeta{}, 100, 40, thm, false)

	screen.searchQuery = "b.go"
	screen.cursor = 0 // on a.go
	screen.searchNext(true)

	if screen.cursor != 1 {
		t.Errorf("expected cursor at 1 (b.go), got %d", screen.cursor)
	}

	screen.searchQuery = "nonexistent"
	screen.searchNext(true)
	if screen.cursor != 1 {
		t.Errorf("cursor should stay at 1, got %d", screen.cursor)
	}
}

func TestCommitFilesScreen_Update(t *testing.T) {
	files := []models.CommitFile{
		{Filename: "a.go", ChangeType: "M"},
		{Filename: "b.go", ChangeType: "M"},
	}
	thm := theme.Dracula()
	screen := NewCommitFilesScreen("123", "", files, commitMeta{}, 100, 40, thm, false)

	// Test navigation
	screen.cursor = 0
	screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if screen.cursor != 1 {
		t.Errorf("expected cursor 1 after 'j', got %d", screen.cursor)
	}
	screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if screen.cursor != 0 {
		t.Errorf("expected cursor 0 after 'k', got %d", screen.cursor)
	}

	// Test entering filter mode
	screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	if !screen.showingFilter {
		t.Error("expected showingFilter to be true after 'f'")
	}
	if !screen.filterInput.Focused() {
		t.Error("expected filter input to be focused")
	}

	// Test typing in filter
	screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if screen.filterInput.Value() != "a" {
		t.Errorf("expected filter input 'a', got %s", screen.filterInput.Value())
	}
	// Should auto-apply filter
	if screen.filterQuery != "a" {
		t.Errorf("expected filter query 'a', got %s", screen.filterQuery)
	}

	// Exit filter
	screen.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if screen.showingFilter {
		t.Error("expected filter mode to end on Esc")
	}
	if screen.filterQuery != "" {
		t.Error("expected filter to clear on Esc")
	}
}

func TestCommitFilesScreen_View(t *testing.T) {
	files := []models.CommitFile{
		{Filename: "test.go", ChangeType: "M"},
	}
	meta := commitMeta{
		sha:     "abcdef",
		author:  "Me",
		subject: "Fix it",
	}
	thm := theme.Dracula()
	screen := NewCommitFilesScreen("abcdef", "", files, meta, 100, 40, thm, false)

	view := screen.View()
	if !strings.Contains(view, "Files in commit abcdef") {
		t.Error("view missing title")
	}
	if !strings.Contains(view, "test.go") {
		t.Error("view missing file name")
	}
	if !strings.Contains(view, "Fix it") {
		t.Error("view missing commit subject")
	}
}

func TestCommitFilesScreen_GetSelectedNode(t *testing.T) {
	files := []models.CommitFile{
		{Filename: "a.go", ChangeType: "M"},
	}
	thm := theme.Dracula()
	screen := NewCommitFilesScreen("123", "", files, commitMeta{}, 100, 40, thm, false)

	node := screen.GetSelectedNode()
	if node == nil {
		t.Fatal("expected node, got nil")
	}
	if node.Path != "a.go" {
		t.Errorf("expected path 'a.go', got %s", node.Path)
	}

	screen.cursor = 100
	if screen.GetSelectedNode() != nil {
		t.Error("expected nil node for out of bounds cursor")
	}
}

func TestGetCIStatusIcon(t *testing.T) {
	tests := []struct {
		name     string
		ciStatus string
		isDraft  bool
		expected string
	}{
		{name: "draft takes precedence", ciStatus: "success", isDraft: true, expected: "D"},
		{name: "draft over failure", ciStatus: "failure", isDraft: true, expected: "D"},
		{name: "success icon", ciStatus: "success", isDraft: false, expected: "S"},
		{name: "failure icon", ciStatus: "failure", isDraft: false, expected: "F"},
		{name: "pending icon", ciStatus: "pending", isDraft: false, expected: "P"},
		{name: "skipped icon", ciStatus: "skipped", isDraft: false, expected: "-"},
		{name: "cancelled icon", ciStatus: "cancelled", isDraft: false, expected: "C"},
		{name: "none icon", ciStatus: "none", isDraft: false, expected: "?"},
		{name: "unknown defaults to none", ciStatus: "unknown", isDraft: false, expected: "?"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getCIStatusIcon(tt.ciStatus, tt.isDraft, false)
			if result != tt.expected {
				t.Errorf("getCIStatusIcon(%q, %v, false) = %q, want %q", tt.ciStatus, tt.isDraft, result, tt.expected)
			}
		})
	}
}

func TestNewConfirmScreenWithDefault(t *testing.T) {
	thm := theme.Dracula()

	t.Run("default button 0 (Confirm)", func(t *testing.T) {
		screen := NewConfirmScreenWithDefault("Test message", 0, thm)
		if screen.selectedButton != 0 {
			t.Fatalf("expected default button to be 0, got %d", screen.selectedButton)
		}
		if screen.message != "Test message" {
			t.Fatalf("expected message 'Test message', got %s", screen.message)
		}
	})

	t.Run("default button 1 (Cancel)", func(t *testing.T) {
		screen := NewConfirmScreenWithDefault("Test message", 1, thm)
		if screen.selectedButton != 1 {
			t.Fatalf("expected default button to be 1, got %d", screen.selectedButton)
		}
	})

	t.Run("regular NewConfirmScreen defaults to 0", func(t *testing.T) {
		screen := NewConfirmScreen("Test message", thm)
		if screen.selectedButton != 0 {
			t.Fatalf("expected NewConfirmScreen default button to be 0, got %d", screen.selectedButton)
		}
	})
}

func TestNewLoadingScreen(t *testing.T) {
	thm := theme.Dracula()
	screen := NewLoadingScreen("Loading data...", thm, true)

	if screen.message != "Loading data..." {
		t.Errorf("expected message 'Loading data...', got %q", screen.message)
	}
	if screen.tip == "" {
		t.Error("expected tip to be set from loadingTips")
	}
	if screen.thm != thm {
		t.Error("expected theme to be set")
	}
	if screen.frameIdx != 0 {
		t.Errorf("expected frameIdx to be 0, got %d", screen.frameIdx)
	}
	if screen.borderColorIdx != 0 {
		t.Errorf("expected borderColorIdx to be 0, got %d", screen.borderColorIdx)
	}
}

func TestLoadingScreenTick(t *testing.T) {
	thm := theme.Dracula()
	screen := NewLoadingScreen("Loading...", thm, true)

	// Initial state
	if screen.frameIdx != 0 || screen.borderColorIdx != 0 {
		t.Fatal("expected initial indices to be 0")
	}

	// First tick
	screen.Tick()
	if screen.frameIdx != 1 {
		t.Errorf("expected frameIdx to be 1 after tick, got %d", screen.frameIdx)
	}
	if screen.borderColorIdx != 1 {
		t.Errorf("expected borderColorIdx to be 1 after tick, got %d", screen.borderColorIdx)
	}

	// Tick until wrap around (spinnerFrames has 3 frames)
	screen.Tick()
	screen.Tick()
	if screen.frameIdx != 0 {
		t.Errorf("expected frameIdx to wrap to 0, got %d", screen.frameIdx)
	}
}

func TestLoadingScreenView(t *testing.T) {
	thm := theme.Dracula()
	screen := NewLoadingScreen("Fetching PR data...", thm, true)

	view := screen.View()

	// Check that the view contains key elements
	if !strings.Contains(view, "Fetching PR data...") {
		t.Error("expected view to contain message")
	}
	if !strings.Contains(view, "Tip:") {
		t.Error("expected view to contain tip label")
	}
	// Check for spinner characters (one of the frames)
	hasSpinner := strings.Contains(view, "●") || strings.Contains(view, "◌")
	if !hasSpinner {
		t.Error("expected view to contain spinner characters")
	}
	// Check for separator line
	if !strings.Contains(view, "─") {
		t.Error("expected view to contain separator line")
	}
}

func TestLoadingScreenTipTruncation(t *testing.T) {
	thm := theme.Dracula()
	// Create a screen and manually set a very long tip
	screen := &LoadingScreen{
		message:   "Loading...",
		tip:       "This is an extremely long tip that should definitely be truncated because it exceeds the maximum allowed length for display in the modal",
		thm:       thm,
		showIcons: true,
	}

	view := screen.View()

	// The tip should be truncated and end with "..."
	if !strings.Contains(view, "...") {
		t.Error("expected long tip to be truncated with ellipsis")
	}
}

func TestLoadingScreenBorderColors(t *testing.T) {
	thm := theme.Dracula()
	screen := NewLoadingScreen("Loading...", thm, true)

	colors := screen.loadingBorderColors()
	if len(colors) != 4 {
		t.Errorf("expected 4 border colors, got %d", len(colors))
	}
	// First and last should be accent (they cycle)
	if colors[0] != thm.Accent {
		t.Error("expected first color to be accent")
	}
	if colors[3] != thm.Accent {
		t.Error("expected last color to be accent")
	}
}
