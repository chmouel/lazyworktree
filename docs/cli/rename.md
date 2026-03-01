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

## Next Steps

<div class="mint-card-grid">
  <a class="mint-card" href="create.md">
    <strong>CLI create</strong>
    <span>Create worktrees with naming conventions from the start.</span>
  </a>
  <a class="mint-card" href="exec.md">
    <strong>CLI exec</strong>
    <span>Run workspace-aware commands after rename operations.</span>
  </a>
</div>
