package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/theme"
)

func TestTrustScreenUpdateAndView(t *testing.T) {
	thm := theme.Dracula()
	screen := appscreen.NewTrustScreen("/tmp/.wt.yaml", []string{"echo hi"}, thm)

	called := false
	screen.OnTrust = func() tea.Cmd {
		called = true
		return nil
	}
	_, cmd := screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	if cmd != nil {
		t.Fatal("expected no command for trust")
	}
	if !called {
		t.Fatal("expected trust callback to be called")
	}

	view := screen.View()
	if !strings.Contains(view, "Trust") {
		t.Fatalf("expected trust screen view to include Trust label, got %q", view)
	}
}

func TestWelcomeScreenUpdateAndView(t *testing.T) {
	thm := theme.Dracula()
	screen := appscreen.NewWelcomeScreen("/tmp", "/tmp/worktrees", thm)

	called := false
	screen.OnQuit = func() tea.Cmd {
		called = true
		return tea.Quit
	}
	_, cmd := screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("expected quit command for quit key")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("expected quit command to return tea.QuitMsg")
	}
	if !called {
		t.Fatal("expected quit callback to be called")
	}

	view := screen.View()
	if !strings.Contains(view, "No worktrees found") {
		t.Fatalf("expected welcome view to include message, got %q", view)
	}
}

func TestCommitScreenUpdateAndView(t *testing.T) {
	thm := theme.Dracula()
	meta := appscreen.CommitMeta{
		SHA:     "abc123",
		Author:  "Test",
		Email:   "test@example.com",
		Date:    "Mon Jan 1 00:00:00 2024 +0000",
		Subject: "Add feature",
	}
	screen := appscreen.NewCommitScreen(meta, "stat", strings.Repeat("diff\n", 5), false, thm)

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
	meta := appscreen.CommitMeta{SHA: "123456"}
	thm := theme.Dracula()
	screen := appscreen.NewCommitFilesScreen("123456", "/tmp", files, meta, 100, 40, thm, false)

	if screen.CommitSHA != "123456" {
		t.Errorf("expected sha 123456, got %s", screen.CommitSHA)
	}
	if len(screen.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(screen.Files))
	}
	if screen.Tree == nil {
		t.Fatal("expected tree to be built")
	}
}

func TestBuildCommitFileTree(t *testing.T) {
	files := []models.CommitFile{
		{Filename: "a/b/c.go", ChangeType: "M"},
		{Filename: "a/d.go", ChangeType: "A"},
		{Filename: "e.go", ChangeType: "D"},
	}
	tree := appscreen.BuildCommitFileTree(files)

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
	tree := appscreen.BuildCommitFileTree(files)
	appscreen.SortCommitFileTree(tree)

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
	tree := appscreen.BuildCommitFileTree(files)
	// tree is Root -> a -> b -> c -> d.go
	// We want to test compression logic on child 'a'
	nodeA := tree.Children[0]
	appscreen.CompressCommitFileTree(nodeA)

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
	screen := appscreen.NewCommitFilesScreen("123", "", files, appscreen.CommitMeta{}, 100, 40, thm, false)

	// With NewCommitFilesScreen not compressing root, we expect [dir, file.go]
	if len(screen.TreeFlat) != 2 {
		t.Errorf("expected 2 items in flat list, got %d", len(screen.TreeFlat))
	}

	// Collapse "dir"
	screen.ToggleCollapse("dir")
	// flat: [dir]
	if len(screen.TreeFlat) != 1 {
		t.Errorf("expected 1 item in flat list after collapse, got %d", len(screen.TreeFlat))
	}

	screen.ToggleCollapse("dir")
	if len(screen.TreeFlat) != 2 {
		t.Errorf("expected 2 items in flat list after expand, got %d", len(screen.TreeFlat))
	}
}

func TestCommitFilesScreen_ApplyFilter(t *testing.T) {
	files := []models.CommitFile{
		{Filename: "foo.go", ChangeType: "M"},
		{Filename: "bar.go", ChangeType: "M"},
	}
	thm := theme.Dracula()
	screen := appscreen.NewCommitFilesScreen("123", "", files, appscreen.CommitMeta{}, 100, 40, thm, false)

	screen.FilterQuery = "foo"
	screen.ApplyFilter()

	if len(screen.TreeFlat) != 1 {
		t.Errorf("expected 1 item after filter, got %d", len(screen.TreeFlat))
	}
	if screen.TreeFlat[0].Path != "foo.go" {
		t.Errorf("expected 'foo.go', got %s", screen.TreeFlat[0].Path)
	}

	screen.FilterQuery = ""
	screen.ApplyFilter()
	if len(screen.TreeFlat) != 2 {
		t.Errorf("expected 2 items after clearing filter, got %d", len(screen.TreeFlat))
	}
}

