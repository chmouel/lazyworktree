# Multiplexer Integration

LazyWorktree can create and manage tmux, zellij, or Herdr sessions directly from the TUI, giving each worktree a dedicated terminal environment.


## Overview

With multiplexer integration:

- Create dedicated tmux, zellij, or Herdr sessions for each worktree
- Define custom window/tab layouts with specific commands
- Switch to existing sessions or create new ones
- Manage sessions from the [command palette](../core/command-palette.md)

Default keybindings: `t` for tmux, `Z` for zellij, `H` for Herdr (configurable via [custom commands](../custom-commands.md)).

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

## Herdr Configuration

Herdr organises terminals into workspaces, each containing tabs. LazyWorktree maps configured `windows` to tabs: the first window becomes the workspace's initial tab, and its `name` and `cwd` are applied to that tab, whilst each additional window becomes another tab.

Two modes are available:

- `workspace` (default): one workspace per worktree inside an existing Herdr session.
- `session`: one named Herdr session per worktree, with its own workspace layout.

### Workspace per Worktree

```yaml
custom_commands:
  H:
    description: Herdr
    show_help: true
    herdr:
      mode: workspace
      session_name: "wt:$WORKTREE_NAME"
      attach: true
      on_exists: switch
      windows:
        - name: editor
          command: nvim
        - name: shell
        - name: tests
          command: npm run test:watch
```

By default the workspace is created in the session LazyWorktree is running in, or in the default Herdr session when no session is ambient. An empty `herdr_session` inherits the current session; set `herdr_session: default` to target the default session explicitly. A configured session that matches the one you are attached to reuses your current Herdr socket, so focusing works even when Herdr was started with a custom socket path. LazyWorktree starts a detached Herdr server when the target session is not running.

### Named Session per Worktree

```yaml
custom_commands:
  H:
    description: Herdr session
    show_help: true
    herdr:
      mode: session
      session_name: "wt-$WORKTREE_NAME"
      attach: true
      on_exists: switch
      windows:
        - name: editor
          command: nvim
        - name: shell
```

### Herdr Field Reference

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `mode` | string | `workspace` | `workspace` creates a workspace per worktree; `session` creates a named Herdr session |
| `session_name` | string | `wt:$WORKTREE_NAME` | Workspace label in workspace mode, session name in session mode (env vars supported) |
| `herdr_session` | string | `""` | Herdr session hosting the workspace (workspace mode only; empty inherits the current session, `default` targets the default session) |
| `attach` | bool | `true` | Attach or focus immediately; if `false`, show modal with instructions |
| `on_exists` | string | `switch` | Behaviour if the workspace or session exists: `switch`, `attach`, `kill`, `new` (for Herdr, `attach` is an alias of `switch`) |
| `windows` | list | `[{name: "shell"}]` | Tab definitions for the workspace |

#### Tab Fields

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `name` | string | `window-N` | Tab label (supports env vars) |
| `command` | string | `""` | Command to run (empty uses default shell) |
| `cwd` | string | `$WORKTREE_PATH` | Working directory (supports env vars) |

!!! note
    Herdr session names are limited to letters, digits, `.`, `_`, and `-`, with a maximum of 64 bytes. Other characters are replaced with `-`, and workspace labels are sanitised in the same manner so the palette can find them. A name that sanitises to nothing is rejected rather than silently falling back to the default session. The first tab takes the `name` and `cwd` configured for the first window. In workspace mode, LazyWorktree focuses the workspace directly when already running inside Herdr; otherwise it focuses a reused workspace before attaching. When the configured `herdr_session` differs from the session hosting the current client, LazyWorktree shows attach instructions instead, because focusing a workspace in another session cannot display it. In session mode, Herdr cannot switch between sessions from inside a running client, so LazyWorktree always shows attach instructions instead of nesting a second client — including when the session already exists.

!!! warning
    Herdr offers no reliable completion marker for a workspace layout. When a previous attempt failed part-way, a retry reuses the workspace that was already created and does **not** re-run the configured commands, so a partially configured workspace can be reused as-is. Use `on_exists: kill` to discard it and build a fresh layout; `on_exists: new` instead builds a fresh workspace under an incremented label and leaves the old one in place. In session mode, these options apply when the session is running, and reusing a running session ensures the worktree workspace exists, creating it when the previous attempt never got that far.

!!! note
    `new_tab` is honoured when LazyWorktree runs outside Herdr. Inside Herdr, workspace mode focuses the workspace in place and session mode shows attach instructions, because Herdr cannot nest a client or switch sessions from within a running client. A workspace in another session also shows attach instructions.

!!! warning
    `on_exists: kill` and `on_exists: new` act on the normalised name, exactly as they do for tmux. If sanitisation makes two worktrees share a name, killing or incrementing one can target the other worktree's session or workspace.

## on_exists Behaviour

Controls what happens when a session with the same name already exists:

| Value | Behaviour |
| --- | --- |
| `switch` | Switch to the existing session (default) |
| `attach` | Attach to the existing session in the current terminal |
| `kill` | Kill the existing session and create a new one |
| `new` | Create a new session with an incremented name (e.g., `wt:feature-1`) |

!!! note
    For Herdr, `attach` and `switch` both reuse the existing workspace or session; `attach` is effectively an alias of `switch`. A reused workspace is focused before the external client attaches, so the attach lands on the right workspace. When Herdr is already nested inside a running client, session mode shows attach instructions rather than switching.

## Environment Variables

All session configuration fields support environment variable substitution:

| Variable | Description | Example |
| --- | --- | --- |
| `$WORKTREE_NAME` | Name of the worktree | `my-feature` |
| `$WORKTREE_BRANCH` | Branch name for the worktree | `feature/my-feature` |
| `$WORKTREE_PATH` | Full path to the worktree directory | `/home/user/worktrees/my-feature` |
| `$MAIN_WORKTREE_PATH` | Path to the main/root worktree | `/home/user/repo` |
| `$REPO_NAME` | Repository key, usually `owner/repo` | `chmouel/lazyworktree` |
| `$REPO_OWNER` | Repository owner when available | `chmouel` |
| `$REPO_REPONAME` | Repository name without the owner | `lazyworktree` |
| `$LAZYWORKTREE_TYPE` | Source context, such as `pr`, `issue`, or `diff` when known | `pr` |
| `$LAZYWORKTREE_NUMBER` | PR/MR or issue number when known | `42` |
| `$LAZYWORKTREE_TITLE` | PR/MR or issue title when known | `Add worktree context` |

Contextual `LAZYWORKTREE_*` values are empty when no source context is known.

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

Press `F1`, `Ctrl+p`, or `:` to open the command palette, where you can:

- View all active sessions matching your configured `session_prefix`
- Switch between sessions quickly
- Create new sessions for worktrees that lack one
- Sessions are sorted by MRU (Most Recently Used) order
