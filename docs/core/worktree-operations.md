# Worktree Operations

Create and manage multiple active branches in parallel without branch checkout churn.

<div class="mint-callout">
  <p><strong>Use this page when:</strong> you want the core lifecycle actions for worktrees, from creation to cleanup.</p>
</div>

## Core Actions

| Action | Purpose | Typical Entry Point |
| --- | --- | --- |
| Create | Start isolated work from branch/PR/issue | `c` in TUI, `lazyworktree create` |
| Rename | Keep workspace names meaningful | `m` in TUI, `lazyworktree rename` |
| Delete | Remove workspace and optionally branch | `D` in TUI, `lazyworktree delete` |
| Absorb | Integrate selected worktree into main | `A` in TUI |
| Prune | Remove merged worktrees in bulk | `X` in TUI |
| Sync | Pull and push clean worktrees | `S` in TUI |

## Creation Sources

You can create worktrees from:

- current branch
- explicit base branch
- PR/MR
- issue
- custom create menu actions

For exact CLI patterns, see [CLI `create`](../cli/create.md).

## Lifecycle Hooks

Worktree creation/removal can run commands from repository `.wt` files and global config hooks.

For hook setup and trust behaviour, see [Lifecycle Hooks](../configuration/lifecycle-hooks.md).

## Environment-Aware Commands

Custom commands and lifecycle hooks receive worktree context variables such as:

- `WORKTREE_BRANCH`
- `WORKTREE_PATH`
- `WORKTREE_NAME`
- `MAIN_WORKTREE_PATH`

## Next Steps

<div class="mint-card-grid">
  <a class="mint-card" href="../cli/create.md">
    <strong>CLI: create</strong>
    <span>Create worktrees from branch, PR, issue, or interactive selection.</span>
  </a>
  <a class="mint-card" href="../configuration/lifecycle-hooks.md">
    <strong>Lifecycle Hooks</strong>
    <span>Automate setup and teardown with trusted `.wt` commands.</span>
  </a>
</div>
