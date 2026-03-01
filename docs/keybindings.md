# Key Bindings

<div class="lw-callout">
  <p><strong>Tip:</strong> use this page by context. Start with global controls, then jump to the pane you are working in.</p>
</div>

## Jump by Context

- [Global Controls](#global-controls)
- [Pane Focus and Layout](#pane-focus-and-layout)
- [Notes and Taskboard](#notes-and-taskboard)
- [Commit Pane](#commit-pane)
- [Commit File Tree](#commit-file-tree)
- [Status Pane](#status-pane)
- [Git Status Pane](#git-status-pane)
- [Filter and Search Modes](#filter-and-search-modes)
- [Command History and Palette](#command-history-and-palette)
- [Mouse Controls](#mouse-controls)

## Global Controls

| Key | Action |
| --- | --- |
| `Enter` | Jump to worktree (exit and `cd`) |
| `j`, `k` | Move selection up/down in lists and menus |
| `c` | Create new worktree (from branch, commit, PR/MR, or issue) |
| `i` | Open selected worktree notes (viewer if present, editor if empty) |
| `T` | Open Taskboard (grouped markdown checkbox tasks across worktrees) |
| `m` | Rename selected worktree |
| `D` | Delete selected worktree |
| `d` | View diff in pager (worktree or commit, depending on pane) |
| `A` | Absorb worktree into main |
| `X` | Prune merged worktrees (refreshes PR data, checks merge status) |
| `!` | Run arbitrary command in selected worktree (with command history) |
| `v` | View CI checks (Enter opens browser, `Ctrl+v` opens logs in pager) |
| `o` | Open PR/MR in browser (or root repo in editor if main branch with merged/closed/no PR) |
| `ctrl+p`, `:` | Command palette |
| `g` | Open LazyGit |
| `r` | Refresh list (also refreshes PR/MR/CI for current worktree) |
| `R` | Fetch all remotes |
| `S` | Synchronise with upstream (pull + push, requires clean worktree) |
| `P` | Push to upstream (prompts to set upstream if missing) |
| `f` | Filter focused pane (worktrees, files, commits) |
| `/` | Search focused pane (incremental) |
| `alt+n`, `alt+p` | Move selection and fill filter input |
| `↑`, `↓` | Move selection (filter active, no fill) |
| `s` | Cycle sort mode (Path / Last Active / Last Switched) |
| `Home` | Go to first item in focused pane |
| `End` | Go to last item in focused pane |
| `?` | Show help |
| `q` | Quit |
| `y` | Copy context-aware value to clipboard (path/file/SHA via OSC52) |
| `Y` | Copy selected worktree branch name to clipboard |

## Pane Focus and Layout

| Key | Action |
| --- | --- |
| `1` | Focus Worktree pane (toggle zoom if already focused) |
| `2` | Focus Status pane (toggle zoom if already focused) |
| `3` | Focus Git Status pane (toggle zoom if already focused) |
| `4` | Focus Commit pane (toggle zoom if already focused) |
| `5` | Focus Notes pane (toggle zoom if already focused; only when note exists) |
| `h`, `l` | Navigate left/right (`h` worktree pane, `l` cycle right panes) |
| `Tab`, `]` | Cycle to next pane |
| `[` | Cycle to previous pane |
| `=` | Toggle zoom for focused pane |
| `L` | Toggle layout (`default` / `top`) |

## Notes and Taskboard

### Notes Viewer and Editor

Press `i` to open notes for the selected worktree.

- If a note exists, lazyworktree opens the viewer first.
- If no note exists, lazyworktree opens the editor.

Viewer controls: `j`/`k`, arrows, `Ctrl+D`, `Ctrl+U`, `g`, `G`, `e`, `q`, `Esc`.

Editor controls: `Ctrl+S`, `Ctrl+X`, `Enter`, `Esc`.

The Info pane renders Markdown and highlights uppercase tags such as `TODO`, `FIXME`, and `WARNING:` outside fenced code blocks.

### Taskboard

Press `T` to open Taskboard (Kanban-lite grouped by worktree notes).

- Collects markdown checkboxes, e.g. `- [ ] draft release notes`.
- Use `j`/`k` to move, `Enter` or `Space` to toggle, `a` to add, `f` to filter, `q`/`Esc` to close.

## Commit Pane

| Key | Action |
| --- | --- |
| `Enter` | Open commit file tree (browse files changed in commit) |
| `d` | Show full commit diff in pager |
| `C` | Cherry-pick commit to another worktree |
| `j/k` | Navigate commits |
| `ctrl+j` | Next commit and open file tree |
| `/` | Search commit titles (incremental) |

## Commit File Tree

| Key | Action |
| --- | --- |
| `j/k` | Navigate files and directories |
| `Enter` | Toggle directory collapse/expand, or show file diff |
| `d` | Show full commit diff in pager |
| `f` | Filter files by name |
| `/` | Search files (incremental) |
| `n/N` | Next/previous search match |
| `ctrl+d`, `Space` | Half page down |
| `ctrl+u` | Half page up |
| `g`, `G` | Jump to top/bottom |
| `q`, `Esc` | Return to commit log |

## Status Pane

Displays PR info, CI checks, notes, and divergence status.

| Key | Action |
| --- | --- |
| `j/k` | Navigate CI checks (when visible) |
| `Enter` | Open selected CI check URL in browser |
| `Ctrl+v` | View selected CI check logs in pager |
| `Ctrl+r` | Restart CI job (GitHub Actions only) |

## Git Status Pane

Displays changed files in a collapsible tree view grouped by directory.

| Key | Action |
| --- | --- |
| `j/k` | Navigate files and directories |
| `Enter` | Toggle directory expand/collapse, or show file diff |
| `e` | Open selected file in editor |
| `d` | Show full diff of all files in pager |
| `s` | Stage/unstage selected file or directory |
| `D` | Delete selected file or directory (with confirmation) |
| `c` | Commit staged changes |
| `C` | Stage all changes and commit |
| `g` | Open LazyGit |
| `ctrl+←`, `ctrl+→` | Jump to previous/next folder |
| `/` | Search file/directory names (incremental) |
| `ctrl+d`, `Space` | Half page down |
| `ctrl+u` | Half page up |
| `PageUp`, `PageDown` | Half page up/down |

## Filter and Search Modes

### Filter Mode

Applies to focused pane (worktrees, files, commits). Active filter shows `[Esc] Clear`.

- `alt+n`, `alt+p`: navigate and update filter input
- `↑`, `↓`, `ctrl+j`, `ctrl+k`: navigate without changing input
- `Enter`: exit filter mode (filter remains)
- `Esc`, `Ctrl+C`: clear filter

### Search Mode

- Type to jump to the first match
- `n`, `N`: next/previous match
- `Enter`: close search
- `Esc`, `Ctrl+C`: clear search

## Command History and Palette

- Command history (`!`) is saved per repository (max 100 entries).
- Use `↑` / `↓` to navigate command history.
- In Command Palette:
  - **Select theme** changes theme with live preview (see [Themes](themes.md)).
  - **Create from current branch** can include current file changes and may use `branch_name_script`.

## Mouse Controls

- **Click**: select and focus panes or items.
- **Scroll**: navigate lists in any pane.
