package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

func TestParseCommitOptions(t *testing.T) {
	raw := strings.Join([]string{
		"full1\x1fshort1\x1f2024-01-01\x1fFirst commit",
		"bad-line",
		"full2\x1fshort2\x1f2024-01-02\x1fSecond commit",
	}, "\n")

	got := parseCommitOptions(raw)
	if len(got) != 2 {
		t.Fatalf("expected 2 commit options, got %d", len(got))
	}
	if got[0].fullHash != "full1" || got[0].shortHash != "short1" || got[0].date != "2024-01-01" || got[0].subject != "First commit" {
		t.Errorf("unexpected first commit: %+v", got[0])
	}
	if got[1].fullHash != "full2" || got[1].shortHash != "short2" || got[1].date != "2024-01-02" || got[1].subject != "Second commit" {
		t.Errorf("unexpected second commit: %+v", got[1])
	}
}

func TestSuggestBranchNameWithExisting(t *testing.T) {
	existing := map[string]struct{}{
		"main":   {},
		"main-1": {},
		"dev":    {},
	}

	tests := []struct {
		name     string
		base     string
		expected string
	}{
		{
			name:     "empty base",
			base:     "",
			expected: "",
		},
		{
			name:     "base not taken",
			base:     "feature",
			expected: "feature",
		},
		{
			name:     "base taken uses suffix",
			base:     "dev",
			expected: "dev-1",
		},
		{
			name:     "increments suffix",
			base:     "main",
			expected: "main-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := suggestBranchNameWithExisting(tt.base, existing); got != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestSanitizeBranchNameFromTitle(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		fallback string
		expected string
	}{
		// Basic functionality
		{
			name:     "basic title",
			title:    "Fix: Add new feature!",
			fallback: "abc123",
			expected: "fix-add-new-feature",
		},
		{
			name:     "limits length",
			title:    "This is a very long commit title that should be truncated to fifty characters",
			fallback: "abc123",
			expected: "this-is-a-very-long-commit-title-that-should-be-tr",
		},
		{
			name:     "empty uses fallback",
			title:    "!!!",
			fallback: "abc123",
			expected: "abc123",
		},
		{
			name:     "empty uses commit",
			title:    "",
			fallback: "",
			expected: "commit",
		},
		// Special character conversions (the bug fix)
		{
			name:     "dots converted to hyphens",
			title:    "feature.new",
			fallback: "",
			expected: "feature-new",
		},
		{
			name:     "multiple dots converted to single hyphen",
			title:    "better.agent.md",
			fallback: "",
			expected: "better-agent-md",
		},
		{
			name:     "spaces converted to hyphens",
			title:    "bug fix here",
			fallback: "",
			expected: "bug-fix-here",
		},
		{
			name:     "multiple spaces converted to single hyphen",
			title:    "feature   with   spaces",
			fallback: "",
			expected: "feature-with-spaces",
		},
		{
			name:     "slashes converted to hyphens",
			title:    "path/to/branch",
			fallback: "",
			expected: "path-to-branch",
		},
		{
			name:     "backslashes converted to hyphens",
			title:    "path\\to\\branch",
			fallback: "",
			expected: "path-to-branch",
		},
		{
			name:     "colons converted to hyphens",
			title:    "feature:test",
			fallback: "",
			expected: "feature-test",
		},
		{
			name:     "underscores converted to hyphens",
			title:    "test_underscore",
			fallback: "",
			expected: "test-underscore",
		},
		{
			name:     "mixed special characters",
			title:    "feature.new test:case v2_0",
			fallback: "",
			expected: "feature-new-test-case-v2-0",
		},
		{
			name:     "leading hyphens removed",
			title:    "---feature",
			fallback: "",
			expected: "feature",
		},
		{
			name:     "trailing hyphens removed",
			title:    "feature---",
			fallback: "",
			expected: "feature",
		},
		{
			name:     "consecutive hyphens collapsed",
			title:    "feature---new---thing",
			fallback: "",
			expected: "feature-new-thing",
		},
		{
			name:     "special chars at edges",
			title:    "...feature...",
			fallback: "",
			expected: "feature",
		},
		{
			name:     "github issue format",
			title:    "#42: Fix the login API",
			fallback: "",
			expected: "42-fix-the-login-api",
		},
		{
			name:     "gitlab mr format",
			title:    "!123 Add feature.new",
			fallback: "",
			expected: "123-add-feature-new",
		},
		{
			name:     "common filename pattern",
			title:    "update package.json",
			fallback: "",
			expected: "update-package-json",
		},
		{
			name:     "pr title with special chars",
			title:    "Add user.authentication feature (v2.0)",
			fallback: "",
			expected: "add-user-authentication-feature-v2-0",
		},
		{
			name:     "case insensitivity",
			title:    "UPPERCASE FEATURE",
			fallback: "",
			expected: "uppercase-feature",
		},
		{
			name:     "all special chars together",
			title:    "test.file (v1_0) [beta]: feature/update!@#$%",
			fallback: "",
			expected: "test-file-v1-0-beta-feature-update",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeBranchNameFromTitle(tt.title, tt.fallback); got != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestBaseRefExists(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir: t.TempDir(),
	}
	m := NewModel(cfg, "")

	if !m.baseRefExists("HEAD") {
		t.Fatal("expected HEAD to exist")
	}
	if m.baseRefExists("refs/does-not-exist") {
		t.Fatal("expected ref to not exist")
	}
}

func TestStripRemotePrefix(t *testing.T) {
	tests := []struct {
		name     string
		branch   string
		expected string
	}{
		{
			name:     "remote branch with origin",
			branch:   "origin/main",
			expected: "main",
		},
		{
			name:     "remote branch with other remote",
			branch:   "upstream/feature",
			expected: "feature",
		},
		{
			name:     "local branch",
			branch:   "main",
			expected: "main",
		},
		{
			name:     "branch with multiple slashes",
			branch:   "origin/feature/test",
			expected: "feature/test",
		},
		{
			name:     "empty string",
			branch:   "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripRemotePrefix(tt.branch); got != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestParseBranchOptionsWithDate(t *testing.T) {
	now := time.Now().Unix()
	yesterday := now - 86400
	raw := strings.Join([]string{
		fmt.Sprintf("main\trefs/heads/main\t%d", now),
		fmt.Sprintf("feature/x\trefs/heads/feature/x\t%d", yesterday),
		fmt.Sprintf("origin/main\trefs/remotes/origin/main\t%d", now),
		fmt.Sprintf("v1.0.0\trefs/tags/v1.0.0\t%d", yesterday),
		fmt.Sprintf("origin/HEAD\trefs/remotes/origin/HEAD\t%d", now),
		"",
	}, "\n")

	got := parseBranchOptionsWithDate(raw)
	if len(got) != 4 {
		t.Fatalf("expected 4 branch/tag options, got %d", len(got))
	}
	if got[0].name != mainWorktreeName || got[0].isRemote || got[0].isTag {
		t.Errorf("expected main to be local branch, got %+v", got[0])
	}
	if got[0].committerDate.IsZero() {
		t.Errorf("expected main to have a commit date")
	}
	if got[1].name != "feature/x" || got[1].isRemote || got[1].isTag {
		t.Errorf("expected feature/x to be local branch, got %+v", got[1])
	}
	if got[2].name != "origin/main" || !got[2].isRemote || got[2].isTag {
		t.Errorf("expected origin/main to be remote branch, got %+v", got[2])
	}
	if got[3].name != "v1.0.0" || got[3].isRemote || !got[3].isTag {
		t.Errorf("expected v1.0.0 to be tag, got %+v", got[3])
	}
}

func TestSortBranchOptions(t *testing.T) {
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	lastWeek := now.Add(-7 * 24 * time.Hour)

	tests := []struct {
		name     string
		input    []branchOption
		expected []string
	}{
		{
			name: "local main first",
			input: []branchOption{
				{name: "feature", isRemote: false, committerDate: now},
				{name: "main", isRemote: false, committerDate: yesterday},
				{name: "dev", isRemote: false, committerDate: lastWeek},
			},
			expected: []string{"main", "feature", "dev"},
		},
		{
			name: "local master when no main",
			input: []branchOption{
				{name: "feature", isRemote: false, committerDate: now},
				{name: "master", isRemote: false, committerDate: yesterday},
				{name: "dev", isRemote: false, committerDate: lastWeek},
			},
			expected: []string{"master", "feature", "dev"},
		},
		{
			name: "remote origin/main after local main",
			input: []branchOption{
				{name: "feature", isRemote: false, committerDate: now},
				{name: "main", isRemote: false, committerDate: yesterday},
				{name: "origin/main", isRemote: true, committerDate: now},
			},
			expected: []string{"main", "origin/main", "feature"},
		},
		{
			name: "date sorting for others",
			input: []branchOption{
				{name: "old-branch", isRemote: false, committerDate: lastWeek},
				{name: "new-branch", isRemote: false, committerDate: now},
				{name: "mid-branch", isRemote: false, committerDate: yesterday},
			},
			expected: []string{"new-branch", "mid-branch", "old-branch"},
		},
		{
			name: "same date alphabetical tiebreaker",
			input: []branchOption{
				{name: "zebra", isRemote: false, committerDate: now},
				{name: "alpha", isRemote: false, committerDate: now},
				{name: "beta", isRemote: false, committerDate: now},
			},
			expected: []string{"alpha", "beta", "zebra"},
		},
		{
			name:     "empty list",
			input:    []branchOption{},
			expected: []string{},
		},
		{
			name: "all priority branches",
			input: []branchOption{
				{name: "feature", isRemote: false, committerDate: now},
				{name: "origin/master", isRemote: true, committerDate: now},
				{name: "main", isRemote: false, committerDate: yesterday},
				{name: "origin/main", isRemote: true, committerDate: now},
				{name: "master", isRemote: false, committerDate: lastWeek},
			},
			expected: []string{"main", "origin/main", "origin/master", "feature"},
		},
		{
			name: "tags mixed with branches by date",
			input: []branchOption{
				{name: "feature", isRemote: false, isTag: false, committerDate: now},
				{name: "v1.0.0", isRemote: false, isTag: true, committerDate: yesterday},
				{name: "main", isRemote: false, isTag: false, committerDate: lastWeek},
				{name: "v0.9.0", isRemote: false, isTag: true, committerDate: lastWeek.Add(-24 * time.Hour)},
				{name: "dev", isRemote: false, isTag: false, committerDate: yesterday.Add(-12 * time.Hour)},
			},
			expected: []string{"main", "feature", "v1.0.0", "dev", "v0.9.0"},
		},
		{
			name: "tags don't appear in priority positions",
			input: []branchOption{
				{name: "main", isRemote: false, isTag: true, committerDate: now},
				{name: "feature", isRemote: false, isTag: false, committerDate: yesterday},
				{name: "main", isRemote: false, isTag: false, committerDate: lastWeek},
			},
			expected: []string{"main", "main", "feature"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sortBranchOptions(tt.input)
			if len(got) != len(tt.expected) {
				t.Fatalf("expected %d branches, got %d", len(tt.expected), len(got))
			}
			for i, expected := range tt.expected {
				if got[i].name != expected {
					t.Errorf("at index %d: expected %q, got %q", i, expected, got[i].name)
				}
			}
		})
	}
}

type repoInfo struct {
	dir    string
	branch string
	commit commitOption
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...) //#nosec G204 -- test helper with controlled args
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func initTestRepo(t *testing.T) repoInfo {
	t.Helper()

	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")
	runGit(t, dir, "config", "commit.gpgsign", "false")

	filePath := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(filePath, []byte("one\n"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGit(t, dir, "add", "file.txt")
	runGit(t, dir, "commit", "-m", "Initial commit")

	if err := os.WriteFile(filePath, []byte("two\n"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGit(t, dir, "commit", "-am", "Add new feature")

	branch := runGit(t, dir, "branch", "--show-current")
	if branch == "" {
		t.Fatal("expected branch name")
	}

	runGit(t, dir, "branch", featureBranch)
	runGit(t, dir, "update-ref", "refs/remotes/origin/"+branch, "HEAD")

	log := runGit(t, dir, "log", "-1", "--pretty=format:%H%x1f%h%x1f%s")
	parts := strings.SplitN(log, "\x1f", 3)
	if len(parts) < 3 {
		t.Fatalf("unexpected log output: %q", log)
	}

	return repoInfo{
		dir:    dir,
		branch: branch,
		commit: commitOption{fullHash: parts[0], shortHash: parts[1], subject: parts[2]},
	}
}

func withCwd(t *testing.T, dir string) {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})
}

func TestShowBaseSelection(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40

	cmd := m.showBaseSelection(mainWorktreeName)
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeListSelect {
		t.Fatal("expected list screen to be active")
	}

	listScreen := m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)
	if listScreen.OnSelect == nil {
		t.Fatal("expected OnSelect callback to be set")
	}
	listScreen.OnSelect(appscreen.SelectionItem{ID: "freeform"})

	// Input screen should now be pushed to screen manager
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInput {
		t.Fatalf("expected input screen to be active, got type %v", m.state.ui.screenManager.Type())
	}
	inputScr := m.state.ui.screenManager.Current().(*appscreen.InputScreen)
	if inputScr.Prompt != "Base ref" {
		t.Fatalf("expected base ref prompt, got %q", inputScr.Prompt)
	}
}

func TestHandleScreenKeyKeepsNewScreenOnSelection(t *testing.T) {
	repo := initTestRepo(t)
	withCwd(t, repo.dir)

	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40

	cmd := m.showBaseSelection(repo.branch)
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeListSelect {
		t.Fatal("expected list screen to be active")
	}

	baseScreen := m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)
	idx := -1
	for i, item := range baseScreen.Items {
		if item.ID == "branch-list" {
			idx = i
			break
		}
	}
	if idx == -1 {
		t.Fatal("expected branch-list option in base selection")
	}
	baseScreen.Cursor = idx

	_, _ = m.handleScreenKey(tea.KeyPressMsg{Code: tea.KeyEnter})

	if !m.state.ui.screenManager.IsActive() {
		t.Fatal("expected a screen to remain active after selection")
	}
	if m.state.ui.screenManager.Current() == baseScreen {
		t.Fatal("expected selection to open a new screen")
	}
	if m.state.ui.screenManager.StackDepth() == 0 {
		t.Fatal("expected previous screen to remain on the stack")
	}
}

func TestShowBaseSelectionFromPROption(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40

	cmd := m.showBaseSelection(mainWorktreeName)
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeListSelect {
		t.Fatal("expected list screen to be active")
	}

	listScreen := m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)

	// Verify the from-pr option exists
	found := false
	for _, item := range listScreen.Items {
		if item.ID == "from-pr" {
			found = true
			if item.Label != "Create from PR/MR" {
				t.Fatalf("expected label 'Create from PR/MR', got %q", item.Label)
			}
			break
		}
	}
	if !found {
		t.Fatal("expected 'from-pr' option in base selection")
	}

	// Verify selecting from-pr returns a command (the async PR fetch)
	resultCmd := listScreen.OnSelect(appscreen.SelectionItem{ID: "from-pr"})
	if resultCmd == nil {
		t.Fatal("expected command from from-pr selection")
	}
}

func TestShowFreeformBaseInputValidation(t *testing.T) {
	repo := initTestRepo(t)
	withCwd(t, repo.dir)

	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	m.showFreeformBaseInput(repo.branch)
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInput {
		t.Fatal("expected input screen to be active")
	}
	inputScr := m.state.ui.screenManager.Current().(*appscreen.InputScreen)

	// Test empty base ref validation
	inputScr.OnSubmit(" ", false)
	if inputScr.ErrorMsg != "Base ref cannot be empty." {
		t.Fatalf("unexpected error: %q", inputScr.ErrorMsg)
	}

	// Test invalid base ref validation
	inputScr.OnSubmit("missing-ref", false)
	if inputScr.ErrorMsg != "Base ref not found." {
		t.Fatalf("unexpected error: %q", inputScr.ErrorMsg)
	}

	// Test valid base ref - should push branch name input
	inputScr.OnSubmit(repo.branch, false)
	// A new input screen should be pushed for branch name
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInput {
		t.Fatal("expected branch name input screen to be shown")
	}
	branchInputScr := m.state.ui.screenManager.Current().(*appscreen.InputScreen)
	if branchInputScr.Prompt != "Create worktree: branch name" {
		t.Fatalf("expected branch name prompt, got %q", branchInputScr.Prompt)
	}
}

func TestShowBranchSelection(t *testing.T) {
	repo := initTestRepo(t)
	withCwd(t, repo.dir)

	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40

	selected := ""
	cmd := m.showBranchSelection("Pick", "Filter...", "None", repo.branch, func(branch string) tea.Cmd {
		selected = branch
		return nil
	})

	if cmd == nil {
		t.Fatal("expected command to be returned")
	}

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeListSelect {
		t.Fatal("expected list screen to be active")
	}

	listScreen := m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)
	if len(listScreen.Items) == 0 {
		t.Fatal("expected branch list to be populated")
	}
	if listScreen.Items[0].ID != repo.branch {
		t.Fatalf("expected preferred branch first, got %q", listScreen.Items[0].ID)
	}

	remoteFound := false
	for _, item := range listScreen.Items {
		if item.Description == "remote" {
			remoteFound = true
			break
		}
	}
	if !remoteFound {
		t.Fatal("expected a remote branch entry")
	}

	listScreen.OnSelect(listScreen.Items[0])
	if selected != repo.branch {
		t.Fatalf("expected %q to be selected, got %q", repo.branch, selected)
	}
}

func TestShowCommitBaseBranchSelectionLocalOnly(t *testing.T) {
	repo := initTestRepo(t)
	withCwd(t, repo.dir)

	runGit(t, repo.dir, "checkout", "-b", "feature-one")
	runGit(t, repo.dir, "tag", "v1.0.0")
	runGit(t, repo.dir, "update-ref", "refs/remotes/origin/feature-one", "HEAD")

	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: repo.dir, Branch: "feature-one", IsMain: true},
	}

	cmd := m.showCommitBaseBranchSelection(mainWorktreeName)
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeListSelect {
		t.Fatal("expected list screen to be active")
	}

	listScreen := m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)
	selected, ok := listScreen.Selected()
	if !ok {
		t.Fatal("expected a selected branch")
	}
	if selected.ID != "feature-one" {
		t.Fatalf("expected current branch to be selected, got %q", selected.ID)
	}

	for _, item := range listScreen.Items {
		if item.Description != "" {
			t.Fatalf("expected local branch descriptions to be empty, got %q", item.Description)
		}
		if strings.HasPrefix(item.ID, "origin/") {
			t.Fatalf("unexpected remote branch in local list: %q", item.ID)
		}
		if item.ID == "v1.0.0" {
			t.Fatalf("unexpected tag in local list: %q", item.ID)
		}
	}
}

