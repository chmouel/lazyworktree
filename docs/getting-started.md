# Getting Started

<div class="lw-callout">
  <p><strong>Quick path:</strong> install lazyworktree, open a repository, run <code>lazyworktree</code>, then press <code>Enter</code> to jump to the selected worktree path.</p>
</div>

## First Run

1. Install lazyworktree using one of the methods in [Installation](installation.md).
2. Move to any Git repository in your terminal.
3. Run `lazyworktree`.
4. Press `?` to open the interactive help screen.

## Jumping to Worktrees from Your Shell

By default, pressing `Enter` outputs the selected worktree path. To jump directly:

```bash
cd "$(lazyworktree)"
```

For richer shell helpers and functions, see the [shell integration guide](https://github.com/chmouel/lazyworktree/blob/main/shell/README.md).

## Requirements

- **Git**: 2.31+
- **Forge CLI**: `gh` or `glab` for PR/MR status

Optional tools:

- Nerd Font: icons default to Nerd Font glyphs.
- delta: syntax-highlighted diffs (recommended).
- lazygit: full TUI git control.
- tmux / zellij: session management.
- [aichat](https://github.com/sigoden/aichat) or similar LLM CLI for automatic branch naming from diffs/issues/PRs.

!!! important
    If characters render incorrectly when starting lazyworktree, set `icon_set: text` or install a Nerd Font patched terminal font.

Build-time requirement:

- Go 1.25+

## Next Steps

<div class="mint-card-grid">
  <a class="mint-card" href="core/navigation-and-keybindings.md">
    <strong>Navigation and Keybindings</strong>
    <span>Learn the keyboard flow for everyday work.</span>
  </a>
  <a class="mint-card" href="configuration/overview.md">
    <strong>Configuration Overview</strong>
    <span>Tune behaviour, appearance, and integrations.</span>
  </a>
</div>
