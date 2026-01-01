package app

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/chmouel/lazyworktree/internal/models"
)

// runBranchNameScript executes the configured branch_name_script with the diff as stdin.
// It returns the generated branch name or an error.
func runBranchNameScript(ctx context.Context, script, diff string) (string, error) {
	if script == "" {
		return "", nil
	}

	// Create a context with timeout to prevent hanging
	const scriptTimeout = 30 * time.Second
	ctx, cancel := context.WithTimeout(ctx, scriptTimeout)
	defer cancel()

	// #nosec G204 -- script is user-configured and trusted
	cmd := exec.CommandContext(ctx, "sh", "-c", script)
	cmd.Stdin = strings.NewReader(diff)

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

// generatePRWorktreeName creates a worktree name from a PR in the format pr{number}-{sanitized-title}
// The name is sanitized to be a valid git branch name and truncated to 100 characters.
func generatePRWorktreeName(pr *models.PRInfo) string {
	// Start with pr{number}-
	name := fmt.Sprintf("pr%d", pr.Number)

	// Sanitize the title
	title := strings.ToLower(pr.Title)

	// Replace spaces and special characters with hyphens
	re := regexp.MustCompile(`[^a-z0-9]+`)
	title = re.ReplaceAllString(title, "-")

	// Remove leading/trailing hyphens and consecutive hyphens
	title = strings.Trim(title, "-")
	re2 := regexp.MustCompile(`-+`)
	title = re2.ReplaceAllString(title, "-")

	// Combine: pr{number}-{title}
	if title != "" {
		name = name + "-" + title
	}

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
