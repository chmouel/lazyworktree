# Setting Worktree Notes and Auto-Generated Context

Configure `worktree_note_script` to generate initial worktree notes when creating from a PR/MR or issue. The script receives the selected item's title and body on stdin and can produce multiline output.

<div class="mint-callout">
  <p><strong>Use this page when:</strong> you want to keep clear context per worktree and auto-generate starter notes from PRs or issues.</p>
</div>

## Why notes matter

Worktree notes make it easy to remember context when switching between branches.

A short note can capture:

- what this worktree is for
- what has already been done
- what should happen next

This is especially useful when you return to a worktree after a few days, or when you are juggling multiple PRs in parallel.

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
worktree_note_script: "aichat -m gemini:gemini-2.5-flash-lite 'Summarise this ticket into practical implementation notes.'"
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

- `$REPO_NAME` — `owner/repo` combined (e.g. `myorg/myrepo`) — convenient shorthand for a two-level directory
- `$REPO_OWNER` — repository owner only (e.g. `myorg`)
- `$REPO_REPONAME` — repository name only (e.g. `myrepo`)
- `$WORKTREE_NAME` — worktree directory basename

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
  -m gemini:gemini-2.5-flash-lite \
  'Create concise developer worktree notes with context, done items, and next actions.'
```

## Script Requirements

| Requirement | Detail |
| --- | --- |
| **Input** | Receives PR/issue title and description on stdin |
| **Output** | Writes the note text to stdout (can be multiline) |
| **Timeout** | Must complete within 30 seconds — scripts exceeding this are terminated |
| **Failure** | If the script fails or outputs nothing, worktree creation continues normally and no note is saved |

The script runs silently — there is no visible progress indicator whilst it executes. If you notice a brief pause during worktree creation, the note script is likely running.

## Environment Variables

The following variables are set when the script executes:

| Variable | Description | Example values |
| --- | --- | --- |
| `LAZYWORKTREE_TYPE` | Type of item being processed | `pr`, `issue` |
| `LAZYWORKTREE_NUMBER` | PR or issue number | `42` |
| `LAZYWORKTREE_TITLE` | Title of the PR or issue | `Add session management` |
| `LAZYWORKTREE_URL` | URL of the PR or issue | `https://github.com/org/repo/pull/42` |

You can use these variables to vary behaviour by context — for example, generating more detailed notes for issues than for PRs.

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

- [Notes and Taskboard](core/notes-and-taskboard.md) — viewing, editing, and using the taskboard with worktree notes
- [AI Integration](guides/ai-integration.md) — full AI tool setup, branch naming scripts, and troubleshooting
