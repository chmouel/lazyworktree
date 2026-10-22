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
lazyworktree exec --key=_review --workspace=my-feature
lazyworktree exec -k z -w my-feature

# Auto-detect and trigger
cd ~/worktrees/repo/my-feature
lazyworktree exec --key=t
```

## Behaviour

- `--workspace` (`-w`) accepts worktree name or path.
- Without `--workspace`, lazyworktree auto-detects from current directory.
- Accepts either positional shell command or `--key` custom action.
- Palette-only custom commands still use their `_name` identifier with `--key`.
- Exposes the same managed `WORKTREE_*`, `REPO_*`, and contextual `LAZYWORKTREE_*` environment variables as custom commands in the TUI.
- Supports shell, tmux, zellij, herdr, and show-output command types.
- `new_tab` is ignored for tmux and zellij commands, which have no terminal tab to open. Their configured `attach` setting decides whether the session opens in the current terminal or is created in the background with instructions for attaching later. `new_tab` is not supported for Herdr or plain commands in CLI mode.
