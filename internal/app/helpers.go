package app

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/chmouel/lazyworktree/internal/models"
)

// runBranchNameScript executes the configured branch_name_script with the content as stdin.
// It returns the generated branch name or an error.
// The scriptType indicates the context: "pr", "issue", or "diff".
// For PRs and issues, number, template, and suggestedName provide additional context.
func runBranchNameScript(ctx context.Context, script, content, scriptType, number, template, suggestedName string) (string, error) {
	if script == "" {
		return "", nil
	}

	// Create a context with timeout to prevent hanging
	const scriptTimeout = 30 * time.Second
	ctx, cancel := context.WithTimeout(ctx, scriptTimeout)
	defer cancel()

	// #nosec G204 -- script is user-configured and trusted
	cmd := exec.CommandContext(ctx, "bash", "-c", script)
	cmd.Stdin = strings.NewReader(content)

	// Set environment variables to provide context to the script
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("LAZYWORKTREE_TYPE=%s", scriptType),
		fmt.Sprintf("LAZYWORKTREE_NUMBER=%s", number),
		fmt.Sprintf("LAZYWORKTREE_TEMPLATE=%s", template),
		fmt.Sprintf("LAZYWORKTREE_SUGGESTED_NAME=%s", suggestedName),
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("branch name script failed: %w (stderr: %s)", err, stderr.String())
	}

	// Trim whitespace and get first line only
	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return "", nil
	}

	// Get only the first line
	if idx := strings.IndexAny(output, "\n\r"); idx >= 0 {
		output = output[:idx]
	}

	return strings.TrimSpace(output), nil
}

type scoredPaletteItem struct {
	item  paletteItem
	score int
}

func filterPaletteItems(items []paletteItem, query string) []paletteItem {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return items
	}

	scored := make([]scoredPaletteItem, 0, len(items))
	for _, it := range items {
		// Skip sections when filtering
		if it.isSection {
			continue
		}
		score, ok := paletteMatchScore(q, it)
		if !ok {
			continue
		}
		scored = append(scored, scoredPaletteItem{item: it, score: score})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].score < scored[j].score
	})

	filtered := make([]paletteItem, len(scored))
	for i, scoredItem := range scored {
		filtered[i] = scoredItem.item
	}
	return filtered
}

func paletteMatchScore(queryLower string, item paletteItem) (int, bool) {
	if queryLower == "" {
		return 0, true
	}

	label := strings.ToLower(item.label)
	desc := strings.ToLower(item.description)

	bestScore := 0
	matched := false

	if score, ok := fuzzyScoreLower(queryLower, label); ok {
		matched = true
		if strings.Contains(label, queryLower) {
			score -= 5
		}
		bestScore = score
	}

	if score, ok := fuzzyScoreLower(queryLower, desc); ok {
		score += 15
		if strings.Contains(desc, queryLower) {
			score -= 3
		}
		if !matched || score < bestScore {
			matched = true
			bestScore = score
		}
	}

	return bestScore, matched
}

