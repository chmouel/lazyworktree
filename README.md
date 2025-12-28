# lazyworktree

A Textual-based TUI for managing Git worktrees efficiently. Visualize status, manage branches, and jump between worktrees with ease.

![Python](https://img.shields.io/badge/python-3.12%2B-blue)

## Features

- **Worktree Management**: Create, delete, and absorb worktrees seamlessly.
- **Status at a Glance**: View dirty state, ahead/behind counts, and divergence from main.
- **GitHub Integration**: Fetch and display associated Pull Request status (via `gh` CLI).
- **Diff Viewer**: Integrated diff viewer with optional `delta` support.
- **LazyGit Integration**: Launch `lazygit` directly for the selected worktree.
- **Shell Integration**: Jump (cd) directly to selected worktrees upon exit.

## Prerequisites

- **Python**: 3.12+
- **Git**: 2.31+ (recommended)
- **GitHub CLI (`gh`)**: Required for repo resolution and PR status.
- **uv**: Recommended for dependency management and running.

**Optional:**

- **delta**: For syntax-highlighted diffs.
- **lazygit**: For full TUI git control.

## Installation

### Using uv (Recommended)

Clone the repository and run directly:

```bash
git clone https://github.com/yourusername/lazyworktree.git
cd lazyworktree
uv run main.py
```

## Shell Integration (Zsh)

To enable the "jump" functionality (changing your shell's current directory on exit), add the helper functions from `shell/functions.shell` to your `.zshrc`.

Example configuration:

```bash
# Add to .zshrc
source /path/to/lazyworktree/shell/functions.shell

# Create an alias for a specific repository
# assumes worktrees are stored in ~/.local/share/worktrees/<repo_name>
pm() { worktree_jump ~/path/to/your/main/repo "$@"; }
```

Now you can run `pm` to open the TUI, select a worktree, and upon pressing `Enter`, your shell will `cd` into that directory.

## Key Bindings

| Key | Action |
| --- | --- |
| `Enter` | Jump to worktree (exit and cd) |
| `c` | Create new worktree |
| `D` | Delete selected worktree |
| `d` | View diff (auto-refreshes) |
| `p` | Fetch GitHub PR status |
| `g` | Open LazyGit |
| `r` | Refresh list |
| `/` | Filter worktrees |
| `?` | Show help |

## Configuration

Worktrees are expected to be organized under `~/.local/share/worktrees/<repo_name>` by default, though the script attempts to resolve locations via `gh repo view`.
