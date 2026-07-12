# AI Integration

LazyWorktree can integrate with AI tools to automatically generate branch names and worktree notes. AI features are completely optional.

## For Coding Agents

If you are a coding agent (Claude Code, Copilot, etc.) or writing automation that shells out to `lazyworktree`, follow this hierarchy:

### 1. Introspect with `describe`

Use `describe` to discover flags and subcommands without parsing `--help`:

```bash
# Full CLI structure
lazyworktree describe | jq '.subcommands[].name'

# Flags for a specific command
lazyworktree describe create | jq '.flags[].name'

# Check a nested subcommand
lazyworktree describe note show | jq '.'
```

`describe` always emits valid JSON and exits 0 on success. The output is stable and safe to parse programmatically.

### 2. Verify setup with `doctor`

Use `doctor --json` before assuming repository state or helper tools are available:

```bash
lazyworktree doctor --json | jq '{repo: .repository.repo, worktrees: .repository.worktree_count, git: .tools.git.available}'
```

`doctor --json` reports config loading, repository detection, worktree visibility, and helper tool availability. It remains usable even when the current directory is not ready for full worktree operations.

### 3. Resolve and read exact worktrees

Prefer resolving once, then reading exact objects:

```bash
lazyworktree worktrees list --json | jq '.items[].name'
lazyworktree worktrees resolve --name my-feature --json | jq -r '.worktree.path'
lazyworktree worktrees get my-feature --json | jq '{path, branch, dirty}'
lazyworktree worktrees context my-feature --json | jq '{worktree, note}'
lazyworktree notes get my-feature --json | jq '{note, description, tags}'
```

These commands keep discovery, resolution, and exact reads separate so follow-up commands do not have to repeat broad searches.

### 4. Use `--json` for existing lifecycle commands

Every mutating command (`create`, `delete`, `rename`) and `note show` accept `--json`. In this mode:

- Structured JSON is written to **stdout**
- Human-readable progress messages go to **stderr**
- Exit code reflects success/failure, not the JSON content

```bash
# Create a worktree and capture its path
path=$(lazyworktree create my-feature --json | jq -r '.path')

# Delete and confirm what was removed
lazyworktree delete my-feature --json | jq '{name, path, branch_deleted}'

# List worktrees with agent session data
lazyworktree list --json | jq '.[].agent_count'

# Show note metadata
lazyworktree note show --json | jq '{note, description, tags}'
```

When agent session data is present, each session includes `liveness` and `source` fields so automation can distinguish confirmed active sessions from recent or heuristic matches.

```bash
# Show only confirmed-active sessions across all worktrees
lazyworktree list --json | jq '[.[] | .agent_sessions[] | select(.liveness == "active")]'

# Check whether any active session is open for a specific worktree
lazyworktree worktrees context my-feature --json | jq '.agent_sessions[] | select(.liveness == "active") | .activity'
```

| `liveness` value | Meaning |
|---|---|
| `"active"` | Confirmed running: matched to a live process |
| `"recent"` | No live process found, but activity within the last 10 minutes |
| `"suspect"` | Matched by working-directory heuristic only — less certain |
| `"inactive"` | No live process and no recent activity |

| `source` value | Meaning |
|---|---|
| `"native"` | Session reported itself as active (e.g. Claude Code's own status) |
| `"hook"` | An agent lifecycle hook reported the session and its process is alive |
| `"exact_file"` | Live process has the session transcript file open |
| `"cwd_heuristic"` | Live process working directory matches the worktree path |
| `"registry"` | Recovered from the persistent registry after a parse failure |
| `"none"` | No liveness signal found |

### Precise session tracking with lifecycle hooks

For the most accurate and lowest-cost session tracking, install the agent
lifecycle hooks:

```bash
lazyworktree setup-hooks --dry-run   # preview the changes
lazyworktree setup-hooks             # install
```

This wires Claude Code (`~/.claude/settings.json`), the Codex CLI
(`$CODEX_HOME/hooks.json`, or `~/.codex/hooks.json` when `CODEX_HOME` is
unset), and the Copilot CLI (`$COPILOT_HOME/hooks/lazyworktree.json`, or
`~/.copilot/hooks/lazyworktree.json` when `COPILOT_HOME` is unset) to report
session lifecycle events directly to lazyworktree. Hook-tracked sessions
carry the agent process id, so liveness is confirmed with a cheap PID probe,
and Codex CLI and Copilot CLI sessions become visible in the agents pane. See
[setup-hooks](../cli/setup-hooks.md) for details, including the Codex `/hooks`
approval step. Claude Code and Copilot CLI hooks also report when a question
dialog opens and when its answer returns, so the pane switches from
**thinking** to **waiting** while the agent needs your input. Codex CLI does
not currently expose an equivalent question-dialog hook.

Hooks are the default liveness source. The former process-table scan
(ps/lsof) is deprecated and disabled by default; set
`agent_sessions.process_scan: true` in the configuration to opt back in.

### 5. Use `exec --json` for command automation

```bash
# Run a command in a worktree and capture the exit code
result=$(lazyworktree exec --workspace my-feature --json "make test")
echo "$result" | jq '.exit_code'
```

### Introspection hierarchy

| Method | When to use |
|---|---|
| `describe` | Discover available flags and subcommands |
| `doctor --json` | Verify config, repo visibility, and helper tools |
| `worktrees ... --json` | Discover, resolve, and read exact worktree state |
| `notes get --json` | Read exact note metadata for one worktree |
| `--json` flags | Parse command results programmatically |
| `--help` | Human-readable reference only — do not parse |


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