func fuzzyScoreLower(query, target string) (int, bool) {
	if query == "" {
		return 0, true
	}

	qRunes := []rune(query)
	tRunes := []rune(target)
	if len(qRunes) == 0 {
		return 0, true
	}

	score := 0
	lastIdx := -1
	searchFrom := 0

	for _, qc := range qRunes {
		found := false
		for i := searchFrom; i < len(tRunes); i++ {
			if tRunes[i] == qc {
				if lastIdx >= 0 {
					gap := i - lastIdx - 1
					score += gap * 2
					if gap == 0 {
						score--
					}
				} else {
					score += i * 2
				}
				lastIdx = i
				searchFrom = i + 1
				found = true
				break
			}
		}
		if !found {
			return 0, false
		}
	}

	return score, true
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// generatePRWorktreeName creates a worktree name from a PR using a template.
// The template supports placeholders: {number}, {title}, {generated}
// The name is sanitized to be a valid git branch name and truncated to 100 characters.
// generatedTitle is the AI-generated title (optional). If empty, {generated} falls back to {title}.
func generatePRWorktreeName(pr *models.PRInfo, template, generatedTitle string) string {
	// Sanitize the original title
	title := strings.ToLower(pr.Title)

	// Replace spaces and special characters with hyphens
	re := regexp.MustCompile(`[^a-z0-9]+`)
	title = re.ReplaceAllString(title, "-")

	// Remove leading/trailing hyphens and consecutive hyphens
	title = strings.Trim(title, "-")
	re2 := regexp.MustCompile(`-+`)
	title = re2.ReplaceAllString(title, "-")

	// Sanitize the generated title (same process)
	generated := title // fallback to original title
	if generatedTitle != "" {
		generated = strings.ToLower(generatedTitle)
		generated = re.ReplaceAllString(generated, "-")
		generated = strings.Trim(generated, "-")
		generated = re2.ReplaceAllString(generated, "-")
	}

	// Replace placeholders in template
	name := template
	name = strings.ReplaceAll(name, "{number}", fmt.Sprintf("%d", pr.Number))
	name = strings.ReplaceAll(name, "{title}", title)
	name = strings.ReplaceAll(name, "{generated}", generated)

	// Remove trailing hyphens that might result from empty title
	// This handles cases like "{prefix}{number}-{title}" when title is empty
	name = strings.TrimRight(name, "-")

	// Truncate to 100 characters
	if len(name) > 100 {
		name = name[:100]
		// Make sure we don't end with a hyphen after truncation
		name = strings.TrimRight(name, "-")
	}

	return name
}

// generateIssueWorktreeName creates a worktree name from an issue using a template.
// The template supports placeholders: {number}, {title}, {generated}
// The name is sanitized to be a valid git branch name and truncated to 100 characters.
// generatedTitle is the AI-generated title (optional). If empty, {generated} falls back to {title}.
func generateIssueWorktreeName(issue *models.IssueInfo, template, generatedTitle string) string {
	// Sanitize the original title
	title := strings.ToLower(issue.Title)

	// Replace spaces and special characters with hyphens
	re := regexp.MustCompile(`[^a-z0-9]+`)
	title = re.ReplaceAllString(title, "-")

	// Remove leading/trailing hyphens and consecutive hyphens
	title = strings.Trim(title, "-")
	re2 := regexp.MustCompile(`-+`)
	title = re2.ReplaceAllString(title, "-")

	// Sanitize the generated title (same process)
	generated := title // fallback to original title
	if generatedTitle != "" {
		generated = strings.ToLower(generatedTitle)
		generated = re.ReplaceAllString(generated, "-")
		generated = strings.Trim(generated, "-")
		generated = re2.ReplaceAllString(generated, "-")
	}

	// Replace placeholders in template
	name := template
	name = strings.ReplaceAll(name, "{number}", fmt.Sprintf("%d", issue.Number))
	name = strings.ReplaceAll(name, "{title}", title)
	name = strings.ReplaceAll(name, "{generated}", generated)

	// Remove trailing hyphens that might result from empty title
	// This handles cases like "{prefix}{number}-{title}" when title is empty
	name = strings.TrimRight(name, "-")

	// Truncate to 100 characters
	if len(name) > 100 {
		name = name[:100]
		// Make sure we don't end with a hyphen
		name = strings.TrimRight(name, "-")
	}

	return name
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

type scoredSuggestion struct {
	suggestion string
	score      int
}

// filterInputSuggestions filters a list of suggestions using fuzzy matching
// and returns them sorted by relevance score.
func filterInputSuggestions(suggestions []string, query string) []string {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return suggestions
	}

	scored := make([]scoredSuggestion, 0, len(suggestions))
	for _, suggestion := range suggestions {
		if score, ok := fuzzyScoreLower(q, strings.ToLower(suggestion)); ok {
			scored = append(scored, scoredSuggestion{suggestion: suggestion, score: score})
		}
	}

	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].score < scored[j].score
	})

	filtered := make([]string, len(scored))
	for i, scoredSuggestion := range scored {
		filtered[i] = scoredSuggestion.suggestion
	}
	return filtered
}

// formatRelativeTime formats a time as a human-readable relative string.
func formatRelativeTime(t time.Time) string {
	d := time.Since(t)

	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case d < 24*time.Hour:
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case d < 7*24*time.Hour:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "yesterday"
		}
		return fmt.Sprintf("%d days ago", days)
	case d < 30*24*time.Hour:
		weeks := int(d.Hours() / 24 / 7)
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	default:
		return t.Format("Jan 2, 2006")
	}
}

// formatCreateFromCurrentLabel formats the "Create from current" menu label
// with the current branch name, applying ellipsis if the total length exceeds maxLength.
func formatCreateFromCurrentLabel(branch string) string {
	const maxLength = 78
	const baseLabel = "Create from current"

	if branch == "" {
		return baseLabel
	}

	labelWithBranch := fmt.Sprintf("%s (%s)", baseLabel, branch)
	if len(labelWithBranch) <= maxLength {
		return labelWithBranch
	}

	// Truncate to maxLength - 3 (for "...") and append ellipsis
	truncated := labelWithBranch[:maxLength-3]
	return truncated + "..."
}
