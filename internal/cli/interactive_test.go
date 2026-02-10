package cli

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"

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

// --- selectIssueWithPrompt tests ---

func TestSelectIssueWithPrompt_ValidSelection(t *testing.T) {
	issues := sampleIssues()
	stdin := strings.NewReader("2\n")
	stderr := &bytes.Buffer{}

	selected, err := selectIssueWithPrompt(issues, stdin, stderr)
	require.NoError(t, err)
	assert.Equal(t, 42, selected.Number)
	assert.Equal(t, "Add dark mode", selected.Title)

	// Verify the prompt was printed to stderr
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

	selected, err := selectIssueWithPrompt(issues, stdin, stderr)
	require.NoError(t, err)
	assert.Equal(t, 10, selected.Number)
}

func TestSelectIssueWithPrompt_LastItem(t *testing.T) {
	issues := sampleIssues()
	stdin := strings.NewReader("3\n")
	stderr := &bytes.Buffer{}

	selected, err := selectIssueWithPrompt(issues, stdin, stderr)
	require.NoError(t, err)
	assert.Equal(t, 99, selected.Number)
}

func TestSelectIssueWithPrompt_OutOfRangeTooHigh(t *testing.T) {
	issues := sampleIssues()
	stdin := strings.NewReader("5\n")
	stderr := &bytes.Buffer{}

	_, err := selectIssueWithPrompt(issues, stdin, stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "selection out of range")
}

func TestSelectIssueWithPrompt_OutOfRangeZero(t *testing.T) {
	issues := sampleIssues()
	stdin := strings.NewReader("0\n")
	stderr := &bytes.Buffer{}

	_, err := selectIssueWithPrompt(issues, stdin, stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "selection out of range")
}

func TestSelectIssueWithPrompt_NegativeNumber(t *testing.T) {
	issues := sampleIssues()
	stdin := strings.NewReader("-1\n")
	stderr := &bytes.Buffer{}

	_, err := selectIssueWithPrompt(issues, stdin, stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "selection out of range")
}

