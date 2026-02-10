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

// selectableItem is implemented by types that can be presented in an interactive selector.
type selectableItem interface {
	ItemNumber() int
	FormatLine() string
	FormatPreview() string
}

// issueItem wraps IssueInfo to implement selectableItem.
type issueItem struct{ *models.IssueInfo }

func (i issueItem) ItemNumber() int { return i.Number }

func (i issueItem) FormatLine() string {
	return fmt.Sprintf("#%-6d %s", i.Number, strings.Join(strings.Fields(i.Title), " "))
}

func (i issueItem) FormatPreview() string {
	if i.Body == "" {
		return "(no description)"
	}
	return i.Body
}

// prItem wraps PRInfo to implement selectableItem.
type prItem struct{ *models.PRInfo }

func (p prItem) ItemNumber() int { return p.Number }

func (p prItem) FormatLine() string {
	title := strings.Join(strings.Fields(p.Title), " ")
	var tags []string
	if p.IsDraft {
		tags = append(tags, "[draft]")
	}
	if p.CIStatus != "" && p.CIStatus != "none" {
		tags = append(tags, fmt.Sprintf("[CI: %s]", p.CIStatus))
	}
	tagStr := ""
	if len(tags) > 0 {
		tagStr = "  " + strings.Join(tags, " ")
	}
	return fmt.Sprintf("#%-6d %-12s %s%s", p.Number, p.Author, title, tagStr)
}

func (p prItem) FormatPreview() string {
	var parts []string
	parts = append(parts, fmt.Sprintf("Author: %s", p.Author))
	parts = append(parts, fmt.Sprintf("Branch: %s -> %s", p.Branch, p.BaseBranch))
	if p.IsDraft {
		parts = append(parts, "Status: Draft")
	}
	if p.CIStatus != "" && p.CIStatus != "none" {
		parts = append(parts, fmt.Sprintf("CI: %s", p.CIStatus))
	}
	body := p.Body
	if body == "" {
		body = "(no description)"
	}
	parts = append(parts, "", body)
	return strings.Join(parts, "\n")
}

// selector function types used for test injection.
type (
	issueSelector func(issues []*models.IssueInfo, stdin io.Reader, stderr io.Writer) (*models.IssueInfo, error)
	prSelector    func(prs []*models.PRInfo, stdin io.Reader, stderr io.Writer) (*models.PRInfo, error)
)

var (
	selectIssueFunc issueSelector = selectIssueDefault
	selectPRFunc    prSelector    = selectPRDefault
	fzfLookPath                   = exec.LookPath
)

// --- Generic selection helpers ---

// selectWithFzf pipes items through fzf and returns the selected item.
func selectWithFzf[T selectableItem](items []T, prompt, header, cancelMsg, notFoundMsg string, stderr io.Writer) (T, error) {
	var zero T
	lookup := make(map[int]T, len(items))
	var lines []string
	for _, item := range items {
		lookup[item.ItemNumber()] = item
		lines = append(lines, item.FormatLine())
	}
	input := strings.Join(lines, "\n")
	previewScript := buildGenericPreviewScript(items)

	//nolint:gosec // This is not executing user input, just a static script we built
	cmd := exec.Command("fzf",
		"--ansi",
		"--prompt", prompt,
		"--header", header,
		"--preview", previewScript,
		"--preview-window", "wrap:down:40%",
	)
	cmd.Stdin = strings.NewReader(input)
	cmd.Stderr = stderr

	out, err := cmd.Output()
	if err != nil {
		return zero, fmt.Errorf("%s", cancelMsg)
	}

	selected := strings.TrimSpace(string(out))
	if selected == "" {
		return zero, fmt.Errorf("%s", notFoundMsg)
	}

	num, err := parseNumberFromLine(selected)
	if err != nil {
		return zero, err
	}
	item, ok := lookup[num]
	if !ok {
		return zero, fmt.Errorf("%s #%d not found", notFoundMsg, num)
	}
	return item, nil
}

// selectWithPrompt displays a numbered list and reads the user's choice.
func selectWithPrompt[T selectableItem](items []T, noun string, stdin io.Reader, stderr io.Writer) (T, error) {
	var zero T
	fmt.Fprintf(stderr, "\nOpen %ss:\n\n", noun)
	for i, item := range items {
		fmt.Fprintf(stderr, "  [%d] %s\n", i+1, item.FormatLine())
	}
	fmt.Fprintf(stderr, "\nSelect %s [1-%d]: ", noun, len(items))

	scanner := bufio.NewScanner(stdin)
	if !scanner.Scan() {
		return zero, fmt.Errorf("%s selection cancelled", noun)
	}

	text := strings.TrimSpace(scanner.Text())
	if text == "" {
		return zero, fmt.Errorf("no %s selected", noun)
	}

	idx, err := strconv.Atoi(text)
	if err != nil {
		return zero, fmt.Errorf("invalid selection: %q", text)
	}

	if idx < 1 || idx > len(items) {
		return zero, fmt.Errorf("selection out of range: %d (must be 1-%d)", idx, len(items))
	}

	return items[idx-1], nil
}