func TestShowCommitSelection(t *testing.T) {
	repo := initTestRepo(t)
	withCwd(t, repo.dir)

	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40

	cmd := m.showCommitSelection(repo.branch)
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeListSelect {
		t.Fatal("expected list screen to be active")
	}

	listScreen := m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)
	if len(listScreen.Items) == 0 {
		t.Fatal("expected commit list to be populated")
	}

	item := listScreen.Items[0]
	if item.Description == "" {
		t.Fatal("expected commit item to include date")
	}

	listScreen.OnSelect(item)
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInput {
		t.Fatal("expected branch name input to be shown")
	}
	inputScr := m.state.ui.screenManager.Current().(*appscreen.InputScreen)
	if inputScr.Prompt != "Create worktree: branch name" {
		t.Fatalf("expected branch name prompt, got %q", inputScr.Prompt)
	}

	expected := sanitizeBranchNameFromTitle(repo.commit.subject, repo.commit.shortHash)
	if got := inputScr.Input.Value(); got != expected {
		t.Fatalf("expected branch name %q, got %q", expected, got)
	}
}

func TestShowCommitSelectionShowsInfoOnBranchNameScriptError(t *testing.T) {
	repo := initTestRepo(t)
	withCwd(t, repo.dir)

	cfg := &config.AppConfig{
		WorktreeDir:      t.TempDir(),
		BranchNameScript: "false",
	}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40

	cmd := m.showCommitSelection(repo.branch)
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeListSelect {
		t.Fatal("expected list screen to be active")
	}

	listScreen := m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)
	if len(listScreen.Items) == 0 {
		t.Fatal("expected commit list to be populated")
	}

	item := listScreen.Items[0]
	if cmd := listScreen.OnSelect(item); cmd != nil {
		t.Fatal("expected nil command on script error")
	}

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInfo {
		t.Fatalf("expected info screen, got active=%v type=%v", m.state.ui.screenManager.IsActive(), m.state.ui.screenManager.Type())
	}
	infoScr := m.state.ui.screenManager.Current().(*appscreen.InfoScreen)
	if !strings.Contains(infoScr.Message, "Branch name script error") {
		t.Fatalf("expected branch name script error modal, got %q", infoScr.Message)
	}

	// After dismissing the info screen, we should be back at the list selection
	m.state.ui.screenManager.Pop()
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeListSelect {
		t.Fatalf("expected list screen after dismissing error, got %v", m.state.ui.screenManager.Type())
	}
}

