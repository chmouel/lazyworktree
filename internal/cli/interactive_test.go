package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	lwlog "github.com/chmouel/lazyworktree/internal/log"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleIssues() []*models.IssueInfo {
	return []*models.IssueInfo{
		{Number: 10, Title: "Fix login bug", Body: "The login page crashes on submit."},
		{Number: 42, Title: "Add dark mode", Body: "Support dark theme across the UI."},
		{Number: 99, Title: "Improve performance", Body: ""},
	}
}

func TestSelectIssueWithPrompt_ValidSelection(t *testing.T) {
	issues := sampleIssues()
	stdin := strings.NewReader("2\n")
	stderr := &bytes.Buffer{}

	selected, err := selectIssueWithPrompt(issues, nil, "", stdin, stderr)
	require.NoError(t, err)
	assert.Equal(t, 42, selected.Number)
	assert.Equal(t, "Add dark mode", selected.Title)

	output := stderr.String()
	assert.Contains(t, output, "Open issues:")
	assert.Contains(t, output, "[1] #10")
	assert.Contains(t, output, "[2] #42")
	assert.Contains(t, output, "[3] #99")
	assert.Contains(t, output, "Select issue [1-3]:")
}

func TestSelectIssueWithPrompt_FirstItem(t *testing.T) {
	issues := sampleIssues()
	stdin := strings.NewReader("1\n")
	stderr := &bytes.Buffer{}

	selected, err := selectIssueWithPrompt(issues, nil, "", stdin, stderr)
	require.NoError(t, err)
	assert.Equal(t, 10, selected.Number)
}

func TestSelectIssueWithPrompt_LastItem(t *testing.T) {
	issues := sampleIssues()
	stdin := strings.NewReader("3\n")
	stderr := &bytes.Buffer{}

	selected, err := selectIssueWithPrompt(issues, nil, "", stdin, stderr)
	require.NoError(t, err)
	assert.Equal(t, 99, selected.Number)
}

func TestSelectIssueWithPrompt_OutOfRangeTooHigh(t *testing.T) {
	issues := sampleIssues()
	stdin := strings.NewReader("5\n")
	stderr := &bytes.Buffer{}

	_, err := selectIssueWithPrompt(issues, nil, "", stdin, stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "selection out of range")
}

func TestSelectIssueWithPrompt_OutOfRangeZero(t *testing.T) {
	issues := sampleIssues()
	stdin := strings.NewReader("0\n")
	stderr := &bytes.Buffer{}

	_, err := selectIssueWithPrompt(issues, nil, "", stdin, stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "selection out of range")
}

func TestSelectIssueWithPrompt_NegativeNumber(t *testing.T) {
	issues := sampleIssues()
	stdin := strings.NewReader("-1\n")
	stderr := &bytes.Buffer{}

	_, err := selectIssueWithPrompt(issues, nil, "", stdin, stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "selection out of range")
}

