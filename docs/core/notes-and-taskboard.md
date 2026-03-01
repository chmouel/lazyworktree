# Notes and Taskboard

Worktree notes keep implementation context close to the worktree itself.
Taskboard extracts markdown checkboxes into a grouped actionable view.

<div class="mint-callout">
  <p><strong>Use this page when:</strong> you want to track context, TODOs, and progress per worktree.</p>
</div>

## Notes Behaviour

Press `i` on a selected worktree:

- if a note exists: opens note viewer first
- if no note exists: opens note editor

Viewer controls include scrolling, half-page navigation, and quick edit entry.
Editor supports save, external editor handoff, newline insertion, and cancel.

When a note exists for a worktree, a note marker appears in the worktree list and the Notes pane (pane 5) becomes visible.

<div class="lw-media-grid">
  <figure>
    <img alt="Worktree notes authoring" src="../assets/screenshot-annotations.png" loading="lazy" />
    <figcaption>Worktree notes authoring and context capture.</figcaption>
  </figure>
  <figure>
    <img alt="Rendered markdown notes" src="../assets/screenshot-rendered-notes.png" loading="lazy" />
    <figcaption>Rendered markdown notes with highlighted tags.</figcaption>
  </figure>
</div>

## Markdown Rendering

The info pane renders common markdown elements, including:

- headings (styled hierarchically)
- ordered and unordered lists
- quotes
- inline code and fenced code blocks (syntax highlighted with delta)
- links (highlighted)

Uppercase tags like `TODO`, `FIXME`, and `WARNING:` are highlighted (outside fenced code blocks).

## Taskboard

Press `T` to open Taskboard. It sources markdown checkboxes from worktree notes and presents them in a grouped, actionable view.

![Taskboard view](../assets/screenshot-todolist.png)

### Taskboard Keybindings

| Key | Action |
| --- | --- |
| `j` / `k` | Move between tasks |
| `Enter` or `Space` | Toggle task completion |
| `a` | Add a new task |
| `f` | Filter tasks |
| `q` / `Esc` | Close Taskboard |

Example checkbox syntax in notes:

```markdown
- [ ] draft release notes
- [x] update changelog
```

## Automatically Generated Notes

You can prefill notes for PR/issue-based worktrees using `worktree_note_script`. When creating a worktree from a PR or issue, the script receives the title and description on stdin and outputs a note to stdout.

```yaml
worktree_note_script: "aichat -m gemini:gemini-2.5-flash-lite 'Summarise this ticket into practical implementation notes.'"
```

If the script fails or outputs nothing, worktree creation continues normally without saving a note.

For full script configuration and environment variables, see [AI Integration](../advanced/ai-integration.md).

## Synchronisable Notes

By default, notes are stored in git config (local to each repository clone). To share notes across machines or with team members, configure a JSON storage path:

```yaml
worktree_notes_path: ".lazyworktree/notes.json"
```

This creates a single JSON file in your repository with all worktree notes. Commit the file to share notes with your team.

!!! tip
    When `worktree_notes_path` is set, keys are stored relative to `worktree_dir` instead of absolute paths, making them portable across different systems.
