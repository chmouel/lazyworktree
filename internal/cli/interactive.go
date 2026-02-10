package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/chmouel/lazyworktree/internal/models"
)

type issueSelector func(issues []*models.IssueInfo, stdin io.Reader, stderr io.Writer) (*models.IssueInfo, error)

// selectIssueFunc is the default issue selector, replaceable in tests.
var selectIssueFunc issueSelector = selectIssueDefault

var fzfLookPath = exec.LookPath

// SelectIssueInteractive presents an interactive issue selector (fzf when available, numbered list otherwise).
func SelectIssueInteractive(ctx context.Context, gitSvc gitService, stdin io.Reader, stderr io.Writer) (int, error) {
	fmt.Fprintf(stderr, "Fetching open issues...\n")

	issues, err := gitSvc.FetchAllOpenIssues(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch issues: %w", err)
	}

	if len(issues) == 0 {
		return 0, fmt.Errorf("no open issues found")
	}

	selected, err := selectIssueFunc(issues, stdin, stderr)
	if err != nil {
		return 0, err
	}

	return selected.Number, nil
}

// selectIssueDefault chooses between fzf and the plain fallback.
func selectIssueDefault(issues []*models.IssueInfo, stdin io.Reader, stderr io.Writer) (*models.IssueInfo, error) {
	if _, err := fzfLookPath("fzf"); err == nil {
		return selectIssueWithFzf(issues, stderr)
	}
	return selectIssueWithPrompt(issues, stdin, stderr)
}

// selectIssueWithFzf pipes issues through fzf with a preview of the body.
func selectIssueWithFzf(issues []*models.IssueInfo, stderr io.Writer) (*models.IssueInfo, error) {
	lookup := make(map[int]*models.IssueInfo, len(issues))
	var lines []string
	for _, issue := range issues {
		lookup[issue.Number] = issue
		title := strings.Join(strings.Fields(issue.Title), " ")
		line := fmt.Sprintf("#%-6d %s", issue.Number, title)
		lines = append(lines, line)
	}
	input := strings.Join(lines, "\n")

	previewScript := buildPreviewScript(issues)

	//nolint:gosec // This is not executing user input, just a static script we built
	cmd := exec.Command("fzf",
		"--ansi",
		"--prompt", "Select issue> ",
		"--header", "Issue selection (type to filter)",
		"--preview", previewScript,
		"--preview-window", "wrap:down:40%",
	)
	cmd.Stdin = strings.NewReader(input)
	cmd.Stderr = stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("issue selection cancelled")
	}

	selected := strings.TrimSpace(string(out))
	if selected == "" {
		return nil, fmt.Errorf("no issue selected")
	}

	num, err := parseIssueNumberFromLine(selected)
	if err != nil {
		return nil, err
	}

	issue, ok := lookup[num]
	if !ok {
		return nil, fmt.Errorf("issue #%d not found", num)
	}
	return issue, nil
}

// buildPreviewScript creates a shell script that maps issue numbers to their
// body text for the fzf --preview option.
func buildPreviewScript(issues []*models.IssueInfo) string {
	var sb strings.Builder
	sb.WriteString("num=$(echo {} | sed 's/^#\\([0-9]*\\).*/\\1/'); case $num in ")
	for _, issue := range issues {
		body := issue.Body
		if body == "" {
			body = "(no description)"
		}
		// Escape single quotes for the shell
		body = strings.ReplaceAll(body, "'", "'\\''")
		sb.WriteString(fmt.Sprintf("%d) echo '%s';; ", issue.Number, body))
	}
	sb.WriteString("*) echo 'No preview available';; esac")
	return sb.String()
}