func TestShowBranchNameInputValidation(t *testing.T) {
	repo := initTestRepo(t)
	withCwd(t, repo.dir)

	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.data.worktrees = []*models.WorktreeInfo{{Branch: "demo"}}

	m.showBranchNameInput(repo.branch, "demo")
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInput {
		t.Fatal("expected input screen to be active")
	}
	inputScr := m.state.ui.screenManager.Current().(*appscreen.InputScreen)

	if got := inputScr.Input.Value(); got != "demo-1" {
		t.Fatalf("expected suggested branch name, got %q", got)
	}

	// Test duplicate branch validation
	inputScr.OnSubmit("demo", false)
	if !strings.Contains(inputScr.ErrorMsg, "already exists") {
		t.Fatalf("unexpected error: %q", inputScr.ErrorMsg)
	}

	// Test existing path validation
	pathBranch := "path-branch"
	if err := os.MkdirAll(filepath.Join(m.getRepoWorktreeDir(), pathBranch), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	inputScr.OnSubmit(pathBranch, false)
	if !strings.Contains(inputScr.ErrorMsg, "Path already exists") {
		t.Fatalf("unexpected error: %q", inputScr.ErrorMsg)
	}
}

func TestSuggestBranchName(t *testing.T) {
	repo := initTestRepo(t)
	withCwd(t, repo.dir)

	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.data.worktrees = []*models.WorktreeInfo{{Branch: "demo"}}

	if got := m.suggestBranchName("demo"); got != "demo-1" {
		t.Fatalf("expected demo-1, got %q", got)
	}
}

func TestCommitLog(t *testing.T) {
	repo := initTestRepo(t)
	withCwd(t, repo.dir)

	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	log := m.commitLog(repo.commit.fullHash)
	if !strings.Contains(log, repo.commit.subject) {
		t.Fatalf("expected commit log to include subject, got %q", log)
	}
}

func TestCreateWorktreeFromBase(t *testing.T) {
	repo := initTestRepo(t)
	withCwd(t, repo.dir)

	worktreeDir := t.TempDir()
	cfg := &config.AppConfig{WorktreeDir: worktreeDir}
	m := NewModel(cfg, "")

	newBranch := "new-worktree"
	targetPath := filepath.Join(worktreeDir, newBranch)

	cmd := m.createWorktreeFromBase(newBranch, targetPath, repo.branch)
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}

	msg := cmd()
	loaded, ok := msg.(worktreesLoadedMsg)
	if !ok {
		t.Fatalf("expected worktreesLoadedMsg, got %T", msg)
	}

	if m.pendingSelectWorktreePath != targetPath {
		t.Fatalf("expected pendingSelectWorktreePath %q, got %q", targetPath, m.pendingSelectWorktreePath)
	}

	if loaded.err != nil {
		t.Fatalf("unexpected load error: %v", loaded.err)
	}

	if _, err := os.Stat(targetPath); err != nil {
		t.Fatalf("expected worktree path to exist: %v", err)
	}

	found := false
	for _, wt := range loaded.worktrees {
		if wt.Branch == newBranch {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected worktree for branch %s", newBranch)
	}
}

func TestClearListSelection(t *testing.T) {
	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	m.clearListSelection()
	if m.state.ui.screenManager.IsActive() {
		t.Fatalf("expected no screen, got %v", m.state.ui.screenManager.Type())
	}
}

func TestBuildCommitItems(t *testing.T) {
	items := buildCommitItems([]commitOption{{
		fullHash:  "full",
		shortHash: "short",
		date:      "2024-01-01",
		subject:   "subject",
	}})

	if len(items) != 1 {
		t.Fatalf("expected one item, got %d", len(items))
	}
	if items[0].label != "short subject" {
		t.Fatalf("unexpected label: %q", items[0].label)
	}
	if items[0].description != "2024-01-01" {
		t.Fatalf("unexpected description: %q", items[0].description)
	}
}

func TestShowBaseBranchForCustomCreateMenu(t *testing.T) {
	repo := initTestRepo(t)
	withCwd(t, repo.dir)

	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40

	menu := &config.CustomCreateMenu{
		Label:       "Test Menu",
		Description: "Test Description",
		Command:     "echo test-branch",
		Interactive: false,
	}

	cmd := m.showBaseBranchForCustomCreateMenu(menu)
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeListSelect {
		t.Fatal("expected list screen to be set")
	}

	listScreen := m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)

	if len(listScreen.Items) > 0 {
		listScreen.OnSelect(listScreen.Items[0])
		if m.pending.CustomBaseRef == "" {
			t.Error("expected pendingCustomBaseRef to be set")
		}
		if m.pending.CustomMenu == nil {
			t.Error("expected pendingCustomMenu to be set")
		}
	}
}

