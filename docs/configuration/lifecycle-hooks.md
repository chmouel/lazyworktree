# Lifecycle Hooks (`.wt`)

Run repository-local commands when creating or removing worktrees.


## `.wt` Example

```yaml
init_commands:
  - link_topsymlinks
  - cp $MAIN_WORKTREE_PATH/.env $WORKTREE_PATH/.env
  - npm install
  - code .

terminate_commands:
  - echo "Cleaning up $WORKTREE_NAME"
```

## Available Environment Variables

Lifecycle hooks receive the same managed command environment as custom commands:

| Variable | Description |
| --- | --- |
| `WORKTREE_BRANCH` | Branch checked out in the worktree |
| `MAIN_WORKTREE_PATH` | Path to the main/root worktree |
| `WORKTREE_PATH` | Full path to the worktree |
| `WORKTREE_NAME` | Basename of the worktree directory |
| `REPO_NAME` | Repository key, usually `owner/repo` |
| `REPO_OWNER` | Repository owner when available |
| `REPO_REPONAME` | Repository name without the owner |
| `LAZYWORKTREE_TYPE` | Source context type, such as `pr`, `issue`, or `diff` when known |
| `LAZYWORKTREE_NUMBER` | PR/MR or issue number when known |
| `LAZYWORKTREE_TEMPLATE` | Branch-name template used during PR/MR or issue creation when known |
| `LAZYWORKTREE_SUGGESTED_NAME` | LazyWorktree's default branch/worktree name suggestion when known |
| `LAZYWORKTREE_TITLE` | PR/MR or issue title when known |
| `LAZYWORKTREE_URL` | PR/MR or issue URL when known |
| `LAZYWORKTREE_DESCRIPTION` | PR/MR or issue body when known |

Contextual values are empty when LazyWorktree does not know the PR/MR, issue, or diff source. Issue metadata is available to creation-time hooks, but is not persisted for later terminate hooks after reload.

## Trust on First Use (TOFU)

Because `.wt` executes arbitrary commands, lazyworktree checks trust state.

Trust modes:

- `tofu` (default): prompt on first encounter or content change
- `never`: do not run `.wt` commands
- `always`: run without prompt

Trust hashes are stored in:

- `~/.local/share/lazyworktree/trusted.json`

## Built-in Special Command

- `link_topsymlinks`: symlinks untracked/ignored root files and common editor config directories, creates `tmp/`, and runs `direnv allow` when `.envrc` exists.
