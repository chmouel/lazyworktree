# Action IDs Reference

Use these IDs in the `keybindings:` section of your configuration file to bind any key to a built-in palette action. Keybindings use a pane-scoped structure where `universal` bindings apply everywhere and pane-specific sections override them when that pane is focused.

**Valid pane scope names:** `universal`, `worktrees`, `info`, `status`, `log`, `notes`, `agent_sessions`

```yaml
keybindings:
  universal:
    G: git-lazygit
    ctrl+d: worktree-delete
    F: git-fetch
  worktrees:
    x: worktree-delete
  log:
    d: git-diff
```

Keys defined in `keybindings:` take priority over `custom_commands` and built-in keys. The bound key is also displayed as the shortcut in the command palette. Pane-specific bindings override universal ones for the same key.

---

## Git Operations

| ID | Label | Default Key | Description |
|----|-------|-------------|-------------|
| `git-diff` | Show diff | `d` | Show diff for current worktree or commit |
| `git-refresh` | Refresh | `r` | Reload worktrees |
| `git-fetch` | Fetch remotes | `R` | git fetch --all |
| `git-push` | Push to upstream | `P` | git push (clean worktree only) |
| `git-sync` | Synchronise with upstream | `S` | git pull, then git push (clean worktree only) |
| `git-fetch-pr-data` | Fetch PR data | `p` | Fetch PR/MR status from GitHub/GitLab |
| `git-ci-checks` | View CI checks | `v` | View CI check logs for current worktree |
| `git-pr` | Open in browser | `o` | Open PR, branch, or repo in browser |
| `git-lazygit` | Open LazyGit | `g` | Open LazyGit in selected worktree |
| `git-run-command` | Run command | `!` | Run arbitrary shell command in worktree |

## Status Pane

| ID | Label | Default Key | Description |
|----|-------|-------------|-------------|
| `status-stage-file` | Stage/unstage file | `s` | Stage or unstage selected file |
| `status-commit-staged` | Open commit screen | `c` | Open the commit screen for staged changes (or prompt to stage all) |
| `status-commit-all` | Commit changes using git editor | `C` | Commit using git editor |
| `status-edit-file` | Edit file | `e` | Open selected file in editor |
| `status-delete-file` | Delete selected file or directory | — | Delete selected file or directory |

## Log Pane

| ID | Label | Default Key | Description |
|----|-------|-------------|-------------|
| `log-cherry-pick` | Cherry-pick commit | `C` | Cherry-pick commit to another worktree |
| `log-commit-view` | Browse commit files | — | Browse files changed in selected commit |

## Navigation

| ID | Label | Default Key | Description |
|----|-------|-------------|-------------|
| `nav-zoom-toggle` | Toggle zoom | `=` | Toggle zoom on focused pane |
| `nav-toggle-layout` | Toggle layout | `L` | Switch between default and top layout |
| `nav-filter` | Filter | `f` | Filter items in focused pane |
| `nav-search` | Search | `/` | Search items in focused pane |
| `nav-focus-worktrees` | Focus worktrees | `1` | Focus worktree pane |
| `nav-focus-status` | Focus status | `2` | Focus status pane |
| `nav-focus-log` | Focus log | `3` | Focus log pane |
| `nav-sort-cycle` | Cycle sort | `s` | Cycle sort mode (path/active/switched) |
| `nav-copy-path` | Copy path / file / SHA | `y` | Copy context-aware content (path, file, or commit SHA) |
| `nav-copy-branch` | Copy branch name | `Y` | Copy selected worktree branch name |
| `nav-copy-pr-url` | Copy PR/MR URL | — | Copy selected worktree PR/MR URL |

## Settings

| ID | Label | Default Key | Description |
|----|-------|-------------|-------------|
| `settings-theme` | Select theme | — | Change the application theme with live preview |
| `settings-taskboard` | Taskboard | `T` | Browse and toggle worktree tasks |
| `settings-help` | Help | `?` | Show help |
| `cache-clear` | Clear cache | — | Clear all in-memory and on-disk caches then refresh |
