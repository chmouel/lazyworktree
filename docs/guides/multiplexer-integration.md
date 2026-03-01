# Multiplexer Integration

LazyWorktree can create and manage tmux or zellij sessions directly from the TUI, giving each worktree a dedicated terminal environment.

<div class="mint-callout">
  <p><strong>Use this page when:</strong> you want to pair worktrees with multiplexer sessions for isolated, reproducible terminal layouts.</p>
</div>

## Overview

Multiplexer integration enables you to:

- Create dedicated tmux or zellij sessions for each worktree
- Define custom window/tab layouts with specific commands
- Automatically switch to existing sessions or create new ones
- Manage sessions from the [command palette](../core/command-palette.md)

Default keybindings: `t` for tmux, `Z` for zellij (configurable via [custom commands](../custom-commands.md)).

## tmux Configuration

### Basic Session

```yaml
custom_commands:
  t:
    description: Tmux session
    show_help: true
    tmux:
      session_name: "wt:$WORKTREE_NAME"
      attach: true
      on_exists: switch
```

### Multi-Window Session

```yaml
custom_commands:
  t:
    description: Tmux with layout
    show_help: true
    tmux:
      session_name: "wt:$WORKTREE_NAME"
      attach: true
      on_exists: switch
      windows:
        - name: editor
          command: nvim
        - name: shell
          command: zsh
        - name: lazygit
          command: lazygit
        - name: tests
          command: npm run test:watch
          cwd: $WORKTREE_PATH/tests
```

### tmux Field Reference

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `session_name` | string | `wt:$WORKTREE_NAME` | Session name (env vars supported, special chars replaced) |
| `attach` | bool | `true` | Attach immediately; if `false`, show modal with instructions |
| `on_exists` | string | `switch` | Behaviour if session exists: `switch`, `attach`, `kill`, `new` |
| `windows` | list | `[{name: "shell"}]` | Window definitions for the session |

#### Window Fields

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `name` | string | `window-N` | Window name (supports env vars) |
| `command` | string | `""` | Command to run (empty uses default shell) |
| `cwd` | string | `$WORKTREE_PATH` | Working directory (supports env vars) |

## zellij Configuration

### Basic Session

```yaml
custom_commands:
  Z:
    description: Zellij
    show_help: true
    zellij:
      session_name: "wt:$WORKTREE_NAME"
      attach: true
      on_exists: switch
```

### Multi-Tab Session

```yaml
custom_commands:
  Z:
    description: Zellij with tabs
    show_help: true
    zellij:
      session_name: "wt-$WORKTREE_NAME"
      attach: true
      on_exists: switch
      windows:
        - name: editor
          command: nvim
        - name: shell
        - name: server
          command: npm run dev
        - name: tests
          command: npm run test:watch
```

### zellij Field Reference

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `session_name` | string | `wt:$WORKTREE_NAME` | Session name (env vars supported, special chars replaced) |
| `attach` | bool | `true` | Attach immediately; if `false`, show modal with instructions |
| `on_exists` | string | `switch` | Behaviour if session exists: `switch`, `attach`, `kill`, `new` |
| `windows` | list | `[{name: "shell"}]` | Tab definitions for the session |

#### Tab Fields

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `name` | string | `window-N` | Tab name (supports env vars) |
| `command` | string | `""` | Command to run (empty uses default shell) |
| `cwd` | string | `$WORKTREE_PATH` | Working directory (supports env vars) |

!!! important
    Session names with `/`, `\`, or `:` are replaced with `-`. If `windows` is empty, a single shell window/tab is created by default.

## on_exists Behaviour

Controls what happens when a session with the same name already exists:

| Value | Behaviour |
| --- | --- |
| `switch` | Switch to the existing session (default) |
| `attach` | Attach to the existing session in the current terminal |
| `kill` | Kill the existing session and create a new one |
| `new` | Create a new session with an incremented name (e.g., `wt:feature-1`) |

## Environment Variables

All session configuration fields support environment variable substitution:

| Variable | Description | Example |
| --- | --- | --- |
| `$WORKTREE_NAME` | Name of the worktree | `my-feature` |
| `$WORKTREE_BRANCH` | Branch name for the worktree | `feature/my-feature` |
| `$WORKTREE_PATH` | Full path to the worktree directory | `/home/user/worktrees/my-feature` |
| `$MAIN_WORKTREE_PATH` | Path to the main/root worktree | `/home/user/repo` |
| `$REPO_NAME` | Name of the repository | `lazyworktree` |

### Example Using Environment Variables

```yaml
custom_commands:
  t:
    description: Project session
    tmux:
      session_name: "$REPO_NAME:$WORKTREE_NAME"
      windows:
        - name: "$WORKTREE_BRANCH"
          command: nvim
          cwd: $WORKTREE_PATH
        - name: root
          command: zsh
          cwd: $MAIN_WORKTREE_PATH
```

## Session Prefix

Filter which sessions appear in the command palette by setting a prefix:

```yaml
session_prefix: "wt-"
```

Only sessions whose names start with the configured prefix are listed. Use a unique prefix like `wt-` or `work-` to distinguish LazyWorktree sessions from others.

## New Terminal Tab

Launch multiplexer sessions in a new terminal tab instead of the current one:

```yaml
custom_commands:
  t:
    description: Tmux in new tab
    new_tab: true
    tmux:
      session_name: "wt:$WORKTREE_NAME"
      attach: true
```

!!! tip
    New-tab support requires Kitty (with remote control enabled), WezTerm, or iTerm.

## CLI Integration

Launch multiplexer sessions from scripts or the command line:

```bash
# Launch tmux session for a specific worktree
lazyworktree exec --key=t --workspace=my-feature

# Launch zellij session
lazyworktree exec -k Z -w my-feature

# Auto-detect worktree from current directory
cd ~/worktrees/repo/my-feature
lazyworktree exec --key=t
```

## Command Palette Session Management

Press `Ctrl+p` or `:` to open the command palette, where you can:

- View all active sessions matching your configured `session_prefix`
- Switch between sessions quickly
- Create new sessions for worktrees that lack one
- Sessions are sorted by MRU (Most Recently Used) order