func TestCommitFilesScreen_SearchNext(t *testing.T) {
	files := []models.CommitFile{
		{Filename: "a.go", ChangeType: "M"},
		{Filename: "b.go", ChangeType: "M"},
		{Filename: "c.go", ChangeType: "M"},
	}
	thm := theme.Dracula()
	screen := appscreen.NewCommitFilesScreen("123", "", files, appscreen.CommitMeta{}, 100, 40, thm, false)

	screen.SearchQuery = "b.go"
	screen.Cursor = 0 // on a.go
	screen.SearchNext(true)

	if screen.Cursor != 1 {
		t.Errorf("expected cursor at 1 (b.go), got %d", screen.Cursor)
	}

	screen.SearchQuery = "nonexistent"
	screen.SearchNext(true)
	if screen.Cursor != 1 {
		t.Errorf("cursor should stay at 1, got %d", screen.Cursor)
	}
}

func TestCommitFilesScreen_Update(t *testing.T) {
	files := []models.CommitFile{
		{Filename: "a.go", ChangeType: "M"},
		{Filename: "b.go", ChangeType: "M"},
	}
	thm := theme.Dracula()
	screen := appscreen.NewCommitFilesScreen("123", "", files, appscreen.CommitMeta{}, 100, 40, thm, false)

	// Test navigation
	screen.Cursor = 0
	screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if screen.Cursor != 1 {
		t.Errorf("expected cursor 1 after 'j', got %d", screen.Cursor)
	}
	screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if screen.Cursor != 0 {
		t.Errorf("expected cursor 0 after 'k', got %d", screen.Cursor)
	}

	// Test entering filter mode
	screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	if !screen.ShowingFilter {
		t.Error("expected ShowingFilter to be true after 'f'")
	}
	if !screen.FilterInput.Focused() {
		t.Error("expected filter input to be focused")
	}

	// Test typing in filter
	screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if screen.FilterInput.Value() != "a" {
		t.Errorf("expected filter input 'a', got %s", screen.FilterInput.Value())
	}
	// Should auto-apply filter
	if screen.FilterQuery != "a" {
		t.Errorf("expected filter query 'a', got %s", screen.FilterQuery)
	}

	// Exit filter
	screen.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if screen.ShowingFilter {
		t.Error("expected filter mode to end on Esc")
	}
	if screen.FilterQuery != "" {
		t.Error("expected filter to clear on Esc")
	}
}

func TestCommitFilesScreen_View(t *testing.T) {
	files := []models.CommitFile{
		{Filename: "test.go", ChangeType: "M"},
	}
	meta := appscreen.CommitMeta{
		SHA:     "abcdef",
		Author:  "Me",
		Subject: "Fix it",
	}
	thm := theme.Dracula()
	screen := appscreen.NewCommitFilesScreen("abcdef", "", files, meta, 100, 40, thm, false)

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
	screen := appscreen.NewCommitFilesScreen("123", "", files, appscreen.CommitMeta{}, 100, 40, thm, false)

	node := screen.GetSelectedNode()
	if node == nil {
		t.Fatal("expected node, got nil")
	}
	if node.Path != "a.go" {
		t.Errorf("expected path 'a.go', got %s", node.Path)
	}

	screen.Cursor = 100
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
		screen := appscreen.NewConfirmScreenWithDefault("Test message", 0, thm)
		if screen.SelectedButton != 0 {
			t.Fatalf("expected default button to be 0, got %d", screen.SelectedButton)
		}
		if screen.Message != "Test message" {
			t.Fatalf("expected message 'Test message', got %s", screen.Message)
		}
	})

	t.Run("default button 1 (Cancel)", func(t *testing.T) {
		screen := appscreen.NewConfirmScreenWithDefault("Test message", 1, thm)
		if screen.SelectedButton != 1 {
			t.Fatalf("expected default button to be 1, got %d", screen.SelectedButton)
		}
	})

	t.Run("regular NewConfirmScreen defaults to 0", func(t *testing.T) {
		screen := appscreen.NewConfirmScreen("Test message", thm)
		if screen.SelectedButton != 0 {
			t.Fatalf("expected NewConfirmScreen default button to be 0, got %d", screen.SelectedButton)
		}
	})
}

