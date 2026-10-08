package cli

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	log "github.com/chmouel/lazyworktree/internal/log"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/multiplexer"
	"golang.org/x/term"
)

const (
	worktreeTag               = "[worktree]"
	ansiGreen                 = "\x1b[32m"
	ansiReset                 = "\x1b[0m"
	defaultIssueBranchPattern = "issue-{number}-{title}"
)

// selectableItem is implemented by types that can be presented in an interactive selector.
type selectableItem interface {
	ItemNumber() int
	FormatLine() string
	FormatPreview() string
	HasWorktree() bool
}

// issueItem wraps IssueInfo to implement selectableItem.
type issueItem struct {
	*models.IssueInfo
	hasWorktree bool
}

func (i issueItem) ItemNumber() int { return i.Number }

func (i issueItem) HasWorktree() bool { return i.hasWorktree }

func (i issueItem) FormatLine() string {
	line := fmt.Sprintf("#%-6d %s", i.Number, strings.Join(strings.Fields(i.Title), " "))
	if i.hasWorktree {
		line += "  " + worktreeTag
	}
	return line
}

func (i issueItem) FormatPreview() string {
	if i.Body == "" {
		return "(no description)"
	}
	return i.Body
}

// prItem wraps PRInfo to implement selectableItem.
type prItem struct {
	*models.PRInfo
	hasWorktree bool
}

func (p prItem) ItemNumber() int { return p.Number }

func (p prItem) HasWorktree() bool { return p.hasWorktree }

func (p prItem) FormatLine() string {
	title := strings.Join(strings.Fields(p.Title), " ")
	var tags []string
	if p.IsDraft {
		tags = append(tags, "[draft]")
	}
	if p.CIStatus != "" && p.CIStatus != "none" {
		tags = append(tags, fmt.Sprintf("[CI: %s]", p.CIStatus))
	}
	if p.hasWorktree {
		tags = append(tags, worktreeTag)
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
	issueSelector func(issues []*models.IssueInfo, checkedOut map[int]bool, query string, stdin io.Reader, stderr io.Writer) (*models.IssueInfo, error)
	prSelector    func(prs []*models.PRInfo, checkedOut map[int]bool, query string, stdin io.Reader, stderr io.Writer) (*models.PRInfo, error)
)

var (
	selectIssueFunc issueSelector = selectIssueDefault
	selectPRFunc    prSelector    = selectPRDefault
	fzfLookPath                   = exec.LookPath
)

// --- Generic selection helpers ---

// displayLine returns the item's line, coloured green when it already has a worktree and colour is enabled.
func displayLine[T selectableItem](item T, colour bool) string {
	line := item.FormatLine()
	if colour && item.HasWorktree() {
		return ansiGreen + line + ansiReset
	}
	return line
}

func isTerminalWriter(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd())) //#nosec G115 -- fd conversion is safe on supported platforms
}

// selectWithFzf pipes items through fzf and returns the selected item.
func selectWithFzf[T selectableItem](items []T, query, prompt, header, cancelMsg, notFoundMsg string, stderr io.Writer) (T, error) {
	var zero T
	lookup := make(map[int]T, len(items))
	var lines []string
	for _, item := range items {
		lookup[item.ItemNumber()] = item
		lines = append(lines, displayLine(item, true))
	}
	input := strings.Join(lines, "\n")
	previewCommand, cleanupPreview, err := prepareGenericPreviewCommand(items)
	if err != nil {
		return zero, err
	}
	defer cleanupPreview()

	fzfArgs := []string{
		"--ansi",
		"--prompt", prompt,
		"--header", header,
		"--preview", previewCommand,
		"--preview-window", "wrap:down:40%",
	}
	if query != "" {
		fzfArgs = append(fzfArgs, "--query", query)
	}

	log.Printf("selector: invoking fzf prompt=%q items=%d query=%q", prompt, len(items), query)
	//nolint:gosec // This is not executing user input, just a static script we built
	cmd := exec.Command("fzf", fzfArgs...)
	cmd.Stdin = strings.NewReader(input)
	var fzfStderr bytes.Buffer
	cmd.Stderr = io.MultiWriter(stderr, &fzfStderr)

	out, err := cmd.Output()
	if err != nil {
		stderrText := strings.TrimSpace(fzfStderr.String())
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			log.Printf("selector: fzf failed prompt=%q exit=%d stderr=%q", prompt, exitErr.ExitCode(), stderrText)
		} else {
			log.Printf("selector: fzf failed prompt=%q error=%v stderr=%q", prompt, err, stderrText)
		}
		return zero, fmt.Errorf("%s", cancelMsg)
	}

	selected := strings.TrimSpace(string(out))
	if selected == "" {
		log.Printf("selector: fzf returned empty selection prompt=%q", prompt)
		return zero, fmt.Errorf("%s", notFoundMsg)
	}

	num, err := parseNumberFromLine(selected)
	if err != nil {
		log.Printf("selector: failed to parse fzf selection prompt=%q selection=%q error=%v", prompt, selected, err)
		return zero, err
	}
	item, ok := lookup[num]
	if !ok {
		log.Printf("selector: fzf selection not found prompt=%q number=%d", prompt, num)
		return zero, fmt.Errorf("%s #%d not found", notFoundMsg, num)
	}
	log.Printf("selector: fzf selected prompt=%q number=%d", prompt, num)
	return item, nil
}

