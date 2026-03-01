# CLI `create`

Create worktrees from local branches, issues, PRs, or interactive selection.

## Examples

```bash
lazyworktree create                          # Auto-generated from current branch
lazyworktree create my-feature               # Explicit name
lazyworktree create my-feature --with-change # Include uncommitted changes
lazyworktree create --from-branch main my-feature
lazyworktree create --from-pr 123
lazyworktree create --from-issue 42
lazyworktree create --from-issue 42 --from-branch main
lazyworktree create -I                       # Interactive issue selection
lazyworktree create -P                       # Interactive PR selection
lazyworktree create -P -q "dark"             # Pre-filter interactive PR selection
lazyworktree create -I --query "login"       # Pre-filter interactive issue selection
lazyworktree create --from-pr 123 --no-workspace
lazyworktree create --from-issue 42 --no-workspace
lazyworktree create my-feature --exec 'npm test'
```

## Behaviour Notes

- `--exec` runs after successful creation.
- With `--no-workspace`, `--exec` runs in current directory.
- Shell execution uses your shell mode (`zsh -ilc`, `bash -ic`, otherwise `-lc`).

PR-specific branch behaviour:

- worktree name always uses generated worktree name
- local branch may preserve PR branch if you are PR author
- unresolved identity falls back to PR branch name

## Next Steps

<div class="mint-card-grid">
  <a class="mint-card" href="rename.md">
    <strong>CLI rename</strong>
    <span>Rename worktree directories and optionally matching branches.</span>
  </a>
  <a class="mint-card" href="exec.md">
    <strong>CLI exec</strong>
    <span>Run post-create checks or command keys in the new worktree.</span>
  </a>
</div>
