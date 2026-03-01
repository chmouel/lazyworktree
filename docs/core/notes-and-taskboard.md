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

## Markdown Rendering

The info pane renders common markdown elements, including:

- headings
- lists
- quotes
- inline code and fenced code blocks
- links

Uppercase tags like `TODO`, `FIXME`, and `WARNING:` are highlighted (outside fenced code blocks).

## Taskboard

Press `T` to open Taskboard.

- Sources only markdown checkboxes from worktree notes.
- Supports moving, toggling completion, adding tasks, and filtering.

Example checkbox syntax:

```markdown
- [ ] draft release notes
- [x] update changelog
```

## Automatically Generated Notes

You can prefill notes for PR/issue-based worktrees using `worktree_note_script`.

For script configuration and environment variables, see [Worktree Notes Script](../worktree-notes.md).

## Next Steps

<div class="mint-card-grid">
  <a class="mint-card" href="../configuration/overview.md">
    <strong>Configuration Overview</strong>
    <span>Configure shared notes path and scripting behaviour.</span>
  </a>
  <a class="mint-card" href="../troubleshooting/common-problems.md">
    <strong>Troubleshooting</strong>
    <span>Resolve common issues in notes, rendering, and integrations.</span>
  </a>
</div>
