# CLI `exec`

Run shell commands or trigger custom command keys in a target worktree.

## Examples

### Shell command mode

```bash
lazyworktree exec --workspace=my-feature "make test"
lazyworktree exec -w my-feature "git status"

# Auto-detect worktree from current directory
cd ~/worktrees/repo/my-feature
lazyworktree exec "npm run build"
```

### Custom command key mode

```bash
lazyworktree exec --key=t --workspace=my-feature
lazyworktree exec -k z -w my-feature

# Auto-detect and trigger
cd ~/worktrees/repo/my-feature
lazyworktree exec --key=t
```

## Behaviour

- `--workspace` (`-w`) accepts worktree name or path.
- Without `--workspace`, lazyworktree auto-detects from current directory.
- Accepts either positional shell command or `--key` custom action.
- Exposes `WORKTREE_*` environment variables.
- Supports shell, tmux, zellij, and show-output command types.
- `new-tab` commands are not supported in CLI mode.

## Next Steps

<div class="mint-card-grid">
  <a class="mint-card" href="overview.md">
    <strong>CLI Overview</strong>
    <span>Review command surface and when to use each command.</span>
  </a>
  <a class="mint-card" href="../core/command-palette.md">
    <strong>Command Palette</strong>
    <span>Map CLI execution patterns to interactive palette workflows.</span>
  </a>
</div>