// selectIssueWithPrompt displays a numbered list and reads the user's choice.
func selectIssueWithPrompt(issues []*models.IssueInfo, stdin io.Reader, stderr io.Writer) (*models.IssueInfo, error) {
	fmt.Fprintf(stderr, "\nOpen issues:\n\n")
	for i, issue := range issues {
		title := strings.Join(strings.Fields(issue.Title), " ")
		fmt.Fprintf(stderr, "  [%d] #%-6d %s\n", i+1, issue.Number, title)
	}
	fmt.Fprintf(stderr, "\nSelect issue [1-%d]: ", len(issues))

	scanner := bufio.NewScanner(stdin)
	if !scanner.Scan() {
		return nil, fmt.Errorf("issue selection cancelled")
	}

	text := strings.TrimSpace(scanner.Text())
	if text == "" {
		return nil, fmt.Errorf("no issue selected")
	}

	idx, err := strconv.Atoi(text)
	if err != nil {
		return nil, fmt.Errorf("invalid selection: %q", text)
	}

	if idx < 1 || idx > len(issues) {
		return nil, fmt.Errorf("selection out of range: %d (must be 1-%d)", idx, len(issues))
	}

	return issues[idx-1], nil
}

// parseIssueNumberFromLine extracts the issue number from a line like "#42     Fix the bug".
func parseIssueNumberFromLine(line string) (int, error) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "#") {
		return 0, fmt.Errorf("unexpected line format: %q", line)
	}
	rest := strings.TrimPrefix(line, "#")
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return 0, fmt.Errorf("unexpected line format: %q", line)
	}
	num, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("failed to parse issue number from %q: %w", line, err)
	}
	return num, nil
}

// SelectIssueInteractiveFromStdio wraps SelectIssueInteractive with os.Stdin/os.Stderr.
func SelectIssueInteractiveFromStdio(ctx context.Context, gitSvc gitService) (int, error) {
	return SelectIssueInteractive(ctx, gitSvc, os.Stdin, os.Stderr)
}

type prSelector func(prs []*models.PRInfo, stdin io.Reader, stderr io.Writer) (*models.PRInfo, error)

// selectPRFunc is the default PR selector, replaceable in tests.
var selectPRFunc prSelector = selectPRDefault

// SelectPRInteractive presents an interactive PR selector (fzf when available, numbered list otherwise).
func SelectPRInteractive(ctx context.Context, gitSvc gitService, stdin io.Reader, stderr io.Writer) (int, error) {
	fmt.Fprintf(stderr, "Fetching open pull requests...\n")

	prs, err := gitSvc.FetchAllOpenPRs(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch pull requests: %w", err)
	}

	if len(prs) == 0 {
		return 0, fmt.Errorf("no open pull requests found")
	}

	selected, err := selectPRFunc(prs, stdin, stderr)
	if err != nil {
		return 0, err
	}

	return selected.Number, nil
}

// selectPRDefault chooses between fzf and the plain fallback.
func selectPRDefault(prs []*models.PRInfo, stdin io.Reader, stderr io.Writer) (*models.PRInfo, error) {
	if _, err := fzfLookPath("fzf"); err == nil {
		return selectPRWithFzf(prs, stderr)
	}
	return selectPRWithPrompt(prs, stdin, stderr)
}

// selectPRWithFzf pipes PRs through fzf with a preview of the body.
func selectPRWithFzf(prs []*models.PRInfo, stderr io.Writer) (*models.PRInfo, error) {
	lookup := make(map[int]*models.PRInfo, len(prs))
	var lines []string
	for _, pr := range prs {
		lookup[pr.Number] = pr
		title := strings.Join(strings.Fields(pr.Title), " ")
		var tags []string
		if pr.IsDraft {
			tags = append(tags, "[draft]")
		}
		if pr.CIStatus != "" && pr.CIStatus != "none" {
			tags = append(tags, fmt.Sprintf("[CI: %s]", pr.CIStatus))
		}
		tagStr := ""
		if len(tags) > 0 {
			tagStr = "  " + strings.Join(tags, " ")
		}
		line := fmt.Sprintf("#%-6d %-12s %s%s", pr.Number, pr.Author, title, tagStr)
		lines = append(lines, line)
	}
	input := strings.Join(lines, "\n")

	previewScript := buildPRPreviewScript(prs)

	//nolint:gosec // This is not executing user input, just a static script we built
	cmd := exec.Command("fzf",
		"--ansi",
		"--prompt", "Select PR> ",
		"--header", "Pull request selection (type to filter)",
		"--preview", previewScript,
		"--preview-window", "wrap:down:40%",
	)
	cmd.Stdin = strings.NewReader(input)
	cmd.Stderr = stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("pull request selection cancelled")
	}

	selected := strings.TrimSpace(string(out))
	if selected == "" {
		return nil, fmt.Errorf("no pull request selected")
	}

	num, err := parseIssueNumberFromLine(selected)
	if err != nil {
		return nil, err
	}

	pr, ok := lookup[num]
	if !ok {
		return nil, fmt.Errorf("pull request #%d not found", num)
	}
	return pr, nil
}

