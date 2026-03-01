# Custom Initialisation and Termination

Create a `.wt` file in your repository to run commands when creating/removing worktrees. Format inspired by [wt](https://github.com/taecontrol/wt).

### Example `.wt` configuration

```yaml
init_commands:
    - link_topsymlinks
    - cp $MAIN_WORKTREE_PATH/.env $WORKTREE_PATH/.env
    - npm install
    - code .

terminate_commands:
    - echo "Cleaning up $WORKTREE_NAME"
```

Environment variables: `WORKTREE_BRANCH`, `MAIN_WORKTREE_PATH`, `WORKTREE_PATH`, `WORKTREE_NAME`.

### Security: Trust on First Use (TOFU)

Since `.wt` files execute arbitrary commands, lazyworktree uses TOFU. On first encounter or modification, select **Trust**, **Block**, or **Cancel**. Hashes stored in `~/.local/share/lazyworktree/trusted.json`.

Configure `trust_mode`: `tofu` (default, prompt), `never` (skip all), `always` (no prompts).

### Special Commands

* `link_topsymlinks`: Built-in command that symlinks untracked/ignored root files, editor configs (`.vscode`, `.idea`, `.cursor`, `.claude/settings.local.json`), creates `tmp/`, and runs `direnv allow` if `.envrc` exists.