func TestExecuteCustomCreateCommand(t *testing.T) {
	repo := initTestRepo(t)
	withCwd(t, repo.dir)

	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: repo.dir, Branch: repo.branch, IsMain: true},
	}

	menu := &config.CustomCreateMenu{
		Label:       "Test Menu",
		Description: "Test Description",
		Command:     "echo test-branch-name",
		Interactive: false,
	}

	cmd := m.executeCustomCreateCommand(menu)
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}
	if m.state.ui.screenManager.Type() != appscreen.TypeLoading {
		t.Errorf("expected loading screen, got %v", m.state.ui.screenManager.Type())
	}
	if m.loadingScreen() == nil {
		t.Error("expected loading screen to be set")
	}

	// Execute the command
	msg := cmd()
	result, ok := msg.(customCreateResultMsg)
	if !ok {
		t.Fatalf("expected customCreateResultMsg, got %T", msg)
	}
	if result.err != nil {
		t.Fatalf("unexpected error: %v", result.err)
	}
	if result.branchName != "test-branch-name" {
		t.Errorf("expected branch name 'test-branch-name', got %q", result.branchName)
	}
}

func TestExecuteCustomCreateCommandError(t *testing.T) {
	repo := initTestRepo(t)
	withCwd(t, repo.dir)

	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: repo.dir, Branch: repo.branch, IsMain: true},
	}

	menu := &config.CustomCreateMenu{
		Label:       "Test Menu",
		Description: "Test Description",
		Command:     "false", // Command that fails
		Interactive: false,
	}

	cmd := m.executeCustomCreateCommand(menu)
	msg := cmd()
	result, ok := msg.(customCreateResultMsg)
	if !ok {
		t.Fatalf("expected customCreateResultMsg, got %T", msg)
	}
	if result.err == nil {
		t.Error("expected error from failing command")
	}
}

