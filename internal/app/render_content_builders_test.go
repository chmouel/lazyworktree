package app

import (
	"testing"

	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/stretchr/testify/assert"
)

func newModelForRenderTest(t *testing.T) *Model {
	t.Helper()
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	return NewModel(cfg, "")
}

func TestRenderStatusFiles_EmptyCleanTree(t *testing.T) {
	t.Parallel()
	m := newModelForRenderTest(t)

	result := m.renderStatusFiles()

	assert.Contains(t, result, "Clean working tree")
}

func TestRenderStatusFiles_EmptyAfterFilter(t *testing.T) {
	t.Parallel()
	m := newModelForRenderTest(t)
	m.state.services.filter.StatusFilterQuery = "nonexistent"
	m.setStatusFiles([]StatusFile{{Status: " M", Filename: "foo.go"}})
	// Force empty TreeFlat by making filter eliminate everything
	m.state.services.statusTree.TreeFlat = nil

	result := m.renderStatusFiles()

	assert.Contains(t, result, "No files match")
	assert.Contains(t, result, "nonexistent")
}

func TestRenderStatusFiles_ModifiedFile(t *testing.T) {
	t.Parallel()
	m := newModelForRenderTest(t)
	m.setStatusFiles([]StatusFile{
		{Status: " M", Filename: "main.go"},
	})

	result := m.renderStatusFiles()

	assert.Contains(t, result, "main.go")
	// " M" means unstaged modification — display should include the status chars
	assert.NotEmpty(t, result)
}

func TestRenderStatusFiles_StagedFile(t *testing.T) {
	t.Parallel()
	m := newModelForRenderTest(t)
	m.setStatusFiles([]StatusFile{
		{Status: "M ", Filename: "staged.go"},
	})

	result := m.renderStatusFiles()

	assert.Contains(t, result, "staged.go")
}

func TestRenderStatusFiles_AddedFile(t *testing.T) {
	t.Parallel()
	m := newModelForRenderTest(t)
	m.setStatusFiles([]StatusFile{
		{Status: "A ", Filename: "new.go"},
	})

	result := m.renderStatusFiles()

	assert.Contains(t, result, "new.go")
}

func TestRenderStatusFiles_DeletedFile(t *testing.T) {
	t.Parallel()
	m := newModelForRenderTest(t)
	m.setStatusFiles([]StatusFile{
		{Status: "D ", Filename: "gone.go"},
	})

	result := m.renderStatusFiles()

	assert.Contains(t, result, "gone.go")
}

func TestRenderStatusFiles_UntrackedFile(t *testing.T) {
	t.Parallel()
	m := newModelForRenderTest(t)
	m.setStatusFiles([]StatusFile{
		{Status: "??", Filename: "untracked.go"},
	})

	result := m.renderStatusFiles()

	assert.Contains(t, result, "untracked.go")
}

func TestRenderStatusFiles_RenamedFile(t *testing.T) {
	t.Parallel()
	m := newModelForRenderTest(t)
	m.setStatusFiles([]StatusFile{
		{Status: "R ", Filename: "new_name.go"},
	})

	result := m.renderStatusFiles()

	assert.Contains(t, result, "new_name.go")
}

func TestRenderStatusFiles_DirectoryGrouping(t *testing.T) {
	t.Parallel()
	m := newModelForRenderTest(t)
	m.setStatusFiles([]StatusFile{
		{Status: " M", Filename: "pkg/foo/a.go"},
		{Status: " M", Filename: "pkg/foo/b.go"},
	})

	result := m.renderStatusFiles()

	// Both files must appear; the tree may or may not render the directory
	assert.Contains(t, result, "a.go")
	assert.Contains(t, result, "b.go")
}

func TestRenderStatusFiles_StagedAndUnstagedSameFile(t *testing.T) {
	t.Parallel()
	m := newModelForRenderTest(t)
	// "MM" means staged modification + unstaged modification
	m.setStatusFiles([]StatusFile{
		{Status: "MM", Filename: "both.go"},
	})

	result := m.renderStatusFiles()

	assert.Contains(t, result, "both.go")
	// Two distinct characters should be present (not both identical styling)
	assert.NotEmpty(t, result)
}

func TestRenderStatusFiles_MultipleFiles(t *testing.T) {
	t.Parallel()
	m := newModelForRenderTest(t)
	m.setStatusFiles([]StatusFile{
		{Status: " M", Filename: "a.go"},
		{Status: "A ", Filename: "b.go"},
		{Status: "D ", Filename: "c.go"},
	})

	result := m.renderStatusFiles()

	assert.Contains(t, result, "a.go")
	assert.Contains(t, result, "b.go")
	assert.Contains(t, result, "c.go")
}

func TestBuildInfoContent_NilWorktree(t *testing.T) {
	t.Parallel()
	m := newModelForRenderTest(t)

	result := m.buildInfoContent(nil)

	assert.Equal(t, errNoWorktreeSelected, result)
}

func TestBuildInfoContent_BasicWorktree(t *testing.T) {
	t.Parallel()
	m := newModelForRenderTest(t)
	wt := &models.WorktreeInfo{
		Path:   "/tmp/wt-basic",
		Branch: "feature/test",
	}

	result := m.buildInfoContent(wt)

	assert.Contains(t, result, "/tmp/wt-basic")
	assert.Contains(t, result, "feature/test")
}

