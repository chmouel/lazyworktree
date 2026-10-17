# Setting Worktree Notes and Auto-Generated Context

Configure `worktree_note_script` to generate initial worktree notes when creating from a PR/MR or issue. The script receives the selected item's title and body on stdin and can produce multiline output.


## Why notes matter

Worktree notes keep context when switching between branches.

Notes record:

- what this worktree is for
- what has already been done
- what should happen next

This matters most when returning after days away, or when several PRs are active at once.

## Setting notes manually

Open a worktree and press `i` to edit its note.

Keep the note brief and practical, for example:

```markdown
## Context
- PR: #142
- Goal: finish retry handling in webhook reconciliation

## Next actions
- add failing test for timeout branch
- update retry backoff to cap at 30s
```

## Auto-generating notes with `worktree_note_script`

```yaml
worktree_note_script: "aichat -m gemini:gemini-3.5-flash-lite 'Summarise this ticket into practical implementation notes.'"
```

To store notes in a single synchronisable JSON file rather than git config:

```yaml
worktree_notes_path: ".lazyworktree/notes.json"
```

When `worktree_notes_path` is set, note keys are stored relative to `worktree_dir` instead of absolute filesystem paths, making the file portable across machines and clones.

### Splitted mode

For individual markdown files instead of a single JSON file, use the `splitted` note type. Each worktree note becomes a separate file with YAML frontmatter (icon, color, timestamp) and a markdown body:

```yaml
worktree_note_type: splitted
worktree_notes_path: "~/notes/$REPO_OWNER/$REPO_REPONAME/$WORKTREE_NAME.md"
```

Template variables:

- `$REPO_NAME`: combined `owner/repo` (e.g. `myorg/myrepo`), a shorthand for a two-level directory
- `$REPO_OWNER`: repository owner only (e.g. `myorg`)
- `$REPO_REPONAME`: repository name only (e.g. `myrepo`)
- `$WORKTREE_NAME`: worktree directory basename

Note files use this format:

```markdown
---
icon: "⚡"
color: red
updated_at: 1709740800
---
The note content here.
Supports **markdown**.
```

This makes notes easy to browse, search, and edit with external tools.

#### Concurrent edits with the `splitted` mode

The `splitted` mode supports concurrent access from multiple processes or clones. Saving or removing a note changes only that worktree's file. Other note files remain untouched, including files created after LazyWorktree last read the directory. A stale in-memory view cannot delete or overwrite another process's note. You must remove notes explicitly; batch saves do not remove them as a side effect.

## Example script: PR details to LLM note

For more control, point `worktree_note_script` to your own script. The script can receive PR title/body on stdin, call your preferred LLM, and return a concise note.

```yaml
worktree_note_script: "$HOME/.config/lazyworktree/scripts/worktree-note-from-pr.sh"
```

```bash
#!/usr/bin/env bash
set -euo pipefail

PR_CONTENT=$(cat)

printf '%s\n' "$PR_CONTENT" | aichat \
  -m gemini:gemini-3.5-flash-lite \
  'Create concise developer worktree notes with context, done items, and next actions.'
```

## Script Requirements

| Requirement | Detail |
| --- | --- |
| **Input** | Receives PR/issue title and description on stdin |
| **Output** | Writes the note text to stdout (can be multiline) |
| **Timeout** | Scripts must finish within 30 seconds. LazyWorktree terminates scripts that exceed this limit. |
| **Failure** | If the script fails or outputs nothing, worktree creation continues normally and no note is saved |

The script runs without a progress indicator. A brief pause during worktree creation may mean the note script is running.

## Environment Variables

The following variables are set when the script executes:

| Variable | Description | Example values |
| --- | --- | --- |
| `LAZYWORKTREE_TYPE` | Type of item being processed | `pr`, `issue` |
| `LAZYWORKTREE_NUMBER` | PR or issue number | `42` |
| `LAZYWORKTREE_TITLE` | Title of the PR or issue | `Add session management` |
| `LAZYWORKTREE_URL` | URL of the PR or issue | `https://github.com/org/repo/pull/42` |
| `LAZYWORKTREE_DESCRIPTION` | Body/description of the PR or issue | `Adds session management…` |

Use these variables to change behaviour by context, such as generating more detailed notes for issues than for PRs.

## Practical Example

Given a PR titled "Add session management" with a description covering authentication tokens and timeout handling:

```bash
# The script receives on stdin:
# Add session management
#
# This PR implements session management with JWT tokens.
# Sessions expire after 30 minutes of inactivity.
# Includes refresh token rotation.

# The script outputs to stdout:
# ## Implementation Notes
# - JWT-based session tokens with 30-min inactivity timeout
# - Refresh token rotation on each use
# - Key files: auth/session.go, middleware/auth.go
```

The output is saved as the worktree note and can be viewed by pressing `i` on the worktree.

## Related Pages

- [Notes and Taskboard](core/notes-and-taskboard.md): view, edit, and use the taskboard with worktree notes
- [AI Integration](guides/ai-integration.md): configure AI tools, branch naming scripts, and troubleshooting