// buildGenericPreviewScript creates a shell script that maps item numbers to their
// preview text for the fzf --preview option.
func buildGenericPreviewScript[T selectableItem](items []T) string {
	var sb strings.Builder
	sb.WriteString("num=$(echo {} | sed 's/^#\\([0-9]*\\).*/\\1/'); case $num in ")
	for _, item := range items {
		preview := item.FormatPreview()
		// Escape single quotes for the shell
		preview = strings.ReplaceAll(preview, "'", "'\\''")
		sb.WriteString(fmt.Sprintf("%d) echo '%s';; ", item.ItemNumber(), preview))
	}
	sb.WriteString("*) echo 'No preview available';; esac")
	return sb.String()
}

// parseNumberFromLine extracts the number from a line like "#42     Fix the bug".
func parseNumberFromLine(line string) (int, error) {
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

// --- Issue selectors (thin wrappers around generic helpers) ---

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

func selectIssueDefault(issues []*models.IssueInfo, stdin io.Reader, stderr io.Writer) (*models.IssueInfo, error) {
	if _, err := fzfLookPath("fzf"); err == nil {
		return selectIssueWithFzf(issues, stderr)
	}
	return selectIssueWithPrompt(issues, stdin, stderr)
}

func selectIssueWithFzf(issues []*models.IssueInfo, stderr io.Writer) (*models.IssueInfo, error) {
	items := wrapIssues(issues)
	selected, err := selectWithFzf(items, "Select issue> ", "Issue selection (type to filter)", "issue selection cancelled", "no issue selected", stderr)
	if err != nil {
		return nil, err
	}
	return selected.IssueInfo, nil
}

func selectIssueWithPrompt(issues []*models.IssueInfo, stdin io.Reader, stderr io.Writer) (*models.IssueInfo, error) {
	items := wrapIssues(issues)
	selected, err := selectWithPrompt(items, "issue", stdin, stderr)
	if err != nil {
		return nil, err
	}
	return selected.IssueInfo, nil
}

// buildPreviewScript creates a shell script for issue previews (kept for test compatibility).
func buildPreviewScript(issues []*models.IssueInfo) string {
	return buildGenericPreviewScript(wrapIssues(issues))
}

// SelectIssueInteractiveFromStdio wraps SelectIssueInteractive with os.Stdin/os.Stderr.
func SelectIssueInteractiveFromStdio(ctx context.Context, gitSvc gitService) (int, error) {
	return SelectIssueInteractive(ctx, gitSvc, os.Stdin, os.Stderr)
}

// --- PR selectors (thin wrappers around generic helpers) ---

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

func selectPRDefault(prs []*models.PRInfo, stdin io.Reader, stderr io.Writer) (*models.PRInfo, error) {
	if _, err := fzfLookPath("fzf"); err == nil {
		return selectPRWithFzf(prs, stderr)
	}
	return selectPRWithPrompt(prs, stdin, stderr)
}

func selectPRWithFzf(prs []*models.PRInfo, stderr io.Writer) (*models.PRInfo, error) {
	items := wrapPRs(prs)
	selected, err := selectWithFzf(items, "Select PR> ", "Pull request selection (type to filter)", "pull request selection cancelled", "no pull request selected", stderr)
	if err != nil {
		return nil, err
	}
	return selected.PRInfo, nil
}

func selectPRWithPrompt(prs []*models.PRInfo, stdin io.Reader, stderr io.Writer) (*models.PRInfo, error) {
	items := wrapPRs(prs)
	selected, err := selectWithPrompt(items, "pull request", stdin, stderr)
	if err != nil {
		return nil, err
	}
	return selected.PRInfo, nil
}

// buildPRPreviewScript creates a shell script for PR previews (kept for test compatibility).
func buildPRPreviewScript(prs []*models.PRInfo) string {
	return buildGenericPreviewScript(wrapPRs(prs))
}

// SelectPRInteractiveFromStdio wraps SelectPRInteractive with os.Stdin/os.Stderr.
func SelectPRInteractiveFromStdio(ctx context.Context, gitSvc gitService) (int, error) {
	return SelectPRInteractive(ctx, gitSvc, os.Stdin, os.Stderr)
}

// --- Conversion helpers ---

func wrapIssues(issues []*models.IssueInfo) []issueItem {
	items := make([]issueItem, len(issues))
	for i, issue := range issues {
		items[i] = issueItem{issue}
	}
	return items
}

func wrapPRs(prs []*models.PRInfo) []prItem {
	items := make([]prItem, len(prs))
	for i, pr := range prs {
		items[i] = prItem{pr}
	}
	return items
}

// parseIssueNumberFromLine is an alias for parseNumberFromLine kept for test compatibility.
var parseIssueNumberFromLine = parseNumberFromLine