// filterItems returns items whose FormatLine() contains query (case-insensitive).
func filterItems[T selectableItem](items []T, query string) []T {
	if query == "" {
		return items
	}
	lower := strings.ToLower(query)
	var result []T
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.FormatLine()), lower) {
			result = append(result, item)
		}
	}
	return result
}

// selectWithPrompt displays a numbered list and reads the user's choice.
func selectWithPrompt[T selectableItem](items []T, query, noun string, stdin io.Reader, stderr io.Writer) (T, error) {
	var zero T
	total := len(items)
	items = filterItems(items, query)
	log.Printf("selector: using numbered prompt noun=%q items=%d filtered=%d query=%q", noun, total, len(items), query)
	if len(items) == 0 {
		return zero, fmt.Errorf("no %ss matching %q", noun, query)
	}
	colour := isTerminalWriter(stderr)
	fmt.Fprintf(stderr, "\nOpen %ss:\n\n", noun)
	for i, item := range items {
		fmt.Fprintf(stderr, "  [%d] %s\n", i+1, displayLine(item, colour))
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

func prepareGenericPreviewCommand[T selectableItem](items []T) (string, func(), error) {
	previewDir, err := os.MkdirTemp("", "lazyworktree-fzf-preview-*")
	if err != nil {
		return "", func() {}, fmt.Errorf("failed to create fzf preview directory: %w", err)
	}
	cleanup := func() {
		_ = os.RemoveAll(previewDir) //nolint:gosec // previewDir comes from os.MkdirTemp.
	}

	for _, item := range items {
		previewPath := filepath.Join(previewDir, strconv.Itoa(item.ItemNumber()))
		// #nosec G306 - preview files only need to be readable by the current user.
		if err := os.WriteFile(previewPath, []byte(item.FormatPreview()), 0o600); err != nil {
			cleanup()
			return "", func() {}, fmt.Errorf("failed to write fzf preview file: %w", err)
		}
	}

	quotedDir := multiplexer.ShellQuote(previewDir)
	command := "num=$(printf '%s\\n' {1} | tr -cd '0-9'); " +
		"file=" + quotedDir + "/\"$num\"; " +
		"if [ -f \"$file\" ]; then cat \"$file\"; else echo 'No preview available'; fi"
	return command, cleanup, nil
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
		fmt.Fprintf(&sb, "%d) echo '%s';; ", item.ItemNumber(), preview)
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
// Issues that appear to have a worktree already, according to issueTemplate, are highlighted.
func SelectIssueInteractive(ctx context.Context, gitSvc gitService, query, issueTemplate string, stdin io.Reader, stderr io.Writer) (int, error) {
	fmt.Fprintf(stderr, "Fetching open issues...\n")

	issues, err := gitSvc.FetchAllOpenIssues(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch issues: %w", err)
	}
	if len(issues) == 0 {
		return 0, fmt.Errorf("no open issues found")
	}

	checkedOut := issuesWithWorktree(issues, listWorktreesForSelector(ctx, gitSvc), issueTemplate)
	selected, err := selectIssueFunc(issues, checkedOut, query, stdin, stderr)
	if err != nil {
		return 0, err
	}
	return selected.Number, nil
}

func selectIssueDefault(issues []*models.IssueInfo, checkedOut map[int]bool, query string, stdin io.Reader, stderr io.Writer) (*models.IssueInfo, error) {
	if _, err := fzfLookPath("fzf"); err == nil {
		return selectIssueWithFzf(issues, checkedOut, query, stderr)
	}
	return selectIssueWithPrompt(issues, checkedOut, query, stdin, stderr)
}

func selectIssueWithFzf(issues []*models.IssueInfo, checkedOut map[int]bool, query string, stderr io.Writer) (*models.IssueInfo, error) {
	items := wrapIssues(issues, checkedOut)
	selected, err := selectWithFzf(items, query, "Select issue> ", "Issue selection (type to filter)", "issue selection cancelled", "no issue selected", stderr)
	if err != nil {
		return nil, err
	}
	return selected.IssueInfo, nil
}

func selectIssueWithPrompt(issues []*models.IssueInfo, checkedOut map[int]bool, query string, stdin io.Reader, stderr io.Writer) (*models.IssueInfo, error) {
	items := wrapIssues(issues, checkedOut)
	selected, err := selectWithPrompt(items, query, "issue", stdin, stderr)
	if err != nil {
		return nil, err
	}
	return selected.IssueInfo, nil
}

// buildPreviewScript creates a shell script for issue previews (kept for test compatibility).
func buildPreviewScript(issues []*models.IssueInfo) string {
	return buildGenericPreviewScript(wrapIssues(issues, nil))
}

// SelectIssueInteractiveFromStdio wraps SelectIssueInteractive with os.Stdin/os.Stderr.
func SelectIssueInteractiveFromStdio(ctx context.Context, gitSvc gitService, query, issueTemplate string) (int, error) {
	return SelectIssueInteractive(ctx, gitSvc, query, issueTemplate, os.Stdin, os.Stderr)
}

// --- PR selectors (thin wrappers around generic helpers) ---

// SelectPRInteractive presents an interactive PR selector (fzf when available, numbered list otherwise).
func SelectPRInteractive(ctx context.Context, gitSvc gitService, query string, stdin io.Reader, stderr io.Writer) (int, error) {
	fmt.Fprintf(stderr, "Fetching open pull requests...\n")
	log.Printf("pr selector: fetching open pull requests query=%q", query)

	prs, err := gitSvc.FetchAllOpenPRs(ctx)
	if err != nil {
		log.Printf("pr selector: fetch failed: %v", err)
		return 0, fmt.Errorf("failed to fetch pull requests: %w", err)
	}
	log.Printf("pr selector: fetched %d open pull requests", len(prs))
	if len(prs) == 0 {
		return 0, fmt.Errorf("no open pull requests found")
	}

	checkedOut := prsWithWorktree(prs, listWorktreesForSelector(ctx, gitSvc))
	selected, err := selectPRFunc(prs, checkedOut, query, stdin, stderr)
	if err != nil {
		log.Printf("pr selector: selection failed: %v", err)
		return 0, err
	}
	log.Printf("pr selector: selected PR #%d", selected.Number)
	return selected.Number, nil
}

func selectPRDefault(prs []*models.PRInfo, checkedOut map[int]bool, query string, stdin io.Reader, stderr io.Writer) (*models.PRInfo, error) {
	if path, err := fzfLookPath("fzf"); err == nil {
		log.Printf("pr selector: using fzf at %q", path)
		return selectPRWithFzf(prs, checkedOut, query, stderr)
	} else {
		log.Printf("pr selector: fzf unavailable: %v; using numbered prompt", err)
	}
	return selectPRWithPrompt(prs, checkedOut, query, stdin, stderr)
}

func selectPRWithFzf(prs []*models.PRInfo, checkedOut map[int]bool, query string, stderr io.Writer) (*models.PRInfo, error) {
	items := wrapPRs(prs, checkedOut)
	selected, err := selectWithFzf(items, query, "Select PR> ", "Pull request selection (type to filter)", "pull request selection cancelled", "no pull request selected", stderr)
	if err != nil {
		return nil, err
	}
	return selected.PRInfo, nil
}

func selectPRWithPrompt(prs []*models.PRInfo, checkedOut map[int]bool, query string, stdin io.Reader, stderr io.Writer) (*models.PRInfo, error) {
	items := wrapPRs(prs, checkedOut)
	selected, err := selectWithPrompt(items, query, "pull request", stdin, stderr)
	if err != nil {
		return nil, err
	}
	return selected.PRInfo, nil
}

// buildPRPreviewScript creates a shell script for PR previews (kept for test compatibility).
func buildPRPreviewScript(prs []*models.PRInfo) string {
	return buildGenericPreviewScript(wrapPRs(prs, nil))
}

// SelectPRInteractiveFromStdio wraps SelectPRInteractive with os.Stdin/os.Stderr.
func SelectPRInteractiveFromStdio(ctx context.Context, gitSvc gitService, query string) (int, error) {
	return SelectPRInteractive(ctx, gitSvc, query, os.Stdin, os.Stderr)
}

// --- Worktree detection helpers ---

// listWorktreesForSelector returns existing worktrees, or nil on failure so selection can proceed unmarked.
func listWorktreesForSelector(ctx context.Context, gitSvc gitService) []*models.WorktreeInfo {
	worktrees, err := gitSvc.GetWorktrees(ctx)
	if err != nil {
		log.Printf("selector: failed to list worktrees, not marking checked-out items: %v", err)
		return nil
	}
	return worktrees
}

// prsWithWorktree returns the PR numbers whose head branch is already checked out in a worktree.
func prsWithWorktree(prs []*models.PRInfo, worktrees []*models.WorktreeInfo) map[int]bool {
	checkedOut := make(map[int]bool)
	for _, pr := range prs {
		branch := strings.TrimSpace(pr.Branch)
		if branch == "" {
			continue
		}
		if _, ok := findWorktreePathForBranch(worktrees, branch); ok {
			checkedOut[pr.Number] = true
		}
	}
	return checkedOut
}

// issuesWithWorktree returns the issue numbers that appear to have a worktree.
// Nothing records which issue a worktree came from, so this matches the literal
// template text before {number}, followed by the issue number, against branch
// names and worktree directory names. Templates without {number}, or with a
// placeholder before it, cannot be matched.
func issuesWithWorktree(issues []*models.IssueInfo, worktrees []*models.WorktreeInfo, template string) map[int]bool {
	checkedOut := make(map[int]bool)
	if strings.TrimSpace(template) == "" {
		template = defaultIssueBranchPattern
	}
	idx := strings.Index(template, "{number}")
	if idx < 0 {
		return checkedOut
	}
	prefix := template[:idx]
	if strings.Contains(prefix, "{title}") || strings.Contains(prefix, "{generated}") {
		return checkedOut
	}

	var names []string
	for _, wt := range worktrees {
		if wt.Branch != "" {
			names = append(names, wt.Branch)
		}
		if wt.Path != "" {
			names = append(names, filepath.Base(wt.Path))
		}
	}

	for _, issue := range issues {
		key := prefix + strconv.Itoa(issue.Number)
		for _, name := range names {
			rest, ok := strings.CutPrefix(name, key)
			if ok && (rest == "" || !unicode.IsDigit(rune(rest[0]))) {
				checkedOut[issue.Number] = true
				break
			}
		}
	}
	return checkedOut
}

// --- Conversion helpers ---

func wrapIssues(issues []*models.IssueInfo, checkedOut map[int]bool) []issueItem {
	items := make([]issueItem, len(issues))
	for i, issue := range issues {
		items[i] = issueItem{IssueInfo: issue, hasWorktree: checkedOut[issue.Number]}
	}
	return items
}

func wrapPRs(prs []*models.PRInfo, checkedOut map[int]bool) []prItem {
	items := make([]prItem, len(prs))
	for i, pr := range prs {
		items[i] = prItem{PRInfo: pr, hasWorktree: checkedOut[pr.Number]}
	}
	return items
}

// parseIssueNumberFromLine is an alias for parseNumberFromLine kept for test compatibility.
var parseIssueNumberFromLine = parseNumberFromLine
