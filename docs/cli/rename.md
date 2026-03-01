# CLI `rename`

Rename worktrees by current context, name, or path.

## Examples

```bash
lazyworktree rename new-feature-name              # Rename current worktree from cwd
lazyworktree rename feature new-feature-name
lazyworktree rename /path/to/worktree new-name
```

## Notes

- Branch is renamed only when the worktree directory name matches branch name.
- Use explicit path to avoid ambiguity in script contexts.