func TestExecuteCustomCreateCommandNoOutput(t *testing.T) {
	repo := initTestRepo(t)
	withCwd(t, repo.dir)

	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: repo.dir, Branch: repo.branch, IsMain: true},
	}

	menu := &config.CustomCreateMenu{
		Label:       "Test Menu",
		Description: "Test Description",
		Command:     "true", // Command that succeeds but produces no output
		Interactive: false,
	}

	cmd := m.executeCustomCreateCommand(menu)
	msg := cmd()
	result, ok := msg.(customCreateResultMsg)
	if !ok {
		t.Fatalf("expected customCreateResultMsg, got %T", msg)
	}
	if result.err == nil {
		t.Error("expected error for command with no output")
	}
}

func TestExecuteCustomPostCommand(t *testing.T) {
	repo := initTestRepo(t)
	withCwd(t, repo.dir)

	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	targetPath := t.TempDir()
	env := map[string]string{
		"WORKTREE_PATH": targetPath,
		"TEST_VAR":      "test-value",
	}

	cmd := m.executeCustomPostCommand("echo success", targetPath, env)
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}
	if m.state.ui.screenManager.Type() != appscreen.TypeLoading {
		t.Errorf("expected loading screen, got %v", m.state.ui.screenManager.Type())
	}
	if m.loadingScreen() == nil {
		t.Error("expected loading screen to be set")
	}

	// Execute the command
	msg := cmd()
	result, ok := msg.(customPostCommandResultMsg)
	if !ok {
		t.Fatalf("expected customPostCommandResultMsg, got %T", msg)
	}
	if result.err != nil {
		t.Fatalf("unexpected error: %v", result.err)
	}
}

