# Branch Naming Conventions

Special characters are converted to hyphens for Git compatibility. Leading/trailing hyphens are removed, consecutive hyphens collapsed. Length capped at 50 (manual) or 100 (auto) characters.

| Input | Converted |
|-------|-----------|
| `feature.new` | `feature-new` |
| `bug fix here` | `bug-fix-here` |
| `feature:test` | `feature-test` |

## Automatically Generated Branch Names

Configure `branch_name_script` to generate names via tools like [aichat](https://github.com/sigoden/aichat/) or [claude code](https://claude.com/product/claude-code). Issues/PRs output to `{generated}` placeholder; diffs output complete names.

!!! note
    Smaller, faster models are usually sufficient for branch names.

### Configuration

```yaml
# For PRs/issues: generate a title (available via {generated} placeholder)
branch_name_script: "aichat -m gemini:gemini-2.5-flash-lite 'Generate a short title for this PR or issue. Output only the title (like feat-session-manager), nothing else.'"

# Use the generated title in PR branch/worktree naming
pr_branch_name_template: "pr-{number}-{generated}"

# For diffs: generate a complete branch name
# branch_name_script: "aichat -m gemini:gemini-2.5-flash-lite 'Generate a short git branch name (no spaces, use hyphens) for this diff. Output only the branch name, nothing else.'"
```

### Template Placeholders

* `{number}` - PR/issue number
* `{title}` - Original sanitised title
* `{generated}` - Generated title (falls back to `{title}`)
* `{pr_author}` - PR author username (PR templates only)

**Examples:**

| Template | Result | Generated: `feat-ai-session-manager` |
|----------|--------|--------------------------------------|
| `issue-{number}-{title}` | `issue-2-add-ai-session-management` (Issue #2) | Not used |
| `issue-{number}-{generated}` | `issue-2-feat-ai-session-manager` (Issue #2) | Used |
| `pr-{number}-{generated}` | `pr-7-feat-ai-session-manager` (PR #7) | Used |
| `pr-{number}-{pr_author}-{title}` | `pr-7-alice-add-ai-session-management` (PR #7 by @alice) | Not used |

If script fails, `{generated}` falls back to `{title}`.

### Script Requirements

Receives content on stdin, outputs branch name on stdout (first line). Timeout: 30s.

### Environment Variables

`LAZYWORKTREE_TYPE` (pr/issue/diff), `LAZYWORKTREE_NUMBER`, `LAZYWORKTREE_TEMPLATE`, `LAZYWORKTREE_SUGGESTED_NAME`.

**Example:**

```yaml
# Different prompts for different types
branch_name_script: |
  if [ "$LAZYWORKTREE_TYPE" = "diff" ]; then
    aichat -m gemini:gemini-2.5-flash-lite 'Generate a complete branch name for this diff'
  else
    aichat -m gemini:gemini-2.5-flash-lite 'Generate a short title (no issue-/pr- prefix) for this issue or PR'
  fi

# Use issue/PR number in the prompt
branch_name_script: |
  aichat -m gemini:gemini-2.5-flash-lite "Generate a title for item #$LAZYWORKTREE_NUMBER. Output only the title."
```
