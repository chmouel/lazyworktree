# Lifecycle Hooks (`.wt`)

Run repository-local commands when creating or removing worktrees.

<div class="mint-callout">
  <p><strong>Use this page when:</strong> you want repeatable project bootstrap and cleanup per worktree.</p>
</div>

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

- `WORKTREE_BRANCH`
- `MAIN_WORKTREE_PATH`
- `WORKTREE_PATH`
- `WORKTREE_NAME`

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
