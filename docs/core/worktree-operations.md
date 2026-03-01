# Worktree Operations

Create and manage multiple active branches in parallel without branch checkout churn.

<div class="mint-callout">
  <p><strong>Use this page when:</strong> you want the core lifecycle actions for worktrees, from creation to cleanup.</p>
</div>

![LazyWorktree main interface](../assets/screenshot-main.png)

## Core Actions

| Action | Purpose | Typical Entry Point |
| --- | --- | --- |
| Create | Start isolated work from branch/PR/issue | `c` in TUI, `lazyworktree create` |
| Rename | Keep workspace names meaningful | `m` in TUI, `lazyworktree rename` |
| Delete | Remove workspace and optionally branch | `D` in TUI, `lazyworktree delete` |
| Absorb | Integrate selected worktree into main | `A` in TUI |
| Prune | Remove merged worktrees in bulk | `X` in TUI |
| Sync | Pull and push clean worktrees | `S` in TUI |

### Default Location

Worktrees are created under:

```
~/.local/share/worktrees/<organization>-<repo_name>
```

Customise this with the `worktree_dir` configuration option.

## Creation Sources

Press `c` to open the creation menu with the following modes:

- **From current branch** — with or without uncommitted changes
- **Checkout existing branch** — select from available branches or create a new one
- **From PR/MR** — create a worktree directly from an open pull or merge request
- **From issue** — create a worktree from a GitHub or GitLab issue

![Branch creation flow](../assets/screenshot-branch.png)

### Creating from a PR or Issue

When creating from a PR or issue, LazyWorktree:

1. Fetches the PR/issue metadata using `gh` or `glab`
2. Generates a branch name from the title (or via AI if `branch_name_script` is configured)
3. Checks out the branch and sets up the worktree
4. Optionally generates a note from the description (if `worktree_note_script` is configured)

!!! tip
    Configure `pr_branch_name_template` and `issue_branch_name_template` to control how branch names are derived. See [Branch Naming](../configuration/branch-naming.md).

### Branch Name Sanitisation

All branch names — manual or generated — are automatically sanitised:

| Input | Converted |
| --- | --- |
| `feature.new` | `feature-new` |
| `bug fix here` | `bug-fix-here` |
| `feature:test` | `feature-test` |
| `user@domain/fix` | `user-domain-fix` |

Special characters are converted to hyphens, leading/trailing hyphens are removed, and consecutive hyphens are collapsed.

For exact CLI patterns, see [CLI `create`](../cli/create.md).

## Lifecycle Hooks

Worktree creation/removal can run commands from repository `.wt` files and global config hooks. These are protected by TOFU (Trust On First Use) security — you must explicitly approve each `.wt` file before its commands execute.

For hook setup and trust behaviour, see [Lifecycle Hooks](../configuration/lifecycle-hooks.md).

## Environment-Aware Commands

Custom commands and lifecycle hooks receive worktree context variables:

| Variable | Description |
| --- | --- |
| `WORKTREE_NAME` | Name of the worktree (e.g., `my-feature`) |
| `WORKTREE_BRANCH` | Branch name for the worktree |
| `WORKTREE_PATH` | Full path to the worktree directory |
| `MAIN_WORKTREE_PATH` | Path to the main/root worktree |
| `REPO_NAME` | Name of the repository |
