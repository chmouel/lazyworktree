# CI and PR/MR Status

LazyWorktree surfaces pull/merge request information and CI state directly in the status pane.

<div class="mint-callout">
  <p><strong>Use this page when:</strong> you need to inspect checks, view logs, and open CI context without leaving the terminal.</p>
</div>

## Status Indicators

For worktrees linked to PR/MR items:

- `✓` passed
- `✗` failed
- `●` pending
- `○` skipped
- `⊘` cancelled

Status data is fetched lazily and cached briefly for responsiveness.

## Common Actions

| Key | Action |
| --- | --- |
| `j/k` | Navigate CI checks |
| `Enter` | Open selected check URL |
| `Ctrl+v` | View selected check logs in pager |
| `Ctrl+r` | Restart CI job (GitHub Actions) |

## Hyperlinks and Context

In terminals that support OSC-8 hyperlinks, PR/MR identifiers in status details are clickable.

## Next Steps

<div class="mint-card-grid">
  <a class="mint-card" href="../configuration/diff-pager-and-editor.md">
    <strong>Pager and Editor Settings</strong>
    <span>Tune pager behaviour for logs and diff output.</span>
  </a>
  <a class="mint-card" href="../cli/exec.md">
    <strong>CLI exec</strong>
    <span>Trigger commands and custom actions from scripts and terminals.</span>
  </a>
</div>