func TestNewLoadingScreen(t *testing.T) {
	thm := theme.Dracula()
	screen := appscreen.NewLoadingScreen("Loading data...", appscreen.TipOperationGeneral, thm, appscreen.DefaultSpinnerFrames(), false)

	if screen.Message != "Loading data..." {
		t.Errorf("expected message 'Loading data...', got %q", screen.Message)
	}
	if screen.Tip == "" {
		t.Error("expected tip to be set from LoadingTips")
	}
	if screen.Thm != thm {
		t.Error("expected theme to be set")
	}
	if screen.FrameIdx != 0 {
		t.Errorf("expected frameIdx to be 0, got %d", screen.FrameIdx)
	}
	if screen.BorderColorIdx != 0 {
		t.Errorf("expected borderColorIdx to be 0, got %d", screen.BorderColorIdx)
	}
}

func TestLoadingScreenTick(t *testing.T) {
	thm := theme.Dracula()
	screen := appscreen.NewLoadingScreen("Loading...", appscreen.TipOperationGeneral, thm, appscreen.DefaultSpinnerFrames(), false)

	// Initial state
	if screen.FrameIdx != 0 || screen.BorderColorIdx != 0 {
		t.Fatal("expected initial indices to be 0")
	}

	// First tick
	screen.Tick()
	if screen.FrameIdx != 1 {
		t.Errorf("expected frameIdx to be 1 after tick, got %d", screen.FrameIdx)
	}
	if screen.BorderColorIdx != 1 {
		t.Errorf("expected borderColorIdx to be 1 after tick, got %d", screen.BorderColorIdx)
	}

	// Tick until wrap around (spinnerFrames has 3 frames)
	screen.Tick()
	screen.Tick()
	if screen.FrameIdx != 0 {
		t.Errorf("expected frameIdx to wrap to 0, got %d", screen.FrameIdx)
	}
}

func TestLoadingScreenView(t *testing.T) {
	thm := theme.Dracula()
	screen := appscreen.NewLoadingScreen("Fetching PR data...", appscreen.TipOperationFetch, thm, appscreen.DefaultSpinnerFrames(), false)

	view := screen.View()

	// Check that the view contains key elements
	if !strings.Contains(view, "Fetching PR data...") {
		t.Error("expected view to contain message")
	}
	if !strings.Contains(view, "Tip:") {
		t.Error("expected view to contain tip label")
	}
	// Check for spinner characters (one of the frames)
	hasSpinner := strings.Contains(view, "●") || strings.Contains(view, "◌") || strings.Contains(view, ".")
	if !hasSpinner {
		t.Error("expected view to contain spinner characters")
	}
	// Check for separator line
	if !strings.Contains(view, "-") {
		t.Error("expected view to contain separator line")
	}
}

func TestLoadingScreenTipTruncation(t *testing.T) {
	thm := theme.Dracula()
	// Create a screen and manually set a very long tip
	screen := &appscreen.LoadingScreen{
		Message:       "Loading...",
		Tip:           strings.Repeat("very long tip segment ", 12),
		Thm:           thm,
		SpinnerFrames: appscreen.DefaultSpinnerFrames(),
	}

	view := screen.View()

	// The tip should be truncated and end with an ellipsis.
	if !strings.Contains(view, "…") {
		t.Error("expected long tip to be truncated with ellipsis")
	}
}

func TestLoadingScreenBorderColors(t *testing.T) {
	thm := theme.Dracula()
	screen := appscreen.NewLoadingScreen("Loading...", appscreen.TipOperationGeneral, thm, appscreen.DefaultSpinnerFrames(), false)

	colors := screen.LoadingBorderColours()
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

func TestSelectLoadingTipContextual(t *testing.T) {
	tip := appscreen.SelectLoadingTip(appscreen.TipOperationRerun, "")
	if tip.ID == "" {
		t.Fatal("expected a tip to be selected")
	}
	if !strings.Contains(strings.ToLower(tip.Text), "ci") {
		t.Fatalf("expected rerun tip to be CI-related, got %q", tip.Text)
	}
}

func TestSelectLoadingTipAvoidsImmediateRepeat(t *testing.T) {
	tip := appscreen.SelectLoadingTip(appscreen.TipOperationGeneral, "help")
	if tip.ID == "help" {
		t.Fatalf("expected selected tip to avoid immediate repeat, got %q", tip.ID)
	}
}

func TestTipOperationFromContext(t *testing.T) {
	if got := appscreen.TipOperationFromContext("sync", ""); got != appscreen.TipOperationSync {
		t.Fatalf("expected sync operation, got %q", got)
	}
	if got := appscreen.TipOperationFromContext("", "Fetching remotes..."); got != appscreen.TipOperationFetch {
		t.Fatalf("expected fetch operation, got %q", got)
	}
}

func TestHelpTips(t *testing.T) {
	lines := appscreen.HelpTips()
	if len(lines) == 0 {
		t.Fatal("expected at least one help tip")
	}
	if !strings.HasPrefix(lines[0], "- ") {
		t.Fatalf("expected help tip lines to be bullet points, got %q", lines[0])
	}
}