func TestExecuteCustomPostCommandError(t *testing.T) {
	repo := initTestRepo(t)
	withCwd(t, repo.dir)

	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	targetPath := t.TempDir()
	env := map[string]string{}

	cmd := m.executeCustomPostCommand("false", targetPath, env)
	msg := cmd()
	result, ok := msg.(customPostCommandResultMsg)
	if !ok {
		t.Fatalf("expected customPostCommandResultMsg, got %T", msg)
	}
	if result.err == nil {
		t.Error("expected error from failing command")
	}
}

func TestLocalBranchExists(t *testing.T) {
	repo := initTestRepo(t)
	withCwd(t, repo.dir)

	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")

	// The repo has a 'main' or 'master' branch and 'feature' branch
	if !m.localBranchExists(repo.branch) {
		t.Fatalf("expected %s to exist as local branch", repo.branch)
	}

	if !m.localBranchExists(featureBranch) {
		t.Fatalf("expected %s to exist as local branch", featureBranch)
	}

	if m.localBranchExists("nonexistent-branch") {
		t.Fatal("expected nonexistent-branch to not exist")
	}

	// Remote branches should not be detected as local
	if m.localBranchExists("origin/" + repo.branch) {
		t.Fatal("expected remote branch to not be detected as local")
	}
}

