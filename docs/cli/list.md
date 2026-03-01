# CLI `list`

List available worktrees in table, pristine, or JSON form.

## Examples

```bash
lazyworktree list              # Table output (default)
lazyworktree list --pristine   # Paths only
lazyworktree list --json       # JSON output
lazyworktree ls                # Alias
```

## Notes

- `--pristine` and `--json` are mutually exclusive.
- Use `--pristine` for shell pipelines.

## Next Steps

<div class="mint-card-grid">
  <a class="mint-card" href="create.md">
    <strong>CLI create</strong>
    <span>Create new worktrees after inspecting current ones.</span>
  </a>
  <a class="mint-card" href="delete.md">
    <strong>CLI delete</strong>
    <span>Remove stale worktrees and branches safely.</span>
  </a>
</div>
