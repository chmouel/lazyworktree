# Command Palette

The command palette is the fastest way to trigger actions without remembering every key.

<div class="mint-callout">
  <p><strong>Use this page when:</strong> you want discoverable, searchable commands with recent-item prioritisation.</p>
</div>

## Opening and Filtering

- Open with `ctrl+p` or `:`.
- Type to filter actions and custom commands.
- MRU ordering can prioritise recently used entries.

## Built-in Workflows

The palette can trigger:

- worktree actions
- git operations
- navigation and pane controls
- settings actions such as theme selection

## Custom Command Integration

Custom key bindings are added to palette entries.
Session-oriented custom commands (tmux/zellij) can also expose active sessions.

For full command schema, see [Custom Commands Reference](../custom-commands.md).

## Suggested Configuration

In `config.yaml`:

```yaml
palette_mru: true
palette_mru_limit: 5
```

## Next Steps

<div class="mint-card-grid">
  <a class="mint-card" href="../custom-commands.md">
    <strong>Custom Commands</strong>
    <span>Define shell, tmux, and zellij command entries.</span>
  </a>
  <a class="mint-card" href="navigation-and-keybindings.md">
    <strong>Navigation and Keys</strong>
    <span>Pair palette usage with keyboard-first movement patterns.</span>
  </a>
</div>
