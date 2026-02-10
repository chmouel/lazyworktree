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

func TestSelectIssueWithPrompt_ValidSelection(t *testing.T) {
	issues := sampleIssues()
	stdin := strings.NewReader("2\n")
	stderr := &bytes.Buffer{}

	selected, err := selectIssueWithPrompt(issues, stdin, stderr)
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
	issues []*models.IssueInfo
	err    error
	prs    []*models.PRInfo
	prsErr error
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

	selected, err := selectPRWithPrompt(prs, stdin, stderr)
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

	selected, err := selectPRWithPrompt(prs, stdin, stderr)
	require.NoError(t, err)
	assert.Equal(t, 10, selected.Number)
}

func TestSelectPRWithPrompt_LastItem(t *testing.T) {
	prs := samplePRs()
	stdin := strings.NewReader("3\n")
	stderr := &bytes.Buffer{}

	selected, err := selectPRWithPrompt(prs, stdin, stderr)
	require.NoError(t, err)
	assert.Equal(t, 99, selected.Number)
}

func TestSelectPRWithPrompt_OutOfRangeTooHigh(t *testing.T) {
	prs := samplePRs()
	stdin := strings.NewReader("5\n")
	stderr := &bytes.Buffer{}

	_, err := selectPRWithPrompt(prs, stdin, stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "selection out of range")
}

func TestSelectPRWithPrompt_OutOfRangeZero(t *testing.T) {
	prs := samplePRs()
	stdin := strings.NewReader("0\n")
	stderr := &bytes.Buffer{}

	_, err := selectPRWithPrompt(prs, stdin, stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "selection out of range")
}

func TestSelectPRWithPrompt_NonNumeric(t *testing.T) {
	prs := samplePRs()
	stdin := strings.NewReader("abc\n")
	stderr := &bytes.Buffer{}

	_, err := selectPRWithPrompt(prs, stdin, stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid selection")
}

func TestSelectPRWithPrompt_EmptyInput(t *testing.T) {
	prs := samplePRs()
	stdin := strings.NewReader("\n")
	stderr := &bytes.Buffer{}

	_, err := selectPRWithPrompt(prs, stdin, stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no pull request selected")
}

func TestSelectPRWithPrompt_EOF(t *testing.T) {
	prs := samplePRs()
	stdin := strings.NewReader("")
	stderr := &bytes.Buffer{}

	_, err := selectPRWithPrompt(prs, stdin, stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
}

func TestSelectPRWithPrompt_DraftAndCITags(t *testing.T) {
	prs := samplePRs()
	stdin := strings.NewReader("2\n")
	stderr := &bytes.Buffer{}

	_, err := selectPRWithPrompt(prs, stdin, stderr)
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

func TestSelectPRInteractive_NoPRs(t *testing.T) {
	gitSvc := &mockGitServiceForInteractive{prs: []*models.PRInfo{}}
	stderr := &bytes.Buffer{}

	_, err := SelectPRInteractive(context.Background(), gitSvc, strings.NewReader(""), stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no open pull requests found")
}

func TestSelectPRInteractive_FetchError(t *testing.T) {
	gitSvc := &mockGitServiceForInteractive{prsErr: assert.AnError}
	stderr := &bytes.Buffer{}

	_, err := SelectPRInteractive(context.Background(), gitSvc, strings.NewReader(""), stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch pull requests")
}

func TestSelectPRInteractive_UsesPromptFallback(t *testing.T) {
	oldFunc := selectPRFunc
	t.Cleanup(func() { selectPRFunc = oldFunc })

	selectPRFunc = selectPRWithPrompt

	gitSvc := &mockGitServiceForInteractive{prs: samplePRs()}
	stderr := &bytes.Buffer{}

	num, err := SelectPRInteractive(context.Background(), gitSvc, strings.NewReader("2\n"), stderr)
	require.NoError(t, err)
	assert.Equal(t, 42, num)
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

	selected, err := selectPRDefault(prs, stdin, stderr)
	require.NoError(t, err)
	assert.Equal(t, 10, selected.Number)
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
