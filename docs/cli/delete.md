# CLI `delete`

Delete a worktree, with optional branch retention.

## Examples

```bash
lazyworktree delete             # Delete worktree and branch
lazyworktree delete --no-branch # Delete worktree only
```

## Notes

- Use `--no-branch` when branch preservation is required.
- For bulk stale cleanup, use TUI prune (`X`) in interactive mode.

## Next Steps

<div class="mint-card-grid">
  <a class="mint-card" href="list.md">
    <strong>CLI list</strong>
    <span>Review current worktrees before and after deletion.</span>
  </a>
  <a class="mint-card" href="create.md">
    <strong>CLI create</strong>
    <span>Create replacement worktrees from branch/PR/issue contexts.</span>
  </a>
</div>
