# Custom Initialisation and Termination

Create a `.wt` file in your repository to run commands when creating or removing worktrees. Format inspired by [wt](https://github.com/taecontrol/wt).

<div class="mint-callout">
  <p><strong>Use this page when:</strong> you want to automate setup or cleanup tasks when worktrees are created or deleted.</p>
</div>

## `.wt` File Format

Place a `.wt` file in your repository root. It uses YAML with two keys: `init_commands` (run after worktree creation) and `terminate_commands` (run before worktree removal).

### Example

```yaml
init_commands:
    - link_topsymlinks
    - cp $MAIN_WORKTREE_PATH/.env $WORKTREE_PATH/.env
    - npm install
    - direnv allow $WORKTREE_PATH
    - docker compose -f $WORKTREE_PATH/docker-compose.yml up -d

terminate_commands:
    - docker compose -f $WORKTREE_PATH/docker-compose.yml down
    - echo "Cleaned up $WORKTREE_NAME"
```

## Environment Variables

The following variables are available to all init and terminate commands:

| Variable | Description | Example |
| --- | --- | --- |
| `WORKTREE_BRANCH` | Branch checked out in the new worktree | `feature/auth` |
| `MAIN_WORKTREE_PATH` | Absolute path to the main (bare) worktree | `/home/user/project` |
| `WORKTREE_PATH` | Absolute path to the new worktree | `/home/user/project-worktrees/feature-auth` |
| `WORKTREE_NAME` | Short name of the worktree | `feature-auth` |

## Execution Order

Commands execute in the order listed. If you also have global `init_commands` or `terminate_commands` in your LazyWorktree configuration file, those run **before** the repository `.wt` commands. This allows global setup (e.g., shell environment) to complete before project-specific commands.

If any command fails (non-zero exit code), subsequent commands in the list still execute — failure does not halt the sequence.

## Special Commands

### `link_topsymlinks`

A built-in command (not a shell command) that automates common worktree setup tasks:

1. **Symlinks untracked and ignored root files** from the main worktree into the new worktree — useful for configuration files, local overrides, and build artefacts that should not be duplicated
2. **Symlinks editor configuration directories**: `.vscode`, `.idea`, `.cursor`, and `.claude/settings.local.json`
3. **Creates a `tmp/` directory** in the new worktree
4. **Runs `direnv allow`** if a `.envrc` file exists in the main worktree

This command is particularly useful for projects that rely on local configuration files (`.env`, editor settings) that are not tracked in git but need to be present in every worktree.

## Security: Trust on First Use (TOFU)

Since `.wt` files execute arbitrary commands, LazyWorktree uses a TOFU model to protect against unintended execution.

### How it works

1. **First encounter**: LazyWorktree hashes the `.wt` file and prompts you to **Trust**, **Block**, or **Cancel**
2. **Subsequent runs**: the hash is compared against the stored value. If it matches, commands run without prompting
3. **File modified**: if the `.wt` file has changed since you last trusted it (even whitespace changes), the hash will no longer match and you will be prompted again

Trust decisions are stored in `~/.local/share/lazyworktree/trusted.json`.

### Trust modes

| Mode | Behaviour |
| --- | --- |
| `tofu` | Prompt on first use and when the file changes (default) |
| `never` | Never execute `.wt` commands — all hooks are silently skipped |
| `always` | Always execute without prompting (use with caution) |

Configure in your LazyWorktree config:

```yaml
trust_mode: tofu
```

### Re-prompting triggers

You will be prompted to re-evaluate trust when:

- The `.wt` file content changes (hash mismatch)
- The `trusted.json` file is deleted or the entry is removed
- You switch from `never` or `always` back to `tofu` mode