func TestShowCheckoutOrCreatePrompt(t *testing.T) {
	repo := initTestRepo(t)
	withCwd(t, repo.dir)

	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40

	cmd := m.showCheckoutOrCreatePrompt(repo.branch)
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeListSelect {
		t.Fatal("expected list screen to be active")
	}

	listScreen := m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)

	if len(listScreen.Items) != 2 {
		t.Fatalf("expected 2 options, got %d", len(listScreen.Items))
	}

	checkoutFound := false
	createFound := false
	for _, item := range listScreen.Items {
		if item.ID == "checkout" {
			checkoutFound = true
			if item.Label != "Checkout existing branch" {
				t.Fatalf("expected label 'Checkout existing branch', got %q", item.Label)
			}
		}
		if item.ID == "create" {
			createFound = true
			if item.Label != "Create new branch" {
				t.Fatalf("expected label 'Create new branch', got %q", item.Label)
			}
		}
	}
	if !checkoutFound {
		t.Fatal("expected 'checkout' option in prompt")
	}
	if !createFound {
		t.Fatal("expected 'create' option in prompt")
	}

	listScreen.OnSelect(appscreen.SelectionItem{ID: "create"})
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInput {
		t.Fatal("expected input screen for branch name")
	}
	inputScr := m.state.ui.screenManager.Current().(*appscreen.InputScreen)
	if inputScr.Prompt != "Create worktree: branch name" {
		t.Fatalf("expected branch name prompt, got %q", inputScr.Prompt)
	}
}

func TestShowWorktreeNameForExistingBranch(t *testing.T) {
	repo := initTestRepo(t)
	withCwd(t, repo.dir)

	worktreeDir := t.TempDir()
	cfg := &config.AppConfig{WorktreeDir: worktreeDir}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40

	// Show the worktree name input for an existing branch
	cmd := m.showWorktreeNameForExistingBranch(featureBranch)
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInput {
		t.Fatal("expected input screen to be active")
	}
	inputScr := m.state.ui.screenManager.Current().(*appscreen.InputScreen)

	// Verify suggested name format
	expected := featureBranch + "-wt"
	if got := inputScr.Input.Value(); got != expected {
		t.Fatalf("expected suggested name %q, got %q", expected, got)
	}

	// Verify prompt mentions existing branch
	if !strings.Contains(inputScr.Prompt, featureBranch) {
		t.Fatalf("expected prompt to mention branch name, got %q", inputScr.Prompt)
	}

	// Test path conflict validation - create the target path first
	pathBranch := "path-branch"
	if err := os.MkdirAll(filepath.Join(m.getRepoWorktreeDir(), pathBranch), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	inputScr.OnSubmit(pathBranch, false)
	if !strings.Contains(inputScr.ErrorMsg, "Path already exists") {
		t.Fatalf("unexpected error: %q", inputScr.ErrorMsg)
	}
}

func TestCheckoutExistingBranchAsync(t *testing.T) {
	repo := initTestRepo(t)
	withCwd(t, repo.dir)

	worktreeDir := t.TempDir()
	cfg := &config.AppConfig{WorktreeDir: worktreeDir}
	m := NewModel(cfg, "")

	// Use the feature branch that was created in initTestRepo
	worktreeName := "feature-wt"
	targetPath := filepath.Join(worktreeDir, worktreeName)

	cmd := m.checkoutExistingBranchAsync(worktreeName, targetPath, featureBranch)
	if cmd == nil {
		t.Fatal("expected command to be returned")
	}

	msg := cmd()
	loaded, ok := msg.(worktreesLoadedMsg)
	if !ok {
		t.Fatalf("expected worktreesLoadedMsg, got %T", msg)
	}

	if loaded.err != nil {
		t.Fatalf("unexpected error: %v", loaded.err)
	}

	// Verify worktree path exists
	if _, err := os.Stat(targetPath); err != nil {
		t.Fatalf("expected worktree path to exist: %v", err)
	}

	// Verify the branch is checked out (not a new branch created)
	branch := runGit(t, targetPath, "branch", "--show-current")
	if branch != featureBranch {
		t.Fatalf("expected branch %s, got %s", featureBranch, branch)
	}

	// Verify no new branch was created with the worktree name
	allBranches := runGit(t, repo.dir, "branch", "-a")
	if strings.Contains(allBranches, worktreeName) {
		t.Fatalf("unexpected branch %s created; got branches: %s", worktreeName, allBranches)
	}
}

func TestBranchSelectionWithLocalBranch(t *testing.T) {
	repo := initTestRepo(t)
	withCwd(t, repo.dir)

	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40

	m.showBaseSelection(repo.branch)

	listScreen := m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)
	cmd := listScreen.OnSelect(appscreen.SelectionItem{ID: "branch-list"})
	if cmd == nil {
		t.Fatal("expected command from branch-list selection")
	}

	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeListSelect {
		t.Fatal("expected branch list screen")
	}

	listScreen = m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)

	var featureItem appscreen.SelectionItem
	featureFound := false
	for i := range listScreen.Items {
		if listScreen.Items[i].ID == featureBranch {
			featureItem = listScreen.Items[i]
			featureFound = true
			break
		}
	}
	if !featureFound {
		t.Fatalf("expected to find %s in branch list", featureBranch)
	}

	listScreen.OnSelect(featureItem)
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeListSelect {
		t.Fatal("expected checkout/create prompt screen")
	}

	listScreen = m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)
	checkoutFound := false
	for _, item := range listScreen.Items {
		if item.ID == "checkout" {
			checkoutFound = true
			break
		}
	}
	if !checkoutFound {
		t.Fatal("expected checkout option in prompt for local branch")
	}
}

