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
