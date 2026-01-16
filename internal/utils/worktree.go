package utils

import (
	"fmt"
	"strings"

	"github.com/chmouel/lazyworktree/internal/models"
)

// GeneratePRWorktreeName generates a worktree name from a PR using a template.
// Supports placeholders: {number}, {title}, {generated}, {pr_author}
func GeneratePRWorktreeName(pr *models.PRInfo, template, generatedTitle string) string {
	// Sanitize all components (no length limit here, will truncate final result)
	title := SanitizeBranchName(pr.Title, 0)
	generated := title
	if generatedTitle != "" {
		generated = SanitizeBranchName(generatedTitle, 0)
	}
	author := SanitizeBranchName(pr.Author, 0)

	replacements := []placeholderReplacement{
		{placeholder: "{number}", value: fmt.Sprintf("%d", pr.Number)},
		{placeholder: "{title}", value: title},
		{placeholder: "{generated}", value: generated},
		{placeholder: "{pr_author}", value: author},
	}

	return applyWorktreeTemplate(template, replacements)
}

// GenerateIssueWorktreeName generates a worktree name from an issue using a template.
// Supports placeholders: {number}, {title}, {generated}
func GenerateIssueWorktreeName(issue *models.IssueInfo, template, generatedTitle string) string {
	// Sanitize all components (no length limit here, will truncate final result)
	title := SanitizeBranchName(issue.Title, 0)
	generated := title
	if generatedTitle != "" {
		generated = SanitizeBranchName(generatedTitle, 0)
	}

	replacements := []placeholderReplacement{
		{placeholder: "{number}", value: fmt.Sprintf("%d", issue.Number)},
		{placeholder: "{title}", value: title},
		{placeholder: "{generated}", value: generated},
	}

	return applyWorktreeTemplate(template, replacements)
}

type placeholderReplacement struct {
	placeholder string
	value       string
}

func applyWorktreeTemplate(template string, replacements []placeholderReplacement) string {
	name := template
	for _, replacement := range replacements {
		name = strings.ReplaceAll(name, replacement.placeholder, replacement.value)
	}

	// Remove trailing hyphens that might result from empty title.
	name = strings.TrimRight(name, "-")

	// Truncate to 100 characters.
	if len(name) > 100 {
		name = strings.TrimRight(name[:100], "-")
	}

	return name
}
