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
