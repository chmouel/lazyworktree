# AI Integration

LazyWorktree can integrate with AI tools to automatically generate branch names and worktree notes. AI features are completely optional.

<div class="mint-callout">
  <p><strong>Use this page when:</strong> you want to automate branch naming from PR/issue titles or generate implementation notes from descriptions.</p>
</div>

## Automatic Branch Names

When creating worktrees from PRs, issues, or uncommitted diffs, LazyWorktree can pipe the content to an AI tool to generate concise, semantic branch names.

### Configuration

For PRs and issues:

```yaml
branch_name_script: "aichat -m gemini:gemini-2.5-flash-lite 'Generate a short title for this PR or issue. Output only the title (like feat-session-manager), nothing else.'"
pr_branch_name_template: "pr-{number}-{generated}"
issue_branch_name_template: "issue-{number}-{generated}"
```

For uncommitted diffs:

```yaml
branch_name_script: "aichat -m gemini:gemini-2.5-flash-lite 'Generate a short git branch name (no spaces, use hyphens) for this diff. Output only the branch name, nothing else.'"
```

### Template Placeholders

| Placeholder | Description | Example |
| --- | --- | --- |
| `{number}` | PR or issue number | `42` |
| `{title}` | Original title, sanitised for Git | `add-ai-session-management` |
| `{generated}` | AI-generated title (falls back to `{title}` on failure) | `feat-ai-session-manager` |
| `{pr_author}` | PR author username (PR templates only) | `alice` |

### Template Examples

Assuming AI generates `feat-ai-session-manager` for Issue #2:

| Template | Result |
| --- | --- |
| `issue-{number}-{title}` | `issue-2-add-ai-session-management` |
| `issue-{number}-{generated}` | `issue-2-feat-ai-session-manager` |
| `pr-{number}-{generated}` | `pr-7-feat-ai-session-manager` |
| `pr-{number}-{pr_author}-{title}` | `pr-7-alice-add-ai-session-management` |
| `{generated}` | `feat-ai-session-manager` |

!!! tip
    If the AI script fails or times out, `{generated}` automatically falls back to `{title}`. Workflow continues uninterrupted.

### Script Requirements

Your `branch_name_script` must:

- **Read from stdin** — LazyWorktree pipes PR/issue content or diffs
- **Write to stdout** — output the branch name (first line is used)
- **Complete within 30 seconds** — scripts exceeding this timeout are terminated

### Environment Variables for Scripts

| Variable | Description | Values |
| --- | --- | --- |
| `LAZYWORKTREE_TYPE` | Type of item being processed | `pr`, `issue`, or `diff` |
| `LAZYWORKTREE_NUMBER` | PR or issue number | `42` (empty for diffs) |
| `LAZYWORKTREE_TEMPLATE` | The template being used | `pr-{number}-{generated}` |
| `LAZYWORKTREE_SUGGESTED_NAME` | LazyWorktree's default suggestion | `pr-42-add-feature` |

You can use these variables to vary behaviour by context:

```bash
branch_name_script: |
  if [ "$LAZYWORKTREE_TYPE" = "diff" ]; then
    aichat -m gemini:gemini-2.5-flash-lite 'Generate a complete branch name for this diff'
  else
    aichat -m gemini:gemini-2.5-flash-lite 'Generate a short title (no issue-/pr- prefix) for this issue or PR'
  fi
```

## Automatic Worktree Notes

When creating worktrees from PRs or issues, AI can summarise the description into practical implementation notes, providing immediate context when switching worktrees.

### Configuration

```yaml
worktree_note_script: "aichat -m gemini:gemini-2.5-flash-lite 'Summarise this ticket into practical implementation notes.'"
```

The script receives the PR/issue title and body on stdin and outputs the worktree note to stdout.

### Script Requirements

Your `worktree_note_script` must:

- **Read from stdin** — receives PR/issue title and description
- **Write to stdout** — output the note text (can be multiline)
- **Complete within 30 seconds** — timeout enforced

If the note script fails or outputs nothing, worktree creation continues normally without saving a note.

## AI Tool Setup

### Using aichat

[aichat](https://github.com/sigoden/aichat) is a CLI tool supporting multiple AI providers.

```bash
# macOS
brew install aichat

# Linux
cargo install aichat
```

LazyWorktree configuration:

```yaml
branch_name_script: "aichat -m gemini:gemini-2.5-flash-lite 'Generate a short branch name. Output only the name.'"
worktree_note_script: "aichat -m gemini:gemini-2.5-flash-lite 'Summarise into practical notes.'"
```

### Using Claude Code

```yaml
branch_name_script: "claude code --prompt 'Generate a short branch name for this content. Output only the name.'"
```

### Custom Scripts

Any script or tool that reads stdin and writes stdout works:

```yaml
branch_name_script: "/path/to/custom-ai-script.sh"
```

```bash
#!/bin/bash
# custom-ai-script.sh
content=$(cat)
curl -s https://api.your-ai-service.com/generate \
  -d "{\"prompt\": \"Generate branch name\", \"content\": \"$content\"}" \
  | jq -r '.result'
```

## Performance Tips

- **Use smaller models** — branch naming does not require powerful models. `gemini-2.5-flash-lite`, `gpt-3.5-turbo`, or similar fast models complete in under 2 seconds
- **Limit context** — if PR descriptions are very long, truncate in your script:

    ```bash
    head -n 50 | aichat -m gemini:gemini-2.5-flash-lite 'Generate name'
    ```

## Troubleshooting

### Script times out after 30 seconds

Your AI provider may be too slow. Try a faster model (e.g., flash variants), reduce the context sent, or check network connectivity.

### {generated} always uses fallback

Your script is failing. Test it manually:

```bash
echo "Test PR title\n\nTest description" | aichat -m gemini:gemini-2.5-flash-lite 'Generate a branch name'
```

Check for API authentication issues, network errors, or model availability.

### Worktree notes not appearing

Verify that `worktree_note_script` is defined in your config and that the script runs successfully when tested manually. If using `worktree_notes_path`, ensure you are viewing notes from the correct storage location.

### Notes not syncing across machines

Ensure `worktree_notes_path` is set to a file in your repository, the JSON file is committed and pushed, and other machines have pulled the latest changes.