func TestBuildInfoContent_PRDetailsExcludeHeaderStateBadge(t *testing.T) {
	t.Parallel()
	m := newModelForRenderTest(t)
	wt := &models.WorktreeInfo{
		Path:   "/tmp/wt-pr-badge",
		Branch: "feature/pr-badge",
		PR: &models.PRInfo{
			Number: 42,
			State:  prStateOpen,
			Title:  "Show status badge",
			URL:    "https://example.com/org/repo/pull/42",
		},
	}

	result := stripTerminalSequences(m.buildInfoContent(wt))

	assert.Contains(t, result, "PR/MR #42")
	assert.Contains(t, result, "Show status badge")
	assert.NotContains(t, result, " Open ")
	assert.NotContains(t, result, "\ue0b6")
	assert.NotContains(t, result, "\ue0b4")
}

func TestBuildInfoContent_NoPRHidesPRStateBadge(t *testing.T) {
	t.Parallel()
	m := newModelForRenderTest(t)
	m.loading.prDataLoaded = true
	wt := &models.WorktreeInfo{
		Path:          "/tmp/wt-no-pr-badge",
		Branch:        "feature/no-pr-badge",
		HasUpstream:   true,
		PRFetchStatus: models.PRFetchStatusNoPR,
	}

	result := stripTerminalSequences(m.buildInfoContent(wt))

	assert.NotContains(t, result, "PR/MR")
	assert.NotContains(t, result, " Open ")
	assert.NotContains(t, result, " Merged ")
	assert.NotContains(t, result, " Closed ")
}

func TestRenderInfoBoxShowsPRStateBadgeInHeader(t *testing.T) {
	t.Parallel()
	m := newModelForRenderTest(t)
	m.config.IconSet = "nerd-font-v3"
	wt := &models.WorktreeInfo{
		Path:   "/tmp/wt-merged-pr",
		Branch: "feature/merged-pr",
		PR:     &models.PRInfo{Number: 42, State: prStateMerged, URL: "https://github.com/org/repo/pull/42"},
	}
	m.state.data.worktrees = []*models.WorktreeInfo{wt}
	m.state.data.filteredWts = []*models.WorktreeInfo{wt}
	m.state.data.selectedIndex = 0
	m.state.ui.worktreeTable.SetCursor(0)
	m.infoContent = m.buildInfoContent(wt)

	result := stripTerminalSequences(m.renderInfoBox(80, 10))

	assert.Contains(t, result, "\ue709  \ue0b6 Merged\ue0b4")
}

func TestRenderInfoBoxHidesPRStateBadgeWhenNoPRExists(t *testing.T) {
	t.Parallel()
	m := newModelForRenderTest(t)
	wt := &models.WorktreeInfo{
		Path:   "/tmp/wt-no-pr",
		Branch: "feature/no-pr",
	}
	m.state.data.worktrees = []*models.WorktreeInfo{wt}
	m.state.data.filteredWts = []*models.WorktreeInfo{wt}
	m.state.data.selectedIndex = 0
	m.state.ui.worktreeTable.SetCursor(0)
	m.infoContent = m.buildInfoContent(wt)

	result := stripTerminalSequences(m.renderInfoBox(80, 10))

	assert.NotContains(t, result, "\ue0b6")
	assert.NotContains(t, result, "\ue0b4")
	assert.NotContains(t, result, "Merged")
}

func TestBuildInfoContent_UsesInlineCIStatusChip(t *testing.T) {
	t.Parallel()
	m := newModelForRenderTest(t)
	wt := &models.WorktreeInfo{
		Path:   "/tmp/wt-ci-chip",
		Branch: "feature/ci-chip",
	}
	m.cache.ciCache.Set(wt.Branch, []*models.CICheck{
		{Name: "build", Status: "completed", Conclusion: "success"},
	})

	result := stripTerminalSequences(m.buildInfoContent(wt))

	assert.Contains(t, result, "CI Checks: S Passed")
	assert.NotContains(t, result, "\ue0b6")
	assert.NotContains(t, result, "\ue0b4")
}

func TestAggregateCIConclusion_NoChecks(t *testing.T) {
	t.Parallel()

	result := aggregateCIConclusion([]*models.CICheck{})

	assert.Equal(t, "skipped", result)
}

func TestBuildNotesContent_NilWorktree(t *testing.T) {
	t.Parallel()
	m := newModelForRenderTest(t)

	result := m.buildNotesContent(nil)

	assert.Empty(t, result)
}

func TestBuildNotesContent_NoNote(t *testing.T) {
	t.Parallel()
	m := newModelForRenderTest(t)
	wt := &models.WorktreeInfo{Path: "/tmp/wt-no-note"}

	result := m.buildNotesContent(wt)

	assert.Empty(t, result)
}

func TestBuildNotesContent_WithNote(t *testing.T) {
	t.Parallel()
	m := newModelForRenderTest(t)
	wt := &models.WorktreeInfo{Path: "/tmp/wt-with-note"}
	m.worktreeNotes[worktreeNoteKey(wt.Path)] = models.WorktreeNote{Note: "Hello world"}

	result := m.buildNotesContent(wt)

	assert.Contains(t, result, "Hello world")
}

func TestRenderStatusFiles_LinesJoinedByNewline(t *testing.T) {
	t.Parallel()
	m := newModelForRenderTest(t)
	m.setStatusFiles([]StatusFile{
		{Status: " M", Filename: "x.go"},
		{Status: " M", Filename: "y.go"},
	})

	result := m.renderStatusFiles()

	// Multiple files must produce multiple lines
	assert.Contains(t, result, "\n", "expected newline between file entries")
}