func TestSelectIssueWithPrompt_NonNumeric(t *testing.T) {
	issues := sampleIssues()
	stdin := strings.NewReader("abc\n")
	stderr := &bytes.Buffer{}

	_, err := selectIssueWithPrompt(issues, nil, "", stdin, stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid selection")
}

func TestSelectIssueWithPrompt_EmptyInput(t *testing.T) {
	issues := sampleIssues()
	stdin := strings.NewReader("\n")
	stderr := &bytes.Buffer{}

	_, err := selectIssueWithPrompt(issues, nil, "", stdin, stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no issue selected")
}

func TestSelectIssueWithPrompt_EOF(t *testing.T) {
	issues := sampleIssues()
	stdin := strings.NewReader("") // EOF immediately
	stderr := &bytes.Buffer{}

	_, err := selectIssueWithPrompt(issues, nil, "", stdin, stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
}

func TestParseIssueNumberFromLine(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		want    int
		wantErr bool
	}{
		{name: "standard format", line: "#42     Add dark mode", want: 42},
		{name: "single digit", line: "#1      Fix bug", want: 1},
		{name: "large number", line: "#12345  Feature request", want: 12345},
		{name: "no space padding", line: "#7 Quick fix", want: 7},
		{name: "leading whitespace", line: "  #99   Improve performance", want: 99},
		{name: "no hash prefix", line: "42 Add dark mode", wantErr: true},
		{name: "empty after hash", line: "#", wantErr: true},
		{name: "non-numeric after hash", line: "#abc Fix bug", wantErr: true},
		{name: "empty string", line: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseIssueNumberFromLine(tt.line)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestBuildPreviewScript(t *testing.T) {
	issues := sampleIssues()
	script := buildPreviewScript(issues)

	assert.Contains(t, script, "10)")
	assert.Contains(t, script, "42)")
	assert.Contains(t, script, "99)")
	assert.Contains(t, script, "(no description)")
	assert.Contains(t, script, "The login page crashes on submit.")
	assert.Contains(t, script, "Support dark theme across the UI.")
}

func TestBuildPreviewScript_SingleQuoteEscaping(t *testing.T) {
	issues := []*models.IssueInfo{
		{Number: 1, Title: "Test", Body: "It's a bug that can't be fixed"},
	}
	script := buildPreviewScript(issues)

	assert.Contains(t, script, "It'\\''s a bug that can'\\''t be fixed")
}

type mockGitServiceForInteractive struct {
	issues    []*models.IssueInfo
	err       error
	prs       []*models.PRInfo
	prsErr    error
	worktrees []*models.WorktreeInfo
	wtErr     error
}

func (m *mockGitServiceForInteractive) FetchAllOpenIssues(_ context.Context) ([]*models.IssueInfo, error) {
	return m.issues, m.err
}

func (m *mockGitServiceForInteractive) CheckoutPRBranch(context.Context, int, string, string) bool {
	return false
}

func (m *mockGitServiceForInteractive) CreateWorktreeFromPR(context.Context, int, string, string, string) bool {
	return false
}

func (m *mockGitServiceForInteractive) ExecuteCommands(context.Context, []string, string, map[string]string) error {
	return nil
}

func (m *mockGitServiceForInteractive) FetchAllOpenPRs(_ context.Context) ([]*models.PRInfo, error) {
	return m.prs, m.prsErr
}

func (m *mockGitServiceForInteractive) FetchIssue(_ context.Context, issueNumber int) (*models.IssueInfo, error) {
	for _, issue := range m.issues {
		if issue.Number == issueNumber {
			return issue, nil
		}
	}
	if m.err != nil {
		return nil, m.err
	}
	return nil, fmt.Errorf("issue #%d not found", issueNumber)
}

func (m *mockGitServiceForInteractive) FetchPR(_ context.Context, prNumber int) (*models.PRInfo, error) {
	for _, pr := range m.prs {
		if pr.Number == prNumber {
			return pr, nil
		}
	}
	if m.prsErr != nil {
		return nil, m.prsErr
	}
	return nil, fmt.Errorf("PR #%d not found", prNumber)
}

func (m *mockGitServiceForInteractive) FetchPRForWorktreeWithError(context.Context, string) (*models.PRInfo, error) {
	return nil, nil
}

func (m *mockGitServiceForInteractive) GetCurrentBranch(context.Context) (string, error) {
	return "main", nil
}
func (m *mockGitServiceForInteractive) GetMainWorktreePath(context.Context) string { return "" }
func (m *mockGitServiceForInteractive) GetWorktrees(context.Context) ([]*models.WorktreeInfo, error) {
	return m.worktrees, m.wtErr
}

func (m *mockGitServiceForInteractive) RenameWorktree(context.Context, string, string, string, string) bool {
	return true
}
func (m *mockGitServiceForInteractive) ResolveRepoName(context.Context) string { return "repo" }
func (m *mockGitServiceForInteractive) RunCommandChecked(context.Context, []string, string, string) bool {
	return true
}

func (m *mockGitServiceForInteractive) RunCommandQuiet(context.Context, []string, string) bool {
	return true
}

func (m *mockGitServiceForInteractive) RunGit(context.Context, []string, string, []int, bool, bool) string {
	return ""
}

func (m *mockGitServiceForInteractive) RunGitWithCombinedOutput(context.Context, []string, string, map[string]string) ([]byte, error) {
	return nil, nil
}

func TestSelectIssueInteractive_NoIssues(t *testing.T) {
	gitSvc := &mockGitServiceForInteractive{issues: []*models.IssueInfo{}}
	stderr := &bytes.Buffer{}

	_, err := SelectIssueInteractive(context.Background(), gitSvc, "", "", strings.NewReader(""), stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no open issues found")
}

func TestSelectIssueInteractive_FetchError(t *testing.T) {
	gitSvc := &mockGitServiceForInteractive{err: assert.AnError}
	stderr := &bytes.Buffer{}

	_, err := SelectIssueInteractive(context.Background(), gitSvc, "", "", strings.NewReader(""), stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch issues")
}

func TestSelectIssueInteractive_UsesPromptFallback(t *testing.T) {
	oldFunc := selectIssueFunc
	t.Cleanup(func() { selectIssueFunc = oldFunc })

	selectIssueFunc = selectIssueWithPrompt

	gitSvc := &mockGitServiceForInteractive{issues: sampleIssues()}
	stderr := &bytes.Buffer{}

	num, err := SelectIssueInteractive(context.Background(), gitSvc, "", "", strings.NewReader("2\n"), stderr)
	require.NoError(t, err)
	assert.Equal(t, 42, num)
}

func TestSelectIssueDefault_FallsBackToPromptWhenNoFzf(t *testing.T) {
	oldLookPath := fzfLookPath
	t.Cleanup(func() { fzfLookPath = oldLookPath })
	fzfLookPath = func(name string) (string, error) {
		return "", exec.ErrNotFound
	}

	issues := sampleIssues()
	stdin := strings.NewReader("1\n")
	stderr := &bytes.Buffer{}

	selected, err := selectIssueDefault(issues, nil, "", stdin, stderr)
	require.NoError(t, err)
	assert.Equal(t, 10, selected.Number)
}

func TestSelectIssueInteractive_FormattedLinesParseable(t *testing.T) {
	issues := sampleIssues()
	for _, issue := range issues {
		line := fmt.Sprintf("#%-6d %s", issue.Number, issue.Title)
		num, err := parseIssueNumberFromLine(line)
		require.NoError(t, err, "failed to parse line: %q", line)
		assert.Equal(t, issue.Number, num)
	}
}

func TestSelectIssueWithFzf_Integration(t *testing.T) {
	if _, err := exec.LookPath("fzf"); err != nil {
		t.Skip("fzf not installed, skipping integration test")
	}

	issues := sampleIssues()

	var lines []string
	for _, issue := range issues {
		lines = append(lines, fmt.Sprintf("#%-6d %s", issue.Number, issue.Title))
	}
	input := strings.Join(lines, "\n")

	// Filter for "dark" should match issue #42 "Add dark mode"
	cmd := exec.Command("fzf", "--filter", "dark")
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.Output()
	require.NoError(t, err, "fzf --filter failed")

	firstLine := strings.Split(strings.TrimSpace(string(out)), "\n")[0]
	num, err := parseIssueNumberFromLine(firstLine)
	require.NoError(t, err)
	assert.Equal(t, 42, num)
}

func samplePRs() []*models.PRInfo {
	return []*models.PRInfo{
		{Number: 10, Title: "Fix login bug", Body: "The login page crashes.", Author: "alice", Branch: "fix-login", BaseBranch: "main", CIStatus: "success"},
		{Number: 42, Title: "Add dark mode", Body: "Support dark theme.", Author: "bob", Branch: "dark-mode", BaseBranch: "main", IsDraft: true, CIStatus: "pending"},
		{Number: 99, Title: "Improve performance", Body: "", Author: "charlie", Branch: "perf", BaseBranch: "develop", CIStatus: "none"},
	}
}

func TestSelectPRWithPrompt_ValidSelection(t *testing.T) {
	prs := samplePRs()
	stdin := strings.NewReader("2\n")
	stderr := &bytes.Buffer{}

	selected, err := selectPRWithPrompt(prs, nil, "", stdin, stderr)
	require.NoError(t, err)
	assert.Equal(t, 42, selected.Number)
	assert.Equal(t, "Add dark mode", selected.Title)

	output := stderr.String()
	assert.Contains(t, output, "Open pull requests:")
	assert.Contains(t, output, "[1] #10")
	assert.Contains(t, output, "[2] #42")
	assert.Contains(t, output, "[3] #99")
	assert.Contains(t, output, "Select pull request [1-3]:")
}

func TestSelectPRWithPrompt_FirstItem(t *testing.T) {
	prs := samplePRs()
	stdin := strings.NewReader("1\n")
	stderr := &bytes.Buffer{}

	selected, err := selectPRWithPrompt(prs, nil, "", stdin, stderr)
	require.NoError(t, err)
	assert.Equal(t, 10, selected.Number)
}

func TestSelectPRWithPrompt_LastItem(t *testing.T) {
	prs := samplePRs()
	stdin := strings.NewReader("3\n")
	stderr := &bytes.Buffer{}

	selected, err := selectPRWithPrompt(prs, nil, "", stdin, stderr)
	require.NoError(t, err)
	assert.Equal(t, 99, selected.Number)
}

func TestSelectPRWithPrompt_OutOfRangeTooHigh(t *testing.T) {
	prs := samplePRs()
	stdin := strings.NewReader("5\n")
	stderr := &bytes.Buffer{}

	_, err := selectPRWithPrompt(prs, nil, "", stdin, stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "selection out of range")
}

func TestSelectPRWithPrompt_OutOfRangeZero(t *testing.T) {
	prs := samplePRs()
	stdin := strings.NewReader("0\n")
	stderr := &bytes.Buffer{}

	_, err := selectPRWithPrompt(prs, nil, "", stdin, stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "selection out of range")
}

func TestSelectPRWithPrompt_NonNumeric(t *testing.T) {
	prs := samplePRs()
	stdin := strings.NewReader("abc\n")
	stderr := &bytes.Buffer{}

	_, err := selectPRWithPrompt(prs, nil, "", stdin, stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid selection")
}

func TestSelectPRWithPrompt_EmptyInput(t *testing.T) {
	prs := samplePRs()
	stdin := strings.NewReader("\n")
	stderr := &bytes.Buffer{}

	_, err := selectPRWithPrompt(prs, nil, "", stdin, stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no pull request selected")
}

func TestSelectPRWithPrompt_EOF(t *testing.T) {
	prs := samplePRs()
	stdin := strings.NewReader("")
	stderr := &bytes.Buffer{}

	_, err := selectPRWithPrompt(prs, nil, "", stdin, stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
}

func TestSelectPRWithPrompt_DraftAndCITags(t *testing.T) {
	prs := samplePRs()
	stdin := strings.NewReader("2\n")
	stderr := &bytes.Buffer{}

	_, err := selectPRWithPrompt(prs, nil, "", stdin, stderr)
	require.NoError(t, err)

	output := stderr.String()
	assert.Contains(t, output, "[draft]")
	assert.Contains(t, output, "[CI: pending]")
}

func TestBuildPRPreviewScript(t *testing.T) {
	prs := samplePRs()
	script := buildPRPreviewScript(prs)

	assert.Contains(t, script, "10)")
	assert.Contains(t, script, "42)")
	assert.Contains(t, script, "99)")
	assert.Contains(t, script, "Author: alice")
	assert.Contains(t, script, "Branch: fix-login -> main")
	assert.Contains(t, script, "Status: Draft")
	assert.Contains(t, script, "CI: success")
	assert.Contains(t, script, "CI: pending")
	assert.Contains(t, script, "(no description)")
	assert.Contains(t, script, "The login page crashes.")
	assert.Contains(t, script, "Support dark theme.")
}

func TestBuildPRPreviewScript_SingleQuoteEscaping(t *testing.T) {
	prs := []*models.PRInfo{
		{Number: 1, Title: "Test", Body: "It's a bug that can't be fixed", Author: "dev", Branch: "fix", BaseBranch: "main"},
	}
	script := buildPRPreviewScript(prs)

	assert.Contains(t, script, "It'\\''s a bug that can'\\''t be fixed")
}

func TestPrepareGenericPreviewCommand_UsesTempFiles(t *testing.T) {
	longBody := strings.Repeat("body ", 4096)
	items := wrapPRs([]*models.PRInfo{
		{Number: 42, Title: "Large PR", Body: longBody, Author: "dev", Branch: "feature", BaseBranch: "main"},
	}, nil)

	command, cleanup, err := prepareGenericPreviewCommand(items)
	require.NoError(t, err)
	defer cleanup()

	assert.Less(t, len(command), 500)
	assert.NotContains(t, command, longBody)
	assert.Contains(t, command, "lazyworktree-fzf-preview-")
	assert.Contains(t, command, "cat \"$file\"")
}

func TestSelectPRInteractive_NoPRs(t *testing.T) {
	gitSvc := &mockGitServiceForInteractive{prs: []*models.PRInfo{}}
	stderr := &bytes.Buffer{}

	_, err := SelectPRInteractive(context.Background(), gitSvc, "", strings.NewReader(""), stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no open pull requests found")
}

func TestSelectPRInteractive_FetchError(t *testing.T) {
	gitSvc := &mockGitServiceForInteractive{prsErr: assert.AnError}
	stderr := &bytes.Buffer{}

	_, err := SelectPRInteractive(context.Background(), gitSvc, "", strings.NewReader(""), stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch pull requests")
}

func TestSelectPRInteractive_UsesPromptFallback(t *testing.T) {
	oldFunc := selectPRFunc
	t.Cleanup(func() { selectPRFunc = oldFunc })

	selectPRFunc = selectPRWithPrompt

	gitSvc := &mockGitServiceForInteractive{prs: samplePRs()}
	stderr := &bytes.Buffer{}

	num, err := SelectPRInteractive(context.Background(), gitSvc, "", strings.NewReader("2\n"), stderr)
	require.NoError(t, err)
	assert.Equal(t, 42, num)
}

func TestSelectPRInteractive_LogsFetchCountAndSelection(t *testing.T) {
	oldFunc := selectPRFunc
	t.Cleanup(func() { selectPRFunc = oldFunc })
	selectPRFunc = func(prs []*models.PRInfo, _ map[int]bool, _ string, _ io.Reader, _ io.Writer) (*models.PRInfo, error) {
		return prs[1], nil
	}

	logFile := filepath.Join(t.TempDir(), "pr-selector.log")
	require.NoError(t, lwlog.SetFile(logFile))
	t.Cleanup(func() {
		_ = lwlog.Close()
		_ = lwlog.SetFile("")
	})

	gitSvc := &mockGitServiceForInteractive{prs: samplePRs()}
	stderr := &bytes.Buffer{}

	num, err := SelectPRInteractive(context.Background(), gitSvc, "dark", strings.NewReader(""), stderr)
	require.NoError(t, err)
	assert.Equal(t, 42, num)
	require.NoError(t, lwlog.Close())

	// #nosec G304 - test path comes from t.TempDir().
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), `pr selector: fetching open pull requests query="dark"`)
	assert.Contains(t, string(content), "pr selector: fetched 3 open pull requests")
	assert.Contains(t, string(content), "pr selector: selected PR #42")
}

func TestSelectPRDefault_FallsBackToPromptWhenNoFzf(t *testing.T) {
	oldLookPath := fzfLookPath
	t.Cleanup(func() { fzfLookPath = oldLookPath })

	fzfLookPath = func(name string) (string, error) {
		return "", exec.ErrNotFound
	}

	prs := samplePRs()
	stdin := strings.NewReader("1\n")
	stderr := &bytes.Buffer{}

	selected, err := selectPRDefault(prs, nil, "", stdin, stderr)
	require.NoError(t, err)
	assert.Equal(t, 10, selected.Number)
}

func TestSelectPRDefault_LogsPromptFallback(t *testing.T) {
	oldLookPath := fzfLookPath
	t.Cleanup(func() { fzfLookPath = oldLookPath })
	fzfLookPath = func(name string) (string, error) {
		return "", exec.ErrNotFound
	}

	logFile := filepath.Join(t.TempDir(), "prompt-fallback.log")
	require.NoError(t, lwlog.SetFile(logFile))
	t.Cleanup(func() {
		_ = lwlog.Close()
		_ = lwlog.SetFile("")
	})

	selected, err := selectPRDefault(samplePRs(), nil, "", strings.NewReader("1\n"), &bytes.Buffer{})
	require.NoError(t, err)
	assert.Equal(t, 10, selected.Number)
	require.NoError(t, lwlog.Close())

	// #nosec G304 - test path comes from t.TempDir().
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "pr selector: fzf unavailable")
	assert.Contains(t, string(content), `selector: using numbered prompt noun="pull request" items=3 filtered=3 query=""`)
}

func TestSelectPRWithFzf_LogsExitError(t *testing.T) {
	tmpDir := t.TempDir()
	fzfPath := filepath.Join(tmpDir, "fzf")
	script := "#!/bin/sh\necho fzf boom >&2\nexit 2\n"
	require.NoError(t, os.WriteFile(fzfPath, []byte(script), 0o700))
	t.Setenv("PATH", tmpDir)

	logFile := filepath.Join(t.TempDir(), "fzf-error.log")
	require.NoError(t, lwlog.SetFile(logFile))
	t.Cleanup(func() {
		_ = lwlog.Close()
		_ = lwlog.SetFile("")
	})

	stderr := &bytes.Buffer{}
	_, err := selectPRWithFzf(samplePRs(), nil, "", stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pull request selection cancelled")
	assert.Contains(t, stderr.String(), "fzf boom")
	require.NoError(t, lwlog.Close())

	// #nosec G304 - test path comes from t.TempDir().
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), `selector: invoking fzf prompt="Select PR> " items=3 query=""`)
	assert.Contains(t, string(content), `selector: fzf failed prompt="Select PR> " exit=2 stderr="fzf boom"`)
}

func TestSelectPRInteractive_FormattedLinesParseable(t *testing.T) {
	prs := samplePRs()
	for _, pr := range prs {
		line := fmt.Sprintf("#%-6d %-12s %s", pr.Number, pr.Author, pr.Title)
		num, err := parseIssueNumberFromLine(line)
		require.NoError(t, err, "failed to parse line: %q", line)
		assert.Equal(t, pr.Number, num)
	}
}

func TestSelectPRWithFzf_Integration(t *testing.T) {
	if _, err := exec.LookPath("fzf"); err != nil {
		t.Skip("fzf not installed, skipping integration test")
	}

	prs := samplePRs()

	var lines []string
	for _, pr := range prs {
		lines = append(lines, fmt.Sprintf("#%-6d %-12s %s", pr.Number, pr.Author, pr.Title))
	}
	input := strings.Join(lines, "\n")

	// Filter for "dark" should match PR #42 "Add dark mode"
	cmd := exec.Command("fzf", "--filter", "dark")
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.Output()
	require.NoError(t, err, "fzf --filter failed")

	firstLine := strings.Split(strings.TrimSpace(string(out)), "\n")[0]
	num, err := parseIssueNumberFromLine(firstLine)
	require.NoError(t, err)
	assert.Equal(t, 42, num)
}

// --- Query filtering tests ---

func TestFilterItems_Issues(t *testing.T) {
	items := wrapIssues(sampleIssues(), nil)

	tests := []struct {
		name      string
		query     string
		wantCount int
		wantFirst int
	}{
		{name: "empty query returns all", query: "", wantCount: 3, wantFirst: 10},
		{name: "match one", query: "dark", wantCount: 1, wantFirst: 42},
		{name: "case insensitive", query: "DARK", wantCount: 1, wantFirst: 42},
		{name: "match none", query: "nonexistent", wantCount: 0},
		{name: "match by number", query: "#99", wantCount: 1, wantFirst: 99},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterItems(items, tt.query)
			assert.Len(t, result, tt.wantCount)
			if tt.wantCount > 0 {
				assert.Equal(t, tt.wantFirst, result[0].ItemNumber())
			}
		})
	}
}

func TestFilterItems_PRs(t *testing.T) {
	items := wrapPRs(samplePRs(), nil)

	tests := []struct {
		name      string
		query     string
		wantCount int
		wantFirst int
	}{
		{name: "empty query returns all", query: "", wantCount: 3, wantFirst: 10},
		{name: "match by author", query: "bob", wantCount: 1, wantFirst: 42},
		{name: "match by title", query: "performance", wantCount: 1, wantFirst: 99},
		{name: "case insensitive author", query: "BOB", wantCount: 1, wantFirst: 42},
		{name: "no match", query: "zzz", wantCount: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterItems(items, tt.query)
			assert.Len(t, result, tt.wantCount)
			if tt.wantCount > 0 {
				assert.Equal(t, tt.wantFirst, result[0].ItemNumber())
			}
		})
	}
}

func TestSelectIssueWithPrompt_QueryFiltersResults(t *testing.T) {
	issues := sampleIssues()
	stdin := strings.NewReader("1\n")
	stderr := &bytes.Buffer{}

	selected, err := selectIssueWithPrompt(issues, nil, "dark", stdin, stderr)
	require.NoError(t, err)
	assert.Equal(t, 42, selected.Number)

	output := stderr.String()
	assert.Contains(t, output, "[1] #42")
	assert.NotContains(t, output, "[2]")
}

func TestSelectIssueWithPrompt_QueryNoMatch(t *testing.T) {
	issues := sampleIssues()
	stdin := strings.NewReader("1\n")
	stderr := &bytes.Buffer{}

	_, err := selectIssueWithPrompt(issues, nil, "nonexistent", stdin, stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `no issues matching "nonexistent"`)
}

func TestSelectPRWithPrompt_QueryFiltersResults(t *testing.T) {
	prs := samplePRs()
	stdin := strings.NewReader("1\n")
	stderr := &bytes.Buffer{}

	selected, err := selectPRWithPrompt(prs, nil, "dark", stdin, stderr)
	require.NoError(t, err)
	assert.Equal(t, 42, selected.Number)
}

func TestSelectPRWithPrompt_QueryNoMatch(t *testing.T) {
	prs := samplePRs()
	stdin := strings.NewReader("1\n")
	stderr := &bytes.Buffer{}

	_, err := selectPRWithPrompt(prs, nil, "nonexistent", stdin, stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `no pull requests matching "nonexistent"`)
}

func TestIssuesWithWorktree(t *testing.T) {
	worktrees := []*models.WorktreeInfo{
		{Path: "/repo", Branch: "main", IsMain: true},
		{Path: "/wt/issue-42-add-dark-mode", Branch: "issue-42-add-dark-mode"},
		{Path: "/wt/gh-10", Branch: "renamed-branch"},
		{Path: "/wt/feat-99", Branch: "feat/99-ai-title"},
	}
	tests := []struct {
		name     string
		template string
		want     map[int]bool
	}{
		{name: "default template matches branch", template: "", want: map[int]bool{42: true}},
		{name: "explicit default template", template: "issue-{number}-{title}", want: map[int]bool{42: true}},
		{name: "matches worktree directory name", template: "gh-{number}-{title}", want: map[int]bool{10: true}},
		{name: "prefix with slash and generated title", template: "feat/{number}-{generated}", want: map[int]bool{99: true}},
		{name: "template without number", template: "{title}", want: map[int]bool{}},
		{name: "placeholder before number", template: "{title}-{number}", want: map[int]bool{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := issuesWithWorktree(sampleIssues(), worktrees, tt.template)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIssuesWithWorktree_NumberBoundary(t *testing.T) {
	issues := []*models.IssueInfo{{Number: 4}, {Number: 42}, {Number: 420}}
	worktrees := []*models.WorktreeInfo{{Path: "/wt/x", Branch: "issue-42-something"}}

	got := issuesWithWorktree(issues, worktrees, "issue-{number}-{title}")
	assert.Equal(t, map[int]bool{42: true}, got)
}

func TestPRsWithWorktree(t *testing.T) {
	prs := append(samplePRs(), &models.PRInfo{Number: 7, Branch: ""})
	worktrees := []*models.WorktreeInfo{
		{Path: "/repo", Branch: "main", IsMain: true},
		{Path: "/wt/pr-42-dark", Branch: "dark-mode"},
		{Path: "/wt/detached", Branch: ""},
	}

	got := prsWithWorktree(prs, worktrees)
	assert.Equal(t, map[int]bool{42: true}, got)
}

func TestSelectPRInteractive_MarksCheckedOutPRs(t *testing.T) {
	oldFunc := selectPRFunc
	t.Cleanup(func() { selectPRFunc = oldFunc })
	selectPRFunc = selectPRWithPrompt

	gitSvc := &mockGitServiceForInteractive{
		prs:       samplePRs(),
		worktrees: []*models.WorktreeInfo{{Path: "/wt/dark", Branch: "dark-mode"}},
	}
	stderr := &bytes.Buffer{}

	num, err := SelectPRInteractive(context.Background(), gitSvc, "", strings.NewReader("1\n"), stderr)
	require.NoError(t, err)
	assert.Equal(t, 10, num)

	out := stderr.String()
	assert.Contains(t, out, "[draft] [CI: pending] [worktree]")
	assert.Equal(t, 1, strings.Count(out, worktreeTag))
	assert.NotContains(t, out, ansiGreen, "non-terminal output must not be coloured")
}

func TestSelectIssueInteractive_MarksCheckedOutIssues(t *testing.T) {
	oldFunc := selectIssueFunc
	t.Cleanup(func() { selectIssueFunc = oldFunc })
	selectIssueFunc = selectIssueWithPrompt

	gitSvc := &mockGitServiceForInteractive{
		issues:    sampleIssues(),
		worktrees: []*models.WorktreeInfo{{Path: "/wt/bug-10-login", Branch: "bug-10-login"}},
	}
	stderr := &bytes.Buffer{}

	num, err := SelectIssueInteractive(context.Background(), gitSvc, "", "bug-{number}-{title}", strings.NewReader("2\n"), stderr)
	require.NoError(t, err)
	assert.Equal(t, 42, num)
	assert.Contains(t, stderr.String(), "Fix login bug  [worktree]")
	assert.Equal(t, 1, strings.Count(stderr.String(), worktreeTag))
}

func TestSelectInteractive_WorktreeListErrorIsNotFatal(t *testing.T) {
	oldIssue, oldPR := selectIssueFunc, selectPRFunc
	t.Cleanup(func() { selectIssueFunc, selectPRFunc = oldIssue, oldPR })

	var issueMarks, prMarks map[int]bool
	selectIssueFunc = func(issues []*models.IssueInfo, checkedOut map[int]bool, _ string, _ io.Reader, _ io.Writer) (*models.IssueInfo, error) {
		issueMarks = checkedOut
		return issues[0], nil
	}
	selectPRFunc = func(prs []*models.PRInfo, checkedOut map[int]bool, _ string, _ io.Reader, _ io.Writer) (*models.PRInfo, error) {
		prMarks = checkedOut
		return prs[0], nil
	}

	gitSvc := &mockGitServiceForInteractive{issues: sampleIssues(), prs: samplePRs(), wtErr: assert.AnError}

	_, err := SelectIssueInteractive(context.Background(), gitSvc, "", "", strings.NewReader(""), &bytes.Buffer{})
	require.NoError(t, err)
	assert.Empty(t, issueMarks)

	_, err = SelectPRInteractive(context.Background(), gitSvc, "", strings.NewReader(""), &bytes.Buffer{})
	require.NoError(t, err)
	assert.Empty(t, prMarks)
}

func TestDisplayLine_ColoursCheckedOutItems(t *testing.T) {
	items := wrapPRs(samplePRs(), map[int]bool{42: true})

	assert.Equal(t, items[0].FormatLine(), displayLine(items[0], true))
	coloured := displayLine(items[1], true)
	assert.Equal(t, ansiGreen+items[1].FormatLine()+ansiReset, coloured)
	assert.Equal(t, items[1].FormatLine(), displayLine(items[1], false))

	if _, err := exec.LookPath("fzf"); err != nil {
		t.Skip("fzf not installed, skipping ANSI round-trip check")
	}
	cmd := exec.Command("fzf", "--ansi", "--filter", "dark")
	cmd.Stdin = strings.NewReader(coloured + "\n")
	out, err := cmd.Output()
	require.NoError(t, err)
	num, err := parseNumberFromLine(strings.TrimSpace(string(out)))
	require.NoError(t, err)
	assert.Equal(t, 42, num)
}