func TestSelectIssueWithPrompt_NonNumeric(t *testing.T) {
	issues := sampleIssues()
	stdin := strings.NewReader("abc\n")
	stderr := &bytes.Buffer{}

	_, err := selectIssueWithPrompt(issues, stdin, stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid selection")
}

func TestSelectIssueWithPrompt_EmptyInput(t *testing.T) {
	issues := sampleIssues()
	stdin := strings.NewReader("\n")
	stderr := &bytes.Buffer{}

	_, err := selectIssueWithPrompt(issues, stdin, stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no issue selected")
}

func TestSelectIssueWithPrompt_EOF(t *testing.T) {
	issues := sampleIssues()
	stdin := strings.NewReader("") // EOF immediately
	stderr := &bytes.Buffer{}

	_, err := selectIssueWithPrompt(issues, stdin, stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
}

// --- parseIssueNumberFromLine tests ---

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

// --- buildPreviewScript tests ---

func TestBuildPreviewScript(t *testing.T) {
	issues := sampleIssues()
	script := buildPreviewScript(issues)

	// Should contain a case statement for each issue
	assert.Contains(t, script, "10)")
	assert.Contains(t, script, "42)")
	assert.Contains(t, script, "99)")

	// Issue with no body should show placeholder
	assert.Contains(t, script, "(no description)")

	// Should include issue body text
	assert.Contains(t, script, "The login page crashes on submit.")
	assert.Contains(t, script, "Support dark theme across the UI.")
}

func TestBuildPreviewScript_SingleQuoteEscaping(t *testing.T) {
	issues := []*models.IssueInfo{
		{Number: 1, Title: "Test", Body: "It's a bug that can't be fixed"},
	}
	script := buildPreviewScript(issues)

	// Single quotes must be escaped for shell safety
	assert.Contains(t, script, "It'\\''s a bug that can'\\''t be fixed")
}

// --- SelectIssueInteractive tests (with mock gitService) ---

type mockGitServiceForInteractive struct {
	issues []*models.IssueInfo
	err    error
}

func (m *mockGitServiceForInteractive) FetchAllOpenIssues(_ context.Context) ([]*models.IssueInfo, error) {
	return m.issues, m.err
}

// Implement remaining gitService methods as no-ops for the interface.
func (m *mockGitServiceForInteractive) CheckoutPRBranch(context.Context, int, string, string) bool {
	return false
}

func (m *mockGitServiceForInteractive) CreateWorktreeFromPR(context.Context, int, string, string, string) bool {
	return false
}

func (m *mockGitServiceForInteractive) ExecuteCommands(context.Context, []string, string, map[string]string) error {
	return nil
}

func (m *mockGitServiceForInteractive) FetchAllOpenPRs(context.Context) ([]*models.PRInfo, error) {
	return nil, nil
}

func (m *mockGitServiceForInteractive) GetCurrentBranch(context.Context) (string, error) {
	return "main", nil
}
func (m *mockGitServiceForInteractive) GetMainWorktreePath(context.Context) string { return "" }
func (m *mockGitServiceForInteractive) GetWorktrees(context.Context) ([]*models.WorktreeInfo, error) {
	return nil, nil
}
func (m *mockGitServiceForInteractive) ResolveRepoName(context.Context) string { return "repo" }
func (m *mockGitServiceForInteractive) RunCommandChecked(context.Context, []string, string, string) bool {
	return true
}

func (m *mockGitServiceForInteractive) RunGit(context.Context, []string, string, []int, bool, bool) string {
	return ""
}

func TestSelectIssueInteractive_NoIssues(t *testing.T) {
	gitSvc := &mockGitServiceForInteractive{issues: []*models.IssueInfo{}}
	stderr := &bytes.Buffer{}

	_, err := SelectIssueInteractive(context.Background(), gitSvc, strings.NewReader(""), stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no open issues found")
}

func TestSelectIssueInteractive_FetchError(t *testing.T) {
	gitSvc := &mockGitServiceForInteractive{err: assert.AnError}
	stderr := &bytes.Buffer{}

	_, err := SelectIssueInteractive(context.Background(), gitSvc, strings.NewReader(""), stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch issues")
}

func TestSelectIssueInteractive_UsesPromptFallback(t *testing.T) {
	// Override selectIssueFunc to use prompt fallback directly
	oldFunc := selectIssueFunc
	t.Cleanup(func() { selectIssueFunc = oldFunc })

	selectIssueFunc = selectIssueWithPrompt

	gitSvc := &mockGitServiceForInteractive{issues: sampleIssues()}
	stderr := &bytes.Buffer{}

	num, err := SelectIssueInteractive(context.Background(), gitSvc, strings.NewReader("2\n"), stderr)
	require.NoError(t, err)
	assert.Equal(t, 42, num)
}

func TestSelectIssueDefault_FallsBackToPromptWhenNoFzf(t *testing.T) {
	// Override fzfLookPath to simulate fzf not being installed
	oldLookPath := fzfLookPath
	t.Cleanup(func() { fzfLookPath = oldLookPath })

	fzfLookPath = func(name string) (string, error) {
		return "", exec.ErrNotFound
	}

	issues := sampleIssues()
	stdin := strings.NewReader("1\n")
	stderr := &bytes.Buffer{}

	selected, err := selectIssueDefault(issues, stdin, stderr)
	require.NoError(t, err)
	assert.Equal(t, 10, selected.Number)
}

func TestSelectIssueInteractive_FormattedLinesParseable(t *testing.T) {
	// Verify the formatted lines produced by selectIssueWithFzf can be parsed back
	issues := sampleIssues()
	for _, issue := range issues {
		// This matches the format used in selectIssueWithFzf
		line := fmt.Sprintf("#%-6d %s", issue.Number, issue.Title)
		num, err := parseIssueNumberFromLine(line)
		require.NoError(t, err, "failed to parse line: %q", line)
		assert.Equal(t, issue.Number, num)
	}
}

func TestSelectIssueWithFzf_Integration(t *testing.T) {
	// Skip if fzf is not installed
	if _, err := exec.LookPath("fzf"); err != nil {
		t.Skip("fzf not installed, skipping integration test")
	}

	// Use fzf --filter to simulate non-interactive selection
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

	// First result line should be parseable
	firstLine := strings.Split(strings.TrimSpace(string(out)), "\n")[0]
	num, err := parseIssueNumberFromLine(firstLine)
	require.NoError(t, err)
	assert.Equal(t, 42, num)
}
