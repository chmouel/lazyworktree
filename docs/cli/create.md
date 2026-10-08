# CLI `create`

Create worktrees from local branches, issues, PRs, or interactive selection.

## Examples

```bash
lazyworktree create                           # From current branch (generated name, as it is already checked out)
lazyworktree create my-feature                # Explicit name
lazyworktree create my-feature --with-change  # Include uncommitted changes
lazyworktree create --from-branch feature/x   # Check out feature/x directly in worktree feature-x
lazyworktree create --from-branch main my-feature
lazyworktree create --branch main my-feature  # --branch is an alias for --from-branch
lazyworktree create --from-pr 123
lazyworktree create --from-issue 42
lazyworktree create --from-issue 42 --from-branch main
lazyworktree create -I                        # Interactive issue selection
lazyworktree create -P                        # Interactive PR selection
lazyworktree create -P -q "dark"              # Pre-filter interactive PR selection
lazyworktree create -I --query "login"        # Pre-filter interactive issue selection
lazyworktree create --from-pr 123 --no-workspace
lazyworktree create --from-issue 42 --no-workspace
lazyworktree create my-feature --exec 'npm test'
lazyworktree create my-feature --exec 'npm test' --exec-mode direct
lazyworktree create my-feature --json         # Machine-readable output
lazyworktree create my-feature --description "Implement login flow" --tags "auth,backend"
lazyworktree create my-feature --note "See RFC-42 for context"
lazyworktree create my-feature --note-file ./notes.md
```

## Behaviour Notes

- `--exec` runs after successful creation.
- With `--no-workspace`, `--exec` runs in current directory.
- Shell execution uses your shell mode (`zsh -ilc`, `bash -ic`, otherwise `-lc`).
- `--exec-mode` controls how `--exec` is invoked:
  - `login-shell` (default): your `$SHELL` with login/interactive flags
  - `shell`: your `$SHELL -c` only (non-interactive, faster)
  - `direct`: splits the command string and execs without any shell wrapper

Branch behaviour without a positional name:

- the source branch is checked out as is, in a worktree named after it (`feature/x` becomes `feature-x`)
- a remote branch such as `origin/foo` uses local `foo`, created to track the remote if missing
- if the branch is already checked out in a worktree, a new `<branch>-<adjective>-<noun>` branch is created from it instead
- with a positional name, a branch of that name is created from the source branch

PR-specific branch behaviour:

- worktree name is generated from the configured template
- local branch name always matches the PR head branch name