// buildPRPreviewScript creates a shell script that maps PR numbers to their
// metadata for the fzf --preview option.
func buildPRPreviewScript(prs []*models.PRInfo) string {
	var sb strings.Builder
	sb.WriteString("num=$(echo {} | sed 's/^#\\([0-9]*\\).*/\\1/'); case $num in ")
	for _, pr := range prs {
		var parts []string
		parts = append(parts, fmt.Sprintf("Author: %s", pr.Author))
		parts = append(parts, fmt.Sprintf("Branch: %s -> %s", pr.Branch, pr.BaseBranch))
		if pr.IsDraft {
			parts = append(parts, "Status: Draft")
		}
		if pr.CIStatus != "" && pr.CIStatus != "none" {
			parts = append(parts, fmt.Sprintf("CI: %s", pr.CIStatus))
		}
		body := pr.Body
		if body == "" {
			body = "(no description)"
		}
		parts = append(parts, "", body)
		preview := strings.Join(parts, "\n")
		// Escape single quotes for the shell
		preview = strings.ReplaceAll(preview, "'", "'\\''")
		sb.WriteString(fmt.Sprintf("%d) echo '%s';; ", pr.Number, preview))
	}
	sb.WriteString("*) echo 'No preview available';; esac")
	return sb.String()
}

// selectPRWithPrompt displays a numbered list and reads the user's choice.
func selectPRWithPrompt(prs []*models.PRInfo, stdin io.Reader, stderr io.Writer) (*models.PRInfo, error) {
	fmt.Fprintf(stderr, "\nOpen pull requests:\n\n")
	for i, pr := range prs {
		title := strings.Join(strings.Fields(pr.Title), " ")
		var tags []string
		if pr.IsDraft {
			tags = append(tags, "[draft]")
		}
		if pr.CIStatus != "" && pr.CIStatus != "none" {
			tags = append(tags, fmt.Sprintf("[CI: %s]", pr.CIStatus))
		}
		tagStr := ""
		if len(tags) > 0 {
			tagStr = "  " + strings.Join(tags, " ")
		}
		fmt.Fprintf(stderr, "  [%d] #%-6d %-12s %s%s\n", i+1, pr.Number, pr.Author, title, tagStr)
	}
	fmt.Fprintf(stderr, "\nSelect pull request [1-%d]: ", len(prs))

	scanner := bufio.NewScanner(stdin)
	if !scanner.Scan() {
		return nil, fmt.Errorf("pull request selection cancelled")
	}

	text := strings.TrimSpace(scanner.Text())
	if text == "" {
		return nil, fmt.Errorf("no pull request selected")
	}

	idx, err := strconv.Atoi(text)
	if err != nil {
		return nil, fmt.Errorf("invalid selection: %q", text)
	}

	if idx < 1 || idx > len(prs) {
		return nil, fmt.Errorf("selection out of range: %d (must be 1-%d)", idx, len(prs))
	}

	return prs[idx-1], nil
}

// SelectPRInteractiveFromStdio wraps SelectPRInteractive with os.Stdin/os.Stderr.
func SelectPRInteractiveFromStdio(ctx context.Context, gitSvc gitService) (int, error) {
	return SelectPRInteractive(ctx, gitSvc, os.Stdin, os.Stderr)
}