func TestBranchSelectionWithSlashLocalBranch(t *testing.T) {
	repo := initTestRepo(t)
	withCwd(t, repo.dir)

	branchWithSlash := "topic/with-slash"
	runGit(t, repo.dir, "branch", branchWithSlash)

	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40

	m.showBaseSelection(repo.branch)

	listScreen := m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)
	cmd := listScreen.OnSelect(appscreen.SelectionItem{ID: "branch-list"})
	if cmd == nil {
		t.Fatal("expected command from branch-list selection")
	}

	listScreen = m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)

	var slashItem appscreen.SelectionItem
	slashFound := false
	for i := range listScreen.Items {
		if listScreen.Items[i].ID == branchWithSlash {
			slashItem = listScreen.Items[i]
			slashFound = true
			break
		}
	}
	if !slashFound {
		t.Fatalf("expected to find %s in branch list", branchWithSlash)
	}

	listScreen.OnSelect(slashItem)
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeListSelect {
		t.Fatal("expected checkout/create prompt screen")
	}

	listScreen = m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)
	checkoutFound := false
	for _, item := range listScreen.Items {
		if item.ID == "checkout" {
			checkoutFound = true
			break
		}
	}
	if !checkoutFound {
		t.Fatal("expected checkout option in prompt for local branch with slash")
	}
}

func TestBranchSelectionSkipsCheckoutForCheckedOutBranch(t *testing.T) {
	repo := initTestRepo(t)
	withCwd(t, repo.dir)

	cfg := &config.AppConfig{WorktreeDir: t.TempDir()}
	m := NewModel(cfg, "")
	m.state.view.WindowWidth = 120
	m.state.view.WindowHeight = 40
	m.state.data.worktrees = []*models.WorktreeInfo{
		{Path: repo.dir, Branch: repo.branch},
	}

	m.showBaseSelection(repo.branch)

	listScreen := m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)
	cmd := listScreen.OnSelect(appscreen.SelectionItem{ID: "branch-list"})
	if cmd == nil {
		t.Fatal("expected command from branch-list selection")
	}

	listScreen = m.state.ui.screenManager.Current().(*appscreen.ListSelectionScreen)

	var mainItem appscreen.SelectionItem
	mainFound := false
	for i := range listScreen.Items {
		if listScreen.Items[i].ID == repo.branch {
			mainItem = listScreen.Items[i]
			mainFound = true
			break
		}
	}
	if !mainFound {
		t.Fatalf("expected to find %s in branch list", repo.branch)
	}

	listScreen.OnSelect(mainItem)
	if !m.state.ui.screenManager.IsActive() || m.state.ui.screenManager.Type() != appscreen.TypeInput {
		t.Fatal("expected input screen for branch name")
	}
	inputScr := m.state.ui.screenManager.Current().(*appscreen.InputScreen)
	if inputScr.Prompt != "Create worktree: branch name" {
		t.Fatalf("expected branch name prompt, got %q", inputScr.Prompt)
	}
}
