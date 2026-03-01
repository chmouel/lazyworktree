# Automatically Generated Worktree Notes

Configure `worktree_note_script` to generate initial worktree notes when creating from a PR/MR or issue. The script receives the selected item's title and body on stdin and can produce multiline output.

<div class="mint-callout">
  <p><strong>Use this page when:</strong> you want to automate note generation for new worktrees created from PRs or issues.</p>
</div>

## Configuration

```yaml
worktree_note_script: "aichat -m gemini:gemini-2.5-flash-lite 'Summarise this ticket into practical implementation notes.'"
```

To store notes in a single synchronisable JSON file rather than git config:

```yaml
worktree_notes_path: ".lazyworktree/notes.json"
```

When `worktree_notes_path` is set, note keys are stored relative to `worktree_dir` instead of absolute filesystem paths, making the file portable across machines and clones.

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
