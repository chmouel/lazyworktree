# Navigation and Keybindings

This page focuses on movement, pane control, search, and command invocation.

<div class="mint-callout">
  <p><strong>Use this page when:</strong> you are learning daily navigation patterns and keyboard flow in the TUI.</p>
</div>

## Global Navigation

| Key | Action |
| --- | --- |
| `j`, `k` | Move selection up/down |
| `Tab`, `]` | Next pane |
| `[` | Previous pane |
| `h`, `l` | Move left/right across panes |
| `Home`, `End` | Jump to first/last item |
| `q` | Quit |
| `?` | Help |

## Pane Focus and Layout

| Key | Action |
| --- | --- |
| `1`..`5` | Focus specific panes |
| `=` | Toggle zoom for focused pane |
| `L` | Toggle layout (`default` / `top`) |

## Search and Filter

| Mode | Key | Behaviour |
| --- | --- | --- |
| Filter | `f` | Filter focused pane list |
| Search | `/` | Incremental search in focused pane |
| Next match | `n` | Move to next search match |
| Previous match | `N` | Move to previous search match |
| Clear | `Esc` | Clear active filter/search |

## Command Access

| Key | Action |
| --- | --- |
| `ctrl+p`, `:` | Open command palette |
| `!` | Run arbitrary command in selected worktree |
| `g` | Open lazygit |

## Clipboard Shortcuts

| Key | Action |
| --- | --- |
| `y` | Copy context-aware value (path/file/SHA) |
| `Y` | Copy selected worktree branch name |

## Full Reference

For complete pane-by-pane key coverage, see [Key Bindings Reference](../keybindings.md).

## Next Steps

<div class="mint-card-grid">
  <a class="mint-card" href="notes-and-taskboard.md">
    <strong>Notes and Taskboard</strong>
    <span>Learn note editing flow and checkbox task management.</span>
  </a>
  <a class="mint-card" href="command-palette.md">
    <strong>Command Palette</strong>
    <span>Use palette-driven actions and custom command integration.</span>
  </a>
</div>
