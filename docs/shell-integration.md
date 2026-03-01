# Shell Integration

Shell helpers change directory to the selected worktree on exit. Optional but recommended.

Zsh helpers are in `shell/functions.zsh`. See the [shell integration README](https://github.com/chmouel/lazyworktree/blob/main/shell/README.md) for details.

## Quick Usage

Without helper functions:

```bash
cd "$(lazyworktree)"
```

With helper functions loaded, you can wrap this in a reusable shell command and preserve consistent behaviour across repositories.

## Zsh Setup

Add this to your `.zshrc`:

```zsh
source /path/to/lazyworktree/shell/functions.zsh
```

Open a new shell, then run the helper function described in `shell/README.md`.

## Bash/Fish

If you are not on zsh, use the plain command form:

```bash
cd "$(lazyworktree)"
```

This works with any POSIX-like shell and does not require additional integration files.

## Troubleshooting

- If `cd "$(lazyworktree)"` does not change directory, confirm `lazyworktree` is in your `PATH`.
- If output is empty, ensure a worktree is selected before quitting the TUI.
- If shell profile changes do not load, restart your terminal session.
