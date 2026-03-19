# Action IDs Reference

Use these IDs in the `keybindings:` section of your configuration file to bind any key to a built-in palette action. Keybindings use a pane-scoped structure where `universal` bindings apply everywhere and pane-specific sections override them when that pane is focused.

**Valid pane scope names:** `universal`, `worktrees`, `info`, `status`, `log`, `notes`, `agent_sessions`

```yaml
keybindings:
  universal:
    G: lazygit
    ctrl+d: delete
    F: fetch
  worktrees:
    x: delete
  log:
    d: diff
```

Keys defined in `keybindings:` take priority over `custom_commands` and built-in keys. The bound key is also displayed as the shortcut in the command palette. Pane-specific bindings override universal ones for the same key.

---

## Git Operations

| ID | Label | Default Key | Description |
|----|-------|-------------|-------------|
| `diff` | Show diff | `d` | Show diff for current worktree or commit |
| `refresh` | Refresh | `r` | Reload worktrees |
| `fetch` | Fetch remotes | `R` | git fetch --all |
| `push` | Push to upstream | `P` | git push (clean worktree only) |
| `sync` | Synchronise with upstream | `S` | git pull, then git push (clean worktree only) |
| `fetch-pr-data` | Fetch PR data | `p` | Fetch PR/MR status from GitHub/GitLab |
| `ci-checks` | View CI checks | `v` | View CI check logs for current worktree |
| `pr` | Open in browser | `o` | Open PR, branch, or repo in browser |
| `lazygit` | Open LazyGit | `g` | Open LazyGit in selected worktree |
| `run-command` | Run command | `!` | Run arbitrary command in worktree |

## Status Pane

| ID | Label | Default Key | Description |
|----|-------|-------------|-------------|
| `stage-file` | Stage/unstage file | `s` | Stage or unstage selected file |
| `commit-staged` | Open commit screen | `c` | Open the commit screen for staged changes (or prompt to stage all) |
| `commit-all` | Commit changes using git editor | `C` | Commit using git editor |
| `edit-file` | Edit file | `e` | Open selected file in editor |
| `delete-file` | Delete selected file or directory | — | Delete selected file or directory |

## Log Pane

| ID | Label | Default Key | Description |
|----|-------|-------------|-------------|
| `cherry-pick` | Cherry-pick commit | `C` | Cherry-pick commit to another worktree |
| `commit-view` | Browse commit files | — | Browse files changed in selected commit |

## Navigation

| ID | Label | Default Key | Description |
|----|-------|-------------|-------------|
| `zoom-toggle` | Toggle zoom | `=` | Toggle zoom on focused pane |
| `toggle-layout` | Toggle layout | `L` | Switch between default and top layout |
| `filter` | Filter | `f` | Filter items in focused pane |
| `search` | Search | `/` | Search items in focused pane |
| `focus-worktrees` | Focus worktrees | `1` | Focus worktree pane |
| `focus-status` | Focus status | `2` | Focus status pane |
| `focus-log` | Focus log | `3` | Focus log pane |
| `sort-cycle` | Cycle sort | `s` | Cycle sort mode (path/active/switched) |
| `copy-path` | Copy path / file / SHA | `y` | Copy context-aware content (path, file, or commit SHA) |
| `copy-branch` | Copy branch name | `Y` | Copy selected worktree branch name |
| `copy-pr-url` | Copy PR/MR URL | — | Copy selected worktree PR/MR URL |

## Settings

| ID | Label | Default Key | Description |
|----|-------|-------------|-------------|
| `theme` | Select theme | — | Change the application theme with live preview |
| `taskboard` | Taskboard | `T` | Browse and toggle worktree tasks |
| `help` | Help | `?` | Show help |
